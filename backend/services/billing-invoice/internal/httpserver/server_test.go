package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omsbizfoundry/billing-invoice/internal/auth"
	"omsbizfoundry/billing-invoice/internal/invoice"
)

const commandID = "66666666-6666-4666-8666-666666666666"

type useCasesStub struct {
	registration    invoice.Registration
	historyPage     invoice.HistoryPage
	listPage        invoice.ListPage
	pingErr         error
	createErr       error
	getErr          error
	historyErr      error
	createCalls     int
	historyCalls    int
	listCalls       int
	correctCalls    int
	invalidateCalls int
	evidenceCalls   int
	referenceCalls  int
	actor           invoice.Actor
	input           invoice.CreateInput
	tenant          string
}

func (s *useCasesStub) Ping(context.Context) error { return s.pingErr }
func (s *useCasesStub) Create(_ context.Context, actor invoice.Actor, input invoice.CreateInput) (invoice.Registration, error) {
	s.createCalls++
	s.actor, s.input = actor, input
	return s.registration, s.createErr
}
func (s *useCasesStub) Get(_ context.Context, tenant, _ string) (invoice.Registration, error) {
	s.tenant = tenant
	return s.registration, s.getErr
}
func (s *useCasesStub) List(_ context.Context, tenant string, _ invoice.ListQuery) (invoice.ListPage, error) {
	s.listCalls++
	s.tenant = tenant
	return s.listPage, nil
}
func (s *useCasesStub) History(_ context.Context, tenant, _ string, _ *int64, _ int) (invoice.HistoryPage, error) {
	s.historyCalls++
	s.tenant = tenant
	return s.historyPage, s.historyErr
}
func (s *useCasesStub) Correct(_ context.Context, _ invoice.Actor, _ string, _ invoice.CorrectInput) (invoice.Registration, error) {
	s.correctCalls++
	return s.registration, nil
}
func (s *useCasesStub) Invalidate(_ context.Context, _ invoice.Actor, _ string, _ invoice.InvalidateInput) (invoice.Registration, error) {
	s.invalidateCalls++
	return s.registration, nil
}
func (s *useCasesStub) ReplaceEvidence(_ context.Context, _ invoice.Actor, _ string, _ invoice.EvidenceInput) (invoice.Registration, error) {
	s.evidenceCalls++
	return s.registration, nil
}
func (s *useCasesStub) Reference(_ context.Context, tenant, _ string) (invoice.Reference, error) {
	s.referenceCalls++
	return invoice.Reference{SchemaVersion: "1", SourceApplication: "billing-invoice", ID: invoiceID, TenantID: tenant, Status: invoice.StatusRecorded, AmountMinor: 1, Currency: "CNY", Revision: 1}, nil
}

func TestProtectedCreateRequiresCapabilityAndMatchingProjectScope(t *testing.T) {
	body := createBody(projectA)
	principal := recordPrincipal(projectA)

	for _, test := range []struct {
		name      string
		principal auth.Principal
		authErr   error
		want      int
		calls     int
	}{
		{name: "missing credential", authErr: auth.ErrUnauthenticated, want: http.StatusUnauthorized},
		{name: "missing capability", principal: scopedPrincipal(auth.ActionRead, projectA), want: http.StatusForbidden},
		{name: "record without amount read", principal: scopedPrincipal(auth.ActionRecord, projectA), want: http.StatusForbidden},
		{name: "adjacent project", principal: recordPrincipal(projectB), want: http.StatusForbidden},
		{name: "allowed", principal: principal, want: http.StatusCreated, calls: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			useCases := &useCasesStub{registration: sampleRegistration(invoice.StatusRecorded)}
			authenticator := &fakeAuthenticator{principal: test.principal, err: test.authErr}
			server, err := New(useCases, authenticator)
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodPost, "/api/v1/invoice-registrations", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Actor-ID", "forged")
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)
			if response.Code != test.want || useCases.createCalls != test.calls {
				t.Fatalf("status=%d calls=%d body=%s", response.Code, useCases.createCalls, response.Body.String())
			}
			if authenticator.header != "" {
				t.Fatalf("forged identity reached authenticator: %q", authenticator.header)
			}
			if test.calls == 1 {
				var view RegistrationView
				if err := json.Unmarshal(response.Body.Bytes(), &view); err != nil || !view.AmountVisible || view.AmountMinor == nil || view.AmountMinorDecimal == nil {
					t.Fatalf("create view=%#v error=%v", view, err)
				}
				if useCases.actor.CredentialID != principal.CredentialID || useCases.actor.TenantID != tenantID {
					t.Fatalf("actor=%#v", useCases.actor)
				}
			}
		})
	}
}

