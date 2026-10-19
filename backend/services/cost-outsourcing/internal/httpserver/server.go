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

	"omsbizfoundry/cost-outsourcing/internal/auth"
	"omsbizfoundry/cost-outsourcing/internal/costoutsourcing"
	"omsbizfoundry/cost-outsourcing/internal/referencecost"
)

const maxRequestBytes = 1 << 20

type UseCases interface {
	Ping(context.Context) error
	RecordCost(context.Context, costoutsourcing.Actor, costoutsourcing.RecordCostInput) (costoutsourcing.CostEntry, error)
	GetCost(context.Context, costoutsourcing.Actor, string) (costoutsourcing.CostEntry, error)
	ListCosts(context.Context, costoutsourcing.Actor, string) ([]costoutsourcing.CostEntry, error)
	CostHistory(context.Context, costoutsourcing.Actor, string) ([]costoutsourcing.CostRevision, error)
	CorrectCost(context.Context, costoutsourcing.Actor, string, costoutsourcing.CorrectCostInput) (costoutsourcing.CostEntry, error)
	WithdrawCost(context.Context, costoutsourcing.Actor, string, costoutsourcing.WithdrawInput) (costoutsourcing.CostEntry, error)
	RecordEngagement(context.Context, costoutsourcing.Actor, costoutsourcing.RecordEngagementInput) (costoutsourcing.Engagement, error)
	GetEngagement(context.Context, costoutsourcing.Actor, string) (costoutsourcing.Engagement, error)
	ListEngagements(context.Context, costoutsourcing.Actor, string) ([]costoutsourcing.Engagement, error)
	EngagementHistory(context.Context, costoutsourcing.Actor, string) ([]costoutsourcing.EngagementRevision, error)
	CorrectEngagement(context.Context, costoutsourcing.Actor, string, costoutsourcing.CorrectEngagementInput) (costoutsourcing.Engagement, error)
	WithdrawEngagement(context.Context, costoutsourcing.Actor, string, costoutsourcing.WithdrawInput) (costoutsourcing.Engagement, error)
	Assess(context.Context, costoutsourcing.Actor, string, costoutsourcing.AssessInput) (costoutsourcing.Assessment, error)
	AssessmentHistory(context.Context, costoutsourcing.Actor, string) ([]costoutsourcing.Assessment, error)
	ListLatestAssessments(context.Context, costoutsourcing.Actor, string) ([]costoutsourcing.Assessment, error)
	ProposePayableSuggestion(context.Context, costoutsourcing.Actor, costoutsourcing.ProposeSuggestionInput) (costoutsourcing.PayableSuggestion, error)
	GetPayableSuggestion(context.Context, costoutsourcing.Actor, string) (costoutsourcing.PayableSuggestion, error)
	ListPayableSuggestions(context.Context, costoutsourcing.Actor, string) ([]costoutsourcing.PayableSuggestion, error)
	PayableSuggestionHistory(context.Context, costoutsourcing.Actor, string) ([]costoutsourcing.PayableSuggestionRevision, error)
	SynchronizePayableSuggestion(context.Context, costoutsourcing.Actor, string, costoutsourcing.SuggestionCommandInput) (costoutsourcing.PayableSuggestion, error)
	WithdrawPayableSuggestion(context.Context, costoutsourcing.Actor, string, costoutsourcing.SuggestionCommandInput) (costoutsourcing.PayableSuggestion, error)
}

type Server struct {
	useCases       UseCases
	referenceCosts *referencecost.Service
	handler        http.Handler
}

type requestIDKey struct{}

