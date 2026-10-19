package partymigration

import (
	"reflect"
	"testing"
	"time"
)

const testTenantID = "33333333-3333-4333-8333-333333333333"

func TestEvaluateProducesDeterministicManualReviewInventory(t *testing.T) {
	created := time.Date(2026, 9, 1, 1, 2, 3, 0, time.UTC)
	identifier := "91330000TEST"
	parties := []LegacyParty{
		{ID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", Kind: "individual", LegalName: "张三", Revision: 1, CreatedAt: created, UpdatedAt: created},
		{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Kind: "organization", LegalName: "示例公司", RegistrationIdentifier: &identifier, Revision: 1, CreatedAt: created, UpdatedAt: created, DraftLinkCount: 2, RevisionSnapshotCount: 3, SettlementSnapshotCount: 1},
	}
	first, err := Evaluate(testTenantID, parties)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Evaluate(testTenantID, parties)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("preflight is not deterministic\nfirst=%#v\nsecond=%#v", first, second)
	}
	if !first.ReadyForReview || first.ReadyForCutover || first.AutomaticMergeAllowed || first.Counts.Parties != 2 || first.Counts.Referenced != 1 || first.Counts.Unreferenced != 1 || first.Counts.Unresolved != 2 {
		t.Fatalf("unexpected report: %#v", first)
	}
	if first.Items[0].LocalPartyID != parties[1].ID || first.Items[0].Usage.Total() != 6 || len(first.Items[0].LocalSnapshotSHA256) != 64 {
		t.Fatalf("unexpected first inventory item: %#v", first.Items[0])
	}
	if got := first.Items[0].AllowedDecisions; !reflect.DeepEqual(got, []string{"create_directory_party", "map_existing_directory_party"}) {
		t.Fatalf("allowed decisions = %#v", got)
	}
}

func TestEvaluateMarksDuplicateRegistrationForManualReview(t *testing.T) {
	created := time.Date(2026, 9, 1, 1, 2, 3, 0, time.UTC)
	firstIdentifier, secondIdentifier := " DUP-001 ", "dup-001"
	report, err := Evaluate(testTenantID, []LegacyParty{
		{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Kind: "organization", LegalName: "甲公司", RegistrationIdentifier: &firstIdentifier, Revision: 1, CreatedAt: created, UpdatedAt: created},
		{ID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", Kind: "organization", LegalName: "乙公司", RegistrationIdentifier: &secondIdentifier, Revision: 1, CreatedAt: created, UpdatedAt: created},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.ReadyForReview || report.Counts.DuplicateRegistrationGroups != 1 {
		t.Fatalf("unexpected duplicate report: %#v", report)
	}
	for _, item := range report.Items {
		if len(item.Signals) != 1 || item.Signals[0].Code != "duplicate_registration_identifier" {
			t.Fatalf("duplicate signal missing: %#v", item)
		}
	}
}

func TestEvaluateBlocksStructurallyInvalidLegacyParty(t *testing.T) {
	created := time.Date(2026, 9, 1, 1, 2, 3, 0, time.UTC)
	report, err := Evaluate(testTenantID, []LegacyParty{{
		ID: "not-a-uuid", Kind: "company", LegalName: " ", Revision: 0,
		CreatedAt: created, UpdatedAt: created.Add(-time.Minute),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if report.ReadyForReview || report.ReadyForCutover || report.Counts.BlockingIssues != 1 || report.BlockingIssues[0].Code != "invalid_legacy_party" {
		t.Fatalf("unexpected blocked report: %#v", report)
	}
}

func TestEvaluateRejectsInvalidTenant(t *testing.T) {
	if _, err := Evaluate("bad", nil); err == nil {
		t.Fatal("invalid tenant was accepted")
	}
}
