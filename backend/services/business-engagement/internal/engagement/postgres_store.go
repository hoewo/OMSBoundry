package engagement

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
	return &PostgresStore{pool: pool}, nil
}

func (s *PostgresStore) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

func (s *PostgresStore) List(ctx context.Context, tenantID string, limit int) ([]Engagement, error) {
	return s.list(ctx, tenantID, "", limit)
}

func (s *PostgresStore) ListProject(ctx context.Context, tenantID, projectID string, limit int) ([]Engagement, error) {
	return s.list(ctx, tenantID, projectID, limit)
}

func (s *PostgresStore) list(ctx context.Context, tenantID, projectID string, limit int) ([]Engagement, error) {
	query := `SELECT id,tenant_id,project_id,project_source_revision,applicability,engagement_method,material_policy_version,status,revision,not_applicable_reason,cancellation_reason,created_by,created_at,updated_at FROM business_engagements WHERE tenant_id=$1`
	args := []any{tenantID, limit}
	if projectID != "" {
		query += " AND project_id=$3"
		args = append(args, projectID)
	}
	rows, err := s.pool.Query(ctx, query+" ORDER BY updated_at DESC,id DESC LIMIT $2", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Engagement, 0, limit)
	for rows.Next() {
		value, err := scanEngagement(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, value)
	}
	return items, rows.Err()
}

func (s *PostgresStore) Get(ctx context.Context, tenantID, id string) (Details, error) {
	return loadDetails(ctx, s.pool, tenantID, id)
}

func (s *PostgresStore) History(ctx context.Context, tenantID, id string, after int64, limit int) (HistoryPage, error) {
	if _, err := loadEngagement(ctx, s.pool, tenantID, id, ""); err != nil {
		return HistoryPage{}, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id,tenant_id,engagement_id,engagement_revision,event_sequence,event_type,command_id,actor_subject,occurred_at,payload FROM business_engagement_history_events WHERE tenant_id=$1 AND engagement_id=$2 AND event_sequence>$3 ORDER BY event_sequence,id LIMIT $4`, tenantID, id, after, limit+1)
	if err != nil {
		return HistoryPage{}, err
	}
	defer rows.Close()
	items := []HistoryEvent{}
	for rows.Next() {
		var event HistoryEvent
		var raw []byte
		if err := rows.Scan(&event.ID, &event.TenantID, &event.EngagementID, &event.EngagementRevision, &event.EventSequence, &event.EventType, &event.CommandID, &event.ActorSubject, &event.OccurredAt, &raw); err != nil {
			return HistoryPage{}, err
		}
		if err := json.Unmarshal(raw, &event.Payload); err != nil {
			return HistoryPage{}, err
		}
		items = append(items, event)
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

func (s *PostgresStore) Replay(ctx context.Context, tenantID, commandID, hash string, destination any) (bool, error) {
	var stored string
	var raw []byte
	err := s.pool.QueryRow(ctx, `SELECT request_hash,response_json FROM business_engagement_commands WHERE tenant_id=$1 AND command_id=$2`, tenantID, commandID).Scan(&stored, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if stored != hash {
		return false, ErrCommandConflict
	}
	if err := json.Unmarshal(raw, destination); err != nil {
		return false, err
	}
	return true, nil
}

func (s *PostgresStore) Create(ctx context.Context, actor Actor, input CreateInput, project ProjectReference, hash string) (Details, error) {
	tx, err := s.begin(ctx, actor.TenantID, input.CommandID)
	if err != nil {
		return Details{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var replay Details
	if found, err := replayTx(ctx, tx, actor.TenantID, input.CommandID, hash, &replay); err != nil || found {
		return replay, err
	}
	now := time.Now().UTC()
	value := Engagement{
		ID: newUUID(), TenantID: actor.TenantID, ProjectID: input.ProjectID,
		ProjectSourceRevision: input.ProjectSourceRevision,
		Applicability:         ApplicabilityUndetermined, MaterialPolicyVersion: 1, Status: StatusDraft,
		Revision: 1, CreatedBy: actor.ID, CreatedAt: now, UpdatedAt: now,
	}
	_, err = tx.Exec(ctx, `INSERT INTO business_engagements(id,tenant_id,project_id,project_source_revision,applicability,material_policy_version,status,revision,created_by,created_at,updated_at) VALUES($1,$2,$3,$4,$5,1,$6,1,$7,$8,$8)`, value.ID, value.TenantID, value.ProjectID, value.ProjectSourceRevision, value.Applicability, value.Status, value.CreatedBy, now)
	if err != nil {
		return Details{}, mapWriteError(err)
	}
	if err := insertHistory(ctx, tx, value, actor, input.CommandID, "engagement_created", map[string]any{"project_id": input.ProjectID, "project_source_revision": input.ProjectSourceRevision}); err != nil {
		return Details{}, err
	}
	result := BuildDetails(value, nil)
	if err := recordCommand(ctx, tx, actor.TenantID, input.CommandID, hash, result, now); err != nil {
		return Details{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Details{}, err
	}
	return result, nil
}

func (s *PostgresStore) SetApplicability(ctx context.Context, actor Actor, id string, input ApplicabilityInput, hash string) (Details, error) {
	return s.mutate(ctx, actor, id, input.CommandID, hash, func(ctx context.Context, tx pgx.Tx, value *Engagement, materials []MaterialReference, now time.Time) (string, map[string]any, error) {
		if err := mutable(*value, input.ExpectedRevision); err != nil {
			return "", nil, err
		}
		value.Revision++
		if input.Applicable {
			value.Applicability = ApplicabilityApplicable
			value.NotApplicableReason = nil
		} else {
			value.Applicability = ApplicabilityNotApplicable
			reason := input.Reason
			value.NotApplicableReason = &reason
		}
		value.Status = DeriveStatus(*value, materials)
		_, err := tx.Exec(ctx, `UPDATE business_engagements SET applicability=$3,status=$4,revision=$5,not_applicable_reason=$6,updated_at=$7 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, id, value.Applicability, value.Status, value.Revision, value.NotApplicableReason, now)
		return "applicability_changed", map[string]any{"applicability": value.Applicability, "reason": input.Reason}, err
	})
}

