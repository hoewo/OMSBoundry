package references

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"omsbizfoundry/cost-outsourcing/internal/costoutsourcing"
)

const (
	testTenant   = "11111111-1111-4111-8111-111111111111"
	testProject  = "22222222-2222-4222-8222-222222222222"
	testParty    = "33333333-3333-4333-8333-333333333333"
	testContract = "44444444-4444-4444-8444-444444444444"
	testFile     = "55555555-5555-4555-8555-555555555555"
)

type testCredentials struct {
	mu            sync.Mutex
	calls         []string
	invalidations int
	first         string
}

func (c *testCredentials) Credential(_ context.Context, audience, action, resourceType, resourceID string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, strings.Join([]string{audience, action, resourceType, resourceID}, ":"))
	if c.first != "" && len(c.calls) == 1 {
		return c.first, nil
	}
	return "fresh-reference-token", nil
}
func (c *testCredentials) Invalidate(string, string, string, string) {
	c.mu.Lock()
	c.invalidations++
	c.mu.Unlock()
}

func TestHTTPResolverValidatesExactProjectPartyContractAndEvidence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fresh-reference-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/v1/references/projects/" + testProject:
			_, _ = fmt.Fprintf(w, `{"id":%q,"tenant_id":%q,"resource_type":"project","project_number":"P-001","display_summary":"道路设计项目","status":"active","revision":7,"resource_links":[{"target_service":"contract-ledger","resource_type":"contract","resource_id":%q,"source_revision":9}]}`, testProject, testTenant, testContract)
		case "/api/v1/references/parties/" + testParty:
			_, _ = fmt.Fprintf(w, `{"schema_version":"1","source_application":"party-directory","id":%q,"tenant_id":%q,"kind":"organization","legal_name":"协作单位甲","status":"active","revision":5}`, testParty, testTenant)
		case "/api/v1/references/contracts/" + testContract:
			_, _ = fmt.Fprintf(w, `{"schema_version":"1","source_application":"contract-ledger","id":%q,"tenant_id":%q,"resource_type":"contract","display_summary":"测量协作合同","status":"active","revision":9,"currency":"CNY","current_effective_amount":"5000.00","party_ids":[%q]}`, testContract, testTenant, testParty)
		case "/v1/references/files/" + testFile + "/versions/2":
			_, _ = fmt.Fprintf(w, `{"id":%q,"file_id":%q,"tenant_id":%q,"resource_type":"evidence-file","status":"available","version":2,"revision":2,"display_name":"合同附件.pdf","sha256":"%s"}`, testFile, testFile, testTenant, strings.Repeat("a", 64))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	credentials := &testCredentials{}
	endpoint := func(audience, action string) Endpoint {
		return Endpoint{BaseURL: server.URL, Audience: audience, Action: action, Credentials: credentials}
	}
	resolver, err := NewHTTPResolver(time.Second, endpoint("project-workspace", "project-workspace.read"), endpoint("party-directory", "party-directory.reference.read"), endpoint("contract-ledger", "contract-ledger.reference.read"), endpoint("evidence-service", "evidence-service.metadata.read"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	projectRef := costoutsourcing.ProjectRef{ID: testProject, Revision: 7}
	partyRef := costoutsourcing.PartyRef{ID: testParty, Revision: 5}
	contractRef := costoutsourcing.ContractRef{Application: "contract-ledger", ResourceType: "contract", ResourceID: testContract, Revision: 9}
	if value, err := resolver.ResolveProject(ctx, testTenant, projectRef); err != nil || value.Code != "P-001" {
		t.Fatalf("project=%#v err=%v", value, err)
	}
	if value, err := resolver.ResolveParty(ctx, testTenant, partyRef); err != nil || value.LegalName != "协作单位甲" {
		t.Fatalf("party=%#v err=%v", value, err)
	}
	if value, err := resolver.ResolveContract(ctx, testTenant, projectRef, partyRef, contractRef); err != nil || value.ProjectID != testProject || value.PartyID != testParty || value.AmountMinor != 500000 || !strings.HasPrefix(value.Digest, "sha256:") {
		t.Fatalf("contract=%#v err=%v", value, err)
	}
	if value, err := resolver.ResolveEvidence(ctx, testTenant, costoutsourcing.EvidenceRef{FileID: testFile, Version: 2, Slot: "contract"}); err != nil || value.DisplayName != "合同附件.pdf" {
		t.Fatalf("evidence=%#v err=%v", value, err)
	}
	joined := strings.Join(credentials.calls, "\n")
	for _, wanted := range []string{"project-workspace:project-workspace.read:project:" + testProject, "party-directory:party-directory.reference.read:party:" + testParty, "contract-ledger:contract-ledger.reference.read:contract:" + testContract, "evidence-service:evidence-service.metadata.read:file:" + testFile} {
		if !strings.Contains(joined, wanted) {
			t.Fatalf("credential calls=%s missing=%s", joined, wanted)
		}
	}
}

func TestHTTPResolverRejectsMissingProjectLinkAndProvider(t *testing.T) {
	partyIDs := `[]`
	linked := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/projects/"):
			links := `[]`
			if linked {
				links = fmt.Sprintf(`[{"target_service":"contract-ledger","resource_type":"contract","resource_id":%q,"source_revision":9}]`, testContract)
			}
			_, _ = fmt.Fprintf(w, `{"id":%q,"tenant_id":%q,"resource_type":"project","project_number":"P-001","display_summary":"项目","status":"active","revision":7,"resource_links":%s}`, testProject, testTenant, links)
		case strings.Contains(r.URL.Path, "/contracts/"):
			_, _ = fmt.Fprintf(w, `{"schema_version":"1","source_application":"contract-ledger","id":%q,"tenant_id":%q,"resource_type":"contract","display_summary":"合同","status":"active","revision":9,"currency":"CNY","current_effective_amount":"1.00","party_ids":%s}`, testContract, testTenant, partyIDs)
		}
	}))
	defer server.Close()
	endpoint := Endpoint{BaseURL: server.URL, AccessToken: "static-token-123456789012345"}
	resolver, _ := NewHTTPResolver(time.Second, endpoint, endpoint, endpoint, endpoint)
	projectRef := costoutsourcing.ProjectRef{ID: testProject, Revision: 7}
	partyRef := costoutsourcing.PartyRef{ID: testParty, Revision: 5}
	contractRef := costoutsourcing.ContractRef{Application: "contract-ledger", ResourceType: "contract", ResourceID: testContract, Revision: 9}
	if _, err := resolver.ResolveContract(context.Background(), testTenant, projectRef, partyRef, contractRef); !errors.Is(err, costoutsourcing.ErrReferenceInvalid) {
		t.Fatalf("missing link err=%v", err)
	}
	linked = true
	if _, err := resolver.ResolveContract(context.Background(), testTenant, projectRef, partyRef, contractRef); !errors.Is(err, costoutsourcing.ErrReferenceInvalid) {
		t.Fatalf("missing provider err=%v", err)
	}
	partyIDs = fmt.Sprintf(`[%q]`, testParty)
	if value, err := resolver.ResolveContract(context.Background(), testTenant, projectRef, partyRef, contractRef); err != nil || value.PartyID != testParty {
		t.Fatalf("recovered=%#v err=%v", value, err)
	}
}

