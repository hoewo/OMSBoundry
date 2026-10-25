package config

import "testing"

func TestPersonReferenceCanBeConfiguredWithoutProjectPolicy(t *testing.T) {
	t.Setenv("TENANT_ID", "11111111-1111-4111-8111-111111111111")
	t.Setenv("TENANT_NAME", "测试租户")
	t.Setenv("PUBLIC_ISSUER", "https://oms.example.test/auth")
	t.Setenv("PERSON_REFERENCE_BASE_URL", "http://127.0.0.1:18084")
	t.Setenv("PERSON_REFERENCE_SERVICE_SUBJECT_ID", "33333333-3333-4333-8333-333333333333")
	value, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if value.PolicyEnabled() || !value.PersonReferenceEnabled() || value.PersonReferenceBaseURL != "http://127.0.0.1:18084" {
		t.Fatalf("unexpected independent Person reference configuration: %#v", value)
	}
}

func TestPersonReferenceRequiresCompletePair(t *testing.T) {
	t.Setenv("TENANT_ID", "11111111-1111-4111-8111-111111111111")
	t.Setenv("TENANT_NAME", "测试租户")
	t.Setenv("PUBLIC_ISSUER", "https://oms.example.test/auth")
	t.Setenv("PERSON_REFERENCE_BASE_URL", "http://127.0.0.1:18084")
	if _, err := Load(); err == nil {
		t.Fatal("expected incomplete Person reference configuration to fail")
	}
}
