package partymigration

import "testing"

func TestNormalizeRuntimeTransitionRequest(t *testing.T) {
	request := RuntimeTransitionRequest{
		Format: RuntimeTransitionRequestFormat, TenantID: testTenantID,
		CommandID: "99999999-9999-4999-8999-999999999999", Action: " CUTOVER ",
		MigrationBatchID: testBatchID, ReviewedBySubjectID: testSubjectID,
		ReviewedByDisplayName: " 迁移审核员 ", Reason: " 完成显式映射后切换 ",
	}
	normalized, err := normalizeRuntimeTransitionRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if normalized.Action != RuntimeActionCutover || normalized.ReviewedByDisplayName != "迁移审核员" || normalized.Reason != "完成显式映射后切换" {
		t.Fatalf("unexpected normalized request: %#v", normalized)
	}
	first, err := runtimeTransitionDigest(normalized)
	if err != nil {
		t.Fatal(err)
	}
	second, err := runtimeTransitionDigest(normalized)
	if err != nil || first != second || len(first) != 64 {
		t.Fatalf("runtime transition digest is not deterministic: %q %q %v", first, second, err)
	}
}

func TestRuntimeMappingsDigestChangesWithReviewedSet(t *testing.T) {
	first := []runtimeMapping{{LocalPartyID: testLocalID, LocalPartyRevision: 1, LocalSnapshotSHA256: "digest", Target: TargetSnapshot{PartyID: testTargetID, Revision: 3, Kind: "organization", LegalName: "目标公司"}}}
	left, err := runtimeMappingsDigest(first)
	if err != nil {
		t.Fatal(err)
	}
	first[0].Target.Revision = 4
	right, err := runtimeMappingsDigest(first)
	if err != nil {
		t.Fatal(err)
	}
	if left == right {
		t.Fatal("mapping digest did not change with the reviewed target revision")
	}
}

func TestNormalizeRuntimeTransitionRequestRejectsUnknownAction(t *testing.T) {
	request := RuntimeTransitionRequest{
		Format: RuntimeTransitionRequestFormat, TenantID: testTenantID,
		CommandID: "99999999-9999-4999-8999-999999999999", Action: "merge",
		MigrationBatchID: testBatchID, ReviewedBySubjectID: testSubjectID,
		ReviewedByDisplayName: "迁移审核员", Reason: "不允许隐式合并",
	}
	if _, err := normalizeRuntimeTransitionRequest(request); err == nil {
		t.Fatal("unknown runtime transition action was accepted")
	}
}
