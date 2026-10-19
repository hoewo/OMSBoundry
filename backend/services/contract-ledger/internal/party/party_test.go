package party

import (
	"context"
	"errors"
	"strings"
	"testing"

	"omsbizfoundry/contract-ledger/internal/audit"
)

var testActor = audit.ActorRef{
	Kind:        audit.ActorKindLocalUser,
	ID:          "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
	DisplayName: "测试用户",
}

type memoryRepository struct {
	parties   []Party
	createErr error
	listErr   error
}

func (r *memoryRepository) Create(_ context.Context, value Party, _ audit.Entry) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.parties = append(r.parties, value)
	return nil
}

func (r *memoryRepository) List(context.Context) ([]Party, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return append([]Party(nil), r.parties...), nil
}

func TestServiceCreateNormalizesAndPersistsParty(t *testing.T) {
	repository := &memoryRepository{}
	service := NewService(repository)

	created, err := service.Create(context.Background(), CreateInput{
		Kind:                   " organization ",
		LegalName:              "  示例设计有限公司  ",
		RegistrationIdentifier: " 91330000EXAMPLE ",
		Actor:                  testActor,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == "" {
		t.Fatal("Create() ID is empty")
	}
	if created.Kind != KindOrganization || created.LegalName != "示例设计有限公司" || created.RegistrationIdentifier != "91330000EXAMPLE" {
		t.Fatalf("Create() = %#v", created)
	}
	if created.Revision != 1 || created.CreatedAt.IsZero() || !created.CreatedAt.Equal(created.UpdatedAt) {
		t.Fatalf("Create() metadata = %#v", created)
	}
	if len(repository.parties) != 1 || repository.parties[0].ID != created.ID {
		t.Fatalf("repository parties = %#v", repository.parties)
	}
}

func TestServiceCreateValidation(t *testing.T) {
	tests := []struct {
		name  string
		input CreateInput
		field string
	}{
		{name: "invalid kind", input: CreateInput{Kind: "vendor", LegalName: "示例"}, field: "kind"},
		{name: "empty legal name", input: CreateInput{Kind: "organization", LegalName: "  "}, field: "legal_name"},
		{name: "long legal name", input: CreateInput{Kind: "organization", LegalName: strings.Repeat("名", 201)}, field: "legal_name"},
		{name: "long registration identifier", input: CreateInput{Kind: "individual", LegalName: "张三", RegistrationIdentifier: strings.Repeat("A", 101)}, field: "registration_identifier"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &memoryRepository{}
			test.input.Actor = testActor
			_, err := NewService(repository).Create(context.Background(), test.input)
			var validationError *ValidationError
			if !errors.As(err, &validationError) {
				t.Fatalf("Create() error = %v, want ValidationError", err)
			}
			if validationError.Field != test.field {
				t.Fatalf("ValidationError.Field = %q, want %q", validationError.Field, test.field)
			}
			if len(repository.parties) != 0 {
				t.Fatalf("invalid party was persisted: %#v", repository.parties)
			}
		})
	}
}

func TestServicePropagatesRepositoryErrors(t *testing.T) {
	repository := &memoryRepository{createErr: errors.New("write failed")}
	_, err := NewService(repository).Create(context.Background(), CreateInput{Kind: "organization", LegalName: "示例", Actor: testActor})
	if err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("Create() error = %v", err)
	}
}
