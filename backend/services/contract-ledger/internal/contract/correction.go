package contract

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	InstrumentTypeCorrection          = "correction_instrument"
	AmountEffectCorrected             = "corrected"
	AuditEventContractCorrectionAdded = "contract_correction_recorded"
)

type CorrectContractInput struct {
	CommandID             string
	ContractID            string
	ExpectedStatus        string
	ExpectedRevision      int64
	InstrumentNumber      string
	InstrumentTitle       string
	CorrectionReason      string
	InstrumentSignedOn    string
	InstrumentEffectiveOn string
	EffectiveOrder        int64
	CorrectedTitle        string
	CorrectedCategory     string
	CorrectedTotalAmount  string
	CorrectedSignedOn     string
	CorrectedEffectiveOn  string
	Actor                 ActorRef
}

type ContractCorrectionCommand struct {
	CommandID             string
	AuditID               string
	InstrumentID          string
	ContractID            string
	ExpectedStatus        Status
	ExpectedRevision      int64
	InstrumentNumber      string
	InstrumentTitle       string
	CorrectionReason      string
	InstrumentSignedOn    string
	InstrumentEffectiveOn string
	EffectiveOrder        int64
	CorrectedTitle        string
	CorrectedCategory     string
	CorrectedTotalAmount  string
	CorrectedSignedOn     string
	CorrectedEffectiveOn  string
	Actor                 ActorRef
	ResultingRevision     int64
	OccurredAt            time.Time
}

type ContractCorrectionResult struct {
	ContractID      string
	InstrumentID    string
	Revision        int64
	EffectiveAmount string
	Status          Status
	Replayed        bool
}

type ContractCorrectionStateConflictError struct {
	Expected Status
	Actual   Status
}

func (e *ContractCorrectionStateConflictError) Error() string {
	return fmt.Sprintf("合同状态已变化：提交基于 %s，当前为 %s，请刷新后确认是否仍需更正", e.Expected, e.Actual)
}

type ContractCorrectionPreconditionError struct {
	Field   string
	Message string
}

func (e *ContractCorrectionPreconditionError) Error() string { return e.Message }

type ContractCorrectionNoChangesError struct{}

func (e *ContractCorrectionNoChangesError) Error() string {
	return "更正后的合同核心字段没有变化"
}

