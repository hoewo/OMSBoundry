package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"time"

	"omsbizfoundry/evidence-service/internal/auth"
	"omsbizfoundry/evidence-service/internal/evidence"
	"omsbizfoundry/evidence-service/internal/storage"
)

type UseCases interface {
	Ping(context.Context) error
	ListVersions(context.Context, string, int) (evidence.VersionPage, error)
	CreateUploadSession(context.Context, evidence.Actor, string, evidence.CreateUploadInput) (evidence.UploadSession, error)
	CreateVersionUploadSession(context.Context, evidence.Actor, string, string, evidence.CreateUploadInput) (evidence.UploadSession, error)
	CompleteUpload(context.Context, evidence.Actor, string, string, evidence.CompleteUploadInput) (evidence.Version, error)
	AbortUpload(context.Context, evidence.Actor, string, string, evidence.AbortUploadInput) error
	CreateAccessGrant(context.Context, evidence.Actor, string, int, evidence.AccessInput) (evidence.AccessGrant, error)
	GetVersion(context.Context, string, string, int) (evidence.Version, error)
	GetLatestVersion(context.Context, string, string) (evidence.Version, error)
	GetUploadIntentByGlobalID(context.Context, string) (storage.UploadIntent, string, error)
	GetVersionByGlobalID(context.Context, string, int) (evidence.Version, error)
}

type Server struct {
	useCases       UseCases
	authenticator  auth.Authenticator
	local          storage.LocalDataPlane
	provider       storage.Provider
	maxUploadBytes int64
	handler        http.Handler
}

func New(useCases UseCases, authenticator auth.Authenticator, provider storage.Provider, maxUploadBytes int64) (*Server, error) {
	if useCases == nil || authenticator == nil || provider == nil || maxUploadBytes <= 0 {
		return nil, fmt.Errorf("HTTP server dependencies are required")
	}
	s := &Server{useCases: useCases, authenticator: authenticator, provider: provider, maxUploadBytes: maxUploadBytes}
	if local, ok := provider.(storage.LocalDataPlane); ok {
		s.local = local
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.index)
	mux.HandleFunc("GET /assets/demo.js", evidenceDemoScript)
	mux.HandleFunc("GET /health/live", s.live)
	mux.HandleFunc("GET /health/ready", s.ready)
	mux.HandleFunc("GET /v1/files", s.protectedAny([]string{auth.ActionMetadataRead, auth.ActionView}, s.listFiles))
	mux.HandleFunc("POST /v1/upload-sessions", s.protected(auth.ActionUpload, s.createUpload))
	mux.HandleFunc("POST /v1/files/{file_id}/versions/upload-sessions", s.protected(auth.ActionUpload, s.createVersionUpload))
	mux.HandleFunc("POST /v1/upload-sessions/{session_id}/complete", s.protected(auth.ActionUpload, s.completeUpload))
	mux.HandleFunc("POST /v1/upload-sessions/{session_id}/abort", s.protected(auth.ActionUpload, s.abortUpload))
	mux.HandleFunc("POST /v1/files/{file_id}/versions/{version}/access-grants", s.protectedAny([]string{auth.ActionDownload, auth.ActionView}, s.accessGrant))
	mux.HandleFunc("GET /v1/references/files/{file_id}", s.protectedAny([]string{auth.ActionMetadataRead, auth.ActionView}, s.fileReference))
	mux.HandleFunc("GET /v1/references/files/{file_id}/versions/{version}", s.protectedAny([]string{auth.ActionMetadataRead, auth.ActionView}, s.fileVersionReference))
	mux.HandleFunc("PUT /v1/local-uploads/{session_id}", s.localUpload)
	mux.HandleFunc("GET /v1/local-files/{file_id}/versions/{version}", s.localDownload)
	s.handler = securityHeaders(mux)
	return s, nil
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }
func (s *Server) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]string{"service": "evidence-service", "status": "live"})
}
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.useCases.Ping(r.Context()); err != nil {
		writeProblem(w, 503, "not_ready", "Evidence Service 暂不可用", true)
		return
	}
	mode := s.provider.Name()
	if configured, ok := s.provider.(interface{ StorageMode() string }); ok {
		mode = configured.StorageMode()
	}
	writeJSON(w, 200, map[string]string{"service": "evidence-service", "status": "ready", "storage_provider": s.provider.Name(), "storage_mode": mode})
}

type principalKey struct{}

func (s *Server) protected(action string, next http.HandlerFunc) http.HandlerFunc {
	return s.protectedAny([]string{action}, next)
}

