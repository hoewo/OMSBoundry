package referencebonus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	core "omsbizfoundry/bonus-distribution/internal/bonus"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) (*PostgresStore, error) {
	if pool == nil {
		return nil, fmt.Errorf("pool is required")
	}
	return &PostgresStore{pool}, nil
}
func (s *PostgresStore) Get(ctx context.Context, tenant, id string) (Record, error) {
	var b []byte
	e := s.pool.QueryRow(ctx, `SELECT snapshot FROM reference_oa_bonus_allocations WHERE tenant_id=$1 AND id=$2`, tenant, id).Scan(&b)
	if errors.Is(e, pgx.ErrNoRows) {
		return Record{}, core.ErrNotFound
	}
	if e != nil {
		return Record{}, e
	}
	var v Record
	e = json.Unmarshal(b, &v)
	return v, e
}
func (s *PostgresStore) List(ctx context.Context, tenant, project, scope string) ([]Record, error) {
	rows, e := s.pool.Query(ctx, `SELECT snapshot FROM reference_oa_bonus_allocations WHERE tenant_id=$1 AND project_id=$2 AND scope=$3 ORDER BY id LIMIT 1001`, tenant, project, scope)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Record{}
	for rows.Next() {
		var b []byte
		var v Record
		if e = rows.Scan(&b); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(b, &v); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	if len(out) > 1000 {
		return nil, fmt.Errorf("invalid manual bonus list size; explicit pagination required")
	}
	return out, rows.Err()
}
func (s *PostgresStore) History(ctx context.Context, tenant, id string) ([]Revision, error) {
	rows, e := s.pool.Query(ctx, `SELECT snapshot,command_id::text,operation,reason FROM reference_oa_bonus_revisions WHERE tenant_id=$1 AND allocation_id=$2 ORDER BY revision DESC LIMIT 1001`, tenant, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Revision{}
	for rows.Next() {
		var b []byte
		var v Revision
		if e = rows.Scan(&b, &v.CommandID, &v.Operation, &v.Reason); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(b, &v.Snapshot); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	if len(out) > 1000 {
		return nil, fmt.Errorf("invalid manual bonus history size")
	}
	return out, rows.Err()
}

type querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func replay(ctx context.Context, q querier, a core.Actor, command, hash string) ([]Record, bool, error) {
	var actor, oldHash string
	var b []byte
	e := q.QueryRow(ctx, `SELECT actor_id::text,payload_hash,response FROM reference_oa_bonus_commands WHERE tenant_id=$1 AND command_id=$2`, a.TenantID, command).Scan(&actor, &oldHash, &b)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if e != nil {
		return nil, false, e
	}
	if actor != a.ID || oldHash != hash {
		return nil, false, core.ErrCommandConflict
	}
	var rows []Record
	e = json.Unmarshal(b, &rows)
	if e == nil && len(rows) == 0 {
		e = fmt.Errorf("invalid stored manual bonus response")
	}
	return rows, true, e
}
func (s *PostgresStore) Replay(ctx context.Context, a core.Actor, command, hash string) ([]Record, bool, error) {
	return replay(ctx, s.pool, a, command, hash)
}
func (s *PostgresStore) Commit(ctx context.Context, a core.Actor, command, hash, op, reason string, expected int64, values []Record) ([]Record, error) {
	if len(values) < 1 || len(values) > 100 || (op != "record" && op != "withdraw") || op == "withdraw" && len(values) != 1 {
		return nil, fmt.Errorf("invalid manual bonus commit")
	}
	tx, e := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(context.Background())
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.TenantID+":manual-bonus:"+command); e != nil {
		return nil, e
	}
	if rows, found, e := replay(ctx, tx, a, command, hash); e != nil || found {
		return rows, e
	}
	for _, v := range values {
		if expected > 0 {
			var b []byte
			if e = tx.QueryRow(ctx, `SELECT snapshot FROM reference_oa_bonus_allocations WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, a.TenantID, v.ID).Scan(&b); errors.Is(e, pgx.ErrNoRows) {
				return nil, core.ErrNotFound
			} else if e != nil {
				return nil, e
			}
			var old Record
			if e = json.Unmarshal(b, &old); e != nil {
				return nil, e
			}
			if old.State != "recorded" {
				return nil, core.ErrStateConflict
			}
			if old.Revision != expected {
				return nil, core.ErrRevisionConflict
			}
			check := v
			check.State = old.State
			check.Revision = old.Revision
			check.UpdatedAt = old.UpdatedAt
			check.UpdatedBy = old.UpdatedBy
			if !reflect.DeepEqual(check, old) {
				return nil, fmt.Errorf("invalid manual bonus immutable fields")
			}
		}
		b, e := json.Marshal(v)
		if e != nil {
			return nil, e
		}
		if _, e = tx.Exec(ctx, `INSERT INTO reference_oa_bonus_revisions(tenant_id,allocation_id,revision,command_id,operation,reason,snapshot) VALUES($1,$2,$3,$4,$5,$6,$7)`, a.TenantID, v.ID, v.Revision, command, op, reason, b); e != nil {
			return nil, e
		}
		if expected == 0 {
			_, e = tx.Exec(ctx, `INSERT INTO reference_oa_bonus_allocations(tenant_id,id,batch_id,project_id,scope,state,revision,snapshot) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, a.TenantID, v.ID, v.BatchID, v.Project.ID, v.Scope, v.State, v.Revision, b)
		} else {
			_, e = tx.Exec(ctx, `UPDATE reference_oa_bonus_allocations SET state=$3,revision=$4,snapshot=$5 WHERE tenant_id=$1 AND id=$2`, a.TenantID, v.ID, v.State, v.Revision, b)
		}
		if e != nil {
			return nil, e
		}
		if _, e = tx.Exec(ctx, `INSERT INTO reference_oa_bonus_audit(tenant_id,command_id,allocation_id,revision,actor_id,credential_id,operation,reason,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, a.TenantID, command, v.ID, v.Revision, a.ID, a.Credential, op, reason, v.UpdatedAt); e != nil {
			return nil, e
		}
	}
	b, e := json.Marshal(values)
	if e != nil {
		return nil, e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO reference_oa_bonus_commands(tenant_id,command_id,actor_id,payload_hash,response) VALUES($1,$2,$3,$4,$5)`, a.TenantID, command, a.ID, hash, b); e != nil {
		return nil, e
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, e
	}
	return values, nil
}