func (s *PostgresStore) SelectMethod(ctx context.Context, actor Actor, id string, input MethodInput, hash string) (Details, error) {
	return s.mutate(ctx, actor, id, input.CommandID, hash, func(ctx context.Context, tx pgx.Tx, value *Engagement, materials []MaterialReference, now time.Time) (string, map[string]any, error) {
		if err := mutable(*value, input.ExpectedRevision); err != nil {
			return "", nil, err
		}
		value.Revision++
		method := input.Method
		value.Method = &method
		value.Status = DeriveStatus(*value, materials)
		_, err := tx.Exec(ctx, `UPDATE business_engagements SET engagement_method=$3,status=$4,revision=$5,updated_at=$6 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, id, method, value.Status, value.Revision, now)
		return "method_selected", map[string]any{"method": method, "missing_requirements": BuildDetails(*value, materials).MissingRequirements}, err
	})
}

func (s *PostgresStore) ApplyMaterial(ctx context.Context, actor Actor, id string, input MaterialInput, hash string) (Details, error) {
	return s.mutate(ctx, actor, id, input.CommandID, hash, func(ctx context.Context, tx pgx.Tx, value *Engagement, materials []MaterialReference, now time.Time) (string, map[string]any, error) {
		if err := mutable(*value, input.ExpectedRevision); err != nil {
			return "", nil, err
		}
		if value.Applicability != ApplicabilityApplicable || value.Method == nil || !contains(RequiredKeys(*value.Method), input.RequirementKey) {
			return "", nil, ErrStateConflict
		}
		if err := validateMaterialChange(input, materials); err != nil {
			return "", nil, err
		}
		value.Revision++
		if input.Operation == "put" || input.Operation == "remove" {
			if _, err := tx.Exec(ctx, `UPDATE business_engagement_materials SET superseded_engagement_revision=$4,superseded_at=$5 WHERE tenant_id=$1 AND engagement_id=$2 AND requirement_key=$3 AND superseded_at IS NULL`, actor.TenantID, id, input.RequirementKey, value.Revision, now); err != nil {
				return "", nil, err
			}
		} else if input.Operation == "remove_one" {
			changed, err := tx.Exec(ctx, `UPDATE business_engagement_materials SET superseded_engagement_revision=$4,superseded_at=$5 WHERE tenant_id=$1 AND engagement_id=$2 AND requirement_key=$3 AND id=$6 AND superseded_at IS NULL`, actor.TenantID, id, input.RequirementKey, value.Revision, now, input.MaterialReferenceID)
			if err != nil {
				return "", nil, err
			}
			if changed.RowsAffected() != 1 {
				return "", nil, ErrStateConflict
			}
		}
		materialID := input.MaterialReferenceID
		if input.Operation == "put" || input.Operation == "append" {
			material := MaterialReference{ID: newUUID(), TenantID: actor.TenantID, EngagementID: id, RequirementKey: input.RequirementKey, EvidenceFileID: input.EvidenceFileID, EvidenceVersion: input.EvidenceVersion, EvidenceSourceRevision: input.EvidenceSourceRevision, DisplaySnapshot: input.DisplaySnapshot, CreatedEngagementRevision: value.Revision, CreatedAt: now}
			materialID = material.ID
			if _, err := tx.Exec(ctx, `INSERT INTO business_engagement_materials(id,tenant_id,engagement_id,requirement_key,evidence_file_id,evidence_version,evidence_source_revision,display_snapshot,created_engagement_revision,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, material.ID, material.TenantID, material.EngagementID, material.RequirementKey, material.EvidenceFileID, material.EvidenceVersion, material.EvidenceSourceRevision, material.DisplaySnapshot, material.CreatedEngagementRevision, material.CreatedAt); err != nil {
				return "", nil, mapWriteError(err)
			}
		}
		updatedMaterials, err := loadMaterials(ctx, tx, actor.TenantID, id)
		if err != nil {
			return "", nil, err
		}
		value.Status = DeriveStatus(*value, updatedMaterials)
		if _, err := tx.Exec(ctx, `UPDATE business_engagements SET status=$3,revision=$4,updated_at=$5 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, id, value.Status, value.Revision, now); err != nil {
			return "", nil, err
		}
		payload := map[string]any{"requirement_key": input.RequirementKey, "evidence_file_id": input.EvidenceFileID, "evidence_version": input.EvidenceVersion}
		if input.Operation == "append" || input.Operation == "remove_one" {
			payload["material_reference_id"] = materialID
		}
		return "material_" + input.Operation, payload, nil
	})
}

// Called under the Engagement row lock: concurrent commands cannot bypass the
// collection limit or supersede a neighbour. Only this owner's rows are loaded.
func validateMaterialChange(input MaterialInput, materials []MaterialReference) error {
	active, sameKey := 0, 0
	target := false
	for _, material := range materials {
		if material.SupersededAt != nil {
			continue
		}
		active++
		if material.RequirementKey != input.RequirementKey {
			continue
		}
		sameKey++
		if material.ID == input.MaterialReferenceID {
			target = true
		}
		if input.Operation == "append" && material.EvidenceFileID == input.EvidenceFileID && material.EvidenceVersion == input.EvidenceVersion {
			return ErrStateConflict
		}
	}
	switch input.Operation {
	case "append":
		if active >= MaxActiveMaterials {
			return ErrStateConflict
		}
	case "put":
		if active-sameKey+1 > MaxActiveMaterials {
			return ErrStateConflict
		}
	case "remove":
		if sameKey == 0 {
			return ErrStateConflict
		}
	case "remove_one":
		if !target {
			return ErrStateConflict
		}
	default:
		return ErrStateConflict
	}
	return nil
}

func (s *PostgresStore) Cancel(ctx context.Context, actor Actor, id string, input CancelInput, hash string) (Details, error) {
	return s.mutate(ctx, actor, id, input.CommandID, hash, func(ctx context.Context, tx pgx.Tx, value *Engagement, _ []MaterialReference, now time.Time) (string, map[string]any, error) {
		if err := mutable(*value, input.ExpectedRevision); err != nil {
			return "", nil, err
		}
		value.Revision++
		value.Status = StatusCancelled
		reason := input.Reason
		value.CancellationReason = &reason
		_, err := tx.Exec(ctx, `UPDATE business_engagements SET status=$3,revision=$4,cancellation_reason=$5,updated_at=$6 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, id, value.Status, value.Revision, reason, now)
		return "engagement_cancelled", map[string]any{"reason": reason}, err
	})
}

