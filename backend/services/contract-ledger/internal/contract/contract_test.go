package contract

import (
	"context"
	"errors"
	"strings"
	"testing"

	"omsbizfoundry/contract-ledger/internal/audit"
)

var testActor = ActorRef{
	Kind:        audit.ActorKindLocalUser,
	ID:          "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
	DisplayName: "测试用户",
}

type repositoryStub struct {
	persisted           Contract
	created             bool
	err                 error
	received            Contract
	list                []Contract
	search              []Contract
	receivedSearch      ContractSearchQuery
	draft               Contract
	draftParties        []DraftParty
	draftCancellation   *DraftCancellation
	draftFound          bool
	addCommand          DraftPartyCommand
	addParty            DraftParty
	addCreated          bool
	receivedAdd         DraftPartyCommand
	receivedAddLink     DraftParty
	removeCommand       DraftPartyCommand
	removeParty         DraftParty
	removeCreated       bool
	receivedRemove      DraftPartyCommand
	updateCommand       DraftChangeCommand
	updateCreated       bool
	receivedUpdate      DraftChangeCommand
	receivedUpdateDraft Contract
	cancelCommand       DraftChangeCommand
	cancelCreated       bool
	receivedCancel      DraftChangeCommand
	activationCommand   ActivationCommand
	activationCreated   bool
	receivedActivation  ActivationCommand
	changeCommand       ContractChangeCommand
	changeCreated       bool
	receivedChange      ContractChangeCommand
	terminationCommand  TerminationCommand
	terminationCreated  bool
	receivedTermination TerminationCommand
	relationCommand     ContractRelationCommand
	relation            ContractRelation
	relationCreated     bool
	receivedRelation    ContractRelationCommand
	settlementCommand   ContractSettlementCommand
	settlement          ContractSettlement
	settlementCreated   bool
	receivedSettlement  ContractSettlementCommand
	closureCommand      ContractClosureCommand
	closure             ContractClosure
	closureCreated      bool
	receivedClosure     ContractClosureCommand
	correctionCommand   ContractCorrectionCommand
	correctionCreated   bool
	receivedCorrection  ContractCorrectionCommand
}

func (r *repositoryStub) GetSettlement(context.Context, string) (ContractSettlement, bool, error) {
	if r.err != nil {
		return ContractSettlement{}, false, r.err
	}
	return r.settlement, r.settlement.ID != "", nil
}

func (r *repositoryStub) CorrectContract(_ context.Context, command ContractCorrectionCommand) (ContractCorrectionCommand, bool, error) {
	r.receivedCorrection = command
	if r.err != nil {
		return ContractCorrectionCommand{}, false, r.err
	}
	if r.correctionCommand.CommandID != "" {
		return r.correctionCommand, r.correctionCreated, nil
	}
	return command, true, nil
}

func (r *repositoryStub) CloseContract(_ context.Context, command ContractClosureCommand) (ContractClosureCommand, ContractClosure, bool, error) {
	r.receivedClosure = command
	if r.err != nil {
		return ContractClosureCommand{}, ContractClosure{}, false, r.err
	}
	if r.closureCommand.CommandID != "" {
		return r.closureCommand, r.closure, r.closureCreated, nil
	}
	return command, ContractClosure{
		ID: command.ClosureID, ContractID: command.ContractID,
		SourceRevision:           command.ExpectedContractRevision,
		SourceSettlementSequence: command.ExpectedSettlementSequence,
		SettlementDisposition:    command.SettlementDisposition,
		FinalSettlementID:        command.FinalSettlementID,
		ClosedOn:                 command.ClosedOn, Reason: command.Reason,
		Actor: command.Actor, RecordedAt: command.OccurredAt,
	}, true, nil
}

