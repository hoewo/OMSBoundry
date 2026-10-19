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
	testAdjustmentID      = "77777777-7777-4777-8777-777777777777"
	testAdjustmentAuditID = "88888888-8888-4888-8888-888888888888"
)

func validAdjustAmountInput() AdjustAmountInput {
	return AdjustAmountInput{
		CommandID:               testCommandID,
		ObligationID:            testObligationID,
		ExpectedCurrentRevision: 1,
		Type:                    string(AdjustmentTypeDecrease),
		Amount:                  "20.500000",
		Reason:                  "业务依据金额更正",
		Actor:                   testActor(),
	}
}

func adjustmentService(repository Repository) *Service {
	service := NewService(repository)
	ids := []string{testAdjustmentID, testAdjustmentAuditID}
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

func TestAdjustAmountNormalizesIntentAndBuildsTrustedAudit(t *testing.T) {
	repository := &repositoryStub{
		adjustResult: Obligation{ID: testObligationID, CurrentRevision: 2, EffectiveAmount: "80"},
		adjustmentResult: Adjustment{
			ID: testAdjustmentID, ObligationID: testObligationID,
			SourceRevision: 1, ResultingRevision: 2,
			Type: AdjustmentTypeDecrease, Amount: "20.5",
		},
		adjustRevision: Revision{ObligationID: testObligationID, Revision: 2, EffectiveAmount: "80"},
	}
	result, err := adjustmentService(repository).AdjustAmount(context.Background(), AdjustAmountInput{
		CommandID:               " " + testCommandID + " ",
		ObligationID:            " " + testObligationID + " ",
		ExpectedCurrentRevision: 1,
		Type:                    " DECREASE ",
		Amount:                  " 020.500000 ",
		Reason:                  "  业务依据金额更正  ",
		Actor:                   testActor(),
	})
	if err != nil {
		t.Fatalf("AdjustAmount() error = %v", err)
	}
	if result.Replayed || result.Obligation.CurrentRevision != 2 || result.Adjustment.ID != testAdjustmentID || result.Revision.Revision != 2 {
		t.Fatalf("AdjustAmount() result = %#v", result)
	}
	value := repository.adjustValue
	if value.ID != testAdjustmentID || value.ObligationID != testObligationID || value.SourceRevision != 1 ||
		value.Type != AdjustmentTypeDecrease || value.Amount != "20.5" || value.Reason != "业务依据金额更正" ||
		value.SourceKind != SourceKindManual || value.AdjustedBy != testActor() ||
		!value.AdjustedAt.Equal(time.Date(2026, 9, 19, 16, 0, 0, 0, time.UTC)) {
		t.Fatalf("adjustment value = %#v", value)
	}
	if repository.adjustCommand.CommandID != testCommandID || len(repository.adjustCommand.PayloadHash) != 32 ||
		repository.adjustCommand.Actor != testActor() || !repository.adjustCommand.AdjustedAt.Equal(value.AdjustedAt) {
		t.Fatalf("adjustment command = %#v", repository.adjustCommand)
	}
	entry := repository.adjustAudit
	if entry.ID != testAdjustmentAuditID || entry.ObjectType != audit.ObjectObligation || entry.ObjectID != testObligationID ||
		entry.Action != AuditActionAmountAdjusted || entry.CommandID != testCommandID || entry.Actor != testActor() ||
		entry.Details["expected_current_revision"] != int64(1) || entry.Details["adjustment_type"] != "decrease" ||
		entry.Details["adjustment_amount"] != "20.5" || entry.Details["adjustment_reason"] != "业务依据金额更正" {
		t.Fatalf("adjustment audit = %#v", entry)
	}
}

func TestAdjustAmountValidationDoesNotCallRepository(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*AdjustAmountInput)
		field  string
	}{
		{name: "command", mutate: func(v *AdjustAmountInput) { v.CommandID = "bad" }, field: "command_id"},
		{name: "obligation", mutate: func(v *AdjustAmountInput) { v.ObligationID = "bad" }, field: "obligation_id"},
		{name: "revision", mutate: func(v *AdjustAmountInput) { v.ExpectedCurrentRevision = 0 }, field: "expected_current_revision"},
		{name: "type", mutate: func(v *AdjustAmountInput) { v.Type = "replace" }, field: "adjustment_type"},
		{name: "empty amount", mutate: func(v *AdjustAmountInput) { v.Amount = "" }, field: "adjustment_amount"},
		{name: "zero amount", mutate: func(v *AdjustAmountInput) { v.Amount = "0.000000" }, field: "adjustment_amount"},
		{name: "negative amount", mutate: func(v *AdjustAmountInput) { v.Amount = "-1" }, field: "adjustment_amount"},
		{name: "exponent amount", mutate: func(v *AdjustAmountInput) { v.Amount = "1e3" }, field: "adjustment_amount"},
		{name: "amount precision", mutate: func(v *AdjustAmountInput) { v.Amount = "1234567890123456789" }, field: "adjustment_amount"},
		{name: "amount scale", mutate: func(v *AdjustAmountInput) { v.Amount = "1.0000001" }, field: "adjustment_amount"},
		{name: "empty reason", mutate: func(v *AdjustAmountInput) { v.Reason = "  " }, field: "adjustment_reason"},
		{name: "long reason", mutate: func(v *AdjustAmountInput) { v.Reason = string(make([]rune, 201)) }, field: "adjustment_reason"},
		{name: "actor", mutate: func(v *AdjustAmountInput) { v.Actor.ID = "bad" }, field: "actor"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validAdjustAmountInput()
			test.mutate(&input)
			repository := &repositoryStub{}
			_, err := NewService(repository).AdjustAmount(context.Background(), input)
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Field != test.field {
				t.Fatalf("AdjustAmount() error = %v", err)
			}
			if repository.adjustValue.ID != "" {
				t.Fatalf("repository called with %#v", repository.adjustValue)
			}
		})
	}
}

