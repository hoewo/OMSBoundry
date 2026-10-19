package evidencemigration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

const ReportFormat = "omsbizfoundry.contract-evidence-migration-preflight/v1"

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type LegacyVersion struct {
	ContractID       string
	DocumentID       string
	VersionID        string
	Version          int64
	OriginalFilename string
	MediaType        string
	SizeBytes        int64
	SHA256           string
	StorageProvider  string
	StorageKey       string
	ContentStatus    string
}

type Issue struct {
	Code            string `json:"code"`
	Message         string `json:"message"`
	LegacyVersionID string `json:"legacy_version_id,omitempty"`
}

type Mapping struct {
	ContractID             string  `json:"contract_id"`
	DocumentID             string  `json:"document_id"`
	LegacyVersionID        string  `json:"legacy_version_id"`
	LegacyVersion          int64   `json:"legacy_version"`
	ContentStatus          string  `json:"content_status"`
	OriginalFilename       string  `json:"original_filename"`
	MediaType              string  `json:"media_type"`
	SizeBytes              int64   `json:"size_bytes"`
	SHA256                 string  `json:"sha256"`
	SourceObject           string  `json:"source_object,omitempty"`
	ProposedEvidenceFileID string  `json:"proposed_evidence_file_id,omitempty"`
	ProposedVersion        int     `json:"proposed_evidence_version,omitempty"`
	Migratable             bool    `json:"migratable"`
	Issues                 []Issue `json:"issues,omitempty"`
}

type Counts struct {
	Documents     int   `json:"documents"`
	Versions      int   `json:"versions"`
	Available     int   `json:"available"`
	Missing       int   `json:"missing"`
	Migratable    int   `json:"migratable"`
	Blocked       int   `json:"blocked"`
	ExpectedBytes int64 `json:"expected_bytes"`
}

type Report struct {
	Format      string    `json:"format"`
	TenantID    string    `json:"tenant_id"`
	StorageRoot string    `json:"storage_root"`
	Ready       bool      `json:"ready"`
	Counts      Counts    `json:"counts"`
	Issues      []Issue   `json:"issues,omitempty"`
	Mappings    []Mapping `json:"mappings"`
}

