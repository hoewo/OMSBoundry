package auth

import (
	"errors"
	"net/http"
	"strings"
)

const (
	ActionCreate         = "approval-audit-result.create"
	ActionRead           = "approval-audit-result.read"
	ActionAmountRead     = "approval-audit-result.amount.read"
	ActionCorrect        = "approval-audit-result.correct"
	ActionWithdraw       = "approval-audit-result.withdraw"
	ActionEvidenceManage = "approval-audit-result.evidence.manage"
	ActionHistoryRead    = "approval-audit-result.history.read"
)

var UserActions = []string{ActionCreate, ActionRead, ActionAmountRead, ActionCorrect, ActionWithdraw, ActionEvidenceManage, ActionHistoryRead}
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

func (p Principal) CanResult(id, projectID string) bool {
	return p.ScopeKind == "tenant" || p.CanResource("approval_audit_result", id) || p.CanResource("project", projectID)
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
