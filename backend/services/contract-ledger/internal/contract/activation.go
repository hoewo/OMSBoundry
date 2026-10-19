package contract

import (
	"context"
	"fmt"
	"time"

	"omsbizfoundry/contract-ledger/internal/audit"
)

const AuditEventContractActivated = "contract_activated"

type ActorRef = audit.ActorRef

type ActivationCommand struct {
	CommandID            string
	AuditID              string
	InstrumentID         string
	ContractID           string
	ExpectedDraftVersion int64
	Actor                ActorRef
	ResultingRevision    int64
	OccurredAt           time.Time
}

type ActivateContractInput struct {
	CommandID            string
	ContractID           string
	ExpectedDraftVersion int64
	Actor                ActorRef
}

type ActivationResult struct {
	ContractID string
	Revision   int64
	Replayed   bool
}

type ContractRevision struct {
	ContractID          string
	Revision            int64
	ActivationCommandID string
	SourceDraftVersion  int64
	SourceInstrumentID  string
	NumberNamespace     string
	ContractNumber      string
	Title               string
	Category            string
	Currency            string
	TotalAmount         string
	SignedOn            string
	EffectiveOn         string
	Actor               ActorRef
	RecordedAt          time.Time
}

type ContractPartySnapshot struct {
	ContractID             string
	Revision               int64
	Position               int64
	SourcePartyLinkID      string
	SourcePartyID          string
	SourcePartyRevision    int64
	PartyKind              string
	LegalName              string
	RegistrationIdentifier string
	Role                   string
}

type ContractAuditEntry struct {
	ID                 string
	ContractID         string
	EventType          string
	CommandID          string
	Actor              ActorRef
	OccurredAt         time.Time
	SourceDraftVersion int64
	ResultingRevision  int64
	Summary            string
}

type ActivationPreconditionError struct {
	Field   string
	Message string
}

func (e *ActivationPreconditionError) Error() string { return e.Message }

func (s *Service) Activate(ctx context.Context, input ActivateContractInput) (ActivationResult, error) {
	normalized, err := normalizeActivateContractInput(input)
	if err != nil {
		return ActivationResult{}, err
	}
	auditID, err := s.newID()
	if err != nil {
		return ActivationResult{}, fmt.Errorf("generate activation audit ID: %w", err)
	}
	instrumentID, err := s.newID()
	if err != nil {
		return ActivationResult{}, fmt.Errorf("generate original contract instrument ID: %w", err)
	}
	command := ActivationCommand{
		CommandID:            normalized.CommandID,
		AuditID:              auditID,
		InstrumentID:         instrumentID,
		ContractID:           normalized.ContractID,
		ExpectedDraftVersion: normalized.ExpectedDraftVersion,
		Actor:                normalized.Actor,
		ResultingRevision:    1,
		OccurredAt:           s.now(),
	}
	persisted, created, err := s.repository.Activate(ctx, command)
	if err != nil {
		return ActivationResult{}, fmt.Errorf("activate contract: %w", err)
	}
	if !created && !sameActivationIntent(persisted, command) {
		return ActivationResult{}, &IdempotencyConflictError{CommandID: command.CommandID}
	}
	return ActivationResult{
		ContractID: persisted.ContractID,
		Revision:   persisted.ResultingRevision,
		Replayed:   !created,
	}, nil
}

func normalizeActivateContractInput(input ActivateContractInput) (ActivateContractInput, error) {
	commandID, err := normalizeUUID("command_id", "命令标识无效，请刷新页面后重试", input.CommandID)
	if err != nil {
		return ActivateContractInput{}, err
	}
	contractID, err := normalizeUUID("contract_id", "合同标识无效", input.ContractID)
	if err != nil {
		return ActivateContractInput{}, err
	}
	if input.ExpectedDraftVersion < 1 {
		return ActivateContractInput{}, &ValidationError{Field: "expected_draft_version", Message: "合同草稿版本无效，请刷新页面后重试"}
	}
	actor, err := normalizeActor(input.Actor)
	if err != nil {
		return ActivateContractInput{}, err
	}
	return ActivateContractInput{
		CommandID:            commandID,
		ContractID:           contractID,
		ExpectedDraftVersion: input.ExpectedDraftVersion,
		Actor:                actor,
	}, nil
}

func normalizeActor(value ActorRef) (ActorRef, error) {
	actor, err := audit.NormalizeActor(value)
	if err != nil {
		return ActorRef{}, &ValidationError{Field: "actor", Message: err.Error()}
	}
	return actor, nil
}

func sameActivationIntent(left, right ActivationCommand) bool {
	return left.CommandID == right.CommandID &&
		left.ContractID == right.ContractID &&
		left.ExpectedDraftVersion == right.ExpectedDraftVersion &&
		left.Actor == right.Actor &&
		left.ResultingRevision == 1
}
