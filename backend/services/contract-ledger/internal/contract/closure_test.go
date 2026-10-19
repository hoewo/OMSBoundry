package contract

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func validCloseContractInput(t *testing.T) CloseContractInput {
	t.Helper()
	commandID, err := NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	return CloseContractInput{
		CommandID: commandID, ContractID: testContractID,
		ExpectedContractRevision: 3, ExpectedSettlementSequence: 2,
		SettlementDisposition: " FINAL_RECORDED ", ClosedOn: "2026-11-10",
		Reason: " 最终结算已经确认，常规合同维护结束 ",
		Actor:  ActorRef{Kind: " LOCAL_USER ", ID: " aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa ", DisplayName: " 测试用户 "},
	}
}

func TestCloseContractNormalizesAndPersists(t *testing.T) {
	repository := &repositoryStub{}
	result, err := NewService(repository).CloseContract(context.Background(), validCloseContractInput(t))
	if err != nil {
		t.Fatalf("CloseContract() error = %v", err)
	}
	command := repository.receivedClosure
	if command.CommandID == "" || command.ClosureID == "" || command.AuditID == "" {
		t.Fatalf("identifiers = %#v", command)
	}
	if command.ContractID != testContractID || command.ExpectedContractRevision != 3 || command.ExpectedSettlementSequence != 2 ||
		command.SettlementDisposition != SettlementDispositionFinalRecorded || command.ClosedOn != "2026-11-10" ||
		command.Reason != "最终结算已经确认，常规合同维护结束" || command.Actor != testActor || command.OccurredAt.IsZero() {
		t.Fatalf("normalized closure = %#v", command)
	}
	if result.Replayed || result.Closure.ID == "" || result.Closure.SettlementDisposition != SettlementDispositionFinalRecorded {
		t.Fatalf("result = %#v", result)
	}
}

func TestCloseContractValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CloseContractInput)
		field  string
	}{
		{name: "command", mutate: func(v *CloseContractInput) { v.CommandID = "bad" }, field: "command_id"},
		{name: "contract", mutate: func(v *CloseContractInput) { v.ContractID = "bad" }, field: "contract_id"},
		{name: "revision", mutate: func(v *CloseContractInput) { v.ExpectedContractRevision = 0 }, field: "expected_revision"},
		{name: "sequence", mutate: func(v *CloseContractInput) { v.ExpectedSettlementSequence = -1 }, field: "expected_settlement_sequence"},
		{name: "disposition", mutate: func(v *CloseContractInput) { v.SettlementDisposition = "pending" }, field: "settlement_disposition"},
		{name: "empty date", mutate: func(v *CloseContractInput) { v.ClosedOn = "" }, field: "closed_on"},
		{name: "invalid date", mutate: func(v *CloseContractInput) { v.ClosedOn = "2026-13-01" }, field: "closed_on"},
		{name: "empty reason", mutate: func(v *CloseContractInput) { v.Reason = " " }, field: "closure_reason"},
		{name: "long reason", mutate: func(v *CloseContractInput) { v.Reason = strings.Repeat("字", 1001) }, field: "closure_reason"},
		{name: "actor", mutate: func(v *CloseContractInput) { v.Actor.ID = " " }, field: "actor"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validCloseContractInput(t)
			test.mutate(&input)
			repository := &repositoryStub{}
			_, err := NewService(repository).CloseContract(context.Background(), input)
			var validation *ValidationError
			if !errors.As(err, &validation) || validation.Field != test.field {
				t.Fatalf("error = %v, want ValidationError field %q", err, test.field)
			}
			if repository.receivedClosure.CommandID != "" {
				t.Fatalf("invalid closure reached repository: %#v", repository.receivedClosure)
			}
		})
	}
}

func TestCloseContractIdempotency(t *testing.T) {
	input := validCloseContractInput(t)
	normalized, err := normalizeCloseContractInput(input)
	if err != nil {
		t.Fatalf("normalizeCloseContractInput() error = %v", err)
	}
	existing := ContractClosureCommand{
		CommandID: normalized.CommandID, ClosureID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		ContractID:                 normalized.ContractID,
		ExpectedContractRevision:   normalized.ExpectedContractRevision,
		ExpectedSettlementSequence: normalized.ExpectedSettlementSequence,
		SettlementDisposition:      SettlementDisposition(normalized.SettlementDisposition),
		ClosedOn:                   normalized.ClosedOn, Reason: normalized.Reason, Actor: normalized.Actor,
		FinalSettlementID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
	}
	repository := &repositoryStub{
		closureCommand: existing,
		closure:        ContractClosure{ID: existing.ClosureID, ContractID: existing.ContractID, SettlementDisposition: existing.SettlementDisposition},
	}
	result, err := NewService(repository).CloseContract(context.Background(), input)
	if err != nil || !result.Replayed || result.Closure.ID != existing.ClosureID {
		t.Fatalf("CloseContract() = %#v, error=%v", result, err)
	}
	repository.closureCommand.Reason = "改变关闭原因"
	_, err = NewService(repository).CloseContract(context.Background(), input)
	var conflict *IdempotencyConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("changed closure replay error = %v, want IdempotencyConflictError", err)
	}
}

func TestSettlementDispositionLabels(t *testing.T) {
	if SettlementDispositionFinalRecorded.Label() != "已登记最终结算" ||
		SettlementDispositionNotRequired.Label() != "无需单独最终结算" {
		t.Fatal("settlement disposition labels changed")
	}
}
