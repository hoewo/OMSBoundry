package bonus

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	contract "omsbizfoundry/payable-suggestion-contract"
)

func (s *Service) ProposePayableSuggestion(ctx context.Context, actor Actor, input ProposePayableSuggestionInput) (BonusPayableSuggestion, error) {
	actor, err := normalizeActor(actor)
	if err != nil {
		return BonusPayableSuggestion{}, err
	}
	if s.payableStore == nil {
		return BonusPayableSuggestion{}, fmt.Errorf("payable suggestion store is unavailable")
	}
	input, err = normalizePayableSuggestionInput(input)
	if err != nil {
		return BonusPayableSuggestion{}, err
	}
	hash, err := requestHash("propose_bonus_payable_suggestion", "", actor.ID, input)
	if err != nil {
		return BonusPayableSuggestion{}, err
	}
	var replay BonusPayableSuggestion
	if found, replayErr := s.store.Replay(ctx, actor, input.CommandID, hash, "bonus_payable_suggestion", &replay); replayErr != nil {
		return BonusPayableSuggestion{}, replayErr
	} else if found {
		current, getErr := s.payableStore.GetSuggestion(ctx, actor.TenantID, replay.ID)
		if getErr != nil {
			return BonusPayableSuggestion{}, getErr
		}
		if err = authorizeSuggestion(actor, ActionSuggestionCreate, current); err != nil {
			return BonusPayableSuggestion{}, err
		}
		if current.State == SuggestionTargetUnavailable {
			return s.submitPayableSuggestion(ctx, actor, current)
		}
		return current, nil
	}
	allocation, err := s.payableStore.GetAllocation(ctx, actor.TenantID, input.AllocationID)
	if err != nil {
		return BonusPayableSuggestion{}, err
	}
	if allocation.Revision != input.AllocationRevision || allocation.State != StatePublished || allocation.AmountMinor <= 0 {
		return BonusPayableSuggestion{}, ErrReferenceInvalid
	}
	if !contains(actor.Actions, ActionSuggestionCreate) || !hasAllocationAccess(actor, allocation.ID, allocation.PoolID, allocation.ProjectID) {
		return BonusPayableSuggestion{}, ErrForbidden
	}
	if err = requireAction(actor, ActionAllocationAmountRead); err != nil {
		return BonusPayableSuggestion{}, err
	}
	raw, err := json.Marshal(allocation)
	if err != nil {
		return BonusPayableSuggestion{}, err
	}
	digest := sha256.Sum256(raw)
	id, err := newUUID()
	if err != nil {
		return BonusPayableSuggestion{}, err
	}
	now := time.Now().UTC()
	value := BonusPayableSuggestion{
		ID: id, TenantID: actor.TenantID,
		Source: PayableSourceSnapshot{
			AllocationID: allocation.ID, AllocationRevision: allocation.Revision,
			AllocationDigest: "sha256:" + hex.EncodeToString(digest[:]), PoolID: allocation.PoolID,
			PoolRevision: allocation.PoolRevision,
			Project:      ProjectSnapshot{ID: allocation.ProjectID, Revision: 1}, Person: allocation.Person,
		},
		LedgerOwner: input.LedgerOwner, AmountMinor: allocation.AmountMinor, Currency: allocation.Currency,
		DueOn: input.DueOn, Purpose: input.Purpose, State: SuggestionTargetUnavailable, Revision: 1,
		CreatedBy: actor.ID, UpdatedBy: actor.ID, CreatedAt: now, UpdatedAt: now,
	}
	pool, err := s.store.GetPool(ctx, actor.TenantID, allocation.PoolID)
	if err != nil || pool.Project.ID != allocation.ProjectID {
		return BonusPayableSuggestion{}, ErrReferenceInvalid
	}
	value.Source.Project = pool.Project
	wire := payableWireSubmission(value)
	value.RequestDigest, err = contract.Digest(wire)
	if err != nil {
		return BonusPayableSuggestion{}, err
	}
	created, err := s.payableStore.CreateSuggestion(ctx, actor, value, input.CommandID, hash)
	if err != nil {
		return BonusPayableSuggestion{}, err
	}
	return s.submitPayableSuggestion(ctx, actor, created)
}

