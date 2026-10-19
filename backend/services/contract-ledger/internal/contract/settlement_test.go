package contract

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func validSettlementInput(t *testing.T) RecordContractSettlementInput {
	t.Helper()
	commandID, err := NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	return RecordContractSettlementInput{
		CommandID: commandID, ContractID: testContractID,
		ExpectedContractRevision: 2, ExpectedSettlementSequence: 1,
		Kind: " PROGRESS ", StatementNumber: " 结-002 ", Title: " 第二阶段结算 ",
		Amount: " 001200.500000 ", ConfirmedOn: "2026-11-05",
		Basis:                    " 双方依据验收记录确认累计结算金额 ",
		ConfirmingPartyPositions: []int64{2, 1},
		Actor:                    ActorRef{Kind: " LOCAL_USER ", ID: " aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa ", DisplayName: " 测试用户 "},
	}
}

func TestRecordSettlementNormalizesAndPersists(t *testing.T) {
	repository := &repositoryStub{}
	result, err := NewService(repository).RecordSettlement(context.Background(), validSettlementInput(t))
	if err != nil {
		t.Fatalf("RecordSettlement() error = %v", err)
	}
	command := repository.receivedSettlement
	if command.CommandID == "" || command.SettlementID == "" || command.AuditID == "" {
		t.Fatalf("identifiers = %#v", command)
	}
	if command.Kind != SettlementKindProgress || command.StatementNumber != "结-002" || command.Title != "第二阶段结算" ||
		command.Amount != "1200.5" || command.ConfirmedOn != "2026-11-05" || command.Basis != "双方依据验收记录确认累计结算金额" {
		t.Fatalf("normalized settlement = %#v", command)
	}
	if len(command.ConfirmingPartyPositions) != 2 || command.ConfirmingPartyPositions[0] != 1 || command.ConfirmingPartyPositions[1] != 2 {
		t.Fatalf("positions = %#v", command.ConfirmingPartyPositions)
	}
	if command.Actor != testActor || command.ResultingSettlementSequence != 2 || command.OccurredAt.IsZero() {
		t.Fatalf("metadata = %#v", command)
	}
	if result.Replayed || result.Settlement.Sequence != 2 || result.Settlement.Amount != "1200.5" {
		t.Fatalf("result = %#v", result)
	}
}

func TestRecordSettlementValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*RecordContractSettlementInput)
		field  string
	}{
		{name: "command", mutate: func(v *RecordContractSettlementInput) { v.CommandID = "bad" }, field: "command_id"},
		{name: "contract", mutate: func(v *RecordContractSettlementInput) { v.ContractID = "bad" }, field: "contract_id"},
		{name: "revision", mutate: func(v *RecordContractSettlementInput) { v.ExpectedContractRevision = 0 }, field: "expected_revision"},
		{name: "sequence", mutate: func(v *RecordContractSettlementInput) { v.ExpectedSettlementSequence = -1 }, field: "expected_settlement_sequence"},
		{name: "kind", mutate: func(v *RecordContractSettlementInput) { v.Kind = "interim" }, field: "settlement_kind"},
		{name: "number", mutate: func(v *RecordContractSettlementInput) { v.StatementNumber = " " }, field: "settlement_number"},
		{name: "title", mutate: func(v *RecordContractSettlementInput) { v.Title = " " }, field: "settlement_title"},
		{name: "empty amount", mutate: func(v *RecordContractSettlementInput) { v.Amount = "" }, field: "settlement_amount"},
		{name: "negative amount", mutate: func(v *RecordContractSettlementInput) { v.Amount = "-1" }, field: "settlement_amount"},
		{name: "exponent amount", mutate: func(v *RecordContractSettlementInput) { v.Amount = "1e3" }, field: "settlement_amount"},
		{name: "large amount", mutate: func(v *RecordContractSettlementInput) { v.Amount = "1000000000000000000" }, field: "settlement_amount"},
		{name: "precise amount", mutate: func(v *RecordContractSettlementInput) { v.Amount = "1.0000001" }, field: "settlement_amount"},
		{name: "date", mutate: func(v *RecordContractSettlementInput) { v.ConfirmedOn = "2026-13-01" }, field: "settlement_confirmed_on"},
		{name: "basis", mutate: func(v *RecordContractSettlementInput) { v.Basis = strings.Repeat("字", 1001) }, field: "settlement_basis"},
		{name: "no party", mutate: func(v *RecordContractSettlementInput) { v.ConfirmingPartyPositions = nil }, field: "confirming_party_positions"},
		{name: "bad party", mutate: func(v *RecordContractSettlementInput) { v.ConfirmingPartyPositions = []int64{0} }, field: "confirming_party_positions"},
		{name: "duplicate party", mutate: func(v *RecordContractSettlementInput) { v.ConfirmingPartyPositions = []int64{1, 1} }, field: "confirming_party_positions"},
		{name: "actor", mutate: func(v *RecordContractSettlementInput) { v.Actor.ID = " " }, field: "actor"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validSettlementInput(t)
			test.mutate(&input)
			repository := &repositoryStub{}
			_, err := NewService(repository).RecordSettlement(context.Background(), input)
			var validation *ValidationError
			if !errors.As(err, &validation) || validation.Field != test.field {
				t.Fatalf("error = %v, want ValidationError field %q", err, test.field)
			}
			if repository.receivedSettlement.CommandID != "" {
				t.Fatalf("invalid settlement reached repository: %#v", repository.receivedSettlement)
			}
		})
	}
}

