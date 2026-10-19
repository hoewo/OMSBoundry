package credential

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestVerifierAcceptsValidCredentialAndFailsClosed(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	keyID := "33333333-3333-4333-8333-333333333333"
	path := filepath.Join(t.TempDir(), "keys.json")
	keyData, _ := json.Marshal(map[string]any{"keys": []map[string]string{{"kty": "OKP", "crv": "Ed25519", "use": "sig", "alg": "EdDSA", "kid": keyID, "x": base64.RawURLEncoding.EncodeToString(publicKey), "status": "active"}}})
	if err := os.WriteFile(path, keyData, 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	verifier := New(path, "https://oms.example.test/auth", "22222222-2222-4222-8222-222222222222")
	verifier.now = func() time.Time { return now }
	base := map[string]any{"ver": "1", "iss": "https://oms.example.test/auth", "sub": "11111111-1111-4111-8111-111111111111", "tid": "22222222-2222-4222-8222-222222222222", "aud": "contract-ledger", "name": "User", "actions": []string{"contract-ledger.view"}, "scope": map[string]any{"kind": "tenant", "resources": []any{}}, "iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(), "auth_time": now.Add(-time.Minute).Unix(), "jti": "44444444-4444-4444-8444-444444444444"}
	valid := sign(t, keyID, privateKey, base)
	if _, err := verifier.Verify(valid, "contract-ledger", "contract-ledger.view"); err != nil {
		t.Fatal(err)
	}
	claims, err := verifier.Verify(valid, "contract-ledger", "contract-ledger.export", "contract-ledger.view")
	if err != nil || len(claims.Actions) != 1 || claims.Actions[0] != "contract-ledger.view" {
		t.Fatal("ANY admission changed or rejected original actions")
	}
	for _, entries := range [][]string{nil, {}, {""}, {"*"}, {"contract-ledger.*"}, {"contract-ledger.export"}} {
		if _, err := verifier.Verify(valid, "contract-ledger", entries...); err == nil {
			t.Fatalf("unmatched admission accepted: %v", entries)
		}
	}
	resourceScoped := clone(t, base)
	resourceScoped["scope"] = map[string]any{"kind": "resources", "resources": []map[string]string{{"type": "contract", "id": "55555555-5555-4555-8555-555555555555"}}}
	if claims, err := verifier.Verify(sign(t, keyID, privateKey, resourceScoped), "contract-ledger", "contract-ledger.view"); err != nil || claims.Scope.Kind != "resources" || len(claims.Scope.Resources) != 1 {
		t.Fatalf("valid resource credential rejected: claims=%#v err=%v", claims, err)
	}
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{{"wrong issuer", func(v map[string]any) { v["iss"] = "https://attacker.test" }}, {"wrong tenant", func(v map[string]any) { v["tid"] = "55555555-5555-4555-8555-555555555555" }}, {"wrong audience", func(v map[string]any) { v["aud"] = "money-flow-ledger" }}, {"missing entry action", func(v map[string]any) { v["actions"] = []string{"contract-ledger.export"} }}, {"expired", func(v map[string]any) { v["exp"] = now.Add(-time.Minute).Unix() }}, {"long ttl", func(v map[string]any) { v["exp"] = now.Add(6 * time.Minute).Unix() }}, {"empty resource scope", func(v map[string]any) {
		v["scope"] = map[string]any{"kind": "resources", "resources": []any{}}
	}}, {"tenant scope with resource", func(v map[string]any) {
		v["scope"] = map[string]any{"kind": "tenant", "resources": []map[string]string{{"type": "contract", "id": "one"}}}
	}}, {"duplicate resource", func(v map[string]any) {
		v["scope"] = map[string]any{"kind": "resources", "resources": []map[string]string{{"type": "contract", "id": "one"}, {"type": "contract", "id": "one"}}}
	}}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := clone(t, base)
			test.mutate(value)
			if _, err := verifier.Verify(sign(t, keyID, privateKey, value), "contract-ledger", "contract-ledger.view"); err == nil {
				t.Fatal("invalid credential accepted")
			}
		})
	}
	tamperedBytes := []byte(valid)
	signatureStart := len(tamperedBytes) - 1
	for signatureStart >= 0 && tamperedBytes[signatureStart] != '.' {
		signatureStart--
	}
	if signatureStart < 0 || signatureStart+1 >= len(tamperedBytes) {
		t.Fatal("signed credential has no signature segment")
	}
	signatureStart++
	if tamperedBytes[signatureStart] == 'A' {
		tamperedBytes[signatureStart] = 'B'
	} else {
		tamperedBytes[signatureStart] = 'A'
	}
	tampered := string(tamperedBytes)
	if _, err := verifier.Verify(tampered, "contract-ledger", "contract-ledger.view"); err == nil {
		t.Fatal("tampered signature accepted")
	}
}
func sign(t *testing.T, keyID string, key ed25519.PrivateKey, claims any) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": keyID})
	body, _ := json.Marshal(claims)
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, []byte(unsigned)))
}
func clone(t *testing.T, value map[string]any) map[string]any {
	t.Helper()
	data, _ := json.Marshal(value)
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
