package provider

import (
	"context"
	"testing"

	contract "omsbizfoundry/invoice-recognition-contract"
	"omsbizfoundry/invoice-recognition/internal/recognition"
)

func TestXMLProviderRecognizesNamespacedInvoiceExactly(t *testing.T) {
	document := recognition.Document{Evidence: contract.EvidenceReference{MIMEType: "application/xml"}, Bytes: []byte(`<Invoice><InvoiceCode>011001900111</InvoiceCode><InvoiceNo>12345678</InvoiceNo><IssueDate>20261105</IssueDate><SellerName>甲公司</SellerName><SellerTaxID>91110000123456789X</SellerTaxID><BuyerName>乙公司</BuyerName><BuyerTaxID>91110000987654321A</BuyerTaxID><AmountWithoutTax>100.00</AmountWithoutTax><TaxAmount>13.00</TaxAmount><TotalAmountWithTax>113.00</TotalAmountWithTax></Invoice>`)}
	result, err := (XML{}).Recognize(context.Background(), document)
	if err != nil {
		t.Fatal(err)
	}
	if result.Candidate.IssuedOn != "2026-11-05" || *result.Candidate.TotalAmountMinor != 11300 || result.Candidate.FieldConfidenceBPS["invoice_number"] != 10000 {
		t.Fatalf("result=%+v", result)
	}
}
