package costoutsourcing

import (
	"strings"
	"testing"
)

func TestCostNormalizationKeepsExactMinorUnitsAndCategoryRules(t *testing.T) {
	input := RecordCostInput{
		CommandID: id(1), ProjectRef: ProjectRef{ID: id(2), Revision: 3}, Category: CategoryVehicle,
		OccurredOn: "2026-09-28", Currency: "cny", AmountMinor: 101, DistanceMeters: 12345, Description: "现场车辆成本",
	}
	value, err := normalizeRecordCost(input)
	if err != nil {
		t.Fatal(err)
	}
	if value.AmountMinor != 101 || value.Currency != "CNY" || value.DistanceMeters != 12345 {
		t.Fatalf("exact values changed: %#v", value)
	}
	input.AmountMinor = -1
	if _, err = normalizeRecordCost(input); err == nil {
		t.Fatal("negative amount must be rejected")
	}
	input.AmountMinor, input.Category, input.DistanceMeters = 101, CategoryAccommodation, 12
	if _, err = normalizeRecordCost(input); err == nil {
		t.Fatal("distance outside vehicle category must be rejected")
	}
	input.DistanceMeters, input.OccurredOn = 0, "2026-02-30"
	if _, err = normalizeRecordCost(input); err == nil {
		t.Fatal("invalid calendar date must be rejected")
	}
}

func TestEngagementAndAssessmentNormalizationRejectsInvalidFacts(t *testing.T) {
	input := RecordEngagementInput{
		CommandID: id(1), ProjectRef: ProjectRef{ID: id(2), Revision: 3}, ProviderRef: PartyRef{ID: id(3), Revision: 4},
		ContactName: "  李工  ", ContactPhone: " 13800000000 ", Scope: "桥梁测量协作", Currency: "CNY", AmountMinor: 50000,
	}
	value, err := normalizeRecordEngagement(input)
	if err != nil {
		t.Fatal(err)
	}
	if value.ContactName != "李工" || value.ContactPhone != "13800000000" {
		t.Fatalf("contact snapshot was not normalized: %#v", value)
	}
	input.AmountMinor = -1
	if _, err := normalizeRecordEngagement(input); err == nil {
		t.Fatal("negative engagement amount must be rejected")
	}
	input.AmountMinor, input.ContactName = 50000, strings.Repeat("联", 101)
	if _, err := normalizeRecordEngagement(input); err == nil {
		t.Fatal("oversized engagement contact must be rejected")
	}
	if _, err := normalizeAssess(AssessInput{CommandID: id(5), ExpectedEngagementRevision: 1, Score: 6, Reason: "越界"}); err == nil {
		t.Fatal("score outside five-point rule must be rejected")
	}
}

func TestActorNormalizationDoesNotMutateCallerScope(t *testing.T) {
	projects := []string{" 00000000-0000-4000-8000-000000000002 "}
	actor := Actor{ID: id(1), TenantID: id(3), Kind: "user", CredentialID: "cred", ProjectIDs: projects, Actions: []string{ActionCostRead}}
	value, err := normalizeActor(actor)
	if err != nil {
		t.Fatal(err)
	}
	if projects[0] != " 00000000-0000-4000-8000-000000000002 " {
		t.Fatalf("caller-owned scope was mutated: %q", projects[0])
	}
	if value.ProjectIDs[0] != id(2) {
		t.Fatalf("normalized scope missing: %#v", value.ProjectIDs)
	}
}

func id(value int) string {
	return "00000000-0000-4000-8000-" + leftPad(value)
}

func leftPad(value int) string {
	digits := "000000000000" + fmtInt(value)
	return digits[len(digits)-12:]
}

func fmtInt(value int) string {
	if value == 0 {
		return "0"
	}
	var raw [20]byte
	position := len(raw)
	for value > 0 {
		position--
		raw[position] = byte('0' + value%10)
		value /= 10
	}
	return string(raw[position:])
}
