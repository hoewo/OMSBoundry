package gateway

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"omsbizfoundry/gateway/internal/credential"
	"omsbizfoundry/gateway/internal/manifest"
)

const sessionCookieName = "oms_suite_session"
const csrfCookieName = "oms_iam_csrf"

var identityUUIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type Options struct {
	CookieSecure                   bool
	TrustProxyClientIP             bool
	RequestsPerMinute              int
	OpenAPIKeyRequestsPerMinute    int
	OpenAPITenantRequestsPerMinute int
	OpenAPIUploadConcurrency       int
	OpenAPIRequireHTTPS            bool
	Feedback                       FeedbackOptions
}
type Server struct {
	suite                manifest.Suite
	verifier             *credential.Verifier
	logger               *slog.Logger
	client               *http.Client
	proxies              map[string]*httputil.ReverseProxy
	iam                  manifest.Application
	cookieSecure         bool
	trustProxyClientIP   bool
	limiter              *rateLimiter
	openAPIKeyLimiter    *rateLimiter
	openAPITenantLimiter *rateLimiter
	openAPIRequireHTTPS  bool
	openAPIUploadSlots   chan struct{}
	feedback             feedbackRuntime
	handler              http.Handler
}
type sessionResponse struct {
	Principal struct {
		SubjectID           string    `json:"subject_id"`
		TenantID            string    `json:"tenant_id"`
		Username            string    `json:"username"`
		DisplayName         string    `json:"display_name"`
		Administrator       bool      `json:"administrator"`
		SystemAdministrator bool      `json:"system_administrator"`
		AuthTime            time.Time `json:"auth_time"`
	} `json:"principal"`
}
type credentialResponse struct {
	Token          string          `json:"token"`
	TokenType      string          `json:"token_type"`
	ExpiresAt      time.Time       `json:"expires_at"`
	Claims         json.RawMessage `json:"claims"`
	PolicyDecision json.RawMessage `json:"policy_decision,omitempty"`
}
type capabilityLeaseResponse struct {
	ApplicationID    string    `json:"application_id"`
	Audience         string    `json:"audience"`
	ExpiresAt        time.Time `json:"expires_at"`
	ExpiresInSeconds int       `json:"expires_in_seconds"`
}

func New(suite manifest.Suite, verifier *credential.Verifier, logger *slog.Logger, options Options) (*Server, error) {
	if verifier == nil || logger == nil {
		return nil, fmt.Errorf("credential verifier and logger are required")
	}
	iam, found := suite.Application("identity-access")
	if !found || !iam.Enabled {
		return nil, fmt.Errorf("enabled identity-access route is required")
	}
	if options.RequestsPerMinute < 10 {
		return nil, fmt.Errorf("request rate limit must be at least 10")
	}
	if options.OpenAPIKeyRequestsPerMinute == 0 {
		options.OpenAPIKeyRequestsPerMinute = 60
	}
	if options.OpenAPITenantRequestsPerMinute == 0 {
		options.OpenAPITenantRequestsPerMinute = 600
	}
	if options.OpenAPIUploadConcurrency == 0 {
		options.OpenAPIUploadConcurrency = 4
	}
	if options.OpenAPIKeyRequestsPerMinute < 1 || options.OpenAPITenantRequestsPerMinute < options.OpenAPIKeyRequestsPerMinute || options.OpenAPIUploadConcurrency < 1 || options.OpenAPIUploadConcurrency > 128 {
		return nil, fmt.Errorf("open API rate limits are invalid")
	}
	feedback, err := newFeedbackRuntime(options.Feedback)
	if err != nil {
		return nil, err
	}
	server := &Server{suite: suite, verifier: verifier, logger: logger, client: &http.Client{Timeout: 5 * time.Second}, proxies: map[string]*httputil.ReverseProxy{}, iam: iam, cookieSecure: options.CookieSecure, trustProxyClientIP: options.TrustProxyClientIP, limiter: newRateLimiter(options.RequestsPerMinute), openAPIKeyLimiter: newRateLimiter(options.OpenAPIKeyRequestsPerMinute), openAPITenantLimiter: newRateLimiter(options.OpenAPITenantRequestsPerMinute), openAPIRequireHTTPS: options.OpenAPIRequireHTTPS, openAPIUploadSlots: make(chan struct{}, options.OpenAPIUploadConcurrency), feedback: feedback}
	for _, app := range suite.Applications {
		if app.Enabled {
			proxy, err := server.proxyFor(app)
			if err != nil {
				return nil, err
			}
			server.proxies[app.ID] = proxy
		}
	}
	server.handler = server.accessLog(server.limit(server.openAPI(server.sanitize(http.HandlerFunc(server.route)))))
	return server, nil
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }

