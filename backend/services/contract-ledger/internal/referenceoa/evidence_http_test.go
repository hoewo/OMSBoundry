package referenceoa

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type credentialFixture struct {
	calls, invalidations int
	t                    *testing.T
}

func (c *credentialFixture) Credential(_ context.Context, audience, action, kind, id string) (string, error) {
	if audience != "evidence-service" || action != "evidence-service.metadata.read" || kind != "file" || id != "33333333-3333-4333-8333-333333333333" {
		c.t.Fatal("scope expanded")
	}
	c.calls++
	return "fixture-short-token", nil
}
func (c *credentialFixture) Invalidate(a, b, d, e string) { c.invalidations++ }
func TestExternalEvidenceExactScopeRefreshAndFailClosed(t *testing.T) {
	tenant := "22222222-2222-4222-8222-222222222222"
	id := "33333333-3333-4333-8333-333333333333"
	calls := 0
	status := 200
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" || r.URL.Path != "/v1/references/files/"+id+"/versions/2" || r.Header.Get("Authorization") != "Bearer fixture-short-token" {
			t.Error("unexpected file request")
		}
		if calls == 1 {
			w.WriteHeader(401)
			return
		}
		if status != 200 {
			w.WriteHeader(status)
			return
		}
		json.NewEncoder(w).Encode(FileReference{"1", "evidence-service", "evidence-file", id, id, tenant, 2, 2, "available", "合同.pdf", 5, "application/pdf", strings.Repeat("a", 64)})
	}))
	defer server.Close()
	credentials := &credentialFixture{t: t}
	v, e := NewHTTPFileValidator(server.URL, "", credentials, time.Second)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = v.ValidateFile(context.Background(), tenant, id, 2); e != nil || credentials.calls != 2 || credentials.invalidations != 1 {
		t.Fatalf("refresh failed: %v", e)
	}
	status = 503
	if _, e = v.ValidateFile(context.Background(), tenant, id, 2); e != ErrEvidenceUnavailable {
		t.Fatalf("503 = %v", e)
	}
	status = 403
	if _, e = v.ValidateFile(context.Background(), tenant, id, 2); e != ErrScope {
		t.Fatalf("403 = %v", e)
	}
	if _, e = NewHTTPFileValidator(server.URL, "static", credentials, time.Second); e == nil {
		t.Fatal("ambiguous credential source accepted")
	}
}