func Inspect(ctx context.Context, pool *pgxpool.Pool, storageRoot, tenantID string) (Report, error) {
	if pool == nil {
		return Report{}, fmt.Errorf("Contract database pool is required")
	}
	if !uuidPattern.MatchString(tenantID) {
		return Report{}, fmt.Errorf("tenant ID must be a lowercase RFC 4122 UUID")
	}
	rows, err := pool.Query(ctx, `
		SELECT d.contract_id::text, d.id::text, v.id::text, v.version,
			v.original_filename, v.detected_media_type, v.size_bytes, v.sha256,
			COALESCE(v.storage_provider, ''), COALESCE(v.storage_key, ''), v.content_status
		FROM contract_evidence_documents d
		JOIN contract_evidence_versions v ON v.document_id = d.id
		ORDER BY d.contract_id, d.id, v.version, v.id
	`)
	if err != nil {
		return Report{}, fmt.Errorf("query legacy Contract evidence: %w", err)
	}
	defer rows.Close()
	versions := make([]LegacyVersion, 0)
	for rows.Next() {
		var version LegacyVersion
		if err := rows.Scan(
			&version.ContractID, &version.DocumentID, &version.VersionID, &version.Version,
			&version.OriginalFilename, &version.MediaType, &version.SizeBytes, &version.SHA256,
			&version.StorageProvider, &version.StorageKey, &version.ContentStatus,
		); err != nil {
			return Report{}, fmt.Errorf("scan legacy Contract evidence: %w", err)
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return Report{}, fmt.Errorf("read legacy Contract evidence: %w", err)
	}
	return Evaluate(ctx, storageRoot, tenantID, versions)
}

func Evaluate(ctx context.Context, storageRoot, tenantID string, versions []LegacyVersion) (Report, error) {
	if !uuidPattern.MatchString(tenantID) {
		return Report{}, fmt.Errorf("tenant ID must be a lowercase RFC 4122 UUID")
	}
	absoluteRoot, err := filepath.Abs(storageRoot)
	if err != nil {
		return Report{}, fmt.Errorf("resolve legacy Evidence storage root: %w", err)
	}
	if filepath.Clean(absoluteRoot) == string(filepath.Separator) {
		return Report{}, fmt.Errorf("legacy Evidence storage root cannot be filesystem root")
	}

	ordered := append([]LegacyVersion(nil), versions...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].ContractID != ordered[j].ContractID {
			return ordered[i].ContractID < ordered[j].ContractID
		}
		if ordered[i].DocumentID != ordered[j].DocumentID {
			return ordered[i].DocumentID < ordered[j].DocumentID
		}
		if ordered[i].Version != ordered[j].Version {
			return ordered[i].Version < ordered[j].Version
		}
		return ordered[i].VersionID < ordered[j].VersionID
	})

	report := Report{Format: ReportFormat, TenantID: tenantID, StorageRoot: absoluteRoot, Mappings: make([]Mapping, 0, len(ordered))}
	documents := make(map[string]struct{})
	lastVersion := make(map[string]int64)
	for _, version := range ordered {
		if err := ctx.Err(); err != nil {
			return Report{}, err
		}
		documents[version.DocumentID] = struct{}{}
		report.Counts.Versions++
		mapping := Mapping{
			ContractID: version.ContractID, DocumentID: version.DocumentID,
			LegacyVersionID: version.VersionID, LegacyVersion: version.Version,
			ContentStatus: version.ContentStatus, OriginalFilename: version.OriginalFilename,
			MediaType: version.MediaType, SizeBytes: version.SizeBytes, SHA256: version.SHA256,
		}

		expectedVersion := lastVersion[version.DocumentID] + 1
		if version.Version != expectedVersion {
			mapping.Issues = append(mapping.Issues, Issue{
				Code: "version_sequence_gap", LegacyVersionID: version.VersionID,
				Message: fmt.Sprintf("document %s expected version %d but found %d", version.DocumentID, expectedVersion, version.Version),
			})
		}
		lastVersion[version.DocumentID] = version.Version

		switch version.ContentStatus {
		case "missing":
			report.Counts.Missing++
			if version.StorageProvider != "" || version.StorageKey != "" {
				mapping.Issues = append(mapping.Issues, Issue{Code: "missing_content_shape_invalid", LegacyVersionID: version.VersionID, Message: "missing content unexpectedly has a storage provider or key"})
			}
		case "available":
			report.Counts.Available++
			report.Counts.ExpectedBytes += version.SizeBytes
			mapping.ProposedEvidenceFileID = version.VersionID
			mapping.ProposedVersion = 1
			if version.StorageProvider != "local" {
				mapping.Issues = append(mapping.Issues, Issue{Code: "available_provider_not_local", LegacyVersionID: version.VersionID, Message: fmt.Sprintf("available legacy content uses provider %q instead of local", version.StorageProvider)})
			}
			if !uuidPattern.MatchString(version.StorageKey) {
				mapping.Issues = append(mapping.Issues, Issue{Code: "available_storage_key_invalid", LegacyVersionID: version.VersionID, Message: "available legacy content has an invalid storage key"})
			} else {
				mapping.SourceObject = filepath.ToSlash(filepath.Join("objects", version.StorageKey[:2], version.StorageKey))
				mapping.Issues = append(mapping.Issues, inspectObject(ctx, absoluteRoot, version, mapping.SourceObject)...)
			}
		default:
			mapping.Issues = append(mapping.Issues, Issue{Code: "content_status_invalid", LegacyVersionID: version.VersionID, Message: fmt.Sprintf("unsupported content status %q", version.ContentStatus)})
		}

		mapping.Migratable = version.ContentStatus == "available" && len(mapping.Issues) == 0
		if mapping.Migratable {
			report.Counts.Migratable++
		} else if version.ContentStatus == "available" || len(mapping.Issues) > 0 {
			report.Counts.Blocked++
		}
		for _, issue := range mapping.Issues {
			report.Issues = append(report.Issues, issue)
		}
		report.Mappings = append(report.Mappings, mapping)
	}
	report.Counts.Documents = len(documents)
	report.Ready = len(report.Issues) == 0
	return report, nil
}

func inspectObject(ctx context.Context, root string, version LegacyVersion, relative string) []Issue {
	path := filepath.Join(root, filepath.FromSlash(relative))
	info, err := os.Lstat(path)
	if err != nil {
		return []Issue{{Code: "content_unavailable", LegacyVersionID: version.VersionID, Message: fmt.Sprintf("cannot inspect %s: %v", relative, err)}}
	}
	if !info.Mode().IsRegular() {
		return []Issue{{Code: "content_not_regular", LegacyVersionID: version.VersionID, Message: fmt.Sprintf("%s is not a regular file", relative)}}
	}
	issues := make([]Issue, 0, 2)
	if info.Size() != version.SizeBytes {
		issues = append(issues, Issue{Code: "content_size_mismatch", LegacyVersionID: version.VersionID, Message: fmt.Sprintf("%s has %d bytes; database expects %d", relative, info.Size(), version.SizeBytes)})
	}
	file, err := os.Open(path)
	if err != nil {
		return append(issues, Issue{Code: "content_read_failed", LegacyVersionID: version.VersionID, Message: fmt.Sprintf("cannot open %s: %v", relative, err)})
	}
	defer file.Close()
	hash := sha256.New()
	buffer := make([]byte, 1024*1024)
	for {
		if err := ctx.Err(); err != nil {
			return append(issues, Issue{Code: "content_read_cancelled", LegacyVersionID: version.VersionID, Message: fmt.Sprintf("hashing %s was cancelled", relative)})
		}
		n, readErr := file.Read(buffer)
		if n > 0 {
			_, _ = hash.Write(buffer[:n])
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return append(issues, Issue{Code: "content_read_failed", LegacyVersionID: version.VersionID, Message: fmt.Sprintf("cannot hash %s: %v", relative, readErr)})
		}
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	if digest != version.SHA256 {
		issues = append(issues, Issue{Code: "content_sha256_mismatch", LegacyVersionID: version.VersionID, Message: fmt.Sprintf("%s has SHA-256 %s; database expects %s", relative, digest, version.SHA256)})
	}
	return issues
}
