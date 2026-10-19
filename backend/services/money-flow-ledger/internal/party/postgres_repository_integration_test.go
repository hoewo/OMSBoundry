//go:build integration

package party

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/database"
	"omsbizfoundry/money-flow-ledger/internal/testdatabase"
)

func TestPostgresPartyCreationListingIdempotencyAndConcurrency(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	service := NewService(NewPostgresRepository(environment.Pool))
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "主体联调管理员",
	}

	organization, err := service.Create(ctx, CreateInput{
		CommandID: "22222222-2222-4222-8222-222222222222", Kind: " organization ",
		LegalName: " beta company ", RegistrationIdentifier: " BETA-001 ", Actor: actor,
	})
	if err != nil || organization.Replayed {
		t.Fatalf("create organization = %#v, %v", organization, err)
	}
	individual, err := service.Create(ctx, CreateInput{
		CommandID: "33333333-3333-4333-8333-333333333333", Kind: "individual",
		LegalName: "Alpha Person", RegistrationIdentifier: "   ", Actor: actor,
	})
	if err != nil || individual.Replayed || individual.Party.RegistrationIdentifier != "" {
		t.Fatalf("create individual = %#v, %v", individual, err)
	}
	listed, err := service.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listed) != 2 || listed[0].ID != individual.Party.ID || listed[1].ID != organization.Party.ID ||
		listed[0].Revision != 1 || listed[1].RegistrationIdentifier != "BETA-001" {
		t.Fatalf("listed parties = %#v", listed)
	}

	replayed, err := service.Create(ctx, CreateInput{
		CommandID: "22222222-2222-4222-8222-222222222222", Kind: "ORGANIZATION",
		LegalName: "beta company", RegistrationIdentifier: "BETA-001", Actor: actor,
	})
	if err != nil || !replayed.Replayed || replayed.Party.ID != organization.Party.ID {
		t.Fatalf("replay = %#v, %v", replayed, err)
	}
	_, err = service.Create(ctx, CreateInput{
		CommandID: "22222222-2222-4222-8222-222222222222", Kind: "organization",
		LegalName: "changed company", RegistrationIdentifier: "BETA-001", Actor: actor,
	})
	var conflict *IdempotencyConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("changed replay error = %v", err)
	}
	changedActor := actor
	changedActor.ID = "99999999-9999-4999-8999-999999999999"
	_, err = service.Create(ctx, CreateInput{
		CommandID: "22222222-2222-4222-8222-222222222222", Kind: "organization",
		LegalName: "beta company", RegistrationIdentifier: "BETA-001", Actor: changedActor,
	})
	if !errors.As(err, &conflict) {
		t.Fatalf("changed actor replay error = %v", err)
	}

	const concurrentCommandID = "44444444-4444-4444-8444-444444444444"
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
			result, createErr := service.Create(ctx, CreateInput{
				CommandID: concurrentCommandID, Kind: "individual", LegalName: "Concurrent Person", Actor: actor,
			})
			results <- concurrentResult{result: result, err: createErr}
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	var concurrentPartyID string
	replayedCount := 0
	for value := range results {
		if value.err != nil {
			t.Fatalf("concurrent Create() error = %v", value.err)
		}
		if concurrentPartyID == "" {
			concurrentPartyID = value.result.Party.ID
		}
		if value.result.Party.ID != concurrentPartyID {
			t.Fatalf("concurrent party IDs differ: %s and %s", concurrentPartyID, value.result.Party.ID)
		}
		if value.result.Replayed {
			replayedCount++
		}
	}
	if replayedCount != 1 {
		t.Fatalf("concurrent replay count = %d", replayedCount)
	}

	var parties, commands, audits int
	var auditActorKind, auditActorID, auditActorName string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM parties),
			(SELECT count(*) FROM party_create_commands),
			(SELECT count(*) FROM audit_entries WHERE action = 'party_created'),
			actor_kind, actor_id, actor_display_name
		FROM audit_entries
		WHERE action = 'party_created' AND command_id = '22222222-2222-4222-8222-222222222222'
	`).Scan(&parties, &commands, &audits, &auditActorKind, &auditActorID, &auditActorName); err != nil {
		t.Fatalf("read party facts: %v", err)
	}
	if parties != 3 || commands != 3 || audits != 3 || auditActorKind != actor.Kind || auditActorID != actor.ID || auditActorName != actor.DisplayName {
		t.Fatalf("facts parties=%d commands=%d audits=%d actor=%s/%s/%s", parties, commands, audits, auditActorKind, auditActorID, auditActorName)
	}
}

func TestPostgresPartyCreationRollsBackWithAuditAndProtectsCommands(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	repository := NewPostgresRepository(environment.Pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "回滚管理员",
	}
	partyID := "55555555-5555-4555-8555-555555555555"
	commandID := "66666666-6666-4666-8666-666666666666"
	_, _, err := repository.Create(ctx, Party{
		ID: partyID, Kind: KindOrganization, LegalName: "应整体回滚的主体", Revision: 1, CreatedAt: now, UpdatedAt: now,
	}, CreateCommand{
		CommandID: commandID, PayloadHash: make([]byte, 32), Actor: actor, CreatedAt: now,
	}, audit.Entry{
		ID: "77777777-7777-4777-8777-777777777777", ObjectType: audit.ObjectParty, ObjectID: partyID,
		Action: AuditActionPartyCreated, CommandID: commandID, Actor: actor, OccurredAt: now,
		Summary: "", Details: map[string]any{},
	})
	if err == nil {
		t.Fatal("Create() error = nil")
	}
	var parties, commands, audits int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM parties WHERE id = $1),
			(SELECT count(*) FROM party_create_commands WHERE command_id = $2),
			(SELECT count(*) FROM audit_entries WHERE command_id = $2)
	`, partyID, commandID).Scan(&parties, &commands, &audits); err != nil {
		t.Fatalf("read rollback facts: %v", err)
	}
	if parties != 0 || commands != 0 || audits != 0 {
		t.Fatalf("rollback facts parties=%d commands=%d audits=%d", parties, commands, audits)
	}

	service := NewService(repository)
	created, err := service.Create(ctx, CreateInput{
		CommandID: "88888888-8888-4888-8888-888888888888", Kind: "organization", LegalName: "不可变命令主体", Actor: actor,
	})
	if err != nil {
		t.Fatalf("service.Create() error = %v", err)
	}
	for _, statement := range []string{
		`UPDATE party_create_commands SET actor_display_name = 'changed' WHERE party_id = $1`,
		`DELETE FROM party_create_commands WHERE party_id = $1`,
	} {
		_, err := environment.Pool.Exec(ctx, statement, created.Party.ID)
		var databaseError *pgconn.PgError
		if !errors.As(err, &databaseError) || databaseError.Code != "55000" {
			t.Fatalf("immutable statement error = %v", err)
		}
	}
}
