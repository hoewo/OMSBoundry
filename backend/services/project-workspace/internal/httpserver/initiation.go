package httpserver

import (
	"net/http"
	"strings"

	"omsbizfoundry/project-workspace/internal/auth"
	"omsbizfoundry/project-workspace/internal/projects"
)

func (s *Server) dispatchInitiation(w http.ResponseWriter, r *http.Request) {
	var input projects.InitiationDispatchInput
	if !decodeJSON(w, r, &input) {
		return
	}
	principal := principalFrom(r)
	creationAction := auth.ActionCreate
	if strings.ToLower(strings.TrimSpace(input.NodeType)) == projects.NodeTypePackage {
		creationAction = auth.ActionHierarchyManage
	}
	if !principal.Can(creationAction) || (!principal.Can(auth.ActionAssignmentManage) && !principal.Can(auth.ActionAssignmentBusinessOwnerManage)) {
		writeProblem(w, r, http.StatusForbidden, "capability_denied", "派发需要创建及经营负责人分工能力", false, nil)
		return
	}
	value, err := s.useCases.DispatchInitiation(r.Context(), actorFrom(r), input)
	s.writeResult(w, r, http.StatusCreated, value, err)
}
