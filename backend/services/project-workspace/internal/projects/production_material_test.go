package projects

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

const productionMaterialWorkItem = "66666666-6666-4666-8666-666666666671"
const productionMaterialFile = "77777777-7777-4777-8777-777777777771"

func productionMaterialSource(projectID string) ResourceReference {
	return ResourceReference{SourceApplication: "production-operations", ID: productionMaterialWorkItem, TenantID: testTenant, ResourceType: "production-work-item", Status: "in-progress", Revision: 5, MaterialSource: &MaterialSourceFacts{ProjectID: projectID, Digest: strings.Repeat("b", 64), Items: []MaterialSourceItem{{RelationKey: "report.primary-output", FileID: productionMaterialFile, Version: 2, EvidenceSourceRevision: 2, DisplayName: "咨询成果.pdf"}}}}
}

func productionMaterialLink() ResourceLink {
	revision := int64(5)
	return ResourceLink{ID: testCommand, TargetService: "production-operations", ResourceType: "production-work-item", ResourceID: productionMaterialWorkItem, RelationKey: "production-materials", SourceRevision: &revision}
}

func TestProductionMaterialsTwoLayerCompositionAndIsolatedFailures(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*ResourceReference, *ResourceLink, *materialReferencesStub)
		state  string
		calls  int
	}{
		{"available", func(*ResourceReference, *ResourceLink, *materialReferencesStub) {}, MaterialSegmentAvailable, 2},
		{"source drift", func(v *ResourceReference, _ *ResourceLink, _ *materialReferencesStub) { v.Revision = 6 }, MaterialSegmentRevisionChanged, 2},
		{"file version drift", func(_ *ResourceReference, _ *ResourceLink, r *materialReferencesStub) {
			key := "evidence-service/evidence-file/" + productionMaterialFile
			file := r.values[key]
			file.Revision, file.Evidence.Version = 3, 3
			r.values[key] = file
		}, MaterialSegmentRevisionChanged, 2},
		{"wrong project", func(v *ResourceReference, _ *ResourceLink, _ *materialReferencesStub) {
			v.MaterialSource.ProjectID = testActorID
		}, MaterialSegmentInvalid, 1},
		{"wrong tenant", func(v *ResourceReference, _ *ResourceLink, _ *materialReferencesStub) { v.TenantID = testActorID }, MaterialSegmentInvalid, 1},
		{"wrong id", func(v *ResourceReference, _ *ResourceLink, _ *materialReferencesStub) { v.ID = testActorID }, MaterialSegmentInvalid, 1},
		{"wrong source", func(v *ResourceReference, _ *ResourceLink, _ *materialReferencesStub) {
			v.SourceApplication = "design-operations"
		}, MaterialSegmentInvalid, 1},
		{"empty", func(v *ResourceReference, _ *ResourceLink, _ *materialReferencesStub) { v.MaterialSource.Items = nil }, MaterialSegmentEmpty, 1},
		{"empty source drift", func(v *ResourceReference, _ *ResourceLink, _ *materialReferencesStub) {
			v.MaterialSource.Items = nil
			v.Revision = 6
		}, MaterialSegmentRevisionChanged, 1},
		{"source forbidden", func(_ *ResourceReference, _ *ResourceLink, r *materialReferencesStub) {
			r.errors["production-operations/production-work-item/"+productionMaterialWorkItem] = ErrReferenceForbidden
		}, MaterialSegmentForbidden, 1},
		{"file unavailable", func(_ *ResourceReference, _ *ResourceLink, r *materialReferencesStub) {
			r.errors["evidence-service/evidence-file/"+productionMaterialFile] = ErrReferenceUnavailable
		}, MaterialSegmentUnavailable, 2},
		{"file forbidden", func(_ *ResourceReference, _ *ResourceLink, r *materialReferencesStub) {
			r.errors["evidence-service/evidence-file/"+productionMaterialFile] = ErrReferenceForbidden
		}, MaterialSegmentForbidden, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			source, link := productionMaterialSource(testProject), productionMaterialLink()
			refs := &materialReferencesStub{values: map[string]ResourceReference{"evidence-service/evidence-file/" + productionMaterialFile: {ID: productionMaterialFile, TenantID: testTenant, SourceApplication: "evidence-service", ResourceType: "evidence-file", Status: "available", Revision: 2, Evidence: &EvidenceFileFacts{Version: 2, FileName: "咨询成果.pdf", SizeBytes: 64, MediaType: "application/pdf", SHA256: strings.Repeat("a", 64)}}}, errors: map[string]error{}}
			test.mutate(&source, &link, refs)
			refs.values["production-operations/production-work-item/"+productionMaterialWorkItem] = source
			store := &storeStub{details: Details{Project: Project{ID: testProject, TenantID: testTenant, Revision: 9}, ResourceLinks: []ResourceLink{link}}}
			service, _ := NewService(store, refs)
			value, err := service.Materials(context.Background(), testTenant, testProject)
			if err != nil || value.ProjectRevision != 9 || len(value.Segments) != 1 || value.Segments[0].State != test.state || len(refs.calls) != test.calls || store.resourceCalls != 0 {
				t.Fatalf("collection=%#v calls=%v writes=%d err=%v", value, refs.calls, store.resourceCalls, err)
			}
			if test.state == MaterialSegmentAvailable && (value.Segments[0].SourceApplication != "production-operations" || value.Segments[0].EvidenceFileID != productionMaterialFile || value.Segments[0].EvidenceVersion != 2 || value.Segments[0].SourceRelationKey != "production-materials") {
				t.Fatalf("lost source lineage=%#v", value.Segments[0])
			}
		})
	}
}

