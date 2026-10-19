package identity

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

type memorySession struct {
	principal Principal
	expiresAt time.Time
	revoked   bool
}

type memoryRepository struct {
	credential   Credential
	hasUser      bool
	audits       []audit.Entry
	sessions     map[string]memorySession
	hasUsersErr  error
	bootstrapErr error
	findErr      error
}

func (r *memoryRepository) HasUsers(context.Context) (bool, error) {
	return r.hasUser, r.hasUsersErr
}

func (r *memoryRepository) Bootstrap(_ context.Context, user User, passwordHash string, entry audit.Entry) (bool, error) {
	if r.bootstrapErr != nil {
		return false, r.bootstrapErr
	}
	if r.hasUser {
		return false, nil
	}
	r.hasUser = true
	r.credential = Credential{User: user, PasswordHash: passwordHash}
	r.audits = append(r.audits, entry)
	return true, nil
}

func (r *memoryRepository) FindCredential(_ context.Context, username string) (Credential, bool, error) {
	if r.findErr != nil {
		return Credential{}, false, r.findErr
	}
	if !r.hasUser || r.credential.Username != username {
		return Credential{}, false, nil
	}
	return r.credential, true, nil
}

func (r *memoryRepository) RecordLoginFailure(_ context.Context, _ string, now time.Time, maximumAttempts int, lockDuration time.Duration) error {
	r.credential.FailedLoginAttempts++
	if r.credential.FailedLoginAttempts >= maximumAttempts {
		r.credential.LockedUntil = now.Add(lockDuration)
	}
	return nil
}

func (r *memoryRepository) ResetLoginFailures(_ context.Context, _ string, now time.Time) error {
	r.credential.FailedLoginAttempts = 0
	r.credential.LockedUntil = time.Time{}
	r.credential.UpdatedAt = now
	return nil
}

func (r *memoryRepository) CreateSession(_ context.Context, tokenHash []byte, userID string, generation int64, _ time.Time, expiresAt time.Time) error {
	if userID != r.credential.ID || generation != r.credential.SessionGeneration || r.credential.Status != StatusActive {
		return &AuthenticationError{}
	}
	if r.sessions == nil {
		r.sessions = make(map[string]memorySession)
	}
	r.sessions[string(tokenHash)] = memorySession{principal: principalFromUser(r.credential.User), expiresAt: expiresAt}
	return nil
}

func (r *memoryRepository) ResolveSession(_ context.Context, tokenHash []byte, now time.Time) (Principal, bool, error) {
	session, found := r.sessions[string(tokenHash)]
	if !found || session.revoked || !session.expiresAt.After(now) || r.credential.Status != StatusActive ||
		session.principal.SessionGeneration != r.credential.SessionGeneration {
		return Principal{}, false, nil
	}
	return session.principal, true, nil
}

func (r *memoryRepository) RevokeSession(_ context.Context, tokenHash []byte, _ time.Time) error {
	session, found := r.sessions[string(tokenHash)]
	if found {
		session.revoked = true
		r.sessions[string(tokenHash)] = session
	}
	return nil
}

func newTestService(t *testing.T, repository *memoryRepository) *Service {
	t.Helper()
	service, err := newService(repository, 2*time.Hour, bcrypt.MinCost)
	if err != nil {
		t.Fatalf("newService() error = %v", err)
	}
	service.now = func() time.Time { return time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC) }
	ids := []string{
		"11111111-1111-4111-8111-111111111111",
		"22222222-2222-4222-8222-222222222222",
	}
	service.newID = func() (string, error) {
		if len(ids) == 0 {
			return "", errors.New("no test IDs")
		}
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}
	return service
}

func TestEnsureBootstrapRequiresExplicitCredentials(t *testing.T) {
	repository := &memoryRepository{}
	service := newTestService(t, repository)
	_, err := service.EnsureBootstrap(context.Background(), BootstrapInput{})
	var validation *ValidationError
	if !errors.As(err, &validation) || validation.Field != "bootstrap" {
		t.Fatalf("EnsureBootstrap() error = %v", err)
	}
	if repository.hasUser {
		t.Fatal("missing configuration created a user")
	}
}

