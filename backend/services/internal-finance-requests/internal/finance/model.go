package finance

import (
	"errors"
	"time"
)

var (
	ErrNotFound         = errors.New("finance request not found")
	ErrRevisionConflict = errors.New("finance request revision conflict")
	ErrCommandConflict  = errors.New("command payload conflict")
)

type Actor struct{ ID, TenantID, DisplayName, Kind, CredentialID string }
type Request struct {
	ID                 string    `json:"id"`
	TenantID           string    `json:"tenant_id"`
	ApplicantSubjectID string    `json:"applicant_subject_id"`
	RequestType        string    `json:"request_type"`
	Title              string    `json:"title"`
	AmountMinor        int64     `json:"amount_minor"`
	Currency           string    `json:"currency"`
	Status             string    `json:"status"`
	Revision           int64     `json:"revision"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	AllowedActions     []string  `json:"allowed_actions,omitempty"`
}
type CreateInput struct {
	CommandID   string `json:"command_id"`
	RequestType string `json:"request_type"`
	Title       string `json:"title"`
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
}
type UpdateInput struct {
	CommandID        string `json:"command_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Title            string `json:"title"`
	AmountMinor      int64  `json:"amount_minor"`
}
type ListFilter struct{ ApplicantSubjectID string }
type Page struct {
	Items []Request `json:"items"`
}
