package systembackup

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	maxManifestBytes     = 1 << 20
	maxArchiveEntries    = 2
	maxArchiveBytes      = int64(256) << 30
	maxTableCountEntries = 10_000
)

type RestoreOptions struct {
	Pool      *pgxpool.Pool
	Client    PostgreSQLClient
	InputPath string
}

func Restore(ctx context.Context, options RestoreOptions) (Manifest, error) {
	if options.Pool == nil {
		return Manifest{}, fmt.Errorf("database pool is required")
	}
	if options.Client == nil {
		return Manifest{}, fmt.Errorf("PostgreSQL client is required")
	}
	inputPath, err := safeAbsolutePath("backup input", options.InputPath)
	if err != nil {
		return Manifest{}, err
	}
	archive, entries, manifest, err := openAndValidateArchive(inputPath)
	if err != nil {
		return Manifest{}, err
	}
	defer archive.Close()

	targetServerVersion, err := validateEmptyTarget(ctx, options.Pool, options.Client.DatabaseName())
	if err != nil {
		return Manifest{}, err
	}
	workspace, err := os.MkdirTemp("", ".money-flow-ledger-restore-*")
	if err != nil {
		return Manifest{}, fmt.Errorf("create restore workspace: %w", err)
	}
	defer os.RemoveAll(workspace)
	if err := os.Chmod(workspace, 0o700); err != nil {
		return Manifest{}, fmt.Errorf("restrict restore workspace: %w", err)
	}
	dumpPath := filepath.Join(workspace, databaseEntry)
	if err := extractVerifiedEntry(entries[databaseEntry], dumpPath, manifest.Database.DumpBytes, manifest.Database.DumpSHA256); err != nil {
		return Manifest{}, fmt.Errorf("extract database dump: %w", err)
	}
	if err := options.Client.Restore(ctx, dumpPath, manifest.Database.PostgreSQLVersion, targetServerVersion); err != nil {
		return Manifest{}, err
	}
	if err := verifyRestoredTarget(ctx, options.Pool, manifest); err != nil {
		return Manifest{}, fmt.Errorf("database restore committed but verification failed; keep target isolated and recreate an empty target before retrying: %w", err)
	}
	return manifest, nil
}

func openAndValidateArchive(inputPath string) (*os.File, map[string]*zip.File, Manifest, error) {
	archive, err := os.Open(inputPath)
	if err != nil {
		return nil, nil, Manifest{}, fmt.Errorf("open backup archive: %w", err)
	}
	fail := func(cause error) (*os.File, map[string]*zip.File, Manifest, error) {
		_ = archive.Close()
		return nil, nil, Manifest{}, cause
	}
	info, err := archive.Stat()
	if err != nil {
		return fail(fmt.Errorf("inspect backup archive: %w", err))
	}
	if !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maxArchiveBytes {
		return fail(fmt.Errorf("backup archive size or type is invalid"))
	}
	reader, err := zip.NewReader(archive, info.Size())
	if err != nil {
		return fail(fmt.Errorf("open backup ZIP: %w", err))
	}
	if len(reader.File) != maxArchiveEntries {
		return fail(fmt.Errorf("backup ZIP must contain exactly manifest.json and database.dump"))
	}
	entries := make(map[string]*zip.File, len(reader.File))
	for _, entry := range reader.File {
		if err := validateZIPEntry(entry); err != nil {
			return fail(err)
		}
		if _, exists := entries[entry.Name]; exists {
			return fail(fmt.Errorf("duplicate backup ZIP path %q", entry.Name))
		}
		entries[entry.Name] = entry
	}
	manifestFile, exists := entries[manifestEntry]
	if !exists || manifestFile.UncompressedSize64 > maxManifestBytes {
		return fail(fmt.Errorf("backup manifest is missing or too large"))
	}
	manifestBytes, err := readZIPEntry(manifestFile, maxManifestBytes)
	if err != nil {
		return fail(fmt.Errorf("read backup manifest: %w", err))
	}
	manifest, err := decodeManifest(manifestBytes)
	if err != nil {
		return fail(err)
	}
	dump, exists := entries[databaseEntry]
	if !exists || dump.UncompressedSize64 != uint64(manifest.Database.DumpBytes) {
		return fail(fmt.Errorf("database dump size does not match manifest"))
	}
	for name := range entries {
		if name != manifestEntry && name != databaseEntry {
			return fail(fmt.Errorf("unknown backup ZIP entry %q", name))
		}
	}
	return archive, entries, manifest, nil
}

