//go:build integration

package cashtransaction

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"omsbizfoundry/money-flow-ledger/internal/allocation"
	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/database"
	"omsbizfoundry/money-flow-ledger/internal/obligation"
	"omsbizfoundry/money-flow-ledger/internal/party"
	"omsbizfoundry/money-flow-ledger/internal/testdatabase"
)

func TestPostgresCashTransactionSnapshotsIdempotencyAndExternalReference(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "资金联调管理员",
	}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	service := NewService(NewPostgresRepository(environment.Pool))
	firstTime := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return firstTime }

	receiptInput := CreateInput{
		CommandID: "44444444-4444-4444-8444-444444444444", Type: " receipt ",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Description: " 九月服务费收款 ", Amount: "000100.500000", Currency: " cny ",
		OccurredOn: "2026-09-19", AccountReference: " 基本户 ABC ",
		ManualSourceReference: " 银行回单截图 CASH-001 ", ExternalTransactionReference: " Bank-Txn-001 ", Actor: actor,
	}
	receipt, err := service.Create(ctx, receiptInput)
	if err != nil || receipt.Replayed {
		t.Fatalf("create receipt = %#v, %v", receipt, err)
	}
	value := receipt.Transaction
	if value.Type != TypeReceipt || value.Payer.PartyID != counterparty.ID || value.Payee.PartyID != owner.ID ||
		value.Payer.LegalName != "Counter Person" || value.Payer.RegistrationIdentifier != "" ||
		value.Payee.LegalName != "Owner Company" || value.Payee.RegistrationIdentifier != "OWNER-001" ||
		value.Payer.PartyRevision != 1 || value.Payee.PartyRevision != 1 || value.Amount != "100.5" ||
		value.Status != StatusRecorded || value.RecordedBy != actor {
		t.Fatalf("receipt = %#v", value)
	}

	secondTime := firstTime.Add(time.Minute)
	service.now = func() time.Time { return secondTime }
	payment, err := service.Create(ctx, CreateInput{
		CommandID: "55555555-5555-4555-8555-555555555555", Type: "payment",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Description: "代办费用付款", Amount: "0.000001", Currency: "USD",
		OccurredOn: "2026-09-01", AccountReference: "现金账户",
		ManualSourceReference: "历史人工付款依据", ExternalTransactionReference: "PAY-001", Actor: actor,
	})
	if err != nil || payment.Replayed {
		t.Fatalf("create payment = %#v, %v", payment, err)
	}
	if payment.Transaction.Payer.PartyID != owner.ID || payment.Transaction.Payee.PartyID != counterparty.ID ||
		payment.Transaction.Amount != "0.000001" {
		t.Fatalf("payment = %#v", payment.Transaction)
	}
	listed, err := service.List(ctx)
	if err != nil || len(listed) != 2 || listed[0].ID != payment.Transaction.ID || listed[1].ID != receipt.Transaction.ID {
		t.Fatalf("List() = %#v, %v", listed, err)
	}
	got, found, err := service.Get(ctx, receipt.Transaction.ID)
	if err != nil || !found || got.ID != receipt.Transaction.ID {
		t.Fatalf("Get() = %#v, %v, %v", got, found, err)
	}

	if _, err := environment.Pool.Exec(ctx, `
		UPDATE parties
		SET legal_name = 'Owner Company Changed', revision = 2, updated_at = clock_timestamp()
		WHERE id = $1
	`, owner.ID); err != nil {
		t.Fatalf("update current Party: %v", err)
	}
	got, found, err = service.Get(ctx, receipt.Transaction.ID)
	if err != nil || !found || got.Payee.LegalName != "Owner Company" || got.Payee.PartyRevision != 1 {
		t.Fatalf("snapshot after Party update = %#v, %v, %v", got.Payee, found, err)
	}

	replayInput := receiptInput
	replayInput.Type = "RECEIPT"
	replayInput.Description = "九月服务费收款"
	replayInput.Amount = "100.5"
	replayInput.Currency = "CNY"
	replayInput.AccountReference = "基本户 abc"
	replayInput.ManualSourceReference = "银行回单截图 CASH-001"
	replayInput.ExternalTransactionReference = "bank-txn-001"
	replayed, err := service.Create(ctx, replayInput)
	if err != nil || !replayed.Replayed || replayed.Transaction.ID != receipt.Transaction.ID ||
		replayed.Transaction.Payee.LegalName != "Owner Company" || replayed.Transaction.AccountReference != "基本户 ABC" {
		t.Fatalf("replay = %#v, %v", replayed, err)
	}
	changedPayload := replayInput
	changedPayload.Amount = "101"
	_, err = service.Create(ctx, changedPayload)
	var idempotencyConflict *IdempotencyConflictError
	if !errors.As(err, &idempotencyConflict) {
		t.Fatalf("changed payload error = %v", err)
	}
	changedActor := replayInput
	changedActor.Actor.ID = "66666666-6666-4666-8666-666666666666"
	_, err = service.Create(ctx, changedActor)
	if !errors.As(err, &idempotencyConflict) {
		t.Fatalf("changed actor error = %v", err)
	}

	duplicateExternal := replayInput
	duplicateExternal.CommandID = "77777777-7777-4777-8777-777777777777"
	_, err = service.Create(ctx, duplicateExternal)
	var externalConflict *ExternalReferenceConflictError
	if !errors.As(err, &externalConflict) || externalConflict.ExistingTransactionID != receipt.Transaction.ID {
		t.Fatalf("duplicate external reference error = %v", err)
	}

	differentAccount := replayInput
	differentAccount.CommandID = "88888888-8888-4888-8888-888888888888"
	differentAccount.AccountReference = "备用账户"
	differentAccount.Description = "不同账户同流水"
	if _, err := service.Create(ctx, differentAccount); err != nil {
		t.Fatalf("same external reference in different account: %v", err)
	}
	differentOwner := replayInput
	differentOwner.CommandID = "99999999-9999-4999-8999-999999999999"
	differentOwner.LedgerOwnerPartyID = counterparty.ID
	differentOwner.CounterpartyPartyID = owner.ID
	differentOwner.Description = "不同账簿主体同流水"
	if _, err := service.Create(ctx, differentOwner); err != nil {
		t.Fatalf("same external reference for different ledger owner: %v", err)
	}

	for index, commandID := range []string{
		"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
	} {
		withoutExternal := replayInput
		withoutExternal.CommandID = commandID
		withoutExternal.Description = "无外部流水允许独立登记"
		withoutExternal.ExternalTransactionReference = ""
		withoutExternal.Amount = string(rune('1' + index))
		if _, err := service.Create(ctx, withoutExternal); err != nil {
			t.Fatalf("create without external reference %d: %v", index, err)
		}
	}

	missingCommandID := "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	missingInput := replayInput
	missingInput.CommandID = missingCommandID
	missingInput.CounterpartyPartyID = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	missingInput.ExternalTransactionReference = "MISSING-PARTY"
	_, err = service.Create(ctx, missingInput)
	var partyNotFound *PartyNotFoundError
	if !errors.As(err, &partyNotFound) {
		t.Fatalf("missing party error = %v", err)
	}

	var transactions, commands, audits, missingCommands int
	var auditActorKind, auditActorID, auditActorName string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM cash_transactions),
			(SELECT count(*) FROM cash_transaction_create_commands),
			(SELECT count(*) FROM audit_entries WHERE action = 'cash_transaction_recorded'),
			(SELECT count(*) FROM cash_transaction_create_commands WHERE command_id = $1),
			actor_kind, actor_id, actor_display_name
		FROM audit_entries
		WHERE action = 'cash_transaction_recorded' AND command_id = '44444444-4444-4444-8444-444444444444'
	`, missingCommandID).Scan(
		&transactions, &commands, &audits, &missingCommands,
		&auditActorKind, &auditActorID, &auditActorName,
	); err != nil {
		t.Fatalf("read cash transaction facts: %v", err)
	}
	if transactions != 6 || commands != 6 || audits != 6 || missingCommands != 0 ||
		auditActorKind != actor.Kind || auditActorID != actor.ID || auditActorName != actor.DisplayName {
		t.Fatalf("facts transactions=%d commands=%d audits=%d missing=%d actor=%s/%s/%s",
			transactions, commands, audits, missingCommands, auditActorKind, auditActorID, auditActorName)
	}
}

type cashReversalFixture struct {
	actor             audit.ActorRef
	owner             party.Party
	counterparty      party.Party
	transaction       Transaction
	obligation        obligation.Obligation
	cashService       *Service
	allocationService *allocation.Service
}

func setupCashReversalFixture(
	t *testing.T,
	ctx context.Context,
	environment testdatabase.Environment,
	transactionCommandID string,
) cashReversalFixture {
	t.Helper()
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "b1000000-0000-4000-8000-000000000001", DisplayName: "资金冲销管理员",
	}
	owner, counterparty := createTestParties(t, ctx, environment, actor)

	obligationService := obligation.NewService(obligation.NewPostgresRepository(environment.Pool))
	draft, err := obligationService.CreateDraft(ctx, obligation.CreateDraftInput{
		CommandID: "b2000000-0000-4000-8000-000000000001", Direction: "receivable",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Title: "冲销测试义务", OriginalAmount: "100", Currency: "CNY", DueOn: "2026-10-31",
		ManualSourceReference: "冲销测试义务依据", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create reversal fixture obligation: %v", err)
	}
	confirmed, err := obligationService.Confirm(ctx, obligation.ConfirmInput{
		CommandID: "b2000000-0000-4000-8000-000000000002", ObligationID: draft.Obligation.ID,
		ExpectedDraftVersion: draft.Obligation.DraftVersion, Actor: actor,
	})
	if err != nil {
		t.Fatalf("confirm reversal fixture obligation: %v", err)
	}

	cashService := NewService(NewPostgresRepository(environment.Pool))
	created, err := cashService.Create(ctx, CreateInput{
		CommandID: transactionCommandID, Type: "receipt",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Description: "冲销测试收款", Amount: "100", Currency: "CNY", OccurredOn: "2026-09-19",
		AccountReference: "冲销测试账户", ManualSourceReference: "冲销测试回单",
		ExternalTransactionReference: transactionCommandID, Actor: actor,
	})
	if err != nil {
		t.Fatalf("create reversal fixture cash transaction: %v", err)
	}
	return cashReversalFixture{
		actor: actor, owner: owner, counterparty: counterparty,
		transaction: created.Transaction, obligation: confirmed.Obligation,
		cashService: cashService, allocationService: allocation.NewService(allocation.NewPostgresRepository(environment.Pool)),
	}
}

func cashReverseInput(fixture cashReversalFixture, commandID string) ReverseInput {
	return ReverseInput{
		CommandID: commandID, CashTransactionID: fixture.transaction.ID,
		Reason: "原收款重复录入", Actor: fixture.actor,
	}
}

func allocationInput(fixture cashReversalFixture, commandID, amount string) allocation.ApplyInput {
	return allocation.ApplyInput{
		CommandID: commandID, CashTransactionID: fixture.transaction.ID,
		ObligationID: fixture.obligation.ID, ExpectedObligationRevision: fixture.obligation.CurrentRevision,
		Amount: amount, ManualSourceReference: "冲销并发分配", Actor: fixture.actor,
	}
}

func TestPostgresCashTransactionReversalProjectionAndAllocationProtection(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	fixture := setupCashReversalFixture(t, ctx, environment, "b3000000-0000-4000-8000-000000000001")

	result, err := fixture.cashService.Reverse(ctx, cashReverseInput(fixture, "b4000000-0000-4000-8000-000000000001"))
	if err != nil || result.Replayed {
		t.Fatalf("Reverse() = %#v, %v", result, err)
	}
	if result.Reversal.CashTransactionID != fixture.transaction.ID || result.Reversal.TransactionType != TypeReceipt ||
		result.Reversal.Amount != "100" || result.Reversal.Currency != "CNY" || result.Reversal.Reason != "原收款重复录入" ||
		result.Reversal.ReversedBy != fixture.actor {
		t.Fatalf("reversal = %#v", result.Reversal)
	}
	got, found, err := fixture.cashService.Get(ctx, fixture.transaction.ID)
	if err != nil || !found || got.Status != StatusReversed || got.Amount != "100" || got.EffectiveAmount != "0" ||
		got.AllocatedAmount != "0" || got.UnallocatedAmount != "0" || got.Reversal == nil || got.Reversal.ID != result.Reversal.ID {
		t.Fatalf("Get() = %#v, %v, %v", got, found, err)
	}
	listed, err := fixture.cashService.List(ctx)
	if err != nil || len(listed) != 1 || listed[0].Status != StatusReversed || listed[0].EffectiveAmount != "0" {
		t.Fatalf("List() = %#v, %v", listed, err)
	}

	_, err = fixture.allocationService.Apply(ctx, allocationInput(fixture, "b5000000-0000-4000-8000-000000000001", "10"))
	var stateConflict *allocation.CashTransactionStateConflictError
	if !errors.As(err, &stateConflict) {
		t.Fatalf("Apply() after cash reversal error = %v", err)
	}
	_, err = environment.Pool.Exec(ctx, `
		INSERT INTO allocations (
			id, cash_transaction_id, obligation_id, obligation_revision,
			amount, currency, source_kind, manual_source_reference,
			applied_by_actor_kind, applied_by_actor_id, applied_by_actor_display_name, applied_at
		) VALUES (
			'b5000000-0000-4000-8000-000000000002', $1, $2, 1,
			10, 'CNY', 'manual', '直接 SQL 绕过尝试',
			'local_user', $3, '资金冲销管理员', clock_timestamp()
		)
	`, fixture.transaction.ID, fixture.obligation.ID, fixture.actor.ID)
	var databaseError *pgconn.PgError
	if !errors.As(err, &databaseError) || databaseError.Code != "23514" {
		t.Fatalf("direct allocation after reversal error = %v", err)
	}

	var reversals, commands, audits, allocations int
	var actorKind, actorID, actorName, auditAmount, auditCurrency string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM cash_transaction_reversals),
			(SELECT count(*) FROM cash_transaction_reversal_create_commands),
			(SELECT count(*) FROM audit_entries WHERE action = 'cash_transaction_reversed'),
			(SELECT count(*) FROM allocations),
			a.actor_kind, a.actor_id, a.actor_display_name,
			a.details->>'amount', a.details->>'currency'
		FROM audit_entries a
		WHERE a.action = 'cash_transaction_reversed'
	`).Scan(&reversals, &commands, &audits, &allocations, &actorKind, &actorID, &actorName, &auditAmount, &auditCurrency); err != nil {
		t.Fatalf("read reversal projection facts: %v", err)
	}
	if reversals != 1 || commands != 1 || audits != 1 || allocations != 0 ||
		actorKind != fixture.actor.Kind || actorID != fixture.actor.ID || actorName != fixture.actor.DisplayName ||
		auditAmount != "100" || auditCurrency != "CNY" {
		t.Fatalf("facts reversals=%d commands=%d audits=%d allocations=%d actor=%s/%s/%s amount=%s currency=%s",
			reversals, commands, audits, allocations, actorKind, actorID, actorName, auditAmount, auditCurrency)
	}

	duplicate := CreateInput{
		CommandID: "b3000000-0000-4000-8000-000000000002", Type: "receipt",
		LedgerOwnerPartyID: fixture.owner.ID, CounterpartyPartyID: fixture.counterparty.ID,
		Description: "试图复用已冲销流水", Amount: "100", Currency: "CNY", OccurredOn: "2026-09-19",
		AccountReference: "冲销测试账户", ManualSourceReference: "冲销测试回单",
		ExternalTransactionReference: "b3000000-0000-4000-8000-000000000001", Actor: fixture.actor,
	}
	_, err = fixture.cashService.Create(ctx, duplicate)
	var externalConflict *ExternalReferenceConflictError
	if !errors.As(err, &externalConflict) || externalConflict.ExistingTransactionID != fixture.transaction.ID {
		t.Fatalf("duplicate external reference after reversal error = %v", err)
	}
}

