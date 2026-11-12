package recognition

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	contract "omsbizfoundry/invoice-recognition-contract"
)

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) (*PostgresRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("invoice recognition database is required")
	}
	return &PostgresRepository{pool: pool}, nil
}
func (r *PostgresRepository) Ping(ctx context.Context) error { return r.pool.Ping(ctx) }

func (r *PostgresRepository) ReplayProposal(ctx context.Context, tenantID, commandID, requestHash string) (contract.Proposal, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return contract.Proposal{}, false, err
	}
	defer tx.Rollback(context.Background())
	return replay[contract.Proposal](ctx, tx, tenantID, commandID, requestHash, "proposal")
}
func (r *PostgresRepository) ReplayConfirmation(ctx context.Context, tenantID, commandID, requestHash string) (contract.Confirmation, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return contract.Confirmation{}, false, err
	}
	defer tx.Rollback(context.Background())
	return replay[contract.Confirmation](ctx, tx, tenantID, commandID, requestHash, "confirmation")
}
func (r *PostgresRepository) ReplayDiscard(ctx context.Context, tenantID, commandID, requestHash string) (contract.Proposal, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return contract.Proposal{}, false, err
	}
	defer tx.Rollback(context.Background())
	return replay[contract.Proposal](ctx, tx, tenantID, commandID, requestHash, "discard")
}

func replay[T any](ctx context.Context, tx pgx.Tx, tenantID, commandID, requestHash, kind string) (T, bool, error) {
	var zero T
	var storedHash, storedKind string
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT request_hash,response_kind,response_json FROM invoice_recognition_commands WHERE tenant_id=$1 AND command_id=$2`, tenantID, commandID).Scan(&storedHash, &storedKind, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return zero, false, nil
	}
	if err != nil {
		return zero, false, err
	}
	if storedHash != requestHash || storedKind != kind {
		return zero, false, ErrIdempotencyConflict
	}
	if err = json.Unmarshal(raw, &zero); err != nil {
		return zero, false, err
	}
	return zero, true, nil
}
func insertCommand(ctx context.Context, tx pgx.Tx, tenantID, commandID, requestHash, kind string, value any, at time.Time) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO invoice_recognition_commands(tenant_id,command_id,request_hash,response_kind,response_json,created_at) VALUES($1,$2,$3,$4,$5,$6)`, tenantID, commandID, requestHash, kind, raw, at)
	return err
}
func insertAudit(ctx context.Context, tx pgx.Tx, actor Actor, proposalID, action string, sourceRevision, resultRevision int64, at time.Time) error {
	id, err := newUUID()
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO invoice_recognition_audit_events(id,tenant_id,proposal_id,action,source_revision,result_revision,actor_subject_id,credential_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, actor.TenantID, proposalID, action, sourceRevision, resultRevision, actor.SubjectID, actor.CredentialID, at)
	return err
}