func (r *repositoryStub) RecordSettlement(_ context.Context, command ContractSettlementCommand) (ContractSettlementCommand, ContractSettlement, bool, error) {
	r.receivedSettlement = command
	if r.err != nil {
		return ContractSettlementCommand{}, ContractSettlement{}, false, r.err
	}
	if r.settlementCommand.CommandID != "" {
		return r.settlementCommand, r.settlement, r.settlementCreated, nil
	}
	return command, ContractSettlement{
		ID: command.SettlementID, ContractID: command.ContractID,
		SourceRevision: command.ExpectedContractRevision,
		Sequence:       command.ResultingSettlementSequence, Kind: command.Kind,
		StatementNumber: command.StatementNumber, Title: command.Title, Amount: command.Amount,
		Currency: "CNY", ConfirmedOn: command.ConfirmedOn, Basis: command.Basis,
		Actor: command.Actor, RecordedAt: command.OccurredAt,
	}, true, nil
}

func (r *repositoryStub) CreateDraft(_ context.Context, value Contract, _ audit.Entry) (Contract, bool, error) {
	r.received = value
	if r.err != nil {
		return Contract{}, false, r.err
	}
	if r.persisted.ID != "" {
		return r.persisted, r.created, nil
	}
	return value, true, nil
}

func (r *repositoryStub) List(context.Context) ([]Contract, error) {
	if r.err != nil {
		return nil, r.err
	}
	return append([]Contract(nil), r.list...), nil
}

func (r *repositoryStub) Search(_ context.Context, query ContractSearchQuery) ([]Contract, error) {
	r.receivedSearch = query
	if r.err != nil {
		return nil, r.err
	}
	return append([]Contract(nil), r.search...), nil
}

func (r *repositoryStub) GetDetails(context.Context, string) (DraftDetails, bool, error) {
	if r.err != nil {
		return DraftDetails{}, false, r.err
	}
	return DraftDetails{
		Contract:     r.draft,
		Parties:      append([]DraftParty(nil), r.draftParties...),
		Cancellation: r.draftCancellation,
	}, r.draftFound, nil
}

func (r *repositoryStub) UpdateDraft(_ context.Context, command DraftChangeCommand, draft Contract) (DraftChangeCommand, bool, error) {
	r.receivedUpdate = command
	r.receivedUpdateDraft = draft
	if r.err != nil {
		return DraftChangeCommand{}, false, r.err
	}
	if r.updateCommand.CommandID != "" {
		return r.updateCommand, r.updateCreated, nil
	}
	command.ResultingDraftVersion = command.ExpectedDraftVersion + 1
	return command, true, nil
}

func (r *repositoryStub) CancelDraft(_ context.Context, command DraftChangeCommand) (DraftChangeCommand, bool, error) {
	r.receivedCancel = command
	if r.err != nil {
		return DraftChangeCommand{}, false, r.err
	}
	if r.cancelCommand.CommandID != "" {
		return r.cancelCommand, r.cancelCreated, nil
	}
	command.ResultingDraftVersion = command.ExpectedDraftVersion + 1
	return command, true, nil
}

func (r *repositoryStub) Activate(_ context.Context, command ActivationCommand) (ActivationCommand, bool, error) {
	r.receivedActivation = command
	if r.err != nil {
		return ActivationCommand{}, false, r.err
	}
	if r.activationCommand.CommandID != "" {
		return r.activationCommand, r.activationCreated, nil
	}
	command.ResultingRevision = 1
	return command, true, nil
}

func (r *repositoryStub) AppendChange(_ context.Context, command ContractChangeCommand) (ContractChangeCommand, bool, error) {
	r.receivedChange = command
	if r.err != nil {
		return ContractChangeCommand{}, false, r.err
	}
	if r.changeCommand.CommandID != "" {
		return r.changeCommand, r.changeCreated, nil
	}
	command.ResultingRevision = command.ExpectedRevision + 1
	command.ResultingAmount = "1250.5"
	return command, true, nil
}

func (r *repositoryStub) Terminate(_ context.Context, command TerminationCommand) (TerminationCommand, bool, error) {
	r.receivedTermination = command
	if r.err != nil {
		return TerminationCommand{}, false, r.err
	}
	if r.terminationCommand.CommandID != "" {
		return r.terminationCommand, r.terminationCreated, nil
	}
	command.ResultingRevision = command.ExpectedRevision + 1
	command.ResultingAmount = "1250.5"
	return command, true, nil
}

