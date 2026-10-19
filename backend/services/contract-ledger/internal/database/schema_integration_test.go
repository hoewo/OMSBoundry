//go:build integration

package database

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestExplicitSchemaGateAndAtomicPendingMigration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pool := openIsolatedMigrationDatabase(t, ctx, databaseURL)
	postgres, err := CheckPostgreSQL(ctx, pool)
	if err != nil || postgres.Major != SupportedPostgreSQLMajor {
		t.Fatalf("CheckPostgreSQL() = %#v, %v", postgres, err)
	}
	status, err := InspectSchema(ctx, pool)
	if err != nil || status.State != SchemaEmpty || status.ServerReady {
		t.Fatalf("empty InspectSchema() = %#v, %v", status, err)
	}
	if _, err := pool.Exec(ctx, `CREATE TABLE unmanaged_probe (id integer)`); err != nil {
		t.Fatal(err)
	}
	status, err = InspectSchema(ctx, pool)
	if err != nil || status.State != SchemaIncompatible || !strings.Contains(status.Reason, "user relations") {
		t.Fatalf("unmanaged InspectSchema() = %#v, %v", status, err)
	}
	if err := Migrate(ctx, pool); err == nil || !strings.Contains(err.Error(), "incompatible") {
		t.Fatalf("Migrate(unmanaged database) error = %v", err)
	}
	var migrationTableExists bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass(current_schema() || '.schema_migrations') IS NOT NULL`).Scan(&migrationTableExists); err != nil {
		t.Fatal(err)
	}
	if migrationTableExists {
		t.Fatal("rejected unmanaged database gained schema_migrations")
	}
	if _, err := pool.Exec(ctx, `DROP TABLE unmanaged_probe`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		CREATE TABLE schema_migrations (
			version text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT clock_timestamp()
		)
	`); err != nil {
		t.Fatal(err)
	}

	connection, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	versions, err := migrationVersions()
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range versions[:len(versions)-1] {
		if err := applyMigration(ctx, connection, version); err != nil {
			connection.Release()
			t.Fatalf("apply historical migration %s: %v", version, err)
		}
	}
	connection.Release()
	status, err = InspectSchema(ctx, pool)
	if err != nil || status.State != SchemaPending || len(status.Pending) != 1 || status.Pending[0] != versions[len(versions)-1] {
		t.Fatalf("pending InspectSchema() = %#v, %v", status, err)
	}
	if _, err := RequireCurrentSchema(ctx, pool); err == nil {
		t.Fatal("RequireCurrentSchema() accepted pending database")
	}

	if _, err := pool.Exec(ctx, `CREATE TABLE reference_oa_materials (dummy integer)`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, pool); err == nil || !strings.Contains(err.Error(), "execute migration") {
		t.Fatalf("Migrate(conflicting latest migration) error = %v", err)
	}
	var commandIDColumns, latestMigrationRows int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND table_name = 'reference_oa_materials'
		  AND column_name = 'contract_id'
	`).Scan(&commandIDColumns); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations WHERE version = $1`, versions[len(versions)-1]).Scan(&latestMigrationRows); err != nil {
		t.Fatal(err)
	}
	if commandIDColumns != 0 || latestMigrationRows != 0 {
		t.Fatalf("failed migration left command_id=%d migration_rows=%d", commandIDColumns, latestMigrationRows)
	}
	if _, err := pool.Exec(ctx, `DROP TABLE reference_oa_materials`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate() after conflict removal: %v", err)
	}
	status, err = RequireCurrentSchema(ctx, pool)
	if err != nil || status.State != SchemaCurrent || !status.ServerReady {
		t.Fatalf("current RequireCurrentSchema() = %#v, %v", status, err)
	}

	if _, err := pool.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ('999999_future.sql')`); err != nil {
		t.Fatal(err)
	}
	status, err = InspectSchema(ctx, pool)
	if err != nil || status.State != SchemaIncompatible || len(status.Unknown) != 1 {
		t.Fatalf("unknown InspectSchema() = %#v, %v", status, err)
	}
	if err := Migrate(ctx, pool); err == nil || !strings.Contains(err.Error(), "incompatible") {
		t.Fatalf("Migrate(unknown schema) error = %v", err)
	}
}

func TestConcurrentExplicitMigrationsSerialize(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pool := openIsolatedMigrationDatabase(t, ctx, databaseURL)
	start := make(chan struct{})
	errorsByWorker := make([]error, 2)
	var workers sync.WaitGroup
	for index := range errorsByWorker {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			errorsByWorker[index] = Migrate(ctx, pool)
		}()
	}
	close(start)
	workers.Wait()
	for index, err := range errorsByWorker {
		if err != nil {
			t.Fatalf("concurrent Migrate worker %d: %v", index, err)
		}
	}
	status, err := RequireCurrentSchema(ctx, pool)
	if err != nil || status.State != SchemaCurrent || len(status.Applied) != 24 {
		t.Fatalf("schema after concurrent migrations = %#v, %v", status, err)
	}
}
