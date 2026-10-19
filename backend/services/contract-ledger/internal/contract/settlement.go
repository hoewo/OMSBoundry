package contract

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"
)

const AuditEventContractSettlementRecorded = "contract_settlement_recorded"

type SettlementKind string

const (
	SettlementKindProgress SettlementKind = "progress"
	SettlementKindFinal    SettlementKind = "final"
)

func (k SettlementKind) Valid() bool {
	return k == SettlementKindProgress || k == SettlementKindFinal
}

func (k SettlementKind) Label() string {
	if k == SettlementKindFinal {
		return "最终结算"
	}
	return "阶段结算"
}

type ContractSettlementPartySnapshot struct {
	SettlementID           string
	Position               int64
	SourcePartyLinkID      string
	SourcePartyID          string
	SourcePartyRevision    int64
	PartyKind              string
	LegalName              string
	RegistrationIdentifier string
	Role                   string
}

type ContractSettlement struct {
	ID                string
	ContractID        string
	SourceRevision    int64
	Sequence          int64
	Kind              SettlementKind
	StatementNumber   string
	Title             string
	Amount            string
	Currency          string
	ConfirmedOn       string
	Basis             string
	ConfirmingParties []ContractSettlementPartySnapshot
	Actor             ActorRef
	RecordedAt        time.Time
}

type RecordContractSettlementInput struct {
	CommandID                  string
	ContractID                 string
	ExpectedContractRevision   int64
	ExpectedSettlementSequence int64
	Kind                       string
	StatementNumber            string
	Title                      string
	Amount                     string
	ConfirmedOn                string
	Basis                      string
	ConfirmingPartyPositions   []int64
	Actor                      ActorRef
}

type ContractSettlementCommand struct {
	CommandID                   string
	AuditID                     string
	SettlementID                string
	ContractID                  string
	ExpectedContractRevision    int64
	ExpectedSettlementSequence  int64
	Kind                        SettlementKind
	StatementNumber             string
	Title                       string
	Amount                      string
	ConfirmedOn                 string
	Basis                       string
	ConfirmingPartyPositions    []int64
	Actor                       ActorRef
	ResultingSettlementSequence int64
	Currency                    string
	OccurredAt                  time.Time
}

type ContractSettlementResult struct {
	Settlement ContractSettlement
	Replayed   bool
}

type ContractSettlementStateConflictError struct{ Status Status }

func (e *ContractSettlementStateConflictError) Error() string {
	return "只有已激活或已终止合同可以登记结算"
}

type ContractSettlementSequenceConflictError struct {
	Expected int64
	Actual   int64
}

func (e *ContractSettlementSequenceConflictError) Error() string {
	return fmt.Sprintf("结算 sequence 已变化：提交基于 %d，当前为 %d，请刷新后重试", e.Expected, e.Actual)
}

type ContractSettlementFinalizedError struct{}

func (e *ContractSettlementFinalizedError) Error() string {
	return "合同已经登记最终结算，不能继续新增结算"
}

type DuplicateSettlementNumberError struct{ StatementNumber string }

func (e *DuplicateSettlementNumberError) Error() string {
	return "该合同已存在相同结算文书编号"
}

type ContractSettlementPartyError struct{ Message string }

func (e *ContractSettlementPartyError) Error() string { return e.Message }

type ContractSettlementNotFoundError struct{ SettlementID string }

func (e *ContractSettlementNotFoundError) Error() string { return "合同结算不存在" }

func (s *Service) GetSettlement(ctx context.Context, settlementID string) (ContractSettlement, error) {
	normalizedID, err := normalizeUUID("settlement_id", "合同结算标识无效", settlementID)
	if err != nil {
		return ContractSettlement{}, err
	}
	value, found, err := s.repository.GetSettlement(ctx, normalizedID)
	if err != nil {
		return ContractSettlement{}, fmt.Errorf("get contract settlement: %w", err)
	}
	if !found {
		return ContractSettlement{}, &ContractSettlementNotFoundError{SettlementID: normalizedID}
	}
	return value, nil
}

