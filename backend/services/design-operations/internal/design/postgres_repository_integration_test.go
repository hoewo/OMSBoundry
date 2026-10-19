//go:build integration

package design

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"omsbizfoundry/design-operations/internal/database"
	"omsbizfoundry/design-operations/internal/testdatabase"
)

const (
	intTenant     = "11111111-1111-4111-8111-111111111111"
	intActor      = "22222222-2222-4222-8222-222222222222"
	intProject    = "33333333-3333-4333-8333-333333333333"
	intAssignment = "44444444-4444-4444-8444-444444444444"
)

type stableReferences struct {
	unavailable       bool
	projectStatus     string
	projectTypeKeys   []string
	projectRevision   int64
	assignmentStatus  string
	assignmentRole    string
	assignmentProject string
	evidenceStatus    string
}

func (r *stableReferences) ValidateProject(_ context.Context, tenant, id string, revision int64) (ProjectReference, error) {
	if r.unavailable {
		return ProjectReference{}, ErrReferenceUnavailable
	}
	if r.projectRevision > 0 && revision != r.projectRevision {
		return ProjectReference{}, ErrReferenceInvalid
	}
	status := r.projectStatus
	if status == "" {
		status = "active"
	}
	types := r.projectTypeKeys
	if types == nil {
		types = []string{"design"}
	}
	return ProjectReference{ID: id, TenantID: tenant, Status: status, Revision: revision, TypeKeys: types}, nil
}
func (r *stableReferences) ValidateAssignment(_ context.Context, tenant, project, id string, revision int64) (AssignmentReference, error) {
	if r.unavailable {
		return AssignmentReference{}, ErrReferenceUnavailable
	}
	status := r.assignmentStatus
	if status == "" {
		status = "active"
	}
	role := r.assignmentRole
	if role == "" {
		role = "production_owner"
	}
	assignmentProject := r.assignmentProject
	if assignmentProject == "" {
		assignmentProject = project
	}
	return AssignmentReference{ID: id, TenantID: tenant, ProjectID: assignmentProject, RoleKey: role, Status: status, DisplayName: "设计组", Revision: revision}, nil
}
func (r *stableReferences) ValidatePersonIAMSubject(_ context.Context, tenant, id string, revision int64) (PersonIAMSubjectFact, error) {
	return PersonIAMSubjectFact{SchemaVersion: "1", SourceApplication: "people-groups", FactKind: "person-iam-subject-link", ID: id, TenantID: tenant, Status: "active", Revision: revision, IAMSubjectID: intActor}, nil
}
func (r *stableReferences) ValidateEvidence(_ context.Context, tenant, id string, version int, revision int64) (EvidenceReference, error) {
	if r.unavailable {
		return EvidenceReference{}, ErrReferenceUnavailable
	}
	status := r.evidenceStatus
	if status == "" {
		status = "available"
	}
	return EvidenceReference{FileID: id, TenantID: tenant, Status: status, DisplayName: id + ".pdf", Version: version, Revision: revision}, nil
}

