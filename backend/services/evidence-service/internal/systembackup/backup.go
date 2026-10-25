package systembackup

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/evidence-service/internal/database"
)

const (
	Format              = "omsbizfoundry.evidence-service/full-backup-v1"
	FormatVersion       = 1
	manifestEntry       = "manifest.json"
	databaseEntry       = "database.dump"
	maxManifestBytes    = 4 << 20
	maxArchiveEntries   = 100_002
	maxArchiveBytes     = int64(256) << 30
	restoreMarkerName   = ".evidence-service-restore-in-progress"
	localStorageLayout  = "local-object-tree-v1"
	externalStorageKind = "external-provider-v1"
	mixedFormat         = "omsbizfoundry.evidence-service/full-backup-v2"
	mixedStorageLayout  = "local-and-external-v2"
)

type Counts struct {
	Files          int64 `json:"files"`
	Versions       int64 `json:"versions"`
	UploadSessions int64 `json:"upload_sessions"`
	AuditEntries   int64 `json:"audit_entries"`
}
type Object struct {
	Key       string `json:"key"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}

// ExternalObject records immutable locations, NOT external object bytes.
type ExternalObject struct {
	Bucket    string `json:"bucket"`
	Key       string `json:"key"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}
type Manifest struct {
	Format      string `json:"format"`
	Version     int    `json:"version"`
	GeneratedAt string `json:"generated_at"`
	Database    struct {
		Name              string `json:"name"`
		PostgreSQLVersion string `json:"postgresql_version"`
		SchemaVersion     string `json:"schema_version"`
		DumpPath          string `json:"dump_path"`
		DumpBytes         int64  `json:"dump_bytes"`
		DumpSHA256        string `json:"dump_sha256"`
		Counts            Counts `json:"counts"`
	} `json:"database"`
	Storage struct {
		Provider        string           `json:"provider"`
		Layout          string           `json:"layout"`
		Objects         []Object         `json:"objects"`
		ExternalObjects []ExternalObject `json:"external_objects,omitempty"`
	} `json:"storage"`
}

type CreateOptions struct {
	Pool        *pgxpool.Pool
	Client      PostgreSQLClient
	Provider    string
	StorageRoot string
	OutputPath  string
	Now         func() time.Time
}

