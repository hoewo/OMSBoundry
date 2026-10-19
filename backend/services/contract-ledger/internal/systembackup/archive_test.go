package systembackup

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteAndValidateArchive(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	dump := []byte("custom database dump bytes")
	evidence := []byte("immutable evidence bytes")
	dumpPath := filepath.Join(directory, "source.dump")
	evidenceRoot := filepath.Join(directory, "evidence")
	storageKey := "11111111-1111-4111-8111-111111111111"
	evidencePath := filepath.Join(evidenceRoot, "objects", "11", storageKey)
	writeTestFile(t, dumpPath, dump)
	writeTestFile(t, evidencePath, evidence)
	manifest := testManifest(dump, storageKey, evidence)
	archivePath := filepath.Join(directory, "backup.zip")
	if err := writeArchive(archivePath, dumpPath, evidenceRoot, manifest); err != nil {
		t.Fatalf("writeArchive() error = %v", err)
	}
	archive, entries, decoded, err := openAndValidateArchive(archivePath)
	if err != nil {
		t.Fatalf("openAndValidateArchive() error = %v", err)
	}
	defer archive.Close()
	if decoded.Format != Format || len(entries) != 3 || entries[databaseEntry] == nil || entries[manifestEntry] == nil {
		t.Fatalf("decoded archive = %#v entries=%v", decoded, entries)
	}
}

func TestWriteArchiveRejectsChangedEvidence(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	dump := []byte("dump")
	original := []byte("original")
	storageKey := "22222222-2222-4222-8222-222222222222"
	dumpPath := filepath.Join(directory, "source.dump")
	evidenceRoot := filepath.Join(directory, "evidence")
	evidencePath := filepath.Join(evidenceRoot, "objects", "22", storageKey)
	writeTestFile(t, dumpPath, dump)
	writeTestFile(t, evidencePath, []byte("changed"))
	manifest := testManifest(dump, storageKey, original)
	archivePath := filepath.Join(directory, "backup.zip")
	if err := writeArchive(archivePath, dumpPath, evidenceRoot, manifest); err == nil || !strings.Contains(err.Error(), "integrity mismatch") {
		t.Fatalf("writeArchive() error = %v", err)
	}
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Fatalf("failed archive exists: %v", err)
	}
}

