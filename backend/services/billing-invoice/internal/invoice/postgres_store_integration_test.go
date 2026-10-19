//go:build integration

package invoice

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"omsbizfoundry/billing-invoice/internal/database"
)

func TestPostgresLifecycleIdempotencyHistoryIsolationAndImmutability(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	resolver := &fakeResolver{}
	service, _ := NewService(store, resolver, "CNY")
	actor := testActor()
	create := testCreateInput()

	created, err := service.Create(ctx, actor, create)
	if err != nil {
		t.Fatal(err)
	}
	if created.Revision != 1 || created.Status != StatusRecorded || created.AmountMinor != 12345 || created.Currency != "CNY" {
		t.Fatalf("unexpected registration: %#v", created)
	}
	replayed, err := service.Create(ctx, actor, create)
	if err != nil || replayed.ID != created.ID || replayed.Revision != 1 {
		t.Fatalf("bad replay: %#v %v", replayed, err)
	}
	changed := create
	changed.AmountMinor = 99999
	if _, err = service.Create(ctx, actor, changed); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("payload conflict expected, got %v", err)
	}

	newAmount := int64(20000)
	corrected, err := service.Correct(ctx, actor, created.ID, CorrectInput{CommandID: "66666666-6666-4666-8666-666666666666", ExpectedRevision: 1, Reason: "金额录入错误", Patch: CorrectionPatch{AmountMinor: &newAmount}})
	if err != nil {
		t.Fatal(err)
	}
	if corrected.Revision != 2 || corrected.AmountMinor != newAmount {
		t.Fatalf("unexpected correction: %#v", corrected)
	}
	staleNote := "过期"
	if _, err = service.Correct(ctx, actor, created.ID, CorrectInput{CommandID: "77777777-7777-4777-8777-777777777777", ExpectedRevision: 1, Reason: "stale", Patch: CorrectionPatch{Note: &staleNote}}); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale correction expected, got %v", err)
	}

	evidence := EvidenceRef{FileID: "88888888-8888-4888-8888-888888888888", Version: 1, Purpose: "invoice_document", DisplayName: "invoice.pdf"}
	withEvidence, err := service.ReplaceEvidence(ctx, actor, created.ID, EvidenceInput{CommandID: "99999999-9999-4999-8999-999999999999", ExpectedRevision: 2, Reason: "补充发票文件", EvidenceRefs: []EvidenceRef{evidence}})
	if err != nil {
		t.Fatal(err)
	}
	if withEvidence.Revision != 3 || len(withEvidence.EvidenceRefs) != 1 {
		t.Fatalf("unexpected evidence revision: %#v", withEvidence)
	}

	invalidated, err := service.Invalidate(ctx, actor, created.ID, InvalidateInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaab", ExpectedRevision: 3, Reason: "重复登记"})
	if err != nil {
		t.Fatal(err)
	}
	if invalidated.Revision != 4 || invalidated.Status != StatusRegistrationInvalidated {
		t.Fatalf("unexpected invalidation: %#v", invalidated)
	}
	if _, err = service.ReplaceEvidence(ctx, actor, created.ID, EvidenceInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaac", ExpectedRevision: 4, Reason: "无效后修改", EvidenceRefs: nil}); !errors.Is(err, ErrRegistrationInvalidated) {
		t.Fatalf("invalidated registration must reject writes, got %v", err)
	}

	history, err := service.History(ctx, actor.TenantID, created.ID, nil, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Items) != 4 || history.Items[0].EventKind != EventRegistrationInvalidated || history.Items[3].EventKind != EventRecorded {
		t.Fatalf("unexpected history: %#v", history.Items)
	}
	if history.Items[0].Snapshot.CreatedAt.IsZero() || history.Items[0].Snapshot.UpdatedAt.IsZero() || !history.Items[0].Snapshot.CreatedAt.Equal(created.CreatedAt) {
		t.Fatalf("history must preserve full timestamps: %#v", history.Items[0].Snapshot)
	}
	if _, err = service.Get(ctx, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant read must look not found: %v", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE invoice_registration_revisions SET amount_minor=1 WHERE tenant_id=$1 AND invoice_registration_id=$2 AND revision=1`, actor.TenantID, created.ID); err == nil {
		t.Fatal("revision update must be rejected")
	}
	if _, err = pool.Exec(ctx, `UPDATE invoice_registrations SET amount_minor=1 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, created.ID); err == nil {
		t.Fatal("current projection without revision must be rejected")
	}
	var registrations, revisions, commands, audits int
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM invoice_registrations),(SELECT count(*) FROM invoice_registration_revisions),(SELECT count(*) FROM invoice_registration_commands),(SELECT count(*) FROM invoice_registration_audit_events)`).Scan(&registrations, &revisions, &commands, &audits); err != nil {
		t.Fatal(err)
	}
	if registrations != 1 || revisions != 4 || commands != 4 || audits != 4 {
		t.Fatalf("failed writes changed facts: registrations=%d revisions=%d commands=%d audits=%d", registrations, revisions, commands, audits)
	}
}

