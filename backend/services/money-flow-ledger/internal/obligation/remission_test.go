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
	testRemissionID      = "77777777-7777-4777-8777-777777777778"
	testRemissionAuditID = "88888888-8888-4888-8888-888888888889"
)

func validRecordRemissionInput() RecordRemissionInput {
	return RecordRemissionInput{
		CommandID:               testCommandID,
		ObligationID:            testObligationID,
		ExpectedCurrentRevision: 2,
		Amount:                  "20.500000",
		Reason:                  "双方确认免除部分服务费",
		ManualSourceReference:   "减免确认函 RM-001",
		Actor:                   testActor(),
	}
}

func remissionService(repository Repository) *Service {
	service := NewService(repository)
	ids := []string{testRemissionID, testRemissionAuditID}
	service.newID = func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}
	service.now = func() time.Time {
		return time.Date(2026, 9, 19, 17, 0, 0, 0, time.UTC)
	}
	return service
}

func TestRecordRemissionNormalizesIntentAndBuildsTrustedAudit(t *testing.T) {
	repository := &repositoryStub{
		remissionResult:   Obligation{ID: testObligationID, CurrentRevision: 2, RemittedAmount: "20.5", OutstandingAmount: "79.5"},
		recordedRemission: Remission{ID: testRemissionID, ObligationID: testObligationID, ObligationRevision: 2, Amount: "20.5"},
	}
	result, err := remissionService(repository).RecordRemission(context.Background(), RecordRemissionInput{
		CommandID:               " " + testCommandID + " ",
		ObligationID:            " " + testObligationID + " ",
		ExpectedCurrentRevision: 2,
		Amount:                  " 020.500000 ",
		Reason:                  "  双方确认免除部分服务费  ",
		ManualSourceReference:   "  减免确认函 RM-001  ",
		Actor:                   testActor(),
	})
	if err != nil {
		t.Fatalf("RecordRemission() error = %v", err)
	}
	if result.Replayed || result.Obligation.RemittedAmount != "20.5" || result.Remission.ID != testRemissionID {
		t.Fatalf("RecordRemission() result = %#v", result)
	}
	value := repository.remissionValue
	if value.ID != testRemissionID || value.ObligationID != testObligationID || value.ObligationRevision != 2 ||
		value.Amount != "20.5" || value.Reason != "双方确认免除部分服务费" ||
		value.ManualSourceReference != "减免确认函 RM-001" || value.SourceKind != SourceKindManual ||
		value.RecordedBy != testActor() || !value.RecordedAt.Equal(time.Date(2026, 9, 19, 17, 0, 0, 0, time.UTC)) ||
		value.RecordedEffectiveAmount != "" || value.Currency != "" {
		t.Fatalf("remission value = %#v", value)
	}
	if repository.remissionCommand.CommandID != testCommandID || len(repository.remissionCommand.PayloadHash) != 32 ||
		repository.remissionCommand.Actor != testActor() || !repository.remissionCommand.RecordedAt.Equal(value.RecordedAt) {
		t.Fatalf("remission command = %#v", repository.remissionCommand)
	}
	entry := repository.remissionAudit
	if entry.ID != testRemissionAuditID || entry.ObjectType != audit.ObjectObligation || entry.ObjectID != testObligationID ||
		entry.Action != AuditActionRemissionRecorded || entry.CommandID != testCommandID || entry.Actor != testActor() ||
		entry.Details["expected_current_revision"] != int64(2) || entry.Details["remission_amount"] != "20.5" ||
		entry.Details["remission_reason"] != "双方确认免除部分服务费" ||
		entry.Details["manual_source_reference"] != "减免确认函 RM-001" {
		t.Fatalf("remission audit = %#v", entry)
	}
}

