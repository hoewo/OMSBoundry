package obligation

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/party"
)

const (
	testCashExpectationID      = "77777777-7777-4777-8777-777777777771"
	testCashExpectationAuditID = "88888888-8888-4888-8888-888888888881"
)

func cashExpectationService(repository *repositoryStub) *Service {
	service := NewService(repository)
	ids := []string{testCashExpectationID, testCashExpectationAuditID}
	service.newID = func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}
	service.now = func() time.Time {
		return time.Date(2026, 9, 20, 8, 30, 0, 0, time.UTC)
	}
	return service
}

func validCashExpectationInput() RecordCashExpectationInput {
	return RecordCashExpectationInput{
		CommandID: testCommandID, ObligationID: testObligationID,
		ExpectedCurrentRevision: 2, Action: string(CashExpectationActionSet),
		ExpectedOn: "2026-10-15", Reason: "客户确认十月中旬付款", Actor: testActor(),
	}
}

func TestRecordCashExpectationNormalizesAndBuildsAudit(t *testing.T) {
	repository := &repositoryStub{}
	input := validCashExpectationInput()
	input.CommandID = " " + input.CommandID + " "
	input.ObligationID = " " + input.ObligationID + " "
	input.Action = " SET "
	input.ExpectedOn = " 2026-10-15 "
	input.Reason = "  客户确认十月中旬付款  "

	result, err := cashExpectationService(repository).RecordCashExpectation(context.Background(), input)
	if err != nil {
		t.Fatalf("RecordCashExpectation() error = %v", err)
	}
	value := repository.cashExpectationValue
	if result.Replayed || value.ID != testCashExpectationID || value.ObligationID != testObligationID ||
		value.Action != CashExpectationActionSet || value.ExpectedOn != "2026-10-15" || value.ObligationRevision != 2 ||
		value.SourceKind != SourceKindManual || value.Reason != "客户确认十月中旬付款" || value.RecordedBy != testActor() ||
		!value.RecordedAt.Equal(time.Date(2026, 9, 20, 8, 30, 0, 0, time.UTC)) || value.Sequence != 0 {
		t.Fatalf("cash expectation = %#v", value)
	}
	command := repository.cashExpectationCommand
	if command.CommandID != testCommandID || len(command.PayloadHash) != 32 || command.Actor != testActor() ||
		!command.RecordedAt.Equal(value.RecordedAt) {
		t.Fatalf("cash expectation command = %#v", command)
	}
	entry := repository.cashExpectationAudit
	if entry.ID != testCashExpectationAuditID || entry.ObjectType != audit.ObjectObligation || entry.ObjectID != testObligationID ||
		entry.Action != AuditActionCashExpectationRecorded || entry.CommandID != testCommandID || entry.Actor != testActor() ||
		entry.Details["action"] != string(CashExpectationActionSet) || entry.Details["expected_on"] != "2026-10-15" ||
		entry.Details["expected_current_revision"] != int64(2) || entry.Details["expectation_reason"] != "客户确认十月中旬付款" {
		t.Fatalf("cash expectation audit = %#v", entry)
	}
}

func TestRecordCashExpectationValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*RecordCashExpectationInput)
		field  string
	}{
		{name: "command", mutate: func(v *RecordCashExpectationInput) { v.CommandID = "bad" }, field: "command_id"},
		{name: "obligation", mutate: func(v *RecordCashExpectationInput) { v.ObligationID = "bad" }, field: "obligation_id"},
		{name: "revision", mutate: func(v *RecordCashExpectationInput) { v.ExpectedCurrentRevision = 0 }, field: "expected_current_revision"},
		{name: "action", mutate: func(v *RecordCashExpectationInput) { v.Action = "predict" }, field: "action"},
		{name: "empty date", mutate: func(v *RecordCashExpectationInput) { v.ExpectedOn = "" }, field: "expected_on"},
		{name: "invalid date", mutate: func(v *RecordCashExpectationInput) { v.ExpectedOn = "2026-02-30" }, field: "expected_on"},
		{name: "clear with date", mutate: func(v *RecordCashExpectationInput) { v.Action = "clear" }, field: "expected_on"},
		{name: "reason", mutate: func(v *RecordCashExpectationInput) { v.Reason = " " }, field: "expectation_reason"},
		{name: "long reason", mutate: func(v *RecordCashExpectationInput) { v.Reason = strings.Repeat("界", 201) }, field: "expectation_reason"},
		{name: "actor", mutate: func(v *RecordCashExpectationInput) { v.Actor.ID = "bad" }, field: "actor"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validCashExpectationInput()
			test.mutate(&input)
			repository := &repositoryStub{}
			_, err := cashExpectationService(repository).RecordCashExpectation(context.Background(), input)
			var validation *ValidationError
			if !errors.As(err, &validation) || validation.Field != test.field {
				t.Fatalf("RecordCashExpectation() error = %v", err)
			}
			if repository.cashExpectationValue.ID != "" {
				t.Fatalf("repository called with %#v", repository.cashExpectationValue)
			}
		})
	}

	clearInput := validCashExpectationInput()
	clearInput.Action = " clear "
	clearInput.ExpectedOn = ""
	repository := &repositoryStub{}
	if _, err := cashExpectationService(repository).RecordCashExpectation(context.Background(), clearInput); err != nil {
		t.Fatalf("valid clear error = %v", err)
	}
	if repository.cashExpectationValue.Action != CashExpectationActionClear || repository.cashExpectationValue.ExpectedOn != "" {
		t.Fatalf("clear value = %#v", repository.cashExpectationValue)
	}
}

func TestCashExpectationPayloadHashUsesNormalizedIntent(t *testing.T) {
	first, err := normalizeCashExpectationInput(validCashExpectationInput())
	if err != nil {
		t.Fatal(err)
	}
	secondInput := validCashExpectationInput()
	secondInput.CommandID = " " + secondInput.CommandID + " "
	secondInput.Action = " SET "
	secondInput.Reason = " 客户确认十月中旬付款 "
	second, err := normalizeCashExpectationInput(secondInput)
	if err != nil {
		t.Fatal(err)
	}
	firstHash, _ := cashExpectationPayloadHash(first)
	secondHash, _ := cashExpectationPayloadHash(second)
	if !bytes.Equal(firstHash, secondHash) {
		t.Fatalf("normalized hashes differ: %x %x", firstHash, secondHash)
	}
	for _, change := range []func(*RecordCashExpectationInput){
		func(v *RecordCashExpectationInput) { v.ExpectedCurrentRevision++ },
		func(v *RecordCashExpectationInput) { v.Action = "clear"; v.ExpectedOn = "" },
		func(v *RecordCashExpectationInput) { v.ExpectedOn = "2026-10-16" },
		func(v *RecordCashExpectationInput) { v.Reason = "其他原因" },
		func(v *RecordCashExpectationInput) { v.Actor.ID = "99999999-9999-4999-8999-999999999999" },
	} {
		changed := first
		change(&changed)
		changedHash, _ := cashExpectationPayloadHash(changed)
		if bytes.Equal(firstHash, changedHash) {
			t.Fatalf("changed intent is absent from payload hash: %#v", changed)
		}
	}
}

