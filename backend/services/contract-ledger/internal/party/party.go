package party

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"omsbizfoundry/contract-ledger/internal/audit"
)

type Kind string

const (
	KindOrganization Kind = "organization"
	KindIndividual   Kind = "individual"

	AuditActionPartyCreated = "party_created"
)

type Party struct {
	ID                     string
	Kind                   Kind
	LegalName              string
	RegistrationIdentifier string
	Revision               int64
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type CreateInput struct {
	Kind                   string
	LegalName              string
	RegistrationIdentifier string
	Actor                  audit.ActorRef
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

type Repository interface {
	Create(context.Context, Party, audit.Entry) error
	List(context.Context) ([]Party, error)
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

func (s *Service) Create(ctx context.Context, input CreateInput) (Party, error) {
	actor, err := audit.NormalizeActor(input.Actor)
	if err != nil {
		return Party{}, &ValidationError{Field: "actor", Message: err.Error()}
	}
	kind := Kind(strings.TrimSpace(input.Kind))
	if kind != KindOrganization && kind != KindIndividual {
		return Party{}, &ValidationError{Field: "kind", Message: "请选择有效的主体类型"}
	}

	legalName := strings.TrimSpace(input.LegalName)
	if legalName == "" {
		return Party{}, &ValidationError{Field: "legal_name", Message: "法定名称或姓名不能为空"}
	}
	if len([]rune(legalName)) > 200 {
		return Party{}, &ValidationError{Field: "legal_name", Message: "法定名称或姓名不能超过 200 个字符"}
	}

	registrationIdentifier := strings.TrimSpace(input.RegistrationIdentifier)
	if len([]rune(registrationIdentifier)) > 100 {
		return Party{}, &ValidationError{Field: "registration_identifier", Message: "登记标识不能超过 100 个字符"}
	}

	id, err := s.newID()
	if err != nil {
		return Party{}, fmt.Errorf("generate party ID: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return Party{}, fmt.Errorf("generate party audit ID: %w", err)
	}
	now := s.now()
	created := Party{
		ID:                     id,
		Kind:                   kind,
		LegalName:              legalName,
		RegistrationIdentifier: registrationIdentifier,
		Revision:               1,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	if err := s.repository.Create(ctx, created, audit.Entry{
		ID:         auditID,
		ObjectType: audit.ObjectParty,
		ObjectID:   created.ID,
		Action:     AuditActionPartyCreated,
		Actor:      actor,
		OccurredAt: now,
		Summary:    "创建主体，revision 1",
		Details:    map[string]any{"party_revision": created.Revision},
	}); err != nil {
		return Party{}, fmt.Errorf("create party: %w", err)
	}
	return created, nil
}

func (s *Service) List(ctx context.Context) ([]Party, error) {
	parties, err := s.repository.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list parties: %w", err)
	}
	return parties, nil
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