func New(useCases UseCases, authenticator auth.Authenticator, overlay ...*referencecost.Service) (*Server, error) {
	if useCases == nil || authenticator == nil {
		return nil, fmt.Errorf("use cases and authenticator are required")
	}
	s := &Server{useCases: useCases}
	if len(overlay) > 1 {
		return nil, fmt.Errorf("only one selected reference cost profile is allowed")
	}
	if len(overlay) == 1 {
		s.referenceCosts = overlay[0]
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", s.live)
	mux.HandleFunc("GET /health/ready", s.ready)
	mux.HandleFunc("GET /", s.index)
	mux.HandleFunc("GET /assets/cost-outsourcing.js", costOutsourcingWorkspaceScript)
	mux.Handle("GET /api/v1/cost-entries", Protect(authenticator, auth.ActionCostRead, http.HandlerFunc(s.listCosts)))
	mux.Handle("POST /api/v1/cost-entries", Protect(authenticator, auth.ActionCostCreate, http.HandlerFunc(s.recordCost)))
	mux.Handle("GET /api/v1/cost-entries/{id}", Protect(authenticator, auth.ActionCostRead, http.HandlerFunc(s.getCost)))
	mux.Handle("GET /api/v1/references/cost-entries/{id}/materials", Protect(authenticator, auth.ActionCostRead, http.HandlerFunc(s.costMaterials)))
	mux.Handle("GET /api/v1/cost-entries/{id}/history", Protect(authenticator, auth.ActionCostHistory, http.HandlerFunc(s.costHistory)))
	mux.Handle("POST /api/v1/cost-entries/{id}/corrections", Protect(authenticator, auth.ActionCostCorrect, http.HandlerFunc(s.correctCost)))
	mux.Handle("POST /api/v1/cost-entries/{id}/withdrawal", Protect(authenticator, auth.ActionCostWithdraw, http.HandlerFunc(s.withdrawCost)))
	mux.Handle("GET /api/v1/outsourcing-engagements", Protect(authenticator, auth.ActionEngagementRead, http.HandlerFunc(s.listEngagements)))
	mux.Handle("POST /api/v1/outsourcing-engagements", Protect(authenticator, auth.ActionEngagementCreate, http.HandlerFunc(s.recordEngagement)))
	mux.Handle("GET /api/v1/outsourcing-engagements/{id}", Protect(authenticator, auth.ActionEngagementRead, http.HandlerFunc(s.getEngagement)))
	mux.Handle("GET /api/v1/references/outsourcing-engagements/{id}/materials", Protect(authenticator, auth.ActionEngagementRead, http.HandlerFunc(s.engagementMaterials)))
	mux.Handle("GET /api/v1/outsourcing-engagements/{id}/history", Protect(authenticator, auth.ActionEngagementHistory, http.HandlerFunc(s.engagementHistory)))
	mux.Handle("POST /api/v1/outsourcing-engagements/{id}/corrections", Protect(authenticator, auth.ActionEngagementCorrect, http.HandlerFunc(s.correctEngagement)))
	mux.Handle("POST /api/v1/outsourcing-engagements/{id}/withdrawal", Protect(authenticator, auth.ActionEngagementWithdraw, http.HandlerFunc(s.withdrawEngagement)))
	mux.Handle("POST /api/v1/outsourcing-engagements/{id}/assessments", Protect(authenticator, auth.ActionEngagementAssess, http.HandlerFunc(s.assess)))
	mux.Handle("GET /api/v1/outsourcing-engagements/{id}/assessments", Protect(authenticator, auth.ActionEngagementHistory, http.HandlerFunc(s.assessmentHistory)))
	mux.Handle("GET /api/v1/outsourcing-engagement-assessments", Protect(authenticator, auth.ActionEngagementHistory, http.HandlerFunc(s.listLatestAssessments)))
	mux.Handle("GET /api/v1/payable-suggestions", Protect(authenticator, auth.ActionSuggestionRead, http.HandlerFunc(s.listPayableSuggestions)))
	mux.Handle("POST /api/v1/payable-suggestions", Protect(authenticator, auth.ActionSuggestionCreate, http.HandlerFunc(s.proposePayableSuggestion)))
	mux.Handle("GET /api/v1/payable-suggestions/{id}", Protect(authenticator, auth.ActionSuggestionRead, http.HandlerFunc(s.getPayableSuggestion)))
	mux.Handle("GET /api/v1/payable-suggestions/{id}/history", Protect(authenticator, auth.ActionSuggestionRead, http.HandlerFunc(s.payableSuggestionHistory)))
	mux.Handle("POST /api/v1/payable-suggestions/{id}/synchronization", Protect(authenticator, auth.ActionSuggestionCreate, http.HandlerFunc(s.synchronizePayableSuggestion)))
	mux.Handle("POST /api/v1/payable-suggestions/{id}/withdrawal", Protect(authenticator, auth.ActionSuggestionWithdraw, http.HandlerFunc(s.withdrawPayableSuggestion)))
	if s.referenceCosts != nil {
		s.referenceCostRoutes(mux, authenticator)
	}
	s.handler = requestContext(securityHeaders(mux))
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }

func (s *Server) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"service": "cost-outsourcing", "status": "live"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.useCases.Ping(r.Context()); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "NOT_READY", true)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"service": "cost-outsourcing", "status": "ready"})
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	renderCostOutsourcingWorkspace(w)
}

