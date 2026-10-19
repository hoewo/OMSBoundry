package costoutsourcing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type Store interface {
	Ping(context.Context) error
	Replay(context.Context, Actor, string, string, string, any) (bool, error)
	RecordCost(context.Context, Actor, RecordCostInput, ProjectSnapshot, []EvidenceSnapshot, string) (CostEntry, error)
	GetCost(context.Context, string, string) (CostEntry, error)
	ListCosts(context.Context, string, string) ([]CostEntry, error)
	CostHistory(context.Context, string, string) ([]CostRevision, error)
	CorrectCost(context.Context, Actor, string, CorrectCostInput, []EvidenceSnapshot, string) (CostEntry, error)
	WithdrawCost(context.Context, Actor, string, WithdrawInput, string) (CostEntry, error)
	RecordEngagement(context.Context, Actor, RecordEngagementInput, ProjectSnapshot, PartySnapshot, *ContractSnapshot, []EvidenceSnapshot, string) (Engagement, error)
	GetEngagement(context.Context, string, string) (Engagement, error)
	ListEngagements(context.Context, string, string) ([]Engagement, error)
	EngagementHistory(context.Context, string, string) ([]EngagementRevision, error)
	CorrectEngagement(context.Context, Actor, string, CorrectEngagementInput, PartySnapshot, *ContractSnapshot, []EvidenceSnapshot, string) (Engagement, error)
	WithdrawEngagement(context.Context, Actor, string, WithdrawInput, string) (Engagement, error)
	Assess(context.Context, Actor, string, AssessInput, string) (Assessment, error)
	AssessmentHistory(context.Context, string, string) ([]Assessment, error)
	ListLatestAssessments(context.Context, string, string) ([]Assessment, error)
	CreateSuggestion(context.Context, Actor, PayableSuggestion, string, string) (PayableSuggestion, error)
	GetSuggestion(context.Context, string, string) (PayableSuggestion, error)
	ListSuggestions(context.Context, string, string) ([]PayableSuggestion, error)
	SuggestionHistory(context.Context, string, string) ([]PayableSuggestionRevision, error)
	ApplySuggestionTarget(context.Context, Actor, string, int64, string, string, string, string, PayableSuggestion) (PayableSuggestion, error)
}

type ReferenceResolver interface {
	ResolveProject(context.Context, string, ProjectRef) (ProjectReference, error)
	ResolveParty(context.Context, string, PartyRef) (PartyReference, error)
	ResolveContract(context.Context, string, ProjectRef, PartyRef, ContractRef) (ContractReference, error)
	ResolveEvidence(context.Context, string, EvidenceRef) (EvidenceReference, error)
}

type MoneyPort interface {
	SubmitPayableSuggestion(context.Context, string, string, PayableSubmission) (PayableIntake, error)
	GetPayableSuggestion(context.Context, string, string) (PayableIntake, error)
	WithdrawPayableSuggestion(context.Context, string, string, string, int64, string) (PayableIntake, error)
}

type Service struct {
	store    Store
	resolver ReferenceResolver
	money    MoneyPort
}

func NewService(store Store, resolver ReferenceResolver, money ...MoneyPort) (*Service, error) {
	if store == nil || resolver == nil {
		return nil, fmt.Errorf("store and reference resolver are required")
	}
	if len(money) > 1 {
		return nil, fmt.Errorf("at most one Money port may be configured")
	}
	service := &Service{store: store, resolver: resolver}
	if len(money) == 1 {
		if money[0] == nil {
			return nil, fmt.Errorf("Money port cannot be nil")
		}
		service.money = money[0]
	}
	return service, nil
}

func (s *Service) Ping(ctx context.Context) error { return s.store.Ping(ctx) }

