package projects

import (
	"context"
	"errors"
	"strings"

	personnel "omsbizfoundry/project-workspace/internal/referenceoa/productionpersonnel"
)

// Closed overlay service, selected explicitly by the composition. All HTTP
// access still authenticates and checks native actions/scope before calling it.
type OAProductionService struct {
	store  *PostgresStore
	people ReferenceValidator
}

func NewOAProductionService(store *PostgresStore, people ReferenceValidator) (*OAProductionService, error) {
	if store == nil || people == nil {
		return nil, errors.New("OA production store and People resolver are required")
	}
	return &OAProductionService{store: store, people: people}, nil
}
func (s *OAProductionService) Personnel(ctx context.Context, tenant, id string, includeEnded bool) (OAProductionView, error) {
	return s.store.OAProductionPersonnel(ctx, tenant, id, includeEnded)
}
func (s *OAProductionService) ProductionProfile(ctx context.Context, tenant, id string) (OAProductionProfileContext, error) {
	return s.store.OAProductionProfile(ctx, tenant, id)
}
func (s *OAProductionService) Enroll(ctx context.Context, a Actor, id string, in RevisionCommand) (OAProductionReceipt, error) {
	return s.store.EnrollOAProduction(ctx, a, id, in)
}
func (s *OAProductionService) SetParticipantApplicable(ctx context.Context, a Actor, id string, in RevisionCommand, notApplicable bool) (OAProductionReceipt, error) {
	return s.store.SetOAProductionParticipantApplicable(ctx, a, id, in, notApplicable)
}
func (s *OAProductionService) Disciplines(ctx context.Context, tenant string, includeArchived bool) ([]OADiscipline, error) {
	return s.store.OADisciplines(ctx, tenant, includeArchived)
}
func (s *OAProductionService) ChangeDiscipline(ctx context.Context, a Actor, in OADisciplineCommand) (OADiscipline, error) {
	return s.store.ApplyOADiscipline(ctx, a, in)
}
func (s *OAProductionService) ApplyPersonnel(ctx context.Context, a Actor, id string, input personnel.Intention) (OAProductionReceipt, error) {
	if !validOAActor(a, id) {
		return OAProductionReceipt{}, personnel.ErrInvalid
	}
	in, err := personnel.Normalize(input)
	if err != nil {
		return OAProductionReceipt{}, err
	}
	// The protected handler has already authorized this exact resource. Retry
	// the original intention before contacting People or reading a newer state.
	if receipt, found, err := s.store.ReplayOAProduction(ctx, a.TenantID, id, in); err != nil {
		return receipt, err
	} else if found {
		return receipt, nil
	}
	view, err := s.store.OAProductionPersonnel(ctx, a.TenantID, id, false)
	if err != nil {
		return OAProductionReceipt{}, err
	}
	if view.ProjectRevision != in.ExpectedProjectRevision {
		return OAProductionReceipt{}, personnel.ErrStale
	}
	if view.ProjectStatus == StatusCancelled {
		return OAProductionReceipt{}, personnel.ErrState
	}
	resolved := personnel.Resolved{People: map[personnel.Ref]personnel.ResolvedPerson{}}
	for _, change := range in.Changes {
		if change.Operation != personnel.Replace {
			continue
		}
		for _, ref := range change.People {
			if _, exists := resolved.People[ref]; exists {
				continue
			}
			value, err := s.people.ValidatePeople(ctx, a.TenantID, "person", ref.ID, ref.Revision)
			if err != nil {
				return OAProductionReceipt{}, err
			}
			if value.Revision != ref.Revision || strings.TrimSpace(value.DisplayName) == "" {
				return OAProductionReceipt{}, ErrReferenceInvalid
			}
			resolved.People[ref] = personnel.ResolvedPerson{TenantID: a.TenantID, State: "active", PersonSnapshot: personnel.PersonSnapshot{Ref: ref, Name: value.DisplayName}}
		}
	}
	// Project-owned catalogue is resolved and locked by the transaction itself.
	return s.store.ApplyOAProductionPersonnel(ctx, a, id, in, resolved)
}
