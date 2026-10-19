package costoutsourcing

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	StateRecorded  = "recorded"
	StateWithdrawn = "withdrawn"

	CategoryVehicle         = "vehicle"
	CategoryAccommodation   = "accommodation"
	CategoryPublicTransport = "public_transport"
	CategoryOther           = "other"

	AssessmentRuleReferenceOAFivePointV1 = "reference-oa-five-point-v1"

	ActionCostCreate           = "cost-entry.create"
	ActionCostRead             = "cost-entry.read"
	ActionCostAmountRead       = "cost-entry.amount.read"
	ActionCostCorrect          = "cost-entry.correct"
	ActionCostWithdraw         = "cost-entry.withdraw"
	ActionCostEvidenceManage   = "cost-entry.evidence.manage"
	ActionCostHistory          = "cost-entry.history.read"
	ActionEngageCreate         = "outsourcing-engagement.create"
	ActionEngageRead           = "outsourcing-engagement.read"
	ActionEngageAmountRead     = "outsourcing-engagement.amount.read"
	ActionEngageCorrect        = "outsourcing-engagement.correct"
	ActionEngageWithdraw       = "outsourcing-engagement.withdraw"
	ActionEngageAssess         = "outsourcing-engagement.assess"
	ActionEngageEvidenceManage = "outsourcing-engagement.evidence.manage"
	ActionEngageHistory        = "outsourcing-engagement.history.read"
	ActionSuggestionCreate     = "outsourcing-payable-suggestion.create"
	ActionSuggestionRead       = "outsourcing-payable-suggestion.read"
	ActionSuggestionWithdraw   = "outsourcing-payable-suggestion.withdraw"

	SuggestionProposed          = "proposed"
	SuggestionAccepted          = "accepted"
	SuggestionRejected          = "rejected"
	SuggestionWithdrawn         = "withdrawn"
	SuggestionTargetUnavailable = "target_unavailable"
)

