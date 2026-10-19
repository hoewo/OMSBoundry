//go:build integration

package projects

import (
	"context"
	"errors"
	"omsbizfoundry/project-workspace/internal/database"
	"omsbizfoundry/project-workspace/internal/testdatabase"
	"testing"
)

func TestPostgresCommercialContributorHistoryReplayAndOldOwnerInvariants(t *testing.T) {
	ctx := context.Background()
	env := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, env.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(env.Pool)
	actor := Actor{ID: intActorA, TenantID: intTenantA}
	p, err := store.Create(ctx, actor, CreateInput{CommandID: command(501), ProjectNumber: "COMMERCIAL-501", Name: "经营参与闭环", ProjectTypeKeys: []string{"design"}})
	if err != nil {
		t.Fatal(err)
	}
	input := AssignmentCommand{CommandID: command(502), ExpectedRevision: p.Revision, Operation: "assign", SubjectKind: "person", SubjectID: intPersonA, SourceRevision: 2, RoleKey: RoleCommercialContributor, DisplaySnapshot: "参与人员"}
	added, err := store.ApplyAssignment(ctx, actor, p.ID, input)
	if err != nil || added.Project.Revision != 2 {
		t.Fatalf("add=%+v err=%v", added, err)
	}
	replay, err := store.ApplyAssignment(ctx, actor, p.ID, input)
	if err != nil || replay.Assignment.ID != added.Assignment.ID || replay.Project.Revision != 2 {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	duplicate := input
	duplicate.CommandID = command(503)
	duplicate.ExpectedRevision = 2
	if _, err := store.ApplyAssignment(ctx, actor, p.ID, duplicate); err == nil {
		t.Fatal("duplicate active commercial assignment accepted")
	}
	forged := duplicate
	forged.CommandID = command(504)
	forged.SubjectKind = "group"
	forged.SubjectID = intPersonB
	if _, err := store.ApplyAssignment(ctx, actor, p.ID, forged); err == nil {
		t.Fatal("database accepted commercial group")
	}
	owner := duplicate
	owner.CommandID = command(505)
	owner.RoleKey = RoleBusinessOwner
	owned, err := store.ApplyAssignment(ctx, actor, p.ID, owner)
	if err != nil || owned.Project.Revision != 3 {
		t.Fatalf("owner=%+v err=%v", owned, err)
	}
	owner.CommandID = command(506)
	owner.ExpectedRevision = 3
	owner.SubjectID = intPersonB
	if _, err := store.ApplyAssignment(ctx, actor, p.ID, owner); !errors.Is(err, ErrOwnerConflict) {
		t.Fatalf("owner invariant changed: %v", err)
	}
	end := AssignmentCommand{CommandID: command(507), ExpectedRevision: 3, Operation: "end", AssignmentID: added.Assignment.ID, EndReason: "移出经营参与人"}
	ended, err := store.ApplyAssignment(ctx, actor, p.ID, end)
	if err != nil || ended.Assignment.EndedAt == nil || ended.Project.Revision != 4 {
		t.Fatalf("end=%+v err=%v", ended, err)
	}
	again, err := store.ApplyAssignment(ctx, actor, p.ID, end)
	if err != nil || again.Assignment.ID != added.Assignment.ID || again.Project.Revision != 4 {
		t.Fatalf("end replay=%+v err=%v", again, err)
	}
	input.CommandID = command(508)
	input.ExpectedRevision = 4
	rejoined, err := store.ApplyAssignment(ctx, actor, p.ID, input)
	if err != nil || rejoined.Assignment.ID == added.Assignment.ID {
		t.Fatalf("rejoin=%+v err=%v", rejoined, err)
	}
	all, err := store.Get(ctx, intTenantA, p.ID, true)
	if err != nil || len(all.Assignments) != 3 || all.Project.Revision != 5 {
		t.Fatalf("details=%+v err=%v", all, err)
	}
	active, err := store.Get(ctx, intTenantA, p.ID, false)
	if err != nil || len(active.Assignments) != 2 {
		t.Fatalf("active=%+v err=%v", active, err)
	}
	history, err := store.History(ctx, intTenantA, p.ID, "", 100)
	if err != nil || len(history.Items) != 5 {
		t.Fatalf("history=%+v err=%v", history, err)
	}
	if _, err := store.Get(ctx, intTenantB, p.ID, true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross tenant: %v", err)
	}
	t.Log("commercial_person_only=true duplicate_rejected=true replay_no_revision=true ended_history_retained=true rejoin_new_identity=true old_owner_unique=true cross_tenant_denied=true")
}
