package ledgerrestore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/contract-ledger/internal/audit"
	"omsbizfoundry/contract-ledger/internal/ledgerexport"
)

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Restore(ctx context.Context, command Command) (Record, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Record{}, false, fmt.Errorf("begin contract ledger restore: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('contract-ledger-restore'))`); err != nil {
		return Record{}, false, fmt.Errorf("lock contract ledger restore: %w", err)
	}

	if existing, found, err := readRestoreByCommand(ctx, tx, command.CommandID); err != nil {
		return Record{}, false, err
	} else if found {
		if existing.LedgerSHA256 != command.Candidate.LedgerSHA256 || existing.Actor != command.Actor {
			return Record{}, false, &IdempotencyConflictError{CommandID: command.CommandID}
		}
		return existing, false, nil
	}
	if existing, found, err := readRestoreByDigest(ctx, tx, command.Candidate.LedgerSHA256); err != nil {
		return Record{}, false, err
	} else if found {
		return existing, false, nil
	}

	var contracts, parties int64
	if err := tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM contracts), (SELECT count(*) FROM parties)`).Scan(&contracts, &parties); err != nil {
		return Record{}, false, fmt.Errorf("check restore destination: %w", err)
	}
	if contracts != 0 || parties != 0 {
		return Record{}, false, &DestinationNotEmptyError{}
	}
	if err := checkSourceAuditConflicts(ctx, tx, command.Candidate.Ledger.Audit); err != nil {
		return Record{}, false, err
	}

	ledger := command.Candidate.Ledger
	if err := insertRestoredParties(ctx, tx, ledger.DraftParties); err != nil {
		return Record{}, false, err
	}
	createAudit, err := uniqueAudit(ledger.Audit, "contract_draft_created", "", "")
	if err != nil {
		return Record{}, false, err
	}
	if err := insertRestoredContract(ctx, tx, ledger.Contract, *createAudit.CommandID); err != nil {
		return Record{}, false, err
	}
	if err := insertRestoredDraftParties(ctx, tx, ledger.Contract.ID, ledger.DraftParties); err != nil {
		return Record{}, false, err
	}
	if err := insertRestoredCancellation(ctx, tx, ledger.Contract.ID, ledger.Cancellation); err != nil {
		return Record{}, false, err
	}
	if err := insertRestoredFormalHistory(ctx, tx, ledger); err != nil {
		return Record{}, false, err
	}
	if err := insertRestoredSettlements(ctx, tx, ledger); err != nil {
		return Record{}, false, err
	}
	if err := insertRestoredClosure(ctx, tx, ledger); err != nil {
		return Record{}, false, err
	}
	if err := insertRestoredEvidence(ctx, tx, ledger); err != nil {
		return Record{}, false, err
	}
	if err := insertSourceAudit(ctx, tx, ledger.Contract.ID, ledger.Audit); err != nil {
		return Record{}, false, err
	}

	missing := countEvidenceVersions(ledger.Evidence)
	if _, err := tx.Exec(ctx, `
		INSERT INTO contract_restore_operations (
			command_id, ledger_sha256, archive_format, source_generated_at,
			contract_id, missing_evidence_versions,
			actor_kind, actor_id, actor_display_name, restored_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, command.CommandID, command.Candidate.LedgerSHA256, ledgerexport.FormatID,
		command.Candidate.SourceGeneratedAt, ledger.Contract.ID, missing,
		command.Actor.Kind, command.Actor.ID, command.Actor.DisplayName, command.RestoredAt); err != nil {
		return Record{}, false, fmt.Errorf("insert contract restore operation: %w", err)
	}
	if err := audit.Insert(ctx, tx, audit.Entry{
		ID: command.AuditID, ObjectType: audit.ObjectContract, ObjectID: ledger.Contract.ID,
		Action: ActionContractLedgerRestored, CommandID: command.CommandID,
		Actor: command.Actor, OccurredAt: command.RestoredAt,
		Summary: "从单合同可验证账簿恢复合同",
		Details: map[string]any{
			"format_id":                 ledgerexport.FormatID,
			"ledger_sha256":             command.Candidate.LedgerSHA256,
			"source_generated_at":       command.Candidate.SourceGeneratedAt.Format(time.RFC3339Nano),
			"snapshot_status":           ledger.Contract.Status,
			"current_revision":          nullableIntValue(ledger.Contract.CurrentRevision),
			"missing_evidence_versions": missing,
		},
	}); err != nil {
		return Record{}, false, fmt.Errorf("insert contract restore audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Record{}, false, fmt.Errorf("commit contract ledger restore: %w", err)
	}
	return restoreRecord(command), true, nil
}