func TestRecordSettlementIdempotencyIncludesParties(t *testing.T) {
	input := validSettlementInput(t)
	normalized, err := normalizeRecordContractSettlementInput(input)
	if err != nil {
		t.Fatalf("normalizeRecordContractSettlementInput() error = %v", err)
	}
	existing := ContractSettlementCommand{
		CommandID: normalized.CommandID, SettlementID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		ContractID:                 normalized.ContractID,
		ExpectedContractRevision:   normalized.ExpectedContractRevision,
		ExpectedSettlementSequence: normalized.ExpectedSettlementSequence,
		Kind:                       SettlementKind(normalized.Kind), StatementNumber: normalized.StatementNumber,
		Title: normalized.Title, Amount: normalized.Amount, ConfirmedOn: normalized.ConfirmedOn,
		Basis: normalized.Basis, ConfirmingPartyPositions: normalized.ConfirmingPartyPositions,
		Actor: normalized.Actor, ResultingSettlementSequence: 2, Currency: "CNY",
	}
	repository := &repositoryStub{
		settlementCommand: existing,
		settlement:        ContractSettlement{ID: existing.SettlementID, Sequence: 2, Amount: normalized.Amount},
	}
	result, err := NewService(repository).RecordSettlement(context.Background(), input)
	if err != nil || !result.Replayed || result.Settlement.Sequence != 2 {
		t.Fatalf("RecordSettlement() = %#v, error=%v", result, err)
	}
	repository.settlementCommand.ConfirmingPartyPositions = []int64{1}
	_, err = NewService(repository).RecordSettlement(context.Background(), input)
	var conflict *IdempotencyConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("changed replay error = %v, want IdempotencyConflictError", err)
	}
}

func TestSettlementKindLabels(t *testing.T) {
	if SettlementKindProgress.Label() != "阶段结算" || SettlementKindFinal.Label() != "最终结算" {
		t.Fatalf("settlement labels changed")
	}
}

func TestGetSettlementReturnsStableOwnedFact(t *testing.T) {
	want := ContractSettlement{ID: testContractID, ContractID: "22222222-2222-4222-8222-222222222222", Title: "最终结算书", Sequence: 2}
	value, err := NewService(&repositoryStub{settlement: want}).GetSettlement(context.Background(), " "+strings.ToUpper(testContractID)+" ")
	if err != nil || value.ID != want.ID || value.ContractID != want.ContractID || value.Title != want.Title || value.Sequence != want.Sequence {
		t.Fatalf("value=%#v err=%v", value, err)
	}
	_, err = NewService(&repositoryStub{}).GetSettlement(context.Background(), testContractID)
	var notFound *ContractSettlementNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("error=%v", err)
	}
}
