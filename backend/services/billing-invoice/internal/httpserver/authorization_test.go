package httpserver

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"omsbizfoundry/billing-invoice/internal/auth"
	"omsbizfoundry/billing-invoice/internal/invoice"
)

const (
	tenantID  = "11111111-1111-4111-8111-111111111111"
	subjectID = "22222222-2222-4222-8222-222222222222"
	projectA  = "33333333-3333-4333-8333-333333333333"
	projectB  = "44444444-4444-4444-8444-444444444444"
	invoiceID = "55555555-5555-4555-8555-555555555555"
)

type fakeAuthenticator struct {
	principal auth.Principal
	err       error
	header    string
}

func (f *fakeAuthenticator) ProviderName() string { return "fake" }
func (f *fakeAuthenticator) Authenticate(r *http.Request) (auth.Principal, error) {
	f.header = r.Header.Get("X-Actor-ID")
	return f.principal, f.err
}

func TestProtectRemovesUntrustedHeadersAndFailsClosed(t *testing.T) {
	authenticator := &fakeAuthenticator{err: errors.New("missing")}
	called := false
	handler := Protect(authenticator, auth.ActionRead, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Actor-ID", subjectID)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || called || authenticator.header != "" {
		t.Fatalf("status=%d called=%t leaked=%q", response.Code, called, authenticator.header)
	}

	authenticator.err = nil
	authenticator.principal = auth.Principal{AllowedActions: map[string]bool{}}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusForbidden || called {
		t.Fatalf("status=%d called=%t", response.Code, called)
	}
}

func TestAmountVisibilityAndHistoryAreServerComputed(t *testing.T) {
	registration := sampleRegistration(invoice.StatusRecorded)
	principal := auth.Principal{
		SubjectID: subjectID, TenantID: tenantID, ScopeKind: "resources",
		AllowedActions: map[string]bool{auth.ActionRead: true, auth.ActionHistoryRead: true, auth.ActionCorrect: true},
		Resources:      map[string]map[string]bool{"project": {projectA: true}},
	}
	if !CanAccess(registration, principal) {
		t.Fatal("matching project scope was denied")
	}
	view := ViewRegistration(registration, principal)
	if view.AmountVisible || view.AmountMinor != nil || view.AmountMinorDecimal != nil || view.Currency != nil {
		t.Fatalf("amount leaked: %#v", view)
	}
	if !contains(view.AllowedActions, "correct") || contains(view.AllowedActions, "amount_read") {
		t.Fatalf("allowed actions=%v", view.AllowedActions)
	}
	history := ViewHistory(invoice.HistoryPage{Items: []invoice.Revision{{Snapshot: registration}}}, principal)
	if len(history) != 1 || history[0].Snapshot.AmountVisible || history[0].Snapshot.AmountMinor != nil || history[0].Snapshot.AmountMinorDecimal != nil {
		t.Fatalf("history amount leaked: %#v", history)
	}

	principal.AllowedActions[auth.ActionAmountRead] = true
	view = ViewRegistration(registration, principal)
	if !view.AmountVisible || view.AmountMinor == nil || *view.AmountMinor != 12345 || view.Currency == nil || *view.Currency != "CNY" {
		t.Fatalf("amount not projected: %#v", view)
	}
	registration.AmountMinor = 900719925474099301
	view = ViewRegistration(registration, principal)
	if view.AmountMinorDecimal == nil || *view.AmountMinorDecimal != "900719925474099301" {
		t.Fatalf("exact decimal missing: %#v", view)
	}
	principal.AllowedActions[auth.ActionRecord] = true
	if !contains(listAllowedActions(principal), "record") {
		t.Fatal("record missing with amount permission")
	}
	delete(principal.AllowedActions, auth.ActionAmountRead)
	if contains(listAllowedActions(principal), "record") {
		t.Fatal("blind record exposed")
	}
}

func TestScopeAndStateConstrainActions(t *testing.T) {
	registration := sampleRegistration(invoice.StatusRegistrationInvalidated)
	principal := auth.Principal{
		SubjectID: subjectID, TenantID: tenantID, ScopeKind: "resources",
		AllowedActions: map[string]bool{
			auth.ActionRead: true, auth.ActionCorrect: true,
			auth.ActionRegistrationInvalidate: true, auth.ActionEvidenceManage: true,
		},
		Resources: map[string]map[string]bool{"project": {projectB: true}},
	}
	if CanAccess(registration, principal) {
		t.Fatal("wrong project scope was accepted")
	}
	principal.AllowedActions[auth.ActionAmountRead] = true
	deniedView := ViewRegistration(registration, principal)
	if deniedView.AmountVisible || deniedView.AmountMinor != nil || len(deniedView.AllowedActions) != 0 {
		t.Fatalf("out-of-scope view leaked data or actions: %#v", deniedView)
	}
	principal.Resources = map[string]map[string]bool{"invoice_registration": {invoiceID: true}}
	if !CanAccess(registration, principal) {
		t.Fatal("exact invoice scope was denied")
	}
	view := ViewRegistration(registration, principal)
	for _, action := range []string{"correct", "registration_invalidate", "evidence_manage"} {
		if contains(view.AllowedActions, action) {
			t.Fatalf("invalidated registration exposes %s", action)
		}
	}
	principal.TenantID = "77777777-7777-4777-8777-777777777777"
	if CanAccess(registration, principal) {
		t.Fatal("cross-tenant exact scope was accepted")
	}
}

func sampleRegistration(status string) invoice.Registration {
	resourceID := projectA
	return invoice.Registration{
		ID: invoiceID, TenantID: tenantID, Status: status, IssuedOn: "2026-09-27",
		AmountMinor: 12345, Currency: "CNY", Revision: 2,
		SourceRefs:   []invoice.SourceSnapshot{{Kind: invoice.SourceProject, ResourceID: &resourceID}},
		EvidenceRefs: []invoice.EvidenceRef{}, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
