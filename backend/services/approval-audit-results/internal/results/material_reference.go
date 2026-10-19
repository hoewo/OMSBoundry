package results

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

// MaterialReferenceProjection is a versioned reference wire, not a copy of
// the amount/result snapshot or a storage object locator.
type MaterialReferenceProjection struct {
	SchemaVersion     string                  `json:"schema_version"`
	SourceApplication string                  `json:"source_application"`
	ID                string                  `json:"id"`
	TenantID          string                  `json:"tenant_id"`
	ProjectID         string                  `json:"project_id"`
	ResourceType      string                  `json:"resource_type"`
	ResultKind        string                  `json:"result_kind"`
	Status            string                  `json:"status"`
	Revision          int64                   `json:"revision"`
	Digest            string                  `json:"digest"`
	Items             []MaterialReferenceItem `json:"items"`
}

type MaterialReferenceItem struct {
	RelationKey            string `json:"relation_key"`
	FileID                 string `json:"file_id"`
	Version                int64  `json:"version"`
	EvidenceSourceRevision int64  `json:"evidence_source_revision"`
	DisplayName            string `json:"display_name"`
}

// MaterialReference reads only an already-owned, single-revision snapshot.
// It never resolves files or rewrites a withdrawn result or its history.
func MaterialReference(value Result) (MaterialReferenceProjection, error) {
	canonicalID := func(id string) bool { return ValidUUID(id) && id == normalizeUUID(id) }
	if !canonicalID(value.ID) || !canonicalID(value.TenantID) || !canonicalID(value.Project.ID) || value.Revision < 1 ||
		(value.Kind != KindApproval && value.Kind != KindAudit) || (value.State != StateRecorded && value.State != StateWithdrawn) || len(value.Evidence) > 100 {
		return MaterialReferenceProjection{}, ErrReferenceInvalid
	}
	items := make([]MaterialReferenceItem, 0, len(value.Evidence))
	seen := map[EvidenceRef]bool{}
	for _, file := range value.Evidence {
		if !canonicalID(file.FileID) || file.Version < 1 || !validText(file.Slot, 1, 100) || file.Slot != strings.TrimSpace(file.Slot) ||
			!validText(file.DisplayName, 1, 500) || file.DisplayName != strings.TrimSpace(file.DisplayName) || seen[file.EvidenceRef] {
			return MaterialReferenceProjection{}, ErrReferenceInvalid
		}
		seen[file.EvidenceRef] = true
		items = append(items, MaterialReferenceItem{RelationKey: file.Slot, FileID: file.FileID, Version: file.Version, EvidenceSourceRevision: file.Version, DisplayName: file.DisplayName})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].RelationKey != items[j].RelationKey {
			return items[i].RelationKey < items[j].RelationKey
		}
		if items[i].FileID != items[j].FileID {
			return items[i].FileID < items[j].FileID
		}
		return items[i].Version < items[j].Version
	})
	raw, err := json.Marshal(items)
	if err != nil {
		return MaterialReferenceProjection{}, ErrReferenceInvalid
	}
	digest := sha256.Sum256(raw)
	return MaterialReferenceProjection{SchemaVersion: "1", SourceApplication: "approval-audit-results", ID: value.ID, TenantID: value.TenantID, ProjectID: value.Project.ID, ResourceType: "approval_audit_result", ResultKind: value.Kind, Status: value.State, Revision: value.Revision, Digest: hex.EncodeToString(digest[:]), Items: items}, nil
}
