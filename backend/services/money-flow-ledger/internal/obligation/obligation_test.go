package obligation

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

const (
	testCommandID      = "11111111-1111-4111-8111-111111111111"
	testActorID        = "22222222-2222-4222-8222-222222222222"
	testOwnerPartyID   = "33333333-3333-4333-8333-333333333333"
	testCounterpartyID = "44444444-4444-4444-8444-444444444444"
	testObligationID   = "55555555-5555-4555-8555-555555555555"
	testAuditID        = "66666666-6666-4666-8666-666666666666"
)

type repositoryStub struct {
	createdValue            Obligation
	createdCommand          CreateCommand
	createdAudit            audit.Entry
	createResult            Obligation
	replayed                bool
	createErr               error
	confirmID               string
	confirmVersion          int64
	confirmCommand          ConfirmationCommand
	confirmAudit            audit.Entry
	confirmResult           Obligation
	confirmRevision         Revision
	confirmReplayed         bool
	confirmErr              error
	adjustValue             Adjustment
	adjustCommand           AdjustmentCommand
	adjustAudit             audit.Entry
	adjustResult            Obligation
	adjustmentResult        Adjustment
	adjustRevision          Revision
	adjustReplayed          bool
	adjustErr               error
	remissionValue          Remission
	remissionCommand        RemissionCommand
	remissionAudit          audit.Entry
	remissionResult         Obligation
	recordedRemission       Remission
	remissionReplayed       bool
	remissionErr            error
	closureValue            Closure
	closureCommand          ClosureCommand
	closureAudit            audit.Entry
	closureResult           Obligation
	closedClosure           Closure
	closureReplayed         bool
	closureErr              error
	listResult              []Obligation
	listErr                 error
	getResult               Obligation
	getFound                bool
	getErr                  error
	getID                   string
	getCalls                int
	detailsResult           Details
	detailsFound            bool
	detailsErr              error
	detailsID               string
	historicalResult        HistoricalSnapshotLookup
	historicalErr           error
	historicalID            string
	historicalAsOf          time.Time
	historicalCalls         int
	cashExpectationValue    CashExpectation
	cashExpectationCommand  CashExpectationCommand
	cashExpectationAudit    audit.Entry
	cashExpectationResult   CashExpectation
	cashExpectationReplayed bool
	cashExpectationErr      error
	cashForecastRows        []CashExpectationRow
	cashForecastErr         error
	cashHistoryObligation   Obligation
	cashHistoryFacts        []CashExpectation
	cashHistoryFound        bool
	cashHistoryErr          error
	cashHistoryID           string
}

func (r *repositoryStub) CreateDraft(_ context.Context, value Obligation, command CreateCommand, entry audit.Entry) (Obligation, bool, error) {
	r.createdValue = value
	r.createdCommand = command
	r.createdAudit = entry
	if r.createErr != nil {
		return Obligation{}, false, r.createErr
	}
	if r.createResult.ID != "" {
		return r.createResult, r.replayed, nil
	}
	return value, r.replayed, nil
}

func (r *repositoryStub) List(context.Context) ([]Obligation, error) {
	return r.listResult, r.listErr
}

func (r *repositoryStub) Confirm(_ context.Context, id string, version int64, command ConfirmationCommand, entry audit.Entry) (Obligation, Revision, bool, error) {
	r.confirmID = id
	r.confirmVersion = version
	r.confirmCommand = command
	r.confirmAudit = entry
	return r.confirmResult, r.confirmRevision, r.confirmReplayed, r.confirmErr
}

func (r *repositoryStub) AdjustAmount(_ context.Context, value Adjustment, command AdjustmentCommand, entry audit.Entry) (Obligation, Adjustment, Revision, bool, error) {
	r.adjustValue = value
	r.adjustCommand = command
	r.adjustAudit = entry
	return r.adjustResult, r.adjustmentResult, r.adjustRevision, r.adjustReplayed, r.adjustErr
}

func (r *repositoryStub) RecordRemission(_ context.Context, value Remission, command RemissionCommand, entry audit.Entry) (Obligation, Remission, bool, error) {
	r.remissionValue = value
	r.remissionCommand = command
	r.remissionAudit = entry
	return r.remissionResult, r.recordedRemission, r.remissionReplayed, r.remissionErr
}

