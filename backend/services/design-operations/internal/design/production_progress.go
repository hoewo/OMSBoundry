package design

import (
	"context"
	"errors"
	"fmt"
)

const ProductionProgressProfile = "reference-oa-production-progress-v1"

// ProductionStageProgress is a read projection over Production-owned stage
// facts. It deliberately contains no current Project fields: type
// applicability has already been frozen at the Work Item boundary.
type ProductionStageProgress struct {
	StageKey        string  `json:"stage_key"`
	Sequence        int     `json:"sequence"`
	Required        bool    `json:"required"`
	Status          string  `json:"status"`
	MainFileCount   int     `json:"main_file_count"`
	ReviewFileCount int     `json:"review_file_count"`
	Score           *string `json:"score"`
}

type ProductionWorkItemProgress struct {
	WorkItemID            string                    `json:"work_item_id"`
	TypeDefinitionVersion int64                     `json:"type_definition_version"`
	ProjectSourceRevision int64                     `json:"project_source_revision"`
	Revision              int64                     `json:"revision"`
	Status                string                    `json:"status"`
	CurrentStageKey       *string                   `json:"current_stage_key"`
	ApplicableStageCount  int                       `json:"applicable_stage_count"`
	CompletedStageCount   int                       `json:"completed_stage_count"`
	RequiredStageCount    int                       `json:"required_stage_count"`
	RequiredCompleteCount int                       `json:"required_complete_count"`
	ProductionComplete    bool                      `json:"production_complete"`
	ProjectTypeKeys       []string                  `json:"project_type_keys"`
	TypeProgress          []ProductionTypeProgress  `json:"type_progress"`
	Stages                []ProductionStageProgress `json:"stages"`
}

// ProductionTypeProgress preserves the original OA's one progress line per
// selected project type while every line consumes the same Production-owned
// stage facts. Shared stages are views, never duplicated writable facts.
type ProductionTypeProgress struct {
	ProjectTypeKey        string   `json:"project_type_key"`
	ApplicableStageCount  int      `json:"applicable_stage_count"`
	CompletedStageCount   int      `json:"completed_stage_count"`
	RequiredStageCount    int      `json:"required_stage_count"`
	RequiredCompleteCount int      `json:"required_complete_count"`
	Complete              bool     `json:"complete"`
	StageKeys             []string `json:"stage_keys"`
}

type ProductionProgressProjection struct {
	SchemaVersion         string                       `json:"schema_version"`
	Profile               string                       `json:"profile"`
	TenantID              string                       `json:"tenant_id"`
	ProjectID             string                       `json:"project_id"`
	ApplicableStageCount  int                          `json:"applicable_stage_count"`
	CompletedStageCount   int                          `json:"completed_stage_count"`
	RequiredStageCount    int                          `json:"required_stage_count"`
	RequiredCompleteCount int                          `json:"required_complete_count"`
	ProductionComplete    bool                         `json:"production_complete"`
	Items                 []ProductionWorkItemProgress `json:"items"`
}

