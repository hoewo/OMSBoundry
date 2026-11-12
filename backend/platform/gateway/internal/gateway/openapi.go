package gateway

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"omsbizfoundry/gateway/internal/credential"
)

const openAPIUseAction = "open-api.invoice-reimbursement.use"

var errOpenAPIPersonMappingRequired = errors.New("open API action requires a Person mapping")

//go:embed openapi.v1.json
var openAPISpec []byte

type openAPIContextKey struct{}

type openAPIIdentity struct {
	Credential      string    `json:"credential"`
	OwnerCredential string    `json:"owner_credential"`
	OwnerAudience   string    `json:"owner_audience"`
	ExpiresAt       time.Time `json:"expires_at"`
	TenantID        string    `json:"tenant_id"`
	SubjectID       string    `json:"subject_id"`
	PersonID        string    `json:"person_id,omitempty"`
	KeyID           string    `json:"key_id"`
	KeyPrefix       string    `json:"key_prefix"`
	RequestID       string    `json:"request_id"`
}

type openAPIResultWriter struct {
	http.ResponseWriter
	status int
}

func (w *openAPIResultWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *openAPIResultWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func (s *Server) openAPI(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/open-api/v1/") {
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/open-api/v1/openapi.json" || r.URL.Path == "/open-api/v1/docs" {
			next.ServeHTTP(w, r)
			return
		}
		if !allowedOpenAPIRoute(r) {
			next.ServeHTTP(w, r)
			return
		}
		requestID := w.Header().Get("X-Request-ID")
		if requestID == "" {
			requestID = r.Header.Get("X-Request-ID")
		}
		if s.openAPIRequireHTTPS && requestScheme(r, s.trustProxyClientIP) != "https" {
			openAPIProblem(w, http.StatusBadRequest, "HTTPS_REQUIRED", "开放 API 只接受 HTTPS 请求", requestID, false, nil)
			return
		}
		secret := publicAPIKey(r)
		if secret == "" {
			openAPIProblem(w, http.StatusUnauthorized, "INVALID_API_KEY", "API Key 缺失、无效或不可用", requestID, false, nil)
			return
		}
		action := openAPIAction(r)
		identity, status, err := s.exchangeAPIKey(r.Context(), secret, requestID, action, maskedClientIP(r, s.trustProxyClientIP))
		if err != nil {
			if errors.Is(err, errOpenAPIPersonMappingRequired) {
				openAPIProblemWithRecovery(w, http.StatusForbidden, "PERSON_MAPPING_REQUIRED", "该业务需要先关联成员档案；请在成员管理中完成关联后重试", requestID, false, nil, map[string]string{
					"kind":   "member_account_link",
					"method": http.MethodGet,
					"path":   "/workbench/?view=oms-v1#member-account-link",
				})
			} else if status == http.StatusServiceUnavailable {
				openAPIProblem(w, status, "AUTHENTICATION_UNAVAILABLE", "API Key 验证服务暂不可用", requestID, true, nil)
			} else {
				openAPIProblem(w, http.StatusUnauthorized, "INVALID_API_KEY", "API Key 缺失、无效或不可用", requestID, false, nil)
			}
			return
		}
		claims, err := s.verifier.Verify(identity.Credential, "open-api", openAPIUseAction)
		if err != nil || claims.Subject != identity.SubjectID || claims.Tenant != identity.TenantID || len(claims.Scope.Resources) != 1 || claims.Scope.Resources[0] != (credential.ResourceRef{Type: "api_key", ID: identity.KeyID}) {
			openAPIProblem(w, http.StatusUnauthorized, "INVALID_API_KEY", "API Key 缺失、无效或不可用", requestID, false, nil)
			return
		}
		if !s.openAPIKeyLimiter.Allow(identity.KeyID, time.Now()) || !s.openAPITenantLimiter.Allow(identity.TenantID, time.Now()) {
			w.Header().Set("Retry-After", "60")
			openAPIProblem(w, http.StatusTooManyRequests, "RATE_LIMITED", "请求超过开放 API 限额，请稍后重试", requestID, true, nil)
			s.completeOpenAPIUsage(identity, http.StatusTooManyRequests, "RATE_LIMITED")
			return
		}
		if strings.HasPrefix(action, "open-api.evidence.upload.") {
			select {
			case s.openAPIUploadSlots <- struct{}{}:
				defer func() { <-s.openAPIUploadSlots }()
			default:
				w.Header().Set("Retry-After", "1")
				openAPIProblem(w, http.StatusTooManyRequests, "UPLOAD_CONCURRENCY_LIMITED", "发票上传请求较多，请稍后重试", requestID, true, nil)
				s.completeOpenAPIUsage(identity, http.StatusTooManyRequests, "UPLOAD_CONCURRENCY_LIMITED")
				return
			}
		}
		// Never let the account credential continue as an upstream Authorization
		// header. Only the short-lived IAM credential may cross the Gateway edge.
		r.Header.Del("Authorization")
		r = r.WithContext(context.WithValue(r.Context(), openAPIContextKey{}, identity))
		wrapped := &openAPIResultWriter{ResponseWriter: w}
		next.ServeHTTP(wrapped, r)
		status = wrapped.status
		if status == 0 {
			status = http.StatusOK
		}
		resultCode := "OK"
		if status >= 400 {
			resultCode = fmt.Sprintf("HTTP_%d", status)
		}
		s.completeOpenAPIUsage(identity, status, resultCode)
	})
}