func (r *repositoryStub) Close(_ context.Context, value Closure, command ClosureCommand, entry audit.Entry) (Obligation, Closure, bool, error) {
	r.closureValue = value
	r.closureCommand = command
	r.closureAudit = entry
	return r.closureResult, r.closedClosure, r.closureReplayed, r.closureErr
}

func (r *repositoryStub) Get(_ context.Context, id string) (Obligation, bool, error) {
	r.getCalls++
	r.getID = id
	return r.getResult, r.getFound, r.getErr
}

func (r *repositoryStub) GetDetails(_ context.Context, id string) (Details, bool, error) {
	r.detailsID = id
	return r.detailsResult, r.detailsFound, r.detailsErr
}

func (r *repositoryStub) GetHistoricalSnapshot(_ context.Context, id string, asOf time.Time) (HistoricalSnapshotLookup, error) {
	r.historicalCalls++
	r.historicalID = id
	r.historicalAsOf = asOf
	return r.historicalResult, r.historicalErr
}

func (r *repositoryStub) RecordCashExpectation(_ context.Context, value CashExpectation, command CashExpectationCommand, entry audit.Entry) (CashExpectation, bool, error) {
	r.cashExpectationValue = value
	r.cashExpectationCommand = command
	r.cashExpectationAudit = entry
	if r.cashExpectationErr != nil {
		return CashExpectation{}, false, r.cashExpectationErr
	}
	if r.cashExpectationResult.ID != "" {
		return r.cashExpectationResult, r.cashExpectationReplayed, nil
	}
	return value, r.cashExpectationReplayed, nil
}

func (r *repositoryStub) ListCurrentCashExpectationRows(context.Context) ([]CashExpectationRow, error) {
	return r.cashForecastRows, r.cashForecastErr
}

func (r *repositoryStub) ListCashExpectationHistory(_ context.Context, id string) (Obligation, []CashExpectation, bool, error) {
	r.cashHistoryID = id
	return r.cashHistoryObligation, r.cashHistoryFacts, r.cashHistoryFound, r.cashHistoryErr
}

func testActor() audit.ActorRef {
	return audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: testActorID, DisplayName: "资金管理员"}
}

func validCreateInput() CreateDraftInput {
	return CreateDraftInput{
		CommandID:             testCommandID,
		Direction:             string(DirectionReceivable),
		LedgerOwnerPartyID:    testOwnerPartyID,
		CounterpartyPartyID:   testCounterpartyID,
		Title:                 "九月服务费",
		OriginalAmount:        "100.5",
		Currency:              "CNY",
		DueOn:                 "2026-10-31",
		ManualSourceReference: "人工确认单 MF-001",
		Actor:                 testActor(),
	}
}

func testService(repository Repository) *Service {
	service := NewService(repository)
	ids := []string{testObligationID, testAuditID}
	service.newID = func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}
	service.now = func() time.Time {
		return time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	}
	return service
}

