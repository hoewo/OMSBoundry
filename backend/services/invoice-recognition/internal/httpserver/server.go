package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	contract "omsbizfoundry/invoice-recognition-contract"
	"omsbizfoundry/invoice-recognition/internal/auth"
	"omsbizfoundry/invoice-recognition/internal/recognition"
)

const maxRequestBytes = 1 << 20

type UseCases interface {
	Ping(context.Context) error
	Start(context.Context, recognition.Actor, recognition.StartInput) (contract.Proposal, error)
	Get(context.Context, string, string) (contract.Proposal, error)
	List(context.Context, string) ([]contract.Proposal, error)
	Confirm(context.Context, recognition.Actor, recognition.ConfirmInput) (contract.Confirmation, error)
	Discard(context.Context, recognition.Actor, recognition.DiscardInput) (contract.Proposal, error)
}

type Server struct {
	useCases      UseCases
	authenticator auth.Authenticator
	handler       http.Handler
}

type principalKey struct{}

func New(useCases UseCases, authenticator auth.Authenticator) (*Server, error) {
	if useCases == nil || authenticator == nil {
		return nil, fmt.Errorf("invoice recognition use cases and authenticator are required")
	}
	server := &Server{useCases: useCases, authenticator: authenticator}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", server.live)
	mux.HandleFunc("GET /health/ready", server.ready)
	for _, prefix := range []string{"/api/v1", ""} {
		mux.Handle("GET "+prefix+"/proposals", server.protected(auth.ActionTenantRead, http.HandlerFunc(server.list)))
		mux.Handle("POST "+prefix+"/proposals", server.protected(auth.ActionStart, http.HandlerFunc(server.start)))
		mux.Handle("GET "+prefix+"/proposals/{proposal_id}", server.protected(auth.ActionTenantRead, http.HandlerFunc(server.get)))
		mux.Handle("POST "+prefix+"/proposals/{proposal_id}/confirm", server.protected(auth.ActionConfirm, http.HandlerFunc(server.confirm)))
		mux.Handle("POST "+prefix+"/proposals/{proposal_id}/discard", server.protected(auth.ActionDiscard, http.HandlerFunc(server.discard)))
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
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, principal)))
	})
}

func principalFrom(r *http.Request) auth.Principal {
	return r.Context().Value(principalKey{}).(auth.Principal)
}

func actorFrom(principal auth.Principal) recognition.Actor {
	return recognition.Actor{
		SubjectID: principal.SubjectID, TenantID: principal.TenantID,
		DisplayName: principal.DisplayName, CredentialID: principal.CredentialID,
	}
}

func requireTenant(w http.ResponseWriter, principal auth.Principal) bool {
	if principal.ScopeKind == "tenant" {
		return true
	}
	writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", "该操作需要租户范围", false)
	return false
}

func requireResource(w http.ResponseWriter, principal auth.Principal, kind, id string) bool {
	if principal.CanResource(kind, id) {
		return true
	}
	writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", "凭证范围不包含该资源", false)
	return false
}

func (s *Server) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "live"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.useCases.Ping(r.Context()); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "NOT_READY", "发票识别数据库未就绪", true)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	principal := principalFrom(r)
	if !requireTenant(w, principal) {
		return
	}
	items, err := s.useCases.List(r.Context(), principal.TenantID)
	s.write(w, http.StatusOK, map[string]any{"items": items}, err)
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	principal := principalFrom(r)
	id := strings.ToLower(strings.TrimSpace(r.PathValue("proposal_id")))
	if !requireResource(w, principal, "proposal", id) {
		return
	}
	value, err := s.useCases.Get(r.Context(), principal.TenantID, id)
	s.write(w, http.StatusOK, value, err)
}

func (s *Server) start(w http.ResponseWriter, r *http.Request) {
	principal := principalFrom(r)
	var input recognition.StartInput
	if !decodeJSON(w, r, &input) || !requireResource(w, principal, "file", strings.ToLower(strings.TrimSpace(input.Evidence.FileID))) {
		return
	}
	value, err := s.useCases.Start(r.Context(), actorFrom(principal), input)
	s.write(w, http.StatusCreated, value, err)
}

func (s *Server) confirm(w http.ResponseWriter, r *http.Request) {
	principal := principalFrom(r)
	id := strings.ToLower(strings.TrimSpace(r.PathValue("proposal_id")))
	if !requireResource(w, principal, "proposal", id) {
		return
	}
	var input recognition.ConfirmInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.ProposalID = id
	value, err := s.useCases.Confirm(r.Context(), actorFrom(principal), input)
	s.write(w, http.StatusOK, value, err)
}

func (s *Server) discard(w http.ResponseWriter, r *http.Request) {
	principal := principalFrom(r)
	id := strings.ToLower(strings.TrimSpace(r.PathValue("proposal_id")))
	if !requireResource(w, principal, "proposal", id) {
		return
	}
	var input recognition.DiscardInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.ProposalID = id
	value, err := s.useCases.Discard(r.Context(), actorFrom(principal), input)
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
	case errors.Is(err, recognition.ErrInvalid):
		writeProblem(w, http.StatusUnprocessableEntity, "INVOICE_RECOGNITION_INVALID", "发票识别数据无效", false)
	case errors.Is(err, recognition.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "INVOICE_RECOGNITION_NOT_FOUND", "发票识别结果不存在", false)
	case errors.Is(err, recognition.ErrConflict), errors.Is(err, recognition.ErrIdempotencyConflict):
		writeProblem(w, http.StatusConflict, "INVOICE_RECOGNITION_CONFLICT", "识别结果已变化或命令冲突", false)
	case errors.Is(err, recognition.ErrEvidenceUnavailable):
		writeProblem(w, http.StatusServiceUnavailable, "EVIDENCE_UNAVAILABLE", "发票文件暂不可用", true)
	case errors.Is(err, recognition.ErrProviderUnavailable):
		writeProblem(w, http.StatusServiceUnavailable, "RECOGNITION_PROVIDER_UNAVAILABLE", "图片或 PDF 识别能力暂不可用", true)
	default:
		writeProblem(w, http.StatusInternalServerError, "INVOICE_RECOGNITION_INTERNAL", "发票识别服务暂不可用", true)
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
