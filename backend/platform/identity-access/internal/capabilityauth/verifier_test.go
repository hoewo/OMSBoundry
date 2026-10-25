package capabilityauth

import (
	"errors"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"omsbizfoundry/identity-access/internal/identity"
	"omsbizfoundry/identity-access/internal/signing"
)

const (
	testTenant   = "11111111-1111-4111-8111-111111111111"
	testSubject  = "22222222-2222-4222-8222-222222222222"
	testResource = "33333333-3333-4333-8333-333333333333"
)

func TestVerifierAcceptsExactSubjectReferenceCredential(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	keys, err := signing.Open(filepath.Join(t.TempDir(), "keys.json"), now)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := New("https://iam.example.test", keys)
	if err != nil {
		t.Fatal(err)
	}
	verifier.now = func() time.Time { return now }
	claims := identity.CapabilityClaims{
		Version: "1", Issuer: "https://iam.example.test", Subject: testSubject, Tenant: testTenant,
		Audience: "identity-access", Name: "People & Groups", Actions: []string{"identity-access.subject.reference.read"},
		Scope:    identity.Scope{Kind: "resources", Resources: []identity.ResourceRef{{Type: "iam_subject", ID: testResource}}},
		IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix(), AuthTime: now.Unix(), ID: "44444444-4444-4444-8444-444444444444",
	}
	token, err := keys.Sign(claims)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	got, err := verifier.Authenticate(request, "identity-access", "identity-access.subject.reference.read", "iam_subject", testResource)
	if err != nil || got.Tenant != testTenant {
		t.Fatalf("claims=%#v err=%v", got, err)
	}
	if _, err := verifier.Authenticate(request, "identity-access", "identity-access.subject.reference.read", "iam_subject", testSubject); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("neighbor resource error=%v", err)
	}
}