func TestCreateDraftNormalizesReceivableAndBuildsAudit(t *testing.T) {
	repository := &repositoryStub{}
	service := testService(repository)
	input := validCreateInput()
	input.CommandID = " 11111111-1111-4111-8111-111111111111 "
	input.Direction = " RECEIVABLE "
	input.LedgerOwnerPartyID = " 33333333-3333-4333-8333-333333333333 "
	input.Title = "  九月服务费  "
	input.OriginalAmount = "000100.500000"
	input.Currency = " cny "
	input.ManualSourceReference = "  人工确认单 MF-001  "

	result, err := service.CreateDraft(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateDraft() error = %v", err)
	}
	value := result.Obligation
	if result.Replayed || value.ID != testObligationID || value.Direction != DirectionReceivable ||
		value.LedgerOwnerPartyID != testOwnerPartyID || value.Creditor.PartyID != testOwnerPartyID ||
		value.Debtor.PartyID != testCounterpartyID || value.Title != "九月服务费" ||
		value.OriginalAmount != "100.5" || value.Currency != "CNY" || value.DueOn != "2026-10-31" ||
		value.SourceKind != SourceKindManual || value.ManualSourceReference != "人工确认单 MF-001" ||
		value.Status != StatusDraft || value.DraftVersion != 1 || !value.CreatedAt.Equal(value.UpdatedAt) {
		t.Fatalf("obligation = %#v", value)
	}
	if repository.createdCommand.CommandID != testCommandID || len(repository.createdCommand.PayloadHash) != 32 ||
		repository.createdCommand.Actor != testActor() {
		t.Fatalf("command = %#v", repository.createdCommand)
	}
	entry := repository.createdAudit
	if entry.ID != testAuditID || entry.ObjectType != audit.ObjectObligation || entry.ObjectID != testObligationID ||
		entry.Action != AuditActionDraftCreated || entry.CommandID != testCommandID || entry.Actor != testActor() {
		t.Fatalf("audit = %#v", entry)
	}
	if entry.Details["direction"] != string(DirectionReceivable) || entry.Details["ledger_owner_party_id"] != testOwnerPartyID ||
		entry.Details["counterparty_party_id"] != testCounterpartyID || entry.Details["original_amount"] != "100.5" ||
		entry.Details["resulting_draft_version"] != int64(1) {
		t.Fatalf("audit details = %#v", entry.Details)
	}
}

func TestCreateDraftMapsPayableParties(t *testing.T) {
	repository := &repositoryStub{}
	input := validCreateInput()
	input.Direction = string(DirectionPayable)
	result, err := testService(repository).CreateDraft(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateDraft() error = %v", err)
	}
	if result.Obligation.Creditor.PartyID != testCounterpartyID || result.Obligation.Debtor.PartyID != testOwnerPartyID ||
		result.Obligation.LedgerOwner().PartyID != testOwnerPartyID || result.Obligation.Counterparty().PartyID != testCounterpartyID {
		t.Fatalf("payable parties = creditor %#v debtor %#v", result.Obligation.Creditor, result.Obligation.Debtor)
	}
}

func TestCreateDraftValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CreateDraftInput)
		field  string
	}{
		{name: "command", mutate: func(v *CreateDraftInput) { v.CommandID = "bad" }, field: "command_id"},
		{name: "actor", mutate: func(v *CreateDraftInput) { v.Actor.ID = "bad" }, field: "actor"},
		{name: "direction", mutate: func(v *CreateDraftInput) { v.Direction = "income" }, field: "direction"},
		{name: "owner ID", mutate: func(v *CreateDraftInput) { v.LedgerOwnerPartyID = "bad" }, field: "ledger_owner_party_id"},
		{name: "counterparty ID", mutate: func(v *CreateDraftInput) { v.CounterpartyPartyID = "bad" }, field: "counterparty_party_id"},
		{name: "same parties", mutate: func(v *CreateDraftInput) { v.CounterpartyPartyID = v.LedgerOwnerPartyID }, field: "counterparty_party_id"},
		{name: "title empty", mutate: func(v *CreateDraftInput) { v.Title = " " }, field: "title"},
		{name: "title long", mutate: func(v *CreateDraftInput) { v.Title = strings.Repeat("界", 201) }, field: "title"},
		{name: "amount empty", mutate: func(v *CreateDraftInput) { v.OriginalAmount = "" }, field: "original_amount"},
		{name: "amount zero", mutate: func(v *CreateDraftInput) { v.OriginalAmount = "0.000000" }, field: "original_amount"},
		{name: "amount negative", mutate: func(v *CreateDraftInput) { v.OriginalAmount = "-1" }, field: "original_amount"},
		{name: "amount exponent", mutate: func(v *CreateDraftInput) { v.OriginalAmount = "1e3" }, field: "original_amount"},
		{name: "amount integer precision", mutate: func(v *CreateDraftInput) { v.OriginalAmount = "1234567890123456789" }, field: "original_amount"},
		{name: "amount scale", mutate: func(v *CreateDraftInput) { v.OriginalAmount = "1.0000001" }, field: "original_amount"},
		{name: "currency", mutate: func(v *CreateDraftInput) { v.Currency = "人民币" }, field: "currency"},
		{name: "date empty", mutate: func(v *CreateDraftInput) { v.DueOn = "" }, field: "due_on"},
		{name: "date invalid", mutate: func(v *CreateDraftInput) { v.DueOn = "2026-02-30" }, field: "due_on"},
		{name: "date year zero", mutate: func(v *CreateDraftInput) { v.DueOn = "0000-01-01" }, field: "due_on"},
		{name: "source empty", mutate: func(v *CreateDraftInput) { v.ManualSourceReference = " " }, field: "manual_source_reference"},
		{name: "source long", mutate: func(v *CreateDraftInput) { v.ManualSourceReference = strings.Repeat("界", 201) }, field: "manual_source_reference"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validCreateInput()
			test.mutate(&input)
			repository := &repositoryStub{}
			_, err := testService(repository).CreateDraft(context.Background(), input)
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Field != test.field {
				t.Fatalf("CreateDraft() error = %v", err)
			}
			if repository.createdValue.ID != "" {
				t.Fatalf("repository called with %#v", repository.createdValue)
			}
		})
	}
}

