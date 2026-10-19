package design

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) (*PostgresStore, error) {
	if pool == nil {
		return nil, fmt.Errorf("database pool is required")
	}
	return &PostgresStore{pool}, nil
}
func (s *PostgresStore) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

func (s *PostgresStore) List(ctx context.Context, tenantID string, limit int) ([]WorkItem, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,tenant_id,project_id,project_source_revision,type_key,type_definition_version,status,revision,owner_assignment_id,owner_source_revision,current_stage_key,cancellation_reason,created_by,created_at,updated_at FROM design_work_items WHERE tenant_id=$1 ORDER BY updated_at DESC,id DESC LIMIT $2`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]WorkItem, 0, limit)
	for rows.Next() {
		var item WorkItem
		if err := rows.Scan(&item.ID, &item.TenantID, &item.ProjectID, &item.ProjectSourceRevision, &item.TypeKey, &item.TypeDefinitionVersion, &item.Status, &item.Revision, &item.OwnerAssignmentID, &item.OwnerSourceRevision, &item.CurrentStageKey, &item.CancellationReason, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *PostgresStore) Replay(ctx context.Context, tenant, commandID, hash string, dest any) (bool, error) {
	var stored string
	var raw []byte
	err := s.pool.QueryRow(ctx, `SELECT request_hash,response_json FROM design_commands WHERE tenant_id=$1 AND command_id=$2`, tenant, commandID).Scan(&stored, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if stored != hash {
		return false, ErrCommandConflict
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return false, err
	}
	return true, nil
}

func (s *PostgresStore) Create(ctx context.Context, actor Actor, input CreateInput, hash string) (Details, error) {
	typeKey := input.TypeKey
	if typeKey == "" {
		typeKey = DesignTypeKey
	}
	input.TypeKey = typeKey
	definition, ok := createDefinition(input)
	if !ok || len(definition.Stages) == 0 {
		return Details{}, ErrTypeInvalid
	}
	if definition.Key == StageMaterialTypeKey && definition.Version == StageMaterialDefinitionVersion && len(input.ProjectTypeSnapshot) == 0 {
		return Details{}, ErrTypeInvalid
	}
	input.TypeKey = definition.Key
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Details{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lockCommand(ctx, tx, actor.TenantID, input.CommandID); err != nil {
		return Details{}, err
	}
	var replay Details
	if found, err := replayTx(ctx, tx, actor.TenantID, input.CommandID, hash, &replay); err != nil || found {
		return replay, err
	}
	now := time.Now().UTC()
	id := newUUID()
	first := definition.Stages[0].Key
	item := WorkItem{ID: id, TenantID: actor.TenantID, ProjectID: input.ProjectID, ProjectSourceRevision: input.ProjectSourceRevision, TypeKey: definition.Key, TypeDefinitionVersion: definition.Version, Status: StatusDraft, Revision: 1, CurrentStageKey: &first, CreatedBy: actor.ID, CreatedAt: now, UpdatedAt: now}
	_, err = tx.Exec(ctx, `INSERT INTO design_work_items(id,tenant_id,project_id,project_source_revision,type_key,type_definition_version,status,revision,current_stage_key,created_by,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,1,$8,$9,$10,$10)`, id, actor.TenantID, input.ProjectID, input.ProjectSourceRevision, definition.Key, definition.Version, StatusDraft, first, actor.ID, now)
	if err != nil {
		return Details{}, mapWriteError(err)
	}
	if definition.Key == StageMaterialTypeKey && definition.Version == StageMaterialDefinitionVersion {
		for _, projectType := range input.ProjectTypeSnapshot {
			if _, err := tx.Exec(ctx, `INSERT INTO production_project_type_snapshots(tenant_id,work_item_id,project_id,project_source_revision,project_type_key,captured_at) VALUES($1,$2,$3,$4,$5,$6)`, actor.TenantID, id, input.ProjectID, input.ProjectSourceRevision, projectType, now); err != nil {
				return Details{}, mapWriteError(err)
			}
		}
	}
	for i, d := range definition.Stages {
		status := StagePending
		var reason *string
		if i == 0 {
			status = StageReady
		}
		if definition.Version == StageMaterialDefinitionVersion {
			applicable, err := stageMaterialApplicable(definition.Version, d.Key, input.ProjectTypeSnapshot)
			if err != nil {
				return Details{}, err
			}
			if !applicable {
				status = StageNotApplicable
				text := "project-type-snapshot"
				reason = &text
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO design_stages(tenant_id,work_item_id,stage_key,sequence,status,not_applicable_reason,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, actor.TenantID, id, d.Key, d.Sequence, status, reason, now); err != nil {
			return Details{}, err
		}
	}
	if err := insertHistory(ctx, tx, item, actor, input.CommandID, "work_item_created", map[string]any{"project_id": input.ProjectID, "project_source_revision": input.ProjectSourceRevision, "project_type_snapshot": input.ProjectTypeSnapshot, "type_key": definition.Key, "type_definition_version": definition.Version}); err != nil {
		return Details{}, err
	}
	result, err := loadDetails(ctx, tx, actor.TenantID, id, true)
	if err != nil {
		return Details{}, err
	}
	if err := recordCommand(ctx, tx, actor.TenantID, input.CommandID, hash, 201, result, now); err != nil {
		return Details{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Details{}, err
	}
	return result, nil
}

func (s *PostgresStore) SetOwner(ctx context.Context, actor Actor, id string, input OwnerInput, display, hash string) (Details, error) {
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
	now := time.Now().UTC()
	item.Revision++
	item.OwnerAssignmentID = &input.AssignmentID
	item.OwnerSourceRevision = &input.AssignmentSourceRevision
	if item.Status == StatusDraft {
		item.Status = StatusInProgress
	}
	item.UpdatedAt = now
	if _, err := tx.Exec(ctx, `UPDATE design_work_items SET owner_assignment_id=$3,owner_source_revision=$4,status=$5,revision=$6,updated_at=$7 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, id, input.AssignmentID, input.AssignmentSourceRevision, item.Status, item.Revision, now); err != nil {
		return Details{}, err
	}
	if err := insertHistory(ctx, tx, item, actor, input.CommandID, "owner_set", map[string]any{"assignment_id": input.AssignmentID, "assignment_source_revision": input.AssignmentSourceRevision, "display_snapshot": display}); err != nil {
		return Details{}, err
	}
	return finishDetails(ctx, tx, actor.TenantID, id, input.CommandID, hash, item.UpdatedAt)
}

