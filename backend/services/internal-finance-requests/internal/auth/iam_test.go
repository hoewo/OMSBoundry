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

func TestIAMProviderValidatesAudienceCapabilityScopeAndSignature(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	path := filepath.Join(t.TempDir(), "keys.json")
	raw, _ := json.Marshal(map[string]any{"keys": []any{map[string]any{"kty": "OKP", "crv": "Ed25519", "use": "sig", "alg": "EdDSA", "kid": keyID, "x": base64.RawURLEncoding.EncodeToString(public), "status": "active"}}})
	if err = os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := NewIAMProvider("https://iam.example.test/auth", "11111111-1111-4111-8111-111111111111", path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1800000000, 0).UTC()
	provider.now = func() time.Time { return now }
	claims := iamClaims{Version: "1", Issuer: "https://iam.example.test/auth", Subject: "22222222-2222-4222-8222-222222222222", Tenant: "99999999-9999-4999-8999-999999999999", Audience: "internal-finance-requests", Name: "Finance User", Actions: []string{ActionSelfRead}, Scope: iamScope{Kind: "resources", Resources: []iamResource{{Type: "finance_request", ID: "33333333-3333-4333-8333-333333333333"}}}, IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix(), AuthTime: now.Unix(), ID: "44444444-4444-4444-8444-444444444444"}
	token := signedToken(t, keyID, private, claims)
	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	principal, err := provider.Authenticate(request)
	if err != nil || principal.TenantID != claims.Tenant || !principal.CanResource(claims.Scope.Resources[0].ID) {
		t.Fatalf("valid credential rejected: %#v %v", principal, err)
	}
	claims.Audience = "payroll"
	request.Header.Set("Authorization", "Bearer "+signedToken(t, keyID, private, claims))
	if _, err = provider.Authenticate(request); err == nil {
		t.Fatal("wrong audience accepted")
	}
	claims.Audience = "internal-finance-requests"
	claims.Scope.Resources[0].Type = "payroll_run"
	request.Header.Set("Authorization", "Bearer "+signedToken(t, keyID, private, claims))
	if _, err = provider.Authenticate(request); err == nil {
		t.Fatal("wrong resource type accepted")
	}
	request.Header.Set("Authorization", "Bearer "+token+"x")
	if _, err = provider.Authenticate(request); err == nil {
		t.Fatal("invalid signature accepted")
	}
}
func signedToken(t *testing.T, keyID string, private ed25519.PrivateKey, claims iamClaims) string {
	t.Helper()
	header, _ := json.Marshal(iamHeader{Algorithm: "EdDSA", Type: "JWT", KeyID: keyID})
	payload, _ := json.Marshal(claims)
	encodedHeader := base64.RawURLEncoding.EncodeToString(header)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	message := encodedHeader + "." + encodedPayload
	return message + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, []byte(message)))
}
