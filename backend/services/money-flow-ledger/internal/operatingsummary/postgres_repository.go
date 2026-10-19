package operatingsummary

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) ListGroups(ctx context.Context) ([]Group, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("begin current operating summary read: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if err := validateCurrentProjection(ctx, tx); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, currentSummaryQuery)
	if err != nil {
		return nil, fmt.Errorf("query current operating summary: %w", err)
	}
	groups := make([]Group, 0)
	for rows.Next() {
		var group Group
		if err := rows.Scan(
			&group.LedgerOwnerPartyID,
			&group.LedgerOwnerLegalName,
			&group.Currency,
			&group.ReceivableOutstanding.Amount,
			&group.ReceivableOutstanding.FactCount,
			&group.PayableOutstanding.Amount,
			&group.PayableOutstanding.FactCount,
			&group.ReceiptEffective.Amount,
			&group.ReceiptEffective.FactCount,
			&group.PaymentEffective.Amount,
			&group.PaymentEffective.FactCount,
			&group.ReceiptUnallocated.Amount,
			&group.ReceiptUnallocated.FactCount,
			&group.PaymentUnallocated.Amount,
			&group.PaymentUnallocated.FactCount,
		); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan current operating summary: %w", err)
		}
		groups = append(groups, group)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate current operating summary: %w", err)
	}
	rows.Close()
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit current operating summary read: %w", err)
	}
	return groups, nil
}

func (r *PostgresRepository) ListObligationFacts(
	ctx context.Context,
	ledgerOwnerPartyID string,
	currency string,
	direction string,
) ([]ObligationFact, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("begin current obligation summary drilldown: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := validateCurrentProjection(ctx, tx); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, currentObligationFactsQuery, ledgerOwnerPartyID, currency, direction)
	if err != nil {
		return nil, fmt.Errorf("query current obligation summary facts: %w", err)
	}
	facts := make([]ObligationFact, 0)
	for rows.Next() {
		var fact ObligationFact
		if err := rows.Scan(
			&fact.ID,
			&fact.Title,
			&fact.Direction,
			&fact.LedgerOwnerPartyID,
			&fact.LedgerOwnerLegalName,
			&fact.CounterpartyLegalName,
			&fact.Currency,
			&fact.DueOn,
			&fact.CurrentRevision,
			&fact.EffectiveAmount,
			&fact.NetAllocatedAmount,
			&fact.RemittedAmount,
			&fact.OutstandingAmount,
			&fact.ContributionAmount,
		); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan current obligation summary fact: %w", err)
		}
		facts = append(facts, fact)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate current obligation summary facts: %w", err)
	}
	rows.Close()
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit current obligation summary drilldown: %w", err)
	}
	return facts, nil
}

func (r *PostgresRepository) ListCashFacts(
	ctx context.Context,
	ledgerOwnerPartyID string,
	currency string,
	transactionType string,
	amountKind string,
) ([]CashFact, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("begin current cash summary drilldown: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := validateCurrentProjection(ctx, tx); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, currentCashFactsQuery, ledgerOwnerPartyID, currency, transactionType, amountKind)
	if err != nil {
		return nil, fmt.Errorf("query current cash summary facts: %w", err)
	}
	facts := make([]CashFact, 0)
	for rows.Next() {
		var fact CashFact
		if err := rows.Scan(
			&fact.ID,
			&fact.Description,
			&fact.Type,
			&fact.LedgerOwnerPartyID,
			&fact.LedgerOwnerLegalName,
			&fact.CounterpartyLegalName,
			&fact.Currency,
			&fact.OccurredOn,
			&fact.AccountReference,
			&fact.OriginalAmount,
			&fact.EffectiveAmount,
			&fact.NetAllocatedAmount,
			&fact.UnallocatedAmount,
			&fact.ContributionAmount,
		); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan current cash summary fact: %w", err)
		}
		facts = append(facts, fact)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate current cash summary facts: %w", err)
	}
	rows.Close()
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit current cash summary drilldown: %w", err)
	}
	return facts, nil
}

func validateCurrentProjection(ctx context.Context, tx pgx.Tx) error {
	var invalidObligation, invalidCash bool
	if err := tx.QueryRow(ctx, currentProjectionGuardQuery).Scan(&invalidObligation, &invalidCash); err != nil {
		return fmt.Errorf("validate current operating projection: %w", err)
	}
	if invalidObligation || invalidCash {
		return &ProjectionInvariantError{Reason: "stored balance invariant failed"}
	}
	return nil
}

