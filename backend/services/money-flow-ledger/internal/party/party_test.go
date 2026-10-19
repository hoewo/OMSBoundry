package party

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

const (
	testCommandID = "11111111-1111-4111-8111-111111111111"
	testActorID   = "22222222-2222-4222-8222-222222222222"
	testPartyID   = "33333333-3333-4333-8333-333333333333"
	testAuditID   = "44444444-4444-4444-8444-444444444444"
)

type repositoryStub struct {
	createdParty   Party
	createdCommand CreateCommand
	createdAudit   audit.Entry
	createResult   Party
	replayed       bool
	createErr      error
	listResult     []Party
	listErr        error
}

func (r *repositoryStub) Create(_ context.Context, value Party, command CreateCommand, entry audit.Entry) (Party, bool, error) {
	r.createdParty = value
	r.createdCommand = command
	r.createdAudit = entry
	if r.createErr != nil {
		return Party{}, false, r.createErr
	}
	if r.createResult.ID != "" {
		return r.createResult, r.replayed, nil
	}
	return value, r.replayed, nil
}

func (r *repositoryStub) List(context.Context) ([]Party, error) {
	return r.listResult, r.listErr
}

func testActor() audit.ActorRef {
	return audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: testActorID, DisplayName: "资金管理员"}
}

func testService(repository Repository) *Service {
	service := NewService(repository)
	ids := []string{testPartyID, testAuditID}
	service.newID = func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}
	service.now = func() time.Time {
		return time.Date(2026, 9, 19, 11, 30, 0, 0, time.UTC)
	}
	return service
}

func TestCreateNormalizesFactsAndBuildsTrustedAudit(t *testing.T) {
	repository := &repositoryStub{}
	service := testService(repository)

	result, err := service.Create(context.Background(), CreateInput{
		CommandID:              " 11111111-1111-4111-8111-111111111111 ",
		Kind:                   " ORGANIZATION ",
		LegalName:              "  示例科技有限公司  ",
		RegistrationIdentifier: "  91310000TEST  ",
		Actor:                  testActor(),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.Replayed || result.Party.ID != testPartyID || result.Party.Kind != KindOrganization ||
		result.Party.LegalName != "示例科技有限公司" || result.Party.RegistrationIdentifier != "91310000TEST" ||
		result.Party.Revision != 1 || !result.Party.CreatedAt.Equal(result.Party.UpdatedAt) {
		t.Fatalf("result = %#v", result)
	}
	if repository.createdCommand.CommandID != testCommandID || len(repository.createdCommand.PayloadHash) != 32 ||
		repository.createdCommand.Actor != testActor() {
		t.Fatalf("command = %#v", repository.createdCommand)
	}
	entry := repository.createdAudit
	if entry.ID != testAuditID || entry.ObjectType != audit.ObjectParty || entry.ObjectID != testPartyID ||
		entry.Action != AuditActionPartyCreated || entry.CommandID != testCommandID || entry.Actor != testActor() {
		t.Fatalf("audit = %#v", entry)
	}
	if entry.Details["kind"] != KindOrganization || entry.Details["legal_name"] != "示例科技有限公司" ||
		entry.Details["registration_identifier"] != "91310000TEST" || entry.Details["party_revision"] != int64(1) {
		t.Fatalf("audit details = %#v", entry.Details)
	}
}

func TestCreateValidation(t *testing.T) {
	tests := []struct {
		name  string
		input CreateInput
		field string
	}{
		{name: "command", input: CreateInput{CommandID: "not-a-uuid", Kind: string(KindOrganization), LegalName: "主体", Actor: testActor()}, field: "command_id"},
		{name: "actor", input: CreateInput{CommandID: testCommandID, Kind: string(KindOrganization), LegalName: "主体", Actor: audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: "forged"}}, field: "actor"},
		{name: "kind", input: CreateInput{CommandID: testCommandID, Kind: "customer", LegalName: "主体", Actor: testActor()}, field: "kind"},
		{name: "name empty", input: CreateInput{CommandID: testCommandID, Kind: string(KindOrganization), LegalName: "  ", Actor: testActor()}, field: "legal_name"},
		{name: "name long", input: CreateInput{CommandID: testCommandID, Kind: string(KindOrganization), LegalName: runeString(201), Actor: testActor()}, field: "legal_name"},
		{name: "identifier long", input: CreateInput{CommandID: testCommandID, Kind: string(KindIndividual), LegalName: "个人", RegistrationIdentifier: runeString(101), Actor: testActor()}, field: "registration_identifier"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &repositoryStub{}
			_, err := testService(repository).Create(context.Background(), test.input)
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Field != test.field {
				t.Fatalf("Create() error = %v", err)
			}
			if repository.createdParty.ID != "" {
				t.Fatalf("repository called with %#v", repository.createdParty)
			}
		})
	}
}

