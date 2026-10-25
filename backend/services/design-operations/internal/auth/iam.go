package auth

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type iamHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	KeyID     string `json:"kid"`
}
type iamJWK struct {
	KTY, CRV, Use, ALG string
	KeyID              string `json:"kid"`
	X                  string `json:"x"`
	Status             string `json:"status"`
}
type iamKeySet struct {
	Keys []iamJWK `json:"keys"`
}
type iamResource struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}
type iamScope struct {
	Kind      string        `json:"kind"`
	Resources []iamResource `json:"resources"`
}
type iamClaims struct {
	Version   string   `json:"ver"`
	Issuer    string   `json:"iss"`
	Subject   string   `json:"sub"`
	Tenant    string   `json:"tid"`
	Audience  string   `json:"aud"`
	Name      string   `json:"name"`
	Actions   []string `json:"actions"`
	Scope     iamScope `json:"scope"`
	IssuedAt  int64    `json:"iat"`
	NotBefore int64    `json:"nbf"`
	ExpiresAt int64    `json:"exp"`
	AuthTime  int64    `json:"auth_time"`
	ID        string   `json:"jti"`
}
type IAMProvider struct {
	issuer, keyPath, audience string
	actions, resourceTypes    []string
	now                       func() time.Time
}

func NewIAMProvider(issuer, tenant, keyPath string) (*IAMProvider, error) {
	return newIAMProvider(issuer, tenant, keyPath, DesignAudience, AllActions, []string{"project", "design-work-item"})
}

func NewProductionIAMProvider(issuer, tenant, keyPath string) (*IAMProvider, error) {
	return newIAMProvider(issuer, tenant, keyPath, ProductionAudience, ProductionActions, []string{"project", "production-work-item"})
}

