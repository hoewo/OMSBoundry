package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	contract "omsbizfoundry/attendance-summary-contract"
	"omsbizfoundry/workforce-attendance/internal/attendance"
	"omsbizfoundry/workforce-attendance/internal/auth"
)

const maxRequestBytes = 1 << 20

type UseCases interface {
	Ping(context.Context) error
	UpsertPolicy(context.Context, attendance.Actor, attendance.UpsertPolicyInput) (attendance.PolicyRecord, error)
	GetPolicy(context.Context, string, string) (attendance.PolicyRecord, error)
	ListPolicies(context.Context, string) ([]attendance.PolicyRecord, error)
	RecordAttendance(context.Context, attendance.Actor, attendance.RecordAttendanceInput) (attendance.AttendanceRecordView, error)
	ListAttendance(context.Context, string, string, string) ([]attendance.AttendanceRecordView, error)
	RecordLeave(context.Context, attendance.Actor, attendance.RecordLeaveInput) (attendance.LeaveView, error)
	ListLeaves(context.Context, string, string, string) ([]attendance.LeaveView, error)
	CalculateSummary(context.Context, attendance.Actor, attendance.CalculateSummaryInput) (contract.MonthlySummary, error)
	GetSummary(context.Context, string, string) (contract.MonthlySummary, error)
	ListSummaries(context.Context, string, string) ([]contract.MonthlySummary, error)
}

type Server struct {
	useCases      UseCases
	authenticator auth.Authenticator
	handler       http.Handler
}
type principalKey struct{}

func New(useCases UseCases, authenticator auth.Authenticator) (*Server, error) {
	if useCases == nil || authenticator == nil {
		return nil, fmt.Errorf("attendance use cases and authenticator are required")
	}
	server := &Server{useCases: useCases, authenticator: authenticator}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", server.live)
	mux.HandleFunc("GET /health/ready", server.ready)
	for _, prefix := range []string{"/api/v1", ""} {
		mux.Handle("GET "+prefix+"/policies", server.protected(auth.ActionTenantRead, http.HandlerFunc(server.listPolicies)))
		mux.Handle("PUT "+prefix+"/policies/{policy_key}", server.protected(auth.ActionPolicyManage, http.HandlerFunc(server.upsertPolicy)))
		mux.Handle("GET "+prefix+"/attendance-records", server.protected(auth.ActionTenantRead, http.HandlerFunc(server.listAttendance)))
		mux.Handle("POST "+prefix+"/attendance-records", server.protected(auth.ActionRecordManage, http.HandlerFunc(server.recordAttendance)))
		mux.Handle("GET "+prefix+"/leave-records", server.protected(auth.ActionTenantRead, http.HandlerFunc(server.listLeaves)))
		mux.Handle("POST "+prefix+"/leave-records", server.protected(auth.ActionRecordManage, http.HandlerFunc(server.recordLeave)))
		mux.Handle("GET "+prefix+"/monthly-summaries", server.protected(auth.ActionSummaryRead, http.HandlerFunc(server.listSummaries)))
		mux.Handle("GET "+prefix+"/monthly-summaries/{summary_id}", server.protected(auth.ActionSummaryRead, http.HandlerFunc(server.getSummary)))
		mux.Handle("POST "+prefix+"/monthly-summaries:calculate", server.protected(auth.ActionSummaryCalculate, http.HandlerFunc(server.calculateSummary)))
	}
	server.handler = securityHeaders(mux)
	return server, nil
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }

