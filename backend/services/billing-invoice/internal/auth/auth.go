package auth

import (
	"errors"
	"net/http"
	"strings"
)

const (
	ActionRead                   = "billing-invoice.read"
	ActionAmountRead             = "billing-invoice.amount.read"
	ActionRecord                 = "billing-invoice.record"
	ActionCorrect                = "billing-invoice.correct"
	ActionRegistrationInvalidate = "billing-invoice.registration.invalidate"
	ActionEvidenceManage         = "billing-invoice.evidence.manage"
	ActionHistoryRead            = "billing-invoice.history.read"
	ActionReferenceRead          = "billing-invoice.reference.read"
	ActionExport                 = "billing-invoice.export"
)

var UserActions = []string{
	ActionRead,
	ActionAmountRead,
	ActionRecord,
	ActionCorrect,
	ActionRegistrationInvalidate,
	ActionEvidenceManage,
	ActionHistoryRead,
	ActionExport,
}

var AllActions = append(append([]string{}, UserActions...), ActionReferenceRead)

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

func (p Principal) CanRegistration(id string, projectIDs []string) bool {
	if p.ScopeKind == "tenant" || p.CanResource("invoice_registration", id) {
		return true
	}
	for _, projectID := range projectIDs {
		if p.CanResource("project", projectID) {
			return true
		}
	}
	return false
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
