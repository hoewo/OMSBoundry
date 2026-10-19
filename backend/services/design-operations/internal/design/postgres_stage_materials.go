package design

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *PostgresStore) StageWorkItems(ctx context.Context, tenant, project, typeKey string) ([]WorkItem, error) {
	// Include incompatible legacy tasks so the caller cannot mistake them for
	// an empty original-OA collection or create parallel writable stage facts.
	rows, err := s.pool.Query(ctx, `SELECT id FROM design_work_items WHERE tenant_id=$1 AND project_id=$2 AND (type_key=$3 OR type_definition_version=1) AND status<>'cancelled' ORDER BY created_at,id LIMIT 101`, tenant, project, typeKey)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if len(ids) > 100 {
		return nil, fmt.Errorf("invalid stage work-item count")
	}
	items := make([]WorkItem, 0, len(ids))
	for _, id := range ids {
		item, err := loadItem(ctx, s.pool, tenant, id, "")
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *PostgresStore) StageMaterials(ctx context.Context, tenant, id, stage string) (StageMaterialCollection, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return StageMaterialCollection{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	result, err := loadStageMaterials(ctx, tx, tenant, id, stage)
	if err != nil {
		return StageMaterialCollection{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return StageMaterialCollection{}, err
	}
	return result, nil
}

func loadStageMaterials(ctx context.Context, q queryer, tenant, id, stage string) (StageMaterialCollection, error) {
	item, err := loadItem(ctx, q, tenant, id, "")
	if err != nil {
		return StageMaterialCollection{}, err
	}
	if !IsStageMaterialDefinitionVersion(item.TypeDefinitionVersion) {
		return StageMaterialCollection{}, ErrTypeInvalid
	}
	if _, ok := StageDefinitionFor(item.TypeKey, item.TypeDefinitionVersion, stage); !ok {
		return StageMaterialCollection{}, ErrStageInvalid
	}
	projectTypes, err := loadStageMaterialProjectTypes(ctx, q, item)
	if err != nil {
		return StageMaterialCollection{}, err
	}
	if applicable, err := stageMaterialApplicable(item.TypeDefinitionVersion, stage, projectTypes); err != nil || !applicable {
		if err != nil {
			return StageMaterialCollection{}, err
		}
		return StageMaterialCollection{}, ErrStageInvalid
	}
	st, err := loadStage(ctx, q, tenant, id, stage, "")
	if err != nil {
		return StageMaterialCollection{}, err
	}
	result := StageMaterialCollection{SchemaVersion: "1.0", Profile: StageMaterialProfile, WorkItem: item, Stage: st, ProjectTypeSnapshot: projectTypes, MainFiles: []StageMaterial{}, ReviewSheets: []StageMaterial{}}
	rows, err := q.Query(ctx, `SELECT m.id,m.tenant_id,m.work_item_id,m.stage_key,m.kind,m.file_id,m.file_version,m.source_revision,m.display_snapshot,m.created_revision,m.batch_position,m.created_at FROM production_stage_materials m WHERE m.tenant_id=$1 AND m.work_item_id=$2 AND m.stage_key=$3 AND NOT EXISTS(SELECT 1 FROM production_stage_material_endings e WHERE e.reference_id=m.id) ORDER BY m.created_revision,m.batch_position,m.id LIMIT 201`, tenant, id, stage)
	if err != nil {
		return StageMaterialCollection{}, err
	}
	for rows.Next() {
		var m StageMaterial
		if err := rows.Scan(&m.ID, &m.TenantID, &m.WorkItemID, &m.StageKey, &m.Kind, &m.FileID, &m.Version, &m.SourceRevision, &m.DisplaySnapshot, &m.CreatedRevision, &m.BatchPosition, &m.CreatedAt); err != nil {
			rows.Close()
			return StageMaterialCollection{}, err
		}
		if m.Kind == MaterialMain {
			result.MainFiles = append(result.MainFiles, m)
		} else {
			result.ReviewSheets = append(result.ReviewSheets, m)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return StageMaterialCollection{}, err
	}
	if len(result.MainFiles) > MaxStageMaterialCurrent || len(result.ReviewSheets) > MaxStageMaterialCurrent {
		return StageMaterialCollection{}, fmt.Errorf("invalid current material limit")
	}
	err = q.QueryRow(ctx, `SELECT score FROM production_stage_scores WHERE tenant_id=$1 AND work_item_id=$2 AND stage_key=$3 ORDER BY created_revision DESC LIMIT 1`, tenant, id, stage).Scan(&result.Score)
	if err == pgx.ErrNoRows {
		err = nil
	}
	return result, err
}

func (s *PostgresStore) UpgradeStageMaterialDefinition(ctx context.Context, actor Actor, id string, input DefinitionUpgradeInput, hash string) (Details, error) {
	tx, err := s.beginMutation(ctx, actor.TenantID, input.CommandID)
	if err != nil {
		return Details{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var replay Details
	if found, err := replayTx(ctx, tx, actor.TenantID, input.CommandID, hash, &replay); err != nil || found {
		return replay, err
	}
	item, err := loadItem(ctx, tx, actor.TenantID, id, "FOR UPDATE")
	if err != nil {
		return Details{}, err
	}
	if err := checkMutable(item, input.ExpectedRevision); err != nil {
		return Details{}, err
	}
	fromVersion := item.TypeDefinitionVersion
	if item.TypeKey != StageMaterialTypeKey || !IsStageMaterialDefinitionVersion(fromVersion) || input.TargetVersion != fromVersion+1 || input.TargetVersion > StageMaterialDefinitionVersion {
		return Details{}, ErrTypeInvalid
	}
	currentDefinition, ok := RegisteredTypeDefinition(StageMaterialTypeKey, fromVersion)
	if !ok {
		return Details{}, ErrTypeInvalid
	}
	definition, ok := RegisteredTypeDefinition(StageMaterialTypeKey, input.TargetVersion)
	if !ok {
		return Details{}, ErrTypeInvalid
	}
	now := time.Now().UTC()
	if input.TargetVersion == StageMaterialDefinitionVersion {
		if len(input.ProjectTypeSnapshot) == 0 {
			return Details{}, ErrTypeInvalid
		}
		for _, projectType := range input.ProjectTypeSnapshot {
			if _, err := tx.Exec(ctx, `INSERT INTO production_project_type_snapshots(tenant_id,work_item_id,project_id,project_source_revision,project_type_key,captured_at) VALUES($1,$2,$3,$4,$5,$6)`, actor.TenantID, id, item.ProjectID, item.ProjectSourceRevision, projectType, now); err != nil {
				return Details{}, mapWriteError(err)
			}
		}
	}
	item.Revision++
	item.TypeDefinitionVersion = input.TargetVersion
	item.UpdatedAt = now
	currentStages := map[string]bool{}
	for _, stage := range currentDefinition.Stages {
		currentStages[stage.Key] = true
	}
	addedStages := []string{}
	for _, stage := range definition.Stages {
		if currentStages[stage.Key] {
			continue
		}
		if _, err := tx.Exec(ctx, `INSERT INTO design_stages(tenant_id,work_item_id,stage_key,sequence,status,updated_at) VALUES($1,$2,$3,$4,'pending',$5) ON CONFLICT (tenant_id,work_item_id,stage_key) DO NOTHING`, actor.TenantID, id, stage.Key, stage.Sequence, now); err != nil {
			return Details{}, err
		}
		addedStages = append(addedStages, stage.Key)
	}
	if len(addedStages) == 0 || len(addedStages) != len(definition.Stages)-len(currentDefinition.Stages) {
		return Details{}, ErrTypeInvalid
	}
	if _, err := tx.Exec(ctx, `INSERT INTO production_definition_upgrades(tenant_id,work_item_id,from_version,to_version,upgraded_revision,command_id,actor_subject,upgraded_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, actor.TenantID, id, fromVersion, input.TargetVersion, item.Revision, input.CommandID, actor.ID, now); err != nil {
		return Details{}, mapWriteError(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE design_work_items SET type_definition_version=$3,revision=$4,updated_at=$5 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, id, item.TypeDefinitionVersion, item.Revision, now); err != nil {
		return Details{}, err
	}
	if err := projectStageMaterials(ctx, tx, &item); err != nil {
		return Details{}, err
	}
	if err := insertHistory(ctx, tx, item, actor, input.CommandID, "definition_upgraded", map[string]any{"from_version": fromVersion, "to_version": input.TargetVersion, "added_stages": addedStages, "project_type_snapshot": input.ProjectTypeSnapshot}); err != nil {
		return Details{}, err
	}
	return finishDetails(ctx, tx, actor.TenantID, id, input.CommandID, hash, now)
}

func (s *PostgresStore) RegisterStageMaterials(ctx context.Context, actor Actor, id, stage string, input StageMaterialInput, hash string) (StageMaterialCollection, error) {
	tx, err := s.beginMutation(ctx, actor.TenantID, input.CommandID)
	if err != nil {
		return StageMaterialCollection{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var replay StageMaterialCollection
	if found, err := replayTx(ctx, tx, actor.TenantID, input.CommandID, hash, &replay); err != nil || found {
		return replay, err
	}
	item, err := loadItem(ctx, tx, actor.TenantID, id, "FOR UPDATE")
	if err != nil {
		return StageMaterialCollection{}, err
	}
	if err := checkMutable(item, input.ExpectedRevision); err != nil {
		return StageMaterialCollection{}, err
	}
	current, err := loadStageMaterials(ctx, tx, actor.TenantID, id, stage)
	if err != nil {
		return StageMaterialCollection{}, err
	}
	if len(current.MainFiles)+len(input.MainFiles) == 0 {
		return StageMaterialCollection{}, ErrCompletionGaps
	}
	for i, files := range [][]StageFileInput{input.MainFiles, input.ReviewSheets} {
		active := current.MainFiles
		if i == 1 {
			active = current.ReviewSheets
		}
		if len(active)+len(files) > MaxStageMaterialCurrent {
			return StageMaterialCollection{}, fmt.Errorf("invalid current material limit")
		}
		for _, f := range files {
			for _, m := range active {
				if m.FileID == f.FileID && m.Version == f.Version {
					return StageMaterialCollection{}, ErrStateConflict
				}
			}
		}
	}
	now := time.Now().UTC()
	item.Revision++
	item.UpdatedAt = now
	if _, err := tx.Exec(ctx, `UPDATE design_work_items SET revision=$3,updated_at=$4 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, id, item.Revision, now); err != nil {
		return StageMaterialCollection{}, err
	}
	added := []StageMaterial{}
	for i, files := range [][]StageFileInput{input.MainFiles, input.ReviewSheets} {
		kind := MaterialMain
		if i == 1 {
			kind = MaterialReview
		}
		for position, f := range files {
			m := StageMaterial{ID: newUUID(), TenantID: actor.TenantID, WorkItemID: id, StageKey: stage, Kind: kind, FileID: f.FileID, Version: f.Version, SourceRevision: f.SourceRevision, DisplaySnapshot: f.DisplaySnapshot, CreatedRevision: item.Revision, BatchPosition: position + 1, CreatedAt: now}
			if _, err := tx.Exec(ctx, `INSERT INTO production_stage_materials(id,tenant_id,work_item_id,stage_key,kind,file_id,file_version,source_revision,display_snapshot,created_revision,batch_position,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, m.ID, m.TenantID, m.WorkItemID, m.StageKey, m.Kind, m.FileID, m.Version, m.SourceRevision, m.DisplaySnapshot, m.CreatedRevision, m.BatchPosition, now); err != nil {
				return StageMaterialCollection{}, err
			}
			added = append(added, m)
		}
	}
	if input.Score != nil {
		if _, err := tx.Exec(ctx, `INSERT INTO production_stage_scores(id,tenant_id,work_item_id,stage_key,score,created_revision,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, newUUID(), actor.TenantID, id, stage, *input.Score, item.Revision, now); err != nil {
			return StageMaterialCollection{}, err
		}
	}
	if err := projectStageMaterials(ctx, tx, &item); err != nil {
		return StageMaterialCollection{}, err
	}
	if err := insertHistory(ctx, tx, item, actor, input.CommandID, "stage_materials_registered", map[string]any{"stage_key": stage, "materials": added, "score": input.Score}); err != nil {
		return StageMaterialCollection{}, err
	}
	return finishStageMaterials(ctx, tx, item, stage, input.CommandID, hash)
}

func (s *PostgresStore) WithdrawStageMaterial(ctx context.Context, actor Actor, id, stage, refID string, input ReasonInput, hash string) (StageMaterialCollection, error) {
	tx, err := s.beginMutation(ctx, actor.TenantID, input.CommandID)
	if err != nil {
		return StageMaterialCollection{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var replay StageMaterialCollection
	if found, err := replayTx(ctx, tx, actor.TenantID, input.CommandID, hash, &replay); err != nil || found {
		return replay, err
	}
	item, err := loadItem(ctx, tx, actor.TenantID, id, "FOR UPDATE")
	if err != nil {
		return StageMaterialCollection{}, err
	}
	if !IsStageMaterialDefinitionVersion(item.TypeDefinitionVersion) {
		return StageMaterialCollection{}, ErrTypeInvalid
	}
	if err := checkMutable(item, input.ExpectedRevision); err != nil {
		return StageMaterialCollection{}, err
	}
	if _, ok := StageDefinitionFor(item.TypeKey, item.TypeDefinitionVersion, stage); !ok {
		return StageMaterialCollection{}, ErrStageInvalid
	}
	var active bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM production_stage_materials m WHERE m.tenant_id=$1 AND m.work_item_id=$2 AND m.stage_key=$3 AND m.id=$4 AND NOT EXISTS(SELECT 1 FROM production_stage_material_endings e WHERE e.reference_id=m.id))`, actor.TenantID, id, stage, refID).Scan(&active); err != nil {
		return StageMaterialCollection{}, err
	}
	if !active {
		return StageMaterialCollection{}, ErrNotFound
	}
	item.Revision++
	item.UpdatedAt = time.Now().UTC()
	if _, err := tx.Exec(ctx, `UPDATE design_work_items SET revision=$3,updated_at=$4 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, id, item.Revision, item.UpdatedAt); err != nil {
		return StageMaterialCollection{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO production_stage_material_endings(reference_id,tenant_id,work_item_id,stage_key,ended_revision,reason,ended_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, refID, actor.TenantID, id, stage, item.Revision, input.Reason, item.UpdatedAt); err != nil {
		return StageMaterialCollection{}, err
	}
	if err := projectStageMaterials(ctx, tx, &item); err != nil {
		return StageMaterialCollection{}, err
	}
	if err := insertHistory(ctx, tx, item, actor, input.CommandID, "stage_material_withdrawn", map[string]any{"stage_key": stage, "reference_id": refID, "reason": input.Reason}); err != nil {
		return StageMaterialCollection{}, err
	}
	return finishStageMaterials(ctx, tx, item, stage, input.CommandID, hash)
}

func projectStageMaterials(ctx context.Context, tx pgx.Tx, item *WorkItem) error {
	details, err := loadDetails(ctx, tx, item.TenantID, item.ID, false)
	if err != nil {
		return err
	}
	counts := map[string]int{}
	rows, err := tx.Query(ctx, `SELECT m.stage_key,count(*) FROM production_stage_materials m WHERE m.tenant_id=$1 AND m.work_item_id=$2 AND m.kind='main-file' AND NOT EXISTS(SELECT 1 FROM production_stage_material_endings e WHERE e.reference_id=m.id) GROUP BY m.stage_key`, item.TenantID, item.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var key string
		var n int
		if err := rows.Scan(&key, &n); err != nil {
			rows.Close()
			return err
		}
		counts[key] = n
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	projectTypes, err := loadStageMaterialProjectTypes(ctx, tx, *item)
	if err != nil {
		return err
	}
	projected, frontier, err := materialStageProjectionForProject(item.TypeKey, item.TypeDefinitionVersion, details.Stages, counts, projectTypes, item.Revision, item.UpdatedAt)
	if err != nil {
		return err
	}
	// Clear the old ready row first; the partial unique index remains valid in either direction.
	if _, err := tx.Exec(ctx, `UPDATE design_stages SET status='pending' WHERE tenant_id=$1 AND work_item_id=$2 AND status='ready'`, item.TenantID, item.ID); err != nil {
		return err
	}
	for _, st := range projected {
		if _, err := tx.Exec(ctx, `UPDATE design_stages SET status=$4,not_applicable_reason=$5,completed_at=$6,completed_work_item_revision=$7,updated_at=$8 WHERE tenant_id=$1 AND work_item_id=$2 AND stage_key=$3`, item.TenantID, item.ID, st.Key, st.Status, st.NotApplicableReason, st.CompletedAt, st.CompletedRevision, st.UpdatedAt); err != nil {
			return err
		}
	}
	item.CurrentStageKey = frontier
	item.Status = StatusInProgress
	if frontier == nil {
		item.Status = StatusCompleted
	}
	_, err = tx.Exec(ctx, `UPDATE design_work_items SET status=$3,current_stage_key=$4 WHERE tenant_id=$1 AND id=$2`, item.TenantID, item.ID, item.Status, item.CurrentStageKey)
	return err
}

func materialStageProjection(typeKey string, version int64, stages []Stage, counts map[string]int, revision int64, at time.Time) ([]Stage, *string, error) {
	return materialStageProjectionForProject(typeKey, version, stages, counts, nil, revision, at)
}

func materialStageProjectionForProject(typeKey string, version int64, stages []Stage, counts map[string]int, projectTypes []string, revision int64, at time.Time) ([]Stage, *string, error) {
	definition, ok := RegisteredTypeDefinition(typeKey, version)
	if !ok || len(definition.Stages) != len(stages) {
		return nil, nil, ErrTypeInvalid
	}
	policies := make(map[string]StageDefinition, len(definition.Stages))
	for _, stage := range definition.Stages {
		policies[stage.Key] = stage
	}
	result := append([]Stage(nil), stages...)
	var frontier *string
	for i := range result {
		st := &result[i]
		policy, ok := policies[st.Key]
		if !ok {
			return nil, nil, ErrTypeInvalid
		}
		applicable, err := stageMaterialApplicable(version, st.Key, projectTypes)
		if err != nil {
			return nil, nil, err
		}
		status := StageCompleted
		var reason *string
		if !applicable {
			status = StageNotApplicable
			text := "project-type-snapshot"
			reason = &text
		}
		if applicable && counts[st.Key] == 0 {
			status = StagePending
			if !policy.Skippable && frontier == nil {
				key := st.Key
				frontier = &key
				status = StageReady
			}
		}
		if st.Status != status {
			st.Status = status
			st.UpdatedAt = at
			st.CompletedAt = nil
			st.CompletedRevision = nil
			if status == StageCompleted {
				stamp := at
				rev := revision
				st.CompletedAt = &stamp
				st.CompletedRevision = &rev
			}
		}
		st.NotApplicableReason = reason
	}
	return result, frontier, nil
}

func loadStageMaterialProjectTypes(ctx context.Context, q queryer, item WorkItem) ([]string, error) {
	if item.TypeDefinitionVersion < StageMaterialDefinitionVersion {
		return nil, nil
	}
	rows, err := q.Query(ctx, `SELECT project_type_key FROM production_project_type_snapshots WHERE tenant_id=$1 AND work_item_id=$2 ORDER BY project_type_key`, item.TenantID, item.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []string{}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return normalizeStageMaterialProjectTypes(result)
}

func finishStageMaterials(ctx context.Context, tx pgx.Tx, item WorkItem, stage, commandID, hash string) (StageMaterialCollection, error) {
	result, err := loadStageMaterials(ctx, tx, item.TenantID, item.ID, stage)
	if err != nil {
		return StageMaterialCollection{}, err
	}
	if err := recordCommand(ctx, tx, item.TenantID, commandID, hash, 200, result, item.UpdatedAt); err != nil {
		return StageMaterialCollection{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return StageMaterialCollection{}, err
	}
	return result, nil
}
