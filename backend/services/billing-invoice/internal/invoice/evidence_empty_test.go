package invoice

import (
	"encoding/json"
	"testing"
)

func TestNormalizeEmptyEvidenceAndStorageEncoding(t *testing.T) {
	for _, refs := range [][]EvidenceRef{nil, {}} {
		input, err := NormalizeEvidence(EvidenceInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaab", ExpectedRevision: 2, Reason: "remove last reference", EvidenceRefs: refs})
		if err != nil || input.EvidenceRefs == nil || len(input.EvidenceRefs) != 0 {
			t.Fatalf("empty set = %#v, %v", input, err)
		}
		value := Registration{EvidenceRefs: append([]EvidenceRef{}, input.EvidenceRefs...)}
		_, _, _, raw, err := encodeParts(value)
		if err != nil || string(raw) != "[]" {
			t.Fatalf("database array encoding = %s, %v", raw, err)
		}
		encoded, err := json.Marshal(input)
		if err != nil || len(encoded) == 0 {
			t.Fatal(err)
		}
	}
}
