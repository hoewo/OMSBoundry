package contract

import (
	"context"
	"fmt"
	"time"
)

const (
	InstrumentTypeTermination    = "termination_instrument"
	AuditEventContractTerminated = "contract_terminated"
)

type TerminateContractInput struct {
	CommandID         string
	ContractID        string
	ExpectedRevision  int64
	InstrumentNumber  string
	Title             string
	TerminationReason string
	SignedOn          string
	EffectiveOn       string
	EffectiveOrder    int64
	Actor             ActorRef
}

type TerminationCommand struct {
	CommandID         string
	AuditID           string
	InstrumentID      string
	ContractID        string
	ExpectedRevision  int64
	InstrumentNumber  string
	Title             string
	TerminationReason string
	SignedOn          string
	EffectiveOn       string
	EffectiveOrder    int64
	Actor             ActorRef
	ResultingRevision int64
	ResultingAmount   string
	OccurredAt        time.Time
}

type TerminationResult struct {
	ContractID      string
	InstrumentID    string
	Revision        int64
	EffectiveAmount string
	Status          Status
	Replayed        bool
}

type ContractTerminationStateConflictError struct{ Status Status }

func (e *ContractTerminationStateConflictError) Error() string {
	return "只有已激活合同可以登记生效终止文书"
}

type ContractTerminationPreconditionError struct {
	Field   string
	Message string
}

func (e *ContractTerminationPreconditionError) Error() string { return e.Message }

func (s *Service) Terminate(ctx context.Context, input TerminateContractInput) (TerminationResult, error) {
	normalized, err := normalizeTerminateContractInput(input)
	if err != nil {
		return TerminationResult{}, err
	}
	instrumentID, err := s.newID()
	if err != nil {
		return TerminationResult{}, fmt.Errorf("generate termination instrument ID: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return TerminationResult{}, fmt.Errorf("generate contract termination audit ID: %w", err)
	}
	command := TerminationCommand{
		CommandID: normalized.CommandID, AuditID: auditID, InstrumentID: instrumentID,
		ContractID: normalized.ContractID, ExpectedRevision: normalized.ExpectedRevision,
		InstrumentNumber: normalized.InstrumentNumber, Title: normalized.Title,
		TerminationReason: normalized.TerminationReason,
		SignedOn:          normalized.SignedOn, EffectiveOn: normalized.EffectiveOn,
		EffectiveOrder: normalized.EffectiveOrder, Actor: normalized.Actor,
		ResultingRevision: normalized.ExpectedRevision + 1, OccurredAt: s.now(),
	}
	persisted, created, err := s.repository.Terminate(ctx, command)
	if err != nil {
		return TerminationResult{}, fmt.Errorf("terminate contract: %w", err)
	}
	if !created && !sameTerminationIntent(persisted, command) {
		return TerminationResult{}, &IdempotencyConflictError{CommandID: command.CommandID}
	}
	return TerminationResult{
		ContractID: persisted.ContractID, InstrumentID: persisted.InstrumentID,
		Revision: persisted.ResultingRevision, EffectiveAmount: persisted.ResultingAmount,
		Status: StatusTerminated, Replayed: !created,
	}, nil
}

func normalizeTerminateContractInput(input TerminateContractInput) (TerminateContractInput, error) {
	commandID, err := normalizeUUID("command_id", "命令标识无效，请刷新页面后重试", input.CommandID)
	if err != nil {
		return TerminateContractInput{}, err
	}
	contractID, err := normalizeUUID("contract_id", "合同标识无效", input.ContractID)
	if err != nil {
		return TerminateContractInput{}, err
	}
	if input.ExpectedRevision < 1 {
		return TerminateContractInput{}, &ValidationError{Field: "expected_revision", Message: "合同 revision 无效，请刷新页面后重试"}
	}
	if input.EffectiveOrder != input.ExpectedRevision+1 {
		return TerminateContractInput{}, &ValidationError{Field: "effective_order", Message: "终止文书生效顺序必须紧接当前合同 revision"}
	}
	instrumentNumber, err := requiredText("instrument_number", "终止文书编号不能为空", input.InstrumentNumber, 100)
	if err != nil {
		return TerminateContractInput{}, err
	}
	title, err := requiredText("instrument_title", "终止文书标题不能为空", input.Title, 200)
	if err != nil {
		return TerminateContractInput{}, err
	}
	reason, err := requiredText("termination_reason", "终止原因或依据不能为空", input.TerminationReason, 1000)
	if err != nil {
		return TerminateContractInput{}, err
	}
	signedOn, err := normalizeDate("instrument_signed_on", "终止文书签署或出具日期格式无效", input.SignedOn)
	if err != nil {
		return TerminateContractInput{}, err
	}
	if signedOn == "" {
		return TerminateContractInput{}, &ValidationError{Field: "instrument_signed_on", Message: "终止文书签署或出具日期不能为空"}
	}
	effectiveOn, err := normalizeDate("instrument_effective_on", "终止生效日期格式无效", input.EffectiveOn)
	if err != nil {
		return TerminateContractInput{}, err
	}
	if effectiveOn == "" {
		return TerminateContractInput{}, &ValidationError{Field: "instrument_effective_on", Message: "终止生效日期不能为空"}
	}
	actor, err := normalizeActor(input.Actor)
	if err != nil {
		return TerminateContractInput{}, err
	}
	return TerminateContractInput{
		CommandID: commandID, ContractID: contractID, ExpectedRevision: input.ExpectedRevision,
		InstrumentNumber: instrumentNumber, Title: title, TerminationReason: reason,
		SignedOn: signedOn, EffectiveOn: effectiveOn, EffectiveOrder: input.EffectiveOrder,
		Actor: actor,
	}, nil
}

func sameTerminationIntent(left, right TerminationCommand) bool {
	return left.CommandID == right.CommandID &&
		left.ContractID == right.ContractID &&
		left.ExpectedRevision == right.ExpectedRevision &&
		left.InstrumentNumber == right.InstrumentNumber &&
		left.Title == right.Title &&
		left.TerminationReason == right.TerminationReason &&
		left.SignedOn == right.SignedOn &&
		left.EffectiveOn == right.EffectiveOn &&
		left.EffectiveOrder == right.EffectiveOrder &&
		left.Actor == right.Actor &&
		left.ResultingRevision == right.ResultingRevision
}
