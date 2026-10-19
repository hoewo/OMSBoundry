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
	"omsbizfoundry/money-flow-ledger/internal/testdatabase"
)

func TestPostgresObligationConfirmationSnapshotIdempotencyAndConflicts(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "确认管理员",
	}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	service := NewService(NewPostgresRepository(environment.Pool))
	draft, err := service.CreateDraft(ctx, CreateDraftInput{
		CommandID: "44444444-4444-4444-8444-444444444444", Direction: "receivable",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Title: "待确认服务费", OriginalAmount: "100.500000", Currency: "CNY",
		DueOn: "2026-10-31", ManualSourceReference: "确认依据 OBL-001", Actor: actor,
	})
	if err != nil {
		t.Fatalf("CreateDraft() error = %v", err)
	}
	if _, err := environment.Pool.Exec(ctx, `
		UPDATE parties
		SET legal_name = '确认前已变化的主体', revision = 2, updated_at = clock_timestamp()
		WHERE id = $1
	`, owner.ID); err != nil {
		t.Fatalf("update current Party: %v", err)
	}

	confirmedAt := draft.Obligation.CreatedAt.Add(time.Minute)
	service.now = func() time.Time { return confirmedAt }
	confirmationInput := ConfirmInput{
		CommandID:            "55555555-5555-4555-8555-555555555555",
		ObligationID:         draft.Obligation.ID,
		ExpectedDraftVersion: 1,
		Actor:                actor,
	}
	confirmed, err := service.Confirm(ctx, confirmationInput)
	if err != nil || confirmed.Replayed {
		t.Fatalf("Confirm() = %#v, %v", confirmed, err)
	}
	if confirmed.Obligation.Status != StatusOpen || confirmed.Obligation.CurrentRevision != 1 ||
		confirmed.Revision.Revision != 1 || confirmed.Revision.SourceDraftVersion != 1 ||
		confirmed.Revision.Creditor.LegalName != "Owner Company" || confirmed.Revision.Creditor.PartyRevision != 1 ||
		confirmed.Revision.EffectiveAmount != "100.5" || confirmed.Revision.RecordedBy != actor ||
		!confirmed.Revision.RecordedAt.Equal(confirmedAt) {
		t.Fatalf("confirmed = %#v", confirmed)
	}

	if _, err := environment.Pool.Exec(ctx, `
		UPDATE obligations
		SET title = '不应成为正式投影', original_amount = 999
		WHERE id = $1
	`, draft.Obligation.ID); err != nil {
		t.Fatalf("mutate root draft columns: %v", err)
	}
	details, found, err := service.GetDetails(ctx, draft.Obligation.ID)
	if err != nil || !found || details.Obligation.Title != "待确认服务费" ||
		details.Obligation.OriginalAmount != "100.5" || len(details.Revisions) != 1 ||
		details.Revisions[0].Creditor.LegalName != "Owner Company" {
		t.Fatalf("GetDetails() = %#v, %v, %v", details, found, err)
	}

	replayed, err := service.Confirm(ctx, confirmationInput)
	if err != nil || !replayed.Replayed || replayed.Obligation.ID != draft.Obligation.ID || replayed.Revision.Revision != 1 {
		t.Fatalf("confirmation replay = %#v, %v", replayed, err)
	}
	changedVersion := confirmationInput
	changedVersion.ExpectedDraftVersion = 2
	_, err = service.Confirm(ctx, changedVersion)
	var idempotencyConflict *ConfirmationIdempotencyConflictError
	if !errors.As(err, &idempotencyConflict) {
		t.Fatalf("changed version replay error = %v", err)
	}
	changedActor := confirmationInput
	changedActor.Actor.ID = "66666666-6666-4666-8666-666666666666"
	_, err = service.Confirm(ctx, changedActor)
	if !errors.As(err, &idempotencyConflict) {
		t.Fatalf("changed actor replay error = %v", err)
	}

	_, err = service.Confirm(ctx, ConfirmInput{
		CommandID: "77777777-7777-4777-8777-777777777777", ObligationID: draft.Obligation.ID,
		ExpectedDraftVersion: 1, Actor: actor,
	})
	var stateConflict *ObligationStateConflictError
	if !errors.As(err, &stateConflict) || stateConflict.Status != StatusOpen {
		t.Fatalf("second confirmation error = %v", err)
	}
	_, err = service.Confirm(ctx, ConfirmInput{
		CommandID:            "88888888-8888-4888-8888-888888888888",
		ObligationID:         "99999999-9999-4999-8999-999999999999",
		ExpectedDraftVersion: 1, Actor: actor,
	})
	var notFound *ObligationNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("missing obligation error = %v", err)
	}

	versionDraft := createConfirmationDraft(t, ctx, service, actor, owner.ID, counterparty.ID,
		"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "版本冲突草稿")
	_, err = service.Confirm(ctx, ConfirmInput{
		CommandID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", ObligationID: versionDraft.ID,
		ExpectedDraftVersion: 2, Actor: actor,
	})
	var versionConflict *DraftVersionConflictError
	if !errors.As(err, &versionConflict) || versionConflict.Expected != 2 || versionConflict.Actual != 1 {
		t.Fatalf("version conflict error = %v", err)
	}

	var revisionCount, commandCount, auditCount int
	var auditActorKind, auditActorID, auditActorName string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM obligation_revisions WHERE obligation_id = $1),
			(SELECT count(*) FROM obligation_confirmation_commands WHERE obligation_id = $1),
			(SELECT count(*) FROM audit_entries WHERE object_type = 'obligation' AND object_id = $1 AND action = 'obligation_confirmed'),
			actor_kind, actor_id, actor_display_name
		FROM audit_entries
		WHERE object_type = 'obligation' AND object_id = $1 AND action = 'obligation_confirmed'
	`, draft.Obligation.ID).Scan(&revisionCount, &commandCount, &auditCount, &auditActorKind, &auditActorID, &auditActorName); err != nil {
		t.Fatalf("read confirmation facts: %v", err)
	}
	if revisionCount != 1 || commandCount != 1 || auditCount != 1 ||
		auditActorKind != actor.Kind || auditActorID != actor.ID || auditActorName != actor.DisplayName {
		t.Fatalf("confirmation facts revisions=%d commands=%d audits=%d actor=%s/%s/%s",
			revisionCount, commandCount, auditCount, auditActorKind, auditActorID, auditActorName)
	}
}

func TestPostgresObligationConfirmationConcurrency(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "并发确认管理员",
	}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	service := NewService(NewPostgresRepository(environment.Pool))

	sameCommandDraft := createConfirmationDraft(t, ctx, service, actor, owner.ID, counterparty.ID,
		"44444444-4444-4444-8444-444444444444", "同命令并发草稿")
	type result struct {
		value ConfirmResult
		err   error
	}
	runConcurrent := func(inputs []ConfirmInput) []result {
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
				value, err := service.Confirm(ctx, input)
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

	sameInput := ConfirmInput{
		CommandID: "55555555-5555-4555-8555-555555555555", ObligationID: sameCommandDraft.ID,
		ExpectedDraftVersion: 1, Actor: actor,
	}
	sameResults := runConcurrent([]ConfirmInput{sameInput, sameInput})
	replayCount := 0
	for _, result := range sameResults {
		if result.err != nil || result.value.Obligation.Status != StatusOpen {
			t.Fatalf("same command result = %#v, %v", result.value, result.err)
		}
		if result.value.Replayed {
			replayCount++
		}
	}
	if replayCount != 1 {
		t.Fatalf("same command replay count = %d", replayCount)
	}

	differentCommandDraft := createConfirmationDraft(t, ctx, service, actor, owner.ID, counterparty.ID,
		"66666666-6666-4666-8666-666666666666", "不同命令并发草稿")
	differentResults := runConcurrent([]ConfirmInput{
		{CommandID: "77777777-7777-4777-8777-777777777777", ObligationID: differentCommandDraft.ID, ExpectedDraftVersion: 1, Actor: actor},
		{CommandID: "88888888-8888-4888-8888-888888888888", ObligationID: differentCommandDraft.ID, ExpectedDraftVersion: 1, Actor: actor},
	})
	successes, stateConflicts := 0, 0
	for _, result := range differentResults {
		if result.err == nil {
			successes++
			continue
		}
		var conflict *ObligationStateConflictError
		if errors.As(result.err, &conflict) {
			stateConflicts++
			continue
		}
		t.Fatalf("unexpected different command error = %v", result.err)
	}
	if successes != 1 || stateConflicts != 1 {
		t.Fatalf("different command results successes=%d state_conflicts=%d", successes, stateConflicts)
	}

	for _, obligationID := range []string{sameCommandDraft.ID, differentCommandDraft.ID} {
		var revisions, commands, audits int
		if err := environment.Pool.QueryRow(ctx, `
			SELECT
				(SELECT count(*) FROM obligation_revisions WHERE obligation_id = $1),
				(SELECT count(*) FROM obligation_confirmation_commands WHERE obligation_id = $1),
				(SELECT count(*) FROM audit_entries WHERE object_type = 'obligation' AND object_id = $1 AND action = 'obligation_confirmed')
		`, obligationID).Scan(&revisions, &commands, &audits); err != nil {
			t.Fatalf("read concurrent facts: %v", err)
		}
		if revisions != 1 || commands != 1 || audits != 1 {
			t.Fatalf("concurrent facts obligation=%s revisions=%d commands=%d audits=%d", obligationID, revisions, commands, audits)
		}
	}
}

func TestPostgresObligationConfirmationRollsBackAndProtectsFacts(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "回滚确认管理员",
	}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	service := NewService(NewPostgresRepository(environment.Pool))
	draft := createConfirmationDraft(t, ctx, service, actor, owner.ID, counterparty.ID,
		"44444444-4444-4444-8444-444444444444", "回滚确认草稿")
	now := time.Now().UTC().Truncate(time.Microsecond)
	repository := NewPostgresRepository(environment.Pool)
	_, _, _, err := repository.Confirm(ctx, draft.ID, 1, ConfirmationCommand{
		CommandID: "55555555-5555-4555-8555-555555555555", PayloadHash: make([]byte, 32), Actor: actor, ConfirmedAt: now,
	}, audit.Entry{
		ID: "66666666-6666-4666-8666-666666666666", ObjectType: audit.ObjectObligation, ObjectID: draft.ID,
		Action: AuditActionConfirmed, CommandID: "55555555-5555-4555-8555-555555555555", Actor: actor,
		OccurredAt: now, Summary: "", Details: map[string]any{},
	})
	if err == nil {
		t.Fatal("Confirm() error = nil")
	}
	var status string
	var currentRevision *int64
	var revisions, commands, audits int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			status,
			current_revision,
			(SELECT count(*) FROM obligation_revisions WHERE obligation_id = $1),
			(SELECT count(*) FROM obligation_confirmation_commands WHERE obligation_id = $1),
			(SELECT count(*) FROM audit_entries WHERE object_type = 'obligation' AND object_id = $1 AND action = 'obligation_confirmed')
		FROM obligations WHERE id = $1
	`, draft.ID).Scan(&status, &currentRevision, &revisions, &commands, &audits); err != nil {
		t.Fatalf("read rollback facts: %v", err)
	}
	if status != StatusDraft || currentRevision != nil || revisions != 0 || commands != 0 || audits != 0 {
		t.Fatalf("rollback status=%s current=%v revisions=%d commands=%d audits=%d", status, currentRevision, revisions, commands, audits)
	}

	confirmed, err := service.Confirm(ctx, ConfirmInput{
		CommandID: "77777777-7777-4777-8777-777777777777", ObligationID: draft.ID,
		ExpectedDraftVersion: 1, Actor: actor,
	})
	if err != nil {
		t.Fatalf("service.Confirm() error = %v", err)
	}
	for _, statement := range []string{
		`UPDATE obligation_revisions SET title = 'changed' WHERE obligation_id = $1`,
		`DELETE FROM obligation_revisions WHERE obligation_id = $1`,
		`UPDATE obligation_confirmation_commands SET actor_display_name = 'changed' WHERE obligation_id = $1`,
		`DELETE FROM obligation_confirmation_commands WHERE obligation_id = $1`,
	} {
		_, err := environment.Pool.Exec(ctx, statement, confirmed.Obligation.ID)
		var databaseError *pgconn.PgError
		if !errors.As(err, &databaseError) || databaseError.Code != "55000" {
			t.Fatalf("immutable statement error = %v", err)
		}
	}
}

func createConfirmationDraft(
	t *testing.T,
	ctx context.Context,
	service *Service,
	actor audit.ActorRef,
	ownerID string,
	counterpartyID string,
	commandID string,
	title string,
) Obligation {
	t.Helper()
	created, err := service.CreateDraft(ctx, CreateDraftInput{
		CommandID: commandID, Direction: "payable",
		LedgerOwnerPartyID: ownerID, CounterpartyPartyID: counterpartyID,
		Title: title, OriginalAmount: "25", Currency: "CNY",
		DueOn: "2026-12-31", ManualSourceReference: "确认测试人工依据", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create confirmation draft: %v", err)
	}
	return created.Obligation
}