func (s *PostgresStore) PutDeliverable(ctx context.Context, actor Actor, id, stage string, input DeliverableInput, hash string) (Deliverable, error) {
	tx, err := s.beginMutation(ctx, actor.TenantID, input.CommandID)
	if err != nil {
		return Deliverable{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var replay Deliverable
	if found, err := replayTx(ctx, tx, actor.TenantID, input.CommandID, hash, &replay); err != nil || found {
		return replay, err
	}
	item, err := loadItem(ctx, tx, actor.TenantID, id, "FOR UPDATE")
	if err != nil {
		return Deliverable{}, err
	}
	if err := checkMutable(item, input.ExpectedRevision); err != nil {
		return Deliverable{}, err
	}
	st, err := loadStage(ctx, tx, actor.TenantID, id, stage, "FOR UPDATE")
	if err != nil {
		return Deliverable{}, err
	}
	if st.Status != StageReady {
		return Deliverable{}, ErrStageInvalid
	}
	now := time.Now().UTC()
	item.Revision++
	item.UpdatedAt = now
	if _, err := tx.Exec(ctx, `UPDATE design_deliverables SET superseded_at=$5,superseded_work_item_revision=$6 WHERE tenant_id=$1 AND work_item_id=$2 AND stage_key=$3 AND requirement_key=$4 AND superseded_at IS NULL`, actor.TenantID, id, stage, input.RequirementKey, now, item.Revision); err != nil {
		return Deliverable{}, err
	}
	d := Deliverable{ID: newUUID(), TenantID: actor.TenantID, WorkItemID: id, StageKey: stage, RequirementKey: input.RequirementKey, EvidenceFileID: input.EvidenceFileID, EvidenceVersion: input.EvidenceVersion, EvidenceSourceRevision: input.EvidenceSourceRevision, DisplaySnapshot: input.DisplaySnapshot, CreatedRevision: item.Revision, CreatedAt: now}
	if _, err := tx.Exec(ctx, `INSERT INTO design_deliverables(id,tenant_id,work_item_id,stage_key,requirement_key,evidence_file_id,evidence_version,evidence_source_revision,display_snapshot,created_work_item_revision,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, d.ID, d.TenantID, d.WorkItemID, d.StageKey, d.RequirementKey, d.EvidenceFileID, d.EvidenceVersion, d.EvidenceSourceRevision, d.DisplaySnapshot, d.CreatedRevision, d.CreatedAt); err != nil {
		return Deliverable{}, mapWriteError(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE design_work_items SET revision=$3,updated_at=$4 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, id, item.Revision, now); err != nil {
		return Deliverable{}, err
	}
	if err := insertHistory(ctx, tx, item, actor, input.CommandID, "deliverable_registered", map[string]any{"deliverable_id": d.ID, "stage_key": stage, "requirement_key": d.RequirementKey, "evidence_file_id": d.EvidenceFileID, "evidence_version": d.EvidenceVersion, "evidence_source_revision": d.EvidenceSourceRevision}); err != nil {
		return Deliverable{}, err
	}
	if err := recordCommand(ctx, tx, actor.TenantID, input.CommandID, hash, 200, d, now); err != nil {
		return Deliverable{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Deliverable{}, err
	}
	return d, nil
}

func (s *PostgresStore) SetApplicability(ctx context.Context, actor Actor, id, stage string, input ApplicabilityInput, hash string) (Details, error) {
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
	st, err := loadStage(ctx, tx, actor.TenantID, id, stage, "FOR UPDATE")
	if err != nil {
		return Details{}, err
	}
	now := time.Now().UTC()
	item.Revision++
	event := "stage_not_applicable"
	if input.Operation == "set-not-applicable" {
		definition, ok := StageDefinitionFor(item.TypeKey, item.TypeDefinitionVersion, stage)
		if !ok || !definition.Skippable || st.Status != StageReady {
			return Details{}, ErrStageInvalid
		}
		if _, err := tx.Exec(ctx, `UPDATE design_stages SET status='not-applicable',not_applicable_reason=$4,completed_at=NULL,completed_work_item_revision=NULL,updated_at=$5 WHERE tenant_id=$1 AND work_item_id=$2 AND stage_key=$3`, actor.TenantID, id, stage, input.Reason, now); err != nil {
			return Details{}, err
		}
		next, err := advanceFrontier(ctx, tx, actor.TenantID, id, st.Sequence, now)
		if err != nil {
			return Details{}, err
		}
		item.CurrentStageKey = next
	} else {
		if st.Status != StageNotApplicable {
			return Details{}, ErrStageInvalid
		}
		event = "stage_applicability_restored"
		if err := rollbackFrom(ctx, tx, actor.TenantID, id, st.Sequence, stage, now); err != nil {
			return Details{}, err
		}
		item.CurrentStageKey = &stage
		item.Status = StatusInProgress
	}
	item.UpdatedAt = now
	if item.CurrentStageKey == nil {
		item.Status = StatusCompleted
	} else if item.OwnerAssignmentID != nil {
		item.Status = StatusInProgress
	}
	if _, err := tx.Exec(ctx, `UPDATE design_work_items SET status=$3,current_stage_key=$4,revision=$5,updated_at=$6 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, id, item.Status, item.CurrentStageKey, item.Revision, now); err != nil {
		return Details{}, err
	}
	if err := insertHistory(ctx, tx, item, actor, input.CommandID, event, map[string]any{"stage_key": stage, "reason": input.Reason}); err != nil {
		return Details{}, err
	}
	return finishDetails(ctx, tx, actor.TenantID, id, input.CommandID, hash, now)
}

func (s *PostgresStore) CompleteStage(ctx context.Context, actor Actor, id, stage string, input RevisionInput, hash string) (Details, error) {
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
	st, err := loadStage(ctx, tx, actor.TenantID, id, stage, "FOR UPDATE")
	if err != nil {
		return Details{}, err
	}
	if st.Status != StageReady {
		return Details{}, ErrStageInvalid
	}
	var has bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM design_deliverables WHERE tenant_id=$1 AND work_item_id=$2 AND stage_key=$3 AND requirement_key='primary-output' AND superseded_at IS NULL)`, actor.TenantID, id, stage).Scan(&has); err != nil {
		return Details{}, err
	}
	if !has || item.OwnerAssignmentID == nil {
		return Details{}, ErrCompletionGaps
	}
	now := time.Now().UTC()
	item.Revision++
	if _, err := tx.Exec(ctx, `UPDATE design_stages SET status='completed',completed_at=$4,completed_work_item_revision=$5,updated_at=$4 WHERE tenant_id=$1 AND work_item_id=$2 AND stage_key=$3`, actor.TenantID, id, stage, now, item.Revision); err != nil {
		return Details{}, err
	}
	next, err := advanceFrontier(ctx, tx, actor.TenantID, id, st.Sequence, now)
	if err != nil {
		return Details{}, err
	}
	item.CurrentStageKey = next
	item.Status = StatusInProgress
	if next == nil {
		item.Status = StatusCompleted
	}
	item.UpdatedAt = now
	if _, err := tx.Exec(ctx, `UPDATE design_work_items SET status=$3,current_stage_key=$4,revision=$5,updated_at=$6 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, id, item.Status, item.CurrentStageKey, item.Revision, now); err != nil {
		return Details{}, err
	}
	if err := insertHistory(ctx, tx, item, actor, input.CommandID, "stage_completed", map[string]any{"stage_key": stage}); err != nil {
		return Details{}, err
	}
	return finishDetails(ctx, tx, actor.TenantID, id, input.CommandID, hash, now)
}

