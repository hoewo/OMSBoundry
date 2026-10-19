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
