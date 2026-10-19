package contract

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"omsbizfoundry/contract-ledger/internal/audit"
)

const contractRevisionColumns = `
	contract_id::text,
	revision,
	COALESCE(activation_command_id::text, ''),
	COALESCE(source_draft_version, 0),
	source_instrument_id::text,
	number_namespace,
	contract_number,
	title,
	COALESCE(category, ''),
	currency,
	COALESCE(total_amount::text, ''),
	to_char(signed_on, 'YYYY-MM-DD'),
	to_char(effective_on, 'YYYY-MM-DD'),
	actor_kind,
	actor_id,
	COALESCE((
		SELECT i.actor_display_name
		FROM contract_instruments i
		WHERE i.id = contract_revisions.source_instrument_id
	), actor_id),
	recorded_at
`

const contractPartySnapshotColumns = `
	contract_id::text,
	revision,
	position,
	source_party_link_id::text,
	source_party_id::text,
	source_party_revision,
	party_kind,
	legal_name,
	COALESCE(registration_identifier, ''),
	role
`

const contractAuditColumns = `
	id::text,
	object_id::text,
	action,
	COALESCE(command_id::text, ''),
	actor_kind,
	actor_id,
	actor_display_name,
	occurred_at,
	COALESCE((details->>'source_draft_version')::bigint, 0),
	COALESCE((details->>'resulting_revision')::bigint, 0),
	summary
`

func (r *PostgresRepository) Activate(ctx context.Context, command ActivationCommand) (ActivationCommand, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ActivationCommand{}, false, fmt.Errorf("begin contract activation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if err := lockActivationCommandID(ctx, tx, command.CommandID); err != nil {
		return ActivationCommand{}, false, err
	}
	if existing, found, err := readActivationCommand(ctx, tx, command.CommandID); err != nil {
		return ActivationCommand{}, false, err
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
		return ActivationCommand{}, false, &ContractNotFoundError{ContractID: command.ContractID}
	}
	if err != nil {
		return ActivationCommand{}, false, fmt.Errorf("lock contract for activation: %w", err)
	}
	if current.Status != StatusDraft {
		return ActivationCommand{}, false, &ContractStateConflictError{Status: current.Status}
	}
	if current.DraftVersion != command.ExpectedDraftVersion {
		return ActivationCommand{}, false, &DraftVersionConflictError{Expected: command.ExpectedDraftVersion, Actual: current.DraftVersion}
	}
	if current.SignedOn == "" {
		return ActivationCommand{}, false, &ActivationPreconditionError{Field: "signed_on", Message: "激活前必须填写签署日期"}
	}
	if current.EffectiveOn == "" {
		return ActivationCommand{}, false, &ActivationPreconditionError{Field: "effective_on", Message: "激活前必须填写生效日期"}
	}

	parties, err := lockActivationParties(ctx, tx, command.ContractID)
	if err != nil {
		return ActivationCommand{}, false, err
	}
	distinctParties := make(map[string]struct{}, len(parties))
	for _, linked := range parties {
		distinctParties[linked.PartyID] = struct{}{}
	}
	if len(distinctParties) < 2 {
		return ActivationCommand{}, false, &ActivationPreconditionError{
			Field:   "parties",
			Message: "激活前至少需要两个不同的签约主体",
		}
	}

	command.ResultingRevision = 1
	if err := insertActivationCommand(ctx, tx, command); err != nil {
		return ActivationCommand{}, false, err
	}
	if err := insertOriginalContractInstrument(ctx, tx, command, current); err != nil {
		return ActivationCommand{}, false, err
	}
	if err := insertContractRevision(ctx, tx, command, current); err != nil {
		return ActivationCommand{}, false, err
	}
	for index, linked := range parties {
		if err := insertContractPartySnapshot(ctx, tx, command, linked, int64(index+1)); err != nil {
			return ActivationCommand{}, false, err
		}
	}
	result, err := tx.Exec(ctx, `
		UPDATE contracts
		SET status = $2,
			current_revision = $3,
			updated_at = $4
		WHERE id = $1 AND status = $5 AND draft_version = $6
	`, command.ContractID, StatusActive, command.ResultingRevision, command.OccurredAt, StatusDraft, command.ExpectedDraftVersion)
	if err != nil {
		return ActivationCommand{}, false, fmt.Errorf("mark contract active: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ActivationCommand{}, false, fmt.Errorf("mark contract active: expected one row, updated %d", result.RowsAffected())
	}
	if err := insertActivationAudit(ctx, tx, command); err != nil {
		return ActivationCommand{}, false, err
	}
	if err := insertApplicationActivationAudit(ctx, tx, command); err != nil {
		return ActivationCommand{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ActivationCommand{}, false, fmt.Errorf("commit contract activation: %w", err)
	}
	return command, true, nil
}

func lockActivationCommandID(ctx context.Context, tx pgx.Tx, commandID string) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, commandID); err != nil {
		return fmt.Errorf("lock activation command ID: %w", err)
	}
	return nil
}

