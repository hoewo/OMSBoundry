package projects

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	StatusDraft             = "draft"
	StatusPendingCompletion = "pending_completion"
	StatusActive            = "active"
	StatusCancelled         = "cancelled"

	InitiationProfileDualOwnerV1     = "dual-owner-v1"
	InitiationProfileBusinessOwnerV1 = "business-owner-v1"

	CompletionProfileReferenceOAC5V1 = "reference-oa-c5-v1"
	CompletionSatisfied              = "satisfied"
	CompletionPending                = "pending"
	CompletionNotApplicable          = "not_applicable"
	CompletionUnavailable            = "unavailable"

	RoleBusinessOwner         = "business_owner"
	RoleProductionOwner       = "production_owner"
	RoleParticipant           = "participant"
	RoleCommercialContributor = "commercial_contributor"

	NodeTypeProject = "project"
	NodeTypePackage = "package"

	ConversionAttachExisting = "attach_existing"
	ConversionWrapAsPackage  = "wrap_as_package"
	ConversionActive         = "active"
	ConversionRolledBack     = "rolled_back"
)

var (
	ErrNotFound             = errors.New("project not found")
	ErrRevisionConflict     = errors.New("project revision conflict")
	ErrNumberConflict       = errors.New("project number conflict")
	ErrCommandConflict      = errors.New("command payload conflict")
	ErrStateConflict        = errors.New("project state conflict")
	ErrOwnerConflict        = errors.New("active owner conflict")
	ErrActiveLinkConflict   = errors.New("active resource link conflict")
	ErrReferenceInvalid     = errors.New("external reference invalid")
	ErrReferenceForbidden   = errors.New("external reference forbidden")
	ErrReferenceUnavailable = errors.New("external reference unavailable")
	ErrInitiationIncomplete = errors.New("project initiation incomplete")
	ErrHierarchyConflict    = errors.New("project hierarchy conflict")
	ErrRollbackUnsafe       = errors.New("project package rollback is unsafe")
	ErrCompletionIncomplete = errors.New("completion profile is not satisfied")
	ErrCompletionDrift      = errors.New("completion sources changed")
)

var projectNumberPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,63}$`)
var keyPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type Actor struct {
	Kind        string `json:"kind"`
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	TenantID    string `json:"tenant_id"`
}

type Project struct {
	ID                 string         `json:"id"`
	TenantID           string         `json:"tenant_id"`
	ProjectNumber      string         `json:"project_number"`
	NodeType           string         `json:"node_type"`
	ParentProjectID    *string        `json:"parent_project_id,omitempty"`
	SortOrder          int            `json:"sort_order"`
	Name               string         `json:"name"`
	Summary            string         `json:"summary"`
	PlannedStartDate   *time.Time     `json:"planned_start_date,omitempty"`
	PlannedEndDate     *time.Time     `json:"planned_end_date,omitempty"`
	ProjectTypeKeys    []string       `json:"project_type_keys"`
	ClientParty        *PartySnapshot `json:"client_party,omitempty"`
	Status             string         `json:"status"`
	Revision           int64          `json:"revision"`
	CancellationReason *string        `json:"cancellation_reason,omitempty"`
	CreatedBy          string         `json:"created_by"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type Assignment struct {
	ID                     string     `json:"id"`
	TenantID               string     `json:"tenant_id"`
	ProjectID              string     `json:"project_id"`
	SubjectKind            string     `json:"subject_kind"`
	SubjectID              string     `json:"subject_id"`
	SourceRevision         int64      `json:"source_revision"`
	DisplaySnapshot        string     `json:"display_snapshot"`
	RoleKey                string     `json:"role_key"`
	EffectiveFrom          time.Time  `json:"effective_from"`
	EndedAt                *time.Time `json:"ended_at,omitempty"`
	EndReason              *string    `json:"end_reason,omitempty"`
	CreatedProjectRevision int64      `json:"created_project_revision"`
	EndedProjectRevision   *int64     `json:"ended_project_revision,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
}

type ResourceLink struct {
	ID                      string     `json:"id"`
	TenantID                string     `json:"tenant_id"`
	ProjectID               string     `json:"project_id"`
	TargetService           string     `json:"target_service"`
	ResourceType            string     `json:"resource_type"`
	ResourceID              string     `json:"resource_id"`
	SourceRevision          *int64     `json:"source_revision,omitempty"`
	RelationKey             string     `json:"relation_key"`
	Note                    string     `json:"note"`
	CreatedProjectRevision  int64      `json:"created_project_revision"`
	UnlinkedProjectRevision *int64     `json:"unlinked_project_revision,omitempty"`
	LinkedAt                time.Time  `json:"linked_at"`
	UnlinkedAt              *time.Time `json:"unlinked_at,omitempty"`
}

type HistoryEvent struct {
	ID              string         `json:"id"`
	TenantID        string         `json:"tenant_id"`
	ProjectID       string         `json:"project_id"`
	ProjectRevision int64          `json:"project_revision"`
	EventSequence   int64          `json:"event_sequence"`
	EventType       string         `json:"event_type"`
	CommandID       string         `json:"command_id"`
	ActorSubject    string         `json:"actor_subject"`
	OccurredAt      time.Time      `json:"occurred_at"`
	Payload         map[string]any `json:"payload"`
}

type Details struct {
	Project                 Project               `json:"project"`
	Parent                  *ProjectSummary       `json:"parent,omitempty"`
	Children                []ProjectSummary      `json:"children"`
	Assignments             []Assignment          `json:"assignments"`
	ResourceLinks           []ResourceLink        `json:"resource_links"`
	ReferenceStatuses       []ReferenceLinkStatus `json:"reference_statuses"`
	ActivePackageConversion *PackageConversion    `json:"active_package_conversion,omitempty"`
}

type ProjectSummary struct {
	ID              string  `json:"id"`
	ProjectNumber   string  `json:"project_number"`
	Name            string  `json:"name"`
	NodeType        string  `json:"node_type"`
	ParentProjectID *string `json:"parent_project_id,omitempty"`
	SortOrder       int     `json:"sort_order"`
	Status          string  `json:"status"`
	Revision        int64   `json:"revision"`
}

type ReferenceLinkStatus struct {
	LinkID string `json:"link_id"`
	Status string `json:"status"`
}

const (
	CommercialSegmentAvailable       = "available"
	CommercialSegmentRevisionChanged = "revision_changed"
	CommercialSegmentInvalid         = "invalid"
	CommercialSegmentUnavailable     = "unavailable"
)

// CommercialSummary is an ephemeral, read-only composition. Project Workspace
// owns the links and project context, while every nested fact remains owned by
// the source application and is never persisted by Project Workspace.
type CommercialSummary struct {
	ProjectID       string              `json:"project_id"`
	ProjectRevision int64               `json:"project_revision"`
	ObservedAt      time.Time           `json:"observed_at"`
	TotalDurationMS int64               `json:"total_duration_ms"`
	Segments        []CommercialSegment `json:"segments"`
}

type CommercialSegment struct {
	LinkID            string                      `json:"link_id"`
	SourceApplication string                      `json:"source_application"`
	ResourceType      string                      `json:"resource_type"`
	ResourceID        string                      `json:"resource_id"`
	RelationKey       string                      `json:"relation_key"`
	LinkedAt          time.Time                   `json:"linked_at"`
	LinkedRevision    *int64                      `json:"linked_revision,omitempty"`
	SourceRevision    *int64                      `json:"source_revision,omitempty"`
	State             string                      `json:"state"`
	ErrorCode         string                      `json:"error_code,omitempty"`
	DisplaySummary    string                      `json:"display_summary,omitempty"`
	BusinessStatus    string                      `json:"business_status,omitempty"`
	ObservedAt        time.Time                   `json:"observed_at"`
	DurationMS        int64                       `json:"duration_ms"`
	DeepLink          string                      `json:"deep_link"`
	Contract          *ContractCommercialFacts    `json:"contract,omitempty"`
	Obligation        *ObligationCommercialFacts  `json:"obligation,omitempty"`
	Transaction       *TransactionCommercialFacts `json:"transaction,omitempty"`
	Invoice           *InvoiceCommercialFacts     `json:"invoice_registration,omitempty"`
}

const (
	MaterialCollectionAvailable    = "available"
	MaterialCollectionEmpty        = "empty"
	MaterialSegmentAvailable       = "available"
	MaterialSegmentEmpty           = "empty"
	MaterialSegmentRevisionChanged = "revision_changed"
	MaterialSegmentInvalid         = "invalid"
	MaterialSegmentForbidden       = "forbidden"
	MaterialSegmentUnavailable     = "unavailable"
)

// MaterialCollection is an ephemeral Project-scoped composition of direct
// Evidence links and source-owned Evidence-reference projections. Project owns
// only its links and ordering; Evidence remains the owner of file metadata,
// versions, digests and access grants.
type MaterialCollection struct {
	ProjectID       string            `json:"project_id"`
	ProjectRevision int64             `json:"project_revision"`
	State           string            `json:"state"`
	ObservedAt      time.Time         `json:"observed_at"`
	TotalDurationMS int64             `json:"total_duration_ms"`
	Segments        []MaterialSegment `json:"segments"`
}

type MaterialSegment struct {
	LinkID                 string    `json:"link_id"`
	SourceApplication      string    `json:"source_application"`
	ResourceType           string    `json:"resource_type"`
	ResourceID             string    `json:"resource_id"`
	RelationKey            string    `json:"relation_key"`
	LinkedRevision         *int64    `json:"linked_revision,omitempty"`
	SourceRevision         *int64    `json:"source_revision,omitempty"`
	SourceDigest           string    `json:"source_digest,omitempty"`
	SourceStatus           string    `json:"source_status,omitempty"`
	SourceRelationKey      string    `json:"source_relation_key,omitempty"`
	State                  string    `json:"state"`
	ErrorCode              string    `json:"error_code,omitempty"`
	EvidenceFileID         string    `json:"evidence_file_id,omitempty"`
	EvidenceLinkedRevision *int64    `json:"evidence_linked_revision,omitempty"`
	EvidenceSourceRevision *int64    `json:"evidence_source_revision,omitempty"`
	EvidenceVersion        int64     `json:"evidence_version,omitempty"`
	EvidenceDigest         string    `json:"evidence_digest,omitempty"`
	FileName               string    `json:"file_name,omitempty"`
	SizeBytes              int64     `json:"size_bytes,omitempty"`
	MediaType              string    `json:"media_type,omitempty"`
	ObservedAt             time.Time `json:"observed_at"`
	DurationMS             int64     `json:"duration_ms"`
	DeepLink               string    `json:"deep_link"`
}

type EvidenceFileFacts struct {
	Version   int64
	FileName  string
	SizeBytes int64
	MediaType string
	SHA256    string
}

// MaterialSourceFacts is a source-owned, read-only list of Evidence
// references. Project Workspace may compose it with Evidence metadata but
// never persists the list or imports the source application's domain model.
type MaterialSourceFacts struct {
	ProjectID string
	Digest    string
	Items     []MaterialSourceItem
}

type MaterialSourceItem struct {
	RelationKey            string
	FileID                 string
	Version                int64
	EvidenceSourceRevision int64
	DisplayName            string
}

type ContractCommercialFacts struct {
	Currency               string                    `json:"currency"`
	CurrentEffectiveAmount *string                   `json:"current_effective_amount"`
	Parties                []CommercialPartySnapshot `json:"parties,omitempty"`
}

type ObligationCommercialFacts struct {
	Direction         string                    `json:"direction"`
	Currency          string                    `json:"currency"`
	EffectiveAmount   string                    `json:"effective_amount"`
	AllocatedAmount   string                    `json:"allocated_amount"`
	RemittedAmount    string                    `json:"remitted_amount"`
	OutstandingAmount string                    `json:"outstanding_amount"`
	BalanceSequence   int64                     `json:"balance_sequence"`
	Parties           []CommercialPartySnapshot `json:"parties,omitempty"`
}

type TransactionCommercialFacts struct {
	TransactionType   string                    `json:"transaction_type"`
	Currency          string                    `json:"currency"`
	EffectiveAmount   string                    `json:"effective_amount"`
	AllocatedAmount   string                    `json:"allocated_amount"`
	UnallocatedAmount string                    `json:"unallocated_amount"`
	OccurredOn        string                    `json:"occurred_on"`
	Parties           []CommercialPartySnapshot `json:"parties,omitempty"`
}

// CommercialPartySnapshot is a source-owned Party reference carried only in
// the ephemeral commercial projection. Project never stores or edits it.
type CommercialPartySnapshot struct {
	ID        string `json:"id"`
	Revision  int64  `json:"revision"`
	Kind      string `json:"kind"`
	LegalName string `json:"legal_name"`
	Source    string `json:"source"`
	Role      string `json:"role,omitempty"`
}

type InvoiceCommercialFacts struct {
	IssuedOn    string `json:"issued_on"`
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
}

type HistoryPage struct {
	Items      []HistoryEvent `json:"items"`
	NextCursor *string        `json:"next_cursor"`
}

// CompletionResultReference is a minimal source projection. Approval & Audit
// Results remains authoritative; Project Workspace only evaluates and freezes
// exact source identities when a user confirms a profile checkpoint.
type CompletionResultReference struct {
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

type CompletionRequirement struct {
	Key    string                     `json:"key"`
	Status string                     `json:"status"`
	Reason string                     `json:"reason"`
	Source *CompletionResultReference `json:"source,omitempty"`
}

type CompletionSource struct {
	RequirementKey string `json:"requirement_key"`
	ResultID       string `json:"result_id"`
	ResultKind     string `json:"result_kind"`
	Revision       int64  `json:"revision"`
	Digest         string `json:"digest"`
}

type CompletionConfirmation struct {
	ID                       string             `json:"id"`
	TenantID                 string             `json:"tenant_id"`
	ProjectID                string             `json:"project_id"`
	ProfileKey               string             `json:"profile_key"`
	ObservedProjectRevision  int64              `json:"observed_project_revision"`
	ConfirmedProjectRevision int64              `json:"confirmed_project_revision"`
	Sources                  []CompletionSource `json:"sources"`
	CommandID                string             `json:"command_id"`
	ConfirmedBy              string             `json:"confirmed_by"`
	ConfirmedAt              time.Time          `json:"confirmed_at"`
}

type CompletionAssessment struct {
	ProjectID          string                  `json:"project_id"`
	ProjectRevision    int64                   `json:"project_revision"`
	ProfileKey         string                  `json:"profile_key"`
	OverallStatus      string                  `json:"overall_status"`
	Requirements       []CompletionRequirement `json:"requirements"`
	ObservedAt         time.Time               `json:"observed_at"`
	LatestConfirmation *CompletionConfirmation `json:"latest_confirmation,omitempty"`
	Drifted            bool                    `json:"drifted"`
}

type CompletionCommand struct {
	CommandID        string             `json:"command_id"`
	ExpectedRevision int64              `json:"expected_revision"`
	ProfileKey       string             `json:"profile_key"`
	ExpectedSources  []CompletionSource `json:"expected_sources"`
}

type CreateInput struct {
	CommandID            string               `json:"command_id"`
	ProjectNumber        string               `json:"project_number"`
	Name                 string               `json:"name"`
	Summary              string               `json:"summary"`
	PlannedStartDate     *time.Time           `json:"planned_start_date"`
	PlannedEndDate       *time.Time           `json:"planned_end_date"`
	ProjectTypeKeys      []string             `json:"project_type_keys"`
	ClientParty          *PartyReferenceInput `json:"client_party,omitempty"`
	validatedClientParty *PartySnapshot       `json:"-"`
}

// PartyReferenceInput identifies the exact Party revision selected while the
// Project is created. The caller cannot submit Party display data.
type PartyReferenceInput struct {
	PartyID        string `json:"party_id"`
	SourceRevision int64  `json:"source_revision"`
}

// PartySnapshot is the minimum immutable business-occurrence snapshot owned
// by Project Workspace. Party Directory remains authoritative for the current
// profile; Project history keeps the name and kind that were verified here.
type PartySnapshot struct {
	PartyID        string `json:"party_id"`
	SourceRevision int64  `json:"source_revision"`
	Kind           string `json:"kind"`
	LegalName      string `json:"legal_name"`
}

type UpdateInput struct {
	CreateInput
	ExpectedRevision int64 `json:"expected_revision"`
}

type ProfileCorrectionInput struct {
	UpdateInput
	CorrectionReason string `json:"correction_reason"`
}

type AssignmentCommand struct {
	CommandID        string     `json:"command_id"`
	ExpectedRevision int64      `json:"expected_revision"`
	Operation        string     `json:"operation"`
	AssignmentID     string     `json:"assignment_id"`
	SubjectKind      string     `json:"subject_kind"`
	SubjectID        string     `json:"subject_id"`
	SourceRevision   int64      `json:"source_revision"`
	RoleKey          string     `json:"role_key"`
	EffectiveFrom    *time.Time `json:"effective_from"`
	EndReason        string     `json:"end_reason"`
	DisplaySnapshot  string     `json:"-"`
}

type ResourceLinkCommand struct {
	CommandID        string `json:"command_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Operation        string `json:"operation"`
	LinkID           string `json:"link_id"`
	TargetService    string `json:"target_service"`
	ResourceType     string `json:"resource_type"`
	ResourceID       string `json:"resource_id"`
	SourceRevision   *int64 `json:"source_revision"`
	RelationKey      string `json:"relation_key"`
	Note             string `json:"note"`
}

