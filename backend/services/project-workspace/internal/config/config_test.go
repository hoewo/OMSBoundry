package config

import "testing"

func TestLoadTemplateConfiguration(t *testing.T) {
	clearConfiguration(t)
	t.Setenv("TEMPLATE_TENANT_ID", "11111111-1111-4111-8111-111111111111")
	t.Setenv("TEMPLATE_SUBJECT_ID", "22222222-2222-4222-8222-222222222222")
	t.Setenv("TEMPLATE_DISPLAY_NAME", "本地管理员")
	t.Setenv("TEMPLATE_ACCESS_TOKEN", "this-is-a-replaced-template-token")
	t.Setenv("PEOPLE_BASE_URL", "http://127.0.0.1:8084")
	t.Setenv("PEOPLE_ACCESS_TOKEN", "people-reference-token")
	t.Setenv("PARTY_BASE_URL", "http://127.0.0.1:8089")
	t.Setenv("PARTY_ACCESS_TOKEN", "party-reference-token")
	t.Setenv("BUSINESS_ENGAGEMENT_BASE_URL", "http://127.0.0.1:8091")
	t.Setenv("BUSINESS_ENGAGEMENT_ACCESS_TOKEN", "business-engagement-reference-token")
	value, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if value.IdentityProvider != IdentityTemplate || value.DatabaseURL == "" || value.PeopleBaseURL != "http://127.0.0.1:8084" || value.PartyBaseURL != "http://127.0.0.1:8089" || value.BusinessEngagementBaseURL != "http://127.0.0.1:8091" {
		t.Fatalf("config=%#v", value)
	}
}

func TestLoadRejectsAuthenticationAndEndpointHalfConfiguration(t *testing.T) {
	clearConfiguration(t)
	t.Setenv("TEMPLATE_TENANT_ID", "11111111-1111-4111-8111-111111111111")
	t.Setenv("TEMPLATE_SUBJECT_ID", "22222222-2222-4222-8222-222222222222")
	t.Setenv("TEMPLATE_DISPLAY_NAME", "本地管理员")
	t.Setenv("TEMPLATE_ACCESS_TOKEN", "replace-with-at-least-24-random-bytes")
	if _, err := Load(); err == nil {
		t.Fatal("placeholder template token was accepted")
	}
	t.Setenv("TEMPLATE_ACCESS_TOKEN", "this-is-a-replaced-template-token")
	t.Setenv("CONTRACT_BASE_URL", "http://contract.example")
	if _, err := Load(); err == nil {
		t.Fatal("endpoint without token was accepted")
	}
}

func TestLoadIAMRejectsTemplateCredentialMix(t *testing.T) {
	clearConfiguration(t)
	t.Setenv("IDENTITY_PROVIDER", "iam")
	t.Setenv("IAM_ISSUER", "https://iam.example.test")
	t.Setenv("IAM_TENANT_ID", "11111111-1111-4111-8111-111111111111")
	t.Setenv("IAM_PUBLIC_KEYS_PATH", "/tmp/keys.json")
	t.Setenv("TEMPLATE_ACCESS_TOKEN", "this-should-not-be-present")
	if _, err := Load(); err == nil {
		t.Fatal("mixed IAM/template configuration was accepted")
	}
}

func TestLoadIAMRequiresWorkloadIdentityForPeopleReferences(t *testing.T) {
	clearConfiguration(t)
	t.Setenv("IDENTITY_PROVIDER", "iam")
	t.Setenv("IAM_ISSUER", "https://iam.example.test")
	t.Setenv("IAM_TENANT_ID", "11111111-1111-4111-8111-111111111111")
	t.Setenv("IAM_PUBLIC_KEYS_PATH", "/tmp/keys.json")
	t.Setenv("PEOPLE_BASE_URL", "http://people:8084")
	if _, err := Load(); err == nil {
		t.Fatal("people reference without workload identity was accepted")
	}
	t.Setenv("IAM_SERVICE_CREDENTIAL_URL", "http://identity:8080/api/v1/service-credentials")
	t.Setenv("WORKLOAD_ID", "project-workspace")
	t.Setenv("WORKLOAD_SUBJECT_ID", "22222222-2222-4222-8222-222222222222")
	t.Setenv("WORKLOAD_KEY_FILE", "/private/project-workspace.json")
	value, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if value.PeopleAccessToken != "" || value.WorkloadID != "project-workspace" {
		t.Fatalf("config=%#v", value)
	}
}

func TestLoadIAMRequiresWorkloadIdentityForPartyReferences(t *testing.T) {
	clearConfiguration(t)
	t.Setenv("IDENTITY_PROVIDER", "iam")
	t.Setenv("IAM_ISSUER", "https://iam.example.test")
	t.Setenv("IAM_TENANT_ID", "11111111-1111-4111-8111-111111111111")
	t.Setenv("IAM_PUBLIC_KEYS_PATH", "/tmp/keys.json")
	t.Setenv("PARTY_BASE_URL", "http://party:8089")
	if _, err := Load(); err == nil {
		t.Fatal("party reference without workload identity was accepted")
	}
	t.Setenv("IAM_SERVICE_CREDENTIAL_URL", "http://identity:8080/api/v1/service-credentials")
	t.Setenv("WORKLOAD_ID", "project-workspace")
	t.Setenv("WORKLOAD_SUBJECT_ID", "22222222-2222-4222-8222-222222222222")
	t.Setenv("WORKLOAD_KEY_FILE", "/private/project-workspace.json")
	value, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if value.PartyAccessToken != "" || value.PartyBaseURL != "http://party:8089" {
		t.Fatalf("config=%#v", value)
	}
}

