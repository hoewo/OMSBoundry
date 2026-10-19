package contract

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"omsbizfoundry/contract-ledger/internal/audit"
)

const draftPartyColumns = `
	dp.id::text,
	dp.contract_id::text,
	dp.party_id::text,
	p.kind,
	p.legal_name,
	COALESCE(p.registration_identifier, ''),
	p.revision,
	dp.role,
	dp.added_at,
	dp.removed_at
`

const draftPartyCommandColumns = `
	command_id::text,
	contract_id::text,
	action,
	party_link_id::text,
	party_id::text,
	role,
	expected_draft_version,
	resulting_draft_version,
	created_at
`

func (r *PostgresRepository) GetDetails(ctx context.Context, contractID string) (DraftDetails, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return DraftDetails{}, false, fmt.Errorf("begin contract details read: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	value, err := scanContract(tx.QueryRow(ctx, `SELECT `+contractColumns+` FROM contracts WHERE id = $1`, contractID))
	if errors.Is(err, pgx.ErrNoRows) {
		return DraftDetails{}, false, nil
	}
	if err != nil {
		return DraftDetails{}, false, fmt.Errorf("query contract: %w", err)
	}
	details := DraftDetails{Contract: value}

	if value.Status == StatusDraft || value.Status == StatusCancelled {
		parties, err := listDraftParties(ctx, tx, contractID)
		if err != nil {
			return DraftDetails{}, false, err
		}
		details.Parties = parties
		if value.Status == StatusCancelled {
			cancelled, err := scanDraftCancellation(tx.QueryRow(ctx, `
				SELECT command_id::text, cancellation_reason, created_at, resulting_draft_version
				FROM contract_draft_change_commands
				WHERE contract_id = $1 AND action = 'cancel'
			`, contractID))
			if err != nil {
				return DraftDetails{}, false, fmt.Errorf("query contract draft cancellation: %w", err)
			}
			details.Cancellation = &cancelled
		}
		auditEntries, err := loadContractAuditEntries(ctx, tx, contractID)
		if err != nil {
			return DraftDetails{}, false, err
		}
		details.AuditEntries = auditEntries
	} else {
		revision, snapshots, auditEntries, err := loadContractRevisionDetails(ctx, tx, contractID, value.CurrentRevision)
		if err != nil {
			return DraftDetails{}, false, err
		}
		details.Revision = &revision
		details.Contract.NumberNamespace = revision.NumberNamespace
		details.Contract.ContractNumber = revision.ContractNumber
		details.Contract.Title = revision.Title
		details.Contract.Category = revision.Category
		details.Contract.Currency = revision.Currency
		details.Contract.TotalAmount = revision.TotalAmount
		details.Contract.SignedOn = revision.SignedOn
		details.Contract.EffectiveOn = revision.EffectiveOn
		details.PartySnapshots = snapshots
		details.AuditEntries = auditEntries
		details.Revisions, err = loadContractRevisions(ctx, tx, contractID)
		if err != nil {
			return DraftDetails{}, false, err
		}
		details.Instruments, err = loadContractInstruments(ctx, tx, contractID)
		if err != nil {
			return DraftDetails{}, false, err
		}
		details.Settlements, err = listContractSettlements(ctx, tx, contractID)
		if err != nil {
			return DraftDetails{}, false, err
		}
		closure, found, err := findContractClosure(ctx, tx, contractID)
		if err != nil {
			return DraftDetails{}, false, err
		}
		if found {
			details.Closure = &closure
		}
	}
	details.OutgoingRelations, err = listContractRelations(ctx, tx, contractID, true)
	if err != nil {
		return DraftDetails{}, false, err
	}
	details.IncomingRelations, err = listContractRelations(ctx, tx, contractID, false)
	if err != nil {
		return DraftDetails{}, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return DraftDetails{}, false, fmt.Errorf("commit contract details read: %w", err)
	}
	return details, true, nil
}

func listDraftParties(ctx context.Context, tx pgx.Tx, contractID string) ([]DraftParty, error) {
	rows, err := tx.Query(ctx, `
		SELECT `+draftPartyColumns+`
		FROM contract_draft_parties dp
		JOIN parties p ON p.id = dp.party_id
		WHERE dp.contract_id = $1 AND dp.removed_at IS NULL
		ORDER BY dp.added_at, dp.id
	`, contractID)
	if err != nil {
		return nil, fmt.Errorf("query contract draft parties: %w", err)
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
		return nil, fmt.Errorf("iterate contract draft parties: %w", err)
	}
	return parties, nil
}

func scanDraftCancellation(row rowScanner) (DraftCancellation, error) {
	var value DraftCancellation
	if err := row.Scan(&value.CommandID, &value.Reason, &value.CancelledAt, &value.DraftVersion); err != nil {
		return DraftCancellation{}, err
	}
	return value, nil
}

func (r *PostgresRepository) AddDraftParty(
	ctx context.Context,
	command DraftPartyCommand,
	candidate DraftParty,
) (DraftPartyCommand, DraftParty, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return DraftPartyCommand{}, DraftParty{}, false, fmt.Errorf("begin add draft party: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if err := lockDraftPartyCommandID(ctx, tx, command.CommandID); err != nil {
		return DraftPartyCommand{}, DraftParty{}, false, err
	}
	if existingCommand, existingParty, found, err := readDraftPartyCommand(ctx, tx, command.CommandID); err != nil {
		return DraftPartyCommand{}, DraftParty{}, false, err
	} else if found {
		return existingCommand, existingParty, false, nil
	}

	if err := lockAndCheckDraft(ctx, tx, command.ContractID, command.ExpectedDraftVersion); err != nil {
		return DraftPartyCommand{}, DraftParty{}, false, err
	}
	var partyExists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM parties WHERE id = $1)`, command.PartyID).Scan(&partyExists); err != nil {
		return DraftPartyCommand{}, DraftParty{}, false, fmt.Errorf("check party: %w", err)
	}
	if !partyExists {
		return DraftPartyCommand{}, DraftParty{}, false, &PartyNotFoundError{PartyID: command.PartyID}
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO contract_draft_parties (id, contract_id, party_id, role, added_at)
		VALUES ($1, $2, $3, $4, $5)
	`, candidate.ID, candidate.ContractID, candidate.PartyID, candidate.Role, candidate.AddedAt)
	if err != nil {
		return DraftPartyCommand{}, DraftParty{}, false, mapDraftPartyInsertError(err, candidate)
	}

	command.ResultingDraftVersion = command.ExpectedDraftVersion + 1
	if err := updateDraftVersion(ctx, tx, command); err != nil {
		return DraftPartyCommand{}, DraftParty{}, false, err
	}
	if err := insertDraftPartyCommand(ctx, tx, command); err != nil {
		return DraftPartyCommand{}, DraftParty{}, false, err
	}
	if err := insertDraftPartyAudit(ctx, tx, command); err != nil {
		return DraftPartyCommand{}, DraftParty{}, false, err
	}
	persisted, err := getDraftPartyByID(ctx, tx, candidate.ID)
	if err != nil {
		return DraftPartyCommand{}, DraftParty{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return DraftPartyCommand{}, DraftParty{}, false, fmt.Errorf("commit add draft party: %w", err)
	}
	return command, persisted, true, nil
}