type RevisionCommand struct {
	CommandID         string `json:"command_id"`
	ExpectedRevision  int64  `json:"expected_revision"`
	InitiationProfile string `json:"initiation_profile,omitempty"`
}

type CancelCommand struct {
	RevisionCommand
	Reason string `json:"reason"`
}

type PackageCreateInput struct {
	CommandID        string     `json:"command_id"`
	ProjectNumber    string     `json:"project_number"`
	Name             string     `json:"name"`
	Summary          string     `json:"summary"`
	PlannedStartDate *time.Time `json:"planned_start_date"`
	PlannedEndDate   *time.Time `json:"planned_end_date"`
	ProjectTypeKeys  []string   `json:"project_type_keys"`
}

type PackageConversionCommand struct {
	CommandID               string              `json:"command_id"`
	Operation               string              `json:"operation"`
	ExpectedProjectRevision int64               `json:"expected_project_revision"`
	TargetPackageID         string              `json:"target_package_id"`
	ExpectedPackageRevision int64               `json:"expected_package_revision"`
	SortOrder               int                 `json:"sort_order"`
	NewPackage              *PackageCreateInput `json:"new_package,omitempty"`
}

type PackageRollbackCommand struct {
	CommandID               string `json:"command_id"`
	ExpectedProjectRevision int64  `json:"expected_project_revision"`
	ExpectedPackageRevision int64  `json:"expected_package_revision"`
	Reason                  string `json:"reason"`
}

