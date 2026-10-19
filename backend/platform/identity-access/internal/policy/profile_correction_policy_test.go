package policy

import (
	"path/filepath"
	"slices"
	"testing"

	"omsbizfoundry/identity-access/internal/capability"
)

func TestProfileCorrectionPolicyIsExplicitAndPreservesLegacyAssignments(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "..", "contracts", "access", "v1", "examples")
	legacy, err := loadPolicy(filepath.Join(root, "project-assignment-policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	candidatePath := filepath.Join(root, "project-assignment-policy-v1.1.json")
	if err := Validate([]string{candidatePath}); err != nil {
		t.Fatal(err)
	}
	candidate, err := loadPolicy(candidatePath)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := capability.Load([]string{filepath.Join(root, "project-workspace-capabilities.json")})
	if err != nil {
		t.Fatal(err)
	}
	const correction = "project-workspace.profile.correct"
	if candidate.PolicyVersion != "1.1.0" || legacy.PolicyVersion != "1.0.0" ||
		!slices.Contains(candidate.Roles["business_owner"], correction) ||
		slices.Contains(legacy.Roles["business_owner"], correction) ||
		slices.Contains(candidate.Roles["production_owner"], correction) ||
		slices.Contains(candidate.Roles["participant"], correction) {
		t.Fatal("correction must be an opt-in business-owner capability, not an implicit legacy or production grant")
	}
	for _, role := range []string{"production_owner", "participant"} {
		if !slices.Equal(candidate.Roles[role], legacy.Roles[role]) {
			t.Fatalf("unrelated role %s changed", role)
		}
	}
	for _, actions := range candidate.Roles {
		for _, action := range actions {
			if !catalog.AllowsResource("project-workspace", action, "project") {
				t.Fatalf("policy action %s is not a published Project capability", action)
			}
		}
	}
	if catalog.AllowsResource("project-workspace", correction, "contract") {
		t.Fatal("basic Project correction must not authorize a Contract resource")
	}
}
