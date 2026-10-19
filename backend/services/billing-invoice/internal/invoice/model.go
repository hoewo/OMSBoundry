package invoice

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	StatusRecorded                = "recorded"
	StatusRegistrationInvalidated = "registration_invalidated"

	EventRecorded                = "recorded"
	EventDetailsCorrected        = "details_corrected"
	EventEvidenceChanged         = "evidence_changed"
	EventRegistrationInvalidated = "registration_invalidated"

	SourceManualConfirmation = "manual_confirmation"
	SourceProject            = "project"
	SourceContract           = "contract"
	SourceSettlement         = "settlement"
	SourceMoneyObligation    = "money_obligation"
)

var (
	ErrNotFound                = errors.New("invoice registration not found")
	ErrRevisionConflict        = errors.New("invoice registration revision conflict")
	ErrCommandConflict         = errors.New("command payload conflict")
	ErrNoEffectiveChange       = errors.New("no effective change")
	ErrRegistrationInvalidated = errors.New("invoice registration invalidated")
	ErrReferenceInvalid        = errors.New("reference invalid")
	ErrReferenceUnavailable    = errors.New("reference unavailable")
	uuidPattern                = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	currencyPattern            = regexp.MustCompile(`^[A-Z]{3}$`)
)

type Actor struct {
	ID           string `json:"id"`
	TenantID     string `json:"tenant_id"`
	DisplayName  string `json:"display_name"`
	Kind         string `json:"kind"`
	CredentialID string `json:"credential_id"`
}

type PartyRef struct {
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
}

type PartySnapshot struct {
	ID                     string  `json:"id"`
	Revision               int64   `json:"revision"`
	Kind                   string  `json:"kind"`
	LegalName              string  `json:"legal_name"`
	RegistrationIdentifier *string `json:"registration_identifier,omitempty"`
}

type PartyReference struct {
	PartySnapshot
	TenantID string `json:"tenant_id"`
	Status   string `json:"status"`
}

type SourceRef struct {
	Kind           string  `json:"kind"`
	ResourceID     *string `json:"resource_id,omitempty"`
	SourceRevision *int64  `json:"source_revision,omitempty"`
	ManualSummary  *string `json:"manual_summary,omitempty"`
}

type SourceSnapshot struct {
	Kind              string    `json:"kind"`
	SourceApplication *string   `json:"source_application,omitempty"`
	ResourceID        *string   `json:"resource_id,omitempty"`
	SourceRevision    *int64    `json:"source_revision,omitempty"`
	DisplaySummary    string    `json:"display_summary"`
	ConfirmedBy       string    `json:"confirmed_by"`
	ConfirmedAt       time.Time `json:"confirmed_at"`
}

type SourceReference struct {
	SourceApplication string
	TenantID          string
	ResourceID        string
	Revision          int64
	Status            string
	DisplaySummary    string
}

type EvidenceRef struct {
	FileID      string `json:"file_id"`
	Version     int64  `json:"version"`
	Purpose     string `json:"purpose"`
	DisplayName string `json:"display_name,omitempty"`
}

type EvidenceReference struct {
	TenantID string
	FileID   string
	Version  int64
	Revision int64
	Status   string
}

