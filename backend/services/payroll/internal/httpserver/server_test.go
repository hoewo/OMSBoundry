package httpserver

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"omsbizfoundry/payroll/internal/auth"
	domain "omsbizfoundry/payroll/internal/payroll"
	"testing"
)

const (
	testTenant  = "11111111-1111-4111-8111-111111111111"
	testSubject = "22222222-2222-4222-8222-222222222222"
	testRun     = "33333333-3333-4333-8333-333333333333"
)

type fixedAuth struct {
	principal    auth.Principal
	err          error
	sawUntrusted bool
}

func (a *fixedAuth) ProviderName() string { return "test" }
func (a *fixedAuth) Authenticate(r *http.Request) (auth.Principal, error) {
	a.sawUntrusted = r.Header.Get("X-Tenant-ID") != ""
	return a.principal, a.err
}

type fakeCases struct {
	value domain.Run
	err   error
}

type profileCases struct {
	*fakeCases
	payment domain.PaymentProfile
}

func (f *profileCases) UpsertCompensation(context.Context, domain.Actor, domain.UpsertCompensationProfileInput) (domain.CompensationProfile, error) {
	return domain.CompensationProfile{}, f.err
}
func (f *profileCases) GetCompensation(context.Context, string, string) (domain.CompensationProfile, error) {
	return domain.CompensationProfile{}, f.err
}
func (f *profileCases) ListCompensations(context.Context, string) ([]domain.CompensationProfile, error) {
	return []domain.CompensationProfile{}, f.err
}
func (f *profileCases) UpsertPayment(context.Context, domain.Actor, domain.UpsertPaymentProfileInput) (domain.PaymentProfile, error) {
	return f.payment, f.err
}
func (f *profileCases) GetPayment(context.Context, string, string) (domain.PaymentProfile, error) {
	return f.payment, f.err
}
func (f *profileCases) ListPayments(context.Context, string) ([]domain.PaymentProfile, error) {
	return []domain.PaymentProfile{f.payment}, f.err
}
func (f *profileCases) ListPaymentHistory(context.Context, string, string) ([]domain.PaymentProfile, error) {
	return []domain.PaymentProfile{f.payment}, f.err
}
func (f *profileCases) ReadSensitivePayment(context.Context, domain.Actor, string, string) (domain.SensitivePaymentProfile, error) {
	return domain.SensitivePaymentProfile{PaymentProfile: f.payment, AccountNumber: "6222021234567890"}, f.err
}
func (f *profileCases) CreateCustomScheme(context.Context, domain.Actor, domain.CreateCustomSchemeInput) (domain.ContributionScheme, error) {
	return domain.ContributionScheme{}, f.err
}
func (f *profileCases) ReviseCustomScheme(context.Context, domain.Actor, string, domain.ReviseCustomSchemeInput) (domain.ContributionScheme, error) {
	return domain.ContributionScheme{}, f.err
}
func (f *profileCases) ListSchemes(context.Context, string) ([]domain.ContributionScheme, error) {
	return []domain.ContributionScheme{}, f.err
}

func (f *fakeCases) Ping(context.Context) error { return f.err }
func (f *fakeCases) Create(context.Context, domain.Actor, domain.CreateInput) (domain.Run, error) {
	return f.value, f.err
}
func (f *fakeCases) Update(context.Context, domain.Actor, string, domain.UpdateInput) (domain.Run, error) {
	return f.value, f.err
}
func (f *fakeCases) Get(context.Context, string, string) (domain.Run, error) { return f.value, f.err }
func (f *fakeCases) List(context.Context, string) (domain.Page, error) {
	return domain.Page{Items: []domain.Run{f.value}}, f.err
}
func TestProtectedRoutesFailClosedAndStripHeaders(t *testing.T) {
	cases := &fakeCases{}
	a := &fixedAuth{err: auth.ErrUnauthenticated}
	server, _ := New(cases, a)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/payroll-runs", nil)
	req.Header.Set("X-Tenant-ID", "attacker")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != 401 || a.sawUntrusted {
		t.Fatalf("expected sanitized 401, got %d untrusted=%v", res.Code, a.sawUntrusted)
	}
	a.err = nil
	a.principal = auth.Principal{SubjectID: testSubject, TenantID: testTenant, ScopeKind: "tenant", AllowedActions: map[string]bool{auth.ActionRunManage: true}}
	res = httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/payroll-runs", nil))
	if res.Code != 403 {
		t.Fatalf("expected capability 403, got %d", res.Code)
	}
}
func TestCreateRequiresTenantScope(t *testing.T) {
	cases := &fakeCases{}
	a := &fixedAuth{principal: auth.Principal{SubjectID: testSubject, TenantID: testTenant, ScopeKind: "resources", AllowedActions: map[string]bool{auth.ActionRunManage: true}, Resources: map[string]map[string]bool{"payroll_run": {testRun: true}}}}
	server, _ := New(cases, a)
	body := bytes.NewBufferString(`{"command_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","period":"2026-10","label":"October"}`)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/payroll-runs", body))
	if res.Code != 403 {
		t.Fatalf("expected scope 403, got %d", res.Code)
	}
}

func TestPaymentAccountIsMaskedAndSensitiveReadRequiresExactScope(t *testing.T) {
	const personID = "44444444-4444-4444-8444-444444444444"
	const profileID = "55555555-5555-4555-8555-555555555555"
	cases := &profileCases{fakeCases: &fakeCases{}, payment: domain.PaymentProfile{ID: profileID, TenantID: testTenant, PersonID: personID, MaskedAccountNumber: "**** **** **** 7890", Revision: 1}}
	a := &fixedAuth{principal: auth.Principal{SubjectID: testSubject, TenantID: testTenant, ScopeKind: "tenant", AllowedActions: map[string]bool{auth.ActionTenantRead: true}, Resources: map[string]map[string]bool{}}}
	server, _ := New(cases, a)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/payment-profiles/"+personID, nil))
	if res.Code != 200 || !bytes.Contains(res.Body.Bytes(), []byte("7890")) || bytes.Contains(res.Body.Bytes(), []byte("6222021234567890")) {
		t.Fatalf("masked response status=%d body=%s", res.Code, res.Body.String())
	}
	a.principal.AllowedActions = map[string]bool{auth.ActionSensitiveAccountRead: true}
	res = httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/payment-profiles/"+personID+"/sensitive-account", bytes.NewBufferString(`{"purpose":"发放前核对"}`)))
	if res.Code != 403 {
		t.Fatalf("tenant-wide sensitive read must be rejected, got %d %s", res.Code, res.Body.String())
	}
	a.principal.ScopeKind = "resources"
	a.principal.Resources = map[string]map[string]bool{"payment_profile": {profileID: true}}
	res = httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/payment-profiles/"+personID+"/sensitive-account", bytes.NewBufferString(`{"purpose":"发放前核对"}`)))
	if res.Code != 200 || !bytes.Contains(res.Body.Bytes(), []byte("6222021234567890")) {
		t.Fatalf("exact sensitive response status=%d body=%s", res.Code, res.Body.String())
	}
	a.principal.AllowedActions = map[string]bool{}
	res = httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/payment-profiles/"+personID+"/sensitive-account", bytes.NewBufferString(`{"purpose":"发放前核对"}`)))
	if res.Code != 403 {
		t.Fatalf("revoked capability must fail, got %d", res.Code)
	}
}
