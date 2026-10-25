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

const (
	authTenant      = "11111111-1111-4111-8111-111111111111"
	authOtherTenant = "77777777-7777-4777-8777-777777777777"
	authSubject     = "22222222-2222-4222-8222-222222222222"
	authKey         = "33333333-3333-4333-8333-333333333333"
	authTokenID     = "44444444-4444-4444-8444-444444444444"
	authProject     = "55555555-5555-4555-8555-555555555555"
	authEngagement  = "66666666-6666-4666-8666-666666666666"
)

func TestIAMAcceptsOnlyNativeActionsAndBoundScope(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keySet := iamKeySet{Keys: []iamJWK{{KTY: "OKP", CRV: "Ed25519", Use: "sig", ALG: "EdDSA", KeyID: authKey, X: base64.RawURLEncoding.EncodeToString(publicKey), Status: "active"}}}
	raw, _ := json.Marshal(keySet)
	keyPath := filepath.Join(t.TempDir(), "keys.json")
	if err := os.WriteFile(keyPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := NewIAMProvider("https://iam.example.test", authTenant, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 27, 8, 0, 0, 0, time.UTC)
	provider.now = func() time.Time { return now }
	claims := iamClaims{
		Version: "1", Issuer: "https://iam.example.test", Subject: authSubject, Tenant: authTenant,
		Audience: "business-engagement", Name: "经营管理员",
		Actions:  []string{ActionRead, ActionMethodSelect},
		Scope:    iamScope{Kind: "resources", Resources: []iamResource{{Type: "project", ID: authProject}, {Type: "business-engagement", ID: authEngagement}}},
		IssuedAt: now.Add(-time.Minute).Unix(), NotBefore: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(time.Minute).Unix(), AuthTime: now.Add(-time.Minute).Unix(), ID: authTokenID,
	}
	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Authorization", "Bearer "+signedToken(t, privateKey, claims))
	principal, err := provider.Authenticate(request)
	if err != nil || !principal.Can(ActionMethodSelect) || principal.Can(ActionCancel) || !principal.CanEngagement(authEngagement, authProject) {
		t.Fatalf("principal=%#v err=%v", principal, err)
	}

	claims.Actions = []string{"manage_business_info"}
	request.Header.Set("Authorization", "Bearer "+signedToken(t, privateKey, claims))
	if _, err := provider.Authenticate(request); err != ErrUnauthenticated {
		t.Fatalf("legacy broad capability accepted: %v", err)
	}
	claims.Actions = []string{ActionRead}
	claims.Audience = "gateway"
	request.Header.Set("Authorization", "Bearer "+signedToken(t, privateKey, claims))
	if _, err := provider.Authenticate(request); err != ErrUnauthenticated {
		t.Fatalf("wrong audience accepted: %v", err)
	}
	claims.Audience = "business-engagement"
	claims.Tenant = authOtherTenant
	request.Header.Set("Authorization", "Bearer "+signedToken(t, privateKey, claims))
	otherTenantPrincipal, err := provider.Authenticate(request)
	if err != nil || otherTenantPrincipal.TenantID != authOtherTenant {
		t.Fatalf("other tenant principal=%#v err=%v", otherTenantPrincipal, err)
	}
	claims.Tenant = "not-a-tenant"
	request.Header.Set("Authorization", "Bearer "+signedToken(t, privateKey, claims))
	if _, err := provider.Authenticate(request); err != ErrUnauthenticated {
		t.Fatalf("malformed tenant accepted: %v", err)
	}
}

func signedToken(t *testing.T, privateKey ed25519.PrivateKey, claims iamClaims) string {
	t.Helper()
	headerJSON, _ := json.Marshal(iamHeader{Algorithm: "EdDSA", Type: "JWT", KeyID: authKey})
	claimsJSON, _ := json.Marshal(claims)
	unsigned := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, []byte(unsigned)))
}
