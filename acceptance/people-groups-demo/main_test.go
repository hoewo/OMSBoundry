package main

import "testing"

func TestHiddenValue(t *testing.T) {
	page := `<form><input type="hidden" name="csrf_token" value="right&amp;safe"></form>`
	if got := hiddenValue(page, "csrf_token"); got != "right&safe" {
		t.Fatalf("hiddenValue(csrf_token) = %q", got)
	}
}

func TestContainsRole(t *testing.T) {
	var snapshot rosterSnapshot
	snapshot.Members = append(snapshot.Members, struct {
		MembershipID string `json:"membership_id"`
		RoleKey      string `json:"role_key"`
		Person       struct {
			ID            string `json:"id"`
			Revision      int64  `json:"revision"`
			DisplayName   string `json:"display_name"`
			ReferenceCode string `json:"reference_code"`
		} `json:"person"`
	}{RoleKey: "lead_designer"})
	if !containsRole(snapshot, "lead_designer") || containsRole(snapshot, "designer") {
		t.Fatal("containsRole returned an unexpected result")
	}
	if containsPerson(snapshot, "负责人", 3) {
		t.Fatal("containsPerson matched an absent person")
	}
	snapshot.Members[0].Person.DisplayName = "负责人"
	snapshot.Members[0].Person.Revision = 3
	if !containsPerson(snapshot, "负责人", 3) || containsPerson(snapshot, "负责人", 2) {
		t.Fatal("containsPerson returned an unexpected result")
	}
}

func TestVerifyLifecycle(t *testing.T) {
	persons := []person{
		{ID: "alice", DisplayName: "林清和（负责人）", Status: "active", IAMSubjectID: iamAliceSubject, Revision: 3},
		{ID: "bob", DisplayName: "周予安", Status: "inactive", Revision: 2},
	}
	definitions := []definition{{Kind: "capability", Key: "design_delivery", Status: "inactive", Revision: 2}}
	if err := verifyLifecycle(persons, definitions, "alice", "bob"); err != nil {
		t.Fatal(err)
	}
	definitions[0].Status = "active"
	if err := verifyLifecycle(persons, definitions, "alice", "bob"); err == nil {
		t.Fatal("verifyLifecycle accepted an active capability definition")
	}
}
