//go:build integration

package systembackup

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/evidence-service/internal/database"
	"omsbizfoundry/evidence-service/internal/evidence"
	"omsbizfoundry/evidence-service/internal/storage"
)

func TestFullBackupRestorePreservesDatabaseAndLocalObjects(t *testing.T) {
	testFullBackupRestore(t, "local")
}

func TestMixedBackupRestorePreservesLocalBytesAndOSSReferences(t *testing.T) {
	testFullBackupRestore(t, "hybrid")
}

func testFullBackupRestore(t *testing.T, storageMode string) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	adminConfig, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	adminConfig.Database = "postgres"
	admin, err := pgx.ConnectConfig(ctx, adminConfig)
	if err != nil {
		t.Fatal(err)
	}
	sourceName := "evidence_backup_source_" + randomSuffix(t)
	targetName := "evidence_backup_target_" + randomSuffix(t)
	for _, name := range []string{sourceName, targetName} {
		if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()+" TEMPLATE template0"); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, name := range []string{targetName, sourceName} {
			if _, err := admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
				t.Errorf("drop backup test database %s: %v", name, err)
			}
		}
		_ = admin.Close(context.Background())
	})
	sourceURL := databaseNamed(t, databaseURL, sourceName)
	targetURL := databaseNamed(t, databaseURL, targetName)
	sourcePool := openPool(t, ctx, sourceURL)
	if err := database.Migrate(ctx, sourcePool); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "source-objects")
	provider, err := storage.NewLocalProvider(root, "http://127.0.0.1:8085", []byte(strings.Repeat("k", 32)), 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	service, err := evidence.NewService(sourcePool, provider, 15*time.Minute, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	actor := evidence.Actor{TenantID: "33333333-3333-4333-8333-333333333333", SubjectID: "44444444-4444-4444-8444-444444444444", DisplayName: "Backup Admin", Kind: "template_subject"}
	content := []byte("backup preserves confirmed bytes")
	session, err := service.CreateUploadSession(ctx, actor, "backup-create", evidence.CreateUploadInput{FileName: "proof.txt", SizeBytes: int64(len(content)), MediaType: "text/plain"})
	if err != nil {
		t.Fatal(err)
	}
	intent, _, err := service.GetUploadIntentByGlobalID(ctx, session.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Put(ctx, intent, strings.NewReader(string(content)), int64(len(content))); err != nil {
		t.Fatal(err)
	}
	version, err := service.CompleteUpload(ctx, actor, session.SessionID, "backup-complete", evidence.CompleteUploadInput{ObservedSizeBytes: int64(len(content))})
	if err != nil {
		t.Fatal(err)
	}
	revisionContent := []byte("backup preserves a second immutable version")
	revisionSession, err := service.CreateVersionUploadSession(ctx, actor, version.FileID, "backup-create-version-2", evidence.CreateUploadInput{FileName: "proof-v2.txt", SizeBytes: int64(len(revisionContent)), MediaType: "text/plain"})
	if err != nil {
		t.Fatal(err)
	}
	revisionIntent, _, err := service.GetUploadIntentByGlobalID(ctx, revisionSession.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Put(ctx, revisionIntent, strings.NewReader(string(revisionContent)), int64(len(revisionContent))); err != nil {
		t.Fatal(err)
	}
	revision, err := service.CompleteUpload(ctx, actor, revisionSession.SessionID, "backup-complete-version-2", evidence.CompleteUploadInput{ObservedSizeBytes: int64(len(revisionContent))})
	if err != nil {
		t.Fatal(err)
	}
	expectedVersions := int64(2)
	if storageMode == "hybrid" {
		remote := &backupTestOSS{contents: []byte("external immutable bytes")}
		hybrid, err := storage.NewHybridProvider("aliyun-oss", provider, remote)
		if err != nil {
			t.Fatal(err)
		}
		mixedService, err := evidence.NewService(sourcePool, hybrid, 15*time.Minute, 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		third, err := mixedService.CreateVersionUploadSession(ctx, actor, version.FileID, "backup-oss-version-3", evidence.CreateUploadInput{FileName: "proof-v3.txt", SizeBytes: int64(len(remote.contents)), MediaType: "text/plain"})
		if err != nil {
			t.Fatal(err)
		}
		confirmed, err := mixedService.CompleteUpload(ctx, actor, third.SessionID, "backup-oss-complete-3", evidence.CompleteUploadInput{ObservedSizeBytes: int64(len(remote.contents))})
		if err != nil || confirmed.Object.Provider != "aliyun-oss" {
			t.Fatalf("mixed version: %v", err)
		}
		expectedVersions = 3
	}
	dumpBinary, _ := filepath.Abs("../../ops/local-pg-dump")
	restoreBinary, _ := filepath.Abs("../../ops/local-pg-restore")
	if value := os.Getenv("TEST_PG_DUMP_BIN"); value != "" {
		dumpBinary = value
	}
	if value := os.Getenv("TEST_PG_RESTORE_BIN"); value != "" {
		restoreBinary = value
	}
	sourceClient, err := NewCommandClient(sourceURL, dumpBinary, restoreBinary)
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "evidence-backup.zip")
	manifest, err := Create(ctx, CreateOptions{Pool: sourcePool, Client: sourceClient, Provider: storageMode, StorageRoot: root, OutputPath: archive})
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Database.Counts.Files != 1 || manifest.Database.Counts.Versions != expectedVersions || len(manifest.Storage.Objects) != 2 {
		t.Fatalf("unexpected backup manifest: %+v", manifest)
	}
	if storageMode == "hybrid" && (manifest.Version != 2 || len(manifest.Storage.ExternalObjects) != 1 || manifest.Storage.ExternalObjects[0].Bucket != "test-bucket") {
		t.Fatal("external reference missing from mixed manifest")
	}
	sourcePool.Close()
	targetPool := openPool(t, ctx, targetURL)
	defer targetPool.Close()
	targetClient, err := NewCommandClient(targetURL, dumpBinary, restoreBinary)
	if err != nil {
		t.Fatal(err)
	}
	targetRoot := filepath.Join(t.TempDir(), "restored-objects")
	restoredManifest, err := Restore(ctx, RestoreOptions{Pool: targetPool, Client: targetClient, Provider: storageMode, StorageRoot: targetRoot, InputPath: archive})
	if err != nil {
		t.Fatal(err)
	}
	if restoredManifest.Database.Counts != manifest.Database.Counts {
		t.Fatalf("restored counts=%+v want %+v", restoredManifest.Database.Counts, manifest.Database.Counts)
	}
	value, err := os.ReadFile(filepath.Join(targetRoot, filepath.FromSlash(version.Object.ObjectKey)))
	if err != nil || string(value) != string(content) {
		t.Fatalf("restored object=%q error=%v", value, err)
	}
	revisionValue, err := os.ReadFile(filepath.Join(targetRoot, filepath.FromSlash(revision.Object.ObjectKey)))
	if err != nil || string(revisionValue) != string(revisionContent) {
		t.Fatalf("restored revision object=%q error=%v", revisionValue, err)
	}
	rows, err := targetPool.Query(ctx, `SELECT version,sha256 FROM file_versions WHERE file_id=$1 ORDER BY version`, version.FileID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var restored []struct {
		version int
		digest  string
	}
	for rows.Next() {
		var item struct {
			version int
			digest  string
		}
		if err := rows.Scan(&item.version, &item.digest); err != nil {
			t.Fatal(err)
		}
		restored = append(restored, item)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if int64(len(restored)) != expectedVersions || restored[0].version != 1 || restored[0].digest != version.SHA256 || restored[1].version != 2 || restored[1].digest != revision.SHA256 {
		t.Fatalf("restored versions=%+v", restored)
	}
}

// This integration fixture tests database/backup mechanics, not cloud access.
type backupTestOSS struct{ contents []byte }

func (p *backupTestOSS) Name() string                { return "aliyun-oss" }
func (p *backupTestOSS) BucketName() string          { return "test-bucket" }
func (p *backupTestOSS) Ready(context.Context) error { return nil }
func (p *backupTestOSS) AuthorizeUpload(context.Context, storage.UploadIntent) (storage.UploadAuthorization, error) {
	return storage.UploadAuthorization{Provider: p.Name()}, nil
}
func (p *backupTestOSS) Open(context.Context, storage.ObjectRef) (storage.ObjectReader, error) {
	return storage.ObjectReader{Reader: io.NopCloser(strings.NewReader(string(p.contents))), Metadata: storage.ObjectMetadata{SizeBytes: int64(len(p.contents))}}, nil
}
func (p *backupTestOSS) AuthorizeAccess(context.Context, storage.AccessIntent) (storage.AccessGrant, error) {
	return storage.AccessGrant{}, nil
}
func (p *backupTestOSS) Delete(context.Context, storage.ObjectRef) error { return nil }

func openPool(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	return pool
}
func databaseNamed(t *testing.T, raw, name string) string {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Path = "/" + name
	return parsed.String()
}
func randomSuffix(t *testing.T) string {
	t.Helper()
	value := make([]byte, 4)
	if _, err := rand.Read(value); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(value)
}
