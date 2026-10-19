package obligation

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

func (r *PostgresRepository) RecordRemission(
	ctx context.Context,
	value Remission,
	command RemissionCommand,
	entry audit.Entry,
) (Obligation, Remission, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Obligation{}, Remission{}, false, fmt.Errorf("begin obligation remission: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, command.CommandID); err != nil {
		return Obligation{}, Remission{}, false, fmt.Errorf("lock obligation remission command: %w", err)
	}
	var existingRemissionID string
	var existingPayloadHash []byte
	err = tx.QueryRow(ctx, `
		SELECT obligation_remission_id::text, payload_hash
		FROM obligation_remission_commands
		WHERE command_id = $1
	`, command.CommandID).Scan(&existingRemissionID, &existingPayloadHash)
	switch {
	case err == nil:
		if !bytes.Equal(existingPayloadHash, command.PayloadHash) {
			return Obligation{}, Remission{}, false, &RemissionIdempotencyConflictError{CommandID: command.CommandID}
		}
		existing, found, err := readRemission(ctx, tx, existingRemissionID)
		if err != nil {
			return Obligation{}, Remission{}, false, err
		}
		if !found {
			return Obligation{}, Remission{}, false, fmt.Errorf("obligation remission command references missing remission %s", existingRemissionID)
		}
		obligationValue, found, err := readObligation(ctx, tx, existing.ObligationID)
		if err != nil {
			return Obligation{}, Remission{}, false, err
		}
		if !found {
			return Obligation{}, Remission{}, false, fmt.Errorf("obligation remission references missing obligation %s", existing.ObligationID)
		}
		if err := tx.Commit(ctx); err != nil {
			return Obligation{}, Remission{}, false, fmt.Errorf("commit obligation remission replay: %w", err)
		}
		return obligationValue, existing, true, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return Obligation{}, Remission{}, false, fmt.Errorf("read obligation remission command: %w", err)
	}

	var status string
	var currentRevision int64
	err = tx.QueryRow(ctx, `
		SELECT status, COALESCE(current_revision, 0)
		FROM obligations
		WHERE id = $1
		FOR UPDATE
	`, value.ObligationID).Scan(&status, &currentRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return Obligation{}, Remission{}, false, &ObligationNotFoundError{ObligationID: value.ObligationID}
	}
	if err != nil {
		return Obligation{}, Remission{}, false, fmt.Errorf("lock obligation for remission: %w", err)
	}
	if status != StatusOpen {
		return Obligation{}, Remission{}, false, &RemissionStateConflictError{Status: status}
	}
	if currentRevision != value.ObligationRevision {
		return Obligation{}, Remission{}, false, &CurrentRevisionConflictError{
			Expected: value.ObligationRevision,
			Actual:   currentRevision,
		}
	}

	current, found, err := readObligation(ctx, tx, value.ObligationID)
	if err != nil {
		return Obligation{}, Remission{}, false, fmt.Errorf("read obligation balance for remission: %w", err)
	}
	if !found {
		return Obligation{}, Remission{}, false, fmt.Errorf("locked obligation disappeared: %s", value.ObligationID)
	}
	comparison, err := compareDecimalAmounts(value.Amount, current.OutstandingAmount)
	if err != nil {
		return Obligation{}, Remission{}, false, fmt.Errorf("compare remission and outstanding amount: %w", err)
	}
	if comparison > 0 {
		return Obligation{}, Remission{}, false, &RemissionBalanceConflictError{
			Requested: value.Amount, Outstanding: current.OutstandingAmount,
		}
	}

	value.RecordedEffectiveAmount = current.EffectiveAmount
	value.Currency = current.Currency
	if _, err := tx.Exec(ctx, `
		INSERT INTO obligation_remissions (
			id, obligation_id, obligation_revision,
			amount, recorded_effective_amount, currency,
			source_kind, remission_reason, manual_source_reference,
			recorded_by_actor_kind, recorded_by_actor_id,
			recorded_by_actor_display_name, recorded_at
		)
		VALUES (
			$1, $2, $3,
			$4::numeric, $5::numeric, $6,
			$7, $8, $9,
			$10, $11, $12, $13
		)
	`,
		value.ID,
		value.ObligationID,
		value.ObligationRevision,
		value.Amount,
		value.RecordedEffectiveAmount,
		value.Currency,
		value.SourceKind,
		value.Reason,
		value.ManualSourceReference,
		value.RecordedBy.Kind,
		value.RecordedBy.ID,
		value.RecordedBy.DisplayName,
		value.RecordedAt,
	); err != nil {
		return Obligation{}, Remission{}, false, fmt.Errorf("insert obligation remission: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO obligation_remission_commands (
			command_id, obligation_remission_id, obligation_id,
			expected_current_revision, payload_hash,
			actor_kind, actor_id, actor_display_name, recorded_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`,
		command.CommandID,
		value.ID,
		value.ObligationID,
		value.ObligationRevision,
		command.PayloadHash,
		command.Actor.Kind,
		command.Actor.ID,
		command.Actor.DisplayName,
		command.RecordedAt,
	); err != nil {
		return Obligation{}, Remission{}, false, fmt.Errorf("insert obligation remission command: %w", err)
	}

	if entry.Details == nil {
		entry.Details = make(map[string]any)
	}
	entry.Details["obligation_remission_id"] = value.ID
	entry.Details["obligation_revision"] = value.ObligationRevision
	entry.Details["recorded_effective_amount"] = value.RecordedEffectiveAmount
	entry.Details["currency"] = value.Currency
	if err := audit.Insert(ctx, tx, entry); err != nil {
		return Obligation{}, Remission{}, false, err
	}

	persisted, found, err := readRemission(ctx, tx, value.ID)
	if err != nil {
		return Obligation{}, Remission{}, false, err
	}
	if !found {
		return Obligation{}, Remission{}, false, fmt.Errorf("created obligation remission disappeared: %s", value.ID)
	}
	updated, found, err := readObligation(ctx, tx, value.ObligationID)
	if err != nil {
		return Obligation{}, Remission{}, false, err
	}
	if !found {
		return Obligation{}, Remission{}, false, fmt.Errorf("remitted obligation disappeared: %s", value.ObligationID)
	}
	if err := tx.Commit(ctx); err != nil {
		return Obligation{}, Remission{}, false, fmt.Errorf("commit obligation remission: %w", err)
	}
	return updated, persisted, false, nil
}

const remissionSelect = `
	SELECT
		id::text,
		obligation_id::text,
		obligation_revision,
		amount::text,
		recorded_effective_amount::text,
		currency,
		source_kind,
		remission_reason,
		manual_source_reference,
		recorded_by_actor_kind,
		recorded_by_actor_id,
		recorded_by_actor_display_name,
		recorded_at
	FROM obligation_remissions
`

func readRemission(ctx context.Context, queryer queryRower, remissionID string) (Remission, bool, error) {
	value, err := scanRemission(queryer.QueryRow(ctx, remissionSelect+` WHERE id = $1`, remissionID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Remission{}, false, nil
	}
	if err != nil {
		return Remission{}, false, err
	}
	return value, true, nil
}

func readRemissions(ctx context.Context, queryer queryer, obligationID string) ([]Remission, error) {
	rows, err := queryer.Query(ctx, remissionSelect+` WHERE obligation_id = $1 ORDER BY recorded_at, id`, obligationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]Remission, 0)
	for rows.Next() {
		value, err := scanRemission(rows)
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

func scanRemission(row rowScanner) (Remission, error) {
	var value Remission
	if err := row.Scan(
		&value.ID,
		&value.ObligationID,
		&value.ObligationRevision,
		&value.Amount,
		&value.RecordedEffectiveAmount,
		&value.Currency,
		&value.SourceKind,
		&value.Reason,
		&value.ManualSourceReference,
		&value.RecordedBy.Kind,
		&value.RecordedBy.ID,
		&value.RecordedBy.DisplayName,
		&value.RecordedAt,
	); err != nil {
		return Remission{}, err
	}
	var err error
	value.Amount, err = normalizeAmountForField("remission_amount", "减免金额", value.Amount, true)
	if err != nil {
		return Remission{}, fmt.Errorf("invalid stored obligation remission amount: %w", err)
	}
	value.RecordedEffectiveAmount, err = normalizeAmountForField("recorded_effective_amount", "记录时有效金额", value.RecordedEffectiveAmount, true)
	if err != nil {
		return Remission{}, fmt.Errorf("invalid stored obligation remission effective amount: %w", err)
	}
	return value, nil
}
