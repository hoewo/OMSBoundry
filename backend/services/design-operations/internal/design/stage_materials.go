package design

import (
	"context"
	"fmt"
	"math/big"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	StageMaterialProfile = "reference-oa-stage-materials-v1"
	// This is a Production-owned project/stage collection, not a project type.
	// All original OA project types share this single stage-set identity. Each
	// immutable definition version adds only explicitly registered OA stages.
	StageMaterialTypeKey                   = "reference-oa-stage-set-v1"
	StageMaterialDefinitionVersionV2 int64 = 2
	StageMaterialDefinitionVersionV3 int64 = 3
	StageMaterialDefinitionVersionV4 int64 = 4
	StageMaterialDefinitionVersionV5 int64 = 5
	StageMaterialDefinitionVersion   int64 = 6
	MaterialMain                           = "main-file"
	MaterialReview                         = "review-sheet"
	MaxStageMaterialBatch                  = 20
	MaxStageMaterialCurrent                = 100
)

type StageFileInput struct {
	FileID          string `json:"file_id"`
	Version         int    `json:"version"`
	SourceRevision  int64  `json:"source_revision"`
	DisplaySnapshot string `json:"-"`
}
type StageMaterialInput struct {
	RevisionInput
	MainFiles    []StageFileInput `json:"main_files"`
	ReviewSheets []StageFileInput `json:"review_sheets"`
	Score        *string          `json:"score,omitempty"`
}
type StageMaterial struct {
	ID              string    `json:"id"`
	TenantID        string    `json:"tenant_id"`
	WorkItemID      string    `json:"work_item_id"`
	StageKey        string    `json:"stage_key"`
	Kind            string    `json:"kind"`
	FileID          string    `json:"file_id"`
	Version         int       `json:"version"`
	SourceRevision  int64     `json:"source_revision"`
	DisplaySnapshot string    `json:"display_name"`
	CreatedRevision int64     `json:"created_revision"`
	BatchPosition   int       `json:"batch_position"`
	CreatedAt       time.Time `json:"created_at"`
}
type StageMaterialCollection struct {
	SchemaVersion       string          `json:"schema_version"`
	Profile             string          `json:"profile"`
	WorkItem            WorkItem        `json:"work_item"`
	Stage               Stage           `json:"stage"`
	ProjectTypeSnapshot []string        `json:"project_type_keys,omitempty"`
	MainFiles           []StageMaterial `json:"main_files"`
	ReviewSheets        []StageMaterial `json:"review_sheets"`
	Score               *string         `json:"score"`
}
type StageMaterialStore interface {
	StageMaterials(context.Context, string, string, string) (StageMaterialCollection, error)
	RegisterStageMaterials(context.Context, Actor, string, string, StageMaterialInput, string) (StageMaterialCollection, error)
	WithdrawStageMaterial(context.Context, Actor, string, string, string, ReasonInput, string) (StageMaterialCollection, error)
	StageWorkItems(context.Context, string, string, string) ([]WorkItem, error)
}
type stageDefinitionUpgradeStore interface {
	UpgradeStageMaterialDefinition(context.Context, Actor, string, DefinitionUpgradeInput, string) (Details, error)
}

func IsStageMaterialDefinitionVersion(version int64) bool {
	return version >= StageMaterialDefinitionVersionV2 && version <= StageMaterialDefinitionVersion
}

func isMainOnlyMaterialStage(stage string) bool {
	return stage == "change" || stage == "completion" || stage == "test" || stage == "report"
}

func validateStageMaterialShape(stage string, input StageMaterialInput) error {
	if isMainOnlyMaterialStage(stage) && (len(input.ReviewSheets) > 0 || input.Score != nil) {
		return fmt.Errorf("stage accepts main files only")
	}
	return nil
}

func validateStageMaterialDisplayName(stage, name string) error {
	if !isMainOnlyMaterialStage(stage) {
		return nil
	}
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(name)))
	allowed := map[string]bool{".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".png": true, ".jpg": true, ".jpeg": true, ".dwg": true, ".dxf": true}
	if stage == "completion" {
		allowed[".zip"] = true
		allowed[".rar"] = true
	}
	if !allowed[ext] {
		return fmt.Errorf("unsupported auxiliary stage file type")
	}
	return nil
}

