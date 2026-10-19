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

	"omsbizfoundry/billing-invoice/internal/auth"
	"omsbizfoundry/billing-invoice/internal/invoice"
)

const maxRequestBytes = 1 << 20

type UseCases interface {
	Ping(context.Context) error
	Create(context.Context, invoice.Actor, invoice.CreateInput) (invoice.Registration, error)
	Get(context.Context, string, string) (invoice.Registration, error)
	List(context.Context, string, invoice.ListQuery) (invoice.ListPage, error)
	History(context.Context, string, string, *int64, int) (invoice.HistoryPage, error)
	Correct(context.Context, invoice.Actor, string, invoice.CorrectInput) (invoice.Registration, error)
	Invalidate(context.Context, invoice.Actor, string, invoice.InvalidateInput) (invoice.Registration, error)
	ReplaceEvidence(context.Context, invoice.Actor, string, invoice.EvidenceInput) (invoice.Registration, error)
	Reference(context.Context, string, string) (invoice.Reference, error)
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
	mux.HandleFunc("GET /assets/demo.js", billingDemoScript)
	mux.HandleFunc("GET /", server.index)
	mux.Handle("POST /api/v1/invoice-registrations", Protect(authenticator, auth.ActionRecord, http.HandlerFunc(server.create)))
	mux.Handle("GET /api/v1/invoice-registrations", Protect(authenticator, auth.ActionRead, http.HandlerFunc(server.list)))
	mux.Handle("GET /api/v1/invoice-registrations/{invoice_registration_id}", Protect(authenticator, auth.ActionRead, http.HandlerFunc(server.get)))
	mux.Handle("GET /api/v1/invoice-registrations/{invoice_registration_id}/export-view", Protect(authenticator, auth.ActionExport, http.HandlerFunc(server.exportView)))
	mux.Handle("GET /api/v1/invoice-registrations/{invoice_registration_id}/history", Protect(authenticator, auth.ActionHistoryRead, http.HandlerFunc(server.history)))
	mux.Handle("POST /api/v1/invoice-registrations/{invoice_registration_id}/corrections", Protect(authenticator, auth.ActionCorrect, http.HandlerFunc(server.correct)))
	mux.Handle("POST /api/v1/invoice-registrations/{invoice_registration_id}/invalidation", Protect(authenticator, auth.ActionRegistrationInvalidate, http.HandlerFunc(server.invalidate)))
	mux.Handle("PUT /api/v1/invoice-registrations/{invoice_registration_id}/evidence", Protect(authenticator, auth.ActionEvidenceManage, http.HandlerFunc(server.replaceEvidence)))
	mux.Handle("GET /api/v1/references/invoice-registrations/{invoice_registration_id}", Protect(authenticator, auth.ActionReferenceRead, http.HandlerFunc(server.reference)))
	server.handler = requestContext(securityHeaders(mux))
	return server, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }

func (s *Server) exportView(w http.ResponseWriter, r *http.Request) {
	principal := PrincipalFrom(r)
	if !principal.Can(auth.ActionAmountRead) {
		writeProblem(w, http.StatusForbidden, "AMOUNT_READ_DENIED", false)
		return
	}
	s.get(w, r)
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	renderBillingWorkspace(w)
}

func (s *Server) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"service": "billing-invoice", "status": "live"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.useCases.Ping(r.Context()); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "NOT_READY", true)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"service": "billing-invoice", "status": "ready"})
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	var input invoice.CreateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	principal := PrincipalFrom(r)
	if !principal.Can(auth.ActionAmountRead) {
		writeProblem(w, http.StatusForbidden, "CAPABILITY_DENIED", false)
		return
	}
	if !canCreate(input, principal) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", false)
		return
	}
	value, err := s.useCases.Create(r.Context(), actorFrom(principal), input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ViewRegistration(value, principal))
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	principal := PrincipalFrom(r)
	value, err := s.useCases.Get(r.Context(), principal.TenantID, r.PathValue("invoice_registration_id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	if !CanAccess(value, principal) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", false)
		return
	}
	writeJSON(w, http.StatusOK, ViewRegistration(value, principal))
}

