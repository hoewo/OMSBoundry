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
	"sort"
	"strings"
	"time"

	"omsbizfoundry/bonus-distribution/internal/auth"
	"omsbizfoundry/bonus-distribution/internal/bonus"
	"omsbizfoundry/bonus-distribution/internal/referencebonus"
)

const maxRequestBytes = 1 << 20

type UseCases interface {
	Ping(context.Context) error
	CreateBusinessDraft(context.Context, bonus.Actor, bonus.CreateBusinessDraftInput) (bonus.BonusPool, error)
	CreateProductionDraft(context.Context, bonus.Actor, bonus.CreateProductionDraftInput) (bonus.BonusPool, error)
	Publish(context.Context, bonus.Actor, string, bonus.PoolCommandInput) (bonus.BonusPool, error)
	Supersede(context.Context, bonus.Actor, bonus.SupersedeInput) (bonus.SupersessionResult, error)
	Withdraw(context.Context, bonus.Actor, string, bonus.PoolCommandInput) (bonus.BonusPool, error)
	GetPool(context.Context, bonus.Actor, string) (bonus.BonusPool, error)
	ListPools(context.Context, bonus.Actor, string) ([]bonus.BonusPool, error)
	PoolHistory(context.Context, bonus.Actor, string) ([]bonus.PoolRevision, error)
	AllocationHistory(context.Context, bonus.Actor, string) ([]bonus.AllocationRevision, error)
}

type PayableSuggestionUseCases interface {
	ProposePayableSuggestion(context.Context, bonus.Actor, bonus.ProposePayableSuggestionInput) (bonus.BonusPayableSuggestion, error)
	GetPayableSuggestion(context.Context, bonus.Actor, string) (bonus.BonusPayableSuggestion, error)
	ListPayableSuggestions(context.Context, bonus.Actor, string) ([]bonus.BonusPayableSuggestion, error)
	PayableSuggestionHistory(context.Context, bonus.Actor, string) ([]bonus.BonusPayableSuggestionRevision, error)
	SynchronizePayableSuggestion(context.Context, bonus.Actor, string, bonus.SuggestionCommandInput) (bonus.BonusPayableSuggestion, error)
	WithdrawPayableSuggestion(context.Context, bonus.Actor, string, bonus.SuggestionCommandInput) (bonus.BonusPayableSuggestion, error)
}

type Server struct {
	referenceBonus     *referencebonus.Service
	useCases           UseCases
	payableSuggestions PayableSuggestionUseCases
	handler            http.Handler
}

type requestIDKey struct{}

type Option func(*Server) error