func (s *Server) exchangeAPIKey(ctx context.Context, secret, requestID, action, sourceIP string) (openAPIIdentity, int, error) {
	payload, _ := json.Marshal(map[string]string{"request_id": requestID, "action": action, "source_ip_masked": sourceIP})
	base, err := url.Parse(s.iam.Upstream)
	if err != nil {
		return openAPIIdentity{}, http.StatusServiceUnavailable, err
	}
	target := base.ResolveReference(&url.URL{Path: "/api/v1/internal/open-api/authenticate"})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), bytes.NewReader(payload))
	if err != nil {
		return openAPIIdentity{}, http.StatusServiceUnavailable, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "ApiKey "+secret)
	request.Header.Set("X-Request-ID", requestID)
	response, err := s.client.Do(request)
	if err != nil {
		return openAPIIdentity{}, http.StatusServiceUnavailable, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		var value struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		_ = json.Unmarshal(body, &value)
		if response.StatusCode == http.StatusForbidden && value.Error.Code == "person_mapping_required" {
			return openAPIIdentity{}, http.StatusForbidden, errOpenAPIPersonMappingRequired
		}
		if response.StatusCode >= 500 {
			return openAPIIdentity{}, http.StatusServiceUnavailable, fmt.Errorf("IAM returned HTTP %d", response.StatusCode)
		}
		return openAPIIdentity{}, http.StatusUnauthorized, fmt.Errorf("API key rejected")
	}
	var value openAPIIdentity
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil || value.Credential == "" || (ownerForAction(action) != "" && (value.OwnerCredential == "" || value.OwnerAudience != ownerForAction(action))) || value.RequestID != requestID || value.KeyID == "" || value.TenantID == "" || value.SubjectID == "" || (openAPIActionRequiresPerson(action) && value.PersonID == "") || !value.ExpiresAt.After(time.Now()) {
		return openAPIIdentity{}, http.StatusServiceUnavailable, fmt.Errorf("IAM returned an invalid API key context")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return openAPIIdentity{}, http.StatusServiceUnavailable, fmt.Errorf("IAM API key context contains trailing data")
	}
	return value, http.StatusOK, nil
}

