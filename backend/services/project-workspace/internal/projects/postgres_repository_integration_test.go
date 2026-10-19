//go:build integration

package projects

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"omsbizfoundry/project-workspace/internal/database"
	"omsbizfoundry/project-workspace/internal/testdatabase"
)

const (
	intTenantA  = "11111111-1111-4111-8111-111111111111"
	intTenantB  = "22222222-2222-4222-8222-222222222222"
	intActorA   = "33333333-3333-4333-8333-333333333333"
	intPersonA  = "44444444-4444-4444-8444-444444444444"
	intPersonB  = "55555555-5555-4555-8555-555555555555"
	intContract = "66666666-6666-4666-8666-666666666666"
)

func TestPostgresClientPartySnapshotIsImmutableProjectHistory(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(environment.Pool)
	actor := Actor{ID: intActorA, TenantID: intTenantA}
	snapshot := &PartySnapshot{PartyID: intContract, SourceRevision: 7, Kind: "organization", LegalName: "创建时甲方名称"}
	input := CreateInput{CommandID: command(90), ProjectNumber: "OA-PARTY-SNAPSHOT", Name: "主体快照", ProjectTypeKeys: []string{"design"}, ClientParty: &PartyReferenceInput{PartyID: intContract, SourceRevision: 7}, validatedClientParty: snapshot}
	created, err := store.Create(ctx, actor, input)
	if err != nil || created.ClientParty == nil || created.ClientParty.LegalName != snapshot.LegalName {
		t.Fatalf("created=%#v error=%v", created, err)
	}
	details, err := store.Get(ctx, intTenantA, created.ID, false)
	if err != nil || details.Project.ClientParty == nil || *details.Project.ClientParty != *snapshot {
		t.Fatalf("details=%#v error=%v", details, err)
	}
	history, err := store.History(ctx, intTenantA, created.ID, "", 10)
	if err != nil || len(history.Items) != 1 {
		t.Fatalf("history=%#v error=%v", history, err)
	}
	client, ok := history.Items[0].Payload["client_party"].(map[string]any)
	if !ok || client["legal_name"] != snapshot.LegalName || client["source_revision"] != float64(snapshot.SourceRevision) {
		t.Fatalf("history payload=%#v", history.Items[0].Payload)
	}

	updated, err := store.Update(ctx, actor, created.ID, UpdateInput{CreateInput: CreateInput{CommandID: command(91), ProjectNumber: created.ProjectNumber, Name: "项目名称更正", ProjectTypeKeys: []string{"design"}}, ExpectedRevision: 1})
	if err != nil || updated.ClientParty == nil || *updated.ClientParty != *snapshot {
		t.Fatalf("updated=%#v error=%v", updated, err)
	}
}

