package httpserver

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/identity-access/internal/apikey"
	"omsbizfoundry/identity-access/internal/capabilityauth"
	"omsbizfoundry/identity-access/internal/identity"
	"omsbizfoundry/identity-access/internal/signing"
	"omsbizfoundry/identity-access/internal/workload"
)

const (
	sessionCookieName = "oms_suite_session"
	csrfCookieName    = "oms_iam_csrf"
)

type principalKey struct{}

type Options struct {
	SessionCookieSecure bool
	Workloads           *workload.Authenticator
	APIKeys             *apikey.Service
}

type Server struct {
	pool         *pgxpool.Pool
	identities   *identity.Service
	keys         *signing.Store
	workloads    *workload.Authenticator
	apiKeys      *apikey.Service
	capabilities *capabilityauth.Verifier
	secureCookie bool
	handler      http.Handler
}

func New(pool *pgxpool.Pool, identities *identity.Service, keys *signing.Store, options Options) (*Server, error) {
	if pool == nil || identities == nil || keys == nil {
		return nil, fmt.Errorf("database, identity service and signing keys are required")
	}
	capabilities, err := capabilityauth.New(identities.Issuer(), keys)
	if err != nil {
		return nil, err
	}
	server := &Server{pool: pool, identities: identities, keys: keys, workloads: options.Workloads, apiKeys: options.APIKeys, capabilities: capabilities, secureCookie: options.SessionCookieSecure}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", server.home)
	mux.HandleFunc("GET /health/live", server.live)
	mux.HandleFunc("GET /health/ready", server.ready)
	mux.HandleFunc("GET /api/v1/csrf", server.csrf)
	mux.HandleFunc("GET /api/v1/.well-known/jwks.json", server.jwks)
	mux.HandleFunc("POST /api/v1/login", server.requireCSRF(server.login))
	mux.HandleFunc("POST /api/v1/service-credentials", server.issueServiceCredential)
	mux.HandleFunc("GET /api/v1/references/subjects/{subjectID}", server.subjectReference)
	mux.HandleFunc("POST /api/v1/activate", server.requireCSRF(server.activate))
	mux.HandleFunc("POST /api/v1/password-reset/complete", server.requireCSRF(server.completePasswordReset))
	mux.Handle("GET /api/v1/session", server.authenticate(http.HandlerFunc(server.currentSession)))
	mux.Handle("GET /api/v1/tenant", server.authenticate(http.HandlerFunc(server.currentTenant)))
	mux.Handle("POST /api/v1/logout", server.authenticate(http.HandlerFunc(server.requireCSRF(server.logout))))
	mux.Handle("POST /api/v1/credentials", server.authenticate(http.HandlerFunc(server.requireCSRF(server.issueCredential))))
	if server.apiKeys != nil {
		mux.Handle("GET /api/v1/api-keys", server.authenticate(http.HandlerFunc(server.listAPIKeys)))
		mux.Handle("POST /api/v1/api-keys", server.authenticate(http.HandlerFunc(server.requireCSRF(server.createAPIKey))))
		mux.Handle("POST /api/v1/api-keys/{keyID}/revoke", server.authenticate(http.HandlerFunc(server.requireCSRF(server.revokeAPIKey))))
		mux.Handle("GET /api/v1/open-api-usage", server.authenticate(http.HandlerFunc(server.listOpenAPIUsage)))
		mux.HandleFunc("POST /api/v1/internal/open-api/authenticate", server.authenticateAPIKey)
		mux.HandleFunc("POST /api/v1/internal/open-api/usage-result", server.completeAPIUsage)
	}
	mux.Handle("GET /api/v1/admin/subjects", server.authenticate(server.requireAdmin(http.HandlerFunc(server.listSubjects))))
	mux.Handle("POST /api/v1/admin/invitations", server.authenticate(server.requireAdmin(http.HandlerFunc(server.requireCSRF(server.invite)))))
	mux.Handle("POST /api/v1/admin/invitations/reissue", server.authenticate(server.requireAdmin(http.HandlerFunc(server.requireCSRF(server.reissueInvitation)))))
	mux.Handle("POST /api/v1/admin/password-resets", server.authenticate(server.requireAdmin(http.HandlerFunc(server.requireCSRF(server.createPasswordReset)))))
	mux.Handle("POST /api/v1/admin/grants", server.authenticate(server.requireAdmin(http.HandlerFunc(server.requireCSRF(server.grant)))))
	mux.Handle("POST /api/v1/admin/grants/revoke", server.authenticate(server.requireAdmin(http.HandlerFunc(server.requireCSRF(server.revokeGrant)))))
	mux.Handle("POST /api/v1/admin/status", server.authenticate(server.requireAdmin(http.HandlerFunc(server.requireCSRF(server.changeStatus)))))
	server.handler = securityHeaders(server.requestID(mux))
	return server, nil
}

