package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"omsbizfoundry/payroll/internal/auth"
	domain "omsbizfoundry/payroll/internal/payroll"
	"strings"
)

const maxRequestBytes = 1 << 20

type UseCases interface {
	Ping(context.Context) error
	Create(context.Context, domain.Actor, domain.CreateInput) (domain.Run, error)
	Update(context.Context, domain.Actor, string, domain.UpdateInput) (domain.Run, error)
	Get(context.Context, string, string) (domain.Run, error)
	List(context.Context, string) (domain.Page, error)
}
type Server struct {
	useCases      UseCases
	authenticator auth.Authenticator
	handler       http.Handler
}
type principalKey struct{}

func New(useCases UseCases, authenticator auth.Authenticator) (*Server, error) {
	if useCases == nil || authenticator == nil {
		return nil, fmt.Errorf("use cases and authenticator are required")
	}
	s := &Server{useCases: useCases, authenticator: authenticator}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", s.live)
	mux.HandleFunc("GET /health/ready", s.ready)
	mux.Handle("POST /api/v1/payroll-runs", s.protected(auth.ActionRunManage, http.HandlerFunc(s.create)))
	mux.Handle("GET /api/v1/payroll-runs", s.protected(auth.ActionTenantRead, http.HandlerFunc(s.list)))
	mux.Handle("GET /api/v1/payroll-runs/{run_id}", s.protected(auth.ActionTenantRead, http.HandlerFunc(s.get)))
	mux.Handle("PUT /api/v1/payroll-runs/{run_id}", s.protected(auth.ActionRunManage, http.HandlerFunc(s.update)))
	mux.Handle("GET /api/v1/references/payroll-runs/{run_id}", s.protected(auth.ActionReferenceRead, http.HandlerFunc(s.reference)))
	// Gateway strips the public, versioned /payroll/v1 prefix.
	mux.Handle("POST /payroll-runs", s.protected(auth.ActionRunManage, http.HandlerFunc(s.create)))
	mux.Handle("GET /payroll-runs", s.protected(auth.ActionTenantRead, http.HandlerFunc(s.list)))
	mux.Handle("GET /payroll-runs/{run_id}", s.protected(auth.ActionTenantRead, http.HandlerFunc(s.get)))
	mux.Handle("PUT /payroll-runs/{run_id}", s.protected(auth.ActionRunManage, http.HandlerFunc(s.update)))
	mux.Handle("GET /references/payroll-runs/{run_id}", s.protected(auth.ActionReferenceRead, http.HandlerFunc(s.reference)))
	s.handler = securityHeaders(mux)
	return s, nil
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }
func (s *Server) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]string{"service": "payroll", "status": "live"})
}
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.useCases.Ping(r.Context()); err != nil {
		writeProblem(w, 503, "NOT_READY", "数据库暂不可用", true)
		return
	}
	writeJSON(w, 200, map[string]string{"service": "payroll", "status": "ready"})
}
func (s *Server) protected(action string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stripUntrustedIdentityHeaders(r.Header)
		principal, err := s.authenticator.Authenticate(r)
		if err != nil {
			writeProblem(w, 401, "UNAUTHENTICATED", "需要有效访问凭证", false)
			return
		}
		if !principal.Can(action) {
			writeProblem(w, 403, "CAPABILITY_DENIED", "缺少 "+action, false)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, principal)))
	})
}
func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if p.ScopeKind != "tenant" {
		writeProblem(w, 403, "SCOPE_DENIED", "新建工资批次需要租户范围", false)
		return
	}
	var input domain.CreateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.Create(r.Context(), actorFrom(p), input)
	s.writeRun(w, 201, value, p, err)
}
func (s *Server) update(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	id := strings.ToLower(strings.TrimSpace(r.PathValue("run_id")))
	if !p.CanResource(id) {
		writeProblem(w, 403, "SCOPE_DENIED", "凭证范围不包含该工资批次", false)
		return
	}
	var input domain.UpdateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.Update(r.Context(), actorFrom(p), id, input)
	s.writeRun(w, 200, value, p, err)
}
func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	id := strings.ToLower(strings.TrimSpace(r.PathValue("run_id")))
	if !p.CanResource(id) {
		writeProblem(w, 403, "SCOPE_DENIED", "凭证范围不包含该工资批次", false)
		return
	}
	value, err := s.useCases.Get(r.Context(), p.TenantID, id)
	s.writeRun(w, 200, value, p, err)
}

