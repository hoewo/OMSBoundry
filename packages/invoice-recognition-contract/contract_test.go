package invoicerecognitioncontract

import "testing"

func minor(value int64) *int64 { return &value }

func TestConfirmedCandidateRequiresExactArithmeticAndIdentity(t *testing.T) {
	valid := Candidate{
		InvoiceCode: "011001900111", InvoiceNumber: "12345678", IssuedOn: "2026-11-05",
		SellerTaxID: "91110000123456789X", BuyerTaxID: "91110000987654321A",
		AmountExcludingTaxMinor: minor(10000), TaxAmountMinor: minor(1300), TotalAmountMinor: minor(11300),
		Currency: "CNY", FieldConfidenceBPS: map[string]int64{"invoice_number": 9850},
	}
	if err := ValidateCandidate(valid, true); err != nil {
		t.Fatal(err)
	}
	invalid := valid
	invalid.TotalAmountMinor = minor(11301)
	if err := ValidateCandidate(invalid, true); err == nil {
		t.Fatal("inconsistent recognized total accepted")
	}
	invalid = valid
	invalid.InvoiceNumber = ""
	if err := ValidateCandidate(invalid, true); err == nil {
		t.Fatal("incomplete confirmed invoice identity accepted")
	}
}

func TestFullyDigitalInvoiceMayOmitLegacyInvoiceCode(t *testing.T) {
	value := Candidate{InvoiceNumber: "24442000000000000001", TotalAmountMinor: minor(10000), Currency: "CNY"}
	if err := ValidateCandidate(value, true); err != nil {
		t.Fatal(err)
	}
}

func TestDuplicateKeysKeepEvidenceAndInvoiceIdentityIndependent(t *testing.T) {
	keys := DuplicateKeys(EvidenceReference{ContentSHA256: "sha256:0000000000000000000000000000000000000000000000000000000000000000"}, Candidate{InvoiceCode: "A", InvoiceNumber: "B"})
	if len(keys) != 2 || keys[1] != "invoice:A:B" {
		t.Fatalf("unexpected duplicate keys: %#v", keys)
	}
}

func TestDuplicateKeysUseNumberForFullyDigitalInvoiceWithoutLegacyCode(t *testing.T) {
	keys := DuplicateKeys(EvidenceReference{ContentSHA256: "sha256:0000000000000000000000000000000000000000000000000000000000000000"}, Candidate{InvoiceNumber: "24932000000038888512"})
	if len(keys) != 2 || keys[1] != "invoice:24932000000038888512" {
		t.Fatalf("unexpected fully digital invoice duplicate keys: %#v", keys)
	}
}

func TestFieldRegionsUseBoundedNormalizedCoordinates(t *testing.T) {
	valid := map[string][]FieldRegion{"invoice_number": {{Page: 1, LeftBPS: 1200, TopBPS: 800, WidthBPS: 2500, HeightBPS: 400, Text: "12345678", ConfidenceBPS: 9875}}}
	if err := ValidateFieldRegions(valid); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]map[string][]FieldRegion{
		"unknown field": {"raw_text": {{Page: 1, WidthBPS: 10, HeightBPS: 10}}},
		"outside page":  {"invoice_number": {{Page: 1, LeftBPS: 9900, WidthBPS: 200, HeightBPS: 10}}},
		"empty box":     {"invoice_number": {{Page: 1, WidthBPS: 0, HeightBPS: 10}}},
	} {
		if err := ValidateFieldRegions(value); err == nil {
			t.Fatalf("%s was accepted", name)
		}
	}
}
