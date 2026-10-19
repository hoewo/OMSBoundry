package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"omsbizfoundry/party-directory/internal/auth"
	"omsbizfoundry/party-directory/internal/party"
)

const maxRequestBytes = 1 << 20

type UseCases interface {
	Ping(context.Context) error
	Create(context.Context, party.Actor, party.CreateInput) (party.Party, error)
	List(context.Context, string, party.ListFilter, string) (party.Page, error)
	Get(context.Context, string, string) (party.Party, error)
	Reference(context.Context, string, string) (party.Reference, error)
	History(context.Context, string, string, *int64, int) (party.HistoryPage, error)
	Update(context.Context, party.Actor, string, party.UpdateInput) (party.Party, error)
	ChangeStatus(context.Context, party.Actor, string, party.StatusInput) (party.Party, error)
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
	server := &Server{useCases: useCases, authenticator: authenticator}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", server.live)
	mux.HandleFunc("GET /health/ready", server.ready)
	mux.HandleFunc("GET /assets/demo.js", partyDemoScript)
	mux.HandleFunc("GET /", server.index)
	mux.Handle("GET /api/v1/parties", server.protected(auth.ActionRead, http.HandlerFunc(server.list)))
	mux.Handle("POST /api/v1/parties", server.protected(auth.ActionCreate, http.HandlerFunc(server.create)))
	mux.Handle("GET /api/v1/parties/{party_id}", server.protected(auth.ActionRead, http.HandlerFunc(server.get)))
	mux.Handle("PUT /api/v1/parties/{party_id}", server.protected(auth.ActionUpdate, http.HandlerFunc(server.update)))
	mux.Handle("POST /api/v1/parties/{party_id}/status-changes", server.protected(auth.ActionStatusManage, http.HandlerFunc(server.changeStatus)))
	mux.Handle("GET /api/v1/parties/{party_id}/history", server.protected(auth.ActionHistoryRead, http.HandlerFunc(server.history)))
	mux.Handle("GET /api/v1/references/parties/{party_id}", server.protected(auth.ActionReferenceRead, http.HandlerFunc(server.reference)))
	server.handler = securityHeaders(mux)
	return server, nil
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }
func (s *Server) index(w http.ResponseWriter, _ *http.Request)     { renderPartyDirectory(w) }
func (s *Server) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"service": "party-directory", "status": "live"})
}
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.useCases.Ping(r.Context()); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "NOT_READY", "数据库暂不可用", true)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"service": "party-directory", "status": "ready"})
}
func (s *Server) protected(action string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stripUntrustedIdentityHeaders(r.Header)
		principal, err := s.authenticator.Authenticate(r)
		if err != nil {
			writeProblem(w, http.StatusUnauthorized, "UNAUTHENTICATED", "需要有效访问凭证", false)
			return
		}
		if !principal.Can(action) {
			writeProblem(w, http.StatusForbidden, "CAPABILITY_DENIED", "缺少 "+action, false)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, principal)))
	})
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	principal := principalFrom(r)
	if !principal.CanEnumerate() {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", "登记主体需要租户范围", false)
		return
	}
	var input party.CreateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.Create(r.Context(), actorFrom(principal), input)
	s.writeParty(w, http.StatusCreated, value, principal, err)
}
func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	principal := principalFrom(r)
	if !principal.CanEnumerate() {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", "列表查询需要租户范围", false)
		return
	}
	limit, ok := queryInt(w, r, "limit", 0)
	if !ok {
		return
	}
	filter := party.ListFilter{Query: r.URL.Query().Get("query"), Kind: r.URL.Query().Get("kind"), Status: r.URL.Query().Get("status"), Limit: limit}
	page, err := s.useCases.List(r.Context(), principal.TenantID, filter, r.URL.Query().Get("cursor"))
	if err != nil {
		s.fail(w, err)
		return
	}
	for i := range page.Items {
		page.Items[i].AllowedActions = allowedActions(page.Items[i], principal)
	}
	page.AllowedActions = listAllowedActions(principal)
	writeJSON(w, http.StatusOK, page)
}
func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	principal := principalFrom(r)
	id := strings.ToLower(strings.TrimSpace(r.PathValue("party_id")))
	if !principal.CanParty(id) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", "凭证范围不包含该 Party", false)
		return
	}
	value, err := s.useCases.Get(r.Context(), principal.TenantID, id)
	s.writeParty(w, http.StatusOK, value, principal, err)
}
func (s *Server) reference(w http.ResponseWriter, r *http.Request) {
	principal := principalFrom(r)
	id := strings.ToLower(strings.TrimSpace(r.PathValue("party_id")))
	if principal.ScopeKind != "resources" || !principal.CanParty(id) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", "引用读取只接受精确 Party 范围", false)
		return
	}
	value, err := s.useCases.Reference(r.Context(), principal.TenantID, id)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) update(w http.ResponseWriter, r *http.Request) {
	principal := principalFrom(r)
	id := strings.ToLower(strings.TrimSpace(r.PathValue("party_id")))
	if !principal.CanParty(id) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", "凭证范围不包含该 Party", false)
		return
	}
	var input party.UpdateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.Update(r.Context(), actorFrom(principal), id, input)
	s.writeParty(w, http.StatusOK, value, principal, err)
}
func (s *Server) changeStatus(w http.ResponseWriter, r *http.Request) {
	principal := principalFrom(r)
	id := strings.ToLower(strings.TrimSpace(r.PathValue("party_id")))
	if !principal.CanParty(id) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", "凭证范围不包含该 Party", false)
		return
	}
	var input party.StatusInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.ChangeStatus(r.Context(), actorFrom(principal), id, input)
	s.writeParty(w, http.StatusOK, value, principal, err)
}
func (s *Server) history(w http.ResponseWriter, r *http.Request) {
	principal := principalFrom(r)
	id := strings.ToLower(strings.TrimSpace(r.PathValue("party_id")))
	if !principal.CanParty(id) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", "凭证范围不包含该 Party", false)
		return
	}
	limit, ok := queryInt(w, r, "limit", 0)
	if !ok {
		return
	}
	var before *int64
	if raw := r.URL.Query().Get("before_revision"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "INVALID_REQUEST", "before_revision 必须是整数", false)
			return
		}
		before = &value
	}
	value, err := s.useCases.History(r.Context(), principal.TenantID, id, before, limit)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) writeParty(w http.ResponseWriter, status int, value party.Party, principal auth.Principal, err error) {
	if err != nil {
		s.fail(w, err)
		return
	}
	value.AllowedActions = allowedActions(value, principal)
	writeJSON(w, status, value)
}
func (s *Server) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, party.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "PARTY_NOT_FOUND", err.Error(), false)
	case errors.Is(err, party.ErrRevisionConflict):
		writeProblem(w, http.StatusConflict, "PARTY_REVISION_CONFLICT", err.Error(), true)
	case errors.Is(err, party.ErrCommandConflict):
		writeProblem(w, http.StatusConflict, "COMMAND_PAYLOAD_CONFLICT", err.Error(), false)
	case errors.Is(err, party.ErrNoEffectiveChange):
		writeProblem(w, http.StatusUnprocessableEntity, "NO_EFFECTIVE_CHANGE", err.Error(), false)
	case errors.Is(err, party.ErrInactive):
		writeProblem(w, http.StatusConflict, "PARTY_INACTIVE", err.Error(), false)
	default:
		if strings.Contains(err.Error(), "invalid") {
			writeProblem(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false)
		} else {
			writeProblem(w, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "服务暂不可用", true)
		}
	}
}
func allowedActions(value party.Party, principal auth.Principal) []string {
	if !principal.CanParty(value.ID) {
		return nil
	}
	var actions []string
	if principal.Can(auth.ActionRead) {
		actions = append(actions, "read")
	}
	if principal.Can(auth.ActionUpdate) {
		actions = append(actions, "update")
	}
	if principal.Can(auth.ActionStatusManage) {
		actions = append(actions, "status_manage")
	}
	if principal.Can(auth.ActionHistoryRead) {
		actions = append(actions, "history_read")
	}
	return actions
}
func listAllowedActions(principal auth.Principal) []string {
	var actions []string
	if principal.Can(auth.ActionRead) && principal.CanEnumerate() {
		actions = append(actions, auth.ActionRead)
	}
	if principal.Can(auth.ActionCreate) && principal.CanEnumerate() {
		actions = append(actions, auth.ActionCreate)
	}
	return actions
}
func actorFrom(principal auth.Principal) party.Actor {
	kind := "iam_subject"
	if principal.Provider == "template" {
		kind = "template_subject"
	}
	return party.Actor{ID: principal.SubjectID, TenantID: principal.TenantID, DisplayName: principal.DisplayName, Kind: kind, CredentialID: principal.CredentialID}
}
func principalFrom(r *http.Request) auth.Principal {
	return r.Context().Value(principalKey{}).(auth.Principal)
}
func queryInt(w http.ResponseWriter, r *http.Request, name string, fallback int) (int, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "INVALID_REQUEST", name+" 必须是整数", false)
		return 0, false
	}
	return value, true
}
func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	reader := http.MaxBytesReader(w, r.Body, maxRequestBytes)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeProblem(w, http.StatusBadRequest, "INVALID_REQUEST", "JSON 请求无效", false)
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		writeProblem(w, http.StatusBadRequest, "INVALID_REQUEST", "请求只能包含一个 JSON 值", false)
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
