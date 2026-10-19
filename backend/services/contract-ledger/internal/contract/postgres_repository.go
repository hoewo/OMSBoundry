package contract

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/contract-ledger/internal/audit"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const contractColumns = `
	id::text,
	command_id::text,
	number_namespace,
	contract_number,
	title,
	COALESCE(category, ''),
	currency,
	COALESCE(total_amount::text, ''),
	COALESCE(to_char(signed_on, 'YYYY-MM-DD'), ''),
	COALESCE(to_char(effective_on, 'YYYY-MM-DD'), ''),
	status,
	draft_version,
	COALESCE(current_revision, 0),
	created_at,
	updated_at
`

const contractListColumns = `
	c.id::text,
	c.command_id::text,
	CASE WHEN c.current_revision IS NULL THEN c.number_namespace ELSE r.number_namespace END,
	CASE WHEN c.current_revision IS NULL THEN c.contract_number ELSE r.contract_number END,
	CASE WHEN c.current_revision IS NULL THEN c.title ELSE r.title END,
	CASE WHEN c.current_revision IS NULL THEN COALESCE(c.category, '') ELSE COALESCE(r.category, '') END,
	CASE WHEN c.current_revision IS NULL THEN c.currency ELSE r.currency END,
	CASE
		WHEN c.current_revision IS NULL THEN COALESCE(c.total_amount::text, '')
		ELSE COALESCE(r.total_amount::text, '')
	END,
	CASE WHEN c.current_revision IS NULL THEN COALESCE(to_char(c.signed_on, 'YYYY-MM-DD'), '') ELSE to_char(r.signed_on, 'YYYY-MM-DD') END,
	CASE WHEN c.current_revision IS NULL THEN COALESCE(to_char(c.effective_on, 'YYYY-MM-DD'), '') ELSE to_char(r.effective_on, 'YYYY-MM-DD') END,
	c.status,
	c.draft_version,
	COALESCE(c.current_revision, 0),
	c.created_at,
	c.updated_at
`

func (r *PostgresRepository) CreateDraft(ctx context.Context, value Contract, entry audit.Entry) (Contract, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Contract{}, false, fmt.Errorf("begin create contract draft: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	row := tx.QueryRow(ctx, `
		INSERT INTO contracts (
			id,
			command_id,
			number_namespace,
			contract_number,
			title,
			category,
			currency,
			total_amount,
			signed_on,
			effective_on,
			status,
			draft_version,
			current_revision,
			created_at,
			updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, NULLIF($6, ''), $7,
			NULLIF($8, '')::numeric,
			NULLIF($9, '')::date,
			NULLIF($10, '')::date,
			$11, $12, NULLIF($13, 0), $14, $15
		)
		ON CONFLICT (command_id) DO NOTHING
		RETURNING `+contractColumns,
		value.ID,
		value.CommandID,
		value.NumberNamespace,
		value.ContractNumber,
		value.Title,
		value.Category,
		value.Currency,
		value.TotalAmount,
		value.SignedOn,
		value.EffectiveOn,
		value.Status,
		value.DraftVersion,
		value.CurrentRevision,
		value.CreatedAt,
		value.UpdatedAt,
	)

	created, err := scanContract(row)
	if err == nil {
		if err := audit.Insert(ctx, tx, entry); err != nil {
			return Contract{}, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Contract{}, false, fmt.Errorf("commit create contract draft: %w", err)
		}
		return created, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Contract{}, false, mapWriteError(err, value)
	}

	existing, err := scanContract(tx.QueryRow(ctx, `SELECT `+contractColumns+` FROM contracts WHERE command_id = $1`, value.CommandID))
	if err != nil {
		return Contract{}, false, fmt.Errorf("read idempotent contract draft: %w", err)
	}
	return existing, false, nil
}

func (r *PostgresRepository) List(ctx context.Context) ([]Contract, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+contractListColumns+`
		FROM contracts c
		LEFT JOIN contract_revisions r
			ON r.contract_id = c.id AND r.revision = c.current_revision
		ORDER BY c.created_at DESC, c.id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query contracts: %w", err)
	}
	defer rows.Close()

	contracts := make([]Contract, 0)
	for rows.Next() {
		value, err := scanContract(rows)
		if err != nil {
			return nil, err
		}
		contracts = append(contracts, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contracts: %w", err)
	}
	return contracts, nil
}

