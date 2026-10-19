package costoutsourcing

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *PostgresStore) CreateSuggestion(ctx context.Context, actor Actor, value PayableSuggestion, commandID, hash string) (PayableSuggestion, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return PayableSuggestion{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = lockCommand(ctx, tx, actor, commandID); err != nil {
		return PayableSuggestion{}, err
	}
	var repeated PayableSuggestion
	if found, replayErr := replay(ctx, tx, actor, commandID, hash, "payable_suggestion", &repeated); replayErr != nil || found {
		return repeated, replayErr
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return PayableSuggestion{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO payable_suggestions(tenant_id,id,project_id,source_type,source_id,source_revision,state,revision,target_revision,request_digest,snapshot,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, value.TenantID, value.ID, value.Project.ID, value.Source.Type, value.Source.ID, value.Source.Revision, value.State, value.Revision, value.TargetRevision, value.RequestDigest, raw, value.CreatedAt, value.UpdatedAt); err != nil {
		return PayableSuggestion{}, err
	}
	if err = insertSuggestionRevision(ctx, tx, actor, commandID, value, "proposed", ""); err != nil {
		return PayableSuggestion{}, err
	}
	if err = finishCommand(ctx, tx, actor, commandID, hash, "payable_suggestion", value, value.ID, value.Revision, "proposed"); err != nil {
		return PayableSuggestion{}, err
	}
	return value, tx.Commit(ctx)
}

func (s *PostgresStore) GetSuggestion(ctx context.Context, tenantID, id string) (PayableSuggestion, error) {
	var raw []byte
	if err := s.pool.QueryRow(ctx, `SELECT snapshot FROM payable_suggestions WHERE tenant_id=$1 AND id=$2`, tenantID, id).Scan(&raw); err != nil {
		return PayableSuggestion{}, notFound(err)
	}
	var value PayableSuggestion
	return value, json.Unmarshal(raw, &value)
}

func (s *PostgresStore) ListSuggestions(ctx context.Context, tenantID, projectID string) ([]PayableSuggestion, error) {
	rows, err := s.pool.Query(ctx, `SELECT snapshot FROM payable_suggestions WHERE tenant_id=$1 AND project_id=$2 ORDER BY created_at,id`, tenantID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []PayableSuggestion{}
	for rows.Next() {
		var raw []byte
		var value PayableSuggestion
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

func (s *PostgresStore) SuggestionHistory(ctx context.Context, tenantID, id string) ([]PayableSuggestionRevision, error) {
	rows, err := s.pool.Query(ctx, `SELECT revision,event_kind,command_id,snapshot,reason,actor_ref,occurred_at FROM payable_suggestion_revisions WHERE tenant_id=$1 AND suggestion_id=$2 ORDER BY revision`, tenantID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []PayableSuggestionRevision{}
	for rows.Next() {
		var raw []byte
		var value PayableSuggestionRevision
		value.TenantID, value.SuggestionID = tenantID, id
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

func (s *PostgresStore) ApplySuggestionTarget(ctx context.Context, actor Actor, id string, expectedRevision int64, commandID, hash, event, reason string, requested PayableSuggestion) (PayableSuggestion, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return PayableSuggestion{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = lockCommand(ctx, tx, actor, commandID); err != nil {
		return PayableSuggestion{}, err
	}
	var repeated PayableSuggestion
	if found, replayErr := replay(ctx, tx, actor, commandID, hash, "payable_suggestion", &repeated); replayErr != nil || found {
		return repeated, replayErr
	}
	var raw []byte
	if err = tx.QueryRow(ctx, `SELECT snapshot FROM payable_suggestions WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, actor.TenantID, id).Scan(&raw); err != nil {
		return PayableSuggestion{}, notFound(err)
	}
	var current PayableSuggestion
	if err = json.Unmarshal(raw, &current); err != nil {
		return PayableSuggestion{}, err
	}
	if current.Revision != expectedRevision {
		return PayableSuggestion{}, ErrRevisionConflict
	}
	if requested.ID != current.ID || requested.TenantID != current.TenantID || requested.Project != current.Project || requested.Source != current.Source || requested.LedgerOwner != current.LedgerOwner || requested.Counterparty != current.Counterparty || requested.AmountMinor != current.AmountMinor || requested.Currency != current.Currency || requested.DueOn != current.DueOn || requested.Purpose != current.Purpose || requested.RequestDigest != current.RequestDigest {
		return PayableSuggestion{}, ErrReferenceInvalid
	}
	requested.Revision = current.Revision + 1
	requested.CreatedBy, requested.CreatedAt = current.CreatedBy, current.CreatedAt
	requested.UpdatedBy, requested.UpdatedAt = actor.ID, time.Now().UTC()
	raw, err = json.Marshal(requested)
	if err != nil {
		return PayableSuggestion{}, err
	}
	if err = insertSuggestionRevision(ctx, tx, actor, commandID, requested, event, reason); err != nil {
		return PayableSuggestion{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE payable_suggestions SET state=$3,revision=$4,target_revision=$5,snapshot=$6,updated_at=$7 WHERE tenant_id=$1 AND id=$2`, requested.TenantID, requested.ID, requested.State, requested.Revision, requested.TargetRevision, raw, requested.UpdatedAt); err != nil {
		return PayableSuggestion{}, err
	}
	if err = finishCommand(ctx, tx, actor, commandID, hash, "payable_suggestion", requested, requested.ID, requested.Revision, event); err != nil {
		return PayableSuggestion{}, err
	}
	return requested, tx.Commit(ctx)
}

func insertSuggestionRevision(ctx context.Context, tx pgx.Tx, actor Actor, commandID string, value PayableSuggestion, event, reason string) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO payable_suggestion_revisions(tenant_id,suggestion_id,revision,event_kind,command_id,snapshot,reason,actor_ref,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, value.TenantID, value.ID, value.Revision, event, commandID, raw, reason, actor.ID, value.UpdatedAt)
	return err
}
