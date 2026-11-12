package peoplegroups

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type DefinitionKind string

const (
	DefinitionGroupType      DefinitionKind = "group_type"
	DefinitionMembershipRole DefinitionKind = "membership_role"
	DefinitionCapability     DefinitionKind = "capability"
)

var (
	keyPattern                     = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,62}[a-z0-9]$`)
	uuidPattern                    = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	ErrNotFound                    = errors.New("resource not found")
	ErrConflict                    = errors.New("resource state conflict")
	ErrSubjectReferenceInvalid     = errors.New("IAM subject reference is invalid")
	ErrSubjectReferenceUnavailable = errors.New("IAM subject reference is unavailable")
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

type Actor struct {
	Kind        string `json:"kind"`
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	TenantID    string `json:"-"`
}

type Person struct {
	ID                   string    `json:"id"`
	TenantID             string    `json:"tenant_id"`
	ReferenceCode        string    `json:"reference_code,omitempty"`
	DisplayName          string    `json:"display_name"`
	Department           string    `json:"department,omitempty"`
	PositionTitle        string    `json:"position_title,omitempty"`
	Status               string    `json:"status"`
	StatusEffectiveDate  string    `json:"status_effective_date"`
	IAMSubjectID         string    `json:"iam_subject_id,omitempty"`
	IAMSubjectStatus     string    `json:"iam_subject_status,omitempty"`
	IAMSubjectRevision   int64     `json:"iam_subject_revision,omitempty"`
	LastActorSubjectID   string    `json:"last_actor_subject_id,omitempty"`
	LastActorDisplayName string    `json:"last_actor_display_name,omitempty"`
	Revision             int64     `json:"revision"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
	employmentReason     string
}

type EmploymentEvent struct {
	PersonID       string    `json:"person_id"`
	Sequence       int64     `json:"sequence"`
	PersonRevision int64     `json:"person_revision"`
	EventKind      string    `json:"event_kind"`
	EffectiveDate  string    `json:"effective_date"`
	Reason         string    `json:"reason,omitempty"`
	Department     string    `json:"department,omitempty"`
	PositionTitle  string    `json:"position_title,omitempty"`
	OccurredAt     time.Time `json:"occurred_at"`
}

type IAMSubjectReference struct {
	SchemaVersion     string `json:"schema_version"`
	SourceApplication string `json:"source_application"`
	ID                string `json:"id"`
	TenantID          string `json:"tenant_id"`
	Username          string `json:"username"`
	DisplayName       string `json:"display_name"`
	Status            string `json:"status"`
	Revision          int64  `json:"revision"`
}

type IAMSubjectReferencePort interface {
	ValidateIAMSubject(context.Context, string, string) (IAMSubjectReference, error)
}

type Definition struct {
	Kind        DefinitionKind `json:"kind"`
	Key         string         `json:"key"`
	DisplayName string         `json:"display_name"`
	Description string         `json:"description"`
	Status      string         `json:"status"`
	Revision    int64          `json:"revision"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type Group struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	GroupTypeKey string    `json:"group_type_key"`
	Name         string    `json:"name"`
	Purpose      string    `json:"purpose"`
	Status       string    `json:"status"`
	Revision     int64     `json:"revision"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Membership struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	GroupID       string    `json:"group_id"`
	PersonID      string    `json:"person_id"`
	Person        Person    `json:"person"`
	RoleKey       string    `json:"role_key"`
	StartDate     string    `json:"start_date"`
	EndDate       string    `json:"end_date,omitempty"`
	Status        string    `json:"status"`
	Revision      int64     `json:"revision"`
	GroupRevision int64     `json:"group_revision,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type GroupCapability struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	GroupID       string    `json:"group_id"`
	CapabilityKey string    `json:"capability_key"`
	Note          string    `json:"note"`
	StartDate     string    `json:"start_date"`
	EndDate       string    `json:"end_date,omitempty"`
	Status        string    `json:"status"`
	Revision      int64     `json:"revision"`
	GroupRevision int64     `json:"group_revision,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type GroupDetails struct {
	Group        Group             `json:"group"`
	Memberships  []Membership      `json:"memberships"`
	Capabilities []GroupCapability `json:"capabilities"`
	Snapshots    []SnapshotSummary `json:"snapshots"`
}

type SnapshotSummary struct {
	ID            string    `json:"id"`
	GroupRevision int64     `json:"group_revision"`
	SHA256        string    `json:"sha256"`
	CreatedAt     time.Time `json:"created_at"`
}

type RosterSnapshot struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	GroupID       string    `json:"group_id"`
	GroupRevision int64     `json:"group_revision"`
	Payload       []byte    `json:"-"`
	SHA256        string    `json:"sha256"`
	CreatedAt     time.Time `json:"created_at"`
}

