package ledgerexport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/contract-ledger/internal/audit"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Load(ctx context.Context, contractID string) (Ledger, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return Ledger{}, false, fmt.Errorf("begin contract ledger snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	ledger := emptyLedger()
	found, err := loadContract(ctx, tx, contractID, &ledger.Contract)
	if err != nil || !found {
		return Ledger{}, found, err
	}
	// The legacy single-contract archive has no typed customer-overlay field.
	// Refuse a lossy export until its versioned archive/restore port is extended.
	var overlayExists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM reference_oa_contract_drafts WHERE contract_id=$1)`, contractID).Scan(&overlayExists); err != nil {
		return Ledger{}, false, err
	}
	if overlayExists {
		return Ledger{}, false, fmt.Errorf("reference OA overlay archive/restore is not available; use a complete database backup")
	}
	if ledger.DraftParties, err = loadDraftParties(ctx, tx, contractID); err != nil {
		return Ledger{}, false, err
	}
	if ledger.Cancellation, err = loadCancellation(ctx, tx, contractID); err != nil {
		return Ledger{}, false, err
	}
	if ledger.Revisions, err = loadRevisions(ctx, tx, contractID); err != nil {
		return Ledger{}, false, err
	}
	if err := loadRevisionParties(ctx, tx, contractID, ledger.Revisions); err != nil {
		return Ledger{}, false, err
	}
	if ledger.Instruments, err = loadInstruments(ctx, tx, contractID); err != nil {
		return Ledger{}, false, err
	}
	if ledger.Relations, err = loadRelations(ctx, tx, contractID); err != nil {
		return Ledger{}, false, err
	}
	if ledger.Settlements, err = loadSettlements(ctx, tx, contractID); err != nil {
		return Ledger{}, false, err
	}
	if err := loadSettlementParties(ctx, tx, ledger.Settlements); err != nil {
		return Ledger{}, false, err
	}
	if ledger.Closure, err = loadClosure(ctx, tx, contractID); err != nil {
		return Ledger{}, false, err
	}
	if ledger.Evidence, err = loadEvidence(ctx, tx, contractID); err != nil {
		return Ledger{}, false, err
	}
	if err := loadEvidenceVersions(ctx, tx, ledger.Evidence); err != nil {
		return Ledger{}, false, err
	}
	if ledger.Audit, err = loadAudit(ctx, tx, contractID); err != nil {
		return Ledger{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Ledger{}, false, fmt.Errorf("commit contract ledger snapshot: %w", err)
	}
	return ledger, true, nil
}

func (r *PostgresRepository) RecordExport(ctx context.Context, entry audit.Entry) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin ledger export audit: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := audit.Insert(ctx, tx, entry); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit ledger export audit: %w", err)
	}
	return nil
}

func emptyLedger() Ledger {
	return Ledger{
		DraftParties: make([]DraftParty, 0),
		Revisions:    make([]Revision, 0),
		Instruments:  make([]Instrument, 0),
		Relations:    make([]Relation, 0),
		Settlements:  make([]Settlement, 0),
		Evidence:     make([]EvidenceDocument, 0),
		Audit:        make([]AuditEntry, 0),
	}
}

func loadContract(ctx context.Context, tx pgx.Tx, contractID string, value *ContractSnapshot) (bool, error) {
	var category, amount, signedOn, effectiveOn string
	var currentRevision int64
	var createdAt, updatedAt time.Time
	err := tx.QueryRow(ctx, `
		SELECT c.id::text,
			CASE WHEN c.current_revision IS NULL THEN c.number_namespace ELSE r.number_namespace END,
			CASE WHEN c.current_revision IS NULL THEN c.contract_number ELSE r.contract_number END,
			CASE WHEN c.current_revision IS NULL THEN c.title ELSE r.title END,
			COALESCE(CASE WHEN c.current_revision IS NULL THEN c.category ELSE r.category END, ''),
			CASE WHEN c.current_revision IS NULL THEN c.currency ELSE r.currency END,
			COALESCE((CASE WHEN c.current_revision IS NULL THEN c.total_amount ELSE r.total_amount END)::text, ''),
			COALESCE(to_char(CASE WHEN c.current_revision IS NULL THEN c.signed_on ELSE r.signed_on END, 'YYYY-MM-DD'), ''),
			COALESCE(to_char(CASE WHEN c.current_revision IS NULL THEN c.effective_on ELSE r.effective_on END, 'YYYY-MM-DD'), ''),
			c.status, c.draft_version, COALESCE(c.current_revision, 0), c.created_at, c.updated_at
		FROM contracts c
		LEFT JOIN contract_revisions r
			ON r.contract_id = c.id AND r.revision = c.current_revision
		WHERE c.id = $1
	`, contractID).Scan(
		&value.ID, &value.NumberNamespace, &value.ContractNumber, &value.Title,
		&category, &value.Currency, &amount, &signedOn, &effectiveOn,
		&value.Status, &value.DraftVersion, &currentRevision, &createdAt, &updatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query contract export root: %w", err)
	}
	value.Category = optionalString(category)
	value.TotalAmount = optionalString(amount)
	value.SignedOn = optionalString(signedOn)
	value.EffectiveOn = optionalString(effectiveOn)
	value.CurrentRevision = optionalInt64(currentRevision)
	value.CreatedAt = formatTime(createdAt)
	value.UpdatedAt = formatTime(updatedAt)
	return true, nil
}

func loadDraftParties(ctx context.Context, tx pgx.Tx, contractID string) ([]DraftParty, error) {
	rows, err := tx.Query(ctx, `
		SELECT link.id::text, link.party_id::text, party.kind, party.legal_name,
			COALESCE(party.registration_identifier, ''), party.revision,
			link.role, link.added_at, link.removed_at
		FROM contract_draft_parties link
		JOIN parties party ON party.id = link.party_id
		WHERE link.contract_id = $1
		ORDER BY link.added_at, link.id
	`, contractID)
	if err != nil {
		return nil, fmt.Errorf("query contract export draft parties: %w", err)
	}
	defer rows.Close()
	values := make([]DraftParty, 0)
	for rows.Next() {
		var value DraftParty
		var registration string
		var addedAt time.Time
		var removedAt *time.Time
		if err := rows.Scan(
			&value.ID, &value.PartyID, &value.PartyKind, &value.PartyLegalName,
			&registration, &value.PartyRevision, &value.Role, &addedAt, &removedAt,
		); err != nil {
			return nil, fmt.Errorf("scan contract export draft party: %w", err)
		}
		value.RegistrationIdentifier = optionalString(registration)
		value.AddedAt = formatTime(addedAt)
		value.RemovedAt = optionalTime(removedAt)
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contract export draft parties: %w", err)
	}
	return values, nil
}

func loadCancellation(ctx context.Context, tx pgx.Tx, contractID string) (*DraftCancellation, error) {
	var value DraftCancellation
	var cancelledAt time.Time
	err := tx.QueryRow(ctx, `
		SELECT command.command_id::text, command.expected_draft_version,
			command.resulting_draft_version, command.cancellation_reason,
			audit.actor_kind, audit.actor_id, audit.actor_display_name, command.created_at
		FROM contract_draft_change_commands command
		JOIN audit_entries audit
			ON audit.action = 'contract_draft_cancelled' AND audit.command_id = command.command_id
		WHERE command.contract_id = $1 AND command.action = 'cancel'
	`, contractID).Scan(
		&value.CommandID, &value.ExpectedDraftVersion, &value.ResultingDraftVersion, &value.Reason,
		&value.Actor.Kind, &value.Actor.ID, &value.Actor.DisplayName, &cancelledAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query contract export cancellation: %w", err)
	}
	value.CancelledAt = formatTime(cancelledAt)
	return &value, nil
}

func loadRevisions(ctx context.Context, tx pgx.Tx, contractID string) ([]Revision, error) {
	rows, err := tx.Query(ctx, `
		SELECT r.revision, COALESCE(r.activation_command_id::text, ''), COALESCE(r.source_draft_version, 0),
			r.source_instrument_id::text, r.number_namespace, r.contract_number, r.title,
			COALESCE(r.category, ''), r.currency, COALESCE(r.total_amount::text, ''),
			to_char(r.signed_on, 'YYYY-MM-DD'), to_char(r.effective_on, 'YYYY-MM-DD'),
			r.actor_kind, r.actor_id, i.actor_display_name, r.recorded_at
		FROM contract_revisions r
		JOIN contract_instruments i ON i.id = r.source_instrument_id
		WHERE r.contract_id = $1
		ORDER BY r.revision
	`, contractID)
	if err != nil {
		return nil, fmt.Errorf("query contract export revisions: %w", err)
	}
	defer rows.Close()
	values := make([]Revision, 0)
	for rows.Next() {
		var value Revision
		var activationCommandID, category, amount string
		var sourceDraftVersion int64
		var recordedAt time.Time
		if err := rows.Scan(
			&value.Revision, &activationCommandID, &sourceDraftVersion,
			&value.SourceInstrumentID, &value.NumberNamespace, &value.ContractNumber, &value.Title,
			&category, &value.Currency, &amount, &value.SignedOn, &value.EffectiveOn,
			&value.Actor.Kind, &value.Actor.ID, &value.Actor.DisplayName, &recordedAt,
		); err != nil {
			return nil, fmt.Errorf("scan contract export revision: %w", err)
		}
		value.ActivationCommandID = optionalString(activationCommandID)
		value.SourceDraftVersion = optionalInt64(sourceDraftVersion)
		value.Category = optionalString(category)
		value.TotalAmount = optionalString(amount)
		value.RecordedAt = formatTime(recordedAt)
		value.Parties = make([]PartySnapshot, 0)
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contract export revisions: %w", err)
	}
	return values, nil
}

func loadRevisionParties(ctx context.Context, tx pgx.Tx, contractID string, revisions []Revision) error {
	rows, err := tx.Query(ctx, `
		SELECT revision, position, source_party_link_id::text, source_party_id::text,
			source_party_revision, party_kind, legal_name,
			COALESCE(registration_identifier, ''), role
		FROM contract_revision_parties
		WHERE contract_id = $1
		ORDER BY revision, position
	`, contractID)
	if err != nil {
		return fmt.Errorf("query contract export revision parties: %w", err)
	}
	defer rows.Close()
	indexes := make(map[int64]int, len(revisions))
	for index := range revisions {
		indexes[revisions[index].Revision] = index
	}
	for rows.Next() {
		var revision int64
		var value PartySnapshot
		var registration string
		if err := rows.Scan(
			&revision, &value.Position, &value.SourcePartyLinkID, &value.SourcePartyID,
			&value.SourcePartyRevision, &value.PartyKind, &value.LegalName, &registration, &value.Role,
		); err != nil {
			return fmt.Errorf("scan contract export revision party: %w", err)
		}
		index, found := indexes[revision]
		if !found {
			return fmt.Errorf("revision party references missing revision %d", revision)
		}
		value.RegistrationIdentifier = optionalString(registration)
		revisions[index].Parties = append(revisions[index].Parties, value)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate contract export revision parties: %w", err)
	}
	return nil
}

func loadInstruments(ctx context.Context, tx pgx.Tx, contractID string) ([]Instrument, error) {
	rows, err := tx.Query(ctx, `
		SELECT id::text, instrument_type, instrument_number, title, change_summary,
			to_char(signed_on, 'YYYY-MM-DD'), to_char(effective_on, 'YYYY-MM-DD'),
			effective_order, amount_effect, COALESCE(amount_value::text, ''),
			source_command_id::text, resulting_revision,
			actor_kind, actor_id, actor_display_name, recorded_at
		FROM contract_instruments
		WHERE contract_id = $1
		ORDER BY effective_order, id
	`, contractID)
	if err != nil {
		return nil, fmt.Errorf("query contract export instruments: %w", err)
	}
	defer rows.Close()
	values := make([]Instrument, 0)
	for rows.Next() {
		var value Instrument
		var amount string
		var recordedAt time.Time
		if err := rows.Scan(
			&value.ID, &value.InstrumentType, &value.InstrumentNumber, &value.Title, &value.ChangeSummary,
			&value.SignedOn, &value.EffectiveOn, &value.EffectiveOrder, &value.AmountEffect, &amount,
			&value.SourceCommandID, &value.ResultingRevision,
			&value.Actor.Kind, &value.Actor.ID, &value.Actor.DisplayName, &recordedAt,
		); err != nil {
			return nil, fmt.Errorf("scan contract export instrument: %w", err)
		}
		value.AmountValue = optionalString(amount)
		value.RecordedAt = formatTime(recordedAt)
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contract export instruments: %w", err)
	}
	return values, nil
}

func loadRelations(ctx context.Context, tx pgx.Tx, contractID string) ([]Relation, error) {
	rows, err := tx.Query(ctx, `
		SELECT relation.id::text, relation.relation_type,
			relation.source_contract_id::text, relation.source_revision,
			source.number_namespace, source.contract_number, source.title,
			relation.target_contract_id::text, relation.target_revision,
			target.number_namespace, target.contract_number, target.title,
			COALESCE(to_char(relation.effective_on, 'YYYY-MM-DD'), ''), COALESCE(relation.note, ''),
			relation.actor_kind, relation.actor_id, relation.actor_display_name, relation.created_at
		FROM contract_relations relation
		JOIN contract_revisions source
			ON source.contract_id = relation.source_contract_id AND source.revision = relation.source_revision
		JOIN contract_revisions target
			ON target.contract_id = relation.target_contract_id AND target.revision = relation.target_revision
		WHERE relation.source_contract_id = $1 OR relation.target_contract_id = $1
		ORDER BY relation.created_at, relation.id
	`, contractID)
	if err != nil {
		return nil, fmt.Errorf("query contract export relations: %w", err)
	}
	defer rows.Close()
	values := make([]Relation, 0)
	for rows.Next() {
		var value Relation
		var effectiveOn, note string
		var createdAt time.Time
		if err := rows.Scan(
			&value.ID, &value.Type,
			&value.Source.ContractID, &value.Source.Revision,
			&value.Source.NumberNamespace, &value.Source.ContractNumber, &value.Source.Title,
			&value.Target.ContractID, &value.Target.Revision,
			&value.Target.NumberNamespace, &value.Target.ContractNumber, &value.Target.Title,
			&effectiveOn, &note, &value.Actor.Kind, &value.Actor.ID, &value.Actor.DisplayName, &createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan contract export relation: %w", err)
		}
		value.EffectiveOn = optionalString(effectiveOn)
		value.Note = optionalString(note)
		value.CreatedAt = formatTime(createdAt)
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contract export relations: %w", err)
	}
	return values, nil
}

func loadSettlements(ctx context.Context, tx pgx.Tx, contractID string) ([]Settlement, error) {
	rows, err := tx.Query(ctx, `
		SELECT id::text, source_revision, settlement_sequence, settlement_kind,
			statement_number, title, total_amount::text, currency,
			to_char(confirmed_on, 'YYYY-MM-DD'), basis,
			actor_kind, actor_id, actor_display_name, recorded_at
		FROM contract_settlements
		WHERE contract_id = $1
		ORDER BY settlement_sequence, id
	`, contractID)
	if err != nil {
		return nil, fmt.Errorf("query contract export settlements: %w", err)
	}
	defer rows.Close()
	values := make([]Settlement, 0)
	for rows.Next() {
		var value Settlement
		var recordedAt time.Time
		if err := rows.Scan(
			&value.ID, &value.SourceRevision, &value.Sequence, &value.Kind,
			&value.StatementNumber, &value.Title, &value.Amount, &value.Currency,
			&value.ConfirmedOn, &value.Basis,
			&value.Actor.Kind, &value.Actor.ID, &value.Actor.DisplayName, &recordedAt,
		); err != nil {
			return nil, fmt.Errorf("scan contract export settlement: %w", err)
		}
		value.RecordedAt = formatTime(recordedAt)
		value.ConfirmingParties = make([]PartySnapshot, 0)
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contract export settlements: %w", err)
	}
	return values, nil
}

func loadSettlementParties(ctx context.Context, tx pgx.Tx, settlements []Settlement) error {
	if len(settlements) == 0 {
		return nil
	}
	ids := make([]string, len(settlements))
	indexes := make(map[string]int, len(settlements))
	for index := range settlements {
		ids[index] = settlements[index].ID
		indexes[settlements[index].ID] = index
	}
	rows, err := tx.Query(ctx, `
		SELECT settlement_id::text, position, source_party_link_id::text, source_party_id::text,
			source_party_revision, party_kind, legal_name,
			COALESCE(registration_identifier, ''), role
		FROM contract_settlement_parties
		WHERE settlement_id = ANY($1::uuid[])
		ORDER BY settlement_id, position
	`, ids)
	if err != nil {
		return fmt.Errorf("query contract export settlement parties: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var settlementID, registration string
		var value PartySnapshot
		if err := rows.Scan(
			&settlementID, &value.Position, &value.SourcePartyLinkID, &value.SourcePartyID,
			&value.SourcePartyRevision, &value.PartyKind, &value.LegalName, &registration, &value.Role,
		); err != nil {
			return fmt.Errorf("scan contract export settlement party: %w", err)
		}
		index, found := indexes[settlementID]
		if !found {
			return fmt.Errorf("settlement party references missing settlement %s", settlementID)
		}
		value.RegistrationIdentifier = optionalString(registration)
		settlements[index].ConfirmingParties = append(settlements[index].ConfirmingParties, value)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate contract export settlement parties: %w", err)
	}
	return nil
}

func loadClosure(ctx context.Context, tx pgx.Tx, contractID string) (*Closure, error) {
	var value Closure
	var finalSettlementID string
	var recordedAt time.Time
	err := tx.QueryRow(ctx, `
		SELECT id::text, source_revision, source_settlement_sequence,
			settlement_disposition, COALESCE(final_settlement_id::text, ''),
			to_char(closed_on, 'YYYY-MM-DD'), reason,
			actor_kind, actor_id, actor_display_name, recorded_at
		FROM contract_closures
		WHERE contract_id = $1
	`, contractID).Scan(
		&value.ID, &value.SourceRevision, &value.SourceSettlementSequence,
		&value.SettlementDisposition, &finalSettlementID,
		&value.ClosedOn, &value.Reason,
		&value.Actor.Kind, &value.Actor.ID, &value.Actor.DisplayName, &recordedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query contract export closure: %w", err)
	}
	value.FinalSettlementID = optionalString(finalSettlementID)
	value.RecordedAt = formatTime(recordedAt)
	return &value, nil
}

func loadEvidence(ctx context.Context, tx pgx.Tx, contractID string) ([]EvidenceDocument, error) {
	rows, err := tx.Query(ctx, `
		SELECT id::text, target_type, target_id::text, title, purpose,
			actor_kind, actor_id, actor_display_name, created_at
		FROM contract_evidence_documents
		WHERE contract_id = $1
		ORDER BY created_at, id
	`, contractID)
	if err != nil {
		return nil, fmt.Errorf("query contract export evidence: %w", err)
	}
	defer rows.Close()
	values := make([]EvidenceDocument, 0)
	for rows.Next() {
		var value EvidenceDocument
		var createdAt time.Time
		if err := rows.Scan(
			&value.ID, &value.TargetType, &value.TargetID, &value.Title, &value.Purpose,
			&value.Actor.Kind, &value.Actor.ID, &value.Actor.DisplayName, &createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan contract export evidence: %w", err)
		}
		value.CreatedAt = formatTime(createdAt)
		value.Versions = make([]EvidenceVersion, 0)
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contract export evidence: %w", err)
	}
	return values, nil
}

func loadEvidenceVersions(ctx context.Context, tx pgx.Tx, documents []EvidenceDocument) error {
	if len(documents) == 0 {
		return nil
	}
	ids := make([]string, len(documents))
	indexes := make(map[string]int, len(documents))
	for index := range documents {
		ids[index] = documents[index].ID
		indexes[documents[index].ID] = index
	}
	rows, err := tx.Query(ctx, `
		SELECT id::text, document_id::text, version, original_filename,
			detected_media_type, size_bytes, sha256, content_status = 'available',
			actor_kind, actor_id, actor_display_name, recorded_at
		FROM contract_evidence_versions
		WHERE document_id = ANY($1::uuid[])
		ORDER BY document_id, version
	`, ids)
	if err != nil {
		return fmt.Errorf("query contract export evidence versions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var documentID string
		var value EvidenceVersion
		var recordedAt time.Time
		if err := rows.Scan(
			&value.ID, &documentID, &value.Version, &value.OriginalFilename,
			&value.DetectedMediaType, &value.SizeBytes, &value.SHA256, &value.ContentAvailable,
			&value.Actor.Kind, &value.Actor.ID, &value.Actor.DisplayName, &recordedAt,
		); err != nil {
			return fmt.Errorf("scan contract export evidence version: %w", err)
		}
		index, found := indexes[documentID]
		if !found {
			return fmt.Errorf("evidence version references missing document %s", documentID)
		}
		value.RecordedAt = formatTime(recordedAt)
		documents[index].Versions = append(documents[index].Versions, value)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate contract export evidence versions: %w", err)
	}
	return nil
}

func loadAudit(ctx context.Context, tx pgx.Tx, contractID string) ([]AuditEntry, error) {
	rows, err := tx.Query(ctx, `
		SELECT id::text, action, COALESCE(command_id::text, ''),
			actor_kind, actor_id, actor_display_name, occurred_at, summary, details
		FROM audit_entries
		WHERE object_type = 'contract' AND object_id = $1
		ORDER BY occurred_at, id
	`, contractID)
	if err != nil {
		return nil, fmt.Errorf("query contract export audit: %w", err)
	}
	defer rows.Close()
	values := make([]AuditEntry, 0)
	for rows.Next() {
		var value AuditEntry
		var commandID string
		var occurredAt time.Time
		var details []byte
		if err := rows.Scan(
			&value.ID, &value.Action, &commandID,
			&value.Actor.Kind, &value.Actor.ID, &value.Actor.DisplayName,
			&occurredAt, &value.Summary, &details,
		); err != nil {
			return nil, fmt.Errorf("scan contract export audit: %w", err)
		}
		if !json.Valid(details) {
			return nil, fmt.Errorf("contract export audit %s contains invalid JSON", value.ID)
		}
		value.CommandID = optionalString(commandID)
		value.OccurredAt = formatTime(occurredAt)
		value.Details = append(json.RawMessage(nil), details...)
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contract export audit: %w", err)
	}
	return values, nil
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}

func optionalInt64(value int64) *int64 {
	if value == 0 {
		return nil
	}
	copy := value
	return &copy
}

func optionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := formatTime(*value)
	return &formatted
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}
