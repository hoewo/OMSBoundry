//go:build integration

package internaltransfer

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
	"omsbizfoundry/money-flow-ledger/internal/party"
	"omsbizfoundry/money-flow-ledger/internal/testdatabase"
)

func TestPostgresInternalTransferSnapshotsIdempotencyScopeAndOperatingIsolation(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "内部转账管理员",
	}
	owner := createTestParty(t, ctx, environment, actor, "22222222-2222-4222-8222-222222222222", "Owner Company", "OWNER-001")
	counterparty := createTestParty(t, ctx, environment, actor, "33333333-3333-4333-8333-333333333333", "Counterparty", "")
	otherOwner := createTestParty(t, ctx, environment, actor, "44444444-4444-4444-8444-444444444444", "Other Owner", "OTHER-001")

	cashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	for _, input := range []cashtransaction.CreateInput{
		{
			CommandID: "55555555-5555-4555-8555-555555555555", Type: "receipt",
			LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
			Description: "经营收款", Amount: "125.5", Currency: "CNY", OccurredOn: "2026-09-18",
			AccountReference: "基本户", ManualSourceReference: "经营收款回单", Actor: actor,
		},
		{
			CommandID: "66666666-6666-4666-8666-666666666666", Type: "payment",
			LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
			Description: "经营付款", Amount: "25.25", Currency: "CNY", OccurredOn: "2026-09-18",
			AccountReference: "基本户", ManualSourceReference: "经营付款回单", Actor: actor,
		},
	} {
		if _, err := cashService.Create(ctx, input); err != nil {
			t.Fatalf("create cash fixture: %v", err)
		}
	}

	service := NewService(NewPostgresRepository(environment.Pool))
	firstTime := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return firstTime }
	input := CreateInput{
		CommandID: "77777777-7777-4777-8777-777777777777", LedgerOwnerPartyID: owner.ID,
		Description: " 基本户向备用金账户转账 ", Amount: "000100.500000", Currency: " cny ", OccurredOn: "2026-09-19",
		SourceAccountReference: " 基本户 ABC ", DestinationAccountReference: " 备用金账户 ",
		ManualSourceReference: " 银行回单 TRANSFER-001 ", ExternalTransferReference: " Bank-Transfer-001 ", Actor: actor,
	}
	created, err := service.Create(ctx, input)
	if err != nil || created.Replayed {
		t.Fatalf("Create() = %#v, %v", created, err)
	}
	value := created.Transfer
	if value.LedgerOwner.PartyID != owner.ID || value.LedgerOwner.PartyRevision != 1 ||
		value.LedgerOwner.LegalName != "Owner Company" || value.LedgerOwner.RegistrationIdentifier != "OWNER-001" ||
		value.Amount != "100.5" || value.SourceAccountReference != "基本户 ABC" || value.DestinationAccountReference != "备用金账户" ||
		value.Status != StatusRecorded || value.RecordedBy != actor {
		t.Fatalf("transfer = %#v", value)
	}
	listed, err := service.List(ctx)
	if err != nil || len(listed) != 1 || listed[0].ID != value.ID {
		t.Fatalf("List() = %#v, %v", listed, err)
	}
	got, found, err := service.Get(ctx, value.ID)
	if err != nil || !found || got.ID != value.ID {
		t.Fatalf("Get() = %#v, %v, %v", got, found, err)
	}

	if _, err := environment.Pool.Exec(ctx, `
		UPDATE parties
		SET legal_name = 'Owner Company Changed', revision = 2, updated_at = clock_timestamp()
		WHERE id = $1
	`, owner.ID); err != nil {
		t.Fatalf("update current Party: %v", err)
	}
	got, found, err = service.Get(ctx, value.ID)
	if err != nil || !found || got.LedgerOwner.LegalName != "Owner Company" || got.LedgerOwner.PartyRevision != 1 {
		t.Fatalf("snapshot after Party update = %#v, %v, %v", got.LedgerOwner, found, err)
	}

	replay := input
	replay.Description = "基本户向备用金账户转账"
	replay.Amount = "100.5"
	replay.Currency = "CNY"
	replay.SourceAccountReference = "基本户 abc"
	replay.DestinationAccountReference = "备用金账户"
	replay.ManualSourceReference = "银行回单 TRANSFER-001"
	replay.ExternalTransferReference = "bank-transfer-001"
	replayed, err := service.Create(ctx, replay)
	if err != nil || !replayed.Replayed || replayed.Transfer.ID != value.ID || replayed.Transfer.SourceAccountReference != "基本户 ABC" {
		t.Fatalf("replay = %#v, %v", replayed, err)
	}
	changedPayload := replay
	changedPayload.Amount = "101"
	_, err = service.Create(ctx, changedPayload)
	var idempotencyConflict *IdempotencyConflictError
	if !errors.As(err, &idempotencyConflict) {
		t.Fatalf("changed payload error = %v", err)
	}
	changedActor := replay
	changedActor.Actor.ID = "88888888-8888-4888-8888-888888888888"
	_, err = service.Create(ctx, changedActor)
	if !errors.As(err, &idempotencyConflict) {
		t.Fatalf("changed actor error = %v", err)
	}

	duplicate := replay
	duplicate.CommandID = "99999999-9999-4999-8999-999999999999"
	_, err = service.Create(ctx, duplicate)
	var externalConflict *ExternalReferenceConflictError
	if !errors.As(err, &externalConflict) || externalConflict.ExistingTransferID != value.ID {
		t.Fatalf("duplicate external reference error = %v", err)
	}

	reversed := replay
	reversed.CommandID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	reversed.Description = "反向账户转账"
	reversed.SourceAccountReference, reversed.DestinationAccountReference = reversed.DestinationAccountReference, reversed.SourceAccountReference
	if _, err := service.Create(ctx, reversed); err != nil {
		t.Fatalf("same external reference in reversed account direction: %v", err)
	}
	differentOwner := replay
	differentOwner.CommandID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	differentOwner.LedgerOwnerPartyID = otherOwner.ID
	differentOwner.Description = "另一账簿主体同流水"
	if _, err := service.Create(ctx, differentOwner); err != nil {
		t.Fatalf("same external reference for different owner: %v", err)
	}

	missingCommandID := "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	missing := replay
	missing.CommandID = missingCommandID
	missing.LedgerOwnerPartyID = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	missing.ExternalTransferReference = "MISSING-PARTY"
	_, err = service.Create(ctx, missing)
	var partyNotFound *PartyNotFoundError
	if !errors.As(err, &partyNotFound) {
		t.Fatalf("missing Party error = %v", err)
	}

	var transfers, commands, audits, missingCommands, allocations, cashTransactions int
	var receipts, payments, receiptTotal, paymentTotal string
	var auditActorKind, auditActorID, auditActorName, classification string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM internal_transfers),
			(SELECT count(*) FROM internal_transfer_create_commands),
			(SELECT count(*) FROM audit_entries WHERE action = 'internal_transfer_recorded'),
			(SELECT count(*) FROM internal_transfer_create_commands WHERE command_id = $1),
			(SELECT count(*) FROM allocations),
			(SELECT count(*) FROM cash_transactions),
			(SELECT count(*)::text FROM cash_transactions WHERE type = 'receipt'),
			(SELECT count(*)::text FROM cash_transactions WHERE type = 'payment'),
			(SELECT COALESCE(sum(amount), 0)::text FROM cash_transactions WHERE type = 'receipt'),
			(SELECT COALESCE(sum(amount), 0)::text FROM cash_transactions WHERE type = 'payment'),
			actor_kind, actor_id, actor_display_name,
			details->>'operating_cash_flow_classification'
		FROM audit_entries
		WHERE action = 'internal_transfer_recorded' AND command_id = '77777777-7777-4777-8777-777777777777'
	`, missingCommandID).Scan(
		&transfers, &commands, &audits, &missingCommands, &allocations, &cashTransactions,
		&receipts, &payments, &receiptTotal, &paymentTotal,
		&auditActorKind, &auditActorID, &auditActorName, &classification,
	); err != nil {
		t.Fatalf("read internal transfer facts: %v", err)
	}
	if transfers != 3 || commands != 3 || audits != 3 || missingCommands != 0 || allocations != 0 || cashTransactions != 2 ||
		receipts != "1" || payments != "1" || receiptTotal != "125.500000" || paymentTotal != "25.250000" ||
		auditActorKind != actor.Kind || auditActorID != actor.ID || auditActorName != actor.DisplayName ||
		classification != "excluded_internal_transfer" {
		t.Fatalf("facts transfers=%d commands=%d audits=%d missing=%d allocations=%d cash=%d receipt=%s/%s payment=%s/%s actor=%s/%s/%s class=%s",
			transfers, commands, audits, missingCommands, allocations, cashTransactions,
			receipts, receiptTotal, payments, paymentTotal, auditActorKind, auditActorID, auditActorName, classification)
	}
}

func TestPostgresInternalTransferConcurrency(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "并发转账管理员"}
	owner := createTestParty(t, ctx, environment, actor, "22222222-2222-4222-8222-222222222222", "Concurrent Owner", "")
	service := NewService(NewPostgresRepository(environment.Pool))
	base := CreateInput{
		LedgerOwnerPartyID: owner.ID, Description: "并发内部转账", Amount: "20", Currency: "CNY", OccurredOn: "2026-09-19",
		SourceAccountReference: "账户 A", DestinationAccountReference: "账户 B",
		ManualSourceReference: "并发测试依据", ExternalTransferReference: "CONCURRENT-001", Actor: actor,
	}
	type result struct {
		value CreateResult
		err   error
	}
	run := func(inputs []CreateInput) []result {
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
				value, err := service.Create(ctx, input)
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

	same := base
	same.CommandID = "33333333-3333-4333-8333-333333333333"
	sameResults := run([]CreateInput{same, same})
	replays := 0
	var transferID string
	for _, item := range sameResults {
		if item.err != nil {
			t.Fatalf("same command error = %v", item.err)
		}
		if transferID == "" {
			transferID = item.value.Transfer.ID
		}
		if item.value.Transfer.ID != transferID {
			t.Fatalf("same command IDs differ: %s %s", transferID, item.value.Transfer.ID)
		}
		if item.value.Replayed {
			replays++
		}
	}
	if replays != 1 {
		t.Fatalf("same command replay count = %d", replays)
	}

	first := base
	first.CommandID = "44444444-4444-4444-8444-444444444444"
	first.ExternalTransferReference = "CONCURRENT-002"
	second := first
	second.CommandID = "55555555-5555-4555-8555-555555555555"
	results := run([]CreateInput{first, second})
	successes, conflicts := 0, 0
	for _, item := range results {
		if item.err == nil {
			successes++
			continue
		}
		var conflict *ExternalReferenceConflictError
		if errors.As(item.err, &conflict) {
			conflicts++
			continue
		}
		t.Fatalf("unexpected concurrent error = %v", item.err)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent external scope successes=%d conflicts=%d", successes, conflicts)
	}

	var transfers, commands, audits int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM internal_transfers),
			(SELECT count(*) FROM internal_transfer_create_commands),
			(SELECT count(*) FROM audit_entries WHERE action = 'internal_transfer_recorded')
	`).Scan(&transfers, &commands, &audits); err != nil {
		t.Fatalf("read concurrent facts: %v", err)
	}
	if transfers != 2 || commands != 2 || audits != 2 {
		t.Fatalf("concurrent facts transfers=%d commands=%d audits=%d", transfers, commands, audits)
	}
}

