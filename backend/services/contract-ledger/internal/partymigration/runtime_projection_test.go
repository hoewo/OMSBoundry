package partymigration

import "testing"

func TestNormalizeRuntimeProjectionRequestAndDigestAreDeterministic(t *testing.T) {
	request := RuntimeProjectionRequest{
		Format: RuntimeProjectionRequestFormat, TenantID: testTenantID,
		CommandID: testCommandID, TargetPartyID: testTargetID, TargetPartyRevision: 3,
		ReviewedBySubjectID: testSubjectID, ReviewedByDisplayName: " 迁移审核员 ", Reason: " 合同主体冻结 ",
	}
	first, err := normalizeRuntimeProjectionRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := normalizeRuntimeProjectionRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	firstDigest, err := runtimeProjectionRequestDigest(first)
	if err != nil {
		t.Fatal(err)
	}
	secondDigest, err := runtimeProjectionRequestDigest(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstDigest != secondDigest || len(firstDigest) != 64 || deterministicRuntimeProjectionUUID("same") != deterministicRuntimeProjectionUUID("same") || deterministicRuntimeProjectionUUID("same") == deterministicRuntimeProjectionUUID("different") {
		t.Fatalf("runtime projection normalization is not deterministic: %#v %#v", first, second)
	}
}

func TestNormalizeRuntimeProjectionRequestRejectsIncompleteReview(t *testing.T) {
	request := RuntimeProjectionRequest{
		Format: RuntimeProjectionRequestFormat, TenantID: testTenantID,
		CommandID: testCommandID, TargetPartyID: testTargetID, TargetPartyRevision: 3,
		ReviewedBySubjectID: testSubjectID, ReviewedByDisplayName: "迁移审核员",
	}
	if _, err := normalizeRuntimeProjectionRequest(request); err == nil {
		t.Fatal("empty review reason was accepted")
	}
}