func WithReferenceBonus(v *referencebonus.Service) Option {
	return func(s *Server) error {
		if v == nil {
			return fmt.Errorf("manual bonus service is required")
		}
		s.referenceBonus = v
		return nil
	}
}
func New(useCases UseCases, authenticator auth.Authenticator, options ...Option) (*Server, error) {
	if useCases == nil || authenticator == nil {
		return nil, fmt.Errorf("use cases and authenticator are required")
	}
	s := &Server{useCases: useCases}
	for _, option := range options {
		if option == nil {
			return nil, fmt.Errorf("server option is required")
		}
		if err := option(s); err != nil {
			return nil, err
		}
	}
	s.payableSuggestions, _ = useCases.(PayableSuggestionUseCases)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", s.live)
	mux.HandleFunc("GET /health/ready", s.ready)
	mux.HandleFunc("GET /", s.index)
	mux.HandleFunc("GET /assets/bonus-distribution.js", bonusDistributionWorkspaceScript)
	if s.referenceBonus != nil {
		s.referenceBonusRoutes(mux, authenticator)
	}
	mux.Handle("GET /api/v1/bonus-pools", ProtectAny(authenticator, []string{bonus.ActionPoolRead}, http.HandlerFunc(s.listPools)))
	mux.Handle("POST /api/v1/bonus-pools/business-drafts", ProtectAny(authenticator, []string{bonus.ActionBusinessCalculate}, http.HandlerFunc(s.createBusinessDraft)))
	mux.Handle("POST /api/v1/bonus-pools/production-drafts", ProtectAny(authenticator, []string{bonus.ActionProductionCalculate}, http.HandlerFunc(s.createProductionDraft)))
	mux.Handle("GET /api/v1/bonus-pools/{id}", ProtectAny(authenticator, []string{bonus.ActionPoolRead}, http.HandlerFunc(s.getPool)))
	mux.Handle("GET /api/v1/bonus-pools/{id}/history", ProtectAny(authenticator, []string{bonus.ActionPoolHistory}, http.HandlerFunc(s.poolHistory)))
	mux.Handle("POST /api/v1/bonus-pools/{id}/publication", ProtectAny(authenticator, []string{bonus.ActionBusinessPublish, bonus.ActionProductionPublish}, http.HandlerFunc(s.publish)))
	mux.Handle("POST /api/v1/bonus-pools/supersession", ProtectAny(authenticator, []string{bonus.ActionBusinessSupersede, bonus.ActionProductionSupersede}, http.HandlerFunc(s.supersede)))
	mux.Handle("POST /api/v1/bonus-pools/{id}/withdrawal", ProtectAny(authenticator, []string{bonus.ActionBusinessWithdraw, bonus.ActionProductionWithdraw}, http.HandlerFunc(s.withdraw)))
	mux.Handle("GET /api/v1/bonus-allocations/{id}/history", ProtectAny(authenticator, []string{bonus.ActionAllocationRead}, http.HandlerFunc(s.allocationHistory)))
	if s.payableSuggestions != nil {
		mux.Handle("GET /api/v1/bonus-payable-suggestions", ProtectAny(authenticator, []string{bonus.ActionSuggestionRead}, http.HandlerFunc(s.listPayableSuggestions)))
		mux.Handle("POST /api/v1/bonus-payable-suggestions", ProtectAny(authenticator, []string{bonus.ActionSuggestionCreate}, http.HandlerFunc(s.proposePayableSuggestion)))
		mux.Handle("GET /api/v1/bonus-payable-suggestions/{id}", ProtectAny(authenticator, []string{bonus.ActionSuggestionRead}, http.HandlerFunc(s.getPayableSuggestion)))
		mux.Handle("GET /api/v1/bonus-payable-suggestions/{id}/history", ProtectAny(authenticator, []string{bonus.ActionSuggestionRead}, http.HandlerFunc(s.payableSuggestionHistory)))
		mux.Handle("POST /api/v1/bonus-payable-suggestions/{id}/synchronization", ProtectAny(authenticator, []string{bonus.ActionSuggestionCreate}, http.HandlerFunc(s.synchronizePayableSuggestion)))
		mux.Handle("POST /api/v1/bonus-payable-suggestions/{id}/withdrawal", ProtectAny(authenticator, []string{bonus.ActionSuggestionWithdraw}, http.HandlerFunc(s.withdrawPayableSuggestion)))
	}
	s.handler = requestContext(securityHeaders(mux))
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	renderBonusDistributionWorkspace(w)
}

func (s *Server) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"service": "bonus-distribution", "status": "live"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.useCases.Ping(r.Context()); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "NOT_READY", true)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"service": "bonus-distribution", "status": "ready"})
}

func (s *Server) createBusinessDraft(w http.ResponseWriter, r *http.Request) {
	var input bonus.CreateBusinessDraftInput
	if !decodeJSON(w, r, &input) {
		return
	}
	p := PrincipalFrom(r)
	if !p.CanResource("project", strings.ToLower(strings.TrimSpace(input.Project.ID))) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", false)
		return
	}
	if !p.Can(bonus.ActionAllocationManage) || !p.Can(bonus.ActionAllocationAmountRead) {
		writeProblem(w, http.StatusForbidden, "CAPABILITY_DENIED", false)
		return
	}
	value, err := s.useCases.CreateBusinessDraft(r.Context(), actorFrom(p), input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, viewPool(value, p))
}

