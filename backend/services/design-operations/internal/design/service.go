package design

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type Store interface {
	Ping(context.Context) error
	List(context.Context, string, int) ([]WorkItem, error)
	Replay(context.Context, string, string, string, any) (bool, error)
	Create(context.Context, Actor, CreateInput, string) (Details, error)
	SetOwner(context.Context, Actor, string, OwnerInput, string, string) (Details, error)
	PutDeliverable(context.Context, Actor, string, string, DeliverableInput, string) (Deliverable, error)
	SetApplicability(context.Context, Actor, string, string, ApplicabilityInput, string) (Details, error)
	CompleteStage(context.Context, Actor, string, string, RevisionInput, string) (Details, error)
	ReopenStage(context.Context, Actor, string, string, ReasonInput, string) (Details, error)
	Cancel(context.Context, Actor, string, ReasonInput, string) (Details, error)
	Get(context.Context, string, string, bool) (Details, error)
	History(context.Context, string, string, int64, int) (HistoryPage, error)
}

type References interface {
	ValidateProject(context.Context, string, string, int64) (ProjectReference, error)
	ValidateAssignment(context.Context, string, string, string, int64) (AssignmentReference, error)
	ValidatePersonIAMSubject(context.Context, string, string, int64) (PersonIAMSubjectFact, error)
	ValidateEvidence(context.Context, string, string, int, int64) (EvidenceReference, error)
}

type Service struct {
	store                Store
	refs                 References
	stageMaterialProfile string
}

func NewService(store Store, refs References) (*Service, error) {
	if store == nil || refs == nil {
		return nil, fmt.Errorf("store and references are required")
	}
	return &Service{store: store, refs: refs}, nil
}

func NewServiceWithStageMaterials(store Store, refs References, profile string) (*Service, error) {
	s, err := NewService(store, refs)
	if err != nil {
		return nil, err
	}
	if profile != "" && profile != StageMaterialProfile {
		return nil, ErrTypeInvalid
	}
	s.stageMaterialProfile = profile
	return s, nil
}
func (s *Service) Ping(ctx context.Context) error { return s.store.Ping(ctx) }

func (s *Service) List(ctx context.Context, tenantID string, limit int) ([]WorkItem, error) {
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("limit must be between 1 and 100")
	}
	return s.store.List(ctx, tenantID, limit)
}

func (s *Service) WorkItemsForSubject(ctx context.Context, tenantID, subjectID string, limit int) ([]WorkItemCandidate, error) {
	if !ValidUUID(tenantID) || !ValidUUID(subjectID) {
		return nil, fmt.Errorf("invalid work item subject")
	}
	items, err := s.List(ctx, tenantID, limit)
	if err != nil {
		return nil, err
	}
	candidates := make([]WorkItemCandidate, 0, len(items))
	for _, item := range items {
		if item.Status == StatusCompleted || item.Status == StatusCancelled || item.OwnerAssignmentID == nil || item.OwnerSourceRevision == nil {
			continue
		}
		owner, err := s.refs.ValidateAssignment(ctx, tenantID, item.ProjectID, *item.OwnerAssignmentID, *item.OwnerSourceRevision)
		if errors.Is(err, ErrReferenceInvalid) {
			// An ended or revision-changed Assignment is no longer a current
			// responsibility and must not create a new executable work item.
			continue
		}
		if err != nil {
			return nil, err
		}
		if owner.TenantID != tenantID || owner.ProjectID != item.ProjectID || owner.Status != "active" || owner.RoleKey != "production_owner" || owner.SubjectKind != "person" {
			continue
		}
		person, err := s.refs.ValidatePersonIAMSubject(ctx, tenantID, owner.SubjectID, owner.SourceRevision)
		if errors.Is(err, ErrReferenceInvalid) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if person.IAMSubjectID != subjectID {
			continue
		}
		candidates = append(candidates, WorkItemCandidate{WorkItem: item, Owner: owner})
	}
	return candidates, nil
}

