package projects

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

var recordedSources = []struct{ service, kind string }{
	{"approval-audit-results", "approval_audit_result"},
	{"cost-outsourcing", "cost_entry"},
	{"cost-outsourcing", "outsourcing_engagement"},
}

func TestRecordedMaterialLinksAreExplicitAndSourceProjectChecked(t *testing.T) {
	for _, typ := range recordedSources {
		t.Run(typ.kind, func(t *testing.T) {
			for _, mutation := range []string{"none", "project", "tenant", "id", "source", "type", "revision", "withdrawn", "unknown"} {
				source := productionMaterialSource(testProject)
				source.SourceApplication, source.ResourceType, source.Status = typ.service, typ.kind, "recorded"
				switch mutation {
				case "project":
					source.MaterialSource.ProjectID = testActorID
				case "tenant":
					source.TenantID = testActorID
				case "id":
					source.ID = testActorID
				case "source":
					source.SourceApplication = "other"
				case "type":
					source.ResourceType = "other"
				case "revision":
					source.Revision = 6
				case "withdrawn":
					source.Status = "withdrawn"
				case "unknown":
					source.Status = "unknown"
				}
				store := &storeStub{}
				refs := &materialReferencesStub{values: map[string]ResourceReference{typ.service + "/" + typ.kind + "/" + productionMaterialWorkItem: source}}
				service, _ := NewService(store, refs)
				revision := int64(5)
				_, err := service.ApplyResourceLink(context.Background(), Actor{ID: testActorID, TenantID: testTenant}, testProject, ResourceLinkCommand{CommandID: testCommand, Operation: "link", ExpectedRevision: 9, TargetService: typ.service, ResourceType: typ.kind, ResourceID: productionMaterialWorkItem, RelationKey: "source-materials", SourceRevision: &revision})
				if mutation == "none" {
					if err != nil || store.resourceCalls != 1 {
						t.Fatalf("valid link: %v writes=%d", err, store.resourceCalls)
					}
				} else if !errors.Is(err, ErrReferenceInvalid) || store.resourceCalls != 0 {
					t.Fatalf("%s wrong link: %v writes=%d", mutation, err, store.resourceCalls)
				}
			}
		})
	}
	if ValidResource("cost-outsourcing", "approval_audit_result", "files") || ValidResource("approval-audit-results", "cost_entry", "files") {
		t.Fatal("cross-owner resource type accepted")
	}
}
func TestRecordedMaterialCompositionPreservesStatusDriftAndLocalFailures(t *testing.T) {
	for _, typ := range recordedSources {
		t.Run(typ.kind, func(t *testing.T) {
			for _, mode := range []string{"available", "withdrawn", "empty", "empty-drift", "source-drift", "file-drift", "source-forbidden", "file-unavailable", "wrong-project", "unlinked"} {
				source := productionMaterialSource(testProject)
				source.SourceApplication, source.ResourceType, source.Status = typ.service, typ.kind, "recorded"
				link := productionMaterialLink()
				link.TargetService, link.ResourceType = typ.service, typ.kind
				sourceKey := typ.service + "/" + typ.kind + "/" + productionMaterialWorkItem
				fileKey := "evidence-service/evidence-file/" + productionMaterialFile
				refs := &materialReferencesStub{values: map[string]ResourceReference{fileKey: {ID: productionMaterialFile, TenantID: testTenant, SourceApplication: "evidence-service", ResourceType: "evidence-file", Status: "available", Revision: 2, Evidence: &EvidenceFileFacts{Version: 2, FileName: "资料.pdf", SizeBytes: 64, MediaType: "application/pdf", SHA256: strings.Repeat("a", 64)}}}, errors: map[string]error{}}
				expected, calls := MaterialSegmentAvailable, 2
				switch mode {
				case "withdrawn":
					source.Status = "withdrawn"
					expected, calls = MaterialSegmentInvalid, 1
				case "empty":
					source.MaterialSource.Items = nil
					expected, calls = MaterialSegmentEmpty, 1
				case "empty-drift":
					source.MaterialSource.Items = nil
					source.Revision = 6
					expected, calls = MaterialSegmentRevisionChanged, 1
				case "source-drift":
					source.Revision = 6
					expected = MaterialSegmentRevisionChanged
				case "file-drift":
					f := refs.values[fileKey]
					f.Revision, f.Evidence.Version = 3, 3
					refs.values[fileKey] = f
					expected = MaterialSegmentRevisionChanged
				case "source-forbidden":
					refs.errors[sourceKey] = ErrReferenceForbidden
					expected, calls = MaterialSegmentForbidden, 1
				case "file-unavailable":
					refs.errors[fileKey] = ErrReferenceUnavailable
					expected = MaterialSegmentUnavailable
				case "wrong-project":
					source.MaterialSource.ProjectID = testActorID
					expected, calls = MaterialSegmentInvalid, 1
				case "unlinked":
					at := time.Now()
					link.UnlinkedAt = &at
					calls = 0
				}
				refs.values[sourceKey] = source
				store := &storeStub{details: Details{Project: Project{ID: testProject, TenantID: testTenant, Revision: 9}, ResourceLinks: []ResourceLink{link}}}
				service, _ := NewService(store, refs)
				value, err := service.Materials(context.Background(), testTenant, testProject)
				if err != nil || len(refs.calls) != calls || store.resourceCalls != 0 {
					t.Fatalf("%s calls=%v writes=%d err=%v", mode, refs.calls, store.resourceCalls, err)
				}
				if mode == "unlinked" {
					if len(value.Segments) != 0 || value.State != MaterialCollectionEmpty {
						t.Fatal("unlinked source included")
					}
					continue
				}
				if len(value.Segments) != 1 || value.Segments[0].State != expected {
					t.Fatalf("%s collection=%#v", mode, value)
				}
				if mode == "withdrawn" && (value.Segments[0].ErrorCode != "source_withdrawn" || value.Segments[0].SourceRevision == nil || value.Segments[0].SourceDigest == "" || value.Segments[0].SourceStatus != "withdrawn" || value.Segments[0].EvidenceFileID != "") {
					t.Fatal("withdrawn source silently became available or lost explanation")
				}
			}
		})
	}
}
