package config

import "testing"

func TestTemplateConfigurationRequiresPeopleReference(t *testing.T) {
	t.Setenv("IDENTITY_PROVIDER", "template")
	t.Setenv("TEMPLATE_TENANT_ID", "55555555-5555-4555-8555-555555555555")
	t.Setenv("TEMPLATE_SUBJECT_ID", "66666666-6666-4666-8666-666666666666")
	t.Setenv("TEMPLATE_DISPLAY_NAME", "Admin")
	t.Setenv("TEMPLATE_ACCESS_TOKEN", "template-access-token-long-enough")
	t.Setenv("PEOPLE_BASE_URL", "http://people.test")
	t.Setenv("PEOPLE_ACCESS_TOKEN", "people-access-token-long-enough")
	config, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if config.HTTPAddress != ":8096" || config.WorkloadID != "" {
		t.Fatalf("config=%+v", config)
	}
}