func (s *Service) Create(ctx context.Context, actor Actor, input CreateInput) (Details, error) {
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	input.ProjectID = strings.ToLower(strings.TrimSpace(input.ProjectID))
	input.TypeKey = strings.ToLower(strings.TrimSpace(input.TypeKey))
	if input.TypeKey == "" {
		input.TypeKey = DesignTypeKey
	}
	if !ValidUUID(input.CommandID) || !ValidUUID(input.ProjectID) || input.ProjectSourceRevision < 1 {
		return Details{}, fmt.Errorf("invalid create input")
	}
	definition, ok := createDefinition(input)
	if !ok {
		return Details{}, ErrTypeInvalid
	}
	if IsStageMaterialDefinitionVersion(definition.Version) && s.stageMaterialProfile != StageMaterialProfile {
		return Details{}, ErrTypeInvalid
	}
	hash, _ := requestHash(struct {
		Kind  string
		Input CreateInput
	}{"create", input})
	var replay Details
	if found, err := s.store.Replay(ctx, actor.TenantID, input.CommandID, hash, &replay); err != nil || found {
		return replay, err
	}
	project, err := s.refs.ValidateProject(ctx, actor.TenantID, input.ProjectID, input.ProjectSourceRevision)
	if err != nil {
		return Details{}, err
	}
	typeMatches := hasProjectType(project.TypeKeys, definition.RequiredProjectTypeKey)
	if definition.Key == StageMaterialTypeKey {
		if project.ID != input.ProjectID || project.TenantID != actor.TenantID || project.Revision != input.ProjectSourceRevision {
			return Details{}, ErrReferenceInvalid
		}
		typeMatches = hasProjectType(project.TypeKeys, "design") || hasProjectType(project.TypeKeys, "consultation") || hasProjectType(project.TypeKeys, "survey") || hasProjectType(project.TypeKeys, "mapping")
		if definition.Version == StageMaterialDefinitionVersion {
			input.ProjectTypeSnapshot, err = normalizeStageMaterialProjectTypes(project.TypeKeys)
			if err != nil {
				return Details{}, err
			}
		}
	}
	if project.Status != "active" || !typeMatches {
		return Details{}, ErrReferenceInvalid
	}
	return s.store.Create(ctx, actor, input, hash)
}

func (s *Service) SetOwner(ctx context.Context, actor Actor, id string, input OwnerInput) (Details, error) {
	var err error
	input.CommandID, err = normalizeCommand(input.CommandID, input.ExpectedRevision)
	input.AssignmentID = strings.ToLower(strings.TrimSpace(input.AssignmentID))
	if err != nil || !ValidUUID(id) || !ValidUUID(input.AssignmentID) || input.AssignmentSourceRevision < 1 {
		return Details{}, fmt.Errorf("invalid owner input")
	}
	hash, _ := requestHash(struct {
		Kind, ID string
		Input    OwnerInput
	}{"owner", id, input})
	var replay Details
	if found, err := s.store.Replay(ctx, actor.TenantID, input.CommandID, hash, &replay); err != nil || found {
		return replay, err
	}
	owned, err := s.store.Get(ctx, actor.TenantID, id, false)
	if err != nil {
		return Details{}, err
	}
	ref, err := s.refs.ValidateAssignment(ctx, actor.TenantID, owned.WorkItem.ProjectID, input.AssignmentID, input.AssignmentSourceRevision)
	if err != nil {
		return Details{}, err
	}
	if ref.Status != "active" || ref.RoleKey != "production_owner" || ref.ProjectID != owned.WorkItem.ProjectID {
		return Details{}, ErrReferenceInvalid
	}
	return s.store.SetOwner(ctx, actor, id, input, ref.DisplayName, hash)
}

