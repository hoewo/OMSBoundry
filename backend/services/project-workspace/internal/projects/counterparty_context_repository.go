package projects

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"time"
)

func loadCounterpartyContext(ctx context.Context, q commandQueryer, tenant, id string) (*CounterpartyContext, error) {
	var raw []byte
	err := q.QueryRow(ctx, `SELECT payload->'context' FROM project_history_events WHERE tenant_id=$1 AND project_id=$2 AND payload->>'schema_version'=$3 ORDER BY project_revision DESC,event_sequence DESC,id DESC LIMIT 1`, tenant, id, CounterpartyContextSchema).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var value CounterpartyContext
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	if value.SchemaVersion != CounterpartyContextSchema || value.Role != "client" || value.RecordedProjectRevision < 1 || value.Name == "" || value.RecordedAt.IsZero() {
		return nil, fmt.Errorf("invalid stored project counterparty context")
	}
	return &value, nil
}
func (s *PostgresStore) CounterpartyContext(ctx context.Context, tenant, id string) (CounterpartyContextView, error) {
	// One snapshot keeps the returned Project revision and context consistent.
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return CounterpartyContextView{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	p, err := loadProject(ctx, tx, tenant, id, "")
	if err != nil {
		return CounterpartyContextView{}, err
	}
	value, err := loadCounterpartyContext(ctx, tx, tenant, id)
	if err != nil {
		return CounterpartyContextView{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CounterpartyContextView{}, err
	}
	return CounterpartyContextView{ProjectID: p.ID, ProjectRevision: p.Revision, ProjectStatus: p.Status, Context: value}, nil
}
func (s *PostgresStore) ChangeCounterpartyContext(ctx context.Context, actor Actor, id string, input CounterpartyContextInput) (CounterpartyContextView, error) {
	in, err := NormalizeCounterpartyContext(input)
	if err != nil {
		return CounterpartyContextView{}, err
	}
	hash, err := requestHash(struct {
		Kind, ID string
		Input    CounterpartyContextInput
	}{CounterpartyContextSchema, id, in})
	if err != nil {
		return CounterpartyContextView{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CounterpartyContextView{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lockCommand(ctx, tx, actor.TenantID, in.CommandID); err != nil {
		return CounterpartyContextView{}, err
	}
	var result CounterpartyContextView
	if found, _, err := lookupCommand(ctx, tx, actor.TenantID, in.CommandID, hash, &result); err != nil {
		return result, err
	} else if found {
		return result, nil
	}
	p, err := loadProjectForUpdate(ctx, tx, actor.TenantID, id)
	if err != nil {
		return result, err
	}
	if p.Revision != in.ExpectedRevision {
		return result, ErrRevisionConflict
	}
	if p.Status != StatusActive {
		return result, ErrStateConflict
	}
	old, err := loadCounterpartyContext(ctx, tx, actor.TenantID, id)
	if err != nil {
		return result, err
	}
	now := time.Now().UTC()
	value := old
	if old == nil || old.CounterpartyContact != in.CounterpartyContact {
		p.Revision++
		p.UpdatedAt = now
		value = &CounterpartyContext{SchemaVersion: CounterpartyContextSchema, Role: "client", CounterpartyContact: in.CounterpartyContact, RecordedProjectRevision: p.Revision, RecordedAt: now}
		if err := updateProjectRevision(ctx, tx, p); err != nil {
			return result, err
		}
		payload := map[string]any{"schema_version": CounterpartyContextSchema, "previous_revision": in.ExpectedRevision, "reason": in.Reason, "context": value}
		if err := insertHistory(ctx, tx, p, actor, in.CommandID, "project_details_replaced", payload); err != nil {
			return result, err
		}
	}
	result = CounterpartyContextView{ProjectID: p.ID, ProjectRevision: p.Revision, ProjectStatus: p.Status, Context: value}
	if err := recordCommand(ctx, tx, actor.TenantID, in.CommandID, hash, 200, result, now); err != nil {
		return result, err
	}
	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}
