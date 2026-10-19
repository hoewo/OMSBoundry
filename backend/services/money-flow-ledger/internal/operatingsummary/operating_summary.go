package operatingsummary

import (
	"context"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Metric string

const (
	MetricReceivableOutstanding Metric = "receivable_outstanding"
	MetricPayableOutstanding    Metric = "payable_outstanding"
	MetricReceiptEffective      Metric = "receipt_effective"
	MetricPaymentEffective      Metric = "payment_effective"
	MetricReceiptUnallocated    Metric = "receipt_unallocated"
	MetricPaymentUnallocated    Metric = "payment_unallocated"
)

var (
	uuidPattern     = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
	amountPattern   = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?$`)
)

func (m Metric) Label() string {
	switch m {
	case MetricReceivableOutstanding:
		return "当前应收余额"
	case MetricPayableOutstanding:
		return "当前应付余额"
	case MetricReceiptEffective:
		return "当前有效实际收款"
	case MetricPaymentEffective:
		return "当前有效实际付款"
	case MetricReceiptUnallocated:
		return "未分配收款"
	case MetricPaymentUnallocated:
		return "未分配付款"
	default:
		return string(m)
	}
}

func (m Metric) isObligation() bool {
	return m == MetricReceivableOutstanding || m == MetricPayableOutstanding
}

func validMetric(value Metric) bool {
	switch value {
	case MetricReceivableOutstanding, MetricPayableOutstanding,
		MetricReceiptEffective, MetricPaymentEffective,
		MetricReceiptUnallocated, MetricPaymentUnallocated:
		return true
	default:
		return false
	}
}

type AmountMetric struct {
	Metric    Metric
	Amount    string
	FactCount int64
}

type Group struct {
	LedgerOwnerPartyID    string
	LedgerOwnerLegalName  string
	Currency              string
	ReceivableOutstanding AmountMetric
	PayableOutstanding    AmountMetric
	ReceiptEffective      AmountMetric
	PaymentEffective      AmountMetric
	ReceiptUnallocated    AmountMetric
	PaymentUnallocated    AmountMetric
}

type Report struct {
	GeneratedAt time.Time
	Groups      []Group
}

type DrilldownInput struct {
	Metric             string
	LedgerOwnerPartyID string
	Currency           string
}

type ObligationFact struct {
	ID                    string
	Title                 string
	Direction             string
	DirectionLabel        string
	LedgerOwnerPartyID    string
	LedgerOwnerLegalName  string
	CounterpartyLegalName string
	Currency              string
	DueOn                 string
	CurrentRevision       int64
	EffectiveAmount       string
	NetAllocatedAmount    string
	RemittedAmount        string
	OutstandingAmount     string
	ContributionAmount    string
}

type CashFact struct {
	ID                    string
	Description           string
	Type                  string
	TypeLabel             string
	LedgerOwnerPartyID    string
	LedgerOwnerLegalName  string
	CounterpartyLegalName string
	Currency              string
	OccurredOn            string
	AccountReference      string
	OriginalAmount        string
	EffectiveAmount       string
	NetAllocatedAmount    string
	UnallocatedAmount     string
	ContributionAmount    string
}

type Drilldown struct {
	GeneratedAt          time.Time
	Metric               Metric
	LedgerOwnerPartyID   string
	LedgerOwnerLegalName string
	Currency             string
	TotalAmount          string
	FactCount            int
	ObligationFacts      []ObligationFact
	CashFacts            []CashFact
}

func (d Drilldown) IsObligation() bool { return d.Metric.isObligation() }

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

type DrilldownNotFoundError struct {
	Metric             Metric
	LedgerOwnerPartyID string
	Currency           string
}

func (e *DrilldownNotFoundError) Error() string {
	return "当前没有构成该指标的正金额事实"
}

type ProjectionInvariantError struct {
	Reason string
}

func (e *ProjectionInvariantError) Error() string {
	return "当前经营汇总事实无法形成一致投影"
}

type Repository interface {
	ListGroups(context.Context) ([]Group, error)
	ListObligationFacts(context.Context, string, string, string) ([]ObligationFact, error)
	ListCashFacts(context.Context, string, string, string, string) ([]CashFact, error)
}

type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) Current(ctx context.Context) (Report, error) {
	groups, err := s.repository.ListGroups(ctx)
	if err != nil {
		return Report{}, fmt.Errorf("list current operating summary groups: %w", err)
	}
	seen := make(map[string]struct{}, len(groups))
	for index := range groups {
		if err := normalizeAndValidateGroup(&groups[index]); err != nil {
			return Report{}, err
		}
		key := groups[index].LedgerOwnerPartyID + "\x00" + groups[index].Currency
		if _, exists := seen[key]; exists {
			return Report{}, &ProjectionInvariantError{Reason: "duplicate ledger owner and currency group"}
		}
		seen[key] = struct{}{}
	}
	sort.Slice(groups, func(i, j int) bool {
		leftName := strings.ToLower(groups[i].LedgerOwnerLegalName)
		rightName := strings.ToLower(groups[j].LedgerOwnerLegalName)
		if leftName != rightName {
			return leftName < rightName
		}
		if groups[i].LedgerOwnerPartyID != groups[j].LedgerOwnerPartyID {
			return groups[i].LedgerOwnerPartyID < groups[j].LedgerOwnerPartyID
		}
		return groups[i].Currency < groups[j].Currency
	})
	return Report{GeneratedAt: s.now().UTC(), Groups: groups}, nil
}

func (s *Service) Drilldown(ctx context.Context, input DrilldownInput) (Drilldown, error) {
	normalized, err := normalizeDrilldownInput(input)
	if err != nil {
		return Drilldown{}, err
	}
	metric := Metric(normalized.Metric)
	result := Drilldown{
		GeneratedAt:        s.now().UTC(),
		Metric:             metric,
		LedgerOwnerPartyID: normalized.LedgerOwnerPartyID,
		Currency:           normalized.Currency,
	}
	total := new(big.Int)
	if metric.isObligation() {
		direction := "receivable"
		if metric == MetricPayableOutstanding {
			direction = "payable"
		}
		facts, readErr := s.repository.ListObligationFacts(ctx, normalized.LedgerOwnerPartyID, normalized.Currency, direction)
		if readErr != nil {
			return Drilldown{}, fmt.Errorf("list current obligation summary facts: %w", readErr)
		}
		for index := range facts {
			if normalizeErr := normalizeAndValidateObligationFact(&facts[index], normalized, direction); normalizeErr != nil {
				return Drilldown{}, normalizeErr
			}
			amount, parseErr := decimalToMicros(facts[index].ContributionAmount)
			if parseErr != nil {
				return Drilldown{}, &ProjectionInvariantError{Reason: parseErr.Error()}
			}
			total.Add(total, amount)
		}
		if len(facts) == 0 {
			return Drilldown{}, &DrilldownNotFoundError{Metric: metric, LedgerOwnerPartyID: normalized.LedgerOwnerPartyID, Currency: normalized.Currency}
		}
		result.LedgerOwnerLegalName = facts[0].LedgerOwnerLegalName
		result.ObligationFacts = facts
		result.FactCount = len(facts)
	} else {
		transactionType := "receipt"
		amountKind := "effective"
		if metric == MetricPaymentEffective || metric == MetricPaymentUnallocated {
			transactionType = "payment"
		}
		if metric == MetricReceiptUnallocated || metric == MetricPaymentUnallocated {
			amountKind = "unallocated"
		}
		facts, readErr := s.repository.ListCashFacts(ctx, normalized.LedgerOwnerPartyID, normalized.Currency, transactionType, amountKind)
		if readErr != nil {
			return Drilldown{}, fmt.Errorf("list current cash summary facts: %w", readErr)
		}
		for index := range facts {
			if normalizeErr := normalizeAndValidateCashFact(&facts[index], normalized, transactionType, amountKind); normalizeErr != nil {
				return Drilldown{}, normalizeErr
			}
			amount, parseErr := decimalToMicros(facts[index].ContributionAmount)
			if parseErr != nil {
				return Drilldown{}, &ProjectionInvariantError{Reason: parseErr.Error()}
			}
			total.Add(total, amount)
		}
		if len(facts) == 0 {
			return Drilldown{}, &DrilldownNotFoundError{Metric: metric, LedgerOwnerPartyID: normalized.LedgerOwnerPartyID, Currency: normalized.Currency}
		}
		result.LedgerOwnerLegalName = facts[0].LedgerOwnerLegalName
		result.CashFacts = facts
		result.FactCount = len(facts)
	}
	result.TotalAmount = microsToDecimal(total)
	return result, nil
}

func normalizeDrilldownInput(input DrilldownInput) (DrilldownInput, error) {
	metric := Metric(strings.TrimSpace(input.Metric))
	if !validMetric(metric) {
		return DrilldownInput{}, &ValidationError{Field: "metric", Message: "汇总指标无效"}
	}
	ownerID := strings.ToLower(strings.TrimSpace(input.LedgerOwnerPartyID))
	if !uuidPattern.MatchString(ownerID) {
		return DrilldownInput{}, &ValidationError{Field: "ledger_owner_party_id", Message: "本方账簿主体标识无效"}
	}
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if !currencyPattern.MatchString(currency) {
		return DrilldownInput{}, &ValidationError{Field: "currency", Message: "币种必须是三个英文字母"}
	}
	return DrilldownInput{Metric: string(metric), LedgerOwnerPartyID: ownerID, Currency: currency}, nil
}

func normalizeAndValidateGroup(group *Group) error {
	if !uuidPattern.MatchString(group.LedgerOwnerPartyID) || strings.TrimSpace(group.LedgerOwnerLegalName) == "" ||
		!currencyPattern.MatchString(group.Currency) {
		return &ProjectionInvariantError{Reason: "invalid group identity"}
	}
	metrics := []*AmountMetric{
		&group.ReceivableOutstanding, &group.PayableOutstanding,
		&group.ReceiptEffective, &group.PaymentEffective,
		&group.ReceiptUnallocated, &group.PaymentUnallocated,
	}
	wantMetrics := []Metric{
		MetricReceivableOutstanding, MetricPayableOutstanding,
		MetricReceiptEffective, MetricPaymentEffective,
		MetricReceiptUnallocated, MetricPaymentUnallocated,
	}
	positive := false
	for index, metric := range metrics {
		metric.Metric = wantMetrics[index]
		normalized, err := normalizeAmount(metric.Amount)
		if err != nil || metric.FactCount < 0 {
			return &ProjectionInvariantError{Reason: "invalid metric amount or count"}
		}
		metric.Amount = normalized
		amount, _ := decimalToMicros(normalized)
		if (amount.Sign() == 0) != (metric.FactCount == 0) {
			return &ProjectionInvariantError{Reason: "metric amount and count disagree"}
		}
		positive = positive || amount.Sign() > 0
	}
	receiptEffective, _ := decimalToMicros(group.ReceiptEffective.Amount)
	receiptUnallocated, _ := decimalToMicros(group.ReceiptUnallocated.Amount)
	paymentEffective, _ := decimalToMicros(group.PaymentEffective.Amount)
	paymentUnallocated, _ := decimalToMicros(group.PaymentUnallocated.Amount)
	if receiptUnallocated.Cmp(receiptEffective) > 0 || paymentUnallocated.Cmp(paymentEffective) > 0 ||
		group.ReceiptUnallocated.FactCount > group.ReceiptEffective.FactCount ||
		group.PaymentUnallocated.FactCount > group.PaymentEffective.FactCount {
		return &ProjectionInvariantError{Reason: "unallocated totals exceed effective cash"}
	}
	if !positive {
		return &ProjectionInvariantError{Reason: "empty group"}
	}
	return nil
}

func normalizeAndValidateObligationFact(fact *ObligationFact, input DrilldownInput, direction string) error {
	if !uuidPattern.MatchString(fact.ID) || fact.LedgerOwnerPartyID != input.LedgerOwnerPartyID ||
		fact.Currency != input.Currency || fact.Direction != direction || fact.CurrentRevision < 1 ||
		strings.TrimSpace(fact.Title) == "" || strings.TrimSpace(fact.LedgerOwnerLegalName) == "" ||
		strings.TrimSpace(fact.CounterpartyLegalName) == "" {
		return &ProjectionInvariantError{Reason: "invalid obligation drilldown identity"}
	}
	fact.DirectionLabel = "应收"
	if direction == "payable" {
		fact.DirectionLabel = "应付"
	}
	fields := []*string{&fact.EffectiveAmount, &fact.NetAllocatedAmount, &fact.RemittedAmount, &fact.OutstandingAmount, &fact.ContributionAmount}
	for _, field := range fields {
		normalized, err := normalizeAmount(*field)
		if err != nil {
			return &ProjectionInvariantError{Reason: "invalid obligation drilldown amount"}
		}
		*field = normalized
	}
	if fact.ContributionAmount != fact.OutstandingAmount || fact.ContributionAmount == "0" {
		return &ProjectionInvariantError{Reason: "obligation contribution mismatch"}
	}
	effective, _ := decimalToMicros(fact.EffectiveAmount)
	allocated, _ := decimalToMicros(fact.NetAllocatedAmount)
	remitted, _ := decimalToMicros(fact.RemittedAmount)
	outstanding, _ := decimalToMicros(fact.OutstandingAmount)
	want := new(big.Int).Sub(new(big.Int).Set(effective), allocated)
	want.Sub(want, remitted)
	if outstanding.Cmp(want) != 0 || outstanding.Sign() <= 0 {
		return &ProjectionInvariantError{Reason: "obligation balance mismatch"}
	}
	return nil
}

func normalizeAndValidateCashFact(fact *CashFact, input DrilldownInput, transactionType, amountKind string) error {
	if !uuidPattern.MatchString(fact.ID) || fact.LedgerOwnerPartyID != input.LedgerOwnerPartyID ||
		fact.Currency != input.Currency || fact.Type != transactionType || strings.TrimSpace(fact.Description) == "" ||
		strings.TrimSpace(fact.LedgerOwnerLegalName) == "" || strings.TrimSpace(fact.CounterpartyLegalName) == "" {
		return &ProjectionInvariantError{Reason: "invalid cash drilldown identity"}
	}
	fact.TypeLabel = "收款"
	if transactionType == "payment" {
		fact.TypeLabel = "付款"
	}
	fields := []*string{&fact.OriginalAmount, &fact.EffectiveAmount, &fact.NetAllocatedAmount, &fact.UnallocatedAmount, &fact.ContributionAmount}
	for _, field := range fields {
		normalized, err := normalizeAmount(*field)
		if err != nil {
			return &ProjectionInvariantError{Reason: "invalid cash drilldown amount"}
		}
		*field = normalized
	}
	effective, _ := decimalToMicros(fact.EffectiveAmount)
	allocated, _ := decimalToMicros(fact.NetAllocatedAmount)
	unallocated, _ := decimalToMicros(fact.UnallocatedAmount)
	wantUnallocated := new(big.Int).Sub(new(big.Int).Set(effective), allocated)
	if fact.OriginalAmount != fact.EffectiveAmount || unallocated.Cmp(wantUnallocated) != 0 || unallocated.Sign() < 0 {
		return &ProjectionInvariantError{Reason: "cash balance mismatch"}
	}
	wantContribution := fact.EffectiveAmount
	if amountKind == "unallocated" {
		wantContribution = fact.UnallocatedAmount
	}
	if fact.ContributionAmount != wantContribution || fact.ContributionAmount == "0" {
		return &ProjectionInvariantError{Reason: "cash contribution mismatch"}
	}
	return nil
}

func normalizeAmount(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if !amountPattern.MatchString(value) {
		return "", fmt.Errorf("invalid decimal amount %q", raw)
	}
	parts := strings.SplitN(value, ".", 2)
	integer := strings.TrimLeft(parts[0], "0")
	if integer == "" {
		integer = "0"
	}
	fraction := ""
	if len(parts) == 2 {
		if len(parts[1]) > 6 {
			return "", fmt.Errorf("amount has more than six decimal places")
		}
		fraction = strings.TrimRight(parts[1], "0")
	}
	if fraction == "" {
		return integer, nil
	}
	return integer + "." + fraction, nil
}

func decimalToMicros(value string) (*big.Int, error) {
	normalized, err := normalizeAmount(value)
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(normalized, ".", 2)
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	fraction += strings.Repeat("0", 6-len(fraction))
	result, ok := new(big.Int).SetString(parts[0]+fraction, 10)
	if !ok {
		return nil, fmt.Errorf("invalid decimal amount %q", value)
	}
	return result, nil
}

func microsToDecimal(value *big.Int) string {
	digits := value.String()
	if len(digits) <= 6 {
		digits = strings.Repeat("0", 7-len(digits)) + digits
	}
	integer := digits[:len(digits)-6]
	fraction := strings.TrimRight(digits[len(digits)-6:], "0")
	if fraction == "" {
		return integer
	}
	return integer + "." + fraction
}
