package workload

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/identity-access/internal/identity"
)

var (
	uuidPattern       = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{2,63}$`)
	actionPattern     = regexp.MustCompile(`^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)+$`)
	hexPattern        = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

var (
	ErrUnauthenticated = errors.New("workload assertion is invalid")
	ErrForbidden       = errors.New("workload capability is not allowed")
	ErrReplay          = errors.New("workload assertion was already used")
)

type Permission struct {
	Audience      string   `json:"audience"`
	Action        string   `json:"action"`
	ResourceTypes []string `json:"resource_types"`
}

type Identity struct {
	ID             string       `json:"id"`
	SubjectID      string       `json:"subject_id"`
	DisplayName    string       `json:"display_name"`
	PublicKeysPath string       `json:"public_keys_path"`
	Permissions    []Permission `json:"permissions"`
}

type policyFile struct {
	SchemaVersion string     `json:"schema_version"`
	Identities    []Identity `json:"identities"`
}

type header struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	KeyID     string `json:"kid"`
}

type Claims struct {
	Version       string `json:"ver"`
	Issuer        string `json:"iss"`
	Subject       string `json:"sub"`
	Tenant        string `json:"tid"`
	Audience      string `json:"aud"`
	Purpose       string `json:"purpose"`
	RequestSHA256 string `json:"request_sha256"`
	IssuedAt      int64  `json:"iat"`
	NotBefore     int64  `json:"nbf"`
	ExpiresAt     int64  `json:"exp"`
	ID            string `json:"jti"`
}

type publicKey struct {
	KTY         string `json:"kty"`
	CRV         string `json:"crv"`
	Use         string `json:"use"`
	ALG         string `json:"alg"`
	KeyID       string `json:"kid"`
	X           string `json:"x"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	RetireAfter string `json:"retire_after,omitempty"`
}

type publicBundle struct {
	Version    string      `json:"version"`
	WorkloadID string      `json:"workload_id"`
	SubjectID  string      `json:"subject_id"`
	Keys       []publicKey `json:"keys"`
}

type NonceStore interface {
	Use(context.Context, string, string, time.Time, time.Time) error
}

type PostgresNonceStore struct{ pool *pgxpool.Pool }

func NewPostgresNonceStore(pool *pgxpool.Pool) *PostgresNonceStore {
	return &PostgresNonceStore{pool: pool}
}

