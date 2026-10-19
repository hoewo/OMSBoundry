package contract

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func validUpdateDraftInput(t *testing.T) UpdateDraftInput {
	t.Helper()
	created := validInput(t)
	return UpdateDraftInput{
		CommandID:            created.CommandID,
		ContractID:           testContractID,
		ExpectedDraftVersion: 4,
		NumberNamespace:      created.NumberNamespace,
		ContractNumber:       created.ContractNumber,
		Title:                created.Title,
		Category:             created.Category,
		Currency:             created.Currency,
		TotalAmount:          created.TotalAmount,
		SignedOn:             created.SignedOn,
		EffectiveOn:          created.EffectiveOn,
		Actor:                testActor,
	}
}

func validCancelDraftInput(t *testing.T) CancelDraftInput {
	t.Helper()
	commandID, err := NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	return CancelDraftInput{
		CommandID:            commandID,
		ContractID:           testContractID,
		ExpectedDraftVersion: 5,
		Reason:               " 双方决定不再签署 ",
		Actor:                testActor,
	}
}

func TestUpdateDraftNormalizesAndPersists(t *testing.T) {
	repository := &repositoryStub{}
	result, err := NewService(repository).UpdateDraft(context.Background(), validUpdateDraftInput(t))
	if err != nil {
		t.Fatalf("UpdateDraft() error = %v", err)
	}
	if repository.receivedUpdate.Action != DraftChangeActionUpdate || repository.receivedUpdate.Title != "年度服务合同" {
		t.Fatalf("command = %#v", repository.receivedUpdate)
	}
	if repository.receivedUpdate.TotalAmount != "1000.5" || repository.receivedUpdate.Currency != "CNY" {
		t.Fatalf("normalized money = %#v", repository.receivedUpdate)
	}
	if repository.receivedUpdateDraft.ID != testContractID || repository.receivedUpdateDraft.NumberNamespace != "default" {
		t.Fatalf("candidate = %#v", repository.receivedUpdateDraft)
	}
	if result.DraftVersion != 5 || result.Status != StatusDraft || result.Replayed {
		t.Fatalf("result = %#v", result)
	}
}

func TestUpdateDraftValidation(t *testing.T) {
	longTitle := strings.Repeat("字", 201)
	tests := []struct {
		name   string
		mutate func(*UpdateDraftInput)
		field  string
	}{
		{name: "contract", mutate: func(v *UpdateDraftInput) { v.ContractID = "bad" }, field: "contract_id"},
		{name: "command", mutate: func(v *UpdateDraftInput) { v.CommandID = "bad" }, field: "command_id"},
		{name: "title", mutate: func(v *UpdateDraftInput) { v.Title = longTitle }, field: "title"},
		{name: "amount", mutate: func(v *UpdateDraftInput) { v.TotalAmount = "-1" }, field: "total_amount"},
		{name: "version", mutate: func(v *UpdateDraftInput) { v.ExpectedDraftVersion = 0 }, field: "expected_draft_version"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validUpdateDraftInput(t)
			test.mutate(&input)
			repository := &repositoryStub{}
			_, err := NewService(repository).UpdateDraft(context.Background(), input)
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Field != test.field {
				t.Fatalf("error = %v, want ValidationError field %q", err, test.field)
			}
			if repository.receivedUpdate.CommandID != "" {
				t.Fatalf("invalid input reached repository: %#v", repository.receivedUpdate)
			}
		})
	}
}

