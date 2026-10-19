//go:build integration

package obligation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/database"
	"omsbizfoundry/money-flow-ledger/internal/party"
	"omsbizfoundry/money-flow-ledger/internal/testdatabase"
)

func TestPostgresObligationDraftSnapshotsIdempotencyAndConcurrency(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "义务联调管理员",
	}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	service := NewService(NewPostgresRepository(environment.Pool))
	firstTime := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return firstTime }

	receivable, err := service.CreateDraft(ctx, CreateDraftInput{
		CommandID: "44444444-4444-4444-8444-444444444444", Direction: " receivable ",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Title: " 九月服务费 ", OriginalAmount: "000100.500000", Currency: " cny ",
		DueOn: "2026-10-31", ManualSourceReference: " 人工确认单 OBL-001 ", Actor: actor,
	})
	if err != nil || receivable.Replayed {
		t.Fatalf("create receivable = %#v, %v", receivable, err)
	}
	value := receivable.Obligation
	if value.Direction != DirectionReceivable || value.Creditor.PartyID != owner.ID || value.Debtor.PartyID != counterparty.ID ||
		value.Creditor.LegalName != "Owner Company" || value.Creditor.RegistrationIdentifier != "OWNER-001" ||
		value.Debtor.LegalName != "Counter Person" || value.Debtor.RegistrationIdentifier != "" ||
		value.Creditor.PartyRevision != 1 || value.Debtor.PartyRevision != 1 || value.OriginalAmount != "100.5" {
		t.Fatalf("receivable = %#v", value)
	}

	secondTime := firstTime.Add(time.Minute)
	service.now = func() time.Time { return secondTime }
	payable, err := service.CreateDraft(ctx, CreateDraftInput{
		CommandID: "55555555-5555-4555-8555-555555555555", Direction: "payable",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Title: "代办费用", OriginalAmount: "0.000001", Currency: "USD",
		DueOn: "2026-09-01", ManualSourceReference: "历史人工依据 OBL-002", Actor: actor,
	})
	if err != nil || payable.Replayed {
		t.Fatalf("create payable = %#v, %v", payable, err)
	}
	if payable.Obligation.Creditor.PartyID != counterparty.ID || payable.Obligation.Debtor.PartyID != owner.ID ||
		payable.Obligation.OriginalAmount != "0.000001" {
		t.Fatalf("payable = %#v", payable.Obligation)
	}
	listed, err := service.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listed) != 2 || listed[0].ID != payable.Obligation.ID || listed[1].ID != receivable.Obligation.ID {
		t.Fatalf("listed = %#v", listed)
	}
	got, found, err := service.Get(ctx, receivable.Obligation.ID)
	if err != nil || !found || got.ID != receivable.Obligation.ID {
		t.Fatalf("Get() = %#v, %v, %v", got, found, err)
	}

	if _, err := environment.Pool.Exec(ctx, `
		UPDATE parties
		SET legal_name = 'Owner Company Changed', revision = 2, updated_at = clock_timestamp()
		WHERE id = $1
	`, owner.ID); err != nil {
		t.Fatalf("update current party: %v", err)
	}
	got, found, err = service.Get(ctx, receivable.Obligation.ID)
	if err != nil || !found || got.Creditor.LegalName != "Owner Company" || got.Creditor.PartyRevision != 1 {
		t.Fatalf("snapshot after Party update = %#v, %v, %v", got.Creditor, found, err)
	}

	replayed, err := service.CreateDraft(ctx, CreateDraftInput{
		CommandID: "44444444-4444-4444-8444-444444444444", Direction: "RECEIVABLE",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Title: "九月服务费", OriginalAmount: "100.5", Currency: "CNY",
		DueOn: "2026-10-31", ManualSourceReference: "人工确认单 OBL-001", Actor: actor,
	})
	if err != nil || !replayed.Replayed || replayed.Obligation.ID != receivable.Obligation.ID ||
		replayed.Obligation.Creditor.LegalName != "Owner Company" {
		t.Fatalf("replay = %#v, %v", replayed, err)
	}
	_, err = service.CreateDraft(ctx, CreateDraftInput{
		CommandID: "44444444-4444-4444-8444-444444444444", Direction: "receivable",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Title: "changed", OriginalAmount: "100.5", Currency: "CNY",
		DueOn: "2026-10-31", ManualSourceReference: "人工确认单 OBL-001", Actor: actor,
	})
	var conflict *IdempotencyConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("changed payload error = %v", err)
	}
	changedActor := actor
	changedActor.ID = "66666666-6666-4666-8666-666666666666"
	_, err = service.CreateDraft(ctx, CreateDraftInput{
		CommandID: "44444444-4444-4444-8444-444444444444", Direction: "receivable",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Title: "九月服务费", OriginalAmount: "100.5", Currency: "CNY",
		DueOn: "2026-10-31", ManualSourceReference: "人工确认单 OBL-001", Actor: changedActor,
	})
	if !errors.As(err, &conflict) {
		t.Fatalf("changed actor error = %v", err)
	}

	const concurrentCommandID = "77777777-7777-4777-8777-777777777777"
	start := make(chan struct{})
	type concurrentResult struct {
		result CreateResult
		err    error
	}
	results := make(chan concurrentResult, 2)
	var workers sync.WaitGroup
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			result, createErr := service.CreateDraft(ctx, CreateDraftInput{
				CommandID: concurrentCommandID, Direction: "receivable",
				LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
				Title: "并发义务", OriginalAmount: "20", Currency: "CNY",
				DueOn: "2026-11-01", ManualSourceReference: "并发人工依据", Actor: actor,
			})
			results <- concurrentResult{result: result, err: createErr}
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	var concurrentObligationID string
	replayCount := 0
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent CreateDraft() error = %v", result.err)
		}
		if concurrentObligationID == "" {
			concurrentObligationID = result.result.Obligation.ID
		}
		if result.result.Obligation.ID != concurrentObligationID {
			t.Fatalf("concurrent IDs differ: %s and %s", concurrentObligationID, result.result.Obligation.ID)
		}
		if result.result.Replayed {
			replayCount++
		}
	}
	if replayCount != 1 {
		t.Fatalf("concurrent replay count = %d", replayCount)
	}

	missingCommandID := "88888888-8888-4888-8888-888888888888"
	_, err = service.CreateDraft(ctx, CreateDraftInput{
		CommandID: missingCommandID, Direction: "receivable",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: "99999999-9999-4999-8999-999999999999",
		Title: "缺失主体", OriginalAmount: "10", Currency: "CNY",
		DueOn: "2026-11-01", ManualSourceReference: "缺失主体依据", Actor: actor,
	})
	var partyNotFound *PartyNotFoundError
	if !errors.As(err, &partyNotFound) {
		t.Fatalf("missing party error = %v", err)
	}

	var obligations, commands, audits, missingCommands int
	var auditActorKind, auditActorID, auditActorName string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM obligations),
			(SELECT count(*) FROM obligation_create_commands),
			(SELECT count(*) FROM audit_entries WHERE action = 'obligation_draft_created'),
			(SELECT count(*) FROM obligation_create_commands WHERE command_id = $1),
			actor_kind, actor_id, actor_display_name
		FROM audit_entries
		WHERE action = 'obligation_draft_created' AND command_id = '44444444-4444-4444-8444-444444444444'
	`, missingCommandID).Scan(
		&obligations, &commands, &audits, &missingCommands,
		&auditActorKind, &auditActorID, &auditActorName,
	); err != nil {
		t.Fatalf("read obligation facts: %v", err)
	}
	if obligations != 3 || commands != 3 || audits != 3 || missingCommands != 0 ||
		auditActorKind != actor.Kind || auditActorID != actor.ID || auditActorName != actor.DisplayName {
		t.Fatalf("facts obligations=%d commands=%d audits=%d missing=%d actor=%s/%s/%s",
			obligations, commands, audits, missingCommands, auditActorKind, auditActorID, auditActorName)
	}
}

func TestPostgresObligationDraftRollsBackWithAuditAndProtectsCommand(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "回滚管理员",
	}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	repository := NewPostgresRepository(environment.Pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	obligationID := "44444444-4444-4444-8444-444444444444"
	commandID := "55555555-5555-4555-8555-555555555555"
	_, _, err := repository.CreateDraft(ctx, Obligation{
		ID: obligationID, Direction: DirectionReceivable, Title: "应整体回滚的义务",
		LedgerOwnerPartyID:    owner.ID,
		Creditor:              PartySnapshot{PartyID: owner.ID},
		Debtor:                PartySnapshot{PartyID: counterparty.ID},
		OriginalAmount:        "100",
		Currency:              "CNY",
		DueOn:                 "2026-10-31",
		SourceKind:            SourceKindManual,
		ManualSourceReference: "回滚依据",
		Status:                StatusDraft, DraftVersion: 1, CreatedAt: now, UpdatedAt: now,
	}, CreateCommand{
		CommandID: commandID, PayloadHash: make([]byte, 32), Actor: actor, CreatedAt: now,
	}, audit.Entry{
		ID: "66666666-6666-4666-8666-666666666666", ObjectType: audit.ObjectObligation, ObjectID: obligationID,
		Action: AuditActionDraftCreated, CommandID: commandID, Actor: actor, OccurredAt: now,
		Summary: "", Details: map[string]any{},
	})
	if err == nil {
		t.Fatal("CreateDraft() error = nil")
	}
	var obligations, commands, audits int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM obligations WHERE id = $1),
			(SELECT count(*) FROM obligation_create_commands WHERE command_id = $2),
			(SELECT count(*) FROM audit_entries WHERE command_id = $2)
	`, obligationID, commandID).Scan(&obligations, &commands, &audits); err != nil {
		t.Fatalf("read rollback facts: %v", err)
	}
	if obligations != 0 || commands != 0 || audits != 0 {
		t.Fatalf("rollback facts obligations=%d commands=%d audits=%d", obligations, commands, audits)
	}

	service := NewService(repository)
	created, err := service.CreateDraft(ctx, CreateDraftInput{
		CommandID: "77777777-7777-4777-8777-777777777777", Direction: "payable",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Title: "不可变命令义务", OriginalAmount: "1", Currency: "CNY",
		DueOn: "2026-10-31", ManualSourceReference: "不可变命令依据", Actor: actor,
	})
	if err != nil {
		t.Fatalf("service.CreateDraft() error = %v", err)
	}
	for _, statement := range []string{
		`UPDATE obligation_create_commands SET actor_display_name = 'changed' WHERE obligation_id = $1`,
		`DELETE FROM obligation_create_commands WHERE obligation_id = $1`,
	} {
		_, err := environment.Pool.Exec(ctx, statement, created.Obligation.ID)
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
