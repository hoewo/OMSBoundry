//go:build integration

package design

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"omsbizfoundry/design-operations/internal/database"
	"omsbizfoundry/design-operations/internal/testdatabase"
)

func materialDBFixture(t *testing.T) (*Service, *PostgresStore, *stableReferences, Actor, string, testdatabase.Environment) {
	t.Helper()
	ctx := context.Background()
	env := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, env.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(env.Pool)
	refs := &stableReferences{}
	s, _ := NewServiceWithStageMaterials(store, refs, StageMaterialProfile)
	actor := Actor{ID: intActor, TenantID: intTenant, Kind: "test_subject"}
	created, err := s.Create(ctx, actor, CreateInput{CommandID: command(300), ProjectID: intProject, ProjectSourceRevision: 1, TypeKey: StageMaterialTypeKey, TypeDefinitionVersion: 2})
	if err != nil {
		t.Fatal(err)
	}
	return s, store, refs, actor, created.WorkItem.ID, env
}
func stageFile(n int) StageFileInput {
	return StageFileInput{FileID: fmt.Sprintf("aaaaaaaa-aaaa-4aaa-8aaa-%012d", n), Version: 1, SourceRevision: 1}
}
func materialInput(n int, revision int64, main int, score *string) StageMaterialInput {
	in := StageMaterialInput{RevisionInput: RevisionInput{CommandID: command(n), ExpectedRevision: revision}, MainFiles: []StageFileInput{}, ReviewSheets: []StageFileInput{}, Score: score}
	if main > 0 {
		in.MainFiles = append(in.MainFiles, stageFile(main))
	}
	return in
}

func TestStageMaterialV6FreezesProjectTypesAndScopesTestReport(t *testing.T) {
	ctx := context.Background()
	env := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, env.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(env.Pool)
	refs := &stableReferences{projectTypeKeys: []string{"survey"}}
	service, _ := NewServiceWithStageMaterials(store, refs, StageMaterialProfile)
	actor := Actor{ID: intActor, TenantID: intTenant, Kind: "test_subject"}
	created, err := service.Create(ctx, actor, CreateInput{CommandID: command(280), ProjectID: intProject, ProjectSourceRevision: 9, TypeKey: StageMaterialTypeKey, TypeDefinitionVersion: 6})
	if err != nil || created.WorkItem.TypeDefinitionVersion != 6 || len(created.Stages) != 7 {
		t.Fatalf("create=%+v err=%v", created.WorkItem, err)
	}
	status := map[string]string{}
	for _, stage := range created.Stages {
		status[stage.Key] = stage.Status
	}
	if status["scheme"] != StageReady || status["test"] != StagePending || status["report"] != StagePending || status["preliminary-design"] != StageNotApplicable || status["completion"] != StageNotApplicable {
		t.Fatalf("survey stage applicability: %+v", status)
	}
	var snapshotCount int
	if err := env.Pool.QueryRow(ctx, `SELECT count(*) FROM production_project_type_snapshots WHERE tenant_id=$1 AND work_item_id=$2 AND project_id=$3 AND project_source_revision=9 AND project_type_key='survey'`, intTenant, created.WorkItem.ID, intProject).Scan(&snapshotCount); err != nil || snapshotCount != 1 {
		t.Fatalf("snapshot count=%d err=%v", snapshotCount, err)
	}
	if _, err := store.StageMaterials(ctx, intTenant, created.WorkItem.ID, "preliminary-design"); !errors.Is(err, ErrStageInvalid) {
		t.Fatalf("inapplicable stage read=%v", err)
	}
	refs.unavailable = true
	if _, err := service.StageMaterials(ctx, intTenant, created.WorkItem.ID, "test"); err != nil {
		t.Fatalf("frozen snapshot read used Project runtime: %v", err)
	}
	refs.unavailable = false
	testFiles, err := service.RegisterStageMaterials(ctx, actor, created.WorkItem.ID, "test", materialInput(281, 1, 81, nil))
	if err != nil || testFiles.Stage.Status != StageCompleted || testFiles.WorkItem.Revision != 2 {
		t.Fatalf("test files=%+v err=%v", testFiles.Stage, err)
	}
	reportFiles, err := service.RegisterStageMaterials(ctx, actor, created.WorkItem.ID, "report", materialInput(282, 2, 82, nil))
	if err != nil || reportFiles.Stage.Status != StageCompleted || reportFiles.WorkItem.Revision != 3 {
		t.Fatalf("report files=%+v err=%v", reportFiles.Stage, err)
	}
	if _, err := env.Pool.Exec(ctx, `DELETE FROM production_project_type_snapshots WHERE tenant_id=$1 AND work_item_id=$2`, intTenant, created.WorkItem.ID); err == nil {
		t.Fatal("immutable project type snapshot was deleted")
	}
}

