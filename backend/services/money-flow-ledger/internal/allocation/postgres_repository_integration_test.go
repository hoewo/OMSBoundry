//go:build integration

package allocation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/cashtransaction"
	"omsbizfoundry/money-flow-ledger/internal/database"
	"omsbizfoundry/money-flow-ledger/internal/obligation"
	"omsbizfoundry/money-flow-ledger/internal/party"
	"omsbizfoundry/money-flow-ledger/internal/testdatabase"
)

type allocationFixture struct {
	actor             audit.ActorRef
	owner             party.Party
	counterparty      party.Party
	obligation        obligation.Obligation
	cash              cashtransaction.Transaction
	obligationService *obligation.Service
	cashService       *cashtransaction.Service
	allocationService *Service
}

func setupAllocationFixture(t *testing.T, ctx context.Context, environment testdatabase.Environment, cashAmount, obligationAmount string) allocationFixture {
	t.Helper()
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "核销联调管理员",
	}
	partyService := party.NewService(party.NewPostgresRepository(environment.Pool))
	ownerResult, err := partyService.Create(ctx, party.CreateInput{
		CommandID: "21111111-1111-4111-8111-111111111111", Kind: "organization",
		LegalName: "Allocation Owner", RegistrationIdentifier: "ALLOC-OWNER", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create owner Party: %v", err)
	}
	counterpartyResult, err := partyService.Create(ctx, party.CreateInput{
		CommandID: "31111111-1111-4111-8111-111111111111", Kind: "individual",
		LegalName: "Allocation Counterparty", RegistrationIdentifier: "ALLOC-COUNTER", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create counterparty Party: %v", err)
	}
	obligationService := obligation.NewService(obligation.NewPostgresRepository(environment.Pool))
	draft, err := obligationService.CreateDraft(ctx, obligation.CreateDraftInput{
		CommandID: "41111111-1111-4111-8111-111111111111", Direction: "receivable",
		LedgerOwnerPartyID: ownerResult.Party.ID, CounterpartyPartyID: counterpartyResult.Party.ID,
		Title: "Allocation receivable", OriginalAmount: obligationAmount, Currency: "CNY",
		DueOn: "2026-10-31", ManualSourceReference: "Allocation obligation source", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create obligation draft: %v", err)
	}
	confirmed, err := obligationService.Confirm(ctx, obligation.ConfirmInput{
		CommandID: "51111111-1111-4111-8111-111111111111", ObligationID: draft.Obligation.ID,
		ExpectedDraftVersion: draft.Obligation.DraftVersion, Actor: actor,
	})
	if err != nil {
		t.Fatalf("confirm obligation: %v", err)
	}
	cashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	cashResult, err := cashService.Create(ctx, cashtransaction.CreateInput{
		CommandID: "61111111-1111-4111-8111-111111111111", Type: "receipt",
		LedgerOwnerPartyID: ownerResult.Party.ID, CounterpartyPartyID: counterpartyResult.Party.ID,
		Description: "Allocation receipt", Amount: cashAmount, Currency: "CNY",
		OccurredOn: "2026-09-19", AccountReference: "Allocation account",
		ManualSourceReference: "Allocation cash source", ExternalTransactionReference: "ALLOC-CASH-001", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create cash transaction: %v", err)
	}
	return allocationFixture{
		actor:             actor,
		owner:             ownerResult.Party,
		counterparty:      counterpartyResult.Party,
		obligation:        confirmed.Obligation,
		cash:              cashResult.Transaction,
		obligationService: obligationService,
		cashService:       cashService,
		allocationService: NewService(NewPostgresRepository(environment.Pool)),
	}
}

func applyInput(fixture allocationFixture, commandID, amount string) ApplyInput {
	return ApplyInput{
		CommandID: commandID, CashTransactionID: fixture.cash.ID,
		ObligationID: fixture.obligation.ID, ExpectedObligationRevision: fixture.obligation.CurrentRevision,
		Amount: amount, ManualSourceReference: "人工核对资金和义务", Actor: fixture.actor,
	}
}

func reverseInput(fixture allocationFixture, commandID, allocationID string) ReverseInput {
	return ReverseInput{
		CommandID: commandID, AllocationID: allocationID,
		Reason: "原分配选择了错误义务", Actor: fixture.actor,
	}
}

func TestPostgresAllocationReversalRestoresBalancesAndAllowsReallocation(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	fixture := setupAllocationFixture(t, ctx, environment, "100", "80")
	applied, err := fixture.allocationService.Apply(ctx, applyInput(fixture, "71000000-0000-4000-8000-000000000001", "40.25"))
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	input := reverseInput(fixture, "72000000-0000-4000-8000-000000000001", applied.Allocation.ID)
	reversed, err := fixture.allocationService.Reverse(ctx, input)
	if err != nil || reversed.Replayed {
		t.Fatalf("Reverse() = %#v, %v", reversed, err)
	}
	value := reversed.Reversal
	if value.AllocationID != applied.Allocation.ID || value.CashTransactionID != fixture.cash.ID ||
		value.ObligationID != fixture.obligation.ID || value.ObligationRevision != fixture.obligation.CurrentRevision ||
		value.Amount != "40.25" || value.Currency != "CNY" || value.SourceKind != SourceKindManual ||
		value.Reason != "原分配选择了错误义务" || value.ReversedBy != fixture.actor {
		t.Fatalf("reversal = %#v", value)
	}

	got, found, err := fixture.allocationService.Get(ctx, applied.Allocation.ID)
	if err != nil || !found || got.Status != StatusReversed || got.Reversal == nil || got.Reversal.ID != value.ID {
		t.Fatalf("Get() after reversal = %#v, %v, %v", got, found, err)
	}
	listed, err := fixture.allocationService.List(ctx)
	if err != nil || len(listed) != 1 || listed[0].Status != StatusReversed || listed[0].Reversal == nil {
		t.Fatalf("List() after reversal = %#v, %v", listed, err)
	}
	cashAfter, found, err := fixture.cashService.Get(ctx, fixture.cash.ID)
	if err != nil || !found || cashAfter.AllocatedAmount != "0" || cashAfter.UnallocatedAmount != "100" {
		t.Fatalf("cash after reversal = %#v, %v, %v", cashAfter, found, err)
	}
	obligationAfter, found, err := fixture.obligationService.GetDetails(ctx, fixture.obligation.ID)
	if err != nil || !found || obligationAfter.Obligation.AllocatedAmount != "0" || obligationAfter.Obligation.OutstandingAmount != "80" {
		t.Fatalf("obligation after reversal = %#v, %v, %v", obligationAfter.Obligation, found, err)
	}

	replay := input
	replay.Reason = " 原分配选择了错误义务 "
	replayed, err := fixture.allocationService.Reverse(ctx, replay)
	if err != nil || !replayed.Replayed || replayed.Reversal.ID != value.ID {
		t.Fatalf("replay Reverse() = %#v, %v", replayed, err)
	}
	changed := replay
	changed.Reason = "另一原因"
	_, err = fixture.allocationService.Reverse(ctx, changed)
	var idempotencyConflict *ReversalIdempotencyConflictError
	if !errors.As(err, &idempotencyConflict) {
		t.Fatalf("changed reversal payload error = %v", err)
	}
	duplicate := input
	duplicate.CommandID = "72000000-0000-4000-8000-000000000002"
	_, err = fixture.allocationService.Reverse(ctx, duplicate)
	var alreadyReversed *AllocationAlreadyReversedError
	if !errors.As(err, &alreadyReversed) || alreadyReversed.ReversalID != value.ID {
		t.Fatalf("duplicate reversal error = %v", err)
	}

	reallocated, err := fixture.allocationService.Apply(ctx, applyInput(fixture, "71000000-0000-4000-8000-000000000002", "80"))
	if err != nil || reallocated.Allocation.Status != StatusActive {
		t.Fatalf("reallocate released balance = %#v, %v", reallocated, err)
	}
	cashAfter, _, _ = fixture.cashService.Get(ctx, fixture.cash.ID)
	obligationAfter, _, _ = fixture.obligationService.GetDetails(ctx, fixture.obligation.ID)
	if cashAfter.AllocatedAmount != "80" || cashAfter.UnallocatedAmount != "20" ||
		obligationAfter.Obligation.AllocatedAmount != "80" || obligationAfter.Obligation.OutstandingAmount != "0" {
		t.Fatalf("reallocated balances cash=%#v obligation=%#v", cashAfter, obligationAfter.Obligation)
	}

	var allocations, reversals, reversalCommands, reversalAudits int
	var actorKind, actorID, actorName, auditAmount, auditCurrency string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM allocations),
			(SELECT count(*) FROM allocation_reversals),
			(SELECT count(*) FROM allocation_reversal_create_commands),
			(SELECT count(*) FROM audit_entries WHERE action = 'allocation_reversed'),
			actor_kind, actor_id, actor_display_name,
			details->>'amount', details->>'currency'
		FROM audit_entries
		WHERE action = 'allocation_reversed' AND object_id = $1
	`, value.ID).Scan(
		&allocations, &reversals, &reversalCommands, &reversalAudits,
		&actorKind, &actorID, &actorName, &auditAmount, &auditCurrency,
	); err != nil {
		t.Fatalf("read reversal facts: %v", err)
	}
	if allocations != 2 || reversals != 1 || reversalCommands != 1 || reversalAudits != 1 ||
		actorKind != fixture.actor.Kind || actorID != fixture.actor.ID || actorName != fixture.actor.DisplayName ||
		auditAmount != "40.25" || auditCurrency != "CNY" {
		t.Fatalf("facts allocations=%d reversals=%d commands=%d audits=%d actor=%s/%s/%s amount=%s currency=%s",
			allocations, reversals, reversalCommands, reversalAudits, actorKind, actorID, actorName, auditAmount, auditCurrency)
	}
}

func TestPostgresAllocationReversalConcurrency(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	fixture := setupAllocationFixture(t, ctx, environment, "100", "100")
	first, err := fixture.allocationService.Apply(ctx, applyInput(fixture, "73000000-0000-4000-8000-000000000001", "20"))
	if err != nil {
		t.Fatalf("first Apply() error = %v", err)
	}
	second, err := fixture.allocationService.Apply(ctx, applyInput(fixture, "73000000-0000-4000-8000-000000000002", "20"))
	if err != nil {
		t.Fatalf("second Apply() error = %v", err)
	}
	type concurrentResult struct {
		result ReverseResult
		err    error
	}
	run := func(inputs []ReverseInput) []concurrentResult {
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
				result, err := fixture.allocationService.Reverse(ctx, input)
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

	same := reverseInput(fixture, "74000000-0000-4000-8000-000000000001", first.Allocation.ID)
	sameResults := run([]ReverseInput{same, same})
	replays := 0
	var reversalID string
	for _, item := range sameResults {
		if item.err != nil {
			t.Fatalf("same command Reverse() error = %v", item.err)
		}
		if reversalID == "" {
			reversalID = item.result.Reversal.ID
		}
		if item.result.Reversal.ID != reversalID {
			t.Fatalf("same command reversal IDs differ: %s %s", reversalID, item.result.Reversal.ID)
		}
		if item.result.Replayed {
			replays++
		}
	}
	if replays != 1 {
		t.Fatalf("same command replay count = %d", replays)
	}

	left := reverseInput(fixture, "74000000-0000-4000-8000-000000000002", second.Allocation.ID)
	right := left
	right.CommandID = "74000000-0000-4000-8000-000000000003"
	differentResults := run([]ReverseInput{left, right})
	successes, conflicts := 0, 0
	for _, item := range differentResults {
		if item.err == nil {
			successes++
			continue
		}
		var conflict *AllocationAlreadyReversedError
		if errors.As(item.err, &conflict) {
			conflicts++
			continue
		}
		t.Fatalf("unexpected different command error = %v", item.err)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("different command results successes=%d conflicts=%d", successes, conflicts)
	}

	var reversals, commands, audits int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM allocation_reversals),
			(SELECT count(*) FROM allocation_reversal_create_commands),
			(SELECT count(*) FROM audit_entries WHERE action = 'allocation_reversed')
	`).Scan(&reversals, &commands, &audits); err != nil {
		t.Fatalf("read concurrent reversal facts: %v", err)
	}
	if reversals != 2 || commands != 2 || audits != 2 {
		t.Fatalf("concurrent reversal facts reversals=%d commands=%d audits=%d", reversals, commands, audits)
	}
}

func TestPostgresAllocationReversalAndApplyUseOneBalanceOrder(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	fixture := setupAllocationFixture(t, ctx, environment, "100", "100")
	original, err := fixture.allocationService.Apply(ctx, applyInput(fixture, "75000000-0000-4000-8000-000000000001", "100"))
	if err != nil {
		t.Fatalf("original Apply() error = %v", err)
	}

	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(2)
	var reverseErr, applyErr error
	go func() {
		defer workers.Done()
		<-start
		_, reverseErr = fixture.allocationService.Reverse(ctx, reverseInput(fixture, "76000000-0000-4000-8000-000000000001", original.Allocation.ID))
	}()
	go func() {
		defer workers.Done()
		<-start
		_, applyErr = fixture.allocationService.Apply(ctx, applyInput(fixture, "75000000-0000-4000-8000-000000000002", "100"))
	}()
	close(start)
	workers.Wait()
	if reverseErr != nil {
		t.Fatalf("concurrent Reverse() error = %v", reverseErr)
	}
	applySucceeded := applyErr == nil
	if applyErr != nil {
		var cashConflict *CashBalanceConflictError
		var obligationConflict *ObligationBalanceConflictError
		if !errors.As(applyErr, &cashConflict) && !errors.As(applyErr, &obligationConflict) {
			t.Fatalf("unexpected concurrent Apply() error = %v", applyErr)
		}
	}
	cashAfter, _, err := fixture.cashService.Get(ctx, fixture.cash.ID)
	if err != nil {
		t.Fatal(err)
	}
	obligationAfter, _, err := fixture.obligationService.GetDetails(ctx, fixture.obligation.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantAllocated, wantAvailable := "0", "100"
	if applySucceeded {
		wantAllocated, wantAvailable = "100", "0"
	}
	if cashAfter.AllocatedAmount != wantAllocated || cashAfter.UnallocatedAmount != wantAvailable ||
		obligationAfter.Obligation.AllocatedAmount != wantAllocated || obligationAfter.Obligation.OutstandingAmount != wantAvailable {
		t.Fatalf("concurrent balances applySucceeded=%v cash=%#v obligation=%#v", applySucceeded, cashAfter, obligationAfter.Obligation)
	}
}

func TestPostgresAllocationReversalRollbackConstraintsAndImmutability(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	fixture := setupAllocationFixture(t, ctx, environment, "100", "100")
	applied, err := fixture.allocationService.Apply(ctx, applyInput(fixture, "77000000-0000-4000-8000-000000000001", "40"))
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	repository := NewPostgresRepository(environment.Pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	reversalID := "78000000-0000-4000-8000-000000000001"
	commandID := "79000000-0000-4000-8000-000000000001"
	_, _, err = repository.Reverse(ctx, Reversal{
		ID: reversalID, AllocationID: applied.Allocation.ID, SourceKind: SourceKindManual,
		Reason: "审计失败应整体回滚", ReversedBy: fixture.actor, ReversedAt: now,
	}, ReverseCommand{
		CommandID: commandID, PayloadHash: make([]byte, 32), Actor: fixture.actor, ReversedAt: now,
	}, audit.Entry{
		ID: "7a000000-0000-4000-8000-000000000001", ObjectType: audit.ObjectAllocationReversal,
		ObjectID: reversalID, Action: AuditActionReversed, CommandID: commandID,
		Actor: fixture.actor, OccurredAt: now, Summary: "", Details: map[string]any{},
	})
	if err == nil {
		t.Fatal("Reverse() error = nil")
	}
	var reversals, commands, audits int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM allocation_reversals WHERE id = $1),
			(SELECT count(*) FROM allocation_reversal_create_commands WHERE command_id = $2),
			(SELECT count(*) FROM audit_entries WHERE command_id = $2)
	`, reversalID, commandID).Scan(&reversals, &commands, &audits); err != nil {
		t.Fatalf("read reversal rollback facts: %v", err)
	}
	if reversals != 0 || commands != 0 || audits != 0 {
		t.Fatalf("rollback facts reversals=%d commands=%d audits=%d", reversals, commands, audits)
	}

	missingInput := ReverseInput{
		CommandID:    "79000000-0000-4000-8000-000000000002",
		AllocationID: "7b000000-0000-4000-8000-000000000001",
		Reason:       "缺失 Allocation", Actor: fixture.actor,
	}
	_, err = fixture.allocationService.Reverse(ctx, missingInput)
	var notFound *AllocationNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("missing allocation error = %v", err)
	}

	created, err := fixture.allocationService.Reverse(ctx, reverseInput(fixture, "79000000-0000-4000-8000-000000000003", applied.Allocation.ID))
	if err != nil {
		t.Fatalf("service.Reverse() error = %v", err)
	}
	for _, statement := range []string{
		`UPDATE allocation_reversals SET reversal_reason = 'changed' WHERE id = $1`,
		`DELETE FROM allocation_reversals WHERE id = $1`,
		`UPDATE allocation_reversal_create_commands SET actor_display_name = 'changed' WHERE allocation_reversal_id = $1`,
		`DELETE FROM allocation_reversal_create_commands WHERE allocation_reversal_id = $1`,
	} {
		_, err := environment.Pool.Exec(ctx, statement, created.Reversal.ID)
		var databaseError *pgconn.PgError
		if !errors.As(err, &databaseError) || databaseError.Code != "55000" {
			t.Fatalf("immutable reversal statement error = %v", err)
		}
	}

	second, err := fixture.allocationService.Apply(ctx, applyInput(fixture, "77000000-0000-4000-8000-000000000002", "10"))
	if err != nil {
		t.Fatalf("second Apply() error = %v", err)
	}
	_, err = environment.Pool.Exec(ctx, `
		INSERT INTO allocation_reversals (
			id, allocation_id, cash_transaction_id, obligation_id, obligation_revision,
			amount, currency, source_kind, reversal_reason,
			reversed_by_actor_kind, reversed_by_actor_id, reversed_by_actor_display_name, reversed_at
		) VALUES (
			'7c000000-0000-4000-8000-000000000001', $1, $2, $3, $4,
			9, 'CNY', 'manual', '伪造不等额反向', $5, $6, $7, $8
		)
	`,
		second.Allocation.ID,
		second.Allocation.CashTransactionID,
		second.Allocation.ObligationID,
		second.Allocation.ObligationRevision,
		fixture.actor.Kind,
		fixture.actor.ID,
		fixture.actor.DisplayName,
		now,
	)
	var constraintError *pgconn.PgError
	if !errors.As(err, &constraintError) || constraintError.Code != "23514" {
		t.Fatalf("mismatched reversal constraint error = %v", err)
	}
}

func TestPostgresAllocationBalancesReplayAndAudit(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	fixture := setupAllocationFixture(t, ctx, environment, "100", "80")
	firstInput := applyInput(fixture, "71111111-1111-4111-8111-111111111111", "000040.250000")
	first, err := fixture.allocationService.Apply(ctx, firstInput)
	if err != nil || first.Replayed {
		t.Fatalf("first Apply() = %#v, %v", first, err)
	}
	value := first.Allocation
	if value.Amount != "40.25" || value.Currency != "CNY" || value.Status != StatusActive ||
		value.CashDescription != "Allocation receipt" || value.CashType != "receipt" ||
		value.ObligationTitle != "Allocation receivable" || value.ObligationDirection != "receivable" ||
		value.LedgerOwnerPartyID != fixture.owner.ID || value.CounterpartyPartyID != fixture.counterparty.ID ||
		value.AppliedBy != fixture.actor {
		t.Fatalf("allocation = %#v", value)
	}

	cashAfter, found, err := fixture.cashService.Get(ctx, fixture.cash.ID)
	if err != nil || !found || cashAfter.Amount != "100" || cashAfter.AllocatedAmount != "40.25" || cashAfter.UnallocatedAmount != "59.75" {
		t.Fatalf("cash balance = %#v, %v, %v", cashAfter, found, err)
	}
	obligationAfter, found, err := fixture.obligationService.GetDetails(ctx, fixture.obligation.ID)
	if err != nil || !found || obligationAfter.Obligation.OriginalAmount != "80" ||
		obligationAfter.Obligation.AllocatedAmount != "40.25" || obligationAfter.Obligation.OutstandingAmount != "39.75" ||
		obligationAfter.Obligation.Status != obligation.StatusOpen {
		t.Fatalf("obligation balance = %#v, %v, %v", obligationAfter.Obligation, found, err)
	}

	replayInput := firstInput
	replayInput.Amount = "40.250000"
	replayInput.ManualSourceReference = " 人工核对资金和义务 "
	replayed, err := fixture.allocationService.Apply(ctx, replayInput)
	if err != nil || !replayed.Replayed || replayed.Allocation.ID != first.Allocation.ID {
		t.Fatalf("replay Apply() = %#v, %v", replayed, err)
	}
	changed := replayInput
	changed.Amount = "40.26"
	_, err = fixture.allocationService.Apply(ctx, changed)
	var idempotencyConflict *IdempotencyConflictError
	if !errors.As(err, &idempotencyConflict) {
		t.Fatalf("changed payload error = %v", err)
	}
	changed = replayInput
	changed.Actor.ID = "81111111-1111-4111-8111-111111111111"
	_, err = fixture.allocationService.Apply(ctx, changed)
	if !errors.As(err, &idempotencyConflict) {
		t.Fatalf("changed actor error = %v", err)
	}

	second, err := fixture.allocationService.Apply(ctx, applyInput(fixture, "91111111-1111-4111-8111-111111111111", "39.75"))
	if err != nil || second.Replayed {
		t.Fatalf("exact outstanding Apply() = %#v, %v", second, err)
	}
	cashAfter, _, _ = fixture.cashService.Get(ctx, fixture.cash.ID)
	obligationAfter, _, _ = fixture.obligationService.GetDetails(ctx, fixture.obligation.ID)
	if cashAfter.AllocatedAmount != "80" || cashAfter.UnallocatedAmount != "20" ||
		obligationAfter.Obligation.AllocatedAmount != "80" || obligationAfter.Obligation.OutstandingAmount != "0" ||
		obligationAfter.Obligation.Status != obligation.StatusOpen {
		t.Fatalf("final projections cash=%#v obligation=%#v", cashAfter, obligationAfter.Obligation)
	}

	_, err = fixture.allocationService.Apply(ctx, applyInput(fixture, "a1111111-1111-4111-8111-111111111111", "0.000001"))
	var obligationBalance *ObligationBalanceConflictError
	if !errors.As(err, &obligationBalance) || obligationBalance.Available != "0" {
		t.Fatalf("zero outstanding error = %v", err)
	}

	listed, err := fixture.allocationService.List(ctx)
	if err != nil || len(listed) != 2 || listed[0].ID != second.Allocation.ID || listed[1].ID != first.Allocation.ID {
		t.Fatalf("List() = %#v, %v", listed, err)
	}
	got, found, err := fixture.allocationService.Get(ctx, first.Allocation.ID)
	if err != nil || !found || got.ID != first.Allocation.ID {
		t.Fatalf("Get() = %#v, %v, %v", got, found, err)
	}

	var allocations, commands, audits int
	var auditCurrency string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM allocations),
			(SELECT count(*) FROM allocation_apply_commands),
			(SELECT count(*) FROM audit_entries WHERE action = 'allocation_applied'),
			details->>'currency'
		FROM audit_entries
		WHERE action = 'allocation_applied' AND command_id = $1
	`, firstInput.CommandID).Scan(&allocations, &commands, &audits, &auditCurrency); err != nil {
		t.Fatalf("read allocation facts: %v", err)
	}
	if allocations != 2 || commands != 2 || audits != 2 || auditCurrency != "CNY" {
		t.Fatalf("facts allocations=%d commands=%d audits=%d currency=%s", allocations, commands, audits, auditCurrency)
	}
}

func TestPostgresMultiAllocationCompositionAndScopedHistory(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	fixture := setupAllocationFixture(t, ctx, environment, "120", "100")

	secondDraft, err := fixture.obligationService.CreateDraft(ctx, obligation.CreateDraftInput{
		CommandID: "0a111111-1111-4111-8111-111111111111", Direction: "receivable",
		LedgerOwnerPartyID: fixture.owner.ID, CounterpartyPartyID: fixture.counterparty.ID,
		Title: "Second composition receivable", OriginalAmount: "80", Currency: "CNY",
		DueOn: "2026-11-30", ManualSourceReference: "Second composition obligation", Actor: fixture.actor,
	})
	if err != nil {
		t.Fatalf("create second obligation: %v", err)
	}
	secondObligation, err := fixture.obligationService.Confirm(ctx, obligation.ConfirmInput{
		CommandID: "0b111111-1111-4111-8111-111111111111", ObligationID: secondDraft.Obligation.ID,
		ExpectedDraftVersion: secondDraft.Obligation.DraftVersion, Actor: fixture.actor,
	})
	if err != nil {
		t.Fatalf("confirm second obligation: %v", err)
	}
	secondCash, err := fixture.cashService.Create(ctx, cashtransaction.CreateInput{
		CommandID: "0c111111-1111-4111-8111-111111111111", Type: "receipt",
		LedgerOwnerPartyID: fixture.owner.ID, CounterpartyPartyID: fixture.counterparty.ID,
		Description: "Second composition receipt", Amount: "80", Currency: "CNY",
		OccurredOn: "2026-09-20", AccountReference: "Second composition account",
		ManualSourceReference: "Second composition cash", ExternalTransactionReference: "ALLOC-CASH-002", Actor: fixture.actor,
	})
	if err != nil {
		t.Fatalf("create second cash transaction: %v", err)
	}

	var baseTime time.Time
	if err := environment.Pool.QueryRow(ctx, `
		SELECT max(recorded_at)
		FROM obligation_revisions
		WHERE obligation_id IN ($1, $2)
	`, fixture.obligation.ID, secondObligation.Obligation.ID).Scan(&baseTime); err != nil {
		t.Fatalf("read latest obligation recorded time: %v", err)
	}
	applyNumber := 0
	fixture.allocationService.now = func() time.Time {
		applyNumber++
		return baseTime.Add(time.Duration(applyNumber) * time.Second)
	}
	first, err := fixture.allocationService.Apply(ctx, applyInput(fixture, "0d111111-1111-4111-8111-111111111111", "40"))
	if err != nil || first.Replayed {
		t.Fatalf("apply cash A to obligation X = %#v, %v", first, err)
	}
	secondInput := applyInput(fixture, "0e111111-1111-4111-8111-111111111111", "80")
	secondInput.ObligationID = secondObligation.Obligation.ID
	secondInput.ExpectedObligationRevision = secondObligation.Obligation.CurrentRevision
	second, err := fixture.allocationService.Apply(ctx, secondInput)
	if err != nil || second.Replayed {
		t.Fatalf("apply cash A to obligation Y = %#v, %v", second, err)
	}
	thirdInput := applyInput(fixture, "0f111111-1111-4111-8111-111111111111", "60")
	thirdInput.CashTransactionID = secondCash.Transaction.ID
	third, err := fixture.allocationService.Apply(ctx, thirdInput)
	if err != nil || third.Replayed {
		t.Fatalf("apply cash B to obligation X = %#v, %v", third, err)
	}

	assertHistory := func(name string, values []Allocation, err error, wantIDs ...string) {
		t.Helper()
		if err != nil || len(values) != len(wantIDs) {
			t.Fatalf("%s history = %#v, %v; want IDs %#v", name, values, err, wantIDs)
		}
		for index, wantID := range wantIDs {
			if values[index].ID != wantID {
				t.Fatalf("%s history[%d].ID = %s, want %s", name, index, values[index].ID, wantID)
			}
		}
	}
	values, listErr := fixture.allocationService.ListForCashTransaction(ctx, fixture.cash.ID)
	assertHistory("cash A", values, listErr, second.Allocation.ID, first.Allocation.ID)
	values, listErr = fixture.allocationService.ListForCashTransaction(ctx, secondCash.Transaction.ID)
	assertHistory("cash B", values, listErr, third.Allocation.ID)
	values, listErr = fixture.allocationService.ListForObligation(ctx, fixture.obligation.ID)
	assertHistory("obligation X", values, listErr, third.Allocation.ID, first.Allocation.ID)
	values, listErr = fixture.allocationService.ListForObligation(ctx, secondObligation.Obligation.ID)
	assertHistory("obligation Y", values, listErr, second.Allocation.ID)
	values, listErr = fixture.allocationService.List(ctx)
	assertHistory("global", values, listErr, third.Allocation.ID, second.Allocation.ID, first.Allocation.ID)

	firstCash, found, err := fixture.cashService.Get(ctx, fixture.cash.ID)
	if err != nil || !found || firstCash.Amount != "120" || firstCash.AllocatedAmount != "120" || firstCash.UnallocatedAmount != "0" {
		t.Fatalf("cash A balance = %#v, %v, %v", firstCash, found, err)
	}
	secondCashDetails, found, err := fixture.cashService.Get(ctx, secondCash.Transaction.ID)
	if err != nil || !found || secondCashDetails.Amount != "80" || secondCashDetails.AllocatedAmount != "60" || secondCashDetails.UnallocatedAmount != "20" {
		t.Fatalf("cash B balance = %#v, %v, %v", secondCashDetails, found, err)
	}
	firstObligation, found, err := fixture.obligationService.GetDetails(ctx, fixture.obligation.ID)
	if err != nil || !found || firstObligation.Obligation.OriginalAmount != "100" ||
		firstObligation.Obligation.AllocatedAmount != "100" || firstObligation.Obligation.OutstandingAmount != "0" ||
		firstObligation.Obligation.Status != obligation.StatusOpen {
		t.Fatalf("obligation X balance = %#v, %v, %v", firstObligation.Obligation, found, err)
	}
	secondObligationDetails, found, err := fixture.obligationService.GetDetails(ctx, secondObligation.Obligation.ID)
	if err != nil || !found || secondObligationDetails.Obligation.OriginalAmount != "80" ||
		secondObligationDetails.Obligation.AllocatedAmount != "80" || secondObligationDetails.Obligation.OutstandingAmount != "0" ||
		secondObligationDetails.Obligation.Status != obligation.StatusOpen {
		t.Fatalf("obligation Y balance = %#v, %v, %v", secondObligationDetails.Obligation, found, err)
	}

	var allocationCount, commandCount, auditCount int
	var allocationTotal, cashProjectionTotal, obligationProjectionTotal string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM allocations),
			(SELECT count(*) FROM allocation_apply_commands),
			(SELECT count(*) FROM audit_entries WHERE action = 'allocation_applied'),
			(SELECT COALESCE(SUM(amount), 0)::text FROM allocations),
			(SELECT COALESCE(SUM(allocated), 0)::text FROM (
				SELECT COALESCE(SUM(a.amount), 0) AS allocated
				FROM cash_transactions c
				LEFT JOIN allocations a ON a.cash_transaction_id = c.id
				GROUP BY c.id
			) cash_balances),
			(SELECT COALESCE(SUM(allocated), 0)::text FROM (
				SELECT COALESCE(SUM(a.amount), 0) AS allocated
				FROM obligations o
				LEFT JOIN allocations a ON a.obligation_id = o.id
				GROUP BY o.id
			) obligation_balances)
	`).Scan(&allocationCount, &commandCount, &auditCount, &allocationTotal, &cashProjectionTotal, &obligationProjectionTotal); err != nil {
		t.Fatalf("read composition conservation: %v", err)
	}
	if allocationCount != 3 || commandCount != 3 || auditCount != 3 ||
		allocationTotal != "180.000000" || cashProjectionTotal != "180.000000" || obligationProjectionTotal != "180.000000" {
		t.Fatalf("composition facts counts=%d/%d/%d totals=%s/%s/%s",
			allocationCount, commandCount, auditCount, allocationTotal, cashProjectionTotal, obligationProjectionTotal)
	}
}

func TestPostgresAllocationRejectsIncompatibleStaleMissingAndExcess(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	fixture := setupAllocationFixture(t, ctx, environment, "100", "80")

	stale := applyInput(fixture, "72111111-1111-4111-8111-111111111111", "1")
	stale.ExpectedObligationRevision = 2
	_, err := fixture.allocationService.Apply(ctx, stale)
	var revisionConflict *ObligationRevisionConflictError
	if !errors.As(err, &revisionConflict) || revisionConflict.Actual != 1 {
		t.Fatalf("stale revision error = %v", err)
	}

	draft, err := fixture.obligationService.CreateDraft(ctx, obligation.CreateDraftInput{
		CommandID: "82111111-1111-4111-8111-111111111111", Direction: "receivable",
		LedgerOwnerPartyID: fixture.owner.ID, CounterpartyPartyID: fixture.counterparty.ID,
		Title: "Draft cannot allocate", OriginalAmount: "10", Currency: "CNY",
		DueOn: "2026-11-30", ManualSourceReference: "draft", Actor: fixture.actor,
	})
	if err != nil {
		t.Fatal(err)
	}
	draftInput := applyInput(fixture, "92111111-1111-4111-8111-111111111111", "1")
	draftInput.ObligationID = draft.Obligation.ID
	draftInput.ExpectedObligationRevision = 1
	_, err = fixture.allocationService.Apply(ctx, draftInput)
	var stateConflict *ObligationStateConflictError
	if !errors.As(err, &stateConflict) || stateConflict.Status != obligation.StatusDraft {
		t.Fatalf("draft state error = %v", err)
	}

	missingCash := applyInput(fixture, "a2111111-1111-4111-8111-111111111111", "1")
	missingCash.CashTransactionID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	_, err = fixture.allocationService.Apply(ctx, missingCash)
	var cashNotFound *CashTransactionNotFoundError
	if !errors.As(err, &cashNotFound) {
		t.Fatalf("missing cash error = %v", err)
	}
	missingObligation := applyInput(fixture, "b2111111-1111-4111-8111-111111111111", "1")
	missingObligation.ObligationID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	_, err = fixture.allocationService.Apply(ctx, missingObligation)
	var obligationNotFound *ObligationNotFoundError
	if !errors.As(err, &obligationNotFound) {
		t.Fatalf("missing obligation error = %v", err)
	}

	overObligation := applyInput(fixture, "c2111111-1111-4111-8111-111111111111", "80.000001")
	_, err = fixture.allocationService.Apply(ctx, overObligation)
	var obligationBalance *ObligationBalanceConflictError
	if !errors.As(err, &obligationBalance) || obligationBalance.Available != "80" {
		t.Fatalf("obligation excess error = %v", err)
	}

	smallCash, err := fixture.cashService.Create(ctx, cashtransaction.CreateInput{
		CommandID: "d2111111-1111-4111-8111-111111111111", Type: "receipt",
		LedgerOwnerPartyID: fixture.owner.ID, CounterpartyPartyID: fixture.counterparty.ID,
		Description: "Small receipt", Amount: "10", Currency: "CNY", OccurredOn: "2026-09-19",
		AccountReference: "Small account", ManualSourceReference: "Small source", Actor: fixture.actor,
	})
	if err != nil {
		t.Fatal(err)
	}
	overCash := applyInput(fixture, "e2111111-1111-4111-8111-111111111111", "10.000001")
	overCash.CashTransactionID = smallCash.Transaction.ID
	_, err = fixture.allocationService.Apply(ctx, overCash)
	var cashBalance *CashBalanceConflictError
	if !errors.As(err, &cashBalance) || cashBalance.Available != "10" {
		t.Fatalf("cash excess error = %v", err)
	}

	usdCash, err := fixture.cashService.Create(ctx, cashtransaction.CreateInput{
		CommandID: "f2111111-1111-4111-8111-111111111111", Type: "receipt",
		LedgerOwnerPartyID: fixture.owner.ID, CounterpartyPartyID: fixture.counterparty.ID,
		Description: "USD receipt", Amount: "10", Currency: "USD", OccurredOn: "2026-09-19",
		AccountReference: "USD account", ManualSourceReference: "USD source", Actor: fixture.actor,
	})
	if err != nil {
		t.Fatal(err)
	}
	incompatible := applyInput(fixture, "03111111-1111-4111-8111-111111111111", "1")
	incompatible.CashTransactionID = usdCash.Transaction.ID
	_, err = fixture.allocationService.Apply(ctx, incompatible)
	var compatibilityConflict *CompatibilityConflictError
	if !errors.As(err, &compatibilityConflict) || compatibilityConflict.Reason != "currency" {
		t.Fatalf("currency compatibility error = %v", err)
	}

	thirdPartyService := party.NewService(party.NewPostgresRepository(environment.Pool))
	thirdParty, err := thirdPartyService.Create(ctx, party.CreateInput{
		CommandID: "13111111-1111-4111-8111-111111111111", Kind: "individual",
		LegalName: "Third Party", Actor: fixture.actor,
	})
	if err != nil {
		t.Fatal(err)
	}
	thirdPartyCash, err := fixture.cashService.Create(ctx, cashtransaction.CreateInput{
		CommandID: "23111111-1111-4111-8111-111111111111", Type: "receipt",
		LedgerOwnerPartyID: fixture.owner.ID, CounterpartyPartyID: thirdParty.Party.ID,
		Description: "Third party receipt", Amount: "10", Currency: "CNY", OccurredOn: "2026-09-19",
		AccountReference: "Third party account", ManualSourceReference: "Third party source", Actor: fixture.actor,
	})
	if err != nil {
		t.Fatal(err)
	}
	incompatible = applyInput(fixture, "33111111-1111-4111-8111-111111111111", "1")
	incompatible.CashTransactionID = thirdPartyCash.Transaction.ID
	_, err = fixture.allocationService.Apply(ctx, incompatible)
	if !errors.As(err, &compatibilityConflict) || compatibilityConflict.Reason != "parties" {
		t.Fatalf("party compatibility error = %v", err)
	}

	var allocations, commands int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM allocations), (SELECT count(*) FROM allocation_apply_commands)
	`).Scan(&allocations, &commands); err != nil {
		t.Fatal(err)
	}
	if allocations != 0 || commands != 0 {
		t.Fatalf("failed attempts left allocations=%d commands=%d", allocations, commands)
	}
}

