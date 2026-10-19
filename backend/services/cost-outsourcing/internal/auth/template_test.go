package auth

import (
	"net/http/httptest"
	"testing"
)

func TestTemplateCredentialHasStableTrustedUUIDWithoutBypassingBearer(t *testing.T) {
	const tenant = "11111111-1111-4111-8111-111111111111"
	const subject = "22222222-2222-4222-8222-222222222222"
	const token = "explicit-local-test-provider-token"
	provider, err := NewTemplateProvider(tenant, subject, "Fixture", token)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "/", nil)
	if _, err = provider.Authenticate(request); err == nil {
		t.Fatal("anonymous accepted")
	}
	request.Header.Set("Authorization", "Bearer "+token)
	a, err := provider.Authenticate(request)
	if err != nil {
		t.Fatal(err)
	}
	b, err := provider.Authenticate(request)
	if err != nil {
		t.Fatal(err)
	}
	if !uuidPattern.MatchString(a.CredentialID) || a.CredentialID != b.CredentialID || a.Provider != "template" || a.TenantID != tenant || a.SubjectID != subject {
		t.Fatal("invalid trusted local actor")
	}
	second, err := NewTemplateProvider(tenant, subject, "Fixture", token)
	if err != nil {
		t.Fatal(err)
	}
	c, err := second.Authenticate(request)
	if err != nil {
		t.Fatal(err)
	}
	if c.CredentialID == a.CredentialID {
		t.Fatal("separate provider reused credential identity")
	}
	request.Header.Set("Authorization", "Bearer invalid-replaced-token")
	if _, err = provider.Authenticate(request); err == nil {
		t.Fatal("wrong bearer accepted")
	}
}