func (s *Server) subjectReference(w http.ResponseWriter, r *http.Request) {
	subjectID := strings.ToLower(strings.TrimSpace(r.PathValue("subjectID")))
	claims, err := s.capabilities.Authenticate(r, "identity-access", "identity-access.subject.reference.read", "iam_subject", subjectID)
	if err != nil {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "capability credential is invalid")
		return
	}
	value, err := s.identities.SubjectReference(r.Context(), claims.Tenant, subjectID)
	if err != nil {
		mapError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) issueServiceCredential(w http.ResponseWriter, r *http.Request) {
	if s.workloads == nil {
		writeProblem(w, http.StatusNotFound, "not_found", "service credential exchange is not configured")
		return
	}
	var input struct {
		Assertion string         `json:"assertion"`
		Audience  string         `json:"audience"`
		Action    string         `json:"action"`
		Scope     identity.Scope `json:"scope"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	requestHash, err := workload.RequestHash(input.Audience, input.Action, input.Scope)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_request", "service credential request is invalid")
		return
	}
	authenticated, err := s.workloads.Authenticate(r.Context(), input.Assertion, requestHash)
	if err != nil {
		status, code := http.StatusUnauthorized, "invalid_workload_assertion"
		if errors.Is(err, workload.ErrReplay) {
			status, code = http.StatusConflict, "workload_assertion_replayed"
		}
		writeProblem(w, status, code, "service identity authentication failed")
		return
	}
	if err := s.workloads.Authorize(authenticated, input.Audience, input.Action, input.Scope); err != nil {
		writeProblem(w, http.StatusForbidden, "service_capability_denied", "service identity cannot request this capability")
		return
	}
	result, err := s.identities.IssueWorkloadCredential(r.Context(), authenticated.TenantID, identity.ServiceIdentity{
		ID: authenticated.Identity.ID, SubjectID: authenticated.Identity.SubjectID, DisplayName: authenticated.Identity.DisplayName,
		KeyID: authenticated.KeyID, AssertionID: authenticated.AssertionID, AuthTime: authenticated.IssuedAt,
	}, input.Audience, input.Action, input.Scope)
	if err != nil {
		mapError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }

func (s *Server) home(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"application": "identity-access", "contract_version": "1", "links": map[string]string{"csrf": "/api/v1/csrf", "jwks": "/api/v1/.well-known/jwks.json", "ready": "/health/ready"}})
}
func (s *Server) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "live"})
}
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.pool.Ping(ctx); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "not_ready", "IAM database is unavailable")
		return
	}
	if _, err := s.keys.PublicKeys(time.Now().UTC()); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "not_ready", "IAM signing keys are unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) csrf(w http.ResponseWriter, _ *http.Request) {
	token, err := newToken()
	if err != nil {
		writeProblem(w, 500, "internal_error", "无法建立安全请求")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: csrfCookieName, Value: token, Path: "/", Secure: s.secureCookie, HttpOnly: false, SameSite: http.SameSiteStrictMode, MaxAge: 3600})
	writeJSON(w, http.StatusOK, map[string]string{"csrf_token": token})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := s.identities.Login(r.Context(), input.Username, input.Password)
	if err != nil {
		mapError(w, err)
		return
	}
	maxAge := int(time.Until(result.ExpiresAt).Seconds())
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: result.Token, Path: "/", Secure: s.secureCookie, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: maxAge, Expires: result.ExpiresAt})
	writeJSON(w, http.StatusOK, result)
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(sessionCookieName)
	if cookie != nil {
		if err := s.identities.Logout(r.Context(), cookie.Value); err != nil {
			mapError(w, err)
			return
		}
	}
	s.clearSession(w)
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) currentSession(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"principal": currentPrincipal(r.Context())})
}
func (s *Server) currentTenant(w http.ResponseWriter, r *http.Request) {
	principal := currentPrincipal(r.Context())
	var value struct {
		ID        string    `json:"id"`
		Name      string    `json:"name"`
		CreatedAt time.Time `json:"created_at"`
	}
	if err := s.pool.QueryRow(r.Context(), `SELECT id::text,name,created_at FROM tenants WHERE id=$1`, principal.TenantID).Scan(&value.ID, &value.Name, &value.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeProblem(w, http.StatusNotFound, "tenant_not_found", "当前租户不存在")
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal_error", "无法读取当前租户")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"tenant": value})
}
func (s *Server) activate(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	subject, err := s.identities.Activate(r.Context(), input.Token, input.Password)
	if err != nil {
		mapError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"subject": subject})
}
func (s *Server) completePasswordReset(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	subject, err := s.identities.CompletePasswordReset(r.Context(), input.Token, input.Password)
	if err != nil {
		mapError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"subject": subject})
}
func (s *Server) issueCredential(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Audience     string                          `json:"audience"`
		Scope        identity.Scope                  `json:"scope"`
		BusinessFact *identity.BusinessFactReference `json:"business_fact,omitempty"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := s.identities.IssueCredentialWithFact(r.Context(), currentPrincipal(r.Context()), input.Audience, input.Scope, input.BusinessFact)
	if err != nil {
		mapError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	values, err := s.apiKeys.List(r.Context(), currentPrincipal(r.Context()))
	if err != nil {
		mapError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) createAPIKey(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Label   string `json:"label"`
		TTLDays int    `json:"ttl_days"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.TTLDays == 0 {
		input.TTLDays = 180
	}
	value, err := s.apiKeys.Create(r.Context(), currentPrincipal(r.Context()), input.Label, input.TTLDays)
	if err != nil {
		mapError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, value)
}

func (s *Server) revokeAPIKey(w http.ResponseWriter, r *http.Request) {
	value, err := s.apiKeys.Revoke(r.Context(), currentPrincipal(r.Context()), strings.ToLower(strings.TrimSpace(r.PathValue("keyID"))))
	if err != nil {
		mapError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) listOpenAPIUsage(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeProblem(w, http.StatusUnprocessableEntity, "invalid_page_size", "每页数量无效")
			return
		}
		limit = parsed
	}
	tenantWide := r.URL.Query().Get("scope") == "tenant"
	page, err := s.apiKeys.ListUsage(r.Context(), currentPrincipal(r.Context()), tenantWide, r.URL.Query().Get("cursor"), limit)
	if err != nil {
		mapError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) authenticateAPIKey(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RequestID      string `json:"request_id"`
		Action         string `json:"action"`
		SourceIPMasked string `json:"source_ip_masked"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	secret := apiKeySecret(r)
	value, err := s.apiKeys.Authenticate(r.Context(), secret, input.RequestID, input.Action, input.SourceIPMasked)
	if err != nil {
		mapError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) completeAPIUsage(w http.ResponseWriter, r *http.Request) {
	var input struct {
		KeyID      string `json:"key_id"`
		RequestID  string `json:"request_id"`
		ResultCode string `json:"result_code"`
		HTTPStatus int    `json:"http_status"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	claims, err := s.capabilities.Authenticate(r, "open-api", apikey.ActionUse, "api_key", strings.ToLower(strings.TrimSpace(input.KeyID)))
	if err != nil {
		writeProblem(w, http.StatusUnauthorized, "invalid_open_api_exchange", "开放 API 调用上下文无效")
		return
	}
	if err := s.apiKeys.CompleteUsage(r.Context(), claims.Tenant, claims.Subject, input.KeyID, input.RequestID, input.ResultCode, input.HTTPStatus); err != nil {
		mapError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func apiKeySecret(r *http.Request) string {
	raw := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(raw) > 7 && strings.EqualFold(raw[:7], "ApiKey ") {
		return strings.TrimSpace(raw[7:])
	}
	return ""
}
func (s *Server) listSubjects(w http.ResponseWriter, r *http.Request) {
	values, err := s.identities.ListSubjects(r.Context(), currentPrincipal(r.Context()))
	if err != nil {
		mapError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"subjects": values})
}
func (s *Server) invite(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username        string `json:"username,omitempty"`
		LoginIdentifier string `json:"login_identifier,omitempty"`
		DisplayName     string `json:"display_name"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	identifier := input.LoginIdentifier
	if strings.TrimSpace(identifier) == "" {
		identifier = input.Username
	}
	result, err := s.identities.Invite(r.Context(), currentPrincipal(r.Context()), identifier, input.DisplayName)
	if err != nil {
		mapError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, result)
}
func (s *Server) reissueInvitation(w http.ResponseWriter, r *http.Request) {
	var input struct {
		SubjectID string `json:"subject_id"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := s.identities.ReissueInvitation(r.Context(), currentPrincipal(r.Context()), input.SubjectID)
	if err != nil {
		mapError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, result)
}
func (s *Server) createPasswordReset(w http.ResponseWriter, r *http.Request) {
	var input struct {
		SubjectID string `json:"subject_id"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := s.identities.CreatePasswordReset(r.Context(), currentPrincipal(r.Context()), input.SubjectID)
	if err != nil {
		mapError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, result)
}
func (s *Server) grant(w http.ResponseWriter, r *http.Request) {
	var input struct {
		SubjectID string         `json:"subject_id"`
		Audience  string         `json:"audience"`
		Action    string         `json:"action"`
		Scope     identity.Scope `json:"scope"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.identities.Grant(r.Context(), currentPrincipal(r.Context()), input.SubjectID, input.Audience, input.Action, input.Scope); err != nil {
		mapError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) revokeGrant(w http.ResponseWriter, r *http.Request) {
	var input struct {
		SubjectID string         `json:"subject_id"`
		Audience  string         `json:"audience"`
		Action    string         `json:"action"`
		Scope     identity.Scope `json:"scope"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.identities.RevokeGrant(r.Context(), currentPrincipal(r.Context()), input.SubjectID, input.Audience, input.Action, input.Scope); err != nil {
		mapError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) changeStatus(w http.ResponseWriter, r *http.Request) {
	var input struct {
		SubjectID string          `json:"subject_id"`
		Status    identity.Status `json:"status"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.identities.ChangeStatus(r.Context(), currentPrincipal(r.Context()), input.SubjectID, input.Status); err != nil {
		mapError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) jwks(w http.ResponseWriter, _ *http.Request) {
	keys, err := s.keys.PublicKeys(time.Now().UTC())
	if err != nil {
		writeProblem(w, 503, "keys_unavailable", "签名公钥不可用")
		return
	}
	type jwk struct {
		KTY    string `json:"kty"`
		CRV    string `json:"crv"`
		Use    string `json:"use"`
		ALG    string `json:"alg"`
		KID    string `json:"kid"`
		X      string `json:"x"`
		Status string `json:"status"`
	}
	values := make([]jwk, 0, len(keys))
	for _, key := range keys {
		values = append(values, jwk{KTY: "OKP", CRV: "Ed25519", Use: "sig", ALG: "EdDSA", KID: key.KeyID, X: base64.RawURLEncoding.EncodeToString(key.PublicKey), Status: key.Status})
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	writeJSON(w, 200, map[string]any{"keys": values})
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			writeProblem(w, 401, "authentication_required", "需要登录")
			return
		}
		principal, found, err := s.identities.ResolveSession(r.Context(), cookie.Value)
		if err != nil {
			writeProblem(w, 500, "internal_error", "无法验证会话")
			return
		}
		if !found {
			s.clearSession(w)
			writeProblem(w, 401, "authentication_required", "会话无效或已过期")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, principal)))
	})
}
func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !currentPrincipal(r.Context()).Administrator {
			writeProblem(w, 403, "permission_denied", "需要 IAM 管理权限")
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) requireCSRF(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(csrfCookieName)
		header := r.Header.Get("X-CSRF-Token")
		if err != nil || cookie.Value == "" || header == "" || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
			writeProblem(w, 403, "csrf_failed", "安全校验失败")
			return
		}
		next(w, r)
	}
}
func currentPrincipal(ctx context.Context) identity.Principal {
	value, _ := ctx.Value(principalKey{}).(identity.Principal)
	return value
}
func (s *Server) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", Secure: s.secureCookie, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0)})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeProblem(w, 400, "invalid_request", "请求 JSON 无效")
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeProblem(w, 400, "invalid_request", "请求只能包含一个有效 JSON 值")
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeProblem(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func mapError(w http.ResponseWriter, err error) {
	var apiKeyError *apikey.DomainError
	var validation *identity.ValidationError
	var authentication *identity.AuthenticationError
	var authorization *identity.AuthorizationError
	var dependencyUnavailable *identity.DependencyUnavailableError
	var conflict *identity.ConflictError
	var notFound *identity.NotFoundError
	switch {
	case errors.As(err, &apiKeyError):
		status := http.StatusInternalServerError
		switch apiKeyError.Kind {
		case apikey.KindValidation:
			status = http.StatusUnprocessableEntity
		case apikey.KindUnauthorized:
			status = http.StatusUnauthorized
		case apikey.KindForbidden:
			status = http.StatusForbidden
		case apikey.KindNotFound:
			status = http.StatusNotFound
		case apikey.KindConflict:
			status = http.StatusConflict
		case apikey.KindDependency:
			status = http.StatusServiceUnavailable
		}
		writeProblem(w, status, apiKeyError.Code, apiKeyError.Message)
	case errors.As(err, &validation):
		writeProblem(w, 422, "validation_error", validation.Error())
	case errors.As(err, &authentication):
		writeProblem(w, 401, "authentication_failed", authentication.Error())
	case errors.As(err, &authorization):
		writeProblem(w, 403, "permission_denied", authorization.Error())
	case errors.As(err, &dependencyUnavailable):
		writeProblem(w, 503, "policy_dependency_unavailable", dependencyUnavailable.Error())
	case errors.As(err, &conflict):
		writeProblem(w, 409, "conflict", conflict.Error())
	case errors.As(err, &notFound):
		writeProblem(w, 404, "not_found", notFound.Error())
	default:
		writeProblem(w, 500, "internal_error", "服务暂时无法完成请求")
	}
}
func newToken() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
func (s *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if len(requestID) < 8 || len(requestID) > 128 {
			requestID, _ = newToken()
		}
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r)
	})
}
