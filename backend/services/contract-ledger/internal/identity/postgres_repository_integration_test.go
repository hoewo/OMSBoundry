//go:build integration

package identity

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"omsbizfoundry/contract-ledger/internal/database"
)

func TestPostgresIdentityLifecycleAndAuditAtomicity(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pool := openIsolatedIdentityDatabase(t, ctx, databaseURL)

	repository := NewPostgresRepository(pool)
	service, err := newService(repository, 2*time.Hour, bcrypt.MinCost)
	if err != nil {
		t.Fatalf("newService() error = %v", err)
	}
	baseTime := time.Date(2026, 9, 19, 8, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return baseTime }

	_, err = service.EnsureBootstrap(ctx, BootstrapInput{})
	var validation *ValidationError
	if !errors.As(err, &validation) || validation.Field != "bootstrap" {
		t.Fatalf("empty bootstrap error = %v", err)
	}
	created, err := service.EnsureBootstrap(ctx, BootstrapInput{
		Username: "admin", DisplayName: "首位管理员", Password: "initial-admin-password",
	})
	if err != nil || !created {
		t.Fatalf("EnsureBootstrap() = %v, error = %v", created, err)
	}
	created, err = service.EnsureBootstrap(ctx, BootstrapInput{})
	if err != nil || created {
		t.Fatalf("repeated EnsureBootstrap() = %v, error = %v", created, err)
	}

	var adminID, passwordHash string
	if err := pool.QueryRow(ctx, `SELECT id::text, password_hash FROM local_users WHERE username = 'admin'`).Scan(&adminID, &passwordHash); err != nil {
		t.Fatalf("read bootstrap admin: %v", err)
	}
	if passwordHash == "initial-admin-password" || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte("initial-admin-password")) != nil {
		t.Fatalf("bootstrap password is not safely hashed")
	}
	assertAuditCount(t, ctx, pool, ActionUserBootstrapped, 1)

	for attempt := 1; attempt <= 5; attempt++ {
		_, err := service.Login(ctx, "admin", "wrong-password")
		var authentication *AuthenticationError
		if !errors.As(err, &authentication) {
			t.Fatalf("wrong login attempt %d error = %v", attempt, err)
		}
	}
	var failedAttempts int
	var lockedUntil time.Time
	if err := pool.QueryRow(ctx, `SELECT failed_login_attempts, locked_until FROM local_users WHERE id = $1`, adminID).Scan(&failedAttempts, &lockedUntil); err != nil {
		t.Fatalf("read login lock: %v", err)
	}
	if failedAttempts != 5 || !lockedUntil.Equal(baseTime.Add(15*time.Minute)) {
		t.Fatalf("login lock attempts=%d until=%s", failedAttempts, lockedUntil)
	}
	if _, err := service.Login(ctx, "admin", "initial-admin-password"); err == nil {
		t.Fatal("locked account login error = nil")
	}

	service.now = func() time.Time { return baseTime.Add(16 * time.Minute) }
	adminLogin, err := service.Login(ctx, "admin", "initial-admin-password")
	if err != nil {
		t.Fatalf("admin Login() after lock expiry error = %v", err)
	}
	resolvedAdmin, found, err := service.ResolveSession(ctx, adminLogin.Token)
	if err != nil || !found || resolvedAdmin.UserID != adminID {
		t.Fatalf("ResolveSession() = %#v, found=%v, error=%v", resolvedAdmin, found, err)
	}
	var storedRawTokenCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM local_sessions WHERE encode(token_hash, 'base64') = $1`, adminLogin.Token).Scan(&storedRawTokenCount); err != nil {
		t.Fatalf("check raw session token: %v", err)
	}
	if storedRawTokenCount != 0 {
		t.Fatalf("database contains raw session token")
	}

	editor, err := service.CreateUser(ctx, CreateUserInput{
		Username: "editor", DisplayName: "编辑者", Password: "initial-editor-password", Role: "editor", Actor: resolvedAdmin.Actor(),
	})
	if err != nil {
		t.Fatalf("CreateUser(editor) error = %v", err)
	}
	secondAdmin, err := service.CreateUser(ctx, CreateUserInput{
		Username: "admin-two", DisplayName: "第二管理员", Password: "second-admin-password", Role: "administrator", Actor: resolvedAdmin.Actor(),
	})
	if err != nil {
		t.Fatalf("CreateUser(admin-two) error = %v", err)
	}
	assertAuditCount(t, ctx, pool, ActionUserCreated, 2)

	editorLogin, err := service.Login(ctx, "editor", "initial-editor-password")
	if err != nil {
		t.Fatalf("editor Login() error = %v", err)
	}
	if err := service.ChangeStatus(ctx, ChangeStatusInput{
		UserID: editor.ID, Status: "disabled", Actor: resolvedAdmin.Actor(),
	}); err != nil {
		t.Fatalf("disable editor error = %v", err)
	}
	if _, found, err := service.ResolveSession(ctx, editorLogin.Token); err != nil || found {
		t.Fatalf("disabled editor session found=%v error=%v", found, err)
	}
	if err := service.ChangeStatus(ctx, ChangeStatusInput{
		UserID: editor.ID, Status: "active", Actor: resolvedAdmin.Actor(),
	}); err != nil {
		t.Fatalf("enable editor error = %v", err)
	}
	if _, found, err := service.ResolveSession(ctx, editorLogin.Token); err != nil || found {
		t.Fatalf("old editor session revived found=%v error=%v", found, err)
	}

	if err := service.ResetPassword(ctx, ResetPasswordInput{
		UserID: editor.ID, NewPassword: "reset-editor-password", Actor: resolvedAdmin.Actor(),
	}); err != nil {
		t.Fatalf("ResetPassword(editor) error = %v", err)
	}
	if _, err := service.Login(ctx, "editor", "initial-editor-password"); err == nil {
		t.Fatal("old editor password still works")
	}
	if _, err := service.Login(ctx, "editor", "reset-editor-password"); err != nil {
		t.Fatalf("reset editor password login error = %v", err)
	}

	if err := service.ChangeRole(ctx, ChangeRoleInput{
		UserID: secondAdmin.ID, Role: "viewer", Actor: resolvedAdmin.Actor(),
	}); err != nil {
		t.Fatalf("demote second administrator error = %v", err)
	}
	err = service.ChangeRole(ctx, ChangeRoleInput{
		UserID: adminID, Role: "viewer", Actor: resolvedAdmin.Actor(),
	})
	var lastAdministrator *LastAdministratorError
	if !errors.As(err, &lastAdministrator) {
		t.Fatalf("last administrator error = %v", err)
	}
	err = service.ChangeStatus(ctx, ChangeStatusInput{
		UserID: adminID, Status: "disabled", Actor: resolvedAdmin.Actor(),
	})
	var selfDisable *SelfDisableError
	if !errors.As(err, &selfDisable) {
		t.Fatalf("self disable error = %v", err)
	}

	if err := service.ChangePassword(ctx, ChangePasswordInput{
		UserID: adminID, CurrentPassword: "initial-admin-password", NewPassword: "changed-admin-password",
		SessionGeneration: resolvedAdmin.SessionGeneration, Actor: resolvedAdmin.Actor(),
	}); err != nil {
		t.Fatalf("ChangePassword(admin) error = %v", err)
	}
	if _, found, err := service.ResolveSession(ctx, adminLogin.Token); err != nil || found {
		t.Fatalf("old admin session found=%v error=%v", found, err)
	}
	if _, err := service.Login(ctx, "admin", "changed-admin-password"); err != nil {
		t.Fatalf("changed admin password login error = %v", err)
	}

	var existingAuditID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM audit_entries ORDER BY occurred_at, id LIMIT 1`).Scan(&existingAuditID); err != nil {
		t.Fatalf("read existing audit ID: %v", err)
	}
	ids := []string{"dddddddd-dddd-4ddd-8ddd-dddddddddddd", existingAuditID}
	service.newID = func() (string, error) {
		value := ids[0]
		ids = ids[1:]
		return value, nil
	}
	_, err = service.CreateUser(ctx, CreateUserInput{
		Username: "rollback-user", DisplayName: "回滚用户", Password: "rollback-user-password", Role: "viewer", Actor: resolvedAdmin.Actor(),
	})
	if err == nil {
		t.Fatal("CreateUser() with duplicate audit ID error = nil")
	}
	var rollbackUserCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM local_users WHERE username = 'rollback-user'`).Scan(&rollbackUserCount); err != nil {
		t.Fatalf("count rollback user: %v", err)
	}
	if rollbackUserCount != 0 {
		t.Fatalf("audit failure left rollback user")
	}

	_, err = pool.Exec(ctx, `UPDATE audit_entries SET summary = 'tampered' WHERE id = $1`, existingAuditID)
	var databaseError *pgconn.PgError
	if !errors.As(err, &databaseError) || databaseError.Code != "55000" {
		t.Fatalf("immutable audit update error = %v", err)
	}

	for _, action := range []string{
		ActionUserBootstrapped, ActionUserCreated, ActionUserStatusChanged,
		ActionUserPasswordReset, ActionUserRoleChanged, ActionUserPasswordChange,
	} {
		assertAuditCountAtLeast(t, ctx, pool, action, 1)
	}
}

func assertAuditCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, action string, want int) {
	t.Helper()
	var got int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_entries WHERE action = $1`, action).Scan(&got); err != nil {
		t.Fatalf("count audit %q: %v", action, err)
	}
	if got != want {
		t.Fatalf("audit %q count = %d, want %d", action, got, want)
	}
}

