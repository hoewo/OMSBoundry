package allocation

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

type cashFacts struct {
	Type               string
	LedgerOwnerPartyID string
	PayerPartyID       string
	PayeePartyID       string
	Amount             string
	Currency           string
}

type obligationFacts struct {
	Status             string
	CurrentRevision    int64
	Direction          string
	LedgerOwnerPartyID string
	CreditorPartyID    string
	DebtorPartyID      string
	Amount             string
	Currency           string
}

func (r *PostgresRepository) Apply(
	ctx context.Context,
	value Allocation,
	command ApplyCommand,
	entry audit.Entry,
) (Allocation, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Allocation{}, false, fmt.Errorf("begin apply allocation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, command.CommandID); err != nil {
		return Allocation{}, false, fmt.Errorf("lock allocation command: %w", err)
	}
	var existingAllocationID string
	var existingPayloadHash []byte
	err = tx.QueryRow(ctx, `
		SELECT allocation_id::text, payload_hash
		FROM allocation_apply_commands
		WHERE command_id = $1
	`, command.CommandID).Scan(&existingAllocationID, &existingPayloadHash)
	switch {
	case err == nil:
		if !bytes.Equal(existingPayloadHash, command.PayloadHash) {
			return Allocation{}, false, &IdempotencyConflictError{CommandID: command.CommandID}
		}
		existing, found, err := readAllocation(ctx, tx, existingAllocationID)
		if err != nil {
			return Allocation{}, false, err
		}
		if !found {
			return Allocation{}, false, fmt.Errorf("allocation command references missing allocation %s", existingAllocationID)
		}
		if err := tx.Commit(ctx); err != nil {
			return Allocation{}, false, fmt.Errorf("commit allocation replay: %w", err)
		}
		return existing, true, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return Allocation{}, false, fmt.Errorf("read allocation command: %w", err)
	}

	var cash cashFacts
	err = tx.QueryRow(ctx, `
		SELECT
			type,
			ledger_owner_party_id::text,
			payer_party_id::text,
			payee_party_id::text,
			amount::text,
			currency
		FROM cash_transactions
		WHERE id = $1
		FOR UPDATE
	`, value.CashTransactionID).Scan(
		&cash.Type,
		&cash.LedgerOwnerPartyID,
		&cash.PayerPartyID,
		&cash.PayeePartyID,
		&cash.Amount,
		&cash.Currency,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Allocation{}, false, &CashTransactionNotFoundError{CashTransactionID: value.CashTransactionID}
	}
	if err != nil {
		return Allocation{}, false, fmt.Errorf("lock cash transaction for allocation: %w", err)
	}
	var cashReversed bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM cash_transaction_reversals
			WHERE cash_transaction_id = $1
		)
	`, value.CashTransactionID).Scan(&cashReversed); err != nil {
		return Allocation{}, false, fmt.Errorf("read cash transaction reversal state for allocation: %w", err)
	}
	if cashReversed {
		return Allocation{}, false, &CashTransactionStateConflictError{Status: "reversed"}
	}

	var obligation obligationFacts
	err = tx.QueryRow(ctx, `
		SELECT status, COALESCE(current_revision, 0)
		FROM obligations
		WHERE id = $1
		FOR UPDATE
	`, value.ObligationID).Scan(&obligation.Status, &obligation.CurrentRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return Allocation{}, false, &ObligationNotFoundError{ObligationID: value.ObligationID}
	}
	if err != nil {
		return Allocation{}, false, fmt.Errorf("lock obligation for allocation: %w", err)
	}
	if obligation.Status != "open" {
		return Allocation{}, false, &ObligationStateConflictError{Status: obligation.Status}
	}
	if obligation.CurrentRevision != value.ObligationRevision {
		return Allocation{}, false, &ObligationRevisionConflictError{
			Expected: value.ObligationRevision,
			Actual:   obligation.CurrentRevision,
		}
	}
	err = tx.QueryRow(ctx, `
		SELECT
			direction,
			ledger_owner_party_id::text,
			creditor_party_id::text,
			debtor_party_id::text,
			effective_amount::text,
			currency
		FROM obligation_revisions
		WHERE obligation_id = $1 AND revision = $2
	`, value.ObligationID, value.ObligationRevision).Scan(
		&obligation.Direction,
		&obligation.LedgerOwnerPartyID,
		&obligation.CreditorPartyID,
		&obligation.DebtorPartyID,
		&obligation.Amount,
		&obligation.Currency,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Allocation{}, false, fmt.Errorf("current obligation revision is missing: %s/%d", value.ObligationID, value.ObligationRevision)
	}
	if err != nil {
		return Allocation{}, false, fmt.Errorf("read obligation revision for allocation: %w", err)
	}

	if reason := compatibilityConflictReason(cash, obligation); reason != "" {
		return Allocation{}, false, &CompatibilityConflictError{Reason: reason}
	}

	cashAvailable, err := readAvailableAmount(ctx, tx, `
		SELECT ($2::numeric - COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0))::text
		FROM allocations a
		LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
		WHERE a.cash_transaction_id = $1
	`, value.CashTransactionID, cash.Amount)
	if err != nil {
		return Allocation{}, false, fmt.Errorf("read cash unallocated amount: %w", err)
	}
	comparison, err := compareAmounts(value.Amount, cashAvailable)
	if err != nil {
		return Allocation{}, false, fmt.Errorf("compare cash unallocated amount: %w", err)
	}
	if comparison > 0 {
		return Allocation{}, false, &CashBalanceConflictError{Requested: value.Amount, Available: cashAvailable}
	}

	obligationAvailable, err := readAvailableAmount(ctx, tx, `
		SELECT (
			$2::numeric
			- COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0)
			- (SELECT COALESCE(SUM(r.amount), 0) FROM obligation_remissions r WHERE r.obligation_id = $1)
		)::text
		FROM allocations a
		LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
		WHERE a.obligation_id = $1
	`, value.ObligationID, obligation.Amount)
	if err != nil {
		return Allocation{}, false, fmt.Errorf("read obligation outstanding amount: %w", err)
	}
	comparison, err = compareAmounts(value.Amount, obligationAvailable)
	if err != nil {
		return Allocation{}, false, fmt.Errorf("compare obligation outstanding amount: %w", err)
	}
	if comparison > 0 {
		return Allocation{}, false, &ObligationBalanceConflictError{Requested: value.Amount, Available: obligationAvailable}
	}

	value.Currency = cash.Currency
	if _, err := tx.Exec(ctx, `
		INSERT INTO allocations (
			id, cash_transaction_id, obligation_id, obligation_revision,
			amount, currency, source_kind, manual_source_reference,
			applied_by_actor_kind, applied_by_actor_id,
			applied_by_actor_display_name, applied_at
		)
		VALUES (
			$1, $2, $3, $4,
			$5::numeric, $6, $7, $8,
			$9, $10, $11, $12
		)
	`,
		value.ID,
		value.CashTransactionID,
		value.ObligationID,
		value.ObligationRevision,
		value.Amount,
		value.Currency,
		value.SourceKind,
		value.ManualSourceReference,
		value.AppliedBy.Kind,
		value.AppliedBy.ID,
		value.AppliedBy.DisplayName,
		value.AppliedAt,
	); err != nil {
		return Allocation{}, false, fmt.Errorf("insert allocation: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO allocation_apply_commands (
			command_id, allocation_id, cash_transaction_id,
			obligation_id, expected_obligation_revision,
			payload_hash, actor_kind, actor_id, actor_display_name, applied_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`,
		command.CommandID,
		value.ID,
		value.CashTransactionID,
		value.ObligationID,
		value.ObligationRevision,
		command.PayloadHash,
		command.Actor.Kind,
		command.Actor.ID,
		command.Actor.DisplayName,
		command.AppliedAt,
	); err != nil {
		return Allocation{}, false, fmt.Errorf("insert allocation command: %w", err)
	}
	if entry.Details == nil {
		entry.Details = make(map[string]any)
	}
	entry.Details["currency"] = value.Currency
	if err := audit.Insert(ctx, tx, entry); err != nil {
		return Allocation{}, false, err
	}

	persisted, found, err := readAllocation(ctx, tx, value.ID)
	if err != nil {
		return Allocation{}, false, err
	}
	if !found {
		return Allocation{}, false, fmt.Errorf("created allocation disappeared: %s", value.ID)
	}
	if err := tx.Commit(ctx); err != nil {
		return Allocation{}, false, fmt.Errorf("commit allocation: %w", err)
	}
	return persisted, false, nil
}

