package evidence

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/contract-ledger/internal/audit"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Record(
	ctx context.Context,
	command Command,
) (Command, Document, Version, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Command{}, Document{}, Version{}, false, fmt.Errorf("begin evidence record: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('contract-evidence:' || $1))`, command.CommandID); err != nil {
		return Command{}, Document{}, Version{}, false, fmt.Errorf("lock evidence command: %w", err)
	}
	if existing, document, version, found, err := readEvidenceCommand(ctx, tx, command.CommandID); err != nil {
		return Command{}, Document{}, Version{}, false, err
	} else if found {
		return existing, document, version, false, nil
	}

	var contractExists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM contracts WHERE id = $1)`, command.ContractID).Scan(&contractExists); err != nil {
		return Command{}, Document{}, Version{}, false, fmt.Errorf("check evidence contract: %w", err)
	}
	if !contractExists {
		return Command{}, Document{}, Version{}, false, &ContractNotFoundError{ContractID: command.ContractID}
	}

	switch command.Action {
	case CommandCreateDocument:
		if err := validateEvidenceTarget(ctx, tx, command.ContractID, command.TargetType, command.TargetID); err != nil {
			return Command{}, Document{}, Version{}, false, err
		}
		if err := insertEvidenceDocument(ctx, tx, command); err != nil {
			return Command{}, Document{}, Version{}, false, err
		}
	case CommandAddVersion:
		document, err := lockEvidenceDocument(ctx, tx, command.ContractID, command.DocumentID)
		if errors.Is(err, pgx.ErrNoRows) {
			return Command{}, Document{}, Version{}, false, &DocumentNotFoundError{DocumentID: command.DocumentID}
		}
		if err != nil {
			return Command{}, Document{}, Version{}, false, err
		}
		var latest int64
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(max(version), 0)
			FROM contract_evidence_versions
			WHERE document_id = $1
		`, command.DocumentID).Scan(&latest); err != nil {
			return Command{}, Document{}, Version{}, false, fmt.Errorf("read latest evidence version: %w", err)
		}
		if latest != command.ExpectedLatestVersion {
			return Command{}, Document{}, Version{}, false, &VersionConflictError{Expected: command.ExpectedLatestVersion, Actual: latest}
		}
		command.TargetType = document.TargetType
		command.TargetID = document.TargetID
		command.Title = document.Title
		command.Purpose = document.Purpose
	default:
		return Command{}, Document{}, Version{}, false, fmt.Errorf("unsupported evidence command action %q", command.Action)
	}

	if err := insertEvidenceVersion(ctx, tx, command); err != nil {
		return Command{}, Document{}, Version{}, false, err
	}
	if err := insertEvidenceCommand(ctx, tx, command); err != nil {
		return Command{}, Document{}, Version{}, false, err
	}
	if err := insertEvidenceAudit(ctx, tx, command); err != nil {
		return Command{}, Document{}, Version{}, false, err
	}
	document, version, err := getEvidenceResult(ctx, tx, command.DocumentID, command.VersionID)
	if err != nil {
		return Command{}, Document{}, Version{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Command{}, Document{}, Version{}, false, fmt.Errorf("commit evidence record: %w", err)
	}
	return command, document, version, true, nil
}

