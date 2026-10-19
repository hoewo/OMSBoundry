package cashevidence

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

var (
	ErrInvalid             = errors.New("invalid cash evidence input")
	ErrNotFound            = errors.New("cash transaction not found")
	ErrRevisionConflict    = errors.New("cash evidence revision conflict")
	ErrIdempotencyConflict = errors.New("cash evidence command conflict")
	ErrEvidenceInvalid     = errors.New("evidence reference invalid")
	ErrEvidenceUnavailable = errors.New("evidence reference unavailable")
	uuidPattern            = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)

type Ref struct {
	FileID      string `json:"file_id"`
	Version     int64  `json:"version"`
	Purpose     string `json:"purpose"`
	DisplayName string `json:"display_name,omitempty"`
}

type FileReference struct {
	TenantID string
	FileID   string
	Version  int64
	Revision int64
	Status   string
	Name     string
}

type Actor struct {
	Kind        string `json:"kind"`
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type Snapshot struct {
	CashTransactionID string    `json:"cash_transaction_id"`
	Revision          int64     `json:"revision"`
	EvidenceRefs      []Ref     `json:"evidence_refs"`
	Reason            string    `json:"reason,omitempty"`
	Actor             Actor     `json:"actor,omitempty"`
	RecordedAt        time.Time `json:"recorded_at,omitempty"`
}

func snapshotActor(value audit.ActorRef) Actor {
	return Actor{Kind: value.Kind, ID: value.ID, DisplayName: value.DisplayName}
}

type ReplaceInput struct {
	CommandID         string         `json:"command_id"`
	ExpectedRevision  int64          `json:"expected_revision"`
	Reason            string         `json:"reason"`
	EvidenceRefs      []Ref          `json:"evidence_refs"`
	Actor             audit.ActorRef `json:"-"`
	TenantID          string         `json:"-"`
	CashTransactionID string         `json:"-"`
}

type Result struct {
	Snapshot Snapshot `json:"evidence"`
	Replayed bool     `json:"replayed"`
}

type Validator interface {
	Resolve(context.Context, string, Ref) (FileReference, error)
}

type Repository interface {
	Current(context.Context, string) (Snapshot, bool, error)
	Replace(context.Context, ReplaceInput, []byte, time.Time) (Result, error)
}

type Service struct {
	repository Repository
	validator  Validator
	now        func() time.Time
}

func NewService(repository Repository, validator Validator) (*Service, error) {
	if repository == nil || validator == nil {
		return nil, fmt.Errorf("cash evidence repository and validator are required")
	}
	return &Service{repository: repository, validator: validator, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (s *Service) Current(ctx context.Context, cashTransactionID string) (Snapshot, bool, error) {
	id := strings.ToLower(strings.TrimSpace(cashTransactionID))
	if !uuidPattern.MatchString(id) {
		return Snapshot{}, false, ErrInvalid
	}
	return s.repository.Current(ctx, id)
}

func (s *Service) Replace(ctx context.Context, input ReplaceInput) (Result, error) {
	value, err := normalize(input)
	if err != nil {
		return Result{}, err
	}
	for index := range value.EvidenceRefs {
		resolved, resolveErr := s.validator.Resolve(ctx, value.TenantID, value.EvidenceRefs[index])
		if resolveErr != nil {
			return Result{}, resolveErr
		}
		if resolved.TenantID != value.TenantID || resolved.FileID != value.EvidenceRefs[index].FileID || resolved.Version != value.EvidenceRefs[index].Version || resolved.Revision != value.EvidenceRefs[index].Version || resolved.Status != "available" {
			return Result{}, ErrEvidenceInvalid
		}
		if value.EvidenceRefs[index].DisplayName == "" {
			value.EvidenceRefs[index].DisplayName = strings.TrimSpace(resolved.Name)
		}
	}
	payload, err := json.Marshal(struct {
		CashTransactionID string `json:"cash_transaction_id"`
		CommandID         string `json:"command_id"`
		ExpectedRevision  int64  `json:"expected_revision"`
		Reason            string `json:"reason"`
		EvidenceRefs      []Ref  `json:"evidence_refs"`
		ActorID           string `json:"actor_id"`
	}{value.CashTransactionID, value.CommandID, value.ExpectedRevision, value.Reason, value.EvidenceRefs, value.Actor.ID})
	if err != nil {
		return Result{}, err
	}
	hash := sha256.Sum256(payload)
	return s.repository.Replace(ctx, value, hash[:], s.now())
}

func normalize(input ReplaceInput) (ReplaceInput, error) {
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	input.CashTransactionID = strings.ToLower(strings.TrimSpace(input.CashTransactionID))
	input.TenantID = strings.ToLower(strings.TrimSpace(input.TenantID))
	input.Reason = strings.TrimSpace(input.Reason)
	actor, err := audit.NormalizeActor(input.Actor)
	if err != nil || !uuidPattern.MatchString(input.CommandID) || !uuidPattern.MatchString(input.CashTransactionID) || !uuidPattern.MatchString(input.TenantID) || input.ExpectedRevision < 0 || input.ExpectedRevision >= 1<<53-1 || input.Reason == "" || len([]rune(input.Reason)) > 1000 || len(input.EvidenceRefs) > 20 {
		return ReplaceInput{}, ErrInvalid
	}
	input.Actor = actor
	if input.EvidenceRefs == nil {
		input.EvidenceRefs = []Ref{}
	}
	seen := map[string]bool{}
	for index := range input.EvidenceRefs {
		ref := &input.EvidenceRefs[index]
		ref.FileID = strings.ToLower(strings.TrimSpace(ref.FileID))
		ref.Purpose = strings.TrimSpace(ref.Purpose)
		ref.DisplayName = strings.TrimSpace(ref.DisplayName)
		key := fmt.Sprintf("%s:%d:%s", ref.FileID, ref.Version, ref.Purpose)
		if !uuidPattern.MatchString(ref.FileID) || ref.Version < 1 || ref.Version > 2147483647 || (ref.Purpose != "receipt_proof" && ref.Purpose != "payment_proof" && ref.Purpose != "supporting_document") || len([]rune(ref.DisplayName)) > 500 || seen[key] {
			return ReplaceInput{}, ErrInvalid
		}
		seen[key] = true
	}
	return input, nil
}