func (s *Service) RecordCost(ctx context.Context, actor Actor, input RecordCostInput) (CostEntry, error) {
	actor, err := authorize(actor, ActionCostCreate, input.ProjectRef.ID)
	if err != nil {
		return CostEntry{}, err
	}
	input, err = normalizeRecordCost(input)
	if err != nil {
		return CostEntry{}, err
	}
	if len(input.EvidenceRefs) > 0 {
		if _, err = authorize(actor, ActionCostEvidenceManage, input.ProjectRef.ID); err != nil {
			return CostEntry{}, err
		}
	}
	hash, err := requestHash("record_cost", "", actor.ID, input)
	if err != nil {
		return CostEntry{}, err
	}
	var replay CostEntry
	if found, replayErr := s.store.Replay(ctx, actor, input.CommandID, hash, "cost_entry", &replay); replayErr != nil || found {
		return replay, replayErr
	}
	project, err := s.resolveProject(ctx, actor.TenantID, input.ProjectRef)
	if err != nil {
		return CostEntry{}, err
	}
	evidence, err := s.resolveEvidence(ctx, actor.TenantID, input.EvidenceRefs)
	if err != nil {
		return CostEntry{}, err
	}
	return s.store.RecordCost(ctx, actor, input, project, evidence, hash)
}

func (s *Service) CorrectCost(ctx context.Context, actor Actor, id string, input CorrectCostInput) (CostEntry, error) {
	actor, id, err := normalizeMutationActor(actor, id)
	if err != nil {
		return CostEntry{}, err
	}
	current, err := s.store.GetCost(ctx, actor.TenantID, id)
	if err != nil {
		return CostEntry{}, err
	}
	if _, err = authorizeResource(actor, ActionCostCorrect, current.Project.ID, "cost_entry", current.ID); err != nil {
		return CostEntry{}, err
	}
	input, err = normalizeCorrectCost(input)
	if err != nil {
		return CostEntry{}, err
	}
	if evidenceRefsChanged(current.Evidence, input.EvidenceRefs) {
		if _, err = authorizeResource(actor, ActionCostEvidenceManage, current.Project.ID, "cost_entry", current.ID); err != nil {
			return CostEntry{}, err
		}
	}
	hash, err := requestHash("correct_cost", id, actor.ID, input)
	if err != nil {
		return CostEntry{}, err
	}
	var replay CostEntry
	if found, replayErr := s.store.Replay(ctx, actor, input.CommandID, hash, "cost_entry", &replay); replayErr != nil || found {
		return replay, replayErr
	}
	if _, err = s.resolveProject(ctx, actor.TenantID, ProjectRef{ID: current.Project.ID}); err != nil {
		return CostEntry{}, err
	}
	evidence, err := s.resolveEvidence(ctx, actor.TenantID, input.EvidenceRefs)
	if err != nil {
		return CostEntry{}, err
	}
	return s.store.CorrectCost(ctx, actor, id, input, evidence, hash)
}

func (s *Service) WithdrawCost(ctx context.Context, actor Actor, id string, input WithdrawInput) (CostEntry, error) {
	actor, id, err := normalizeMutationActor(actor, id)
	if err != nil {
		return CostEntry{}, err
	}
	current, err := s.store.GetCost(ctx, actor.TenantID, id)
	if err != nil {
		return CostEntry{}, err
	}
	if _, err = authorizeResource(actor, ActionCostWithdraw, current.Project.ID, "cost_entry", current.ID); err != nil {
		return CostEntry{}, err
	}
	input, err = normalizeWithdraw(input)
	if err != nil {
		return CostEntry{}, err
	}
	hash, err := requestHash("withdraw_cost", id, actor.ID, input)
	if err != nil {
		return CostEntry{}, err
	}
	var replay CostEntry
	if found, replayErr := s.store.Replay(ctx, actor, input.CommandID, hash, "cost_entry", &replay); replayErr != nil || found {
		return replay, replayErr
	}
	return s.store.WithdrawCost(ctx, actor, id, input, hash)
}