func checkSourceAuditConflicts(ctx context.Context, tx pgx.Tx, entries []ledgerexport.AuditEntry) error {
	for _, entry := range entries {
		var conflict bool
		if entry.CommandID == nil {
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM audit_entries WHERE id = $1)`, entry.ID).Scan(&conflict); err != nil {
				return fmt.Errorf("check restored audit conflict: %w", err)
			}
		} else {
			if err := tx.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT 1 FROM audit_entries
					WHERE id = $1 OR (action = $2 AND command_id = $3)
				)
			`, entry.ID, entry.Action, *entry.CommandID).Scan(&conflict); err != nil {
				return fmt.Errorf("check restored audit conflict: %w", err)
			}
		}
		if conflict {
			return &DestinationConflictError{Message: "归档源审计与目标库中的本地审计冲突"}
		}
	}
	return nil
}

func readRestoreByCommand(ctx context.Context, tx pgx.Tx, commandID string) (Record, bool, error) {
	return scanRestoreRecord(tx.QueryRow(ctx, restoreRecordQuery+` WHERE operation.command_id = $1`, commandID))
}

func readRestoreByDigest(ctx context.Context, tx pgx.Tx, digest string) (Record, bool, error) {
	return scanRestoreRecord(tx.QueryRow(ctx, restoreRecordQuery+` WHERE operation.ledger_sha256 = $1`, digest))
}

const restoreRecordQuery = `
	SELECT operation.command_id::text, operation.ledger_sha256, operation.contract_id::text,
		contract.status, COALESCE(contract.current_revision, 0), operation.missing_evidence_versions,
		operation.actor_kind, operation.actor_id, operation.actor_display_name, operation.restored_at
	FROM contract_restore_operations operation
	JOIN contracts contract ON contract.id = operation.contract_id
`

func scanRestoreRecord(row pgx.Row) (Record, bool, error) {
	var value Record
	err := row.Scan(&value.CommandID, &value.LedgerSHA256, &value.ContractID,
		&value.Status, &value.CurrentRevision, &value.MissingEvidenceVersions,
		&value.Actor.Kind, &value.Actor.ID, &value.Actor.DisplayName, &value.RestoredAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Record{}, false, nil
	}
	if err != nil {
		return Record{}, false, fmt.Errorf("query contract restore operation: %w", err)
	}
	return value, true, nil
}

func insertRestoredParties(ctx context.Context, tx pgx.Tx, values []ledgerexport.DraftParty) error {
	type partyFact struct {
		value     ledgerexport.DraftParty
		createdAt string
	}
	parties := make(map[string]partyFact)
	for _, value := range values {
		fact, exists := parties[value.PartyID]
		if !exists || value.AddedAt < fact.createdAt {
			parties[value.PartyID] = partyFact{value: value, createdAt: value.AddedAt}
		}
	}
	for _, fact := range parties {
		_, err := tx.Exec(ctx, `
			INSERT INTO parties (
				id, kind, legal_name, registration_identifier, revision, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $6)
		`, fact.value.PartyID, fact.value.PartyKind, fact.value.PartyLegalName,
			optionalStringValue(fact.value.RegistrationIdentifier), fact.value.PartyRevision, fact.createdAt)
		if err != nil {
			return fmt.Errorf("insert restored party %s: %w", fact.value.PartyID, err)
		}
	}
	return nil
}

