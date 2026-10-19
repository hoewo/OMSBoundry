package contract

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const (
	testContractID = "11111111-1111-4111-8111-111111111111"
	testPartyID    = "22222222-2222-4222-8222-222222222222"
	testLinkID     = "33333333-3333-4333-8333-333333333333"
)

func validAddDraftPartyInput(t *testing.T) AddDraftPartyInput {
	t.Helper()
	commandID, err := NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	return AddDraftPartyInput{
		CommandID:            commandID,
		ContractID:           testContractID,
		PartyID:              testPartyID,
		Role:                 " 委托方 ",
		ExpectedDraftVersion: 1,
		Actor:                testActor,
	}
}

func validRemoveDraftPartyInput(t *testing.T) RemoveDraftPartyInput {
	t.Helper()
	commandID, err := NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	return RemoveDraftPartyInput{
		CommandID:            commandID,
		ContractID:           testContractID,
		PartyLinkID:          testLinkID,
		ExpectedDraftVersion: 2,
		Actor:                testActor,
	}
}

func TestGetDraftReturnsConsistentDetails(t *testing.T) {
	repository := &repositoryStub{
		draft:        Contract{ID: testContractID, Status: StatusDraft, DraftVersion: 2},
		draftParties: []DraftParty{{ID: testLinkID, ContractID: testContractID, PartyID: testPartyID, Role: "委托方"}},
		draftFound:   true,
	}
	details, err := NewService(repository).GetDraft(context.Background(), " "+strings.ToUpper(testContractID)+" ")
	if err != nil {
		t.Fatalf("GetDraft() error = %v", err)
	}
	if details.Contract.ID != testContractID || len(details.Parties) != 1 || details.Parties[0].Role != "委托方" {
		t.Fatalf("GetDraft() = %#v", details)
	}
}

func TestGetDraftRejectsMissingContract(t *testing.T) {
	_, err := NewService(&repositoryStub{}).GetDraft(context.Background(), testContractID)
	var notFound *ContractNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("GetDraft() error = %v, want ContractNotFoundError", err)
	}
}

func TestAddDraftPartyNormalizesAndPersists(t *testing.T) {
	repository := &repositoryStub{}
	result, err := NewService(repository).AddDraftParty(context.Background(), validAddDraftPartyInput(t))
	if err != nil {
		t.Fatalf("AddDraftParty() error = %v", err)
	}
	if repository.receivedAdd.Role != "委托方" || repository.receivedAddLink.Role != "委托方" {
		t.Fatalf("normalized command = %#v, link = %#v", repository.receivedAdd, repository.receivedAddLink)
	}
	if repository.receivedAdd.Action != DraftPartyActionAdd || repository.receivedAdd.PartyLinkID == "" {
		t.Fatalf("command = %#v", repository.receivedAdd)
	}
	if result.DraftVersion != 2 || result.Replayed {
		t.Fatalf("result = %#v", result)
	}
}

func TestAddDraftPartyValidation(t *testing.T) {
	longRole := strings.Repeat("角", 101)
	tests := []struct {
		name   string
		mutate func(*AddDraftPartyInput)
		field  string
	}{
		{name: "command", mutate: func(v *AddDraftPartyInput) { v.CommandID = "bad" }, field: "command_id"},
		{name: "contract", mutate: func(v *AddDraftPartyInput) { v.ContractID = "bad" }, field: "contract_id"},
		{name: "party", mutate: func(v *AddDraftPartyInput) { v.PartyID = "bad" }, field: "party_id"},
		{name: "empty role", mutate: func(v *AddDraftPartyInput) { v.Role = " " }, field: "role"},
		{name: "long role", mutate: func(v *AddDraftPartyInput) { v.Role = longRole }, field: "role"},
		{name: "version", mutate: func(v *AddDraftPartyInput) { v.ExpectedDraftVersion = 0 }, field: "expected_draft_version"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validAddDraftPartyInput(t)
			test.mutate(&input)
			repository := &repositoryStub{}
			_, err := NewService(repository).AddDraftParty(context.Background(), input)
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Field != test.field {
				t.Fatalf("error = %v, want ValidationError field %q", err, test.field)
			}
			if repository.receivedAdd.CommandID != "" {
				t.Fatalf("invalid input reached repository: %#v", repository.receivedAdd)
			}
		})
	}
}

func TestAddDraftPartyIdempotency(t *testing.T) {
	input := validAddDraftPartyInput(t)
	normalized, err := normalizeAddDraftPartyInput(input)
	if err != nil {
		t.Fatalf("normalizeAddDraftPartyInput() error = %v", err)
	}
	existing := DraftPartyCommand{
		CommandID:             normalized.CommandID,
		ContractID:            normalized.ContractID,
		Action:                DraftPartyActionAdd,
		PartyLinkID:           testLinkID,
		PartyID:               normalized.PartyID,
		Role:                  normalized.Role,
		ExpectedDraftVersion:  normalized.ExpectedDraftVersion,
		ResultingDraftVersion: 2,
	}
	repository := &repositoryStub{
		addCommand: existing,
		addParty:   DraftParty{ID: testLinkID, ContractID: testContractID, PartyID: testPartyID, Role: "委托方"},
		addCreated: false,
	}
	result, err := NewService(repository).AddDraftParty(context.Background(), input)
	if err != nil {
		t.Fatalf("AddDraftParty() error = %v", err)
	}
	if !result.Replayed || result.Party.ID != testLinkID || result.DraftVersion != 2 {
		t.Fatalf("result = %#v", result)
	}

	repository.addCommand.Role = "受托方"
	_, err = NewService(repository).AddDraftParty(context.Background(), input)
	var conflict *IdempotencyConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("changed replay error = %v, want IdempotencyConflictError", err)
	}
}

func TestRemoveDraftPartyNormalizesAndPersists(t *testing.T) {
	repository := &repositoryStub{}
	result, err := NewService(repository).RemoveDraftParty(context.Background(), validRemoveDraftPartyInput(t))
	if err != nil {
		t.Fatalf("RemoveDraftParty() error = %v", err)
	}
	if repository.receivedRemove.Action != DraftPartyActionRemove || repository.receivedRemove.PartyLinkID != testLinkID {
		t.Fatalf("command = %#v", repository.receivedRemove)
	}
	if result.DraftVersion != 3 || result.Replayed {
		t.Fatalf("result = %#v", result)
	}
}

func TestRemoveDraftPartyValidation(t *testing.T) {
	input := validRemoveDraftPartyInput(t)
	input.PartyLinkID = "bad"
	_, err := NewService(&repositoryStub{}).RemoveDraftParty(context.Background(), input)
	var validationError *ValidationError
	if !errors.As(err, &validationError) || validationError.Field != "party_link_id" {
		t.Fatalf("error = %v, want party_link_id ValidationError", err)
	}
}

func TestRemoveDraftPartyRejectsChangedReplay(t *testing.T) {
	input := validRemoveDraftPartyInput(t)
	existing := DraftPartyCommand{
		CommandID:             input.CommandID,
		ContractID:            input.ContractID,
		Action:                DraftPartyActionRemove,
		PartyLinkID:           "44444444-4444-4444-8444-444444444444",
		ExpectedDraftVersion:  input.ExpectedDraftVersion,
		ResultingDraftVersion: input.ExpectedDraftVersion + 1,
	}
	_, err := NewService(&repositoryStub{removeCommand: existing}).RemoveDraftParty(context.Background(), input)
	var conflict *IdempotencyConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %v, want IdempotencyConflictError", err)
	}
}
