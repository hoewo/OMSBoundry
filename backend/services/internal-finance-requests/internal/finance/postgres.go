package finance

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
func (s *PostgresStore) Create(ctx context.Context, actor Actor, input CreateInput, requestHash string) (Request, error) {
	tx, err := s.begin(ctx, actor.TenantID, input.CommandID)
	if err != nil {
		return Request{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, replayErr := replay(ctx, tx, actor, input.CommandID, requestHash); replayErr != nil || found {
		return value, replayErr
	}
	now := time.Now().UTC()
	value := Request{ID: newUUID(), TenantID: actor.TenantID, ApplicantSubjectID: actor.ID, RequestType: input.RequestType, Title: input.Title, AmountMinor: input.AmountMinor, Currency: input.Currency, Status: "draft", Revision: 1, CreatedAt: now, UpdatedAt: now}
	if _, err = tx.Exec(ctx, `INSERT INTO finance_requests(id,tenant_id,applicant_subject_id,request_type,title,amount_minor,currency,status,revision,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, value.ID, value.TenantID, value.ApplicantSubjectID, value.RequestType, value.Title, value.AmountMinor, value.Currency, value.Status, value.Revision, value.CreatedAt, value.UpdatedAt); err != nil {
		return Request{}, err
	}
	if err = insertRevision(ctx, tx, value, actor, input.CommandID, "created"); err != nil {
		return Request{}, err
	}
	if err = insertAudit(ctx, tx, value, actor, input.CommandID, 0, "finance_request_created"); err != nil {
		return Request{}, err
	}
	if err = record(ctx, tx, actor, input.CommandID, requestHash, value); err != nil {
		return Request{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Request{}, err
	}
	return value, nil
}
func (s *PostgresStore) Update(ctx context.Context, actor Actor, id string, input UpdateInput, requestHash string) (Request, error) {
	tx, err := s.begin(ctx, actor.TenantID, input.CommandID)
	if err != nil {
		return Request{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, replayErr := replay(ctx, tx, actor, input.CommandID, requestHash); replayErr != nil || found {
		return value, replayErr
	}
	value, err := load(ctx, tx, actor.TenantID, id, "FOR UPDATE")
	if err != nil {
		return Request{}, err
	}
	if value.ApplicantSubjectID != actor.ID {
		return Request{}, ErrNotFound
	}
	if value.Revision != input.ExpectedRevision {
		return Request{}, ErrRevisionConflict
	}
	source := value.Revision
	value.Title = input.Title
	value.AmountMinor = input.AmountMinor
	value.Revision++
	value.UpdatedAt = time.Now().UTC()
	if _, err = tx.Exec(ctx, `UPDATE finance_requests SET title=$3,amount_minor=$4,revision=$5,updated_at=$6 WHERE tenant_id=$1 AND id=$2`, value.TenantID, value.ID, value.Title, value.AmountMinor, value.Revision, value.UpdatedAt); err != nil {
		return Request{}, err
	}
	if err = insertRevision(ctx, tx, value, actor, input.CommandID, "draft_updated"); err != nil {
		return Request{}, err
	}
	if err = insertAudit(ctx, tx, value, actor, input.CommandID, source, "finance_request_draft_updated"); err != nil {
		return Request{}, err
	}
	if err = record(ctx, tx, actor, input.CommandID, requestHash, value); err != nil {
		return Request{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Request{}, err
	}
	return value, nil
}
func (s *PostgresStore) Get(ctx context.Context, tenant, id string) (Request, error) {
	return load(ctx, s.pool, tenant, id, "")
}
func (s *PostgresStore) List(ctx context.Context, tenant string, filter ListFilter) ([]Request, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,tenant_id,applicant_subject_id,request_type,title,amount_minor,currency,status,revision,created_at,updated_at FROM finance_requests WHERE tenant_id=$1 AND ($2='' OR applicant_subject_id=$2::uuid) ORDER BY updated_at DESC,id`, tenant, filter.ApplicantSubjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []Request
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
func replay(ctx context.Context, tx pgx.Tx, actor Actor, command, requestHash string) (Request, bool, error) {
	var stored, storedActor string
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT request_hash,actor_subject_id::text,response_json FROM finance_request_commands WHERE tenant_id=$1 AND command_id=$2`, actor.TenantID, command).Scan(&stored, &storedActor, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Request{}, false, nil
	}
	if err != nil {
		return Request{}, false, err
	}
	if stored != requestHash || storedActor != actor.ID {
		return Request{}, false, ErrCommandConflict
	}
	var value Request
	if err = json.Unmarshal(raw, &value); err != nil {
		return Request{}, false, err
	}
	return value, true, nil
}
func record(ctx context.Context, tx pgx.Tx, actor Actor, command, requestHash string, value Request) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO finance_request_commands(tenant_id,command_id,request_hash,actor_subject_id,response_json,created_at) VALUES($1,$2,$3,$4,$5,$6)`, actor.TenantID, command, requestHash, actor.ID, raw, value.UpdatedAt)
	return err
}
func insertRevision(ctx context.Context, tx pgx.Tx, value Request, actor Actor, command, event string) error {
	_, err := tx.Exec(ctx, `INSERT INTO finance_request_revisions(tenant_id,request_id,revision,command_id,event_kind,applicant_subject_id,request_type,title,amount_minor,currency,status,actor_subject_id,credential_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, value.TenantID, value.ID, value.Revision, command, event, value.ApplicantSubjectID, value.RequestType, value.Title, value.AmountMinor, value.Currency, value.Status, actor.ID, actor.CredentialID, value.UpdatedAt)
	return err
}
func insertAudit(ctx context.Context, tx pgx.Tx, value Request, actor Actor, command string, source int64, action string) error {
	_, err := tx.Exec(ctx, `INSERT INTO finance_request_audit_events(id,tenant_id,request_id,command_id,source_revision,result_revision,action,actor_subject_id,credential_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, newUUID(), value.TenantID, value.ID, command, source, value.Revision, action, actor.ID, actor.CredentialID, value.UpdatedAt)
	return err
}

type scanner interface{ Scan(...any) error }

func scan(row scanner) (Request, error) {
	var value Request
	err := row.Scan(&value.ID, &value.TenantID, &value.ApplicantSubjectID, &value.RequestType, &value.Title, &value.AmountMinor, &value.Currency, &value.Status, &value.Revision, &value.CreatedAt, &value.UpdatedAt)
	return value, err
}
func load(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, tenant, id, suffix string) (Request, error) {
	value, err := scan(q.QueryRow(ctx, `SELECT id,tenant_id,applicant_subject_id,request_type,title,amount_minor,currency,status,revision,created_at,updated_at FROM finance_requests WHERE tenant_id=$1 AND id=$2 `+suffix, tenant, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Request{}, ErrNotFound
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