func (s *Service) PutDeliverable(ctx context.Context, actor Actor, id, stage string, input DeliverableInput) (Deliverable, error) {
	var err error
	input.CommandID, err = normalizeCommand(input.CommandID, input.ExpectedRevision)
	input.RequirementKey = strings.ToLower(strings.TrimSpace(input.RequirementKey))
	input.EvidenceFileID = strings.ToLower(strings.TrimSpace(input.EvidenceFileID))
	stage = strings.ToLower(strings.TrimSpace(stage))
	if err != nil || !ValidUUID(id) || !ValidUUID(input.EvidenceFileID) || input.RequirementKey != RequirementPrimary || input.EvidenceVersion < 1 || input.EvidenceSourceRevision < 1 {
		return Deliverable{}, fmt.Errorf("invalid deliverable input")
	}
	if !IsKnownStageKey(stage) {
		return Deliverable{}, ErrStageInvalid
	}
	hash, _ := requestHash(struct {
		Kind, ID, Stage string
		Input           DeliverableInput
	}{"deliverable", id, stage, input})
	var replay Deliverable
	if found, err := s.store.Replay(ctx, actor.TenantID, input.CommandID, hash, &replay); err != nil || found {
		return replay, err
	}
	owned, definition, err := s.ownedStage(ctx, actor.TenantID, id, stage)
	if err != nil {
		return Deliverable{}, err
	}
	if owned.WorkItem.TypeDefinitionVersion != typeDefinitionVersionV1 {
		return Deliverable{}, ErrTypeInvalid
	}
	if owned.WorkItem.Revision != input.ExpectedRevision || !requiresRequirement(definition, input.RequirementKey) {
		if owned.WorkItem.Revision != input.ExpectedRevision {
			return Deliverable{}, ErrRevisionConflict
		}
		return Deliverable{}, ErrStageInvalid
	}
	ref, err := s.refs.ValidateEvidence(ctx, actor.TenantID, input.EvidenceFileID, input.EvidenceVersion, input.EvidenceSourceRevision)
	if err != nil {
		return Deliverable{}, err
	}
	if ref.Status != "available" {
		return Deliverable{}, ErrReferenceInvalid
	}
	input.DisplaySnapshot = ref.DisplayName
	return s.store.PutDeliverable(ctx, actor, id, stage, input, hash)
}

func (s *Service) SetApplicability(ctx context.Context, actor Actor, id, stage string, input ApplicabilityInput) (Details, error) {
	var err error
	input.CommandID, err = normalizeCommand(input.CommandID, input.ExpectedRevision)
	input.Operation = strings.ToLower(strings.TrimSpace(input.Operation))
	input.Reason = strings.TrimSpace(input.Reason)
	stage = strings.ToLower(strings.TrimSpace(stage))
	if err != nil || !ValidUUID(id) || !IsKnownStageKey(stage) || input.Reason == "" || len([]rune(input.Reason)) > 1000 || (input.Operation != "set-not-applicable" && input.Operation != "restore-applicability") {
		return Details{}, fmt.Errorf("invalid applicability input")
	}
	hash, _ := requestHash(struct {
		Kind, ID, Stage string
		Input           ApplicabilityInput
	}{"applicability", id, stage, input})
	var replay Details
	if found, err := s.store.Replay(ctx, actor.TenantID, input.CommandID, hash, &replay); err != nil || found {
		return replay, err
	}
	owned, definition, err := s.ownedStage(ctx, actor.TenantID, id, stage)
	if err != nil {
		return Details{}, err
	}
	if owned.WorkItem.TypeDefinitionVersion != typeDefinitionVersionV1 {
		return Details{}, ErrTypeInvalid
	}
	if owned.WorkItem.Revision != input.ExpectedRevision {
		return Details{}, ErrRevisionConflict
	}
	if !definition.Skippable {
		return Details{}, ErrStageInvalid
	}
	return s.store.SetApplicability(ctx, actor, id, stage, input, hash)
}

