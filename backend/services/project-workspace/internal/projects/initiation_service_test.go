package projects

import (
	"context"
	"errors"
	"testing"
)

func TestDispatchValidatesPeopleAndReplaysWithoutExternalDependency(t *testing.T) {
	input := InitiationDispatchInput{CommandID: testCommand, NodeType: NodeTypeProject, Name: " 待立项项目 ", BusinessOwner: InitiationOwnerReference{PersonID: testSubject, SourceRevision: 3}}
	store := &storeStub{}
	references := &referencesStub{peopleResult: ReferenceSnapshot{DisplayName: "经营负责人"}}
	service, _ := NewService(store, references)
	if _, err := service.DispatchInitiation(context.Background(), Actor{TenantID: testTenant}, input); err != nil || store.dispatchCalls != 1 || store.dispatchInput.ownerSnapshot != "经营负责人" || store.dispatchInput.Name != "待立项项目" {
		t.Fatalf("dispatch=%#v error=%v", store.dispatchInput, err)
	}
	references.peopleErr = ErrReferenceUnavailable
	if _, err := service.DispatchInitiation(context.Background(), Actor{TenantID: testTenant}, input); !errors.Is(err, ErrReferenceUnavailable) || store.dispatchCalls != 1 {
		t.Fatalf("dependency failure changed store: calls=%d error=%v", store.dispatchCalls, err)
	}
	store.dispatchReplay, store.dispatchFound = Project{ID: testProject, Revision: 1}, true
	before := references.peopleCalls
	if replay, err := service.DispatchInitiation(context.Background(), Actor{TenantID: testTenant}, input); err != nil || replay.ID != testProject || references.peopleCalls != before || store.dispatchCalls != 1 {
		t.Fatalf("replay=%#v calls=%d error=%v", replay, references.peopleCalls, err)
	}
}

func TestDispatchRejectsUnversionedOrInvalidOwnerBeforeAnyDependency(t *testing.T) {
	for _, input := range []InitiationDispatchInput{
		{CommandID: testCommand, NodeType: NodeTypeProject, Name: "项目"},
		{CommandID: testCommand, NodeType: NodeTypeProject, Name: "项目", BusinessOwner: InitiationOwnerReference{PersonID: testSubject}},
		{CommandID: testCommand, NodeType: "unknown", Name: "项目", BusinessOwner: InitiationOwnerReference{PersonID: testSubject, SourceRevision: 1}},
	} {
		store, references := &storeStub{}, &referencesStub{}
		service, _ := NewService(store, references)
		if _, err := service.DispatchInitiation(context.Background(), Actor{}, input); err == nil || store.dispatchCalls != 0 || references.peopleCalls != 0 {
			t.Fatalf("invalid input reached dependency/store: input=%#v error=%v", input, err)
		}
	}
}

func TestCompleteInitiationValidatesProfileBeforeStore(t *testing.T) {
	store := &storeStub{}
	service, err := NewService(store, &referencesStub{})
	if err != nil {
		t.Fatal(err)
	}
	actor := Actor{ID: testActorID, TenantID: testTenant}
	for _, profile := range []string{"", InitiationProfileDualOwnerV1, " Business-Owner-V1 "} {
		_, err := service.CompleteInitiation(context.Background(), actor, testProject, RevisionCommand{CommandID: testCommand, ExpectedRevision: 1, InitiationProfile: profile})
		if err != nil {
			t.Fatal(err)
		}
	}
	if store.initiationCalls != 3 || store.initiationInput.InitiationProfile != InitiationProfileBusinessOwnerV1 {
		t.Fatalf("store=%#v", store)
	}
	if _, err := service.CompleteInitiation(context.Background(), actor, testProject, RevisionCommand{CommandID: testCommand, ExpectedRevision: 1, InitiationProfile: "skip-owners"}); err == nil || store.initiationCalls != 3 {
		t.Fatalf("unknown profile reached store: calls=%d error=%v", store.initiationCalls, err)
	}
}

func TestCancelDoesNotSilentlyAcceptInitiationProfile(t *testing.T) {
	store := &storeStub{}
	service, err := NewService(store, &referencesStub{})
	if err != nil {
		t.Fatal(err)
	}
	input := CancelCommand{RevisionCommand: RevisionCommand{CommandID: testCommand, ExpectedRevision: 1, InitiationProfile: InitiationProfileBusinessOwnerV1}, Reason: "取消"}
	if _, err := service.Cancel(context.Background(), Actor{}, testProject, input); err == nil || store.cancelCalls != 0 {
		t.Fatalf("unexpected cancellation: calls=%d error=%v", store.cancelCalls, err)
	}
}
