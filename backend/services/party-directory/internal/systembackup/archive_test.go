package systembackup

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
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
	dumpPath := filepath.Join(directory, "source.dump")
	writeTestFile(t, dumpPath, dump)
	manifest := testManifest(dump)
	archivePath := filepath.Join(directory, "backup.zip")
	if err := writeArchive(archivePath, dumpPath, manifest); err != nil {
		t.Fatalf("writeArchive() error = %v", err)
	}
	archive, entries, decoded, err := openAndValidateArchive(archivePath)
	if err != nil {
		t.Fatalf("openAndValidateArchive() error = %v", err)
	}
	defer archive.Close()
	if decoded.Format != Format || len(entries) != 2 || entries[databaseEntry] == nil || entries[manifestEntry] == nil {
		t.Fatalf("decoded archive = %#v entries=%v", decoded, entries)
	}
	info, err := os.Stat(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("archive mode = %o, want 600", info.Mode().Perm())
	}
}

func TestWriteArchiveRejectsChangedDump(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	dumpPath := filepath.Join(directory, "source.dump")
	writeTestFile(t, dumpPath, []byte("changed"))
	manifest := testManifest([]byte("original"))
	archivePath := filepath.Join(directory, "backup.zip")
	if err := writeArchive(archivePath, dumpPath, manifest); err == nil || !strings.Contains(err.Error(), "integrity mismatch") {
		t.Fatalf("writeArchive() error = %v", err)
	}
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Fatalf("failed archive exists: %v", err)
	}
}

func TestOpenArchiveRejectsUnsafeUnknownCompressedAndDuplicateEntries(t *testing.T) {
	t.Parallel()
	dump := []byte("dump")
	manifestBytes, err := marshalManifest(testManifest(dump))
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
				{name: "../database.dump", value: dump}, {name: manifestEntry, value: manifestBytes},
			},
			want: "unsafe backup ZIP path",
		},
		{
			name: "unknown dump path",
			entries: []testZIPEntry{
				{name: "other.dump", value: dump}, {name: manifestEntry, value: manifestBytes},
			},
			want: "database dump size",
		},
		{
			name: "compressed",
			entries: []testZIPEntry{
				{name: databaseEntry, value: dump, method: zip.Deflate}, {name: manifestEntry, value: manifestBytes},
			},
			want: "store method",
		},
		{
			name: "duplicate",
			entries: []testZIPEntry{
				{name: databaseEntry, value: dump}, {name: databaseEntry, value: dump},
			},
			want: "duplicate backup ZIP path",
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

func TestDecodeManifestRejectsUnknownFieldsAndInvalidTableCounts(t *testing.T) {
	t.Parallel()
	manifest := testManifest([]byte("dump"))
	value, err := marshalManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	withUnknown := strings.Replace(string(value), `"version": 1,`, `"version": 1, "unknown": true,`, 1)
	if _, err := decodeManifest([]byte(withUnknown)); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("decodeManifest(unknown) error = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Manifest)
		want   string
	}{
		{name: "unsorted", mutate: func(value *Manifest) {
			value.Database.Tables = []TableCount{{Name: "schema_migrations", Rows: 14}, {Name: "app_metadata", Rows: 1}}
		}, want: "uniquely sorted"},
		{name: "missing migrations", mutate: func(value *Manifest) {
			value.Database.Tables = []TableCount{{Name: "app_metadata", Rows: 1}}
		}, want: "do not include schema_migrations"},
		{name: "negative", mutate: func(value *Manifest) { value.Database.Tables[0].Rows = -1 }, want: "table count is invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := manifest
			candidate.Database.Tables = append([]TableCount(nil), manifest.Database.Tables...)
			test.mutate(&candidate)
			if err := candidate.validate(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validate() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestExtractVerifiedEntryRejectsTamperedDump(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	original := []byte("original")
	tampered := []byte("tampered")
	manifest := testManifest(original)
	manifestBytes, err := marshalManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(directory, "tampered.zip")
	writeTestZIP(t, archivePath, []testZIPEntry{
		{name: databaseEntry, value: tampered}, {name: manifestEntry, value: manifestBytes},
	})
	archive, entries, _, err := openAndValidateArchive(archivePath)
	if err != nil {
		t.Fatalf("archive shape validation error = %v", err)
	}
	defer archive.Close()
	if err := extractVerifiedEntry(entries[databaseEntry], filepath.Join(directory, "restored.dump"), int64(len(original)), digest(original)); err == nil || !strings.Contains(err.Error(), "integrity mismatch") {
		t.Fatalf("extractVerifiedEntry() error = %v", err)
	}
}

func TestPublishDoesNotOverwrite(t *testing.T) {
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

func TestCommandClientRejectsClientAndRestoreMajorMismatch(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	versionBinary := filepath.Join(directory, "pg-client")
	if err := os.WriteFile(versionBinary, []byte("#!/bin/sh\necho 'pg_dump (PostgreSQL) 17.9'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	client := &CommandClient{databaseName: "party_directory", environment: os.Environ(), dumpBinary: versionBinary, restoreBinary: versionBinary}
	if err := client.Dump(context.Background(), "snapshot-id", "18.6", io.Discard); err == nil || !strings.Contains(err.Error(), "major 17 does not match") {
		t.Fatalf("Dump(client major mismatch) error = %v", err)
	}
	dumpPath := filepath.Join(directory, "database.dump")
	writeTestFile(t, dumpPath, []byte("dump"))
	if err := client.Restore(context.Background(), dumpPath, "17.9", "18.6"); err == nil || !strings.Contains(err.Error(), "backup PostgreSQL major 17 does not match target major 18") {
		t.Fatalf("Restore(backup major mismatch) error = %v", err)
	}
}

func TestCreateAndRestoreRequireDependencies(t *testing.T) {
	t.Parallel()
	if _, err := Create(t.Context(), CreateOptions{}); err == nil || !strings.Contains(err.Error(), "pool") {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := Restore(t.Context(), RestoreOptions{}); err == nil || !strings.Contains(err.Error(), "pool") {
		t.Fatalf("Restore() error = %v", err)
	}
}

func testManifest(dump []byte) Manifest {
	return Manifest{
		Format: Format, Version: FormatVersion,
		GeneratedAt: time.Date(2026, 9, 20, 2, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		Database: DatabaseManifest{
			Name: "party_directory", PostgreSQLVersion: "18.6", SchemaVersion: "000014.sql",
			DumpPath: databaseEntry, DumpBytes: int64(len(dump)), DumpSHA256: digest(dump),
			Tables: []TableCount{{Name: "app_metadata", Rows: 1}, {Name: "schema_migrations", Rows: 14}},
		},
	}
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
