package httpserver

import (
	"net/http"
	"strings"

	"omsbizfoundry/bonus-distribution/internal/bonus"
)

func (s *Server) proposePayableSuggestion(w http.ResponseWriter, r *http.Request) {
	var input bonus.ProposePayableSuggestionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	p := PrincipalFrom(r)
	if !p.CanResource("bonus_allocation", strings.ToLower(strings.TrimSpace(input.AllocationID))) && p.ScopeKind != "tenant" && len(p.Resources["project"]) == 0 && len(p.Resources["bonus_pool"]) == 0 {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", false)
		return
	}
	if !p.Can(bonus.ActionAllocationAmountRead) {
		writeProblem(w, http.StatusForbidden, "CAPABILITY_DENIED", false)
		return
	}
	value, err := s.payableSuggestions.ProposePayableSuggestion(r.Context(), actorFrom(p), input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, value)
}

func (s *Server) listPayableSuggestions(w http.ResponseWriter, r *http.Request) {
	p := PrincipalFrom(r)
	projectID := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("project_id")))
	if !p.CanResource("project", projectID) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", false)
		return
	}
	values, err := s.payableSuggestions.ListPayableSuggestions(r.Context(), actorFrom(p), projectID)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"project_id": projectID, "items": values})
}

func (s *Server) getPayableSuggestion(w http.ResponseWriter, r *http.Request) {
	value, err := s.payableSuggestions.GetPayableSuggestion(r.Context(), actorFrom(PrincipalFrom(r)), r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) payableSuggestionHistory(w http.ResponseWriter, r *http.Request) {
	values, err := s.payableSuggestions.PayableSuggestionHistory(r.Context(), actorFrom(PrincipalFrom(r)), r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) synchronizePayableSuggestion(w http.ResponseWriter, r *http.Request) {
	var input bonus.SuggestionCommandInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.payableSuggestions.SynchronizePayableSuggestion(r.Context(), actorFrom(PrincipalFrom(r)), r.PathValue("id"), input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) withdrawPayableSuggestion(w http.ResponseWriter, r *http.Request) {
	var input bonus.SuggestionCommandInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.payableSuggestions.WithdrawPayableSuggestion(r.Context(), actorFrom(PrincipalFrom(r)), r.PathValue("id"), input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