func (s *Server) listCosts(w http.ResponseWriter, r *http.Request) {
	p := PrincipalFrom(r)
	projectID := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("project_id")))
	if !p.CanResource("project", projectID) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", false)
		return
	}
	values, err := s.useCases.ListCosts(r.Context(), actorFrom(p), projectID)
	if err != nil {
		s.fail(w, err)
		return
	}
	items := make([]costView, 0, len(values))
	for _, value := range values {
		items = append(items, viewCost(value, p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"project_id": projectID, "items": items, "allowed_actions": allowedCollectionActions(p, auth.CostActions)})
}

func (s *Server) recordCost(w http.ResponseWriter, r *http.Request) {
	var input costoutsourcing.RecordCostInput
	if !decodeJSON(w, r, &input) {
		return
	}
	p := PrincipalFrom(r)
	if !p.CanResource("project", strings.ToLower(strings.TrimSpace(input.ProjectRef.ID))) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", false)
		return
	}
	if !p.Can(auth.ActionCostAmountRead) {
		writeProblem(w, http.StatusForbidden, "CAPABILITY_DENIED", false)
		return
	}
	if len(input.EvidenceRefs) > 0 && !p.Can(auth.ActionCostEvidenceManage) {
		writeProblem(w, http.StatusForbidden, "CAPABILITY_DENIED", false)
		return
	}
	value, err := s.useCases.RecordCost(r.Context(), actorFrom(p), input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, viewCost(value, p))
}

func (s *Server) getCost(w http.ResponseWriter, r *http.Request) {
	p := PrincipalFrom(r)
	value, err := s.useCases.GetCost(r.Context(), actorFrom(p), r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, viewCost(value, p))
}

func (s *Server) costHistory(w http.ResponseWriter, r *http.Request) {
	p := PrincipalFrom(r)
	values, err := s.useCases.CostHistory(r.Context(), actorFrom(p), r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	items := make([]costRevisionView, 0, len(values))
	for _, value := range values {
		items = append(items, costRevisionView{CostEntryID: value.CostEntryID, Revision: value.Revision, EventKind: value.EventKind, CommandID: value.CommandID, Snapshot: viewCost(value.Snapshot, p), Reason: value.Reason, ActorRef: value.ActorRef, OccurredAt: value.OccurredAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) correctCost(w http.ResponseWriter, r *http.Request) {
	var input costoutsourcing.CorrectCostInput
	if !decodeJSON(w, r, &input) {
		return
	}
	p := PrincipalFrom(r)
	if !p.Can(auth.ActionCostAmountRead) {
		writeProblem(w, http.StatusForbidden, "CAPABILITY_DENIED", false)
		return
	}
	value, err := s.useCases.CorrectCost(r.Context(), actorFrom(p), r.PathValue("id"), input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, viewCost(value, p))
}

func (s *Server) withdrawCost(w http.ResponseWriter, r *http.Request) {
	var input costoutsourcing.WithdrawInput
	if !decodeJSON(w, r, &input) {
		return
	}
	p := PrincipalFrom(r)
	value, err := s.useCases.WithdrawCost(r.Context(), actorFrom(p), r.PathValue("id"), input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, viewCost(value, p))
}

func (s *Server) listEngagements(w http.ResponseWriter, r *http.Request) {
	p := PrincipalFrom(r)
	projectID := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("project_id")))
	if !p.CanResource("project", projectID) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", false)
		return
	}
	values, err := s.useCases.ListEngagements(r.Context(), actorFrom(p), projectID)
	if err != nil {
		s.fail(w, err)
		return
	}
	items := make([]engagementView, 0, len(values))
	for _, value := range values {
		items = append(items, viewEngagement(value, p))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"project_id":                 projectID,
		"items":                      items,
		"allowed_actions":            allowedCollectionActions(p, auth.EngagementActions),
		"suggestion_allowed_actions": allowedCollectionActions(p, auth.SuggestionActions),
	})
}