func (s *Server) createProductionDraft(w http.ResponseWriter, r *http.Request) {
	var input bonus.CreateProductionDraftInput
	if !decodeJSON(w, r, &input) {
		return
	}
	p := PrincipalFrom(r)
	if !p.CanResource("project", strings.ToLower(strings.TrimSpace(input.Project.ID))) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", false)
		return
	}
	if !p.Can(bonus.ActionAllocationManage) || !p.Can(bonus.ActionAllocationAmountRead) {
		writeProblem(w, http.StatusForbidden, "CAPABILITY_DENIED", false)
		return
	}
	value, err := s.useCases.CreateProductionDraft(r.Context(), actorFrom(p), input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, viewPool(value, p))
}

func (s *Server) listPools(w http.ResponseWriter, r *http.Request) {
	p := PrincipalFrom(r)
	projectID := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("project_id")))
	if !p.CanResource("project", projectID) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", false)
		return
	}
	values, err := s.useCases.ListPools(r.Context(), actorFrom(p), projectID)
	if err != nil {
		s.fail(w, err)
		return
	}
	items := make([]poolView, 0, len(values))
	for _, value := range values {
		items = append(items, viewPool(value, p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"project_id": projectID, "items": items, "allowed_actions": allowedCollectionActions(p, projectID)})
}

func (s *Server) getPool(w http.ResponseWriter, r *http.Request) {
	p := PrincipalFrom(r)
	value, err := s.useCases.GetPool(r.Context(), actorFrom(p), r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, viewPool(value, p))
}

