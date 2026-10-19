//go:build integration

package referencebonus

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	core "omsbizfoundry/bonus-distribution/internal/bonus"
	"omsbizfoundry/bonus-distribution/internal/database"
)

func integrationPool(t *testing.T) (*pgxpool.Pool, func()) {
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
	schema := "manual_bonus_test_" + fmt.Sprintf("%x", newID())
	if _, e = admin.Exec(ctx, `CREATE SCHEMA `+schema); e != nil {
		t.Fatal(e)
	}
	cfg, e := pgxpool.ParseConfig(url)
	if e != nil {
		t.Fatal(e)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	p, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	cleanup := func() {
		p.Close()
		if _, e := admin.Exec(context.Background(), `DROP SCHEMA `+schema+` CASCADE`); e != nil {
			t.Error(e)
		}
		admin.Close()
	}
	if e = database.Migrate(ctx, p); e != nil {
		cleanup()
		t.Fatal(e)
	}
	return p, cleanup
}
func TestPostgresManualBonusAtomicLifecycleConcurrencyAndAppendOnly(t *testing.T) {
	p, cleanup := integrationPool(t)
	defer cleanup()
	ctx := context.Background()
	m, _ := NewPostgresStore(p)
	a, f, in := fixture()
	s, _ := New(m, f)
	// Force failure after the first item has been resolved: no partial batch.
	key := in.Items[1].AssignmentRef.ID
	valid := f.values[key]
	invalid := valid
	invalid.Active = false
	f.values[key] = invalid
	if _, e := s.Create(ctx, a, in); !errors.Is(e, core.ErrReferenceInvalid) {
		t.Fatal(e)
	}
	var count int
	if e := p.QueryRow(ctx, `SELECT count(*) FROM reference_oa_bonus_allocations`).Scan(&count); e != nil || count != 0 {
		t.Fatalf("partial rows %d %v", count, e)
	}
	f.values[key] = valid
	values, e := s.Create(ctx, a, in)
	if e != nil || len(values) != 2 {
		t.Fatal(e)
	}
	frozen := values
	f.fail = true
	again, e := s.Create(ctx, a, in)
	if e != nil || !reflect.DeepEqual(frozen, again) {
		t.Fatalf("frozen replay=%v", e)
	}
	// Exercise the transactional command lock directly; all writers return one response.
	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, e := m.Commit(ctx, a, in.CommandID, hash(a, "record", "", in), "record", "", 0, values)
			if e != nil {
				errs <- e
				return
			}
			if !reflect.DeepEqual(v, frozen) {
				errs <- fmt.Errorf("concurrent replay differs")
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
	if e = p.QueryRow(ctx, `SELECT count(*) FROM reference_oa_bonus_commands`).Scan(&count); e != nil || count != 1 {
		t.Fatalf("duplicate commands %d %v", count, e)
	}
	request := WithdrawInput{CommandID: testID(31), ExpectedRevision: 1, Reason: "留史撤回"}
	v, e := s.Withdraw(ctx, a, values[0].ID, request)
	if e != nil || v.Revision != 2 {
		t.Fatalf("withdraw=%+v %v", v, e)
	}
	replay, e := s.Withdraw(ctx, a, v.ID, request)
	if e != nil || !reflect.DeepEqual(v, replay) {
		t.Fatal("withdraw replay", e)
	}
	h, e := s.History(ctx, a, v.ID)
	if e != nil || len(h) != 2 || h[0].Snapshot.State != "withdrawn" || h[1].Snapshot.AmountMinor != 500 {
		t.Fatal("history not frozen", e)
	}
	for _, query := range []string{`DELETE FROM reference_oa_bonus_allocations`, `UPDATE reference_oa_bonus_allocations SET snapshot=jsonb_set(snapshot,'{amount_minor}','1')`, `UPDATE reference_oa_bonus_revisions SET reason='changed'`, `DELETE FROM reference_oa_bonus_revisions`, `DELETE FROM reference_oa_bonus_commands`, `DELETE FROM reference_oa_bonus_audit`} {
		if _, e = p.Exec(ctx, query); e == nil {
			t.Fatalf("mutation accepted: %s", query)
		}
	}
	if e = p.QueryRow(ctx, `SELECT count(*) FROM bonus_pools`).Scan(&count); e != nil || count != 0 {
		t.Fatal("manual bonus leaked into Pool", e)
	}
	if e = p.QueryRow(ctx, `SELECT count(*) FROM bonus_payable_suggestions`).Scan(&count); e != nil || count != 0 {
		t.Fatal("manual bonus created Money suggestion", e)
	}
	other := a
	other.TenantID = testID(99)
	if _, e = s.Get(ctx, other, v.ID); !errors.Is(e, core.ErrNotFound) {
		t.Fatal("cross tenant read", e)
	}
}
func TestPostgresManualBonusSimultaneousNewCommandIsOneAtomicBatch(t *testing.T) {
	p, cleanup := integrationPool(t)
	defer cleanup()
	ctx := context.Background()
	m, _ := NewPostgresStore(p)
	a, f, in := fixture()
	s, _ := New(m, f)
	values, e := s.Create(ctx, a, in)
	if e != nil {
		t.Fatal(e)
	}
	// Re-use fully validated snapshots with fresh native identities in a new command.
	batch := newID()
	for i := range values {
		values[i].ID = newID()
		values[i].BatchID = batch
	}
	cmd := testID(40)
	payload := hash(a, "record", "", in)
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := m.Commit(ctx, a, cmd, payload, "record", "", 0, values)
			if e != nil {
				errs <- e
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
	var rows, commands, histories, audits int
	e = p.QueryRow(ctx, `SELECT (SELECT count(*) FROM reference_oa_bonus_allocations),(SELECT count(*) FROM reference_oa_bonus_commands),(SELECT count(*) FROM reference_oa_bonus_revisions),(SELECT count(*) FROM reference_oa_bonus_audit)`).Scan(&rows, &commands, &histories, &audits)
	if e != nil || rows != 4 || commands != 2 || histories != 4 || audits != 4 {
		t.Fatalf("atomic counts=%d/%d/%d/%d %v", rows, commands, histories, audits, e)
	}
}
