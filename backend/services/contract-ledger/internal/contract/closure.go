package contract

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const AuditEventContractClosed = "contract_closed"

type SettlementDisposition string

const (
	SettlementDispositionFinalRecorded SettlementDisposition = "final_recorded"
	SettlementDispositionNotRequired   SettlementDisposition = "not_required"
)

func (d SettlementDisposition) Valid() bool {
	return d == SettlementDispositionFinalRecorded || d == SettlementDispositionNotRequired
}

func (d SettlementDisposition) Label() string {
	if d == SettlementDispositionFinalRecorded {
		return "已登记最终结算"
	}
	return "无需单独最终结算"
}

type ContractClosure struct {
	ID                       string
	ContractID               string
	SourceRevision           int64
	SourceSettlementSequence int64
	SettlementDisposition    SettlementDisposition
	FinalSettlementID        string
	ClosedOn                 string
	Reason                   string
	Actor                    ActorRef
	RecordedAt               time.Time
}

type CloseContractInput struct {
	CommandID                  string
	ContractID                 string
	ExpectedContractRevision   int64
	ExpectedSettlementSequence int64
	SettlementDisposition      string
	ClosedOn                   string
	Reason                     string
	Actor                      ActorRef
}

type ContractClosureCommand struct {
	CommandID                  string
	AuditID                    string
	ClosureID                  string
	ContractID                 string
	ExpectedContractRevision   int64
	ExpectedSettlementSequence int64
	SettlementDisposition      SettlementDisposition
	ClosedOn                   string
	Reason                     string
	Actor                      ActorRef
	FinalSettlementID          string
	OccurredAt                 time.Time
}

type ContractClosureResult struct {
	Closure  ContractClosure
	Replayed bool
}

type ContractClosureStateConflictError struct{ Status Status }

func (e *ContractClosureStateConflictError) Error() string {
	return "只有已激活或已终止合同可以执行操作性关闭"
}

type ContractClosureSettlementConflictError struct{ Message string }

func (e *ContractClosureSettlementConflictError) Error() string { return e.Message }

type ContractClosurePreconditionError struct {
	Field   string
	Message string
}

func (e *ContractClosurePreconditionError) Error() string { return e.Message }

func (s *Service) CloseContract(ctx context.Context, input CloseContractInput) (ContractClosureResult, error) {
	normalized, err := normalizeCloseContractInput(input)
	if err != nil {
		return ContractClosureResult{}, err
	}
	closureID, err := s.newID()
	if err != nil {
		return ContractClosureResult{}, fmt.Errorf("generate contract closure ID: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return ContractClosureResult{}, fmt.Errorf("generate contract closure audit ID: %w", err)
	}
	command := ContractClosureCommand{
		CommandID: normalized.CommandID, AuditID: auditID, ClosureID: closureID,
		ContractID:                 normalized.ContractID,
		ExpectedContractRevision:   normalized.ExpectedContractRevision,
		ExpectedSettlementSequence: normalized.ExpectedSettlementSequence,
		SettlementDisposition:      SettlementDisposition(normalized.SettlementDisposition),
		ClosedOn:                   normalized.ClosedOn, Reason: normalized.Reason,
		Actor: normalized.Actor, OccurredAt: s.now(),
	}
	persistedCommand, closure, created, err := s.repository.CloseContract(ctx, command)
	if err != nil {
		return ContractClosureResult{}, fmt.Errorf("close contract: %w", err)
	}
	if !created && !sameContractClosureIntent(persistedCommand, command) {
		return ContractClosureResult{}, &IdempotencyConflictError{CommandID: command.CommandID}
	}
	return ContractClosureResult{Closure: closure, Replayed: !created}, nil
}

func normalizeCloseContractInput(input CloseContractInput) (CloseContractInput, error) {
	commandID, err := normalizeUUID("command_id", "命令标识无效，请刷新页面后重试", input.CommandID)
	if err != nil {
		return CloseContractInput{}, err
	}
	contractID, err := normalizeUUID("contract_id", "合同标识无效", input.ContractID)
	if err != nil {
		return CloseContractInput{}, err
	}
	if input.ExpectedContractRevision < 1 {
		return CloseContractInput{}, &ValidationError{Field: "expected_revision", Message: "合同 revision 无效，请刷新页面后重试"}
	}
	if input.ExpectedSettlementSequence < 0 {
		return CloseContractInput{}, &ValidationError{Field: "expected_settlement_sequence", Message: "结算 sequence 无效，请刷新页面后重试"}
	}
	disposition := SettlementDisposition(strings.ToLower(strings.TrimSpace(input.SettlementDisposition)))
	if !disposition.Valid() {
		return CloseContractInput{}, &ValidationError{Field: "settlement_disposition", Message: "必须明确确认结算处置"}
	}
	closedOn, err := normalizeDate("closed_on", "关闭日期格式无效", input.ClosedOn)
	if err != nil {
		return CloseContractInput{}, err
	}
	if closedOn == "" {
		return CloseContractInput{}, &ValidationError{Field: "closed_on", Message: "关闭日期不能为空"}
	}
	reason, err := requiredText("closure_reason", "关闭依据或原因不能为空", input.Reason, 1000)
	if err != nil {
		return CloseContractInput{}, err
	}
	actor, err := normalizeActor(input.Actor)
	if err != nil {
		return CloseContractInput{}, err
	}
	return CloseContractInput{
		CommandID: commandID, ContractID: contractID,
		ExpectedContractRevision:   input.ExpectedContractRevision,
		ExpectedSettlementSequence: input.ExpectedSettlementSequence,
		SettlementDisposition:      string(disposition), ClosedOn: closedOn,
		Reason: reason, Actor: actor,
	}, nil
}

func sameContractClosureIntent(left, right ContractClosureCommand) bool {
	return left.CommandID == right.CommandID &&
		left.ContractID == right.ContractID &&
		left.ExpectedContractRevision == right.ExpectedContractRevision &&
		left.ExpectedSettlementSequence == right.ExpectedSettlementSequence &&
		left.SettlementDisposition == right.SettlementDisposition &&
		left.ClosedOn == right.ClosedOn &&
		left.Reason == right.Reason &&
		left.Actor == right.Actor
}
