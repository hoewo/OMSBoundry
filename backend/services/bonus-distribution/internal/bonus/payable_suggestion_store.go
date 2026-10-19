package bonus

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *PostgresStore) GetAllocation(ctx context.Context, tenantID, id string) (BonusAllocation, error) {
	var raw []byte
	if err := s.pool.QueryRow(ctx, `SELECT snapshot FROM bonus_allocations WHERE tenant_id=$1 AND id=$2`, tenantID, id).Scan(&raw); err != nil {
		return BonusAllocation{}, notFound(err)
	}
	var value BonusAllocation
	return value, json.Unmarshal(raw, &value)
}

func (s *PostgresStore) CreateSuggestion(ctx context.Context, actor Actor, value BonusPayableSuggestion, commandID, hash string) (BonusPayableSuggestion, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return BonusPayableSuggestion{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = lockCommand(ctx, tx, actor, commandID); err != nil {
		return BonusPayableSuggestion{}, err
	}
	var repeated BonusPayableSuggestion
	if found, replayErr := replay(ctx, tx, actor, commandID, hash, "bonus_payable_suggestion", &repeated); replayErr != nil || found {
		return repeated, replayErr
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return BonusPayableSuggestion{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO bonus_payable_suggestions(tenant_id,id,project_id,allocation_id,allocation_revision,allocation_digest,state,revision,target_revision,request_digest,snapshot,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, value.TenantID, value.ID, value.Source.Project.ID, value.Source.AllocationID, value.Source.AllocationRevision, value.Source.AllocationDigest, value.State, value.Revision, value.TargetRevision, value.RequestDigest, raw, value.CreatedAt, value.UpdatedAt); err != nil {
		return BonusPayableSuggestion{}, err
	}
	if err = insertBonusSuggestionRevision(ctx, tx, actor, commandID, value, "proposed", ""); err != nil {
		return BonusPayableSuggestion{}, err
	}
	if err = finishCommand(ctx, tx, actor, commandID, hash, "bonus_payable_suggestion", value, nil); err != nil {
		return BonusPayableSuggestion{}, err
	}
	return value, tx.Commit(ctx)
}

func (s *PostgresStore) GetSuggestion(ctx context.Context, tenantID, id string) (BonusPayableSuggestion, error) {
	var raw []byte
	if err := s.pool.QueryRow(ctx, `SELECT snapshot FROM bonus_payable_suggestions WHERE tenant_id=$1 AND id=$2`, tenantID, id).Scan(&raw); err != nil {
		return BonusPayableSuggestion{}, notFound(err)
	}
	var value BonusPayableSuggestion
	return value, json.Unmarshal(raw, &value)
}

func (s *PostgresStore) ListSuggestions(ctx context.Context, tenantID, projectID string) ([]BonusPayableSuggestion, error) {
	rows, err := s.pool.Query(ctx, `SELECT snapshot FROM bonus_payable_suggestions WHERE tenant_id=$1 AND project_id=$2 ORDER BY created_at,id`, tenantID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []BonusPayableSuggestion{}
	for rows.Next() {
		var raw []byte
		var value BonusPayableSuggestion
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

func (s *PostgresStore) SuggestionHistory(ctx context.Context, tenantID, id string) ([]BonusPayableSuggestionRevision, error) {
	rows, err := s.pool.Query(ctx, `SELECT revision,event_kind,command_id,snapshot,reason,actor_ref,occurred_at FROM bonus_payable_suggestion_revisions WHERE tenant_id=$1 AND suggestion_id=$2 ORDER BY revision`, tenantID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []BonusPayableSuggestionRevision{}
	for rows.Next() {
		var raw []byte
		var value BonusPayableSuggestionRevision
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

func (s *PostgresStore) ApplySuggestionTarget(ctx context.Context, actor Actor, id string, expectedRevision int64, commandID, hash, event, reason string, requested BonusPayableSuggestion) (BonusPayableSuggestion, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return BonusPayableSuggestion{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = lockCommand(ctx, tx, actor, commandID); err != nil {
		return BonusPayableSuggestion{}, err
	}
	var repeated BonusPayableSuggestion
	if found, replayErr := replay(ctx, tx, actor, commandID, hash, "bonus_payable_suggestion", &repeated); replayErr != nil || found {
		return repeated, replayErr
	}
	var raw []byte
	if err = tx.QueryRow(ctx, `SELECT snapshot FROM bonus_payable_suggestions WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, actor.TenantID, id).Scan(&raw); err != nil {
		return BonusPayableSuggestion{}, notFound(err)
	}
	var current BonusPayableSuggestion
	if err = json.Unmarshal(raw, &current); err != nil {
		return BonusPayableSuggestion{}, err
	}
	if current.Revision != expectedRevision {
		return BonusPayableSuggestion{}, ErrRevisionConflict
	}
	if requested.ID != current.ID || requested.TenantID != current.TenantID || requested.Source.AllocationID != current.Source.AllocationID || requested.Source.AllocationRevision != current.Source.AllocationRevision || requested.Source.AllocationDigest != current.Source.AllocationDigest || requested.RequestDigest != current.RequestDigest || requested.AmountMinor != current.AmountMinor || requested.Currency != current.Currency {
		return BonusPayableSuggestion{}, ErrReferenceInvalid
	}
	requested.Revision, requested.CreatedAt, requested.CreatedBy = current.Revision+1, current.CreatedAt, current.CreatedBy
	requested.UpdatedAt, requested.UpdatedBy = time.Now().UTC(), actor.ID
	raw, err = json.Marshal(requested)
	if err != nil {
		return BonusPayableSuggestion{}, err
	}
	if err = insertBonusSuggestionRevision(ctx, tx, actor, commandID, requested, event, reason); err != nil {
		return BonusPayableSuggestion{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE bonus_payable_suggestions SET state=$3,revision=$4,target_revision=$5,snapshot=$6,updated_at=$7 WHERE tenant_id=$1 AND id=$2`, requested.TenantID, requested.ID, requested.State, requested.Revision, requested.TargetRevision, raw, requested.UpdatedAt); err != nil {
		return BonusPayableSuggestion{}, err
	}
	if err = finishCommand(ctx, tx, actor, commandID, hash, "bonus_payable_suggestion", requested, nil); err != nil {
		return BonusPayableSuggestion{}, err
	}
	return requested, tx.Commit(ctx)
}

func insertBonusSuggestionRevision(ctx context.Context, tx pgx.Tx, actor Actor, commandID string, value BonusPayableSuggestion, event, reason string) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO bonus_payable_suggestion_revisions(tenant_id,suggestion_id,revision,event_kind,command_id,snapshot,reason,actor_ref,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, value.TenantID, value.ID, value.Revision, event, commandID, raw, reason, actor.ID, value.UpdatedAt)
	return err
}
