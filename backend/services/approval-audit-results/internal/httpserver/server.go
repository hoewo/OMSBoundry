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
	"reflect"
	"strings"
	"time"

	"omsbizfoundry/approval-audit-results/internal/auth"
	"omsbizfoundry/approval-audit-results/internal/results"
)

const maxRequestBytes = 1 << 20

type UseCases interface {
	Ping(context.Context) error
	Record(context.Context, results.Actor, results.RecordInput) (results.Result, error)
	Get(context.Context, string, string) (results.Result, error)
	ListProject(context.Context, string, string) ([]results.Result, error)
	ReferenceProjectResults(context.Context, string, string) (results.ProjectResultReferenceList, error)
	History(context.Context, string, string) ([]results.Revision, error)
	Correct(context.Context, results.Actor, string, results.CorrectInput) (results.Result, error)
	Withdraw(context.Context, results.Actor, string, results.WithdrawInput) (results.Result, error)
}

type Server struct {
	useCases UseCases
	handler  http.Handler
}
type requestIDKey struct{}

func New(useCases UseCases, authenticator auth.Authenticator) (*Server, error) {
	if useCases == nil || authenticator == nil {
		return nil, fmt.Errorf("use cases and authenticator are required")
	}
	server := &Server{useCases: useCases}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", server.live)
	mux.HandleFunc("GET /health/ready", server.ready)
	mux.HandleFunc("GET /assets/results.js", resultsWorkspaceScript)
	mux.HandleFunc("GET /", server.index)
	mux.Handle("GET /api/v1/approval-audit-results", Protect(authenticator, auth.ActionRead, http.HandlerFunc(server.listProject)))
	mux.Handle("POST /api/v1/approval-audit-results", Protect(authenticator, auth.ActionCreate, http.HandlerFunc(server.record)))
	mux.Handle("GET /api/v1/approval-audit-results/{result_id}", Protect(authenticator, auth.ActionRead, http.HandlerFunc(server.get)))
	mux.Handle("GET /api/v1/approval-audit-results/{result_id}/history", Protect(authenticator, auth.ActionHistoryRead, http.HandlerFunc(server.history)))
	mux.Handle("POST /api/v1/approval-audit-results/{result_id}/corrections", Protect(authenticator, auth.ActionCorrect, http.HandlerFunc(server.correct)))
	mux.Handle("POST /api/v1/approval-audit-results/{result_id}/withdrawal", Protect(authenticator, auth.ActionWithdraw, http.HandlerFunc(server.withdraw)))
	mux.Handle("GET /api/v1/references/projects/{project_id}/approval-audit-results", Protect(authenticator, auth.ActionRead, http.HandlerFunc(server.referenceProjectResults)))
	mux.Handle("GET /api/v1/references/approval-audit-results/{result_id}/materials", Protect(authenticator, auth.ActionRead, http.HandlerFunc(server.referenceMaterials)))
	server.handler = requestContext(securityHeaders(mux))
	return server, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }

func (s *Server) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"service": "approval-audit-results", "status": "live"})
}
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.useCases.Ping(r.Context()); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "NOT_READY", true)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"service": "approval-audit-results", "status": "ready"})
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	renderResultsWorkspace(w)
}

func (s *Server) listProject(w http.ResponseWriter, r *http.Request) {
	projectID := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("project_id")))
	principal := PrincipalFrom(r)
	if !results.ValidUUID(projectID) {
		writeProblem(w, http.StatusBadRequest, "INVALID_REQUEST", false)
		return
	}
	if !principal.CanResource("project", projectID) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", false)
		return
	}
	values, err := s.useCases.ListProject(r.Context(), principal.TenantID, projectID)
	if err != nil {
		s.fail(w, err)
		return
	}
	items := make([]resultView, 0, len(values))
	for _, value := range values {
		items = append(items, viewResult(value, principal))
	}
	allowed := make([]string, 0, len(auth.UserActions))
	for _, action := range auth.UserActions {
		if principal.Can(action) {
			allowed = append(allowed, action)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"project_id": projectID, "items": items, "allowed_actions": allowed})
}

