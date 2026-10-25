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
	authProject = "33333333-3333-4333-8333-333333333333"
	authKeyID   = "44444444-4444-4444-8444-444444444444"
	authTokenID = "55555555-5555-4555-8555-555555555555"
)

func TestIAMProviderAcceptsNativeAudienceCapabilityAndExactScope(t *testing.T) {
	provider, privateKey := testProvider(t)
	token := testToken(t, privateKey, "approval-audit-results", authTenant, []string{ActionRead, ActionAmountRead}, "project", authProject)
	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	principal, err := provider.Authenticate(request)
	if err != nil {
		t.Fatal(err)
	}
	if principal.TenantID != authTenant || principal.SubjectID != authSubject || !principal.Can(ActionRead) || !principal.Can(ActionAmountRead) || !principal.CanResource("project", authProject) || principal.CanResource("project", authTokenID) {
		t.Fatalf("principal=%#v", principal)
	}
}

func TestIAMProviderAcceptsAnySignedTenantAndRejectsMalformedTenant(t *testing.T) {
	provider, privateKey := testProvider(t)
	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Authorization", "Bearer "+testToken(t, privateKey, "approval-audit-results", authSubject, []string{ActionRead}, "project", authProject))
	principal, err := provider.Authenticate(request)
	if err != nil || principal.TenantID != authSubject {
		t.Fatalf("other tenant principal=%#v err=%v", principal, err)
	}
	for _, test := range []struct{ name, audience, tenant, resourceType string }{
		{"audience", "billing-invoice", authTenant, "project"}, {"malformed tenant", "approval-audit-results", "not-a-tenant", "project"}, {"resource", "approval-audit-results", authTenant, "contract"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest("GET", "/", nil)
			request.Header.Set("Authorization", "Bearer "+testToken(t, privateKey, test.audience, test.tenant, []string{ActionRead}, test.resourceType, authProject))
			if _, err := provider.Authenticate(request); err == nil {
				t.Fatal("credential accepted")
			}
		})
	}
}

func testProvider(t *testing.T) (*IAMProvider, ed25519.PrivateKey) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"keys": []map[string]any{{"kty": "OKP", "crv": "Ed25519", "use": "sig", "alg": "EdDSA", "kid": authKeyID, "x": base64.RawURLEncoding.EncodeToString(publicKey), "status": "active"}}})
	path := filepath.Join(t.TempDir(), "keys.json")
	if err = os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := NewIAMProvider("https://iam.results.test", authTenant, path)
	if err != nil {
		t.Fatal(err)
	}
	return provider, privateKey
}

func testToken(t *testing.T, privateKey ed25519.PrivateKey, audience, tenant string, actions []string, resourceType, resourceID string) string {
	t.Helper()
	now := time.Now().UTC()
	header, _ := json.Marshal(map[string]any{"alg": "EdDSA", "typ": "JWT", "kid": authKeyID})
	claims, _ := json.Marshal(map[string]any{"ver": "1", "iss": "https://iam.results.test", "sub": authSubject, "tid": tenant, "aud": audience, "name": "结果登记员", "actions": actions, "scope": map[string]any{"kind": "resources", "resources": []map[string]string{{"type": resourceType, "id": resourceID}}}, "iat": now.Add(-time.Second).Unix(), "nbf": now.Add(-time.Second).Unix(), "exp": now.Add(time.Minute).Unix(), "auth_time": now.Add(-time.Second).Unix(), "jti": authTokenID})
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, []byte(unsigned)))
}
