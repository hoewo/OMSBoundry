package httpserver

import (
	"net/http"

	"omsbizfoundry/project-workspace/internal/projects"
)

type partyCandidatePageResponse struct {
	projects.PartyCandidatePage
	AllowedActions []string `json:"allowed_actions"`
}

func (s *Server) partyCandidates(w http.ResponseWriter, r *http.Request) {
	principal := principalFrom(r)
	value, err := s.useCases.ListPartyCandidates(r.Context(), principal.TenantID)
	s.writeResult(w, r, http.StatusOK, partyCandidatePageResponse{PartyCandidatePage: value, AllowedActions: credentialActions(principal)}, err)
}

func (s *Server) registerPartyCandidate(w http.ResponseWriter, r *http.Request) {
	var input projects.PartyCandidateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.RegisterPartyCandidate(r.Context(), actorFrom(r), input)
	s.writeResult(w, r, http.StatusCreated, value, err)
}