func (s *Server) protected(action string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, err := s.authenticator.Authenticate(r)
		if err != nil {
			writeProblem(w, http.StatusUnauthorized, "CREDENTIAL_REQUIRED", "需要有效访问凭证", false)
			return
		}
		if !principal.Can(action) {
			writeProblem(w, http.StatusForbidden, "CAPABILITY_DENIED", "缺少 "+action, false)
			return
		}
		ctx := context.WithValue(r.Context(), principalKey{}, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func principalFrom(r *http.Request) auth.Principal {
	return r.Context().Value(principalKey{}).(auth.Principal)
}
func actorFrom(p auth.Principal) attendance.Actor {
	return attendance.Actor{TenantID: p.TenantID, SubjectID: p.SubjectID, DisplayName: p.DisplayName, CredentialID: p.CredentialID}
}
func requireTenant(w http.ResponseWriter, p auth.Principal) bool {
	if p.ScopeKind == "tenant" {
		return true
	}
	writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", "该操作需要租户范围", false)
	return false
}
func requirePerson(w http.ResponseWriter, p auth.Principal, personID string) bool {
	if p.CanResource("person", personID) {
		return true
	}
	writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", "凭证范围不包含该人员", false)
	return false
}

func (s *Server) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "live"})
}
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.useCases.Ping(r.Context()); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "NOT_READY", "考勤数据库未就绪", true)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) listPolicies(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if !requireTenant(w, p) {
		return
	}
	values, err := s.useCases.ListPolicies(r.Context(), p.TenantID)
	s.write(w, http.StatusOK, map[string]any{"items": values}, err)
}
func (s *Server) upsertPolicy(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if !requireTenant(w, p) {
		return
	}
	var input attendance.UpsertPolicyInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Key = r.PathValue("policy_key")
	value, err := s.useCases.UpsertPolicy(r.Context(), actorFrom(p), input)
	s.write(w, http.StatusOK, value, err)
}
func (s *Server) listAttendance(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	personID := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("person_id")))
	if personID == "" && !requireTenant(w, p) {
		return
	}
	if personID != "" && !requirePerson(w, p, personID) {
		return
	}
	values, err := s.useCases.ListAttendance(r.Context(), p.TenantID, personID, r.URL.Query().Get("period"))
	s.write(w, http.StatusOK, map[string]any{"items": values}, err)
}
func (s *Server) recordAttendance(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	var input attendance.RecordAttendanceInput
	if !decodeJSON(w, r, &input) || !requirePerson(w, p, strings.ToLower(strings.TrimSpace(input.PersonID))) {
		return
	}
	value, err := s.useCases.RecordAttendance(r.Context(), actorFrom(p), input)
	s.write(w, http.StatusCreated, value, err)
}
func (s *Server) listLeaves(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	personID := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("person_id")))
	if personID == "" && !requireTenant(w, p) {
		return
	}
	if personID != "" && !requirePerson(w, p, personID) {
		return
	}
	values, err := s.useCases.ListLeaves(r.Context(), p.TenantID, personID, r.URL.Query().Get("period"))
	s.write(w, http.StatusOK, map[string]any{"items": values}, err)
}
func (s *Server) recordLeave(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	var input attendance.RecordLeaveInput
	if !decodeJSON(w, r, &input) || !requirePerson(w, p, strings.ToLower(strings.TrimSpace(input.PersonID))) {
		return
	}
	value, err := s.useCases.RecordLeave(r.Context(), actorFrom(p), input)
	s.write(w, http.StatusCreated, value, err)
}
func (s *Server) calculateSummary(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	var input attendance.CalculateSummaryInput
	if !decodeJSON(w, r, &input) || !requirePerson(w, p, strings.ToLower(strings.TrimSpace(input.PersonID))) {
		return
	}
	value, err := s.useCases.CalculateSummary(r.Context(), actorFrom(p), input)
	s.write(w, http.StatusOK, value, err)
}
func (s *Server) listSummaries(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if !requireTenant(w, p) {
		return
	}
	values, err := s.useCases.ListSummaries(r.Context(), p.TenantID, r.URL.Query().Get("period"))
	s.write(w, http.StatusOK, map[string]any{"items": values}, err)
}
func (s *Server) getSummary(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	id := strings.ToLower(strings.TrimSpace(r.PathValue("summary_id")))
	if !p.CanResource("monthly_summary", id) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", "凭证范围不包含该月度摘要", false)
		return
	}
	value, err := s.useCases.GetSummary(r.Context(), p.TenantID, id)
	s.write(w, http.StatusOK, value, err)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeProblem(w, http.StatusBadRequest, "INVALID_JSON", "请求 JSON 无效", false)
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeProblem(w, http.StatusBadRequest, "INVALID_JSON", "请求只能包含一个 JSON 值", false)
		return false
	}
	return true
}
func (s *Server) write(w http.ResponseWriter, status int, value any, err error) {
	if err == nil {
		writeJSON(w, status, value)
		return
	}
	switch {
	case errors.Is(err, attendance.ErrInvalid):
		writeProblem(w, http.StatusUnprocessableEntity, "ATTENDANCE_INVALID", "考勤数据无效", false)
	case errors.Is(err, attendance.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "ATTENDANCE_NOT_FOUND", "考勤数据不存在", false)
	case errors.Is(err, attendance.ErrConflict), errors.Is(err, attendance.ErrIdempotencyConflict):
		writeProblem(w, http.StatusConflict, "ATTENDANCE_CONFLICT", "考勤数据已变化或命令冲突", false)
	case errors.Is(err, attendance.ErrDependency):
		writeProblem(w, http.StatusServiceUnavailable, "ATTENDANCE_DEPENDENCY_UNAVAILABLE", "人员任职资料暂不可用", true)
	default:
		writeProblem(w, http.StatusInternalServerError, "ATTENDANCE_INTERNAL", "考勤服务暂不可用", true)
	}
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeProblem(w http.ResponseWriter, status int, code, detail string, retryable bool) {
	writeJSON(w, status, map[string]any{"code": code, "detail": detail, "retryable": retryable})
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
