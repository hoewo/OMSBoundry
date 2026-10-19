package results

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5"
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

func (s *PostgresStore) Replay(ctx context.Context, actor Actor, commandID, hash string) (Result, bool, error) {
	return replay(ctx, s.pool, actor, commandID, hash)
}

func (s *PostgresStore) Record(ctx context.Context, actor Actor, input RecordInput, resolved ResolvedResult, hash string) (Result, error) {
	tx, err := s.begin(ctx, actor.TenantID, input.CommandID)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, replayErr := replay(ctx, tx, actor, input.CommandID, hash); replayErr != nil || found {
		return value, replayErr
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, actor.TenantID+":"+resolved.Project.ID+":"+input.Kind); err != nil {
		return Result{}, err
	}
	current, getErr := loadByProjectKind(ctx, tx, actor.TenantID, resolved.Project.ID, input.Kind, "FOR UPDATE")
	now := time.Now().UTC()
	eventKind, action, reason := EventRecorded, "result_recorded", "initial result registration"
	var value Result
	if errors.Is(getErr, ErrNotFound) {
		if input.ExpectedRevision != 0 {
			return Result{}, ErrRevisionConflict
		}
		value = resultFromResolved(newUUID(), actor.TenantID, input.Kind, StateRecorded, 1, now, now, resolved)
		if err = insertCurrent(ctx, tx, value); err != nil {
			return Result{}, err
		}
	} else if getErr != nil {
		return Result{}, getErr
	} else {
		if current.State != StateWithdrawn {
			return Result{}, ErrAlreadyRecorded
		}
		if input.ExpectedRevision != current.Revision {
			return Result{}, ErrRevisionConflict
		}
		eventKind, action, reason = EventReregister, "result_reregistered", "result re-registration after withdrawal"
		value = resultFromResolved(current.ID, actor.TenantID, input.Kind, StateRecorded, current.Revision+1, current.CreatedAt, now, resolved)
		if err = updateCurrent(ctx, tx, value); err != nil {
			return Result{}, err
		}
	}
	if err = insertRevision(ctx, tx, value, actor, input.CommandID, eventKind, reason); err != nil {
		return Result{}, err
	}
	if err = insertAudit(ctx, tx, value, actor, input.CommandID, value.Revision-1, action, reason); err != nil {
		return Result{}, err
	}
	if err = recordCommand(ctx, tx, actor, input.CommandID, hash, value, now); err != nil {
		return Result{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Result{}, err
	}
	return value, nil
}

func (s *PostgresStore) Get(ctx context.Context, tenantID, id string) (Result, error) {
	return loadResult(ctx, s.pool, tenantID, id, "")
}

func (s *PostgresStore) ListProject(ctx context.Context, tenantID, projectID string) ([]Result, error) {
	rows, err := s.pool.Query(ctx, `SELECT snapshot_json FROM approval_audit_results WHERE tenant_id=$1 AND project_id=$2 ORDER BY result_kind,id`, tenantID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]Result, 0, 2)
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var value Result
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *PostgresStore) GetRevision(ctx context.Context, tenantID, id string, revision int64) (Revision, error) {
	return loadRevision(ctx, s.pool, tenantID, id, revision)
}

