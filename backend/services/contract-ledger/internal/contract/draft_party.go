package contract

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type DraftPartyAction string

const (
	DraftPartyActionAdd    DraftPartyAction = "add"
	DraftPartyActionRemove DraftPartyAction = "remove"

	AuditActionDraftPartyAdded   = "contract_draft_party_added"
	AuditActionDraftPartyRemoved = "contract_draft_party_removed"
)

type DraftParty struct {
	ID                     string
	ContractID             string
	PartyID                string
	PartyKind              string
	PartyLegalName         string
	RegistrationIdentifier string
	PartyRevision          int64
	Role                   string
	AddedAt                time.Time
	RemovedAt              time.Time
}

func (p DraftParty) Active() bool { return p.RemovedAt.IsZero() }

type DraftDetails struct {
	Contract          Contract
	Parties           []DraftParty
	Cancellation      *DraftCancellation
	Revision          *ContractRevision
	Revisions         []ContractRevision
	Instruments       []ContractInstrument
	PartySnapshots    []ContractPartySnapshot
	AuditEntries      []ContractAuditEntry
	OutgoingRelations []ContractRelation
	IncomingRelations []ContractRelation
	Settlements       []ContractSettlement
	Closure           *ContractClosure
}

type AddDraftPartyInput struct {
	CommandID            string
	ContractID           string
	PartyID              string
	Role                 string
	ExpectedDraftVersion int64
	Actor                ActorRef
}

type RemoveDraftPartyInput struct {
	CommandID            string
	ContractID           string
	PartyLinkID          string
	ExpectedDraftVersion int64
	Actor                ActorRef
}

type DraftPartyMutationResult struct {
	Party        DraftParty
	DraftVersion int64
	Replayed     bool
}

type DraftPartyCommand struct {
	CommandID             string
	AuditID               string
	ContractID            string
	Action                DraftPartyAction
	PartyLinkID           string
	PartyID               string
	Role                  string
	ExpectedDraftVersion  int64
	ResultingDraftVersion int64
	CreatedAt             time.Time
	Actor                 ActorRef
}

type ContractNotFoundError struct{ ContractID string }

func (e *ContractNotFoundError) Error() string { return "合同不存在" }

type PartyNotFoundError struct{ PartyID string }

func (e *PartyNotFoundError) Error() string { return "所选主体不存在" }

type DraftPartyNotFoundError struct{ PartyLinkID string }

func (e *DraftPartyNotFoundError) Error() string { return "签约主体关联不存在或已移除" }

type ContractStateConflictError struct{ Status Status }

func (e *ContractStateConflictError) Error() string {
	return "只有合同草稿可以执行此操作"
}

type DraftVersionConflictError struct {
	Expected int64
	Actual   int64
}

func (e *DraftVersionConflictError) Error() string {
	return "合同草稿已被其他操作更新，请确认最新内容后重试"
}

type DuplicateDraftPartyError struct {
	PartyID string
	Role    string
}

func (e *DuplicateDraftPartyError) Error() string { return "该主体已经承担相同合同角色" }

func (s *Service) GetDraft(ctx context.Context, contractID string) (DraftDetails, error) {
	normalizedID, err := normalizeUUID("contract_id", "合同标识无效", contractID)
	if err != nil {
		return DraftDetails{}, err
	}
	details, found, err := s.repository.GetDetails(ctx, normalizedID)
	if err != nil {
		return DraftDetails{}, fmt.Errorf("get contract draft: %w", err)
	}
	if !found {
		return DraftDetails{}, &ContractNotFoundError{ContractID: normalizedID}
	}
	return details, nil
}

