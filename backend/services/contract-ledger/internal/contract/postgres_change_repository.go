package contract

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"omsbizfoundry/contract-ledger/internal/audit"
)

const contractInstrumentColumns = `
	id::text,
	contract_id::text,
	instrument_type,
	instrument_number,
	title,
	change_summary,
	to_char(signed_on, 'YYYY-MM-DD'),
	to_char(effective_on, 'YYYY-MM-DD'),
	effective_order,
	amount_effect,
	COALESCE(amount_value::text, ''),
	source_command_id::text,
	resulting_revision,
	actor_kind,
	actor_id,
	actor_display_name,
	recorded_at
`

func (r *PostgresRepository) AppendChange(
	ctx context.Context,
	command ContractChangeCommand,
) (ContractChangeCommand, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ContractChangeCommand{}, false, fmt.Errorf("begin append contract change: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	value, created, err := r.AppendChangeInTransaction(ctx, tx, command)
	if err != nil {
		return ContractChangeCommand{}, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return ContractChangeCommand{}, false, fmt.Errorf("commit contract change: %w", err)
	}
	return value, created, nil
}

// AppendChangeInTransaction shares exactly the native invariants with the
// independent command. Only a Contract-owned caller may compose its typed
// customer intent mapping in this same transaction; this never commits it.
func (r *PostgresRepository) AppendChangeInTransaction(ctx context.Context, tx pgx.Tx, command ContractChangeCommand) (ContractChangeCommand, bool, error) {

	if err := lockActivationCommandID(ctx, tx, command.CommandID); err != nil {
		return ContractChangeCommand{}, false, err
	}
	if existing, found, err := readContractChangeCommand(ctx, tx, command.CommandID); err != nil {
		return ContractChangeCommand{}, false, err
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
		return ContractChangeCommand{}, false, &ContractNotFoundError{ContractID: command.ContractID}
	}
	if err != nil {
		return ContractChangeCommand{}, false, fmt.Errorf("lock contract for change: %w", err)
	}
	if current.Status != StatusActive {
		return ContractChangeCommand{}, false, &ContractChangeStateConflictError{Status: current.Status}
	}
	if current.CurrentRevision != command.ExpectedRevision {
		return ContractChangeCommand{}, false, &ContractRevisionConflictError{
			Expected: command.ExpectedRevision,
			Actual:   current.CurrentRevision,
		}
	}
	command.ResultingRevision = command.ExpectedRevision + 1
	if command.EffectiveOrder != command.ResultingRevision {
		return ContractChangeCommand{}, false, &ContractChangePreconditionError{
			Field:   "effective_order",
			Message: "生效顺序必须紧接当前合同 revision",
		}
	}

	previous, err := scanContractRevision(tx.QueryRow(ctx, `
		SELECT `+contractRevisionColumns+`
		FROM contract_revisions
		WHERE contract_id = $1 AND revision = $2
		FOR SHARE
	`, command.ContractID, command.ExpectedRevision))
	if err != nil {
		return ContractChangeCommand{}, false, fmt.Errorf("read previous contract revision: %w", err)
	}
	var previousEffectiveOn string
	if err := tx.QueryRow(ctx, `
		SELECT to_char(effective_on, 'YYYY-MM-DD')
		FROM contract_instruments
		WHERE id = $1
	`, previous.SourceInstrumentID).Scan(&previousEffectiveOn); err != nil {
		return ContractChangeCommand{}, false, fmt.Errorf("read previous contract instrument: %w", err)
	}
	if command.EffectiveOn < previousEffectiveOn {
		return ContractChangeCommand{}, false, &ContractChangePreconditionError{
			Field: "instrument_effective_on",
			Message: fmt.Sprintf(
				"补充协议生效日期不能早于前一份文书的生效日期 %s",
				previousEffectiveOn,
			),
		}
	}

	resultingAmount, err := applyAmountEffect(previous.TotalAmount, command.AmountEffect, command.AmountValue)
	if err != nil {
		return ContractChangeCommand{}, false, err
	}
	command.ResultingAmount = resultingAmount

	if err := insertContractInstrument(ctx, tx, command); err != nil {
		return ContractChangeCommand{}, false, err
	}
	if err := insertContractChangeCommand(ctx, tx, command); err != nil {
		return ContractChangeCommand{}, false, err
	}
	if err := insertChangedContractRevision(ctx, tx, command, previous); err != nil {
		return ContractChangeCommand{}, false, err
	}
	if err := copyContractRevisionParties(ctx, tx, command); err != nil {
		return ContractChangeCommand{}, false, err
	}
	result, err := tx.Exec(ctx, `
		UPDATE contracts
		SET current_revision = $3,
			updated_at = $4
		WHERE id = $1 AND status = 'active' AND current_revision = $2
	`, command.ContractID, command.ExpectedRevision, command.ResultingRevision, command.OccurredAt)
	if err != nil {
		return ContractChangeCommand{}, false, fmt.Errorf("advance current contract revision: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ContractChangeCommand{}, false, &ContractRevisionConflictError{
			Expected: command.ExpectedRevision,
			Actual:   current.CurrentRevision,
		}
	}
	if err := insertContractChangeAudit(ctx, tx, command); err != nil {
		return ContractChangeCommand{}, false, err
	}
	return command, true, nil
}

func readContractChangeCommand(
	ctx context.Context,
	tx pgx.Tx,
	commandID string,
) (ContractChangeCommand, bool, error) {
	var value ContractChangeCommand
	err := tx.QueryRow(ctx, `
		SELECT c.command_id::text, c.instrument_id::text, c.contract_id::text,
			c.expected_revision, c.instrument_number, c.title, c.change_summary,
			to_char(c.signed_on, 'YYYY-MM-DD'), to_char(c.effective_on, 'YYYY-MM-DD'),
			c.effective_order, c.amount_effect, COALESCE(c.amount_value::text, ''),
			c.actor_kind, c.actor_id, c.actor_display_name,
			c.resulting_revision, COALESCE(r.total_amount::text, ''), c.occurred_at
		FROM contract_change_commands c
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
		&value.ChangeSummary,
		&value.SignedOn,
		&value.EffectiveOn,
		&value.EffectiveOrder,
		&value.AmountEffect,
		&value.AmountValue,
		&value.Actor.Kind,
		&value.Actor.ID,
		&value.Actor.DisplayName,
		&value.ResultingRevision,
		&value.ResultingAmount,
		&value.OccurredAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ContractChangeCommand{}, false, nil
	}
	if err != nil {
		return ContractChangeCommand{}, false, fmt.Errorf("query contract change command: %w", err)
	}
	normalizedValue, err := normalizeStoredChangeAmount(value.AmountEffect, value.AmountValue)
	if err != nil {
		return ContractChangeCommand{}, false, err
	}
	value.AmountValue = normalizedValue
	if value.ResultingAmount != "" {
		resultingAmount, err := normalizeAmount(value.ResultingAmount)
		if err != nil {
			return ContractChangeCommand{}, false, fmt.Errorf("normalize stored resulting amount: %w", err)
		}
		value.ResultingAmount = resultingAmount
	}
	return value, true, nil
}

func normalizeStoredChangeAmount(effect, value string) (string, error) {
	normalized, err := normalizeChangeAmount(effect, value)
	if err != nil {
		return "", fmt.Errorf("normalize stored change amount: %w", err)
	}
	return normalized, nil
}

func insertContractInstrument(ctx context.Context, tx pgx.Tx, command ContractChangeCommand) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_instruments (
			id, contract_id, instrument_type, instrument_number, title,
			change_summary, signed_on, effective_on, effective_order,
			amount_effect, amount_value, source_command_id, resulting_revision,
			actor_kind, actor_id, actor_display_name, recorded_at
		)
		VALUES (
			$1, $2, 'supplemental_agreement', $3, $4,
			$5, $6::date, $7::date, $8,
			$9, NULLIF($10, '')::numeric, $11, $12,
			$13, $14, $15, $16
		)
	`,
		command.InstrumentID,
		command.ContractID,
		command.InstrumentNumber,
		command.Title,
		command.ChangeSummary,
		command.SignedOn,
		command.EffectiveOn,
		command.EffectiveOrder,
		command.AmountEffect,
		command.AmountValue,
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
			return &ContractChangePreconditionError{Field: "effective_order", Message: "该生效顺序已经被其他文书占用"}
		}
	}
	return fmt.Errorf("insert contract instrument: %w", err)
}

