package invoice

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

type Store interface {
	Ping(context.Context) error
	Replay(context.Context, Actor, string, string) (Registration, bool, error)
	Create(context.Context, Actor, CreateInput, ResolvedCreate, string) (Registration, error)
	Get(context.Context, string, string) (Registration, error)
	List(context.Context, string, ListFilter) (ListPage, error)
	History(context.Context, string, string, *int64, int) (HistoryPage, error)
	Correct(context.Context, Actor, string, CorrectInput, ResolvedCorrection, string) (Registration, error)
	Invalidate(context.Context, Actor, string, InvalidateInput, string) (Registration, error)
	ReplaceEvidence(context.Context, Actor, string, EvidenceInput, string) (Registration, error)
}

type ReferenceResolver interface {
	ResolveParty(context.Context, string, PartyRef) (PartyReference, error)
	ResolveSource(context.Context, string, SourceRef) (SourceReference, error)
	ResolveEvidence(context.Context, string, EvidenceRef) (EvidenceReference, error)
}

type Service struct {
	store             Store
	resolver          ReferenceResolver
	allowedCurrencies map[string]struct{}
	now               func() time.Time
}

func NewService(store Store, resolver ReferenceResolver, allowedCurrencies ...string) (*Service, error) {
	if store == nil || resolver == nil || len(allowedCurrencies) == 0 {
		return nil, fmt.Errorf("store, reference resolver and allowed currencies are required")
	}
	allowed := make(map[string]struct{}, len(allowedCurrencies))
	for _, currency := range allowedCurrencies {
		currency = strings.ToUpper(strings.TrimSpace(currency))
		if len(currency) != 3 {
			return nil, fmt.Errorf("invalid allowed currency")
		}
		allowed[currency] = struct{}{}
	}
	return &Service{store: store, resolver: resolver, allowedCurrencies: allowed, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (s *Service) Ping(ctx context.Context) error { return s.store.Ping(ctx) }

func (s *Service) Create(ctx context.Context, actor Actor, input CreateInput) (Registration, error) {
	actor, err := NormalizeActor(actor)
	if err != nil {
		return Registration{}, err
	}
	input, err = NormalizeCreate(input)
	if err != nil {
		return Registration{}, err
	}
	if !s.currencyAllowed(input.Currency) {
		return Registration{}, fmt.Errorf("currency is not enabled")
	}
	hash, err := requestHash("create", "", actor.ID, input)
	if err != nil {
		return Registration{}, err
	}
	if value, found, replayErr := s.store.Replay(ctx, actor, input.CommandID, hash); replayErr != nil || found {
		return value, replayErr
	}
	resolved, err := s.resolveCreate(ctx, actor, input)
	if err != nil {
		return Registration{}, err
	}
	return s.store.Create(ctx, actor, input, resolved, hash)
}

func (s *Service) Get(ctx context.Context, tenantID, id string) (Registration, error) {
	tenantID, id = normalizeUUID(tenantID), normalizeUUID(id)
	if !ValidUUID(tenantID) || !ValidUUID(id) {
		return Registration{}, fmt.Errorf("invalid invoice registration reference")
	}
	return s.store.Get(ctx, tenantID, id)
}

func (s *Service) List(ctx context.Context, tenantID string, query ListQuery) (ListPage, error) {
	tenantID = normalizeUUID(tenantID)
	query.ProjectID = normalizeUUID(query.ProjectID)
	query.PartyID = normalizeUUID(query.PartyID)
	query.Status = strings.TrimSpace(query.Status)
	query.IssuedFrom = strings.TrimSpace(query.IssuedFrom)
	query.IssuedTo = strings.TrimSpace(query.IssuedTo)
	query.Cursor = strings.TrimSpace(query.Cursor)
	if !ValidUUID(tenantID) || (query.ProjectID != "" && !ValidUUID(query.ProjectID)) ||
		(query.PartyID != "" && !ValidUUID(query.PartyID)) ||
		(query.Status != "" && query.Status != StatusRecorded && query.Status != StatusRegistrationInvalidated) ||
		(query.IssuedFrom != "" && !validDate(query.IssuedFrom)) ||
		(query.IssuedTo != "" && !validDate(query.IssuedTo)) ||
		(query.IssuedFrom != "" && query.IssuedTo != "" && query.IssuedFrom > query.IssuedTo) {
		return ListPage{}, fmt.Errorf("invalid invoice registration list query")
	}
	if query.Limit == 0 {
		query.Limit = 50
	}
	if query.Limit < 1 || query.Limit > 100 {
		return ListPage{}, fmt.Errorf("invalid invoice registration list limit")
	}
	filter := ListFilter{ProjectID: query.ProjectID, PartyID: query.PartyID, Status: query.Status, IssuedFrom: query.IssuedFrom, IssuedTo: query.IssuedTo, Limit: query.Limit}
	if query.Cursor != "" {
		cursor, err := decodeListCursor(query.Cursor)
		if err != nil {
			return ListPage{}, fmt.Errorf("invalid invoice registration list cursor")
		}
		filter.BeforeDate, filter.BeforeID = cursor.IssuedOn, cursor.ID
	}
	page, err := s.store.List(ctx, tenantID, filter)
	if err != nil {
		return ListPage{}, err
	}
	if page.NextCursor != "" {
		return ListPage{}, fmt.Errorf("store returned unexpected encoded cursor")
	}
	if len(page.Items) > query.Limit {
		visible := page.Items[:query.Limit]
		last := visible[len(visible)-1]
		page.Items = visible
		page.NextCursor = encodeListCursor(listCursor{IssuedOn: last.IssuedOn, ID: last.ID})
	}
	return page, nil
}

func (s *Service) Reference(ctx context.Context, tenantID, id string) (Reference, error) {
	value, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return Reference{}, err
	}
	projects := make([]string, 0)
	seen := map[string]struct{}{}
	for _, source := range value.SourceRefs {
		if source.Kind != SourceProject || source.ResourceID == nil {
			continue
		}
		if _, exists := seen[*source.ResourceID]; !exists {
			seen[*source.ResourceID] = struct{}{}
			projects = append(projects, *source.ResourceID)
		}
	}
	return Reference{SchemaVersion: "1", SourceApplication: "billing-invoice", ID: value.ID, TenantID: value.TenantID, Status: value.Status, IssuedOn: value.IssuedOn, AmountMinor: value.AmountMinor, Currency: value.Currency, ProjectIDs: projects, Revision: value.Revision}, nil
}

func (s *Service) History(ctx context.Context, tenantID, id string, before *int64, limit int) (HistoryPage, error) {
	tenantID, id = normalizeUUID(tenantID), normalizeUUID(id)
	if !ValidUUID(tenantID) || !ValidUUID(id) || (before != nil && *before < 2) {
		return HistoryPage{}, fmt.Errorf("invalid history query")
	}
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 100 {
		return HistoryPage{}, fmt.Errorf("invalid history limit")
	}
	return s.store.History(ctx, tenantID, id, before, limit)
}

func (s *Service) Correct(ctx context.Context, actor Actor, id string, input CorrectInput) (Registration, error) {
	actor, id, err := normalizeMutation(actor, id)
	if err != nil {
		return Registration{}, err
	}
	input, err = NormalizeCorrect(input)
	if err != nil {
		return Registration{}, err
	}
	if input.Patch.Currency != nil && !s.currencyAllowed(*input.Patch.Currency) {
		return Registration{}, fmt.Errorf("currency is not enabled")
	}
	hash, err := requestHash("correct", id, actor.ID, input)
	if err != nil {
		return Registration{}, err
	}
	if value, found, replayErr := s.store.Replay(ctx, actor, input.CommandID, hash); replayErr != nil || found {
		return value, replayErr
	}
	current, err := s.store.Get(ctx, actor.TenantID, id)
	if err != nil {
		return Registration{}, err
	}
	if current.Status != StatusRecorded {
		return Registration{}, ErrRegistrationInvalidated
	}
	if current.Revision != input.ExpectedRevision {
		return Registration{}, ErrRevisionConflict
	}
	resolved, err := s.resolveCorrection(ctx, actor, input.Patch)
	if err != nil {
		return Registration{}, err
	}
	return s.store.Correct(ctx, actor, id, input, resolved, hash)
}

func (s *Service) Invalidate(ctx context.Context, actor Actor, id string, input InvalidateInput) (Registration, error) {
	actor, id, err := normalizeMutation(actor, id)
	if err != nil {
		return Registration{}, err
	}
	input, err = NormalizeInvalidate(input)
	if err != nil {
		return Registration{}, err
	}
	hash, err := requestHash("invalidate", id, actor.ID, input)
	if err != nil {
		return Registration{}, err
	}
	if value, found, replayErr := s.store.Replay(ctx, actor, input.CommandID, hash); replayErr != nil || found {
		return value, replayErr
	}
	return s.store.Invalidate(ctx, actor, id, input, hash)
}

func (s *Service) ReplaceEvidence(ctx context.Context, actor Actor, id string, input EvidenceInput) (Registration, error) {
	actor, id, err := normalizeMutation(actor, id)
	if err != nil {
		return Registration{}, err
	}
	input, err = NormalizeEvidence(input)
	if err != nil {
		return Registration{}, err
	}
	hash, err := requestHash("evidence", id, actor.ID, input)
	if err != nil {
		return Registration{}, err
	}
	if value, found, replayErr := s.store.Replay(ctx, actor, input.CommandID, hash); replayErr != nil || found {
		return value, replayErr
	}
	current, err := s.store.Get(ctx, actor.TenantID, id)
	if err != nil {
		return Registration{}, err
	}
	if current.Status != StatusRecorded {
		return Registration{}, ErrRegistrationInvalidated
	}
	if current.Revision != input.ExpectedRevision {
		return Registration{}, ErrRevisionConflict
	}
	for _, reference := range input.EvidenceRefs {
		resolved, resolveErr := s.resolver.ResolveEvidence(ctx, actor.TenantID, reference)
		if resolveErr != nil {
			return Registration{}, resolveErr
		}
		if resolved.TenantID != actor.TenantID || resolved.FileID != reference.FileID || resolved.Version != reference.Version || resolved.Revision != reference.Version || resolved.Status != "available" {
			return Registration{}, ErrReferenceInvalid
		}
	}
	return s.store.ReplaceEvidence(ctx, actor, id, input, hash)
}

func (s *Service) resolveCreate(ctx context.Context, actor Actor, input CreateInput) (ResolvedCreate, error) {
	issuer, err := s.resolveParty(ctx, actor.TenantID, input.IssuerPartyRef)
	if err != nil {
		return ResolvedCreate{}, err
	}
	recipient, err := s.resolveParty(ctx, actor.TenantID, input.RecipientPartyRef)
	if err != nil {
		return ResolvedCreate{}, err
	}
	if issuer.ID == recipient.ID {
		return ResolvedCreate{}, ErrReferenceInvalid
	}
	sources, err := s.resolveSources(ctx, actor, input.SourceRefs)
	if err != nil {
		return ResolvedCreate{}, err
	}
	return ResolvedCreate{IssuerParty: issuer, RecipientParty: recipient, SourceRefs: sources}, nil
}

func (s *Service) resolveCorrection(ctx context.Context, actor Actor, patch CorrectionPatch) (ResolvedCorrection, error) {
	var resolved ResolvedCorrection
	if patch.IssuerPartyRef != nil {
		value, err := s.resolveParty(ctx, actor.TenantID, *patch.IssuerPartyRef)
		if err != nil {
			return ResolvedCorrection{}, err
		}
		resolved.IssuerParty = &value
	}
	if patch.RecipientPartyRef != nil {
		value, err := s.resolveParty(ctx, actor.TenantID, *patch.RecipientPartyRef)
		if err != nil {
			return ResolvedCorrection{}, err
		}
		resolved.RecipientParty = &value
	}
	if patch.SourceRefs != nil {
		value, err := s.resolveSources(ctx, actor, *patch.SourceRefs)
		if err != nil {
			return ResolvedCorrection{}, err
		}
		resolved.SourceRefs = &value
	}
	return resolved, nil
}

func (s *Service) resolveParty(ctx context.Context, tenantID string, reference PartyRef) (PartySnapshot, error) {
	value, err := s.resolver.ResolveParty(ctx, tenantID, reference)
	if err != nil {
		return PartySnapshot{}, err
	}
	if value.TenantID != tenantID || value.ID != reference.ID || value.Revision != reference.Revision || value.Status != "active" || !validText(value.Kind, 1, 100) || !validText(value.LegalName, 1, 200) {
		return PartySnapshot{}, ErrReferenceInvalid
	}
	return value.PartySnapshot, nil
}

func (s *Service) resolveSources(ctx context.Context, actor Actor, references []SourceRef) ([]SourceSnapshot, error) {
	values := make([]SourceSnapshot, 0, len(references))
	for _, reference := range references {
		if reference.Kind == SourceManualConfirmation {
			values = append(values, SourceSnapshot{Kind: reference.Kind, DisplaySummary: *reference.ManualSummary, ConfirmedBy: actor.ID, ConfirmedAt: s.now()})
			continue
		}
		resolved, err := s.resolver.ResolveSource(ctx, actor.TenantID, reference)
		if err != nil {
			return nil, err
		}
		expectedApplication := sourceApplication(reference.Kind)
		if resolved.TenantID != actor.TenantID || resolved.SourceApplication != expectedApplication || resolved.ResourceID != *reference.ResourceID || resolved.Revision != *reference.SourceRevision || resolved.Status == "" || !validText(resolved.DisplaySummary, 1, 500) {
			return nil, ErrReferenceInvalid
		}
		application, resourceID, revision := resolved.SourceApplication, resolved.ResourceID, resolved.Revision
		values = append(values, SourceSnapshot{Kind: reference.Kind, SourceApplication: &application, ResourceID: &resourceID, SourceRevision: &revision, DisplaySummary: resolved.DisplaySummary, ConfirmedBy: actor.ID, ConfirmedAt: s.now()})
	}
	return values, nil
}

func sourceApplication(kind string) string {
	switch kind {
	case SourceProject:
		return "project-workspace"
	case SourceContract, SourceSettlement:
		return "contract-ledger"
	case SourceMoneyObligation:
		return "money-flow-ledger"
	default:
		return ""
	}
}

func (s *Service) currencyAllowed(currency string) bool {
	_, ok := s.allowedCurrencies[strings.ToUpper(strings.TrimSpace(currency))]
	return ok
}

func normalizeMutation(actor Actor, id string) (Actor, string, error) {
	actor, err := NormalizeActor(actor)
	if err != nil {
		return Actor{}, "", err
	}
	id = normalizeUUID(id)
	if !ValidUUID(id) {
		return Actor{}, "", fmt.Errorf("invalid invoice registration id")
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
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

type listCursor struct {
	IssuedOn string `json:"issued_on"`
	ID       string `json:"id"`
}

func encodeListCursor(value listCursor) string {
	raw, _ := json.Marshal(value)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeListCursor(raw string) (listCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(data) > 256 {
		return listCursor{}, fmt.Errorf("invalid cursor")
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var value listCursor
	if err = decoder.Decode(&value); err != nil || !validDate(value.IssuedOn) || !ValidUUID(normalizeUUID(value.ID)) {
		return listCursor{}, fmt.Errorf("invalid cursor")
	}
	var trailing any
	if err = decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return listCursor{}, fmt.Errorf("invalid cursor")
	}
	value.ID = normalizeUUID(value.ID)
	return value, nil
}
