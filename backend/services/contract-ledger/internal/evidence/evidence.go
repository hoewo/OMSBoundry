package evidence

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode"

	"omsbizfoundry/contract-ledger/internal/audit"
)

const (
	AuditActionCreated        = "contract_evidence_created"
	AuditActionVersionAdded   = "contract_evidence_version_added"
	AuditActionDownloaded     = "contract_evidence_downloaded"
	StorageProviderLocal      = "local"
	DefaultMaxUploadBytes     = int64(20 << 20)
	maxOriginalFilenameRunes  = 255
	maxEvidenceTitleRunes     = 200
	maxEvidencePurposeRunes   = 500
	maxDetectedMediaTypeRunes = 200
	candidateCleanupTimeout   = 10 * time.Second
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type TargetType string

const (
	TargetContract   TargetType = "contract"
	TargetInstrument TargetType = "instrument"
	TargetSettlement TargetType = "settlement"
	TargetClosure    TargetType = "closure"
)

func (t TargetType) Valid() bool {
	switch t {
	case TargetContract, TargetInstrument, TargetSettlement, TargetClosure:
		return true
	default:
		return false
	}
}

func (t TargetType) Label() string {
	switch t {
	case TargetContract:
		return "合同整体"
	case TargetInstrument:
		return "合同文书"
	case TargetSettlement:
		return "结算事实"
	case TargetClosure:
		return "操作性关闭"
	default:
		return string(t)
	}
}

type Document struct {
	ID          string
	ContractID  string
	TargetType  TargetType
	TargetID    string
	Title       string
	Purpose     string
	Actor       audit.ActorRef
	CreatedAt   time.Time
	Versions    []Version
	Latest      int64
	TargetLabel string
}

type Version struct {
	ID                string
	DocumentID        string
	Version           int64
	OriginalFilename  string
	DetectedMediaType string
	SizeBytes         int64
	SHA256            string
	StorageProvider   string
	StorageKey        string
	ContentStatus     string
	Actor             audit.ActorRef
	RecordedAt        time.Time
}

func (v Version) ContentAvailable() bool { return v.ContentStatus != "missing" }

type CommandAction string

const (
	CommandCreateDocument CommandAction = "create_document"
	CommandAddVersion     CommandAction = "add_version"
)

type Command struct {
	CommandID             string
	AuditID               string
	Action                CommandAction
	ContractID            string
	DocumentID            string
	VersionID             string
	TargetType            TargetType
	TargetID              string
	Title                 string
	Purpose               string
	ExpectedLatestVersion int64
	ResultingVersion      int64
	OriginalFilename      string
	DetectedMediaType     string
	SizeBytes             int64
	SHA256                string
	StorageProvider       string
	StorageKey            string
	Actor                 audit.ActorRef
	OccurredAt            time.Time
}

type CreateInput struct {
	CommandID        string
	ContractID       string
	TargetType       string
	TargetID         string
	Title            string
	Purpose          string
	OriginalFilename string
	Actor            audit.ActorRef
}

type AddVersionInput struct {
	CommandID             string
	ContractID            string
	DocumentID            string
	ExpectedLatestVersion int64
	OriginalFilename      string
	Actor                 audit.ActorRef
}

type UploadResult struct {
	Document Document
	Version  Version
	Replayed bool
}

type DownloadInput struct {
	ContractID string
	DocumentID string
	VersionID  string
	Actor      audit.ActorRef
}

type Download struct {
	Document Document
	Version  Version
	Content  ReadSeekCloser
}

type StoredBlob struct {
	StorageKey        string
	SizeBytes         int64
	SHA256            string
	DetectedMediaType string
}

type ReadSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

type BlobStore interface {
	Put(context.Context, string, io.Reader, int64) (StoredBlob, error)
	Open(context.Context, string) (ReadSeekCloser, error)
	Delete(context.Context, string) error
}

type Repository interface {
	Record(context.Context, Command) (Command, Document, Version, bool, error)
	List(context.Context, string) ([]Document, bool, error)
	FindVersion(context.Context, string, string, string) (Document, Version, bool, error)
	RecordDownload(context.Context, DownloadAudit) error
}

type DownloadAudit struct {
	AuditID    string
	AccessID   string
	ContractID string
	Document   Document
	Version    Version
	Actor      audit.ActorRef
	OccurredAt time.Time
}

type Service struct {
	repository     Repository
	store          BlobStore
	maxUploadBytes int64
	now            func() time.Time
	newID          func() (string, error)
}

func NewService(repository Repository, store BlobStore, maxUploadBytes int64) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("evidence repository is required")
	}
	if store == nil {
		return nil, fmt.Errorf("evidence blob store is required")
	}
	if maxUploadBytes < 1 {
		return nil, fmt.Errorf("evidence max upload bytes must be positive")
	}
	return &Service{
		repository:     repository,
		store:          store,
		maxUploadBytes: maxUploadBytes,
		now:            func() time.Time { return time.Now().UTC() },
		newID:          randomUUID,
	}, nil
}