func (s *Server) poolHistory(w http.ResponseWriter, r *http.Request) {
	p := PrincipalFrom(r)
	values, err := s.useCases.PoolHistory(r.Context(), actorFrom(p), r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	items := make([]poolRevisionView, 0, len(values))
	for _, value := range values {
		items = append(items, poolRevisionView{PoolID: value.PoolID, TenantID: value.TenantID, Revision: value.Revision, EventKind: value.EventKind, CommandID: value.CommandID, Snapshot: viewPool(value.Snapshot, p), Reason: value.Reason, ActorRef: value.ActorRef, OccurredAt: value.OccurredAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) allocationHistory(w http.ResponseWriter, r *http.Request) {
	p := PrincipalFrom(r)
	values, err := s.useCases.AllocationHistory(r.Context(), actorFrom(p), r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	items := make([]allocationRevisionView, 0, len(values))
	for _, value := range values {
		items = append(items, allocationRevisionView{AllocationID: value.AllocationID, PoolID: value.PoolID, TenantID: value.TenantID, Revision: value.Revision, EventKind: value.EventKind, CommandID: value.CommandID, Snapshot: viewAllocation(value.Snapshot, p), Reason: value.Reason, ActorRef: value.ActorRef, OccurredAt: value.OccurredAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) publish(w http.ResponseWriter, r *http.Request) {
	var input bonus.PoolCommandInput
	if !decodeJSON(w, r, &input) {
		return
	}
	p := PrincipalFrom(r)
	if !p.Can(bonus.ActionAllocationAmountRead) {
		writeProblem(w, http.StatusForbidden, "CAPABILITY_DENIED", false)
		return
	}
	value, err := s.useCases.Publish(r.Context(), actorFrom(p), r.PathValue("id"), input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, viewPool(value, p))
}

func (s *Server) supersede(w http.ResponseWriter, r *http.Request) {
	var input bonus.SupersedeInput
	if !decodeJSON(w, r, &input) {
		return
	}
	p := PrincipalFrom(r)
	if !p.Can(bonus.ActionAllocationAmountRead) {
		writeProblem(w, http.StatusForbidden, "CAPABILITY_DENIED", false)
		return
	}
	value, err := s.useCases.Supersede(r.Context(), actorFrom(p), input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"superseded": viewPool(value.Superseded, p), "published": viewPool(value.Published, p)})
}

func (s *Server) withdraw(w http.ResponseWriter, r *http.Request) {
	var input bonus.PoolCommandInput
	if !decodeJSON(w, r, &input) {
		return
	}
	p := PrincipalFrom(r)
	value, err := s.useCases.Withdraw(r.Context(), actorFrom(p), r.PathValue("id"), input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, viewPool(value, p))
}

type sourceItemView struct {
	Type                string `json:"type"`
	ID                  string `json:"id"`
	Revision            int64  `json:"revision"`
	TransactionID       string `json:"transaction_id,omitempty"`
	TransactionRevision int64  `json:"transaction_revision,omitempty"`
	AmountMinor         *int64 `json:"amount_minor"`
	Currency            string `json:"currency"`
	State               string `json:"state"`
	Digest              string `json:"digest"`
}

type sourceView struct {
	Type             string           `json:"type"`
	ID               string           `json:"id"`
	Revision         int64            `json:"revision"`
	ProjectID        string           `json:"project_id"`
	Currency         string           `json:"currency"`
	TotalAmountMinor *int64           `json:"total_amount_minor"`
	State            string           `json:"state"`
	Digest           string           `json:"digest"`
	Items            []sourceItemView `json:"items"`
}

type allocationView struct {
	ID               string                   `json:"id"`
	TenantID         string                   `json:"tenant_id"`
	PoolID           string                   `json:"pool_id"`
	PoolRevision     int64                    `json:"pool_revision"`
	ProjectID        string                   `json:"project_id"`
	BonusType        string                   `json:"bonus_type"`
	Person           bonus.PersonSnapshot     `json:"person"`
	Assignment       bonus.AssignmentSnapshot `json:"assignment"`
	RatioBasisPoints int64                    `json:"ratio_basis_points"`
	AmountMinor      *int64                   `json:"amount_minor"`
	Currency         string                   `json:"currency"`
	Reason           string                   `json:"reason"`
	State            string                   `json:"state"`
	Revision         int64                    `json:"revision"`
	CreatedAt        time.Time                `json:"created_at"`
	UpdatedAt        time.Time                `json:"updated_at"`
}

type poolView struct {
	ID               string                     `json:"id"`
	TenantID         string                     `json:"tenant_id"`
	Project          bonus.ProjectSnapshot      `json:"project"`
	BonusType        string                     `json:"bonus_type"`
	State            string                     `json:"state"`
	Source           sourceView                 `json:"source"`
	Policy           bonus.PolicySnapshot       `json:"policy"`
	Currency         string                     `json:"currency"`
	AmountMinor      *int64                     `json:"amount_minor"`
	UnallocatedMinor *int64                     `json:"unallocated_minor"`
	Allocations      []allocationView           `json:"allocations"`
	Supersedes       *bonus.SupersededReference `json:"supersedes,omitempty"`
	SupersededBy     *bonus.SupersededReference `json:"superseded_by,omitempty"`
	Revision         int64                      `json:"revision"`
	CreatedBy        string                     `json:"created_by"`
	UpdatedBy        string                     `json:"updated_by"`
	CreatedAt        time.Time                  `json:"created_at"`
	UpdatedAt        time.Time                  `json:"updated_at"`
	AllowedActions   []string                   `json:"allowed_actions"`
}

type poolRevisionView struct {
	PoolID     string    `json:"pool_id"`
	TenantID   string    `json:"tenant_id"`
	Revision   int64     `json:"revision"`
	EventKind  string    `json:"event_kind"`
	CommandID  string    `json:"command_id"`
	Snapshot   poolView  `json:"snapshot"`
	Reason     string    `json:"reason"`
	ActorRef   string    `json:"actor_ref"`
	OccurredAt time.Time `json:"occurred_at"`
}

type allocationRevisionView struct {
	AllocationID string         `json:"allocation_id"`
	PoolID       string         `json:"pool_id"`
	TenantID     string         `json:"tenant_id"`
	Revision     int64          `json:"revision"`
	EventKind    string         `json:"event_kind"`
	CommandID    string         `json:"command_id"`
	Snapshot     allocationView `json:"snapshot"`
	Reason       string         `json:"reason"`
	ActorRef     string         `json:"actor_ref"`
	OccurredAt   time.Time      `json:"occurred_at"`
}

func viewPool(value bonus.BonusPool, p auth.Principal) poolView {
	view := poolView{ID: value.ID, TenantID: value.TenantID, Project: value.Project, BonusType: value.BonusType, State: value.State, Source: viewSource(value.Source, p), Policy: value.Policy, Currency: value.Currency, Allocations: make([]allocationView, 0, len(value.Allocations)), Supersedes: value.Supersedes, SupersededBy: value.SupersededBy, Revision: value.Revision, CreatedBy: value.CreatedBy, UpdatedBy: value.UpdatedBy, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, AllowedActions: allowedPoolActions(value, p)}
	if p.Can(bonus.ActionAllocationAmountRead) {
		amount, unallocated := value.AmountMinor, value.UnallocatedMinor
		view.AmountMinor, view.UnallocatedMinor = &amount, &unallocated
	}
	for _, allocation := range value.Allocations {
		view.Allocations = append(view.Allocations, viewAllocation(allocation, p))
	}
	return view
}

func viewSource(value bonus.SourceSnapshot, p auth.Principal) sourceView {
	view := sourceView{Type: value.Type, ID: value.ID, Revision: value.Revision, ProjectID: value.ProjectID, Currency: value.Currency, State: value.State, Digest: value.Digest, Items: make([]sourceItemView, 0, len(value.Items))}
	if p.Can(bonus.ActionAllocationAmountRead) {
		amount := value.TotalAmountMinor
		view.TotalAmountMinor = &amount
	}
	for _, item := range value.Items {
		itemView := sourceItemView{Type: item.Type, ID: item.ID, Revision: item.Revision, TransactionID: item.TransactionID, TransactionRevision: item.TransactionRevision, Currency: item.Currency, State: item.State, Digest: item.Digest}
		if p.Can(bonus.ActionAllocationAmountRead) {
			amount := item.AmountMinor
			itemView.AmountMinor = &amount
		}
		view.Items = append(view.Items, itemView)
	}
	return view
}

func viewAllocation(value bonus.BonusAllocation, p auth.Principal) allocationView {
	view := allocationView{ID: value.ID, TenantID: value.TenantID, PoolID: value.PoolID, PoolRevision: value.PoolRevision, ProjectID: value.ProjectID, BonusType: value.BonusType, Person: value.Person, Assignment: value.Assignment, RatioBasisPoints: value.RatioBasisPoints, Currency: value.Currency, Reason: value.Reason, State: value.State, Revision: value.Revision, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
	if p.Can(bonus.ActionAllocationAmountRead) {
		amount := value.AmountMinor
		view.AmountMinor = &amount
	}
	return view
}

func allowedPoolActions(value bonus.BonusPool, p auth.Principal) []string {
	if !p.CanPool(value.ID, value.Project.ID) {
		return []string{}
	}
	actions := []string{}
	for _, action := range auth.UserActions {
		if !p.Can(action) {
			continue
		}
		switch action {
		case bonus.ActionBusinessPublish:
			if value.BonusType != bonus.TypeBusiness || value.State != bonus.StateDraft {
				continue
			}
		case bonus.ActionProductionPublish:
			if value.BonusType != bonus.TypeProduction || value.State != bonus.StateDraft {
				continue
			}
		case bonus.ActionBusinessSupersede:
			if value.BonusType != bonus.TypeBusiness || value.State != bonus.StatePublished {
				continue
			}
		case bonus.ActionProductionSupersede:
			if value.BonusType != bonus.TypeProduction || value.State != bonus.StatePublished {
				continue
			}
		case bonus.ActionBusinessWithdraw:
			if value.BonusType != bonus.TypeBusiness || (value.State != bonus.StateDraft && value.State != bonus.StatePublished) {
				continue
			}
		case bonus.ActionProductionWithdraw:
			if value.BonusType != bonus.TypeProduction || (value.State != bonus.StateDraft && value.State != bonus.StatePublished) {
				continue
			}
		case bonus.ActionBusinessCalculate, bonus.ActionProductionCalculate, bonus.ActionAllocationManage:
			continue
		}
		actions = append(actions, action)
	}
	return actions
}

func allowedCollectionActions(p auth.Principal, projectID string) []string {
	if !p.CanResource("project", projectID) {
		return []string{}
	}
	result := []string{}
	for _, action := range []string{bonus.ActionBusinessCalculate, bonus.ActionProductionCalculate, bonus.ActionPoolRead} {
		if p.Can(action) {
			result = append(result, action)
		}
	}
	return result
}

func actorFrom(p auth.Principal) bonus.Actor {
	actor := bonus.Actor{ID: p.SubjectID, TenantID: p.TenantID, TenantScope: p.ScopeKind == "tenant", Credential: p.CredentialID, SubjectKind: "iam_subject", Actions: make([]string, 0, len(p.AllowedActions))}
	if p.Provider == "template" {
		actor.SubjectKind = "template_subject"
	}
	for action, allowed := range p.AllowedActions {
		if allowed {
			actor.Actions = append(actor.Actions, action)
		}
	}
	for id := range p.Resources["project"] {
		actor.ProjectIDs = append(actor.ProjectIDs, id)
	}
	for _, kind := range []string{"bonus_pool", "bonus_allocation", "bonus_payable_suggestion", "business_bonus_project", "production_bonus_project", "reference_oa_bonus_allocation"} {
		for id := range p.Resources[kind] {
			actor.ResourceScopes = append(actor.ResourceScopes, bonus.ResourceScope{Type: kind, ID: id})
		}
	}
	sort.Strings(actor.Actions)
	sort.Strings(actor.ProjectIDs)
	return actor
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
	case errors.Is(err, bonus.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "RESOURCE_NOT_FOUND", false)
	case errors.Is(err, bonus.ErrRevisionConflict):
		writeProblem(w, http.StatusConflict, "REVISION_CONFLICT", false)
	case errors.Is(err, bonus.ErrCommandConflict):
		writeProblem(w, http.StatusConflict, "COMMAND_PAYLOAD_CONFLICT", false)
	case errors.Is(err, bonus.ErrStateConflict):
		writeProblem(w, http.StatusConflict, "STATE_CONFLICT", false)
	case errors.Is(err, bonus.ErrForbidden):
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", false)
	case errors.Is(err, bonus.ErrReferenceInvalid), errors.Is(err, bonus.ErrPolicyInvalid), errors.Is(err, bonus.ErrAllocationInvalid):
		writeProblem(w, http.StatusUnprocessableEntity, "REFERENCE_OR_POLICY_INVALID", false)
	case errors.Is(err, bonus.ErrReferenceUnavailable):
		writeProblem(w, http.StatusServiceUnavailable, "REFERENCE_SERVICE_UNAVAILABLE", true)
	case errors.Is(err, bonus.ErrTargetUnavailable):
		writeProblem(w, http.StatusServiceUnavailable, "PAYABLE_TARGET_UNAVAILABLE", true)
	case requestError(err):
		writeProblem(w, http.StatusBadRequest, "INVALID_REQUEST", false)
	default:
		writeProblem(w, http.StatusInternalServerError, "INTERNAL_ERROR", true)
	}
}

func requestError(err error) bool {
	message := err.Error()
	return strings.HasPrefix(message, "invalid ") || strings.Contains(message, " is required") || strings.HasPrefix(message, "duplicate ")
}

func requestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Request-ID")))
		if !validUUID(requestID) {
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

func validUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	for i, char := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		if !strings.ContainsRune("0123456789abcdef", char) {
			return false
		}
	}
	return true
}
