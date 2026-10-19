package auth

import (
	"errors"
	"net/http"
	"strings"
)

const (
	DesignAudience                           = "design-operations"
	ProductionAudience                       = "production-operations"
	ActionCreate                             = "design-work-item.create"
	ActionRead                               = "design-work-item.read"
	ActionOwnerManage                        = "design-work-item.owner.manage"
	ActionDeliverableManage                  = "design-work-item.deliverable.manage"
	ActionStageManage                        = "design-work-item.stage.manage"
	ActionCancel                             = "design-work-item.cancel"
	ActionHistoryRead                        = "design-work-item.history.read"
	ActionReferenceRead                      = "design-work-item.reference.read"
	ProductionActionCreate                   = "production-work-item.create"
	ProductionActionRead                     = "production-work-item.read"
	ProductionActionOwnerManage              = "production-work-item.owner.manage"
	ProductionActionDeliverableManage        = "production-work-item.deliverable.manage"
	ProductionActionStageManage              = "production-work-item.stage.manage"
	ProductionActionCancel                   = "production-work-item.cancel"
	ProductionActionHistoryRead              = "production-work-item.history.read"
	ProductionActionReferenceRead            = "production-work-item.reference.read"
	ProductionActionWorkItemsRead            = "production-work-item.work-items.read"
	ProductionActionStageMaterialRead        = "production-work-item.stage-material.read"
	ProductionActionStageMaterialRegister    = "production-work-item.stage-material.register"
	ProductionActionStageMaterialWithdraw    = "production-work-item.stage-material.withdraw"
	ProductionActionStageMaterialHistoryRead = "production-work-item.stage-material.history.read"
)

var AllActions = []string{ActionCreate, ActionRead, ActionOwnerManage, ActionDeliverableManage, ActionStageManage, ActionCancel, ActionHistoryRead, ActionReferenceRead}
var ProductionActions = []string{ProductionActionCreate, ProductionActionRead, ProductionActionOwnerManage, ProductionActionDeliverableManage, ProductionActionStageManage, ProductionActionCancel, ProductionActionHistoryRead, ProductionActionReferenceRead, ProductionActionWorkItemsRead, ProductionActionStageMaterialRead, ProductionActionStageMaterialRegister, ProductionActionStageMaterialWithdraw, ProductionActionStageMaterialHistoryRead}
var ErrUnauthenticated = errors.New("authentication required")

type Principal struct {
	SubjectID, TenantID, DisplayName, Provider, CredentialID, ScopeKind string
	AllowedActions                                                      map[string]bool
	Resources                                                           map[string]map[string]bool
}

func (p Principal) Can(action string) bool { return p.AllowedActions[action] }
func (p Principal) CanResource(kind, id string) bool {
	return p.ScopeKind == "tenant" || (p.ScopeKind == "resources" && p.Resources[kind][id])
}
func (p Principal) CanWorkItem(workItemID, projectID string) bool {
	return p.ScopeKind == "tenant" || p.CanResource("design-work-item", workItemID) || p.CanResource("project", projectID)
}
func (p Principal) CanProductionWorkItem(workItemID, projectID string) bool {
	return p.ScopeKind == "tenant" || p.CanResource("production-work-item", workItemID) || p.CanResource("project", projectID)
}

type Authenticator interface {
	Authenticate(*http.Request) (Principal, error)
	ProviderName() string
}

func bearerToken(r *http.Request) string {
	raw := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(raw) > 7 && strings.EqualFold(raw[:7], "Bearer ") {
		return strings.TrimSpace(raw[7:])
	}
	return ""
}
func includes(values []string, candidate string) bool {
	for _, v := range values {
		if v == candidate {
			return true
		}
	}
	return false
}