func (s *Service) GetCost(ctx context.Context, actor Actor, id string) (CostEntry, error) {
	actor, id, err := normalizeMutationActor(actor, id)
	if err != nil {
		return CostEntry{}, err
	}
	value, err := s.store.GetCost(ctx, actor.TenantID, id)
	if err != nil {
		return CostEntry{}, err
	}
	if _, err = authorizeResource(actor, ActionCostRead, value.Project.ID, "cost_entry", value.ID); err != nil {
		return CostEntry{}, err
	}
	return value, nil
}

func (s *Service) ListCosts(ctx context.Context, actor Actor, projectID string) ([]CostEntry, error) {
	actor, err := authorize(actor, ActionCostRead, projectID)
	if err != nil {
		return nil, err
	}
	return s.store.ListCosts(ctx, actor.TenantID, normalizeUUID(projectID))
}

func (s *Service) CostHistory(ctx context.Context, actor Actor, id string) ([]CostRevision, error) {
	actor, id, err := normalizeMutationActor(actor, id)
	if err != nil {
		return nil, err
	}
	value, err := s.store.GetCost(ctx, actor.TenantID, id)
	if err != nil {
		return nil, err
	}
	if _, err = authorizeResource(actor, ActionCostHistory, value.Project.ID, "cost_entry", value.ID); err != nil {
		return nil, err
	}
	return s.store.CostHistory(ctx, actor.TenantID, id)
}

func (s *Service) RecordEngagement(ctx context.Context, actor Actor, input RecordEngagementInput) (Engagement, error) {
	actor, err := authorize(actor, ActionEngageCreate, input.ProjectRef.ID)
	if err != nil {
		return Engagement{}, err
	}
	input, err = normalizeRecordEngagement(input)
	if err != nil {
		return Engagement{}, err
	}
	if len(input.EvidenceRefs) > 0 {
		if _, err = authorize(actor, ActionEngageEvidenceManage, input.ProjectRef.ID); err != nil {
			return Engagement{}, err
		}
	}
	hash, err := requestHash("record_engagement", "", actor.ID, input)
	if err != nil {
		return Engagement{}, err
	}
	var replay Engagement
	if found, replayErr := s.store.Replay(ctx, actor, input.CommandID, hash, "outsourcing_engagement", &replay); replayErr != nil || found {
		return replay, replayErr
	}
	project, err := s.resolveProject(ctx, actor.TenantID, input.ProjectRef)
	if err != nil {
		return Engagement{}, err
	}
	provider, contract, evidence, err := s.resolveEngagementRefs(ctx, actor.TenantID, project, input.ProviderRef, input.ContractRef, input.EvidenceRefs, input.Currency)
	if err != nil {
		return Engagement{}, err
	}
	return s.store.RecordEngagement(ctx, actor, input, project, provider, contract, evidence, hash)
}

func (s *Service) CorrectEngagement(ctx context.Context, actor Actor, id string, input CorrectEngagementInput) (Engagement, error) {
	actor, id, err := normalizeMutationActor(actor, id)
	if err != nil {
		return Engagement{}, err
	}
	current, err := s.store.GetEngagement(ctx, actor.TenantID, id)
	if err != nil {
		return Engagement{}, err
	}
	if _, err = authorizeResource(actor, ActionEngageCorrect, current.Project.ID, "outsourcing_engagement", current.ID); err != nil {
		return Engagement{}, err
	}
	input, err = normalizeCorrectEngagement(input)
	if err != nil {
		return Engagement{}, err
	}
	if evidenceRefsChanged(current.Evidence, input.EvidenceRefs) {
		if _, err = authorizeResource(actor, ActionEngageEvidenceManage, current.Project.ID, "outsourcing_engagement", current.ID); err != nil {
			return Engagement{}, err
		}
	}
	hash, err := requestHash("correct_engagement", id, actor.ID, input)
	if err != nil {
		return Engagement{}, err
	}
	var replay Engagement
	if found, replayErr := s.store.Replay(ctx, actor, input.CommandID, hash, "outsourcing_engagement", &replay); replayErr != nil || found {
		return replay, replayErr
	}
	project, err := s.resolveProject(ctx, actor.TenantID, ProjectRef{ID: current.Project.ID})
	if err != nil {
		return Engagement{}, err
	}
	provider, contract, evidence, err := s.resolveEngagementRefs(ctx, actor.TenantID, project, input.ProviderRef, input.ContractRef, input.EvidenceRefs, input.Currency)
	if err != nil {
		return Engagement{}, err
	}
	return s.store.CorrectEngagement(ctx, actor, id, input, provider, contract, evidence, hash)
}