const currentProjectionGuardQuery = `
	WITH obligation_current AS (
		SELECT
			o.status,
			o.current_revision,
			r.revision,
			r.effective_amount,
			COALESCE(allocation_totals.net_allocated, 0) AS net_allocated,
			COALESCE(remission_totals.remitted, 0) AS remitted
		FROM obligations o
		LEFT JOIN obligation_revisions r
			ON r.obligation_id = o.id AND r.revision = o.current_revision
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0) AS net_allocated
			FROM allocations a
			LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
			WHERE a.obligation_id = o.id
		) allocation_totals ON true
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(rem.amount), 0) AS remitted
			FROM obligation_remissions rem
			WHERE rem.obligation_id = o.id
		) remission_totals ON true
	), cash_current AS (
		SELECT
			ct.amount,
			CASE WHEN ctr.id IS NULL THEN ct.amount ELSE 0::numeric END AS effective_amount,
			COALESCE(allocation_totals.net_allocated, 0) AS net_allocated
		FROM cash_transactions ct
		LEFT JOIN cash_transaction_reversals ctr ON ctr.cash_transaction_id = ct.id
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0) AS net_allocated
			FROM allocations a
			LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
			WHERE a.cash_transaction_id = ct.id
		) allocation_totals ON true
	)
	SELECT
		EXISTS (
			SELECT 1
			FROM obligation_current
			WHERE (current_revision IS NULL AND status <> 'draft')
			   OR (current_revision IS NOT NULL AND (
					revision IS NULL
					OR status NOT IN ('open', 'closed')
					OR net_allocated < 0
					OR remitted < 0
					OR effective_amount - net_allocated - remitted < 0
					OR (status = 'closed' AND effective_amount - net_allocated - remitted <> 0)
			   ))
		),
		EXISTS (
			SELECT 1
			FROM cash_current
			WHERE amount <= 0
			   OR effective_amount < 0
			   OR net_allocated < 0
			   OR effective_amount - net_allocated < 0
		)
`

const currentSummaryQuery = `
	WITH obligation_current AS (
		SELECT
			r.ledger_owner_party_id,
			r.currency,
			r.direction,
			(r.effective_amount
				- COALESCE(allocation_totals.net_allocated, 0)
				- COALESCE(remission_totals.remitted, 0)) AS outstanding
		FROM obligations o
		JOIN obligation_revisions r
			ON r.obligation_id = o.id AND r.revision = o.current_revision
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0) AS net_allocated
			FROM allocations a
			LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
			WHERE a.obligation_id = o.id
		) allocation_totals ON true
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(rem.amount), 0) AS remitted
			FROM obligation_remissions rem
			WHERE rem.obligation_id = o.id
		) remission_totals ON true
	), obligation_metrics AS (
		SELECT
			ledger_owner_party_id,
			currency,
			COALESCE(SUM(outstanding) FILTER (WHERE direction = 'receivable' AND outstanding > 0), 0) AS receivable_outstanding,
			COUNT(*) FILTER (WHERE direction = 'receivable' AND outstanding > 0) AS receivable_count,
			COALESCE(SUM(outstanding) FILTER (WHERE direction = 'payable' AND outstanding > 0), 0) AS payable_outstanding,
			COUNT(*) FILTER (WHERE direction = 'payable' AND outstanding > 0) AS payable_count
		FROM obligation_current
		GROUP BY ledger_owner_party_id, currency
	), cash_current AS (
		SELECT
			ct.ledger_owner_party_id,
			ct.currency,
			ct.type,
			CASE WHEN ctr.id IS NULL THEN ct.amount ELSE 0::numeric END AS effective_amount,
			CASE WHEN ctr.id IS NULL THEN ct.amount ELSE 0::numeric END
				- COALESCE(allocation_totals.net_allocated, 0) AS unallocated_amount
		FROM cash_transactions ct
		LEFT JOIN cash_transaction_reversals ctr ON ctr.cash_transaction_id = ct.id
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0) AS net_allocated
			FROM allocations a
			LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
			WHERE a.cash_transaction_id = ct.id
		) allocation_totals ON true
	), cash_metrics AS (
		SELECT
			ledger_owner_party_id,
			currency,
			COALESCE(SUM(effective_amount) FILTER (WHERE type = 'receipt' AND effective_amount > 0), 0) AS receipt_effective,
			COUNT(*) FILTER (WHERE type = 'receipt' AND effective_amount > 0) AS receipt_effective_count,
			COALESCE(SUM(effective_amount) FILTER (WHERE type = 'payment' AND effective_amount > 0), 0) AS payment_effective,
			COUNT(*) FILTER (WHERE type = 'payment' AND effective_amount > 0) AS payment_effective_count,
			COALESCE(SUM(unallocated_amount) FILTER (WHERE type = 'receipt' AND unallocated_amount > 0), 0) AS receipt_unallocated,
			COUNT(*) FILTER (WHERE type = 'receipt' AND unallocated_amount > 0) AS receipt_unallocated_count,
			COALESCE(SUM(unallocated_amount) FILTER (WHERE type = 'payment' AND unallocated_amount > 0), 0) AS payment_unallocated,
			COUNT(*) FILTER (WHERE type = 'payment' AND unallocated_amount > 0) AS payment_unallocated_count
		FROM cash_current
		GROUP BY ledger_owner_party_id, currency
	), grouping_keys AS (
		SELECT ledger_owner_party_id, currency
		FROM obligation_current
		WHERE outstanding > 0
		UNION
		SELECT ledger_owner_party_id, currency
		FROM cash_current
		WHERE effective_amount > 0 OR unallocated_amount > 0
	)
	SELECT
		keys.ledger_owner_party_id::text,
		p.legal_name,
		keys.currency,
		COALESCE(obligation_metrics.receivable_outstanding, 0)::text,
		COALESCE(obligation_metrics.receivable_count, 0)::bigint,
		COALESCE(obligation_metrics.payable_outstanding, 0)::text,
		COALESCE(obligation_metrics.payable_count, 0)::bigint,
		COALESCE(cash_metrics.receipt_effective, 0)::text,
		COALESCE(cash_metrics.receipt_effective_count, 0)::bigint,
		COALESCE(cash_metrics.payment_effective, 0)::text,
		COALESCE(cash_metrics.payment_effective_count, 0)::bigint,
		COALESCE(cash_metrics.receipt_unallocated, 0)::text,
		COALESCE(cash_metrics.receipt_unallocated_count, 0)::bigint,
		COALESCE(cash_metrics.payment_unallocated, 0)::text,
		COALESCE(cash_metrics.payment_unallocated_count, 0)::bigint
	FROM grouping_keys keys
	JOIN parties p ON p.id = keys.ledger_owner_party_id
	LEFT JOIN obligation_metrics
		ON obligation_metrics.ledger_owner_party_id = keys.ledger_owner_party_id
		AND obligation_metrics.currency = keys.currency
	LEFT JOIN cash_metrics
		ON cash_metrics.ledger_owner_party_id = keys.ledger_owner_party_id
		AND cash_metrics.currency = keys.currency
	ORDER BY lower(p.legal_name), p.id, keys.currency
`

