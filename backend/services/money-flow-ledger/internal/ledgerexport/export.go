package ledgerexport

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

const (
	FormatID                      = "omsbizfoundry.money-flow-ledger/owner-ledger-export-v2"
	FormatVersion                 = 2
	ActionMoneyFlowLedgerExported = "money_flow_ledger_exported"
	manifestFilename              = "manifest.json"
	csvEncoding                   = "UTF-8, RFC 4180, CRLF, header row, exact text values, empty cell means NULL"
	spreadsheetFormulaPolicy      = "raw values are preserved; import every column as text and do not directly execute CSV cells"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type tableSpec struct {
	Filename string
	Header   []string
}

var tableSpecs = []tableSpec{
	{Filename: "parties.csv", Header: []string{
		"id", "kind", "legal_name", "registration_identifier", "revision", "created_at", "updated_at",
	}},
	{Filename: "obligations.csv", Header: []string{
		"id", "direction", "title", "ledger_owner_party_id",
		"creditor_party_id", "creditor_party_revision", "creditor_kind", "creditor_legal_name", "creditor_registration_identifier",
		"debtor_party_id", "debtor_party_revision", "debtor_kind", "debtor_legal_name", "debtor_registration_identifier",
		"original_amount", "currency", "due_on", "source_kind", "manual_source_reference", "status", "draft_version", "current_revision",
		"created_at", "updated_at",
	}},
	{Filename: "obligation_revisions.csv", Header: []string{
		"obligation_id", "revision", "direction", "title", "ledger_owner_party_id",
		"creditor_party_id", "creditor_party_revision", "creditor_kind", "creditor_legal_name", "creditor_registration_identifier",
		"debtor_party_id", "debtor_party_revision", "debtor_kind", "debtor_legal_name", "debtor_registration_identifier",
		"effective_amount", "currency", "due_on", "source_kind", "manual_source_reference", "source_draft_version", "revision_kind", "source_revision",
		"recorded_by_actor_kind", "recorded_by_actor_id", "recorded_by_actor_display_name", "recorded_at",
	}},
	{Filename: "obligation_amount_adjustments.csv", Header: []string{
		"id", "obligation_id", "source_revision", "resulting_revision", "adjustment_type", "adjustment_amount",
		"previous_effective_amount", "resulting_effective_amount", "currency", "source_kind", "adjustment_reason",
		"adjusted_by_actor_kind", "adjusted_by_actor_id", "adjusted_by_actor_display_name", "adjusted_at",
	}},
	{Filename: "obligation_remissions.csv", Header: []string{
		"id", "obligation_id", "obligation_revision", "amount", "recorded_effective_amount", "currency", "source_kind",
		"remission_reason", "manual_source_reference", "recorded_by_actor_kind", "recorded_by_actor_id", "recorded_by_actor_display_name", "recorded_at",
	}},
	{Filename: "obligation_closures.csv", Header: []string{
		"id", "obligation_id", "obligation_revision", "balance_sequence", "recorded_effective_amount", "net_allocated_amount",
		"remitted_amount", "currency", "settlement_basis", "source_kind", "closure_reason",
		"closed_by_actor_kind", "closed_by_actor_id", "closed_by_actor_display_name", "closed_at",
	}},
	{Filename: "obligation_cash_expectations.csv", Header: []string{
		"id", "obligation_id", "sequence", "action", "expected_on", "obligation_revision", "source_kind",
		"expectation_reason", "recorded_by_actor_kind", "recorded_by_actor_id", "recorded_by_actor_display_name", "recorded_at",
	}},
	{Filename: "cash_transactions.csv", Header: []string{
		"id", "type", "description", "ledger_owner_party_id",
		"payer_party_id", "payer_party_revision", "payer_kind", "payer_legal_name", "payer_registration_identifier",
		"payee_party_id", "payee_party_revision", "payee_kind", "payee_legal_name", "payee_registration_identifier",
		"amount", "currency", "occurred_on", "account_reference", "source_kind", "manual_source_reference", "external_transaction_reference",
		"recorded_by_actor_kind", "recorded_by_actor_id", "recorded_by_actor_display_name", "recorded_at",
	}},
	{Filename: "cash_transaction_reversals.csv", Header: []string{
		"id", "cash_transaction_id", "transaction_type", "amount", "currency", "source_kind", "reversal_reason",
		"reversed_by_actor_kind", "reversed_by_actor_id", "reversed_by_actor_display_name", "reversed_at",
	}},
	{Filename: "allocations.csv", Header: []string{
		"id", "cash_transaction_id", "obligation_id", "obligation_revision", "amount", "currency", "source_kind", "manual_source_reference",
		"applied_by_actor_kind", "applied_by_actor_id", "applied_by_actor_display_name", "applied_at",
	}},
	{Filename: "allocation_reversals.csv", Header: []string{
		"id", "allocation_id", "cash_transaction_id", "obligation_id", "obligation_revision", "amount", "currency", "source_kind", "reversal_reason",
		"reversed_by_actor_kind", "reversed_by_actor_id", "reversed_by_actor_display_name", "reversed_at",
	}},
	{Filename: "internal_transfers.csv", Header: []string{
		"id", "description", "ledger_owner_party_id", "ledger_owner_party_revision", "ledger_owner_kind", "ledger_owner_legal_name",
		"ledger_owner_registration_identifier", "amount", "currency", "occurred_on", "source_account_reference", "destination_account_reference",
		"source_kind", "manual_source_reference", "external_transfer_reference",
		"recorded_by_actor_kind", "recorded_by_actor_id", "recorded_by_actor_display_name", "recorded_at",
	}},
	{Filename: "audit_entries.csv", Header: []string{
		"id", "object_type", "object_id", "action", "command_id", "actor_kind", "actor_id", "actor_display_name",
		"occurred_at", "summary", "details_json",
	}},
}

type Owner struct {
	ID                     string `json:"id"`
	Kind                   string `json:"kind"`
	LegalName              string `json:"legal_name"`
	RegistrationIdentifier string `json:"registration_identifier,omitempty"`
	Revision               int64  `json:"revision"`
}

type Table struct {
	Filename string
	Rows     [][]string
}

type Snapshot struct {
	Owner         Owner
	SchemaVersion string
	Tables        []Table
}

type ManifestFile struct {
	Filename    string `json:"filename"`
	MediaType   string `json:"media_type"`
	RecordCount int    `json:"record_count"`
	Bytes       int    `json:"bytes"`
	SHA256      string `json:"sha256"`
}

type Manifest struct {
	FormatID                 string         `json:"format_id"`
	FormatVersion            int            `json:"format_version"`
	GeneratedAt              string         `json:"generated_at"`
	LedgerOwner              Owner          `json:"ledger_owner"`
	SourceSchemaVersion      string         `json:"source_schema_version"`
	CSVEncoding              string         `json:"csv_encoding"`
	SpreadsheetFormulaPolicy string         `json:"spreadsheet_formula_policy"`
	Files                    []ManifestFile `json:"files"`
	ExcludedContent          []string       `json:"excluded_content"`
}

type Input struct {
	LedgerOwnerPartyID string
	Actor              audit.ActorRef
}

type Artifact struct {
	Filename    string
	Content     []byte
	GeneratedAt time.Time
	Manifest    Manifest
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

type NotFoundError struct{ LedgerOwnerPartyID string }

func (e *NotFoundError) Error() string { return "本方账簿主体不存在" }

type SnapshotInvariantError struct{ Message string }

func (e *SnapshotInvariantError) Error() string { return e.Message }

type Repository interface {
	Load(context.Context, string) (Snapshot, bool, error)
	RecordExport(context.Context, audit.Entry) error
}

type Service struct {
	repository Repository
	now        func() time.Time
	newID      func() (string, error)
}

func NewService(repository Repository) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("ledger export repository is required")
	}
	return &Service{
		repository: repository,
		now:        func() time.Time { return time.Now().UTC() },
		newID:      randomUUID,
	}, nil
}