func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	if s.serveOpenAPI(w, r) {
		return
	}
	switch {
	case r.URL.Path == "/health/live":
		writeJSON(w, 200, map[string]string{"status": "live"})
		return
	case r.URL.Path == "/health/ready":
		if err := s.verifier.Ready(); err != nil {
			writeProblem(w, 503, "not_ready", "IAM public keys are unavailable")
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ready"})
		return
	case r.URL.Path == "/":
		// OMS phase 1 is the current product surface. Keep the public root on the
		// protected Workbench route so login, IAM and every owner authorization
		// remain unchanged, while avoiding a second navigation through the legacy
		// OA sidebar.
		http.Redirect(w, r, "/workbench/?view=oms-v1", http.StatusSeeOther)
		return
	case (r.URL.Path == "/oms" || r.URL.Path == "/oms/") && (r.Method == http.MethodGet || r.Method == http.MethodHead):
		// The old read-only OMS prototype is retired. Keep the historical alias
		// stable, but send it to the current protected OMS instead of booting the
		// reference UI and its legacy runtime.
		http.Redirect(w, r, "/workbench/?view=oms-v1", http.StatusSeeOther)
		return
	case (r.URL.Path == "/oms-v1" || r.URL.Path == "/oms-v1/") && (r.Method == http.MethodGet || r.Method == http.MethodHead):
		// OMS phase 1 is a protected Workbench composition. Unlike the historical
		// /oms/ reference preview, this route never enables a fabricated preview
		// actor: the Workbench will render IAM login until Gateway resolves a real
		// session, and every owner still performs final capability authorization.
		http.Redirect(w, r, "/workbench/?view=oms-v1", http.StatusSeeOther)
		return
	case (r.URL.Path == "/legacy" || r.URL.Path == "/legacy/") && (r.Method == http.MethodGet || r.Method == http.MethodHead):
		// The former OA workbench is no longer a public product surface.
		http.Redirect(w, r, "/workbench/?view=oms-v1", http.StatusSeeOther)
		return
	case (r.URL.Path == "/workbench" || r.URL.Path == "/workbench/") && (r.Method == http.MethodGet || r.Method == http.MethodHead) && retiredWorkbenchView(r):
		// Prevent direct query-string access from bypassing the retired public
		// aliases. Source remains available for archaeology, not as a live UI.
		http.Redirect(w, r, "/workbench/?view=oms-v1", http.StatusSeeOther)
		return
	case retiredWorkbenchAsset(r.URL.Path) && (r.Method == http.MethodGet || r.Method == http.MethodHead):
		// Retired UI assets remain in source history but are no longer public
		// runtime surfaces. This also prevents stale bookmarks from reconstructing
		// the former OA/OMS prototype outside the current product entry.
		writeProblem(w, http.StatusGone, "retired_ui_asset", "旧版界面资源已停止提供")
		return
	case strings.HasPrefix(r.URL.Path, s.iam.RoutePrefix+"/api/v1/internal/"):
		// IAM's API-key exchange and usage-completion routes are reachable only
		// over the internal service network. Never expose them through the edge.
		writeProblem(w, http.StatusNotFound, "route_not_found", "路由不存在")
		return
	case r.URL.Path == "/gateway/login" && r.Method == http.MethodPost:
		s.login(w, r)
		return
	case r.URL.Path == "/gateway/logout" && r.Method == http.MethodPost:
		s.logout(w, r)
		return
	case r.URL.Path == "/gateway/feedback/config" && r.Method == http.MethodPost:
		s.feedbackConfig(w, r)
		return
	case r.URL.Path == "/gateway/feedback/submission-snapshot" && r.Method == http.MethodPost:
		s.feedbackSubmissionSnapshot(w, r)
		return
	case strings.HasPrefix(r.URL.Path, "/gateway/authorize/") && r.Method == http.MethodPost:
		s.authorize(w, r)
		return
	}
	app, found := s.suite.Match(r.URL.Path)
	if !found {
		writeProblem(w, 404, "route_not_found", "应用未安装、未启用或路由不存在")
		return
	}
	if app.ID == "identity-access" {
		s.proxies[app.ID].ServeHTTP(w, r)
		return
	}
	if app.ID == "unified-workbench" {
		s.ensureCSRF(w, r)
		if session, ok := s.resolveSession(r.Context(), r); ok {
			r.Header.Set("X-OMS-Subject-ID", session.Principal.SubjectID)
			r.Header.Set("X-OMS-Tenant-ID", session.Principal.TenantID)
			r.Header.Set("X-OMS-Display-Name", session.Principal.DisplayName)
			r.Header.Set("X-OMS-Administrator", strconv.FormatBool(session.Principal.Administrator))
			r.Header.Set("X-OMS-System-Administrator", strconv.FormatBool(session.Principal.SystemAdministrator))
		}
		if cookie, err := r.Cookie(csrfCookieName); err == nil {
			r.Header.Set("X-OMS-CSRF-Token", cookie.Value)
		}
		s.proxies[app.ID].ServeHTTP(w, r)
		return
	}
	if app.ID == "evidence-service" && signedEvidenceDataPlaneRequest(app.RoutePrefix, r) {
		// Evidence signs short-lived, single-target upload/download URLs. API
		// clients do not have browser capability cookies, so the edge forwards
		// only these exact data-plane methods and lets Evidence verify the token,
		// target, action and expiry before touching object bytes.
		if r.Method == http.MethodPut {
			select {
			case s.openAPIUploadSlots <- struct{}{}:
				defer func() { <-s.openAPIUploadSlots }()
			default:
				w.Header().Set("Retry-After", "1")
				openAPIProblem(w, http.StatusTooManyRequests, "UPLOAD_CONCURRENCY_LIMITED", "发票上传请求较多，请稍后重试", w.Header().Get("X-Request-ID"), true, nil)
				return
			}
		}
		s.proxies[app.ID].ServeHTTP(w, r)
		return
	}
	cookie, err := r.Cookie(capabilityCookieName(app.ID))
	if err != nil {
		requireLogin(w, r)
		return
	}
	_, err = s.verifier.Verify(cookie.Value, app.Audience, app.EntryCapabilities()...)
	if err != nil {
		s.logger.Warn("capability credential rejected", "application_id", app.ID, "reason", err.Error())
		s.clearCapability(w, app)
		requireLogin(w, r)
		return
	}
	r.Header.Set("Authorization", "Bearer "+cookie.Value)
	s.proxies[app.ID].ServeHTTP(w, r)
}