func (s *Service) GetPayableSuggestion(ctx context.Context, actor Actor, id string) (BonusPayableSuggestion, error) {
	actor, err := normalizeActor(actor)
	id = normalizeUUID(id)
	if err != nil || !validUUID(id) || s.payableStore == nil {
		return BonusPayableSuggestion{}, ErrReferenceInvalid
	}
	value, err := s.payableStore.GetSuggestion(ctx, actor.TenantID, id)
	if err != nil {
		return BonusPayableSuggestion{}, err
	}
	if err = authorizeSuggestion(actor, ActionSuggestionRead, value); err != nil {
		return BonusPayableSuggestion{}, err
	}
	return value, nil
}

func (s *Service) ListPayableSuggestions(ctx context.Context, actor Actor, projectID string) ([]BonusPayableSuggestion, error) {
	actor, err := normalizeActor(actor)
	projectID = normalizeUUID(projectID)
	if err != nil || !validUUID(projectID) || s.payableStore == nil {
		return nil, ErrReferenceInvalid
	}
	if err = authorize(actor, ActionSuggestionRead, projectID); err != nil {
		return nil, err
	}
	return s.payableStore.ListSuggestions(ctx, actor.TenantID, projectID)
}

func (s *Service) PayableSuggestionHistory(ctx context.Context, actor Actor, id string) ([]BonusPayableSuggestionRevision, error) {
	if _, err := s.GetPayableSuggestion(ctx, actor, id); err != nil {
		return nil, err
	}
	return s.payableStore.SuggestionHistory(ctx, actor.TenantID, normalizeUUID(id))
}

func (s *Service) SynchronizePayableSuggestion(ctx context.Context, actor Actor, id string, input SuggestionCommandInput) (BonusPayableSuggestion, error) {
	actor, err := normalizeActor(actor)
	id, input, err = normalizeSuggestionCommand(id, input, false, err)
	if err != nil || s.payableStore == nil {
		return BonusPayableSuggestion{}, ErrReferenceInvalid
	}
	hash, err := requestHash("synchronize_bonus_payable_suggestion", id, actor.ID, input)
	if err != nil {
		return BonusPayableSuggestion{}, err
	}
	var replay BonusPayableSuggestion
	if found, replayErr := s.store.Replay(ctx, actor, input.CommandID, hash, "bonus_payable_suggestion", &replay); replayErr != nil || found {
		return replay, replayErr
	}
	current, err := s.payableStore.GetSuggestion(ctx, actor.TenantID, id)
	if err != nil {
		return BonusPayableSuggestion{}, err
	}
	if err = authorizeSuggestion(actor, ActionSuggestionCreate, current); err != nil || current.Revision != input.ExpectedRevision {
		if err != nil {
			return BonusPayableSuggestion{}, err
		}
		return BonusPayableSuggestion{}, ErrRevisionConflict
	}
	if current.State == SuggestionTargetUnavailable && current.TargetRevision == 0 {
		if s.money == nil {
			return BonusPayableSuggestion{}, ErrTargetUnavailable
		}
		intake, submitErr := s.money.SubmitPayableSuggestion(ctx, actor.TenantID, current.ID, payableSubmission(current))
		if submitErr != nil {
			return BonusPayableSuggestion{}, ErrTargetUnavailable
		}
		return s.applyPayableIntake(ctx, actor, current, input.CommandID, hash, "submitted", input.Reason, intake)
	}
	if s.money == nil {
		return BonusPayableSuggestion{}, ErrTargetUnavailable
	}
	intake, err := s.money.GetPayableSuggestion(ctx, actor.TenantID, current.ID)
	if err != nil {
		return BonusPayableSuggestion{}, ErrTargetUnavailable
	}
	return s.applyPayableIntake(ctx, actor, current, input.CommandID, hash, "synchronized", input.Reason, intake)
}

