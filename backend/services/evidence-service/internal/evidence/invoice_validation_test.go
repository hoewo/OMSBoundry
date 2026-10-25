package evidence

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestValidateInvoiceBytes(t *testing.T) {
	minimalPDF := []byte("%PDF-1.7\n1 0 obj<<>>endobj\n%%EOF")
	if media, err := validateInvoiceBytes("invoice.pdf", minimalPDF); err != nil || media != "application/pdf" {
		t.Fatalf("PDF validation = %q, %v", media, err)
	}
	if _, err := validateInvoiceBytes("invoice.pdf", []byte("%PDF-1.7 /JavaScript")); err == nil {
		t.Fatal("active PDF content must be rejected")
	}
	if _, err := validateInvoiceBytes("invoice.xml", []byte(`<?xml version="1.0"?><!DOCTYPE a [<!ENTITY x SYSTEM "file:///etc/passwd">]><a>&x;</a>`)); err == nil {
		t.Fatal("XML external entities must be rejected")
	}
	if media, err := validateInvoiceBytes("invoice.xml", []byte(`<?xml version="1.0"?><invoice><amount>100</amount></invoice>`)); err != nil || media != "application/xml" {
		t.Fatalf("XML validation = %q, %v", media, err)
	}

	var content bytes.Buffer
	writer := zip.NewWriter(&content)
	entry, err := writer.Create("OFD.xml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = entry.Write([]byte(`<ofd/>`))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if media, err := validateInvoiceBytes("invoice.ofd", content.Bytes()); err != nil || media != "application/ofd" {
		t.Fatalf("OFD validation = %q, %v", media, err)
	}
}

func TestValidateInvoiceCreate(t *testing.T) {
	input := CreateUploadInput{FileName: "invoice.pdf", SizeBytes: invoiceMaxBytes, ClientContext: ClientContext{Purpose: "invoice"}}
	if err := validateInvoiceCreate(input); err != nil {
		t.Fatal(err)
	}
	input.FileName = "invoice.docx"
	if err := validateInvoiceCreate(input); err == nil {
		t.Fatal("Office files must be rejected")
	}
	input.FileName = "invoice.pdf"
	input.SizeBytes++
	if err := validateInvoiceCreate(input); err == nil {
		t.Fatal("oversized invoice must be rejected")
	}
}