func (s *Service) RecordSettlement(
	ctx context.Context,
	input RecordContractSettlementInput,
) (ContractSettlementResult, error) {
	normalized, err := normalizeRecordContractSettlementInput(input)
	if err != nil {
		return ContractSettlementResult{}, err
	}
	settlementID, err := s.newID()
	if err != nil {
		return ContractSettlementResult{}, fmt.Errorf("generate settlement ID: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return ContractSettlementResult{}, fmt.Errorf("generate settlement audit ID: %w", err)
	}
	command := ContractSettlementCommand{
		CommandID: normalized.CommandID, AuditID: auditID, SettlementID: settlementID,
		ContractID:                 normalized.ContractID,
		ExpectedContractRevision:   normalized.ExpectedContractRevision,
		ExpectedSettlementSequence: normalized.ExpectedSettlementSequence,
		Kind:                       SettlementKind(normalized.Kind), StatementNumber: normalized.StatementNumber,
		Title: normalized.Title, Amount: normalized.Amount, ConfirmedOn: normalized.ConfirmedOn,
		Basis:                       normalized.Basis,
		ConfirmingPartyPositions:    append([]int64(nil), normalized.ConfirmingPartyPositions...),
		Actor:                       normalized.Actor,
		ResultingSettlementSequence: normalized.ExpectedSettlementSequence + 1,
		OccurredAt:                  s.now(),
	}
	persistedCommand, settlement, created, err := s.repository.RecordSettlement(ctx, command)
	if err != nil {
		return ContractSettlementResult{}, fmt.Errorf("record contract settlement: %w", err)
	}
	if !created && !sameContractSettlementIntent(persistedCommand, command) {
		return ContractSettlementResult{}, &IdempotencyConflictError{CommandID: command.CommandID}
	}
	return ContractSettlementResult{Settlement: settlement, Replayed: !created}, nil
}

func normalizeRecordContractSettlementInput(input RecordContractSettlementInput) (RecordContractSettlementInput, error) {
	commandID, err := normalizeUUID("command_id", "命令标识无效，请刷新页面后重试", input.CommandID)
	if err != nil {
		return RecordContractSettlementInput{}, err
	}
	contractID, err := normalizeUUID("contract_id", "合同标识无效", input.ContractID)
	if err != nil {
		return RecordContractSettlementInput{}, err
	}
	if input.ExpectedContractRevision < 1 {
		return RecordContractSettlementInput{}, &ValidationError{Field: "expected_revision", Message: "合同 revision 无效，请刷新页面后重试"}
	}
	if input.ExpectedSettlementSequence < 0 {
		return RecordContractSettlementInput{}, &ValidationError{Field: "expected_settlement_sequence", Message: "结算 sequence 无效，请刷新页面后重试"}
	}
	kind := SettlementKind(strings.ToLower(strings.TrimSpace(input.Kind)))
	if !kind.Valid() {
		return RecordContractSettlementInput{}, &ValidationError{Field: "settlement_kind", Message: "结算类型无效"}
	}
	statementNumber, err := requiredText("settlement_number", "结算文书编号不能为空", input.StatementNumber, 100)
	if err != nil {
		return RecordContractSettlementInput{}, err
	}
	title, err := requiredText("settlement_title", "结算标题不能为空", input.Title, 200)
	if err != nil {
		return RecordContractSettlementInput{}, err
	}
	amount, err := normalizeSettlementAmount(input.Amount)
	if err != nil {
		return RecordContractSettlementInput{}, err
	}
	confirmedOn, err := normalizeDate("settlement_confirmed_on", "结算确认日期格式无效", input.ConfirmedOn)
	if err != nil {
		return RecordContractSettlementInput{}, err
	}
	if confirmedOn == "" {
		return RecordContractSettlementInput{}, &ValidationError{Field: "settlement_confirmed_on", Message: "结算确认日期不能为空"}
	}
	basis, err := requiredText("settlement_basis", "结算依据或口径说明不能为空", input.Basis, 1000)
	if err != nil {
		return RecordContractSettlementInput{}, err
	}
	positions := append([]int64(nil), input.ConfirmingPartyPositions...)
	if len(positions) == 0 {
		return RecordContractSettlementInput{}, &ValidationError{Field: "confirming_party_positions", Message: "至少选择一个确认主体"}
	}
	sort.Slice(positions, func(i, j int) bool { return positions[i] < positions[j] })
	for index, position := range positions {
		if position < 1 {
			return RecordContractSettlementInput{}, &ValidationError{Field: "confirming_party_positions", Message: "确认主体位置无效"}
		}
		if index > 0 && position == positions[index-1] {
			return RecordContractSettlementInput{}, &ValidationError{Field: "confirming_party_positions", Message: "确认主体不能重复选择"}
		}
	}
	actor, err := normalizeActor(input.Actor)
	if err != nil {
		return RecordContractSettlementInput{}, err
	}
	return RecordContractSettlementInput{
		CommandID: commandID, ContractID: contractID,
		ExpectedContractRevision:   input.ExpectedContractRevision,
		ExpectedSettlementSequence: input.ExpectedSettlementSequence,
		Kind:                       string(kind), StatementNumber: statementNumber, Title: title, Amount: amount,
		ConfirmedOn: confirmedOn, Basis: basis, ConfirmingPartyPositions: positions, Actor: actor,
	}, nil
}

func normalizeSettlementAmount(value string) (string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" || !amountPattern.MatchString(raw) {
		return "", &ValidationError{Field: "settlement_amount", Message: "累计结算金额必须是非负十进制数"}
	}
	parts := strings.SplitN(raw, ".", 2)
	integer := strings.TrimLeft(parts[0], "0")
	if integer == "" {
		integer = "0"
	}
	if len(integer) > 18 {
		return "", &ValidationError{Field: "settlement_amount", Message: "累计结算金额整数部分不能超过 18 位"}
	}
	if len(parts) == 1 {
		return integer, nil
	}
	if len(parts[1]) > 6 {
		return "", &ValidationError{Field: "settlement_amount", Message: "累计结算金额小数部分不能超过 6 位"}
	}
	fraction := strings.TrimRight(parts[1], "0")
	if fraction == "" {
		return integer, nil
	}
	return integer + "." + fraction, nil
}

func sameContractSettlementIntent(left, right ContractSettlementCommand) bool {
	return left.CommandID == right.CommandID &&
		left.ContractID == right.ContractID &&
		left.ExpectedContractRevision == right.ExpectedContractRevision &&
		left.ExpectedSettlementSequence == right.ExpectedSettlementSequence &&
		left.Kind == right.Kind &&
		left.StatementNumber == right.StatementNumber &&
		left.Title == right.Title &&
		left.Amount == right.Amount &&
		left.ConfirmedOn == right.ConfirmedOn &&
		left.Basis == right.Basis &&
		slices.Equal(left.ConfirmingPartyPositions, right.ConfirmingPartyPositions) &&
		left.Actor == right.Actor &&
		left.ResultingSettlementSequence == right.ResultingSettlementSequence
}
