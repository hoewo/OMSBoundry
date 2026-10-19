package results

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	KindApproval = "approval"
	KindAudit    = "audit"

	StateRecorded  = "recorded"
	StateWithdrawn = "withdrawn"

	AmountProfileReferenceOAV1 = "reference-oa-v1"

	ComponentDesignFee       = "design_fee"
	ComponentSurveyFee       = "survey_fee"
	ComponentConsultationFee = "consultation_fee"

	EventRecorded   = "recorded"
	EventCorrected  = "corrected"
	EventWithdrawn  = "withdrawn"
	EventReregister = "reregistered"
)

var (
	ErrNotFound             = errors.New("approval or audit result not found")
	ErrRevisionConflict     = errors.New("approval or audit result revision conflict")
	ErrCommandConflict      = errors.New("command payload conflict")
	ErrAlreadyRecorded      = errors.New("approval or audit result is already recorded")
	ErrResultWithdrawn      = errors.New("approval or audit result is withdrawn")
	ErrNoEffectiveChange    = errors.New("no effective change")
	ErrReferenceInvalid     = errors.New("reference invalid")
	ErrReferenceUnavailable = errors.New("reference unavailable")
	uuidPattern             = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	currencyPattern         = regexp.MustCompile(`^[A-Z]{3}$`)
)

type Actor struct {
	ID           string `json:"id"`
	TenantID     string `json:"tenant_id"`
	Kind         string `json:"kind"`
	CredentialID string `json:"credential_id"`
}

type ProjectRef struct {
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
}

type ProjectSnapshot struct {
	ID          string `json:"id"`
	Revision    int64  `json:"revision"`
	Code        string `json:"code"`
	DisplayName string `json:"display_name"`
}

type ProjectReference struct {
	ProjectSnapshot
	TenantID string `json:"tenant_id"`
	Status   string `json:"status"`
}

type AmountComponent struct {
	Kind        string `json:"kind"`
	AmountMinor int64  `json:"amount_minor"`
}

type SourceRef struct {
	Application  string `json:"application"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	Revision     int64  `json:"revision"`
}

type SourceSnapshot struct {
	SourceRef
	State            string            `json:"state"`
	ProjectID        string            `json:"project_id"`
	Currency         string            `json:"currency"`
	AmountProfile    string            `json:"amount_profile"`
	AmountComponents []AmountComponent `json:"amount_components"`
	TotalAmountMinor int64             `json:"total_amount_minor"`
	Digest           string            `json:"digest"`
	DisplaySummary   string            `json:"display_summary"`
}

type SourceReference struct {
	SourceSnapshot
	TenantID string `json:"tenant_id"`
	Eligible bool   `json:"eligible"`
}

type EvidenceRef struct {
	FileID  string `json:"file_id"`
	Version int64  `json:"version"`
	Slot    string `json:"slot"`
}

type EvidenceSnapshot struct {
	EvidenceRef
	Digest      string `json:"digest"`
	DisplayName string `json:"display_name"`
}

type EvidenceReference struct {
	EvidenceSnapshot
	TenantID string `json:"tenant_id"`
	Status   string `json:"status"`
}

type Result struct {
	ID                  string             `json:"id"`
	TenantID            string             `json:"tenant_id"`
	Project             ProjectSnapshot    `json:"project"`
	Kind                string             `json:"kind"`
	State               string             `json:"state"`
	AmountProfile       string             `json:"amount_profile"`
	Currency            string             `json:"currency"`
	AmountComponents    []AmountComponent  `json:"amount_components"`
	TotalAmountMinor    int64              `json:"total_amount_minor"`
	Source              SourceSnapshot     `json:"source"`
	Evidence            []EvidenceSnapshot `json:"evidence"`
	OverrideReason      string             `json:"override_reason,omitempty"`
	ExternalConfirmedBy string             `json:"external_confirmed_by"`
	ExternalConfirmedAt time.Time          `json:"external_confirmed_at"`
	Revision            int64              `json:"revision"`
	CreatedAt           time.Time          `json:"created_at"`
	UpdatedAt           time.Time          `json:"updated_at"`
}

type Revision struct {
	ResultID   string    `json:"result_id"`
	TenantID   string    `json:"tenant_id"`
	Revision   int64     `json:"revision"`
	EventKind  string    `json:"event_kind"`
	CommandID  string    `json:"command_id"`
	Snapshot   Result    `json:"snapshot"`
	Reason     string    `json:"reason"`
	ActorRef   string    `json:"actor_ref"`
	OccurredAt time.Time `json:"occurred_at"`
}

// ProjectResultReference is the minimum, source-owned projection consumed by
// Project Workspace completion evaluation. It deliberately excludes amounts,
// evidence details and mutable domain objects.
type ProjectResultReference struct {
	ID             string `json:"id"`
	TenantID       string `json:"tenant_id"`
	ProjectID      string `json:"project_id"`
	ResourceType   string `json:"resource_type"`
	Kind           string `json:"kind"`
	State          string `json:"state"`
	Revision       int64  `json:"revision"`
	Digest         string `json:"digest"`
	DisplaySummary string `json:"display_summary"`
}

type ProjectResultReferenceList struct {
	SchemaVersion     string                   `json:"schema_version"`
	SourceApplication string                   `json:"source_application"`
	ProjectID         string                   `json:"project_id"`
	Items             []ProjectResultReference `json:"items"`
}

type RecordInput struct {
	CommandID           string            `json:"command_id"`
	ExpectedRevision    int64             `json:"expected_revision"`
	ProjectRef          ProjectRef        `json:"project_ref"`
	Kind                string            `json:"kind"`
	Currency            string            `json:"currency"`
	AmountComponents    []AmountComponent `json:"amount_components,omitempty"`
	SourceRef           SourceRef         `json:"source_ref"`
	EvidenceRefs        []EvidenceRef     `json:"evidence_refs,omitempty"`
	OverrideReason      string            `json:"override_reason,omitempty"`
	ExternalConfirmedBy string            `json:"external_confirmed_by"`
	ExternalConfirmedAt time.Time         `json:"external_confirmed_at"`
}

type CorrectInput struct {
	CommandID           string            `json:"command_id"`
	ExpectedRevision    int64             `json:"expected_revision"`
	Reason              string            `json:"reason"`
	Currency            string            `json:"currency"`
	AmountComponents    []AmountComponent `json:"amount_components,omitempty"`
	SourceRef           SourceRef         `json:"source_ref"`
	EvidenceRefs        []EvidenceRef     `json:"evidence_refs,omitempty"`
	OverrideReason      string            `json:"override_reason,omitempty"`
	ExternalConfirmedBy string            `json:"external_confirmed_by"`
	ExternalConfirmedAt time.Time         `json:"external_confirmed_at"`
}

type WithdrawInput struct {
	CommandID        string `json:"command_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Reason           string `json:"reason"`
}

