//go:build integration

package operatingsummary

import (
	"context"
	"errors"
	"testing"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/allocation"
	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/cashtransaction"
	"omsbizfoundry/money-flow-ledger/internal/database"
	"omsbizfoundry/money-flow-ledger/internal/internaltransfer"
	"omsbizfoundry/money-flow-ledger/internal/obligation"
	"omsbizfoundry/money-flow-ledger/internal/party"
	"omsbizfoundry/money-flow-ledger/internal/testdatabase"
)

func TestPostgresCurrentOperatingSummarySeparatesGroupsAndDrillsIntoExactFacts(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "10000000-0000-4000-8000-000000000001", DisplayName: "经营汇总管理员",
	}
	partyService := party.NewService(party.NewPostgresRepository(environment.Pool))
	ownerA := createSummaryParty(t, ctx, partyService, actor, "11000000-0000-4000-8000-000000000001", "同名经营主体", "OWNER-A")
	ownerB := createSummaryParty(t, ctx, partyService, actor, "11000000-0000-4000-8000-000000000002", "同名经营主体", "OWNER-B")
	counterparty := createSummaryParty(t, ctx, partyService, actor, "11000000-0000-4000-8000-000000000003", "经营往来方", "COUNTERPARTY")

	obligationService := obligation.NewService(obligation.NewPostgresRepository(environment.Pool))
	cashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	allocationService := allocation.NewService(allocation.NewPostgresRepository(environment.Pool))

	receivableA1 := createSummaryObligation(t, ctx, obligationService, actor,
		"12000000-0000-4000-8000-000000000001", "12000000-0000-4000-8000-000000000002",
		"receivable", ownerA.ID, counterparty.ID, "应收一", "100.1", "CNY")
	receiptA1 := createSummaryCash(t, ctx, cashService, actor,
		"13000000-0000-4000-8000-000000000001", "receipt", ownerA.ID, counterparty.ID, "收款一", "100.1", "CNY")
	applyA1 := applySummaryAllocation(t, ctx, allocationService, actor,
		"14000000-0000-4000-8000-000000000001", receiptA1.ID, receivableA1.ID, "40")
	if _, err := obligationService.RecordRemission(ctx, obligation.RecordRemissionInput{
		CommandID: "15000000-0000-4000-8000-000000000001", ObligationID: receivableA1.ID,
		ExpectedCurrentRevision: 1, Amount: "10", Reason: "汇总测试减免",
		ManualSourceReference: "汇总测试减免依据", Actor: actor,
	}); err != nil {
		t.Fatalf("record summary remission: %v", err)
	}
	if applyA1.Allocation.ID == "" {
		t.Fatal("first allocation ID is empty")
	}

	receivableA2 := createSummaryObligation(t, ctx, obligationService, actor,
		"12000000-0000-4000-8000-000000000003", "12000000-0000-4000-8000-000000000004",
		"receivable", ownerA.ID, counterparty.ID, "应收二", "0.2", "CNY")
	receiptA2 := createSummaryCash(t, ctx, cashService, actor,
		"13000000-0000-4000-8000-000000000002", "receipt", ownerA.ID, counterparty.ID, "收款二", "0.2", "CNY")

	payableA := createSummaryObligation(t, ctx, obligationService, actor,
		"12000000-0000-4000-8000-000000000005", "12000000-0000-4000-8000-000000000006",
		"payable", ownerA.ID, counterparty.ID, "应付一", "70", "CNY")
	paymentA := createSummaryCash(t, ctx, cashService, actor,
		"13000000-0000-4000-8000-000000000003", "payment", ownerA.ID, counterparty.ID, "付款一", "70", "CNY")
	paymentAllocation := applySummaryAllocation(t, ctx, allocationService, actor,
		"14000000-0000-4000-8000-000000000002", paymentA.ID, payableA.ID, "20")
	if _, err := allocationService.Reverse(ctx, allocation.ReverseInput{
		CommandID: "16000000-0000-4000-8000-000000000001", AllocationID: paymentAllocation.Allocation.ID,
		Reason: "汇总测试反向核销", Actor: actor,
	}); err != nil {
		t.Fatalf("reverse summary allocation: %v", err)
	}

	receivableUSD := createSummaryObligation(t, ctx, obligationService, actor,
		"12000000-0000-4000-8000-000000000007", "12000000-0000-4000-8000-000000000008",
		"receivable", ownerA.ID, counterparty.ID, "美元应收", "5", "USD")
	receiptUSD := createSummaryCash(t, ctx, cashService, actor,
		"13000000-0000-4000-8000-000000000004", "receipt", ownerA.ID, counterparty.ID, "美元收款", "5", "USD")

	receivableB := createSummaryObligation(t, ctx, obligationService, actor,
		"12000000-0000-4000-8000-000000000009", "12000000-0000-4000-8000-00000000000a",
		"receivable", ownerB.ID, counterparty.ID, "另一账簿应收", "9", "CNY")
	receiptB := createSummaryCash(t, ctx, cashService, actor,
		"13000000-0000-4000-8000-000000000005", "receipt", ownerB.ID, counterparty.ID, "另一账簿收款", "9", "CNY")

	closedObligation := createSummaryObligation(t, ctx, obligationService, actor,
		"12000000-0000-4000-8000-00000000000b", "12000000-0000-4000-8000-00000000000c",
		"receivable", ownerA.ID, counterparty.ID, "已关闭应收", "10", "CNY")
	closedCash := createSummaryCash(t, ctx, cashService, actor,
		"13000000-0000-4000-8000-000000000006", "receipt", ownerA.ID, counterparty.ID, "结清收款", "10", "CNY")
	applySummaryAllocation(t, ctx, allocationService, actor,
		"14000000-0000-4000-8000-000000000003", closedCash.ID, closedObligation.ID, "10")
	if _, err := obligationService.Close(ctx, obligation.CloseInput{
		CommandID: "17000000-0000-4000-8000-000000000001", ObligationID: closedObligation.ID,
		ExpectedCurrentRevision: 1, ExpectedBalanceSequence: 1, Reason: "汇总测试零余额关闭", Actor: actor,
	}); err != nil {
		t.Fatalf("close summary obligation: %v", err)
	}

	reversedCash := createSummaryCash(t, ctx, cashService, actor,
		"13000000-0000-4000-8000-000000000007", "receipt", ownerA.ID, counterparty.ID, "错误收款", "8", "CNY")
	if _, err := cashService.Reverse(ctx, cashtransaction.ReverseInput{
		CommandID: "18000000-0000-4000-8000-000000000001", CashTransactionID: reversedCash.ID,
		Reason: "汇总测试错误资金冲销", Actor: actor,
	}); err != nil {
		t.Fatalf("reverse summary cash: %v", err)
	}

	transferService := internaltransfer.NewService(internaltransfer.NewPostgresRepository(environment.Pool))
	if _, err := transferService.Create(ctx, internaltransfer.CreateInput{
		CommandID: "19000000-0000-4000-8000-000000000001", LedgerOwnerPartyID: ownerA.ID,
		Description: "汇总必须排除的内部转账", Amount: "999", Currency: "CNY", OccurredOn: "2026-09-20",
		SourceAccountReference: "基本户", DestinationAccountReference: "备用户",
		ManualSourceReference: "内部转账依据", Actor: actor,
	}); err != nil {
		t.Fatalf("create excluded internal transfer: %v", err)
	}

	service := NewService(NewPostgresRepository(environment.Pool))
	report, err := service.Current(ctx)
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if len(report.Groups) != 3 {
		t.Fatalf("group count = %d, groups = %#v", len(report.Groups), report.Groups)
	}
	assertSummaryGroup(t, findSummaryGroup(t, report, ownerA.ID, "CNY"),
		"50.3", 2, "70", 1, "110.3", 3, "70", 1, "60.3", 2, "70", 1)
	assertSummaryGroup(t, findSummaryGroup(t, report, ownerA.ID, "USD"),
		"5", 1, "0", 0, "5", 1, "0", 0, "5", 1, "0", 0)
	assertSummaryGroup(t, findSummaryGroup(t, report, ownerB.ID, "CNY"),
		"9", 1, "0", 0, "9", 1, "0", 0, "9", 1, "0", 0)

	receivableDrilldown, err := service.Drilldown(ctx, DrilldownInput{
		Metric: string(MetricReceivableOutstanding), LedgerOwnerPartyID: ownerA.ID, Currency: "CNY",
	})
	if err != nil {
		t.Fatalf("receivable Drilldown() error = %v", err)
	}
	if receivableDrilldown.TotalAmount != "50.3" || receivableDrilldown.FactCount != 2 ||
		containsObligationFact(receivableDrilldown.ObligationFacts, closedObligation.ID) ||
		!containsObligationFact(receivableDrilldown.ObligationFacts, receivableA1.ID) ||
		!containsObligationFact(receivableDrilldown.ObligationFacts, receivableA2.ID) {
		t.Fatalf("receivable drilldown = %#v", receivableDrilldown)
	}

	effectiveReceiptDrilldown, err := service.Drilldown(ctx, DrilldownInput{
		Metric: string(MetricReceiptEffective), LedgerOwnerPartyID: ownerA.ID, Currency: "CNY",
	})
	if err != nil {
		t.Fatalf("effective receipt Drilldown() error = %v", err)
	}
	if effectiveReceiptDrilldown.TotalAmount != "110.3" || effectiveReceiptDrilldown.FactCount != 3 ||
		containsCashFact(effectiveReceiptDrilldown.CashFacts, reversedCash.ID) ||
		!containsCashFact(effectiveReceiptDrilldown.CashFacts, closedCash.ID) {
		t.Fatalf("effective receipt drilldown = %#v", effectiveReceiptDrilldown)
	}

	unallocatedReceiptDrilldown, err := service.Drilldown(ctx, DrilldownInput{
		Metric: string(MetricReceiptUnallocated), LedgerOwnerPartyID: ownerA.ID, Currency: "CNY",
	})
	if err != nil {
		t.Fatalf("unallocated receipt Drilldown() error = %v", err)
	}
	if unallocatedReceiptDrilldown.TotalAmount != "60.3" || unallocatedReceiptDrilldown.FactCount != 2 ||
		containsCashFact(unallocatedReceiptDrilldown.CashFacts, closedCash.ID) ||
		!containsCashFact(unallocatedReceiptDrilldown.CashFacts, receiptA1.ID) ||
		!containsCashFact(unallocatedReceiptDrilldown.CashFacts, receiptA2.ID) {
		t.Fatalf("unallocated receipt drilldown = %#v", unallocatedReceiptDrilldown)
	}

	for _, test := range []struct {
		metric       Metric
		wantAmount   string
		wantCount    int
		obligationID string
		cashID       string
	}{
		{metric: MetricPayableOutstanding, wantAmount: "70", wantCount: 1, obligationID: payableA.ID},
		{metric: MetricPaymentEffective, wantAmount: "70", wantCount: 1, cashID: paymentA.ID},
		{metric: MetricPaymentUnallocated, wantAmount: "70", wantCount: 1, cashID: paymentA.ID},
	} {
		details, drilldownErr := service.Drilldown(ctx, DrilldownInput{
			Metric: string(test.metric), LedgerOwnerPartyID: ownerA.ID, Currency: "CNY",
		})
		if drilldownErr != nil {
			t.Fatalf("%s Drilldown() error = %v", test.metric, drilldownErr)
		}
		if details.TotalAmount != test.wantAmount || details.FactCount != test.wantCount ||
			(test.obligationID != "" && !containsObligationFact(details.ObligationFacts, test.obligationID)) ||
			(test.cashID != "" && !containsCashFact(details.CashFacts, test.cashID)) {
			t.Fatalf("%s drilldown = %#v", test.metric, details)
		}
	}

	_, err = service.Drilldown(ctx, DrilldownInput{
		Metric: string(MetricPaymentEffective), LedgerOwnerPartyID: ownerB.ID, Currency: "CNY",
	})
	var notFound *DrilldownNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("empty metric drilldown error = %v", err)
	}

	if receivableUSD.ID == "" || receiptUSD.ID == "" || receivableB.ID == "" || receiptB.ID == "" || paymentA.ID == "" {
		t.Fatal("fixture facts unexpectedly empty")
	}
}

