package auth

import (
	"crypto/subtle"
	"fmt"
	"net/http"
)

type TemplateProvider struct {
	tenantID, subjectID, displayName, token string
	principal                               Principal
}

func NewTemplateProvider(tenantID, subjectID, displayName, token string) (*TemplateProvider, error) {
	if tenantID == "" || subjectID == "" || displayName == "" || len(token) < 24 {
		return nil, fmt.Errorf("template provider configuration is invalid")
	}
	actions := make(map[string]bool, len(AllActions))
	for _, action := range AllActions {
		actions[action] = true
	}
	return &TemplateProvider{
		tenantID: tenantID, subjectID: subjectID, displayName: displayName, token: token,
		principal: Principal{SubjectID: subjectID, TenantID: tenantID, DisplayName: displayName, Provider: "template", CredentialID: "template-credential", AllowedActions: actions, ScopeKind: "tenant"},
	}, nil
}

func (p *TemplateProvider) ProviderName() string { return "template" }

func (p *TemplateProvider) Authenticate(r *http.Request) (Principal, error) {
	token := bearerToken(r)
	if token == "" {
		if cookie, err := r.Cookie(TemplateCookieName); err == nil {
			token = cookie.Value
		}
	}
	if len(token) != len(p.token) || subtle.ConstantTimeCompare([]byte(token), []byte(p.token)) != 1 {
		return Principal{}, ErrUnauthenticated
	}
	return p.principal, nil
}

func (p *TemplateProvider) ValidToken(token string) bool {
	return len(token) == len(p.token) && subtle.ConstantTimeCompare([]byte(token), []byte(p.token)) == 1
}