func (s *Service) WithdrawPayableSuggestion(ctx context.Context, actor Actor, id string, input SuggestionCommandInput) (BonusPayableSuggestion, error) {
	actor, err := normalizeActor(actor)
	id, input, err = normalizeSuggestionCommand(id, input, true, err)
	if err != nil || s.payableStore == nil {
		return BonusPayableSuggestion{}, ErrReferenceInvalid
	}
	hash, err := requestHash("withdraw_bonus_payable_suggestion", id, actor.ID, input)
	if err != nil {
		return BonusPayableSuggestion{}, err
	}
	var replay BonusPayableSuggestion
	if found, replayErr := s.store.Replay(ctx, actor, input.CommandID, hash, "bonus_payable_suggestion", &replay); replayErr != nil || found {
		return replay, replayErr
	}
	current, err := s.payableStore.GetSuggestion(ctx, actor.TenantID, id)
	if err != nil {
		return BonusPayableSuggestion{}, err
	}
	if err = authorizeSuggestion(actor, ActionSuggestionWithdraw, current); err != nil {
		return BonusPayableSuggestion{}, err
	}
	if current.Revision != input.ExpectedRevision {
		return BonusPayableSuggestion{}, ErrRevisionConflict
	}
	if current.State == SuggestionTargetUnavailable && current.TargetRevision == 0 {
		current, err = s.submitPayableSuggestion(ctx, actor, current)
		if err != nil || current.TargetRevision == 0 {
			return current, err
		}
	}
	if current.State != SuggestionProposed || s.money == nil {
		return BonusPayableSuggestion{}, ErrStateConflict
	}
	intake, err := s.money.WithdrawPayableSuggestion(ctx, actor.TenantID, id, input.CommandID, current.TargetRevision, input.Reason)
	if err != nil {
		return BonusPayableSuggestion{}, ErrTargetUnavailable
	}
	return s.applyPayableIntake(ctx, actor, current, input.CommandID, hash, "withdrawn", input.Reason, intake)
}

func (s *Service) submitPayableSuggestion(ctx context.Context, actor Actor, current BonusPayableSuggestion) (BonusPayableSuggestion, error) {
	if s.money == nil {
		return current, nil
	}
	intake, err := s.money.SubmitPayableSuggestion(ctx, actor.TenantID, current.ID, payableSubmission(current))
	if err != nil {
		return current, nil
	}
	commandID := deterministicSuggestionUUID(current.ID + ":target:" + fmt.Sprint(intake.Revision) + ":" + intake.Status)
	hash, err := requestHash("apply_bonus_payable_suggestion_target", current.ID, actor.ID, intake)
	if err != nil {
		return BonusPayableSuggestion{}, err
	}
	return s.applyPayableIntake(ctx, actor, current, commandID, hash, "submitted", "", intake)
}

func (s *Service) applyPayableIntake(ctx context.Context, actor Actor, current BonusPayableSuggestion, commandID, hash, event, reason string, intake PayableIntake) (BonusPayableSuggestion, error) {
	if intake.SuggestionID != current.ID || intake.TenantID != current.TenantID || intake.RequestDigest != current.RequestDigest || intake.Revision < 1 {
		return BonusPayableSuggestion{}, ErrReferenceInvalid
	}
	state := ""
	switch intake.Status {
	case contract.StatusPending:
		state = SuggestionProposed
	case contract.StatusAccepted:
		state, event = SuggestionAccepted, "accepted"
	case contract.StatusRejected:
		state, event = SuggestionRejected, "rejected"
	case contract.StatusWithdrawn:
		state, event = SuggestionWithdrawn, "withdrawn"
	default:
		return BonusPayableSuggestion{}, ErrReferenceInvalid
	}
	if intake.Revision < current.TargetRevision {
		return BonusPayableSuggestion{}, ErrReferenceInvalid
	}
	if intake.Revision == current.TargetRevision && state == current.State {
		return current, nil
	}
	updated := current
	updated.State, updated.TargetRevision, updated.DecisionReason, updated.Obligation = state, intake.Revision, intake.DecisionReason, intake.Obligation
	return s.payableStore.ApplySuggestionTarget(ctx, actor, current.ID, current.Revision, commandID, hash, event, reason, updated)
}

func payableSubmission(value BonusPayableSuggestion) PayableSubmission {
	return PayableSubmission{SuggestionID: value.ID, TenantID: value.TenantID, ProjectID: value.Source.Project.ID, Source: value.Source, LedgerOwner: value.LedgerOwner, AmountMinor: value.AmountMinor, Currency: value.Currency, DueOn: value.DueOn, Purpose: value.Purpose, RequestDigest: value.RequestDigest}
}

func payableWireSubmission(value BonusPayableSuggestion) contract.Submission {
	return payableWire(payableSubmission(value))
}