func (s *Service) AddDraftParty(ctx context.Context, input AddDraftPartyInput) (DraftPartyMutationResult, error) {
	normalized, err := normalizeAddDraftPartyInput(input)
	if err != nil {
		return DraftPartyMutationResult{}, err
	}
	linkID, err := s.newID()
	if err != nil {
		return DraftPartyMutationResult{}, fmt.Errorf("generate draft party link ID: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return DraftPartyMutationResult{}, fmt.Errorf("generate draft party audit ID: %w", err)
	}
	now := s.now()
	command := DraftPartyCommand{
		CommandID:            normalized.CommandID,
		AuditID:              auditID,
		ContractID:           normalized.ContractID,
		Action:               DraftPartyActionAdd,
		PartyLinkID:          linkID,
		PartyID:              normalized.PartyID,
		Role:                 normalized.Role,
		ExpectedDraftVersion: normalized.ExpectedDraftVersion,
		CreatedAt:            now,
		Actor:                normalized.Actor,
	}
	candidate := DraftParty{
		ID:         linkID,
		ContractID: normalized.ContractID,
		PartyID:    normalized.PartyID,
		Role:       normalized.Role,
		AddedAt:    now,
	}

	persistedCommand, persistedParty, created, err := s.repository.AddDraftParty(ctx, command, candidate)
	if err != nil {
		return DraftPartyMutationResult{}, fmt.Errorf("add contract draft party: %w", err)
	}
	if !created && !sameAddDraftPartyIntent(persistedCommand, command) {
		return DraftPartyMutationResult{}, &IdempotencyConflictError{CommandID: command.CommandID}
	}
	return DraftPartyMutationResult{
		Party:        persistedParty,
		DraftVersion: persistedCommand.ResultingDraftVersion,
		Replayed:     !created,
	}, nil
}

func (s *Service) RemoveDraftParty(ctx context.Context, input RemoveDraftPartyInput) (DraftPartyMutationResult, error) {
	normalized, err := normalizeRemoveDraftPartyInput(input)
	if err != nil {
		return DraftPartyMutationResult{}, err
	}
	command := DraftPartyCommand{
		CommandID:            normalized.CommandID,
		ContractID:           normalized.ContractID,
		Action:               DraftPartyActionRemove,
		PartyLinkID:          normalized.PartyLinkID,
		ExpectedDraftVersion: normalized.ExpectedDraftVersion,
		CreatedAt:            s.now(),
		Actor:                normalized.Actor,
	}
	auditID, err := s.newID()
	if err != nil {
		return DraftPartyMutationResult{}, fmt.Errorf("generate draft party audit ID: %w", err)
	}
	command.AuditID = auditID

	persistedCommand, persistedParty, created, err := s.repository.RemoveDraftParty(ctx, command)
	if err != nil {
		return DraftPartyMutationResult{}, fmt.Errorf("remove contract draft party: %w", err)
	}
	if !created && !sameRemoveDraftPartyIntent(persistedCommand, command) {
		return DraftPartyMutationResult{}, &IdempotencyConflictError{CommandID: command.CommandID}
	}
	return DraftPartyMutationResult{
		Party:        persistedParty,
		DraftVersion: persistedCommand.ResultingDraftVersion,
		Replayed:     !created,
	}, nil
}

func normalizeAddDraftPartyInput(input AddDraftPartyInput) (AddDraftPartyInput, error) {
	actor, err := normalizeActor(input.Actor)
	if err != nil {
		return AddDraftPartyInput{}, err
	}
	commandID, err := normalizeUUID("command_id", "命令标识无效，请刷新页面后重试", input.CommandID)
	if err != nil {
		return AddDraftPartyInput{}, err
	}
	contractID, err := normalizeUUID("contract_id", "合同标识无效", input.ContractID)
	if err != nil {
		return AddDraftPartyInput{}, err
	}
	partyID, err := normalizeUUID("party_id", "请选择有效的签约主体", input.PartyID)
	if err != nil {
		return AddDraftPartyInput{}, err
	}
	role, err := requiredText("role", "合同角色不能为空", input.Role, 100)
	if err != nil {
		return AddDraftPartyInput{}, err
	}
	if input.ExpectedDraftVersion < 1 {
		return AddDraftPartyInput{}, &ValidationError{Field: "expected_draft_version", Message: "合同草稿版本无效，请刷新页面后重试"}
	}
	return AddDraftPartyInput{
		CommandID:            commandID,
		ContractID:           contractID,
		PartyID:              partyID,
		Role:                 role,
		ExpectedDraftVersion: input.ExpectedDraftVersion,
		Actor:                actor,
	}, nil
}

func normalizeRemoveDraftPartyInput(input RemoveDraftPartyInput) (RemoveDraftPartyInput, error) {
	actor, err := normalizeActor(input.Actor)
	if err != nil {
		return RemoveDraftPartyInput{}, err
	}
	commandID, err := normalizeUUID("command_id", "命令标识无效，请刷新页面后重试", input.CommandID)
	if err != nil {
		return RemoveDraftPartyInput{}, err
	}
	contractID, err := normalizeUUID("contract_id", "合同标识无效", input.ContractID)
	if err != nil {
		return RemoveDraftPartyInput{}, err
	}
	partyLinkID, err := normalizeUUID("party_link_id", "签约主体关联标识无效", input.PartyLinkID)
	if err != nil {
		return RemoveDraftPartyInput{}, err
	}
	if input.ExpectedDraftVersion < 1 {
		return RemoveDraftPartyInput{}, &ValidationError{Field: "expected_draft_version", Message: "合同草稿版本无效，请刷新页面后重试"}
	}
	return RemoveDraftPartyInput{
		CommandID:            commandID,
		ContractID:           contractID,
		PartyLinkID:          partyLinkID,
		ExpectedDraftVersion: input.ExpectedDraftVersion,
		Actor:                actor,
	}, nil
}

func normalizeUUID(field, message, value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if !uuidPattern.MatchString(normalized) {
		return "", &ValidationError{Field: field, Message: message}
	}
	return normalized, nil
}

func sameAddDraftPartyIntent(left, right DraftPartyCommand) bool {
	return left.CommandID == right.CommandID &&
		left.ContractID == right.ContractID &&
		left.Action == DraftPartyActionAdd &&
		left.PartyID == right.PartyID &&
		left.Role == right.Role &&
		left.ExpectedDraftVersion == right.ExpectedDraftVersion
}

func sameRemoveDraftPartyIntent(left, right DraftPartyCommand) bool {
	return left.CommandID == right.CommandID &&
		left.ContractID == right.ContractID &&
		left.Action == DraftPartyActionRemove &&
		left.PartyLinkID == right.PartyLinkID &&
		left.ExpectedDraftVersion == right.ExpectedDraftVersion
}