func (r *PostgresRepository) RemoveDraftParty(
	ctx context.Context,
	command DraftPartyCommand,
) (DraftPartyCommand, DraftParty, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return DraftPartyCommand{}, DraftParty{}, false, fmt.Errorf("begin remove draft party: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if err := lockDraftPartyCommandID(ctx, tx, command.CommandID); err != nil {
		return DraftPartyCommand{}, DraftParty{}, false, err
	}
	if existingCommand, existingParty, found, err := readDraftPartyCommand(ctx, tx, command.CommandID); err != nil {
		return DraftPartyCommand{}, DraftParty{}, false, err
	} else if found {
		return existingCommand, existingParty, false, nil
	}
	if err := lockAndCheckDraft(ctx, tx, command.ContractID, command.ExpectedDraftVersion); err != nil {
		return DraftPartyCommand{}, DraftParty{}, false, err
	}

	linked, err := getActiveDraftPartyForUpdate(ctx, tx, command.ContractID, command.PartyLinkID)
	if errors.Is(err, pgx.ErrNoRows) {
		return DraftPartyCommand{}, DraftParty{}, false, &DraftPartyNotFoundError{PartyLinkID: command.PartyLinkID}
	}
	if err != nil {
		return DraftPartyCommand{}, DraftParty{}, false, err
	}
	command.PartyID = linked.PartyID
	command.Role = linked.Role
	command.ResultingDraftVersion = command.ExpectedDraftVersion + 1

	if _, err := tx.Exec(ctx, `
		UPDATE contract_draft_parties
		SET removed_at = $2
		WHERE id = $1
	`, linked.ID, command.CreatedAt); err != nil {
		return DraftPartyCommand{}, DraftParty{}, false, fmt.Errorf("mark draft party removed: %w", err)
	}
	if err := updateDraftVersion(ctx, tx, command); err != nil {
		return DraftPartyCommand{}, DraftParty{}, false, err
	}
	if err := insertDraftPartyCommand(ctx, tx, command); err != nil {
		return DraftPartyCommand{}, DraftParty{}, false, err
	}
	if err := insertDraftPartyAudit(ctx, tx, command); err != nil {
		return DraftPartyCommand{}, DraftParty{}, false, err
	}
	linked.RemovedAt = command.CreatedAt

	if err := tx.Commit(ctx); err != nil {
		return DraftPartyCommand{}, DraftParty{}, false, fmt.Errorf("commit remove draft party: %w", err)
	}
	return command, linked, true, nil
}

