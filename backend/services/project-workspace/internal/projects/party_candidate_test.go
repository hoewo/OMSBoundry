package projects

import (
	"context"
	"errors"
	"testing"
)

type partyCandidateStoreStub struct {
	*storeStub
	items       []PartyCandidate
	replay      PartyCandidate
	replayFound bool
	replayErr   error
	registered  PartyCandidate
	input       PartyCandidateInput
	calls       int
}

func (s *partyCandidateStoreStub) ListPartyCandidates(context.Context, string) ([]PartyCandidate, error) {
	return s.items, nil
}
func (s *partyCandidateStoreStub) ReplayPartyCandidate(context.Context, string, PartyCandidateInput) (PartyCandidate, bool, error) {
	return s.replay, s.replayFound, s.replayErr
}
func (s *partyCandidateStoreStub) RegisterPartyCandidate(_ context.Context, _ Actor, input PartyCandidateInput) (PartyCandidate, error) {
	s.calls++
	s.input = input
	return s.registered, nil
}

func TestRegisterPartyCandidateValidatesPartyBeforePersisting(t *testing.T) {
	store := &partyCandidateStoreStub{storeStub: &storeStub{}, registered: PartyCandidate{PartyID: testSubject, RoleKey: PartyCandidateRoleContractor, SourceRevision: 3}}
	references := &referencesStub{partyResult: PartySnapshot{PartyID: testSubject, SourceRevision: 3, Kind: "organization", LegalName: "施工方"}}
	service, err := NewService(store, references)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.RegisterPartyCandidate(context.Background(), Actor{TenantID: testTenant, ID: testActorID}, PartyCandidateInput{CommandID: testCommand, PartyID: testSubject, SourceRevision: 3, RoleKey: " CONTRACTOR "})
	if err != nil || result.PartyID != testSubject || references.partyCalls != 1 || store.calls != 1 || store.input.RoleKey != PartyCandidateRoleContractor {
		t.Fatalf("result=%#v error=%v party_calls=%d store=%#v", result, err, references.partyCalls, store)
	}
}

func TestRegisterPartyCandidateReplayDoesNotRequirePartyDependency(t *testing.T) {
	store := &partyCandidateStoreStub{storeStub: &storeStub{}, replayFound: true, replay: PartyCandidate{PartyID: testSubject, RoleKey: PartyCandidateRoleClient, SourceRevision: 1}}
	references := &referencesStub{partyErr: errors.New("party directory unavailable")}
	service, _ := NewService(store, references)
	result, err := service.RegisterPartyCandidate(context.Background(), Actor{TenantID: testTenant, ID: testActorID}, PartyCandidateInput{CommandID: testCommand, PartyID: testSubject, SourceRevision: 1, RoleKey: PartyCandidateRoleClient})
	if err != nil || result.RoleKey != PartyCandidateRoleClient || references.partyCalls != 0 || store.calls != 0 {
		t.Fatalf("result=%#v error=%v party_calls=%d store_calls=%d", result, err, references.partyCalls, store.calls)
	}
}

func TestRegisterPartyCandidateRejectsUnknownRole(t *testing.T) {
	store := &partyCandidateStoreStub{storeStub: &storeStub{}}
	service, _ := NewService(store, &referencesStub{})
	if _, err := service.RegisterPartyCandidate(context.Background(), Actor{TenantID: testTenant, ID: testActorID}, PartyCandidateInput{CommandID: testCommand, PartyID: testSubject, SourceRevision: 1, RoleKey: "supplier"}); err == nil || store.calls != 0 {
		t.Fatalf("invalid role error=%v calls=%d", err, store.calls)
	}
}
