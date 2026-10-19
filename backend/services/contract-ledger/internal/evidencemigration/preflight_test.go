package evidencemigration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const testTenantID = "33333333-3333-4333-8333-333333333333"

func TestEvaluateProducesDeterministicMappingAndKeepsMissingExplicit(t *testing.T) {
	root := t.TempDir()
	key := "11111111-1111-4111-8111-111111111111"
	content := []byte("legacy contract attachment")
	digest := sha256.Sum256(content)
	directory := filepath.Join(root, "objects", key[:2])
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, key), content, 0o600); err != nil {
		t.Fatal(err)
	}
	versions := []LegacyVersion{
		{ContractID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", DocumentID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", VersionID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", Version: 2, OriginalFilename: "missing.pdf", MediaType: "application/pdf", SizeBytes: 12, SHA256: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", ContentStatus: "missing"},
		{ContractID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", DocumentID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", VersionID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", Version: 1, OriginalFilename: "source.txt", MediaType: "text/plain; charset=utf-8", SizeBytes: int64(len(content)), SHA256: hex.EncodeToString(digest[:]), StorageProvider: "local", StorageKey: key, ContentStatus: "available"},
	}

	first, err := Evaluate(context.Background(), root, testTenantID, versions)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Evaluate(context.Background(), root, testTenantID, versions)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("preflight is not deterministic\nfirst=%#v\nsecond=%#v", first, second)
	}
	if !first.Ready || first.Counts.Documents != 1 || first.Counts.Versions != 2 || first.Counts.Available != 1 || first.Counts.Missing != 1 || first.Counts.Migratable != 1 || first.Counts.Blocked != 0 {
		t.Fatalf("unexpected report: %#v", first)
	}
	if got := first.Mappings[0].ProposedEvidenceFileID; got != versions[1].VersionID {
		t.Fatalf("proposed file ID = %q", got)
	}
	if first.Mappings[1].Migratable || first.Mappings[1].ProposedEvidenceFileID != "" {
		t.Fatalf("missing content must not be presented as migratable: %#v", first.Mappings[1])
	}
}

func TestEvaluateBlocksSizeDigestAndSequenceMismatch(t *testing.T) {
	root := t.TempDir()
	key := "11111111-1111-4111-8111-111111111111"
	directory := filepath.Join(root, "objects", key[:2])
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, key), []byte("actual"), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := Evaluate(context.Background(), root, testTenantID, []LegacyVersion{{
		ContractID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", DocumentID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		VersionID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", Version: 2,
		OriginalFilename: "bad.txt", MediaType: "text/plain", SizeBytes: 100,
		SHA256:          "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		StorageProvider: "local", StorageKey: key, ContentStatus: "available",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Ready || report.Counts.Blocked != 1 || len(report.Issues) != 3 {
		t.Fatalf("unexpected blocked report: %#v", report)
	}
	want := []string{"version_sequence_gap", "content_size_mismatch", "content_sha256_mismatch"}
	for index, code := range want {
		if report.Issues[index].Code != code {
			t.Fatalf("issue %d = %q, want %q", index, report.Issues[index].Code, code)
		}
	}
}

func TestEvaluateRejectsFilesystemRoot(t *testing.T) {
	if _, err := Evaluate(context.Background(), string(filepath.Separator), testTenantID, nil); err == nil {
		t.Fatal("expected filesystem root to be rejected")
	}
}