func TestPostgresProjectLifecycleInvariantsAndHistory(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(environment.Pool)
	actor := Actor{Kind: "test_subject", ID: intActorA, TenantID: intTenantA, DisplayName: "验收管理员"}
	create := CreateInput{CommandID: command(1), ProjectNumber: "OA-001", Name: "设计演示", Summary: "J1", ProjectTypeKeys: []string{"design"}}
	project, err := store.Create(ctx, actor, create)
	if err != nil || project.Revision != 1 || project.Status != StatusDraft {
		t.Fatalf("create=%#v error=%v", project, err)
	}
	replay, err := store.Create(ctx, actor, create)
	if err != nil || replay.ID != project.ID {
		t.Fatalf("replay=%#v error=%v", replay, err)
	}
	conflicting := create
	conflicting.Name = "不同内容"
	if _, err := store.Create(ctx, actor, conflicting); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("command conflict error=%v", err)
	}
	numberConflict := create
	numberConflict.CommandID = command(2)
	numberConflict.ProjectNumber = "oa-001"
	if _, err := store.Create(ctx, actor, numberConflict); !errors.Is(err, ErrNumberConflict) {
		t.Fatalf("number conflict error=%v", err)
	}
	otherTenant := actor
	otherTenant.TenantID = intTenantB
	otherTenant.ID = intTenantB
	numberConflict.CommandID = command(3)
	if _, err := store.Create(ctx, otherTenant, numberConflict); err != nil {
		t.Fatalf("other tenant should reuse project number: %v", err)
	}

	business, err := store.ApplyAssignment(ctx, actor, project.ID, AssignmentCommand{CommandID: command(4), ExpectedRevision: 1, Operation: "assign", SubjectKind: "person", SubjectID: intPersonA, SourceRevision: 2, DisplaySnapshot: "经营负责人", RoleKey: RoleBusinessOwner, EffectiveFrom: &project.CreatedAt})
	if err != nil || business.Project.Revision != 2 || business.Assignment.SourceRevision != 2 {
		t.Fatalf("business assignment=%#v error=%v", business, err)
	}
	if _, err := store.ApplyAssignment(ctx, actor, project.ID, AssignmentCommand{CommandID: command(5), ExpectedRevision: 2, Operation: "assign", SubjectKind: "person", SubjectID: intPersonB, SourceRevision: 1, DisplaySnapshot: "另一负责人", RoleKey: RoleBusinessOwner, EffectiveFrom: &project.CreatedAt}); !errors.Is(err, ErrOwnerConflict) {
		t.Fatalf("owner conflict error=%v", err)
	}
	production, err := store.ApplyAssignment(ctx, actor, project.ID, AssignmentCommand{CommandID: command(6), ExpectedRevision: 2, Operation: "assign", SubjectKind: "group", SubjectID: intPersonB, SourceRevision: 1, DisplaySnapshot: "设计组", RoleKey: RoleProductionOwner, EffectiveFrom: &project.CreatedAt})
	if err != nil || production.Project.Revision != 3 {
		t.Fatalf("production assignment=%#v error=%v", production, err)
	}
	active, err := store.CompleteInitiation(ctx, actor, project.ID, RevisionCommand{CommandID: command(7), ExpectedRevision: 3})
	if err != nil || active.Status != StatusActive || active.Revision != 4 {
		t.Fatalf("complete=%#v error=%v", active, err)
	}
	if _, err := store.Update(ctx, actor, project.ID, UpdateInput{CreateInput: CreateInput{CommandID: command(8), ProjectNumber: "OA-001", Name: "不应修改", ProjectTypeKeys: []string{"design"}}, ExpectedRevision: 4}); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("active update error=%v", err)
	}
	link, err := store.ApplyResourceLink(ctx, actor, project.ID, ResourceLinkCommand{CommandID: command(9), ExpectedRevision: 4, Operation: "link", TargetService: "contract-ledger", ResourceType: "contract", ResourceID: intContract, RelationKey: "primary-contract"})
	if err != nil || link.Project.Revision != 5 {
		t.Fatalf("link=%#v error=%v", link, err)
	}
	if _, err := store.ApplyResourceLink(ctx, actor, project.ID, ResourceLinkCommand{CommandID: command(10), ExpectedRevision: 5, Operation: "link", TargetService: "contract-ledger", ResourceType: "contract", ResourceID: intContract, RelationKey: "primary-contract"}); !errors.Is(err, ErrActiveLinkConflict) {
		t.Fatalf("duplicate link error=%v", err)
	}
	unlinked, err := store.ApplyResourceLink(ctx, actor, project.ID, ResourceLinkCommand{CommandID: command(11), ExpectedRevision: 5, Operation: "unlink", LinkID: link.ResourceLink.ID})
	if err != nil || unlinked.Project.Revision != 6 || unlinked.ResourceLink.UnlinkedAt == nil {
		t.Fatalf("unlink=%#v error=%v", unlinked, err)
	}
	cancelled, err := store.Cancel(ctx, actor, project.ID, CancelCommand{RevisionCommand: RevisionCommand{CommandID: command(12), ExpectedRevision: 6}, Reason: "演示结束"})
	if err != nil || cancelled.Status != StatusCancelled || cancelled.Revision != 7 {
		t.Fatalf("cancel=%#v error=%v", cancelled, err)
	}
	if _, err := store.ApplyAssignment(ctx, actor, project.ID, AssignmentCommand{CommandID: command(13), ExpectedRevision: 7, Operation: "end", AssignmentID: business.Assignment.ID, EndReason: "不应成功"}); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("cancelled write error=%v", err)
	}
	details, err := store.Get(ctx, intTenantA, project.ID, true)
	if err != nil || len(details.Assignments) != 2 || len(details.ResourceLinks) != 1 {
		t.Fatalf("details=%#v error=%v", details, err)
	}
	page, err := store.History(ctx, intTenantA, project.ID, "", 100)
	if err != nil || len(page.Items) != 7 {
		t.Fatalf("history count=%d error=%v", len(page.Items), err)
	}
	for index := 1; index < len(page.Items); index++ {
		if page.Items[index-1].ProjectRevision <= page.Items[index].ProjectRevision {
			t.Fatalf("history is not descending: %#v", page.Items)
		}
	}
	if _, err := environment.Pool.Exec(ctx, `UPDATE project_history_events SET event_type='project_created' WHERE id=$1`, page.Items[0].ID); err == nil {
		t.Fatal("history update was not rejected")
	}
	if _, err := environment.Pool.Exec(ctx, `DELETE FROM project_history_events WHERE id=$1`, page.Items[0].ID); err == nil {
		t.Fatal("history delete was not rejected")
	}
}

