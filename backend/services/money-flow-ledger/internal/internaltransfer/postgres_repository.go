package internaltransfer

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
	value Transfer,
	command CreateCommand,
	entry audit.Entry,
) (Transfer, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Transfer{}, false, fmt.Errorf("begin create internal transfer: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, command.CommandID); err != nil {
		return Transfer{}, false, fmt.Errorf("lock internal transfer create command: %w", err)
	}
	var existingTransferID string
	var existingPayloadHash []byte
	err = tx.QueryRow(ctx, `
		SELECT internal_transfer_id::text, payload_hash
		FROM internal_transfer_create_commands
		WHERE command_id = $1
	`, command.CommandID).Scan(&existingTransferID, &existingPayloadHash)
	switch {
	case err == nil:
		if !bytes.Equal(existingPayloadHash, command.PayloadHash) {
			return Transfer{}, false, &IdempotencyConflictError{CommandID: command.CommandID}
		}
		existing, found, err := readTransfer(ctx, tx, existingTransferID)
		if err != nil {
			return Transfer{}, false, err
		}
		if !found {
			return Transfer{}, false, fmt.Errorf("internal transfer create command references missing transfer %s", existingTransferID)
		}
		if err := tx.Commit(ctx); err != nil {
			return Transfer{}, false, fmt.Errorf("commit internal transfer replay: %w", err)
		}
		return existing, true, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return Transfer{}, false, fmt.Errorf("read internal transfer create command: %w", err)
	}

	externalLockKey, err := externalReferenceLockKey(value)
	if err != nil {
		return Transfer{}, false, fmt.Errorf("build external transfer reference lock: %w", err)
	}
	if externalLockKey != "" {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, externalLockKey); err != nil {
			return Transfer{}, false, fmt.Errorf("lock external transfer reference: %w", err)
		}
		var duplicateID string
		err := tx.QueryRow(ctx, `
			SELECT id::text
			FROM internal_transfers
			WHERE ledger_owner_party_id = $1
			  AND lower(source_account_reference) = lower($2)
			  AND lower(destination_account_reference) = lower($3)
			  AND source_kind = $4
			  AND lower(external_transfer_reference) = lower($5)
		`,
			value.LedgerOwner.PartyID,
			value.SourceAccountReference,
			value.DestinationAccountReference,
			value.SourceKind,
			value.ExternalTransferReference,
		).Scan(&duplicateID)
		switch {
		case err == nil:
			return Transfer{}, false, &ExternalReferenceConflictError{ExistingTransferID: duplicateID}
		case !errors.Is(err, pgx.ErrNoRows):
			return Transfer{}, false, fmt.Errorf("check external transfer reference: %w", err)
		}
	}

	if err := tx.QueryRow(ctx, `
		SELECT
			id::text,
			revision,
			kind,
			legal_name,
			COALESCE(registration_identifier, '')
		FROM parties
		WHERE id = $1
		FOR SHARE
	`, value.LedgerOwner.PartyID).Scan(
		&value.LedgerOwner.PartyID,
		&value.LedgerOwner.PartyRevision,
		&value.LedgerOwner.Kind,
		&value.LedgerOwner.LegalName,
		&value.LedgerOwner.RegistrationIdentifier,
	); errors.Is(err, pgx.ErrNoRows) {
		return Transfer{}, false, &PartyNotFoundError{PartyID: value.LedgerOwner.PartyID}
	} else if err != nil {
		return Transfer{}, false, fmt.Errorf("read internal transfer party snapshot: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO internal_transfers (
			id, description,
			ledger_owner_party_id, ledger_owner_party_revision, ledger_owner_kind,
			ledger_owner_legal_name, ledger_owner_registration_identifier,
			amount, currency, occurred_on,
			source_account_reference, destination_account_reference,
			source_kind, manual_source_reference, external_transfer_reference,
			recorded_by_actor_kind, recorded_by_actor_id,
			recorded_by_actor_display_name, recorded_at
		)
		VALUES (
			$1, $2,
			$3, $4, $5, $6, NULLIF($7, ''),
			$8::numeric, $9, $10::date,
			$11, $12,
			$13, $14, NULLIF($15, ''),
			$16, $17, $18, $19
		)
	`,
		value.ID,
		value.Description,
		value.LedgerOwner.PartyID,
		value.LedgerOwner.PartyRevision,
		value.LedgerOwner.Kind,
		value.LedgerOwner.LegalName,
		value.LedgerOwner.RegistrationIdentifier,
		value.Amount,
		value.Currency,
		value.OccurredOn,
		value.SourceAccountReference,
		value.DestinationAccountReference,
		value.SourceKind,
		value.ManualSourceReference,
		value.ExternalTransferReference,
		value.RecordedBy.Kind,
		value.RecordedBy.ID,
		value.RecordedBy.DisplayName,
		value.RecordedAt,
	); err != nil {
		return Transfer{}, false, fmt.Errorf("insert internal transfer: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO internal_transfer_create_commands (
			command_id, internal_transfer_id, payload_hash,
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
		return Transfer{}, false, fmt.Errorf("insert internal transfer create command: %w", err)
	}
	if err := audit.Insert(ctx, tx, entry); err != nil {
		return Transfer{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Transfer{}, false, fmt.Errorf("commit create internal transfer: %w", err)
	}
	return value, false, nil
}

func (r *PostgresRepository) List(ctx context.Context) ([]Transfer, error) {
	rows, err := r.pool.Query(ctx, transferSelect+` ORDER BY recorded_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("query internal transfers: %w", err)
	}
	defer rows.Close()

	values := make([]Transfer, 0)
	for rows.Next() {
		value, err := scanTransfer(rows)
		if err != nil {
			return nil, fmt.Errorf("scan internal transfer: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate internal transfers: %w", err)
	}
	return values, nil
}

