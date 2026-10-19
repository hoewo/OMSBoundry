//go:build integration

package referencecost

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	core "omsbizfoundry/cost-outsourcing/internal/costoutsourcing"
	"omsbizfoundry/cost-outsourcing/internal/database"
	"os"
	"sync"
	"testing"
)

func referencePool(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	var random [8]byte
	if _, e = rand.Read(random[:]); e != nil {
		t.Fatal(e)
	}
	schema := "reference_cost_test_" + hex.EncodeToString(random[:])
	if _, e = admin.Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	cfg, e := pgxpool.ParseConfig(url)
	if e != nil {
		t.Fatal(e)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	if e = database.Migrate(ctx, pool); e != nil {
		t.Fatal(e)
	}
	return pool, func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
	}
}
func TestReferenceCostPostgresReplayFrozenHistoryAndSideIsolation(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := referencePool(t)
	defer cleanup()
	store, e := NewPostgresStore(pool)
	if e != nil {
		t.Fatal(e)
	}
	r := &resolver{}
	s, _ := New(store, r)
	a := actor()
	in := input("business")
	in.Fields.AmountMinor = "9223372036854775807"
	in.Fields.EvidenceRefs = []core.EvidenceRef{{FileID: file, Version: 2, Slot: "invoice"}}
	v, e := s.Create(ctx, a, in)
	if e != nil {
		t.Fatal(e)
	}
	if v.AmountMinor != 9223372036854775807 || v.OccurredOn != nil || v.Description != nil {
		t.Fatal("exact/nullable values changed")
	}
	r.unavailable = true
	replay, e := s.Create(ctx, a, in)
	if e != nil || replay.ID != v.ID {
		t.Fatalf("replay depended on external source: %v", e)
	}
	r.unavailable = false
	bad := in
	bad.Fields.AmountMinor = "1"
	if _, e = s.Create(ctx, a, bad); !errors.Is(e, core.ErrCommandConflict) {
		t.Fatalf("payload conflict: %v", e)
	}
	fields := in.Fields
	fields.AmountMinor = "101"
	change := ChangeInput{CommandID: newID(), ExpectedRevision: 1, Reason: "append, not overwrite", Fields: fields}
	updated, e := s.Correct(ctx, a, v.ID, change)
	if e != nil || updated.Revision != 2 {
		t.Fatalf("correct: %v", e)
	}
	stale := change
	stale.CommandID = newID()
	if _, e = s.Correct(ctx, a, v.ID, stale); !errors.Is(e, core.ErrRevisionConflict) {
		t.Fatalf("stale write: %v", e)
	}
	withdrawn, e := s.Withdraw(ctx, a, v.ID, WithdrawInput{CommandID: newID(), ExpectedRevision: 2, Reason: "preserve all bytes and facts"})
	if e != nil || withdrawn.Revision != 3 || withdrawn.State != "withdrawn" || withdrawn.AmountMinor != 101 || len(withdrawn.Evidence) != 1 {
		t.Fatalf("withdrawal: %v", e)
	}
	r.unavailable = true
	if replay, e = s.Correct(ctx, a, v.ID, change); e != nil || replay.Revision != 2 {
		t.Fatalf("committed correction replay after withdrawal: %v", e)
	}
	h, e := s.History(ctx, a, v.ID)
	if e != nil || len(h) != 3 || h[2].Record.AmountMinor != 9223372036854775807 || h[0].Record.AmountMinor != 101 || h[0].Record.Project.Revision != 7 {
		t.Fatalf("history: %#v %v", h, e)
	}
	generic := a
	generic.TenantScope = false
	generic.ProjectIDs = []string{project}
	generic.ResourceScopes = []core.ResourceScope{{Type: "project", ID: project}}
	if _, e = s.Get(ctx, generic, v.ID); !errors.Is(e, core.ErrForbidden) {
		t.Fatalf("generic project grants crossed overlay: %v", e)
	}
	exact := a
	exact.TenantScope = false
	exact.ResourceScopes = []core.ResourceScope{{Type: SideKind("production"), ID: project}}
	if _, e = s.Get(ctx, exact, v.ID); !errors.Is(e, core.ErrForbidden) {
		t.Fatal("production grant saw business cost")
	}
	exact.ResourceScopes = []core.ResourceScope{{Type: ResourceType, ID: v.ID}}
	if _, e = s.Get(ctx, exact, v.ID); e != nil {
		t.Fatal(e)
	}
	for _, query := range []string{
		`UPDATE reference_oa_cost_entries SET scope='production' WHERE id=$1`,
		`UPDATE reference_oa_cost_entries SET revision=revision+1,snapshot=jsonb_set(snapshot,'{amount_minor}','102') WHERE id=$1`,
		`DELETE FROM reference_oa_cost_entries WHERE id=$1`,
		`UPDATE reference_oa_cost_revisions SET reason='tampered' WHERE cost_entry_id=$1`,
		`DELETE FROM reference_oa_cost_commands WHERE response->>'id'=$1`,
		`DELETE FROM reference_oa_cost_audit WHERE cost_entry_id=$1`,
	} {
		if _, e = pool.Exec(ctx, query, v.ID); e == nil {
			t.Fatalf("immutable guard failed: %s", query)
		}
	}
	var heads, revs, cmds, audits int
	if e = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM reference_oa_cost_entries),(SELECT count(*) FROM reference_oa_cost_revisions),(SELECT count(*) FROM reference_oa_cost_commands),(SELECT count(*) FROM reference_oa_cost_audit)`).Scan(&heads, &revs, &cmds, &audits); e != nil {
		t.Fatal(e)
	}
	if heads != 1 || revs != 3 || cmds != 3 || audits != 3 {
		t.Fatalf("atomic facts %d/%d/%d/%d", heads, revs, cmds, audits)
	}
}
func TestReferenceCostPostgresConcurrentSameCommandHasOneFact(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := referencePool(t)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	s, _ := New(store, &resolver{})
	in := input("production")
	in.Fields.AmountMinor = "0"
	var wg sync.WaitGroup
	values := make(chan Record, 8)
	failures := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, e := s.Create(ctx, actor(), in)
			if e != nil {
				failures <- e
			} else {
				values <- v
			}
		}()
	}
	wg.Wait()
	close(values)
	close(failures)
	for e := range failures {
		t.Fatal(e)
	}
	id := ""
	for v := range values {
		if id == "" {
			id = v.ID
		}
		if v.ID != id {
			t.Fatal("same command duplicated cost")
		}
	}
	var count int
	if e := pool.QueryRow(ctx, `SELECT count(*) FROM reference_oa_cost_entries`).Scan(&count); e != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, e)
	}
}
