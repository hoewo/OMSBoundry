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