func lockDraftPartyCommandID(ctx context.Context, tx pgx.Tx, commandID string) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, commandID); err != nil {
		return fmt.Errorf("lock draft party command ID: %w", err)
	}
	return nil
}

func lockAndCheckDraft(ctx context.Context, tx pgx.Tx, contractID string, expectedVersion int64) error {
	var status Status
	var actualVersion int64
	err := tx.QueryRow(ctx, `
		SELECT status, draft_version
		FROM contracts
		WHERE id = $1
		FOR UPDATE
	`, contractID).Scan(&status, &actualVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return &ContractNotFoundError{ContractID: contractID}
	}
	if err != nil {
		return fmt.Errorf("lock contract draft: %w", err)
	}
	if status != StatusDraft {
		return &ContractStateConflictError{Status: status}
	}
	if actualVersion != expectedVersion {
		return &DraftVersionConflictError{Expected: expectedVersion, Actual: actualVersion}
	}
	return nil
}

func updateDraftVersion(ctx context.Context, tx pgx.Tx, command DraftPartyCommand) error {
	result, err := tx.Exec(ctx, `
		UPDATE contracts
		SET draft_version = $2, updated_at = $3
		WHERE id = $1 AND draft_version = $4
	`, command.ContractID, command.ResultingDraftVersion, command.CreatedAt, command.ExpectedDraftVersion)
	if err != nil {
		return fmt.Errorf("update contract draft version: %w", err)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("update contract draft version: expected one row, updated %d", result.RowsAffected())
	}
	return nil
}

