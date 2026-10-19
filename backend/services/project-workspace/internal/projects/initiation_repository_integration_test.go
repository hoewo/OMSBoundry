//go:build integration

package projects

import (
	"context"
	"errors"
	"testing"

	"omsbizfoundry/project-workspace/internal/database"
	"omsbizfoundry/project-workspace/internal/testdatabase"
)

func TestPostgresBusinessOwnerInitiationAndLaterProductionAssignment(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(environment.Pool)
	actor := Actor{ID: intActorA, TenantID: intTenantA}
	project, err := store.Create(ctx, actor, CreateInput{CommandID: command(900), ProjectNumber: "OA-DEFERRED-OWNER", Name: "生产负责人后补", ProjectTypeKeys: []string{"design"}})
	if err != nil {
		t.Fatal(err)
	}
	assigned, err := store.ApplyAssignment(ctx, actor, project.ID, AssignmentCommand{CommandID: command(901), ExpectedRevision: 1, Operation: "assign", SubjectKind: "person", SubjectID: intPersonA, SourceRevision: 1, DisplaySnapshot: "经营负责人", RoleKey: RoleBusinessOwner})
	if err != nil {
		t.Fatal(err)
	}
	input := RevisionCommand{CommandID: command(902), ExpectedRevision: assigned.Project.Revision, InitiationProfile: InitiationProfileBusinessOwnerV1}
	active, err := store.CompleteInitiation(ctx, actor, project.ID, input)
	if err != nil || active.Status != StatusActive || active.Revision != 3 {
		t.Fatalf("active=%#v error=%v", active, err)
	}
	production, err := store.ApplyAssignment(ctx, actor, project.ID, AssignmentCommand{CommandID: command(903), ExpectedRevision: active.Revision, Operation: "assign", SubjectKind: "person", SubjectID: intPersonB, SourceRevision: 2, DisplaySnapshot: "后补生产负责人", RoleKey: RoleProductionOwner})
	if err != nil || production.Project.Status != StatusActive || production.Project.Revision != 4 {
		t.Fatalf("later production=%#v error=%v", production, err)
	}
	replay, err := store.CompleteInitiation(ctx, actor, project.ID, input)
	if err != nil || replay.Revision != active.Revision || replay.ID != active.ID {
		t.Fatalf("replay=%#v error=%v", replay, err)
	}
	changed := input
	changed.InitiationProfile = InitiationProfileDualOwnerV1
	if _, err := store.CompleteInitiation(ctx, actor, project.ID, changed); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("changed profile replay error=%v", err)
	}
	history, err := store.History(ctx, intTenantA, project.ID, "", 20)
	if err != nil || len(history.Items) != 4 {
		t.Fatalf("history=%#v error=%v", history, err)
	}
	foundActivation := false
	for _, item := range history.Items {
		if item.EventType == "project_activated" {
			foundActivation = item.Payload["initiation_profile"] == InitiationProfileBusinessOwnerV1
		}
	}
	if !foundActivation {
		t.Fatal("activation history must capture the selected readiness profile")
	}
}