func (s *Server) completeOpenAPIUsage(identity openAPIIdentity, status int, resultCode string) {
	payload, _ := json.Marshal(map[string]any{"key_id": identity.KeyID, "request_id": identity.RequestID, "result_code": resultCode, "http_status": status})
	base, err := url.Parse(s.iam.Upstream)
	if err != nil {
		s.logger.Warn("open API usage completion skipped", "request_id", identity.RequestID, "key_prefix", identity.KeyPrefix, "reason", "invalid IAM upstream")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, base.ResolveReference(&url.URL{Path: "/api/v1/internal/open-api/usage-result"}).String(), bytes.NewReader(payload))
	if err != nil {
		s.logger.Warn("open API usage completion skipped", "request_id", identity.RequestID, "key_prefix", identity.KeyPrefix, "reason", "request creation failed")
		return
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+identity.Credential)
	request.Header.Set("X-Request-ID", identity.RequestID)
	response, err := s.client.Do(request)
	if err != nil {
		// Keep the failure observable without logging an internal URL or network
		// topology from the transport error.
		s.logger.Warn("open API usage completion failed", "request_id", identity.RequestID, "key_prefix", identity.KeyPrefix, "reason", "IAM request failed")
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		s.logger.Warn("open API usage completion rejected", "request_id", identity.RequestID, "key_prefix", identity.KeyPrefix, "status", response.StatusCode)
	}
}

func (s *Server) serveOpenAPI(w http.ResponseWriter, r *http.Request) bool {
	if !strings.HasPrefix(r.URL.Path, "/open-api/v1/") {
		return false
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/open-api/v1/openapi.json":
		w.Header().Set("Content-Type", "application/vnd.oai.openapi+json; charset=utf-8")
		w.Header().Set("Content-Disposition", `inline; filename="oms-open-api-v1.json"`)
		w.Header().Set("Cache-Control", "public, max-age=300")
		_, _ = w.Write(openAPISpec)
	case r.Method == http.MethodGet && r.URL.Path == "/open-api/v1/docs":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "OMS1 开放 API V1\n\n认证：Authorization: ApiKey <secret>\nAPI Key 绑定当前组织与登录账号，创建不要求关联成员。发票 Evidence 上传可直接使用；本人报销草稿／提交／撤回／归档恢复／草稿删除和本人列表／详情查询需要当前账号已关联有效成员，否则返回 PERSON_MAPPING_REQUIRED。所有写请求必须提供 Idempotency-Key。Webhook 仅预留设置，当前不启用。\n\n验证示例（请从安全环境变量读取 Key）：\n  curl --fail --silent --show-error \\\n    -H \"Authorization: ApiKey $OMS_API_KEY\" \\\n    https://<your-oms-host>/open-api/v1/me\n\nOpenAPI：/open-api/v1/openapi.json\n")
	case r.Method == http.MethodGet && (r.URL.Path == "/open-api/v1/" || r.URL.Path == "/open-api/v1/me"):
		identity, _ := r.Context().Value(openAPIContextKey{}).(openAPIIdentity)
		writeJSON(w, http.StatusOK, map[string]any{"version": "v1", "tenant_id": identity.TenantID, "subject_id": identity.SubjectID, "key_id": identity.KeyID, "key_prefix": identity.KeyPrefix, "capabilities": []string{openAPIUseAction}})
	case r.Method == http.MethodGet && r.URL.Path == "/open-api/v1/settings":
		writeJSON(w, http.StatusOK, map[string]any{"webhook": map[string]any{"available": false, "status": "not_available", "callback_url_editable": false}})
	default:
		if route, ok := describeOpenAPIRoute(r); ok {
			s.forwardOpenAPI(w, r, route)
		} else {
			requestID := w.Header().Get("X-Request-ID")
			openAPIProblem(w, http.StatusNotFound, "ROUTE_NOT_FOUND", "开放 API 路由不存在或尚未开放", requestID, false, nil)
		}
	}
	return true
}

func publicAPIKey(r *http.Request) string {
	raw := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(raw) > 7 && strings.EqualFold(raw[:7], "ApiKey ") {
		return strings.TrimSpace(raw[7:])
	}
	return ""
}

func openAPIAction(r *http.Request) string {
	switch r.URL.Path {
	case "/open-api/v1/", "/open-api/v1/me":
		return "open-api.profile.read"
	case "/open-api/v1/settings":
		return "open-api.settings.read"
	default:
		if route, ok := describeOpenAPIRoute(r); ok {
			return route.action
		}
		return ""
	}
}

func allowedOpenAPIRoute(r *http.Request) bool {
	if r.Method == http.MethodGet && (r.URL.Path == "/open-api/v1/" || r.URL.Path == "/open-api/v1/me" || r.URL.Path == "/open-api/v1/settings") {
		return true
	}
	_, ok := describeOpenAPIRoute(r)
	return ok
}

