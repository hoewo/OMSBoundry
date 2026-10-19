package invoice

import "testing"

func TestNormalizeCreateRejectsBinaryFloatStyleAndMalformedSources(t *testing.T) {
	manual := "人工确认"
	input := CreateInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", IssuedOn: "2026-09-27", AmountMinor: 12345, Currency: "cny", IssuerPartyRef: PartyRef{ID: "11111111-1111-4111-8111-111111111111", Revision: 2}, RecipientPartyRef: PartyRef{ID: "22222222-2222-4222-8222-222222222222", Revision: 3}, SourceRefs: []SourceRef{{Kind: SourceManualConfirmation, ManualSummary: &manual}}}
	normalized, err := NormalizeCreate(input)
	if err != nil {
		t.Fatal(err)
	}
	if normalized.Currency != "CNY" || normalized.AmountMinor != 12345 {
		t.Fatalf("unexpected money normalization: %#v", normalized)
	}
	bad := input
	bad.AmountMinor = 0
	if _, err = NormalizeCreate(bad); err == nil {
		t.Fatal("zero amount must fail")
	}
	bad = input
	bad.SourceRefs = []SourceRef{{Kind: SourceProject, ManualSummary: &manual}}
	if _, err = NormalizeCreate(bad); err == nil {
		t.Fatal("typed source without id and revision must fail")
	}
	bad = input
	bad.RecipientPartyRef = bad.IssuerPartyRef
	if _, err = NormalizeCreate(bad); err == nil {
		t.Fatal("issuer and recipient must differ")
	}
}

func TestNormalizeEvidenceRejectsDuplicateFileVersion(t *testing.T) {
	input := EvidenceInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", ExpectedRevision: 1, Reason: "补文件", EvidenceRefs: []EvidenceRef{{FileID: "11111111-1111-4111-8111-111111111111", Version: 1, Purpose: "invoice_document"}, {FileID: "11111111-1111-4111-8111-111111111111", Version: 1, Purpose: "supporting_material"}}}
	if _, err := NormalizeEvidence(input); err == nil {
		t.Fatal("duplicate file version must fail")
	}
}