type PackageConversion struct {
	ID                      string     `json:"id"`
	TenantID                string     `json:"tenant_id"`
	Operation               string     `json:"operation"`
	ProjectID               string     `json:"project_id"`
	PackageID               string     `json:"package_id"`
	PreviousParentProjectID *string    `json:"previous_parent_project_id,omitempty"`
	PreviousSortOrder       int        `json:"previous_sort_order"`
	CreatedPackage          bool       `json:"created_package"`
	CreatedProjectRevision  int64      `json:"created_project_revision"`
	CreatedPackageRevision  int64      `json:"created_package_revision"`
	Status                  string     `json:"status"`
	CreatedBy               string     `json:"created_by"`
	CreatedAt               time.Time  `json:"created_at"`
	RolledBackBy            *string    `json:"rolled_back_by,omitempty"`
	RolledBackAt            *time.Time `json:"rolled_back_at,omitempty"`
	RollbackReason          *string    `json:"rollback_reason,omitempty"`
}

type PackageConversionResult struct {
	Conversion PackageConversion `json:"conversion"`
	Project    Project           `json:"project"`
	Package    Project           `json:"package"`
}

type AssignmentResult struct {
	Project    Project    `json:"project"`
	Assignment Assignment `json:"assignment"`
}

type ResourceLinkResult struct {
	Project      Project      `json:"project"`
	ResourceLink ResourceLink `json:"resource_link"`
}