func (s *Service) CompleteStage(ctx context.Context, actor Actor, id, stage string, input RevisionInput) (Details, error) {
	var err error
	input.CommandID, err = normalizeCommand(input.CommandID, input.ExpectedRevision)
	stage = strings.ToLower(strings.TrimSpace(stage))
	if err != nil || !ValidUUID(id) {
		return Details{}, fmt.Errorf("invalid completion input")
	}
	if !IsKnownStageKey(stage) {
		return Details{}, ErrStageInvalid
	}
	hash, _ := requestHash(struct {
		Kind, ID, Stage string
		Input           RevisionInput
	}{"complete", id, stage, input})
	var replay Details
	if found, err := s.store.Replay(ctx, actor.TenantID, input.CommandID, hash, &replay); err != nil || found {
		return replay, err
	}
	owned, err := s.store.Get(ctx, actor.TenantID, id, false)
	if err != nil {
		return Details{}, err
	}
	if owned.WorkItem.TypeDefinitionVersion != typeDefinitionVersionV1 {
		return Details{}, ErrTypeInvalid
	}
	if owned.WorkItem.Revision != input.ExpectedRevision {
		return Details{}, ErrRevisionConflict
	}
	definition, ok := StageDefinitionFor(owned.WorkItem.TypeKey, owned.WorkItem.TypeDefinitionVersion, stage)
	if !ok {
		return Details{}, ErrStageInvalid
	}
	stageReady := false
	for _, candidate := range owned.Stages {
		if candidate.Key == stage {
			stageReady = candidate.Status == StageReady
			break
		}
	}
	if !stageReady {
		return Details{}, ErrStageInvalid
	}
	if owned.WorkItem.OwnerAssignmentID == nil || owned.WorkItem.OwnerSourceRevision == nil {
		return Details{}, ErrCompletionGaps
	}
	owner, err := s.refs.ValidateAssignment(ctx, actor.TenantID, owned.WorkItem.ProjectID, *owned.WorkItem.OwnerAssignmentID, *owned.WorkItem.OwnerSourceRevision)
	if err != nil {
		return Details{}, err
	}
	if owner.Status != "active" || owner.RoleKey != "production_owner" || owner.ProjectID != owned.WorkItem.ProjectID {
		return Details{}, ErrReferenceInvalid
	}
	activeDeliverables := map[string]*Deliverable{}
	for i := range owned.Deliverables {
		candidate := &owned.Deliverables[i]
		if candidate.StageKey == stage && candidate.SupersededAt == nil {
			activeDeliverables[candidate.RequirementKey] = candidate
		}
	}
	for _, requirementKey := range definition.RequiredRequirementKeys {
		deliverable := activeDeliverables[requirementKey]
		if deliverable == nil {
			return Details{}, ErrCompletionGaps
		}
		evidence, err := s.refs.ValidateEvidence(ctx, actor.TenantID, deliverable.EvidenceFileID, deliverable.EvidenceVersion, deliverable.EvidenceSourceRevision)
		if err != nil {
			return Details{}, err
		}
		if evidence.Status != "available" {
			return Details{}, ErrReferenceInvalid
		}
	}
	return s.store.CompleteStage(ctx, actor, id, stage, input, hash)
}

func (s *Service) ReopenStage(ctx context.Context, actor Actor, id, stage string, input ReasonInput) (Details, error) {
	return s.reasonCommand(ctx, actor, id, stage, "reopen", input, s.store.ReopenStage)
}
func (s *Service) Cancel(ctx context.Context, actor Actor, id string, input ReasonInput) (Details, error) {
	return s.reasonCommand(ctx, actor, id, "", "cancel", input, func(ctx context.Context, a Actor, id, _ string, in ReasonInput, h string) (Details, error) {
		return s.store.Cancel(ctx, a, id, in, h)
	})
}
func (s *Service) reasonCommand(ctx context.Context, actor Actor, id, stage, kind string, input ReasonInput, apply func(context.Context, Actor, string, string, ReasonInput, string) (Details, error)) (Details, error) {
	var err error
	input.CommandID, err = normalizeCommand(input.CommandID, input.ExpectedRevision)
	input.Reason = strings.TrimSpace(input.Reason)
	stage = strings.ToLower(strings.TrimSpace(stage))
	if err != nil || !ValidUUID(id) || input.Reason == "" || len([]rune(input.Reason)) > 1000 {
		return Details{}, fmt.Errorf("invalid reason command")
	}
	if stage != "" {
		if !IsKnownStageKey(stage) {
			return Details{}, ErrStageInvalid
		}
	}
	hash, _ := requestHash(struct {
		Kind, ID, Stage string
		Input           ReasonInput
	}{kind, id, stage, input})
	var replay Details
	if found, err := s.store.Replay(ctx, actor.TenantID, input.CommandID, hash, &replay); err != nil || found {
		return replay, err
	}
	if stage != "" {
		owned, _, err := s.ownedStage(ctx, actor.TenantID, id, stage)
		if err != nil {
			return Details{}, err
		}
		if owned.WorkItem.TypeDefinitionVersion != typeDefinitionVersionV1 {
			return Details{}, ErrTypeInvalid
		}
		if owned.WorkItem.Revision != input.ExpectedRevision {
			return Details{}, ErrRevisionConflict
		}
	}
	return apply(ctx, actor, id, stage, input, hash)
}

func (s *Service) ownedStage(ctx context.Context, tenant, id, stage string) (Details, StageDefinition, error) {
	owned, err := s.store.Get(ctx, tenant, id, false)
	if err != nil {
		return Details{}, StageDefinition{}, err
	}
	definition, ok := StageDefinitionFor(owned.WorkItem.TypeKey, owned.WorkItem.TypeDefinitionVersion, stage)
	if !ok {
		return Details{}, StageDefinition{}, ErrStageInvalid
	}
	return owned, definition, nil
}

