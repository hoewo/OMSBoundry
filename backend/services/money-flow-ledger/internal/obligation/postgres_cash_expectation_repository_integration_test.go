//go:build integration

package obligation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"omsbizfoundry/money-flow-ledger/internal/allocation"
	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/cashtransaction"
	"omsbizfoundry/money-flow-ledger/internal/database"
	"omsbizfoundry/money-flow-ledger/internal/testdatabase"
)

func TestPostgresCashExpectationLifecycleProjectionAndIdempotency(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "排期管理员",
	}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	service := NewService(NewPostgresRepository(environment.Pool))
	confirmed := createConfirmedAdjustmentObligation(t, ctx, service, actor, owner.ID, counterparty.ID, "100")

	report, err := service.CurrentCashForecast(ctx)
	if err != nil || len(report.ScheduledItems) != 0 || len(report.UnscheduledItems) != 1 ||
		report.UnscheduledItems[0].Obligation.ID != confirmed.ID ||
		report.UnscheduledSummaries[0].OutstandingAmount != "100" {
		t.Fatalf("initial CurrentCashForecast() = %#v, %v", report, err)
	}

	firstInput := RecordCashExpectationInput{
		CommandID: "41111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
		ExpectedCurrentRevision: 1, Action: "set", ExpectedOn: "2026-10-15",
		Reason: "客户首次确认回款日期", Actor: actor,
	}
	first, err := service.RecordCashExpectation(ctx, firstInput)
	if err != nil || first.Replayed || first.Expectation.Sequence != 1 || first.Expectation.ExpectedOn != "2026-10-15" ||
		first.Expectation.ObligationRevision != 1 || first.Expectation.RecordedBy != actor {
		t.Fatalf("first RecordCashExpectation() = %#v, %v", first, err)
	}
	replayed, err := service.RecordCashExpectation(ctx, firstInput)
	if err != nil || !replayed.Replayed || replayed.Expectation.ID != first.Expectation.ID || replayed.Expectation.Sequence != 1 {
		t.Fatalf("cash expectation replay = %#v, %v", replayed, err)
	}
	changed := firstInput
	changed.ExpectedOn = "2026-10-16"
	_, err = service.RecordCashExpectation(ctx, changed)
	var idempotencyConflict *CashExpectationIdempotencyConflictError
	if !errors.As(err, &idempotencyConflict) {
		t.Fatalf("changed replay error = %v", err)
	}

	second, err := service.RecordCashExpectation(ctx, RecordCashExpectationInput{
		CommandID: "42111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
		ExpectedCurrentRevision: 1, Action: "set", ExpectedOn: "2026-10-20",
		Reason: "客户更新回款日期", Actor: actor,
	})
	if err != nil || second.Expectation.Sequence != 2 || second.Expectation.ExpectedOn != "2026-10-20" {
		t.Fatalf("second RecordCashExpectation() = %#v, %v", second, err)
	}
	report, err = service.CurrentCashForecast(ctx)
	if err != nil || len(report.ScheduledItems) != 1 || len(report.UnscheduledItems) != 0 ||
		report.ScheduledItems[0].Expectation.Sequence != 2 ||
		report.ScheduledSummaries[0].ExpectedOn != "2026-10-20" ||
		report.ScheduledSummaries[0].OutstandingAmount != "100" {
		t.Fatalf("scheduled CurrentCashForecast() = %#v, %v", report, err)
	}

	cashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	receipt, err := cashService.Create(ctx, cashtransaction.CreateInput{
		CommandID: "43111111-1111-4111-8111-111111111111", Type: "receipt",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Description: "预计回款后的实际收款", Amount: "40", Currency: "CNY",
		OccurredOn: "2026-10-20", AccountReference: "测试账户",
		ManualSourceReference: "排期测试收款依据", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create cash for expectation: %v", err)
	}
	allocationService := allocation.NewService(allocation.NewPostgresRepository(environment.Pool))
	if _, err := allocationService.Apply(ctx, allocation.ApplyInput{
		CommandID:         "44111111-1111-4111-8111-111111111111",
		CashTransactionID: receipt.Transaction.ID, ObligationID: confirmed.ID,
		ExpectedObligationRevision: 1, Amount: "40", ManualSourceReference: "排期测试核销依据", Actor: actor,
	}); err != nil {
		t.Fatalf("apply cash for expectation: %v", err)
	}
	report, err = service.CurrentCashForecast(ctx)
	if err != nil || report.ScheduledSummaries[0].OutstandingAmount != "60" ||
		report.ScheduledItems[0].Obligation.OutstandingAmount != "60" {
		t.Fatalf("forecast after allocation = %#v, %v", report, err)
	}

	cleared, err := service.RecordCashExpectation(ctx, RecordCashExpectationInput{
		CommandID: "45111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
		ExpectedCurrentRevision: 1, Action: "clear", Reason: "客户日期重新待定", Actor: actor,
	})
	if err != nil || cleared.Expectation.Sequence != 3 || cleared.Expectation.Action != CashExpectationActionClear ||
		cleared.Expectation.ExpectedOn != "" {
		t.Fatalf("clear RecordCashExpectation() = %#v, %v", cleared, err)
	}
	_, err = service.RecordCashExpectation(ctx, RecordCashExpectationInput{
		CommandID: "46111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
		ExpectedCurrentRevision: 1, Action: "clear", Reason: "不能重复清除", Actor: actor,
	})
	var clearConflict *CashExpectationClearConflictError
	if !errors.As(err, &clearConflict) {
		t.Fatalf("repeated clear error = %v", err)
	}
	report, err = service.CurrentCashForecast(ctx)
	if err != nil || len(report.ScheduledItems) != 0 || len(report.UnscheduledItems) != 1 ||
		report.UnscheduledItems[0].Obligation.OutstandingAmount != "60" {
		t.Fatalf("forecast after clear = %#v, %v", report, err)
	}
	history, err := service.CashExpectationHistory(ctx, confirmed.ID)
	if err != nil || len(history.Facts) != 3 || history.Facts[0].ID != first.Expectation.ID ||
		history.Facts[1].ID != second.Expectation.ID || history.Facts[2].ID != cleared.Expectation.ID {
		t.Fatalf("CashExpectationHistory() = %#v, %v", history, err)
	}

	if _, err := environment.Pool.Exec(ctx, `UPDATE obligation_cash_expectations SET expectation_reason = 'changed' WHERE id = $1`, first.Expectation.ID); !isImmutableCashExpectationError(err) {
		t.Fatalf("update immutable cash expectation error = %v", err)
	}
	if _, err := environment.Pool.Exec(ctx, `DELETE FROM obligation_cash_expectation_commands WHERE obligation_cash_expectation_id = $1`, first.Expectation.ID); !isImmutableCashExpectationError(err) {
		t.Fatalf("delete immutable cash expectation command error = %v", err)
	}

	adjusted, err := service.AdjustAmount(ctx, AdjustAmountInput{
		CommandID: "47111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
		ExpectedCurrentRevision: 1, Type: "increase", Amount: "10", Reason: "追加服务范围", Actor: actor,
	})
	if err != nil {
		t.Fatalf("adjust after cash expectation: %v", err)
	}
	_, err = service.RecordCashExpectation(ctx, RecordCashExpectationInput{
		CommandID: "48111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
		ExpectedCurrentRevision: 1, Action: "set", ExpectedOn: "2026-11-01", Reason: "过期 revision", Actor: actor,
	})
	var revisionConflict *CurrentRevisionConflictError
	if !errors.As(err, &revisionConflict) || revisionConflict.Actual != 2 {
		t.Fatalf("stale cash expectation revision error = %v", err)
	}
	fourth, err := service.RecordCashExpectation(ctx, RecordCashExpectationInput{
		CommandID: "49111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
		ExpectedCurrentRevision: adjusted.Obligation.CurrentRevision, Action: "set", ExpectedOn: "2026-11-01",
		Reason: "调额后重新排期", Actor: actor,
	})
	if err != nil || fourth.Expectation.Sequence != 4 || fourth.Expectation.ObligationRevision != 2 {
		t.Fatalf("cash expectation after adjustment = %#v, %v", fourth, err)
	}

	var facts, commands, audits int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM obligation_cash_expectations WHERE obligation_id = $1),
			(SELECT count(*) FROM obligation_cash_expectation_commands WHERE obligation_id = $1),
			(SELECT count(*) FROM audit_entries WHERE object_type = 'obligation' AND object_id = $1 AND action = $2)
	`, confirmed.ID, AuditActionCashExpectationRecorded).Scan(&facts, &commands, &audits); err != nil {
		t.Fatalf("count cash expectation facts: %v", err)
	}
	if facts != 4 || commands != 4 || audits != 4 {
		t.Fatalf("cash expectation counts facts=%d commands=%d audits=%d", facts, commands, audits)
	}
}

func TestPostgresCashExpectationConcurrencyAndDatabaseGuards(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "并发排期管理员",
	}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	service := NewService(NewPostgresRepository(environment.Pool))
	confirmed := createConfirmedAdjustmentObligation(t, ctx, service, actor, owner.ID, counterparty.ID, "100")

	inputs := []RecordCashExpectationInput{
		{CommandID: "51111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID, ExpectedCurrentRevision: 1, Action: "set", ExpectedOn: "2026-10-21", Reason: "并发排期一", Actor: actor},
		{CommandID: "52111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID, ExpectedCurrentRevision: 1, Action: "set", ExpectedOn: "2026-10-22", Reason: "并发排期二", Actor: actor},
	}
	start := make(chan struct{})
	type result struct {
		value RecordCashExpectationResult
		err   error
	}
	results := make(chan result, 2)
	var workers sync.WaitGroup
	for _, input := range inputs {
		input := input
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			value, err := service.RecordCashExpectation(ctx, input)
			results <- result{value: value, err: err}
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	sequences := map[int64]bool{}
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent RecordCashExpectation() error = %v", result.err)
		}
		sequences[result.value.Expectation.Sequence] = true
	}
	if !sequences[1] || !sequences[2] || len(sequences) != 2 {
		t.Fatalf("concurrent cash expectation sequences = %#v", sequences)
	}

	_, err := environment.Pool.Exec(ctx, `
		INSERT INTO obligation_cash_expectations (
			id, obligation_id, sequence, action, expected_on, obligation_revision,
			source_kind, expectation_reason, recorded_by_actor_kind, recorded_by_actor_id,
			recorded_by_actor_display_name, recorded_at
		)
		VALUES (
			'59999999-9999-4999-8999-999999999999', $1, 4, 'set', '2026-10-30', 1,
			'manual', '跳号事实', $2, $3, $4, clock_timestamp()
		)
	`, confirmed.ID, actor.Kind, actor.ID, actor.DisplayName)
	if !isCashExpectationCheckViolation(err) {
		t.Fatalf("direct sequence gap error = %v", err)
	}

	draft, err := service.CreateDraft(ctx, CreateDraftInput{
		CommandID: "53111111-1111-4111-8111-111111111111", Direction: "receivable",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Title: "草稿排期拒绝", OriginalAmount: "10", Currency: "CNY", DueOn: "2026-11-30",
		ManualSourceReference: "草稿依据", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create draft for expectation rejection: %v", err)
	}
	_, err = service.RecordCashExpectation(ctx, RecordCashExpectationInput{
		CommandID: "54111111-1111-4111-8111-111111111111", ObligationID: draft.Obligation.ID,
		ExpectedCurrentRevision: 1, Action: "set", ExpectedOn: "2026-11-01", Reason: "草稿不能排期", Actor: actor,
	})
	var stateConflict *CashExpectationStateConflictError
	if !errors.As(err, &stateConflict) || stateConflict.Status != StatusDraft {
		t.Fatalf("draft cash expectation error = %v", err)
	}

	zero := createConfirmedAdjustmentObligation(t, ctx, service, actor, owner.ID, counterparty.ID, "20")
	if _, err := service.RecordRemission(ctx, RecordRemissionInput{
		CommandID: "55111111-1111-4111-8111-111111111111", ObligationID: zero.ID,
		ExpectedCurrentRevision: 1, Amount: "20", Reason: "归零测试", ManualSourceReference: "归零依据", Actor: actor,
	}); err != nil {
		t.Fatalf("remit zero expectation obligation: %v", err)
	}
	_, err = service.RecordCashExpectation(ctx, RecordCashExpectationInput{
		CommandID: "56111111-1111-4111-8111-111111111111", ObligationID: zero.ID,
		ExpectedCurrentRevision: 1, Action: "set", ExpectedOn: "2026-11-02", Reason: "零余额不能排期", Actor: actor,
	})
	var balanceConflict *CashExpectationBalanceConflictError
	if !errors.As(err, &balanceConflict) {
		t.Fatalf("zero balance cash expectation error = %v", err)
	}
}

func isImmutableCashExpectationError(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "55000"
}

func isCashExpectationCheckViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23514"
}
