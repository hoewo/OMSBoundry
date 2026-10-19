package ledgerexport

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

type PostgresRepository struct {
	pool             *pgxpool.Pool
	afterTableLoaded func(string)
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Load(ctx context.Context, ownerID string) (Snapshot, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return Snapshot{}, false, fmt.Errorf("begin owner ledger export snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	snapshot := Snapshot{Tables: make([]Table, 0, len(tableSpecs))}
	found, err := loadOwner(ctx, tx, ownerID, &snapshot.Owner)
	if err != nil || !found {
		return Snapshot{}, found, err
	}
	if err := validateOwnerAllocationBoundary(ctx, tx, ownerID); err != nil {
		return Snapshot{}, false, err
	}
	if err := tx.QueryRow(ctx, `
		SELECT version
		FROM schema_migrations
		ORDER BY version DESC
		LIMIT 1
	`).Scan(&snapshot.SchemaVersion); err != nil {
		return Snapshot{}, false, fmt.Errorf("query owner ledger export schema version: %w", err)
	}
	for _, spec := range tableSpecs {
		query, ok := tableQueries[spec.Filename]
		if !ok {
			return Snapshot{}, false, fmt.Errorf("missing owner ledger export query for %s", spec.Filename)
		}
		table, err := loadTable(ctx, tx, ownerID, spec.Filename, query)
		if err != nil {
			return Snapshot{}, false, err
		}
		snapshot.Tables = append(snapshot.Tables, table)
		if r.afterTableLoaded != nil {
			r.afterTableLoaded(spec.Filename)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Snapshot{}, false, fmt.Errorf("commit owner ledger export snapshot: %w", err)
	}
	return snapshot, true, nil
}

func (r *PostgresRepository) RecordExport(ctx context.Context, entry audit.Entry) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin owner ledger export audit: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := audit.Insert(ctx, tx, entry); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit owner ledger export audit: %w", err)
	}
	return nil
}

func loadOwner(ctx context.Context, tx pgx.Tx, ownerID string, owner *Owner) (bool, error) {
	err := tx.QueryRow(ctx, `
		SELECT id::text, kind, legal_name, COALESCE(registration_identifier, ''), revision
		FROM parties
		WHERE id = $1
	`, ownerID).Scan(
		&owner.ID,
		&owner.Kind,
		&owner.LegalName,
		&owner.RegistrationIdentifier,
		&owner.Revision,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query owner ledger export party: %w", err)
	}
	return true, nil
}

func validateOwnerAllocationBoundary(ctx context.Context, tx pgx.Tx, ownerID string) error {
	var broken bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM allocations allocation
			JOIN cash_transactions cash ON cash.id = allocation.cash_transaction_id
			JOIN obligations obligation ON obligation.id = allocation.obligation_id
			WHERE (cash.ledger_owner_party_id = $1 OR obligation.ledger_owner_party_id = $1)
			  AND cash.ledger_owner_party_id <> obligation.ledger_owner_party_id
		)
	`, ownerID).Scan(&broken); err != nil {
		return fmt.Errorf("validate owner ledger allocation boundary: %w", err)
	}
	if broken {
		return &SnapshotInvariantError{Message: "资金分配跨越本方账簿主体，无法形成闭合导出"}
	}
	return nil
}

func loadTable(ctx context.Context, tx pgx.Tx, ownerID, filename, query string) (Table, error) {
	rows, err := tx.Query(ctx, query, ownerID)
	if err != nil {
		return Table{}, fmt.Errorf("query owner ledger export %s: %w", filename, err)
	}
	defer rows.Close()

	result := Table{Filename: filename, Rows: make([][]string, 0)}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return Table{}, fmt.Errorf("read owner ledger export %s row: %w", filename, err)
		}
		row := make([]string, len(values))
		for index, value := range values {
			switch typed := value.(type) {
			case nil:
				row[index] = ""
			case string:
				row[index] = typed
			case []byte:
				row[index] = string(typed)
			default:
				return Table{}, fmt.Errorf("read owner ledger export %s: column %d is not text", filename, index+1)
			}
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return Table{}, fmt.Errorf("iterate owner ledger export %s: %w", filename, err)
	}
	return result, nil
}

var tableQueries = map[string]string{
	"parties.csv": `
		WITH referenced_party_ids AS (
			SELECT $1::uuid AS id
			UNION SELECT creditor_party_id FROM obligations WHERE ledger_owner_party_id = $1
			UNION SELECT debtor_party_id FROM obligations WHERE ledger_owner_party_id = $1
			UNION SELECT payer_party_id FROM cash_transactions WHERE ledger_owner_party_id = $1
			UNION SELECT payee_party_id FROM cash_transactions WHERE ledger_owner_party_id = $1
		)
		SELECT party.id::text, party.kind, party.legal_name,
			COALESCE(party.registration_identifier, ''), party.revision::text,
			to_char(party.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
			to_char(party.updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
		FROM parties party
		JOIN referenced_party_ids referenced ON referenced.id = party.id
		ORDER BY CASE WHEN party.id = $1 THEN 0 ELSE 1 END,
			lower(party.legal_name) COLLATE "C", party.legal_name COLLATE "C", party.id
	`,
	"obligations.csv": `
		SELECT obligation.id::text, obligation.direction, obligation.title, obligation.ledger_owner_party_id::text,
			obligation.creditor_party_id::text, obligation.creditor_party_revision::text, obligation.creditor_kind,
			obligation.creditor_legal_name, COALESCE(obligation.creditor_registration_identifier, ''),
			obligation.debtor_party_id::text, obligation.debtor_party_revision::text, obligation.debtor_kind,
			obligation.debtor_legal_name, COALESCE(obligation.debtor_registration_identifier, ''),
			obligation.original_amount::text, obligation.currency, obligation.due_on::text,
			obligation.source_kind, obligation.manual_source_reference, obligation.status,
			obligation.draft_version::text, COALESCE(obligation.current_revision::text, ''),
			to_char(obligation.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
			to_char(obligation.updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
		FROM obligations obligation
		WHERE obligation.ledger_owner_party_id = $1
		ORDER BY obligation.created_at, obligation.id
	`,
	"obligation_revisions.csv": `
		SELECT revision.obligation_id::text, revision.revision::text, revision.direction, revision.title,
			revision.ledger_owner_party_id::text,
			revision.creditor_party_id::text, revision.creditor_party_revision::text, revision.creditor_kind,
			revision.creditor_legal_name, COALESCE(revision.creditor_registration_identifier, ''),
			revision.debtor_party_id::text, revision.debtor_party_revision::text, revision.debtor_kind,
			revision.debtor_legal_name, COALESCE(revision.debtor_registration_identifier, ''),
			revision.effective_amount::text, revision.currency, revision.due_on::text,
			revision.source_kind, revision.manual_source_reference,
			COALESCE(revision.source_draft_version::text, ''), revision.revision_kind,
			COALESCE(revision.source_revision::text, ''),
			revision.recorded_by_actor_kind, revision.recorded_by_actor_id, revision.recorded_by_actor_display_name,
			to_char(revision.recorded_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
		FROM obligation_revisions revision
		WHERE revision.ledger_owner_party_id = $1
		ORDER BY revision.obligation_id, revision.revision
	`,
	"obligation_amount_adjustments.csv": `
		SELECT adjustment.id::text, adjustment.obligation_id::text,
			adjustment.source_revision::text, adjustment.resulting_revision::text,
			adjustment.adjustment_type, adjustment.adjustment_amount::text,
			adjustment.previous_effective_amount::text, adjustment.resulting_effective_amount::text,
			adjustment.currency, adjustment.source_kind, adjustment.adjustment_reason,
			adjustment.adjusted_by_actor_kind, adjustment.adjusted_by_actor_id, adjustment.adjusted_by_actor_display_name,
			to_char(adjustment.adjusted_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
		FROM obligation_amount_adjustments adjustment
		JOIN obligations obligation ON obligation.id = adjustment.obligation_id
		WHERE obligation.ledger_owner_party_id = $1
		ORDER BY adjustment.adjusted_at, adjustment.id
	`,
	"obligation_remissions.csv": `
		SELECT remission.id::text, remission.obligation_id::text, remission.obligation_revision::text,
			remission.amount::text, remission.recorded_effective_amount::text,
			remission.currency, remission.source_kind, remission.remission_reason, remission.manual_source_reference,
			remission.recorded_by_actor_kind, remission.recorded_by_actor_id, remission.recorded_by_actor_display_name,
			to_char(remission.recorded_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
		FROM obligation_remissions remission
		JOIN obligations obligation ON obligation.id = remission.obligation_id
		WHERE obligation.ledger_owner_party_id = $1
		ORDER BY remission.recorded_at, remission.id
	`,
	"obligation_closures.csv": `
		SELECT closure.id::text, closure.obligation_id::text, closure.obligation_revision::text,
			closure.balance_sequence::text, closure.recorded_effective_amount::text,
			closure.net_allocated_amount::text, closure.remitted_amount::text,
			closure.currency, closure.settlement_basis, closure.source_kind, closure.closure_reason,
			closure.closed_by_actor_kind, closure.closed_by_actor_id, closure.closed_by_actor_display_name,
			to_char(closure.closed_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
		FROM obligation_closures closure
		JOIN obligations obligation ON obligation.id = closure.obligation_id
		WHERE obligation.ledger_owner_party_id = $1
		ORDER BY closure.closed_at, closure.id
	`,
	"obligation_cash_expectations.csv": `
		SELECT expectation.id::text, expectation.obligation_id::text,
			expectation.sequence::text, expectation.action, COALESCE(expectation.expected_on::text, ''),
			expectation.obligation_revision::text, expectation.source_kind, expectation.expectation_reason,
			expectation.recorded_by_actor_kind, expectation.recorded_by_actor_id,
			expectation.recorded_by_actor_display_name,
			to_char(expectation.recorded_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
		FROM obligation_cash_expectations expectation
		JOIN obligations obligation ON obligation.id = expectation.obligation_id
		WHERE obligation.ledger_owner_party_id = $1
		ORDER BY expectation.obligation_id, expectation.sequence
	`,
	"cash_transactions.csv": `
		SELECT cash.id::text, cash.type, cash.description, cash.ledger_owner_party_id::text,
			cash.payer_party_id::text, cash.payer_party_revision::text, cash.payer_kind,
			cash.payer_legal_name, COALESCE(cash.payer_registration_identifier, ''),
			cash.payee_party_id::text, cash.payee_party_revision::text, cash.payee_kind,
			cash.payee_legal_name, COALESCE(cash.payee_registration_identifier, ''),
			cash.amount::text, cash.currency, cash.occurred_on::text, cash.account_reference,
			cash.source_kind, cash.manual_source_reference, COALESCE(cash.external_transaction_reference, ''),
			cash.recorded_by_actor_kind, cash.recorded_by_actor_id, cash.recorded_by_actor_display_name,
			to_char(cash.recorded_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
		FROM cash_transactions cash
		WHERE cash.ledger_owner_party_id = $1
		ORDER BY cash.recorded_at, cash.id
	`,
	"cash_transaction_reversals.csv": `
		SELECT reversal.id::text, reversal.cash_transaction_id::text, reversal.transaction_type,
			reversal.amount::text, reversal.currency, reversal.source_kind, reversal.reversal_reason,
			reversal.reversed_by_actor_kind, reversal.reversed_by_actor_id, reversal.reversed_by_actor_display_name,
			to_char(reversal.reversed_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
		FROM cash_transaction_reversals reversal
		JOIN cash_transactions cash ON cash.id = reversal.cash_transaction_id
		WHERE cash.ledger_owner_party_id = $1
		ORDER BY reversal.reversed_at, reversal.id
	`,
	"allocations.csv": `
		SELECT allocation.id::text, allocation.cash_transaction_id::text,
			allocation.obligation_id::text, allocation.obligation_revision::text,
			allocation.amount::text, allocation.currency, allocation.source_kind, allocation.manual_source_reference,
			allocation.applied_by_actor_kind, allocation.applied_by_actor_id, allocation.applied_by_actor_display_name,
			to_char(allocation.applied_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
		FROM allocations allocation
		JOIN cash_transactions cash ON cash.id = allocation.cash_transaction_id
		JOIN obligations obligation ON obligation.id = allocation.obligation_id
		WHERE cash.ledger_owner_party_id = $1
		  AND obligation.ledger_owner_party_id = $1
		ORDER BY allocation.applied_at, allocation.id
	`,
	"allocation_reversals.csv": `
		SELECT reversal.id::text, reversal.allocation_id::text, reversal.cash_transaction_id::text,
			reversal.obligation_id::text, reversal.obligation_revision::text,
			reversal.amount::text, reversal.currency, reversal.source_kind, reversal.reversal_reason,
			reversal.reversed_by_actor_kind, reversal.reversed_by_actor_id, reversal.reversed_by_actor_display_name,
			to_char(reversal.reversed_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
		FROM allocation_reversals reversal
		JOIN cash_transactions cash ON cash.id = reversal.cash_transaction_id
		JOIN obligations obligation ON obligation.id = reversal.obligation_id
		WHERE cash.ledger_owner_party_id = $1
		  AND obligation.ledger_owner_party_id = $1
		ORDER BY reversal.reversed_at, reversal.id
	`,
	"internal_transfers.csv": `
		SELECT transfer.id::text, transfer.description, transfer.ledger_owner_party_id::text,
			transfer.ledger_owner_party_revision::text, transfer.ledger_owner_kind,
			transfer.ledger_owner_legal_name, COALESCE(transfer.ledger_owner_registration_identifier, ''),
			transfer.amount::text, transfer.currency, transfer.occurred_on::text,
			transfer.source_account_reference, transfer.destination_account_reference,
			transfer.source_kind, transfer.manual_source_reference, COALESCE(transfer.external_transfer_reference, ''),
			transfer.recorded_by_actor_kind, transfer.recorded_by_actor_id, transfer.recorded_by_actor_display_name,
			to_char(transfer.recorded_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
		FROM internal_transfers transfer
		WHERE transfer.ledger_owner_party_id = $1
		ORDER BY transfer.recorded_at, transfer.id
	`,
	"audit_entries.csv": `
		WITH referenced_party_ids AS (
			SELECT $1::uuid AS id
			UNION SELECT creditor_party_id FROM obligations WHERE ledger_owner_party_id = $1
			UNION SELECT debtor_party_id FROM obligations WHERE ledger_owner_party_id = $1
			UNION SELECT payer_party_id FROM cash_transactions WHERE ledger_owner_party_id = $1
			UNION SELECT payee_party_id FROM cash_transactions WHERE ledger_owner_party_id = $1
		), owner_obligations AS (
			SELECT id FROM obligations WHERE ledger_owner_party_id = $1
		), owner_cash AS (
			SELECT id FROM cash_transactions WHERE ledger_owner_party_id = $1
		), owner_cash_reversals AS (
			SELECT reversal.id
			FROM cash_transaction_reversals reversal
			JOIN owner_cash cash ON cash.id = reversal.cash_transaction_id
		), owner_allocations AS (
			SELECT allocation.id
			FROM allocations allocation
			JOIN owner_cash cash ON cash.id = allocation.cash_transaction_id
			JOIN owner_obligations obligation ON obligation.id = allocation.obligation_id
		), owner_allocation_reversals AS (
			SELECT reversal.id
			FROM allocation_reversals reversal
			JOIN owner_allocations allocation ON allocation.id = reversal.allocation_id
		), owner_internal_transfers AS (
			SELECT id FROM internal_transfers WHERE ledger_owner_party_id = $1
		)
		SELECT entry.id::text, entry.object_type, entry.object_id::text, entry.action,
			COALESCE(entry.command_id::text, ''), entry.actor_kind, entry.actor_id, entry.actor_display_name,
			to_char(entry.occurred_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
			entry.summary, entry.details::text
		FROM audit_entries entry
		WHERE (entry.object_type = 'party' AND entry.object_id IN (SELECT id FROM referenced_party_ids))
		   OR (entry.object_type = 'obligation' AND entry.object_id IN (SELECT id FROM owner_obligations))
		   OR (entry.object_type = 'cash_transaction' AND entry.object_id IN (SELECT id FROM owner_cash))
		   OR (entry.object_type = 'cash_transaction_reversal' AND entry.object_id IN (SELECT id FROM owner_cash_reversals))
		   OR (entry.object_type = 'allocation' AND entry.object_id IN (SELECT id FROM owner_allocations))
		   OR (entry.object_type = 'allocation_reversal' AND entry.object_id IN (SELECT id FROM owner_allocation_reversals))
		   OR (entry.object_type = 'internal_transfer' AND entry.object_id IN (SELECT id FROM owner_internal_transfers))
		   OR (entry.object_type = 'ledger_export' AND entry.object_id = $1)
		ORDER BY entry.occurred_at, entry.id
	`,
}
