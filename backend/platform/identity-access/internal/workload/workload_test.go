package workload

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"omsbizfoundry/identity-access/internal/identity"
)

const (
	testTenant  = "11111111-1111-4111-8111-111111111111"
	testSubject = "22222222-2222-4222-8222-222222222222"
	testKeyID   = "33333333-3333-4333-8333-333333333333"
)

type memoryNonces struct{ used map[string]bool }

func (m *memoryNonces) Use(_ context.Context, workloadID, assertionID string, _, _ time.Time) error {
	key := workloadID + "\x00" + assertionID
	if m.used[key] {
		return ErrReplay
	}
	m.used[key] = true
	return nil
}

func TestAuthenticatorVerifiesBindingPermissionAndReplay(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	publicValue, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	publicPath := filepath.Join(directory, "public.json")
	policyPath := filepath.Join(directory, "policy.json")
	writeJSON(t, publicPath, publicBundle{Version: "1", WorkloadID: "project-workspace", SubjectID: testSubject, Keys: []publicKey{{KTY: "OKP", CRV: "Ed25519", Use: "sig", ALG: "EdDSA", KeyID: testKeyID, X: base64.RawURLEncoding.EncodeToString(publicValue), Status: "active", CreatedAt: now.Format(time.RFC3339Nano)}}})
	writeJSON(t, policyPath, policyFile{SchemaVersion: "1", Identities: []Identity{{ID: "project-workspace", SubjectID: testSubject, DisplayName: "Project Workspace", PublicKeysPath: publicPath, Permissions: []Permission{{Audience: "people-groups", Action: "people-groups.view", ResourceTypes: []string{"person"}}}}}})
	authenticator, err := Load(policyPath, &memoryNonces{used: map[string]bool{}})
	if err != nil {
		t.Fatal(err)
	}
	authenticator.now = func() time.Time { return now }
	scope := identity.Scope{Kind: "resources", Resources: []identity.ResourceRef{{Type: "person", ID: testSubject}}}
	hash, _ := RequestHash("people-groups", "people-groups.view", scope)
	assertion := signAssertion(t, privateKey, Claims{Version: "1", Issuer: "project-workspace", Subject: testSubject, Tenant: testTenant, Audience: "identity-access", Purpose: "capability-exchange", RequestSHA256: hash, IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix(), ID: "44444444-4444-4444-8444-444444444444"})
	value, err := authenticator.Authenticate(t.Context(), assertion, hash)
	if err != nil {
		t.Fatal(err)
	}
	if value.TenantID != testTenant {
		t.Fatalf("tenant = %q, want %q", value.TenantID, testTenant)
	}
	if err := authenticator.Authorize(value, "people-groups", "people-groups.view", scope); err != nil {
		t.Fatal(err)
	}
	if err := authenticator.Authorize(value, "people-groups", "people-groups.person.manage", scope); !errors.Is(err, ErrForbidden) {
		t.Fatalf("write capability error=%v", err)
	}
	if _, err := authenticator.Authenticate(t.Context(), assertion, hash); !errors.Is(err, ErrReplay) {
		t.Fatalf("replay error=%v", err)
	}
	wrongHash, _ := RequestHash("people-groups", "people-groups.view", identity.Scope{Kind: "resources", Resources: []identity.ResourceRef{{Type: "person", ID: "55555555-5555-4555-8555-555555555555"}}})
	if _, err := authenticator.Authenticate(t.Context(), assertion, wrongHash); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("request binding error=%v", err)
	}
	otherTenant := "55555555-5555-4555-8555-555555555555"
	otherAssertion := signAssertion(t, privateKey, Claims{Version: "1", Issuer: "project-workspace", Subject: testSubject, Tenant: otherTenant, Audience: "identity-access", Purpose: "capability-exchange", RequestSHA256: hash, IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix(), ID: "66666666-6666-4666-8666-666666666666"})
	otherValue, err := authenticator.Authenticate(t.Context(), otherAssertion, hash)
	if err != nil || otherValue.TenantID != otherTenant {
		t.Fatalf("second tenant assertion = %#v, %v", otherValue, err)
	}
	invalidTenantAssertion := signAssertion(t, privateKey, Claims{Version: "1", Issuer: "project-workspace", Subject: testSubject, Tenant: "client-supplied", Audience: "identity-access", Purpose: "capability-exchange", RequestSHA256: hash, IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix(), ID: "77777777-7777-4777-8777-777777777777"})
	if _, err := authenticator.Authenticate(t.Context(), invalidTenantAssertion, hash); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("invalid tenant assertion error=%v", err)
	}
}

func signAssertion(t *testing.T, privateKey ed25519.PrivateKey, claims Claims) string {
	t.Helper()
	headerJSON, _ := json.Marshal(header{Algorithm: "EdDSA", Type: "JWT", KeyID: testKeyID})
	claimsJSON, _ := json.Marshal(claims)
	unsigned := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, []byte(unsigned)))
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(fmt.Errorf("write fixture: %w", err))
	}
}
