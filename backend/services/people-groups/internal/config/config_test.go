package config

import "testing"

func TestLoadRequiresExplicitTemplateCredential(t *testing.T) {
	t.Setenv("IDENTITY_PROVIDER", "template")
	t.Setenv("TEMPLATE_TENANT_ID", "33333333-3333-4333-8333-333333333333")
	t.Setenv("TEMPLATE_SUBJECT_ID", "44444444-4444-4444-8444-444444444444")
	t.Setenv("TEMPLATE_DISPLAY_NAME", "Test Admin")
	t.Setenv("TEMPLATE_ACCESS_TOKEN", "replace-with-at-least-24-random-bytes")
	if _, err := Load(); err == nil {
		t.Fatal("expected placeholder token to be rejected")
	}
	t.Setenv("TEMPLATE_ACCESS_TOKEN", "a-real-test-token-with-24-bytes")
	if _, err := Load(); err != nil {
		t.Fatalf("expected valid template config: %v", err)
	}
}

func TestLoadIAMRejectsTemplateSecret(t *testing.T) {
	t.Setenv("IDENTITY_PROVIDER", "iam")
	t.Setenv("IAM_ISSUER", "https://issuer.example/auth")
	t.Setenv("IAM_TENANT_ID", "33333333-3333-4333-8333-333333333333")
	t.Setenv("IAM_PUBLIC_KEYS_PATH", "/keys.json")
	t.Setenv("TEMPLATE_ACCESS_TOKEN", "must-not-coexist-with-iam")
	if _, err := Load(); err == nil {
		t.Fatal("expected mixed identity providers to be rejected")
	}
}

func TestLoadAcceptsCompleteIAMReferenceConfiguration(t *testing.T) {
	t.Setenv("IDENTITY_PROVIDER", "iam")
	t.Setenv("IAM_ISSUER", "https://issuer.example/auth")
	t.Setenv("IAM_TENANT_ID", "33333333-3333-4333-8333-333333333333")
	t.Setenv("IAM_PUBLIC_KEYS_PATH", "/keys.json")
	t.Setenv("IAM_BASE_URL", "http://identity-access:8080")
	t.Setenv("IAM_SERVICE_CREDENTIAL_URL", "http://identity-access:8080/api/v1/service-credentials")
	t.Setenv("WORKLOAD_ID", "people-groups")
	t.Setenv("WORKLOAD_SUBJECT_ID", "dddddddd-dddd-4ddd-8ddd-dddddddddddd")
	t.Setenv("WORKLOAD_KEY_FILE", "/keys/people-groups.json")
	t.Setenv("WORKLOAD_REFRESH_BEFORE", "15s")
	t.Setenv("REFERENCE_TIMEOUT", "3s")
	if _, err := Load(); err != nil {
		t.Fatalf("expected complete IAM configuration: %v", err)
	}
}