func (s *PostgresStore) History(ctx context.Context, tenantID, id string) ([]Revision, error) {
	if _, err := loadResult(ctx, s.pool, tenantID, id, ""); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT result_id,tenant_id,revision,event_kind,command_id,snapshot_json,reason,actor_ref,occurred_at FROM approval_audit_result_revisions WHERE tenant_id=$1 AND result_id=$2 ORDER BY revision`, tenantID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []Revision
	for rows.Next() {
		value, scanErr := scanRevision(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *PostgresStore) Correct(ctx context.Context, actor Actor, id string, input CorrectInput, resolved ResolvedResult, hash string) (Result, error) {
	return s.mutate(ctx, actor, id, input.CommandID, hash, input.ExpectedRevision, input.Reason, func(value *Result) (string, string, error) {
		before := *value
		before.AmountComponents = append([]AmountComponent(nil), value.AmountComponents...)
		before.Evidence = append([]EvidenceSnapshot(nil), value.Evidence...)
		value.Project = resolved.Project
		value.Currency = resolved.Currency
		value.AmountComponents = append([]AmountComponent(nil), resolved.AmountComponents...)
		value.TotalAmountMinor = resolved.TotalAmountMinor
		value.Source = resolved.Source
		value.Evidence = append([]EvidenceSnapshot(nil), resolved.Evidence...)
		value.OverrideReason = resolved.OverrideReason
		value.ExternalConfirmedBy = resolved.ExternalConfirmedBy
		value.ExternalConfirmedAt = resolved.ExternalConfirmedAt
		before.Revision, before.UpdatedAt = 0, time.Time{}
		comparison := *value
		comparison.Revision, comparison.UpdatedAt = 0, time.Time{}
		if reflect.DeepEqual(before, comparison) {
			return "", "", ErrNoEffectiveChange
		}
		return EventCorrected, "result_corrected", nil
	})
}

func (s *PostgresStore) Withdraw(ctx context.Context, actor Actor, id string, input WithdrawInput, hash string) (Result, error) {
	return s.mutate(ctx, actor, id, input.CommandID, hash, input.ExpectedRevision, input.Reason, func(value *Result) (string, string, error) {
		value.State = StateWithdrawn
		return EventWithdrawn, "result_withdrawn", nil
	})
}

type changeFunc func(*Result) (string, string, error)

func (s *PostgresStore) mutate(ctx context.Context, actor Actor, id, commandID, hash string, expected int64, reason string, change changeFunc) (Result, error) {
	tx, err := s.begin(ctx, actor.TenantID, commandID)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, replayErr := replay(ctx, tx, actor, commandID, hash); replayErr != nil || found {
		return value, replayErr
	}
	value, err := loadResult(ctx, tx, actor.TenantID, id, "FOR UPDATE")
	if err != nil {
		return Result{}, err
	}
	if value.Revision != expected {
		return Result{}, ErrRevisionConflict
	}
	if value.State != StateRecorded {
		return Result{}, ErrResultWithdrawn
	}
	eventKind, action, err := change(&value)
	if err != nil {
		return Result{}, err
	}
	sourceRevision := value.Revision
	value.Revision++
	value.UpdatedAt = time.Now().UTC()
	if err = updateCurrent(ctx, tx, value); err != nil {
		return Result{}, err
	}
	if err = insertRevision(ctx, tx, value, actor, commandID, eventKind, reason); err != nil {
		return Result{}, err
	}
	if err = insertAudit(ctx, tx, value, actor, commandID, sourceRevision, action, reason); err != nil {
		return Result{}, err
	}
	if err = recordCommand(ctx, tx, actor, commandID, hash, value, value.UpdatedAt); err != nil {
		return Result{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Result{}, err
	}
	return value, nil
}

func resultFromResolved(id, tenantID, kind, state string, revision int64, createdAt, updatedAt time.Time, resolved ResolvedResult) Result {
	return Result{ID: id, TenantID: tenantID, Project: resolved.Project, Kind: kind, State: state, AmountProfile: AmountProfileReferenceOAV1, Currency: resolved.Currency, AmountComponents: append([]AmountComponent(nil), resolved.AmountComponents...), TotalAmountMinor: resolved.TotalAmountMinor, Source: resolved.Source, Evidence: append([]EvidenceSnapshot(nil), resolved.Evidence...), OverrideReason: resolved.OverrideReason, ExternalConfirmedBy: resolved.ExternalConfirmedBy, ExternalConfirmedAt: resolved.ExternalConfirmedAt, Revision: revision, CreatedAt: createdAt, UpdatedAt: updatedAt}
}

func (s *PostgresStore) begin(ctx context.Context, tenantID, commandID string) (pgx.Tx, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, tenantID+":"+commandID); err != nil {
		_ = tx.Rollback(context.Background())
		return nil, err
	}
	return tx, nil
}

type rowScanner interface{ Scan(...any) error }
type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func replay(ctx context.Context, q rowQuerier, actor Actor, commandID, hash string) (Result, bool, error) {
	var storedHash, storedActor string
	var raw []byte
	err := q.QueryRow(ctx, `SELECT request_hash,actor_subject::text,response_json FROM approval_audit_result_commands WHERE tenant_id=$1 AND command_id=$2`, actor.TenantID, commandID).Scan(&storedHash, &storedActor, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, false, nil
	}
	if err != nil {
		return Result{}, false, err
	}
	if storedHash != hash || storedActor != actor.ID {
		return Result{}, false, ErrCommandConflict
	}
	var value Result
	if err = json.Unmarshal(raw, &value); err != nil {
		return Result{}, false, err
	}
	return value, true, nil
}

func insertCurrent(ctx context.Context, tx pgx.Tx, value Result) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO approval_audit_results(id,tenant_id,project_id,result_kind,state,currency,total_amount_minor,revision,snapshot_json,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, value.ID, value.TenantID, value.Project.ID, value.Kind, value.State, value.Currency, value.TotalAmountMinor, value.Revision, raw, value.CreatedAt, value.UpdatedAt)
	return err
}

func updateCurrent(ctx context.Context, tx pgx.Tx, value Result) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE approval_audit_results SET project_id=$3,result_kind=$4,state=$5,currency=$6,total_amount_minor=$7,revision=$8,snapshot_json=$9,updated_at=$10 WHERE tenant_id=$1 AND id=$2`, value.TenantID, value.ID, value.Project.ID, value.Kind, value.State, value.Currency, value.TotalAmountMinor, value.Revision, raw, value.UpdatedAt)
	return err
}

