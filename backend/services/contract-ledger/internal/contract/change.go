package contract

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"math/big"
	"regexp"
	"strings"
	"time"
)

const (
	InstrumentTypeOriginalContract      = "original_contract"
	InstrumentTypeSupplementalAgreement = "supplemental_agreement"

	AmountEffectOriginal    = "original"
	AmountEffectAdjustment  = "adjustment"
	AmountEffectRestatement = "restatement"
	AmountEffectUnchanged   = "unchanged"

	AuditEventContractChangeActivated = "contract_change_activated"
)

var signedAmountPattern = regexp.MustCompile(`^[+-]?[0-9]+(?:\.[0-9]+)?$`)

type ContractInstrument struct {
	ID                string
	ContractID        string
	InstrumentType    string
	InstrumentNumber  string
	Title             string
	ChangeSummary     string
	SignedOn          string
	EffectiveOn       string
	EffectiveOrder    int64
	AmountEffect      string
	AmountValue       string
	SourceCommandID   string
	ResultingRevision int64
	Actor             ActorRef
	RecordedAt        time.Time
}

type AppendContractChangeInput struct {
	CommandID        string
	ContractID       string
	ExpectedRevision int64
	InstrumentNumber string
	Title            string
	ChangeSummary    string
	SignedOn         string
	EffectiveOn      string
	EffectiveOrder   int64
	AmountEffect     string
	AmountValue      string
	Actor            ActorRef
}

type ContractChangeCommand struct {
	CommandID         string
	AuditID           string
	InstrumentID      string
	ContractID        string
	ExpectedRevision  int64
	InstrumentNumber  string
	Title             string
	ChangeSummary     string
	SignedOn          string
	EffectiveOn       string
	EffectiveOrder    int64
	AmountEffect      string
	AmountValue       string
	Actor             ActorRef
	ResultingRevision int64
	ResultingAmount   string
	OccurredAt        time.Time
}

type ContractChangeResult struct {
	ContractID      string
	InstrumentID    string
	Revision        int64
	EffectiveAmount string
	Replayed        bool
}

type ContractRevisionConflictError struct {
	Expected int64
	Actual   int64
}

type ContractChangeStateConflictError struct{ Status Status }

func (e *ContractChangeStateConflictError) Error() string {
	return "只有已激活合同可以登记生效补充协议"
}

func (e *ContractRevisionConflictError) Error() string {
	return fmt.Sprintf("合同 revision 已变化：提交基于 %d，当前为 %d，请刷新后重试", e.Expected, e.Actual)
}

type DuplicateInstrumentNumberError struct {
	InstrumentNumber string
}

func (e *DuplicateInstrumentNumberError) Error() string {
	return "该合同已存在相同文书编号"
}

type ContractChangePreconditionError struct {
	Field   string
	Message string
}

func (e *ContractChangePreconditionError) Error() string { return e.Message }

func (s *Service) AppendChange(ctx context.Context, input AppendContractChangeInput) (ContractChangeResult, error) {
	return s.appendChange(ctx, nil, input)
}

// AppendChangeInTransaction lets Contract's own typed overlay atomically append
// a native change and its intent mapping, without duplicating domain rules.
// It is an internal application seam, not a cross-application transaction API.
func (s *Service) AppendChangeInTransaction(ctx context.Context, tx pgx.Tx, input AppendContractChangeInput) (ContractChangeResult, error) {
	if tx == nil {
		return ContractChangeResult{}, fmt.Errorf("contract transaction required")
	}
	return s.appendChange(ctx, tx, input)
}