func (s *Service) WithdrawEngagement(ctx context.Context, actor Actor, id string, input WithdrawInput) (Engagement, error) {
	actor, id, err := normalizeMutationActor(actor, id)
	if err != nil {
		return Engagement{}, err
	}
	current, err := s.store.GetEngagement(ctx, actor.TenantID, id)
	if err != nil {
		return Engagement{}, err
	}
	if _, err = authorizeResource(actor, ActionEngageWithdraw, current.Project.ID, "outsourcing_engagement", current.ID); err != nil {
		return Engagement{}, err
	}
	input, err = normalizeWithdraw(input)
	if err != nil {
		return Engagement{}, err
	}
	hash, err := requestHash("withdraw_engagement", id, actor.ID, input)
	if err != nil {
		return Engagement{}, err
	}
	var replay Engagement
	if found, replayErr := s.store.Replay(ctx, actor, input.CommandID, hash, "outsourcing_engagement", &replay); replayErr != nil || found {
		return replay, replayErr
	}
	return s.store.WithdrawEngagement(ctx, actor, id, input, hash)
}

func (s *Service) Assess(ctx context.Context, actor Actor, engagementID string, input AssessInput) (Assessment, error) {
	actor, engagementID, err := normalizeMutationActor(actor, engagementID)
	if err != nil {
		return Assessment{}, err
	}
	current, err := s.store.GetEngagement(ctx, actor.TenantID, engagementID)
	if err != nil {
		return Assessment{}, err
	}
	if _, err = authorizeResource(actor, ActionEngageAssess, current.Project.ID, "outsourcing_engagement", current.ID); err != nil {
		return Assessment{}, err
	}
	input, err = normalizeAssess(input)
	if err != nil {
		return Assessment{}, err
	}
	hash, err := requestHash("assess_engagement", engagementID, actor.ID, input)
	if err != nil {
		return Assessment{}, err
	}
	var replay Assessment
	if found, replayErr := s.store.Replay(ctx, actor, input.CommandID, hash, "outsourcing_assessment", &replay); replayErr != nil || found {
		return replay, replayErr
	}
	return s.store.Assess(ctx, actor, engagementID, input, hash)
}

func (s *Service) GetEngagement(ctx context.Context, actor Actor, id string) (Engagement, error) {
	actor, id, err := normalizeMutationActor(actor, id)
	if err != nil {
		return Engagement{}, err
	}
	value, err := s.store.GetEngagement(ctx, actor.TenantID, id)
	if err != nil {
		return Engagement{}, err
	}
	if _, err = authorizeResource(actor, ActionEngageRead, value.Project.ID, "outsourcing_engagement", value.ID); err != nil {
		return Engagement{}, err
	}
	return value, nil
}

func (s *Service) ListEngagements(ctx context.Context, actor Actor, projectID string) ([]Engagement, error) {
	actor, err := authorize(actor, ActionEngageRead, projectID)
	if err != nil {
		return nil, err
	}
	return s.store.ListEngagements(ctx, actor.TenantID, normalizeUUID(projectID))
}

func (s *Service) EngagementHistory(ctx context.Context, actor Actor, id string) ([]EngagementRevision, error) {
	actor, id, err := normalizeMutationActor(actor, id)
	if err != nil {
		return nil, err
	}
	value, err := s.store.GetEngagement(ctx, actor.TenantID, id)
	if err != nil {
		return nil, err
	}
	if _, err = authorizeResource(actor, ActionEngageHistory, value.Project.ID, "outsourcing_engagement", value.ID); err != nil {
		return nil, err
	}
	return s.store.EngagementHistory(ctx, actor.TenantID, id)
}