func TestUpdateDraftIdempotency(t *testing.T) {
	input := validUpdateDraftInput(t)
	normalized, err := normalizeUpdateDraftInput(input)
	if err != nil {
		t.Fatalf("normalizeUpdateDraftInput() error = %v", err)
	}
	existing := DraftChangeCommand{
		CommandID:             normalized.CommandID,
		ContractID:            normalized.ContractID,
		Action:                DraftChangeActionUpdate,
		ExpectedDraftVersion:  normalized.ExpectedDraftVersion,
		ResultingDraftVersion: normalized.ExpectedDraftVersion + 1,
		NumberNamespace:       normalized.NumberNamespace,
		ContractNumber:        normalized.ContractNumber,
		Title:                 normalized.Title,
		Category:              normalized.Category,
		Currency:              normalized.Currency,
		TotalAmount:           normalized.TotalAmount,
		SignedOn:              normalized.SignedOn,
		EffectiveOn:           normalized.EffectiveOn,
	}
	repository := &repositoryStub{updateCommand: existing}
	result, err := NewService(repository).UpdateDraft(context.Background(), input)
	if err != nil {
		t.Fatalf("UpdateDraft() replay error = %v", err)
	}
	if !result.Replayed || result.DraftVersion != 5 {
		t.Fatalf("result = %#v", result)
	}
	repository.updateCommand.Title = "另一个标题"
	_, err = NewService(repository).UpdateDraft(context.Background(), input)
	var conflict *IdempotencyConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("changed replay error = %v, want IdempotencyConflictError", err)
	}
}

func TestCancelDraftNormalizesAndPersists(t *testing.T) {
	repository := &repositoryStub{}
	result, err := NewService(repository).CancelDraft(context.Background(), validCancelDraftInput(t))
	if err != nil {
		t.Fatalf("CancelDraft() error = %v", err)
	}
	if repository.receivedCancel.Action != DraftChangeActionCancel || repository.receivedCancel.CancellationReason != "双方决定不再签署" {
		t.Fatalf("command = %#v", repository.receivedCancel)
	}
	if result.DraftVersion != 6 || result.Status != StatusCancelled || result.Replayed {
		t.Fatalf("result = %#v", result)
	}
}

func TestCancelDraftValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CancelDraftInput)
		field  string
	}{
		{name: "command", mutate: func(v *CancelDraftInput) { v.CommandID = "bad" }, field: "command_id"},
		{name: "contract", mutate: func(v *CancelDraftInput) { v.ContractID = "bad" }, field: "contract_id"},
		{name: "reason", mutate: func(v *CancelDraftInput) { v.Reason = " " }, field: "cancellation_reason"},
		{name: "long reason", mutate: func(v *CancelDraftInput) { v.Reason = strings.Repeat("原", 501) }, field: "cancellation_reason"},
		{name: "version", mutate: func(v *CancelDraftInput) { v.ExpectedDraftVersion = 0 }, field: "expected_draft_version"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validCancelDraftInput(t)
			test.mutate(&input)
			repository := &repositoryStub{}
			_, err := NewService(repository).CancelDraft(context.Background(), input)
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Field != test.field {
				t.Fatalf("error = %v, want ValidationError field %q", err, test.field)
			}
		})
	}
}

func TestCancelDraftIdempotencyAndCrossActionConflict(t *testing.T) {
	input := validCancelDraftInput(t)
	normalized, err := normalizeCancelDraftInput(input)
	if err != nil {
		t.Fatalf("normalizeCancelDraftInput() error = %v", err)
	}
	existing := DraftChangeCommand{
		CommandID:             normalized.CommandID,
		ContractID:            normalized.ContractID,
		Action:                DraftChangeActionCancel,
		ExpectedDraftVersion:  normalized.ExpectedDraftVersion,
		ResultingDraftVersion: normalized.ExpectedDraftVersion + 1,
		CancellationReason:    normalized.Reason,
	}
	repository := &repositoryStub{cancelCommand: existing}
	result, err := NewService(repository).CancelDraft(context.Background(), input)
	if err != nil || !result.Replayed {
		t.Fatalf("CancelDraft() replay result = %#v, error = %v", result, err)
	}
	repository.cancelCommand.Action = DraftChangeActionUpdate
	_, err = NewService(repository).CancelDraft(context.Background(), input)
	var conflict *IdempotencyConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("cross-action replay error = %v, want IdempotencyConflictError", err)
	}
}
