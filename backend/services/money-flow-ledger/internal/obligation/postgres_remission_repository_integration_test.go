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

func TestPostgresObligationRemissionBalancesRevisionsAndIdempotency(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "减免管理员",
	}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	service := NewService(NewPostgresRepository(environment.Pool))
	confirmed := createConfirmedAdjustmentObligation(t, ctx, service, actor, owner.ID, counterparty.ID, "100")
	adjusted, err := service.AdjustAmount(ctx, AdjustAmountInput{
		CommandID: "51111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
		ExpectedCurrentRevision: 1, Type: "increase", Amount: "20", Reason: "先增加业务依据金额", Actor: actor,
	})
	if err != nil {
		t.Fatalf("adjust before remission: %v", err)
	}
	if adjusted.Obligation.CurrentRevision != 2 || adjusted.Obligation.EffectiveAmount != "120" {
		t.Fatalf("adjusted obligation = %#v", adjusted.Obligation)
	}

	cashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	cashResult, err := cashService.Create(ctx, cashtransaction.CreateInput{
		CommandID: "61111111-1111-4111-8111-111111111111", Type: "receipt",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Description: "减免测试收款", Amount: "100", Currency: "CNY",
		OccurredOn: "2026-09-19", AccountReference: "减免测试账户",
		ManualSourceReference: "减免测试回单", ExternalTransactionReference: "REMISSION-CASH-001", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create remission cash: %v", err)
	}
	allocationRepository := allocation.NewPostgresRepository(environment.Pool)
	allocationService := allocation.NewService(allocationRepository)
	firstAllocation, err := allocationService.Apply(ctx, allocation.ApplyInput{
		CommandID:         "71111111-1111-4111-8111-111111111111",
		CashTransactionID: cashResult.Transaction.ID, ObligationID: confirmed.ID,
		ExpectedObligationRevision: 2, Amount: "40", ManualSourceReference: "减免前分配四十", Actor: actor,
	})
	if err != nil {
		t.Fatalf("apply before remission: %v", err)
	}

	firstInput := RecordRemissionInput{
		CommandID: "81111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
		ExpectedCurrentRevision: 2, Amount: "30", Reason: "协商免除部分服务费",
		ManualSourceReference: "减免确认函 RM-001", Actor: actor,
	}
	first, err := service.RecordRemission(ctx, firstInput)
	if err != nil || first.Replayed || first.Obligation.CurrentRevision != 2 ||
		first.Obligation.OriginalAmount != "100" || first.Obligation.EffectiveAmount != "120" ||
		first.Obligation.AllocatedAmount != "40" || first.Obligation.RemittedAmount != "30" ||
		first.Obligation.OutstandingAmount != "50" || first.Obligation.Status != StatusOpen ||
		first.Remission.ObligationRevision != 2 || first.Remission.Amount != "30" ||
		first.Remission.RecordedEffectiveAmount != "120" || first.Remission.Currency != "CNY" ||
		first.Remission.RecordedBy != actor {
		t.Fatalf("first RecordRemission() = %#v, %v", first, err)
	}
	replayed, err := service.RecordRemission(ctx, firstInput)
	if err != nil || !replayed.Replayed || replayed.Remission.ID != first.Remission.ID || replayed.Obligation.RemittedAmount != "30" {
		t.Fatalf("remission replay = %#v, %v", replayed, err)
	}
	changed := firstInput
	changed.Reason = "改变减免原因"
	_, err = service.RecordRemission(ctx, changed)
	var idempotencyConflict *RemissionIdempotencyConflictError
	if !errors.As(err, &idempotencyConflict) {
		t.Fatalf("changed remission replay error = %v", err)
	}

	second, err := service.RecordRemission(ctx, RecordRemissionInput{
		CommandID: "91111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
		ExpectedCurrentRevision: 2, Amount: "50", Reason: "免除剩余未核销部分",
		ManualSourceReference: "减免确认函 RM-002", Actor: actor,
	})
	if err != nil || second.Obligation.CurrentRevision != 2 || second.Obligation.RemittedAmount != "80" ||
		second.Obligation.OutstandingAmount != "0" || second.Obligation.Status != StatusOpen {
		t.Fatalf("second RecordRemission() = %#v, %v", second, err)
	}
	_, err = service.RecordRemission(ctx, RecordRemissionInput{
		CommandID: "a1111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
		ExpectedCurrentRevision: 2, Amount: "1", Reason: "不应超过剩余金额",
		ManualSourceReference: "超额减免依据", Actor: actor,
	})
	var balanceConflict *RemissionBalanceConflictError
	if !errors.As(err, &balanceConflict) || balanceConflict.Requested != "1" || balanceConflict.Outstanding != "0" {
		t.Fatalf("over remission error = %#v / %v", balanceConflict, err)
	}

	_, err = allocationService.Apply(ctx, allocation.ApplyInput{
		CommandID:         "b1111111-1111-4111-8111-111111111111",
		CashTransactionID: cashResult.Transaction.ID, ObligationID: confirmed.ID,
		ExpectedObligationRevision: 2, Amount: "1", ManualSourceReference: "减免归零后分配", Actor: actor,
	})
	var allocationBalanceConflict *allocation.ObligationBalanceConflictError
	if !errors.As(err, &allocationBalanceConflict) || allocationBalanceConflict.Available != "0" {
		t.Fatalf("allocation after full remission error = %v", err)
	}

	_, err = service.AdjustAmount(ctx, AdjustAmountInput{
		CommandID: "c1111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
		ExpectedCurrentRevision: 2, Type: "decrease", Amount: "1", Reason: "不能低于已核销与减免总额", Actor: actor,
	})
	var adjustmentConflict *AmountAdjustmentResultConflictError
	if !errors.As(err, &adjustmentConflict) || adjustmentConflict.Reason != "below_allocated_and_remitted" ||
		adjustmentConflict.NetAllocatedAmount != "40" || adjustmentConflict.RemittedAmount != "80" {
		t.Fatalf("adjustment below allocated and remitted error = %#v / %v", adjustmentConflict, err)
	}
	increased, err := service.AdjustAmount(ctx, AdjustAmountInput{
		CommandID: "d1111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
		ExpectedCurrentRevision: 2, Type: "increase", Amount: "10", Reason: "新增有效服务范围", Actor: actor,
	})
	if err != nil || increased.Obligation.CurrentRevision != 3 || increased.Obligation.EffectiveAmount != "130" ||
		increased.Obligation.RemittedAmount != "80" || increased.Obligation.OutstandingAmount != "10" {
		t.Fatalf("increase after remission = %#v, %v", increased, err)
	}
	secondAllocation, err := allocationService.Apply(ctx, allocation.ApplyInput{
		CommandID:         "e1111111-1111-4111-8111-111111111111",
		CashTransactionID: cashResult.Transaction.ID, ObligationID: confirmed.ID,
		ExpectedObligationRevision: 3, Amount: "10", ManualSourceReference: "增加后分配十元", Actor: actor,
	})
	if err != nil {
		t.Fatalf("apply after increase: %v", err)
	}

	details, found, err := service.GetDetails(ctx, confirmed.ID)
	if err != nil || !found || details.Obligation.OriginalAmount != "100" || details.Obligation.EffectiveAmount != "130" ||
		details.Obligation.AllocatedAmount != "50" || details.Obligation.RemittedAmount != "80" ||
		details.Obligation.OutstandingAmount != "0" || len(details.Revisions) != 3 ||
		len(details.Adjustments) != 2 || len(details.Remissions) != 2 {
		t.Fatalf("GetDetails() = %#v, %v, %v", details, found, err)
	}
	if details.Remissions[0].ID != first.Remission.ID || details.Remissions[1].ID != second.Remission.ID ||
		details.Remissions[0].ObligationRevision != 2 || details.Remissions[1].ObligationRevision != 2 ||
		details.Remissions[0].RecordedEffectiveAmount != "120" || details.Remissions[1].RecordedEffectiveAmount != "120" {
		t.Fatalf("remission history = %#v", details.Remissions)
	}
	allocations, err := allocationService.ListForObligation(ctx, confirmed.ID)
	if err != nil || len(allocations) != 2 ||
		allocations[0].ID != secondAllocation.Allocation.ID || allocations[0].ObligationRevision != 3 ||
		allocations[1].ID != firstAllocation.Allocation.ID || allocations[1].ObligationRevision != 2 {
		t.Fatalf("allocation history = %#v, %v", allocations, err)
	}
	_, err = service.RecordRemission(ctx, RecordRemissionInput{
		CommandID: "f1111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
		ExpectedCurrentRevision: 2, Amount: "1", Reason: "过期 revision 减免",
		ManualSourceReference: "过期减免依据", Actor: actor,
	})
	var revisionConflict *CurrentRevisionConflictError
	if !errors.As(err, &revisionConflict) || revisionConflict.Expected != 2 || revisionConflict.Actual != 3 {
		t.Fatalf("stale remission revision error = %v", err)
	}

	var remissions, commands, audits int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM obligation_remissions WHERE obligation_id = $1),
			(SELECT count(*) FROM obligation_remission_commands WHERE obligation_id = $1),
			(SELECT count(*) FROM audit_entries WHERE object_type = 'obligation' AND object_id = $1 AND action = 'obligation_remission_recorded')
	`, confirmed.ID).Scan(&remissions, &commands, &audits); err != nil {
		t.Fatalf("read remission fact counts: %v", err)
	}
	if remissions != 2 || commands != 2 || audits != 2 {
		t.Fatalf("remission fact counts remissions=%d commands=%d audits=%d", remissions, commands, audits)
	}
}

func TestPostgresObligationRemissionConcurrency(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "并发减免管理员"}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	service := NewService(NewPostgresRepository(environment.Pool))

	type result struct {
		value RecordRemissionResult
		err   error
	}
	run := func(inputs []RecordRemissionInput) []result {
		t.Helper()
		start := make(chan struct{})
		results := make(chan result, len(inputs))
		var workers sync.WaitGroup
		for _, input := range inputs {
			input := input
			workers.Add(1)
			go func() {
				defer workers.Done()
				<-start
				value, err := service.RecordRemission(ctx, input)
				results <- result{value: value, err: err}
			}()
		}
		close(start)
		workers.Wait()
		close(results)
		values := make([]result, 0, len(inputs))
		for value := range results {
			values = append(values, value)
		}
		return values
	}

	same := createConfirmedAdjustmentObligation(t, ctx, service, actor, owner.ID, counterparty.ID, "100")
	sameInput := RecordRemissionInput{
		CommandID: "51111111-1111-4111-8111-111111111111", ObligationID: same.ID,
		ExpectedCurrentRevision: 1, Amount: "20", Reason: "同命令并发减免",
		ManualSourceReference: "同命令减免依据", Actor: actor,
	}
	sameResults := run([]RecordRemissionInput{sameInput, sameInput})
	replays := 0
	for _, result := range sameResults {
		if result.err != nil || result.value.Remission.Amount != "20" {
			t.Fatalf("same command result = %#v, %v", result.value, result.err)
		}
		if result.value.Replayed {
			replays++
		}
	}
	if replays != 1 {
		t.Fatalf("same command replay count = %d", replays)
	}

	different := createConfirmedAdjustmentObligation(t, ctx, service, actor, owner.ID, counterparty.ID, "100")
	differentResults := run([]RecordRemissionInput{
		{CommandID: "61111111-1111-4111-8111-111111111111", ObligationID: different.ID, ExpectedCurrentRevision: 1, Amount: "60", Reason: "并发减免一", ManualSourceReference: "并发依据一", Actor: actor},
		{CommandID: "71111111-1111-4111-8111-111111111111", ObligationID: different.ID, ExpectedCurrentRevision: 1, Amount: "60", Reason: "并发减免二", ManualSourceReference: "并发依据二", Actor: actor},
	})
	successes, balanceConflicts := 0, 0
	for _, result := range differentResults {
		if result.err == nil {
			successes++
			continue
		}
		var conflict *RemissionBalanceConflictError
		if errors.As(result.err, &conflict) && conflict.Outstanding == "40" {
			balanceConflicts++
			continue
		}
		t.Fatalf("unexpected different command error = %v", result.err)
	}
	if successes != 1 || balanceConflicts != 1 {
		t.Fatalf("different command results successes=%d balance_conflicts=%d", successes, balanceConflicts)
	}
}

func TestPostgresObligationRemissionRollsBackAndProtectsFacts(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "减免保护管理员"}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	service := NewService(NewPostgresRepository(environment.Pool))
	confirmed := createConfirmedAdjustmentObligation(t, ctx, service, actor, owner.ID, counterparty.ID, "100")
	repository := NewPostgresRepository(environment.Pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	_, _, _, err := repository.RecordRemission(ctx, Remission{
		ID: "51111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
		ObligationRevision: 1, Amount: "10", SourceKind: SourceKindManual,
		Reason: "审计失败应整体回滚", ManualSourceReference: "减免回滚依据",
		RecordedBy: actor, RecordedAt: now,
	}, RemissionCommand{
		CommandID: "61111111-1111-4111-8111-111111111111", PayloadHash: make([]byte, 32), Actor: actor, RecordedAt: now,
	}, audit.Entry{
		ID: "71111111-1111-4111-8111-111111111111", ObjectType: audit.ObjectObligation, ObjectID: confirmed.ID,
		Action: AuditActionRemissionRecorded, CommandID: "61111111-1111-4111-8111-111111111111",
		Actor: actor, OccurredAt: now, Summary: "", Details: map[string]any{},
	})
	if err == nil {
		t.Fatal("RecordRemission() audit failure error = nil")
	}
	assertRemissionFactCounts(t, ctx, environment, confirmed.ID, 0, 0, 0)

	recorded, err := service.RecordRemission(ctx, RecordRemissionInput{
		CommandID: "81111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
		ExpectedCurrentRevision: 1, Amount: "20", Reason: "形成受保护减免",
		ManualSourceReference: "受保护减免依据", Actor: actor,
	})
	if err != nil {
		t.Fatalf("service.RecordRemission() error = %v", err)
	}
	for _, statement := range []string{
		`UPDATE obligation_remissions SET remission_reason = 'changed' WHERE obligation_id = $1`,
		`DELETE FROM obligation_remissions WHERE obligation_id = $1`,
		`UPDATE obligation_remission_commands SET actor_display_name = 'changed' WHERE obligation_id = $1`,
		`DELETE FROM obligation_remission_commands WHERE obligation_id = $1`,
	} {
		_, err := environment.Pool.Exec(ctx, statement, confirmed.ID)
		var databaseError *pgconn.PgError
		if !errors.As(err, &databaseError) || databaseError.Code != "55000" {
			t.Fatalf("immutable statement error = %v", err)
		}
	}

	invalidStatements := []string{
		`INSERT INTO obligation_remissions (
			id, obligation_id, obligation_revision, amount, recorded_effective_amount,
			currency, source_kind, remission_reason, manual_source_reference,
			recorded_by_actor_kind, recorded_by_actor_id, recorded_by_actor_display_name, recorded_at
		 ) VALUES (
			'91111111-1111-4111-8111-111111111111', $1, 1, 1, 999,
			'CNY', 'manual', '伪造有效金额', '伪造减免依据',
			'local_user', '11111111-1111-4111-8111-111111111111', '减免保护管理员', clock_timestamp()
		 )`,
		`INSERT INTO obligation_remissions (
			id, obligation_id, obligation_revision, amount, recorded_effective_amount,
			currency, source_kind, remission_reason, manual_source_reference,
			recorded_by_actor_kind, recorded_by_actor_id, recorded_by_actor_display_name, recorded_at
		 ) VALUES (
			'a1111111-1111-4111-8111-111111111111', $1, 1, 81, 100,
			'CNY', 'manual', '超额减免', '超额减免依据',
			'local_user', '11111111-1111-4111-8111-111111111111', '减免保护管理员', clock_timestamp()
		 )`,
		`INSERT INTO obligation_amount_adjustments (
			id, obligation_id, source_revision, resulting_revision, adjustment_type,
			adjustment_amount, previous_effective_amount, resulting_effective_amount,
			currency, source_kind, adjustment_reason,
			adjusted_by_actor_kind, adjusted_by_actor_id, adjusted_by_actor_display_name, adjusted_at
		 ) VALUES (
			'b1111111-1111-4111-8111-111111111111', $1, 1, 2, 'decrease',
			90, 100, 10, 'CNY', 'manual', '不应低于已减免金额',
			'local_user', '11111111-1111-4111-8111-111111111111', '减免保护管理员', clock_timestamp()
		 )`,
	}
	for _, statement := range invalidStatements {
		_, err := environment.Pool.Exec(ctx, statement, confirmed.ID)
		var databaseError *pgconn.PgError
		if !errors.As(err, &databaseError) || databaseError.Code != "23514" {
			t.Fatalf("protected statement error = %v", err)
		}
	}
	cashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	cashResult, err := cashService.Create(ctx, cashtransaction.CreateInput{
		CommandID: "c1111111-1111-4111-8111-111111111111", Type: "receipt",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Description: "减免保护收款", Amount: "100", Currency: "CNY", OccurredOn: "2026-09-19",
		AccountReference: "减免保护账户", ManualSourceReference: "减免保护回单",
		ExternalTransactionReference: "REMISSION-PROTECTION-CASH", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create protection cash: %v", err)
	}
	_, err = environment.Pool.Exec(ctx, `
		INSERT INTO allocations (
			id, cash_transaction_id, obligation_id, obligation_revision,
			amount, currency, source_kind, manual_source_reference,
			applied_by_actor_kind, applied_by_actor_id, applied_by_actor_display_name, applied_at
		) VALUES (
			'd1111111-1111-4111-8111-111111111111', $1, $2, 1,
			81, 'CNY', 'manual', '不应超过扣除减免后的余额',
			'local_user', '11111111-1111-4111-8111-111111111111', '减免保护管理员', clock_timestamp()
		)
	`, cashResult.Transaction.ID, confirmed.ID)
	var allocationDatabaseError *pgconn.PgError
	if !errors.As(err, &allocationDatabaseError) || allocationDatabaseError.Code != "23514" {
		t.Fatalf("direct allocation after remission error = %v", err)
	}
	assertRemissionFactCounts(t, ctx, environment, confirmed.ID, 1, 1, 1)
	if recorded.Obligation.RemittedAmount != "20" || recorded.Obligation.OutstandingAmount != "80" {
		t.Fatalf("recorded remission = %#v", recorded)
	}
}

func TestPostgresObligationRemissionAndAllocationApplySerialize(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "减免分配并发管理员"}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	obligationService := NewService(NewPostgresRepository(environment.Pool))
	confirmed := createConfirmedAdjustmentObligation(t, ctx, obligationService, actor, owner.ID, counterparty.ID, "100")
	cashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	cashResult, err := cashService.Create(ctx, cashtransaction.CreateInput{
		CommandID: "51111111-1111-4111-8111-111111111111", Type: "receipt",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Description: "减免与分配竞态收款", Amount: "100", Currency: "CNY",
		OccurredOn: "2026-09-19", AccountReference: "减免分配竞态账户",
		ManualSourceReference: "减免分配竞态回单", ExternalTransactionReference: "REMISSION-APPLY-RACE", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create race cash: %v", err)
	}
	allocationService := allocation.NewService(allocation.NewPostgresRepository(environment.Pool))

	start := make(chan struct{})
	remissionErrors := make(chan error, 1)
	allocationErrors := make(chan error, 1)
	go func() {
		<-start
		_, err := obligationService.RecordRemission(ctx, RecordRemissionInput{
			CommandID: "61111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
			ExpectedCurrentRevision: 1, Amount: "60", Reason: "竞态减免六十",
			ManualSourceReference: "竞态减免依据", Actor: actor,
		})
		remissionErrors <- err
	}()
	go func() {
		<-start
		_, err := allocationService.Apply(ctx, allocation.ApplyInput{
			CommandID:         "71111111-1111-4111-8111-111111111111",
			CashTransactionID: cashResult.Transaction.ID, ObligationID: confirmed.ID,
			ExpectedObligationRevision: 1, Amount: "60", ManualSourceReference: "竞态分配六十", Actor: actor,
		})
		allocationErrors <- err
	}()
	close(start)
	remissionErr := <-remissionErrors
	allocationErr := <-allocationErrors

	successes := 0
	if remissionErr == nil {
		successes++
	} else {
		var conflict *RemissionBalanceConflictError
		if !errors.As(remissionErr, &conflict) || conflict.Outstanding != "40" {
			t.Fatalf("remission race error = %v", remissionErr)
		}
	}
	if allocationErr == nil {
		successes++
	} else {
		var conflict *allocation.ObligationBalanceConflictError
		if !errors.As(allocationErr, &conflict) || conflict.Available != "40" {
			t.Fatalf("allocation race error = %v", allocationErr)
		}
	}
	if successes != 1 {
		t.Fatalf("race successes=%d remission=%v allocation=%v", successes, remissionErr, allocationErr)
	}
	details, found, err := obligationService.GetDetails(ctx, confirmed.ID)
	if err != nil || !found || details.Obligation.EffectiveAmount != "100" ||
		details.Obligation.OutstandingAmount != "40" {
		t.Fatalf("final race projection = %#v, %v, %v", details.Obligation, found, err)
	}
	if remissionErr == nil && (details.Obligation.RemittedAmount != "60" || details.Obligation.AllocatedAmount != "0") {
		t.Fatalf("remission-first projection = %#v", details.Obligation)
	}
	if allocationErr == nil && (details.Obligation.RemittedAmount != "0" || details.Obligation.AllocatedAmount != "60") {
		t.Fatalf("allocation-first projection = %#v", details.Obligation)
	}
}

func TestPostgresDirectObligationRemissionWaitsForAllocationInsert(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "直接减免并发管理员"}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	obligationService := NewService(NewPostgresRepository(environment.Pool))
	confirmed := createConfirmedAdjustmentObligation(t, ctx, obligationService, actor, owner.ID, counterparty.ID, "100")
	cashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	cashResult, err := cashService.Create(ctx, cashtransaction.CreateInput{
		CommandID: "51111111-1111-4111-8111-111111111111", Type: "receipt",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Description: "直接减免竞态收款", Amount: "100", Currency: "CNY",
		OccurredOn: "2026-09-19", AccountReference: "直接减免竞态账户",
		ManualSourceReference: "直接减免竞态回单", ExternalTransactionReference: "DIRECT-REMISSION-RACE", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create direct race cash: %v", err)
	}

	allocationTx, err := environment.Pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin direct allocation transaction: %v", err)
	}
	defer func() { _ = allocationTx.Rollback(context.Background()) }()
	if _, err := allocationTx.Exec(ctx, `SELECT id FROM cash_transactions WHERE id = $1 FOR UPDATE`, cashResult.Transaction.ID); err != nil {
		t.Fatalf("lock cash for direct allocation: %v", err)
	}
	if _, err := allocationTx.Exec(ctx, `SELECT id FROM obligations WHERE id = $1 FOR UPDATE`, confirmed.ID); err != nil {
		t.Fatalf("lock obligation for direct allocation: %v", err)
	}

	type directResult struct {
		backendPID int32
		err        error
	}
	backendPIDs := make(chan int32, 1)
	directResults := make(chan directResult, 1)
	go func() {
		connection, err := environment.Pool.Acquire(ctx)
		if err != nil {
			directResults <- directResult{err: err}
			return
		}
		defer connection.Release()
		var backendPID int32
		if err := connection.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&backendPID); err != nil {
			directResults <- directResult{err: err}
			return
		}
		backendPIDs <- backendPID
		_, err = connection.Exec(ctx, `
			INSERT INTO obligation_remissions (
				id, obligation_id, obligation_revision,
				amount, recorded_effective_amount, currency,
				source_kind, remission_reason, manual_source_reference,
				recorded_by_actor_kind, recorded_by_actor_id,
				recorded_by_actor_display_name, recorded_at
			) VALUES (
				'61111111-1111-4111-8111-111111111111', $1, 1,
				60, 100, 'CNY',
				'manual', '直接写入竞态减免六十', '直接写入竞态减免依据',
				$2, $3, $4, clock_timestamp()
			)
		`, confirmed.ID, actor.Kind, actor.ID, actor.DisplayName)
		directResults <- directResult{backendPID: backendPID, err: err}
	}()

	var directBackendPID int32
	select {
	case directBackendPID = <-backendPIDs:
	case result := <-directResults:
		t.Fatalf("direct remission stopped before waiting: %v", result.err)
	case <-ctx.Done():
		t.Fatalf("wait for direct remission backend: %v", ctx.Err())
	}
	waitForPostgresLock(t, ctx, environment, directBackendPID)

	if _, err := allocationTx.Exec(ctx, `
		INSERT INTO allocations (
			id, cash_transaction_id, obligation_id, obligation_revision,
			amount, currency, source_kind, manual_source_reference,
			applied_by_actor_kind, applied_by_actor_id,
			applied_by_actor_display_name, applied_at
		) VALUES (
			'71111111-1111-4111-8111-111111111111', $1, $2, 1,
			60, 'CNY', 'manual', '直接写入竞态分配六十',
			$3, $4, $5, clock_timestamp()
		)
	`, cashResult.Transaction.ID, confirmed.ID, actor.Kind, actor.ID, actor.DisplayName); err != nil {
		t.Fatalf("insert direct allocation: %v", err)
	}
	if err := allocationTx.Commit(ctx); err != nil {
		t.Fatalf("commit direct allocation: %v", err)
	}

	direct := <-directResults
	var databaseError *pgconn.PgError
	if !errors.As(direct.err, &databaseError) || databaseError.Code != "23514" {
		t.Fatalf("direct remission after allocation commit error = %v", direct.err)
	}
	details, found, err := obligationService.GetDetails(ctx, confirmed.ID)
	if err != nil || !found || details.Obligation.EffectiveAmount != "100" ||
		details.Obligation.AllocatedAmount != "60" || details.Obligation.RemittedAmount != "0" ||
		details.Obligation.OutstandingAmount != "40" || len(details.Remissions) != 0 {
		t.Fatalf("direct race final details = %#v, %v, %v", details, found, err)
	}
}

func TestPostgresObligationRemissionAndAmountAdjustmentSerialize(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "减免调整并发管理员"}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	service := NewService(NewPostgresRepository(environment.Pool))
	confirmed := createConfirmedAdjustmentObligation(t, ctx, service, actor, owner.ID, counterparty.ID, "100")

	start := make(chan struct{})
	remissionErrors := make(chan error, 1)
	adjustmentErrors := make(chan error, 1)
	go func() {
		<-start
		_, err := service.RecordRemission(ctx, RecordRemissionInput{
			CommandID: "51111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
			ExpectedCurrentRevision: 1, Amount: "60", Reason: "竞态减免六十",
			ManualSourceReference: "竞态减免调整依据", Actor: actor,
		})
		remissionErrors <- err
	}()
	go func() {
		<-start
		_, err := service.AdjustAmount(ctx, AdjustAmountInput{
			CommandID: "61111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
			ExpectedCurrentRevision: 1, Type: "decrease", Amount: "50", Reason: "竞态减少至五十", Actor: actor,
		})
		adjustmentErrors <- err
	}()
	close(start)
	remissionErr := <-remissionErrors
	adjustmentErr := <-adjustmentErrors

	successes := 0
	if remissionErr == nil {
		successes++
	} else {
		var conflict *CurrentRevisionConflictError
		if !errors.As(remissionErr, &conflict) || conflict.Expected != 1 || conflict.Actual != 2 {
			t.Fatalf("remission race error = %v", remissionErr)
		}
	}
	if adjustmentErr == nil {
		successes++
	} else {
		var conflict *AmountAdjustmentResultConflictError
		if !errors.As(adjustmentErr, &conflict) || conflict.Reason != "below_allocated_and_remitted" || conflict.RemittedAmount != "60" {
			t.Fatalf("adjustment race error = %v", adjustmentErr)
		}
	}
	if successes != 1 {
		t.Fatalf("race successes=%d remission=%v adjustment=%v", successes, remissionErr, adjustmentErr)
	}
	details, found, err := service.GetDetails(ctx, confirmed.ID)
	if err != nil || !found {
		t.Fatalf("GetDetails() = %#v, %v, %v", details, found, err)
	}
	if remissionErr == nil && (details.Obligation.CurrentRevision != 1 || details.Obligation.EffectiveAmount != "100" ||
		details.Obligation.RemittedAmount != "60" || details.Obligation.OutstandingAmount != "40") {
		t.Fatalf("remission-first projection = %#v", details.Obligation)
	}
	if adjustmentErr == nil && (details.Obligation.CurrentRevision != 2 || details.Obligation.EffectiveAmount != "50" ||
		details.Obligation.RemittedAmount != "0" || details.Obligation.OutstandingAmount != "50") {
		t.Fatalf("adjustment-first projection = %#v", details.Obligation)
	}
}

func TestPostgresObligationRemissionAndAllocationReversalSerialize(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "减免反向并发管理员"}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	obligationService := NewService(NewPostgresRepository(environment.Pool))
	confirmed := createConfirmedAdjustmentObligation(t, ctx, obligationService, actor, owner.ID, counterparty.ID, "100")
	cashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	cashResult, err := cashService.Create(ctx, cashtransaction.CreateInput{
		CommandID: "51111111-1111-4111-8111-111111111111", Type: "receipt",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Description: "减免与反向竞态收款", Amount: "100", Currency: "CNY",
		OccurredOn: "2026-09-19", AccountReference: "减免反向竞态账户",
		ManualSourceReference: "减免反向竞态回单", ExternalTransactionReference: "REMISSION-REVERSE-RACE", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create race cash: %v", err)
	}
	allocationRepository := allocation.NewPostgresRepository(environment.Pool)
	allocationService := allocation.NewService(allocationRepository)
	applied, err := allocationService.Apply(ctx, allocation.ApplyInput{
		CommandID:         "61111111-1111-4111-8111-111111111111",
		CashTransactionID: cashResult.Transaction.ID, ObligationID: confirmed.ID,
		ExpectedObligationRevision: 1, Amount: "60", ManualSourceReference: "反向前分配六十", Actor: actor,
	})
	if err != nil {
		t.Fatalf("apply before race: %v", err)
	}
	var raceAt time.Time
	if err := environment.Pool.QueryRow(ctx, `
		SELECT obligation_latest_recorded_fact_at($1) + interval '1 microsecond'
	`, confirmed.ID).Scan(&raceAt); err != nil {
		t.Fatalf("read legal race time: %v", err)
	}
	obligationService.now = func() time.Time { return raceAt }

	start := make(chan struct{})
	remissionErrors := make(chan error, 1)
	reversalErrors := make(chan error, 1)
	go func() {
		<-start
		_, err := obligationService.RecordRemission(ctx, RecordRemissionInput{
			CommandID: "71111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
			ExpectedCurrentRevision: 1, Amount: "40", Reason: "竞态减免剩余四十",
			ManualSourceReference: "竞态减免反向依据", Actor: actor,
		})
		remissionErrors <- err
	}()
	go func() {
		<-start
		_, _, err := allocationRepository.Reverse(ctx, allocation.Reversal{
			ID: "91111111-1111-4111-8111-111111111111", AllocationID: applied.Allocation.ID,
			SourceKind: allocation.SourceKindManual, Reason: "竞态反向原分配", ReversedBy: actor, ReversedAt: raceAt,
		}, allocation.ReverseCommand{
			CommandID: "81111111-1111-4111-8111-111111111111", PayloadHash: make([]byte, 32),
			Actor: actor, ReversedAt: raceAt,
		}, audit.Entry{
			ID: "a1111111-1111-4111-8111-111111111111", ObjectType: audit.ObjectAllocationReversal,
			ObjectID: "91111111-1111-4111-8111-111111111111", Action: allocation.AuditActionReversed,
			CommandID: "81111111-1111-4111-8111-111111111111", Actor: actor, OccurredAt: raceAt,
			Summary: "完整反向一条资金分配", Details: map[string]any{"reason": "竞态反向原分配"},
		})
		reversalErrors <- err
	}()
	close(start)
	if err := <-remissionErrors; err != nil {
		t.Fatalf("remission race error = %v", err)
	}
	if err := <-reversalErrors; err != nil {
		t.Fatalf("reversal race error = %v", err)
	}
	details, found, err := obligationService.GetDetails(ctx, confirmed.ID)
	if err != nil || !found || details.Obligation.CurrentRevision != 1 || details.Obligation.EffectiveAmount != "100" ||
		details.Obligation.AllocatedAmount != "0" || details.Obligation.RemittedAmount != "40" ||
		details.Obligation.OutstandingAmount != "60" {
		t.Fatalf("final remission/reversal projection = %#v, %v, %v", details.Obligation, found, err)
	}
	allocationValue, found, err := allocationService.Get(ctx, applied.Allocation.ID)
	if err != nil || !found || allocationValue.Status != allocation.StatusReversed {
		t.Fatalf("reversed allocation = %#v, %v, %v", allocationValue, found, err)
	}
}

func assertRemissionFactCounts(
	t *testing.T,
	ctx context.Context,
	environment testdatabase.Environment,
	obligationID string,
	wantRemissions, wantCommands, wantAudits int,
) {
	t.Helper()
	var remissions, commands, audits int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM obligation_remissions WHERE obligation_id = $1),
			(SELECT count(*) FROM obligation_remission_commands WHERE obligation_id = $1),
			(SELECT count(*) FROM audit_entries WHERE object_type = 'obligation' AND object_id = $1 AND action = 'obligation_remission_recorded')
	`, obligationID).Scan(&remissions, &commands, &audits); err != nil {
		t.Fatalf("read remission fact counts: %v", err)
	}
	if remissions != wantRemissions || commands != wantCommands || audits != wantAudits {
		t.Fatalf("remission fact counts remissions=%d commands=%d audits=%d", remissions, commands, audits)
	}
}
