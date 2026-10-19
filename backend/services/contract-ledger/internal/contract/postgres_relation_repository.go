package contract

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"omsbizfoundry/contract-ledger/internal/audit"
)

const contractRelationColumns = `
	r.id::text,
	r.relation_type,
	COALESCE(to_char(r.effective_on, 'YYYY-MM-DD'), ''),
	COALESCE(r.note, ''),
	r.actor_kind,
	r.actor_id,
	r.actor_display_name,
	r.created_at,
	sr.contract_id::text,
	sr.revision,
	sr.number_namespace,
	sr.contract_number,
	sr.title,
	tr.contract_id::text,
	tr.revision,
	tr.number_namespace,
	tr.contract_number,
	tr.title
`

func (r *PostgresRepository) LinkRelation(
	ctx context.Context,
	command ContractRelationCommand,
) (ContractRelationCommand, ContractRelation, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ContractRelationCommand{}, ContractRelation{}, false, fmt.Errorf("begin link contract relation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if err := lockActivationCommandID(ctx, tx, command.CommandID); err != nil {
		return ContractRelationCommand{}, ContractRelation{}, false, err
	}
	if existingCommand, existingRelation, found, err := readContractRelationCommand(ctx, tx, command.CommandID); err != nil {
		return ContractRelationCommand{}, ContractRelation{}, false, err
	} else if found {
		return existingCommand, existingRelation, false, nil
	}
	if command.RelationType.Acyclic() {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "contract-relation:"+string(command.RelationType)); err != nil {
			return ContractRelationCommand{}, ContractRelation{}, false, fmt.Errorf("lock contract relation type: %w", err)
		}
	}

	contracts, err := lockRelationContracts(ctx, tx, command.SourceContractID, command.TargetContractID)
	if err != nil {
		return ContractRelationCommand{}, ContractRelation{}, false, err
	}
	if err := checkRelationContract(contracts, "source", command.SourceContractID, command.ExpectedSourceRevision); err != nil {
		return ContractRelationCommand{}, ContractRelation{}, false, err
	}
	if err := checkRelationContract(contracts, "target", command.TargetContractID, command.ExpectedTargetRevision); err != nil {
		return ContractRelationCommand{}, ContractRelation{}, false, err
	}
	if command.RelationType.Acyclic() {
		cycle, err := relationWouldCreateCycle(ctx, tx, command)
		if err != nil {
			return ContractRelationCommand{}, ContractRelation{}, false, err
		}
		if cycle {
			return ContractRelationCommand{}, ContractRelation{}, false, &ContractRelationCycleError{}
		}
	}
	if err := insertContractRelation(ctx, tx, command); err != nil {
		return ContractRelationCommand{}, ContractRelation{}, false, err
	}
	if err := insertContractRelationCommand(ctx, tx, command); err != nil {
		return ContractRelationCommand{}, ContractRelation{}, false, err
	}
	if err := insertContractRelationAudit(ctx, tx, command); err != nil {
		return ContractRelationCommand{}, ContractRelation{}, false, err
	}
	persisted, err := getContractRelationByID(ctx, tx, command.RelationID)
	if err != nil {
		return ContractRelationCommand{}, ContractRelation{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ContractRelationCommand{}, ContractRelation{}, false, fmt.Errorf("commit contract relation: %w", err)
	}
	return command, persisted, true, nil
}

type relationContractState struct {
	Status   Status
	Revision int64
}

func lockRelationContracts(
	ctx context.Context,
	tx pgx.Tx,
	sourceContractID string,
	targetContractID string,
) (map[string]relationContractState, error) {
	rows, err := tx.Query(ctx, `
		SELECT id::text, status, COALESCE(current_revision, 0)
		FROM contracts
		WHERE id IN ($1, $2)
		ORDER BY id
		FOR UPDATE
	`, sourceContractID, targetContractID)
	if err != nil {
		return nil, fmt.Errorf("lock relation contracts: %w", err)
	}
	defer rows.Close()
	values := make(map[string]relationContractState, 2)
	for rows.Next() {
		var id string
		var state relationContractState
		if err := rows.Scan(&id, &state.Status, &state.Revision); err != nil {
			return nil, fmt.Errorf("scan relation contract: %w", err)
		}
		values[id] = state
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate relation contracts: %w", err)
	}
	return values, nil
}

