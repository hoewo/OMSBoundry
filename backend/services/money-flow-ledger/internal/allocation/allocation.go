package allocation

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

const (
	StatusActive   = "active"
	StatusReversed = "reversed"

	SourceKindManual = "manual"

	AuditActionApplied = "allocation_applied"
)

var (
	uuidPattern   = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	amountPattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?$`)
)

type Allocation struct {
	ID                    string
	CashTransactionID     string
	CashDescription       string
	CashType              string
	ObligationID          string
	ObligationRevision    int64
	ObligationTitle       string
	ObligationDirection   string
	LedgerOwnerPartyID    string
	LedgerOwnerLegalName  string
	CounterpartyPartyID   string
	CounterpartyLegalName string
	Amount                string
	Currency              string
	SourceKind            string
	ManualSourceReference string
	Status                string
	AppliedBy             audit.ActorRef
	AppliedAt             time.Time
	Reversal              *Reversal
}

type ApplyInput struct {
	CommandID                  string
	CashTransactionID          string
	ObligationID               string
	ExpectedObligationRevision int64
	Amount                     string
	ManualSourceReference      string
	Actor                      audit.ActorRef
}

type ApplyCommand struct {
	CommandID   string
	PayloadHash []byte
	Actor       audit.ActorRef
	AppliedAt   time.Time
}

type ApplyResult struct {
	Allocation Allocation
	Replayed   bool
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

type IdempotencyConflictError struct {
	CommandID string
}

func (e *IdempotencyConflictError) Error() string {
	return "该 Command ID 已用于不同的资金分配意图"
}

type CashTransactionNotFoundError struct {
	CashTransactionID string
}

func (e *CashTransactionNotFoundError) Error() string { return "选择的实际收付款不存在" }

type CashTransactionStateConflictError struct {
	Status string
}

func (e *CashTransactionStateConflictError) Error() string {
	return "已冲销的实际收付款不能形成新的资金分配"
}

type ObligationNotFoundError struct {
	ObligationID string
}

func (e *ObligationNotFoundError) Error() string { return "选择的义务不存在" }

type ObligationStateConflictError struct {
	Status string
}

func (e *ObligationStateConflictError) Error() string {
	return "只有 open 状态的义务可以分配资金"
}

type ObligationRevisionConflictError struct {
	Expected int64
	Actual   int64
}

func (e *ObligationRevisionConflictError) Error() string {
	return "义务 revision 已经变化，请刷新后重试"
}

type CompatibilityConflictError struct {
	Reason string
}

func (e *CompatibilityConflictError) Error() string {
	return "实际收付款与义务的方向、主体或币种不兼容"
}

type CashBalanceConflictError struct {
	Requested string
	Available string
}

func (e *CashBalanceConflictError) Error() string {
	return "分配金额超过该笔资金的未分配金额"
}

type ObligationBalanceConflictError struct {
	Requested string
	Available string
}

func (e *ObligationBalanceConflictError) Error() string {
	return "分配金额超过该义务的剩余金额"
}

type Repository interface {
	Apply(context.Context, Allocation, ApplyCommand, audit.Entry) (Allocation, bool, error)
	Reverse(context.Context, Reversal, ReverseCommand, audit.Entry) (Reversal, bool, error)
	List(context.Context) ([]Allocation, error)
	ListForCashTransaction(context.Context, string) ([]Allocation, error)
	ListForObligation(context.Context, string) ([]Allocation, error)
	Get(context.Context, string) (Allocation, bool, error)
}

type Service struct {
	repository Repository
	now        func() time.Time
	newID      func() (string, error)
}

func NewService(repository Repository) *Service {
	return &Service{
		repository: repository,
		now:        func() time.Time { return time.Now().UTC() },
		newID:      randomUUID,
	}
}

func NewCommandID() (string, error) {
	return randomUUID()
}

func (s *Service) Apply(ctx context.Context, input ApplyInput) (ApplyResult, error) {
	normalized, err := normalizeApplyInput(input)
	if err != nil {
		return ApplyResult{}, err
	}
	payloadHash, err := applyPayloadHash(normalized)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("hash allocation apply intent: %w", err)
	}
	allocationID, err := s.newID()
	if err != nil {
		return ApplyResult{}, fmt.Errorf("generate allocation ID: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return ApplyResult{}, fmt.Errorf("generate allocation audit ID: %w", err)
	}
	now := s.now()
	created := Allocation{
		ID:                    allocationID,
		CashTransactionID:     normalized.CashTransactionID,
		ObligationID:          normalized.ObligationID,
		ObligationRevision:    normalized.ExpectedObligationRevision,
		Amount:                normalized.Amount,
		SourceKind:            SourceKindManual,
		ManualSourceReference: normalized.ManualSourceReference,
		Status:                StatusActive,
		AppliedBy:             normalized.Actor,
		AppliedAt:             now,
	}
	persisted, replayed, err := s.repository.Apply(ctx, created, ApplyCommand{
		CommandID:   normalized.CommandID,
		PayloadHash: payloadHash,
		Actor:       normalized.Actor,
		AppliedAt:   now,
	}, audit.Entry{
		ID:         auditID,
		ObjectType: audit.ObjectAllocation,
		ObjectID:   allocationID,
		Action:     AuditActionApplied,
		CommandID:  normalized.CommandID,
		Actor:      normalized.Actor,
		OccurredAt: now,
		Summary:    "人工分配实际资金至义务",
		Details: map[string]any{
			"cash_transaction_id":     normalized.CashTransactionID,
			"obligation_id":           normalized.ObligationID,
			"obligation_revision":     normalized.ExpectedObligationRevision,
			"amount":                  normalized.Amount,
			"source_kind":             SourceKindManual,
			"manual_source_reference": normalized.ManualSourceReference,
		},
	})
	if err != nil {
		return ApplyResult{}, fmt.Errorf("apply allocation: %w", err)
	}
	return ApplyResult{Allocation: persisted, Replayed: replayed}, nil
}

func (s *Service) List(ctx context.Context) ([]Allocation, error) {
	values, err := s.repository.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list allocations: %w", err)
	}
	return values, nil
}

func (s *Service) ListForCashTransaction(ctx context.Context, cashTransactionID string) ([]Allocation, error) {
	id, err := normalizeUUID("cash_transaction_id", cashTransactionID, "实际收付款标识无效")
	if err != nil {
		return nil, err
	}
	values, err := s.repository.ListForCashTransaction(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list allocations for cash transaction: %w", err)
	}
	return values, nil
}

func (s *Service) ListForObligation(ctx context.Context, obligationID string) ([]Allocation, error) {
	id, err := normalizeUUID("obligation_id", obligationID, "义务标识无效")
	if err != nil {
		return nil, err
	}
	values, err := s.repository.ListForObligation(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list allocations for obligation: %w", err)
	}
	return values, nil
}

func (s *Service) Get(ctx context.Context, allocationID string) (Allocation, bool, error) {
	id := strings.ToLower(strings.TrimSpace(allocationID))
	if !uuidPattern.MatchString(id) {
		return Allocation{}, false, &ValidationError{Field: "allocation_id", Message: "资金分配标识无效"}
	}
	value, found, err := s.repository.Get(ctx, id)
	if err != nil {
		return Allocation{}, false, fmt.Errorf("get allocation: %w", err)
	}
	return value, found, nil
}

func normalizeApplyInput(input ApplyInput) (ApplyInput, error) {
	commandID, err := normalizeUUID("command_id", input.CommandID, "Command ID 无效，请刷新页面后重试")
	if err != nil {
		return ApplyInput{}, err
	}
	cashTransactionID, err := normalizeUUID("cash_transaction_id", input.CashTransactionID, "请选择有效的实际收付款")
	if err != nil {
		return ApplyInput{}, err
	}
	obligationID, err := normalizeUUID("obligation_id", input.ObligationID, "请选择有效的义务")
	if err != nil {
		return ApplyInput{}, err
	}
	if input.ExpectedObligationRevision < 1 {
		return ApplyInput{}, &ValidationError{Field: "expected_obligation_revision", Message: "Expected Obligation Revision 无效，请刷新后重试"}
	}
	amount, err := normalizeAmount(input.Amount)
	if err != nil {
		return ApplyInput{}, err
	}
	manualSourceReference := strings.TrimSpace(input.ManualSourceReference)
	if manualSourceReference == "" {
		return ApplyInput{}, &ValidationError{Field: "manual_source_reference", Message: "人工分配依据不能为空"}
	}
	if len([]rune(manualSourceReference)) > 200 {
		return ApplyInput{}, &ValidationError{Field: "manual_source_reference", Message: "人工分配依据不能超过 200 个字符"}
	}
	actor, err := audit.NormalizeActor(input.Actor)
	if err != nil {
		return ApplyInput{}, &ValidationError{Field: "actor", Message: err.Error()}
	}
	return ApplyInput{
		CommandID:                  commandID,
		CashTransactionID:          cashTransactionID,
		ObligationID:               obligationID,
		ExpectedObligationRevision: input.ExpectedObligationRevision,
		Amount:                     amount,
		ManualSourceReference:      manualSourceReference,
		Actor:                      actor,
	}, nil
}

func normalizeUUID(field, value, message string) (string, error) {
	id := strings.ToLower(strings.TrimSpace(value))
	if !uuidPattern.MatchString(id) {
		return "", &ValidationError{Field: field, Message: message}
	}
	return id, nil
}

func normalizeAmount(value string) (string, error) {
	return normalizeDecimal(value, true)
}

func normalizeStoredAmount(value string) (string, error) {
	return normalizeDecimal(value, false)
}

func normalizeDecimal(value string, requirePositive bool) (string, error) {
	raw := strings.TrimSpace(value)
	if !amountPattern.MatchString(raw) {
		return "", &ValidationError{Field: "amount", Message: "分配金额必须是大于 0 的普通十进制数"}
	}
	parts := strings.SplitN(raw, ".", 2)
	integer := strings.TrimLeft(parts[0], "0")
	if integer == "" {
		integer = "0"
	}
	if len(integer) > 18 {
		return "", &ValidationError{Field: "amount", Message: "分配金额整数部分不能超过 18 位"}
	}
	fraction := ""
	if len(parts) == 2 {
		if len(parts[1]) > 6 {
			return "", &ValidationError{Field: "amount", Message: "分配金额小数部分不能超过 6 位"}
		}
		fraction = strings.TrimRight(parts[1], "0")
	}
	if integer == "0" && fraction == "" {
		if requirePositive {
			return "", &ValidationError{Field: "amount", Message: "分配金额必须大于 0"}
		}
		return "0", nil
	}
	if fraction == "" {
		return integer, nil
	}
	return integer + "." + fraction, nil
}

func compareAmounts(left, right string) (int, error) {
	leftUnits, err := amountUnits(left)
	if err != nil {
		return 0, err
	}
	rightUnits, err := amountUnits(right)
	if err != nil {
		return 0, err
	}
	return leftUnits.Cmp(rightUnits), nil
}

func amountUnits(value string) (*big.Int, error) {
	normalized, err := normalizeStoredAmount(value)
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(normalized, ".", 2)
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	fraction += strings.Repeat("0", 6-len(fraction))
	units := new(big.Int)
	if _, ok := units.SetString(parts[0]+fraction, 10); !ok {
		return nil, fmt.Errorf("parse amount units")
	}
	return units, nil
}

func applyPayloadHash(input ApplyInput) ([]byte, error) {
	payload, err := json.Marshal(struct {
		Format                     string `json:"format"`
		CashTransactionID          string `json:"cash_transaction_id"`
		ObligationID               string `json:"obligation_id"`
		ExpectedObligationRevision int64  `json:"expected_obligation_revision"`
		Amount                     string `json:"amount"`
		SourceKind                 string `json:"source_kind"`
		ManualSourceReference      string `json:"manual_source_reference"`
		ActorKind                  string `json:"actor_kind"`
		ActorID                    string `json:"actor_id"`
	}{
		Format:                     "money-flow-allocation-apply-v1",
		CashTransactionID:          input.CashTransactionID,
		ObligationID:               input.ObligationID,
		ExpectedObligationRevision: input.ExpectedObligationRevision,
		Amount:                     input.Amount,
		SourceKind:                 SourceKindManual,
		ManualSourceReference:      input.ManualSourceReference,
		ActorKind:                  input.Actor.Kind,
		ActorID:                    input.Actor.ID,
	})
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(payload)
	return hash[:], nil
}

func randomUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		value[0:4], value[4:6], value[6:8], value[8:10], value[10:16],
	), nil
}
