//go:build integration

package engagement

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"omsbizfoundry/business-engagement/internal/database"
	"omsbizfoundry/business-engagement/internal/testdatabase"
)

const intAwardFile = "88888888-8888-4888-8888-888888888888"

type stableReferences struct{}

func (stableReferences) ValidateProject(_ context.Context, tenantID, id string, revision int64) (ProjectReference, error) {
	return ProjectReference{ID: id, TenantID: tenantID, ResourceType: "project", Status: "active", Revision: revision}, nil
}

func (stableReferences) ValidateEvidence(_ context.Context, tenantID, fileID string, version int, revision int64) (EvidenceReference, error) {
	return EvidenceReference{TenantID: tenantID, FileID: fileID, Version: version, Revision: revision, DisplayName: fileID + ".pdf"}, nil
}

func TestBusinessEngagementLifecycleIdempotencyAndHistory(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(environment.Pool)
	service, _ := NewService(store, stableReferences{})
	actor := Actor{Kind: "test_subject", ID: testActor, TenantID: testTenant, DisplayName: "经营管理员"}

	created, err := service.Create(ctx, actor, CreateInput{CommandID: command(1), ProjectID: testProject, ProjectSourceRevision: 4})
	if err != nil || created.Engagement.Revision != 1 || created.Engagement.Status != StatusDraft {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	replay, err := service.Create(ctx, actor, CreateInput{CommandID: command(1), ProjectID: testProject, ProjectSourceRevision: 4})
	if err != nil || replay.Engagement.ID != created.Engagement.ID {
		t.Fatalf("replay=%#v err=%v", replay, err)
	}
	if _, err := service.Create(ctx, actor, CreateInput{CommandID: command(1), ProjectID: testProject, ProjectSourceRevision: 5}); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("command conflict=%v", err)
	}

	id := created.Engagement.ID
	applicable, err := service.SetApplicability(ctx, actor, id, ApplicabilityInput{CommandID: command(2), ExpectedRevision: 1, Applicable: true})
	if err != nil || applicable.Engagement.Revision != 2 || applicable.Engagement.Status != StatusDraft {
		t.Fatalf("applicable=%#v err=%v", applicable, err)
	}
	method, err := service.SelectMethod(ctx, actor, id, MethodInput{CommandID: command(3), ExpectedRevision: 2, Method: MethodComparison})
	if err != nil || method.Engagement.Status != StatusCollectingEvidence || len(method.MissingRequirements) != 2 {
		t.Fatalf("method=%#v err=%v", method, err)
	}
	bid, err := service.ApplyMaterial(ctx, actor, id, MaterialInput{CommandID: command(4), ExpectedRevision: 3, Operation: "put", RequirementKey: RequirementBidDocument, EvidenceFileID: testFile, EvidenceVersion: 1, EvidenceSourceRevision: 1})
	if err != nil || bid.Engagement.Revision != 4 || bid.Engagement.Status != StatusCollectingEvidence {
		t.Fatalf("bid=%#v err=%v", bid, err)
	}
	ready, err := service.ApplyMaterial(ctx, actor, id, MaterialInput{CommandID: command(5), ExpectedRevision: 4, Operation: "put", RequirementKey: RequirementAwardDocument, EvidenceFileID: intAwardFile, EvidenceVersion: 1, EvidenceSourceRevision: 1})
	if err != nil || ready.Engagement.Status != StatusReady || len(ready.MissingRequirements) != 0 {
		t.Fatalf("ready=%#v err=%v", ready, err)
	}
	replaced, err := service.ApplyMaterial(ctx, actor, id, MaterialInput{CommandID: command(6), ExpectedRevision: 5, Operation: "put", RequirementKey: RequirementBidDocument, EvidenceFileID: "99999999-9999-4999-8999-999999999999", EvidenceVersion: 2, EvidenceSourceRevision: 2})
	if err != nil || replaced.Engagement.Revision != 6 || len(replaced.Materials) != 3 || replaced.Materials[0].SupersededAt == nil {
		t.Fatalf("replaced=%#v err=%v", replaced, err)
	}
	removed, err := service.ApplyMaterial(ctx, actor, id, MaterialInput{CommandID: command(7), ExpectedRevision: 6, Operation: "remove", RequirementKey: RequirementAwardDocument})
	if err != nil || removed.Engagement.Status != StatusCollectingEvidence || len(removed.MissingRequirements) != 1 {
		t.Fatalf("removed=%#v err=%v", removed, err)
	}
	replayedRemove, err := service.ApplyMaterial(ctx, actor, id, MaterialInput{CommandID: command(7), ExpectedRevision: 6, Operation: "remove", RequirementKey: RequirementAwardDocument})
	if err != nil || replayedRemove.Engagement.Revision != 7 {
		t.Fatalf("remove replay=%#v err=%v", replayedRemove, err)
	}
	skipped, err := service.SetApplicability(ctx, actor, id, ApplicabilityInput{CommandID: command(8), ExpectedRevision: 7, Applicable: false, Reason: "客户明确不走经营承接"})
	if err != nil || skipped.Engagement.Status != StatusNotApplicable || skipped.Engagement.Revision != 8 {
		t.Fatalf("skipped=%#v err=%v", skipped, err)
	}
	restored, err := service.SetApplicability(ctx, actor, id, ApplicabilityInput{CommandID: command(9), ExpectedRevision: 8, Applicable: true})
	if err != nil || restored.Engagement.Status != StatusCollectingEvidence || restored.Engagement.Revision != 9 || len(restored.MissingRequirements) != 1 {
		t.Fatalf("restored=%#v err=%v", restored, err)
	}
	cancelled, err := service.Cancel(ctx, actor, id, CancelInput{CommandID: command(10), ExpectedRevision: 9, Reason: "客户终止经营流程"})
	if err != nil || cancelled.Engagement.Status != StatusCancelled || cancelled.Engagement.Revision != 10 {
		t.Fatalf("cancelled=%#v err=%v", cancelled, err)
	}
	if _, err := service.SelectMethod(ctx, actor, id, MethodInput{CommandID: command(11), ExpectedRevision: 10, Method: MethodTender}); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("cancelled write=%v", err)
	}
	page, err := service.History(ctx, testTenant, id, 0, 200)
	if err != nil || len(page.Items) != 10 || page.Items[9].EventType != "engagement_cancelled" {
		t.Fatalf("history=%#v err=%v", page, err)
	}
	if _, err := environment.Pool.Exec(ctx, `UPDATE business_engagement_history_events SET event_type='engagement_created' WHERE id=$1`, page.Items[1].ID); err == nil {
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
	service, _ := NewService(store, stableReferences{})
	actor := Actor{ID: testActor, TenantID: testTenant}
	created, err := service.Create(ctx, actor, CreateInput{CommandID: command(20), ProjectID: testProject, ProjectSourceRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	inputs := []ApplicabilityInput{
		{CommandID: command(21), ExpectedRevision: 1, Applicable: true},
		{CommandID: command(22), ExpectedRevision: 1, Applicable: false, Reason: "本项目不适用"},
	}
	errorsFound := make([]error, len(inputs))
	var wait sync.WaitGroup
	for index := range inputs {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			_, errorsFound[index] = service.SetApplicability(ctx, actor, created.Engagement.ID, inputs[index])
		}(index)
	}
	wait.Wait()
	successes, conflicts := 0, 0
	for _, writeErr := range errorsFound {
		if writeErr == nil {
			successes++
		} else if errors.Is(writeErr, ErrRevisionConflict) {
			conflicts++
		} else {
			t.Fatalf("unexpected error: %v", writeErr)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d errors=%v", successes, conflicts, errorsFound)
	}
}

func command(value int) string {
	return "aaaaaaaa-aaaa-4aaa-8aaa-" + fmt.Sprintf("%012d", value)
}

func TestExactProjectFilteringPrecedesLimitAndIsTenantIsolated(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(environment.Pool)
	service, _ := NewService(store, stableReferences{})
	actor := Actor{ID: testActor, TenantID: testTenant}
	old, err := service.Create(ctx, actor, CreateInput{CommandID: command(300), ProjectID: testProject, ProjectSourceRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := environment.Pool.Exec(ctx, `UPDATE business_engagements SET updated_at=updated_at-interval '1 day' WHERE id=$1`, old.Engagement.ID); err != nil {
		t.Fatal(err)
	}
	for index := 1; index <= 3; index++ {
		_, err := service.Create(ctx, actor, CreateInput{CommandID: command(300 + index), ProjectID: fmt.Sprintf("bbbbbbbb-bbbb-4bbb-8bbb-%012d", index), ProjectSourceRevision: 1})
		if err != nil {
			t.Fatal(err)
		}
	}
	otherTenant := "99999999-9999-4999-8999-999999999999"
	_, err = service.Create(ctx, Actor{ID: testActor, TenantID: otherTenant}, CreateInput{CommandID: command(310), ProjectID: testProject, ProjectSourceRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	latest, err := service.List(ctx, testTenant, 2)
	if err != nil || len(latest) != 2 {
		t.Fatalf("latest=%#v err=%v", latest, err)
	}
	for _, item := range latest {
		if item.ID == old.Engagement.ID {
			t.Fatal("fixture did not exceed unfiltered limit")
		}
	}
	items, err := service.ListProject(ctx, testTenant, testProject, 2)
	if err != nil || len(items) != 1 || items[0].ID != old.Engagement.ID || items[0].TenantID != testTenant {
		t.Fatalf("items=%#v err=%v", items, err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := service.ListProject(ctx, testTenant, testProject, 2); err != nil {
			t.Fatal(err)
		}
	}
	history, err := service.History(ctx, testTenant, old.Engagement.ID, 0, 100)
	if err != nil || len(history.Items) != 1 || history.Items[0].EngagementRevision != 1 {
		t.Fatalf("GET appended facts: %#v err=%v", history, err)
	}
}