type openAPIRoute struct{ action, applicationID, upstreamPath, ownerAction string }

func describeOpenAPIRoute(r *http.Request) (openAPIRoute, bool) {
	path := strings.TrimPrefix(r.URL.Path, "/open-api/v1")
	if path == "/evidence/upload-sessions" && r.Method == http.MethodPost {
		return openAPIRoute{"open-api.evidence.upload.create", "evidence-service", "/v1/upload-sessions", "evidence-service.upload"}, true
	}
	if strings.HasPrefix(path, "/evidence/upload-sessions/") && r.Method == http.MethodPost {
		tail := strings.TrimPrefix(path, "/evidence/upload-sessions/")
		parts := strings.Split(tail, "/")
		if len(parts) == 2 && validOpenAPIUUID(parts[0]) && (parts[1] == "complete" || parts[1] == "abort") {
			return openAPIRoute{"open-api.evidence.upload." + parts[1], "evidence-service", "/v1/upload-sessions/" + parts[0] + "/" + parts[1], "evidence-service.upload"}, true
		}
	}
	if path == "/reimbursements" {
		switch r.Method {
		case http.MethodPost:
			return openAPIRoute{"open-api.reimbursement.create", "internal-finance-requests", "/api/v1/open-api/reimbursements", "internal-finance-requests.self.manage"}, true
		case http.MethodGet:
			return openAPIRoute{"open-api.reimbursement.list", "internal-finance-requests", "/api/v1/open-api/reimbursements", "internal-finance-requests.self.read"}, true
		}
	}
	if strings.HasPrefix(path, "/reimbursements/") {
		parts := strings.Split(strings.TrimPrefix(path, "/reimbursements/"), "/")
		if len(parts) == 1 && validOpenAPIUUID(parts[0]) {
			action := "read"
			owner := "internal-finance-requests.self.read"
			if r.Method == http.MethodPut {
				action, owner = "update", "internal-finance-requests.self.manage"
			} else if r.Method == http.MethodDelete {
				action, owner = "delete", "internal-finance-requests.self.manage"
			} else if r.Method != http.MethodGet {
				return openAPIRoute{}, false
			}
			return openAPIRoute{"open-api.reimbursement." + action, "internal-finance-requests", "/api/v1/open-api/reimbursements/" + parts[0], owner}, true
		}
		if len(parts) == 2 && validOpenAPIUUID(parts[0]) && r.Method == http.MethodPost && (parts[1] == "submit" || parts[1] == "withdraw" || parts[1] == "restore") {
			return openAPIRoute{"open-api.reimbursement." + parts[1], "internal-finance-requests", "/api/v1/open-api/reimbursements/" + parts[0] + "/" + parts[1], "internal-finance-requests.self.manage"}, true
		}
	}
	return openAPIRoute{}, false
}

func ownerForAction(action string) string {
	if strings.HasPrefix(action, "open-api.evidence.") {
		return "evidence-service"
	}
	if strings.HasPrefix(action, "open-api.reimbursement.") {
		return "internal-finance-requests"
	}
	return ""
}

func openAPIActionRequiresPerson(action string) bool {
	return strings.HasPrefix(action, "open-api.reimbursement.")
}

func validOpenAPIUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, char := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if char != '-' {
				return false
			}
			continue
		}
		if !strings.ContainsRune("0123456789abcdef", char) {
			return false
		}
	}
	return true
}

