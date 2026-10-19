package obligation

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

type CashExpectationAction string

const (
	CashExpectationActionSet   CashExpectationAction = "set"
	CashExpectationActionClear CashExpectationAction = "clear"

	AuditActionCashExpectationRecorded = "obligation_cash_expectation_recorded"
)

func (a CashExpectationAction) Label() string {
	switch a {
	case CashExpectationActionSet:
		return "设置／改期"
	case CashExpectationActionClear:
		return "清除排期"
	default:
		return string(a)
	}
}

type CashExpectation struct {
	ID                 string
	ObligationID       string
	Sequence           int64
	Action             CashExpectationAction
	ExpectedOn         string
	ObligationRevision int64
	SourceKind         string
	Reason             string
	RecordedBy         audit.ActorRef
	RecordedAt         time.Time
}

type RecordCashExpectationInput struct {
	CommandID               string
	ObligationID            string
	ExpectedCurrentRevision int64
	Action                  string
	ExpectedOn              string
	Reason                  string
	Actor                   audit.ActorRef
}

type CashExpectationCommand struct {
	CommandID   string
	PayloadHash []byte
	Actor       audit.ActorRef
	RecordedAt  time.Time
}

type RecordCashExpectationResult struct {
	Expectation CashExpectation
	Replayed    bool
}

type CashExpectationRow struct {
	Obligation  Obligation
	Expectation *CashExpectation
}

type CashForecastSummary struct {
	ExpectedOn           string
	LedgerOwnerPartyID   string
	LedgerOwnerLegalName string
	Direction            Direction
	Currency             string
	ObligationCount      int
	OutstandingAmount    string
}

type CashForecastReport struct {
	GeneratedAt          time.Time
	ScheduledSummaries   []CashForecastSummary
	UnscheduledSummaries []CashForecastSummary
	ScheduledItems       []CashExpectationRow
	UnscheduledItems     []CashExpectationRow
}

type CashExpectationHistory struct {
	Obligation Obligation
	Facts      []CashExpectation
}

type CashExpectationIdempotencyConflictError struct{ CommandID string }

func (e *CashExpectationIdempotencyConflictError) Error() string {
	return "该 Command ID 已用于不同的预计现金日期意图"
}

type CashExpectationStateConflictError struct{ Status string }

func (e *CashExpectationStateConflictError) Error() string {
	return "只有 open 状态的义务可以记录预计现金日期"
}

type CashExpectationBalanceConflictError struct{ Outstanding string }

func (e *CashExpectationBalanceConflictError) Error() string {
	return "只有当前剩余金额大于 0 的义务可以设置预计现金日期"
}

type CashExpectationClearConflictError struct{}

func (e *CashExpectationClearConflictError) Error() string {
	return "该义务当前没有可清除的预计现金日期"
}

type cashExpectationRepository interface {
	RecordCashExpectation(context.Context, CashExpectation, CashExpectationCommand, audit.Entry) (CashExpectation, bool, error)
	ListCurrentCashExpectationRows(context.Context) ([]CashExpectationRow, error)
	ListCashExpectationHistory(context.Context, string) (Obligation, []CashExpectation, bool, error)
}