func signedEvidenceDataPlaneRequest(prefix string, r *http.Request) bool {
	path := strings.TrimPrefix(r.URL.Path, prefix)
	if r.Method == http.MethodPut && strings.HasPrefix(path, "/v1/local-uploads/") {
		return r.URL.Query().Get("token") != ""
	}
	if (r.Method == http.MethodGet || r.Method == http.MethodHead) && strings.HasPrefix(path, "/v1/local-files/") {
		return r.URL.Query().Get("token") != ""
	}
	return false
}

func retiredWorkbenchView(r *http.Request) bool {
	view := strings.TrimSpace(r.URL.Query().Get("view"))
	return r.URL.Query().Get("preview") == "oa" || view == "legacy" || view == "oms-reference"
}

func retiredWorkbenchAsset(path string) bool {
	return path == "/workbench/assets/workbench.js" ||
		strings.HasPrefix(path, "/workbench/assets/reference-oa-") ||
		strings.HasPrefix(path, "/workbench/assets/reference-oms-demo.")
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !s.validCSRF(r) {
		writeProblem(w, 403, "csrf_failed", "安全校验失败")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		writeProblem(w, 400, "invalid_request", "登录请求无效")
		return
	}
	payload, _ := json.Marshal(map[string]string{"username": r.PostForm.Get("username"), "password": r.PostForm.Get("password")})
	request, err := s.iamRequest(r.Context(), http.MethodPost, "/api/v1/login", bytes.NewReader(payload), r)
	if err != nil {
		writeProblem(w, 502, "iam_unavailable", "IAM 暂时不可用")
		return
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		writeProblem(w, 502, "iam_unavailable", "IAM 暂时不可用")
		return
	}
	defer response.Body.Close()
	copySetCookies(w, response)
	if response.StatusCode != 200 {
		copyProblem(w, response)
		return
	}
	s.clearAllCapabilities(w)
	_, _ = io.Copy(io.Discard, response.Body)
	http.Redirect(w, r, safeReturnPath(r.PostForm.Get("return_to")), http.StatusSeeOther)
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if !s.validLogoutCSRF(r) {
		writeProblem(w, 403, "csrf_failed", "安全校验失败")
		return
	}
	request, err := s.iamRequest(r.Context(), http.MethodPost, "/api/v1/logout", http.NoBody, r)
	if err == nil {
		request.Header.Set("Content-Type", "application/json")
		response, doErr := s.client.Do(request)
		if doErr == nil {
			copySetCookies(w, response)
			_ = response.Body.Close()
		}
	}
	s.clearAllCapabilities(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
func (s *Server) validLogoutCSRF(r *http.Request) bool {
	if s.validCSRF(r) {
		return true
	}
	if cookie, err := r.Cookie(csrfCookieName); err != nil || cookie.Value == "" {
		return false
	}
	origin, err := url.Parse(r.Header.Get("Origin"))
	if err != nil || origin.Host == "" || origin.Host != r.Host {
		return false
	}
	expectedScheme := r.Header.Get("X-Forwarded-Proto")
	if expectedScheme == "" {
		expectedScheme = "http"
		if r.TLS != nil {
			expectedScheme = "https"
		}
	}
	return origin.Scheme == expectedScheme
}
func (s *Server) authorize(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !s.validCSRF(r) {
		writeProblem(w, 403, "csrf_failed", "安全校验失败")
		return
	}
	appID := strings.TrimPrefix(r.URL.Path, "/gateway/authorize/")
	app, found := s.suite.Application(appID)
	if !found || !app.Enabled || len(app.EntryCapabilities()) == 0 {
		writeProblem(w, 404, "application_not_found", "应用未启用")
		return
	}
	payloadValue := map[string]any{"audience": app.Audience}
	resourceType := strings.TrimSpace(r.PostForm.Get("resource_type"))
	resourceID := strings.TrimSpace(r.PostForm.Get("resource_id"))
	if resourceType != "" || resourceID != "" {
		if resourceType == "" || resourceID == "" {
			writeProblem(w, 400, "invalid_scope", "资源类型和资源 ID 必须同时提供")
			return
		}
		payloadValue["scope"] = map[string]any{"kind": "resources", "resources": []map[string]string{{"type": resourceType, "id": resourceID}}}
	}
	factKind := strings.TrimSpace(r.PostForm.Get("business_fact_kind"))
	factSource := strings.TrimSpace(r.PostForm.Get("business_fact_source"))
	factID := strings.TrimSpace(r.PostForm.Get("business_fact_id"))
	factRevisionRaw := strings.TrimSpace(r.PostForm.Get("business_fact_revision"))
	if factKind != "" || factSource != "" || factID != "" || factRevisionRaw != "" {
		factRevision, err := strconv.ParseInt(factRevisionRaw, 10, 64)
		if factKind == "" || factSource == "" || factID == "" || err != nil || factRevision < 1 {
			writeProblem(w, 400, "invalid_business_fact", "业务事实来源、种类、ID 和 revision 必须完整提供")
			return
		}
		payloadValue["business_fact"] = map[string]any{"kind": factKind, "source": factSource, "id": factID, "revision": factRevision}
	}
	payload, _ := json.Marshal(payloadValue)
	request, err := s.iamRequest(r.Context(), http.MethodPost, "/api/v1/credentials", bytes.NewReader(payload), r)
	if err != nil {
		writeProblem(w, 502, "iam_unavailable", "IAM 暂时不可用")
		return
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		writeProblem(w, 502, "iam_unavailable", "IAM 暂时不可用")
		return
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		// A definitive IAM rejection invalidates the browser's previous lease.
		// Upstream outages and 5xx responses must not erase an otherwise valid,
		// route-scoped capability that the owner can still verify independently.
		if response.StatusCode >= 400 && response.StatusCode < 500 {
			s.clearCapability(w, app)
		}
		copyProblem(w, response)
		return
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64<<10))
	decoder.DisallowUnknownFields()
	var issued credentialResponse
	if err := decoder.Decode(&issued); err != nil || issued.Token == "" || issued.TokenType != "Bearer" || !issued.ExpiresAt.After(time.Now()) || len(issued.Claims) == 0 {
		writeProblem(w, 502, "invalid_iam_response", "IAM 返回了无效凭证")
		return
	}
	claims, verifyErr := s.verifier.Verify(issued.Token, app.Audience, app.EntryCapabilities()...)
	if verifyErr != nil {
		s.clearCapability(w, app)
		writeProblem(w, 403, "entry_capability_denied", "当前凭证不满足应用入口能力")
		return
	}
	// The unsigned response expiry must never extend the signed credential TTL.
	if signedExpiry := time.Unix(claims.ExpiresAt, 0); signedExpiry.Before(issued.ExpiresAt) {
		issued.ExpiresAt = signedExpiry
	}
	maxAge := int(time.Until(issued.ExpiresAt).Seconds())
	if maxAge < 1 {
		writeProblem(w, 502, "invalid_iam_response", "IAM 返回了已过期凭证")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: capabilityCookieName(app.ID), Value: issued.Token, Path: app.RoutePrefix, Secure: s.cookieSecure, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: maxAge, Expires: issued.ExpiresAt})
	if strings.Contains(strings.ToLower(r.Header.Get("Accept")), "application/json") {
		writeJSON(w, http.StatusOK, capabilityLeaseResponse{ApplicationID: app.ID, Audience: app.Audience, ExpiresAt: issued.ExpiresAt, ExpiresInSeconds: maxAge})
		return
	}
	http.Redirect(w, r, app.RoutePrefix+"/", http.StatusSeeOther)
}