func (s *PostgresNonceStore) Use(ctx context.Context, workloadID, assertionID string, expiresAt, now time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `DELETE FROM workload_assertion_nonces WHERE expires_at < $1`, now.Add(-time.Minute)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO workload_assertion_nonces(assertion_id,workload_id,expires_at,used_at) VALUES($1,$2,$3,$4)`, assertionID, workloadID, expiresAt, now); err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			return ErrReplay
		}
		return err
	}
	return tx.Commit(ctx)
}

type Authenticated struct {
	Identity    Identity
	TenantID    string
	KeyID       string
	AssertionID string
	IssuedAt    time.Time
}

type Authenticator struct {
	identities map[string]Identity
	nonces     NonceStore
	now        func() time.Time
}

func Load(policyPath string, nonces NonceStore) (*Authenticator, error) {
	if nonces == nil {
		return nil, fmt.Errorf("workload identity policy is invalid")
	}
	identities, err := loadIdentities(policyPath)
	if err != nil {
		return nil, err
	}
	return &Authenticator{identities: identities, nonces: nonces, now: func() time.Time { return time.Now().UTC() }}, nil
}

// Validate checks the workload policy document without requiring a database
// backed nonce store. Runtime authentication still requires Load.
func Validate(policyPath string) error {
	_, err := loadIdentities(policyPath)
	return err
}

func loadIdentities(policyPath string) (map[string]Identity, error) {
	data, err := os.ReadFile(strings.TrimSpace(policyPath))
	if err != nil {
		return nil, fmt.Errorf("read workload identity policy: %w", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var value policyFile
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode workload identity policy: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("workload identity policy contains trailing JSON")
	}
	if value.SchemaVersion != "1" || len(value.Identities) == 0 {
		return nil, fmt.Errorf("workload identity policy is invalid")
	}
	identities := make(map[string]Identity, len(value.Identities))
	for _, item := range value.Identities {
		if !identifierPattern.MatchString(item.ID) || !uuidPattern.MatchString(item.SubjectID) || strings.TrimSpace(item.DisplayName) == "" || strings.TrimSpace(item.PublicKeysPath) == "" || len(item.Permissions) == 0 {
			return nil, fmt.Errorf("workload identity %q is invalid", item.ID)
		}
		if _, duplicate := identities[item.ID]; duplicate {
			return nil, fmt.Errorf("duplicate workload identity %q", item.ID)
		}
		for _, permission := range item.Permissions {
			if !identifierPattern.MatchString(permission.Audience) || !actionPattern.MatchString(permission.Action) || len(permission.ResourceTypes) == 0 {
				return nil, fmt.Errorf("workload identity %q permission is invalid", item.ID)
			}
		}
		identities[item.ID] = item
	}
	return identities, nil
}

func (a *Authenticator) Authenticate(ctx context.Context, assertion, requestHash string) (Authenticated, error) {
	parts := strings.Split(assertion, ".")
	if len(parts) != 3 || !hexPattern.MatchString(requestHash) {
		return Authenticated{}, ErrUnauthenticated
	}
	var tokenHeader header
	if err := decodePart(parts[0], &tokenHeader); err != nil || tokenHeader.Algorithm != "EdDSA" || tokenHeader.Type != "JWT" || !uuidPattern.MatchString(tokenHeader.KeyID) {
		return Authenticated{}, ErrUnauthenticated
	}
	var claims Claims
	if err := decodePart(parts[1], &claims); err != nil {
		return Authenticated{}, ErrUnauthenticated
	}
	identityValue, ok := a.identities[claims.Issuer]
	if !ok || claims.Subject != identityValue.SubjectID || !uuidPattern.MatchString(claims.Tenant) || claims.Version != "1" || claims.Audience != "identity-access" || claims.Purpose != "capability-exchange" || claims.RequestSHA256 != requestHash || !uuidPattern.MatchString(claims.ID) {
		return Authenticated{}, ErrUnauthenticated
	}
	now := a.now()
	if claims.IssuedAt > now.Unix()+30 || claims.NotBefore > now.Unix()+30 || claims.ExpiresAt <= now.Unix()-30 || claims.ExpiresAt <= claims.IssuedAt || claims.ExpiresAt-claims.IssuedAt > 60 {
		return Authenticated{}, ErrUnauthenticated
	}
	key, err := loadPublicKey(identityValue, tokenHeader.KeyID, now)
	if err != nil {
		return Authenticated{}, ErrUnauthenticated
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !ed25519.Verify(key, []byte(parts[0]+"."+parts[1]), signature) {
		return Authenticated{}, ErrUnauthenticated
	}
	if err := a.nonces.Use(ctx, identityValue.ID, claims.ID, time.Unix(claims.ExpiresAt, 0).UTC(), now); err != nil {
		return Authenticated{}, err
	}
	return Authenticated{Identity: identityValue, TenantID: claims.Tenant, KeyID: tokenHeader.KeyID, AssertionID: claims.ID, IssuedAt: time.Unix(claims.IssuedAt, 0).UTC()}, nil
}

func (a *Authenticator) Authorize(value Authenticated, audience, action string, scope identity.Scope) error {
	for _, permission := range value.Identity.Permissions {
		if permission.Audience != audience || permission.Action != action {
			continue
		}
		if scope.Kind != "resources" || len(scope.Resources) == 0 || len(scope.Resources) > 32 {
			return ErrForbidden
		}
		allowedTypes := map[string]bool{}
		for _, resourceType := range permission.ResourceTypes {
			allowedTypes[resourceType] = true
		}
		for _, resource := range scope.Resources {
			if !allowedTypes[resource.Type] || strings.TrimSpace(resource.ID) == "" {
				return ErrForbidden
			}
		}
		return nil
	}
	return ErrForbidden
}

func RequestHash(audience, action string, scope identity.Scope) (string, error) {
	value := struct {
		Audience string         `json:"audience"`
		Action   string         `json:"action"`
		Scope    identity.Scope `json:"scope"`
	}{audience, action, scope}
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

func loadPublicKey(identityValue Identity, keyID string, now time.Time) (ed25519.PublicKey, error) {
	data, err := os.ReadFile(identityValue.PublicKeysPath)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var bundle publicBundle
	if err := decoder.Decode(&bundle); err != nil || bundle.Version != "1" || bundle.WorkloadID != identityValue.ID || bundle.SubjectID != identityValue.SubjectID {
		return nil, ErrUnauthenticated
	}
	for _, item := range bundle.Keys {
		if item.KeyID != keyID {
			continue
		}
		if item.KTY != "OKP" || item.CRV != "Ed25519" || item.Use != "sig" || item.ALG != "EdDSA" || (item.Status != "active" && item.Status != "retiring") {
			return nil, ErrUnauthenticated
		}
		if item.Status == "retiring" {
			retireAfter, parseErr := time.Parse(time.RFC3339Nano, item.RetireAfter)
			if parseErr != nil || !now.Before(retireAfter) {
				return nil, ErrUnauthenticated
			}
		}
		raw, decodeErr := base64.RawURLEncoding.DecodeString(item.X)
		if decodeErr != nil || len(raw) != ed25519.PublicKeySize {
			return nil, ErrUnauthenticated
		}
		return ed25519.PublicKey(raw), nil
	}
	return nil, ErrUnauthenticated
}

func decodePart(raw string, destination any) error {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	return decoder.Decode(destination)
}