func createSummaryParty(
	t *testing.T,
	ctx context.Context,
	service *party.Service,
	actor audit.ActorRef,
	commandID, legalName, registrationIdentifier string,
) party.Party {
	t.Helper()
	result, err := service.Create(ctx, party.CreateInput{
		CommandID: commandID, Kind: "organization", LegalName: legalName,
		RegistrationIdentifier: registrationIdentifier, Actor: actor,
	})
	if err != nil {
		t.Fatalf("create summary Party %s: %v", legalName, err)
	}
	return result.Party
}

func createSummaryObligation(
	t *testing.T,
	ctx context.Context,
	service *obligation.Service,
	actor audit.ActorRef,
	createCommandID, confirmCommandID, direction, ownerID, counterpartyID, title, amount, currency string,
) obligation.Obligation {
	t.Helper()
	draft, err := service.CreateDraft(ctx, obligation.CreateDraftInput{
		CommandID: createCommandID, Direction: direction, LedgerOwnerPartyID: ownerID,
		CounterpartyPartyID: counterpartyID, Title: title, OriginalAmount: amount,
		Currency: currency, DueOn: "2026-12-31", ManualSourceReference: title + " 依据", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create summary obligation %s: %v", title, err)
	}
	confirmed, err := service.Confirm(ctx, obligation.ConfirmInput{
		CommandID: confirmCommandID, ObligationID: draft.Obligation.ID, ExpectedDraftVersion: 1, Actor: actor,
	})
	if err != nil {
		t.Fatalf("confirm summary obligation %s: %v", title, err)
	}
	return confirmed.Obligation
}

func createSummaryCash(
	t *testing.T,
	ctx context.Context,
	service *cashtransaction.Service,
	actor audit.ActorRef,
	commandID, transactionType, ownerID, counterpartyID, description, amount, currency string,
) cashtransaction.Transaction {
	t.Helper()
	result, err := service.Create(ctx, cashtransaction.CreateInput{
		CommandID: commandID, Type: transactionType, LedgerOwnerPartyID: ownerID,
		CounterpartyPartyID: counterpartyID, Description: description, Amount: amount,
		Currency: currency, OccurredOn: "2026-09-20", AccountReference: "汇总测试账户",
		ManualSourceReference: description + " 回单", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create summary cash %s: %v", description, err)
	}
	return result.Transaction
}

func applySummaryAllocation(
	t *testing.T,
	ctx context.Context,
	service *allocation.Service,
	actor audit.ActorRef,
	commandID, cashID, obligationID, amount string,
) allocation.ApplyResult {
	t.Helper()
	result, err := service.Apply(ctx, allocation.ApplyInput{
		CommandID: commandID, CashTransactionID: cashID, ObligationID: obligationID,
		ExpectedObligationRevision: 1, Amount: amount, ManualSourceReference: "汇总测试核销依据", Actor: actor,
	})
	if err != nil {
		t.Fatalf("apply summary allocation: %v", err)
	}
	return result
}

func findSummaryGroup(t *testing.T, report Report, ownerID, currency string) Group {
	t.Helper()
	for _, group := range report.Groups {
		if group.LedgerOwnerPartyID == ownerID && group.Currency == currency {
			return group
		}
	}
	t.Fatalf("summary group %s/%s not found: %#v", ownerID, currency, report.Groups)
	return Group{}
}

func assertSummaryGroup(
	t *testing.T,
	group Group,
	receivable string, receivableCount int64,
	payable string, payableCount int64,
	receipt string, receiptCount int64,
	payment string, paymentCount int64,
	receiptUnallocated string, receiptUnallocatedCount int64,
	paymentUnallocated string, paymentUnallocatedCount int64,
) {
	t.Helper()
	if group.ReceivableOutstanding.Amount != receivable || group.ReceivableOutstanding.FactCount != receivableCount ||
		group.PayableOutstanding.Amount != payable || group.PayableOutstanding.FactCount != payableCount ||
		group.ReceiptEffective.Amount != receipt || group.ReceiptEffective.FactCount != receiptCount ||
		group.PaymentEffective.Amount != payment || group.PaymentEffective.FactCount != paymentCount ||
		group.ReceiptUnallocated.Amount != receiptUnallocated || group.ReceiptUnallocated.FactCount != receiptUnallocatedCount ||
		group.PaymentUnallocated.Amount != paymentUnallocated || group.PaymentUnallocated.FactCount != paymentUnallocatedCount {
		t.Fatalf("summary group = %#v", group)
	}
}

func containsObligationFact(facts []ObligationFact, id string) bool {
	for _, fact := range facts {
		if fact.ID == id {
			return true
		}
	}
	return false
}

func containsCashFact(facts []CashFact, id string) bool {
	for _, fact := range facts {
		if fact.ID == id {
			return true
		}
	}
	return false
}
