package design

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// Unimplemented embedded ports panic if the read accidentally writes or calls
// an upstream application. Only owned Get is permitted in this fixture.
type materialReferenceStore struct {
	Store
	details Details
	err     error
	calls   int
}

func (s *materialReferenceStore) Get(_ context.Context, tenant, id string, historical bool) (Details, error) {
	s.calls++
	if tenant != workItemTenant || id != workItemID || historical {
		panic("material projection widened its owned query")
	}
	return s.details, s.err
}

type materialReferenceNoUpstream struct{ References }

func materialDetails(typeKey string) Details {
	definition, _ := RegisteredTypeDefinition(typeKey, 1)
	details := Details{WorkItem: WorkItem{ID: workItemID, TenantID: workItemTenant, ProjectID: workItemProject, TypeKey: typeKey, TypeDefinitionVersion: 1, Status: StatusInProgress, Revision: 9}}
	for _, stage := range definition.Stages {
		details.Stages = append(details.Stages, Stage{TenantID: workItemTenant, WorkItemID: workItemID, Key: stage.Key, Sequence: stage.Sequence, Status: StagePending})
	}
	return details
}

func materialAt(stage, file string, version int) Deliverable {
	return Deliverable{ID: workItemAssignment, TenantID: workItemTenant, WorkItemID: workItemID, StageKey: stage, RequirementKey: RequirementPrimary, EvidenceFileID: file, EvidenceVersion: version, EvidenceSourceRevision: 7, DisplaySnapshot: "成果.pdf", CreatedRevision: 5}
}

func TestMaterialReferenceUsesOnlyOwnedFactsAndDeterministicDigest(t *testing.T) {
	for _, typeKey := range []string{DesignTypeKey, ConsultationTypeKey} {
		t.Run(typeKey, func(t *testing.T) {
			details := materialDetails(typeKey)
			last := details.Stages[len(details.Stages)-1].Key
			details.Deliverables = []Deliverable{materialAt("scheme", workItemPerson, 2), materialAt(last, workItemAssignment, 1)}
			before, _ := json.Marshal(details)
			store := &materialReferenceStore{details: details}
			service, _ := NewService(store, &materialReferenceNoUpstream{})
			projection, err := service.ReferenceMaterials(context.Background(), workItemTenant, workItemID)
			if err != nil || store.calls != 1 || projection.SchemaVersion != "1" || projection.SourceApplication != "production-operations" || projection.ResourceType != "production-work-item" || projection.ID != workItemID || projection.TenantID != workItemTenant || projection.ProjectID != workItemProject || projection.Status != StatusInProgress || projection.Revision != 9 || len(projection.Items) != 2 {
				t.Fatalf("projection=%#v calls=%d err=%v", projection, store.calls, err)
			}
			if projection.Items[0].RelationKey > projection.Items[1].RelationKey || projection.Items[0].EvidenceSourceRevision != 7 {
				t.Fatalf("non-canonical items=%#v", projection.Items)
			}
			encoded, _ := json.Marshal(projection.Items)
			digest := sha256.Sum256(encoded)
			if projection.Digest != hex.EncodeToString(digest[:]) {
				t.Fatal("digest does not describe the sorted items")
			}
			after, _ := json.Marshal(store.details)
			if string(before) != string(after) {
				t.Fatal("projection mutated owned details")
			}
			store.details.Deliverables[0], store.details.Deliverables[1] = store.details.Deliverables[1], store.details.Deliverables[0]
			second, err := service.ReferenceMaterials(context.Background(), workItemTenant, workItemID)
			if err != nil || second.Digest != projection.Digest {
				t.Fatalf("input order changed digest: %v", err)
			}
		})
	}
}

func TestMaterialReferenceCurrentOnlyAndExplicitEmpty(t *testing.T) {
	details := materialDetails(DesignTypeKey)
	details.WorkItem.Status = StatusCancelled
	details.Stages[0].Status = StageNotApplicable
	old := materialAt("construction-drawing", workItemPerson, 1)
	supersededAt, supersededRevision := time.Now().UTC(), int64(6)
	old.SupersededAt, old.SupersededRevision = &supersededAt, &supersededRevision
	details.Deliverables = []Deliverable{old, materialAt("scheme", workItemPerson, 1), materialAt("construction-drawing", workItemAssignment, 2)}
	store := &materialReferenceStore{details: details}
	service, _ := NewService(store, &materialReferenceNoUpstream{})
	projection, err := service.ReferenceMaterials(context.Background(), workItemTenant, workItemID)
	if err != nil || projection.Status != StatusCancelled || len(projection.Items) != 1 || projection.Items[0].Version != 2 || projection.Items[0].RelationKey != "construction-drawing.primary-output" {
		t.Fatalf("current projection=%#v err=%v", projection, err)
	}
	// Reopened/pending stages retain current immutable file references.
	store.details.Stages[0].Status = StagePending
	projection, err = service.ReferenceMaterials(context.Background(), workItemTenant, workItemID)
	if err != nil || len(projection.Items) != 2 {
		t.Fatalf("reopened projection=%#v err=%v", projection, err)
	}
	store.details.Deliverables = nil
	projection, err = service.ReferenceMaterials(context.Background(), workItemTenant, workItemID)
	empty, _ := json.Marshal(projection.Items)
	if err != nil || string(empty) != "[]" || projection.Digest != "4f53cda18c2baa0c0354bb5f9a3ecbe5ed12ab4d8e11ba873c2f11161202b945" {
		t.Fatalf("empty projection=%#v err=%v", projection, err)
	}
}