func TestDesignLifecycleRollbackHistoryAndIdempotency(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(environment.Pool)
	refs := &stableReferences{}
	service, _ := NewService(store, refs)
	actor := Actor{Kind: "test_subject", ID: intActor, TenantID: intTenant, DisplayName: "验收管理员"}

	created, err := service.Create(ctx, actor, CreateInput{CommandID: command(1), ProjectID: intProject, ProjectSourceRevision: 4})
	if err != nil || created.WorkItem.Revision != 1 || created.WorkItem.Status != StatusDraft || len(created.Stages) != 4 || created.Stages[0].Status != StageReady {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	replay, err := service.Create(ctx, actor, CreateInput{CommandID: command(1), ProjectID: intProject, ProjectSourceRevision: 4})
	if err != nil || replay.WorkItem.ID != created.WorkItem.ID {
		t.Fatalf("replay=%#v err=%v", replay, err)
	}
	if _, err := service.Create(ctx, actor, CreateInput{CommandID: command(1), ProjectID: intProject, ProjectSourceRevision: 5}); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("command conflict=%v", err)
	}

	itemID := created.WorkItem.ID
	owned, err := service.SetOwner(ctx, actor, itemID, OwnerInput{CommandID: command(2), ExpectedRevision: 1, AssignmentID: intAssignment, AssignmentSourceRevision: 3})
	if err != nil || owned.WorkItem.Revision != 2 || owned.WorkItem.Status != StatusInProgress {
		t.Fatalf("owner=%#v err=%v", owned, err)
	}
	revision := owned.WorkItem.Revision
	for index, stage := range []string{"scheme", "construction-drawing", "construction-support"} {
		if stage == "construction-drawing" {
			skipped, skipErr := service.SetApplicability(ctx, actor, itemID, "preliminary-design", ApplicabilityInput{CommandID: command(3), ExpectedRevision: revision, Operation: "set-not-applicable", Reason: "本项目无需初设"})
			if skipErr != nil || skipped.WorkItem.Revision != revision+1 {
				t.Fatalf("skip=%#v err=%v", skipped, skipErr)
			}
			revision = skipped.WorkItem.Revision
		}
		fileID := fmt.Sprintf("55555555-5555-4555-8555-%012d", index+1)
		deliverable, deliverableErr := service.PutDeliverable(ctx, actor, itemID, stage, DeliverableInput{CommandID: command(10 + index*2), ExpectedRevision: revision, RequirementKey: RequirementPrimary, EvidenceFileID: fileID, EvidenceVersion: 1, EvidenceSourceRevision: 1})
		if deliverableErr != nil || deliverable.CreatedRevision != revision+1 {
			t.Fatalf("deliverable=%#v err=%v", deliverable, deliverableErr)
		}
		revision++
		completed, completeErr := service.CompleteStage(ctx, actor, itemID, stage, RevisionInput{CommandID: command(11 + index*2), ExpectedRevision: revision})
		if completeErr != nil || completed.WorkItem.Revision != revision+1 {
			t.Fatalf("complete %s=%#v err=%v", stage, completed, completeErr)
		}
		revision = completed.WorkItem.Revision
	}
	completed, err := service.GetOwned(ctx, intTenant, itemID)
	if err != nil || completed.WorkItem.Status != StatusCompleted || revision != 9 {
		t.Fatalf("completed=%#v revision=%d err=%v", completed, revision, err)
	}
	reference, err := service.Reference(ctx, intTenant, itemID)
	if err != nil || reference.ID != itemID || reference.WorkItemID != itemID || reference.Status != StatusCompleted || len(reference.CompletionGaps) != 0 || reference.ResourceType != "design-work-item" {
		t.Fatalf("reference=%#v err=%v", reference, err)
	}

	reopened, err := service.ReopenStage(ctx, actor, itemID, "construction-drawing", ReasonInput{RevisionInput: RevisionInput{CommandID: command(20), ExpectedRevision: revision}, Reason: "图纸修订"})
	if err != nil || reopened.WorkItem.Status != StatusInProgress || *reopened.WorkItem.CurrentStageKey != "construction-drawing" {
		t.Fatalf("reopened=%#v err=%v", reopened, err)
	}
	revision = reopened.WorkItem.Revision
	if reopened.Stages[2].Status != StageReady || reopened.Stages[3].Status != StagePending || len(reopened.Deliverables) != 3 {
		t.Fatalf("rollback projection=%#v", reopened)
	}
	for _, stage := range []string{"construction-drawing", "construction-support"} {
		completed, err = service.CompleteStage(ctx, actor, itemID, stage, RevisionInput{CommandID: command(int(revision) + 30), ExpectedRevision: revision})
		if err != nil {
			t.Fatalf("recomplete %s: %v", stage, err)
		}
		revision = completed.WorkItem.Revision
	}
	restored, err := service.SetApplicability(ctx, actor, itemID, "preliminary-design", ApplicabilityInput{CommandID: command(50), ExpectedRevision: revision, Operation: "restore-applicability", Reason: "客户要求补做初设"})
	if err != nil || restored.Stages[1].Status != StageReady || restored.Stages[2].Status != StagePending || restored.Stages[3].Status != StagePending || len(restored.Deliverables) != 3 {
		t.Fatalf("restored=%#v err=%v", restored, err)
	}

	refs.unavailable = true
	readable, err := service.Get(ctx, intTenant, itemID)
	if err != nil || readable.WorkItem.ID != itemID || len(readable.ReferenceStatuses) != 4 {
		t.Fatalf("read while references down=%#v err=%v", readable, err)
	}
	for _, status := range readable.ReferenceStatuses {
		if status.Status != "unavailable" {
			t.Fatalf("reference status=%#v", readable.ReferenceStatuses)
		}
	}
	restoredReplay, err := service.SetApplicability(ctx, actor, itemID, "preliminary-design", ApplicabilityInput{CommandID: command(50), ExpectedRevision: revision, Operation: "restore-applicability", Reason: "客户要求补做初设"})
	if err != nil || restoredReplay.WorkItem.Revision != restored.WorkItem.Revision {
		t.Fatalf("replay while refs down=%#v err=%v", restoredReplay, err)
	}
	page, err := service.History(ctx, intTenant, itemID, 0, 200)
	if err != nil || len(page.Items) != int(restored.WorkItem.Revision) {
		t.Fatalf("history=%d revision=%d err=%v", len(page.Items), restored.WorkItem.Revision, err)
	}
	if _, err := environment.Pool.Exec(ctx, `UPDATE design_history_events SET event_type='work_item_created' WHERE id=$1`, page.Items[0].ID); err == nil {
		t.Fatal("history update was not rejected")
	}
}

