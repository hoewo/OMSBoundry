package contract

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func validTerminateInput(t *testing.T) TerminateContractInput {
	t.Helper()
	commandID, err := NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	return TerminateContractInput{
		CommandID: commandID, ContractID: testContractID, ExpectedRevision: 2,
		InstrumentNumber: " 终-001 ", Title: " 合同终止协议 ",
		TerminationReason: " 双方确认自约定日期终止 ",
		SignedOn:          "2026-10-10", EffectiveOn: "2026-10-15", EffectiveOrder: 3,
		Actor: ActorRef{Kind: " LOCAL_USER ", ID: " aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa ", DisplayName: " 测试用户 "},
	}
}

func TestTerminateNormalizesAndPersists(t *testing.T) {
	repository := &repositoryStub{}
	result, err := NewService(repository).Terminate(context.Background(), validTerminateInput(t))
	if err != nil {
		t.Fatalf("Terminate() error = %v", err)
	}
	command := repository.receivedTermination
	if command.CommandID == "" || command.InstrumentID == "" || command.AuditID == "" {
		t.Fatalf("identifiers = %#v", command)
	}
	if command.InstrumentNumber != "终-001" || command.Title != "合同终止协议" || command.TerminationReason != "双方确认自约定日期终止" {
		t.Fatalf("termination fields = %#v", command)
	}
	if command.Actor != testActor || command.ResultingRevision != 3 || command.OccurredAt.IsZero() {
		t.Fatalf("termination metadata = %#v", command)
	}
	if result.ContractID != testContractID || result.Revision != 3 || result.EffectiveAmount != "1250.5" || result.Status != StatusTerminated || result.Replayed {
		t.Fatalf("result = %#v", result)
	}
}

func TestTerminateValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*TerminateContractInput)
		field  string
	}{
		{name: "command", mutate: func(v *TerminateContractInput) { v.CommandID = "bad" }, field: "command_id"},
		{name: "contract", mutate: func(v *TerminateContractInput) { v.ContractID = "bad" }, field: "contract_id"},
		{name: "revision", mutate: func(v *TerminateContractInput) { v.ExpectedRevision = 0 }, field: "expected_revision"},
		{name: "order", mutate: func(v *TerminateContractInput) { v.EffectiveOrder = 4 }, field: "effective_order"},
		{name: "number", mutate: func(v *TerminateContractInput) { v.InstrumentNumber = " " }, field: "instrument_number"},
		{name: "title", mutate: func(v *TerminateContractInput) { v.Title = " " }, field: "instrument_title"},
		{name: "reason", mutate: func(v *TerminateContractInput) { v.TerminationReason = strings.Repeat("a", 1001) }, field: "termination_reason"},
		{name: "signed date", mutate: func(v *TerminateContractInput) { v.SignedOn = "" }, field: "instrument_signed_on"},
		{name: "effective date", mutate: func(v *TerminateContractInput) { v.EffectiveOn = "2026-13-01" }, field: "instrument_effective_on"},
		{name: "actor", mutate: func(v *TerminateContractInput) { v.Actor.ID = "forged" }, field: "actor"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validTerminateInput(t)
			test.mutate(&input)
			repository := &repositoryStub{}
			_, err := NewService(repository).Terminate(context.Background(), input)
			var validation *ValidationError
			if !errors.As(err, &validation) || validation.Field != test.field {
				t.Fatalf("error = %v, want ValidationError field %q", err, test.field)
			}
			if repository.receivedTermination.CommandID != "" {
				t.Fatalf("invalid input reached repository: %#v", repository.receivedTermination)
			}
		})
	}
}

func TestTerminateIdempotency(t *testing.T) {
	input := validTerminateInput(t)
	normalized, err := normalizeTerminateContractInput(input)
	if err != nil {
		t.Fatalf("normalizeTerminateContractInput() error = %v", err)
	}
	existing := TerminationCommand{
		CommandID: normalized.CommandID, InstrumentID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		ContractID: normalized.ContractID, ExpectedRevision: normalized.ExpectedRevision,
		InstrumentNumber: normalized.InstrumentNumber, Title: normalized.Title,
		TerminationReason: normalized.TerminationReason,
		SignedOn:          normalized.SignedOn, EffectiveOn: normalized.EffectiveOn,
		EffectiveOrder: normalized.EffectiveOrder, Actor: normalized.Actor,
		ResultingRevision: 3, ResultingAmount: "1250.5",
	}
	repository := &repositoryStub{terminationCommand: existing}
	result, err := NewService(repository).Terminate(context.Background(), input)
	if err != nil || !result.Replayed || result.Status != StatusTerminated || result.Revision != 3 {
		t.Fatalf("Terminate() = %#v, error=%v", result, err)
	}
	repository.terminationCommand.TerminationReason = "改变终止原因"
	_, err = NewService(repository).Terminate(context.Background(), input)
	var conflict *IdempotencyConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("changed termination replay error = %v, want IdempotencyConflictError", err)
	}
}