func (s *Service) CorrectContract(ctx context.Context, input CorrectContractInput) (ContractCorrectionResult, error) {
	normalized, err := normalizeCorrectContractInput(input)
	if err != nil {
		return ContractCorrectionResult{}, err
	}
	instrumentID, err := s.newID()
	if err != nil {
		return ContractCorrectionResult{}, fmt.Errorf("generate correction instrument ID: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return ContractCorrectionResult{}, fmt.Errorf("generate contract correction audit ID: %w", err)
	}
	command := ContractCorrectionCommand{
		CommandID: normalized.CommandID, AuditID: auditID, InstrumentID: instrumentID,
		ContractID: normalized.ContractID, ExpectedStatus: Status(normalized.ExpectedStatus),
		ExpectedRevision: normalized.ExpectedRevision,
		InstrumentNumber: normalized.InstrumentNumber, InstrumentTitle: normalized.InstrumentTitle,
		CorrectionReason:   normalized.CorrectionReason,
		InstrumentSignedOn: normalized.InstrumentSignedOn, InstrumentEffectiveOn: normalized.InstrumentEffectiveOn,
		EffectiveOrder: normalized.EffectiveOrder,
		CorrectedTitle: normalized.CorrectedTitle, CorrectedCategory: normalized.CorrectedCategory,
		CorrectedTotalAmount: normalized.CorrectedTotalAmount,
		CorrectedSignedOn:    normalized.CorrectedSignedOn, CorrectedEffectiveOn: normalized.CorrectedEffectiveOn,
		Actor: normalized.Actor, ResultingRevision: normalized.ExpectedRevision + 1, OccurredAt: s.now(),
	}
	persisted, created, err := s.repository.CorrectContract(ctx, command)
	if err != nil {
		return ContractCorrectionResult{}, fmt.Errorf("correct contract: %w", err)
	}
	if !created && !sameContractCorrectionIntent(persisted, command) {
		return ContractCorrectionResult{}, &IdempotencyConflictError{CommandID: command.CommandID}
	}
	return ContractCorrectionResult{
		ContractID: persisted.ContractID, InstrumentID: persisted.InstrumentID,
		Revision: persisted.ResultingRevision, EffectiveAmount: persisted.CorrectedTotalAmount,
		Status: persisted.ExpectedStatus, Replayed: !created,
	}, nil
}

func normalizeCorrectContractInput(input CorrectContractInput) (CorrectContractInput, error) {
	commandID, err := normalizeUUID("command_id", "命令标识无效，请刷新页面后重试", input.CommandID)
	if err != nil {
		return CorrectContractInput{}, err
	}
	contractID, err := normalizeUUID("contract_id", "合同标识无效", input.ContractID)
	if err != nil {
		return CorrectContractInput{}, err
	}
	expectedStatus := Status(strings.ToLower(strings.TrimSpace(input.ExpectedStatus)))
	if expectedStatus != StatusActive && expectedStatus != StatusTerminated && expectedStatus != StatusClosed {
		return CorrectContractInput{}, &ValidationError{Field: "expected_status", Message: "只有已激活、已终止或已关闭合同可以受控更正"}
	}
	if input.ExpectedRevision < 1 {
		return CorrectContractInput{}, &ValidationError{Field: "expected_revision", Message: "合同 revision 无效，请刷新页面后重试"}
	}
	if input.EffectiveOrder != input.ExpectedRevision+1 {
		return CorrectContractInput{}, &ValidationError{Field: "effective_order", Message: "更正文书顺序必须紧接当前合同 revision"}
	}
	instrumentNumber, err := requiredText("correction_instrument_number", "更正文书编号不能为空", input.InstrumentNumber, 100)
	if err != nil {
		return CorrectContractInput{}, err
	}
	instrumentTitle, err := requiredText("correction_instrument_title", "更正文书标题不能为空", input.InstrumentTitle, 200)
	if err != nil {
		return CorrectContractInput{}, err
	}
	reason, err := requiredText("correction_reason", "更正原因或依据不能为空", input.CorrectionReason, 1000)
	if err != nil {
		return CorrectContractInput{}, err
	}
	instrumentSignedOn, err := requiredCorrectionDate("correction_instrument_signed_on", "更正文书签署或出具日期无效", input.InstrumentSignedOn)
	if err != nil {
		return CorrectContractInput{}, err
	}
	instrumentEffectiveOn, err := requiredCorrectionDate("correction_instrument_effective_on", "更正文书生效或确认日期无效", input.InstrumentEffectiveOn)
	if err != nil {
		return CorrectContractInput{}, err
	}
	correctedTitle, err := requiredText("corrected_title", "更正后合同标题不能为空", input.CorrectedTitle, 200)
	if err != nil {
		return CorrectContractInput{}, err
	}
	correctedCategory, err := optionalText("corrected_category", "更正后通用分类不能超过 100 个字符", input.CorrectedCategory, 100)
	if err != nil {
		return CorrectContractInput{}, err
	}
	correctedTotalAmount, err := normalizeAmount(input.CorrectedTotalAmount)
	if err != nil {
		var validation *ValidationError
		if errors.As(err, &validation) {
			return CorrectContractInput{}, &ValidationError{Field: "corrected_total_amount", Message: strings.Replace(validation.Message, "合同总额", "更正后合同总额", 1)}
		}
		return CorrectContractInput{}, err
	}
	correctedSignedOn, err := requiredCorrectionDate("corrected_signed_on", "更正后合同签署日期无效", input.CorrectedSignedOn)
	if err != nil {
		return CorrectContractInput{}, err
	}
	correctedEffectiveOn, err := requiredCorrectionDate("corrected_effective_on", "更正后合同生效日期无效", input.CorrectedEffectiveOn)
	if err != nil {
		return CorrectContractInput{}, err
	}
	actor, err := normalizeActor(input.Actor)
	if err != nil {
		return CorrectContractInput{}, err
	}
	return CorrectContractInput{
		CommandID: commandID, ContractID: contractID, ExpectedStatus: string(expectedStatus),
		ExpectedRevision: input.ExpectedRevision,
		InstrumentNumber: instrumentNumber, InstrumentTitle: instrumentTitle, CorrectionReason: reason,
		InstrumentSignedOn: instrumentSignedOn, InstrumentEffectiveOn: instrumentEffectiveOn,
		EffectiveOrder: input.EffectiveOrder,
		CorrectedTitle: correctedTitle, CorrectedCategory: correctedCategory,
		CorrectedTotalAmount: correctedTotalAmount,
		CorrectedSignedOn:    correctedSignedOn, CorrectedEffectiveOn: correctedEffectiveOn,
		Actor: actor,
	}, nil
}

func requiredCorrectionDate(field, message, value string) (string, error) {
	normalized, err := normalizeDate(field, message, value)
	if err != nil {
		return "", err
	}
	if normalized == "" {
		return "", &ValidationError{Field: field, Message: message}
	}
	return normalized, nil
}

func sameContractCorrectionIntent(left, right ContractCorrectionCommand) bool {
	return left.CommandID == right.CommandID && left.ContractID == right.ContractID &&
		left.ExpectedStatus == right.ExpectedStatus && left.ExpectedRevision == right.ExpectedRevision &&
		left.InstrumentNumber == right.InstrumentNumber && left.InstrumentTitle == right.InstrumentTitle &&
		left.CorrectionReason == right.CorrectionReason && left.InstrumentSignedOn == right.InstrumentSignedOn &&
		left.InstrumentEffectiveOn == right.InstrumentEffectiveOn && left.EffectiveOrder == right.EffectiveOrder &&
		left.CorrectedTitle == right.CorrectedTitle && left.CorrectedCategory == right.CorrectedCategory &&
		left.CorrectedTotalAmount == right.CorrectedTotalAmount && left.CorrectedSignedOn == right.CorrectedSignedOn &&
		left.CorrectedEffectiveOn == right.CorrectedEffectiveOn && left.Actor == right.Actor &&
		left.ResultingRevision == right.ResultingRevision
}