func TestCurrentCashForecastSeparatesScheduledAndUnscheduledExactly(t *testing.T) {
	ownerA := PartySnapshot{PartyID: testOwnerPartyID, Kind: party.KindOrganization, LegalName: "甲公司", PartyRevision: 1}
	ownerB := PartySnapshot{PartyID: "99999999-9999-4999-8999-999999999999", Kind: party.KindOrganization, LegalName: "乙公司", PartyRevision: 1}
	counterparty := PartySnapshot{PartyID: testCounterpartyID, Kind: party.KindOrganization, LegalName: "往来方", PartyRevision: 1}
	makeObligation := func(id string, owner PartySnapshot, direction Direction, amount, currency string) Obligation {
		value := Obligation{ID: id, Direction: direction, Title: id, LedgerOwnerPartyID: owner.PartyID,
			OutstandingAmount: amount, Currency: currency, Status: StatusOpen, CurrentRevision: 1}
		if direction == DirectionReceivable {
			value.Creditor, value.Debtor = owner, counterparty
		} else {
			value.Creditor, value.Debtor = counterparty, owner
		}
		return value
	}
	setA := CashExpectation{Action: CashExpectationActionSet, ExpectedOn: "2026-10-15"}
	setB := CashExpectation{Action: CashExpectationActionSet, ExpectedOn: "2026-10-16"}
	cleared := CashExpectation{Action: CashExpectationActionClear}
	repository := &repositoryStub{cashForecastRows: []CashExpectationRow{
		{Obligation: makeObligation("00000000-0000-4000-8000-000000000004", ownerA, DirectionPayable, "4.000001", "CNY"), Expectation: &setB},
		{Obligation: makeObligation("00000000-0000-4000-8000-000000000002", ownerA, DirectionReceivable, "0.000001", "CNY"), Expectation: &setA},
		{Obligation: makeObligation("00000000-0000-4000-8000-000000000001", ownerA, DirectionReceivable, "2.5", "CNY"), Expectation: &setA},
		{Obligation: makeObligation("00000000-0000-4000-8000-000000000003", ownerB, DirectionReceivable, "3", "USD"), Expectation: &cleared},
		{Obligation: makeObligation("00000000-0000-4000-8000-000000000005", ownerA, DirectionPayable, "5", "CNY")},
		{Obligation: func() Obligation {
			value := makeObligation("00000000-0000-4000-8000-000000000006", ownerA, DirectionReceivable, "9", "CNY")
			value.Status = StatusClosed
			return value
		}(), Expectation: &setA},
	}}
	report, err := cashExpectationService(repository).CurrentCashForecast(context.Background())
	if err != nil {
		t.Fatalf("CurrentCashForecast() error = %v", err)
	}
	if len(report.ScheduledItems) != 3 || len(report.UnscheduledItems) != 2 ||
		len(report.ScheduledSummaries) != 2 || len(report.UnscheduledSummaries) != 2 {
		t.Fatalf("cash forecast report = %#v", report)
	}
	first := report.ScheduledSummaries[0]
	if first.ExpectedOn != "2026-10-15" || first.LedgerOwnerPartyID != ownerA.PartyID ||
		first.Direction != DirectionReceivable || first.Currency != "CNY" || first.ObligationCount != 2 ||
		first.OutstandingAmount != "2.500001" {
		t.Fatalf("first scheduled summary = %#v", first)
	}
	if report.ScheduledItems[0].Obligation.ID != "00000000-0000-4000-8000-000000000001" ||
		report.ScheduledItems[1].Obligation.ID != "00000000-0000-4000-8000-000000000002" {
		t.Fatalf("scheduled order = %#v", report.ScheduledItems)
	}
}

func TestCashExpectationHistoryAndTypedErrors(t *testing.T) {
	repository := &repositoryStub{
		cashHistoryObligation: Obligation{ID: testObligationID},
		cashHistoryFacts:      []CashExpectation{{ID: testCashExpectationID, Sequence: 1}},
		cashHistoryFound:      true,
	}
	history, err := cashExpectationService(repository).CashExpectationHistory(context.Background(), " "+testObligationID+" ")
	if err != nil || history.Obligation.ID != testObligationID || len(history.Facts) != 1 || repository.cashHistoryID != testObligationID {
		t.Fatalf("CashExpectationHistory() = %#v, %v", history, err)
	}
	for _, typed := range []error{
		&CashExpectationIdempotencyConflictError{}, &ObligationNotFoundError{},
		&CashExpectationStateConflictError{}, &CurrentRevisionConflictError{},
		&CashExpectationBalanceConflictError{}, &CashExpectationClearConflictError{},
	} {
		repository = &repositoryStub{cashExpectationErr: typed}
		_, err = cashExpectationService(repository).RecordCashExpectation(context.Background(), validCashExpectationInput())
		if !errors.Is(err, typed) {
			t.Fatalf("RecordCashExpectation() error = %v, want %T", err, typed)
		}
	}
	repository = &repositoryStub{cashHistoryFound: false}
	_, err = cashExpectationService(repository).CashExpectationHistory(context.Background(), testObligationID)
	var notFound *ObligationNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("missing history error = %v", err)
	}
}

func TestCashExpectationActionLabels(t *testing.T) {
	if CashExpectationActionSet.Label() != "设置／改期" || CashExpectationActionClear.Label() != "清除排期" {
		t.Fatal("cash expectation action labels changed")
	}
}
