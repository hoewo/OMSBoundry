package contract

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func validAppendChangeInput(t *testing.T) AppendContractChangeInput {
	t.Helper()
	commandID, err := NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	return AppendContractChangeInput{
		CommandID: commandID, ContractID: testContractID, ExpectedRevision: 1,
		InstrumentNumber: " 补-001 ", Title: " 第一份补充协议 ",
		ChangeSummary: " 增加服务范围并调整金额 ",
		SignedOn:      "2026-10-01", EffectiveOn: "2026-10-02", EffectiveOrder: 2,
		AmountEffect: " ADJUSTMENT ", AmountValue: " +000250.500000 ",
		Actor: ActorRef{Kind: " LOCAL_USER ", ID: " aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa ", DisplayName: " 测试用户 "},
	}
}

func TestAppendChangeNormalizesAndPersists(t *testing.T) {
	repository := &repositoryStub{}
	result, err := NewService(repository).AppendChange(context.Background(), validAppendChangeInput(t))
	if err != nil {
		t.Fatalf("AppendChange() error = %v", err)
	}
	command := repository.receivedChange
	if command.CommandID == "" || command.InstrumentID == "" || command.AuditID == "" {
		t.Fatalf("identifiers = %#v", command)
	}
	if command.InstrumentNumber != "补-001" || command.Title != "第一份补充协议" || command.ChangeSummary != "增加服务范围并调整金额" {
		t.Fatalf("instrument fields = %#v", command)
	}
	if command.AmountEffect != AmountEffectAdjustment || command.AmountValue != "250.5" {
		t.Fatalf("amount input = %#v", command)
	}
	if command.Actor != testActor || command.ResultingRevision != 2 || command.OccurredAt.IsZero() {
		t.Fatalf("command metadata = %#v", command)
	}
	if result.ContractID != testContractID || result.Revision != 2 || result.EffectiveAmount != "1250.5" || result.Replayed {
		t.Fatalf("result = %#v", result)
	}
}

func TestAppendChangeValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*AppendContractChangeInput)
		field  string
	}{
		{name: "command", mutate: func(v *AppendContractChangeInput) { v.CommandID = "bad" }, field: "command_id"},
		{name: "contract", mutate: func(v *AppendContractChangeInput) { v.ContractID = "bad" }, field: "contract_id"},
		{name: "revision", mutate: func(v *AppendContractChangeInput) { v.ExpectedRevision = 0 }, field: "expected_revision"},
		{name: "order", mutate: func(v *AppendContractChangeInput) { v.EffectiveOrder = 3 }, field: "effective_order"},
		{name: "number", mutate: func(v *AppendContractChangeInput) { v.InstrumentNumber = " " }, field: "instrument_number"},
		{name: "title", mutate: func(v *AppendContractChangeInput) { v.Title = " " }, field: "instrument_title"},
		{name: "summary", mutate: func(v *AppendContractChangeInput) { v.ChangeSummary = strings.Repeat("a", 1001) }, field: "change_summary"},
		{name: "signed date", mutate: func(v *AppendContractChangeInput) { v.SignedOn = "" }, field: "instrument_signed_on"},
		{name: "effective date", mutate: func(v *AppendContractChangeInput) { v.EffectiveOn = "2026-13-01" }, field: "instrument_effective_on"},
		{name: "effect", mutate: func(v *AppendContractChangeInput) { v.AmountEffect = "magic" }, field: "amount_effect"},
		{name: "zero adjustment", mutate: func(v *AppendContractChangeInput) { v.AmountValue = "0.000000" }, field: "amount_value"},
		{name: "adjustment precision", mutate: func(v *AppendContractChangeInput) { v.AmountValue = "1.0000001" }, field: "amount_value"},
		{name: "actor", mutate: func(v *AppendContractChangeInput) { v.Actor.ID = "forged" }, field: "actor"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validAppendChangeInput(t)
			test.mutate(&input)
			repository := &repositoryStub{}
			_, err := NewService(repository).AppendChange(context.Background(), input)
			var validation *ValidationError
			if !errors.As(err, &validation) || validation.Field != test.field {
				t.Fatalf("error = %v, want ValidationError field %q", err, test.field)
			}
			if repository.receivedChange.CommandID != "" {
				t.Fatalf("invalid input reached repository: %#v", repository.receivedChange)
			}
		})
	}
}

