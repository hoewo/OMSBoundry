package auth

import (
	"errors"
	"net/http"
	"strings"
)

const (
	ActionRead          = "party-directory.read"
	ActionCreate        = "party-directory.create"
	ActionUpdate        = "party-directory.update"
	ActionStatusManage  = "party-directory.status.manage"
	ActionHistoryRead   = "party-directory.history.read"
	ActionReferenceRead = "party-directory.reference.read"
)

var AllActions = []string{ActionRead, ActionCreate, ActionUpdate, ActionStatusManage, ActionHistoryRead, ActionReferenceRead}
var ErrUnauthenticated = errors.New("authentication required")

type Principal struct {
	SubjectID, TenantID, DisplayName, Provider, CredentialID, ScopeKind string
	AllowedActions                                                      map[string]bool
	Resources                                                           map[string]map[string]bool
}

func (p Principal) Can(action string) bool { return p.AllowedActions[action] }
func (p Principal) CanParty(id string) bool {
	return p.ScopeKind == "tenant" || (p.ScopeKind == "resources" && p.Resources["party"][id])
}
func (p Principal) CanEnumerate() bool { return p.ScopeKind == "tenant" }

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
