//go:build integration

package finance

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"omsbizfoundry/internal-finance-requests/internal/database"
	"os"
	"testing"
)

func TestLifecycleIdempotencyRevisionAuditAndTenantIsolation(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	service, _ := NewService(store)
	actor := Actor{ID: "22222222-2222-4222-8222-222222222222", TenantID: "11111111-1111-4111-8111-111111111111", CredentialID: "33333333-3333-4333-8333-333333333333"}
	input := CreateInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", RequestType: "reimbursement", Title: "差旅报销", AmountMinor: 12345, Currency: "CNY"}
	created, err := service.Create(ctx, actor, input)
	if err != nil || created.Revision != 1 {
		t.Fatalf("create: %#v %v", created, err)
	}
	replayed, err := service.Create(ctx, actor, input)
	if err != nil || replayed.ID != created.ID {
		t.Fatalf("replay: %#v %v", replayed, err)
	}
	changed := input
	changed.Title = "冲突"
	if _, err = service.Create(ctx, actor, changed); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("expected command conflict, got %v", err)
	}
	updated, err := service.Update(ctx, actor, created.ID, UpdateInput{CommandID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", ExpectedRevision: 1, Title: "差旅报销更正", AmountMinor: 12000})
	if err != nil || updated.Revision != 2 {
		t.Fatalf("update: %#v %v", updated, err)
	}
	if _, err = service.Update(ctx, actor, created.ID, UpdateInput{CommandID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", ExpectedRevision: 1, Title: "过期", AmountMinor: 1}); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("expected revision conflict, got %v", err)
	}
	if _, err = service.Get(ctx, "99999999-9999-4999-8999-999999999999", created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross tenant read must be 404, got %v", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE finance_requests SET title='tampered' WHERE tenant_id=$1 AND id=$2`, actor.TenantID, created.ID); err == nil {
		t.Fatal("current projection mutation without revision must fail")
	}
	if _, err = pool.Exec(ctx, `UPDATE finance_request_revisions SET title='tampered' WHERE tenant_id=$1 AND request_id=$2 AND revision=1`, actor.TenantID, created.ID); err == nil {
		t.Fatal("revision mutation must fail")
	}
	var requests, revisions, commands, audits int
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM finance_requests),(SELECT count(*) FROM finance_request_revisions),(SELECT count(*) FROM finance_request_commands),(SELECT count(*) FROM finance_request_audit_events)`).Scan(&requests, &revisions, &commands, &audits); err != nil {
		t.Fatal(err)
	}
	if requests != 1 || revisions != 2 || commands != 2 || audits != 2 {
		t.Fatalf("unexpected counts %d/%d/%d/%d", requests, revisions, commands, audits)
	}
}
func integrationPool(t *testing.T, ctx context.Context) (*pgxpool.Pool, func()) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	var random [8]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	schema := "finance_test_" + hex.EncodeToString(random[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool, func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
		admin.Close()
	}
}