type listResponse struct {
	Items                []RegistrationView `json:"items"`
	NextCursor           *string            `json:"next_cursor"`
	AllowedActions       []string           `json:"allowed_actions"`
	EffectiveAmountMinor *int64             `json:"effective_amount_minor"`
	Currency             *string            `json:"currency"`
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	query, ok := parseListQuery(w, r)
	if !ok {
		return
	}
	principal := PrincipalFrom(r)
	if principal.ScopeKind != "tenant" && (principal.ScopeKind != "resources" || query.ProjectID == "" || !principal.CanResource("project", query.ProjectID)) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", false)
		return
	}
	page, err := s.useCases.List(r.Context(), principal.TenantID, query)
	if err != nil {
		s.fail(w, err)
		return
	}
	response := listResponse{Items: ViewRegistrations(page.Items, principal), AllowedActions: listAllowedActions(principal)}
	if page.NextCursor != "" {
		response.NextCursor = &page.NextCursor
	}
	if principal.Can(auth.ActionAmountRead) {
		amount := page.EffectiveAmountMinor
		response.EffectiveAmountMinor = &amount
		if page.Currency != "" {
			currency := page.Currency
			response.Currency = &currency
		}
	}
	writeJSON(w, http.StatusOK, response)
}

type historyResponse struct {
	Items              []RevisionView `json:"items"`
	NextBeforeRevision *int64         `json:"next_before_revision,omitempty"`
}

func (s *Server) history(w http.ResponseWriter, r *http.Request) {
	before, limit, ok := historyQuery(w, r)
	if !ok {
		return
	}
	principal := PrincipalFrom(r)
	id := r.PathValue("invoice_registration_id")
	current, err := s.useCases.Get(r.Context(), principal.TenantID, id)
	if err != nil {
		s.fail(w, err)
		return
	}
	if !CanAccess(current, principal) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", false)
		return
	}
	page, err := s.useCases.History(r.Context(), principal.TenantID, id, before, limit)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, historyResponse{Items: ViewHistory(page, principal), NextBeforeRevision: page.NextBeforeRevision})
}

func (s *Server) correct(w http.ResponseWriter, r *http.Request) {
	var input invoice.CorrectInput
	if !decodeJSON(w, r, &input) {
		return
	}
	principal := PrincipalFrom(r)
	id := r.PathValue("invoice_registration_id")
	if (input.Patch.AmountMinor != nil || input.Patch.Currency != nil) && !principal.Can(auth.ActionAmountRead) {
		writeProblem(w, http.StatusForbidden, "CAPABILITY_DENIED", false)
		return
	}
	_, ok := s.authorizeCurrent(w, r, id, principal)
	if !ok {
		return
	}
	if !canCorrectSources(input, principal) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", false)
		return
	}
	value, err := s.useCases.Correct(r.Context(), actorFrom(principal), id, input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ViewRegistration(value, principal))
}

func (s *Server) invalidate(w http.ResponseWriter, r *http.Request) {
	var input invoice.InvalidateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	principal := PrincipalFrom(r)
	id := r.PathValue("invoice_registration_id")
	if _, ok := s.authorizeCurrent(w, r, id, principal); !ok {
		return
	}
	value, err := s.useCases.Invalidate(r.Context(), actorFrom(principal), id, input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ViewRegistration(value, principal))
}

func (s *Server) replaceEvidence(w http.ResponseWriter, r *http.Request) {
	var input invoice.EvidenceInput
	if !decodeJSON(w, r, &input) {
		return
	}
	principal := PrincipalFrom(r)
	id := r.PathValue("invoice_registration_id")
	if _, ok := s.authorizeCurrent(w, r, id, principal); !ok {
		return
	}
	value, err := s.useCases.ReplaceEvidence(r.Context(), actorFrom(principal), id, input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ViewRegistration(value, principal))
}

func (s *Server) reference(w http.ResponseWriter, r *http.Request) {
	principal := PrincipalFrom(r)
	id := strings.ToLower(strings.TrimSpace(r.PathValue("invoice_registration_id")))
	if !exactReferenceScope(principal, id) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", false)
		return
	}
	value, err := s.useCases.Reference(r.Context(), principal.TenantID, id)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) authorizeCurrent(w http.ResponseWriter, r *http.Request, id string, principal auth.Principal) (invoice.Registration, bool) {
	value, err := s.useCases.Get(r.Context(), principal.TenantID, id)
	if err != nil {
		s.fail(w, err)
		return invoice.Registration{}, false
	}
	if !CanAccess(value, principal) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", false)
		return invoice.Registration{}, false
	}
	return value, true
}

func historyQuery(w http.ResponseWriter, r *http.Request) (*int64, int, bool) {
	var before *int64
	if raw := strings.TrimSpace(r.URL.Query().Get("before_revision")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 2 {
			writeProblem(w, http.StatusBadRequest, "INVALID_REQUEST", false)
			return nil, 0, false
		}
		before = &value
	}
	limit := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100 {
			writeProblem(w, http.StatusBadRequest, "INVALID_REQUEST", false)
			return nil, 0, false
		}
		limit = value
	}
	return before, limit, true
}