type ReferenceSnapshot struct {
	Revision    int64
	DisplayName string
}

// ResourceReference is the source-owned stable projection returned by the
// versioned reference ports. Optional facts are type-specific; Project never
// converts them into a writable aggregate or stores them in its database.
type ResourceReference struct {
	SourceApplication string
	ID                string
	TenantID          string
	ResourceType      string
	DisplaySummary    string
	Status            string
	Revision          int64
	Contract          *ContractCommercialFacts
	Obligation        *ObligationCommercialFacts
	Transaction       *TransactionCommercialFacts
	Invoice           *InvoiceCommercialFacts
	Evidence          *EvidenceFileFacts
	MaterialSource    *MaterialSourceFacts
}

// ProjectReference and AssignmentReference are intentionally small read models
// for other applications. They expose stable identifiers and lifecycle state,
// never Project Workspace's internal tables or mutable domain objects.
type ProjectReference struct {
	ID              string                  `json:"id"`
	TenantID        string                  `json:"tenant_id"`
	ResourceType    string                  `json:"resource_type"`
	ProjectNumber   string                  `json:"project_number"`
	DisplaySummary  string                  `json:"display_summary"`
	Status          string                  `json:"status"`
	Revision        int64                   `json:"revision"`
	ProjectTypeKeys []string                `json:"project_type_keys"`
	NodeType        string                  `json:"node_type"`
	ParentProjectID *string                 `json:"parent_project_id,omitempty"`
	ResourceLinks   []ResourceLinkReference `json:"resource_links"`
}