func checkRelationContract(
	contracts map[string]relationContractState,
	side string,
	contractID string,
	expectedRevision int64,
) error {
	current, found := contracts[contractID]
	if !found {
		return &ContractNotFoundError{ContractID: contractID}
	}
	switch current.Status {
	case StatusActive, StatusTerminated, StatusClosed:
	default:
		return &ContractRelationStateConflictError{Side: side, Status: current.Status}
	}
	if current.Revision != expectedRevision {
		return &ContractRelationRevisionConflictError{Side: side, Expected: expectedRevision, Actual: current.Revision}
	}
	return nil
}

func relationWouldCreateCycle(ctx context.Context, tx pgx.Tx, command ContractRelationCommand) (bool, error) {
	var cycle bool
	err := tx.QueryRow(ctx, `
		WITH RECURSIVE reachable(contract_id) AS (
			SELECT target_contract_id
			FROM contract_relations
			WHERE source_contract_id = $1 AND relation_type = $3
			UNION
			SELECT relation.target_contract_id
			FROM contract_relations relation
			JOIN reachable ON relation.source_contract_id = reachable.contract_id
			WHERE relation.relation_type = $3
		)
		SELECT EXISTS (
			SELECT 1 FROM reachable WHERE contract_id = $2
		)
	`, command.TargetContractID, command.SourceContractID, string(command.RelationType)).Scan(&cycle)
	if err != nil {
		return false, fmt.Errorf("check contract relation cycle: %w", err)
	}
	return cycle, nil
}

func insertContractRelation(ctx context.Context, tx pgx.Tx, command ContractRelationCommand) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_relations (
			id, source_contract_id, target_contract_id, relation_type,
			source_revision, target_revision, effective_on, note,
			actor_kind, actor_id, actor_display_name, created_at
		)
		VALUES (
			$1, $2, $3, $4,
			$5, $6, NULLIF($7, '')::date, NULLIF($8, ''),
			$9, $10, $11, $12
		)
	`,
		command.RelationID,
		command.SourceContractID,
		command.TargetContractID,
		command.RelationType,
		command.ExpectedSourceRevision,
		command.ExpectedTargetRevision,
		command.EffectiveOn,
		command.Note,
		command.Actor.Kind,
		command.Actor.ID,
		command.Actor.DisplayName,
		command.OccurredAt,
	)
	if err == nil {
		return nil
	}
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && databaseError.ConstraintName == "contract_relations_unique_edge" {
		return &DuplicateContractRelationError{}
	}
	return fmt.Errorf("insert contract relation: %w", err)
}

func insertContractRelationCommand(ctx context.Context, tx pgx.Tx, command ContractRelationCommand) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_relation_commands (
			command_id, relation_id, source_contract_id, target_contract_id,
			relation_type, expected_source_revision, expected_target_revision,
			effective_on, note, actor_kind, actor_id, actor_display_name, occurred_at
		)
		VALUES (
			$1, $2, $3, $4,
			$5, $6, $7,
			NULLIF($8, '')::date, NULLIF($9, ''), $10, $11, $12, $13
		)
	`,
		command.CommandID,
		command.RelationID,
		command.SourceContractID,
		command.TargetContractID,
		command.RelationType,
		command.ExpectedSourceRevision,
		command.ExpectedTargetRevision,
		command.EffectiveOn,
		command.Note,
		command.Actor.Kind,
		command.Actor.ID,
		command.Actor.DisplayName,
		command.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("insert contract relation command: %w", err)
	}
	return nil
}

func insertContractRelationAudit(ctx context.Context, tx pgx.Tx, command ContractRelationCommand) error {
	return audit.Insert(ctx, tx, audit.Entry{
		ID: command.AuditID, ObjectType: audit.ObjectContract, ObjectID: command.SourceContractID,
		Action: AuditActionContractRelationLinked, CommandID: command.CommandID,
		Actor: command.Actor, OccurredAt: command.OccurredAt,
		Summary: fmt.Sprintf(
			"建立合同关系（%s）：source revision %d → target revision %d",
			command.RelationType.Label(),
			command.ExpectedSourceRevision,
			command.ExpectedTargetRevision,
		),
		Details: map[string]any{
			"relation_id":        command.RelationID,
			"target_contract_id": command.TargetContractID,
			"relation_type":      command.RelationType,
			"source_revision":    command.ExpectedSourceRevision,
			"target_revision":    command.ExpectedTargetRevision,
			"effective_on":       command.EffectiveOn,
			"note":               command.Note,
		},
	})
}

