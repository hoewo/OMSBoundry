package design

import (
	"context"
	"errors"
	"testing"
)

type progressTestStore struct {
	stageTestStore
	item        WorkItem
	collections map[string]StageMaterialCollection
	calls       []string
}

func (s *progressTestStore) StageWorkItems(_ context.Context, tenant, project, typeKey string) ([]WorkItem, error) {
	if tenant != s.item.TenantID || project != s.item.ProjectID || typeKey != StageMaterialTypeKey {
		return nil, ErrNotFound
	}
	return []WorkItem{s.item}, nil
}

func (s *progressTestStore) StageMaterials(_ context.Context, tenant, id, stage string) (StageMaterialCollection, error) {
	s.calls = append(s.calls, stage)
	if tenant != s.item.TenantID || id != s.item.ID {
		return StageMaterialCollection{}, ErrNotFound
	}
	value, ok := s.collections[stage]
	if !ok {
		return StageMaterialCollection{}, ErrStageInvalid
	}
	return value, nil
}

func progressCollection(item WorkItem, key, status string, main, review int) StageMaterialCollection {
	mainFiles := make([]StageMaterial, main)
	reviewFiles := make([]StageMaterial, review)
	return StageMaterialCollection{
		SchemaVersion:       "1.0",
		Profile:             StageMaterialProfile,
		WorkItem:            item,
		Stage:               Stage{TenantID: item.TenantID, WorkItemID: item.ID, Key: key, Status: status},
		ProjectTypeSnapshot: []string{"consultation", "design", "mapping", "survey"},
		MainFiles:           mainFiles,
		ReviewSheets:        reviewFiles,
	}
}

func TestProductionProgressUsesOnlyProductionOwnedStageFacts(t *testing.T) {
	item := WorkItem{
		ID:                    workItemID,
		TenantID:              workItemTenant,
		ProjectID:             workItemProject,
		ProjectSourceRevision: 9,
		TypeKey:               StageMaterialTypeKey,
		TypeDefinitionVersion: StageMaterialDefinitionVersion,
		Status:                StatusInProgress,
		Revision:              8,
	}
	store := &progressTestStore{item: item, collections: map[string]StageMaterialCollection{}}
	for _, value := range []struct {
		key, status  string
		main, review int
	}{
		{"scheme", StageCompleted, 2, 1},
		{"test", StageCompleted, 2, 0},
		{"report", StageReady, 0, 0},
	} {
		store.collections[value.key] = progressCollection(item, value.key, value.status, value.main, value.review)
	}
	refs := &stageTestRefs{failAt: 1}
	service, err := NewServiceWithStageMaterials(store, refs, StageMaterialProfile)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := service.ProductionProgress(context.Background(), workItemTenant, workItemProject, StageMaterialTypeKey)
	if err != nil {
		t.Fatal(err)
	}
	if refs.calls != 0 {
		t.Fatal("progress read called Project or Evidence")
	}
	if projection.SchemaVersion != "1.0" || projection.Profile != ProductionProgressProfile || projection.ProjectID != workItemProject || len(projection.Items) != 1 {
		t.Fatalf("unexpected projection %#v", projection)
	}
	got := projection.Items[0]
	if got.ApplicableStageCount != 3 || got.CompletedStageCount != 2 || got.RequiredStageCount != 3 || got.RequiredCompleteCount != 2 || got.ProductionComplete {
		t.Fatalf("unexpected progress counts %#v", got)
	}
	if len(got.Stages) != 3 || got.Stages[0].StageKey != "scheme" || got.Stages[0].MainFileCount != 2 || got.Stages[0].ReviewFileCount != 1 || got.Stages[1].StageKey != "test" || got.Stages[2].StageKey != "report" {
		t.Fatalf("unexpected stage projection %#v", got.Stages)
	}
	if len(got.TypeProgress) != 4 || got.TypeProgress[0].ProjectTypeKey != "consultation" || got.TypeProgress[0].ApplicableStageCount != 2 || got.TypeProgress[1].ProjectTypeKey != "design" || got.TypeProgress[1].ApplicableStageCount != 1 || got.TypeProgress[3].ProjectTypeKey != "survey" || got.TypeProgress[3].ApplicableStageCount != 3 {
		t.Fatalf("unexpected per-type progress %#v", got.TypeProgress)
	}
	if len(store.calls) != 7 {
		t.Fatalf("definition closure not inspected: %#v", store.calls)
	}
}

func TestProductionProgressFailsClosedOnInvalidOwnerProjection(t *testing.T) {
	item := WorkItem{ID: workItemID, TenantID: workItemTenant, ProjectID: workItemProject, TypeKey: StageMaterialTypeKey, TypeDefinitionVersion: 2, Status: StatusInProgress, Revision: 1}
	bad := progressCollection(item, "scheme", StageReady, 0, 0)
	bad.Stage.WorkItemID = workItemPerson
	store := &progressTestStore{item: item, collections: map[string]StageMaterialCollection{"scheme": bad}}
	service, _ := NewServiceWithStageMaterials(store, &stageTestRefs{}, StageMaterialProfile)
	if _, err := service.ProductionProgress(context.Background(), workItemTenant, workItemProject, StageMaterialTypeKey); !errors.Is(err, ErrTypeInvalid) {
		t.Fatalf("invalid owner projection accepted: %v", err)
	}
}

func TestProductionProgressDoesNotMergeParallelStageTruths(t *testing.T) {
	item := WorkItem{ID: workItemID, TenantID: workItemTenant, ProjectID: workItemProject, TypeKey: StageMaterialTypeKey, TypeDefinitionVersion: 2, Status: StatusInProgress, Revision: 1}
	store := &progressTestStore{item: item, collections: map[string]StageMaterialCollection{}}
	store.stageTestStore.items = []WorkItem{item, item}
	service, _ := NewServiceWithStageMaterials(&parallelProgressStore{progressTestStore: store}, &stageTestRefs{}, StageMaterialProfile)
	if _, err := service.ProductionProgress(context.Background(), workItemTenant, workItemProject, StageMaterialTypeKey); !errors.Is(err, ErrTypeInvalid) {
		t.Fatalf("parallel stage truths were merged: %v", err)
	}
}

type parallelProgressStore struct{ *progressTestStore }

func (s *parallelProgressStore) StageWorkItems(context.Context, string, string, string) ([]WorkItem, error) {
	return append([]WorkItem(nil), s.stageTestStore.items...), nil
}