func TestEnsureBootstrapNormalizesHashesAndDoesNotOverwrite(t *testing.T) {
	repository := &memoryRepository{}
	service := newTestService(t, repository)
	created, err := service.EnsureBootstrap(context.Background(), BootstrapInput{
		Username:    " Money.Admin ",
		DisplayName: " 资金管理员 ",
		Password:    "correct-password-2026",
	})
	if err != nil {
		t.Fatalf("EnsureBootstrap() error = %v", err)
	}
	if !created || repository.credential.Username != "money.admin" || repository.credential.DisplayName != "资金管理员" ||
		repository.credential.Role != RoleAdministrator || repository.credential.Status != StatusActive {
		t.Fatalf("bootstrap credential = %#v", repository.credential)
	}
	if repository.credential.PasswordHash == "correct-password-2026" ||
		bcrypt.CompareHashAndPassword([]byte(repository.credential.PasswordHash), []byte("correct-password-2026")) != nil {
		t.Fatal("bootstrap password was not safely hashed")
	}
	if len(repository.audits) != 1 || repository.audits[0].Action != ActionUserBootstrapped ||
		repository.audits[0].ObjectID != repository.credential.ID {
		t.Fatalf("bootstrap audits = %#v", repository.audits)
	}

	created, err = service.EnsureBootstrap(context.Background(), BootstrapInput{
		Username: "replacement", Password: "replacement-password",
	})
	if err != nil || created {
		t.Fatalf("repeated EnsureBootstrap() = %v, %v", created, err)
	}
	if repository.credential.Username != "money.admin" || len(repository.audits) != 1 {
		t.Fatal("repeated bootstrap changed existing identity")
	}
}

func TestLoginResolveAndLogout(t *testing.T) {
	repository := &memoryRepository{}
	service := newTestService(t, repository)
	if _, err := service.EnsureBootstrap(context.Background(), BootstrapInput{
		Username: "admin", Password: "correct-password-2026",
	}); err != nil {
		t.Fatalf("EnsureBootstrap() error = %v", err)
	}
	result, err := service.Login(context.Background(), " ADMIN ", "correct-password-2026")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if result.Token == "" || result.Principal.UserID != repository.credential.ID || len(repository.sessions) != 1 {
		t.Fatalf("Login() = %#v sessions=%d", result, len(repository.sessions))
	}
	for storedHash := range repository.sessions {
		if storedHash == result.Token || strings.Contains(storedHash, result.Token) {
			t.Fatal("repository received raw session token")
		}
	}
	principal, found, err := service.ResolveSession(context.Background(), result.Token)
	if err != nil || !found || principal.DisplayName != "admin" || principal.Actor().ID != principal.UserID {
		t.Fatalf("ResolveSession() = %#v, %v, %v", principal, found, err)
	}
	if err := service.Logout(context.Background(), result.Token); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if _, found, err := service.ResolveSession(context.Background(), result.Token); err != nil || found {
		t.Fatalf("resolved revoked session: found=%v err=%v", found, err)
	}
}

func TestLoginFailureAndLockUseGenericAuthenticationError(t *testing.T) {
	repository := &memoryRepository{}
	service := newTestService(t, repository)
	if _, err := service.EnsureBootstrap(context.Background(), BootstrapInput{
		Username: "admin", Password: "correct-password-2026",
	}); err != nil {
		t.Fatalf("EnsureBootstrap() error = %v", err)
	}
	for attempt := 1; attempt <= 5; attempt++ {
		_, err := service.Login(context.Background(), "admin", "wrong-password")
		var authentication *AuthenticationError
		if !errors.As(err, &authentication) {
			t.Fatalf("attempt %d error = %v", attempt, err)
		}
	}
	if !repository.credential.LockedUntil.After(service.now()) {
		t.Fatal("fifth failure did not lock user")
	}
	_, lockedErr := service.Login(context.Background(), "admin", "correct-password-2026")
	_, unknownErr := service.Login(context.Background(), "unknown", "correct-password-2026")
	if lockedErr == nil || unknownErr == nil || lockedErr.Error() != unknownErr.Error() {
		t.Fatalf("generic errors locked=%v unknown=%v", lockedErr, unknownErr)
	}
}

