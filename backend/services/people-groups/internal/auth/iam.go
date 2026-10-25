package auth

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
type iamScope struct {
	Kind      string `json:"kind"`
	Resources []struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	} `json:"resources"`
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
	if strings.TrimSpace(issuer) == "" || !uuidPattern.MatchString(tenant) || strings.TrimSpace(keyPath) == "" {
		return nil, fmt.Errorf("IAM provider configuration is invalid")
	}
	provider := &IAMProvider{issuer: strings.TrimRight(issuer, "/"), keyPath: keyPath, now: func() time.Time { return time.Now().UTC() }}
	if _, err := provider.loadKeys(); err != nil {
		return nil, err
	}
	return provider, nil
}

func (p *IAMProvider) ProviderName() string { return "iam" }

func (p *IAMProvider) Authenticate(r *http.Request) (Principal, error) {
	claims, err := p.verify(bearerToken(r))
	if err != nil {
		return Principal{}, ErrUnauthenticated
	}
	actions := make(map[string]bool, len(claims.Actions))
	for _, action := range claims.Actions {
		actions[action] = true
	}
	resources := make(map[string]bool, len(claims.Scope.Resources))
	for _, resource := range claims.Scope.Resources {
		resources[resource.Type+"\x00"+resource.ID] = true
	}
	return Principal{SubjectID: claims.Subject, TenantID: claims.Tenant, DisplayName: claims.Name, Provider: "iam", CredentialID: claims.ID, AllowedActions: actions, ScopeKind: claims.Scope.Kind, ResourceIDs: resources}, nil
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
	if err := decodePart(parts[1], &claims); err != nil {
		return iamClaims{}, err
	}
	now := p.now().Unix()
	if claims.Version != "1" || claims.Issuer != p.issuer || !uuidPattern.MatchString(claims.Tenant) || claims.Audience != "people-groups" || !uuidPattern.MatchString(claims.Subject) || !uuidPattern.MatchString(claims.ID) || strings.TrimSpace(claims.Name) == "" {
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
	for _, resource := range claims.Scope.Resources {
		if (resource.Type != "person" && resource.Type != "iam_subject") || !uuidPattern.MatchString(resource.ID) {
			return iamClaims{}, fmt.Errorf("invalid people resource scope")
		}
	}
	if len(claims.Actions) == 0 {
		return iamClaims{}, fmt.Errorf("missing capabilities")
	}
	for _, action := range claims.Actions {
		if !validAction(action) {
			return iamClaims{}, fmt.Errorf("unknown capability")
		}
	}
	if !contains(claims.Actions, ActionView) && !contains(claims.Actions, ActionPolicyFactRead) && !contains(claims.Actions, ActionPersonReferenceRead) {
		return iamClaims{}, fmt.Errorf("missing entry capability")
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
	if err := decoder.Decode(&set); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("public key file contains trailing JSON")
	}
	keys := make(map[string]ed25519.PublicKey, len(set.Keys))
	for _, item := range set.Keys {
		if item.KTY != "OKP" || item.CRV != "Ed25519" || item.Use != "sig" || item.ALG != "EdDSA" || (item.Status != "active" && item.Status != "retiring") || !uuidPattern.MatchString(item.KeyID) {
			return nil, fmt.Errorf("invalid public key metadata")
		}
		raw, err := base64.RawURLEncoding.DecodeString(item.X)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("invalid public key")
		}
		if _, duplicate := keys[item.KeyID]; duplicate {
			return nil, fmt.Errorf("duplicate public key")
		}
		keys[item.KeyID] = ed25519.PublicKey(raw)
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
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("trailing JSON")
	}
	return nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
