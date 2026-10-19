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

func TestPostgresObligationClosureBasesIdempotencyAndTerminalState(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "关闭管理员",
	}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	service := NewService(NewPostgresRepository(environment.Pool))
	cashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	allocationService := allocation.NewService(allocation.NewPostgresRepository(environment.Pool))

	createCash := func(commandID, description, externalReference string) cashtransaction.Transaction {
		t.Helper()
		result, err := cashService.Create(ctx, cashtransaction.CreateInput{
			CommandID: commandID, Type: "receipt",
			LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
			Description: description, Amount: "100", Currency: "CNY",
			OccurredOn: "2026-09-19", AccountReference: "关闭测试账户",
			ManualSourceReference: description + "回单", ExternalTransactionReference: externalReference, Actor: actor,
		})
		if err != nil {
			t.Fatalf("create cash %s: %v", description, err)
		}
		return result.Transaction
	}
	apply := func(commandID, cashID, obligationID, amount, source string) allocation.Allocation {
		t.Helper()
		result, err := allocationService.Apply(ctx, allocation.ApplyInput{
			CommandID: commandID, CashTransactionID: cashID, ObligationID: obligationID,
			ExpectedObligationRevision: 1, Amount: amount, ManualSourceReference: source, Actor: actor,
		})
		if err != nil {
			t.Fatalf("apply %s: %v", source, err)
		}
		return result.Allocation
	}

	allocatedObligation := createConfirmedAdjustmentObligation(t, ctx, service, actor, owner.ID, counterparty.ID, "100")
	allocatedCash := createCash("51000000-0000-4000-8000-000000000001", "全核销关闭收款", "CLOSE-CASH-ALLOCATED")
	apply("52000000-0000-4000-8000-000000000001", allocatedCash.ID, allocatedObligation.ID, "100", "全额资金分配")
	allocatedClosed, err := service.Close(ctx, CloseInput{
		CommandID: "53000000-0000-4000-8000-000000000001", ObligationID: allocatedObligation.ID,
		ExpectedCurrentRevision: 1, ExpectedBalanceSequence: 1, Reason: "全额收款核对完成", Actor: actor,
	})
	if err != nil || allocatedClosed.Closure.SettlementBasis != ClosureBasisAllocated ||
		allocatedClosed.Obligation.Status != StatusClosed || allocatedClosed.Obligation.OutstandingAmount != "0" {
		t.Fatalf("allocated Close() = %#v, %v", allocatedClosed, err)
	}

	remittedObligation := createConfirmedAdjustmentObligation(t, ctx, service, actor, owner.ID, counterparty.ID, "100")
	_, err = service.RecordRemission(ctx, RecordRemissionInput{
		CommandID: "54000000-0000-4000-8000-000000000001", ObligationID: remittedObligation.ID,
		ExpectedCurrentRevision: 1, Amount: "100", Reason: "确认全部免除",
		ManualSourceReference: "全部减免确认函", Actor: actor,
	})
	if err != nil {
		t.Fatalf("record full remission: %v", err)
	}
	remittedClosed, err := service.Close(ctx, CloseInput{
		CommandID: "55000000-0000-4000-8000-000000000001", ObligationID: remittedObligation.ID,
		ExpectedCurrentRevision: 1, ExpectedBalanceSequence: 1, Reason: "全部减免核对完成", Actor: actor,
	})
	if err != nil || remittedClosed.Closure.SettlementBasis != ClosureBasisRemitted ||
		remittedClosed.Closure.NetAllocatedAmount != "0" || remittedClosed.Closure.RemittedAmount != "100" {
		t.Fatalf("remitted Close() = %#v, %v", remittedClosed, err)
	}

	mixedObligation := createConfirmedAdjustmentObligation(t, ctx, service, actor, owner.ID, counterparty.ID, "100")
	mixedCash := createCash("56000000-0000-4000-8000-000000000001", "混合关闭收款", "CLOSE-CASH-MIXED")
	mixedAllocation := apply("57000000-0000-4000-8000-000000000001", mixedCash.ID, mixedObligation.ID, "40", "混合关闭分配")
	_, err = service.RecordRemission(ctx, RecordRemissionInput{
		CommandID: "58000000-0000-4000-8000-000000000001", ObligationID: mixedObligation.ID,
		ExpectedCurrentRevision: 1, Amount: "60", Reason: "免除剩余部分",
		ManualSourceReference: "混合关闭减免确认函", Actor: actor,
	})
	if err != nil {
		t.Fatalf("record mixed remission: %v", err)
	}
	mixedInput := CloseInput{
		CommandID: "59000000-0000-4000-8000-000000000001", ObligationID: mixedObligation.ID,
		ExpectedCurrentRevision: 1, ExpectedBalanceSequence: 2, Reason: "核销与减免合计归零", Actor: actor,
	}
	mixedClosed, err := service.Close(ctx, mixedInput)
	if err != nil || mixedClosed.Replayed || mixedClosed.Obligation.Status != StatusClosed ||
		mixedClosed.Obligation.CurrentRevision != 1 || mixedClosed.Obligation.EffectiveAmount != "100" ||
		mixedClosed.Obligation.AllocatedAmount != "40" || mixedClosed.Obligation.RemittedAmount != "60" ||
		mixedClosed.Obligation.OutstandingAmount != "0" || mixedClosed.Obligation.BalanceSequence != 2 ||
		mixedClosed.Closure.ObligationRevision != 1 || mixedClosed.Closure.BalanceSequence != 2 ||
		mixedClosed.Closure.RecordedEffectiveAmount != "100" || mixedClosed.Closure.NetAllocatedAmount != "40" ||
		mixedClosed.Closure.RemittedAmount != "60" || mixedClosed.Closure.Currency != "CNY" ||
		mixedClosed.Closure.SettlementBasis != ClosureBasisMixed || mixedClosed.Closure.ClosedBy != actor {
		t.Fatalf("mixed Close() = %#v, %v", mixedClosed, err)
	}
	replayed, err := service.Close(ctx, mixedInput)
	if err != nil || !replayed.Replayed || replayed.Closure.ID != mixedClosed.Closure.ID || replayed.Obligation.Status != StatusClosed {
		t.Fatalf("closure replay = %#v, %v", replayed, err)
	}
	changed := mixedInput
	changed.Reason = "改变关闭原因"
	_, err = service.Close(ctx, changed)
	var idempotencyConflict *ClosureIdempotencyConflictError
	if !errors.As(err, &idempotencyConflict) {
		t.Fatalf("changed closure replay error = %v", err)
	}

	details, found, err := service.GetDetails(ctx, mixedObligation.ID)
	if err != nil || !found || details.Closure == nil || details.Closure.ID != mixedClosed.Closure.ID ||
		details.Obligation.Status != StatusClosed || len(details.Revisions) != 1 || len(details.Remissions) != 1 {
		t.Fatalf("closed GetDetails() = %#v, %v, %v", details, found, err)
	}

	_, err = service.AdjustAmount(ctx, AdjustAmountInput{
		CommandID: "5a000000-0000-4000-8000-000000000001", ObligationID: mixedObligation.ID,
		ExpectedCurrentRevision: 1, Type: "increase", Amount: "1", Reason: "关闭后不应调额", Actor: actor,
	})
	var adjustmentState *AmountAdjustmentStateConflictError
	if !errors.As(err, &adjustmentState) || adjustmentState.Status != StatusClosed {
		t.Fatalf("adjust after closure error = %v", err)
	}
	_, err = service.RecordRemission(ctx, RecordRemissionInput{
		CommandID: "5b000000-0000-4000-8000-000000000001", ObligationID: mixedObligation.ID,
		ExpectedCurrentRevision: 1, Amount: "1", Reason: "关闭后不应减免",
		ManualSourceReference: "关闭后减免测试", Actor: actor,
	})
	var remissionState *RemissionStateConflictError
	if !errors.As(err, &remissionState) || remissionState.Status != StatusClosed {
		t.Fatalf("remission after closure error = %v", err)
	}
	_, err = allocationService.Apply(ctx, allocation.ApplyInput{
		CommandID: "5c000000-0000-4000-8000-000000000001", CashTransactionID: mixedCash.ID,
		ObligationID: mixedObligation.ID, ExpectedObligationRevision: 1, Amount: "1",
		ManualSourceReference: "关闭后不应分配", Actor: actor,
	})
	var allocationState *allocation.ObligationStateConflictError
	if !errors.As(err, &allocationState) || allocationState.Status != StatusClosed {
		t.Fatalf("allocation after closure error = %v", err)
	}
	_, err = allocationService.Reverse(ctx, allocation.ReverseInput{
		CommandID: "5d000000-0000-4000-8000-000000000001", AllocationID: mixedAllocation.ID,
		Reason: "关闭后不应反向", Actor: actor,
	})
	var reversalState *allocation.ReversalObligationStateConflictError
	if !errors.As(err, &reversalState) || reversalState.Status != StatusClosed {
		t.Fatalf("allocation reversal after closure error = %v", err)
	}
	secondClose := mixedInput
	secondClose.CommandID = "5e000000-0000-4000-8000-000000000001"
	_, err = service.Close(ctx, secondClose)
	var closureState *ClosureStateConflictError
	if !errors.As(err, &closureState) || closureState.Status != StatusClosed {
		t.Fatalf("second closure error = %v", err)
	}

	var closures, commands, audits int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM obligation_closures WHERE obligation_id = $1),
			(SELECT count(*) FROM obligation_closure_commands WHERE obligation_id = $1),
			(SELECT count(*) FROM audit_entries WHERE object_type = 'obligation' AND object_id = $1 AND action = 'obligation_closed')
	`, mixedObligation.ID).Scan(&closures, &commands, &audits); err != nil {
		t.Fatalf("read closure fact counts: %v", err)
	}
	if closures != 1 || commands != 1 || audits != 1 {
		t.Fatalf("closure fact counts closures=%d commands=%d audits=%d", closures, commands, audits)
	}
}

func TestPostgresObligationClosureConcurrency(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "关闭并发管理员",
	}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	service := NewService(NewPostgresRepository(environment.Pool))
	cashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	allocationService := allocation.NewService(allocation.NewPostgresRepository(environment.Pool))

	type closeResult struct {
		value CloseResult
		err   error
	}
	runClose := func(inputs []CloseInput) []closeResult {
		t.Helper()
		start := make(chan struct{})
		results := make(chan closeResult, len(inputs))
		var workers sync.WaitGroup
		for _, input := range inputs {
			input := input
			workers.Add(1)
			go func() {
				defer workers.Done()
				<-start
				value, err := service.Close(ctx, input)
				results <- closeResult{value: value, err: err}
			}()
		}
		close(start)
		workers.Wait()
		close(results)
		values := make([]closeResult, 0, len(inputs))
		for result := range results {
			values = append(values, result)
		}
		return values
	}
	remitToZero := func(commandID string, value Obligation) {
		t.Helper()
		_, err := service.RecordRemission(ctx, RecordRemissionInput{
			CommandID: commandID, ObligationID: value.ID, ExpectedCurrentRevision: 1,
			Amount: value.EffectiveAmount, Reason: "并发关闭前确认全部免除",
			ManualSourceReference: "并发关闭减免确认函", Actor: actor,
		})
		if err != nil {
			t.Fatalf("remit obligation to zero: %v", err)
		}
	}
	allocateToZero := func(cashCommandID, allocationCommandID, externalReference string, value Obligation) allocation.Allocation {
		t.Helper()
		cashResult, err := cashService.Create(ctx, cashtransaction.CreateInput{
			CommandID: cashCommandID, Type: "receipt",
			LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
			Description: "关闭并发收款", Amount: value.EffectiveAmount, Currency: "CNY",
			OccurredOn: "2026-09-19", AccountReference: "关闭并发账户",
			ManualSourceReference: "关闭并发回单", ExternalTransactionReference: externalReference, Actor: actor,
		})
		if err != nil {
			t.Fatalf("create closure concurrency cash: %v", err)
		}
		applied, err := allocationService.Apply(ctx, allocation.ApplyInput{
			CommandID: allocationCommandID, CashTransactionID: cashResult.Transaction.ID,
			ObligationID: value.ID, ExpectedObligationRevision: 1,
			Amount: value.EffectiveAmount, ManualSourceReference: "关闭并发资金分配", Actor: actor,
		})
		if err != nil {
			t.Fatalf("allocate obligation to zero: %v", err)
		}
		return applied.Allocation
	}

	sameObligation := createConfirmedAdjustmentObligation(t, ctx, service, actor, owner.ID, counterparty.ID, "100")
	remitToZero("71000000-0000-4000-8000-000000000001", sameObligation)
	sameInput := CloseInput{
		CommandID: "72000000-0000-4000-8000-000000000001", ObligationID: sameObligation.ID,
		ExpectedCurrentRevision: 1, ExpectedBalanceSequence: 1, Reason: "同命令并发关闭", Actor: actor,
	}
	sameResults := runClose([]CloseInput{sameInput, sameInput})
	replays := 0
	var sameClosureID string
	for _, result := range sameResults {
		if result.err != nil {
			t.Fatalf("same command concurrent closure error = %v", result.err)
		}
		if sameClosureID == "" {
			sameClosureID = result.value.Closure.ID
		}
		if result.value.Closure.ID != sameClosureID {
			t.Fatalf("same command closure IDs differ: %s and %s", sameClosureID, result.value.Closure.ID)
		}
		if result.value.Replayed {
			replays++
		}
	}
	if replays != 1 {
		t.Fatalf("same command closure replay count = %d", replays)
	}

	differentObligation := createConfirmedAdjustmentObligation(t, ctx, service, actor, owner.ID, counterparty.ID, "100")
	remitToZero("73000000-0000-4000-8000-000000000001", differentObligation)
	left := CloseInput{
		CommandID: "74000000-0000-4000-8000-000000000001", ObligationID: differentObligation.ID,
		ExpectedCurrentRevision: 1, ExpectedBalanceSequence: 1, Reason: "不同命令并发关闭一", Actor: actor,
	}
	right := left
	right.CommandID = "74000000-0000-4000-8000-000000000002"
	right.Reason = "不同命令并发关闭二"
	differentResults := runClose([]CloseInput{left, right})
	successes, stateConflicts := 0, 0
	for _, result := range differentResults {
		if result.err == nil {
			successes++
			continue
		}
		var conflict *ClosureStateConflictError
		if errors.As(result.err, &conflict) && conflict.Status == StatusClosed {
			stateConflicts++
			continue
		}
		t.Fatalf("unexpected different command closure error = %v", result.err)
	}
	if successes != 1 || stateConflicts != 1 {
		t.Fatalf("different command closure results successes=%d state_conflicts=%d", successes, stateConflicts)
	}

	reversalObligation := createConfirmedAdjustmentObligation(t, ctx, service, actor, owner.ID, counterparty.ID, "100")
	reversalAllocation := allocateToZero(
		"75000000-0000-4000-8000-000000000001",
		"76000000-0000-4000-8000-000000000001",
		"CLOSE-RACE-REVERSAL",
		reversalObligation,
	)
	start := make(chan struct{})
	closureErrors := make(chan error, 1)
	reversalErrors := make(chan error, 1)
	go func() {
		<-start
		_, err := service.Close(ctx, CloseInput{
			CommandID: "77000000-0000-4000-8000-000000000001", ObligationID: reversalObligation.ID,
			ExpectedCurrentRevision: 1, ExpectedBalanceSequence: 1, Reason: "与资金分配反向竞争关闭", Actor: actor,
		})
		closureErrors <- err
	}()
	go func() {
		<-start
		_, err := allocationService.Reverse(ctx, allocation.ReverseInput{
			CommandID: "78000000-0000-4000-8000-000000000001", AllocationID: reversalAllocation.ID,
			Reason: "与义务关闭竞争反向", Actor: actor,
		})
		reversalErrors <- err
	}()
	close(start)
	closureErr := <-closureErrors
	reversalErr := <-reversalErrors
	if (closureErr == nil) == (reversalErr == nil) {
		t.Fatalf("closure/reversal race must have one success: closure=%v reversal=%v", closureErr, reversalErr)
	}
	if closureErr == nil {
		var conflict *allocation.ReversalObligationStateConflictError
		if !errors.As(reversalErr, &conflict) || conflict.Status != StatusClosed {
			t.Fatalf("closure-first reversal error = %v", reversalErr)
		}
	} else {
		var conflict *BalanceSequenceConflictError
		if !errors.As(closureErr, &conflict) || conflict.Expected != 1 || conflict.Actual != 2 {
			t.Fatalf("reversal-first closure error = %v", closureErr)
		}
	}
	reversalDetails, found, err := service.GetDetails(ctx, reversalObligation.ID)
	if err != nil || !found {
		t.Fatalf("read closure/reversal race result: found=%v error=%v", found, err)
	}
	if closureErr == nil {
		if reversalDetails.Obligation.Status != StatusClosed || reversalDetails.Obligation.OutstandingAmount != "0" ||
			reversalDetails.Obligation.AllocatedAmount != "100" || reversalDetails.Obligation.BalanceSequence != 1 {
			t.Fatalf("closure-first race projection = %#v", reversalDetails.Obligation)
		}
	} else if reversalDetails.Obligation.Status != StatusOpen || reversalDetails.Obligation.OutstandingAmount != "100" ||
		reversalDetails.Obligation.AllocatedAmount != "0" || reversalDetails.Obligation.BalanceSequence != 2 {
		t.Fatalf("reversal-first race projection = %#v", reversalDetails.Obligation)
	}

	adjustmentObligation := createConfirmedAdjustmentObligation(t, ctx, service, actor, owner.ID, counterparty.ID, "100")
	remitToZero("79000000-0000-4000-8000-000000000001", adjustmentObligation)
	start = make(chan struct{})
	closureErrors = make(chan error, 1)
	adjustmentErrors := make(chan error, 1)
	go func() {
		<-start
		_, err := service.Close(ctx, CloseInput{
			CommandID: "7a000000-0000-4000-8000-000000000001", ObligationID: adjustmentObligation.ID,
			ExpectedCurrentRevision: 1, ExpectedBalanceSequence: 1, Reason: "与调额竞争关闭", Actor: actor,
		})
		closureErrors <- err
	}()
	go func() {
		<-start
		_, err := service.AdjustAmount(ctx, AdjustAmountInput{
			CommandID: "7b000000-0000-4000-8000-000000000001", ObligationID: adjustmentObligation.ID,
			ExpectedCurrentRevision: 1, Type: "increase", Amount: "10", Reason: "与关闭竞争增加十元", Actor: actor,
		})
		adjustmentErrors <- err
	}()
	close(start)
	closureErr = <-closureErrors
	adjustmentErr := <-adjustmentErrors
	if (closureErr == nil) == (adjustmentErr == nil) {
		t.Fatalf("closure/adjustment race must have one success: closure=%v adjustment=%v", closureErr, adjustmentErr)
	}
	if closureErr == nil {
		var conflict *AmountAdjustmentStateConflictError
		if !errors.As(adjustmentErr, &conflict) || conflict.Status != StatusClosed {
			t.Fatalf("closure-first adjustment error = %v", adjustmentErr)
		}
	} else {
		var conflict *CurrentRevisionConflictError
		if !errors.As(closureErr, &conflict) || conflict.Expected != 1 || conflict.Actual != 2 {
			t.Fatalf("adjustment-first closure error = %v", closureErr)
		}
	}
	adjustmentDetails, found, err := service.GetDetails(ctx, adjustmentObligation.ID)
	if err != nil || !found {
		t.Fatalf("read closure/adjustment race result: found=%v error=%v", found, err)
	}
	if closureErr == nil {
		if adjustmentDetails.Obligation.Status != StatusClosed || adjustmentDetails.Obligation.CurrentRevision != 1 ||
			adjustmentDetails.Obligation.EffectiveAmount != "100" || adjustmentDetails.Obligation.OutstandingAmount != "0" {
			t.Fatalf("closure-first adjustment projection = %#v", adjustmentDetails.Obligation)
		}
	} else if adjustmentDetails.Obligation.Status != StatusOpen || adjustmentDetails.Obligation.CurrentRevision != 2 ||
		adjustmentDetails.Obligation.EffectiveAmount != "110" || adjustmentDetails.Obligation.OutstandingAmount != "10" {
		t.Fatalf("adjustment-first closure projection = %#v", adjustmentDetails.Obligation)
	}
}

func TestPostgresObligationClosureRollbackConstraintsAndImmutability(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "关闭约束管理员",
	}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	service := NewService(NewPostgresRepository(environment.Pool))
	repository := NewPostgresRepository(environment.Pool)

	assertPostgresCode := func(label string, err error, code string) {
		t.Helper()
		var databaseError *pgconn.PgError
		if !errors.As(err, &databaseError) || databaseError.Code != code {
			t.Fatalf("%s error = %v", label, err)
		}
	}
	createRemittedZero := func(remissionCommandID string) Obligation {
		t.Helper()
		value := createConfirmedAdjustmentObligation(t, ctx, service, actor, owner.ID, counterparty.ID, "100")
		_, err := service.RecordRemission(ctx, RecordRemissionInput{
			CommandID: remissionCommandID, ObligationID: value.ID, ExpectedCurrentRevision: 1,
			Amount: "100", Reason: "关闭约束测试全部免除",
			ManualSourceReference: "关闭约束测试减免确认函", Actor: actor,
		})
		if err != nil {
			t.Fatalf("remit obligation to zero: %v", err)
		}
		return value
	}

	value := createRemittedZero("81000000-0000-4000-8000-000000000001")
	var existingAuditID string
	if err := environment.Pool.QueryRow(ctx, `SELECT id::text FROM audit_entries ORDER BY occurred_at, id LIMIT 1`).Scan(&existingAuditID); err != nil {
		t.Fatalf("read existing audit ID: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	_, _, _, err := repository.Close(ctx, Closure{
		ID: "82000000-0000-4000-8000-000000000001", ObligationID: value.ID,
		ObligationRevision: 1, BalanceSequence: 1, SourceKind: SourceKindManual,
		Reason: "审计失败应整体回滚", ClosedBy: actor, ClosedAt: now,
	}, ClosureCommand{
		CommandID: "83000000-0000-4000-8000-000000000001", PayloadHash: make([]byte, 32),
		Actor: actor, ClosedAt: now,
	}, audit.Entry{
		ID: existingAuditID, ObjectType: audit.ObjectObligation, ObjectID: value.ID,
		Action: AuditActionClosed, CommandID: "83000000-0000-4000-8000-000000000001",
		Actor: actor, OccurredAt: now, Summary: "该审计写入必须失败",
	})
	assertPostgresCode("closure audit rollback", err, "23505")
	var closures, commands int
	var status string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM obligation_closures WHERE obligation_id = $1),
			(SELECT count(*) FROM obligation_closure_commands WHERE obligation_id = $1),
			(SELECT status FROM obligations WHERE id = $1)
	`, value.ID).Scan(&closures, &commands, &status); err != nil {
		t.Fatalf("read closure rollback result: %v", err)
	}
	if closures != 0 || commands != 0 || status != StatusOpen {
		t.Fatalf("closure audit rollback closures=%d commands=%d status=%s", closures, commands, status)
	}

	_, err = environment.Pool.Exec(ctx, `
		INSERT INTO obligation_closures (
			id, obligation_id, obligation_revision, balance_sequence,
			recorded_effective_amount, net_allocated_amount, remitted_amount,
			currency, settlement_basis, source_kind, closure_reason,
			closed_by_actor_kind, closed_by_actor_id, closed_by_actor_display_name, closed_at
		) VALUES (
			'84000000-0000-4000-8000-000000000001', $1, 1, 1,
			99, 0, 99,
			'CNY', 'remitted', 'manual', '伪造冻结金额',
			$2, $3, $4, $5
		)
	`, value.ID, actor.Kind, actor.ID, actor.DisplayName, now)
	assertPostgresCode("forged closure facts", err, "23514")

	_, err = environment.Pool.Exec(ctx, `
		INSERT INTO obligation_closures (
			id, obligation_id, obligation_revision, balance_sequence,
			recorded_effective_amount, net_allocated_amount, remitted_amount,
			currency, settlement_basis, source_kind, closure_reason,
			closed_by_actor_kind, closed_by_actor_id, closed_by_actor_display_name, closed_at
		) VALUES (
			'85000000-0000-4000-8000-000000000001', $1, 1, 1,
			100, 0, 100,
			'CNY', 'remitted', 'manual', '仅写关闭事实不得提交',
			$2, $3, $4, $5
		)
	`, value.ID, actor.Kind, actor.ID, actor.DisplayName, now)
	assertPostgresCode("closure without closed root", err, "23514")

	_, err = environment.Pool.Exec(ctx, `UPDATE obligations SET status = 'closed', updated_at = $2 WHERE id = $1`, value.ID, now)
	assertPostgresCode("closed root without closure", err, "23514")

	closed, err := service.Close(ctx, CloseInput{
		CommandID: "86000000-0000-4000-8000-000000000001", ObligationID: value.ID,
		ExpectedCurrentRevision: 1, ExpectedBalanceSequence: 1,
		Reason: "完成关闭约束测试", Actor: actor,
	})
	if err != nil {
		t.Fatalf("close after rejected direct writes: %v", err)
	}
	immutableStatements := []string{
		`UPDATE obligation_closures SET closure_reason = closure_reason WHERE id = $1`,
		`DELETE FROM obligation_closures WHERE id = $1`,
		`UPDATE obligation_closure_commands SET actor_display_name = actor_display_name WHERE obligation_closure_id = $1`,
		`DELETE FROM obligation_closure_commands WHERE obligation_closure_id = $1`,
	}
	for _, statement := range immutableStatements {
		_, err := environment.Pool.Exec(ctx, statement, closed.Closure.ID)
		assertPostgresCode("immutable closure statement", err, "55000")
	}
	_, err = environment.Pool.Exec(ctx, `UPDATE obligations SET status = 'open' WHERE id = $1`, value.ID)
	assertPostgresCode("reopen closed obligation", err, "55000")

	allocated := createConfirmedAdjustmentObligation(t, ctx, service, actor, owner.ID, counterparty.ID, "100")
	cashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	cashResult, err := cashService.Create(ctx, cashtransaction.CreateInput{
		CommandID: "87000000-0000-4000-8000-000000000001", Type: "receipt",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Description: "关闭后反向约束收款", Amount: "100", Currency: "CNY",
		OccurredOn: "2026-09-19", AccountReference: "关闭后反向约束账户",
		ManualSourceReference: "关闭后反向约束回单", ExternalTransactionReference: "CLOSE-DIRECT-REVERSAL", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create direct reversal cash: %v", err)
	}
	allocationService := allocation.NewService(allocation.NewPostgresRepository(environment.Pool))
	applied, err := allocationService.Apply(ctx, allocation.ApplyInput{
		CommandID: "88000000-0000-4000-8000-000000000001", CashTransactionID: cashResult.Transaction.ID,
		ObligationID: allocated.ID, ExpectedObligationRevision: 1, Amount: "100",
		ManualSourceReference: "关闭后反向约束分配", Actor: actor,
	})
	if err != nil {
		t.Fatalf("apply direct reversal allocation: %v", err)
	}
	_, err = service.Close(ctx, CloseInput{
		CommandID: "89000000-0000-4000-8000-000000000001", ObligationID: allocated.ID,
		ExpectedCurrentRevision: 1, ExpectedBalanceSequence: 1,
		Reason: "关闭后禁止直接反向", Actor: actor,
	})
	if err != nil {
		t.Fatalf("close allocated obligation: %v", err)
	}
	_, err = environment.Pool.Exec(ctx, `
		INSERT INTO allocation_reversals (
			id, allocation_id, cash_transaction_id, obligation_id, obligation_revision,
			amount, currency, source_kind, reversal_reason,
			reversed_by_actor_kind, reversed_by_actor_id, reversed_by_actor_display_name, reversed_at
		) VALUES (
			'8a000000-0000-4000-8000-000000000001', $1, $2, $3, 1,
			100, 'CNY', 'manual', '直接写入也不得反向已关闭义务',
			$4, $5, $6, $7
		)
	`, applied.Allocation.ID, cashResult.Transaction.ID, allocated.ID, actor.Kind, actor.ID, actor.DisplayName, now)
	assertPostgresCode("direct reversal after closure", err, "23514")
}

