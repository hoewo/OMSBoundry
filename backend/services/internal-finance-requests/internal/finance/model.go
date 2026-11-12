package finance

import (
	"errors"
	"time"
)

var (
	ErrNotFound           = errors.New("finance request not found")
	ErrRevisionConflict   = errors.New("finance request revision conflict")
	ErrCommandConflict    = errors.New("command payload conflict")
	ErrStateConflict      = errors.New("finance request state conflict")
	ErrPaymentUnknown     = errors.New("finance payment outcome is unknown")
	ErrPaymentRejected    = errors.New("finance payment was rejected")
	ErrDependency         = errors.New("finance dependency is unavailable")
	ErrStrongDuplicate    = errors.New("strong invoice duplicate")
	ErrSuspectedDuplicate = errors.New("suspected invoice duplicate requires acknowledgement")
)

type DuplicateError struct {
	Strong            bool
	MatchedRequestIDs []string
}

func (e *DuplicateError) Error() string {
	if e.Strong {
		return ErrStrongDuplicate.Error()
	}
	return ErrSuspectedDuplicate.Error()
}

func (e *DuplicateError) Unwrap() error {
	if e.Strong {
		return ErrStrongDuplicate
	}
	return ErrSuspectedDuplicate
}

type Actor struct{ ID, TenantID, DisplayName, Kind, CredentialID string }

type ReferenceSnapshot struct {
	ID          string `json:"id"`
	Revision    int64  `json:"revision"`
	Digest      string `json:"digest"`
	DisplayName string `json:"display_name"`
}

type EvidenceRef struct {
	FileID            string `json:"file_id"`
	Version           int64  `json:"version"`
	Purpose           string `json:"purpose"`
	SHA256            string `json:"sha256,omitempty"`
	MediaType         string `json:"media_type,omitempty"`
	SizeBytes         int64  `json:"size_bytes,omitempty"`
	ValidationProfile string `json:"validation_profile,omitempty"`
	SecurityStatus    string `json:"security_status,omitempty"`
}

type InvoiceSnapshot struct {
	Code        string `json:"code,omitempty"`
	Number      string `json:"number,omitempty"`
	IssuerName  string `json:"issuer_name"`
	InvoiceDate string `json:"invoice_date"`
	AmountMinor int64  `json:"amount_minor"`
}

type Line struct {
	ID           string           `json:"id"`
	BusinessDate string           `json:"business_date,omitempty"`
	CategoryKey  string           `json:"category_key,omitempty"`
	CategoryName string           `json:"category_name,omitempty"`
	CategoryRev  int64            `json:"category_revision,omitempty"`
	Purpose      string           `json:"purpose"`
	AmountMinor  int64            `json:"amount_minor"`
	Evidence     []EvidenceRef    `json:"evidence,omitempty"`
	Invoice      *InvoiceSnapshot `json:"invoice,omitempty"`
}

type PayeeSnapshot struct {
	Kind           string            `json:"kind"`
	EntityKind     string            `json:"entity_kind,omitempty"`
	PaymentMethod  string            `json:"payment_method,omitempty"`
	Reference      ReferenceSnapshot `json:"reference,omitempty"`
	DisplayName    string            `json:"display_name"`
	BankName       string            `json:"bank_name"`
	AccountNumber  string            `json:"account_number,omitempty"`
	AccountLast4   string            `json:"account_last4"`
	DelegateReason string            `json:"delegate_reason,omitempty"`
}

type AttributionSnapshot struct {
	Department *ReferenceSnapshot `json:"department,omitempty"`
	Project    *ReferenceSnapshot `json:"project,omitempty"`
	Contract   *ReferenceSnapshot `json:"contract,omitempty"`
}

type ApprovalSnapshot struct {
	Policy   string    `json:"policy"`
	Status   string    `json:"status"`
	Revision int64     `json:"revision"`
	FrozenAt time.Time `json:"frozen_at,omitempty"`
}

type DetailSnapshot struct {
	Applicant                   ReferenceSnapshot   `json:"applicant"`
	Payee                       PayeeSnapshot       `json:"payee"`
	Lines                       []Line              `json:"lines"`
	Evidence                    []EvidenceRef       `json:"evidence,omitempty"`
	Attribution                 AttributionSnapshot `json:"attribution,omitempty"`
	MissingEvidenceAcknowledged bool                `json:"missing_evidence_acknowledged"`
	Approval                    ApprovalSnapshot    `json:"approval"`
	Duplicate                   DuplicateAssessment `json:"duplicate"`
}

