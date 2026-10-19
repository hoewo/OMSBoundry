package referencecost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	core "omsbizfoundry/cost-outsourcing/internal/costoutsourcing"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) (*PostgresStore, error) {
	if pool == nil {
		return nil, fmt.Errorf("pool is required")
	}
	return &PostgresStore{pool}, nil
}
func (s *PostgresStore) Get(ctx context.Context, tenant, id string) (Record, error) {
	var bytes []byte
	err := s.pool.QueryRow(ctx, `SELECT snapshot FROM reference_oa_cost_entries WHERE tenant_id=$1 AND id=$2`, tenant, id).Scan(&bytes)
	if errors.Is(err, pgx.ErrNoRows) {
		return Record{}, core.ErrNotFound
	}
	if err != nil {
		return Record{}, err
	}
	var v Record
	err = json.Unmarshal(bytes, &v)
	return v, err
}
func (s *PostgresStore) List(ctx context.Context, tenant, project, scope string) ([]Record, error) {
	rows, err := s.pool.Query(ctx, `SELECT snapshot FROM reference_oa_cost_entries WHERE tenant_id=$1 AND project_id=$2 AND scope=$3 ORDER BY id LIMIT 1001`, tenant, project, scope)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Record{}
	for rows.Next() {
		var bytes []byte
		if err = rows.Scan(&bytes); err != nil {
			return nil, err
		}
		var v Record
		if err = json.Unmarshal(bytes, &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if len(out) > 1000 {
		return nil, fmt.Errorf("invalid cost list size; explicit pagination required")
	}
	return out, rows.Err()
}
func (s *PostgresStore) History(ctx context.Context, tenant, id string) ([]Revision, error) {
	rows, err := s.pool.Query(ctx, `SELECT snapshot,command_id::text,operation,reason FROM reference_oa_cost_revisions WHERE tenant_id=$1 AND cost_entry_id=$2 ORDER BY revision DESC LIMIT 1001`, tenant, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Revision{}
	for rows.Next() {
		var bytes []byte
		var v Revision
		if err = rows.Scan(&bytes, &v.CommandID, &v.Operation, &v.Reason); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(bytes, &v.Record); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if len(out) > 1000 {
		return nil, fmt.Errorf("invalid cost history size; explicit pagination required")
	}
	return out, rows.Err()
}

type querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func replay(ctx context.Context, q querier, a core.Actor, command, hash string) (Record, bool, error) {
	var actor, oldHash string
	var bytes []byte
	err := q.QueryRow(ctx, `SELECT actor_id::text,payload_hash,response FROM reference_oa_cost_commands WHERE tenant_id=$1 AND command_id=$2`, a.TenantID, command).Scan(&actor, &oldHash, &bytes)
	if errors.Is(err, pgx.ErrNoRows) {
		return Record{}, false, nil
	}
	if err != nil {
		return Record{}, false, err
	}
	if actor != a.ID || oldHash != hash {
		return Record{}, false, core.ErrCommandConflict
	}
	var v Record
	err = json.Unmarshal(bytes, &v)
	return v, true, err
}
func (s *PostgresStore) Replay(ctx context.Context, a core.Actor, command, hash string) (Record, bool, error) {
	return replay(ctx, s.pool, a, command, hash)
}
func (s *PostgresStore) Commit(ctx context.Context, a core.Actor, command, hash, op, reason string, expected int64, v Record) (Record, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Record{}, err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.TenantID+":"+command); err != nil {
		return Record{}, err
	}
	if old, found, err := replay(ctx, tx, a, command, hash); err != nil || found {
		return old, err
	}
	if expected > 0 {
		var bytes []byte
		if err = tx.QueryRow(ctx, `SELECT snapshot FROM reference_oa_cost_entries WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, a.TenantID, v.ID).Scan(&bytes); errors.Is(err, pgx.ErrNoRows) {
			return Record{}, core.ErrNotFound
		} else if err != nil {
			return Record{}, err
		}
		var old Record
		if err = json.Unmarshal(bytes, &old); err != nil {
			return Record{}, err
		}
		if old.State != "recorded" {
			return Record{}, core.ErrWithdrawn
		}
		if old.Revision != expected {
			return Record{}, core.ErrRevisionConflict
		}
		if old.Scope != v.Scope || old.Project != v.Project || old.ID != v.ID || old.TenantID != v.TenantID || old.CreatedBy != v.CreatedBy || !old.CreatedAt.Equal(v.CreatedAt) {
			return Record{}, fmt.Errorf("invalid cost immutable fields")
		}
	}
	bytes, err := json.Marshal(v)
	if err != nil {
		return Record{}, err
	}
	if expected == 0 {
		_, err = tx.Exec(ctx, `INSERT INTO reference_oa_cost_entries(tenant_id,id,project_id,scope,state,revision,snapshot) VALUES($1,$2,$3,$4,$5,$6,$7)`, a.TenantID, v.ID, v.Project.ID, v.Scope, v.State, v.Revision, bytes)
	} else {
		_, err = tx.Exec(ctx, `UPDATE reference_oa_cost_entries SET state=$3,revision=$4,snapshot=$5 WHERE tenant_id=$1 AND id=$2`, a.TenantID, v.ID, v.State, v.Revision, bytes)
	}
	if err != nil {
		return Record{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO reference_oa_cost_revisions(tenant_id,cost_entry_id,revision,command_id,operation,reason,snapshot) VALUES($1,$2,$3,$4,$5,$6,$7)`, a.TenantID, v.ID, v.Revision, command, op, reason, bytes); err != nil {
		return Record{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO reference_oa_cost_commands(tenant_id,command_id,actor_id,payload_hash,response) VALUES($1,$2,$3,$4,$5)`, a.TenantID, command, a.ID, hash, bytes); err != nil {
		return Record{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO reference_oa_cost_audit(tenant_id,command_id,cost_entry_id,revision,actor_id,credential_id,operation,reason,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, a.TenantID, command, v.ID, v.Revision, a.ID, a.CredentialID, op, reason, v.UpdatedAt); err != nil {
		return Record{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Record{}, err
	}
	return v, nil
}