func TestPostgresConsultationOnlyProjectCanCompleteInitiation(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(environment.Pool)
	actor := Actor{Kind: "test_subject", ID: intActorA, TenantID: intTenantA, DisplayName: "咨询验收管理员"}
	project, err := store.Create(ctx, actor, CreateInput{
		CommandID: command(14), ProjectNumber: "OA-CONSULTATION-ONLY", Name: "咨询交付演示", ProjectTypeKeys: []string{"consultation"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ApplyAssignment(ctx, actor, project.ID, AssignmentCommand{
		CommandID: command(15), ExpectedRevision: 1, Operation: "assign", SubjectKind: "person", SubjectID: intPersonA, SourceRevision: 1, DisplaySnapshot: "经营负责人", RoleKey: RoleBusinessOwner, EffectiveFrom: &project.CreatedAt,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ApplyAssignment(ctx, actor, project.ID, AssignmentCommand{
		CommandID: command(16), ExpectedRevision: 2, Operation: "assign", SubjectKind: "group", SubjectID: intPersonB, SourceRevision: 1, DisplaySnapshot: "咨询生产组", RoleKey: RoleProductionOwner, EffectiveFrom: &project.CreatedAt,
	}); err != nil {
		t.Fatal(err)
	}
	active, err := store.CompleteInitiation(ctx, actor, project.ID, RevisionCommand{CommandID: command(17), ExpectedRevision: 3})
	if err != nil || active.Status != StatusActive || active.Revision != 4 || len(active.ProjectTypeKeys) != 1 || active.ProjectTypeKeys[0] != "consultation" {
		t.Fatalf("consultation-only complete=%#v error=%v", active, err)
	}
}

func TestPostgresCompletionConfirmationIsProjectOnlyAppendOnlyAndIdempotent(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(environment.Pool)
	actor := Actor{Kind: "test_subject", ID: intActorA, TenantID: intTenantA, DisplayName: "完成检查管理员"}
	project, err := store.Create(ctx, actor, CreateInput{CommandID: command(180), ProjectNumber: "OA-C5-CHECKPOINT", Name: "C5 检查点", ProjectTypeKeys: []string{"design"}})
	if err != nil {
		t.Fatal(err)
	}
	business, err := store.ApplyAssignment(ctx, actor, project.ID, AssignmentCommand{CommandID: command(181), ExpectedRevision: 1, Operation: "assign", SubjectKind: "person", SubjectID: intPersonA, SourceRevision: 1, DisplaySnapshot: "经营负责人", RoleKey: RoleBusinessOwner})
	if err != nil {
		t.Fatal(err)
	}
	production, err := store.ApplyAssignment(ctx, actor, project.ID, AssignmentCommand{CommandID: command(182), ExpectedRevision: business.Project.Revision, Operation: "assign", SubjectKind: "group", SubjectID: intPersonB, SourceRevision: 1, DisplaySnapshot: "生产组", RoleKey: RoleProductionOwner})
	if err != nil {
		t.Fatal(err)
	}
	active, err := store.CompleteInitiation(ctx, actor, project.ID, RevisionCommand{CommandID: command(183), ExpectedRevision: production.Project.Revision})
	if err != nil || active.Revision != 4 {
		t.Fatalf("active=%#v err=%v", active, err)
	}
	approvalID := "77777777-7777-4777-8777-777777777777"
	auditID := "88888888-8888-4888-8888-888888888888"
	requirements := []CompletionRequirement{
		{Key: "approval_result_recorded", Status: CompletionSatisfied, Source: &CompletionResultReference{ID: approvalID, Kind: "approval", Revision: 2, Digest: "approval-r2"}},
		{Key: "audit_result_recorded", Status: CompletionSatisfied, Source: &CompletionResultReference{ID: auditID, Kind: "audit", Revision: 1, Digest: "audit-r1"}},
	}
	sources, err := completionSources(requirements)
	if err != nil {
		t.Fatal(err)
	}
	input := CompletionCommand{CommandID: command(184), ExpectedRevision: active.Revision, ProfileKey: CompletionProfileReferenceOAC5V1, ExpectedSources: sources}
	assessment := CompletionAssessment{ProjectID: project.ID, ProjectRevision: active.Revision, ProfileKey: CompletionProfileReferenceOAC5V1, OverallStatus: CompletionSatisfied, Requirements: requirements}
	confirmed, err := store.ConfirmCompletion(ctx, actor, project.ID, input, assessment)
	if err != nil || confirmed.ObservedProjectRevision != 4 || confirmed.ConfirmedProjectRevision != 5 || len(confirmed.Sources) != 2 {
		t.Fatalf("confirmed=%#v err=%v", confirmed, err)
	}
	replay, err := store.ConfirmCompletion(ctx, actor, project.ID, input, assessment)
	if err != nil || replay.ID != confirmed.ID {
		t.Fatalf("replay=%#v err=%v", replay, err)
	}
	latest, err := store.LatestCompletionConfirmation(ctx, intTenantA, project.ID, CompletionProfileReferenceOAC5V1)
	if err != nil || latest == nil || latest.ID != confirmed.ID {
		t.Fatalf("latest=%#v err=%v", latest, err)
	}
	details, err := store.Get(ctx, intTenantA, project.ID, true)
	if err != nil || details.Project.Status != StatusActive || details.Project.Revision != 5 {
		t.Fatalf("details=%#v err=%v", details, err)
	}
	var confirmations, events, commands int
	if err := environment.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM project_completion_confirmations WHERE tenant_id=$1 AND project_id=$2),(SELECT count(*) FROM project_history_events WHERE tenant_id=$1 AND project_id=$2 AND event_type='completion_profile_confirmed'),(SELECT count(*) FROM project_commands WHERE tenant_id=$1 AND command_id=$3)`, intTenantA, project.ID, input.CommandID).Scan(&confirmations, &events, &commands); err != nil {
		t.Fatal(err)
	}
	if confirmations != 1 || events != 1 || commands != 1 {
		t.Fatalf("confirmations=%d events=%d commands=%d", confirmations, events, commands)
	}
	if _, err := environment.Pool.Exec(ctx, `UPDATE project_completion_confirmations SET profile_key='reference-oa-c5-v1' WHERE id=$1`, confirmed.ID); err == nil {
		t.Fatal("completion confirmation update was not rejected")
	}
	stale := input
	stale.CommandID = command(185)
	if _, err := store.ConfirmCompletion(ctx, actor, project.ID, stale, assessment); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale confirmation err=%v", err)
	}
}

func TestPostgresIncompleteInitiationIsDurableAndIdempotent(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(environment.Pool)
	actor := Actor{ID: intActorA, TenantID: intTenantA}
	project, err := store.Create(ctx, actor, CreateInput{CommandID: command(20), ProjectNumber: "OA-MISSING", Name: "待补全", ProjectTypeKeys: []string{"design"}})
	if err != nil {
		t.Fatal(err)
	}
	input := RevisionCommand{CommandID: command(21), ExpectedRevision: 1}
	pending, err := store.CompleteInitiation(ctx, actor, project.ID, input)
	var incomplete *InitiationError
	if !errors.As(err, &incomplete) || pending.Status != StatusPendingCompletion || pending.Revision != 2 || len(incomplete.Missing) != 2 {
		t.Fatalf("pending=%#v missing=%#v error=%v", pending, incomplete, err)
	}
	replay, err := store.CompleteInitiation(ctx, actor, project.ID, input)
	if !errors.As(err, &incomplete) || replay.Revision != 2 {
		t.Fatalf("replay=%#v error=%v", replay, err)
	}
}

func TestPostgresOptimisticConcurrencyAllowsOneWriter(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(environment.Pool)
	actor := Actor{ID: intActorA, TenantID: intTenantA}
	project, _ := store.Create(ctx, actor, CreateInput{CommandID: command(30), ProjectNumber: "OA-RACE", Name: "并发", ProjectTypeKeys: []string{"design"}})
	inputs := []UpdateInput{
		{CreateInput: CreateInput{CommandID: command(31), ProjectNumber: "OA-RACE", Name: "并发甲", ProjectTypeKeys: []string{"design"}}, ExpectedRevision: 1},
		{CreateInput: CreateInput{CommandID: command(32), ProjectNumber: "OA-RACE", Name: "并发乙", ProjectTypeKeys: []string{"design"}}, ExpectedRevision: 1},
	}
	var wait sync.WaitGroup
	errorsFound := make([]error, 2)
	for index := range inputs {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			_, errorsFound[index] = store.Update(ctx, actor, project.ID, inputs[index])
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

func TestPostgresConcurrentSameCommandReturnsOneResult(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(environment.Pool)
	actor := Actor{ID: intActorA, TenantID: intTenantA}
	input := CreateInput{CommandID: command(40), ProjectNumber: "OA-SAME-COMMAND", Name: "并发幂等", ProjectTypeKeys: []string{"design"}}
	var wait sync.WaitGroup
	results := make([]Project, 2)
	errorsFound := make([]error, 2)
	for index := range results {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			results[index], errorsFound[index] = store.Create(ctx, actor, input)
		}(index)
	}
	wait.Wait()
	if errorsFound[0] != nil || errorsFound[1] != nil || results[0].ID != results[1].ID {
		t.Fatalf("results=%#v errors=%v", results, errorsFound)
	}
	var projectsCount, eventsCount int
	if err := environment.Pool.QueryRow(ctx, `SELECT count(*) FROM projects WHERE tenant_id=$1 AND project_number_normalized='oa-same-command'`, intTenantA).Scan(&projectsCount); err != nil {
		t.Fatal(err)
	}
	if err := environment.Pool.QueryRow(ctx, `SELECT count(*) FROM project_history_events WHERE tenant_id=$1 AND command_id=$2`, intTenantA, input.CommandID).Scan(&eventsCount); err != nil {
		t.Fatal(err)
	}
	if projectsCount != 1 || eventsCount != 1 {
		t.Fatalf("projects=%d events=%d", projectsCount, eventsCount)
	}
}

func TestPostgresOwnerReplacementEndsOldAssignmentWithoutRewrite(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(environment.Pool)
	actor := Actor{ID: intActorA, TenantID: intTenantA}
	project, _ := store.Create(ctx, actor, CreateInput{CommandID: command(50), ProjectNumber: "OA-OWNER-CHANGE", Name: "负责人更换", ProjectTypeKeys: []string{"design"}})
	first, err := store.ApplyAssignment(ctx, actor, project.ID, AssignmentCommand{CommandID: command(51), ExpectedRevision: 1, Operation: "assign", SubjectKind: "person", SubjectID: intPersonA, SourceRevision: 1, DisplaySnapshot: "原负责人", RoleKey: RoleBusinessOwner})
	if err != nil {
		t.Fatal(err)
	}
	ended, err := store.ApplyAssignment(ctx, actor, project.ID, AssignmentCommand{CommandID: command(52), ExpectedRevision: 2, Operation: "end", AssignmentID: first.Assignment.ID, EndReason: "责任调整"})
	if err != nil || ended.Assignment.EndedAt == nil || ended.Assignment.EndedProjectRevision == nil || *ended.Assignment.EndedProjectRevision != 3 {
		t.Fatalf("ended=%#v error=%v", ended, err)
	}
	replacement, err := store.ApplyAssignment(ctx, actor, project.ID, AssignmentCommand{CommandID: command(53), ExpectedRevision: 3, Operation: "assign", SubjectKind: "person", SubjectID: intPersonB, SourceRevision: 4, DisplaySnapshot: "新负责人", RoleKey: RoleBusinessOwner})
	if err != nil || replacement.Project.Revision != 4 {
		t.Fatalf("replacement=%#v error=%v", replacement, err)
	}
	details, err := store.Get(ctx, intTenantA, project.ID, true)
	if err != nil || len(details.Assignments) != 2 || details.Assignments[0].ID != first.Assignment.ID || details.Assignments[0].SubjectID != intPersonA || details.Assignments[0].EndedAt == nil || details.Assignments[1].SubjectID != intPersonB || details.Assignments[1].EndedAt != nil {
		t.Fatalf("details=%#v error=%v", details, err)
	}
}

func TestPostgresProjectPackageConversionPreservesFactsAndRollsBackSafely(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(environment.Pool)
	actor := Actor{ID: intActorA, TenantID: intTenantA}

	packageProject, err := store.CreatePackage(ctx, actor, PackageCreateInput{CommandID: command(60), Name: "华东项目包", ProjectTypeKeys: []string{"design"}})
	if err != nil || packageProject.NodeType != NodeTypePackage || packageProject.ProjectNumber != "" || packageProject.ParentProjectID != nil {
		t.Fatalf("package=%#v error=%v", packageProject, err)
	}
	project, err := store.Create(ctx, actor, CreateInput{CommandID: command(61), ProjectNumber: "OA-PKG-01", Name: "原项目", ProjectTypeKeys: []string{"design"}})
	if err != nil {
		t.Fatal(err)
	}
	otherTenantActor := Actor{ID: intTenantB, TenantID: intTenantB}
	otherTenantPackage, err := store.CreatePackage(ctx, otherTenantActor, PackageCreateInput{CommandID: command(68), Name: "其他租户项目包", ProjectTypeKeys: []string{"design"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConvertPackage(ctx, actor, project.ID, PackageConversionCommand{CommandID: command(69), Operation: ConversionAttachExisting, ExpectedProjectRevision: project.Revision, TargetPackageID: otherTenantPackage.ID, ExpectedPackageRevision: otherTenantPackage.Revision}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant package error=%v", err)
	}
	link, err := store.ApplyResourceLink(ctx, actor, project.ID, ResourceLinkCommand{CommandID: command(62), ExpectedRevision: 1, Operation: "link", TargetService: "contract-ledger", ResourceType: "contract", ResourceID: intContract, RelationKey: "primary-contract"})
	if err != nil {
		t.Fatal(err)
	}
	attached, err := store.ConvertPackage(ctx, actor, project.ID, PackageConversionCommand{CommandID: command(63), Operation: ConversionAttachExisting, ExpectedProjectRevision: link.Project.Revision, TargetPackageID: packageProject.ID, ExpectedPackageRevision: packageProject.Revision, SortOrder: 20})
	if err != nil || attached.Project.ID != project.ID || attached.Project.ParentProjectID == nil || *attached.Project.ParentProjectID != packageProject.ID || attached.Project.SortOrder != 20 || attached.Package.Revision != 2 {
		t.Fatalf("attached=%#v error=%v", attached, err)
	}
	replay, err := store.ConvertPackage(ctx, actor, project.ID, PackageConversionCommand{CommandID: command(63), Operation: ConversionAttachExisting, ExpectedProjectRevision: link.Project.Revision, TargetPackageID: packageProject.ID, ExpectedPackageRevision: packageProject.Revision, SortOrder: 20})
	if err != nil || replay.Conversion.ID != attached.Conversion.ID {
		t.Fatalf("replay=%#v error=%v", replay, err)
	}
	competing, err := store.Create(ctx, actor, CreateInput{CommandID: command(75), ProjectNumber: "OA-PKG-RACE", Name: "竞争项目", ProjectTypeKeys: []string{"design"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConvertPackage(ctx, actor, competing.ID, PackageConversionCommand{CommandID: command(76), Operation: ConversionAttachExisting, ExpectedProjectRevision: competing.Revision, TargetPackageID: packageProject.ID, ExpectedPackageRevision: packageProject.Revision}); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale package revision error=%v", err)
	}
	competingDetails, err := store.Get(ctx, intTenantA, competing.ID, true)
	if err != nil || competingDetails.Project.ParentProjectID != nil {
		t.Fatalf("stale package revision caused partial write: %#v error=%v", competingDetails, err)
	}
	details, err := store.Get(ctx, intTenantA, project.ID, true)
	if err != nil || details.Parent == nil || details.Parent.ID != packageProject.ID || len(details.ResourceLinks) != 1 || details.ResourceLinks[0].ID != link.ResourceLink.ID || details.ActivePackageConversion == nil || details.ActivePackageConversion.ID != attached.Conversion.ID {
		t.Fatalf("project details=%#v error=%v", details, err)
	}
	packageDetails, err := store.Get(ctx, intTenantA, packageProject.ID, true)
	if err != nil || len(packageDetails.Children) != 1 || packageDetails.Children[0].ID != project.ID {
		t.Fatalf("package details=%#v error=%v", packageDetails, err)
	}
	rolledBack, err := store.RollbackPackageConversion(ctx, actor, project.ID, attached.Conversion.ID, PackageRollbackCommand{CommandID: command(64), ExpectedProjectRevision: attached.Project.Revision, ExpectedPackageRevision: attached.Package.Revision, Reason: "恢复独立项目"})
	if err != nil || rolledBack.Project.ParentProjectID != nil || rolledBack.Project.ID != project.ID || rolledBack.Package.Status != StatusDraft || rolledBack.Conversion.Status != ConversionRolledBack {
		t.Fatalf("rollback=%#v error=%v", rolledBack, err)
	}
	afterRollback, err := store.Get(ctx, intTenantA, project.ID, true)
	if err != nil || len(afterRollback.ResourceLinks) != 1 || afterRollback.ResourceLinks[0].ID != link.ResourceLink.ID || afterRollback.ActivePackageConversion != nil {
		t.Fatalf("facts changed after rollback: %#v error=%v", afterRollback, err)
	}
	second, err := store.Create(ctx, actor, CreateInput{CommandID: command(65), ProjectNumber: "OA-PKG-02", Name: "安全包裹项目", ProjectTypeKeys: []string{"design"}})
	if err != nil {
		t.Fatal(err)
	}
	wrapped, err := store.ConvertPackage(ctx, actor, second.ID, PackageConversionCommand{CommandID: command(66), Operation: ConversionWrapAsPackage, ExpectedProjectRevision: 1, SortOrder: 5, NewPackage: &PackageCreateInput{CommandID: command(66), Name: "临时外层包", ProjectTypeKeys: []string{"design"}}})
	if err != nil {
		t.Fatal(err)
	}
	wrappedDetails, err := store.Get(ctx, intTenantA, second.ID, true)
	if err != nil || wrappedDetails.ActivePackageConversion == nil || wrappedDetails.ActivePackageConversion.ID != wrapped.Conversion.ID {
		t.Fatalf("wrapped conversion projection=%#v error=%v", wrappedDetails.ActivePackageConversion, err)
	}
	removed, err := store.RollbackPackageConversion(ctx, actor, second.ID, wrapped.Conversion.ID, PackageRollbackCommand{CommandID: command(67), ExpectedProjectRevision: wrapped.Project.Revision, ExpectedPackageRevision: wrapped.Package.Revision, Reason: "方案撤回"})
	if err != nil || removed.Project.ParentProjectID != nil || removed.Package.Status != StatusCancelled || removed.Package.CancellationReason == nil {
		t.Fatalf("wrapped rollback=%#v error=%v", removed, err)
	}
}

func TestPostgresWrapPackageRejectsUnsafeRollbackAndDeeperNesting(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(environment.Pool)
	actor := Actor{ID: intActorA, TenantID: intTenantA}
	project, _ := store.Create(ctx, actor, CreateInput{CommandID: command(70), ProjectNumber: "OA-WRAP-01", Name: "待包裹项目", ProjectTypeKeys: []string{"design"}})
	wrapped, err := store.ConvertPackage(ctx, actor, project.ID, PackageConversionCommand{CommandID: command(71), Operation: ConversionWrapAsPackage, ExpectedProjectRevision: 1, SortOrder: 10, NewPackage: &PackageCreateInput{CommandID: command(71), Name: "自动外层包", ProjectTypeKeys: []string{"design"}}})
	if err != nil || wrapped.Project.ID != project.ID || !wrapped.Conversion.CreatedPackage || wrapped.Package.NodeType != NodeTypePackage {
		t.Fatalf("wrapped=%#v error=%v", wrapped, err)
	}
	if _, err := store.ConvertPackage(ctx, actor, wrapped.Package.ID, PackageConversionCommand{CommandID: command(72), Operation: ConversionWrapAsPackage, ExpectedProjectRevision: wrapped.Package.Revision, NewPackage: &PackageCreateInput{CommandID: command(72), Name: "不允许第三层", ProjectTypeKeys: []string{"design"}}}); !errors.Is(err, ErrHierarchyConflict) {
		t.Fatalf("package nesting error=%v", err)
	}
	changed, err := store.ApplyResourceLink(ctx, actor, wrapped.Package.ID, ResourceLinkCommand{CommandID: command(73), ExpectedRevision: wrapped.Package.Revision, Operation: "link", TargetService: "evidence-service", ResourceType: "evidence-file", ResourceID: intContract, RelationKey: "package-brief"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.RollbackPackageConversion(ctx, actor, project.ID, wrapped.Conversion.ID, PackageRollbackCommand{CommandID: command(74), ExpectedProjectRevision: wrapped.Project.Revision, ExpectedPackageRevision: changed.Project.Revision, Reason: "尝试自动撤销"}); !errors.Is(err, ErrRollbackUnsafe) {
		t.Fatalf("unsafe rollback error=%v", err)
	}
	after, err := store.Get(ctx, intTenantA, project.ID, true)
	if err != nil || after.Project.ParentProjectID == nil || *after.Project.ParentProjectID != wrapped.Package.ID {
		t.Fatalf("unsafe rollback caused partial write: %#v error=%v", after, err)
	}
}

func command(value int) string {
	return "aaaaaaaa-aaaa-4aaa-8aaa-" + leftPad(value)
}

func leftPad(value int) string {
	raw := "000000000000" + fmt.Sprint(value)
	return raw[len(raw)-12:]
}
