package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"omsbizfoundry/design-operations/internal/auth"
	"omsbizfoundry/design-operations/internal/design"
)

const maxRequestBytes = 1 << 20

type UseCases interface {
	Ping(context.Context) error
	List(context.Context, string, int) ([]design.WorkItem, error)
	WorkItemsForSubject(context.Context, string, string, int) ([]design.WorkItemCandidate, error)
	Create(context.Context, design.Actor, design.CreateInput) (design.Details, error)
	SetOwner(context.Context, design.Actor, string, design.OwnerInput) (design.Details, error)
	PutDeliverable(context.Context, design.Actor, string, string, design.DeliverableInput) (design.Deliverable, error)
	SetApplicability(context.Context, design.Actor, string, string, design.ApplicabilityInput) (design.Details, error)
	CompleteStage(context.Context, design.Actor, string, string, design.RevisionInput) (design.Details, error)
	ReopenStage(context.Context, design.Actor, string, string, design.ReasonInput) (design.Details, error)
	Cancel(context.Context, design.Actor, string, design.ReasonInput) (design.Details, error)
	GetOwned(context.Context, string, string) (design.Details, error)
	Get(context.Context, string, string) (design.Details, error)
	History(context.Context, string, string, int64, int) (design.HistoryPage, error)
	Reference(context.Context, string, string) (design.ReferenceView, error)
	ProductionReference(context.Context, string, string) (design.ReferenceView, error)
	ReferenceMaterials(context.Context, string, string) (design.MaterialReferenceProjection, error)
}

type Server struct {
	useCases                UseCases
	designAuthenticator     auth.Authenticator
	productionAuthenticator auth.Authenticator
	productionSurface       bool
	handler                 http.Handler
}

type principalKey struct{}
type requestIDKey struct{}
type productionAPIKey struct{}

func New(useCases UseCases, authenticator auth.Authenticator) (*Server, error) {
	return newServer(useCases, authenticator, nil)
}

func NewWithProduction(useCases UseCases, designAuthenticator, productionAuthenticator auth.Authenticator) (*Server, error) {
	if productionAuthenticator == nil {
		return nil, fmt.Errorf("production authenticator is required")
	}
	return newServer(useCases, designAuthenticator, productionAuthenticator)
}

