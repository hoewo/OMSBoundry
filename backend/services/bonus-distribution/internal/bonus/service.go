package bonus

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

type Store interface {
	Ping(context.Context) error
	Replay(context.Context, Actor, string, string, string, any) (bool, error)
	CreateDraft(context.Context, Actor, BonusPool, string, string) (BonusPool, error)
	GetPool(context.Context, string, string) (BonusPool, error)
	ListPools(context.Context, string, string) ([]BonusPool, error)
	PoolHistory(context.Context, string, string) ([]PoolRevision, error)
	AllocationHistory(context.Context, string, string) ([]AllocationRevision, error)
	Publish(context.Context, Actor, string, PoolCommandInput, string) (BonusPool, error)
	Supersede(context.Context, Actor, SupersedeInput, string) (SupersessionResult, error)
	Withdraw(context.Context, Actor, string, PoolCommandInput, string) (BonusPool, error)
}

type PayableSuggestionStore interface {
	GetAllocation(context.Context, string, string) (BonusAllocation, error)
	CreateSuggestion(context.Context, Actor, BonusPayableSuggestion, string, string) (BonusPayableSuggestion, error)
	GetSuggestion(context.Context, string, string) (BonusPayableSuggestion, error)
	ListSuggestions(context.Context, string, string) ([]BonusPayableSuggestion, error)
	SuggestionHistory(context.Context, string, string) ([]BonusPayableSuggestionRevision, error)
	ApplySuggestionTarget(context.Context, Actor, string, int64, string, string, string, string, BonusPayableSuggestion) (BonusPayableSuggestion, error)
}

type MoneyPayablePort interface {
	SubmitPayableSuggestion(context.Context, string, string, PayableSubmission) (PayableIntake, error)
	GetPayableSuggestion(context.Context, string, string) (PayableIntake, error)
	WithdrawPayableSuggestion(context.Context, string, string, string, int64, string) (PayableIntake, error)
}

type ReferenceResolver interface {
	ResolveProject(context.Context, string, ProjectRef) (ProjectReference, error)
	ResolveBusinessSource(context.Context, string, ProjectSnapshot, BusinessSourceRef) (SourceSnapshot, error)
	ResolveProductionSource(context.Context, string, ProjectSnapshot, ProductionSourceRef) (SourceSnapshot, error)
	ResolveParticipant(context.Context, string, ProjectSnapshot, string, PersonRef, AssignmentRef) (ParticipantReference, error)
}

type PolicyEvaluator interface {
	Evaluate(context.Context, string, int64, string, PolicyRequest) (PolicySnapshot, int64, error)
}

type Service struct {
	store        Store
	payableStore PayableSuggestionStore
	resolver     ReferenceResolver
	evaluator    PolicyEvaluator
	money        MoneyPayablePort
}

func NewService(store Store, resolver ReferenceResolver, evaluator PolicyEvaluator, money ...MoneyPayablePort) (*Service, error) {
	if store == nil || resolver == nil || evaluator == nil {
		return nil, fmt.Errorf("store, reference resolver and policy evaluator are required")
	}
	service := &Service{store: store, resolver: resolver, evaluator: evaluator}
	service.payableStore, _ = store.(PayableSuggestionStore)
	if len(money) > 0 {
		service.money = money[0]
	}
	return service, nil
}

func (s *Service) Ping(ctx context.Context) error { return s.store.Ping(ctx) }

func (s *Service) CreateBusinessDraft(ctx context.Context, actor Actor, input CreateBusinessDraftInput) (BonusPool, error) {
	actor, err := normalizeActor(actor)
	if err != nil {
		return BonusPool{}, err
	}
	input.CommandID = normalizeUUID(input.CommandID)
	input.Project, err = normalizeProjectRef(input.Project)
	if err != nil || !validUUID(input.CommandID) {
		return BonusPool{}, fmt.Errorf("invalid business draft command")
	}
	input.Source, err = normalizeBusinessSourceRef(input.Source)
	if err != nil {
		return BonusPool{}, err
	}
	input.Policy, err = normalizePolicyRequest(input.Policy)
	if err != nil {
		return BonusPool{}, err
	}
	input.Allocations, err = normalizeAllocationRequests(input.Allocations)
	if err != nil {
		return BonusPool{}, err
	}
	if err = authorize(actor, ActionBusinessCalculate, input.Project.ID); err != nil {
		return BonusPool{}, err
	}
	if err = requireAction(actor, ActionAllocationManage); err != nil {
		return BonusPool{}, err
	}
	hash, err := requestHash("create_business_draft", "", actor.ID, input)
	if err != nil {
		return BonusPool{}, err
	}
	var replay BonusPool
	if found, replayErr := s.store.Replay(ctx, actor, input.CommandID, hash, "bonus_pool", &replay); replayErr != nil || found {
		return replay, replayErr
	}
	project, err := s.resolveProject(ctx, actor.TenantID, input.Project)
	if err != nil {
		return BonusPool{}, err
	}
	source, err := s.resolver.ResolveBusinessSource(ctx, actor.TenantID, project, input.Source)
	if err != nil {
		return BonusPool{}, normalizeReferenceError(err)
	}
	source, err = validateSourceSnapshot(source, project, TypeBusiness, input.Source.SettlementID, input.Source.Revision)
	if err != nil {
		return BonusPool{}, err
	}
	return s.createDraft(ctx, actor, input.CommandID, hash, TypeBusiness, project, source, input.Policy, input.Allocations)
}

