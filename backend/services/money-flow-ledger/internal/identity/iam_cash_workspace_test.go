package identity

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIAMCashWorkspaceScopeRejectsCreateMixingAndNeighborResources(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	kid := "33333333-3333-4333-8333-333333333333"
	path := filepath.Join(t.TempDir(), "keys.json")
	data, _ := json.Marshal(map[string]any{"keys": []map[string]string{{"kty": "OKP", "crv": "Ed25519", "use": "sig", "alg": "EdDSA", "kid": kid, "x": base64.RawURLEncoding.EncodeToString(pub), "status": "active"}}})
	if e := os.WriteFile(path, data, 0600); e != nil {
		t.Fatal(e)
	}
	p, e := NewIAMProvider("https://cash.example.test", "22222222-2222-4222-8222-222222222222", path)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC()
	base := map[string]any{"ver": "1", "iss": "https://cash.example.test", "sub": "11111111-1111-4111-8111-111111111111", "tid": "22222222-2222-4222-8222-222222222222", "aud": "money-flow-ledger", "name": "Cash user", "actions": []string{"money-flow-ledger.view", "money-flow-ledger.amount.read", "money-flow-ledger.cash.reverse"}, "scope": map[string]any{"kind": "resources", "resources": []map[string]string{{"type": "cash_transaction", "id": "55555555-5555-4555-8555-555555555555"}}}, "iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(), "auth_time": now.Unix(), "jti": "44444444-4444-4444-8444-444444444444"}
	if v, ok, e := p.ResolveSession(context.Background(), signIAMToken(t, kid, key, base)); e != nil || !ok || !v.Can(PermissionCashReverse) || v.Can(PermissionCashRecord) || v.CanReference("cash_transaction", "66666666-6666-4666-8666-666666666666") {
		t.Fatal(v, ok, e)
	}
	for _, actions := range [][]string{{"money-flow-ledger.cash.record"}, {"money-flow-ledger.view", "money-flow-ledger.reference.read"}, {"money-flow-ledger.view", "money-flow-ledger.view"}, {"money-flow-ledger.obligation.manage"}, {}} {
		v := copyClaims(base)
		v["actions"] = actions
		if _, ok, e := p.ResolveSession(context.Background(), signIAMToken(t, kid, key, v)); e != nil || ok {
			t.Fatal("invalid cash actions accepted", actions, ok, e)
		}
	}
	v := copyClaims(base)
	v["scope"] = map[string]any{"kind": "resources", "resources": []map[string]string{{"type": "cash_transaction", "id": "55555555-5555-4555-8555-555555555555"}, {"type": "cash_transaction", "id": "66666666-6666-4666-8666-666666666666"}}}
	if _, ok, e := p.ResolveSession(context.Background(), signIAMToken(t, kid, key, v)); e != nil || ok {
		t.Fatal("multi-resource accepted", ok, e)
	}
}
