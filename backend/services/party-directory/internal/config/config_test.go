package config

import "testing"

func TestTemplateConfigurationRequiresReplacedSecrets(t *testing.T) {
	t.Setenv("CURSOR_HMAC_KEY", "12345678901234567890123456789012")
	t.Setenv("IDENTITY_PROVIDER", "template")
	t.Setenv("TEMPLATE_TENANT_ID", "11111111-1111-4111-8111-111111111111")
	t.Setenv("TEMPLATE_SUBJECT_ID", "22222222-2222-4222-8222-222222222222")
	t.Setenv("TEMPLATE_DISPLAY_NAME", "管理员")
	t.Setenv("TEMPLATE_ACCESS_TOKEN", "123456789012345678901234")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CURSOR_HMAC_KEY", "replace-with-a-random-secret-at-least-32-bytes")
	if _, err := Load(); err == nil {
		t.Fatal("placeholder cursor secret must fail")
	}
}

func TestIAMAndTemplateSettingsCannotMix(t *testing.T) {
	t.Setenv("CURSOR_HMAC_KEY", "12345678901234567890123456789012")
	t.Setenv("IDENTITY_PROVIDER", "iam")
	t.Setenv("IAM_ISSUER", "https://iam.example.test")
	t.Setenv("IAM_TENANT_ID", "11111111-1111-4111-8111-111111111111")
	t.Setenv("IAM_PUBLIC_KEYS_PATH", "/tmp/keys.json")
	t.Setenv("TEMPLATE_TENANT_ID", "11111111-1111-4111-8111-111111111111")
	if _, err := Load(); err == nil {
		t.Fatal("mixed provider configuration must fail")
	}
}