type ResolvedResult struct {
	Project             ProjectSnapshot
	Currency            string
	AmountComponents    []AmountComponent
	TotalAmountMinor    int64
	Source              SourceSnapshot
	Evidence            []EvidenceSnapshot
	OverrideReason      string
	ExternalConfirmedBy string
	ExternalConfirmedAt time.Time
}

func NormalizeActor(actor Actor) (Actor, error) {
	actor.ID = normalizeUUID(actor.ID)
	actor.TenantID = normalizeUUID(actor.TenantID)
	actor.Kind = strings.TrimSpace(actor.Kind)
	actor.CredentialID = strings.TrimSpace(actor.CredentialID)
	if !ValidUUID(actor.ID) || !ValidUUID(actor.TenantID) || !validText(actor.Kind, 1, 100) || !validText(actor.CredentialID, 1, 256) {
		return Actor{}, fmt.Errorf("invalid actor")
	}
	return actor, nil
}

func NormalizeRecord(input RecordInput) (RecordInput, error) {
	input.CommandID = normalizeUUID(input.CommandID)
	input.ProjectRef.ID = normalizeUUID(input.ProjectRef.ID)
	input.Kind = strings.ToLower(strings.TrimSpace(input.Kind))
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	input.OverrideReason = strings.TrimSpace(input.OverrideReason)
	input.ExternalConfirmedBy = strings.TrimSpace(input.ExternalConfirmedBy)
	input.SourceRef = normalizeSourceRef(input.SourceRef)
	if !ValidUUID(input.CommandID) || input.ExpectedRevision < 0 || !validProjectRef(input.ProjectRef) || !validKind(input.Kind) || !currencyPattern.MatchString(input.Currency) || !validText(input.ExternalConfirmedBy, 1, 200) || input.ExternalConfirmedAt.IsZero() || len([]rune(input.OverrideReason)) > 1000 {
		return RecordInput{}, fmt.Errorf("invalid result registration")
	}
	if err := validateSourceRefForKind(input.Kind, input.SourceRef); err != nil {
		return RecordInput{}, err
	}
	components, _, err := NormalizeComponents(input.AmountComponents, true)
	if err != nil {
		return RecordInput{}, err
	}
	input.AmountComponents = components
	evidence, err := normalizeEvidenceRefs(input.EvidenceRefs)
	if err != nil {
		return RecordInput{}, err
	}
	input.EvidenceRefs = evidence
	input.ExternalConfirmedAt = input.ExternalConfirmedAt.UTC()
	return input, nil
}

