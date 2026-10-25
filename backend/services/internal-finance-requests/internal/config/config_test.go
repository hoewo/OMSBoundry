package config

import "testing"

const (
	testTenant  = "11111111-1111-4111-8111-111111111111"
	testSubject = "ffffffff-ffff-4fff-bfff-ffffffffffff"
)

func TestIAMDownstreamsRequireCompleteWorkloadIdentity(t *testing.T) {
	setIAMEnvironment(t)
	config, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if config.WorkloadID != "internal-finance-requests" || config.MoneyBaseURL == "" || config.EvidenceBaseURL == "" {
		t.Fatalf("unexpected config %#v", config)
	}

	t.Setenv("EVIDENCE_BASE_URL", "")
	if _, err = Load(); err == nil {
		t.Fatal("IAM mode accepted a partial Evidence/Money dependency set")
	}
}

func TestIAMModeRejectsStaticDownstreamTokens(t *testing.T) {
	setIAMEnvironment(t)
	t.Setenv("MONEY_ACCESS_TOKEN", "must-not-be-used-in-iam-mode")
	if _, err := Load(); err == nil {
		t.Fatal("IAM mode accepted a static downstream token")
	}
}

func setIAMEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{"TEMPLATE_TENANT_ID", "TEMPLATE_SUBJECT_ID", "TEMPLATE_DISPLAY_NAME", "TEMPLATE_ACCESS_TOKEN", "MONEY_ACCESS_TOKEN", "EVIDENCE_ACCESS_TOKEN"} {
		t.Setenv(name, "")
	}
	t.Setenv("IDENTITY_PROVIDER", "iam")
	t.Setenv("IAM_ISSUER", "https://oms.example.test/auth")
	t.Setenv("IAM_TENANT_ID", testTenant)
	t.Setenv("IAM_PUBLIC_KEYS_PATH", "/tmp/public-keys.json")
	t.Setenv("MONEY_BASE_URL", "http://money-flow-ledger:8083")
	t.Setenv("EVIDENCE_BASE_URL", "http://evidence-service:8085")
	t.Setenv("IAM_SERVICE_CREDENTIAL_URL", "http://identity-access:8080/api/v1/service-credentials")
	t.Setenv("WORKLOAD_ID", "internal-finance-requests")
	t.Setenv("WORKLOAD_SUBJECT_ID", testSubject)
	t.Setenv("WORKLOAD_KEY_FILE", "/workload-private/internal-finance-requests.json")
}
