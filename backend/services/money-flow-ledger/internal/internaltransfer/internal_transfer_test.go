package internaltransfer

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
	testCommandID  = "11111111-1111-4111-8111-111111111111"
	testActorID    = "22222222-2222-4222-8222-222222222222"
	testOwnerID    = "33333333-3333-4333-8333-333333333333"
	testTransferID = "44444444-4444-4444-8444-444444444444"
	testAuditID    = "55555555-5555-4555-8555-555555555555"
)

type repositoryStub struct {
	createdValue   Transfer
	createdCommand CreateCommand
	createdAudit   audit.Entry
	createResult   Transfer
	replayed       bool
	createErr      error
	listResult     []Transfer
	listErr        error
	getResult      Transfer
	getFound       bool
	getErr         error
	getID          string
	getCalls       int
}

func (r *repositoryStub) Create(_ context.Context, value Transfer, command CreateCommand, entry audit.Entry) (Transfer, bool, error) {
	r.createdValue = value
	r.createdCommand = command
	r.createdAudit = entry
	if r.createErr != nil {
		return Transfer{}, false, r.createErr
	}
	if r.createResult.ID != "" {
		return r.createResult, r.replayed, nil
	}
	return value, r.replayed, nil
}

func (r *repositoryStub) List(context.Context) ([]Transfer, error) {
	return r.listResult, r.listErr
}

func (r *repositoryStub) Get(_ context.Context, id string) (Transfer, bool, error) {
	r.getCalls++
	r.getID = id
	return r.getResult, r.getFound, r.getErr
}

func testActor() audit.ActorRef {
	return audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: testActorID, DisplayName: "资金管理员"}
}

func validCreateInput() CreateInput {
	return CreateInput{
		CommandID:                   testCommandID,
		LedgerOwnerPartyID:          testOwnerID,
		Description:                 "基本户向备用金账户转账",
		Amount:                      "100.5",
		Currency:                    "CNY",
		OccurredOn:                  "2026-09-19",
		SourceAccountReference:      "基本户尾号 1234",
		DestinationAccountReference: "备用金账户",
		ManualSourceReference:       "银行回单截图 TRANSFER-001",
		ExternalTransferReference:   "BANK-TRANSFER-001",
		Actor:                       testActor(),
	}
}

