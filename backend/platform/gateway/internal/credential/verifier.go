package credential

import (
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

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
var resourceTypePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

type header struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	KeyID     string `json:"kid"`
}
type jwk struct {
	KTY    string `json:"kty"`
	CRV    string `json:"crv"`
	Use    string `json:"use"`
	ALG    string `json:"alg"`
	KeyID  string `json:"kid"`
	X      string `json:"x"`
	Status string `json:"status"`
}
type keySet struct {
	Keys []jwk `json:"keys"`
}
type ResourceRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}
type Scope struct {
	Kind      string        `json:"kind"`
	Resources []ResourceRef `json:"resources"`
}
type Claims struct {
	Version   string   `json:"ver"`
	Issuer    string   `json:"iss"`
	Subject   string   `json:"sub"`
	Tenant    string   `json:"tid"`
	Audience  string   `json:"aud"`
	Name      string   `json:"name"`
	Actions   []string `json:"actions"`
	Scope     Scope    `json:"scope"`
	IssuedAt  int64    `json:"iat"`
	NotBefore int64    `json:"nbf"`
	ExpiresAt int64    `json:"exp"`
	AuthTime  int64    `json:"auth_time"`
	ID        string   `json:"jti"`
	Person    *struct {
		ID            string `json:"id"`
		TenantID      string `json:"tenant_id"`
		Status        string `json:"status"`
		Revision      int64  `json:"revision"`
		ReferenceCode string `json:"reference_code,omitempty"`
		DisplayName   string `json:"display_name"`
		Digest        string `json:"digest"`
	} `json:"person,omitempty"`
}
type Verifier struct {
	path, issuer, tenant string
	now                  func() time.Time
}

func New(path, issuer, tenant string) *Verifier {
	return &Verifier{path: path, issuer: strings.TrimRight(issuer, "/"), tenant: tenant, now: func() time.Time { return time.Now().UTC() }}
}
func (v *Verifier) Ready() error { _, err := v.loadKeys(); return err }
func (v *Verifier) Verify(token, audience string, entryActions ...string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, fmt.Errorf("credential is not compact JWS")
	}
	var tokenHeader header
	if err := decodePart(parts[0], &tokenHeader); err != nil {
		return Claims{}, fmt.Errorf("decode header: %w", err)
	}
	if tokenHeader.Algorithm != "EdDSA" || tokenHeader.Type != "JWT" || !uuidPattern.MatchString(tokenHeader.KeyID) {
		return Claims{}, fmt.Errorf("credential header is invalid")
	}
	keys, err := v.loadKeys()
	if err != nil {
		return Claims{}, err
	}
	publicKey, found := keys[tokenHeader.KeyID]
	if !found {
		return Claims{}, fmt.Errorf("credential key is unknown")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !ed25519.Verify(publicKey, []byte(parts[0]+"."+parts[1]), signature) {
		return Claims{}, fmt.Errorf("credential signature is invalid")
	}
	var claims Claims
	if err := decodePart(parts[1], &claims); err != nil {
		return Claims{}, fmt.Errorf("decode claims: %w", err)
	}
	now := v.now().Unix()
	if claims.Version != "1" || claims.Issuer != v.issuer || claims.Tenant != v.tenant || claims.Audience != audience || !uuidPattern.MatchString(claims.Subject) || !uuidPattern.MatchString(claims.ID) {
		return Claims{}, fmt.Errorf("credential identity is invalid")
	}
	if claims.IssuedAt > now+30 || claims.NotBefore > now+30 || claims.ExpiresAt <= now-30 || claims.ExpiresAt <= claims.IssuedAt || claims.ExpiresAt-claims.IssuedAt > 300 || claims.AuthTime <= 0 || claims.AuthTime > claims.IssuedAt {
		return Claims{}, fmt.Errorf("credential time is invalid")
	}
	matchedEntry := false
	for _, action := range entryActions {
		if action != "" && slices.Contains(claims.Actions, action) {
			matchedEntry = true
		}
	}
	if !matchedEntry {
		return Claims{}, fmt.Errorf("credential lacks entry action")
	}
	if claims.Scope.Kind == "tenant" {
		if len(claims.Scope.Resources) != 0 {
			return Claims{}, fmt.Errorf("tenant credential scope contains resources")
		}
	} else if claims.Scope.Kind == "resources" {
		if len(claims.Scope.Resources) == 0 || len(claims.Scope.Resources) > 32 {
			return Claims{}, fmt.Errorf("resource credential scope size is invalid")
		}
		seen := map[string]struct{}{}
		for _, resource := range claims.Scope.Resources {
			if !resourceTypePattern.MatchString(resource.Type) || strings.TrimSpace(resource.ID) != resource.ID || resource.ID == "" || len(resource.ID) > 128 {
				return Claims{}, fmt.Errorf("resource credential scope contains invalid resource")
			}
			key := resource.Type + "\x00" + resource.ID
			if _, duplicate := seen[key]; duplicate {
				return Claims{}, fmt.Errorf("resource credential scope contains duplicate resource")
			}
			seen[key] = struct{}{}
		}
	} else {
		return Claims{}, fmt.Errorf("credential scope kind is invalid")
	}
	return claims, nil
}
func (v *Verifier) loadKeys() (map[string]ed25519.PublicKey, error) {
	data, err := os.ReadFile(v.path)
	if err != nil {
		return nil, fmt.Errorf("read IAM public keys: %w", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var set keySet
	if err := decoder.Decode(&set); err != nil {
		return nil, fmt.Errorf("decode IAM public keys: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("IAM public keys contain trailing JSON")
	}
	keys := map[string]ed25519.PublicKey{}
	for _, item := range set.Keys {
		if item.KTY != "OKP" || item.CRV != "Ed25519" || item.Use != "sig" || item.ALG != "EdDSA" || (item.Status != "active" && item.Status != "retiring") || !uuidPattern.MatchString(item.KeyID) {
			return nil, fmt.Errorf("IAM public key metadata is invalid")
		}
		raw, err := base64.RawURLEncoding.DecodeString(item.X)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("IAM public key material is invalid")
		}
		if _, duplicate := keys[item.KeyID]; duplicate {
			return nil, fmt.Errorf("duplicate IAM public key")
		}
		keys[item.KeyID] = ed25519.PublicKey(raw)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("IAM public key set is empty")
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
