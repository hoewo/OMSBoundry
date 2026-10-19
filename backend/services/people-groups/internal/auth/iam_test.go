package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIAMProviderAcceptsOnlyPeopleGroupsCredential(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyID := "11111111-1111-4111-8111-111111111111"
	tenantID := "22222222-2222-4222-8222-222222222222"
	subjectID := "33333333-3333-4333-8333-333333333333"
	credentialID := "44444444-4444-4444-8444-444444444444"
	path := filepath.Join(t.TempDir(), "keys.json")
	keys := map[string]any{"keys": []map[string]string{{"kty": "OKP", "crv": "Ed25519", "use": "sig", "alg": "EdDSA", "kid": keyID, "x": base64.RawURLEncoding.EncodeToString(publicKey), "status": "active"}}}
	data, _ := json.Marshal(keys)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := NewIAMProvider("https://issuer.example/auth", tenantID, path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	provider.now = func() time.Time { return now }
	claims := iamClaims{Version: "1", Issuer: "https://issuer.example/auth", Subject: subjectID, Tenant: tenantID, Audience: "people-groups", Name: "测试用户", Actions: []string{ActionView, ActionMembershipManage}, Scope: iamScope{Kind: "tenant", Resources: []struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}{}}, IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: now.Add(5 * time.Minute).Unix(), AuthTime: now.Add(-time.Minute).Unix(), ID: credentialID}
	token := signTestToken(t, keyID, claims, privateKey)
	request := httptest.NewRequest("GET", "/api/v1/groups", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	principal, err := provider.Authenticate(request)
	if err != nil {
		t.Fatal(err)
	}
	if principal.SubjectID != subjectID || !principal.Can(ActionMembershipManage) {
		t.Fatalf("unexpected principal: %+v", principal)
	}
	claims.Audience = "contract-ledger"
	request.Header.Set("Authorization", "Bearer "+signTestToken(t, keyID, claims, privateKey))
	if _, err := provider.Authenticate(request); err == nil {
		t.Fatal("wrong audience was accepted")
	}
	claims.Audience = "people-groups"
	claims.Actions = []string{ActionMembershipManage}
	request.Header.Set("Authorization", "Bearer "+signTestToken(t, keyID, claims, privateKey))
	if _, err := provider.Authenticate(request); err == nil {
		t.Fatal("credential without view was accepted")
	}
}

func TestIAMProviderAcceptsOnlyExactPersonReferenceCredential(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyID := "11111111-1111-4111-8111-111111111111"
	tenantID := "22222222-2222-4222-8222-222222222222"
	personID := "33333333-3333-4333-8333-333333333333"
	path := filepath.Join(t.TempDir(), "keys.json")
	data, _ := json.Marshal(map[string]any{"keys": []map[string]string{{"kty": "OKP", "crv": "Ed25519", "use": "sig", "alg": "EdDSA", "kid": keyID, "x": base64.RawURLEncoding.EncodeToString(publicKey), "status": "active"}}})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := NewIAMProvider("https://issuer.example/auth", tenantID, path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	provider.now = func() time.Time { return now }
	scope := iamScope{Kind: "resources", Resources: []struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}{{Type: "person", ID: personID}}}
	claims := iamClaims{Version: "1", Issuer: "https://issuer.example/auth", Subject: personID, Tenant: tenantID, Audience: "people-groups", Name: "Bonus Distribution", Actions: []string{ActionPersonReferenceRead}, Scope: scope, IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: now.Add(5 * time.Minute).Unix(), AuthTime: now.Unix(), ID: "44444444-4444-4444-8444-444444444444"}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/references/persons/"+personID, nil)
	request.Header.Set("Authorization", "Bearer "+signTestToken(t, keyID, claims, privateKey))
	principal, err := provider.Authenticate(request)
	if err != nil || !principal.Can(ActionPersonReferenceRead) || !principal.CanResource("person", personID) {
		t.Fatalf("principal=%+v err=%v", principal, err)
	}
	claims.Actions = []string{ActionView}
	request.Header.Set("Authorization", "Bearer "+signTestToken(t, keyID, claims, privateKey))
	adjacent, err := provider.Authenticate(request)
	if err != nil || adjacent.Can(ActionPersonReferenceRead) {
		t.Fatalf("adjacent view credential principal=%+v err=%v", adjacent, err)
	}
}

func signTestToken(t *testing.T, keyID string, claims iamClaims, key ed25519.PrivateKey) string {
	t.Helper()
	header, _ := json.Marshal(iamHeader{Algorithm: "EdDSA", Type: "JWT", KeyID: keyID})
	body, _ := json.Marshal(claims)
	left := base64.RawURLEncoding.EncodeToString(header)
	right := base64.RawURLEncoding.EncodeToString(body)
	signature := ed25519.Sign(key, []byte(left+"."+right))
	return left + "." + right + "." + base64.RawURLEncoding.EncodeToString(signature)
}
