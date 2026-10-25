package payroll

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) (*PostgresStore, error) {
	if pool == nil {
		return nil, fmt.Errorf("pool is required")
	}
	return &PostgresStore{pool: pool}, nil
}
func (s *PostgresStore) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }
func (s *PostgresStore) Create(ctx context.Context, actor Actor, input CreateInput, requestHash string) (Run, error) {
	tx, err := s.begin(ctx, actor.TenantID, input.CommandID)
	if err != nil {
		return Run{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, replayErr := replay(ctx, tx, actor, input.CommandID, requestHash); replayErr != nil || found {
		return value, replayErr
	}
	now := time.Now().UTC()
	value := Run{ID: newUUID(), TenantID: actor.TenantID, Period: input.Period, Label: input.Label, Status: "draft", Revision: 1, CreatedBySubjectID: actor.ID, CreatedAt: now, UpdatedAt: now}
	if _, err = tx.Exec(ctx, `INSERT INTO payroll_runs(id,tenant_id,period,label,status,revision,created_by_subject_id,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, value.ID, value.TenantID, value.Period, value.Label, value.Status, value.Revision, value.CreatedBySubjectID, value.CreatedAt, value.UpdatedAt); err != nil {
		return Run{}, err
	}
	if err = insertRevision(ctx, tx, value, actor, input.CommandID, "created"); err != nil {
		return Run{}, err
	}
	if err = insertAudit(ctx, tx, value, actor, input.CommandID, 0, "payroll_run_created"); err != nil {
		return Run{}, err
	}
	if err = record(ctx, tx, actor, input.CommandID, requestHash, value); err != nil {
		return Run{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Run{}, err
	}
	return value, nil
}
func (s *PostgresStore) Update(ctx context.Context, actor Actor, id string, input UpdateInput, requestHash string) (Run, error) {
	tx, err := s.begin(ctx, actor.TenantID, input.CommandID)
	if err != nil {
		return Run{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, replayErr := replay(ctx, tx, actor, input.CommandID, requestHash); replayErr != nil || found {
		return value, replayErr
	}
	value, err := load(ctx, tx, actor.TenantID, id, "FOR UPDATE")
	if err != nil {
		return Run{}, err
	}
	if value.Revision != input.ExpectedRevision {
		return Run{}, ErrRevisionConflict
	}
	source := value.Revision
	value.Label = input.Label
	value.Revision++
	value.UpdatedAt = time.Now().UTC()
	if _, err = tx.Exec(ctx, `UPDATE payroll_runs SET label=$3,revision=$4,updated_at=$5 WHERE tenant_id=$1 AND id=$2`, value.TenantID, value.ID, value.Label, value.Revision, value.UpdatedAt); err != nil {
		return Run{}, err
	}
	if err = insertRevision(ctx, tx, value, actor, input.CommandID, "draft_updated"); err != nil {
		return Run{}, err
	}
	if err = insertAudit(ctx, tx, value, actor, input.CommandID, source, "payroll_run_draft_updated"); err != nil {
		return Run{}, err
	}
	if err = record(ctx, tx, actor, input.CommandID, requestHash, value); err != nil {
		return Run{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Run{}, err
	}
	return value, nil
}
func (s *PostgresStore) Get(ctx context.Context, tenant, id string) (Run, error) {
	return load(ctx, s.pool, tenant, id, "")
}
func (s *PostgresStore) List(ctx context.Context, tenant string) ([]Run, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,tenant_id,period,label,status,revision,created_by_subject_id,created_at,updated_at FROM payroll_runs WHERE tenant_id=$1 ORDER BY period DESC,id`, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []Run
	for rows.Next() {
		value, scanErr := scan(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}
func (s *PostgresStore) begin(ctx context.Context, tenant, command string) (pgx.Tx, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, tenant+":"+command); err != nil {
		_ = tx.Rollback(context.Background())
		return nil, err
	}
	return tx, nil
}
func replay(ctx context.Context, tx pgx.Tx, actor Actor, command, requestHash string) (Run, bool, error) {
	var stored, storedActor string
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT request_hash,actor_subject_id::text,response_json FROM payroll_commands WHERE tenant_id=$1 AND command_id=$2`, actor.TenantID, command).Scan(&stored, &storedActor, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Run{}, false, nil
	}
	if err != nil {
		return Run{}, false, err
	}
	if stored != requestHash || storedActor != actor.ID {
		return Run{}, false, ErrCommandConflict
	}
	var value Run
	if err = json.Unmarshal(raw, &value); err != nil {
		return Run{}, false, err
	}
	return value, true, nil
}
func record(ctx context.Context, tx pgx.Tx, actor Actor, command, requestHash string, value Run) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO payroll_commands(tenant_id,command_id,request_hash,actor_subject_id,response_json,created_at) VALUES($1,$2,$3,$4,$5,$6)`, actor.TenantID, command, requestHash, actor.ID, raw, value.UpdatedAt)
	return err
}
func insertRevision(ctx context.Context, tx pgx.Tx, value Run, actor Actor, command, event string) error {
	_, err := tx.Exec(ctx, `INSERT INTO payroll_run_revisions(tenant_id,payroll_run_id,revision,command_id,event_kind,period,label,status,actor_subject_id,credential_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, value.TenantID, value.ID, value.Revision, command, event, value.Period, value.Label, value.Status, actor.ID, actor.CredentialID, value.UpdatedAt)
	return err
}
func insertAudit(ctx context.Context, tx pgx.Tx, value Run, actor Actor, command string, source int64, action string) error {
	_, err := tx.Exec(ctx, `INSERT INTO payroll_audit_events(id,tenant_id,payroll_run_id,command_id,source_revision,result_revision,action,actor_subject_id,credential_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, newUUID(), value.TenantID, value.ID, command, source, value.Revision, action, actor.ID, actor.CredentialID, value.UpdatedAt)
	return err
}

type scanner interface{ Scan(...any) error }

func scan(row scanner) (Run, error) {
	var value Run
	err := row.Scan(&value.ID, &value.TenantID, &value.Period, &value.Label, &value.Status, &value.Revision, &value.CreatedBySubjectID, &value.CreatedAt, &value.UpdatedAt)
	return value, err
}
func load(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, tenant, id, suffix string) (Run, error) {
	value, err := scan(q.QueryRow(ctx, `SELECT id,tenant_id,period,label,status,revision,created_by_subject_id,created_at,updated_at FROM payroll_runs WHERE tenant_id=$1 AND id=$2 `+suffix, tenant, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Run{}, ErrNotFound
	}
	return value, err
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
