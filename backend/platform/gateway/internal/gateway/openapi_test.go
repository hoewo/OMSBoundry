package gateway

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"omsbizfoundry/gateway/internal/credential"
	"omsbizfoundry/gateway/internal/manifest"
)

func TestOpenAPIBoundaryAuthenticationRateLimitAndAudit(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := writeKeys(t, publicKey)
	now := time.Now().UTC()
	credentialToken := signToken(t, privateKey, map[string]any{
		"ver": "1", "iss": "https://oms.example.test/auth", "sub": testSubject, "tid": testTenant,
		"aud": "open-api", "name": "API Key bfk_example000", "actions": []string{openAPIUseAction},
		"scope": map[string]any{"kind": "resources", "resources": []map[string]string{{"type": "api_key", "id": testKeyID}}},
		"iat":   now.Unix(), "nbf": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(), "auth_time": now.Unix(),
		"jti": "44444444-4444-4444-8444-444444444444",
	})
	var mutex sync.Mutex
	authCalls := 0
	usageResults := make([]map[string]any, 0)
	iam := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/internal/open-api/authenticate":
			mutex.Lock()
			authCalls++
			mutex.Unlock()
			if r.Header.Get("Authorization") != "ApiKey bfk_example000_secret-value-for-test" {
				writeProblem(w, http.StatusUnauthorized, "invalid_api_key", "invalid")
				return
			}
			var input map[string]string
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Fatal(err)
			}
			if input["action"] != "open-api.profile.read" || input["source_ip_masked"] != "198.51.100.0/24" {
				t.Errorf("authentication input = %#v", input)
			}
			writeJSON(w, http.StatusOK, map[string]any{"credential": credentialToken, "expires_at": now.Add(5 * time.Minute), "tenant_id": testTenant, "subject_id": testSubject, "person_id": "55555555-5555-4555-8555-555555555555", "key_id": testKeyID, "key_prefix": "bfk_example000", "request_id": input["request_id"]})
		case "/api/v1/internal/open-api/usage-result":
			if r.Header.Get("Authorization") != "Bearer "+credentialToken {
				t.Errorf("usage completion credential was not bounded")
			}
			var value map[string]any
			if err := json.NewDecoder(r.Body).Decode(&value); err != nil {
				t.Fatal(err)
			}
			mutex.Lock()
			usageResults = append(usageResults, value)
			mutex.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer iam.Close()
	workbench := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer workbench.Close()
	suite := manifest.Suite{SchemaVersion: "1", SuiteID: "open-api-test", TenantID: testTenant, Applications: []manifest.Application{
		{ID: "identity-access", Enabled: true, Required: true, RoutePrefix: "/auth", Upstream: iam.URL, Audience: "identity-access"},
		{ID: "unified-workbench", Enabled: true, Required: true, RoutePrefix: "/workbench", Upstream: workbench.URL, Audience: "unified-workbench"},
	}}
	handler, err := New(suite, credential.New(keyPath, "https://oms.example.test/auth"), slog.New(slog.NewTextHandler(io.Discard, nil)), Options{RequestsPerMinute: 100, TrustProxyClientIP: true, OpenAPIKeyRequestsPerMinute: 1, OpenAPITenantRequestsPerMinute: 2, OpenAPIRequireHTTPS: true})
	if err != nil {
		t.Fatal(err)
	}

	docs := httptest.NewRequest(http.MethodGet, "http://gateway.test/open-api/v1/docs", nil)
	docsResponse := httptest.NewRecorder()
	SecurityHeaders(handler).ServeHTTP(docsResponse, docs)
	if docsResponse.Code != http.StatusOK || !strings.Contains(docsResponse.Body.String(), "curl --fail") || strings.Contains(docsResponse.Body.String(), "bfk_example000_secret") {
		t.Fatalf("public docs response = %d %q", docsResponse.Code, docsResponse.Body.String())
	}

	missing := openAPIRequest(http.MethodGet, "/open-api/v1/me", "", true)
	missingResponse := httptest.NewRecorder()
	SecurityHeaders(handler).ServeHTTP(missingResponse, missing)
	assertOpenAPIProblem(t, missingResponse, http.StatusUnauthorized, "INVALID_API_KEY")

	valid := openAPIRequest(http.MethodGet, "/open-api/v1/me", "bfk_example000_secret-value-for-test", true)
	validResponse := httptest.NewRecorder()
	SecurityHeaders(handler).ServeHTTP(validResponse, valid)
	if validResponse.Code != http.StatusOK {
		t.Fatalf("valid API response = %d %s", validResponse.Code, validResponse.Body.String())
	}
	var profile map[string]any
	if err := json.Unmarshal(validResponse.Body.Bytes(), &profile); err != nil || profile["tenant_id"] != testTenant || profile["key_id"] != testKeyID {
		t.Fatalf("profile = %#v, %v", profile, err)
	}

	limited := openAPIRequest(http.MethodGet, "/open-api/v1/me", "bfk_example000_secret-value-for-test", true)
	limitedResponse := httptest.NewRecorder()
	SecurityHeaders(handler).ServeHTTP(limitedResponse, limited)
	assertOpenAPIProblem(t, limitedResponse, http.StatusTooManyRequests, "RATE_LIMITED")
	if limitedResponse.Header().Get("Retry-After") != "60" {
		t.Fatalf("retry-after = %q", limitedResponse.Header().Get("Retry-After"))
	}

	mutex.Lock()
	if authCalls != 2 || len(usageResults) != 2 || int(usageResults[0]["http_status"].(float64)) != http.StatusOK || int(usageResults[1]["http_status"].(float64)) != http.StatusTooManyRequests {
		t.Fatalf("auth calls=%d usage=%#v", authCalls, usageResults)
	}
	mutex.Unlock()

	unknown := openAPIRequest(http.MethodPost, "/open-api/v1/not-enabled", "bfk_example000_secret-value-for-test", true)
	unknownResponse := httptest.NewRecorder()
	SecurityHeaders(handler).ServeHTTP(unknownResponse, unknown)
	assertOpenAPIProblem(t, unknownResponse, http.StatusNotFound, "ROUTE_NOT_FOUND")
	mutex.Lock()
	if authCalls != 2 {
		t.Fatalf("unknown route reached API key verifier: %d", authCalls)
	}
	mutex.Unlock()

	internal := httptest.NewRequest(http.MethodPost, "http://gateway.test/auth/api/v1/internal/open-api/authenticate", strings.NewReader("{}"))
	internalResponse := httptest.NewRecorder()
	SecurityHeaders(handler).ServeHTTP(internalResponse, internal)
	if internalResponse.Code != http.StatusNotFound {
		t.Fatalf("public internal IAM route status = %d", internalResponse.Code)
	}

	plainHTTP := openAPIRequest(http.MethodGet, "/open-api/v1/settings", "bfk_example000_secret-value-for-test", false)
	plainHTTPResponse := httptest.NewRecorder()
	SecurityHeaders(handler).ServeHTTP(plainHTTPResponse, plainHTTP)
	assertOpenAPIProblem(t, plainHTTPResponse, http.StatusBadRequest, "HTTPS_REQUIRED")

	// The outer source-IP guard runs before API Key parsing. It must still use
	// the public V1 error envelope rather than the browser Gateway error shape.
	handler.limiter = newRateLimiter(1)
	firstDocs := httptest.NewRequest(http.MethodGet, "http://gateway.test/open-api/v1/docs", nil)
	firstDocs.RemoteAddr = "203.0.113.8:40000"
	SecurityHeaders(handler).ServeHTTP(httptest.NewRecorder(), firstDocs)
	secondDocs := httptest.NewRequest(http.MethodGet, "http://gateway.test/open-api/v1/docs", nil)
	secondDocs.RemoteAddr = "203.0.113.8:40001"
	secondDocsResponse := httptest.NewRecorder()
	SecurityHeaders(handler).ServeHTTP(secondDocsResponse, secondDocs)
	assertOpenAPIProblem(t, secondDocsResponse, http.StatusTooManyRequests, "RATE_LIMITED")
	if secondDocsResponse.Header().Get("Retry-After") != "60" {
		t.Fatalf("source rate-limit retry-after = %q", secondDocsResponse.Header().Get("Retry-After"))
	}
}