// ResourceLinkReference is the minimal active-link projection required by
// trusted consumers to prove that a source resource belongs to this Project.
// It deliberately omits notes and historical unlink metadata.
type ResourceLinkReference struct {
	TargetService  string `json:"target_service"`
	ResourceType   string `json:"resource_type"`
	ResourceID     string `json:"resource_id"`
	SourceRevision *int64 `json:"source_revision,omitempty"`
	RelationKey    string `json:"relation_key"`
}

func NormalizePackage(input PackageCreateInput) (PackageCreateInput, error) {
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	input.ProjectNumber = strings.TrimSpace(input.ProjectNumber)
	input.Name = strings.TrimSpace(input.Name)
	input.Summary = strings.TrimSpace(input.Summary)
	if !ValidUUID(input.CommandID) || (input.ProjectNumber != "" && !projectNumberPattern.MatchString(input.ProjectNumber)) || input.Name == "" || len([]rune(input.Name)) > 200 || len([]rune(input.Summary)) > 2000 {
		return PackageCreateInput{}, fmt.Errorf("invalid project package fields")
	}
	if input.PlannedStartDate != nil && input.PlannedEndDate != nil && input.PlannedEndDate.Before(*input.PlannedStartDate) {
		return PackageCreateInput{}, fmt.Errorf("planned end date precedes start date")
	}
	seen := map[string]bool{}
	types := make([]string, 0, len(input.ProjectTypeKeys))
	for _, value := range input.ProjectTypeKeys {
		value = strings.ToLower(strings.TrimSpace(value))
		if !keyPattern.MatchString(value) || seen[value] {
			return PackageCreateInput{}, fmt.Errorf("invalid or duplicate project type")
		}
		seen[value] = true
		types = append(types, value)
	}
	if len(types) == 0 || len(types) > 16 {
		return PackageCreateInput{}, fmt.Errorf("project type is required")
	}
	input.ProjectTypeKeys = types
	return input, nil
}

