//go:build integration

package projects

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"omsbizfoundry/project-workspace/internal/database"
	"omsbizfoundry/project-workspace/internal/testdatabase"
)

func TestPostgresActiveProfileCorrectionKeepsOwnedAndReferencedFacts(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(environment.Pool)
	actor := Actor{ID: intActorA, TenantID: intTenantA}
	for index, node := range []string{NodeTypeProject, NodeTypePackage} {
		t.Run(node, func(t *testing.T) {
			seq := 1100 + index*30
			number := "OA-PROFILE-001"
			if node == NodeTypePackage {
				number = ""
			}
			created, err := store.DispatchInitiation(ctx, actor, InitiationDispatchInput{CommandID: command(seq), NodeType: node, Name: "待立项", BusinessOwner: InitiationOwnerReference{PersonID: intPersonA, SourceRevision: 1}, ownerSnapshot: "经营负责人"})
			if err != nil {
				t.Fatal(err)
			}
			fields := CreateInput{CommandID: command(seq + 1), ProjectNumber: number, Name: "基本资料", Summary: "原概况", ProjectTypeKeys: []string{"design", "survey"}}
			input := ProfileCorrectionInput{UpdateInput: UpdateInput{CreateInput: fields, ExpectedRevision: 1}, CorrectionReason: "资料更正"}
			if _, err := store.CorrectProfile(ctx, actor, created.ID, input); !errors.Is(err, ErrStateConflict) {
				t.Fatalf("draft correction=%v", err)
			}
			updated, err := store.Update(ctx, actor, created.ID, input.UpdateInput)
			if err != nil {
				t.Fatal(err)
			}
			active, err := store.CompleteInitiation(ctx, actor, created.ID, RevisionCommand{CommandID: command(seq + 2), ExpectedRevision: updated.Revision, InitiationProfile: InitiationProfileBusinessOwnerV1})
			if err != nil {
				t.Fatal(err)
			}
			sourceRevision := int64(7)
			linked, err := store.ApplyResourceLink(ctx, actor, created.ID, ResourceLinkCommand{CommandID: command(seq + 3), ExpectedRevision: active.Revision, Operation: "link", TargetService: "contract-ledger", ResourceType: "contract", ResourceID: intContract, RelationKey: "business-contract", SourceRevision: &sourceRevision})
			if err != nil {
				t.Fatal(err)
			}
			before, err := store.Get(ctx, intTenantA, created.ID, true)
			if err != nil {
				t.Fatal(err)
			}
			input.CommandID, input.ExpectedRevision, input.Name, input.ProjectTypeKeys = command(seq+4), linked.Project.Revision, "更正后名称", []string{"design", "consultation"}
			corrected, err := store.CorrectProfile(ctx, actor, created.ID, input)
			if err != nil || corrected.ID != created.ID || corrected.Status != StatusActive || corrected.Revision != before.Project.Revision+1 || corrected.Name != input.Name {
				t.Fatalf("correction=%#v error=%v", corrected, err)
			}
			freshStore, _ := NewPostgresStore(environment.Pool)
			after, err := freshStore.Get(ctx, intTenantA, created.ID, true)
			if err != nil || !reflect.DeepEqual(before.Assignments, after.Assignments) || !reflect.DeepEqual(before.ResourceLinks, after.ResourceLinks) || !reflect.DeepEqual(before.Project.ClientParty, after.Project.ClientParty) || !reflect.DeepEqual(before.Project.ParentProjectID, after.Project.ParentProjectID) {
				t.Fatalf("referenced facts changed: %#v error=%v", after, err)
			}
			replay, err := freshStore.CorrectProfile(ctx, actor, created.ID, input)
			if err != nil || replay.Revision != corrected.Revision {
				t.Fatalf("replay=%#v error=%v", replay, err)
			}
			changed := input
			changed.CorrectionReason = "不同原因"
			if _, err := store.CorrectProfile(ctx, actor, created.ID, changed); !errors.Is(err, ErrCommandConflict) {
				t.Fatalf("changed receipt=%v", err)
			}
			stale := input
			stale.CommandID = command(seq + 5)
			if _, err := store.CorrectProfile(ctx, actor, created.ID, stale); !errors.Is(err, ErrRevisionConflict) {
				t.Fatalf("stale=%v", err)
			}
			if _, err := store.CorrectProfile(ctx, Actor{TenantID: intTenantB, ID: intActorA}, created.ID, stale); !errors.Is(err, ErrNotFound) {
				t.Fatalf("cross tenant=%v", err)
			}
			legacy := input.UpdateInput
			legacy.CommandID, legacy.ExpectedRevision = command(seq+6), corrected.Revision
			if _, err := store.Update(ctx, actor, created.ID, legacy); !errors.Is(err, ErrStateConflict) {
				t.Fatalf("legacy active update=%v", err)
			}
			unchanged := input
			unchanged.CommandID, unchanged.ExpectedRevision = command(seq+7), corrected.Revision
			noChange, err := store.CorrectProfile(ctx, actor, created.ID, unchanged)
			if err != nil || noChange.Revision != corrected.Revision {
				t.Fatalf("no change=%#v error=%v", noChange, err)
			}
			history, err := store.History(ctx, intTenantA, created.ID, "", 100)
			if err != nil || len(history.Items) != 5 {
				t.Fatalf("history=%#v error=%v", history, err)
			}
			latest := history.Items[0]
			if latest.EventType != "project_details_replaced" || latest.Payload["correction_reason"] != input.CorrectionReason || latest.Payload["previous_revision"] != float64(input.ExpectedRevision) {
				t.Fatalf("correction audit=%#v", latest)
			}
			if _, err := store.Cancel(ctx, actor, created.ID, CancelCommand{RevisionCommand: RevisionCommand{CommandID: command(seq + 8), ExpectedRevision: corrected.Revision}, Reason: "保留历史"}); err != nil {
				t.Fatal(err)
			}
			if replay, err := store.CorrectProfile(ctx, actor, created.ID, input); err != nil || replay.Revision != corrected.Revision {
				t.Fatalf("cancelled replay=%#v error=%v", replay, err)
			}
			cancelledNew := input
			cancelledNew.CommandID, cancelledNew.ExpectedRevision = command(seq+9), corrected.Revision+1
			if _, err := store.CorrectProfile(ctx, actor, created.ID, cancelledNew); !errors.Is(err, ErrStateConflict) {
				t.Fatalf("new cancelled correction=%v", err)
			}
		})
	}
}