func TestPostgresCashTransactionReversalRequiresAllocationReversal(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	fixture := setupCashReversalFixture(t, ctx, environment, "c3000000-0000-4000-8000-000000000001")
	applied, err := fixture.allocationService.Apply(ctx, allocationInput(fixture, "c4000000-0000-4000-8000-000000000001", "40"))
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	_, err = fixture.cashService.Reverse(ctx, cashReverseInput(fixture, "c5000000-0000-4000-8000-000000000001"))
	var allocatedConflict *TransactionAllocatedConflictError
	if !errors.As(err, &allocatedConflict) || allocatedConflict.AllocatedAmount != "40" {
		t.Fatalf("Reverse() with allocation error = %v", err)
	}
	var cashReversals, cashCommands, cashAudits int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM cash_transaction_reversals),
			(SELECT count(*) FROM cash_transaction_reversal_create_commands),
			(SELECT count(*) FROM audit_entries WHERE action = 'cash_transaction_reversed')
	`).Scan(&cashReversals, &cashCommands, &cashAudits); err != nil {
		t.Fatalf("read blocked reversal facts: %v", err)
	}
	if cashReversals != 0 || cashCommands != 0 || cashAudits != 0 {
		t.Fatalf("blocked reversal facts = %d/%d/%d", cashReversals, cashCommands, cashAudits)
	}
	_, err = environment.Pool.Exec(ctx, `
		INSERT INTO cash_transaction_reversals (
			id, cash_transaction_id, transaction_type, amount, currency,
			source_kind, reversal_reason,
			reversed_by_actor_kind, reversed_by_actor_id,
			reversed_by_actor_display_name, reversed_at
		) VALUES (
			'c5000000-0000-4000-8000-000000000003', $1, 'receipt', 100, 'CNY',
			'manual', '直接 SQL 忽略分配尝试',
			'local_user', $2, '资金冲销管理员', clock_timestamp()
		)
	`, fixture.transaction.ID, fixture.actor.ID)
	var databaseError *pgconn.PgError
	if !errors.As(err, &databaseError) || databaseError.Code != "23514" {
		t.Fatalf("direct reversal with active allocation error = %v", err)
	}

	_, err = fixture.allocationService.Reverse(ctx, allocation.ReverseInput{
		CommandID: "c6000000-0000-4000-8000-000000000001", AllocationID: applied.Allocation.ID,
		Reason: "先释放错误核销", Actor: fixture.actor,
	})
	if err != nil {
		t.Fatalf("Reverse allocation error = %v", err)
	}
	result, err := fixture.cashService.Reverse(ctx, cashReverseInput(fixture, "c5000000-0000-4000-8000-000000000002"))
	if err != nil || result.Replayed {
		t.Fatalf("Reverse() after allocation reversal = %#v, %v", result, err)
	}
	got, found, err := fixture.cashService.Get(ctx, fixture.transaction.ID)
	if err != nil || !found || got.Status != StatusReversed || got.AllocatedAmount != "0" || got.UnallocatedAmount != "0" {
		t.Fatalf("cash projection after ordered reversals = %#v, %v, %v", got, found, err)
	}
	allocations, err := fixture.allocationService.ListForCashTransaction(ctx, fixture.transaction.ID)
	if err != nil || len(allocations) != 1 || allocations[0].Status != allocation.StatusReversed || allocations[0].Reversal == nil {
		t.Fatalf("allocation history after cash reversal = %#v, %v", allocations, err)
	}
}

func TestPostgresCashTransactionReversalConcurrency(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	fixture := setupCashReversalFixture(t, ctx, environment, "d3000000-0000-4000-8000-000000000001")

	type reverseOutcome struct {
		result ReverseResult
		err    error
	}
	run := func(inputs []ReverseInput) []reverseOutcome {
		t.Helper()
		start := make(chan struct{})
		results := make(chan reverseOutcome, len(inputs))
		var workers sync.WaitGroup
		for _, input := range inputs {
			input := input
			workers.Add(1)
			go func() {
				defer workers.Done()
				<-start
				result, err := fixture.cashService.Reverse(ctx, input)
				results <- reverseOutcome{result: result, err: err}
			}()
		}
		close(start)
		workers.Wait()
		close(results)
		values := make([]reverseOutcome, 0, len(inputs))
		for value := range results {
			values = append(values, value)
		}
		return values
	}

	same := cashReverseInput(fixture, "d4000000-0000-4000-8000-000000000001")
	sameResults := run([]ReverseInput{same, same})
	replays := 0
	var reversalID string
	for _, outcome := range sameResults {
		if outcome.err != nil {
			t.Fatalf("same command Reverse() error = %v", outcome.err)
		}
		if reversalID == "" {
			reversalID = outcome.result.Reversal.ID
		}
		if outcome.result.Reversal.ID != reversalID {
			t.Fatalf("same command reversal IDs differ: %s/%s", reversalID, outcome.result.Reversal.ID)
		}
		if outcome.result.Replayed {
			replays++
		}
	}
	if replays != 1 {
		t.Fatalf("same command replay count = %d", replays)
	}

	secondFixture := setupCashReversalFixtureWithExistingParties(t, ctx, environment, fixture, "d3000000-0000-4000-8000-000000000002")
	fixture = secondFixture
	first := cashReverseInput(fixture, "d5000000-0000-4000-8000-000000000001")
	second := cashReverseInput(fixture, "d5000000-0000-4000-8000-000000000002")
	differentResults := run([]ReverseInput{first, second})
	successes, conflicts := 0, 0
	for _, outcome := range differentResults {
		if outcome.err == nil {
			successes++
			continue
		}
		var conflict *TransactionAlreadyReversedError
		if errors.As(outcome.err, &conflict) {
			conflicts++
			continue
		}
		t.Fatalf("unexpected different command error = %v", outcome.err)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("different command results successes=%d conflicts=%d", successes, conflicts)
	}
}

func setupCashReversalFixtureWithExistingParties(
	t *testing.T,
	ctx context.Context,
	environment testdatabase.Environment,
	base cashReversalFixture,
	commandID string,
) cashReversalFixture {
	t.Helper()
	created, err := base.cashService.Create(ctx, CreateInput{
		CommandID: commandID, Type: "receipt",
		LedgerOwnerPartyID: base.owner.ID, CounterpartyPartyID: base.counterparty.ID,
		Description: "第二笔冲销测试收款", Amount: "100", Currency: "CNY", OccurredOn: "2026-09-20",
		AccountReference: "第二冲销测试账户", ManualSourceReference: "第二冲销测试回单",
		ExternalTransactionReference: commandID, Actor: base.actor,
	})
	if err != nil {
		t.Fatalf("create second reversal fixture cash transaction: %v", err)
	}
	base.transaction = created.Transaction
	return base
}

func TestPostgresCashTransactionReversalAndAllocationApplySerialize(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	fixture := setupCashReversalFixture(t, ctx, environment, "e3000000-0000-4000-8000-000000000001")
	start := make(chan struct{})
	var reverseErr, applyErr error
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		<-start
		_, reverseErr = fixture.cashService.Reverse(ctx, cashReverseInput(fixture, "e4000000-0000-4000-8000-000000000001"))
	}()
	go func() {
		defer workers.Done()
		<-start
		_, applyErr = fixture.allocationService.Apply(ctx, allocationInput(fixture, "e5000000-0000-4000-8000-000000000001", "10"))
	}()
	close(start)
	workers.Wait()

	if reverseErr == nil {
		var stateConflict *allocation.CashTransactionStateConflictError
		if !errors.As(applyErr, &stateConflict) {
			t.Fatalf("reverse won, apply error = %v", applyErr)
		}
	} else if applyErr == nil {
		var allocatedConflict *TransactionAllocatedConflictError
		if !errors.As(reverseErr, &allocatedConflict) {
			t.Fatalf("apply won, reverse error = %v", reverseErr)
		}
	} else {
		t.Fatalf("both concurrent operations failed: reverse=%v apply=%v", reverseErr, applyErr)
	}

	var cashReversals, allocations int
	var netAllocated string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM cash_transaction_reversals),
			(SELECT count(*) FROM allocations),
			(SELECT COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0)::text
			 FROM allocations a LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
			 WHERE a.cash_transaction_id = $1)
	`, fixture.transaction.ID).Scan(&cashReversals, &allocations, &netAllocated); err != nil {
		t.Fatalf("read serialized outcome: %v", err)
	}
	if cashReversals == 1 {
		if allocations != 0 || netAllocated != "0" {
			t.Fatalf("reversal outcome reversals=%d allocations=%d net=%s", cashReversals, allocations, netAllocated)
		}
	} else if cashReversals == 0 {
		if allocations != 1 || netAllocated != "10.000000" {
			t.Fatalf("allocation outcome reversals=%d allocations=%d net=%s", cashReversals, allocations, netAllocated)
		}
	} else {
		t.Fatalf("unexpected reversal count = %d", cashReversals)
	}
}

