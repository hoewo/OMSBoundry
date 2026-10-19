package systembackup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestManifestAndLocalObjectValidation(t *testing.T) {
	root := t.TempDir()
	objectName := filepath.Join(root, "tenants", "t", "files", "f", "versions", "1", "s")
	if err := os.MkdirAll(filepath.Dir(objectName), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(objectName, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	objects, err := scanLocalObjects(root)
	if err != nil || len(objects) != 1 || objects[0].Key != "tenants/t/files/f/versions/1/s" {
		t.Fatalf("scanLocalObjects()=%+v error=%v", objects, err)
	}
	manifest := Manifest{Format: Format, Version: FormatVersion, GeneratedAt: time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)}
	manifest.Database.Name = "evidence_service"
	manifest.Database.PostgreSQLVersion = "18.6"
	manifest.Database.SchemaVersion = "000002_file_version_uploads.sql"
	manifest.Database.DumpPath = databaseEntry
	manifest.Database.DumpBytes = 10
	manifest.Database.DumpSHA256 = strings.Repeat("a", 64)
	manifest.Storage.Provider = "local"
	manifest.Storage.Layout = localStorageLayout
	manifest.Storage.Objects = objects
	if err := manifest.validate(); err != nil {
		t.Fatal(err)
	}
	manifest.Storage.Objects[0].Path = "../escape"
	if err := manifest.validate(); err == nil {
		t.Fatal("unsafe object path was accepted")
	}
}

func TestLocalObjectScanRejectsSymlinks(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	if _, err := scanLocalObjects(root); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink scan error=%v", err)
	}
}

func TestPostgreSQLMajor(t *testing.T) {
	for value, expected := range map[string]int{"18.6": 18, "pg_dump (PostgreSQL) 18.6": 18, "pg_restore (PostgreSQL) 17.9": 17} {
		actual, err := postgresqlMajor(value)
		if err != nil || actual != expected {
			t.Fatalf("postgresqlMajor(%q)=%d,%v want %d", value, actual, err, expected)
		}
	}
	if _, err := postgresqlMajor("unknown"); err == nil {
		t.Fatal("invalid PostgreSQL version was accepted")
	}
}

func TestMixedFormatExplicitlySeparatesLocalBytesFromExternalReferences(t *testing.T) {
	manifest := Manifest{Format: mixedFormat, Version: 2, GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	manifest.Database.Name, manifest.Database.SchemaVersion = "evidence", "schema"
	manifest.Database.DumpPath, manifest.Database.DumpBytes, manifest.Database.DumpSHA256 = databaseEntry, 1, strings.Repeat("a", 64)
	manifest.Storage.Provider, manifest.Storage.Layout = "hybrid", mixedStorageLayout
	manifest.Storage.ExternalObjects = []ExternalObject{{Bucket: "test-bucket", Key: "tenants/t/files/f/versions/2/s", SizeBytes: 4, SHA256: strings.Repeat("b", 64)}}
	if err := manifest.validate(); err != nil {
		t.Fatal(err)
	}
	manifest.Version = 1
	if err := manifest.validate(); err == nil {
		t.Fatal("mixed layout accepted as legacy format")
	}
	manifest.Version = 2
	manifest.Storage.ExternalObjects = append(manifest.Storage.ExternalObjects, manifest.Storage.ExternalObjects[0])
	if err := manifest.validate(); err == nil {
		t.Fatal("duplicate external location accepted")
	}
	manifest.Storage.ExternalObjects = manifest.Storage.ExternalObjects[:1]
	manifest.Storage.ExternalObjects[0].Key = "../escape"
	if err := manifest.validate(); err == nil {
		t.Fatal("unsafe external reference accepted")
	}
}

func TestPartitionReferencesRejectsMisconfiguredStorage(t *testing.T) {
	refs := []reference{{Provider: "local", Key: "local", Size: 1, SHA256: strings.Repeat("a", 64)}, {Provider: "aliyun-oss", Bucket: "bucket", Key: "remote", Size: 1, SHA256: strings.Repeat("b", 64)}}
	local, external, err := partitionReferences("hybrid", refs)
	if err != nil || len(local) != 1 || len(external) != 1 {
		t.Fatalf("mixed references: %v", err)
	}
	for _, provider := range []string{"local", "aliyun-oss"} {
		if _, _, err := partitionReferences(provider, refs); err == nil {
			t.Fatal("mixed database accepted by single provider backup")
		}
	}
	refs[0].Bucket = "unexpected"
	if _, _, err := partitionReferences("hybrid", refs); err == nil {
		t.Fatal("local bucket accepted")
	}
	refs[0].Provider = "unknown"
	if _, _, err := partitionReferences("hybrid", refs); err == nil {
		t.Fatal("unknown provider accepted")
	}
}
