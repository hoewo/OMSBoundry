package bonus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
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

func (s *PostgresStore) Replay(ctx context.Context, actor Actor, commandID, hash, responseType string, destination any) (bool, error) {
	return replay(ctx, s.pool, actor, commandID, hash, responseType, destination)
}

func (s *PostgresStore) CreateDraft(ctx context.Context, actor Actor, value BonusPool, commandID, hash string) (BonusPool, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return BonusPool{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = lockCommand(ctx, tx, actor, commandID); err != nil {
		return BonusPool{}, err
	}
	var repeated BonusPool
	if found, replayErr := replay(ctx, tx, actor, commandID, hash, "bonus_pool", &repeated); replayErr != nil || found {
		return repeated, replayErr
	}
	now := time.Now().UTC()
	value.CreatedAt, value.UpdatedAt = now, now
	for i := range value.Allocations {
		value.Allocations[i].CreatedAt, value.Allocations[i].UpdatedAt = now, now
	}
	poolRaw, err := json.Marshal(value)
	if err != nil {
		return BonusPool{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO bonus_pools(tenant_id,id,project_id,bonus_type,state,currency,amount_minor,unallocated_minor,revision,snapshot,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, value.TenantID, value.ID, value.Project.ID, value.BonusType, value.State, value.Currency, value.AmountMinor, value.UnallocatedMinor, value.Revision, poolRaw, now, now); err != nil {
		return BonusPool{}, err
	}
	for _, allocation := range value.Allocations {
		allocationRaw, marshalErr := json.Marshal(allocation)
		if marshalErr != nil {
			return BonusPool{}, marshalErr
		}
		if _, err = tx.Exec(ctx, `INSERT INTO bonus_allocations(tenant_id,id,pool_id,project_id,bonus_type,person_id,assignment_id,state,currency,amount_minor,ratio_basis_points,pool_revision,revision,snapshot,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`, allocation.TenantID, allocation.ID, allocation.PoolID, allocation.ProjectID, allocation.BonusType, allocation.Person.ID, allocation.Assignment.ID, allocation.State, allocation.Currency, allocation.AmountMinor, allocation.RatioBasisPoints, allocation.PoolRevision, allocation.Revision, allocationRaw, now, now); err != nil {
			return BonusPool{}, err
		}
		if err = insertAllocationRevision(ctx, tx, actor, commandID, allocation, "drafted", ""); err != nil {
			return BonusPool{}, err
		}
	}
	if err = insertPoolRevision(ctx, tx, actor, commandID, value, "drafted", ""); err != nil {
		return BonusPool{}, err
	}
	if err = finishCommand(ctx, tx, actor, commandID, hash, "bonus_pool", value, []auditRecord{{value.ID, value.Revision, "drafted", now}}); err != nil {
		return BonusPool{}, err
	}
	return value, tx.Commit(ctx)
}

func (s *PostgresStore) GetPool(ctx context.Context, tenantID, id string) (BonusPool, error) {
	var raw []byte
	if err := s.pool.QueryRow(ctx, `SELECT snapshot FROM bonus_pools WHERE tenant_id=$1 AND id=$2`, tenantID, id).Scan(&raw); err != nil {
		return BonusPool{}, notFound(err)
	}
	var value BonusPool
	return value, json.Unmarshal(raw, &value)
}

func (s *PostgresStore) ListPools(ctx context.Context, tenantID, projectID string) ([]BonusPool, error) {
	rows, err := s.pool.Query(ctx, `SELECT snapshot FROM bonus_pools WHERE tenant_id=$1 AND project_id=$2 ORDER BY created_at,id`, tenantID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []BonusPool
	for rows.Next() {
		var raw []byte
		var value BonusPool
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *PostgresStore) PoolHistory(ctx context.Context, tenantID, id string) ([]PoolRevision, error) {
	rows, err := s.pool.Query(ctx, `SELECT revision,event_kind,command_id,snapshot,reason,actor_ref,occurred_at FROM bonus_pool_revisions WHERE tenant_id=$1 AND pool_id=$2 ORDER BY revision`, tenantID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []PoolRevision
	for rows.Next() {
		var value PoolRevision
		var raw []byte
		value.TenantID, value.PoolID = tenantID, id
		if err = rows.Scan(&value.Revision, &value.EventKind, &value.CommandID, &raw, &value.Reason, &value.ActorRef, &value.OccurredAt); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &value.Snapshot); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, ErrNotFound
	}
	return values, nil
}

func (s *PostgresStore) AllocationHistory(ctx context.Context, tenantID, id string) ([]AllocationRevision, error) {
	rows, err := s.pool.Query(ctx, `SELECT pool_id,revision,event_kind,command_id,snapshot,reason,actor_ref,occurred_at FROM bonus_allocation_revisions WHERE tenant_id=$1 AND allocation_id=$2 ORDER BY revision`, tenantID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []AllocationRevision
	for rows.Next() {
		var value AllocationRevision
		var raw []byte
		value.TenantID, value.AllocationID = tenantID, id
		if err = rows.Scan(&value.PoolID, &value.Revision, &value.EventKind, &value.CommandID, &raw, &value.Reason, &value.ActorRef, &value.OccurredAt); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &value.Snapshot); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, ErrNotFound
	}
	return values, nil
}

func (s *PostgresStore) Publish(ctx context.Context, actor Actor, id string, input PoolCommandInput, hash string) (BonusPool, error) {
	return s.mutateOne(ctx, actor, id, input, hash, StateDraft, StatePublished, "published")
}

func (s *PostgresStore) Withdraw(ctx context.Context, actor Actor, id string, input PoolCommandInput, hash string) (BonusPool, error) {
	return s.mutateOne(ctx, actor, id, input, hash, "", StateWithdrawn, "withdrawn")
}

func (s *PostgresStore) mutateOne(ctx context.Context, actor Actor, id string, input PoolCommandInput, hash, requiredState, nextState, event string) (BonusPool, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return BonusPool{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = lockCommand(ctx, tx, actor, input.CommandID); err != nil {
		return BonusPool{}, err
	}
	var repeated BonusPool
	if found, replayErr := replay(ctx, tx, actor, input.CommandID, hash, "bonus_pool", &repeated); replayErr != nil || found {
		return repeated, replayErr
	}
	value, err := loadPoolForUpdate(ctx, tx, actor.TenantID, id)
	if err != nil {
		return BonusPool{}, err
	}
	if value.Revision != input.ExpectedRevision {
		return BonusPool{}, ErrRevisionConflict
	}
	if requiredState != "" && value.State != requiredState {
		return BonusPool{}, ErrStateConflict
	}
	if requiredState == "" && value.State != StateDraft && value.State != StatePublished {
		return BonusPool{}, ErrStateConflict
	}
	now := time.Now().UTC()
	value.State, value.Revision, value.UpdatedBy, value.UpdatedAt = nextState, value.Revision+1, actor.ID, now
	for i := range value.Allocations {
		value.Allocations[i].State = nextState
		value.Allocations[i].PoolRevision = value.Revision
		value.Allocations[i].Revision++
		value.Allocations[i].UpdatedAt = now
	}
	if err = persistMutation(ctx, tx, actor, input.CommandID, input.Reason, event, value); err != nil {
		return BonusPool{}, err
	}
	if err = finishCommand(ctx, tx, actor, input.CommandID, hash, "bonus_pool", value, []auditRecord{{value.ID, value.Revision, event, now}}); err != nil {
		return BonusPool{}, err
	}
	return value, tx.Commit(ctx)
}

func (s *PostgresStore) Supersede(ctx context.Context, actor Actor, input SupersedeInput, hash string) (SupersessionResult, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return SupersessionResult{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = lockCommand(ctx, tx, actor, input.CommandID); err != nil {
		return SupersessionResult{}, err
	}
	var repeated SupersessionResult
	if found, replayErr := replay(ctx, tx, actor, input.CommandID, hash, "bonus_supersession", &repeated); replayErr != nil || found {
		return repeated, replayErr
	}
	ids := []string{input.PublishedPoolID, input.ReplacementPoolID}
	sort.Strings(ids)
	locked := map[string]BonusPool{}
	for _, id := range ids {
		value, loadErr := loadPoolForUpdate(ctx, tx, actor.TenantID, id)
		if loadErr != nil {
			return SupersessionResult{}, loadErr
		}
		locked[id] = value
	}
	published, replacement := locked[input.PublishedPoolID], locked[input.ReplacementPoolID]
	if published.Revision != input.ExpectedPublishedRevision || replacement.Revision != input.ExpectedReplacementRevision {
		return SupersessionResult{}, ErrRevisionConflict
	}
	if published.State != StatePublished || replacement.State != StateDraft || published.Project.ID != replacement.Project.ID || published.BonusType != replacement.BonusType {
		return SupersessionResult{}, ErrStateConflict
	}
	now := time.Now().UTC()
	published.SupersededBy = &SupersededReference{PoolID: replacement.ID, Revision: replacement.Revision + 1}
	replacement.Supersedes = &SupersededReference{PoolID: published.ID, Revision: published.Revision}
	published.State, published.Revision, published.UpdatedBy, published.UpdatedAt = StateSuperseded, published.Revision+1, actor.ID, now
	replacement.State, replacement.Revision, replacement.UpdatedBy, replacement.UpdatedAt = StatePublished, replacement.Revision+1, actor.ID, now
	for i := range published.Allocations {
		published.Allocations[i].State = StateSuperseded
		published.Allocations[i].PoolRevision = published.Revision
		published.Allocations[i].Revision++
		published.Allocations[i].UpdatedAt = now
	}
	for i := range replacement.Allocations {
		replacement.Allocations[i].State = StatePublished
		replacement.Allocations[i].PoolRevision = replacement.Revision
		replacement.Allocations[i].Revision++
		replacement.Allocations[i].UpdatedAt = now
	}
	if err = persistMutation(ctx, tx, actor, input.CommandID, input.Reason, "superseded", published); err != nil {
		return SupersessionResult{}, err
	}
	if err = persistMutation(ctx, tx, actor, input.CommandID, input.Reason, "published", replacement); err != nil {
		return SupersessionResult{}, err
	}
	result := SupersessionResult{Superseded: published, Published: replacement}
	if err = finishCommand(ctx, tx, actor, input.CommandID, hash, "bonus_supersession", result, []auditRecord{{published.ID, published.Revision, "superseded", now}, {replacement.ID, replacement.Revision, "published", now}}); err != nil {
		return SupersessionResult{}, err
	}
	return result, tx.Commit(ctx)
}

func loadPoolForUpdate(ctx context.Context, tx pgx.Tx, tenantID, id string) (BonusPool, error) {
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT snapshot FROM bonus_pools WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, tenantID, id).Scan(&raw); err != nil {
		return BonusPool{}, notFound(err)
	}
	var value BonusPool
	return value, json.Unmarshal(raw, &value)
}

func persistMutation(ctx context.Context, tx pgx.Tx, actor Actor, commandID, reason, event string, value BonusPool) error {
	for _, allocation := range value.Allocations {
		if err := insertAllocationRevision(ctx, tx, actor, commandID, allocation, event, reason); err != nil {
			return err
		}
		raw, err := json.Marshal(allocation)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE bonus_allocations SET state=$3,amount_minor=$4,ratio_basis_points=$5,pool_revision=$6,revision=$7,snapshot=$8,updated_at=$9 WHERE tenant_id=$1 AND id=$2`, allocation.TenantID, allocation.ID, allocation.State, allocation.AmountMinor, allocation.RatioBasisPoints, allocation.PoolRevision, allocation.Revision, raw, allocation.UpdatedAt); err != nil {
			return err
		}
	}
	if err := insertPoolRevision(ctx, tx, actor, commandID, value, event, reason); err != nil {
		return err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE bonus_pools SET state=$3,currency=$4,amount_minor=$5,unallocated_minor=$6,revision=$7,snapshot=$8,updated_at=$9 WHERE tenant_id=$1 AND id=$2`, value.TenantID, value.ID, value.State, value.Currency, value.AmountMinor, value.UnallocatedMinor, value.Revision, raw, value.UpdatedAt)
	return err
}

type queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func replay(ctx context.Context, db queryer, actor Actor, commandID, hash, responseType string, destination any) (bool, error) {
	var storedActor, storedHash, storedType string
	var raw []byte
	err := db.QueryRow(ctx, `SELECT actor_ref::text,request_hash,response_type,response FROM bonus_distribution_commands WHERE tenant_id=$1 AND command_id=$2`, actor.TenantID, commandID).Scan(&storedActor, &storedHash, &storedType, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if storedActor != actor.ID || storedHash != hash || storedType != responseType {
		return false, ErrCommandConflict
	}
	return true, json.Unmarshal(raw, destination)
}

func lockCommand(ctx context.Context, tx pgx.Tx, actor Actor, commandID string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, actor.TenantID+":"+commandID)
	return err
}

type auditRecord struct {
	aggregateID string
	revision    int64
	event       string
	at          time.Time
}

func finishCommand(ctx context.Context, tx pgx.Tx, actor Actor, commandID, hash, responseType string, response any, audits []auditRecord) error {
	raw, err := json.Marshal(response)
	if err != nil {
		return err
	}
	createdAt := time.Now().UTC()
	if len(audits) > 0 {
		createdAt = audits[0].at
	}
	if _, err = tx.Exec(ctx, `INSERT INTO bonus_distribution_commands(tenant_id,command_id,actor_ref,request_hash,response_type,response,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, actor.TenantID, commandID, actor.ID, hash, responseType, raw, createdAt); err != nil {
		return err
	}
	for _, audit := range audits {
		eventID, idErr := newUUID()
		if idErr != nil {
			return idErr
		}
		if _, err = tx.Exec(ctx, `INSERT INTO bonus_distribution_audit_events(tenant_id,event_id,aggregate_type,aggregate_id,aggregate_revision,command_id,actor_ref,event_kind,occurred_at) VALUES($1,$2,'bonus_pool',$3,$4,$5,$6,$7,$8)`, actor.TenantID, eventID, audit.aggregateID, audit.revision, commandID, actor.ID, audit.event, audit.at); err != nil {
			return err
		}
	}
	return nil
}

func insertPoolRevision(ctx context.Context, tx pgx.Tx, actor Actor, commandID string, value BonusPool, event, reason string) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO bonus_pool_revisions(tenant_id,pool_id,revision,event_kind,command_id,snapshot,reason,actor_ref,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, value.TenantID, value.ID, value.Revision, event, commandID, raw, reason, actor.ID, value.UpdatedAt)
	return err
}

func insertAllocationRevision(ctx context.Context, tx pgx.Tx, actor Actor, commandID string, value BonusAllocation, event, reason string) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO bonus_allocation_revisions(tenant_id,allocation_id,pool_id,revision,event_kind,command_id,snapshot,reason,actor_ref,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, value.TenantID, value.ID, value.PoolID, value.Revision, event, commandID, raw, reason, actor.ID, value.UpdatedAt)
	return err
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
