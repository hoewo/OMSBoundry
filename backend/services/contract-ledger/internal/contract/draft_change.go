package contract

import (
	"context"
	"fmt"
	"time"
)

type DraftChangeAction string

const (
	DraftChangeActionUpdate DraftChangeAction = "update"
	DraftChangeActionCancel DraftChangeAction = "cancel"

	AuditActionDraftUpdated   = "contract_draft_updated"
	AuditActionDraftCancelled = "contract_draft_cancelled"
)

type DraftChangeCommand struct {
	CommandID             string
	AuditID               string
	ContractID            string
	Action                DraftChangeAction
	ExpectedDraftVersion  int64
	ResultingDraftVersion int64
	NumberNamespace       string
	ContractNumber        string
	Title                 string
	Category              string
	Currency              string
	TotalAmount           string
	SignedOn              string
	EffectiveOn           string
	CancellationReason    string
	CreatedAt             time.Time
	Actor                 ActorRef
}

type DraftCancellation struct {
	CommandID    string
	Reason       string
	CancelledAt  time.Time
	DraftVersion int64
}

type UpdateDraftInput struct {
	CommandID            string
	ContractID           string
	ExpectedDraftVersion int64
	NumberNamespace      string
	ContractNumber       string
	Title                string
	Category             string
	Currency             string
	TotalAmount          string
	SignedOn             string
	EffectiveOn          string
	Actor                ActorRef
}

type CancelDraftInput struct {
	CommandID            string
	ContractID           string
	ExpectedDraftVersion int64
	Reason               string
	Actor                ActorRef
}

type DraftMutationResult struct {
	ContractID   string
	DraftVersion int64
	Status       Status
	Replayed     bool
}

type NoDraftChangesError struct{}

func (e *NoDraftChangesError) Error() string { return "合同草稿内容没有变化" }

func (s *Service) UpdateDraft(ctx context.Context, input UpdateDraftInput) (DraftMutationResult, error) {
	normalized, err := normalizeUpdateDraftInput(input)
	if err != nil {
		return DraftMutationResult{}, err
	}
	now := s.now()
	auditID, err := s.newID()
	if err != nil {
		return DraftMutationResult{}, fmt.Errorf("generate draft update audit ID: %w", err)
	}
	command := DraftChangeCommand{
		CommandID:            normalized.CommandID,
		AuditID:              auditID,
		ContractID:           normalized.ContractID,
		Action:               DraftChangeActionUpdate,
		ExpectedDraftVersion: normalized.ExpectedDraftVersion,
		NumberNamespace:      normalized.NumberNamespace,
		ContractNumber:       normalized.ContractNumber,
		Title:                normalized.Title,
		Category:             normalized.Category,
		Currency:             normalized.Currency,
		TotalAmount:          normalized.TotalAmount,
		SignedOn:             normalized.SignedOn,
		EffectiveOn:          normalized.EffectiveOn,
		CreatedAt:            now,
		Actor:                normalized.Actor,
	}
	candidate := Contract{
		ID:              normalized.ContractID,
		NumberNamespace: normalized.NumberNamespace,
		ContractNumber:  normalized.ContractNumber,
		Title:           normalized.Title,
		Category:        normalized.Category,
		Currency:        normalized.Currency,
		TotalAmount:     normalized.TotalAmount,
		SignedOn:        normalized.SignedOn,
		EffectiveOn:     normalized.EffectiveOn,
		UpdatedAt:       now,
	}
	persisted, created, err := s.repository.UpdateDraft(ctx, command, candidate)
	if err != nil {
		return DraftMutationResult{}, fmt.Errorf("update contract draft: %w", err)
	}
	if !created && !sameUpdateDraftIntent(persisted, command) {
		return DraftMutationResult{}, &IdempotencyConflictError{CommandID: command.CommandID}
	}
	return DraftMutationResult{
		ContractID:   persisted.ContractID,
		DraftVersion: persisted.ResultingDraftVersion,
		Status:       StatusDraft,
		Replayed:     !created,
	}, nil
}

