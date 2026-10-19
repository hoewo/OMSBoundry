package costoutsourcing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func (s *PostgresStore) RecordCost(ctx context.Context, actor Actor, input RecordCostInput, project ProjectSnapshot, evidence []EvidenceSnapshot, hash string) (CostEntry, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CostEntry{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = lockCommand(ctx, tx, actor, input.CommandID); err != nil {
		return CostEntry{}, err
	}
	var repeated CostEntry
	if found, replayErr := replay(ctx, tx, actor, input.CommandID, hash, "cost_entry", &repeated); replayErr != nil || found {
		return repeated, replayErr
	}
	id, err := newUUID()
	if err != nil {
		return CostEntry{}, err
	}
	now := time.Now().UTC()
	value := CostEntry{ID: id, TenantID: actor.TenantID, Project: project, Category: input.Category, OccurredOn: input.OccurredOn, Currency: input.Currency, AmountMinor: input.AmountMinor, DistanceMeters: input.DistanceMeters, Description: input.Description, Evidence: evidence, State: StateRecorded, Revision: 1, CreatedBy: actor.ID, UpdatedBy: actor.ID, CreatedAt: now, UpdatedAt: now}
	raw, _ := json.Marshal(value)
	if _, err = tx.Exec(ctx, `INSERT INTO cost_entries(tenant_id,id,project_id,state,currency,amount_minor,revision,snapshot,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, value.TenantID, value.ID, value.Project.ID, value.State, value.Currency, value.AmountMinor, value.Revision, raw, value.CreatedAt, value.UpdatedAt); err != nil {
		return CostEntry{}, err
	}
	if err = insertCostRevision(ctx, tx, actor, input.CommandID, value, "recorded", ""); err != nil {
		return CostEntry{}, err
	}
	if err = finishCommand(ctx, tx, actor, input.CommandID, hash, "cost_entry", value, value.ID, value.Revision, "recorded"); err != nil {
		return CostEntry{}, err
	}
	return value, tx.Commit(ctx)
}

func (s *PostgresStore) GetCost(ctx context.Context, tenantID, id string) (CostEntry, error) {
	var raw []byte
	if err := s.pool.QueryRow(ctx, `SELECT snapshot FROM cost_entries WHERE tenant_id=$1 AND id=$2`, tenantID, id).Scan(&raw); err != nil {
		return CostEntry{}, notFound(err)
	}
	var value CostEntry
	return value, json.Unmarshal(raw, &value)
}

func (s *PostgresStore) ListCosts(ctx context.Context, tenantID, projectID string) ([]CostEntry, error) {
	rows, err := s.pool.Query(ctx, `SELECT snapshot FROM cost_entries WHERE tenant_id=$1 AND project_id=$2 ORDER BY created_at,id`, tenantID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []CostEntry
	for rows.Next() {
		var raw []byte
		var value CostEntry
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

func (s *PostgresStore) CostHistory(ctx context.Context, tenantID, id string) ([]CostRevision, error) {
	rows, err := s.pool.Query(ctx, `SELECT revision,event_kind,command_id,snapshot,reason,actor_ref,occurred_at FROM cost_entry_revisions WHERE tenant_id=$1 AND cost_entry_id=$2 ORDER BY revision`, tenantID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []CostRevision
	for rows.Next() {
		var value CostRevision
		var raw []byte
		value.TenantID, value.CostEntryID = tenantID, id
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

func (s *PostgresStore) CorrectCost(ctx context.Context, actor Actor, id string, input CorrectCostInput, evidence []EvidenceSnapshot, hash string) (CostEntry, error) {
	return s.mutateCost(ctx, actor, id, input.CommandID, hash, input.ExpectedRevision, "corrected", input.Reason, func(value *CostEntry) {
		value.Category, value.OccurredOn, value.Currency = input.Category, input.OccurredOn, input.Currency
		value.AmountMinor, value.DistanceMeters, value.Description = input.AmountMinor, input.DistanceMeters, input.Description
		value.Evidence = evidence
	})
}

func (s *PostgresStore) WithdrawCost(ctx context.Context, actor Actor, id string, input WithdrawInput, hash string) (CostEntry, error) {
	return s.mutateCost(ctx, actor, id, input.CommandID, hash, input.ExpectedRevision, "withdrawn", input.Reason, func(value *CostEntry) { value.State = StateWithdrawn })
}

func (s *PostgresStore) mutateCost(ctx context.Context, actor Actor, id, commandID, hash string, expected int64, event, reason string, change func(*CostEntry)) (CostEntry, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CostEntry{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = lockCommand(ctx, tx, actor, commandID); err != nil {
		return CostEntry{}, err
	}
	var repeated CostEntry
	if found, replayErr := replay(ctx, tx, actor, commandID, hash, "cost_entry", &repeated); replayErr != nil || found {
		return repeated, replayErr
	}
	var raw []byte
	if err = tx.QueryRow(ctx, `SELECT snapshot FROM cost_entries WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, actor.TenantID, id).Scan(&raw); err != nil {
		return CostEntry{}, notFound(err)
	}
	var value CostEntry
	if err = json.Unmarshal(raw, &value); err != nil {
		return CostEntry{}, err
	}
	if value.State == StateWithdrawn {
		return CostEntry{}, ErrWithdrawn
	}
	if value.Revision != expected {
		return CostEntry{}, ErrRevisionConflict
	}
	change(&value)
	value.Revision++
	value.UpdatedBy = actor.ID
	value.UpdatedAt = time.Now().UTC()
	raw, _ = json.Marshal(value)
	if err = insertCostRevision(ctx, tx, actor, commandID, value, event, reason); err != nil {
		return CostEntry{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE cost_entries SET state=$3,currency=$4,amount_minor=$5,revision=$6,snapshot=$7,updated_at=$8 WHERE tenant_id=$1 AND id=$2`, value.TenantID, value.ID, value.State, value.Currency, value.AmountMinor, value.Revision, raw, value.UpdatedAt); err != nil {
		return CostEntry{}, err
	}
	if err = finishCommand(ctx, tx, actor, commandID, hash, "cost_entry", value, value.ID, value.Revision, event); err != nil {
		return CostEntry{}, err
	}
	return value, tx.Commit(ctx)
}

func (s *PostgresStore) RecordEngagement(ctx context.Context, actor Actor, input RecordEngagementInput, project ProjectSnapshot, provider PartySnapshot, contract *ContractSnapshot, evidence []EvidenceSnapshot, hash string) (Engagement, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Engagement{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = lockCommand(ctx, tx, actor, input.CommandID); err != nil {
		return Engagement{}, err
	}
	var repeated Engagement
	if found, replayErr := replay(ctx, tx, actor, input.CommandID, hash, "outsourcing_engagement", &repeated); replayErr != nil || found {
		return repeated, replayErr
	}
	id, err := newUUID()
	if err != nil {
		return Engagement{}, err
	}
	now := time.Now().UTC()
	value := Engagement{ID: id, TenantID: actor.TenantID, Project: project, Provider: provider, ContactName: input.ContactName, ContactPhone: input.ContactPhone, Scope: input.Scope, Currency: input.Currency, AmountMinor: input.AmountMinor, Contract: contract, Evidence: evidence, Note: input.Note, State: StateRecorded, Revision: 1, CreatedBy: actor.ID, UpdatedBy: actor.ID, CreatedAt: now, UpdatedAt: now}
	raw, _ := json.Marshal(value)
	if _, err = tx.Exec(ctx, `INSERT INTO outsourcing_engagements(tenant_id,id,project_id,provider_party_id,state,currency,amount_minor,revision,snapshot,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, value.TenantID, value.ID, value.Project.ID, value.Provider.ID, value.State, value.Currency, value.AmountMinor, value.Revision, raw, value.CreatedAt, value.UpdatedAt); err != nil {
		return Engagement{}, err
	}
	if err = insertEngagementRevision(ctx, tx, actor, input.CommandID, value, "recorded", ""); err != nil {
		return Engagement{}, err
	}
	if err = finishCommand(ctx, tx, actor, input.CommandID, hash, "outsourcing_engagement", value, value.ID, value.Revision, "recorded"); err != nil {
		return Engagement{}, err
	}
	return value, tx.Commit(ctx)
}

func (s *PostgresStore) GetEngagement(ctx context.Context, tenantID, id string) (Engagement, error) {
	var raw []byte
	if err := s.pool.QueryRow(ctx, `SELECT snapshot FROM outsourcing_engagements WHERE tenant_id=$1 AND id=$2`, tenantID, id).Scan(&raw); err != nil {
		return Engagement{}, notFound(err)
	}
	var value Engagement
	return value, json.Unmarshal(raw, &value)
}

func (s *PostgresStore) ListEngagements(ctx context.Context, tenantID, projectID string) ([]Engagement, error) {
	rows, err := s.pool.Query(ctx, `SELECT snapshot FROM outsourcing_engagements WHERE tenant_id=$1 AND project_id=$2 ORDER BY created_at,id`, tenantID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []Engagement
	for rows.Next() {
		var raw []byte
		var value Engagement
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

func (s *PostgresStore) EngagementHistory(ctx context.Context, tenantID, id string) ([]EngagementRevision, error) {
	rows, err := s.pool.Query(ctx, `SELECT revision,event_kind,command_id,snapshot,reason,actor_ref,occurred_at FROM outsourcing_engagement_revisions WHERE tenant_id=$1 AND engagement_id=$2 ORDER BY revision`, tenantID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []EngagementRevision
	for rows.Next() {
		var value EngagementRevision
		var raw []byte
		value.TenantID, value.EngagementID = tenantID, id
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

func (s *PostgresStore) CorrectEngagement(ctx context.Context, actor Actor, id string, input CorrectEngagementInput, provider PartySnapshot, contract *ContractSnapshot, evidence []EvidenceSnapshot, hash string) (Engagement, error) {
	return s.mutateEngagement(ctx, actor, id, input.CommandID, hash, input.ExpectedRevision, "corrected", input.Reason, func(value *Engagement) {
		value.Provider, value.Scope, value.Currency, value.AmountMinor = provider, input.Scope, input.Currency, input.AmountMinor
		value.ContactName, value.ContactPhone = input.ContactName, input.ContactPhone
		value.Contract, value.Evidence, value.Note = contract, evidence, input.Note
	})
}

func (s *PostgresStore) WithdrawEngagement(ctx context.Context, actor Actor, id string, input WithdrawInput, hash string) (Engagement, error) {
	return s.mutateEngagement(ctx, actor, id, input.CommandID, hash, input.ExpectedRevision, "withdrawn", input.Reason, func(value *Engagement) { value.State = StateWithdrawn })
}

func (s *PostgresStore) mutateEngagement(ctx context.Context, actor Actor, id, commandID, hash string, expected int64, event, reason string, change func(*Engagement)) (Engagement, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Engagement{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = lockCommand(ctx, tx, actor, commandID); err != nil {
		return Engagement{}, err
	}
	var repeated Engagement
	if found, replayErr := replay(ctx, tx, actor, commandID, hash, "outsourcing_engagement", &repeated); replayErr != nil || found {
		return repeated, replayErr
	}
	var raw []byte
	if err = tx.QueryRow(ctx, `SELECT snapshot FROM outsourcing_engagements WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, actor.TenantID, id).Scan(&raw); err != nil {
		return Engagement{}, notFound(err)
	}
	var value Engagement
	if err = json.Unmarshal(raw, &value); err != nil {
		return Engagement{}, err
	}
	if value.State == StateWithdrawn {
		return Engagement{}, ErrWithdrawn
	}
	if value.Revision != expected {
		return Engagement{}, ErrRevisionConflict
	}
	change(&value)
	value.Revision++
	value.UpdatedBy = actor.ID
	value.UpdatedAt = time.Now().UTC()
	raw, _ = json.Marshal(value)
	if err = insertEngagementRevision(ctx, tx, actor, commandID, value, event, reason); err != nil {
		return Engagement{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE outsourcing_engagements SET provider_party_id=$3,state=$4,currency=$5,amount_minor=$6,revision=$7,snapshot=$8,updated_at=$9 WHERE tenant_id=$1 AND id=$2`, value.TenantID, value.ID, value.Provider.ID, value.State, value.Currency, value.AmountMinor, value.Revision, raw, value.UpdatedAt); err != nil {
		return Engagement{}, err
	}
	if err = finishCommand(ctx, tx, actor, commandID, hash, "outsourcing_engagement", value, value.ID, value.Revision, event); err != nil {
		return Engagement{}, err
	}
	return value, tx.Commit(ctx)
}

func (s *PostgresStore) Assess(ctx context.Context, actor Actor, engagementID string, input AssessInput, hash string) (Assessment, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Assessment{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = lockCommand(ctx, tx, actor, input.CommandID); err != nil {
		return Assessment{}, err
	}
	var repeated Assessment
	if found, replayErr := replay(ctx, tx, actor, input.CommandID, hash, "outsourcing_assessment", &repeated); replayErr != nil || found {
		return repeated, replayErr
	}
	var engagementRaw []byte
	if err = tx.QueryRow(ctx, `SELECT snapshot FROM outsourcing_engagements WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, actor.TenantID, engagementID).Scan(&engagementRaw); err != nil {
		return Assessment{}, notFound(err)
	}
	var engagement Engagement
	if err = json.Unmarshal(engagementRaw, &engagement); err != nil {
		return Assessment{}, err
	}
	if engagement.State == StateWithdrawn {
		return Assessment{}, ErrWithdrawn
	}
	if engagement.Revision != input.ExpectedEngagementRevision {
		return Assessment{}, ErrRevisionConflict
	}
	var current Assessment
	var assessmentRaw []byte
	rowErr := tx.QueryRow(ctx, `SELECT snapshot FROM outsourcing_assessment_revisions WHERE tenant_id=$1 AND engagement_id=$2 ORDER BY revision DESC LIMIT 1 FOR UPDATE`, actor.TenantID, engagementID).Scan(&assessmentRaw)
	if rowErr != nil && !errors.Is(rowErr, pgx.ErrNoRows) {
		return Assessment{}, rowErr
	}
	if rowErr == nil {
		if err = json.Unmarshal(assessmentRaw, &current); err != nil {
			return Assessment{}, err
		}
	}
	if current.Revision != input.ExpectedAssessmentRevision {
		return Assessment{}, ErrRevisionConflict
	}
	if current.ID == "" {
		current.ID, err = newUUID()
		if err != nil {
			return Assessment{}, err
		}
	}
	current.TenantID, current.EngagementID = actor.TenantID, engagementID
	current.EngagementRevision = engagement.Revision
	current.RuleKey, current.Score, current.Reason = AssessmentRuleReferenceOAFivePointV1, input.Score, input.Reason
	current.Revision++
	current.ActorRef, current.OccurredAt = actor.ID, time.Now().UTC()
	raw, _ := json.Marshal(current)
	if _, err = tx.Exec(ctx, `INSERT INTO outsourcing_assessment_revisions(tenant_id,engagement_id,assessment_id,revision,engagement_revision,command_id,rule_key,score,reason,actor_ref,occurred_at,snapshot) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, current.TenantID, current.EngagementID, current.ID, current.Revision, current.EngagementRevision, input.CommandID, current.RuleKey, current.Score, current.Reason, current.ActorRef, current.OccurredAt, raw); err != nil {
		return Assessment{}, err
	}
	if err = finishCommand(ctx, tx, actor, input.CommandID, hash, "outsourcing_assessment", current, engagementID, current.Revision, "assessed"); err != nil {
		return Assessment{}, err
	}
	return current, tx.Commit(ctx)
}

func (s *PostgresStore) AssessmentHistory(ctx context.Context, tenantID, engagementID string) ([]Assessment, error) {
	rows, err := s.pool.Query(ctx, `SELECT snapshot FROM outsourcing_assessment_revisions WHERE tenant_id=$1 AND engagement_id=$2 ORDER BY revision`, tenantID, engagementID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []Assessment
	for rows.Next() {
		var raw []byte
		var value Assessment
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &value); err != nil {
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

func (s *PostgresStore) ListLatestAssessments(ctx context.Context, tenantID, projectID string) ([]Assessment, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT latest.snapshot
		FROM outsourcing_engagements AS engagement
		JOIN LATERAL (
			SELECT assessment.snapshot
			FROM outsourcing_assessment_revisions AS assessment
			WHERE assessment.tenant_id=engagement.tenant_id AND assessment.engagement_id=engagement.id
			ORDER BY assessment.revision DESC
			LIMIT 1
		) AS latest ON true
		WHERE engagement.tenant_id=$1 AND engagement.project_id=$2
		ORDER BY engagement.created_at,engagement.id`, tenantID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]Assessment, 0)
	for rows.Next() {
		var raw []byte
		var value Assessment
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

type queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func replay(ctx context.Context, db queryer, actor Actor, commandID, hash, responseType string, destination any) (bool, error) {
	var storedActor, storedHash, storedType string
	var raw []byte
	err := db.QueryRow(ctx, `SELECT actor_ref,request_hash,response_type,response FROM cost_outsourcing_commands WHERE tenant_id=$1 AND command_id=$2`, actor.TenantID, commandID).Scan(&storedActor, &storedHash, &storedType, &raw)
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

func finishCommand(ctx context.Context, tx pgx.Tx, actor Actor, commandID, hash, responseType string, response any, aggregateID string, revision int64, eventKind string) error {
	raw, err := json.Marshal(response)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if _, err = tx.Exec(ctx, `INSERT INTO cost_outsourcing_commands(tenant_id,command_id,actor_ref,request_hash,response_type,response,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, actor.TenantID, commandID, actor.ID, hash, responseType, raw, now); err != nil {
		return err
	}
	eventID, err := newUUID()
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO cost_outsourcing_audit_events(tenant_id,event_id,aggregate_type,aggregate_id,aggregate_revision,command_id,actor_ref,event_kind,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, actor.TenantID, eventID, responseType, aggregateID, revision, commandID, actor.ID, eventKind, now)
	return err
}

func insertCostRevision(ctx context.Context, tx pgx.Tx, actor Actor, commandID string, value CostEntry, event, reason string) error {
	raw, _ := json.Marshal(value)
	_, err := tx.Exec(ctx, `INSERT INTO cost_entry_revisions(tenant_id,cost_entry_id,revision,event_kind,command_id,snapshot,reason,actor_ref,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, value.TenantID, value.ID, value.Revision, event, commandID, raw, reason, actor.ID, value.UpdatedAt)
	return err
}

func insertEngagementRevision(ctx context.Context, tx pgx.Tx, actor Actor, commandID string, value Engagement, event, reason string) error {
	raw, _ := json.Marshal(value)
	_, err := tx.Exec(ctx, `INSERT INTO outsourcing_engagement_revisions(tenant_id,engagement_id,revision,event_kind,command_id,snapshot,reason,actor_ref,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, value.TenantID, value.ID, value.Revision, event, commandID, raw, reason, actor.ID, value.UpdatedAt)
	return err
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
