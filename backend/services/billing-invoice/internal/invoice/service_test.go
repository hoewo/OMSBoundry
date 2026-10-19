package invoice

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

type fakeStore struct {
	replayValue Registration
	replayFound bool
	current     Registration
	listPage    ListPage
	listFilter  ListFilter
}

func (s *fakeStore) Ping(context.Context) error { return nil }
func (s *fakeStore) Replay(context.Context, Actor, string, string) (Registration, bool, error) {
	return s.replayValue, s.replayFound, nil
}
func (s *fakeStore) Create(_ context.Context, actor Actor, input CreateInput, resolved ResolvedCreate, _ string) (Registration, error) {
	return Registration{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", TenantID: actor.TenantID, Status: StatusRecorded, IssuedOn: input.IssuedOn, AmountMinor: input.AmountMinor, Currency: input.Currency, IssuerParty: resolved.IssuerParty, RecipientParty: resolved.RecipientParty, SourceRefs: resolved.SourceRefs, Revision: 1}, nil
}
func (s *fakeStore) Get(context.Context, string, string) (Registration, error) { return s.current, nil }
func (s *fakeStore) List(_ context.Context, _ string, filter ListFilter) (ListPage, error) {
	s.listFilter = filter
	return s.listPage, nil
}
func (s *fakeStore) History(context.Context, string, string, *int64, int) (HistoryPage, error) {
	return HistoryPage{}, nil
}
func (s *fakeStore) Correct(context.Context, Actor, string, CorrectInput, ResolvedCorrection, string) (Registration, error) {
	return Registration{}, nil
}
func (s *fakeStore) Invalidate(context.Context, Actor, string, InvalidateInput, string) (Registration, error) {
	return Registration{}, nil
}
func (s *fakeStore) ReplaceEvidence(context.Context, Actor, string, EvidenceInput, string) (Registration, error) {
	return Registration{}, nil
}

type fakeResolver struct {
	calls atomic.Int64
	fail  error
}

func (r *fakeResolver) ResolveParty(_ context.Context, tenant string, reference PartyRef) (PartyReference, error) {
	r.calls.Add(1)
	if r.fail != nil {
		return PartyReference{}, r.fail
	}
	return PartyReference{PartySnapshot: PartySnapshot{ID: reference.ID, Revision: reference.Revision, Kind: "organization", LegalName: "主体"}, TenantID: tenant, Status: "active"}, nil
}
func (r *fakeResolver) ResolveSource(_ context.Context, tenant string, reference SourceRef) (SourceReference, error) {
	r.calls.Add(1)
	if r.fail != nil {
		return SourceReference{}, r.fail
	}
	return SourceReference{SourceApplication: sourceApplication(reference.Kind), TenantID: tenant, ResourceID: *reference.ResourceID, Revision: *reference.SourceRevision, Status: "active", DisplaySummary: "来源"}, nil
}
func (r *fakeResolver) ResolveEvidence(_ context.Context, tenant string, reference EvidenceRef) (EvidenceReference, error) {
	r.calls.Add(1)
	if r.fail != nil {
		return EvidenceReference{}, r.fail
	}
	return EvidenceReference{TenantID: tenant, FileID: reference.FileID, Version: reference.Version, Revision: reference.Version, Status: "available"}, nil
}

type recoveringStore struct {
	fakeStore
	created     Registration
	createCalls atomic.Int64
}

func (s *recoveringStore) Replay(_ context.Context, _ Actor, _ string, _ string) (Registration, bool, error) {
	return s.created, s.created.ID != "", nil
}

func (s *recoveringStore) Create(_ context.Context, actor Actor, input CreateInput, resolved ResolvedCreate, _ string) (Registration, error) {
	s.createCalls.Add(1)
	s.created = Registration{
		ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", TenantID: actor.TenantID, Status: StatusRecorded,
		IssuedOn: input.IssuedOn, AmountMinor: input.AmountMinor, Currency: input.Currency,
		IssuerParty: resolved.IssuerParty, RecipientParty: resolved.RecipientParty, SourceRefs: resolved.SourceRefs, Revision: 1,
	}
	return s.created, nil
}

func TestCreateReplaySkipsExternalReferences(t *testing.T) {
	stored := Registration{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Revision: 1}
	store := &fakeStore{replayValue: stored, replayFound: true}
	resolver := &fakeResolver{fail: errors.New("dependency down")}
	service, err := NewService(store, resolver, "CNY")
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Create(context.Background(), testActor(), testCreateInput())
	if err != nil || result.ID != stored.ID {
		t.Fatalf("replay failed: %#v %v", result, err)
	}
	if resolver.calls.Load() != 0 {
		t.Fatalf("replay called dependencies %d times", resolver.calls.Load())
	}
}

func TestCreateFreezesResolvedReferences(t *testing.T) {
	store := &fakeStore{}
	resolver := &fakeResolver{}
	service, _ := NewService(store, resolver, "CNY")
	result, err := service.Create(context.Background(), testActor(), testCreateInput())
	if err != nil {
		t.Fatal(err)
	}
	if result.IssuerParty.Revision != 1 || result.RecipientParty.Revision != 1 || len(result.SourceRefs) != 1 || result.SourceRefs[0].Kind != SourceManualConfirmation {
		t.Fatalf("references were not frozen: %#v", result)
	}
	if resolver.calls.Load() != 2 {
		t.Fatalf("expected exactly two Party lookups, got %d", resolver.calls.Load())
	}
}

func TestCreateDependencyFailureCanRecoverAndReplayWithoutDuplicateWrite(t *testing.T) {
	store := &recoveringStore{}
	resolver := &fakeResolver{fail: ErrReferenceUnavailable}
	service, err := NewService(store, resolver, "CNY")
	if err != nil {
		t.Fatal(err)
	}

	input := testCreateInput()
	if _, err := service.Create(context.Background(), testActor(), input); !errors.Is(err, ErrReferenceUnavailable) {
		t.Fatalf("first error=%v", err)
	}
	if store.createCalls.Load() != 0 {
		t.Fatalf("failed dependency wrote %d registrations", store.createCalls.Load())
	}

	resolver.fail = nil
	created, err := service.Create(context.Background(), testActor(), input)
	if err != nil || created.Revision != 1 || store.createCalls.Load() != 1 {
		t.Fatalf("created=%#v error=%v create calls=%d", created, err, store.createCalls.Load())
	}
	resolvedCalls := resolver.calls.Load()

	resolver.fail = ErrReferenceUnavailable
	replayed, err := service.Create(context.Background(), testActor(), input)
	if err != nil || replayed.ID != created.ID || store.createCalls.Load() != 1 {
		t.Fatalf("replayed=%#v error=%v create calls=%d", replayed, err, store.createCalls.Load())
	}
	if resolver.calls.Load() != resolvedCalls {
		t.Fatalf("replay called external resolver: before=%d after=%d", resolvedCalls, resolver.calls.Load())
	}
}

func TestCreateRejectsCurrencyOutsideConfiguredSet(t *testing.T) {
	service, _ := NewService(&fakeStore{}, &fakeResolver{}, "CNY")
	input := testCreateInput()
	input.Currency = "USD"
	if _, err := service.Create(context.Background(), testActor(), input); err == nil {
		t.Fatal("unconfigured currency must fail closed")
	}
}

func TestListValidatesAndRoundTripsStableCursor(t *testing.T) {
	first := Registration{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", IssuedOn: "2026-09-27"}
	second := Registration{ID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", IssuedOn: "2026-09-26"}
	store := &fakeStore{listPage: ListPage{Items: []Registration{first, second}, EffectiveAmountMinor: 12345, Currency: "CNY"}}
	service, _ := NewService(store, &fakeResolver{}, "CNY")
	page, err := service.List(context.Background(), testActor().TenantID, ListQuery{ProjectID: "11111111-1111-4111-8111-111111111111", Limit: 1})
	if err != nil || len(page.Items) != 1 || page.NextCursor == "" || store.listFilter.Limit != 1 {
		t.Fatalf("page=%#v filter=%#v error=%v", page, store.listFilter, err)
	}
	store.listPage = ListPage{}
	_, err = service.List(context.Background(), testActor().TenantID, ListQuery{Cursor: page.NextCursor, Limit: 1})
	if err != nil || store.listFilter.BeforeDate != first.IssuedOn || store.listFilter.BeforeID != first.ID {
		t.Fatalf("cursor filter=%#v error=%v", store.listFilter, err)
	}
	if _, err = service.List(context.Background(), testActor().TenantID, ListQuery{Cursor: "not-a-cursor"}); err == nil {
		t.Fatal("invalid cursor was accepted")
	}
}

func testActor() Actor {
	return Actor{ID: "33333333-3333-4333-8333-333333333333", TenantID: "44444444-4444-4444-8444-444444444444", Kind: "iam_subject", CredentialID: "cred-1"}
}

func testCreateInput() CreateInput {
	manual := "人工确认应收"
	return CreateInput{CommandID: "55555555-5555-4555-8555-555555555555", IssuedOn: "2026-09-27", AmountMinor: 12345, Currency: "CNY", IssuerPartyRef: PartyRef{ID: "11111111-1111-4111-8111-111111111111", Revision: 1}, RecipientPartyRef: PartyRef{ID: "22222222-2222-4222-8222-222222222222", Revision: 1}, SourceRefs: []SourceRef{{Kind: SourceManualConfirmation, ManualSummary: &manual}}, Note: "首票"}
}
