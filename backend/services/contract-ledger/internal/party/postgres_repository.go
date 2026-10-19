package party

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/contract-ledger/internal/audit"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Create(ctx context.Context, value Party, entry audit.Entry) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin create party: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	_, err = tx.Exec(ctx, `
		INSERT INTO parties (
			id,
			kind,
			legal_name,
			registration_identifier,
			revision,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7)
	`, value.ID, value.Kind, value.LegalName, value.RegistrationIdentifier, value.Revision, value.CreatedAt, value.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert party: %w", err)
	}
	if err := audit.Insert(ctx, tx, entry); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit create party: %w", err)
	}
	return nil
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
		ORDER BY legal_name, id
	`)
	if err != nil {
		return nil, fmt.Errorf("query parties: %w", err)
	}
	defer rows.Close()

	parties := make([]Party, 0)
	for rows.Next() {
		var value Party
		if err := rows.Scan(
			&value.ID,
			&value.Kind,
			&value.LegalName,
			&value.RegistrationIdentifier,
			&value.Revision,
			&value.CreatedAt,
			&value.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan party: %w", err)
		}
		parties = append(parties, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate parties: %w", err)
	}
	return parties, nil
}
