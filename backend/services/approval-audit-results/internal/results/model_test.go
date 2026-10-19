package results

import (
	"math"
	"testing"
	"time"
)

func TestNormalizeComponentsRequiresExactReferenceOAProfile(t *testing.T) {
	values := []AmountComponent{
		{Kind: ComponentSurveyFee, AmountMinor: 25},
		{Kind: ComponentDesignFee, AmountMinor: 100},
		{Kind: ComponentConsultationFee, AmountMinor: 5},
	}
	normalized, total, err := NormalizeComponents(values, false)
	if err != nil {
		t.Fatal(err)
	}
	if total != 130 {
		t.Fatalf("total=%d, want 130", total)
	}
	if normalized[0].Kind != ComponentConsultationFee || normalized[1].Kind != ComponentDesignFee || normalized[2].Kind != ComponentSurveyFee {
		t.Fatalf("components are not deterministically ordered: %#v", normalized)
	}

	invalid := [][]AmountComponent{
		values[:2],
		{{Kind: ComponentDesignFee, AmountMinor: 1}, {Kind: ComponentDesignFee, AmountMinor: 2}, {Kind: ComponentSurveyFee, AmountMinor: 3}},
		{{Kind: ComponentDesignFee, AmountMinor: -1}, {Kind: ComponentSurveyFee, AmountMinor: 2}, {Kind: ComponentConsultationFee, AmountMinor: 3}},
		{{Kind: ComponentDesignFee, AmountMinor: math.MaxInt64}, {Kind: ComponentSurveyFee, AmountMinor: 1}, {Kind: ComponentConsultationFee, AmountMinor: 0}},
	}
	for index, candidate := range invalid {
		if _, _, candidateErr := NormalizeComponents(candidate, false); candidateErr == nil {
			t.Fatalf("invalid case %d was accepted", index)
		}
	}
}

func TestNormalizeRecordEnforcesKindSpecificExactSource(t *testing.T) {
	base := RecordInput{
		CommandID:           id(1),
		ProjectRef:          ProjectRef{ID: id(2), Revision: 4},
		Kind:                KindApproval,
		Currency:            "cny",
		SourceRef:           SourceRef{Application: "contract-ledger", ResourceType: "settlement", ResourceID: id(3), Revision: 9},
		ExternalConfirmedBy: "OA 审批记录",
		ExternalConfirmedAt: time.Date(2026, 9, 28, 9, 0, 0, 0, time.FixedZone("CST", 8*60*60)),
	}
	normalized, err := NormalizeRecord(base)
	if err != nil {
		t.Fatal(err)
	}
	if normalized.Currency != "CNY" || normalized.ExternalConfirmedAt.Location() != time.UTC {
		t.Fatalf("normalization failed: %#v", normalized)
	}

	base.Kind = KindAudit
	if _, err = NormalizeRecord(base); err == nil {
		t.Fatal("audit accepted contract-ledger source")
	}
	base.SourceRef = SourceRef{Application: "approval-audit-results", ResourceType: "approval-result", ResourceID: id(4), Revision: 1}
	if _, err = NormalizeRecord(base); err != nil {
		t.Fatalf("valid audit source rejected: %v", err)
	}
}

func id(n int) string {
	return "00000000-0000-4000-8000-" + []string{"000000000000", "000000000001", "000000000002", "000000000003", "000000000004", "000000000005", "000000000006", "000000000007", "000000000008", "000000000009", "000000000010", "000000000011", "000000000012", "000000000013", "000000000014", "000000000015", "000000000016", "000000000017", "000000000018", "000000000019", "000000000020"}[n]
}
