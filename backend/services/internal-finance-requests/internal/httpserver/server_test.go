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
	value            finance.Request
	err              error
	sensitiveReads   int
	sensitivePurpose string
}

func (f *fakeCases) Ping(context.Context) error { return f.err }
func (f *fakeCases) Create(context.Context, finance.Actor, finance.CreateInput) (finance.Request, error) {
	return f.value, f.err
}
func (f *fakeCases) Update(context.Context, finance.Actor, string, finance.UpdateInput) (finance.Request, error) {
	return f.value, f.err
}
func (f *fakeCases) Submit(context.Context, finance.Actor, string, finance.SubmitInput) (finance.Request, error) {
	return f.value, f.err
}
func (f *fakeCases) Withdraw(context.Context, finance.Actor, string, finance.WithdrawInput) (finance.Request, error) {
	return f.value, f.err
}
func (f *fakeCases) ConfirmPayment(context.Context, finance.Actor, string, finance.ConfirmPaymentInput) (finance.Request, error) {
	return f.value, f.err
}
func (f *fakeCases) ConfirmPayments(context.Context, finance.Actor, finance.BatchPaymentInput) finance.BatchPaymentResult {
	return finance.BatchPaymentResult{}
}
func (f *fakeCases) Get(context.Context, string, string) (finance.Request, error) {
	return f.value, f.err
}
func (f *fakeCases) GetSensitive(_ context.Context, _ finance.Actor, _ string, purpose string) (finance.Request, error) {
	f.sensitiveReads++
	f.sensitivePurpose = purpose
	return f.value, f.err
}
func (f *fakeCases) List(context.Context, string, finance.ListFilter) (finance.Page, error) {
	value := f.value
	value.Details.Payee.AccountNumber = ""
	return finance.Page{Items: []finance.Request{value}}, f.err
}
func (f *fakeCases) ListCategories(context.Context, string) (finance.CategoryPage, error) {
	return finance.CategoryPage{}, f.err
}
func (f *fakeCases) CreateCategory(context.Context, finance.Actor, finance.CategoryCreateInput) (finance.ExpenseCategory, error) {
	return finance.ExpenseCategory{}, f.err
}
func (f *fakeCases) UpdateCategory(context.Context, finance.Actor, string, finance.CategoryUpdateInput) (finance.ExpenseCategory, error) {
	return finance.ExpenseCategory{}, f.err
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

func TestExportsRequireIndependentCapabilitiesAndSensitivePurpose(t *testing.T) {
	cases := &fakeCases{value: finance.Request{ID: testRequest, TenantID: testTenant, ApplicantSubjectID: testSubject, Details: finance.DetailSnapshot{Payee: finance.PayeeSnapshot{AccountNumber: "6222000012345678", AccountLast4: "5678"}}}}
	a := &fixedAuth{principal: auth.Principal{SubjectID: testSubject, TenantID: testTenant, ScopeKind: "tenant", AllowedActions: map[string]bool{auth.ActionExport: true}}}
	server, _ := New(cases, a)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/finance-request-exports", nil))
	if res.Code != 200 || res.Header().Get("X-Content-Sensitivity") != "masked" || bytes.Contains(res.Body.Bytes(), []byte("6222000012345678")) {
		t.Fatalf("masked export status=%d header=%q body=%s", res.Code, res.Header().Get("X-Content-Sensitivity"), res.Body.String())
	}
	res = httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/finance-request-sensitive-exports?purpose=payroll", nil))
	if res.Code != 403 {
		t.Fatalf("sensitive export without capability=%d", res.Code)
	}
	a.principal.AllowedActions = map[string]bool{auth.ActionSensitiveExport: true}
	res = httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/finance-request-sensitive-exports", nil))
	if res.Code != 400 || cases.sensitiveReads != 0 {
		t.Fatalf("missing purpose status=%d reads=%d", res.Code, cases.sensitiveReads)
	}
	res = httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/finance-request-sensitive-exports?purpose=%E4%BB%98%E6%AC%BE%E5%AE%A1%E8%AE%A1", nil))
	if res.Code != 200 || cases.sensitiveReads != 1 || cases.sensitivePurpose != "付款审计" || !bytes.Contains(res.Body.Bytes(), []byte("6222000012345678")) {
		t.Fatalf("sensitive export status=%d reads=%d purpose=%q body=%s", res.Code, cases.sensitiveReads, cases.sensitivePurpose, res.Body.String())
	}
}
