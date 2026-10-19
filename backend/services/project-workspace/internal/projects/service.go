package projects

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

type Store interface {
	CounterpartyNameSuggestions(context.Context, string, CounterpartyNamesQuery) (CounterpartyNames, error)
	CounterpartyContext(context.Context, string, string) (CounterpartyContextView, error)
	ChangeCounterpartyContext(context.Context, Actor, string, CounterpartyContextInput) (CounterpartyContextView, error)
	Ping(context.Context) error
	List(context.Context, string, int) ([]Project, error)
	Directory(context.Context, string, DirectoryQuery) (DirectoryPage, error)
	Create(context.Context, Actor, CreateInput) (Project, error)
	ReplayCreate(context.Context, string, CreateInput) (Project, bool, error)
	DispatchInitiation(context.Context, Actor, InitiationDispatchInput) (Project, error)
	ReplayInitiationDispatch(context.Context, string, InitiationDispatchInput) (Project, bool, error)
	CreatePackage(context.Context, Actor, PackageCreateInput) (Project, error)
	ConvertPackage(context.Context, Actor, string, PackageConversionCommand) (PackageConversionResult, error)
	RollbackPackageConversion(context.Context, Actor, string, string, PackageRollbackCommand) (PackageConversionResult, error)
	Update(context.Context, Actor, string, UpdateInput) (Project, error)
	CorrectProfile(context.Context, Actor, string, ProfileCorrectionInput) (Project, error)
	ReplayBasicInfoChange(context.Context, string, string, BasicInfoChangeInput) (BasicInfoChangeResult, bool, error)
	ChangeBasicInfo(context.Context, Actor, string, BasicInfoChangeInput) (BasicInfoChangeResult, error)
	ReplayInitiationInfoChange(context.Context, string, string, BasicInfoChangeInput) (BasicInfoChangeResult, bool, error)
	ChangeInitiationInfo(context.Context, Actor, string, BasicInfoChangeInput) (BasicInfoChangeResult, error)
	ReplayAssignment(context.Context, string, string, AssignmentCommand) (AssignmentResult, bool, error)
	ApplyAssignment(context.Context, Actor, string, AssignmentCommand) (AssignmentResult, error)
	ReplayResourceLink(context.Context, string, string, ResourceLinkCommand) (ResourceLinkResult, bool, error)
	ApplyResourceLink(context.Context, Actor, string, ResourceLinkCommand) (ResourceLinkResult, error)
	CompleteInitiation(context.Context, Actor, string, RevisionCommand) (Project, error)
	Cancel(context.Context, Actor, string, CancelCommand) (Project, error)
	Get(context.Context, string, string, bool) (Details, error)
	History(context.Context, string, string, string, int) (HistoryPage, error)
	LatestCompletionConfirmation(context.Context, string, string, string) (*CompletionConfirmation, error)
	ReplayCompletion(context.Context, string, string, CompletionCommand) (CompletionConfirmation, bool, error)
	ConfirmCompletion(context.Context, Actor, string, CompletionCommand, CompletionAssessment) (CompletionConfirmation, error)
}

type ReferenceValidator interface {
	ValidatePeople(context.Context, string, string, string, int64) (ReferenceSnapshot, error)
	ValidateParty(context.Context, string, string, int64) (PartySnapshot, error)
	ValidateResource(context.Context, string, string, string, string, *int64) error
}

type ResourceReader interface {
	ResolveResource(context.Context, string, string, string, string) (ResourceReference, error)
}

type CompletionReader interface {
	ResolveCompletionResults(context.Context, string, string) ([]CompletionResultReference, error)
}

type Service struct {
	store      Store
	references ReferenceValidator
	now        func() time.Time
}

