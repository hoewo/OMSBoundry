package projects

import "testing"

func TestOAProductionDoesNotChangeBaseRoleCatalogue(t *testing.T) {
	for _, role := range []string{"designer", "reviewer", "auditor", "approver", "estimator"} {
		if ValidRole(role) {
			t.Fatalf("customer role %s leaked into Base", role)
		}
	}
	for _, role := range []string{"business_owner", "production_owner", "participant", "commercial_contributor"} {
		if !ValidRole(role) {
			t.Fatalf("old role %s changed", role)
		}
	}
}
