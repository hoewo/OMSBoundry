package contract

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"omsbizfoundry/contract-ledger/internal/audit"
)

func (r *PostgresRepository) CorrectContract(
	ctx context.Context,
	command ContractCorrectionCommand,
) (ContractCorrectionCommand, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ContractCorrectionCommand{}, false, fmt.Errorf("begin contract correction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if err := lockActivationCommandID(ctx, tx, command.CommandID); err != nil {
		return ContractCorrectionCommand{}, false, err
	}
	if existing, found, err := readContractCorrectionCommand(ctx, tx, command.CommandID); err != nil {
		return ContractCorrectionCommand{}, false, err
	} else if found {
		return existing, false, nil
	}

	current, err := scanContract(tx.QueryRow(ctx, `
		SELECT `+contractColumns+`
		FROM contracts
		WHERE id = $1
		FOR UPDATE
	`, command.ContractID))
	if errors.Is(err, pgx.ErrNoRows) {
		return ContractCorrectionCommand{}, false, &ContractNotFoundError{ContractID: command.ContractID}
	}
	if err != nil {
		return ContractCorrectionCommand{}, false, fmt.Errorf("lock contract for correction: %w", err)
	}
	if current.Status != command.ExpectedStatus {
		return ContractCorrectionCommand{}, false, &ContractCorrectionStateConflictError{
			Expected: command.ExpectedStatus,
			Actual:   current.Status,
		}
	}
	if current.Status != StatusActive && current.Status != StatusTerminated && current.Status != StatusClosed {
		return ContractCorrectionCommand{}, false, &ContractCorrectionStateConflictError{
			Expected: command.ExpectedStatus,
			Actual:   current.Status,
		}
	}
	if current.CurrentRevision != command.ExpectedRevision {
		return ContractCorrectionCommand{}, false, &ContractRevisionConflictError{
			Expected: command.ExpectedRevision,
			Actual:   current.CurrentRevision,
		}
	}
	command.ResultingRevision = command.ExpectedRevision + 1
	if command.EffectiveOrder != command.ResultingRevision {
		return ContractCorrectionCommand{}, false, &ContractCorrectionPreconditionError{
			Field:   "effective_order",
			Message: "更正文书顺序必须紧接当前合同 revision",
		}
	}

	previous, err := scanContractRevision(tx.QueryRow(ctx, `
		SELECT `+contractRevisionColumns+`
		FROM contract_revisions
		WHERE contract_id = $1 AND revision = $2
		FOR SHARE
	`, command.ContractID, command.ExpectedRevision))
	if err != nil {
		return ContractCorrectionCommand{}, false, fmt.Errorf("read current revision for correction: %w", err)
	}
	var previousInstrumentEffectiveOn string
	if err := tx.QueryRow(ctx, `
		SELECT to_char(effective_on, 'YYYY-MM-DD')
		FROM contract_instruments
		WHERE id = $1
	`, previous.SourceInstrumentID).Scan(&previousInstrumentEffectiveOn); err != nil {
		return ContractCorrectionCommand{}, false, fmt.Errorf("read current instrument for correction: %w", err)
	}
	if command.InstrumentEffectiveOn < previousInstrumentEffectiveOn {
		return ContractCorrectionCommand{}, false, &ContractCorrectionPreconditionError{
			Field: "correction_instrument_effective_on",
			Message: fmt.Sprintf(
				"更正文书生效或确认日期不能早于前一份文书的生效日期 %s",
				previousInstrumentEffectiveOn,
			),
		}
	}
	if sameCorrectionSnapshot(previous, command) {
		return ContractCorrectionCommand{}, false, &ContractCorrectionNoChangesError{}
	}

	if err := insertContractCorrectionInstrument(ctx, tx, command); err != nil {
		return ContractCorrectionCommand{}, false, err
	}
	if err := insertCorrectedContractRevision(ctx, tx, command, previous); err != nil {
		return ContractCorrectionCommand{}, false, err
	}
	if err := copyCorrectionRevisionParties(ctx, tx, command); err != nil {
		return ContractCorrectionCommand{}, false, err
	}
	if err := insertContractCorrectionCommand(ctx, tx, command); err != nil {
		return ContractCorrectionCommand{}, false, err
	}
	result, err := tx.Exec(ctx, `
		UPDATE contracts
		SET current_revision = $4,
			updated_at = $5
		WHERE id = $1 AND status = $2 AND current_revision = $3
	`, command.ContractID, command.ExpectedStatus, command.ExpectedRevision,
		command.ResultingRevision, command.OccurredAt)
	if err != nil {
		return ContractCorrectionCommand{}, false, fmt.Errorf("advance corrected contract revision: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ContractCorrectionCommand{}, false, &ContractRevisionConflictError{
			Expected: command.ExpectedRevision,
			Actual:   current.CurrentRevision,
		}
	}
	if err := insertContractCorrectionAudit(ctx, tx, command, previous); err != nil {
		return ContractCorrectionCommand{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ContractCorrectionCommand{}, false, fmt.Errorf("commit contract correction: %w", err)
	}
	return command, true, nil
}

func sameCorrectionSnapshot(previous ContractRevision, command ContractCorrectionCommand) bool {
	return previous.Title == command.CorrectedTitle &&
		previous.Category == command.CorrectedCategory &&
		previous.TotalAmount == command.CorrectedTotalAmount &&
		previous.SignedOn == command.CorrectedSignedOn &&
		previous.EffectiveOn == command.CorrectedEffectiveOn
}

func readContractCorrectionCommand(
	ctx context.Context,
	tx pgx.Tx,
	commandID string,
) (ContractCorrectionCommand, bool, error) {
	var value ContractCorrectionCommand
	err := tx.QueryRow(ctx, `
		SELECT command_id::text, instrument_id::text, contract_id::text,
			expected_status, expected_revision,
			instrument_number, instrument_title, correction_reason,
			to_char(instrument_signed_on, 'YYYY-MM-DD'),
			to_char(instrument_effective_on, 'YYYY-MM-DD'), effective_order,
			corrected_title, COALESCE(corrected_category, ''),
			COALESCE(corrected_total_amount::text, ''),
			to_char(corrected_signed_on, 'YYYY-MM-DD'),
			to_char(corrected_effective_on, 'YYYY-MM-DD'),
			actor_kind, actor_id, actor_display_name,
			resulting_revision, occurred_at
		FROM contract_correction_commands
		WHERE command_id = $1
	`, commandID).Scan(
		&value.CommandID, &value.InstrumentID, &value.ContractID,
		&value.ExpectedStatus, &value.ExpectedRevision,
		&value.InstrumentNumber, &value.InstrumentTitle, &value.CorrectionReason,
		&value.InstrumentSignedOn, &value.InstrumentEffectiveOn, &value.EffectiveOrder,
		&value.CorrectedTitle, &value.CorrectedCategory, &value.CorrectedTotalAmount,
		&value.CorrectedSignedOn, &value.CorrectedEffectiveOn,
		&value.Actor.Kind, &value.Actor.ID, &value.Actor.DisplayName,
		&value.ResultingRevision, &value.OccurredAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ContractCorrectionCommand{}, false, nil
	}
	if err != nil {
		return ContractCorrectionCommand{}, false, fmt.Errorf("query contract correction command: %w", err)
	}
	value.CorrectedTotalAmount, err = normalizeAmount(value.CorrectedTotalAmount)
	if err != nil {
		return ContractCorrectionCommand{}, false, fmt.Errorf("normalize stored corrected amount: %w", err)
	}
	return value, true, nil
}

func insertContractCorrectionInstrument(ctx context.Context, tx pgx.Tx, command ContractCorrectionCommand) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_instruments (
			id, contract_id, instrument_type, instrument_number, title,
			change_summary, signed_on, effective_on, effective_order,
			amount_effect, amount_value, source_command_id, resulting_revision,
			actor_kind, actor_id, actor_display_name, recorded_at
		)
		VALUES (
			$1, $2, 'correction_instrument', $3, $4,
			$5, $6::date, $7::date, $8,
			'corrected', NULLIF($9, '')::numeric, $10, $11,
			$12, $13, $14, $15
		)
	`, command.InstrumentID, command.ContractID, command.InstrumentNumber,
		command.InstrumentTitle, command.CorrectionReason,
		command.InstrumentSignedOn, command.InstrumentEffectiveOn, command.EffectiveOrder,
		command.CorrectedTotalAmount, command.CommandID, command.ResultingRevision,
		command.Actor.Kind, command.Actor.ID, command.Actor.DisplayName, command.OccurredAt)
	if err == nil {
		return nil
	}
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) {
		switch databaseError.ConstraintName {
		case "contract_instruments_number_unique":
			return &DuplicateInstrumentNumberError{InstrumentNumber: command.InstrumentNumber}
		case "contract_instruments_effective_order_unique":
			return &ContractCorrectionPreconditionError{Field: "effective_order", Message: "该生效顺序已经被其他文书占用"}
		}
	}
	return fmt.Errorf("insert contract correction instrument: %w", err)
}

func insertCorrectedContractRevision(
	ctx context.Context,
	tx pgx.Tx,
	command ContractCorrectionCommand,
	previous ContractRevision,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_revisions (
			contract_id, revision, activation_command_id, source_draft_version,
			source_instrument_id, number_namespace, contract_number, title,
			category, currency, total_amount, signed_on, effective_on,
			actor_kind, actor_id, recorded_at
		)
		VALUES (
			$1, $2, NULL, NULL,
			$3, $4, $5, $6,
			NULLIF($7, ''), $8, NULLIF($9, '')::numeric, $10::date, $11::date,
			$12, $13, $14
		)
	`, command.ContractID, command.ResultingRevision, command.InstrumentID,
		previous.NumberNamespace, previous.ContractNumber,
		command.CorrectedTitle, command.CorrectedCategory,
		previous.Currency, command.CorrectedTotalAmount,
		command.CorrectedSignedOn, command.CorrectedEffectiveOn,
		command.Actor.Kind, command.Actor.ID, command.OccurredAt)
	if err != nil {
		return fmt.Errorf("insert corrected contract revision: %w", err)
	}
	return nil
}