func TestCreatePayloadHashUsesNormalizedIntentAndActor(t *testing.T) {
	firstRepository := &repositoryStub{}
	secondRepository := &repositoryStub{}
	firstService := testService(firstRepository)
	secondService := testService(secondRepository)

	first, err := firstService.Create(context.Background(), CreateInput{
		CommandID: testCommandID, Kind: " ORGANIZATION ", LegalName: " 示例公司 ", RegistrationIdentifier: "  ABC  ", Actor: testActor(),
	})
	if err != nil {
		t.Fatalf("first Create() error = %v", err)
	}
	second, err := secondService.Create(context.Background(), CreateInput{
		CommandID: testCommandID, Kind: "organization", LegalName: "示例公司", RegistrationIdentifier: "ABC", Actor: testActor(),
	})
	if err != nil {
		t.Fatalf("second Create() error = %v", err)
	}
	if first.Party.LegalName != second.Party.LegalName || !bytes.Equal(firstRepository.createdCommand.PayloadHash, secondRepository.createdCommand.PayloadHash) {
		t.Fatalf("normalized hashes differ: %x %x", firstRepository.createdCommand.PayloadHash, secondRepository.createdCommand.PayloadHash)
	}

	changedActorHash, err := createPayloadHash(KindOrganization, "示例公司", "ABC", audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "55555555-5555-4555-8555-555555555555", DisplayName: "另一管理员",
	})
	if err != nil {
		t.Fatalf("createPayloadHash() error = %v", err)
	}
	if bytes.Equal(firstRepository.createdCommand.PayloadHash, changedActorHash) {
		t.Fatal("actor was not included in payload hash")
	}
}

func TestCreateReturnsRepositoryReplayAndPreservesErrors(t *testing.T) {
	replayedParty := Party{ID: "66666666-6666-4666-8666-666666666666", Kind: KindIndividual, LegalName: "原主体", Revision: 1}
	repository := &repositoryStub{createResult: replayedParty, replayed: true}
	result, err := testService(repository).Create(context.Background(), CreateInput{
		CommandID: testCommandID, Kind: string(KindIndividual), LegalName: "原主体", Actor: testActor(),
	})
	if err != nil || !result.Replayed || result.Party.ID != replayedParty.ID {
		t.Fatalf("Create() = %#v, %v", result, err)
	}

	conflict := &IdempotencyConflictError{CommandID: testCommandID}
	repository = &repositoryStub{createErr: conflict}
	_, err = testService(repository).Create(context.Background(), CreateInput{
		CommandID: testCommandID, Kind: string(KindIndividual), LegalName: "原主体", Actor: testActor(),
	})
	var conflictError *IdempotencyConflictError
	if !errors.As(err, &conflictError) {
		t.Fatalf("Create() conflict error = %v", err)
	}
}

func TestListReturnsRepositoryFactsAndError(t *testing.T) {
	want := []Party{{ID: testPartyID, Kind: KindOrganization, LegalName: "甲公司", Revision: 1}}
	service := NewService(&repositoryStub{listResult: want})
	got, err := service.List(context.Background())
	if err != nil || len(got) != 1 || got[0] != want[0] {
		t.Fatalf("List() = %#v, %v", got, err)
	}

	sentinel := errors.New("database unavailable")
	service = NewService(&repositoryStub{listErr: sentinel})
	_, err = service.List(context.Background())
	if !errors.Is(err, sentinel) {
		t.Fatalf("List() error = %v", err)
	}
}

func TestNewCommandID(t *testing.T) {
	id, err := NewCommandID()
	if err != nil || !uuidPattern.MatchString(id) {
		t.Fatalf("NewCommandID() = %q, %v", id, err)
	}
}

func runeString(length int) string {
	value := make([]rune, length)
	for i := range value {
		value[i] = '界'
	}
	return string(value)
}
