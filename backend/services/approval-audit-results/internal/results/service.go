package results

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)

type Store interface {
	Ping(context.Context) error
	Replay(context.Context, Actor, string, string) (Result, bool, error)
	Record(context.Context, Actor, RecordInput, ResolvedResult, string) (Result, error)
	Get(context.Context, string, string) (Result, error)
	ListProject(context.Context, string, string) ([]Result, error)
	GetRevision(context.Context, string, string, int64) (Revision, error)
	History(context.Context, string, string) ([]Revision, error)
	Correct(context.Context, Actor, string, CorrectInput, ResolvedResult, string) (Result, error)
	Withdraw(context.Context, Actor, string, WithdrawInput, string) (Result, error)
}

type ReferenceResolver interface {
	ResolveProject(context.Context, string, ProjectRef) (ProjectReference, error)
	ResolveSource(context.Context, string, ProjectRef, SourceRef) (SourceReference, error)
	ResolveEvidence(context.Context, string, EvidenceRef) (EvidenceReference, error)
}

type Service struct {
	store    Store
	resolver ReferenceResolver
}

func NewService(store Store, resolver ReferenceResolver) (*Service, error) {
	if store == nil || resolver == nil {
		return nil, fmt.Errorf("store and reference resolver are required")
	}
	return &Service{store: store, resolver: resolver}, nil
}

func (s *Service) Ping(ctx context.Context) error { return s.store.Ping(ctx) }

func (s *Service) Record(ctx context.Context, actor Actor, input RecordInput) (Result, error) {
	actor, err := NormalizeActor(actor)
	if err != nil {
		return Result{}, err
	}
	input, err = NormalizeRecord(input)
	if err != nil {
		return Result{}, err
	}
	hash, err := requestHash("record", "", actor.ID, input)
	if err != nil {
		return Result{}, err
	}
	if value, found, replayErr := s.store.Replay(ctx, actor, input.CommandID, hash); replayErr != nil || found {
		return value, replayErr
	}
	resolved, err := s.resolve(ctx, actor, input.ProjectRef, input.Kind, input.Currency, input.AmountComponents, input.SourceRef, input.EvidenceRefs, input.OverrideReason, input.ExternalConfirmedBy, input.ExternalConfirmedAt)
	if err != nil {
		return Result{}, err
	}
	return s.store.Record(ctx, actor, input, resolved, hash)
}

func (s *Service) Correct(ctx context.Context, actor Actor, id string, input CorrectInput) (Result, error) {
	actor, id, err := normalizeMutation(actor, id)
	if err != nil {
		return Result{}, err
	}
	current, err := s.store.Get(ctx, actor.TenantID, id)
	if err != nil {
		return Result{}, err
	}
	input, err = NormalizeCorrect(input, current.Kind)
	if err != nil {
		return Result{}, err
	}
	hash, err := requestHash("correct", id, actor.ID, input)
	if err != nil {
		return Result{}, err
	}
	if value, found, replayErr := s.store.Replay(ctx, actor, input.CommandID, hash); replayErr != nil || found {
		return value, replayErr
	}
	// A correction must validate the Project's current active state. The Project
	// revision frozen on the original result remains historical evidence, but it
	// can legitimately be older after an independent Project command (for
	// example a completion checkpoint confirmation) advances the Project.
	resolved, err := s.resolve(ctx, actor, ProjectRef{ID: current.Project.ID}, current.Kind, input.Currency, input.AmountComponents, input.SourceRef, input.EvidenceRefs, input.OverrideReason, input.ExternalConfirmedBy, input.ExternalConfirmedAt)
	if err != nil {
		return Result{}, err
	}
	return s.store.Correct(ctx, actor, id, input, resolved, hash)
}

func (s *Service) Withdraw(ctx context.Context, actor Actor, id string, input WithdrawInput) (Result, error) {
	actor, id, err := normalizeMutation(actor, id)
	if err != nil {
		return Result{}, err
	}
	input, err = NormalizeWithdraw(input)
	if err != nil {
		return Result{}, err
	}
	hash, err := requestHash("withdraw", id, actor.ID, input)
	if err != nil {
		return Result{}, err
	}
	if value, found, replayErr := s.store.Replay(ctx, actor, input.CommandID, hash); replayErr != nil || found {
		return value, replayErr
	}
	return s.store.Withdraw(ctx, actor, id, input, hash)
}

