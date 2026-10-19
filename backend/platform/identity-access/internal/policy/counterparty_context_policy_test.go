package policy

import (
	"omsbizfoundry/identity-access/internal/capability"
	"path/filepath"
	"slices"
	"testing"
)

func TestCounterpartyContextPolicyIsExplicitAndNotAnIdentityOrGenericParticipantGrant(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "..", "contracts", "access", "v1", "examples")
	path := filepath.Join(root, "project-assignment-policy-v1.2.json")
	if err := Validate([]string{path}); err != nil {
		t.Fatal(err)
	}
	old, err := loadPolicy(filepath.Join(root, "project-assignment-policy-v1.1.json"))
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := loadPolicy(path)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := capability.Load([]string{filepath.Join(root, "project-workspace-capabilities.json")})
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"project-workspace.counterparty-context.read", "project-workspace.counterparty-context.manage"} {
		if !slices.Contains(candidate.Roles["business_owner"], action) || slices.Contains(old.Roles["business_owner"], action) || slices.Contains(candidate.Roles["participant"], action) || slices.Contains(candidate.Roles["production_owner"], action) || !catalog.AllowsResource("project-workspace", action, "project") || catalog.AllowsResource("project-workspace", action, "party") {
			t.Fatalf("invalid opt-in capability %s", action)
		}
	}
	if candidate.PolicyVersion != "1.2.0" || !slices.Equal(old.Roles["participant"], candidate.Roles["participant"]) || !slices.Equal(old.Roles["production_owner"], candidate.Roles["production_owner"]) {
		t.Fatal("unrelated policy changed")
	}
}
