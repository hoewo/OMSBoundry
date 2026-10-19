package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"omsbizfoundry/business-engagement/internal/auth"
	"omsbizfoundry/business-engagement/internal/engagement"
)

const maxRequestBytes = 1 << 20

type UseCases interface {
	Ping(context.Context) error
	List(context.Context, string, int) ([]engagement.Engagement, error)
	ListProject(context.Context, string, string, int) ([]engagement.Engagement, error)
	Get(context.Context, string, string) (engagement.Details, error)
	ReferenceMaterials(context.Context, string, string) (engagement.MaterialReferenceProjection, error)
	History(context.Context, string, string, int64, int) (engagement.HistoryPage, error)
	Create(context.Context, engagement.Actor, engagement.CreateInput) (engagement.Details, error)
	SetApplicability(context.Context, engagement.Actor, string, engagement.ApplicabilityInput) (engagement.Details, error)
	SelectMethod(context.Context, engagement.Actor, string, engagement.MethodInput) (engagement.Details, error)
	ApplyMaterial(context.Context, engagement.Actor, string, engagement.MaterialInput) (engagement.Details, error)
	Cancel(context.Context, engagement.Actor, string, engagement.CancelInput) (engagement.Details, error)
}

type Server struct {
	useCases      UseCases
	authenticator auth.Authenticator
	handler       http.Handler
}

type principalKey struct{}
type requestIDKey struct{}

func New(useCases UseCases, authenticator auth.Authenticator) (*Server, error) {
	if useCases == nil || authenticator == nil {
		return nil, fmt.Errorf("use cases and authenticator are required")
	}
	server := &Server{useCases: useCases, authenticator: authenticator}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", server.live)
	mux.HandleFunc("GET /health/ready", server.ready)
	mux.HandleFunc("GET /assets/demo.js", businessEngagementDemoScript)
	mux.HandleFunc("GET /", server.index)
	mux.Handle("GET /api/v1/business-engagements", server.protected(auth.ActionRead, http.HandlerFunc(server.list)))
	mux.Handle("POST /api/v1/business-engagements", server.protected(auth.ActionCreate, http.HandlerFunc(server.create)))
	mux.Handle("GET /api/v1/business-engagements/{engagement_id}", server.protected(auth.ActionRead, http.HandlerFunc(server.get)))
	mux.Handle("GET /api/v1/references/business-engagements/{engagement_id}/materials", server.protected(auth.ActionMaterialReferenceRead, http.HandlerFunc(server.referenceMaterials)))
	mux.Handle("POST /api/v1/business-engagements/{engagement_id}/applicability", server.protected(auth.ActionApplicabilityManage, http.HandlerFunc(server.setApplicability)))
	mux.Handle("POST /api/v1/business-engagements/{engagement_id}/method", server.protected(auth.ActionMethodSelect, http.HandlerFunc(server.selectMethod)))
	mux.Handle("POST /api/v1/business-engagements/{engagement_id}/materials", server.protected(auth.ActionEvidenceManage, http.HandlerFunc(server.applyMaterial)))
	mux.Handle("POST /api/v1/business-engagements/{engagement_id}/cancel", server.protected(auth.ActionCancel, http.HandlerFunc(server.cancel)))
	mux.Handle("GET /api/v1/business-engagements/{engagement_id}/history", server.protected(auth.ActionHistoryRead, http.HandlerFunc(server.history)))
	server.handler = requestContext(securityHeaders(mux))
	return server, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	renderBusinessEngagementWorkspace(w)
}

func (s *Server) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"service": "business-engagement", "status": "live"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.useCases.Ping(r.Context()); err != nil {
		writeProblem(w, r, http.StatusServiceUnavailable, "NOT_READY", "数据库暂不可用", true)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"service": "business-engagement", "status": "ready"})
}

func (s *Server) protected(action string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stripUntrustedIdentityHeaders(r.Header)
		principal, err := s.authenticator.Authenticate(r)
		if err != nil {
			writeProblem(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "需要有效访问凭证", false)
			return
		}
		if !principal.Can(action) {
			writeProblem(w, r, http.StatusForbidden, "CAPABILITY_DENIED", "缺少 "+action, false)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, principal)))
	})
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	var input engagement.CreateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	principal := principalFrom(r)
	if !principal.CanResource("project", strings.ToLower(strings.TrimSpace(input.ProjectID))) {
		writeProblem(w, r, http.StatusForbidden, "SCOPE_DENIED", "凭证范围不包含该 Project", false)
		return
	}
	value, err := s.useCases.Create(r.Context(), actorFrom(principal), input)
	s.writeResult(w, r, http.StatusCreated, value, err)
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			writeProblem(w, r, http.StatusBadRequest, "INVALID_REQUEST", "limit 必须是整数", false)
			return
		}
		limit = value
	}
	if limit < 0 || limit > 100 {
		writeProblem(w, r, http.StatusBadRequest, "INVALID_REQUEST", "limit 必须在 1 到 100 之间，或使用默认值 0", false)
		return
	}
	principal := principalFrom(r)
	projectID := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("project_id")))
	if r.URL.Query().Has("project_id") && !engagement.ValidUUID(projectID) {
		writeProblem(w, r, http.StatusBadRequest, "INVALID_REQUEST", "project_id 必须是有效 UUID", false)
		return
	}
	// Exact-project queries still undergo per-record native authorization below.
	var items []engagement.Engagement
	var err error
	if projectID != "" {
		items, err = s.useCases.ListProject(r.Context(), principal.TenantID, projectID, limit)
	} else {
		items, err = s.useCases.List(r.Context(), principal.TenantID, limit)
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	visible := items[:0]
	for _, item := range items {
		if (projectID == "" || item.ProjectID == projectID) && principal.CanEngagement(item.ID, item.ProjectID) {
			visible = append(visible, item)
		}
	}
	writeJSON(w, http.StatusOK, engagementListResponse{Items: visible, ProjectID: projectID, AllowedActions: credentialActions(principal)})
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	value, ok := s.authorizeEngagement(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, detailsResponse(value, principalFrom(r)))
}

