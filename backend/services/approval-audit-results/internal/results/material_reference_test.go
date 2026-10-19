package results

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func materialFixture() Result {
	return Result{ID: id(20), TenantID: id(2), Project: ProjectSnapshot{ID: id(3)}, Kind: KindApproval, State: StateRecorded, Revision: 2,
		TotalAmountMinor: 125, Currency: "CNY", Source: SourceSnapshot{Digest: "private-source-amount-digest"},
		Evidence: []EvidenceSnapshot{
			{EvidenceRef: EvidenceRef{FileID: id(14), Version: 2, Slot: "批复文书"}, DisplayName: "补充批复.pdf"},
			{EvidenceRef: EvidenceRef{FileID: id(13), Version: 1, Slot: "批复文书"}, DisplayName: "批复.pdf"},
		}}
}

func TestMaterialReferenceDeterministicOwnedSnapshotWithoutAmounts(t *testing.T) {
	value := materialFixture()
	original, _ := json.Marshal(value)
	projection, err := MaterialReference(value)
	if err != nil || projection.SchemaVersion != "1" || projection.SourceApplication != "approval-audit-results" || projection.ResourceType != "approval_audit_result" || projection.ResultKind != KindApproval || projection.Revision != 2 || len(projection.Items) != 2 {
		t.Fatalf("projection=%#v err=%v", projection, err)
	}
	if projection.Items[0].FileID != id(13) || projection.Items[1].EvidenceSourceRevision != 2 {
		t.Fatalf("sorting/frozen version lost: %#v", projection.Items)
	}
	raw, _ := json.Marshal(projection.Items)
	digest := sha256.Sum256(raw)
	if projection.Digest != hex.EncodeToString(digest[:]) {
		t.Fatal("digest is not canonical items digest")
	}
	after, _ := json.Marshal(value)
	if string(after) != string(original) {
		t.Fatal("projection mutated the owned snapshot")
	}
	value.TotalAmountMinor = 999999
	value.Source.Digest = "another-private-digest"
	value.Evidence[0], value.Evidence[1] = value.Evidence[1], value.Evidence[0]
	second, err := MaterialReference(value)
	if err != nil || !reflect.DeepEqual(projection, second) {
		t.Fatalf("amount/order changed materials: %#v %v", second, err)
	}
	wire, _ := json.Marshal(projection)
	for _, forbidden := range []string{"amount", "currency", "private", "object_key", "url", "source_ref", "credential"} {
		if strings.Contains(string(wire), forbidden) {
			t.Fatalf("material projection leaked %q", forbidden)
		}
	}
}

func TestMaterialReferenceEmptyAndWithdrawnAreExplicit(t *testing.T) {
	for _, kind := range []string{KindApproval, KindAudit} {
		value := materialFixture()
		value.Kind, value.State = kind, StateWithdrawn
		projection, err := MaterialReference(value)
		if err != nil || projection.Status != StateWithdrawn || projection.ResultKind != kind || len(projection.Items) != 2 {
			t.Fatalf("withdrawn projection=%#v err=%v", projection, err)
		}
		value.Evidence = nil
		empty, err := MaterialReference(value)
		raw, _ := json.Marshal(empty.Items)
		digest := sha256.Sum256([]byte("[]"))
		if err != nil || string(raw) != "[]" || empty.Digest != hex.EncodeToString(digest[:]) {
			t.Fatalf("empty projection=%#v err=%v", empty, err)
		}
	}
}

func TestMaterialReferenceRejectsMalformedOwnedSnapshots(t *testing.T) {
	tests := map[string]func(*Result){
		"bad id":          func(v *Result) { v.ID = "invalid" },
		"noncanonical id": func(v *Result) { v.ID = " " + v.ID },
		"bad tenant":      func(v *Result) { v.TenantID = "invalid" },
		"bad project":     func(v *Result) { v.Project.ID = "invalid" },
		"bad kind":        func(v *Result) { v.Kind = "invoice" },
		"bad status":      func(v *Result) { v.State = "pending" },
		"zero revision":   func(v *Result) { v.Revision = 0 },
		"bad file":        func(v *Result) { v.Evidence[0].FileID = "invalid" },
		"uppercase file":  func(v *Result) { v.Evidence[0].FileID = "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA" },
		"zero version":    func(v *Result) { v.Evidence[0].Version = 0 },
		"empty slot":      func(v *Result) { v.Evidence[0].Slot = "" },
		"untrimmed slot":  func(v *Result) { v.Evidence[0].Slot = " x" },
		"long slot":       func(v *Result) { v.Evidence[0].Slot = strings.Repeat("字", 101) },
		"empty name":      func(v *Result) { v.Evidence[0].DisplayName = "" },
		"untrimmed name":  func(v *Result) { v.Evidence[0].DisplayName = "x " },
		"long name":       func(v *Result) { v.Evidence[0].DisplayName = strings.Repeat("字", 501) },
		"duplicate tuple": func(v *Result) { v.Evidence = append(v.Evidence, v.Evidence[0]) },
		"unbounded items": func(v *Result) { v.Evidence = make([]EvidenceSnapshot, 101) },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			value := materialFixture()
			mutate(&value)
			if _, err := MaterialReference(value); !errors.Is(err, ErrReferenceInvalid) {
				t.Fatalf("expected fail closed, got %v", err)
			}
		})
	}
}
