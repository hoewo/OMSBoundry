package contract

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func validActivateInput(t *testing.T) ActivateContractInput {
	t.Helper()
	commandID, err := NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	return ActivateContractInput{
		CommandID:            commandID,
		ContractID:           testContractID,
		ExpectedDraftVersion: 4,
		Actor: ActorRef{
			Kind:        " LOCAL_USER ",
			ID:          " aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa ",
			DisplayName: " 测试用户 ",
		},
	}
}

func TestActivateNormalizesAndPersists(t *testing.T) {
	repository := &repositoryStub{}
	result, err := NewService(repository).Activate(context.Background(), validActivateInput(t))
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	command := repository.receivedActivation
	if command.CommandID == "" || command.AuditID == "" || command.ContractID != testContractID {
		t.Fatalf("activation identifiers = %#v", command)
	}
	if command.Actor != testActor || command.ExpectedDraftVersion != 4 || command.ResultingRevision != 1 {
		t.Fatalf("activation command = %#v", command)
	}
	if command.OccurredAt.IsZero() {
		t.Fatalf("activation time = %#v", command)
	}
	if result.ContractID != testContractID || result.Revision != 1 || result.Replayed {
		t.Fatalf("result = %#v", result)
	}
}

func TestActivateValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ActivateContractInput)
		field  string
	}{
		{name: "command", mutate: func(v *ActivateContractInput) { v.CommandID = "bad" }, field: "command_id"},
		{name: "contract", mutate: func(v *ActivateContractInput) { v.ContractID = "bad" }, field: "contract_id"},
		{name: "version", mutate: func(v *ActivateContractInput) { v.ExpectedDraftVersion = 0 }, field: "expected_draft_version"},
		{name: "actor kind", mutate: func(v *ActivateContractInput) { v.Actor.Kind = "Local User" }, field: "actor"},
		{name: "actor ID", mutate: func(v *ActivateContractInput) { v.Actor.ID = " " }, field: "actor"},
		{name: "long actor ID", mutate: func(v *ActivateContractInput) { v.Actor.ID = strings.Repeat("a", 101) }, field: "actor"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validActivateInput(t)
			test.mutate(&input)
			repository := &repositoryStub{}
			_, err := NewService(repository).Activate(context.Background(), input)
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Field != test.field {
				t.Fatalf("error = %v, want ValidationError field %q", err, test.field)
			}
			if repository.receivedActivation.CommandID != "" {
				t.Fatalf("invalid activation reached repository: %#v", repository.receivedActivation)
			}
		})
	}
}

func TestActivateIdempotency(t *testing.T) {
	input := validActivateInput(t)
	normalized, err := normalizeActivateContractInput(input)
	if err != nil {
		t.Fatalf("normalizeActivateContractInput() error = %v", err)
	}
	existing := ActivationCommand{
		CommandID:            normalized.CommandID,
		ContractID:           normalized.ContractID,
		ExpectedDraftVersion: normalized.ExpectedDraftVersion,
		Actor:                normalized.Actor,
		ResultingRevision:    1,
	}
	repository := &repositoryStub{activationCommand: existing}
	result, err := NewService(repository).Activate(context.Background(), input)
	if err != nil {
		t.Fatalf("Activate() replay error = %v", err)
	}
	if !result.Replayed || result.Revision != 1 {
		t.Fatalf("result = %#v", result)
	}

	repository.activationCommand.Actor.ID = "another-operator"
	_, err = NewService(repository).Activate(context.Background(), input)
	var conflict *IdempotencyConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("changed replay error = %v, want IdempotencyConflictError", err)
	}
}
