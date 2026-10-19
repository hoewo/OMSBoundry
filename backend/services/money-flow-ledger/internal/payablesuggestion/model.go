package payablesuggestion

import (
	"errors"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
	contract "omsbizfoundry/payable-suggestion-contract"
)

var (
	ErrNotFound         = errors.New("payable suggestion intake not found")
	ErrCommandConflict  = errors.New("payable suggestion command payload conflict")
	ErrStateConflict    = errors.New("payable suggestion intake state conflict")
	ErrRevisionConflict = errors.New("payable suggestion intake revision conflict")
	ErrPartyMapping     = errors.New("payable suggestion Party mapping is unavailable")
	ErrInvalid          = errors.New("invalid payable suggestion")
)

type Intake struct {
	SchemaVersion            string                        `json:"schema_version"`
	SuggestionID             string                        `json:"suggestion_id"`
	TenantID                 string                        `json:"tenant_id"`
	RequestDigest            string                        `json:"request_digest"`
	Submission               contract.Submission           `json:"submission"`
	Status                   string                        `json:"status"`
	Revision                 int64                         `json:"revision"`
	Obligation               *contract.ObligationReference `json:"obligation,omitempty"`
	DecisionReason           string                        `json:"decision_reason,omitempty"`
	SubmittedBy              audit.ActorRef                `json:"submitted_by"`
	DecidedBy                *audit.ActorRef               `json:"decided_by,omitempty"`
	CreatedAt                time.Time                     `json:"created_at"`
	UpdatedAt                time.Time                     `json:"updated_at"`
	LedgerOwnerLocalPartyID  string                        `json:"-"`
	CounterpartyLocalPartyID string                        `json:"-"`
}

type Revision struct {
	SuggestionID string         `json:"suggestion_id"`
	Revision     int64          `json:"revision"`
	CommandID    string         `json:"command_id"`
	EventKind    string         `json:"event_kind"`
	Snapshot     Intake         `json:"snapshot"`
	Actor        audit.ActorRef `json:"actor"`
	OccurredAt   time.Time      `json:"occurred_at"`
}

type SubmitResult struct {
	Intake   Intake `json:"intake"`
	Replayed bool   `json:"replayed"`
}

type DecisionResult struct {
	Intake   Intake `json:"intake"`
	Replayed bool   `json:"replayed"`
}