func TestPostgresCashTransactionAndAllocationReversalsSerialize(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	fixture := setupCashReversalFixture(t, ctx, environment, "ea000000-0000-4000-8000-000000000001")
	applied, err := fixture.allocationService.Apply(ctx, allocationInput(fixture, "ea000000-0000-4000-8000-000000000002", "25"))
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	start := make(chan struct{})
	var cashReverseErr, allocationReverseErr error
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		<-start
		_, cashReverseErr = fixture.cashService.Reverse(ctx, cashReverseInput(fixture, "ea000000-0000-4000-8000-000000000003"))
	}()
	go func() {
		defer workers.Done()
		<-start
		_, allocationReverseErr = fixture.allocationService.Reverse(ctx, allocation.ReverseInput{
			CommandID: "ea000000-0000-4000-8000-000000000004", AllocationID: applied.Allocation.ID,
			Reason: "并发释放核销", Actor: fixture.actor,
		})
	}()
	close(start)
	workers.Wait()

	if allocationReverseErr != nil {
		t.Fatalf("allocation reversal must succeed: %v", allocationReverseErr)
	}
	if cashReverseErr != nil {
		var allocatedConflict *TransactionAllocatedConflictError
		if !errors.As(cashReverseErr, &allocatedConflict) {
			t.Fatalf("unexpected cash reversal error = %v", cashReverseErr)
		}
	}
	var cashReversals, allocationReversals int
	var netAllocated string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM cash_transaction_reversals),
			(SELECT count(*) FROM allocation_reversals),
			(SELECT COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0)::text
			 FROM allocations a LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
			 WHERE a.cash_transaction_id = $1)
	`, fixture.transaction.ID).Scan(&cashReversals, &allocationReversals, &netAllocated); err != nil {
		t.Fatalf("read reversal serialization facts: %v", err)
	}
	if allocationReversals != 1 || netAllocated != "0.000000" || cashReversals < 0 || cashReversals > 1 {
		t.Fatalf("serialized reversals cash=%d allocation=%d net=%s", cashReversals, allocationReversals, netAllocated)
	}
	if cashReverseErr == nil && cashReversals != 1 {
		t.Fatalf("successful cash reversal count = %d", cashReversals)
	}
	if cashReverseErr != nil && cashReversals != 0 {
		t.Fatalf("failed cash reversal count = %d", cashReversals)
	}
}

func TestPostgresDirectCashReversalAndAllocationInsertSerialize(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	fixture := setupCashReversalFixture(t, ctx, environment, "eb000000-0000-4000-8000-000000000001")

	start := make(chan struct{})
	var cashReverseErr, allocationInsertErr error
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		<-start
		_, cashReverseErr = environment.Pool.Exec(ctx, `
			INSERT INTO cash_transaction_reversals (
				id, cash_transaction_id, transaction_type, amount, currency,
				source_kind, reversal_reason,
				reversed_by_actor_kind, reversed_by_actor_id,
				reversed_by_actor_display_name, reversed_at
			) VALUES (
				'eb000000-0000-4000-8000-000000000002', $1, 'receipt', 100, 'CNY',
				'manual', '直接 SQL 并发冲销', 'local_user', $2, '资金冲销管理员', clock_timestamp()
			)
		`, fixture.transaction.ID, fixture.actor.ID)
	}()
	go func() {
		defer workers.Done()
		<-start
		_, allocationInsertErr = environment.Pool.Exec(ctx, `
			INSERT INTO allocations (
				id, cash_transaction_id, obligation_id, obligation_revision,
				amount, currency, source_kind, manual_source_reference,
				applied_by_actor_kind, applied_by_actor_id, applied_by_actor_display_name, applied_at
			) VALUES (
				'eb000000-0000-4000-8000-000000000003', $1, $2, 1,
				10, 'CNY', 'manual', '直接 SQL 并发分配',
				'local_user', $3, '资金冲销管理员', clock_timestamp()
			)
		`, fixture.transaction.ID, fixture.obligation.ID, fixture.actor.ID)
	}()
	close(start)
	workers.Wait()

	if (cashReverseErr == nil) == (allocationInsertErr == nil) {
		t.Fatalf("expected exactly one direct insert success: cash=%v allocation=%v", cashReverseErr, allocationInsertErr)
	}
	failed := cashReverseErr
	if failed == nil {
		failed = allocationInsertErr
	}
	var databaseError *pgconn.PgError
	if !errors.As(failed, &databaseError) || databaseError.Code != "23514" {
		t.Fatalf("losing direct insert error = %v", failed)
	}

	var cashReversals, allocations int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM cash_transaction_reversals),
			(SELECT count(*) FROM allocations)
	`).Scan(&cashReversals, &allocations); err != nil {
		t.Fatalf("read direct serialization facts: %v", err)
	}
	if cashReversals+allocations != 1 {
		t.Fatalf("direct serialization facts cash_reversals=%d allocations=%d", cashReversals, allocations)
	}
}