func readActivationCommand(ctx context.Context, tx pgx.Tx, commandID string) (ActivationCommand, bool, error) {
	var value ActivationCommand
	err := tx.QueryRow(ctx, `
		SELECT c.command_id::text, i.id::text, c.contract_id::text, c.expected_draft_version,
			c.actor_kind, c.actor_id,
			COALESCE(a.actor_display_name, c.actor_id),
			c.resulting_revision, c.occurred_at
		FROM contract_activation_commands c
		LEFT JOIN audit_entries a
			ON a.action = 'contract_activated' AND a.command_id = c.command_id
		JOIN contract_instruments i
			ON i.source_command_id = c.command_id AND i.instrument_type = 'original_contract'
		WHERE c.command_id = $1
	`, commandID).Scan(
		&value.CommandID,
		&value.InstrumentID,
		&value.ContractID,
		&value.ExpectedDraftVersion,
		&value.Actor.Kind,
		&value.Actor.ID,
		&value.Actor.DisplayName,
		&value.ResultingRevision,
		&value.OccurredAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ActivationCommand{}, false, nil
	}
	if err != nil {
		return ActivationCommand{}, false, fmt.Errorf("query activation command: %w", err)
	}
	return value, true, nil
}

func insertOriginalContractInstrument(ctx context.Context, tx pgx.Tx, command ActivationCommand, current Contract) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_instruments (
			id, contract_id, instrument_type, instrument_number, title,
			change_summary, signed_on, effective_on, effective_order,
			amount_effect, amount_value, source_command_id, resulting_revision,
			actor_kind, actor_id, actor_display_name, recorded_at
		)
		VALUES (
			$1, $2, 'original_contract', $3, $4,
			'原合同激活', $5::date, $6::date, 1,
			'original', NULLIF($7, '')::numeric, $8, 1,
			$9, $10, $11, $12
		)
	`,
		command.InstrumentID,
		command.ContractID,
		current.ContractNumber,
		current.Title,
		current.SignedOn,
		current.EffectiveOn,
		current.TotalAmount,
		command.CommandID,
		command.Actor.Kind,
		command.Actor.ID,
		command.Actor.DisplayName,
		command.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("insert original contract instrument: %w", err)
	}
	return nil
}

func lockActivationParties(ctx context.Context, tx pgx.Tx, contractID string) ([]DraftParty, error) {
	rows, err := tx.Query(ctx, `
		SELECT `+draftPartyColumns+`
		FROM contract_draft_parties dp
		JOIN parties p ON p.id = dp.party_id
		WHERE dp.contract_id = $1 AND dp.removed_at IS NULL
		ORDER BY dp.added_at, dp.id
		FOR SHARE OF dp, p
	`, contractID)
	if err != nil {
		return nil, fmt.Errorf("lock activation parties: %w", err)
	}
	defer rows.Close()
	parties := make([]DraftParty, 0)
	for rows.Next() {
		linked, err := scanDraftParty(rows)
		if err != nil {
			return nil, err
		}
		parties = append(parties, linked)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate activation parties: %w", err)
	}
	return parties, nil
}

func insertActivationCommand(ctx context.Context, tx pgx.Tx, command ActivationCommand) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_activation_commands (
			command_id, contract_id, expected_draft_version,
			actor_kind, actor_id, resulting_revision, occurred_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`,
		command.CommandID,
		command.ContractID,
		command.ExpectedDraftVersion,
		command.Actor.Kind,
		command.Actor.ID,
		command.ResultingRevision,
		command.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("insert activation command: %w", err)
	}
	return nil
}