func readContractRelationCommand(
	ctx context.Context,
	tx pgx.Tx,
	commandID string,
) (ContractRelationCommand, ContractRelation, bool, error) {
	var command ContractRelationCommand
	err := tx.QueryRow(ctx, `
		SELECT command_id::text, relation_id::text, source_contract_id::text,
			target_contract_id::text, relation_type,
			expected_source_revision, expected_target_revision,
			COALESCE(to_char(effective_on, 'YYYY-MM-DD'), ''), COALESCE(note, ''),
			actor_kind, actor_id, actor_display_name, occurred_at
		FROM contract_relation_commands
		WHERE command_id = $1
	`, commandID).Scan(
		&command.CommandID,
		&command.RelationID,
		&command.SourceContractID,
		&command.TargetContractID,
		&command.RelationType,
		&command.ExpectedSourceRevision,
		&command.ExpectedTargetRevision,
		&command.EffectiveOn,
		&command.Note,
		&command.Actor.Kind,
		&command.Actor.ID,
		&command.Actor.DisplayName,
		&command.OccurredAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ContractRelationCommand{}, ContractRelation{}, false, nil
	}
	if err != nil {
		return ContractRelationCommand{}, ContractRelation{}, false, fmt.Errorf("query contract relation command: %w", err)
	}
	relation, err := getContractRelationByID(ctx, tx, command.RelationID)
	if err != nil {
		return ContractRelationCommand{}, ContractRelation{}, false, err
	}
	return command, relation, true, nil
}

func getContractRelationByID(ctx context.Context, tx pgx.Tx, relationID string) (ContractRelation, error) {
	return scanContractRelation(tx.QueryRow(ctx, `
		SELECT `+contractRelationColumns+`
		FROM contract_relations r
		JOIN contract_revisions sr
			ON sr.contract_id = r.source_contract_id AND sr.revision = r.source_revision
		JOIN contract_revisions tr
			ON tr.contract_id = r.target_contract_id AND tr.revision = r.target_revision
		WHERE r.id = $1
	`, relationID))
}

func listContractRelations(
	ctx context.Context,
	tx pgx.Tx,
	contractID string,
	outgoing bool,
) ([]ContractRelation, error) {
	column := "r.target_contract_id"
	if outgoing {
		column = "r.source_contract_id"
	}
	rows, err := tx.Query(ctx, `
		SELECT `+contractRelationColumns+`
		FROM contract_relations r
		JOIN contract_revisions sr
			ON sr.contract_id = r.source_contract_id AND sr.revision = r.source_revision
		JOIN contract_revisions tr
			ON tr.contract_id = r.target_contract_id AND tr.revision = r.target_revision
		WHERE `+column+` = $1
		ORDER BY r.created_at, r.id
	`, contractID)
	if err != nil {
		return nil, fmt.Errorf("query contract relations: %w", err)
	}
	defer rows.Close()
	values := make([]ContractRelation, 0)
	for rows.Next() {
		value, err := scanContractRelation(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contract relations: %w", err)
	}
	return values, nil
}

func scanContractRelation(row rowScanner) (ContractRelation, error) {
	var value ContractRelation
	if err := row.Scan(
		&value.ID,
		&value.Type,
		&value.EffectiveOn,
		&value.Note,
		&value.Actor.Kind,
		&value.Actor.ID,
		&value.Actor.DisplayName,
		&value.CreatedAt,
		&value.Source.ContractID,
		&value.Source.Revision,
		&value.Source.NumberNamespace,
		&value.Source.ContractNumber,
		&value.Source.Title,
		&value.Target.ContractID,
		&value.Target.Revision,
		&value.Target.NumberNamespace,
		&value.Target.ContractNumber,
		&value.Target.Title,
	); err != nil {
		return ContractRelation{}, fmt.Errorf("scan contract relation: %w", err)
	}
	return value, nil
}
