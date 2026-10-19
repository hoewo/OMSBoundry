package receiptset

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

const (
	StatusConfirmed      = "confirmed"
	CurrencyCNY          = "CNY"
	AuditActionConfirmed = "receipt_set_confirmed"
)

var (
	uuidPattern        = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	ErrInvalid         = errors.New("receipt set input is invalid")
	ErrNotFound        = errors.New("receipt set not found")
	ErrStateConflict   = errors.New("receipt set source is not confirmable")
	ErrCommandConflict = errors.New("receipt set command id was reused for another intent")
)

type Item struct {
	TransactionID       string `json:"transaction_id"`
	TransactionRevision int64  `json:"transaction_revision"`
	AllocationID        string `json:"allocation_id"`
	AllocationRevision  int64  `json:"allocation_revision"`
	AmountMinor         int64  `json:"amount_minor"`
	Currency            string `json:"currency"`
	Status              string `json:"status"`
}

type Set struct {
	ID               string         `json:"id"`
	TenantID         string         `json:"tenant_id"`
	ProjectID        string         `json:"project_id"`
	Status           string         `json:"status"`
	Currency         string         `json:"currency"`
	Revision         int64          `json:"revision"`
	TotalAmountMinor int64          `json:"total_amount_minor"`
	Items            []Item         `json:"items"`
	ConfirmedBy      audit.ActorRef `json:"-"`
	ConfirmedAt      time.Time      `json:"-"`
}

type ConfirmInput struct {
	CommandID     string         `json:"command_id"`
	ProjectID     string         `json:"project_id"`
	AllocationIDs []string       `json:"allocation_ids"`
	TenantID      string         `json:"-"`
	Actor         audit.ActorRef `json:"-"`
}

type ConfirmResult struct {
	Set      Set  `json:"receipt_set"`
	Replayed bool `json:"replayed"`
}

type Command struct {
	ID          string
	PayloadHash []byte
	Actor       audit.ActorRef
	OccurredAt  time.Time
}

type Repository interface {
	Confirm(context.Context, Set, []string, Command, audit.Entry) (Set, bool, error)
	Get(context.Context, string, string) (Set, error)
}

type Service struct {
	repository Repository
	now        func() time.Time
	newID      func() (string, error)
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository, now: func() time.Time { return time.Now().UTC() }, newID: randomUUID}
}

func (s *Service) Confirm(ctx context.Context, input ConfirmInput) (ConfirmResult, error) {
	normalized, err := normalize(input)
	if err != nil {
		return ConfirmResult{}, err
	}
	digest, err := json.Marshal(struct {
		Format, TenantID, ProjectID string
		AllocationIDs               []string
	}{"money-receipt-set-confirm-v1", normalized.TenantID, normalized.ProjectID, normalized.AllocationIDs})
	if err != nil {
		return ConfirmResult{}, err
	}
	hash := sha256.Sum256(digest)
	setID, err := s.newID()
	if err != nil {
		return ConfirmResult{}, fmt.Errorf("generate receipt set id: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return ConfirmResult{}, fmt.Errorf("generate receipt set audit id: %w", err)
	}
	now := s.now()
	value := Set{ID: setID, TenantID: normalized.TenantID, ProjectID: normalized.ProjectID, Status: StatusConfirmed, Currency: CurrencyCNY, Revision: 1, ConfirmedBy: normalized.Actor, ConfirmedAt: now}
	persisted, replayed, err := s.repository.Confirm(ctx, value, normalized.AllocationIDs, Command{ID: normalized.CommandID, PayloadHash: hash[:], Actor: normalized.Actor, OccurredAt: now}, audit.Entry{
		ID: auditID, ObjectType: audit.ObjectReceiptSet, ObjectID: setID, Action: AuditActionConfirmed, CommandID: normalized.CommandID,
		Actor: normalized.Actor, OccurredAt: now, Summary: "确认项目回款集合",
		Details: map[string]any{"tenant_id": normalized.TenantID, "project_id": normalized.ProjectID, "allocation_ids": normalized.AllocationIDs},
	})
	if err != nil {
		return ConfirmResult{}, err
	}
	return ConfirmResult{Set: persisted, Replayed: replayed}, nil
}

func (s *Service) Get(ctx context.Context, tenantID, setID string) (Set, error) {
	tenantID, setID = strings.ToLower(strings.TrimSpace(tenantID)), strings.ToLower(strings.TrimSpace(setID))
	if !uuidPattern.MatchString(tenantID) || !uuidPattern.MatchString(setID) {
		return Set{}, ErrInvalid
	}
	return s.repository.Get(ctx, tenantID, setID)
}

func normalize(input ConfirmInput) (ConfirmInput, error) {
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	input.TenantID = strings.ToLower(strings.TrimSpace(input.TenantID))
	input.ProjectID = strings.ToLower(strings.TrimSpace(input.ProjectID))
	actor, err := audit.NormalizeActor(input.Actor)
	if err != nil || !uuidPattern.MatchString(input.CommandID) || !uuidPattern.MatchString(input.TenantID) || !uuidPattern.MatchString(input.ProjectID) || len(input.AllocationIDs) == 0 || len(input.AllocationIDs) > 100 {
		return ConfirmInput{}, ErrInvalid
	}
	input.Actor = actor
	seen := make(map[string]bool, len(input.AllocationIDs))
	for index, value := range input.AllocationIDs {
		value = strings.ToLower(strings.TrimSpace(value))
		if !uuidPattern.MatchString(value) || seen[value] {
			return ConfirmInput{}, ErrInvalid
		}
		seen[value] = true
		input.AllocationIDs[index] = value
	}
	slices.Sort(input.AllocationIDs)
	return input, nil
}

func randomUUID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	value := hex.EncodeToString(raw)
	return value[0:8] + "-" + value[8:12] + "-" + value[12:16] + "-" + value[16:20] + "-" + value[20:32], nil
}
