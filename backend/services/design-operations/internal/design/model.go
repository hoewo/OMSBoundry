package design

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	DesignTypeKey       = "design-v1"
	ConsultationTypeKey = "consultation-v1"
	// TypeKey is retained for the versioned Design API compatibility surface.
	TypeKey            = DesignTypeKey
	StatusDraft        = "draft"
	StatusInProgress   = "in-progress"
	StatusCompleted    = "completed"
	StatusCancelled    = "cancelled"
	StagePending       = "pending"
	StageReady         = "ready"
	StageCompleted     = "completed"
	StageNotApplicable = "not-applicable"
	RequirementPrimary = "primary-output"
)

var (
	ErrNotFound             = errors.New("design work item not found")
	ErrRevisionConflict     = errors.New("design revision conflict")
	ErrCommandConflict      = errors.New("design command conflict")
	ErrActiveConflict       = errors.New("active design work item conflict")
	ErrStateConflict        = errors.New("design state conflict")
	ErrStageInvalid         = errors.New("design stage invalid")
	ErrTypeInvalid          = errors.New("production type definition invalid")
	ErrCompletionGaps       = errors.New("stage completion gaps")
	ErrReferenceInvalid     = errors.New("external reference invalid")
	ErrReferenceUnavailable = errors.New("external reference unavailable")
	uuidPattern             = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)

type StageDefinition struct {
	Key                     string
	Sequence                int
	Skippable               bool
	RequiredRequirementKeys []string
}

func ValidUUID(v string) bool { return uuidPattern.MatchString(strings.ToLower(strings.TrimSpace(v))) }

type Actor struct {
	Kind        string `json:"kind"`
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	TenantID    string `json:"tenant_id"`
}
type WorkItem struct {
	ID                    string    `json:"id"`
	TenantID              string    `json:"tenant_id"`
	ProjectID             string    `json:"project_id"`
	ProjectSourceRevision int64     `json:"project_source_revision"`
	TypeKey               string    `json:"type_key"`
	TypeDefinitionVersion int64     `json:"type_definition_version"`
	Status                string    `json:"status"`
	Revision              int64     `json:"revision"`
	OwnerAssignmentID     *string   `json:"owner_assignment_id,omitempty"`
	OwnerSourceRevision   *int64    `json:"owner_source_revision,omitempty"`
	CurrentStageKey       *string   `json:"current_stage_key,omitempty"`
	CancellationReason    *string   `json:"cancellation_reason,omitempty"`
	CreatedBy             string    `json:"created_by"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}
type Stage struct {
	TenantID            string     `json:"tenant_id"`
	WorkItemID          string     `json:"work_item_id"`
	Key                 string     `json:"stage_key"`
	Sequence            int        `json:"sequence"`
	Status              string     `json:"status"`
	NotApplicableReason *string    `json:"not_applicable_reason,omitempty"`
	CompletedAt         *time.Time `json:"completed_at,omitempty"`
	CompletedRevision   *int64     `json:"completed_work_item_revision,omitempty"`
	UpdatedAt           time.Time  `json:"updated_at"`
}
type Deliverable struct {
	ID                     string     `json:"id"`
	TenantID               string     `json:"tenant_id"`
	WorkItemID             string     `json:"work_item_id"`
	StageKey               string     `json:"stage_key"`
	RequirementKey         string     `json:"requirement_key"`
	EvidenceFileID         string     `json:"evidence_file_id"`
	EvidenceVersion        int        `json:"evidence_version"`
	EvidenceSourceRevision int64      `json:"evidence_source_revision"`
	DisplaySnapshot        string     `json:"display_snapshot"`
	CreatedRevision        int64      `json:"created_work_item_revision"`
	SupersededRevision     *int64     `json:"superseded_work_item_revision,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	SupersededAt           *time.Time `json:"superseded_at,omitempty"`
}
type HistoryEvent struct {
	ID               string         `json:"id"`
	TenantID         string         `json:"tenant_id"`
	WorkItemID       string         `json:"work_item_id"`
	WorkItemRevision int64          `json:"work_item_revision"`
	EventSequence    int64          `json:"event_sequence"`
	EventType        string         `json:"event_type"`
	CommandID        string         `json:"command_id"`
	ActorSubject     string         `json:"actor_subject"`
	OccurredAt       time.Time      `json:"occurred_at"`
	Payload          map[string]any `json:"payload"`
}
type ReferenceStatus struct {
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	Status string `json:"status"`
}
type Details struct {
	WorkItem          WorkItem          `json:"work_item"`
	Stages            []Stage           `json:"stages"`
	Deliverables      []Deliverable     `json:"deliverables"`
	ReferenceStatuses []ReferenceStatus `json:"reference_statuses"`
}
type HistoryPage struct {
	Items             []HistoryEvent `json:"items"`
	NextAfterSequence *int64         `json:"next_after_sequence"`
}
type ReferenceView struct {
	TenantID        string   `json:"tenant_id"`
	ID              string   `json:"id"`
	WorkItemID      string   `json:"work_item_id"`
	ProjectID       string   `json:"project_id"`
	TypeKey         string   `json:"type_key"`
	ResourceType    string   `json:"resource_type"`
	Status          string   `json:"status"`
	Revision        int64    `json:"revision"`
	CurrentStageKey *string  `json:"current_stage_key"`
	CompletionGaps  []string `json:"completion_gaps"`
}