func insertRestoredContract(ctx context.Context, tx pgx.Tx, value ledgerexport.ContractSnapshot, commandID string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO contracts (
			id, command_id, number_namespace, contract_number, title, category,
			currency, total_amount, signed_on, effective_on, status,
			draft_version, current_revision, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`, value.ID, commandID, value.NumberNamespace, value.ContractNumber, value.Title,
		optionalStringValue(value.Category), value.Currency, optionalStringValue(value.TotalAmount),
		optionalStringValue(value.SignedOn), optionalStringValue(value.EffectiveOn), value.Status,
		value.DraftVersion, optionalInt64Value(value.CurrentRevision), value.CreatedAt, value.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert restored contract: %w", err)
	}
	return nil
}

func insertRestoredDraftParties(ctx context.Context, tx pgx.Tx, contractID string, values []ledgerexport.DraftParty) error {
	for _, value := range values {
		_, err := tx.Exec(ctx, `
			INSERT INTO contract_draft_parties (
				id, contract_id, party_id, role, added_at, removed_at
			) VALUES ($1, $2, $3, $4, $5, $6)
		`, value.ID, contractID, value.PartyID, value.Role, value.AddedAt, optionalStringValue(value.RemovedAt))
		if err != nil {
			return fmt.Errorf("insert restored draft party %s: %w", value.ID, err)
		}
	}
	return nil
}

func insertRestoredCancellation(ctx context.Context, tx pgx.Tx, contractID string, value *ledgerexport.DraftCancellation) error {
	if value == nil {
		return nil
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_draft_change_commands (
			command_id, contract_id, action, expected_draft_version, resulting_draft_version,
			cancellation_reason, created_at
		) VALUES ($1, $2, 'cancel', $3, $4, $5, $6)
	`, value.CommandID, contractID, value.ExpectedDraftVersion, value.ResultingDraftVersion,
		value.Reason, value.CancelledAt)
	if err != nil {
		return fmt.Errorf("insert restored cancellation: %w", err)
	}
	return nil
}

func insertRestoredFormalHistory(ctx context.Context, tx pgx.Tx, ledger ledgerexport.Ledger) error {
	if len(ledger.Revisions) == 0 {
		return nil
	}
	first := ledger.Revisions[0]
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_activation_commands (
			command_id, contract_id, expected_draft_version,
			actor_kind, actor_id, resulting_revision, occurred_at
		) VALUES ($1, $2, $3, $4, $5, 1, $6)
	`, *first.ActivationCommandID, ledger.Contract.ID, *first.SourceDraftVersion,
		first.Actor.Kind, first.Actor.ID, first.RecordedAt)
	if err != nil {
		return fmt.Errorf("insert restored activation command: %w", err)
	}
	for _, value := range ledger.Instruments {
		_, err := tx.Exec(ctx, `
			INSERT INTO contract_instruments (
				id, contract_id, instrument_type, instrument_number, title, change_summary,
				signed_on, effective_on, effective_order, amount_effect, amount_value,
				source_command_id, resulting_revision,
				actor_kind, actor_id, actor_display_name, recorded_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		`, value.ID, ledger.Contract.ID, value.InstrumentType, value.InstrumentNumber,
			value.Title, value.ChangeSummary, value.SignedOn, value.EffectiveOn,
			value.EffectiveOrder, value.AmountEffect, optionalStringValue(value.AmountValue),
			value.SourceCommandID, value.ResultingRevision,
			value.Actor.Kind, value.Actor.ID, value.Actor.DisplayName, value.RecordedAt)
		if err != nil {
			return fmt.Errorf("insert restored instrument %s: %w", value.ID, err)
		}
	}
	for _, value := range ledger.Revisions {
		_, err := tx.Exec(ctx, `
			INSERT INTO contract_revisions (
				contract_id, revision, activation_command_id, source_draft_version,
				number_namespace, contract_number, title, category, currency, total_amount,
				signed_on, effective_on, actor_kind, actor_id, recorded_at, source_instrument_id
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		`, ledger.Contract.ID, value.Revision, optionalStringValue(value.ActivationCommandID),
			optionalInt64Value(value.SourceDraftVersion), value.NumberNamespace, value.ContractNumber,
			value.Title, optionalStringValue(value.Category), value.Currency, optionalStringValue(value.TotalAmount),
			value.SignedOn, value.EffectiveOn, value.Actor.Kind, value.Actor.ID,
			value.RecordedAt, value.SourceInstrumentID)
		if err != nil {
			return fmt.Errorf("insert restored revision %d: %w", value.Revision, err)
		}
		for _, party := range value.Parties {
			if err := insertRevisionParty(ctx, tx, ledger.Contract.ID, value.Revision, party); err != nil {
				return err
			}
		}
	}
	activationAudit, err := uniqueAudit(ledger.Audit, "contract_activated", "", "")
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO contract_audit_entries (
			id, contract_id, event_type, command_id, actor_kind, actor_id,
			occurred_at, source_draft_version, resulting_revision
		) VALUES ($1, $2, 'contract_activated', $3, $4, $5, $6, $7, 1)
	`, activationAudit.ID, ledger.Contract.ID, *first.ActivationCommandID,
		first.Actor.Kind, first.Actor.ID, first.RecordedAt, *first.SourceDraftVersion)
	if err != nil {
		return fmt.Errorf("insert restored activation legacy audit: %w", err)
	}
	for index, instrument := range ledger.Instruments {
		if index == 0 {
			continue
		}
		revision := ledger.Revisions[index]
		if err := insertRestoredFormalCommand(ctx, tx, ledger, instrument, revision); err != nil {
			return err
		}
	}
	return nil
}

func insertRevisionParty(ctx context.Context, tx pgx.Tx, contractID string, revision int64, value ledgerexport.PartySnapshot) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_revision_parties (
			contract_id, revision, position, source_party_link_id, source_party_id,
			source_party_revision, party_kind, legal_name, registration_identifier, role
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, contractID, revision, value.Position, value.SourcePartyLinkID, value.SourcePartyID,
		value.SourcePartyRevision, value.PartyKind, value.LegalName,
		optionalStringValue(value.RegistrationIdentifier), value.Role)
	if err != nil {
		return fmt.Errorf("insert restored revision %d party position %d: %w", revision, value.Position, err)
	}
	return nil
}

