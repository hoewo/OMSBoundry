package auth

import (
	"errors"
	"net/http"
	"strings"
)

const (
	ActionTenantRead               = "payroll.tenant.read"
	ActionRunManage                = "payroll.run.manage"
	ActionReferenceRead            = "payroll.reference.read"
	ActionCompensationManage       = "payroll.compensation-profile.manage"
	ActionPaymentProfileManage     = "payroll.payment-profile.manage"
	ActionSensitiveAccountRead     = "payroll.sensitive-account.read"
	ActionContributionSchemeManage = "payroll.contribution-scheme.manage"
	ActionPaymentConfirm           = "payroll.payment.confirm"
	ActionPaymentBatchConfirm      = "payroll.payment.batch-confirm"
	ActionSelfRead                 = "payroll.self.read"
)

var AllActions = []string{ActionSelfRead, ActionTenantRead, ActionCompensationManage, ActionPaymentProfileManage, ActionSensitiveAccountRead, "payroll.sensitive-account.export", ActionContributionSchemeManage, ActionRunManage, ActionPaymentConfirm, ActionPaymentBatchConfirm, "payroll.export", ActionReferenceRead}
var ErrUnauthenticated = errors.New("authentication required")

type Principal struct {
	SubjectID, TenantID, DisplayName, Provider, CredentialID, ScopeKind string
	AllowedActions                                                      map[string]bool
	Resources                                                           map[string]map[string]bool
}

func (p Principal) Can(action string) bool { return p.AllowedActions[action] }
func (p Principal) CanResource(id string) bool {
	return p.ScopeKind == "tenant" || (p.ScopeKind == "resources" && p.Resources["payroll_run"][id])
}
func (p Principal) CanResourceType(resourceType, id string) bool {
	return p.ScopeKind == "tenant" || (p.ScopeKind == "resources" && p.Resources[resourceType][id])
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
