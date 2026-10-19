package httpserver

import (
	"net/http"
	"omsbizfoundry/project-workspace/internal/auth"
	"omsbizfoundry/project-workspace/internal/projects"
)

func (s *Server) changeBasicInfo(w http.ResponseWriter, r *http.Request) {
	s.changeOwnerInfo(w, r, false)
}
func (s *Server) changeInitiationInfo(w http.ResponseWriter, r *http.Request) {
	s.changeOwnerInfo(w, r, true)
}
func (s *Server) changeOwnerInfo(w http.ResponseWriter, r *http.Request, initiation bool) {
	var input projects.BasicInfoChangeInput
	if !decodeJSON(w, r, &input) {
		return
	}
	in, err := projects.NormalizeBasicInfoChange(input)
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", err.Error(), false, nil)
		return
	}
	p := principalFrom(r)
	profileAction := auth.ActionProfileCorrect
	if initiation {
		profileAction = auth.ActionUpdate
	}
	if in.Profile != nil && !p.Can(profileAction) {
		message := "缺少基本资料更正能力"
		if initiation {
			message = "缺少立项资料编辑能力"
		}
		writeProblem(w, r, http.StatusForbidden, "capability_denied", message, false, nil)
		return
	}
	for _, v := range in.Owners {
		if !p.Can(auth.ActionAssignmentManage) && !p.Can(assignmentManageAction(v.RoleKey)) {
			writeProblem(w, r, http.StatusForbidden, "capability_denied", "缺少该负责人分工管理能力", false, nil)
			return
		}
	}
	change := s.useCases.ChangeBasicInfo
	if initiation {
		change = s.useCases.ChangeInitiationInfo
	}
	result, err := change(r.Context(), actorFrom(r), r.PathValue("project_id"), in)
	s.writeResult(w, r, http.StatusOK, result, err)
}