func (s *Service) CreateProductionDraft(ctx context.Context, actor Actor, input CreateProductionDraftInput) (BonusPool, error) {
	actor, err := normalizeActor(actor)
	if err != nil {
		return BonusPool{}, err
	}
	input.CommandID = normalizeUUID(input.CommandID)
	input.Project, err = normalizeProjectRef(input.Project)
	if err != nil || !validUUID(input.CommandID) {
		return BonusPool{}, fmt.Errorf("invalid production draft command")
	}
	input.Source, err = normalizeProductionSourceRef(input.Source)
	if err != nil {
		return BonusPool{}, err
	}
	input.Policy, err = normalizePolicyRequest(input.Policy)
	if err != nil {
		return BonusPool{}, err
	}
	input.Allocations, err = normalizeAllocationRequests(input.Allocations)
	if err != nil {
		return BonusPool{}, err
	}
	if err = authorize(actor, ActionProductionCalculate, input.Project.ID); err != nil {
		return BonusPool{}, err
	}
	if err = requireAction(actor, ActionAllocationManage); err != nil {
		return BonusPool{}, err
	}
	hash, err := requestHash("create_production_draft", "", actor.ID, input)
	if err != nil {
		return BonusPool{}, err
	}
	var replay BonusPool
	if found, replayErr := s.store.Replay(ctx, actor, input.CommandID, hash, "bonus_pool", &replay); replayErr != nil || found {
		return replay, replayErr
	}
	project, err := s.resolveProject(ctx, actor.TenantID, input.Project)
	if err != nil {
		return BonusPool{}, err
	}
	source, err := s.resolver.ResolveProductionSource(ctx, actor.TenantID, project, input.Source)
	if err != nil {
		return BonusPool{}, normalizeReferenceError(err)
	}
	source, err = validateSourceSnapshot(source, project, TypeProduction, input.Source.ReceiptSetID, input.Source.Revision)
	if err != nil {
		return BonusPool{}, err
	}
	return s.createDraft(ctx, actor, input.CommandID, hash, TypeProduction, project, source, input.Policy, input.Allocations)
}

func (s *Service) createDraft(ctx context.Context, actor Actor, commandID, hash, bonusType string, project ProjectSnapshot, source SourceSnapshot, policyRequest PolicyRequest, requests []AllocationRequest) (BonusPool, error) {
	policy, amountMinor, err := s.evaluator.Evaluate(ctx, bonusType, source.TotalAmountMinor, source.Currency, policyRequest)
	if err != nil {
		return BonusPool{}, normalizePolicyError(err)
	}
	policy, err = validatePolicySnapshot(policy, bonusType, policyRequest, amountMinor, source.TotalAmountMinor)
	if err != nil {
		return BonusPool{}, err
	}
	poolID, err := newUUID()
	if err != nil {
		return BonusPool{}, err
	}
	allocations := make([]BonusAllocation, 0, len(requests))
	var allocated int64
	for _, request := range requests {
		reference, resolveErr := s.resolver.ResolveParticipant(ctx, actor.TenantID, project, bonusType, request.Person, request.Assignment)
		if resolveErr != nil {
			return BonusPool{}, normalizeReferenceError(resolveErr)
		}
		person, assignment, validateErr := validateParticipant(reference, actor.TenantID, bonusType, project, request)
		if validateErr != nil {
			return BonusPool{}, validateErr
		}
		allocationAmount, amountErr := exactRatioAmount(amountMinor, request.RatioBasisPoints)
		if amountErr != nil {
			return BonusPool{}, amountErr
		}
		var addOK bool
		allocated, addOK = checkedAdd(allocated, allocationAmount)
		if !addOK || allocated > amountMinor {
			return BonusPool{}, ErrAllocationInvalid
		}
		allocationID, idErr := newUUID()
		if idErr != nil {
			return BonusPool{}, idErr
		}
		allocations = append(allocations, BonusAllocation{ID: allocationID, TenantID: actor.TenantID, PoolID: poolID, PoolRevision: 1, ProjectID: project.ID, BonusType: bonusType, Person: person, Assignment: assignment, RatioBasisPoints: request.RatioBasisPoints, AmountMinor: allocationAmount, Currency: source.Currency, Reason: request.Reason, State: StateDraft, Revision: 1})
	}
	pool := BonusPool{ID: poolID, TenantID: actor.TenantID, Project: project, BonusType: bonusType, State: StateDraft, Source: source, Policy: policy, Currency: source.Currency, AmountMinor: amountMinor, UnallocatedMinor: amountMinor - allocated, Allocations: allocations, Revision: 1, CreatedBy: actor.ID, UpdatedBy: actor.ID}
	return s.store.CreateDraft(ctx, actor, pool, commandID, hash)
}