func TestStageMaterialPostgresBatchOrderSurvivesReadAndReplay(t *testing.T) {
	ctx := context.Background()
	s, store, _, actor, id, _ := materialDBFixture(t)
	in := materialInput(390, 1, 0, nil)
	in.MainFiles = []StageFileInput{stageFile(90), stageFile(80)}
	in.ReviewSheets = []StageFileInput{stageFile(70), stageFile(60)}
	first, err := s.RegisterStageMaterials(ctx, actor, id, "scheme", in)
	if err != nil {
		t.Fatal(err)
	}
	check := func(rows []StageMaterial, files []StageFileInput, revision int64) {
		t.Helper()
		if len(rows) != len(files) {
			t.Fatalf("count %d", len(rows))
		}
		for i, row := range rows {
			if row.FileID != files[i].FileID || row.CreatedRevision != revision || row.BatchPosition != i+1 {
				t.Fatalf("unstable batch order: %+v", rows)
			}
		}
	}
	check(first.MainFiles, in.MainFiles, 2)
	check(first.ReviewSheets, in.ReviewSheets, 2)
	second := materialInput(391, 2, 0, nil)
	second.ReviewSheets = []StageFileInput{stageFile(50), stageFile(40)}
	if _, err := s.RegisterStageMaterials(ctx, actor, id, "scheme", second); err != nil {
		t.Fatal(err)
	}
	current, err := store.StageMaterials(ctx, intTenant, id, "scheme")
	if err != nil {
		t.Fatal(err)
	}
	check(current.MainFiles, in.MainFiles, 2)
	if len(current.ReviewSheets) != 4 {
		t.Fatal("review count")
	}
	check(current.ReviewSheets[:2], in.ReviewSheets, 2)
	check(current.ReviewSheets[2:], second.ReviewSheets, 3)
	replay, err := s.RegisterStageMaterials(ctx, actor, id, "scheme", in)
	if err != nil {
		t.Fatal(err)
	}
	check(replay.MainFiles, in.MainFiles, 2)
	check(replay.ReviewSheets, in.ReviewSheets, 2)
}