func (s *Server) protectedAny(actions []string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, err := s.authenticator.Authenticate(r)
		if err != nil {
			writeProblem(w, 401, "credential_required", "需要有效访问凭证", false)
			return
		}
		allowed := false
		for _, action := range actions {
			if principal.Can(action) {
				allowed = true
				break
			}
		}
		if !allowed {
			writeProblem(w, 403, "capability_denied", "缺少所需文件能力", false)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, principal)))
	}
}
func principalFrom(r *http.Request) auth.Principal {
	return r.Context().Value(principalKey{}).(auth.Principal)
}
func actorFrom(p auth.Principal) evidence.Actor {
	kind := "iam_subject"
	if p.Provider == "template" {
		kind = "template_subject"
	}
	return evidence.Actor{TenantID: p.TenantID, SubjectID: p.SubjectID, DisplayName: p.DisplayName, Kind: kind}
}

func (s *Server) createUpload(w http.ResponseWriter, r *http.Request) {
	var input evidence.CreateUploadInput
	if !decodeJSON(w, r, 1<<20, &input) {
		return
	}
	result, err := s.useCases.CreateUploadSession(r.Context(), actorFrom(principalFrom(r)), r.Header.Get("Idempotency-Key"), input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 201, result)
}
func (s *Server) createVersionUpload(w http.ResponseWriter, r *http.Request) {
	principal := principalFrom(r)
	if !principal.CanFile(r.PathValue("file_id")) {
		writeProblem(w, http.StatusForbidden, "scope_denied", "凭证范围不包含该文件", false)
		return
	}
	var input evidence.CreateUploadInput
	if !decodeJSON(w, r, 1<<20, &input) {
		return
	}
	result, err := s.useCases.CreateVersionUploadSession(r.Context(), actorFrom(principal), r.PathValue("file_id"), r.Header.Get("Idempotency-Key"), input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}
func (s *Server) listFiles(w http.ResponseWriter, r *http.Request) {
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			s.fail(w, evidence.ErrInvalid)
			return
		}
		limit = value
	}
	principal := principalFrom(r)
	result, err := s.useCases.ListVersions(r.Context(), principal.TenantID, limit)
	if err != nil {
		s.fail(w, err)
		return
	}
	visible := result.Items[:0]
	for _, item := range result.Items {
		if principal.CanFile(item.FileID) {
			visible = append(visible, item)
		}
	}
	result.Items = visible
	writeJSON(w, http.StatusOK, result)
}
func (s *Server) completeUpload(w http.ResponseWriter, r *http.Request) {
	var input evidence.CompleteUploadInput
	if !decodeJSON(w, r, 2<<20, &input) {
		return
	}
	result, err := s.useCases.CompleteUpload(r.Context(), actorFrom(principalFrom(r)), r.PathValue("session_id"), r.Header.Get("Idempotency-Key"), input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) abortUpload(w http.ResponseWriter, r *http.Request) {
	var input evidence.AbortUploadInput
	if !decodeJSON(w, r, 64<<10, &input) {
		return
	}
	err := s.useCases.AbortUpload(r.Context(), actorFrom(principalFrom(r)), r.PathValue("session_id"), r.Header.Get("Idempotency-Key"), input)
	if err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) accessGrant(w http.ResponseWriter, r *http.Request) {
	version, err := strconv.Atoi(r.PathValue("version"))
	if err != nil {
		s.fail(w, evidence.ErrInvalid)
		return
	}
	if !principalFrom(r).CanFile(r.PathValue("file_id")) {
		writeProblem(w, http.StatusForbidden, "scope_denied", "凭证范围不包含该文件", false)
		return
	}
	var input evidence.AccessInput
	if !decodeJSON(w, r, 64<<10, &input) {
		return
	}
	result, err := s.useCases.CreateAccessGrant(r.Context(), actorFrom(principalFrom(r)), r.PathValue("file_id"), version, input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) fileReference(w http.ResponseWriter, r *http.Request) {
	principal := principalFrom(r)
	if !principal.CanFile(r.PathValue("file_id")) {
		writeProblem(w, http.StatusForbidden, "scope_denied", "凭证范围不包含该文件", false)
		return
	}
	value, err := s.useCases.GetLatestVersion(r.Context(), principal.TenantID, r.PathValue("file_id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	s.writeFileReference(w, principal.TenantID, value)
}

func (s *Server) fileVersionReference(w http.ResponseWriter, r *http.Request) {
	version, err := strconv.Atoi(r.PathValue("version"))
	if err != nil {
		s.fail(w, evidence.ErrInvalid)
		return
	}
	s.writeFileVersionReference(w, r, version)
}

func (s *Server) writeFileVersionReference(w http.ResponseWriter, r *http.Request, version int) {
	principal := principalFrom(r)
	if !principal.CanFile(r.PathValue("file_id")) {
		writeProblem(w, http.StatusForbidden, "scope_denied", "凭证范围不包含该文件", false)
		return
	}
	value, err := s.useCases.GetVersion(r.Context(), principal.TenantID, r.PathValue("file_id"), version)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.writeFileReference(w, principal.TenantID, value)
}

func (s *Server) writeFileReference(w http.ResponseWriter, tenantID string, value evidence.Version) {
	writeJSON(w, http.StatusOK, map[string]any{
		"schema_version":     "1",
		"source_application": "evidence-service",
		"id":                 value.FileID,
		"file_id":            value.FileID,
		"tenant_id":          tenantID,
		"resource_type":      "evidence-file",
		"status":             value.Status,
		"version":            value.Version,
		"revision":           value.Version,
		"display_name":       value.FileName,
		"display_summary":    value.FileName,
		"file_name":          value.FileName,
		"size_bytes":         value.SizeBytes,
		"media_type":         value.MediaType,
		"sha256":             value.SHA256,
	})
}

func (s *Server) localUpload(w http.ResponseWriter, r *http.Request) {
	if s.local == nil {
		http.NotFound(w, r)
		return
	}
	intent, status, err := s.useCases.GetUploadIntentByGlobalID(r.Context(), r.PathValue("session_id"))
	if err != nil {
		s.failDataPlane(w, err)
		return
	}
	if status != "pending" || !intent.ExpiresAt.After(time.Now().UTC()) {
		writeProblem(w, 409, "upload_not_pending", "上传会话不可写入", false)
		return
	}
	if err := s.local.VerifyUploadToken(r.URL.Query().Get("token"), intent); err != nil {
		s.failDataPlane(w, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.maxUploadBytes+1)
	etag, err := s.local.Put(r.Context(), intent, r.Body, r.ContentLength)
	if err != nil {
		s.failDataPlane(w, err)
		return
	}
	w.Header().Set("ETag", fmt.Sprintf("\"%s\"", etag))
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) localDownload(w http.ResponseWriter, r *http.Request) {
	if s.local == nil {
		http.NotFound(w, r)
		return
	}
	version, err := strconv.Atoi(r.PathValue("version"))
	if err != nil {
		s.failDataPlane(w, evidence.ErrInvalid)
		return
	}
	value, err := s.useCases.GetVersionByGlobalID(r.Context(), r.PathValue("file_id"), version)
	if err != nil {
		s.failDataPlane(w, err)
		return
	}
	intent := storage.AccessIntent{FileID: value.FileID, Version: value.Version, FileName: r.URL.Query().Get("file_name"), Disposition: r.URL.Query().Get("disposition"), Object: value.Object}
	if err := s.local.VerifyAccessToken(r.URL.Query().Get("token"), intent); err != nil {
		s.failDataPlane(w, err)
		return
	}
	object, err := s.provider.Open(r.Context(), value.Object)
	if err != nil {
		s.failDataPlane(w, err)
		return
	}
	defer object.Reader.Close()
	seeker, ok := object.Reader.(io.ReadSeeker)
	if !ok {
		writeProblem(w, 500, "storage_contract_error", "本地存储不支持范围读取", false)
		return
	}
	w.Header().Set("Content-Type", value.MediaType)
	disposition := mime.FormatMediaType(intent.Disposition, map[string]string{"filename": intent.FileName})
	w.Header().Set("Content-Disposition", disposition)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, intent.FileName, object.Metadata.LastModified, seeker)
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, evidence.ErrInvalid):
		writeProblem(w, 422, "invalid_request", "请求内容无效", false)
	case errors.Is(err, evidence.ErrNotFound), errors.Is(err, storage.ErrNotFound):
		writeProblem(w, 404, "not_found", "文件或上传会话不存在", false)
	case errors.Is(err, evidence.ErrIdempotencyConflict):
		writeProblem(w, 409, "idempotency_conflict", "幂等键已用于不同请求", false)
	case errors.Is(err, evidence.ErrExpired), errors.Is(err, storage.ErrExpiredAuthorization):
		writeProblem(w, 409, "authorization_expired", "上传授权已过期", false)
	case errors.Is(err, evidence.ErrConflict), errors.Is(err, storage.ErrObjectExists):
		writeProblem(w, 409, "state_conflict", "当前状态不允许该操作", false)
	default:
		writeProblem(w, 500, "internal_error", "Evidence Service 处理失败", true)
	}
}
func (s *Server) failDataPlane(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, storage.ErrInvalidAuthorization):
		writeProblem(w, 401, "invalid_storage_authorization", "文件访问授权无效", false)
	case errors.Is(err, storage.ErrExpiredAuthorization):
		writeProblem(w, 401, "storage_authorization_expired", "文件访问授权已过期", false)
	default:
		s.fail(w, err)
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, limit int64, destination any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeProblem(w, 400, "invalid_json", "请求 JSON 无效", false)
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		writeProblem(w, 400, "invalid_json", "请求只能包含一个 JSON 对象", false)
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeProblem(w http.ResponseWriter, status int, code, message string, retryable bool) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "retryable": retryable}})
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'unsafe-inline'; connect-src 'self' https: blob:; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}
