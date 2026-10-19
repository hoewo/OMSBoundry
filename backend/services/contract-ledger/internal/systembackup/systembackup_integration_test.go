//go:build integration

package systembackup

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/contract-ledger/internal/contract"
	"omsbizfoundry/contract-ledger/internal/database"
	"omsbizfoundry/contract-ledger/internal/evidence"
	"omsbizfoundry/contract-ledger/internal/identity"
)

func TestFullBackupRestoreWithPostgreSQLAndEvidence(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	admin, config := openBackupTestAdmin(t, ctx, databaseURL)
	defer admin.Close(ctx)
	sourceName := "cl_backup_source_" + randomHex(t, 4)
	targetName := "cl_backup_target_" + randomHex(t, 4)
	failureName := "cl_backup_failure_" + randomHex(t, 4)
	createBackupTestDatabase(t, ctx, admin, sourceName)
	createBackupTestDatabase(t, ctx, admin, targetName)
	createBackupTestDatabase(t, ctx, admin, failureName)
	defer func() {
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer dropCancel()
		dropBackupTestDatabase(t, dropCtx, admin, failureName)
		dropBackupTestDatabase(t, dropCtx, admin, targetName)
		dropBackupTestDatabase(t, dropCtx, admin, sourceName)
	}()

	sourcePool := openBackupTestPool(t, ctx, config, sourceName)
	defer sourcePool.Close()
	if err := database.Migrate(ctx, sourcePool); err != nil {
		t.Fatalf("migrate source: %v", err)
	}
	identityService, err := identity.NewService(identity.NewPostgresRepository(sourcePool), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if created, err := identityService.Bootstrap(ctx, identity.BootstrapInput{
		Username: "backup-admin", DisplayName: "Backup Admin", Password: "backup-password-123",
	}); err != nil || !created {
		t.Fatalf("bootstrap source identity created=%v error=%v", created, err)
	}
	login, err := identityService.Login(ctx, "backup-admin", "backup-password-123")
	if err != nil {
		t.Fatalf("create source session: %v", err)
	}
	commandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatal(err)
	}
	contractService := contract.NewService(contract.NewPostgresRepository(sourcePool))
	draft, err := contractService.CreateDraft(ctx, contract.CreateDraftInput{
		CommandID: commandID, NumberNamespace: "backup-test", ContractNumber: "BK-001",
		Title: "Backup integration contract", Currency: "CNY", TotalAmount: "123.45",
		Actor: login.Principal.Actor(),
	})
	if err != nil {
		t.Fatalf("create source contract: %v", err)
	}
	sourceEvidenceRoot := filepath.Join(t.TempDir(), "source-evidence")
	store, err := evidence.NewLocalStore(sourceEvidenceRoot)
	if err != nil {
		t.Fatal(err)
	}
	evidenceService, err := evidence.NewService(evidence.NewPostgresRepository(sourcePool), store, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	evidenceCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatal(err)
	}
	content := "evidence bytes preserved by full backup"
	upload, err := evidenceService.Create(ctx, evidence.CreateInput{
		CommandID: evidenceCommandID, ContractID: draft.ID, TargetType: string(evidence.TargetContract),
		TargetID: draft.ID, Title: "Backup proof", Purpose: "Verify backup bytes",
		OriginalFilename: "proof.txt", Actor: login.Principal.Actor(),
	}, strings.NewReader(content))
	if err != nil {
		t.Fatalf("create source evidence: %v", err)
	}

	dumpBinary, err := filepath.Abs("../../ops/local-pg-dump")
	if err != nil {
		t.Fatal(err)
	}
	restoreBinary, err := filepath.Abs("../../ops/local-pg-restore")
	if err != nil {
		t.Fatal(err)
	}
	sourceClient, err := NewCommandClient(containerDatabaseURL(config, sourceName), dumpBinary, restoreBinary)
	if err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(t.TempDir(), "contract-ledger.zip")
	manifest, err := Create(ctx, CreateOptions{
		Pool: sourcePool, Client: sourceClient, EvidenceRoot: sourceEvidenceRoot, OutputPath: archivePath,
		Now: func() time.Time { return time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if manifest.Database.Counts.Contracts != 1 || manifest.Database.Counts.AvailableEvidenceVersion != 1 || len(manifest.Evidence.Objects) != 1 {
		t.Fatalf("backup manifest counts = %#v", manifest)
	}
	sourceEvidencePath := filepath.Join(sourceEvidenceRoot, "objects", upload.Version.StorageKey[:2], upload.Version.StorageKey)
	hiddenEvidencePath := sourceEvidencePath + ".hidden-for-test"
	if err := os.Rename(sourceEvidencePath, hiddenEvidencePath); err != nil {
		t.Fatal(err)
	}
	missingOutput := filepath.Join(t.TempDir(), "must-not-exist.zip")
	_, missingErr := Create(ctx, CreateOptions{
		Pool: sourcePool, Client: sourceClient, EvidenceRoot: sourceEvidenceRoot, OutputPath: missingOutput,
	})
	if err := os.Rename(hiddenEvidencePath, sourceEvidencePath); err != nil {
		t.Fatalf("restore source evidence after failure check: %v", err)
	}
	if missingErr == nil || !strings.Contains(missingErr.Error(), "add evidence") {
		t.Fatalf("Create(missing evidence) error = %v", missingErr)
	}
	if _, err := os.Stat(missingOutput); !os.IsNotExist(err) {
		t.Fatalf("missing evidence backup left final output: %v", err)
	}

	targetPool := openBackupTestPool(t, ctx, config, targetName)
	defer targetPool.Close()
	targetClient, err := NewCommandClient(containerDatabaseURL(config, targetName), dumpBinary, restoreBinary)
	if err != nil {
		t.Fatal(err)
	}
	targetEvidenceRoot := filepath.Join(t.TempDir(), "target-evidence")
	restored, err := Restore(ctx, RestoreOptions{
		Pool: targetPool, Client: targetClient, EvidenceRoot: targetEvidenceRoot, InputPath: archivePath,
	})
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if restored.Database.Counts != manifest.Database.Counts {
		t.Fatalf("restored counts = %#v, want %#v", restored.Database.Counts, manifest.Database.Counts)
	}
	var sessions int
	if err := targetPool.QueryRow(ctx, `SELECT count(*) FROM local_sessions`).Scan(&sessions); err != nil || sessions != 0 {
		t.Fatalf("restored sessions=%d error=%v", sessions, err)
	}
	filePath := filepath.Join(targetEvidenceRoot, "objects", upload.Version.StorageKey[:2], upload.Version.StorageKey)
	value, err := os.ReadFile(filePath)
	if err != nil || string(value) != content {
		t.Fatalf("restored evidence=%q error=%v", value, err)
	}
	if err := database.Migrate(ctx, targetPool); err != nil {
		t.Fatalf("replay migrations after restore: %v", err)
	}
	restoredIdentity, err := identity.NewService(identity.NewPostgresRepository(targetPool), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restoredIdentity.Login(ctx, "backup-admin", "backup-password-123"); err != nil {
		t.Fatalf("login with restored identity: %v", err)
	}

	failurePool := openBackupTestPool(t, ctx, config, failureName)
	defer failurePool.Close()
	invalidDump := []byte("not a PostgreSQL custom archive")
	invalidDumpPath := filepath.Join(t.TempDir(), "invalid.dump")
	writeTestFile(t, invalidDumpPath, invalidDump)
	invalidManifest := manifest
	invalidManifest.Database.DumpBytes = int64(len(invalidDump))
	invalidManifest.Database.DumpSHA256 = digest(invalidDump)
	invalidArchivePath := filepath.Join(t.TempDir(), "invalid-backup.zip")
	if err := writeArchive(invalidArchivePath, invalidDumpPath, sourceEvidenceRoot, invalidManifest); err != nil {
		t.Fatalf("write invalid restore fixture: %v", err)
	}
	failureClient, err := NewCommandClient(containerDatabaseURL(config, failureName), dumpBinary, restoreBinary)
	if err != nil {
		t.Fatal(err)
	}
	failureEvidenceRoot := filepath.Join(t.TempDir(), "failure-evidence")
	if _, err := Restore(ctx, RestoreOptions{
		Pool: failurePool, Client: failureClient, EvidenceRoot: failureEvidenceRoot, InputPath: invalidArchivePath,
	}); err == nil || !strings.Contains(err.Error(), "pg_restore") {
		t.Fatalf("Restore(invalid dump) error = %v", err)
	}
	if _, err := os.Stat(failureEvidenceRoot); !os.IsNotExist(err) {
		t.Fatalf("failed restore left evidence root: %v", err)
	}
	var relations int
	if err := failurePool.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
		  AND n.nspname !~ '^pg_toast'
		  AND c.relkind IN ('r', 'p', 'v', 'm', 'S', 'f')
	`).Scan(&relations); err != nil || relations != 0 {
		t.Fatalf("failed restore relations=%d error=%v", relations, err)
	}
}

func openBackupTestAdmin(t *testing.T, ctx context.Context, databaseURL string) (*pgx.Conn, *pgx.ConnConfig) {
	t.Helper()
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	adminConfig := config.Copy()
	adminConfig.Database = "postgres"
	admin, err := pgx.ConnectConfig(ctx, adminConfig)
	if err != nil {
		t.Fatal(err)
	}
	return admin, config
}

func createBackupTestDatabase(t *testing.T, ctx context.Context, admin *pgx.Conn, name string) {
	t.Helper()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()+" TEMPLATE template0"); err != nil {
		t.Fatalf("create database %s: %v", name, err)
	}
}

func dropBackupTestDatabase(t *testing.T, ctx context.Context, admin *pgx.Conn, name string) {
	t.Helper()
	if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
		t.Errorf("drop database %s: %v", name, err)
	}
}

func openBackupTestPool(t *testing.T, ctx context.Context, config *pgx.ConnConfig, name string) *pgxpool.Pool {
	t.Helper()
	poolConfig, err := pgxpool.ParseConfig(config.ConnString())
	if err != nil {
		t.Fatal(err)
	}
	poolConfig.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	return pool
}

func containerDatabaseURL(config *pgx.ConnConfig, databaseName string) string {
	value := &url.URL{
		Scheme: "postgres", Host: "127.0.0.1:5432", Path: "/" + databaseName,
		User: url.UserPassword(config.User, config.Password),
	}
	query := value.Query()
	query.Set("sslmode", "disable")
	value.RawQuery = query.Encode()
	return value.String()
}

func randomHex(t *testing.T, bytes int) string {
	t.Helper()
	value := make([]byte, bytes)
	if _, err := rand.Read(value); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(value)
}