func (s *Server) recordEngagement(w http.ResponseWriter, r *http.Request) {
	var input costoutsourcing.RecordEngagementInput
	if !decodeJSON(w, r, &input) {
		return
	}
	p := PrincipalFrom(r)
	if !p.CanResource("project", strings.ToLower(strings.TrimSpace(input.ProjectRef.ID))) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", false)
		return
	}
	if !p.Can(auth.ActionEngagementAmountRead) {
		writeProblem(w, http.StatusForbidden, "CAPABILITY_DENIED", false)
		return
	}
	if len(input.EvidenceRefs) > 0 && !p.Can(auth.ActionEngagementEvidenceManage) {
		writeProblem(w, http.StatusForbidden, "CAPABILITY_DENIED", false)
		return
	}
	value, err := s.useCases.RecordEngagement(r.Context(), actorFrom(p), input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, viewEngagement(value, p))
}

func (s *Server) getEngagement(w http.ResponseWriter, r *http.Request) {
	p := PrincipalFrom(r)
	value, err := s.useCases.GetEngagement(r.Context(), actorFrom(p), r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, viewEngagement(value, p))
}

func (s *Server) engagementHistory(w http.ResponseWriter, r *http.Request) {
	p := PrincipalFrom(r)
	values, err := s.useCases.EngagementHistory(r.Context(), actorFrom(p), r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	items := make([]engagementRevisionView, 0, len(values))
	for _, value := range values {
		items = append(items, engagementRevisionView{EngagementID: value.EngagementID, Revision: value.Revision, EventKind: value.EventKind, CommandID: value.CommandID, Snapshot: viewEngagement(value.Snapshot, p), Reason: value.Reason, ActorRef: value.ActorRef, OccurredAt: value.OccurredAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) correctEngagement(w http.ResponseWriter, r *http.Request) {
	var input costoutsourcing.CorrectEngagementInput
	if !decodeJSON(w, r, &input) {
		return
	}
	p := PrincipalFrom(r)
	if !p.Can(auth.ActionEngagementAmountRead) {
		writeProblem(w, http.StatusForbidden, "CAPABILITY_DENIED", false)
		return
	}
	value, err := s.useCases.CorrectEngagement(r.Context(), actorFrom(p), r.PathValue("id"), input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, viewEngagement(value, p))
}

func (s *Server) withdrawEngagement(w http.ResponseWriter, r *http.Request) {
	var input costoutsourcing.WithdrawInput
	if !decodeJSON(w, r, &input) {
		return
	}
	p := PrincipalFrom(r)
	value, err := s.useCases.WithdrawEngagement(r.Context(), actorFrom(p), r.PathValue("id"), input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, viewEngagement(value, p))
}

func (s *Server) assess(w http.ResponseWriter, r *http.Request) {
	var input costoutsourcing.AssessInput
	if !decodeJSON(w, r, &input) {
		return
	}
	p := PrincipalFrom(r)
	value, err := s.useCases.Assess(r.Context(), actorFrom(p), r.PathValue("id"), input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, value)
}

func (s *Server) assessmentHistory(w http.ResponseWriter, r *http.Request) {
	p := PrincipalFrom(r)
	values, err := s.useCases.AssessmentHistory(r.Context(), actorFrom(p), r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) listLatestAssessments(w http.ResponseWriter, r *http.Request) {
	p := PrincipalFrom(r)
	projectID := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("project_id")))
	if !p.CanResource("project", projectID) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", false)
		return
	}
	values, err := s.useCases.ListLatestAssessments(r.Context(), actorFrom(p), projectID)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"project_id": projectID, "items": values})
}

