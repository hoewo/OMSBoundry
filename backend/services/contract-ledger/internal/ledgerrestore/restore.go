package ledgerrestore

import (
	"context"
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
	"time"

	"omsbizfoundry/contract-ledger/internal/audit"
	"omsbizfoundry/contract-ledger/internal/ledgerexport"
)

const (
	ActionContractLedgerRestored = "contract_ledger_restored"
	MaxArchiveBytes              = int64(64 << 20)
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type Input struct {
	CommandID string
	Archive   []byte
	Actor     audit.ActorRef
}

type Candidate struct {
	Manifest          ledgerexport.Manifest
	Ledger            ledgerexport.Ledger
	LedgerSHA256      string
	SourceGeneratedAt time.Time
}

type Command struct {
	CommandID  string
	AuditID    string
	Candidate  Candidate
	Actor      audit.ActorRef
	RestoredAt time.Time
}

type Record struct {
	CommandID               string
	LedgerSHA256            string
	ContractID              string
	Status                  string
	CurrentRevision         int64
	MissingEvidenceVersions int
	Actor                   audit.ActorRef
	RestoredAt              time.Time
}

type Result struct {
	ContractID              string
	Status                  string
	CurrentRevision         int64
	MissingEvidenceVersions int
	Replayed                bool
}

type Repository interface {
	Restore(context.Context, Command) (Record, bool, error)
}

type Service struct {
	repository Repository
	now        func() time.Time
	newID      func() (string, error)
}

func NewService(repository Repository) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("ledger restore repository is required")
	}
	return &Service{
		repository: repository,
		now:        func() time.Time { return time.Now().UTC() },
		newID:      randomUUID,
	}, nil
}

func NewCommandID() (string, error) { return randomUUID() }

func (s *Service) Restore(ctx context.Context, input Input) (Result, error) {
	commandID := strings.ToLower(strings.TrimSpace(input.CommandID))
	if !uuidPattern.MatchString(commandID) {
		return Result{}, &ValidationError{Field: "command_id", Message: "恢复请求标识无效，请刷新页面后重试"}
	}
	actor, err := audit.NormalizeActor(input.Actor)
	if err != nil {
		return Result{}, &ValidationError{Field: "actor", Message: err.Error()}
	}
	candidate, err := ParseArchive(input.Archive)
	if err != nil {
		return Result{}, err
	}
	auditID, err := s.newID()
	if err != nil {
		return Result{}, fmt.Errorf("generate ledger restore audit ID: %w", err)
	}
	record, created, err := s.repository.Restore(ctx, Command{
		CommandID:  commandID,
		AuditID:    auditID,
		Candidate:  candidate,
		Actor:      actor,
		RestoredAt: s.now().UTC(),
	})
	if err != nil {
		return Result{}, fmt.Errorf("restore contract ledger: %w", err)
	}
	return Result{
		ContractID:              record.ContractID,
		Status:                  record.Status,
		CurrentRevision:         record.CurrentRevision,
		MissingEvidenceVersions: record.MissingEvidenceVersions,
		Replayed:                !created,
	}, nil
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

type TooLargeError struct{ Message string }

func (e *TooLargeError) Error() string {
	if strings.TrimSpace(e.Message) == "" {
		return "恢复归档超过大小限制"
	}
	return e.Message
}

type UnsupportedRelationsError struct{ Count int }

func (e *UnsupportedRelationsError) Error() string {
	return "单合同归档包含外部合同关系；请使用未来的多合同依赖恢复包"
}

type DestinationNotEmptyError struct{}

func (e *DestinationNotEmptyError) Error() string {
	return "目标业务库不为空，不能执行单合同恢复"
}

type DestinationConflictError struct{ Message string }

func (e *DestinationConflictError) Error() string {
	if strings.TrimSpace(e.Message) == "" {
		return "归档事实与目标库中的既有记录冲突"
	}
	return e.Message
}

type IdempotencyConflictError struct{ CommandID string }

func (e *IdempotencyConflictError) Error() string {
	return "恢复请求标识已经使用，但归档或操作者与首次请求不同"
}

func randomUUID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16]), nil
}
