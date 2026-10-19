package costoutsourcing

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

func (s *Service) ProposePayableSuggestion(ctx context.Context, actor Actor, input ProposeSuggestionInput) (PayableSuggestion, error) {
	actor, err := normalizeActor(actor)
	if err != nil {
		return PayableSuggestion{}, err
	}
	input, err = normalizeProposeSuggestion(input)
	if err != nil {
		return PayableSuggestion{}, err
	}
	hash, err := requestHash("propose_payable_suggestion", "", actor.ID, input)
	if err != nil {
		return PayableSuggestion{}, err
	}
	var replay PayableSuggestion
	if found, replayErr := s.store.Replay(ctx, actor, input.CommandID, hash, "payable_suggestion", &replay); replayErr != nil {
		return PayableSuggestion{}, replayErr
	} else if found {
		current, getErr := s.store.GetSuggestion(ctx, actor.TenantID, replay.ID)
		if getErr != nil {
			return PayableSuggestion{}, getErr
		}
		if _, getErr = authorizeResource(actor, ActionSuggestionCreate, current.Project.ID, "payable_suggestion", current.ID); getErr != nil {
			return PayableSuggestion{}, getErr
		}
		if current.State == SuggestionTargetUnavailable {
			return s.submitSuggestion(ctx, actor, current)
		}
		return current, nil
	}

	project, source, counterparty, amountMinor, currency, err := s.suggestionSource(ctx, actor, input)
	if err != nil {
		return PayableSuggestion{}, err
	}
	if _, err = authorize(actor, ActionSuggestionCreate, project.ID); err != nil {
		return PayableSuggestion{}, err
	}
	owner, err := s.resolveSuggestionParty(ctx, actor.TenantID, input.LedgerOwnerRef)
	if err != nil {
		return PayableSuggestion{}, err
	}
	if owner.ID == counterparty.ID {
		return PayableSuggestion{}, fmt.Errorf("ledger owner and counterparty must differ")
	}
	id, err := newUUID()
	if err != nil {
		return PayableSuggestion{}, err
	}
	now := time.Now().UTC()
	value := PayableSuggestion{
		ID: id, TenantID: actor.TenantID, Project: project, Source: source,
		LedgerOwner: owner, Counterparty: counterparty, AmountMinor: amountMinor,
		Currency: currency, DueOn: input.DueOn, Purpose: input.Purpose,
		State: SuggestionTargetUnavailable, Revision: 1, CreatedBy: actor.ID,
		UpdatedBy: actor.ID, CreatedAt: now, UpdatedAt: now,
	}
	submission := submissionFrom(value)
	digest, err := contract.Digest(toWireSubmission(submission))
	if err != nil {
		return PayableSuggestion{}, err
	}
	value.RequestDigest = digest
	created, err := s.store.CreateSuggestion(ctx, actor, value, input.CommandID, hash)
	if err != nil {
		return PayableSuggestion{}, err
	}
	return s.submitSuggestion(ctx, actor, created)
}

func (s *Service) GetPayableSuggestion(ctx context.Context, actor Actor, id string) (PayableSuggestion, error) {
	actor, id, err := normalizeMutationActor(actor, id)
	if err != nil {
		return PayableSuggestion{}, err
	}
	value, err := s.store.GetSuggestion(ctx, actor.TenantID, id)
	if err != nil {
		return PayableSuggestion{}, err
	}
	if _, err = authorizeResource(actor, ActionSuggestionRead, value.Project.ID, "payable_suggestion", value.ID); err != nil {
		return PayableSuggestion{}, err
	}
	return value, nil
}

func (s *Service) ListPayableSuggestions(ctx context.Context, actor Actor, projectID string) ([]PayableSuggestion, error) {
	actor, err := authorize(actor, ActionSuggestionRead, projectID)
	if err != nil {
		return nil, err
	}
	return s.store.ListSuggestions(ctx, actor.TenantID, normalizeUUID(projectID))
}

func (s *Service) PayableSuggestionHistory(ctx context.Context, actor Actor, id string) ([]PayableSuggestionRevision, error) {
	if _, err := s.GetPayableSuggestion(ctx, actor, id); err != nil {
		return nil, err
	}
	return s.store.SuggestionHistory(ctx, actor.TenantID, normalizeUUID(id))
}

