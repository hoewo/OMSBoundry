package auth

import (
	"errors"
	"net/http"
	"strings"
)

const (
	ActionTenantRead       = "workforce-attendance.tenant.read"
	ActionPolicyManage     = "workforce-attendance.policy.manage"
	ActionRecordManage     = "workforce-attendance.record.manage"
	ActionSummaryCalculate = "workforce-attendance.summary.calculate"
	ActionSummaryRead      = "workforce-attendance.summary.read"
)

var AllActions = []string{ActionTenantRead, ActionPolicyManage, ActionRecordManage, ActionSummaryCalculate, ActionSummaryRead}
var ErrUnauthenticated = errors.New("authentication required")

type Principal struct {
	SubjectID, TenantID, DisplayName, Provider, CredentialID, ScopeKind string
	AllowedActions                                                      map[string]bool
	Resources                                                           map[string]map[string]bool
}

func (p Principal) Can(action string) bool { return p.AllowedActions[action] }
func (p Principal) CanResource(resourceType, id string) bool {
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
	for _, candidate := range AllActions {
		if candidate == action {
			return true
		}
	}
	return false
}
func validResourceType(value string) bool {
	switch value {
	case "person", "attendance_record", "leave_record", "monthly_summary":
		return true
	default:
		return false
	}
}
