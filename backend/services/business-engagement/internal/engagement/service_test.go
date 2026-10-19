package engagement

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

const (
	testTenant     = "11111111-1111-4111-8111-111111111111"
	testActor      = "22222222-2222-4222-8222-222222222222"
	testCommand    = "33333333-3333-4333-8333-333333333333"
	testProject    = "44444444-4444-4444-8444-444444444444"
	testEngagement = "55555555-5555-4555-8555-555555555555"
	testFile       = "66666666-6666-4666-8666-666666666666"
)

type fakeStore struct {
	replayFound bool
	replayValue Details
	createInput CreateInput
	material    MaterialInput
	details     Details
	listTenant  string
	listProject string
	listLimit   int
}

func (f *fakeStore) Ping(context.Context) error                              { return nil }
func (f *fakeStore) List(context.Context, string, int) ([]Engagement, error) { return nil, nil }
func (f *fakeStore) ListProject(_ context.Context, tenant, project string, limit int) ([]Engagement, error) {
	f.listTenant, f.listProject, f.listLimit = tenant, project, limit
	return []Engagement{f.details.Engagement}, nil
}
func (f *fakeStore) Get(context.Context, string, string) (Details, error) { return f.details, nil }
func (f *fakeStore) History(context.Context, string, string, int64, int) (HistoryPage, error) {
	return HistoryPage{}, nil
}
func (f *fakeStore) Replay(_ context.Context, _, _, _ string, destination any) (bool, error) {
	if f.replayFound {
		*destination.(*Details) = f.replayValue
	}
	return f.replayFound, nil
}
func (f *fakeStore) Create(_ context.Context, _ Actor, input CreateInput, _ ProjectReference, _ string) (Details, error) {
	f.createInput = input
	return Details{Engagement: Engagement{ID: testEngagement}}, nil
}
func (f *fakeStore) SetApplicability(context.Context, Actor, string, ApplicabilityInput, string) (Details, error) {
	return Details{}, nil
}
func (f *fakeStore) SelectMethod(context.Context, Actor, string, MethodInput, string) (Details, error) {
	return Details{}, nil
}
func (f *fakeStore) ApplyMaterial(_ context.Context, _ Actor, _ string, input MaterialInput, _ string) (Details, error) {
	f.material = input
	return Details{}, nil
}
func (f *fakeStore) Cancel(context.Context, Actor, string, CancelInput, string) (Details, error) {
	return Details{}, nil
}

type fakeReferences struct {
	projectCalls  int
	evidenceCalls int
	project       ProjectReference
	evidence      EvidenceReference
	err           error
}

func (f *fakeReferences) ValidateProject(context.Context, string, string, int64) (ProjectReference, error) {
	f.projectCalls++
	return f.project, f.err
}
func (f *fakeReferences) ValidateEvidence(context.Context, string, string, int, int64) (EvidenceReference, error) {
	f.evidenceCalls++
	return f.evidence, f.err
}

func TestCreateReplaysBeforeExternalReferenceLookup(t *testing.T) {
	store := &fakeStore{replayFound: true, replayValue: Details{Engagement: Engagement{ID: testEngagement}}}
	references := &fakeReferences{err: errors.New("must not be called")}
	service, _ := NewService(store, references)
	value, err := service.Create(context.Background(), Actor{ID: testActor, TenantID: testTenant}, CreateInput{CommandID: testCommand, ProjectID: testProject, ProjectSourceRevision: 7})
	if err != nil || value.Engagement.ID != testEngagement || references.projectCalls != 0 {
		t.Fatalf("replay result=%#v err=%v project_calls=%d", value, err, references.projectCalls)
	}
}

