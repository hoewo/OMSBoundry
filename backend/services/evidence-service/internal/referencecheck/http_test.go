package referencecheck

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientRequiresMatchingProtectedOwnerResponse(t *testing.T) {
	const tenant = "11111111-1111-4111-8111-111111111111"
	const fileID = "22222222-2222-4222-8222-222222222222"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/references/evidence-files/"+fileID+"/versions/1/usage" || r.Header.Get("Authorization") != "Bearer test-reference-token-long-enough" {
			t.Fatalf("request path=%q authorization=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"tenant_id": tenant, "file_id": fileID, "version": 1, "referenced": true})
	}))
	defer server.Close()
	client, err := New(server.URL, "test-reference-token-long-enough", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	referenced, err := client.IsReferenced(t.Context(), tenant, fileID, 1)
	if err != nil || !referenced {
		t.Fatalf("referenced=%v err=%v", referenced, err)
	}
}

func TestClientFailsClosedOnTenantMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"tenant_id": "99999999-9999-4999-8999-999999999999", "file_id": "22222222-2222-4222-8222-222222222222", "version": 1, "referenced": false})
	}))
	defer server.Close()
	client, err := New(server.URL, "test-reference-token-long-enough", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.IsReferenced(t.Context(), "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", 1); err == nil {
		t.Fatal("tenant mismatch was accepted")
	}
}
