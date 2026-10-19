package engagement

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	ApplicabilityUndetermined  = "undetermined"
	ApplicabilityApplicable    = "applicable"
	ApplicabilityNotApplicable = "not_applicable"

	MethodComparison             = "comparison"
	MethodCompetitiveNegotiation = "competitive_negotiation"
	MethodTender                 = "tender"

	StatusDraft              = "draft"
	StatusCollectingEvidence = "collecting_evidence"
	StatusReady              = "ready"
	StatusNotApplicable      = "not_applicable"
	StatusCancelled          = "cancelled"

	RequirementTenderDocument = "tender_document"
	RequirementBidDocument    = "bid_document"
	RequirementAwardDocument  = "award_document"

	// Project's material-reference v1 consumer accepts at most 100 items.
	MaxActiveMaterials = 100
)

var (
	ErrNotFound             = errors.New("business engagement not found")
	ErrRevisionConflict     = errors.New("business engagement revision conflict")
	ErrProjectConflict      = errors.New("project already has a business engagement")
	ErrCommandConflict      = errors.New("command payload conflict")
	ErrStateConflict        = errors.New("business engagement state conflict")
	ErrReferenceInvalid     = errors.New("external reference invalid")
	ErrReferenceUnavailable = errors.New("external reference unavailable")
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type Actor struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenant_id"`
	DisplayName string `json:"display_name"`
	Kind        string `json:"kind"`
}

type Engagement struct {
	ID                    string    `json:"id"`
	TenantID              string    `json:"tenant_id"`
	ProjectID             string    `json:"project_id"`
	ProjectSourceRevision int64     `json:"project_source_revision"`
	Applicability         string    `json:"applicability"`
	Method                *string   `json:"method,omitempty"`
	MaterialPolicyVersion int       `json:"material_policy_version"`
	Status                string    `json:"status"`
	Revision              int64     `json:"revision"`
	NotApplicableReason   *string   `json:"not_applicable_reason,omitempty"`
	CancellationReason    *string   `json:"cancellation_reason,omitempty"`
	CreatedBy             string    `json:"created_by"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type MaterialReference struct {
	ID                           string     `json:"id"`
	TenantID                     string     `json:"tenant_id"`
	EngagementID                 string     `json:"engagement_id"`
	RequirementKey               string     `json:"requirement_key"`
	EvidenceFileID               string     `json:"evidence_file_id"`
	EvidenceVersion              int        `json:"evidence_version"`
	EvidenceSourceRevision       int64      `json:"evidence_source_revision"`
	DisplaySnapshot              string     `json:"display_snapshot"`
	CreatedEngagementRevision    int64      `json:"created_engagement_revision"`
	SupersededEngagementRevision *int64     `json:"superseded_engagement_revision,omitempty"`
	CreatedAt                    time.Time  `json:"created_at"`
	SupersededAt                 *time.Time `json:"superseded_at,omitempty"`
}

type Requirement struct {
	Key                string              `json:"key"`
	Required           bool                `json:"required"`
	Satisfied          bool                `json:"satisfied"`
	MaterialReference  *MaterialReference  `json:"material_reference,omitempty"`
	MaterialReferences []MaterialReference `json:"material_references,omitempty"`
}

type Details struct {
	Engagement          Engagement          `json:"engagement"`
	Materials           []MaterialReference `json:"materials"`
	Requirements        []Requirement       `json:"requirements"`
	MissingRequirements []string            `json:"missing_requirements"`
}

// MaterialReferenceProjection is the versioned, read-only business projection
// consumed by Project Workspace. It exposes only current Evidence references;
// Business Engagement keeps ownership of requirement meaning and history.
type MaterialReferenceProjection struct {
	SchemaVersion     string                   `json:"schema_version"`
	SourceApplication string                   `json:"source_application"`
	ID                string                   `json:"id"`
	TenantID          string                   `json:"tenant_id"`
	ProjectID         string                   `json:"project_id"`
	ResourceType      string                   `json:"resource_type"`
	Status            string                   `json:"status"`
	Revision          int64                    `json:"revision"`
	Digest            string                   `json:"digest"`
	Items             []MaterialProjectionItem `json:"items"`
}

type MaterialProjectionItem struct {
	RelationKey            string `json:"relation_key"`
	FileID                 string `json:"file_id"`
	Version                int    `json:"version"`
	EvidenceSourceRevision int64  `json:"evidence_source_revision"`
	DisplayName            string `json:"display_name"`
}

type HistoryEvent struct {
	ID                 string         `json:"id"`
	TenantID           string         `json:"tenant_id"`
	EngagementID       string         `json:"engagement_id"`
	EngagementRevision int64          `json:"engagement_revision"`
	EventSequence      int64          `json:"event_sequence"`
	EventType          string         `json:"event_type"`
	CommandID          string         `json:"command_id"`
	ActorSubject       string         `json:"actor_subject"`
	OccurredAt         time.Time      `json:"occurred_at"`
	Payload            map[string]any `json:"payload"`
}

type HistoryPage struct {
	Items             []HistoryEvent `json:"items"`
	NextAfterSequence *int64         `json:"next_after_sequence,omitempty"`
}

type CreateInput struct {
	CommandID             string `json:"command_id"`
	ProjectID             string `json:"project_id"`
	ProjectSourceRevision int64  `json:"project_source_revision"`
}

type ApplicabilityInput struct {
	CommandID        string `json:"command_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Applicable       bool   `json:"applicable"`
	Reason           string `json:"reason"`
}

