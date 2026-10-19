package config

import "testing"

const (
	testTenant  = "11111111-1111-4111-8111-111111111111"
	testSubject = "22222222-2222-4222-8222-222222222222"
)

func TestLoadTemplateModeRequiresFourDistinctReferenceTokens(t *testing.T) {
	setBase(t)
	t.Setenv("IDENTITY_PROVIDER", "template")
	t.Setenv("TEMPLATE_TENANT_ID", testTenant)
	t.Setenv("TEMPLATE_SUBJECT_ID", testSubject)
	t.Setenv("TEMPLATE_DISPLAY_NAME", "C6 验证用户")
	t.Setenv("TEMPLATE_ACCESS_TOKEN", "cost-outsourcing-user-token-123")
	t.Setenv("PROJECT_ACCESS_TOKEN", "project-reference-token-12345")
	t.Setenv("PARTY_ACCESS_TOKEN", "party-reference-token-1234567")
	t.Setenv("CONTRACT_ACCESS_TOKEN", "contract-reference-token-1234")
	t.Setenv("EVIDENCE_ACCESS_TOKEN", "evidence-reference-token-1234")
	t.Setenv("MONEY_ACCESS_TOKEN", "money-suggestion-token-12345")
	cfg, err := Load()
	if err != nil || cfg.IdentityProvider != IdentityTemplate || cfg.PartyBaseURL != "http://party.test" {
		t.Fatalf("config=%#v error=%v", cfg, err)
	}
	t.Setenv("PARTY_ACCESS_TOKEN", "replace-with-token")
	if _, err = Load(); err == nil {
		t.Fatal("placeholder reference token accepted")
	}
}

func TestLoadIAMModeRequiresWorkloadExchangeAndRejectsStaticTokens(t *testing.T) {
	setBase(t)
	t.Setenv("IDENTITY_PROVIDER", "iam")
	t.Setenv("IAM_ISSUER", "https://iam.example.test")
	t.Setenv("IAM_TENANT_ID", testTenant)
	t.Setenv("IAM_PUBLIC_KEYS_PATH", "/run/iam/public-keys.json")
	t.Setenv("IAM_SERVICE_CREDENTIAL_URL", "http://identity-access:8080/api/v1/service-credentials")
	t.Setenv("WORKLOAD_ID", "cost-outsourcing")
	t.Setenv("WORKLOAD_SUBJECT_ID", testSubject)
	t.Setenv("WORKLOAD_KEY_FILE", "/run/workload/private-key.json")
	cfg, err := Load()
	if err != nil || cfg.IdentityProvider != IdentityIAM || cfg.WorkloadID != "cost-outsourcing" {
		t.Fatalf("config=%#v error=%v", cfg, err)
	}
	t.Setenv("PARTY_ACCESS_TOKEN", "static-party-token-1234567")
	if _, err = Load(); err == nil {
		t.Fatal("static reference token accepted in IAM mode")
	}
}

func setBase(t *testing.T) {
	t.Helper()
	for _, name := range []string{"IDENTITY_PROVIDER", "IAM_ISSUER", "IAM_TENANT_ID", "IAM_PUBLIC_KEYS_PATH", "TEMPLATE_TENANT_ID", "TEMPLATE_SUBJECT_ID", "TEMPLATE_DISPLAY_NAME", "TEMPLATE_ACCESS_TOKEN", "PROJECT_ACCESS_TOKEN", "PARTY_ACCESS_TOKEN", "CONTRACT_ACCESS_TOKEN", "EVIDENCE_ACCESS_TOKEN", "MONEY_ACCESS_TOKEN", "IAM_SERVICE_CREDENTIAL_URL", "WORKLOAD_ID", "WORKLOAD_SUBJECT_ID", "WORKLOAD_KEY_FILE", "SHUTDOWN_TIMEOUT", "REFERENCE_TIMEOUT", "WORKLOAD_REFRESH_BEFORE"} {
		t.Setenv(name, "")
	}
	t.Setenv("PROJECT_BASE_URL", "http://project.test")
	t.Setenv("PARTY_BASE_URL", "http://party.test")
	t.Setenv("CONTRACT_BASE_URL", "http://contract.test")
	t.Setenv("EVIDENCE_BASE_URL", "http://evidence.test")
	t.Setenv("MONEY_BASE_URL", "http://money.test")
}