func TestLoadIAMUsesWorkloadIdentityForCommercialReferenceEndpoints(t *testing.T) {
	clearConfiguration(t)
	t.Setenv("IDENTITY_PROVIDER", "iam")
	t.Setenv("IAM_ISSUER", "https://iam.example.test")
	t.Setenv("IAM_TENANT_ID", "11111111-1111-4111-8111-111111111111")
	t.Setenv("IAM_PUBLIC_KEYS_PATH", "/tmp/keys.json")
	t.Setenv("CONTRACT_BASE_URL", "http://contract:8080")
	t.Setenv("MONEY_BASE_URL", "http://money:8080")
	t.Setenv("BILLING_BASE_URL", "http://billing:8080")
	if _, err := Load(); err == nil {
		t.Fatal("commercial endpoints without workload identity were accepted")
	}
	t.Setenv("IAM_SERVICE_CREDENTIAL_URL", "http://identity:8080/api/v1/service-credentials")
	t.Setenv("WORKLOAD_ID", "project-workspace")
	t.Setenv("WORKLOAD_SUBJECT_ID", "22222222-2222-4222-8222-222222222222")
	t.Setenv("WORKLOAD_KEY_FILE", "/private/project-workspace.json")
	value, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if value.ContractAccessToken != "" || value.MoneyAccessToken != "" || value.BillingAccessToken != "" {
		t.Fatalf("static service token leaked into IAM config: %#v", value)
	}
}

func clearConfiguration(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"HTTP_ADDR", "DATABASE_URL", "SHUTDOWN_TIMEOUT", "MIGRATION_MODE", "IDENTITY_PROVIDER", "REFERENCE_OA_PRODUCTION_PERSONNEL_PROFILE",
		"IAM_ISSUER", "IAM_TENANT_ID", "IAM_PUBLIC_KEYS_PATH", "TEMPLATE_TENANT_ID", "TEMPLATE_SUBJECT_ID", "TEMPLATE_DISPLAY_NAME", "TEMPLATE_ACCESS_TOKEN",
		"PEOPLE_BASE_URL", "PEOPLE_ACCESS_TOKEN", "PARTY_BASE_URL", "PARTY_ACCESS_TOKEN", "IAM_SERVICE_CREDENTIAL_URL", "WORKLOAD_ID", "WORKLOAD_SUBJECT_ID", "WORKLOAD_KEY_FILE", "WORKLOAD_REFRESH_BEFORE", "CONTRACT_BASE_URL", "CONTRACT_ACCESS_TOKEN", "MONEY_BASE_URL", "MONEY_ACCESS_TOKEN", "EVIDENCE_BASE_URL", "EVIDENCE_ACCESS_TOKEN", "DESIGN_BASE_URL", "DESIGN_ACCESS_TOKEN", "BILLING_BASE_URL", "BILLING_ACCESS_TOKEN", "BUSINESS_ENGAGEMENT_BASE_URL", "BUSINESS_ENGAGEMENT_ACCESS_TOKEN", "RESULTS_BASE_URL", "RESULTS_ACCESS_TOKEN", "REFERENCE_TIMEOUT",
	} {
		t.Setenv(name, "")
	}
	t.Setenv("PRODUCTION_BASE_URL", "")
	t.Setenv("COST_BASE_URL", "")
	t.Setenv("COST_ACCESS_TOKEN", "")
	t.Setenv("PRODUCTION_ACCESS_TOKEN", "")
}

func TestProductionMaterialEndpointIsOptionalButIAMUsesOnlyWorkloadIdentity(t *testing.T) {
	clearConfiguration(t)
	t.Setenv("IDENTITY_PROVIDER", "iam")
	t.Setenv("IAM_ISSUER", "https://iam.example.test")
	t.Setenv("IAM_TENANT_ID", "11111111-1111-4111-8111-111111111111")
	t.Setenv("IAM_PUBLIC_KEYS_PATH", "/tmp/keys.json")
	if _, err := Load(); err != nil {
		t.Fatalf("unselected Production became a startup dependency: %v", err)
	}
	t.Setenv("PRODUCTION_BASE_URL", " https://production.example.test/ ")
	if _, err := Load(); err == nil {
		t.Fatal("production endpoint without workload identity accepted")
	}
	t.Setenv("IAM_SERVICE_CREDENTIAL_URL", "https://iam.example.test/api/v1/service-credentials")
	t.Setenv("WORKLOAD_ID", "project-workspace")
	t.Setenv("WORKLOAD_SUBJECT_ID", "22222222-2222-4222-8222-222222222222")
	t.Setenv("WORKLOAD_KEY_FILE", "/private/project-workspace.json")
	value, err := Load()
	if err != nil || value.ProductionBaseURL != "https://production.example.test" || value.ProductionAccessToken != "" {
		t.Fatalf("production endpoint=%q err=%v", value.ProductionBaseURL, err)
	}
	t.Setenv("PRODUCTION_ACCESS_TOKEN", "long-lived-token-is-forbidden")
	if _, err := Load(); err == nil {
		t.Fatal("production long-lived credential accepted in IAM mode")
	}
}