func (s *Service) SynchronizePayableSuggestion(ctx context.Context, actor Actor, id string, input SuggestionCommandInput) (PayableSuggestion, error) {
	actor, id, err := normalizeMutationActor(actor, id)
	if err != nil {
		return PayableSuggestion{}, err
	}
	input, err = normalizeSuggestionCommand(input, false)
	if err != nil {
		return PayableSuggestion{}, err
	}
	hash, err := requestHash("synchronize_payable_suggestion", id, actor.ID, input)
	if err != nil {
		return PayableSuggestion{}, err
	}
	var replay PayableSuggestion
	if found, replayErr := s.store.Replay(ctx, actor, input.CommandID, hash, "payable_suggestion", &replay); replayErr != nil || found {
		if found {
			_, replayErr = authorizeResource(actor, ActionSuggestionCreate, replay.Project.ID, "payable_suggestion", replay.ID)
		}
		return replay, replayErr
	}
	current, err := s.store.GetSuggestion(ctx, actor.TenantID, id)
	if err != nil {
		return PayableSuggestion{}, err
	}
	if _, err = authorizeResource(actor, ActionSuggestionCreate, current.Project.ID, "payable_suggestion", current.ID); err != nil {
		return PayableSuggestion{}, err
	}
	if current.Revision != input.ExpectedRevision {
		return PayableSuggestion{}, ErrRevisionConflict
	}
	if current.State == SuggestionTargetUnavailable && current.TargetRevision == 0 {
		if s.money == nil {
			return PayableSuggestion{}, ErrTargetUnavailable
		}
		intake, submitErr := s.money.SubmitPayableSuggestion(ctx, actor.TenantID, current.ID, submissionFrom(current))
		if submitErr != nil {
			return PayableSuggestion{}, ErrTargetUnavailable
		}
		return s.applyIntake(ctx, actor, current, input.CommandID, hash, "submitted", input.Reason, intake)
	}
	if s.money == nil {
		return PayableSuggestion{}, ErrTargetUnavailable
	}
	intake, err := s.money.GetPayableSuggestion(ctx, actor.TenantID, id)
	if err != nil {
		return PayableSuggestion{}, ErrTargetUnavailable
	}
	return s.applyIntake(ctx, actor, current, input.CommandID, hash, "synchronized", input.Reason, intake)
}

func (s *Service) WithdrawPayableSuggestion(ctx context.Context, actor Actor, id string, input SuggestionCommandInput) (PayableSuggestion, error) {
	actor, id, err := normalizeMutationActor(actor, id)
	if err != nil {
		return PayableSuggestion{}, err
	}
	input, err = normalizeSuggestionCommand(input, true)
	if err != nil {
		return PayableSuggestion{}, err
	}
	hash, err := requestHash("withdraw_payable_suggestion", id, actor.ID, input)
	if err != nil {
		return PayableSuggestion{}, err
	}
	var replay PayableSuggestion
	if found, replayErr := s.store.Replay(ctx, actor, input.CommandID, hash, "payable_suggestion", &replay); replayErr != nil || found {
		if found {
			_, replayErr = authorizeResource(actor, ActionSuggestionWithdraw, replay.Project.ID, "payable_suggestion", replay.ID)
		}
		return replay, replayErr
	}
	current, err := s.store.GetSuggestion(ctx, actor.TenantID, id)
	if err != nil {
		return PayableSuggestion{}, err
	}
	if _, err = authorizeResource(actor, ActionSuggestionWithdraw, current.Project.ID, "payable_suggestion", current.ID); err != nil {
		return PayableSuggestion{}, err
	}
	if current.Revision != input.ExpectedRevision {
		return PayableSuggestion{}, ErrRevisionConflict
	}
	if current.State == SuggestionTargetUnavailable && current.TargetRevision == 0 {
		current, err = s.submitSuggestion(ctx, actor, current)
		if err != nil || current.TargetRevision == 0 {
			return current, err
		}
	}
	if current.State != SuggestionProposed {
		return PayableSuggestion{}, ErrRevisionConflict
	}
	if s.money == nil {
		return PayableSuggestion{}, ErrTargetUnavailable
	}
	intake, err := s.money.WithdrawPayableSuggestion(ctx, actor.TenantID, id, input.CommandID, current.TargetRevision, input.Reason)
	if err != nil {
		return PayableSuggestion{}, ErrTargetUnavailable
	}
	return s.applyIntake(ctx, actor, current, input.CommandID, hash, "withdrawn", input.Reason, intake)
}

