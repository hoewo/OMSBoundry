package cashtransaction

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
	testTransactionID  = "55555555-5555-4555-8555-555555555555"
	testAuditID        = "66666666-6666-4666-8666-666666666666"
)

type repositoryStub struct {
	createdValue    Transaction
	createdCommand  CreateCommand
	createdAudit    audit.Entry
	createResult    Transaction
	replayed        bool
	createErr       error
	reversedValue   Reversal
	reverseCommand  ReverseCommand
	reverseAudit    audit.Entry
	reverseResult   Reversal
	reverseReplayed bool
	reverseErr      error
	listResult      []Transaction
	listErr         error
	getResult       Transaction
	getFound        bool
	getErr          error
	getID           string
	getCalls        int
}

func (r *repositoryStub) Create(_ context.Context, value Transaction, command CreateCommand, entry audit.Entry) (Transaction, bool, error) {
	r.createdValue = value
	r.createdCommand = command
	r.createdAudit = entry
	if r.createErr != nil {
		return Transaction{}, false, r.createErr
	}
	if r.createResult.ID != "" {
		return r.createResult, r.replayed, nil
	}
	return value, r.replayed, nil
}

func (r *repositoryStub) Reverse(_ context.Context, value Reversal, command ReverseCommand, entry audit.Entry) (Reversal, bool, error) {
	r.reversedValue = value
	r.reverseCommand = command
	r.reverseAudit = entry
	if r.reverseErr != nil {
		return Reversal{}, false, r.reverseErr
	}
	if r.reverseResult.ID != "" {
		return r.reverseResult, r.reverseReplayed, nil
	}
	return value, r.reverseReplayed, nil
}

func (r *repositoryStub) List(context.Context) ([]Transaction, error) {
	return r.listResult, r.listErr
}

func (r *repositoryStub) Get(_ context.Context, id string) (Transaction, bool, error) {
	r.getCalls++
	r.getID = id
	return r.getResult, r.getFound, r.getErr
}

func testActor() audit.ActorRef {
	return audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: testActorID, DisplayName: "资金管理员"}
}

func validCreateInput() CreateInput {
	return CreateInput{
		CommandID:                    testCommandID,
		Type:                         string(TypeReceipt),
		LedgerOwnerPartyID:           testOwnerPartyID,
		CounterpartyPartyID:          testCounterpartyID,
		Description:                  "九月服务费收款",
		Amount:                       "100.5",
		Currency:                     "CNY",
		OccurredOn:                   "2026-09-19",
		AccountReference:             "基本户尾号 1234",
		ManualSourceReference:        "银行回单截图 CASH-001",
		ExternalTransactionReference: "BANK-TXN-001",
		Actor:                        testActor(),
	}
}

func testService(repository Repository) *Service {
	service := NewService(repository)
	ids := []string{testTransactionID, testAuditID}
	service.newID = func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}
	service.now = func() time.Time {
		return time.Date(2026, 9, 19, 16, 0, 0, 0, time.UTC)
	}
	return service
}

