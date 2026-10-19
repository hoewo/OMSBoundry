package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omsbizfoundry/approval-audit-results/internal/auth"
	"omsbizfoundry/approval-audit-results/internal/results"
)

const materialResultID = "44444444-4444-4444-8444-444444444444"

type materialUseCases struct {
	unitUseCases
	value                        results.Result
	err                          error
	getCalls                     int
	requestedTenant, requestedID string
}

func (u *materialUseCases) Get(_ context.Context, tenant, id string) (results.Result, error) {
	u.getCalls++
	u.requestedTenant, u.requestedID = tenant, id
	return u.value, u.err
}

type materialUnauthenticated struct{}

func (*materialUnauthenticated) ProviderName() string { return "test" }
func (*materialUnauthenticated) Authenticate(*http.Request) (auth.Principal, error) {
	return auth.Principal{}, auth.ErrUnauthenticated
}

func materialHTTPFixture() *materialUseCases {
	return &materialUseCases{value: results.Result{ID: materialResultID, TenantID: unitTenant, Project: results.ProjectSnapshot{ID: unitProject}, Kind: results.KindAudit, State: results.StateRecorded, Revision: 3,
		TotalAmountMinor: 12345, Source: results.SourceSnapshot{Digest: "private-amount-digest"}, Evidence: []results.EvidenceSnapshot{{EvidenceRef: results.EvidenceRef{FileID: "55555555-5555-4555-8555-555555555555", Version: 2, Slot: "audit_document"}, DisplayName: "审计.pdf"}}}}
}

func TestMaterialReferenceHTTPNativeActionAndOwnedScopeOnly(t *testing.T) {
	for _, scope := range []string{"project", "approval_audit_result", "tenant"} {
		t.Run(scope, func(t *testing.T) {
			useCases := materialHTTPFixture()
			principal := unitPrincipal(auth.ActionRead)
			if scope == "tenant" {
				principal.ScopeKind, principal.Resources = "tenant", nil
			} else if scope == "approval_audit_result" {
				principal.Resources = map[string]map[string]bool{scope: {materialResultID: true}}
			}
			server, err := New(useCases, &unitAuthenticator{principal: principal})
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/v1/references/approval-audit-results/"+materialResultID+"/materials", nil)
			request.Header.Set("X-Tenant-ID", "untrusted-tenant")
			request.Header.Set("X-Allowed-Actions", auth.ActionAmountRead)
			server.ServeHTTP(response, request)
			var projection results.MaterialReferenceProjection
			if err := json.Unmarshal(response.Body.Bytes(), &projection); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusOK || projection.ID != materialResultID || projection.Revision != 3 || len(projection.Items) != 1 || useCases.getCalls != 1 || useCases.requestedTenant != unitTenant || useCases.requestedID != materialResultID || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d projection=%#v calls=%d", response.Code, projection, useCases.getCalls)
			}
			for _, forbidden := range []string{"amount", "private", "currency", "object_key", "url", "credential"} {
				if strings.Contains(response.Body.String(), forbidden) {
					t.Fatalf("leaked %q", forbidden)
				}
			}
		})
	}
}

func TestMaterialReferenceHTTPDeniedAndMalformedSourcesFailClosed(t *testing.T) {
	tests := []struct {
		name          string
		status, calls int
		mutate        func(*materialUseCases, *auth.Principal)
		anonymous     bool
	}{
		{name: "anonymous", status: 401, anonymous: true},
		{name: "amount does not grant read", status: 403, mutate: func(_ *materialUseCases, p *auth.Principal) {
			p.AllowedActions = map[string]bool{auth.ActionAmountRead: true}
		}},
		{name: "neighbor result", status: 403, calls: 1, mutate: func(_ *materialUseCases, p *auth.Principal) {
			p.Resources = map[string]map[string]bool{"approval_audit_result": {"55555555-5555-4555-8555-555555555555": true}}
		}},
		{name: "neighbor project", status: 403, calls: 1, mutate: func(_ *materialUseCases, p *auth.Principal) {
			p.Resources = map[string]map[string]bool{"project": {"55555555-5555-4555-8555-555555555555": true}}
		}},
		{name: "not found", status: 404, calls: 1, mutate: func(u *materialUseCases, _ *auth.Principal) { u.err = results.ErrNotFound }},
		{name: "bad store tenant", status: 422, calls: 1, mutate: func(u *materialUseCases, _ *auth.Principal) {
			u.value.TenantID = "66666666-6666-4666-8666-666666666666"
		}},
		{name: "bad store id", status: 422, calls: 1, mutate: func(u *materialUseCases, _ *auth.Principal) { u.value.ID = "66666666-6666-4666-8666-666666666666" }},
		{name: "bad stored evidence", status: 422, calls: 1, mutate: func(u *materialUseCases, _ *auth.Principal) { u.value.Evidence[0].Version = 0 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			useCases := materialHTTPFixture()
			principal := unitPrincipal(auth.ActionRead)
			if test.mutate != nil {
				test.mutate(useCases, &principal)
			}
			var authenticator auth.Authenticator = &unitAuthenticator{principal: principal}
			if test.anonymous {
				authenticator = &materialUnauthenticated{}
			}
			server, err := New(useCases, authenticator)
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/references/approval-audit-results/"+materialResultID+"/materials", nil))
			if response.Code != test.status || useCases.getCalls != test.calls {
				t.Fatalf("status=%d calls=%d body=%s", response.Code, useCases.getCalls, response.Body.String())
			}
		})
	}
}
