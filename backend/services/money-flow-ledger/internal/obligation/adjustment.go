package obligation

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

type AdjustmentType string

const (
	AdjustmentTypeIncrease AdjustmentType = "increase"
	AdjustmentTypeDecrease AdjustmentType = "decrease"
)

func (t AdjustmentType) Label() string {
	switch t {
	case AdjustmentTypeIncrease:
		return "增加"
	case AdjustmentTypeDecrease:
		return "减少"
	default:
		return string(t)
	}
}

type Adjustment struct {
	ID                       string
	ObligationID             string
	SourceRevision           int64
	ResultingRevision        int64
	Type                     AdjustmentType
	Amount                   string
	PreviousEffectiveAmount  string
	ResultingEffectiveAmount string
	Currency                 string
	SourceKind               string
	Reason                   string
	AdjustedBy               audit.ActorRef
	AdjustedAt               time.Time
}

type AdjustAmountInput struct {
	CommandID               string
	ObligationID            string
	ExpectedCurrentRevision int64
	Type                    string
	Amount                  string
	Reason                  string
	Actor                   audit.ActorRef
}

type AdjustmentCommand struct {
	CommandID   string
	PayloadHash []byte
	Actor       audit.ActorRef
	AdjustedAt  time.Time
}

type AdjustAmountResult struct {
	Obligation Obligation
	Adjustment Adjustment
	Revision   Revision
	Replayed   bool
}

type AmountAdjustmentIdempotencyConflictError struct {
	CommandID string
}

func (e *AmountAdjustmentIdempotencyConflictError) Error() string {
	return "该 Command ID 已用于不同的义务金额调整意图"
}

type AmountAdjustmentStateConflictError struct {
	Status string
}

func (e *AmountAdjustmentStateConflictError) Error() string {
	return "只有 open 状态的义务可以调整金额"
}

type CurrentRevisionConflictError struct {
	Expected int64
	Actual   int64
}

func (e *CurrentRevisionConflictError) Error() string {
	return "义务 current revision 已变化，请刷新后重试"
}

type AmountAdjustmentResultConflictError struct {
	Reason                   string
	CurrentEffectiveAmount   string
	AdjustmentAmount         string
	ResultingEffectiveAmount string
	NetAllocatedAmount       string
	RemittedAmount           string
}

func (e *AmountAdjustmentResultConflictError) Error() string {
	switch e.Reason {
	case "non_positive":
		return "调整后的义务金额必须大于 0"
	case "below_allocated":
		return "调整后的义务金额不能低于当前已分配净额；请先反向不再有效的 Allocation"
	case "below_allocated_and_remitted":
		return "调整后的义务金额不能低于当前已分配净额与已减免金额之和"
	case "out_of_range":
		return "调整后的义务金额超出可保存范围"
	default:
		return "义务金额调整结果无效"
	}
}

