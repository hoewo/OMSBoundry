package main

import (
	"os"
	"path/filepath"
	"testing"

	"omsbizfoundry/identity-access/internal/config"
)

const testTenantID = "22222222-2222-4222-8222-222222222222"

func TestValidateContractsAcceptsPublishedD1Documents(t *testing.T) {
	examples := filepath.Join("..", "..", "..", "..", "..", "contracts", "access", "v1", "examples")
	manifests, err := filepath.Glob(filepath.Join(examples, "*-capabilities.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(manifests) == 0 {
		t.Fatal("published capability manifests are missing")
	}
	cfg := config.Config{
		TenantID:                testTenantID,
		CapabilityManifestPaths: manifests,
		ProjectAssignmentPolicyPaths: []string{
			filepath.Join(examples, "project-assignment-policy.json"),
			filepath.Join(examples, "business-engagement-project-assignment-policy.json"),
			filepath.Join(examples, "production-operations-project-assignment-policy.json"),
		},
		WorkloadIdentityPolicyPath: filepath.Join(examples, "workload-identity-policy.json"),
	}
	if err := validateContracts(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestValidateContractsRejectsManifestUnknownToCandidateParser(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future-capabilities.json")
	data := []byte(`{"schema_version":"1","application_id":"app-one","application_version":"1.0.0","audience":"app-one","capability_namespace":"app-one","capabilities":[{"id":"app-one.read","description":"read","risk":"read","resource_types":["item"]}],"future_contract_field":true}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateContracts(config.Config{TenantID: testTenantID, CapabilityManifestPaths: []string{path}}); err == nil {
		t.Fatal("candidate parser accepted an unknown contract field")
	}
}
