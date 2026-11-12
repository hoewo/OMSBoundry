package provider

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
	"time"

	contract "omsbizfoundry/invoice-recognition-contract"
	"omsbizfoundry/invoice-recognition/internal/recognition"
)

func TestLocalImageRecognizesConfiguredFixture(t *testing.T) {
	path := os.Getenv("INVOICE_OCR_TEST_IMAGE")
	if path == "" {
		t.Skip("set INVOICE_OCR_TEST_IMAGE to run the local OCR integration test")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := NewLocalImage("tesseract", "pdftoppm", 90*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	result, err := provider.Recognize(context.Background(), recognition.Document{
		Evidence: contract.EvidenceReference{
			FileID:        "00000000-0000-4000-8000-000000000001",
			Version:       1,
			ContentSHA256: fmt.Sprintf("sha256:%x", sha256.Sum256(content)),
			MIMEType:      "image/jpeg",
			SizeBytes:     int64(len(content)),
		},
		Bytes: content,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Candidate.InvoiceNumber == "" || result.Candidate.TotalAmountMinor == nil || len(result.FieldRegions) < 4 {
		t.Fatalf("insufficient OCR result: candidate=%+v regions=%+v warnings=%v", result.Candidate, result.FieldRegions, result.Warnings)
	}
	t.Logf("candidate=%+v regions=%d warnings=%v", result.Candidate, len(result.FieldRegions), result.Warnings)
}