type Registration struct {
	ID             string           `json:"id"`
	TenantID       string           `json:"tenant_id"`
	Status         string           `json:"status"`
	IssuedOn       string           `json:"issued_on"`
	AmountMinor    int64            `json:"amount_minor"`
	Currency       string           `json:"currency"`
	IssuerParty    PartySnapshot    `json:"issuer_party"`
	RecipientParty PartySnapshot    `json:"recipient_party"`
	SourceRefs     []SourceSnapshot `json:"source_refs"`
	EvidenceRefs   []EvidenceRef    `json:"evidence_refs"`
	Note           string           `json:"note"`
	Revision       int64            `json:"revision"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
	AllowedActions []string         `json:"allowed_actions,omitempty"`
}

type Revision struct {
	InvoiceRegistrationID string       `json:"invoice_registration_id"`
	TenantID              string       `json:"tenant_id"`
	Revision              int64        `json:"revision"`
	EventKind             string       `json:"event_kind"`
	CommandID             string       `json:"command_id"`
	Snapshot              Registration `json:"snapshot"`
	Reason                string       `json:"reason"`
	ActorRef              string       `json:"actor_ref"`
	OccurredAt            time.Time    `json:"occurred_at"`
}

type Reference struct {
	SchemaVersion     string   `json:"schema_version"`
	SourceApplication string   `json:"source_application"`
	ID                string   `json:"id"`
	TenantID          string   `json:"tenant_id"`
	Status            string   `json:"status"`
	IssuedOn          string   `json:"issued_on"`
	AmountMinor       int64    `json:"amount_minor"`
	Currency          string   `json:"currency"`
	ProjectIDs        []string `json:"project_ids"`
	Revision          int64    `json:"revision"`
}

type CreateInput struct {
	CommandID         string      `json:"command_id"`
	IssuedOn          string      `json:"issued_on"`
	AmountMinor       int64       `json:"amount_minor"`
	Currency          string      `json:"currency"`
	IssuerPartyRef    PartyRef    `json:"issuer_party_ref"`
	RecipientPartyRef PartyRef    `json:"recipient_party_ref"`
	SourceRefs        []SourceRef `json:"source_refs"`
	Note              string      `json:"note"`
}

type CorrectionPatch struct {
	IssuedOn          *string      `json:"issued_on,omitempty"`
	AmountMinor       *int64       `json:"amount_minor,omitempty"`
	Currency          *string      `json:"currency,omitempty"`
	IssuerPartyRef    *PartyRef    `json:"issuer_party_ref,omitempty"`
	RecipientPartyRef *PartyRef    `json:"recipient_party_ref,omitempty"`
	SourceRefs        *[]SourceRef `json:"source_refs,omitempty"`
	Note              *string      `json:"note,omitempty"`
}

type CorrectInput struct {
	CommandID        string          `json:"command_id"`
	ExpectedRevision int64           `json:"expected_revision"`
	Reason           string          `json:"reason"`
	Patch            CorrectionPatch `json:"patch"`
}

type InvalidateInput struct {
	CommandID        string `json:"command_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Reason           string `json:"reason"`
}

type EvidenceInput struct {
	CommandID        string        `json:"command_id"`
	ExpectedRevision int64         `json:"expected_revision"`
	Reason           string        `json:"reason"`
	EvidenceRefs     []EvidenceRef `json:"evidence_refs"`
}

type ResolvedCreate struct {
	IssuerParty    PartySnapshot
	RecipientParty PartySnapshot
	SourceRefs     []SourceSnapshot
}

type ResolvedCorrection struct {
	IssuerParty    *PartySnapshot
	RecipientParty *PartySnapshot
	SourceRefs     *[]SourceSnapshot
}

type HistoryPage struct {
	Items              []Revision `json:"items"`
	NextBeforeRevision *int64     `json:"next_before_revision,omitempty"`
}

type ListFilter struct {
	ProjectID  string
	PartyID    string
	Status     string
	IssuedFrom string
	IssuedTo   string
	BeforeDate string
	BeforeID   string
	Limit      int
}

type ListQuery struct {
	ProjectID  string
	PartyID    string
	Status     string
	IssuedFrom string
	IssuedTo   string
	Cursor     string
	Limit      int
}

type ListPage struct {
	Items                []Registration
	NextCursor           string
	EffectiveAmountMinor int64
	Currency             string
}

func NormalizeActor(actor Actor) (Actor, error) {
	actor.ID = normalizeUUID(actor.ID)
	actor.TenantID = normalizeUUID(actor.TenantID)
	actor.DisplayName = strings.TrimSpace(actor.DisplayName)
	actor.Kind = strings.TrimSpace(actor.Kind)
	actor.CredentialID = strings.TrimSpace(actor.CredentialID)
	if !ValidUUID(actor.ID) || !ValidUUID(actor.TenantID) || !validText(actor.Kind, 1, 100) || !validText(actor.CredentialID, 1, 256) {
		return Actor{}, fmt.Errorf("invalid actor")
	}
	return actor, nil
}

