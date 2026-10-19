package httpserver

import (
	"net/http"
	"strconv"

	"omsbizfoundry/bonus-distribution/internal/auth"
	core "omsbizfoundry/bonus-distribution/internal/bonus"
	rb "omsbizfoundry/bonus-distribution/internal/referencebonus"
)

type manualBonusView struct {
	AssignmentSource string                  `json:"assignment_source,omitempty"`
	Discipline       *rb.DisciplineSnapshot  `json:"discipline,omitempty"`
	ID               string                  `json:"id"`
	BatchID          string                  `json:"batch_id"`
	TenantID         string                  `json:"tenant_id"`
	Profile          string                  `json:"profile"`
	Project          core.ProjectSnapshot    `json:"project"`
	Scope            string                  `json:"scope"`
	Person           core.PersonSnapshot     `json:"person"`
	Assignment       core.AssignmentSnapshot `json:"assignment"`
	Method           string                  `json:"method"`
	OccurredOn       string                  `json:"occurred_on"`
	Description      string                  `json:"description"`
	State            string                  `json:"state"`
	Revision         int64                   `json:"revision"`
	AllowedActions   []string                `json:"allowed_actions"`
	Currency         *string                 `json:"currency,omitempty"`
	BaseAmountMinor  *string                 `json:"base_amount_minor,omitempty"`
	RatioBasisPoints *int64                  `json:"ratio_basis_points,omitempty"`
	FixedAmountMinor *string                 `json:"fixed_amount_minor,omitempty"`
	AmountMinor      *string                 `json:"amount_minor,omitempty"`
}