func NewService(store Store, references ReferenceValidator) (*Service, error) {
	if store == nil || references == nil {
		return nil, fmt.Errorf("store and reference validator are required")
	}
	return &Service{store: store, references: references, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (s *Service) Ping(ctx context.Context) error { return s.store.Ping(ctx) }
func (s *Service) List(ctx context.Context, tenantID string, limit int) ([]Project, error) {
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("limit must be between 1 and 100")
	}
	return s.store.List(ctx, tenantID, limit)
}
func (s *Service) Create(ctx context.Context, actor Actor, input CreateInput) (Project, error) {
	normalized, err := NormalizeCreate(input)
	if err != nil {
		return Project{}, err
	}
	if replay, found, err := s.store.ReplayCreate(ctx, actor.TenantID, normalized); err != nil {
		return Project{}, err
	} else if found {
		return replay, nil
	}
	if normalized.ClientParty != nil {
		snapshot, err := s.references.ValidateParty(ctx, actor.TenantID, normalized.ClientParty.PartyID, normalized.ClientParty.SourceRevision)
		if err != nil {
			return Project{}, err
		}
		normalized.validatedClientParty = &snapshot
	}
	return s.store.Create(ctx, actor, normalized)
}
func (s *Service) CreatePackage(ctx context.Context, actor Actor, input PackageCreateInput) (Project, error) {
	normalized, err := NormalizePackage(input)
	if err != nil {
		return Project{}, err
	}
	return s.store.CreatePackage(ctx, actor, normalized)
}
func (s *Service) ConvertPackage(ctx context.Context, actor Actor, id string, input PackageConversionCommand) (PackageConversionResult, error) {
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	input.Operation = strings.ToLower(strings.TrimSpace(input.Operation))
	input.TargetPackageID = strings.ToLower(strings.TrimSpace(input.TargetPackageID))
	if !ValidUUID(id) || !ValidUUID(input.CommandID) || input.ExpectedProjectRevision < 1 || input.SortOrder < 0 {
		return PackageConversionResult{}, fmt.Errorf("invalid package conversion command")
	}
	switch input.Operation {
	case ConversionAttachExisting:
		if !ValidUUID(input.TargetPackageID) || input.ExpectedPackageRevision < 1 || input.NewPackage != nil {
			return PackageConversionResult{}, fmt.Errorf("invalid existing package conversion")
		}
	case ConversionWrapAsPackage:
		if input.TargetPackageID != "" || input.ExpectedPackageRevision != 0 || input.NewPackage == nil {
			return PackageConversionResult{}, fmt.Errorf("invalid wrapper package conversion")
		}
		input.NewPackage.CommandID = input.CommandID
		normalized, err := NormalizePackage(*input.NewPackage)
		if err != nil {
			return PackageConversionResult{}, err
		}
		input.NewPackage = &normalized
	default:
		return PackageConversionResult{}, fmt.Errorf("invalid package conversion operation")
	}
	return s.store.ConvertPackage(ctx, actor, id, input)
}
func (s *Service) RollbackPackageConversion(ctx context.Context, actor Actor, projectID, conversionID string, input PackageRollbackCommand) (PackageConversionResult, error) {
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	input.Reason = strings.TrimSpace(input.Reason)
	if !ValidUUID(projectID) || !ValidUUID(conversionID) || !ValidUUID(input.CommandID) || input.ExpectedProjectRevision < 1 || input.ExpectedPackageRevision < 1 || input.Reason == "" || len([]rune(input.Reason)) > 1000 {
		return PackageConversionResult{}, fmt.Errorf("invalid package rollback command")
	}
	return s.store.RollbackPackageConversion(ctx, actor, projectID, conversionID, input)
}
func (s *Service) Update(ctx context.Context, actor Actor, id string, input UpdateInput) (Project, error) {
	normalized, err := s.normalizeProfile(ctx, actor.TenantID, id, input)
	if err != nil {
		return Project{}, err
	}
	return s.store.Update(ctx, actor, id, normalized)
}

func (s *Service) CorrectProfile(ctx context.Context, actor Actor, id string, input ProfileCorrectionInput) (Project, error) {
	input.CorrectionReason = strings.TrimSpace(input.CorrectionReason)
	if input.CorrectionReason == "" || len([]rune(input.CorrectionReason)) > 1000 {
		return Project{}, fmt.Errorf("correction reason is required (at most 1000 characters)")
	}
	normalized, err := s.normalizeProfile(ctx, actor.TenantID, id, input.UpdateInput)
	if err != nil {
		return Project{}, err
	}
	input.UpdateInput = normalized
	return s.store.CorrectProfile(ctx, actor, id, input)
}

func (s *Service) normalizeProfile(ctx context.Context, tenantID, id string, input UpdateInput) (UpdateInput, error) {
	if !ValidUUID(id) {
		return UpdateInput{}, fmt.Errorf("invalid project id")
	}
	if input.ExpectedRevision < 1 {
		return UpdateInput{}, fmt.Errorf("expected revision is required")
	}
	if input.ClientParty != nil {
		return UpdateInput{}, fmt.Errorf("client party can only be selected when creating a project")
	}
	details, err := s.store.Get(ctx, tenantID, id, true)
	if err != nil {
		return UpdateInput{}, err
	}
	if details.Project.NodeType == NodeTypePackage {
		normalized, err := NormalizePackage(PackageCreateInput{
			CommandID: input.CommandID, ProjectNumber: input.ProjectNumber, Name: input.Name,
			Summary: input.Summary, PlannedStartDate: input.PlannedStartDate,
			PlannedEndDate: input.PlannedEndDate, ProjectTypeKeys: input.ProjectTypeKeys,
		})
		if err != nil {
			return UpdateInput{}, err
		}
		input.CreateInput = CreateInput{
			CommandID: normalized.CommandID, ProjectNumber: normalized.ProjectNumber, Name: normalized.Name,
			Summary: normalized.Summary, PlannedStartDate: normalized.PlannedStartDate,
			PlannedEndDate: normalized.PlannedEndDate, ProjectTypeKeys: normalized.ProjectTypeKeys,
		}
	} else {
		normalized, err := NormalizeCreate(input.CreateInput)
		if err != nil {
			return UpdateInput{}, err
		}
		input.CreateInput = normalized
	}
	return input, nil
}
func (s *Service) ApplyAssignment(ctx context.Context, actor Actor, id string, input AssignmentCommand) (AssignmentResult, error) {
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	input.Operation = strings.ToLower(strings.TrimSpace(input.Operation))
	input.SubjectKind = strings.ToLower(strings.TrimSpace(input.SubjectKind))
	input.RoleKey = strings.ToLower(strings.TrimSpace(input.RoleKey))
	input.EndReason = strings.TrimSpace(input.EndReason)
	if !ValidUUID(id) || !ValidUUID(input.CommandID) || input.ExpectedRevision < 1 {
		return AssignmentResult{}, fmt.Errorf("command id and expected revision are required")
	}
	switch input.Operation {
	case "assign":
		if (input.SubjectKind != "person" && input.SubjectKind != "group") || !ValidUUID(input.SubjectID) || input.SourceRevision < 1 || !ValidRole(input.RoleKey) {
			return AssignmentResult{}, fmt.Errorf("invalid assignment")
		}
		if input.RoleKey == RoleCommercialContributor && input.SubjectKind != "person" {
			return AssignmentResult{}, fmt.Errorf("commercial contributor must reference a person")
		}
		if replay, found, err := s.store.ReplayAssignment(ctx, actor.TenantID, id, input); err != nil {
			return AssignmentResult{}, err
		} else if found {
			return replay, nil
		}
		snapshot, err := s.references.ValidatePeople(ctx, actor.TenantID, input.SubjectKind, input.SubjectID, input.SourceRevision)
		if err != nil {
			return AssignmentResult{}, err
		}
		input.DisplaySnapshot = snapshot.DisplayName
	case "end":
		if !ValidUUID(input.AssignmentID) || input.EndReason == "" || len([]rune(input.EndReason)) > 500 {
			return AssignmentResult{}, fmt.Errorf("assignment id and end reason are required")
		}
	default:
		return AssignmentResult{}, fmt.Errorf("assignment operation must be assign or end")
	}
	return s.store.ApplyAssignment(ctx, actor, id, input)
}
func (s *Service) ApplyResourceLink(ctx context.Context, actor Actor, id string, input ResourceLinkCommand) (ResourceLinkResult, error) {
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	input.Operation = strings.ToLower(strings.TrimSpace(input.Operation))
	input.TargetService = strings.ToLower(strings.TrimSpace(input.TargetService))
	input.ResourceType = strings.ToLower(strings.TrimSpace(input.ResourceType))
	input.RelationKey = strings.ToLower(strings.TrimSpace(input.RelationKey))
	input.Note = strings.TrimSpace(input.Note)
	if !ValidUUID(id) || !ValidUUID(input.CommandID) || input.ExpectedRevision < 1 || len([]rune(input.Note)) > 500 {
		return ResourceLinkResult{}, fmt.Errorf("invalid resource link command")
	}
	switch input.Operation {
	case "link":
		if !ValidUUID(input.ResourceID) || !ValidResource(input.TargetService, input.ResourceType, input.RelationKey) {
			return ResourceLinkResult{}, fmt.Errorf("invalid resource link")
		}
		if replay, found, err := s.store.ReplayResourceLink(ctx, actor.TenantID, id, input); err != nil {
			return ResourceLinkResult{}, err
		} else if found {
			return replay, nil
		}
		if input.TargetService == "production-operations" || recordedMaterialResource(input.TargetService, input.ResourceType) {
			reader, ok := s.references.(ResourceReader)
			if !ok {
				return ResourceLinkResult{}, ErrReferenceUnavailable
			}
			value, err := reader.ResolveResource(ctx, actor.TenantID, input.TargetService, input.ResourceType, input.ResourceID)
			if err != nil {
				return ResourceLinkResult{}, err
			}
			if value.ID != input.ResourceID || value.TenantID != actor.TenantID || value.SourceApplication != input.TargetService || value.ResourceType != input.ResourceType || value.MaterialSource == nil || value.MaterialSource.ProjectID != id || (input.SourceRevision != nil && value.Revision != *input.SourceRevision) || (recordedMaterialResource(input.TargetService, input.ResourceType) && value.Status != "recorded") {
				return ResourceLinkResult{}, ErrReferenceInvalid
			}
		} else if input.TargetService == "party-directory" && input.ResourceType == "party" {
			if input.SourceRevision == nil || *input.SourceRevision < 1 {
				return ResourceLinkResult{}, fmt.Errorf("party resource link requires source revision")
			}
			if _, err := s.references.ValidateParty(ctx, actor.TenantID, input.ResourceID, *input.SourceRevision); err != nil {
				return ResourceLinkResult{}, err
			}
		} else if err := s.references.ValidateResource(ctx, actor.TenantID, input.TargetService, input.ResourceType, input.ResourceID, input.SourceRevision); err != nil {
			return ResourceLinkResult{}, err
		}
	case "unlink":
		if !ValidUUID(input.LinkID) {
			return ResourceLinkResult{}, fmt.Errorf("link id is required")
		}
	default:
		return ResourceLinkResult{}, fmt.Errorf("resource link operation must be link or unlink")
	}
	return s.store.ApplyResourceLink(ctx, actor, id, input)
}
func (s *Service) CompleteInitiation(ctx context.Context, actor Actor, id string, input RevisionCommand) (Project, error) {
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	if !ValidUUID(id) || !ValidUUID(input.CommandID) || input.ExpectedRevision < 1 {
		return Project{}, fmt.Errorf("command id and expected revision are required")
	}
	var err error
	input.InitiationProfile, err = normalizeInitiationProfile(input.InitiationProfile)
	if err != nil {
		return Project{}, err
	}
	return s.store.CompleteInitiation(ctx, actor, id, input)
}
func (s *Service) Cancel(ctx context.Context, actor Actor, id string, input CancelCommand) (Project, error) {
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	input.Reason = strings.TrimSpace(input.Reason)
	if !ValidUUID(id) || !ValidUUID(input.CommandID) || input.ExpectedRevision < 1 || input.Reason == "" || len([]rune(input.Reason)) > 1000 || input.InitiationProfile != "" {
		return Project{}, fmt.Errorf("invalid cancellation command")
	}
	return s.store.Cancel(ctx, actor, id, input)
}
func (s *Service) Get(ctx context.Context, tenantID, id string, includeInactive bool) (Details, error) {
	if !ValidUUID(id) {
		return Details{}, fmt.Errorf("invalid project id")
	}
	details, err := s.store.Get(ctx, tenantID, id, includeInactive)
	if err != nil {
		return Details{}, err
	}
	details.ReferenceStatuses = s.referenceStatuses(ctx, tenantID, details.ResourceLinks)
	return details, nil
}

func (s *Service) referenceStatuses(ctx context.Context, tenantID string, links []ResourceLink) []ReferenceLinkStatus {
	statuses := make([]ReferenceLinkStatus, len(links))
	var wait sync.WaitGroup
	semaphore := make(chan struct{}, 8)
	for index, link := range links {
		index, link := index, link
		wait.Add(1)
		go func() {
			defer wait.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			status := "available"
			var err error
			if link.TargetService == "party-directory" && link.ResourceType == "party" {
				if link.SourceRevision == nil || *link.SourceRevision < 1 {
					err = ErrReferenceInvalid
				} else {
					_, err = s.references.ValidateParty(ctx, tenantID, link.ResourceID, *link.SourceRevision)
				}
			} else {
				err = s.references.ValidateResource(ctx, tenantID, link.TargetService, link.ResourceType, link.ResourceID, link.SourceRevision)
			}
			if err != nil {
				status = "invalid"
				if errors.Is(err, ErrReferenceUnavailable) {
					status = "unavailable"
				} else if errors.Is(err, ErrReferenceForbidden) {
					status = "forbidden"
				}
			}
			statuses[index] = ReferenceLinkStatus{LinkID: link.ID, Status: status}
		}()
	}
	wait.Wait()
	return statuses
}

func (s *Service) CommercialSummary(ctx context.Context, tenantID, id string) (CommercialSummary, error) {
	if !ValidUUID(id) {
		return CommercialSummary{}, fmt.Errorf("invalid project id")
	}
	startedAt := s.now()
	details, err := s.store.Get(ctx, tenantID, id, false)
	if err != nil {
		return CommercialSummary{}, err
	}
	links := make([]ResourceLink, 0, len(details.ResourceLinks))
	for _, link := range details.ResourceLinks {
		if link.UnlinkedAt == nil && commercialResource(link.TargetService, link.ResourceType) {
			links = append(links, link)
		}
	}
	segments := make([]CommercialSegment, len(links))
	reader, canRead := s.references.(ResourceReader)
	var wait sync.WaitGroup
	semaphore := make(chan struct{}, 8)
	for index, link := range links {
		index, link := index, link
		wait.Add(1)
		go func() {
			defer wait.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			segmentStartedAt := s.now()
			segment := CommercialSegment{
				LinkID: link.ID, SourceApplication: link.TargetService, ResourceType: link.ResourceType,
				ResourceID: link.ResourceID, RelationKey: link.RelationKey, LinkedAt: link.LinkedAt, LinkedRevision: link.SourceRevision,
				State: CommercialSegmentUnavailable, ErrorCode: "source_unavailable", DeepLink: commercialDeepLink(link),
			}
			if canRead {
				value, resolveErr := reader.ResolveResource(ctx, tenantID, link.TargetService, link.ResourceType, link.ResourceID)
				if resolveErr == nil {
					segment.SourceApplication = value.SourceApplication
					segment.SourceRevision = &value.Revision
					segment.State = CommercialSegmentAvailable
					segment.ErrorCode = ""
					segment.DisplaySummary = value.DisplaySummary
					segment.BusinessStatus = value.Status
					segment.Contract = value.Contract
					segment.Obligation = value.Obligation
					segment.Transaction = value.Transaction
					segment.Invoice = value.Invoice
					if link.SourceRevision != nil && *link.SourceRevision != value.Revision {
						segment.State = CommercialSegmentRevisionChanged
						segment.ErrorCode = "source_revision_changed"
					}
				} else if errors.Is(resolveErr, ErrReferenceInvalid) {
					segment.State = CommercialSegmentInvalid
					segment.ErrorCode = "reference_invalid"
				}
			}
			segment.ObservedAt = s.now()
			segment.DurationMS = segment.ObservedAt.Sub(segmentStartedAt).Milliseconds()
			segments[index] = segment
		}()
	}
	wait.Wait()
	observedAt := s.now()
	return CommercialSummary{
		ProjectID: details.Project.ID, ProjectRevision: details.Project.Revision,
		ObservedAt: observedAt, TotalDurationMS: observedAt.Sub(startedAt).Milliseconds(), Segments: segments,
	}, nil
}

func (s *Service) Materials(ctx context.Context, tenantID, id string) (MaterialCollection, error) {
	if !ValidUUID(id) {
		return MaterialCollection{}, fmt.Errorf("invalid project id")
	}
	startedAt := s.now()
	details, err := s.store.Get(ctx, tenantID, id, false)
	if err != nil {
		return MaterialCollection{}, err
	}
	links := make([]ResourceLink, 0, len(details.ResourceLinks))
	for _, link := range details.ResourceLinks {
		if link.UnlinkedAt == nil && materialResource(link.TargetService, link.ResourceType) {
			links = append(links, link)
		}
	}
	grouped := make([][]MaterialSegment, len(links))
	reader, canRead := s.references.(ResourceReader)
	var wait sync.WaitGroup
	semaphore := make(chan struct{}, 4)
	for index, link := range links {
		index, link := index, link
		wait.Add(1)
		go func() {
			defer wait.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			if !canRead {
				grouped[index] = []MaterialSegment{unavailableMaterialSegment(s.now(), link)}
				return
			}
			if link.TargetService == "business-engagement" || link.TargetService == "production-operations" || recordedMaterialResource(link.TargetService, link.ResourceType) {
				grouped[index] = s.resolveIndirectMaterials(ctx, reader, tenantID, details.Project.ID, link)
				return
			}
			grouped[index] = []MaterialSegment{s.resolveDirectMaterial(ctx, reader, tenantID, link)}
		}()
	}
	wait.Wait()
	segments := make([]MaterialSegment, 0, len(links))
	for _, values := range grouped {
		segments = append(segments, values...)
	}
	observedAt := s.now()
	state := MaterialCollectionAvailable
	if len(segments) == 0 {
		state = MaterialCollectionEmpty
	}
	return MaterialCollection{
		ProjectID: details.Project.ID, ProjectRevision: details.Project.Revision, State: state,
		ObservedAt: observedAt, TotalDurationMS: observedAt.Sub(startedAt).Milliseconds(), Segments: segments,
	}, nil
}

func materialResource(service, resourceType string) bool {
	return (service == "evidence-service" && resourceType == "evidence-file") ||
		(service == "business-engagement" && resourceType == "business-engagement") ||
		(service == "production-operations" && resourceType == "production-work-item") || recordedMaterialResource(service, resourceType)
}

func recordedMaterialResource(service, resourceType string) bool {
	return (service == "approval-audit-results" && resourceType == "approval_audit_result") ||
		(service == "cost-outsourcing" && (resourceType == "cost_entry" || resourceType == "outsourcing_engagement"))
}

func unavailableMaterialSegment(observedAt time.Time, link ResourceLink) MaterialSegment {
	return MaterialSegment{
		LinkID: link.ID, SourceApplication: link.TargetService, ResourceType: link.ResourceType,
		ResourceID: link.ResourceID, RelationKey: link.RelationKey, SourceRelationKey: link.RelationKey,
		LinkedRevision: link.SourceRevision, State: MaterialSegmentUnavailable,
		ErrorCode: "source_unavailable", ObservedAt: observedAt,
	}
}

func (s *Service) resolveDirectMaterial(ctx context.Context, reader ResourceReader, tenantID string, link ResourceLink) MaterialSegment {
	startedAt := s.now()
	segment := unavailableMaterialSegment(startedAt, link)
	segment.EvidenceFileID = link.ResourceID
	segment.EvidenceLinkedRevision = link.SourceRevision
	segment.DeepLink = "/evidence/?file_id=" + link.ResourceID
	value, err := reader.ResolveResource(ctx, tenantID, link.TargetService, link.ResourceType, link.ResourceID)
	if err == nil && value.Evidence != nil {
		segment.SourceApplication = value.SourceApplication
		segment.SourceRevision = &value.Revision
		segment.SourceDigest = value.Evidence.SHA256
		segment.SourceStatus = value.Status
		segment.State, segment.ErrorCode = MaterialSegmentAvailable, ""
		segment.EvidenceSourceRevision = &value.Revision
		segment.EvidenceVersion = value.Evidence.Version
		segment.EvidenceDigest = value.Evidence.SHA256
		segment.FileName = value.Evidence.FileName
		segment.SizeBytes = value.Evidence.SizeBytes
		segment.MediaType = value.Evidence.MediaType
		if link.SourceRevision != nil && *link.SourceRevision != value.Revision {
			segment.State, segment.ErrorCode = MaterialSegmentRevisionChanged, "source_revision_changed"
		}
	} else {
		applyMaterialResolveError(&segment, err)
	}
	segment.ObservedAt = s.now()
	segment.DurationMS = segment.ObservedAt.Sub(startedAt).Milliseconds()
	return segment
}

func (s *Service) resolveIndirectMaterials(ctx context.Context, reader ResourceReader, tenantID, projectID string, link ResourceLink) []MaterialSegment {
	startedAt := s.now()
	base := unavailableMaterialSegment(startedAt, link)
	value, err := reader.ResolveResource(ctx, tenantID, link.TargetService, link.ResourceType, link.ResourceID)
	if err != nil || value.MaterialSource == nil || value.MaterialSource.ProjectID != projectID || value.ID != link.ResourceID || value.TenantID != tenantID || value.SourceApplication != link.TargetService || value.ResourceType != link.ResourceType || value.Revision < 1 {
		if err == nil {
			err = ErrReferenceInvalid
		}
		applyMaterialResolveError(&base, err)
		base.ObservedAt = s.now()
		base.DurationMS = base.ObservedAt.Sub(startedAt).Milliseconds()
		return []MaterialSegment{base}
	}
	base.SourceApplication = value.SourceApplication
	base.SourceRevision = &value.Revision
	base.SourceDigest = value.MaterialSource.Digest
	base.SourceStatus = value.Status
	if recordedMaterialResource(link.TargetService, link.ResourceType) && value.Status != "recorded" {
		base.State, base.ErrorCode = MaterialSegmentInvalid, "source_withdrawn"
		if value.Status != "withdrawn" {
			base.ErrorCode = "reference_invalid"
		}
		base.ObservedAt = s.now()
		base.DurationMS = base.ObservedAt.Sub(startedAt).Milliseconds()
		return []MaterialSegment{base}
	}
	if len(value.MaterialSource.Items) == 0 {
		base.State, base.ErrorCode = MaterialSegmentEmpty, ""
		if link.SourceRevision != nil && *link.SourceRevision != value.Revision {
			base.State, base.ErrorCode = MaterialSegmentRevisionChanged, "source_revision_changed"
		}
		base.ObservedAt = s.now()
		base.DurationMS = base.ObservedAt.Sub(startedAt).Milliseconds()
		return []MaterialSegment{base}
	}
	segments := make([]MaterialSegment, 0, len(value.MaterialSource.Items))
	for _, item := range value.MaterialSource.Items {
		segmentStartedAt := s.now()
		segment := base
		segment.RelationKey = item.RelationKey
		segment.EvidenceFileID = item.FileID
		linkedEvidenceRevision := item.EvidenceSourceRevision
		segment.EvidenceLinkedRevision = &linkedEvidenceRevision
		segment.EvidenceVersion = item.Version
		segment.FileName = item.DisplayName
		segment.DeepLink = "/evidence/?file_id=" + item.FileID
		evidence, resolveErr := reader.ResolveResource(ctx, tenantID, "evidence-service", "evidence-file", item.FileID)
		if resolveErr == nil && evidence.Evidence != nil {
			segment.State, segment.ErrorCode = MaterialSegmentAvailable, ""
			segment.EvidenceSourceRevision = &evidence.Revision
			segment.EvidenceVersion = evidence.Evidence.Version
			segment.EvidenceDigest = evidence.Evidence.SHA256
			segment.FileName = evidence.Evidence.FileName
			segment.SizeBytes = evidence.Evidence.SizeBytes
			segment.MediaType = evidence.Evidence.MediaType
			if (link.SourceRevision != nil && *link.SourceRevision != value.Revision) ||
				item.EvidenceSourceRevision != evidence.Revision || item.Version != evidence.Evidence.Version {
				segment.State, segment.ErrorCode = MaterialSegmentRevisionChanged, "source_revision_changed"
			}
		} else {
			applyMaterialResolveError(&segment, resolveErr)
		}
		segment.ObservedAt = s.now()
		segment.DurationMS = segment.ObservedAt.Sub(segmentStartedAt).Milliseconds()
		segments = append(segments, segment)
	}
	return segments
}

func applyMaterialResolveError(segment *MaterialSegment, err error) {
	switch {
	case err == nil, errors.Is(err, ErrReferenceInvalid):
		segment.State, segment.ErrorCode = MaterialSegmentInvalid, "reference_invalid"
	case errors.Is(err, ErrReferenceForbidden):
		segment.State, segment.ErrorCode = MaterialSegmentForbidden, "source_forbidden"
	default:
		segment.State, segment.ErrorCode = MaterialSegmentUnavailable, "source_unavailable"
	}
}

func (s *Service) EvaluateCompletion(ctx context.Context, tenantID, id, profileKey string) (CompletionAssessment, error) {
	if !ValidUUID(id) || profileKey != CompletionProfileReferenceOAC5V1 {
		return CompletionAssessment{}, fmt.Errorf("invalid completion profile lookup")
	}
	details, err := s.store.Get(ctx, tenantID, id, false)
	if err != nil {
		return CompletionAssessment{}, err
	}
	if details.Project.NodeType != NodeTypeProject || details.Project.Status != StatusActive {
		return CompletionAssessment{}, ErrStateConflict
	}
	assessment := CompletionAssessment{
		ProjectID: details.Project.ID, ProjectRevision: details.Project.Revision,
		ProfileKey: profileKey, OverallStatus: CompletionPending, ObservedAt: s.now(),
	}
	reader, ok := s.references.(CompletionReader)
	if !ok {
		assessment.OverallStatus = CompletionUnavailable
		assessment.Requirements = unavailableCompletionRequirements("结果读取端口未配置")
	} else {
		values, resolveErr := reader.ResolveCompletionResults(ctx, tenantID, id)
		if resolveErr != nil {
			assessment.OverallStatus = CompletionUnavailable
			assessment.Requirements = unavailableCompletionRequirements("批复审计结果暂不可用")
		} else {
			assessment.Requirements, assessment.OverallStatus = evaluateC5Results(values)
		}
	}
	confirmation, err := s.store.LatestCompletionConfirmation(ctx, tenantID, id, profileKey)
	if err != nil {
		return CompletionAssessment{}, err
	}
	assessment.LatestConfirmation = confirmation
	assessment.Drifted = completionDrifted(assessment, confirmation)
	return assessment, nil
}

func (s *Service) ConfirmCompletion(ctx context.Context, actor Actor, id string, input CompletionCommand) (CompletionConfirmation, error) {
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	input.ProfileKey = strings.TrimSpace(input.ProfileKey)
	if !ValidUUID(id) || !ValidUUID(input.CommandID) || input.ExpectedRevision < 1 || input.ProfileKey != CompletionProfileReferenceOAC5V1 {
		return CompletionConfirmation{}, fmt.Errorf("invalid completion confirmation")
	}
	expected, err := normalizeCompletionSources(input.ExpectedSources)
	if err != nil {
		return CompletionConfirmation{}, err
	}
	input.ExpectedSources = expected
	if replay, found, err := s.store.ReplayCompletion(ctx, actor.TenantID, id, input); err != nil {
		return CompletionConfirmation{}, err
	} else if found {
		return replay, nil
	}
	assessment, err := s.EvaluateCompletion(ctx, actor.TenantID, id, input.ProfileKey)
	if err != nil {
		return CompletionConfirmation{}, err
	}
	if assessment.ProjectRevision != input.ExpectedRevision {
		return CompletionConfirmation{}, ErrRevisionConflict
	}
	if assessment.OverallStatus == CompletionUnavailable {
		return CompletionConfirmation{}, ErrReferenceUnavailable
	}
	if assessment.OverallStatus != CompletionSatisfied {
		return CompletionConfirmation{}, ErrCompletionIncomplete
	}
	actual, err := completionSources(assessment.Requirements)
	if err != nil || !reflect.DeepEqual(actual, expected) {
		return CompletionConfirmation{}, ErrCompletionDrift
	}
	return s.store.ConfirmCompletion(ctx, actor, id, input, assessment)
}

func unavailableCompletionRequirements(reason string) []CompletionRequirement {
	return []CompletionRequirement{
		{Key: "approval_result_recorded", Status: CompletionUnavailable, Reason: reason},
		{Key: "audit_result_recorded", Status: CompletionUnavailable, Reason: reason},
	}
}

func evaluateC5Results(values []CompletionResultReference) ([]CompletionRequirement, string) {
	byKind := make(map[string]CompletionResultReference, len(values))
	for _, value := range values {
		if value.Kind == "approval" || value.Kind == "audit" {
			byKind[value.Kind] = value
		}
	}
	requirements := make([]CompletionRequirement, 0, 2)
	overall := CompletionSatisfied
	for _, item := range []struct{ key, kind, label string }{
		{"approval_result_recorded", "approval", "批复结果"},
		{"audit_result_recorded", "audit", "审计结果"},
	} {
		value, found := byKind[item.kind]
		if !found {
			requirements = append(requirements, CompletionRequirement{Key: item.key, Status: CompletionPending, Reason: item.label + "尚未登记"})
			overall = CompletionPending
			continue
		}
		copyValue := value
		status, reason := CompletionSatisfied, item.label+"已登记并可追溯"
		if value.State != "recorded" {
			status, reason, overall = CompletionPending, item.label+"已撤回，需要显式重新登记", CompletionPending
		}
		requirements = append(requirements, CompletionRequirement{Key: item.key, Status: status, Reason: reason, Source: &copyValue})
	}
	return requirements, overall
}

func completionSources(requirements []CompletionRequirement) ([]CompletionSource, error) {
	values := make([]CompletionSource, 0, len(requirements))
	for _, requirement := range requirements {
		if requirement.Status != CompletionSatisfied || requirement.Source == nil {
			return nil, ErrCompletionIncomplete
		}
		values = append(values, CompletionSource{RequirementKey: requirement.Key, ResultID: requirement.Source.ID, ResultKind: requirement.Source.Kind, Revision: requirement.Source.Revision, Digest: requirement.Source.Digest})
	}
	return normalizeCompletionSources(values)
}

func normalizeCompletionSources(values []CompletionSource) ([]CompletionSource, error) {
	if len(values) != 2 {
		return nil, fmt.Errorf("completion confirmation requires exact approval and audit sources")
	}
	seen := map[string]bool{}
	result := append([]CompletionSource(nil), values...)
	for index := range result {
		value := &result[index]
		value.RequirementKey = strings.TrimSpace(value.RequirementKey)
		value.ResultID = strings.ToLower(strings.TrimSpace(value.ResultID))
		value.ResultKind = strings.ToLower(strings.TrimSpace(value.ResultKind))
		value.Digest = strings.ToLower(strings.TrimSpace(value.Digest))
		if !ValidUUID(value.ResultID) || value.Revision < 1 || value.Digest == "" || seen[value.RequirementKey] ||
			(value.RequirementKey == "approval_result_recorded" && value.ResultKind != "approval") ||
			(value.RequirementKey == "audit_result_recorded" && value.ResultKind != "audit") ||
			(value.RequirementKey != "approval_result_recorded" && value.RequirementKey != "audit_result_recorded") {
			return nil, fmt.Errorf("invalid completion source")
		}
		seen[value.RequirementKey] = true
	}
	sort.Slice(result, func(i, j int) bool { return result[i].RequirementKey < result[j].RequirementKey })
	return result, nil
}

func completionDrifted(assessment CompletionAssessment, confirmation *CompletionConfirmation) bool {
	if confirmation == nil {
		return false
	}
	current, err := completionSources(assessment.Requirements)
	if err != nil {
		return true
	}
	frozen, err := normalizeCompletionSources(confirmation.Sources)
	return err != nil || !reflect.DeepEqual(current, frozen)
}

func commercialResource(service, resourceType string) bool {
	switch service + "/" + resourceType {
	case "contract-ledger/contract", "money-flow-ledger/obligation", "money-flow-ledger/transaction", "billing-invoice/invoice_registration":
		return true
	default:
		return false
	}
}

func commercialDeepLink(link ResourceLink) string {
	switch link.TargetService + "/" + link.ResourceType {
	case "contract-ledger/contract":
		return "/contract/contracts/" + link.ResourceID
	case "money-flow-ledger/obligation":
		return "/money/obligations/" + link.ResourceID
	case "money-flow-ledger/transaction":
		return "/money/cash-transactions/" + link.ResourceID
	case "billing-invoice/invoice_registration":
		return "/billing/?invoice_registration_id=" + link.ResourceID
	default:
		return ""
	}
}
func (s *Service) History(ctx context.Context, tenantID, id, cursor string, limit int) (HistoryPage, error) {
	if !ValidUUID(id) {
		return HistoryPage{}, fmt.Errorf("invalid project id")
	}
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 100 {
		return HistoryPage{}, fmt.Errorf("limit must be between 1 and 100")
	}
	return s.store.History(ctx, tenantID, id, cursor, limit)
}

func (s *Service) ReferenceProject(ctx context.Context, tenantID, id string) (ProjectReference, error) {
	if !ValidUUID(id) {
		return ProjectReference{}, fmt.Errorf("invalid project id")
	}
	details, err := s.store.Get(ctx, tenantID, id, true)
	if err != nil {
		return ProjectReference{}, err
	}
	links := make([]ResourceLinkReference, 0, len(details.ResourceLinks))
	for _, link := range details.ResourceLinks {
		if link.UnlinkedAt != nil {
			continue
		}
		links = append(links, ResourceLinkReference{
			TargetService: link.TargetService, ResourceType: link.ResourceType,
			ResourceID: link.ResourceID, SourceRevision: link.SourceRevision,
			RelationKey: link.RelationKey,
		})
	}
	return ProjectReference{
		ID:              details.Project.ID,
		TenantID:        details.Project.TenantID,
		ResourceType:    "project",
		ProjectNumber:   details.Project.ProjectNumber,
		DisplaySummary:  details.Project.Name,
		Status:          details.Project.Status,
		Revision:        details.Project.Revision,
		ProjectTypeKeys: append([]string(nil), details.Project.ProjectTypeKeys...),
		NodeType:        details.Project.NodeType,
		ParentProjectID: details.Project.ParentProjectID,
		ResourceLinks:   links,
	}, nil
}

func (s *Service) ReferenceAssignment(ctx context.Context, tenantID, projectID, assignmentID string) (AssignmentReference, error) {
	if !ValidUUID(projectID) || !ValidUUID(assignmentID) {
		return AssignmentReference{}, fmt.Errorf("invalid assignment reference")
	}
	details, err := s.store.Get(ctx, tenantID, projectID, true)
	if err != nil {
		return AssignmentReference{}, err
	}
	for _, assignment := range details.Assignments {
		if assignment.ID != assignmentID {
			continue
		}
		status := "active"
		revision := assignment.CreatedProjectRevision
		if assignment.EndedAt != nil {
			status = "ended"
			if assignment.EndedProjectRevision != nil {
				revision = *assignment.EndedProjectRevision
			}
		}
		return AssignmentReference{
			ID:             assignment.ID,
			TenantID:       assignment.TenantID,
			ProjectID:      assignment.ProjectID,
			ResourceType:   "project-assignment",
			SubjectKind:    assignment.SubjectKind,
			SubjectID:      assignment.SubjectID,
			SourceRevision: assignment.SourceRevision,
			RoleKey:        assignment.RoleKey,
			Status:         status,
			Revision:       revision,
			EffectiveFrom:  assignment.EffectiveFrom,
			EndedAt:        assignment.EndedAt,
			DisplayName:    assignment.DisplaySnapshot,
		}, nil
	}
	return AssignmentReference{}, ErrNotFound
}