func TestMaterialReferenceRejectsMalformedOwnedFacts(t *testing.T) {
	cases := map[string]func(*Details){
		"wrong tenant":         func(d *Details) { d.WorkItem.TenantID = workItemSubject },
		"wrong item":           func(d *Details) { d.WorkItem.ID = workItemSubject },
		"bad project":          func(d *Details) { d.WorkItem.ProjectID = "not-a-project" },
		"bad revision":         func(d *Details) { d.WorkItem.Revision = 0 },
		"unknown status":       func(d *Details) { d.WorkItem.Status = "unknown" },
		"unknown type":         func(d *Details) { d.WorkItem.TypeKey = "unknown" },
		"unknown version":      func(d *Details) { d.WorkItem.TypeDefinitionVersion = 7 },
		"missing stage":        func(d *Details) { d.Stages = d.Stages[1:] },
		"unknown stage":        func(d *Details) { d.Stages[0].Key = "report" },
		"wrong sequence":       func(d *Details) { d.Stages[0].Sequence = 20 },
		"duplicate stage":      func(d *Details) { d.Stages[1] = d.Stages[0] },
		"stage tenant":         func(d *Details) { d.Stages[0].TenantID = workItemSubject },
		"stage item":           func(d *Details) { d.Stages[0].WorkItemID = workItemSubject },
		"stage status":         func(d *Details) { d.Stages[0].Status = "unknown" },
		"mandatory skipped":    func(d *Details) { d.Stages[2].Status = StageNotApplicable },
		"reference tenant":     func(d *Details) { d.Deliverables[0].TenantID = workItemSubject },
		"reference item":       func(d *Details) { d.Deliverables[0].WorkItemID = workItemSubject },
		"reference stage":      func(d *Details) { d.Deliverables[0].StageKey = "report" },
		"unknown requirement":  func(d *Details) { d.Deliverables[0].RequirementKey = "unknown" },
		"duplicate slot":       func(d *Details) { d.Deliverables = append(d.Deliverables, d.Deliverables[0]) },
		"bad file":             func(d *Details) { d.Deliverables[0].EvidenceFileID = "not-a-file" },
		"bad version":          func(d *Details) { d.Deliverables[0].EvidenceVersion = 0 },
		"bad source revision":  func(d *Details) { d.Deliverables[0].EvidenceSourceRevision = 0 },
		"future reference":     func(d *Details) { d.Deliverables[0].CreatedRevision = 10 },
		"bad created revision": func(d *Details) { d.Deliverables[0].CreatedRevision = 0 },
		"empty display":        func(d *Details) { d.Deliverables[0].DisplaySnapshot = "" },
		"long display":         func(d *Details) { d.Deliverables[0].DisplaySnapshot = strings.Repeat("界", 256) },
		"half superseded":      func(d *Details) { revision := int64(6); d.Deliverables[0].SupersededRevision = &revision },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			details := materialDetails(DesignTypeKey)
			details.Deliverables = []Deliverable{materialAt("scheme", workItemPerson, 1)}
			mutate(&details)
			store := &materialReferenceStore{details: details}
			service, _ := NewService(store, &materialReferenceNoUpstream{})
			projection, err := service.ReferenceMaterials(context.Background(), workItemTenant, workItemID)
			if !errors.Is(err, ErrReferenceInvalid) || projection.Items != nil || projection.Digest != "" {
				t.Fatalf("malformed facts did not fail closed: %#v err=%v", projection, err)
			}
		})
	}
}

func TestMaterialReferencePreservesSourceErrorAndRejectsBadQuery(t *testing.T) {
	store := &materialReferenceStore{err: ErrNotFound}
	service, _ := NewService(store, &materialReferenceNoUpstream{})
	for _, sourceError := range []error{ErrNotFound, ErrReferenceUnavailable, errors.New("database unavailable")} {
		store.err = sourceError
		value, err := service.ReferenceMaterials(context.Background(), workItemTenant, workItemID)
		if !errors.Is(err, sourceError) || value.Items != nil || value.Digest != "" {
			t.Fatalf("source failure became empty success: %#v %v", value, err)
		}
	}
	before := store.calls
	for _, query := range [][2]string{{"bad-tenant", workItemID}, {workItemTenant, "bad-item"}, {workItemTenant, " " + workItemID}} {
		if _, err := service.ReferenceMaterials(context.Background(), query[0], query[1]); !errors.Is(err, ErrReferenceInvalid) {
			t.Fatalf("invalid query err=%v", err)
		}
	}
	if store.calls != before {
		t.Fatal("invalid query reached the database")
	}
}
