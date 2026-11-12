package payroll

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var (
	uuidPattern   = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	periodPattern = regexp.MustCompile(`^[0-9]{4}-(0[1-9]|1[0-2])$`)
	datePattern   = regexp.MustCompile(`^[0-9]{4}-(0[1-9]|1[0-2])-([0-2][0-9]|3[0-1])$`)
)

const maxPayrollMinor int64 = 900_000_000_000_000

type Store interface {
	Ping(context.Context) error
	Create(context.Context, Actor, CreateInput, string) (Run, error)
	Update(context.Context, Actor, string, UpdateInput, string) (Run, error)
	Get(context.Context, string, string) (Run, error)
	List(context.Context, string) ([]Run, error)
}
type Service struct {
	store            Store
	personReferences PersonReferencePort
	paymentDelivery  PaymentDeliveryPort
	attendance       AttendanceSummaryPort
}

func NewService(store Store, dependencies ...any) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("store is required")
	}
	service := &Service{store: store}
	for _, dependency := range dependencies {
		switch value := dependency.(type) {
		case PersonReferencePort:
			if service.personReferences != nil {
				return nil, fmt.Errorf("at most one person reference port is allowed")
			}
			service.personReferences = value
		case PaymentDeliveryPort:
			if service.paymentDelivery != nil {
				return nil, fmt.Errorf("at most one payment delivery port is allowed")
			}
			service.paymentDelivery = value
		case AttendanceSummaryPort:
			if service.attendance != nil {
				return nil, fmt.Errorf("at most one attendance summary port is allowed")
			}
			service.attendance = value
		default:
			return nil, fmt.Errorf("unsupported payroll dependency")
		}
	}
	return service, nil
}
func (s *Service) Ping(ctx context.Context) error { return s.store.Ping(ctx) }
func (s *Service) Create(ctx context.Context, actor Actor, input CreateInput) (Run, error) {
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	input.Period = strings.TrimSpace(input.Period)
	input.Label = strings.TrimSpace(input.Label)
	if err := validateActor(actor); err != nil {
		return Run{}, err
	}
	if !uuidPattern.MatchString(input.CommandID) || !periodPattern.MatchString(input.Period) || input.Label == "" || len(input.Label) > 200 {
		return Run{}, fmt.Errorf("invalid payroll run")
	}
	return s.store.Create(ctx, actor, input, hash(input))
}
func (s *Service) Update(ctx context.Context, actor Actor, id string, input UpdateInput) (Run, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	input.Label = strings.TrimSpace(input.Label)
	if err := validateActor(actor); err != nil {
		return Run{}, err
	}
	if !uuidPattern.MatchString(id) || !uuidPattern.MatchString(input.CommandID) || input.ExpectedRevision < 1 || input.Label == "" || len(input.Label) > 200 {
		return Run{}, fmt.Errorf("invalid update")
	}
	return s.store.Update(ctx, actor, id, input, hash(input))
}
func (s *Service) Get(ctx context.Context, tenant, id string) (Run, error) {
	if !uuidPattern.MatchString(tenant) || !uuidPattern.MatchString(id) {
		return Run{}, fmt.Errorf("invalid identity")
	}
	return s.store.Get(ctx, tenant, id)
}
func (s *Service) List(ctx context.Context, tenant string) (Page, error) {
	if !uuidPattern.MatchString(tenant) {
		return Page{}, fmt.Errorf("invalid tenant")
	}
	items, err := s.store.List(ctx, tenant)
	return Page{Items: items}, err
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
