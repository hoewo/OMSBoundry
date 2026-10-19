package payablesuggestion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/obligation"
	contract "omsbizfoundry/payable-suggestion-contract"
)

var (
	uuidPattern     = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
)

type Repository interface {
	Submit(context.Context, audit.ActorRef, contract.Submission, string) (Intake, bool, error)
	Get(context.Context, string, string) (Intake, error)
	History(context.Context, string, string) ([]Revision, error)
	ReplayCommand(context.Context, string, string, string) (Intake, bool, error)
	RecordDecision(context.Context, audit.ActorRef, string, string, int64, string, string, string, string, *contract.ObligationReference) (Intake, error)
}

type ObligationUseCases interface {
	CreateSuggestedDraft(context.Context, obligation.CreateSuggestedDraftInput) (obligation.CreateResult, error)
	Confirm(context.Context, obligation.ConfirmInput) (obligation.ConfirmResult, error)
}

type Service struct {
	repository  Repository
	obligations ObligationUseCases
}

func NewService(repository Repository, obligations ObligationUseCases) (*Service, error) {
	if repository == nil || obligations == nil {
		return nil, fmt.Errorf("payable suggestion repository and obligation use cases are required")
	}
	return &Service{repository: repository, obligations: obligations}, nil
}

func (s *Service) Submit(ctx context.Context, actor audit.ActorRef, value contract.Submission) (SubmitResult, error) {
	actor, err := audit.NormalizeActor(actor)
	if err != nil {
		return SubmitResult{}, ErrInvalid
	}
	value, digest, err := normalizeSubmission(value)
	if err != nil {
		return SubmitResult{}, err
	}
	intake, replayed, err := s.repository.Submit(ctx, actor, value, digest)
	if err != nil {
		return SubmitResult{}, err
	}
	return SubmitResult{Intake: intake, Replayed: replayed}, nil
}

func (s *Service) Get(ctx context.Context, tenantID, suggestionID string) (Intake, error) {
	tenantID, suggestionID = strings.ToLower(strings.TrimSpace(tenantID)), strings.ToLower(strings.TrimSpace(suggestionID))
	if !uuidPattern.MatchString(tenantID) || !uuidPattern.MatchString(suggestionID) {
		return Intake{}, ErrInvalid
	}
	return s.repository.Get(ctx, tenantID, suggestionID)
}

func (s *Service) History(ctx context.Context, tenantID, suggestionID string) ([]Revision, error) {
	if _, err := s.Get(ctx, tenantID, suggestionID); err != nil {
		return nil, err
	}
	return s.repository.History(ctx, tenantID, suggestionID)
}

