package systembackup

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CreateOptions struct {
	Pool         *pgxpool.Pool
	Client       PostgreSQLClient
	EvidenceRoot string
	OutputPath   string
	Now          func() time.Time
}

func Create(ctx context.Context, options CreateOptions) (Manifest, error) {
	if options.Pool == nil {
		return Manifest{}, fmt.Errorf("database pool is required")
	}
	if options.Client == nil {
		return Manifest{}, fmt.Errorf("PostgreSQL client is required")
	}
	evidenceRoot, err := safeAbsolutePath("evidence root", options.EvidenceRoot)
	if err != nil {
		return Manifest{}, err
	}
	if err := validateSourceEvidenceRoot(evidenceRoot); err != nil {
		return Manifest{}, err
	}
	outputPath, err := safeAbsolutePath("backup output", options.OutputPath)
	if err != nil {
		return Manifest{}, err
	}
	if info, err := os.Stat(outputPath); err == nil {
		return Manifest{}, fmt.Errorf("backup output already exists: %s", info.Name())
	} else if !os.IsNotExist(err) {
		return Manifest{}, fmt.Errorf("check backup output: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o700); err != nil {
		return Manifest{}, fmt.Errorf("create backup output directory: %w", err)
	}
	temporaryDirectory, err := os.MkdirTemp(filepath.Dir(outputPath), ".contract-ledger-backup-*")
	if err != nil {
		return Manifest{}, fmt.Errorf("create backup temporary directory: %w", err)
	}
	defer os.RemoveAll(temporaryDirectory)
	if err := os.Chmod(temporaryDirectory, 0o700); err != nil {
		return Manifest{}, fmt.Errorf("restrict backup temporary directory: %w", err)
	}

	dumpPath := filepath.Join(temporaryDirectory, databaseEntry)
	dumpFile, err := os.OpenFile(dumpPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return Manifest{}, fmt.Errorf("create temporary database dump: %w", err)
	}

	metadata, snapshot, err := readSnapshotMetadata(ctx, options.Pool)
	if err != nil {
		_ = dumpFile.Close()
		return Manifest{}, err
	}
	if metadata.databaseName != options.Client.DatabaseName() {
		_ = snapshot.Rollback(context.Background())
		_ = dumpFile.Close()
		return Manifest{}, fmt.Errorf("PostgreSQL client targets database %q, snapshot source is %q", options.Client.DatabaseName(), metadata.databaseName)
	}
	if err := options.Client.Dump(ctx, metadata.snapshotID, metadata.postgresqlVersion, dumpFile); err != nil {
		_ = snapshot.Rollback(context.Background())
		_ = dumpFile.Close()
		return Manifest{}, err
	}
	if err := dumpFile.Sync(); err != nil {
		_ = snapshot.Rollback(context.Background())
		_ = dumpFile.Close()
		return Manifest{}, fmt.Errorf("sync database dump: %w", err)
	}
	if err := dumpFile.Close(); err != nil {
		_ = snapshot.Rollback(context.Background())
		return Manifest{}, fmt.Errorf("close database dump: %w", err)
	}
	if err := snapshot.Commit(ctx); err != nil {
		return Manifest{}, fmt.Errorf("finish backup snapshot: %w", err)
	}
	dumpSize, dumpHash, err := hashRegularFile(dumpPath)
	if err != nil {
		return Manifest{}, fmt.Errorf("hash database dump: %w", err)
	}

	now := time.Now
	if options.Now != nil {
		now = options.Now
	}
	manifest := Manifest{
		Format:      Format,
		Version:     FormatVersion,
		GeneratedAt: now().UTC().Format(time.RFC3339Nano),
		Database: DatabaseManifest{
			Name: metadata.databaseName, PostgreSQLVersion: metadata.postgresqlVersion,
			SchemaVersion: metadata.schemaVersion, DumpPath: databaseEntry,
			DumpBytes: dumpSize, DumpSHA256: dumpHash, SessionsExcluded: true,
			Counts: metadata.counts,
		},
		Evidence: EvidenceManifest{Layout: "local-store-v1", Objects: metadata.evidence},
	}
	manifest.Evidence.Objects = sortedEvidenceObjects(manifest.Evidence.Objects)
	if err := manifest.validate(); err != nil {
		return Manifest{}, fmt.Errorf("validate generated backup manifest: %w", err)
	}

	temporaryArchivePath := filepath.Join(temporaryDirectory, "backup.zip")
	if err := writeArchive(temporaryArchivePath, dumpPath, evidenceRoot, manifest); err != nil {
		return Manifest{}, err
	}
	archiveInfo, err := os.Stat(temporaryArchivePath)
	if err != nil {
		return Manifest{}, fmt.Errorf("inspect completed backup archive: %w", err)
	}
	if archiveInfo.Size() > maxArchiveBytes {
		return Manifest{}, fmt.Errorf("backup archive exceeds format size limit")
	}
	if err := publishWithoutOverwrite(temporaryArchivePath, outputPath); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func validateSourceEvidenceRoot(evidenceRoot string) error {
	rootInfo, err := os.Stat(evidenceRoot)
	if err != nil {
		return fmt.Errorf("inspect evidence root: %w", err)
	}
	if !rootInfo.IsDir() {
		return fmt.Errorf("evidence root is not a directory")
	}
	objectsInfo, err := os.Stat(filepath.Join(evidenceRoot, "objects"))
	if err != nil {
		return fmt.Errorf("inspect evidence objects directory: %w", err)
	}
	if !objectsInfo.IsDir() {
		return fmt.Errorf("evidence objects path is not a directory")
	}
	return nil
}

type snapshotMetadata struct {
	snapshotID        string
	databaseName      string
	postgresqlVersion string
	schemaVersion     string
	counts            Counts
	evidence          []EvidenceObject
}

func readSnapshotMetadata(ctx context.Context, pool *pgxpool.Pool) (snapshotMetadata, pgx.Tx, error) {
	transaction, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return snapshotMetadata{}, nil, fmt.Errorf("begin backup snapshot: %w", err)
	}
	fail := func(cause error) (snapshotMetadata, pgx.Tx, error) {
		_ = transaction.Rollback(context.Background())
		return snapshotMetadata{}, nil, cause
	}
	metadata := snapshotMetadata{}
	if err := transaction.QueryRow(ctx, `
		SELECT pg_export_snapshot(), current_database(), current_setting('server_version'),
		       COALESCE((SELECT max(version) FROM schema_migrations), '')
	`).Scan(&metadata.snapshotID, &metadata.databaseName, &metadata.postgresqlVersion, &metadata.schemaVersion); err != nil {
		return fail(fmt.Errorf("read backup snapshot identity: %w", err))
	}
	if metadata.schemaVersion == "" {
		return fail(fmt.Errorf("database has no applied schema migration"))
	}
	if err := transaction.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM contracts),
			(SELECT count(*) FROM parties),
			(SELECT count(*) FROM contract_evidence_versions),
			(SELECT count(*) FROM contract_evidence_versions WHERE content_status = 'available'),
			(SELECT count(*) FROM contract_evidence_versions WHERE content_status = 'missing')
	`).Scan(
		&metadata.counts.Contracts,
		&metadata.counts.Parties,
		&metadata.counts.EvidenceVersions,
		&metadata.counts.AvailableEvidenceVersion,
		&metadata.counts.MissingEvidenceVersions,
	); err != nil {
		return fail(fmt.Errorf("read backup snapshot counts: %w", err))
	}
	rows, err := transaction.Query(ctx, `
		SELECT storage_key, size_bytes, sha256
		FROM contract_evidence_versions
		WHERE content_status = 'available'
		ORDER BY storage_key
	`)
	if err != nil {
		return fail(fmt.Errorf("query backup evidence references: %w", err))
	}
	defer rows.Close()
	for rows.Next() {
		var storageKey, digest string
		var size int64
		if err := rows.Scan(&storageKey, &size, &digest); err != nil {
			return fail(fmt.Errorf("scan backup evidence reference: %w", err))
		}
		metadata.evidence = append(metadata.evidence, newEvidenceObject(storageKey, size, digest))
	}
	if err := rows.Err(); err != nil {
		return fail(fmt.Errorf("iterate backup evidence references: %w", err))
	}
	return metadata, transaction, nil
}

func writeArchive(archivePath, dumpPath, evidenceRoot string, manifest Manifest) error {
	archive, err := os.OpenFile(archivePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create backup archive: %w", err)
	}
	succeeded := false
	defer func() {
		_ = archive.Close()
		if !succeeded {
			_ = os.Remove(archivePath)
		}
	}()
	zipWriter := zip.NewWriter(archive)
	if err := addVerifiedFile(zipWriter, databaseEntry, dumpPath, manifest.Database.DumpBytes, manifest.Database.DumpSHA256); err != nil {
		_ = zipWriter.Close()
		return fmt.Errorf("add database dump: %w", err)
	}
	for _, object := range manifest.Evidence.Objects {
		objectPath := filepath.Join(evidenceRoot, "objects", object.StorageKey[:2], object.StorageKey)
		if err := addVerifiedFile(zipWriter, object.Path, objectPath, object.SizeBytes, object.SHA256); err != nil {
			_ = zipWriter.Close()
			return fmt.Errorf("add evidence %s: %w", object.StorageKey, err)
		}
	}
	manifestBytes, err := marshalManifest(manifest)
	if err != nil {
		_ = zipWriter.Close()
		return err
	}
	if err := addBytes(zipWriter, manifestEntry, manifestBytes); err != nil {
		_ = zipWriter.Close()
		return fmt.Errorf("add backup manifest: %w", err)
	}
	if err := zipWriter.Close(); err != nil {
		return fmt.Errorf("close backup ZIP: %w", err)
	}
	if err := archive.Sync(); err != nil {
		return fmt.Errorf("sync backup archive: %w", err)
	}
	if err := archive.Close(); err != nil {
		return fmt.Errorf("close backup archive: %w", err)
	}
	succeeded = true
	return nil
}

func addVerifiedFile(writer *zip.Writer, entryName, filePath string, expectedSize int64, expectedSHA256 string) error {
	file, err := openRegularFile(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	header := &zip.FileHeader{Name: entryName, Method: zip.Store}
	header.SetMode(0o600)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create ZIP entry: %w", err)
	}
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(entry, hash), file)
	if err != nil {
		return fmt.Errorf("copy file into ZIP: %w", err)
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	if size != expectedSize || digest != expectedSHA256 {
		return fmt.Errorf("content integrity mismatch: expected %d bytes/%s, got %d bytes/%s", expectedSize, expectedSHA256, size, digest)
	}
	return nil
}

func addBytes(writer *zip.Writer, entryName string, value []byte) error {
	header := &zip.FileHeader{Name: entryName, Method: zip.Store}
	header.SetMode(0o600)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = entry.Write(value)
	return err
}

func openRegularFile(filePath string) (*os.File, error) {
	info, err := os.Lstat(filePath)
	if err != nil {
		return nil, fmt.Errorf("inspect file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("path is not a regular file")
	}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	return file, nil
}

func hashRegularFile(filePath string) (int64, string, error) {
	file, err := openRegularFile(filePath)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return 0, "", err
	}
	return size, hex.EncodeToString(hash.Sum(nil)), nil
}

func safeAbsolutePath(label, value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", label, err)
	}
	cleaned := filepath.Clean(absolute)
	if cleaned == string(filepath.Separator) {
		return "", fmt.Errorf("%s cannot be filesystem root", label)
	}
	return cleaned, nil
}

func publishWithoutOverwrite(temporaryPath, outputPath string) error {
	if err := os.Link(temporaryPath, outputPath); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("backup output already exists")
		}
		return fmt.Errorf("publish backup archive: %w", err)
	}
	if err := os.Chmod(outputPath, 0o600); err != nil {
		_ = os.Remove(outputPath)
		return fmt.Errorf("restrict backup archive: %w", err)
	}
	if err := os.Remove(temporaryPath); err != nil {
		return fmt.Errorf("remove published temporary archive: %w", err)
	}
	return nil
}
