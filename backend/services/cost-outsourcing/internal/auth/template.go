package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"net/http"
)

type TemplateProvider struct {
	token     string
	principal Principal
}

func NewTemplateProvider(tenantID, subjectID, displayName, token string) (*TemplateProvider, error) {
	if !uuidPattern.MatchString(tenantID) || !uuidPattern.MatchString(subjectID) || displayName == "" || len(token) < 24 {
		return nil, fmt.Errorf("template provider configuration is invalid")
	}
	actions := map[string]bool{}
	for _, action := range UserActions {
		actions[action] = true
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return nil, err
	}
	id[6] = id[6]&0x0f | 0x40
	id[8] = id[8]&0x3f | 0x80
	credential := fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:])
	return &TemplateProvider{token: token, principal: Principal{SubjectID: subjectID, TenantID: tenantID, DisplayName: displayName, Provider: "template", CredentialID: credential, ScopeKind: "tenant", AllowedActions: actions, Resources: map[string]map[string]bool{}}}, nil
}

func (p *TemplateProvider) ProviderName() string { return "template" }

func (p *TemplateProvider) Authenticate(r *http.Request) (Principal, error) {
	token := bearerToken(r)
	if len(token) != len(p.token) || subtle.ConstantTimeCompare([]byte(token), []byte(p.token)) != 1 {
		return Principal{}, ErrUnauthenticated
	}
	return p.principal, nil
}
