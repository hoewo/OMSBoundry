package projects

import (
	"context"
	"testing"
)

func TestCommercialContributorIsAnExplicitPersonAssignment(t *testing.T) {
	for _, kind := range []string{"person", "group"} {
		t.Run(kind, func(t *testing.T) {
			store := &storeStub{}
			refs := &referencesStub{peopleResult: ReferenceSnapshot{Revision: 3, DisplayName: "经营参与人员"}}
			service, _ := NewService(store, refs)
			_, err := service.ApplyAssignment(context.Background(), Actor{TenantID: testTenant}, testProject, AssignmentCommand{CommandID: testCommand, ExpectedRevision: 1, Operation: "assign", SubjectKind: kind, SubjectID: testSubject, SourceRevision: 3, RoleKey: RoleCommercialContributor})
			if kind == "person" {
				if err != nil || store.assignmentCalls != 1 || refs.peopleCalls != 1 || store.assignmentInput.RoleKey != RoleCommercialContributor || store.assignmentInput.DisplaySnapshot != "经营参与人员" {
					t.Fatalf("err=%v calls=%d refs=%d input=%+v", err, store.assignmentCalls, refs.peopleCalls, store.assignmentInput)
				}
			} else if err == nil || store.assignmentCalls != 0 || refs.peopleCalls != 0 {
				t.Fatalf("group accepted: err=%v writes=%d refs=%d", err, store.assignmentCalls, refs.peopleCalls)
			}
		})
	}
	if !ValidRole(RoleCommercialContributor) || !ValidRole(RoleParticipant) || ValidRole("business_personnel") {
		t.Fatal("UI terminology must not replace stable role values")
	}
}
