package contract

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func validCorrectionInput(t *testing.T) CorrectContractInput {
	t.Helper()
	commandID, err := NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	return CorrectContractInput{
		CommandID: commandID, ContractID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		ExpectedStatus: " CLOSED ", ExpectedRevision: 3,
		InstrumentNumber: " CORR-001 ", InstrumentTitle: " 合同记录更正书 ",
		CorrectionReason:   " 原录入金额与已确认原件不一致 ",
		InstrumentSignedOn: "2026-11-11", InstrumentEffectiveOn: "2026-11-12", EffectiveOrder: 4,
		CorrectedTitle: " 更正后的合同标题 ", CorrectedCategory: " 服务 ",
		CorrectedTotalAmount: " 000899.750000 ",
		CorrectedSignedOn:    "2026-09-19", CorrectedEffectiveOn: "2026-10-01",
		Actor: testActor,
	}
}

func TestCorrectContractNormalizesAndPersists(t *testing.T) {
	repository := &repositoryStub{}
	result, err := NewService(repository).CorrectContract(context.Background(), validCorrectionInput(t))
	if err != nil {
		t.Fatalf("CorrectContract() error = %v", err)
	}
	command := repository.receivedCorrection
	if command.CommandID == "" || command.InstrumentID == "" || command.AuditID == "" {
		t.Fatalf("correction identifiers = %#v", command)
	}
	if command.ExpectedStatus != StatusClosed || command.ExpectedRevision != 3 || command.ResultingRevision != 4 ||
		command.EffectiveOrder != 4 || command.InstrumentNumber != "CORR-001" ||
		command.InstrumentTitle != "合同记录更正书" || command.CorrectionReason != "原录入金额与已确认原件不一致" ||
		command.CorrectedTitle != "更正后的合同标题" || command.CorrectedCategory != "服务" ||
		command.CorrectedTotalAmount != "899.75" || command.Actor != testActor || command.OccurredAt.IsZero() {
		t.Fatalf("normalized correction = %#v", command)
	}
	if result.Replayed || result.Revision != 4 || result.Status != StatusClosed || result.EffectiveAmount != "899.75" {
		t.Fatalf("correction result = %#v", result)
	}
}

func TestCorrectContractValidation(t *testing.T) {
	long := func(size int) string { return strings.Repeat("字", size) }
	tests := []struct {
		name   string
		mutate func(*CorrectContractInput)
		field  string
	}{
		{name: "command", mutate: func(v *CorrectContractInput) { v.CommandID = "bad" }, field: "command_id"},
		{name: "contract", mutate: func(v *CorrectContractInput) { v.ContractID = "bad" }, field: "contract_id"},
		{name: "status", mutate: func(v *CorrectContractInput) { v.ExpectedStatus = "draft" }, field: "expected_status"},
		{name: "revision", mutate: func(v *CorrectContractInput) { v.ExpectedRevision = 0 }, field: "expected_revision"},
		{name: "order", mutate: func(v *CorrectContractInput) { v.EffectiveOrder = 5 }, field: "effective_order"},
		{name: "number", mutate: func(v *CorrectContractInput) { v.InstrumentNumber = "" }, field: "correction_instrument_number"},
		{name: "instrument title", mutate: func(v *CorrectContractInput) { v.InstrumentTitle = long(201) }, field: "correction_instrument_title"},
		{name: "reason", mutate: func(v *CorrectContractInput) { v.CorrectionReason = "" }, field: "correction_reason"},
		{name: "instrument signed", mutate: func(v *CorrectContractInput) { v.InstrumentSignedOn = "" }, field: "correction_instrument_signed_on"},
		{name: "instrument effective", mutate: func(v *CorrectContractInput) { v.InstrumentEffectiveOn = "bad" }, field: "correction_instrument_effective_on"},
		{name: "corrected title", mutate: func(v *CorrectContractInput) { v.CorrectedTitle = "" }, field: "corrected_title"},
		{name: "corrected category", mutate: func(v *CorrectContractInput) { v.CorrectedCategory = long(101) }, field: "corrected_category"},
		{name: "corrected amount", mutate: func(v *CorrectContractInput) { v.CorrectedTotalAmount = "-1" }, field: "corrected_total_amount"},
		{name: "corrected signed", mutate: func(v *CorrectContractInput) { v.CorrectedSignedOn = "2026-02-30" }, field: "corrected_signed_on"},
		{name: "corrected effective", mutate: func(v *CorrectContractInput) { v.CorrectedEffectiveOn = "" }, field: "corrected_effective_on"},
		{name: "actor", mutate: func(v *CorrectContractInput) { v.Actor = ActorRef{} }, field: "actor"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validCorrectionInput(t)
			test.mutate(&input)
			repository := &repositoryStub{}
			_, err := NewService(repository).CorrectContract(context.Background(), input)
			var validation *ValidationError
			if !errors.As(err, &validation) || validation.Field != test.field {
				t.Fatalf("CorrectContract() error = %v", err)
			}
			if repository.receivedCorrection.CommandID != "" {
				t.Fatalf("invalid correction reached repository: %#v", repository.receivedCorrection)
			}
		})
	}
}

func TestCorrectContractIdempotency(t *testing.T) {
	input := validCorrectionInput(t)
	normalized, err := normalizeCorrectContractInput(input)
	if err != nil {
		t.Fatalf("normalizeCorrectContractInput() error = %v", err)
	}
	existing := ContractCorrectionCommand{
		CommandID: normalized.CommandID, InstrumentID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
		ContractID: normalized.ContractID, ExpectedStatus: Status(normalized.ExpectedStatus),
		ExpectedRevision: normalized.ExpectedRevision,
		InstrumentNumber: normalized.InstrumentNumber, InstrumentTitle: normalized.InstrumentTitle,
		CorrectionReason:   normalized.CorrectionReason,
		InstrumentSignedOn: normalized.InstrumentSignedOn, InstrumentEffectiveOn: normalized.InstrumentEffectiveOn,
		EffectiveOrder: normalized.EffectiveOrder,
		CorrectedTitle: normalized.CorrectedTitle, CorrectedCategory: normalized.CorrectedCategory,
		CorrectedTotalAmount: normalized.CorrectedTotalAmount,
		CorrectedSignedOn:    normalized.CorrectedSignedOn, CorrectedEffectiveOn: normalized.CorrectedEffectiveOn,
		Actor: normalized.Actor, ResultingRevision: normalized.ExpectedRevision + 1,
	}
	repository := &repositoryStub{correctionCommand: existing, correctionCreated: false}
	replayed, err := NewService(repository).CorrectContract(context.Background(), input)
	if err != nil || !replayed.Replayed || replayed.InstrumentID != existing.InstrumentID {
		t.Fatalf("replayed correction = %#v error=%v", replayed, err)
	}
	repository.correctionCommand.CorrectedTitle = "不同标题"
	_, err = NewService(repository).CorrectContract(context.Background(), input)
	var conflict *IdempotencyConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("changed correction replay error = %v", err)
	}
}
