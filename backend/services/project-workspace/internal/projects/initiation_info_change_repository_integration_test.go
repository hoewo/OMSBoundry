//go:build integration

package projects

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"omsbizfoundry/project-workspace/internal/database"
	"omsbizfoundry/project-workspace/internal/testdatabase"
)

func TestPostgresLegacyInitiationRecoveryIsAtomicHistoricalAndReplayable(t *testing.T) {
	ctx := context.Background()
	env := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, env.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(env.Pool)
	actor := Actor{ID: intActorA, TenantID: intTenantA}
	p, err := store.Create(ctx, actor, CreateInput{CommandID: command(2200), ProjectNumber: "OA-RECOVERY-101", Name: "旧待立项", ProjectTypeKeys: []string{"consultation", "survey"}})
	if err != nil {
		t.Fatal(err)
	}
	a, err := store.ApplyAssignment(ctx, actor, p.ID, AssignmentCommand{CommandID: command(2201), ExpectedRevision: p.Revision, Operation: "assign", SubjectKind: "person", SubjectID: intPersonA, SourceRevision: 1, DisplaySnapshot: "原生产", RoleKey: RoleProductionOwner})
	if err != nil {
		t.Fatal(err)
	}
	p, err = store.CompleteInitiation(ctx, actor, p.ID, RevisionCommand{CommandID: command(2202), ExpectedRevision: a.Project.Revision, InitiationProfile: InitiationProfileBusinessOwnerV1})
	if !errors.Is(err, ErrInitiationIncomplete) || p.Status != StatusPendingCompletion {
		t.Fatalf("pending=%#v err=%v", p, err)
	}
	before, _ := store.Get(ctx, intTenantA, p.ID, true)
	fields := basicInfoFields(CreateInput{ProjectNumber: p.ProjectNumber, Name: "补齐旧项目", ProjectTypeKeys: p.ProjectTypeKeys})
	in := BasicInfoChangeInput{CommandID: command(2203), ExpectedRevision: p.Revision, Reason: "补齐项目立项资料", Profile: &fields, Owners: []OwnerChange{
		{RoleKey: RoleBusinessOwner, Operation: "set", Person: &InitiationOwnerReference{PersonID: intPersonB, SourceRevision: 2}, snapshot: "新经营"},
		{RoleKey: RoleProductionOwner, Operation: "set", Person: &InitiationOwnerReference{PersonID: intPersonB, SourceRevision: 2}, snapshot: "新生产"},
	}}
	if _, err := store.ChangeBasicInfo(ctx, actor, p.ID, in); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("active-only contract widened: %v", err)
	}
	broken := in
	broken.Owners = append([]OwnerChange(nil), in.Owners...)
	broken.Owners[1].snapshot = ""
	if _, err := store.ChangeInitiationInfo(ctx, actor, p.ID, broken); err == nil {
		t.Fatal("accepted unvalidated snapshot")
	}
	afterFailure, _ := store.Get(ctx, intTenantA, p.ID, true)
	if !reflect.DeepEqual(before, afterFailure) {
		t.Fatal("failed save left partial profile or owner writes")
	}
	result, err := store.ChangeInitiationInfo(ctx, actor, p.ID, in)
	if err != nil || result.Project.Revision != p.Revision+1 || result.Project.Status != StatusPendingCompletion || len(result.Owners) != 2 {
		t.Fatalf("recovery=%#v err=%v", result, err)
	}
	after, _ := store.Get(ctx, intTenantA, p.ID, true)
	if len(after.Assignments) != 3 || after.Project.Name != fields.Name {
		t.Fatalf("after=%#v", after)
	}
	for _, owner := range after.Assignments {
		if owner.ID == a.Assignment.ID && (owner.EndedAt == nil || owner.DisplaySnapshot != "原生产" || owner.SubjectID != intPersonA || *owner.EndedProjectRevision != result.Project.Revision) {
			t.Fatalf("old history overwritten: %#v", owner)
		}
	}
	history, err := store.History(ctx, intTenantA, p.ID, "", 100)
	if err != nil || history.Items[0].Payload["schema_version"] != "project-initiation-info-change-v1" {
		t.Fatalf("history=%#v err=%v", history, err)
	}
	noop := in
	noop.CommandID = command(2204)
	noop.ExpectedRevision = result.Project.Revision
	unchanged, err := store.ChangeInitiationInfo(ctx, actor, p.ID, noop)
	if err != nil || unchanged.Project.Revision != result.Project.Revision {
		t.Fatalf("noop=%#v err=%v", unchanged, err)
	}
	stale := in
	stale.CommandID = command(2205)
	if _, err := store.ChangeInitiationInfo(ctx, actor, p.ID, stale); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale=%v", err)
	}
	if _, err := store.ChangeInitiationInfo(ctx, Actor{ID: intActorA, TenantID: intTenantB}, p.ID, stale); !errors.Is(err, ErrNotFound) {
		t.Fatalf("tenant=%v", err)
	}
	active, err := store.CompleteInitiation(ctx, actor, p.ID, RevisionCommand{CommandID: command(2206), ExpectedRevision: result.Project.Revision, InitiationProfile: InitiationProfileBusinessOwnerV1})
	if err != nil || active.Status != StatusActive {
		t.Fatalf("complete=%#v err=%v", active, err)
	}
	if replay, err := store.ChangeInitiationInfo(ctx, actor, p.ID, in); err != nil || replay.Project.Revision != result.Project.Revision {
		t.Fatalf("post-activation replay=%#v %v", replay, err)
	}
	if _, err := store.ChangeBasicInfo(ctx, actor, p.ID, in); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("command namespaces conflated: %v", err)
	}
	noop.CommandID = command(2207)
	noop.ExpectedRevision = active.Revision
	if _, err := store.ChangeInitiationInfo(ctx, actor, p.ID, noop); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("active recovery accepted: %v", err)
	}
	if _, err := store.Cancel(ctx, actor, p.ID, CancelCommand{RevisionCommand: RevisionCommand{CommandID: command(2208), ExpectedRevision: active.Revision}, Reason: "验证结束"}); err != nil {
		t.Fatal(err)
	}
	noop.CommandID = command(2209)
	noop.ExpectedRevision = active.Revision + 1
	if _, err := store.ChangeInitiationInfo(ctx, actor, p.ID, noop); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("cancelled recovery accepted: %v", err)
	}
}
