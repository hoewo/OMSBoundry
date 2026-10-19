package party

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
	value Party,
	command CreateCommand,
	entry audit.Entry,
) (Party, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Party{}, false, fmt.Errorf("begin create party: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, command.CommandID); err != nil {
		return Party{}, false, fmt.Errorf("lock party create command: %w", err)
	}
	var existingPartyID string
	var existingPayloadHash []byte
	err = tx.QueryRow(ctx, `
		SELECT party_id::text, payload_hash
		FROM party_create_commands
		WHERE command_id = $1
	`, command.CommandID).Scan(&existingPartyID, &existingPayloadHash)
	switch {
	case err == nil:
		if !bytes.Equal(existingPayloadHash, command.PayloadHash) {
			return Party{}, false, &IdempotencyConflictError{CommandID: command.CommandID}
		}
		existing, err := readParty(ctx, tx, existingPartyID)
		if err != nil {
			return Party{}, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Party{}, false, fmt.Errorf("commit party create replay: %w", err)
		}
		return existing, true, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return Party{}, false, fmt.Errorf("read party create command: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO parties (
			id, kind, legal_name, registration_identifier,
			revision, created_at, updated_at
		)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7)
	`,
		value.ID,
		value.Kind,
		value.LegalName,
		value.RegistrationIdentifier,
		value.Revision,
		value.CreatedAt,
		value.UpdatedAt,
	); err != nil {
		return Party{}, false, fmt.Errorf("insert party: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO party_create_commands (
			command_id, party_id, payload_hash,
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
		return Party{}, false, fmt.Errorf("insert party create command: %w", err)
	}
	if err := audit.Insert(ctx, tx, entry); err != nil {
		return Party{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Party{}, false, fmt.Errorf("commit create party: %w", err)
	}
	return value, false, nil
}

func (r *PostgresRepository) List(ctx context.Context) ([]Party, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			id::text,
			kind,
			legal_name,
			COALESCE(registration_identifier, ''),
			revision,
			created_at,
			updated_at
		FROM parties
		ORDER BY lower(legal_name) COLLATE "C", legal_name COLLATE "C", id
	`)
	if err != nil {
		return nil, fmt.Errorf("query parties: %w", err)
	}
	defer rows.Close()

	parties := make([]Party, 0)
	for rows.Next() {
		value, err := scanParty(rows)
		if err != nil {
			return nil, fmt.Errorf("scan party: %w", err)
		}
		parties = append(parties, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate parties: %w", err)
	}
	return parties, nil
}

func readParty(ctx context.Context, tx pgx.Tx, partyID string) (Party, error) {
	value, err := scanParty(tx.QueryRow(ctx, `
		SELECT
			id::text,
			kind,
			legal_name,
			COALESCE(registration_identifier, ''),
			revision,
			created_at,
			updated_at
		FROM parties
		WHERE id = $1
	`, partyID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Party{}, fmt.Errorf("party create command references missing party %s", partyID)
	}
	if err != nil {
		return Party{}, fmt.Errorf("read party create result: %w", err)
	}
	return value, nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanParty(row rowScanner) (Party, error) {
	var value Party
	err := row.Scan(
		&value.ID,
		&value.Kind,
		&value.LegalName,
		&value.RegistrationIdentifier,
		&value.Revision,
		&value.CreatedAt,
		&value.UpdatedAt,
	)
	return value, err
}
