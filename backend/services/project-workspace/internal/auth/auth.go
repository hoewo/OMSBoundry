package auth

import (
	"errors"
	"net/http"
	"strings"
)

const (
	ActionCreate                                = "project-workspace.create"
	ActionRead                                  = "project-workspace.read"
	ActionCommercialSummaryRead                 = "project-workspace.commercial-summary.read"
	ActionMaterialsRead                         = "project-workspace.materials.read"
	ActionUpdate                                = "project-workspace.update"
	ActionProfileCorrect                        = "project-workspace.profile.correct"
	ActionAssignmentManage                      = "project-workspace.assignment.manage"
	ActionAssignmentBusinessOwnerManage         = "project-workspace.assignment.business-owner.manage"
	ActionAssignmentProductionOwnerManage       = "project-workspace.assignment.production-owner.manage"
	ActionAssignmentParticipantManage           = "project-workspace.assignment.participant.manage"
	ActionAssignmentCommercialContributorManage = "project-workspace.assignment.commercial-contributor.manage"
	ActionResourceManage                        = "project-workspace.resource-link.manage"
	ActionComplete                              = "project-workspace.initiation.complete"
	ActionCancel                                = "project-workspace.cancel"
	ActionHistoryRead                           = "project-workspace.history.read"
	ActionPolicyFactRead                        = "project-workspace.policy-fact.read"
	ActionHierarchyManage                       = "project-workspace.hierarchy.manage"
	ActionCompletionRead                        = "project-workspace.completion.read"
	ActionCompletionConfirm                     = "project-workspace.completion.confirm"
)

var AllActions = []string{
	ActionCreate,
	ActionRead,
	ActionCommercialSummaryRead,
	ActionMaterialsRead,
	ActionUpdate,
	ActionProfileCorrect,
	ActionAssignmentManage,
	ActionAssignmentBusinessOwnerManage,
	ActionAssignmentProductionOwnerManage,
	ActionAssignmentParticipantManage,
	ActionAssignmentCommercialContributorManage,
	ActionResourceManage,
	ActionComplete,
	ActionCancel,
	ActionHistoryRead,
	ActionPolicyFactRead,
	ActionHierarchyManage,
	ActionCompletionRead,
	ActionCompletionConfirm,
	ActionOAProductionPersonnelRead,
	ActionOAProductionPersonnelManage,
	ActionOAProductionDisciplineRead,
	ActionOAProductionDisciplineManage,
	ActionCounterpartyContextRead,
	ActionCounterpartyContextManage,
}
var ErrUnauthenticated = errors.New("authentication required")

const ActionCounterpartyContextRead = "project-workspace.counterparty-context.read"
const ActionCounterpartyContextManage = "project-workspace.counterparty-context.manage"
const ActionOAProductionPersonnelRead = "project-workspace.reference-oa-personnel.read"
const ActionOAProductionPersonnelManage = "project-workspace.reference-oa-personnel.manage"
const ActionOAProductionDisciplineRead = "project-workspace.reference-oa-discipline.read"
const ActionOAProductionDisciplineManage = "project-workspace.reference-oa-discipline.manage"

type Principal struct {
	SubjectID      string
	TenantID       string
	DisplayName    string
	Provider       string
	CredentialID   string
	AllowedActions map[string]bool
	ScopeKind      string
	ResourceIDs    map[string]bool
}

func (p Principal) Can(action string) bool { return p.AllowedActions[action] }
func (p Principal) CanProject(projectID string) bool {
	return p.ScopeKind == "tenant" || (p.ScopeKind == "resources" && p.ResourceIDs[projectID])
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

func validAction(action string) bool {
	for _, candidate := range AllActions {
		if action == candidate {
			return true
		}
	}
	return false
}
