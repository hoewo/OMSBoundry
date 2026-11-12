package auth

import (
	"crypto/ed25519"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	ActionTenantRead = "invoice-recognition.tenant.read"
	ActionStart      = "invoice-recognition.start"
	ActionConfirm    = "invoice-recognition.confirm"
	ActionDiscard    = "invoice-recognition.discard"
)

var (
	AllActions         = []string{ActionTenantRead, ActionStart, ActionConfirm, ActionDiscard}
	ErrUnauthenticated = errors.New("authentication required")
	uuidPattern        = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)

type Principal struct {
	SubjectID, TenantID, DisplayName, Provider, CredentialID, ScopeKind string
	AllowedActions                                                      map[string]bool
	Resources                                                           map[string]map[string]bool
}

func (p Principal) Can(action string) bool { return p.AllowedActions[action] }
func (p Principal) CanResource(kind, id string) bool {
	return p.ScopeKind == "tenant" || (p.ScopeKind == "resources" && p.Resources[kind][id])
}

type Authenticator interface {
	Authenticate(*http.Request) (Principal, error)
	ProviderName() string
}

func bearer(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(value) > 7 && strings.EqualFold(value[:7], "Bearer ") {
		return strings.TrimSpace(value[7:])
	}
	return ""
}

type TemplateProvider struct {
	token     string
	principal Principal
}

func NewTemplateProvider(tenant, subject, name, token string) (*TemplateProvider, error) {
	if !uuidPattern.MatchString(tenant) || !uuidPattern.MatchString(subject) || strings.TrimSpace(name) == "" || len(token) < 24 {
		return nil, fmt.Errorf("template identity is invalid")
	}
	actions := map[string]bool{}
	for _, action := range AllActions {
		actions[action] = true
	}
	return &TemplateProvider{token: token, principal: Principal{SubjectID: subject, TenantID: tenant, DisplayName: name, Provider: "template", CredentialID: "template-invoice-recognition", ScopeKind: "tenant", AllowedActions: actions, Resources: map[string]map[string]bool{}}}, nil
}
func (p *TemplateProvider) ProviderName() string { return "template" }
func (p *TemplateProvider) Authenticate(r *http.Request) (Principal, error) {
	token := bearer(r)
	if len(token) != len(p.token) || subtle.ConstantTimeCompare([]byte(token), []byte(p.token)) != 1 {
		return Principal{}, ErrUnauthenticated
	}
	return p.principal, nil
}

type header struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	KeyID     string `json:"kid"`
}
type resource struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}
type scope struct {
	Kind      string     `json:"kind"`
	Resources []resource `json:"resources"`
}
type claims struct {
	Version   string   `json:"ver"`
	Issuer    string   `json:"iss"`
	Subject   string   `json:"sub"`
	Tenant    string   `json:"tid"`
	Audience  string   `json:"aud"`
	Name      string   `json:"name"`
	Actions   []string `json:"actions"`
	Scope     scope    `json:"scope"`
	IssuedAt  int64    `json:"iat"`
	NotBefore int64    `json:"nbf"`
	ExpiresAt int64    `json:"exp"`
	AuthTime  int64    `json:"auth_time"`
	ID        string   `json:"jti"`
}
type keySet struct {
	Keys []struct {
		KTY    string `json:"kty"`
		CRV    string `json:"crv"`
		Use    string `json:"use"`
		ALG    string `json:"alg"`
		ID     string `json:"kid"`
		X      string `json:"x"`
		Status string `json:"status"`
	} `json:"keys"`
}
type IAMProvider struct {
	issuer, keyPath string
	now             func() time.Time
}

