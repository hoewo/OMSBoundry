package invoicerecognitioncontract

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	ProposalSchemaVersion     = "invoice-recognition.proposal/v1"
	ConfirmationSchemaVersion = "invoice-recognition.confirmation/v1"
	MaximumEvidenceBytes      = 20 * 1024 * 1024
)

var (
	uuidPattern       = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	digestPattern     = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	invoiceKeyPattern = regexp.MustCompile(`^[0-9A-Z-]{1,64}$`)
	taxIDPattern      = regexp.MustCompile(`^[0-9A-Z]{15,20}$`)
)

var supportedMIMETypes = map[string]bool{
	"application/pdf": true,
	"application/ofd": true,
	"application/xml": true,
	"text/xml":        true,
	"image/jpeg":      true,
	"image/png":       true,
}

var recognizedFields = map[string]bool{
	"invoice_code": true, "invoice_number": true, "issued_on": true,
	"seller_name": true, "seller_tax_id": true, "buyer_name": true,
	"buyer_tax_id": true, "amount_excluding_tax_minor": true,
	"tax_amount_minor": true, "total_amount_minor": true, "currency": true,
}

type EvidenceReference struct {
	FileID        string `json:"file_id"`
	Version       int64  `json:"version"`
	ContentSHA256 string `json:"content_sha256"`
	MIMEType      string `json:"mime_type"`
	SizeBytes     int64  `json:"size_bytes"`
}

type Candidate struct {
	InvoiceCode             string           `json:"invoice_code,omitempty"`
	InvoiceNumber           string           `json:"invoice_number,omitempty"`
	IssuedOn                string           `json:"issued_on,omitempty"`
	SellerName              string           `json:"seller_name,omitempty"`
	SellerTaxID             string           `json:"seller_tax_id,omitempty"`
	BuyerName               string           `json:"buyer_name,omitempty"`
	BuyerTaxID              string           `json:"buyer_tax_id,omitempty"`
	AmountExcludingTaxMinor *int64           `json:"amount_excluding_tax_minor,omitempty"`
	TaxAmountMinor          *int64           `json:"tax_amount_minor,omitempty"`
	TotalAmountMinor        *int64           `json:"total_amount_minor,omitempty"`
	Currency                string           `json:"currency,omitempty"`
	FieldConfidenceBPS      map[string]int64 `json:"field_confidence_bps,omitempty"`
}

// FieldRegion keeps the visual provenance of one recognized field without
// owning the source bytes. Coordinates use basis points (0..10000) so the
// Workbench can render the same annotation at any display size.
type FieldRegion struct {
	Page          int64  `json:"page"`
	LeftBPS       int64  `json:"left_bps"`
	TopBPS        int64  `json:"top_bps"`
	WidthBPS      int64  `json:"width_bps"`
	HeightBPS     int64  `json:"height_bps"`
	Text          string `json:"text,omitempty"`
	ConfidenceBPS int64  `json:"confidence_bps,omitempty"`
}

type Proposal struct {
	SchemaVersion    string                   `json:"schema_version"`
	ID               string                   `json:"id"`
	TenantID         string                   `json:"tenant_id"`
	CommandID        string                   `json:"command_id"`
	Evidence         EvidenceReference        `json:"evidence"`
	ProviderKey      string                   `json:"provider_key"`
	ProviderModel    string                   `json:"provider_model"`
	ProviderRevision string                   `json:"provider_revision"`
	Candidate        Candidate                `json:"candidate"`
	FieldRegions     map[string][]FieldRegion `json:"field_regions,omitempty"`
	Warnings         []string                 `json:"warnings,omitempty"`
	Status           string                   `json:"status"`
	Revision         int64                    `json:"revision"`
	CreatedAt        time.Time                `json:"created_at"`
	DiscardReason    string                   `json:"discard_reason,omitempty"`
	DiscardedBy      string                   `json:"discarded_by_subject,omitempty"`
	DiscardedAt      *time.Time               `json:"discarded_at,omitempty"`
}

