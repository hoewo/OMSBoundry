package config

import "testing"

func TestCostMaterialEndpointOptionalAndIAMNoStaticCredential(t *testing.T) {
	clearConfiguration(t)
	t.Setenv("IDENTITY_PROVIDER", "iam")
	t.Setenv("IAM_ISSUER", "https://iam.example.test")
	t.Setenv("IAM_TENANT_ID", "11111111-1111-4111-8111-111111111111")
	t.Setenv("IAM_PUBLIC_KEYS_PATH", "/private/keys.json")
	if _, err := Load(); err != nil {
		t.Fatalf("unselected Cost startup dependency: %v", err)
	}
	t.Setenv("COST_BASE_URL", " https://cost.example.test/ ")
	if _, err := Load(); err == nil {
		t.Fatal("Cost endpoint without workload identity accepted")
	}
	t.Setenv("IAM_SERVICE_CREDENTIAL_URL", "https://iam.example.test/api/v1/service-credentials")
	t.Setenv("WORKLOAD_ID", "project-workspace")
	t.Setenv("WORKLOAD_SUBJECT_ID", "22222222-2222-4222-8222-222222222222")
	t.Setenv("WORKLOAD_KEY_FILE", "/private/workload.json")
	value, err := Load()
	if err != nil || value.CostBaseURL != "https://cost.example.test" || value.CostAccessToken != "" {
		t.Fatalf("Cost config %q %v", value.CostBaseURL, err)
	}
	t.Setenv("COST_ACCESS_TOKEN", "forbidden-static-token")
	if _, err := Load(); err == nil {
		t.Fatal("static Cost credential accepted in IAM")
	}
}
