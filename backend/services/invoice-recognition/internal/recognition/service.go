package recognition

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	contract "omsbizfoundry/invoice-recognition-contract"
)

var (
	uuidPattern            = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	ErrInvalid             = errors.New("invoice recognition input is invalid")
	ErrNotFound            = errors.New("invoice recognition proposal not found")
	ErrConflict            = errors.New("invoice recognition revision conflict")
	ErrIdempotencyConflict = errors.New("invoice recognition command conflict")
	ErrEvidenceUnavailable = errors.New("invoice evidence unavailable")
	ErrProviderUnavailable = errors.New("invoice recognition provider unavailable")
)

type Actor struct {
	SubjectID    string
	TenantID     string
	DisplayName  string
	CredentialID string
}

type StartInput struct {
	CommandID string                     `json:"command_id"`
	Evidence  contract.EvidenceReference `json:"evidence"`
}

type ConfirmInput struct {
	CommandID        string             `json:"command_id"`
	ProposalID       string             `json:"proposal_id"`
	ExpectedRevision int64              `json:"expected_revision"`
	Candidate        contract.Candidate `json:"candidate"`
}

type DiscardInput struct {
	CommandID        string `json:"command_id"`
	ProposalID       string `json:"proposal_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Reason           string `json:"reason"`
}

type Document struct {
	Evidence contract.EvidenceReference
	Bytes    []byte
}

type ProviderResult struct {
	ProviderKey      string                            `json:"provider_key"`
	ProviderModel    string                            `json:"provider_model"`
	ProviderRevision string                            `json:"provider_revision"`
	Candidate        contract.Candidate                `json:"candidate"`
	FieldRegions     map[string][]contract.FieldRegion `json:"field_regions,omitempty"`
	Warnings         []string                          `json:"warnings,omitempty"`
}

type EvidencePort interface {
	Open(context.Context, string, contract.EvidenceReference) (io.ReadCloser, error)
}

type Provider interface {
	Recognize(context.Context, Document) (ProviderResult, error)
}

type Repository interface {
	ReplayProposal(context.Context, string, string, string) (contract.Proposal, bool, error)
	ReplayConfirmation(context.Context, string, string, string) (contract.Confirmation, bool, error)
	ReplayDiscard(context.Context, string, string, string) (contract.Proposal, bool, error)
	Create(context.Context, Actor, contract.Proposal, string) (contract.Proposal, error)
	Get(context.Context, string, string) (contract.Proposal, error)
	List(context.Context, string) ([]contract.Proposal, error)
	Confirm(context.Context, Actor, string, contract.Proposal, contract.Confirmation, string) (contract.Confirmation, error)
	Discard(context.Context, Actor, string, contract.Proposal, string, time.Time, string) (contract.Proposal, error)
}

type IDGenerator func() string

type Service struct {
	evidence EvidencePort
	provider Provider
	repo     Repository
	newID    IDGenerator
	now      func() time.Time
}

func New(evidence EvidencePort, provider Provider, repo Repository, newID IDGenerator, now func() time.Time) *Service {
	return &Service{evidence: evidence, provider: provider, repo: repo, newID: newID, now: now}
}

func (s *Service) Ping(ctx context.Context) error {
	if repository, ok := s.repo.(interface{ Ping(context.Context) error }); ok {
		return repository.Ping(ctx)
	}
	return nil
}

func (s *Service) Get(ctx context.Context, tenantID, id string) (contract.Proposal, error) {
	if !uuidPattern.MatchString(strings.ToLower(strings.TrimSpace(tenantID))) || !uuidPattern.MatchString(strings.ToLower(strings.TrimSpace(id))) {
		return contract.Proposal{}, ErrInvalid
	}
	return s.repo.Get(ctx, strings.ToLower(strings.TrimSpace(tenantID)), strings.ToLower(strings.TrimSpace(id)))
}
func (s *Service) List(ctx context.Context, tenantID string) ([]contract.Proposal, error) {
	if !uuidPattern.MatchString(strings.ToLower(strings.TrimSpace(tenantID))) {
		return nil, ErrInvalid
	}
	return s.repo.List(ctx, strings.ToLower(strings.TrimSpace(tenantID)))
}

func (s *Service) Start(ctx context.Context, actor Actor, input StartInput) (contract.Proposal, error) {
	if err := validateActor(actor); err != nil || !uuidPattern.MatchString(input.CommandID) {
		return contract.Proposal{}, ErrInvalid
	}
	if err := contract.ValidateEvidence(input.Evidence); err != nil {
		return contract.Proposal{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	requestHash := intentHash(input)
	if proposal, found, err := s.repo.ReplayProposal(ctx, actor.TenantID, input.CommandID, requestHash); err != nil || found {
		return proposal, err
	}
	stream, err := s.evidence.Open(ctx, actor.TenantID, input.Evidence)
	if err != nil {
		return contract.Proposal{}, fmt.Errorf("%w: open evidence version: %v", ErrEvidenceUnavailable, err)
	}
	defer stream.Close()
	content, err := io.ReadAll(io.LimitReader(stream, contract.MaximumEvidenceBytes+1))
	if err != nil {
		return contract.Proposal{}, fmt.Errorf("%w: read evidence version: %v", ErrEvidenceUnavailable, err)
	}
	if int64(len(content)) != input.Evidence.SizeBytes || len(content) > contract.MaximumEvidenceBytes {
		return contract.Proposal{}, fmt.Errorf("%w: evidence size does not match the trusted version", ErrEvidenceUnavailable)
	}
	digest := sha256.Sum256(content)
	if "sha256:"+hex.EncodeToString(digest[:]) != input.Evidence.ContentSHA256 {
		return contract.Proposal{}, fmt.Errorf("%w: evidence digest does not match the trusted version", ErrEvidenceUnavailable)
	}
	result, err := s.provider.Recognize(ctx, Document{Evidence: input.Evidence, Bytes: content})
	if err != nil {
		return contract.Proposal{}, fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
	}
	result.Candidate = normalizeCandidate(result.Candidate)
	if err := validateProviderResult(result); err != nil {
		return contract.Proposal{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	createdAt := s.now().UTC()
	proposal := contract.Proposal{
		SchemaVersion: contract.ProposalSchemaVersion, ID: s.newID(), TenantID: actor.TenantID,
		CommandID: input.CommandID, Evidence: input.Evidence, ProviderKey: result.ProviderKey,
		ProviderModel: result.ProviderModel, ProviderRevision: result.ProviderRevision,
		Candidate: result.Candidate, FieldRegions: result.FieldRegions,
		Warnings: normalizeWarnings(result.Warnings), Status: "needs_review",
		Revision: 1, CreatedAt: createdAt,
	}
	if !uuidPattern.MatchString(proposal.ID) {
		return contract.Proposal{}, fmt.Errorf("generated proposal identity is invalid")
	}
	return s.repo.Create(ctx, actor, proposal, requestHash)
}

func (s *Service) Confirm(ctx context.Context, actor Actor, input ConfirmInput) (contract.Confirmation, error) {
	if err := validateActor(actor); err != nil || !uuidPattern.MatchString(input.CommandID) || !uuidPattern.MatchString(input.ProposalID) || input.ExpectedRevision < 1 {
		return contract.Confirmation{}, ErrInvalid
	}
	requestHash := intentHash(input)
	if confirmation, found, err := s.repo.ReplayConfirmation(ctx, actor.TenantID, input.CommandID, requestHash); err != nil || found {
		return confirmation, err
	}
	proposal, err := s.repo.Get(ctx, actor.TenantID, input.ProposalID)
	if err != nil {
		return contract.Confirmation{}, err
	}
	if proposal.Status != "needs_review" || proposal.Revision != input.ExpectedRevision {
		return contract.Confirmation{}, ErrConflict
	}
	candidate := normalizeCandidate(input.Candidate)
	if err := contract.ValidateCandidate(candidate, true); err != nil {
		return contract.Confirmation{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	confirmation := contract.Confirmation{
		SchemaVersion: contract.ConfirmationSchemaVersion, ProposalID: proposal.ID,
		ProposalRevision: proposal.Revision, TenantID: proposal.TenantID, Evidence: proposal.Evidence,
		Candidate: candidate, DuplicateKeys: contract.DuplicateKeys(proposal.Evidence, candidate),
		ConfirmedBySubject: actor.SubjectID, ConfirmedAt: s.now().UTC(),
	}
	return s.repo.Confirm(ctx, actor, input.CommandID, proposal, confirmation, requestHash)
}

func (s *Service) Discard(ctx context.Context, actor Actor, input DiscardInput) (contract.Proposal, error) {
	input.Reason = strings.TrimSpace(input.Reason)
	if err := validateActor(actor); err != nil || !uuidPattern.MatchString(input.CommandID) || !uuidPattern.MatchString(input.ProposalID) || input.ExpectedRevision < 1 || len([]rune(input.Reason)) < 1 || len([]rune(input.Reason)) > 500 {
		return contract.Proposal{}, ErrInvalid
	}
	requestHash := intentHash(input)
	if proposal, found, err := s.repo.ReplayDiscard(ctx, actor.TenantID, input.CommandID, requestHash); err != nil || found {
		return proposal, err
	}
	proposal, err := s.repo.Get(ctx, actor.TenantID, input.ProposalID)
	if err != nil {
		return contract.Proposal{}, err
	}
	if !discardableStatus(proposal.Status) || proposal.Revision != input.ExpectedRevision {
		return contract.Proposal{}, ErrConflict
	}
	return s.repo.Discard(ctx, actor, input.CommandID, proposal, input.Reason, s.now().UTC(), requestHash)
}

func discardableStatus(status string) bool {
	return status == "needs_review" || status == "confirmed"
}

func validateActor(actor Actor) error {
	if !uuidPattern.MatchString(actor.SubjectID) || !uuidPattern.MatchString(actor.TenantID) {
		return fmt.Errorf("trusted actor is invalid")
	}
	return nil
}

func validateProviderResult(result ProviderResult) error {
	for _, value := range []string{result.ProviderKey, result.ProviderModel, result.ProviderRevision} {
		if strings.TrimSpace(value) == "" || len(value) > 160 {
			return fmt.Errorf("recognition provider identity is invalid")
		}
	}
	if err := contract.ValidateCandidate(result.Candidate, false); err != nil {
		return err
	}
	return contract.ValidateFieldRegions(result.FieldRegions)
}

func normalizeCandidate(value contract.Candidate) contract.Candidate {
	value.InvoiceCode = strings.ToUpper(strings.TrimSpace(value.InvoiceCode))
	value.InvoiceNumber = strings.ToUpper(strings.TrimSpace(value.InvoiceNumber))
	value.IssuedOn = strings.TrimSpace(value.IssuedOn)
	value.SellerName = strings.TrimSpace(value.SellerName)
	value.SellerTaxID = strings.ToUpper(strings.TrimSpace(value.SellerTaxID))
	value.BuyerName = strings.TrimSpace(value.BuyerName)
	value.BuyerTaxID = strings.ToUpper(strings.TrimSpace(value.BuyerTaxID))
	value.Currency = strings.ToUpper(strings.TrimSpace(value.Currency))
	return value
}

func normalizeWarnings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && len([]rune(value)) <= 300 && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func intentHash(value any) string {
	encoded, _ := json.Marshal(value)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}
