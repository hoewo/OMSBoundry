package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
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

func TestTemplateAndIAMProduceEquivalentDesignContext(t *testing.T) {
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
	claims := iamClaims{Version: "1", Issuer: issuer, Subject: authSubject, Tenant: authTenant, Audience: "design-operations", Name: "正式管理员", Actions: AllActions, Scope: iamScope{Kind: "resources", Resources: []iamResource{{Type: "project", ID: authProject}}}, IssuedAt: now.Add(-time.Minute).Unix(), NotBefore: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(time.Minute).Unix(), AuthTime: now.Add(-time.Minute).Unix(), ID: authTokenID}
	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Authorization", "Bearer "+signedToken(t, privateKey, claims))
	principal, err := provider.Authenticate(request)
	if err != nil || !principal.Can(ActionRead) || !principal.CanResource("project", authProject) || principal.CanResource("project", authTokenID) {
		t.Fatalf("principal=%#v err=%v", principal, err)
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
		if !templatePrincipal.Can(action) || !principal.Can(action) {
			t.Fatalf("missing action %s", action)
		}
	}
	if templatePrincipal.TenantID != principal.TenantID || templatePrincipal.SubjectID != principal.SubjectID {
		t.Fatalf("contexts differ: template=%#v iam=%#v", templatePrincipal, principal)
	}
}

func TestProductionIAMRejectsLegacyAudienceCapabilityAndResource(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	keySet := iamKeySet{Keys: []iamJWK{{KTY: "OKP", CRV: "Ed25519", Use: "sig", ALG: "EdDSA", KeyID: authKey, X: base64.RawURLEncoding.EncodeToString(publicKey), Status: "active"}}}
	raw, _ := json.Marshal(keySet)
	keyPath := filepath.Join(t.TempDir(), "keys.json")
	if err := os.WriteFile(keyPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	issuer := "https://iam.example.test"
	provider, err := NewProductionIAMProvider(issuer, authTenant, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	claims := iamClaims{Version: "1", Issuer: issuer, Subject: authSubject, Tenant: authTenant, Audience: ProductionAudience, Name: "生产负责人", Actions: ProductionActions, Scope: iamScope{Kind: "resources", Resources: []iamResource{{Type: "project", ID: authProject}}}, IssuedAt: now.Add(-time.Minute).Unix(), NotBefore: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(time.Minute).Unix(), AuthTime: now.Add(-time.Minute).Unix(), ID: authTokenID}
	authenticate := func(value iamClaims) error {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.Header.Set("Authorization", "Bearer "+signedToken(t, privateKey, value))
		_, err := provider.Authenticate(request)
		return err
	}
	if err := authenticate(claims); err != nil {
		t.Fatalf("production claims rejected: %v", err)
	}
	claims.Audience = DesignAudience
	if err := authenticate(claims); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("legacy audience accepted: %v", err)
	}
	claims.Audience = ProductionAudience
	claims.Actions = []string{ActionRead}
	if err := authenticate(claims); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("legacy capability accepted: %v", err)
	}
	claims.Actions = []string{ProductionActionRead}
	claims.Scope.Resources = []iamResource{{Type: "design-work-item", ID: authProject}}
	if err := authenticate(claims); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("legacy resource type accepted: %v", err)
	}
}

func TestIAMRejectsWrongAudienceDuplicateCapabilityAndInsecureIssuer(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	keySet := iamKeySet{Keys: []iamJWK{{KTY: "OKP", CRV: "Ed25519", Use: "sig", ALG: "EdDSA", KeyID: authKey, X: base64.RawURLEncoding.EncodeToString(publicKey), Status: "active"}}}
	raw, _ := json.Marshal(keySet)
	keyPath := filepath.Join(t.TempDir(), "keys.json")
	_ = os.WriteFile(keyPath, raw, 0o600)
	if _, err := NewIAMProvider("http://iam.example.test", authTenant, keyPath); err == nil {
		t.Fatal("insecure issuer accepted")
	}
	provider, _ := NewIAMProvider("https://iam.example.test", authTenant, keyPath)
	now := time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC)
	provider.now = func() time.Time { return now }
	claims := iamClaims{Version: "1", Issuer: "https://iam.example.test", Subject: authSubject, Tenant: authTenant, Audience: "wrong", Name: "管理员", Actions: []string{ActionRead}, Scope: iamScope{Kind: "tenant"}, IssuedAt: now.Add(-time.Minute).Unix(), NotBefore: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(time.Minute).Unix(), AuthTime: now.Add(-time.Minute).Unix(), ID: authTokenID}
	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Authorization", "Bearer "+signedToken(t, privateKey, claims))
	if _, err := provider.Authenticate(request); err != ErrUnauthenticated {
		t.Fatalf("wrong audience=%v", err)
	}
	claims.Audience, claims.Actions = "design-operations", []string{ActionRead, ActionRead}
	request.Header.Set("Authorization", "Bearer "+signedToken(t, privateKey, claims))
	if _, err := provider.Authenticate(request); err != ErrUnauthenticated {
		t.Fatalf("duplicate action=%v", err)
	}
}

func signedToken(t *testing.T, privateKey ed25519.PrivateKey, claims iamClaims) string {
	t.Helper()
	headerJSON, _ := json.Marshal(iamHeader{Algorithm: "EdDSA", Type: "JWT", KeyID: authKey})
	claimsJSON, _ := json.Marshal(claims)
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, []byte(signingInput)))
}