func TestNormalizeAmountUsesExactCanonicalDecimal(t *testing.T) {
	tests := map[string]string{
		"0.000001":                  "0.000001",
		"000100.500000":             "100.5",
		"0000001":                   "1",
		"999999999999999999.999999": "999999999999999999.999999",
	}
	for input, want := range tests {
		got, err := normalizeAmount(input)
		if err != nil || got != want {
			t.Errorf("normalizeAmount(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
}

func TestCreatePayloadHashUsesNormalizedIntentAndTrustedActor(t *testing.T) {
	first, err := normalizeCreateDraftInput(validCreateInput())
	if err != nil {
		t.Fatalf("normalize first: %v", err)
	}
	secondInput := validCreateInput()
	secondInput.Direction = " RECEIVABLE "
	secondInput.Title = " 九月服务费 "
	secondInput.OriginalAmount = "000100.500000"
	secondInput.Currency = " cny "
	secondInput.ManualSourceReference = " 人工确认单 MF-001 "
	second, err := normalizeCreateDraftInput(secondInput)
	if err != nil {
		t.Fatalf("normalize second: %v", err)
	}
	firstHash, err := createPayloadHash(first)
	if err != nil {
		t.Fatalf("first hash: %v", err)
	}
	secondHash, err := createPayloadHash(second)
	if err != nil {
		t.Fatalf("second hash: %v", err)
	}
	if !bytes.Equal(firstHash, secondHash) {
		t.Fatalf("normalized hashes differ: %x %x", firstHash, secondHash)
	}
	second.Actor.ID = "77777777-7777-4777-8777-777777777777"
	changedActorHash, err := createPayloadHash(second)
	if err != nil {
		t.Fatalf("changed actor hash: %v", err)
	}
	if bytes.Equal(firstHash, changedActorHash) {
		t.Fatal("actor identity is absent from payload hash")
	}
}

func TestCreateDraftReturnsReplayAndPreservesTypedErrors(t *testing.T) {
	replayed := Obligation{ID: "88888888-8888-4888-8888-888888888888", Title: "原草稿"}
	repository := &repositoryStub{createResult: replayed, replayed: true}
	result, err := testService(repository).CreateDraft(context.Background(), validCreateInput())
	if err != nil || !result.Replayed || result.Obligation.ID != replayed.ID {
		t.Fatalf("CreateDraft() = %#v, %v", result, err)
	}

	for _, typedError := range []error{
		&IdempotencyConflictError{CommandID: testCommandID},
		&PartyNotFoundError{PartyID: testCounterpartyID},
	} {
		repository = &repositoryStub{createErr: typedError}
		_, err = testService(repository).CreateDraft(context.Background(), validCreateInput())
		if !errors.Is(err, typedError) {
			t.Fatalf("CreateDraft() error = %v, want wrapped %T", err, typedError)
		}
	}
}

func TestListAndGet(t *testing.T) {
	want := Obligation{ID: testObligationID, Title: "义务草稿"}
	repository := &repositoryStub{listResult: []Obligation{want}, getResult: want, getFound: true}
	service := NewService(repository)
	listed, err := service.List(context.Background())
	if err != nil || len(listed) != 1 || listed[0].ID != want.ID {
		t.Fatalf("List() = %#v, %v", listed, err)
	}
	got, found, err := service.Get(context.Background(), " 55555555-5555-4555-8555-555555555555 ")
	if err != nil || !found || got.ID != want.ID || repository.getID != testObligationID {
		t.Fatalf("Get() = %#v, %v, %v; repository ID=%q", got, found, err, repository.getID)
	}

	_, _, err = service.Get(context.Background(), "bad")
	var validationError *ValidationError
	if !errors.As(err, &validationError) || repository.getCalls != 1 {
		t.Fatalf("invalid Get() error=%v calls=%d", err, repository.getCalls)
	}
}

func TestListAndGetPreserveRepositoryErrors(t *testing.T) {
	sentinel := errors.New("database unavailable")
	service := NewService(&repositoryStub{listErr: sentinel})
	if _, err := service.List(context.Background()); !errors.Is(err, sentinel) {
		t.Fatalf("List() error = %v", err)
	}
	service = NewService(&repositoryStub{getErr: sentinel})
	if _, _, err := service.Get(context.Background(), testObligationID); !errors.Is(err, sentinel) {
		t.Fatalf("Get() error = %v", err)
	}
}

func TestNewCommandID(t *testing.T) {
	id, err := NewCommandID()
	if err != nil || !uuidPattern.MatchString(id) {
		t.Fatalf("NewCommandID() = %q, %v", id, err)
	}
}

func TestConfirmNormalizesIntentAndBuildsTrustedAudit(t *testing.T) {
	confirmedAt := time.Date(2026, 9, 19, 14, 0, 0, 0, time.UTC)
	repository := &repositoryStub{
		confirmResult:   Obligation{ID: testObligationID, Status: StatusOpen, CurrentRevision: 1},
		confirmRevision: Revision{ObligationID: testObligationID, Revision: 1},
	}
	service := NewService(repository)
	service.newID = func() (string, error) { return testAuditID, nil }
	service.now = func() time.Time { return confirmedAt }
	result, err := service.Confirm(context.Background(), ConfirmInput{
		CommandID:            " 11111111-1111-4111-8111-111111111111 ",
		ObligationID:         " 55555555-5555-4555-8555-555555555555 ",
		ExpectedDraftVersion: 1,
		Actor:                testActor(),
	})
	if err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	if result.Replayed || result.Obligation.Status != StatusOpen || result.Revision.Revision != 1 ||
		repository.confirmID != testObligationID || repository.confirmVersion != 1 {
		t.Fatalf("Confirm() result=%#v repository=%#v", result, repository)
	}
	if repository.confirmCommand.CommandID != testCommandID || len(repository.confirmCommand.PayloadHash) != 32 ||
		repository.confirmCommand.Actor != testActor() || !repository.confirmCommand.ConfirmedAt.Equal(confirmedAt) {
		t.Fatalf("confirmation command = %#v", repository.confirmCommand)
	}
	entry := repository.confirmAudit
	if entry.ID != testAuditID || entry.ObjectType != audit.ObjectObligation || entry.ObjectID != testObligationID ||
		entry.Action != AuditActionConfirmed || entry.CommandID != testCommandID || entry.Actor != testActor() ||
		entry.Details["expected_draft_version"] != int64(1) || entry.Details["resulting_revision"] != int64(1) ||
		entry.Details["resulting_status"] != StatusOpen {
		t.Fatalf("confirmation audit = %#v", entry)
	}
}

func TestConfirmValidationDoesNotCallRepository(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ConfirmInput)
		field  string
	}{
		{name: "command", mutate: func(v *ConfirmInput) { v.CommandID = "bad" }, field: "command_id"},
		{name: "obligation", mutate: func(v *ConfirmInput) { v.ObligationID = "bad" }, field: "obligation_id"},
		{name: "version", mutate: func(v *ConfirmInput) { v.ExpectedDraftVersion = 0 }, field: "expected_draft_version"},
		{name: "actor", mutate: func(v *ConfirmInput) { v.Actor.ID = "bad" }, field: "actor"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := ConfirmInput{CommandID: testCommandID, ObligationID: testObligationID, ExpectedDraftVersion: 1, Actor: testActor()}
			test.mutate(&input)
			repository := &repositoryStub{}
			_, err := NewService(repository).Confirm(context.Background(), input)
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Field != test.field {
				t.Fatalf("Confirm() error = %v", err)
			}
			if repository.confirmID != "" {
				t.Fatalf("repository called with %q", repository.confirmID)
			}
		})
	}
}

