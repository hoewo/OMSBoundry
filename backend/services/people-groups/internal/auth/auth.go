package auth

import (
	"errors"
	"net/http"
	"strings"
)

const TemplateCookieName = "oms_people_groups_template"

const (
	ActionView                = "people-groups.view"
	ActionPersonManage        = "people-groups.person.manage"
	ActionConfigurationManage = "people-groups.configuration.manage"
	ActionGroupManage         = "people-groups.group.manage"
	ActionMembershipManage    = "people-groups.membership.manage"
	ActionCapabilityManage    = "people-groups.capability.manage"
	ActionSnapshotPublish     = "people-groups.snapshot.publish"
	ActionPolicyFactRead      = "people-groups.policy-fact.read"
	ActionPersonReferenceRead = "people-groups.person.reference.read"
)

var AllActions = []string{
	ActionView,
	ActionPersonManage,
	ActionConfigurationManage,
	ActionGroupManage,
	ActionMembershipManage,
	ActionCapabilityManage,
	ActionSnapshotPublish,
	ActionPolicyFactRead,
	ActionPersonReferenceRead,
}

var ErrUnauthenticated = errors.New("authentication required")

type Principal struct {
	SubjectID      string
	TenantID       string
	DisplayName    string
	Provider       string
	CredentialID   string
	AllowedActions map[string]bool
	ScopeKind      string
	ResourceIDs    map[string]bool
}

func (p Principal) Can(action string) bool { return p.AllowedActions[action] }
func (p Principal) CanResource(resourceType, resourceID string) bool {
	return p.ScopeKind == "tenant" || (p.ScopeKind == "resources" && p.ResourceIDs[resourceType+"\x00"+resourceID])
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
		if action == candidate {
			return true
		}
	}
	return false
}
