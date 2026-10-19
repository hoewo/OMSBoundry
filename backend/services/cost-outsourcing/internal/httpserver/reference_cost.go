package httpserver

import (
	"net/http"
	"strconv"

	"omsbizfoundry/cost-outsourcing/internal/auth"
	core "omsbizfoundry/cost-outsourcing/internal/costoutsourcing"
	rc "omsbizfoundry/cost-outsourcing/internal/referencecost"
)

// Native owner view: exact string money, no hidden amount in revision snapshots.
type referenceCostView struct {
	ID             string                  `json:"id"`
	TenantID       string                  `json:"tenant_id"`
	Project        core.ProjectSnapshot    `json:"project"`
	Profile        string                  `json:"profile"`
	Scope          string                  `json:"scope"`
	Category       string                  `json:"category"`
	Currency       *string                 `json:"currency,omitempty"`
	AmountMinor    *string                 `json:"amount_minor,omitempty"`
	OccurredOn     *string                 `json:"occurred_on"`
	Description    *string                 `json:"description"`
	Evidence       []core.EvidenceSnapshot `json:"evidence"`
	State          string                  `json:"state"`
	Revision       int64                   `json:"revision"`
	AllowedActions []string                `json:"allowed_actions"`
}

func referenceView(v rc.Record, p auth.Principal) referenceCostView {
	a := actorFrom(p)
	out := referenceCostView{ID: v.ID, TenantID: v.TenantID, Project: v.Project, Profile: rc.Profile, Scope: v.Scope, Category: v.Category, OccurredOn: v.OccurredOn, Description: v.Description, Evidence: v.Evidence, State: v.State, Revision: v.Revision, AllowedActions: []string{}}
	for _, action := range auth.CostActions {
		if action == auth.ActionCostCreate {
			continue
		}
		if !rc.Can(a, action, v.Project.ID, v.Scope, v.ID) {
			continue
		}
		if v.State != "recorded" && (action == auth.ActionCostCorrect || action == auth.ActionCostWithdraw || action == auth.ActionCostEvidenceManage) {
			continue
		}
		if action == auth.ActionCostCorrect && !rc.Can(a, auth.ActionCostAmountRead, v.Project.ID, v.Scope, v.ID) {
			continue
		}
		out.AllowedActions = append(out.AllowedActions, action)
	}
	if rc.Can(a, auth.ActionCostAmountRead, v.Project.ID, v.Scope, v.ID) {
		amount := strconv.FormatInt(v.AmountMinor, 10)
		currency := v.Currency
		out.AmountMinor = &amount
		out.Currency = &currency
	}
	return out
}
func (s *Server) referenceCostRoutes(mux *http.ServeMux, a auth.Authenticator) {
	const base = "/api/v1/reference-oa/cost-entries"
	mux.Handle("GET "+base, Protect(a, auth.ActionCostRead, http.HandlerFunc(s.referenceCostList)))
	mux.Handle("POST "+base, Protect(a, auth.ActionCostCreate, http.HandlerFunc(s.referenceCostCreate)))
	mux.Handle("GET "+base+"/{id}", Protect(a, auth.ActionCostRead, http.HandlerFunc(s.referenceCostGet)))
	mux.Handle("GET "+base+"/{id}/history", Protect(a, auth.ActionCostHistory, http.HandlerFunc(s.referenceCostHistory)))
	mux.Handle("POST "+base+"/{id}/corrections", Protect(a, auth.ActionCostCorrect, http.HandlerFunc(s.referenceCostCorrect)))
	mux.Handle("POST "+base+"/{id}/withdrawal", Protect(a, auth.ActionCostWithdraw, http.HandlerFunc(s.referenceCostWithdraw)))
}
func (s *Server) referenceCostList(w http.ResponseWriter, r *http.Request) {
	p := PrincipalFrom(r)
	a := actorFrom(p)
	project, scope := r.URL.Query().Get("project_id"), r.URL.Query().Get("scope")
	values, err := s.referenceCosts.List(r.Context(), a, project, scope)
	if err != nil {
		s.fail(w, err)
		return
	}
	items := make([]referenceCostView, 0, len(values))
	for _, v := range values {
		items = append(items, referenceView(v, p))
	}
	actions := []string{}
	for _, action := range auth.CostActions {
		if rc.Can(a, action, project, scope, "") {
			if action == auth.ActionCostCreate && !rc.Can(a, auth.ActionCostAmountRead, project, scope, "") {
				continue
			}
			actions = append(actions, action)
		}
	}
	writeJSON(w, 200, map[string]any{"profile": rc.Profile, "project_id": project, "scope": scope, "items": items, "allowed_actions": actions})
}
func (s *Server) referenceCostCreate(w http.ResponseWriter, r *http.Request) {
	var in rc.CreateInput
	if !decodeJSON(w, r, &in) {
		return
	}
	p := PrincipalFrom(r)
	v, err := s.referenceCosts.Create(r.Context(), actorFrom(p), in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 201, referenceView(v, p))
}
func (s *Server) referenceCostGet(w http.ResponseWriter, r *http.Request) {
	p := PrincipalFrom(r)
	v, err := s.referenceCosts.Get(r.Context(), actorFrom(p), r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 200, referenceView(v, p))
}
func (s *Server) referenceCostHistory(w http.ResponseWriter, r *http.Request) {
	p := PrincipalFrom(r)
	values, err := s.referenceCosts.History(r.Context(), actorFrom(p), r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	items := make([]map[string]any, 0, len(values))
	for _, v := range values {
		items = append(items, map[string]any{"snapshot": referenceView(v.Record, p), "command_id": v.CommandID, "operation": v.Operation, "reason": v.Reason})
	}
	writeJSON(w, 200, map[string]any{"items": items})
}
func (s *Server) referenceCostCorrect(w http.ResponseWriter, r *http.Request) {
	var in rc.ChangeInput
	if !decodeJSON(w, r, &in) {
		return
	}
	p := PrincipalFrom(r)
	v, err := s.referenceCosts.Correct(r.Context(), actorFrom(p), r.PathValue("id"), in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 200, referenceView(v, p))
}
func (s *Server) referenceCostWithdraw(w http.ResponseWriter, r *http.Request) {
	var in rc.WithdrawInput
	if !decodeJSON(w, r, &in) {
		return
	}
	p := PrincipalFrom(r)
	v, err := s.referenceCosts.Withdraw(r.Context(), actorFrom(p), r.PathValue("id"), in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 200, referenceView(v, p))
}