func TestRecordRemissionValidationDoesNotCallRepository(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*RecordRemissionInput)
		field  string
	}{
		{name: "command", mutate: func(v *RecordRemissionInput) { v.CommandID = "bad" }, field: "command_id"},
		{name: "obligation", mutate: func(v *RecordRemissionInput) { v.ObligationID = "bad" }, field: "obligation_id"},
		{name: "revision", mutate: func(v *RecordRemissionInput) { v.ExpectedCurrentRevision = 0 }, field: "expected_current_revision"},
		{name: "empty amount", mutate: func(v *RecordRemissionInput) { v.Amount = "" }, field: "remission_amount"},
		{name: "zero amount", mutate: func(v *RecordRemissionInput) { v.Amount = "0" }, field: "remission_amount"},
		{name: "negative amount", mutate: func(v *RecordRemissionInput) { v.Amount = "-1" }, field: "remission_amount"},
		{name: "amount scale", mutate: func(v *RecordRemissionInput) { v.Amount = "1.0000001" }, field: "remission_amount"},
		{name: "empty reason", mutate: func(v *RecordRemissionInput) { v.Reason = " " }, field: "remission_reason"},
		{name: "long reason", mutate: func(v *RecordRemissionInput) { v.Reason = string(make([]rune, 201)) }, field: "remission_reason"},
		{name: "empty source", mutate: func(v *RecordRemissionInput) { v.ManualSourceReference = " " }, field: "manual_source_reference"},
		{name: "long source", mutate: func(v *RecordRemissionInput) { v.ManualSourceReference = string(make([]rune, 201)) }, field: "manual_source_reference"},
		{name: "actor", mutate: func(v *RecordRemissionInput) { v.Actor.ID = "bad" }, field: "actor"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validRecordRemissionInput()
			test.mutate(&input)
			repository := &repositoryStub{}
			_, err := NewService(repository).RecordRemission(context.Background(), input)
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Field != test.field {
				t.Fatalf("RecordRemission() error = %v", err)
			}
			if repository.remissionValue.ID != "" {
				t.Fatalf("repository called with %#v", repository.remissionValue)
			}
		})
	}
}

func TestRemissionPayloadHashUsesNormalizedIntentAndTrustedActor(t *testing.T) {
	first, err := normalizeRecordRemissionInput(validRecordRemissionInput())
	if err != nil {
		t.Fatal(err)
	}
	secondInput := validRecordRemissionInput()
	secondInput.CommandID = " " + secondInput.CommandID + " "
	secondInput.ObligationID = " " + secondInput.ObligationID + " "
	secondInput.Amount = "020.500000"
	secondInput.Reason = " 双方确认免除部分服务费 "
	secondInput.ManualSourceReference = " 减免确认函 RM-001 "
	second, err := normalizeRecordRemissionInput(secondInput)
	if err != nil {
		t.Fatal(err)
	}
	firstHash, _ := remissionPayloadHash(first)
	secondHash, _ := remissionPayloadHash(second)
	if !bytes.Equal(firstHash, secondHash) {
		t.Fatalf("normalized hashes differ: %x %x", firstHash, secondHash)
	}
	changes := []func(*RecordRemissionInput){
		func(v *RecordRemissionInput) { v.ExpectedCurrentRevision++ },
		func(v *RecordRemissionInput) { v.Amount = "20.6" },
		func(v *RecordRemissionInput) { v.Reason = "其他原因" },
		func(v *RecordRemissionInput) { v.ManualSourceReference = "其他依据" },
		func(v *RecordRemissionInput) { v.Actor.ID = "99999999-9999-4999-8999-999999999999" },
	}
	for _, change := range changes {
		changed := first
		change(&changed)
		changedHash, _ := remissionPayloadHash(changed)
		if bytes.Equal(firstHash, changedHash) {
			t.Fatalf("changed intent is absent from payload hash: %#v", changed)
		}
	}
}

func TestRecordRemissionReturnsReplayAndPreservesTypedErrors(t *testing.T) {
	repository := &repositoryStub{
		remissionResult:   Obligation{ID: testObligationID, RemittedAmount: "20.5"},
		recordedRemission: Remission{ID: testRemissionID, Amount: "20.5"},
		remissionReplayed: true,
	}
	result, err := remissionService(repository).RecordRemission(context.Background(), validRecordRemissionInput())
	if err != nil || !result.Replayed || result.Remission.Amount != "20.5" {
		t.Fatalf("RecordRemission() = %#v, %v", result, err)
	}
	for _, typedError := range []error{
		&RemissionIdempotencyConflictError{CommandID: testCommandID},
		&ObligationNotFoundError{ObligationID: testObligationID},
		&RemissionStateConflictError{Status: StatusDraft},
		&CurrentRevisionConflictError{Expected: 1, Actual: 2},
		&RemissionBalanceConflictError{Requested: "20", Outstanding: "10"},
	} {
		repository = &repositoryStub{remissionErr: typedError}
		_, err = remissionService(repository).RecordRemission(context.Background(), validRecordRemissionInput())
		if !errors.Is(err, typedError) {
			t.Fatalf("RecordRemission() error = %v, want wrapped %T", err, typedError)
		}
	}
}
