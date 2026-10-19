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
)

func TestTemplateProviderRequiresTokenAndProducesTenantCredential(t *testing.T) {
	provider, err := NewTemplateProvider(authTenant, authSubject, "本地管理员", "this-is-a-long-template-token")
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "/api/v1/projects/"+authProject, nil)
	if _, err := provider.Authenticate(request); err != ErrUnauthenticated {
		t.Fatalf("missing token error = %v", err)
	}
	request.Header.Set("Authorization", "Bearer this-is-a-long-template-token")
	principal, err := provider.Authenticate(request)
	if err != nil {
		t.Fatal(err)
	}
	if principal.TenantID != authTenant || !principal.Can(ActionCreate) || !principal.CanProject(authProject) || principal.Provider != "template" {
		t.Fatalf("principal = %#v", principal)
	}
}

func TestIAMAndTemplateProvidersProduceEquivalentCapabilityContext(t *testing.T) {
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
	issuer := "https://iam.example.test"
	provider, err := NewIAMProvider(issuer, authTenant, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC)
	provider.now = func() time.Time { return now }
	claims := iamClaims{
		Version: "1", Issuer: issuer, Subject: authSubject, Tenant: authTenant, Audience: "project-workspace", Name: "正式管理员",
		Actions: AllActions, Scope: iamScope{Kind: "resources", Resources: []iamResource{{Type: "project", ID: authProject}}},
		IssuedAt: now.Add(-time.Minute).Unix(), NotBefore: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(time.Minute).Unix(), AuthTime: now.Add(-time.Minute).Unix(), ID: authTokenID,
	}
	token := signedToken(t, privateKey, claims)
	request := httptest.NewRequest("GET", "/api/v1/projects/"+authProject, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	principal, err := provider.Authenticate(request)
	if err != nil {
		t.Fatal(err)
	}
	if principal.TenantID != authTenant || principal.SubjectID != authSubject || !principal.Can(ActionRead) || !principal.CanProject(authProject) || principal.CanProject(authTokenID) || principal.Provider != "iam" {
		t.Fatalf("principal = %#v", principal)
	}
	template, err := NewTemplateProvider(authTenant, authSubject, "本地管理员", "this-is-a-long-template-token")
	if err != nil {
		t.Fatal(err)
	}
	templateRequest := httptest.NewRequest("GET", "/", nil)
	templateRequest.Header.Set("Authorization", "Bearer this-is-a-long-template-token")
	templatePrincipal, err := template.Authenticate(templateRequest)
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range AllActions {
		if templatePrincipal.Can(action) != principal.Can(action) {
			t.Fatalf("action %s differs between providers", action)
		}
	}
	if templatePrincipal.TenantID != principal.TenantID || templatePrincipal.SubjectID != principal.SubjectID {
		t.Fatalf("identity context differs: template=%#v iam=%#v", templatePrincipal, principal)
	}
}

func TestIAMProviderRejectsWrongAudienceTenantAndExpiredCredential(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	keySet := iamKeySet{Keys: []iamJWK{{KTY: "OKP", CRV: "Ed25519", Use: "sig", ALG: "EdDSA", KeyID: authKey, X: base64.RawURLEncoding.EncodeToString(publicKey), Status: "active"}}}
	raw, _ := json.Marshal(keySet)
	keyPath := filepath.Join(t.TempDir(), "keys.json")
	_ = os.WriteFile(keyPath, raw, 0o600)
	provider, _ := NewIAMProvider("https://iam.example.test", authTenant, keyPath)
	now := time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC)
	provider.now = func() time.Time { return now }
	valid := iamClaims{Version: "1", Issuer: "https://iam.example.test", Subject: authSubject, Tenant: authTenant, Audience: "project-workspace", Name: "管理员", Actions: []string{ActionRead}, Scope: iamScope{Kind: "tenant"}, IssuedAt: now.Add(-time.Minute).Unix(), NotBefore: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(time.Minute).Unix(), AuthTime: now.Add(-time.Minute).Unix(), ID: authTokenID}
	for _, test := range []struct {
		name   string
		mutate func(*iamClaims)
	}{
		{"wrong audience", func(value *iamClaims) { value.Audience = "wrong" }},
		{"wrong tenant", func(value *iamClaims) { value.Tenant = "66666666-6666-4666-8666-666666666666" }},
		{"expired", func(value *iamClaims) { value.ExpiresAt = now.Add(-time.Minute).Unix() }},
	} {
		t.Run(test.name, func(t *testing.T) {
			claims := valid
			test.mutate(&claims)
			request := httptest.NewRequest("GET", "/", nil)
			request.Header.Set("Authorization", "Bearer "+signedToken(t, privateKey, claims))
			if _, err := provider.Authenticate(request); err != ErrUnauthenticated {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func signedToken(t *testing.T, privateKey ed25519.PrivateKey, claims iamClaims) string {
	t.Helper()
	header := iamHeader{Algorithm: "EdDSA", Type: "JWT", KeyID: authKey}
	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	signature := ed25519.Sign(privateKey, []byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}