type DuplicateAssessment struct {
	Status            string    `json:"status"`
	MatchedRequestIDs []string  `json:"matched_request_ids,omitempty"`
	CheckedAt         time.Time `json:"checked_at,omitempty"`
}

type PaymentDelivery struct {
	CommandID               string    `json:"command_id"`
	RequestID               string    `json:"request_id"`
	SourceRequestRevision   int64     `json:"source_request_revision"`
	ActualAmountMinor       int64     `json:"actual_amount_minor"`
	AllocatedAmountMinor    int64     `json:"allocated_amount_minor"`
	OverageAmountMinor      int64     `json:"overage_amount_minor"`
	OccurredOn              string    `json:"occurred_on"`
	Note                    string    `json:"note"`
	OverageReason           string    `json:"overage_reason,omitempty"`
	Status                  string    `json:"status"`
	AttemptCount            int64     `json:"attempt_count"`
	MoneyPaymentID          string    `json:"money_payment_id,omitempty"`
	MoneyPaymentRevision    int64     `json:"money_payment_revision,omitempty"`
	MoneyAllocationID       string    `json:"money_allocation_id,omitempty"`
	MoneyAllocationRevision int64     `json:"money_allocation_revision,omitempty"`
	MoneyObligationID       string    `json:"money_obligation_id,omitempty"`
	MoneyObligationRevision int64     `json:"money_obligation_revision,omitempty"`
	LastErrorCode           string    `json:"last_error_code,omitempty"`
	OperatorSubjectID       string    `json:"operator_subject_id"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

type Request struct {
	ID                  string            `json:"id"`
	TenantID            string            `json:"tenant_id"`
	ApplicantSubjectID  string            `json:"applicant_subject_id"`
	RequestType         string            `json:"request_type"`
	Title               string            `json:"title"`
	AmountMinor         int64             `json:"amount_minor"`
	Currency            string            `json:"currency"`
	Status              string            `json:"status"`
	Details             DetailSnapshot    `json:"details"`
	ApprovalPolicy      string            `json:"approval_policy"`
	ApprovalStatus      string            `json:"approval_status"`
	PaidMinor           int64             `json:"paid_minor"`
	AllocatedMinor      int64             `json:"allocated_minor"`
	OverageMinor        int64             `json:"overage_minor"`
	PaymentUnknownCount int64             `json:"payment_unknown_count"`
	SubmittedAt         *time.Time        `json:"submitted_at,omitempty"`
	WithdrawnAt         *time.Time        `json:"withdrawn_at,omitempty"`
	WithdrawalReason    string            `json:"withdrawal_reason,omitempty"`
	Payments            []PaymentDelivery `json:"payments,omitempty"`
	Revision            int64             `json:"revision"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
	ArchivedAt          *time.Time        `json:"archived_at,omitempty"`
	DeletedAt           *time.Time        `json:"deleted_at,omitempty"`
	AllowedActions      []string          `json:"allowed_actions,omitempty"`
}
type CreateInput struct {
	CommandID   string         `json:"command_id"`
	RequestType string         `json:"request_type"`
	Title       string         `json:"title"`
	AmountMinor int64          `json:"amount_minor"`
	Currency    string         `json:"currency"`
	Details     DetailSnapshot `json:"details"`
}
type UpdateInput struct {
	CommandID        string         `json:"command_id"`
	ExpectedRevision int64          `json:"expected_revision"`
	Title            string         `json:"title"`
	AmountMinor      int64          `json:"amount_minor"`
	Details          DetailSnapshot `json:"details"`
}

type SubmitInput struct {
	CommandID                      string          `json:"command_id"`
	ExpectedRevision               int64           `json:"expected_revision"`
	MissingEvidenceAcknowledged    bool            `json:"missing_evidence_acknowledged"`
	SuspectedDuplicateAcknowledged bool            `json:"suspected_duplicate_acknowledged"`
	ValidatedDetails               *DetailSnapshot `json:"-"`
}