func NormalizeCorrect(input CorrectInput, kind string) (CorrectInput, error) {
	input.CommandID = normalizeUUID(input.CommandID)
	input.Reason = strings.TrimSpace(input.Reason)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	input.OverrideReason = strings.TrimSpace(input.OverrideReason)
	input.ExternalConfirmedBy = strings.TrimSpace(input.ExternalConfirmedBy)
	input.SourceRef = normalizeSourceRef(input.SourceRef)
	if !ValidUUID(input.CommandID) || input.ExpectedRevision < 1 || !validText(input.Reason, 1, 1000) || !currencyPattern.MatchString(input.Currency) || !validText(input.ExternalConfirmedBy, 1, 200) || input.ExternalConfirmedAt.IsZero() || len([]rune(input.OverrideReason)) > 1000 {
		return CorrectInput{}, fmt.Errorf("invalid result correction")
	}
	if err := validateSourceRefForKind(kind, input.SourceRef); err != nil {
		return CorrectInput{}, err
	}
	components, _, err := NormalizeComponents(input.AmountComponents, true)
	if err != nil {
		return CorrectInput{}, err
	}
	input.AmountComponents = components
	evidence, err := normalizeEvidenceRefs(input.EvidenceRefs)
	if err != nil {
		return CorrectInput{}, err
	}
	input.EvidenceRefs = evidence
	input.ExternalConfirmedAt = input.ExternalConfirmedAt.UTC()
	return input, nil
}

func NormalizeWithdraw(input WithdrawInput) (WithdrawInput, error) {
	input.CommandID = normalizeUUID(input.CommandID)
	input.Reason = strings.TrimSpace(input.Reason)
	if !ValidUUID(input.CommandID) || input.ExpectedRevision < 1 || !validText(input.Reason, 1, 1000) {
		return WithdrawInput{}, fmt.Errorf("invalid result withdrawal")
	}
	return input, nil
}

func NormalizeComponents(values []AmountComponent, allowEmpty bool) ([]AmountComponent, int64, error) {
	if len(values) == 0 && allowEmpty {
		return nil, 0, nil
	}
	if len(values) != 3 {
		return nil, 0, fmt.Errorf("amount profile requires exactly three components")
	}
	allowed := map[string]bool{ComponentDesignFee: true, ComponentSurveyFee: true, ComponentConsultationFee: true}
	seen := make(map[string]bool, 3)
	result := make([]AmountComponent, 0, 3)
	var total int64
	for _, value := range values {
		value.Kind = strings.ToLower(strings.TrimSpace(value.Kind))
		if !allowed[value.Kind] || seen[value.Kind] || value.AmountMinor < 0 {
			return nil, 0, fmt.Errorf("invalid amount component")
		}
		if value.AmountMinor > math.MaxInt64-total {
			return nil, 0, fmt.Errorf("amount total overflows int64")
		}
		seen[value.Kind] = true
		total += value.AmountMinor
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Kind < result[j].Kind })
	return result, total, nil
}

func validKind(kind string) bool { return kind == KindApproval || kind == KindAudit }

func validateSourceRefForKind(kind string, value SourceRef) error {
	if !ValidUUID(value.ResourceID) || value.Revision < 1 || !validText(value.Application, 1, 100) || !validText(value.ResourceType, 1, 100) {
		return ErrReferenceInvalid
	}
	if kind == KindApproval {
		if value.Application != "contract-ledger" || (value.ResourceType != "contract" && value.ResourceType != "settlement") {
			return ErrReferenceInvalid
		}
		return nil
	}
	if value.Application != "approval-audit-results" || value.ResourceType != "approval-result" {
		return ErrReferenceInvalid
	}
	return nil
}

func normalizeSourceRef(value SourceRef) SourceRef {
	value.Application = strings.ToLower(strings.TrimSpace(value.Application))
	value.ResourceType = strings.ToLower(strings.TrimSpace(value.ResourceType))
	value.ResourceID = normalizeUUID(value.ResourceID)
	return value
}

func normalizeEvidenceRefs(values []EvidenceRef) ([]EvidenceRef, error) {
	result := make([]EvidenceRef, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value.FileID = normalizeUUID(value.FileID)
		value.Slot = strings.TrimSpace(value.Slot)
		key := fmt.Sprintf("%s:%d:%s", value.FileID, value.Version, value.Slot)
		if !ValidUUID(value.FileID) || value.Version < 1 || !validText(value.Slot, 1, 100) || seen[key] {
			return nil, ErrReferenceInvalid
		}
		seen[key] = true
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Slot != result[j].Slot {
			return result[i].Slot < result[j].Slot
		}
		if result[i].FileID != result[j].FileID {
			return result[i].FileID < result[j].FileID
		}
		return result[i].Version < result[j].Version
	})
	return result, nil
}

func normalizeUUID(value string) string     { return strings.ToLower(strings.TrimSpace(value)) }
func ValidUUID(value string) bool           { return uuidPattern.MatchString(normalizeUUID(value)) }
func validProjectRef(value ProjectRef) bool { return ValidUUID(value.ID) && value.Revision > 0 }
func validText(value string, min, max int) bool {
	count := len([]rune(strings.TrimSpace(value)))
	return count >= min && count <= max
}