func parseListQuery(w http.ResponseWriter, r *http.Request) (invoice.ListQuery, bool) {
	allowed := map[string]bool{"project_id": true, "party_id": true, "status": true, "issued_from": true, "issued_to": true, "cursor": true, "limit": true}
	for key, values := range r.URL.Query() {
		if !allowed[key] || len(values) != 1 {
			writeProblem(w, http.StatusBadRequest, "INVALID_REQUEST", false)
			return invoice.ListQuery{}, false
		}
	}
	query := invoice.ListQuery{
		ProjectID:  strings.ToLower(strings.TrimSpace(r.URL.Query().Get("project_id"))),
		PartyID:    strings.ToLower(strings.TrimSpace(r.URL.Query().Get("party_id"))),
		Status:     strings.TrimSpace(r.URL.Query().Get("status")),
		IssuedFrom: strings.TrimSpace(r.URL.Query().Get("issued_from")),
		IssuedTo:   strings.TrimSpace(r.URL.Query().Get("issued_to")),
		Cursor:     strings.TrimSpace(r.URL.Query().Get("cursor")),
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "INVALID_REQUEST", false)
			return invoice.ListQuery{}, false
		}
		query.Limit = value
	}
	return query, true
}

func canCreate(input invoice.CreateInput, principal auth.Principal) bool {
	if principal.ScopeKind == "tenant" {
		return true
	}
	if principal.ScopeKind != "resources" {
		return false
	}
	matchedProject := false
	for _, source := range input.SourceRefs {
		if source.Kind != invoice.SourceProject || source.ResourceID == nil {
			continue
		}
		matchedProject = true
		if !principal.CanResource("project", strings.ToLower(strings.TrimSpace(*source.ResourceID))) {
			return false
		}
	}
	return matchedProject
}

func canCorrectSources(input invoice.CorrectInput, principal auth.Principal) bool {
	if input.Patch.SourceRefs == nil || principal.ScopeKind == "tenant" {
		return true
	}
	if principal.ScopeKind != "resources" {
		return false
	}
	matchedProject := false
	for _, source := range *input.Patch.SourceRefs {
		if source.Kind != invoice.SourceProject || source.ResourceID == nil {
			continue
		}
		matchedProject = true
		if !principal.CanResource("project", strings.ToLower(strings.TrimSpace(*source.ResourceID))) {
			return false
		}
	}
	return matchedProject
}

func exactReferenceScope(principal auth.Principal, id string) bool {
	if principal.Provider != "iam" || principal.ScopeKind != "resources" || !invoice.ValidUUID(id) || len(principal.AllowedActions) != 1 || !principal.Can(auth.ActionReferenceRead) || len(principal.Resources) != 1 {
		return false
	}
	registrations := principal.Resources["invoice_registration"]
	return len(registrations) == 1 && registrations[id]
}

func actorFrom(principal auth.Principal) invoice.Actor {
	kind := "iam_subject"
	if principal.Provider == "template" {
		kind = "template_subject"
	}
	return invoice.Actor{
		ID: principal.SubjectID, TenantID: principal.TenantID, DisplayName: principal.DisplayName,
		Kind: kind, CredentialID: principal.CredentialID,
	}
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
	case errors.Is(err, invoice.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "INVOICE_REGISTRATION_NOT_FOUND", false)
	case errors.Is(err, invoice.ErrRevisionConflict):
		writeProblem(w, http.StatusConflict, "REVISION_CONFLICT", false)
	case errors.Is(err, invoice.ErrCommandConflict):
		writeProblem(w, http.StatusConflict, "COMMAND_PAYLOAD_CONFLICT", false)
	case errors.Is(err, invoice.ErrRegistrationInvalidated):
		writeProblem(w, http.StatusConflict, "REGISTRATION_INVALIDATED", false)
	case errors.Is(err, invoice.ErrNoEffectiveChange):
		writeProblem(w, http.StatusUnprocessableEntity, "NO_EFFECTIVE_CHANGE", false)
	case errors.Is(err, invoice.ErrReferenceInvalid):
		writeProblem(w, http.StatusUnprocessableEntity, "REFERENCE_INVALID", false)
	case errors.Is(err, invoice.ErrReferenceUnavailable):
		writeProblem(w, http.StatusServiceUnavailable, "REFERENCE_SERVICE_UNAVAILABLE", true)
	case requestError(err):
		writeProblem(w, http.StatusBadRequest, "INVALID_REQUEST", false)
	default:
		writeProblem(w, http.StatusInternalServerError, "INTERNAL_ERROR", true)
	}
}

func requestError(err error) bool {
	message := err.Error()
	return strings.HasPrefix(message, "invalid ") || strings.HasPrefix(message, "at least one ") ||
		strings.HasPrefix(message, "duplicate ") || strings.Contains(message, " is required") ||
		message == "currency is not enabled"
}

func requestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Request-ID")))
		if !invoice.ValidUUID(requestID) {
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