func (r *PostgresRepository) Create(ctx context.Context, actor Actor, proposal contract.Proposal, requestHash string) (contract.Proposal, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return contract.Proposal{}, err
	}
	defer tx.Rollback(context.Background())
	if value, found, replayErr := replay[contract.Proposal](ctx, tx, proposal.TenantID, proposal.CommandID, requestHash, "proposal"); replayErr != nil || found {
		return value, replayErr
	}
	raw, err := json.Marshal(proposal)
	if err != nil {
		return contract.Proposal{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO invoice_recognition_proposals(id,tenant_id,evidence_file_id,evidence_version,proposal,status,revision,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$8)`, proposal.ID, proposal.TenantID, proposal.Evidence.FileID, proposal.Evidence.Version, raw, proposal.Status, proposal.Revision, proposal.CreatedAt); err != nil {
		return contract.Proposal{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO invoice_recognition_proposal_revisions(tenant_id,proposal_id,revision,command_id,event_kind,snapshot,actor_subject_id,credential_id,occurred_at) VALUES($1,$2,$3,$4,'recognized',$5,$6,$7,$8)`, proposal.TenantID, proposal.ID, proposal.Revision, proposal.CommandID, raw, actor.SubjectID, actor.CredentialID, proposal.CreatedAt); err != nil {
		return contract.Proposal{}, err
	}
	if err = insertCommand(ctx, tx, proposal.TenantID, proposal.CommandID, requestHash, "proposal", proposal, proposal.CreatedAt); err != nil {
		return contract.Proposal{}, err
	}
	if err = insertAudit(ctx, tx, actor, proposal.ID, "invoice_recognized", 0, proposal.Revision, proposal.CreatedAt); err != nil {
		return contract.Proposal{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return contract.Proposal{}, err
	}
	return proposal, nil
}

func scanProposal(row pgx.Row) (contract.Proposal, error) {
	var raw []byte
	err := row.Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return contract.Proposal{}, ErrNotFound
	}
	if err != nil {
		return contract.Proposal{}, err
	}
	var value contract.Proposal
	if err = json.Unmarshal(raw, &value); err != nil {
		return contract.Proposal{}, err
	}
	return value, nil
}
func (r *PostgresRepository) Get(ctx context.Context, tenantID, id string) (contract.Proposal, error) {
	return scanProposal(r.pool.QueryRow(ctx, `SELECT proposal FROM invoice_recognition_proposals WHERE tenant_id=$1 AND id=$2`, tenantID, id))
}
func (r *PostgresRepository) List(ctx context.Context, tenantID string) ([]contract.Proposal, error) {
	rows, err := r.pool.Query(ctx, `SELECT proposal FROM invoice_recognition_proposals WHERE tenant_id=$1 ORDER BY created_at DESC,id`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []contract.Proposal{}
	for rows.Next() {
		value, scanErr := scanProposal(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (r *PostgresRepository) Confirm(ctx context.Context, actor Actor, commandID string, proposal contract.Proposal, confirmation contract.Confirmation, requestHash string) (contract.Confirmation, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return contract.Confirmation{}, err
	}
	defer tx.Rollback(context.Background())
	if value, found, replayErr := replay[contract.Confirmation](ctx, tx, actor.TenantID, commandID, requestHash, "confirmation"); replayErr != nil || found {
		return value, replayErr
	}
	current, err := scanProposal(tx.QueryRow(ctx, `SELECT proposal FROM invoice_recognition_proposals WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, actor.TenantID, proposal.ID))
	if err != nil {
		return contract.Confirmation{}, err
	}
	if current.Status != "needs_review" || current.Revision != proposal.Revision {
		return contract.Confirmation{}, ErrConflict
	}
	confirmationRaw, _ := json.Marshal(confirmation)
	if _, err = tx.Exec(ctx, `INSERT INTO invoice_recognition_confirmations(tenant_id,proposal_id,proposal_revision,confirmation,confirmed_by_subject,confirmed_at) VALUES($1,$2,$3,$4,$5,$6)`, actor.TenantID, current.ID, current.Revision, confirmationRaw, actor.SubjectID, confirmation.ConfirmedAt); err != nil {
		return contract.Confirmation{}, err
	}
	current.Status = "confirmed"
	current.Candidate = confirmation.Candidate
	current.Revision++
	currentRaw, _ := json.Marshal(current)
	if _, err = tx.Exec(ctx, `UPDATE invoice_recognition_proposals SET proposal=$1,status='confirmed',revision=$2,updated_at=$3 WHERE tenant_id=$4 AND id=$5 AND revision=$6`, currentRaw, current.Revision, confirmation.ConfirmedAt, actor.TenantID, current.ID, proposal.Revision); err != nil {
		return contract.Confirmation{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO invoice_recognition_proposal_revisions(tenant_id,proposal_id,revision,command_id,event_kind,snapshot,actor_subject_id,credential_id,occurred_at) VALUES($1,$2,$3,$4,'confirmed',$5,$6,$7,$8)`, actor.TenantID, current.ID, current.Revision, commandID, currentRaw, actor.SubjectID, actor.CredentialID, confirmation.ConfirmedAt); err != nil {
		return contract.Confirmation{}, err
	}
	if err = insertCommand(ctx, tx, actor.TenantID, commandID, requestHash, "confirmation", confirmation, confirmation.ConfirmedAt); err != nil {
		return contract.Confirmation{}, err
	}
	if err = insertAudit(ctx, tx, actor, current.ID, "invoice_recognition_confirmed", proposal.Revision, current.Revision, confirmation.ConfirmedAt); err != nil {
		return contract.Confirmation{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return contract.Confirmation{}, err
	}
	return confirmation, nil
}

func (r *PostgresRepository) Discard(ctx context.Context, actor Actor, commandID string, proposal contract.Proposal, reason string, discardedAt time.Time, requestHash string) (contract.Proposal, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return contract.Proposal{}, err
	}
	defer tx.Rollback(context.Background())
	if value, found, replayErr := replay[contract.Proposal](ctx, tx, actor.TenantID, commandID, requestHash, "discard"); replayErr != nil || found {
		return value, replayErr
	}
	current, err := scanProposal(tx.QueryRow(ctx, `SELECT proposal FROM invoice_recognition_proposals WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, actor.TenantID, proposal.ID))
	if err != nil {
		return contract.Proposal{}, err
	}
	if current.Status != "needs_review" || current.Revision != proposal.Revision {
		return contract.Proposal{}, ErrConflict
	}
	current.Status = "discarded"
	current.DiscardReason = reason
	current.DiscardedBy = actor.SubjectID
	current.DiscardedAt = &discardedAt
	current.Revision++
	currentRaw, _ := json.Marshal(current)
	if _, err = tx.Exec(ctx, `UPDATE invoice_recognition_proposals SET proposal=$1,status='discarded',revision=$2,updated_at=$3 WHERE tenant_id=$4 AND id=$5 AND revision=$6`, currentRaw, current.Revision, discardedAt, actor.TenantID, current.ID, proposal.Revision); err != nil {
		return contract.Proposal{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO invoice_recognition_proposal_revisions(tenant_id,proposal_id,revision,command_id,event_kind,snapshot,actor_subject_id,credential_id,occurred_at) VALUES($1,$2,$3,$4,'discarded',$5,$6,$7,$8)`, actor.TenantID, current.ID, current.Revision, commandID, currentRaw, actor.SubjectID, actor.CredentialID, discardedAt); err != nil {
		return contract.Proposal{}, err
	}
	if err = insertCommand(ctx, tx, actor.TenantID, commandID, requestHash, "discard", current, discardedAt); err != nil {
		return contract.Proposal{}, err
	}
	if err = insertAudit(ctx, tx, actor, current.ID, "invoice_recognition_discarded", proposal.Revision, current.Revision, discardedAt); err != nil {
		return contract.Proposal{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return contract.Proposal{}, err
	}
	return current, nil
}

func newUUID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16]), nil
}

var _ Repository = (*PostgresRepository)(nil)
