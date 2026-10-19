package httpserver

import (
	"net/http"
	"omsbizfoundry/project-workspace/internal/auth"
	"omsbizfoundry/project-workspace/internal/projects"
)

type counterpartyContextResponse struct {
	projects.CounterpartyContextView
	AllowedActions []string `json:"allowed_actions"`
}

func (s *Server) counterpartyContext(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	value, err := s.useCases.CounterpartyContext(r.Context(), p.TenantID, r.PathValue("project_id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	actions := []string{auth.ActionCounterpartyContextRead}
	if value.ProjectStatus == projects.StatusActive && p.Can(auth.ActionCounterpartyContextManage) {
		actions = append(actions, auth.ActionCounterpartyContextManage)
	}
	s.writeResult(w, r, http.StatusOK, counterpartyContextResponse{value, actions}, nil)
}
func (s *Server) changeCounterpartyContext(w http.ResponseWriter, r *http.Request) {
	var input projects.CounterpartyContextInput
	if !decodeJSON(w, r, &input) {
		return
	}
	in, err := projects.NormalizeCounterpartyContext(input)
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", err.Error(), false, nil)
		return
	}
	result, err := s.useCases.ChangeCounterpartyContext(r.Context(), actorFrom(r), r.PathValue("project_id"), in)
	s.writeResult(w, r, http.StatusOK, result, err)
}

func redactCounterpartyHistory(page projects.HistoryPage, p auth.Principal) projects.HistoryPage {
	if p.Can(auth.ActionCounterpartyContextRead) {
		return page
	}
	page.Items = append([]projects.HistoryEvent(nil), page.Items...)
	for i := range page.Items {
		if page.Items[i].Payload["schema_version"] == projects.CounterpartyContextSchema {
			page.Items[i].Payload = map[string]any{"schema_version": projects.CounterpartyContextSchema, "redacted": true}
		}
	}
	return page
}