func Create(ctx context.Context, options CreateOptions) (Manifest, error) {
	if options.Pool == nil || options.Client == nil {
		return Manifest{}, fmt.Errorf("database pool and PostgreSQL client are required")
	}
	provider, root, output, err := validateCreateOptions(options.Provider, options.StorageRoot, options.OutputPath)
	if err != nil {
		return Manifest{}, err
	}
	if _, err := os.Lstat(output); err == nil {
		return Manifest{}, fmt.Errorf("backup output already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return Manifest{}, fmt.Errorf("inspect backup output: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o700); err != nil {
		return Manifest{}, err
	}
	workspace, err := os.MkdirTemp(filepath.Dir(output), ".evidence-backup-*")
	if err != nil {
		return Manifest{}, err
	}
	defer os.RemoveAll(workspace)
	dumpPath := filepath.Join(workspace, databaseEntry)
	dump, err := os.OpenFile(dumpPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return Manifest{}, err
	}
	metadata, snapshot, err := snapshotMetadata(ctx, options.Pool)
	if err != nil {
		_ = dump.Close()
		return Manifest{}, err
	}
	if metadata.databaseName != options.Client.DatabaseName() {
		_ = snapshot.Rollback(context.Background())
		_ = dump.Close()
		return Manifest{}, fmt.Errorf("PostgreSQL client targets database %q; source is %q", options.Client.DatabaseName(), metadata.databaseName)
	}
	if err := options.Client.Dump(ctx, metadata.snapshotID, metadata.postgresqlVersion, dump); err != nil {
		_ = snapshot.Rollback(context.Background())
		_ = dump.Close()
		return Manifest{}, err
	}
	if err := dump.Sync(); err != nil {
		_ = snapshot.Rollback(context.Background())
		_ = dump.Close()
		return Manifest{}, err
	}
	if err := dump.Close(); err != nil {
		_ = snapshot.Rollback(context.Background())
		return Manifest{}, err
	}
	if err := snapshot.Commit(ctx); err != nil {
		return Manifest{}, err
	}
	dumpSize, dumpHash, err := hashRegularFile(dumpPath)
	if err != nil {
		return Manifest{}, err
	}
	objects := []Object{}
	localReferences, externalObjects, err := partitionReferences(provider, metadata.references)
	if err != nil {
		return Manifest{}, err
	}
	if hasLocalStorage(provider) {
		objects, err = scanLocalObjects(root)
		if err != nil {
			return Manifest{}, err
		}
		if err := validateReferences(localReferences, objects); err != nil {
			return Manifest{}, err
		}
	}
	now := time.Now
	if options.Now != nil {
		now = options.Now
	}
	manifest := Manifest{Format: Format, Version: FormatVersion, GeneratedAt: now().UTC().Format(time.RFC3339Nano)}
	if provider == "hybrid" {
		manifest.Format, manifest.Version = mixedFormat, 2
		manifest.Storage.ExternalObjects = externalObjects
	}
	manifest.Database.Name = metadata.databaseName
	manifest.Database.PostgreSQLVersion = metadata.postgresqlVersion
	manifest.Database.SchemaVersion = metadata.schemaVersion
	manifest.Database.DumpPath = databaseEntry
	manifest.Database.DumpBytes = dumpSize
	manifest.Database.DumpSHA256 = dumpHash
	manifest.Database.Counts = metadata.counts
	manifest.Storage.Provider = provider
	manifest.Storage.Objects = objects
	if provider == "local" {
		manifest.Storage.Layout = localStorageLayout
	} else if provider == "hybrid" {
		manifest.Storage.Layout = mixedStorageLayout
	} else {
		manifest.Storage.Layout = externalStorageKind
	}
	if err := manifest.validate(); err != nil {
		return Manifest{}, err
	}
	temporary := filepath.Join(workspace, "backup.zip")
	if err := writeArchive(temporary, dumpPath, root, manifest); err != nil {
		return Manifest{}, err
	}
	if err := publishWithoutOverwrite(temporary, output); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

type reference struct {
	Provider string
	Bucket   string
	Key      string
	Size     int64
	SHA256   string
}
type sourceMetadata struct {
	snapshotID        string
	databaseName      string
	postgresqlVersion string
	schemaVersion     string
	counts            Counts
	references        []reference
}

func snapshotMetadata(ctx context.Context, pool *pgxpool.Pool) (sourceMetadata, pgx.Tx, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return sourceMetadata{}, nil, err
	}
	fail := func(cause error) (sourceMetadata, pgx.Tx, error) {
		_ = tx.Rollback(context.Background())
		return sourceMetadata{}, nil, cause
	}
	var value sourceMetadata
	if err := tx.QueryRow(ctx, `SELECT pg_export_snapshot(),current_database(),current_setting('server_version'),COALESCE((SELECT max(version) FROM schema_migrations),'')`).Scan(&value.snapshotID, &value.databaseName, &value.postgresqlVersion, &value.schemaVersion); err != nil {
		return fail(err)
	}
	if value.schemaVersion == "" {
		return fail(fmt.Errorf("database has no applied migration"))
	}
	if err := tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM files),(SELECT count(*) FROM file_versions),(SELECT count(*) FROM upload_sessions),(SELECT count(*) FROM evidence_audit_entries)`).Scan(&value.counts.Files, &value.counts.Versions, &value.counts.UploadSessions, &value.counts.AuditEntries); err != nil {
		return fail(err)
	}
	rows, err := tx.Query(ctx, `SELECT v.provider,COALESCE(v.bucket,''),v.object_key,v.size_bytes,v.sha256 FROM file_versions v WHERE NOT EXISTS(SELECT 1 FROM file_version_cleanup_attempts c WHERE c.tenant_id=v.tenant_id AND c.file_id=v.file_id AND c.version=v.version AND c.status='succeeded') ORDER BY v.provider,v.bucket,v.object_key`)
	if err != nil {
		return fail(err)
	}
	for rows.Next() {
		var item reference
		if err := rows.Scan(&item.Provider, &item.Bucket, &item.Key, &item.Size, &item.SHA256); err != nil {
			rows.Close()
			return fail(err)
		}
		value.references = append(value.references, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fail(err)
	}
	return value, tx, nil
}

func scanLocalObjects(root string) ([]Object, error) {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("local storage root must be an existing directory")
	}
	result := []Object{}
	err = filepath.WalkDir(root, func(name string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name == root {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("local storage contains symlink %q", name)
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("local storage contains non-regular file %q", name)
		}
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		key := filepath.ToSlash(relative)
		if key == restoreMarkerName {
			return nil
		}
		if !safeObjectKey(key) {
			return fmt.Errorf("unsafe local object key %q", key)
		}
		size, digest, err := hashRegularFile(name)
		if err != nil {
			return err
		}
		result = append(result, Object{Key: key, Path: path.Join("objects", key), SizeBytes: size, SHA256: digest})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result, nil
}

func validateReferences(references []reference, objects []Object) error {
	byKey := make(map[string]Object, len(objects))
	for _, object := range objects {
		byKey[object.Key] = object
	}
	for _, item := range references {
		if item.Provider != "local" {
			return fmt.Errorf("database version uses provider %q but backup provider is local", item.Provider)
		}
		object, exists := byKey[item.Key]
		if !exists || object.SizeBytes != item.Size || object.SHA256 != item.SHA256 {
			return fmt.Errorf("confirmed object %q is missing or fails integrity verification", item.Key)
		}
	}
	return nil
}

func hasLocalStorage(provider string) bool { return provider == "local" || provider == "hybrid" }

func partitionReferences(provider string, refs []reference) ([]reference, []ExternalObject, error) {
	local := []reference{}
	external := []ExternalObject{}
	for _, ref := range refs {
		switch ref.Provider {
		case "local":
			if !hasLocalStorage(provider) || ref.Bucket != "" {
				return nil, nil, fmt.Errorf("local reference conflicts with configured backup storage")
			}
			local = append(local, ref)
		case "aliyun-oss":
			if provider != "aliyun-oss" && provider != "hybrid" {
				return nil, nil, fmt.Errorf("OSS reference conflicts with configured backup storage")
			}
			item := ExternalObject{Bucket: ref.Bucket, Key: ref.Key, SizeBytes: ref.Size, SHA256: ref.SHA256}
			if err := validateExternalObject(item); err != nil {
				return nil, nil, err
			}
			external = append(external, item)
		default:
			return nil, nil, fmt.Errorf("unknown storage provider in confirmed version")
		}
	}
	sort.Slice(external, func(i, j int) bool {
		return external[i].Bucket+"/"+external[i].Key < external[j].Bucket+"/"+external[j].Key
	})
	return local, external, nil
}

func validateExternalObject(item ExternalObject) error {
	if item.Bucket == "" || strings.ContainsAny(item.Bucket, "/\\: \t\n") || !safeObjectKey(item.Key) || item.SizeBytes < 1 || !isSHA256(item.SHA256) {
		return fmt.Errorf("invalid external storage reference")
	}
	return nil
}

func writeArchive(output, dumpPath, root string, manifest Manifest) error {
	file, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	succeeded := false
	defer func() {
		_ = file.Close()
		if !succeeded {
			_ = os.Remove(output)
		}
	}()
	writer := zip.NewWriter(file)
	manifestBytes, _ := json.MarshalIndent(manifest, "", "  ")
	manifestBytes = append(manifestBytes, '\n')
	if err := addBytes(writer, manifestEntry, manifestBytes); err != nil {
		return err
	}
	if err := addFile(writer, databaseEntry, dumpPath); err != nil {
		return err
	}
	for _, object := range manifest.Storage.Objects {
		if err := addFile(writer, object.Path, filepath.Join(root, filepath.FromSlash(object.Key))); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	succeeded = true
	return nil
}

func addBytes(writer *zip.Writer, name string, value []byte) error {
	header := &zip.FileHeader{Name: name, Method: zip.Store}
	header.SetMode(0o600)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = entry.Write(value)
	return err
}
func addFile(writer *zip.Writer, archiveName, source string) error {
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()
	header := &zip.FileHeader{Name: archiveName, Method: zip.Store}
	header.SetMode(0o600)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, file)
	return err
}

func (manifest Manifest) validate() error {
	legacy := manifest.Format == Format && manifest.Version == FormatVersion
	mixed := manifest.Format == mixedFormat && manifest.Version == 2
	if !legacy && !mixed {
		return fmt.Errorf("unsupported backup format")
	}
	generated, err := time.Parse(time.RFC3339Nano, manifest.GeneratedAt)
	if err != nil || generated.Location() != time.UTC || manifest.Database.Name == "" || manifest.Database.SchemaVersion == "" || manifest.Database.DumpPath != databaseEntry || manifest.Database.DumpBytes < 1 || !isSHA256(manifest.Database.DumpSHA256) {
		return fmt.Errorf("invalid backup manifest")
	}
	if manifest.Database.Counts.Files < 0 || manifest.Database.Counts.Versions < 0 || manifest.Database.Counts.UploadSessions < 0 || manifest.Database.Counts.AuditEntries < 0 {
		return fmt.Errorf("invalid backup counts")
	}
	if manifest.Storage.Provider != "local" && manifest.Storage.Provider != "aliyun-oss" && manifest.Storage.Provider != "hybrid" {
		return fmt.Errorf("unsupported storage provider")
	}
	if manifest.Storage.Provider == "local" && manifest.Storage.Layout != localStorageLayout {
		return fmt.Errorf("invalid local storage layout")
	}
	if manifest.Storage.Provider == "aliyun-oss" && (manifest.Storage.Layout != externalStorageKind || len(manifest.Storage.Objects) != 0) {
		return fmt.Errorf("invalid external storage manifest")
	}
	if (mixed && (manifest.Storage.Provider != "hybrid" || manifest.Storage.Layout != mixedStorageLayout)) || (legacy && (manifest.Storage.Provider == "hybrid" || len(manifest.Storage.ExternalObjects) != 0)) {
		return fmt.Errorf("storage layout does not match backup format version")
	}
	externalPrevious := ""
	for _, item := range manifest.Storage.ExternalObjects {
		if err := validateExternalObject(item); err != nil {
			return err
		}
		key := item.Bucket + "/" + item.Key
		if key <= externalPrevious {
			return fmt.Errorf("duplicate or unsorted external storage reference")
		}
		externalPrevious = key
	}
	previous := ""
	seen := map[string]struct{}{}
	for _, object := range manifest.Storage.Objects {
		if !safeObjectKey(object.Key) || object.Path != path.Join("objects", object.Key) || object.SizeBytes < 1 || !isSHA256(object.SHA256) || object.Key <= previous {
			return fmt.Errorf("invalid or unsorted storage object")
		}
		if _, exists := seen[object.Key]; exists {
			return fmt.Errorf("duplicate storage object")
		}
		seen[object.Key] = struct{}{}
		previous = object.Key
	}
	return nil
}

func validateCreateOptions(provider, root, output string) (string, string, string, error) {
	provider = strings.TrimSpace(provider)
	if provider != "local" && provider != "aliyun-oss" && provider != "hybrid" {
		return "", "", "", fmt.Errorf("provider must be local, aliyun-oss or hybrid")
	}
	output, err := absolutePath("backup output", output)
	if err != nil {
		return "", "", "", err
	}
	if hasLocalStorage(provider) {
		root, err = absolutePath("storage root", root)
		if err != nil {
			return "", "", "", err
		}
		if relative, relErr := filepath.Rel(root, output); relErr == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return "", "", "", fmt.Errorf("backup output must be outside the local storage root")
		}
	} else {
		root = ""
	}
	return provider, root, output, nil
}

func absolutePath(label, value string) (string, error) {
	if strings.TrimSpace(value) == "" || !filepath.IsAbs(value) {
		return "", fmt.Errorf("%s must be an absolute path", label)
	}
	return filepath.Clean(value), nil
}
func safeObjectKey(value string) bool {
	return value != "" && !strings.Contains(value, "\\") && !strings.HasPrefix(value, "/") && path.Clean(value) == value && value != ".." && !strings.HasPrefix(value, "../")
}
func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
func hashRegularFile(name string) (int64, string, error) {
	file, err := os.Open(name)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return 0, "", fmt.Errorf("%q is not a regular file", name)
	}
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return 0, "", err
	}
	return size, hex.EncodeToString(hash.Sum(nil)), nil
}
func publishWithoutOverwrite(source, destination string) error {
	if err := os.Link(source, destination); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("backup output already exists")
		}
		return err
	}
	return os.Remove(source)
}

type RestoreOptions struct {
	Pool        *pgxpool.Pool
	Client      PostgreSQLClient
	Provider    string
	StorageRoot string
	InputPath   string
}

func Restore(ctx context.Context, options RestoreOptions) (Manifest, error) {
	if options.Pool == nil || options.Client == nil {
		return Manifest{}, fmt.Errorf("database pool and PostgreSQL client are required")
	}
	input, err := absolutePath("backup input", options.InputPath)
	if err != nil {
		return Manifest{}, err
	}
	archive, entries, manifest, err := openArchive(input)
	if err != nil {
		return Manifest{}, err
	}
	defer archive.Close()
	if options.Provider != manifest.Storage.Provider {
		return Manifest{}, fmt.Errorf("target provider %q does not match backup provider %q", options.Provider, manifest.Storage.Provider)
	}
	serverVersion, err := validateEmptyDatabase(ctx, options.Pool, options.Client.DatabaseName())
	if err != nil {
		return Manifest{}, err
	}
	parent := filepath.Dir(input)
	root := ""
	if hasLocalStorage(manifest.Storage.Provider) {
		root, err = absolutePath("storage root", options.StorageRoot)
		if err != nil {
			return Manifest{}, err
		}
		if _, err := os.Lstat(root); err == nil {
			return Manifest{}, fmt.Errorf("target storage root already exists")
		} else if !errors.Is(err, os.ErrNotExist) {
			return Manifest{}, err
		}
		parent = filepath.Dir(root)
	}
	workspace, err := os.MkdirTemp(parent, ".evidence-restore-*")
	if err != nil {
		return Manifest{}, err
	}
	defer os.RemoveAll(workspace)
	dumpPath := filepath.Join(workspace, databaseEntry)
	if err := extractEntry(entries[databaseEntry], dumpPath, manifest.Database.DumpBytes, manifest.Database.DumpSHA256); err != nil {
		return Manifest{}, err
	}
	marker := ""
	if hasLocalStorage(manifest.Storage.Provider) {
		staged := filepath.Join(workspace, "storage")
		if err := os.MkdirAll(staged, 0o700); err != nil {
			return Manifest{}, err
		}
		for _, object := range manifest.Storage.Objects {
			if err := extractEntry(entries[object.Path], filepath.Join(staged, filepath.FromSlash(object.Key)), object.SizeBytes, object.SHA256); err != nil {
				return Manifest{}, err
			}
		}
		marker, err = randomMarker()
		if err != nil {
			return Manifest{}, err
		}
		if err := os.WriteFile(filepath.Join(staged, restoreMarkerName), []byte(marker), 0o600); err != nil {
			return Manifest{}, err
		}
		if err := os.Rename(staged, root); err != nil {
			return Manifest{}, err
		}
	}
	if err := options.Client.Restore(ctx, dumpPath, manifest.Database.PostgreSQLVersion, serverVersion); err != nil {
		if root != "" {
			_ = removeOwnedRoot(root, marker)
		}
		return Manifest{}, err
	}
	if err := verifyRestored(ctx, options.Pool, root, manifest); err != nil {
		return Manifest{}, fmt.Errorf("restore committed but verification failed; isolate and recreate both targets before retrying: %w", err)
	}
	if root != "" {
		if err := os.Remove(filepath.Join(root, restoreMarkerName)); err != nil {
			return Manifest{}, err
		}
	}
	return manifest, nil
}

func openArchive(name string) (*os.File, map[string]*zip.File, Manifest, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, nil, Manifest{}, err
	}
	fail := func(cause error) (*os.File, map[string]*zip.File, Manifest, error) {
		_ = file.Close()
		return nil, nil, Manifest{}, cause
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maxArchiveBytes {
		return fail(fmt.Errorf("backup archive size or type is invalid"))
	}
	reader, err := zip.NewReader(file, info.Size())
	if err != nil || len(reader.File) < 2 || len(reader.File) > maxArchiveEntries {
		return fail(fmt.Errorf("backup ZIP is invalid"))
	}
	entries := map[string]*zip.File{}
	for _, entry := range reader.File {
		if entry.Name == "" || strings.Contains(entry.Name, "\\") || strings.HasPrefix(entry.Name, "/") || path.Clean(entry.Name) != entry.Name || !entry.Mode().IsRegular() || entry.Method != zip.Store {
			return fail(fmt.Errorf("unsafe backup entry %q", entry.Name))
		}
		if _, exists := entries[entry.Name]; exists {
			return fail(fmt.Errorf("duplicate backup entry"))
		}
		entries[entry.Name] = entry
	}
	manifestFile := entries[manifestEntry]
	if manifestFile == nil || manifestFile.UncompressedSize64 > maxManifestBytes {
		return fail(fmt.Errorf("backup manifest is missing or too large"))
	}
	data, err := readEntry(manifestFile, maxManifestBytes)
	if err != nil {
		return fail(err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return fail(err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fail(fmt.Errorf("manifest must contain one JSON value"))
	}
	if err := manifest.validate(); err != nil {
		return fail(err)
	}
	expected := map[string]struct{}{manifestEntry: {}, databaseEntry: {}}
	for _, object := range manifest.Storage.Objects {
		expected[object.Path] = struct{}{}
	}
	if len(entries) != len(expected) {
		return fail(fmt.Errorf("backup contains missing or unknown entries"))
	}
	for name := range expected {
		if entries[name] == nil {
			return fail(fmt.Errorf("backup entry %q is missing", name))
		}
	}
	return file, entries, manifest, nil
}

func readEntry(entry *zip.File, limit uint64) ([]byte, error) {
	reader, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, int64(limit)+1))
	if err != nil || uint64(len(data)) != entry.UncompressedSize64 || uint64(len(data)) > limit {
		return nil, fmt.Errorf("backup entry length is invalid")
	}
	return data, nil
}
func extractEntry(entry *zip.File, destination string, size int64, digest string) error {
	if entry == nil || entry.UncompressedSize64 != uint64(size) {
		return fmt.Errorf("backup entry metadata mismatch")
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	reader, err := entry.Open()
	if err != nil {
		return err
	}
	defer reader.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		_ = output.Close()
		if !ok {
			_ = os.Remove(destination)
		}
	}()
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(output, hash), reader)
	if err != nil || written != size || hex.EncodeToString(hash.Sum(nil)) != digest {
		return fmt.Errorf("backup entry integrity mismatch")
	}
	if err := output.Sync(); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	ok = true
	return nil
}

func validateEmptyDatabase(ctx context.Context, pool *pgxpool.Pool, clientName string) (string, error) {
	var name, version string
	var relations int64
	if err := pool.QueryRow(ctx, `SELECT current_database(),current_setting('server_version'),(SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname !~ '^pg_toast' AND c.relkind IN ('r','p','v','m','S','f'))`).Scan(&name, &version, &relations); err != nil {
		return "", err
	}
	if name != clientName || relations != 0 {
		return "", fmt.Errorf("restore target database must be the matching empty database")
	}
	return version, nil
}

func verifyRestored(ctx context.Context, pool *pgxpool.Pool, root string, manifest Manifest) error {
	version, err := database.RequireCurrentSchema(ctx, pool)
	if err != nil || version != manifest.Database.SchemaVersion {
		return fmt.Errorf("restored schema mismatch: %w", err)
	}
	var counts Counts
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM files),(SELECT count(*) FROM file_versions),(SELECT count(*) FROM upload_sessions),(SELECT count(*) FROM evidence_audit_entries)`).Scan(&counts.Files, &counts.Versions, &counts.UploadSessions, &counts.AuditEntries); err != nil || counts != manifest.Database.Counts {
		return fmt.Errorf("restored counts do not match manifest")
	}
	rows, err := pool.Query(ctx, `SELECT v.provider,COALESCE(v.bucket,''),v.object_key,v.size_bytes,v.sha256 FROM file_versions v WHERE NOT EXISTS(SELECT 1 FROM file_version_cleanup_attempts c WHERE c.tenant_id=v.tenant_id AND c.file_id=v.file_id AND c.version=v.version AND c.status='succeeded') ORDER BY v.provider,v.bucket,v.object_key`)
	if err != nil {
		return err
	}
	refs := []reference{}
	for rows.Next() {
		var ref reference
		if err := rows.Scan(&ref.Provider, &ref.Bucket, &ref.Key, &ref.Size, &ref.SHA256); err != nil {
			rows.Close()
			return err
		}
		refs = append(refs, ref)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	localRefs, external, err := partitionReferences(manifest.Storage.Provider, refs)
	if err != nil {
		return err
	}
	if manifest.Storage.Provider == "hybrid" {
		if len(external) != len(manifest.Storage.ExternalObjects) {
			return fmt.Errorf("restored external reference set does not match")
		}
		for i := range external {
			if external[i] != manifest.Storage.ExternalObjects[i] {
				return fmt.Errorf("restored external reference does not match")
			}
		}
	}
	if !hasLocalStorage(manifest.Storage.Provider) {
		return nil
	}
	objects, err := scanLocalObjectsExcludingMarker(root)
	if err != nil || len(objects) != len(manifest.Storage.Objects) {
		return fmt.Errorf("restored local object set does not match")
	}
	for index := range objects {
		if objects[index] != manifest.Storage.Objects[index] {
			return fmt.Errorf("restored local object %q does not match", objects[index].Key)
		}
	}
	return validateReferences(localRefs, objects)
}

func scanLocalObjectsExcludingMarker(root string) ([]Object, error) {
	return scanLocalObjects(root)
}

func randomMarker() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
func removeOwnedRoot(root, marker string) error {
	value, err := os.ReadFile(filepath.Join(root, restoreMarkerName))
	if err != nil || string(value) != marker {
		return fmt.Errorf("restore marker does not match")
	}
	return os.RemoveAll(root)
}
