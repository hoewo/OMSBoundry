package costoutsourcing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func materialFixture() CostEntry {
	return CostEntry{ID: id(10), TenantID: id(1), Project: ProjectSnapshot{ID: id(3)}, State: StateRecorded, Revision: 2, AmountMinor: 125, Currency: "CNY", Evidence: []EvidenceSnapshot{
		{EvidenceRef: EvidenceRef{FileID: id(7), Version: 2, Slot: "凭证"}, DisplayName: "补充凭证.pdf"},
		{EvidenceRef: EvidenceRef{FileID: id(6), Version: 1, Slot: "凭证"}, DisplayName: "原凭证.pdf"},
	}}
}
func projectEngagement(cost CostEntry) Engagement {
	return Engagement{ID: cost.ID, TenantID: cost.TenantID, Project: cost.Project, State: cost.State, Revision: cost.Revision, Evidence: cost.Evidence, AmountMinor: cost.AmountMinor, Provider: PartySnapshot{ID: id(4), LegalName: "private-provider"}, Contract: &ContractSnapshot{Digest: "private-contract-digest"}}
}
func TestMaterialReferencesTwoAggregatesDeterministicAndNoFinancialLeak(t *testing.T) {
	for _, kind := range []string{"cost_entry", "outsourcing_engagement"} {
		t.Run(kind, func(t *testing.T) {
			value := materialFixture()
			before, _ := json.Marshal(value)
			factory := func(value CostEntry) (MaterialReferenceProjection, error) {
				if kind == "cost_entry" {
					return CostMaterialReference(value)
				}
				return EngagementMaterialReference(projectEngagement(value))
			}
			first, err := factory(value)
			if err != nil || first.SchemaVersion != "1" || first.SourceApplication != "cost-outsourcing" || first.ResourceType != kind || len(first.Items) != 2 || first.Items[0].FileID != id(6) || first.Items[1].EvidenceSourceRevision != 2 {
				t.Fatalf("projection=%#v err=%v", first, err)
			}
			after, _ := json.Marshal(value)
			if string(after) != string(before) {
				t.Fatal("projection mutated snapshot")
			}
			raw, _ := json.Marshal(first.Items)
			digest := sha256.Sum256(raw)
			if first.Digest != hex.EncodeToString(digest[:]) {
				t.Fatal("wrong canonical digest")
			}
			value.AmountMinor = 999999
			value.Evidence[0], value.Evidence[1] = value.Evidence[1], value.Evidence[0]
			second, err := factory(value)
			if err != nil || !reflect.DeepEqual(first, second) {
				t.Fatal("amount/order changed material projection")
			}
			wire, _ := json.Marshal(first)
			for _, forbidden := range []string{"amount", "currency", "provider", "contract", "suggestion", "assessment", "object_key", "credential", "url", "private"} {
				if strings.Contains(string(wire), forbidden) {
					t.Fatalf("leaked %q", forbidden)
				}
			}
			value.State = StateWithdrawn
			withdrawn, err := factory(value)
			if err != nil || withdrawn.Status != StateWithdrawn || len(withdrawn.Items) != 2 || withdrawn.Digest != first.Digest {
				t.Fatalf("withdrawn=%#v err=%v", withdrawn, err)
			}
			value.Evidence = nil
			empty, err := factory(value)
			raw, _ = json.Marshal(empty.Items)
			digest = sha256.Sum256([]byte("[]"))
			if err != nil || string(raw) != "[]" || empty.Digest != hex.EncodeToString(digest[:]) {
				t.Fatal("empty did not stay explicit")
			}
		})
	}
}
func TestMaterialReferencesRejectMalformedOwnedFacts(t *testing.T) {
	tests := map[string]func(*CostEntry){
		"invalid id": func(v *CostEntry) { v.ID = "invalid" }, "invalid tenant": func(v *CostEntry) { v.TenantID = "invalid" }, "invalid project": func(v *CostEntry) { v.Project.ID = "invalid" },
		"uppercase id": func(v *CostEntry) { v.ID = "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA" }, "untrimmed id": func(v *CostEntry) { v.ID = " " + v.ID },
		"zero revision": func(v *CostEntry) { v.Revision = 0 }, "unknown state": func(v *CostEntry) { v.State = "pending" }, "bad file": func(v *CostEntry) { v.Evidence[0].FileID = "bad" },
		"zero version": func(v *CostEntry) { v.Evidence[0].Version = 0 }, "empty slot": func(v *CostEntry) { v.Evidence[0].Slot = "" }, "untrimmed slot": func(v *CostEntry) { v.Evidence[0].Slot = " x" },
		"long slot": func(v *CostEntry) { v.Evidence[0].Slot = strings.Repeat("字", 101) }, "empty name": func(v *CostEntry) { v.Evidence[0].DisplayName = "" }, "untrimmed name": func(v *CostEntry) { v.Evidence[0].DisplayName = "x " },
		"long name": func(v *CostEntry) { v.Evidence[0].DisplayName = strings.Repeat("字", 501) }, "duplicate": func(v *CostEntry) { v.Evidence = append(v.Evidence, v.Evidence[0]) }, "unbounded": func(v *CostEntry) { v.Evidence = make([]EvidenceSnapshot, 101) },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			value := materialFixture()
			mutate(&value)
			if _, err := CostMaterialReference(value); !errors.Is(err, ErrReferenceInvalid) {
				t.Fatalf("cost err=%v", err)
			}
			if _, err := EngagementMaterialReference(projectEngagement(value)); !errors.Is(err, ErrReferenceInvalid) {
				t.Fatalf("engagement err=%v", err)
			}
		})
	}
}