func newIAMProvider(issuer, tenant, keyPath, audience string, actions, resourceTypes []string) (*IAMProvider, error) {
	parsedIssuer, parseErr := url.Parse(strings.TrimSpace(issuer))
	if parseErr != nil || parsedIssuer.Scheme != "https" || parsedIssuer.Host == "" || !uuidPattern.MatchString(tenant) || strings.TrimSpace(keyPath) == "" || strings.TrimSpace(audience) == "" || len(actions) == 0 || len(resourceTypes) == 0 {
		return nil, fmt.Errorf("IAM provider configuration is invalid")
	}
	p := &IAMProvider{issuer: strings.TrimRight(issuer, "/"), keyPath: keyPath, audience: audience, actions: append([]string(nil), actions...), resourceTypes: append([]string(nil), resourceTypes...), now: func() time.Time { return time.Now().UTC() }}
	if _, err := p.loadKeys(); err != nil {
		return nil, err
	}
	return p, nil
}
func (p *IAMProvider) ProviderName() string { return "iam" }
func (p *IAMProvider) Authenticate(r *http.Request) (Principal, error) {
	c, err := p.verify(bearerToken(r))
	if err != nil {
		return Principal{}, ErrUnauthenticated
	}
	actions := map[string]bool{}
	for _, a := range c.Actions {
		actions[a] = true
	}
	resources := map[string]map[string]bool{}
	for _, v := range c.Scope.Resources {
		if resources[v.Type] == nil {
			resources[v.Type] = map[string]bool{}
		}
		resources[v.Type][v.ID] = true
	}
	return Principal{SubjectID: c.Subject, TenantID: c.Tenant, DisplayName: c.Name, Provider: "iam", CredentialID: c.ID, ScopeKind: c.Scope.Kind, AllowedActions: actions, Resources: resources}, nil
}
func (p *IAMProvider) verify(token string) (iamClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return iamClaims{}, fmt.Errorf("invalid compact token")
	}
	var h iamHeader
	if err := decodePart(parts[0], &h); err != nil || h.Algorithm != "EdDSA" || h.Type != "JWT" || !uuidPattern.MatchString(h.KeyID) {
		return iamClaims{}, fmt.Errorf("invalid header")
	}
	keys, err := p.loadKeys()
	if err != nil {
		return iamClaims{}, err
	}
	key, ok := keys[h.KeyID]
	if !ok {
		return iamClaims{}, fmt.Errorf("unknown key")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !ed25519.Verify(key, []byte(parts[0]+"."+parts[1]), sig) {
		return iamClaims{}, fmt.Errorf("invalid signature")
	}
	var c iamClaims
	if err := decodePart(parts[1], &c); err != nil {
		return iamClaims{}, err
	}
	now := p.now().Unix()
	if c.Version != "1" || c.Issuer != p.issuer || !uuidPattern.MatchString(c.Tenant) || c.Audience != p.audience || !uuidPattern.MatchString(c.Subject) || !uuidPattern.MatchString(c.ID) || strings.TrimSpace(c.Name) == "" {
		return iamClaims{}, fmt.Errorf("invalid identity")
	}
	if c.IssuedAt > now+30 || c.NotBefore > now+30 || c.ExpiresAt <= now-30 || c.ExpiresAt <= c.IssuedAt || c.ExpiresAt-c.IssuedAt > 300 || c.AuthTime <= 0 || c.AuthTime > c.IssuedAt {
		return iamClaims{}, fmt.Errorf("invalid time")
	}
	if c.Scope.Kind != "tenant" && c.Scope.Kind != "resources" {
		return iamClaims{}, fmt.Errorf("invalid scope")
	}
	if c.Scope.Kind == "tenant" && len(c.Scope.Resources) != 0 {
		return iamClaims{}, fmt.Errorf("tenant scope cannot contain resources")
	}
	if c.Scope.Kind == "resources" && len(c.Scope.Resources) == 0 {
		return iamClaims{}, fmt.Errorf("resource scope is empty")
	}
	for _, v := range c.Scope.Resources {
		if !includes(p.resourceTypes, v.Type) || !uuidPattern.MatchString(v.ID) {
			return iamClaims{}, fmt.Errorf("invalid resource scope")
		}
	}
	if len(c.Actions) == 0 {
		return iamClaims{}, fmt.Errorf("missing capabilities")
	}
	seen := map[string]bool{}
	for _, a := range c.Actions {
		if !includes(p.actions, a) || seen[a] {
			return iamClaims{}, fmt.Errorf("invalid capability")
		}
		seen[a] = true
	}
	return c, nil
}
func (p *IAMProvider) loadKeys() (map[string]ed25519.PublicKey, error) {
	data, err := os.ReadFile(p.keyPath)
	if err != nil {
		return nil, err
	}
	d := json.NewDecoder(strings.NewReader(string(data)))
	d.DisallowUnknownFields()
	var set iamKeySet
	if err := d.Decode(&set); err != nil {
		return nil, err
	}
	var trailing any
	if err := d.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("public key file contains trailing JSON")
	}
	keys := map[string]ed25519.PublicKey{}
	for _, v := range set.Keys {
		if v.KTY != "OKP" || v.CRV != "Ed25519" || v.Use != "sig" || v.ALG != "EdDSA" || (v.Status != "active" && v.Status != "retiring") || !uuidPattern.MatchString(v.KeyID) {
			return nil, fmt.Errorf("invalid public key metadata")
		}
		raw, err := base64.RawURLEncoding.DecodeString(v.X)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("invalid public key")
		}
		if _, ok := keys[v.KeyID]; ok {
			return nil, fmt.Errorf("duplicate public key")
		}
		keys[v.KeyID] = ed25519.PublicKey(raw)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("public key set is empty")
	}
	return keys, nil
}
func decodePart(raw string, dest any) error {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return err
	}
	d := json.NewDecoder(strings.NewReader(string(data)))
	d.DisallowUnknownFields()
	if err := d.Decode(dest); err != nil {
		return err
	}
	var trailing any
	if err := d.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("trailing JSON")
	}
	return nil
}
