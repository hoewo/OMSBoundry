package cashtransaction

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Create(
	ctx context.Context,
	value Transaction,
	command CreateCommand,
	entry audit.Entry,
) (Transaction, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Transaction{}, false, fmt.Errorf("begin create cash transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, command.CommandID); err != nil {
		return Transaction{}, false, fmt.Errorf("lock cash transaction create command: %w", err)
	}
	var existingTransactionID string
	var existingPayloadHash []byte
	err = tx.QueryRow(ctx, `
		SELECT cash_transaction_id::text, payload_hash
		FROM cash_transaction_create_commands
		WHERE command_id = $1
	`, command.CommandID).Scan(&existingTransactionID, &existingPayloadHash)
	switch {
	case err == nil:
		if !bytes.Equal(existingPayloadHash, command.PayloadHash) {
			return Transaction{}, false, &IdempotencyConflictError{CommandID: command.CommandID}
		}
		existing, found, err := readTransaction(ctx, tx, existingTransactionID)
		if err != nil {
			return Transaction{}, false, err
		}
		if !found {
			return Transaction{}, false, fmt.Errorf("cash transaction create command references missing transaction %s", existingTransactionID)
		}
		if err := tx.Commit(ctx); err != nil {
			return Transaction{}, false, fmt.Errorf("commit cash transaction replay: %w", err)
		}
		return existing, true, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return Transaction{}, false, fmt.Errorf("read cash transaction create command: %w", err)
	}

	externalLockKey, err := externalReferenceLockKey(value)
	if err != nil {
		return Transaction{}, false, fmt.Errorf("build external transaction reference lock: %w", err)
	}
	if externalLockKey != "" {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, externalLockKey); err != nil {
			return Transaction{}, false, fmt.Errorf("lock external transaction reference: %w", err)
		}
		var duplicateID string
		err := tx.QueryRow(ctx, `
			SELECT id::text
			FROM cash_transactions
			WHERE ledger_owner_party_id = $1
			  AND lower(account_reference) = lower($2)
			  AND source_kind = $3
			  AND lower(external_transaction_reference) = lower($4)
		`,
			value.LedgerOwnerPartyID,
			value.AccountReference,
			value.SourceKind,
			value.ExternalTransactionReference,
		).Scan(&duplicateID)
		switch {
		case err == nil:
			return Transaction{}, false, &ExternalReferenceConflictError{ExistingTransactionID: duplicateID}
		case !errors.Is(err, pgx.ErrNoRows):
			return Transaction{}, false, fmt.Errorf("check external transaction reference: %w", err)
		}
	}

	snapshots, err := loadPartySnapshots(ctx, tx, value.Payer.PartyID, value.Payee.PartyID)
	if err != nil {
		return Transaction{}, false, err
	}
	payer, found := snapshots[value.Payer.PartyID]
	if !found {
		return Transaction{}, false, &PartyNotFoundError{PartyID: value.Payer.PartyID}
	}
	payee, found := snapshots[value.Payee.PartyID]
	if !found {
		return Transaction{}, false, &PartyNotFoundError{PartyID: value.Payee.PartyID}
	}
	value.Payer = payer
	value.Payee = payee

	if _, err := tx.Exec(ctx, `
		INSERT INTO cash_transactions (
			id, type, description, ledger_owner_party_id,
			payer_party_id, payer_party_revision, payer_kind,
			payer_legal_name, payer_registration_identifier,
			payee_party_id, payee_party_revision, payee_kind,
			payee_legal_name, payee_registration_identifier,
			amount, currency, occurred_on, account_reference,
			source_kind, manual_source_reference, external_transaction_reference,
			recorded_by_actor_kind, recorded_by_actor_id,
			recorded_by_actor_display_name, recorded_at
		)
		VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8, NULLIF($9, ''),
			$10, $11, $12, $13, NULLIF($14, ''),
			$15::numeric, $16, $17::date, $18,
			$19, $20, NULLIF($21, ''),
			$22, $23, $24, $25
		)
	`,
		value.ID,
		value.Type,
		value.Description,
		value.LedgerOwnerPartyID,
		value.Payer.PartyID,
		value.Payer.PartyRevision,
		value.Payer.Kind,
		value.Payer.LegalName,
		value.Payer.RegistrationIdentifier,
		value.Payee.PartyID,
		value.Payee.PartyRevision,
		value.Payee.Kind,
		value.Payee.LegalName,
		value.Payee.RegistrationIdentifier,
		value.Amount,
		value.Currency,
		value.OccurredOn,
		value.AccountReference,
		value.SourceKind,
		value.ManualSourceReference,
		value.ExternalTransactionReference,
		value.RecordedBy.Kind,
		value.RecordedBy.ID,
		value.RecordedBy.DisplayName,
		value.RecordedAt,
	); err != nil {
		return Transaction{}, false, fmt.Errorf("insert cash transaction: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO cash_transaction_create_commands (
			command_id, cash_transaction_id, payload_hash,
			actor_kind, actor_id, actor_display_name, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`,
		command.CommandID,
		value.ID,
		command.PayloadHash,
		command.Actor.Kind,
		command.Actor.ID,
		command.Actor.DisplayName,
		command.CreatedAt,
	); err != nil {
		return Transaction{}, false, fmt.Errorf("insert cash transaction create command: %w", err)
	}
	if err := audit.Insert(ctx, tx, entry); err != nil {
		return Transaction{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Transaction{}, false, fmt.Errorf("commit create cash transaction: %w", err)
	}
	return value, false, nil
}

func (r *PostgresRepository) List(ctx context.Context) ([]Transaction, error) {
	rows, err := r.pool.Query(ctx, transactionSelect+`
		ORDER BY ct.recorded_at DESC, ct.id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query cash transactions: %w", err)
	}
	defer rows.Close()

	values := make([]Transaction, 0)
	for rows.Next() {
		value, err := scanTransaction(rows)
		if err != nil {
			return nil, fmt.Errorf("scan cash transaction: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cash transactions: %w", err)
	}
	return values, nil
}

func (r *PostgresRepository) Get(ctx context.Context, transactionID string) (Transaction, bool, error) {
	value, found, err := readTransaction(ctx, r.pool, transactionID)
	if err != nil {
		return Transaction{}, false, fmt.Errorf("query cash transaction: %w", err)
	}
	return value, found, nil
}

func loadPartySnapshots(ctx context.Context, tx pgx.Tx, firstPartyID, secondPartyID string) (map[string]PartySnapshot, error) {
	rows, err := tx.Query(ctx, `
		SELECT
			id::text,
			revision,
			kind,
			legal_name,
			COALESCE(registration_identifier, '')
		FROM parties
		WHERE id = $1 OR id = $2
		ORDER BY id
		FOR SHARE
	`, firstPartyID, secondPartyID)
	if err != nil {
		return nil, fmt.Errorf("read cash transaction party snapshots: %w", err)
	}
	defer rows.Close()

	values := make(map[string]PartySnapshot, 2)
	for rows.Next() {
		var value PartySnapshot
		if err := rows.Scan(
			&value.PartyID,
			&value.PartyRevision,
			&value.Kind,
			&value.LegalName,
			&value.RegistrationIdentifier,
		); err != nil {
			return nil, fmt.Errorf("scan cash transaction party snapshot: %w", err)
		}
		values[value.PartyID] = value
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cash transaction party snapshots: %w", err)
	}
	return values, nil
}

const transactionSelect = `
	SELECT
		ct.id::text,
		ct.type,
		ct.description,
		ct.ledger_owner_party_id::text,
		ct.payer_party_id::text,
		ct.payer_party_revision,
		ct.payer_kind,
		ct.payer_legal_name,
		COALESCE(ct.payer_registration_identifier, ''),
		ct.payee_party_id::text,
		ct.payee_party_revision,
		ct.payee_kind,
		ct.payee_legal_name,
		COALESCE(ct.payee_registration_identifier, ''),
		ct.amount::text,
		(CASE WHEN ctr.id IS NULL THEN ct.amount ELSE 0::numeric END)::text,
		allocation_totals.allocated_amount::text,
		(CASE WHEN ctr.id IS NULL THEN ct.amount ELSE 0::numeric END - allocation_totals.allocated_amount)::text,
		ct.currency,
		ct.occurred_on::text,
		ct.account_reference,
		ct.source_kind,
		ct.manual_source_reference,
		COALESCE(ct.external_transaction_reference, ''),
		ct.recorded_by_actor_kind,
		ct.recorded_by_actor_id,
		ct.recorded_by_actor_display_name,
		ct.recorded_at,
		COALESCE(ctr.id::text, ''),
		COALESCE(ctr.cash_transaction_id::text, ''),
		COALESCE(ctr.transaction_type, ''),
		COALESCE(ctr.amount::text, ''),
		COALESCE(ctr.currency, ''),
		COALESCE(ctr.source_kind, ''),
		COALESCE(ctr.reversal_reason, ''),
		COALESCE(ctr.reversed_by_actor_kind, ''),
		COALESCE(ctr.reversed_by_actor_id, ''),
		COALESCE(ctr.reversed_by_actor_display_name, ''),
		COALESCE(ctr.reversed_at, 'epoch'::timestamptz)
	FROM cash_transactions ct
	LEFT JOIN LATERAL (
		SELECT COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0) AS allocated_amount
		FROM allocations a
		LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
		WHERE a.cash_transaction_id = ct.id
	) allocation_totals ON true
	LEFT JOIN cash_transaction_reversals ctr ON ctr.cash_transaction_id = ct.id
`

type queryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func readTransaction(ctx context.Context, queryer queryRower, transactionID string) (Transaction, bool, error) {
	value, err := scanTransaction(queryer.QueryRow(ctx, transactionSelect+` WHERE ct.id = $1`, transactionID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Transaction{}, false, nil
	}
	if err != nil {
		return Transaction{}, false, err
	}
	return value, true, nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanTransaction(row rowScanner) (Transaction, error) {
	var value Transaction
	var reversal Reversal
	err := row.Scan(
		&value.ID,
		&value.Type,
		&value.Description,
		&value.LedgerOwnerPartyID,
		&value.Payer.PartyID,
		&value.Payer.PartyRevision,
		&value.Payer.Kind,
		&value.Payer.LegalName,
		&value.Payer.RegistrationIdentifier,
		&value.Payee.PartyID,
		&value.Payee.PartyRevision,
		&value.Payee.Kind,
		&value.Payee.LegalName,
		&value.Payee.RegistrationIdentifier,
		&value.Amount,
		&value.EffectiveAmount,
		&value.AllocatedAmount,
		&value.UnallocatedAmount,
		&value.Currency,
		&value.OccurredOn,
		&value.AccountReference,
		&value.SourceKind,
		&value.ManualSourceReference,
		&value.ExternalTransactionReference,
		&value.RecordedBy.Kind,
		&value.RecordedBy.ID,
		&value.RecordedBy.DisplayName,
		&value.RecordedAt,
		&reversal.ID,
		&reversal.CashTransactionID,
		&reversal.TransactionType,
		&reversal.Amount,
		&reversal.Currency,
		&reversal.SourceKind,
		&reversal.Reason,
		&reversal.ReversedBy.Kind,
		&reversal.ReversedBy.ID,
		&reversal.ReversedBy.DisplayName,
		&reversal.ReversedAt,
	)
	if err != nil {
		return Transaction{}, err
	}
	normalizedAmount, err := normalizeAmount(value.Amount)
	if err != nil {
		return Transaction{}, fmt.Errorf("invalid stored cash transaction amount: %w", err)
	}
	value.Amount = normalizedAmount
	normalizedEffectiveAmount, err := normalizeAmountAllowZero(value.EffectiveAmount)
	if err != nil {
		return Transaction{}, fmt.Errorf("invalid stored cash transaction effective amount: %w", err)
	}
	value.EffectiveAmount = normalizedEffectiveAmount
	normalizedAllocatedAmount, err := normalizeAmountAllowZero(value.AllocatedAmount)
	if err != nil {
		return Transaction{}, fmt.Errorf("invalid stored cash transaction allocated amount: %w", err)
	}
	value.AllocatedAmount = normalizedAllocatedAmount
	normalizedUnallocatedAmount, err := normalizeAmountAllowZero(value.UnallocatedAmount)
	if err != nil {
		return Transaction{}, fmt.Errorf("invalid stored cash transaction unallocated amount: %w", err)
	}
	value.UnallocatedAmount = normalizedUnallocatedAmount
	if reversal.ID == "" {
		value.Status = StatusRecorded
		return value, nil
	}
	normalizedReversalAmount, err := normalizeAmount(reversal.Amount)
	if err != nil {
		return Transaction{}, fmt.Errorf("invalid stored cash transaction reversal amount: %w", err)
	}
	reversal.Amount = normalizedReversalAmount
	value.Reversal = &reversal
	value.Status = StatusReversed
	return value, nil
}
