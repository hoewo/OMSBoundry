package payablesuggestion

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/money-flow-ledger/internal/audit"
	contract "omsbizfoundry/payable-suggestion-contract"
)

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Submit(ctx context.Context, actor audit.ActorRef, value contract.Submission, commandDigest string) (Intake, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Intake{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "payable-suggestion:"+value.TenantID+":"+value.SuggestionID); err != nil {
		return Intake{}, false, err
	}
	if existing, found, replayErr := replayCommand(ctx, tx, value.TenantID, value.SuggestionID, commandDigest); replayErr != nil || found {
		if replayErr == nil {
			replayErr = tx.Commit(ctx)
		}
		return existing, found, replayErr
	}
	ownerLocal, err := resolveLocalParty(ctx, tx, value.TenantID, value.LedgerOwner)
	if err != nil {
		return Intake{}, false, err
	}
	counterpartyLocal, err := resolveLocalParty(ctx, tx, value.TenantID, value.Counterparty)
	if err != nil {
		return Intake{}, false, err
	}
	now := time.Now().UTC()
	intake := Intake{
		SchemaVersion: contract.SchemaVersion, SuggestionID: value.SuggestionID, TenantID: value.TenantID,
		RequestDigest: value.RequestDigest, Submission: value, Status: contract.StatusPending, Revision: 1,
		SubmittedBy: actor, CreatedAt: now, UpdatedAt: now,
		LedgerOwnerLocalPartyID: ownerLocal, CounterpartyLocalPartyID: counterpartyLocal,
	}
	raw, err := json.Marshal(intake)
	if err != nil {
		return Intake{}, false, err
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO payable_suggestion_intakes(
			tenant_id,suggestion_id,request_digest,source_application,source_resource_type,source_resource_id,source_revision,
			project_id,ledger_owner_party_id,counterparty_party_id,
			ledger_owner_directory_party_id,ledger_owner_directory_party_revision,
			counterparty_directory_party_id,counterparty_directory_party_revision,
			ledger_owner_source_application,ledger_owner_source_resource_type,
			counterparty_source_application,counterparty_source_resource_type,
			amount_minor,currency,due_on,purpose,status,revision,
			submitted_by_actor_kind,submitted_by_actor_id,created_at,updated_at,snapshot
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29)
	`, value.TenantID, value.SuggestionID, value.RequestDigest, value.Source.Application, value.Source.ResourceType, value.Source.ResourceID, value.Source.Revision,
		value.ProjectID, ownerLocal, counterpartyLocal, value.LedgerOwner.ID, value.LedgerOwner.Revision, value.Counterparty.ID, value.Counterparty.Revision,
		partySourceApplication(value.LedgerOwner), partyResourceType(value.LedgerOwner), partySourceApplication(value.Counterparty), partyResourceType(value.Counterparty),
		value.AmountMinor, value.Currency, value.DueOn, value.Purpose, intake.Status, intake.Revision, actor.Kind, actor.ID, now, now, raw); err != nil {
		return Intake{}, false, err
	}
	if err = insertRevision(ctx, tx, intake, value.SuggestionID, "submitted", value.RequestDigest, commandDigest, actor); err != nil {
		return Intake{}, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Intake{}, false, err
	}
	return intake, false, nil
}

func (r *PostgresRepository) Get(ctx context.Context, tenantID, suggestionID string) (Intake, error) {
	return readIntake(ctx, r.pool, tenantID, suggestionID)
}

func (r *PostgresRepository) History(ctx context.Context, tenantID, suggestionID string) ([]Revision, error) {
	rows, err := r.pool.Query(ctx, `SELECT revision,command_id::text,event_kind,snapshot,actor_kind,actor_id,occurred_at FROM payable_suggestion_intake_revisions WHERE tenant_id=$1 AND suggestion_id=$2 ORDER BY revision`, tenantID, suggestionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []Revision
	for rows.Next() {
		var value Revision
		var raw []byte
		value.SuggestionID = suggestionID
		if err = rows.Scan(&value.Revision, &value.CommandID, &value.EventKind, &raw, &value.Actor.Kind, &value.Actor.ID, &value.OccurredAt); err != nil {
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

func (r *PostgresRepository) ReplayCommand(ctx context.Context, tenantID, commandID, commandDigest string) (Intake, bool, error) {
	return replayCommand(ctx, r.pool, tenantID, commandID, commandDigest)
}

func (r *PostgresRepository) RecordDecision(ctx context.Context, actor audit.ActorRef, tenantID, suggestionID string, expected int64, commandID, commandDigest, status, reason string, obligationRef *contract.ObligationReference) (Intake, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Intake{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "payable-suggestion:"+tenantID+":"+suggestionID); err != nil {
		return Intake{}, err
	}
	if replay, found, replayErr := replayCommand(ctx, tx, tenantID, commandID, commandDigest); replayErr != nil || found {
		if replayErr == nil {
			replayErr = tx.Commit(ctx)
		}
		return replay, replayErr
	}
	current, err := readIntake(ctx, tx, tenantID, suggestionID)
	if err != nil {
		return Intake{}, err
	}
	if current.Revision != expected {
		return Intake{}, ErrRevisionConflict
	}
	if current.Status != contract.StatusPending {
		return Intake{}, ErrStateConflict
	}
	current.Status, current.DecisionReason, current.Revision, current.UpdatedAt = status, strings.TrimSpace(reason), current.Revision+1, time.Now().UTC()
	current.DecidedBy = &actor
	if obligationRef != nil {
		copy := *obligationRef
		current.Obligation = &copy
	}
	raw, err := json.Marshal(current)
	if err != nil {
		return Intake{}, err
	}
	if err = insertRevision(ctx, tx, current, commandID, map[string]string{contract.StatusAccepted: "accepted", contract.StatusRejected: "rejected", contract.StatusWithdrawn: "withdrawn"}[status], current.RequestDigest, commandDigest, actor); err != nil {
		return Intake{}, err
	}
	var obligationID any
	var obligationRevision any
	if current.Obligation != nil {
		obligationID, obligationRevision = current.Obligation.ID, current.Obligation.Revision
	}
	result, err := tx.Exec(ctx, `UPDATE payable_suggestion_intakes SET status=$4,revision=$5,obligation_id=$6,obligation_revision=$7,decision_reason=$8,decided_by_actor_kind=$9,decided_by_actor_id=$10,updated_at=$11,snapshot=$12 WHERE tenant_id=$1 AND suggestion_id=$2 AND revision=$3 AND status='pending'`, tenantID, suggestionID, expected, current.Status, current.Revision, obligationID, obligationRevision, current.DecisionReason, actor.Kind, actor.ID, current.UpdatedAt, raw)
	if err != nil {
		return Intake{}, err
	}
	if result.RowsAffected() != 1 {
		return Intake{}, ErrRevisionConflict
	}
	if err = tx.Commit(ctx); err != nil {
		return Intake{}, err
	}
	return current, nil
}

type queryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func replayCommand(ctx context.Context, db queryRower, tenantID, commandID, commandDigest string) (Intake, bool, error) {
	var storedDigest string
	var raw []byte
	err := db.QueryRow(ctx, `SELECT command_digest,snapshot FROM payable_suggestion_intake_revisions WHERE tenant_id=$1 AND command_id=$2`, tenantID, commandID).Scan(&storedDigest, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Intake{}, false, nil
	}
	if err != nil {
		return Intake{}, false, err
	}
	if storedDigest != commandDigest {
		return Intake{}, false, ErrCommandConflict
	}
	var value Intake
	if err = json.Unmarshal(raw, &value); err != nil {
		return Intake{}, false, err
	}
	return value, true, nil
}

func resolveLocalParty(ctx context.Context, db queryRower, tenantID string, source contract.PartySnapshot) (string, error) {
	application, resourceType := partySourceApplication(source), partyResourceType(source)
	if application == contract.PartySourceApplicationMoneyFlow && resourceType == contract.PartyResourceParty {
		var localID string
		var revision int64
		var kind, legalName string
		err := db.QueryRow(ctx, `SELECT id::text,revision,kind,legal_name FROM parties WHERE id=$1`, source.ID).Scan(&localID, &revision, &kind, &legalName)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && (revision != source.Revision || kind != source.Kind || legalName != source.LegalName) {
			return "", ErrPartyMapping
		}
		return localID, err
	}
	if application == contract.PartySourceApplicationPeopleGroups && resourceType == contract.PartyResourcePerson {
		var localID string
		err := db.QueryRow(ctx, `SELECT m.local_party_id::text
			FROM payable_party_source_mappings m
			JOIN parties p ON p.id=m.local_party_id
			WHERE m.tenant_id=$1 AND m.source_application=$2 AND m.source_resource_type=$3
			  AND m.source_resource_id=$4 AND m.source_revision=$5 AND m.source_digest=$6
			  AND p.revision=m.local_party_revision AND p.kind=m.local_party_kind AND p.legal_name=m.local_party_legal_name`, tenantID, application, resourceType, source.ID, source.Revision, source.Digest).Scan(&localID)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrPartyMapping
		}
		return localID, err
	}
	var action, batchID string
	err := db.QueryRow(ctx, `SELECT action,migration_batch_id::text FROM party_runtime_transitions WHERE tenant_id=$1 ORDER BY sequence DESC LIMIT 1`, tenantID).Scan(&action, &batchID)
	if err != nil || action != "cutover" {
		return "", ErrPartyMapping
	}
	var localID string
	err = db.QueryRow(ctx, `
		SELECT local_party_id FROM (
			SELECT local_party_id::text,1 AS priority
			FROM party_migration_mappings
			WHERE tenant_id=$1 AND migration_batch_id=$2 AND target_party_id=$3 AND target_party_revision=$4
			UNION ALL
			SELECT local_party_id::text,2 AS priority
			FROM party_directory_runtime_projections
			WHERE tenant_id=$1 AND target_party_id=$3 AND target_party_revision=$4
		) resolved
		ORDER BY priority
		LIMIT 1
	`, tenantID, batchID, source.ID, source.Revision).Scan(&localID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrPartyMapping
	}
	return localID, err
}

func partySourceApplication(value contract.PartySnapshot) string {
	if value.SourceApplication == "" {
		return contract.PartySourceApplicationPartyDirectory
	}
	return value.SourceApplication
}

func partyResourceType(value contract.PartySnapshot) string {
	if value.ResourceType == "" {
		return contract.PartyResourceParty
	}
	return value.ResourceType
}

func readIntake(ctx context.Context, db queryRower, tenantID, suggestionID string) (Intake, error) {
	var value Intake
	var raw []byte
	err := db.QueryRow(ctx, `SELECT snapshot,ledger_owner_party_id::text,counterparty_party_id::text FROM payable_suggestion_intakes WHERE tenant_id=$1 AND suggestion_id=$2`, tenantID, suggestionID).Scan(&raw, &value.LedgerOwnerLocalPartyID, &value.CounterpartyLocalPartyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Intake{}, ErrNotFound
	}
	if err != nil {
		return Intake{}, err
	}
	ownerLocal, counterpartyLocal := value.LedgerOwnerLocalPartyID, value.CounterpartyLocalPartyID
	if err = json.Unmarshal(raw, &value); err != nil {
		return Intake{}, err
	}
	value.LedgerOwnerLocalPartyID, value.CounterpartyLocalPartyID = ownerLocal, counterpartyLocal
	return value, nil
}

func insertRevision(ctx context.Context, tx pgx.Tx, value Intake, commandID, event, requestDigest, commandDigest string, actor audit.ActorRef) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO payable_suggestion_intake_revisions(tenant_id,suggestion_id,revision,command_id,event_kind,request_digest,command_digest,snapshot,actor_kind,actor_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, value.TenantID, value.SuggestionID, value.Revision, commandID, event, requestDigest, commandDigest, raw, actor.Kind, actor.ID, value.UpdatedAt)
	return err
}

var _ Repository = (*PostgresRepository)(nil)
