package obligation

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

func (r *PostgresRepository) RecordCashExpectation(
	ctx context.Context,
	value CashExpectation,
	command CashExpectationCommand,
	entry audit.Entry,
) (CashExpectation, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return CashExpectation{}, false, fmt.Errorf("begin cash expectation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, command.CommandID); err != nil {
		return CashExpectation{}, false, fmt.Errorf("lock cash expectation command: %w", err)
	}
	var existingID string
	var existingPayloadHash []byte
	err = tx.QueryRow(ctx, `
		SELECT obligation_cash_expectation_id::text, payload_hash
		FROM obligation_cash_expectation_commands
		WHERE command_id = $1
	`, command.CommandID).Scan(&existingID, &existingPayloadHash)
	switch {
	case err == nil:
		if !bytes.Equal(existingPayloadHash, command.PayloadHash) {
			return CashExpectation{}, false, &CashExpectationIdempotencyConflictError{CommandID: command.CommandID}
		}
		existing, found, err := readCashExpectation(ctx, tx, existingID)
		if err != nil {
			return CashExpectation{}, false, err
		}
		if !found {
			return CashExpectation{}, false, fmt.Errorf("cash expectation command references missing fact %s", existingID)
		}
		if err := tx.Commit(ctx); err != nil {
			return CashExpectation{}, false, fmt.Errorf("commit cash expectation replay: %w", err)
		}
		return existing, true, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return CashExpectation{}, false, fmt.Errorf("read cash expectation command: %w", err)
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
		return CashExpectation{}, false, &ObligationNotFoundError{ObligationID: value.ObligationID}
	}
	if err != nil {
		return CashExpectation{}, false, fmt.Errorf("lock obligation for cash expectation: %w", err)
	}
	if status != StatusOpen {
		return CashExpectation{}, false, &CashExpectationStateConflictError{Status: status}
	}
	if currentRevision != value.ObligationRevision {
		return CashExpectation{}, false, &CurrentRevisionConflictError{Expected: value.ObligationRevision, Actual: currentRevision}
	}

	current, found, err := readObligation(ctx, tx, value.ObligationID)
	if err != nil {
		return CashExpectation{}, false, fmt.Errorf("read obligation for cash expectation: %w", err)
	}
	if !found {
		return CashExpectation{}, false, fmt.Errorf("locked obligation disappeared: %s", value.ObligationID)
	}
	if value.Action == CashExpectationActionSet && current.OutstandingAmount == "0" {
		return CashExpectation{}, false, &CashExpectationBalanceConflictError{Outstanding: current.OutstandingAmount}
	}

	var previousSequence int64
	var previousAction CashExpectationAction
	err = tx.QueryRow(ctx, `
		SELECT sequence, action
		FROM obligation_cash_expectations
		WHERE obligation_id = $1
		ORDER BY sequence DESC
		LIMIT 1
	`, value.ObligationID).Scan(&previousSequence, &previousAction)
	if errors.Is(err, pgx.ErrNoRows) {
		previousSequence = 0
		previousAction = ""
	} else if err != nil {
		return CashExpectation{}, false, fmt.Errorf("read latest cash expectation: %w", err)
	}
	if value.Action == CashExpectationActionClear && previousAction != CashExpectationActionSet {
		return CashExpectation{}, false, &CashExpectationClearConflictError{}
	}
	value.Sequence = previousSequence + 1

	if _, err := tx.Exec(ctx, `
		INSERT INTO obligation_cash_expectations (
			id, obligation_id, sequence, action, expected_on,
			obligation_revision, source_kind, expectation_reason,
			recorded_by_actor_kind, recorded_by_actor_id,
			recorded_by_actor_display_name, recorded_at
		)
		VALUES (
			$1, $2, $3, $4, NULLIF($5, '')::date,
			$6, $7, $8, $9, $10, $11, $12
		)
	`,
		value.ID, value.ObligationID, value.Sequence, value.Action, value.ExpectedOn,
		value.ObligationRevision, value.SourceKind, value.Reason,
		value.RecordedBy.Kind, value.RecordedBy.ID, value.RecordedBy.DisplayName, value.RecordedAt,
	); err != nil {
		return CashExpectation{}, false, fmt.Errorf("insert cash expectation: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO obligation_cash_expectation_commands (
			command_id, obligation_cash_expectation_id, obligation_id,
			expectation_sequence, expected_current_revision,
			payload_hash, actor_kind, actor_id, actor_display_name, recorded_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`,
		command.CommandID, value.ID, value.ObligationID, value.Sequence,
		value.ObligationRevision, command.PayloadHash,
		command.Actor.Kind, command.Actor.ID, command.Actor.DisplayName, command.RecordedAt,
	); err != nil {
		return CashExpectation{}, false, fmt.Errorf("insert cash expectation command: %w", err)
	}

	if entry.Details == nil {
		entry.Details = make(map[string]any)
	}
	entry.Details["obligation_cash_expectation_id"] = value.ID
	entry.Details["expectation_sequence"] = value.Sequence
	if err := audit.Insert(ctx, tx, entry); err != nil {
		return CashExpectation{}, false, err
	}

	persisted, found, err := readCashExpectation(ctx, tx, value.ID)
	if err != nil {
		return CashExpectation{}, false, err
	}
	if !found {
		return CashExpectation{}, false, fmt.Errorf("created cash expectation disappeared: %s", value.ID)
	}
	if err := tx.Commit(ctx); err != nil {
		return CashExpectation{}, false, fmt.Errorf("commit cash expectation: %w", err)
	}
	return persisted, false, nil
}

func (r *PostgresRepository) ListCurrentCashExpectationRows(ctx context.Context) ([]CashExpectationRow, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("begin current cash forecast read: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	rows, err := tx.Query(ctx, obligationSelect+`
		WHERE o.status = 'open'
		ORDER BY o.id
	`)
	if err != nil {
		return nil, fmt.Errorf("query current cash forecast obligations: %w", err)
	}
	values := make([]CashExpectationRow, 0)
	for rows.Next() {
		value, err := scanObligation(rows)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan current cash forecast obligation: %w", err)
		}
		values = append(values, CashExpectationRow{Obligation: value})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate current cash forecast obligations: %w", err)
	}
	rows.Close()

	expectationRows, err := tx.Query(ctx, cashExpectationSelect+`
		WHERE (obligation_id, sequence) IN (
			SELECT obligation_id, max(sequence)
			FROM obligation_cash_expectations
			GROUP BY obligation_id
		)
		ORDER BY obligation_id
	`)
	if err != nil {
		return nil, fmt.Errorf("query current cash expectations: %w", err)
	}
	latest := make(map[string]CashExpectation)
	for expectationRows.Next() {
		value, err := scanCashExpectation(expectationRows)
		if err != nil {
			expectationRows.Close()
			return nil, fmt.Errorf("scan current cash expectation: %w", err)
		}
		latest[value.ObligationID] = value
	}
	if err := expectationRows.Err(); err != nil {
		expectationRows.Close()
		return nil, fmt.Errorf("iterate current cash expectations: %w", err)
	}
	expectationRows.Close()

	for index := range values {
		if value, ok := latest[values[index].Obligation.ID]; ok {
			copy := value
			values[index].Expectation = &copy
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit current cash forecast read: %w", err)
	}
	return values, nil
}

func (r *PostgresRepository) ListCashExpectationHistory(
	ctx context.Context,
	obligationID string,
) (Obligation, []CashExpectation, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Obligation{}, nil, false, fmt.Errorf("begin cash expectation history read: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	obligationValue, found, err := readObligation(ctx, tx, obligationID)
	if err != nil {
		return Obligation{}, nil, false, fmt.Errorf("query cash expectation history obligation: %w", err)
	}
	if !found {
		if err := tx.Commit(ctx); err != nil {
			return Obligation{}, nil, false, fmt.Errorf("commit missing cash expectation history read: %w", err)
		}
		return Obligation{}, nil, false, nil
	}
	rows, err := tx.Query(ctx, cashExpectationSelect+`
		WHERE obligation_id = $1
		ORDER BY sequence
	`, obligationID)
	if err != nil {
		return Obligation{}, nil, false, fmt.Errorf("query cash expectation history: %w", err)
	}
	facts := make([]CashExpectation, 0)
	for rows.Next() {
		value, err := scanCashExpectation(rows)
		if err != nil {
			rows.Close()
			return Obligation{}, nil, false, fmt.Errorf("scan cash expectation history: %w", err)
		}
		facts = append(facts, value)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Obligation{}, nil, false, fmt.Errorf("iterate cash expectation history: %w", err)
	}
	rows.Close()
	if err := tx.Commit(ctx); err != nil {
		return Obligation{}, nil, false, fmt.Errorf("commit cash expectation history read: %w", err)
	}
	return obligationValue, facts, true, nil
}

const cashExpectationSelect = `
	SELECT
		id::text,
		obligation_id::text,
		sequence,
		action,
		COALESCE(expected_on::text, ''),
		obligation_revision,
		source_kind,
		expectation_reason,
		recorded_by_actor_kind,
		recorded_by_actor_id,
		recorded_by_actor_display_name,
		recorded_at
	FROM obligation_cash_expectations
`

func readCashExpectation(ctx context.Context, queryer queryRower, expectationID string) (CashExpectation, bool, error) {
	value, err := scanCashExpectation(queryer.QueryRow(ctx, cashExpectationSelect+` WHERE id = $1`, expectationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return CashExpectation{}, false, nil
	}
	if err != nil {
		return CashExpectation{}, false, err
	}
	return value, true, nil
}

func scanCashExpectation(row rowScanner) (CashExpectation, error) {
	var value CashExpectation
	if err := row.Scan(
		&value.ID,
		&value.ObligationID,
		&value.Sequence,
		&value.Action,
		&value.ExpectedOn,
		&value.ObligationRevision,
		&value.SourceKind,
		&value.Reason,
		&value.RecordedBy.Kind,
		&value.RecordedBy.ID,
		&value.RecordedBy.DisplayName,
		&value.RecordedAt,
	); err != nil {
		return CashExpectation{}, err
	}
	return value, nil
}