func requiresRequirement(stage StageDefinition, requirementKey string) bool {
	for _, candidate := range stage.RequiredRequirementKeys {
		if candidate == requirementKey {
			return true
		}
	}
	return false
}

func (s *Service) GetOwned(ctx context.Context, tenant, id string) (Details, error) {
	if !ValidUUID(id) {
		return Details{}, fmt.Errorf("invalid work item id")
	}
	return s.store.Get(ctx, tenant, id, false)
}
func (s *Service) Get(ctx context.Context, tenant, id string) (Details, error) {
	d, err := s.GetOwned(ctx, tenant, id)
	if err != nil {
		return Details{}, err
	}
	d.ReferenceStatuses = []ReferenceStatus{}
	projectStatus := "available"
	if _, err := s.refs.ValidateProject(ctx, tenant, d.WorkItem.ProjectID, d.WorkItem.ProjectSourceRevision); err != nil {
		projectStatus = referenceStatus(err)
	}
	d.ReferenceStatuses = append(d.ReferenceStatuses, ReferenceStatus{"project", d.WorkItem.ProjectID, projectStatus})
	for _, v := range d.Deliverables {
		if v.SupersededAt != nil {
			continue
		}
		status := "available"
		if _, err := s.refs.ValidateEvidence(ctx, tenant, v.EvidenceFileID, v.EvidenceVersion, v.EvidenceSourceRevision); err != nil {
			status = referenceStatus(err)
		}
		d.ReferenceStatuses = append(d.ReferenceStatuses, ReferenceStatus{"evidence", v.EvidenceFileID, status})
	}
	return d, nil
}
func (s *Service) History(ctx context.Context, tenant, id string, after int64, limit int) (HistoryPage, error) {
	if !ValidUUID(id) || after < 0 {
		return HistoryPage{}, fmt.Errorf("invalid history query")
	}
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 200 {
		return HistoryPage{}, fmt.Errorf("limit must be between 1 and 200")
	}
	return s.store.History(ctx, tenant, id, after, limit)
}
func (s *Service) Reference(ctx context.Context, tenant, id string) (ReferenceView, error) {
	d, err := s.GetOwned(ctx, tenant, id)
	if err != nil {
		return ReferenceView{}, err
	}
	if IsStageMaterialDefinitionVersion(d.WorkItem.TypeDefinitionVersion) {
		return ReferenceView{}, ErrTypeInvalid
	}
	return referenceView(tenant, id, d, true), nil
}
func (s *Service) ProductionReference(ctx context.Context, tenant, id string) (ReferenceView, error) {
	d, err := s.GetOwned(ctx, tenant, id)
	if err != nil {
		return ReferenceView{}, err
	}
	return referenceView(tenant, id, d, false), nil
}
func referenceView(tenant, id string, d Details, legacy bool) ReferenceView {
	gaps := []string{}
	if d.WorkItem.OwnerAssignmentID == nil && !IsStageMaterialDefinitionVersion(d.WorkItem.TypeDefinitionVersion) {
		gaps = append(gaps, "production_owner")
	}
	for _, st := range d.Stages {
		if st.Status != StageCompleted && st.Status != StageNotApplicable {
			gaps = append(gaps, st.Key)
		}
	}
	typeKey := d.WorkItem.TypeKey
	resourceType := "production-work-item"
	if legacy && d.WorkItem.TypeKey == DesignTypeKey {
		typeKey = "design"
		resourceType = "design-work-item"
	} else if legacy {
		typeKey = strings.TrimSuffix(d.WorkItem.TypeKey, "-v1")
	}
	return ReferenceView{TenantID: tenant, ID: id, WorkItemID: id, ProjectID: d.WorkItem.ProjectID, TypeKey: typeKey, ResourceType: resourceType, Status: d.WorkItem.Status, Revision: d.WorkItem.Revision, CurrentStageKey: d.WorkItem.CurrentStageKey, CompletionGaps: gaps}
}
func referenceStatus(err error) string {
	if errors.Is(err, ErrReferenceUnavailable) {
		return "unavailable"
	}
	return "invalid"
}
func requestHash(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