func (r *PostgresRepository) Get(ctx context.Context, transferID string) (Transfer, bool, error) {
	value, found, err := readTransfer(ctx, r.pool, transferID)
	if err != nil {
		return Transfer{}, false, fmt.Errorf("query internal transfer: %w", err)
	}
	return value, found, nil
}

const transferSelect = `
	SELECT
		id::text,
		description,
		ledger_owner_party_id::text,
		ledger_owner_party_revision,
		ledger_owner_kind,
		ledger_owner_legal_name,
		COALESCE(ledger_owner_registration_identifier, ''),
		amount::text,
		currency,
		occurred_on::text,
		source_account_reference,
		destination_account_reference,
		source_kind,
		manual_source_reference,
		COALESCE(external_transfer_reference, ''),
		recorded_by_actor_kind,
		recorded_by_actor_id,
		recorded_by_actor_display_name,
		recorded_at
	FROM internal_transfers
`

type queryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func readTransfer(ctx context.Context, queryer queryRower, transferID string) (Transfer, bool, error) {
	value, err := scanTransfer(queryer.QueryRow(ctx, transferSelect+` WHERE id = $1`, transferID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Transfer{}, false, nil
	}
	if err != nil {
		return Transfer{}, false, err
	}
	return value, true, nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanTransfer(row rowScanner) (Transfer, error) {
	var value Transfer
	err := row.Scan(
		&value.ID,
		&value.Description,
		&value.LedgerOwner.PartyID,
		&value.LedgerOwner.PartyRevision,
		&value.LedgerOwner.Kind,
		&value.LedgerOwner.LegalName,
		&value.LedgerOwner.RegistrationIdentifier,
		&value.Amount,
		&value.Currency,
		&value.OccurredOn,
		&value.SourceAccountReference,
		&value.DestinationAccountReference,
		&value.SourceKind,
		&value.ManualSourceReference,
		&value.ExternalTransferReference,
		&value.RecordedBy.Kind,
		&value.RecordedBy.ID,
		&value.RecordedBy.DisplayName,
		&value.RecordedAt,
	)
	if err != nil {
		return Transfer{}, err
	}
	normalizedAmount, err := normalizeAmount(value.Amount)
	if err != nil {
		return Transfer{}, fmt.Errorf("invalid stored internal transfer amount: %w", err)
	}
	value.Amount = normalizedAmount
	value.Status = StatusRecorded
	return value, nil
}