type CreatePersonInput struct {
	CommandID           string `json:"command_id,omitempty"`
	ReferenceCode       string `json:"reference_code,omitempty"`
	DisplayName         string `json:"display_name"`
	Department          string `json:"department,omitempty"`
	PositionTitle       string `json:"position_title,omitempty"`
	EmploymentStartDate string `json:"employment_start_date,omitempty"`
	IAMSubjectID        string `json:"iam_subject_id,omitempty"`
}
type UpdatePersonProfileInput struct {
	CommandID        string `json:"command_id,omitempty"`
	ExpectedRevision int64  `json:"expected_revision"`
	ReferenceCode    string `json:"reference_code,omitempty"`
	DisplayName      string `json:"display_name"`
	Department       string `json:"department,omitempty"`
	PositionTitle    string `json:"position_title,omitempty"`
}
type SetPersonStatusInput struct {
	CommandID        string `json:"command_id,omitempty"`
	ExpectedRevision int64  `json:"expected_revision"`
	Status           string `json:"status"`
	EffectiveDate    string `json:"effective_date"`
	Reason           string `json:"reason,omitempty"`
}
type SetIAMSubjectInput struct {
	CommandID        string `json:"command_id,omitempty"`
	ExpectedRevision int64  `json:"expected_revision"`
	IAMSubjectID     string `json:"iam_subject_id,omitempty"`
}
type CreateDefinitionInput struct {
	CommandID   string `json:"command_id,omitempty"`
	Key         string `json:"key"`
	DisplayName string `json:"display_name"`
	Description string `json:"description,omitempty"`
}
type UpdateDefinitionInput struct {
	CommandID        string `json:"command_id,omitempty"`
	ExpectedRevision int64  `json:"expected_revision"`
	DisplayName      string `json:"display_name"`
	Description      string `json:"description,omitempty"`
	Status           string `json:"status"`
}
type CreateGroupInput struct {
	CommandID    string `json:"command_id,omitempty"`
	GroupTypeKey string `json:"group_type_key"`
	Name         string `json:"name"`
	Purpose      string `json:"purpose,omitempty"`
}
type UpdateGroupInput struct {
	CommandID        string `json:"command_id,omitempty"`
	ExpectedRevision int64  `json:"expected_revision"`
	GroupTypeKey     string `json:"group_type_key"`
	Name             string `json:"name"`
	Purpose          string `json:"purpose,omitempty"`
}
type GroupCommandInput struct {
	CommandID        string `json:"command_id,omitempty"`
	ExpectedRevision int64  `json:"expected_revision"`
}
type JoinMembershipInput struct {
	CommandID        string `json:"command_id,omitempty"`
	ExpectedRevision int64  `json:"expected_revision"`
	PersonID         string `json:"person_id"`
	RoleKey          string `json:"role_key"`
	StartDate        string `json:"start_date"`
}
type ChangeMembershipRoleInput struct {
	CommandID             string `json:"command_id,omitempty"`
	ExpectedGroupRevision int64  `json:"expected_group_revision"`
	ExpectedRevision      int64  `json:"expected_revision"`
	RoleKey               string `json:"role_key"`
}
type EndMembershipInput struct {
	CommandID             string `json:"command_id,omitempty"`
	ExpectedGroupRevision int64  `json:"expected_group_revision"`
	ExpectedRevision      int64  `json:"expected_revision"`
	EndDate               string `json:"end_date"`
}
type AssignCapabilityInput struct {
	CommandID        string `json:"command_id,omitempty"`
	ExpectedRevision int64  `json:"expected_revision"`
	CapabilityKey    string `json:"capability_key"`
	Note             string `json:"note,omitempty"`
	StartDate        string `json:"start_date"`
}
type EndCapabilityInput struct {
	CommandID             string `json:"command_id,omitempty"`
	ExpectedGroupRevision int64  `json:"expected_group_revision"`
	ExpectedRevision      int64  `json:"expected_revision"`
	EndDate               string `json:"end_date"`
}

func NormalizeDefinitionKind(raw string) (DefinitionKind, error) {
	kind := DefinitionKind(strings.ToLower(strings.TrimSpace(raw)))
	switch kind {
	case DefinitionGroupType, DefinitionMembershipRole, DefinitionCapability:
		return kind, nil
	default:
		return "", &ValidationError{Field: "kind", Message: "definition kind is invalid"}
	}
}

func cleanText(field, raw string, minimum, maximum int) (string, error) {
	value := strings.TrimSpace(raw)
	if len([]rune(value)) < minimum || len([]rune(value)) > maximum {
		return "", &ValidationError{Field: field, Message: fmt.Sprintf("%s must contain %d-%d characters", field, minimum, maximum)}
	}
	return value, nil
}

func cleanOptional(field, raw string, maximum int) (string, error) {
	value := strings.TrimSpace(raw)
	if len([]rune(value)) > maximum {
		return "", &ValidationError{Field: field, Message: fmt.Sprintf("%s must contain at most %d characters", field, maximum)}
	}
	return value, nil
}

func cleanKey(field, raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if !keyPattern.MatchString(value) {
		return "", &ValidationError{Field: field, Message: field + " must be a stable lower-case key"}
	}
	return value, nil
}

func cleanUUID(field, raw string, optional bool) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if optional && value == "" {
		return "", nil
	}
	if !uuidPattern.MatchString(value) {
		return "", &ValidationError{Field: field, Message: field + " must be a UUID"}
	}
	return value, nil
}

func cleanDate(field, raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return "", &ValidationError{Field: field, Message: field + " must use YYYY-MM-DD"}
	}
	return value, nil
}

func validateRevision(value int64, field string) error {
	if value < 1 {
		return &ValidationError{Field: field, Message: field + " must be positive"}
	}
	return nil
}

func validateCommandID(raw string) (string, error) { return cleanUUID("command_id", raw, false) }
