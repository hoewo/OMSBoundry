package engagement

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// PostgreSQL scans time.Local whereas JSON replays UTC. Compare the complete
// public wire value, not private time.Location pointers for the same instant.
func equalMaterialWire(t *testing.T, left, right any) bool {
	t.Helper()
	a, err := json.Marshal(left)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(right)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Equal(a, b)
}

func TestMaterialCollectionNormalizationAndLegacyHash(t *testing.T) {
	legacy := MaterialInput{CommandID: testCommand, ExpectedRevision: 3, Operation: "put", RequirementKey: RequirementBidDocument, EvidenceFileID: testFile, EvidenceVersion: 1, EvidenceSourceRevision: 2}
	normalized, err := NormalizeMaterial(legacy)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"command_id":"33333333-3333-4333-8333-333333333333","expected_revision":3,"operation":"put","requirement_key":"bid_document","evidence_file_id":"66666666-6666-4666-8666-666666666666","evidence_version":1,"evidence_source_revision":2}`
	raw, _ := json.Marshal(normalized)
	if string(raw) != want {
		t.Fatalf("legacy hash input changed: %s", raw)
	}
	legacy.Operation = "remove"
	legacy.EvidenceFileID, legacy.EvidenceVersion, legacy.EvidenceSourceRevision = "", 0, 0
	raw, _ = json.Marshal(legacy)
	if strings.Contains(string(raw), "material_reference_id") {
		t.Fatalf("legacy remove gained hash field: %s", raw)
	}
	for _, operation := range []string{"put", "append"} {
		input := normalized
		input.Operation = operation
		if _, err := NormalizeMaterial(input); err != nil {
			t.Fatalf("%s: %v", operation, err)
		}
		input.MaterialReferenceID = testEngagement
		if _, err := NormalizeMaterial(input); err == nil {
			t.Fatal("write accepted removal target")
		}
	}
	one := legacy
	one.Operation = "remove_one"
	one.MaterialReferenceID = "  " + strings.ToUpper(testEngagement) + "  "
	if value, err := NormalizeMaterial(one); err != nil || value.MaterialReferenceID != testEngagement {
		t.Fatalf("remove_one=%#v err=%v", value, err)
	}
	for _, change := range []func(*MaterialInput){
		func(i *MaterialInput) { i.MaterialReferenceID = "" },
		func(i *MaterialInput) { i.MaterialReferenceID = "bad" },
		func(i *MaterialInput) { i.EvidenceFileID = testFile },
		func(i *MaterialInput) { i.EvidenceVersion = 1 },
		func(i *MaterialInput) { i.EvidenceSourceRevision = 1 },
		func(i *MaterialInput) { i.Operation = "remove" },
	} {
		input := one
		change(&input)
		if _, err := NormalizeMaterial(input); err == nil {
			t.Fatalf("invalid input accepted: %#v", input)
		}
	}
}

func TestMaterialCollectionDetailsKeepAllCurrentAndLatestLegacyRepresentative(t *testing.T) {
	method := MethodComparison
	materials := []MaterialReference{
		{ID: "b", RequirementKey: RequirementBidDocument, EvidenceFileID: testFile, CreatedEngagementRevision: 5},
		{ID: "old", RequirementKey: RequirementBidDocument, CreatedEngagementRevision: 2, SupersededAt: ptrTime()},
		{ID: "a", RequirementKey: RequirementBidDocument, EvidenceFileID: testProject, CreatedEngagementRevision: 4},
	}
	value := Engagement{Applicability: ApplicabilityApplicable, Method: &method}
	detail := BuildDetails(value, materials)
	if len(detail.Requirements[1].MaterialReferences) != 2 || detail.Requirements[1].MaterialReferences[0].ID != "a" || detail.Requirements[1].MaterialReference.ID != "b" || len(detail.Materials) != 3 {
		t.Fatalf("detail=%#v", detail)
	}
	if detail.Requirements[0].MaterialReference != nil || !detail.Requirements[0].Satisfied || detail.Requirements[0].Required {
		t.Fatal("optional is not registered")
	}
	materials[0], materials[2] = materials[2], materials[0]
	if !reflect.DeepEqual(BuildDetails(value, materials).Requirements, detail.Requirements) {
		t.Fatal("representative depends on query order")
	}
}

func TestMaterialCollectionGuardsDuplicatesTargetsAndConsumerLimit(t *testing.T) {
	items := []MaterialReference{{ID: testEngagement, RequirementKey: RequirementBidDocument, EvidenceFileID: testFile, EvidenceVersion: 1}}
	input := MaterialInput{Operation: "append", RequirementKey: RequirementBidDocument, EvidenceFileID: testFile, EvidenceVersion: 1}
	if !errors.Is(validateMaterialChange(input, items), ErrStateConflict) {
		t.Fatal("duplicate accepted")
	}
	input.EvidenceVersion = 2
	if err := validateMaterialChange(input, items); err != nil {
		t.Fatal(err)
	}
	input.Operation, input.MaterialReferenceID = "remove_one", testEngagement
	if err := validateMaterialChange(input, items); err != nil {
		t.Fatal(err)
	}
	input.RequirementKey = RequirementAwardDocument
	if !errors.Is(validateMaterialChange(input, items), ErrStateConflict) {
		t.Fatal("wrong kind accepted")
	}
	input.RequirementKey, input.MaterialReferenceID = RequirementBidDocument, testProject
	if !errors.Is(validateMaterialChange(input, items), ErrStateConflict) {
		t.Fatal("foreign reference accepted")
	}
	input.MaterialReferenceID = testEngagement
	items[0].SupersededAt = ptrTime()
	if !errors.Is(validateMaterialChange(input, items), ErrStateConflict) {
		t.Fatal("old reference accepted")
	}
	items = make([]MaterialReference, MaxActiveMaterials)
	for i := range items {
		items[i].RequirementKey = RequirementBidDocument
	}
	input = MaterialInput{Operation: "append", RequirementKey: RequirementAwardDocument, EvidenceFileID: testFile, EvidenceVersion: 1}
	if !errors.Is(validateMaterialChange(input, items), ErrStateConflict) {
		t.Fatal("101 references accepted")
	}
	input.Operation = "put"
	if !errors.Is(validateMaterialChange(input, items), ErrStateConflict) {
		t.Fatal("new kind exceeds bound")
	}
	input.RequirementKey = RequirementBidDocument
	if err := validateMaterialChange(input, items); err != nil {
		t.Fatal("legacy replacement must free its old members", err)
	}
}

func TestMaterialAppendValidatesEvidenceAndReplaysBeforeDependency(t *testing.T) {
	input := MaterialInput{CommandID: testCommand, ExpectedRevision: 3, Operation: "append", RequirementKey: RequirementBidDocument, EvidenceFileID: testFile, EvidenceVersion: 1, EvidenceSourceRevision: 1}
	store := &fakeStore{}
	refs := &fakeReferences{err: ErrReferenceUnavailable}
	service, _ := NewService(store, refs)
	if _, err := service.ApplyMaterial(context.Background(), Actor{TenantID: testTenant}, testEngagement, input); !errors.Is(err, ErrReferenceUnavailable) || store.material.CommandID != "" || refs.evidenceCalls != 1 {
		t.Fatalf("err=%v store=%#v refs=%#v", err, store, refs)
	}
	store.replayFound = true
	refs.evidenceCalls = 0
	if _, err := service.ApplyMaterial(context.Background(), Actor{TenantID: testTenant}, testEngagement, input); err != nil || refs.evidenceCalls != 0 {
		t.Fatalf("replay err=%v refs=%#v", err, refs)
	}
	store.replayFound = false
	input.Operation, input.MaterialReferenceID, input.EvidenceFileID, input.EvidenceVersion, input.EvidenceSourceRevision = "remove_one", testProject, "", 0, 0
	if _, err := service.ApplyMaterial(context.Background(), Actor{TenantID: testTenant}, testEngagement, input); err != nil || refs.evidenceCalls != 0 || store.material.MaterialReferenceID != testProject {
		t.Fatalf("remove_one read Evidence: err=%v refs=%#v", err, refs)
	}
}
