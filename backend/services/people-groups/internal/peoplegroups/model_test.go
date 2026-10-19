package peoplegroups

import "testing"

func TestDefinitionKindAndStableKeyValidation(t *testing.T) {
	if kind, err := NormalizeDefinitionKind(" Membership_Role "); err != nil || kind != DefinitionMembershipRole {
		t.Fatalf("unexpected kind: %q %v", kind, err)
	}
	for _, invalid := range []string{"施工队", "Leader", "a", "a.b", "-leader"} {
		if _, err := cleanKey("key", invalid); err == nil {
			t.Fatalf("expected key %q to fail", invalid)
		}
	}
	if value, err := cleanKey("key", " construction_team "); err != nil || value != "construction_team" {
		t.Fatalf("unexpected normalized key: %q %v", value, err)
	}
}

func TestDatesAndUUIDsAreExplicit(t *testing.T) {
	if _, err := cleanDate("start_date", "2026-02-30"); err == nil {
		t.Fatal("expected impossible date to fail")
	}
	if _, err := cleanUUID("person_id", "not-a-uuid", false); err == nil {
		t.Fatal("expected invalid UUID to fail")
	}
}