func (s *Service) AssessmentHistory(ctx context.Context, actor Actor, id string) ([]Assessment, error) {
	actor, id, err := normalizeMutationActor(actor, id)
	if err != nil {
		return nil, err
	}
	value, err := s.store.GetEngagement(ctx, actor.TenantID, id)
	if err != nil {
		return nil, err
	}
	if _, err = authorizeResource(actor, ActionEngageHistory, value.Project.ID, "outsourcing_engagement", value.ID); err != nil {
		return nil, err
	}
	return s.store.AssessmentHistory(ctx, actor.TenantID, id)
}

// ListLatestAssessments is the owner-side list projection used by the original
// OA summary. It avoids turning one page read into one HTTP request per
// engagement while keeping assessment facts inside Cost & Outsourcing.
func (s *Service) ListLatestAssessments(ctx context.Context, actor Actor, projectID string) ([]Assessment, error) {
	actor, err := authorize(actor, ActionEngageHistory, projectID)
	if err != nil {
		return nil, err
	}
	return s.store.ListLatestAssessments(ctx, actor.TenantID, normalizeUUID(projectID))
}

func (s *Service) resolveProject(ctx context.Context, tenantID string, ref ProjectRef) (ProjectSnapshot, error) {
	value, err := s.resolver.ResolveProject(ctx, tenantID, ref)
	if err != nil {
		return ProjectSnapshot{}, err
	}
	if value.TenantID != tenantID || value.ID != ref.ID || (ref.Revision > 0 && value.Revision != ref.Revision) || value.Revision <= 0 || value.Status != "active" || !validText(value.Code, 1, 100) || !validText(value.DisplayName, 1, 200) {
		return ProjectSnapshot{}, ErrReferenceInvalid
	}
	return value.ProjectSnapshot, nil
}

func (s *Service) resolveEngagementRefs(ctx context.Context, tenantID string, project ProjectSnapshot, partyRef PartyRef, contractRef *ContractRef, evidenceRefs []EvidenceRef, currency string) (PartySnapshot, *ContractSnapshot, []EvidenceSnapshot, error) {
	party, err := s.resolver.ResolveParty(ctx, tenantID, partyRef)
	if err != nil {
		return PartySnapshot{}, nil, nil, err
	}
	if party.TenantID != tenantID || party.PartyRef() != partyRef || party.Status != "active" || !validText(party.Kind, 1, 100) || !validText(party.LegalName, 1, 300) {
		return PartySnapshot{}, nil, nil, ErrReferenceInvalid
	}
	var contract *ContractSnapshot
	if contractRef != nil {
		value, resolveErr := s.resolver.ResolveContract(ctx, tenantID, ProjectRef{ID: project.ID, Revision: project.Revision}, partyRef, *contractRef)
		if resolveErr != nil {
			return PartySnapshot{}, nil, nil, resolveErr
		}
		if value.TenantID != tenantID || value.ContractRef != *contractRef || !value.Eligible || value.ProjectID != project.ID || value.PartyID != party.ID || value.Currency != currency || value.AmountMinor < 0 || !validText(value.Digest, 1, 256) || !validText(value.DisplaySummary, 1, 500) {
			return PartySnapshot{}, nil, nil, ErrReferenceInvalid
		}
		copy := value.ContractSnapshot
		contract = &copy
	}
	evidence, err := s.resolveEvidence(ctx, tenantID, evidenceRefs)
	return party.PartySnapshot, contract, evidence, err
}

