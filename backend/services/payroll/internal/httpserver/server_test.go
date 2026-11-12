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

type executionCases struct {
	*fakeCases
	run         domain.ExecutionRun
	paymentErrs []error
	paymentCall int
}

func (f *executionCases) CreateRegularRun(context.Context, domain.Actor, domain.CreateRegularRunInput) (domain.ExecutionRun, error) {
	return f.run, f.err
}
func (f *executionCases) CreateAdjustmentRun(context.Context, domain.Actor, domain.CreateAdjustmentRunInput) (domain.ExecutionRun, error) {
	return f.run, f.err
}
func (f *executionCases) PreviewPayrollImport(context.Context, domain.Actor, string, domain.PreviewPayrollImportInput) (domain.PayrollImportPreview, error) {
	return domain.PayrollImportPreview{ID: "55555555-5555-4555-8555-555555555555", PayrollRunID: testRun, PreviewHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, f.err
}
func (f *executionCases) CommitPayrollImport(context.Context, domain.Actor, string, domain.CommitPayrollImportInput) (domain.ExecutionRun, error) {
	return f.run, f.err
}
func (f *executionCases) GetExecutionRun(context.Context, string, string) (domain.ExecutionRun, error) {
	return f.run, f.err
}
func (f *executionCases) ListExecutionRuns(context.Context, string) ([]domain.ExecutionRun, error) {
	return []domain.ExecutionRun{f.run}, f.err
}
func (f *executionCases) RevisePayrollLine(context.Context, domain.Actor, string, string, domain.ReviseLineInput) (domain.ExecutionRun, error) {
	return f.run, f.err
}
func (f *executionCases) ApplyAttendanceSummary(context.Context, domain.Actor, string, string, domain.ApplyAttendanceSummaryInput) (domain.ExecutionRun, error) {
	return f.run, f.err
}
func (f *executionCases) ReviewPayrollRun(context.Context, domain.Actor, string, domain.RunCommandInput) (domain.ExecutionRun, error) {
	return f.run, f.err
}
func (f *executionCases) ConfirmZeroLine(context.Context, domain.Actor, string, string, domain.LineCommandInput) (domain.ExecutionRun, error) {
	return f.run, f.err
}
func (f *executionCases) VoidPayrollRun(context.Context, domain.Actor, string, domain.RunCommandInput) (domain.ExecutionRun, error) {
	return f.run, f.err
}
func (f *executionCases) RecordPayment(context.Context, domain.Actor, string, string, domain.RecordPaymentInput) (domain.ExecutionRun, error) {
	index := f.paymentCall
	f.paymentCall++
	if index < len(f.paymentErrs) {
		return f.run, f.paymentErrs[index]
	}
	return f.run, f.err
}
func (f *executionCases) SyncMembers(context.Context, domain.Actor, string, domain.SyncMembersInput) (domain.ExecutionRun, error) {
	return f.run, f.err
}
func (f *executionCases) PreviewCompensationSync(context.Context, domain.Actor, string, string) (domain.SnapshotSyncPreview, error) {
	return domain.SnapshotSyncPreview{}, f.err
}
func (f *executionCases) ConfirmCompensationSync(context.Context, domain.Actor, string, string, domain.ConfirmSnapshotSyncInput) (domain.ExecutionRun, error) {
	return f.run, f.err
}
func (f *executionCases) PreviewPaymentProfileSync(context.Context, domain.Actor, string, string) (domain.SnapshotSyncPreview, error) {
	return domain.SnapshotSyncPreview{}, f.err
}
func (f *executionCases) ConfirmPaymentProfileSync(context.Context, domain.Actor, string, string, domain.ConfirmSnapshotSyncInput) (domain.ExecutionRun, error) {
	return f.run, f.err
}
func (f *executionCases) ListSelfPayslips(context.Context, domain.Actor) (domain.SelfPayslipPage, error) {
	return domain.SelfPayslipPage{PersonID: testSubject, PersonDisplayName: "本人", Items: []domain.Payslip{{PayrollRunID: testRun, Status: "paid"}}}, f.err
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

func TestExecutionRoutesEnforceSeparateCapabilitiesAndExactLineScope(t *testing.T) {
	const lineID = "44444444-4444-4444-8444-444444444444"
	cases := &executionCases{fakeCases: &fakeCases{}, run: domain.ExecutionRun{Run: domain.Run{ID: testRun, TenantID: testTenant, Revision: 2}, Lines: []domain.PayrollLine{{ID: lineID, PayrollRunID: testRun}}}}
	a := &fixedAuth{principal: auth.Principal{SubjectID: testSubject, TenantID: testTenant, ScopeKind: "tenant", AllowedActions: map[string]bool{auth.ActionRunManage: true}, Resources: map[string]map[string]bool{}}}
	server, _ := New(cases, a)
	createBody := bytes.NewBufferString(`{"command_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","period":"2026-10","label":"October","planned_pay_date":"2026-10-31","contribution_scheme_id":"55555555-5555-4555-8555-555555555555","contribution_scheme_revision":1,"person_ids":["66666666-6666-4666-8666-666666666666"]}`)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/payroll-periods", createBody))
	if res.Code != http.StatusCreated {
		t.Fatalf("create execution run status=%d body=%s", res.Code, res.Body.String())
	}

	a.principal = auth.Principal{SubjectID: testSubject, TenantID: testTenant, ScopeKind: "resources", AllowedActions: map[string]bool{auth.ActionPaymentConfirm: true}, Resources: map[string]map[string]bool{"payroll_run": {testRun: true}}}
	paymentBody := `{"command_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaab","expected_run_revision":2,"expected_line_revision":1,"actual_amount_minor":100,"occurred_on":"2026-10-31"}`
	res = httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/payroll-periods/"+testRun+"/lines/"+lineID+"/payments", bytes.NewBufferString(paymentBody)))
	if res.Code != http.StatusForbidden {
		t.Fatalf("payment without exact line scope status=%d body=%s", res.Code, res.Body.String())
	}
	a.principal.Resources["payroll_line"] = map[string]bool{lineID: true}
	res = httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/payroll-periods/"+testRun+"/lines/"+lineID+"/payments", bytes.NewBufferString(paymentBody)))
	if res.Code != http.StatusOK || cases.paymentCall != 1 {
		t.Fatalf("exact scoped payment status=%d calls=%d body=%s", res.Code, cases.paymentCall, res.Body.String())
	}
}

func TestAdjustmentRunRequiresTenantManageCapability(t *testing.T) {
	cases := &executionCases{fakeCases: &fakeCases{}, run: domain.ExecutionRun{Run: domain.Run{ID: testRun, TenantID: testTenant, Revision: 1}, RunKind: "adjustment"}}
	a := &fixedAuth{principal: auth.Principal{SubjectID: testSubject, TenantID: testTenant, ScopeKind: "tenant", AllowedActions: map[string]bool{auth.ActionTenantRead: true}}}
	server, _ := New(cases, a)
	body := `{"command_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","period":"2026-10","label":"October supplement","planned_pay_date":"2026-10-31","items":[{"source_payroll_line_id":"44444444-4444-4444-8444-444444444444","source_payroll_line_revision":3,"direction":"supplement","amount_minor":100,"reason":"补发"}]}`
	res := httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/payroll-adjustment-runs", bytes.NewBufferString(body)))
	if res.Code != http.StatusForbidden {
		t.Fatalf("adjustment without manage capability status=%d body=%s", res.Code, res.Body.String())
	}
	a.principal.AllowedActions[auth.ActionRunManage] = true
	res = httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/payroll-adjustment-runs", bytes.NewBufferString(body)))
	if res.Code != http.StatusCreated || !bytes.Contains(res.Body.Bytes(), []byte(`"run_kind":"adjustment"`)) {
		t.Fatalf("adjustment create status=%d body=%s", res.Code, res.Body.String())
	}
}

func TestPayrollImportRequiresManageCapabilityAndExactRunScope(t *testing.T) {
	cases := &executionCases{fakeCases: &fakeCases{}, run: domain.ExecutionRun{Run: domain.Run{ID: testRun, TenantID: testTenant, Revision: 2}}}
	a := &fixedAuth{principal: auth.Principal{SubjectID: testSubject, TenantID: testTenant, ScopeKind: "resources", AllowedActions: map[string]bool{auth.ActionRunManage: true}, Resources: map[string]map[string]bool{"payroll_run": {}}}}
	server, _ := New(cases, a)
	body := `{"expected_run_revision":2,"rows":[{"person_id":"44444444-4444-4444-8444-444444444444","expected_line_revision":1,"performance_minor":100,"tax_minor":20,"tax_source_note":"外部个税表"}]}`
	res := httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/payroll-periods/"+testRun+"/imports:preview", bytes.NewBufferString(body)))
	if res.Code != http.StatusForbidden {
		t.Fatalf("import preview without exact run scope status=%d body=%s", res.Code, res.Body.String())
	}
	a.principal.Resources["payroll_run"][testRun] = true
	res = httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/payroll-periods/"+testRun+"/imports:preview", bytes.NewBufferString(body)))
	if res.Code != http.StatusOK || !bytes.Contains(res.Body.Bytes(), []byte(`"payroll_run_id":"`+testRun+`"`)) {
		t.Fatalf("exact scoped import preview status=%d body=%s", res.Code, res.Body.String())
	}

	a.principal.AllowedActions = map[string]bool{auth.ActionTenantRead: true}
	res = httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/payroll-periods/"+testRun+"/imports:commit", bytes.NewBufferString(`{"command_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","preview_id":"55555555-5555-4555-8555-555555555555","preview_hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","expected_run_revision":2,"reason":"确认导入"}`)))
	if res.Code != http.StatusForbidden {
		t.Fatalf("import commit without manage capability status=%d body=%s", res.Code, res.Body.String())
	}
}

func TestSelfPayslipsUseAuthenticatedSubjectAndSeparateCapability(t *testing.T) {
	cases := &executionCases{fakeCases: &fakeCases{}}
	a := &fixedAuth{principal: auth.Principal{SubjectID: testSubject, TenantID: testTenant, ScopeKind: "tenant", AllowedActions: map[string]bool{auth.ActionTenantRead: true}}}
	server, _ := New(cases, a)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/me/payslips", nil))
	if res.Code != http.StatusForbidden {
		t.Fatalf("self payslips without self capability status=%d body=%s", res.Code, res.Body.String())
	}
	a.principal.AllowedActions = map[string]bool{auth.ActionSelfRead: true}
	res = httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/me/payslips", nil))
	if res.Code != http.StatusOK || !bytes.Contains(res.Body.Bytes(), []byte(`"person_id":"`+testSubject+`"`)) || !bytes.Contains(res.Body.Bytes(), []byte(`"status":"paid"`)) {
		t.Fatalf("self payslips status=%d body=%s", res.Code, res.Body.String())
	}
}

func TestBatchPaymentReportsIndependentSuccessFailureAndUnknown(t *testing.T) {
	lineA := "44444444-4444-4444-8444-444444444441"
	lineB := "44444444-4444-4444-8444-444444444442"
	lineC := "44444444-4444-4444-8444-444444444443"
	cases := &executionCases{fakeCases: &fakeCases{}, run: domain.ExecutionRun{Run: domain.Run{ID: testRun, TenantID: testTenant, Revision: 2}}, paymentErrs: []error{nil, domain.ErrPaymentFailed, domain.ErrPaymentUnknown}}
	a := &fixedAuth{principal: auth.Principal{SubjectID: testSubject, TenantID: testTenant, ScopeKind: "tenant", AllowedActions: map[string]bool{auth.ActionPaymentBatchConfirm: true}, Resources: map[string]map[string]bool{}}}
	server, _ := New(cases, a)
	cases.run.Lines = []domain.PayrollLine{{ID: lineA, Revision: 1}, {ID: lineB, Revision: 1}, {ID: lineC, Revision: 1}}
	body := bytes.NewBufferString(`{"items":[{"line_id":"` + lineA + `","input":{"command_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa1","expected_run_revision":2,"expected_line_revision":1,"actual_amount_minor":100,"occurred_on":"2026-10-31"}},{"line_id":"` + lineB + `","input":{"command_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa2","expected_run_revision":2,"expected_line_revision":1,"actual_amount_minor":100,"occurred_on":"2026-10-31"}},{"line_id":"` + lineC + `","input":{"command_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa3","expected_run_revision":2,"expected_line_revision":1,"actual_amount_minor":100,"occurred_on":"2026-10-31"}}]}`)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/payroll-periods/"+testRun+"/payments:batch", body))
	if res.Code != http.StatusMultiStatus || !bytes.Contains(res.Body.Bytes(), []byte(`"status":"succeeded"`)) || !bytes.Contains(res.Body.Bytes(), []byte(`"status":"failed"`)) || !bytes.Contains(res.Body.Bytes(), []byte(`"status":"unknown"`)) {
		t.Fatalf("batch result status=%d body=%s", res.Code, res.Body.String())
	}
}