func (s *Server) referenceProjectResults(w http.ResponseWriter, r *http.Request) {
	projectID := strings.ToLower(strings.TrimSpace(r.PathValue("project_id")))
	principal := PrincipalFrom(r)
	if !principal.CanResource("project", projectID) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", false)
		return
	}
	value, err := s.useCases.ReferenceProjectResults(r.Context(), principal.TenantID, projectID)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) record(w http.ResponseWriter, r *http.Request) {
	var input results.RecordInput
	if !decodeJSON(w, r, &input) {
		return
	}
	principal := PrincipalFrom(r)
	if !principal.CanResource("project", strings.ToLower(strings.TrimSpace(input.ProjectRef.ID))) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", false)
		return
	}
	if len(input.AmountComponents) > 0 && !principal.Can(auth.ActionAmountRead) {
		writeProblem(w, http.StatusForbidden, "CAPABILITY_DENIED", false)
		return
	}
	if len(input.EvidenceRefs) > 0 && !principal.Can(auth.ActionEvidenceManage) {
		writeProblem(w, http.StatusForbidden, "CAPABILITY_DENIED", false)
		return
	}
	value, err := s.useCases.Record(r.Context(), actorFrom(principal), input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, viewResult(value, principal))
}

func (s *Server) referenceMaterials(w http.ResponseWriter, r *http.Request) {
	principal := PrincipalFrom(r)
	value, ok := s.authorizeCurrent(w, r, principal)
	if !ok {
		return
	}
	// Do not let a malformed/custom store bypass the trusted Tenant or ID.
	if value.TenantID != principal.TenantID || value.ID != r.PathValue("result_id") {
		s.fail(w, results.ErrReferenceInvalid)
		return
	}
	projection, err := results.MaterialReference(value)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, projection)
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	principal := PrincipalFrom(r)
	value, ok := s.authorizeCurrent(w, r, principal)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, viewResult(value, principal))
}