type AssignmentReference struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	ProjectID      string     `json:"project_id"`
	ResourceType   string     `json:"resource_type"`
	SubjectKind    string     `json:"subject_kind"`
	SubjectID      string     `json:"subject_id"`
	SourceRevision int64      `json:"source_revision"`
	RoleKey        string     `json:"role_key"`
	Status         string     `json:"status"`
	Revision       int64      `json:"revision"`
	EffectiveFrom  time.Time  `json:"effective_from"`
	EndedAt        *time.Time `json:"ended_at,omitempty"`
	DisplayName    string     `json:"display_name"`
}

type InitiationError struct{ Missing []string }

func (e *InitiationError) Error() string { return ErrInitiationIncomplete.Error() }
func (e *InitiationError) Unwrap() error { return ErrInitiationIncomplete }

func NormalizeCreate(input CreateInput) (CreateInput, error) {
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	input.ProjectNumber = strings.TrimSpace(input.ProjectNumber)
	input.Name = strings.TrimSpace(input.Name)
	input.Summary = strings.TrimSpace(input.Summary)
	if input.ClientParty != nil {
		input.ClientParty.PartyID = strings.ToLower(strings.TrimSpace(input.ClientParty.PartyID))
		if !ValidUUID(input.ClientParty.PartyID) || input.ClientParty.SourceRevision < 1 {
			return CreateInput{}, fmt.Errorf("invalid client party reference")
		}
	}
	if !ValidUUID(input.CommandID) || !projectNumberPattern.MatchString(input.ProjectNumber) || input.Name == "" || len([]rune(input.Name)) > 200 || len([]rune(input.Summary)) > 2000 {
		return CreateInput{}, fmt.Errorf("invalid project fields")
	}
	if input.PlannedStartDate != nil && input.PlannedEndDate != nil && input.PlannedEndDate.Before(*input.PlannedStartDate) {
		return CreateInput{}, fmt.Errorf("planned end date precedes start date")
	}
	seen := map[string]bool{}
	types := make([]string, 0, len(input.ProjectTypeKeys))
	for _, value := range input.ProjectTypeKeys {
		value = strings.ToLower(strings.TrimSpace(value))
		if !keyPattern.MatchString(value) || seen[value] {
			return CreateInput{}, fmt.Errorf("invalid or duplicate project type")
		}
		seen[value] = true
		types = append(types, value)
	}
	if len(types) == 0 || len(types) > 16 {
		return CreateInput{}, fmt.Errorf("project type is required")
	}
	input.ProjectTypeKeys = types
	return input, nil
}

func ValidUUID(value string) bool {
	return uuidPattern.MatchString(strings.ToLower(strings.TrimSpace(value)))
}

func ValidRole(role string) bool {
	return role == RoleBusinessOwner || role == RoleProductionOwner || role == RoleParticipant || role == RoleCommercialContributor
}

func ValidResource(service, resourceType, relation string) bool {
	allowed := map[string]map[string]bool{
		"party-directory":        {"party": true},
		"contract-ledger":        {"contract": true, "settlement": true},
		"money-flow-ledger":      {"obligation": true, "transaction": true},
		"billing-invoice":        {"invoice_registration": true},
		"evidence-service":       {"evidence-file": true},
		"business-engagement":    {"business-engagement": true},
		"design-operations":      {"design-work-item": true},
		"production-operations":  {"production-work-item": true},
		"approval-audit-results": {"approval_audit_result": true},
		"cost-outsourcing":       {"cost_entry": true, "outsourcing_engagement": true},
	}
	return allowed[service][resourceType] && keyPattern.MatchString(relation)
}

func HasType(project Project, key string) bool {
	for _, value := range project.ProjectTypeKeys {
		if value == key {
			return true
		}
	}
	return false
}
