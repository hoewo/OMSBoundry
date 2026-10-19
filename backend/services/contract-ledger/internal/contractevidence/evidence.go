package contractevidence

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"omsbizfoundry/contract-ledger/internal/audit"
	"omsbizfoundry/contract-ledger/internal/referenceoa"
)

var (
	ErrInvalid             = errors.New("invalid contract evidence input")
	ErrNotFound            = errors.New("contract not found")
	ErrRevisionConflict    = errors.New("contract evidence revision conflict")
	ErrIdempotencyConflict = errors.New("contract evidence command conflict")
	ErrEvidenceInvalid     = errors.New("Evidence reference invalid")
	ErrEvidenceUnavailable = errors.New("Evidence reference unavailable")
	uuidPattern            = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)

type Ref struct {
	FileID      string `json:"file_id"`
	Version     int    `json:"version"`
	Purpose     string `json:"purpose"`
	DisplayName string `json:"display_name,omitempty"`
}

type Actor struct {
	Kind        string `json:"kind"`
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type Snapshot struct {
	ContractID   string    `json:"contract_id"`
	TenantID     string    `json:"tenant_id"`
	Revision     int64     `json:"revision"`
	EvidenceRefs []Ref     `json:"evidence_refs"`
	Reason       string    `json:"reason,omitempty"`
	Actor        Actor     `json:"actor,omitempty"`
	RecordedAt   time.Time `json:"recorded_at,omitempty"`
}

type ReplaceInput struct {
	CommandID        string         `json:"command_id"`
	ExpectedRevision int64          `json:"expected_revision"`
	Reason           string         `json:"reason"`
	EvidenceRefs     []Ref          `json:"evidence_refs"`
	Actor            audit.ActorRef `json:"-"`
	TenantID         string         `json:"-"`
	ContractID       string         `json:"-"`
}

type Result struct {
	Snapshot Snapshot `json:"evidence"`
	Replayed bool     `json:"replayed"`
}

type Repository interface {
	Current(context.Context, string, string) (Snapshot, bool, error)
	Replace(context.Context, ReplaceInput, []byte, time.Time) (Result, error)
}

type Service struct {
	repository Repository
	validator  referenceoa.EvidenceValidator
	now        func() time.Time
}

func NewService(repository Repository, validator referenceoa.EvidenceValidator) (*Service, error) {
	if repository == nil || validator == nil {
		return nil, fmt.Errorf("contract evidence repository and validator are required")
	}
	return &Service{repository: repository, validator: validator, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (s *Service) Current(ctx context.Context, tenantID, contractID string) (Snapshot, bool, error) {
	tenantID = strings.ToLower(strings.TrimSpace(tenantID))
	contractID = strings.ToLower(strings.TrimSpace(contractID))
	if !uuidPattern.MatchString(tenantID) || !uuidPattern.MatchString(contractID) {
		return Snapshot{}, false, ErrInvalid
	}
	return s.repository.Current(ctx, tenantID, contractID)
}

func (s *Service) Replace(ctx context.Context, input ReplaceInput) (Result, error) {
	value, err := normalize(input)
	if err != nil {
		return Result{}, err
	}
	for index := range value.EvidenceRefs {
		ref := &value.EvidenceRefs[index]
		resolved, resolveErr := s.validator.ValidateFile(ctx, value.TenantID, ref.FileID, ref.Version)
		if resolveErr != nil {
			if errors.Is(resolveErr, referenceoa.ErrEvidenceUnavailable) {
				return Result{}, ErrEvidenceUnavailable
			}
			return Result{}, ErrEvidenceInvalid
		}
		if resolved.TenantID != value.TenantID || resolved.FileID != ref.FileID || resolved.ID != ref.FileID || resolved.Version != ref.Version || resolved.Revision != ref.Version || resolved.Status != "available" {
			return Result{}, ErrEvidenceInvalid
		}
		if ref.DisplayName == "" {
			ref.DisplayName = strings.TrimSpace(resolved.Name)
		}
	}
	payload, err := json.Marshal(struct {
		ContractID       string `json:"contract_id"`
		TenantID         string `json:"tenant_id"`
		CommandID        string `json:"command_id"`
		ExpectedRevision int64  `json:"expected_revision"`
		Reason           string `json:"reason"`
		EvidenceRefs     []Ref  `json:"evidence_refs"`
		ActorID          string `json:"actor_id"`
	}{value.ContractID, value.TenantID, value.CommandID, value.ExpectedRevision, value.Reason, value.EvidenceRefs, value.Actor.ID})
	if err != nil {
		return Result{}, err
	}
	hash := sha256.Sum256(payload)
	return s.repository.Replace(ctx, value, hash[:], s.now())
}

func normalize(input ReplaceInput) (ReplaceInput, error) {
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	input.ContractID = strings.ToLower(strings.TrimSpace(input.ContractID))
	input.TenantID = strings.ToLower(strings.TrimSpace(input.TenantID))
	input.Reason = strings.TrimSpace(input.Reason)
	actor, err := audit.NormalizeActor(input.Actor)
	if err != nil || actor.Kind != audit.ActorKindIAMSubject || !uuidPattern.MatchString(input.CommandID) || !uuidPattern.MatchString(input.ContractID) || !uuidPattern.MatchString(input.TenantID) || input.ExpectedRevision < 0 || input.ExpectedRevision >= 1<<53-1 || input.Reason == "" || len([]rune(input.Reason)) > 1000 || len(input.EvidenceRefs) > 20 {
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
		validPurpose := ref.Purpose == "contract_document" || ref.Purpose == "supplemental_agreement" || ref.Purpose == "contractor_agreement" || ref.Purpose == "supporting_document"
		if !uuidPattern.MatchString(ref.FileID) || ref.Version < 1 || ref.Version > 2147483647 || !validPurpose || len([]rune(ref.DisplayName)) > 500 || seen[key] {
			return ReplaceInput{}, ErrInvalid
		}
		seen[key] = true
	}
	return input, nil
}

func snapshotActor(value audit.ActorRef) Actor {
	return Actor{Kind: value.Kind, ID: value.ID, DisplayName: value.DisplayName}
}