func (s *Service) appendChange(ctx context.Context, tx pgx.Tx, input AppendContractChangeInput) (ContractChangeResult, error) {
	normalized, err := normalizeAppendContractChangeInput(input)
	if err != nil {
		return ContractChangeResult{}, err
	}
	instrumentID, err := s.newID()
	if err != nil {
		return ContractChangeResult{}, fmt.Errorf("generate contract instrument ID: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return ContractChangeResult{}, fmt.Errorf("generate contract change audit ID: %w", err)
	}
	command := ContractChangeCommand{
		CommandID:         normalized.CommandID,
		AuditID:           auditID,
		InstrumentID:      instrumentID,
		ContractID:        normalized.ContractID,
		ExpectedRevision:  normalized.ExpectedRevision,
		InstrumentNumber:  normalized.InstrumentNumber,
		Title:             normalized.Title,
		ChangeSummary:     normalized.ChangeSummary,
		SignedOn:          normalized.SignedOn,
		EffectiveOn:       normalized.EffectiveOn,
		EffectiveOrder:    normalized.EffectiveOrder,
		AmountEffect:      normalized.AmountEffect,
		AmountValue:       normalized.AmountValue,
		Actor:             normalized.Actor,
		ResultingRevision: normalized.ExpectedRevision + 1,
		OccurredAt:        s.now(),
	}
	var persisted ContractChangeCommand
	var created bool
	if tx == nil {
		persisted, created, err = s.repository.AppendChange(ctx, command)
	} else {
		repository, ok := s.repository.(interface {
			AppendChangeInTransaction(context.Context, pgx.Tx, ContractChangeCommand) (ContractChangeCommand, bool, error)
		})
		if !ok {
			return ContractChangeResult{}, fmt.Errorf("transactional contract change repository required")
		}
		persisted, created, err = repository.AppendChangeInTransaction(ctx, tx, command)
	}
	if err != nil {
		return ContractChangeResult{}, fmt.Errorf("append contract change: %w", err)
	}
	if !created && !sameContractChangeIntent(persisted, command) {
		return ContractChangeResult{}, &IdempotencyConflictError{CommandID: command.CommandID}
	}
	return ContractChangeResult{
		ContractID:      persisted.ContractID,
		InstrumentID:    persisted.InstrumentID,
		Revision:        persisted.ResultingRevision,
		EffectiveAmount: persisted.ResultingAmount,
		Replayed:        !created,
	}, nil
}

func normalizeAppendContractChangeInput(input AppendContractChangeInput) (AppendContractChangeInput, error) {
	commandID, err := normalizeUUID("command_id", "命令标识无效，请刷新页面后重试", input.CommandID)
	if err != nil {
		return AppendContractChangeInput{}, err
	}
	contractID, err := normalizeUUID("contract_id", "合同标识无效", input.ContractID)
	if err != nil {
		return AppendContractChangeInput{}, err
	}
	if input.ExpectedRevision < 1 {
		return AppendContractChangeInput{}, &ValidationError{Field: "expected_revision", Message: "合同 revision 无效，请刷新页面后重试"}
	}
	if input.EffectiveOrder != input.ExpectedRevision+1 {
		return AppendContractChangeInput{}, &ValidationError{Field: "effective_order", Message: "生效顺序必须紧接当前合同 revision"}
	}
	instrumentNumber, err := requiredText("instrument_number", "补充协议编号不能为空", input.InstrumentNumber, 100)
	if err != nil {
		return AppendContractChangeInput{}, err
	}
	title, err := requiredText("instrument_title", "补充协议标题不能为空", input.Title, 200)
	if err != nil {
		return AppendContractChangeInput{}, err
	}
	changeSummary, err := requiredText("change_summary", "变更说明不能为空", input.ChangeSummary, 1000)
	if err != nil {
		return AppendContractChangeInput{}, err
	}
	signedOn, err := normalizeDate("instrument_signed_on", "补充协议签署日期格式无效", input.SignedOn)
	if err != nil {
		return AppendContractChangeInput{}, err
	}
	if signedOn == "" {
		return AppendContractChangeInput{}, &ValidationError{Field: "instrument_signed_on", Message: "补充协议签署日期不能为空"}
	}
	effectiveOn, err := normalizeDate("instrument_effective_on", "补充协议生效日期格式无效", input.EffectiveOn)
	if err != nil {
		return AppendContractChangeInput{}, err
	}
	if effectiveOn == "" {
		return AppendContractChangeInput{}, &ValidationError{Field: "instrument_effective_on", Message: "补充协议生效日期不能为空"}
	}
	amountEffect := strings.ToLower(strings.TrimSpace(input.AmountEffect))
	amountValue, err := normalizeChangeAmount(amountEffect, input.AmountValue)
	if err != nil {
		return AppendContractChangeInput{}, err
	}
	actor, err := normalizeActor(input.Actor)
	if err != nil {
		return AppendContractChangeInput{}, err
	}
	return AppendContractChangeInput{
		CommandID: commandID, ContractID: contractID, ExpectedRevision: input.ExpectedRevision,
		InstrumentNumber: instrumentNumber, Title: title, ChangeSummary: changeSummary,
		SignedOn: signedOn, EffectiveOn: effectiveOn, EffectiveOrder: input.EffectiveOrder,
		AmountEffect: amountEffect, AmountValue: amountValue, Actor: actor,
	}, nil
}

func normalizeChangeAmount(effect, rawValue string) (string, error) {
	switch effect {
	case AmountEffectAdjustment:
		value := strings.TrimSpace(rawValue)
		if !signedAmountPattern.MatchString(value) {
			return "", &ValidationError{Field: "amount_value", Message: "金额调整值必须是最多 18 位整数、6 位小数的有符号十进制数"}
		}
		sign := ""
		if value[0] == '-' || value[0] == '+' {
			sign = value[:1]
			value = value[1:]
		}
		normalized, err := normalizeAmount(value)
		if err != nil {
			return "", &ValidationError{Field: "amount_value", Message: "金额调整值必须是最多 18 位整数、6 位小数的有符号十进制数"}
		}
		if normalized == "0" {
			return "", &ValidationError{Field: "amount_value", Message: "金额调整值不能为零"}
		}
		if sign == "-" {
			return "-" + normalized, nil
		}
		return normalized, nil
	case AmountEffectRestatement:
		if strings.TrimSpace(rawValue) == "" {
			return "", &ValidationError{Field: "amount_value", Message: "金额重述必须填写新金额"}
		}
		normalized, err := normalizeAmount(rawValue)
		if err != nil {
			return "", &ValidationError{Field: "amount_value", Message: "重述金额必须是最多 18 位整数、6 位小数的非负十进制数"}
		}
		return normalized, nil
	case AmountEffectUnchanged:
		if strings.TrimSpace(rawValue) != "" {
			return "", &ValidationError{Field: "amount_value", Message: "金额不变时不能填写金额值"}
		}
		return "", nil
	default:
		return "", &ValidationError{Field: "amount_effect", Message: "请选择有效的金额影响语义"}
	}
}

func applyAmountEffect(current, effect, value string) (string, error) {
	switch effect {
	case AmountEffectUnchanged:
		return current, nil
	case AmountEffectRestatement:
		return value, nil
	case AmountEffectAdjustment:
		if current == "" {
			return "", &ContractChangePreconditionError{Field: "amount_value", Message: "当前有效金额尚未确定，不能使用金额调整；请使用金额重述"}
		}
		currentScaled, ok := decimalToScaledInteger(current)
		if !ok {
			return "", fmt.Errorf("stored current amount is invalid")
		}
		deltaScaled, ok := decimalToScaledInteger(value)
		if !ok {
			return "", fmt.Errorf("normalized adjustment amount is invalid")
		}
		result := new(big.Int).Add(currentScaled, deltaScaled)
		if result.Sign() < 0 {
			return "", &ContractChangePreconditionError{Field: "amount_value", Message: "金额调整后的有效金额不能小于零"}
		}
		normalized := scaledIntegerToDecimal(result)
		if _, err := normalizeAmount(normalized); err != nil {
			return "", &ContractChangePreconditionError{Field: "amount_value", Message: "金额调整后的有效金额超出精度边界"}
		}
		return normalized, nil
	default:
		return "", fmt.Errorf("unsupported amount effect %q", effect)
	}
}

func decimalToScaledInteger(value string) (*big.Int, bool) {
	raw := value
	sign := 1
	if strings.HasPrefix(raw, "-") {
		sign = -1
		raw = strings.TrimPrefix(raw, "-")
	}
	parts := strings.SplitN(raw, ".", 2)
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > 6 {
		return nil, false
	}
	fraction += strings.Repeat("0", 6-len(fraction))
	digits := strings.TrimLeft(parts[0]+fraction, "0")
	if digits == "" {
		digits = "0"
	}
	result := new(big.Int)
	if _, ok := result.SetString(digits, 10); !ok {
		return nil, false
	}
	if sign < 0 {
		result.Neg(result)
	}
	return result, true
}

func scaledIntegerToDecimal(value *big.Int) string {
	abs := new(big.Int).Abs(new(big.Int).Set(value))
	scale := big.NewInt(1_000_000)
	integer, fraction := new(big.Int), new(big.Int)
	integer.QuoRem(abs, scale, fraction)
	result := integer.String()
	if fraction.Sign() != 0 {
		fractionText := fraction.String()
		fractionText = strings.Repeat("0", 6-len(fractionText)) + fractionText
		result += "." + strings.TrimRight(fractionText, "0")
	}
	if value.Sign() < 0 {
		return "-" + result
	}
	return result
}

func sameContractChangeIntent(left, right ContractChangeCommand) bool {
	return left.CommandID == right.CommandID &&
		left.ContractID == right.ContractID &&
		left.ExpectedRevision == right.ExpectedRevision &&
		left.InstrumentNumber == right.InstrumentNumber &&
		left.Title == right.Title &&
		left.ChangeSummary == right.ChangeSummary &&
		left.SignedOn == right.SignedOn &&
		left.EffectiveOn == right.EffectiveOn &&
		left.EffectiveOrder == right.EffectiveOrder &&
		left.AmountEffect == right.AmountEffect &&
		left.AmountValue == right.AmountValue &&
		left.Actor == right.Actor &&
		left.ResultingRevision == right.ResultingRevision
}