func (s *Service) Decide(ctx context.Context, tenantID, suggestionID string, actor audit.ActorRef, input contract.Decision) (DecisionResult, error) {
	actor, err := audit.NormalizeActor(actor)
	if err != nil {
		return DecisionResult{}, ErrInvalid
	}
	tenantID, suggestionID = strings.ToLower(strings.TrimSpace(tenantID)), strings.ToLower(strings.TrimSpace(suggestionID))
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	input.Decision, input.Reason = strings.ToLower(strings.TrimSpace(input.Decision)), strings.TrimSpace(input.Reason)
	if !uuidPattern.MatchString(tenantID) || !uuidPattern.MatchString(suggestionID) || !uuidPattern.MatchString(input.CommandID) ||
		input.ExpectedRevision < 1 || (input.Decision != contract.DecisionAccept && input.Decision != contract.DecisionReject) ||
		len([]rune(input.Reason)) < 1 || len([]rune(input.Reason)) > 1000 {
		return DecisionResult{}, ErrInvalid
	}
	commandDigest, err := digestValue(struct {
		Format, TenantID, SuggestionID, CommandID, Decision, Reason string
		ExpectedRevision                                            int64
	}{"money-flow-payable-suggestion-decision-v1", tenantID, suggestionID, input.CommandID, input.Decision, input.Reason, input.ExpectedRevision})
	if err != nil {
		return DecisionResult{}, err
	}
	if replay, found, replayErr := s.repository.ReplayCommand(ctx, tenantID, input.CommandID, commandDigest); replayErr != nil || found {
		return DecisionResult{Intake: replay, Replayed: found}, replayErr
	}
	current, err := s.repository.Get(ctx, tenantID, suggestionID)
	if err != nil {
		return DecisionResult{}, err
	}
	if current.Revision != input.ExpectedRevision {
		return DecisionResult{}, ErrRevisionConflict
	}
	if current.Status != contract.StatusPending {
		return DecisionResult{}, ErrStateConflict
	}
	var obligationRef *contract.ObligationReference
	if input.Decision == contract.DecisionAccept {
		draftCommand := deterministicUUID(input.CommandID + ":obligation-draft")
		confirmCommand := deterministicUUID(input.CommandID + ":obligation-confirm")
		draft, createErr := s.obligations.CreateSuggestedDraft(ctx, obligation.CreateSuggestedDraftInput{
			CommandID: draftCommand, LedgerOwnerPartyID: current.LedgerOwnerLocalPartyID,
			CounterpartyPartyID: current.CounterpartyLocalPartyID, Title: current.Submission.Purpose,
			OriginalAmount: minorToDecimal(current.Submission.AmountMinor), Currency: current.Submission.Currency,
			DueOn: current.Submission.DueOn,
			Source: obligation.SuggestedSource{
				Application: current.Submission.Source.Application, ResourceType: current.Submission.Source.ResourceType,
				ResourceID: current.Submission.Source.ResourceID, Revision: current.Submission.Source.Revision,
				Digest: current.Submission.Source.Digest,
			}, Actor: actor,
		})
		if createErr != nil {
			return DecisionResult{}, createErr
		}
		confirmed, confirmErr := s.obligations.Confirm(ctx, obligation.ConfirmInput{
			CommandID: confirmCommand, ObligationID: draft.Obligation.ID, ExpectedDraftVersion: draft.Obligation.DraftVersion, Actor: actor,
		})
		if confirmErr != nil {
			return DecisionResult{}, confirmErr
		}
		obligationRef = &contract.ObligationReference{ID: confirmed.Obligation.ID, Revision: confirmed.Revision.Revision}
	}
	status := contract.StatusAccepted
	if input.Decision == contract.DecisionReject {
		status = contract.StatusRejected
	}
	updated, err := s.repository.RecordDecision(ctx, actor, tenantID, suggestionID, input.ExpectedRevision, input.CommandID, commandDigest, status, input.Reason, obligationRef)
	if err != nil {
		return DecisionResult{}, err
	}
	return DecisionResult{Intake: updated}, nil
}

func (s *Service) Withdraw(ctx context.Context, tenantID, suggestionID string, actor audit.ActorRef, input contract.Withdrawal) (DecisionResult, error) {
	actor, err := audit.NormalizeActor(actor)
	if err != nil {
		return DecisionResult{}, ErrInvalid
	}
	tenantID, suggestionID, input.CommandID = strings.ToLower(strings.TrimSpace(tenantID)), strings.ToLower(strings.TrimSpace(suggestionID)), strings.ToLower(strings.TrimSpace(input.CommandID))
	input.Reason = strings.TrimSpace(input.Reason)
	if !uuidPattern.MatchString(tenantID) || !uuidPattern.MatchString(suggestionID) || !uuidPattern.MatchString(input.CommandID) || input.ExpectedRevision < 1 || len([]rune(input.Reason)) < 1 || len([]rune(input.Reason)) > 1000 {
		return DecisionResult{}, ErrInvalid
	}
	commandDigest, err := digestValue(struct {
		Format, TenantID, SuggestionID, CommandID, Reason string
		ExpectedRevision                                  int64
	}{
		"money-flow-payable-suggestion-withdrawal-v1", tenantID, suggestionID, input.CommandID, input.Reason, input.ExpectedRevision,
	})
	if err != nil {
		return DecisionResult{}, err
	}
	if replay, found, replayErr := s.repository.ReplayCommand(ctx, tenantID, input.CommandID, commandDigest); replayErr != nil || found {
		return DecisionResult{Intake: replay, Replayed: found}, replayErr
	}
	updated, err := s.repository.RecordDecision(ctx, actor, tenantID, suggestionID, input.ExpectedRevision, input.CommandID, commandDigest, contract.StatusWithdrawn, input.Reason, nil)
	if err != nil {
		return DecisionResult{}, err
	}
	return DecisionResult{Intake: updated}, nil
}