func TestPostgresProfileCorrectionNumberConflictConcurrencyAndClientSnapshot(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(environment.Pool)
	actor := Actor{ID: intActorA, TenantID: intTenantA}
	snapshot := &PartySnapshot{PartyID: intContract, SourceRevision: 7, Kind: "organization", LegalName: "不可覆盖的甲方快照"}
	created, err := store.Create(ctx, actor, CreateInput{CommandID: command(1200), ProjectNumber: "OA-SNAPSHOT-EDIT", Name: "主体快照", ProjectTypeKeys: []string{"design"}, ClientParty: &PartyReferenceInput{PartyID: intContract, SourceRevision: 7}, validatedClientParty: snapshot})
	if err != nil {
		t.Fatal(err)
	}
	assigned, err := store.ApplyAssignment(ctx, actor, created.ID, AssignmentCommand{CommandID: command(1201), ExpectedRevision: 1, Operation: "assign", SubjectKind: "person", SubjectID: intPersonA, SourceRevision: 1, DisplaySnapshot: "经营", RoleKey: RoleBusinessOwner})
	if err != nil {
		t.Fatal(err)
	}
	active, err := store.CompleteInitiation(ctx, actor, created.ID, RevisionCommand{CommandID: command(1202), ExpectedRevision: assigned.Project.Revision, InitiationProfile: InitiationProfileBusinessOwnerV1})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Create(ctx, actor, CreateInput{CommandID: command(1203), ProjectNumber: "OA-NUMBER-OCCUPIED", Name: "已有编号", ProjectTypeKeys: []string{"design"}})
	if err != nil {
		t.Fatal(err)
	}
	input := ProfileCorrectionInput{UpdateInput: UpdateInput{CreateInput: CreateInput{CommandID: command(1204), ProjectNumber: "OA-NUMBER-OCCUPIED", Name: "编号冲突", ProjectTypeKeys: []string{"design"}}, ExpectedRevision: active.Revision}, CorrectionReason: "改编号"}
	if _, err := store.CorrectProfile(ctx, actor, active.ID, input); !errors.Is(err, ErrNumberConflict) {
		t.Fatalf("number conflict=%v", err)
	}
	input.ProjectNumber = active.ProjectNumber
	var wg sync.WaitGroup
	outcomes := make(chan error, 2)
	for i := 0; i < 2; i++ {
		attempt := input
		attempt.CommandID = command(1205 + i)
		attempt.Name = "并发更正"
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.CorrectProfile(ctx, actor, active.ID, attempt)
			outcomes <- err
		}()
	}
	wg.Wait()
	close(outcomes)
	success, conflicts := 0, 0
	for err := range outcomes {
		if err == nil {
			success++
		} else if errors.Is(err, ErrRevisionConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("success=%d conflicts=%d", success, conflicts)
	}
	after, err := store.Get(ctx, intTenantA, active.ID, true)
	if err != nil || after.Project.Revision != active.Revision+1 || after.Project.ClientParty == nil || *after.Project.ClientParty != *snapshot {
		t.Fatalf("snapshot or revision changed incorrectly: %#v error=%v", after, err)
	}
	history, err := store.History(ctx, intTenantA, active.ID, "", 100)
	if err != nil || len(history.Items) != 4 {
		t.Fatalf("history len=%d error=%v", len(history.Items), err)
	}
	client := history.Items[0].Payload["client_party"].(map[string]any)
	if client["legal_name"] != snapshot.LegalName {
		t.Fatalf("history lost snapshot: %#v", client)
	}
}