func (s *Service) Publish(ctx context.Context, actor Actor, poolID string, input PoolCommandInput) (BonusPool, error) {
	actor, poolID, input, current, hash, err := s.preparePoolMutation(ctx, actor, poolID, input, "publish")
	if err != nil {
		return BonusPool{}, err
	}
	if err = authorizePool(actor, actionFor(current.BonusType, "publish"), current.ID, current.Project.ID); err != nil {
		return BonusPool{}, err
	}
	var replay BonusPool
	if found, replayErr := s.store.Replay(ctx, actor, input.CommandID, hash, "bonus_pool", &replay); replayErr != nil || found {
		return replay, replayErr
	}
	if current.State != StateDraft {
		return BonusPool{}, ErrStateConflict
	}
	if err = s.revalidatePool(ctx, actor.TenantID, current); err != nil {
		return BonusPool{}, err
	}
	return s.store.Publish(ctx, actor, poolID, input, hash)
}

func (s *Service) Supersede(ctx context.Context, actor Actor, input SupersedeInput) (SupersessionResult, error) {
	actor, err := normalizeActor(actor)
	if err != nil {
		return SupersessionResult{}, err
	}
	input, err = normalizeSupersede(input)
	if err != nil {
		return SupersessionResult{}, err
	}
	published, err := s.store.GetPool(ctx, actor.TenantID, input.PublishedPoolID)
	if err != nil {
		return SupersessionResult{}, err
	}
	replacement, err := s.store.GetPool(ctx, actor.TenantID, input.ReplacementPoolID)
	if err != nil {
		return SupersessionResult{}, err
	}
	if err = authorizePool(actor, actionFor(published.BonusType, "supersede"), published.ID, published.Project.ID); err != nil {
		return SupersessionResult{}, err
	}
	if !hasResourceAccess(actor, "bonus_pool", replacement.ID, replacement.Project.ID) {
		return SupersessionResult{}, ErrForbidden
	}
	hash, err := requestHash("supersede", input.PublishedPoolID+":"+input.ReplacementPoolID, actor.ID, input)
	if err != nil {
		return SupersessionResult{}, err
	}
	var replay SupersessionResult
	if found, replayErr := s.store.Replay(ctx, actor, input.CommandID, hash, "bonus_supersession", &replay); replayErr != nil || found {
		return replay, replayErr
	}
	if published.State != StatePublished || replacement.State != StateDraft || published.Project.ID != replacement.Project.ID || published.BonusType != replacement.BonusType {
		return SupersessionResult{}, ErrStateConflict
	}
	if err = s.revalidatePool(ctx, actor.TenantID, replacement); err != nil {
		return SupersessionResult{}, err
	}
	return s.store.Supersede(ctx, actor, input, hash)
}

func (s *Service) Withdraw(ctx context.Context, actor Actor, poolID string, input PoolCommandInput) (BonusPool, error) {
	actor, poolID, input, current, hash, err := s.preparePoolMutation(ctx, actor, poolID, input, "withdraw")
	if err != nil {
		return BonusPool{}, err
	}
	if err = authorizePool(actor, actionFor(current.BonusType, "withdraw"), current.ID, current.Project.ID); err != nil {
		return BonusPool{}, err
	}
	var replay BonusPool
	if found, replayErr := s.store.Replay(ctx, actor, input.CommandID, hash, "bonus_pool", &replay); replayErr != nil || found {
		return replay, replayErr
	}
	if current.State != StateDraft && current.State != StatePublished {
		return BonusPool{}, ErrStateConflict
	}
	return s.store.Withdraw(ctx, actor, poolID, input, hash)
}

