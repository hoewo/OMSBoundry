package gateway

import (
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"omsbizfoundry/gateway/internal/credential"
	"omsbizfoundry/gateway/internal/manifest"
)

// This is Gateway HTTP wiring against a final-authorizing stub, not a claim
// that the real Evidence or Nginx/IAM integration has run.
func TestEvidenceAnyAdmissionDoesNotGrantTargetActionsOrScope(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	verifier := credential.New(writeKeys(t, public), "https://oms.example.test/auth", testTenant)
	file := "55555555-5555-4555-8555-555555555555"
	issue := func(actions []string, id, tenant string) string {
		now := time.Now().UTC()
		return signToken(t, private, map[string]any{"ver": "1", "iss": "https://oms.example.test/auth", "sub": testSubject, "tid": tenant, "aud": "evidence-service", "name": "Fixture", "actions": actions, "scope": map[string]any{"kind": "resources", "resources": []map[string]string{{"type": "file", "id": id}}}, "iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(time.Minute).Unix(), "auth_time": now.Add(-time.Minute).Unix(), "jti": "44444444-4444-4444-8444-444444444444"})
	}
	var requests atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("X-OMS-Subject-ID") != "" || r.Header.Get("X-OMS-Forged") != "" {
			t.Error("forged context forwarded")
		}
		action := "evidence-service.metadata.read"
		if strings.HasSuffix(r.URL.Path, "/download") {
			action = "evidence-service.download"
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		claims, err := verifier.Verify(token, "evidence-service", action)
		if err != nil || claims.Scope.Kind != "resources" || claims.Scope.Resources[0].ID != file {
			w.WriteHeader(403)
			return
		}
		// Read-only response lets the test prove the original signature/actions survived.
		w.Header().Set("X-Test-Original-Token", token)
		w.WriteHeader(200)
	}))
	defer target.Close()
	var issued string
	iam := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/credentials" {
			writeJSON(w, 200, map[string]any{"token": issued, "token_type": "Bearer", "expires_at": time.Now().Add(5 * time.Minute), "claims": map[string]any{"aud": "evidence-service"}})
			return
		}
		if r.URL.Path == "/api/v1/logout" {
			w.WriteHeader(204)
			return
		}
		http.NotFound(w, r)
	}))
	defer iam.Close()
	suite := manifest.Suite{SchemaVersion: "1", SuiteID: "test-suite", TenantID: testTenant, Applications: []manifest.Application{
		{ID: "identity-access", Audience: "identity-access", Enabled: true, Required: true, RoutePrefix: "/auth", Upstream: iam.URL},
		{ID: "evidence-service", Audience: "evidence-service", Enabled: true, RoutePrefix: "/evidence", Upstream: target.URL, EntryActions: []string{"evidence-service.view", "evidence-service.metadata.read", "evidence-service.upload", "evidence-service.download"}},
	}}
	handler, err := New(suite, verifier, slog.New(slog.NewTextHandler(io.Discard, nil)), Options{RequestsPerMinute: 1000})
	if err != nil {
		t.Fatal(err)
	}
	read := func(token, path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "http://suite.test/evidence/api/v1/files/"+file+path, nil)
		if token != "" {
			r.AddCookie(&http.Cookie{Name: capabilityCookieName("evidence-service"), Value: token})
		}
		r.Header.Set("Authorization", "Bearer forged")
		r.Header.Set("X-OMS-Subject-ID", testSubject)
		r.Header.Set("X-OMS-Forged", "true")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	cases := []struct {
		name               string
		actions            []string
		id, tenant         string
		metadata, download int
	}{
		{"metadata only", []string{"evidence-service.metadata.read"}, file, testTenant, 200, 403},
		{"download only", []string{"evidence-service.download"}, file, testTenant, 403, 200},
		{"metadata and download without view", []string{"evidence-service.metadata.read", "evidence-service.download"}, file, testTenant, 200, 200},
		{"neighbor file", []string{"evidence-service.metadata.read", "evidence-service.download"}, "66666666-6666-4666-8666-666666666666", testTenant, 403, 403},
		{"wrong tenant", []string{"evidence-service.download"}, file, "77777777-7777-4777-8777-777777777777", 303, 303},
		{"unmatched action", []string{"evidence-service.reference.read"}, file, testTenant, 303, 303},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			token := issue(test.actions, test.id, test.tenant)
			for _, check := range []struct {
				path   string
				status int
			}{{"", test.metadata}, {"/download", test.download}} {
				before := requests.Load()
				w := read(token, check.path)
				if w.Code != check.status {
					t.Fatalf("%s: got %d want %d", check.path, w.Code, check.status)
				}
				if check.status == 200 && w.Header().Get("X-Test-Original-Token") != token {
					t.Fatal("Gateway rewrote token")
				}
				if check.status == 303 && requests.Load() != before {
					t.Fatal("rejected credential reached target")
				}
			}
		})
	}
	if w := read("", "/download"); w.Code != 303 {
		t.Fatal("forged headers bypassed cookie admission")
	}
	post := func(path string) *httptest.ResponseRecorder {
		form := url.Values{"csrf_token": {"test-csrf"}}
		r := httptest.NewRequest("POST", "http://suite.test"+path, strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "test-csrf"})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	issued = issue([]string{"evidence-service.download"}, file, testTenant)
	authorized := post("/gateway/authorize/evidence-service")
	if authorized.Code != 303 {
		t.Fatal("download-only credential not admitted", authorized.Code)
	}
	cookies := authorized.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Path != "/evidence" || !cookies[0].HttpOnly || cookies[0].Value != issued || cookies[0].MaxAge > 60 {
		t.Fatal("bounded path cookie changed", cookies)
	}
	issued = issue([]string{"evidence-service.reference.read"}, file, testTenant)
	denied := post("/gateway/authorize/evidence-service")
	if denied.Code != 403 || len(denied.Result().Cookies()) != 1 || denied.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("invalid admission did not clear old cookie")
	}
	loggedOut := post("/gateway/logout")
	if loggedOut.Code != 303 || len(loggedOut.Result().Cookies()) != 1 || loggedOut.Result().Cookies()[0].Name != capabilityCookieName("evidence-service") || loggedOut.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("ANY entry cookie survived logout")
	}
}