func (s *Service) MaxUploadBytes() int64 { return s.maxUploadBytes }

func (s *Service) Create(ctx context.Context, input CreateInput, content io.Reader) (UploadResult, error) {
	normalized, err := normalizeCreateInput(input)
	if err != nil {
		return UploadResult{}, err
	}
	return s.upload(ctx, Command{
		CommandID:             normalized.CommandID,
		Action:                CommandCreateDocument,
		ContractID:            normalized.ContractID,
		TargetType:            TargetType(normalized.TargetType),
		TargetID:              normalized.TargetID,
		Title:                 normalized.Title,
		Purpose:               normalized.Purpose,
		ExpectedLatestVersion: 0,
		ResultingVersion:      1,
		OriginalFilename:      normalized.OriginalFilename,
		Actor:                 normalized.Actor,
	}, content)
}

func (s *Service) AddVersion(ctx context.Context, input AddVersionInput, content io.Reader) (UploadResult, error) {
	normalized, err := normalizeAddVersionInput(input)
	if err != nil {
		return UploadResult{}, err
	}
	return s.upload(ctx, Command{
		CommandID:             normalized.CommandID,
		Action:                CommandAddVersion,
		ContractID:            normalized.ContractID,
		DocumentID:            normalized.DocumentID,
		ExpectedLatestVersion: normalized.ExpectedLatestVersion,
		ResultingVersion:      normalized.ExpectedLatestVersion + 1,
		OriginalFilename:      normalized.OriginalFilename,
		Actor:                 normalized.Actor,
	}, content)
}

func (s *Service) upload(ctx context.Context, command Command, content io.Reader) (UploadResult, error) {
	if content == nil {
		return UploadResult{}, &ValidationError{Field: "file", Message: "必须选择文件"}
	}
	var err error
	if command.Action == CommandCreateDocument {
		command.DocumentID, err = s.newID()
		if err != nil {
			return UploadResult{}, fmt.Errorf("generate evidence document ID: %w", err)
		}
	}
	command.VersionID, err = s.newID()
	if err != nil {
		return UploadResult{}, fmt.Errorf("generate evidence version ID: %w", err)
	}
	command.AuditID, err = s.newID()
	if err != nil {
		return UploadResult{}, fmt.Errorf("generate evidence audit ID: %w", err)
	}
	command.StorageProvider = StorageProviderLocal
	command.StorageKey = command.VersionID
	command.OccurredAt = s.now()

	stored, err := s.store.Put(ctx, command.StorageKey, content, s.maxUploadBytes)
	if err != nil {
		return UploadResult{}, fmt.Errorf("store evidence content: %w", err)
	}
	command.StorageKey = stored.StorageKey
	command.SizeBytes = stored.SizeBytes
	command.SHA256 = stored.SHA256
	command.DetectedMediaType = stored.DetectedMediaType

	persistedCommand, document, version, created, err := s.repository.Record(ctx, command)
	if err != nil {
		return UploadResult{}, s.cleanupCandidate(ctx, command.StorageKey, fmt.Errorf("record evidence: %w", err))
	}
	if !created {
		if err := s.deleteCandidate(ctx, command.StorageKey); err != nil {
			return UploadResult{}, fmt.Errorf("remove replayed evidence candidate: %w", err)
		}
		if !sameIntent(persistedCommand, command) {
			return UploadResult{}, &IdempotencyConflictError{CommandID: command.CommandID}
		}
	}
	return UploadResult{Document: document, Version: version, Replayed: !created}, nil
}

