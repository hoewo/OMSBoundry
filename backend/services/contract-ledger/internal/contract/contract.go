package contract

import (
	"context"
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
	"time"

	"omsbizfoundry/contract-ledger/internal/audit"
)

type Status string

const (
	StatusDraft      Status = "draft"
	StatusActive     Status = "active"
	StatusTerminated Status = "terminated"
	StatusClosed     Status = "closed"
	StatusCancelled  Status = "cancelled"

	AuditActionDraftCreated = "contract_draft_created"
)

var (
	uuidPattern     = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
	amountPattern   = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?$`)
)

type Contract struct {
	ID              string
	CommandID       string
	NumberNamespace string
	ContractNumber  string
	Title           string
	Category        string
	Currency        string
	TotalAmount     string
	SignedOn        string
	EffectiveOn     string
	Status          Status
	DraftVersion    int64
	CurrentRevision int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CreateDraftInput struct {
	CommandID       string
	NumberNamespace string
	ContractNumber  string
	Title           string
	Category        string
	Currency        string
	TotalAmount     string
	SignedOn        string
	EffectiveOn     string
	Actor           ActorRef
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

type DuplicateNumberError struct {
	NumberNamespace string
	ContractNumber  string
}

func (e *DuplicateNumberError) Error() string {
	return "该编号命名空间中已存在相同合同编号"
}

type IdempotencyConflictError struct {
	CommandID string
}

func (e *IdempotencyConflictError) Error() string {
	return "命令标识已经使用，但提交内容与首次请求不同"
}

type Repository interface {
	CreateDraft(context.Context, Contract, audit.Entry) (Contract, bool, error)
	List(context.Context) ([]Contract, error)
	Search(context.Context, ContractSearchQuery) ([]Contract, error)
	GetDetails(context.Context, string) (DraftDetails, bool, error)
	AddDraftParty(context.Context, DraftPartyCommand, DraftParty) (DraftPartyCommand, DraftParty, bool, error)
	RemoveDraftParty(context.Context, DraftPartyCommand) (DraftPartyCommand, DraftParty, bool, error)
	UpdateDraft(context.Context, DraftChangeCommand, Contract) (DraftChangeCommand, bool, error)
	CancelDraft(context.Context, DraftChangeCommand) (DraftChangeCommand, bool, error)
	Activate(context.Context, ActivationCommand) (ActivationCommand, bool, error)
	AppendChange(context.Context, ContractChangeCommand) (ContractChangeCommand, bool, error)
	Terminate(context.Context, TerminationCommand) (TerminationCommand, bool, error)
	LinkRelation(context.Context, ContractRelationCommand) (ContractRelationCommand, ContractRelation, bool, error)
	RecordSettlement(context.Context, ContractSettlementCommand) (ContractSettlementCommand, ContractSettlement, bool, error)
	GetSettlement(context.Context, string) (ContractSettlement, bool, error)
	CloseContract(context.Context, ContractClosureCommand) (ContractClosureCommand, ContractClosure, bool, error)
	CorrectContract(context.Context, ContractCorrectionCommand) (ContractCorrectionCommand, bool, error)
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

func (s *Service) CreateDraft(ctx context.Context, input CreateDraftInput) (Contract, error) {
	normalized, err := normalizeCreateDraftInput(input)
	if err != nil {
		return Contract{}, err
	}

	id, err := s.newID()
	if err != nil {
		return Contract{}, fmt.Errorf("generate contract ID: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return Contract{}, fmt.Errorf("generate contract draft audit ID: %w", err)
	}
	now := s.now()
	candidate := Contract{
		ID:              id,
		CommandID:       normalized.CommandID,
		NumberNamespace: normalized.NumberNamespace,
		ContractNumber:  normalized.ContractNumber,
		Title:           normalized.Title,
		Category:        normalized.Category,
		Currency:        normalized.Currency,
		TotalAmount:     normalized.TotalAmount,
		SignedOn:        normalized.SignedOn,
		EffectiveOn:     normalized.EffectiveOn,
		Status:          StatusDraft,
		DraftVersion:    1,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	persisted, created, err := s.repository.CreateDraft(ctx, candidate, audit.Entry{
		ID:         auditID,
		ObjectType: audit.ObjectContract,
		ObjectID:   candidate.ID,
		Action:     AuditActionDraftCreated,
		CommandID:  candidate.CommandID,
		Actor:      normalized.Actor,
		OccurredAt: now,
		Summary:    "创建合同草稿，草稿版本 1",
		Details:    map[string]any{"resulting_draft_version": 1},
	})
	if err != nil {
		return Contract{}, fmt.Errorf("create contract draft: %w", err)
	}
	if !created && !sameCreateIntent(persisted, candidate) {
		return Contract{}, &IdempotencyConflictError{CommandID: candidate.CommandID}
	}
	return persisted, nil
}

func (s *Service) List(ctx context.Context) ([]Contract, error) {
	contracts, err := s.repository.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list contracts: %w", err)
	}
	return contracts, nil
}

func normalizeCreateDraftInput(input CreateDraftInput) (CreateDraftInput, error) {
	actor, err := normalizeActor(input.Actor)
	if err != nil {
		return CreateDraftInput{}, err
	}
	commandID := strings.ToLower(strings.TrimSpace(input.CommandID))
	if !uuidPattern.MatchString(commandID) {
		return CreateDraftInput{}, &ValidationError{Field: "command_id", Message: "创建请求标识无效，请刷新页面后重试"}
	}

	namespace, err := requiredText("number_namespace", "编号命名空间不能为空", input.NumberNamespace, 64)
	if err != nil {
		return CreateDraftInput{}, err
	}
	contractNumber, err := requiredText("contract_number", "合同编号不能为空", input.ContractNumber, 100)
	if err != nil {
		return CreateDraftInput{}, err
	}
	title, err := requiredText("title", "合同标题不能为空", input.Title, 200)
	if err != nil {
		return CreateDraftInput{}, err
	}
	category, err := optionalText("category", "通用分类不能超过 100 个字符", input.Category, 100)
	if err != nil {
		return CreateDraftInput{}, err
	}

	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if !currencyPattern.MatchString(currency) {
		return CreateDraftInput{}, &ValidationError{Field: "currency", Message: "币种必须是三个英文字母"}
	}
	totalAmount, err := normalizeAmount(input.TotalAmount)
	if err != nil {
		return CreateDraftInput{}, err
	}
	signedOn, err := normalizeDate("signed_on", "签署日期格式无效", input.SignedOn)
	if err != nil {
		return CreateDraftInput{}, err
	}
	effectiveOn, err := normalizeDate("effective_on", "生效日期格式无效", input.EffectiveOn)
	if err != nil {
		return CreateDraftInput{}, err
	}

	return CreateDraftInput{
		CommandID:       commandID,
		NumberNamespace: namespace,
		ContractNumber:  contractNumber,
		Title:           title,
		Category:        category,
		Currency:        currency,
		TotalAmount:     totalAmount,
		SignedOn:        signedOn,
		EffectiveOn:     effectiveOn,
		Actor:           actor,
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

func optionalText(field, tooLongMessage, value string, maxLength int) (string, error) {
	normalized := strings.TrimSpace(value)
	if len([]rune(normalized)) > maxLength {
		return "", &ValidationError{Field: field, Message: tooLongMessage}
	}
	return normalized, nil
}

func normalizeAmount(value string) (string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return "", nil
	}
	if !amountPattern.MatchString(raw) {
		return "", &ValidationError{Field: "total_amount", Message: "合同总额必须是非负十进制数"}
	}

	parts := strings.SplitN(raw, ".", 2)
	integer := strings.TrimLeft(parts[0], "0")
	if integer == "" {
		integer = "0"
	}
	if len(integer) > 18 {
		return "", &ValidationError{Field: "total_amount", Message: "合同总额整数部分不能超过 18 位"}
	}
	if len(parts) == 1 {
		return integer, nil
	}
	if len(parts[1]) > 6 {
		return "", &ValidationError{Field: "total_amount", Message: "合同总额小数部分不能超过 6 位"}
	}
	fraction := strings.TrimRight(parts[1], "0")
	if fraction == "" {
		return integer, nil
	}
	return integer + "." + fraction, nil
}

func normalizeDate(field, message, value string) (string, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", nil
	}
	parsed, err := time.Parse("2006-01-02", normalized)
	if err != nil || parsed.Format("2006-01-02") != normalized {
		return "", &ValidationError{Field: field, Message: message}
	}
	return normalized, nil
}

func sameCreateIntent(left, right Contract) bool {
	return left.CommandID == right.CommandID &&
		left.NumberNamespace == right.NumberNamespace &&
		left.ContractNumber == right.ContractNumber &&
		left.Title == right.Title &&
		left.Category == right.Category &&
		left.Currency == right.Currency &&
		left.TotalAmount == right.TotalAmount &&
		left.SignedOn == right.SignedOn &&
		left.EffectiveOn == right.EffectiveOn
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