type costView struct {
	ID             string                             `json:"id"`
	TenantID       string                             `json:"tenant_id"`
	Project        costoutsourcing.ProjectSnapshot    `json:"project"`
	Category       string                             `json:"category"`
	OccurredOn     string                             `json:"occurred_on"`
	Currency       *string                            `json:"currency,omitempty"`
	AmountMinor    *int64                             `json:"amount_minor,omitempty"`
	DistanceMeters int64                              `json:"distance_meters,omitempty"`
	Description    string                             `json:"description"`
	Evidence       []costoutsourcing.EvidenceSnapshot `json:"evidence"`
	State          string                             `json:"state"`
	Revision       int64                              `json:"revision"`
	CreatedBy      string                             `json:"created_by"`
	UpdatedBy      string                             `json:"updated_by"`
	CreatedAt      time.Time                          `json:"created_at"`
	UpdatedAt      time.Time                          `json:"updated_at"`
	AllowedActions []string                           `json:"allowed_actions"`
}
type engagementView struct {
	ID             string                             `json:"id"`
	TenantID       string                             `json:"tenant_id"`
	Project        costoutsourcing.ProjectSnapshot    `json:"project"`
	Provider       costoutsourcing.PartySnapshot      `json:"provider"`
	ContactName    string                             `json:"contact_name,omitempty"`
	ContactPhone   string                             `json:"contact_phone,omitempty"`
	Scope          string                             `json:"scope"`
	Currency       *string                            `json:"currency,omitempty"`
	AmountMinor    *int64                             `json:"amount_minor,omitempty"`
	Contract       *contractView                      `json:"contract,omitempty"`
	Evidence       []costoutsourcing.EvidenceSnapshot `json:"evidence"`
	Note           string                             `json:"note,omitempty"`
	State          string                             `json:"state"`
	Revision       int64                              `json:"revision"`
	CreatedBy      string                             `json:"created_by"`
	UpdatedBy      string                             `json:"updated_by"`
	CreatedAt      time.Time                          `json:"created_at"`
	UpdatedAt      time.Time                          `json:"updated_at"`
	AllowedActions []string                           `json:"allowed_actions"`
}
type contractView struct {
	Application    string  `json:"application"`
	ResourceType   string  `json:"resource_type"`
	ResourceID     string  `json:"resource_id"`
	Revision       int64   `json:"revision"`
	ProjectID      string  `json:"project_id"`
	PartyID        string  `json:"party_id"`
	State          string  `json:"state"`
	Currency       *string `json:"currency,omitempty"`
	AmountMinor    *int64  `json:"amount_minor,omitempty"`
	Digest         string  `json:"digest"`
	DisplaySummary string  `json:"display_summary"`
}
type costRevisionView struct {
	CostEntryID string    `json:"cost_entry_id"`
	Revision    int64     `json:"revision"`
	EventKind   string    `json:"event_kind"`
	CommandID   string    `json:"command_id"`
	Snapshot    costView  `json:"snapshot"`
	Reason      string    `json:"reason"`
	ActorRef    string    `json:"actor_ref"`
	OccurredAt  time.Time `json:"occurred_at"`
}
type engagementRevisionView struct {
	EngagementID string         `json:"engagement_id"`
	Revision     int64          `json:"revision"`
	EventKind    string         `json:"event_kind"`
	CommandID    string         `json:"command_id"`
	Snapshot     engagementView `json:"snapshot"`
	Reason       string         `json:"reason"`
	ActorRef     string         `json:"actor_ref"`
	OccurredAt   time.Time      `json:"occurred_at"`
}

func viewCost(value costoutsourcing.CostEntry, p auth.Principal) costView {
	view := costView{ID: value.ID, TenantID: value.TenantID, Project: value.Project, Category: value.Category, OccurredOn: value.OccurredOn, DistanceMeters: value.DistanceMeters, Description: value.Description, Evidence: value.Evidence, State: value.State, Revision: value.Revision, CreatedBy: value.CreatedBy, UpdatedBy: value.UpdatedBy, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, AllowedActions: allowedCostActions(value, p)}
	if p.Can(auth.ActionCostAmountRead) {
		currency, amount := value.Currency, value.AmountMinor
		view.Currency, view.AmountMinor = &currency, &amount
	}
	return view
}

func viewEngagement(value costoutsourcing.Engagement, p auth.Principal) engagementView {
	view := engagementView{ID: value.ID, TenantID: value.TenantID, Project: value.Project, Provider: value.Provider, ContactName: value.ContactName, ContactPhone: value.ContactPhone, Scope: value.Scope, Evidence: value.Evidence, Note: value.Note, State: value.State, Revision: value.Revision, CreatedBy: value.CreatedBy, UpdatedBy: value.UpdatedBy, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, AllowedActions: allowedEngagementActions(value, p)}
	if value.Contract != nil {
		view.Contract = &contractView{Application: value.Contract.Application, ResourceType: value.Contract.ResourceType, ResourceID: value.Contract.ResourceID, Revision: value.Contract.Revision, ProjectID: value.Contract.ProjectID, PartyID: value.Contract.PartyID, State: value.Contract.State, Digest: value.Contract.Digest, DisplaySummary: value.Contract.DisplaySummary}
	}
	if p.Can(auth.ActionEngagementAmountRead) {
		currency, amount := value.Currency, value.AmountMinor
		view.Currency, view.AmountMinor = &currency, &amount
		if view.Contract != nil {
			contractCurrency, contractAmount := value.Contract.Currency, value.Contract.AmountMinor
			view.Contract.Currency, view.Contract.AmountMinor = &contractCurrency, &contractAmount
		}
	}
	return view
}