func manualView(v rb.Record, p auth.Principal) manualBonusView {
	a := actorFrom(p)
	out := manualBonusView{AssignmentSource: v.AssignmentSource, Discipline: v.Discipline, ID: v.ID, BatchID: v.BatchID, TenantID: v.TenantID, Profile: v.EffectiveProfile(), Project: v.Project, Scope: v.Scope, Person: v.Person, Assignment: v.Assignment, Method: v.Method, OccurredOn: v.OccurredOn, Description: v.Description, State: v.State, Revision: v.Revision, AllowedActions: []string{}}
	for _, action := range rb.Actions {
		if action == rb.ActionRecord || action == rb.ActionWithdraw && v.State != "recorded" || !rb.Can(a, action, v.Project.ID, v.Scope, v.ID) {
			continue
		}
		out.AllowedActions = append(out.AllowedActions, action)
	}
	if rb.Can(a, rb.ActionAmountRead, v.Project.ID, v.Scope, v.ID) {
		base, fixed, amount := strconv.FormatInt(v.BaseAmountMinor, 10), strconv.FormatInt(v.FixedAmountMinor, 10), strconv.FormatInt(v.AmountMinor, 10)
		out.Currency = &v.Currency
		out.BaseAmountMinor = &base
		out.FixedAmountMinor = &fixed
		out.AmountMinor = &amount
		out.RatioBasisPoints = &v.RatioBasisPoints
	}
	return out
}
func (s *Server) referenceBonusRoutes(mux *http.ServeMux, a auth.Authenticator) {
	const base = "/api/v1/reference-oa/bonus-allocations"
	mux.Handle("GET "+base, ProtectAny(a, []string{rb.ActionRead}, http.HandlerFunc(s.manualBonusList)))
	mux.Handle("POST "+base, ProtectAny(a, []string{rb.ActionRecord}, http.HandlerFunc(s.manualBonusCreate)))
	mux.Handle("GET "+base+"/candidates", ProtectAny(a, []string{rb.ActionRecord}, http.HandlerFunc(s.manualBonusCandidates)))
	mux.Handle("GET "+base+"/{id}", ProtectAny(a, []string{rb.ActionRead}, http.HandlerFunc(s.manualBonusGet)))
	mux.Handle("GET "+base+"/{id}/history", ProtectAny(a, []string{rb.ActionHistory}, http.HandlerFunc(s.manualBonusHistory)))
	mux.Handle("POST "+base+"/{id}/withdrawal", ProtectAny(a, []string{rb.ActionWithdraw}, http.HandlerFunc(s.manualBonusWithdraw)))
}
func (s *Server) manualBonusList(w http.ResponseWriter, r *http.Request) {
	p := PrincipalFrom(r)
	project, scope := r.URL.Query().Get("project_id"), r.URL.Query().Get("scope")
	rows, e := s.referenceBonus.List(r.Context(), actorFrom(p), project, scope)
	if e != nil {
		s.fail(w, e)
		return
	}
	items := make([]manualBonusView, 0, len(rows))
	for _, v := range rows {
		items = append(items, manualView(v, p))
	}
	actions := []string{}
	for _, v := range rb.Actions {
		if rb.Can(actorFrom(p), v, project, scope, "") && (v != rb.ActionRecord || rb.Can(actorFrom(p), rb.ActionAmountRead, project, scope, "")) {
			actions = append(actions, v)
		}
	}
	writeJSON(w, 200, map[string]any{"profile": s.referenceBonus.Profile(), "project_id": project, "scope": scope, "items": items, "allowed_actions": actions})
}
func (s *Server) manualBonusCreate(w http.ResponseWriter, r *http.Request) {
	var in rb.CreateInput
	if !decodeJSON(w, r, &in) {
		return
	}
	p := PrincipalFrom(r)
	rows, e := s.referenceBonus.Create(r.Context(), actorFrom(p), in)
	if e != nil {
		s.fail(w, e)
		return
	}
	items := make([]manualBonusView, 0, len(rows))
	for _, v := range rows {
		items = append(items, manualView(v, p))
	}
	writeJSON(w, 201, map[string]any{"profile": s.referenceBonus.Profile(), "project_id": in.ProjectRef.ID, "scope": in.Scope, "items": items})
}
func (s *Server) manualBonusGet(w http.ResponseWriter, r *http.Request) {
	p := PrincipalFrom(r)
	v, e := s.referenceBonus.Get(r.Context(), actorFrom(p), r.PathValue("id"))
	if e != nil {
		s.fail(w, e)
		return
	}
	writeJSON(w, 200, manualView(v, p))
}
func (s *Server) manualBonusHistory(w http.ResponseWriter, r *http.Request) {
	p := PrincipalFrom(r)
	rows, e := s.referenceBonus.History(r.Context(), actorFrom(p), r.PathValue("id"))
	if e != nil {
		s.fail(w, e)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, v := range rows {
		items = append(items, map[string]any{"snapshot": manualView(v.Snapshot, p), "command_id": v.CommandID, "operation": v.Operation, "reason": v.Reason})
	}
	writeJSON(w, 200, map[string]any{"items": items})
}
func (s *Server) manualBonusWithdraw(w http.ResponseWriter, r *http.Request) {
	var in rb.WithdrawInput
	if !decodeJSON(w, r, &in) {
		return
	}
	p := PrincipalFrom(r)
	v, e := s.referenceBonus.Withdraw(r.Context(), actorFrom(p), r.PathValue("id"), in)
	if e != nil {
		s.fail(w, e)
		return
	}
	writeJSON(w, 200, manualView(v, p))
}
func (s *Server) manualBonusCandidates(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	revision, e := strconv.ParseInt(q.Get("project_revision"), 10, 64)
	if e != nil || revision < 1 {
		writeProblem(w, 400, "INVALID_REQUEST", false)
		return
	}
	project, scope := q.Get("project_id"), q.Get("scope")
	rows, e := s.referenceBonus.Candidates(r.Context(), actorFrom(PrincipalFrom(r)), core.ProjectRef{ID: project, Revision: revision}, scope)
	if e != nil {
		s.fail(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"profile": s.referenceBonus.Profile(), "project_id": project, "project_revision": revision, "scope": scope, "items": rows})
}
