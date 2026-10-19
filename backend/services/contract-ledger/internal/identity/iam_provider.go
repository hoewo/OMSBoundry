package identity

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"
)

var iamUUIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

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
	issuer, tenant, keyPath string
	now                     func() time.Time
}
type ExternalIdentityError struct{}

func (e *ExternalIdentityError) Error() string { return "账号生命周期由中心 IAM 管理" }

func NewIAMProvider(issuer, tenant, keyPath string) (*IAMProvider, error) {
	if strings.TrimSpace(issuer) == "" || !iamUUIDPattern.MatchString(tenant) || strings.TrimSpace(keyPath) == "" {
		return nil, fmt.Errorf("IAM provider configuration is invalid")
	}
	provider := &IAMProvider{issuer: strings.TrimRight(issuer, "/"), tenant: tenant, keyPath: keyPath, now: func() time.Time { return time.Now().UTC() }}
	if _, err := provider.loadKeys(); err != nil {
		return nil, err
	}
	return provider, nil
}
func (p *IAMProvider) ResolveSession(_ context.Context, token string) (Principal, bool, error) {
	claims, err := p.verify(token)
	if err != nil {
		return Principal{}, false, nil
	}
	actions := map[string]bool{}
	for _, action := range claims.Actions {
		actions[action] = true
	}
	resources := map[string]map[string]bool{}
	for _, resource := range claims.Scope.Resources {
		if resources[resource.Type] == nil {
			resources[resource.Type] = map[string]bool{}
		}
		resources[resource.Type][resource.ID] = true
	}
	return Principal{UserID: claims.Subject, Username: claims.Subject, DisplayName: claims.Name, Role: Role("iam"), Provider: "iam", TenantID: claims.Tenant, CredentialID: claims.ID, AllowedActions: actions, ScopeKind: claims.Scope.Kind, Resources: resources}, true, nil
}
func (p *IAMProvider) verify(token string) (iamClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return iamClaims{}, fmt.Errorf("invalid compact token")
	}
	var header iamHeader
	if err := decodeIAMPart(parts[0], &header); err != nil {
		return iamClaims{}, err
	}
	if header.Algorithm != "EdDSA" || header.Type != "JWT" || !iamUUIDPattern.MatchString(header.KeyID) {
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
	if err := decodeIAMPart(parts[1], &claims); err != nil {
		return iamClaims{}, err
	}
	now := p.now().Unix()
	if claims.Version != "1" || claims.Issuer != p.issuer || claims.Tenant != p.tenant || claims.Audience != "contract-ledger" || !iamUUIDPattern.MatchString(claims.Subject) || !iamUUIDPattern.MatchString(claims.ID) || strings.TrimSpace(claims.Name) == "" {
		return iamClaims{}, fmt.Errorf("invalid identity")
	}
	if claims.IssuedAt > now+30 || claims.NotBefore > now+30 || claims.ExpiresAt <= now-30 || claims.ExpiresAt <= claims.IssuedAt || claims.ExpiresAt-claims.IssuedAt > 300 || claims.AuthTime <= 0 || claims.AuthTime > claims.IssuedAt {
		return iamClaims{}, fmt.Errorf("invalid time")
	}
	if claims.Scope.Kind != "tenant" && claims.Scope.Kind != "resources" {
		return iamClaims{}, fmt.Errorf("invalid scope")
	}
	if claims.Scope.Kind == "tenant" {
		if len(claims.Scope.Resources) != 0 || !slices.Contains(claims.Actions, "contract-ledger.view") {
			return iamClaims{}, fmt.Errorf("invalid tenant scope")
		}
	} else {
		if len(claims.Scope.Resources) != 1 || len(claims.Actions) == 0 {
			return iamClaims{}, fmt.Errorf("invalid resource scope")
		}
		resource := claims.Scope.Resources[0]
		if (resource.Type != "contract" && resource.Type != "settlement") || !iamUUIDPattern.MatchString(resource.ID) {
			return iamClaims{}, fmt.Errorf("invalid reference resource")
		}
		if len(claims.Actions) == 1 && claims.Actions[0] == "contract-ledger.reference.read" {
			// Retain the existing exact Contract / Settlement workload contract.
		} else {
			if resource.Type != "contract" {
				return iamClaims{}, fmt.Errorf("contract workspace scope required")
			}
			seen := map[string]bool{}
			for _, action := range claims.Actions {
				if seen[action] || (action != "contract-ledger.view" && action != "contract-ledger.draft.manage" && action != "contract-ledger.lifecycle.manage" && action != "contract-ledger.evidence.manage") {
					return iamClaims{}, fmt.Errorf("invalid workspace capability")
				}
				seen[action] = true
			}
		}
	}
	for _, action := range claims.Actions {
		if !validContractAction(action) {
			return iamClaims{}, fmt.Errorf("unknown capability")
		}
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
	keys := map[string]ed25519.PublicKey{}
	for _, item := range set.Keys {
		if item.KTY != "OKP" || item.CRV != "Ed25519" || item.Use != "sig" || item.ALG != "EdDSA" || (item.Status != "active" && item.Status != "retiring") || !iamUUIDPattern.MatchString(item.KeyID) {
			return nil, fmt.Errorf("invalid public key metadata")
		}
		raw, err := base64.RawURLEncoding.DecodeString(item.X)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("invalid public key")
		}
		if _, exists := keys[item.KeyID]; exists {
			return nil, fmt.Errorf("duplicate public key")
		}
		keys[item.KeyID] = ed25519.PublicKey(raw)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("public key set is empty")
	}
	return keys, nil
}
func validContractAction(action string) bool {
	switch action {
	case "contract-ledger.view", "contract-ledger.party.register", "contract-ledger.draft.manage", "contract-ledger.lifecycle.manage", "contract-ledger.correct", "contract-ledger.evidence.manage", "contract-ledger.export", "contract-ledger.restore", "contract-ledger.reference.read":
		return true
	default:
		return false
	}
}
func decodeIAMPart(raw string, destination any) error {
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

func (p *IAMProvider) Login(context.Context, string, string) (LoginResult, error) {
	return LoginResult{}, &ExternalIdentityError{}
}
func (p *IAMProvider) Logout(context.Context, string) error { return &ExternalIdentityError{} }
func (p *IAMProvider) ListUsers(context.Context) ([]User, error) {
	return nil, &ExternalIdentityError{}
}
func (p *IAMProvider) CreateUser(context.Context, CreateUserInput) (User, error) {
	return User{}, &ExternalIdentityError{}
}
func (p *IAMProvider) ChangeRole(context.Context, ChangeRoleInput) error {
	return &ExternalIdentityError{}
}
func (p *IAMProvider) ChangeStatus(context.Context, ChangeStatusInput) error {
	return &ExternalIdentityError{}
}
func (p *IAMProvider) ChangePassword(context.Context, ChangePasswordInput) error {
	return &ExternalIdentityError{}
}
func (p *IAMProvider) ResetPassword(context.Context, ResetPasswordInput) error {
	return &ExternalIdentityError{}
}
