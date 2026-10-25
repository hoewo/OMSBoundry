package obligation

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/party"
)

type Direction string

const (
	DirectionReceivable Direction = "receivable"
	DirectionPayable    Direction = "payable"

	StatusDraft  = "draft"
	StatusOpen   = "open"
	StatusClosed = "closed"

	SourceKindManual                    = "manual"
	SourceKindCostOutsourcingSuggestion = "cost_outsourcing_payable_suggestion"
	SourceKindBonusSuggestion           = "bonus_payable_suggestion"
	SourceKindPayrollPayment            = "payroll_payment"

	AuditActionDraftCreated      = "obligation_draft_created"
	AuditActionConfirmed         = "obligation_confirmed"
	AuditActionAmountAdjusted    = "obligation_amount_adjusted"
	AuditActionRemissionRecorded = "obligation_remission_recorded"
	AuditActionClosed            = "obligation_closed"
)

var (
	uuidPattern     = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
	amountPattern   = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?$`)
)

func (d Direction) Label() string {
	switch d {
	case DirectionReceivable:
		return "应收"
	case DirectionPayable:
		return "应付"
	default:
		return string(d)
	}
}

type PartySnapshot struct {
	PartyID                string
	PartyRevision          int64
	Kind                   party.Kind
	LegalName              string
	RegistrationIdentifier string
}

type Obligation struct {
	ID                    string
	Direction             Direction
	Title                 string
	LedgerOwnerPartyID    string
	Creditor              PartySnapshot
	Debtor                PartySnapshot
	OriginalAmount        string
	EffectiveAmount       string
	AllocatedAmount       string
	RemittedAmount        string
	OutstandingAmount     string
	BalanceSequence       int64
	Currency              string
	DueOn                 string
	SourceKind            string
	ManualSourceReference string
	SourceApplication     string
	SourceResourceType    string
	SourceResourceID      string
	SourceFactRevision    int64
	SourceDigest          string
	Status                string
	DraftVersion          int64
	CurrentRevision       int64
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func (o Obligation) LedgerOwner() PartySnapshot {
	if o.Creditor.PartyID == o.LedgerOwnerPartyID {
		return o.Creditor
	}
	return o.Debtor
}

func (o Obligation) Counterparty() PartySnapshot {
	if o.Creditor.PartyID == o.LedgerOwnerPartyID {
		return o.Debtor
	}
	return o.Creditor
}

type CreateDraftInput struct {
	CommandID             string
	Direction             string
	LedgerOwnerPartyID    string
	CounterpartyPartyID   string
	Title                 string
	OriginalAmount        string
	Currency              string
	DueOn                 string
	ManualSourceReference string
	Actor                 audit.ActorRef
}

type CreateCommand struct {
	CommandID   string
	PayloadHash []byte
	Actor       audit.ActorRef
	CreatedAt   time.Time
}

type CreateResult struct {
	Obligation Obligation
	Replayed   bool
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
	return "该 Command ID 已用于不同的义务草稿创建意图"
}

type PartyNotFoundError struct {
	PartyID string
}

func (e *PartyNotFoundError) Error() string {
	return "选择的主体不存在或已不可用"
}

type Repository interface {
	CreateDraft(context.Context, Obligation, CreateCommand, audit.Entry) (Obligation, bool, error)
	Confirm(context.Context, string, int64, ConfirmationCommand, audit.Entry) (Obligation, Revision, bool, error)
	AdjustAmount(context.Context, Adjustment, AdjustmentCommand, audit.Entry) (Obligation, Adjustment, Revision, bool, error)
	RecordRemission(context.Context, Remission, RemissionCommand, audit.Entry) (Obligation, Remission, bool, error)
	Close(context.Context, Closure, ClosureCommand, audit.Entry) (Obligation, Closure, bool, error)
	GetHistoricalSnapshot(context.Context, string, time.Time) (HistoricalSnapshotLookup, error)
	List(context.Context) ([]Obligation, error)
	Get(context.Context, string) (Obligation, bool, error)
	GetDetails(context.Context, string) (Details, bool, error)
}

type Service struct {
	repository       Repository
	cashExpectations cashExpectationRepository
	now              func() time.Time
	newID            func() (string, error)
}

func NewService(repository Repository) *Service {
	cashExpectations, _ := repository.(cashExpectationRepository)
	return &Service{
		repository:       repository,
		cashExpectations: cashExpectations,
		now:              func() time.Time { return time.Now().UTC() },
		newID:            randomUUID,
	}
}

func NewCommandID() (string, error) {
	return randomUUID()
}

func (s *Service) CreateDraft(ctx context.Context, input CreateDraftInput) (CreateResult, error) {
	normalized, err := normalizeCreateDraftInput(input)
	if err != nil {
		return CreateResult{}, err
	}
	payloadHash, err := createPayloadHash(normalized)
	if err != nil {
		return CreateResult{}, fmt.Errorf("hash obligation draft create intent: %w", err)
	}
	obligationID, err := s.newID()
	if err != nil {
		return CreateResult{}, fmt.Errorf("generate obligation ID: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return CreateResult{}, fmt.Errorf("generate obligation audit ID: %w", err)
	}
	now := s.now()
	created := Obligation{
		ID:                    obligationID,
		Direction:             Direction(normalized.Direction),
		Title:                 normalized.Title,
		LedgerOwnerPartyID:    normalized.LedgerOwnerPartyID,
		OriginalAmount:        normalized.OriginalAmount,
		EffectiveAmount:       normalized.OriginalAmount,
		AllocatedAmount:       "0",
		RemittedAmount:        "0",
		OutstandingAmount:     normalized.OriginalAmount,
		Currency:              normalized.Currency,
		DueOn:                 normalized.DueOn,
		SourceKind:            SourceKindManual,
		ManualSourceReference: normalized.ManualSourceReference,
		Status:                StatusDraft,
		DraftVersion:          1,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if created.Direction == DirectionReceivable {
		created.Creditor.PartyID = normalized.LedgerOwnerPartyID
		created.Debtor.PartyID = normalized.CounterpartyPartyID
	} else {
		created.Creditor.PartyID = normalized.CounterpartyPartyID
		created.Debtor.PartyID = normalized.LedgerOwnerPartyID
	}
	persisted, replayed, err := s.repository.CreateDraft(ctx, created, CreateCommand{
		CommandID:   normalized.CommandID,
		PayloadHash: payloadHash,
		Actor:       normalized.Actor,
		CreatedAt:   now,
	}, audit.Entry{
		ID:         auditID,
		ObjectType: audit.ObjectObligation,
		ObjectID:   obligationID,
		Action:     AuditActionDraftCreated,
		CommandID:  normalized.CommandID,
		Actor:      normalized.Actor,
		OccurredAt: now,
		Summary:    "创建人工义务草稿，draft version 1",
		Details: map[string]any{
			"direction":               normalized.Direction,
			"ledger_owner_party_id":   normalized.LedgerOwnerPartyID,
			"counterparty_party_id":   normalized.CounterpartyPartyID,
			"original_amount":         normalized.OriginalAmount,
			"currency":                normalized.Currency,
			"due_on":                  normalized.DueOn,
			"source_kind":             SourceKindManual,
			"manual_source_reference": normalized.ManualSourceReference,
			"resulting_draft_version": int64(1),
		},
	})
	if err != nil {
		return CreateResult{}, fmt.Errorf("create obligation draft: %w", err)
	}
	return CreateResult{Obligation: persisted, Replayed: replayed}, nil
}

func (s *Service) List(ctx context.Context) ([]Obligation, error) {
	values, err := s.repository.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list obligation drafts: %w", err)
	}
	return values, nil
}

func (s *Service) Get(ctx context.Context, obligationID string) (Obligation, bool, error) {
	id := strings.ToLower(strings.TrimSpace(obligationID))
	if !uuidPattern.MatchString(id) {
		return Obligation{}, false, &ValidationError{Field: "obligation_id", Message: "义务标识无效"}
	}
	value, found, err := s.repository.Get(ctx, id)
	if err != nil {
		return Obligation{}, false, fmt.Errorf("get obligation draft: %w", err)
	}
	return value, found, nil
}

func (s *Service) GetDetails(ctx context.Context, obligationID string) (Details, bool, error) {
	id := strings.ToLower(strings.TrimSpace(obligationID))
	if !uuidPattern.MatchString(id) {
		return Details{}, false, &ValidationError{Field: "obligation_id", Message: "义务标识无效"}
	}
	value, found, err := s.repository.GetDetails(ctx, id)
	if err != nil {
		return Details{}, false, fmt.Errorf("get obligation details: %w", err)
	}
	return value, found, nil
}

func normalizeCreateDraftInput(input CreateDraftInput) (CreateDraftInput, error) {
	commandID := strings.ToLower(strings.TrimSpace(input.CommandID))
	if !uuidPattern.MatchString(commandID) {
		return CreateDraftInput{}, &ValidationError{Field: "command_id", Message: "Command ID 无效，请刷新页面后重试"}
	}
	actor, err := audit.NormalizeActor(input.Actor)
	if err != nil {
		return CreateDraftInput{}, &ValidationError{Field: "actor", Message: err.Error()}
	}
	direction := Direction(strings.ToLower(strings.TrimSpace(input.Direction)))
	if direction != DirectionReceivable && direction != DirectionPayable {
		return CreateDraftInput{}, &ValidationError{Field: "direction", Message: "请选择有效的应收或应付方向"}
	}
	ledgerOwnerPartyID, err := normalizePartyID("ledger_owner_party_id", input.LedgerOwnerPartyID)
	if err != nil {
		return CreateDraftInput{}, err
	}
	counterpartyPartyID, err := normalizePartyID("counterparty_party_id", input.CounterpartyPartyID)
	if err != nil {
		return CreateDraftInput{}, err
	}
	if ledgerOwnerPartyID == counterpartyPartyID {
		return CreateDraftInput{}, &ValidationError{Field: "counterparty_party_id", Message: "本方账簿主体与往来主体必须不同"}
	}
	title, err := requiredText("title", "义务标题不能为空", input.Title, 200)
	if err != nil {
		return CreateDraftInput{}, err
	}
	originalAmount, err := normalizeAmount(input.OriginalAmount)
	if err != nil {
		return CreateDraftInput{}, err
	}
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if !currencyPattern.MatchString(currency) {
		return CreateDraftInput{}, &ValidationError{Field: "currency", Message: "币种必须是三个英文字母"}
	}
	dueOn := strings.TrimSpace(input.DueOn)
	parsedDueOn, parseErr := time.Parse("2006-01-02", dueOn)
	if parseErr != nil || parsedDueOn.Year() < 1 || parsedDueOn.Format("2006-01-02") != dueOn {
		return CreateDraftInput{}, &ValidationError{Field: "due_on", Message: "到期日必须是有效的 YYYY-MM-DD 日期"}
	}
	manualSourceReference, err := requiredText("manual_source_reference", "人工来源依据不能为空", input.ManualSourceReference, 200)
	if err != nil {
		return CreateDraftInput{}, err
	}
	return CreateDraftInput{
		CommandID:             commandID,
		Direction:             string(direction),
		LedgerOwnerPartyID:    ledgerOwnerPartyID,
		CounterpartyPartyID:   counterpartyPartyID,
		Title:                 title,
		OriginalAmount:        originalAmount,
		Currency:              currency,
		DueOn:                 dueOn,
		ManualSourceReference: manualSourceReference,
		Actor:                 actor,
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
	return normalizeAmountForField("original_amount", "义务金额", value, true)
}

func normalizeAmountAllowZero(value string) (string, error) {
	return normalizeAmountForField("original_amount", "义务金额", value, false)
}

func normalizeAmountForField(field, label, value string, requirePositive bool) (string, error) {
	raw := strings.TrimSpace(value)
	if !amountPattern.MatchString(raw) {
		return "", &ValidationError{Field: field, Message: label + "必须是大于 0 的普通十进制数"}
	}
	parts := strings.SplitN(raw, ".", 2)
	integer := strings.TrimLeft(parts[0], "0")
	if integer == "" {
		integer = "0"
	}
	if len(integer) > 18 {
		return "", &ValidationError{Field: field, Message: label + "整数部分不能超过 18 位"}
	}
	if len(parts) == 1 {
		if requirePositive && integer == "0" {
			return "", &ValidationError{Field: field, Message: label + "必须大于 0"}
		}
		return integer, nil
	}
	if len(parts[1]) > 6 {
		return "", &ValidationError{Field: field, Message: label + "小数部分不能超过 6 位"}
	}
	fraction := strings.TrimRight(parts[1], "0")
	if fraction == "" {
		if requirePositive && integer == "0" {
			return "", &ValidationError{Field: field, Message: label + "必须大于 0"}
		}
		return integer, nil
	}
	return integer + "." + fraction, nil
}

func createPayloadHash(input CreateDraftInput) ([]byte, error) {
	payload, err := json.Marshal(struct {
		Format                string `json:"format"`
		Direction             string `json:"direction"`
		LedgerOwnerPartyID    string `json:"ledger_owner_party_id"`
		CounterpartyPartyID   string `json:"counterparty_party_id"`
		Title                 string `json:"title"`
		OriginalAmount        string `json:"original_amount"`
		Currency              string `json:"currency"`
		DueOn                 string `json:"due_on"`
		SourceKind            string `json:"source_kind"`
		ManualSourceReference string `json:"manual_source_reference"`
		ActorKind             string `json:"actor_kind"`
		ActorID               string `json:"actor_id"`
	}{
		Format:                "money-flow-obligation-draft-create-v1",
		Direction:             input.Direction,
		LedgerOwnerPartyID:    input.LedgerOwnerPartyID,
		CounterpartyPartyID:   input.CounterpartyPartyID,
		Title:                 input.Title,
		OriginalAmount:        input.OriginalAmount,
		Currency:              input.Currency,
		DueOn:                 input.DueOn,
		SourceKind:            SourceKindManual,
		ManualSourceReference: input.ManualSourceReference,
		ActorKind:             input.Actor.Kind,
		ActorID:               input.Actor.ID,
	})
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(payload)
	return hash[:], nil
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
