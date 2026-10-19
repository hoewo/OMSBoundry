package httpserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"omsbizfoundry/project-workspace/internal/auth"
	"omsbizfoundry/project-workspace/internal/projects"
	personnel "omsbizfoundry/project-workspace/internal/referenceoa/productionpersonnel"
)

type OAProductionUseCases interface {
	AssignmentReferences(context.Context, string, string, string) (projects.OAProductionAssignmentReferences, error)
	Personnel(context.Context, string, string, bool) (projects.OAProductionView, error)
	ProductionProfile(context.Context, string, string) (projects.OAProductionProfileContext, error)
	Enroll(context.Context, projects.Actor, string, projects.RevisionCommand) (projects.OAProductionReceipt, error)
	ApplyPersonnel(context.Context, projects.Actor, string, personnel.Intention) (projects.OAProductionReceipt, error)
	SetParticipantApplicable(context.Context, projects.Actor, string, projects.RevisionCommand, bool) (projects.OAProductionReceipt, error)
	Disciplines(context.Context, string, bool) ([]projects.OADiscipline, error)
	ChangeDiscipline(context.Context, projects.Actor, projects.OADisciplineCommand) (projects.OADiscipline, error)
}

func (s *Server) registerOAProduction(mux *http.ServeMux) {
	const references = "/api/v1/references/projects/{project_id}/reference-oa/production-assignments"
	mux.Handle("GET "+references, s.protected(auth.ActionOAProductionPersonnelRead, true, http.HandlerFunc(s.oaAssignmentReferences)))
	mux.Handle("GET "+references+"/{assignment_id}", s.protected(auth.ActionOAProductionPersonnelRead, true, http.HandlerFunc(s.oaAssignmentReferences)))
	mux.Handle("GET /api/v1/projects/{project_id}/reference-oa/production-personnel", s.protected(auth.ActionOAProductionPersonnelRead, true, http.HandlerFunc(s.oaPersonnel)))
	mux.Handle("POST /api/v1/projects/{project_id}/reference-oa/production-personnel", s.protected(auth.ActionOAProductionPersonnelManage, true, http.HandlerFunc(s.oaApplyPersonnel)))
	mux.Handle("GET /api/v1/projects/{project_id}/reference-oa/production-profile", s.protected(auth.ActionOAProductionPersonnelManage, true, http.HandlerFunc(s.oaProductionProfile)))
	mux.Handle("POST /api/v1/projects/{project_id}/reference-oa/production-profile", s.protected(auth.ActionOAProductionPersonnelManage, true, http.HandlerFunc(s.oaEnroll)))
	mux.Handle("POST /api/v1/projects/{project_id}/reference-oa/participant-applicability", s.protected(auth.ActionOAProductionPersonnelManage, true, http.HandlerFunc(s.oaParticipantApplicable)))
	mux.Handle("GET /api/v1/projects/{project_id}/reference-oa/production-disciplines", s.protected(auth.ActionOAProductionPersonnelRead, true, http.HandlerFunc(s.oaProjectDisciplines)))
	mux.Handle("GET /api/v1/reference-oa/production-disciplines", s.protected(auth.ActionOAProductionDisciplineRead, false, http.HandlerFunc(s.oaDisciplines)))
	mux.Handle("POST /api/v1/reference-oa/production-disciplines", s.protected(auth.ActionOAProductionDisciplineManage, false, http.HandlerFunc(s.oaChangeDiscipline)))
}
func (s *Server) oaProductionProfile(w http.ResponseWriter, r *http.Request) {
	view, err := s.oaProduction.ProductionProfile(r.Context(), principalFrom(r).TenantID, r.PathValue("project_id"))
	if err != nil {
		s.oaFail(w, r, err)
		return
	}
	allowed := []string{}
	if view.ProjectStatus != projects.StatusCancelled {
		allowed = append(allowed, auth.ActionOAProductionPersonnelManage)
	}
	writeJSON(w, http.StatusOK, struct {
		projects.OAProductionProfileContext
		AllowedActions []string `json:"allowed_actions"`
	}{view, allowed})
}
func (s *Server) oaAssignmentReferences(w http.ResponseWriter, r *http.Request) {
	v, err := s.oaProduction.AssignmentReferences(r.Context(), principalFrom(r).TenantID, r.PathValue("project_id"), r.PathValue("assignment_id"))
	if err != nil {
		s.oaFail(w, r, err)
		return
	}
	if r.PathValue("assignment_id") != "" {
		if len(v.Items) != 1 {
			s.oaFail(w, r, projects.ErrNotFound)
			return
		}
		writeJSON(w, 200, v.Items[0])
		return
	}
	writeJSON(w, 200, v)
}
func (s *Server) oaPersonnel(w http.ResponseWriter, r *http.Request) {
	includeEnded := false
	if raw := r.URL.Query().Get("include_ended"); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			writeProblem(w, r, 400, "invalid_request", "include_ended 必须为布尔值", false, nil)
			return
		}
		includeEnded = v
	}
	view, err := s.oaProduction.Personnel(r.Context(), principalFrom(r).TenantID, r.PathValue("project_id"), includeEnded)
	if err != nil {
		s.oaFail(w, r, err)
		return
	}
	permissions := []string{}
	p := principalFrom(r)
	if view.ProjectStatus != projects.StatusCancelled && p.Can(auth.ActionOAProductionPersonnelManage) {
		permissions = append(permissions, auth.ActionOAProductionPersonnelManage)
	}
	if p.ScopeKind == "tenant" && p.Can(auth.ActionOAProductionDisciplineManage) {
		permissions = append(permissions, auth.ActionOAProductionDisciplineManage)
	}
	writeJSON(w, 200, struct {
		projects.OAProductionView
		AllowedActions []string `json:"allowed_actions"`
	}{view, permissions})
}
func (s *Server) oaApplyPersonnel(w http.ResponseWriter, r *http.Request) {
	var in personnel.Intention
	if !decodeJSON(w, r, &in) {
		return
	}
	v, err := s.oaProduction.ApplyPersonnel(r.Context(), actorFrom(r), r.PathValue("project_id"), in)
	s.oaResult(w, r, v, err)
}
func (s *Server) oaEnroll(w http.ResponseWriter, r *http.Request) {
	var in projects.RevisionCommand
	if !decodeJSON(w, r, &in) {
		return
	}
	v, err := s.oaProduction.Enroll(r.Context(), actorFrom(r), r.PathValue("project_id"), in)
	s.oaResult(w, r, v, err)
}
func (s *Server) oaParticipantApplicable(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CommandID        string `json:"command_id"`
		ExpectedRevision int64  `json:"expected_revision"`
		NotApplicable    *bool  `json:"not_applicable"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if in.NotApplicable == nil {
		writeProblem(w, r, 400, "invalid_request", "必须明确 not_applicable", false, nil)
		return
	}
	v, err := s.oaProduction.SetParticipantApplicable(r.Context(), actorFrom(r), r.PathValue("project_id"), projects.RevisionCommand{CommandID: in.CommandID, ExpectedRevision: in.ExpectedRevision}, *in.NotApplicable)
	s.oaResult(w, r, v, err)
}
func (s *Server) oaProjectDisciplines(w http.ResponseWriter, r *http.Request) {
	if _, err := s.oaProduction.Personnel(r.Context(), principalFrom(r).TenantID, r.PathValue("project_id"), false); err != nil {
		s.oaFail(w, r, err)
		return
	}
	v, err := s.oaProduction.Disciplines(r.Context(), principalFrom(r).TenantID, false)
	s.oaResult(w, r, map[string]any{"profile": personnel.Profile, "items": v}, err)
}
func (s *Server) oaDisciplines(w http.ResponseWriter, r *http.Request) {
	includeArchived := false
	if raw := r.URL.Query().Get("include_archived"); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			writeProblem(w, r, 400, "invalid_request", "include_archived 必须为布尔值", false, nil)
			return
		}
		includeArchived = v
	}
	v, err := s.oaProduction.Disciplines(r.Context(), principalFrom(r).TenantID, includeArchived)
	allowed := []string{}
	if principalFrom(r).Can(auth.ActionOAProductionDisciplineManage) {
		allowed = append(allowed, auth.ActionOAProductionDisciplineManage)
	}
	s.oaResult(w, r, map[string]any{"profile": personnel.Profile, "items": v, "allowed_actions": allowed}, err)
}
func (s *Server) oaChangeDiscipline(w http.ResponseWriter, r *http.Request) {
	var in projects.OADisciplineCommand
	if !decodeJSON(w, r, &in) {
		return
	}
	v, err := s.oaProduction.ChangeDiscipline(r.Context(), actorFrom(r), in)
	s.oaResult(w, r, v, err)
}
func (s *Server) oaResult(w http.ResponseWriter, r *http.Request, v any, err error) {
	if err != nil {
		s.oaFail(w, r, err)
		return
	}
	writeJSON(w, 200, v)
}
func (s *Server) oaFail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, personnel.ErrInvalid):
		writeProblem(w, r, 400, "invalid_oa_personnel", "原 OA 分工意图无效，未提交", false, nil)
	case errors.Is(err, personnel.ErrStale):
		writeProblem(w, r, 409, "revision_conflict", "Project revision 已变化，请复核", false, nil)
	case errors.Is(err, personnel.ErrState):
		writeProblem(w, r, 409, "state_conflict", "当前状态不允许该分工操作", false, nil)
	case errors.Is(err, personnel.ErrTargetOccupied):
		writeProblem(w, r, 409, "discipline_occupied", "目标专业已有配置，不允许自动覆盖", false, nil)
	case errors.Is(err, personnel.ErrSource):
		writeProblem(w, r, 422, "reference_invalid", "人员或专业已失效或 revision 不匹配", false, nil)
	case errors.Is(err, projects.ErrOADisciplineNameConflict):
		writeProblem(w, r, 409, "discipline_name_conflict", "已有同名有效专业", false, nil)
	default:
		s.fail(w, r, err)
	}
}
