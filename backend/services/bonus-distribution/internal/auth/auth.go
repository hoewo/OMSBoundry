package auth

import (
	"errors"
	"net/http"
	"strings"

	"omsbizfoundry/bonus-distribution/internal/bonus"
	"omsbizfoundry/bonus-distribution/internal/referencebonus"
)

var (
	PoolActions = []string{
		bonus.ActionBusinessCalculate, bonus.ActionBusinessPublish, bonus.ActionBusinessSupersede, bonus.ActionBusinessWithdraw,
		bonus.ActionProductionCalculate, bonus.ActionProductionPublish, bonus.ActionProductionSupersede, bonus.ActionProductionWithdraw,
		bonus.ActionPoolRead, bonus.ActionPoolHistory,
	}
	AllocationActions  = []string{bonus.ActionAllocationManage, bonus.ActionAllocationRead, bonus.ActionAllocationAmountRead}
	SuggestionActions  = []string{bonus.ActionSuggestionCreate, bonus.ActionSuggestionRead, bonus.ActionSuggestionWithdraw}
	UserActions        = append(append(append(append([]string(nil), PoolActions...), AllocationActions...), SuggestionActions...), referencebonus.Actions...)
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

func (p Principal) CanPool(poolID, projectID string) bool {
	return p.CanResource("bonus_pool", poolID) || p.CanResource("project", projectID)
}

func (p Principal) CanAllocation(allocationID, poolID, projectID string) bool {
	return p.CanResource("bonus_allocation", allocationID) || p.CanPool(poolID, projectID)
}

func (p Principal) CanSuggestion(suggestionID, poolID, projectID string) bool {
	return p.CanResource("bonus_payable_suggestion", suggestionID) || p.CanPool(poolID, projectID)
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
		if action == candidate {
			return true
		}
	}
	return false
}