func payableWire(value PayableSubmission) contract.Submission {
	return contract.Submission{SchemaVersion: contract.SchemaVersion, SuggestionID: value.SuggestionID, TenantID: value.TenantID, ProjectID: value.ProjectID,
		Source:       contract.SourceSnapshot{Application: contract.SourceApplicationBonusDistribution, ResourceType: contract.SourceBonusAllocation, ResourceID: value.Source.AllocationID, Revision: value.Source.AllocationRevision, Digest: value.Source.AllocationDigest},
		LedgerOwner:  contract.PartySnapshot{SourceApplication: contract.PartySourceApplicationMoneyFlow, ResourceType: contract.PartyResourceParty, ID: value.LedgerOwner.ID, Revision: value.LedgerOwner.Revision, Kind: value.LedgerOwner.Kind, LegalName: value.LedgerOwner.LegalName},
		Counterparty: contract.PartySnapshot{SourceApplication: contract.PartySourceApplicationPeopleGroups, ResourceType: contract.PartyResourcePerson, ID: value.Source.Person.ID, Revision: value.Source.Person.Revision, Kind: "individual", LegalName: value.Source.Person.DisplayName, Digest: value.Source.Person.Digest},
		AmountMinor:  value.AmountMinor, Currency: value.Currency, DueOn: value.DueOn, Purpose: value.Purpose, RequestDigest: value.RequestDigest}
}

func normalizePayableSuggestionInput(input ProposePayableSuggestionInput) (ProposePayableSuggestionInput, error) {
	input.CommandID, input.AllocationID = normalizeUUID(input.CommandID), normalizeUUID(input.AllocationID)
	input.LedgerOwner.ID = normalizeUUID(input.LedgerOwner.ID)
	input.LedgerOwner.Kind, input.LedgerOwner.LegalName = strings.TrimSpace(input.LedgerOwner.Kind), strings.TrimSpace(input.LedgerOwner.LegalName)
	input.DueOn, input.Purpose = strings.TrimSpace(input.DueOn), strings.TrimSpace(input.Purpose)
	if !validUUID(input.CommandID) || !validUUID(input.AllocationID) || input.AllocationRevision < 1 || !validUUID(input.LedgerOwner.ID) || input.LedgerOwner.Revision < 1 ||
		(input.LedgerOwner.Kind != "organization" && input.LedgerOwner.Kind != "individual") || len([]rune(input.LedgerOwner.LegalName)) < 1 || len([]rune(input.LedgerOwner.LegalName)) > 200 || len([]rune(input.Purpose)) < 1 || len([]rune(input.Purpose)) > 1000 {
		return ProposePayableSuggestionInput{}, fmt.Errorf("invalid bonus payable suggestion")
	}
	parsed, err := time.Parse("2006-01-02", input.DueOn)
	if err != nil || parsed.Format("2006-01-02") != input.DueOn {
		return ProposePayableSuggestionInput{}, fmt.Errorf("invalid payable due date")
	}
	return input, nil
}

func normalizeSuggestionCommand(id string, input SuggestionCommandInput, requireReason bool, prior error) (string, SuggestionCommandInput, error) {
	if prior != nil {
		return "", SuggestionCommandInput{}, prior
	}
	id, input.CommandID, input.Reason = normalizeUUID(id), normalizeUUID(input.CommandID), strings.TrimSpace(input.Reason)
	if !validUUID(id) || !validUUID(input.CommandID) || input.ExpectedRevision < 1 || len([]rune(input.Reason)) > 1000 || requireReason && input.Reason == "" {
		return "", SuggestionCommandInput{}, fmt.Errorf("invalid payable suggestion command")
	}
	return id, input, nil
}

func authorizeSuggestion(actor Actor, action string, value BonusPayableSuggestion) error {
	if !contains(actor.Actions, action) || !hasAllocationAccess(actor, value.Source.AllocationID, value.Source.PoolID, value.Source.Project.ID) {
		return ErrForbidden
	}
	return nil
}

func deterministicSuggestionUUID(seed string) string {
	digest := sha256.Sum256([]byte(seed))
	digest[6] = (digest[6] & 0x0f) | 0x40
	digest[8] = (digest[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", digest[0:4], digest[4:6], digest[6:8], digest[8:10], digest[10:16])
}