func NormalizeCreate(input CreateInput) (CreateInput, error) {
	input.CommandID = normalizeUUID(input.CommandID)
	input.IssuedOn = strings.TrimSpace(input.IssuedOn)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	input.Note = strings.TrimSpace(input.Note)
	input.IssuerPartyRef = normalizePartyRef(input.IssuerPartyRef)
	input.RecipientPartyRef = normalizePartyRef(input.RecipientPartyRef)
	if !ValidUUID(input.CommandID) || !validDate(input.IssuedOn) || input.AmountMinor < 1 || !currencyPattern.MatchString(input.Currency) || len([]rune(input.Note)) > 2000 || !validPartyRef(input.IssuerPartyRef) || !validPartyRef(input.RecipientPartyRef) || input.IssuerPartyRef.ID == input.RecipientPartyRef.ID {
		return CreateInput{}, fmt.Errorf("invalid invoice registration creation")
	}
	sources, err := normalizeSources(input.SourceRefs)
	if err != nil {
		return CreateInput{}, err
	}
	input.SourceRefs = sources
	return input, nil
}

func NormalizeCorrect(input CorrectInput) (CorrectInput, error) {
	input.CommandID = normalizeUUID(input.CommandID)
	input.Reason = strings.TrimSpace(input.Reason)
	if !ValidUUID(input.CommandID) || input.ExpectedRevision < 1 || !validText(input.Reason, 1, 1000) || !input.Patch.hasAny() {
		return CorrectInput{}, fmt.Errorf("invalid invoice registration correction")
	}
	if input.Patch.IssuedOn != nil {
		value := strings.TrimSpace(*input.Patch.IssuedOn)
		if !validDate(value) {
			return CorrectInput{}, fmt.Errorf("invalid issued date")
		}
		input.Patch.IssuedOn = &value
	}
	if input.Patch.AmountMinor != nil && *input.Patch.AmountMinor < 1 {
		return CorrectInput{}, fmt.Errorf("invalid amount")
	}
	if input.Patch.Currency != nil {
		value := strings.ToUpper(strings.TrimSpace(*input.Patch.Currency))
		if !currencyPattern.MatchString(value) {
			return CorrectInput{}, fmt.Errorf("invalid currency")
		}
		input.Patch.Currency = &value
	}
	if input.Patch.IssuerPartyRef != nil {
		value := normalizePartyRef(*input.Patch.IssuerPartyRef)
		if !validPartyRef(value) {
			return CorrectInput{}, fmt.Errorf("invalid issuer party reference")
		}
		input.Patch.IssuerPartyRef = &value
	}
	if input.Patch.RecipientPartyRef != nil {
		value := normalizePartyRef(*input.Patch.RecipientPartyRef)
		if !validPartyRef(value) {
			return CorrectInput{}, fmt.Errorf("invalid recipient party reference")
		}
		input.Patch.RecipientPartyRef = &value
	}
	if input.Patch.SourceRefs != nil {
		value, err := normalizeSources(*input.Patch.SourceRefs)
		if err != nil {
			return CorrectInput{}, err
		}
		input.Patch.SourceRefs = &value
	}
	if input.Patch.Note != nil {
		value := strings.TrimSpace(*input.Patch.Note)
		if len([]rune(value)) > 2000 {
			return CorrectInput{}, fmt.Errorf("invalid note")
		}
		input.Patch.Note = &value
	}
	return input, nil
}

func NormalizeInvalidate(input InvalidateInput) (InvalidateInput, error) {
	input.CommandID = normalizeUUID(input.CommandID)
	input.Reason = strings.TrimSpace(input.Reason)
	if !ValidUUID(input.CommandID) || input.ExpectedRevision < 1 || !validText(input.Reason, 1, 1000) {
		return InvalidateInput{}, fmt.Errorf("invalid invoice registration invalidation")
	}
	return input, nil
}