func allowedCostActions(value costoutsourcing.CostEntry, p auth.Principal) []string {
	actions := []string{}
	for _, action := range auth.CostActions {
		if !p.Can(action) {
			continue
		}
		if (action == auth.ActionCostCorrect || action == auth.ActionCostWithdraw || action == auth.ActionCostEvidenceManage) && value.State != costoutsourcing.StateRecorded {
			continue
		}
		actions = append(actions, action)
	}
	return actions
}

func allowedEngagementActions(value costoutsourcing.Engagement, p auth.Principal) []string {
	actions := []string{}
	for _, action := range auth.EngagementActions {
		if !p.Can(action) {
			continue
		}
		if (action == auth.ActionEngagementCorrect || action == auth.ActionEngagementWithdraw || action == auth.ActionEngagementAssess || action == auth.ActionEngagementEvidenceManage) && value.State != costoutsourcing.StateRecorded {
			continue
		}
		actions = append(actions, action)
	}
	return actions
}

func allowedCollectionActions(p auth.Principal, candidates []string) []string {
	actions := make([]string, 0, len(candidates))
	for _, action := range candidates {
		if p.Can(action) {
			actions = append(actions, action)
		}
	}
	return actions
}

func actorFrom(p auth.Principal) costoutsourcing.Actor {
	actor := costoutsourcing.Actor{ID: p.SubjectID, TenantID: p.TenantID, Kind: "iam_subject", CredentialID: p.CredentialID, TenantScope: p.ScopeKind == "tenant", Actions: make([]string, 0, len(p.AllowedActions))}
	if p.Provider == "template" {
		actor.Kind = "template_subject"
	}
	for action, allowed := range p.AllowedActions {
		if allowed {
			actor.Actions = append(actor.Actions, action)
		}
	}
	for id := range p.Resources["project"] {
		actor.ProjectIDs = append(actor.ProjectIDs, id)
	}
	for _, kind := range []string{"cost_entry", "outsourcing_engagement", "payable_suggestion", "reference_oa_cost_entry", "business_cost_project", "production_cost_project"} {
		for id := range p.Resources[kind] {
			actor.ResourceScopes = append(actor.ResourceScopes, costoutsourcing.ResourceScope{Type: kind, ID: id})
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
	case errors.Is(err, costoutsourcing.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "RESOURCE_NOT_FOUND", false)
	case errors.Is(err, costoutsourcing.ErrRevisionConflict):
		writeProblem(w, http.StatusConflict, "REVISION_CONFLICT", false)
	case errors.Is(err, costoutsourcing.ErrCommandConflict):
		writeProblem(w, http.StatusConflict, "COMMAND_PAYLOAD_CONFLICT", false)
	case errors.Is(err, costoutsourcing.ErrWithdrawn):
		writeProblem(w, http.StatusConflict, "RESOURCE_WITHDRAWN", false)
	case errors.Is(err, costoutsourcing.ErrForbidden):
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", false)
	case errors.Is(err, costoutsourcing.ErrReferenceInvalid):
		writeProblem(w, http.StatusUnprocessableEntity, "REFERENCE_INVALID", false)
	case errors.Is(err, costoutsourcing.ErrReferenceUnavailable):
		writeProblem(w, http.StatusServiceUnavailable, "REFERENCE_SERVICE_UNAVAILABLE", true)
	case errors.Is(err, costoutsourcing.ErrTargetUnavailable):
		writeProblem(w, http.StatusServiceUnavailable, "MONEY_TARGET_UNAVAILABLE", true)
	case requestError(err):
		writeProblem(w, http.StatusBadRequest, "INVALID_REQUEST", false)
	default:
		writeProblem(w, http.StatusInternalServerError, "INTERNAL_ERROR", true)
	}
}

func requestError(err error) bool {
	message := err.Error()
	return strings.HasPrefix(message, "invalid ") || strings.Contains(message, " is required") || strings.HasPrefix(message, "duplicate ") || strings.Contains(message, "only valid")
}

func requestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Request-ID")))
		if !uuidPattern.MatchString(requestID) {
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

var uuidPattern = func() interface{ MatchString(string) bool } { return uuidMatcher{} }()

type uuidMatcher struct{}

func (uuidMatcher) MatchString(value string) bool {
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
