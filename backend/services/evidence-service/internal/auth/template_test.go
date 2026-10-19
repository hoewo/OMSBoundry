package auth

import (
	"net/http/httptest"
	"testing"
)

func TestTemplateProviderStillRequiresCredential(t *testing.T) {
	provider, err := NewTemplateProvider("33333333-3333-4333-8333-333333333333", "44444444-4444-4444-8444-444444444444", "Admin", "this-is-a-long-template-token")
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "/v1/upload-sessions", nil)
	if _, err := provider.Authenticate(request); err == nil {
		t.Fatal("anonymous request was accepted")
	}
	request.Header.Set("Authorization", "Bearer this-is-a-long-template-token")
	principal, err := provider.Authenticate(request)
	if err != nil {
		t.Fatal(err)
	}
	if !principal.Can(ActionView) || !principal.Can(ActionMetadataRead) || !principal.Can(ActionDownload) || !principal.Can(ActionUpload) {
		t.Fatalf("template principal lacks native capabilities: %+v", principal)
	}
}