func (s *Service) cleanupCandidate(ctx context.Context, storageKey string, cause error) error {
	if cleanupErr := s.deleteCandidate(ctx, storageKey); cleanupErr != nil {
		return fmt.Errorf("%w; cleanup candidate: %v", cause, cleanupErr)
	}
	return cause
}

func (s *Service) deleteCandidate(ctx context.Context, storageKey string) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), candidateCleanupTimeout)
	defer cancel()
	return s.store.Delete(cleanupCtx, storageKey)
}

func (s *Service) List(ctx context.Context, contractID string) ([]Document, error) {
	normalizedID, err := normalizeUUID("contract_id", "合同标识无效", contractID)
	if err != nil {
		return nil, err
	}
	documents, found, err := s.repository.List(ctx, normalizedID)
	if err != nil {
		return nil, fmt.Errorf("list contract evidence: %w", err)
	}
	if !found {
		return nil, &ContractNotFoundError{ContractID: normalizedID}
	}
	return documents, nil
}

func (s *Service) Open(ctx context.Context, input DownloadInput) (Download, error) {
	normalized, err := normalizeDownloadInput(input)
	if err != nil {
		return Download{}, err
	}
	document, version, found, err := s.repository.FindVersion(ctx, normalized.ContractID, normalized.DocumentID, normalized.VersionID)
	if err != nil {
		return Download{}, fmt.Errorf("find evidence version: %w", err)
	}
	if !found {
		return Download{}, &VersionNotFoundError{VersionID: normalized.VersionID}
	}
	if !version.ContentAvailable() {
		return Download{}, &UnavailableError{Message: "证据文件内容尚未恢复"}
	}
	content, err := s.store.Open(ctx, version.StorageKey)
	if err != nil {
		return Download{}, &UnavailableError{Message: "证据文件当前不可用"}
	}
	valid := false
	defer func() {
		if !valid {
			_ = content.Close()
		}
	}()
	hash := sha256.New()
	size, err := io.Copy(hash, content)
	if err != nil {
		return Download{}, &UnavailableError{Message: "读取证据文件失败"}
	}
	if size != version.SizeBytes || hex.EncodeToString(hash.Sum(nil)) != version.SHA256 {
		return Download{}, &IntegrityError{VersionID: version.ID}
	}
	if _, err := content.Seek(0, io.SeekStart); err != nil {
		return Download{}, &UnavailableError{Message: "重置证据文件读取位置失败"}
	}
	auditID, err := s.newID()
	if err != nil {
		return Download{}, fmt.Errorf("generate evidence download audit ID: %w", err)
	}
	accessID, err := s.newID()
	if err != nil {
		return Download{}, fmt.Errorf("generate evidence access ID: %w", err)
	}
	if err := s.repository.RecordDownload(ctx, DownloadAudit{
		AuditID: auditID, AccessID: accessID, ContractID: normalized.ContractID,
		Document: document, Version: version, Actor: normalized.Actor, OccurredAt: s.now(),
	}); err != nil {
		return Download{}, fmt.Errorf("record evidence download audit: %w", err)
	}
	valid = true
	return Download{Document: document, Version: version, Content: content}, nil
}

