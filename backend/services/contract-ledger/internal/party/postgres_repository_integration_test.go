//go:build integration

package party_test

import (
	"context"
	"os"
	"testing"
	"time"

	"omsbizfoundry/contract-ledger/internal/audit"
	"omsbizfoundry/contract-ledger/internal/database"
	"omsbizfoundry/contract-ledger/internal/party"
)

func TestPostgresRepositoryCreateAndList(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("second database.Migrate() error = %v", err)
	}

	service := party.NewService(party.NewPostgresRepository(pool))
	created, err := service.Create(ctx, party.CreateInput{
		Actor: audit.ActorRef{
			Kind:        audit.ActorKindLocalUser,
			ID:          "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
			DisplayName: "主体集成测试操作者",
		},
		Kind:                   "organization",
		LegalName:              "集成测试主体",
		RegistrationIdentifier: "INTEGRATION-TEST",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		connection, acquireErr := pool.Acquire(cleanupCtx)
		if acquireErr != nil {
			t.Errorf("acquire cleanup connection: %v", acquireErr)
			return
		}
		defer connection.Release()
		if _, disableErr := connection.Exec(cleanupCtx, `SET session_replication_role = replica`); disableErr != nil {
			t.Errorf("disable audit trigger: %v", disableErr)
			return
		}
		defer func() { _, _ = connection.Exec(context.Background(), `SET session_replication_role = origin`) }()
		if _, deleteAuditErr := connection.Exec(cleanupCtx, `DELETE FROM audit_entries WHERE object_id = $1`, created.ID); deleteAuditErr != nil {
			t.Errorf("clean up party audit %q: %v", created.ID, deleteAuditErr)
		}
		if _, err := pool.Exec(cleanupCtx, "DELETE FROM parties WHERE id = $1", created.ID); err != nil {
			t.Errorf("clean up party %q: %v", created.ID, err)
		}
	}()

	parties, err := service.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	found := false
	for _, value := range parties {
		if value.ID == created.ID {
			found = value.LegalName == "集成测试主体" && value.RegistrationIdentifier == "INTEGRATION-TEST"
			break
		}
	}
	if !found {
		t.Fatalf("created party not found in %#v", parties)
	}
	var auditID, actorID string
	if err := pool.QueryRow(ctx, `
		SELECT id::text, actor_id
		FROM audit_entries
		WHERE object_type = 'party' AND object_id = $1 AND action = 'party_created'
	`, created.ID).Scan(&auditID, &actorID); err != nil {
		t.Fatalf("read party audit: %v", err)
	}
	if actorID != "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" {
		t.Fatalf("party audit actor = %q", actorID)
	}

	now := time.Now().UTC()
	rollbackID := "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	err = party.NewPostgresRepository(pool).Create(ctx, party.Party{
		ID: rollbackID, Kind: party.KindOrganization, LegalName: "审计失败回滚主体",
		Revision: 1, CreatedAt: now, UpdatedAt: now,
	}, audit.Entry{
		ID: auditID, ObjectType: audit.ObjectParty, ObjectID: rollbackID,
		Action: party.AuditActionPartyCreated,
		Actor: audit.ActorRef{
			Kind: audit.ActorKindLocalUser, ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", DisplayName: "主体集成测试操作者",
		},
		OccurredAt: now, Summary: "应当回滚", Details: map[string]any{"party_revision": 1},
	})
	if err == nil {
		t.Fatal("Create() with duplicate audit ID error = nil")
	}
	var rollbackCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM parties WHERE id = $1`, rollbackID).Scan(&rollbackCount); err != nil {
		t.Fatalf("count rollback party: %v", err)
	}
	if rollbackCount != 0 {
		t.Fatalf("audit failure left party")
	}
}
