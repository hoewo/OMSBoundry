package auth

import (
	"errors"
	"net/http"
	"strings"
)

const (
	// ActionView is the bounded legacy capability retained while existing
	// consumers migrate to the narrower metadata and download actions.
	ActionView         = "evidence-service.view"
	ActionMetadataRead = "evidence-service.metadata.read"
	ActionDownload     = "evidence-service.download"
	ActionUpload       = "evidence-service.upload"
)

var AllActions = []string{ActionView, ActionMetadataRead, ActionDownload, ActionUpload}
var ErrUnauthenticated = errors.New("authentication required")

type Principal struct {
	SubjectID      string
	TenantID       string
	DisplayName    string
	Provider       string
	CredentialID   string
	ScopeKind      string
	AllowedActions map[string]bool
	Resources      map[string]map[string]bool
}

func (p Principal) Can(action string) bool { return p.AllowedActions[action] }

func (p Principal) CanFile(fileID string) bool {
	return p.ScopeKind == "tenant" || (p.ScopeKind == "resources" && p.Resources["file"][fileID])
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
