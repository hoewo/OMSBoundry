package httpserver

import (
	"context"
	"net/http"
	"omsbizfoundry/design-operations/internal/auth"
	"omsbizfoundry/design-operations/internal/design"
	"strconv"
)

type stageMaterialUseCases interface {
	StageWorkItems(context.Context, string, string, string) ([]design.WorkItem, error)
	ProductionProgress(context.Context, string, string, string) (design.ProductionProgressProjection, error)
	StageMaterials(context.Context, string, string, string) (design.StageMaterialCollection, error)
	RegisterStageMaterials(context.Context, design.Actor, string, string, design.StageMaterialInput) (design.StageMaterialCollection, error)
	WithdrawStageMaterial(context.Context, design.Actor, string, string, string, design.ReasonInput) (design.StageMaterialCollection, error)
	StageMaterialHistory(context.Context, string, string, int64, int) (design.HistoryPage, error)
	UpgradeStageMaterialDefinition(context.Context, design.Actor, string, design.DefinitionUpgradeInput) (design.Details, error)
}

func (s *Server) productionProgress(w http.ResponseWriter, r *http.Request) {
	uc, ok := s.stageCases(w, r)
	if !ok {
		return
	}
	project, typeKey := r.URL.Query().Get("project_id"), r.URL.Query().Get("type_key")
	if !design.ValidUUID(project) {
		writeProblem(w, r, http.StatusBadRequest, "INVALID_REQUEST", "invalid project_id", false)
		return
	}
	p := principalFrom(r)
	if !p.CanResource("project", project) {
		writeProblem(w, r, http.StatusForbidden, "SCOPE_DENIED", "需要精确 Project Scope", false)
		return
	}
	value, err := uc.ProductionProgress(r.Context(), p.TenantID, project, typeKey)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		design.ProductionProgressProjection
		AllowedActions []string `json:"allowed_actions"`
	}{value, []string{auth.ProductionActionStageMaterialRead}})
}

func (s *Server) upgradeStageMaterialDefinition(w http.ResponseWriter, r *http.Request) {
	if !s.stageWriteAllowed(w, r) {
		return
	}
	uc, ok := s.stageCases(w, r)
	if !ok {
		return
	}
	var input design.DefinitionUpgradeInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := uc.UpgradeStageMaterialDefinition(r.Context(), actorFrom(principalFrom(r)), r.PathValue("work_item_id"), input)
	s.writeResult(w, r, http.StatusOK, value, err)
}

func (s *Server) stageCases(w http.ResponseWriter, r *http.Request) (stageMaterialUseCases, bool) {
	uc, ok := s.useCases.(stageMaterialUseCases)
	if !ok {
		s.fail(w, r, design.ErrTypeInvalid)
	}
	return uc, ok
}
func (s *Server) stageWorkItems(w http.ResponseWriter, r *http.Request) {
	uc, ok := s.stageCases(w, r)
	if !ok {
		return
	}
	project, typeKey := r.URL.Query().Get("project_id"), r.URL.Query().Get("type_key")
	if !design.ValidUUID(project) {
		writeProblem(w, r, http.StatusBadRequest, "INVALID_REQUEST", "invalid project_id", false)
		return
	}
	p := principalFrom(r)
	if !p.CanResource("project", project) {
		writeProblem(w, r, http.StatusForbidden, "SCOPE_DENIED", "需要精确 Project Scope", false)
		return
	}
	items, err := uc.StageWorkItems(r.Context(), p.TenantID, project, typeKey)
	actions := []string{auth.ProductionActionStageMaterialRead}
	for _, a := range []string{auth.ProductionActionCreate, auth.ProductionActionStageMaterialRegister, auth.ProductionActionStageMaterialWithdraw, auth.ProductionActionStageMaterialHistoryRead} {
		if p.Can(a) {
			actions = append(actions, a)
		}
	}
	s.writeResult(w, r, http.StatusOK, map[string]any{"schema_version": "1.0", "profile": design.StageMaterialProfile, "tenant_id": p.TenantID, "project_id": project, "type_key": typeKey, "items": items, "allowed_actions": actions}, err)
}
func (s *Server) stageMaterials(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeWorkItem(w, r) {
		return
	}
	uc, ok := s.stageCases(w, r)
	if !ok {
		return
	}
	value, err := uc.StageMaterials(r.Context(), principalFrom(r).TenantID, r.PathValue("work_item_id"), r.PathValue("stage_key"))
	s.writeStageMaterials(w, r, value, err)
}
func (s *Server) stageWriteAllowed(w http.ResponseWriter, r *http.Request) bool {
	if !principalFrom(r).Can(auth.ProductionActionStageMaterialRead) {
		writeProblem(w, r, http.StatusForbidden, "CAPABILITY_DENIED", "资料写入同时需要读取能力", false)
		return false
	}
	return s.authorizeWorkItem(w, r)
}
func (s *Server) registerStageMaterials(w http.ResponseWriter, r *http.Request) {
	if !s.stageWriteAllowed(w, r) {
		return
	}
	uc, ok := s.stageCases(w, r)
	if !ok {
		return
	}
	var input design.StageMaterialInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := uc.RegisterStageMaterials(r.Context(), actorFrom(principalFrom(r)), r.PathValue("work_item_id"), r.PathValue("stage_key"), input)
	s.writeStageMaterials(w, r, value, err)
}
func (s *Server) withdrawStageMaterial(w http.ResponseWriter, r *http.Request) {
	if !s.stageWriteAllowed(w, r) {
		return
	}
	uc, ok := s.stageCases(w, r)
	if !ok {
		return
	}
	var input design.ReasonInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := uc.WithdrawStageMaterial(r.Context(), actorFrom(principalFrom(r)), r.PathValue("work_item_id"), r.PathValue("stage_key"), r.PathValue("reference_id"), input)
	s.writeStageMaterials(w, r, value, err)
}
func (s *Server) writeStageMaterials(w http.ResponseWriter, r *http.Request, value design.StageMaterialCollection, err error) {
	if err != nil {
		s.fail(w, r, err)
		return
	}
	p := principalFrom(r)
	actions := []string{auth.ProductionActionStageMaterialRead}
	for _, a := range []string{auth.ProductionActionStageMaterialRegister, auth.ProductionActionStageMaterialWithdraw, auth.ProductionActionStageMaterialHistoryRead} {
		if p.Can(a) && (a == auth.ProductionActionStageMaterialHistoryRead || value.WorkItem.Status != design.StatusCancelled) {
			actions = append(actions, a)
		}
	}
	writeJSON(w, http.StatusOK, struct {
		design.StageMaterialCollection
		AllowedActions []string `json:"allowed_actions"`
	}{value, actions})
}
func (s *Server) stageMaterialHistory(w http.ResponseWriter, r *http.Request) {
	if !s.stageWriteAllowed(w, r) {
		return
	}
	uc, ok := s.stageCases(w, r)
	if !ok {
		return
	}
	after, limit := int64(0), 0
	var err error
	if raw := r.URL.Query().Get("after_sequence"); raw != "" {
		after, err = strconv.ParseInt(raw, 10, 64)
	}
	if err == nil {
		if raw := r.URL.Query().Get("limit"); raw != "" {
			limit, err = strconv.Atoi(raw)
		}
	}
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "INVALID_REQUEST", "invalid history query", false)
		return
	}
	value, err := uc.StageMaterialHistory(r.Context(), principalFrom(r).TenantID, r.PathValue("work_item_id"), after, limit)
	s.writeResult(w, r, http.StatusOK, value, err)
}