const currentObligationFactsQuery = `
	WITH obligation_current AS (
		SELECT
			o.id,
			r.title,
			r.direction,
			r.ledger_owner_party_id,
			p.legal_name AS ledger_owner_legal_name,
			CASE WHEN r.direction = 'receivable' THEN r.debtor_legal_name ELSE r.creditor_legal_name END AS counterparty_legal_name,
			r.currency,
			r.due_on,
			r.revision,
			r.effective_amount,
			COALESCE(allocation_totals.net_allocated, 0) AS net_allocated,
			COALESCE(remission_totals.remitted, 0) AS remitted
		FROM obligations o
		JOIN obligation_revisions r
			ON r.obligation_id = o.id AND r.revision = o.current_revision
		JOIN parties p ON p.id = r.ledger_owner_party_id
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0) AS net_allocated
			FROM allocations a
			LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
			WHERE a.obligation_id = o.id
		) allocation_totals ON true
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(rem.amount), 0) AS remitted
			FROM obligation_remissions rem
			WHERE rem.obligation_id = o.id
		) remission_totals ON true
	)
	SELECT
		id::text,
		title,
		direction,
		ledger_owner_party_id::text,
		ledger_owner_legal_name,
		counterparty_legal_name,
		currency,
		due_on::text,
		revision,
		effective_amount::text,
		net_allocated::text,
		remitted::text,
		(effective_amount - net_allocated - remitted)::text,
		(effective_amount - net_allocated - remitted)::text
	FROM obligation_current
	WHERE ledger_owner_party_id = $1
	  AND currency = $2
	  AND direction = $3
	  AND effective_amount - net_allocated - remitted > 0
	ORDER BY due_on, id
`

const currentCashFactsQuery = `
	WITH cash_current AS (
		SELECT
			ct.id,
			ct.description,
			ct.type,
			ct.ledger_owner_party_id,
			p.legal_name AS ledger_owner_legal_name,
			CASE WHEN ct.type = 'receipt' THEN ct.payer_legal_name ELSE ct.payee_legal_name END AS counterparty_legal_name,
			ct.currency,
			ct.occurred_on,
			ct.account_reference,
			ct.amount,
			CASE WHEN ctr.id IS NULL THEN ct.amount ELSE 0::numeric END AS effective_amount,
			COALESCE(allocation_totals.net_allocated, 0) AS net_allocated
		FROM cash_transactions ct
		JOIN parties p ON p.id = ct.ledger_owner_party_id
		LEFT JOIN cash_transaction_reversals ctr ON ctr.cash_transaction_id = ct.id
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0) AS net_allocated
			FROM allocations a
			LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
			WHERE a.cash_transaction_id = ct.id
		) allocation_totals ON true
	)
	SELECT
		id::text,
		description,
		type,
		ledger_owner_party_id::text,
		ledger_owner_legal_name,
		counterparty_legal_name,
		currency,
		occurred_on::text,
		account_reference,
		amount::text,
		effective_amount::text,
		net_allocated::text,
		(effective_amount - net_allocated)::text,
		CASE WHEN $4 = 'effective' THEN effective_amount ELSE effective_amount - net_allocated END::text
	FROM cash_current
	WHERE ledger_owner_party_id = $1
	  AND currency = $2
	  AND type = $3
	  AND (($4 = 'effective' AND effective_amount > 0)
		OR ($4 = 'unallocated' AND effective_amount - net_allocated > 0))
	ORDER BY occurred_on, id
`