func (s *Server) forwardOpenAPI(w http.ResponseWriter, r *http.Request, route openAPIRoute) {
	identity, ok := r.Context().Value(openAPIContextKey{}).(openAPIIdentity)
	requestID := w.Header().Get("X-Request-ID")
	if !ok || identity.OwnerAudience != route.applicationID {
		openAPIProblem(w, http.StatusUnauthorized, "INVALID_API_KEY", "API Key 缺失、无效或不可用", requestID, false, nil)
		return
	}
	claims, err := s.verifier.Verify(identity.OwnerCredential, route.applicationID, route.ownerAction)
	if err != nil || claims.Subject != identity.SubjectID || claims.Tenant != identity.TenantID || claims.Scope.Kind != "tenant" {
		openAPIProblem(w, http.StatusUnauthorized, "INVALID_API_KEY", "API Key 缺失、无效或不可用", requestID, false, nil)
		return
	}
	application, found := s.suite.Application(route.applicationID)
	if !found || !application.Enabled {
		openAPIProblem(w, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "业务服务暂不可用", requestID, true, nil)
		return
	}
	base, err := url.Parse(application.Upstream)
	if err != nil {
		openAPIProblem(w, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "业务服务暂不可用", requestID, true, nil)
		return
	}
	target := base.ResolveReference(&url.URL{Path: route.upstreamPath, RawQuery: r.URL.RawQuery})
	request, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), http.MaxBytesReader(w, r.Body, 2<<20))
	if err != nil {
		openAPIProblem(w, http.StatusBadRequest, "INVALID_REQUEST", "请求无效", requestID, false, nil)
		return
	}
	request.Header.Set("Authorization", "Bearer "+identity.OwnerCredential)
	request.Header.Set("X-Request-ID", requestID)
	for _, header := range []string{"Content-Type", "Accept", "Idempotency-Key", "If-None-Match"} {
		if value := r.Header.Get(header); value != "" {
			request.Header.Set(header, value)
		}
	}
	response, err := s.client.Do(request)
	if err != nil {
		openAPIProblem(w, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "业务服务暂不可用", requestID, true, nil)
		return
	}
	defer response.Body.Close()
	for _, header := range []string{"Content-Type", "ETag", "Location", "Retry-After"} {
		if value := response.Header.Get(header); value != "" {
			w.Header().Set(header, value)
		}
	}
	if response.StatusCode < 400 {
		w.WriteHeader(response.StatusCode)
		_, _ = io.Copy(w, io.LimitReader(response.Body, 2<<20))
		return
	}
	var upstream struct {
		Code      string `json:"code"`
		Detail    string `json:"detail"`
		Message   string `json:"message"`
		Retryable bool   `json:"retryable"`
		Error     *struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			Retryable bool   `json:"retryable"`
		} `json:"error"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64<<10))
	_ = decoder.Decode(&upstream)
	if upstream.Error != nil {
		upstream.Code, upstream.Message, upstream.Retryable = upstream.Error.Code, upstream.Error.Message, upstream.Error.Retryable
	}
	if upstream.Code == "" {
		upstream.Code = fmt.Sprintf("UPSTREAM_HTTP_%d", response.StatusCode)
	}
	if upstream.Message == "" {
		upstream.Message = upstream.Detail
	}
	if upstream.Message == "" {
		upstream.Message = "请求未能完成"
	}
	openAPIProblem(w, response.StatusCode, strings.ToUpper(upstream.Code), upstream.Message, requestID, upstream.Retryable, nil)
}

func requestScheme(r *http.Request, trustProxy bool) string {
	if value := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))); trustProxy && value != "" {
		return value
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func maskedClientIP(r *http.Request, trustProxy bool) string {
	host := rateLimitClientKey(r, trustProxy)
	ip := net.ParseIP(host)
	if ip == nil {
		return "unknown"
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return fmt.Sprintf("%d.%d.%d.0/24", ipv4[0], ipv4[1], ipv4[2])
	}
	value := ip.To16()
	if value == nil {
		return "unknown"
	}
	for index := 8; index < len(value); index++ {
		value[index] = 0
	}
	return value.String() + "/64"
}

func openAPIProblem(w http.ResponseWriter, status int, code, message, requestID string, retryable bool, fields []map[string]string) {
	openAPIProblemWithRecovery(w, status, code, message, requestID, retryable, fields, nil)
}

func openAPIProblemWithRecovery(w http.ResponseWriter, status int, code, message, requestID string, retryable bool, fields []map[string]string, recovery map[string]string) {
	if fields == nil {
		fields = []map[string]string{}
	}
	problem := map[string]any{"code": code, "message": message, "request_id": requestID, "retryable": retryable, "field_errors": fields}
	if recovery != nil {
		problem["recovery"] = recovery
	}
	writeJSON(w, status, problem)
}
