//go:build integration

package apikey

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"omsbizfoundry/identity-access/internal/database"
	"omsbizfoundry/identity-access/internal/identity"
)

const (
	testTenant  = "22222222-2222-4222-8222-222222222222"
	testSubject = "11111111-1111-4111-8111-111111111111"
	testPerson  = "33333333-3333-4333-8333-333333333333"
)

type staticSigner struct{}

func (staticSigner) Sign(any) (string, error) { return "signed-open-api-context", nil }

type mutablePeople struct{ value PersonReference }

func (m *mutablePeople) ResolveBySubject(context.Context, string, string) (PersonReference, error) {
	return m.value, nil
}

func TestAPIKeyLifecycleAuditAndRetention(t *testing.T) {
	ctx := context.Background()
	pool := openIsolatedDatabase(t)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 25, 8, 0, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx, `INSERT INTO tenants(id,name,created_at) VALUES($1,'API 测试租户',$2)`, testTenant, now); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO subjects(id,tenant_id,username,display_name,password_hash,status,password_changed_at,created_at,updated_at) VALUES($1,$2,'api-user','API 用户','not-used','active',$3,$3,$3)`, testSubject, testTenant, now); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{ActionUse, ActionKeyManage, ActionSelfAudit} {
		if _, err := pool.Exec(ctx, `INSERT INTO capability_grants(subject_id,audience,action,scope_kind,granted_by,granted_at) VALUES($1,'open-api',$2,'tenant',$1,$3)`, testSubject, action, now); err != nil {
			t.Fatal(err)
		}
	}
	people := &mutablePeople{value: PersonReference{ID: testPerson, TenantID: testTenant, Status: "active", Revision: 4}}
	service, err := New(pool, staticSigner{}, people, "https://oms.example.test/auth")
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return now }
	service.bcryptCost = bcrypt.MinCost
	actor := identity.Principal{TenantID: testTenant, SubjectID: testSubject}

	created, err := service.Create(ctx, actor, "个人报销 CLI", 180)
	if err != nil {
		t.Fatal(err)
	}
	if !secretPattern.MatchString(created.Secret) || created.Prefix == "" || created.PersonID != testPerson || !created.ExpiresAt.Equal(now.AddDate(0, 0, 180)) {
		t.Fatalf("created key = %#v", created)
	}
	var verifier string
	if err := pool.QueryRow(ctx, `SELECT secret_verifier FROM api_keys WHERE id=$1`, created.ID).Scan(&verifier); err != nil {
		t.Fatal(err)
	}
	if verifier == created.Secret || bcrypt.CompareHashAndPassword([]byte(verifier), []byte(created.Secret)) != nil {
		t.Fatal("database must contain only an irreversible verifier")
	}
	keys, err := service.List(ctx, actor)
	if err != nil || len(keys) != 1 || keys[0].Prefix != created.Prefix {
		t.Fatalf("list = %#v, %v", keys, err)
	}

	authenticated, err := service.Authenticate(ctx, created.Secret, "request-00000001", "open-api.profile.read", "203.0.113.0/24")
	if err != nil || authenticated.Credential == "" || authenticated.PersonID != testPerson || authenticated.KeyID != created.ID {
		t.Fatalf("authentication = %#v, %v", authenticated, err)
	}
	if err := service.CompleteUsage(ctx, testTenant, testSubject, created.ID, "request-00000001", "OK", 200); err != nil {
		t.Fatal(err)
	}
	page, err := service.ListUsage(ctx, actor, false, "", 20)
	if err != nil || len(page.Items) != 1 || page.Items[0].SourceIPMasked != "203.0.113.0/24" || page.Items[0].HTTPStatus != 200 {
		t.Fatalf("usage page = %#v, %v", page, err)
	}

	oldUsageID := "44444444-4444-4444-8444-444444444444"
	oldOccurred := now.AddDate(0, 0, -181)
	if _, err := pool.Exec(ctx, `INSERT INTO open_api_usage_records(id,tenant_id,subject_id,person_id,api_key_id,key_prefix,action,request_id,source_ip_masked,result_code,http_status,occurred_at,completed_at) VALUES($1,$2,$3,$4,$5,$6,'open-api.profile.read','request-old-0001','198.51.100.0/24','OK',200,$7,$7)`, oldUsageID, testTenant, testSubject, testPerson, created.ID, created.Prefix, oldOccurred); err != nil {
		t.Fatal(err)
	}
	purge, err := PurgeUsageRecords(ctx, pool, now, "integration-test")
	if err != nil || purge.DeletedCount != 1 || !purge.Cutoff.Equal(now.AddDate(0, 0, -180)) {
		t.Fatalf("purge = %#v, %v", purge, err)
	}
	var usageCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM open_api_usage_records`).Scan(&usageCount); err != nil || usageCount != 1 {
		t.Fatalf("retained usage count = %d, %v", usageCount, err)
	}
	_, err = pool.Exec(ctx, `UPDATE open_api_usage_cleanup_runs SET operator_id='tampered' WHERE id=$1`, purge.RunID)
	var databaseError *pgconn.PgError
	if !errors.As(err, &databaseError) || databaseError.Code != "55000" {
		t.Fatalf("cleanup audit mutation error = %v", err)
	}

	activeSecrets := make([]string, 0, 4)
	for index := 0; index < 4; index++ {
		value, err := service.Create(ctx, actor, "并行工具 "+string(rune('A'+index)), 30)
		if err != nil {
			t.Fatal(err)
		}
		activeSecrets = append(activeSecrets, value.Secret)
	}
	if _, err := service.Create(ctx, actor, "第六个有效 Key", 30); errorCode(err) != "active_api_key_limit" {
		t.Fatalf("sixth key error = %v", err)
	}
	if _, err := service.Revoke(ctx, actor, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, created.Secret, "request-00000002", "open-api.profile.read", "203.0.113.0/24"); errorCode(err) != "invalid_api_key" {
		t.Fatalf("revoked key authentication error = %v", err)
	}

	people.value.Status = "inactive"
	if _, err := service.Authenticate(ctx, activeSecrets[0], "request-00000003", "open-api.profile.read", "203.0.113.0/24"); errorCode(err) != "invalid_api_key" {
		t.Fatalf("inactive Person authentication error = %v", err)
	}
	people.value.Status = "active"
	if _, err := pool.Exec(ctx, `UPDATE capability_grants SET revoked_at=$3,revoked_by=$2 WHERE subject_id=$1 AND audience='open-api' AND action=$4`, testSubject, testSubject, now, ActionUse); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, activeSecrets[0], "request-00000004", "open-api.profile.read", "203.0.113.0/24"); errorCode(err) != "invalid_api_key" {
		t.Fatalf("revoked Grant authentication error = %v", err)
	}
}

func errorCode(err error) string {
	var problem *DomainError
	if errors.As(err, &problem) {
		return problem.Code
	}
	return ""
}

func openIsolatedDatabase(t *testing.T) *pgxpool.Pool {
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
	databaseName := "iam_apikey_test_" + hex.EncodeToString(suffix[:])
	identifier := pgx.Identifier{databaseName}.Sanitize()
	if _, err := maintenance.Exec(context.Background(), "CREATE DATABASE "+identifier); err != nil {
		_ = maintenance.Close(context.Background())
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = maintenance.Exec(context.Background(), "DROP DATABASE "+identifier+" WITH (FORCE)")
		_ = maintenance.Close(context.Background())
	})
	targetConfig := config.Copy()
	targetConfig.ConnConfig.Database = databaseName
	pool, err := pgxpool.NewWithConfig(context.Background(), targetConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}
