package allocation

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

const testReversalID = "77777777-7777-4777-8777-777777777777"

func validReverseInput() ReverseInput {
	return ReverseInput{
		CommandID:    testCommandID,
		AllocationID: testAllocationID,
		Reason:       "原分配选择了错误义务",
		Actor:        testActor(),
	}
}

func testReversalService(repository Repository) *Service {
	service := NewService(repository)
	ids := []string{testReversalID, testAuditID}
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

func TestReverseNormalizesInputAndBuildsAudit(t *testing.T) {
	repository := &repositoryStub{}
	input := validReverseInput()
	input.CommandID = " " + strings.ToUpper(testCommandID) + " "
	input.AllocationID = " " + strings.ToUpper(testAllocationID) + " "
	input.Reason = "  原分配选择了错误义务  "

	result, err := testReversalService(repository).Reverse(context.Background(), input)
	if err != nil {
		t.Fatalf("Reverse() error = %v", err)
	}
	value := result.Reversal
	if result.Replayed || value.ID != testReversalID || value.AllocationID != testAllocationID ||
		value.SourceKind != SourceKindManual || value.Reason != "原分配选择了错误义务" ||
		value.ReversedBy != testActor() || !value.ReversedAt.Equal(time.Date(2026, 9, 19, 20, 0, 0, 0, time.UTC)) ||
		value.Amount != "" || value.Currency != "" || value.CashTransactionID != "" || value.ObligationID != "" {
		t.Fatalf("reversal = %#v", value)
	}
	if repository.reversedCommand.CommandID != testCommandID || len(repository.reversedCommand.PayloadHash) != 32 ||
		repository.reversedCommand.Actor != testActor() {
		t.Fatalf("command = %#v", repository.reversedCommand)
	}
	entry := repository.reversedAudit
	if entry.ID != testAuditID || entry.ObjectType != audit.ObjectAllocationReversal || entry.ObjectID != testReversalID ||
		entry.Action != AuditActionReversed || entry.CommandID != testCommandID || entry.Actor != testActor() {
		t.Fatalf("audit = %#v", entry)
	}
	if entry.Details["allocation_id"] != testAllocationID || entry.Details["source_kind"] != SourceKindManual ||
		entry.Details["reason"] != "原分配选择了错误义务" {
		t.Fatalf("audit details = %#v", entry.Details)
	}
}

func TestReverseValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ReverseInput)
		field  string
	}{
		{name: "command", mutate: func(v *ReverseInput) { v.CommandID = "bad" }, field: "command_id"},
		{name: "allocation", mutate: func(v *ReverseInput) { v.AllocationID = "bad" }, field: "allocation_id"},
		{name: "reason empty", mutate: func(v *ReverseInput) { v.Reason = " " }, field: "reason"},
		{name: "reason long", mutate: func(v *ReverseInput) { v.Reason = strings.Repeat("界", 201) }, field: "reason"},
		{name: "actor", mutate: func(v *ReverseInput) { v.Actor.ID = "bad" }, field: "actor"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validReverseInput()
			test.mutate(&input)
			repository := &repositoryStub{}
			_, err := testReversalService(repository).Reverse(context.Background(), input)
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Field != test.field {
				t.Fatalf("Reverse() error = %v", err)
			}
			if repository.reversedValue.ID != "" {
				t.Fatalf("repository called with %#v", repository.reversedValue)
			}
		})
	}
}

func TestReversePayloadHashUsesNormalizedIntentAndTrustedActor(t *testing.T) {
	first, err := normalizeReverseInput(validReverseInput())
	if err != nil {
		t.Fatal(err)
	}
	secondInput := validReverseInput()
	secondInput.CommandID = " " + strings.ToUpper(testCommandID) + " "
	secondInput.AllocationID = " " + strings.ToUpper(testAllocationID) + " "
	secondInput.Reason = "  原分配选择了错误义务  "
	secondInput.Actor.Kind = " LOCAL_USER "
	secondInput.Actor.ID = " " + strings.ToUpper(testActorID) + " "
	second, err := normalizeReverseInput(secondInput)
	if err != nil {
		t.Fatal(err)
	}
	firstHash, err := reversePayloadHash(first)
	if err != nil {
		t.Fatal(err)
	}
	secondHash, err := reversePayloadHash(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstHash, secondHash) {
		t.Fatalf("equivalent normalized inputs produced different hashes")
	}
	second.Reason = "另一原因"
	changedHash, err := reversePayloadHash(second)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(firstHash, changedHash) {
		t.Fatal("changed reason produced equal hash")
	}
}

func TestReversePropagatesReplayAndTypedErrors(t *testing.T) {
	want := Reversal{ID: testReversalID, AllocationID: testAllocationID, Amount: "60.5", Currency: "CNY"}
	repository := &repositoryStub{reverseResult: want, reverseReplayed: true}
	result, err := testReversalService(repository).Reverse(context.Background(), validReverseInput())
	if err != nil || !result.Replayed || result.Reversal != want {
		t.Fatalf("Reverse() = %#v, %v", result, err)
	}

	for _, typedError := range []error{
		&ReversalIdempotencyConflictError{CommandID: testCommandID},
		&AllocationNotFoundError{AllocationID: testAllocationID},
		&AllocationAlreadyReversedError{AllocationID: testAllocationID, ReversalID: testReversalID},
	} {
		repository = &repositoryStub{reverseErr: typedError}
		_, err := testReversalService(repository).Reverse(context.Background(), validReverseInput())
		if err == nil || !strings.Contains(err.Error(), typedError.Error()) {
			t.Fatalf("Reverse() error = %v, want wrapped %v", err, typedError)
		}
	}
}
