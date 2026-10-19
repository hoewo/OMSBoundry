package config

import "testing"

func TestReferenceBonusProfileIsExplicitAndDoesNotReplaceIdentityValidation(t *testing.T) {
	for _, key := range []string{"IAM_ISSUER", "IAM_TENANT_ID", "IAM_PUBLIC_KEYS_PATH", "IAM_SERVICE_CREDENTIAL_URL", "WORKLOAD_ID", "WORKLOAD_SUBJECT_ID", "WORKLOAD_KEY_FILE", "MONEY_BASE_URL", "MONEY_ACCESS_TOKEN"} {
		t.Setenv(key, "")
	}
	t.Setenv("IDENTITY_PROVIDER", "template")
	t.Setenv("TEMPLATE_TENANT_ID", "11111111-1111-4111-8111-111111111111")
	t.Setenv("TEMPLATE_SUBJECT_ID", "22222222-2222-4222-8222-222222222222")
	t.Setenv("TEMPLATE_DISPLAY_NAME", "local test provider")
	for _, key := range []string{"TEMPLATE_ACCESS_TOKEN", "PROJECT_ACCESS_TOKEN", "PEOPLE_ACCESS_TOKEN", "CONTRACT_ACCESS_TOKEN"} {
		t.Setenv(key, "explicit-test-only-secret-123456789")
	}
	for _, key := range []string{"PROJECT_BASE_URL", "PEOPLE_BASE_URL", "CONTRACT_BASE_URL"} {
		t.Setenv(key, "http://configured-source.example.test")
	}
	for _, profile := range []string{"", "reference-oa-bonus-v2", "reference-oa-bonus-v3"} {
		t.Setenv("REFERENCE_OA_BONUS_PROFILE", profile)
		cfg, e := Load()
		if e != nil || cfg.ReferenceOABonusProfile != profile {
			t.Fatalf("profile %q: %v", profile, e)
		}
	}
	t.Setenv("REFERENCE_OA_BONUS_PROFILE", "reference-oa-bonus-v1")
	if _, e := Load(); e == nil {
		t.Fatal("unknown manual profile accepted")
	}
	t.Setenv("REFERENCE_OA_BONUS_PROFILE", "reference-oa-bonus-v2")
	t.Setenv("TEMPLATE_ACCESS_TOKEN", "")
	if _, e := Load(); e == nil {
		t.Fatal("selected overlay bypassed identity configuration")
	}
}
