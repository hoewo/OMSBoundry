package identity

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"omsbizfoundry/contract-ledger/internal/audit"
)

type repositoryStub struct {
	hasUsers          bool
	credential        Credential
	credentialFound   bool
	bootstrapUser     User
	bootstrapHash     string
	bootstrapAudit    audit.Entry
	bootstrapCreated  bool
	createdUser       User
	createdHash       string
	createdAudit      audit.Entry
	failureCount      int
	resetCount        int
	sessionHash       []byte
	sessionUserID     string
	sessionGeneration int64
	sessionCreatedAt  time.Time
	sessionExpiresAt  time.Time
	resolved          Principal
	resolvedFound     bool
}

func (r *repositoryStub) HasUsers(context.Context) (bool, error) { return r.hasUsers, nil }
func (r *repositoryStub) Bootstrap(_ context.Context, user User, hash string, entry audit.Entry) (bool, error) {
	r.bootstrapUser, r.bootstrapHash, r.bootstrapAudit = user, hash, entry
	return r.bootstrapCreated, nil
}
func (r *repositoryStub) FindCredential(context.Context, string) (Credential, bool, error) {
	return r.credential, r.credentialFound, nil
}
func (r *repositoryStub) FindCredentialByID(context.Context, string) (Credential, bool, error) {
	return r.credential, r.credentialFound, nil
}
func (r *repositoryStub) RecordLoginFailure(context.Context, string, time.Time, int, time.Duration) error {
	r.failureCount++
	return nil
}
func (r *repositoryStub) ResetLoginFailures(context.Context, string, time.Time) error {
	r.resetCount++
	return nil
}
func (r *repositoryStub) CreateSession(_ context.Context, hash []byte, userID string, generation int64, createdAt, expiresAt time.Time) error {
	r.sessionHash = append([]byte(nil), hash...)
	r.sessionUserID = userID
	r.sessionGeneration = generation
	r.sessionCreatedAt = createdAt
	r.sessionExpiresAt = expiresAt
	return nil
}
func (r *repositoryStub) ResolveSession(context.Context, []byte, time.Time) (Principal, bool, error) {
	return r.resolved, r.resolvedFound, nil
}
func (r *repositoryStub) RevokeSession(context.Context, []byte, time.Time) error { return nil }
func (r *repositoryStub) ListUsers(context.Context) ([]User, error)              { return nil, nil }
func (r *repositoryStub) CreateUser(_ context.Context, user User, hash string, entry audit.Entry) error {
	r.createdUser, r.createdHash, r.createdAudit = user, hash, entry
	return nil
}
func (r *repositoryStub) ChangeRole(context.Context, string, Role, audit.Entry) error { return nil }
func (r *repositoryStub) ChangeStatus(context.Context, string, Status, string, audit.Entry) error {
	return nil
}
func (r *repositoryStub) ChangePassword(context.Context, string, int64, string, time.Time, audit.Entry) error {
	return nil
}
func (r *repositoryStub) ResetPassword(context.Context, string, string, time.Time, audit.Entry) error {
	return nil
}

func newTestService(t *testing.T, repository Repository) *Service {
	t.Helper()
	service, err := newService(repository, 2*time.Hour, bcrypt.MinCost)
	if err != nil {
		t.Fatalf("newService() error = %v", err)
	}
	service.now = func() time.Time { return time.Date(2026, 9, 19, 8, 0, 0, 0, time.UTC) }
	return service
}

func TestPrincipalPermissions(t *testing.T) {
	tests := []struct {
		role            Role
		businessWrite   bool
		businessCorrect bool
		businessExport  bool
		businessRestore bool
		manage          bool
	}{
		{role: RoleAdministrator, businessWrite: true, businessCorrect: true, businessExport: true, businessRestore: true, manage: true},
		{role: RoleEditor, businessWrite: true, businessCorrect: false, businessExport: false, businessRestore: false, manage: false},
		{role: RoleViewer, businessWrite: false, businessCorrect: false, businessExport: false, businessRestore: false, manage: false},
	}
	for _, test := range tests {
		principal := Principal{Role: test.role}
		if got := principal.Can(PermissionBusinessWrite); got != test.businessWrite {
			t.Fatalf("role %q business write = %v", test.role, got)
		}
		if got := principal.Can(PermissionBusinessCorrect); got != test.businessCorrect {
			t.Fatalf("role %q business correct = %v", test.role, got)
		}
		if got := principal.Can(PermissionBusinessExport); got != test.businessExport {
			t.Fatalf("role %q business export = %v", test.role, got)
		}
		if got := principal.Can(PermissionBusinessRestore); got != test.businessRestore {
			t.Fatalf("role %q business restore = %v", test.role, got)
		}
		if got := principal.Can(PermissionIdentityManage); got != test.manage {
			t.Fatalf("role %q identity manage = %v", test.role, got)
		}
	}
}

