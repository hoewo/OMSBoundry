package operatingsummary

import (
	"context"
	"errors"
	"testing"
	"time"
)

type repositoryStub struct {
	groups          []Group
	groupsErr       error
	obligationFacts []ObligationFact
	obligationErr   error
	cashFacts       []CashFact
	cashErr         error
	ownerID         string
	currency        string
	direction       string
	transactionType string
	amountKind      string
}

func (s *repositoryStub) ListGroups(context.Context) ([]Group, error) {
	return s.groups, s.groupsErr
}

func (s *repositoryStub) ListObligationFacts(_ context.Context, ownerID, currency, direction string) ([]ObligationFact, error) {
	s.ownerID, s.currency, s.direction = ownerID, currency, direction
	return s.obligationFacts, s.obligationErr
}

func (s *repositoryStub) ListCashFacts(_ context.Context, ownerID, currency, transactionType, amountKind string) ([]CashFact, error) {
	s.ownerID, s.currency, s.transactionType, s.amountKind = ownerID, currency, transactionType, amountKind
	return s.cashFacts, s.cashErr
}

func TestCurrentNormalizesMetricsAndSortsByOwnerIdentityAndCurrency(t *testing.T) {
	repository := &repositoryStub{groups: []Group{
		testGroup("20000000-0000-4000-8000-000000000001", "同名主体", "USD", "0", 0, "2.500000", 1, "0", 0, "3", 1, "0", 0, "1.25", 1),
		testGroup("10000000-0000-4000-8000-000000000001", "同名主体", "CNY", "000.100000", 1, "0.2", 1, "10", 1, "20", 1, "9.9", 1, "19.8", 1),
	}}
	service := NewService(repository)
	now := time.Date(2026, 9, 20, 8, 9, 10, 0, time.FixedZone("CST", 8*60*60))
	service.now = func() time.Time { return now }

	report, err := service.Current(context.Background())
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if !report.GeneratedAt.Equal(now.UTC()) || len(report.Groups) != 2 {
		t.Fatalf("report = %#v", report)
	}
	first := report.Groups[0]
	if first.LedgerOwnerPartyID != "10000000-0000-4000-8000-000000000001" || first.Currency != "CNY" ||
		first.ReceivableOutstanding.Amount != "0.1" || first.ReceivableOutstanding.Metric != MetricReceivableOutstanding ||
		first.PaymentUnallocated.Amount != "19.8" || first.PaymentUnallocated.Metric != MetricPaymentUnallocated {
		t.Fatalf("first group = %#v", first)
	}
	if report.Groups[1].LedgerOwnerPartyID != "20000000-0000-4000-8000-000000000001" ||
		report.Groups[1].PayableOutstanding.Amount != "2.5" {
		t.Fatalf("second group = %#v", report.Groups[1])
	}
}

func TestCurrentRejectsRepositoryAndProjectionInvariantFailures(t *testing.T) {
	repositoryError := errors.New("database unavailable")
	service := NewService(&repositoryStub{groupsErr: repositoryError})
	if _, err := service.Current(context.Background()); !errors.Is(err, repositoryError) {
		t.Fatalf("repository error = %v", err)
	}

	tests := []Group{
		testGroup("invalid", "主体", "CNY", "1", 1, "0", 0, "0", 0, "0", 0, "0", 0, "0", 0),
		testGroup("10000000-0000-4000-8000-000000000001", "主体", "CNY", "1", 0, "0", 0, "0", 0, "0", 0, "0", 0, "0", 0),
		testGroup("10000000-0000-4000-8000-000000000001", "主体", "CNY", "0", 0, "0", 0, "0", 0, "0", 0, "0", 0, "0", 0),
		testGroup("10000000-0000-4000-8000-000000000001", "主体", "CNY", "0.0000001", 1, "0", 0, "0", 0, "0", 0, "0", 0, "0", 0),
	}
	for index, group := range tests {
		_, err := NewService(&repositoryStub{groups: []Group{group}}).Current(context.Background())
		var invariant *ProjectionInvariantError
		if !errors.As(err, &invariant) {
			t.Fatalf("case %d error = %v", index, err)
		}
	}

	duplicate := testGroup("10000000-0000-4000-8000-000000000001", "主体", "CNY", "1", 1, "0", 0, "0", 0, "0", 0, "0", 0, "0", 0)
	_, err := NewService(&repositoryStub{groups: []Group{duplicate, duplicate}}).Current(context.Background())
	var invariant *ProjectionInvariantError
	if !errors.As(err, &invariant) {
		t.Fatalf("duplicate error = %v", err)
	}
}

func TestDrilldownSumsObligationFactsExactlyAndNormalizesInput(t *testing.T) {
	ownerID := "10000000-0000-4000-8000-000000000001"
	repository := &repositoryStub{obligationFacts: []ObligationFact{
		testObligationFact("20000000-0000-4000-8000-000000000001", ownerID, "CNY", "receivable", "1", "0.9", "0", "0.1"),
		testObligationFact("20000000-0000-4000-8000-000000000002", ownerID, "CNY", "receivable", "1", "0.8", "0", "0.2"),
	}}
	service := NewService(repository)
	result, err := service.Drilldown(context.Background(), DrilldownInput{
		Metric: " receivable_outstanding ", LedgerOwnerPartyID: " 10000000-0000-4000-8000-000000000001 ", Currency: " cny ",
	})
	if err != nil {
		t.Fatalf("Drilldown() error = %v", err)
	}
	if repository.ownerID != ownerID || repository.currency != "CNY" || repository.direction != "receivable" ||
		result.TotalAmount != "0.3" || result.FactCount != 2 || !result.IsObligation() ||
		result.LedgerOwnerLegalName != "本方主体" || result.ObligationFacts[0].DirectionLabel != "应收" {
		t.Fatalf("drilldown = %#v, repository = %#v", result, repository)
	}
}

