package identity

import (
	"context"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	bootstrapTenant = "11111111-1111-4111-8111-111111111111"
	secondTenant    = "22222222-2222-4222-8222-222222222222"
	testSubject     = "33333333-3333-4333-8333-333333333333"
	testActor       = "44444444-4444-4444-8444-444444444444"
)

type multiTenantRepository struct {
	credential       Credential
	resolved         Principal
	tenantExists     bool
	createdSessionID string
	grantTenant      string
	listedTenant     string
	tokenTenant      string
	createdToken     ActivationToken
	audits           []AuditEntry
}

func (r *multiTenantRepository) HasSubjects(context.Context) (bool, error) { return true, nil }
func (r *multiTenantRepository) Bootstrap(context.Context, string, string, Subject, string, AuditEntry) (bool, error) {
	return false, nil
}
func (r *multiTenantRepository) TenantExists(context.Context, string) (bool, error) {
	return r.tenantExists, nil
}
func (r *multiTenantRepository) FindSubject(_ context.Context, tenantID, subjectID string) (Subject, bool, error) {
	if r.credential.TenantID != tenantID || r.credential.ID != subjectID {
		return Subject{}, false, nil
	}
	return r.credential.Subject, true, nil
}
func (r *multiTenantRepository) FindCredential(context.Context, string) (Credential, bool, error) {
	return r.credential, r.credential.ID != "", nil
}
func (r *multiTenantRepository) RecordLoginFailure(context.Context, string, time.Time, int, time.Duration) error {
	return nil
}
func (r *multiTenantRepository) ResetLoginFailures(context.Context, string, time.Time) error {
	return nil
}
func (r *multiTenantRepository) CreateSession(_ context.Context, _ []byte, subjectID string, _ int64, _, _ time.Time) error {
	r.createdSessionID = subjectID
	return nil
}
func (r *multiTenantRepository) ResolveSession(context.Context, []byte, time.Time) (Principal, bool, error) {
	return r.resolved, r.resolved.SubjectID != "", nil
}
func (r *multiTenantRepository) RevokeSession(context.Context, []byte, time.Time) error { return nil }
func (r *multiTenantRepository) Invite(context.Context, Subject, ActivationToken, AuditEntry) error {
	return nil
}
func (r *multiTenantRepository) Activate(context.Context, []byte, TokenKind, string, time.Time, AuditEntry) (Subject, error) {
	return Subject{}, nil
}
func (r *multiTenantRepository) CreateActivationToken(_ context.Context, tenantID string, token ActivationToken, entry AuditEntry) error {
	r.tokenTenant = tenantID
	r.createdToken = token
	r.audits = append(r.audits, entry)
	return nil
}
func (r *multiTenantRepository) Grant(_ context.Context, tenantID string, _ Grant, entry AuditEntry) error {
	r.grantTenant = tenantID
	r.audits = append(r.audits, entry)
	return nil
}
func (r *multiTenantRepository) RevokeGrant(context.Context, string, Grant, AuditEntry) error {
	return nil
}
func (r *multiTenantRepository) ListGrants(_ context.Context, tenantID, subjectID, audience string) ([]Grant, error) {
	r.listedTenant = tenantID
	return []Grant{{SubjectID: subjectID, Audience: audience, Action: audience + ".view", Scope: Scope{Kind: "tenant"}}}, nil
}
func (r *multiTenantRepository) RecordAudit(_ context.Context, entry AuditEntry) error {
	r.audits = append(r.audits, entry)
	return nil
}
func (r *multiTenantRepository) ListSubjects(context.Context, string) ([]Subject, error) {
	return nil, nil
}
func (r *multiTenantRepository) ChangeStatus(context.Context, string, string, Status, AuditEntry) error {
	return nil
}

type captureSigner struct{ value any }

func (s *captureSigner) Sign(value any) (string, error) {
	s.value = value
	return "signed", nil
}

type allowCatalog struct{}

func (allowCatalog) Contains(string, string) bool               { return true }
func (allowCatalog) AllowsResource(string, string, string) bool { return true }

func newMultiTenantService(t *testing.T, repository Repository, signer Signer) *Service {
	t.Helper()
	service, err := NewService(repository, signer, allowCatalog{}, bootstrapTenant, "引导租户", "https://oms.example.test/auth", 12*time.Hour, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	service.newID = func() (string, error) { return "55555555-5555-4555-8555-555555555555", nil }
	service.now = func() time.Time { return time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC) }
	return service
}

func TestLoginAndSessionResolveTenantFromAccount(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct horse battery staple"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	repository := &multiTenantRepository{credential: Credential{Subject: Subject{
		ID: testSubject, TenantID: secondTenant, Username: "operator-b", DisplayName: "第二租户操作员",
		Status: StatusActive, SessionGeneration: 1,
	}, PasswordHash: string(hash)}}
	service := newMultiTenantService(t, repository, &captureSigner{})
	result, err := service.Login(context.Background(), "operator-b", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if result.Principal.TenantID != secondTenant || repository.createdSessionID != testSubject {
		t.Fatalf("login result = %#v, session subject = %q", result.Principal, repository.createdSessionID)
	}
	repository.resolved = result.Principal
	resolved, found, err := service.ResolveSession(context.Background(), result.Token)
	if err != nil || !found || resolved.TenantID != secondTenant {
		t.Fatalf("resolved = %#v, %v, %v", resolved, found, err)
	}
}

