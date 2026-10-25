package payrollpayment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/allocation"
	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/cashtransaction"
	"omsbizfoundry/money-flow-ledger/internal/obligation"
)

var (
	ErrInvalid         = errors.New("invalid payroll payment")
	ErrOwnerMissing    = errors.New("payroll ledger owner is not configured")
	ErrPersonMapping   = errors.New("payroll person is not mapped to a Money party")
	ErrCommandConflict = errors.New("payroll payment command already has a different request")
	uuidPattern        = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	digestPattern      = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type Request struct {
	SchemaVersion          string `json:"schema_version"`
	CommandID              string `json:"command_id"`
	TenantID               string `json:"tenant_id"`
	PayrollRunID           string `json:"payroll_run_id"`
	PayrollPeriod          string `json:"payroll_period"`
	PayrollLabel           string `json:"payroll_label"`
	PayrollLineID          string `json:"payroll_line_id"`
	PayrollLineRevision    int64  `json:"payroll_line_revision"`
	PayrollLineDigest      string `json:"payroll_line_digest"`
	PersonID               string `json:"person_id"`
	PersonRevision         int64  `json:"person_revision"`
	PersonDigest           string `json:"person_digest"`
	PersonDisplayName      string `json:"person_display_name"`
	TotalNetPayableMinor   int64  `json:"total_net_payable_minor"`
	ActualAmountMinor      int64  `json:"actual_amount_minor"`
	OccurredOn             string `json:"occurred_on"`
	Note                   string `json:"note"`
	PaymentProfileID       string `json:"payment_profile_id"`
	PaymentProfileRevision int64  `json:"payment_profile_revision"`
	PaymentBankName        string `json:"payment_bank_name"`
	MaskedPaymentAccount   string `json:"masked_payment_account"`
}

type Result struct {
	Status                  string `json:"status"`
	MoneyPaymentID          string `json:"money_payment_id"`
	MoneyPaymentRevision    int64  `json:"money_payment_revision"`
	MoneyAllocationID       string `json:"money_allocation_id,omitempty"`
	MoneyAllocationRevision int64  `json:"money_allocation_revision,omitempty"`
	MoneyObligationID       string `json:"money_obligation_id"`
	MoneyObligationRevision int64  `json:"money_obligation_revision"`
	AllocatedAmountMinor    int64  `json:"allocated_amount_minor"`
	OverageAmountMinor      int64  `json:"overage_amount_minor"`
	Replayed                bool   `json:"replayed"`
}

type Binding struct {
	ObligationID, PayrollLineID              string
	ObligationRevision, TotalNetPayableMinor int64
}

type Resolution struct {
	OwnerPartyID, PersonPartyID string
	Binding                     *Binding
}

type Repository interface {
	Replay(context.Context, string, string, string) (Result, bool, error)
	Resolve(context.Context, Request) (Resolution, error)
	Bind(context.Context, Request, string, int64) (Binding, error)
	Record(context.Context, Request, string, Result, audit.ActorRef) error
}
type ObligationUseCases interface {
	CreateSuggestedDraft(context.Context, obligation.CreateSuggestedDraftInput) (obligation.CreateResult, error)
	Confirm(context.Context, obligation.ConfirmInput) (obligation.ConfirmResult, error)
	GetDetails(context.Context, string) (obligation.Details, bool, error)
}
type CashUseCases interface {
	Create(context.Context, cashtransaction.CreateInput) (cashtransaction.CreateResult, error)
}
type AllocationUseCases interface {
	Apply(context.Context, allocation.ApplyInput) (allocation.ApplyResult, error)
}

type Service struct {
	repository  Repository
	obligations ObligationUseCases
	cash        CashUseCases
	allocations AllocationUseCases
}

func NewService(repository Repository, obligations ObligationUseCases, cash CashUseCases, allocations AllocationUseCases) (*Service, error) {
	if repository == nil || obligations == nil || cash == nil || allocations == nil {
		return nil, fmt.Errorf("payroll payment dependencies are required")
	}
	return &Service{repository: repository, obligations: obligations, cash: cash, allocations: allocations}, nil
}

func (s *Service) Record(ctx context.Context, actor audit.ActorRef, input Request) (Result, error) {
	value, err := normalize(input)
	if err != nil {
		return Result{}, err
	}
	actor, err = audit.NormalizeActor(actor)
	if err != nil {
		return Result{}, ErrInvalid
	}
	requestDigest, err := digest(struct {
		Request            Request `json:"request"`
		ActorKind, ActorID string
	}{value, actor.Kind, actor.ID})
	if err != nil {
		return Result{}, err
	}
	if replay, found, replayErr := s.repository.Replay(ctx, value.TenantID, value.CommandID, requestDigest); replayErr != nil || found {
		replay.Replayed = found
		return replay, replayErr
	}
	resolved, err := s.repository.Resolve(ctx, value)
	if err != nil {
		return Result{}, err
	}
	binding := resolved.Binding
	if binding == nil {
		draft, createErr := s.obligations.CreateSuggestedDraft(ctx, obligation.CreateSuggestedDraftInput{
			CommandID:          deterministicUUID(value.TenantID + ":" + value.PayrollLineID + ":payroll-obligation-create"),
			LedgerOwnerPartyID: resolved.OwnerPartyID, CounterpartyPartyID: resolved.PersonPartyID,
			Title:          "工资：" + value.PersonDisplayName + " " + value.PayrollPeriod,
			OriginalAmount: minorToDecimal(value.TotalNetPayableMinor), Currency: "CNY", DueOn: value.OccurredOn,
			Source: obligation.SuggestedSource{Application: "payroll", ResourceType: "payroll_line", ResourceID: value.PayrollLineID, Revision: value.PayrollLineRevision, Digest: value.PayrollLineDigest}, Actor: actor,
		})
		if createErr != nil {
			return Result{}, createErr
		}
		confirmed, confirmErr := s.obligations.Confirm(ctx, obligation.ConfirmInput{CommandID: deterministicUUID(value.TenantID + ":" + value.PayrollLineID + ":payroll-obligation-confirm"), ObligationID: draft.Obligation.ID, ExpectedDraftVersion: draft.Obligation.DraftVersion, Actor: actor})
		if confirmErr != nil {
			return Result{}, confirmErr
		}
		stored, bindErr := s.repository.Bind(ctx, value, confirmed.Obligation.ID, confirmed.Revision.Revision)
		if bindErr != nil {
			return Result{}, bindErr
		}
		binding = &stored
	}
	if binding.TotalNetPayableMinor != value.TotalNetPayableMinor {
		return Result{}, ErrInvalid
	}
	details, found, err := s.obligations.GetDetails(ctx, binding.ObligationID)
	if err != nil || !found {
		return Result{}, fmt.Errorf("load payroll obligation: %w", err)
	}
	outstanding, err := decimalToMinor(details.Obligation.OutstandingAmount)
	if err != nil {
		return Result{}, err
	}
	accountReference := strings.TrimSpace(value.PaymentBankName + " " + value.MaskedPaymentAccount)
	if accountReference == "" {
		accountReference = "工资付款账户"
	}
	if len([]rune(accountReference)) > 100 {
		accountReference = string([]rune(accountReference)[:100])
	}
	cashResult, err := s.cash.Create(ctx, cashtransaction.CreateInput{
		CommandID: deterministicUUID(value.TenantID + ":" + value.CommandID + ":payroll-cash"), Type: string(cashtransaction.TypePayment),
		LedgerOwnerPartyID: resolved.OwnerPartyID, CounterpartyPartyID: resolved.PersonPartyID,
		Description: "工资实付：" + value.PersonDisplayName + " " + value.PayrollPeriod, Amount: minorToDecimal(value.ActualAmountMinor), Currency: "CNY", OccurredOn: value.OccurredOn,
		AccountReference: accountReference, ManualSourceReference: "Payroll " + value.PayrollLineID + " / " + value.CommandID,
		ExternalTransactionReference: value.CommandID, Actor: actor,
	})
	if err != nil {
		return Result{}, err
	}
	allocated := value.ActualAmountMinor
	if allocated > outstanding {
		allocated = outstanding
	}
	if allocated < 0 {
		allocated = 0
	}
	result := Result{Status: "succeeded", MoneyPaymentID: cashResult.Transaction.ID, MoneyPaymentRevision: 1, MoneyObligationID: binding.ObligationID, MoneyObligationRevision: details.Obligation.CurrentRevision, AllocatedAmountMinor: allocated, OverageAmountMinor: value.ActualAmountMinor - allocated}
	if allocated > 0 {
		allocationResult, applyErr := s.allocations.Apply(ctx, allocation.ApplyInput{
			CommandID: deterministicUUID(value.TenantID + ":" + value.CommandID + ":payroll-allocation"), CashTransactionID: cashResult.Transaction.ID,
			ObligationID: binding.ObligationID, ExpectedObligationRevision: details.Obligation.CurrentRevision, Amount: minorToDecimal(allocated),
			ManualSourceReference: "Payroll " + value.PayrollLineID + " / " + value.CommandID, Actor: actor,
		})
		if applyErr != nil {
			return Result{}, applyErr
		}
		result.MoneyAllocationID, result.MoneyAllocationRevision = allocationResult.Allocation.ID, 1
	}
	if err = s.repository.Record(ctx, value, requestDigest, result, actor); err != nil {
		if replay, found, replayErr := s.repository.Replay(ctx, value.TenantID, value.CommandID, requestDigest); replayErr == nil && found {
			replay.Replayed = true
			return replay, nil
		}
		return Result{}, err
	}
	return result, nil
}

func normalize(v Request) (Request, error) {
	v.SchemaVersion = strings.TrimSpace(v.SchemaVersion)
	v.CommandID, v.TenantID, v.PayrollRunID, v.PayrollLineID, v.PersonID, v.PaymentProfileID = strings.ToLower(strings.TrimSpace(v.CommandID)), strings.ToLower(strings.TrimSpace(v.TenantID)), strings.ToLower(strings.TrimSpace(v.PayrollRunID)), strings.ToLower(strings.TrimSpace(v.PayrollLineID)), strings.ToLower(strings.TrimSpace(v.PersonID)), strings.ToLower(strings.TrimSpace(v.PaymentProfileID))
	v.PayrollLineDigest, v.PersonDigest = strings.ToLower(strings.TrimSpace(v.PayrollLineDigest)), strings.ToLower(strings.TrimSpace(v.PersonDigest))
	v.PayrollPeriod, v.PayrollLabel, v.PersonDisplayName, v.OccurredOn, v.Note, v.PaymentBankName, v.MaskedPaymentAccount = strings.TrimSpace(v.PayrollPeriod), strings.TrimSpace(v.PayrollLabel), strings.TrimSpace(v.PersonDisplayName), strings.TrimSpace(v.OccurredOn), strings.TrimSpace(v.Note), strings.TrimSpace(v.PaymentBankName), strings.TrimSpace(v.MaskedPaymentAccount)
	if v.SchemaVersion != "1" || !uuidPattern.MatchString(v.CommandID) || !uuidPattern.MatchString(v.TenantID) || !uuidPattern.MatchString(v.PayrollRunID) || !uuidPattern.MatchString(v.PayrollLineID) || !uuidPattern.MatchString(v.PersonID) || !uuidPattern.MatchString(v.PaymentProfileID) || v.PayrollLineRevision < 1 || v.PersonRevision < 1 || v.PaymentProfileRevision < 1 || !digestPattern.MatchString(v.PayrollLineDigest) || !digestPattern.MatchString(v.PersonDigest) || v.TotalNetPayableMinor <= 0 || v.ActualAmountMinor <= 0 || len([]rune(v.PersonDisplayName)) < 1 || len([]rune(v.PersonDisplayName)) > 200 || len([]rune(v.PayrollLabel)) > 200 || len([]rune(v.Note)) > 500 {
		return Request{}, ErrInvalid
	}
	if parsed, parseErr := time.Parse("2006-01-02", v.OccurredOn); parseErr != nil || parsed.Format("2006-01-02") != v.OccurredOn {
		return Request{}, ErrInvalid
	}
	if len(v.PayrollPeriod) != 7 {
		return Request{}, ErrInvalid
	}
	return v, nil
}

func digest(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
func deterministicUUID(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	sum[6] = (sum[6] & 0x0f) | 0x40
	sum[8] = (sum[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}
func minorToDecimal(v int64) string {
	return strconv.FormatInt(v/100, 10) + "." + fmt.Sprintf("%02d", v%100)
}
func decimalToMinor(value string) (int64, error) {
	rat, ok := new(big.Rat).SetString(value)
	if !ok {
		return 0, fmt.Errorf("invalid Money amount")
	}
	rat.Mul(rat, big.NewRat(100, 1))
	if !rat.IsInt() || !rat.Num().IsInt64() {
		return 0, fmt.Errorf("Money amount does not use minor-unit precision")
	}
	return rat.Num().Int64(), nil
}