func compatibilityConflictReason(cash cashFacts, obligation obligationFacts) string {
	if cash.Currency != obligation.Currency {
		return "currency"
	}
	if cash.LedgerOwnerPartyID != obligation.LedgerOwnerPartyID {
		return "ledger_owner"
	}
	if cash.PayerPartyID != obligation.DebtorPartyID || cash.PayeePartyID != obligation.CreditorPartyID {
		return "parties"
	}
	if obligation.Direction == "receivable" && cash.Type != "receipt" {
		return "direction"
	}
	if obligation.Direction == "payable" && cash.Type != "payment" {
		return "direction"
	}
	if obligation.Direction != "receivable" && obligation.Direction != "payable" {
		return "obligation_direction"
	}
	return ""
}

func readAvailableAmount(ctx context.Context, queryer queryRower, query string, id, total string) (string, error) {
	var amount string
	if err := queryer.QueryRow(ctx, query, id, total).Scan(&amount); err != nil {
		return "", err
	}
	normalized, err := normalizeStoredAmount(amount)
	if err != nil {
		return "", err
	}
	return normalized, nil
}

func (r *PostgresRepository) List(ctx context.Context) ([]Allocation, error) {
	return r.list(ctx, "")
}

func (r *PostgresRepository) ListForCashTransaction(ctx context.Context, cashTransactionID string) ([]Allocation, error) {
	return r.list(ctx, ` WHERE a.cash_transaction_id = $1`, cashTransactionID)
}

