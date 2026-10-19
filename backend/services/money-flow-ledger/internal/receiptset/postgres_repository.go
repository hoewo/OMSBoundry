package receiptset

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Confirm(ctx context.Context, value Set, allocationIDs []string, command Command, entry audit.Entry) (Set, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Set{}, false, fmt.Errorf("begin receipt set confirmation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, command.ID); err != nil {
		return Set{}, false, fmt.Errorf("lock receipt set command: %w", err)
	}
	var existingID string
	var existingHash []byte
	err = tx.QueryRow(ctx, `SELECT receipt_set_id::text, payload_hash FROM receipt_set_confirm_commands WHERE command_id=$1`, command.ID).Scan(&existingID, &existingHash)
	switch {
	case err == nil:
		if !bytes.Equal(existingHash, command.PayloadHash) {
			return Set{}, false, ErrCommandConflict
		}
		existing, readErr := readSet(ctx, tx, value.TenantID, existingID)
		if readErr != nil {
			return Set{}, false, readErr
		}
		if err := tx.Commit(ctx); err != nil {
			return Set{}, false, fmt.Errorf("commit receipt set replay: %w", err)
		}
		return existing, true, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return Set{}, false, fmt.Errorf("read receipt set command: %w", err)
	}

	items := make([]Item, 0, len(allocationIDs))
	var total int64
	for _, allocationID := range allocationIDs {
		var item Item
		var amount string
		var transactionType string
		var transactionReversed, allocationReversed bool
		err := tx.QueryRow(ctx, `
			SELECT
				a.cash_transaction_id::text,
				a.id::text,
				a.amount::text,
				a.currency,
				ct.type,
				EXISTS (SELECT 1 FROM cash_transaction_reversals ctr WHERE ctr.cash_transaction_id=ct.id),
				EXISTS (SELECT 1 FROM allocation_reversals ar WHERE ar.allocation_id=a.id)
			FROM allocations a
			JOIN cash_transactions ct ON ct.id=a.cash_transaction_id
			WHERE a.id=$1
			FOR SHARE OF a, ct
		`, allocationID).Scan(&item.TransactionID, &item.AllocationID, &amount, &item.Currency, &transactionType, &transactionReversed, &allocationReversed)
		if errors.Is(err, pgx.ErrNoRows) {
			return Set{}, false, ErrNotFound
		}
		if err != nil {
			return Set{}, false, fmt.Errorf("read receipt allocation %s: %w", allocationID, err)
		}
		minor, convertErr := decimalToMinor(amount)
		if convertErr != nil || transactionType != "receipt" || transactionReversed || allocationReversed || item.Currency != CurrencyCNY {
			return Set{}, false, ErrStateConflict
		}
		if minor <= 0 || total > int64(^uint64(0)>>1)-minor {
			return Set{}, false, ErrStateConflict
		}
		item.TransactionRevision = 1
		item.AllocationRevision = 1
		item.AmountMinor = minor
		item.Status = StatusConfirmed
		total += minor
		items = append(items, item)
	}
	value.TotalAmountMinor, value.Items = total, items
	if _, err := tx.Exec(ctx, `
		INSERT INTO receipt_sets (
			id, tenant_id, project_id, status, currency, revision, total_amount_minor,
			confirmed_by_actor_kind, confirmed_by_actor_id, confirmed_by_actor_display_name, confirmed_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`, value.ID, value.TenantID, value.ProjectID, value.Status, value.Currency, value.Revision, value.TotalAmountMinor,
		value.ConfirmedBy.Kind, value.ConfirmedBy.ID, value.ConfirmedBy.DisplayName, value.ConfirmedAt); err != nil {
		return Set{}, false, fmt.Errorf("insert receipt set: %w", err)
	}
	for index, item := range items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO receipt_set_items (
				receipt_set_id, ordinal, transaction_id, transaction_revision,
				allocation_id, allocation_revision, amount_minor, currency, status
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		`, value.ID, index+1, item.TransactionID, item.TransactionRevision, item.AllocationID, item.AllocationRevision, item.AmountMinor, item.Currency, item.Status); err != nil {
			return Set{}, false, fmt.Errorf("insert receipt set item: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO receipt_set_confirm_commands (
			command_id, receipt_set_id, payload_hash, actor_kind, actor_id, actor_display_name, confirmed_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, command.ID, value.ID, command.PayloadHash, command.Actor.Kind, command.Actor.ID, command.Actor.DisplayName, command.OccurredAt); err != nil {
		return Set{}, false, fmt.Errorf("insert receipt set command: %w", err)
	}
	entry.Details["currency"] = value.Currency
	entry.Details["total_amount_minor"] = value.TotalAmountMinor
	entry.Details["receipt_set_revision"] = value.Revision
	if err := audit.Insert(ctx, tx, entry); err != nil {
		return Set{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Set{}, false, fmt.Errorf("commit receipt set: %w", err)
	}
	return value, false, nil
}

func (r *PostgresRepository) Get(ctx context.Context, tenantID, setID string) (Set, error) {
	return readSet(ctx, r.pool, tenantID, setID)
}

type queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func readSet(ctx context.Context, q queryer, tenantID, setID string) (Set, error) {
	var value Set
	err := q.QueryRow(ctx, `
		SELECT id::text, tenant_id::text, project_id::text, status, currency, revision,
			total_amount_minor, confirmed_by_actor_kind, confirmed_by_actor_id,
			confirmed_by_actor_display_name, confirmed_at
		FROM receipt_sets WHERE tenant_id=$1 AND id=$2
	`, tenantID, setID).Scan(&value.ID, &value.TenantID, &value.ProjectID, &value.Status, &value.Currency, &value.Revision,
		&value.TotalAmountMinor, &value.ConfirmedBy.Kind, &value.ConfirmedBy.ID, &value.ConfirmedBy.DisplayName, &value.ConfirmedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Set{}, ErrNotFound
	}
	if err != nil {
		return Set{}, fmt.Errorf("read receipt set: %w", err)
	}
	rows, err := q.Query(ctx, `
		SELECT transaction_id::text, transaction_revision, allocation_id::text,
			allocation_revision, amount_minor, currency, status
		FROM receipt_set_items WHERE receipt_set_id=$1 ORDER BY ordinal
	`, setID)
	if err != nil {
		return Set{}, fmt.Errorf("read receipt set items: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.TransactionID, &item.TransactionRevision, &item.AllocationID, &item.AllocationRevision, &item.AmountMinor, &item.Currency, &item.Status); err != nil {
			return Set{}, fmt.Errorf("scan receipt set item: %w", err)
		}
		value.Items = append(value.Items, item)
	}
	if err := rows.Err(); err != nil {
		return Set{}, fmt.Errorf("iterate receipt set items: %w", err)
	}
	return value, nil
}

func decimalToMinor(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	parts := strings.Split(raw, ".")
	if len(parts) > 2 || len(parts) == 0 || parts[0] == "" {
		return 0, ErrStateConflict
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = strings.TrimRight(parts[1], "0")
	}
	if len(fraction) > 2 {
		return 0, ErrStateConflict
	}
	fraction += strings.Repeat("0", 2-len(fraction))
	minor, ok := new(big.Int).SetString(parts[0]+fraction, 10)
	if !ok || !minor.IsInt64() {
		return 0, ErrStateConflict
	}
	return minor.Int64(), nil
}