func TestStageMaterialPostgresAtomicCollectionsReplayAndHistory(t *testing.T) {
	ctx := context.Background()
	s, store, refs, actor, id, env := materialDBFixture(t)
	zero := "0"
	in := materialInput(301, 1, 1, &zero)
	in.MainFiles = append(in.MainFiles, stageFile(2))
	in.ReviewSheets = []StageFileInput{stageFile(3)}
	current, err := s.RegisterStageMaterials(ctx, actor, id, "scheme", in)
	if err != nil {
		t.Fatal(err)
	}
	if current.WorkItem.Revision != 2 || len(current.MainFiles) != 2 || len(current.ReviewSheets) != 1 || current.Score == nil || *current.Score != "0" || current.Stage.Status != StageCompleted || current.WorkItem.OwnerAssignmentID != nil || current.WorkItem.CurrentStageKey != nil {
		t.Fatalf("%+v", current)
	}
	refs.unavailable = true
	replay, err := s.RegisterStageMaterials(ctx, actor, id, "scheme", in)
	if err != nil || replay.WorkItem.Revision != 2 || replay.MainFiles[0].ID != current.MainFiles[0].ID {
		t.Fatalf("replay %+v %v", replay, err)
	}
	if _, err := s.RegisterStageMaterials(ctx, actor, id, "scheme", materialInput(302, 2, 4, nil)); !errors.Is(err, ErrReferenceUnavailable) {
		t.Fatal(err)
	}
	unchanged, _ := store.StageMaterials(ctx, intTenant, id, "scheme")
	if unchanged.WorkItem.Revision != 2 || len(unchanged.MainFiles) != 2 {
		t.Fatal("upstream failure partial commit")
	}
	refs.unavailable = false
	changed := in
	one := "1"
	changed.Score = &one
	if _, err := s.RegisterStageMaterials(ctx, actor, id, "scheme", changed); !errors.Is(err, ErrCommandConflict) {
		t.Fatal("payload conflict", err)
	}
	if _, err := s.RegisterStageMaterials(ctx, actor, id, "scheme", materialInput(303, 2, 1, nil)); !errors.Is(err, ErrStateConflict) {
		t.Fatal("active duplicate", err)
	}
	score := "88.12345678901234567890123456789"
	current, err = s.RegisterStageMaterials(ctx, actor, id, "scheme", materialInput(304, 2, 0, &score))
	if err != nil || *current.Score != score {
		t.Fatal("precision", err)
	}
	blank := ""
	review := materialInput(305, 3, 0, &blank)
	review.ReviewSheets = []StageFileInput{stageFile(5)}
	current, err = s.RegisterStageMaterials(ctx, actor, id, "scheme", review)
	if err != nil || *current.Score != score || len(current.ReviewSheets) != 2 {
		t.Fatal("blank preserve", err)
	}
	withdraw := ReasonInput{RevisionInput: RevisionInput{CommandID: command(306), ExpectedRevision: 4}, Reason: "原 OA 移出"}
	target := current.MainFiles[0].ID
	current, err = s.WithdrawStageMaterial(ctx, actor, id, "scheme", target, withdraw)
	if err != nil || len(current.MainFiles) != 1 || current.Stage.Status != StageCompleted {
		t.Fatal("withdraw", err)
	}
	afterFirst := current.Stage.CompletedAt
	current, err = s.WithdrawStageMaterial(ctx, actor, id, "scheme", current.MainFiles[0].ID, ReasonInput{RevisionInput: RevisionInput{CommandID: command(307), ExpectedRevision: 5}, Reason: "结束最后主文件"})
	if err != nil || len(current.MainFiles) != 0 || len(current.ReviewSheets) != 2 || *current.Score != score || current.Stage.Status != StageReady || *current.WorkItem.CurrentStageKey != "scheme" {
		t.Fatal("last main", err)
	}
	again, err := s.WithdrawStageMaterial(ctx, actor, id, "scheme", target, withdraw)
	if err != nil || again.WorkItem.Revision != 5 {
		t.Fatal("withdraw original replay", err)
	}
	current, err = s.RegisterStageMaterials(ctx, actor, id, "scheme", materialInput(308, 6, 1, nil))
	if err != nil || len(current.MainFiles) != 1 || current.MainFiles[0].ID == target || current.Stage.CompletedAt == nil || !current.Stage.CompletedAt.After(*afterFirst) {
		t.Fatal("re-add ended identity", err)
	}
	if _, err := s.ReferenceMaterials(ctx, intTenant, id); !errors.Is(err, ErrTypeInvalid) {
		t.Fatal("legacy material projection", err)
	}
	if _, err := s.PutDeliverable(ctx, actor, id, "scheme", DeliverableInput{CommandID: command(309), ExpectedRevision: 7, RequirementKey: RequirementPrimary, EvidenceFileID: stageFile(8).FileID, EvidenceVersion: 1, EvidenceSourceRevision: 1}); !errors.Is(err, ErrTypeInvalid) {
		t.Fatal("legacy slot allowed", err)
	}
	if _, err := s.CompleteStage(ctx, actor, id, "scheme", RevisionInput{CommandID: command(310), ExpectedRevision: 7}); !errors.Is(err, ErrTypeInvalid) {
		t.Fatal("legacy completion allowed", err)
	}
	history, err := s.StageMaterialHistory(ctx, intTenant, id, 0, 50)
	if err != nil || len(history.Items) != 7 {
		t.Fatalf("history %d %v", len(history.Items), err)
	}
	for _, query := range []string{`UPDATE production_stage_materials SET display_snapshot='changed' WHERE work_item_id=$1`, `DELETE FROM production_stage_materials WHERE work_item_id=$1`, `DELETE FROM production_stage_material_endings WHERE work_item_id=$1`, `UPDATE production_stage_scores SET score='1' WHERE work_item_id=$1`, `UPDATE design_work_items SET type_definition_version=1 WHERE id=$1`} {
		if _, err := env.Pool.Exec(ctx, query, id); err == nil {
			t.Fatal("immutable mutation accepted", query)
		}
	}
	var commands, materials, endings, scores int
	if err := env.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM design_commands),(SELECT count(*) FROM production_stage_materials),(SELECT count(*) FROM production_stage_material_endings),(SELECT count(*) FROM production_stage_scores)`).Scan(&commands, &materials, &endings, &scores); err != nil {
		t.Fatal(err)
	}
	if commands != 7 || materials != 5 || endings != 2 || scores != 2 {
		t.Fatalf("facts %d %d %d %d", commands, materials, endings, scores)
	}
}

func TestStageMaterialPostgresConcurrentCommandsAndIsolation(t *testing.T) {
	ctx := context.Background()
	s, store, _, actor, id, _ := materialDBFixture(t)
	in := materialInput(320, 1, 10, nil)
	var wg sync.WaitGroup
	errorsOut := make(chan error, 2)
	ids := make(chan string, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := s.RegisterStageMaterials(ctx, actor, id, "scheme", in)
			errorsOut <- err
			if err == nil {
				ids <- v.MainFiles[0].ID
			}
		}()
	}
	wg.Wait()
	close(errorsOut)
	close(ids)
	for err := range errorsOut {
		if err != nil {
			t.Fatal("same command", err)
		}
	}
	first := ""
	for v := range ids {
		if first != "" && first != v {
			t.Fatal("same command duplicated")
		}
		first = v
	}
	errorsOut = make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := s.RegisterStageMaterials(ctx, actor, id, "scheme", materialInput(321+i, 2, 11+i, nil))
			errorsOut <- err
		}(i)
	}
	wg.Wait()
	close(errorsOut)
	successes, conflicts := 0, 0
	for err := range errorsOut {
		if err == nil {
			successes++
		} else if errors.Is(err, ErrRevisionConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("race %d/%d", successes, conflicts)
	}
	current, _ := store.StageMaterials(ctx, intTenant, id, "scheme")
	if len(current.MainFiles) != 2 || current.WorkItem.Revision != 3 {
		t.Fatal("race facts")
	}
	if _, err := s.WithdrawStageMaterial(ctx, actor, id, "preliminary-design", current.MainFiles[0].ID, ReasonInput{RevisionInput: RevisionInput{CommandID: command(330), ExpectedRevision: 3}, Reason: "wrong stage"}); !errors.Is(err, ErrStageInvalid) {
		t.Fatal("wrong stage", err)
	}
	wrong := actor
	wrong.TenantID = stageFile(99).FileID
	if _, err := s.StageMaterials(ctx, wrong.TenantID, id, "scheme"); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross tenant", err)
	}
	if _, err := s.StageMaterials(ctx, intTenant, id, "report"); !errors.Is(err, ErrStageInvalid) {
		t.Fatal("cross type stage", err)
	}
	if _, err := s.Create(ctx, actor, CreateInput{CommandID: command(331), ProjectID: intProject, ProjectSourceRevision: 1, TypeKey: DesignTypeKey}); !errors.Is(err, ErrActiveConflict) {
		t.Fatal("v1 parallel truth", err)
	}
	items, err := s.StageWorkItems(ctx, intTenant, intProject, StageMaterialTypeKey)
	if err != nil || len(items) != 1 || items[0].ID != id {
		t.Fatal("exact project list", err)
	}
}

func TestStageMaterialPostgresProjectionAllStagesAndScoreDoesNotComplete(t *testing.T) {
	ctx := context.Background()
	s, store, _, actor, id, _ := materialDBFixture(t)
	score := "0"
	if _, err := s.RegisterStageMaterials(ctx, actor, id, "scheme", materialInput(340, 1, 0, &score)); !errors.Is(err, ErrCompletionGaps) {
		t.Fatal("score completion", err)
	}
	revision := int64(1)
	for i, key := range []string{"scheme"} {
		v, err := s.RegisterStageMaterials(ctx, actor, id, key, materialInput(341+i, revision, 30+i, nil))
		if err != nil {
			t.Fatal(err)
		}
		revision = v.WorkItem.Revision
	}
	current, _ := store.StageMaterials(ctx, intTenant, id, "scheme")
	if current.WorkItem.Status != StatusCompleted || current.WorkItem.CurrentStageKey != nil {
		t.Fatal("all stages projection")
	}
	prior := *current.Stage.CompletedAt
	time.Sleep(time.Millisecond)
	current, err := s.RegisterStageMaterials(ctx, actor, id, "scheme", materialInput(345, revision, 0, &score))
	if err != nil || !current.Stage.CompletedAt.Equal(prior) {
		t.Fatal("score changed completion time", err)
	}
	current, err = s.WithdrawStageMaterial(ctx, actor, id, "scheme", current.MainFiles[0].ID, ReasonInput{RevisionInput: RevisionInput{CommandID: command(346), ExpectedRevision: current.WorkItem.Revision}, Reason: "恢复缺口"})
	if err != nil || current.WorkItem.Status != StatusInProgress || *current.WorkItem.CurrentStageKey != "scheme" {
		t.Fatal("reopen via material", err)
	}
}

func TestStageMaterialDefinitionSequentialUpgradesPreserveEarlierStages(t *testing.T) {
	ctx := context.Background()
	s, store, _, actor, id, env := materialDBFixture(t)
	scheme, err := s.RegisterStageMaterials(ctx, actor, id, "scheme", materialInput(370, 1, 70, nil))
	if err != nil || scheme.WorkItem.Revision != 2 || scheme.WorkItem.TypeDefinitionVersion != StageMaterialDefinitionVersionV2 || scheme.Stage.Status != StageCompleted {
		t.Fatalf("scheme=%+v err=%v", scheme, err)
	}
	up := DefinitionUpgradeInput{RevisionInput: RevisionInput{CommandID: command(371), ExpectedRevision: 2}, TargetVersion: StageMaterialDefinitionVersionV3}
	upgraded, err := s.UpgradeStageMaterialDefinition(ctx, actor, id, up)
	if err != nil || upgraded.WorkItem.ID != id || upgraded.WorkItem.TypeDefinitionVersion != 3 || upgraded.WorkItem.Revision != 3 || len(upgraded.Stages) != 2 {
		t.Fatalf("upgrade=%+v err=%v", upgraded, err)
	}
	if upgraded.Stages[0].Key != "scheme" || upgraded.Stages[0].Status != StageCompleted || upgraded.Stages[1].Key != "preliminary-design" || upgraded.Stages[1].Status != StageReady || upgraded.WorkItem.CurrentStageKey == nil || *upgraded.WorkItem.CurrentStageKey != "preliminary-design" {
		t.Fatalf("upgrade projection=%+v", upgraded)
	}
	replay, err := s.UpgradeStageMaterialDefinition(ctx, actor, id, up)
	if err != nil || replay.WorkItem.Revision != 3 || replay.WorkItem.TypeDefinitionVersion != 3 {
		t.Fatalf("upgrade replay=%+v err=%v", replay, err)
	}
	preliminary, err := store.StageMaterials(ctx, intTenant, id, "preliminary-design")
	if err != nil || preliminary.WorkItem.Revision != 3 || len(preliminary.MainFiles) != 0 || preliminary.Stage.Status != StageReady {
		t.Fatalf("preliminary=%+v err=%v", preliminary, err)
	}
	preliminary, err = s.RegisterStageMaterials(ctx, actor, id, "preliminary-design", materialInput(372, 3, 71, nil))
	if err != nil || preliminary.WorkItem.Revision != 4 || preliminary.WorkItem.Status != StatusCompleted || preliminary.WorkItem.CurrentStageKey != nil || preliminary.Stage.Status != StageCompleted {
		t.Fatalf("preliminary saved=%+v err=%v", preliminary, err)
	}
	schemeAfter, err := store.StageMaterials(ctx, intTenant, id, "scheme")
	if err != nil || len(schemeAfter.MainFiles) != 1 || schemeAfter.MainFiles[0].FileID != stageFile(70).FileID || schemeAfter.Stage.Status != StageCompleted {
		t.Fatalf("scheme changed=%+v err=%v", schemeAfter, err)
	}
	constructionUpgrade := DefinitionUpgradeInput{RevisionInput: RevisionInput{CommandID: command(373), ExpectedRevision: 4}, TargetVersion: StageMaterialDefinitionVersionV4}
	upgraded, err = s.UpgradeStageMaterialDefinition(ctx, actor, id, constructionUpgrade)
	if err != nil || upgraded.WorkItem.ID != id || upgraded.WorkItem.TypeDefinitionVersion != 4 || upgraded.WorkItem.Revision != 5 || len(upgraded.Stages) != 3 {
		t.Fatalf("construction upgrade=%+v err=%v", upgraded, err)
	}
	if upgraded.Stages[0].Status != StageCompleted || upgraded.Stages[1].Status != StageCompleted || upgraded.Stages[2].Key != "construction-drawing" || upgraded.Stages[2].Status != StageReady || upgraded.WorkItem.CurrentStageKey == nil || *upgraded.WorkItem.CurrentStageKey != "construction-drawing" {
		t.Fatalf("construction upgrade projection=%+v", upgraded)
	}
	replay, err = s.UpgradeStageMaterialDefinition(ctx, actor, id, constructionUpgrade)
	if err != nil || replay.WorkItem.Revision != 5 || replay.WorkItem.TypeDefinitionVersion != 4 {
		t.Fatalf("construction upgrade replay=%+v err=%v", replay, err)
	}
	construction, err := s.RegisterStageMaterials(ctx, actor, id, "construction-drawing", materialInput(374, 5, 72, nil))
	if err != nil || construction.WorkItem.Revision != 6 || construction.WorkItem.Status != StatusCompleted || construction.WorkItem.CurrentStageKey != nil || construction.Stage.Status != StageCompleted {
		t.Fatalf("construction saved=%+v err=%v", construction, err)
	}
	preliminaryAfter, err := store.StageMaterials(ctx, intTenant, id, "preliminary-design")
	if err != nil || len(preliminaryAfter.MainFiles) != 1 || preliminaryAfter.MainFiles[0].FileID != stageFile(71).FileID || preliminaryAfter.Stage.Status != StageCompleted {
		t.Fatalf("preliminary changed=%+v err=%v", preliminaryAfter, err)
	}
	auxiliaryUpgrade := DefinitionUpgradeInput{RevisionInput: RevisionInput{CommandID: command(375), ExpectedRevision: 6}, TargetVersion: StageMaterialDefinitionVersion}
	upgraded, err = s.UpgradeStageMaterialDefinition(ctx, actor, id, auxiliaryUpgrade)
	if err != nil || upgraded.WorkItem.ID != id || upgraded.WorkItem.TypeDefinitionVersion != 5 || upgraded.WorkItem.Revision != 7 || len(upgraded.Stages) != 5 {
		t.Fatalf("auxiliary upgrade=%+v err=%v", upgraded, err)
	}
	if upgraded.Stages[2].Status != StageCompleted || upgraded.Stages[3].Key != "change" || upgraded.Stages[3].Status != StagePending || upgraded.Stages[4].Key != "completion" || upgraded.Stages[4].Status != StageReady || upgraded.WorkItem.CurrentStageKey == nil || *upgraded.WorkItem.CurrentStageKey != "completion" {
		t.Fatalf("auxiliary upgrade projection=%+v", upgraded)
	}
	completion, err := s.RegisterStageMaterials(ctx, actor, id, "completion", materialInput(376, 7, 73, nil))
	if err != nil || completion.WorkItem.Revision != 8 || completion.WorkItem.Status != StatusCompleted || completion.WorkItem.CurrentStageKey != nil || completion.Stage.Status != StageCompleted {
		t.Fatalf("completion saved=%+v err=%v", completion, err)
	}
	change, err := s.RegisterStageMaterials(ctx, actor, id, "change", materialInput(377, 8, 74, nil))
	if err != nil || change.WorkItem.Revision != 9 || change.WorkItem.Status != StatusCompleted || change.WorkItem.CurrentStageKey != nil || change.Stage.Status != StageCompleted {
		t.Fatalf("optional change saved=%+v err=%v", change, err)
	}
	completionAfter, err := store.StageMaterials(ctx, intTenant, id, "completion")
	if err != nil || len(completionAfter.MainFiles) != 1 || completionAfter.MainFiles[0].FileID != stageFile(73).FileID || completionAfter.Stage.Status != StageCompleted {
		t.Fatalf("completion changed=%+v err=%v", completionAfter, err)
	}
	var upgrades, events int
	if err := env.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM production_definition_upgrades WHERE tenant_id=$1 AND work_item_id=$2),(SELECT count(*) FROM design_history_events WHERE tenant_id=$1 AND work_item_id=$2 AND event_type='definition_upgraded')`, intTenant, id).Scan(&upgrades, &events); err != nil || upgrades != 3 || events != 3 {
		t.Fatalf("upgrade audit=%d/%d err=%v", upgrades, events, err)
	}
	if _, err := env.Pool.Exec(ctx, `UPDATE design_work_items SET type_definition_version=2,revision=revision+1 WHERE tenant_id=$1 AND id=$2`, intTenant, id); err == nil {
		t.Fatal("definition downgrade accepted")
	}
}

