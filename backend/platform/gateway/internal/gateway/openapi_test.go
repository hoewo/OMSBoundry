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
	handler, err := New(suite, credential.New(keyPath, "https://oms.example.test/auth", testTenant), slog.New(slog.NewTextHandler(io.Discard, nil)), Options{RequestsPerMinute: 100, TrustProxyClientIP: true, OpenAPIKeyRequestsPerMinute: 1, OpenAPITenantRequestsPerMinute: 2, OpenAPIRequireHTTPS: true})
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
	for _, path := range []string{"/", "/me", "/settings"} {
		if _, found := document.Paths[path]; !found {
			t.Fatalf("OpenAPI path %q is missing", path)
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
