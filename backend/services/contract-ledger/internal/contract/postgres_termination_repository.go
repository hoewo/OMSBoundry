package contract

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"omsbizfoundry/contract-ledger/internal/audit"
)

func (r *PostgresRepository) Terminate(
	ctx context.Context,
	command TerminationCommand,
) (TerminationCommand, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return TerminationCommand{}, false, fmt.Errorf("begin terminate contract: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if err := lockActivationCommandID(ctx, tx, command.CommandID); err != nil {
		return TerminationCommand{}, false, err
	}
	if existing, found, err := readTerminationCommand(ctx, tx, command.CommandID); err != nil {
		return TerminationCommand{}, false, err
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
		return TerminationCommand{}, false, &ContractNotFoundError{ContractID: command.ContractID}
	}
	if err != nil {
		return TerminationCommand{}, false, fmt.Errorf("lock contract for termination: %w", err)
	}
	if current.Status != StatusActive {
		return TerminationCommand{}, false, &ContractTerminationStateConflictError{Status: current.Status}
	}
	if current.CurrentRevision != command.ExpectedRevision {
		return TerminationCommand{}, false, &ContractRevisionConflictError{
			Expected: command.ExpectedRevision,
			Actual:   current.CurrentRevision,
		}
	}
	command.ResultingRevision = command.ExpectedRevision + 1
	if command.EffectiveOrder != command.ResultingRevision {
		return TerminationCommand{}, false, &ContractTerminationPreconditionError{
			Field:   "effective_order",
			Message: "终止文书生效顺序必须紧接当前合同 revision",
		}
	}

	previous, err := scanContractRevision(tx.QueryRow(ctx, `
		SELECT `+contractRevisionColumns+`
		FROM contract_revisions
		WHERE contract_id = $1 AND revision = $2
		FOR SHARE
	`, command.ContractID, command.ExpectedRevision))
	if err != nil {
		return TerminationCommand{}, false, fmt.Errorf("read previous contract revision for termination: %w", err)
	}
	var previousEffectiveOn string
	if err := tx.QueryRow(ctx, `
		SELECT to_char(effective_on, 'YYYY-MM-DD')
		FROM contract_instruments
		WHERE id = $1
	`, previous.SourceInstrumentID).Scan(&previousEffectiveOn); err != nil {
		return TerminationCommand{}, false, fmt.Errorf("read previous contract instrument for termination: %w", err)
	}
	if command.EffectiveOn < previousEffectiveOn {
		return TerminationCommand{}, false, &ContractTerminationPreconditionError{
			Field: "instrument_effective_on",
			Message: fmt.Sprintf(
				"终止生效日期不能早于前一份文书的生效日期 %s",
				previousEffectiveOn,
			),
		}
	}
	command.ResultingAmount = previous.TotalAmount

	if err := insertTerminationInstrument(ctx, tx, command); err != nil {
		return TerminationCommand{}, false, err
	}
	if err := insertTerminationCommand(ctx, tx, command); err != nil {
		return TerminationCommand{}, false, err
	}
	if err := insertTerminationRevision(ctx, tx, command, previous); err != nil {
		return TerminationCommand{}, false, err
	}
	if err := copyTerminationRevisionParties(ctx, tx, command); err != nil {
		return TerminationCommand{}, false, err
	}
	result, err := tx.Exec(ctx, `
		UPDATE contracts
		SET status = $3,
			current_revision = $4,
			updated_at = $5
		WHERE id = $1 AND status = $2 AND current_revision = $6
	`, command.ContractID, StatusActive, StatusTerminated, command.ResultingRevision, command.OccurredAt, command.ExpectedRevision)
	if err != nil {
		return TerminationCommand{}, false, fmt.Errorf("mark contract terminated: %w", err)
	}
	if result.RowsAffected() != 1 {
		return TerminationCommand{}, false, &ContractRevisionConflictError{
			Expected: command.ExpectedRevision,
			Actual:   current.CurrentRevision,
		}
	}
	if err := insertContractTerminationAudit(ctx, tx, command); err != nil {
		return TerminationCommand{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return TerminationCommand{}, false, fmt.Errorf("commit contract termination: %w", err)
	}
	return command, true, nil
}

func readTerminationCommand(
	ctx context.Context,
	tx pgx.Tx,
	commandID string,
) (TerminationCommand, bool, error) {
	var value TerminationCommand
	err := tx.QueryRow(ctx, `
		SELECT c.command_id::text, c.instrument_id::text, c.contract_id::text,
			c.expected_revision, c.instrument_number, c.title, c.termination_reason,
			to_char(c.signed_on, 'YYYY-MM-DD'), to_char(c.effective_on, 'YYYY-MM-DD'),
			c.effective_order, c.actor_kind, c.actor_id, c.actor_display_name,
			c.resulting_revision, COALESCE(r.total_amount::text, ''), c.occurred_at
		FROM contract_termination_commands c
		JOIN contract_revisions r
			ON r.contract_id = c.contract_id AND r.revision = c.resulting_revision
		WHERE c.command_id = $1
	`, commandID).Scan(
		&value.CommandID,
		&value.InstrumentID,
		&value.ContractID,
		&value.ExpectedRevision,
		&value.InstrumentNumber,
		&value.Title,
		&value.TerminationReason,
		&value.SignedOn,
		&value.EffectiveOn,
		&value.EffectiveOrder,
		&value.Actor.Kind,
		&value.Actor.ID,
		&value.Actor.DisplayName,
		&value.ResultingRevision,
		&value.ResultingAmount,
		&value.OccurredAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return TerminationCommand{}, false, nil
	}
	if err != nil {
		return TerminationCommand{}, false, fmt.Errorf("query contract termination command: %w", err)
	}
	amount, err := normalizeAmount(value.ResultingAmount)
	if err != nil {
		return TerminationCommand{}, false, fmt.Errorf("normalize stored termination amount: %w", err)
	}
	value.ResultingAmount = amount
	return value, true, nil
}

func insertTerminationInstrument(ctx context.Context, tx pgx.Tx, command TerminationCommand) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_instruments (
			id, contract_id, instrument_type, instrument_number, title,
			change_summary, signed_on, effective_on, effective_order,
			amount_effect, amount_value, source_command_id, resulting_revision,
			actor_kind, actor_id, actor_display_name, recorded_at
		)
		VALUES (
			$1, $2, 'termination_instrument', $3, $4,
			$5, $6::date, $7::date, $8,
			'unchanged', NULL, $9, $10,
			$11, $12, $13, $14
		)
	`,
		command.InstrumentID,
		command.ContractID,
		command.InstrumentNumber,
		command.Title,
		command.TerminationReason,
		command.SignedOn,
		command.EffectiveOn,
		command.EffectiveOrder,
		command.CommandID,
		command.ResultingRevision,
		command.Actor.Kind,
		command.Actor.ID,
		command.Actor.DisplayName,
		command.OccurredAt,
	)
	if err == nil {
		return nil
	}
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) {
		switch databaseError.ConstraintName {
		case "contract_instruments_number_unique":
			return &DuplicateInstrumentNumberError{InstrumentNumber: command.InstrumentNumber}
		case "contract_instruments_effective_order_unique":
			return &ContractTerminationPreconditionError{Field: "effective_order", Message: "该生效顺序已经被其他文书占用"}
		}
	}
	return fmt.Errorf("insert termination instrument: %w", err)
}

func insertTerminationCommand(ctx context.Context, tx pgx.Tx, command TerminationCommand) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_termination_commands (
			command_id, instrument_id, contract_id, expected_revision,
			instrument_number, title, termination_reason, signed_on, effective_on,
			effective_order, actor_kind, actor_id, actor_display_name,
			resulting_revision, occurred_at
		)
		VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8::date, $9::date,
			$10, $11, $12, $13,
			$14, $15
		)
	`,
		command.CommandID,
		command.InstrumentID,
		command.ContractID,
		command.ExpectedRevision,
		command.InstrumentNumber,
		command.Title,
		command.TerminationReason,
		command.SignedOn,
		command.EffectiveOn,
		command.EffectiveOrder,
		command.Actor.Kind,
		command.Actor.ID,
		command.Actor.DisplayName,
		command.ResultingRevision,
		command.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("insert contract termination command: %w", err)
	}
	return nil
}

