package references

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"omsbizfoundry/billing-invoice/internal/invoice"
)

const (
	tenantID     = "11111111-1111-4111-8111-111111111111"
	partyID      = "22222222-2222-4222-8222-222222222222"
	projectID    = "33333333-3333-4333-8333-333333333333"
	contractID   = "44444444-4444-4444-8444-444444444444"
	settlementID = "55555555-5555-4555-8555-555555555555"
	obligationID = "66666666-6666-4666-8666-666666666666"
	fileID       = "77777777-7777-4777-8777-777777777777"
)

type credentialCall struct {
	audience, action, resourceType, resourceID string
}

type credentialSpy struct {
	calls       []credentialCall
	invalidated []credentialCall
	fail        error
	staleFirst  bool
}

func (s *credentialSpy) Credential(_ context.Context, audience, action, resourceType, resourceID string) (string, error) {
	call := credentialCall{audience, action, resourceType, resourceID}
	s.calls = append(s.calls, call)
	if s.fail != nil {
		return "", s.fail
	}
	if s.staleFirst && len(s.calls) == 1 {
		return "stale", nil
	}
	return "fresh", nil
}

func (s *credentialSpy) Invalidate(audience, action, resourceType, resourceID string) {
	s.invalidated = append(s.invalidated, credentialCall{audience, action, resourceType, resourceID})
}

func TestResolverUsesOneExactCredentialPerReferenceType(t *testing.T) {
	credentials := &credentialSpy{}
	server := httptest.NewServer(referenceHandler(t))
	defer server.Close()
	resolver := dynamicResolver(t, server.URL, credentials)

	if _, err := resolver.ResolveParty(context.Background(), tenantID, invoice.PartyRef{ID: partyID, Revision: 3}); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		kind     string
		id       string
		revision int64
	}{
		{invoice.SourceProject, projectID, 4},
		{invoice.SourceContract, contractID, 5},
		{invoice.SourceSettlement, settlementID, 1},
		{invoice.SourceMoneyObligation, obligationID, 6},
	}
	for _, test := range tests {
		id, revision := test.id, test.revision
		if _, err := resolver.ResolveSource(context.Background(), tenantID, invoice.SourceRef{Kind: test.kind, ResourceID: &id, SourceRevision: &revision}); err != nil {
			t.Fatalf("%s: %v", test.kind, err)
		}
	}
	if _, err := resolver.ResolveEvidence(context.Background(), tenantID, invoice.EvidenceRef{FileID: fileID, Version: 2}); err != nil {
		t.Fatal(err)
	}

	want := []credentialCall{
		{"party-directory", "party-directory.reference.read", "party", partyID},
		{"project-workspace", "project-workspace.read", "project", projectID},
		{"contract-ledger", "contract-ledger.reference.read", "contract", contractID},
		{"contract-ledger", "contract-ledger.reference.read", "settlement", settlementID},
		{"money-flow-ledger", "money-flow-ledger.reference.read", "obligation", obligationID},
		{"evidence-service", "evidence-service.metadata.read", "file", fileID},
	}
	if fmt.Sprint(credentials.calls) != fmt.Sprint(want) {
		t.Fatalf("credential calls = %#v, want %#v", credentials.calls, want)
	}
}

func TestResolverInvalidatesAndRetriesExactlyOnceAfterUnauthorized(t *testing.T) {
	credentials := &credentialSpy{staleFirst: true}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") == "Bearer stale" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = fmt.Fprintf(w, `{"schema_version":"1","source_application":"party-directory","id":%q,"tenant_id":%q,"kind":"organization","legal_name":"甲方","status":"active","revision":3}`, partyID, tenantID)
	}))
	defer server.Close()
	resolver := dynamicResolver(t, server.URL, credentials)

	if _, err := resolver.ResolveParty(context.Background(), tenantID, invoice.PartyRef{ID: partyID, Revision: 3}); err != nil {
		t.Fatal(err)
	}
	if requests != 2 || len(credentials.calls) != 2 || len(credentials.invalidated) != 1 {
		t.Fatalf("requests=%d calls=%#v invalidated=%#v", requests, credentials.calls, credentials.invalidated)
	}
	want := credentialCall{"party-directory", "party-directory.reference.read", "party", partyID}
	if credentials.invalidated[0] != want {
		t.Fatalf("invalidated=%#v want=%#v", credentials.invalidated[0], want)
	}
}