func (s *Service) suggestionSource(ctx context.Context, actor Actor, input ProposeSuggestionInput) (ProjectSnapshot, PayableSourceSnapshot, PartySnapshot, int64, string, error) {
	var project ProjectSnapshot
	var counterparty PartySnapshot
	var amount int64
	var currency string
	var raw []byte
	if input.SourceType == contract.SourceCostEntry {
		value, err := s.store.GetCost(ctx, actor.TenantID, input.SourceID)
		if err != nil {
			return project, PayableSourceSnapshot{}, counterparty, 0, "", err
		}
		if value.Revision != input.SourceRevision || value.State != StateRecorded || input.CounterpartyRef == nil {
			return project, PayableSourceSnapshot{}, counterparty, 0, "", ErrReferenceInvalid
		}
		counterparty, err = s.resolveSuggestionParty(ctx, actor.TenantID, *input.CounterpartyRef)
		if err != nil {
			return project, PayableSourceSnapshot{}, counterparty, 0, "", err
		}
		project, amount, currency = value.Project, value.AmountMinor, value.Currency
		raw, _ = json.Marshal(value)
	} else {
		value, err := s.store.GetEngagement(ctx, actor.TenantID, input.SourceID)
		if err != nil {
			return project, PayableSourceSnapshot{}, counterparty, 0, "", err
		}
		if value.Revision != input.SourceRevision || value.State != StateRecorded {
			return project, PayableSourceSnapshot{}, counterparty, 0, "", ErrReferenceInvalid
		}
		if input.CounterpartyRef != nil && (input.CounterpartyRef.ID != value.Provider.ID || input.CounterpartyRef.Revision != value.Provider.Revision) {
			return project, PayableSourceSnapshot{}, counterparty, 0, "", ErrReferenceInvalid
		}
		project, counterparty, amount, currency = value.Project, value.Provider, value.AmountMinor, value.Currency
		raw, _ = json.Marshal(value)
	}
	if amount <= 0 {
		return project, PayableSourceSnapshot{}, counterparty, 0, "", ErrReferenceInvalid
	}
	digest := sha256.Sum256(raw)
	source := PayableSourceSnapshot{Type: input.SourceType, ID: input.SourceID, Revision: input.SourceRevision, Digest: "sha256:" + hex.EncodeToString(digest[:])}
	return project, source, counterparty, amount, currency, nil
}

func (s *Service) resolveSuggestionParty(ctx context.Context, tenantID string, ref PartyRef) (PartySnapshot, error) {
	ref, err := normalizePartyRef(ref)
	if err != nil {
		return PartySnapshot{}, err
	}
	value, err := s.resolver.ResolveParty(ctx, tenantID, ref)
	if err != nil {
		return PartySnapshot{}, err
	}
	if value.TenantID != tenantID || value.PartyRef() != ref || value.Status != "active" || !validText(value.LegalName, 1, 200) {
		return PartySnapshot{}, ErrReferenceInvalid
	}
	return value.PartySnapshot, nil
}

func (s *Service) submitSuggestion(ctx context.Context, actor Actor, current PayableSuggestion) (PayableSuggestion, error) {
	if s.money == nil {
		return current, nil
	}
	intake, err := s.money.SubmitPayableSuggestion(ctx, actor.TenantID, current.ID, submissionFrom(current))
	if err != nil {
		return current, nil
	}
	commandID := deterministicUUID(current.ID + ":target:" + fmt.Sprint(intake.Revision) + ":" + intake.Status)
	hash, err := requestHash("apply_payable_suggestion_target", current.ID, actor.ID, intake)
	if err != nil {
		return PayableSuggestion{}, err
	}
	return s.applyIntake(ctx, actor, current, commandID, hash, "submitted", "", intake)
}

