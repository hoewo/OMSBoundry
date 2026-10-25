package capabilityauth

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"omsbizfoundry/identity-access/internal/identity"
	"omsbizfoundry/identity-access/internal/signing"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

var ErrUnauthenticated = errors.New("capability credential is invalid")

type header struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	KeyID     string `json:"kid"`
}

type Verifier struct {
	issuer string
	keys   *signing.Store
	now    func() time.Time
}

func New(issuer string, keys *signing.Store) (*Verifier, error) {
	issuer = strings.TrimRight(strings.TrimSpace(issuer), "/")
	if issuer == "" || keys == nil {
		return nil, fmt.Errorf("capability verifier configuration is invalid")
	}
	return &Verifier{issuer: issuer, keys: keys, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (v *Verifier) Authenticate(request *http.Request, audience, action, resourceType, resourceID string) (identity.CapabilityClaims, error) {
	authorization := strings.TrimSpace(request.Header.Get("Authorization"))
	if !strings.HasPrefix(authorization, "Bearer ") {
		return identity.CapabilityClaims{}, ErrUnauthenticated
	}
	parts := strings.Split(strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer ")), ".")
	if len(parts) != 3 {
		return identity.CapabilityClaims{}, ErrUnauthenticated
	}
	var tokenHeader header
	if decode(parts[0], &tokenHeader) != nil || tokenHeader.Algorithm != "EdDSA" || tokenHeader.Type != "JWT" || !uuidPattern.MatchString(tokenHeader.KeyID) {
		return identity.CapabilityClaims{}, ErrUnauthenticated
	}
	var claims identity.CapabilityClaims
	if decode(parts[1], &claims) != nil {
		return identity.CapabilityClaims{}, ErrUnauthenticated
	}
	now := v.now().Unix()
	if claims.Version != "1" || claims.Issuer != v.issuer || claims.Audience != audience || !uuidPattern.MatchString(claims.Subject) || !uuidPattern.MatchString(claims.Tenant) || !uuidPattern.MatchString(claims.ID) || strings.TrimSpace(claims.Name) == "" {
		return identity.CapabilityClaims{}, ErrUnauthenticated
	}
	if claims.IssuedAt > now+30 || claims.NotBefore > now+30 || claims.ExpiresAt <= now-30 || claims.ExpiresAt <= claims.IssuedAt || claims.ExpiresAt-claims.IssuedAt > 300 || claims.AuthTime <= 0 || claims.AuthTime > claims.IssuedAt {
		return identity.CapabilityClaims{}, ErrUnauthenticated
	}
	if len(claims.Actions) != 1 || claims.Actions[0] != action || claims.Scope.Kind != "resources" || len(claims.Scope.Resources) != 1 || claims.Scope.Resources[0] != (identity.ResourceRef{Type: resourceType, ID: resourceID}) {
		return identity.CapabilityClaims{}, ErrUnauthenticated
	}
	keys, err := v.keys.PublicKeys(v.now())
	if err != nil {
		return identity.CapabilityClaims{}, ErrUnauthenticated
	}
	var publicKey ed25519.PublicKey
	for _, key := range keys {
		if key.KeyID == tokenHeader.KeyID {
			publicKey = key.PublicKey
			break
		}
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(publicKey) != ed25519.PublicKeySize || !ed25519.Verify(publicKey, []byte(parts[0]+"."+parts[1]), signature) {
		return identity.CapabilityClaims{}, ErrUnauthenticated
	}
	return claims, nil
}

func decode(raw string, destination any) error {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, destination)
}
