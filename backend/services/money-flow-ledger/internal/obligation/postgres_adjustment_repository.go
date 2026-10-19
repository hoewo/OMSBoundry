package obligation

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

func (r *PostgresRepository) AdjustAmount(
	ctx context.Context,
	value Adjustment,
	command AdjustmentCommand,
	entry audit.Entry,
) (Obligation, Adjustment, Revision, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Obligation{}, Adjustment{}, Revision{}, false, fmt.Errorf("begin obligation amount adjustment: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, command.CommandID); err != nil {
		return Obligation{}, Adjustment{}, Revision{}, false, fmt.Errorf("lock obligation amount adjustment command: %w", err)
	}
	var existingAdjustmentID string
	var existingPayloadHash []byte
	err = tx.QueryRow(ctx, `
		SELECT obligation_amount_adjustment_id::text, payload_hash
		FROM obligation_amount_adjustment_commands
		WHERE command_id = $1
	`, command.CommandID).Scan(&existingAdjustmentID, &existingPayloadHash)
	switch {
	case err == nil:
		if !bytes.Equal(existingPayloadHash, command.PayloadHash) {
			return Obligation{}, Adjustment{}, Revision{}, false, &AmountAdjustmentIdempotencyConflictError{CommandID: command.CommandID}
		}
		existing, found, err := readAdjustment(ctx, tx, existingAdjustmentID)
		if err != nil {
			return Obligation{}, Adjustment{}, Revision{}, false, err
		}
		if !found {
			return Obligation{}, Adjustment{}, Revision{}, false, fmt.Errorf("obligation amount adjustment command references missing adjustment %s", existingAdjustmentID)
		}
		obligationValue, found, err := readObligation(ctx, tx, existing.ObligationID)
		if err != nil {
			return Obligation{}, Adjustment{}, Revision{}, false, err
		}
		if !found {
			return Obligation{}, Adjustment{}, Revision{}, false, fmt.Errorf("obligation amount adjustment references missing obligation %s", existing.ObligationID)
		}
		revision, found, err := readRevision(ctx, tx, existing.ObligationID, existing.ResultingRevision)
		if err != nil {
			return Obligation{}, Adjustment{}, Revision{}, false, err
		}
		if !found {
			return Obligation{}, Adjustment{}, Revision{}, false, fmt.Errorf("obligation amount adjustment references missing revision %s/%d", existing.ObligationID, existing.ResultingRevision)
		}
		if err := tx.Commit(ctx); err != nil {
			return Obligation{}, Adjustment{}, Revision{}, false, fmt.Errorf("commit obligation amount adjustment replay: %w", err)
		}
		return obligationValue, existing, revision, true, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return Obligation{}, Adjustment{}, Revision{}, false, fmt.Errorf("read obligation amount adjustment command: %w", err)
	}

	var status string
	var currentRevision int64
	err = tx.QueryRow(ctx, `
		SELECT status, COALESCE(current_revision, 0)
		FROM obligations
		WHERE id = $1
		FOR UPDATE
	`, value.ObligationID).Scan(&status, &currentRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return Obligation{}, Adjustment{}, Revision{}, false, &ObligationNotFoundError{ObligationID: value.ObligationID}
	}
	if err != nil {
		return Obligation{}, Adjustment{}, Revision{}, false, fmt.Errorf("lock obligation for amount adjustment: %w", err)
	}
	if status != StatusOpen {
		return Obligation{}, Adjustment{}, Revision{}, false, &AmountAdjustmentStateConflictError{Status: status}
	}
	if currentRevision != value.SourceRevision {
		return Obligation{}, Adjustment{}, Revision{}, false, &CurrentRevisionConflictError{
			Expected: value.SourceRevision,
			Actual:   currentRevision,
		}
	}

	sourceRevision, found, err := readRevision(ctx, tx, value.ObligationID, value.SourceRevision)
	if err != nil {
		return Obligation{}, Adjustment{}, Revision{}, false, fmt.Errorf("read source revision for amount adjustment: %w", err)
	}
	if !found {
		return Obligation{}, Adjustment{}, Revision{}, false, fmt.Errorf("current obligation revision is missing: %s/%d", value.ObligationID, value.SourceRevision)
	}
	var netAllocated string
	var remitted string
	var committed string
	if err := tx.QueryRow(ctx, `
		SELECT
			allocation_totals.net_allocated::text,
			remission_totals.remitted::text,
			(allocation_totals.net_allocated + remission_totals.remitted)::text
		FROM (
			SELECT COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0) AS net_allocated
			FROM allocations a
			LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
			WHERE a.obligation_id = $1
		) allocation_totals
		CROSS JOIN (
			SELECT COALESCE(SUM(r.amount), 0) AS remitted
			FROM obligation_remissions r
			WHERE r.obligation_id = $1
		) remission_totals
	`, value.ObligationID).Scan(&netAllocated, &remitted, &committed); err != nil {
		return Obligation{}, Adjustment{}, Revision{}, false, fmt.Errorf("read obligation allocated and remitted amounts for adjustment: %w", err)
	}
	netAllocated, err = normalizeAmountForField("net_allocated_amount", "已分配净额", netAllocated, false)
	if err != nil {
		return Obligation{}, Adjustment{}, Revision{}, false, fmt.Errorf("normalize obligation net allocated amount for adjustment: %w", err)
	}
	remitted, err = normalizeAmountForField("remitted_amount", "已减免金额", remitted, false)
	if err != nil {
		return Obligation{}, Adjustment{}, Revision{}, false, fmt.Errorf("normalize obligation remitted amount for adjustment: %w", err)
	}
	committed, err = normalizeAmountForField("committed_amount", "已处置金额", committed, false)
	if err != nil {
		return Obligation{}, Adjustment{}, Revision{}, false, fmt.Errorf("normalize obligation committed amount for adjustment: %w", err)
	}

	resultingAmount, err := calculateAdjustedAmount(sourceRevision.EffectiveAmount, value.Amount, value.Type)
	if err != nil {
		return Obligation{}, Adjustment{}, Revision{}, false, err
	}
	comparison, err := compareDecimalAmounts(resultingAmount, committed)
	if err != nil {
		return Obligation{}, Adjustment{}, Revision{}, false, fmt.Errorf("compare adjusted amount and net allocated amount: %w", err)
	}
	if comparison < 0 {
		reason := "below_allocated"
		if remitted != "0" {
			reason = "below_allocated_and_remitted"
		}
		return Obligation{}, Adjustment{}, Revision{}, false, &AmountAdjustmentResultConflictError{
			Reason:                   reason,
			CurrentEffectiveAmount:   sourceRevision.EffectiveAmount,
			AdjustmentAmount:         value.Amount,
			ResultingEffectiveAmount: resultingAmount,
			NetAllocatedAmount:       netAllocated,
			RemittedAmount:           remitted,
		}
	}

	value.ResultingRevision = value.SourceRevision + 1
	value.PreviousEffectiveAmount = sourceRevision.EffectiveAmount
	value.ResultingEffectiveAmount = resultingAmount
	value.Currency = sourceRevision.Currency
	if _, err := tx.Exec(ctx, `
		INSERT INTO obligation_amount_adjustments (
			id, obligation_id, source_revision, resulting_revision,
			adjustment_type, adjustment_amount,
			previous_effective_amount, resulting_effective_amount,
			currency, source_kind, adjustment_reason,
			adjusted_by_actor_kind, adjusted_by_actor_id,
			adjusted_by_actor_display_name, adjusted_at
		)
		VALUES (
			$1, $2, $3, $4,
			$5, $6::numeric,
			$7::numeric, $8::numeric,
			$9, $10, $11,
			$12, $13, $14, $15
		)
	`,
		value.ID,
		value.ObligationID,
		value.SourceRevision,
		value.ResultingRevision,
		value.Type,
		value.Amount,
		value.PreviousEffectiveAmount,
		value.ResultingEffectiveAmount,
		value.Currency,
		value.SourceKind,
		value.Reason,
		value.AdjustedBy.Kind,
		value.AdjustedBy.ID,
		value.AdjustedBy.DisplayName,
		value.AdjustedAt,
	); err != nil {
		return Obligation{}, Adjustment{}, Revision{}, false, fmt.Errorf("insert obligation amount adjustment: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO obligation_revisions (
			obligation_id, revision, direction, title, ledger_owner_party_id,
			creditor_party_id, creditor_party_revision, creditor_kind,
			creditor_legal_name, creditor_registration_identifier,
			debtor_party_id, debtor_party_revision, debtor_kind,
			debtor_legal_name, debtor_registration_identifier,
			effective_amount, currency, due_on,
			source_kind, manual_source_reference,
			source_application, source_resource_type, source_resource_id, source_fact_revision, source_digest,
			source_draft_version,
			recorded_by_actor_kind, recorded_by_actor_id,
			recorded_by_actor_display_name, recorded_at,
			revision_kind, source_revision
		)
		SELECT
			obligation_id, $3, direction, title, ledger_owner_party_id,
			creditor_party_id, creditor_party_revision, creditor_kind,
			creditor_legal_name, creditor_registration_identifier,
			debtor_party_id, debtor_party_revision, debtor_kind,
			debtor_legal_name, debtor_registration_identifier,
			$4::numeric, currency, due_on,
			source_kind, manual_source_reference,
			source_application, source_resource_type, source_resource_id, source_fact_revision, source_digest,
			NULL,
			$5, $6, $7, $8,
			$9, $2
		FROM obligation_revisions
		WHERE obligation_id = $1 AND revision = $2
	`,
		value.ObligationID,
		value.SourceRevision,
		value.ResultingRevision,
		value.ResultingEffectiveAmount,
		value.AdjustedBy.Kind,
		value.AdjustedBy.ID,
		value.AdjustedBy.DisplayName,
		value.AdjustedAt,
		RevisionKindAmountAdjustment,
	); err != nil {
		return Obligation{}, Adjustment{}, Revision{}, false, fmt.Errorf("insert adjusted obligation revision: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO obligation_amount_adjustment_commands (
			command_id, obligation_amount_adjustment_id, obligation_id,
			expected_current_revision, resulting_revision, payload_hash,
			actor_kind, actor_id, actor_display_name, adjusted_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`,
		command.CommandID,
		value.ID,
		value.ObligationID,
		value.SourceRevision,
		value.ResultingRevision,
		command.PayloadHash,
		command.Actor.Kind,
		command.Actor.ID,
		command.Actor.DisplayName,
		command.AdjustedAt,
	); err != nil {
		return Obligation{}, Adjustment{}, Revision{}, false, fmt.Errorf("insert obligation amount adjustment command: %w", err)
	}

	result, err := tx.Exec(ctx, `
		UPDATE obligations
		SET current_revision = $3, updated_at = $4
		WHERE id = $1 AND status = 'open' AND current_revision = $2
	`, value.ObligationID, value.SourceRevision, value.ResultingRevision, value.AdjustedAt)
	if err != nil {
		return Obligation{}, Adjustment{}, Revision{}, false, fmt.Errorf("advance adjusted obligation revision: %w", err)
	}
	if result.RowsAffected() != 1 {
		return Obligation{}, Adjustment{}, Revision{}, false, fmt.Errorf("obligation changed after amount adjustment row lock")
	}

	if entry.Details == nil {
		entry.Details = make(map[string]any)
	}
	entry.Details["obligation_amount_adjustment_id"] = value.ID
	entry.Details["source_revision"] = value.SourceRevision
	entry.Details["resulting_revision"] = value.ResultingRevision
	entry.Details["previous_effective_amount"] = value.PreviousEffectiveAmount
	entry.Details["resulting_effective_amount"] = value.ResultingEffectiveAmount
	entry.Details["currency"] = value.Currency
	if err := audit.Insert(ctx, tx, entry); err != nil {
		return Obligation{}, Adjustment{}, Revision{}, false, err
	}

	persistedObligation, found, err := readObligation(ctx, tx, value.ObligationID)
	if err != nil {
		return Obligation{}, Adjustment{}, Revision{}, false, err
	}
	if !found {
		return Obligation{}, Adjustment{}, Revision{}, false, fmt.Errorf("adjusted obligation disappeared: %s", value.ObligationID)
	}
	persistedAdjustment, found, err := readAdjustment(ctx, tx, value.ID)
	if err != nil {
		return Obligation{}, Adjustment{}, Revision{}, false, err
	}
	if !found {
		return Obligation{}, Adjustment{}, Revision{}, false, fmt.Errorf("created obligation amount adjustment disappeared: %s", value.ID)
	}
	persistedRevision, found, err := readRevision(ctx, tx, value.ObligationID, value.ResultingRevision)
	if err != nil {
		return Obligation{}, Adjustment{}, Revision{}, false, err
	}
	if !found {
		return Obligation{}, Adjustment{}, Revision{}, false, fmt.Errorf("created adjusted obligation revision disappeared: %s/%d", value.ObligationID, value.ResultingRevision)
	}
	if err := tx.Commit(ctx); err != nil {
		return Obligation{}, Adjustment{}, Revision{}, false, fmt.Errorf("commit obligation amount adjustment: %w", err)
	}
	return persistedObligation, persistedAdjustment, persistedRevision, false, nil
}

