package contract

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"omsbizfoundry/contract-ledger/internal/audit"
)

const contractSettlementColumns = `
	s.id::text,
	s.contract_id::text,
	s.source_revision,
	s.settlement_sequence,
	s.settlement_kind,
	s.statement_number,
	s.title,
	s.total_amount::text,
	s.currency,
	to_char(s.confirmed_on, 'YYYY-MM-DD'),
	s.basis,
	s.actor_kind,
	s.actor_id,
	s.actor_display_name,
	s.recorded_at
`

func (r *PostgresRepository) GetSettlement(ctx context.Context, settlementID string) (ContractSettlement, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ContractSettlement{}, false, fmt.Errorf("begin read contract settlement: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	value, err := getContractSettlement(ctx, tx, settlementID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ContractSettlement{}, false, nil
	}
	if err != nil {
		return ContractSettlement{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ContractSettlement{}, false, fmt.Errorf("commit read contract settlement: %w", err)
	}
	return value, true, nil
}

func (r *PostgresRepository) RecordSettlement(
	ctx context.Context,
	command ContractSettlementCommand,
) (ContractSettlementCommand, ContractSettlement, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ContractSettlementCommand{}, ContractSettlement{}, false, fmt.Errorf("begin contract settlement: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if err := lockActivationCommandID(ctx, tx, command.CommandID); err != nil {
		return ContractSettlementCommand{}, ContractSettlement{}, false, err
	}
	if existingCommand, settlement, found, err := readContractSettlementCommand(ctx, tx, command.CommandID); err != nil {
		return ContractSettlementCommand{}, ContractSettlement{}, false, err
	} else if found {
		return existingCommand, settlement, false, nil
	}

	current, err := scanContract(tx.QueryRow(ctx, `
		SELECT `+contractColumns+`
		FROM contracts
		WHERE id = $1
		FOR UPDATE
	`, command.ContractID))
	if errors.Is(err, pgx.ErrNoRows) {
		return ContractSettlementCommand{}, ContractSettlement{}, false, &ContractNotFoundError{ContractID: command.ContractID}
	}
	if err != nil {
		return ContractSettlementCommand{}, ContractSettlement{}, false, fmt.Errorf("lock contract for settlement: %w", err)
	}
	if current.Status != StatusActive && current.Status != StatusTerminated {
		return ContractSettlementCommand{}, ContractSettlement{}, false, &ContractSettlementStateConflictError{Status: current.Status}
	}
	if current.CurrentRevision != command.ExpectedContractRevision {
		return ContractSettlementCommand{}, ContractSettlement{}, false, &ContractRevisionConflictError{
			Expected: command.ExpectedContractRevision,
			Actual:   current.CurrentRevision,
		}
	}

	var actualSequence int64
	var finalized bool
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(max(settlement_sequence), 0),
			COALESCE(bool_or(settlement_kind = 'final'), false)
		FROM contract_settlements
		WHERE contract_id = $1
	`, command.ContractID).Scan(&actualSequence, &finalized); err != nil {
		return ContractSettlementCommand{}, ContractSettlement{}, false, fmt.Errorf("read contract settlement head: %w", err)
	}
	if finalized {
		return ContractSettlementCommand{}, ContractSettlement{}, false, &ContractSettlementFinalizedError{}
	}
	if actualSequence != command.ExpectedSettlementSequence {
		return ContractSettlementCommand{}, ContractSettlement{}, false, &ContractSettlementSequenceConflictError{
			Expected: command.ExpectedSettlementSequence,
			Actual:   actualSequence,
		}
	}
	command.ResultingSettlementSequence = actualSequence + 1

	if err := tx.QueryRow(ctx, `
		SELECT currency
		FROM contract_revisions
		WHERE contract_id = $1 AND revision = $2
		FOR SHARE
	`, command.ContractID, command.ExpectedContractRevision).Scan(&command.Currency); err != nil {
		return ContractSettlementCommand{}, ContractSettlement{}, false, fmt.Errorf("read settlement source revision currency: %w", err)
	}

	if err := insertContractSettlement(ctx, tx, command); err != nil {
		return ContractSettlementCommand{}, ContractSettlement{}, false, err
	}
	if err := copyContractSettlementParties(ctx, tx, command); err != nil {
		return ContractSettlementCommand{}, ContractSettlement{}, false, err
	}
	if err := insertContractSettlementCommand(ctx, tx, command); err != nil {
		return ContractSettlementCommand{}, ContractSettlement{}, false, err
	}
	if err := insertContractSettlementAudit(ctx, tx, command); err != nil {
		return ContractSettlementCommand{}, ContractSettlement{}, false, err
	}
	settlement, err := getContractSettlement(ctx, tx, command.SettlementID)
	if err != nil {
		return ContractSettlementCommand{}, ContractSettlement{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ContractSettlementCommand{}, ContractSettlement{}, false, fmt.Errorf("commit contract settlement: %w", err)
	}
	return command, settlement, true, nil
}

func readContractSettlementCommand(
	ctx context.Context,
	tx pgx.Tx,
	commandID string,
) (ContractSettlementCommand, ContractSettlement, bool, error) {
	var value ContractSettlementCommand
	err := tx.QueryRow(ctx, `
		SELECT c.command_id::text, c.settlement_id::text, c.contract_id::text,
			c.expected_contract_revision, c.expected_settlement_sequence,
			c.settlement_kind, c.statement_number, c.title, c.total_amount::text,
			to_char(c.confirmed_on, 'YYYY-MM-DD'), c.basis, c.confirming_party_positions,
			c.actor_kind, c.actor_id, c.actor_display_name,
			c.resulting_settlement_sequence, s.currency, c.occurred_at
		FROM contract_settlement_commands c
		JOIN contract_settlements s ON s.id = c.settlement_id
		WHERE c.command_id = $1
	`, commandID).Scan(
		&value.CommandID, &value.SettlementID, &value.ContractID,
		&value.ExpectedContractRevision, &value.ExpectedSettlementSequence,
		&value.Kind, &value.StatementNumber, &value.Title, &value.Amount,
		&value.ConfirmedOn, &value.Basis, &value.ConfirmingPartyPositions,
		&value.Actor.Kind, &value.Actor.ID, &value.Actor.DisplayName,
		&value.ResultingSettlementSequence, &value.Currency, &value.OccurredAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ContractSettlementCommand{}, ContractSettlement{}, false, nil
	}
	if err != nil {
		return ContractSettlementCommand{}, ContractSettlement{}, false, fmt.Errorf("query contract settlement command: %w", err)
	}
	value.Amount, err = normalizeSettlementAmount(value.Amount)
	if err != nil {
		return ContractSettlementCommand{}, ContractSettlement{}, false, fmt.Errorf("normalize stored settlement amount: %w", err)
	}
	settlement, err := getContractSettlement(ctx, tx, value.SettlementID)
	if err != nil {
		return ContractSettlementCommand{}, ContractSettlement{}, false, err
	}
	return value, settlement, true, nil
}

func insertContractSettlement(ctx context.Context, tx pgx.Tx, command ContractSettlementCommand) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_settlements (
			id, contract_id, source_revision, settlement_sequence, settlement_kind,
			statement_number, title, total_amount, currency, confirmed_on, basis,
			actor_kind, actor_id, actor_display_name, recorded_at
		)
		VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8::numeric, $9, $10::date, $11,
			$12, $13, $14, $15
		)
	`, command.SettlementID, command.ContractID, command.ExpectedContractRevision,
		command.ResultingSettlementSequence, command.Kind, command.StatementNumber,
		command.Title, command.Amount, command.Currency, command.ConfirmedOn, command.Basis,
		command.Actor.Kind, command.Actor.ID, command.Actor.DisplayName, command.OccurredAt)
	if err == nil {
		return nil
	}
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) {
		switch databaseError.ConstraintName {
		case "contract_settlements_number_unique":
			return &DuplicateSettlementNumberError{StatementNumber: command.StatementNumber}
		case "contract_settlements_sequence_unique":
			return &ContractSettlementSequenceConflictError{
				Expected: command.ExpectedSettlementSequence,
				Actual:   command.ResultingSettlementSequence,
			}
		case "contract_settlements_final_unique":
			return &ContractSettlementFinalizedError{}
		}
	}
	return fmt.Errorf("insert contract settlement: %w", err)
}

