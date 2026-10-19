//go:build integration

package engagement

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"omsbizfoundry/business-engagement/internal/database"
	"omsbizfoundry/business-engagement/internal/testdatabase"
)

func collectionFixture(t *testing.T) (*Service, *PostgresStore, Actor, string) {
	t.Helper()
	env := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(context.Background(), env.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(env.Pool)
	service, _ := NewService(store, stableReferences{})
	actor := Actor{ID: testActor, TenantID: testTenant, Kind: "test_subject"}
	d, err := service.Create(context.Background(), actor, CreateInput{CommandID: command(500), ProjectID: testProject, ProjectSourceRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	id := d.Engagement.ID
	if _, err = service.SetApplicability(context.Background(), actor, id, ApplicabilityInput{CommandID: command(501), ExpectedRevision: 1, Applicable: true}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.SelectMethod(context.Background(), actor, id, MethodInput{CommandID: command(502), ExpectedRevision: 2, Method: MethodComparison}); err != nil {
		t.Fatal(err)
	}
	return service, store, actor, id
}

func currentMaterials(details Details, key string) []MaterialReference {
	var result []MaterialReference
	for _, item := range details.Materials {
		if item.RequirementKey == key && item.SupersededAt == nil {
			result = append(result, item)
		}
	}
	return result
}

func TestMaterialCollectionAppendRemoveOneLegacyAndHistory(t *testing.T) {
	ctx := context.Background()
	s, _, actor, id := collectionFixture(t)
	input := MaterialInput{CommandID: command(510), ExpectedRevision: 3, Operation: "append", RequirementKey: RequirementBidDocument, EvidenceFileID: testFile, EvidenceVersion: 1, EvidenceSourceRevision: 1}
	first, err := s.ApplyMaterial(ctx, actor, id, input)
	if err != nil {
		t.Fatal(err)
	}
	firstID := currentMaterials(first, RequirementBidDocument)[0].ID
	input.CommandID, input.ExpectedRevision, input.EvidenceFileID = command(511), 4, intAwardFile
	second, err := s.ApplyMaterial(ctx, actor, id, input)
	if err != nil || len(currentMaterials(second, RequirementBidDocument)) != 2 || second.Engagement.Revision != 5 {
		t.Fatalf("second=%#v err=%v", second, err)
	}
	projection, err := s.ReferenceMaterials(ctx, testTenant, id)
	if err != nil || len(projection.Items) != 2 || projection.Items[0].RelationKey != projection.Items[1].RelationKey {
		t.Fatalf("projection=%#v err=%v", projection, err)
	}
	// Same command replays its original response, with no additional revision.
	replay, err := s.ApplyMaterial(ctx, actor, id, input)
	if err != nil || !equalMaterialWire(t, replay, second) {
		t.Fatalf("replay=%#v err=%v", replay, err)
	}
	t.Logf("complete replay JSON is equal; scanned Location=%s, replay Location=%s", second.Engagement.CreatedAt.Location(), replay.Engagement.CreatedAt.Location())
	before := second
	for index, change := range []func(*MaterialInput){
		func(i *MaterialInput) {
			i.Operation = "append"
			i.MaterialReferenceID = ""
			i.EvidenceFileID = intAwardFile
			i.EvidenceVersion = 1
			i.EvidenceSourceRevision = 1
		},
		func(i *MaterialInput) { i.MaterialReferenceID = testProject },
		func(i *MaterialInput) { i.RequirementKey = RequirementAwardDocument },
	} {
		bad := MaterialInput{CommandID: command(520 + index), ExpectedRevision: 5, Operation: "remove_one", RequirementKey: RequirementBidDocument, MaterialReferenceID: firstID}
		change(&bad)
		if _, err := s.ApplyMaterial(ctx, actor, id, bad); !errors.Is(err, ErrStateConflict) {
			t.Fatalf("bad=%#v err=%v", bad, err)
		}
	}
	foreign := actor
	foreign.TenantID = intAwardFile
	if _, err := s.ApplyMaterial(ctx, foreign, id, MaterialInput{CommandID: command(529), ExpectedRevision: 5, Operation: "remove_one", RequirementKey: RequirementBidDocument, MaterialReferenceID: firstID}); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross tenant removal", err)
	}
	unchanged, _ := s.Get(ctx, testTenant, id)
	if !reflect.DeepEqual(unchanged, before) {
		t.Fatal("rejection partially wrote facts")
	}
	// A real, active neighbour in the same tenant must not be removable through
	// this Engagement's endpoint, even when its kind and file are identical.
	neighbour, err := s.Create(ctx, actor, CreateInput{CommandID: command(540), ProjectID: intAwardFile, ProjectSourceRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	neighbourID := neighbour.Engagement.ID
	if _, err = s.SetApplicability(ctx, actor, neighbourID, ApplicabilityInput{CommandID: command(541), ExpectedRevision: 1, Applicable: true}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SelectMethod(ctx, actor, neighbourID, MethodInput{CommandID: command(542), ExpectedRevision: 2, Method: MethodComparison}); err != nil {
		t.Fatal(err)
	}
	neighbour, err = s.ApplyMaterial(ctx, actor, neighbourID, MaterialInput{CommandID: command(543), ExpectedRevision: 3, Operation: "append", RequirementKey: RequirementBidDocument, EvidenceFileID: testFile, EvidenceVersion: 1, EvidenceSourceRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	neighbourReference := currentMaterials(neighbour, RequirementBidDocument)[0].ID
	if _, err = s.ApplyMaterial(ctx, actor, id, MaterialInput{CommandID: command(544), ExpectedRevision: 5, Operation: "remove_one", RequirementKey: RequirementBidDocument, MaterialReferenceID: neighbourReference}); !errors.Is(err, ErrStateConflict) {
		t.Fatal("real neighbouring reference was accepted", err)
	}
	if got, err := s.Get(ctx, testTenant, neighbourID); err != nil || !equalMaterialWire(t, got, neighbour) {
		t.Fatal("neighbour changed after rejected removal", err)
	}
	if got, err := s.Get(ctx, testTenant, id); err != nil || !equalMaterialWire(t, got, before) {
		t.Fatal("current engagement changed after rejected neighbour removal", err)
	}
	remove := MaterialInput{CommandID: command(530), ExpectedRevision: 5, Operation: "remove_one", RequirementKey: RequirementBidDocument, MaterialReferenceID: firstID}
	removed, err := s.ApplyMaterial(ctx, actor, id, remove)
	if err != nil || removed.Engagement.Revision != 6 || len(currentMaterials(removed, RequirementBidDocument)) != 1 || len(removed.Materials) != 2 || len(removed.MissingRequirements) != 1 {
		t.Fatalf("removed=%#v err=%v", removed, err)
	}
	if got, err := s.ApplyMaterial(ctx, actor, id, remove); err != nil || !equalMaterialWire(t, got, removed) {
		t.Fatal("remove replay", err)
	}
	remove.CommandID, remove.ExpectedRevision = command(531), 6
	if _, err := s.ApplyMaterial(ctx, actor, id, remove); !errors.Is(err, ErrStateConflict) {
		t.Fatal("superseded removal", err)
	}
	// Legacy put must still replace every current member, not silently append.
	input.CommandID, input.ExpectedRevision, input.Operation, input.EvidenceFileID = command(532), 6, "put", testProject
	replaced, err := s.ApplyMaterial(ctx, actor, id, input)
	if err != nil || len(currentMaterials(replaced, RequirementBidDocument)) != 1 || len(replaced.Materials) != 3 {
		t.Fatalf("legacy put=%#v err=%v", replaced, err)
	}
	input.CommandID, input.ExpectedRevision, input.Operation, input.EvidenceFileID = command(533), 7, "append", testFile
	appended, err := s.ApplyMaterial(ctx, actor, id, input)
	if err != nil || len(currentMaterials(appended, RequirementBidDocument)) != 2 {
		t.Fatal("append after replace", err)
	}
	all, err := s.ApplyMaterial(ctx, actor, id, MaterialInput{CommandID: command(534), ExpectedRevision: 8, Operation: "remove", RequirementKey: RequirementBidDocument})
	if err != nil || len(currentMaterials(all, RequirementBidDocument)) != 0 || len(all.Materials) != 4 {
		t.Fatalf("legacy remove=%#v err=%v", all, err)
	}
	history, err := s.History(ctx, testTenant, id, 0, 200)
	if err != nil || len(history.Items) != 9 || history.Items[3].EventType != "material_append" || history.Items[5].EventType != "material_remove_one" || history.Items[5].Payload["material_reference_id"] != firstID {
		t.Fatalf("history=%#v err=%v", history, err)
	}
}

func TestMaterialCollectionConcurrentLimitAndLegacyReplacement(t *testing.T) {
	ctx := context.Background()
	s, _, actor, id := collectionFixture(t)
	for index := 0; index < 99; index++ {
		_, err := s.ApplyMaterial(ctx, actor, id, MaterialInput{CommandID: command(600 + index), ExpectedRevision: int64(3 + index), Operation: "append", RequirementKey: RequirementBidDocument, EvidenceFileID: fmt.Sprintf("bbbbbbbb-bbbb-4bbb-8bbb-%012d", index), EvidenceVersion: 1, EvidenceSourceRevision: 1})
		if err != nil {
			t.Fatal(index, err)
		}
	}
	errs := make([]error, 2)
	var wait sync.WaitGroup
	for index := range errs {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			_, errs[index] = s.ApplyMaterial(ctx, actor, id, MaterialInput{CommandID: command(710 + index), ExpectedRevision: 102, Operation: "append", RequirementKey: RequirementBidDocument, EvidenceFileID: fmt.Sprintf("cccccccc-cccc-4ccc-8ccc-%012d", index), EvidenceVersion: 1, EvidenceSourceRevision: 1})
		}(index)
	}
	wait.Wait()
	successes, conflicts := 0, 0
	for _, err := range errs {
		if err == nil {
			successes++
		} else if errors.Is(err, ErrRevisionConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent errors=%v", errs)
	}
	before, _ := s.Get(ctx, testTenant, id)
	if len(currentMaterials(before, RequirementBidDocument)) != 100 {
		t.Fatal("limit fixture")
	}
	input := MaterialInput{CommandID: command(720), ExpectedRevision: 103, Operation: "append", RequirementKey: RequirementAwardDocument, EvidenceFileID: intAwardFile, EvidenceVersion: 1, EvidenceSourceRevision: 1}
	if _, err := s.ApplyMaterial(ctx, actor, id, input); !errors.Is(err, ErrStateConflict) {
		t.Fatal("101st", err)
	}
	input.CommandID, input.Operation = command(721), "put"
	if _, err := s.ApplyMaterial(ctx, actor, id, input); !errors.Is(err, ErrStateConflict) {
		t.Fatal("new kind exceeds total", err)
	}
	after, _ := s.Get(ctx, testTenant, id)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("bound rejection partially wrote")
	}
	projection, err := s.ReferenceMaterials(ctx, testTenant, id)
	if err != nil || len(projection.Items) != 100 {
		t.Fatal("consumer bound", err)
	}
	input.CommandID, input.RequirementKey = command(722), RequirementBidDocument
	replaced, err := s.ApplyMaterial(ctx, actor, id, input)
	if err != nil || len(currentMaterials(replaced, RequirementBidDocument)) != 1 || len(replaced.Materials) != 101 {
		t.Fatalf("replace at bound=%#v err=%v", replaced, err)
	}
}