func TestPostgresBusinessOwnerProfileCannotSkipBusinessOwnerAndRejectsUnknownProfile(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(environment.Pool)
	actor := Actor{ID: intActorA, TenantID: intTenantA}
	project, err := store.Create(ctx, actor, CreateInput{CommandID: command(910), ProjectNumber: "OA-STILL-REQUIRES-OWNER", Name: "仍需经营负责人", ProjectTypeKeys: []string{"survey"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteInitiation(ctx, actor, project.ID, RevisionCommand{CommandID: command(911), ExpectedRevision: 1, InitiationProfile: "skip-owners"}); err == nil {
		t.Fatal("unknown profile accepted")
	}
	details, err := store.Get(ctx, intTenantA, project.ID, true)
	if err != nil || details.Project.Revision != 1 || details.Project.Status != StatusDraft {
		t.Fatalf("invalid profile changed project=%#v error=%v", details, err)
	}
	input := RevisionCommand{CommandID: command(912), ExpectedRevision: 1, InitiationProfile: InitiationProfileBusinessOwnerV1}
	pending, err := store.CompleteInitiation(ctx, actor, project.ID, input)
	var incomplete *InitiationError
	if !errors.As(err, &incomplete) || pending.Revision != 2 || pending.Status != StatusPendingCompletion || len(incomplete.Missing) != 1 || incomplete.Missing[0] != RoleBusinessOwner {
		t.Fatalf("pending=%#v missing=%#v error=%v", pending, incomplete, err)
	}
	replay, err := store.CompleteInitiation(ctx, actor, project.ID, input)
	if !errors.As(err, &incomplete) || replay.Revision != 2 {
		t.Fatalf("pending replay=%#v error=%v", replay, err)
	}
}

func TestPostgresDispatchThenCompleteKeepsProjectIDAndAtomicOwner(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(environment.Pool)
	actor := Actor{ID: intActorA, TenantID: intTenantA}
	for i, nodeType := range []string{NodeTypeProject, NodeTypePackage} {
		input := InitiationDispatchInput{CommandID: command(920 + i*10), NodeType: nodeType, Name: "名称与经营负责人派发", BusinessOwner: InitiationOwnerReference{PersonID: intPersonA, SourceRevision: 2}, ownerSnapshot: "经营负责人"}
		created, err := store.DispatchInitiation(ctx, actor, input)
		if err != nil || created.ProjectNumber != "" || len(created.ProjectTypeKeys) != 0 || created.Revision != 1 || created.Status != StatusDraft {
			t.Fatalf("created=%#v error=%v", created, err)
		}
		details, err := store.Get(ctx, intTenantA, created.ID, true)
		if err != nil || len(details.Assignments) != 1 || details.Assignments[0].RoleKey != RoleBusinessOwner || details.Assignments[0].SourceRevision != 2 {
			t.Fatalf("details=%#v error=%v", details, err)
		}
		replay, err := store.DispatchInitiation(ctx, actor, input)
		if err != nil || replay.ID != created.ID || replay.Revision != 1 {
			t.Fatalf("dispatch replay=%#v error=%v", replay, err)
		}
		if _, err := store.Get(ctx, intTenantB, created.ID, true); !errors.Is(err, ErrNotFound) {
			t.Fatalf("cross tenant access=%v", err)
		}
		pending, err := store.CompleteInitiation(ctx, actor, created.ID, RevisionCommand{CommandID: command(921 + i*10), ExpectedRevision: 1, InitiationProfile: InitiationProfileBusinessOwnerV1})
		var missing *InitiationError
		if !errors.As(err, &missing) || pending.Status != StatusPendingCompletion || pending.Revision != 2 {
			t.Fatalf("uncompleted draft activated: project=%#v error=%v", pending, err)
		}
		number := "OA-DISPATCH-001"
		if nodeType == NodeTypePackage {
			number = ""
		}
		updated, err := store.Update(ctx, actor, created.ID, UpdateInput{CreateInput: CreateInput{CommandID: command(922 + i*10), ProjectNumber: number, Name: created.Name, ProjectTypeKeys: []string{"design", "mapping"}}, ExpectedRevision: 2})
		if err != nil {
			t.Fatal(err)
		}
		active, err := store.CompleteInitiation(ctx, actor, created.ID, RevisionCommand{CommandID: command(923 + i*10), ExpectedRevision: updated.Revision, InitiationProfile: InitiationProfileBusinessOwnerV1})
		if err != nil || active.ID != created.ID || active.Status != StatusActive || active.Revision != 4 {
			t.Fatalf("active=%#v error=%v", active, err)
		}
		input.Name = "同一命令不同内容"
		if _, err := store.DispatchInitiation(ctx, actor, input); !errors.Is(err, ErrCommandConflict) {
			t.Fatalf("dispatch payload conflict=%v", err)
		}
	}
	// A failed owner insert must also roll back the newly inserted Project.
	invalid := InitiationDispatchInput{CommandID: command(949), NodeType: NodeTypeProject, Name: "应全部回滚", BusinessOwner: InitiationOwnerReference{PersonID: intPersonA, SourceRevision: 1}}
	if _, err := store.DispatchInitiation(ctx, actor, invalid); err == nil {
		t.Fatal("missing owner display snapshot should fail")
	}
	var projectsCount, commandsCount int
	if err := environment.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM projects), (SELECT count(*) FROM project_commands WHERE command_id=$1)`, invalid.CommandID).Scan(&projectsCount, &commandsCount); err != nil || projectsCount != 2 || commandsCount != 0 {
		t.Fatalf("partial dispatch persisted: projects=%d commands=%d error=%v", projectsCount, commandsCount, err)
	}
}
