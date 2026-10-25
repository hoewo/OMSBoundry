//go:build integration

package database

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"omsbizfoundry/money-flow-ledger/internal/testdatabase"
)

func TestExplicitSchemaGateAndAtomicPendingMigrations(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	postgres, err := CheckPostgreSQL(ctx, environment.Pool)
	if err != nil || postgres.Major != SupportedPostgreSQLMajor {
		t.Fatalf("CheckPostgreSQL() = %#v, %v", postgres, err)
	}
	status, err := InspectSchema(ctx, environment.Pool)
	if err != nil || status.State != SchemaEmpty || status.ServerReady {
		t.Fatalf("empty InspectSchema() = %#v, %v", status, err)
	}
	if _, err := environment.Pool.Exec(ctx, `CREATE TABLE unmanaged_probe (id integer)`); err != nil {
		t.Fatal(err)
	}
	status, err = InspectSchema(ctx, environment.Pool)
	if err != nil || status.State != SchemaIncompatible || !strings.Contains(status.Reason, "user relations") {
		t.Fatalf("unmanaged InspectSchema() = %#v, %v", status, err)
	}
	if err := Migrate(ctx, environment.Pool); err == nil || !strings.Contains(err.Error(), "incompatible") {
		t.Fatalf("Migrate(unmanaged database) error = %v", err)
	}
	var migrationTableExists bool
	if err := environment.Pool.QueryRow(ctx, `SELECT to_regclass(current_schema() || '.schema_migrations') IS NOT NULL`).Scan(&migrationTableExists); err != nil {
		t.Fatal(err)
	}
	if migrationTableExists {
		t.Fatal("rejected unmanaged database gained schema_migrations")
	}
	if _, err := environment.Pool.Exec(ctx, `DROP TABLE unmanaged_probe`); err != nil {
		t.Fatal(err)
	}
	if _, err := environment.Pool.Exec(ctx, `
		CREATE TABLE schema_migrations (
			version text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT clock_timestamp()
		)
	`); err != nil {
		t.Fatal(err)
	}
	connection, err := environment.Pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	versions, err := migrationVersions()
	if err != nil {
		connection.Release()
		t.Fatal(err)
	}
	for _, version := range versions[:12] {
		if err := applyMigration(ctx, connection, version); err != nil {
			connection.Release()
			t.Fatalf("apply historical migration %s: %v", version, err)
		}
	}
	connection.Release()
	status, err = InspectSchema(ctx, environment.Pool)
	if err != nil || status.State != SchemaPending || len(status.Pending) != 13 {
		t.Fatalf("pending InspectSchema() = %#v, %v", status, err)
	}
	if _, err := RequireCurrentSchema(ctx, environment.Pool); err == nil {
		t.Fatal("RequireCurrentSchema() accepted pending database")
	}
	if _, err := environment.Pool.Exec(ctx, `
		CREATE FUNCTION obligation_latest_recorded_fact_at(uuid)
		RETURNS timestamptz LANGUAGE sql AS $$ SELECT NULL::timestamptz $$
	`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, environment.Pool); err == nil || !strings.Contains(err.Error(), "execute migration") {
		t.Fatalf("Migrate(conflicting 000014) error = %v", err)
	}
	var migration13Rows int
	var closureTable *string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM schema_migrations WHERE version = '000013_obligation_closures.sql'),
			to_regclass('public.obligation_closures')::text
	`).Scan(&migration13Rows, &closureTable); err != nil {
		t.Fatal(err)
	}
	if migration13Rows != 0 || closureTable != nil {
		t.Fatalf("failed migration left migration13=%d closure_table=%v", migration13Rows, closureTable)
	}
	if _, err := environment.Pool.Exec(ctx, `DROP FUNCTION obligation_latest_recorded_fact_at(uuid)`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("Migrate() after conflict removal: %v", err)
	}
	status, err = RequireCurrentSchema(ctx, environment.Pool)
	if err != nil || status.State != SchemaCurrent || !status.ServerReady {
		t.Fatalf("current RequireCurrentSchema() = %#v, %v", status, err)
	}
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO local_users (
			id, username, display_name, password_hash, role, status,
			password_changed_at, created_at, updated_at
		) VALUES (
			'81000000-0000-4000-8000-000000000001', 'legacy.user', '历史本地资金用户',
			repeat('x', 60), 'administrator', 'active',
			'2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'
		);
		INSERT INTO local_user_iam_subject_links (
			local_user_id, tenant_id, iam_subject_id,
			linked_by_iam_subject_id, linked_by_display_name, reason, linked_at
		) VALUES (
			'81000000-0000-4000-8000-000000000001',
			'82000000-0000-4000-8000-000000000001',
			'83000000-0000-4000-8000-000000000001',
			'84000000-0000-4000-8000-000000000001',
			'IAM 迁移管理员', 'M7 Money Flow 历史身份显式映射验收', '2026-09-21T00:00:00Z'
		)
	`); err != nil {
		t.Fatalf("seed explicit local-to-IAM identity link: %v", err)
	}
	_, err = environment.Pool.Exec(ctx, `UPDATE local_user_iam_subject_links SET reason='tampered' WHERE local_user_id='81000000-0000-4000-8000-000000000001'`)
	var identityLinkError *pgconn.PgError
	if !errors.As(err, &identityLinkError) || identityLinkError.Code != "55000" {
		t.Fatalf("immutable identity link update error = %v", err)
	}
	if _, err := environment.Pool.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ('999999_future.sql')`); err != nil {
		t.Fatal(err)
	}
	status, err = InspectSchema(ctx, environment.Pool)
	if err != nil || status.State != SchemaIncompatible || len(status.Unknown) != 1 {
		t.Fatalf("unknown InspectSchema() = %#v, %v", status, err)
	}
	if err := Migrate(ctx, environment.Pool); err == nil || !strings.Contains(err.Error(), "incompatible") {
		t.Fatalf("Migrate(unknown schema) error = %v", err)
	}
}

func TestConcurrentExplicitMigrationsSerialize(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	start := make(chan struct{})
	errorsByWorker := make([]error, 2)
	var workers sync.WaitGroup
	for index := range errorsByWorker {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			errorsByWorker[index] = Migrate(ctx, environment.Pool)
		}()
	}
	close(start)
	workers.Wait()
	for index, err := range errorsByWorker {
		if err != nil {
			t.Fatalf("concurrent Migrate worker %d: %v", index, err)
		}
	}
	status, err := RequireCurrentSchema(ctx, environment.Pool)
	if err != nil || status.State != SchemaCurrent || len(status.Applied) != 25 {
		t.Fatalf("schema after concurrent migrations = %#v, %v", status, err)
	}
}