func ValidateFieldRegions(values map[string][]FieldRegion) error {
	if len(values) > len(recognizedFields) {
		return fmt.Errorf("too many recognized field regions")
	}
	for field, regions := range values {
		if !recognizedFields[field] || len(regions) == 0 || len(regions) > 8 {
			return fmt.Errorf("recognized field region is invalid")
		}
		for _, region := range regions {
			if region.Page < 1 || region.Page > 100 || region.LeftBPS < 0 || region.TopBPS < 0 ||
				region.WidthBPS < 1 || region.HeightBPS < 1 || region.LeftBPS+region.WidthBPS > 10000 ||
				region.TopBPS+region.HeightBPS > 10000 || region.ConfidenceBPS < 0 || region.ConfidenceBPS > 10000 ||
				len([]rune(region.Text)) > 500 {
				return fmt.Errorf("recognized field region is invalid")
			}
		}
	}
	return nil
}

type Confirmation struct {
	SchemaVersion      string            `json:"schema_version"`
	ProposalID         string            `json:"proposal_id"`
	ProposalRevision   int64             `json:"proposal_revision"`
	TenantID           string            `json:"tenant_id"`
	Evidence           EvidenceReference `json:"evidence"`
	Candidate          Candidate         `json:"candidate"`
	DuplicateKeys      []string          `json:"duplicate_keys"`
	ConfirmedBySubject string            `json:"confirmed_by_subject"`
	ConfirmedAt        time.Time         `json:"confirmed_at"`
}

func ValidateEvidence(value EvidenceReference) error {
	value.MIMEType = strings.ToLower(strings.TrimSpace(value.MIMEType))
	if !uuidPattern.MatchString(value.FileID) || value.Version < 1 || !digestPattern.MatchString(value.ContentSHA256) {
		return fmt.Errorf("evidence reference is invalid")
	}
	if !supportedMIMETypes[value.MIMEType] {
		return fmt.Errorf("evidence MIME type is not supported")
	}
	if value.SizeBytes < 1 || value.SizeBytes > MaximumEvidenceBytes {
		return fmt.Errorf("evidence size is outside the recognition limit")
	}
	return nil
}

func ValidateCandidate(value Candidate, confirmed bool) error {
	if value.InvoiceCode != "" && !invoiceKeyPattern.MatchString(value.InvoiceCode) {
		return fmt.Errorf("invoice code is invalid")
	}
	if value.InvoiceNumber != "" && !invoiceKeyPattern.MatchString(value.InvoiceNumber) {
		return fmt.Errorf("invoice number is invalid")
	}
	if confirmed && value.InvoiceNumber == "" {
		return fmt.Errorf("confirmed invoice identity is incomplete")
	}
	if value.IssuedOn != "" {
		if _, err := time.Parse("2006-01-02", value.IssuedOn); err != nil {
			return fmt.Errorf("invoice issue date is invalid")
		}
	}
	for name, taxID := range map[string]string{"seller": value.SellerTaxID, "buyer": value.BuyerTaxID} {
		if taxID != "" && !taxIDPattern.MatchString(taxID) {
			return fmt.Errorf("%s tax identity is invalid", name)
		}
	}
	if value.Currency != "" && value.Currency != "CNY" {
		return fmt.Errorf("invoice currency must be CNY")
	}
	for _, amount := range []*int64{value.AmountExcludingTaxMinor, value.TaxAmountMinor, value.TotalAmountMinor} {
		if amount != nil && *amount < 0 {
			return fmt.Errorf("invoice amount must not be negative")
		}
	}
	if value.AmountExcludingTaxMinor != nil && value.TaxAmountMinor != nil && value.TotalAmountMinor != nil &&
		*value.AmountExcludingTaxMinor+*value.TaxAmountMinor != *value.TotalAmountMinor {
		return fmt.Errorf("invoice total does not equal amount excluding tax plus tax")
	}
	for field, confidence := range value.FieldConfidenceBPS {
		if !recognizedFields[field] || confidence < 0 || confidence > 10000 {
			return fmt.Errorf("field confidence is invalid")
		}
	}
	return nil
}

func DuplicateKeys(evidence EvidenceReference, candidate Candidate) []string {
	keys := []string{"evidence:" + strings.TrimPrefix(evidence.ContentSHA256, "sha256:")}
	if candidate.InvoiceNumber != "" {
		identity := candidate.InvoiceNumber
		if candidate.InvoiceCode != "" {
			identity = candidate.InvoiceCode + ":" + candidate.InvoiceNumber
		}
		keys = append(keys, "invoice:"+identity)
	}
	return keys
}
