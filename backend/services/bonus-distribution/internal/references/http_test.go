package references

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"omsbizfoundry/bonus-distribution/internal/bonus"
)

const (
	testTenant  = "11111111-1111-4111-8111-111111111111"
	testProject = "22222222-2222-4222-8222-222222222222"
)

type credentialStub struct {
	mu            sync.Mutex
	calls         int
	invalidations int
}

func (c *credentialStub) Credential(context.Context, string, string, string, string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return "reference-token", nil
}

func (c *credentialStub) Invalidate(string, string, string, string) {
	c.mu.Lock()
	c.invalidations++
	c.mu.Unlock()
}

func TestHTTPResolverRetriesOneUnauthorizedAndClassifiesReferenceFailures(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/api/v1/references/projects/"+testProject {
			_, _ = fmt.Fprintf(w, `{"id":%q,"tenant_id":%q,"resource_type":"project","project_number":"P-1","display_summary":"项目一","status":"active","revision":2,"project_type_keys":[],"node_type":"project","resource_links":[]}`, testProject, testTenant)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	credentials := &credentialStub{}
	endpoint := Endpoint{BaseURL: server.URL, Audience: "project-workspace", Action: "project-workspace.read", Credentials: credentials}
	resolver, err := NewHTTPResolver(time.Second, endpoint, endpoint, endpoint, Endpoint{})
	if err != nil {
		t.Fatal(err)
	}
	value, err := resolver.ResolveProject(context.Background(), testTenant, bonus.ProjectRef{ID: testProject, Revision: 2})
	if err != nil || value.ID != testProject {
		t.Fatalf("retry result=%#v error=%v", value, err)
	}
	credentials.mu.Lock()
	calls, invalidations := credentials.calls, credentials.invalidations
	credentials.mu.Unlock()
	if requests != 2 || calls != 2 || invalidations != 1 {
		t.Fatalf("requests=%d credential calls=%d invalidations=%d", requests, calls, invalidations)
	}
	_, err = resolver.ResolveProject(context.Background(), testTenant, bonus.ProjectRef{ID: "33333333-3333-4333-8333-333333333333", Revision: 1})
	if !errors.Is(err, bonus.ErrReferenceInvalid) {
		t.Fatalf("404 must be invalid reference: %v", err)
	}
}

func TestHTTPResolverTimeoutIsRetryableUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(40 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	endpoint := Endpoint{BaseURL: server.URL, AccessToken: "static-reference-token-long-enough"}
	resolver, err := NewHTTPResolver(5*time.Millisecond, endpoint, endpoint, endpoint, Endpoint{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = resolver.ResolveProject(context.Background(), testTenant, bonus.ProjectRef{ID: testProject, Revision: 2})
	if !errors.Is(err, bonus.ErrReferenceUnavailable) {
		t.Fatalf("timeout must be unavailable: %v", err)
	}
}

func TestHTTPResolverBusinessSourceToleratesAdditiveSettlementFields(t *testing.T) {
	const settlementID = "44444444-4444-4444-8444-444444444444"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/references/projects/" + testProject:
			_, _ = fmt.Fprintf(w, `{"id":%q,"tenant_id":%q,"resource_type":"project","project_number":"P-1","display_summary":"项目一","status":"active","revision":2,"project_type_keys":[],"node_type":"project","resource_links":[{"target_service":"contract-ledger","resource_type":"settlement","resource_id":%q,"source_revision":1,"relation_key":"business-settlement"}]}`, testProject, testTenant, settlementID)
		case "/api/v1/references/settlements/" + settlementID:
			_, _ = fmt.Fprintf(w, `{"schema_version":"1","source_application":"contract-ledger","id":%q,"tenant_id":%q,"resource_type":"settlement","display_summary":"阶段结算","status":"recorded","revision":1,"category":"design","contract_id":"55555555-5555-4555-8555-555555555555","currency":"CNY","current_effective_amount":"50000","party_ids":["66666666-6666-4666-8666-666666666666"],"party_references":[{"id":"66666666-6666-4666-8666-666666666666","revision":1,"kind":"organization","legal_name":"客户","source":"directory"}]}`, settlementID, testTenant)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	endpoint := Endpoint{BaseURL: server.URL, AccessToken: "static-reference-token-long-enough"}
	resolver, err := NewHTTPResolver(time.Second, endpoint, endpoint, endpoint, Endpoint{})
	if err != nil {
		t.Fatal(err)
	}
	value, err := resolver.ResolveBusinessSource(context.Background(), testTenant, bonus.ProjectSnapshot{ID: testProject, Revision: 2}, bonus.BusinessSourceRef{SettlementID: settlementID, Revision: 1})
	if err != nil || value.ID != settlementID || value.TotalAmountMinor != 5_000_000 {
		t.Fatalf("business source=%#v error=%v", value, err)
	}
}
