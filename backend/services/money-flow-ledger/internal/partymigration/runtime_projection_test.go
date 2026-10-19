package partymigration

import (
	"reflect"
	"testing"
)

func validRuntimeProjectionRequest() RuntimeProjectionRequest {
	return RuntimeProjectionRequest{
		Format: RuntimeProjectionRequestFormat, TenantID: testTenantID,
		CommandID:     "10101010-1010-4010-8010-101010101010",
		TargetPartyID: "20202020-2020-4020-8020-202020202020", TargetPartyRevision: 2,
		ReviewedBySubjectID: testSubjectID, ReviewedByDisplayName: "资金主体审核员",
		Reason: "奖金收款人已按 Party Directory 精确 revision 复核",
	}
}

func TestNormalizeRuntimeProjectionRequestAndDigestAreDeterministic(t *testing.T) {
	request := validRuntimeProjectionRequest()
	request.ReviewedByDisplayName = " 资金主体审核员 "
	request.Reason = " 奖金收款人已按 Party Directory 精确 revision 复核 "
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
	if firstDigest != secondDigest || len(firstDigest) != 64 || !reflect.DeepEqual(first, second) {
		t.Fatalf("runtime projection normalization is not deterministic: %#v / %#v", first, second)
	}
	if deterministicRuntimeProjectionUUID("same") != deterministicRuntimeProjectionUUID("same") || deterministicRuntimeProjectionUUID("same") == deterministicRuntimeProjectionUUID("different") {
		t.Fatal("runtime projection local Party identity is not deterministic")
	}
}

func TestNormalizeRuntimeProjectionRequestRejectsIncompleteReview(t *testing.T) {
	request := validRuntimeProjectionRequest()
	request.Reason = " "
	if _, err := normalizeRuntimeProjectionRequest(request); err == nil {
		t.Fatal("runtime projection without review reason was accepted")
	}
}
