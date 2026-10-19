//go:build integration

package identity_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"sync"
	"testing"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/database"
	"omsbizfoundry/money-flow-ledger/internal/identity"
	"omsbizfoundry/money-flow-ledger/internal/testdatabase"
)

func TestPostgresBootstrapLoginLockAndLogout(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	repository := identity.NewPostgresRepository(environment.Pool)
	_, err := repository.Bootstrap(ctx, identity.User{
		ID: "11111111-1111-4111-8111-111111111111", Username: "rollback-admin", DisplayName: "回滚管理员",
		Role: identity.RoleAdministrator, Status: identity.StatusActive, SessionGeneration: 1,
		PasswordChangedAt: now, CreatedAt: now, UpdatedAt: now,
	}, "$2a$10$12345678901234567890123456789012345678901234567890123", audit.Entry{
		ID: "22222222-2222-4222-8222-222222222222", ObjectType: audit.ObjectLocalUser,
		ObjectID: "11111111-1111-4111-8111-111111111111", Action: "",
		Actor:      audit.ActorRef{Kind: "bootstrap_configuration", ID: "11111111-1111-4111-8111-111111111111", DisplayName: "回滚管理员"},
		OccurredAt: now, Summary: "应当整体回滚", Details: map[string]any{},
	})
	if err == nil {
		t.Fatal("Bootstrap() with invalid audit error = nil")
	}
	var usersAfterFailure, auditsAfterFailure int
	if err := environment.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM local_users), (SELECT count(*) FROM audit_entries)`).Scan(&usersAfterFailure, &auditsAfterFailure); err != nil {
		t.Fatalf("count failed bootstrap facts: %v", err)
	}
	if usersAfterFailure != 0 || auditsAfterFailure != 0 {
		t.Fatalf("failed bootstrap users=%d audits=%d", usersAfterFailure, auditsAfterFailure)
	}

	services := make([]*identity.Service, 2)
	for index := range services {
		services[index], err = identity.NewService(identity.NewPostgresRepository(environment.Pool), time.Hour)
		if err != nil {
			t.Fatalf("identity.NewService() error = %v", err)
		}
	}
	results := make([]bool, len(services))
	errorsByIndex := make([]error, len(services))
	var waitGroup sync.WaitGroup
	for index, service := range services {
		waitGroup.Add(1)
		go func(index int, service *identity.Service) {
			defer waitGroup.Done()
			results[index], errorsByIndex[index] = service.EnsureBootstrap(ctx, identity.BootstrapInput{
				Username: " Money.Admin ", DisplayName: "资金管理员", Password: "integration-password-2026",
			})
		}(index, service)
	}
	waitGroup.Wait()
	if errorsByIndex[0] != nil || errorsByIndex[1] != nil || results[0] == results[1] {
		t.Fatalf("concurrent bootstrap results=%v errors=%v", results, errorsByIndex)
	}

	var userID, username, displayName, role, passwordHash string
	var userCount, auditCount int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT id::text, username, display_name, role, password_hash,
			(SELECT count(*) FROM local_users),
			(SELECT count(*) FROM audit_entries WHERE action = 'local_user_bootstrapped')
		FROM local_users
	`).Scan(&userID, &username, &displayName, &role, &passwordHash, &userCount, &auditCount); err != nil {
		t.Fatalf("read bootstrap facts: %v", err)
	}
	if userCount != 1 || auditCount != 1 || username != "money.admin" || displayName != "资金管理员" ||
		role != string(identity.RoleAdministrator) || passwordHash == "integration-password-2026" {
		t.Fatalf("bootstrap facts id=%s username=%s display=%s role=%s users=%d audits=%d", userID, username, displayName, role, userCount, auditCount)
	}

	loginService, err := identity.NewService(identity.NewPostgresRepository(environment.Pool), time.Hour)
	if err != nil {
		t.Fatalf("identity.NewService() error = %v", err)
	}
	created, err := loginService.EnsureBootstrap(ctx, identity.BootstrapInput{Username: "replacement", Password: "replacement-password"})
	if err != nil || created {
		t.Fatalf("repeated EnsureBootstrap() = %v, %v", created, err)
	}
	result, err := loginService.Login(ctx, "MONEY.ADMIN", "integration-password-2026")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if result.Principal.UserID != userID || result.Token == "" {
		t.Fatalf("Login() = %#v", result)
	}
	tokenHash := sha256.Sum256([]byte(result.Token))
	var storedHashMatches bool
	var storedHashLength int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT token_hash = $1, octet_length(token_hash)
		FROM local_sessions
		WHERE user_id = $2
	`, tokenHash[:], userID).Scan(&storedHashMatches, &storedHashLength); err != nil {
		t.Fatalf("read stored session hash: %v", err)
	}
	if !storedHashMatches || storedHashLength != 32 {
		t.Fatalf("stored session hash matches=%v length=%d", storedHashMatches, storedHashLength)
	}
	principal, found, err := loginService.ResolveSession(ctx, result.Token)
	if err != nil || !found || principal.UserID != userID || principal.Actor().ID != userID {
		t.Fatalf("ResolveSession() = %#v, %v, %v", principal, found, err)
	}
	if err := loginService.Logout(ctx, result.Token); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if _, found, err := loginService.ResolveSession(ctx, result.Token); err != nil || found {
		t.Fatalf("resolved revoked session found=%v err=%v", found, err)
	}

	for attempt := 1; attempt <= 5; attempt++ {
		_, err := loginService.Login(ctx, "money.admin", "wrong-password")
		var authentication *identity.AuthenticationError
		if !errors.As(err, &authentication) {
			t.Fatalf("failed login %d error = %v", attempt, err)
		}
	}
	var attempts int
	var lockedUntil time.Time
	if err := environment.Pool.QueryRow(ctx, `SELECT failed_login_attempts, locked_until FROM local_users WHERE id = $1`, userID).Scan(&attempts, &lockedUntil); err != nil {
		t.Fatalf("read login lock: %v", err)
	}
	if attempts != 5 || !lockedUntil.After(time.Now().UTC()) {
		t.Fatalf("login lock attempts=%d until=%s", attempts, lockedUntil)
	}
	_, err = loginService.Login(ctx, "money.admin", "integration-password-2026")
	var authentication *identity.AuthenticationError
	if !errors.As(err, &authentication) {
		t.Fatalf("locked correct password error = %v", err)
	}
}