func TestPostgresInternalTransferRollbackConstraintsAndImmutability(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "回滚转账管理员"}
	owner := createTestParty(t, ctx, environment, actor, "22222222-2222-4222-8222-222222222222", "Rollback Owner", "")
	repository := NewPostgresRepository(environment.Pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	transferID := "33333333-3333-4333-8333-333333333333"
	commandID := "44444444-4444-4444-8444-444444444444"
	_, _, err := repository.Create(ctx, Transfer{
		ID: transferID, Description: "应整体回滚的内部转账", LedgerOwner: PartySnapshot{PartyID: owner.ID},
		Amount: "100", Currency: "CNY", OccurredOn: "2026-09-19",
		SourceAccountReference: "账户 A", DestinationAccountReference: "账户 B", SourceKind: SourceKindManual,
		ManualSourceReference: "回滚测试依据", ExternalTransferReference: "ROLLBACK-001",
		Status: StatusRecorded, RecordedBy: actor, RecordedAt: now,
	}, CreateCommand{
		CommandID: commandID, PayloadHash: make([]byte, 32), Actor: actor, CreatedAt: now,
	}, audit.Entry{
		ID: "55555555-5555-4555-8555-555555555555", ObjectType: audit.ObjectInternalTransfer, ObjectID: transferID,
		Action: AuditActionRecorded, CommandID: commandID, Actor: actor, OccurredAt: now,
		Summary: "", Details: map[string]any{},
	})
	if err == nil {
		t.Fatal("Create() error = nil")
	}
	var transfers, commands, audits int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM internal_transfers WHERE id = $1),
			(SELECT count(*) FROM internal_transfer_create_commands WHERE command_id = $2),
			(SELECT count(*) FROM audit_entries WHERE command_id = $2)
	`, transferID, commandID).Scan(&transfers, &commands, &audits); err != nil {
		t.Fatalf("read rollback facts: %v", err)
	}
	if transfers != 0 || commands != 0 || audits != 0 {
		t.Fatalf("rollback facts transfers=%d commands=%d audits=%d", transfers, commands, audits)
	}

	service := NewService(repository)
	created, err := service.Create(ctx, CreateInput{
		CommandID: "66666666-6666-4666-8666-666666666666", LedgerOwnerPartyID: owner.ID,
		Description: "不可变内部转账", Amount: "1", Currency: "CNY", OccurredOn: "2026-09-19",
		SourceAccountReference: "账户 A", DestinationAccountReference: "账户 B",
		ManualSourceReference: "不可变依据", ExternalTransferReference: "IMMUTABLE-001", Actor: actor,
	})
	if err != nil {
		t.Fatalf("service.Create() error = %v", err)
	}
	for _, statement := range []string{
		`UPDATE internal_transfers SET description = 'changed' WHERE id = $1`,
		`DELETE FROM internal_transfers WHERE id = $1`,
		`UPDATE internal_transfer_create_commands SET actor_display_name = 'changed' WHERE internal_transfer_id = $1`,
		`DELETE FROM internal_transfer_create_commands WHERE internal_transfer_id = $1`,
	} {
		_, err := environment.Pool.Exec(ctx, statement, created.Transfer.ID)
		var databaseError *pgconn.PgError
		if !errors.As(err, &databaseError) || databaseError.Code != "55000" {
			t.Fatalf("immutable statement error = %v", err)
		}
	}

	_, err = environment.Pool.Exec(ctx, `
		INSERT INTO internal_transfers (
			id, description,
			ledger_owner_party_id, ledger_owner_party_revision, ledger_owner_kind, ledger_owner_legal_name,
			amount, currency, occurred_on,
			source_account_reference, destination_account_reference,
			source_kind, manual_source_reference,
			recorded_by_actor_kind, recorded_by_actor_id, recorded_by_actor_display_name, recorded_at
		) VALUES (
			'77777777-7777-4777-8777-777777777777', '非法同账户内部转账',
			$1, 1, 'organization', 'Rollback Owner',
			1, 'CNY', '2026-09-19', 'Same Account', 'same account',
			'manual', '直接数据库约束测试', $2, $3, $4, $5
		)
	`, owner.ID, actor.Kind, actor.ID, actor.DisplayName, now)
	var constraintError *pgconn.PgError
	if !errors.As(err, &constraintError) || constraintError.Code != "23514" {
		t.Fatalf("same-account constraint error = %v", err)
	}

	var allocationReferences int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_constraint
		WHERE conrelid IN ('allocations'::regclass, 'allocation_apply_commands'::regclass)
		  AND confrelid = 'internal_transfers'::regclass
	`).Scan(&allocationReferences); err != nil {
		t.Fatalf("inspect Allocation relationships: %v", err)
	}
	if allocationReferences != 0 {
		t.Fatalf("Allocation unexpectedly references internal transfers: %d", allocationReferences)
	}
}

func createTestParty(
	t *testing.T,
	ctx context.Context,
	environment testdatabase.Environment,
	actor audit.ActorRef,
	commandID string,
	legalName string,
	registrationIdentifier string,
) party.Party {
	t.Helper()
	service := party.NewService(party.NewPostgresRepository(environment.Pool))
	created, err := service.Create(ctx, party.CreateInput{
		CommandID: commandID, Kind: "organization", LegalName: legalName,
		RegistrationIdentifier: registrationIdentifier, Actor: actor,
	})
	if err != nil {
		t.Fatalf("create test Party %s: %v", legalName, err)
	}
	return created.Party
}
