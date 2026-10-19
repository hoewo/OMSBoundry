package design

import "strings"

const typeDefinitionVersionV1 int64 = 1

// TypeDefinition is a code-owned, immutable production workflow contract.
// Callers receive a copy so runtime code cannot mutate the registered Base.
type TypeDefinition struct {
	Key                    string
	Version                int64
	RequiredProjectTypeKey string
	Stages                 []StageDefinition
}

var registeredTypeDefinitions = map[string]map[int64]TypeDefinition{
	DesignTypeKey: {
		typeDefinitionVersionV1: {
			Key:                    DesignTypeKey,
			Version:                typeDefinitionVersionV1,
			RequiredProjectTypeKey: "design",
			Stages: []StageDefinition{
				{Key: "scheme", Sequence: 10, Skippable: true, RequiredRequirementKeys: []string{RequirementPrimary}},
				{Key: "preliminary-design", Sequence: 20, Skippable: true, RequiredRequirementKeys: []string{RequirementPrimary}},
				{Key: "construction-drawing", Sequence: 30, Skippable: false, RequiredRequirementKeys: []string{RequirementPrimary}},
				{Key: "construction-support", Sequence: 40, Skippable: false, RequiredRequirementKeys: []string{RequirementPrimary}},
			},
		},
	},
	ConsultationTypeKey: {
		typeDefinitionVersionV1: {
			Key:                    ConsultationTypeKey,
			Version:                typeDefinitionVersionV1,
			RequiredProjectTypeKey: "consultation",
			Stages: []StageDefinition{
				{Key: "scheme", Sequence: 10, Skippable: true, RequiredRequirementKeys: []string{RequirementPrimary}},
				{Key: "report", Sequence: 20, Skippable: false, RequiredRequirementKeys: []string{RequirementPrimary}},
			},
		},
	},
}

func LatestTypeDefinition(typeKey string) (TypeDefinition, bool) {
	typeKey = strings.ToLower(strings.TrimSpace(typeKey))
	versions, ok := registeredTypeDefinitions[typeKey]
	if !ok {
		return TypeDefinition{}, false
	}
	var latest TypeDefinition
	found := false
	for version, definition := range versions {
		if !found || version > latest.Version {
			latest = definition
			found = true
		}
	}
	return cloneTypeDefinition(latest), found
}

func RegisteredTypeDefinition(typeKey string, version int64) (TypeDefinition, bool) {
	typeKey = strings.ToLower(strings.TrimSpace(typeKey))
	if IsStageMaterialDefinitionVersion(version) {
		if typeKey != StageMaterialTypeKey {
			return TypeDefinition{}, false
		}
		stages := []StageDefinition{{Key: "scheme", Sequence: 10, RequiredRequirementKeys: []string{MaterialMain}}}
		if version >= StageMaterialDefinitionVersionV3 {
			stages = append(stages, StageDefinition{Key: "preliminary-design", Sequence: 20, RequiredRequirementKeys: []string{MaterialMain}})
		}
		if version >= StageMaterialDefinitionVersionV4 {
			stages = append(stages, StageDefinition{Key: "construction-drawing", Sequence: 30, RequiredRequirementKeys: []string{MaterialMain}})
		}
		if version >= StageMaterialDefinitionVersionV5 {
			stages = append(stages,
				StageDefinition{Key: "change", Sequence: 40, Skippable: true, RequiredRequirementKeys: []string{MaterialMain}},
				StageDefinition{Key: "completion", Sequence: 50, RequiredRequirementKeys: []string{MaterialMain}},
			)
		}
		if version >= StageMaterialDefinitionVersion {
			stages = append(stages,
				StageDefinition{Key: "test", Sequence: 60, RequiredRequirementKeys: []string{MaterialMain}},
				StageDefinition{Key: "report", Sequence: 70, RequiredRequirementKeys: []string{MaterialMain}},
			)
		}
		// V2 remains scheme-only, V3 adds preliminary design, V4 adds
		// construction drawing, V5 adds optional change plus required
		// completion, and V6 adds type-scoped test/report. Older meanings
		// never change in place.
		return TypeDefinition{Key: StageMaterialTypeKey, Version: version, Stages: stages}, true
	}
	versions, ok := registeredTypeDefinitions[typeKey]
	if !ok {
		return TypeDefinition{}, false
	}
	definition, ok := versions[version]
	return cloneTypeDefinition(definition), ok
}

// An explicit extension version never becomes the default for old callers.
func createDefinition(input CreateInput) (TypeDefinition, bool) {
	if input.TypeDefinitionVersion == 0 {
		return LatestTypeDefinition(input.TypeKey)
	}
	return RegisteredTypeDefinition(input.TypeKey, input.TypeDefinitionVersion)
}

func StageDefinitionFor(typeKey string, version int64, stageKey string) (StageDefinition, bool) {
	definition, ok := RegisteredTypeDefinition(typeKey, version)
	if !ok {
		return StageDefinition{}, false
	}
	stageKey = strings.ToLower(strings.TrimSpace(stageKey))
	for _, stage := range definition.Stages {
		if stage.Key == stageKey {
			return stage, true
		}
	}
	return StageDefinition{}, false
}

func IsKnownStageKey(stageKey string) bool {
	stageKey = strings.ToLower(strings.TrimSpace(stageKey))
	if definition, ok := RegisteredTypeDefinition(StageMaterialTypeKey, StageMaterialDefinitionVersion); ok {
		for _, stage := range definition.Stages {
			if stage.Key == stageKey {
				return true
			}
		}
	}
	for _, versions := range registeredTypeDefinitions {
		for _, definition := range versions {
			for _, stage := range definition.Stages {
				if stage.Key == stageKey {
					return true
				}
			}
		}
	}
	return false
}

func cloneTypeDefinition(definition TypeDefinition) TypeDefinition {
	definition.Stages = append([]StageDefinition(nil), definition.Stages...)
	for index := range definition.Stages {
		definition.Stages[index].RequiredRequirementKeys = append([]string(nil), definition.Stages[index].RequiredRequirementKeys...)
	}
	return definition
}