func newServer(useCases UseCases, designAuthenticator, productionAuthenticator auth.Authenticator) (*Server, error) {
	if useCases == nil || designAuthenticator == nil {
		return nil, fmt.Errorf("use cases and authenticator are required")
	}
	s := &Server{useCases: useCases, designAuthenticator: designAuthenticator, productionAuthenticator: productionAuthenticator, productionSurface: productionAuthenticator != nil}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", s.live)
	mux.HandleFunc("GET /health/ready", s.ready)
	mux.HandleFunc("GET /assets/demo.js", demoScript(s.productionSurface))
	mux.HandleFunc("GET /", s.index)
	mux.Handle("GET /api/v1/design-work-items", s.protected(designAuthenticator, auth.ActionRead, http.HandlerFunc(s.list)))
	mux.Handle("POST /api/v1/design-work-items", s.protected(designAuthenticator, auth.ActionCreate, http.HandlerFunc(s.create)))
	mux.Handle("GET /api/v1/design-work-items/{work_item_id}", s.protected(designAuthenticator, auth.ActionRead, http.HandlerFunc(s.get)))
	mux.Handle("POST /api/v1/design-work-items/{work_item_id}/owner", s.protected(designAuthenticator, auth.ActionOwnerManage, http.HandlerFunc(s.setOwner)))
	mux.Handle("POST /api/v1/design-work-items/{work_item_id}/stages/{stage_key}/deliverables", s.protected(designAuthenticator, auth.ActionDeliverableManage, http.HandlerFunc(s.putDeliverable)))
	mux.Handle("POST /api/v1/design-work-items/{work_item_id}/stages/{stage_key}/applicability", s.protected(designAuthenticator, auth.ActionStageManage, http.HandlerFunc(s.setApplicability)))
	mux.Handle("POST /api/v1/design-work-items/{work_item_id}/stages/{stage_key}/complete", s.protected(designAuthenticator, auth.ActionStageManage, http.HandlerFunc(s.completeStage)))
	mux.Handle("POST /api/v1/design-work-items/{work_item_id}/stages/{stage_key}/reopen", s.protected(designAuthenticator, auth.ActionStageManage, http.HandlerFunc(s.reopenStage)))
	mux.Handle("POST /api/v1/design-work-items/{work_item_id}/cancel", s.protected(designAuthenticator, auth.ActionCancel, http.HandlerFunc(s.cancel)))
	mux.Handle("GET /api/v1/design-work-items/{work_item_id}/history", s.protected(designAuthenticator, auth.ActionHistoryRead, http.HandlerFunc(s.history)))
	mux.Handle("GET /api/v1/references/design-work-items/{work_item_id}", s.protected(designAuthenticator, auth.ActionReferenceRead, http.HandlerFunc(s.reference)))
	if productionAuthenticator != nil {
		mux.Handle("GET /api/v1/production-stage-work-items", s.protected(productionAuthenticator, auth.ProductionActionStageMaterialRead, productionAPI(http.HandlerFunc(s.stageWorkItems))))
		mux.Handle("GET /api/v1/production-stage-progress", s.protected(productionAuthenticator, auth.ProductionActionStageMaterialRead, productionAPI(http.HandlerFunc(s.productionProgress))))
		mux.Handle("GET /api/v1/production-work-items/{work_item_id}/stages/{stage_key}/material-collection", s.protected(productionAuthenticator, auth.ProductionActionStageMaterialRead, productionAPI(http.HandlerFunc(s.stageMaterials))))
		mux.Handle("POST /api/v1/production-work-items/{work_item_id}/stages/{stage_key}/material-collection", s.protected(productionAuthenticator, auth.ProductionActionStageMaterialRegister, productionAPI(http.HandlerFunc(s.registerStageMaterials))))
		mux.Handle("POST /api/v1/production-work-items/{work_item_id}/stage-material-definition-upgrade", s.protected(productionAuthenticator, auth.ProductionActionStageMaterialRegister, productionAPI(http.HandlerFunc(s.upgradeStageMaterialDefinition))))
		mux.Handle("POST /api/v1/production-work-items/{work_item_id}/stages/{stage_key}/material-collection/{reference_id}/withdraw", s.protected(productionAuthenticator, auth.ProductionActionStageMaterialWithdraw, productionAPI(http.HandlerFunc(s.withdrawStageMaterial))))
		mux.Handle("GET /api/v1/production-work-items/{work_item_id}/stage-material-history", s.protected(productionAuthenticator, auth.ProductionActionStageMaterialHistoryRead, productionAPI(http.HandlerFunc(s.stageMaterialHistory))))
		mux.Handle("GET /api/v1/production-work-items", s.protected(productionAuthenticator, auth.ProductionActionRead, productionAPI(http.HandlerFunc(s.list))))
		mux.Handle("POST /api/v1/production-work-items", s.protected(productionAuthenticator, auth.ProductionActionCreate, productionAPI(http.HandlerFunc(s.create))))
		mux.Handle("GET /api/v1/production-work-items/{work_item_id}", s.protected(productionAuthenticator, auth.ProductionActionRead, productionAPI(http.HandlerFunc(s.get))))
		mux.Handle("POST /api/v1/production-work-items/{work_item_id}/owner", s.protected(productionAuthenticator, auth.ProductionActionOwnerManage, productionAPI(http.HandlerFunc(s.setOwner))))
		mux.Handle("POST /api/v1/production-work-items/{work_item_id}/stages/{stage_key}/deliverables", s.protected(productionAuthenticator, auth.ProductionActionDeliverableManage, productionAPI(http.HandlerFunc(s.putDeliverable))))
		mux.Handle("POST /api/v1/production-work-items/{work_item_id}/stages/{stage_key}/applicability", s.protected(productionAuthenticator, auth.ProductionActionStageManage, productionAPI(http.HandlerFunc(s.setApplicability))))
		mux.Handle("POST /api/v1/production-work-items/{work_item_id}/stages/{stage_key}/complete", s.protected(productionAuthenticator, auth.ProductionActionStageManage, productionAPI(http.HandlerFunc(s.completeStage))))
		mux.Handle("POST /api/v1/production-work-items/{work_item_id}/stages/{stage_key}/reopen", s.protected(productionAuthenticator, auth.ProductionActionStageManage, productionAPI(http.HandlerFunc(s.reopenStage))))
		mux.Handle("POST /api/v1/production-work-items/{work_item_id}/cancel", s.protected(productionAuthenticator, auth.ProductionActionCancel, productionAPI(http.HandlerFunc(s.cancel))))
		mux.Handle("GET /api/v1/production-work-items/{work_item_id}/history", s.protected(productionAuthenticator, auth.ProductionActionHistoryRead, productionAPI(http.HandlerFunc(s.history))))
		mux.Handle("GET /api/v1/references/production-work-items/{work_item_id}", s.protected(productionAuthenticator, auth.ProductionActionReferenceRead, productionAPI(http.HandlerFunc(s.reference))))
		mux.Handle("GET /api/v1/references/production-work-items/{work_item_id}/materials", s.protected(productionAuthenticator, auth.ProductionActionReferenceRead, productionAPI(http.HandlerFunc(s.referenceMaterials))))
		mux.Handle("GET /api/v1/work-items", s.protected(productionAuthenticator, auth.ProductionActionWorkItemsRead, productionAPI(http.HandlerFunc(s.workItems))))
	}
	s.handler = requestContext(securityHeaders(mux))
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }

func (s *Server) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"service": "design-operations", "status": "live"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.useCases.Ping(r.Context()); err != nil {
		writeProblem(w, r, http.StatusServiceUnavailable, "NOT_READY", "数据库暂不可用", true)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"service": "design-operations", "status": "ready"})
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	renderOverview(w, s.productionSurface)
}

func (s *Server) protected(authenticator auth.Authenticator, action string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stripUntrustedIdentityHeaders(r.Header)
		principal, err := authenticator.Authenticate(r)
		if err != nil {
			writeProblem(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "需要有效访问凭证", false)
			return
		}
		if !principal.Can(action) {
			writeProblem(w, r, http.StatusForbidden, "CAPABILITY_DENIED", "缺少 "+action, false)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, principal)))
	})
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	var input design.CreateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if isProductionAPI(r) {
		input.TypeKey = strings.ToLower(strings.TrimSpace(input.TypeKey))
		if input.TypeKey == "" {
			writeProblem(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Production API 必须指定 type_key", false)
			return
		}
	} else {
		if input.TypeDefinitionVersion > 1 {
			writeProblem(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Design API 只接受 version 1", false)
			return
		}
		if input.TypeKey != "" && !strings.EqualFold(strings.TrimSpace(input.TypeKey), design.DesignTypeKey) {
			writeProblem(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Design API 只接受 design-v1", false)
			return
		}
		input.TypeKey = design.DesignTypeKey
	}
	principal := principalFrom(r)
	if design.IsStageMaterialDefinitionVersion(input.TypeDefinitionVersion) && (!principal.Can(auth.ProductionActionStageMaterialRead) || !principal.Can(auth.ProductionActionStageMaterialRegister)) {
		writeProblem(w, r, http.StatusForbidden, "CAPABILITY_DENIED", "创建资料集合版本需要资料读取和登记能力", false)
		return
	}
	if !principal.CanResource("project", strings.ToLower(strings.TrimSpace(input.ProjectID))) {
		writeProblem(w, r, http.StatusForbidden, "SCOPE_DENIED", "凭证范围不包含该 Project", false)
		return
	}
	value, err := s.useCases.Create(r.Context(), actorFrom(principal), input)
	s.writeResult(w, r, http.StatusCreated, value, err)
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			writeProblem(w, r, http.StatusBadRequest, "INVALID_REQUEST", "limit 必须是整数", false)
			return
		}
		limit = value
	}
	principal := principalFrom(r)
	items, err := s.useCases.List(r.Context(), principal.TenantID, limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	visible := items[:0]
	for _, item := range items {
		if !isProductionAPI(r) && (item.TypeDefinitionVersion > 1 || (item.TypeKey != "" && item.TypeKey != design.DesignTypeKey)) {
			continue
		}
		allowed := principal.CanWorkItem(item.ID, item.ProjectID)
		if isProductionAPI(r) {
			allowed = principal.CanProductionWorkItem(item.ID, item.ProjectID)
		}
		if allowed {
			visible = append(visible, item)
		}
	}
	writeJSON(w, http.StatusOK, workItemListResponse{
		Items:          visible,
		AllowedActions: credentialActions(principal, isProductionAPI(r)),
	})
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeWorkItem(w, r) {
		return
	}
	principal := principalFrom(r)
	value, err := s.useCases.Get(r.Context(), principal.TenantID, r.PathValue("work_item_id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, workItemDetailsResponse{
		Details:        value,
		AllowedActions: detailsAllowedActions(value, principal, isProductionAPI(r)),
	})
}

type workItemListResponse struct {
	Items          []design.WorkItem `json:"items"`
	AllowedActions []string          `json:"allowed_actions"`
}

type workItemDetailsResponse struct {
	design.Details
	AllowedActions []string `json:"allowed_actions"`
}

type workItemResourceScope struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type workItemProjection struct {
	SchemaVersion     string                `json:"schema_version"`
	SourceApplication string                `json:"source_application"`
	WorkItemID        string                `json:"work_item_id"`
	Kind              string                `json:"kind"`
	TenantID          string                `json:"tenant_id"`
	SubjectID         string                `json:"subject_id"`
	ProjectID         string                `json:"project_id,omitempty"`
	ResourceScope     workItemResourceScope `json:"resource_scope"`
	Reason            string                `json:"reason"`
	State             string                `json:"state"`
	Action            string                `json:"action"`
	ResourceRevision  int64                 `json:"resource_revision"`
	DueAt             *time.Time            `json:"due_at,omitempty"`
	ObservedAt        time.Time             `json:"observed_at"`
	DeepLink          string                `json:"deep_link"`
	AllowedActions    []string              `json:"allowed_actions"`
}

type workItemProjectionPage struct {
	SchemaVersion     string               `json:"schema_version"`
	SourceApplication string               `json:"source_application"`
	SubjectID         string               `json:"subject_id"`
	ObservedAt        time.Time            `json:"observed_at"`
	Items             []workItemProjection `json:"items"`
}

func (s *Server) workItems(w http.ResponseWriter, r *http.Request) {
	principal := principalFrom(r)
	candidates, err := s.useCases.WorkItemsForSubject(r.Context(), principal.TenantID, principal.SubjectID, 100)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	observedAt := time.Now().UTC()
	items := make([]workItemProjection, 0, len(candidates))
	for _, candidate := range candidates {
		item := candidate.WorkItem
		if !principal.CanProductionWorkItem(item.ID, item.ProjectID) {
			continue
		}
		allowed := projectionAllowedActions(principal)
		if len(allowed) == 0 {
			continue
		}
		state := item.Status
		reason := "生产工作项需要处理"
		if item.CurrentStageKey != nil {
			state += ":" + *item.CurrentStageKey
			reason = "当前生产阶段 “" + *item.CurrentStageKey + "” 需要处理"
		}
		items = append(items, workItemProjection{
			SchemaVersion: "1", SourceApplication: "production-operations", WorkItemID: item.ID, Kind: item.TypeKey,
			TenantID: principal.TenantID, SubjectID: principal.SubjectID, ProjectID: item.ProjectID,
			ResourceScope: workItemResourceScope{Type: "production-work-item", ID: item.ID}, Reason: reason,
			State: state, Action: allowed[0], ResourceRevision: item.Revision, ObservedAt: observedAt,
			DeepLink: "/?work_item_id=" + item.ID, AllowedActions: allowed,
		})
	}
	writeJSON(w, http.StatusOK, workItemProjectionPage{SchemaVersion: "1", SourceApplication: "production-operations", SubjectID: principal.SubjectID, ObservedAt: observedAt, Items: items})
}

func projectionAllowedActions(principal auth.Principal) []string {
	catalog := []string{auth.ProductionActionStageManage, auth.ProductionActionDeliverableManage, auth.ProductionActionCancel}
	actions := make([]string, 0, len(catalog))
	for _, action := range catalog {
		if principal.Can(action) {
			actions = append(actions, action)
		}
	}
	return actions
}

func credentialActions(principal auth.Principal, production bool) []string {
	catalog := auth.AllActions
	if production {
		catalog = auth.ProductionActions
	}
	actions := make([]string, 0, len(catalog))
	for _, action := range catalog {
		if principal.Can(action) {
			actions = append(actions, action)
		}
	}
	return actions
}

func detailsAllowedActions(value design.Details, principal auth.Principal, production bool) []string {
	allowed := map[string]bool{}
	for _, action := range credentialActions(principal, production) {
		allowed[action] = true
	}
	// Collection data/actions live behind their own capability-bound port, never generic read.
	for _, action := range []string{auth.ProductionActionStageMaterialRead, auth.ProductionActionStageMaterialRegister, auth.ProductionActionStageMaterialWithdraw, auth.ProductionActionStageMaterialHistoryRead} {
		allowed[action] = false
	}
	if design.IsStageMaterialDefinitionVersion(value.WorkItem.TypeDefinitionVersion) {
		allowed[auth.ProductionActionDeliverableManage] = false
		allowed[auth.ProductionActionStageManage] = false
		allowed[auth.ProductionActionHistoryRead] = false
	}
	if value.WorkItem.Status == design.StatusCancelled {
		if production {
			allowed[auth.ProductionActionOwnerManage] = false
			allowed[auth.ProductionActionDeliverableManage] = false
			allowed[auth.ProductionActionStageManage] = false
			allowed[auth.ProductionActionCancel] = false
		} else {
			allowed[auth.ActionOwnerManage] = false
			allowed[auth.ActionDeliverableManage] = false
			allowed[auth.ActionStageManage] = false
			allowed[auth.ActionCancel] = false
		}
	}
	catalog := auth.AllActions
	if production {
		catalog = auth.ProductionActions
	}
	actions := make([]string, 0, len(catalog))
	for _, action := range catalog {
		if allowed[action] {
			actions = append(actions, action)
		}
	}
	return actions
}

func (s *Server) setOwner(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeWorkItem(w, r) {
		return
	}
	var input design.OwnerInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.SetOwner(r.Context(), actorFrom(principalFrom(r)), r.PathValue("work_item_id"), input)
	s.writeResult(w, r, http.StatusOK, value, err)
}

func (s *Server) putDeliverable(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeWorkItem(w, r) {
		return
	}
	var input design.DeliverableInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.PutDeliverable(r.Context(), actorFrom(principalFrom(r)), r.PathValue("work_item_id"), r.PathValue("stage_key"), input)
	s.writeResult(w, r, http.StatusOK, value, err)
}

func (s *Server) setApplicability(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeWorkItem(w, r) {
		return
	}
	var input design.ApplicabilityInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.SetApplicability(r.Context(), actorFrom(principalFrom(r)), r.PathValue("work_item_id"), r.PathValue("stage_key"), input)
	s.writeResult(w, r, http.StatusOK, value, err)
}

func (s *Server) completeStage(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeWorkItem(w, r) {
		return
	}
	var input design.RevisionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.CompleteStage(r.Context(), actorFrom(principalFrom(r)), r.PathValue("work_item_id"), r.PathValue("stage_key"), input)
	s.writeResult(w, r, http.StatusOK, value, err)
}

func (s *Server) reopenStage(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeWorkItem(w, r) {
		return
	}
	var input design.ReasonInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.ReopenStage(r.Context(), actorFrom(principalFrom(r)), r.PathValue("work_item_id"), r.PathValue("stage_key"), input)
	s.writeResult(w, r, http.StatusOK, value, err)
}

func (s *Server) cancel(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeWorkItem(w, r) {
		return
	}
	var input design.ReasonInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.Cancel(r.Context(), actorFrom(principalFrom(r)), r.PathValue("work_item_id"), input)
	s.writeResult(w, r, http.StatusOK, value, err)
}

func (s *Server) history(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeWorkItem(w, r) {
		return
	}
	owned, err := s.useCases.GetOwned(r.Context(), principalFrom(r).TenantID, r.PathValue("work_item_id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if design.IsStageMaterialDefinitionVersion(owned.WorkItem.TypeDefinitionVersion) {
		s.fail(w, r, design.ErrTypeInvalid)
		return
	}
	after, limit := int64(0), 0
	if raw := r.URL.Query().Get("after_sequence"); raw != "" {
		after, err = strconv.ParseInt(raw, 10, 64)
	}
	if err == nil {
		if raw := r.URL.Query().Get("limit"); raw != "" {
			limit, err = strconv.Atoi(raw)
		}
	}
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "INVALID_REQUEST", "分页参数无效", false)
		return
	}
	principal := principalFrom(r)
	value, err := s.useCases.History(r.Context(), principal.TenantID, r.PathValue("work_item_id"), after, limit)
	s.writeResult(w, r, http.StatusOK, value, err)
}

