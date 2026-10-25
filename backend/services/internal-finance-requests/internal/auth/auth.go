package auth

import (
	"errors"
	"net/http"
	"strings"
)

const (
	ActionSelfRead             = "internal-finance-requests.self.read"
	ActionSelfManage           = "internal-finance-requests.self.manage"
	ActionTenantRead           = "internal-finance-requests.tenant.read"
	ActionReferenceRead        = "internal-finance-requests.reference.read"
	ActionPaymentConfirm       = "internal-finance-requests.payment.confirm"
	ActionPaymentBatchConfirm  = "internal-finance-requests.payment.batch-confirm"
	ActionSensitiveAccountRead = "internal-finance-requests.sensitive-account.read"
	ActionCategoryManage       = "internal-finance-requests.category.manage"
	ActionExport               = "internal-finance-requests.export"
	ActionSensitiveExport      = "internal-finance-requests.sensitive-account.export"
)

var AllActions = []string{ActionSelfRead, ActionSelfManage, ActionTenantRead, ActionReferenceRead, ActionPaymentConfirm, ActionPaymentBatchConfirm, ActionSensitiveAccountRead, ActionCategoryManage, ActionExport, ActionSensitiveExport}
var ErrUnauthenticated = errors.New("authentication required")

type Principal struct {
	SubjectID, TenantID, DisplayName, Provider, CredentialID, ScopeKind string
	PersonID, PersonDisplayName, PersonDigest                           string
	PersonRevision                                                      int64
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
