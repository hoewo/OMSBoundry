package engagement

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type Store interface {
	Ping(context.Context) error
	List(context.Context, string, int) ([]Engagement, error)
	ListProject(context.Context, string, string, int) ([]Engagement, error)
	Get(context.Context, string, string) (Details, error)
	History(context.Context, string, string, int64, int) (HistoryPage, error)
	Replay(context.Context, string, string, string, any) (bool, error)
	Create(context.Context, Actor, CreateInput, ProjectReference, string) (Details, error)
	SetApplicability(context.Context, Actor, string, ApplicabilityInput, string) (Details, error)
	SelectMethod(context.Context, Actor, string, MethodInput, string) (Details, error)
	ApplyMaterial(context.Context, Actor, string, MaterialInput, string) (Details, error)
	Cancel(context.Context, Actor, string, CancelInput, string) (Details, error)
}

type ReferenceValidator interface {
	ValidateProject(context.Context, string, string, int64) (ProjectReference, error)
	ValidateEvidence(context.Context, string, string, int, int64) (EvidenceReference, error)
}

type Service struct {
	store      Store
	references ReferenceValidator
}

func NewService(store Store, references ReferenceValidator) (*Service, error) {
	if store == nil || references == nil {
		return nil, fmt.Errorf("store and reference validator are required")
	}
	return &Service{store: store, references: references}, nil
}

func (s *Service) Ping(ctx context.Context) error { return s.store.Ping(ctx) }

func (s *Service) List(ctx context.Context, tenantID string, limit int) ([]Engagement, error) {
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("limit must be between 1 and 100")
	}
	return s.store.List(ctx, tenantID, limit)
}

func (s *Service) Get(ctx context.Context, tenantID, id string) (Details, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if !ValidUUID(id) {
		return Details{}, fmt.Errorf("invalid engagement id")
	}
	return s.store.Get(ctx, tenantID, id)
}

// Filter at the source before limiting: an older project's record must not be
// missed merely because newer records filled the unfiltered page.
func (s *Service) ListProject(ctx context.Context, tenantID, projectID string, limit int) ([]Engagement, error) {
	projectID = strings.ToLower(strings.TrimSpace(projectID))
	if !ValidUUID(projectID) {
		return nil, fmt.Errorf("invalid project id")
	}
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("limit must be between 1 and 100")
	}
	return s.store.ListProject(ctx, tenantID, projectID, limit)
}

func (s *Service) ReferenceMaterials(ctx context.Context, tenantID, id string) (MaterialReferenceProjection, error) {
	details, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return MaterialReferenceProjection{}, err
	}
	items := make([]MaterialProjectionItem, 0, len(details.Materials))
	for _, material := range details.Materials {
		if material.SupersededAt != nil {
			continue
		}
		items = append(items, MaterialProjectionItem{
			RelationKey: material.RequirementKey, FileID: material.EvidenceFileID,
			Version: material.EvidenceVersion, EvidenceSourceRevision: material.EvidenceSourceRevision,
			DisplayName: material.DisplaySnapshot,
		})
	}
	sort.Slice(items, func(left, right int) bool {
		if items[left].RelationKey != items[right].RelationKey {
			return items[left].RelationKey < items[right].RelationKey
		}
		if items[left].FileID != items[right].FileID {
			return items[left].FileID < items[right].FileID
		}
		return items[left].Version < items[right].Version
	})
	digest, err := requestHash(items)
	if err != nil {
		return MaterialReferenceProjection{}, err
	}
	value := details.Engagement
	return MaterialReferenceProjection{
		SchemaVersion: "1", SourceApplication: "business-engagement", ID: value.ID,
		TenantID: value.TenantID, ProjectID: value.ProjectID, ResourceType: "business-engagement",
		Status: value.Status, Revision: value.Revision, Digest: digest, Items: items,
	}, nil
}

func (s *Service) History(ctx context.Context, tenantID, id string, after int64, limit int) (HistoryPage, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if !ValidUUID(id) || after < 0 {
		return HistoryPage{}, fmt.Errorf("invalid history query")
	}
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 200 {
		return HistoryPage{}, fmt.Errorf("limit must be between 1 and 200")
	}
	return s.store.History(ctx, tenantID, id, after, limit)
}

func (s *Service) Create(ctx context.Context, actor Actor, input CreateInput) (Details, error) {
	input, err := NormalizeCreate(input)
	if err != nil {
		return Details{}, err
	}
	hash, err := requestHash(input)
	if err != nil {
		return Details{}, err
	}
	var replay Details
	if found, err := s.store.Replay(ctx, actor.TenantID, input.CommandID, hash, &replay); err != nil || found {
		return replay, err
	}
	project, err := s.references.ValidateProject(ctx, actor.TenantID, input.ProjectID, input.ProjectSourceRevision)
	if err != nil {
		return Details{}, err
	}
	if project.Status != "active" || project.ResourceType != "project" {
		return Details{}, ErrReferenceInvalid
	}
	return s.store.Create(ctx, actor, input, project, hash)
}

func (s *Service) SetApplicability(ctx context.Context, actor Actor, id string, input ApplicabilityInput) (Details, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if !ValidUUID(id) {
		return Details{}, fmt.Errorf("invalid engagement id")
	}
	input, err := NormalizeApplicability(input)
	if err != nil {
		return Details{}, err
	}
	hash, err := requestHash(input)
	if err != nil {
		return Details{}, err
	}
	return s.store.SetApplicability(ctx, actor, id, input, hash)
}

func (s *Service) SelectMethod(ctx context.Context, actor Actor, id string, input MethodInput) (Details, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if !ValidUUID(id) {
		return Details{}, fmt.Errorf("invalid engagement id")
	}
	input, err := NormalizeMethod(input)
	if err != nil {
		return Details{}, err
	}
	hash, err := requestHash(input)
	if err != nil {
		return Details{}, err
	}
	return s.store.SelectMethod(ctx, actor, id, input, hash)
}

func (s *Service) ApplyMaterial(ctx context.Context, actor Actor, id string, input MaterialInput) (Details, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if !ValidUUID(id) {
		return Details{}, fmt.Errorf("invalid engagement id")
	}
	input, err := NormalizeMaterial(input)
	if err != nil {
		return Details{}, err
	}
	hash, err := requestHash(input)
	if err != nil {
		return Details{}, err
	}
	var replay Details
	if found, err := s.store.Replay(ctx, actor.TenantID, input.CommandID, hash, &replay); err != nil || found {
		return replay, err
	}
	if input.Operation == "put" || input.Operation == "append" {
		reference, err := s.references.ValidateEvidence(ctx, actor.TenantID, input.EvidenceFileID, input.EvidenceVersion, input.EvidenceSourceRevision)
		if err != nil {
			return Details{}, err
		}
		input.DisplaySnapshot = reference.DisplayName
	}
	return s.store.ApplyMaterial(ctx, actor, id, input, hash)
}

func (s *Service) Cancel(ctx context.Context, actor Actor, id string, input CancelInput) (Details, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if !ValidUUID(id) {
		return Details{}, fmt.Errorf("invalid engagement id")
	}
	input, err := NormalizeCancel(input)
	if err != nil {
		return Details{}, err
	}
	hash, err := requestHash(input)
	if err != nil {
		return Details{}, err
	}
	return s.store.Cancel(ctx, actor, id, input, hash)
}

func requestHash(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
