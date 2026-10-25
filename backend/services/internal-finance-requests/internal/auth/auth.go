package auth

import (
	"errors"
	"net/http"
	"strings"
)

const (
	ActionSelfRead      = "internal-finance-requests.self.read"
	ActionSelfManage    = "internal-finance-requests.self.manage"
	ActionTenantRead    = "internal-finance-requests.tenant.read"
	ActionReferenceRead = "internal-finance-requests.reference.read"
)

var AllActions = []string{ActionSelfRead, ActionSelfManage, ActionTenantRead, ActionReferenceRead, "internal-finance-requests.payment.confirm", "internal-finance-requests.payment.batch-confirm", "internal-finance-requests.sensitive-account.read", "internal-finance-requests.export", "internal-finance-requests.sensitive-account.export", "internal-finance-requests.category.manage"}
var ErrUnauthenticated = errors.New("authentication required")

type Principal struct {
	SubjectID, TenantID, DisplayName, Provider, CredentialID, ScopeKind string
	AllowedActions                                                      map[string]bool
	Resources                                                           map[string]map[string]bool
}

func (p Principal) Can(action string) bool { return p.AllowedActions[action] }
func (p Principal) CanResource(id string) bool {
	return p.ScopeKind == "tenant" || (p.ScopeKind == "resources" && p.Resources["finance_request"][id])
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
	for _, value := range AllActions {
		if action == value {
			return true
		}
	}
	return false
}
