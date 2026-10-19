package config

import "testing"

func TestOAProductionProfileIsExplicitAndClosed(t *testing.T) {
	clearConfiguration(t)
	t.Setenv("REFERENCE_OA_PRODUCTION_PERSONNEL_PROFILE", "")
	t.Setenv("TEMPLATE_TENANT_ID", "11111111-1111-4111-8111-111111111111")
	t.Setenv("TEMPLATE_SUBJECT_ID", "22222222-2222-4222-8222-222222222222")
	t.Setenv("TEMPLATE_DISPLAY_NAME", "测试")
	t.Setenv("TEMPLATE_ACCESS_TOKEN", "this-is-a-replaced-template-token")
	v, err := Load()
	if err != nil || v.ReferenceOAProductionProfile != "" {
		t.Fatalf("default %v", err)
	}
	t.Setenv("REFERENCE_OA_PRODUCTION_PERSONNEL_PROFILE", "reference-oa-production-personnel-v1")
	v, err = Load()
	if err != nil || v.ReferenceOAProductionProfile == "" {
		t.Fatalf("explicit %v", err)
	}
	t.Setenv("REFERENCE_OA_PRODUCTION_PERSONNEL_PROFILE", "on")
	if _, err := Load(); err == nil {
		t.Fatal("unknown profile accepted")
	}
}