func (s *Service) Get(ctx context.Context, tenantID, id string) (Result, error) {
	tenantID, id = normalizeUUID(tenantID), normalizeUUID(id)
	if !ValidUUID(tenantID) || !ValidUUID(id) {
		return Result{}, fmt.Errorf("invalid result lookup")
	}
	return s.store.Get(ctx, tenantID, id)
}

func (s *Service) ListProject(ctx context.Context, tenantID, projectID string) ([]Result, error) {
	tenantID, projectID = normalizeUUID(tenantID), normalizeUUID(projectID)
	if !ValidUUID(tenantID) || !ValidUUID(projectID) {
		return nil, fmt.Errorf("invalid project result lookup")
	}
	return s.store.ListProject(ctx, tenantID, projectID)
}

func (s *Service) ReferenceProjectResults(ctx context.Context, tenantID, projectID string) (ProjectResultReferenceList, error) {
	values, err := s.ListProject(ctx, tenantID, projectID)
	if err != nil {
		return ProjectResultReferenceList{}, err
	}
	items := make([]ProjectResultReference, 0, len(values))
	for _, value := range values {
		items = append(items, ProjectResultReference{
			ID: value.ID, TenantID: value.TenantID, ProjectID: value.Project.ID,
			ResourceType: "approval_audit_result", Kind: value.Kind, State: value.State,
			Revision: value.Revision, Digest: resultDigest(value),
			DisplaySummary: fmt.Sprintf("%s result revision %d", value.Kind, value.Revision),
		})
	}
	return ProjectResultReferenceList{SchemaVersion: "1", SourceApplication: "approval-audit-results", ProjectID: projectID, Items: items}, nil
}

func (s *Service) History(ctx context.Context, tenantID, id string) ([]Revision, error) {
	tenantID, id = normalizeUUID(tenantID), normalizeUUID(id)
	if !ValidUUID(tenantID) || !ValidUUID(id) {
		return nil, fmt.Errorf("invalid result history lookup")
	}
	return s.store.History(ctx, tenantID, id)
}

func (s *Service) resolve(ctx context.Context, actor Actor, projectRef ProjectRef, kind, currency string, requested []AmountComponent, sourceRef SourceRef, evidenceRefs []EvidenceRef, overrideReason, confirmedBy string, confirmedAt time.Time) (ResolvedResult, error) {
	project, err := s.resolver.ResolveProject(ctx, actor.TenantID, projectRef)
	if err != nil {
		return ResolvedResult{}, err
	}
	if project.TenantID != actor.TenantID || project.ID != projectRef.ID || (projectRef.Revision > 0 && project.Revision != projectRef.Revision) || project.Status != "active" || !validText(project.Code, 1, 100) || !validText(project.DisplayName, 1, 200) {
		return ResolvedResult{}, ErrReferenceInvalid
	}
	source, err := s.resolveSource(ctx, actor.TenantID, projectRef, kind, sourceRef)
	if err != nil {
		return ResolvedResult{}, err
	}
	if source.Currency != currency || source.AmountProfile != AmountProfileReferenceOAV1 {
		return ResolvedResult{}, ErrReferenceInvalid
	}
	components := requested
	if len(components) == 0 {
		components = append([]AmountComponent(nil), source.AmountComponents...)
	}
	components, total, err := NormalizeComponents(components, false)
	if err != nil {
		return ResolvedResult{}, err
	}
	if (!reflect.DeepEqual(components, source.AmountComponents) || total != source.TotalAmountMinor) && strings.TrimSpace(overrideReason) == "" {
		return ResolvedResult{}, fmt.Errorf("override reason is required when amounts differ from the exact source revision")
	}
	evidence, err := s.resolveEvidence(ctx, actor.TenantID, evidenceRefs)
	if err != nil {
		return ResolvedResult{}, err
	}
	return ResolvedResult{Project: project.ProjectSnapshot, Currency: currency, AmountComponents: components, TotalAmountMinor: total, Source: source, Evidence: evidence, OverrideReason: strings.TrimSpace(overrideReason), ExternalConfirmedBy: confirmedBy, ExternalConfirmedAt: confirmedAt.UTC()}, nil
}

