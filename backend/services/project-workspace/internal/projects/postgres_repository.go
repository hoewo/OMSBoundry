package projects

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) (*PostgresStore, error) {
	if pool == nil {
		return nil, fmt.Errorf("database pool is required")
	}
	return &PostgresStore{pool: pool}, nil
}
func (s *PostgresStore) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

func (s *PostgresStore) List(ctx context.Context, tenantID string, limit int) ([]Project, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,tenant_id,project_number,node_type,parent_project_id,sort_order,name,summary,planned_start_date,planned_end_date,project_type_keys,client_party_id,client_party_source_revision,client_party_kind,client_party_legal_name,status,revision,cancellation_reason,created_by,created_at,updated_at FROM projects WHERE tenant_id=$1 ORDER BY updated_at DESC,id DESC LIMIT $2`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Project, 0, limit)
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, project)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *PostgresStore) ReplayCreate(ctx context.Context, tenantID string, input CreateInput) (Project, bool, error) {
	hash, err := createRequestHash(input)
	if err != nil {
		return Project{}, false, err
	}
	var replay Project
	found, status, err := lookupCommand(ctx, s.pool, tenantID, input.CommandID, hash, &replay)
	if errors.Is(err, ErrCommandConflict) || (found && status == 409) {
		return Project{}, false, ErrCommandConflict
	}
	return replay, found, err
}

func (s *PostgresStore) Create(ctx context.Context, actor Actor, input CreateInput) (Project, error) {
	hash, err := createRequestHash(input)
	if err != nil {
		return Project{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Project{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lockCommand(ctx, tx, actor.TenantID, input.CommandID); err != nil {
		return Project{}, err
	}
	var replay Project
	if found, status, err := lookupCommand(ctx, tx, actor.TenantID, input.CommandID, hash, &replay); err != nil {
		return Project{}, err
	} else if found {
		if status == 409 {
			return Project{}, ErrCommandConflict
		}
		return replay, nil
	}
	now := time.Now().UTC()
	project := Project{
		ID: newUUID(), TenantID: actor.TenantID, ProjectNumber: input.ProjectNumber, Name: input.Name, Summary: input.Summary,
		NodeType:         NodeTypeProject,
		PlannedStartDate: input.PlannedStartDate, PlannedEndDate: input.PlannedEndDate, ProjectTypeKeys: input.ProjectTypeKeys,
		ClientParty: input.validatedClientParty,
		Status:      StatusDraft, Revision: 1, CreatedBy: actor.ID, CreatedAt: now, UpdatedAt: now,
	}
	var partyID, partyKind, partyName any
	var partyRevision any
	if project.ClientParty != nil {
		partyID, partyRevision, partyKind, partyName = project.ClientParty.PartyID, project.ClientParty.SourceRevision, project.ClientParty.Kind, project.ClientParty.LegalName
	}
	_, err = tx.Exec(ctx, `INSERT INTO projects(id,tenant_id,project_number,project_number_normalized,node_type,parent_project_id,sort_order,name,summary,planned_start_date,planned_end_date,project_type_keys,client_party_id,client_party_source_revision,client_party_kind,client_party_legal_name,status,revision,created_by,created_at,updated_at) VALUES($1,$2,$3,$4,$5,NULL,0,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$18)`,
		project.ID, project.TenantID, project.ProjectNumber, strings.ToLower(project.ProjectNumber), project.NodeType, project.Name, project.Summary, project.PlannedStartDate, project.PlannedEndDate, project.ProjectTypeKeys, partyID, partyRevision, partyKind, partyName, project.Status, project.Revision, project.CreatedBy, now)
	if err != nil {
		return Project{}, mapWriteError(err)
	}
	if err := insertHistory(ctx, tx, project, actor, input.CommandID, "project_created", projectHistoryPayload(project)); err != nil {
		return Project{}, err
	}
	if err := recordCommand(ctx, tx, actor.TenantID, input.CommandID, hash, 201, project, now); err != nil {
		return Project{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Project{}, err
	}
	return project, nil
}

func (s *PostgresStore) Update(ctx context.Context, actor Actor, id string, input UpdateInput) (Project, error) {
	hash, _ := requestHash(struct {
		Kind, ID string
		Input    UpdateInput
	}{"update", id, input})
	return s.writeProfile(ctx, actor, id, input, hash, "")
}

func (s *PostgresStore) CorrectProfile(ctx context.Context, actor Actor, id string, input ProfileCorrectionInput) (Project, error) {
	if strings.TrimSpace(input.CorrectionReason) == "" || len([]rune(input.CorrectionReason)) > 1000 {
		return Project{}, fmt.Errorf("correction reason is required")
	}
	hash, err := requestHash(struct {
		Kind, ID string
		Input    ProfileCorrectionInput
	}{"profile-correction-v1", id, input})
	if err != nil {
		return Project{}, err
	}
	return s.writeProfile(ctx, actor, id, input.UpdateInput, hash, input.CorrectionReason)
}

// Keep the original update hash, lifecycle and receipt format unchanged.
// Corrections are an independently authorized command, not an expanded PUT.
func (s *PostgresStore) writeProfile(ctx context.Context, actor Actor, id string, input UpdateInput, hash, reason string) (Project, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Project{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lockCommand(ctx, tx, actor.TenantID, input.CommandID); err != nil {
		return Project{}, err
	}
	var replay Project
	if found, _, err := lookupCommand(ctx, tx, actor.TenantID, input.CommandID, hash, &replay); err != nil {
		return Project{}, err
	} else if found {
		return replay, nil
	}
	project, err := loadProjectForUpdate(ctx, tx, actor.TenantID, id)
	if err != nil {
		return Project{}, err
	}
	if project.Revision != input.ExpectedRevision {
		return Project{}, ErrRevisionConflict
	}
	if (reason == "" && project.Status != StatusDraft && project.Status != StatusPendingCompletion) || (reason != "" && project.Status != StatusActive) {
		return Project{}, ErrStateConflict
	}
	if sameDetails(project, input.CreateInput) {
		if err := recordCommand(ctx, tx, actor.TenantID, input.CommandID, hash, 200, project, time.Now().UTC()); err != nil {
			return Project{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Project{}, err
		}
		return project, nil
	}
	project.ProjectNumber, project.Name, project.Summary = input.ProjectNumber, input.Name, input.Summary
	project.PlannedStartDate, project.PlannedEndDate, project.ProjectTypeKeys = input.PlannedStartDate, input.PlannedEndDate, input.ProjectTypeKeys
	project.Revision++
	project.UpdatedAt = time.Now().UTC()
	_, err = tx.Exec(ctx, `UPDATE projects SET project_number=$3,project_number_normalized=$4,name=$5,summary=$6,planned_start_date=$7,planned_end_date=$8,project_type_keys=$9,revision=$10,updated_at=$11 WHERE tenant_id=$1 AND id=$2`,
		actor.TenantID, id, project.ProjectNumber, strings.ToLower(project.ProjectNumber), project.Name, project.Summary, project.PlannedStartDate, project.PlannedEndDate, project.ProjectTypeKeys, project.Revision, project.UpdatedAt)
	if err != nil {
		return Project{}, mapWriteError(err)
	}
	payload := projectHistoryPayload(project)
	if reason != "" {
		payload["correction_reason"] = reason
		payload["previous_revision"] = input.ExpectedRevision
	}
	if err := insertHistory(ctx, tx, project, actor, input.CommandID, "project_details_replaced", payload); err != nil {
		return Project{}, err
	}
	if err := recordCommand(ctx, tx, actor.TenantID, input.CommandID, hash, 200, project, project.UpdatedAt); err != nil {
		return Project{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Project{}, err
	}
	return project, nil
}

func (s *PostgresStore) ReplayAssignment(ctx context.Context, tenantID, id string, input AssignmentCommand) (AssignmentResult, bool, error) {
	hash, err := assignmentRequestHash(id, input)
	if err != nil {
		return AssignmentResult{}, false, err
	}
	var replay AssignmentResult
	found, _, err := lookupCommand(ctx, s.pool, tenantID, input.CommandID, hash, &replay)
	return replay, found, err
}

func (s *PostgresStore) ApplyAssignment(ctx context.Context, actor Actor, id string, input AssignmentCommand) (AssignmentResult, error) {
	hash, err := assignmentRequestHash(id, input)
	if err != nil {
		return AssignmentResult{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AssignmentResult{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lockCommand(ctx, tx, actor.TenantID, input.CommandID); err != nil {
		return AssignmentResult{}, err
	}
	var replay AssignmentResult
	if found, _, err := lookupCommand(ctx, tx, actor.TenantID, input.CommandID, hash, &replay); err != nil {
		return AssignmentResult{}, err
	} else if found {
		return replay, nil
	}
	project, err := loadProjectForUpdate(ctx, tx, actor.TenantID, id)
	if err != nil {
		return AssignmentResult{}, err
	}
	if project.Revision != input.ExpectedRevision {
		return AssignmentResult{}, ErrRevisionConflict
	}
	if project.Status == StatusCancelled {
		return AssignmentResult{}, ErrStateConflict
	}
	project.Revision++
	project.UpdatedAt = time.Now().UTC()
	var assignment Assignment
	eventType := "assignment_started"
	if input.Operation == "assign" {
		effectiveFrom := project.UpdatedAt
		if input.EffectiveFrom != nil {
			effectiveFrom = *input.EffectiveFrom
		}
		assignment = Assignment{ID: newUUID(), TenantID: actor.TenantID, ProjectID: id, SubjectKind: input.SubjectKind, SubjectID: input.SubjectID, SourceRevision: input.SourceRevision, DisplaySnapshot: input.DisplaySnapshot, RoleKey: input.RoleKey, EffectiveFrom: effectiveFrom, CreatedProjectRevision: project.Revision, CreatedAt: project.UpdatedAt}
		_, err = tx.Exec(ctx, `INSERT INTO project_assignments(id,tenant_id,project_id,subject_kind,subject_id,source_revision,display_snapshot,role_key,effective_from,created_project_revision,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, assignment.ID, assignment.TenantID, assignment.ProjectID, assignment.SubjectKind, assignment.SubjectID, assignment.SourceRevision, assignment.DisplaySnapshot, assignment.RoleKey, assignment.EffectiveFrom, assignment.CreatedProjectRevision, assignment.CreatedAt)
		if err != nil {
			return AssignmentResult{}, mapWriteError(err)
		}
	} else {
		assignment, err = loadAssignmentForUpdate(ctx, tx, actor.TenantID, id, input.AssignmentID)
		if err != nil {
			return AssignmentResult{}, err
		}
		if assignment.EndedAt != nil {
			return AssignmentResult{}, ErrStateConflict
		}
		ended := project.UpdatedAt
		reason := input.EndReason
		endedRevision := project.Revision
		assignment.EndedAt, assignment.EndReason, assignment.EndedProjectRevision = &ended, &reason, &endedRevision
		_, err = tx.Exec(ctx, `UPDATE project_assignments SET ended_at=$4,end_reason=$5,ended_project_revision=$6 WHERE tenant_id=$1 AND project_id=$2 AND id=$3 AND ended_at IS NULL`, actor.TenantID, id, assignment.ID, ended, reason, endedRevision)
		if err != nil {
			return AssignmentResult{}, err
		}
		eventType = "assignment_ended"
	}
	if err := updateProjectRevision(ctx, tx, project); err != nil {
		return AssignmentResult{}, err
	}
	if err := insertHistory(ctx, tx, project, actor, input.CommandID, eventType, assignmentHistoryPayload(assignment)); err != nil {
		return AssignmentResult{}, err
	}
	result := AssignmentResult{Project: project, Assignment: assignment}
	if err := recordCommand(ctx, tx, actor.TenantID, input.CommandID, hash, 200, result, project.UpdatedAt); err != nil {
		return AssignmentResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AssignmentResult{}, err
	}
	return result, nil
}