func NormalizeEvidence(input EvidenceInput) (EvidenceInput, error) {
	input.CommandID = normalizeUUID(input.CommandID)
	input.Reason = strings.TrimSpace(input.Reason)
	if !ValidUUID(input.CommandID) || input.ExpectedRevision < 1 || !validText(input.Reason, 1, 1000) {
		return EvidenceInput{}, fmt.Errorf("invalid invoice evidence change")
	}
	if input.EvidenceRefs == nil {
		input.EvidenceRefs = []EvidenceRef{}
	}
	seen := map[string]struct{}{}
	for index := range input.EvidenceRefs {
		value := &input.EvidenceRefs[index]
		value.FileID = normalizeUUID(value.FileID)
		value.Purpose = strings.ToLower(strings.TrimSpace(value.Purpose))
		value.DisplayName = strings.TrimSpace(value.DisplayName)
		key := fmt.Sprintf("%s:%d", value.FileID, value.Version)
		if !ValidUUID(value.FileID) || value.Version < 1 || (value.Purpose != "invoice_document" && value.Purpose != "supporting_material") || len([]rune(value.DisplayName)) > 255 {
			return EvidenceInput{}, fmt.Errorf("invalid evidence reference")
		}
		if _, exists := seen[key]; exists {
			return EvidenceInput{}, fmt.Errorf("duplicate evidence reference")
		}
		seen[key] = struct{}{}
	}
	return input, nil
}

func (patch CorrectionPatch) hasAny() bool {
	return patch.IssuedOn != nil || patch.AmountMinor != nil || patch.Currency != nil || patch.IssuerPartyRef != nil || patch.RecipientPartyRef != nil || patch.SourceRefs != nil || patch.Note != nil
}

func normalizeSources(values []SourceRef) ([]SourceRef, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("at least one source is required")
	}
	result := make([]SourceRef, len(values))
	for index, source := range values {
		source.Kind = strings.ToLower(strings.TrimSpace(source.Kind))
		if !ValidSourceKind(source.Kind) {
			return nil, fmt.Errorf("invalid source kind")
		}
		if source.Kind == SourceManualConfirmation {
			if source.ResourceID != nil || source.SourceRevision != nil || source.ManualSummary == nil {
				return nil, fmt.Errorf("invalid manual source")
			}
			summary := strings.TrimSpace(*source.ManualSummary)
			if !validText(summary, 1, 500) {
				return nil, fmt.Errorf("invalid manual source summary")
			}
			source.ManualSummary = &summary
		} else {
			if source.ResourceID == nil || source.SourceRevision == nil || source.ManualSummary != nil {
				return nil, fmt.Errorf("invalid typed source")
			}
			resourceID := normalizeUUID(*source.ResourceID)
			if !ValidUUID(resourceID) || *source.SourceRevision < 1 {
				return nil, fmt.Errorf("invalid typed source reference")
			}
			source.ResourceID = &resourceID
		}
		result[index] = source
	}
	return result, nil
}

func normalizePartyRef(value PartyRef) PartyRef {
	value.ID = normalizeUUID(value.ID)
	return value
}

func validPartyRef(value PartyRef) bool { return ValidUUID(value.ID) && value.Revision > 0 }

func ValidUUID(value string) bool { return uuidPattern.MatchString(normalizeUUID(value)) }
func ValidSourceKind(value string) bool {
	switch value {
	case SourceManualConfirmation, SourceProject, SourceContract, SourceSettlement, SourceMoneyObligation:
		return true
	default:
		return false
	}
}
func validDate(value string) bool {
	_, err := time.Parse("2006-01-02", value)
	return err == nil
}
func normalizeUUID(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
func validText(value string, min, max int) bool {
	n := len([]rune(value))
	return n >= min && n <= max
}
