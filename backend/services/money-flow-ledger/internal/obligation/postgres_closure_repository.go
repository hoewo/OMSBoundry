package obligation

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

func (r *PostgresRepository) Close(
	ctx context.Context,
	value Closure,
	command ClosureCommand,
	entry audit.Entry,
) (Obligation, Closure, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Obligation{}, Closure{}, false, fmt.Errorf("begin obligation closure: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, command.CommandID); err != nil {
		return Obligation{}, Closure{}, false, fmt.Errorf("lock obligation closure command: %w", err)
	}
	var existingClosureID string
	var existingPayloadHash []byte
	err = tx.QueryRow(ctx, `
		SELECT obligation_closure_id::text, payload_hash
		FROM obligation_closure_commands
		WHERE command_id = $1
	`, command.CommandID).Scan(&existingClosureID, &existingPayloadHash)
	switch {
	case err == nil:
		if !bytes.Equal(existingPayloadHash, command.PayloadHash) {
			return Obligation{}, Closure{}, false, &ClosureIdempotencyConflictError{CommandID: command.CommandID}
		}
		existing, found, err := readClosure(ctx, tx, existingClosureID)
		if err != nil {
			return Obligation{}, Closure{}, false, err
		}
		if !found {
			return Obligation{}, Closure{}, false, fmt.Errorf("obligation closure command references missing closure %s", existingClosureID)
		}
		obligationValue, found, err := readObligation(ctx, tx, existing.ObligationID)
		if err != nil {
			return Obligation{}, Closure{}, false, err
		}
		if !found {
			return Obligation{}, Closure{}, false, fmt.Errorf("obligation closure references missing obligation %s", existing.ObligationID)
		}
		if err := tx.Commit(ctx); err != nil {
			return Obligation{}, Closure{}, false, fmt.Errorf("commit obligation closure replay: %w", err)
		}
		return obligationValue, existing, true, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return Obligation{}, Closure{}, false, fmt.Errorf("read obligation closure command: %w", err)
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
		return Obligation{}, Closure{}, false, &ObligationNotFoundError{ObligationID: value.ObligationID}
	}
	if err != nil {
		return Obligation{}, Closure{}, false, fmt.Errorf("lock obligation for closure: %w", err)
	}
	if status != StatusOpen {
		return Obligation{}, Closure{}, false, &ClosureStateConflictError{Status: status}
	}
	if currentRevision != value.ObligationRevision {
		return Obligation{}, Closure{}, false, &CurrentRevisionConflictError{
			Expected: value.ObligationRevision,
			Actual:   currentRevision,
		}
	}

	current, found, err := readObligation(ctx, tx, value.ObligationID)
	if err != nil {
		return Obligation{}, Closure{}, false, fmt.Errorf("read obligation balance for closure: %w", err)
	}
	if !found {
		return Obligation{}, Closure{}, false, fmt.Errorf("locked obligation disappeared: %s", value.ObligationID)
	}
	if current.BalanceSequence != value.BalanceSequence {
		return Obligation{}, Closure{}, false, &BalanceSequenceConflictError{
			Expected: value.BalanceSequence,
			Actual:   current.BalanceSequence,
		}
	}
	if current.OutstandingAmount != "0" {
		return Obligation{}, Closure{}, false, &ClosureBalanceConflictError{Outstanding: current.OutstandingAmount}
	}

	value.RecordedEffectiveAmount = current.EffectiveAmount
	value.NetAllocatedAmount = current.AllocatedAmount
	value.RemittedAmount = current.RemittedAmount
	value.Currency = current.Currency
	switch {
	case current.AllocatedAmount != "0" && current.RemittedAmount == "0":
		value.SettlementBasis = ClosureBasisAllocated
	case current.AllocatedAmount == "0" && current.RemittedAmount != "0":
		value.SettlementBasis = ClosureBasisRemitted
	case current.AllocatedAmount != "0" && current.RemittedAmount != "0":
		value.SettlementBasis = ClosureBasisMixed
	default:
		return Obligation{}, Closure{}, false, fmt.Errorf("zero balance obligation has no settlement facts")
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO obligation_closures (
			id, obligation_id, obligation_revision, balance_sequence,
			recorded_effective_amount, net_allocated_amount, remitted_amount,
			currency, settlement_basis, source_kind, closure_reason,
			closed_by_actor_kind, closed_by_actor_id,
			closed_by_actor_display_name, closed_at
		)
		VALUES (
			$1, $2, $3, $4,
			$5::numeric, $6::numeric, $7::numeric,
			$8, $9, $10, $11,
			$12, $13, $14, $15
		)
	`,
		value.ID,
		value.ObligationID,
		value.ObligationRevision,
		value.BalanceSequence,
		value.RecordedEffectiveAmount,
		value.NetAllocatedAmount,
		value.RemittedAmount,
		value.Currency,
		value.SettlementBasis,
		value.SourceKind,
		value.Reason,
		value.ClosedBy.Kind,
		value.ClosedBy.ID,
		value.ClosedBy.DisplayName,
		value.ClosedAt,
	); err != nil {
		return Obligation{}, Closure{}, false, fmt.Errorf("insert obligation closure: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO obligation_closure_commands (
			command_id, obligation_closure_id, obligation_id,
			expected_current_revision, expected_balance_sequence,
			payload_hash, actor_kind, actor_id, actor_display_name, closed_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`,
		command.CommandID,
		value.ID,
		value.ObligationID,
		value.ObligationRevision,
		value.BalanceSequence,
		command.PayloadHash,
		command.Actor.Kind,
		command.Actor.ID,
		command.Actor.DisplayName,
		command.ClosedAt,
	); err != nil {
		return Obligation{}, Closure{}, false, fmt.Errorf("insert obligation closure command: %w", err)
	}

	result, err := tx.Exec(ctx, `
		UPDATE obligations
		SET status = $2, updated_at = $3
		WHERE id = $1 AND status = 'open' AND current_revision = $4
	`, value.ObligationID, StatusClosed, value.ClosedAt, value.ObligationRevision)
	if err != nil {
		return Obligation{}, Closure{}, false, fmt.Errorf("advance closed obligation: %w", err)
	}
	if result.RowsAffected() != 1 {
		return Obligation{}, Closure{}, false, fmt.Errorf("obligation changed after closure row lock")
	}

	if entry.Details == nil {
		entry.Details = make(map[string]any)
	}
	entry.Details["obligation_closure_id"] = value.ID
	entry.Details["obligation_revision"] = value.ObligationRevision
	entry.Details["balance_sequence"] = value.BalanceSequence
	entry.Details["recorded_effective_amount"] = value.RecordedEffectiveAmount
	entry.Details["net_allocated_amount"] = value.NetAllocatedAmount
	entry.Details["remitted_amount"] = value.RemittedAmount
	entry.Details["currency"] = value.Currency
	entry.Details["settlement_basis"] = value.SettlementBasis
	if err := audit.Insert(ctx, tx, entry); err != nil {
		return Obligation{}, Closure{}, false, err
	}

	persisted, found, err := readClosure(ctx, tx, value.ID)
	if err != nil {
		return Obligation{}, Closure{}, false, err
	}
	if !found {
		return Obligation{}, Closure{}, false, fmt.Errorf("created obligation closure disappeared: %s", value.ID)
	}
	updated, found, err := readObligation(ctx, tx, value.ObligationID)
	if err != nil {
		return Obligation{}, Closure{}, false, err
	}
	if !found {
		return Obligation{}, Closure{}, false, fmt.Errorf("closed obligation disappeared: %s", value.ObligationID)
	}
	if err := tx.Commit(ctx); err != nil {
		return Obligation{}, Closure{}, false, fmt.Errorf("commit obligation closure: %w", err)
	}
	return updated, persisted, false, nil
}