func TestCreateNormalizesReceiptAndBuildsAudit(t *testing.T) {
	repository := &repositoryStub{}
	service := testService(repository)
	input := validCreateInput()
	input.CommandID = " 11111111-1111-4111-8111-111111111111 "
	input.Type = " RECEIPT "
	input.LedgerOwnerPartyID = " 33333333-3333-4333-8333-333333333333 "
	input.Description = "  九月服务费收款  "
	input.Amount = "000100.500000"
	input.Currency = " cny "
	input.AccountReference = "  基本户尾号 1234  "
	input.ManualSourceReference = "  银行回单截图 CASH-001  "
	input.ExternalTransactionReference = "  BANK-TXN-001  "

	result, err := service.Create(context.Background(), input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	value := result.Transaction
	if result.Replayed || value.ID != testTransactionID || value.Type != TypeReceipt ||
		value.LedgerOwnerPartyID != testOwnerPartyID || value.Payer.PartyID != testCounterpartyID ||
		value.Payee.PartyID != testOwnerPartyID || value.Description != "九月服务费收款" ||
		value.Amount != "100.5" || value.Currency != "CNY" || value.OccurredOn != "2026-09-19" ||
		value.AccountReference != "基本户尾号 1234" || value.SourceKind != SourceKindManual ||
		value.ManualSourceReference != "银行回单截图 CASH-001" || value.ExternalTransactionReference != "BANK-TXN-001" ||
		value.Status != StatusRecorded || value.RecordedBy != testActor() {
		t.Fatalf("transaction = %#v", value)
	}
	if repository.createdCommand.CommandID != testCommandID || len(repository.createdCommand.PayloadHash) != 32 ||
		repository.createdCommand.Actor != testActor() {
		t.Fatalf("command = %#v", repository.createdCommand)
	}
	entry := repository.createdAudit
	if entry.ID != testAuditID || entry.ObjectType != audit.ObjectCashTransaction || entry.ObjectID != testTransactionID ||
		entry.Action != AuditActionRecorded || entry.CommandID != testCommandID || entry.Actor != testActor() {
		t.Fatalf("audit = %#v", entry)
	}
	if entry.Details["type"] != string(TypeReceipt) || entry.Details["ledger_owner_party_id"] != testOwnerPartyID ||
		entry.Details["counterparty_party_id"] != testCounterpartyID || entry.Details["amount"] != "100.5" ||
		entry.Details["resulting_status"] != StatusRecorded {
		t.Fatalf("audit details = %#v", entry.Details)
	}
}

func TestCreateMapsPaymentParties(t *testing.T) {
	repository := &repositoryStub{}
	input := validCreateInput()
	input.Type = string(TypePayment)
	result, err := testService(repository).Create(context.Background(), input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.Transaction.Payer.PartyID != testOwnerPartyID || result.Transaction.Payee.PartyID != testCounterpartyID ||
		result.Transaction.LedgerOwner().PartyID != testOwnerPartyID || result.Transaction.Counterparty().PartyID != testCounterpartyID {
		t.Fatalf("payment parties = payer %#v payee %#v", result.Transaction.Payer, result.Transaction.Payee)
	}
}

func TestCreateValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CreateInput)
		field  string
	}{
		{name: "command", mutate: func(v *CreateInput) { v.CommandID = "bad" }, field: "command_id"},
		{name: "actor", mutate: func(v *CreateInput) { v.Actor.ID = "bad" }, field: "actor"},
		{name: "type", mutate: func(v *CreateInput) { v.Type = "transfer" }, field: "type"},
		{name: "owner ID", mutate: func(v *CreateInput) { v.LedgerOwnerPartyID = "bad" }, field: "ledger_owner_party_id"},
		{name: "counterparty ID", mutate: func(v *CreateInput) { v.CounterpartyPartyID = "bad" }, field: "counterparty_party_id"},
		{name: "same parties", mutate: func(v *CreateInput) { v.CounterpartyPartyID = v.LedgerOwnerPartyID }, field: "counterparty_party_id"},
		{name: "description empty", mutate: func(v *CreateInput) { v.Description = " " }, field: "description"},
		{name: "description long", mutate: func(v *CreateInput) { v.Description = strings.Repeat("界", 201) }, field: "description"},
		{name: "amount empty", mutate: func(v *CreateInput) { v.Amount = "" }, field: "amount"},
		{name: "amount zero", mutate: func(v *CreateInput) { v.Amount = "0.000000" }, field: "amount"},
		{name: "amount negative", mutate: func(v *CreateInput) { v.Amount = "-1" }, field: "amount"},
		{name: "amount exponent", mutate: func(v *CreateInput) { v.Amount = "1e3" }, field: "amount"},
		{name: "amount integer precision", mutate: func(v *CreateInput) { v.Amount = "1234567890123456789" }, field: "amount"},
		{name: "amount scale", mutate: func(v *CreateInput) { v.Amount = "1.0000001" }, field: "amount"},
		{name: "currency", mutate: func(v *CreateInput) { v.Currency = "人民币" }, field: "currency"},
		{name: "date empty", mutate: func(v *CreateInput) { v.OccurredOn = "" }, field: "occurred_on"},
		{name: "date invalid", mutate: func(v *CreateInput) { v.OccurredOn = "2026-02-30" }, field: "occurred_on"},
		{name: "date year zero", mutate: func(v *CreateInput) { v.OccurredOn = "0000-01-01" }, field: "occurred_on"},
		{name: "account empty", mutate: func(v *CreateInput) { v.AccountReference = " " }, field: "account_reference"},
		{name: "account long", mutate: func(v *CreateInput) { v.AccountReference = strings.Repeat("界", 101) }, field: "account_reference"},
		{name: "source empty", mutate: func(v *CreateInput) { v.ManualSourceReference = " " }, field: "manual_source_reference"},
		{name: "source long", mutate: func(v *CreateInput) { v.ManualSourceReference = strings.Repeat("界", 201) }, field: "manual_source_reference"},
		{name: "external long", mutate: func(v *CreateInput) { v.ExternalTransactionReference = strings.Repeat("A", 101) }, field: "external_transaction_reference"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validCreateInput()
			test.mutate(&input)
			repository := &repositoryStub{}
			_, err := testService(repository).Create(context.Background(), input)
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Field != test.field {
				t.Fatalf("Create() error = %v", err)
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

func TestPayloadHashAndExternalScopeUseCaseInsensitiveReferenceKeys(t *testing.T) {
	first, err := normalizeCreateInput(validCreateInput())
	if err != nil {
		t.Fatal(err)
	}
	secondInput := validCreateInput()
	secondInput.Type = " RECEIPT "
	secondInput.Description = " 九月服务费收款 "
	secondInput.Amount = "000100.500000"
	secondInput.Currency = " cny "
	secondInput.AccountReference = " 基本户尾号 1234 "
	secondInput.ExternalTransactionReference = " bank-txn-001 "
	second, err := normalizeCreateInput(secondInput)
	if err != nil {
		t.Fatal(err)
	}
	firstHash, _ := createPayloadHash(first)
	secondHash, _ := createPayloadHash(second)
	if !bytes.Equal(firstHash, secondHash) {
		t.Fatalf("normalized hashes differ: %x %x", firstHash, secondHash)
	}
	firstValue := Transaction{
		LedgerOwnerPartyID: testOwnerPartyID, AccountReference: "基本户 ABC",
		SourceKind: SourceKindManual, ExternalTransactionReference: "Bank-001",
	}
	secondValue := firstValue
	secondValue.AccountReference = "基本户 abc"
	secondValue.ExternalTransactionReference = "bank-001"
	firstLock, _ := externalReferenceLockKey(firstValue)
	secondLock, _ := externalReferenceLockKey(secondValue)
	if firstLock == "" || firstLock != secondLock {
		t.Fatalf("external lock keys = %q %q", firstLock, secondLock)
	}
	secondValue.LedgerOwnerPartyID = "77777777-7777-4777-8777-777777777777"
	changedLock, _ := externalReferenceLockKey(secondValue)
	if changedLock == firstLock {
		t.Fatal("ledger owner is absent from external reference scope")
	}
	second.Actor.ID = "88888888-8888-4888-8888-888888888888"
	changedActorHash, _ := createPayloadHash(second)
	if bytes.Equal(firstHash, changedActorHash) {
		t.Fatal("actor identity is absent from payload hash")
	}
}

func TestCreateReturnsReplayAndPreservesTypedErrors(t *testing.T) {
	replayed := Transaction{ID: "88888888-8888-4888-8888-888888888888", Description: "原资金事实"}
	repository := &repositoryStub{createResult: replayed, replayed: true}
	result, err := testService(repository).Create(context.Background(), validCreateInput())
	if err != nil || !result.Replayed || result.Transaction.ID != replayed.ID {
		t.Fatalf("Create() = %#v, %v", result, err)
	}
	for _, typedError := range []error{
		&IdempotencyConflictError{CommandID: testCommandID},
		&ExternalReferenceConflictError{ExistingTransactionID: testTransactionID},
		&PartyNotFoundError{PartyID: testCounterpartyID},
	} {
		repository = &repositoryStub{createErr: typedError}
		_, err = testService(repository).Create(context.Background(), validCreateInput())
		if !errors.Is(err, typedError) {
			t.Fatalf("Create() error = %v, want wrapped %T", err, typedError)
		}
	}
}

func TestListAndGet(t *testing.T) {
	want := Transaction{ID: testTransactionID, Description: "资金事实"}
	repository := &repositoryStub{listResult: []Transaction{want}, getResult: want, getFound: true}
	service := NewService(repository)
	listed, err := service.List(context.Background())
	if err != nil || len(listed) != 1 || listed[0].ID != want.ID {
		t.Fatalf("List() = %#v, %v", listed, err)
	}
	got, found, err := service.Get(context.Background(), " 55555555-5555-4555-8555-555555555555 ")
	if err != nil || !found || got.ID != want.ID || repository.getID != testTransactionID {
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
	if _, _, err := service.Get(context.Background(), testTransactionID); !errors.Is(err, sentinel) {
		t.Fatalf("Get() error = %v", err)
	}
}

func TestNewCommandID(t *testing.T) {
	id, err := NewCommandID()
	if err != nil || !uuidPattern.MatchString(id) {
		t.Fatalf("NewCommandID() = %q, %v", id, err)
	}
}