func insertContractRevision(ctx context.Context, tx pgx.Tx, command ActivationCommand, current Contract) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_revisions (
			contract_id, revision, activation_command_id, source_draft_version, source_instrument_id,
			number_namespace, contract_number, title, category, currency, total_amount,
			signed_on, effective_on, actor_kind, actor_id, recorded_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, ''), $10, NULLIF($11, '')::numeric,
			$12::date, $13::date, $14, $15, $16
		)
	`,
		command.ContractID,
		command.ResultingRevision,
		command.CommandID,
		command.ExpectedDraftVersion,
		command.InstrumentID,
		current.NumberNamespace,
		current.ContractNumber,
		current.Title,
		current.Category,
		current.Currency,
		current.TotalAmount,
		current.SignedOn,
		current.EffectiveOn,
		command.Actor.Kind,
		command.Actor.ID,
		command.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("insert contract revision 1: %w", err)
	}
	return nil
}

func insertContractPartySnapshot(ctx context.Context, tx pgx.Tx, command ActivationCommand, linked DraftParty, position int64) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_revision_parties (
			contract_id, revision, position, source_party_link_id,
			source_party_id, source_party_revision, party_kind,
			legal_name, registration_identifier, role
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, ''), $10)
	`,
		command.ContractID,
		command.ResultingRevision,
		position,
		linked.ID,
		linked.PartyID,
		linked.PartyRevision,
		linked.PartyKind,
		linked.PartyLegalName,
		linked.RegistrationIdentifier,
		linked.Role,
	)
	if err != nil {
		return fmt.Errorf("insert contract party snapshot: %w", err)
	}
	return nil
}