func (r *repositoryStub) LinkRelation(_ context.Context, command ContractRelationCommand) (ContractRelationCommand, ContractRelation, bool, error) {
	r.receivedRelation = command
	if r.err != nil {
		return ContractRelationCommand{}, ContractRelation{}, false, r.err
	}
	if r.relationCommand.CommandID != "" {
		return r.relationCommand, r.relation, r.relationCreated, nil
	}
	return command, ContractRelation{
		ID:     command.RelationID,
		Source: ContractRelationSnapshot{ContractID: command.SourceContractID, Revision: command.ExpectedSourceRevision},
		Target: ContractRelationSnapshot{ContractID: command.TargetContractID, Revision: command.ExpectedTargetRevision},
		Type:   command.RelationType, EffectiveOn: command.EffectiveOn, Note: command.Note,
		Actor: command.Actor, CreatedAt: command.OccurredAt,
	}, true, nil
}

func (r *repositoryStub) AddDraftParty(_ context.Context, command DraftPartyCommand, linked DraftParty) (DraftPartyCommand, DraftParty, bool, error) {
	r.receivedAdd = command
	r.receivedAddLink = linked
	if r.err != nil {
		return DraftPartyCommand{}, DraftParty{}, false, r.err
	}
	if r.addCommand.CommandID != "" {
		return r.addCommand, r.addParty, r.addCreated, nil
	}
	command.ResultingDraftVersion = command.ExpectedDraftVersion + 1
	return command, linked, true, nil
}

func (r *repositoryStub) RemoveDraftParty(_ context.Context, command DraftPartyCommand) (DraftPartyCommand, DraftParty, bool, error) {
	r.receivedRemove = command
	if r.err != nil {
		return DraftPartyCommand{}, DraftParty{}, false, r.err
	}
	if r.removeCommand.CommandID != "" {
		return r.removeCommand, r.removeParty, r.removeCreated, nil
	}
	command.ResultingDraftVersion = command.ExpectedDraftVersion + 1
	return command, DraftParty{ID: command.PartyLinkID, ContractID: command.ContractID}, true, nil
}

func validInput(t *testing.T) CreateDraftInput {
	t.Helper()
	commandID, err := NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	return CreateDraftInput{
		CommandID:       commandID,
		NumberNamespace: " default ",
		ContractNumber:  " HT-2026-001 ",
		Title:           " 年度服务合同 ",
		Category:        " 服务 ",
		Currency:        " cny ",
		TotalAmount:     " 001000.500000 ",
		SignedOn:        "2026-09-19",
		EffectiveOn:     "2026-10-01",
		Actor:           testActor,
	}
}

func TestCreateDraftNormalizesAndPersists(t *testing.T) {
	repository := &repositoryStub{}
	created, err := NewService(repository).CreateDraft(context.Background(), validInput(t))
	if err != nil {
		t.Fatalf("CreateDraft() error = %v", err)
	}
	if created.ID == "" || created.CommandID == "" {
		t.Fatalf("CreateDraft() identifiers = %#v", created)
	}
	if created.NumberNamespace != "default" || created.ContractNumber != "HT-2026-001" || created.Title != "年度服务合同" {
		t.Fatalf("CreateDraft() identity = %#v", created)
	}
	if created.Currency != "CNY" || created.TotalAmount != "1000.5" {
		t.Fatalf("CreateDraft() money = %#v", created)
	}
	if created.Status != StatusDraft || created.DraftVersion != 1 || created.CurrentRevision != 0 {
		t.Fatalf("CreateDraft() lifecycle = %#v", created)
	}
	if created.CreatedAt.IsZero() || !created.CreatedAt.Equal(created.UpdatedAt) {
		t.Fatalf("CreateDraft() timestamps = %#v", created)
	}
}

func TestCreateDraftAllowsUndeterminedAmount(t *testing.T) {
	input := validInput(t)
	input.TotalAmount = ""
	created, err := NewService(&repositoryStub{}).CreateDraft(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateDraft() error = %v", err)
	}
	if created.TotalAmount != "" {
		t.Fatalf("TotalAmount = %q, want empty", created.TotalAmount)
	}
}