func insertContractChangeCommand(ctx context.Context, tx pgx.Tx, command ContractChangeCommand) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_change_commands (
			command_id, instrument_id, contract_id, expected_revision,
			instrument_number, title, change_summary, signed_on, effective_on,
			effective_order, amount_effect, amount_value,
			actor_kind, actor_id, actor_display_name, resulting_revision, occurred_at
		)
		VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8::date, $9::date,
			$10, $11, NULLIF($12, '')::numeric,
			$13, $14, $15, $16, $17
		)
	`,
		command.CommandID,
		command.InstrumentID,
		command.ContractID,
		command.ExpectedRevision,
		command.InstrumentNumber,
		command.Title,
		command.ChangeSummary,
		command.SignedOn,
		command.EffectiveOn,
		command.EffectiveOrder,
		command.AmountEffect,
		command.AmountValue,
		command.Actor.Kind,
		command.Actor.ID,
		command.Actor.DisplayName,
		command.ResultingRevision,
		command.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("insert contract change command: %w", err)
	}
	return nil
}

func insertChangedContractRevision(
	ctx context.Context,
	tx pgx.Tx,
	command ContractChangeCommand,
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
		return fmt.Errorf("insert changed contract revision: %w", err)
	}
	return nil
}

func copyContractRevisionParties(ctx context.Context, tx pgx.Tx, command ContractChangeCommand) error {
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
		return fmt.Errorf("copy contract revision party snapshots: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("copy contract revision party snapshots: previous revision has no parties")
	}
	return nil
}

func insertContractChangeAudit(ctx context.Context, tx pgx.Tx, command ContractChangeCommand) error {
	return audit.Insert(ctx, tx, audit.Entry{
		ID:         command.AuditID,
		ObjectType: audit.ObjectContract,
		ObjectID:   command.ContractID,
		Action:     AuditEventContractChangeActivated,
		CommandID:  command.CommandID,
		Actor:      command.Actor,
		OccurredAt: command.OccurredAt,
		Summary: fmt.Sprintf(
			"登记生效补充协议 %s：revision %d → %d",
			command.InstrumentNumber,
			command.ExpectedRevision,
			command.ResultingRevision,
		),
		Details: map[string]any{
			"instrument_id":      command.InstrumentID,
			"instrument_number":  command.InstrumentNumber,
			"effective_order":    command.EffectiveOrder,
			"amount_effect":      command.AmountEffect,
			"amount_value":       command.AmountValue,
			"effective_amount":   command.ResultingAmount,
			"expected_revision":  command.ExpectedRevision,
			"resulting_revision": command.ResultingRevision,
		},
	})
}

func loadContractInstruments(ctx context.Context, tx pgx.Tx, contractID string) ([]ContractInstrument, error) {
	rows, err := tx.Query(ctx, `
		SELECT `+contractInstrumentColumns+`
		FROM contract_instruments
		WHERE contract_id = $1
		ORDER BY effective_order, id
	`, contractID)
	if err != nil {
		return nil, fmt.Errorf("query contract instruments: %w", err)
	}
	defer rows.Close()
	instruments := make([]ContractInstrument, 0)
	for rows.Next() {
		instrument, err := scanContractInstrument(rows)
		if err != nil {
			return nil, err
		}
		instruments = append(instruments, instrument)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contract instruments: %w", err)
	}
	return instruments, nil
}

func scanContractInstrument(row rowScanner) (ContractInstrument, error) {
	var value ContractInstrument
	if err := row.Scan(
		&value.ID,
		&value.ContractID,
		&value.InstrumentType,
		&value.InstrumentNumber,
		&value.Title,
		&value.ChangeSummary,
		&value.SignedOn,
		&value.EffectiveOn,
		&value.EffectiveOrder,
		&value.AmountEffect,
		&value.AmountValue,
		&value.SourceCommandID,
		&value.ResultingRevision,
		&value.Actor.Kind,
		&value.Actor.ID,
		&value.Actor.DisplayName,
		&value.RecordedAt,
	); err != nil {
		return ContractInstrument{}, fmt.Errorf("scan contract instrument: %w", err)
	}
	var err error
	if value.AmountEffect == AmountEffectOriginal || value.AmountEffect == AmountEffectCorrected {
		value.AmountValue, err = normalizeAmount(value.AmountValue)
	} else {
		value.AmountValue, err = normalizeStoredChangeAmount(value.AmountEffect, value.AmountValue)
	}
	if err != nil {
		return ContractInstrument{}, fmt.Errorf("normalize instrument amount: %w", err)
	}
	return value, nil
}

func loadContractRevisions(ctx context.Context, tx pgx.Tx, contractID string) ([]ContractRevision, error) {
	rows, err := tx.Query(ctx, `
		SELECT `+contractRevisionColumns+`
		FROM contract_revisions
		WHERE contract_id = $1
		ORDER BY revision
	`, contractID)
	if err != nil {
		return nil, fmt.Errorf("query contract revisions: %w", err)
	}
	defer rows.Close()
	revisions := make([]ContractRevision, 0)
	for rows.Next() {
		revision, err := scanContractRevision(rows)
		if err != nil {
			return nil, err
		}
		revisions = append(revisions, revision)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contract revisions: %w", err)
	}
	return revisions, nil
}
