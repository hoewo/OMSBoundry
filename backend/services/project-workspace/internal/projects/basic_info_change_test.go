package projects

import (
	"context"
	"errors"
	"testing"
)

func TestBasicInfoChangeExplicitOwnerOperationsAndReplay(t *testing.T) {
	person := &InitiationOwnerReference{PersonID: testSubject, SourceRevision: 3}
	base := BasicInfoChangeInput{CommandID: testCommand, ExpectedRevision: 3, Reason: "修改项目基本信息", Owners: []OwnerChange{{RoleKey: RoleProductionOwner, Operation: "set", Person: person}}}
	for _, owners := range [][]OwnerChange{
		{{RoleKey: RoleParticipant, Operation: "set", Person: person}},
		{{RoleKey: RoleBusinessOwner, Operation: "set"}},
		{{RoleKey: RoleBusinessOwner, Operation: "clear", Person: person}},
		{{RoleKey: RoleBusinessOwner, Operation: ""}},
		{{RoleKey: RoleBusinessOwner, Operation: "clear"}, {RoleKey: RoleBusinessOwner, Operation: "clear"}},
	} {
		in := base
		in.Owners = owners
		if _, err := NormalizeBasicInfoChange(in); err == nil {
			t.Fatalf("accepted %#v", owners)
		}
	}
	store := &storeStub{}
	refs := &referencesStub{peopleErr: ErrReferenceUnavailable}
	service, _ := NewService(store, refs)
	if _, err := service.ChangeBasicInfo(context.Background(), Actor{TenantID: testTenant}, testProject, base); !errors.Is(err, ErrReferenceUnavailable) || store.basicChangeCalls != 0 {
		t.Fatalf("unavailable=%v writes=%d", err, store.basicChangeCalls)
	}
	refs.peopleErr = nil
	refs.peopleResult = ReferenceSnapshot{DisplayName: "新负责人"}
	if _, err := service.ChangeBasicInfo(context.Background(), Actor{TenantID: testTenant}, testProject, base); err != nil || store.basicChangeCalls != 1 || store.basicChangeInput.Owners[0].snapshot != "新负责人" {
		t.Fatalf("validated=%v %#v", err, store.basicChangeInput)
	}
	store.basicChangeFound = true
	store.basicChangeReplay = BasicInfoChangeResult{Project: Project{ID: testProject, Revision: 4}}
	refs.peopleCalls = 0
	refs.peopleErr = ErrReferenceUnavailable
	result, err := service.ChangeBasicInfo(context.Background(), Actor{TenantID: testTenant}, testProject, base)
	if err != nil || result.Project.Revision != 4 || refs.peopleCalls != 0 || store.basicChangeCalls != 1 {
		t.Fatalf("offline replay=%#v err=%v calls=%d", result, err, refs.peopleCalls)
	}
}

func TestInitiationInfoChangeValidatesPeopleAndReplaysWithoutDirectory(t *testing.T) {
	in := BasicInfoChangeInput{CommandID: testCommand, ExpectedRevision: 6, Reason: "补齐立项", Owners: []OwnerChange{{RoleKey: RoleBusinessOwner, Operation: "set", Person: &InitiationOwnerReference{PersonID: testSubject, SourceRevision: 3}}}}
	store := &storeStub{}
	refs := &referencesStub{peopleErr: ErrReferenceUnavailable}
	service, _ := NewService(store, refs)
	if _, err := service.ChangeInitiationInfo(context.Background(), Actor{TenantID: testTenant}, testProject, in); !errors.Is(err, ErrReferenceUnavailable) || store.initiationChangeCalls != 0 {
		t.Fatalf("unavailable=%v writes=%d", err, store.initiationChangeCalls)
	}
	refs.peopleErr = nil
	refs.peopleResult = ReferenceSnapshot{DisplayName: "经营负责人"}
	if _, err := service.ChangeInitiationInfo(context.Background(), Actor{TenantID: testTenant}, testProject, in); err != nil || store.initiationChangeCalls != 1 || store.basicChangeCalls != 0 {
		t.Fatalf("route=%v writes=%d/%d", err, store.initiationChangeCalls, store.basicChangeCalls)
	}
	store.basicChangeFound = true
	store.basicChangeReplay = BasicInfoChangeResult{Project: Project{ID: testProject, Revision: 7}}
	refs.peopleErr = ErrReferenceUnavailable
	refs.peopleCalls = 0
	if result, err := service.ChangeInitiationInfo(context.Background(), Actor{TenantID: testTenant}, testProject, in); err != nil || result.Project.Revision != 7 || refs.peopleCalls != 0 {
		t.Fatalf("replay=%#v err=%v", result, err)
	}
	oldHash, _ := basicInfoChangeHash(testProject, in)
	newHash, _ := ownerInfoChangeHash(testProject, in, "project-initiation-info-change-v1")
	if oldHash == newHash {
		t.Fatal("command namespaces conflated")
	}
}
