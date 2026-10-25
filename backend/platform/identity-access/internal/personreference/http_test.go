package personreference

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"omsbizfoundry/identity-access/internal/identity"
)

const (
	tenantID  = "22222222-2222-4222-8222-222222222222"
	subjectID = "11111111-1111-4111-8111-111111111111"
	personID  = "33333333-3333-4333-8333-333333333333"
)

type captureSigner struct{ claims identity.CapabilityClaims }

func (s *captureSigner) Sign(value any) (string, error) {
	s.claims = value.(identity.CapabilityClaims)
	return "signed-service-credential", nil
}

func TestResolverUsesBoundedPeopleReferenceContract(t *testing.T) {
	signer := &captureSigner{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/references/iam-subjects/"+subjectID+"/person" || r.Header.Get("Authorization") != "Bearer signed-service-credential" {
			t.Fatalf("unexpected request: %s %s %#v", r.Method, r.URL.Path, r.Header)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"schema_version": "1", "source_application": "people-groups", "resource_type": "person", "id": personID, "tenant_id": tenantID, "status": "active", "revision": 7})
	}))
	defer server.Close()
	resolver, err := New(server.URL, subjectID, "https://oms.example.test/auth", signer, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 25, 8, 0, 0, 0, time.UTC)
	resolver.now = func() time.Time { return now }
	value, err := resolver.ResolveBySubject(context.Background(), tenantID, subjectID)
	if err != nil {
		t.Fatal(err)
	}
	if value.ID != personID || value.TenantID != tenantID || value.Status != "active" || value.Revision != 7 {
		t.Fatalf("reference = %#v", value)
	}
	if signer.claims.Audience != "people-groups" || signer.claims.Tenant != tenantID || len(signer.claims.Actions) != 1 || signer.claims.Actions[0] != "people-groups.person.reference.read" || len(signer.claims.Scope.Resources) != 1 || signer.claims.Scope.Resources[0] != (identity.ResourceRef{Type: "iam_subject", ID: subjectID}) || signer.claims.ExpiresAt-signer.claims.IssuedAt != 60 {
		t.Fatalf("claims = %#v", signer.claims)
	}
}

func TestResolverRejectsInconsistentReference(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"schema_version": "1", "source_application": "people-groups", "resource_type": "person", "id": personID, "tenant_id": "44444444-4444-4444-8444-444444444444", "status": "active", "revision": 1})
	}))
	defer server.Close()
	resolver, err := New(server.URL, subjectID, "https://oms.example.test/auth", &captureSigner{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.ResolveBySubject(context.Background(), tenantID, subjectID); err == nil {
		t.Fatal("cross-tenant Person reference was accepted")
	}
}