func TestSharedSchemeProjectTypesAndLegacyExclusionBothDirections(t *testing.T) {
	ctx := context.Background()
	s, store, refs, actor, id, _ := materialDBFixture(t)
	for _, key := range []string{"design", "consultation", "survey", "mapping"} {
		refs.projectTypeKeys = []string{key}
		rows, err := s.StageWorkItems(ctx, intTenant, intProject, StageMaterialTypeKey)
		if err != nil || len(rows) != 1 || rows[0].ID != id {
			t.Fatal("shared identity changed with type", key, err)
		}
	}
	if _, err := s.StageWorkItems(ctx, intTenant, intProject, ConsultationTypeKey); !errors.Is(err, ErrTypeInvalid) {
		t.Fatal("per-type collection accepted", err)
	}
	otherProject := stageFile(201).FileID
	refs.projectTypeKeys = []string{"design", "consultation"}
	old, err := s.Create(ctx, actor, CreateInput{CommandID: command(360), ProjectID: otherProject, ProjectSourceRevision: 1, TypeKey: DesignTypeKey})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, actor, CreateInput{CommandID: command(361), ProjectID: otherProject, ProjectSourceRevision: 1, TypeKey: StageMaterialTypeKey, TypeDefinitionVersion: 2}); !errors.Is(err, ErrActiveConflict) {
		t.Fatal("shared alongside old truth", err)
	}
	rows, err := store.StageWorkItems(ctx, intTenant, otherProject, StageMaterialTypeKey)
	if err != nil || len(rows) != 1 || rows[0].ID != old.WorkItem.ID || rows[0].TypeDefinitionVersion != 1 {
		t.Fatal("legacy is not empty", err)
	}
	for _, key := range []string{"design", "consultation", "survey", "mapping"} {
		refs.projectTypeKeys = []string{key}
		pid := newUUID()
		fresh, err := s.Create(ctx, actor, CreateInput{CommandID: newUUID(), ProjectID: pid, ProjectSourceRevision: 1, TypeKey: StageMaterialTypeKey, TypeDefinitionVersion: 2})
		if err != nil || fresh.WorkItem.TypeKey != StageMaterialTypeKey || len(fresh.Stages) != 1 {
			t.Fatal("original shared scheme not available", key, err)
		}
	}
}

