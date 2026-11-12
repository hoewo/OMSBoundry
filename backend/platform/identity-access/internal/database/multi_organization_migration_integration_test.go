//go:build integration

package database

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSchemasTwelveAndThirteenBackfillAccountsAndLastUsedOrganization(t *testing.T) {
	pool := openMigrationDatabase(t)
	ctx := context.Background()
	versions, err := migrationVersions()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `CREATE TABLE schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT clock_timestamp())`); err != nil {
		t.Fatal(err)
	}
	for _, version := range versions {
		if version == "000012_multi_organization_accounts.sql" {
			break
		}
		script, err := migrationFiles.ReadFile("migrations/" + version)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(script)); err != nil {
			t.Fatalf("apply %s: %v", version, err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, version); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	const tenantA = "11111111-1111-4111-8111-111111111111"
	const tenantB = "22222222-2222-4222-8222-222222222222"
	const subjectA = "33333333-3333-4333-8333-333333333333"
	const subjectB = "44444444-4444-4444-8444-444444444444"
	if _, err := pool.Exec(ctx, `INSERT INTO tenants(id,name,created_at) VALUES($1,'组织 A',$3),($2,'组织 B',$3)`, tenantA, tenantB, now); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO subjects(id,tenant_id,username,display_name,password_hash,status,administrator,revision,session_generation,password_changed_at,created_at,updated_at) VALUES($1,$2,'member@example.com','既有账号','$2a$10$not-a-real-login-hash-but-schema-valid','active',true,2,3,$3,$3,$3)`, subjectA, tenantA, now); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var accountID, loginIdentifier, linkedAccountID, lastUsedSubjectID string
	if err := pool.QueryRow(ctx, `SELECT a.id::text,a.login_identifier,s.account_id::text,a.last_used_subject_id::text FROM login_accounts a JOIN subjects s ON s.id=a.primary_subject_id WHERE s.id=$1`, subjectA).Scan(&accountID, &loginIdentifier, &linkedAccountID, &lastUsedSubjectID); err != nil {
		t.Fatal(err)
	}
	if accountID != subjectA || linkedAccountID != subjectA || loginIdentifier != "member@example.com" || lastUsedSubjectID != subjectA {
		t.Fatalf("backfill account=%q login=%q subject_account=%q last_used=%q", accountID, loginIdentifier, linkedAccountID, lastUsedSubjectID)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO subjects(id,account_id,tenant_id,username,display_name,password_hash,status,administrator,revision,session_generation,password_changed_at,created_at,updated_at) VALUES($1,$2,$3,'member@example.com','组织 B 成员','$2a$10$not-a-real-login-hash-but-schema-valid','active',false,1,1,$4,$4,$4)`, subjectB, accountID, tenantB, now); err != nil {
		t.Fatalf("cross-tenant membership: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO subjects(id,account_id,tenant_id,username,display_name,password_hash,status,administrator,revision,session_generation,password_changed_at,created_at,updated_at) VALUES('55555555-5555-4555-8555-555555555555',$1,$2,'member@example.com','重复成员','$2a$10$not-a-real-login-hash-but-schema-valid','active',false,1,1,$3,$3,$3)`, accountID, tenantA, now); err == nil {
		t.Fatal("same account joined one tenant twice")
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
}

func openMigrationDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	baseURL := os.Getenv("TEST_DATABASE_URL")
	if baseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	config, err := pgxpool.ParseConfig(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	maintenanceConfig := config.ConnConfig.Copy()
	maintenanceConfig.Database = "postgres"
	maintenance, err := pgx.ConnectConfig(context.Background(), maintenanceConfig)
	if err != nil {
		t.Fatal(err)
	}
	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	databaseName := "iam_migration_test_" + hex.EncodeToString(suffix[:])
	identifier := pgx.Identifier{databaseName}.Sanitize()
	if _, err := maintenance.Exec(context.Background(), "CREATE DATABASE "+identifier); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = maintenance.Exec(context.Background(), "DROP DATABASE "+identifier+" WITH (FORCE)")
		_ = maintenance.Close(context.Background())
	})
	target := config.Copy()
	target.ConnConfig.Database = databaseName
	pool, err := pgxpool.NewWithConfig(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}