func (s *Server) reference(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeWorkItem(w, r) {
		return
	}
	principal := principalFrom(r)
	var value design.ReferenceView
	var err error
	if isProductionAPI(r) {
		value, err = s.useCases.ProductionReference(r.Context(), principal.TenantID, r.PathValue("work_item_id"))
	} else {
		value, err = s.useCases.Reference(r.Context(), principal.TenantID, r.PathValue("work_item_id"))
	}
	s.writeResult(w, r, http.StatusOK, value, err)
}

func (s *Server) referenceMaterials(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeWorkItem(w, r) {
		return
	}
	principal := principalFrom(r)
	// The native reference capability manifest admits Work Item resources,
	// not Project-wide reference grants. Keep this new port equally narrow.
	if !principal.CanResource("production-work-item", r.PathValue("work_item_id")) {
		writeProblem(w, r, http.StatusForbidden, "SCOPE_DENIED", "资料引用需要精确 Production Work Item 或 Tenant Scope", false)
		return
	}
	value, err := s.useCases.ReferenceMaterials(r.Context(), principal.TenantID, r.PathValue("work_item_id"))
	s.writeResult(w, r, http.StatusOK, value, err)
}

func (s *Server) authorizeWorkItem(w http.ResponseWriter, r *http.Request) bool {
	principal := principalFrom(r)
	owned, err := s.useCases.GetOwned(r.Context(), principal.TenantID, r.PathValue("work_item_id"))
	if err != nil {
		s.fail(w, r, err)
		return false
	}
	if !isProductionAPI(r) && (owned.WorkItem.TypeDefinitionVersion > 1 || (owned.WorkItem.TypeKey != "" && owned.WorkItem.TypeKey != design.DesignTypeKey)) {
		writeProblem(w, r, http.StatusNotFound, "WORK_ITEM_NOT_FOUND", "Design Work Item 不存在", false)
		return false
	}
	allowed := principal.CanWorkItem(owned.WorkItem.ID, owned.WorkItem.ProjectID)
	if isProductionAPI(r) {
		allowed = principal.CanProductionWorkItem(owned.WorkItem.ID, owned.WorkItem.ProjectID)
	}
	if !allowed {
		writeProblem(w, r, http.StatusForbidden, "SCOPE_DENIED", "凭证范围不包含该 Design Work Item", false)
		return false
	}
	return true
}