func (s *Server) reference(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	id := strings.ToLower(strings.TrimSpace(r.PathValue("run_id")))
	if p.ScopeKind != "resources" || !p.CanResource(id) {
		writeProblem(w, 403, "SCOPE_DENIED", "引用读取只接受精确工资批次范围", false)
		return
	}
	value, err := s.useCases.Get(r.Context(), p.TenantID, id)
	if err != nil {
		s.fail(w, err)
		return
	}
	value.AllowedActions = nil
	writeJSON(w, 200, value)
}
func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if p.ScopeKind != "tenant" {
		writeProblem(w, 403, "SCOPE_DENIED", "列表查询需要租户范围", false)
		return
	}
	page, err := s.useCases.List(r.Context(), p.TenantID)
	if err != nil {
		s.fail(w, err)
		return
	}
	for i := range page.Items {
		page.Items[i].AllowedActions = allowedActions(page.Items[i], p)
	}
	writeJSON(w, 200, page)
}
func (s *Server) writeRun(w http.ResponseWriter, status int, value domain.Run, p auth.Principal, err error) {
	if err != nil {
		s.fail(w, err)
		return
	}
	value.AllowedActions = allowedActions(value, p)
	writeJSON(w, status, value)
}
func (s *Server) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeProblem(w, 404, "PAYROLL_RUN_NOT_FOUND", err.Error(), false)
	case errors.Is(err, domain.ErrRevisionConflict):
		writeProblem(w, 409, "PAYROLL_RUN_REVISION_CONFLICT", err.Error(), true)
	case errors.Is(err, domain.ErrCommandConflict):
		writeProblem(w, 409, "COMMAND_PAYLOAD_CONFLICT", err.Error(), false)
	case strings.Contains(err.Error(), "invalid"):
		writeProblem(w, 400, "INVALID_REQUEST", err.Error(), false)
	default:
		writeProblem(w, 503, "DEPENDENCY_UNAVAILABLE", "服务暂不可用", true)
	}
}
func allowedActions(value domain.Run, p auth.Principal) []string {
	var actions []string
	if p.Can(auth.ActionTenantRead) {
		actions = append(actions, "read")
	}
	if p.Can(auth.ActionRunManage) && value.Status == "draft" {
		actions = append(actions, "update")
	}
	return actions
}
func actorFrom(p auth.Principal) domain.Actor {
	kind := "iam_subject"
	if p.Provider == "template" {
		kind = "template_subject"
	}
	return domain.Actor{ID: p.SubjectID, TenantID: p.TenantID, DisplayName: p.DisplayName, Kind: kind, CredentialID: p.CredentialID}
}
func principalFrom(r *http.Request) auth.Principal {
	return r.Context().Value(principalKey{}).(auth.Principal)
}
func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	reader := http.MaxBytesReader(w, r.Body, maxRequestBytes)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeProblem(w, 400, "INVALID_REQUEST", "JSON 请求无效", false)
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		writeProblem(w, 400, "INVALID_REQUEST", "请求只能包含一个 JSON 值", false)
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeProblem(w http.ResponseWriter, status int, code, detail string, retryable bool) {
	writeJSON(w, status, map[string]any{"code": code, "detail": detail, "retryable": retryable})
}
func stripUntrustedIdentityHeaders(header http.Header) {
	for name := range header {
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "x-oms-") || strings.HasPrefix(lower, "x-auth-") || strings.HasPrefix(lower, "x-user-") || lower == "x-tenant-id" {
			header.Del(name)
		}
	}
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
