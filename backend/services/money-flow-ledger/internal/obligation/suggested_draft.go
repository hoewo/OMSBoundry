package obligation

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

type SuggestedSource struct {
	Application  string
	ResourceType string
	ResourceID   string
	Revision     int64
	Digest       string
}

type CreateSuggestedDraftInput struct {
	CommandID           string
	LedgerOwnerPartyID  string
	CounterpartyPartyID string
	Title               string
	OriginalAmount      string
	Currency            string
	DueOn               string
	Source              SuggestedSource
	Actor               audit.ActorRef
}

func (s *Service) CreateSuggestedDraft(ctx context.Context, input CreateSuggestedDraftInput) (CreateResult, error) {
	normalized, err := normalizeSuggestedDraftInput(input)
	if err != nil {
		return CreateResult{}, err
	}
	payloadHash, err := suggestedDraftPayloadHash(normalized)
	if err != nil {
		return CreateResult{}, fmt.Errorf("hash suggested obligation draft intent: %w", err)
	}
	obligationID, err := s.newID()
	if err != nil {
		return CreateResult{}, fmt.Errorf("generate suggested obligation ID: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return CreateResult{}, fmt.Errorf("generate suggested obligation audit ID: %w", err)
	}
	now := s.now()
	sourceKind := SourceKindCostOutsourcingSuggestion
	auditSummary := "从成本与外协付款建议创建应付义务草稿"
	if normalized.Source.Application == "bonus-distribution" {
		sourceKind = SourceKindBonusSuggestion
		auditSummary = "从奖金分配应付建议创建应付义务草稿"
	} else if normalized.Source.Application == "payroll" {
		sourceKind = SourceKindPayrollPayment
		auditSummary = "从工资实付请求创建应付义务草稿"
	}
	created := Obligation{
		ID: obligationID, Direction: DirectionPayable, Title: normalized.Title,
		LedgerOwnerPartyID: normalized.LedgerOwnerPartyID,
		OriginalAmount:     normalized.OriginalAmount, EffectiveAmount: normalized.OriginalAmount,
		AllocatedAmount: "0", RemittedAmount: "0", OutstandingAmount: normalized.OriginalAmount,
		Currency: normalized.Currency, DueOn: normalized.DueOn,
		SourceKind:        sourceKind,
		SourceApplication: normalized.Source.Application, SourceResourceType: normalized.Source.ResourceType,
		SourceResourceID: normalized.Source.ResourceID, SourceFactRevision: normalized.Source.Revision,
		SourceDigest: normalized.Source.Digest,
		Status:       StatusDraft, DraftVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	created.Creditor.PartyID = normalized.CounterpartyPartyID
	created.Debtor.PartyID = normalized.LedgerOwnerPartyID
	persisted, replayed, err := s.repository.CreateDraft(ctx, created, CreateCommand{
		CommandID: normalized.CommandID, PayloadHash: payloadHash, Actor: normalized.Actor, CreatedAt: now,
	}, audit.Entry{
		ID: auditID, ObjectType: audit.ObjectObligation, ObjectID: obligationID,
		Action: AuditActionDraftCreated, CommandID: normalized.CommandID, Actor: normalized.Actor, OccurredAt: now,
		Summary: auditSummary,
		Details: map[string]any{
			"direction": DirectionPayable, "ledger_owner_party_id": normalized.LedgerOwnerPartyID,
			"counterparty_party_id": normalized.CounterpartyPartyID, "original_amount": normalized.OriginalAmount,
			"currency": normalized.Currency, "due_on": normalized.DueOn,
			"source_kind": sourceKind, "source_application": normalized.Source.Application,
			"source_resource_type": normalized.Source.ResourceType, "source_resource_id": normalized.Source.ResourceID,
			"source_revision": normalized.Source.Revision, "source_digest": normalized.Source.Digest,
		},
	})
	if err != nil {
		return CreateResult{}, fmt.Errorf("create suggested obligation draft: %w", err)
	}
	return CreateResult{Obligation: persisted, Replayed: replayed}, nil
}

func normalizeSuggestedDraftInput(input CreateSuggestedDraftInput) (CreateSuggestedDraftInput, error) {
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	if !uuidPattern.MatchString(input.CommandID) {
		return CreateSuggestedDraftInput{}, &ValidationError{Field: "command_id", Message: "Command ID 无效"}
	}
	var err error
	if input.Actor, err = audit.NormalizeActor(input.Actor); err != nil {
		return CreateSuggestedDraftInput{}, &ValidationError{Field: "actor", Message: err.Error()}
	}
	if input.LedgerOwnerPartyID, err = normalizePartyID("ledger_owner_party_id", input.LedgerOwnerPartyID); err != nil {
		return CreateSuggestedDraftInput{}, err
	}
	if input.CounterpartyPartyID, err = normalizePartyID("counterparty_party_id", input.CounterpartyPartyID); err != nil {
		return CreateSuggestedDraftInput{}, err
	}
	if input.LedgerOwnerPartyID == input.CounterpartyPartyID {
		return CreateSuggestedDraftInput{}, &ValidationError{Field: "counterparty_party_id", Message: "本方与收款主体必须不同"}
	}
	if input.Title, err = requiredText("title", "义务标题不能为空", input.Title, 200); err != nil {
		return CreateSuggestedDraftInput{}, err
	}
	if input.OriginalAmount, err = normalizeAmount(input.OriginalAmount); err != nil {
		return CreateSuggestedDraftInput{}, err
	}
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	if !currencyPattern.MatchString(input.Currency) {
		return CreateSuggestedDraftInput{}, &ValidationError{Field: "currency", Message: "币种必须是三个英文字母"}
	}
	if _, err = normalizeCreateDraftInput(CreateDraftInput{
		CommandID: input.CommandID, Direction: string(DirectionPayable), LedgerOwnerPartyID: input.LedgerOwnerPartyID,
		CounterpartyPartyID: input.CounterpartyPartyID, Title: input.Title, OriginalAmount: input.OriginalAmount,
		Currency: input.Currency, DueOn: input.DueOn, ManualSourceReference: "source-validation", Actor: input.Actor,
	}); err != nil {
		return CreateSuggestedDraftInput{}, err
	}
	input.Source.Application = strings.TrimSpace(input.Source.Application)
	input.Source.ResourceType = strings.TrimSpace(input.Source.ResourceType)
	input.Source.ResourceID = strings.ToLower(strings.TrimSpace(input.Source.ResourceID))
	input.Source.Digest = strings.ToLower(strings.TrimSpace(input.Source.Digest))
	validSource := (input.Source.Application == "cost-outsourcing" &&
		(input.Source.ResourceType == "cost_entry" || input.Source.ResourceType == "outsourcing_engagement")) ||
		(input.Source.Application == "bonus-distribution" && input.Source.ResourceType == "bonus_allocation") ||
		(input.Source.Application == "payroll" && input.Source.ResourceType == "payroll_line")
	if !validSource ||
		!uuidPattern.MatchString(input.Source.ResourceID) || input.Source.Revision < 1 ||
		len(input.Source.Digest) != 71 || !strings.HasPrefix(input.Source.Digest, "sha256:") {
		return CreateSuggestedDraftInput{}, &ValidationError{Field: "source", Message: "付款建议来源无效"}
	}
	for _, ch := range input.Source.Digest[7:] {
		if !strings.ContainsRune("0123456789abcdef", ch) {
			return CreateSuggestedDraftInput{}, &ValidationError{Field: "source", Message: "付款建议来源摘要无效"}
		}
	}
	return input, nil
}

func suggestedDraftPayloadHash(input CreateSuggestedDraftInput) ([]byte, error) {
	payload, err := json.Marshal(struct {
		Format, LedgerOwnerPartyID, CounterpartyPartyID, Title, OriginalAmount, Currency, DueOn string
		Source                                                                                  SuggestedSource `json:"source"`
		ActorKind, ActorID                                                                      string
	}{
		Format: "money-flow-suggested-obligation-draft-create-v1", LedgerOwnerPartyID: input.LedgerOwnerPartyID,
		CounterpartyPartyID: input.CounterpartyPartyID, Title: input.Title, OriginalAmount: input.OriginalAmount,
		Currency: input.Currency, DueOn: input.DueOn, Source: input.Source,
		ActorKind: input.Actor.Kind, ActorID: input.Actor.ID,
	})
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(payload)
	return hash[:], nil
}