const closureSelect = `
	SELECT
		id::text,
		obligation_id::text,
		obligation_revision,
		balance_sequence,
		recorded_effective_amount::text,
		net_allocated_amount::text,
		remitted_amount::text,
		currency,
		settlement_basis,
		source_kind,
		closure_reason,
		closed_by_actor_kind,
		closed_by_actor_id,
		closed_by_actor_display_name,
		closed_at
	FROM obligation_closures
`

func readClosure(ctx context.Context, queryer queryRower, closureID string) (Closure, bool, error) {
	value, err := scanClosure(queryer.QueryRow(ctx, closureSelect+` WHERE id = $1`, closureID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Closure{}, false, nil
	}
	if err != nil {
		return Closure{}, false, err
	}
	return value, true, nil
}

func readClosureForObligation(ctx context.Context, queryer queryRower, obligationID string) (Closure, bool, error) {
	value, err := scanClosure(queryer.QueryRow(ctx, closureSelect+` WHERE obligation_id = $1`, obligationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Closure{}, false, nil
	}
	if err != nil {
		return Closure{}, false, err
	}
	return value, true, nil
}

func scanClosure(row rowScanner) (Closure, error) {
	var value Closure
	if err := row.Scan(
		&value.ID,
		&value.ObligationID,
		&value.ObligationRevision,
		&value.BalanceSequence,
		&value.RecordedEffectiveAmount,
		&value.NetAllocatedAmount,
		&value.RemittedAmount,
		&value.Currency,
		&value.SettlementBasis,
		&value.SourceKind,
		&value.Reason,
		&value.ClosedBy.Kind,
		&value.ClosedBy.ID,
		&value.ClosedBy.DisplayName,
		&value.ClosedAt,
	); err != nil {
		return Closure{}, err
	}
	var err error
	value.RecordedEffectiveAmount, err = normalizeAmount(value.RecordedEffectiveAmount)
	if err != nil {
		return Closure{}, fmt.Errorf("invalid stored closure effective amount: %w", err)
	}
	value.NetAllocatedAmount, err = normalizeAmountAllowZero(value.NetAllocatedAmount)
	if err != nil {
		return Closure{}, fmt.Errorf("invalid stored closure allocated amount: %w", err)
	}
	value.RemittedAmount, err = normalizeAmountAllowZero(value.RemittedAmount)
	if err != nil {
		return Closure{}, fmt.Errorf("invalid stored closure remitted amount: %w", err)
	}
	return value, nil
}
