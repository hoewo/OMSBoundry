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

type Remission struct {
	ID                      string
	ObligationID            string
	ObligationRevision      int64
	Amount                  string
	RecordedEffectiveAmount string
	Currency                string
	SourceKind              string
	Reason                  string
	ManualSourceReference   string
	RecordedBy              audit.ActorRef
	RecordedAt              time.Time
}

type RecordRemissionInput struct {
	CommandID               string
	ObligationID            string
	ExpectedCurrentRevision int64
	Amount                  string
	Reason                  string
	ManualSourceReference   string
	Actor                   audit.ActorRef
}

type RemissionCommand struct {
	CommandID   string
	PayloadHash []byte
	Actor       audit.ActorRef
	RecordedAt  time.Time
}

type RecordRemissionResult struct {
	Obligation Obligation
	Remission  Remission
	Replayed   bool
}

type RemissionIdempotencyConflictError struct {
	CommandID string
}

func (e *RemissionIdempotencyConflictError) Error() string {
	return "该 Command ID 已用于不同的义务减免意图"
}

type RemissionStateConflictError struct {
	Status string
}

func (e *RemissionStateConflictError) Error() string {
	return "只有 open 状态的义务可以记录减免"
}

type RemissionBalanceConflictError struct {
	Requested   string
	Outstanding string
}

func (e *RemissionBalanceConflictError) Error() string {
	return "减免金额不能超过义务当前剩余金额"
}

func (s *Service) RecordRemission(ctx context.Context, input RecordRemissionInput) (RecordRemissionResult, error) {
	normalized, err := normalizeRecordRemissionInput(input)
	if err != nil {
		return RecordRemissionResult{}, err
	}
	payloadHash, err := remissionPayloadHash(normalized)
	if err != nil {
		return RecordRemissionResult{}, fmt.Errorf("hash obligation remission intent: %w", err)
	}
	remissionID, err := s.newID()
	if err != nil {
		return RecordRemissionResult{}, fmt.Errorf("generate obligation remission ID: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return RecordRemissionResult{}, fmt.Errorf("generate obligation remission audit ID: %w", err)
	}
	now := s.now()
	obligationValue, remission, replayed, err := s.repository.RecordRemission(
		ctx,
		Remission{
			ID:                    remissionID,
			ObligationID:          normalized.ObligationID,
			ObligationRevision:    normalized.ExpectedCurrentRevision,
			Amount:                normalized.Amount,
			SourceKind:            SourceKindManual,
			Reason:                normalized.Reason,
			ManualSourceReference: normalized.ManualSourceReference,
			RecordedBy:            normalized.Actor,
			RecordedAt:            now,
		},
		RemissionCommand{
			CommandID:   normalized.CommandID,
			PayloadHash: payloadHash,
			Actor:       normalized.Actor,
			RecordedAt:  now,
		},
		audit.Entry{
			ID:         auditID,
			ObjectType: audit.ObjectObligation,
			ObjectID:   normalized.ObligationID,
			Action:     AuditActionRemissionRecorded,
			CommandID:  normalized.CommandID,
			Actor:      normalized.Actor,
			OccurredAt: now,
			Summary:    "记录义务减免",
			Details: map[string]any{
				"expected_current_revision": normalized.ExpectedCurrentRevision,
				"remission_amount":          normalized.Amount,
				"source_kind":               SourceKindManual,
				"remission_reason":          normalized.Reason,
				"manual_source_reference":   normalized.ManualSourceReference,
			},
		},
	)
	if err != nil {
		return RecordRemissionResult{}, fmt.Errorf("record obligation remission: %w", err)
	}
	return RecordRemissionResult{Obligation: obligationValue, Remission: remission, Replayed: replayed}, nil
}

func normalizeRecordRemissionInput(input RecordRemissionInput) (RecordRemissionInput, error) {
	commandID := strings.ToLower(strings.TrimSpace(input.CommandID))
	if !uuidPattern.MatchString(commandID) {
		return RecordRemissionInput{}, &ValidationError{Field: "command_id", Message: "Command ID 无效，请刷新页面后重试"}
	}
	obligationID := strings.ToLower(strings.TrimSpace(input.ObligationID))
	if !uuidPattern.MatchString(obligationID) {
		return RecordRemissionInput{}, &ValidationError{Field: "obligation_id", Message: "义务标识无效"}
	}
	if input.ExpectedCurrentRevision < 1 {
		return RecordRemissionInput{}, &ValidationError{Field: "expected_current_revision", Message: "Expected Current Revision 无效，请刷新页面后重试"}
	}
	amount, err := normalizeAmountForField("remission_amount", "减免金额", input.Amount, true)
	if err != nil {
		return RecordRemissionInput{}, err
	}
	reason, err := requiredText("remission_reason", "减免原因不能为空", input.Reason, 200)
	if err != nil {
		return RecordRemissionInput{}, err
	}
	sourceReference, err := requiredText("manual_source_reference", "人工来源依据不能为空", input.ManualSourceReference, 200)
	if err != nil {
		return RecordRemissionInput{}, err
	}
	actor, err := audit.NormalizeActor(input.Actor)
	if err != nil {
		return RecordRemissionInput{}, &ValidationError{Field: "actor", Message: err.Error()}
	}
	return RecordRemissionInput{
		CommandID:               commandID,
		ObligationID:            obligationID,
		ExpectedCurrentRevision: input.ExpectedCurrentRevision,
		Amount:                  amount,
		Reason:                  reason,
		ManualSourceReference:   sourceReference,
		Actor:                   actor,
	}, nil
}

func remissionPayloadHash(input RecordRemissionInput) ([]byte, error) {
	payload, err := json.Marshal(struct {
		Format                  string `json:"format"`
		ObligationID            string `json:"obligation_id"`
		ExpectedCurrentRevision int64  `json:"expected_current_revision"`
		Amount                  string `json:"amount"`
		SourceKind              string `json:"source_kind"`
		Reason                  string `json:"reason"`
		ManualSourceReference   string `json:"manual_source_reference"`
		ActorKind               string `json:"actor_kind"`
		ActorID                 string `json:"actor_id"`
	}{
		Format:                  "money-flow-obligation-remission-v1",
		ObligationID:            input.ObligationID,
		ExpectedCurrentRevision: input.ExpectedCurrentRevision,
		Amount:                  input.Amount,
		SourceKind:              SourceKindManual,
		Reason:                  input.Reason,
		ManualSourceReference:   input.ManualSourceReference,
		ActorKind:               input.Actor.Kind,
		ActorID:                 input.Actor.ID,
	})
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(payload)
	return hash[:], nil
}