func (s *Service) GetPool(ctx context.Context, actor Actor, poolID string) (BonusPool, error) {
	actor, err := normalizeActor(actor)
	if err != nil {
		return BonusPool{}, err
	}
	poolID = normalizeUUID(poolID)
	if !validUUID(poolID) {
		return BonusPool{}, ErrNotFound
	}
	value, err := s.store.GetPool(ctx, actor.TenantID, poolID)
	if err != nil {
		return BonusPool{}, err
	}
	if err = authorizePool(actor, ActionPoolRead, value.ID, value.Project.ID); err != nil {
		return BonusPool{}, err
	}
	return value, nil
}

func (s *Service) ListPools(ctx context.Context, actor Actor, projectID string) ([]BonusPool, error) {
	actor, err := normalizeActor(actor)
	if err != nil {
		return nil, err
	}
	projectID = normalizeUUID(projectID)
	if err = authorize(actor, ActionPoolRead, projectID); err != nil {
		return nil, err
	}
	return s.store.ListPools(ctx, actor.TenantID, projectID)
}

func (s *Service) PoolHistory(ctx context.Context, actor Actor, poolID string) ([]PoolRevision, error) {
	actor, err := normalizeActor(actor)
	if err != nil {
		return nil, err
	}
	poolID = normalizeUUID(poolID)
	value, err := s.store.GetPool(ctx, actor.TenantID, poolID)
	if err != nil {
		return nil, err
	}
	if err = authorizePool(actor, ActionPoolHistory, value.ID, value.Project.ID); err != nil {
		return nil, err
	}
	return s.store.PoolHistory(ctx, actor.TenantID, poolID)
}

func (s *Service) AllocationHistory(ctx context.Context, actor Actor, allocationID string) ([]AllocationRevision, error) {
	actor, err := normalizeActor(actor)
	if err != nil {
		return nil, err
	}
	allocationID = normalizeUUID(allocationID)
	if !validUUID(allocationID) {
		return nil, ErrNotFound
	}
	history, err := s.store.AllocationHistory(ctx, actor.TenantID, allocationID)
	if err != nil {
		return nil, err
	}
	if len(history) == 0 {
		return nil, ErrNotFound
	}
	if !contains(actor.Actions, ActionAllocationRead) || !hasAllocationAccess(actor, allocationID, history[0].Snapshot.PoolID, history[0].Snapshot.ProjectID) {
		return nil, ErrForbidden
	}
	return history, nil
}

func (s *Service) preparePoolMutation(ctx context.Context, actor Actor, poolID string, input PoolCommandInput, operation string) (Actor, string, PoolCommandInput, BonusPool, string, error) {
	actor, err := normalizeActor(actor)
	if err != nil {
		return Actor{}, "", PoolCommandInput{}, BonusPool{}, "", err
	}
	poolID = normalizeUUID(poolID)
	if !validUUID(poolID) {
		return Actor{}, "", PoolCommandInput{}, BonusPool{}, "", ErrNotFound
	}
	input, err = normalizePoolCommand(input)
	if err != nil {
		return Actor{}, "", PoolCommandInput{}, BonusPool{}, "", err
	}
	current, err := s.store.GetPool(ctx, actor.TenantID, poolID)
	if err != nil {
		return Actor{}, "", PoolCommandInput{}, BonusPool{}, "", err
	}
	hash, err := requestHash(operation, poolID, actor.ID, input)
	return actor, poolID, input, current, hash, err
}