func (s *Server) history(w http.ResponseWriter, r *http.Request) {
	principal := PrincipalFrom(r)
	current, ok := s.authorizeCurrent(w, r, principal)
	if !ok {
		return
	}
	values, err := s.useCases.History(r.Context(), principal.TenantID, current.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	items := make([]revisionView, 0, len(values))
	for _, value := range values {
		items = append(items, revisionView{ResultID: value.ResultID, Revision: value.Revision, EventKind: value.EventKind, CommandID: value.CommandID, Snapshot: viewResult(value.Snapshot, principal), Reason: value.Reason, ActorRef: value.ActorRef, OccurredAt: value.OccurredAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) correct(w http.ResponseWriter, r *http.Request) {
	var input results.CorrectInput
	if !decodeJSON(w, r, &input) {
		return
	}
	principal := PrincipalFrom(r)
	current, ok := s.authorizeCurrent(w, r, principal)
	if !ok {
		return
	}
	if len(input.AmountComponents) > 0 && !principal.Can(auth.ActionAmountRead) {
		writeProblem(w, http.StatusForbidden, "CAPABILITY_DENIED", false)
		return
	}
	if evidenceChanged(current.Evidence, input.EvidenceRefs) && !principal.Can(auth.ActionEvidenceManage) {
		writeProblem(w, http.StatusForbidden, "CAPABILITY_DENIED", false)
		return
	}
	value, err := s.useCases.Correct(r.Context(), actorFrom(principal), current.ID, input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, viewResult(value, principal))
}

func (s *Server) withdraw(w http.ResponseWriter, r *http.Request) {
	var input results.WithdrawInput
	if !decodeJSON(w, r, &input) {
		return
	}
	principal := PrincipalFrom(r)
	current, ok := s.authorizeCurrent(w, r, principal)
	if !ok {
		return
	}
	value, err := s.useCases.Withdraw(r.Context(), actorFrom(principal), current.ID, input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, viewResult(value, principal))
}

func (s *Server) authorizeCurrent(w http.ResponseWriter, r *http.Request, principal auth.Principal) (results.Result, bool) {
	value, err := s.useCases.Get(r.Context(), principal.TenantID, r.PathValue("result_id"))
	if err != nil {
		s.fail(w, err)
		return results.Result{}, false
	}
	if !principal.CanResult(value.ID, value.Project.ID) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", false)
		return results.Result{}, false
	}
	return value, true
}

func evidenceChanged(current []results.EvidenceSnapshot, requested []results.EvidenceRef) bool {
	refs := make([]results.EvidenceRef, 0, len(current))
	for _, value := range current {
		refs = append(refs, value.EvidenceRef)
	}
	return !reflect.DeepEqual(refs, requested)
}

func actorFrom(principal auth.Principal) results.Actor {
	kind := "iam_subject"
	if principal.Provider == "template" {
		kind = "template_subject"
	}
	return results.Actor{ID: principal.SubjectID, TenantID: principal.TenantID, Kind: kind, CredentialID: principal.CredentialID}
}

type sourceView struct {
	Application      string                     `json:"application"`
	ResourceType     string                     `json:"resource_type"`
	ResourceID       string                     `json:"resource_id"`
	Revision         int64                      `json:"revision"`
	State            string                     `json:"state"`
	ProjectID        string                     `json:"project_id"`
	Currency         *string                    `json:"currency,omitempty"`
	AmountProfile    *string                    `json:"amount_profile,omitempty"`
	AmountComponents *[]results.AmountComponent `json:"amount_components,omitempty"`
	TotalAmountMinor *int64                     `json:"total_amount_minor,omitempty"`
	Digest           string                     `json:"digest"`
	DisplaySummary   string                     `json:"display_summary"`
}

type resultView struct {
	ID                  string                     `json:"id"`
	TenantID            string                     `json:"tenant_id"`
	Project             results.ProjectSnapshot    `json:"project"`
	Kind                string                     `json:"kind"`
	State               string                     `json:"state"`
	AmountProfile       *string                    `json:"amount_profile,omitempty"`
	Currency            *string                    `json:"currency,omitempty"`
	AmountComponents    *[]results.AmountComponent `json:"amount_components,omitempty"`
	TotalAmountMinor    *int64                     `json:"total_amount_minor,omitempty"`
	Source              sourceView                 `json:"source"`
	Evidence            []results.EvidenceSnapshot `json:"evidence"`
	OverrideReason      string                     `json:"override_reason,omitempty"`
	ExternalConfirmedBy string                     `json:"external_confirmed_by"`
	ExternalConfirmedAt time.Time                  `json:"external_confirmed_at"`
	Revision            int64                      `json:"revision"`
	CreatedAt           time.Time                  `json:"created_at"`
	UpdatedAt           time.Time                  `json:"updated_at"`
	AllowedActions      []string                   `json:"allowed_actions"`
}

type revisionView struct {
	ResultID   string     `json:"result_id"`
	Revision   int64      `json:"revision"`
	EventKind  string     `json:"event_kind"`
	CommandID  string     `json:"command_id"`
	Snapshot   resultView `json:"snapshot"`
	Reason     string     `json:"reason"`
	ActorRef   string     `json:"actor_ref"`
	OccurredAt time.Time  `json:"occurred_at"`
}

func viewResult(value results.Result, principal auth.Principal) resultView {
	// Evidence is a collection in the public contract. Keep the zero-value
	// response stable as [] rather than null so clients can validate and replay
	// a legitimate result that has no attached report yet.
	view := resultView{ID: value.ID, TenantID: value.TenantID, Project: value.Project, Kind: value.Kind, State: value.State, Evidence: append([]results.EvidenceSnapshot{}, value.Evidence...), OverrideReason: value.OverrideReason, ExternalConfirmedBy: value.ExternalConfirmedBy, ExternalConfirmedAt: value.ExternalConfirmedAt, Revision: value.Revision, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, AllowedActions: allowedActions(value, principal)}
	view.Source = sourceView{Application: value.Source.Application, ResourceType: value.Source.ResourceType, ResourceID: value.Source.ResourceID, Revision: value.Source.Revision, State: value.Source.State, ProjectID: value.Source.ProjectID, Digest: value.Source.Digest, DisplaySummary: value.Source.DisplaySummary}
	if principal.Can(auth.ActionAmountRead) {
		profile, currency, total := value.AmountProfile, value.Currency, value.TotalAmountMinor
		components := append([]results.AmountComponent(nil), value.AmountComponents...)
		sourceCurrency, sourceProfile, sourceTotal := value.Source.Currency, value.Source.AmountProfile, value.Source.TotalAmountMinor
		sourceComponents := append([]results.AmountComponent(nil), value.Source.AmountComponents...)
		view.AmountProfile, view.Currency, view.TotalAmountMinor, view.AmountComponents = &profile, &currency, &total, &components
		view.Source.Currency, view.Source.AmountProfile, view.Source.TotalAmountMinor, view.Source.AmountComponents = &sourceCurrency, &sourceProfile, &sourceTotal, &sourceComponents
	}
	return view
}

func allowedActions(value results.Result, principal auth.Principal) []string {
	actions := make([]string, 0, len(auth.UserActions))
	for _, action := range auth.UserActions {
		if !principal.Can(action) {
			continue
		}
		if (action == auth.ActionCorrect || action == auth.ActionWithdraw || action == auth.ActionEvidenceManage) && value.State != results.StateRecorded {
			continue
		}
		actions = append(actions, action)
	}
	return actions
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	if contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type"))); contentType != "" && !strings.HasPrefix(contentType, "application/json") {
		writeProblem(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", false)
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeProblem(w, http.StatusBadRequest, "INVALID_JSON", false)
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeProblem(w, http.StatusBadRequest, "INVALID_JSON", false)
		return false
	}
	return true
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, results.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "RESULT_NOT_FOUND", false)
	case errors.Is(err, results.ErrRevisionConflict):
		writeProblem(w, http.StatusConflict, "REVISION_CONFLICT", false)
	case errors.Is(err, results.ErrCommandConflict):
		writeProblem(w, http.StatusConflict, "COMMAND_PAYLOAD_CONFLICT", false)
	case errors.Is(err, results.ErrAlreadyRecorded):
		writeProblem(w, http.StatusConflict, "RESULT_ALREADY_RECORDED", false)
	case errors.Is(err, results.ErrResultWithdrawn):
		writeProblem(w, http.StatusConflict, "RESULT_WITHDRAWN", false)
	case errors.Is(err, results.ErrNoEffectiveChange):
		writeProblem(w, http.StatusUnprocessableEntity, "NO_EFFECTIVE_CHANGE", false)
	case errors.Is(err, results.ErrReferenceInvalid):
		writeProblem(w, http.StatusUnprocessableEntity, "REFERENCE_INVALID", false)
	case errors.Is(err, results.ErrReferenceUnavailable):
		writeProblem(w, http.StatusServiceUnavailable, "REFERENCE_SERVICE_UNAVAILABLE", true)
	case requestError(err):
		writeProblem(w, http.StatusBadRequest, "INVALID_REQUEST", false)
	default:
		writeProblem(w, http.StatusInternalServerError, "INTERNAL_ERROR", true)
	}
}

func requestError(err error) bool {
	message := err.Error()
	return strings.HasPrefix(message, "invalid ") || strings.HasPrefix(message, "amount profile ") || strings.Contains(message, " is required") || strings.HasPrefix(message, "duplicate ")
}

func requestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Request-ID")))
		if !results.ValidUUID(requestID) {
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

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func newUUID() string {
	var raw [16]byte
	_, _ = rand.Read(raw[:])
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(raw[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}
