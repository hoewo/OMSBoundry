package projects

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

func initiationDispatchHash(input InitiationDispatchInput) (string, error) {
	return requestHash(struct {
		Kind  string
		Input InitiationDispatchInput
	}{"initiation-dispatch", input})
}

func (s *PostgresStore) ReplayInitiationDispatch(ctx context.Context, tenantID string, input InitiationDispatchInput) (Project, bool, error) {
	hash, err := initiationDispatchHash(input)
	if err != nil {
		return Project{}, false, err
	}
	var replay Project
	found, _, err := lookupCommand(ctx, s.pool, tenantID, input.CommandID, hash, &replay)
	return replay, found, err
}

func (s *PostgresStore) DispatchInitiation(ctx context.Context, actor Actor, input InitiationDispatchInput) (Project, error) {
	hash, err := initiationDispatchHash(input)
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
	if found, _, err := lookupCommand(ctx, tx, actor.TenantID, input.CommandID, hash, &replay); err != nil {
		return Project{}, err
	} else if found {
		return replay, nil
	}
	now := time.Now().UTC()
	project := Project{ID: newUUID(), TenantID: actor.TenantID, NodeType: input.NodeType, Name: input.Name, ProjectTypeKeys: []string{}, Status: StatusDraft, Revision: 1, CreatedBy: actor.ID, CreatedAt: now, UpdatedAt: now}
	if err := insertProject(ctx, tx, project); err != nil {
		return Project{}, err
	}
	assignment := Assignment{ID: newUUID(), TenantID: actor.TenantID, ProjectID: project.ID, SubjectKind: "person", SubjectID: input.BusinessOwner.PersonID, SourceRevision: input.BusinessOwner.SourceRevision, DisplaySnapshot: input.ownerSnapshot, RoleKey: RoleBusinessOwner, EffectiveFrom: now, CreatedProjectRevision: 1, CreatedAt: now}
	if _, err := tx.Exec(ctx, `INSERT INTO project_assignments(id,tenant_id,project_id,subject_kind,subject_id,source_revision,display_snapshot,role_key,effective_from,created_project_revision,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, assignment.ID, assignment.TenantID, assignment.ProjectID, assignment.SubjectKind, assignment.SubjectID, assignment.SourceRevision, assignment.DisplaySnapshot, assignment.RoleKey, assignment.EffectiveFrom, assignment.CreatedProjectRevision, assignment.CreatedAt); err != nil {
		return Project{}, mapWriteError(err)
	}
	payload := projectHistoryPayload(project)
	payload["initial_business_owner"] = assignmentHistoryPayload(assignment)
	payload["initiation_profile"] = InitiationProfileBusinessOwnerV1
	if err := insertHistory(ctx, tx, project, actor, input.CommandID, "project_created", payload); err != nil {
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