func TestResolverRejectsCrossTenantStaleWrongTypeAndUnavailableStatus(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"cross tenant", `{"id":"` + projectID + `","tenant_id":"99999999-9999-4999-8999-999999999999","resource_type":"project","display_summary":"项目 A","status":"active","revision":4}`},
		{"stale revision", `{"id":"` + projectID + `","tenant_id":"` + tenantID + `","resource_type":"project","display_summary":"项目 A","status":"active","revision":3}`},
		{"wrong type", `{"id":"` + projectID + `","tenant_id":"` + tenantID + `","resource_type":"contract","display_summary":"项目 A","status":"active","revision":4}`},
		{"unavailable status", `{"id":"` + projectID + `","tenant_id":"` + tenantID + `","resource_type":"project","display_summary":"项目 A","status":"cancelled","revision":4}`},
		{"blank summary", `{"id":"` + projectID + `","tenant_id":"` + tenantID + `","resource_type":"project","display_summary":" ","status":"active","revision":4}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(test.body)) }))
			defer server.Close()
			resolver := staticResolver(t, server.URL)
			id, revision := projectID, int64(4)
			_, err := resolver.ResolveSource(context.Background(), tenantID, invoice.SourceRef{Kind: invoice.SourceProject, ResourceID: &id, SourceRevision: &revision})
			if !errors.Is(err, invoice.ErrReferenceInvalid) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestResolverRejectsInactivePartyAndUnavailableEvidence(t *testing.T) {
	t.Run("inactive party", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprintf(w, `{"schema_version":"1","source_application":"party-directory","id":%q,"tenant_id":%q,"kind":"organization","legal_name":"甲方","status":"inactive","revision":3}`, partyID, tenantID)
		}))
		defer server.Close()
		resolver := staticResolver(t, server.URL)
		_, err := resolver.ResolveParty(context.Background(), tenantID, invoice.PartyRef{ID: partyID, Revision: 3})
		if !errors.Is(err, invoice.ErrReferenceInvalid) {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("evidence not available", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprintf(w, `{"id":%q,"file_id":%q,"tenant_id":%q,"resource_type":"evidence-file","status":"quarantined","version":2,"revision":2,"display_name":"发票.pdf"}`, fileID, fileID, tenantID)
		}))
		defer server.Close()
		resolver := staticResolver(t, server.URL)
		_, err := resolver.ResolveEvidence(context.Background(), tenantID, invoice.EvidenceRef{FileID: fileID, Version: 2})
		if !errors.Is(err, invoice.ErrReferenceInvalid) {
			t.Fatalf("error=%v", err)
		}
	})
}

func TestResolverDistinguishesInvalidReferenceFromUnavailableDependency(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"missing", http.StatusNotFound, "", invoice.ErrReferenceInvalid},
		{"unprocessable", http.StatusUnprocessableEntity, "", invoice.ErrReferenceInvalid},
		{"forbidden", http.StatusForbidden, "", invoice.ErrReferenceUnavailable},
		{"upstream failure", http.StatusServiceUnavailable, "", invoice.ErrReferenceUnavailable},
		{"malformed", http.StatusOK, "{", invoice.ErrReferenceUnavailable},
		{"trailing JSON", http.StatusOK, `{}` + "\n" + `{}`, invoice.ErrReferenceUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			resolver := staticResolver(t, server.URL)
			_, err := resolver.ResolveParty(context.Background(), tenantID, invoice.PartyRef{ID: partyID, Revision: 3})
			if !errors.Is(err, test.want) {
				t.Fatalf("error=%v want=%v", err, test.want)
			}
		})
	}
}

func TestCredentialExchangeFailureFailsClosed(t *testing.T) {
	server := httptest.NewServer(referenceHandler(t))
	defer server.Close()
	resolver := dynamicResolver(t, server.URL, &credentialSpy{fail: errors.New("IAM unavailable")})
	_, err := resolver.ResolveParty(context.Background(), tenantID, invoice.PartyRef{ID: partyID, Revision: 3})
	if !errors.Is(err, invoice.ErrReferenceUnavailable) {
		t.Fatalf("error=%v", err)
	}
}

func referenceHandler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Errorf("missing bearer credential")
		}
		switch r.URL.Path {
		case "/api/v1/references/parties/" + partyID:
			_, _ = fmt.Fprintf(w, `{"schema_version":"1","source_application":"party-directory","id":%q,"tenant_id":%q,"kind":"organization","legal_name":"甲方","status":"active","revision":3}`, partyID, tenantID)
		case "/api/v1/references/projects/" + projectID:
			_, _ = fmt.Fprintf(w, `{"id":%q,"tenant_id":%q,"resource_type":"project","display_summary":"项目 A","status":"active","revision":4}`, projectID, tenantID)
		case "/api/v1/references/contracts/" + contractID:
			_, _ = fmt.Fprintf(w, `{"id":%q,"tenant_id":%q,"resource_type":"contract","display_summary":"设计合同","status":"active","revision":5}`, contractID, tenantID)
		case "/api/v1/references/settlements/" + settlementID:
			_, _ = fmt.Fprintf(w, `{"id":%q,"tenant_id":%q,"resource_type":"settlement","display_summary":"结算单","status":"recorded","revision":1}`, settlementID, tenantID)
		case "/api/v1/references/obligations/" + obligationID:
			_, _ = fmt.Fprintf(w, `{"id":%q,"tenant_id":%q,"resource_type":"obligation","display_summary":"应收义务","status":"open","revision":6}`, obligationID, tenantID)
		case "/v1/references/files/" + fileID + "/versions/2":
			_, _ = fmt.Fprintf(w, `{"id":%q,"file_id":%q,"tenant_id":%q,"resource_type":"evidence-file","status":"available","version":2,"revision":2,"display_name":"发票.pdf"}`, fileID, fileID, tenantID)
		default:
			http.NotFound(w, r)
		}
	})
}

func dynamicResolver(t *testing.T, baseURL string, credentials CredentialSource) *HTTPResolver {
	t.Helper()
	resolver, err := NewHTTPResolver(time.Second,
		Endpoint{BaseURL: baseURL, Audience: "party-directory", Action: "party-directory.reference.read", Credentials: credentials},
		Endpoint{BaseURL: baseURL, Audience: "project-workspace", Action: "project-workspace.read", Credentials: credentials},
		Endpoint{BaseURL: baseURL, Audience: "contract-ledger", Action: "contract-ledger.reference.read", Credentials: credentials},
		Endpoint{BaseURL: baseURL, Audience: "money-flow-ledger", Action: "money-flow-ledger.reference.read", Credentials: credentials},
		Endpoint{BaseURL: baseURL, Audience: "evidence-service", Action: "evidence-service.metadata.read", Credentials: credentials},
	)
	if err != nil {
		t.Fatal(err)
	}
	return resolver
}

func staticResolver(t *testing.T, baseURL string) *HTTPResolver {
	t.Helper()
	endpoint := Endpoint{BaseURL: baseURL, AccessToken: "local-template-reference-token"}
	resolver, err := NewHTTPResolver(time.Second, endpoint, endpoint, endpoint, endpoint, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	return resolver
}