func (s *Service) applyIntake(ctx context.Context, actor Actor, current PayableSuggestion, commandID, hash, event, reason string, intake PayableIntake) (PayableSuggestion, error) {
	if intake.SuggestionID != current.ID || intake.TenantID != current.TenantID || intake.RequestDigest != current.RequestDigest || intake.Revision < 1 {
		return PayableSuggestion{}, ErrReferenceInvalid
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
		return PayableSuggestion{}, ErrReferenceInvalid
	}
	if intake.Revision < current.TargetRevision {
		return PayableSuggestion{}, ErrReferenceInvalid
	}
	if intake.Revision == current.TargetRevision && state == current.State {
		return current, nil
	}
	updated := current
	updated.State, updated.TargetRevision, updated.DecisionReason = state, intake.Revision, intake.DecisionReason
	updated.Obligation = intake.Obligation
	return s.store.ApplySuggestionTarget(ctx, actor, current.ID, current.Revision, commandID, hash, event, reason, updated)
}

func submissionFrom(value PayableSuggestion) PayableSubmission {
	return PayableSubmission{SuggestionID: value.ID, TenantID: value.TenantID, ProjectID: value.Project.ID, Source: value.Source, LedgerOwner: value.LedgerOwner, Counterparty: value.Counterparty, AmountMinor: value.AmountMinor, Currency: value.Currency, DueOn: value.DueOn, Purpose: value.Purpose, RequestDigest: value.RequestDigest}
}

func toWireSubmission(value PayableSubmission) contract.Submission {
	return contract.Submission{SchemaVersion: contract.SchemaVersion, SuggestionID: value.SuggestionID, TenantID: value.TenantID, ProjectID: value.ProjectID,
		Source:       contract.SourceSnapshot{Application: contract.SourceApplication, ResourceType: value.Source.Type, ResourceID: value.Source.ID, Revision: value.Source.Revision, Digest: value.Source.Digest},
		LedgerOwner:  contract.PartySnapshot{ID: value.LedgerOwner.ID, Revision: value.LedgerOwner.Revision, Kind: value.LedgerOwner.Kind, LegalName: value.LedgerOwner.LegalName},
		Counterparty: contract.PartySnapshot{ID: value.Counterparty.ID, Revision: value.Counterparty.Revision, Kind: value.Counterparty.Kind, LegalName: value.Counterparty.LegalName},
		AmountMinor:  value.AmountMinor, Currency: value.Currency, DueOn: value.DueOn, Purpose: value.Purpose, RequestDigest: value.RequestDigest}
}

func normalizeProposeSuggestion(input ProposeSuggestionInput) (ProposeSuggestionInput, error) {
	input.CommandID, input.SourceType, input.SourceID = normalizeUUID(input.CommandID), strings.TrimSpace(input.SourceType), normalizeUUID(input.SourceID)
	input.DueOn, input.Purpose = strings.TrimSpace(input.DueOn), strings.TrimSpace(input.Purpose)
	if !validUUID(input.CommandID) || (input.SourceType != contract.SourceCostEntry && input.SourceType != contract.SourceEngagement) || !validUUID(input.SourceID) || input.SourceRevision < 1 || !datePattern.MatchString(input.DueOn) || !validText(input.Purpose, 1, 1000) {
		return ProposeSuggestionInput{}, fmt.Errorf("invalid payable suggestion")
	}
	if parsed, err := time.Parse("2006-01-02", input.DueOn); err != nil || parsed.Format("2006-01-02") != input.DueOn {
		return ProposeSuggestionInput{}, fmt.Errorf("invalid payable due date")
	}
	return input, nil
}

func normalizeSuggestionCommand(input SuggestionCommandInput, requireReason bool) (SuggestionCommandInput, error) {
	input.CommandID, input.Reason = normalizeUUID(input.CommandID), strings.TrimSpace(input.Reason)
	if !validUUID(input.CommandID) || input.ExpectedRevision < 1 || len([]rune(input.Reason)) > 1000 || (requireReason && input.Reason == "") {
		return SuggestionCommandInput{}, fmt.Errorf("invalid payable suggestion command")
	}
	return input, nil
}

func deterministicUUID(seed string) string {
	digest := sha256.Sum256([]byte(seed))
	digest[6] = (digest[6] & 0x0f) | 0x40
	digest[8] = (digest[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", digest[0:4], digest[4:6], digest[6:8], digest[8:10], digest[10:16])
}
