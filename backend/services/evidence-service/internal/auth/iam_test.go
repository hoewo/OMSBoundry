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

func TestIAMProviderAcceptsOnlyEvidenceAudienceAndCapabilities(t *testing.T) {
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
	claims := iamClaims{Version: "1", Issuer: "https://issuer.example/auth", Subject: subjectID, Tenant: tenantID, Audience: "evidence-service", Name: "Evidence User", Actions: []string{ActionView, ActionMetadataRead, ActionDownload, ActionUpload}, Scope: iamScope{Kind: "tenant"}, IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: now.Add(5 * time.Minute).Unix(), AuthTime: now.Add(-time.Minute).Unix(), ID: credentialID}
	request := httptest.NewRequest("POST", "/v1/upload-sessions", nil)
	request.Header.Set("Authorization", "Bearer "+signToken(t, keyID, claims, privateKey))
	principal, err := provider.Authenticate(request)
	if err != nil {
		t.Fatal(err)
	}
	if principal.SubjectID != subjectID || !principal.Can(ActionUpload) {
		t.Fatalf("unexpected principal: %+v", principal)
	}
	if !principal.CanFile("55555555-5555-4555-8555-555555555555") {
		t.Fatal("tenant-scoped principal cannot access tenant file")
	}

	fileID := "55555555-5555-4555-8555-555555555555"
	claims.Actions = []string{ActionView}
	claims.Scope = iamScope{Kind: "resources", Resources: []iamResource{{Type: "file", ID: fileID}}}
	request.Header.Set("Authorization", "Bearer "+signToken(t, keyID, claims, privateKey))
	principal, err = provider.Authenticate(request)
	if err != nil || !principal.Can(ActionView) || !principal.CanFile(fileID) || principal.CanFile("66666666-6666-4666-8666-666666666666") {
		t.Fatalf("unexpected resource-scoped principal=%+v error=%v", principal, err)
	}
	for _, action := range []string{ActionMetadataRead, ActionDownload} {
		claims.Actions = []string{action}
		request.Header.Set("Authorization", "Bearer "+signToken(t, keyID, claims, privateKey))
		principal, err = provider.Authenticate(request)
		if err != nil || !principal.Can(action) || !principal.CanFile(fileID) {
			t.Fatalf("resource-scoped %s principal=%+v error=%v", action, principal, err)
		}
	}
	claims.Actions = []string{ActionUpload}
	request.Header.Set("Authorization", "Bearer "+signToken(t, keyID, claims, privateKey))
	if _, err := provider.Authenticate(request); err == nil {
		t.Fatal("resource-scoped upload capability accepted")
	}
	claims.Actions = []string{ActionView}
	claims.Scope.Resources = []iamResource{{Type: "project", ID: fileID}}
	request.Header.Set("Authorization", "Bearer "+signToken(t, keyID, claims, privateKey))
	if _, err := provider.Authenticate(request); err == nil {
		t.Fatal("non-file resource scope accepted")
	}

	claims.Scope = iamScope{Kind: "tenant"}
	claims.Actions = []string{ActionUpload}
	request.Header.Set("Authorization", "Bearer "+signToken(t, keyID, claims, privateKey))
	principal, err = provider.Authenticate(request)
	if err != nil || !principal.Can(ActionUpload) || principal.Can(ActionView) {
		t.Fatalf("least-privilege upload principal=%+v error=%v", principal, err)
	}
	claims.Actions = []string{ActionView, ActionUpload}
	claims.Audience = "contract-ledger"
	request.Header.Set("Authorization", "Bearer "+signToken(t, keyID, claims, privateKey))
	if _, err := provider.Authenticate(request); err == nil {
		t.Fatal("wrong audience accepted")
	}
	claims.Audience = "evidence-service"
	claims.Actions = []string{ActionView, "evidence-service.delete"}
	request.Header.Set("Authorization", "Bearer "+signToken(t, keyID, claims, privateKey))
	if _, err := provider.Authenticate(request); err == nil {
		t.Fatal("unknown capability accepted")
	}
	claims.Actions = []string{ActionUpload, ActionUpload}
	request.Header.Set("Authorization", "Bearer "+signToken(t, keyID, claims, privateKey))
	if _, err := provider.Authenticate(request); err == nil {
		t.Fatal("duplicate capability accepted")
	}
	if _, err := NewIAMProvider("http://issuer.example/auth", tenantID, path); err == nil {
		t.Fatal("non-HTTPS issuer accepted")
	}
}

func signToken(t *testing.T, keyID string, claims iamClaims, key ed25519.PrivateKey) string {
	t.Helper()
	header, _ := json.Marshal(iamHeader{Algorithm: "EdDSA", Type: "JWT", KeyID: keyID})
	body, _ := json.Marshal(claims)
	left := base64.RawURLEncoding.EncodeToString(header)
	right := base64.RawURLEncoding.EncodeToString(body)
	signature := ed25519.Sign(key, []byte(left+"."+right))
	return left + "." + right + "." + base64.RawURLEncoding.EncodeToString(signature)
}