func (s *Service) resolveEvidence(ctx context.Context, tenantID string, refs []EvidenceRef) ([]EvidenceSnapshot, error) {
	values := make([]EvidenceSnapshot, 0, len(refs))
	for _, ref := range refs {
		value, err := s.resolver.ResolveEvidence(ctx, tenantID, ref)
		if err != nil {
			return nil, err
		}
		if value.TenantID != tenantID || value.EvidenceRef != ref || value.Status != "available" || !validText(value.Digest, 1, 256) || !validText(value.DisplayName, 1, 300) {
			return nil, ErrReferenceInvalid
		}
		values = append(values, value.EvidenceSnapshot)
	}
	return values, nil
}

func (p PartyReference) PartyRef() PartyRef { return PartyRef{ID: p.ID, Revision: p.Revision} }

func authorize(actor Actor, action, projectID string) (Actor, error) {
	value, err := normalizeActor(actor)
	if err != nil {
		return Actor{}, err
	}
	projectID = normalizeUUID(projectID)
	if !validUUID(projectID) || !contains(value.Actions, action) || (!value.TenantScope && !contains(value.ProjectIDs, projectID)) {
		return Actor{}, ErrForbidden
	}
	return value, nil
}

func authorizeResource(actor Actor, action, projectID, resourceType, resourceID string) (Actor, error) {
	value, err := normalizeActor(actor)
	if err != nil {
		return Actor{}, err
	}
	projectID, resourceID = normalizeUUID(projectID), normalizeUUID(resourceID)
	if !validUUID(projectID) || !validUUID(resourceID) || !contains(value.Actions, action) {
		return Actor{}, ErrForbidden
	}
	if value.TenantScope || contains(value.ProjectIDs, projectID) {
		return value, nil
	}
	for _, scope := range value.ResourceScopes {
		if scope.Type == resourceType && scope.ID == resourceID {
			return value, nil
		}
	}
	return Actor{}, ErrForbidden
}

func normalizeMutationActor(actor Actor, id string) (Actor, string, error) {
	value, err := normalizeActor(actor)
	id = normalizeUUID(id)
	if err != nil || !validUUID(id) {
		if err != nil {
			return Actor{}, "", err
		}
		return Actor{}, "", fmt.Errorf("invalid aggregate id")
	}
	return value, id, nil
}

func normalizeRecordCost(input RecordCostInput) (RecordCostInput, error) {
	input.CommandID = normalizeUUID(input.CommandID)
	var err error
	input.ProjectRef, err = normalizeProjectRef(input.ProjectRef)
	if err != nil || !validUUID(input.CommandID) {
		return RecordCostInput{}, fmt.Errorf("invalid cost command")
	}
	input.Category, input.OccurredOn, input.Currency, input.Description, err = normalizeCostFields(input.Category, input.OccurredOn, input.Currency, input.AmountMinor, input.DistanceMeters, input.Description)
	if err != nil {
		return RecordCostInput{}, err
	}
	input.EvidenceRefs, err = normalizeEvidenceRefs(input.EvidenceRefs)
	return input, err
}

func normalizeCorrectCost(input CorrectCostInput) (CorrectCostInput, error) {
	input.CommandID, input.Reason = normalizeUUID(input.CommandID), strings.TrimSpace(input.Reason)
	var err error
	if !validUUID(input.CommandID) || input.ExpectedRevision <= 0 || !validText(input.Reason, 1, 1000) {
		return CorrectCostInput{}, fmt.Errorf("invalid correction command")
	}
	input.Category, input.OccurredOn, input.Currency, input.Description, err = normalizeCostFields(input.Category, input.OccurredOn, input.Currency, input.AmountMinor, input.DistanceMeters, input.Description)
	if err != nil {
		return CorrectCostInput{}, err
	}
	input.EvidenceRefs, err = normalizeEvidenceRefs(input.EvidenceRefs)
	return input, err
}

