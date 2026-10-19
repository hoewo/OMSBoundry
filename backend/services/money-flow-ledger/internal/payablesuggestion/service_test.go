package payablesuggestion

import (
	"context"
	"errors"
	"testing"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/obligation"
	contract "omsbizfoundry/payable-suggestion-contract"
)

const (
	tenantID       = "11000000-0000-4000-8000-000000000001"
	suggestionID   = "11000000-0000-4000-8000-000000000002"
	projectID      = "11000000-0000-4000-8000-000000000003"
	ownerID        = "11000000-0000-4000-8000-000000000004"
	counterpartyID = "11000000-0000-4000-8000-000000000005"
	sourceID       = "11000000-0000-4000-8000-000000000006"
	actorID        = "11000000-0000-4000-8000-000000000007"
	decisionID     = "11000000-0000-4000-8000-000000000008"
	obligationID   = "11000000-0000-4000-8000-000000000009"
)

type memoryRepository struct {
	current  Intake
	commands map[string]struct {
		digest string
		value  Intake
	}
	failDecisionOnce bool
}

func (r *memoryRepository) Submit(_ context.Context, actor audit.ActorRef, value contract.Submission, digest string) (Intake, bool, error) {
	if r.commands == nil {
		r.commands = map[string]struct {
			digest string
			value  Intake
		}{}
	}
	if command, ok := r.commands[value.SuggestionID]; ok {
		if command.digest != digest {
			return Intake{}, false, ErrCommandConflict
		}
		return command.value, true, nil
	}
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	r.current = Intake{SchemaVersion: contract.SchemaVersion, SuggestionID: value.SuggestionID, TenantID: value.TenantID, RequestDigest: value.RequestDigest, Submission: value, Status: contract.StatusPending, Revision: 1, SubmittedBy: actor, CreatedAt: now, UpdatedAt: now, LedgerOwnerLocalPartyID: ownerID, CounterpartyLocalPartyID: counterpartyID}
	r.commands[value.SuggestionID] = struct {
		digest string
		value  Intake
	}{digest, r.current}
	return r.current, false, nil
}
func (r *memoryRepository) Get(_ context.Context, tenant, suggestion string) (Intake, error) {
	if r.current.TenantID != tenant || r.current.SuggestionID != suggestion {
		return Intake{}, ErrNotFound
	}
	return r.current, nil
}
func (r *memoryRepository) History(context.Context, string, string) ([]Revision, error) {
	return nil, nil
}
func (r *memoryRepository) ReplayCommand(_ context.Context, _ string, commandID, digest string) (Intake, bool, error) {
	command, ok := r.commands[commandID]
	if !ok {
		return Intake{}, false, nil
	}
	if command.digest != digest {
		return Intake{}, false, ErrCommandConflict
	}
	return command.value, true, nil
}
func (r *memoryRepository) RecordDecision(_ context.Context, actor audit.ActorRef, tenant, suggestion string, expected int64, commandID, digest, status, reason string, obligationRef *contract.ObligationReference) (Intake, error) {
	if r.failDecisionOnce {
		r.failDecisionOnce = false
		return Intake{}, errors.New("simulated commit interruption")
	}
	if r.current.TenantID != tenant || r.current.SuggestionID != suggestion || r.current.Revision != expected {
		return Intake{}, ErrRevisionConflict
	}
	r.current.Status, r.current.DecisionReason, r.current.Obligation = status, reason, obligationRef
	r.current.Revision++
	r.current.DecidedBy = &actor
	r.commands[commandID] = struct {
		digest string
		value  Intake
	}{digest, r.current}
	return r.current, nil
}

type idempotentObligations struct {
	drafts        map[string]obligation.CreateResult
	confirmations map[string]obligation.ConfirmResult
}