func TestPostgresAllocationConcurrency(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	fixture := setupAllocationFixture(t, ctx, environment, "100", "100")
	type result struct {
		value ApplyResult
		err   error
	}
	run := func(inputs []ApplyInput) []result {
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
				value, err := fixture.allocationService.Apply(ctx, input)
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

	same := applyInput(fixture, "43111111-1111-4111-8111-111111111111", "10")
	sameResults := run([]ApplyInput{same, same})
	replayCount := 0
	var allocationID string
	for _, current := range sameResults {
		if current.err != nil {
			t.Fatalf("same command Apply() error = %v", current.err)
		}
		if allocationID == "" {
			allocationID = current.value.Allocation.ID
		}
		if current.value.Allocation.ID != allocationID {
			t.Fatalf("same command allocation IDs differ: %s %s", allocationID, current.value.Allocation.ID)
		}
		if current.value.Replayed {
			replayCount++
		}
	}
	if replayCount != 1 {
		t.Fatalf("same command replay count = %d", replayCount)
	}

	first := applyInput(fixture, "53111111-1111-4111-8111-111111111111", "60")
	second := applyInput(fixture, "63111111-1111-4111-8111-111111111111", "60")
	competing := run([]ApplyInput{first, second})
	successes, conflicts := 0, 0
	for _, current := range competing {
		if current.err == nil {
			successes++
			continue
		}
		var cashConflict *CashBalanceConflictError
		if errors.As(current.err, &cashConflict) {
			conflicts++
			continue
		}
		t.Fatalf("unexpected competing error = %v", current.err)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("competing results successes=%d conflicts=%d", successes, conflicts)
	}
	cashAfter, _, _ := fixture.cashService.Get(ctx, fixture.cash.ID)
	obligationAfter, _, _ := fixture.obligationService.GetDetails(ctx, fixture.obligation.ID)
	if cashAfter.AllocatedAmount != "70" || cashAfter.UnallocatedAmount != "30" ||
		obligationAfter.Obligation.AllocatedAmount != "70" || obligationAfter.Obligation.OutstandingAmount != "30" {
		t.Fatalf("concurrent balances cash=%#v obligation=%#v", cashAfter, obligationAfter.Obligation)
	}
}

func TestPostgresAllocationConcurrencyProtectsEachBalanceIndependently(t *testing.T) {
	t.Run("same cash different obligations", func(t *testing.T) {
		environment := testdatabase.New(t, testdatabase.RequireURL(t))
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := database.Migrate(ctx, environment.Pool); err != nil {
			t.Fatal(err)
		}
		fixture := setupAllocationFixture(t, ctx, environment, "100", "100")
		secondDraft, err := fixture.obligationService.CreateDraft(ctx, obligation.CreateDraftInput{
			CommandID: "75111111-1111-4111-8111-111111111111", Direction: "receivable",
			LedgerOwnerPartyID: fixture.owner.ID, CounterpartyPartyID: fixture.counterparty.ID,
			Title: "Second receivable", OriginalAmount: "100", Currency: "CNY",
			DueOn: "2026-11-30", ManualSourceReference: "Second obligation", Actor: fixture.actor,
		})
		if err != nil {
			t.Fatal(err)
		}
		secondConfirmed, err := fixture.obligationService.Confirm(ctx, obligation.ConfirmInput{
			CommandID: "85111111-1111-4111-8111-111111111111", ObligationID: secondDraft.Obligation.ID,
			ExpectedDraftVersion: 1, Actor: fixture.actor,
		})
		if err != nil {
			t.Fatal(err)
		}
		first := applyInput(fixture, "95111111-1111-4111-8111-111111111111", "60")
		second := first
		second.CommandID = "a5111111-1111-4111-8111-111111111111"
		second.ObligationID = secondConfirmed.Obligation.ID
		results := runConcurrentAllocationInputs(ctx, fixture.allocationService, []ApplyInput{first, second})
		assertOneBalanceConflict(t, results, true)
		cashAfter, _, _ := fixture.cashService.Get(ctx, fixture.cash.ID)
		if cashAfter.AllocatedAmount != "60" || cashAfter.UnallocatedAmount != "40" {
			t.Fatalf("cash balance = %#v", cashAfter)
		}
	})

	t.Run("different cash same obligation", func(t *testing.T) {
		environment := testdatabase.New(t, testdatabase.RequireURL(t))
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := database.Migrate(ctx, environment.Pool); err != nil {
			t.Fatal(err)
		}
		fixture := setupAllocationFixture(t, ctx, environment, "100", "100")
		secondCash, err := fixture.cashService.Create(ctx, cashtransaction.CreateInput{
			CommandID: "b5111111-1111-4111-8111-111111111111", Type: "receipt",
			LedgerOwnerPartyID: fixture.owner.ID, CounterpartyPartyID: fixture.counterparty.ID,
			Description: "Second receipt", Amount: "100", Currency: "CNY", OccurredOn: "2026-09-19",
			AccountReference: "Second account", ManualSourceReference: "Second cash", Actor: fixture.actor,
		})
		if err != nil {
			t.Fatal(err)
		}
		first := applyInput(fixture, "c5111111-1111-4111-8111-111111111111", "60")
		second := first
		second.CommandID = "d5111111-1111-4111-8111-111111111111"
		second.CashTransactionID = secondCash.Transaction.ID
		results := runConcurrentAllocationInputs(ctx, fixture.allocationService, []ApplyInput{first, second})
		assertOneBalanceConflict(t, results, false)
		obligationAfter, _, _ := fixture.obligationService.GetDetails(ctx, fixture.obligation.ID)
		if obligationAfter.Obligation.AllocatedAmount != "60" || obligationAfter.Obligation.OutstandingAmount != "40" {
			t.Fatalf("obligation balance = %#v", obligationAfter.Obligation)
		}
	})
}

type concurrentAllocationResult struct {
	value ApplyResult
	err   error
}

func runConcurrentAllocationInputs(ctx context.Context, service *Service, inputs []ApplyInput) []concurrentAllocationResult {
	start := make(chan struct{})
	results := make(chan concurrentAllocationResult, len(inputs))
	var workers sync.WaitGroup
	for _, input := range inputs {
		input := input
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			value, err := service.Apply(ctx, input)
			results <- concurrentAllocationResult{value: value, err: err}
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	values := make([]concurrentAllocationResult, 0, len(inputs))
	for value := range results {
		values = append(values, value)
	}
	return values
}

func assertOneBalanceConflict(t *testing.T, results []concurrentAllocationResult, expectCashConflict bool) {
	t.Helper()
	successes, conflicts := 0, 0
	for _, current := range results {
		if current.err == nil {
			successes++
			continue
		}
		if expectCashConflict {
			var conflict *CashBalanceConflictError
			if errors.As(current.err, &conflict) {
				conflicts++
				continue
			}
		} else {
			var conflict *ObligationBalanceConflictError
			if errors.As(current.err, &conflict) {
				conflicts++
				continue
			}
		}
		t.Fatalf("unexpected concurrent error = %v", current.err)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent results successes=%d conflicts=%d", successes, conflicts)
	}
}

func TestPostgresAllocationRollbackTriggerAndImmutability(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	fixture := setupAllocationFixture(t, ctx, environment, "100", "80")

	for _, statement := range []string{
		`INSERT INTO allocations (
			id, cash_transaction_id, obligation_id, obligation_revision,
			amount, currency, source_kind, manual_source_reference,
			applied_by_actor_kind, applied_by_actor_id, applied_by_actor_display_name, applied_at
		) VALUES (
			'74111111-1111-4111-8111-111111111111', $1, $2, 1,
			1000, 'CNY', 'manual', '直接超额', 'local_user', $3, '直接写入', clock_timestamp()
		)`,
		`INSERT INTO allocations (
			id, cash_transaction_id, obligation_id, obligation_revision,
			amount, currency, source_kind, manual_source_reference,
			applied_by_actor_kind, applied_by_actor_id, applied_by_actor_display_name, applied_at
		) VALUES (
			'84111111-1111-4111-8111-111111111111', $1, $2, 1,
			1, 'USD', 'manual', '直接错币种', 'local_user', $3, '直接写入', clock_timestamp()
		)`,
	} {
		_, err := environment.Pool.Exec(ctx, statement, fixture.cash.ID, fixture.obligation.ID, fixture.actor.ID)
		var databaseError *pgconn.PgError
		if !errors.As(err, &databaseError) || databaseError.Code != "23514" {
			t.Fatalf("direct invalid allocation error = %v", err)
		}
	}

	rollbackCommand := "94111111-1111-4111-8111-111111111111"
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO audit_entries (
			id, object_type, object_id, action, command_id,
			actor_kind, actor_id, actor_display_name, occurred_at, summary, details
		) VALUES (
			'a4111111-1111-4111-8111-111111111111', 'allocation',
			'b4111111-1111-4111-8111-111111111111', 'allocation_applied', $1,
			'local_user', $2, '预占审计', clock_timestamp(), '预占审计', '{}'::jsonb
		)
	`, rollbackCommand, fixture.actor.ID); err != nil {
		t.Fatalf("insert audit failure fixture: %v", err)
	}
	_, err := fixture.allocationService.Apply(ctx, applyInput(fixture, rollbackCommand, "10"))
	if err == nil {
		t.Fatal("Apply() unexpectedly succeeded with audit collision")
	}
	var rolledBackAllocations, rolledBackCommands int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM allocations),
			(SELECT count(*) FROM allocation_apply_commands WHERE command_id = $1)
	`, rollbackCommand).Scan(&rolledBackAllocations, &rolledBackCommands); err != nil {
		t.Fatal(err)
	}
	if rolledBackAllocations != 0 || rolledBackCommands != 0 {
		t.Fatalf("audit failure left allocations=%d commands=%d", rolledBackAllocations, rolledBackCommands)
	}

	commandID := "c4111111-1111-4111-8111-111111111111"
	created, err := fixture.allocationService.Apply(ctx, applyInput(fixture, commandID, "10"))
	if err != nil {
		t.Fatalf("create allocation for immutability: %v", err)
	}
	for _, statement := range []struct {
		query string
		id    string
	}{
		{query: `UPDATE allocations SET amount = 9 WHERE id = $1`, id: created.Allocation.ID},
		{query: `DELETE FROM allocations WHERE id = $1`, id: created.Allocation.ID},
		{query: `UPDATE allocation_apply_commands SET actor_display_name = 'changed' WHERE command_id = $1`, id: commandID},
		{query: `DELETE FROM allocation_apply_commands WHERE command_id = $1`, id: commandID},
	} {
		_, err := environment.Pool.Exec(ctx, statement.query, statement.id)
		var databaseError *pgconn.PgError
		if !errors.As(err, &databaseError) || databaseError.Code != "55000" {
			t.Fatalf("immutable statement error = %v", err)
		}
	}
}
