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
	"testing"
	"time"

	"omsbizfoundry/gateway/internal/credential"
	"omsbizfoundry/gateway/internal/manifest"
)

func TestFeedbackIdentityIsStableAndSeparatedByTenantAndSubject(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	first := deriveFeedbackUserID(secret, "v1", testTenant, testSubject)
	if first != deriveFeedbackUserID(secret, "v1", testTenant, testSubject) {
		t.Fatal("feedback identity is not stable")
	}
	if first == deriveFeedbackUserID(secret, "v1", "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", testSubject) {
		t.Fatal("different tenants share a feedback identity")
	}
	if first == deriveFeedbackUserID(secret, "v1", testTenant, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb") {
		t.Fatal("different subjects share a feedback identity")
	}
	if strings.Contains(first, testTenant) || strings.Contains(first, testSubject) || !strings.HasPrefix(first, "oms1_v1_") {
		t.Fatalf("feedback identity leaks source identity or lacks version: %q", first)
	}
}

func TestFeedbackRuntimeRejectsUnsafeOrAmbiguousConfiguration(t *testing.T) {
	valid := FeedbackOptions{AuthMode: "apiKey", GatewayURL: "https://api.example.test", ProjectID: 115, APIKeys: []string{"ak_0123456789abcdef"}, IdentityHMACKey: []byte("0123456789abcdef0123456789abcdef"), IdentityVersion: "v1"}
	if runtime, err := newFeedbackRuntime(valid); err != nil || !runtime.enabled {
		t.Fatalf("valid feedback runtime rejected: %#v %v", runtime, err)
	}
	for name, mutate := range map[string]func(*FeedbackOptions){
		"http remote":        func(value *FeedbackOptions) { value.GatewayURL = "http://api.example.test" },
		"url credentials":    func(value *FeedbackOptions) { value.GatewayURL = "https://user:pass@api.example.test" },
		"missing project":    func(value *FeedbackOptions) { value.ProjectID = 0 },
		"short identity key": func(value *FeedbackOptions) { value.IdentityHMACKey = []byte("short") },
		"bad version":        func(value *FeedbackOptions) { value.IdentityVersion = "latest" },
		"duplicate key":      func(value *FeedbackOptions) { value.APIKeys = []string{"ak_0123456789abcdef", "ak_0123456789abcdef"} },
		"unbounded rotation": func(value *FeedbackOptions) {
			value.APIKeys = []string{"ak_0123456789abcdef", "ak_1123456789abcdef", "ak_2123456789abcdef"}
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			candidate.APIKeys = append([]string(nil), valid.APIKeys...)
			mutate(&candidate)
			if _, err := newFeedbackRuntime(candidate); err == nil {
				t.Fatal("unsafe feedback runtime accepted")
			}
		})
	}
}

func TestFeedbackConfigRequiresOnlyActiveSessionAndReturnsPseudonymousScope(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := writeKeys(t, publicKey)
	now := time.Now().UTC()
	iam := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie(sessionCookieName); err != nil || cookie.Value != "session-good" {
			writeProblem(w, http.StatusUnauthorized, "session", "bad")
			return
		}
		if r.URL.Path == "/api/v1/session" {
			writeJSON(w, http.StatusOK, map[string]any{"principal": map[string]any{"subject_id": testSubject, "tenant_id": testTenant, "username": "tester", "display_name": "体验账号", "administrator": false, "auth_time": now}})
			return
		}
		http.NotFound(w, r)
	}))
	defer iam.Close()

	suite := manifest.Suite{SchemaVersion: "1", SuiteID: "feedback-test", TenantID: testTenant, Applications: []manifest.Application{{ID: "identity-access", Version: "0.1.0", Enabled: true, Required: true, RoutePrefix: "/auth", Upstream: iam.URL, ReadinessPath: "/health/ready", Audience: "identity-access"}}}
	server, err := New(suite, credential.New(keyPath, "https://oms.example.test/auth"), slog.New(slog.NewTextHandler(io.Discard, nil)), Options{RequestsPerMinute: 100, Feedback: FeedbackOptions{AuthMode: "apiKey", GatewayURL: "https://api.feitianchengzi.com", ProjectID: 115, APIKeys: []string{"ak_0123456789abcdef"}, IdentityHMACKey: []byte("0123456789abcdef0123456789abcdef"), IdentityVersion: "v1", NotificationsEnabled: true}})
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "https://oms.example.test/gateway/feedback/config", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-good"})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-token-0123456789"})
	request.Header.Set("X-CSRF-Token", "csrf-token-0123456789")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("feedback config response = %d %s", response.Code, response.Body.String())
	}
	var body struct {
		Config map[string]any `json:"config"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	customUserID, _ := body.Config["customUserId"].(string)
	if body.Config["projectId"] != float64(115) || body.Config["apiKey"] != "ak_0123456789abcdef" || body.Config["feedbackV2AuthMode"] != "apiKey" || !strings.HasPrefix(customUserID, "oms1_v1_") {
		t.Fatalf("unexpected feedback config: %#v", body.Config)
	}
	encoded := response.Body.String()
	if strings.Contains(encoded, testTenant) || strings.Contains(encoded, testSubject) || strings.Contains(encoded, "session-good") {
		t.Fatalf("feedback config leaked IAM identity/session: %s", encoded)
	}

	request = httptest.NewRequest(http.MethodPost, "https://oms.example.test/gateway/feedback/config", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-invalid"})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-token-0123456789"})
	request.Header.Set("X-CSRF-Token", "csrf-token-0123456789")
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || strings.Contains(response.Body.String(), "ak_0123456789abcdef") {
		t.Fatalf("invalid feedback session response = %d %s", response.Code, response.Body.String())
	}
}

func TestFeedbackSessionExchangeUsesBoundedRotationFallback(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") == "Bearer ak_previous_0123456789" {
			writeProblem(w, http.StatusUnauthorized, "key", "rotated")
			return
		}
		if r.Header.Get("Authorization") != "Bearer ak_current_0123456789" {
			writeProblem(w, http.StatusForbidden, "key", "unknown")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": "OK", "data": map[string]any{
			"token": "fbs_0123456789abcdef", "token_type": "Bearer", "project_id": 115,
			"custom_user_id": "oms1_v1_example", "expires_at": time.Now().UTC().Add(15 * time.Minute),
		}})
	}))
	defer server.Close()

	adapter := &Server{client: server.Client(), feedback: feedbackRuntime{gatewayURL: server.URL, projectID: 115, apiKeys: []string{"ak_previous_0123456789", "ak_current_0123456789"}}}
	session, err := adapter.exchangeFeedbackSession(t.Context(), "oms1_v1_example")
	if err != nil || session.Token != "fbs_0123456789abcdef" || calls != 2 {
		t.Fatalf("rotated Feedback key fallback session=%#v calls=%d err=%v", session, calls, err)
	}
}

func TestFeedbackSessionExchangeRejectsUnavailableService(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeProblem(w, http.StatusServiceUnavailable, "unavailable", "down")
	}))
	serverURL := server.URL
	server.Close()
	adapter := &Server{client: &http.Client{Timeout: 100 * time.Millisecond}, feedback: feedbackRuntime{gatewayURL: serverURL, projectID: 115, apiKeys: []string{"ak_placeholder_0123456789"}}}
	if _, err := adapter.exchangeFeedbackSession(t.Context(), "oms1_v1_example"); err == nil {
		t.Fatal("unavailable Feedback service was treated as a successful session exchange")
	}
}