func (r *PostgresRepository) List(ctx context.Context, contractID string) ([]Document, bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM contracts WHERE id = $1)`, contractID).Scan(&exists); err != nil {
		return nil, false, fmt.Errorf("check evidence contract: %w", err)
	}
	if !exists {
		return nil, false, nil
	}
	rows, err := r.pool.Query(ctx, evidenceListQuery+`
		WHERE d.contract_id = $1
		ORDER BY d.created_at, d.id, v.version
	`, contractID)
	if err != nil {
		return nil, false, fmt.Errorf("query contract evidence: %w", err)
	}
	defer rows.Close()
	documents, err := scanEvidenceDocuments(rows)
	if err != nil {
		return nil, false, err
	}
	return documents, true, nil
}

func (r *PostgresRepository) FindVersion(
	ctx context.Context,
	contractID string,
	documentID string,
	versionID string,
) (Document, Version, bool, error) {
	row := r.pool.QueryRow(ctx, evidenceListQuery+`
		WHERE d.contract_id = $1 AND d.id = $2 AND v.id = $3
	`, contractID, documentID, versionID)
	document, version, err := scanEvidenceRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Document{}, Version{}, false, nil
	}
	if err != nil {
		return Document{}, Version{}, false, fmt.Errorf("query evidence version: %w", err)
	}
	document.Versions = []Version{version}
	document.Latest = version.Version
	return document, version, true, nil
}

func (r *PostgresRepository) RecordDownload(ctx context.Context, event DownloadAudit) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin evidence download audit: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := audit.Insert(ctx, tx, audit.Entry{
		ID: event.AuditID, ObjectType: audit.ObjectContract, ObjectID: event.ContractID,
		Action: AuditActionDownloaded, CommandID: event.AccessID,
		Actor: event.Actor, OccurredAt: event.OccurredAt,
		Summary: fmt.Sprintf("下载合同证据：%s v%d", event.Document.Title, event.Version.Version),
		Details: map[string]any{
			"document_id":       event.Document.ID,
			"version_id":        event.Version.ID,
			"version":           event.Version.Version,
			"original_filename": event.Version.OriginalFilename,
			"size_bytes":        event.Version.SizeBytes,
			"sha256":            event.Version.SHA256,
		},
	}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit evidence download audit: %w", err)
	}
	return nil
}

func validateEvidenceTarget(ctx context.Context, tx pgx.Tx, contractID string, targetType TargetType, targetID string) error {
	var exists bool
	switch targetType {
	case TargetContract:
		exists = targetID == contractID
	case TargetInstrument:
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM contract_instruments WHERE id = $1 AND contract_id = $2)`, targetID, contractID).Scan(&exists); err != nil {
			return fmt.Errorf("check evidence instrument target: %w", err)
		}
	case TargetSettlement:
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM contract_settlements WHERE id = $1 AND contract_id = $2)`, targetID, contractID).Scan(&exists); err != nil {
			return fmt.Errorf("check evidence settlement target: %w", err)
		}
	case TargetClosure:
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM contract_closures WHERE id = $1 AND contract_id = $2)`, targetID, contractID).Scan(&exists); err != nil {
			return fmt.Errorf("check evidence closure target: %w", err)
		}
	}
	if !exists {
		return &TargetConflictError{TargetType: targetType, TargetID: targetID}
	}
	return nil
}

func insertEvidenceDocument(ctx context.Context, tx pgx.Tx, command Command) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_evidence_documents (
			id, contract_id, target_type, target_id, title, purpose,
			actor_kind, actor_id, actor_display_name, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, command.DocumentID, command.ContractID, command.TargetType, command.TargetID,
		command.Title, command.Purpose, command.Actor.Kind, command.Actor.ID,
		command.Actor.DisplayName, command.OccurredAt)
	if err != nil {
		return mapEvidenceInsertError(err, command)
	}
	return nil
}

func insertEvidenceVersion(ctx context.Context, tx pgx.Tx, command Command) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_evidence_versions (
			id, document_id, version, original_filename, detected_media_type,
			size_bytes, sha256, storage_provider, storage_key,
			actor_kind, actor_id, actor_display_name, recorded_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, command.VersionID, command.DocumentID, command.ResultingVersion,
		command.OriginalFilename, command.DetectedMediaType, command.SizeBytes,
		command.SHA256, command.StorageProvider, command.StorageKey,
		command.Actor.Kind, command.Actor.ID, command.Actor.DisplayName, command.OccurredAt)
	if err != nil {
		return mapEvidenceInsertError(err, command)
	}
	return nil
}

func insertEvidenceCommand(ctx context.Context, tx pgx.Tx, command Command) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO contract_evidence_commands (
			command_id, action, contract_id, document_id, version_id,
			expected_latest_version, resulting_version,
			actor_kind, actor_id, actor_display_name, occurred_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, command.CommandID, command.Action, command.ContractID, command.DocumentID,
		command.VersionID, command.ExpectedLatestVersion, command.ResultingVersion,
		command.Actor.Kind, command.Actor.ID, command.Actor.DisplayName, command.OccurredAt)
	if err != nil {
		return fmt.Errorf("insert evidence command: %w", err)
	}
	return nil
}