func (s *Server) resolveSession(ctx context.Context, original *http.Request) (sessionResponse, bool) {
	request, err := s.iamRequest(ctx, http.MethodGet, "/api/v1/session", nil, original)
	if err != nil {
		return sessionResponse{}, false
	}
	response, err := s.client.Do(request)
	if err != nil {
		return sessionResponse{}, false
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return sessionResponse{}, false
	}
	var value sessionResponse
	// Tenant identity comes from the trusted IAM session. suite.TenantID is the
	// deployment's bootstrap/default tenant and must not turn a shared Gateway
	// into a single-tenant allowlist.
	if err := json.NewDecoder(io.LimitReader(response.Body, 32<<10)).Decode(&value); err != nil || !identityUUIDPattern.MatchString(value.Principal.TenantID) || !identityUUIDPattern.MatchString(value.Principal.SubjectID) {
		return sessionResponse{}, false
	}
	return value, true
}
func (s *Server) iamRequest(ctx context.Context, method, path string, body io.Reader, original *http.Request) (*http.Request, error) {
	base, err := url.Parse(s.iam.Upstream)
	if err != nil {
		return nil, err
	}
	target := base.ResolveReference(&url.URL{Path: path})
	request, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"Cookie", "X-CSRF-Token", "X-Request-ID"} {
		if value := original.Header.Get(name); value != "" {
			request.Header.Set(name, value)
		}
	}
	if request.Header.Get("X-CSRF-Token") == "" {
		if cookie, cookieErr := original.Cookie(csrfCookieName); cookieErr == nil {
			request.Header.Set("X-CSRF-Token", cookie.Value)
		}
	}
	return request, nil
}
func (s *Server) proxyFor(app manifest.Application) (*httputil.ReverseProxy, error) {
	target, err := url.Parse(app.Upstream)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(r *http.Request) {
		prefix := app.RoutePrefix
		r.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
		if r.URL.Path == "" {
			r.URL.Path = "/"
		}
		originalDirector(r)
		r.Host = target.Host
	}
	proxy.ModifyResponse = func(response *http.Response) error {
		if app.ID == "identity-access" && response.StatusCode >= 200 && response.StatusCode < 300 && response.Request.Method == http.MethodPost && (response.Request.URL.Path == "/api/v1/login" || response.Request.URL.Path == "/api/v1/logout" || response.Request.URL.Path == "/api/v1/session/organization") {
			for _, protectedApp := range s.suite.Applications {
				if len(protectedApp.EntryCapabilities()) != 0 {
					response.Header.Add("Set-Cookie", expiredCapabilityCookie(protectedApp, s.cookieSecure).String())
				}
			}
		}
		location := response.Header.Get("Location")
		if strings.HasPrefix(location, "/") && !strings.HasPrefix(location, app.RoutePrefix+"/") {
			response.Header.Set("Location", app.RoutePrefix+location)
		}
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		writeProblem(w, 503, "upstream_unavailable", "目标应用暂时不可用")
	}
	return proxy, nil
}

