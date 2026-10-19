package obligation

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

func (r *PostgresRepository) Confirm(
	ctx context.Context,
	obligationID string,
	expectedDraftVersion int64,
	command ConfirmationCommand,
	entry audit.Entry,
) (Obligation, Revision, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Obligation{}, Revision{}, false, fmt.Errorf("begin obligation confirmation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, command.CommandID); err != nil {
		return Obligation{}, Revision{}, false, fmt.Errorf("lock obligation confirmation command: %w", err)
	}
	var existingObligationID string
	var existingRevision int64
	var existingPayloadHash []byte
	err = tx.QueryRow(ctx, `
		SELECT obligation_id::text, resulting_revision, payload_hash
		FROM obligation_confirmation_commands
		WHERE command_id = $1
	`, command.CommandID).Scan(&existingObligationID, &existingRevision, &existingPayloadHash)
	switch {
	case err == nil:
		if !bytes.Equal(existingPayloadHash, command.PayloadHash) {
			return Obligation{}, Revision{}, false, &ConfirmationIdempotencyConflictError{CommandID: command.CommandID}
		}
		existing, found, err := readObligation(ctx, tx, existingObligationID)
		if err != nil {
			return Obligation{}, Revision{}, false, err
		}
		if !found {
			return Obligation{}, Revision{}, false, fmt.Errorf("obligation confirmation command references missing obligation %s", existingObligationID)
		}
		revision, found, err := readRevision(ctx, tx, existingObligationID, existingRevision)
		if err != nil {
			return Obligation{}, Revision{}, false, err
		}
		if !found {
			return Obligation{}, Revision{}, false, fmt.Errorf("obligation confirmation command references missing revision %s/%d", existingObligationID, existingRevision)
		}
		if err := tx.Commit(ctx); err != nil {
			return Obligation{}, Revision{}, false, fmt.Errorf("commit obligation confirmation replay: %w", err)
		}
		return existing, revision, true, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return Obligation{}, Revision{}, false, fmt.Errorf("read obligation confirmation command: %w", err)
	}

	var status string
	var actualDraftVersion int64
	err = tx.QueryRow(ctx, `
		SELECT status, draft_version
		FROM obligations
		WHERE id = $1
		FOR UPDATE
	`, obligationID).Scan(&status, &actualDraftVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return Obligation{}, Revision{}, false, &ObligationNotFoundError{ObligationID: obligationID}
	}
	if err != nil {
		return Obligation{}, Revision{}, false, fmt.Errorf("lock obligation for confirmation: %w", err)
	}
	if status != StatusDraft {
		return Obligation{}, Revision{}, false, &ObligationStateConflictError{Status: status}
	}
	if actualDraftVersion != expectedDraftVersion {
		return Obligation{}, Revision{}, false, &DraftVersionConflictError{Expected: expectedDraftVersion, Actual: actualDraftVersion}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO obligation_revisions (
			obligation_id, revision, direction, title, ledger_owner_party_id,
			creditor_party_id, creditor_party_revision, creditor_kind,
			creditor_legal_name, creditor_registration_identifier,
			debtor_party_id, debtor_party_revision, debtor_kind,
			debtor_legal_name, debtor_registration_identifier,
			effective_amount, currency, due_on,
			source_kind, manual_source_reference,
			source_application, source_resource_type, source_resource_id, source_fact_revision, source_digest,
			source_draft_version,
			recorded_by_actor_kind, recorded_by_actor_id,
			recorded_by_actor_display_name, recorded_at,
			revision_kind, source_revision
		)
		SELECT
			id, $2, direction, title, ledger_owner_party_id,
			creditor_party_id, creditor_party_revision, creditor_kind,
			creditor_legal_name, creditor_registration_identifier,
			debtor_party_id, debtor_party_revision, debtor_kind,
			debtor_legal_name, debtor_registration_identifier,
			original_amount, currency, due_on,
			source_kind, manual_source_reference,
			source_application, source_resource_type, source_resource_id, source_revision, source_digest,
			draft_version,
			$3, $4, $5, $6,
			$7, NULL
		FROM obligations
		WHERE id = $1
	`,
		obligationID,
		firstRevision,
		command.Actor.Kind,
		command.Actor.ID,
		command.Actor.DisplayName,
		command.ConfirmedAt,
		RevisionKindConfirmation,
	); err != nil {
		return Obligation{}, Revision{}, false, fmt.Errorf("insert obligation revision 1: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO obligation_confirmation_commands (
			command_id, obligation_id, expected_draft_version,
			resulting_revision, payload_hash,
			actor_kind, actor_id, actor_display_name, confirmed_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`,
		command.CommandID,
		obligationID,
		expectedDraftVersion,
		firstRevision,
		command.PayloadHash,
		command.Actor.Kind,
		command.Actor.ID,
		command.Actor.DisplayName,
		command.ConfirmedAt,
	); err != nil {
		return Obligation{}, Revision{}, false, fmt.Errorf("insert obligation confirmation command: %w", err)
	}

	result, err := tx.Exec(ctx, `
		UPDATE obligations
		SET status = $2, current_revision = $3, updated_at = $4
		WHERE id = $1 AND status = 'draft' AND draft_version = $5
	`, obligationID, StatusOpen, firstRevision, command.ConfirmedAt, expectedDraftVersion)
	if err != nil {
		return Obligation{}, Revision{}, false, fmt.Errorf("advance confirmed obligation: %w", err)
	}
	if result.RowsAffected() != 1 {
		return Obligation{}, Revision{}, false, fmt.Errorf("confirmed obligation changed after row lock")
	}
	if err := audit.Insert(ctx, tx, entry); err != nil {
		return Obligation{}, Revision{}, false, err
	}

	confirmed, found, err := readObligation(ctx, tx, obligationID)
	if err != nil {
		return Obligation{}, Revision{}, false, err
	}
	if !found {
		return Obligation{}, Revision{}, false, fmt.Errorf("confirmed obligation disappeared: %s", obligationID)
	}
	revision, found, err := readRevision(ctx, tx, obligationID, firstRevision)
	if err != nil {
		return Obligation{}, Revision{}, false, err
	}
	if !found {
		return Obligation{}, Revision{}, false, fmt.Errorf("confirmed obligation revision disappeared: %s/%d", obligationID, firstRevision)
	}
	if err := tx.Commit(ctx); err != nil {
		return Obligation{}, Revision{}, false, fmt.Errorf("commit obligation confirmation: %w", err)
	}
	return confirmed, revision, false, nil
}