func TestProtectedCreateRejectsAnyAdjacentProjectSource(t *testing.T) {
	body := strings.Replace(
		createBody(projectA),
		`]`,
		`,{"kind":"project","resource_id":"`+projectB+`","source_revision":1}]`,
		1,
	)
	useCases := &useCasesStub{registration: sampleRegistration(invoice.StatusRecorded)}
	server, err := New(useCases, &fakeAuthenticator{principal: recordPrincipal(projectA)})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/invoice-registrations", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || useCases.createCalls != 0 || !strings.Contains(response.Body.String(), "SCOPE_DENIED") {
		t.Fatalf("status=%d calls=%d body=%s", response.Code, useCases.createCalls, response.Body.String())
	}
}

func TestDetailAndHistoryEnforceScopeAndRedactEveryAmount(t *testing.T) {
	registration := sampleRegistration(invoice.StatusRecorded)
	historyPage := invoice.HistoryPage{Items: []invoice.Revision{{InvoiceRegistrationID: invoiceID, TenantID: tenantID, Revision: 1, Snapshot: registration}}}
	useCases := &useCasesStub{registration: registration, historyPage: historyPage}
	authenticator := &fakeAuthenticator{principal: scopedPrincipal(auth.ActionRead, projectB)}
	server, _ := New(useCases, authenticator)

	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/invoice-registrations/"+invoiceID, nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("adjacent project status=%d body=%s", response.Code, response.Body.String())
	}

	principal := scopedPrincipal(auth.ActionRead, projectA)
	authenticator.principal = principal
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/invoice-registrations/"+invoiceID, nil))
	var detail RegistrationView
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &detail) != nil || detail.AmountVisible || detail.AmountMinor != nil || detail.Currency != nil {
		t.Fatalf("redacted detail status=%d view=%#v body=%s", response.Code, detail, response.Body.String())
	}

	principal.AllowedActions = map[string]bool{auth.ActionHistoryRead: true}
	authenticator.principal = principal
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/invoice-registrations/"+invoiceID+"/history?limit=10", nil))
	var history historyResponse
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &history) != nil || len(history.Items) != 1 || history.Items[0].Snapshot.AmountVisible || history.Items[0].Snapshot.AmountMinor != nil {
		t.Fatalf("history status=%d response=%#v body=%s", response.Code, history, response.Body.String())
	}

	principal.AllowedActions[auth.ActionAmountRead] = true
	authenticator.principal = principal
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/invoice-registrations/"+invoiceID+"/history", nil))
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &history) != nil || history.Items[0].Snapshot.AmountMinor == nil || *history.Items[0].Snapshot.AmountMinor != registration.AmountMinor {
		t.Fatalf("visible history status=%d response=%#v", response.Code, history)
	}
}

