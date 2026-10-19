package auth

import (
	"errors"
	"net/http"
	"strings"
)

const (
	ActionCreate                = "business-engagement.create"
	ActionRead                  = "business-engagement.read"
	ActionApplicabilityManage   = "business-engagement.applicability.manage"
	ActionMethodSelect          = "business-engagement.method.select"
	ActionEvidenceManage        = "business-engagement.evidence.manage"
	ActionMaterialReferenceRead = "business-engagement.material-reference.read"
	ActionCancel                = "business-engagement.cancel"
	ActionHistoryRead           = "business-engagement.history.read"
)

var AllActions = []string{
	ActionCreate,
	ActionRead,
	ActionApplicabilityManage,
	ActionMethodSelect,
	ActionEvidenceManage,
	ActionMaterialReferenceRead,
	ActionCancel,
	ActionHistoryRead,
}

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

func (p Principal) CanEngagement(engagementID, projectID string) bool {
	return p.ScopeKind == "tenant" || p.CanResource("business-engagement", engagementID) || p.CanResource("project", projectID)
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