func productionAPI(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), productionAPIKey{}, true)))
	})
}

func isProductionAPI(r *http.Request) bool {
	value, _ := r.Context().Value(productionAPIKey{}).(bool)
	return value
}

func actorFrom(p auth.Principal) design.Actor {
	kind := "iam_subject"
	if p.Provider == "template" {
		kind = "template_subject"
	}
	return design.Actor{Kind: kind, ID: p.SubjectID, DisplayName: p.DisplayName, TenantID: p.TenantID}
}

func principalFrom(r *http.Request) auth.Principal {
	return r.Context().Value(principalKey{}).(auth.Principal)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	if contentType := r.Header.Get("Content-Type"); contentType != "" && !strings.HasPrefix(strings.ToLower(contentType), "application/json") {
		writeProblem(w, r, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "请求体必须使用 application/json", false)
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "INVALID_JSON", "JSON 请求体无效", false)
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeProblem(w, r, http.StatusBadRequest, "INVALID_JSON", "JSON 请求体只能包含一个对象", false)
		return false
	}
	return true
}

func (s *Server) writeResult(w http.ResponseWriter, r *http.Request, status int, value any, err error) {
	if err == nil {
		writeJSON(w, status, value)
		return
	}
	s.fail(w, r, err)
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, design.ErrNotFound):
		writeProblem(w, r, http.StatusNotFound, "WORK_ITEM_NOT_FOUND", "Design Work Item 不存在", false)
	case errors.Is(err, design.ErrRevisionConflict):
		writeProblem(w, r, http.StatusConflict, "REVISION_CONFLICT", "Design revision 已变化", true)
	case errors.Is(err, design.ErrCommandConflict):
		writeProblem(w, r, http.StatusConflict, "COMMAND_PAYLOAD_CONFLICT", "command_id 已用于不同请求", false)
	case errors.Is(err, design.ErrActiveConflict):
		writeProblem(w, r, http.StatusConflict, "ACTIVE_WORK_ITEM_CONFLICT", "该 Project 已有 active Design Work Item", false)
	case errors.Is(err, design.ErrStateConflict):
		writeProblem(w, r, http.StatusConflict, "WORK_ITEM_STATE_CONFLICT", "当前 Work Item 状态不允许该操作", false)
	case errors.Is(err, design.ErrStageInvalid):
		writeProblem(w, r, http.StatusUnprocessableEntity, "STAGE_STATE_INVALID", "当前 Stage 状态或操作无效", false)
	case errors.Is(err, design.ErrTypeInvalid):
		writeProblem(w, r, http.StatusUnprocessableEntity, "TYPE_DEFINITION_INVALID", "Production Type Definition 不存在或版本无效", false)
	case errors.Is(err, design.ErrCompletionGaps):
		writeProblem(w, r, http.StatusUnprocessableEntity, "STAGE_COMPLETION_GAPS", "Stage 缺少有效负责人或必交成果", false)
	case errors.Is(err, design.ErrReferenceInvalid):
		writeProblem(w, r, http.StatusUnprocessableEntity, "REFERENCE_INVALID", "外部引用不存在、跨租户、已失效或 revision 不匹配", false)
	case errors.Is(err, design.ErrReferenceUnavailable):
		writeProblem(w, r, http.StatusServiceUnavailable, "REFERENCE_SERVICE_UNAVAILABLE", "外部引用服务暂不可用", true)
	default:
		if strings.HasPrefix(err.Error(), "invalid ") || strings.Contains(err.Error(), " is required") || strings.Contains(err.Error(), " must be ") || strings.Contains(err.Error(), "between") {
			writeProblem(w, r, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false)
			return
		}
		writeProblem(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Design Operations 暂时无法完成请求", true)
	}
}

func requestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Request-ID")))
		if !design.ValidUUID(requestID) {
			requestID = newUUID()
		}
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, requestID)))
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'unsafe-inline'; connect-src 'self'; frame-ancestors 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func stripUntrustedIdentityHeaders(header http.Header) {
	for _, name := range []string{"X-Actor-ID", "X-Actor-Name", "X-Tenant-ID", "X-IAM-Subject", "X-Allowed-Actions", "X-Capability-Scope"} {
		header.Del(name)
	}
}

func writeProblem(w http.ResponseWriter, r *http.Request, status int, code, message string, retryable bool) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "retryable": retryable, "request_id": requestIDFrom(r)}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func requestIDFrom(r *http.Request) string {
	value, _ := r.Context().Value(requestIDKey{}).(string)
	return value
}

func newUUID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(err)
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	raw := hex.EncodeToString(value[:])
	return raw[:8] + "-" + raw[8:12] + "-" + raw[12:16] + "-" + raw[16:20] + "-" + raw[20:]
}