func (s *Service) AdjustAmount(ctx context.Context, input AdjustAmountInput) (AdjustAmountResult, error) {
	normalized, err := normalizeAdjustAmountInput(input)
	if err != nil {
		return AdjustAmountResult{}, err
	}
	payloadHash, err := adjustmentPayloadHash(normalized)
	if err != nil {
		return AdjustAmountResult{}, fmt.Errorf("hash obligation amount adjustment intent: %w", err)
	}
	adjustmentID, err := s.newID()
	if err != nil {
		return AdjustAmountResult{}, fmt.Errorf("generate obligation amount adjustment ID: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return AdjustAmountResult{}, fmt.Errorf("generate obligation amount adjustment audit ID: %w", err)
	}
	now := s.now()
	adjusted, adjustment, revision, replayed, err := s.repository.AdjustAmount(
		ctx,
		Adjustment{
			ID:             adjustmentID,
			ObligationID:   normalized.ObligationID,
			SourceRevision: normalized.ExpectedCurrentRevision,
			Type:           AdjustmentType(normalized.Type),
			Amount:         normalized.Amount,
			SourceKind:     SourceKindManual,
			Reason:         normalized.Reason,
			AdjustedBy:     normalized.Actor,
			AdjustedAt:     now,
		},
		AdjustmentCommand{
			CommandID:   normalized.CommandID,
			PayloadHash: payloadHash,
			Actor:       normalized.Actor,
			AdjustedAt:  now,
		},
		audit.Entry{
			ID:         auditID,
			ObjectType: audit.ObjectObligation,
			ObjectID:   normalized.ObligationID,
			Action:     AuditActionAmountAdjusted,
			CommandID:  normalized.CommandID,
			Actor:      normalized.Actor,
			OccurredAt: now,
			Summary:    "调整义务金额并形成下一 revision",
			Details: map[string]any{
				"expected_current_revision": normalized.ExpectedCurrentRevision,
				"adjustment_type":           normalized.Type,
				"adjustment_amount":         normalized.Amount,
				"source_kind":               SourceKindManual,
				"adjustment_reason":         normalized.Reason,
			},
		},
	)
	if err != nil {
		return AdjustAmountResult{}, fmt.Errorf("adjust obligation amount: %w", err)
	}
	return AdjustAmountResult{
		Obligation: adjusted,
		Adjustment: adjustment,
		Revision:   revision,
		Replayed:   replayed,
	}, nil
}

func normalizeAdjustAmountInput(input AdjustAmountInput) (AdjustAmountInput, error) {
	commandID := strings.ToLower(strings.TrimSpace(input.CommandID))
	if !uuidPattern.MatchString(commandID) {
		return AdjustAmountInput{}, &ValidationError{Field: "command_id", Message: "Command ID 无效，请刷新页面后重试"}
	}
	obligationID := strings.ToLower(strings.TrimSpace(input.ObligationID))
	if !uuidPattern.MatchString(obligationID) {
		return AdjustAmountInput{}, &ValidationError{Field: "obligation_id", Message: "义务标识无效"}
	}
	if input.ExpectedCurrentRevision < 1 {
		return AdjustAmountInput{}, &ValidationError{Field: "expected_current_revision", Message: "Expected Current Revision 无效，请刷新页面后重试"}
	}
	adjustmentType := AdjustmentType(strings.ToLower(strings.TrimSpace(input.Type)))
	if adjustmentType != AdjustmentTypeIncrease && adjustmentType != AdjustmentTypeDecrease {
		return AdjustAmountInput{}, &ValidationError{Field: "adjustment_type", Message: "请选择增加或减少"}
	}
	amount, err := normalizeAmountForField("adjustment_amount", "调整金额", input.Amount, true)
	if err != nil {
		return AdjustAmountInput{}, err
	}
	reason, err := requiredText("adjustment_reason", "调整原因不能为空", input.Reason, 200)
	if err != nil {
		return AdjustAmountInput{}, err
	}
	actor, err := audit.NormalizeActor(input.Actor)
	if err != nil {
		return AdjustAmountInput{}, &ValidationError{Field: "actor", Message: err.Error()}
	}
	return AdjustAmountInput{
		CommandID:               commandID,
		ObligationID:            obligationID,
		ExpectedCurrentRevision: input.ExpectedCurrentRevision,
		Type:                    string(adjustmentType),
		Amount:                  amount,
		Reason:                  reason,
		Actor:                   actor,
	}, nil
}

func adjustmentPayloadHash(input AdjustAmountInput) ([]byte, error) {
	payload, err := json.Marshal(struct {
		Format                  string `json:"format"`
		ObligationID            string `json:"obligation_id"`
		ExpectedCurrentRevision int64  `json:"expected_current_revision"`
		AdjustmentType          string `json:"adjustment_type"`
		AdjustmentAmount        string `json:"adjustment_amount"`
		SourceKind              string `json:"source_kind"`
		Reason                  string `json:"reason"`
		ActorKind               string `json:"actor_kind"`
		ActorID                 string `json:"actor_id"`
	}{
		Format:                  "money-flow-obligation-amount-adjust-v1",
		ObligationID:            input.ObligationID,
		ExpectedCurrentRevision: input.ExpectedCurrentRevision,
		AdjustmentType:          input.Type,
		AdjustmentAmount:        input.Amount,
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

func calculateAdjustedAmount(current, adjustment string, adjustmentType AdjustmentType) (string, error) {
	currentValue, ok := new(big.Rat).SetString(current)
	if !ok {
		return "", fmt.Errorf("parse current effective amount %q", current)
	}
	adjustmentValue, ok := new(big.Rat).SetString(adjustment)
	if !ok {
		return "", fmt.Errorf("parse adjustment amount %q", adjustment)
	}
	result := new(big.Rat)
	switch adjustmentType {
	case AdjustmentTypeIncrease:
		result.Add(currentValue, adjustmentValue)
	case AdjustmentTypeDecrease:
		result.Sub(currentValue, adjustmentValue)
	default:
		return "", fmt.Errorf("unsupported adjustment type %q", adjustmentType)
	}
	if result.Sign() <= 0 {
		return "", &AmountAdjustmentResultConflictError{
			Reason:                 "non_positive",
			CurrentEffectiveAmount: current,
			AdjustmentAmount:       adjustment,
		}
	}
	normalized, err := normalizeAmountForField("resulting_effective_amount", "调整后金额", result.FloatString(6), true)
	if err != nil {
		return "", &AmountAdjustmentResultConflictError{
			Reason:                 "out_of_range",
			CurrentEffectiveAmount: current,
			AdjustmentAmount:       adjustment,
		}
	}
	return normalized, nil
}

func compareDecimalAmounts(left, right string) (int, error) {
	leftValue, ok := new(big.Rat).SetString(left)
	if !ok {
		return 0, fmt.Errorf("parse amount %q", left)
	}
	rightValue, ok := new(big.Rat).SetString(right)
	if !ok {
		return 0, fmt.Errorf("parse amount %q", right)
	}
	return leftValue.Cmp(rightValue), nil
}
