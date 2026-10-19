package contract

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"omsbizfoundry/contract-ledger/internal/audit"
)

const contractClosureColumns = `
	c.id::text,
	c.contract_id::text,
	c.source_revision,
	c.source_settlement_sequence,
	c.settlement_disposition,
	COALESCE(c.final_settlement_id::text, ''),
	to_char(c.closed_on, 'YYYY-MM-DD'),
	c.reason,
	c.actor_kind,
	c.actor_id,
	c.actor_display_name,
	c.recorded_at
`

func (r *PostgresRepository) CloseContract(
	ctx context.Context,
	command ContractClosureCommand,
) (ContractClosureCommand, ContractClosure, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ContractClosureCommand{}, ContractClosure{}, false, fmt.Errorf("begin contract closure: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if err := lockActivationCommandID(ctx, tx, command.CommandID); err != nil {
		return ContractClosureCommand{}, ContractClosure{}, false, err
	}
	if existingCommand, closure, found, err := readContractClosureCommand(ctx, tx, command.CommandID); err != nil {
		return ContractClosureCommand{}, ContractClosure{}, false, err
	} else if found {
		return existingCommand, closure, false, nil
	}

	current, err := scanContract(tx.QueryRow(ctx, `
		SELECT `+contractColumns+`
		FROM contracts
		WHERE id = $1
		FOR UPDATE
	`, command.ContractID))
	if errors.Is(err, pgx.ErrNoRows) {
		return ContractClosureCommand{}, ContractClosure{}, false, &ContractNotFoundError{ContractID: command.ContractID}
	}
	if err != nil {
		return ContractClosureCommand{}, ContractClosure{}, false, fmt.Errorf("lock contract for closure: %w", err)
	}
	if current.Status != StatusActive && current.Status != StatusTerminated {
		return ContractClosureCommand{}, ContractClosure{}, false, &ContractClosureStateConflictError{Status: current.Status}
	}
	if current.CurrentRevision != command.ExpectedContractRevision {
		return ContractClosureCommand{}, ContractClosure{}, false, &ContractRevisionConflictError{
			Expected: command.ExpectedContractRevision,
			Actual:   current.CurrentRevision,
		}
	}

	var latestSettlementID, latestSettlementConfirmedOn string
	var latestSettlementSequence int64
	var latestSettlementKind SettlementKind
	err = tx.QueryRow(ctx, `
		SELECT id::text, settlement_sequence, settlement_kind,
			to_char(confirmed_on, 'YYYY-MM-DD')
		FROM contract_settlements
		WHERE contract_id = $1
		ORDER BY settlement_sequence DESC
		LIMIT 1
	`, command.ContractID).Scan(
		&latestSettlementID,
		&latestSettlementSequence,
		&latestSettlementKind,
		&latestSettlementConfirmedOn,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		latestSettlementSequence = 0
		latestSettlementKind = ""
		latestSettlementID = ""
		latestSettlementConfirmedOn = ""
	} else if err != nil {
		return ContractClosureCommand{}, ContractClosure{}, false, fmt.Errorf("read latest settlement for closure: %w", err)
	}
	if latestSettlementSequence != command.ExpectedSettlementSequence {
		return ContractClosureCommand{}, ContractClosure{}, false, &ContractSettlementSequenceConflictError{
			Expected: command.ExpectedSettlementSequence,
			Actual:   latestSettlementSequence,
		}
	}
	switch command.SettlementDisposition {
	case SettlementDispositionFinalRecorded:
		if latestSettlementKind != SettlementKindFinal {
			return ContractClosureCommand{}, ContractClosure{}, false, &ContractClosureSettlementConflictError{
				Message: "当前没有最终结算事实，不能按已登记最终结算关闭",
			}
		}
		command.FinalSettlementID = latestSettlementID
	case SettlementDispositionNotRequired:
		if latestSettlementKind == SettlementKindFinal {
			return ContractClosureCommand{}, ContractClosure{}, false, &ContractClosureSettlementConflictError{
				Message: "当前已存在最终结算事实，必须按已登记最终结算关闭",
			}
		}
	default:
		return ContractClosureCommand{}, ContractClosure{}, false, &ContractClosureSettlementConflictError{Message: "结算处置无效"}
	}

	var currentInstrumentEffectiveOn string
	if err := tx.QueryRow(ctx, `
		SELECT to_char(i.effective_on, 'YYYY-MM-DD')
		FROM contract_revisions r
		JOIN contract_instruments i ON i.id = r.source_instrument_id
		WHERE r.contract_id = $1 AND r.revision = $2
		FOR SHARE OF r, i
	`, command.ContractID, command.ExpectedContractRevision).Scan(&currentInstrumentEffectiveOn); err != nil {
		return ContractClosureCommand{}, ContractClosure{}, false, fmt.Errorf("read current instrument date for closure: %w", err)
	}
	if command.ClosedOn < currentInstrumentEffectiveOn {
		return ContractClosureCommand{}, ContractClosure{}, false, &ContractClosurePreconditionError{
			Field:   "closed_on",
			Message: fmt.Sprintf("关闭日期不能早于当前文书生效日期 %s", currentInstrumentEffectiveOn),
		}
	}
	if latestSettlementConfirmedOn != "" && command.ClosedOn < latestSettlementConfirmedOn {
		return ContractClosureCommand{}, ContractClosure{}, false, &ContractClosurePreconditionError{
			Field:   "closed_on",
			Message: fmt.Sprintf("关闭日期不能早于最新结算确认日期 %s", latestSettlementConfirmedOn),
		}
	}

	if err := insertContractClosure(ctx, tx, command); err != nil {
		return ContractClosureCommand{}, ContractClosure{}, false, err
	}
	if err := insertContractClosureCommand(ctx, tx, command); err != nil {
		return ContractClosureCommand{}, ContractClosure{}, false, err
	}
	result, err := tx.Exec(ctx, `
		UPDATE contracts
		SET status = 'closed', updated_at = $4
		WHERE id = $1 AND status = $2 AND current_revision = $3
	`, command.ContractID, current.Status, command.ExpectedContractRevision, command.OccurredAt)
	if err != nil {
		return ContractClosureCommand{}, ContractClosure{}, false, fmt.Errorf("mark contract closed: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ContractClosureCommand{}, ContractClosure{}, false, &ContractClosureStateConflictError{Status: current.Status}
	}
	if err := insertContractClosureAudit(ctx, tx, command); err != nil {
		return ContractClosureCommand{}, ContractClosure{}, false, err
	}
	closure, err := getContractClosure(ctx, tx, command.ClosureID)
	if err != nil {
		return ContractClosureCommand{}, ContractClosure{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ContractClosureCommand{}, ContractClosure{}, false, fmt.Errorf("commit contract closure: %w", err)
	}
	return command, closure, true, nil
}

func readContractClosureCommand(
	ctx context.Context,
	tx pgx.Tx,
	commandID string,
) (ContractClosureCommand, ContractClosure, bool, error) {
	var value ContractClosureCommand
	err := tx.QueryRow(ctx, `
		SELECT c.command_id::text, c.closure_id::text, c.contract_id::text,
			c.expected_contract_revision, c.expected_settlement_sequence,
			c.settlement_disposition, to_char(c.closed_on, 'YYYY-MM-DD'), c.reason,
			c.actor_kind, c.actor_id, c.actor_display_name,
			COALESCE(f.final_settlement_id::text, ''), c.occurred_at
		FROM contract_closure_commands c
		JOIN contract_closures f ON f.id = c.closure_id
		WHERE c.command_id = $1
	`, commandID).Scan(
		&value.CommandID, &value.ClosureID, &value.ContractID,
		&value.ExpectedContractRevision, &value.ExpectedSettlementSequence,
		&value.SettlementDisposition, &value.ClosedOn, &value.Reason,
		&value.Actor.Kind, &value.Actor.ID, &value.Actor.DisplayName,
		&value.FinalSettlementID, &value.OccurredAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ContractClosureCommand{}, ContractClosure{}, false, nil
	}
	if err != nil {
		return ContractClosureCommand{}, ContractClosure{}, false, fmt.Errorf("query contract closure command: %w", err)
	}
	closure, err := getContractClosure(ctx, tx, value.ClosureID)
	if err != nil {
		return ContractClosureCommand{}, ContractClosure{}, false, err
	}
	return value, closure, true, nil
}

func insertContractClosure(ctx context.Context, tx pgx.Tx, command ContractClosureCommand) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_closures (
			id, contract_id, source_revision, source_settlement_sequence,
			settlement_disposition, final_settlement_id,
			closed_on, reason, actor_kind, actor_id, actor_display_name, recorded_at
		)
		VALUES (
			$1, $2, $3, $4,
			$5, NULLIF($6, '')::uuid,
			$7::date, $8, $9, $10, $11, $12
		)
	`, command.ClosureID, command.ContractID, command.ExpectedContractRevision,
		command.ExpectedSettlementSequence, command.SettlementDisposition,
		command.FinalSettlementID, command.ClosedOn, command.Reason,
		command.Actor.Kind, command.Actor.ID, command.Actor.DisplayName, command.OccurredAt)
	if err != nil {
		return fmt.Errorf("insert contract closure: %w", err)
	}
	return nil
}

func insertContractClosureCommand(ctx context.Context, tx pgx.Tx, command ContractClosureCommand) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_closure_commands (
			command_id, closure_id, contract_id,
			expected_contract_revision, expected_settlement_sequence,
			settlement_disposition, closed_on, reason,
			actor_kind, actor_id, actor_display_name, occurred_at
		)
		VALUES (
			$1, $2, $3,
			$4, $5,
			$6, $7::date, $8,
			$9, $10, $11, $12
		)
	`, command.CommandID, command.ClosureID, command.ContractID,
		command.ExpectedContractRevision, command.ExpectedSettlementSequence,
		command.SettlementDisposition, command.ClosedOn, command.Reason,
		command.Actor.Kind, command.Actor.ID, command.Actor.DisplayName, command.OccurredAt)
	if err != nil {
		return fmt.Errorf("insert contract closure command: %w", err)
	}
	return nil
}