func assertAuditCountAtLeast(t *testing.T, ctx context.Context, pool *pgxpool.Pool, action string, minimum int) {
	t.Helper()
	var got int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_entries WHERE action = $1`, action).Scan(&got); err != nil {
		t.Fatalf("count audit %q: %v", action, err)
	}
	if got < minimum {
		t.Fatalf("audit %q count = %d, want at least %d", action, got, minimum)
	}
}

func openIsolatedIdentityDatabase(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
	t.Helper()
	baseConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	adminConfig := baseConfig.Copy()
	adminConfig.ConnConfig.Database = "postgres"
	adminPool, err := pgxpool.NewWithConfig(ctx, adminConfig)
	if err != nil {
		t.Fatalf("open administration database: %v", err)
	}
	defer adminPool.Close()

	databaseName := fmt.Sprintf("contract_ledger_identity_%d", time.Now().UnixNano())
	quotedName := pgx.Identifier{databaseName}.Sanitize()
	if _, err := adminPool.Exec(ctx, `CREATE DATABASE `+quotedName); err != nil {
		t.Fatalf("create isolated database: %v", err)
	}

	testConfig := baseConfig.Copy()
	testConfig.ConnConfig.Database = databaseName
	pool, err := pgxpool.NewWithConfig(ctx, testConfig)
	if err != nil {
		t.Fatalf("open isolated database: %v", err)
	}
	if err := database.Migrate(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("migrate isolated database: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cleanupAdminConfig := baseConfig.Copy()
		cleanupAdminConfig.ConnConfig.Database = "postgres"
		cleanupPool, cleanupErr := pgxpool.NewWithConfig(cleanupCtx, cleanupAdminConfig)
		if cleanupErr != nil {
			t.Errorf("open cleanup database: %v", cleanupErr)
			return
		}
		defer cleanupPool.Close()
		if _, cleanupErr := cleanupPool.Exec(cleanupCtx, `DROP DATABASE `+quotedName+` WITH (FORCE)`); cleanupErr != nil {
			t.Errorf("drop isolated database %q: %v", databaseName, cleanupErr)
		}
	})
	return pool
}

func TestIsolatedDatabaseNameIsSafe(t *testing.T) {
	name := pgx.Identifier{"contract_ledger_identity_123"}.Sanitize()
	if strings.ContainsAny(name, ";\n") {
		t.Fatalf("unsafe identifier = %q", name)
	}
}