func copyCorrectionRevisionParties(ctx context.Context, tx pgx.Tx, command ContractCorrectionCommand) error {
	result, err := tx.Exec(ctx, `
		INSERT INTO contract_revision_parties (
			contract_id, revision, position, source_party_link_id,
			source_party_id, source_party_revision, party_kind,
			legal_name, registration_identifier, role
		)
		SELECT
			contract_id, $3, position, source_party_link_id,
			source_party_id, source_party_revision, party_kind,
			legal_name, registration_identifier, role
		FROM contract_revision_parties
		WHERE contract_id = $1 AND revision = $2
		ORDER BY position
	`, command.ContractID, command.ExpectedRevision, command.ResultingRevision)
	if err != nil {
		return fmt.Errorf("copy corrected contract party snapshots: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("copy corrected contract party snapshots: current revision has no parties")
	}
	return nil
}

func insertContractCorrectionCommand(ctx context.Context, tx pgx.Tx, command ContractCorrectionCommand) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_correction_commands (
			command_id, instrument_id, contract_id, expected_status, expected_revision,
			instrument_number, instrument_title, correction_reason,
			instrument_signed_on, instrument_effective_on, effective_order,
			corrected_title, corrected_category, corrected_total_amount,
			corrected_signed_on, corrected_effective_on,
			actor_kind, actor_id, actor_display_name, resulting_revision, occurred_at
		)
		VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8,
			$9::date, $10::date, $11,
			$12, NULLIF($13, ''), NULLIF($14, '')::numeric,
			$15::date, $16::date,
			$17, $18, $19, $20, $21
		)
	`, command.CommandID, command.InstrumentID, command.ContractID,
		command.ExpectedStatus, command.ExpectedRevision,
		command.InstrumentNumber, command.InstrumentTitle, command.CorrectionReason,
		command.InstrumentSignedOn, command.InstrumentEffectiveOn, command.EffectiveOrder,
		command.CorrectedTitle, command.CorrectedCategory, command.CorrectedTotalAmount,
		command.CorrectedSignedOn, command.CorrectedEffectiveOn,
		command.Actor.Kind, command.Actor.ID, command.Actor.DisplayName,
		command.ResultingRevision, command.OccurredAt)
	if err != nil {
		return fmt.Errorf("insert contract correction command: %w", err)
	}
	return nil
}

func insertContractCorrectionAudit(
	ctx context.Context,
	tx pgx.Tx,
	command ContractCorrectionCommand,
	previous ContractRevision,
) error {
	return audit.Insert(ctx, tx, audit.Entry{
		ID: command.AuditID, ObjectType: audit.ObjectContract, ObjectID: command.ContractID,
		Action: AuditEventContractCorrectionAdded, CommandID: command.CommandID,
		Actor: command.Actor, OccurredAt: command.OccurredAt,
		Summary: fmt.Sprintf("受控更正合同 revision %d → %d：%s", command.ExpectedRevision, command.ResultingRevision, command.CorrectionReason),
		Details: map[string]any{
			"instrument_id":          command.InstrumentID,
			"instrument_number":      command.InstrumentNumber,
			"expected_status":        command.ExpectedStatus,
			"expected_revision":      command.ExpectedRevision,
			"resulting_revision":     command.ResultingRevision,
			"correction_reason":      command.CorrectionReason,
			"previous_title":         previous.Title,
			"corrected_title":        command.CorrectedTitle,
			"previous_category":      previous.Category,
			"corrected_category":     command.CorrectedCategory,
			"previous_total_amount":  previous.TotalAmount,
			"corrected_total_amount": command.CorrectedTotalAmount,
			"previous_signed_on":     previous.SignedOn,
			"corrected_signed_on":    command.CorrectedSignedOn,
			"previous_effective_on":  previous.EffectiveOn,
			"corrected_effective_on": command.CorrectedEffectiveOn,
		},
	})
}