func TestCreateDraftValidation(t *testing.T) {
	long := func(size int) string { return strings.Repeat("字", size) }
	tests := []struct {
		name   string
		mutate func(*CreateDraftInput)
		field  string
	}{
		{name: "invalid command ID", mutate: func(v *CreateDraftInput) { v.CommandID = "bad" }, field: "command_id"},
		{name: "empty namespace", mutate: func(v *CreateDraftInput) { v.NumberNamespace = " " }, field: "number_namespace"},
		{name: "long namespace", mutate: func(v *CreateDraftInput) { v.NumberNamespace = long(65) }, field: "number_namespace"},
		{name: "empty number", mutate: func(v *CreateDraftInput) { v.ContractNumber = "" }, field: "contract_number"},
		{name: "long number", mutate: func(v *CreateDraftInput) { v.ContractNumber = long(101) }, field: "contract_number"},
		{name: "empty title", mutate: func(v *CreateDraftInput) { v.Title = " " }, field: "title"},
		{name: "long title", mutate: func(v *CreateDraftInput) { v.Title = long(201) }, field: "title"},
		{name: "long category", mutate: func(v *CreateDraftInput) { v.Category = long(101) }, field: "category"},
		{name: "invalid currency", mutate: func(v *CreateDraftInput) { v.Currency = "人民币" }, field: "currency"},
		{name: "negative amount", mutate: func(v *CreateDraftInput) { v.TotalAmount = "-1" }, field: "total_amount"},
		{name: "scientific amount", mutate: func(v *CreateDraftInput) { v.TotalAmount = "1e3" }, field: "total_amount"},
		{name: "too many integer digits", mutate: func(v *CreateDraftInput) { v.TotalAmount = "1234567890123456789" }, field: "total_amount"},
		{name: "too many decimals", mutate: func(v *CreateDraftInput) { v.TotalAmount = "1.1234567" }, field: "total_amount"},
		{name: "invalid signed date", mutate: func(v *CreateDraftInput) { v.SignedOn = "2026-02-30" }, field: "signed_on"},
		{name: "invalid effective date", mutate: func(v *CreateDraftInput) { v.EffectiveOn = "19/09/2026" }, field: "effective_on"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validInput(t)
			test.mutate(&input)
			repository := &repositoryStub{}
			_, err := NewService(repository).CreateDraft(context.Background(), input)
			var validationError *ValidationError
			if !errors.As(err, &validationError) {
				t.Fatalf("CreateDraft() error = %v, want ValidationError", err)
			}
			if validationError.Field != test.field {
				t.Fatalf("ValidationError.Field = %q, want %q", validationError.Field, test.field)
			}
			if repository.received.ID != "" {
				t.Fatalf("invalid draft reached repository: %#v", repository.received)
			}
		})
	}
}

func TestCreateDraftIsIdempotentForSameIntent(t *testing.T) {
	input := validInput(t)
	normalized, err := normalizeCreateDraftInput(input)
	if err != nil {
		t.Fatalf("normalizeCreateDraftInput() error = %v", err)
	}
	existing := Contract{
		ID:              "existing",
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
	}
	repository := &repositoryStub{persisted: existing, created: false}

	got, err := NewService(repository).CreateDraft(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateDraft() error = %v", err)
	}
	if got.ID != existing.ID {
		t.Fatalf("CreateDraft() ID = %q, want %q", got.ID, existing.ID)
	}
}

func TestCreateDraftRejectsReusedCommandWithDifferentIntent(t *testing.T) {
	input := validInput(t)
	normalized, err := normalizeCreateDraftInput(input)
	if err != nil {
		t.Fatalf("normalizeCreateDraftInput() error = %v", err)
	}
	existing := Contract{
		ID:              "existing",
		CommandID:       normalized.CommandID,
		NumberNamespace: normalized.NumberNamespace,
		ContractNumber:  normalized.ContractNumber,
		Title:           "另一个标题",
		Currency:        normalized.Currency,
		TotalAmount:     normalized.TotalAmount,
		Status:          StatusDraft,
		DraftVersion:    1,
	}

	_, err = NewService(&repositoryStub{persisted: existing, created: false}).CreateDraft(context.Background(), input)
	var conflict *IdempotencyConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("CreateDraft() error = %v, want IdempotencyConflictError", err)
	}
}
