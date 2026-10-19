package contract

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"omsbizfoundry/contract-ledger/internal/audit"
)

const draftChangeCommandColumns = `
	command_id::text,
	contract_id::text,
	action,
	expected_draft_version,
	resulting_draft_version,
	COALESCE(number_namespace, ''),
	COALESCE(contract_number, ''),
	COALESCE(title, ''),
	COALESCE(category, ''),
	COALESCE(currency, ''),
	COALESCE(total_amount::text, ''),
	COALESCE(to_char(signed_on, 'YYYY-MM-DD'), ''),
	COALESCE(to_char(effective_on, 'YYYY-MM-DD'), ''),
	COALESCE(cancellation_reason, ''),
	created_at
`

func (r *PostgresRepository) UpdateDraft(
	ctx context.Context,
	command DraftChangeCommand,
	candidate Contract,
) (DraftChangeCommand, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return DraftChangeCommand{}, false, fmt.Errorf("begin update contract draft: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if err := lockDraftChangeCommandID(ctx, tx, command.CommandID); err != nil {
		return DraftChangeCommand{}, false, err
	}
	if existing, found, err := readDraftChangeCommand(ctx, tx, command.CommandID); err != nil {
		return DraftChangeCommand{}, false, err
	} else if found {
		return existing, false, nil
	}
	if err := lockAndCheckDraft(ctx, tx, command.ContractID, command.ExpectedDraftVersion); err != nil {
		return DraftChangeCommand{}, false, err
	}

	current, err := scanContract(tx.QueryRow(ctx, `SELECT `+contractColumns+` FROM contracts WHERE id = $1`, command.ContractID))
	if err != nil {
		return DraftChangeCommand{}, false, fmt.Errorf("read locked contract draft: %w", err)
	}
	if sameDraftFields(current, candidate) {
		return DraftChangeCommand{}, false, &NoDraftChangesError{}
	}

	command.ResultingDraftVersion = command.ExpectedDraftVersion + 1
	result, err := tx.Exec(ctx, `
		UPDATE contracts
		SET number_namespace = $2,
			contract_number = $3,
			title = $4,
			category = NULLIF($5, ''),
			currency = $6,
			total_amount = NULLIF($7, '')::numeric,
			signed_on = NULLIF($8, '')::date,
			effective_on = NULLIF($9, '')::date,
			draft_version = $10,
			updated_at = $11
		WHERE id = $1 AND draft_version = $12
	`,
		command.ContractID,
		candidate.NumberNamespace,
		candidate.ContractNumber,
		candidate.Title,
		candidate.Category,
		candidate.Currency,
		candidate.TotalAmount,
		candidate.SignedOn,
		candidate.EffectiveOn,
		command.ResultingDraftVersion,
		command.CreatedAt,
		command.ExpectedDraftVersion,
	)
	if err != nil {
		return DraftChangeCommand{}, false, mapDraftUpdateError(err, candidate)
	}
	if result.RowsAffected() != 1 {
		return DraftChangeCommand{}, false, fmt.Errorf("update contract draft: expected one row, updated %d", result.RowsAffected())
	}
	if err := insertDraftChangeCommand(ctx, tx, command); err != nil {
		return DraftChangeCommand{}, false, err
	}
	if err := insertDraftChangeAudit(ctx, tx, command); err != nil {
		return DraftChangeCommand{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return DraftChangeCommand{}, false, fmt.Errorf("commit update contract draft: %w", err)
	}
	return command, true, nil
}

func (r *PostgresRepository) CancelDraft(ctx context.Context, command DraftChangeCommand) (DraftChangeCommand, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return DraftChangeCommand{}, false, fmt.Errorf("begin cancel contract draft: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if err := lockDraftChangeCommandID(ctx, tx, command.CommandID); err != nil {
		return DraftChangeCommand{}, false, err
	}
	if existing, found, err := readDraftChangeCommand(ctx, tx, command.CommandID); err != nil {
		return DraftChangeCommand{}, false, err
	} else if found {
		return existing, false, nil
	}
	if err := lockAndCheckDraft(ctx, tx, command.ContractID, command.ExpectedDraftVersion); err != nil {
		return DraftChangeCommand{}, false, err
	}

	command.ResultingDraftVersion = command.ExpectedDraftVersion + 1
	result, err := tx.Exec(ctx, `
		UPDATE contracts
		SET status = $2,
			draft_version = $3,
			updated_at = $4
		WHERE id = $1 AND draft_version = $5
	`, command.ContractID, StatusCancelled, command.ResultingDraftVersion, command.CreatedAt, command.ExpectedDraftVersion)
	if err != nil {
		return DraftChangeCommand{}, false, fmt.Errorf("mark contract draft cancelled: %w", err)
	}
	if result.RowsAffected() != 1 {
		return DraftChangeCommand{}, false, fmt.Errorf("cancel contract draft: expected one row, updated %d", result.RowsAffected())
	}
	if err := insertDraftChangeCommand(ctx, tx, command); err != nil {
		return DraftChangeCommand{}, false, err
	}
	if err := insertDraftChangeAudit(ctx, tx, command); err != nil {
		return DraftChangeCommand{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return DraftChangeCommand{}, false, fmt.Errorf("commit cancel contract draft: %w", err)
	}
	return command, true, nil
}

func lockDraftChangeCommandID(ctx context.Context, tx pgx.Tx, commandID string) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, commandID); err != nil {
		return fmt.Errorf("lock draft change command ID: %w", err)
	}
	return nil
}