func (s *Server) referenceMaterials(w http.ResponseWriter, r *http.Request) {
	principal := principalFrom(r)
	value, err := s.useCases.ReferenceMaterials(r.Context(), principal.TenantID, r.PathValue("engagement_id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if !principal.CanEngagement(value.ID, value.ProjectID) {
		writeProblem(w, r, http.StatusForbidden, "SCOPE_DENIED", "凭证范围不包含该 Business Engagement", false)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

type engagementListResponse struct {
	Items          []engagement.Engagement `json:"items"`
	ProjectID      string                  `json:"project_id,omitempty"`
	AllowedActions []string                `json:"allowed_actions"`
}

type engagementDetailsResponse struct {
	Engagement          engagement.Engagement          `json:"engagement"`
	Materials           []engagement.MaterialReference `json:"materials"`
	Requirements        []engagement.Requirement       `json:"requirements"`
	MissingRequirements []string                       `json:"missing_requirements"`
	AllowedActions      []string                       `json:"allowed_actions"`
}

func detailsResponse(value engagement.Details, principal auth.Principal) engagementDetailsResponse {
	return engagementDetailsResponse{
		Engagement:          value.Engagement,
		Materials:           value.Materials,
		Requirements:        value.Requirements,
		MissingRequirements: value.MissingRequirements,
		AllowedActions:      detailsAllowedActions(value, principal),
	}
}

func credentialActions(principal auth.Principal) []string {
	actions := make([]string, 0, len(auth.AllActions))
	for _, action := range auth.AllActions {
		if principal.Can(action) {
			actions = append(actions, action)
		}
	}
	return actions
}

func detailsAllowedActions(value engagement.Details, principal auth.Principal) []string {
	current := value.Engagement
	mutable := current.Status != engagement.StatusCancelled
	applicable := current.Applicability == engagement.ApplicabilityApplicable
	hasMethod := current.Method != nil
	applicableActions := map[string]bool{
		auth.ActionRead:                true,
		auth.ActionApplicabilityManage: mutable,
		auth.ActionMethodSelect:        mutable && applicable,
		auth.ActionEvidenceManage:      mutable && applicable && hasMethod,
		auth.ActionCancel:              mutable,
		auth.ActionHistoryRead:         true,
	}
	actions := make([]string, 0, len(applicableActions))
	for _, action := range auth.AllActions {
		if applicableActions[action] && principal.Can(action) {
			actions = append(actions, action)
		}
	}
	return actions
}

func (s *Server) setApplicability(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authorizeEngagement(w, r); !ok {
		return
	}
	var input engagement.ApplicabilityInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.SetApplicability(r.Context(), actorFrom(principalFrom(r)), r.PathValue("engagement_id"), input)
	s.writeResult(w, r, http.StatusOK, value, err)
}

func (s *Server) selectMethod(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authorizeEngagement(w, r); !ok {
		return
	}
	var input engagement.MethodInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.SelectMethod(r.Context(), actorFrom(principalFrom(r)), r.PathValue("engagement_id"), input)
	s.writeResult(w, r, http.StatusOK, value, err)
}

func (s *Server) applyMaterial(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authorizeEngagement(w, r); !ok {
		return
	}
	var input engagement.MaterialInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.ApplyMaterial(r.Context(), actorFrom(principalFrom(r)), r.PathValue("engagement_id"), input)
	s.writeResult(w, r, http.StatusOK, value, err)
}

func (s *Server) cancel(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authorizeEngagement(w, r); !ok {
		return
	}
	var input engagement.CancelInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.Cancel(r.Context(), actorFrom(principalFrom(r)), r.PathValue("engagement_id"), input)
	s.writeResult(w, r, http.StatusOK, value, err)
}

func (s *Server) history(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authorizeEngagement(w, r); !ok {
		return
	}
	after, limit := int64(0), 0
	var err error
	if raw := r.URL.Query().Get("after_sequence"); raw != "" {
		after, err = strconv.ParseInt(raw, 10, 64)
	}
	if err == nil {
		if raw := r.URL.Query().Get("limit"); raw != "" {
			limit, err = strconv.Atoi(raw)
		}
	}
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "INVALID_REQUEST", "分页参数无效", false)
		return
	}
	principal := principalFrom(r)
	value, err := s.useCases.History(r.Context(), principal.TenantID, r.PathValue("engagement_id"), after, limit)
	s.writeResult(w, r, http.StatusOK, value, err)
}

func (s *Server) authorizeEngagement(w http.ResponseWriter, r *http.Request) (engagement.Details, bool) {
	principal := principalFrom(r)
	value, err := s.useCases.Get(r.Context(), principal.TenantID, r.PathValue("engagement_id"))
	if err != nil {
		s.fail(w, r, err)
		return engagement.Details{}, false
	}
	if !principal.CanEngagement(value.Engagement.ID, value.Engagement.ProjectID) {
		writeProblem(w, r, http.StatusForbidden, "SCOPE_DENIED", "凭证范围不包含该 Business Engagement", false)
		return engagement.Details{}, false
	}
	return value, true
}

func actorFrom(principal auth.Principal) engagement.Actor {
	kind := "iam_subject"
	if principal.Provider == "template" {
		kind = "template_subject"
	}
	return engagement.Actor{Kind: kind, ID: principal.SubjectID, DisplayName: principal.DisplayName, TenantID: principal.TenantID}
}

func principalFrom(r *http.Request) auth.Principal {
	return r.Context().Value(principalKey{}).(auth.Principal)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	if contentType := r.Header.Get("Content-Type"); contentType != "" && !strings.HasPrefix(strings.ToLower(contentType), "application/json") {
		writeProblem(w, r, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "请求体必须使用 application/json", false)
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "INVALID_JSON", "JSON 请求体无效", false)
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeProblem(w, r, http.StatusBadRequest, "INVALID_JSON", "JSON 请求体只能包含一个对象", false)
		return false
	}
	return true
}