func insertActivationAudit(ctx context.Context, tx pgx.Tx, command ActivationCommand) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_audit_entries (
			id, contract_id, event_type, command_id, actor_kind, actor_id,
			occurred_at, source_draft_version, resulting_revision
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`,
		command.AuditID,
		command.ContractID,
		AuditEventContractActivated,
		command.CommandID,
		command.Actor.Kind,
		command.Actor.ID,
		command.OccurredAt,
		command.ExpectedDraftVersion,
		command.ResultingRevision,
	)
	if err != nil {
		return fmt.Errorf("insert contract activation audit: %w", err)
	}
	return nil
}

func insertApplicationActivationAudit(ctx context.Context, tx pgx.Tx, command ActivationCommand) error {
	return audit.Insert(ctx, tx, audit.Entry{
		ID:         command.AuditID,
		ObjectType: audit.ObjectContract,
		ObjectID:   command.ContractID,
		Action:     AuditEventContractActivated,
		CommandID:  command.CommandID,
		Actor:      command.Actor,
		OccurredAt: command.OccurredAt,
		Summary: fmt.Sprintf(
			"激活合同：草稿版本 %d → revision %d",
			command.ExpectedDraftVersion,
			command.ResultingRevision,
		),
		Details: map[string]any{
			"source_draft_version": command.ExpectedDraftVersion,
			"resulting_revision":   command.ResultingRevision,
		},
	})
}

func loadContractRevisionDetails(
	ctx context.Context,
	tx pgx.Tx,
	contractID string,
	revisionNumber int64,
) (ContractRevision, []ContractPartySnapshot, []ContractAuditEntry, error) {
	revision, err := scanContractRevision(tx.QueryRow(ctx, `
		SELECT `+contractRevisionColumns+`
		FROM contract_revisions
		WHERE contract_id = $1 AND revision = $2
	`, contractID, revisionNumber))
	if err != nil {
		return ContractRevision{}, nil, nil, fmt.Errorf("query contract revision: %w", err)
	}

	partyRows, err := tx.Query(ctx, `
		SELECT `+contractPartySnapshotColumns+`
		FROM contract_revision_parties
		WHERE contract_id = $1 AND revision = $2
		ORDER BY position
	`, contractID, revisionNumber)
	if err != nil {
		return ContractRevision{}, nil, nil, fmt.Errorf("query contract party snapshots: %w", err)
	}
	partySnapshots := make([]ContractPartySnapshot, 0)
	for partyRows.Next() {
		snapshot, err := scanContractPartySnapshot(partyRows)
		if err != nil {
			partyRows.Close()
			return ContractRevision{}, nil, nil, err
		}
		partySnapshots = append(partySnapshots, snapshot)
	}
	if err := partyRows.Err(); err != nil {
		partyRows.Close()
		return ContractRevision{}, nil, nil, fmt.Errorf("iterate contract party snapshots: %w", err)
	}
	partyRows.Close()

	auditEntries, err := loadContractAuditEntries(ctx, tx, contractID)
	if err != nil {
		return ContractRevision{}, nil, nil, err
	}
	return revision, partySnapshots, auditEntries, nil
}

func loadContractAuditEntries(ctx context.Context, tx pgx.Tx, contractID string) ([]ContractAuditEntry, error) {
	auditRows, err := tx.Query(ctx, `
		SELECT `+contractAuditColumns+`
		FROM audit_entries
		WHERE object_type = 'contract' AND object_id = $1
		ORDER BY occurred_at, id
	`, contractID)
	if err != nil {
		return nil, fmt.Errorf("query contract audit entries: %w", err)
	}
	auditEntries := make([]ContractAuditEntry, 0)
	for auditRows.Next() {
		entry, err := scanContractAuditEntry(auditRows)
		if err != nil {
			auditRows.Close()
			return nil, err
		}
		auditEntries = append(auditEntries, entry)
	}
	if err := auditRows.Err(); err != nil {
		auditRows.Close()
		return nil, fmt.Errorf("iterate contract audit entries: %w", err)
	}
	auditRows.Close()
	return auditEntries, nil
}

func scanContractRevision(row rowScanner) (ContractRevision, error) {
	var value ContractRevision
	if err := row.Scan(
		&value.ContractID,
		&value.Revision,
		&value.ActivationCommandID,
		&value.SourceDraftVersion,
		&value.SourceInstrumentID,
		&value.NumberNamespace,
		&value.ContractNumber,
		&value.Title,
		&value.Category,
		&value.Currency,
		&value.TotalAmount,
		&value.SignedOn,
		&value.EffectiveOn,
		&value.Actor.Kind,
		&value.Actor.ID,
		&value.Actor.DisplayName,
		&value.RecordedAt,
	); err != nil {
		return ContractRevision{}, err
	}
	amount, err := normalizeAmount(value.TotalAmount)
	if err != nil {
		return ContractRevision{}, fmt.Errorf("normalize stored revision amount: %w", err)
	}
	value.TotalAmount = amount
	return value, nil
}

func scanContractPartySnapshot(row rowScanner) (ContractPartySnapshot, error) {
	var value ContractPartySnapshot
	if err := row.Scan(
		&value.ContractID,
		&value.Revision,
		&value.Position,
		&value.SourcePartyLinkID,
		&value.SourcePartyID,
		&value.SourcePartyRevision,
		&value.PartyKind,
		&value.LegalName,
		&value.RegistrationIdentifier,
		&value.Role,
	); err != nil {
		return ContractPartySnapshot{}, err
	}
	return value, nil
}

func scanContractAuditEntry(row rowScanner) (ContractAuditEntry, error) {
	var value ContractAuditEntry
	if err := row.Scan(
		&value.ID,
		&value.ContractID,
		&value.EventType,
		&value.CommandID,
		&value.Actor.Kind,
		&value.Actor.ID,
		&value.Actor.DisplayName,
		&value.OccurredAt,
		&value.SourceDraftVersion,
		&value.ResultingRevision,
		&value.Summary,
	); err != nil {
		return ContractAuditEntry{}, err
	}
	return value, nil
}
