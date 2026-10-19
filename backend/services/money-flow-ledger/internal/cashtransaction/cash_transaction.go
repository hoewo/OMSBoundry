package cashtransaction

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/party"
)

type Type string

const (
	TypeReceipt Type = "receipt"
	TypePayment Type = "payment"

	StatusRecorded = "recorded"
	StatusReversed = "reversed"

	SourceKindManual = "manual"

	AuditActionRecorded = "cash_transaction_recorded"
)

var (
	uuidPattern     = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
	amountPattern   = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?$`)
)

func (t Type) Label() string {
	switch t {
	case TypeReceipt:
		return "收款"
	case TypePayment:
		return "付款"
	default:
		return string(t)
	}
}

type PartySnapshot struct {
	PartyID                string
	PartyRevision          int64
	Kind                   party.Kind
	LegalName              string
	RegistrationIdentifier string
}

type Transaction struct {
	ID                           string
	Type                         Type
	Description                  string
	LedgerOwnerPartyID           string
	Payer                        PartySnapshot
	Payee                        PartySnapshot
	Amount                       string
	EffectiveAmount              string
	AllocatedAmount              string
	UnallocatedAmount            string
	Currency                     string
	OccurredOn                   string
	AccountReference             string
	SourceKind                   string
	ManualSourceReference        string
	ExternalTransactionReference string
	Status                       string
	RecordedBy                   audit.ActorRef
	RecordedAt                   time.Time
	Reversal                     *Reversal
}

func (t Transaction) LedgerOwner() PartySnapshot {
	if t.Payer.PartyID == t.LedgerOwnerPartyID {
		return t.Payer
	}
	return t.Payee
}

func (t Transaction) Counterparty() PartySnapshot {
	if t.Payer.PartyID == t.LedgerOwnerPartyID {
		return t.Payee
	}
	return t.Payer
}

type CreateInput struct {
	CommandID                    string
	Type                         string
	LedgerOwnerPartyID           string
	CounterpartyPartyID          string
	Description                  string
	Amount                       string
	Currency                     string
	OccurredOn                   string
	AccountReference             string
	ManualSourceReference        string
	ExternalTransactionReference string
	Actor                        audit.ActorRef
}

type CreateCommand struct {
	CommandID   string
	PayloadHash []byte
	Actor       audit.ActorRef
	CreatedAt   time.Time
}

type CreateResult struct {
	Transaction Transaction
	Replayed    bool
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

type IdempotencyConflictError struct {
	CommandID string
}

func (e *IdempotencyConflictError) Error() string {
	return "该 Command ID 已用于不同的实际收付款登记意图"
}

type ExternalReferenceConflictError struct {
	ExistingTransactionID string
}

func (e *ExternalReferenceConflictError) Error() string {
	return "该资金账户范围内已存在相同的外部流水号"
}

type PartyNotFoundError struct {
	PartyID string
}

func (e *PartyNotFoundError) Error() string {
	return "选择的主体不存在或已不可用"
}

type Repository interface {
	Create(context.Context, Transaction, CreateCommand, audit.Entry) (Transaction, bool, error)
	Reverse(context.Context, Reversal, ReverseCommand, audit.Entry) (Reversal, bool, error)
	List(context.Context) ([]Transaction, error)
	Get(context.Context, string) (Transaction, bool, error)
}

type Service struct {
	repository Repository
	now        func() time.Time
	newID      func() (string, error)
}

func NewService(repository Repository) *Service {
	return &Service{
		repository: repository,
		now:        func() time.Time { return time.Now().UTC() },
		newID:      randomUUID,
	}
}

func NewCommandID() (string, error) {
	return randomUUID()
}

func (s *Service) Create(ctx context.Context, input CreateInput) (CreateResult, error) {
	normalized, err := normalizeCreateInput(input)
	if err != nil {
		return CreateResult{}, err
	}
	payloadHash, err := createPayloadHash(normalized)
	if err != nil {
		return CreateResult{}, fmt.Errorf("hash cash transaction create intent: %w", err)
	}
	transactionID, err := s.newID()
	if err != nil {
		return CreateResult{}, fmt.Errorf("generate cash transaction ID: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return CreateResult{}, fmt.Errorf("generate cash transaction audit ID: %w", err)
	}
	now := s.now()
	created := Transaction{
		ID:                           transactionID,
		Type:                         Type(normalized.Type),
		Description:                  normalized.Description,
		LedgerOwnerPartyID:           normalized.LedgerOwnerPartyID,
		Amount:                       normalized.Amount,
		EffectiveAmount:              normalized.Amount,
		AllocatedAmount:              "0",
		UnallocatedAmount:            normalized.Amount,
		Currency:                     normalized.Currency,
		OccurredOn:                   normalized.OccurredOn,
		AccountReference:             normalized.AccountReference,
		SourceKind:                   SourceKindManual,
		ManualSourceReference:        normalized.ManualSourceReference,
		ExternalTransactionReference: normalized.ExternalTransactionReference,
		Status:                       StatusRecorded,
		RecordedBy:                   normalized.Actor,
		RecordedAt:                   now,
	}
	if created.Type == TypeReceipt {
		created.Payer.PartyID = normalized.CounterpartyPartyID
		created.Payee.PartyID = normalized.LedgerOwnerPartyID
	} else {
		created.Payer.PartyID = normalized.LedgerOwnerPartyID
		created.Payee.PartyID = normalized.CounterpartyPartyID
	}
	persisted, replayed, err := s.repository.Create(ctx, created, CreateCommand{
		CommandID:   normalized.CommandID,
		PayloadHash: payloadHash,
		Actor:       normalized.Actor,
		CreatedAt:   now,
	}, audit.Entry{
		ID:         auditID,
		ObjectType: audit.ObjectCashTransaction,
		ObjectID:   transactionID,
		Action:     AuditActionRecorded,
		CommandID:  normalized.CommandID,
		Actor:      normalized.Actor,
		OccurredAt: now,
		Summary:    "人工登记实际收付款",
		Details: map[string]any{
			"type":                           normalized.Type,
			"ledger_owner_party_id":          normalized.LedgerOwnerPartyID,
			"counterparty_party_id":          normalized.CounterpartyPartyID,
			"amount":                         normalized.Amount,
			"currency":                       normalized.Currency,
			"occurred_on":                    normalized.OccurredOn,
			"account_reference":              normalized.AccountReference,
			"source_kind":                    SourceKindManual,
			"manual_source_reference":        normalized.ManualSourceReference,
			"external_transaction_reference": normalized.ExternalTransactionReference,
			"resulting_status":               StatusRecorded,
		},
	})
	if err != nil {
		return CreateResult{}, fmt.Errorf("create cash transaction: %w", err)
	}
	return CreateResult{Transaction: persisted, Replayed: replayed}, nil
}

func (s *Service) List(ctx context.Context) ([]Transaction, error) {
	values, err := s.repository.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list cash transactions: %w", err)
	}
	return values, nil
}

func (s *Service) Get(ctx context.Context, transactionID string) (Transaction, bool, error) {
	id := strings.ToLower(strings.TrimSpace(transactionID))
	if !uuidPattern.MatchString(id) {
		return Transaction{}, false, &ValidationError{Field: "cash_transaction_id", Message: "实际收付款标识无效"}
	}
	value, found, err := s.repository.Get(ctx, id)
	if err != nil {
		return Transaction{}, false, fmt.Errorf("get cash transaction: %w", err)
	}
	return value, found, nil
}

func normalizeCreateInput(input CreateInput) (CreateInput, error) {
	commandID := strings.ToLower(strings.TrimSpace(input.CommandID))
	if !uuidPattern.MatchString(commandID) {
		return CreateInput{}, &ValidationError{Field: "command_id", Message: "Command ID 无效，请刷新页面后重试"}
	}
	actor, err := audit.NormalizeActor(input.Actor)
	if err != nil {
		return CreateInput{}, &ValidationError{Field: "actor", Message: err.Error()}
	}
	typeValue := Type(strings.ToLower(strings.TrimSpace(input.Type)))
	if typeValue != TypeReceipt && typeValue != TypePayment {
		return CreateInput{}, &ValidationError{Field: "type", Message: "请选择有效的收款或付款方向"}
	}
	ledgerOwnerPartyID, err := normalizePartyID("ledger_owner_party_id", input.LedgerOwnerPartyID)
	if err != nil {
		return CreateInput{}, err
	}
	counterpartyPartyID, err := normalizePartyID("counterparty_party_id", input.CounterpartyPartyID)
	if err != nil {
		return CreateInput{}, err
	}
	if ledgerOwnerPartyID == counterpartyPartyID {
		return CreateInput{}, &ValidationError{Field: "counterparty_party_id", Message: "本方账簿主体与往来主体必须不同"}
	}
	description, err := requiredText("description", "收付款描述不能为空", input.Description, 200)
	if err != nil {
		return CreateInput{}, err
	}
	amount, err := normalizeAmount(input.Amount)
	if err != nil {
		return CreateInput{}, err
	}
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if !currencyPattern.MatchString(currency) {
		return CreateInput{}, &ValidationError{Field: "currency", Message: "币种必须是三个英文字母"}
	}
	occurredOn := strings.TrimSpace(input.OccurredOn)
	parsedOccurredOn, parseErr := time.Parse("2006-01-02", occurredOn)
	if parseErr != nil || parsedOccurredOn.Year() < 1 || parsedOccurredOn.Format("2006-01-02") != occurredOn {
		return CreateInput{}, &ValidationError{Field: "occurred_on", Message: "资金发生日必须是有效的 YYYY-MM-DD 日期"}
	}
	accountReference, err := requiredText("account_reference", "资金账户标签不能为空", input.AccountReference, 100)
	if err != nil {
		return CreateInput{}, err
	}
	manualSourceReference, err := requiredText("manual_source_reference", "人工来源依据不能为空", input.ManualSourceReference, 200)
	if err != nil {
		return CreateInput{}, err
	}
	externalTransactionReference := strings.TrimSpace(input.ExternalTransactionReference)
	if len([]rune(externalTransactionReference)) > 100 {
		return CreateInput{}, &ValidationError{Field: "external_transaction_reference", Message: "外部流水号不能超过 100 个字符"}
	}
	return CreateInput{
		CommandID:                    commandID,
		Type:                         string(typeValue),
		LedgerOwnerPartyID:           ledgerOwnerPartyID,
		CounterpartyPartyID:          counterpartyPartyID,
		Description:                  description,
		Amount:                       amount,
		Currency:                     currency,
		OccurredOn:                   occurredOn,
		AccountReference:             accountReference,
		ManualSourceReference:        manualSourceReference,
		ExternalTransactionReference: externalTransactionReference,
		Actor:                        actor,
	}, nil
}

func normalizePartyID(field, value string) (string, error) {
	id := strings.ToLower(strings.TrimSpace(value))
	if !uuidPattern.MatchString(id) {
		return "", &ValidationError{Field: field, Message: "请选择有效的主体"}
	}
	return id, nil
}

func requiredText(field, emptyMessage, value string, maxLength int) (string, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", &ValidationError{Field: field, Message: emptyMessage}
	}
	if len([]rune(normalized)) > maxLength {
		return "", &ValidationError{Field: field, Message: fmt.Sprintf("内容不能超过 %d 个字符", maxLength)}
	}
	return normalized, nil
}

func normalizeAmount(value string) (string, error) {
	return normalizeAmountValue(value, true)
}

func normalizeAmountAllowZero(value string) (string, error) {
	return normalizeAmountValue(value, false)
}

func normalizeAmountValue(value string, requirePositive bool) (string, error) {
	raw := strings.TrimSpace(value)
	if !amountPattern.MatchString(raw) {
		return "", &ValidationError{Field: "amount", Message: "收付款金额必须是大于 0 的普通十进制数"}
	}
	parts := strings.SplitN(raw, ".", 2)
	integer := strings.TrimLeft(parts[0], "0")
	if integer == "" {
		integer = "0"
	}
	if len(integer) > 18 {
		return "", &ValidationError{Field: "amount", Message: "收付款金额整数部分不能超过 18 位"}
	}
	if len(parts) == 1 {
		if requirePositive && integer == "0" {
			return "", &ValidationError{Field: "amount", Message: "收付款金额必须大于 0"}
		}
		return integer, nil
	}
	if len(parts[1]) > 6 {
		return "", &ValidationError{Field: "amount", Message: "收付款金额小数部分不能超过 6 位"}
	}
	fraction := strings.TrimRight(parts[1], "0")
	if fraction == "" {
		if requirePositive && integer == "0" {
			return "", &ValidationError{Field: "amount", Message: "收付款金额必须大于 0"}
		}
		return integer, nil
	}
	return integer + "." + fraction, nil
}

func createPayloadHash(input CreateInput) ([]byte, error) {
	payload, err := json.Marshal(struct {
		Format                       string `json:"format"`
		Type                         string `json:"type"`
		LedgerOwnerPartyID           string `json:"ledger_owner_party_id"`
		CounterpartyPartyID          string `json:"counterparty_party_id"`
		Description                  string `json:"description"`
		Amount                       string `json:"amount"`
		Currency                     string `json:"currency"`
		OccurredOn                   string `json:"occurred_on"`
		AccountReferenceKey          string `json:"account_reference_key"`
		SourceKind                   string `json:"source_kind"`
		ManualSourceReference        string `json:"manual_source_reference"`
		ExternalTransactionReference string `json:"external_transaction_reference_key"`
		ActorKind                    string `json:"actor_kind"`
		ActorID                      string `json:"actor_id"`
	}{
		Format:                       "money-flow-cash-transaction-create-v1",
		Type:                         input.Type,
		LedgerOwnerPartyID:           input.LedgerOwnerPartyID,
		CounterpartyPartyID:          input.CounterpartyPartyID,
		Description:                  input.Description,
		Amount:                       input.Amount,
		Currency:                     input.Currency,
		OccurredOn:                   input.OccurredOn,
		AccountReferenceKey:          strings.ToLower(input.AccountReference),
		SourceKind:                   SourceKindManual,
		ManualSourceReference:        input.ManualSourceReference,
		ExternalTransactionReference: strings.ToLower(input.ExternalTransactionReference),
		ActorKind:                    input.Actor.Kind,
		ActorID:                      input.Actor.ID,
	})
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(payload)
	return hash[:], nil
}

func externalReferenceLockKey(value Transaction) (string, error) {
	if value.ExternalTransactionReference == "" {
		return "", nil
	}
	payload, err := json.Marshal(struct {
		Format              string `json:"format"`
		LedgerOwnerPartyID  string `json:"ledger_owner_party_id"`
		AccountReferenceKey string `json:"account_reference_key"`
		SourceKind          string `json:"source_kind"`
		ExternalReference   string `json:"external_reference_key"`
	}{
		Format:              "money-flow-cash-external-reference-v1",
		LedgerOwnerPartyID:  value.LedgerOwnerPartyID,
		AccountReferenceKey: strings.ToLower(value.AccountReference),
		SourceKind:          value.SourceKind,
		ExternalReference:   strings.ToLower(value.ExternalTransactionReference),
	})
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(payload)
	return hex.EncodeToString(hash[:]), nil
}

func randomUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		value[0:4], value[4:6], value[6:8], value[8:10], value[10:16],
	), nil
}