func (s *Server) writeResult(w http.ResponseWriter, r *http.Request, status int, value any, err error) {
	if err == nil {
		writeJSON(w, status, value)
		return
	}
	s.fail(w, r, err)
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, engagement.ErrNotFound):
		writeProblem(w, r, http.StatusNotFound, "BUSINESS_ENGAGEMENT_NOT_FOUND", "Business Engagement 不存在", false)
	case errors.Is(err, engagement.ErrRevisionConflict):
		writeProblem(w, r, http.StatusConflict, "REVISION_CONFLICT", "Business Engagement revision 已变化", true)
	case errors.Is(err, engagement.ErrProjectConflict):
		writeProblem(w, r, http.StatusConflict, "PROJECT_ENGAGEMENT_CONFLICT", "该 Project 已有 Business Engagement", false)
	case errors.Is(err, engagement.ErrCommandConflict):
		writeProblem(w, r, http.StatusConflict, "COMMAND_PAYLOAD_CONFLICT", "command_id 已用于不同请求", false)
	case errors.Is(err, engagement.ErrStateConflict):
		writeProblem(w, r, http.StatusConflict, "ENGAGEMENT_STATE_CONFLICT", "当前 Business Engagement 状态不允许该操作", false)
	case errors.Is(err, engagement.ErrReferenceInvalid):
		writeProblem(w, r, http.StatusUnprocessableEntity, "REFERENCE_INVALID", "外部引用不存在、跨租户、已失效或 revision 不匹配", false)
	case errors.Is(err, engagement.ErrReferenceUnavailable):
		writeProblem(w, r, http.StatusServiceUnavailable, "REFERENCE_SERVICE_UNAVAILABLE", "外部引用服务暂不可用", true)
	default:
		if strings.HasPrefix(err.Error(), "invalid ") || strings.Contains(err.Error(), " is required") || strings.Contains(err.Error(), " must be ") || strings.Contains(err.Error(), "between") {
			writeProblem(w, r, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false)
			return
		}
		writeProblem(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Business Engagement 暂时无法完成请求", true)
	}
}

func requestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Request-ID")))
		if !engagement.ValidUUID(requestID) {
			requestID = newUUID()
		}
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, requestID)))
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'unsafe-inline'; connect-src 'self'; frame-ancestors 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func stripUntrustedIdentityHeaders(header http.Header) {
	for _, name := range []string{"X-Actor-ID", "X-Actor-Name", "X-Tenant-ID", "X-IAM-Subject", "X-Allowed-Actions", "X-Capability-Scope"} {
		header.Del(name)
	}
}

func writeProblem(w http.ResponseWriter, r *http.Request, status int, code, message string, retryable bool) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "retryable": retryable, "request_id": requestIDFrom(r)}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func requestIDFrom(r *http.Request) string {
	value, _ := r.Context().Value(requestIDKey{}).(string)
	return value
}

func newUUID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(err)
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	raw := hex.EncodeToString(value[:])
	return raw[:8] + "-" + raw[8:12] + "-" + raw[12:16] + "-" + raw[16:20] + "-" + raw[20:]
}