func TestListProjectNormalizesBeforeOwnStoreAndDoesNotReadDependencies(t *testing.T) {
	store := &fakeStore{details: Details{Engagement: Engagement{ID: testEngagement}}}
	references := &fakeReferences{err: errors.New("no dependency reads")}
	service, _ := NewService(store, references)
	items, err := service.ListProject(context.Background(), testTenant, "  "+strings.ToUpper(testProject)+"  ", 0)
	if err != nil || len(items) != 1 || store.listTenant != testTenant || store.listProject != testProject || store.listLimit != 50 || references.projectCalls != 0 {
		t.Fatalf("items=%v err=%v store=%#v references=%#v", items, err, store, references)
	}
	for _, input := range []struct {
		project string
		limit   int
	}{{"", 2}, {"bad", 2}, {testProject, -1}, {testProject, 101}} {
		store.listProject = ""
		if _, err := service.ListProject(context.Background(), testTenant, input.project, input.limit); err == nil || store.listProject != "" {
			t.Fatalf("invalid query reached store: %#v err=%v", input, err)
		}
	}
}

func TestCreateRequiresActiveProjectReference(t *testing.T) {
	store := &fakeStore{}
	references := &fakeReferences{project: ProjectReference{ID: testProject, TenantID: testTenant, ResourceType: "project", Status: "cancelled", Revision: 7}}
	service, _ := NewService(store, references)
	_, err := service.Create(context.Background(), Actor{ID: testActor, TenantID: testTenant}, CreateInput{CommandID: testCommand, ProjectID: testProject, ProjectSourceRevision: 7})
	if !errors.Is(err, ErrReferenceInvalid) || store.createInput.CommandID != "" {
		t.Fatalf("err=%v create=%#v", err, store.createInput)
	}
}

func TestMaterialStoresEvidenceDisplaySnapshot(t *testing.T) {
	store := &fakeStore{}
	references := &fakeReferences{evidence: EvidenceReference{TenantID: testTenant, FileID: testFile, Version: 2, Revision: 9, DisplayName: "中标通知书.pdf"}}
	service, _ := NewService(store, references)
	_, err := service.ApplyMaterial(context.Background(), Actor{ID: testActor, TenantID: testTenant}, testEngagement, MaterialInput{CommandID: testCommand, ExpectedRevision: 3, Operation: "put", RequirementKey: RequirementAwardDocument, EvidenceFileID: testFile, EvidenceVersion: 2, EvidenceSourceRevision: 9})
	if err != nil || references.evidenceCalls != 1 || store.material.DisplaySnapshot != "中标通知书.pdf" {
		t.Fatalf("err=%v calls=%d material=%#v", err, references.evidenceCalls, store.material)
	}
}

func TestReferenceMaterialsPublishesOnlyCurrentDeterministicEvidenceReferences(t *testing.T) {
	supersededAt := time.Now().UTC()
	store := &fakeStore{details: Details{
		Engagement: Engagement{ID: testEngagement, TenantID: testTenant, ProjectID: testProject, Status: StatusReady, Revision: 7},
		Materials: []MaterialReference{
			{RequirementKey: RequirementBidDocument, EvidenceFileID: testFile, EvidenceVersion: 2, EvidenceSourceRevision: 8, DisplaySnapshot: "投标文件.pdf"},
			{RequirementKey: RequirementAwardDocument, EvidenceFileID: "77777777-7777-4777-8777-777777777777", EvidenceVersion: 1, EvidenceSourceRevision: 4, DisplaySnapshot: "中标通知书.pdf"},
			{RequirementKey: RequirementTenderDocument, EvidenceFileID: "88888888-8888-4888-8888-888888888888", EvidenceVersion: 1, EvidenceSourceRevision: 1, DisplaySnapshot: "旧招标文件.pdf", SupersededAt: &supersededAt},
		},
	}}
	service, _ := NewService(store, &fakeReferences{})
	value, err := service.ReferenceMaterials(context.Background(), testTenant, testEngagement)
	if err != nil || value.SchemaVersion != "1" || value.SourceApplication != "business-engagement" || value.ProjectID != testProject || value.Revision != 7 || len(value.Items) != 2 {
		t.Fatalf("projection=%#v error=%v", value, err)
	}
	if value.Items[0].RelationKey != RequirementAwardDocument || value.Items[1].RelationKey != RequirementBidDocument || len(value.Digest) != 64 || strings.Trim(value.Digest, "0123456789abcdef") != "" {
		t.Fatalf("projection ordering or digest=%#v", value)
	}
}
