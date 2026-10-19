//go:build integration

package referencebonus

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"omsbizfoundry/bonus-distribution/internal/database"
	"os"
	"reflect"
	"testing"
)

func TestProfessionalSchema3To4PreservesV2FactsHashAndOfflineReplay(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "bonus_upgrade3_" + fmt.Sprintf("%x", newID())
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec(context.Background(), `DROP SCHEMA `+schema+` CASCADE`); err != nil {
			t.Error(err)
		}
	}()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err = p.Exec(ctx, `CREATE TABLE schema_migrations(version text PRIMARY KEY,applied_at timestamptz NOT NULL DEFAULT clock_timestamp())`); err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"0001_bonus_distribution.sql", "0002_bonus_payable_suggestions.sql", "0003_reference_oa_manual_allocations.sql"} {
		script, e := os.ReadFile("../database/migrations/" + version)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = p.Exec(ctx, string(script)); e != nil {
			t.Fatal(e)
		}
		if _, e = p.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, version); e != nil {
			t.Fatal(e)
		}
	}
	store, _ := NewPostgresStore(p)
	a, f, in := fixture()
	v2, _ := New(store, f)
	rows, err := v2.Create(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = v2.Withdraw(ctx, a, rows[0].ID, WithdrawInput{CommandID: testID(401), ExpectedRevision: 1, Reason: "升级前撤回"}); err != nil {
		t.Fatal(err)
	}
	const canonical = `SELECT md5(jsonb_build_object('heads',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM reference_oa_bonus_allocations p),'history',(SELECT jsonb_agg(to_jsonb(p) ORDER BY allocation_id,revision) FROM reference_oa_bonus_revisions p),'commands',(SELECT jsonb_agg(to_jsonb(p) ORDER BY command_id) FROM reference_oa_bonus_commands p),'audit',(SELECT jsonb_agg(to_jsonb(p) ORDER BY command_id,allocation_id) FROM reference_oa_bonus_audit p))::text)`
	var before, after string
	if err = p.QueryRow(ctx, canonical).Scan(&before); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = database.Migrate(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = database.RequireCurrentSchema(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err = p.QueryRow(ctx, canonical).Scan(&after); err != nil || after != before {
		t.Fatal("migration rewrote v2 facts", err)
	}
	professionalActor, professional, in3 := professionalTestFixture()
	v3, _ := NewWithProfile(store, professional, ProfessionalProfile)
	professional.fail = true
	// A v2 command retains its exact hash and frozen response under the v3
	// composition even when all source resolutions are unavailable.
	again, err := v3.Create(ctx, a, in)
	if err != nil || !reflect.DeepEqual(rows, again) {
		t.Fatalf("offline v2 replay=%+v %v", again, err)
	}
	if err = p.QueryRow(ctx, canonical).Scan(&after); err != nil || after != before {
		t.Fatal("replay changed facts", err)
	}
	professional.fail = false
	in3.CommandID = testID(402)
	newRows, err := v3.Create(ctx, professionalActor, in3)
	if err != nil || len(newRows) != 6 {
		t.Fatalf("v3 after upgrade=%+v %v", newRows, err)
	}
	if _, err = v3.Withdraw(ctx, a, rows[1].ID, WithdrawInput{CommandID: testID(403), ExpectedRevision: 1, Reason: "升级后继续旧对象"}); err != nil {
		t.Fatal("legacy continuation", err)
	}
}