func validateZIPEntry(entry *zip.File) error {
	if entry.Name == "" || strings.Contains(entry.Name, "\\") || strings.HasPrefix(entry.Name, "/") ||
		entry.Name == ".." || strings.HasPrefix(entry.Name, "../") || path.Clean(entry.Name) != entry.Name {
		return fmt.Errorf("unsafe backup ZIP path %q", entry.Name)
	}
	if entry.Flags&0x1 != 0 {
		return fmt.Errorf("encrypted backup ZIP entry %q is not supported", entry.Name)
	}
	if entry.Method != zip.Store {
		return fmt.Errorf("backup ZIP entry %q must use store method", entry.Name)
	}
	if !entry.Mode().IsRegular() {
		return fmt.Errorf("backup ZIP entry %q is not a regular file", entry.Name)
	}
	return nil
}

func decodeManifest(value []byte) (Manifest, error) {
	decoder := json.NewDecoder(strings.NewReader(string(value)))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode backup manifest: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Manifest{}, fmt.Errorf("backup manifest must contain one JSON value")
	}
	if err := manifest.validate(); err != nil {
		return Manifest{}, fmt.Errorf("validate backup manifest: %w", err)
	}
	return manifest, nil
}

func readZIPEntry(entry *zip.File, limit uint64) ([]byte, error) {
	if entry.UncompressedSize64 > limit {
		return nil, fmt.Errorf("ZIP entry exceeds size limit")
	}
	reader, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	value, err := io.ReadAll(io.LimitReader(reader, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if uint64(len(value)) != entry.UncompressedSize64 {
		return nil, fmt.Errorf("ZIP entry length is invalid")
	}
	return value, nil
}

func extractVerifiedEntry(entry *zip.File, destination string, expectedSize int64, expectedSHA256 string) error {
	if entry == nil {
		return fmt.Errorf("archive entry is missing")
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return fmt.Errorf("create extraction directory: %w", err)
	}
	reader, err := entry.Open()
	if err != nil {
		return fmt.Errorf("open archive entry: %w", err)
	}
	defer reader.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create extracted file: %w", err)
	}
	succeeded := false
	defer func() {
		_ = output.Close()
		if !succeeded {
			_ = os.Remove(destination)
		}
	}()
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(output, hash), reader)
	if err != nil {
		return fmt.Errorf("extract archive entry: %w", err)
	}
	if size != expectedSize || hex.EncodeToString(hash.Sum(nil)) != expectedSHA256 {
		return fmt.Errorf("archive entry integrity mismatch")
	}
	if err := output.Sync(); err != nil {
		return fmt.Errorf("sync extracted file: %w", err)
	}
	if err := output.Close(); err != nil {
		return fmt.Errorf("close extracted file: %w", err)
	}
	succeeded = true
	return nil
}

func validateEmptyTarget(ctx context.Context, pool *pgxpool.Pool, clientDatabaseName string) (string, error) {
	var targetDatabaseName string
	var targetServerVersion string
	var relations int64
	if err := pool.QueryRow(ctx, `
		SELECT current_database(), current_setting('server_version'), (
			SELECT count(*)
			FROM pg_class c
			JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
			  AND n.nspname !~ '^pg_toast'
			  AND c.relkind IN ('r', 'p', 'v', 'm', 'S', 'f')
		)
	`).Scan(&targetDatabaseName, &targetServerVersion, &relations); err != nil {
		return "", fmt.Errorf("inspect restore target database: %w", err)
	}
	if targetDatabaseName != clientDatabaseName {
		return "", fmt.Errorf("PostgreSQL client targets database %q, restore connection is %q", clientDatabaseName, targetDatabaseName)
	}
	if relations != 0 {
		return "", fmt.Errorf("restore target database is not empty")
	}
	return targetServerVersion, nil
}

func verifyRestoredTarget(ctx context.Context, pool *pgxpool.Pool, manifest Manifest) error {
	transaction, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return fmt.Errorf("begin restored database verification: %w", err)
	}
	defer transaction.Rollback(context.Background())
	if _, err := transaction.Exec(ctx, `SELECT pg_advisory_xact_lock_shared(hashtext($1))`, moneyFlowMigrationLockKey); err != nil {
		return fmt.Errorf("acquire restored schema lock: %w", err)
	}
	var schemaVersion string
	var sessions int64
	if err := transaction.QueryRow(ctx, `
		SELECT COALESCE((SELECT max(version) FROM schema_migrations), ''),
		       (SELECT count(*) FROM local_sessions)
	`).Scan(&schemaVersion, &sessions); err != nil {
		return fmt.Errorf("query restored database identity: %w", err)
	}
	tables, err := readPublicTableCounts(ctx, transaction)
	if err != nil {
		return fmt.Errorf("query restored table counts: %w", err)
	}
	if schemaVersion != manifest.Database.SchemaVersion {
		return fmt.Errorf("restored schema version does not match manifest")
	}
	if !tableCountsEqual(tables, manifest.Database.Tables) {
		return fmt.Errorf("restored table names or row counts do not match manifest")
	}
	if sessions != 0 {
		return fmt.Errorf("restored database contains local sessions")
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("finish restored database verification: %w", err)
	}
	return nil
}
