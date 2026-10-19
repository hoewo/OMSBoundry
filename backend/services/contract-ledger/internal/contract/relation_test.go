package contract

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const (
	testRelationSourceID = "11111111-1111-4111-8111-111111111111"
	testRelationTargetID = "22222222-2222-4222-8222-222222222222"
)

func validRelationInput(t *testing.T) LinkContractRelationInput {
	t.Helper()
	commandID, err := NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	return LinkContractRelationInput{
		CommandID: commandID, SourceContractID: testRelationSourceID, TargetContractID: testRelationTargetID,
		RelationType: " SUBCONTRACT_OF ", ExpectedSourceRevision: 3, ExpectedTargetRevision: 2,
		EffectiveOn: "2026-10-01", Note: " 目标合同承接本合同的分包工作 ", Actor: testActor,
	}
}

func TestLinkRelationNormalizesAndPersists(t *testing.T) {
	repository := &repositoryStub{}
	result, err := NewService(repository).LinkRelation(context.Background(), validRelationInput(t))
	if err != nil {
		t.Fatalf("LinkRelation() error = %v", err)
	}
	command := repository.receivedRelation
	if command.RelationID == "" || command.AuditID == "" || command.CommandID == "" {
		t.Fatalf("generated identifiers = %#v", command)
	}
	if command.SourceContractID != testRelationSourceID || command.TargetContractID != testRelationTargetID ||
		command.RelationType != RelationTypeSubcontractOf || command.ExpectedSourceRevision != 3 || command.ExpectedTargetRevision != 2 {
		t.Fatalf("normalized relation command = %#v", command)
	}
	if command.EffectiveOn != "2026-10-01" || command.Note != "目标合同承接本合同的分包工作" || command.Actor != testActor || command.OccurredAt.IsZero() {
		t.Fatalf("normalized relation facts = %#v", command)
	}
	if result.Relation.ID != command.RelationID || result.Relation.Type != RelationTypeSubcontractOf || result.Replayed {
		t.Fatalf("result = %#v", result)
	}
}

func TestLinkRelationRejectsInvalidInputsBeforeRepository(t *testing.T) {
	tests := []struct {
		name  string
		alter func(*LinkContractRelationInput)
		field string
	}{
		{name: "command ID", alter: func(value *LinkContractRelationInput) { value.CommandID = "bad" }, field: "command_id"},
		{name: "source ID", alter: func(value *LinkContractRelationInput) { value.SourceContractID = "bad" }, field: "source_contract_id"},
		{name: "target ID", alter: func(value *LinkContractRelationInput) { value.TargetContractID = "bad" }, field: "target_contract_id"},
		{name: "self", alter: func(value *LinkContractRelationInput) { value.TargetContractID = value.SourceContractID }, field: "target_contract_id"},
		{name: "type", alter: func(value *LinkContractRelationInput) { value.RelationType = "parent" }, field: "relation_type"},
		{name: "source revision", alter: func(value *LinkContractRelationInput) { value.ExpectedSourceRevision = 0 }, field: "expected_source_revision"},
		{name: "target revision", alter: func(value *LinkContractRelationInput) { value.ExpectedTargetRevision = 0 }, field: "expected_target_revision"},
		{name: "date", alter: func(value *LinkContractRelationInput) { value.EffectiveOn = "2026/10/01" }, field: "effective_on"},
		{name: "note", alter: func(value *LinkContractRelationInput) { value.Note = strings.Repeat("界", 501) }, field: "note"},
		{name: "actor", alter: func(value *LinkContractRelationInput) { value.Actor = ActorRef{} }, field: "actor"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validRelationInput(t)
			test.alter(&input)
			repository := &repositoryStub{}
			_, err := NewService(repository).LinkRelation(context.Background(), input)
			var validation *ValidationError
			if !errors.As(err, &validation) || validation.Field != test.field {
				t.Fatalf("error = %v, want ValidationError field %q", err, test.field)
			}
			if repository.receivedRelation.CommandID != "" {
				t.Fatalf("invalid input reached repository: %#v", repository.receivedRelation)
			}
		})
	}
}

func TestLinkRelationIdempotentReplayAndConflict(t *testing.T) {
	input := validRelationInput(t)
	firstRepository := &repositoryStub{}
	if _, err := NewService(firstRepository).LinkRelation(context.Background(), input); err != nil {
		t.Fatalf("first LinkRelation() error = %v", err)
	}
	persisted := firstRepository.receivedRelation
	relation := ContractRelation{
		ID:     persisted.RelationID,
		Source: ContractRelationSnapshot{ContractID: persisted.SourceContractID, Revision: persisted.ExpectedSourceRevision},
		Target: ContractRelationSnapshot{ContractID: persisted.TargetContractID, Revision: persisted.ExpectedTargetRevision},
		Type:   persisted.RelationType, EffectiveOn: persisted.EffectiveOn, Note: persisted.Note,
		Actor: persisted.Actor, CreatedAt: persisted.OccurredAt,
	}
	replayRepository := &repositoryStub{relationCommand: persisted, relation: relation}
	replayed, err := NewService(replayRepository).LinkRelation(context.Background(), input)
	if err != nil || !replayed.Replayed || replayed.Relation.ID != relation.ID {
		t.Fatalf("replayed LinkRelation() = %#v, error = %v", replayed, err)
	}

	changed := input
	changed.Note = "改变后的关系说明"
	_, err = NewService(replayRepository).LinkRelation(context.Background(), changed)
	var conflict *IdempotencyConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("changed replay error = %v, want IdempotencyConflictError", err)
	}
}

func TestRelationTypesHaveStableLabelsAndCyclePolicy(t *testing.T) {
	types := []RelationType{
		RelationTypeFundingSourceFor,
		RelationTypeEntrustmentBasisFor,
		RelationTypeSubcontractOf,
		RelationTypeSpecificUnderFramework,
		RelationTypeSupersedes,
		RelationTypeReferences,
	}
	for _, relationType := range types {
		if !relationType.Valid() || relationType.Label() == "" {
			t.Fatalf("relation type %q valid=%t label=%q", relationType, relationType.Valid(), relationType.Label())
		}
		if got, want := relationType.Acyclic(), relationType != RelationTypeReferences; got != want {
			t.Fatalf("relation type %q Acyclic() = %t, want %t", relationType, got, want)
		}
	}
	if RelationType("unknown").Valid() || RelationType("unknown").Acyclic() {
		t.Fatal("unknown relation type accepted")
	}
}