type CreateInput struct {
	CommandID             string `json:"command_id"`
	ProjectID             string `json:"project_id"`
	ProjectSourceRevision int64  `json:"project_source_revision"`
	TypeKey               string `json:"type_key,omitempty"`
	TypeDefinitionVersion int64  `json:"type_definition_version,omitempty"`
	// ProjectTypeSnapshot is injected only after the Project reference and its
	// exact revision have been verified. It is Production-owned evidence of the
	// applicability decision, not another owner for Project's current types.
	ProjectTypeSnapshot []string `json:"-"`
}
type OwnerInput struct {
	CommandID                string `json:"command_id"`
	ExpectedRevision         int64  `json:"expected_revision"`
	AssignmentID             string `json:"assignment_id"`
	AssignmentSourceRevision int64  `json:"assignment_source_revision"`
}
type DeliverableInput struct {
	CommandID              string `json:"command_id"`
	ExpectedRevision       int64  `json:"expected_revision"`
	RequirementKey         string `json:"requirement_key"`
	EvidenceFileID         string `json:"evidence_file_id"`
	EvidenceVersion        int    `json:"evidence_version"`
	EvidenceSourceRevision int64  `json:"evidence_source_revision"`
	DisplaySnapshot        string `json:"-"`
}
type ApplicabilityInput struct {
	CommandID        string `json:"command_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Operation        string `json:"operation"`
	Reason           string `json:"reason"`
}
type RevisionInput struct {
	CommandID        string `json:"command_id"`
	ExpectedRevision int64  `json:"expected_revision"`
}
type DefinitionUpgradeInput struct {
	RevisionInput
	TargetVersion       int64    `json:"target_version"`
	ProjectTypeSnapshot []string `json:"-"`
}
type ReasonInput struct {
	RevisionInput
	Reason string `json:"reason"`
}
type ProjectReference struct {
	ID       string   `json:"id"`
	TenantID string   `json:"tenant_id"`
	Status   string   `json:"status"`
	Revision int64    `json:"revision"`
	TypeKeys []string `json:"project_type_keys"`
}

type AssignmentReference struct {
	ID             string `json:"id"`
	TenantID       string `json:"tenant_id"`
	ProjectID      string `json:"project_id"`
	SubjectKind    string `json:"subject_kind"`
	SubjectID      string `json:"subject_id"`
	SourceRevision int64  `json:"source_revision"`
	RoleKey        string `json:"role_key"`
	Status         string `json:"status"`
	DisplayName    string `json:"display_name"`
	Revision       int64  `json:"revision"`
}

// PersonIAMSubjectFact is the narrow People-owned policy fact required to
// translate a current Person responsibility into an IAM Subject. Production
// never copies the People record or treats the linkage as its own fact.
type PersonIAMSubjectFact struct {
	SchemaVersion     string `json:"schema_version"`
	SourceApplication string `json:"source_application"`
	FactKind          string `json:"fact_kind"`
	ID                string `json:"id"`
	TenantID          string `json:"tenant_id"`
	Status            string `json:"status"`
	Revision          int64  `json:"revision"`
	IAMSubjectID      string `json:"iam_subject_id"`
}

// WorkItemCandidate is the source-owned state needed to publish a personal
// work projection. Authorization-derived allowed actions are deliberately
// added by the HTTP boundary from the current human credential.
type WorkItemCandidate struct {
	WorkItem WorkItem            `json:"work_item"`
	Owner    AssignmentReference `json:"owner"`
}

type EvidenceReference struct {
	FileID      string `json:"file_id"`
	TenantID    string `json:"tenant_id"`
	Status      string `json:"status"`
	DisplayName string `json:"display_name"`
	Version     int    `json:"version"`
	Revision    int64  `json:"revision"`
}

func normalizeCommand(id string, revision int64) (string, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if !ValidUUID(id) || revision < 1 {
		return "", fmt.Errorf("command_id and expected_revision are required")
	}
	return id, nil
}
func hasProjectType(types []string, required string) bool {
	for _, v := range types {
		if strings.EqualFold(strings.TrimSpace(v), required) {
			return true
		}
	}
	return false
}