func NewIAMProvider(issuer, tenant, keyPath string) (*IAMProvider, error) {
	parsed, err := url.Parse(issuer)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || !uuidPattern.MatchString(tenant) || strings.TrimSpace(keyPath) == "" {
		return nil, fmt.Errorf("IAM configuration is invalid")
	}
	provider := &IAMProvider{issuer: strings.TrimRight(issuer, "/"), keyPath: keyPath, now: func() time.Time { return time.Now().UTC() }}
	if _, err = provider.keys(); err != nil {
		return nil, err
	}
	return provider, nil
}
func (p *IAMProvider) ProviderName() string { return "iam" }
func (p *IAMProvider) Authenticate(r *http.Request) (Principal, error) {
	value, err := p.verify(bearer(r))
	if err != nil {
		return Principal{}, ErrUnauthenticated
	}
	actions := map[string]bool{}
	for _, action := range value.Actions {
		actions[action] = true
	}
	resources := map[string]map[string]bool{}
	for _, item := range value.Scope.Resources {
		if resources[item.Type] == nil {
			resources[item.Type] = map[string]bool{}
		}
		resources[item.Type][item.ID] = true
	}
	return Principal{SubjectID: value.Subject, TenantID: value.Tenant, DisplayName: value.Name, Provider: "iam", CredentialID: value.ID, ScopeKind: value.Scope.Kind, AllowedActions: actions, Resources: resources}, nil
}
func (p *IAMProvider) verify(token string) (claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return claims{}, fmt.Errorf("invalid token")
	}
	var h header
	if decodePart(parts[0], &h) != nil || h.Algorithm != "EdDSA" || h.Type != "JWT" || !uuidPattern.MatchString(h.KeyID) {
		return claims{}, fmt.Errorf("invalid header")
	}
	keys, err := p.keys()
	if err != nil {
		return claims{}, err
	}
	key, ok := keys[h.KeyID]
	sig, sigErr := base64.RawURLEncoding.DecodeString(parts[2])
	if !ok || sigErr != nil || !ed25519.Verify(key, []byte(parts[0]+"."+parts[1]), sig) {
		return claims{}, fmt.Errorf("invalid signature")
	}
	var value claims
	if err = decodePart(parts[1], &value); err != nil {
		return value, err
	}
	now := p.now().Unix()
	if value.Version != "1" || value.Issuer != p.issuer || value.Audience != "invoice-recognition" || !uuidPattern.MatchString(value.Subject) || !uuidPattern.MatchString(value.Tenant) || !uuidPattern.MatchString(value.ID) || strings.TrimSpace(value.Name) == "" || value.IssuedAt > now+30 || value.NotBefore > now+30 || value.ExpiresAt <= now-30 || value.ExpiresAt <= value.IssuedAt || value.ExpiresAt-value.IssuedAt > 300 || value.AuthTime <= 0 || value.AuthTime > value.IssuedAt {
		return value, fmt.Errorf("invalid claims")
	}
	if value.Scope.Kind != "tenant" && value.Scope.Kind != "resources" {
		return value, fmt.Errorf("invalid scope")
	}
	if (value.Scope.Kind == "tenant" && len(value.Scope.Resources) != 0) || (value.Scope.Kind == "resources" && len(value.Scope.Resources) == 0) {
		return value, fmt.Errorf("invalid resources")
	}
	for _, item := range value.Scope.Resources {
		if (item.Type != "proposal" && item.Type != "file") || !uuidPattern.MatchString(item.ID) {
			return value, fmt.Errorf("invalid resource")
		}
	}
	seen := map[string]bool{}
	for _, action := range value.Actions {
		valid := false
		for _, candidate := range AllActions {
			if candidate == action {
				valid = true
			}
		}
		if !valid || seen[action] {
			return value, fmt.Errorf("invalid action")
		}
		seen[action] = true
	}
	if len(seen) == 0 {
		return value, fmt.Errorf("missing action")
	}
	return value, nil
}
func (p *IAMProvider) keys() (map[string]ed25519.PublicKey, error) {
	data, err := os.ReadFile(p.keyPath)
	if err != nil {
		return nil, err
	}
	var set keySet
	if strict(data, &set) != nil {
		return nil, fmt.Errorf("invalid key set")
	}
	result := map[string]ed25519.PublicKey{}
	for _, item := range set.Keys {
		raw, e := base64.RawURLEncoding.DecodeString(item.X)
		if item.KTY != "OKP" || item.CRV != "Ed25519" || item.Use != "sig" || item.ALG != "EdDSA" || (item.Status != "active" && item.Status != "retiring") || !uuidPattern.MatchString(item.ID) || e != nil || len(raw) != ed25519.PublicKeySize || result[item.ID] != nil {
			return nil, fmt.Errorf("invalid public key")
		}
		result[item.ID] = ed25519.PublicKey(raw)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("empty key set")
	}
	return result, nil
}
func decodePart(raw string, destination any) error {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return err
	}
	return strict(data, destination)
}
func strict(data []byte, destination any) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("trailing JSON")
	}
	return nil
}
