package party

import "testing"

func TestNormalizeInputs(t *testing.T) {
	identifier := "  CN-001  "
	created, err := NormalizeCreate(CreateInput{CommandID: "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA", Kind: " Organization ", LegalName: "  Example Co  ", RegistrationIdentifier: &identifier})
	if err != nil {
		t.Fatal(err)
	}
	if created.CommandID != "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" || created.Kind != KindOrganization || created.LegalName != "Example Co" || created.RegistrationIdentifier == nil || *created.RegistrationIdentifier != "CN-001" {
		t.Fatalf("unexpected normalized create: %#v", created)
	}
	if _, err := NormalizeCreate(CreateInput{CommandID: created.CommandID, Kind: "customer", LegalName: "Example"}); err == nil {
		t.Fatal("customer must remain a consumer-owned role, not a Party kind")
	}
	if _, err := NormalizeUpdate(UpdateInput{CommandID: created.CommandID, ExpectedRevision: 1, Kind: KindOrganization, LegalName: "Example", Reason: ""}); err == nil {
		t.Fatal("update reason is required")
	}
	if _, err := NormalizeStatus(StatusInput{CommandID: created.CommandID, ExpectedRevision: 1, Status: "deleted", Reason: "cleanup"}); err == nil {
		t.Fatal("physical deletion state must be rejected")
	}
}

func TestNormalizeListDefaultsAndBounds(t *testing.T) {
	filter, err := NormalizeList(ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if filter.Status != StatusActive || filter.Limit != 50 {
		t.Fatalf("unexpected defaults: %#v", filter)
	}
	if _, err := NormalizeList(ListFilter{Status: "all", Limit: 101}); err == nil {
		t.Fatal("oversized pages must fail")
	}
}
