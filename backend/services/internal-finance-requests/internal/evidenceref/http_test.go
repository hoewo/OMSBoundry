package evidenceref

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"omsbizfoundry/internal-finance-requests/internal/finance"
)

func TestResolveUsesExactImmutableInvoiceVersion(t *testing.T) {
	const tenant = "11111111-1111-4111-8111-111111111111"
	const fileID = "22222222-2222-4222-8222-222222222222"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/references/files/"+fileID+"/versions/2" || r.Header.Get("Authorization") != "Bearer this-is-a-valid-template-token" {
			t.Fatalf("unexpected request %s auth=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"schema_version": "1", "source_application": "evidence-service", "resource_type": "evidence-file", "id": fileID, "file_id": fileID, "tenant_id": tenant, "status": "available", "version": 2, "revision": 2, "display_name": "invoice.pdf", "display_summary": "invoice.pdf", "file_name": "invoice.pdf", "size_bytes": 128, "media_type": "application/pdf", "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "validation_profile": "invoice-v1", "security_status": "passed"})
	}))
	defer server.Close()
	client, err := New(server.URL, "this-is-a-valid-template-token", nil, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	value, err := client.Resolve(context.Background(), tenant, finance.EvidenceRef{FileID: fileID, Version: 2, Purpose: "发票"})
	if err != nil || value.ValidationProfile != "invoice-v1" || value.SecurityStatus != "passed" || value.Version != 2 {
		t.Fatalf("value=%#v err=%v", value, err)
	}
}

func TestResolveRejectsCrossTenantReference(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"schema_version": "1", "source_application": "evidence-service", "resource_type": "evidence-file", "id": "22222222-2222-4222-8222-222222222222", "file_id": "22222222-2222-4222-8222-222222222222", "tenant_id": "99999999-9999-4999-8999-999999999999", "status": "available", "version": 1, "revision": 1, "display_name": "invoice.pdf", "display_summary": "invoice.pdf", "file_name": "invoice.pdf", "size_bytes": 128, "media_type": "application/pdf", "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "validation_profile": "invoice-v1", "security_status": "passed"})
	}))
	defer server.Close()
	client, _ := New(server.URL, "this-is-a-valid-template-token", nil, time.Second)
	_, err := client.Resolve(context.Background(), "11111111-1111-4111-8111-111111111111", finance.EvidenceRef{FileID: "22222222-2222-4222-8222-222222222222", Version: 1, Purpose: "发票"})
	if err == nil {
		t.Fatal("cross-tenant Evidence reference was accepted")
	}
}

func TestResolveRefreshesExactWorkloadCredentialOnce(t *testing.T) {
	const tenant = "11111111-1111-4111-8111-111111111111"
	const fileID = "22222222-2222-4222-8222-222222222222"
	credentials := &credentialSpy{tokens: []string{"expired", "fresh"}}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Authorization") != "Bearer fresh" {
			t.Fatalf("unexpected refreshed token %q", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"schema_version": "1", "source_application": "evidence-service", "resource_type": "evidence-file", "id": fileID, "file_id": fileID, "tenant_id": tenant, "status": "available", "version": 3, "revision": 3, "display_name": "invoice.pdf", "display_summary": "invoice.pdf", "file_name": "invoice.pdf", "size_bytes": 128, "media_type": "application/pdf", "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "validation_profile": "invoice-v1", "security_status": "passed"})
	}))
	defer server.Close()
	client, err := New(server.URL, "", credentials, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.Resolve(context.Background(), tenant, finance.EvidenceRef{FileID: fileID, Version: 3, Purpose: "发票"}); err != nil {
		t.Fatal(err)
	}
	want := tenant + "|evidence-service|evidence-service.metadata.read|file|" + fileID
	if requests != 2 || credentials.invalidations != 1 || len(credentials.calls) != 2 {
		t.Fatalf("requests=%d calls=%d invalidations=%d", requests, len(credentials.calls), credentials.invalidations)
	}
	for _, call := range credentials.calls {
		if call != want {
			t.Fatalf("unexpected credential scope %q", call)
		}
	}
}

type credentialSpy struct {
	tokens        []string
	calls         []string
	invalidations int
}

func (s *credentialSpy) CredentialForTenant(_ context.Context, tenantID, audience, action, resourceType, resourceID string) (string, error) {
	s.calls = append(s.calls, tenantID+"|"+audience+"|"+action+"|"+resourceType+"|"+resourceID)
	token := s.tokens[0]
	s.tokens = s.tokens[1:]
	return token, nil
}

func (s *credentialSpy) InvalidateForTenant(_, _, _, _, _ string) { s.invalidations++ }
