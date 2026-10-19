package allocation

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

const AuditActionReversed = "allocation_reversed"

type Reversal struct {
	ID                 string
	AllocationID       string
	CashTransactionID  string
	ObligationID       string
	ObligationRevision int64
	Amount             string
	Currency           string
	SourceKind         string
	Reason             string
	ReversedBy         audit.ActorRef
	ReversedAt         time.Time
}

type ReverseInput struct {
	CommandID    string
	AllocationID string
	Reason       string
	Actor        audit.ActorRef
}

type ReverseCommand struct {
	CommandID   string
	PayloadHash []byte
	Actor       audit.ActorRef
	ReversedAt  time.Time
}

type ReverseResult struct {
	Reversal Reversal
	Replayed bool
}

type ReversalIdempotencyConflictError struct {
	CommandID string
}

func (e *ReversalIdempotencyConflictError) Error() string {
	return "该 Command ID 已用于不同的反向核销意图"
}

type AllocationNotFoundError struct {
	AllocationID string
}

func (e *AllocationNotFoundError) Error() string {
	return "选择的资金分配不存在"
}

type AllocationAlreadyReversedError struct {
	AllocationID string
	ReversalID   string
}

type ReversalObligationStateConflictError struct {
	Status string
}

func (e *ReversalObligationStateConflictError) Error() string {
	return "只有 open 状态的义务可以反向核销资金分配"
}

func (e *AllocationAlreadyReversedError) Error() string {
	return "该资金分配已经完成反向核销"
}

func (s *Service) Reverse(ctx context.Context, input ReverseInput) (ReverseResult, error) {
	normalized, err := normalizeReverseInput(input)
	if err != nil {
		return ReverseResult{}, err
	}
	payloadHash, err := reversePayloadHash(normalized)
	if err != nil {
		return ReverseResult{}, fmt.Errorf("hash allocation reversal intent: %w", err)
	}
	reversalID, err := s.newID()
	if err != nil {
		return ReverseResult{}, fmt.Errorf("generate allocation reversal ID: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return ReverseResult{}, fmt.Errorf("generate allocation reversal audit ID: %w", err)
	}
	now := s.now()
	created := Reversal{
		ID:           reversalID,
		AllocationID: normalized.AllocationID,
		SourceKind:   SourceKindManual,
		Reason:       normalized.Reason,
		ReversedBy:   normalized.Actor,
		ReversedAt:   now,
	}
	persisted, replayed, err := s.repository.Reverse(ctx, created, ReverseCommand{
		CommandID:   normalized.CommandID,
		PayloadHash: payloadHash,
		Actor:       normalized.Actor,
		ReversedAt:  now,
	}, audit.Entry{
		ID:         auditID,
		ObjectType: audit.ObjectAllocationReversal,
		ObjectID:   reversalID,
		Action:     AuditActionReversed,
		CommandID:  normalized.CommandID,
		Actor:      normalized.Actor,
		OccurredAt: now,
		Summary:    "完整反向一条资金分配",
		Details: map[string]any{
			"allocation_id": normalized.AllocationID,
			"source_kind":   SourceKindManual,
			"reason":        normalized.Reason,
		},
	})
	if err != nil {
		return ReverseResult{}, fmt.Errorf("reverse allocation: %w", err)
	}
	return ReverseResult{Reversal: persisted, Replayed: replayed}, nil
}

func normalizeReverseInput(input ReverseInput) (ReverseInput, error) {
	commandID, err := normalizeUUID("command_id", input.CommandID, "Command ID 无效，请刷新页面后重试")
	if err != nil {
		return ReverseInput{}, err
	}
	allocationID, err := normalizeUUID("allocation_id", input.AllocationID, "资金分配标识无效")
	if err != nil {
		return ReverseInput{}, err
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		return ReverseInput{}, &ValidationError{Field: "reason", Message: "反向核销原因不能为空"}
	}
	if len([]rune(reason)) > 200 {
		return ReverseInput{}, &ValidationError{Field: "reason", Message: "反向核销原因不能超过 200 个字符"}
	}
	actor, err := audit.NormalizeActor(input.Actor)
	if err != nil {
		return ReverseInput{}, &ValidationError{Field: "actor", Message: err.Error()}
	}
	return ReverseInput{
		CommandID:    commandID,
		AllocationID: allocationID,
		Reason:       reason,
		Actor:        actor,
	}, nil
}

func reversePayloadHash(input ReverseInput) ([]byte, error) {
	payload, err := json.Marshal(struct {
		Format       string `json:"format"`
		AllocationID string `json:"allocation_id"`
		SourceKind   string `json:"source_kind"`
		Reason       string `json:"reason"`
		ActorKind    string `json:"actor_kind"`
		ActorID      string `json:"actor_id"`
	}{
		Format:       "money-flow-allocation-reversal-v1",
		AllocationID: input.AllocationID,
		SourceKind:   SourceKindManual,
		Reason:       input.Reason,
		ActorKind:    input.Actor.Kind,
		ActorID:      input.Actor.ID,
	})
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(payload)
	return hash[:], nil
}
