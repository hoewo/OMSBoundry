package party

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	KindOrganization = "organization"
	KindIndividual   = "individual"
	StatusActive     = "active"
	StatusInactive   = "inactive"

	EventCreated        = "created"
	EventProfileUpdated = "profile_updated"
	EventStatusChanged  = "status_changed"
)

var (
	ErrNotFound          = errors.New("party not found")
	ErrRevisionConflict  = errors.New("party revision conflict")
	ErrCommandConflict   = errors.New("command payload conflict")
	ErrNoEffectiveChange = errors.New("no effective change")
	ErrInactive          = errors.New("party inactive")
	uuidPattern          = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)

type Actor struct {
	ID           string `json:"id"`
	TenantID     string `json:"tenant_id"`
	DisplayName  string `json:"display_name"`
	Kind         string `json:"kind"`
	CredentialID string `json:"credential_id"`
}

type Party struct {
	ID                     string    `json:"id"`
	TenantID               string    `json:"tenant_id"`
	Kind                   string    `json:"kind"`
	LegalName              string    `json:"legal_name"`
	RegistrationIdentifier *string   `json:"registration_identifier"`
	Status                 string    `json:"status"`
	Revision               int64     `json:"revision"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
	AllowedActions         []string  `json:"allowed_actions,omitempty"`
}

type Revision struct {
	PartyID                string    `json:"party_id"`
	TenantID               string    `json:"tenant_id"`
	Revision               int64     `json:"revision"`
	EventKind              string    `json:"event_kind"`
	CommandID              string    `json:"command_id"`
	Kind                   string    `json:"kind"`
	LegalName              string    `json:"legal_name"`
	RegistrationIdentifier *string   `json:"registration_identifier"`
	Status                 string    `json:"status"`
	Reason                 string    `json:"reason"`
	ActorRef               string    `json:"actor_ref"`
	OccurredAt             time.Time `json:"occurred_at"`
}

type Reference struct {
	SchemaVersion          string  `json:"schema_version"`
	SourceApplication      string  `json:"source_application"`
	ID                     string  `json:"id"`
	TenantID               string  `json:"tenant_id"`
	Kind                   string  `json:"kind"`
	LegalName              string  `json:"legal_name"`
	RegistrationIdentifier *string `json:"registration_identifier"`
	Status                 string  `json:"status"`
	Revision               int64   `json:"revision"`
}

type CreateInput struct {
	CommandID              string  `json:"command_id"`
	Kind                   string  `json:"kind"`
	LegalName              string  `json:"legal_name"`
	RegistrationIdentifier *string `json:"registration_identifier"`
}

type UpdateInput struct {
	CommandID              string  `json:"command_id"`
	ExpectedRevision       int64   `json:"expected_revision"`
	Kind                   string  `json:"kind"`
	LegalName              string  `json:"legal_name"`
	RegistrationIdentifier *string `json:"registration_identifier"`
	Reason                 string  `json:"reason"`
}

type StatusInput struct {
	CommandID        string `json:"command_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Status           string `json:"status"`
	Reason           string `json:"reason"`
}

type ListFilter struct {
	Query  string
	Kind   string
	Status string
	Limit  int
	Cursor *Cursor
}

type Cursor struct {
	NormalizedName string `json:"normalized_name"`
	LegalName      string `json:"legal_name"`
	PartyID        string `json:"party_id"`
}

type Page struct {
	Items          []Party  `json:"items"`
	NextCursor     string   `json:"next_cursor,omitempty"`
	AllowedActions []string `json:"allowed_actions,omitempty"`
}

type HistoryPage struct {
	Items              []Revision `json:"items"`
	NextBeforeRevision *int64     `json:"next_before_revision,omitempty"`
}

func NormalizeCreate(input CreateInput) (CreateInput, error) {
	input.CommandID = normalizeUUID(input.CommandID)
	input.Kind = strings.ToLower(strings.TrimSpace(input.Kind))
	input.LegalName = strings.TrimSpace(input.LegalName)
	identifier, err := normalizeOptional(input.RegistrationIdentifier, 100)
	if err != nil {
		return CreateInput{}, err
	}
	input.RegistrationIdentifier = identifier
	if !ValidUUID(input.CommandID) || !ValidKind(input.Kind) || !validText(input.LegalName, 1, 200) {
		return CreateInput{}, fmt.Errorf("invalid party creation")
	}
	return input, nil
}

func NormalizeUpdate(input UpdateInput) (UpdateInput, error) {
	input.CommandID = normalizeUUID(input.CommandID)
	input.Kind = strings.ToLower(strings.TrimSpace(input.Kind))
	input.LegalName = strings.TrimSpace(input.LegalName)
	input.Reason = strings.TrimSpace(input.Reason)
	identifier, err := normalizeOptional(input.RegistrationIdentifier, 100)
	if err != nil {
		return UpdateInput{}, err
	}
	input.RegistrationIdentifier = identifier
	if !ValidUUID(input.CommandID) || input.ExpectedRevision < 1 || !ValidKind(input.Kind) || !validText(input.LegalName, 1, 200) || !validText(input.Reason, 1, 1000) {
		return UpdateInput{}, fmt.Errorf("invalid party update")
	}
	return input, nil
}

func NormalizeStatus(input StatusInput) (StatusInput, error) {
	input.CommandID = normalizeUUID(input.CommandID)
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	input.Reason = strings.TrimSpace(input.Reason)
	if !ValidUUID(input.CommandID) || input.ExpectedRevision < 1 || !ValidStatus(input.Status) || !validText(input.Reason, 1, 1000) {
		return StatusInput{}, fmt.Errorf("invalid party status change")
	}
	return input, nil
}

func NormalizeList(filter ListFilter) (ListFilter, error) {
	filter.Query = strings.ToLower(strings.TrimSpace(filter.Query))
	filter.Kind = strings.ToLower(strings.TrimSpace(filter.Kind))
	filter.Status = strings.ToLower(strings.TrimSpace(filter.Status))
	if filter.Status == "" {
		filter.Status = StatusActive
	}
	if filter.Limit == 0 {
		filter.Limit = 50
	}
	if len([]rune(filter.Query)) > 200 || (filter.Kind != "" && !ValidKind(filter.Kind)) || (filter.Status != StatusActive && filter.Status != StatusInactive && filter.Status != "all") || filter.Limit < 1 || filter.Limit > 100 {
		return ListFilter{}, fmt.Errorf("invalid party list query")
	}
	return filter, nil
}

func ValidUUID(value string) bool   { return uuidPattern.MatchString(normalizeUUID(value)) }
func ValidKind(value string) bool   { return value == KindOrganization || value == KindIndividual }
func ValidStatus(value string) bool { return value == StatusActive || value == StatusInactive }
func NormalizeName(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func normalizeUUID(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
func validText(value string, min, max int) bool {
	n := len([]rune(value))
	return n >= min && n <= max
}
func normalizeOptional(value *string, max int) (*string, error) {
	if value == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*value)
	if !validText(trimmed, 1, max) {
		return nil, fmt.Errorf("invalid optional text")
	}
	return &trimmed, nil
}