func TestOpenAPISpecIsValidAndMatchesRuntimeRoutes(t *testing.T) {
	var document struct {
		OpenAPI string                     `json:"openapi"`
		Paths   map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(openAPISpec, &document); err != nil {
		t.Fatal(err)
	}
	if document.OpenAPI != "3.1.0" {
		t.Fatalf("OpenAPI version = %q", document.OpenAPI)
	}
	for _, path := range []string{"/", "/me", "/settings", "/evidence/upload-sessions", "/evidence/upload-sessions/{session_id}/complete", "/evidence/upload-sessions/{session_id}/abort", "/reimbursements", "/reimbursements/{request_id}", "/reimbursements/{request_id}/submit", "/reimbursements/{request_id}/withdraw", "/reimbursements/{request_id}/restore"} {
		if _, found := document.Paths[path]; !found {
			t.Fatalf("OpenAPI path %q is missing", path)
		}
	}
}

func TestOpenAPIForwardsOnlyShortLivedOwnerCredential(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := writeKeys(t, publicKey)
	now := time.Now().UTC()
	openCredential := signToken(t, privateKey, map[string]any{"ver": "1", "iss": "https://oms.example.test/auth", "sub": testSubject, "tid": testTenant, "aud": "open-api", "name": "API Key", "actions": []string{openAPIUseAction}, "scope": map[string]any{"kind": "resources", "resources": []map[string]string{{"type": "api_key", "id": testKeyID}}}, "iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(), "auth_time": now.Unix(), "jti": "44444444-4444-4444-8444-444444444444"})
	ownerCredential := signToken(t, privateKey, map[string]any{"ver": "1", "iss": "https://oms.example.test/auth", "sub": testSubject, "tid": testTenant, "aud": "internal-finance-requests", "name": "测试员工", "actions": []string{"internal-finance-requests.self.read"}, "scope": map[string]any{"kind": "tenant", "resources": []any{}}, "person": map[string]any{"id": "55555555-5555-4555-8555-555555555555", "tenant_id": testTenant, "status": "active", "revision": 1, "reference_code": "EMP-001", "display_name": "测试员工", "digest": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, "iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(), "auth_time": now.Unix(), "jti": "66666666-6666-4666-8666-666666666666"})
	financeOwner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/open-api/reimbursements" || r.URL.Query().Get("status") != "draft" || r.Header.Get("Authorization") != "Bearer "+ownerCredential || r.Header.Get("ApiKey") != "" {
			t.Fatalf("forwarded request path=%s auth=%q", r.URL.String(), r.Header.Get("Authorization"))
		}
		w.Header().Set("ETag", `"page-1"`)
		writeJSON(w, http.StatusOK, map[string]any{"items": []any{}})
	}))
	defer financeOwner.Close()
	iam := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/internal/open-api/authenticate":
			var input map[string]string
			_ = json.NewDecoder(r.Body).Decode(&input)
			if input["action"] != "open-api.reimbursement.list" {
				t.Fatalf("action=%q", input["action"])
			}
			writeJSON(w, http.StatusOK, map[string]any{"credential": openCredential, "owner_credential": ownerCredential, "owner_audience": "internal-finance-requests", "expires_at": now.Add(5 * time.Minute), "tenant_id": testTenant, "subject_id": testSubject, "person_id": "55555555-5555-4555-8555-555555555555", "key_id": testKeyID, "key_prefix": "bfk_example000", "request_id": input["request_id"]})
		case "/api/v1/internal/open-api/usage-result":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer iam.Close()
	workbench := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer workbench.Close()
	suite := manifest.Suite{SchemaVersion: "1", SuiteID: "open-api-owner-test", TenantID: testTenant, Applications: []manifest.Application{{ID: "identity-access", Enabled: true, Required: true, RoutePrefix: "/auth", Upstream: iam.URL, Audience: "identity-access"}, {ID: "unified-workbench", Enabled: true, Required: true, RoutePrefix: "/workbench", Upstream: workbench.URL, Audience: "unified-workbench"}, {ID: "internal-finance-requests", Enabled: true, RoutePrefix: "/internal-finance/v1", Upstream: financeOwner.URL, Audience: "internal-finance-requests"}}}
	handler, err := New(suite, credential.New(keyPath, "https://oms.example.test/auth"), slog.New(slog.NewTextHandler(io.Discard, nil)), Options{RequestsPerMinute: 100, TrustProxyClientIP: true, OpenAPIKeyRequestsPerMinute: 60, OpenAPITenantRequestsPerMinute: 600, OpenAPIRequireHTTPS: true})
	if err != nil {
		t.Fatal(err)
	}
	req := openAPIRequest(http.MethodGet, "/open-api/v1/reimbursements?status=draft", "bfk_example000_secret-value-for-test", true)
	res := httptest.NewRecorder()
	SecurityHeaders(handler).ServeHTTP(res, req)
	if res.Code != http.StatusOK || res.Header().Get("ETag") != `"page-1"` || !strings.Contains(res.Body.String(), `"items"`) {
		t.Fatalf("status=%d etag=%q body=%s", res.Code, res.Header().Get("ETag"), res.Body.String())
	}
}

func TestOpenAPIV1BusinessAllowlistIsStatic(t *testing.T) {
	id := "33333333-3333-4333-8333-333333333333"
	tests := []struct {
		method, path, action, app string
	}{
		{http.MethodPost, "/open-api/v1/evidence/upload-sessions", "open-api.evidence.upload.create", "evidence-service"},
		{http.MethodPost, "/open-api/v1/evidence/upload-sessions/" + id + "/complete", "open-api.evidence.upload.complete", "evidence-service"},
		{http.MethodPost, "/open-api/v1/reimbursements", "open-api.reimbursement.create", "internal-finance-requests"},
		{http.MethodGet, "/open-api/v1/reimbursements", "open-api.reimbursement.list", "internal-finance-requests"},
		{http.MethodPut, "/open-api/v1/reimbursements/" + id, "open-api.reimbursement.update", "internal-finance-requests"},
		{http.MethodDelete, "/open-api/v1/reimbursements/" + id, "open-api.reimbursement.delete", "internal-finance-requests"},
		{http.MethodPost, "/open-api/v1/reimbursements/" + id + "/submit", "open-api.reimbursement.submit", "internal-finance-requests"},
		{http.MethodPost, "/open-api/v1/reimbursements/" + id + "/restore", "open-api.reimbursement.restore", "internal-finance-requests"},
	}
	for _, test := range tests {
		req := httptest.NewRequest(test.method, "https://gateway.test"+test.path, nil)
		route, ok := describeOpenAPIRoute(req)
		if !ok || route.action != test.action || route.applicationID != test.app || !allowedOpenAPIRoute(req) {
			t.Fatalf("%s %s route=%#v allowed=%v", test.method, test.path, route, ok)
		}
	}
	for _, path := range []string{"/open-api/v1/reimbursements/not-a-uuid", "/open-api/v1/evidence/upload-sessions/" + id + "/download", "/open-api/v1/payments"} {
		req := httptest.NewRequest(http.MethodPost, "https://gateway.test"+path, nil)
		if _, ok := describeOpenAPIRoute(req); ok || allowedOpenAPIRoute(req) {
			t.Fatalf("unexpectedly allowed %s", path)
		}
	}
}

func openAPIRequest(method, path, secret string, forwardedHTTPS bool) *http.Request {
	request := httptest.NewRequest(method, "http://gateway.test"+path, nil)
	request.RemoteAddr = "172.20.0.8:49152"
	request.Header.Set("X-Forwarded-For", "198.51.100.24")
	if forwardedHTTPS {
		request.Header.Set("X-Forwarded-Proto", "https")
	}
	if secret != "" {
		request.Header.Set("Authorization", "ApiKey "+secret)
	}
	return request
}

func assertOpenAPIProblem(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("problem status = %d, want %d: %s", response.Code, status, response.Body.String())
	}
	var value map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	if value["code"] != code || value["message"] == "" || value["request_id"] == "" || value["retryable"] == nil || value["field_errors"] == nil {
		t.Fatalf("problem = %#v", value)
	}
}
