package obligation

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

const (
	testClosureID      = "77777777-7777-4777-8777-777777777779"
	testClosureAuditID = "88888888-8888-4888-8888-888888888890"
)

func validCloseInput() CloseInput {
	return CloseInput{
		CommandID:               testCommandID,
		ObligationID:            testObligationID,
		ExpectedCurrentRevision: 2,
		ExpectedBalanceSequence: 3,
		Reason:                  "余额已核对，结束常规处理",
		Actor:                   testActor(),
	}
}

func closureService(repository Repository) *Service {
	service := NewService(repository)
	ids := []string{testClosureID, testClosureAuditID}
	service.newID = func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}
	service.now = func() time.Time {
		return time.Date(2026, 9, 19, 18, 0, 0, 0, time.UTC)
	}
	return service
}

func TestCloseNormalizesIntentAndBuildsTrustedAudit(t *testing.T) {
	repository := &repositoryStub{
		closureResult: Obligation{ID: testObligationID, Status: StatusClosed, CurrentRevision: 2, BalanceSequence: 3},
		closedClosure: Closure{ID: testClosureID, ObligationID: testObligationID, ObligationRevision: 2, BalanceSequence: 3},
	}
	result, err := closureService(repository).Close(context.Background(), CloseInput{
		CommandID:               " " + testCommandID + " ",
		ObligationID:            " " + testObligationID + " ",
		ExpectedCurrentRevision: 2,
		ExpectedBalanceSequence: 3,
		Reason:                  "  余额已核对，结束常规处理  ",
		Actor:                   testActor(),
	})
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if result.Replayed || result.Obligation.Status != StatusClosed || result.Closure.ID != testClosureID {
		t.Fatalf("Close() result = %#v", result)
	}
	value := repository.closureValue
	closedAt := time.Date(2026, 9, 19, 18, 0, 0, 0, time.UTC)
	if value.ID != testClosureID || value.ObligationID != testObligationID || value.ObligationRevision != 2 ||
		value.BalanceSequence != 3 || value.Reason != "余额已核对，结束常规处理" || value.SourceKind != SourceKindManual ||
		value.ClosedBy != testActor() || !value.ClosedAt.Equal(closedAt) || value.RecordedEffectiveAmount != "" ||
		value.NetAllocatedAmount != "" || value.RemittedAmount != "" || value.Currency != "" || value.SettlementBasis != "" {
		t.Fatalf("closure value = %#v", value)
	}
	if repository.closureCommand.CommandID != testCommandID || len(repository.closureCommand.PayloadHash) != 32 ||
		repository.closureCommand.Actor != testActor() || !repository.closureCommand.ClosedAt.Equal(closedAt) {
		t.Fatalf("closure command = %#v", repository.closureCommand)
	}
	entry := repository.closureAudit
	if entry.ID != testClosureAuditID || entry.ObjectType != audit.ObjectObligation || entry.ObjectID != testObligationID ||
		entry.Action != AuditActionClosed || entry.CommandID != testCommandID || entry.Actor != testActor() ||
		entry.Details["expected_current_revision"] != int64(2) || entry.Details["expected_balance_sequence"] != int64(3) ||
		entry.Details["closure_reason"] != "余额已核对，结束常规处理" {
		t.Fatalf("closure audit = %#v", entry)
	}
}

