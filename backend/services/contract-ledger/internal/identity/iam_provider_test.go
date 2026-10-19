package identity

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"omsbizfoundry/contract-ledger/internal/audit"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIAMProviderVerifiesCredentialMapsCapabilitiesAndActor(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	keyID := "33333333-3333-4333-8333-333333333333"
	path := filepath.Join(t.TempDir(), "keys.json")
	keyData, _ := json.Marshal(map[string]any{"keys": []map[string]string{{"kty": "OKP", "crv": "Ed25519", "use": "sig", "alg": "EdDSA", "kid": keyID, "x": base64.RawURLEncoding.EncodeToString(publicKey), "status": "active"}}})
	if err := os.WriteFile(path, keyData, 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := NewIAMProvider("https://oms.example.test/auth", "22222222-2222-4222-8222-222222222222", path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	provider.now = func() time.Time { return now }
	claims := map[string]any{"ver": "1", "iss": "https://oms.example.test/auth", "sub": "11111111-1111-4111-8111-111111111111", "tid": "22222222-2222-4222-8222-222222222222", "aud": "contract-ledger", "name": "中心管理员", "actions": []string{"contract-ledger.view", "contract-ledger.draft.manage"}, "scope": map[string]any{"kind": "tenant", "resources": []any{}}, "iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(), "auth_time": now.Add(-time.Minute).Unix(), "jti": "44444444-4444-4444-8444-444444444444"}
	token := signIAMToken(t, keyID, privateKey, claims)
	principal, found, err := provider.ResolveSession(context.Background(), token)
	if err != nil || !found {
		t.Fatalf("resolve=%v,%v", found, err)
	}
	if !principal.Can(PermissionBusinessView) || !principal.Can(PermissionDraftManage) || principal.Can(PermissionPartyRegister) || principal.Can(PermissionIdentityManage) {
		t.Fatalf("unexpected IAM permissions: %#v", principal.AllowedActions)
	}
	actor := principal.Actor()
	if actor.Kind != audit.ActorKindIAMSubject || actor.ID != principal.UserID || actor.DisplayName != "中心管理员" {
		t.Fatalf("actor=%#v", actor)
	}
	claims["aud"] = "money-flow-ledger"
	if _, found, err := provider.ResolveSession(context.Background(), signIAMToken(t, keyID, privateKey, claims)); err != nil || found {
		t.Fatal("wrong-audience credential accepted")
	}
}

func TestIAMProviderAcceptsOnlyExactContractReferenceCredential(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	keyID := "33333333-3333-4333-8333-333333333333"
	path := filepath.Join(t.TempDir(), "keys.json")
	keyData, _ := json.Marshal(map[string]any{"keys": []map[string]string{{"kty": "OKP", "crv": "Ed25519", "use": "sig", "alg": "EdDSA", "kid": keyID, "x": base64.RawURLEncoding.EncodeToString(publicKey), "status": "active"}}})
	if err := os.WriteFile(path, keyData, 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := NewIAMProvider("https://oms.example.test/auth", "22222222-2222-4222-8222-222222222222", path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	provider.now = func() time.Time { return now }
	claims := map[string]any{"ver": "1", "iss": "https://oms.example.test/auth", "sub": "11111111-1111-4111-8111-111111111111", "tid": "22222222-2222-4222-8222-222222222222", "aud": "contract-ledger", "name": "Billing workload", "actions": []string{"contract-ledger.reference.read"}, "scope": map[string]any{"kind": "resources", "resources": []map[string]string{{"type": "contract", "id": "55555555-5555-4555-8555-555555555555"}}}, "iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(), "auth_time": now.Add(-time.Minute).Unix(), "jti": "44444444-4444-4444-8444-444444444444"}
	principal, found, err := provider.ResolveSession(context.Background(), signIAMToken(t, keyID, privateKey, claims))
	if err != nil || !found || !principal.Can(PermissionReferenceRead) || !principal.CanReference("contract", "55555555-5555-4555-8555-555555555555") || principal.CanReference("contract", "66666666-6666-4666-8666-666666666666") {
		t.Fatalf("principal=%#v found=%v err=%v", principal, found, err)
	}
	for _, test := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"cross tenant", func(value map[string]any) { value["tid"] = "66666666-6666-4666-8666-666666666666" }},
		{"privileged action outside native workspace", func(value map[string]any) { value["actions"] = []string{"contract-ledger.restore"} }},
		{"mixed reference and business actions", func(value map[string]any) {
			value["actions"] = []string{"contract-ledger.reference.read", "contract-ledger.view"}
		}},
		{"wrong type", func(value map[string]any) {
			value["scope"] = map[string]any{"kind": "resources", "resources": []map[string]string{{"type": "obligation", "id": "55555555-5555-4555-8555-555555555555"}}}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			copyClaims := make(map[string]any, len(claims))
			for key, value := range claims {
				copyClaims[key] = value
			}
			test.mutate(copyClaims)
			if _, found, err := provider.ResolveSession(context.Background(), signIAMToken(t, keyID, privateKey, copyClaims)); err != nil || found {
				t.Fatalf("found=%v err=%v", found, err)
			}
		})
	}
	for _, test := range []struct {
		name         string
		actions      []string
		resourceType string
		want         bool
	}{
		{"exact read", []string{"contract-ledger.view"}, "contract", true},
		{"exact draft", []string{"contract-ledger.draft.manage"}, "contract", true},
		{"exact lifecycle", []string{"contract-ledger.view", "contract-ledger.lifecycle.manage"}, "contract", true},
		{"exact material references", []string{"contract-ledger.view", "contract-ledger.evidence.manage"}, "contract", true},
		{"no settlement material write", []string{"contract-ledger.evidence.manage"}, "settlement", false},
		{"no material reference mix", []string{"contract-ledger.evidence.manage", "contract-ledger.reference.read"}, "contract", false},
		{"no settlement write", []string{"contract-ledger.lifecycle.manage"}, "settlement", false},
		{"no reference mix", []string{"contract-ledger.view", "contract-ledger.reference.read"}, "contract", false},
		{"no duplicated capabilities", []string{"contract-ledger.view", "contract-ledger.view"}, "contract", false},
		{"no party administration", []string{"contract-ledger.party.register"}, "contract", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := make(map[string]any, len(claims))
			for k, v := range claims {
				candidate[k] = v
			}
			candidate["actions"] = test.actions
			candidate["scope"] = map[string]any{"kind": "resources", "resources": []map[string]string{{"type": test.resourceType, "id": "55555555-5555-4555-8555-555555555555"}}}
			p, found, err := provider.ResolveSession(context.Background(), signIAMToken(t, keyID, privateKey, candidate))
			if err != nil || found != test.want {
				t.Fatalf("found=%v err=%v", found, err)
			}
			if found && (!p.CanReference("contract", "55555555-5555-4555-8555-555555555555") || p.CanReference("contract", "66666666-6666-4666-8666-666666666666")) {
				t.Fatal("exact native scope widened")
			}
		})
	}
}
func signIAMToken(t *testing.T, keyID string, key ed25519.PrivateKey, claims any) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": keyID})
	body, _ := json.Marshal(claims)
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, []byte(unsigned)))
}