func TestNormalizeChangeAmountSemantics(t *testing.T) {
	tests := []struct {
		name    string
		effect  string
		value   string
		want    string
		wantErr bool
	}{
		{name: "positive adjustment", effect: AmountEffectAdjustment, value: "+001.230000", want: "1.23"},
		{name: "negative adjustment", effect: AmountEffectAdjustment, value: "-001.230000", want: "-1.23"},
		{name: "restatement zero", effect: AmountEffectRestatement, value: "000.000000", want: "0"},
		{name: "unchanged", effect: AmountEffectUnchanged, value: "", want: ""},
		{name: "unchanged with value", effect: AmountEffectUnchanged, value: "1", wantErr: true},
		{name: "restatement missing", effect: AmountEffectRestatement, value: "", wantErr: true},
		{name: "negative restatement", effect: AmountEffectRestatement, value: "-1", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeChangeAmount(test.effect, test.value)
			if test.wantErr {
				if err == nil {
					t.Fatalf("normalizeChangeAmount() = %q, want error", got)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("normalizeChangeAmount() = %q, error=%v, want %q", got, err, test.want)
			}
		})
	}
}

func TestApplyAmountEffectUsesExactDecimalArithmetic(t *testing.T) {
	tests := []struct {
		name    string
		current string
		effect  string
		value   string
		want    string
		wantErr bool
	}{
		{name: "add smallest unit", current: "1000.1", effect: AmountEffectAdjustment, value: "0.000001", want: "1000.100001"},
		{name: "subtract", current: "1000", effect: AmountEffectAdjustment, value: "-250.5", want: "749.5"},
		{name: "to zero", current: "250.5", effect: AmountEffectAdjustment, value: "-250.5", want: "0"},
		{name: "restatement", current: "1000", effect: AmountEffectRestatement, value: "88.88", want: "88.88"},
		{name: "unknown unchanged", current: "", effect: AmountEffectUnchanged, value: "", want: ""},
		{name: "unknown adjustment", current: "", effect: AmountEffectAdjustment, value: "1", wantErr: true},
		{name: "negative result", current: "10", effect: AmountEffectAdjustment, value: "-10.000001", wantErr: true},
		{name: "overflow", current: "999999999999999999.999999", effect: AmountEffectAdjustment, value: "0.000001", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := applyAmountEffect(test.current, test.effect, test.value)
			if test.wantErr {
				if err == nil {
					t.Fatalf("applyAmountEffect() = %q, want error", got)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("applyAmountEffect() = %q, error=%v, want %q", got, err, test.want)
			}
		})
	}
}

func TestAppendChangeIdempotency(t *testing.T) {
	input := validAppendChangeInput(t)
	normalized, err := normalizeAppendContractChangeInput(input)
	if err != nil {
		t.Fatalf("normalizeAppendContractChangeInput() error = %v", err)
	}
	existing := ContractChangeCommand{
		CommandID: normalized.CommandID, InstrumentID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		ContractID: normalized.ContractID, ExpectedRevision: normalized.ExpectedRevision,
		InstrumentNumber: normalized.InstrumentNumber, Title: normalized.Title, ChangeSummary: normalized.ChangeSummary,
		SignedOn: normalized.SignedOn, EffectiveOn: normalized.EffectiveOn, EffectiveOrder: normalized.EffectiveOrder,
		AmountEffect: normalized.AmountEffect, AmountValue: normalized.AmountValue, Actor: normalized.Actor,
		ResultingRevision: 2, ResultingAmount: "1250.5",
	}
	repository := &repositoryStub{changeCommand: existing}
	result, err := NewService(repository).AppendChange(context.Background(), input)
	if err != nil || !result.Replayed || result.Revision != 2 {
		t.Fatalf("AppendChange() = %#v, error=%v", result, err)
	}
	repository.changeCommand.AmountValue = "300"
	_, err = NewService(repository).AppendChange(context.Background(), input)
	var conflict *IdempotencyConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("changed replay error = %v, want IdempotencyConflictError", err)
	}
}
