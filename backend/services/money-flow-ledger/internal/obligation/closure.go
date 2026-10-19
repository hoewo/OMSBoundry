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

type ClosureBasis string

const (
	ClosureBasisAllocated ClosureBasis = "allocated"
	ClosureBasisRemitted  ClosureBasis = "remitted"
	ClosureBasisMixed     ClosureBasis = "mixed"
)

func (b ClosureBasis) Label() string {
	switch b {
	case ClosureBasisAllocated:
		return "全部由资金分配归零"
	case ClosureBasisRemitted:
		return "全部由减免归零"
	case ClosureBasisMixed:
		return "资金分配与减免组合归零"
	default:
		return string(b)
	}
}

type Closure struct {
	ID                      string
	ObligationID            string
	ObligationRevision      int64
	BalanceSequence         int64
	RecordedEffectiveAmount string
	NetAllocatedAmount      string
	RemittedAmount          string
	Currency                string
	SettlementBasis         ClosureBasis
	SourceKind              string
	Reason                  string
	ClosedBy                audit.ActorRef
	ClosedAt                time.Time
}

type CloseInput struct {
	CommandID               string
	ObligationID            string
	ExpectedCurrentRevision int64
	ExpectedBalanceSequence int64
	Reason                  string
	Actor                   audit.ActorRef
}

type ClosureCommand struct {
	CommandID   string
	PayloadHash []byte
	Actor       audit.ActorRef
	ClosedAt    time.Time
}

type CloseResult struct {
	Obligation Obligation
	Closure    Closure
	Replayed   bool
}

type ClosureIdempotencyConflictError struct {
	CommandID string
}

func (e *ClosureIdempotencyConflictError) Error() string {
	return "该 Command ID 已用于不同的义务关闭意图"
}

type ClosureStateConflictError struct {
	Status string
}

func (e *ClosureStateConflictError) Error() string {
	return "只有 open 状态的义务可以关闭"
}

type BalanceSequenceConflictError struct {
	Expected int64
	Actual   int64
}

func (e *BalanceSequenceConflictError) Error() string {
	return "义务余额事实已经变化，请刷新后重试"
}

type ClosureBalanceConflictError struct {
	Outstanding string
}

func (e *ClosureBalanceConflictError) Error() string {
	return "义务剩余金额必须归零后才能关闭"
}

func (s *Service) Close(ctx context.Context, input CloseInput) (CloseResult, error) {
	normalized, err := normalizeCloseInput(input)
	if err != nil {
		return CloseResult{}, err
	}
	payloadHash, err := closurePayloadHash(normalized)
	if err != nil {
		return CloseResult{}, fmt.Errorf("hash obligation closure intent: %w", err)
	}
	closureID, err := s.newID()
	if err != nil {
		return CloseResult{}, fmt.Errorf("generate obligation closure ID: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return CloseResult{}, fmt.Errorf("generate obligation closure audit ID: %w", err)
	}
	now := s.now()
	closed, closure, replayed, err := s.repository.Close(
		ctx,
		Closure{
			ID:                 closureID,
			ObligationID:       normalized.ObligationID,
			ObligationRevision: normalized.ExpectedCurrentRevision,
			BalanceSequence:    normalized.ExpectedBalanceSequence,
			SourceKind:         SourceKindManual,
			Reason:             normalized.Reason,
			ClosedBy:           normalized.Actor,
			ClosedAt:           now,
		},
		ClosureCommand{
			CommandID:   normalized.CommandID,
			PayloadHash: payloadHash,
			Actor:       normalized.Actor,
			ClosedAt:    now,
		},
		audit.Entry{
			ID:         auditID,
			ObjectType: audit.ObjectObligation,
			ObjectID:   normalized.ObligationID,
			Action:     AuditActionClosed,
			CommandID:  normalized.CommandID,
			Actor:      normalized.Actor,
			OccurredAt: now,
			Summary:    "关闭零余额义务",
			Details: map[string]any{
				"expected_current_revision": normalized.ExpectedCurrentRevision,
				"expected_balance_sequence": normalized.ExpectedBalanceSequence,
				"source_kind":               SourceKindManual,
				"closure_reason":            normalized.Reason,
			},
		},
	)
	if err != nil {
		return CloseResult{}, fmt.Errorf("close obligation: %w", err)
	}
	return CloseResult{Obligation: closed, Closure: closure, Replayed: replayed}, nil
}

func normalizeCloseInput(input CloseInput) (CloseInput, error) {
	commandID := strings.ToLower(strings.TrimSpace(input.CommandID))
	if !uuidPattern.MatchString(commandID) {
		return CloseInput{}, &ValidationError{Field: "command_id", Message: "Command ID 无效，请刷新页面后重试"}
	}
	obligationID := strings.ToLower(strings.TrimSpace(input.ObligationID))
	if !uuidPattern.MatchString(obligationID) {
		return CloseInput{}, &ValidationError{Field: "obligation_id", Message: "义务标识无效"}
	}
	if input.ExpectedCurrentRevision < 1 {
		return CloseInput{}, &ValidationError{Field: "expected_current_revision", Message: "Expected Current Revision 无效，请刷新页面后重试"}
	}
	if input.ExpectedBalanceSequence < 1 {
		return CloseInput{}, &ValidationError{Field: "expected_balance_sequence", Message: "Expected Balance Sequence 无效，请刷新页面后重试"}
	}
	reason, err := requiredText("closure_reason", "关闭原因不能为空", input.Reason, 200)
	if err != nil {
		return CloseInput{}, err
	}
	actor, err := audit.NormalizeActor(input.Actor)
	if err != nil {
		return CloseInput{}, &ValidationError{Field: "actor", Message: err.Error()}
	}
	return CloseInput{
		CommandID:               commandID,
		ObligationID:            obligationID,
		ExpectedCurrentRevision: input.ExpectedCurrentRevision,
		ExpectedBalanceSequence: input.ExpectedBalanceSequence,
		Reason:                  reason,
		Actor:                   actor,
	}, nil
}

func closurePayloadHash(input CloseInput) ([]byte, error) {
	payload, err := json.Marshal(struct {
		Format                  string `json:"format"`
		ObligationID            string `json:"obligation_id"`
		ExpectedCurrentRevision int64  `json:"expected_current_revision"`
		ExpectedBalanceSequence int64  `json:"expected_balance_sequence"`
		SourceKind              string `json:"source_kind"`
		Reason                  string `json:"reason"`
		ActorKind               string `json:"actor_kind"`
		ActorID                 string `json:"actor_id"`
	}{
		Format:                  "money-flow-obligation-closure-v1",
		ObligationID:            input.ObligationID,
		ExpectedCurrentRevision: input.ExpectedCurrentRevision,
		ExpectedBalanceSequence: input.ExpectedBalanceSequence,
		SourceKind:              SourceKindManual,
		Reason:                  input.Reason,
		ActorKind:               input.Actor.Kind,
		ActorID:                 input.Actor.ID,
	})
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(payload)
	return hash[:], nil
}
