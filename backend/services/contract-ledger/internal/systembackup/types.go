package systembackup

import (
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	Format        = "omsbizfoundry.contract-ledger/full-backup-v1"
	FormatVersion = 1

	manifestEntry = "manifest.json"
	databaseEntry = "database.dump"
)

var (
	sha256Pattern     = regexp.MustCompile(`^[0-9a-f]{64}$`)
	storageKeyPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

type Manifest struct {
	Format      string           `json:"format"`
	Version     int              `json:"version"`
	GeneratedAt string           `json:"generated_at"`
	Database    DatabaseManifest `json:"database"`
	Evidence    EvidenceManifest `json:"evidence"`
}

type DatabaseManifest struct {
	Name              string `json:"name"`
	PostgreSQLVersion string `json:"postgresql_version"`
	SchemaVersion     string `json:"schema_version"`
	DumpPath          string `json:"dump_path"`
	DumpBytes         int64  `json:"dump_bytes"`
	DumpSHA256        string `json:"dump_sha256"`
	SessionsExcluded  bool   `json:"sessions_excluded"`
	Counts            Counts `json:"counts"`
}

type Counts struct {
	Contracts                int64 `json:"contracts"`
	Parties                  int64 `json:"parties"`
	EvidenceVersions         int64 `json:"evidence_versions"`
	AvailableEvidenceVersion int64 `json:"available_evidence_versions"`
	MissingEvidenceVersions  int64 `json:"missing_evidence_versions"`
}

type EvidenceManifest struct {
	Layout  string           `json:"layout"`
	Objects []EvidenceObject `json:"objects"`
}

type EvidenceObject struct {
	StorageKey string `json:"storage_key"`
	Path       string `json:"path"`
	SizeBytes  int64  `json:"size_bytes"`
	SHA256     string `json:"sha256"`
}

func newEvidenceObject(storageKey string, sizeBytes int64, sha256 string) EvidenceObject {
	return EvidenceObject{
		StorageKey: storageKey,
		Path:       evidenceEntryPath(storageKey),
		SizeBytes:  sizeBytes,
		SHA256:     sha256,
	}
}

func evidenceEntryPath(storageKey string) string {
	prefix := ""
	if len(storageKey) >= 2 {
		prefix = storageKey[:2]
	}
	return path.Join("evidence", "objects", prefix, storageKey)
}

func (manifest Manifest) validate() error {
	if manifest.Format != Format || manifest.Version != FormatVersion {
		return fmt.Errorf("unsupported backup format")
	}
	generatedAt, err := time.Parse(time.RFC3339Nano, manifest.GeneratedAt)
	if err != nil || generatedAt.Location() != time.UTC {
		return fmt.Errorf("generated_at must be an RFC3339 UTC timestamp")
	}
	if strings.TrimSpace(manifest.Database.Name) == "" || strings.ContainsAny(manifest.Database.Name, "\x00\r\n") {
		return fmt.Errorf("database name is invalid")
	}
	if strings.TrimSpace(manifest.Database.PostgreSQLVersion) == "" || strings.ContainsAny(manifest.Database.PostgreSQLVersion, "\x00\r\n") {
		return fmt.Errorf("postgresql version is invalid")
	}
	if strings.TrimSpace(manifest.Database.SchemaVersion) == "" || strings.ContainsAny(manifest.Database.SchemaVersion, "\x00\r\n") {
		return fmt.Errorf("schema version is invalid")
	}
	if manifest.Database.DumpPath != databaseEntry || manifest.Database.DumpBytes < 1 || !sha256Pattern.MatchString(manifest.Database.DumpSHA256) {
		return fmt.Errorf("database dump metadata is invalid")
	}
	if !manifest.Database.SessionsExcluded {
		return fmt.Errorf("backup must exclude local session data")
	}
	counts := manifest.Database.Counts
	if counts.Contracts < 0 || counts.Parties < 0 || counts.EvidenceVersions < 0 ||
		counts.AvailableEvidenceVersion < 0 || counts.MissingEvidenceVersions < 0 ||
		counts.AvailableEvidenceVersion+counts.MissingEvidenceVersions != counts.EvidenceVersions {
		return fmt.Errorf("database counts are invalid")
	}
	if manifest.Evidence.Layout != "local-store-v1" {
		return fmt.Errorf("unsupported evidence layout")
	}
	if len(manifest.Evidence.Objects) > maxArchiveEntries-2 {
		return fmt.Errorf("evidence object count exceeds backup format limit")
	}
	if int64(len(manifest.Evidence.Objects)) != counts.AvailableEvidenceVersion {
		return fmt.Errorf("evidence object count does not match database counts")
	}
	seenKeys := make(map[string]struct{}, len(manifest.Evidence.Objects))
	seenPaths := make(map[string]struct{}, len(manifest.Evidence.Objects))
	previousKey := ""
	for _, object := range manifest.Evidence.Objects {
		if !storageKeyPattern.MatchString(object.StorageKey) || object.Path != evidenceEntryPath(object.StorageKey) ||
			object.SizeBytes < 1 || !sha256Pattern.MatchString(object.SHA256) {
			return fmt.Errorf("evidence object metadata is invalid")
		}
		if previousKey != "" && object.StorageKey <= previousKey {
			return fmt.Errorf("evidence objects must be sorted by storage key")
		}
		previousKey = object.StorageKey
		if _, exists := seenKeys[object.StorageKey]; exists {
			return fmt.Errorf("duplicate evidence storage key")
		}
		seenKeys[object.StorageKey] = struct{}{}
		if _, exists := seenPaths[object.Path]; exists {
			return fmt.Errorf("duplicate evidence archive path")
		}
		seenPaths[object.Path] = struct{}{}
	}
	return nil
}

func marshalManifest(manifest Manifest) ([]byte, error) {
	if err := manifest.validate(); err != nil {
		return nil, err
	}
	value, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode backup manifest: %w", err)
	}
	return append(value, '\n'), nil
}

func sortedEvidenceObjects(values []EvidenceObject) []EvidenceObject {
	result := append([]EvidenceObject(nil), values...)
	sort.Slice(result, func(left, right int) bool { return result[left].StorageKey < result[right].StorageKey })
	return result
}