func insertTerminationRevision(
	ctx context.Context,
	tx pgx.Tx,
	command TerminationCommand,
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
	`,
		command.ContractID,
		command.ResultingRevision,
		command.InstrumentID,
		previous.NumberNamespace,
		previous.ContractNumber,
		previous.Title,
		previous.Category,
		previous.Currency,
		command.ResultingAmount,
		previous.SignedOn,
		previous.EffectiveOn,
		command.Actor.Kind,
		command.Actor.ID,
		command.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("insert termination contract revision: %w", err)
	}
	return nil
}

func copyTerminationRevisionParties(ctx context.Context, tx pgx.Tx, command TerminationCommand) error {
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
		return fmt.Errorf("copy termination revision party snapshots: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("copy termination revision party snapshots: previous revision has no parties")
	}
	return nil
}

func insertContractTerminationAudit(ctx context.Context, tx pgx.Tx, command TerminationCommand) error {
	return audit.Insert(ctx, tx, audit.Entry{
		ID: command.AuditID, ObjectType: audit.ObjectContract, ObjectID: command.ContractID,
		Action: AuditEventContractTerminated, CommandID: command.CommandID,
		Actor: command.Actor, OccurredAt: command.OccurredAt,
		Summary: fmt.Sprintf(
			"登记生效终止文书 %s：revision %d → %d，合同已终止",
			command.InstrumentNumber,
			command.ExpectedRevision,
			command.ResultingRevision,
		),
		Details: map[string]any{
			"instrument_id":      command.InstrumentID,
			"instrument_number":  command.InstrumentNumber,
			"termination_reason": command.TerminationReason,
			"effective_order":    command.EffectiveOrder,
			"effective_amount":   command.ResultingAmount,
			"expected_revision":  command.ExpectedRevision,
			"resulting_revision": command.ResultingRevision,
			"resulting_status":   StatusTerminated,
		},
	})
}