func TestEnsureBootstrapRequiresCredentialsOnlyForEmptyDatabase(t *testing.T) {
	emptyRepository := &repositoryStub{}
	service := newTestService(t, emptyRepository)
	_, err := service.EnsureBootstrap(context.Background(), BootstrapInput{})
	var validation *ValidationError
	if !errors.As(err, &validation) || validation.Field != "bootstrap" {
		t.Fatalf("empty bootstrap error = %v", err)
	}

	existingRepository := &repositoryStub{hasUsers: true}
	service = newTestService(t, existingRepository)
	created, err := service.EnsureBootstrap(context.Background(), BootstrapInput{})
	if err != nil || created {
		t.Fatalf("existing bootstrap = %v, error = %v", created, err)
	}
}

func TestBootstrapHashesPasswordAndAuditsSource(t *testing.T) {
	repository := &repositoryStub{bootstrapCreated: true}
	service := newTestService(t, repository)
	created, err := service.Bootstrap(context.Background(), BootstrapInput{
		Username: " Admin-1 ", DisplayName: " 管理员 ", Password: "bootstrap-password",
	})
	if err != nil || !created {
		t.Fatalf("Bootstrap() = %v, error = %v", created, err)
	}
	if repository.bootstrapUser.Username != "admin-1" || repository.bootstrapUser.Role != RoleAdministrator {
		t.Fatalf("bootstrap user = %#v", repository.bootstrapUser)
	}
	if repository.bootstrapHash == "bootstrap-password" || bcrypt.CompareHashAndPassword([]byte(repository.bootstrapHash), []byte("bootstrap-password")) != nil {
		t.Fatalf("bootstrap password hash is invalid")
	}
	if repository.bootstrapAudit.Action != ActionUserBootstrapped || repository.bootstrapAudit.Actor.Kind != "bootstrap_configuration" {
		t.Fatalf("bootstrap audit = %#v", repository.bootstrapAudit)
	}
}

func TestCreateUserNormalizesAndUsesAuthenticatedActor(t *testing.T) {
	repository := &repositoryStub{}
	service := newTestService(t, repository)
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", DisplayName: "管理员",
	}
	created, err := service.CreateUser(context.Background(), CreateUserInput{
		Username: " EDITOR.ONE ", DisplayName: " 编辑者一 ", Password: "editor-password", Role: "EDITOR", Actor: actor,
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if created.Username != "editor.one" || created.DisplayName != "编辑者一" || created.Role != RoleEditor || created.Status != StatusActive {
		t.Fatalf("created user = %#v", created)
	}
	if bcrypt.CompareHashAndPassword([]byte(repository.createdHash), []byte("editor-password")) != nil {
		t.Fatalf("created password hash is invalid")
	}
	if repository.createdAudit.Actor != actor || repository.createdAudit.Action != ActionUserCreated {
		t.Fatalf("created audit = %#v", repository.createdAudit)
	}
}

func TestLoginUsesGenericFailuresAndStoresOnlyTokenHash(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	repository := &repositoryStub{credentialFound: true, credential: Credential{
		User: User{
			ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Username: "admin", DisplayName: "管理员",
			Role: RoleAdministrator, Status: StatusActive, SessionGeneration: 3,
		},
		PasswordHash: string(passwordHash),
	}}
	service := newTestService(t, repository)

	_, err = service.Login(context.Background(), "admin", "wrong-password")
	var authentication *AuthenticationError
	if !errors.As(err, &authentication) || repository.failureCount != 1 {
		t.Fatalf("wrong login error = %v, failures = %d", err, repository.failureCount)
	}

	result, err := service.Login(context.Background(), " ADMIN ", "correct-password")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if result.Principal.UserID != repository.credential.ID || repository.resetCount != 1 {
		t.Fatalf("login result = %#v, resets = %d", result, repository.resetCount)
	}
	if result.Token == "" || len(repository.sessionHash) != 32 || string(repository.sessionHash) == result.Token {
		t.Fatalf("token or stored hash invalid: token=%q hash=%x", result.Token, repository.sessionHash)
	}
	if repository.sessionGeneration != 3 || !repository.sessionExpiresAt.Equal(repository.sessionCreatedAt.Add(2*time.Hour)) {
		t.Fatalf("session metadata generation=%d created=%s expires=%s", repository.sessionGeneration, repository.sessionCreatedAt, repository.sessionExpiresAt)
	}

	repository.credentialFound = false
	_, err = service.Login(context.Background(), "missing", "wrong-password")
	if !errors.As(err, &authentication) {
		t.Fatalf("missing login error = %v", err)
	}
}

func TestPasswordValidationUsesBCryptLimit(t *testing.T) {
	for _, value := range []string{"short", string(make([]byte, 73))} {
		if err := validatePassword(value); err == nil {
			t.Fatalf("validatePassword(%d bytes) error = nil", len(value))
		}
	}
}
