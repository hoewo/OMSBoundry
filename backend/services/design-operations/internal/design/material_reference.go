package design

import (
	"context"
	"sort"
	"strings"
	"unicode/utf8"
)

// MaterialReferenceProjection publishes Production-owned current references,
// not file bytes, storage coordinates, or a copy of another application's facts.
type MaterialReferenceProjection struct {
	SchemaVersion     string                   `json:"schema_version"`
	SourceApplication string                   `json:"source_application"`
	ID                string                   `json:"id"`
	TenantID          string                   `json:"tenant_id"`
	ProjectID         string                   `json:"project_id"`
	ResourceType      string                   `json:"resource_type"`
	Status            string                   `json:"status"`
	Revision          int64                    `json:"revision"`
	Digest            string                   `json:"digest"`
	Items             []MaterialProjectionItem `json:"items"`
}

type MaterialProjectionItem struct {
	RelationKey            string `json:"relation_key"`
	FileID                 string `json:"file_id"`
	Version                int    `json:"version"`
	EvidenceSourceRevision int64  `json:"evidence_source_revision"`
	DisplayName            string `json:"display_name"`
}

func (s *Service) ReferenceMaterials(ctx context.Context, tenant, id string) (MaterialReferenceProjection, error) {
	if !canonicalReferenceID(tenant) || !canonicalReferenceID(id) {
		return MaterialReferenceProjection{}, ErrReferenceInvalid
	}
	// Get would refresh upstream availability. This port intentionally reads only
	// our owned facts, even while Project, People, or Evidence is unavailable.
	details, err := s.GetOwned(ctx, tenant, id)
	if err != nil {
		return MaterialReferenceProjection{}, err
	}
	item := details.WorkItem
	// The v1 single-slot projection cannot truthfully represent either version of
	// the native multi-file collection. Callers must use the collection port.
	if IsStageMaterialDefinitionVersion(item.TypeDefinitionVersion) {
		return MaterialReferenceProjection{}, ErrTypeInvalid
	}
	definition, known := RegisteredTypeDefinition(item.TypeKey, item.TypeDefinitionVersion)
	if !known || item.TypeKey != definition.Key || item.TenantID != tenant || item.ID != id || !canonicalReferenceID(item.ProjectID) || item.Revision < 1 || !materialWorkItemStatus(item.Status) || len(details.Stages) != len(definition.Stages) {
		return MaterialReferenceProjection{}, ErrReferenceInvalid
	}
	stages := make(map[string]Stage, len(details.Stages))
	for _, stage := range details.Stages {
		stageDefinition, ok := StageDefinitionFor(item.TypeKey, item.TypeDefinitionVersion, stage.Key)
		_, duplicate := stages[stage.Key]
		if !ok || stage.Key != stageDefinition.Key || stage.Sequence != stageDefinition.Sequence || stage.TenantID != tenant || stage.WorkItemID != id || duplicate || !materialStageStatus(stage.Status) || (stage.Status == StageNotApplicable && !stageDefinition.Skippable) {
			return MaterialReferenceProjection{}, ErrReferenceInvalid
		}
		stages[stage.Key] = stage
	}
	items := make([]MaterialProjectionItem, 0)
	seen := make(map[string]bool)
	currentCount := 0
	for _, material := range details.Deliverables {
		if material.TenantID != tenant || material.WorkItemID != id || (material.SupersededAt == nil) != (material.SupersededRevision == nil) {
			return MaterialReferenceProjection{}, ErrReferenceInvalid
		}
		if material.SupersededAt != nil {
			continue
		}
		currentCount++
		stage, exists := stages[material.StageKey]
		stageDefinition, ok := StageDefinitionFor(item.TypeKey, item.TypeDefinitionVersion, material.StageKey)
		relation := material.StageKey + "." + material.RequirementKey
		if currentCount > 100 || !exists || !ok || !requiresRequirement(stageDefinition, material.RequirementKey) || seen[relation] || !canonicalReferenceID(material.EvidenceFileID) || material.EvidenceVersion < 1 || material.EvidenceSourceRevision < 1 || material.CreatedRevision < 1 || material.CreatedRevision > item.Revision || material.DisplaySnapshot != strings.TrimSpace(material.DisplaySnapshot) || utf8.RuneCountInString(material.DisplaySnapshot) < 1 || utf8.RuneCountInString(material.DisplaySnapshot) > 255 {
			return MaterialReferenceProjection{}, ErrReferenceInvalid
		}
		seen[relation] = true
		if stage.Status == StageNotApplicable {
			continue
		}
		items = append(items, MaterialProjectionItem{RelationKey: relation, FileID: material.EvidenceFileID, Version: material.EvidenceVersion, EvidenceSourceRevision: material.EvidenceSourceRevision, DisplayName: material.DisplaySnapshot})
	}
	sort.Slice(items, func(left, right int) bool {
		if items[left].RelationKey != items[right].RelationKey {
			return items[left].RelationKey < items[right].RelationKey
		}
		if items[left].FileID != items[right].FileID {
			return items[left].FileID < items[right].FileID
		}
		return items[left].Version < items[right].Version
	})
	digest, err := requestHash(items)
	if err != nil {
		return MaterialReferenceProjection{}, err
	}
	return MaterialReferenceProjection{SchemaVersion: "1", SourceApplication: "production-operations", ID: id, TenantID: tenant, ProjectID: item.ProjectID, ResourceType: "production-work-item", Status: item.Status, Revision: item.Revision, Digest: digest, Items: items}, nil
}

func canonicalReferenceID(id string) bool {
	return ValidUUID(id) && id == strings.ToLower(strings.TrimSpace(id))
}

func materialWorkItemStatus(status string) bool {
	return status == StatusDraft || status == StatusInProgress || status == StatusCompleted || status == StatusCancelled
}

func materialStageStatus(status string) bool {
	return status == StagePending || status == StageReady || status == StageCompleted || status == StageNotApplicable
}