func TestDrilldownSumsCashFactsAndSelectsContributionKind(t *testing.T) {
	ownerID := "10000000-0000-4000-8000-000000000001"
	repository := &repositoryStub{cashFacts: []CashFact{
		testCashFact("30000000-0000-4000-8000-000000000001", ownerID, "CNY", "payment", "1", "0.1", "0.9", "0.9"),
		testCashFact("30000000-0000-4000-8000-000000000002", ownerID, "CNY", "payment", "2", "0.2", "1.8", "1.8"),
	}}
	result, err := NewService(repository).Drilldown(context.Background(), DrilldownInput{
		Metric: string(MetricPaymentUnallocated), LedgerOwnerPartyID: ownerID, Currency: "CNY",
	})
	if err != nil {
		var invariant *ProjectionInvariantError
		if errors.As(err, &invariant) {
			t.Fatalf("Drilldown() invariant error = %s", invariant.Reason)
		}
		t.Fatalf("Drilldown() error = %v", err)
	}
	if repository.transactionType != "payment" || repository.amountKind != "unallocated" ||
		result.TotalAmount != "2.7" || result.FactCount != 2 || result.IsObligation() ||
		result.CashFacts[0].TypeLabel != "付款" {
		t.Fatalf("drilldown = %#v, repository = %#v", result, repository)
	}
}

func TestDrilldownValidatesInputNotFoundAndStoredFacts(t *testing.T) {
	ownerID := "10000000-0000-4000-8000-000000000001"
	valid := DrilldownInput{Metric: string(MetricReceiptEffective), LedgerOwnerPartyID: ownerID, Currency: "CNY"}
	for _, input := range []DrilldownInput{
		{Metric: "unknown", LedgerOwnerPartyID: ownerID, Currency: "CNY"},
		{Metric: string(MetricReceiptEffective), LedgerOwnerPartyID: "invalid", Currency: "CNY"},
		{Metric: string(MetricReceiptEffective), LedgerOwnerPartyID: ownerID, Currency: "CN"},
	} {
		_, err := NewService(&repositoryStub{}).Drilldown(context.Background(), input)
		var validation *ValidationError
		if !errors.As(err, &validation) {
			t.Fatalf("input %#v error = %v", input, err)
		}
	}

	_, err := NewService(&repositoryStub{}).Drilldown(context.Background(), valid)
	var notFound *DrilldownNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("not found error = %v", err)
	}

	repositoryError := errors.New("database unavailable")
	_, err = NewService(&repositoryStub{cashErr: repositoryError}).Drilldown(context.Background(), valid)
	if !errors.Is(err, repositoryError) {
		t.Fatalf("repository error = %v", err)
	}

	invalidFact := testCashFact("30000000-0000-4000-8000-000000000001", ownerID, "CNY", "receipt", "1", "0.9", "0.1", "0.5")
	_, err = NewService(&repositoryStub{cashFacts: []CashFact{invalidFact}}).Drilldown(context.Background(), valid)
	var invariant *ProjectionInvariantError
	if !errors.As(err, &invariant) {
		t.Fatalf("invalid fact error = %v", err)
	}
}

func testGroup(
	ownerID, ownerName, currency string,
	receivable string, receivableCount int64,
	payable string, payableCount int64,
	receipt string, receiptCount int64,
	payment string, paymentCount int64,
	receiptUnallocated string, receiptUnallocatedCount int64,
	paymentUnallocated string, paymentUnallocatedCount int64,
) Group {
	return Group{
		LedgerOwnerPartyID: ownerID, LedgerOwnerLegalName: ownerName, Currency: currency,
		ReceivableOutstanding: AmountMetric{Amount: receivable, FactCount: receivableCount},
		PayableOutstanding:    AmountMetric{Amount: payable, FactCount: payableCount},
		ReceiptEffective:      AmountMetric{Amount: receipt, FactCount: receiptCount},
		PaymentEffective:      AmountMetric{Amount: payment, FactCount: paymentCount},
		ReceiptUnallocated:    AmountMetric{Amount: receiptUnallocated, FactCount: receiptUnallocatedCount},
		PaymentUnallocated:    AmountMetric{Amount: paymentUnallocated, FactCount: paymentUnallocatedCount},
	}
}

func testObligationFact(id, ownerID, currency, direction, effective, allocated, remitted, outstanding string) ObligationFact {
	return ObligationFact{
		ID: id, Title: "测试义务", Direction: direction,
		LedgerOwnerPartyID: ownerID, LedgerOwnerLegalName: "本方主体", CounterpartyLegalName: "往来方",
		Currency: currency, DueOn: "2026-12-31", CurrentRevision: 1,
		EffectiveAmount: effective, NetAllocatedAmount: allocated, RemittedAmount: remitted,
		OutstandingAmount: outstanding, ContributionAmount: outstanding,
	}
}

func testCashFact(id, ownerID, currency, transactionType, effective, allocated, unallocated, contribution string) CashFact {
	return CashFact{
		ID: id, Description: "测试资金", Type: transactionType,
		LedgerOwnerPartyID: ownerID, LedgerOwnerLegalName: "本方主体", CounterpartyLegalName: "往来方",
		Currency: currency, OccurredOn: "2026-09-20", AccountReference: "基本户",
		OriginalAmount: effective, EffectiveAmount: effective, NetAllocatedAmount: allocated,
		UnallocatedAmount: unallocated, ContributionAmount: contribution,
	}
}