func readDraftChangeCommand(ctx context.Context, tx pgx.Tx, commandID string) (DraftChangeCommand, bool, error) {
	command, err := scanDraftChangeCommand(tx.QueryRow(ctx, `
		SELECT `+draftChangeCommandColumns+`
		FROM contract_draft_change_commands
		WHERE command_id = $1
	`, commandID))
	if errors.Is(err, pgx.ErrNoRows) {
		return DraftChangeCommand{}, false, nil
	}
	if err != nil {
		return DraftChangeCommand{}, false, fmt.Errorf("query draft change command: %w", err)
	}
	return command, true, nil
}

func insertDraftChangeCommand(ctx context.Context, tx pgx.Tx, command DraftChangeCommand) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_draft_change_commands (
			command_id,
			contract_id,
			action,
			expected_draft_version,
			resulting_draft_version,
			number_namespace,
			contract_number,
			title,
			category,
			currency,
			total_amount,
			signed_on,
			effective_on,
			cancellation_reason,
			created_at
		)
		VALUES (
			$1, $2, $3, $4, $5,
			NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), NULLIF($9, ''), NULLIF($10, ''),
			NULLIF($11, '')::numeric, NULLIF($12, '')::date, NULLIF($13, '')::date,
			NULLIF($14, ''), $15
		)
	`,
		command.CommandID,
		command.ContractID,
		command.Action,
		command.ExpectedDraftVersion,
		command.ResultingDraftVersion,
		command.NumberNamespace,
		command.ContractNumber,
		command.Title,
		command.Category,
		command.Currency,
		command.TotalAmount,
		command.SignedOn,
		command.EffectiveOn,
		command.CancellationReason,
		command.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert draft change command: %w", err)
	}
	return nil
}

func insertDraftChangeAudit(ctx context.Context, tx pgx.Tx, command DraftChangeCommand) error {
	action := AuditActionDraftUpdated
	verb := "更新"
	if command.Action == DraftChangeActionCancel {
		action = AuditActionDraftCancelled
		verb = "取消"
	}
	return audit.Insert(ctx, tx, audit.Entry{
		ID:         command.AuditID,
		ObjectType: audit.ObjectContract,
		ObjectID:   command.ContractID,
		Action:     action,
		CommandID:  command.CommandID,
		Actor:      command.Actor,
		OccurredAt: command.CreatedAt,
		Summary: fmt.Sprintf(
			"%s合同草稿：版本 %d → %d",
			verb,
			command.ExpectedDraftVersion,
			command.ResultingDraftVersion,
		),
		Details: map[string]any{
			"expected_draft_version":  command.ExpectedDraftVersion,
			"resulting_draft_version": command.ResultingDraftVersion,
		},
	})
}

func scanDraftChangeCommand(row rowScanner) (DraftChangeCommand, error) {
	var value DraftChangeCommand
	if err := row.Scan(
		&value.CommandID,
		&value.ContractID,
		&value.Action,
		&value.ExpectedDraftVersion,
		&value.ResultingDraftVersion,
		&value.NumberNamespace,
		&value.ContractNumber,
		&value.Title,
		&value.Category,
		&value.Currency,
		&value.TotalAmount,
		&value.SignedOn,
		&value.EffectiveOn,
		&value.CancellationReason,
		&value.CreatedAt,
	); err != nil {
		return DraftChangeCommand{}, err
	}
	amount, err := normalizeAmount(value.TotalAmount)
	if err != nil {
		return DraftChangeCommand{}, fmt.Errorf("normalize stored draft change amount: %w", err)
	}
	value.TotalAmount = amount
	return value, nil
}

func sameDraftFields(left, right Contract) bool {
	return left.NumberNamespace == right.NumberNamespace &&
		left.ContractNumber == right.ContractNumber &&
		left.Title == right.Title &&
		left.Category == right.Category &&
		left.Currency == right.Currency &&
		left.TotalAmount == right.TotalAmount &&
		left.SignedOn == right.SignedOn &&
		left.EffectiveOn == right.EffectiveOn
}

func mapDraftUpdateError(err error, candidate Contract) error {
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && databaseError.Code == "23505" && databaseError.ConstraintName == "contracts_number_scope_unique" {
		return &DuplicateNumberError{
			NumberNamespace: candidate.NumberNamespace,
			ContractNumber:  candidate.ContractNumber,
		}
	}
	return fmt.Errorf("update contract draft fields: %w", err)
}
