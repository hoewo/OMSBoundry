package costoutsourcing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

// This wire contains only source-owned file references, not cost, provider,
// contract, assessment, suggestion or storage-object facts.
type MaterialReferenceProjection struct {
	SchemaVersion     string                  `json:"schema_version"`
	SourceApplication string                  `json:"source_application"`
	ID                string                  `json:"id"`
	TenantID          string                  `json:"tenant_id"`
	ProjectID         string                  `json:"project_id"`
	ResourceType      string                  `json:"resource_type"`
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

func CostMaterialReference(value CostEntry) (MaterialReferenceProjection, error) {
	return materialReference(value.ID, value.TenantID, value.Project.ID, "cost_entry", value.State, value.Revision, value.Evidence)
}
func EngagementMaterialReference(value Engagement) (MaterialReferenceProjection, error) {
	return materialReference(value.ID, value.TenantID, value.Project.ID, "outsourcing_engagement", value.State, value.Revision, value.Evidence)
}
func materialReference(id, tenant, project, kind, state string, revision int64, files []EvidenceSnapshot) (MaterialReferenceProjection, error) {
	canonicalID := func(id string) bool { return validUUID(id) && id == normalizeUUID(id) }
	if !canonicalID(id) || !canonicalID(tenant) || !canonicalID(project) || revision < 1 || (state != StateRecorded && state != StateWithdrawn) || len(files) > 100 {
		return MaterialReferenceProjection{}, ErrReferenceInvalid
	}
	items := make([]MaterialReferenceItem, 0, len(files))
	seen := map[EvidenceRef]bool{}
	for _, file := range files {
		if !canonicalID(file.FileID) || file.Version < 1 || !validText(file.Slot, 1, 100) || file.Slot != strings.TrimSpace(file.Slot) || !validText(file.DisplayName, 1, 500) || file.DisplayName != strings.TrimSpace(file.DisplayName) || seen[file.EvidenceRef] {
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
	return MaterialReferenceProjection{SchemaVersion: "1", SourceApplication: "cost-outsourcing", ID: id, TenantID: tenant, ProjectID: project, ResourceType: kind, Status: state, Revision: revision, Digest: hex.EncodeToString(digest[:]), Items: items}, nil
}
