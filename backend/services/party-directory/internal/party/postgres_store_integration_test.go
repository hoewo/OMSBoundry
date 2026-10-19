//go:build integration

package party

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"omsbizfoundry/party-directory/internal/database"
)

func TestPostgresLifecycleIdempotencyHistoryAndTenantIsolation(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	service, err := NewService(store, strings.Repeat("k", 32))
	if err != nil {
		t.Fatal(err)
	}
	actor := Actor{ID: "22222222-2222-4222-8222-222222222222", TenantID: "11111111-1111-4111-8111-111111111111", DisplayName: "管理员", Kind: "iam_subject", CredentialID: "33333333-3333-4333-8333-333333333333"}
	identifier := "REG-001"
	create := CreateInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Kind: KindOrganization, LegalName: "甲方公司", RegistrationIdentifier: &identifier}
	created, err := service.Create(ctx, actor, create)
	if err != nil {
		t.Fatal(err)
	}
	if created.Revision != 1 || created.Status != StatusActive {
		t.Fatalf("unexpected created party: %#v", created)
	}
	replayed, err := service.Create(ctx, actor, create)
	if err != nil || replayed.ID != created.ID || replayed.Revision != 1 {
		t.Fatalf("bad replay: %#v %v", replayed, err)
	}
	changed := create
	changed.LegalName = "另一家公司"
	if _, err = service.Create(ctx, actor, changed); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("payload conflict expected, got %v", err)
	}
	newIdentifier := "REG-002"
	updated, err := service.Update(ctx, actor, created.ID, UpdateInput{CommandID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", ExpectedRevision: 1, Kind: KindOrganization, LegalName: "甲方公司（更正）", RegistrationIdentifier: &newIdentifier, Reason: "名称更正"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 2 {
		t.Fatalf("expected revision 2: %#v", updated)
	}
	if _, err = service.Update(ctx, actor, created.ID, UpdateInput{CommandID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", ExpectedRevision: 1, Kind: KindOrganization, LegalName: "过期写入", Reason: "stale"}); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale revision expected, got %v", err)
	}
	if _, err = service.Update(ctx, actor, created.ID, UpdateInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", ExpectedRevision: 2, Kind: KindOrganization, LegalName: updated.LegalName, RegistrationIdentifier: &newIdentifier, Reason: "无变化"}); !errors.Is(err, ErrNoEffectiveChange) {
		t.Fatalf("no effective change expected, got %v", err)
	}
	inactive, err := service.ChangeStatus(ctx, actor, created.ID, StatusInput{CommandID: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", ExpectedRevision: 2, Status: StatusInactive, Reason: "暂停合作"})
	if err != nil {
		t.Fatal(err)
	}
	if inactive.Revision != 3 {
		t.Fatalf("expected revision 3: %#v", inactive)
	}
	if _, err = service.Reference(ctx, actor.TenantID, created.ID); !errors.Is(err, ErrInactive) {
		t.Fatalf("inactive reference must fail, got %v", err)
	}
	history, err := service.History(ctx, actor.TenantID, created.ID, nil, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Items) != 3 || history.Items[0].Revision != 3 || history.Items[2].Revision != 1 {
		t.Fatalf("unexpected history: %#v", history.Items)
	}
	page, err := service.List(ctx, actor.TenantID, ListFilter{Status: "all"}, "")
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("list all failed: %#v %v", page, err)
	}
	activePage, err := service.List(ctx, actor.TenantID, ListFilter{}, "")
	if err != nil || len(activePage.Items) != 0 {
		t.Fatalf("inactive party leaked into default list: %#v %v", activePage, err)
	}
	if _, err = service.Get(ctx, "99999999-9999-4999-8999-999999999999", created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross tenant read must look not found: %v", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE party_revisions SET legal_name='tampered' WHERE tenant_id=$1 AND party_id=$2 AND revision=1`, actor.TenantID, created.ID); err == nil {
		t.Fatal("revision update must be rejected by trigger")
	}
	if _, err = pool.Exec(ctx, `UPDATE parties SET legal_name='tampered',legal_name_normalized='tampered' WHERE tenant_id=$1 AND id=$2`, actor.TenantID, created.ID); err == nil {
		t.Fatal("current projection without a matching revision must be rejected")
	}
	var revisions, commands, audits int
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM party_revisions),(SELECT count(*) FROM party_commands),(SELECT count(*) FROM party_audit_events)`).Scan(&revisions, &commands, &audits); err != nil {
		t.Fatal(err)
	}
	if revisions != 3 || commands != 3 || audits != 3 {
		t.Fatalf("failed writes must leave no records: revisions=%d commands=%d audits=%d", revisions, commands, audits)
	}
}

func TestConcurrentCreateReplayProducesOneParty(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	service, _ := NewService(store, strings.Repeat("k", 32))
	actor := Actor{ID: "22222222-2222-4222-8222-222222222222", TenantID: "11111111-1111-4111-8111-111111111111", Kind: "iam_subject", CredentialID: "33333333-3333-4333-8333-333333333333"}
	input := CreateInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Kind: KindIndividual, LegalName: "张三"}
	const workers = 8
	ids := make(chan string, workers)
	errs := make(chan error, workers)
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			value, err := service.Create(ctx, actor, input)
			if err != nil {
				errs <- err
				return
			}
			ids <- value.ID
		}()
	}
	group.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	first := ""
	count := 0
	for id := range ids {
		count++
		if first == "" {
			first = id
		} else if id != first {
			t.Fatalf("concurrent replay returned different ids: %s %s", first, id)
		}
	}
	if count != workers {
		t.Fatalf("expected %d results, got %d", workers, count)
	}
	var parties, revisions int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM parties),(SELECT count(*) FROM party_revisions)`).Scan(&parties, &revisions); err != nil {
		t.Fatal(err)
	}
	if parties != 1 || revisions != 1 {
		t.Fatalf("expected one party/revision, got %d/%d", parties, revisions)
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
	schema := "party_test_" + hex.EncodeToString(random[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	if err = database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
		admin.Close()
	}
	return pool, cleanup
}