func normalizeSubmission(value contract.Submission) (contract.Submission, string, error) {
	value.SchemaVersion = strings.TrimSpace(value.SchemaVersion)
	value.SuggestionID, value.TenantID, value.ProjectID = strings.ToLower(strings.TrimSpace(value.SuggestionID)), strings.ToLower(strings.TrimSpace(value.TenantID)), strings.ToLower(strings.TrimSpace(value.ProjectID))
	value.Source.Application, value.Source.ResourceType = strings.TrimSpace(value.Source.Application), strings.TrimSpace(value.Source.ResourceType)
	value.Source.ResourceID, value.Source.Digest = strings.ToLower(strings.TrimSpace(value.Source.ResourceID)), strings.ToLower(strings.TrimSpace(value.Source.Digest))
	value.LedgerOwner.ID, value.Counterparty.ID = strings.ToLower(strings.TrimSpace(value.LedgerOwner.ID)), strings.ToLower(strings.TrimSpace(value.Counterparty.ID))
	value.LedgerOwner.SourceApplication, value.Counterparty.SourceApplication = strings.TrimSpace(value.LedgerOwner.SourceApplication), strings.TrimSpace(value.Counterparty.SourceApplication)
	value.LedgerOwner.ResourceType, value.Counterparty.ResourceType = strings.TrimSpace(value.LedgerOwner.ResourceType), strings.TrimSpace(value.Counterparty.ResourceType)
	value.LedgerOwner.Digest, value.Counterparty.Digest = strings.ToLower(strings.TrimSpace(value.LedgerOwner.Digest)), strings.ToLower(strings.TrimSpace(value.Counterparty.Digest))
	value.LedgerOwner.Kind, value.Counterparty.Kind = strings.TrimSpace(value.LedgerOwner.Kind), strings.TrimSpace(value.Counterparty.Kind)
	value.LedgerOwner.LegalName, value.Counterparty.LegalName = strings.TrimSpace(value.LedgerOwner.LegalName), strings.TrimSpace(value.Counterparty.LegalName)
	value.Currency, value.DueOn, value.Purpose = strings.ToUpper(strings.TrimSpace(value.Currency)), strings.TrimSpace(value.DueOn), strings.TrimSpace(value.Purpose)
	value.RequestDigest = strings.ToLower(strings.TrimSpace(value.RequestDigest))
	validSource := (value.Source.Application == contract.SourceApplicationCostOutsourcing &&
		(value.Source.ResourceType == contract.SourceCostEntry || value.Source.ResourceType == contract.SourceEngagement)) ||
		(value.Source.Application == contract.SourceApplicationBonusDistribution && value.Source.ResourceType == contract.SourceBonusAllocation)
	if value.SchemaVersion != contract.SchemaVersion || !uuidPattern.MatchString(value.SuggestionID) || !uuidPattern.MatchString(value.TenantID) || !uuidPattern.MatchString(value.ProjectID) ||
		!validSource ||
		!uuidPattern.MatchString(value.Source.ResourceID) || value.Source.Revision < 1 || !validDigest(value.Source.Digest) ||
		!validParty(value.LedgerOwner) || !validParty(value.Counterparty) || value.LedgerOwner.ID == value.Counterparty.ID ||
		value.AmountMinor <= 0 || !currencyPattern.MatchString(value.Currency) || len([]rune(value.Purpose)) < 1 || len([]rune(value.Purpose)) > 1000 {
		return contract.Submission{}, "", ErrInvalid
	}
	if parsed, err := time.Parse("2006-01-02", value.DueOn); err != nil || parsed.Format("2006-01-02") != value.DueOn {
		return contract.Submission{}, "", ErrInvalid
	}
	digest, err := contract.Digest(value)
	if err != nil {
		return contract.Submission{}, "", err
	}
	if value.RequestDigest != digest {
		return contract.Submission{}, "", ErrInvalid
	}
	return value, digest, nil
}

func validParty(value contract.PartySnapshot) bool {
	if !uuidPattern.MatchString(value.ID) || value.Revision < 1 || (value.Kind != "organization" && value.Kind != "individual") || len([]rune(value.LegalName)) < 1 || len([]rune(value.LegalName)) > 200 {
		return false
	}
	application, resourceType := value.SourceApplication, value.ResourceType
	if application == "" && resourceType == "" {
		return value.Digest == ""
	}
	if application == contract.PartySourceApplicationMoneyFlow && resourceType == contract.PartyResourceParty {
		return value.Digest == ""
	}
	if application == contract.PartySourceApplicationPartyDirectory && resourceType == contract.PartyResourceParty {
		return value.Digest == ""
	}
	if application == contract.PartySourceApplicationPeopleGroups && resourceType == contract.PartyResourcePerson && value.Kind == "individual" {
		return validDigest(value.Digest)
	}
	return false
}
func validDigest(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	_, err := hex.DecodeString(value[7:])
	return err == nil
}
func digestValue(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}
func deterministicUUID(seed string) string {
	digest := sha256.Sum256([]byte(seed))
	digest[6] = (digest[6] & 0x0f) | 0x40
	digest[8] = (digest[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", digest[0:4], digest[4:6], digest[6:8], digest[8:10], digest[10:16])
}
func minorToDecimal(value int64) string {
	return strconv.FormatInt(value/100, 10) + "." + fmt.Sprintf("%02d", value%100)
}
