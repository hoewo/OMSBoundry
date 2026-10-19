package httpserver

import (
	"net/http"
	"strings"

	"omsbizfoundry/cost-outsourcing/internal/auth"
	"omsbizfoundry/cost-outsourcing/internal/costoutsourcing"
)

func (s *Server) listPayableSuggestions(w http.ResponseWriter, r *http.Request) {
	p := PrincipalFrom(r)
	projectID := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("project_id")))
	if !p.CanResource("project", projectID) {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", false)
		return
	}
	values, err := s.useCases.ListPayableSuggestions(r.Context(), actorFrom(p), projectID)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"project_id": projectID, "items": values, "allowed_actions": allowedCollectionActions(p, auth.SuggestionActions)})
}

func (s *Server) proposePayableSuggestion(w http.ResponseWriter, r *http.Request) {
	var input costoutsourcing.ProposeSuggestionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.ProposePayableSuggestion(r.Context(), actorFrom(PrincipalFrom(r)), input)
	if err != nil {
		s.fail(w, err)
		return
	}
	status := http.StatusCreated
	if value.State == costoutsourcing.SuggestionTargetUnavailable {
		status = http.StatusAccepted
	}
	writeJSON(w, status, value)
}

func (s *Server) getPayableSuggestion(w http.ResponseWriter, r *http.Request) {
	value, err := s.useCases.GetPayableSuggestion(r.Context(), actorFrom(PrincipalFrom(r)), r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) payableSuggestionHistory(w http.ResponseWriter, r *http.Request) {
	values, err := s.useCases.PayableSuggestionHistory(r.Context(), actorFrom(PrincipalFrom(r)), r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) synchronizePayableSuggestion(w http.ResponseWriter, r *http.Request) {
	var input costoutsourcing.SuggestionCommandInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.SynchronizePayableSuggestion(r.Context(), actorFrom(PrincipalFrom(r)), r.PathValue("id"), input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) withdrawPayableSuggestion(w http.ResponseWriter, r *http.Request) {
	var input costoutsourcing.SuggestionCommandInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.WithdrawPayableSuggestion(r.Context(), actorFrom(PrincipalFrom(r)), r.PathValue("id"), input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
