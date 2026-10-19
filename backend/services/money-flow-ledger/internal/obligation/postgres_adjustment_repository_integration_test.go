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

func TestPostgresObligationAmountAdjustmentRevisionsBalancesAndIdempotency(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "金额调整管理员",
	}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	service := NewService(NewPostgresRepository(environment.Pool))
	confirmed := createConfirmedAdjustmentObligation(t, ctx, service, actor, owner.ID, counterparty.ID, "100")

	firstInput := AdjustAmountInput{
		CommandID: "51111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
		ExpectedCurrentRevision: 1, Type: "increase", Amount: "20.000000",
		Reason: "补充确认追加服务范围", Actor: actor,
	}
	first, err := service.AdjustAmount(ctx, firstInput)
	if err != nil || first.Replayed {
		t.Fatalf("first AdjustAmount() = %#v, %v", first, err)
	}
	if first.Obligation.OriginalAmount != "100" || first.Obligation.EffectiveAmount != "120" ||
		first.Obligation.CurrentRevision != 2 || first.Obligation.AllocatedAmount != "0" ||
		first.Obligation.OutstandingAmount != "120" || first.Obligation.Status != StatusOpen ||
		first.Adjustment.SourceRevision != 1 || first.Adjustment.ResultingRevision != 2 ||
		first.Adjustment.PreviousEffectiveAmount != "100" || first.Adjustment.ResultingEffectiveAmount != "120" ||
		first.Revision.RevisionKind != RevisionKindAmountAdjustment || first.Revision.SourceRevision != 1 ||
		first.Revision.EffectiveAmount != "120" || first.Revision.RecordedBy != actor {
		t.Fatalf("first adjustment = %#v", first)
	}

	replayed, err := service.AdjustAmount(ctx, firstInput)
	if err != nil || !replayed.Replayed || replayed.Adjustment.ID != first.Adjustment.ID || replayed.Revision.Revision != 2 {
		t.Fatalf("adjustment replay = %#v, %v", replayed, err)
	}
	changed := firstInput
	changed.Amount = "21"
	_, err = service.AdjustAmount(ctx, changed)
	var idempotencyConflict *AmountAdjustmentIdempotencyConflictError
	if !errors.As(err, &idempotencyConflict) {
		t.Fatalf("changed adjustment replay error = %v", err)
	}

	cashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	cashResult, err := cashService.Create(ctx, cashtransaction.CreateInput{
		CommandID: "61111111-1111-4111-8111-111111111111", Type: "receipt",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Description: "金额调整测试收款", Amount: "120", Currency: "CNY",
		OccurredOn: "2026-09-19", AccountReference: "金额调整测试账户",
		ManualSourceReference: "金额调整测试回单", ExternalTransactionReference: "ADJUST-CASH-001", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create adjustment cash: %v", err)
	}
	allocationService := allocation.NewService(allocation.NewPostgresRepository(environment.Pool))
	firstAllocation, err := allocationService.Apply(ctx, allocation.ApplyInput{
		CommandID:         "71111111-1111-4111-8111-111111111111",
		CashTransactionID: cashResult.Transaction.ID, ObligationID: confirmed.ID,
		ExpectedObligationRevision: 2, Amount: "60", ManualSourceReference: "按 revision 2 分配", Actor: actor,
	})
	if err != nil {
		t.Fatalf("apply revision 2 allocation: %v", err)
	}

	_, err = service.AdjustAmount(ctx, AdjustAmountInput{
		CommandID: "81111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
		ExpectedCurrentRevision: 2, Type: "decrease", Amount: "61",
		Reason: "不应低于已分配净额", Actor: actor,
	})
	var resultConflict *AmountAdjustmentResultConflictError
	if !errors.As(err, &resultConflict) || resultConflict.Reason != "below_allocated" ||
		resultConflict.ResultingEffectiveAmount != "59" || resultConflict.NetAllocatedAmount != "60" {
		t.Fatalf("below allocated error = %#v / %v", resultConflict, err)
	}

	secondInput := AdjustAmountInput{
		CommandID: "91111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
		ExpectedCurrentRevision: 2, Type: "decrease", Amount: "50",
		Reason: "确认部分服务不再发生", Actor: actor,
	}
	second, err := service.AdjustAmount(ctx, secondInput)
	if err != nil || second.Obligation.CurrentRevision != 3 || second.Obligation.EffectiveAmount != "70" ||
		second.Obligation.AllocatedAmount != "60" || second.Obligation.OutstandingAmount != "10" {
		t.Fatalf("second AdjustAmount() = %#v, %v", second, err)
	}

	_, err = allocationService.Apply(ctx, allocation.ApplyInput{
		CommandID:         "a1111111-1111-4111-8111-111111111111",
		CashTransactionID: cashResult.Transaction.ID, ObligationID: confirmed.ID,
		ExpectedObligationRevision: 2, Amount: "1", ManualSourceReference: "过期 revision 分配", Actor: actor,
	})
	var revisionConflict *allocation.ObligationRevisionConflictError
	if !errors.As(err, &revisionConflict) || revisionConflict.Expected != 2 || revisionConflict.Actual != 3 {
		t.Fatalf("stale allocation revision error = %v", err)
	}
	secondAllocation, err := allocationService.Apply(ctx, allocation.ApplyInput{
		CommandID:         "b1111111-1111-4111-8111-111111111111",
		CashTransactionID: cashResult.Transaction.ID, ObligationID: confirmed.ID,
		ExpectedObligationRevision: 3, Amount: "10", ManualSourceReference: "按 revision 3 分配", Actor: actor,
	})
	if err != nil {
		t.Fatalf("apply revision 3 allocation: %v", err)
	}

	details, found, err := service.GetDetails(ctx, confirmed.ID)
	if err != nil || !found || details.Obligation.OriginalAmount != "100" || details.Obligation.EffectiveAmount != "70" ||
		details.Obligation.AllocatedAmount != "70" || details.Obligation.OutstandingAmount != "0" ||
		details.Obligation.Status != StatusOpen || len(details.Revisions) != 3 || len(details.Adjustments) != 2 {
		t.Fatalf("GetDetails() = %#v, %v, %v", details, found, err)
	}
	if details.Revisions[0].RevisionKind != RevisionKindConfirmation || details.Revisions[0].EffectiveAmount != "100" ||
		details.Revisions[1].SourceRevision != 1 || details.Revisions[1].EffectiveAmount != "120" ||
		details.Revisions[2].SourceRevision != 2 || details.Revisions[2].EffectiveAmount != "70" ||
		details.Revisions[2].Direction != details.Revisions[1].Direction ||
		details.Revisions[2].Title != details.Revisions[1].Title ||
		details.Revisions[2].Creditor != details.Revisions[1].Creditor ||
		details.Revisions[2].Debtor != details.Revisions[1].Debtor ||
		details.Revisions[2].DueOn != details.Revisions[1].DueOn ||
		details.Revisions[2].ManualSourceReference != details.Revisions[1].ManualSourceReference {
		t.Fatalf("revision history = %#v", details.Revisions)
	}
	allocations, err := allocationService.ListForObligation(ctx, confirmed.ID)
	if err != nil || len(allocations) != 2 ||
		allocations[0].ID != secondAllocation.Allocation.ID || allocations[0].ObligationRevision != 3 ||
		allocations[1].ID != firstAllocation.Allocation.ID || allocations[1].ObligationRevision != 2 {
		t.Fatalf("allocation history = %#v, %v", allocations, err)
	}

	_, err = service.AdjustAmount(ctx, AdjustAmountInput{
		CommandID: "c1111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
		ExpectedCurrentRevision: 2, Type: "increase", Amount: "1", Reason: "过期调整", Actor: actor,
	})
	var currentRevisionConflict *CurrentRevisionConflictError
	if !errors.As(err, &currentRevisionConflict) || currentRevisionConflict.Expected != 2 || currentRevisionConflict.Actual != 3 {
		t.Fatalf("stale adjustment revision error = %v", err)
	}

	var revisions, adjustments, commands, audits int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM obligation_revisions WHERE obligation_id = $1),
			(SELECT count(*) FROM obligation_amount_adjustments WHERE obligation_id = $1),
			(SELECT count(*) FROM obligation_amount_adjustment_commands WHERE obligation_id = $1),
			(SELECT count(*) FROM audit_entries WHERE object_type = 'obligation' AND object_id = $1 AND action = 'obligation_amount_adjusted')
	`, confirmed.ID).Scan(&revisions, &adjustments, &commands, &audits); err != nil {
		t.Fatalf("read amount adjustment facts: %v", err)
	}
	if revisions != 3 || adjustments != 2 || commands != 2 || audits != 2 {
		t.Fatalf("amount adjustment facts revisions=%d adjustments=%d commands=%d audits=%d", revisions, adjustments, commands, audits)
	}
}

func TestPostgresObligationAmountAdjustmentConcurrency(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "并发调整管理员",
	}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	service := NewService(NewPostgresRepository(environment.Pool))

	type result struct {
		value AdjustAmountResult
		err   error
	}
	run := func(inputs []AdjustAmountInput) []result {
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
				value, err := service.AdjustAmount(ctx, input)
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
	sameInput := AdjustAmountInput{
		CommandID: "51111111-1111-4111-8111-111111111111", ObligationID: same.ID,
		ExpectedCurrentRevision: 1, Type: "increase", Amount: "10", Reason: "同命令并发", Actor: actor,
	}
	sameResults := run([]AdjustAmountInput{sameInput, sameInput})
	replays := 0
	for _, result := range sameResults {
		if result.err != nil || result.value.Revision.Revision != 2 {
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
	differentResults := run([]AdjustAmountInput{
		{CommandID: "61111111-1111-4111-8111-111111111111", ObligationID: different.ID, ExpectedCurrentRevision: 1, Type: "increase", Amount: "10", Reason: "并发一", Actor: actor},
		{CommandID: "71111111-1111-4111-8111-111111111111", ObligationID: different.ID, ExpectedCurrentRevision: 1, Type: "decrease", Amount: "10", Reason: "并发二", Actor: actor},
	})
	successes, revisionConflicts := 0, 0
	for _, result := range differentResults {
		if result.err == nil {
			successes++
			continue
		}
		var conflict *CurrentRevisionConflictError
		if errors.As(result.err, &conflict) {
			revisionConflicts++
			continue
		}
		t.Fatalf("unexpected different command error = %v", result.err)
	}
	if successes != 1 || revisionConflicts != 1 {
		t.Fatalf("different command results successes=%d revision_conflicts=%d", successes, revisionConflicts)
	}
}

func TestPostgresObligationAmountAdjustmentRollsBackAndProtectsFacts(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "调整保护管理员",
	}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	service := NewService(NewPostgresRepository(environment.Pool))
	confirmed := createConfirmedAdjustmentObligation(t, ctx, service, actor, owner.ID, counterparty.ID, "100")
	repository := NewPostgresRepository(environment.Pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	_, _, _, _, err := repository.AdjustAmount(ctx, Adjustment{
		ID: "51111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
		SourceRevision: 1, Type: AdjustmentTypeIncrease, Amount: "10", SourceKind: SourceKindManual,
		Reason: "审计失败应整体回滚", AdjustedBy: actor, AdjustedAt: now,
	}, AdjustmentCommand{
		CommandID: "61111111-1111-4111-8111-111111111111", PayloadHash: make([]byte, 32), Actor: actor, AdjustedAt: now,
	}, audit.Entry{
		ID: "71111111-1111-4111-8111-111111111111", ObjectType: audit.ObjectObligation, ObjectID: confirmed.ID,
		Action: AuditActionAmountAdjusted, CommandID: "61111111-1111-4111-8111-111111111111",
		Actor: actor, OccurredAt: now, Summary: "", Details: map[string]any{},
	})
	if err == nil {
		t.Fatal("AdjustAmount() audit failure error = nil")
	}
	assertAdjustmentFactCounts(t, ctx, environment, confirmed.ID, 1, 0, 0, 0)

	adjusted, err := service.AdjustAmount(ctx, AdjustAmountInput{
		CommandID: "81111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
		ExpectedCurrentRevision: 1, Type: "increase", Amount: "10", Reason: "形成受保护调整", Actor: actor,
	})
	if err != nil {
		t.Fatalf("service.AdjustAmount() error = %v", err)
	}

	for _, statement := range []string{
		`UPDATE obligation_amount_adjustments SET adjustment_reason = 'changed' WHERE obligation_id = $1`,
		`DELETE FROM obligation_amount_adjustments WHERE obligation_id = $1`,
		`UPDATE obligation_amount_adjustment_commands SET actor_display_name = 'changed' WHERE obligation_id = $1`,
		`DELETE FROM obligation_amount_adjustment_commands WHERE obligation_id = $1`,
		`UPDATE obligation_revisions SET effective_amount = 999 WHERE obligation_id = $1 AND revision = 2`,
		`DELETE FROM obligation_revisions WHERE obligation_id = $1 AND revision = 2`,
	} {
		_, err := environment.Pool.Exec(ctx, statement, confirmed.ID)
		var databaseError *pgconn.PgError
		if !errors.As(err, &databaseError) || databaseError.Code != "55000" {
			t.Fatalf("immutable statement error = %v", err)
		}
	}

	invalidStatements := []string{
		`UPDATE obligations SET current_revision = 1 WHERE id = $1`,
		`INSERT INTO obligation_revisions (
			obligation_id, revision, direction, title, ledger_owner_party_id,
			creditor_party_id, creditor_party_revision, creditor_kind, creditor_legal_name,
			debtor_party_id, debtor_party_revision, debtor_kind, debtor_legal_name,
			effective_amount, currency, due_on, source_kind, manual_source_reference,
			source_draft_version, recorded_by_actor_kind, recorded_by_actor_id,
			recorded_by_actor_display_name, recorded_at, revision_kind, source_revision
		) SELECT
			obligation_id, 3, direction, title, ledger_owner_party_id,
			creditor_party_id, creditor_party_revision, creditor_kind, creditor_legal_name,
			debtor_party_id, debtor_party_revision, debtor_kind, debtor_legal_name,
			120, currency, due_on, source_kind, manual_source_reference,
			NULL, recorded_by_actor_kind, recorded_by_actor_id,
			recorded_by_actor_display_name, recorded_at, 'amount_adjustment', 2
		  FROM obligation_revisions WHERE obligation_id = $1 AND revision = 2`,
		`INSERT INTO obligation_amount_adjustments (
			id, obligation_id, source_revision, resulting_revision, adjustment_type,
			adjustment_amount, previous_effective_amount, resulting_effective_amount,
			currency, source_kind, adjustment_reason,
			adjusted_by_actor_kind, adjusted_by_actor_id, adjusted_by_actor_display_name, adjusted_at
		 ) VALUES (
			'91111111-1111-4111-8111-111111111111', $1, 2, 3, 'decrease',
			20, 999, 979, 'CNY', 'manual', '伪造前值',
			'local_user', '11111111-1111-4111-8111-111111111111', '调整保护管理员', clock_timestamp()
		 )`,
	}
	for _, statement := range invalidStatements {
		_, err := environment.Pool.Exec(ctx, statement, confirmed.ID)
		var databaseError *pgconn.PgError
		if !errors.As(err, &databaseError) || databaseError.Code != "23514" {
			t.Fatalf("protected statement error = %v", err)
		}
	}
	assertAdjustmentFactCounts(t, ctx, environment, confirmed.ID, 2, 1, 1, 1)
	if adjusted.Obligation.CurrentRevision != 2 {
		t.Fatalf("adjusted obligation = %#v", adjusted.Obligation)
	}
}

func TestPostgresObligationAmountAdjustmentAndAllocationApplySerialize(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "调整分配并发管理员",
	}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	obligationService := NewService(NewPostgresRepository(environment.Pool))
	confirmed := createConfirmedAdjustmentObligation(t, ctx, obligationService, actor, owner.ID, counterparty.ID, "100")
	cashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	cashResult, err := cashService.Create(ctx, cashtransaction.CreateInput{
		CommandID: "51111111-1111-4111-8111-111111111111", Type: "receipt",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Description: "调整与分配竞态收款", Amount: "100", Currency: "CNY",
		OccurredOn: "2026-09-19", AccountReference: "调整分配竞态账户",
		ManualSourceReference: "调整分配竞态回单", ExternalTransactionReference: "ADJUST-APPLY-RACE", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create race cash: %v", err)
	}
	allocationService := allocation.NewService(allocation.NewPostgresRepository(environment.Pool))

	start := make(chan struct{})
	adjustmentErrors := make(chan error, 1)
	allocationErrors := make(chan error, 1)
	go func() {
		<-start
		_, err := obligationService.AdjustAmount(ctx, AdjustAmountInput{
			CommandID: "61111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
			ExpectedCurrentRevision: 1, Type: "decrease", Amount: "50", Reason: "竞态减少至五十", Actor: actor,
		})
		adjustmentErrors <- err
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
	adjustmentErr := <-adjustmentErrors
	allocationErr := <-allocationErrors

	successes := 0
	if adjustmentErr == nil {
		successes++
	} else {
		var conflict *AmountAdjustmentResultConflictError
		if !errors.As(adjustmentErr, &conflict) || conflict.Reason != "below_allocated" {
			t.Fatalf("adjustment race error = %v", adjustmentErr)
		}
	}
	if allocationErr == nil {
		successes++
	} else {
		var conflict *allocation.ObligationRevisionConflictError
		if !errors.As(allocationErr, &conflict) || conflict.Expected != 1 || conflict.Actual != 2 {
			t.Fatalf("allocation race error = %v", allocationErr)
		}
	}
	if successes != 1 {
		t.Fatalf("race successes=%d adjustment=%v allocation=%v", successes, adjustmentErr, allocationErr)
	}

	details, found, err := obligationService.GetDetails(ctx, confirmed.ID)
	if err != nil || !found {
		t.Fatalf("GetDetails() = %#v, %v, %v", details, found, err)
	}
	comparison, err := compareDecimalAmounts(details.Obligation.EffectiveAmount, details.Obligation.AllocatedAmount)
	if err != nil || comparison < 0 || details.Obligation.OutstandingAmount == "" {
		t.Fatalf("final race projection = %#v, comparison=%d, error=%v", details.Obligation, comparison, err)
	}
	if adjustmentErr == nil && (details.Obligation.CurrentRevision != 2 || details.Obligation.EffectiveAmount != "50" || details.Obligation.AllocatedAmount != "0") {
		t.Fatalf("adjustment-first projection = %#v", details.Obligation)
	}
	if allocationErr == nil && (details.Obligation.CurrentRevision != 1 || details.Obligation.EffectiveAmount != "100" || details.Obligation.AllocatedAmount != "60") {
		t.Fatalf("allocation-first projection = %#v", details.Obligation)
	}
}

func TestPostgresDirectObligationAmountAdjustmentWaitsForAllocationInsert(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "直接写入并发管理员",
	}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	obligationService := NewService(NewPostgresRepository(environment.Pool))
	confirmed := createConfirmedAdjustmentObligation(t, ctx, obligationService, actor, owner.ID, counterparty.ID, "100")
	cashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	cashResult, err := cashService.Create(ctx, cashtransaction.CreateInput{
		CommandID: "51111111-1111-4111-8111-111111111111", Type: "receipt",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Description: "直接写入调整竞态收款", Amount: "100", Currency: "CNY",
		OccurredOn: "2026-09-19", AccountReference: "直接写入调整竞态账户",
		ManualSourceReference: "直接写入调整竞态回单", ExternalTransactionReference: "DIRECT-ADJUST-RACE", Actor: actor,
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
	directAdjustedAt := time.Now().UTC().Truncate(time.Microsecond)
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
		tx, err := connection.Begin(ctx)
		if err != nil {
			directResults <- directResult{backendPID: backendPID, err: err}
			return
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		_, err = tx.Exec(ctx, `
			INSERT INTO obligation_amount_adjustments (
				id, obligation_id, source_revision, resulting_revision,
				adjustment_type, adjustment_amount,
				previous_effective_amount, resulting_effective_amount,
				currency, source_kind, adjustment_reason,
				adjusted_by_actor_kind, adjusted_by_actor_id,
				adjusted_by_actor_display_name, adjusted_at
			)
			VALUES (
				'61111111-1111-4111-8111-111111111111', $1, 1, 2,
				'decrease', 50,
				100, 50,
				'CNY', 'manual', '直接写入竞态减少至五十',
				$2, $3, $4, $5
			)
		`, confirmed.ID, actor.Kind, actor.ID, actor.DisplayName, directAdjustedAt)
		if err == nil {
			_, err = tx.Exec(ctx, `
				INSERT INTO obligation_revisions (
					obligation_id, revision, direction, title, ledger_owner_party_id,
					creditor_party_id, creditor_party_revision, creditor_kind,
					creditor_legal_name, creditor_registration_identifier,
					debtor_party_id, debtor_party_revision, debtor_kind,
					debtor_legal_name, debtor_registration_identifier,
					effective_amount, currency, due_on,
					source_kind, manual_source_reference, source_draft_version,
					recorded_by_actor_kind, recorded_by_actor_id,
					recorded_by_actor_display_name, recorded_at,
					revision_kind, source_revision
				)
				SELECT
					obligation_id, 2, direction, title, ledger_owner_party_id,
					creditor_party_id, creditor_party_revision, creditor_kind,
					creditor_legal_name, creditor_registration_identifier,
					debtor_party_id, debtor_party_revision, debtor_kind,
					debtor_legal_name, debtor_registration_identifier,
					50, currency, due_on,
					source_kind, manual_source_reference, NULL,
					$2, $3, $4, $5,
					'amount_adjustment', 1
				FROM obligation_revisions
				WHERE obligation_id = $1 AND revision = 1
			`, confirmed.ID, actor.Kind, actor.ID, actor.DisplayName, directAdjustedAt)
		}
		if err == nil {
			_, err = tx.Exec(ctx, `
				UPDATE obligations
				SET current_revision = 2, updated_at = $2
				WHERE id = $1 AND current_revision = 1
			`, confirmed.ID, directAdjustedAt)
		}
		if err == nil {
			err = tx.Commit(ctx)
		}
		directResults <- directResult{backendPID: backendPID, err: err}
	}()

	var directBackendPID int32
	select {
	case directBackendPID = <-backendPIDs:
	case result := <-directResults:
		t.Fatalf("direct adjustment stopped before waiting: %v", result.err)
	case <-ctx.Done():
		t.Fatalf("wait for direct adjustment backend: %v", ctx.Err())
	}
	waitForPostgresLock(t, ctx, environment, directBackendPID)

	if _, err := allocationTx.Exec(ctx, `
		INSERT INTO allocations (
			id, cash_transaction_id, obligation_id, obligation_revision,
			amount, currency, source_kind, manual_source_reference,
			applied_by_actor_kind, applied_by_actor_id,
			applied_by_actor_display_name, applied_at
		)
		VALUES (
			'71111111-1111-4111-8111-111111111111', $1, $2, 1,
			60, 'CNY', 'manual', '直接写入竞态分配六十',
			$3, $4, $5, $6
		)
	`, cashResult.Transaction.ID, confirmed.ID, actor.Kind, actor.ID, actor.DisplayName, directAdjustedAt); err != nil {
		t.Fatalf("insert direct allocation: %v", err)
	}
	if err := allocationTx.Commit(ctx); err != nil {
		t.Fatalf("commit direct allocation: %v", err)
	}

	direct := <-directResults
	var databaseError *pgconn.PgError
	if !errors.As(direct.err, &databaseError) || databaseError.Code != "23514" {
		t.Fatalf("direct adjustment after allocation commit error = %v", direct.err)
	}
	details, found, err := obligationService.GetDetails(ctx, confirmed.ID)
	if err != nil || !found || details.Obligation.CurrentRevision != 1 ||
		details.Obligation.EffectiveAmount != "100" || details.Obligation.AllocatedAmount != "60" ||
		details.Obligation.OutstandingAmount != "40" || len(details.Adjustments) != 0 {
		t.Fatalf("direct race final details = %#v, %v, %v", details, found, err)
	}
}

func TestPostgresObligationAmountAdjustmentAndAllocationReversalSerialize(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "调整反向并发管理员",
	}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	obligationService := NewService(NewPostgresRepository(environment.Pool))
	confirmed := createConfirmedAdjustmentObligation(t, ctx, obligationService, actor, owner.ID, counterparty.ID, "100")
	cashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	cashResult, err := cashService.Create(ctx, cashtransaction.CreateInput{
		CommandID: "51111111-1111-4111-8111-111111111111", Type: "receipt",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Description: "调整与反向竞态收款", Amount: "100", Currency: "CNY",
		OccurredOn: "2026-09-19", AccountReference: "调整反向竞态账户",
		ManualSourceReference: "调整反向竞态回单", ExternalTransactionReference: "ADJUST-REVERSE-RACE", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create race cash: %v", err)
	}
	allocationService := allocation.NewService(allocation.NewPostgresRepository(environment.Pool))
	applied, err := allocationService.Apply(ctx, allocation.ApplyInput{
		CommandID:         "61111111-1111-4111-8111-111111111111",
		CashTransactionID: cashResult.Transaction.ID, ObligationID: confirmed.ID,
		ExpectedObligationRevision: 1, Amount: "60", ManualSourceReference: "反向前分配六十", Actor: actor,
	})
	if err != nil {
		t.Fatalf("apply before race: %v", err)
	}

	start := make(chan struct{})
	adjustmentErrors := make(chan error, 1)
	reversalErrors := make(chan error, 1)
	go func() {
		<-start
		_, err := obligationService.AdjustAmount(ctx, AdjustAmountInput{
			CommandID: "71111111-1111-4111-8111-111111111111", ObligationID: confirmed.ID,
			ExpectedCurrentRevision: 1, Type: "decrease", Amount: "40", Reason: "竞态减少至已分配额", Actor: actor,
		})
		adjustmentErrors <- err
	}()
	go func() {
		<-start
		_, err := allocationService.Reverse(ctx, allocation.ReverseInput{
			CommandID: "81111111-1111-4111-8111-111111111111", AllocationID: applied.Allocation.ID,
			Reason: "竞态反向原分配", Actor: actor,
		})
		reversalErrors <- err
	}()
	close(start)
	if err := <-adjustmentErrors; err != nil {
		t.Fatalf("adjustment race error = %v", err)
	}
	if err := <-reversalErrors; err != nil {
		t.Fatalf("reversal race error = %v", err)
	}

	details, found, err := obligationService.GetDetails(ctx, confirmed.ID)
	if err != nil || !found || details.Obligation.CurrentRevision != 2 || details.Obligation.EffectiveAmount != "60" ||
		details.Obligation.AllocatedAmount != "0" || details.Obligation.OutstandingAmount != "60" {
		t.Fatalf("final adjustment/reversal projection = %#v, %v, %v", details.Obligation, found, err)
	}
	allocationValue, found, err := allocationService.Get(ctx, applied.Allocation.ID)
	if err != nil || !found || allocationValue.Status != allocation.StatusReversed || allocationValue.ObligationRevision != 1 {
		t.Fatalf("reversed historical allocation = %#v, %v, %v", allocationValue, found, err)
	}
}

func createConfirmedAdjustmentObligation(
	t *testing.T,
	ctx context.Context,
	service *Service,
	actor audit.ActorRef,
	ownerID string,
	counterpartyID string,
	amount string,
) Obligation {
	t.Helper()
	draftCommand, err := NewCommandID()
	if err != nil {
		t.Fatal(err)
	}
	confirmCommand, err := NewCommandID()
	if err != nil {
		t.Fatal(err)
	}
	draft, err := service.CreateDraft(ctx, CreateDraftInput{
		CommandID: draftCommand, Direction: "receivable",
		LedgerOwnerPartyID: ownerID, CounterpartyPartyID: counterpartyID,
		Title: "可调整服务费", OriginalAmount: amount, Currency: "CNY",
		DueOn: "2026-12-31", ManualSourceReference: "金额调整测试义务依据", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create adjustment obligation draft: %v", err)
	}
	confirmed, err := service.Confirm(ctx, ConfirmInput{
		CommandID: confirmCommand, ObligationID: draft.Obligation.ID,
		ExpectedDraftVersion: draft.Obligation.DraftVersion, Actor: actor,
	})
	if err != nil {
		t.Fatalf("confirm adjustment obligation: %v", err)
	}
	return confirmed.Obligation
}

func assertAdjustmentFactCounts(
	t *testing.T,
	ctx context.Context,
	environment testdatabase.Environment,
	obligationID string,
	wantRevisions, wantAdjustments, wantCommands, wantAudits int,
) {
	t.Helper()
	var revisions, adjustments, commands, audits int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM obligation_revisions WHERE obligation_id = $1),
			(SELECT count(*) FROM obligation_amount_adjustments WHERE obligation_id = $1),
			(SELECT count(*) FROM obligation_amount_adjustment_commands WHERE obligation_id = $1),
			(SELECT count(*) FROM audit_entries WHERE object_type = 'obligation' AND object_id = $1 AND action = 'obligation_amount_adjusted')
	`, obligationID).Scan(&revisions, &adjustments, &commands, &audits); err != nil {
		t.Fatalf("read adjustment fact counts: %v", err)
	}
	if revisions != wantRevisions || adjustments != wantAdjustments || commands != wantCommands || audits != wantAudits {
		t.Fatalf("adjustment fact counts revisions=%d adjustments=%d commands=%d audits=%d", revisions, adjustments, commands, audits)
	}
}

func waitForPostgresLock(
	t *testing.T,
	ctx context.Context,
	environment testdatabase.Environment,
	backendPID int32,
) {
	t.Helper()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		err := environment.Pool.QueryRow(ctx, `
			SELECT COALESCE(wait_event_type = 'Lock', false)
			FROM pg_stat_activity
			WHERE pid = $1
		`, backendPID).Scan(&waiting)
		if err != nil {
			t.Fatalf("read direct adjustment backend state: %v", err)
		}
		if waiting {
			return
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatalf("direct adjustment did not wait for obligation lock: %v", ctx.Err())
		}
	}
}
