//go:build integration

package database

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This is a private schema-18 fixture, not a migration of the live Main DB.
func TestReferenceOAUpgradeFrom18PreservesFactsAndRollsBackAtomically(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	pool := openIsolatedMigrationDatabase(t, ctx, databaseURL)
	connection, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connection.Exec(ctx, `CREATE TABLE schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT clock_timestamp())`); err != nil {
		t.Fatal(err)
	}
	versions, err := migrationVersions()
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range versions[:6] {
		if err := applyMigration(ctx, connection, version); err != nil {
			t.Fatal(err)
		}
	}
	seedHistoricalFacts(t, ctx, pool)
	for _, version := range versions[6:18] {
		if err := applyMigration(ctx, connection, version); err != nil {
			t.Fatal(err)
		}
	}
	connection.Release()
	status, err := InspectSchema(ctx, pool)
	if err != nil || status.Current != "000018_party_migration_mappings.sql" || len(status.Pending) != 6 {
		t.Fatalf("legacy gate = %#v, %v", status, err)
	}
	if _, err := RequireCurrentSchema(ctx, pool); err == nil {
		t.Fatal("candidate accepted schema 18 without explicit migration")
	}
	tables := upgradeOwnedTables(t, ctx, pool)
	before := upgradeOwnedFacts(t, ctx, pool, tables)
	if _, err := pool.Exec(ctx, `CREATE TABLE reference_oa_materials (dummy integer)`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, pool); err == nil || !strings.Contains(err.Error(), "000022_reference_oa_materials.sql") {
		t.Fatalf("conflict did not reject whole upgrade: %v", err)
	}
	status, err = InspectSchema(ctx, pool)
	if err != nil || status.Current != "000018_party_migration_mappings.sql" || len(status.Applied) != 18 {
		t.Fatalf("failed transaction advanced schema: %#v, %v", status, err)
	}
	var leaked int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema=current_schema() AND table_name IN ('party_runtime_transitions','reference_oa_contract_drafts','reference_oa_amendment_confirmations')`).Scan(&leaked); err != nil || leaked != 0 {
		t.Fatalf("failed upgrade leaked %d tables: %v", leaked, err)
	}
	if !reflect.DeepEqual(before, upgradeOwnedFacts(t, ctx, pool, tables)) {
		t.Fatal("failed upgrade changed existing facts")
	}
	if _, err := pool.Exec(ctx, `DROP TABLE reference_oa_materials`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	status, err = RequireCurrentSchema(ctx, pool)
	if err != nil || status.Current != "000024_contract_external_evidence_revisions.sql" || len(status.Applied) != 24 {
		t.Fatalf("upgraded gate = %#v, %v", status, err)
	}
	if !reflect.DeepEqual(before, upgradeOwnedFacts(t, ctx, pool, tables)) {
		t.Fatal("18→24 changed old ID/revision/snapshot/command/audit facts")
	}
	for _, table := range []string{"party_runtime_transitions", "reference_oa_contract_drafts", "reference_oa_contract_draft_commands", "reference_oa_amendment_confirmations", "reference_oa_materials", "reference_oa_material_commands", "contract_external_evidence_revisions"} {
		var rows int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{table}.Sanitize()).Scan(&rows); err != nil || rows != 0 {
			t.Fatalf("migration invented facts in %s: %d, %v", table, rows, err)
		}
	}
	allTables := upgradeOwnedTables(t, ctx, pool)
	after := upgradeOwnedFacts(t, ctx, pool, allTables)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, upgradeOwnedFacts(t, ctx, pool, allTables)) {
		t.Fatal("migration replay changed owned facts")
	}
	t.Logf("private schema18→22; %d legacy tables unchanged; conflict rollback and replay preserved facts", len(tables))
}

func upgradeOwnedTables(t *testing.T, ctx context.Context, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema=current_schema() AND table_type='BASE TABLE' AND table_name<>'schema_migrations' ORDER BY table_name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return tables
}

func upgradeOwnedFacts(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tables []string) map[string]string {
	t.Helper()
	result := map[string]string{}
	for _, table := range tables {
		var canonical string
		if err := pool.QueryRow(ctx, "SELECT coalesce(jsonb_agg(to_jsonb(owned_row) ORDER BY to_jsonb(owned_row)::text),'[]'::jsonb)::text FROM "+pgx.Identifier{table}.Sanitize()+" AS owned_row").Scan(&canonical); err != nil {
			t.Fatalf("canonical %s: %v", table, err)
		}
		result[table] = canonical
	}
	return result
}