func TestOpenArchiveRejectsUnsafeOrUnknownEntries(t *testing.T) {
	t.Parallel()
	manifest := testManifest([]byte("dump"), "", nil)
	manifest.Database.Counts.AvailableEvidenceVersion = 0
	manifest.Database.Counts.EvidenceVersions = 0
	manifest.Evidence.Objects = nil
	manifestBytes, err := marshalManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		entries []testZIPEntry
		want    string
	}{
		{
			name: "path traversal",
			entries: []testZIPEntry{
				{name: databaseEntry, value: []byte("dump")}, {name: manifestEntry, value: manifestBytes}, {name: "../secret", value: []byte("x")},
			},
			want: "unsafe backup ZIP path",
		},
		{
			name: "unknown",
			entries: []testZIPEntry{
				{name: databaseEntry, value: []byte("dump")}, {name: manifestEntry, value: manifestBytes}, {name: "extra.txt", value: []byte("x")},
			},
			want: "missing or unknown",
		},
		{
			name: "compressed",
			entries: []testZIPEntry{
				{name: databaseEntry, value: []byte("dump"), method: zip.Deflate}, {name: manifestEntry, value: manifestBytes},
			},
			want: "store method",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			archivePath := filepath.Join(t.TempDir(), "backup.zip")
			writeTestZIP(t, archivePath, test.entries)
			archive, _, _, err := openAndValidateArchive(archivePath)
			if archive != nil {
				archive.Close()
			}
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("openAndValidateArchive() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestDecodeManifestRejectsUnknownFieldsAndUnsortedEvidence(t *testing.T) {
	t.Parallel()
	manifest := testManifest([]byte("dump"), "", nil)
	manifest.Database.Counts.AvailableEvidenceVersion = 0
	manifest.Database.Counts.EvidenceVersions = 0
	manifest.Evidence.Objects = nil
	value, err := marshalManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	withUnknown := strings.Replace(string(value), `"version": 1,`, `"version": 1, "unknown": true,`, 1)
	if _, err := decodeManifest([]byte(withUnknown)); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("decodeManifest(unknown) error = %v", err)
	}

	one := newEvidenceObject("33333333-3333-4333-8333-333333333333", 1, strings.Repeat("a", 64))
	two := newEvidenceObject("22222222-2222-4222-8222-222222222222", 1, strings.Repeat("b", 64))
	manifest.Evidence.Objects = []EvidenceObject{one, two}
	manifest.Database.Counts.EvidenceVersions = 2
	manifest.Database.Counts.AvailableEvidenceVersion = 2
	if err := manifest.validate(); err == nil || !strings.Contains(err.Error(), "sorted") {
		t.Fatalf("validate(unsorted) error = %v", err)
	}
}

func TestPublishDoesNotOverwriteAndCleanupRequiresMatchingMarker(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	temporary := filepath.Join(directory, "temporary.zip")
	output := filepath.Join(directory, "backup.zip")
	writeTestFile(t, temporary, []byte("new"))
	writeTestFile(t, output, []byte("existing"))
	if err := publishWithoutOverwrite(temporary, output); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("publishWithoutOverwrite() error = %v", err)
	}
	value, err := os.ReadFile(output)
	if err != nil || string(value) != "existing" {
		t.Fatalf("existing output = %q error=%v", value, err)
	}

	evidenceRoot := filepath.Join(directory, "restored-evidence")
	writeTestFile(t, filepath.Join(evidenceRoot, restoreMarkerName), []byte("owner-one"))
	writeTestFile(t, filepath.Join(evidenceRoot, "objects", "11", "object"), []byte("content"))
	if err := removeOwnedEvidence(evidenceRoot, "owner-two"); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("removeOwnedEvidence(wrong marker) error = %v", err)
	}
	if _, err := os.Stat(evidenceRoot); err != nil {
		t.Fatalf("evidence root was removed with wrong marker: %v", err)
	}
	if err := removeOwnedEvidence(evidenceRoot, "owner-one"); err != nil {
		t.Fatalf("removeOwnedEvidence(correct marker) error = %v", err)
	}
	if _, err := os.Stat(evidenceRoot); !os.IsNotExist(err) {
		t.Fatalf("owned evidence root still exists: %v", err)
	}
}

func TestPostgreSQLMajorVersionParsing(t *testing.T) {
	t.Parallel()
	for value, expected := range map[string]int{
		"18.6":                         18,
		"pg_dump (PostgreSQL) 18.6":    18,
		"pg_restore (PostgreSQL) 17.9": 17,
	} {
		actual, err := postgresqlMajor(value)
		if err != nil || actual != expected {
			t.Fatalf("postgresqlMajor(%q) = %d, %v; want %d", value, actual, err, expected)
		}
	}
	if _, err := postgresqlMajor("unknown"); err == nil {
		t.Fatal("postgresqlMajor(unknown) succeeded")
	}
}

func testManifest(dump []byte, storageKey string, evidence []byte) Manifest {
	manifest := Manifest{
		Format: Format, Version: FormatVersion, GeneratedAt: time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		Database: DatabaseManifest{
			Name: "contract_ledger", PostgreSQLVersion: "18.6", SchemaVersion: "000016.sql",
			DumpPath: databaseEntry, DumpBytes: int64(len(dump)), DumpSHA256: digest(dump), SessionsExcluded: true,
		},
		Evidence: EvidenceManifest{Layout: "local-store-v1"},
	}
	if storageKey != "" {
		manifest.Database.Counts.EvidenceVersions = 1
		manifest.Database.Counts.AvailableEvidenceVersion = 1
		manifest.Evidence.Objects = []EvidenceObject{newEvidenceObject(storageKey, int64(len(evidence)), digest(evidence))}
	}
	return manifest
}

func digest(value []byte) string {
	hash := sha256.Sum256(value)
	return hex.EncodeToString(hash[:])
}

func writeTestFile(t *testing.T, name string, value []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, value, 0o600); err != nil {
		t.Fatal(err)
	}
}

type testZIPEntry struct {
	name   string
	value  []byte
	method uint16
}

func writeTestZIP(t *testing.T, name string, entries []testZIPEntry) {
	t.Helper()
	file, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for _, value := range entries {
		method := value.method
		if method == 0 {
			method = zip.Store
		}
		header := &zip.FileHeader{Name: value.name, Method: method}
		header.SetMode(0o600)
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(value.value); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
