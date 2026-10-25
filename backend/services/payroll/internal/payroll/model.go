package payroll

import (
	"errors"
	"time"
)

var (
	ErrNotFound         = errors.New("payroll run not found")
	ErrRevisionConflict = errors.New("payroll run revision conflict")
	ErrCommandConflict  = errors.New("command payload conflict")
)

type Actor struct{ ID, TenantID, DisplayName, Kind, CredentialID string }
type Run struct {
	ID                 string    `json:"id"`
	TenantID           string    `json:"tenant_id"`
	Period             string    `json:"period"`
	Label              string    `json:"label"`
	Status             string    `json:"status"`
	Revision           int64     `json:"revision"`
	CreatedBySubjectID string    `json:"created_by_subject_id"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	AllowedActions     []string  `json:"allowed_actions,omitempty"`
}
type CreateInput struct {
	CommandID string `json:"command_id"`
	Period    string `json:"period"`
	Label     string `json:"label"`
}
type UpdateInput struct {
	CommandID        string `json:"command_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Label            string `json:"label"`
}
type Page struct {
	Items []Run `json:"items"`
}
