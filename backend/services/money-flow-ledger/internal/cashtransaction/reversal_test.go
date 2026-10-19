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

const testCashReversalID = "77777777-7777-4777-8777-777777777777"

func validReverseInput() ReverseInput {
	return ReverseInput{
		CommandID:         testCommandID,
		CashTransactionID: testTransactionID,
		Reason:            "原资金登记错误",
		Actor:             testActor(),
	}
}

func reversalService(repository Repository) *Service {
	service := NewService(repository)
	ids := []string{testCashReversalID, testAuditID}
	service.newID = func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}
	service.now = func() time.Time {
		return time.Date(2026, 9, 19, 20, 0, 0, 0, time.UTC)
	}
	return service
}

func TestReverseNormalizesIntentAndBuildsAudit(t *testing.T) {
	repository := &repositoryStub{}
	input := validReverseInput()
	input.CommandID = " 11111111-1111-4111-8111-111111111111 "
	input.CashTransactionID = " 55555555-5555-4555-8555-555555555555 "
	input.Reason = "  原资金登记错误  "

	result, err := reversalService(repository).Reverse(context.Background(), input)
	if err != nil {
		t.Fatalf("Reverse() error = %v", err)
	}
	value := result.Reversal
	if result.Replayed || value.ID != testCashReversalID || value.CashTransactionID != testTransactionID ||
		value.SourceKind != SourceKindManual || value.Reason != "原资金登记错误" || value.ReversedBy != testActor() ||
		!value.ReversedAt.Equal(time.Date(2026, 9, 19, 20, 0, 0, 0, time.UTC)) {
		t.Fatalf("reversal = %#v replayed=%v", value, result.Replayed)
	}
	if repository.reverseCommand.CommandID != testCommandID || len(repository.reverseCommand.PayloadHash) != 32 ||
		repository.reverseCommand.Actor != testActor() {
		t.Fatalf("command = %#v", repository.reverseCommand)
	}
	entry := repository.reverseAudit
	if entry.ID != testAuditID || entry.ObjectType != audit.ObjectCashReversal || entry.ObjectID != testCashReversalID ||
		entry.Action != AuditActionReversed || entry.CommandID != testCommandID || entry.Actor != testActor() ||
		entry.Summary == "" || entry.Details["cash_transaction_id"] != testTransactionID ||
		entry.Details["reason"] != "原资金登记错误" || entry.Details["source_kind"] != SourceKindManual {
		t.Fatalf("audit = %#v", entry)
	}
}

func TestReverseRejectsInvalidInputBeforeRepository(t *testing.T) {
	for _, test := range []struct {
		name  string
		apply func(*ReverseInput)
		field string
	}{
		{name: "command", apply: func(input *ReverseInput) { input.CommandID = "bad" }, field: "command_id"},
		{name: "cash transaction", apply: func(input *ReverseInput) { input.CashTransactionID = "bad" }, field: "cash_transaction_id"},
		{name: "empty reason", apply: func(input *ReverseInput) { input.Reason = "  " }, field: "reason"},
		{name: "long reason", apply: func(input *ReverseInput) { input.Reason = strings.Repeat("界", 201) }, field: "reason"},
		{name: "actor", apply: func(input *ReverseInput) { input.Actor = audit.ActorRef{} }, field: "actor"},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &repositoryStub{}
			input := validReverseInput()
			test.apply(&input)
			_, err := reversalService(repository).Reverse(context.Background(), input)
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Field != test.field || repository.reversedValue.ID != "" {
				t.Fatalf("Reverse() error=%v repository=%#v", err, repository.reversedValue)
			}
		})
	}
}

func TestReversePayloadHashUsesNormalizedIntentAndActorIdentity(t *testing.T) {
	first, err := normalizeReverseInput(validReverseInput())
	if err != nil {
		t.Fatal(err)
	}
	secondInput := validReverseInput()
	secondInput.CommandID = " 11111111-1111-4111-8111-111111111111 "
	secondInput.CashTransactionID = " 55555555-5555-4555-8555-555555555555 "
	secondInput.Reason = " 原资金登记错误 "
	second, err := normalizeReverseInput(secondInput)
	if err != nil {
		t.Fatal(err)
	}
	firstHash, _ := reversePayloadHash(first)
	secondHash, _ := reversePayloadHash(second)
	if !bytes.Equal(firstHash, secondHash) {
		t.Fatalf("normalized hashes differ: %x %x", firstHash, secondHash)
	}
	second.Actor.ID = "88888888-8888-4888-8888-888888888888"
	changedActorHash, _ := reversePayloadHash(second)
	if bytes.Equal(firstHash, changedActorHash) {
		t.Fatal("actor identity is absent from reversal payload hash")
	}
	second = first
	second.Reason = "另一原因"
	changedReasonHash, _ := reversePayloadHash(second)
	if bytes.Equal(firstHash, changedReasonHash) {
		t.Fatal("reason is absent from reversal payload hash")
	}
}

func TestReverseReturnsReplayAndPreservesTypedErrors(t *testing.T) {
	replayed := Reversal{ID: "99999999-9999-4999-8999-999999999999", CashTransactionID: testTransactionID}
	repository := &repositoryStub{reverseResult: replayed, reverseReplayed: true}
	result, err := reversalService(repository).Reverse(context.Background(), validReverseInput())
	if err != nil || !result.Replayed || result.Reversal.ID != replayed.ID {
		t.Fatalf("Reverse() = %#v, %v", result, err)
	}
	for _, typedError := range []error{
		&ReversalIdempotencyConflictError{CommandID: testCommandID},
		&TransactionNotFoundError{CashTransactionID: testTransactionID},
		&TransactionAlreadyReversedError{CashTransactionID: testTransactionID},
		&TransactionAllocatedConflictError{CashTransactionID: testTransactionID, AllocatedAmount: "10"},
	} {
		repository = &repositoryStub{reverseErr: typedError}
		_, err = reversalService(repository).Reverse(context.Background(), validReverseInput())
		if !errors.Is(err, typedError) {
			t.Fatalf("Reverse() error = %v, want wrapped %T", err, typedError)
		}
	}
}
