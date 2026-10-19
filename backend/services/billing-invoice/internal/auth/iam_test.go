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
	authTenant  = "11111111-1111-4111-8111-111111111111"
	authSubject = "22222222-2222-4222-8222-222222222222"
	authKey     = "33333333-3333-4333-8333-333333333333"
	authTokenID = "44444444-4444-4444-8444-444444444444"
	authProject = "55555555-5555-4555-8555-555555555555"
	authInvoice = "66666666-6666-4666-8666-666666666666"
)

func TestIAMAcceptsOnlyBillingNativeActionsAndScope(t *testing.T) {
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
	now := time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC)
	provider.now = func() time.Time { return now }
	claims := iamClaims{
		Version: "1", Issuer: "https://iam.example.test", Subject: authSubject, Tenant: authTenant,
		Audience: "billing-invoice", Name: "财务查看者", Actions: []string{ActionRead, ActionAmountRead},
		Scope:    iamScope{Kind: "resources", Resources: []iamResource{{Type: "project", ID: authProject}, {Type: "invoice_registration", ID: authInvoice}}},
		IssuedAt: now.Add(-time.Minute).Unix(), NotBefore: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(time.Minute).Unix(), AuthTime: now.Add(-time.Minute).Unix(), ID: authTokenID,
	}
	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Authorization", "Bearer "+signToken(t, privateKey, claims))
	principal, err := provider.Authenticate(request)
	if err != nil || !principal.Can(ActionAmountRead) || principal.Can(ActionCorrect) || !principal.CanRegistration(authInvoice, []string{authProject}) {
		t.Fatalf("principal=%#v err=%v", principal, err)
	}

	tests := []struct {
		name   string
		mutate func(*iamClaims)
	}{
		{"wrong audience", func(value *iamClaims) { value.Audience = "gateway" }},
		{"wrong tenant", func(value *iamClaims) { value.Tenant = "77777777-7777-4777-8777-777777777777" }},
		{"expired", func(value *iamClaims) { value.ExpiresAt = now.Add(-time.Minute).Unix() }},
		{"broad legacy action", func(value *iamClaims) { value.Actions = []string{"finance.manage"} }},
		{"duplicate action", func(value *iamClaims) { value.Actions = []string{ActionRead, ActionRead} }},
		{"foreign resource", func(value *iamClaims) { value.Scope.Resources = []iamResource{{Type: "contract", ID: authInvoice}} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := claims
			candidate.Actions = append([]string{}, claims.Actions...)
			candidate.Scope.Resources = append([]iamResource{}, claims.Scope.Resources...)
			test.mutate(&candidate)
			request.Header.Set("Authorization", "Bearer "+signToken(t, privateKey, candidate))
			if _, err := provider.Authenticate(request); err != ErrUnauthenticated {
				t.Fatalf("credential accepted: %v", err)
			}
		})
	}
}

func TestTemplateHumanCredentialDoesNotReceiveWorkloadReferenceAction(t *testing.T) {
	provider, err := NewTemplateProvider(authTenant, authSubject, "本地财务", "billing-template-token-1234567890")
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Authorization", "Bearer billing-template-token-1234567890")
	principal, err := provider.Authenticate(request)
	if err != nil || !principal.Can(ActionRead) || principal.Can(ActionReferenceRead) {
		t.Fatalf("principal=%#v err=%v", principal, err)
	}
}

func signToken(t *testing.T, privateKey ed25519.PrivateKey, claims iamClaims) string {
	t.Helper()
	headerJSON, _ := json.Marshal(iamHeader{Algorithm: "EdDSA", Type: "JWT", KeyID: authKey})
	claimsJSON, _ := json.Marshal(claims)
	unsigned := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, []byte(unsigned)))
}
