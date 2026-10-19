package cashtransaction

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

type cashTransactionFacts struct {
	TransactionType Type
	Amount          string
	Currency        string
}

func (r *PostgresRepository) Reverse(
	ctx context.Context,
	value Reversal,
	command ReverseCommand,
	entry audit.Entry,
) (Reversal, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Reversal{}, false, fmt.Errorf("begin reverse cash transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, command.CommandID); err != nil {
		return Reversal{}, false, fmt.Errorf("lock cash transaction reversal command: %w", err)
	}
	var existingReversalID string
	var existingPayloadHash []byte
	err = tx.QueryRow(ctx, `
		SELECT cash_transaction_reversal_id::text, payload_hash
		FROM cash_transaction_reversal_create_commands
		WHERE command_id = $1
	`, command.CommandID).Scan(&existingReversalID, &existingPayloadHash)
	switch {
	case err == nil:
		if !bytes.Equal(existingPayloadHash, command.PayloadHash) {
			return Reversal{}, false, &ReversalIdempotencyConflictError{CommandID: command.CommandID}
		}
		existing, found, err := readCashReversal(ctx, tx, existingReversalID)
		if err != nil {
			return Reversal{}, false, err
		}
		if !found {
			return Reversal{}, false, fmt.Errorf("cash transaction reversal command references missing reversal %s", existingReversalID)
		}
		if err := tx.Commit(ctx); err != nil {
			return Reversal{}, false, fmt.Errorf("commit cash transaction reversal replay: %w", err)
		}
		return existing, true, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return Reversal{}, false, fmt.Errorf("read cash transaction reversal command: %w", err)
	}

	var facts cashTransactionFacts
	err = tx.QueryRow(ctx, `
		SELECT type, amount::text, currency
		FROM cash_transactions
		WHERE id = $1
		FOR UPDATE
	`, value.CashTransactionID).Scan(&facts.TransactionType, &facts.Amount, &facts.Currency)
	if errors.Is(err, pgx.ErrNoRows) {
		return Reversal{}, false, &TransactionNotFoundError{CashTransactionID: value.CashTransactionID}
	}
	if err != nil {
		return Reversal{}, false, fmt.Errorf("lock cash transaction for reversal: %w", err)
	}
	facts.Amount, err = normalizeAmount(facts.Amount)
	if err != nil {
		return Reversal{}, false, fmt.Errorf("invalid stored cash transaction amount for reversal: %w", err)
	}

	var previousReversalID string
	err = tx.QueryRow(ctx, `
		SELECT id::text
		FROM cash_transaction_reversals
		WHERE cash_transaction_id = $1
	`, value.CashTransactionID).Scan(&previousReversalID)
	switch {
	case err == nil:
		return Reversal{}, false, &TransactionAlreadyReversedError{
			CashTransactionID: value.CashTransactionID,
			ReversalID:        previousReversalID,
		}
	case !errors.Is(err, pgx.ErrNoRows):
		return Reversal{}, false, fmt.Errorf("check existing cash transaction reversal: %w", err)
	}

	var allocatedAmount string
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0)::text
		FROM allocations a
		LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
		WHERE a.cash_transaction_id = $1
	`, value.CashTransactionID).Scan(&allocatedAmount); err != nil {
		return Reversal{}, false, fmt.Errorf("read cash transaction net allocated amount for reversal: %w", err)
	}
	allocatedAmount, err = normalizeAmountAllowZero(allocatedAmount)
	if err != nil {
		return Reversal{}, false, fmt.Errorf("invalid stored cash transaction allocated amount for reversal: %w", err)
	}
	if allocatedAmount != "0" {
		return Reversal{}, false, &TransactionAllocatedConflictError{
			CashTransactionID: value.CashTransactionID,
			AllocatedAmount:   allocatedAmount,
		}
	}

	value.TransactionType = facts.TransactionType
	value.Amount = facts.Amount
	value.Currency = facts.Currency
	if _, err := tx.Exec(ctx, `
		INSERT INTO cash_transaction_reversals (
			id, cash_transaction_id, transaction_type,
			amount, currency, source_kind, reversal_reason,
			reversed_by_actor_kind, reversed_by_actor_id,
			reversed_by_actor_display_name, reversed_at
		)
		VALUES ($1, $2, $3, $4::numeric, $5, $6, $7, $8, $9, $10, $11)
	`,
		value.ID,
		value.CashTransactionID,
		value.TransactionType,
		value.Amount,
		value.Currency,
		value.SourceKind,
		value.Reason,
		value.ReversedBy.Kind,
		value.ReversedBy.ID,
		value.ReversedBy.DisplayName,
		value.ReversedAt,
	); err != nil {
		return Reversal{}, false, fmt.Errorf("insert cash transaction reversal: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO cash_transaction_reversal_create_commands (
			command_id, cash_transaction_reversal_id, cash_transaction_id,
			payload_hash, actor_kind, actor_id, actor_display_name, reversed_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`,
		command.CommandID,
		value.ID,
		value.CashTransactionID,
		command.PayloadHash,
		command.Actor.Kind,
		command.Actor.ID,
		command.Actor.DisplayName,
		command.ReversedAt,
	); err != nil {
		return Reversal{}, false, fmt.Errorf("insert cash transaction reversal command: %w", err)
	}
	if entry.Details == nil {
		entry.Details = make(map[string]any)
	}
	entry.Details["cash_transaction_reversal_id"] = value.ID
	entry.Details["transaction_type"] = value.TransactionType
	entry.Details["amount"] = value.Amount
	entry.Details["currency"] = value.Currency
	if err := audit.Insert(ctx, tx, entry); err != nil {
		return Reversal{}, false, err
	}

	persisted, found, err := readCashReversal(ctx, tx, value.ID)
	if err != nil {
		return Reversal{}, false, err
	}
	if !found {
		return Reversal{}, false, fmt.Errorf("created cash transaction reversal disappeared: %s", value.ID)
	}
	if err := tx.Commit(ctx); err != nil {
		return Reversal{}, false, fmt.Errorf("commit cash transaction reversal: %w", err)
	}
	return persisted, false, nil
}

func readCashReversal(ctx context.Context, queryer queryRower, reversalID string) (Reversal, bool, error) {
	var value Reversal
	err := queryer.QueryRow(ctx, `
		SELECT
			id::text,
			cash_transaction_id::text,
			transaction_type,
			amount::text,
			currency,
			source_kind,
			reversal_reason,
			reversed_by_actor_kind,
			reversed_by_actor_id,
			reversed_by_actor_display_name,
			reversed_at
		FROM cash_transaction_reversals
		WHERE id = $1
	`, reversalID).Scan(
		&value.ID,
		&value.CashTransactionID,
		&value.TransactionType,
		&value.Amount,
		&value.Currency,
		&value.SourceKind,
		&value.Reason,
		&value.ReversedBy.Kind,
		&value.ReversedBy.ID,
		&value.ReversedBy.DisplayName,
		&value.ReversedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Reversal{}, false, nil
	}
	if err != nil {
		return Reversal{}, false, err
	}
	value.Amount, err = normalizeAmount(value.Amount)
	if err != nil {
		return Reversal{}, false, fmt.Errorf("invalid stored cash transaction reversal amount: %w", err)
	}
	return value, true, nil
}