func TestPostgresReferenceFailureLeavesNoFacts(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	service, _ := NewService(store, &fakeResolver{fail: errors.New("party unavailable")}, "CNY")
	if _, err := service.Create(ctx, testActor(), testCreateInput()); err == nil {
		t.Fatal("dependency failure must fail")
	}
	var registrations, revisions, commands, audits int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM invoice_registrations),(SELECT count(*) FROM invoice_registration_revisions),(SELECT count(*) FROM invoice_registration_commands),(SELECT count(*) FROM invoice_registration_audit_events)`).Scan(&registrations, &revisions, &commands, &audits); err != nil {
		t.Fatal(err)
	}
	if registrations != 0 || revisions != 0 || commands != 0 || audits != 0 {
		t.Fatalf("dependency failure created partial facts: %d/%d/%d/%d", registrations, revisions, commands, audits)
	}
}

func TestPostgresRemoveLastEvidencePreservesHistoryAndReplay(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	service, _ := NewService(store, &fakeResolver{}, "CNY")
	actor := testActor()
	created, err := service.Create(ctx, actor, testCreateInput())
	if err != nil {
		t.Fatal(err)
	}
	ref := EvidenceRef{FileID: "88888888-8888-4888-8888-888888888888", Version: 1, Purpose: "invoice_document", DisplayName: "invoice.pdf"}
	attached, err := service.ReplaceEvidence(ctx, actor, created.ID, EvidenceInput{CommandID: "99999999-9999-4999-8999-999999999999", ExpectedRevision: 1, Reason: "登记文件版本", EvidenceRefs: []EvidenceRef{ref}})
	if err != nil || attached.Revision != 2 {
		t.Fatalf("attach: %#v, %v", attached, err)
	}
	input := EvidenceInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaab", ExpectedRevision: 2, Reason: "移出最后引用，保留文件", EvidenceRefs: []EvidenceRef{}}
	removed, err := service.ReplaceEvidence(ctx, actor, created.ID, input)
	if err != nil || removed.Revision != 3 || removed.EvidenceRefs == nil || len(removed.EvidenceRefs) != 0 {
		t.Fatalf("remove last: %#v, %v", removed, err)
	}
	// Nil and [] denote the same empty, bounded reference set after normalization.
	input.EvidenceRefs = nil
	replayed, err := service.ReplaceEvidence(ctx, actor, created.ID, input)
	if err != nil || replayed.Revision != 3 || replayed.EvidenceRefs == nil || len(replayed.EvidenceRefs) != 0 {
		t.Fatalf("replay: %#v, %v", replayed, err)
	}
	current, err := service.Get(ctx, actor.TenantID, created.ID)
	if err != nil || current.Revision != 3 || current.EvidenceRefs == nil || len(current.EvidenceRefs) != 0 {
		t.Fatalf("reread: %#v, %v", current, err)
	}
	history, err := service.History(ctx, actor.TenantID, created.ID, nil, 50)
	if err != nil || len(history.Items) != 3 || history.Items[0].Snapshot.EvidenceRefs == nil || len(history.Items[0].Snapshot.EvidenceRefs) != 0 || len(history.Items[1].Snapshot.EvidenceRefs) != 1 || history.Items[1].Snapshot.EvidenceRefs[0] != ref {
		t.Fatalf("frozen evidence history: %#v, %v", history, err)
	}
	var commands, audits int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM invoice_registration_commands),(SELECT count(*) FROM invoice_registration_audit_events)`).Scan(&commands, &audits); err != nil || commands != 3 || audits != 3 {
		t.Fatalf("replay duplicated facts: %d/%d, %v", commands, audits, err)
	}
}

func TestConcurrentCreateReplayProducesOneRegistration(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	service, _ := NewService(store, &fakeResolver{}, "CNY")
	const workers = 8
	ids := make(chan string, workers)
	errs := make(chan error, workers)
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			value, err := service.Create(ctx, testActor(), testCreateInput())
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
	var registrations, revisions, commands, audits int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM invoice_registrations),(SELECT count(*) FROM invoice_registration_revisions),(SELECT count(*) FROM invoice_registration_commands),(SELECT count(*) FROM invoice_registration_audit_events)`).Scan(&registrations, &revisions, &commands, &audits); err != nil {
		t.Fatal(err)
	}
	if registrations != 1 || revisions != 1 || commands != 1 || audits != 1 {
		t.Fatalf("expected one committed command, got %d/%d/%d/%d", registrations, revisions, commands, audits)
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
	schema := "billing_test_" + hex.EncodeToString(random[:])
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
