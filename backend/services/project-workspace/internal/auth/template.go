package auth

import (
	"crypto/subtle"
	"fmt"
	"net/http"
)

type TemplateProvider struct {
	token     string
	principal Principal
}

func NewTemplateProvider(tenantID, subjectID, displayName, token string) (*TemplateProvider, error) {
	if tenantID == "" || subjectID == "" || displayName == "" || len(token) < 24 {
		return nil, fmt.Errorf("template provider configuration is invalid")
	}
	actions := make(map[string]bool, len(AllActions))
	for _, action := range AllActions {
		actions[action] = true
	}
	return &TemplateProvider{token: token, principal: Principal{
		SubjectID: subjectID, TenantID: tenantID, DisplayName: displayName,
		Provider: "template", CredentialID: "template-project-credential",
		AllowedActions: actions, ScopeKind: "tenant", ResourceIDs: map[string]bool{},
	}}, nil
}

func (p *TemplateProvider) ProviderName() string { return "template" }
func (p *TemplateProvider) Authenticate(r *http.Request) (Principal, error) {
	token := bearerToken(r)
	if len(token) != len(p.token) || subtle.ConstantTimeCompare([]byte(token), []byte(p.token)) != 1 {
		return Principal{}, ErrUnauthenticated
	}
	return p.principal, nil
}