func TestPostgresDirectObligationClosureWaitsForAllocationReversal(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "直接关闭并发管理员",
	}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	service := NewService(NewPostgresRepository(environment.Pool))
	value := createConfirmedAdjustmentObligation(t, ctx, service, actor, owner.ID, counterparty.ID, "100")
	cashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	cashResult, err := cashService.Create(ctx, cashtransaction.CreateInput{
		CommandID: "8b000000-0000-4000-8000-000000000001", Type: "receipt",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Description: "直接关闭竞态收款", Amount: "100", Currency: "CNY",
		OccurredOn: "2026-09-19", AccountReference: "直接关闭竞态账户",
		ManualSourceReference: "直接关闭竞态回单", ExternalTransactionReference: "DIRECT-CLOSE-RACE", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create direct closure race cash: %v", err)
	}
	allocationService := allocation.NewService(allocation.NewPostgresRepository(environment.Pool))
	applied, err := allocationService.Apply(ctx, allocation.ApplyInput{
		CommandID: "8c000000-0000-4000-8000-000000000001", CashTransactionID: cashResult.Transaction.ID,
		ObligationID: value.ID, ExpectedObligationRevision: 1, Amount: "100",
		ManualSourceReference: "直接关闭竞态分配", Actor: actor,
	})
	if err != nil {
		t.Fatalf("apply direct closure race allocation: %v", err)
	}

	reversalTx, err := environment.Pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin direct reversal transaction: %v", err)
	}
	defer func() { _ = reversalTx.Rollback(context.Background()) }()
	if _, err := reversalTx.Exec(ctx, `SELECT id FROM cash_transactions WHERE id = $1 FOR UPDATE`, cashResult.Transaction.ID); err != nil {
		t.Fatalf("lock cash for direct reversal: %v", err)
	}
	if _, err := reversalTx.Exec(ctx, `SELECT id FROM obligations WHERE id = $1 FOR UPDATE`, value.ID); err != nil {
		t.Fatalf("lock obligation for direct reversal: %v", err)
	}

	type directResult struct {
		backendPID int32
		err        error
	}
	backendPIDs := make(chan int32, 1)
	directResults := make(chan directResult, 1)
	now := time.Now().UTC().Truncate(time.Microsecond)
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
			INSERT INTO obligation_closures (
				id, obligation_id, obligation_revision, balance_sequence,
				recorded_effective_amount, net_allocated_amount, remitted_amount,
				currency, settlement_basis, source_kind, closure_reason,
				closed_by_actor_kind, closed_by_actor_id, closed_by_actor_display_name, closed_at
			) VALUES (
				'8d000000-0000-4000-8000-000000000001', $1, 1, 1,
				100, 100, 0,
				'CNY', 'allocated', 'manual', '直接关闭必须等待余额事实',
				$2, $3, $4, $5
			)
		`, value.ID, actor.Kind, actor.ID, actor.DisplayName, now)
		directResults <- directResult{backendPID: backendPID, err: err}
	}()

	var directBackendPID int32
	select {
	case directBackendPID = <-backendPIDs:
	case result := <-directResults:
		t.Fatalf("direct closure stopped before waiting: %v", result.err)
	case <-ctx.Done():
		t.Fatalf("wait for direct closure backend: %v", ctx.Err())
	}
	waitForPostgresLock(t, ctx, environment, directBackendPID)

	if _, err := reversalTx.Exec(ctx, `
		INSERT INTO allocation_reversals (
			id, allocation_id, cash_transaction_id, obligation_id, obligation_revision,
			amount, currency, source_kind, reversal_reason,
			reversed_by_actor_kind, reversed_by_actor_id, reversed_by_actor_display_name, reversed_at
		) VALUES (
			'8e000000-0000-4000-8000-000000000001', $1, $2, $3, 1,
			100, 'CNY', 'manual', '先提交资金分配反向',
			$4, $5, $6, $7
		)
	`, applied.Allocation.ID, cashResult.Transaction.ID, value.ID, actor.Kind, actor.ID, actor.DisplayName, now); err != nil {
		t.Fatalf("insert direct allocation reversal: %v", err)
	}
	if err := reversalTx.Commit(ctx); err != nil {
		t.Fatalf("commit direct allocation reversal: %v", err)
	}

	direct := <-directResults
	var databaseError *pgconn.PgError
	if !errors.As(direct.err, &databaseError) || databaseError.Code != "23514" {
		t.Fatalf("direct closure after reversal commit error = %v", direct.err)
	}
	details, found, err := service.GetDetails(ctx, value.ID)
	if err != nil || !found || details.Obligation.Status != StatusOpen ||
		details.Obligation.AllocatedAmount != "0" || details.Obligation.OutstandingAmount != "100" ||
		details.Obligation.BalanceSequence != 2 || details.Closure != nil {
		t.Fatalf("direct closure race final details = %#v, closure=%#v, found=%v, error=%v", details.Obligation, details.Closure, found, err)
	}
}

func TestPostgresObligationClosureRejectsNonZeroAndStaleBalanceSequence(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "关闭冲突管理员",
	}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	service := NewService(NewPostgresRepository(environment.Pool))
	cashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	allocationService := allocation.NewService(allocation.NewPostgresRepository(environment.Pool))

	value := createConfirmedAdjustmentObligation(t, ctx, service, actor, owner.ID, counterparty.ID, "100")
	cashResult, err := cashService.Create(ctx, cashtransaction.CreateInput{
		CommandID: "61000000-0000-4000-8000-000000000001", Type: "receipt",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Description: "关闭冲突收款", Amount: "100", Currency: "CNY", OccurredOn: "2026-09-19",
		AccountReference: "关闭冲突账户", ManualSourceReference: "关闭冲突回单",
		ExternalTransactionReference: "CLOSE-CONFLICT-CASH", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create cash: %v", err)
	}
	first, err := allocationService.Apply(ctx, allocation.ApplyInput{
		CommandID: "62000000-0000-4000-8000-000000000001", CashTransactionID: cashResult.Transaction.ID,
		ObligationID: value.ID, ExpectedObligationRevision: 1, Amount: "50",
		ManualSourceReference: "先部分分配", Actor: actor,
	})
	if err != nil {
		t.Fatalf("apply partial allocation: %v", err)
	}
	_, err = service.Close(ctx, CloseInput{
		CommandID: "63000000-0000-4000-8000-000000000001", ObligationID: value.ID,
		ExpectedCurrentRevision: 1, ExpectedBalanceSequence: 1, Reason: "余额未归零不应关闭", Actor: actor,
	})
	var balanceConflict *ClosureBalanceConflictError
	if !errors.As(err, &balanceConflict) || balanceConflict.Outstanding != "50" {
		t.Fatalf("non-zero closure error = %#v / %v", balanceConflict, err)
	}
	_, err = allocationService.Apply(ctx, allocation.ApplyInput{
		CommandID: "64000000-0000-4000-8000-000000000001", CashTransactionID: cashResult.Transaction.ID,
		ObligationID: value.ID, ExpectedObligationRevision: 1, Amount: "50",
		ManualSourceReference: "补足分配", Actor: actor,
	})
	if err != nil {
		t.Fatalf("apply remaining allocation: %v", err)
	}
	_, err = allocationService.Reverse(ctx, allocation.ReverseInput{
		CommandID: "65000000-0000-4000-8000-000000000001", AllocationID: first.Allocation.ID,
		Reason: "构造余额事实变化", Actor: actor,
	})
	if err != nil {
		t.Fatalf("reverse first allocation: %v", err)
	}
	_, err = allocationService.Apply(ctx, allocation.ApplyInput{
		CommandID: "66000000-0000-4000-8000-000000000001", CashTransactionID: cashResult.Transaction.ID,
		ObligationID: value.ID, ExpectedObligationRevision: 1, Amount: "50",
		ManualSourceReference: "重新补足分配", Actor: actor,
	})
	if err != nil {
		t.Fatalf("reapply after reversal: %v", err)
	}
	_, err = service.Close(ctx, CloseInput{
		CommandID: "67000000-0000-4000-8000-000000000001", ObligationID: value.ID,
		ExpectedCurrentRevision: 1, ExpectedBalanceSequence: 2, Reason: "过期余额序号", Actor: actor,
	})
	var sequenceConflict *BalanceSequenceConflictError
	if !errors.As(err, &sequenceConflict) || sequenceConflict.Expected != 2 || sequenceConflict.Actual != 4 {
		t.Fatalf("stale balance sequence error = %#v / %v", sequenceConflict, err)
	}
	closed, err := service.Close(ctx, CloseInput{
		CommandID: "68000000-0000-4000-8000-000000000001", ObligationID: value.ID,
		ExpectedCurrentRevision: 1, ExpectedBalanceSequence: 4, Reason: "使用最新余额序号关闭", Actor: actor,
	})
	if err != nil || closed.Obligation.Status != StatusClosed || closed.Closure.BalanceSequence != 4 {
		t.Fatalf("close with current sequence = %#v, %v", closed, err)
	}
}