func (s *PostgresStore) ReplayResourceLink(ctx context.Context, tenantID, id string, input ResourceLinkCommand) (ResourceLinkResult, bool, error) {
	hash, err := resourceLinkRequestHash(id, input)
	if err != nil {
		return ResourceLinkResult{}, false, err
	}
	var replay ResourceLinkResult
	found, _, err := lookupCommand(ctx, s.pool, tenantID, input.CommandID, hash, &replay)
	return replay, found, err
}

func (s *PostgresStore) ApplyResourceLink(ctx context.Context, actor Actor, id string, input ResourceLinkCommand) (ResourceLinkResult, error) {
	hash, err := resourceLinkRequestHash(id, input)
	if err != nil {
		return ResourceLinkResult{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ResourceLinkResult{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lockCommand(ctx, tx, actor.TenantID, input.CommandID); err != nil {
		return ResourceLinkResult{}, err
	}
	var replay ResourceLinkResult
	if found, _, err := lookupCommand(ctx, tx, actor.TenantID, input.CommandID, hash, &replay); err != nil {
		return ResourceLinkResult{}, err
	} else if found {
		return replay, nil
	}
	project, err := loadProjectForUpdate(ctx, tx, actor.TenantID, id)
	if err != nil {
		return ResourceLinkResult{}, err
	}
	if project.Revision != input.ExpectedRevision {
		return ResourceLinkResult{}, ErrRevisionConflict
	}
	if project.Status == StatusCancelled {
		return ResourceLinkResult{}, ErrStateConflict
	}
	project.Revision++
	project.UpdatedAt = time.Now().UTC()
	var link ResourceLink
	eventType := "resource_linked"
	if input.Operation == "link" {
		link = ResourceLink{ID: newUUID(), TenantID: actor.TenantID, ProjectID: id, TargetService: input.TargetService, ResourceType: input.ResourceType, ResourceID: input.ResourceID, SourceRevision: input.SourceRevision, RelationKey: input.RelationKey, Note: input.Note, CreatedProjectRevision: project.Revision, LinkedAt: project.UpdatedAt}
		_, err = tx.Exec(ctx, `INSERT INTO project_resource_links(id,tenant_id,project_id,target_service,resource_type,resource_id,source_revision,relation_key,note,created_project_revision,linked_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, link.ID, link.TenantID, link.ProjectID, link.TargetService, link.ResourceType, link.ResourceID, link.SourceRevision, link.RelationKey, link.Note, link.CreatedProjectRevision, link.LinkedAt)
		if err != nil {
			return ResourceLinkResult{}, mapWriteError(err)
		}
	} else {
		link, err = loadLinkForUpdate(ctx, tx, actor.TenantID, id, input.LinkID)
		if err != nil {
			return ResourceLinkResult{}, err
		}
		if link.UnlinkedAt != nil {
			return ResourceLinkResult{}, ErrStateConflict
		}
		at := project.UpdatedAt
		revision := project.Revision
		link.UnlinkedAt = &at
		link.UnlinkedProjectRevision = &revision
		_, err = tx.Exec(ctx, `UPDATE project_resource_links SET unlinked_at=$4,unlinked_project_revision=$5 WHERE tenant_id=$1 AND project_id=$2 AND id=$3 AND unlinked_at IS NULL`, actor.TenantID, id, link.ID, at, revision)
		if err != nil {
			return ResourceLinkResult{}, err
		}
		eventType = "resource_unlinked"
	}
	if err := updateProjectRevision(ctx, tx, project); err != nil {
		return ResourceLinkResult{}, err
	}
	if err := insertHistory(ctx, tx, project, actor, input.CommandID, eventType, resourceLinkHistoryPayload(link)); err != nil {
		return ResourceLinkResult{}, err
	}
	result := ResourceLinkResult{Project: project, ResourceLink: link}
	if err := recordCommand(ctx, tx, actor.TenantID, input.CommandID, hash, 200, result, project.UpdatedAt); err != nil {
		return ResourceLinkResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ResourceLinkResult{}, err
	}
	return result, nil
}

func (s *PostgresStore) CompleteInitiation(ctx context.Context, actor Actor, id string, input RevisionCommand) (Project, error) {
	profile, err := normalizeInitiationProfile(input.InitiationProfile)
	if err != nil {
		return Project{}, err
	}
	input.InitiationProfile = profile
	hash, _ := requestHash(struct {
		Kind, ID string
		Input    RevisionCommand
	}{"complete", id, input})
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Project{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lockCommand(ctx, tx, actor.TenantID, input.CommandID); err != nil {
		return Project{}, err
	}
	type response struct {
		Project Project  `json:"project"`
		Missing []string `json:"missing,omitempty"`
	}
	var replay response
	if found, status, err := lookupCommand(ctx, tx, actor.TenantID, input.CommandID, hash, &replay); err != nil {
		return Project{}, err
	} else if found {
		if status == 422 {
			return replay.Project, &InitiationError{Missing: replay.Missing}
		}
		return replay.Project, nil
	}
	project, err := loadProjectForUpdate(ctx, tx, actor.TenantID, id)
	if err != nil {
		return Project{}, err
	}
	if project.Revision != input.ExpectedRevision {
		return Project{}, ErrRevisionConflict
	}
	if project.Status == StatusCancelled || project.Status == StatusActive {
		return Project{}, ErrStateConflict
	}
	owners := make(map[string]bool)
	for _, role := range initiationOwnerRoles(profile) {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM project_assignments WHERE tenant_id=$1 AND project_id=$2 AND role_key=$3 AND ended_at IS NULL)`, actor.TenantID, id, role).Scan(&exists); err != nil {
			return Project{}, err
		}
		owners[role] = exists
	}
	missing := initiationMissing(project, profile, owners)
	project.Revision++
	project.UpdatedAt = time.Now().UTC()
	status := 200
	event := "project_activated"
	if len(missing) > 0 {
		project.Status = StatusPendingCompletion
		status = 422
		event = "project_initiation_pending"
	} else {
		project.Status = StatusActive
	}
	if _, err := tx.Exec(ctx, `UPDATE projects SET status=$3,revision=$4,updated_at=$5 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, id, project.Status, project.Revision, project.UpdatedAt); err != nil {
		return Project{}, err
	}
	payload := map[string]any{"missing": missing}
	if profile != "" {
		payload["initiation_profile"] = profile
	}
	if err := insertHistory(ctx, tx, project, actor, input.CommandID, event, payload); err != nil {
		return Project{}, err
	}
	result := response{Project: project, Missing: missing}
	if err := recordCommand(ctx, tx, actor.TenantID, input.CommandID, hash, status, result, project.UpdatedAt); err != nil {
		return Project{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Project{}, err
	}
	if status == 422 {
		return project, &InitiationError{Missing: missing}
	}
	return project, nil
}

func (s *PostgresStore) Cancel(ctx context.Context, actor Actor, id string, input CancelCommand) (Project, error) {
	if input.InitiationProfile != "" {
		return Project{}, fmt.Errorf("initiation profile is only accepted for completion")
	}
	hash, _ := requestHash(struct {
		Kind, ID string
		Input    CancelCommand
	}{"cancel", id, input})
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Project{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lockCommand(ctx, tx, actor.TenantID, input.CommandID); err != nil {
		return Project{}, err
	}
	var replay Project
	if found, _, err := lookupCommand(ctx, tx, actor.TenantID, input.CommandID, hash, &replay); err != nil {
		return Project{}, err
	} else if found {
		return replay, nil
	}
	project, err := loadProjectForUpdate(ctx, tx, actor.TenantID, id)
	if err != nil {
		return Project{}, err
	}
	if project.Revision != input.ExpectedRevision {
		return Project{}, ErrRevisionConflict
	}
	if project.Status == StatusCancelled {
		return Project{}, ErrStateConflict
	}
	project.Revision++
	project.Status = StatusCancelled
	project.UpdatedAt = time.Now().UTC()
	reason := input.Reason
	project.CancellationReason = &reason
	if _, err := tx.Exec(ctx, `UPDATE projects SET status='cancelled',revision=$3,cancellation_reason=$4,updated_at=$5 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, id, project.Revision, reason, project.UpdatedAt); err != nil {
		return Project{}, err
	}
	if err := insertHistory(ctx, tx, project, actor, input.CommandID, "project_cancelled", map[string]any{"reason": reason}); err != nil {
		return Project{}, err
	}
	if err := recordCommand(ctx, tx, actor.TenantID, input.CommandID, hash, 200, project, project.UpdatedAt); err != nil {
		return Project{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Project{}, err
	}
	return project, nil
}

func (s *PostgresStore) Get(ctx context.Context, tenantID, id string, includeInactive bool) (Details, error) {
	project, err := loadProject(ctx, s.pool, tenantID, id, "")
	if err != nil {
		return Details{}, err
	}
	details := Details{Project: project, Children: []ProjectSummary{}, Assignments: []Assignment{}, ResourceLinks: []ResourceLink{}}
	if project.ParentProjectID != nil {
		parent, parentErr := loadProject(ctx, s.pool, tenantID, *project.ParentProjectID, "")
		if parentErr != nil {
			return Details{}, parentErr
		}
		summary := summarizeProject(parent)
		details.Parent = &summary
	}
	childRows, err := s.pool.Query(ctx, `SELECT id,tenant_id,project_number,node_type,parent_project_id,sort_order,name,summary,planned_start_date,planned_end_date,project_type_keys,client_party_id,client_party_source_revision,client_party_kind,client_party_legal_name,status,revision,cancellation_reason,created_by,created_at,updated_at FROM projects WHERE tenant_id=$1 AND parent_project_id=$2 ORDER BY sort_order,created_at,id`, tenantID, id)
	if err != nil {
		return Details{}, err
	}
	for childRows.Next() {
		child, scanErr := scanProject(childRows)
		if scanErr != nil {
			childRows.Close()
			return Details{}, scanErr
		}
		details.Children = append(details.Children, summarizeProject(child))
	}
	if err := childRows.Err(); err != nil {
		childRows.Close()
		return Details{}, err
	}
	childRows.Close()
	assignmentSQL := `SELECT id,tenant_id,project_id,subject_kind,subject_id,source_revision,display_snapshot,role_key,effective_from,ended_at,end_reason,created_project_revision,ended_project_revision,created_at FROM project_assignments WHERE tenant_id=$1 AND project_id=$2`
	if !includeInactive {
		assignmentSQL += ` AND ended_at IS NULL`
	}
	assignmentSQL += ` ORDER BY created_at,id`
	rows, err := s.pool.Query(ctx, assignmentSQL, tenantID, id)
	if err != nil {
		return Details{}, err
	}
	for rows.Next() {
		a, err := scanAssignment(rows)
		if err != nil {
			rows.Close()
			return Details{}, err
		}
		details.Assignments = append(details.Assignments, a)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Details{}, err
	}
	rows.Close()
	linkSQL := `SELECT id,tenant_id,project_id,target_service,resource_type,resource_id,source_revision,relation_key,note,created_project_revision,unlinked_project_revision,linked_at,unlinked_at FROM project_resource_links WHERE tenant_id=$1 AND project_id=$2`
	if !includeInactive {
		linkSQL += ` AND unlinked_at IS NULL`
	}
	linkSQL += ` ORDER BY linked_at,id`
	linkRows, err := s.pool.Query(ctx, linkSQL, tenantID, id)
	if err != nil {
		return Details{}, err
	}
	defer linkRows.Close()
	for linkRows.Next() {
		v, err := scanLink(linkRows)
		if err != nil {
			return Details{}, err
		}
		details.ResourceLinks = append(details.ResourceLinks, v)
	}
	if err := linkRows.Err(); err != nil {
		return Details{}, err
	}
	var conversion PackageConversion
	err = s.pool.QueryRow(ctx, `SELECT id,tenant_id,operation,project_id,package_id,previous_parent_project_id,previous_sort_order,created_package,created_project_revision,created_package_revision,status,created_by,created_at,rolled_back_by,rolled_back_at,rollback_reason FROM project_package_conversions WHERE tenant_id=$1 AND project_id=$2 AND status='active' ORDER BY created_at DESC,id DESC LIMIT 1`, tenantID, id).Scan(&conversion.ID, &conversion.TenantID, &conversion.Operation, &conversion.ProjectID, &conversion.PackageID, &conversion.PreviousParentProjectID, &conversion.PreviousSortOrder, &conversion.CreatedPackage, &conversion.CreatedProjectRevision, &conversion.CreatedPackageRevision, &conversion.Status, &conversion.CreatedBy, &conversion.CreatedAt, &conversion.RolledBackBy, &conversion.RolledBackAt, &conversion.RollbackReason)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Details{}, err
	}
	if err == nil {
		details.ActivePackageConversion = &conversion
	}
	return details, nil
}

func (s *PostgresStore) History(ctx context.Context, tenantID, id, cursor string, limit int) (HistoryPage, error) {
	if _, err := loadProject(ctx, s.pool, tenantID, id, ""); err != nil {
		return HistoryPage{}, err
	}
	args := []any{tenantID, id, limit + 1}
	query := `SELECT id,tenant_id,project_id,project_revision,event_sequence,event_type,command_id,actor_subject,occurred_at,payload FROM project_history_events WHERE tenant_id=$1 AND project_id=$2`
	if cursor != "" {
		c, err := decodeCursor(cursor)
		if err != nil {
			return HistoryPage{}, fmt.Errorf("invalid cursor")
		}
		query += ` AND (project_revision,event_sequence,id) < ($4,$5,$6)`
		args = append(args, c.Revision, c.Sequence, c.ID)
	}
	query += ` ORDER BY project_revision DESC,event_sequence DESC,id DESC LIMIT $3`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return HistoryPage{}, err
	}
	defer rows.Close()
	items := make([]HistoryEvent, 0, limit+1)
	for rows.Next() {
		var e HistoryEvent
		var payload []byte
		if err := rows.Scan(&e.ID, &e.TenantID, &e.ProjectID, &e.ProjectRevision, &e.EventSequence, &e.EventType, &e.CommandID, &e.ActorSubject, &e.OccurredAt, &payload); err != nil {
			return HistoryPage{}, err
		}
		if err := json.Unmarshal(payload, &e.Payload); err != nil {
			return HistoryPage{}, err
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		return HistoryPage{}, err
	}
	page := HistoryPage{Items: items}
	if len(items) > limit {
		last := items[limit-1]
		next := encodeCursor(historyCursor{Revision: last.ProjectRevision, Sequence: last.EventSequence, ID: last.ID})
		page.NextCursor = &next
		page.Items = items[:limit]
	}
	return page, nil
}

func (s *PostgresStore) LatestCompletionConfirmation(ctx context.Context, tenantID, projectID, profileKey string) (*CompletionConfirmation, error) {
	var value CompletionConfirmation
	var sources []byte
	err := s.pool.QueryRow(ctx, `SELECT id,tenant_id,project_id,profile_key,observed_project_revision,confirmed_project_revision,sources_json,command_id,confirmed_by,confirmed_at FROM project_completion_confirmations WHERE tenant_id=$1 AND project_id=$2 AND profile_key=$3 ORDER BY confirmed_project_revision DESC,id DESC LIMIT 1`, tenantID, projectID, profileKey).Scan(
		&value.ID, &value.TenantID, &value.ProjectID, &value.ProfileKey,
		&value.ObservedProjectRevision, &value.ConfirmedProjectRevision, &sources,
		&value.CommandID, &value.ConfirmedBy, &value.ConfirmedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(sources, &value.Sources); err != nil {
		return nil, err
	}
	return &value, nil
}

func (s *PostgresStore) ReplayCompletion(ctx context.Context, tenantID, id string, input CompletionCommand) (CompletionConfirmation, bool, error) {
	hash, err := completionRequestHash(id, input)
	if err != nil {
		return CompletionConfirmation{}, false, err
	}
	var replay CompletionConfirmation
	found, _, err := lookupCommand(ctx, s.pool, tenantID, input.CommandID, hash, &replay)
	return replay, found, err
}

func (s *PostgresStore) ConfirmCompletion(ctx context.Context, actor Actor, id string, input CompletionCommand, assessment CompletionAssessment) (CompletionConfirmation, error) {
	hash, err := completionRequestHash(id, input)
	if err != nil {
		return CompletionConfirmation{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CompletionConfirmation{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lockCommand(ctx, tx, actor.TenantID, input.CommandID); err != nil {
		return CompletionConfirmation{}, err
	}
	var replay CompletionConfirmation
	if found, _, err := lookupCommand(ctx, tx, actor.TenantID, input.CommandID, hash, &replay); err != nil {
		return CompletionConfirmation{}, err
	} else if found {
		return replay, nil
	}
	project, err := loadProjectForUpdate(ctx, tx, actor.TenantID, id)
	if err != nil {
		return CompletionConfirmation{}, err
	}
	if project.Revision != input.ExpectedRevision || assessment.ProjectRevision != input.ExpectedRevision {
		return CompletionConfirmation{}, ErrRevisionConflict
	}
	if project.NodeType != NodeTypeProject || project.Status != StatusActive {
		return CompletionConfirmation{}, ErrStateConflict
	}
	if assessment.OverallStatus != CompletionSatisfied {
		return CompletionConfirmation{}, ErrCompletionIncomplete
	}
	sources, err := completionSources(assessment.Requirements)
	if err != nil || !reflect.DeepEqual(sources, input.ExpectedSources) {
		return CompletionConfirmation{}, ErrCompletionDrift
	}
	project.Revision++
	project.UpdatedAt = time.Now().UTC()
	if err := updateProjectRevision(ctx, tx, project); err != nil {
		return CompletionConfirmation{}, err
	}
	value := CompletionConfirmation{
		ID: newUUID(), TenantID: actor.TenantID, ProjectID: id, ProfileKey: input.ProfileKey,
		ObservedProjectRevision: input.ExpectedRevision, ConfirmedProjectRevision: project.Revision,
		Sources: sources, CommandID: input.CommandID, ConfirmedBy: actor.ID, ConfirmedAt: project.UpdatedAt,
	}
	raw, err := json.Marshal(value.Sources)
	if err != nil {
		return CompletionConfirmation{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO project_completion_confirmations(id,tenant_id,project_id,profile_key,observed_project_revision,confirmed_project_revision,sources_json,command_id,confirmed_by,confirmed_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, value.ID, value.TenantID, value.ProjectID, value.ProfileKey, value.ObservedProjectRevision, value.ConfirmedProjectRevision, raw, value.CommandID, value.ConfirmedBy, value.ConfirmedAt); err != nil {
		return CompletionConfirmation{}, mapWriteError(err)
	}
	if err := insertHistory(ctx, tx, project, actor, input.CommandID, "completion_profile_confirmed", map[string]any{"profile_key": value.ProfileKey, "sources": value.Sources, "confirmation_id": value.ID}); err != nil {
		return CompletionConfirmation{}, err
	}
	if err := recordCommand(ctx, tx, actor.TenantID, input.CommandID, hash, 200, value, project.UpdatedAt); err != nil {
		return CompletionConfirmation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CompletionConfirmation{}, err
	}
	return value, nil
}

func completionRequestHash(id string, input CompletionCommand) (string, error) {
	return requestHash(struct {
		Kind, ID string
		Input    CompletionCommand
	}{"completion-confirmation", id, input})
}

type rowScanner interface{ Scan(...any) error }

func loadProject(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, tenant, id, suffix string) (Project, error) {
	return scanProject(q.QueryRow(ctx, `SELECT id,tenant_id,project_number,node_type,parent_project_id,sort_order,name,summary,planned_start_date,planned_end_date,project_type_keys,client_party_id,client_party_source_revision,client_party_kind,client_party_legal_name,status,revision,cancellation_reason,created_by,created_at,updated_at FROM projects WHERE tenant_id=$1 AND id=$2 `+suffix, tenant, id))
}
func loadProjectForUpdate(ctx context.Context, tx pgx.Tx, tenant, id string) (Project, error) {
	return loadProject(ctx, tx, tenant, id, "FOR UPDATE")
}
func scanProject(row rowScanner) (Project, error) {
	var p Project
	var partyID, partyKind, partyName *string
	var partyRevision *int64
	if err := row.Scan(&p.ID, &p.TenantID, &p.ProjectNumber, &p.NodeType, &p.ParentProjectID, &p.SortOrder, &p.Name, &p.Summary, &p.PlannedStartDate, &p.PlannedEndDate, &p.ProjectTypeKeys, &partyID, &partyRevision, &partyKind, &partyName, &p.Status, &p.Revision, &p.CancellationReason, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Project{}, ErrNotFound
		}
		return Project{}, err
	}
	if partyID != nil && partyRevision != nil && partyKind != nil && partyName != nil {
		p.ClientParty = &PartySnapshot{PartyID: *partyID, SourceRevision: *partyRevision, Kind: *partyKind, LegalName: *partyName}
	}
	return p, nil
}

func summarizeProject(p Project) ProjectSummary {
	return ProjectSummary{ID: p.ID, ProjectNumber: p.ProjectNumber, Name: p.Name, NodeType: p.NodeType, ParentProjectID: p.ParentProjectID, SortOrder: p.SortOrder, Status: p.Status, Revision: p.Revision}
}
func loadAssignmentForUpdate(ctx context.Context, tx pgx.Tx, tenant, projectID, id string) (Assignment, error) {
	return scanAssignment(tx.QueryRow(ctx, `SELECT id,tenant_id,project_id,subject_kind,subject_id,source_revision,display_snapshot,role_key,effective_from,ended_at,end_reason,created_project_revision,ended_project_revision,created_at FROM project_assignments WHERE tenant_id=$1 AND project_id=$2 AND id=$3 FOR UPDATE`, tenant, projectID, id))
}
func scanAssignment(row rowScanner) (Assignment, error) {
	var a Assignment
	if err := row.Scan(&a.ID, &a.TenantID, &a.ProjectID, &a.SubjectKind, &a.SubjectID, &a.SourceRevision, &a.DisplaySnapshot, &a.RoleKey, &a.EffectiveFrom, &a.EndedAt, &a.EndReason, &a.CreatedProjectRevision, &a.EndedProjectRevision, &a.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Assignment{}, ErrNotFound
		}
		return Assignment{}, err
	}
	return a, nil
}
func loadLinkForUpdate(ctx context.Context, tx pgx.Tx, tenant, projectID, id string) (ResourceLink, error) {
	return scanLink(tx.QueryRow(ctx, `SELECT id,tenant_id,project_id,target_service,resource_type,resource_id,source_revision,relation_key,note,created_project_revision,unlinked_project_revision,linked_at,unlinked_at FROM project_resource_links WHERE tenant_id=$1 AND project_id=$2 AND id=$3 FOR UPDATE`, tenant, projectID, id))
}
func scanLink(row rowScanner) (ResourceLink, error) {
	var v ResourceLink
	if err := row.Scan(&v.ID, &v.TenantID, &v.ProjectID, &v.TargetService, &v.ResourceType, &v.ResourceID, &v.SourceRevision, &v.RelationKey, &v.Note, &v.CreatedProjectRevision, &v.UnlinkedProjectRevision, &v.LinkedAt, &v.UnlinkedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ResourceLink{}, ErrNotFound
		}
		return ResourceLink{}, err
	}
	return v, nil
}
func updateProjectRevision(ctx context.Context, tx pgx.Tx, p Project) error {
	_, err := tx.Exec(ctx, `UPDATE projects SET revision=$3,updated_at=$4 WHERE tenant_id=$1 AND id=$2`, p.TenantID, p.ID, p.Revision, p.UpdatedAt)
	return err
}
func insertHistory(ctx context.Context, tx pgx.Tx, p Project, actor Actor, commandID, eventType string, payload map[string]any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO project_history_events(id,tenant_id,project_id,project_revision,event_sequence,event_type,command_id,actor_subject,occurred_at,payload) VALUES($1,$2,$3,$4,1,$5,$6,$7,$8,$9)`, newUUID(), p.TenantID, p.ID, p.Revision, eventType, commandID, actor.ID, p.UpdatedAt, raw)
	return err
}

func projectHistoryPayload(project Project) map[string]any {
	return map[string]any{
		"project_number":     project.ProjectNumber,
		"node_type":          project.NodeType,
		"parent_project_id":  project.ParentProjectID,
		"sort_order":         project.SortOrder,
		"name":               project.Name,
		"summary":            project.Summary,
		"planned_start_date": project.PlannedStartDate,
		"planned_end_date":   project.PlannedEndDate,
		"project_type_keys":  project.ProjectTypeKeys,
		"client_party":       project.ClientParty,
		"status":             project.Status,
	}
}

func assignmentHistoryPayload(assignment Assignment) map[string]any {
	return map[string]any{
		"assignment_id":    assignment.ID,
		"subject_kind":     assignment.SubjectKind,
		"subject_id":       assignment.SubjectID,
		"source_revision":  assignment.SourceRevision,
		"display_snapshot": assignment.DisplaySnapshot,
		"role_key":         assignment.RoleKey,
		"effective_from":   assignment.EffectiveFrom,
		"ended_at":         assignment.EndedAt,
		"end_reason":       assignment.EndReason,
	}
}

func resourceLinkHistoryPayload(link ResourceLink) map[string]any {
	return map[string]any{
		"link_id":         link.ID,
		"target_service":  link.TargetService,
		"resource_type":   link.ResourceType,
		"resource_id":     link.ResourceID,
		"source_revision": link.SourceRevision,
		"relation_key":    link.RelationKey,
		"note":            link.Note,
		"linked_at":       link.LinkedAt,
		"unlinked_at":     link.UnlinkedAt,
	}
}

type commandQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func lookupCommand(ctx context.Context, tx commandQueryer, tenant, commandID, hash string, dest any) (bool, int, error) {
	var storedHash string
	var status int
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT request_hash,response_status,response_json FROM project_commands WHERE tenant_id=$1 AND command_id=$2`, tenant, commandID).Scan(&storedHash, &status, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, err
	}
	if storedHash != hash {
		return false, 409, ErrCommandConflict
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return false, 0, err
	}
	return true, status, nil
}

func lockCommand(ctx context.Context, tx pgx.Tx, tenant, commandID string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, tenant+":"+commandID)
	return err
}

func assignmentRequestHash(id string, input AssignmentCommand) (string, error) {
	return requestHash(struct {
		Kind, ID string
		Input    AssignmentCommand
	}{"assignment", id, input})
}

func resourceLinkRequestHash(id string, input ResourceLinkCommand) (string, error) {
	return requestHash(struct {
		Kind, ID string
		Input    ResourceLinkCommand
	}{"resource-link", id, input})
}
func recordCommand(ctx context.Context, tx pgx.Tx, tenant, commandID, hash string, status int, response any, at time.Time) error {
	raw, err := json.Marshal(response)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO project_commands(tenant_id,command_id,request_hash,response_status,response_json,created_at) VALUES($1,$2,$3,$4,$5,$6)`, tenant, commandID, hash, status, raw, at)
	return err
}
func requestHash(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
func mapWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "projects_tenant_id_project_number_normalized_key", "projects_tenant_number_active_idx":
			return ErrNumberConflict
		case "project_assignments_active_owner_idx", "project_assignments_active_subject_role_idx":
			return ErrOwnerConflict
		case "project_resource_links_active_idx":
			return ErrActiveLinkConflict
		case "project_commands_pkey":
			return ErrCommandConflict
		case "project_package_conversions_active_project_idx":
			return ErrHierarchyConflict
		}
	}
	return err
}
func sameDetails(p Project, in CreateInput) bool {
	a, _ := json.Marshal(struct {
		N, S       string
		Start, End *time.Time
		Types      []string
	}{in.Name, in.Summary, in.PlannedStartDate, in.PlannedEndDate, in.ProjectTypeKeys})
	b, _ := json.Marshal(struct {
		N, S       string
		Start, End *time.Time
		Types      []string
	}{p.Name, p.Summary, p.PlannedStartDate, p.PlannedEndDate, p.ProjectTypeKeys})
	return p.ProjectNumber == in.ProjectNumber && string(a) == string(b)
}

func createRequestHash(input CreateInput) (string, error) {
	return requestHash(struct {
		Kind  string
		Input CreateInput
	}{"create", input})
}

type historyCursor struct {
	Revision, Sequence int64
	ID                 string
}

func encodeCursor(c historyCursor) string {
	raw, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(raw)
}
func decodeCursor(raw string) (historyCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return historyCursor{}, err
	}
	var c historyCursor
	if err := json.Unmarshal(data, &c); err != nil || c.Revision < 1 || c.Sequence < 1 || c.ID == "" {
		return historyCursor{}, fmt.Errorf("invalid cursor")
	}
	return c, nil
}