func normalizeCreateInput(input CreateInput) (CreateInput, error) {
	commandID, err := normalizeUUID("command_id", "命令标识无效，请刷新页面后重试", input.CommandID)
	if err != nil {
		return CreateInput{}, err
	}
	contractID, err := normalizeUUID("contract_id", "合同标识无效", input.ContractID)
	if err != nil {
		return CreateInput{}, err
	}
	targetType := TargetType(strings.ToLower(strings.TrimSpace(input.TargetType)))
	if !targetType.Valid() {
		return CreateInput{}, &ValidationError{Field: "target_type", Message: "证据引用目标类型无效"}
	}
	targetID, err := normalizeUUID("target_id", "证据引用目标无效", input.TargetID)
	if err != nil {
		return CreateInput{}, err
	}
	if targetType == TargetContract && targetID != contractID {
		return CreateInput{}, &TargetConflictError{TargetType: targetType, TargetID: targetID}
	}
	title, err := requiredText("title", "证据标题不能为空", input.Title, maxEvidenceTitleRunes)
	if err != nil {
		return CreateInput{}, err
	}
	purpose, err := requiredText("purpose", "证据用途或依据不能为空", input.Purpose, maxEvidencePurposeRunes)
	if err != nil {
		return CreateInput{}, err
	}
	filename, err := normalizeFilename(input.OriginalFilename)
	if err != nil {
		return CreateInput{}, err
	}
	actor, err := audit.NormalizeActor(input.Actor)
	if err != nil {
		return CreateInput{}, &ValidationError{Field: "actor", Message: err.Error()}
	}
	return CreateInput{
		CommandID: commandID, ContractID: contractID,
		TargetType: string(targetType), TargetID: targetID,
		Title: title, Purpose: purpose, OriginalFilename: filename, Actor: actor,
	}, nil
}

func normalizeAddVersionInput(input AddVersionInput) (AddVersionInput, error) {
	commandID, err := normalizeUUID("command_id", "命令标识无效，请刷新页面后重试", input.CommandID)
	if err != nil {
		return AddVersionInput{}, err
	}
	contractID, err := normalizeUUID("contract_id", "合同标识无效", input.ContractID)
	if err != nil {
		return AddVersionInput{}, err
	}
	documentID, err := normalizeUUID("document_id", "证据文档标识无效", input.DocumentID)
	if err != nil {
		return AddVersionInput{}, err
	}
	if input.ExpectedLatestVersion < 1 {
		return AddVersionInput{}, &ValidationError{Field: "expected_latest_version", Message: "证据最新版本无效，请刷新页面后重试"}
	}
	filename, err := normalizeFilename(input.OriginalFilename)
	if err != nil {
		return AddVersionInput{}, err
	}
	actor, err := audit.NormalizeActor(input.Actor)
	if err != nil {
		return AddVersionInput{}, &ValidationError{Field: "actor", Message: err.Error()}
	}
	return AddVersionInput{
		CommandID: commandID, ContractID: contractID, DocumentID: documentID,
		ExpectedLatestVersion: input.ExpectedLatestVersion,
		OriginalFilename:      filename, Actor: actor,
	}, nil
}

func normalizeDownloadInput(input DownloadInput) (DownloadInput, error) {
	contractID, err := normalizeUUID("contract_id", "合同标识无效", input.ContractID)
	if err != nil {
		return DownloadInput{}, err
	}
	documentID, err := normalizeUUID("document_id", "证据文档标识无效", input.DocumentID)
	if err != nil {
		return DownloadInput{}, err
	}
	versionID, err := normalizeUUID("version_id", "证据版本标识无效", input.VersionID)
	if err != nil {
		return DownloadInput{}, err
	}
	actor, err := audit.NormalizeActor(input.Actor)
	if err != nil {
		return DownloadInput{}, &ValidationError{Field: "actor", Message: err.Error()}
	}
	return DownloadInput{ContractID: contractID, DocumentID: documentID, VersionID: versionID, Actor: actor}, nil
}