func (r *PostgresRepository) Search(ctx context.Context, query ContractSearchQuery) ([]Contract, error) {
	if query.Limit < 1 || query.Limit > 101 {
		return nil, fmt.Errorf("contract search limit is outside repository bounds")
	}
	var afterCreatedAt any
	var afterID any
	if !query.AfterCreatedAt.IsZero() {
		afterCreatedAt = query.AfterCreatedAt
		afterID = query.AfterID
	}
	keyword := escapeContractSearchKeyword(query.Keyword)
	rows, err := r.pool.Query(ctx, `
		SELECT `+contractListColumns+`
		FROM contracts c
		LEFT JOIN contract_revisions r
			ON r.contract_id = c.id AND r.revision = c.current_revision
		WHERE (
			$1 = ''
			OR lower(CASE WHEN c.current_revision IS NULL THEN c.number_namespace ELSE r.number_namespace END) LIKE '%' || lower($1) || '%' ESCAPE E'\\'
			OR lower(CASE WHEN c.current_revision IS NULL THEN c.contract_number ELSE r.contract_number END) LIKE '%' || lower($1) || '%' ESCAPE E'\\'
			OR lower(CASE WHEN c.current_revision IS NULL THEN c.title ELSE r.title END) LIKE '%' || lower($1) || '%' ESCAPE E'\\'
		)
		AND ($2 = '' OR c.status = $2)
		AND ($3 = '' OR CASE WHEN c.current_revision IS NULL THEN c.currency ELSE r.currency END = $3)
		AND (
			$4::timestamptz IS NULL
			OR (c.created_at, c.id) < ($4::timestamptz, $5::uuid)
		)
		ORDER BY c.created_at DESC, c.id DESC
		LIMIT $6
	`, keyword, string(query.Status), query.Currency, afterCreatedAt, afterID, query.Limit)
	if err != nil {
		return nil, fmt.Errorf("search contracts: %w", err)
	}
	defer rows.Close()

	contracts := make([]Contract, 0, query.Limit)
	for rows.Next() {
		value, err := scanContract(rows)
		if err != nil {
			return nil, err
		}
		contracts = append(contracts, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate searched contracts: %w", err)
	}
	return contracts, nil
}

func escapeContractSearchKeyword(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}

type rowScanner interface {
	Scan(...any) error
}

func scanContract(row rowScanner) (Contract, error) {
	var value Contract
	if err := row.Scan(
		&value.ID,
		&value.CommandID,
		&value.NumberNamespace,
		&value.ContractNumber,
		&value.Title,
		&value.Category,
		&value.Currency,
		&value.TotalAmount,
		&value.SignedOn,
		&value.EffectiveOn,
		&value.Status,
		&value.DraftVersion,
		&value.CurrentRevision,
		&value.CreatedAt,
		&value.UpdatedAt,
	); err != nil {
		return Contract{}, err
	}

	amount, err := normalizeAmount(value.TotalAmount)
	if err != nil {
		return Contract{}, fmt.Errorf("normalize stored contract amount: %w", err)
	}
	value.TotalAmount = amount
	return value, nil
}

func mapWriteError(err error, value Contract) error {
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && databaseError.Code == "23505" && databaseError.ConstraintName == "contracts_number_scope_unique" {
		return &DuplicateNumberError{
			NumberNamespace: value.NumberNamespace,
			ContractNumber:  value.ContractNumber,
		}
	}
	return fmt.Errorf("insert contract draft: %w", err)
}