func TestPostgresCashTransactionReversalRollbackConstraintsAndImmutability(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	fixture := setupCashReversalFixture(t, ctx, environment, "f3000000-0000-4000-8000-000000000001")
	repository := NewPostgresRepository(environment.Pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	reversalID := "f4000000-0000-4000-8000-000000000001"
	commandID := "f5000000-0000-4000-8000-000000000001"
	_, _, err := repository.Reverse(ctx, Reversal{
		ID: reversalID, CashTransactionID: fixture.transaction.ID,
		SourceKind: SourceKindManual, Reason: "审计失败应整体回滚",
		ReversedBy: fixture.actor, ReversedAt: now,
	}, ReverseCommand{
		CommandID: commandID, PayloadHash: make([]byte, 32), Actor: fixture.actor, ReversedAt: now,
	}, audit.Entry{
		ID: "f6000000-0000-4000-8000-000000000001", ObjectType: audit.ObjectCashReversal,
		ObjectID: reversalID, Action: AuditActionReversed, CommandID: commandID,
		Actor: fixture.actor, OccurredAt: now, Summary: "", Details: map[string]any{},
	})
	if err == nil {
		t.Fatal("Reverse() audit failure error = nil")
	}
	var reversals, commands, audits int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM cash_transaction_reversals WHERE id = $1),
			(SELECT count(*) FROM cash_transaction_reversal_create_commands WHERE command_id = $2),
			(SELECT count(*) FROM audit_entries WHERE command_id = $2)
	`, reversalID, commandID).Scan(&reversals, &commands, &audits); err != nil {
		t.Fatalf("read reversal rollback facts: %v", err)
	}
	if reversals != 0 || commands != 0 || audits != 0 {
		t.Fatalf("rollback facts reversals=%d commands=%d audits=%d", reversals, commands, audits)
	}

	_, err = environment.Pool.Exec(ctx, `
		INSERT INTO cash_transaction_reversals (
			id, cash_transaction_id, transaction_type, amount, currency,
			source_kind, reversal_reason,
			reversed_by_actor_kind, reversed_by_actor_id,
			reversed_by_actor_display_name, reversed_at
		) VALUES (
			'f7000000-0000-4000-8000-000000000001', $1, 'payment', 99, 'USD',
			'manual', '错误复制事实', 'local_user', $2, '资金冲销管理员', clock_timestamp()
		)
	`, fixture.transaction.ID, fixture.actor.ID)
	var databaseError *pgconn.PgError
	if !errors.As(err, &databaseError) || databaseError.Code != "23514" {
		t.Fatalf("mismatched reversal facts error = %v", err)
	}

	created, err := fixture.cashService.Reverse(ctx, cashReverseInput(fixture, "f8000000-0000-4000-8000-000000000001"))
	if err != nil {
		t.Fatalf("service.Reverse() error = %v", err)
	}
	for _, statement := range []string{
		`UPDATE cash_transaction_reversals SET reversal_reason = 'changed' WHERE id = $1`,
		`DELETE FROM cash_transaction_reversals WHERE id = $1`,
		`UPDATE cash_transaction_reversal_create_commands SET actor_display_name = 'changed' WHERE cash_transaction_reversal_id = $1`,
		`DELETE FROM cash_transaction_reversal_create_commands WHERE cash_transaction_reversal_id = $1`,
	} {
		_, err := environment.Pool.Exec(ctx, statement, created.Reversal.ID)
		var immutableError *pgconn.PgError
		if !errors.As(err, &immutableError) || immutableError.Code != "55000" {
			t.Fatalf("immutable statement error = %v", err)
		}
	}

	missingFixture := fixture
	missingFixture.transaction.ID = "f9000000-0000-4000-8000-000000000001"
	_, err = fixture.cashService.Reverse(ctx, cashReverseInput(missingFixture, "f9000000-0000-4000-8000-000000000002"))
	var notFound *TransactionNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("missing cash transaction reversal error = %v", err)
	}
}

func TestPostgresCashTransactionConcurrency(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "并发资金管理员",
	}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	service := NewService(NewPostgresRepository(environment.Pool))
	baseInput := CreateInput{
		Type: "receipt", LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Description: "并发收款", Amount: "20", Currency: "CNY", OccurredOn: "2026-09-19",
		AccountReference: "并发测试账户", ManualSourceReference: "并发测试依据",
		ExternalTransactionReference: "CONCURRENT-001", Actor: actor,
	}
	type concurrentResult struct {
		result CreateResult
		err    error
	}
	run := func(inputs []CreateInput) []concurrentResult {
		t.Helper()
		start := make(chan struct{})
		results := make(chan concurrentResult, len(inputs))
		var workers sync.WaitGroup
		for _, input := range inputs {
			input := input
			workers.Add(1)
			go func() {
				defer workers.Done()
				<-start
				result, err := service.Create(ctx, input)
				results <- concurrentResult{result: result, err: err}
			}()
		}
		close(start)
		workers.Wait()
		close(results)
		values := make([]concurrentResult, 0, len(inputs))
		for value := range results {
			values = append(values, value)
		}
		return values
	}

	sameCommand := baseInput
	sameCommand.CommandID = "44444444-4444-4444-8444-444444444444"
	sameResults := run([]CreateInput{sameCommand, sameCommand})
	replayCount := 0
	var sameTransactionID string
	for _, result := range sameResults {
		if result.err != nil {
			t.Fatalf("same command Create() error = %v", result.err)
		}
		if sameTransactionID == "" {
			sameTransactionID = result.result.Transaction.ID
		}
		if result.result.Transaction.ID != sameTransactionID {
			t.Fatalf("same command IDs differ: %s %s", sameTransactionID, result.result.Transaction.ID)
		}
		if result.result.Replayed {
			replayCount++
		}
	}
	if replayCount != 1 {
		t.Fatalf("same command replay count = %d", replayCount)
	}

	differentBase := baseInput
	differentBase.Description = "不同命令相同外部流水"
	differentBase.ExternalTransactionReference = "CONCURRENT-002"
	first := differentBase
	first.CommandID = "55555555-5555-4555-8555-555555555555"
	second := differentBase
	second.CommandID = "66666666-6666-4666-8666-666666666666"
	differentResults := run([]CreateInput{first, second})
	successes, externalConflicts := 0, 0
	for _, result := range differentResults {
		if result.err == nil {
			successes++
			continue
		}
		var conflict *ExternalReferenceConflictError
		if errors.As(result.err, &conflict) {
			externalConflicts++
			continue
		}
		t.Fatalf("unexpected different command error = %v", result.err)
	}
	if successes != 1 || externalConflicts != 1 {
		t.Fatalf("different command results successes=%d external_conflicts=%d", successes, externalConflicts)
	}

	var transactions, commands, audits int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM cash_transactions),
			(SELECT count(*) FROM cash_transaction_create_commands),
			(SELECT count(*) FROM audit_entries WHERE action = 'cash_transaction_recorded')
	`).Scan(&transactions, &commands, &audits); err != nil {
		t.Fatalf("read concurrent facts: %v", err)
	}
	if transactions != 2 || commands != 2 || audits != 2 {
		t.Fatalf("concurrent facts transactions=%d commands=%d audits=%d", transactions, commands, audits)
	}
}