func (s *PostgresStore) ReopenStage(ctx context.Context, actor Actor, id, stage string, input ReasonInput, hash string) (Details, error) {
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
	st, err := loadStage(ctx, tx, actor.TenantID, id, stage, "FOR UPDATE")
	if err != nil {
		return Details{}, err
	}
	if st.Status != StageCompleted {
		return Details{}, ErrStageInvalid
	}
	now := time.Now().UTC()
	item.Revision++
	if err := rollbackFrom(ctx, tx, actor.TenantID, id, st.Sequence, stage, now); err != nil {
		return Details{}, err
	}
	item.Status = StatusInProgress
	item.CurrentStageKey = &stage
	item.UpdatedAt = now
	if _, err := tx.Exec(ctx, `UPDATE design_work_items SET status=$3,current_stage_key=$4,revision=$5,updated_at=$6 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, id, item.Status, stage, item.Revision, now); err != nil {
		return Details{}, err
	}
	if err := insertHistory(ctx, tx, item, actor, input.CommandID, "stage_reopened", map[string]any{"stage_key": stage, "reason": input.Reason}); err != nil {
		return Details{}, err
	}
	return finishDetails(ctx, tx, actor.TenantID, id, input.CommandID, hash, now)
}

func (s *PostgresStore) Cancel(ctx context.Context, actor Actor, id string, input ReasonInput, hash string) (Details, error) {
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
	now := time.Now().UTC()
	item.Revision++
	item.Status = StatusCancelled
	item.CurrentStageKey = nil
	item.CancellationReason = &input.Reason
	item.UpdatedAt = now
	if _, err := tx.Exec(ctx, `UPDATE design_work_items SET status='cancelled',current_stage_key=NULL,cancellation_reason=$3,revision=$4,updated_at=$5 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, id, input.Reason, item.Revision, now); err != nil {
		return Details{}, err
	}
	if err := insertHistory(ctx, tx, item, actor, input.CommandID, "work_item_cancelled", map[string]any{"reason": input.Reason}); err != nil {
		return Details{}, err
	}
	return finishDetails(ctx, tx, actor.TenantID, id, input.CommandID, hash, now)
}

