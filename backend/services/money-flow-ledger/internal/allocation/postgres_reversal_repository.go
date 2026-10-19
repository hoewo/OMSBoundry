package allocation

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

type allocationFacts struct {
	CashTransactionID  string
	ObligationID       string
	ObligationRevision int64
	Amount             string
	Currency           string
}

func (r *PostgresRepository) Reverse(
	ctx context.Context,
	value Reversal,
	command ReverseCommand,
	entry audit.Entry,
) (Reversal, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Reversal{}, false, fmt.Errorf("begin reverse allocation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, command.CommandID); err != nil {
		return Reversal{}, false, fmt.Errorf("lock allocation reversal command: %w", err)
	}
	var existingReversalID string
	var existingPayloadHash []byte
	err = tx.QueryRow(ctx, `
		SELECT allocation_reversal_id::text, payload_hash
		FROM allocation_reversal_create_commands
		WHERE command_id = $1
	`, command.CommandID).Scan(&existingReversalID, &existingPayloadHash)
	switch {
	case err == nil:
		if !bytes.Equal(existingPayloadHash, command.PayloadHash) {
			return Reversal{}, false, &ReversalIdempotencyConflictError{CommandID: command.CommandID}
		}
		existing, found, err := readReversal(ctx, tx, existingReversalID)
		if err != nil {
			return Reversal{}, false, err
		}
		if !found {
			return Reversal{}, false, fmt.Errorf("allocation reversal command references missing reversal %s", existingReversalID)
		}
		if err := tx.Commit(ctx); err != nil {
			return Reversal{}, false, fmt.Errorf("commit allocation reversal replay: %w", err)
		}
		return existing, true, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return Reversal{}, false, fmt.Errorf("read allocation reversal command: %w", err)
	}

	facts, found, err := readAllocationFacts(ctx, tx, value.AllocationID, "")
	if err != nil {
		return Reversal{}, false, fmt.Errorf("read allocation for reversal: %w", err)
	}
	if !found {
		return Reversal{}, false, &AllocationNotFoundError{AllocationID: value.AllocationID}
	}
	if _, err := tx.Exec(ctx, `SELECT 1 FROM cash_transactions WHERE id = $1 FOR UPDATE`, facts.CashTransactionID); err != nil {
		return Reversal{}, false, fmt.Errorf("lock cash transaction for allocation reversal: %w", err)
	}
	var obligationStatus string
	if err := tx.QueryRow(ctx, `SELECT status FROM obligations WHERE id = $1 FOR UPDATE`, facts.ObligationID).Scan(&obligationStatus); err != nil {
		return Reversal{}, false, fmt.Errorf("lock obligation for allocation reversal: %w", err)
	}
	if obligationStatus != "open" {
		return Reversal{}, false, &ReversalObligationStateConflictError{Status: obligationStatus}
	}
	facts, found, err = readAllocationFacts(ctx, tx, value.AllocationID, " FOR UPDATE")
	if err != nil {
		return Reversal{}, false, fmt.Errorf("lock allocation for reversal: %w", err)
	}
	if !found {
		return Reversal{}, false, &AllocationNotFoundError{AllocationID: value.AllocationID}
	}

	var previousReversalID string
	err = tx.QueryRow(ctx, `
		SELECT id::text
		FROM allocation_reversals
		WHERE allocation_id = $1
	`, value.AllocationID).Scan(&previousReversalID)
	switch {
	case err == nil:
		return Reversal{}, false, &AllocationAlreadyReversedError{
			AllocationID: value.AllocationID,
			ReversalID:   previousReversalID,
		}
	case !errors.Is(err, pgx.ErrNoRows):
		return Reversal{}, false, fmt.Errorf("check existing allocation reversal: %w", err)
	}

	value.CashTransactionID = facts.CashTransactionID
	value.ObligationID = facts.ObligationID
	value.ObligationRevision = facts.ObligationRevision
	value.Amount = facts.Amount
	value.Currency = facts.Currency
	if _, err := tx.Exec(ctx, `
		INSERT INTO allocation_reversals (
			id, allocation_id, cash_transaction_id,
			obligation_id, obligation_revision,
			amount, currency, source_kind, reversal_reason,
			reversed_by_actor_kind, reversed_by_actor_id,
			reversed_by_actor_display_name, reversed_at
		)
		VALUES (
			$1, $2, $3,
			$4, $5,
			$6::numeric, $7, $8, $9,
			$10, $11, $12, $13
		)
	`,
		value.ID,
		value.AllocationID,
		value.CashTransactionID,
		value.ObligationID,
		value.ObligationRevision,
		value.Amount,
		value.Currency,
		value.SourceKind,
		value.Reason,
		value.ReversedBy.Kind,
		value.ReversedBy.ID,
		value.ReversedBy.DisplayName,
		value.ReversedAt,
	); err != nil {
		return Reversal{}, false, fmt.Errorf("insert allocation reversal: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO allocation_reversal_create_commands (
			command_id, allocation_reversal_id, allocation_id,
			payload_hash, actor_kind, actor_id, actor_display_name, reversed_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`,
		command.CommandID,
		value.ID,
		value.AllocationID,
		command.PayloadHash,
		command.Actor.Kind,
		command.Actor.ID,
		command.Actor.DisplayName,
		command.ReversedAt,
	); err != nil {
		return Reversal{}, false, fmt.Errorf("insert allocation reversal command: %w", err)
	}
	if entry.Details == nil {
		entry.Details = make(map[string]any)
	}
	entry.Details["allocation_reversal_id"] = value.ID
	entry.Details["cash_transaction_id"] = value.CashTransactionID
	entry.Details["obligation_id"] = value.ObligationID
	entry.Details["obligation_revision"] = value.ObligationRevision
	entry.Details["amount"] = value.Amount
	entry.Details["currency"] = value.Currency
	if err := audit.Insert(ctx, tx, entry); err != nil {
		return Reversal{}, false, err
	}

	persisted, found, err := readReversal(ctx, tx, value.ID)
	if err != nil {
		return Reversal{}, false, err
	}
	if !found {
		return Reversal{}, false, fmt.Errorf("created allocation reversal disappeared: %s", value.ID)
	}
	if err := tx.Commit(ctx); err != nil {
		return Reversal{}, false, fmt.Errorf("commit allocation reversal: %w", err)
	}
	return persisted, false, nil
}

func readAllocationFacts(ctx context.Context, queryer queryRower, allocationID, lockClause string) (allocationFacts, bool, error) {
	var value allocationFacts
	err := queryer.QueryRow(ctx, `
		SELECT
			cash_transaction_id::text,
			obligation_id::text,
			obligation_revision,
			amount::text,
			currency
		FROM allocations
		WHERE id = $1`+lockClause, allocationID).Scan(
		&value.CashTransactionID,
		&value.ObligationID,
		&value.ObligationRevision,
		&value.Amount,
		&value.Currency,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return allocationFacts{}, false, nil
	}
	if err != nil {
		return allocationFacts{}, false, err
	}
	normalizedAmount, err := normalizeStoredAmount(value.Amount)
	if err != nil {
		return allocationFacts{}, false, fmt.Errorf("invalid stored allocation amount for reversal: %w", err)
	}
	value.Amount = normalizedAmount
	return value, true, nil
}

func readReversal(ctx context.Context, queryer queryRower, reversalID string) (Reversal, bool, error) {
	var value Reversal
	err := queryer.QueryRow(ctx, `
		SELECT
			id::text,
			allocation_id::text,
			cash_transaction_id::text,
			obligation_id::text,
			obligation_revision,
			amount::text,
			currency,
			source_kind,
			reversal_reason,
			reversed_by_actor_kind,
			reversed_by_actor_id,
			reversed_by_actor_display_name,
			reversed_at
		FROM allocation_reversals
		WHERE id = $1
	`, reversalID).Scan(
		&value.ID,
		&value.AllocationID,
		&value.CashTransactionID,
		&value.ObligationID,
		&value.ObligationRevision,
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
	normalizedAmount, err := normalizeStoredAmount(value.Amount)
	if err != nil {
		return Reversal{}, false, fmt.Errorf("invalid stored allocation reversal amount: %w", err)
	}
	value.Amount = normalizedAmount
	return value, true, nil
}
