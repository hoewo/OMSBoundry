package httpserver

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	createdInput     finance.CreateInput
	listFilter       finance.ListFilter
	restoreInput     finance.RestoreInput
	deleteInput      finance.DeleteDraftInput
}

func (f *fakeCases) Ping(context.Context) error { return f.err }
func (f *fakeCases) Create(_ context.Context, _ finance.Actor, input finance.CreateInput) (finance.Request, error) {
	f.createdInput = input
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
func (f *fakeCases) Restore(_ context.Context, _ finance.Actor, _ string, input finance.RestoreInput) (finance.Request, error) {
	f.restoreInput = input
	return f.value, f.err
}
func (f *fakeCases) DeleteDraft(_ context.Context, _ finance.Actor, _ string, input finance.DeleteDraftInput) (finance.Request, error) {
	f.deleteInput = input
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
func (f *fakeCases) List(_ context.Context, _ string, filter finance.ListFilter) (finance.Page, error) {
	f.listFilter = filter
	value := f.value
	value.Details.Payee.AccountNumber = ""
	return finance.Page{Items: []finance.Request{value}}, f.err
}
func (f *fakeCases) IsEvidenceReferenced(context.Context, string, string, int) (bool, error) {
	return true, f.err
}

func TestOpenAPICreateDerivesPersonAndCommandFromTrustedContext(t *testing.T) {
	cases := &fakeCases{value: finance.Request{ID: testRequest, TenantID: testTenant, ApplicantSubjectID: testSubject, RequestType: "reimbursement", Status: "draft", Revision: 1}}
	a := &fixedAuth{principal: auth.Principal{SubjectID: testSubject, TenantID: testTenant, Provider: "iam", CredentialID: "44444444-4444-4444-8444-444444444444", ScopeKind: "tenant", PersonID: "55555555-5555-4555-8555-555555555555", PersonRevision: 3, PersonDisplayName: "张三", PersonDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", AllowedActions: map[string]bool{auth.ActionSelfManage: true}}}
	server, _ := New(cases, a)
	body := bytes.NewBufferString(`{"title":"差旅报销","lines":[{"business_date":"2026-10-25","category_key":"travel","category_name":"差旅费","category_revision":1,"purpose":"出差","amount_minor":10000,"evidence":[{"file_id":"66666666-6666-4666-8666-666666666666","version":1,"purpose":"发票"}],"invoice":{"issuer_name":"测试开票方","invoice_date":"2026-10-24","amount_minor":10000}}],"payee":{"bank_name":"测试银行","account_number":"6222021234567890"}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/open-api/reimbursements", body)
	req.Header.Set("Idempotency-Key", "employee-create-0001")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	input := cases.createdInput
	if input.CommandID == "" || input.RequestType != "reimbursement" || input.AmountMinor != 10000 || input.Details.Applicant.ID != a.principal.PersonID || input.Details.Payee.Reference.ID != a.principal.PersonID || len(input.Details.Lines) != 1 || input.Details.Lines[0].ID == "" {
		t.Fatalf("trusted create input=%#v", input)
	}
	firstCommand, firstLine := input.CommandID, input.Details.Lines[0].ID
	body = bytes.NewBufferString(`{"title":"差旅报销","lines":[{"business_date":"2026-10-25","category_key":"travel","category_name":"差旅费","category_revision":1,"purpose":"出差","amount_minor":10000,"evidence":[{"file_id":"66666666-6666-4666-8666-666666666666","version":1,"purpose":"发票"}],"invoice":{"issuer_name":"测试开票方","invoice_date":"2026-10-24","amount_minor":10000}}],"payee":{"bank_name":"测试银行","account_number":"6222021234567890"}}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/open-api/reimbursements", body)
	req.Header.Set("Idempotency-Key", "employee-create-0001")
	server.ServeHTTP(httptest.NewRecorder(), req)
	if cases.createdInput.CommandID != firstCommand || cases.createdInput.Details.Lines[0].ID != firstLine {
		t.Fatal("same employee/action/key did not derive stable identifiers")
	}
}

func TestOpenAPIListBindsEmployeeAndParsesStableFilters(t *testing.T) {
	cases := &fakeCases{value: finance.Request{ID: testRequest, TenantID: testTenant, ApplicantSubjectID: testSubject, RequestType: "reimbursement", Status: "draft", Revision: 1}}
	a := &fixedAuth{principal: auth.Principal{SubjectID: testSubject, TenantID: testTenant, Provider: "iam", ScopeKind: "tenant", PersonID: "55555555-5555-4555-8555-555555555555", PersonRevision: 1, PersonDisplayName: "张三", PersonDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", AllowedActions: map[string]bool{auth.ActionSelfRead: true}}}
	server, _ := New(cases, a)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/open-api/reimbursements?status=draft&submitted_from=2026-10-01&submitted_to=2026-10-31&limit=25&archived=include", nil)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK || cases.listFilter.ApplicantSubjectID != testSubject || cases.listFilter.RequestType != "reimbursement" || cases.listFilter.Status != "draft" || cases.listFilter.Limit != 25 || !cases.listFilter.IncludeArchived || res.Header().Get("ETag") == "" {
		t.Fatalf("status=%d filter=%#v etag=%q body=%s", res.Code, cases.listFilter, res.Header().Get("ETag"), res.Body.String())
	}
}

func TestOpenAPIRestoreAndDeleteUseStableCommandsAndConfirmation(t *testing.T) {
	now := time.Now().UTC()
	cases := &fakeCases{value: finance.Request{ID: testRequest, TenantID: testTenant, ApplicantSubjectID: testSubject, RequestType: "reimbursement", Status: "draft", Revision: 2, ArchivedAt: &now}}
	a := &fixedAuth{principal: auth.Principal{SubjectID: testSubject, TenantID: testTenant, Provider: "iam", ScopeKind: "tenant", PersonID: "55555555-5555-4555-8555-555555555555", PersonRevision: 1, PersonDisplayName: "张三", PersonDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", AllowedActions: map[string]bool{auth.ActionSelfManage: true}}}
	server, _ := New(cases, a)
	restore := httptest.NewRequest(http.MethodPost, "/api/v1/open-api/reimbursements/"+testRequest+"/restore", nil)
	restore.Header.Set("Idempotency-Key", "restore-draft-0001")
	restoreResult := httptest.NewRecorder()
	server.ServeHTTP(restoreResult, restore)
	if restoreResult.Code != http.StatusOK || cases.restoreInput.CommandID == "" {
		t.Fatalf("restore status=%d input=%#v body=%s", restoreResult.Code, cases.restoreInput, restoreResult.Body.String())
	}
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/open-api/reimbursements/"+testRequest, bytes.NewBufferString(`{"confirmation":"DELETE","reason":"重复草稿"}`))
	deleteRequest.Header.Set("Idempotency-Key", "delete-draft-0001")
	deleteResult := httptest.NewRecorder()
	server.ServeHTTP(deleteResult, deleteRequest)
	if deleteResult.Code != http.StatusOK || cases.deleteInput.CommandID == "" || cases.deleteInput.Confirmation != "DELETE" || cases.deleteInput.Reason != "重复草稿" {
		t.Fatalf("delete status=%d input=%#v body=%s", deleteResult.Code, cases.deleteInput, deleteResult.Body.String())
	}
}

func TestInternalDeleteDraftPassesRequiredReason(t *testing.T) {
	cases := &fakeCases{value: finance.Request{ID: testRequest, TenantID: testTenant, ApplicantSubjectID: testSubject, RequestType: "reimbursement", Status: "draft", Revision: 1}}
	a := &fixedAuth{principal: auth.Principal{SubjectID: testSubject, TenantID: testTenant, Provider: "iam", CredentialID: "44444444-4444-4444-8444-444444444444", ScopeKind: "tenant", AllowedActions: map[string]bool{auth.ActionSelfManage: true}}}
	server, _ := New(cases, a)
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/finance-requests/"+testRequest, bytes.NewBufferString(`{"command_id":"55555555-5555-4555-8555-555555555555","confirmation":"DELETE","reason":"发票已在另一申请中提交"}`))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || cases.deleteInput.CommandID == "" || cases.deleteInput.Confirmation != "DELETE" || cases.deleteInput.Reason != "发票已在另一申请中提交" {
		t.Fatalf("delete status=%d input=%#v body=%s", response.Code, cases.deleteInput, response.Body.String())
	}
}

func TestTenantDeleteCapabilityCoversAnotherApplicantsPaymentRequest(t *testing.T) {
	other := "99999999-9999-4999-8999-999999999999"
	cases := &fakeCases{value: finance.Request{ID: testRequest, TenantID: testTenant, ApplicantSubjectID: other, RequestType: "payment", Status: "withdrawn", Revision: 4}}
	a := &fixedAuth{principal: auth.Principal{SubjectID: testSubject, TenantID: testTenant, Provider: "iam", CredentialID: "44444444-4444-4444-8444-444444444444", ScopeKind: "tenant", AllowedActions: map[string]bool{auth.ActionRequestDelete: true}}}
	server, _ := New(cases, a)
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/finance-requests/"+testRequest, bytes.NewBufferString(`{"command_id":"55555555-5555-4555-8555-555555555555","expected_revision":4,"confirmation":"DELETE","reason":"重复付款申请"}`))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !cases.deleteInput.TenantWide || cases.deleteInput.ExpectedRevision != 4 || cases.deleteInput.Reason != "重复付款申请" {
		t.Fatalf("delete status=%d input=%#v body=%s", response.Code, cases.deleteInput, response.Body.String())
	}
}

func TestFinanceDeletionAuditListRequiresIndependentCapability(t *testing.T) {
	now := time.Now().UTC()
	cases := &fakeCases{value: finance.Request{ID: testRequest, TenantID: testTenant, ApplicantSubjectID: testSubject, RequestType: "reimbursement", Status: "withdrawn", Revision: 3, DeletedAt: &now}}
	a := &fixedAuth{principal: auth.Principal{SubjectID: testSubject, TenantID: testTenant, ScopeKind: "tenant", AllowedActions: map[string]bool{auth.ActionTenantRead: true, auth.ActionRequestDelete: true}}}
	server, _ := New(cases, a)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/finance-requests?include_deleted=true", nil))
	if response.Code != http.StatusOK || !cases.listFilter.IncludeDeleted || !cases.listFilter.IncludeArchived || !bytes.Contains(response.Body.Bytes(), []byte(`"allowed_actions":["view_deleted"]`)) {
		t.Fatalf("status=%d filter=%#v body=%s", response.Code, cases.listFilter, response.Body.String())
	}
	a.principal.AllowedActions = map[string]bool{auth.ActionTenantRead: true}
	denied := httptest.NewRecorder()
	server.ServeHTTP(denied, httptest.NewRequest(http.MethodGet, "/api/v1/finance-requests?include_deleted=true", nil))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("deleted audit without capability status=%d body=%s", denied.Code, denied.Body.String())
	}
}

func TestEvidenceReferenceUsageRequiresTenantReferenceCapability(t *testing.T) {
	cases := &fakeCases{}
	a := &fixedAuth{principal: auth.Principal{SubjectID: testSubject, TenantID: testTenant, ScopeKind: "tenant", AllowedActions: map[string]bool{auth.ActionReferenceRead: true}}}
	server, _ := New(cases, a)
	fileID := "66666666-6666-4666-8666-666666666666"
	request := httptest.NewRequest(http.MethodGet, "/api/v1/references/evidence-files/"+fileID+"/versions/1/usage", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"tenant_id":"`+testTenant+`"`)) || !bytes.Contains(response.Body.Bytes(), []byte(`"referenced":true`)) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	a.principal.ScopeKind = "resources"
	denied := httptest.NewRecorder()
	server.ServeHTTP(denied, request)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("resource scope status=%d body=%s", denied.Code, denied.Body.String())
	}
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

func TestStrongDuplicateResponseReturnsStableMatchedRequestReferences(t *testing.T) {
	matched := "44444444-4444-4444-8444-444444444444"
	cases := &fakeCases{err: &finance.DuplicateError{Strong: true, MatchedRequestIDs: []string{matched}}}
	a := &fixedAuth{principal: auth.Principal{SubjectID: testSubject, TenantID: testTenant, ScopeKind: "tenant", AllowedActions: map[string]bool{auth.ActionSelfManage: true}}}
	server, _ := New(cases, a)
	body := bytes.NewBufferString(`{"command_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","expected_revision":1}`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/finance-requests/"+testRequest+"/submit", body))
	if response.Code != http.StatusConflict || !bytes.Contains(response.Body.Bytes(), []byte(`"code":"INVOICE_STRONG_DUPLICATE"`)) || !bytes.Contains(response.Body.Bytes(), []byte(`"matched_request_ids":["`+matched+`"]`)) || !bytes.Contains(response.Body.Bytes(), []byte(`发票已在其他财务申请中提交`)) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestStrongDuplicateDraftIsNotAdvertisedAsSubmittable(t *testing.T) {
	matched := "44444444-4444-4444-8444-444444444444"
	cases := &fakeCases{value: finance.Request{ID: testRequest, TenantID: testTenant, ApplicantSubjectID: testSubject, RequestType: "reimbursement", Status: "draft", Revision: 1, Details: finance.DetailSnapshot{Duplicate: finance.DuplicateAssessment{Status: "strong", MatchedRequestIDs: []string{matched}}}}}
	a := &fixedAuth{principal: auth.Principal{SubjectID: testSubject, TenantID: testTenant, ScopeKind: "tenant", AllowedActions: map[string]bool{auth.ActionSelfRead: true, auth.ActionSelfManage: true}}}
	server, _ := New(cases, a)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/finance-requests", nil))
	if response.Code != http.StatusOK || bytes.Contains(response.Body.Bytes(), []byte(`"submit"`)) || bytes.Contains(response.Body.Bytes(), []byte(matched)) || !bytes.Contains(response.Body.Bytes(), []byte(`"status":"strong"`)) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
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
