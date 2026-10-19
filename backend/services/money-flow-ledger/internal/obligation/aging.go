package obligation

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"
)

type AgingBucket string

const (
	AgingBucketNotDue     AgingBucket = "not_due"
	AgingBucketDays1To30  AgingBucket = "days_1_30"
	AgingBucketDays31To60 AgingBucket = "days_31_60"
	AgingBucketDays61To90 AgingBucket = "days_61_90"
	AgingBucketDays91Plus AgingBucket = "days_91_plus"
)

func (b AgingBucket) Label() string {
	switch b {
	case AgingBucketNotDue:
		return "未到期"
	case AgingBucketDays1To30:
		return "逾期 1–30 天"
	case AgingBucketDays31To60:
		return "逾期 31–60 天"
	case AgingBucketDays61To90:
		return "逾期 61–90 天"
	case AgingBucketDays91Plus:
		return "逾期 91 天及以上"
	default:
		return string(b)
	}
}

type CurrentAgingInput struct {
	AsOfDate string
}

type AgingItem struct {
	Obligation  Obligation
	Bucket      AgingBucket
	DaysPastDue int
}

type AgingSummary struct {
	Direction         Direction
	Currency          string
	ObligationCount   int
	OverdueCount      int
	OutstandingAmount string
	OverdueAmount     string
	NotDueAmount      string
	Days1To30Amount   string
	Days31To60Amount  string
	Days61To90Amount  string
	Days91PlusAmount  string
}

type AgingReport struct {
	AsOfDate    string
	GeneratedAt time.Time
	Summaries   []AgingSummary
	Items       []AgingItem
}

func (s *Service) CurrentAging(ctx context.Context, input CurrentAgingInput) (AgingReport, error) {
	now := s.now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	asOfDate, asOf, err := normalizeAgingDate(input.AsOfDate, today)
	if err != nil {
		return AgingReport{}, err
	}

	obligations, err := s.repository.List(ctx)
	if err != nil {
		return AgingReport{}, fmt.Errorf("list current obligations for aging: %w", err)
	}

	report := AgingReport{
		AsOfDate:    asOfDate,
		GeneratedAt: now,
		Items:       make([]AgingItem, 0),
	}
	accumulators := make(map[agingSummaryKey]*agingSummaryAccumulator)
	for _, value := range obligations {
		if value.Status != StatusOpen {
			continue
		}
		amount, amountErr := decimalToMicros(value.OutstandingAmount)
		if amountErr != nil {
			return AgingReport{}, fmt.Errorf("obligation %s has invalid stored outstanding amount: %w", value.ID, amountErr)
		}
		if amount.Sign() == 0 {
			continue
		}
		dueOn, parseErr := time.Parse("2006-01-02", value.DueOn)
		if parseErr != nil || dueOn.Format("2006-01-02") != value.DueOn {
			return AgingReport{}, fmt.Errorf("obligation %s has invalid stored due date %q", value.ID, value.DueOn)
		}
		daysPastDue := int(asOf.Sub(dueOn).Hours() / 24)
		bucket := agingBucketFor(daysPastDue)
		if daysPastDue < 0 {
			daysPastDue = 0
		}
		item := AgingItem{Obligation: value, Bucket: bucket, DaysPastDue: daysPastDue}
		report.Items = append(report.Items, item)

		key := agingSummaryKey{Direction: value.Direction, Currency: value.Currency}
		accumulator := accumulators[key]
		if accumulator == nil {
			accumulator = newAgingSummaryAccumulator()
			accumulators[key] = accumulator
		}
		accumulator.add(item, amount)
	}

	sort.Slice(report.Items, func(i, j int) bool {
		left, right := report.Items[i], report.Items[j]
		if left.Obligation.Direction != right.Obligation.Direction {
			return directionRank(left.Obligation.Direction) < directionRank(right.Obligation.Direction)
		}
		if left.Obligation.Currency != right.Obligation.Currency {
			return left.Obligation.Currency < right.Obligation.Currency
		}
		if left.Bucket != right.Bucket {
			return agingBucketRank(left.Bucket) < agingBucketRank(right.Bucket)
		}
		if left.Obligation.DueOn != right.Obligation.DueOn {
			return left.Obligation.DueOn < right.Obligation.DueOn
		}
		return left.Obligation.ID < right.Obligation.ID
	})

	keys := make([]agingSummaryKey, 0, len(accumulators))
	for key := range accumulators {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Direction != keys[j].Direction {
			return directionRank(keys[i].Direction) < directionRank(keys[j].Direction)
		}
		return keys[i].Currency < keys[j].Currency
	})
	report.Summaries = make([]AgingSummary, 0, len(keys))
	for _, key := range keys {
		report.Summaries = append(report.Summaries, accumulators[key].summary(key))
	}
	return report, nil
}