func TestProductionMaterialsDoNotGuessLegacyOrUnlinkedResources(t *testing.T) {
	link := productionMaterialLink()
	unlinked := time.Now().UTC()
	link.UnlinkedAt = &unlinked
	store := &storeStub{details: Details{Project: Project{ID: testProject, TenantID: testTenant}, ResourceLinks: []ResourceLink{link, {TargetService: "design-operations", ResourceType: "design-work-item", ResourceID: productionMaterialWorkItem, RelationKey: "old-design"}}}}
	refs := &materialReferencesStub{}
	service, _ := NewService(store, refs)
	value, err := service.Materials(context.Background(), testTenant, testProject)
	if err != nil || value.State != MaterialCollectionEmpty || len(value.Segments) != 0 || len(refs.calls) != 0 {
		t.Fatalf("legacy/unlinked source was guessed: %#v calls=%v err=%v", value, refs.calls, err)
	}
	if !ValidResource("production-operations", "production-work-item", "production-materials") || ValidResource("production-operations", "design-work-item", "production-materials") || ValidResource("design-operations", "production-work-item", "production-materials") {
		t.Fatal("canonical and legacy type catalogs were mixed")
	}
}

func TestProductionResourceLinkRejectsWrongProjectBeforeWriting(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*ResourceReference)
		valid  bool
	}{
		{"valid", func(*ResourceReference) {}, true},
		{"cross project", func(v *ResourceReference) { v.MaterialSource.ProjectID = testActorID }, false},
		{"wrong tenant", func(v *ResourceReference) { v.TenantID = testActorID }, false},
		{"wrong source", func(v *ResourceReference) { v.SourceApplication = "design-operations" }, false},
		{"stale revision", func(v *ResourceReference) { v.Revision = 6 }, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := productionMaterialSource(testProject)
			test.mutate(&source)
			store := &storeStub{}
			refs := &materialReferencesStub{values: map[string]ResourceReference{"production-operations/production-work-item/" + productionMaterialWorkItem: source}}
			service, _ := NewService(store, refs)
			revision := int64(5)
			_, err := service.ApplyResourceLink(context.Background(), Actor{ID: testActorID, TenantID: testTenant}, testProject, ResourceLinkCommand{CommandID: testCommand, Operation: "link", ExpectedRevision: 9, TargetService: "production-operations", ResourceType: "production-work-item", ResourceID: productionMaterialWorkItem, RelationKey: "production-materials", SourceRevision: &revision})
			if test.valid {
				if err != nil || store.resourceCalls != 1 {
					t.Fatalf("valid link writes=%d err=%v", store.resourceCalls, err)
				}
			} else if !errors.Is(err, ErrReferenceInvalid) || store.resourceCalls != 0 {
				t.Fatalf("invalid source wrote link: writes=%d err=%v", store.resourceCalls, err)
			}
		})
	}
}
