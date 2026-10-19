package design

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestStageScoreExactAndZero(t *testing.T) {
	for _, tc := range []struct{ in, want string }{{"0", "0"}, {"000.000", "0"}, {" 088.5000 ", "88.5"}, {"100.000", "100"}, {"0.12345678901234567890123456789", "0.12345678901234567890123456789"}, {"", ""}} {
		t.Run(tc.in, func(t *testing.T) {
			v, err := normalizeStageScore(&tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == "" {
				if v != nil {
					t.Fatal("blank did not preserve")
				}
			} else if v == nil || *v != tc.want {
				t.Fatalf("%v want %s", v, tc.want)
			}
		})
	}
	for _, v := range []string{"-1", "100.000000000000000001", "101", "1e1", "NaN", ".2", "1.", strings.Repeat("0", 129), "1/2"} {
		if _, err := normalizeStageScore(&v); err == nil {
			t.Fatalf("accepted %s", v)
		}
	}
}
func TestStageMaterialNormalizationIsBoundedAndDoesNotMutateCaller(t *testing.T) {
	files := []StageFileInput{{FileID: workItemPerson, Version: 1, SourceRevision: 1, DisplaySnapshot: "client forged"}}
	in := StageMaterialInput{RevisionInput: RevisionInput{CommandID: workItemAssignment, ExpectedRevision: 1}, MainFiles: files}
	normalized, err := normalizeStageMaterialInput(in)
	if err != nil {
		t.Fatal(err)
	}
	if files[0].DisplaySnapshot != "client forged" || normalized.MainFiles[0].DisplaySnapshot != "" || normalized.ReviewSheets == nil {
		t.Fatal("caller ownership or canonical arrays")
	}
	normalized.MainFiles = append(normalized.MainFiles, normalized.MainFiles[0])
	if _, err := normalizeStageMaterialInput(normalized); err == nil {
		t.Fatal("duplicate accepted")
	}
	in.MainFiles[0].SourceRevision = 2
	if _, err := normalizeStageMaterialInput(in); err == nil {
		t.Fatal("wrong file source revision")
	}
	in.MainFiles = nil
	if _, err := normalizeStageMaterialInput(in); err == nil {
		t.Fatal("empty intent")
	}
	in.MainFiles = make([]StageFileInput, 21)
	if _, err := normalizeStageMaterialInput(in); err == nil {
		t.Fatal("batch bound")
	}
}
func TestStageDefinitionOptInAndLegacyCreateJSON(t *testing.T) {
	for _, key := range []string{DesignTypeKey, ConsultationTypeKey} {
		latest, _ := LatestTypeDefinition(key)
		if _, ok := RegisteredTypeDefinition(key, 2); ok || latest.Version != 1 {
			t.Fatal("default version changed")
		}
	}
	{
		v2, ok := RegisteredTypeDefinition(StageMaterialTypeKey, 2)
		if !ok || len(v2.Stages) != 1 || v2.Stages[0].Key != "scheme" {
			t.Fatal("project-shared scheme identity")
		}
		if _, ok := LatestTypeDefinition(StageMaterialTypeKey); ok {
			t.Fatal("profile became default")
		}
		for _, st := range v2.Stages {
			if st.Skippable || len(st.RequiredRequirementKeys) != 1 || st.RequiredRequirementKeys[0] != MaterialMain {
				t.Fatal("bad v2 stage")
			}
		}
		v2.Stages[0].RequiredRequirementKeys[0] = "changed"
		again, _ := RegisteredTypeDefinition(StageMaterialTypeKey, 2)
		if again.Stages[0].RequiredRequirementKeys[0] != MaterialMain {
			t.Fatal("registry mutated")
		}
		v3, ok := RegisteredTypeDefinition(StageMaterialTypeKey, 3)
		if !ok || len(v3.Stages) != 2 || v3.Stages[0].Key != "scheme" || v3.Stages[1].Key != "preliminary-design" {
			t.Fatal("v3 preliminary definition")
		}
		if v3.Stages[1].Skippable || len(v3.Stages[1].RequiredRequirementKeys) != 1 || v3.Stages[1].RequiredRequirementKeys[0] != MaterialMain {
			t.Fatal("bad v3 preliminary stage")
		}
		v4, ok := RegisteredTypeDefinition(StageMaterialTypeKey, 4)
		if !ok || len(v4.Stages) != 3 || v4.Stages[2].Key != "construction-drawing" {
			t.Fatal("v4 construction definition")
		}
		if v4.Stages[2].Skippable || len(v4.Stages[2].RequiredRequirementKeys) != 1 || v4.Stages[2].RequiredRequirementKeys[0] != MaterialMain {
			t.Fatal("bad v4 construction stage")
		}
		v5, ok := RegisteredTypeDefinition(StageMaterialTypeKey, 5)
		if !ok || len(v5.Stages) != 5 || v5.Stages[3].Key != "change" || !v5.Stages[3].Skippable || v5.Stages[4].Key != "completion" || v5.Stages[4].Skippable {
			t.Fatal("v5 auxiliary definition")
		}
		v6, ok := RegisteredTypeDefinition(StageMaterialTypeKey, 6)
		if !ok || len(v6.Stages) != 7 || v6.Stages[5].Key != "test" || v6.Stages[6].Key != "report" {
			t.Fatal("v6 type-scoped definition")
		}
		if _, ok := RegisteredTypeDefinition(StageMaterialTypeKey, 7); ok {
			t.Fatal("unknown future definition registered")
		}
	}
	in := CreateInput{CommandID: workItemAssignment, ProjectID: workItemProject, ProjectSourceRevision: 1, TypeKey: DesignTypeKey}
	raw, _ := json.Marshal(in)
	if strings.Contains(string(raw), "type_definition_version") {
		t.Fatal("legacy JSON/hash changed")
	}
	s, _ := NewService(&workItemStore{}, &workItemReferences{})
	in.TypeDefinitionVersion = 2
	if _, err := s.Create(context.Background(), Actor{TenantID: workItemTenant, ID: workItemSubject}, in); !errors.Is(err, ErrTypeInvalid) {
		t.Fatalf("default service accepted v2: %v", err)
	}
	if _, err := NewServiceWithStageMaterials(&workItemStore{}, &workItemReferences{}, "unknown"); !errors.Is(err, ErrTypeInvalid) {
		t.Fatal("unknown profile")
	}
}
func TestMaterialProjectionPreservesCompletionTimeAndAllowsLaterStages(t *testing.T) {
	at := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	old := at.Add(-time.Hour)
	rev := int64(2)
	stages := []Stage{{Key: "scheme", Status: StageCompleted, CompletedAt: &old, CompletedRevision: &rev, UpdatedAt: old}, {Key: "preliminary-design", Status: StageReady, UpdatedAt: old}}
	projected, frontier, err := materialStageProjection(StageMaterialTypeKey, 3, stages, map[string]int{"scheme": 2, "preliminary-design": 1}, 3, at)
	if err != nil || frontier != nil || !projected[0].CompletedAt.Equal(old) || *projected[0].CompletedRevision != 2 || *projected[1].CompletedRevision != 3 {
		t.Fatal("completion timestamps changed")
	}
	projected, frontier, err = materialStageProjection(StageMaterialTypeKey, 3, projected, map[string]int{"preliminary-design": 1}, 4, at.Add(time.Minute))
	if err != nil || frontier == nil || *frontier != "scheme" || projected[0].Status != StageReady || projected[0].CompletedAt != nil || projected[1].Status != StageCompleted || *projected[1].CompletedRevision != 3 {
		t.Fatal("last main withdrawal projection")
	}
	if stages[1].Status != StageReady {
		t.Fatal("caller stages mutated")
	}
}

func TestMaterialProjectionSkipsOptionalChangeButTracksItsFiles(t *testing.T) {
	at := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	definition, _ := RegisteredTypeDefinition(StageMaterialTypeKey, 5)
	stages := make([]Stage, len(definition.Stages))
	for index, stage := range definition.Stages {
		stages[index] = Stage{Key: stage.Key, Status: StagePending, UpdatedAt: at.Add(-time.Hour)}
	}
	counts := map[string]int{"scheme": 1, "preliminary-design": 1, "construction-drawing": 1, "completion": 1}
	projected, frontier, err := materialStageProjection(StageMaterialTypeKey, 5, stages, counts, 8, at)
	if err != nil || frontier != nil || projected[3].Key != "change" || projected[3].Status != StagePending || projected[4].Status != StageCompleted {
		t.Fatalf("optional change blocked completion: %+v frontier=%v err=%v", projected, frontier, err)
	}
	counts["change"] = 1
	projected, frontier, err = materialStageProjection(StageMaterialTypeKey, 5, projected, counts, 9, at.Add(time.Minute))
	if err != nil || frontier != nil || projected[3].Status != StageCompleted || projected[4].Status != StageCompleted {
		t.Fatalf("change file projection: %+v frontier=%v err=%v", projected, frontier, err)
	}
}

func TestV6MaterialProjectionUsesFrozenProjectTypes(t *testing.T) {
	at := time.Date(2026, 10, 6, 15, 0, 0, 0, time.UTC)
	definition, _ := RegisteredTypeDefinition(StageMaterialTypeKey, 6)
	stages := make([]Stage, len(definition.Stages))
	for index, stage := range definition.Stages {
		stages[index] = Stage{Key: stage.Key, Status: StagePending, UpdatedAt: at.Add(-time.Hour)}
	}
	projected, frontier, err := materialStageProjectionForProject(StageMaterialTypeKey, 6, stages, map[string]int{"scheme": 1, "test": 1, "report": 1}, []string{"survey"}, 4, at)
	if err != nil || frontier != nil {
		t.Fatalf("survey projection frontier=%v err=%v", frontier, err)
	}
	for _, index := range []int{1, 2, 3, 4} {
		if projected[index].Status != StageNotApplicable || projected[index].NotApplicableReason == nil {
			t.Fatalf("design stage %s was not excluded: %+v", projected[index].Key, projected[index])
		}
	}
	if projected[5].Status != StageCompleted || projected[6].Status != StageCompleted {
		t.Fatalf("survey stages not complete: %+v", projected)
	}
	projected, frontier, err = materialStageProjectionForProject(StageMaterialTypeKey, 6, stages, map[string]int{"scheme": 1, "completion": 1}, []string{"design"}, 3, at)
	if err != nil || frontier == nil || *frontier != "preliminary-design" || projected[5].Status != StageNotApplicable || projected[6].Status != StageNotApplicable {
		t.Fatalf("design applicability: %+v frontier=%v err=%v", projected, frontier, err)
	}
	if applicable, err := stageMaterialApplicable(6, "report", []string{"mapping"}); err != nil || !applicable {
		t.Fatalf("mapping report applicability=%v err=%v", applicable, err)
	}
	if applicable, err := stageMaterialApplicable(6, "test", []string{"consultation"}); err != nil || applicable {
		t.Fatalf("consultation test applicability=%v err=%v", applicable, err)
	}
}

func TestAuxiliaryMaterialShapeAndFileTypes(t *testing.T) {
	zero := "0"
	if err := validateStageMaterialShape("change", StageMaterialInput{ReviewSheets: []StageFileInput{{FileID: workItemID}}}); err == nil {
		t.Fatal("change review accepted")
	}
	if err := validateStageMaterialShape("completion", StageMaterialInput{Score: &zero}); err == nil {
		t.Fatal("completion score accepted")
	}
	for _, tc := range []struct {
		stage, name string
		valid       bool
	}{
		{"change", "变更.xlsx", true}, {"change", "变更.zip", false}, {"completion", "竣工.zip", true}, {"completion", "竣工.RAR", true}, {"completion", "脚本.exe", false}, {"scheme", "任意.bin", true},
	} {
		if err := validateStageMaterialDisplayName(tc.stage, tc.name); (err == nil) != tc.valid {
			t.Fatalf("%s/%s valid=%v err=%v", tc.stage, tc.name, tc.valid, err)
		}
	}
}

type stageTestStore struct {
	workItemStore
	collection StageMaterialCollection
	writes     int
	last       StageMaterialInput
	replay     bool
	upgrades   int
	upgrade    DefinitionUpgradeInput
}

func (s *stageTestStore) StageMaterials(_ context.Context, tenant, id, stage string) (StageMaterialCollection, error) {
	if tenant != s.collection.WorkItem.TenantID || id != s.collection.WorkItem.ID {
		return StageMaterialCollection{}, ErrNotFound
	}
	if _, ok := StageDefinitionFor(s.collection.WorkItem.TypeKey, s.collection.WorkItem.TypeDefinitionVersion, stage); !ok {
		return StageMaterialCollection{}, ErrStageInvalid
	}
	if !IsStageMaterialDefinitionVersion(s.collection.WorkItem.TypeDefinitionVersion) {
		return StageMaterialCollection{}, ErrTypeInvalid
	}
	return s.collection, nil
}
func (s *stageTestStore) UpgradeStageMaterialDefinition(_ context.Context, actor Actor, id string, input DefinitionUpgradeInput, _ string) (Details, error) {
	s.upgrades++
	s.upgrade = input
	item := s.collection.WorkItem
	item.ID = id
	item.TenantID = actor.TenantID
	item.TypeDefinitionVersion = input.TargetVersion
	item.Revision = input.ExpectedRevision + 1
	return Details{WorkItem: item}, nil
}
func (s *stageTestStore) RegisterStageMaterials(_ context.Context, _ Actor, _, _ string, in StageMaterialInput, _ string) (StageMaterialCollection, error) {
	s.writes++
	s.last = in
	return s.collection, nil
}
func (s *stageTestStore) WithdrawStageMaterial(context.Context, Actor, string, string, string, ReasonInput, string) (StageMaterialCollection, error) {
	s.writes++
	return s.collection, nil
}
func (s *stageTestStore) StageWorkItems(context.Context, string, string, string) ([]WorkItem, error) {
	return nil, nil
}
func (s *stageTestStore) Replay(_ context.Context, _, _, _ string, dest any) (bool, error) {
	if !s.replay {
		return false, nil
	}
	raw, _ := json.Marshal(s.collection)
	return true, json.Unmarshal(raw, dest)
}

type stageTestRefs struct {
	workItemReferences
	calls       int
	failAt      int
	wrongTenant bool
}

func (r *stageTestRefs) ValidateEvidence(_ context.Context, tenant, id string, version int, revision int64) (EvidenceReference, error) {
	r.calls++
	if r.calls == r.failAt {
		return EvidenceReference{}, ErrReferenceUnavailable
	}
	if r.wrongTenant {
		tenant = workItemPerson
	}
	return EvidenceReference{TenantID: tenant, FileID: id, Version: version, Revision: revision, Status: "available", DisplayName: "verified.pdf"}, nil
}
func stageFixture() (*Service, *stageTestStore, *stageTestRefs, StageMaterialInput, Actor) {
	store := &stageTestStore{collection: StageMaterialCollection{WorkItem: WorkItem{ID: workItemID, TenantID: workItemTenant, ProjectID: workItemProject, TypeKey: StageMaterialTypeKey, TypeDefinitionVersion: 2, Status: StatusDraft, Revision: 1}, MainFiles: []StageMaterial{}, ReviewSheets: []StageMaterial{}}}
	refs := &stageTestRefs{}
	s, _ := NewServiceWithStageMaterials(store, refs, StageMaterialProfile)
	zero := "0"
	in := StageMaterialInput{RevisionInput: RevisionInput{CommandID: workItemAssignment, ExpectedRevision: 1}, MainFiles: []StageFileInput{{FileID: workItemPerson, Version: 1, SourceRevision: 1}, {FileID: workItemSubject, Version: 1, SourceRevision: 1}}, Score: &zero}
	return s, store, refs, in, Actor{TenantID: workItemTenant, ID: workItemSubject}
}
func TestStageMaterialServiceAtomicValidationReplayAndScope(t *testing.T) {
	ctx := context.Background()
	t.Run("all files validated one intent no owner required", func(t *testing.T) {
		s, store, refs, in, actor := stageFixture()
		if _, err := s.RegisterStageMaterials(ctx, actor, workItemID, "scheme", in); err != nil {
			t.Fatal(err)
		}
		if refs.calls != 2 || store.writes != 1 || *store.last.Score != "0" || store.last.MainFiles[0].DisplaySnapshot != "verified.pdf" || in.MainFiles[0].DisplaySnapshot != "" {
			t.Fatal("not atomic or snapshot forged")
		}
	})
	t.Run("second evidence unavailable zero store calls", func(t *testing.T) {
		s, store, refs, in, actor := stageFixture()
		refs.failAt = 2
		if _, err := s.RegisterStageMaterials(ctx, actor, workItemID, "scheme", in); !errors.Is(err, ErrReferenceUnavailable) || store.writes != 0 {
			t.Fatalf("%v writes=%d", err, store.writes)
		}
	})
	t.Run("cross tenant evidence zero calls", func(t *testing.T) {
		s, store, refs, in, actor := stageFixture()
		refs.wrongTenant = true
		if _, err := s.RegisterStageMaterials(ctx, actor, workItemID, "scheme", in); !errors.Is(err, ErrReferenceInvalid) || store.writes != 0 {
			t.Fatalf("%v", err)
		}
	})
	t.Run("replay before unavailable upstream", func(t *testing.T) {
		s, store, refs, in, actor := stageFixture()
		store.replay = true
		refs.failAt = 1
		if _, err := s.RegisterStageMaterials(ctx, actor, workItemID, "scheme", in); err != nil || refs.calls != 0 || store.writes != 0 {
			t.Fatal("replay used upstream or wrote")
		}
	})
	t.Run("last main required", func(t *testing.T) {
		s, store, _, in, actor := stageFixture()
		in.MainFiles = nil
		if _, err := s.RegisterStageMaterials(ctx, actor, workItemID, "scheme", in); !errors.Is(err, ErrCompletionGaps) || store.writes != 0 {
			t.Fatalf("%v", err)
		}
	})
	t.Run("wrong stage and tenant fail closed", func(t *testing.T) {
		s, store, _, in, actor := stageFixture()
		if _, err := s.RegisterStageMaterials(ctx, actor, workItemID, "preliminary-design", in); !errors.Is(err, ErrStageInvalid) {
			t.Fatalf("%v", err)
		}
		actor.TenantID = workItemPerson
		if _, err := s.RegisterStageMaterials(ctx, actor, workItemID, "scheme", in); !errors.Is(err, ErrNotFound) || store.writes != 0 {
			t.Fatalf("%v", err)
		}
	})
	t.Run("stale before upstream", func(t *testing.T) {
		s, store, refs, in, actor := stageFixture()
		in.ExpectedRevision = 2
		if _, err := s.RegisterStageMaterials(ctx, actor, workItemID, "scheme", in); !errors.Is(err, ErrRevisionConflict) || store.writes != 0 || refs.calls != 0 {
			t.Fatalf("%v", err)
		}
	})
}

func TestStageMaterialDefinitionUpgradeIsExplicitAndBounded(t *testing.T) {
	s, store, _, _, actor := stageFixture()
	input := DefinitionUpgradeInput{RevisionInput: RevisionInput{CommandID: workItemAssignment, ExpectedRevision: 1}, TargetVersion: 3}
	upgraded, err := s.UpgradeStageMaterialDefinition(context.Background(), actor, workItemID, input)
	if err != nil || store.upgrades != 1 || store.upgrade.CommandID != input.CommandID || store.upgrade.ExpectedRevision != input.ExpectedRevision || store.upgrade.TargetVersion != input.TargetVersion || upgraded.WorkItem.ID != workItemID || upgraded.WorkItem.TypeDefinitionVersion != 3 || upgraded.WorkItem.Revision != 2 {
		t.Fatalf("upgrade=%+v calls=%d input=%+v err=%v", upgraded.WorkItem, store.upgrades, store.upgrade, err)
	}
	input.TargetVersion = 7
	if _, err := s.UpgradeStageMaterialDefinition(context.Background(), actor, workItemID, input); err == nil || store.upgrades != 1 {
		t.Fatalf("future upgrade reached store: calls=%d err=%v", store.upgrades, err)
	}
}