func TestHTTPResolverRefreshesCredentialOnce(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != "Bearer fresh-reference-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = fmt.Fprintf(w, `{"id":%q,"tenant_id":%q,"resource_type":"project","project_number":"P-001","display_summary":"项目","status":"active","revision":7,"resource_links":[]}`, testProject, testTenant)
	}))
	defer server.Close()
	credentials := &testCredentials{first: "expired-token"}
	endpoint := Endpoint{BaseURL: server.URL, Audience: "project-workspace", Action: "project-workspace.read", Credentials: credentials}
	resolver, _ := NewHTTPResolver(time.Second, endpoint, endpoint, endpoint, endpoint)
	if _, err := resolver.ResolveProject(context.Background(), testTenant, costoutsourcing.ProjectRef{ID: testProject, Revision: 7}); err != nil {
		t.Fatal(err)
	}
	if requests != 2 || credentials.invalidations != 1 {
		t.Fatalf("requests=%d invalidations=%d", requests, credentials.invalidations)
	}
}

func TestDecimalToMinorIsExact(t *testing.T) {
	for _, test := range []struct {
		raw  string
		want int64
		ok   bool
	}{{"0", 0, true}, {"12.3", 1230, true}, {"12.3400", 1234, true}, {"12.345", 0, false}, {"1e3", 0, false}} {
		got, err := decimalToMinor(test.raw)
		if (err == nil) != test.ok || (err == nil && got != test.want) {
			t.Fatalf("%q got=%d err=%v", test.raw, got, err)
		}
	}
}