const revisionSelect = `
	SELECT
		obligation_id::text,
		revision,
		direction,
		title,
		ledger_owner_party_id::text,
		creditor_party_id::text,
		creditor_party_revision,
		creditor_kind,
		creditor_legal_name,
		COALESCE(creditor_registration_identifier, ''),
		debtor_party_id::text,
		debtor_party_revision,
		debtor_kind,
		debtor_legal_name,
		COALESCE(debtor_registration_identifier, ''),
		effective_amount::text,
		currency,
		due_on::text,
		source_kind,
		COALESCE(manual_source_reference, ''),
		COALESCE(source_application, ''),
		COALESCE(source_resource_type, ''),
		COALESCE(source_resource_id::text, ''),
		COALESCE(source_fact_revision, 0),
		COALESCE(source_digest, ''),
		COALESCE(source_draft_version, 0),
		revision_kind,
		COALESCE(source_revision, 0),
		recorded_by_actor_kind,
		recorded_by_actor_id,
		recorded_by_actor_display_name,
		recorded_at
	FROM obligation_revisions
`

func readRevision(ctx context.Context, queryer queryRower, obligationID string, revisionNumber int64) (Revision, bool, error) {
	value, err := scanRevision(queryer.QueryRow(
		ctx,
		revisionSelect+` WHERE obligation_id = $1 AND revision = $2`,
		obligationID,
		revisionNumber,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return Revision{}, false, nil
	}
	if err != nil {
		return Revision{}, false, err
	}
	return value, true, nil
}

type queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func readRevisions(ctx context.Context, queryer queryer, obligationID string) ([]Revision, error) {
	rows, err := queryer.Query(ctx, revisionSelect+` WHERE obligation_id = $1 ORDER BY revision`, obligationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]Revision, 0)
	for rows.Next() {
		value, err := scanRevision(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func scanRevision(row rowScanner) (Revision, error) {
	var value Revision
	err := row.Scan(
		&value.ObligationID,
		&value.Revision,
		&value.Direction,
		&value.Title,
		&value.LedgerOwnerPartyID,
		&value.Creditor.PartyID,
		&value.Creditor.PartyRevision,
		&value.Creditor.Kind,
		&value.Creditor.LegalName,
		&value.Creditor.RegistrationIdentifier,
		&value.Debtor.PartyID,
		&value.Debtor.PartyRevision,
		&value.Debtor.Kind,
		&value.Debtor.LegalName,
		&value.Debtor.RegistrationIdentifier,
		&value.EffectiveAmount,
		&value.Currency,
		&value.DueOn,
		&value.SourceKind,
		&value.ManualSourceReference,
		&value.SourceApplication,
		&value.SourceResourceType,
		&value.SourceResourceID,
		&value.SourceFactRevision,
		&value.SourceDigest,
		&value.SourceDraftVersion,
		&value.RevisionKind,
		&value.SourceRevision,
		&value.RecordedBy.Kind,
		&value.RecordedBy.ID,
		&value.RecordedBy.DisplayName,
		&value.RecordedAt,
	)
	if err != nil {
		return Revision{}, err
	}
	normalizedAmount, err := normalizeAmountForField("effective_amount", "义务有效金额", value.EffectiveAmount, true)
	if err != nil {
		return Revision{}, fmt.Errorf("invalid stored obligation revision amount: %w", err)
	}
	value.EffectiveAmount = normalizedAmount
	return value, nil
}