func (s *PostgresStore) Get(ctx context.Context, tenant, id string, includeSuperseded bool) (Details, error) {
	// Item revision, stages and current references must describe one snapshot,
	// rather than combining rows on either side of a concurrent mutation.
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Details{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	details, err := loadDetails(ctx, tx, tenant, id, includeSuperseded)
	if err != nil {
		return Details{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Details{}, err
	}
	return details, nil
}
func (s *PostgresStore) History(ctx context.Context, tenant, id string, after int64, limit int) (HistoryPage, error) {
	if _, err := loadItem(ctx, s.pool, tenant, id, ""); err != nil {
		return HistoryPage{}, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id,tenant_id,work_item_id,work_item_revision,event_sequence,event_type,command_id,actor_subject,occurred_at,payload FROM design_history_events WHERE tenant_id=$1 AND work_item_id=$2 AND event_sequence>$3 ORDER BY event_sequence,id LIMIT $4`, tenant, id, after, limit+1)
	if err != nil {
		return HistoryPage{}, err
	}
	defer rows.Close()
	items := []HistoryEvent{}
	for rows.Next() {
		var v HistoryEvent
		var raw []byte
		if err := rows.Scan(&v.ID, &v.TenantID, &v.WorkItemID, &v.WorkItemRevision, &v.EventSequence, &v.EventType, &v.CommandID, &v.ActorSubject, &v.OccurredAt, &raw); err != nil {
			return HistoryPage{}, err
		}
		if err := json.Unmarshal(raw, &v.Payload); err != nil {
			return HistoryPage{}, err
		}
		items = append(items, v)
	}
	if err := rows.Err(); err != nil {
		return HistoryPage{}, err
	}
	page := HistoryPage{Items: items}
	if len(items) > limit {
		next := items[limit-1].EventSequence
		page.Items = items[:limit]
		page.NextAfterSequence = &next
	}
	return page, nil
}

func (s *PostgresStore) beginMutation(ctx context.Context, tenant, commandID string) (pgx.Tx, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	if err := lockCommand(ctx, tx, tenant, commandID); err != nil {
		_ = tx.Rollback(context.Background())
		return nil, err
	}
	return tx, nil
}
func finishDetails(ctx context.Context, tx pgx.Tx, tenant, id, commandID, hash string, at time.Time) (Details, error) {
	result, err := loadDetails(ctx, tx, tenant, id, true)
	if err != nil {
		return Details{}, err
	}
	if err := recordCommand(ctx, tx, tenant, commandID, hash, 200, result, at); err != nil {
		return Details{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Details{}, err
	}
	return result, nil
}
func checkMutable(item WorkItem, expected int64) error {
	if item.Revision != expected {
		return ErrRevisionConflict
	}
	if item.Status == StatusCancelled {
		return ErrStateConflict
	}
	return nil
}
func advanceFrontier(ctx context.Context, tx pgx.Tx, tenant, id string, sequence int, at time.Time) (*string, error) {
	var key string
	err := tx.QueryRow(ctx, `SELECT stage_key FROM design_stages WHERE tenant_id=$1 AND work_item_id=$2 AND sequence>$3 AND status='pending' ORDER BY sequence LIMIT 1`, tenant, id, sequence).Scan(&key)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE design_stages SET status='ready',updated_at=$4 WHERE tenant_id=$1 AND work_item_id=$2 AND stage_key=$3`, tenant, id, key, at); err != nil {
		return nil, err
	}
	return &key, nil
}
func rollbackFrom(ctx context.Context, tx pgx.Tx, tenant, id string, sequence int, stage string, at time.Time) error {
	if _, err := tx.Exec(ctx, `UPDATE design_stages SET status=CASE WHEN stage_key=$4 THEN 'ready' ELSE 'pending' END,not_applicable_reason=CASE WHEN stage_key=$4 THEN NULL ELSE not_applicable_reason END,completed_at=NULL,completed_work_item_revision=NULL,updated_at=$5 WHERE tenant_id=$1 AND work_item_id=$2 AND sequence>=$3 AND (stage_key=$4 OR status='completed')`, tenant, id, sequence, stage, at); err != nil {
		return err
	}
	return nil
}

type rowScanner interface{ Scan(...any) error }
type queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func loadItem(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, tenant, id, suffix string) (WorkItem, error) {
	var v WorkItem
	err := q.QueryRow(ctx, `SELECT id,tenant_id,project_id,project_source_revision,type_key,type_definition_version,status,revision,owner_assignment_id,owner_source_revision,current_stage_key,cancellation_reason,created_by,created_at,updated_at FROM design_work_items WHERE tenant_id=$1 AND id=$2 `+suffix, tenant, id).Scan(&v.ID, &v.TenantID, &v.ProjectID, &v.ProjectSourceRevision, &v.TypeKey, &v.TypeDefinitionVersion, &v.Status, &v.Revision, &v.OwnerAssignmentID, &v.OwnerSourceRevision, &v.CurrentStageKey, &v.CancellationReason, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkItem{}, ErrNotFound
	}
	return v, err
}
func loadStage(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, tenant, id, key, suffix string) (Stage, error) {
	var v Stage
	err := q.QueryRow(ctx, `SELECT tenant_id,work_item_id,stage_key,sequence,status,not_applicable_reason,completed_at,completed_work_item_revision,updated_at FROM design_stages WHERE tenant_id=$1 AND work_item_id=$2 AND stage_key=$3 `+suffix, tenant, id, key).Scan(&v.TenantID, &v.WorkItemID, &v.Key, &v.Sequence, &v.Status, &v.NotApplicableReason, &v.CompletedAt, &v.CompletedRevision, &v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Stage{}, ErrStageInvalid
	}
	return v, err
}
func loadDetails(ctx context.Context, q queryer, tenant, id string, includeSuperseded bool) (Details, error) {
	item, err := loadItem(ctx, q, tenant, id, "")
	if err != nil {
		return Details{}, err
	}
	result := Details{WorkItem: item, Stages: []Stage{}, Deliverables: []Deliverable{}, ReferenceStatuses: []ReferenceStatus{}}
	rows, err := q.Query(ctx, `SELECT tenant_id,work_item_id,stage_key,sequence,status,not_applicable_reason,completed_at,completed_work_item_revision,updated_at FROM design_stages WHERE tenant_id=$1 AND work_item_id=$2 ORDER BY sequence`, tenant, id)
	if err != nil {
		return Details{}, err
	}
	for rows.Next() {
		var v Stage
		if err := rows.Scan(&v.TenantID, &v.WorkItemID, &v.Key, &v.Sequence, &v.Status, &v.NotApplicableReason, &v.CompletedAt, &v.CompletedRevision, &v.UpdatedAt); err != nil {
			rows.Close()
			return Details{}, err
		}
		result.Stages = append(result.Stages, v)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Details{}, err
	}
	rows.Close()
	sql := `SELECT id,tenant_id,work_item_id,stage_key,requirement_key,evidence_file_id,evidence_version,evidence_source_revision,display_snapshot,created_work_item_revision,superseded_work_item_revision,created_at,superseded_at FROM design_deliverables WHERE tenant_id=$1 AND work_item_id=$2`
	if !includeSuperseded {
		sql += ` AND superseded_at IS NULL`
	}
	sql += ` ORDER BY created_at,id`
	drows, err := q.Query(ctx, sql, tenant, id)
	if err != nil {
		return Details{}, err
	}
	defer drows.Close()
	for drows.Next() {
		var v Deliverable
		if err := drows.Scan(&v.ID, &v.TenantID, &v.WorkItemID, &v.StageKey, &v.RequirementKey, &v.EvidenceFileID, &v.EvidenceVersion, &v.EvidenceSourceRevision, &v.DisplaySnapshot, &v.CreatedRevision, &v.SupersededRevision, &v.CreatedAt, &v.SupersededAt); err != nil {
			return Details{}, err
		}
		result.Deliverables = append(result.Deliverables, v)
	}
	return result, drows.Err()
}

func lockCommand(ctx context.Context, tx pgx.Tx, tenant, commandID string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, tenant+":"+commandID)
	return err
}
func replayTx(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, tenant, commandID, hash string, dest any) (bool, error) {
	var stored string
	var raw []byte
	err := q.QueryRow(ctx, `SELECT request_hash,response_json FROM design_commands WHERE tenant_id=$1 AND command_id=$2`, tenant, commandID).Scan(&stored, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if stored != hash {
		return false, ErrCommandConflict
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return false, err
	}
	return true, nil
}
func recordCommand(ctx context.Context, tx pgx.Tx, tenant, commandID, hash string, status int, response any, at time.Time) error {
	raw, err := json.Marshal(response)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO design_commands(tenant_id,command_id,request_hash,response_status,response_json,created_at) VALUES($1,$2,$3,$4,$5,$6)`, tenant, commandID, hash, status, raw, at)
	return err
}
func insertHistory(ctx context.Context, tx pgx.Tx, item WorkItem, actor Actor, commandID, eventType string, payload map[string]any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO design_history_events(id,tenant_id,work_item_id,work_item_revision,event_sequence,event_type,command_id,actor_subject,occurred_at,payload) VALUES($1,$2,$3,$4,$4,$5,$6,$7,$8,$9)`, newUUID(), item.TenantID, item.ID, item.Revision, eventType, commandID, actor.ID, item.UpdatedAt, raw)
	return err
}
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
func mapWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "design_work_items_active_project_type_idx":
			return ErrActiveConflict
		case "design_commands_pkey":
			return ErrCommandConflict
		case "design_deliverables_active_slot_idx":
			return ErrStateConflict
		}
	}
	return err
}