func TestLoginIdentifiersNormalizePhoneEmailAndLegacyUsername(t *testing.T) {
	tests := map[string]string{
		"+86 138-0013-8000":     "13800138000",
		"User.Name@Example.COM": "user.name@example.com",
		"Admin-B":               "admin-b",
	}
	for input, want := range tests {
		got, err := normalizeUsername(input)
		if err != nil || got != want {
			t.Fatalf("normalizeUsername(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	for _, input := range []string{"1380013800", "invalid@localhost", "not a login"} {
		if _, err := normalizeUsername(input); err == nil {
			t.Fatalf("normalizeUsername(%q) succeeded", input)
		}
	}
}

func TestReissueInvitationUsesAuthenticatedTenantAndPendingSubject(t *testing.T) {
	repository := &multiTenantRepository{credential: Credential{Subject: Subject{
		ID: testSubject, TenantID: secondTenant, Username: "member@example.com", DisplayName: "待激活成员", Status: StatusPending,
	}}}
	service := newMultiTenantService(t, repository, &captureSigner{})
	actor := Principal{SubjectID: testActor, TenantID: secondTenant, DisplayName: "第二租户管理员", Administrator: true}
	result, err := service.ReissueInvitation(context.Background(), actor, testSubject)
	if err != nil {
		t.Fatal(err)
	}
	if result.Subject.ID != testSubject || result.Token == "" || repository.tokenTenant != secondTenant || repository.createdToken.Kind != TokenInvitation || repository.createdToken.SubjectID != testSubject {
		t.Fatalf("result = %#v, tenant = %q, token = %#v", result, repository.tokenTenant, repository.createdToken)
	}
	if len(repository.audits) != 1 || repository.audits[0].Action != "iam_invitation_reissued" || repository.audits[0].TenantID != secondTenant {
		t.Fatalf("audits = %#v", repository.audits)
	}
	actor.Administrator = false
	if _, err := service.ReissueInvitation(context.Background(), actor, testSubject); err == nil {
		t.Fatal("non-administrator reissued an invitation")
	}
}

func TestAdminAndCredentialOperationsUseAuthenticatedTenant(t *testing.T) {
	repository := &multiTenantRepository{}
	signer := &captureSigner{}
	service := newMultiTenantService(t, repository, signer)
	actor := Principal{SubjectID: testActor, TenantID: secondTenant, DisplayName: "第二租户管理员", Administrator: true, AuthTime: service.now()}
	if err := service.Grant(context.Background(), actor, testSubject, "contract-ledger", "contract-ledger.view", Scope{}); err != nil {
		t.Fatal(err)
	}
	if repository.grantTenant != secondTenant || len(repository.audits) != 1 || repository.audits[0].TenantID != secondTenant {
		t.Fatalf("grant tenant = %q, audits = %#v", repository.grantTenant, repository.audits)
	}
	credential, err := service.IssueCredential(context.Background(), Principal{SubjectID: testSubject, TenantID: secondTenant, DisplayName: "操作员", AuthTime: service.now()}, "contract-ledger", Scope{})
	if err != nil {
		t.Fatal(err)
	}
	if repository.listedTenant != secondTenant || credential.Claims.Tenant != secondTenant {
		t.Fatalf("listed tenant = %q, claims = %#v", repository.listedTenant, credential.Claims)
	}
	claims, ok := signer.value.(CapabilityClaims)
	if !ok || claims.Tenant != secondTenant {
		t.Fatalf("signed claims = %#v", signer.value)
	}
}

func TestWorkloadCredentialRequiresKnownTenant(t *testing.T) {
	repository := &multiTenantRepository{tenantExists: false}
	service := newMultiTenantService(t, repository, &captureSigner{})
	_, err := service.IssueWorkloadCredential(context.Background(), secondTenant, ServiceIdentity{
		ID: "project-workspace", SubjectID: testSubject, DisplayName: "Project Workspace",
		KeyID: testActor, AssertionID: "55555555-5555-4555-8555-555555555555", AuthTime: service.now(),
	}, "people-groups", "people-groups.view", Scope{Kind: "resources", Resources: []ResourceRef{{Type: "person", ID: testSubject}}})
	if _, ok := err.(*AuthenticationError); !ok {
		t.Fatalf("error = %v, want AuthenticationError", err)
	}
}

func TestSubjectReferenceIsExactAndTenantScoped(t *testing.T) {
	repository := &multiTenantRepository{credential: Credential{Subject: Subject{
		ID: testSubject, TenantID: secondTenant, Username: "operator-b", DisplayName: "第二租户操作员",
		Status: StatusActive, Revision: 3, SessionGeneration: 1,
	}}}
	service := newMultiTenantService(t, repository, &captureSigner{})
	value, err := service.SubjectReference(context.Background(), secondTenant, testSubject)
	if err != nil || value.SourceApplication != "identity-access" || value.TenantID != secondTenant || value.ID != testSubject || value.Revision != 3 {
		t.Fatalf("reference=%#v err=%v", value, err)
	}
	if _, err := service.SubjectReference(context.Background(), bootstrapTenant, testSubject); err == nil {
		t.Fatal("cross-tenant subject reference was visible")
	}
}
