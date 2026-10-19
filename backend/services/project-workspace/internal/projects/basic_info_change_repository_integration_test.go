//go:build integration

package projects

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"omsbizfoundry/project-workspace/internal/database"
	"omsbizfoundry/project-workspace/internal/testdatabase"
)

func TestPostgresBasicInfoAndOwnerChangesAreAtomicHistoricalAndReplayable(t *testing.T) {
	ctx := context.Background()
	env := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, env.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(env.Pool)
	actor := Actor{ID: intActorA, TenantID: intTenantA}
	for index, node := range []string{NodeTypeProject, NodeTypePackage} {
		t.Run(node, func(t *testing.T) {
			seq := 1800 + index*50
			p, err := store.DispatchInitiation(ctx, actor, InitiationDispatchInput{CommandID: command(seq), NodeType: node, Name: "初始", BusinessOwner: InitiationOwnerReference{PersonID: intPersonA, SourceRevision: 1}, ownerSnapshot: "原经营"})
			if err != nil {
				t.Fatal(err)
			}
			number := "OA-OWNER-001"
			if node == NodeTypePackage {
				number = ""
			}
			p, err = store.Update(ctx, actor, p.ID, UpdateInput{CreateInput: CreateInput{CommandID: command(seq + 1), ProjectNumber: number, Name: "初始", ProjectTypeKeys: []string{"design"}}, ExpectedRevision: p.Revision})
			if err != nil {
				t.Fatal(err)
			}
			p, err = store.CompleteInitiation(ctx, actor, p.ID, RevisionCommand{CommandID: command(seq + 2), ExpectedRevision: p.Revision, InitiationProfile: InitiationProfileBusinessOwnerV1})
			if err != nil {
				t.Fatal(err)
			}
			refrev := int64(1)
			linked, err := store.ApplyResourceLink(ctx, actor, p.ID, ResourceLinkCommand{CommandID: command(seq + 3), ExpectedRevision: p.Revision, Operation: "link", TargetService: "contract-ledger", ResourceType: "contract", ResourceID: intContract, SourceRevision: &refrev, RelationKey: "business-contract"})
			if err != nil {
				t.Fatal(err)
			}
			p = linked.Project
			before, _ := store.Get(ctx, intTenantA, p.ID, true)
			profile := basicInfoFields(CreateInput{ProjectNumber: number, Name: "更名并调整负责人", ProjectTypeKeys: []string{"design"}})
			in := BasicInfoChangeInput{CommandID: command(seq + 4), ExpectedRevision: p.Revision, Reason: "修改项目基本信息", Profile: &profile, Owners: []OwnerChange{{RoleKey: RoleBusinessOwner, Operation: "set", Person: &InitiationOwnerReference{PersonID: intPersonB, SourceRevision: 2}, snapshot: "新经营"}, {RoleKey: RoleProductionOwner, Operation: "set", Person: &InitiationOwnerReference{PersonID: intPersonA, SourceRevision: 1}, snapshot: "新生产"}}}
			changed, err := store.ChangeBasicInfo(ctx, actor, p.ID, in)
			if err != nil || changed.Project.Revision != p.Revision+1 || len(changed.Owners) != 2 {
				t.Fatalf("change=%#v error=%v", changed, err)
			}
			after, _ := store.Get(ctx, intTenantA, p.ID, true)
			if after.Project.Name != profile.Name || len(after.Assignments) != 3 || !reflect.DeepEqual(before.ResourceLinks, after.ResourceLinks) || !reflect.DeepEqual(before.Project.ClientParty, after.Project.ClientParty) || !reflect.DeepEqual(before.Project.ParentProjectID, after.Project.ParentProjectID) {
				t.Fatalf("after=%#v", after)
			}
			old := after.Assignments[0]
			for _, v := range after.Assignments {
				if v.ID == before.Assignments[0].ID {
					old = v
				}
			}
			if old.EndedAt == nil || old.DisplaySnapshot != "原经营" || old.SubjectID != intPersonA || *old.EndedProjectRevision != changed.Project.Revision {
				t.Fatalf("history overwritten %#v", old)
			}
			replay, err := store.ChangeBasicInfo(ctx, actor, p.ID, in)
			if err != nil || replay.Project.Revision != changed.Project.Revision {
				t.Fatalf("replay=%v %#v", err, replay)
			}
			mismatch := in
			mismatch.Reason = "不同"
			if _, err := store.ChangeBasicInfo(ctx, actor, p.ID, mismatch); !errors.Is(err, ErrCommandConflict) {
				t.Fatalf("different=%v", err)
			}
			stale := in
			stale.CommandID = command(seq + 5)
			if _, err := store.ChangeBasicInfo(ctx, actor, p.ID, stale); !errors.Is(err, ErrRevisionConflict) {
				t.Fatalf("stale=%v", err)
			}
			if _, err := store.ChangeBasicInfo(ctx, Actor{ID: intActorA, TenantID: intTenantB}, p.ID, stale); !errors.Is(err, ErrNotFound) {
				t.Fatalf("tenant=%v", err)
			}
			noop := in
			noop.CommandID = command(seq + 6)
			noop.ExpectedRevision = changed.Project.Revision
			nochange, err := store.ChangeBasicInfo(ctx, actor, p.ID, noop)
			if err != nil || nochange.Project.Revision != changed.Project.Revision {
				t.Fatalf("noop=%v %#v", err, nochange)
			}
			clear := BasicInfoChangeInput{CommandID: command(seq + 7), ExpectedRevision: changed.Project.Revision, Reason: "清除负责人", Owners: []OwnerChange{{RoleKey: RoleBusinessOwner, Operation: "clear"}, {RoleKey: RoleProductionOwner, Operation: "clear"}}}
			cleared, err := store.ChangeBasicInfo(ctx, actor, p.ID, clear)
			if err != nil || cleared.Project.Revision != changed.Project.Revision+1 {
				t.Fatalf("clear=%v %#v", err, cleared)
			}
			for _, v := range cleared.Owners {
				if v.Current != nil || v.Previous.EndedAt == nil {
					t.Fatalf("not cleared %#v", v)
				}
			}
			clear.CommandID = command(seq + 8)
			clear.ExpectedRevision = cleared.Project.Revision
			if nochange, err := store.ChangeBasicInfo(ctx, actor, p.ID, clear); err != nil || nochange.Project.Revision != cleared.Project.Revision {
				t.Fatalf("empty clear=%v", err)
			}
			history, err := store.History(ctx, intTenantA, p.ID, "", 100)
			if err != nil || len(history.Items) != 6 || history.Items[0].Payload["schema_version"] != "project-basic-info-change-v1" {
				t.Fatalf("history count=%d err=%v", len(history.Items), err)
			}
			if _, err := store.Cancel(ctx, actor, p.ID, CancelCommand{RevisionCommand: RevisionCommand{CommandID: command(seq + 9), ExpectedRevision: cleared.Project.Revision}, Reason: "测试结束"}); err != nil {
				t.Fatal(err)
			}
			if _, err := store.ChangeBasicInfo(ctx, actor, p.ID, in); err != nil {
				t.Fatalf("cancelled replay=%v", err)
			}
			clear.CommandID = command(seq + 10)
			clear.ExpectedRevision = cleared.Project.Revision + 1
			if _, err := store.ChangeBasicInfo(ctx, actor, p.ID, clear); !errors.Is(err, ErrStateConflict) {
				t.Fatalf("cancelled new=%v", err)
			}
		})
	}
}
func TestPostgresBasicInfoFailureRollbackAndConcurrentOwners(t *testing.T) {
	ctx := context.Background()
	env := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, env.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(env.Pool)
	actor := Actor{ID: intActorA, TenantID: intTenantA}
	create := func(seq int, number string) Project {
		p, err := store.Create(ctx, actor, CreateInput{CommandID: command(seq), ProjectNumber: number, Name: "项目", ProjectTypeKeys: []string{"design"}})
		if err != nil {
			t.Fatal(err)
		}
		a, err := store.ApplyAssignment(ctx, actor, p.ID, AssignmentCommand{CommandID: command(seq + 1), ExpectedRevision: 1, Operation: "assign", SubjectKind: "person", SubjectID: intPersonA, SourceRevision: 1, DisplaySnapshot: "原经营", RoleKey: RoleBusinessOwner})
		if err != nil {
			t.Fatal(err)
		}
		p, err = store.CompleteInitiation(ctx, actor, p.ID, RevisionCommand{CommandID: command(seq + 2), ExpectedRevision: a.Project.Revision, InitiationProfile: InitiationProfileBusinessOwnerV1})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := create(1950, "OWNER-001")
	_ = create(1960, "OWNER-002")
	before, _ := store.Get(ctx, intTenantA, p.ID, true)
	fields := basicInfoFields(CreateInput{Name: p.Name, ProjectNumber: "OWNER-002", ProjectTypeKeys: p.ProjectTypeKeys})
	in := BasicInfoChangeInput{CommandID: command(1970), ExpectedRevision: p.Revision, Reason: "更正", Profile: &fields, Owners: []OwnerChange{{RoleKey: RoleBusinessOwner, Operation: "set", Person: &InitiationOwnerReference{PersonID: intPersonB, SourceRevision: 1}, snapshot: "新人"}}}
	if _, err := store.ChangeBasicInfo(ctx, actor, p.ID, in); !errors.Is(err, ErrNumberConflict) {
		t.Fatalf("number=%v", err)
	}
	after, _ := store.Get(ctx, intTenantA, p.ID, true)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed profile left owner changes")
	}
	in.Profile = nil
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			copy := in
			copy.CommandID = command(1971 + index)
			_, err := store.ChangeBasicInfo(ctx, actor, p.ID, copy)
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrRevisionConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("concurrent=%d/%d", success, conflict)
	}
	current, _ := store.Get(ctx, intTenantA, p.ID, true)
	future := time.Now().UTC().Add(time.Hour)
	a, err := store.ApplyAssignment(ctx, actor, p.ID, AssignmentCommand{CommandID: command(1974), ExpectedRevision: current.Project.Revision, Operation: "assign", SubjectKind: "person", SubjectID: intPersonA, SourceRevision: 1, DisplaySnapshot: "计划分工", RoleKey: RoleProductionOwner, EffectiveFrom: &future})
	if err != nil {
		t.Fatal(err)
	}
	futureBefore, _ := store.Get(ctx, intTenantA, p.ID, true)
	in.CommandID = command(1975)
	in.ExpectedRevision = a.Project.Revision
	in.Owners = []OwnerChange{{RoleKey: RoleBusinessOwner, Operation: "clear"}, {RoleKey: RoleProductionOwner, Operation: "clear"}}
	if _, err := store.ChangeBasicInfo(ctx, actor, p.ID, in); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("future=%v", err)
	}
	futureAfter, _ := store.Get(ctx, intTenantA, p.ID, true)
	if !reflect.DeepEqual(futureBefore, futureAfter) {
		t.Fatal("future failure left partial clear")
	}
}