func insertRevision(ctx context.Context, tx pgx.Tx, value Result, actor Actor, commandID, eventKind, reason string) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO approval_audit_result_revisions(tenant_id,result_id,revision,event_kind,command_id,state,snapshot_json,reason,actor_ref,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, value.TenantID, value.ID, value.Revision, eventKind, commandID, value.State, raw, reason, actor.Kind+":"+actor.ID, value.UpdatedAt)
	return err
}

func insertAudit(ctx context.Context, tx pgx.Tx, value Result, actor Actor, commandID string, sourceRevision int64, action, reason string) error {
	_, err := tx.Exec(ctx, `INSERT INTO approval_audit_result_audit_events(id,tenant_id,result_id,command_id,source_revision,result_revision,action,reason,actor_subject,actor_kind,credential_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, newUUID(), value.TenantID, value.ID, commandID, sourceRevision, value.Revision, action, reason, actor.ID, actor.Kind, actor.CredentialID, value.UpdatedAt)
	return err
}

func recordCommand(ctx context.Context, tx pgx.Tx, actor Actor, commandID, hash string, value Result, now time.Time) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO approval_audit_result_commands(tenant_id,command_id,request_hash,actor_subject,response_json,created_at) VALUES($1,$2,$3,$4,$5,$6)`, actor.TenantID, commandID, hash, actor.ID, raw, now)
	return err
}

func loadResult(ctx context.Context, q rowQuerier, tenantID, id, suffix string) (Result, error) {
	var raw []byte
	err := q.QueryRow(ctx, `SELECT snapshot_json FROM approval_audit_results WHERE tenant_id=$1 AND id=$2 `+suffix, tenantID, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, ErrNotFound
	}
	if err != nil {
		return Result{}, err
	}
	var value Result
	if err = json.Unmarshal(raw, &value); err != nil {
		return Result{}, err
	}
	return value, nil
}

func loadByProjectKind(ctx context.Context, q rowQuerier, tenantID, projectID, kind, suffix string) (Result, error) {
	var raw []byte
	err := q.QueryRow(ctx, `SELECT snapshot_json FROM approval_audit_results WHERE tenant_id=$1 AND project_id=$2 AND result_kind=$3 `+suffix, tenantID, projectID, kind).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, ErrNotFound
	}
	if err != nil {
		return Result{}, err
	}
	var value Result
	if err = json.Unmarshal(raw, &value); err != nil {
		return Result{}, err
	}
	return value, nil
}

func loadRevision(ctx context.Context, q rowQuerier, tenantID, id string, revision int64) (Revision, error) {
	value, err := scanRevision(q.QueryRow(ctx, `SELECT result_id,tenant_id,revision,event_kind,command_id,snapshot_json,reason,actor_ref,occurred_at FROM approval_audit_result_revisions WHERE tenant_id=$1 AND result_id=$2 AND revision=$3`, tenantID, id, revision))
	if errors.Is(err, pgx.ErrNoRows) {
		return Revision{}, ErrNotFound
	}
	return value, err
}

func scanRevision(row rowScanner) (Revision, error) {
	var value Revision
	var raw []byte
	if err := row.Scan(&value.ResultID, &value.TenantID, &value.Revision, &value.EventKind, &value.CommandID, &raw, &value.Reason, &value.ActorRef, &value.OccurredAt); err != nil {
		return Revision{}, err
	}
	if err := json.Unmarshal(raw, &value.Snapshot); err != nil {
		return Revision{}, err
	}
	return value, nil
}

func newUUID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(err)
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16])
}
