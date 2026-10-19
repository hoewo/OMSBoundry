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

	"omsbizfoundry/approval-audit-results/internal/results"
)

const (
	testTenant   = "11111111-1111-4111-8111-111111111111"
	testProject  = "22222222-2222-4222-8222-222222222222"
	testContract = "33333333-3333-4333-8333-333333333333"
	testFile     = "44444444-4444-4444-8444-444444444444"
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

func TestHTTPResolverValidatesExactProjectLinkAmountsAndEvidence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fresh-reference-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/v1/references/projects/" + testProject:
			_, _ = fmt.Fprintf(w, `{"id":%q,"tenant_id":%q,"resource_type":"project","project_number":"P-001","display_summary":"道路设计项目","status":"active","revision":7,"resource_links":[{"target_service":"contract-ledger","resource_type":"contract","resource_id":%q,"source_revision":4,"relation_key":"primary-contract"}]}`, testProject, testTenant, testContract)
		case "/api/v1/references/contracts/" + testContract:
			_, _ = fmt.Fprintf(w, `{"schema_version":"1","source_application":"contract-ledger","id":%q,"tenant_id":%q,"resource_type":"contract","display_summary":"设计合同","status":"active","revision":4,"category":"design_fee","currency":"CNY","current_effective_amount":"123.450000"}`, testContract, testTenant)
		case "/v1/references/files/" + testFile + "/versions/2":
			_, _ = fmt.Fprintf(w, `{"id":%q,"file_id":%q,"tenant_id":%q,"resource_type":"evidence-file","status":"available","version":2,"revision":2,"display_name":"批复.pdf","sha256":"%s"}`, testFile, testFile, testTenant, strings.Repeat("a", 64))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	credentials := &testCredentials{}
	endpoint := func(audience, action string) Endpoint {
		return Endpoint{BaseURL: server.URL, Audience: audience, Action: action, Credentials: credentials}
	}
	resolver, err := NewHTTPResolver(time.Second, endpoint("project-workspace", "project-workspace.read"), endpoint("contract-ledger", "contract-ledger.reference.read"), endpoint("evidence-service", "evidence-service.view"))
	if err != nil {
		t.Fatal(err)
	}
	project, err := resolver.ResolveProject(context.Background(), testTenant, results.ProjectRef{ID: testProject, Revision: 7})
	if err != nil || project.Code != "P-001" || project.DisplayName != "道路设计项目" {
		t.Fatalf("project=%#v err=%v", project, err)
	}
	sourceRef := results.SourceRef{Application: "contract-ledger", ResourceType: "contract", ResourceID: testContract, Revision: 4}
	source, err := resolver.ResolveSource(context.Background(), testTenant, results.ProjectRef{ID: testProject, Revision: 7}, sourceRef)
	if err != nil || source.ProjectID != testProject || source.TotalAmountMinor != 12345 || len(source.AmountComponents) != 3 || source.AmountComponents[0].AmountMinor != 12345 || !strings.HasPrefix(source.Digest, "sha256:") {
		t.Fatalf("source=%#v err=%v", source, err)
	}
	evidence, err := resolver.ResolveEvidence(context.Background(), testTenant, results.EvidenceRef{FileID: testFile, Version: 2, Slot: "approval_document"})
	if err != nil || evidence.Digest != "sha256:"+strings.Repeat("a", 64) || evidence.DisplayName != "批复.pdf" {
		t.Fatalf("evidence=%#v err=%v", evidence, err)
	}
	joined := strings.Join(credentials.calls, "\n")
	for _, want := range []string{"project-workspace:project-workspace.read:project:" + testProject, "contract-ledger:contract-ledger.reference.read:contract:" + testContract, "evidence-service:evidence-service.view:file:" + testFile} {
		if !strings.Contains(joined, want) {
			t.Fatalf("credential calls=%s missing %s", joined, want)
		}
	}
}

func TestHTTPResolverRejectsUnlinkedSourceAndSubCentAmount(t *testing.T) {
	amount := "12.345"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/projects/"):
			_, _ = fmt.Fprintf(w, `{"id":%q,"tenant_id":%q,"resource_type":"project","project_number":"P-001","display_summary":"项目","status":"active","revision":7,"resource_links":[]}`, testProject, testTenant)
		case strings.Contains(r.URL.Path, "/contracts/"):
			_, _ = fmt.Fprintf(w, `{"schema_version":"1","source_application":"contract-ledger","id":%q,"tenant_id":%q,"resource_type":"contract","display_summary":"设计合同","status":"active","revision":4,"category":"design_fee","currency":"CNY","current_effective_amount":%q}`, testContract, testTenant, amount)
		}
	}))
	defer server.Close()
	resolver, _ := NewHTTPResolver(time.Second, Endpoint{BaseURL: server.URL, AccessToken: "static-token-123456789012345"}, Endpoint{BaseURL: server.URL, AccessToken: "static-token-123456789012345"}, Endpoint{BaseURL: server.URL, AccessToken: "static-token-123456789012345"})
	ref := results.SourceRef{Application: "contract-ledger", ResourceType: "contract", ResourceID: testContract, Revision: 4}
	if _, err := resolver.ResolveSource(context.Background(), testTenant, results.ProjectRef{ID: testProject, Revision: 7}, ref); !errors.Is(err, results.ErrReferenceInvalid) {
		t.Fatalf("unlinked err=%v", err)
	}
}

func TestHTTPResolverRefreshesCredentialOnceAfterUnauthorized(t *testing.T) {
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
	credentials := &testCredentials{first: "expired-reference-token"}
	endpoint := Endpoint{BaseURL: server.URL, Audience: "project-workspace", Action: "project-workspace.read", Credentials: credentials}
	resolver, _ := NewHTTPResolver(time.Second, endpoint, endpoint, endpoint)
	if _, err := resolver.ResolveProject(context.Background(), testTenant, results.ProjectRef{ID: testProject, Revision: 7}); err != nil {
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
	}{{"0", 0, true}, {"12.3", 1230, true}, {"12.340000", 1234, true}, {"12.345", 0, false}, {"1e3", 0, false}} {
		got, err := decimalToMinor(test.raw)
		if (err == nil) != test.ok || (err == nil && got != test.want) {
			t.Fatalf("%q got=%d err=%v", test.raw, got, err)
		}
	}
}