type MethodInput struct {
	CommandID        string `json:"command_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Method           string `json:"method"`
}

type MaterialInput struct {
	CommandID              string `json:"command_id"`
	ExpectedRevision       int64  `json:"expected_revision"`
	Operation              string `json:"operation"`
	RequirementKey         string `json:"requirement_key"`
	EvidenceFileID         string `json:"evidence_file_id"`
	EvidenceVersion        int    `json:"evidence_version"`
	EvidenceSourceRevision int64  `json:"evidence_source_revision"`
	DisplaySnapshot        string `json:"-"`
	// Omitting the empty additive field preserves all legacy command hashes.
	MaterialReferenceID string `json:"material_reference_id,omitempty"`
}

type CancelInput struct {
	CommandID        string `json:"command_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Reason           string `json:"reason"`
}

type ProjectReference struct {
	ID           string `json:"id"`
	TenantID     string `json:"tenant_id"`
	ResourceType string `json:"resource_type"`
	Status       string `json:"status"`
	Revision     int64  `json:"revision"`
}

type EvidenceReference struct {
	TenantID    string `json:"tenant_id"`
	FileID      string `json:"file_id"`
	Version     int    `json:"version"`
	Revision    int64  `json:"revision"`
	DisplayName string `json:"display_name"`
}

func ValidUUID(value string) bool {
	return uuidPattern.MatchString(strings.ToLower(strings.TrimSpace(value)))
}

func ValidMethod(value string) bool {
	switch value {
	case MethodComparison, MethodCompetitiveNegotiation, MethodTender:
		return true
	default:
		return false
	}
}

func ValidRequirement(value string) bool {
	switch value {
	case RequirementTenderDocument, RequirementBidDocument, RequirementAwardDocument:
		return true
	default:
		return false
	}
}

func RequiredKeys(method string) []string {
	if !ValidMethod(method) {
		return nil
	}
	keys := []string{RequirementBidDocument, RequirementAwardDocument}
	if method == MethodTender {
		return append([]string{RequirementTenderDocument}, keys...)
	}
	return keys
}

func BuildDetails(value Engagement, materials []MaterialReference) Details {
	active := make(map[string][]MaterialReference, len(materials))
	for _, material := range materials {
		if material.SupersededAt == nil {
			active[material.RequirementKey] = append(active[material.RequirementKey], material)
		}
	}
	for _, items := range active {
		sort.Slice(items, func(i, j int) bool {
			if items[i].CreatedEngagementRevision != items[j].CreatedEngagementRevision {
				return items[i].CreatedEngagementRevision < items[j].CreatedEngagementRevision
			}
			return items[i].ID < items[j].ID
		})
	}
	required := map[string]bool{}
	if value.Method != nil {
		for _, key := range RequiredKeys(*value.Method) {
			required[key] = true
		}
	}
	keys := []string{RequirementTenderDocument, RequirementBidDocument, RequirementAwardDocument}
	details := Details{Engagement: value, Materials: materials, Requirements: make([]Requirement, 0, len(keys))}
	for _, key := range keys {
		items := active[key]
		satisfied := len(items) > 0
		requirement := Requirement{Key: key, Required: required[key], Satisfied: !required[key] || satisfied}
		if satisfied {
			copy := items[len(items)-1]
			requirement.MaterialReference = &copy
			requirement.MaterialReferences = items
		}
		if required[key] && !satisfied {
			details.MissingRequirements = append(details.MissingRequirements, key)
		}
		details.Requirements = append(details.Requirements, requirement)
	}
	return details
}