func (s *Service) resolveSource(ctx context.Context, tenantID string, projectRef ProjectRef, kind string, reference SourceRef) (SourceSnapshot, error) {
	if kind == KindAudit {
		revision, err := s.store.GetRevision(ctx, tenantID, reference.ResourceID, reference.Revision)
		if err != nil {
			return SourceSnapshot{}, err
		}
		value := revision.Snapshot
		if value.Kind != KindApproval || value.State != StateRecorded || value.Project.ID != projectRef.ID || value.Revision != reference.Revision {
			return SourceSnapshot{}, ErrReferenceInvalid
		}
		components, total, normalizeErr := NormalizeComponents(value.AmountComponents, false)
		if normalizeErr != nil || total != value.TotalAmountMinor || value.AmountProfile != AmountProfileReferenceOAV1 || !currencyPattern.MatchString(value.Currency) {
			return SourceSnapshot{}, ErrReferenceInvalid
		}
		return SourceSnapshot{SourceRef: reference, State: value.State, ProjectID: value.Project.ID, Currency: value.Currency, AmountProfile: value.AmountProfile, AmountComponents: components, TotalAmountMinor: total, Digest: resultDigest(value), DisplaySummary: fmt.Sprintf("approval result %s revision %d", value.ID, value.Revision)}, nil
	}
	value, err := s.resolver.ResolveSource(ctx, tenantID, projectRef, reference)
	if err != nil {
		return SourceSnapshot{}, err
	}
	components, total, normalizeErr := NormalizeComponents(value.AmountComponents, false)
	if normalizeErr != nil {
		return SourceSnapshot{}, ErrReferenceInvalid
	}
	value.AmountComponents, value.TotalAmountMinor = components, total
	if value.TenantID != tenantID || value.SourceRef != reference || !value.Eligible || value.ProjectID != projectRef.ID || value.State == "" || value.AmountProfile != AmountProfileReferenceOAV1 || !currencyPattern.MatchString(value.Currency) || !validText(value.Digest, 1, 256) || !validText(value.DisplaySummary, 1, 500) {
		return SourceSnapshot{}, ErrReferenceInvalid
	}
	return value.SourceSnapshot, nil
}

func (s *Service) resolveEvidence(ctx context.Context, tenantID string, references []EvidenceRef) ([]EvidenceSnapshot, error) {
	values := make([]EvidenceSnapshot, 0, len(references))
	for _, reference := range references {
		value, err := s.resolver.ResolveEvidence(ctx, tenantID, reference)
		if err != nil {
			return nil, err
		}
		if value.TenantID != tenantID || value.EvidenceRef != reference || value.Status != "available" || !validText(value.Digest, 1, 256) || !validText(value.DisplayName, 1, 300) {
			return nil, ErrReferenceInvalid
		}
		values = append(values, value.EvidenceSnapshot)
	}
	return values, nil
}

func normalizeMutation(actor Actor, id string) (Actor, string, error) {
	actor, err := NormalizeActor(actor)
	if err != nil {
		return Actor{}, "", err
	}
	id = normalizeUUID(id)
	if !ValidUUID(id) {
		return Actor{}, "", fmt.Errorf("invalid result id")
	}
	return actor, id, nil
}

func requestHash(operation, id, actorID string, value any) (string, error) {
	raw, err := json.Marshal(struct {
		Operation string `json:"operation"`
		ID        string `json:"id"`
		ActorID   string `json:"actor_id"`
		Value     any    `json:"value"`
	}{operation, id, strings.ToLower(actorID), value})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func resultDigest(value Result) string {
	raw, _ := json.Marshal(struct {
		ID         string            `json:"id"`
		Revision   int64             `json:"revision"`
		Currency   string            `json:"currency"`
		Components []AmountComponent `json:"components"`
		Total      int64             `json:"total"`
	}{value.ID, value.Revision, value.Currency, value.AmountComponents, value.TotalAmountMinor})
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}