func (s *Service) Export(ctx context.Context, input Input) (Artifact, error) {
	ownerID := strings.ToLower(strings.TrimSpace(input.LedgerOwnerPartyID))
	if !uuidPattern.MatchString(ownerID) {
		return Artifact{}, &ValidationError{Field: "ledger_owner_party_id", Message: "本方账簿主体标识无效"}
	}
	actor, err := audit.NormalizeActor(input.Actor)
	if err != nil {
		return Artifact{}, &ValidationError{Field: "actor", Message: err.Error()}
	}

	snapshot, found, err := s.repository.Load(ctx, ownerID)
	if err != nil {
		return Artifact{}, fmt.Errorf("load money flow ledger export: %w", err)
	}
	if !found {
		return Artifact{}, &NotFoundError{LedgerOwnerPartyID: ownerID}
	}
	orderedTables, err := validateSnapshot(snapshot, ownerID)
	if err != nil {
		return Artifact{}, err
	}

	generatedAt := s.now().UTC()
	files := make([]ManifestFile, 0, len(tableSpecs))
	contents := make(map[string][]byte, len(tableSpecs))
	for index, spec := range tableSpecs {
		content, err := encodeCSV(spec.Header, orderedTables[index].Rows)
		if err != nil {
			return Artifact{}, fmt.Errorf("encode %s: %w", spec.Filename, err)
		}
		digestBytes := sha256.Sum256(content)
		files = append(files, ManifestFile{
			Filename:    spec.Filename,
			MediaType:   "text/csv; charset=utf-8",
			RecordCount: len(orderedTables[index].Rows),
			Bytes:       len(content),
			SHA256:      hex.EncodeToString(digestBytes[:]),
		})
		contents[spec.Filename] = content
	}

	manifest := Manifest{
		FormatID:                 FormatID,
		FormatVersion:            FormatVersion,
		GeneratedAt:              generatedAt.Format(time.RFC3339Nano),
		LedgerOwner:              snapshot.Owner,
		SourceSchemaVersion:      snapshot.SchemaVersion,
		CSVEncoding:              csvEncoding,
		SpreadsheetFormulaPolicy: spreadsheetFormulaPolicy,
		Files:                    files,
		ExcludedContent: []string{
			"identity credentials, users, sessions, lockout state, and login audit",
			"idempotency command rows and payload hashes",
			"business facts owned by other ledger owner parties",
			"database configuration, deployment secrets, and runtime logs",
			"derived current summaries, aging reports, and balances",
			"refunds, bad debt, billing documents, banking, and accounting facts not implemented by this system",
			"Contract Ledger and all external-system data",
		},
	}
	manifestBytes, err := marshalJSONFile(manifest)
	if err != nil {
		return Artifact{}, fmt.Errorf("encode money flow ledger manifest: %w", err)
	}
	archiveBytes, err := buildArchive(generatedAt, manifestBytes, contents)
	if err != nil {
		return Artifact{}, err
	}

	requestID, err := s.newID()
	if err != nil {
		return Artifact{}, fmt.Errorf("generate ledger export request ID: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return Artifact{}, fmt.Errorf("generate ledger export audit ID: %w", err)
	}
	if err := s.repository.RecordExport(ctx, audit.Entry{
		ID:         auditID,
		ObjectType: audit.ObjectLedgerExport,
		ObjectID:   ownerID,
		Action:     ActionMoneyFlowLedgerExported,
		CommandID:  requestID,
		Actor:      actor,
		OccurredAt: generatedAt,
		Summary:    "导出本方主体可校验 Money Flow 台账",
		Details: map[string]any{
			"format_id":             FormatID,
			"ledger_owner_party_id": ownerID,
			"source_schema_version": snapshot.SchemaVersion,
			"archive_bytes":         len(archiveBytes),
			"files":                 files,
		},
	}); err != nil {
		return Artifact{}, fmt.Errorf("record money flow ledger export audit: %w", err)
	}

	return Artifact{
		Filename:    "money-flow-ledger-" + ownerID + ".zip",
		Content:     archiveBytes,
		GeneratedAt: generatedAt,
		Manifest:    manifest,
	}, nil
}

func validateSnapshot(snapshot Snapshot, ownerID string) ([]Table, error) {
	if snapshot.Owner.ID != ownerID || !uuidPattern.MatchString(snapshot.Owner.ID) {
		return nil, &SnapshotInvariantError{Message: "导出快照的本方主体标识不一致"}
	}
	if snapshot.Owner.Kind != "organization" && snapshot.Owner.Kind != "individual" {
		return nil, &SnapshotInvariantError{Message: "导出快照的本方主体类型无效"}
	}
	if strings.TrimSpace(snapshot.Owner.LegalName) == "" || snapshot.Owner.Revision < 1 {
		return nil, &SnapshotInvariantError{Message: "导出快照的本方主体资料无效"}
	}
	if strings.TrimSpace(snapshot.SchemaVersion) == "" {
		return nil, &SnapshotInvariantError{Message: "导出快照缺少 schema version"}
	}
	if len(snapshot.Tables) != len(tableSpecs) {
		return nil, &SnapshotInvariantError{Message: "导出快照的表集合不完整"}
	}
	byFilename := make(map[string]Table, len(snapshot.Tables))
	for _, table := range snapshot.Tables {
		if _, exists := byFilename[table.Filename]; exists {
			return nil, &SnapshotInvariantError{Message: "导出快照包含重复表"}
		}
		byFilename[table.Filename] = table
	}
	ordered := make([]Table, 0, len(tableSpecs))
	for _, spec := range tableSpecs {
		table, ok := byFilename[spec.Filename]
		if !ok {
			return nil, &SnapshotInvariantError{Message: "导出快照缺少固定表 " + spec.Filename}
		}
		for _, row := range table.Rows {
			if len(row) != len(spec.Header) {
				return nil, &SnapshotInvariantError{Message: "导出快照表 " + spec.Filename + " 的列数无效"}
			}
		}
		ordered = append(ordered, table)
	}
	return ordered, nil
}

func encodeCSV(header []string, rows [][]string) ([]byte, error) {
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	writer.UseCRLF = true
	if err := writer.Write(header); err != nil {
		return nil, err
	}
	if err := writer.WriteAll(rows); err != nil {
		return nil, err
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func marshalJSONFile(value any) ([]byte, error) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func buildArchive(generatedAt time.Time, manifest []byte, contents map[string][]byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entries := make([]struct {
		name    string
		content []byte
	}, 0, len(tableSpecs)+1)
	entries = append(entries, struct {
		name    string
		content []byte
	}{name: manifestFilename, content: manifest})
	for _, spec := range tableSpecs {
		content, ok := contents[spec.Filename]
		if !ok {
			_ = writer.Close()
			return nil, fmt.Errorf("missing archive content for %s", spec.Filename)
		}
		entries = append(entries, struct {
			name    string
			content []byte
		}{name: spec.Filename, content: content})
	}
	for _, file := range entries {
		header := &zip.FileHeader{Name: file.name, Method: zip.Deflate}
		header.SetMode(0o600)
		header.SetModTime(generatedAt)
		entry, err := writer.CreateHeader(header)
		if err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("create %s in money flow ledger archive: %w", file.name, err)
		}
		if _, err := entry.Write(file.content); err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("write %s in money flow ledger archive: %w", file.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("finish money flow ledger archive: %w", err)
	}
	return buffer.Bytes(), nil
}

func randomUUID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}