func DeriveStatus(value Engagement, materials []MaterialReference) string {
	if value.Status == StatusCancelled {
		return StatusCancelled
	}
	if value.Applicability == ApplicabilityNotApplicable {
		return StatusNotApplicable
	}
	if value.Applicability != ApplicabilityApplicable || value.Method == nil || !ValidMethod(*value.Method) {
		return StatusDraft
	}
	if len(BuildDetails(value, materials).MissingRequirements) == 0 {
		return StatusReady
	}
	return StatusCollectingEvidence
}

func NormalizeCreate(input CreateInput) (CreateInput, error) {
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	input.ProjectID = strings.ToLower(strings.TrimSpace(input.ProjectID))
	if !ValidUUID(input.CommandID) || !ValidUUID(input.ProjectID) || input.ProjectSourceRevision < 1 {
		return CreateInput{}, fmt.Errorf("command_id, project_id and project_source_revision are required")
	}
	return input, nil
}

func NormalizeApplicability(input ApplicabilityInput) (ApplicabilityInput, error) {
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	input.Reason = strings.TrimSpace(input.Reason)
	if !ValidUUID(input.CommandID) || input.ExpectedRevision < 1 || len([]rune(input.Reason)) > 1000 || (!input.Applicable && input.Reason == "") {
		return ApplicabilityInput{}, fmt.Errorf("invalid applicability command")
	}
	return input, nil
}

func NormalizeMethod(input MethodInput) (MethodInput, error) {
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	input.Method = strings.ToLower(strings.TrimSpace(input.Method))
	if !ValidUUID(input.CommandID) || input.ExpectedRevision < 1 || !ValidMethod(input.Method) {
		return MethodInput{}, fmt.Errorf("invalid method command")
	}
	return input, nil
}

func NormalizeMaterial(input MaterialInput) (MaterialInput, error) {
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	input.Operation = strings.ToLower(strings.TrimSpace(input.Operation))
	input.RequirementKey = strings.ToLower(strings.TrimSpace(input.RequirementKey))
	input.EvidenceFileID = strings.ToLower(strings.TrimSpace(input.EvidenceFileID))
	input.MaterialReferenceID = strings.ToLower(strings.TrimSpace(input.MaterialReferenceID))
	if !ValidUUID(input.CommandID) || input.ExpectedRevision < 1 || !ValidRequirement(input.RequirementKey) {
		return MaterialInput{}, fmt.Errorf("invalid material command")
	}
	switch input.Operation {
	case "put", "append":
		if !ValidUUID(input.EvidenceFileID) || input.EvidenceVersion < 1 || input.EvidenceSourceRevision < 1 {
			return MaterialInput{}, fmt.Errorf("invalid evidence reference")
		}
		if input.MaterialReferenceID != "" {
			return MaterialInput{}, fmt.Errorf("put or append cannot target a material reference")
		}
	case "remove", "remove_one":
		if input.EvidenceFileID != "" || input.EvidenceVersion != 0 || input.EvidenceSourceRevision != 0 {
			return MaterialInput{}, fmt.Errorf("remove cannot contain an evidence reference")
		}
		if (input.Operation == "remove_one" && !ValidUUID(input.MaterialReferenceID)) || (input.Operation == "remove" && input.MaterialReferenceID != "") {
			return MaterialInput{}, fmt.Errorf("only remove_one requires a material reference id")
		}
	default:
		return MaterialInput{}, fmt.Errorf("operation must be put, append, remove or remove_one")
	}
	return input, nil
}

func NormalizeCancel(input CancelInput) (CancelInput, error) {
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	input.Reason = strings.TrimSpace(input.Reason)
	if !ValidUUID(input.CommandID) || input.ExpectedRevision < 1 || input.Reason == "" || len([]rune(input.Reason)) > 1000 {
		return CancelInput{}, fmt.Errorf("invalid cancel command")
	}
	return input, nil
}