type WithdrawInput struct {
	CommandID        string `json:"command_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Reason           string `json:"reason"`
}

type RestoreInput struct {
	CommandID string `json:"command_id"`
}

type DeleteDraftInput struct {
	CommandID    string `json:"command_id"`
	Confirmation string `json:"confirmation"`
	Reason       string `json:"reason"`
}

type ConfirmPaymentInput struct {
	CommandID         string `json:"command_id"`
	ExpectedRevision  int64  `json:"expected_revision"`
	ActualAmountMinor int64  `json:"actual_amount_minor"`
	OccurredOn        string `json:"occurred_on"`
	Note              string `json:"note"`
	OverageConfirmed  bool   `json:"overage_confirmed"`
	OverageReason     string `json:"overage_reason"`
}

type BatchPaymentItem struct {
	RequestID string `json:"request_id"`
	ConfirmPaymentInput
}
type BatchPaymentInput struct {
	Items []BatchPaymentItem `json:"items"`
}
type BatchPaymentResult struct {
	Items []BatchPaymentItemResult `json:"items"`
}
type BatchPaymentItemResult struct {
	RequestID string   `json:"request_id"`
	Request   *Request `json:"request,omitempty"`
	ErrorCode string   `json:"error_code,omitempty"`
	Detail    string   `json:"detail,omitempty"`
}

type MoneyPaymentRequest struct {
	SchemaVersion       string        `json:"schema_version"`
	CommandID           string        `json:"command_id"`
	TenantID            string        `json:"tenant_id"`
	ApplicationID       string        `json:"application_id"`
	ApplicationRevision int64         `json:"application_revision"`
	ApplicationDigest   string        `json:"application_digest"`
	RequestType         string        `json:"request_type"`
	Title               string        `json:"title"`
	TotalPayableMinor   int64         `json:"total_payable_minor"`
	ActualAmountMinor   int64         `json:"actual_amount_minor"`
	OccurredOn          string        `json:"occurred_on"`
	Note                string        `json:"note"`
	Payee               PayeeSnapshot `json:"payee"`
}
type MoneyPaymentResult struct {
	Status                  string `json:"status"`
	MoneyPaymentID          string `json:"money_payment_id"`
	MoneyPaymentRevision    int64  `json:"money_payment_revision"`
	MoneyAllocationID       string `json:"money_allocation_id,omitempty"`
	MoneyAllocationRevision int64  `json:"money_allocation_revision,omitempty"`
	MoneyObligationID       string `json:"money_obligation_id"`
	MoneyObligationRevision int64  `json:"money_obligation_revision"`
	AllocatedAmountMinor    int64  `json:"allocated_amount_minor"`
	OverageAmountMinor      int64  `json:"overage_amount_minor"`
	Replayed                bool   `json:"replayed"`
}
type ListFilter struct {
	ApplicantSubjectID string
	RequestType        string
	Status             string
	SubmittedFrom      string
	SubmittedTo        string
	UpdatedSince       *time.Time
	BeforeUpdatedAt    *time.Time
	BeforeID           string
	Limit              int
	IncludeArchived    bool
	IncludeDeleted     bool
}
type Page struct {
	Items      []Request `json:"items"`
	NextCursor string    `json:"next_cursor,omitempty"`
}

type EvidenceVersion struct {
	FileID            string
	Version           int64
	Status            string
	SHA256            string
	MediaType         string
	SizeBytes         int64
	ValidationProfile string
	SecurityStatus    string
}

type ExpenseCategory struct {
	ID          string    `json:"id"`
	Key         string    `json:"key"`
	DisplayName string    `json:"display_name"`
	Source      string    `json:"source"`
	Enabled     bool      `json:"enabled"`
	Revision    int64     `json:"revision"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CategoryCreateInput struct {
	CommandID   string `json:"command_id"`
	Key         string `json:"key"`
	DisplayName string `json:"display_name"`
	Reason      string `json:"reason"`
}

type CategoryUpdateInput struct {
	CommandID        string `json:"command_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	DisplayName      string `json:"display_name"`
	Enabled          bool   `json:"enabled"`
	Reason           string `json:"reason"`
}

type CategoryPage struct {
	Items []ExpenseCategory `json:"items"`
}
