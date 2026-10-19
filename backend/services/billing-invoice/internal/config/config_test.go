package config

import "testing"

func TestLoadTemplateModeRequiresEveryTypedDependencyCredential(t *testing.T) {
	setTemplateEnvironment(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddress != ":8091" || len(cfg.AllowedCurrencies) != 1 || cfg.AllowedCurrencies[0] != "CNY" || cfg.PartyBaseURL != "http://party:8090" {
		t.Fatalf("config=%#v", cfg)
	}
	t.Setenv("MONEY_ACCESS_TOKEN", "")
	if _, err = Load(); err == nil {
		t.Fatal("missing typed dependency token was accepted")
	}
}

func TestLoadIAMModeRejectsStaticReferenceCredentials(t *testing.T) {
	setTemplateEnvironment(t)
	t.Setenv("IDENTITY_PROVIDER", "iam")
	t.Setenv("IAM_ISSUER", "https://iam.example.test")
	t.Setenv("IAM_TENANT_ID", "11111111-1111-4111-8111-111111111111")
	t.Setenv("IAM_PUBLIC_KEYS_PATH", "/run/secrets/keys.json")
	t.Setenv("IAM_SERVICE_CREDENTIAL_URL", "http://identity-access:8087/api/v1/service-credentials/exchange")
	t.Setenv("WORKLOAD_ID", "billing-invoice")
	t.Setenv("WORKLOAD_SUBJECT_ID", "33333333-3333-4333-8333-333333333333")
	t.Setenv("WORKLOAD_KEY_FILE", "/run/secrets/workload.json")
	if _, err := Load(); err == nil {
		t.Fatal("IAM mode accepted template and static credentials")
	}
	for _, name := range []string{"TEMPLATE_TENANT_ID", "TEMPLATE_SUBJECT_ID", "TEMPLATE_DISPLAY_NAME", "TEMPLATE_ACCESS_TOKEN", "PARTY_ACCESS_TOKEN", "PROJECT_ACCESS_TOKEN", "CONTRACT_ACCESS_TOKEN", "MONEY_ACCESS_TOKEN", "EVIDENCE_ACCESS_TOKEN"} {
		t.Setenv(name, "")
	}
	cfg, err := Load()
	if err != nil || cfg.IdentityProvider != IdentityIAM {
		t.Fatalf("IAM config=%#v error=%v", cfg, err)
	}
}

func setTemplateEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{"HTTP_ADDR", "DATABASE_URL", "ALLOWED_CURRENCIES", "MIGRATION_MODE", "REFERENCE_TIMEOUT", "SHUTDOWN_TIMEOUT", "WORKLOAD_REFRESH_BEFORE", "IAM_ISSUER", "IAM_TENANT_ID", "IAM_PUBLIC_KEYS_PATH", "IAM_SERVICE_CREDENTIAL_URL", "WORKLOAD_ID", "WORKLOAD_SUBJECT_ID", "WORKLOAD_KEY_FILE"} {
		t.Setenv(name, "")
	}
	t.Setenv("IDENTITY_PROVIDER", "template")
	t.Setenv("TEMPLATE_TENANT_ID", "11111111-1111-4111-8111-111111111111")
	t.Setenv("TEMPLATE_SUBJECT_ID", "22222222-2222-4222-8222-222222222222")
	t.Setenv("TEMPLATE_DISPLAY_NAME", "本地管理员")
	t.Setenv("TEMPLATE_ACCESS_TOKEN", "local-billing-token-1234567890")
	for name, value := range map[string]string{
		"PARTY_BASE_URL": "http://party:8090", "PROJECT_BASE_URL": "http://project:8085", "CONTRACT_BASE_URL": "http://contract:8081", "MONEY_BASE_URL": "http://money:8083", "EVIDENCE_BASE_URL": "http://evidence:8084",
		"PARTY_ACCESS_TOKEN": "party-reference-token-1234567890", "PROJECT_ACCESS_TOKEN": "project-reference-token-1234567890", "CONTRACT_ACCESS_TOKEN": "contract-reference-token-1234567890", "MONEY_ACCESS_TOKEN": "money-reference-token-1234567890", "EVIDENCE_ACCESS_TOKEN": "evidence-reference-token-1234567890",
	} {
		t.Setenv(name, value)
	}
}