func insertEvidenceAudit(ctx context.Context, tx pgx.Tx, command Command) error {
	action := AuditActionCreated
	summary := fmt.Sprintf("上传合同证据：%s v1", command.Title)
	if command.Action == CommandAddVersion {
		action = AuditActionVersionAdded
		summary = fmt.Sprintf("追加合同证据版本：%s v%d", command.Title, command.ResultingVersion)
	}
	return audit.Insert(ctx, tx, audit.Entry{
		ID: command.AuditID, ObjectType: audit.ObjectContract, ObjectID: command.ContractID,
		Action: action, CommandID: command.CommandID,
		Actor: command.Actor, OccurredAt: command.OccurredAt, Summary: summary,
		Details: map[string]any{
			"document_id":         command.DocumentID,
			"version_id":          command.VersionID,
			"version":             command.ResultingVersion,
			"target_type":         command.TargetType,
			"target_id":           command.TargetID,
			"title":               command.Title,
			"purpose":             command.Purpose,
			"original_filename":   command.OriginalFilename,
			"detected_media_type": command.DetectedMediaType,
			"size_bytes":          command.SizeBytes,
			"sha256":              command.SHA256,
		},
	})
}

func readEvidenceCommand(ctx context.Context, tx pgx.Tx, commandID string) (Command, Document, Version, bool, error) {
	row := tx.QueryRow(ctx, `
		SELECT c.command_id::text, c.action, c.contract_id::text,
			c.document_id::text, c.version_id::text,
			c.expected_latest_version, c.resulting_version,
			c.actor_kind, c.actor_id, c.actor_display_name, c.occurred_at,
			d.target_type, d.target_id::text, d.title, d.purpose,
			v.original_filename, v.detected_media_type, v.size_bytes,
			v.sha256, COALESCE(v.storage_provider, ''), COALESCE(v.storage_key, ''), v.content_status
		FROM contract_evidence_commands c
		JOIN contract_evidence_documents d ON d.id = c.document_id
		JOIN contract_evidence_versions v ON v.id = c.version_id
		WHERE c.command_id = $1
	`, commandID)
	var command Command
	var contentStatus string
	if err := row.Scan(
		&command.CommandID, &command.Action, &command.ContractID,
		&command.DocumentID, &command.VersionID,
		&command.ExpectedLatestVersion, &command.ResultingVersion,
		&command.Actor.Kind, &command.Actor.ID, &command.Actor.DisplayName, &command.OccurredAt,
		&command.TargetType, &command.TargetID, &command.Title, &command.Purpose,
		&command.OriginalFilename, &command.DetectedMediaType, &command.SizeBytes,
		&command.SHA256, &command.StorageProvider, &command.StorageKey, &contentStatus,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Command{}, Document{}, Version{}, false, nil
		}
		return Command{}, Document{}, Version{}, false, fmt.Errorf("query evidence command: %w", err)
	}
	document, version, err := getEvidenceResult(ctx, tx, command.DocumentID, command.VersionID)
	if err != nil {
		return Command{}, Document{}, Version{}, false, err
	}
	return command, document, version, true, nil
}

func lockEvidenceDocument(ctx context.Context, tx pgx.Tx, contractID, documentID string) (Document, error) {
	var document Document
	err := tx.QueryRow(ctx, `
		SELECT id::text, contract_id::text, target_type, target_id::text,
			title, purpose, actor_kind, actor_id, actor_display_name, created_at
		FROM contract_evidence_documents
		WHERE contract_id = $1 AND id = $2
		FOR UPDATE
	`, contractID, documentID).Scan(
		&document.ID, &document.ContractID, &document.TargetType, &document.TargetID,
		&document.Title, &document.Purpose,
		&document.Actor.Kind, &document.Actor.ID, &document.Actor.DisplayName, &document.CreatedAt,
	)
	if err != nil {
		return Document{}, err
	}
	return document, nil
}

