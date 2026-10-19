package cashtransaction

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

const AuditActionReversed = "cash_transaction_reversed"

type Reversal struct {
	ID                string
	CashTransactionID string
	TransactionType   Type
	Amount            string
	Currency          string
	SourceKind        string
	Reason            string
	ReversedBy        audit.ActorRef
	ReversedAt        time.Time
}

type ReverseInput struct {
	CommandID         string
	CashTransactionID string
	Reason            string
	Actor             audit.ActorRef
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
	return "该 Command ID 已用于不同的实际收付款冲销意图"
}

type TransactionNotFoundError struct {
	CashTransactionID string
}

func (e *TransactionNotFoundError) Error() string {
	return "选择的实际收付款不存在"
}

type TransactionAlreadyReversedError struct {
	CashTransactionID string
	ReversalID        string
}

func (e *TransactionAlreadyReversedError) Error() string {
	return "该实际收付款已经完成冲销"
}

type TransactionAllocatedConflictError struct {
	CashTransactionID string
	AllocatedAmount   string
}

func (e *TransactionAllocatedConflictError) Error() string {
	return "该实际收付款仍有未反向的资金分配，请先反向核销相关 Allocation"
}

func (s *Service) Reverse(ctx context.Context, input ReverseInput) (ReverseResult, error) {
	normalized, err := normalizeReverseInput(input)
	if err != nil {
		return ReverseResult{}, err
	}
	payloadHash, err := reversePayloadHash(normalized)
	if err != nil {
		return ReverseResult{}, fmt.Errorf("hash cash transaction reversal intent: %w", err)
	}
	reversalID, err := s.newID()
	if err != nil {
		return ReverseResult{}, fmt.Errorf("generate cash transaction reversal ID: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return ReverseResult{}, fmt.Errorf("generate cash transaction reversal audit ID: %w", err)
	}
	now := s.now()
	created := Reversal{
		ID:                reversalID,
		CashTransactionID: normalized.CashTransactionID,
		SourceKind:        SourceKindManual,
		Reason:            normalized.Reason,
		ReversedBy:        normalized.Actor,
		ReversedAt:        now,
	}
	persisted, replayed, err := s.repository.Reverse(ctx, created, ReverseCommand{
		CommandID:   normalized.CommandID,
		PayloadHash: payloadHash,
		Actor:       normalized.Actor,
		ReversedAt:  now,
	}, audit.Entry{
		ID:         auditID,
		ObjectType: audit.ObjectCashReversal,
		ObjectID:   reversalID,
		Action:     AuditActionReversed,
		CommandID:  normalized.CommandID,
		Actor:      normalized.Actor,
		OccurredAt: now,
		Summary:    "完整冲销一条实际收付款",
		Details: map[string]any{
			"cash_transaction_id": normalized.CashTransactionID,
			"source_kind":         SourceKindManual,
			"reason":              normalized.Reason,
		},
	})
	if err != nil {
		return ReverseResult{}, fmt.Errorf("reverse cash transaction: %w", err)
	}
	return ReverseResult{Reversal: persisted, Replayed: replayed}, nil
}

func normalizeReverseInput(input ReverseInput) (ReverseInput, error) {
	commandID := strings.ToLower(strings.TrimSpace(input.CommandID))
	if !uuidPattern.MatchString(commandID) {
		return ReverseInput{}, &ValidationError{Field: "command_id", Message: "Command ID 无效，请刷新页面后重试"}
	}
	cashTransactionID := strings.ToLower(strings.TrimSpace(input.CashTransactionID))
	if !uuidPattern.MatchString(cashTransactionID) {
		return ReverseInput{}, &ValidationError{Field: "cash_transaction_id", Message: "实际收付款标识无效"}
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		return ReverseInput{}, &ValidationError{Field: "reason", Message: "冲销原因不能为空"}
	}
	if len([]rune(reason)) > 200 {
		return ReverseInput{}, &ValidationError{Field: "reason", Message: "冲销原因不能超过 200 个字符"}
	}
	actor, err := audit.NormalizeActor(input.Actor)
	if err != nil {
		return ReverseInput{}, &ValidationError{Field: "actor", Message: err.Error()}
	}
	return ReverseInput{
		CommandID:         commandID,
		CashTransactionID: cashTransactionID,
		Reason:            reason,
		Actor:             actor,
	}, nil
}

func reversePayloadHash(input ReverseInput) ([]byte, error) {
	payload, err := json.Marshal(struct {
		Format            string `json:"format"`
		CashTransactionID string `json:"cash_transaction_id"`
		SourceKind        string `json:"source_kind"`
		Reason            string `json:"reason"`
		ActorKind         string `json:"actor_kind"`
		ActorID           string `json:"actor_id"`
	}{
		Format:            "money-flow-cash-transaction-reversal-v1",
		CashTransactionID: input.CashTransactionID,
		SourceKind:        SourceKindManual,
		Reason:            input.Reason,
		ActorKind:         input.Actor.Kind,
		ActorID:           input.Actor.ID,
	})
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(payload)
	return hash[:], nil
}