func (s *Service) CancelDraft(ctx context.Context, input CancelDraftInput) (DraftMutationResult, error) {
	normalized, err := normalizeCancelDraftInput(input)
	if err != nil {
		return DraftMutationResult{}, err
	}
	command := DraftChangeCommand{
		CommandID:            normalized.CommandID,
		ContractID:           normalized.ContractID,
		Action:               DraftChangeActionCancel,
		ExpectedDraftVersion: normalized.ExpectedDraftVersion,
		CancellationReason:   normalized.Reason,
		CreatedAt:            s.now(),
		Actor:                normalized.Actor,
	}
	auditID, err := s.newID()
	if err != nil {
		return DraftMutationResult{}, fmt.Errorf("generate draft cancellation audit ID: %w", err)
	}
	command.AuditID = auditID
	persisted, created, err := s.repository.CancelDraft(ctx, command)
	if err != nil {
		return DraftMutationResult{}, fmt.Errorf("cancel contract draft: %w", err)
	}
	if !created && !sameCancelDraftIntent(persisted, command) {
		return DraftMutationResult{}, &IdempotencyConflictError{CommandID: command.CommandID}
	}
	return DraftMutationResult{
		ContractID:   persisted.ContractID,
		DraftVersion: persisted.ResultingDraftVersion,
		Status:       StatusCancelled,
		Replayed:     !created,
	}, nil
}

func normalizeUpdateDraftInput(input UpdateDraftInput) (UpdateDraftInput, error) {
	contractID, err := normalizeUUID("contract_id", "合同标识无效", input.ContractID)
	if err != nil {
		return UpdateDraftInput{}, err
	}
	normalizedFields, err := normalizeCreateDraftInput(CreateDraftInput{
		CommandID:       input.CommandID,
		NumberNamespace: input.NumberNamespace,
		ContractNumber:  input.ContractNumber,
		Title:           input.Title,
		Category:        input.Category,
		Currency:        input.Currency,
		TotalAmount:     input.TotalAmount,
		SignedOn:        input.SignedOn,
		EffectiveOn:     input.EffectiveOn,
		Actor:           input.Actor,
	})
	if err != nil {
		return UpdateDraftInput{}, err
	}
	if input.ExpectedDraftVersion < 1 {
		return UpdateDraftInput{}, &ValidationError{Field: "expected_draft_version", Message: "合同草稿版本无效，请刷新页面后重试"}
	}
	return UpdateDraftInput{
		CommandID:            normalizedFields.CommandID,
		ContractID:           contractID,
		ExpectedDraftVersion: input.ExpectedDraftVersion,
		NumberNamespace:      normalizedFields.NumberNamespace,
		ContractNumber:       normalizedFields.ContractNumber,
		Title:                normalizedFields.Title,
		Category:             normalizedFields.Category,
		Currency:             normalizedFields.Currency,
		TotalAmount:          normalizedFields.TotalAmount,
		SignedOn:             normalizedFields.SignedOn,
		EffectiveOn:          normalizedFields.EffectiveOn,
		Actor:                normalizedFields.Actor,
	}, nil
}

func normalizeCancelDraftInput(input CancelDraftInput) (CancelDraftInput, error) {
	actor, err := normalizeActor(input.Actor)
	if err != nil {
		return CancelDraftInput{}, err
	}
	commandID, err := normalizeUUID("command_id", "命令标识无效，请刷新页面后重试", input.CommandID)
	if err != nil {
		return CancelDraftInput{}, err
	}
	contractID, err := normalizeUUID("contract_id", "合同标识无效", input.ContractID)
	if err != nil {
		return CancelDraftInput{}, err
	}
	reason, err := requiredText("cancellation_reason", "取消原因不能为空", input.Reason, 500)
	if err != nil {
		return CancelDraftInput{}, err
	}
	if input.ExpectedDraftVersion < 1 {
		return CancelDraftInput{}, &ValidationError{Field: "expected_draft_version", Message: "合同草稿版本无效，请刷新页面后重试"}
	}
	return CancelDraftInput{
		CommandID:            commandID,
		ContractID:           contractID,
		ExpectedDraftVersion: input.ExpectedDraftVersion,
		Reason:               reason,
		Actor:                actor,
	}, nil
}

func sameUpdateDraftIntent(left, right DraftChangeCommand) bool {
	return left.CommandID == right.CommandID &&
		left.ContractID == right.ContractID &&
		left.Action == DraftChangeActionUpdate &&
		left.ExpectedDraftVersion == right.ExpectedDraftVersion &&
		left.NumberNamespace == right.NumberNamespace &&
		left.ContractNumber == right.ContractNumber &&
		left.Title == right.Title &&
		left.Category == right.Category &&
		left.Currency == right.Currency &&
		left.TotalAmount == right.TotalAmount &&
		left.SignedOn == right.SignedOn &&
		left.EffectiveOn == right.EffectiveOn
}

func sameCancelDraftIntent(left, right DraftChangeCommand) bool {
	return left.CommandID == right.CommandID &&
		left.ContractID == right.ContractID &&
		left.Action == DraftChangeActionCancel &&
		left.ExpectedDraftVersion == right.ExpectedDraftVersion &&
		left.CancellationReason == right.CancellationReason
}
