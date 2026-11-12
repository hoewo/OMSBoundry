package gateway

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"omsbizfoundry/gateway/internal/credential"
	"omsbizfoundry/gateway/internal/manifest"
)

const testTenant = "22222222-2222-4222-8222-222222222222"
const testSubject = "11111111-1111-4111-8111-111111111111"
const testKeyID = "33333333-3333-4333-8333-333333333333"

func TestGatewayLoginHeaderCleaningAuthorizationAndDisabledRoute(t *testing.T) {
	sessionTenant := "55555555-5555-4555-8555-555555555555"
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := writeKeys(t, publicKey)
	workbench := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"subject": r.Header.Get("X-OMS-Subject-ID"), "tenant": r.Header.Get("X-OMS-Tenant-ID"), "csrf": r.Header.Get("X-OMS-CSRF-Token"), "system_administrator": r.Header.Get("X-OMS-System-Administrator")})
	}))
	defer workbench.Close()
	contract := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"authorization": r.Header.Get("Authorization"), "forged": r.Header.Get("X-OMS-Forged"), "path": r.URL.Path})
	}))
	defer contract.Close()
	var issuedToken string
	var requestedCredential map[string]any
	denyCredential := false
	iam := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/login":
			csrfCookie, csrfErr := r.Cookie(csrfCookieName)
			if csrfErr != nil || r.Header.Get("X-CSRF-Token") != csrfCookie.Value {
				writeProblem(w, 403, "csrf", "bad")
				return
			}
			http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "session-good", Path: "/", HttpOnly: true})
			writeJSON(w, 200, map[string]any{"principal": map[string]string{"subject_id": testSubject}})
		case "/api/v1/session":
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil || cookie.Value != "session-good" {
				writeProblem(w, 401, "auth", "bad")
				return
			}
			writeJSON(w, 200, map[string]any{"principal": map[string]any{"subject_id": testSubject, "tenant_id": sessionTenant, "username": "admin", "display_name": "系统管理员", "administrator": false, "system_administrator": true, "auth_time": time.Now().UTC()}})
		case "/api/v1/credentials":
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil || cookie.Value != "session-good" {
				writeProblem(w, 401, "auth", "bad")
				return
			}
			if denyCredential {
				writeProblem(w, 403, "capability_denied", "revoked")
				return
			}
			if err := json.NewDecoder(r.Body).Decode(&requestedCredential); err != nil {
				writeProblem(w, 400, "request", "bad")
				return
			}
			writeJSON(w, 200, map[string]any{"token": issuedToken, "token_type": "Bearer", "expires_at": time.Now().UTC().Add(5 * time.Minute), "claims": map[string]any{"aud": "contract-ledger"}})
		case "/api/v1/logout":
			http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1})
			w.WriteHeader(204)
		case "/api/v1/session/organization":
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil || cookie.Value != "session-good" {
				writeProblem(w, 401, "auth", "bad")
				return
			}
			writeJSON(w, 200, map[string]any{"principal": map[string]string{"subject_id": testSubject, "tenant_id": sessionTenant}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer iam.Close()
	now := time.Now().UTC()
	issuedToken = signToken(t, privateKey, map[string]any{"ver": "1", "iss": "https://oms.example.test/auth", "sub": testSubject, "tid": sessionTenant, "aud": "contract-ledger", "name": "系统管理员", "actions": []string{"contract-ledger.view"}, "scope": map[string]any{"kind": "tenant", "resources": []any{}}, "iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(), "auth_time": now.Add(-time.Minute).Unix(), "jti": "44444444-4444-4444-8444-444444444444"})
	suite := manifest.Suite{SchemaVersion: "1", SuiteID: "test-suite", TenantID: testTenant, Applications: []manifest.Application{{ID: "identity-access", Version: "0.1.0", Enabled: true, Required: true, RoutePrefix: "/auth", Upstream: iam.URL, ReadinessPath: "/health/ready", Audience: "identity-access"}, {ID: "unified-workbench", Version: "0.1.0", Enabled: true, Required: true, RoutePrefix: "/workbench", Upstream: workbench.URL, ReadinessPath: "/health/ready", Audience: "unified-workbench"}, {ID: "contract-ledger", Version: "0.1.0", Enabled: true, RoutePrefix: "/contract", Upstream: contract.URL, ReadinessPath: "/health/ready", Audience: "contract-ledger", EntryAction: "contract-ledger.view"}, {ID: "money-flow-ledger", Version: "0.1.0", Enabled: false, RoutePrefix: "/money", Upstream: "http://127.0.0.1:1", ReadinessPath: "/health/ready", Audience: "money-flow-ledger", EntryAction: "money-flow-ledger.view"}}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler, err := New(suite, credential.New(keyPath, "https://oms.example.test/auth"), logger, Options{RequestsPerMinute: 100})
	if err != nil {
		t.Fatal(err)
	}
	gateway := httptest.NewServer(SecurityHeaders(handler))
	defer gateway.Close()
	client := clientWithJar(t)

	request, _ := http.NewRequest(http.MethodGet, gateway.URL+"/workbench/", nil)
	request.Header.Set("Authorization", "Bearer forged")
	request.Header.Set("X-OMS-Subject-ID", "attacker")
	request.Header.Set("X-OMS-System-Administrator", "true")
	response := do(t, client, request, 200)
	anonymous := decodeMap(t, response.Body)
	_ = response.Body.Close()
	if anonymous["subject"] != "" || anonymous["system_administrator"] != "" {
		t.Fatalf("forged subject reached workbench: %#v", anonymous)
	}
	csrf := cookieValue(t, client, gateway.URL, csrfCookieName)
	if csrf == "" {
		t.Fatal("gateway did not establish CSRF cookie")
	}
	if anonymous["csrf"] != csrf {
		t.Fatal("workbench did not receive gateway-derived CSRF value")
	}
	reloginForm := url.Values{"username": {"admin"}, "password": {"correct horse battery staple"}, "csrf_token": {csrf}, "return_to": {"/workbench/"}}
	response = postForm(t, client, gateway.URL+"/gateway/login", reloginForm, http.StatusSeeOther)
	_ = response.Body.Close()
	if cookieValue(t, client, gateway.URL, sessionCookieName) != "session-good" {
		t.Fatal("IAM session cookie was not propagated")
	}
	response = get(t, client, gateway.URL+"/workbench/", 200)
	authenticated := decodeMap(t, response.Body)
	_ = response.Body.Close()
	if authenticated["subject"] != testSubject || authenticated["tenant"] != sessionTenant || authenticated["system_administrator"] != "true" {
		t.Fatalf("validated session context missing: %#v", authenticated)
	}
	form := url.Values{"csrf_token": {csrf}}
	response = postForm(t, client, gateway.URL+"/gateway/authorize/contract-ledger", form, http.StatusSeeOther)
	_ = response.Body.Close()
	if cookieValueForPath(t, client, gateway.URL, "/contract/", capabilityCookieName("contract-ledger")) == "" {
		t.Fatal("route-scoped capability cookie missing")
	}
	request, _ = http.NewRequest(http.MethodGet, gateway.URL+"/contract/contracts", nil)
	request.Header.Set("Authorization", "Bearer forged")
	request.Header.Set("X-OMS-Forged", "attacker")
	response = do(t, client, request, 200)
	upstream := decodeMap(t, response.Body)
	_ = response.Body.Close()
	if upstream["authorization"] != "Bearer "+issuedToken || upstream["forged"] != "" || upstream["path"] != "/contracts" {
		t.Fatalf("unexpected protected upstream request: %#v", upstream)
	}
	response = postForm(t, client, gateway.URL+"/gateway/login", reloginForm, http.StatusSeeOther)
	_ = response.Body.Close()
	if cookieValueForPath(t, client, gateway.URL, "/contract/", capabilityCookieName("contract-ledger")) != "" {
		t.Fatal("successful suite login retained the previous account capability")
	}
	form = url.Values{"csrf_token": {csrf}}
	response = postForm(t, client, gateway.URL+"/gateway/authorize/contract-ledger", form, http.StatusSeeOther)
	_ = response.Body.Close()
	directLogin, _ := http.NewRequest(http.MethodPost, gateway.URL+"/auth/api/v1/login", strings.NewReader(`{"username":"admin","password":"correct horse battery staple"}`))
	directLogin.Header.Set("Content-Type", "application/json")
	directLogin.Header.Set("X-CSRF-Token", csrf)
	response = do(t, client, directLogin, http.StatusOK)
	_ = response.Body.Close()
	if cookieValueForPath(t, client, gateway.URL, "/contract/", capabilityCookieName("contract-ledger")) != "" {
		t.Fatal("successful direct IAM login retained the previous account capability")
	}
	form = url.Values{"csrf_token": {csrf}}
	response = postForm(t, client, gateway.URL+"/gateway/authorize/contract-ledger", form, http.StatusSeeOther)
	_ = response.Body.Close()
	switchRequest, _ := http.NewRequest(http.MethodPost, gateway.URL+"/auth/api/v1/session/organization", strings.NewReader(`{"tenant_id":"`+sessionTenant+`"}`))
	switchRequest.Header.Set("Content-Type", "application/json")
	switchRequest.Header.Set("X-CSRF-Token", csrf)
	response = do(t, client, switchRequest, http.StatusOK)
	_ = response.Body.Close()
	if cookieValueForPath(t, client, gateway.URL, "/contract/", capabilityCookieName("contract-ledger")) != "" {
		t.Fatal("organization switch retained the previous tenant capability")
	}
	issuedToken = signToken(t, privateKey, map[string]any{"ver": "1", "iss": "https://oms.example.test/auth", "sub": testSubject, "tid": sessionTenant, "aud": "contract-ledger", "name": "系统管理员", "actions": []string{"contract-ledger.view"}, "scope": map[string]any{"kind": "resources", "resources": []map[string]string{{"type": "contract", "id": "55555555-5555-4555-8555-555555555555"}}}, "iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(), "auth_time": now.Add(-time.Minute).Unix(), "jti": "55555555-5555-4555-8555-555555555555"})
	form = url.Values{"csrf_token": {csrf}, "resource_type": {"contract"}, "resource_id": {"55555555-5555-4555-8555-555555555555"}}
	response = postForm(t, client, gateway.URL+"/gateway/authorize/contract-ledger", form, http.StatusSeeOther)
	_ = response.Body.Close()
	requestedScope, ok := requestedCredential["scope"].(map[string]any)
	if !ok || requestedScope["kind"] != "resources" {
		t.Fatalf("resource scope was not forwarded to IAM: %#v", requestedCredential)
	}
	response = get(t, client, gateway.URL+"/contract/contracts", http.StatusOK)
	_ = response.Body.Close()
	denyCredential = true
	form = url.Values{"csrf_token": {csrf}}
	response = postForm(t, client, gateway.URL+"/gateway/authorize/contract-ledger", form, http.StatusForbidden)
	_ = response.Body.Close()
	if cookieValueForPath(t, client, gateway.URL, "/contract/", capabilityCookieName("contract-ledger")) != "" {
		t.Fatal("failed reauthorization retained a stale capability cookie")
	}
	denyCredential = false
	form = url.Values{"csrf_token": {csrf}, "resource_type": {"contract"}, "resource_id": {"55555555-5555-4555-8555-555555555555"}}
	response = postForm(t, client, gateway.URL+"/gateway/authorize/contract-ledger", form, http.StatusSeeOther)
	_ = response.Body.Close()
	if cookieValueForPath(t, client, gateway.URL, "/contract/", capabilityCookieName("contract-ledger")) == "" {
		t.Fatal("capability cookie missing before IAM outage")
	}
	iam.Close()
	response = postForm(t, client, gateway.URL+"/gateway/authorize/contract-ledger", form, http.StatusBadGateway)
	_ = response.Body.Close()
	if cookieValueForPath(t, client, gateway.URL, "/contract/", capabilityCookieName("contract-ledger")) != "" {
		t.Fatal("IAM outage retained a stale capability cookie after explicit reauthorization")
	}
	response = get(t, client, gateway.URL+"/money/", 404)
	_ = response.Body.Close()

	logoutForm := url.Values{"csrf_token": {"target-application-csrf"}}
	crossOrigin, _ := http.NewRequest(http.MethodPost, gateway.URL+"/gateway/logout", strings.NewReader(logoutForm.Encode()))
	crossOrigin.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	crossOrigin.Header.Set("Origin", "https://attacker.example")
	response = do(t, client, crossOrigin, http.StatusForbidden)
	_ = response.Body.Close()
	sameOrigin, _ := http.NewRequest(http.MethodPost, gateway.URL+"/gateway/logout", strings.NewReader(logoutForm.Encode()))
	sameOrigin.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	sameOrigin.Header.Set("Origin", gateway.URL)
	response = do(t, client, sameOrigin, http.StatusSeeOther)
	_ = response.Body.Close()
}

func TestGatewayRejectsWrongAudienceAndRateLimits(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	path := writeKeys(t, publicKey)
	now := time.Now().UTC()
	wrong := signToken(t, privateKey, map[string]any{"ver": "1", "iss": "https://oms.example.test/auth", "sub": testSubject, "tid": testTenant, "aud": "money-flow-ledger", "name": "User", "actions": []string{"money-flow-ledger.view"}, "scope": map[string]any{"kind": "tenant", "resources": []any{}}, "iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(time.Minute).Unix(), "auth_time": now.Add(-time.Minute).Unix(), "jti": "44444444-4444-4444-8444-444444444444"})
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("wrong-audience token reached upstream") }))
	defer app.Close()
	iam := httptest.NewServer(http.NotFoundHandler())
	defer iam.Close()
	workbench := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer workbench.Close()
	suite := manifest.Suite{SchemaVersion: "1", SuiteID: "test-suite", TenantID: testTenant, Applications: []manifest.Application{{ID: "identity-access", Enabled: true, Required: true, RoutePrefix: "/auth", Upstream: iam.URL, Audience: "identity-access"}, {ID: "unified-workbench", Enabled: true, Required: true, RoutePrefix: "/workbench", Upstream: workbench.URL, Audience: "unified-workbench"}, {ID: "contract-ledger", Enabled: true, RoutePrefix: "/contract", Upstream: app.URL, Audience: "contract-ledger", EntryAction: "contract-ledger.view"}}}
	handler, err := New(suite, credential.New(path, "https://oms.example.test/auth"), slog.New(slog.NewTextHandler(io.Discard, nil)), Options{RequestsPerMinute: 10})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	client := clientWithJar(t)
	target, _ := url.Parse(server.URL + "/contract/")
	client.Jar.SetCookies(target, []*http.Cookie{{Name: capabilityCookieName("contract-ledger"), Value: wrong, Path: "/contract"}})
	response := get(t, client, target.String(), http.StatusSeeOther)
	_ = response.Body.Close()
	for index := 0; index < 9; index++ {
		response = get(t, client, server.URL+"/health/live", 200)
		_ = response.Body.Close()
	}
	response = get(t, client, server.URL+"/health/live", 429)
	_ = response.Body.Close()
}

func TestRateLimitClientKeyUsesOnlySingleTrustedForwardedIP(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://gateway.test/health/live", nil)
	request.RemoteAddr = "172.20.0.8:49152"
	request.Header.Set("X-Forwarded-For", "198.51.100.24")
	if got := rateLimitClientKey(request, false); got != "172.20.0.8" {
		t.Fatalf("untrusted proxy header changed rate key: %q", got)
	}
	if got := rateLimitClientKey(request, true); got != "198.51.100.24" {
		t.Fatalf("trusted edge client IP was not used: %q", got)
	}

	request.Header.Set("X-Forwarded-For", "203.0.113.7, 198.51.100.24")
	if got := rateLimitClientKey(request, true); got != "172.20.0.8" {
		t.Fatalf("ambiguous forwarded chain was trusted: %q", got)
	}
	request.Header.Set("X-Forwarded-For", "not-an-ip")
	if got := rateLimitClientKey(request, true); got != "172.20.0.8" {
		t.Fatalf("invalid forwarded address was trusted: %q", got)
	}
}

func TestGatewayRateLimitSeparatesClientsBehindTrustedEdge(t *testing.T) {
	publicKey, _, _ := ed25519.GenerateKey(rand.Reader)
	path := writeKeys(t, publicKey)
	iam := httptest.NewServer(http.NotFoundHandler())
	defer iam.Close()
	workbench := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer workbench.Close()
	suite := manifest.Suite{SchemaVersion: "1", SuiteID: "test-suite", TenantID: testTenant, Applications: []manifest.Application{{ID: "identity-access", Enabled: true, Required: true, RoutePrefix: "/auth", Upstream: iam.URL, Audience: "identity-access"}, {ID: "unified-workbench", Enabled: true, Required: true, RoutePrefix: "/workbench", Upstream: workbench.URL, Audience: "unified-workbench"}}}
	handler, err := New(suite, credential.New(path, "https://oms.example.test/auth"), slog.New(slog.NewTextHandler(io.Discard, nil)), Options{TrustProxyClientIP: true, RequestsPerMinute: 10})
	if err != nil {
		t.Fatal(err)
	}

	request := func(clientIP string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "http://gateway.test/health/live", nil)
		req.RemoteAddr = "172.20.0.8:49152"
		req.Header.Set("X-Forwarded-For", clientIP)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}
	for index := 0; index < 10; index++ {
		if status := request("198.51.100.24").Code; status != http.StatusOK {
			t.Fatalf("first client request %d returned %d", index+1, status)
		}
	}
	if status := request("198.51.100.24").Code; status != http.StatusTooManyRequests {
		t.Fatalf("first client limit returned %d", status)
	}
	if status := request("203.0.113.9").Code; status != http.StatusOK {
		t.Fatalf("second client inherited the first client's rate limit: %d", status)
	}
}

func TestOMSReferenceEntryRedirectsThroughWorkbenchPreview(t *testing.T) {
	publicKey, _, _ := ed25519.GenerateKey(rand.Reader)
	path := writeKeys(t, publicKey)
	iam := httptest.NewServer(http.NotFoundHandler())
	defer iam.Close()
	workbench := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("OMS entry redirect must not call the Workbench upstream before the browser follows it")
	}))
	defer workbench.Close()
	suite := manifest.Suite{SchemaVersion: "1", SuiteID: "test-suite", TenantID: testTenant, Applications: []manifest.Application{{ID: "identity-access", Enabled: true, Required: true, RoutePrefix: "/auth", Upstream: iam.URL, Audience: "identity-access"}, {ID: "unified-workbench", Enabled: true, Required: true, RoutePrefix: "/workbench", Upstream: workbench.URL, Audience: "unified-workbench"}}}
	handler, err := New(suite, credential.New(path, "https://oms.example.test/auth"), slog.New(slog.NewTextHandler(io.Discard, nil)), Options{RequestsPerMinute: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []string{"/oms", "/oms/"} {
		request := httptest.NewRequest(http.MethodGet, "http://gateway.test"+entry, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusSeeOther {
			t.Fatalf("%s returned %d", entry, response.Code)
		}
		if location := response.Header().Get("Location"); location != "/workbench/?view=oms-v1" {
			t.Fatalf("%s redirected to %q", entry, location)
		}
	}
}

func TestOMSPhase1EntryRedirectsToProtectedWorkbench(t *testing.T) {
	publicKey, _, _ := ed25519.GenerateKey(rand.Reader)
	path := writeKeys(t, publicKey)
	iam := httptest.NewServer(http.NotFoundHandler())
	defer iam.Close()
	workbench := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("OMS phase 1 entry redirect must not call the Workbench upstream before the browser follows it")
	}))
	defer workbench.Close()
	suite := manifest.Suite{SchemaVersion: "1", SuiteID: "test-suite", TenantID: testTenant, Applications: []manifest.Application{{ID: "identity-access", Enabled: true, Required: true, RoutePrefix: "/auth", Upstream: iam.URL, Audience: "identity-access"}, {ID: "unified-workbench", Enabled: true, Required: true, RoutePrefix: "/workbench", Upstream: workbench.URL, Audience: "unified-workbench"}}}
	handler, err := New(suite, credential.New(path, "https://oms.example.test/auth"), slog.New(slog.NewTextHandler(io.Discard, nil)), Options{RequestsPerMinute: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []string{"/oms-v1", "/oms-v1/"} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			request := httptest.NewRequest(method, "http://gateway.test"+entry, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusSeeOther {
				t.Fatalf("%s %s returned %d", method, entry, response.Code)
			}
			if location := response.Header().Get("Location"); location != "/workbench/?view=oms-v1" {
				t.Fatalf("%s %s redirected to %q", method, entry, location)
			}
		}
	}
}

func TestPrimaryAndRetiredEntriesResolveToCurrentOMS(t *testing.T) {
	publicKey, _, _ := ed25519.GenerateKey(rand.Reader)
	path := writeKeys(t, publicKey)
	iam := httptest.NewServer(http.NotFoundHandler())
	defer iam.Close()
	workbench := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("public entry redirects must not call Workbench before the browser follows them")
	}))
	defer workbench.Close()
	suite := manifest.Suite{SchemaVersion: "1", SuiteID: "test-suite", TenantID: testTenant, Applications: []manifest.Application{{ID: "identity-access", Enabled: true, Required: true, RoutePrefix: "/auth", Upstream: iam.URL, Audience: "identity-access"}, {ID: "unified-workbench", Enabled: true, Required: true, RoutePrefix: "/workbench", Upstream: workbench.URL, Audience: "unified-workbench"}}}
	handler, err := New(suite, credential.New(path, "https://oms.example.test/auth"), slog.New(slog.NewTextHandler(io.Discard, nil)), Options{RequestsPerMinute: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		path     string
		location string
	}{
		{path: "/", location: "/workbench/?view=oms-v1"},
		{path: "/legacy", location: "/workbench/?view=oms-v1"},
		{path: "/legacy/", location: "/workbench/?view=oms-v1"},
		{path: "/oms", location: "/workbench/?view=oms-v1"},
		{path: "/oms/", location: "/workbench/?view=oms-v1"},
		{path: "/workbench/?view=legacy", location: "/workbench/?view=oms-v1"},
		{path: "/workbench/?view=oms-reference", location: "/workbench/?view=oms-v1"},
		{path: "/workbench/?preview=oa&view=oms-v1", location: "/workbench/?view=oms-v1"},
	} {
		request := httptest.NewRequest(http.MethodGet, "http://gateway.test"+item.path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusSeeOther || response.Header().Get("Location") != item.location {
			t.Fatalf("%s returned status=%d location=%q", item.path, response.Code, response.Header().Get("Location"))
		}
	}
	if got := safeReturnPath("https://attacker.example/"); got != "/workbench/?view=oms-v1" {
		t.Fatalf("unsafe return path fell back to %q", got)
	}
	for _, path := range []string{
		"/workbench/assets/reference-oa-react-bidding.js",
		"/workbench/assets/reference-oa-initiation-runtime.js",
		"/workbench/assets/reference-oms-demo.js",
		"/workbench/assets/workbench.js",
	} {
		request := httptest.NewRequest(http.MethodGet, "http://gateway.test"+path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusGone {
			t.Fatalf("retired asset %s returned status=%d", path, response.Code)
		}
	}
}

func TestWorkbenchAssetsDoNotConsumeBusinessRequestBudget(t *testing.T) {
	publicKey, _, _ := ed25519.GenerateKey(rand.Reader)
	path := writeKeys(t, publicKey)
	iam := httptest.NewServer(http.NotFoundHandler())
	defer iam.Close()
	workbench := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer workbench.Close()
	suite := manifest.Suite{SchemaVersion: "1", SuiteID: "test-suite", TenantID: testTenant, Applications: []manifest.Application{{ID: "identity-access", Enabled: true, Required: true, RoutePrefix: "/auth", Upstream: iam.URL, Audience: "identity-access"}, {ID: "unified-workbench", Enabled: true, Required: true, RoutePrefix: "/workbench", Upstream: workbench.URL, Audience: "unified-workbench"}}}
	handler, err := New(suite, credential.New(path, "https://oms.example.test/auth"), slog.New(slog.NewTextHandler(io.Discard, nil)), Options{RequestsPerMinute: 10})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	client := clientWithJar(t)
	for index := 0; index < 20; index++ {
		response := get(t, client, server.URL+"/workbench/assets/oms-phase1.css?v=test", http.StatusOK)
		_ = response.Body.Close()
	}
	for index := 0; index < 10; index++ {
		response := get(t, client, server.URL+"/health/live", http.StatusOK)
		_ = response.Body.Close()
	}
	response := get(t, client, server.URL+"/health/live", http.StatusTooManyRequests)
	_ = response.Body.Close()
}

func TestSignedEvidenceDataPlaneDoesNotRequireBrowserCapabilityCookie(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := writeKeys(t, publicKey)
	var receivedPath, receivedToken string
	evidence := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath, receivedToken = r.URL.Path, r.URL.Query().Get("token")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer evidence.Close()
	iam := httptest.NewServer(http.NotFoundHandler())
	defer iam.Close()
	workbench := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer workbench.Close()
	suite := manifest.Suite{SchemaVersion: "1", SuiteID: "signed-evidence-test", TenantID: testTenant, Applications: []manifest.Application{
		{ID: "identity-access", Enabled: true, Required: true, RoutePrefix: "/auth", Upstream: iam.URL, Audience: "identity-access"},
		{ID: "unified-workbench", Enabled: true, Required: true, RoutePrefix: "/workbench", Upstream: workbench.URL, Audience: "unified-workbench"},
		{ID: "evidence-service", Enabled: true, RoutePrefix: "/evidence", Upstream: evidence.URL, Audience: "evidence-service", EntryAction: "evidence-service.view"},
	}}
	handler, err := New(suite, credential.New(keyPath, "https://oms.example.test/auth"), slog.New(slog.NewTextHandler(io.Discard, nil)), Options{RequestsPerMinute: 100, OpenAPIUploadConcurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPut, "https://oms.example.test/evidence/v1/local-uploads/11111111-1111-4111-8111-111111111111?token=signed", strings.NewReader("invoice"))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || receivedPath != "/v1/local-uploads/11111111-1111-4111-8111-111111111111" || receivedToken != "signed" {
		t.Fatalf("status=%d path=%q token=%q", response.Code, receivedPath, receivedToken)
	}
	handler.openAPIUploadSlots <- struct{}{}
	limited := httptest.NewRecorder()
	handler.ServeHTTP(limited, httptest.NewRequest(http.MethodPut, "https://oms.example.test/evidence/v1/local-uploads/11111111-1111-4111-8111-111111111111?token=signed", strings.NewReader("invoice")))
	<-handler.openAPIUploadSlots
	if limited.Code != http.StatusTooManyRequests || limited.Header().Get("Retry-After") != "1" {
		t.Fatalf("saturated signed upload status=%d retry-after=%q body=%s", limited.Code, limited.Header().Get("Retry-After"), limited.Body.String())
	}

	for _, test := range []struct {
		method, path string
		expected     int
	}{
		{http.MethodPost, "/evidence/v1/local-uploads/11111111-1111-4111-8111-111111111111?token=signed", http.StatusUnauthorized},
		{http.MethodPut, "/evidence/v1/local-uploads/11111111-1111-4111-8111-111111111111", http.StatusUnauthorized},
		{http.MethodGet, "/evidence/v1/files?token=signed", http.StatusSeeOther},
	} {
		result := httptest.NewRecorder()
		handler.ServeHTTP(result, httptest.NewRequest(test.method, "https://oms.example.test"+test.path, nil))
		if result.Code != test.expected {
			t.Fatalf("%s %s unexpectedly bypassed browser authorization: %d", test.method, test.path, result.Code)
		}
	}
}

func writeKeys(t *testing.T, publicKey ed25519.PublicKey) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "keys.json")
	data, _ := json.Marshal(map[string]any{"keys": []map[string]string{{"kty": "OKP", "crv": "Ed25519", "use": "sig", "alg": "EdDSA", "kid": testKeyID, "x": base64.RawURLEncoding.EncodeToString(publicKey), "status": "active"}}})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
func signToken(t *testing.T, key ed25519.PrivateKey, claims any) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": testKeyID})
	body, _ := json.Marshal(claims)
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, []byte(unsigned)))
}
func clientWithJar(t *testing.T) *http.Client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Jar: jar, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
}
func postForm(t *testing.T, client *http.Client, endpoint string, form url.Values, status int) *http.Response {
	t.Helper()
	request, _ := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return do(t, client, request, status)
}
func get(t *testing.T, client *http.Client, endpoint string, status int) *http.Response {
	t.Helper()
	request, _ := http.NewRequest(http.MethodGet, endpoint, nil)
	return do(t, client, request, status)
}
func do(t *testing.T, client *http.Client, request *http.Request, status int) *http.Response {
	t.Helper()
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != status {
		data, _ := io.ReadAll(response.Body)
		_ = response.Body.Close()
		t.Fatalf("%s %s status=%d want=%d body=%s", request.Method, request.URL, response.StatusCode, status, data)
	}
	return response
}
func decodeMap(t *testing.T, reader io.Reader) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.NewDecoder(reader).Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}
func cookieValue(t *testing.T, client *http.Client, base, name string) string {
	return cookieValueForPath(t, client, base, "/", name)
}
func cookieValueForPath(t *testing.T, client *http.Client, base, path, name string) string {
	t.Helper()
	target, err := url.Parse(base + path)
	if err != nil {
		t.Fatal(err)
	}
	for _, cookie := range client.Jar.Cookies(target) {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}
