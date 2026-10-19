package party

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

type Kind string

const (
	KindOrganization Kind = "organization"
	KindIndividual   Kind = "individual"

	AuditActionPartyCreated = "party_created"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

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
	CommandID              string
	Kind                   string
	LegalName              string
	RegistrationIdentifier string
	Actor                  audit.ActorRef
}

type CreateCommand struct {
	CommandID   string
	PayloadHash []byte
	Actor       audit.ActorRef
	CreatedAt   time.Time
}

type CreateResult struct {
	Party    Party
	Replayed bool
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

type IdempotencyConflictError struct {
	CommandID string
}

func (e *IdempotencyConflictError) Error() string {
	return "该 Command ID 已用于不同的主体登记意图"
}

type Repository interface {
	Create(context.Context, Party, CreateCommand, audit.Entry) (Party, bool, error)
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

func (s *Service) Create(ctx context.Context, input CreateInput) (CreateResult, error) {
	commandID := strings.ToLower(strings.TrimSpace(input.CommandID))
	if !uuidPattern.MatchString(commandID) {
		return CreateResult{}, &ValidationError{Field: "command_id", Message: "Command ID 无效，请刷新页面后重试"}
	}
	actor, err := audit.NormalizeActor(input.Actor)
	if err != nil {
		return CreateResult{}, &ValidationError{Field: "actor", Message: err.Error()}
	}
	kind := Kind(strings.ToLower(strings.TrimSpace(input.Kind)))
	if kind != KindOrganization && kind != KindIndividual {
		return CreateResult{}, &ValidationError{Field: "kind", Message: "请选择有效的主体类型"}
	}
	legalName := strings.TrimSpace(input.LegalName)
	if legalName == "" {
		return CreateResult{}, &ValidationError{Field: "legal_name", Message: "法定名称或姓名不能为空"}
	}
	if len([]rune(legalName)) > 200 {
		return CreateResult{}, &ValidationError{Field: "legal_name", Message: "法定名称或姓名不能超过 200 个字符"}
	}
	registrationIdentifier := strings.TrimSpace(input.RegistrationIdentifier)
	if len([]rune(registrationIdentifier)) > 100 {
		return CreateResult{}, &ValidationError{Field: "registration_identifier", Message: "登记标识不能超过 100 个字符"}
	}
	payloadHash, err := createPayloadHash(kind, legalName, registrationIdentifier, actor)
	if err != nil {
		return CreateResult{}, fmt.Errorf("hash party create intent: %w", err)
	}
	partyID, err := s.newID()
	if err != nil {
		return CreateResult{}, fmt.Errorf("generate party ID: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return CreateResult{}, fmt.Errorf("generate party audit ID: %w", err)
	}
	now := s.now()
	created := Party{
		ID:                     partyID,
		Kind:                   kind,
		LegalName:              legalName,
		RegistrationIdentifier: registrationIdentifier,
		Revision:               1,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	result, replayed, err := s.repository.Create(ctx, created, CreateCommand{
		CommandID:   commandID,
		PayloadHash: payloadHash,
		Actor:       actor,
		CreatedAt:   now,
	}, audit.Entry{
		ID:         auditID,
		ObjectType: audit.ObjectParty,
		ObjectID:   partyID,
		Action:     AuditActionPartyCreated,
		CommandID:  commandID,
		Actor:      actor,
		OccurredAt: now,
		Summary:    "创建 Money Flow 主体，revision 1",
		Details: map[string]any{
			"kind":                    kind,
			"legal_name":              legalName,
			"registration_identifier": registrationIdentifier,
			"party_revision":          int64(1),
		},
	})
	if err != nil {
		return CreateResult{}, fmt.Errorf("create party: %w", err)
	}
	return CreateResult{Party: result, Replayed: replayed}, nil
}

func (s *Service) List(ctx context.Context) ([]Party, error) {
	parties, err := s.repository.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list parties: %w", err)
	}
	return parties, nil
}

func NewCommandID() (string, error) {
	return randomUUID()
}

func createPayloadHash(kind Kind, legalName, registrationIdentifier string, actor audit.ActorRef) ([]byte, error) {
	payload, err := json.Marshal(struct {
		Format                 string `json:"format"`
		Kind                   Kind   `json:"kind"`
		LegalName              string `json:"legal_name"`
		RegistrationIdentifier string `json:"registration_identifier"`
		ActorKind              string `json:"actor_kind"`
		ActorID                string `json:"actor_id"`
	}{
		Format:                 "money-flow-party-create-v1",
		Kind:                   kind,
		LegalName:              legalName,
		RegistrationIdentifier: registrationIdentifier,
		ActorKind:              actor.Kind,
		ActorID:                actor.ID,
	})
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(payload)
	return hash[:], nil
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
