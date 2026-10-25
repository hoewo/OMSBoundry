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
	KTY    string `json:"kty"`
	CRV    string `json:"crv"`
	Use    string `json:"use"`
	ALG    string `json:"alg"`
	KeyID  string `json:"kid"`
	X      string `json:"x"`
	Status string `json:"status"`
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
	issuer, keyPath string
	now             func() time.Time
}

func NewIAMProvider(issuer, tenant, keyPath string) (*IAMProvider, error) {
	parsed, err := url.Parse(strings.TrimSpace(issuer))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || !uuidPattern.MatchString(tenant) || strings.TrimSpace(keyPath) == "" {
		return nil, fmt.Errorf("IAM provider configuration is invalid")
	}
	p := &IAMProvider{strings.TrimRight(issuer, "/"), keyPath, func() time.Time { return time.Now().UTC() }}
	if _, err = p.loadKeys(); err != nil {
		return nil, err
	}
	return p, nil
}
func (p *IAMProvider) ProviderName() string { return "iam" }
func (p *IAMProvider) Authenticate(r *http.Request) (Principal, error) {
	claims, err := p.verify(bearerToken(r))
	if err != nil {
		return Principal{}, ErrUnauthenticated
	}
	actions := map[string]bool{}
	for _, value := range claims.Actions {
		actions[value] = true
	}
	resources := map[string]map[string]bool{}
	for _, value := range claims.Scope.Resources {
		if resources[value.Type] == nil {
			resources[value.Type] = map[string]bool{}
		}
		resources[value.Type][value.ID] = true
	}
	return Principal{SubjectID: claims.Subject, TenantID: claims.Tenant, DisplayName: claims.Name, Provider: "iam", CredentialID: claims.ID, ScopeKind: claims.Scope.Kind, AllowedActions: actions, Resources: resources}, nil
}
func (p *IAMProvider) verify(token string) (iamClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return iamClaims{}, fmt.Errorf("invalid compact token")
	}
	var header iamHeader
	if err := decodePart(parts[0], &header); err != nil || header.Algorithm != "EdDSA" || header.Type != "JWT" || !uuidPattern.MatchString(header.KeyID) {
		return iamClaims{}, fmt.Errorf("invalid header")
	}
	keys, err := p.loadKeys()
	if err != nil {
		return iamClaims{}, err
	}
	key, ok := keys[header.KeyID]
	if !ok {
		return iamClaims{}, fmt.Errorf("unknown key")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !ed25519.Verify(key, []byte(parts[0]+"."+parts[1]), signature) {
		return iamClaims{}, fmt.Errorf("invalid signature")
	}
	var claims iamClaims
	if err = decodePart(parts[1], &claims); err != nil {
		return iamClaims{}, err
	}
	now := p.now().Unix()
	if claims.Version != "1" || claims.Issuer != p.issuer || !uuidPattern.MatchString(claims.Tenant) || claims.Audience != "internal-finance-requests" || !uuidPattern.MatchString(claims.Subject) || !uuidPattern.MatchString(claims.ID) || strings.TrimSpace(claims.Name) == "" {
		return iamClaims{}, fmt.Errorf("invalid identity")
	}
	if claims.IssuedAt > now+30 || claims.NotBefore > now+30 || claims.ExpiresAt <= now-30 || claims.ExpiresAt <= claims.IssuedAt || claims.ExpiresAt-claims.IssuedAt > 300 || claims.AuthTime <= 0 || claims.AuthTime > claims.IssuedAt {
		return iamClaims{}, fmt.Errorf("invalid time")
	}
	if claims.Scope.Kind != "tenant" && claims.Scope.Kind != "resources" {
		return iamClaims{}, fmt.Errorf("invalid scope")
	}
	if claims.Scope.Kind == "tenant" && len(claims.Scope.Resources) != 0 {
		return iamClaims{}, fmt.Errorf("tenant scope cannot contain resources")
	}
	if claims.Scope.Kind == "resources" && len(claims.Scope.Resources) == 0 {
		return iamClaims{}, fmt.Errorf("resource scope is empty")
	}
	for _, value := range claims.Scope.Resources {
		if (value.Type != "finance_request" && value.Type != "finance_category") || !uuidPattern.MatchString(value.ID) {
			return iamClaims{}, fmt.Errorf("invalid resource scope")
		}
	}
	if len(claims.Actions) == 0 {
		return iamClaims{}, fmt.Errorf("missing capabilities")
	}
	seen := map[string]bool{}
	for _, value := range claims.Actions {
		if !validAction(value) || seen[value] {
			return iamClaims{}, fmt.Errorf("invalid capability")
		}
		seen[value] = true
	}
	return claims, nil
}
func (p *IAMProvider) loadKeys() (map[string]ed25519.PublicKey, error) {
	data, err := os.ReadFile(p.keyPath)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var set iamKeySet
	if err = decoder.Decode(&set); err != nil {
		return nil, err
	}
	var trailing any
	if err = decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("public key file contains trailing JSON")
	}
	keys := map[string]ed25519.PublicKey{}
	for _, value := range set.Keys {
		if value.KTY != "OKP" || value.CRV != "Ed25519" || value.Use != "sig" || value.ALG != "EdDSA" || (value.Status != "active" && value.Status != "retiring") || !uuidPattern.MatchString(value.KeyID) {
			return nil, fmt.Errorf("invalid public key metadata")
		}
		raw, decodeErr := base64.RawURLEncoding.DecodeString(value.X)
		if decodeErr != nil || len(raw) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("invalid public key")
		}
		if _, exists := keys[value.KeyID]; exists {
			return nil, fmt.Errorf("duplicate public key")
		}
		keys[value.KeyID] = ed25519.PublicKey(raw)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("public key set is empty")
	}
	return keys, nil
}
func decodePart(raw string, destination any) error {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err = decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("trailing JSON")
	}
	return nil
}