func TestSharedSchemeConcurrentCreationSerializesAgainstLegacy(t *testing.T) {
	ctx := context.Background()
	s, _, refs, actor, _, _ := materialDBFixture(t)
	refs.projectTypeKeys = []string{"design", "consultation"}
	for _, keys := range [][]string{{StageMaterialTypeKey, StageMaterialTypeKey}, {StageMaterialTypeKey, DesignTypeKey}, {ConsultationTypeKey, StageMaterialTypeKey}} {
		pid := newUUID()
		out := make(chan error, 2)
		var wg sync.WaitGroup
		for _, key := range keys {
			wg.Add(1)
			go func(key string) {
				defer wg.Done()
				version := int64(0)
				if key == StageMaterialTypeKey {
					version = 2
				}
				_, err := s.Create(ctx, actor, CreateInput{CommandID: newUUID(), ProjectID: pid, ProjectSourceRevision: 1, TypeKey: key, TypeDefinitionVersion: version})
				out <- err
			}(key)
		}
		wg.Wait()
		close(out)
		success, conflict := 0, 0
		for err := range out {
			if err == nil {
				success++
			} else if errors.Is(err, ErrActiveConflict) {
				conflict++
			} else {
				t.Fatal("unexpected create race", err)
			}
		}
		if success != 1 || conflict != 1 {
			t.Fatal("parallel truth", keys, success, conflict)
		}
	}
}