func insertDraftPartyCommand(ctx context.Context, tx pgx.Tx, command DraftPartyCommand) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_draft_party_commands (
			command_id,
			contract_id,
			action,
			party_link_id,
			party_id,
			role,
			expected_draft_version,
			resulting_draft_version,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`,
		command.CommandID,
		command.ContractID,
		command.Action,
		command.PartyLinkID,
		command.PartyID,
		command.Role,
		command.ExpectedDraftVersion,
		command.ResultingDraftVersion,
		command.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert draft party command: %w", err)
	}
	return nil
}

func insertDraftPartyAudit(ctx context.Context, tx pgx.Tx, command DraftPartyCommand) error {
	action := AuditActionDraftPartyAdded
	verb := "添加"
	if command.Action == DraftPartyActionRemove {
		action = AuditActionDraftPartyRemoved
		verb = "移除"
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
			"%s签约主体角色：%s，草稿版本 %d",
			verb,
			command.Role,
			command.ResultingDraftVersion,
		),
		Details: map[string]any{
			"party_id":                command.PartyID,
			"party_link_id":           command.PartyLinkID,
			"role":                    command.Role,
			"expected_draft_version":  command.ExpectedDraftVersion,
			"resulting_draft_version": command.ResultingDraftVersion,
		},
	})
}

func readDraftPartyCommand(
	ctx context.Context,
	tx pgx.Tx,
	commandID string,
) (DraftPartyCommand, DraftParty, bool, error) {
	command, err := scanDraftPartyCommand(tx.QueryRow(ctx, `
		SELECT `+draftPartyCommandColumns+`
		FROM contract_draft_party_commands
		WHERE command_id = $1
	`, commandID))
	if errors.Is(err, pgx.ErrNoRows) {
		return DraftPartyCommand{}, DraftParty{}, false, nil
	}
	if err != nil {
		return DraftPartyCommand{}, DraftParty{}, false, fmt.Errorf("query draft party command: %w", err)
	}
	linked, err := getDraftPartyByID(ctx, tx, command.PartyLinkID)
	if err != nil {
		return DraftPartyCommand{}, DraftParty{}, false, err
	}
	return command, linked, true, nil
}

func getActiveDraftPartyForUpdate(ctx context.Context, tx pgx.Tx, contractID, linkID string) (DraftParty, error) {
	linked, err := scanDraftParty(tx.QueryRow(ctx, `
		SELECT `+draftPartyColumns+`
		FROM contract_draft_parties dp
		JOIN parties p ON p.id = dp.party_id
		WHERE dp.id = $1 AND dp.contract_id = $2 AND dp.removed_at IS NULL
		FOR UPDATE OF dp
	`, linkID, contractID))
	if err != nil {
		return DraftParty{}, err
	}
	return linked, nil
}

func getDraftPartyByID(ctx context.Context, tx pgx.Tx, linkID string) (DraftParty, error) {
	linked, err := scanDraftParty(tx.QueryRow(ctx, `
		SELECT `+draftPartyColumns+`
		FROM contract_draft_parties dp
		JOIN parties p ON p.id = dp.party_id
		WHERE dp.id = $1
	`, linkID))
	if err != nil {
		return DraftParty{}, fmt.Errorf("query contract draft party: %w", err)
	}
	return linked, nil
}

func scanDraftParty(row rowScanner) (DraftParty, error) {
	var value DraftParty
	var removedAt *time.Time
	if err := row.Scan(
		&value.ID,
		&value.ContractID,
		&value.PartyID,
		&value.PartyKind,
		&value.PartyLegalName,
		&value.RegistrationIdentifier,
		&value.PartyRevision,
		&value.Role,
		&value.AddedAt,
		&removedAt,
	); err != nil {
		return DraftParty{}, err
	}
	if removedAt != nil {
		value.RemovedAt = *removedAt
	}
	return value, nil
}

func scanDraftPartyCommand(row rowScanner) (DraftPartyCommand, error) {
	var value DraftPartyCommand
	if err := row.Scan(
		&value.CommandID,
		&value.ContractID,
		&value.Action,
		&value.PartyLinkID,
		&value.PartyID,
		&value.Role,
		&value.ExpectedDraftVersion,
		&value.ResultingDraftVersion,
		&value.CreatedAt,
	); err != nil {
		return DraftPartyCommand{}, err
	}
	return value, nil
}

func mapDraftPartyInsertError(err error, candidate DraftParty) error {
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && databaseError.Code == "23505" && databaseError.ConstraintName == "contract_draft_parties_active_unique" {
		return &DuplicateDraftPartyError{PartyID: candidate.PartyID, Role: candidate.Role}
	}
	return fmt.Errorf("insert contract draft party: %w", err)
}