func (s *Server) sanitize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for name := range r.Header {
			canonical := http.CanonicalHeaderKey(name)
			if canonical == "Authorization" || canonical == "Proxy-Authorization" || strings.HasPrefix(canonical, "X-Oms-") {
				r.Header.Del(name)
			}
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) ensureCSRF(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(csrfCookieName); err == nil && cookie.Value != "" {
		return
	}
	token, _ := newToken()
	http.SetCookie(w, &http.Cookie{Name: csrfCookieName, Value: token, Path: "/", Secure: s.cookieSecure, HttpOnly: false, SameSite: http.SameSiteStrictMode, MaxAge: 3600})
	r.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
}
func (s *Server) validCSRF(r *http.Request) bool {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	token := r.Header.Get("X-CSRF-Token")
	if token == "" {
		_ = r.ParseForm()
		token = r.PostForm.Get("csrf_token")
	}
	return token != "" && subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(token)) == 1
}
func (s *Server) clearCapability(w http.ResponseWriter, app manifest.Application) {
	http.SetCookie(w, expiredCapabilityCookie(app, s.cookieSecure))
}
func (s *Server) clearAllCapabilities(w http.ResponseWriter) {
	for _, app := range s.suite.Applications {
		if len(app.EntryCapabilities()) != 0 {
			s.clearCapability(w, app)
		}
	}
}
func expiredCapabilityCookie(app manifest.Application, secure bool) *http.Cookie {
	return &http.Cookie{Name: capabilityCookieName(app.ID), Value: "", Path: app.RoutePrefix, Secure: secure, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0)}
}
func capabilityCookieName(appID string) string {
	return "oms_cap_" + strings.ReplaceAll(appID, "-", "_")
}
func requireLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		http.Redirect(w, r, "/workbench/?return_to="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
		return
	}
	writeProblem(w, 401, "credential_required", "需要重新进入该应用")
}
func safeReturnPath(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/") || strings.HasPrefix(parsed.Path, "//") || strings.Contains(raw, "\\") {
		return "/workbench/?view=oms-v1"
	}
	return parsed.RequestURI()
}
func copySetCookies(w http.ResponseWriter, response *http.Response) {
	for _, value := range response.Header.Values("Set-Cookie") {
		w.Header().Add("Set-Cookie", value)
	}
}
func copyProblem(w http.ResponseWriter, response *http.Response) {
	data, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(data)
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *statusWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	count, err := w.ResponseWriter.Write(data)
	w.bytes += count
	return count, err
}
func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		wrapped := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(wrapped, r)
		status := wrapped.status
		if status == 0 {
			status = 200
		}
		s.logger.Info("gateway request", "method", r.Method, "path", r.URL.Path, "status", status, "bytes", wrapped.bytes, "duration_ms", time.Since(started).Milliseconds(), "request_id", w.Header().Get("X-Request-ID"))
	})
}
func (s *Server) limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Immutable Workbench assets are not business/API traffic. Charging CSS
		// and JS against the same client budget makes a legitimate API burst turn
		// the next page load into JSON-with-a-stylesheet-MIME-error. The 120/minute
		// rule remains unchanged for HTML, authorization and all business routes.
		if (r.Method == http.MethodGet || r.Method == http.MethodHead) && strings.HasPrefix(r.URL.Path, "/workbench/assets/") {
			next.ServeHTTP(w, r)
			return
		}
		if !s.limiter.Allow(rateLimitClientKey(r, s.trustProxyClientIP), time.Now()) {
			if strings.HasPrefix(r.URL.Path, "/open-api/v1/") {
				w.Header().Set("Retry-After", "60")
				openAPIProblem(w, http.StatusTooManyRequests, "RATE_LIMITED", "请求超过开放 API 限额，请稍后重试", w.Header().Get("X-Request-ID"), true, nil)
				return
			}
			writeProblem(w, 429, "rate_limited", "请求过于频繁")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func rateLimitClientKey(r *http.Request, trustProxyClientIP bool) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	host = strings.TrimSpace(host)
	if !trustProxyClientIP {
		return host
	}
	forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	// The trusted D1 Nginx edge overwrites X-Forwarded-For with one client IP.
	// A list means that boundary was bypassed or misconfigured, so fail back to
	// the directly observed peer instead of accepting a spoofable value.
	if forwarded == "" || strings.Contains(forwarded, ",") {
		return host
	}
	ip := net.ParseIP(forwarded)
	if ip == nil {
		return host
	}
	return ip.String()
}

type rateEntry struct {
	window time.Time
	count  int
}
type rateLimiter struct {
	mu      sync.Mutex
	limit   int
	entries map[string]rateEntry
}

func newRateLimiter(limit int) *rateLimiter {
	return &rateLimiter{limit: limit, entries: map[string]rateEntry{}}
}
func (l *rateLimiter) Allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[key]
	if entry.window.IsZero() || now.Sub(entry.window) >= time.Minute {
		entry = rateEntry{window: now}
	}
	entry.count++
	l.entries[key] = entry
	return entry.count <= l.limit
}
func newToken() (string, error) {
	data := make([]byte, 24)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeProblem(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Request-ID") == "" {
			token, _ := newToken()
			r.Header.Set("X-Request-ID", token)
		}
		w.Header().Set("X-Request-ID", r.Header.Get("X-Request-ID"))
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}