func TestPostgresCashTransactionRollsBackAndProtectsFacts(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "回滚资金管理员",
	}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	repository := NewPostgresRepository(environment.Pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	transactionID := "44444444-4444-4444-8444-444444444444"
	commandID := "55555555-5555-4555-8555-555555555555"
	_, _, err := repository.Create(ctx, Transaction{
		ID: transactionID, Type: TypePayment, Description: "应整体回滚的付款",
		LedgerOwnerPartyID: owner.ID,
		Payer:              PartySnapshot{PartyID: owner.ID},
		Payee:              PartySnapshot{PartyID: counterparty.ID},
		Amount:             "100", Currency: "CNY", OccurredOn: "2026-09-19",
		AccountReference: "回滚测试账户", SourceKind: SourceKindManual,
		ManualSourceReference: "回滚测试依据", ExternalTransactionReference: "ROLLBACK-001",
		Status: StatusRecorded, RecordedBy: actor, RecordedAt: now,
	}, CreateCommand{
		CommandID: commandID, PayloadHash: make([]byte, 32), Actor: actor, CreatedAt: now,
	}, audit.Entry{
		ID: "66666666-6666-4666-8666-666666666666", ObjectType: audit.ObjectCashTransaction, ObjectID: transactionID,
		Action: AuditActionRecorded, CommandID: commandID, Actor: actor, OccurredAt: now,
		Summary: "", Details: map[string]any{},
	})
	if err == nil {
		t.Fatal("Create() error = nil")
	}
	var transactions, commands, audits int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM cash_transactions WHERE id = $1),
			(SELECT count(*) FROM cash_transaction_create_commands WHERE command_id = $2),
			(SELECT count(*) FROM audit_entries WHERE command_id = $2)
	`, transactionID, commandID).Scan(&transactions, &commands, &audits); err != nil {
		t.Fatalf("read rollback facts: %v", err)
	}
	if transactions != 0 || commands != 0 || audits != 0 {
		t.Fatalf("rollback facts transactions=%d commands=%d audits=%d", transactions, commands, audits)
	}

	service := NewService(repository)
	created, err := service.Create(ctx, CreateInput{
		CommandID: "77777777-7777-4777-8777-777777777777", Type: "payment",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Description: "不可变付款", Amount: "1", Currency: "CNY", OccurredOn: "2026-09-19",
		AccountReference: "不可变账户", ManualSourceReference: "不可变依据",
		ExternalTransactionReference: "IMMUTABLE-001", Actor: actor,
	})
	if err != nil {
		t.Fatalf("service.Create() error = %v", err)
	}
	for _, statement := range []string{
		`UPDATE cash_transactions SET description = 'changed' WHERE id = $1`,
		`DELETE FROM cash_transactions WHERE id = $1`,
		`UPDATE cash_transaction_create_commands SET actor_display_name = 'changed' WHERE cash_transaction_id = $1`,
		`DELETE FROM cash_transaction_create_commands WHERE cash_transaction_id = $1`,
	} {
		_, err := environment.Pool.Exec(ctx, statement, created.Transaction.ID)
		var databaseError *pgconn.PgError
		if !errors.As(err, &databaseError) || databaseError.Code != "55000" {
			t.Fatalf("immutable statement error = %v", err)
		}
	}
}

func createTestParties(
	t *testing.T,
	ctx context.Context,
	environment testdatabase.Environment,
	actor audit.ActorRef,
) (party.Party, party.Party) {
	t.Helper()
	service := party.NewService(party.NewPostgresRepository(environment.Pool))
	owner, err := service.Create(ctx, party.CreateInput{
		CommandID: "22222222-2222-4222-8222-222222222222", Kind: "organization",
		LegalName: "Owner Company", RegistrationIdentifier: "OWNER-001", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create owner Party: %v", err)
	}
	counterparty, err := service.Create(ctx, party.CreateInput{
		CommandID: "33333333-3333-4333-8333-333333333333", Kind: "individual",
		LegalName: "Counter Person", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create counterparty Party: %v", err)
	}
	return owner.Party, counterparty.Party
}
