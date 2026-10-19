package obligation

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

const firstRevision int64 = 1

const (
	RevisionKindConfirmation     = "confirmation"
	RevisionKindAmountAdjustment = "amount_adjustment"
)

type Revision struct {
	ObligationID          string
	Revision              int64
	Direction             Direction
	Title                 string
	LedgerOwnerPartyID    string
	Creditor              PartySnapshot
	Debtor                PartySnapshot
	EffectiveAmount       string
	Currency              string
	DueOn                 string
	SourceKind            string
	ManualSourceReference string
	SourceApplication     string
	SourceResourceType    string
	SourceResourceID      string
	SourceFactRevision    int64
	SourceDigest          string
	SourceDraftVersion    int64
	RevisionKind          string
	SourceRevision        int64
	RecordedBy            audit.ActorRef
	RecordedAt            time.Time
}

type Details struct {
	Obligation  Obligation
	Revisions   []Revision
	Adjustments []Adjustment
	Remissions  []Remission
	Closure     *Closure
}

type ConfirmInput struct {
	CommandID            string
	ObligationID         string
	ExpectedDraftVersion int64
	Actor                audit.ActorRef
}

type ConfirmationCommand struct {
	CommandID   string
	PayloadHash []byte
	Actor       audit.ActorRef
	ConfirmedAt time.Time
}

type ConfirmResult struct {
	Obligation Obligation
	Revision   Revision
	Replayed   bool
}

type ConfirmationIdempotencyConflictError struct {
	CommandID string
}

func (e *ConfirmationIdempotencyConflictError) Error() string {
	return "该 Command ID 已用于不同的义务确认意图"
}

type ObligationNotFoundError struct {
	ObligationID string
}

func (e *ObligationNotFoundError) Error() string { return "义务不存在" }

type ObligationStateConflictError struct {
	Status string
}

func (e *ObligationStateConflictError) Error() string {
	return "只有草稿状态的义务可以确认"
}

type DraftVersionConflictError struct {
	Expected int64
	Actual   int64
}

func (e *DraftVersionConflictError) Error() string {
	return "义务草稿版本已经变化，请刷新后重试"
}

func (s *Service) Confirm(ctx context.Context, input ConfirmInput) (ConfirmResult, error) {
	normalized, err := normalizeConfirmInput(input)
	if err != nil {
		return ConfirmResult{}, err
	}
	payloadHash, err := confirmationPayloadHash(normalized)
	if err != nil {
		return ConfirmResult{}, fmt.Errorf("hash obligation confirmation intent: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return ConfirmResult{}, fmt.Errorf("generate obligation confirmation audit ID: %w", err)
	}
	now := s.now()
	confirmed, revision, replayed, err := s.repository.Confirm(
		ctx,
		normalized.ObligationID,
		normalized.ExpectedDraftVersion,
		ConfirmationCommand{
			CommandID:   normalized.CommandID,
			PayloadHash: payloadHash,
			Actor:       normalized.Actor,
			ConfirmedAt: now,
		},
		audit.Entry{
			ID:         auditID,
			ObjectType: audit.ObjectObligation,
			ObjectID:   normalized.ObligationID,
			Action:     AuditActionConfirmed,
			CommandID:  normalized.CommandID,
			Actor:      normalized.Actor,
			OccurredAt: now,
			Summary:    "确认义务并形成 revision 1",
			Details: map[string]any{
				"expected_draft_version": normalized.ExpectedDraftVersion,
				"resulting_revision":     firstRevision,
				"resulting_status":       StatusOpen,
			},
		},
	)
	if err != nil {
		return ConfirmResult{}, fmt.Errorf("confirm obligation: %w", err)
	}
	return ConfirmResult{Obligation: confirmed, Revision: revision, Replayed: replayed}, nil
}

func normalizeConfirmInput(input ConfirmInput) (ConfirmInput, error) {
	commandID := strings.ToLower(strings.TrimSpace(input.CommandID))
	if !uuidPattern.MatchString(commandID) {
		return ConfirmInput{}, &ValidationError{Field: "command_id", Message: "Command ID 无效，请刷新页面后重试"}
	}
	obligationID := strings.ToLower(strings.TrimSpace(input.ObligationID))
	if !uuidPattern.MatchString(obligationID) {
		return ConfirmInput{}, &ValidationError{Field: "obligation_id", Message: "义务标识无效"}
	}
	if input.ExpectedDraftVersion < 1 {
		return ConfirmInput{}, &ValidationError{Field: "expected_draft_version", Message: "Expected Draft Version 无效，请刷新页面后重试"}
	}
	actor, err := audit.NormalizeActor(input.Actor)
	if err != nil {
		return ConfirmInput{}, &ValidationError{Field: "actor", Message: err.Error()}
	}
	return ConfirmInput{
		CommandID:            commandID,
		ObligationID:         obligationID,
		ExpectedDraftVersion: input.ExpectedDraftVersion,
		Actor:                actor,
	}, nil
}

func confirmationPayloadHash(input ConfirmInput) ([]byte, error) {
	payload, err := json.Marshal(struct {
		Format               string `json:"format"`
		ObligationID         string `json:"obligation_id"`
		ExpectedDraftVersion int64  `json:"expected_draft_version"`
		ActorKind            string `json:"actor_kind"`
		ActorID              string `json:"actor_id"`
	}{
		Format:               "money-flow-obligation-confirm-v1",
		ObligationID:         input.ObligationID,
		ExpectedDraftVersion: input.ExpectedDraftVersion,
		ActorKind:            input.Actor.Kind,
		ActorID:              input.Actor.ID,
	})
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(payload)
	return hash[:], nil
}