func TestOptimisticConcurrencyAllowsOneWriter(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(environment.Pool)
	service, _ := NewService(store, &stableReferences{})
	actor := Actor{ID: intActor, TenantID: intTenant}
	created, err := service.Create(ctx, actor, CreateInput{CommandID: command(60), ProjectID: intProject, ProjectSourceRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	inputs := []OwnerInput{
		{CommandID: command(61), ExpectedRevision: 1, AssignmentID: intAssignment, AssignmentSourceRevision: 1},
		{CommandID: command(62), ExpectedRevision: 1, AssignmentID: "66666666-6666-4666-8666-666666666666", AssignmentSourceRevision: 1},
	}
	var wait sync.WaitGroup
	errorsFound := make([]error, 2)
	for index := range inputs {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			_, errorsFound[index] = service.SetOwner(ctx, actor, created.WorkItem.ID, inputs[index])
		}(index)
	}
	wait.Wait()
	successes, conflicts := 0, 0
	for _, err := range errorsFound {
		if err == nil {
			successes++
		} else if errors.Is(err, ErrRevisionConflict) {
			conflicts++
		} else {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d errors=%v", successes, conflicts, errorsFound)
	}
}

func TestProductionTypeCreationKeepsDesignCompatibilityAndRejectsUngovernedTypes(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(environment.Pool)
	refs := &stableReferences{projectTypeKeys: []string{"design", "consultation"}}
	service, _ := NewService(store, refs)
	actor := Actor{Kind: "test_subject", ID: intActor, TenantID: intTenant}

	designItem, err := service.Create(ctx, actor, CreateInput{CommandID: command(90), ProjectID: intProject, ProjectSourceRevision: 1})
	if err != nil || designItem.WorkItem.TypeKey != DesignTypeKey || designItem.WorkItem.TypeDefinitionVersion != 1 || len(designItem.Stages) != 4 {
		t.Fatalf("design item=%#v err=%v", designItem, err)
	}
	consultationItem, err := service.Create(ctx, actor, CreateInput{CommandID: command(91), ProjectID: intProject, ProjectSourceRevision: 1, TypeKey: ConsultationTypeKey})
	if err != nil || consultationItem.WorkItem.TypeKey != ConsultationTypeKey || consultationItem.WorkItem.TypeDefinitionVersion != 1 || len(consultationItem.Stages) != 2 {
		t.Fatalf("consultation item=%#v err=%v", consultationItem, err)
	}
	if consultationItem.Stages[0].Key != "scheme" || consultationItem.Stages[0].Status != StageReady || consultationItem.Stages[1].Key != "report" || consultationItem.Stages[1].Status != StagePending {
		t.Fatalf("consultation stages=%#v", consultationItem.Stages)
	}
	if _, err := service.Create(ctx, actor, CreateInput{CommandID: command(92), ProjectID: intProject, ProjectSourceRevision: 1, TypeKey: ConsultationTypeKey}); !errors.Is(err, ErrActiveConflict) {
		t.Fatalf("duplicate consultation=%v", err)
	}

	otherProject := "77777777-7777-4777-8777-777777777777"
	if _, err := service.Create(ctx, actor, CreateInput{CommandID: command(93), ProjectID: otherProject, ProjectSourceRevision: 1, TypeKey: "customer-defined-v1"}); !errors.Is(err, ErrTypeInvalid) {
		t.Fatalf("unknown type=%v", err)
	}
	refs.projectTypeKeys = []string{"design"}
	if _, err := service.Create(ctx, actor, CreateInput{CommandID: command(94), ProjectID: otherProject, ProjectSourceRevision: 1, TypeKey: ConsultationTypeKey}); !errors.Is(err, ErrReferenceInvalid) {
		t.Fatalf("unselected consultation=%v", err)
	}
	refs.projectTypeKeys = []string{"design", "consultation"}
	refs.projectRevision = 1
	if _, err := service.Create(ctx, actor, CreateInput{CommandID: command(95), ProjectID: otherProject, ProjectSourceRevision: 2, TypeKey: ConsultationTypeKey}); !errors.Is(err, ErrReferenceInvalid) {
		t.Fatalf("stale project revision=%v", err)
	}

	var itemCount, commandCount int
	if err := environment.Pool.QueryRow(ctx, `SELECT count(*) FROM design_work_items`).Scan(&itemCount); err != nil {
		t.Fatal(err)
	}
	if err := environment.Pool.QueryRow(ctx, `SELECT count(*) FROM design_commands`).Scan(&commandCount); err != nil {
		t.Fatal(err)
	}
	if itemCount != 2 || commandCount != 2 {
		t.Fatalf("rejected type attempts wrote state: items=%d commands=%d", itemCount, commandCount)
	}
}

func TestDesignAndConsultationLifecyclesRemainIsolated(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(environment.Pool)
	refs := &stableReferences{projectTypeKeys: []string{"design", "consultation"}}
	service, _ := NewService(store, refs)
	actor := Actor{Kind: "test_subject", ID: intActor, TenantID: intTenant, DisplayName: "生产管理员"}

	designItem, err := service.Create(ctx, actor, CreateInput{CommandID: command(100), ProjectID: intProject, ProjectSourceRevision: 9, TypeKey: DesignTypeKey})
	if err != nil {
		t.Fatal(err)
	}
	consultationItem, err := service.Create(ctx, actor, CreateInput{CommandID: command(101), ProjectID: intProject, ProjectSourceRevision: 9, TypeKey: ConsultationTypeKey})
	if err != nil {
		t.Fatal(err)
	}
	designOwned, err := service.SetOwner(ctx, actor, designItem.WorkItem.ID, OwnerInput{CommandID: command(102), ExpectedRevision: 1, AssignmentID: intAssignment, AssignmentSourceRevision: 3})
	if err != nil || designOwned.WorkItem.Revision != 2 {
		t.Fatalf("design owner=%#v err=%v", designOwned, err)
	}

	refs.assignmentProject = "77777777-7777-4777-8777-777777777777"
	ownerInput := OwnerInput{CommandID: command(103), ExpectedRevision: 1, AssignmentID: intAssignment, AssignmentSourceRevision: 3}
	if _, err := service.SetOwner(ctx, actor, consultationItem.WorkItem.ID, ownerInput); !errors.Is(err, ErrReferenceInvalid) {
		t.Fatalf("consultation accepted owner from another project: %v", err)
	}
	refs.assignmentProject = ""
	refs.assignmentRole = "participant"
	if _, err := service.SetOwner(ctx, actor, consultationItem.WorkItem.ID, ownerInput); !errors.Is(err, ErrReferenceInvalid) {
		t.Fatalf("consultation accepted wrong owner role: %v", err)
	}
	refs.assignmentRole = "production_owner"
	consultationOwned, err := service.SetOwner(ctx, actor, consultationItem.WorkItem.ID, ownerInput)
	if err != nil || consultationOwned.WorkItem.Revision != 2 {
		t.Fatalf("consultation owner=%#v err=%v", consultationOwned, err)
	}

	refs.unavailable = true
	wrongTypeFile := "77777777-7777-4777-8777-777777777777"
	if _, err := service.PutDeliverable(ctx, actor, designItem.WorkItem.ID, "report", DeliverableInput{CommandID: command(104), ExpectedRevision: 2, RequirementKey: RequirementPrimary, EvidenceFileID: wrongTypeFile, EvidenceVersion: 1, EvidenceSourceRevision: 1}); !errors.Is(err, ErrStageInvalid) {
		t.Fatalf("design accepted consultation stage or consulted external reference first: %v", err)
	}
	refs.unavailable = false
	if _, err := service.SetApplicability(ctx, actor, consultationItem.WorkItem.ID, "report", ApplicabilityInput{CommandID: command(105), ExpectedRevision: 2, Operation: "set-not-applicable", Reason: "非法跳过必要报告"}); !errors.Is(err, ErrStageInvalid) {
		t.Fatalf("consultation skipped non-skippable report: %v", err)
	}
	if _, err := service.CompleteStage(ctx, actor, consultationItem.WorkItem.ID, "report", RevisionInput{CommandID: command(116), ExpectedRevision: 2}); !errors.Is(err, ErrStageInvalid) {
		t.Fatalf("consultation completed report before scheme decision: %v", err)
	}
	skipped, err := service.SetApplicability(ctx, actor, consultationItem.WorkItem.ID, "scheme", ApplicabilityInput{CommandID: command(106), ExpectedRevision: 2, Operation: "set-not-applicable", Reason: "直接编制咨询报告"})
	if err != nil || skipped.WorkItem.Revision != 3 || skipped.Stages[0].Status != StageNotApplicable || skipped.Stages[1].Status != StageReady {
		t.Fatalf("consultation skip=%#v err=%v", skipped, err)
	}

	reportFile1 := "88888888-8888-4888-8888-888888888888"
	reportInput := DeliverableInput{CommandID: command(107), ExpectedRevision: 3, RequirementKey: RequirementPrimary, EvidenceFileID: reportFile1, EvidenceVersion: 1, EvidenceSourceRevision: 1}
	refs.unavailable = true
	if _, err := service.PutDeliverable(ctx, actor, consultationItem.WorkItem.ID, "report", reportInput); !errors.Is(err, ErrReferenceUnavailable) {
		t.Fatalf("consultation accepted unavailable evidence: %v", err)
	}
	refs.unavailable = false
	firstReport, err := service.PutDeliverable(ctx, actor, consultationItem.WorkItem.ID, "report", reportInput)
	if err != nil || firstReport.CreatedRevision != 4 {
		t.Fatalf("first report=%#v err=%v", firstReport, err)
	}
	reportFile2 := "99999999-9999-4999-8999-999999999999"
	secondReport, err := service.PutDeliverable(ctx, actor, consultationItem.WorkItem.ID, "report", DeliverableInput{CommandID: command(108), ExpectedRevision: 4, RequirementKey: RequirementPrimary, EvidenceFileID: reportFile2, EvidenceVersion: 2, EvidenceSourceRevision: 2})
	if err != nil || secondReport.CreatedRevision != 5 {
		t.Fatalf("replacement report=%#v err=%v", secondReport, err)
	}
	refs.assignmentStatus = "ended"
	if _, err := service.CompleteStage(ctx, actor, consultationItem.WorkItem.ID, "report", RevisionInput{CommandID: command(109), ExpectedRevision: 5}); !errors.Is(err, ErrReferenceInvalid) {
		t.Fatalf("consultation completed with ended owner: %v", err)
	}
	refs.assignmentStatus = "active"
	completed, err := service.CompleteStage(ctx, actor, consultationItem.WorkItem.ID, "report", RevisionInput{CommandID: command(109), ExpectedRevision: 5})
	if err != nil || completed.WorkItem.Status != StatusCompleted || completed.WorkItem.Revision != 6 {
		t.Fatalf("consultation complete=%#v err=%v", completed, err)
	}

	designAfterConsultation, err := service.GetOwned(ctx, intTenant, designItem.WorkItem.ID)
	if err != nil || designAfterConsultation.WorkItem.Revision != 2 || designAfterConsultation.WorkItem.Status != StatusInProgress || *designAfterConsultation.WorkItem.CurrentStageKey != "scheme" || len(designAfterConsultation.Deliverables) != 0 {
		t.Fatalf("consultation mutated design=%#v err=%v", designAfterConsultation, err)
	}
	allConsultation, err := store.Get(ctx, intTenant, consultationItem.WorkItem.ID, true)
	if err != nil || len(allConsultation.Deliverables) != 2 || allConsultation.Deliverables[0].SupersededAt == nil || allConsultation.Deliverables[1].SupersededAt != nil {
		t.Fatalf("consultation replacement history=%#v err=%v", allConsultation.Deliverables, err)
	}

	reopened, err := service.ReopenStage(ctx, actor, consultationItem.WorkItem.ID, "report", ReasonInput{RevisionInput: RevisionInput{CommandID: command(110), ExpectedRevision: 6}, Reason: "客户意见要求修改报告"})
	if err != nil || reopened.WorkItem.Revision != 7 || reopened.WorkItem.Status != StatusInProgress || reopened.Stages[1].Status != StageReady {
		t.Fatalf("consultation reopen=%#v err=%v", reopened, err)
	}
	recompleted, err := service.CompleteStage(ctx, actor, consultationItem.WorkItem.ID, "report", RevisionInput{CommandID: command(111), ExpectedRevision: 7})
	if err != nil || recompleted.WorkItem.Revision != 8 || recompleted.WorkItem.Status != StatusCompleted {
		t.Fatalf("consultation recomplete=%#v err=%v", recompleted, err)
	}
	restored, err := service.SetApplicability(ctx, actor, consultationItem.WorkItem.ID, "scheme", ApplicabilityInput{CommandID: command(112), ExpectedRevision: 8, Operation: "restore-applicability", Reason: "客户补充要求方案阶段"})
	if err != nil || restored.WorkItem.Revision != 9 || restored.WorkItem.Status != StatusInProgress || restored.Stages[0].Status != StageReady || restored.Stages[1].Status != StagePending {
		t.Fatalf("consultation restore=%#v err=%v", restored, err)
	}
	schemeFile := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	if _, err := service.PutDeliverable(ctx, actor, consultationItem.WorkItem.ID, "scheme", DeliverableInput{CommandID: command(113), ExpectedRevision: 9, RequirementKey: RequirementPrimary, EvidenceFileID: schemeFile, EvidenceVersion: 1, EvidenceSourceRevision: 1}); err != nil {
		t.Fatal(err)
	}
	afterScheme, err := service.CompleteStage(ctx, actor, consultationItem.WorkItem.ID, "scheme", RevisionInput{CommandID: command(114), ExpectedRevision: 10})
	if err != nil || afterScheme.WorkItem.Revision != 11 || afterScheme.Stages[1].Status != StageReady {
		t.Fatalf("scheme completion=%#v err=%v", afterScheme, err)
	}
	finalConsultation, err := service.CompleteStage(ctx, actor, consultationItem.WorkItem.ID, "report", RevisionInput{CommandID: command(115), ExpectedRevision: 11})
	if err != nil || finalConsultation.WorkItem.Revision != 12 || finalConsultation.WorkItem.Status != StatusCompleted {
		t.Fatalf("final consultation=%#v err=%v", finalConsultation, err)
	}
	reference, err := service.Reference(ctx, intTenant, consultationItem.WorkItem.ID)
	if err != nil || reference.TypeKey != "consultation" || reference.ResourceType != "production-work-item" || len(reference.CompletionGaps) != 0 {
		t.Fatalf("consultation reference=%#v err=%v", reference, err)
	}
	productionReference, err := service.ProductionReference(ctx, intTenant, consultationItem.WorkItem.ID)
	if err != nil || productionReference.TypeKey != ConsultationTypeKey || productionReference.ResourceType != "production-work-item" || len(productionReference.CompletionGaps) != 0 {
		t.Fatalf("canonical consultation reference=%#v err=%v", productionReference, err)
	}
	designHistory, err := service.History(ctx, intTenant, designItem.WorkItem.ID, 0, 200)
	if err != nil || len(designHistory.Items) != 2 {
		t.Fatalf("design history=%#v err=%v", designHistory, err)
	}
	consultationHistory, err := service.History(ctx, intTenant, consultationItem.WorkItem.ID, 0, 200)
	if err != nil || len(consultationHistory.Items) != int(finalConsultation.WorkItem.Revision) {
		t.Fatalf("consultation history=%d revision=%d err=%v", len(consultationHistory.Items), finalConsultation.WorkItem.Revision, err)
	}
}

func TestReferenceFailuresCompletionGuardsReplacementAndCancellation(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(environment.Pool)
	refs := &stableReferences{projectStatus: "cancelled"}
	service, _ := NewService(store, refs)
	actor := Actor{ID: intActor, TenantID: intTenant}
	input := CreateInput{CommandID: command(70), ProjectID: intProject, ProjectSourceRevision: 1}
	if _, err := service.Create(ctx, actor, input); !errors.Is(err, ErrReferenceInvalid) {
		t.Fatalf("cancelled project=%v", err)
	}
	refs.projectStatus = "active"
	refs.projectTypeKeys = []string{"survey"}
	input.CommandID = command(71)
	if _, err := service.Create(ctx, actor, input); !errors.Is(err, ErrReferenceInvalid) {
		t.Fatalf("wrong project type=%v", err)
	}
	refs.projectTypeKeys = nil
	refs.unavailable = true
	input.CommandID = command(72)
	if _, err := service.Create(ctx, actor, input); !errors.Is(err, ErrReferenceUnavailable) {
		t.Fatalf("project unavailable=%v", err)
	}
	var count int
	if err := environment.Pool.QueryRow(ctx, `SELECT count(*) FROM design_work_items`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial create count=%d err=%v", count, err)
	}

	refs.unavailable = false
	input.CommandID = command(73)
	created, err := service.Create(ctx, actor, input)
	if err != nil {
		t.Fatal(err)
	}
	input.CommandID = command(74)
	if _, err := service.Create(ctx, actor, input); !errors.Is(err, ErrActiveConflict) {
		t.Fatalf("active duplicate=%v", err)
	}
	itemID := created.WorkItem.ID
	if _, err := service.CompleteStage(ctx, actor, itemID, "scheme", RevisionInput{CommandID: command(75), ExpectedRevision: 1}); !errors.Is(err, ErrCompletionGaps) {
		t.Fatalf("missing owner=%v", err)
	}
	refs.assignmentRole = "participant"
	if _, err := service.SetOwner(ctx, actor, itemID, OwnerInput{CommandID: command(76), ExpectedRevision: 1, AssignmentID: intAssignment, AssignmentSourceRevision: 1}); !errors.Is(err, ErrReferenceInvalid) {
		t.Fatalf("wrong owner role=%v", err)
	}
	refs.assignmentRole = "production_owner"
	owned, err := service.SetOwner(ctx, actor, itemID, OwnerInput{CommandID: command(76), ExpectedRevision: 1, AssignmentID: intAssignment, AssignmentSourceRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CompleteStage(ctx, actor, itemID, "preliminary-design", RevisionInput{CommandID: command(77), ExpectedRevision: owned.WorkItem.Revision}); !errors.Is(err, ErrStageInvalid) {
		t.Fatalf("pending stage=%v", err)
	}
	refs.assignmentStatus = "ended"
	if _, err := service.CompleteStage(ctx, actor, itemID, "scheme", RevisionInput{CommandID: command(78), ExpectedRevision: owned.WorkItem.Revision}); !errors.Is(err, ErrReferenceInvalid) {
		t.Fatalf("ended owner=%v", err)
	}
	refs.assignmentStatus = "active"
	if _, err := service.CompleteStage(ctx, actor, itemID, "scheme", RevisionInput{CommandID: command(79), ExpectedRevision: owned.WorkItem.Revision}); !errors.Is(err, ErrCompletionGaps) {
		t.Fatalf("missing deliverable=%v", err)
	}

	refs.unavailable = true
	firstFile := "55555555-5555-4555-8555-555555555555"
	if _, err := service.PutDeliverable(ctx, actor, itemID, "scheme", DeliverableInput{CommandID: command(80), ExpectedRevision: 2, RequirementKey: RequirementPrimary, EvidenceFileID: firstFile, EvidenceVersion: 1, EvidenceSourceRevision: 1}); !errors.Is(err, ErrReferenceUnavailable) {
		t.Fatalf("evidence unavailable=%v", err)
	}
	refs.unavailable = false
	first, err := service.PutDeliverable(ctx, actor, itemID, "scheme", DeliverableInput{CommandID: command(80), ExpectedRevision: 2, RequirementKey: RequirementPrimary, EvidenceFileID: firstFile, EvidenceVersion: 1, EvidenceSourceRevision: 1})
	if err != nil || first.CreatedRevision != 3 {
		t.Fatalf("first=%#v err=%v", first, err)
	}
	secondFile := "66666666-6666-4666-8666-666666666666"
	second, err := service.PutDeliverable(ctx, actor, itemID, "scheme", DeliverableInput{CommandID: command(81), ExpectedRevision: 3, RequirementKey: RequirementPrimary, EvidenceFileID: secondFile, EvidenceVersion: 1, EvidenceSourceRevision: 1})
	if err != nil || second.CreatedRevision != 4 {
		t.Fatalf("second=%#v err=%v", second, err)
	}
	all, err := store.Get(ctx, intTenant, itemID, true)
	if err != nil || len(all.Deliverables) != 2 || all.Deliverables[0].SupersededAt == nil || all.Deliverables[1].SupersededAt != nil {
		t.Fatalf("deliverables=%#v err=%v", all.Deliverables, err)
	}
	refs.evidenceStatus = "deleted"
	if _, err := service.CompleteStage(ctx, actor, itemID, "scheme", RevisionInput{CommandID: command(82), ExpectedRevision: 4}); !errors.Is(err, ErrReferenceInvalid) {
		t.Fatalf("invalid evidence=%v", err)
	}
	refs.evidenceStatus = "available"
	completed, err := service.CompleteStage(ctx, actor, itemID, "scheme", RevisionInput{CommandID: command(82), ExpectedRevision: 4})
	if err != nil || completed.WorkItem.Revision != 5 {
		t.Fatalf("completed=%#v err=%v", completed, err)
	}
	cancelled, err := service.Cancel(ctx, actor, itemID, ReasonInput{RevisionInput: RevisionInput{CommandID: command(83), ExpectedRevision: 5}, Reason: "停止演示"})
	if err != nil || cancelled.WorkItem.Status != StatusCancelled {
		t.Fatalf("cancelled=%#v err=%v", cancelled, err)
	}
	if _, err := service.SetOwner(ctx, actor, itemID, OwnerInput{CommandID: command(84), ExpectedRevision: 6, AssignmentID: intAssignment, AssignmentSourceRevision: 1}); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("cancelled write=%v", err)
	}

	input.CommandID = command(85)
	replacement, err := service.Create(ctx, actor, input)
	if err != nil {
		t.Fatalf("replacement create=%v", err)
	}
	skipped, err := service.SetApplicability(ctx, actor, replacement.WorkItem.ID, "scheme", ApplicabilityInput{CommandID: command(86), ExpectedRevision: 1, Operation: "set-not-applicable", Reason: "无方案阶段"})
	if err != nil || skipped.Stages[0].Status != StageNotApplicable || skipped.Stages[1].Status != StageReady {
		t.Fatalf("scheme skip=%#v err=%v", skipped, err)
	}
}

func command(value int) string {
	return "aaaaaaaa-aaaa-4aaa-8aaa-" + leftPad(value)
}

func leftPad(value int) string {
	raw := "000000000000" + fmt.Sprint(value)
	return raw[len(raw)-12:]
}