func normalizeRecordEngagement(input RecordEngagementInput) (RecordEngagementInput, error) {
	input.CommandID = normalizeUUID(input.CommandID)
	var err error
	input.ProjectRef, err = normalizeProjectRef(input.ProjectRef)
	if err != nil || !validUUID(input.CommandID) {
		return RecordEngagementInput{}, fmt.Errorf("invalid engagement command")
	}
	input.ProviderRef, err = normalizePartyRef(input.ProviderRef)
	if err != nil {
		return RecordEngagementInput{}, err
	}
	input.Scope, input.Currency, input.Note, err = normalizeEngagementFields(input.Scope, input.Currency, input.AmountMinor, input.Note)
	if err != nil {
		return RecordEngagementInput{}, err
	}
	input.ContactName, input.ContactPhone, err = normalizeEngagementContact(input.ContactName, input.ContactPhone)
	if err != nil {
		return RecordEngagementInput{}, err
	}
	if input.ContractRef != nil {
		value, normalizeErr := normalizeContractRef(*input.ContractRef)
		if normalizeErr != nil {
			return RecordEngagementInput{}, normalizeErr
		}
		input.ContractRef = &value
	}
	input.EvidenceRefs, err = normalizeEvidenceRefs(input.EvidenceRefs)
	return input, err
}

func normalizeCorrectEngagement(input CorrectEngagementInput) (CorrectEngagementInput, error) {
	input.CommandID, input.Reason = normalizeUUID(input.CommandID), strings.TrimSpace(input.Reason)
	if !validUUID(input.CommandID) || input.ExpectedRevision <= 0 || !validText(input.Reason, 1, 1000) {
		return CorrectEngagementInput{}, fmt.Errorf("invalid correction command")
	}
	record := RecordEngagementInput{CommandID: input.CommandID, ProjectRef: ProjectRef{ID: "00000000-0000-4000-8000-000000000000"}, ProviderRef: input.ProviderRef, ContactName: input.ContactName, ContactPhone: input.ContactPhone, Scope: input.Scope, Currency: input.Currency, AmountMinor: input.AmountMinor, ContractRef: input.ContractRef, EvidenceRefs: input.EvidenceRefs, Note: input.Note}
	value, err := normalizeRecordEngagement(record)
	if err != nil {
		return CorrectEngagementInput{}, err
	}
	input.ProviderRef, input.ContactName, input.ContactPhone, input.Scope, input.Currency, input.ContractRef, input.EvidenceRefs, input.Note = value.ProviderRef, value.ContactName, value.ContactPhone, value.Scope, value.Currency, value.ContractRef, value.EvidenceRefs, value.Note
	return input, nil
}

func normalizeWithdraw(input WithdrawInput) (WithdrawInput, error) {
	input.CommandID, input.Reason = normalizeUUID(input.CommandID), strings.TrimSpace(input.Reason)
	if !validUUID(input.CommandID) || input.ExpectedRevision <= 0 || !validText(input.Reason, 1, 1000) {
		return WithdrawInput{}, fmt.Errorf("invalid withdrawal command")
	}
	return input, nil
}

func normalizeAssess(input AssessInput) (AssessInput, error) {
	input.CommandID, input.Reason = normalizeUUID(input.CommandID), strings.TrimSpace(input.Reason)
	if !validUUID(input.CommandID) || input.ExpectedEngagementRevision <= 0 || input.ExpectedAssessmentRevision < 0 || input.Score < 1 || input.Score > 5 || !validText(input.Reason, 1, 1000) {
		return AssessInput{}, fmt.Errorf("invalid assessment command")
	}
	return input, nil
}

func requestHash(operation, id, actorID string, value any) (string, error) {
	raw, err := json.Marshal(struct {
		Operation string `json:"operation"`
		ID        string `json:"id"`
		ActorID   string `json:"actor_id"`
		Value     any    `json:"value"`
	}{operation, id, actorID, value})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func evidenceRefsChanged(current []EvidenceSnapshot, requested []EvidenceRef) bool {
	if len(current) != len(requested) {
		return true
	}
	for i := range current {
		if current[i].EvidenceRef != requested[i] {
			return true
		}
	}
	return false
}