func normalizeUUID(field, message, value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if !uuidPattern.MatchString(normalized) {
		return "", &ValidationError{Field: field, Message: message}
	}
	return normalized, nil
}

func requiredText(field, emptyMessage, value string, maxRunes int) (string, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", &ValidationError{Field: field, Message: emptyMessage}
	}
	if len([]rune(normalized)) > maxRunes {
		return "", &ValidationError{Field: field, Message: fmt.Sprintf("内容不能超过 %d 个字符", maxRunes)}
	}
	return normalized, nil
}

func normalizeFilename(value string) (string, error) {
	raw := strings.TrimSpace(strings.ReplaceAll(value, `\`, "/"))
	name := path.Base(raw)
	if name == "" || name == "." || name == "/" {
		return "", &ValidationError{Field: "file", Message: "文件名无效"}
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", &ValidationError{Field: "file", Message: "文件名不能包含控制字符"}
		}
	}
	if len([]rune(name)) > maxOriginalFilenameRunes {
		return "", &ValidationError{Field: "file", Message: "文件名不能超过 255 个字符"}
	}
	return name, nil
}

func sameIntent(left, right Command) bool {
	if left.CommandID != right.CommandID || left.Action != right.Action ||
		left.ContractID != right.ContractID || left.ExpectedLatestVersion != right.ExpectedLatestVersion ||
		left.OriginalFilename != right.OriginalFilename || left.DetectedMediaType != right.DetectedMediaType ||
		left.SizeBytes != right.SizeBytes || left.SHA256 != right.SHA256 || left.Actor != right.Actor {
		return false
	}
	if left.Action == CommandCreateDocument {
		return left.TargetType == right.TargetType && left.TargetID == right.TargetID &&
			left.Title == right.Title && left.Purpose == right.Purpose
	}
	return left.DocumentID == right.DocumentID
}

func randomUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

type ContractNotFoundError struct{ ContractID string }

func (e *ContractNotFoundError) Error() string { return "合同不存在" }

type TargetConflictError struct {
	TargetType TargetType
	TargetID   string
}

func (e *TargetConflictError) Error() string {
	return "证据引用目标不存在或不属于当前合同"
}

type DocumentNotFoundError struct{ DocumentID string }

func (e *DocumentNotFoundError) Error() string { return "证据文档不存在" }

type VersionNotFoundError struct{ VersionID string }

func (e *VersionNotFoundError) Error() string { return "证据版本不存在" }

type VersionConflictError struct {
	Expected int64
	Actual   int64
}

func (e *VersionConflictError) Error() string {
	return "证据已有更新版本，请确认最新内容后重试"
}

type IdempotencyConflictError struct{ CommandID string }

func (e *IdempotencyConflictError) Error() string {
	return "命令标识已经使用，但提交内容与首次请求不同"
}

type EmptyFileError struct{}

func (e *EmptyFileError) Error() string { return "文件内容不能为空" }

type FileTooLargeError struct{ Limit int64 }

func (e *FileTooLargeError) Error() string {
	return fmt.Sprintf("文件不能超过 %d 字节", e.Limit)
}

type IntegrityError struct{ VersionID string }

func (e *IntegrityError) Error() string { return "证据文件完整性校验失败" }

type UnavailableError struct{ Message string }

func (e *UnavailableError) Error() string { return e.Message }

func IsClientError(err error) bool {
	var validation *ValidationError
	var target *TargetConflictError
	var document *DocumentNotFoundError
	var version *VersionNotFoundError
	var versionConflict *VersionConflictError
	var idempotency *IdempotencyConflictError
	var empty *EmptyFileError
	var tooLarge *FileTooLargeError
	return errors.As(err, &validation) || errors.As(err, &target) || errors.As(err, &document) ||
		errors.As(err, &version) || errors.As(err, &versionConflict) || errors.As(err, &idempotency) ||
		errors.As(err, &empty) || errors.As(err, &tooLarge)
}
