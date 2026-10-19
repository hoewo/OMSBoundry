package ledgerexport

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"omsbizfoundry/contract-ledger/internal/audit"
)

const (
	FormatID                     = "omsbizfoundry.contract-ledger/contract-export-v1"
	FormatVersion                = 1
	ActionContractLedgerExported = "contract_ledger_exported"
	manifestFilename             = "manifest.json"
	ledgerFilename               = "ledger.json"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type Actor struct {
	Kind        string `json:"kind"`
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type ContractSnapshot struct {
	ID              string  `json:"id"`
	NumberNamespace string  `json:"number_namespace"`
	ContractNumber  string  `json:"contract_number"`
	Title           string  `json:"title"`
	Category        *string `json:"category"`
	Currency        string  `json:"currency"`
	TotalAmount     *string `json:"total_amount"`
	SignedOn        *string `json:"signed_on"`
	EffectiveOn     *string `json:"effective_on"`
	Status          string  `json:"status"`
	DraftVersion    int64   `json:"draft_version"`
	CurrentRevision *int64  `json:"current_revision"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

type DraftParty struct {
	ID                     string  `json:"id"`
	PartyID                string  `json:"party_id"`
	PartyKind              string  `json:"party_kind"`
	PartyLegalName         string  `json:"party_legal_name"`
	RegistrationIdentifier *string `json:"registration_identifier"`
	PartyRevision          int64   `json:"party_revision"`
	Role                   string  `json:"role"`
	AddedAt                string  `json:"added_at"`
	RemovedAt              *string `json:"removed_at"`
}

type DraftCancellation struct {
	CommandID             string `json:"command_id"`
	ExpectedDraftVersion  int64  `json:"expected_draft_version"`
	ResultingDraftVersion int64  `json:"resulting_draft_version"`
	Reason                string `json:"reason"`
	Actor                 Actor  `json:"actor"`
	CancelledAt           string `json:"cancelled_at"`
}

type PartySnapshot struct {
	Position               int64   `json:"position"`
	SourcePartyLinkID      string  `json:"source_party_link_id"`
	SourcePartyID          string  `json:"source_party_id"`
	SourcePartyRevision    int64   `json:"source_party_revision"`
	PartyKind              string  `json:"party_kind"`
	LegalName              string  `json:"legal_name"`
	RegistrationIdentifier *string `json:"registration_identifier"`
	Role                   string  `json:"role"`
}

type Revision struct {
	Revision            int64           `json:"revision"`
	ActivationCommandID *string         `json:"activation_command_id"`
	SourceDraftVersion  *int64          `json:"source_draft_version"`
	SourceInstrumentID  string          `json:"source_instrument_id"`
	NumberNamespace     string          `json:"number_namespace"`
	ContractNumber      string          `json:"contract_number"`
	Title               string          `json:"title"`
	Category            *string         `json:"category"`
	Currency            string          `json:"currency"`
	TotalAmount         *string         `json:"total_amount"`
	SignedOn            string          `json:"signed_on"`
	EffectiveOn         string          `json:"effective_on"`
	Actor               Actor           `json:"actor"`
	RecordedAt          string          `json:"recorded_at"`
	Parties             []PartySnapshot `json:"parties"`
}

type Instrument struct {
	ID                string  `json:"id"`
	InstrumentType    string  `json:"instrument_type"`
	InstrumentNumber  string  `json:"instrument_number"`
	Title             string  `json:"title"`
	ChangeSummary     string  `json:"change_summary"`
	SignedOn          string  `json:"signed_on"`
	EffectiveOn       string  `json:"effective_on"`
	EffectiveOrder    int64   `json:"effective_order"`
	AmountEffect      string  `json:"amount_effect"`
	AmountValue       *string `json:"amount_value"`
	SourceCommandID   string  `json:"source_command_id"`
	ResultingRevision int64   `json:"resulting_revision"`
	Actor             Actor   `json:"actor"`
	RecordedAt        string  `json:"recorded_at"`
}

type RelationContractSnapshot struct {
	ContractID      string `json:"contract_id"`
	Revision        int64  `json:"revision"`
	NumberNamespace string `json:"number_namespace"`
	ContractNumber  string `json:"contract_number"`
	Title           string `json:"title"`
}

type Relation struct {
	ID          string                   `json:"id"`
	Type        string                   `json:"type"`
	Source      RelationContractSnapshot `json:"source"`
	Target      RelationContractSnapshot `json:"target"`
	EffectiveOn *string                  `json:"effective_on"`
	Note        *string                  `json:"note"`
	Actor       Actor                    `json:"actor"`
	CreatedAt   string                   `json:"created_at"`
}

type Settlement struct {
	ID                string          `json:"id"`
	SourceRevision    int64           `json:"source_revision"`
	Sequence          int64           `json:"sequence"`
	Kind              string          `json:"kind"`
	StatementNumber   string          `json:"statement_number"`
	Title             string          `json:"title"`
	Amount            string          `json:"amount"`
	Currency          string          `json:"currency"`
	ConfirmedOn       string          `json:"confirmed_on"`
	Basis             string          `json:"basis"`
	ConfirmingParties []PartySnapshot `json:"confirming_parties"`
	Actor             Actor           `json:"actor"`
	RecordedAt        string          `json:"recorded_at"`
}

type Closure struct {
	ID                       string  `json:"id"`
	SourceRevision           int64   `json:"source_revision"`
	SourceSettlementSequence int64   `json:"source_settlement_sequence"`
	SettlementDisposition    string  `json:"settlement_disposition"`
	FinalSettlementID        *string `json:"final_settlement_id"`
	ClosedOn                 string  `json:"closed_on"`
	Reason                   string  `json:"reason"`
	Actor                    Actor   `json:"actor"`
	RecordedAt               string  `json:"recorded_at"`
}

type EvidenceVersion struct {
	ID                string `json:"id"`
	Version           int64  `json:"version"`
	OriginalFilename  string `json:"original_filename"`
	DetectedMediaType string `json:"detected_media_type"`
	SizeBytes         int64  `json:"size_bytes"`
	SHA256            string `json:"sha256"`
	ContentAvailable  bool   `json:"content_available"`
	Actor             Actor  `json:"actor"`
	RecordedAt        string `json:"recorded_at"`
}

type EvidenceDocument struct {
	ID         string            `json:"id"`
	TargetType string            `json:"target_type"`
	TargetID   string            `json:"target_id"`
	Title      string            `json:"title"`
	Purpose    string            `json:"purpose"`
	Actor      Actor             `json:"actor"`
	CreatedAt  string            `json:"created_at"`
	Versions   []EvidenceVersion `json:"versions"`
}

type AuditEntry struct {
	ID         string          `json:"id"`
	Action     string          `json:"action"`
	CommandID  *string         `json:"command_id"`
	Actor      Actor           `json:"actor"`
	OccurredAt string          `json:"occurred_at"`
	Summary    string          `json:"summary"`
	Details    json.RawMessage `json:"details"`
}

type Ledger struct {
	Format       string             `json:"format"`
	Contract     ContractSnapshot   `json:"contract"`
	Cancellation *DraftCancellation `json:"cancellation"`
	DraftParties []DraftParty       `json:"draft_parties"`
	Revisions    []Revision         `json:"revisions"`
	Instruments  []Instrument       `json:"instruments"`
	Relations    []Relation         `json:"relations"`
	Settlements  []Settlement       `json:"settlements"`
	Closure      *Closure           `json:"closure"`
	Evidence     []EvidenceDocument `json:"evidence"`
	Audit        []AuditEntry       `json:"audit"`
}

type Manifest struct {
	FormatID                string   `json:"format_id"`
	FormatVersion           int      `json:"format_version"`
	GeneratedAt             string   `json:"generated_at"`
	ContractID              string   `json:"contract_id"`
	LedgerFilename          string   `json:"ledger_filename"`
	LedgerBytes             int      `json:"ledger_bytes"`
	LedgerSHA256            string   `json:"ledger_sha256"`
	EvidenceContentIncluded bool     `json:"evidence_content_included"`
	ExcludedContent         []string `json:"excluded_content"`
}

type Input struct {
	ContractID string
	Actor      audit.ActorRef
}

type Artifact struct {
	Filename     string
	Content      []byte
	LedgerSHA256 string
	GeneratedAt  time.Time
}

type NotFoundError struct{ ContractID string }

func (e *NotFoundError) Error() string { return "合同不存在" }

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

type Repository interface {
	Load(context.Context, string) (Ledger, bool, error)
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
	contractID := strings.ToLower(strings.TrimSpace(input.ContractID))
	if !uuidPattern.MatchString(contractID) {
		return Artifact{}, &ValidationError{Field: "contract_id", Message: "合同标识无效"}
	}
	actor, err := audit.NormalizeActor(input.Actor)
	if err != nil {
		return Artifact{}, &ValidationError{Field: "actor", Message: err.Error()}
	}

	ledger, found, err := s.repository.Load(ctx, contractID)
	if err != nil {
		return Artifact{}, fmt.Errorf("load contract ledger export: %w", err)
	}
	if !found {
		return Artifact{}, &NotFoundError{ContractID: contractID}
	}
	ledger.Format = FormatID
	normalizeCollections(&ledger)
	ledgerBytes, err := marshalJSONFile(ledger)
	if err != nil {
		return Artifact{}, fmt.Errorf("encode contract ledger: %w", err)
	}
	digestBytes := sha256.Sum256(ledgerBytes)
	digest := hex.EncodeToString(digestBytes[:])
	generatedAt := s.now().UTC()
	manifest := Manifest{
		FormatID:                FormatID,
		FormatVersion:           FormatVersion,
		GeneratedAt:             generatedAt.Format(time.RFC3339Nano),
		ContractID:              contractID,
		LedgerFilename:          ledgerFilename,
		LedgerBytes:             len(ledgerBytes),
		LedgerSHA256:            digest,
		EvidenceContentIncluded: false,
		ExcludedContent: []string{
			"identity credentials and sessions",
			"evidence binary content",
			"evidence storage keys and server paths",
			"internal idempotency payload hashes",
		},
	}
	manifestBytes, err := marshalJSONFile(manifest)
	if err != nil {
		return Artifact{}, fmt.Errorf("encode contract ledger manifest: %w", err)
	}
	archiveBytes, err := buildArchive(generatedAt, manifestBytes, ledgerBytes)
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
	currentRevision := any(nil)
	if ledger.Contract.CurrentRevision != nil {
		currentRevision = *ledger.Contract.CurrentRevision
	}
	if err := s.repository.RecordExport(ctx, audit.Entry{
		ID:         auditID,
		ObjectType: audit.ObjectContract,
		ObjectID:   contractID,
		Action:     ActionContractLedgerExported,
		CommandID:  requestID,
		Actor:      actor,
		OccurredAt: generatedAt,
		Summary:    "导出单合同可验证账簿",
		Details: map[string]any{
			"format_id":          FormatID,
			"ledger_sha256":      digest,
			"ledger_bytes":       len(ledgerBytes),
			"archive_bytes":      len(archiveBytes),
			"snapshot_status":    ledger.Contract.Status,
			"current_revision":   currentRevision,
			"evidence_documents": len(ledger.Evidence),
			"evidence_versions":  countEvidenceVersions(ledger.Evidence),
		},
	}); err != nil {
		return Artifact{}, fmt.Errorf("record contract ledger export audit: %w", err)
	}

	return Artifact{
		Filename:     "contract-" + contractID + "-ledger.zip",
		Content:      archiveBytes,
		LedgerSHA256: digest,
		GeneratedAt:  generatedAt,
	}, nil
}

func marshalJSONFile(value any) ([]byte, error) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func buildArchive(generatedAt time.Time, manifest, ledger []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, file := range []struct {
		name    string
		content []byte
	}{
		{name: manifestFilename, content: manifest},
		{name: ledgerFilename, content: ledger},
	} {
		header := &zip.FileHeader{Name: file.name, Method: zip.Deflate}
		header.SetMode(0o600)
		header.SetModTime(generatedAt)
		entry, err := writer.CreateHeader(header)
		if err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("create %s in contract ledger archive: %w", file.name, err)
		}
		if _, err := entry.Write(file.content); err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("write %s in contract ledger archive: %w", file.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("finish contract ledger archive: %w", err)
	}
	return buffer.Bytes(), nil
}

func countEvidenceVersions(documents []EvidenceDocument) int {
	count := 0
	for _, document := range documents {
		count += len(document.Versions)
	}
	return count
}

func normalizeCollections(ledger *Ledger) {
	if ledger.DraftParties == nil {
		ledger.DraftParties = make([]DraftParty, 0)
	}
	if ledger.Revisions == nil {
		ledger.Revisions = make([]Revision, 0)
	}
	for index := range ledger.Revisions {
		if ledger.Revisions[index].Parties == nil {
			ledger.Revisions[index].Parties = make([]PartySnapshot, 0)
		}
	}
	if ledger.Instruments == nil {
		ledger.Instruments = make([]Instrument, 0)
	}
	if ledger.Relations == nil {
		ledger.Relations = make([]Relation, 0)
	}
	if ledger.Settlements == nil {
		ledger.Settlements = make([]Settlement, 0)
	}
	for index := range ledger.Settlements {
		if ledger.Settlements[index].ConfirmingParties == nil {
			ledger.Settlements[index].ConfirmingParties = make([]PartySnapshot, 0)
		}
	}
	if ledger.Evidence == nil {
		ledger.Evidence = make([]EvidenceDocument, 0)
	}
	for index := range ledger.Evidence {
		if ledger.Evidence[index].Versions == nil {
			ledger.Evidence[index].Versions = make([]EvidenceVersion, 0)
		}
	}
	if ledger.Audit == nil {
		ledger.Audit = make([]AuditEntry, 0)
	}
}

func randomUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		value[0:4], value[4:6], value[6:8], value[8:10], value[10:16],
	), nil
}
