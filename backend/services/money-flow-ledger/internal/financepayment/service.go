package financepayment

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
	"omsbizfoundry/money-flow-ledger/internal/party"
)

var (
	ErrInvalid         = errors.New("invalid internal finance payment")
	ErrOwnerMissing    = errors.New("internal finance ledger owner is not configured")
	ErrPayeeMapping    = errors.New("internal finance payee is not mapped to a Money party")
	ErrCommandConflict = errors.New("internal finance payment command already has a different request")
	uuidPattern        = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	digestPattern      = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type ReferenceSnapshot struct {
	ID          string `json:"id"`
	Revision    int64  `json:"revision"`
	Digest      string `json:"digest"`
	DisplayName string `json:"display_name"`
}
type PayeeSnapshot struct {
	Kind         string            `json:"kind"`
	EntityKind   string            `json:"entity_kind,omitempty"`
	Reference    ReferenceSnapshot `json:"reference"`
	DisplayName  string            `json:"display_name"`
	BankName     string            `json:"bank_name"`
	AccountLast4 string            `json:"account_last4"`
}
type Request struct {
	SchemaVersion       string        `json:"schema_version"`
	CommandID           string        `json:"command_id"`
	TenantID            string        `json:"tenant_id"`
	ApplicationID       string        `json:"application_id"`
	ApplicationRevision int64         `json:"application_revision"`
	ApplicationDigest   string        `json:"application_digest"`
	RequestType         string        `json:"request_type"`
	Title               string        `json:"title"`
	TotalPayableMinor   int64         `json:"total_payable_minor"`
	ActualAmountMinor   int64         `json:"actual_amount_minor"`
	OccurredOn          string        `json:"occurred_on"`
	Note                string        `json:"note"`
	Payee               PayeeSnapshot `json:"payee"`
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
	ObligationID, ApplicationID           string
	ObligationRevision, TotalPayableMinor int64
}
type Resolution struct {
	OwnerPartyID, PayeePartyID string
	Binding                    *Binding
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
type PartyUseCases interface {
	Create(context.Context, party.CreateInput) (party.CreateResult, error)
}
type Service struct {
	repository  Repository
	obligations ObligationUseCases
	cash        CashUseCases
	allocations AllocationUseCases
	parties     PartyUseCases
}

func NewService(repository Repository, obligations ObligationUseCases, cash CashUseCases, allocations AllocationUseCases, parties PartyUseCases) (*Service, error) {
	if repository == nil || obligations == nil || cash == nil || allocations == nil || parties == nil {
		return nil, fmt.Errorf("internal finance payment dependencies are required")
	}
	return &Service{repository: repository, obligations: obligations, cash: cash, allocations: allocations, parties: parties}, nil
}

func (s *Service) Record(ctx context.Context, actor audit.ActorRef, input Request) (Result, error) {
	v, err := normalize(input)
	if err != nil {
		return Result{}, err
	}
	actor, err = audit.NormalizeActor(actor)
	if err != nil {
		return Result{}, ErrInvalid
	}
	requestDigest, err := digest(struct {
		Request   Request `json:"request"`
		ActorKind string  `json:"actor_kind"`
		ActorID   string  `json:"actor_id"`
	}{v, actor.Kind, actor.ID})
	if err != nil {
		return Result{}, err
	}
	if replay, found, replayErr := s.repository.Replay(ctx, v.TenantID, v.CommandID, requestDigest); replayErr != nil || found {
		replay.Replayed = found
		return replay, replayErr
	}
	resolved, err := s.repository.Resolve(ctx, v)
	if err != nil {
		return Result{}, err
	}
	if v.Payee.Kind == "one_time" && resolved.PayeePartyID == "" {
		created, createErr := s.parties.Create(ctx, party.CreateInput{CommandID: deterministicUUID(v.TenantID + ":" + v.ApplicationID + ":" + v.Payee.DisplayName + ":one-time-payee"), Kind: v.Payee.EntityKind, LegalName: v.Payee.DisplayName, RegistrationIdentifier: "finance-one-time-" + v.ApplicationID, Actor: actor})
		if createErr != nil {
			return Result{}, createErr
		}
		resolved.PayeePartyID = created.Party.ID
	}
	if resolved.PayeePartyID == "" {
		return Result{}, ErrPayeeMapping
	}
	binding := resolved.Binding
	if binding == nil {
		draft, createErr := s.obligations.CreateSuggestedDraft(ctx, obligation.CreateSuggestedDraftInput{CommandID: deterministicUUID(v.TenantID + ":" + v.ApplicationID + ":finance-obligation-create"), LedgerOwnerPartyID: resolved.OwnerPartyID, CounterpartyPartyID: resolved.PayeePartyID, Title: "财务申请：" + v.Title, OriginalAmount: minorToDecimal(v.TotalPayableMinor), Currency: "CNY", DueOn: v.OccurredOn, Source: obligation.SuggestedSource{Application: "internal-finance-requests", ResourceType: "finance_request", ResourceID: v.ApplicationID, Revision: v.ApplicationRevision, Digest: v.ApplicationDigest}, Actor: actor})
		if createErr != nil {
			return Result{}, createErr
		}
		confirmed, confirmErr := s.obligations.Confirm(ctx, obligation.ConfirmInput{CommandID: deterministicUUID(v.TenantID + ":" + v.ApplicationID + ":finance-obligation-confirm"), ObligationID: draft.Obligation.ID, ExpectedDraftVersion: draft.Obligation.DraftVersion, Actor: actor})
		if confirmErr != nil {
			return Result{}, confirmErr
		}
		stored, bindErr := s.repository.Bind(ctx, v, confirmed.Obligation.ID, confirmed.Revision.Revision)
		if bindErr != nil {
			return Result{}, bindErr
		}
		binding = &stored
	}
	if binding.TotalPayableMinor != v.TotalPayableMinor {
		return Result{}, ErrInvalid
	}
	details, found, err := s.obligations.GetDetails(ctx, binding.ObligationID)
	if err != nil || !found {
		return Result{}, fmt.Errorf("load internal finance obligation: %w", err)
	}
	outstanding, err := decimalToMinor(details.Obligation.OutstandingAmount)
	if err != nil {
		return Result{}, err
	}
	accountReference := strings.TrimSpace(v.Payee.BankName + " 尾号" + v.Payee.AccountLast4)
	if accountReference == "尾号" {
		accountReference = "财务申请付款账户"
	}
	if len([]rune(accountReference)) > 100 {
		accountReference = string([]rune(accountReference)[:100])
	}
	cashResult, err := s.cash.Create(ctx, cashtransaction.CreateInput{CommandID: deterministicUUID(v.TenantID + ":" + v.CommandID + ":finance-cash"), Type: string(cashtransaction.TypePayment), LedgerOwnerPartyID: resolved.OwnerPartyID, CounterpartyPartyID: resolved.PayeePartyID, Description: "财务申请实付：" + v.Title, Amount: minorToDecimal(v.ActualAmountMinor), Currency: "CNY", OccurredOn: v.OccurredOn, AccountReference: accountReference, ManualSourceReference: "Internal Finance " + v.ApplicationID + " / " + v.CommandID, ExternalTransactionReference: v.CommandID, Actor: actor})
	if err != nil {
		return Result{}, err
	}
	allocated := v.ActualAmountMinor
	if allocated > outstanding {
		allocated = outstanding
	}
	if allocated < 0 {
		allocated = 0
	}
	result := Result{Status: "recorded", MoneyPaymentID: cashResult.Transaction.ID, MoneyPaymentRevision: 1, MoneyObligationID: binding.ObligationID, MoneyObligationRevision: details.Obligation.CurrentRevision, AllocatedAmountMinor: allocated, OverageAmountMinor: v.ActualAmountMinor - allocated}
	if allocated > 0 {
		allocationResult, applyErr := s.allocations.Apply(ctx, allocation.ApplyInput{CommandID: deterministicUUID(v.TenantID + ":" + v.CommandID + ":finance-allocation"), CashTransactionID: cashResult.Transaction.ID, ObligationID: binding.ObligationID, ExpectedObligationRevision: details.Obligation.CurrentRevision, Amount: minorToDecimal(allocated), ManualSourceReference: "Internal Finance " + v.ApplicationID + " / " + v.CommandID, Actor: actor})
		if applyErr != nil {
			return Result{}, applyErr
		}
		result.MoneyAllocationID, result.MoneyAllocationRevision = allocationResult.Allocation.ID, 1
	}
	if err = s.repository.Record(ctx, v, requestDigest, result, actor); err != nil {
		if replay, found, replayErr := s.repository.Replay(ctx, v.TenantID, v.CommandID, requestDigest); replayErr == nil && found {
			replay.Replayed = true
			return replay, nil
		}
		return Result{}, err
	}
	return result, nil
}

func normalize(v Request) (Request, error) {
	v.SchemaVersion = strings.TrimSpace(v.SchemaVersion)
	v.CommandID, v.TenantID, v.ApplicationID = normalizeID(v.CommandID), normalizeID(v.TenantID), normalizeID(v.ApplicationID)
	v.ApplicationDigest = strings.ToLower(strings.TrimSpace(v.ApplicationDigest))
	v.RequestType = strings.ToLower(strings.TrimSpace(v.RequestType))
	v.Title, v.OccurredOn, v.Note = strings.TrimSpace(v.Title), strings.TrimSpace(v.OccurredOn), strings.TrimSpace(v.Note)
	v.Payee.Kind = strings.ToLower(strings.TrimSpace(v.Payee.Kind))
	v.Payee.EntityKind = strings.ToLower(strings.TrimSpace(v.Payee.EntityKind))
	v.Payee.Reference.ID = normalizeID(v.Payee.Reference.ID)
	v.Payee.Reference.Digest = strings.ToLower(strings.TrimSpace(v.Payee.Reference.Digest))
	v.Payee.DisplayName, v.Payee.BankName, v.Payee.AccountLast4 = strings.TrimSpace(v.Payee.DisplayName), strings.TrimSpace(v.Payee.BankName), strings.TrimSpace(v.Payee.AccountLast4)
	if v.SchemaVersion != "1" || !uuidPattern.MatchString(v.CommandID) || !uuidPattern.MatchString(v.TenantID) || !uuidPattern.MatchString(v.ApplicationID) || v.ApplicationRevision < 1 || !digestPattern.MatchString(v.ApplicationDigest) || (v.RequestType != "reimbursement" && v.RequestType != "payment") || v.Title == "" || len([]rune(v.Title)) > 200 || v.TotalPayableMinor < 1 || v.ActualAmountMinor < 1 || v.Payee.DisplayName == "" || len([]rune(v.Payee.DisplayName)) > 200 || len([]rune(v.Note)) > 500 {
		return Request{}, ErrInvalid
	}
	if parsed, parseErr := time.Parse("2006-01-02", v.OccurredOn); parseErr != nil || parsed.Format("2006-01-02") != v.OccurredOn {
		return Request{}, ErrInvalid
	}
	switch v.Payee.Kind {
	case "person", "party":
		if !uuidPattern.MatchString(v.Payee.Reference.ID) || v.Payee.Reference.Revision < 1 || !digestPattern.MatchString(v.Payee.Reference.Digest) {
			return Request{}, ErrInvalid
		}
	case "one_time":
		if v.Payee.EntityKind != string(party.KindOrganization) && v.Payee.EntityKind != string(party.KindIndividual) {
			return Request{}, ErrInvalid
		}
	default:
		return Request{}, ErrInvalid
	}
	return v, nil
}
func normalizeID(v string) string { return strings.ToLower(strings.TrimSpace(v)) }
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
