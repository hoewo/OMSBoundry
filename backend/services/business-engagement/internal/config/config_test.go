package config

import "testing"

const (
	tenantID  = "11111111-1111-4111-8111-111111111111"
	subjectID = "22222222-2222-4222-8222-222222222222"
)

func TestLoadTemplateConfiguration(t *testing.T) {
	clearConfiguration(t)
	setRequiredEndpoints(t)
	t.Setenv("TEMPLATE_TENANT_ID", tenantID)
	t.Setenv("TEMPLATE_SUBJECT_ID", subjectID)
	t.Setenv("TEMPLATE_DISPLAY_NAME", "本地管理员")
	t.Setenv("TEMPLATE_ACCESS_TOKEN", "template-access-token-123456789")
	t.Setenv("PROJECT_ACCESS_TOKEN", "project-reference-token-123456")
	t.Setenv("EVIDENCE_ACCESS_TOKEN", "evidence-reference-token-12345")
	value, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if value.IdentityProvider != IdentityTemplate || value.HTTPAddress != ":8089" || value.ProjectBaseURL != "http://127.0.0.1:8086" {
		t.Fatalf("config=%#v", value)
	}
}

func TestLoadIAMRequiresDynamicWorkloadIdentity(t *testing.T) {
	clearConfiguration(t)
	setRequiredEndpoints(t)
	t.Setenv("IDENTITY_PROVIDER", "iam")
	t.Setenv("IAM_ISSUER", "https://iam.example.test/auth")
	t.Setenv("IAM_TENANT_ID", tenantID)
	t.Setenv("IAM_PUBLIC_KEYS_PATH", "/run/secrets/iam-public-keys.json")
	if _, err := Load(); err == nil {
		t.Fatal("iam mode without workload identity was accepted")
	}
	t.Setenv("IAM_SERVICE_CREDENTIAL_URL", "http://identity:8080/api/v1/service-credentials")
	t.Setenv("WORKLOAD_ID", "business-engagement")
	t.Setenv("WORKLOAD_SUBJECT_ID", subjectID)
	t.Setenv("WORKLOAD_KEY_FILE", "/run/secrets/business-engagement-workload.json")
	value, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if value.ProjectAccessToken != "" || value.EvidenceAccessToken != "" || value.WorkloadID != "business-engagement" {
		t.Fatalf("config=%#v", value)
	}
}

func TestLoadRejectsMixedOrIncompleteCredentials(t *testing.T) {
	clearConfiguration(t)
	setRequiredEndpoints(t)
	t.Setenv("TEMPLATE_TENANT_ID", tenantID)
	t.Setenv("TEMPLATE_SUBJECT_ID", subjectID)
	t.Setenv("TEMPLATE_DISPLAY_NAME", "本地管理员")
	t.Setenv("TEMPLATE_ACCESS_TOKEN", "replace-with-at-least-24-random-bytes")
	if _, err := Load(); err == nil {
		t.Fatal("placeholder template credential was accepted")
	}
	t.Setenv("TEMPLATE_ACCESS_TOKEN", "template-access-token-123456789")
	t.Setenv("PROJECT_ACCESS_TOKEN", "project-reference-token-123456")
	if _, err := Load(); err == nil {
		t.Fatal("template mode without evidence credential was accepted")
	}

	clearConfiguration(t)
	setRequiredEndpoints(t)
	t.Setenv("IDENTITY_PROVIDER", "iam")
	t.Setenv("IAM_ISSUER", "https://iam.example.test/auth")
	t.Setenv("IAM_TENANT_ID", tenantID)
	t.Setenv("IAM_PUBLIC_KEYS_PATH", "/run/secrets/iam-public-keys.json")
	t.Setenv("IAM_SERVICE_CREDENTIAL_URL", "http://identity:8080/api/v1/service-credentials")
	t.Setenv("WORKLOAD_ID", "business-engagement")
	t.Setenv("WORKLOAD_SUBJECT_ID", subjectID)
	t.Setenv("WORKLOAD_KEY_FILE", "/run/secrets/business-engagement-workload.json")
	t.Setenv("PROJECT_ACCESS_TOKEN", "static-token-is-forbidden-123")
	if _, err := Load(); err == nil {
		t.Fatal("static reference credential was accepted in iam mode")
	}
}

func setRequiredEndpoints(t *testing.T) {
	t.Helper()
	t.Setenv("PROJECT_BASE_URL", "http://127.0.0.1:8086")
	t.Setenv("EVIDENCE_BASE_URL", "http://127.0.0.1:8085")
}

func clearConfiguration(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"HTTP_ADDR", "DATABASE_URL", "SHUTDOWN_TIMEOUT", "REFERENCE_TIMEOUT", "WORKLOAD_REFRESH_BEFORE", "MIGRATION_MODE", "IDENTITY_PROVIDER",
		"IAM_ISSUER", "IAM_TENANT_ID", "IAM_PUBLIC_KEYS_PATH", "TEMPLATE_TENANT_ID", "TEMPLATE_SUBJECT_ID", "TEMPLATE_DISPLAY_NAME", "TEMPLATE_ACCESS_TOKEN",
		"PROJECT_BASE_URL", "PROJECT_ACCESS_TOKEN", "EVIDENCE_BASE_URL", "EVIDENCE_ACCESS_TOKEN", "IAM_SERVICE_CREDENTIAL_URL", "WORKLOAD_ID", "WORKLOAD_SUBJECT_ID", "WORKLOAD_KEY_FILE",
	} {
		t.Setenv(name, "")
	}
}