const adjustmentSelect = `
	SELECT
		id::text,
		obligation_id::text,
		source_revision,
		resulting_revision,
		adjustment_type,
		adjustment_amount::text,
		previous_effective_amount::text,
		resulting_effective_amount::text,
		currency,
		source_kind,
		adjustment_reason,
		adjusted_by_actor_kind,
		adjusted_by_actor_id,
		adjusted_by_actor_display_name,
		adjusted_at
	FROM obligation_amount_adjustments
`

func readAdjustment(ctx context.Context, queryer queryRower, adjustmentID string) (Adjustment, bool, error) {
	value, err := scanAdjustment(queryer.QueryRow(ctx, adjustmentSelect+` WHERE id = $1`, adjustmentID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Adjustment{}, false, nil
	}
	if err != nil {
		return Adjustment{}, false, err
	}
	return value, true, nil
}

func readAdjustments(ctx context.Context, queryer queryer, obligationID string) ([]Adjustment, error) {
	rows, err := queryer.Query(ctx, adjustmentSelect+` WHERE obligation_id = $1 ORDER BY resulting_revision`, obligationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]Adjustment, 0)
	for rows.Next() {
		value, err := scanAdjustment(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func scanAdjustment(row rowScanner) (Adjustment, error) {
	var value Adjustment
	if err := row.Scan(
		&value.ID,
		&value.ObligationID,
		&value.SourceRevision,
		&value.ResultingRevision,
		&value.Type,
		&value.Amount,
		&value.PreviousEffectiveAmount,
		&value.ResultingEffectiveAmount,
		&value.Currency,
		&value.SourceKind,
		&value.Reason,
		&value.AdjustedBy.Kind,
		&value.AdjustedBy.ID,
		&value.AdjustedBy.DisplayName,
		&value.AdjustedAt,
	); err != nil {
		return Adjustment{}, err
	}
	var err error
	value.Amount, err = normalizeAmountForField("adjustment_amount", "调整金额", value.Amount, true)
	if err != nil {
		return Adjustment{}, fmt.Errorf("invalid stored obligation adjustment amount: %w", err)
	}
	value.PreviousEffectiveAmount, err = normalizeAmountForField("previous_effective_amount", "调整前金额", value.PreviousEffectiveAmount, true)
	if err != nil {
		return Adjustment{}, fmt.Errorf("invalid stored obligation adjustment previous amount: %w", err)
	}
	value.ResultingEffectiveAmount, err = normalizeAmountForField("resulting_effective_amount", "调整后金额", value.ResultingEffectiveAmount, true)
	if err != nil {
		return Adjustment{}, fmt.Errorf("invalid stored obligation adjustment resulting amount: %w", err)
	}
	return value, nil
}