func copyContractSettlementParties(ctx context.Context, tx pgx.Tx, command ContractSettlementCommand) error {
	result, err := tx.Exec(ctx, `
		INSERT INTO contract_settlement_parties (
			settlement_id, position, source_party_link_id, source_party_id,
			source_party_revision, party_kind, legal_name, registration_identifier, role
		)
		SELECT $1, position, source_party_link_id, source_party_id,
			source_party_revision, party_kind, legal_name, registration_identifier, role
		FROM contract_revision_parties
		WHERE contract_id = $2 AND revision = $3
			AND position = ANY($4::bigint[])
		ORDER BY position
	`, command.SettlementID, command.ContractID, command.ExpectedContractRevision, command.ConfirmingPartyPositions)
	if err != nil {
		return fmt.Errorf("copy settlement confirming parties: %w", err)
	}
	if result.RowsAffected() != int64(len(command.ConfirmingPartyPositions)) {
		return &ContractSettlementPartyError{Message: "所选确认主体不属于当前合同 revision，请刷新后重试"}
	}
	return nil
}

func insertContractSettlementCommand(ctx context.Context, tx pgx.Tx, command ContractSettlementCommand) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_settlement_commands (
			command_id, settlement_id, contract_id,
			expected_contract_revision, expected_settlement_sequence,
			settlement_kind, statement_number, title, total_amount, confirmed_on, basis,
			confirming_party_positions, actor_kind, actor_id, actor_display_name,
			resulting_settlement_sequence, occurred_at
		)
		VALUES (
			$1, $2, $3,
			$4, $5,
			$6, $7, $8, $9::numeric, $10::date, $11,
			$12::bigint[], $13, $14, $15,
			$16, $17
		)
	`, command.CommandID, command.SettlementID, command.ContractID,
		command.ExpectedContractRevision, command.ExpectedSettlementSequence,
		command.Kind, command.StatementNumber, command.Title, command.Amount,
		command.ConfirmedOn, command.Basis, command.ConfirmingPartyPositions,
		command.Actor.Kind, command.Actor.ID, command.Actor.DisplayName,
		command.ResultingSettlementSequence, command.OccurredAt)
	if err != nil {
		return fmt.Errorf("insert contract settlement command: %w", err)
	}
	return nil
}

func insertContractSettlementAudit(ctx context.Context, tx pgx.Tx, command ContractSettlementCommand) error {
	return audit.Insert(ctx, tx, audit.Entry{
		ID: command.AuditID, ObjectType: audit.ObjectContract, ObjectID: command.ContractID,
		Action: AuditEventContractSettlementRecorded, CommandID: command.CommandID,
		Actor: command.Actor, OccurredAt: command.OccurredAt,
		Summary: fmt.Sprintf("登记%s %s：累计金额 %s %s", command.Kind.Label(), command.StatementNumber, command.Amount, command.Currency),
		Details: map[string]any{
			"settlement_id":              command.SettlementID,
			"settlement_kind":            command.Kind,
			"settlement_sequence":        command.ResultingSettlementSequence,
			"statement_number":           command.StatementNumber,
			"source_revision":            command.ExpectedContractRevision,
			"total_amount":               command.Amount,
			"currency":                   command.Currency,
			"confirmed_on":               command.ConfirmedOn,
			"confirming_party_positions": command.ConfirmingPartyPositions,
		},
	})
}

func getContractSettlement(ctx context.Context, tx pgx.Tx, settlementID string) (ContractSettlement, error) {
	value, err := scanContractSettlement(tx.QueryRow(ctx, `
		SELECT `+contractSettlementColumns+`
		FROM contract_settlements s
		WHERE s.id = $1
	`, settlementID))
	if err != nil {
		return ContractSettlement{}, fmt.Errorf("query contract settlement: %w", err)
	}
	value.ConfirmingParties, err = listContractSettlementParties(ctx, tx, settlementID)
	if err != nil {
		return ContractSettlement{}, err
	}
	return value, nil
}

func listContractSettlements(ctx context.Context, tx pgx.Tx, contractID string) ([]ContractSettlement, error) {
	rows, err := tx.Query(ctx, `
		SELECT `+contractSettlementColumns+`
		FROM contract_settlements s
		WHERE s.contract_id = $1
		ORDER BY s.settlement_sequence
	`, contractID)
	if err != nil {
		return nil, fmt.Errorf("query contract settlements: %w", err)
	}
	defer rows.Close()
	values := make([]ContractSettlement, 0)
	for rows.Next() {
		value, err := scanContractSettlement(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contract settlements: %w", err)
	}
	rows.Close()
	for index := range values {
		values[index].ConfirmingParties, err = listContractSettlementParties(ctx, tx, values[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return values, nil
}

func scanContractSettlement(row rowScanner) (ContractSettlement, error) {
	var value ContractSettlement
	if err := row.Scan(
		&value.ID, &value.ContractID, &value.SourceRevision, &value.Sequence, &value.Kind,
		&value.StatementNumber, &value.Title, &value.Amount, &value.Currency,
		&value.ConfirmedOn, &value.Basis,
		&value.Actor.Kind, &value.Actor.ID, &value.Actor.DisplayName, &value.RecordedAt,
	); err != nil {
		return ContractSettlement{}, err
	}
	amount, err := normalizeSettlementAmount(value.Amount)
	if err != nil {
		return ContractSettlement{}, fmt.Errorf("normalize stored settlement amount: %w", err)
	}
	value.Amount = amount
	return value, nil
}

func listContractSettlementParties(ctx context.Context, tx pgx.Tx, settlementID string) ([]ContractSettlementPartySnapshot, error) {
	rows, err := tx.Query(ctx, `
		SELECT settlement_id::text, position, source_party_link_id::text, source_party_id::text,
			source_party_revision, party_kind, legal_name,
			COALESCE(registration_identifier, ''), role
		FROM contract_settlement_parties
		WHERE settlement_id = $1
		ORDER BY position
	`, settlementID)
	if err != nil {
		return nil, fmt.Errorf("query settlement confirming parties: %w", err)
	}
	defer rows.Close()
	values := make([]ContractSettlementPartySnapshot, 0)
	for rows.Next() {
		var value ContractSettlementPartySnapshot
		if err := rows.Scan(
			&value.SettlementID, &value.Position, &value.SourcePartyLinkID, &value.SourcePartyID,
			&value.SourcePartyRevision, &value.PartyKind, &value.LegalName,
			&value.RegistrationIdentifier, &value.Role,
		); err != nil {
			return nil, fmt.Errorf("scan settlement confirming party: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settlement confirming parties: %w", err)
	}
	return values, nil
}