func (o *idempotentObligations) CreateSuggestedDraft(_ context.Context, input obligation.CreateSuggestedDraftInput) (obligation.CreateResult, error) {
	if o.drafts == nil {
		o.drafts = map[string]obligation.CreateResult{}
	}
	if value, ok := o.drafts[input.CommandID]; ok {
		value.Replayed = true
		return value, nil
	}
	value := obligation.CreateResult{Obligation: obligation.Obligation{ID: obligationID, DraftVersion: 1, Status: obligation.StatusDraft, SourceKind: obligation.SourceKindCostOutsourcingSuggestion, SourceApplication: input.Source.Application, SourceResourceID: input.Source.ResourceID, SourceFactRevision: input.Source.Revision}}
	o.drafts[input.CommandID] = value
	return value, nil
}
func (o *idempotentObligations) Confirm(_ context.Context, input obligation.ConfirmInput) (obligation.ConfirmResult, error) {
	if o.confirmations == nil {
		o.confirmations = map[string]obligation.ConfirmResult{}
	}
	if value, ok := o.confirmations[input.CommandID]; ok {
		value.Replayed = true
		return value, nil
	}
	value := obligation.ConfirmResult{Obligation: obligation.Obligation{ID: input.ObligationID, Status: obligation.StatusOpen}, Revision: obligation.Revision{ObligationID: input.ObligationID, Revision: 1}}
	o.confirmations[input.CommandID] = value
	return value, nil
}

func TestAcceptRecoversAfterObligationCommittedBeforeIntakeDecision(t *testing.T) {
	repository := &memoryRepository{failDecisionOnce: true}
	obligations := &idempotentObligations{}
	service, err := NewService(repository, obligations)
	if err != nil {
		t.Fatal(err)
	}
	actor := audit.ActorRef{Kind: audit.ActorKindIAMSubject, ID: actorID, DisplayName: "资金负责人"}
	submission := validSubmission(t)
	if result, submitErr := service.Submit(context.Background(), actor, submission); submitErr != nil || result.Intake.Status != contract.StatusPending {
		t.Fatalf("submit=%#v err=%v", result, submitErr)
	}
	decision := contract.Decision{CommandID: decisionID, ExpectedRevision: 1, Decision: contract.DecisionAccept, Reason: "来源和金额核验通过"}
	if _, err = service.Decide(context.Background(), tenantID, suggestionID, actor, decision); err == nil {
		t.Fatal("simulated post-obligation commit interruption was not returned")
	}
	result, err := service.Decide(context.Background(), tenantID, suggestionID, actor, decision)
	if err != nil || result.Intake.Status != contract.StatusAccepted || result.Intake.Obligation == nil || result.Intake.Obligation.ID != obligationID {
		t.Fatalf("recovered decision=%#v err=%v", result, err)
	}
	if len(obligations.drafts) != 1 || len(obligations.confirmations) != 1 {
		t.Fatalf("recovery duplicated obligation work: drafts=%d confirmations=%d", len(obligations.drafts), len(obligations.confirmations))
	}
	replayed, err := service.Decide(context.Background(), tenantID, suggestionID, actor, decision)
	if err != nil || !replayed.Replayed || replayed.Intake.Revision != 2 {
		t.Fatalf("decision replay=%#v err=%v", replayed, err)
	}
}

func TestSubmitAcceptsExplicitPartyDirectorySnapshot(t *testing.T) {
	repository := &memoryRepository{}
	service, err := NewService(repository, &idempotentObligations{})
	if err != nil {
		t.Fatal(err)
	}
	value := validSubmission(t)
	value.Counterparty.SourceApplication = contract.PartySourceApplicationPartyDirectory
	value.Counterparty.ResourceType = contract.PartyResourceParty
	value.RequestDigest = ""
	digest, err := contract.Digest(value)
	if err != nil {
		t.Fatal(err)
	}
	value.RequestDigest = digest
	actor := audit.ActorRef{Kind: audit.ActorKindIAMSubject, ID: actorID, DisplayName: "资金负责人"}
	if _, err = service.Submit(context.Background(), actor, value); err != nil {
		t.Fatalf("submit explicit Party Directory snapshot: %v", err)
	}
}

func validSubmission(t *testing.T) contract.Submission {
	t.Helper()
	value := contract.Submission{SchemaVersion: contract.SchemaVersion, SuggestionID: suggestionID, TenantID: tenantID, ProjectID: projectID,
		Source:       contract.SourceSnapshot{Application: contract.SourceApplication, ResourceType: contract.SourceEngagement, ResourceID: sourceID, Revision: 3, Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		LedgerOwner:  contract.PartySnapshot{ID: ownerID, Revision: 2, Kind: "organization", LegalName: "设计单位"},
		Counterparty: contract.PartySnapshot{ID: counterpartyID, Revision: 4, Kind: "organization", LegalName: "协作单位"},
		AmountMinor:  880000, Currency: "CNY", DueOn: "2026-10-31", Purpose: "桥梁测量协作应付款"}
	digest, err := contract.Digest(value)
	if err != nil {
		t.Fatal(err)
	}
	value.RequestDigest = digest
	return value
}