func (s *Service) RecordCashExpectation(ctx context.Context, input RecordCashExpectationInput) (RecordCashExpectationResult, error) {
	normalized, err := normalizeCashExpectationInput(input)
	if err != nil {
		return RecordCashExpectationResult{}, err
	}
	if s.cashExpectations == nil {
		return RecordCashExpectationResult{}, fmt.Errorf("cash expectation repository is unavailable")
	}
	payloadHash, err := cashExpectationPayloadHash(normalized)
	if err != nil {
		return RecordCashExpectationResult{}, fmt.Errorf("hash cash expectation intent: %w", err)
	}
	expectationID, err := s.newID()
	if err != nil {
		return RecordCashExpectationResult{}, fmt.Errorf("generate cash expectation ID: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return RecordCashExpectationResult{}, fmt.Errorf("generate cash expectation audit ID: %w", err)
	}
	now := s.now().UTC()
	value := CashExpectation{
		ID:                 expectationID,
		ObligationID:       normalized.ObligationID,
		Action:             CashExpectationAction(normalized.Action),
		ExpectedOn:         normalized.ExpectedOn,
		ObligationRevision: normalized.ExpectedCurrentRevision,
		SourceKind:         SourceKindManual,
		Reason:             normalized.Reason,
		RecordedBy:         normalized.Actor,
		RecordedAt:         now,
	}
	persisted, replayed, err := s.cashExpectations.RecordCashExpectation(ctx, value, CashExpectationCommand{
		CommandID: normalized.CommandID, PayloadHash: payloadHash, Actor: normalized.Actor, RecordedAt: now,
	}, audit.Entry{
		ID: auditID, ObjectType: audit.ObjectObligation, ObjectID: normalized.ObligationID,
		Action: AuditActionCashExpectationRecorded, CommandID: normalized.CommandID,
		Actor: normalized.Actor, OccurredAt: now, Summary: "记录义务预计现金日期",
		Details: map[string]any{
			"action":                    normalized.Action,
			"expected_on":               normalized.ExpectedOn,
			"expected_current_revision": normalized.ExpectedCurrentRevision,
			"source_kind":               SourceKindManual,
			"expectation_reason":        normalized.Reason,
		},
	})
	if err != nil {
		return RecordCashExpectationResult{}, fmt.Errorf("record cash expectation: %w", err)
	}
	return RecordCashExpectationResult{Expectation: persisted, Replayed: replayed}, nil
}

func (s *Service) CurrentCashForecast(ctx context.Context) (CashForecastReport, error) {
	if s.cashExpectations == nil {
		return CashForecastReport{}, fmt.Errorf("cash expectation repository is unavailable")
	}
	rows, err := s.cashExpectations.ListCurrentCashExpectationRows(ctx)
	if err != nil {
		return CashForecastReport{}, fmt.Errorf("list current cash expectation rows: %w", err)
	}
	report := CashForecastReport{GeneratedAt: s.now().UTC()}
	scheduled := make(map[cashForecastSummaryKey]*cashForecastSummaryAccumulator)
	unscheduled := make(map[cashForecastSummaryKey]*cashForecastSummaryAccumulator)
	for _, row := range rows {
		value := row.Obligation
		if value.Status != StatusOpen {
			continue
		}
		amount, amountErr := decimalToMicros(value.OutstandingAmount)
		if amountErr != nil {
			return CashForecastReport{}, fmt.Errorf("obligation %s has invalid stored outstanding amount: %w", value.ID, amountErr)
		}
		if amount.Sign() <= 0 {
			continue
		}
		owner := value.LedgerOwner()
		if row.Expectation != nil && row.Expectation.Action == CashExpectationActionSet {
			report.ScheduledItems = append(report.ScheduledItems, row)
			key := cashForecastSummaryKey{
				ExpectedOn: row.Expectation.ExpectedOn, LedgerOwnerPartyID: value.LedgerOwnerPartyID,
				LedgerOwnerLegalName: owner.LegalName, Direction: value.Direction, Currency: value.Currency,
			}
			addCashForecastSummary(scheduled, key, amount)
			continue
		}
		report.UnscheduledItems = append(report.UnscheduledItems, row)
		key := cashForecastSummaryKey{
			LedgerOwnerPartyID: value.LedgerOwnerPartyID, LedgerOwnerLegalName: owner.LegalName,
			Direction: value.Direction, Currency: value.Currency,
		}
		addCashForecastSummary(unscheduled, key, amount)
	}
	sort.Slice(report.ScheduledItems, func(i, j int) bool {
		left, right := report.ScheduledItems[i], report.ScheduledItems[j]
		if left.Expectation.ExpectedOn != right.Expectation.ExpectedOn {
			return left.Expectation.ExpectedOn < right.Expectation.ExpectedOn
		}
		return cashForecastRowLess(left, right)
	})
	sort.Slice(report.UnscheduledItems, func(i, j int) bool {
		return cashForecastRowLess(report.UnscheduledItems[i], report.UnscheduledItems[j])
	})
	report.ScheduledSummaries = cashForecastSummaries(scheduled, true)
	report.UnscheduledSummaries = cashForecastSummaries(unscheduled, false)
	return report, nil
}

func (s *Service) CashExpectationHistory(ctx context.Context, obligationID string) (CashExpectationHistory, error) {
	id := strings.ToLower(strings.TrimSpace(obligationID))
	if !uuidPattern.MatchString(id) {
		return CashExpectationHistory{}, &ValidationError{Field: "obligation_id", Message: "义务标识无效"}
	}
	if s.cashExpectations == nil {
		return CashExpectationHistory{}, fmt.Errorf("cash expectation repository is unavailable")
	}
	value, facts, found, err := s.cashExpectations.ListCashExpectationHistory(ctx, id)
	if err != nil {
		return CashExpectationHistory{}, fmt.Errorf("list cash expectation history: %w", err)
	}
	if !found {
		return CashExpectationHistory{}, &ObligationNotFoundError{ObligationID: id}
	}
	return CashExpectationHistory{Obligation: value, Facts: facts}, nil
}

func normalizeCashExpectationInput(input RecordCashExpectationInput) (RecordCashExpectationInput, error) {
	commandID := strings.ToLower(strings.TrimSpace(input.CommandID))
	if !uuidPattern.MatchString(commandID) {
		return RecordCashExpectationInput{}, &ValidationError{Field: "command_id", Message: "Command ID 无效，请刷新页面后重试"}
	}
	obligationID := strings.ToLower(strings.TrimSpace(input.ObligationID))
	if !uuidPattern.MatchString(obligationID) {
		return RecordCashExpectationInput{}, &ValidationError{Field: "obligation_id", Message: "义务标识无效"}
	}
	if input.ExpectedCurrentRevision < 1 {
		return RecordCashExpectationInput{}, &ValidationError{Field: "expected_current_revision", Message: "Expected Current Revision 无效，请刷新页面后重试"}
	}
	action := CashExpectationAction(strings.ToLower(strings.TrimSpace(input.Action)))
	if action != CashExpectationActionSet && action != CashExpectationActionClear {
		return RecordCashExpectationInput{}, &ValidationError{Field: "action", Message: "请选择有效的预计现金日期操作"}
	}
	expectedOn := strings.TrimSpace(input.ExpectedOn)
	if action == CashExpectationActionSet {
		parsed, err := time.Parse("2006-01-02", expectedOn)
		if err != nil || parsed.Year() < 1 || parsed.Format("2006-01-02") != expectedOn {
			return RecordCashExpectationInput{}, &ValidationError{Field: "expected_on", Message: "预计现金日期必须是有效的 YYYY-MM-DD 日期"}
		}
	} else if expectedOn != "" {
		return RecordCashExpectationInput{}, &ValidationError{Field: "expected_on", Message: "清除排期时不能提交预计现金日期"}
	}
	reason, err := requiredText("expectation_reason", "预计现金日期原因不能为空", input.Reason, 200)
	if err != nil {
		return RecordCashExpectationInput{}, err
	}
	actor, err := audit.NormalizeActor(input.Actor)
	if err != nil {
		return RecordCashExpectationInput{}, &ValidationError{Field: "actor", Message: err.Error()}
	}
	return RecordCashExpectationInput{
		CommandID: commandID, ObligationID: obligationID,
		ExpectedCurrentRevision: input.ExpectedCurrentRevision,
		Action:                  string(action), ExpectedOn: expectedOn, Reason: reason, Actor: actor,
	}, nil
}

func cashExpectationPayloadHash(input RecordCashExpectationInput) ([]byte, error) {
	payload, err := json.Marshal(struct {
		Format                  string `json:"format"`
		ObligationID            string `json:"obligation_id"`
		ExpectedCurrentRevision int64  `json:"expected_current_revision"`
		Action                  string `json:"action"`
		ExpectedOn              string `json:"expected_on"`
		SourceKind              string `json:"source_kind"`
		Reason                  string `json:"reason"`
		ActorKind               string `json:"actor_kind"`
		ActorID                 string `json:"actor_id"`
	}{
		Format: "money-flow-obligation-cash-expectation-v1", ObligationID: input.ObligationID,
		ExpectedCurrentRevision: input.ExpectedCurrentRevision, Action: input.Action,
		ExpectedOn: input.ExpectedOn, SourceKind: SourceKindManual, Reason: input.Reason,
		ActorKind: input.Actor.Kind, ActorID: input.Actor.ID,
	})
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(payload)
	return hash[:], nil
}

type cashForecastSummaryKey struct {
	ExpectedOn           string
	LedgerOwnerPartyID   string
	LedgerOwnerLegalName string
	Direction            Direction
	Currency             string
}

type cashForecastSummaryAccumulator struct {
	count  int
	amount *big.Int
}

func addCashForecastSummary(values map[cashForecastSummaryKey]*cashForecastSummaryAccumulator, key cashForecastSummaryKey, amount *big.Int) {
	value := values[key]
	if value == nil {
		value = &cashForecastSummaryAccumulator{amount: new(big.Int)}
		values[key] = value
	}
	value.count++
	value.amount.Add(value.amount, amount)
}

func cashForecastSummaries(values map[cashForecastSummaryKey]*cashForecastSummaryAccumulator, includeDate bool) []CashForecastSummary {
	keys := make([]cashForecastSummaryKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		left, right := keys[i], keys[j]
		if includeDate && left.ExpectedOn != right.ExpectedOn {
			return left.ExpectedOn < right.ExpectedOn
		}
		if left.LedgerOwnerLegalName != right.LedgerOwnerLegalName {
			return left.LedgerOwnerLegalName < right.LedgerOwnerLegalName
		}
		if left.LedgerOwnerPartyID != right.LedgerOwnerPartyID {
			return left.LedgerOwnerPartyID < right.LedgerOwnerPartyID
		}
		if left.Direction != right.Direction {
			return directionRank(left.Direction) < directionRank(right.Direction)
		}
		return left.Currency < right.Currency
	})
	result := make([]CashForecastSummary, 0, len(keys))
	for _, key := range keys {
		value := values[key]
		result = append(result, CashForecastSummary{
			ExpectedOn: key.ExpectedOn, LedgerOwnerPartyID: key.LedgerOwnerPartyID,
			LedgerOwnerLegalName: key.LedgerOwnerLegalName, Direction: key.Direction,
			Currency: key.Currency, ObligationCount: value.count,
			OutstandingAmount: microsToDecimal(value.amount),
		})
	}
	return result
}

func cashForecastRowLess(left, right CashExpectationRow) bool {
	leftOwner, rightOwner := left.Obligation.LedgerOwner(), right.Obligation.LedgerOwner()
	if leftOwner.LegalName != rightOwner.LegalName {
		return leftOwner.LegalName < rightOwner.LegalName
	}
	if left.Obligation.LedgerOwnerPartyID != right.Obligation.LedgerOwnerPartyID {
		return left.Obligation.LedgerOwnerPartyID < right.Obligation.LedgerOwnerPartyID
	}
	if left.Obligation.Direction != right.Obligation.Direction {
		return directionRank(left.Obligation.Direction) < directionRank(right.Obligation.Direction)
	}
	if left.Obligation.Currency != right.Obligation.Currency {
		return left.Obligation.Currency < right.Obligation.Currency
	}
	return left.Obligation.ID < right.Obligation.ID
}