func testService(repository Repository) *Service {
	service := NewService(repository)
	ids := []string{testTransferID, testAuditID}
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

func TestCreateNormalizesTransferAndBuildsNonOperatingAudit(t *testing.T) {
	repository := &repositoryStub{}
	input := validCreateInput()
	input.CommandID = " " + testCommandID + " "
	input.LedgerOwnerPartyID = " " + testOwnerID + " "
	input.Description = "  基本户向备用金账户转账  "
	input.Amount = "000100.500000"
	input.Currency = " cny "
	input.SourceAccountReference = "  基本户尾号 1234  "
	input.DestinationAccountReference = "  备用金账户  "
	input.ManualSourceReference = "  银行回单截图 TRANSFER-001  "
	input.ExternalTransferReference = "  BANK-TRANSFER-001  "

	result, err := testService(repository).Create(context.Background(), input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	value := result.Transfer
	if result.Replayed || value.ID != testTransferID || value.LedgerOwner.PartyID != testOwnerID ||
		value.Description != "基本户向备用金账户转账" || value.Amount != "100.5" ||
		value.Currency != "CNY" || value.OccurredOn != "2026-09-19" ||
		value.SourceAccountReference != "基本户尾号 1234" || value.DestinationAccountReference != "备用金账户" ||
		value.SourceKind != SourceKindManual || value.ManualSourceReference != "银行回单截图 TRANSFER-001" ||
		value.ExternalTransferReference != "BANK-TRANSFER-001" || value.Status != StatusRecorded ||
		value.RecordedBy != testActor() {
		t.Fatalf("transfer = %#v", value)
	}
	if repository.createdCommand.CommandID != testCommandID || len(repository.createdCommand.PayloadHash) != 32 ||
		repository.createdCommand.Actor != testActor() {
		t.Fatalf("command = %#v", repository.createdCommand)
	}
	entry := repository.createdAudit
	if entry.ID != testAuditID || entry.ObjectType != audit.ObjectInternalTransfer || entry.ObjectID != testTransferID ||
		entry.Action != AuditActionRecorded || entry.CommandID != testCommandID || entry.Actor != testActor() {
		t.Fatalf("audit = %#v", entry)
	}
	if entry.Details["ledger_owner_party_id"] != testOwnerID || entry.Details["amount"] != "100.5" ||
		entry.Details["source_account_reference"] != "基本户尾号 1234" ||
		entry.Details["destination_account_reference"] != "备用金账户" ||
		entry.Details["operating_cash_flow_classification"] != "excluded_internal_transfer" ||
		entry.Details["resulting_status"] != StatusRecorded {
		t.Fatalf("audit details = %#v", entry.Details)
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
		{name: "owner", mutate: func(v *CreateInput) { v.LedgerOwnerPartyID = "bad" }, field: "ledger_owner_party_id"},
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
		{name: "source account empty", mutate: func(v *CreateInput) { v.SourceAccountReference = " " }, field: "source_account_reference"},
		{name: "source account long", mutate: func(v *CreateInput) { v.SourceAccountReference = strings.Repeat("界", 101) }, field: "source_account_reference"},
		{name: "destination account empty", mutate: func(v *CreateInput) { v.DestinationAccountReference = " " }, field: "destination_account_reference"},
		{name: "destination account long", mutate: func(v *CreateInput) { v.DestinationAccountReference = strings.Repeat("界", 101) }, field: "destination_account_reference"},
		{name: "same account exact", mutate: func(v *CreateInput) { v.DestinationAccountReference = v.SourceAccountReference }, field: "destination_account_reference"},
		{name: "same account case", mutate: func(v *CreateInput) {
			v.SourceAccountReference = "Account A"
			v.DestinationAccountReference = "account a"
		}, field: "destination_account_reference"},
		{name: "source empty", mutate: func(v *CreateInput) { v.ManualSourceReference = " " }, field: "manual_source_reference"},
		{name: "source long", mutate: func(v *CreateInput) { v.ManualSourceReference = strings.Repeat("界", 201) }, field: "manual_source_reference"},
		{name: "external long", mutate: func(v *CreateInput) { v.ExternalTransferReference = strings.Repeat("A", 101) }, field: "external_transfer_reference"},
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

func TestPayloadHashAndExternalScopeUseCaseInsensitiveKeys(t *testing.T) {
	first, err := normalizeCreateInput(validCreateInput())
	if err != nil {
		t.Fatal(err)
	}
	second := first
	second.SourceAccountReference = strings.ToUpper(first.SourceAccountReference)
	second.DestinationAccountReference = strings.ToUpper(first.DestinationAccountReference)
	second.ExternalTransferReference = strings.ToLower(first.ExternalTransferReference)
	firstHash, err := createPayloadHash(first)
	if err != nil {
		t.Fatal(err)
	}
	secondHash, err := createPayloadHash(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstHash, secondHash) {
		t.Fatalf("case-only changes produced different payload hashes")
	}

	firstValue := Transfer{LedgerOwner: PartySnapshot{PartyID: testOwnerID}, SourceAccountReference: first.SourceAccountReference, DestinationAccountReference: first.DestinationAccountReference, SourceKind: SourceKindManual, ExternalTransferReference: first.ExternalTransferReference}
	secondValue := firstValue
	secondValue.SourceAccountReference = second.SourceAccountReference
	secondValue.DestinationAccountReference = second.DestinationAccountReference
	secondValue.ExternalTransferReference = second.ExternalTransferReference
	firstKey, err := externalReferenceLockKey(firstValue)
	if err != nil {
		t.Fatal(err)
	}
	secondKey, err := externalReferenceLockKey(secondValue)
	if err != nil {
		t.Fatal(err)
	}
	if firstKey == "" || firstKey != secondKey {
		t.Fatalf("lock keys first=%q second=%q", firstKey, secondKey)
	}
	secondValue.DestinationAccountReference = "另一个账户"
	thirdKey, err := externalReferenceLockKey(secondValue)
	if err != nil {
		t.Fatal(err)
	}
	if thirdKey == firstKey {
		t.Fatal("different account direction must have a different external reference scope")
	}
	secondValue.ExternalTransferReference = ""
	emptyKey, err := externalReferenceLockKey(secondValue)
	if err != nil || emptyKey != "" {
		t.Fatalf("empty key = %q, %v", emptyKey, err)
	}
}

func TestServicePropagatesReplayAndRepositoryErrors(t *testing.T) {
	want := Transfer{ID: "66666666-6666-4666-8666-666666666666"}
	repository := &repositoryStub{createResult: want, replayed: true}
	result, err := testService(repository).Create(context.Background(), validCreateInput())
	if err != nil || !result.Replayed || result.Transfer != want {
		t.Fatalf("Create() = %#v, %v", result, err)
	}

	repository = &repositoryStub{createErr: &IdempotencyConflictError{CommandID: testCommandID}}
	_, err = testService(repository).Create(context.Background(), validCreateInput())
	var conflict *IdempotencyConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("Create() error = %v", err)
	}

	repository = &repositoryStub{listErr: errors.New("database unavailable")}
	if _, err := NewService(repository).List(context.Background()); err == nil || !strings.Contains(err.Error(), "list internal transfers") {
		t.Fatalf("List() error = %v", err)
	}

	repository = &repositoryStub{getErr: errors.New("database unavailable")}
	if _, _, err := NewService(repository).Get(context.Background(), testTransferID); err == nil || !strings.Contains(err.Error(), "get internal transfer") {
		t.Fatalf("Get() error = %v", err)
	}
}

func TestGetValidatesAndNormalizesID(t *testing.T) {
	repository := &repositoryStub{getFound: true, getResult: Transfer{ID: testTransferID}}
	value, found, err := NewService(repository).Get(context.Background(), " "+strings.ToUpper(testTransferID)+" ")
	if err != nil || !found || value.ID != testTransferID || repository.getID != testTransferID || repository.getCalls != 1 {
		t.Fatalf("Get() = %#v, %v, %v; repository = %#v", value, found, err, repository)
	}
	_, found, err = NewService(repository).Get(context.Background(), "bad")
	var validationError *ValidationError
	if found || !errors.As(err, &validationError) || validationError.Field != "internal_transfer_id" || repository.getCalls != 1 {
		t.Fatalf("invalid Get() = %v, %v", found, err)
	}
}

func TestNewCommandIDReturnsUUID(t *testing.T) {
	id, err := NewCommandID()
	if err != nil || !uuidPattern.MatchString(id) {
		t.Fatalf("NewCommandID() = %q, %v", id, err)
	}
}