func insertRestoredFormalCommand(ctx context.Context, tx pgx.Tx, ledger ledgerexport.Ledger, instrument ledgerexport.Instrument, revision ledgerexport.Revision) error {
	switch instrument.InstrumentType {
	case "supplemental_agreement":
		_, err := tx.Exec(ctx, `
			INSERT INTO contract_change_commands (
				command_id, instrument_id, contract_id, expected_revision,
				instrument_number, title, change_summary, signed_on, effective_on,
				effective_order, amount_effect, amount_value,
				actor_kind, actor_id, actor_display_name, resulting_revision, occurred_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		`, instrument.SourceCommandID, instrument.ID, ledger.Contract.ID, instrument.ResultingRevision-1,
			instrument.InstrumentNumber, instrument.Title, instrument.ChangeSummary,
			instrument.SignedOn, instrument.EffectiveOn, instrument.EffectiveOrder,
			instrument.AmountEffect, optionalStringValue(instrument.AmountValue),
			instrument.Actor.Kind, instrument.Actor.ID, instrument.Actor.DisplayName,
			instrument.ResultingRevision, instrument.RecordedAt)
		if err != nil {
			return fmt.Errorf("insert restored change command: %w", err)
		}
	case "termination_instrument":
		_, err := tx.Exec(ctx, `
			INSERT INTO contract_termination_commands (
				command_id, instrument_id, contract_id, expected_revision,
				instrument_number, title, termination_reason, signed_on, effective_on,
				effective_order, actor_kind, actor_id, actor_display_name,
				resulting_revision, occurred_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		`, instrument.SourceCommandID, instrument.ID, ledger.Contract.ID, instrument.ResultingRevision-1,
			instrument.InstrumentNumber, instrument.Title, instrument.ChangeSummary,
			instrument.SignedOn, instrument.EffectiveOn, instrument.EffectiveOrder,
			instrument.Actor.Kind, instrument.Actor.ID, instrument.Actor.DisplayName,
			instrument.ResultingRevision, instrument.RecordedAt)
		if err != nil {
			return fmt.Errorf("insert restored termination command: %w", err)
		}
	case "correction_instrument":
		auditEntry, err := uniqueAudit(ledger.Audit, "contract_correction_recorded", "instrument_id", instrument.ID)
		if err != nil {
			return err
		}
		details, _ := auditDetails(auditEntry)
		expectedStatus, _ := details["expected_status"].(string)
		_, err = tx.Exec(ctx, `
			INSERT INTO contract_correction_commands (
				command_id, instrument_id, contract_id, expected_status, expected_revision,
				instrument_number, instrument_title, correction_reason,
				instrument_signed_on, instrument_effective_on, effective_order,
				corrected_title, corrected_category, corrected_total_amount,
				corrected_signed_on, corrected_effective_on,
				actor_kind, actor_id, actor_display_name, resulting_revision, occurred_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
		`, instrument.SourceCommandID, instrument.ID, ledger.Contract.ID, expectedStatus,
			instrument.ResultingRevision-1, instrument.InstrumentNumber, instrument.Title,
			instrument.ChangeSummary, instrument.SignedOn, instrument.EffectiveOn,
			instrument.EffectiveOrder, revision.Title, optionalStringValue(revision.Category),
			optionalStringValue(revision.TotalAmount), revision.SignedOn, revision.EffectiveOn,
			instrument.Actor.Kind, instrument.Actor.ID, instrument.Actor.DisplayName,
			instrument.ResultingRevision, instrument.RecordedAt)
		if err != nil {
			return fmt.Errorf("insert restored correction command: %w", err)
		}
	default:
		return fmt.Errorf("unsupported restored instrument type %q", instrument.InstrumentType)
	}
	return nil
}