func (r *PostgresRepository) ListForObligation(ctx context.Context, obligationID string) ([]Allocation, error) {
	return r.list(ctx, ` WHERE a.obligation_id = $1`, obligationID)
}

func (r *PostgresRepository) list(ctx context.Context, predicate string, arguments ...any) ([]Allocation, error) {
	rows, err := r.pool.Query(ctx, allocationSelect+predicate+` ORDER BY a.applied_at DESC, a.id DESC`, arguments...)
	if err != nil {
		return nil, fmt.Errorf("query allocations: %w", err)
	}
	defer rows.Close()
	values := make([]Allocation, 0)
	for rows.Next() {
		value, err := scanAllocation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan allocation: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate allocations: %w", err)
	}
	return values, nil
}

func (r *PostgresRepository) Get(ctx context.Context, allocationID string) (Allocation, bool, error) {
	value, found, err := readAllocation(ctx, r.pool, allocationID)
	if err != nil {
		return Allocation{}, false, fmt.Errorf("query allocation: %w", err)
	}
	return value, found, nil
}

const allocationSelect = `
	SELECT
		a.id::text,
		a.cash_transaction_id::text,
		ct.description,
		ct.type,
		a.obligation_id::text,
		a.obligation_revision,
		r.title,
		r.direction,
		r.ledger_owner_party_id::text,
		CASE
			WHEN r.ledger_owner_party_id = r.creditor_party_id THEN r.creditor_legal_name
			ELSE r.debtor_legal_name
		END,
		CASE
			WHEN r.ledger_owner_party_id = r.creditor_party_id THEN r.debtor_party_id
			ELSE r.creditor_party_id
		END::text,
		CASE
			WHEN r.ledger_owner_party_id = r.creditor_party_id THEN r.debtor_legal_name
			ELSE r.creditor_legal_name
		END,
		a.amount::text,
		a.currency,
		a.source_kind,
		a.manual_source_reference,
		a.applied_by_actor_kind,
		a.applied_by_actor_id,
		a.applied_by_actor_display_name,
		a.applied_at,
		COALESCE(ar.id::text, ''),
		COALESCE(ar.allocation_id::text, ''),
		COALESCE(ar.cash_transaction_id::text, ''),
		COALESCE(ar.obligation_id::text, ''),
		COALESCE(ar.obligation_revision, 0),
		COALESCE(ar.amount::text, ''),
		COALESCE(ar.currency, ''),
		COALESCE(ar.source_kind, ''),
		COALESCE(ar.reversal_reason, ''),
		COALESCE(ar.reversed_by_actor_kind, ''),
		COALESCE(ar.reversed_by_actor_id, ''),
		COALESCE(ar.reversed_by_actor_display_name, ''),
		COALESCE(ar.reversed_at, 'epoch'::timestamptz)
	FROM allocations a
	JOIN cash_transactions ct ON ct.id = a.cash_transaction_id
	JOIN obligation_revisions r
		ON r.obligation_id = a.obligation_id
		AND r.revision = a.obligation_revision
	LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
`

type queryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func readAllocation(ctx context.Context, queryer queryRower, allocationID string) (Allocation, bool, error) {
	value, err := scanAllocation(queryer.QueryRow(ctx, allocationSelect+` WHERE a.id = $1`, allocationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Allocation{}, false, nil
	}
	if err != nil {
		return Allocation{}, false, err
	}
	return value, true, nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanAllocation(row rowScanner) (Allocation, error) {
	var value Allocation
	var reversal Reversal
	err := row.Scan(
		&value.ID,
		&value.CashTransactionID,
		&value.CashDescription,
		&value.CashType,
		&value.ObligationID,
		&value.ObligationRevision,
		&value.ObligationTitle,
		&value.ObligationDirection,
		&value.LedgerOwnerPartyID,
		&value.LedgerOwnerLegalName,
		&value.CounterpartyPartyID,
		&value.CounterpartyLegalName,
		&value.Amount,
		&value.Currency,
		&value.SourceKind,
		&value.ManualSourceReference,
		&value.AppliedBy.Kind,
		&value.AppliedBy.ID,
		&value.AppliedBy.DisplayName,
		&value.AppliedAt,
		&reversal.ID,
		&reversal.AllocationID,
		&reversal.CashTransactionID,
		&reversal.ObligationID,
		&reversal.ObligationRevision,
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
		return Allocation{}, err
	}
	normalizedAmount, err := normalizeStoredAmount(value.Amount)
	if err != nil {
		return Allocation{}, fmt.Errorf("invalid stored allocation amount: %w", err)
	}
	value.Amount = normalizedAmount
	if reversal.ID == "" {
		value.Status = StatusActive
		return value, nil
	}
	normalizedReversalAmount, err := normalizeStoredAmount(reversal.Amount)
	if err != nil {
		return Allocation{}, fmt.Errorf("invalid stored allocation reversal amount: %w", err)
	}
	reversal.Amount = normalizedReversalAmount
	value.Reversal = &reversal
	value.Status = StatusReversed
	return value, nil
}
