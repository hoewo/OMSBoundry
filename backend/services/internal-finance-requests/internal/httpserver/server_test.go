package httpserver

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"omsbizfoundry/internal-finance-requests/internal/auth"
	"omsbizfoundry/internal-finance-requests/internal/finance"
)

const (
	testTenant  = "11111111-1111-4111-8111-111111111111"
	testSubject = "22222222-2222-4222-8222-222222222222"
	testRequest = "33333333-3333-4333-8333-333333333333"
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
	value finance.Request
	err   error
}

func (f *fakeCases) Ping(context.Context) error { return f.err }
func (f *fakeCases) Create(context.Context, finance.Actor, finance.CreateInput) (finance.Request, error) {
	return f.value, f.err
}
func (f *fakeCases) Update(context.Context, finance.Actor, string, finance.UpdateInput) (finance.Request, error) {
	return f.value, f.err
}
func (f *fakeCases) Get(context.Context, string, string) (finance.Request, error) {
	return f.value, f.err
}
func (f *fakeCases) List(context.Context, string, finance.ListFilter) (finance.Page, error) {
	return finance.Page{Items: []finance.Request{f.value}}, f.err
}
func TestProtectedRoutesFailClosedAndStripHeaders(t *testing.T) {
	cases := &fakeCases{}
	a := &fixedAuth{err: auth.ErrUnauthenticated}
	server, _ := New(cases, a)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/finance-requests", nil)
	req.Header.Set("X-Tenant-ID", "attacker")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != 401 || a.sawUntrusted {
		t.Fatalf("expected sanitized 401, got %d untrusted=%v", res.Code, a.sawUntrusted)
	}
	a.err = nil
	a.principal = auth.Principal{SubjectID: testSubject, TenantID: testTenant, ScopeKind: "tenant", AllowedActions: map[string]bool{auth.ActionSelfManage: true}}
	res = httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/finance-requests", nil))
	if res.Code != 403 {
		t.Fatalf("expected capability 403, got %d", res.Code)
	}
}
func TestSelfCannotReadAnotherApplicantsRequest(t *testing.T) {
	cases := &fakeCases{value: finance.Request{ID: testRequest, TenantID: testTenant, ApplicantSubjectID: "99999999-9999-4999-8999-999999999999"}}
	a := &fixedAuth{principal: auth.Principal{SubjectID: testSubject, TenantID: testTenant, ScopeKind: "tenant", AllowedActions: map[string]bool{auth.ActionSelfRead: true}}}
	server, _ := New(cases, a)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/finance-requests/"+testRequest, nil))
	if res.Code != 404 {
		t.Fatalf("expected non-disclosing 404, got %d", res.Code)
	}
}
func TestCreateRequiresTenantScope(t *testing.T) {
	cases := &fakeCases{}
	a := &fixedAuth{principal: auth.Principal{SubjectID: testSubject, TenantID: testTenant, ScopeKind: "resources", AllowedActions: map[string]bool{auth.ActionSelfManage: true}, Resources: map[string]map[string]bool{"finance_request": {testRequest: true}}}}
	server, _ := New(cases, a)
	body := bytes.NewBufferString(`{"command_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","request_type":"payment","title":"test","amount_minor":100}`)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/finance-requests", body))
	if res.Code != 403 {
		t.Fatalf("expected scope 403, got %d", res.Code)
	}
}