type mutation func(context.Context, pgx.Tx, *Engagement, []MaterialReference, time.Time) (string, map[string]any, error)

func (s *PostgresStore) mutate(ctx context.Context, actor Actor, id, commandID, hash string, change mutation) (Details, error) {
	tx, err := s.begin(ctx, actor.TenantID, commandID)
	if err != nil {
		return Details{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var replay Details
	if found, err := replayTx(ctx, tx, actor.TenantID, commandID, hash, &replay); err != nil || found {
		return replay, err
	}
	value, err := loadEngagement(ctx, tx, actor.TenantID, id, "FOR UPDATE")
	if err != nil {
		return Details{}, err
	}
	materials, err := loadMaterials(ctx, tx, actor.TenantID, id)
	if err != nil {
		return Details{}, err
	}
	now := time.Now().UTC()
	eventType, payload, err := change(ctx, tx, &value, materials, now)
	if err != nil {
		return Details{}, err
	}
	value.UpdatedAt = now
	if err := insertHistory(ctx, tx, value, actor, commandID, eventType, payload); err != nil {
		return Details{}, err
	}
	result, err := loadDetails(ctx, tx, actor.TenantID, id)
	if err != nil {
		return Details{}, err
	}
	if err := recordCommand(ctx, tx, actor.TenantID, commandID, hash, result, now); err != nil {
		return Details{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Details{}, err
	}
	return result, nil
}

func (s *PostgresStore) begin(ctx context.Context, tenantID, commandID string) (pgx.Tx, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, tenantID+":"+commandID); err != nil {
		_ = tx.Rollback(context.Background())
		return nil, err
	}
	return tx, nil
}

func replayTx(ctx context.Context, tx pgx.Tx, tenantID, commandID, hash string, destination any) (bool, error) {
	var stored string
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT request_hash,response_json FROM business_engagement_commands WHERE tenant_id=$1 AND command_id=$2`, tenantID, commandID).Scan(&stored, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if stored != hash {
		return false, ErrCommandConflict
	}
	return true, json.Unmarshal(raw, destination)
}

func recordCommand(ctx context.Context, tx pgx.Tx, tenantID, commandID, hash string, response any, now time.Time) error {
	raw, err := json.Marshal(response)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO business_engagement_commands(tenant_id,command_id,request_hash,response_json,created_at) VALUES($1,$2,$3,$4,$5)`, tenantID, commandID, hash, raw, now)
	return err
}

func insertHistory(ctx context.Context, tx pgx.Tx, value Engagement, actor Actor, commandID, eventType string, payload map[string]any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO business_engagement_history_events(id,tenant_id,engagement_id,engagement_revision,event_sequence,event_type,command_id,actor_subject,occurred_at,payload) VALUES($1,$2,$3,$4,$4,$5,$6,$7,$8,$9)`, newUUID(), value.TenantID, value.ID, value.Revision, eventType, commandID, actor.ID, value.UpdatedAt, raw)
	return err
}

type rowScanner interface{ Scan(...any) error }

func scanEngagement(row rowScanner) (Engagement, error) {
	var value Engagement
	err := row.Scan(&value.ID, &value.TenantID, &value.ProjectID, &value.ProjectSourceRevision, &value.Applicability, &value.Method, &value.MaterialPolicyVersion, &value.Status, &value.Revision, &value.NotApplicableReason, &value.CancellationReason, &value.CreatedBy, &value.CreatedAt, &value.UpdatedAt)
	return value, err
}

func loadEngagement(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, tenantID, id, suffix string) (Engagement, error) {
	query := `SELECT id,tenant_id,project_id,project_source_revision,applicability,engagement_method,material_policy_version,status,revision,not_applicable_reason,cancellation_reason,created_by,created_at,updated_at FROM business_engagements WHERE tenant_id=$1 AND id=$2`
	if suffix != "" {
		query += " " + suffix
	}
	value, err := scanEngagement(q.QueryRow(ctx, query, tenantID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Engagement{}, ErrNotFound
	}
	return value, err
}

func loadMaterials(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, tenantID, id string) ([]MaterialReference, error) {
	rows, err := q.Query(ctx, `SELECT id,tenant_id,engagement_id,requirement_key,evidence_file_id,evidence_version,evidence_source_revision,display_snapshot,created_engagement_revision,superseded_engagement_revision,created_at,superseded_at FROM business_engagement_materials WHERE tenant_id=$1 AND engagement_id=$2 ORDER BY created_engagement_revision,id`, tenantID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []MaterialReference{}
	for rows.Next() {
		var value MaterialReference
		if err := rows.Scan(&value.ID, &value.TenantID, &value.EngagementID, &value.RequirementKey, &value.EvidenceFileID, &value.EvidenceVersion, &value.EvidenceSourceRevision, &value.DisplaySnapshot, &value.CreatedEngagementRevision, &value.SupersededEngagementRevision, &value.CreatedAt, &value.SupersededAt); err != nil {
			return nil, err
		}
		items = append(items, value)
	}
	return items, rows.Err()
}

func loadDetails(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, tenantID, id string) (Details, error) {
	value, err := loadEngagement(ctx, q, tenantID, id, "")
	if err != nil {
		return Details{}, err
	}
	materials, err := loadMaterials(ctx, q, tenantID, id)
	if err != nil {
		return Details{}, err
	}
	return BuildDetails(value, materials), nil
}

func mutable(value Engagement, expected int64) error {
	if value.Revision != expected {
		return ErrRevisionConflict
	}
	if value.Status == StatusCancelled {
		return ErrStateConflict
	}
	return nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func mapWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "business_engagements_tenant_id_project_id_key":
			return ErrProjectConflict
		case "business_engagement_commands_pkey":
			return ErrCommandConflict
		case "business_engagement_materials_active_version_key":
			return ErrStateConflict
		}
	}
	return err
}

func newUUID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(err)
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16])
}