var stageMaterialProjectTypes = map[string]bool{
	"design": true, "consultation": true, "survey": true, "mapping": true,
}

func normalizeStageMaterialProjectTypes(values []string) ([]string, error) {
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if stageMaterialProjectTypes[value] {
			seen[value] = true
		}
	}
	result := make([]string, 0, len(seen))
	for _, value := range []string{"consultation", "design", "mapping", "survey"} {
		if seen[value] {
			result = append(result, value)
		}
	}
	if len(result) == 0 {
		return nil, ErrReferenceInvalid
	}
	return result, nil
}

func stageMaterialApplicable(version int64, stage string, projectTypes []string) (bool, error) {
	if version < StageMaterialDefinitionVersion {
		return true, nil
	}
	types, err := normalizeStageMaterialProjectTypes(projectTypes)
	if err != nil {
		return false, ErrTypeInvalid
	}
	has := func(key string) bool {
		for _, value := range types {
			if value == key {
				return true
			}
		}
		return false
	}
	switch stage {
	case "scheme":
		return true, nil
	case "preliminary-design", "construction-drawing", "change", "completion":
		return has("design"), nil
	case "test":
		return has("survey"), nil
	case "report":
		return has("consultation") || has("survey") || has("mapping"), nil
	default:
		return false, ErrStageInvalid
	}
}

var scorePattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?$`)

func normalizeStageScore(input *string) (*string, error) {
	if input == nil || strings.TrimSpace(*input) == "" {
		return nil, nil
	}
	v := strings.TrimSpace(*input)
	if len(v) > 128 || !scorePattern.MatchString(v) {
		return nil, fmt.Errorf("invalid score decimal")
	}
	n, ok := new(big.Rat).SetString(v)
	if !ok || n.Sign() < 0 || n.Cmp(big.NewRat(100, 1)) > 0 {
		return nil, fmt.Errorf("invalid score range")
	}
	parts := strings.SplitN(v, ".", 2)
	v = strings.TrimLeft(parts[0], "0")
	if v == "" {
		v = "0"
	}
	if len(parts) == 2 {
		fraction := strings.TrimRight(parts[1], "0")
		if fraction != "" {
			v += "." + fraction
		}
	}
	return &v, nil
}
func normalizeStageMaterialInput(in StageMaterialInput) (StageMaterialInput, error) {
	var err error
	in.CommandID, err = normalizeCommand(in.CommandID, in.ExpectedRevision)
	if err != nil {
		return in, err
	}
	in.Score, err = normalizeStageScore(in.Score)
	if err != nil {
		return in, err
	}
	// Clone caller-owned slices before injecting verified snapshots.
	in.MainFiles = append([]StageFileInput{}, in.MainFiles...)
	in.ReviewSheets = append([]StageFileInput{}, in.ReviewSheets...)
	for _, files := range [][]StageFileInput{in.MainFiles, in.ReviewSheets} {
		if len(files) > MaxStageMaterialBatch {
			return in, fmt.Errorf("invalid material batch limit")
		}
		seen := map[string]bool{}
		for i := range files {
			files[i].FileID = strings.ToLower(strings.TrimSpace(files[i].FileID))
			files[i].DisplaySnapshot = ""
			key := fmt.Sprintf("%s:%d", files[i].FileID, files[i].Version)
			if !ValidUUID(files[i].FileID) || files[i].Version < 1 || files[i].SourceRevision != int64(files[i].Version) || seen[key] {
				return in, fmt.Errorf("invalid or duplicate stage file")
			}
			seen[key] = true
		}
	}
	if len(in.MainFiles)+len(in.ReviewSheets) == 0 && in.Score == nil {
		return in, fmt.Errorf("invalid empty material intent")
	}
	return in, nil
}
func (s *Service) stageStore() (StageMaterialStore, error) {
	if s.stageMaterialProfile != StageMaterialProfile {
		return nil, ErrTypeInvalid
	}
	store, ok := s.store.(StageMaterialStore)
	if !ok {
		return nil, ErrTypeInvalid
	}
	return store, nil
}
func (s *Service) StageWorkItems(ctx context.Context, tenant, project, typeKey string) ([]WorkItem, error) {
	store, err := s.stageStore()
	if err != nil {
		return nil, err
	}
	if !canonicalReferenceID(tenant) || !canonicalReferenceID(project) {
		return nil, fmt.Errorf("invalid stage project")
	}
	if typeKey != StageMaterialTypeKey {
		return nil, ErrTypeInvalid
	}
	return store.StageWorkItems(ctx, tenant, project, typeKey)
}
func (s *Service) StageMaterials(ctx context.Context, tenant, id, stage string) (StageMaterialCollection, error) {
	store, err := s.stageStore()
	if err != nil {
		return StageMaterialCollection{}, err
	}
	if !canonicalReferenceID(tenant) || !canonicalReferenceID(id) || !IsKnownStageKey(stage) {
		return StageMaterialCollection{}, fmt.Errorf("invalid stage material resource")
	}
	return store.StageMaterials(ctx, tenant, id, stage)
}
func (s *Service) StageMaterialHistory(ctx context.Context, tenant, id string, after int64, limit int) (HistoryPage, error) {
	if _, err := s.stageStore(); err != nil {
		return HistoryPage{}, err
	}
	owned, err := s.GetOwned(ctx, tenant, id)
	if err != nil {
		return HistoryPage{}, err
	}
	if !IsStageMaterialDefinitionVersion(owned.WorkItem.TypeDefinitionVersion) {
		return HistoryPage{}, ErrTypeInvalid
	}
	return s.History(ctx, tenant, id, after, limit)
}

func (s *Service) UpgradeStageMaterialDefinition(ctx context.Context, actor Actor, id string, input DefinitionUpgradeInput) (Details, error) {
	_, err := s.stageStore()
	if err != nil {
		return Details{}, err
	}
	input.CommandID, err = normalizeCommand(input.CommandID, input.ExpectedRevision)
	if err != nil || !canonicalReferenceID(actor.TenantID) || !canonicalReferenceID(actor.ID) || !canonicalReferenceID(id) || input.TargetVersion < StageMaterialDefinitionVersionV3 || input.TargetVersion > StageMaterialDefinitionVersion {
		return Details{}, fmt.Errorf("invalid stage definition upgrade")
	}
	hash, err := requestHash(struct {
		Kind, ID string
		Input    DefinitionUpgradeInput
	}{"stage-material-definition-upgrade-v1", id, input})
	if err != nil {
		return Details{}, err
	}
	var replay Details
	if found, err := s.store.Replay(ctx, actor.TenantID, input.CommandID, hash, &replay); err != nil || found {
		return replay, err
	}
	if input.TargetVersion == StageMaterialDefinitionVersion {
		owned, err := s.GetOwned(ctx, actor.TenantID, id)
		if err != nil {
			return Details{}, err
		}
		project, err := s.refs.ValidateProject(ctx, actor.TenantID, owned.WorkItem.ProjectID, owned.WorkItem.ProjectSourceRevision)
		if err != nil {
			return Details{}, err
		}
		if project.ID != owned.WorkItem.ProjectID || project.TenantID != actor.TenantID || project.Revision != owned.WorkItem.ProjectSourceRevision || project.Status != "active" {
			return Details{}, ErrReferenceInvalid
		}
		input.ProjectTypeSnapshot, err = normalizeStageMaterialProjectTypes(project.TypeKeys)
		if err != nil {
			return Details{}, err
		}
	}
	store, ok := s.store.(stageDefinitionUpgradeStore)
	if !ok {
		return Details{}, ErrTypeInvalid
	}
	return store.UpgradeStageMaterialDefinition(ctx, actor, id, input, hash)
}
func (s *Service) RegisterStageMaterials(ctx context.Context, actor Actor, id, stage string, input StageMaterialInput) (StageMaterialCollection, error) {
	store, err := s.stageStore()
	if err != nil {
		return StageMaterialCollection{}, err
	}
	if !canonicalReferenceID(actor.TenantID) || !canonicalReferenceID(actor.ID) || !canonicalReferenceID(id) || !IsKnownStageKey(stage) {
		return StageMaterialCollection{}, fmt.Errorf("invalid stage material resource")
	}
	input, err = normalizeStageMaterialInput(input)
	if err != nil {
		return StageMaterialCollection{}, err
	}
	if err := validateStageMaterialShape(stage, input); err != nil {
		return StageMaterialCollection{}, err
	}
	hash, err := requestHash(struct {
		Kind, ID, Stage string
		Input           StageMaterialInput
	}{"stage-material-register-v1", id, stage, input})
	if err != nil {
		return StageMaterialCollection{}, err
	}
	var replay StageMaterialCollection
	if found, err := s.store.Replay(ctx, actor.TenantID, input.CommandID, hash, &replay); err != nil || found {
		return replay, err
	}
	current, err := store.StageMaterials(ctx, actor.TenantID, id, stage)
	if err != nil {
		return StageMaterialCollection{}, err
	}
	if err := checkMutable(current.WorkItem, input.ExpectedRevision); err != nil {
		return StageMaterialCollection{}, err
	}
	if len(current.MainFiles)+len(input.MainFiles) == 0 {
		return StageMaterialCollection{}, ErrCompletionGaps
	}
	for index, group := range [][]StageFileInput{input.MainFiles, input.ReviewSheets} {
		active := current.MainFiles
		if index == 1 {
			active = current.ReviewSheets
		}
		if len(active)+len(group) > MaxStageMaterialCurrent {
			return StageMaterialCollection{}, fmt.Errorf("invalid current material limit")
		}
		for i := range group {
			for _, old := range active {
				if old.FileID == group[i].FileID && old.Version == group[i].Version {
					return StageMaterialCollection{}, ErrStateConflict
				}
			}
			ref, err := s.refs.ValidateEvidence(ctx, actor.TenantID, group[i].FileID, group[i].Version, group[i].SourceRevision)
			if err != nil {
				return StageMaterialCollection{}, err
			}
			if ref.TenantID != actor.TenantID || ref.FileID != group[i].FileID || ref.Status != "available" || ref.Version != group[i].Version || ref.Revision != group[i].SourceRevision || strings.TrimSpace(ref.DisplayName) == "" || len([]rune(ref.DisplayName)) > 255 {
				return StageMaterialCollection{}, ErrReferenceInvalid
			}
			if err := validateStageMaterialDisplayName(stage, ref.DisplayName); err != nil {
				return StageMaterialCollection{}, ErrReferenceInvalid
			}
			group[i].DisplaySnapshot = strings.TrimSpace(ref.DisplayName)
		}
	}
	return store.RegisterStageMaterials(ctx, actor, id, stage, input, hash)
}
func (s *Service) WithdrawStageMaterial(ctx context.Context, actor Actor, id, stage, referenceID string, input ReasonInput) (StageMaterialCollection, error) {
	store, err := s.stageStore()
	if err != nil {
		return StageMaterialCollection{}, err
	}
	input.CommandID, err = normalizeCommand(input.CommandID, input.ExpectedRevision)
	input.Reason = strings.TrimSpace(input.Reason)
	if err != nil || !canonicalReferenceID(actor.TenantID) || !canonicalReferenceID(actor.ID) || !canonicalReferenceID(id) || !canonicalReferenceID(referenceID) || !IsKnownStageKey(stage) || input.Reason == "" || len([]rune(input.Reason)) > 1000 {
		return StageMaterialCollection{}, fmt.Errorf("invalid stage material withdrawal")
	}
	hash, _ := requestHash(struct {
		Kind, ID, Stage, ReferenceID string
		Input                        ReasonInput
	}{"stage-material-withdraw-v1", id, stage, referenceID, input})
	var replay StageMaterialCollection
	if found, err := s.store.Replay(ctx, actor.TenantID, input.CommandID, hash, &replay); err != nil || found {
		return replay, err
	}
	// Ending a local business reference never calls Evidence or deletes bytes.
	return store.WithdrawStageMaterial(ctx, actor, id, stage, referenceID, input, hash)
}
