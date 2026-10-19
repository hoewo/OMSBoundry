package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIAMProviderAcceptsExactPartyAndRejectsWrongAudience(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	path := filepath.Join(t.TempDir(), "keys.json")
	keySet := map[string]any{"keys": []map[string]any{{"kty": "OKP", "crv": "Ed25519", "use": "sig", "alg": "EdDSA", "kid": keyID, "x": base64.RawURLEncoding.EncodeToString(public), "status": "active"}}}
	raw, _ := json.Marshal(keySet)
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	provider, err := NewIAMProvider("https://iam.example.test", "11111111-1111-4111-8111-111111111111", path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1800000000, 0).UTC()
	provider.now = func() time.Time { return now }
	claims := iamClaims{Version: "1", Issuer: "https://iam.example.test", Subject: "22222222-2222-4222-8222-222222222222", Tenant: "11111111-1111-4111-8111-111111111111", Audience: "party-directory", Name: "工作负载", Actions: []string{ActionReferenceRead}, Scope: iamScope{Kind: "resources", Resources: []iamResource{{Type: "party", ID: "33333333-3333-4333-8333-333333333333"}}}, IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix(), AuthTime: now.Unix(), ID: "44444444-4444-4444-8444-444444444444"}
	token := signTestToken(t, keyID, claims, private)
	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	principal, err := provider.Authenticate(request)
	if err != nil {
		t.Fatal(err)
	}
	if !principal.Can(ActionReferenceRead) || !principal.CanParty("33333333-3333-4333-8333-333333333333") || principal.CanEnumerate() {
		t.Fatalf("unexpected principal: %#v", principal)
	}
	claims.Audience = "contract-ledger"
	request.Header.Set("Authorization", "Bearer "+signTestToken(t, keyID, claims, private))
	if _, err = provider.Authenticate(request); err != ErrUnauthenticated {
		t.Fatalf("wrong audience must fail: %v", err)
	}
	claims.Audience = "party-directory"
	claims.Scope.Resources[0].Type = "project"
	request.Header.Set("Authorization", "Bearer "+signTestToken(t, keyID, claims, private))
	if _, err = provider.Authenticate(request); err != ErrUnauthenticated {
		t.Fatalf("foreign resource type must fail: %v", err)
	}
	claims.Scope.Resources[0].Type = "party"
	claims.Tenant = "99999999-9999-4999-8999-999999999999"
	request.Header.Set("Authorization", "Bearer "+signTestToken(t, keyID, claims, private))
	if _, err = provider.Authenticate(request); err != ErrUnauthenticated {
		t.Fatalf("wrong tenant must fail: %v", err)
	}
}

func signTestToken(t *testing.T, keyID string, claims iamClaims, private ed25519.PrivateKey) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": keyID})
	body, _ := json.Marshal(claims)
	encodedHeader := base64.RawURLEncoding.EncodeToString(header)
	encodedBody := base64.RawURLEncoding.EncodeToString(body)
	signed := encodedHeader + "." + encodedBody
	return signed + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, []byte(signed)))
}