func TestConfirmationPayloadHashUsesNormalizedIntentAndTrustedActor(t *testing.T) {
	first, err := normalizeConfirmInput(ConfirmInput{
		CommandID: testCommandID, ObligationID: testObligationID, ExpectedDraftVersion: 1, Actor: testActor(),
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := normalizeConfirmInput(ConfirmInput{
		CommandID: " " + testCommandID + " ", ObligationID: " " + testObligationID + " ", ExpectedDraftVersion: 1, Actor: testActor(),
	})
	if err != nil {
		t.Fatal(err)
	}
	firstHash, _ := confirmationPayloadHash(first)
	secondHash, _ := confirmationPayloadHash(second)
	if !bytes.Equal(firstHash, secondHash) {
		t.Fatalf("normalized hashes differ: %x %x", firstHash, secondHash)
	}
	second.Actor.ID = "77777777-7777-4777-8777-777777777777"
	changedHash, _ := confirmationPayloadHash(second)
	if bytes.Equal(firstHash, changedHash) {
		t.Fatal("actor identity is absent from confirmation payload hash")
	}
}

func TestConfirmReturnsReplayAndPreservesTypedErrors(t *testing.T) {
	repository := &repositoryStub{
		confirmResult:   Obligation{ID: testObligationID, Status: StatusOpen},
		confirmRevision: Revision{ObligationID: testObligationID, Revision: 1},
		confirmReplayed: true,
	}
	service := NewService(repository)
	result, err := service.Confirm(context.Background(), ConfirmInput{
		CommandID: testCommandID, ObligationID: testObligationID, ExpectedDraftVersion: 1, Actor: testActor(),
	})
	if err != nil || !result.Replayed || result.Revision.Revision != 1 {
		t.Fatalf("Confirm() = %#v, %v", result, err)
	}
	for _, typedError := range []error{
		&ConfirmationIdempotencyConflictError{CommandID: testCommandID},
		&ObligationNotFoundError{ObligationID: testObligationID},
		&ObligationStateConflictError{Status: StatusOpen},
		&DraftVersionConflictError{Expected: 1, Actual: 2},
	} {
		repository = &repositoryStub{confirmErr: typedError}
		_, err = NewService(repository).Confirm(context.Background(), ConfirmInput{
			CommandID: testCommandID, ObligationID: testObligationID, ExpectedDraftVersion: 1, Actor: testActor(),
		})
		if !errors.Is(err, typedError) {
			t.Fatalf("Confirm() error = %v, want wrapped %T", err, typedError)
		}
	}
}

func TestGetDetailsValidatesAndPreservesRepositoryErrors(t *testing.T) {
	want := Details{Obligation: Obligation{ID: testObligationID}, Revisions: []Revision{{Revision: 1}}}
	repository := &repositoryStub{detailsResult: want, detailsFound: true}
	service := NewService(repository)
	got, found, err := service.GetDetails(context.Background(), " "+testObligationID+" ")
	if err != nil || !found || got.Obligation.ID != testObligationID || repository.detailsID != testObligationID {
		t.Fatalf("GetDetails() = %#v, %v, %v", got, found, err)
	}
	_, _, err = service.GetDetails(context.Background(), "bad")
	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("invalid GetDetails() error = %v", err)
	}
	sentinel := errors.New("database unavailable")
	service = NewService(&repositoryStub{detailsErr: sentinel})
	if _, _, err := service.GetDetails(context.Background(), testObligationID); !errors.Is(err, sentinel) {
		t.Fatalf("GetDetails() error = %v", err)
	}
}