func normalizeAgingDate(raw string, today time.Time) (string, time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return today.Format("2006-01-02"), today, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil || parsed.Format("2006-01-02") != value {
		return "", time.Time{}, &ValidationError{Field: "as_of_date", Message: "账龄分类日必须是有效的 YYYY-MM-DD 日期"}
	}
	if parsed.Before(today) {
		return "", time.Time{}, &ValidationError{Field: "as_of_date", Message: "当前余额账龄不支持过去日期；历史时点余额需要独立重放能力"}
	}
	return value, parsed, nil
}

func agingBucketFor(daysPastDue int) AgingBucket {
	switch {
	case daysPastDue <= 0:
		return AgingBucketNotDue
	case daysPastDue <= 30:
		return AgingBucketDays1To30
	case daysPastDue <= 60:
		return AgingBucketDays31To60
	case daysPastDue <= 90:
		return AgingBucketDays61To90
	default:
		return AgingBucketDays91Plus
	}
}

func agingBucketRank(bucket AgingBucket) int {
	switch bucket {
	case AgingBucketNotDue:
		return 0
	case AgingBucketDays1To30:
		return 1
	case AgingBucketDays31To60:
		return 2
	case AgingBucketDays61To90:
		return 3
	case AgingBucketDays91Plus:
		return 4
	default:
		return 5
	}
}

func directionRank(direction Direction) int {
	if direction == DirectionReceivable {
		return 0
	}
	if direction == DirectionPayable {
		return 1
	}
	return 2
}

type agingSummaryKey struct {
	Direction Direction
	Currency  string
}

type agingSummaryAccumulator struct {
	obligationCount int
	overdueCount    int
	outstanding     *big.Int
	overdue         *big.Int
	buckets         map[AgingBucket]*big.Int
}

func newAgingSummaryAccumulator() *agingSummaryAccumulator {
	return &agingSummaryAccumulator{
		outstanding: new(big.Int),
		overdue:     new(big.Int),
		buckets: map[AgingBucket]*big.Int{
			AgingBucketNotDue:     new(big.Int),
			AgingBucketDays1To30:  new(big.Int),
			AgingBucketDays31To60: new(big.Int),
			AgingBucketDays61To90: new(big.Int),
			AgingBucketDays91Plus: new(big.Int),
		},
	}
}

func (a *agingSummaryAccumulator) add(item AgingItem, amount *big.Int) {
	a.obligationCount++
	a.outstanding.Add(a.outstanding, amount)
	a.buckets[item.Bucket].Add(a.buckets[item.Bucket], amount)
	if item.Bucket != AgingBucketNotDue {
		a.overdueCount++
		a.overdue.Add(a.overdue, amount)
	}
}

func (a *agingSummaryAccumulator) summary(key agingSummaryKey) AgingSummary {
	return AgingSummary{
		Direction:         key.Direction,
		Currency:          key.Currency,
		ObligationCount:   a.obligationCount,
		OverdueCount:      a.overdueCount,
		OutstandingAmount: microsToDecimal(a.outstanding),
		OverdueAmount:     microsToDecimal(a.overdue),
		NotDueAmount:      microsToDecimal(a.buckets[AgingBucketNotDue]),
		Days1To30Amount:   microsToDecimal(a.buckets[AgingBucketDays1To30]),
		Days31To60Amount:  microsToDecimal(a.buckets[AgingBucketDays31To60]),
		Days61To90Amount:  microsToDecimal(a.buckets[AgingBucketDays61To90]),
		Days91PlusAmount:  microsToDecimal(a.buckets[AgingBucketDays91Plus]),
	}
}

func decimalToMicros(value string) (*big.Int, error) {
	normalized, err := normalizeAmountAllowZero(value)
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