func insertRestoredSettlements(ctx context.Context, tx pgx.Tx, ledger ledgerexport.Ledger) error {
	for _, value := range ledger.Settlements {
		_, err := tx.Exec(ctx, `
			INSERT INTO contract_settlements (
				id, contract_id, source_revision, settlement_sequence, settlement_kind,
				statement_number, title, total_amount, currency, confirmed_on, basis,
				actor_kind, actor_id, actor_display_name, recorded_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		`, value.ID, ledger.Contract.ID, value.SourceRevision, value.Sequence, value.Kind,
			value.StatementNumber, value.Title, value.Amount, value.Currency, value.ConfirmedOn,
			value.Basis, value.Actor.Kind, value.Actor.ID, value.Actor.DisplayName, value.RecordedAt)
		if err != nil {
			return fmt.Errorf("insert restored settlement %s: %w", value.ID, err)
		}
		positions := make([]int64, len(value.ConfirmingParties))
		for index, party := range value.ConfirmingParties {
			positions[index] = party.Position
			_, err := tx.Exec(ctx, `
				INSERT INTO contract_settlement_parties (
					settlement_id, position, source_party_link_id, source_party_id,
					source_party_revision, party_kind, legal_name, registration_identifier, role
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`, value.ID, party.Position, party.SourcePartyLinkID, party.SourcePartyID,
				party.SourcePartyRevision, party.PartyKind, party.LegalName,
				optionalStringValue(party.RegistrationIdentifier), party.Role)
			if err != nil {
				return fmt.Errorf("insert restored settlement party: %w", err)
			}
		}
		auditEntry, err := uniqueAudit(ledger.Audit, "contract_settlement_recorded", "settlement_id", value.ID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO contract_settlement_commands (
				command_id, settlement_id, contract_id,
				expected_contract_revision, expected_settlement_sequence,
				settlement_kind, statement_number, title, total_amount, confirmed_on, basis,
				confirming_party_positions, actor_kind, actor_id, actor_display_name,
				resulting_settlement_sequence, occurred_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		`, *auditEntry.CommandID, value.ID, ledger.Contract.ID, value.SourceRevision, value.Sequence-1,
			value.Kind, value.StatementNumber, value.Title, value.Amount, value.ConfirmedOn,
			value.Basis, positions, value.Actor.Kind, value.Actor.ID, value.Actor.DisplayName,
			value.Sequence, value.RecordedAt)
		if err != nil {
			return fmt.Errorf("insert restored settlement command: %w", err)
		}
	}
	return nil
}

func insertRestoredClosure(ctx context.Context, tx pgx.Tx, ledger ledgerexport.Ledger) error {
	if ledger.Closure == nil {
		return nil
	}
	value := ledger.Closure
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_closures (
			id, contract_id, source_revision, source_settlement_sequence,
			settlement_disposition, final_settlement_id, closed_on, reason,
			actor_kind, actor_id, actor_display_name, recorded_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, value.ID, ledger.Contract.ID, value.SourceRevision, value.SourceSettlementSequence,
		value.SettlementDisposition, optionalStringValue(value.FinalSettlementID), value.ClosedOn,
		value.Reason, value.Actor.Kind, value.Actor.ID, value.Actor.DisplayName, value.RecordedAt)
	if err != nil {
		return fmt.Errorf("insert restored closure: %w", err)
	}
	auditEntry, err := uniqueAudit(ledger.Audit, "contract_closed", "closure_id", value.ID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO contract_closure_commands (
			command_id, closure_id, contract_id, expected_contract_revision,
			expected_settlement_sequence, settlement_disposition, closed_on, reason,
			actor_kind, actor_id, actor_display_name, occurred_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, *auditEntry.CommandID, value.ID, ledger.Contract.ID, value.SourceRevision,
		value.SourceSettlementSequence, value.SettlementDisposition, value.ClosedOn,
		value.Reason, value.Actor.Kind, value.Actor.ID, value.Actor.DisplayName, value.RecordedAt)
	if err != nil {
		return fmt.Errorf("insert restored closure command: %w", err)
	}
	return nil
}

func insertRestoredEvidence(ctx context.Context, tx pgx.Tx, ledger ledgerexport.Ledger) error {
	for _, document := range ledger.Evidence {
		_, err := tx.Exec(ctx, `
			INSERT INTO contract_evidence_documents (
				id, contract_id, target_type, target_id, title, purpose,
				actor_kind, actor_id, actor_display_name, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, document.ID, ledger.Contract.ID, document.TargetType, document.TargetID,
			document.Title, document.Purpose, document.Actor.Kind, document.Actor.ID,
			document.Actor.DisplayName, document.CreatedAt)
		if err != nil {
			return fmt.Errorf("insert restored evidence document %s: %w", document.ID, err)
		}
		for index, version := range document.Versions {
			_, err := tx.Exec(ctx, `
				INSERT INTO contract_evidence_versions (
					id, document_id, version, original_filename, detected_media_type,
					size_bytes, sha256, storage_provider, storage_key, content_status,
					actor_kind, actor_id, actor_display_name, recorded_at
				) VALUES ($1, $2, $3, $4, $5, $6, $7, NULL, NULL, 'missing', $8, $9, $10, $11)
			`, version.ID, document.ID, version.Version, version.OriginalFilename,
				version.DetectedMediaType, version.SizeBytes, version.SHA256,
				version.Actor.Kind, version.Actor.ID, version.Actor.DisplayName, version.RecordedAt)
			if err != nil {
				return fmt.Errorf("insert restored evidence version %s: %w", version.ID, err)
			}
			action := "contract_evidence_version_added"
			if index == 0 {
				action = "contract_evidence_created"
			}
			auditEntry, err := uniqueAudit(ledger.Audit, action, "version_id", version.ID)
			if err != nil {
				return err
			}
			commandAction := "add_version"
			if index == 0 {
				commandAction = "create_document"
			}
			_, err = tx.Exec(ctx, `
				INSERT INTO contract_evidence_commands (
					command_id, action, contract_id, document_id, version_id,
					expected_latest_version, resulting_version,
					actor_kind, actor_id, actor_display_name, occurred_at
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			`, *auditEntry.CommandID, commandAction, ledger.Contract.ID, document.ID,
				version.ID, version.Version-1, version.Version,
				version.Actor.Kind, version.Actor.ID, version.Actor.DisplayName, version.RecordedAt)
			if err != nil {
				return fmt.Errorf("insert restored evidence command: %w", err)
			}
		}
	}
	return nil
}

func insertSourceAudit(ctx context.Context, tx pgx.Tx, contractID string, entries []ledgerexport.AuditEntry) error {
	for _, entry := range entries {
		var commandID any
		if entry.CommandID != nil {
			commandID = *entry.CommandID
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO audit_entries (
				id, object_type, object_id, action, command_id,
				actor_kind, actor_id, actor_display_name,
				occurred_at, summary, details
			) VALUES ($1, 'contract', $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb)
		`, entry.ID, contractID, entry.Action, commandID,
			entry.Actor.Kind, entry.Actor.ID, entry.Actor.DisplayName,
			entry.OccurredAt, entry.Summary, []byte(entry.Details))
		if err != nil {
			return fmt.Errorf("insert restored source audit %s: %w", entry.ID, err)
		}
	}
	return nil
}

func restoreRecord(command Command) Record {
	currentRevision := int64(0)
	if command.Candidate.Ledger.Contract.CurrentRevision != nil {
		currentRevision = *command.Candidate.Ledger.Contract.CurrentRevision
	}
	return Record{
		CommandID:               command.CommandID,
		LedgerSHA256:            command.Candidate.LedgerSHA256,
		ContractID:              command.Candidate.Ledger.Contract.ID,
		Status:                  command.Candidate.Ledger.Contract.Status,
		CurrentRevision:         currentRevision,
		MissingEvidenceVersions: countEvidenceVersions(command.Candidate.Ledger.Evidence),
		Actor:                   command.Actor,
		RestoredAt:              command.RestoredAt,
	}
}

func countEvidenceVersions(values []ledgerexport.EvidenceDocument) int {
	count := 0
	for _, value := range values {
		count += len(value.Versions)
	}
	return count
}

func optionalStringValue(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func optionalInt64Value(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableIntValue(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}
