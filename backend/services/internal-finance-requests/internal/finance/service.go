package finance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type Store interface {
	Ping(context.Context) error
	Create(context.Context, Actor, CreateInput, string) (Request, error)
	Update(context.Context, Actor, string, UpdateInput, string) (Request, error)
	Get(context.Context, string, string) (Request, error)
	List(context.Context, string, ListFilter) ([]Request, error)
}
type Service struct{ store Store }

func NewService(store Store) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("store is required")
	}
	return &Service{store: store}, nil
}
func (s *Service) Ping(ctx context.Context) error { return s.store.Ping(ctx) }
func (s *Service) Create(ctx context.Context, actor Actor, input CreateInput) (Request, error) {
	normalizeCreate(&input)
	if err := validateActor(actor); err != nil {
		return Request{}, err
	}
	if err := validateCreate(input); err != nil {
		return Request{}, err
	}
	return s.store.Create(ctx, actor, input, hash(input))
}
func (s *Service) Update(ctx context.Context, actor Actor, id string, input UpdateInput) (Request, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	input.Title = strings.TrimSpace(input.Title)
	if err := validateActor(actor); err != nil {
		return Request{}, err
	}
	if !uuidPattern.MatchString(id) || !uuidPattern.MatchString(input.CommandID) || input.ExpectedRevision < 1 || input.Title == "" || len(input.Title) > 200 || input.AmountMinor < 1 {
		return Request{}, fmt.Errorf("invalid update")
	}
	return s.store.Update(ctx, actor, id, input, hash(input))
}
func (s *Service) Get(ctx context.Context, tenant, id string) (Request, error) {
	if !uuidPattern.MatchString(tenant) || !uuidPattern.MatchString(id) {
		return Request{}, fmt.Errorf("invalid identity")
	}
	return s.store.Get(ctx, tenant, id)
}
func (s *Service) List(ctx context.Context, tenant string, filter ListFilter) (Page, error) {
	if !uuidPattern.MatchString(tenant) {
		return Page{}, fmt.Errorf("invalid tenant")
	}
	if filter.ApplicantSubjectID != "" && !uuidPattern.MatchString(filter.ApplicantSubjectID) {
		return Page{}, fmt.Errorf("invalid applicant")
	}
	items, err := s.store.List(ctx, tenant, filter)
	return Page{Items: items}, err
}
func normalizeCreate(input *CreateInput) {
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	input.RequestType = strings.ToLower(strings.TrimSpace(input.RequestType))
	input.Title = strings.TrimSpace(input.Title)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	if input.Currency == "" {
		input.Currency = "CNY"
	}
}
func validateCreate(input CreateInput) error {
	if !uuidPattern.MatchString(input.CommandID) || (input.RequestType != "reimbursement" && input.RequestType != "payment") || input.Title == "" || len(input.Title) > 200 || input.AmountMinor < 1 || input.Currency != "CNY" {
		return fmt.Errorf("invalid finance request")
	}
	return nil
}
func validateActor(actor Actor) error {
	if !uuidPattern.MatchString(actor.ID) || !uuidPattern.MatchString(actor.TenantID) || actor.CredentialID == "" {
		return fmt.Errorf("invalid actor")
	}
	return nil
}
func hash(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