var (
	ErrNotFound             = errors.New("cost or outsourcing fact not found")
	ErrRevisionConflict     = errors.New("revision conflict")
	ErrCommandConflict      = errors.New("command payload conflict")
	ErrWithdrawn            = errors.New("aggregate is withdrawn")
	ErrForbidden            = errors.New("action forbidden")
	ErrReferenceInvalid     = errors.New("reference invalid")
	ErrReferenceUnavailable = errors.New("reference unavailable")
	ErrTargetUnavailable    = errors.New("Money payable suggestion target unavailable")
	uuidPattern             = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	currencyPattern         = regexp.MustCompile(`^[A-Z]{3}$`)
	datePattern             = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

type Actor struct {
	ID             string          `json:"id"`
	TenantID       string          `json:"tenant_id"`
	Kind           string          `json:"kind"`
	CredentialID   string          `json:"credential_id"`
	TenantScope    bool            `json:"tenant_scope"`
	ProjectIDs     []string        `json:"project_ids"`
	ResourceScopes []ResourceScope `json:"resource_scopes"`
	Actions        []string        `json:"actions"`
}

type ResourceScope struct {
	Type string `json:"type"`
	ID   string `json:"id"`
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

type PartyRef struct {
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
}

type PartySnapshot struct {
	ID        string `json:"id"`
	Revision  int64  `json:"revision"`
	Kind      string `json:"kind"`
	LegalName string `json:"legal_name"`
}

type PartyReference struct {
	PartySnapshot
	TenantID string `json:"tenant_id"`
	Status   string `json:"status"`
}

type ContractRef struct {
	Application  string `json:"application"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	Revision     int64  `json:"revision"`
}

type ContractSnapshot struct {
	ContractRef
	ProjectID      string `json:"project_id"`
	PartyID        string `json:"party_id"`
	State          string `json:"state"`
	Currency       string `json:"currency"`
	AmountMinor    int64  `json:"amount_minor"`
	Digest         string `json:"digest"`
	DisplaySummary string `json:"display_summary"`
}

type ContractReference struct {
	ContractSnapshot
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

type CostEntry struct {
	ID             string             `json:"id"`
	TenantID       string             `json:"tenant_id"`
	Project        ProjectSnapshot    `json:"project"`
	Category       string             `json:"category"`
	OccurredOn     string             `json:"occurred_on"`
	Currency       string             `json:"currency"`
	AmountMinor    int64              `json:"amount_minor"`
	DistanceMeters int64              `json:"distance_meters,omitempty"`
	Description    string             `json:"description"`
	Evidence       []EvidenceSnapshot `json:"evidence"`
	State          string             `json:"state"`
	Revision       int64              `json:"revision"`
	CreatedBy      string             `json:"created_by"`
	UpdatedBy      string             `json:"updated_by"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

type Engagement struct {
	ID           string             `json:"id"`
	TenantID     string             `json:"tenant_id"`
	Project      ProjectSnapshot    `json:"project"`
	Provider     PartySnapshot      `json:"provider"`
	ContactName  string             `json:"contact_name,omitempty"`
	ContactPhone string             `json:"contact_phone,omitempty"`
	Scope        string             `json:"scope"`
	Currency     string             `json:"currency"`
	AmountMinor  int64              `json:"amount_minor"`
	Contract     *ContractSnapshot  `json:"contract,omitempty"`
	Evidence     []EvidenceSnapshot `json:"evidence"`
	Note         string             `json:"note,omitempty"`
	State        string             `json:"state"`
	Revision     int64              `json:"revision"`
	CreatedBy    string             `json:"created_by"`
	UpdatedBy    string             `json:"updated_by"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

type Assessment struct {
	ID                 string    `json:"id"`
	TenantID           string    `json:"tenant_id"`
	EngagementID       string    `json:"engagement_id"`
	EngagementRevision int64     `json:"engagement_revision"`
	RuleKey            string    `json:"rule_key"`
	Score              int       `json:"score"`
	Reason             string    `json:"reason"`
	Revision           int64     `json:"revision"`
	ActorRef           string    `json:"actor_ref"`
	OccurredAt         time.Time `json:"occurred_at"`
}

type CostRevision struct {
	CostEntryID string    `json:"cost_entry_id"`
	TenantID    string    `json:"tenant_id"`
	Revision    int64     `json:"revision"`
	EventKind   string    `json:"event_kind"`
	CommandID   string    `json:"command_id"`
	Snapshot    CostEntry `json:"snapshot"`
	Reason      string    `json:"reason"`
	ActorRef    string    `json:"actor_ref"`
	OccurredAt  time.Time `json:"occurred_at"`
}

type EngagementRevision struct {
	EngagementID string     `json:"engagement_id"`
	TenantID     string     `json:"tenant_id"`
	Revision     int64      `json:"revision"`
	EventKind    string     `json:"event_kind"`
	CommandID    string     `json:"command_id"`
	Snapshot     Engagement `json:"snapshot"`
	Reason       string     `json:"reason"`
	ActorRef     string     `json:"actor_ref"`
	OccurredAt   time.Time  `json:"occurred_at"`
}

type PayableSourceSnapshot struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
	Digest   string `json:"digest"`
}

type ObligationReference struct {
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
}

type PayableSuggestion struct {
	ID             string                `json:"id"`
	TenantID       string                `json:"tenant_id"`
	Project        ProjectSnapshot       `json:"project"`
	Source         PayableSourceSnapshot `json:"source"`
	LedgerOwner    PartySnapshot         `json:"ledger_owner"`
	Counterparty   PartySnapshot         `json:"counterparty"`
	AmountMinor    int64                 `json:"amount_minor"`
	Currency       string                `json:"currency"`
	DueOn          string                `json:"due_on"`
	Purpose        string                `json:"purpose"`
	State          string                `json:"state"`
	Revision       int64                 `json:"revision"`
	TargetRevision int64                 `json:"target_revision,omitempty"`
	RequestDigest  string                `json:"request_digest"`
	Obligation     *ObligationReference  `json:"obligation,omitempty"`
	DecisionReason string                `json:"decision_reason,omitempty"`
	CreatedBy      string                `json:"created_by"`
	UpdatedBy      string                `json:"updated_by"`
	CreatedAt      time.Time             `json:"created_at"`
	UpdatedAt      time.Time             `json:"updated_at"`
}

// PayableSubmission and PayableIntake are C6's port types. The HTTP adapter is
// responsible for translating them to the versioned shared wire contract.
type PayableSubmission struct {
	SuggestionID  string
	TenantID      string
	ProjectID     string
	Source        PayableSourceSnapshot
	LedgerOwner   PartySnapshot
	Counterparty  PartySnapshot
	AmountMinor   int64
	Currency      string
	DueOn         string
	Purpose       string
	RequestDigest string
}

type PayableIntake struct {
	SuggestionID   string
	TenantID       string
	RequestDigest  string
	Status         string
	Revision       int64
	Obligation     *ObligationReference
	DecisionReason string
}

type PayableSuggestionRevision struct {
	SuggestionID string            `json:"suggestion_id"`
	TenantID     string            `json:"tenant_id"`
	Revision     int64             `json:"revision"`
	EventKind    string            `json:"event_kind"`
	CommandID    string            `json:"command_id"`
	Snapshot     PayableSuggestion `json:"snapshot"`
	Reason       string            `json:"reason"`
	ActorRef     string            `json:"actor_ref"`
	OccurredAt   time.Time         `json:"occurred_at"`
}

type ProposeSuggestionInput struct {
	CommandID       string    `json:"command_id"`
	SourceType      string    `json:"source_type"`
	SourceID        string    `json:"source_id"`
	SourceRevision  int64     `json:"source_revision"`
	LedgerOwnerRef  PartyRef  `json:"ledger_owner_ref"`
	CounterpartyRef *PartyRef `json:"counterparty_ref,omitempty"`
	DueOn           string    `json:"due_on"`
	Purpose         string    `json:"purpose"`
}

type SuggestionCommandInput struct {
	CommandID        string `json:"command_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Reason           string `json:"reason,omitempty"`
}

type RecordCostInput struct {
	CommandID      string        `json:"command_id"`
	ProjectRef     ProjectRef    `json:"project_ref"`
	Category       string        `json:"category"`
	OccurredOn     string        `json:"occurred_on"`
	Currency       string        `json:"currency"`
	AmountMinor    int64         `json:"amount_minor"`
	DistanceMeters int64         `json:"distance_meters,omitempty"`
	Description    string        `json:"description"`
	EvidenceRefs   []EvidenceRef `json:"evidence_refs,omitempty"`
}

type CorrectCostInput struct {
	CommandID        string        `json:"command_id"`
	ExpectedRevision int64         `json:"expected_revision"`
	Reason           string        `json:"reason"`
	Category         string        `json:"category"`
	OccurredOn       string        `json:"occurred_on"`
	Currency         string        `json:"currency"`
	AmountMinor      int64         `json:"amount_minor"`
	DistanceMeters   int64         `json:"distance_meters,omitempty"`
	Description      string        `json:"description"`
	EvidenceRefs     []EvidenceRef `json:"evidence_refs,omitempty"`
}

type RecordEngagementInput struct {
	CommandID    string        `json:"command_id"`
	ProjectRef   ProjectRef    `json:"project_ref"`
	ProviderRef  PartyRef      `json:"provider_ref"`
	ContactName  string        `json:"contact_name,omitempty"`
	ContactPhone string        `json:"contact_phone,omitempty"`
	Scope        string        `json:"scope"`
	Currency     string        `json:"currency"`
	AmountMinor  int64         `json:"amount_minor"`
	ContractRef  *ContractRef  `json:"contract_ref,omitempty"`
	EvidenceRefs []EvidenceRef `json:"evidence_refs,omitempty"`
	Note         string        `json:"note,omitempty"`
}

type CorrectEngagementInput struct {
	CommandID        string        `json:"command_id"`
	ExpectedRevision int64         `json:"expected_revision"`
	Reason           string        `json:"reason"`
	ProviderRef      PartyRef      `json:"provider_ref"`
	ContactName      string        `json:"contact_name,omitempty"`
	ContactPhone     string        `json:"contact_phone,omitempty"`
	Scope            string        `json:"scope"`
	Currency         string        `json:"currency"`
	AmountMinor      int64         `json:"amount_minor"`
	ContractRef      *ContractRef  `json:"contract_ref,omitempty"`
	EvidenceRefs     []EvidenceRef `json:"evidence_refs,omitempty"`
	Note             string        `json:"note,omitempty"`
}

type WithdrawInput struct {
	CommandID        string `json:"command_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Reason           string `json:"reason"`
}

type AssessInput struct {
	CommandID                  string `json:"command_id"`
	ExpectedEngagementRevision int64  `json:"expected_engagement_revision"`
	ExpectedAssessmentRevision int64  `json:"expected_assessment_revision"`
	Score                      int    `json:"score"`
	Reason                     string `json:"reason"`
}

func normalizeActor(value Actor) (Actor, error) {
	value.ID, value.TenantID = normalizeUUID(value.ID), normalizeUUID(value.TenantID)
	value.Kind, value.CredentialID = strings.TrimSpace(value.Kind), strings.TrimSpace(value.CredentialID)
	value.Actions = uniqueStrings(value.Actions)
	value.ProjectIDs = append([]string(nil), value.ProjectIDs...)
	for i := range value.ProjectIDs {
		value.ProjectIDs[i] = normalizeUUID(value.ProjectIDs[i])
		if !validUUID(value.ProjectIDs[i]) {
			return Actor{}, fmt.Errorf("invalid actor project scope")
		}
	}
	value.ProjectIDs = uniqueStrings(value.ProjectIDs)
	value.ResourceScopes = append([]ResourceScope(nil), value.ResourceScopes...)
	seenScopes := map[string]bool{}
	for i := range value.ResourceScopes {
		value.ResourceScopes[i].Type = strings.TrimSpace(value.ResourceScopes[i].Type)
		value.ResourceScopes[i].ID = normalizeUUID(value.ResourceScopes[i].ID)
		key := value.ResourceScopes[i].Type + ":" + value.ResourceScopes[i].ID
		if (value.ResourceScopes[i].Type != "cost_entry" && value.ResourceScopes[i].Type != "outsourcing_engagement" && value.ResourceScopes[i].Type != "payable_suggestion") || !validUUID(value.ResourceScopes[i].ID) || seenScopes[key] {
			return Actor{}, fmt.Errorf("invalid actor resource scope")
		}
		seenScopes[key] = true
	}
	sort.Slice(value.ResourceScopes, func(i, j int) bool {
		if value.ResourceScopes[i].Type == value.ResourceScopes[j].Type {
			return value.ResourceScopes[i].ID < value.ResourceScopes[j].ID
		}
		return value.ResourceScopes[i].Type < value.ResourceScopes[j].Type
	})
	if !validUUID(value.ID) || !validUUID(value.TenantID) || !validText(value.Kind, 1, 100) || !validText(value.CredentialID, 1, 256) {
		return Actor{}, fmt.Errorf("invalid actor")
	}
	return value, nil
}

func normalizeCostFields(category, occurredOn, currency string, amountMinor, distanceMeters int64, description string) (string, string, string, string, error) {
	category = strings.ToLower(strings.TrimSpace(category))
	occurredOn = strings.TrimSpace(occurredOn)
	currency = strings.ToUpper(strings.TrimSpace(currency))
	description = strings.TrimSpace(description)
	if category != CategoryVehicle && category != CategoryAccommodation && category != CategoryPublicTransport && category != CategoryOther {
		return "", "", "", "", fmt.Errorf("invalid cost category")
	}
	if !datePattern.MatchString(occurredOn) {
		return "", "", "", "", fmt.Errorf("invalid occurrence date")
	}
	if _, err := time.Parse("2006-01-02", occurredOn); err != nil {
		return "", "", "", "", fmt.Errorf("invalid occurrence date")
	}
	if !currencyPattern.MatchString(currency) || amountMinor < 0 || !validText(description, 1, 1000) {
		return "", "", "", "", fmt.Errorf("invalid exact cost")
	}
	if (category == CategoryVehicle && distanceMeters <= 0) || (category != CategoryVehicle && distanceMeters != 0) {
		return "", "", "", "", fmt.Errorf("distance is only valid and required for vehicle cost")
	}
	return category, occurredOn, currency, description, nil
}

func normalizeEngagementFields(scope, currency string, amountMinor int64, note string) (string, string, string, error) {
	scope, currency, note = strings.TrimSpace(scope), strings.ToUpper(strings.TrimSpace(currency)), strings.TrimSpace(note)
	if !validText(scope, 1, 2000) || !currencyPattern.MatchString(currency) || amountMinor < 0 || len([]rune(note)) > 2000 {
		return "", "", "", fmt.Errorf("invalid outsourcing engagement")
	}
	return scope, currency, note, nil
}

func normalizeEngagementContact(name, phone string) (string, string, error) {
	name, phone = strings.TrimSpace(name), strings.TrimSpace(phone)
	if len([]rune(name)) > 100 || len([]rune(phone)) > 50 {
		return "", "", fmt.Errorf("invalid outsourcing contact snapshot")
	}
	return name, phone, nil
}

func normalizeProjectRef(value ProjectRef) (ProjectRef, error) {
	value.ID = normalizeUUID(value.ID)
	if !validUUID(value.ID) || value.Revision < 0 {
		return ProjectRef{}, fmt.Errorf("invalid project reference")
	}
	return value, nil
}

func normalizePartyRef(value PartyRef) (PartyRef, error) {
	value.ID = normalizeUUID(value.ID)
	if !validUUID(value.ID) || value.Revision <= 0 {
		return PartyRef{}, fmt.Errorf("invalid party reference")
	}
	return value, nil
}

func normalizeContractRef(value ContractRef) (ContractRef, error) {
	value.Application = strings.TrimSpace(value.Application)
	value.ResourceType = strings.TrimSpace(value.ResourceType)
	value.ResourceID = normalizeUUID(value.ResourceID)
	if !validText(value.Application, 1, 100) || !validText(value.ResourceType, 1, 100) || !validUUID(value.ResourceID) || value.Revision <= 0 {
		return ContractRef{}, fmt.Errorf("invalid contract reference")
	}
	return value, nil
}

func normalizeEvidenceRefs(values []EvidenceRef) ([]EvidenceRef, error) {
	result := append([]EvidenceRef(nil), values...)
	seen := map[string]bool{}
	for i := range result {
		result[i].FileID = normalizeUUID(result[i].FileID)
		result[i].Slot = strings.TrimSpace(result[i].Slot)
		key := result[i].FileID + ":" + fmt.Sprint(result[i].Version) + ":" + result[i].Slot
		if !validUUID(result[i].FileID) || result[i].Version <= 0 || !validText(result[i].Slot, 1, 100) || seen[key] {
			return nil, fmt.Errorf("invalid or duplicate evidence reference")
		}
		seen[key] = true
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Slot == result[j].Slot {
			if result[i].FileID == result[j].FileID {
				return result[i].Version < result[j].Version
			}
			return result[i].FileID < result[j].FileID
		}
		return result[i].Slot < result[j].Slot
	})
	return result, nil
}

func normalizeUUID(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
func validUUID(value string) bool       { return uuidPattern.MatchString(value) }
func validText(value string, min, max int) bool {
	length := len([]rune(strings.TrimSpace(value)))
	return length >= min && length <= max
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func newUUID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	raw[6] = raw[6]&0x0f | 0x40
	raw[8] = raw[8]&0x3f | 0x80
	encoded := hex.EncodeToString(raw[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32], nil
}
