package internaltransfer

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

const (
	StatusRecorded = "recorded"

	SourceKindManual = "manual"

	AuditActionRecorded = "internal_transfer_recorded"
)

var (
	uuidPattern     = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
	amountPattern   = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?$`)
)

type PartySnapshot struct {
	PartyID                string
	PartyRevision          int64
	Kind                   party.Kind
	LegalName              string
	RegistrationIdentifier string
}

type Transfer struct {
	ID                          string
	Description                 string
	LedgerOwner                 PartySnapshot
	Amount                      string
	Currency                    string
	OccurredOn                  string
	SourceAccountReference      string
	DestinationAccountReference string
	SourceKind                  string
	ManualSourceReference       string
	ExternalTransferReference   string
	Status                      string
	RecordedBy                  audit.ActorRef
	RecordedAt                  time.Time
}

type CreateInput struct {
	CommandID                   string
	LedgerOwnerPartyID          string
	Description                 string
	Amount                      string
	Currency                    string
	OccurredOn                  string
	SourceAccountReference      string
	DestinationAccountReference string
	ManualSourceReference       string
	ExternalTransferReference   string
	Actor                       audit.ActorRef
}

type CreateCommand struct {
	CommandID   string
	PayloadHash []byte
	Actor       audit.ActorRef
	CreatedAt   time.Time
}

type CreateResult struct {
	Transfer Transfer
	Replayed bool
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
	return "该 Command ID 已用于不同的内部账户转账登记意图"
}

type ExternalReferenceConflictError struct {
	ExistingTransferID string
}

func (e *ExternalReferenceConflictError) Error() string {
	return "该账户转账范围内已存在相同的外部转账参考"
}

type PartyNotFoundError struct {
	PartyID string
}

func (e *PartyNotFoundError) Error() string {
	return "选择的本方账簿主体不存在或已不可用"
}

type Repository interface {
	Create(context.Context, Transfer, CreateCommand, audit.Entry) (Transfer, bool, error)
	List(context.Context) ([]Transfer, error)
	Get(context.Context, string) (Transfer, bool, error)
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
		return CreateResult{}, fmt.Errorf("hash internal transfer create intent: %w", err)
	}
	transferID, err := s.newID()
	if err != nil {
		return CreateResult{}, fmt.Errorf("generate internal transfer ID: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return CreateResult{}, fmt.Errorf("generate internal transfer audit ID: %w", err)
	}
	now := s.now()
	created := Transfer{
		ID:                          transferID,
		Description:                 normalized.Description,
		LedgerOwner:                 PartySnapshot{PartyID: normalized.LedgerOwnerPartyID},
		Amount:                      normalized.Amount,
		Currency:                    normalized.Currency,
		OccurredOn:                  normalized.OccurredOn,
		SourceAccountReference:      normalized.SourceAccountReference,
		DestinationAccountReference: normalized.DestinationAccountReference,
		SourceKind:                  SourceKindManual,
		ManualSourceReference:       normalized.ManualSourceReference,
		ExternalTransferReference:   normalized.ExternalTransferReference,
		Status:                      StatusRecorded,
		RecordedBy:                  normalized.Actor,
		RecordedAt:                  now,
	}
	persisted, replayed, err := s.repository.Create(ctx, created, CreateCommand{
		CommandID:   normalized.CommandID,
		PayloadHash: payloadHash,
		Actor:       normalized.Actor,
		CreatedAt:   now,
	}, audit.Entry{
		ID:         auditID,
		ObjectType: audit.ObjectInternalTransfer,
		ObjectID:   transferID,
		Action:     AuditActionRecorded,
		CommandID:  normalized.CommandID,
		Actor:      normalized.Actor,
		OccurredAt: now,
		Summary:    "人工登记内部账户转账",
		Details: map[string]any{
			"ledger_owner_party_id":              normalized.LedgerOwnerPartyID,
			"amount":                             normalized.Amount,
			"currency":                           normalized.Currency,
			"occurred_on":                        normalized.OccurredOn,
			"source_account_reference":           normalized.SourceAccountReference,
			"destination_account_reference":      normalized.DestinationAccountReference,
			"source_kind":                        SourceKindManual,
			"manual_source_reference":            normalized.ManualSourceReference,
			"external_transfer_reference":        normalized.ExternalTransferReference,
			"operating_cash_flow_classification": "excluded_internal_transfer",
			"resulting_status":                   StatusRecorded,
		},
	})
	if err != nil {
		return CreateResult{}, fmt.Errorf("create internal transfer: %w", err)
	}
	return CreateResult{Transfer: persisted, Replayed: replayed}, nil
}

func (s *Service) List(ctx context.Context) ([]Transfer, error) {
	values, err := s.repository.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list internal transfers: %w", err)
	}
	return values, nil
}

func (s *Service) Get(ctx context.Context, transferID string) (Transfer, bool, error) {
	id := strings.ToLower(strings.TrimSpace(transferID))
	if !uuidPattern.MatchString(id) {
		return Transfer{}, false, &ValidationError{Field: "internal_transfer_id", Message: "内部账户转账标识无效"}
	}
	value, found, err := s.repository.Get(ctx, id)
	if err != nil {
		return Transfer{}, false, fmt.Errorf("get internal transfer: %w", err)
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
	ledgerOwnerPartyID := strings.ToLower(strings.TrimSpace(input.LedgerOwnerPartyID))
	if !uuidPattern.MatchString(ledgerOwnerPartyID) {
		return CreateInput{}, &ValidationError{Field: "ledger_owner_party_id", Message: "请选择有效的本方账簿主体"}
	}
	description, err := requiredText("description", "转账描述不能为空", input.Description, 200)
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
		return CreateInput{}, &ValidationError{Field: "occurred_on", Message: "转账发生日必须是有效的 YYYY-MM-DD 日期"}
	}
	sourceAccountReference, err := requiredText("source_account_reference", "来源账户标签不能为空", input.SourceAccountReference, 100)
	if err != nil {
		return CreateInput{}, err
	}
	destinationAccountReference, err := requiredText("destination_account_reference", "目标账户标签不能为空", input.DestinationAccountReference, 100)
	if err != nil {
		return CreateInput{}, err
	}
	if strings.EqualFold(sourceAccountReference, destinationAccountReference) {
		return CreateInput{}, &ValidationError{Field: "destination_account_reference", Message: "来源账户标签与目标账户标签必须不同"}
	}
	manualSourceReference, err := requiredText("manual_source_reference", "人工来源依据不能为空", input.ManualSourceReference, 200)
	if err != nil {
		return CreateInput{}, err
	}
	externalTransferReference := strings.TrimSpace(input.ExternalTransferReference)
	if len([]rune(externalTransferReference)) > 100 {
		return CreateInput{}, &ValidationError{Field: "external_transfer_reference", Message: "外部转账参考不能超过 100 个字符"}
	}
	return CreateInput{
		CommandID:                   commandID,
		LedgerOwnerPartyID:          ledgerOwnerPartyID,
		Description:                 description,
		Amount:                      amount,
		Currency:                    currency,
		OccurredOn:                  occurredOn,
		SourceAccountReference:      sourceAccountReference,
		DestinationAccountReference: destinationAccountReference,
		ManualSourceReference:       manualSourceReference,
		ExternalTransferReference:   externalTransferReference,
		Actor:                       actor,
	}, nil
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
	raw := strings.TrimSpace(value)
	if !amountPattern.MatchString(raw) {
		return "", &ValidationError{Field: "amount", Message: "转账金额必须是大于 0 的普通十进制数"}
	}
	parts := strings.SplitN(raw, ".", 2)
	integer := strings.TrimLeft(parts[0], "0")
	if integer == "" {
		integer = "0"
	}
	if len(integer) > 18 {
		return "", &ValidationError{Field: "amount", Message: "转账金额整数部分不能超过 18 位"}
	}
	if len(parts) == 1 {
		if integer == "0" {
			return "", &ValidationError{Field: "amount", Message: "转账金额必须大于 0"}
		}
		return integer, nil
	}
	if len(parts[1]) > 6 {
		return "", &ValidationError{Field: "amount", Message: "转账金额小数部分不能超过 6 位"}
	}
	fraction := strings.TrimRight(parts[1], "0")
	if fraction == "" {
		if integer == "0" {
			return "", &ValidationError{Field: "amount", Message: "转账金额必须大于 0"}
		}
		return integer, nil
	}
	return integer + "." + fraction, nil
}

func createPayloadHash(input CreateInput) ([]byte, error) {
	payload, err := json.Marshal(struct {
		Format                         string `json:"format"`
		LedgerOwnerPartyID             string `json:"ledger_owner_party_id"`
		Description                    string `json:"description"`
		Amount                         string `json:"amount"`
		Currency                       string `json:"currency"`
		OccurredOn                     string `json:"occurred_on"`
		SourceAccountReferenceKey      string `json:"source_account_reference_key"`
		DestinationAccountReferenceKey string `json:"destination_account_reference_key"`
		SourceKind                     string `json:"source_kind"`
		ManualSourceReference          string `json:"manual_source_reference"`
		ExternalTransferReferenceKey   string `json:"external_transfer_reference_key"`
		ActorKind                      string `json:"actor_kind"`
		ActorID                        string `json:"actor_id"`
	}{
		Format:                         "money-flow-internal-transfer-create-v1",
		LedgerOwnerPartyID:             input.LedgerOwnerPartyID,
		Description:                    input.Description,
		Amount:                         input.Amount,
		Currency:                       input.Currency,
		OccurredOn:                     input.OccurredOn,
		SourceAccountReferenceKey:      strings.ToLower(input.SourceAccountReference),
		DestinationAccountReferenceKey: strings.ToLower(input.DestinationAccountReference),
		SourceKind:                     SourceKindManual,
		ManualSourceReference:          input.ManualSourceReference,
		ExternalTransferReferenceKey:   strings.ToLower(input.ExternalTransferReference),
		ActorKind:                      input.Actor.Kind,
		ActorID:                        input.Actor.ID,
	})
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(payload)
	return hash[:], nil
}

func externalReferenceLockKey(value Transfer) (string, error) {
	if value.ExternalTransferReference == "" {
		return "", nil
	}
	payload, err := json.Marshal(struct {
		Format                         string `json:"format"`
		LedgerOwnerPartyID             string `json:"ledger_owner_party_id"`
		SourceAccountReferenceKey      string `json:"source_account_reference_key"`
		DestinationAccountReferenceKey string `json:"destination_account_reference_key"`
		SourceKind                     string `json:"source_kind"`
		ExternalReferenceKey           string `json:"external_reference_key"`
	}{
		Format:                         "money-flow-internal-transfer-external-reference-v1",
		LedgerOwnerPartyID:             value.LedgerOwner.PartyID,
		SourceAccountReferenceKey:      strings.ToLower(value.SourceAccountReference),
		DestinationAccountReferenceKey: strings.ToLower(value.DestinationAccountReference),
		SourceKind:                     value.SourceKind,
		ExternalReferenceKey:           strings.ToLower(value.ExternalTransferReference),
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