func TestDisabledExpiredAndGenerationMismatchedSessionsAreRejected(t *testing.T) {
	newLoggedIn := func(t *testing.T) (*Service, *memoryRepository, string) {
		t.Helper()
		repository := &memoryRepository{}
		service := newTestService(t, repository)
		if _, err := service.EnsureBootstrap(context.Background(), BootstrapInput{
			Username: "admin", Password: "correct-password-2026",
		}); err != nil {
			t.Fatalf("EnsureBootstrap() error = %v", err)
		}
		result, err := service.Login(context.Background(), "admin", "correct-password-2026")
		if err != nil {
			t.Fatalf("Login() error = %v", err)
		}
		return service, repository, result.Token
	}

	t.Run("disabled", func(t *testing.T) {
		service, repository, token := newLoggedIn(t)
		repository.credential.Status = StatusDisabled
		if _, found, err := service.ResolveSession(context.Background(), token); err != nil || found {
			t.Fatalf("disabled ResolveSession() found=%v err=%v", found, err)
		}
		_, err := service.Login(context.Background(), "admin", "correct-password-2026")
		var authentication *AuthenticationError
		if !errors.As(err, &authentication) {
			t.Fatalf("disabled Login() error = %v", err)
		}
	})

	t.Run("expired", func(t *testing.T) {
		service, _, token := newLoggedIn(t)
		service.now = func() time.Time { return time.Date(2026, 9, 19, 15, 0, 1, 0, time.UTC) }
		if _, found, err := service.ResolveSession(context.Background(), token); err != nil || found {
			t.Fatalf("expired ResolveSession() found=%v err=%v", found, err)
		}
	})

	t.Run("generation mismatch", func(t *testing.T) {
		service, repository, token := newLoggedIn(t)
		repository.credential.SessionGeneration++
		if _, found, err := service.ResolveSession(context.Background(), token); err != nil || found {
			t.Fatalf("generation mismatch ResolveSession() found=%v err=%v", found, err)
		}
	})
}

func TestValidationAndRepositoryErrors(t *testing.T) {
	tests := []struct {
		name  string
		input BootstrapInput
		field string
	}{
		{name: "username", input: BootstrapInput{Username: "x", Password: "correct-password"}, field: "username"},
		{name: "display", input: BootstrapInput{Username: "admin", DisplayName: strings.Repeat("名", 101), Password: "correct-password"}, field: "display_name"},
		{name: "short password", input: BootstrapInput{Username: "admin", Password: "short"}, field: "password"},
		{name: "long password", input: BootstrapInput{Username: "admin", Password: strings.Repeat("x", 73)}, field: "password"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := newTestService(t, &memoryRepository{})
			_, err := service.EnsureBootstrap(context.Background(), test.input)
			var validation *ValidationError
			if !errors.As(err, &validation) || validation.Field != test.field {
				t.Fatalf("error = %v", err)
			}
		})
	}

	service := newTestService(t, &memoryRepository{hasUsersErr: errors.New("database unavailable")})
	if _, err := service.EnsureBootstrap(context.Background(), BootstrapInput{}); err == nil || !strings.Contains(err.Error(), "database unavailable") {
		t.Fatalf("repository error = %v", err)
	}
}

func TestResolveAndLogoutIgnoreMalformedToken(t *testing.T) {
	service := newTestService(t, &memoryRepository{})
	if _, found, err := service.ResolveSession(context.Background(), "not-a-token"); err != nil || found {
		t.Fatalf("ResolveSession() found=%v err=%v", found, err)
	}
	if err := service.Logout(context.Background(), "not-a-token"); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
}

func TestReferenceTemplatePrincipalDoesNotMasqueradeAsLocalUser(t *testing.T) {
	actor := (Principal{Provider: "reference-template", UserID: "payable-suggestion-template", DisplayName: "Template connector"}).Actor()
	if actor.Kind != "template_subject" || actor.ID != "payable-suggestion-template" {
		t.Fatalf("actor = %#v", actor)
	}
	if _, err := audit.NormalizeActor(actor); err != nil {
		t.Fatalf("template actor should be auditable without a local user UUID: %v", err)
	}
}