func TestCloseValidationDoesNotCallRepository(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CloseInput)
		field  string
	}{
		{name: "command", mutate: func(v *CloseInput) { v.CommandID = "bad" }, field: "command_id"},
		{name: "obligation", mutate: func(v *CloseInput) { v.ObligationID = "bad" }, field: "obligation_id"},
		{name: "revision", mutate: func(v *CloseInput) { v.ExpectedCurrentRevision = 0 }, field: "expected_current_revision"},
		{name: "sequence", mutate: func(v *CloseInput) { v.ExpectedBalanceSequence = 0 }, field: "expected_balance_sequence"},
		{name: "empty reason", mutate: func(v *CloseInput) { v.Reason = " " }, field: "closure_reason"},
		{name: "long reason", mutate: func(v *CloseInput) { v.Reason = string(make([]rune, 201)) }, field: "closure_reason"},
		{name: "actor", mutate: func(v *CloseInput) { v.Actor.ID = "bad" }, field: "actor"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validCloseInput()
			test.mutate(&input)
			repository := &repositoryStub{}
			_, err := NewService(repository).Close(context.Background(), input)
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Field != test.field {
				t.Fatalf("Close() error = %v", err)
			}
			if repository.closureValue.ID != "" {
				t.Fatalf("repository called with %#v", repository.closureValue)
			}
		})
	}
}

func TestClosurePayloadHashUsesNormalizedIntentAndTrustedActor(t *testing.T) {
	first, err := normalizeCloseInput(validCloseInput())
	if err != nil {
		t.Fatal(err)
	}
	secondInput := validCloseInput()
	secondInput.CommandID = " " + secondInput.CommandID + " "
	secondInput.ObligationID = " " + secondInput.ObligationID + " "
	secondInput.Reason = " 余额已核对，结束常规处理 "
	second, err := normalizeCloseInput(secondInput)
	if err != nil {
		t.Fatal(err)
	}
	firstHash, _ := closurePayloadHash(first)
	secondHash, _ := closurePayloadHash(second)
	if !bytes.Equal(firstHash, secondHash) {
		t.Fatalf("normalized hashes differ: %x %x", firstHash, secondHash)
	}
	changes := []func(*CloseInput){
		func(v *CloseInput) { v.ExpectedCurrentRevision++ },
		func(v *CloseInput) { v.ExpectedBalanceSequence++ },
		func(v *CloseInput) { v.Reason = "其他原因" },
		func(v *CloseInput) { v.Actor.ID = "99999999-9999-4999-8999-999999999999" },
	}
	for _, change := range changes {
		changed := first
		change(&changed)
		changedHash, _ := closurePayloadHash(changed)
		if bytes.Equal(firstHash, changedHash) {
			t.Fatalf("changed intent is absent from payload hash: %#v", changed)
		}
	}
}

func TestCloseReturnsReplayAndPreservesTypedErrors(t *testing.T) {
	repository := &repositoryStub{
		closureResult:   Obligation{ID: testObligationID, Status: StatusClosed},
		closedClosure:   Closure{ID: testClosureID, SettlementBasis: ClosureBasisMixed},
		closureReplayed: true,
	}
	result, err := closureService(repository).Close(context.Background(), validCloseInput())
	if err != nil || !result.Replayed || result.Closure.SettlementBasis != ClosureBasisMixed {
		t.Fatalf("Close() = %#v, %v", result, err)
	}
	for _, typedError := range []error{
		&ClosureIdempotencyConflictError{CommandID: testCommandID},
		&ObligationNotFoundError{ObligationID: testObligationID},
		&ClosureStateConflictError{Status: StatusClosed},
		&CurrentRevisionConflictError{Expected: 1, Actual: 2},
		&BalanceSequenceConflictError{Expected: 2, Actual: 3},
		&ClosureBalanceConflictError{Outstanding: "1"},
	} {
		repository = &repositoryStub{closureErr: typedError}
		_, err = closureService(repository).Close(context.Background(), validCloseInput())
		if !errors.Is(err, typedError) {
			t.Fatalf("Close() error = %v, want wrapped %T", err, typedError)
		}
	}
}

func TestClosureBasisLabels(t *testing.T) {
	for basis, want := range map[ClosureBasis]string{
		ClosureBasisAllocated: "全部由资金分配归零",
		ClosureBasisRemitted:  "全部由减免归零",
		ClosureBasisMixed:     "资金分配与减免组合归零",
	} {
		if got := basis.Label(); got != want {
			t.Fatalf("%s.Label() = %q", basis, got)
		}
	}
}