func getEvidenceResult(ctx context.Context, tx pgx.Tx, documentID, versionID string) (Document, Version, error) {
	document, version, err := scanEvidenceRow(tx.QueryRow(ctx, evidenceListQuery+`
		WHERE d.id = $1 AND v.id = $2
	`, documentID, versionID))
	if err != nil {
		return Document{}, Version{}, fmt.Errorf("query evidence result: %w", err)
	}
	document.Versions = []Version{version}
	document.Latest = version.Version
	return document, version, nil
}

const evidenceListQuery = `
	SELECT d.id::text, d.contract_id::text, d.target_type, d.target_id::text,
		d.title, d.purpose, d.actor_kind, d.actor_id, d.actor_display_name, d.created_at,
		CASE d.target_type
			WHEN 'contract' THEN '合同整体'
			WHEN 'instrument' THEN COALESCE((
				SELECT CASE i.instrument_type
					WHEN 'original_contract' THEN '原合同'
					WHEN 'supplemental_agreement' THEN '补充协议'
					WHEN 'termination_instrument' THEN '终止文书'
					WHEN 'correction_instrument' THEN '更正文书'
					ELSE i.instrument_type END || ' / ' || i.instrument_number
				FROM contract_instruments i WHERE i.id = d.target_id
			), '合同文书')
			WHEN 'settlement' THEN COALESCE((
				SELECT CASE s.settlement_kind WHEN 'final' THEN '最终结算' ELSE '阶段结算' END || ' / ' || s.statement_number
				FROM contract_settlements s WHERE s.id = d.target_id
			), '结算事实')
			WHEN 'closure' THEN COALESCE((
				SELECT '操作性关闭 / ' || to_char(c.closed_on, 'YYYY-MM-DD')
				FROM contract_closures c WHERE c.id = d.target_id
			), '操作性关闭')
			ELSE d.target_type
		END,
		v.id::text, v.document_id::text, v.version,
		v.original_filename, v.detected_media_type, v.size_bytes, v.sha256,
		COALESCE(v.storage_provider, ''), COALESCE(v.storage_key, ''), v.content_status,
		v.actor_kind, v.actor_id, v.actor_display_name, v.recorded_at
	FROM contract_evidence_documents d
	JOIN contract_evidence_versions v ON v.document_id = d.id
`

type rowScanner interface{ Scan(...any) error }

func scanEvidenceRow(row rowScanner) (Document, Version, error) {
	var document Document
	var version Version
	if err := row.Scan(
		&document.ID, &document.ContractID, &document.TargetType, &document.TargetID,
		&document.Title, &document.Purpose,
		&document.Actor.Kind, &document.Actor.ID, &document.Actor.DisplayName, &document.CreatedAt,
		&document.TargetLabel,
		&version.ID, &version.DocumentID, &version.Version,
		&version.OriginalFilename, &version.DetectedMediaType, &version.SizeBytes, &version.SHA256,
		&version.StorageProvider, &version.StorageKey, &version.ContentStatus,
		&version.Actor.Kind, &version.Actor.ID, &version.Actor.DisplayName, &version.RecordedAt,
	); err != nil {
		return Document{}, Version{}, err
	}
	return document, version, nil
}

type evidenceRows interface {
	Next() bool
	Scan(...any) error
	Err() error
}

func scanEvidenceDocuments(rows evidenceRows) ([]Document, error) {
	documents := make([]Document, 0)
	indexByID := make(map[string]int)
	for rows.Next() {
		document, version, err := scanEvidenceRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan contract evidence: %w", err)
		}
		index, found := indexByID[document.ID]
		if !found {
			index = len(documents)
			indexByID[document.ID] = index
			document.Versions = make([]Version, 0, 1)
			documents = append(documents, document)
		}
		documents[index].Versions = append(documents[index].Versions, version)
		documents[index].Latest = version.Version
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contract evidence: %w", err)
	}
	return documents, nil
}

func mapEvidenceInsertError(err error, command Command) error {
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && databaseError.ConstraintName == "contract_evidence_documents_target_fk" {
		return &TargetConflictError{TargetType: command.TargetType, TargetID: command.TargetID}
	}
	return fmt.Errorf("insert contract evidence: %w", err)
}