func insertContractClosureAudit(ctx context.Context, tx pgx.Tx, command ContractClosureCommand) error {
	return audit.Insert(ctx, tx, audit.Entry{
		ID: command.AuditID, ObjectType: audit.ObjectContract, ObjectID: command.ContractID,
		Action: AuditEventContractClosed, CommandID: command.CommandID,
		Actor: command.Actor, OccurredAt: command.OccurredAt,
		Summary: fmt.Sprintf("操作性关闭合同：%s", command.Reason),
		Details: map[string]any{
			"closure_id":                 command.ClosureID,
			"source_revision":            command.ExpectedContractRevision,
			"source_settlement_sequence": command.ExpectedSettlementSequence,
			"settlement_disposition":     command.SettlementDisposition,
			"final_settlement_id":        command.FinalSettlementID,
			"closed_on":                  command.ClosedOn,
			"reason":                     command.Reason,
		},
	})
}

func getContractClosure(ctx context.Context, tx pgx.Tx, closureID string) (ContractClosure, error) {
	value, err := scanContractClosure(tx.QueryRow(ctx, `
		SELECT `+contractClosureColumns+`
		FROM contract_closures c
		WHERE c.id = $1
	`, closureID))
	if err != nil {
		return ContractClosure{}, fmt.Errorf("query contract closure: %w", err)
	}
	return value, nil
}

func findContractClosure(ctx context.Context, tx pgx.Tx, contractID string) (ContractClosure, bool, error) {
	value, err := scanContractClosure(tx.QueryRow(ctx, `
		SELECT `+contractClosureColumns+`
		FROM contract_closures c
		WHERE c.contract_id = $1
	`, contractID))
	if errors.Is(err, pgx.ErrNoRows) {
		return ContractClosure{}, false, nil
	}
	if err != nil {
		return ContractClosure{}, false, fmt.Errorf("query contract closure by contract: %w", err)
	}
	return value, true, nil
}

func scanContractClosure(row rowScanner) (ContractClosure, error) {
	var value ContractClosure
	if err := row.Scan(
		&value.ID, &value.ContractID, &value.SourceRevision,
		&value.SourceSettlementSequence, &value.SettlementDisposition,
		&value.FinalSettlementID, &value.ClosedOn, &value.Reason,
		&value.Actor.Kind, &value.Actor.ID, &value.Actor.DisplayName, &value.RecordedAt,
	); err != nil {
		return ContractClosure{}, err
	}
	return value, nil
}