func TestReferenceErrorsHaveDistinctHTTPMeaning(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want int
		code string
	}{
		{name: "invalid", err: invoice.ErrReferenceInvalid, want: http.StatusUnprocessableEntity, code: "REFERENCE_INVALID"},
		{name: "unavailable", err: invoice.ErrReferenceUnavailable, want: http.StatusServiceUnavailable, code: "REFERENCE_SERVICE_UNAVAILABLE"},
		{name: "command conflict", err: invoice.ErrCommandConflict, want: http.StatusConflict, code: "COMMAND_PAYLOAD_CONFLICT"},
		{name: "unexpected", err: errors.New("database down"), want: http.StatusInternalServerError, code: "INTERNAL_ERROR"},
	} {
		t.Run(test.name, func(t *testing.T) {
			useCases := &useCasesStub{createErr: test.err}
			server, _ := New(useCases, &fakeAuthenticator{principal: recordPrincipal(projectA)})
			request := httptest.NewRequest(http.MethodPost, "/api/v1/invoice-registrations", strings.NewReader(createBody(projectA)))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)
			if response.Code != test.want || !strings.Contains(response.Body.String(), test.code) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestHealthAndHistoryQueryValidation(t *testing.T) {
	useCases := &useCasesStub{registration: sampleRegistration(invoice.StatusRecorded)}
	server, _ := New(useCases, &fakeAuthenticator{principal: scopedPrincipal(auth.ActionHistoryRead, projectA)})
	ready := httptest.NewRecorder()
	server.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if ready.Code != http.StatusOK || ready.Header().Get("X-Request-ID") == "" || ready.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("ready status=%d headers=%v", ready.Code, ready.Header())
	}
	invalid := httptest.NewRecorder()
	server.ServeHTTP(invalid, httptest.NewRequest(http.MethodGet, "/api/v1/invoice-registrations/"+invoiceID+"/history?before_revision=1", nil))
	if invalid.Code != http.StatusBadRequest || useCases.historyCalls != 0 {
		t.Fatalf("invalid query status=%d calls=%d", invalid.Code, useCases.historyCalls)
	}
}

func TestListRequiresAVisibleProjectForResourceScopeAndRedactsAggregate(t *testing.T) {
	page := invoice.ListPage{Items: []invoice.Registration{sampleRegistration(invoice.StatusRecorded)}, EffectiveAmountMinor: 12345, Currency: "CNY"}
	useCases := &useCasesStub{listPage: page}
	principal := scopedPrincipal(auth.ActionRead, projectA)
	authenticator := &fakeAuthenticator{principal: principal}
	server, _ := New(useCases, authenticator)

	denied := httptest.NewRecorder()
	server.ServeHTTP(denied, httptest.NewRequest(http.MethodGet, "/api/v1/invoice-registrations", nil))
	if denied.Code != http.StatusForbidden || useCases.listCalls != 0 {
		t.Fatalf("unscoped list status=%d calls=%d", denied.Code, useCases.listCalls)
	}

	allowed := httptest.NewRecorder()
	server.ServeHTTP(allowed, httptest.NewRequest(http.MethodGet, "/api/v1/invoice-registrations?project_id="+projectA, nil))
	var redacted listResponse
	if allowed.Code != http.StatusOK || json.Unmarshal(allowed.Body.Bytes(), &redacted) != nil || len(redacted.Items) != 1 || redacted.Items[0].AmountVisible || redacted.EffectiveAmountMinor != nil || redacted.Currency != nil {
		t.Fatalf("redacted list status=%d response=%#v body=%s", allowed.Code, redacted, allowed.Body.String())
	}

	principal.AllowedActions[auth.ActionAmountRead] = true
	authenticator.principal = principal
	visible := httptest.NewRecorder()
	server.ServeHTTP(visible, httptest.NewRequest(http.MethodGet, "/api/v1/invoice-registrations?project_id="+projectA, nil))
	var withAmount listResponse
	if visible.Code != http.StatusOK || json.Unmarshal(visible.Body.Bytes(), &withAmount) != nil || withAmount.EffectiveAmountMinor == nil || *withAmount.EffectiveAmountMinor != 12345 || withAmount.Currency == nil || *withAmount.Currency != "CNY" {
		t.Fatalf("visible list status=%d response=%#v", visible.Code, withAmount)
	}
}

func TestCorrectionCannotReplaceProjectScopeAndReferenceRequiresExactWorkloadScope(t *testing.T) {
	useCases := &useCasesStub{registration: sampleRegistration(invoice.StatusRecorded)}
	principal := scopedPrincipal(auth.ActionCorrect, projectA)
	authenticator := &fakeAuthenticator{principal: principal}
	server, _ := New(useCases, authenticator)
	correction := `{"command_id":"55555555-5555-4555-8555-555555555555","expected_revision":1,"reason":"改正来源","patch":{"source_refs":[{"kind":"project","resource_id":"` + projectB + `","source_revision":1}]}}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/invoice-registrations/"+invoiceID+"/corrections", strings.NewReader(correction))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || useCases.correctCalls != 0 {
		t.Fatalf("adjacent correction status=%d calls=%d body=%s", response.Code, useCases.correctCalls, response.Body.String())
	}

	principal = auth.Principal{
		SubjectID: subjectID, TenantID: tenantID, DisplayName: "Billing workload", Provider: "iam", CredentialID: commandID,
		ScopeKind: "resources", AllowedActions: map[string]bool{auth.ActionReferenceRead: true},
		Resources: map[string]map[string]bool{"invoice_registration": {invoiceID: true}},
	}
	authenticator.principal = principal
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/references/invoice-registrations/"+invoiceID, nil))
	if response.Code != http.StatusOK || useCases.referenceCalls != 1 {
		t.Fatalf("exact reference status=%d calls=%d body=%s", response.Code, useCases.referenceCalls, response.Body.String())
	}

	principal.Resources["invoice_registration"]["aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"] = true
	authenticator.principal = principal
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/references/invoice-registrations/"+invoiceID, nil))
	if response.Code != http.StatusForbidden || useCases.referenceCalls != 1 {
		t.Fatalf("broad reference status=%d calls=%d", response.Code, useCases.referenceCalls)
	}
	delete(principal.Resources["invoice_registration"], "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	principal.AllowedActions[auth.ActionRead] = true
	authenticator.principal = principal
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/references/invoice-registrations/"+invoiceID, nil))
	if response.Code != http.StatusForbidden || useCases.referenceCalls != 1 {
		t.Fatalf("multi-action reference status=%d calls=%d", response.Code, useCases.referenceCalls)
	}
}

func TestAmountCorrectionRequiresAmountVisibilityCapability(t *testing.T) {
	useCases := &useCasesStub{registration: sampleRegistration(invoice.StatusRecorded)}
	principal := scopedPrincipal(auth.ActionCorrect, projectA)
	authenticator := &fakeAuthenticator{principal: principal}
	server, _ := New(useCases, authenticator)
	body := `{"command_id":"55555555-5555-4555-8555-555555555555","expected_revision":1,"reason":"纠正金额","patch":{"amount_minor":23456}}`

	denied := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/invoice-registrations/"+invoiceID+"/corrections", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(denied, request)
	if denied.Code != http.StatusForbidden || useCases.correctCalls != 0 {
		t.Fatalf("blind amount correction status=%d calls=%d body=%s", denied.Code, useCases.correctCalls, denied.Body.String())
	}

	principal.AllowedActions[auth.ActionAmountRead] = true
	authenticator.principal = principal
	allowed := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/invoice-registrations/"+invoiceID+"/corrections", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(allowed, request)
	if allowed.Code != http.StatusOK || useCases.correctCalls != 1 {
		t.Fatalf("visible amount correction status=%d calls=%d body=%s", allowed.Code, useCases.correctCalls, allowed.Body.String())
	}
}

func TestWorkspaceIsPublicButBillingFactsRemainProtected(t *testing.T) {
	server, err := New(&useCasesStub{}, &fakeAuthenticator{err: auth.ErrUnauthenticated})
	if err != nil {
		t.Fatal(err)
	}

	page := httptest.NewRecorder()
	server.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK {
		t.Fatalf("workspace status=%d body=%s", page.Code, page.Body.String())
	}
	for _, marker := range []string{"开票登记工作区", "当前允许动作", "assets/demo.js", "登记无效", "Evidence 引用"} {
		if !strings.Contains(page.Body.String(), marker) {
			t.Fatalf("workspace is missing marker %q", marker)
		}
	}
	csp := page.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self'") || !strings.Contains(csp, "connect-src 'self'") {
		t.Fatalf("workspace csp=%q", csp)
	}

	script := httptest.NewRecorder()
	server.ServeHTTP(script, httptest.NewRequest(http.MethodGet, "/assets/demo.js", nil))
	if script.Code != http.StatusOK {
		t.Fatalf("workspace script status=%d", script.Code)
	}
	for _, marker := range []string{"allowed_actions", "error.status===409", "error.status===503", "amount_visible", "expected_revision", "billing-invoice"} {
		if !strings.Contains(script.Body.String(), marker) {
			t.Fatalf("workspace script is missing marker %q", marker)
		}
	}

	protected := httptest.NewRecorder()
	server.ServeHTTP(protected, httptest.NewRequest(http.MethodGet, "/api/v1/invoice-registrations", nil))
	if protected.Code != http.StatusUnauthorized {
		t.Fatalf("protected list status=%d", protected.Code)
	}
}

func recordPrincipal(project string) auth.Principal {
	p := scopedPrincipal(auth.ActionRecord, project)
	p.AllowedActions[auth.ActionAmountRead] = true
	return p
}

func TestStatisticsInvoiceExportRequiresExportAmountAndScope(t *testing.T) {
	useCases := &useCasesStub{registration: sampleRegistration(invoice.StatusRecorded)}
	authenticator := &fakeAuthenticator{principal: scopedPrincipal(auth.ActionRead, projectA)}
	server, _ := New(useCases, authenticator)
	request := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		server.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/invoice-registrations/"+invoiceID+"/export-view", nil))
		return w
	}
	if w := request(); w.Code != 403 {
		t.Fatalf("read-only exported: %d", w.Code)
	}
	authenticator.principal = scopedPrincipal(auth.ActionExport, projectA)
	if w := request(); w.Code != 403 {
		t.Fatalf("amount hidden but exported: %d", w.Code)
	}
	authenticator.principal.AllowedActions[auth.ActionAmountRead] = true
	if w := request(); w.Code != 200 || !strings.Contains(w.Body.String(), `"amount_minor_decimal"`) {
		t.Fatalf("export failed: %d %s", w.Code, w.Body.String())
	}
	authenticator.principal.Resources = map[string]map[string]bool{"project": {projectB: true}}
	if w := request(); w.Code != 403 {
		t.Fatalf("adjacent project exported: %d", w.Code)
	}
}

func scopedPrincipal(action, project string) auth.Principal {
	return auth.Principal{
		SubjectID: subjectID, TenantID: tenantID, DisplayName: "测试财务", Provider: "iam", CredentialID: commandID,
		ScopeKind: "resources", AllowedActions: map[string]bool{action: true},
		Resources: map[string]map[string]bool{"project": {project: true}},
	}
}

func createBody(project string) string {
	return `{"command_id":"` + commandID + `","issued_on":"2026-09-27","amount_minor":12345,"currency":"CNY","issuer_party_ref":{"id":"77777777-7777-4777-8777-777777777777","revision":1},"recipient_party_ref":{"id":"88888888-8888-4888-8888-888888888888","revision":1},"source_refs":[{"kind":"project","resource_id":"` + project + `","source_revision":1}],"note":"首张发票"}`
}
