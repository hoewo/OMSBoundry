package design

import (
	"context"
	"errors"
	"testing"
)

const (
	workItemTenant     = "11111111-1111-4111-8111-111111111111"
	workItemSubject    = "22222222-2222-4222-8222-222222222222"
	workItemPerson     = "77777777-7777-4777-8777-777777777777"
	workItemProject    = "33333333-3333-4333-8333-333333333333"
	workItemID         = "44444444-4444-4444-8444-444444444444"
	workItemAssignment = "55555555-5555-4555-8555-555555555555"
)

type workItemStore struct {
	items []WorkItem
}

func (*workItemStore) Ping(context.Context) error                              { return nil }
func (s *workItemStore) List(context.Context, string, int) ([]WorkItem, error) { return s.items, nil }
func (*workItemStore) Replay(context.Context, string, string, string, any) (bool, error) {
	return false, nil
}
func (*workItemStore) Create(context.Context, Actor, CreateInput, string) (Details, error) {
	return Details{}, nil
}
func (*workItemStore) SetOwner(context.Context, Actor, string, OwnerInput, string, string) (Details, error) {
	return Details{}, nil
}
func (*workItemStore) PutDeliverable(context.Context, Actor, string, string, DeliverableInput, string) (Deliverable, error) {
	return Deliverable{}, nil
}
func (*workItemStore) SetApplicability(context.Context, Actor, string, string, ApplicabilityInput, string) (Details, error) {
	return Details{}, nil
}
func (*workItemStore) CompleteStage(context.Context, Actor, string, string, RevisionInput, string) (Details, error) {
	return Details{}, nil
}
func (*workItemStore) ReopenStage(context.Context, Actor, string, string, ReasonInput, string) (Details, error) {
	return Details{}, nil
}
func (*workItemStore) Cancel(context.Context, Actor, string, ReasonInput, string) (Details, error) {
	return Details{}, nil
}
func (*workItemStore) Get(context.Context, string, string, bool) (Details, error) {
	return Details{}, nil
}
func (*workItemStore) History(context.Context, string, string, int64, int) (HistoryPage, error) {
	return HistoryPage{}, nil
}

type workItemReferences struct {
	assignment    AssignmentReference
	person        PersonIAMSubjectFact
	assignmentErr error
	personErr     error
}

func (*workItemReferences) ValidateProject(context.Context, string, string, int64) (ProjectReference, error) {
	return ProjectReference{}, nil
}
func (r *workItemReferences) ValidateAssignment(context.Context, string, string, string, int64) (AssignmentReference, error) {
	return r.assignment, r.assignmentErr
}
func (r *workItemReferences) ValidatePersonIAMSubject(context.Context, string, string, int64) (PersonIAMSubjectFact, error) {
	return r.person, r.personErr
}
func (*workItemReferences) ValidateEvidence(context.Context, string, string, int, int64) (EvidenceReference, error) {
	return EvidenceReference{}, nil
}

func TestWorkItemsForSubjectRequiresCurrentDirectPersonAssignment(t *testing.T) {
	revision := int64(3)
	assignmentID := workItemAssignment
	store := &workItemStore{items: []WorkItem{{ID: workItemID, TenantID: workItemTenant, ProjectID: workItemProject, TypeKey: DesignTypeKey, Status: StatusInProgress, Revision: 4, OwnerAssignmentID: &assignmentID, OwnerSourceRevision: &revision}}}
	refs := &workItemReferences{
		assignment: AssignmentReference{ID: assignmentID, TenantID: workItemTenant, ProjectID: workItemProject, SubjectKind: "person", SubjectID: workItemPerson, SourceRevision: 7, RoleKey: "production_owner", Status: "active", Revision: revision},
		person:     PersonIAMSubjectFact{ID: workItemPerson, TenantID: workItemTenant, Status: "active", Revision: 7, IAMSubjectID: workItemSubject},
	}
	service, err := NewService(store, refs)
	if err != nil {
		t.Fatal(err)
	}
	items, err := service.WorkItemsForSubject(context.Background(), workItemTenant, workItemSubject, 100)
	if err != nil || len(items) != 1 || items[0].WorkItem.ID != workItemID || items[0].Owner.SubjectID != workItemPerson {
		t.Fatalf("items=%#v err=%v", items, err)
	}

	refs.assignment.Status = "ended"
	items, err = service.WorkItemsForSubject(context.Background(), workItemTenant, workItemSubject, 100)
	if err != nil || len(items) != 0 {
		t.Fatalf("ended assignment items=%#v err=%v", items, err)
	}
	refs.assignment.Status = "active"
	refs.person.IAMSubjectID = "66666666-6666-4666-8666-666666666666"
	items, err = service.WorkItemsForSubject(context.Background(), workItemTenant, workItemSubject, 100)
	if err != nil || len(items) != 0 {
		t.Fatalf("adjacent subject items=%#v err=%v", items, err)
	}
	refs.person.IAMSubjectID = workItemSubject
	refs.assignmentErr = ErrReferenceInvalid
	items, err = service.WorkItemsForSubject(context.Background(), workItemTenant, workItemSubject, 100)
	if err != nil || len(items) != 0 {
		t.Fatalf("stale assignment items=%#v err=%v", items, err)
	}
	refs.assignmentErr = nil
	refs.personErr = ErrReferenceInvalid
	items, err = service.WorkItemsForSubject(context.Background(), workItemTenant, workItemSubject, 100)
	if err != nil || len(items) != 0 {
		t.Fatalf("stale person linkage items=%#v err=%v", items, err)
	}
	refs.personErr = ErrReferenceUnavailable
	if _, err := service.WorkItemsForSubject(context.Background(), workItemTenant, workItemSubject, 100); !errors.Is(err, ErrReferenceUnavailable) {
		t.Fatalf("reference outage error=%v", err)
	}
}
