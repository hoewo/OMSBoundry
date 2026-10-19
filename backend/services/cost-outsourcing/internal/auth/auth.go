package auth

import (
	"errors"
	"net/http"
	"strings"
)

const (
	ActionCostCreate         = "cost-entry.create"
	ActionCostRead           = "cost-entry.read"
	ActionCostAmountRead     = "cost-entry.amount.read"
	ActionCostCorrect        = "cost-entry.correct"
	ActionCostWithdraw       = "cost-entry.withdraw"
	ActionCostEvidenceManage = "cost-entry.evidence.manage"
	ActionCostHistory        = "cost-entry.history.read"

	ActionEngagementCreate         = "outsourcing-engagement.create"
	ActionEngagementRead           = "outsourcing-engagement.read"
	ActionEngagementAmountRead     = "outsourcing-engagement.amount.read"
	ActionEngagementCorrect        = "outsourcing-engagement.correct"
	ActionEngagementWithdraw       = "outsourcing-engagement.withdraw"
	ActionEngagementAssess         = "outsourcing-engagement.assess"
	ActionEngagementEvidenceManage = "outsourcing-engagement.evidence.manage"
	ActionEngagementHistory        = "outsourcing-engagement.history.read"

	ActionSuggestionCreate   = "outsourcing-payable-suggestion.create"
	ActionSuggestionRead     = "outsourcing-payable-suggestion.read"
	ActionSuggestionWithdraw = "outsourcing-payable-suggestion.withdraw"
)

var (
	CostActions        = []string{ActionCostCreate, ActionCostRead, ActionCostAmountRead, ActionCostCorrect, ActionCostWithdraw, ActionCostEvidenceManage, ActionCostHistory}
	EngagementActions  = []string{ActionEngagementCreate, ActionEngagementRead, ActionEngagementAmountRead, ActionEngagementCorrect, ActionEngagementWithdraw, ActionEngagementAssess, ActionEngagementEvidenceManage, ActionEngagementHistory}
	SuggestionActions  = []string{ActionSuggestionCreate, ActionSuggestionRead, ActionSuggestionWithdraw}
	UserActions        = append(append(append([]string(nil), CostActions...), EngagementActions...), SuggestionActions...)
	ErrUnauthenticated = errors.New("authentication required")
)

type Principal struct {
	SubjectID, TenantID, DisplayName, Provider, CredentialID, ScopeKind string
	AllowedActions                                                      map[string]bool
	Resources                                                           map[string]map[string]bool
}

func (p Principal) Can(action string) bool { return p.AllowedActions[action] }

func (p Principal) CanResource(kind, id string) bool {
	return p.ScopeKind == "tenant" || (p.ScopeKind == "resources" && p.Resources[kind][id])
}

func (p Principal) CanCost(id, projectID string) bool {
	return p.CanResource("cost_entry", id) || p.CanResource("project", projectID)
}

func (p Principal) CanEngagement(id, projectID string) bool {
	return p.CanResource("outsourcing_engagement", id) || p.CanResource("project", projectID)
}

func (p Principal) CanSuggestion(id, projectID string) bool {
	return p.CanResource("payable_suggestion", id) || p.CanResource("project", projectID)
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
	for _, candidate := range UserActions {
		if candidate == action {
			return true
		}
	}
	return false
}