// ProductionProgress returns one bounded owner-native read for the OA
// overview. It never validates Project again and therefore cannot turn a page
// read into a synchronous Production -> Project dependency.
func (s *Service) ProductionProgress(ctx context.Context, tenant, project, typeKey string) (ProductionProgressProjection, error) {
	store, err := s.stageStore()
	if err != nil {
		return ProductionProgressProjection{}, err
	}
	if !canonicalReferenceID(tenant) || !canonicalReferenceID(project) || typeKey != StageMaterialTypeKey {
		return ProductionProgressProjection{}, fmt.Errorf("invalid production progress scope")
	}
	workItems, err := store.StageWorkItems(ctx, tenant, project, typeKey)
	if err != nil {
		return ProductionProgressProjection{}, err
	}
	// The original-OA stage set is one active Production truth per Project.
	// A legacy task is deliberately returned by the store as well, so more
	// than one row is a compatibility conflict rather than progress to merge.
	if len(workItems) > 1 {
		return ProductionProgressProjection{}, ErrTypeInvalid
	}
	result := ProductionProgressProjection{
		SchemaVersion: "1.0",
		Profile:       ProductionProgressProfile,
		TenantID:      tenant,
		ProjectID:     project,
		Items:         []ProductionWorkItemProgress{},
	}
	for _, item := range workItems {
		definition, ok := RegisteredTypeDefinition(item.TypeKey, item.TypeDefinitionVersion)
		if !ok || item.TenantID != tenant || item.ProjectID != project {
			return ProductionProgressProjection{}, ErrTypeInvalid
		}
		progress := ProductionWorkItemProgress{
			WorkItemID:            item.ID,
			TypeDefinitionVersion: item.TypeDefinitionVersion,
			ProjectSourceRevision: item.ProjectSourceRevision,
			Revision:              item.Revision,
			Status:                item.Status,
			CurrentStageKey:       item.CurrentStageKey,
			ProductionComplete:    item.Status == StatusCompleted,
			ProjectTypeKeys:       []string{},
			TypeProgress:          []ProductionTypeProgress{},
			Stages:                []ProductionStageProgress{},
		}
		for _, stage := range definition.Stages {
			collection, err := store.StageMaterials(ctx, tenant, item.ID, stage.Key)
			if errors.Is(err, ErrStageInvalid) {
				// V6 project-type applicability omits a stage from this Work Item;
				// it is not a zero-file pending stage.
				continue
			}
			if err != nil {
				return ProductionProgressProjection{}, err
			}
			if collection.WorkItem.ID != item.ID || collection.WorkItem.TenantID != tenant || collection.WorkItem.ProjectID != project || collection.Stage.WorkItemID != item.ID || collection.Stage.Key != stage.Key {
				return ProductionProgressProjection{}, ErrTypeInvalid
			}
			if item.TypeDefinitionVersion == StageMaterialDefinitionVersion {
				types, typeErr := normalizeStageMaterialProjectTypes(collection.ProjectTypeSnapshot)
				if typeErr != nil {
					return ProductionProgressProjection{}, ErrTypeInvalid
				}
				if len(progress.ProjectTypeKeys) == 0 {
					progress.ProjectTypeKeys = types
				} else if !sameStrings(progress.ProjectTypeKeys, types) {
					return ProductionProgressProjection{}, ErrTypeInvalid
				}
			}
			completed := collection.Stage.Status == StageCompleted
			row := ProductionStageProgress{
				StageKey:        stage.Key,
				Sequence:        stage.Sequence,
				Required:        !stage.Skippable,
				Status:          collection.Stage.Status,
				MainFileCount:   len(collection.MainFiles),
				ReviewFileCount: len(collection.ReviewSheets),
				Score:           collection.Score,
			}
			progress.Stages = append(progress.Stages, row)
			progress.ApplicableStageCount++
			result.ApplicableStageCount++
			if completed {
				progress.CompletedStageCount++
				result.CompletedStageCount++
			}
			if row.Required {
				progress.RequiredStageCount++
				result.RequiredStageCount++
				if completed {
					progress.RequiredCompleteCount++
					result.RequiredCompleteCount++
				}
			}
		}
		for _, projectType := range progress.ProjectTypeKeys {
			line := ProductionTypeProgress{ProjectTypeKey: projectType, StageKeys: []string{}}
			for _, stage := range progress.Stages {
				applicable, applyErr := stageMaterialApplicable(item.TypeDefinitionVersion, stage.StageKey, []string{projectType})
				if applyErr != nil {
					return ProductionProgressProjection{}, ErrTypeInvalid
				}
				if !applicable {
					continue
				}
				line.StageKeys = append(line.StageKeys, stage.StageKey)
				line.ApplicableStageCount++
				if stage.Status == StageCompleted {
					line.CompletedStageCount++
				}
				if stage.Required {
					line.RequiredStageCount++
					if stage.Status == StageCompleted {
						line.RequiredCompleteCount++
					}
				}
			}
			line.Complete = line.RequiredStageCount > 0 && line.RequiredCompleteCount == line.RequiredStageCount
			progress.TypeProgress = append(progress.TypeProgress, line)
		}
		result.Items = append(result.Items, progress)
	}
	result.ProductionComplete = len(result.Items) > 0
	for _, item := range result.Items {
		if !item.ProductionComplete {
			result.ProductionComplete = false
			break
		}
	}
	return result, nil
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
