package projects

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestInitiationProfilesKeepRequiredFactsAndPermitDeferredProductionOwner(t *testing.T) {
	ordinary := Project{NodeType: NodeTypeProject, ProjectNumber: "OA-001", Name: "项目", ProjectTypeKeys: []string{"design"}}
	owners := map[string]bool{RoleBusinessOwner: true}
	tests := []struct {
		name    string
		profile string
		project Project
		owners  map[string]bool
		missing []string
	}{
		{"legacy dual owner", "", ordinary, owners, []string{RoleProductionOwner}},
		{"explicit dual owner", InitiationProfileDualOwnerV1, ordinary, owners, []string{RoleProductionOwner}},
		{"business owner only", InitiationProfileBusinessOwnerV1, ordinary, owners, []string{}},
		{"business owner still required", InitiationProfileBusinessOwnerV1, ordinary, nil, []string{RoleBusinessOwner}},
		{"ordinary number still required", InitiationProfileBusinessOwnerV1, Project{NodeType: NodeTypeProject, Name: "项目", ProjectTypeKeys: ordinary.ProjectTypeKeys}, owners, []string{"project_number"}},
		{"package number optional", InitiationProfileBusinessOwnerV1, Project{NodeType: NodeTypePackage, Name: "项目包", ProjectTypeKeys: ordinary.ProjectTypeKeys}, owners, []string{}},
		{"legacy package unchanged", "", Project{NodeType: NodeTypePackage, Name: "项目包", ProjectTypeKeys: ordinary.ProjectTypeKeys}, owners, []string{"project_number", RoleProductionOwner}},
		{"name and types still required", InitiationProfileBusinessOwnerV1, Project{NodeType: NodeTypeProject, ProjectNumber: "OA-002"}, owners, []string{"name", "project_type"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := initiationMissing(test.project, test.profile, test.owners); !reflect.DeepEqual(got, test.missing) {
				t.Fatalf("missing=%v want=%v", got, test.missing)
			}
		})
	}
}

func TestInitiationProfileCatalogIsClosed(t *testing.T) {
	if got, err := normalizeInitiationProfile(" Business-Owner-V1 "); err != nil || got != InitiationProfileBusinessOwnerV1 {
		t.Fatalf("normalized=%q error=%v", got, err)
	}
	for _, value := range []string{"reference-oa-c5-v1", "skip-owners", "admin", "business-owner-v2"} {
		if _, err := normalizeInitiationProfile(value); err == nil {
			t.Fatalf("unsupported profile %q accepted", value)
		}
	}
}

func TestOmittedInitiationProfilePreservesLegacyCommandHash(t *testing.T) {
	input := RevisionCommand{CommandID: testCommand, ExpectedRevision: 3}
	legacy := struct {
		Kind, ID string
		Input    struct {
			CommandID        string `json:"command_id"`
			ExpectedRevision int64  `json:"expected_revision"`
		}
	}{Kind: "complete", ID: testProject}
	legacy.Input.CommandID, legacy.Input.ExpectedRevision = input.CommandID, input.ExpectedRevision
	current := struct {
		Kind, ID string
		Input    RevisionCommand
	}{"complete", testProject, input}
	oldJSON, _ := json.Marshal(legacy)
	newJSON, _ := json.Marshal(current)
	oldHash, _ := requestHash(legacy)
	newHash, _ := requestHash(current)
	if string(oldJSON) != string(newJSON) || oldHash != newHash {
		t.Fatalf("legacy payload/hash changed: %s != %s", oldJSON, newJSON)
	}
	current.Input.InitiationProfile = InitiationProfileBusinessOwnerV1
	explicitHash, _ := requestHash(current)
	if explicitHash == oldHash {
		t.Fatal("explicit rule profile must change the command hash")
	}
}