func (s *Service) revalidatePool(ctx context.Context, tenantID string, current BonusPool) error {
	project, err := s.resolveProject(ctx, tenantID, ProjectRef{ID: current.Project.ID, Revision: current.Project.Revision})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(project, current.Project) {
		return ErrReferenceInvalid
	}
	var source SourceSnapshot
	if current.BonusType == TypeBusiness {
		source, err = s.resolver.ResolveBusinessSource(ctx, tenantID, project, BusinessSourceRef{SettlementID: current.Source.ID, Revision: current.Source.Revision})
	} else {
		source, err = s.resolver.ResolveProductionSource(ctx, tenantID, project, ProductionSourceRef{ReceiptSetID: current.Source.ID, Revision: current.Source.Revision})
	}
	if err != nil {
		return normalizeReferenceError(err)
	}
	source, err = validateSourceSnapshot(source, project, current.BonusType, current.Source.ID, current.Source.Revision)
	if err != nil || !reflect.DeepEqual(source, current.Source) {
		return ErrReferenceInvalid
	}
	policyRequest := PolicyRequest{Key: current.Policy.Key, Version: current.Policy.Version, RateBasisPoints: current.Policy.RateBasisPoints, Rounding: current.Policy.Rounding}
	policy, amount, err := s.evaluator.Evaluate(ctx, current.BonusType, source.TotalAmountMinor, source.Currency, policyRequest)
	if err != nil {
		return normalizePolicyError(err)
	}
	policy, err = validatePolicySnapshot(policy, current.BonusType, policyRequest, amount, source.TotalAmountMinor)
	if err != nil || amount != current.AmountMinor || !reflect.DeepEqual(policy, current.Policy) {
		return ErrPolicyInvalid
	}
	var allocated int64
	for _, allocation := range current.Allocations {
		request := AllocationRequest{Person: PersonRef{ID: allocation.Person.ID, Revision: allocation.Person.Revision}, Assignment: AssignmentRef{ID: allocation.Assignment.ID, Revision: allocation.Assignment.Revision}, RatioBasisPoints: allocation.RatioBasisPoints, Reason: allocation.Reason}
		reference, resolveErr := s.resolver.ResolveParticipant(ctx, tenantID, project, current.BonusType, request.Person, request.Assignment)
		if resolveErr != nil {
			return normalizeReferenceError(resolveErr)
		}
		person, assignment, validateErr := validateParticipant(reference, tenantID, current.BonusType, project, request)
		if validateErr != nil || !reflect.DeepEqual(person, allocation.Person) || !reflect.DeepEqual(assignment, allocation.Assignment) {
			return ErrReferenceInvalid
		}
		expected, amountErr := exactRatioAmount(current.AmountMinor, allocation.RatioBasisPoints)
		if amountErr != nil || expected != allocation.AmountMinor {
			return ErrAllocationInvalid
		}
		var addOK bool
		allocated, addOK = checkedAdd(allocated, expected)
		if !addOK {
			return ErrAllocationInvalid
		}
	}
	if allocated > current.AmountMinor || current.UnallocatedMinor != current.AmountMinor-allocated {
		return ErrAllocationInvalid
	}
	return nil
}

func (s *Service) resolveProject(ctx context.Context, tenantID string, ref ProjectRef) (ProjectSnapshot, error) {
	value, err := s.resolver.ResolveProject(ctx, tenantID, ref)
	if err != nil {
		return ProjectSnapshot{}, normalizeReferenceError(err)
	}
	return validateProjectReference(value, tenantID, ref)
}

func authorize(actor Actor, action, projectID string) error {
	projectID = normalizeUUID(projectID)
	if !contains(actor.Actions, action) || (!actor.TenantScope && !contains(actor.ProjectIDs, projectID)) {
		return ErrForbidden
	}
	return nil
}

func authorizePool(actor Actor, action, poolID, projectID string) error {
	if !contains(actor.Actions, action) || !hasResourceAccess(actor, "bonus_pool", poolID, projectID) {
		return ErrForbidden
	}
	return nil
}

func hasResourceAccess(actor Actor, resourceType, resourceID, projectID string) bool {
	if actor.TenantScope || contains(actor.ProjectIDs, normalizeUUID(projectID)) {
		return true
	}
	for _, scope := range actor.ResourceScopes {
		if scope.Type == resourceType && scope.ID == normalizeUUID(resourceID) {
			return true
		}
	}
	return false
}

func hasAllocationAccess(actor Actor, allocationID, poolID, projectID string) bool {
	return hasResourceAccess(actor, "bonus_allocation", allocationID, projectID) || hasResourceAccess(actor, "bonus_pool", poolID, projectID)
}

func requireAction(actor Actor, action string) error {
	if !contains(actor.Actions, action) {
		return ErrForbidden
	}
	return nil
}

func actionFor(bonusType, operation string) string {
	prefix := "bonus-pool.business."
	if bonusType == TypeProduction {
		prefix = "bonus-pool.production."
	}
	return prefix + operation
}

func requestHash(operation, aggregateID, actorID string, value any) (string, error) {
	payload, err := json.Marshal(struct {
		Operation   string `json:"operation"`
		AggregateID string `json:"aggregate_id"`
		ActorID     string `json:"actor_id"`
		Value       any    `json:"value"`
	}{operation, aggregateID, actorID, value})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func normalizeReferenceError(err error) error {
	if errors.Is(err, ErrReferenceUnavailable) {
		return ErrReferenceUnavailable
	}
	return ErrReferenceInvalid
}

func normalizePolicyError(err error) error {
	if errors.Is(err, ErrReferenceUnavailable) {
		return ErrReferenceUnavailable
	}
	return ErrPolicyInvalid
}