func TestAdjustmentPayloadHashUsesNormalizedIntentAndTrustedActor(t *testing.T) {
	first, err := normalizeAdjustAmountInput(validAdjustAmountInput())
	if err != nil {
		t.Fatal(err)
	}
	secondInput := validAdjustAmountInput()
	secondInput.CommandID = " " + secondInput.CommandID + " "
	secondInput.ObligationID = " " + secondInput.ObligationID + " "
	secondInput.Type = " DECREASE "
	secondInput.Amount = "020.500000"
	secondInput.Reason = " 业务依据金额更正 "
	second, err := normalizeAdjustAmountInput(secondInput)
	if err != nil {
		t.Fatal(err)
	}
	firstHash, _ := adjustmentPayloadHash(first)
	secondHash, _ := adjustmentPayloadHash(second)
	if !bytes.Equal(firstHash, secondHash) {
		t.Fatalf("normalized hashes differ: %x %x", firstHash, secondHash)
	}
	changes := []func(*AdjustAmountInput){
		func(v *AdjustAmountInput) { v.ExpectedCurrentRevision++ },
		func(v *AdjustAmountInput) { v.Type = string(AdjustmentTypeIncrease) },
		func(v *AdjustAmountInput) { v.Amount = "20.6" },
		func(v *AdjustAmountInput) { v.Reason = "其他原因" },
		func(v *AdjustAmountInput) { v.Actor.ID = "99999999-9999-4999-8999-999999999999" },
	}
	for _, change := range changes {
		changed := first
		change(&changed)
		changedHash, _ := adjustmentPayloadHash(changed)
		if bytes.Equal(firstHash, changedHash) {
			t.Fatalf("changed intent is absent from payload hash: %#v", changed)
		}
	}
}

func TestAdjustAmountReturnsReplayAndPreservesTypedErrors(t *testing.T) {
	repository := &repositoryStub{
		adjustResult:     Obligation{ID: testObligationID, CurrentRevision: 2},
		adjustmentResult: Adjustment{ID: testAdjustmentID, ResultingRevision: 2},
		adjustRevision:   Revision{ObligationID: testObligationID, Revision: 2},
		adjustReplayed:   true,
	}
	result, err := adjustmentService(repository).AdjustAmount(context.Background(), validAdjustAmountInput())
	if err != nil || !result.Replayed || result.Adjustment.ResultingRevision != 2 || result.Revision.Revision != 2 {
		t.Fatalf("AdjustAmount() = %#v, %v", result, err)
	}
	for _, typedError := range []error{
		&AmountAdjustmentIdempotencyConflictError{CommandID: testCommandID},
		&ObligationNotFoundError{ObligationID: testObligationID},
		&AmountAdjustmentStateConflictError{Status: StatusDraft},
		&CurrentRevisionConflictError{Expected: 1, Actual: 2},
		&AmountAdjustmentResultConflictError{Reason: "below_allocated"},
	} {
		repository = &repositoryStub{adjustErr: typedError}
		_, err = adjustmentService(repository).AdjustAmount(context.Background(), validAdjustAmountInput())
		if !errors.Is(err, typedError) {
			t.Fatalf("AdjustAmount() error = %v, want wrapped %T", err, typedError)
		}
	}
}

func TestCalculateAdjustedAmountIsExactAndBounded(t *testing.T) {
	tests := []struct {
		name           string
		current        string
		adjustment     string
		adjustmentType AdjustmentType
		want           string
		conflictReason string
	}{
		{name: "increase", current: "100.1", adjustment: "0.2", adjustmentType: AdjustmentTypeIncrease, want: "100.3"},
		{name: "decrease", current: "100", adjustment: "99.999999", adjustmentType: AdjustmentTypeDecrease, want: "0.000001"},
		{name: "zero", current: "100", adjustment: "100", adjustmentType: AdjustmentTypeDecrease, conflictReason: "non_positive"},
		{name: "negative", current: "100", adjustment: "101", adjustmentType: AdjustmentTypeDecrease, conflictReason: "non_positive"},
		{name: "overflow", current: "999999999999999999.999999", adjustment: "0.000001", adjustmentType: AdjustmentTypeIncrease, conflictReason: "out_of_range"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := calculateAdjustedAmount(test.current, test.adjustment, test.adjustmentType)
			if test.conflictReason == "" {
				if err != nil || got != test.want {
					t.Fatalf("calculateAdjustedAmount() = %q, %v; want %q", got, err, test.want)
				}
				return
			}
			var conflict *AmountAdjustmentResultConflictError
			if !errors.As(err, &conflict) || conflict.Reason != test.conflictReason {
				t.Fatalf("calculateAdjustedAmount() error = %v", err)
			}
		})
	}
}
