package projects

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *PostgresStore) CreatePackage(ctx context.Context, actor Actor, input PackageCreateInput) (Project, error) {
	hash, err := requestHash(struct {
		Kind  string
		Input PackageCreateInput
	}{"create-package", input})
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
	project := Project{
		ID: newUUID(), TenantID: actor.TenantID, ProjectNumber: input.ProjectNumber,
		NodeType: NodeTypePackage, Name: input.Name, Summary: input.Summary,
		PlannedStartDate: input.PlannedStartDate, PlannedEndDate: input.PlannedEndDate,
		ProjectTypeKeys: input.ProjectTypeKeys, Status: StatusDraft, Revision: 1,
		CreatedBy: actor.ID, CreatedAt: now, UpdatedAt: now,
	}
	if err := insertProject(ctx, tx, project); err != nil {
		return Project{}, err
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

func (s *PostgresStore) ConvertPackage(ctx context.Context, actor Actor, projectID string, input PackageConversionCommand) (PackageConversionResult, error) {
	hash, err := requestHash(struct {
		Kind, ProjectID string
		Input           PackageConversionCommand
	}{"package-conversion", projectID, input})
	if err != nil {
		return PackageConversionResult{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return PackageConversionResult{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lockCommand(ctx, tx, actor.TenantID, input.CommandID); err != nil {
		return PackageConversionResult{}, err
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, actor.TenantID+":project-hierarchy"); err != nil {
		return PackageConversionResult{}, err
	}
	var replay PackageConversionResult
	if found, _, err := lookupCommand(ctx, tx, actor.TenantID, input.CommandID, hash, &replay); err != nil {
		return PackageConversionResult{}, err
	} else if found {
		return replay, nil
	}
	project, err := loadProjectForUpdate(ctx, tx, actor.TenantID, projectID)
	if err != nil {
		return PackageConversionResult{}, err
	}
	if project.Revision != input.ExpectedProjectRevision {
		return PackageConversionResult{}, ErrRevisionConflict
	}
	if project.NodeType != NodeTypeProject || project.ParentProjectID != nil || project.Status == StatusCancelled {
		return PackageConversionResult{}, ErrHierarchyConflict
	}

	now := time.Now().UTC()
	var packageProject Project
	createdPackage := false
	eventType := "project_attached_to_package"
	switch input.Operation {
	case ConversionAttachExisting:
		packageProject, err = loadProjectForUpdate(ctx, tx, actor.TenantID, input.TargetPackageID)
		if err != nil {
			return PackageConversionResult{}, err
		}
		if packageProject.Revision != input.ExpectedPackageRevision {
			return PackageConversionResult{}, ErrRevisionConflict
		}
		if packageProject.ID == project.ID || packageProject.NodeType != NodeTypePackage || packageProject.ParentProjectID != nil || packageProject.Status == StatusCancelled {
			return PackageConversionResult{}, ErrHierarchyConflict
		}
		packageProject.Revision++
		packageProject.UpdatedAt = now
		if err := updateProjectRevision(ctx, tx, packageProject); err != nil {
			return PackageConversionResult{}, err
		}
	case ConversionWrapAsPackage:
		if input.NewPackage == nil {
			return PackageConversionResult{}, fmt.Errorf("new package is required")
		}
		createdPackage = true
		eventType = "project_wrapped_as_package"
		packageProject = Project{
			ID: newUUID(), TenantID: actor.TenantID, ProjectNumber: input.NewPackage.ProjectNumber,
			NodeType: NodeTypePackage, Name: input.NewPackage.Name, Summary: input.NewPackage.Summary,
			PlannedStartDate: input.NewPackage.PlannedStartDate, PlannedEndDate: input.NewPackage.PlannedEndDate,
			ProjectTypeKeys: input.NewPackage.ProjectTypeKeys, Status: StatusDraft, Revision: 1,
			CreatedBy: actor.ID, CreatedAt: now, UpdatedAt: now,
		}
		if err := insertProject(ctx, tx, packageProject); err != nil {
			return PackageConversionResult{}, err
		}
		payload := projectHistoryPayload(packageProject)
		payload["created_by_conversion"] = true
		payload["first_child_project_id"] = project.ID
		if err := insertHistory(ctx, tx, packageProject, actor, input.CommandID, "project_created", payload); err != nil {
			return PackageConversionResult{}, err
		}
	default:
		return PackageConversionResult{}, fmt.Errorf("invalid package conversion operation")
	}

	project.ParentProjectID = &packageProject.ID
	project.SortOrder = input.SortOrder
	project.Revision++
	project.UpdatedAt = now
	if _, err := tx.Exec(ctx, `UPDATE projects SET parent_project_id=$3,sort_order=$4,revision=$5,updated_at=$6 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, project.ID, packageProject.ID, project.SortOrder, project.Revision, now); err != nil {
		return PackageConversionResult{}, err
	}
	payload := map[string]any{"package_id": packageProject.ID, "operation": input.Operation, "sort_order": project.SortOrder, "project_id_preserved": true}
	if err := insertHistory(ctx, tx, project, actor, input.CommandID, eventType, payload); err != nil {
		return PackageConversionResult{}, err
	}
	if !createdPackage {
		if err := insertHistory(ctx, tx, packageProject, actor, input.CommandID, "package_child_attached", payload); err != nil {
			return PackageConversionResult{}, err
		}
	}
	conversion := PackageConversion{
		ID: newUUID(), TenantID: actor.TenantID, Operation: input.Operation,
		ProjectID: project.ID, PackageID: packageProject.ID, PreviousSortOrder: 0,
		CreatedPackage: createdPackage, CreatedProjectRevision: project.Revision,
		CreatedPackageRevision: packageProject.Revision, Status: ConversionActive,
		CreatedBy: actor.ID, CreatedAt: now,
	}
	if _, err := tx.Exec(ctx, `INSERT INTO project_package_conversions(id,tenant_id,operation,project_id,package_id,previous_parent_project_id,previous_sort_order,created_package,created_project_revision,created_package_revision,status,created_by,created_at) VALUES($1,$2,$3,$4,$5,NULL,$6,$7,$8,$9,$10,$11,$12)`, conversion.ID, conversion.TenantID, conversion.Operation, conversion.ProjectID, conversion.PackageID, conversion.PreviousSortOrder, conversion.CreatedPackage, conversion.CreatedProjectRevision, conversion.CreatedPackageRevision, conversion.Status, conversion.CreatedBy, conversion.CreatedAt); err != nil {
		return PackageConversionResult{}, mapWriteError(err)
	}
	result := PackageConversionResult{Conversion: conversion, Project: project, Package: packageProject}
	if err := recordCommand(ctx, tx, actor.TenantID, input.CommandID, hash, 200, result, now); err != nil {
		return PackageConversionResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PackageConversionResult{}, err
	}
	return result, nil
}

func (s *PostgresStore) RollbackPackageConversion(ctx context.Context, actor Actor, projectID, conversionID string, input PackageRollbackCommand) (PackageConversionResult, error) {
	hash, err := requestHash(struct {
		Kind, ProjectID, ConversionID string
		Input                         PackageRollbackCommand
	}{"rollback-package-conversion", projectID, conversionID, input})
	if err != nil {
		return PackageConversionResult{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return PackageConversionResult{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lockCommand(ctx, tx, actor.TenantID, input.CommandID); err != nil {
		return PackageConversionResult{}, err
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, actor.TenantID+":project-hierarchy"); err != nil {
		return PackageConversionResult{}, err
	}
	var replay PackageConversionResult
	if found, _, err := lookupCommand(ctx, tx, actor.TenantID, input.CommandID, hash, &replay); err != nil {
		return PackageConversionResult{}, err
	} else if found {
		return replay, nil
	}
	conversion, err := loadConversionForUpdate(ctx, tx, actor.TenantID, conversionID)
	if err != nil {
		return PackageConversionResult{}, err
	}
	if conversion.ProjectID != projectID || conversion.Status != ConversionActive {
		return PackageConversionResult{}, ErrHierarchyConflict
	}
	project, err := loadProjectForUpdate(ctx, tx, actor.TenantID, projectID)
	if err != nil {
		return PackageConversionResult{}, err
	}
	packageProject, err := loadProjectForUpdate(ctx, tx, actor.TenantID, conversion.PackageID)
	if err != nil {
		return PackageConversionResult{}, err
	}
	if project.Revision != input.ExpectedProjectRevision || packageProject.Revision != input.ExpectedPackageRevision {
		return PackageConversionResult{}, ErrRevisionConflict
	}
	if project.ParentProjectID == nil || *project.ParentProjectID != packageProject.ID {
		return PackageConversionResult{}, ErrHierarchyConflict
	}
	if conversion.CreatedPackage {
		var children, assignments, links int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM projects WHERE tenant_id=$1 AND parent_project_id=$2`, actor.TenantID, packageProject.ID).Scan(&children); err != nil {
			return PackageConversionResult{}, err
		}
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM project_assignments WHERE tenant_id=$1 AND project_id=$2`, actor.TenantID, packageProject.ID).Scan(&assignments); err != nil {
			return PackageConversionResult{}, err
		}
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM project_resource_links WHERE tenant_id=$1 AND project_id=$2`, actor.TenantID, packageProject.ID).Scan(&links); err != nil {
			return PackageConversionResult{}, err
		}
		if children != 1 || assignments != 0 || links != 0 || packageProject.Revision != conversion.CreatedPackageRevision || packageProject.Status != StatusDraft {
			return PackageConversionResult{}, ErrRollbackUnsafe
		}
	}
	now := time.Now().UTC()
	project.ParentProjectID = conversion.PreviousParentProjectID
	project.SortOrder = conversion.PreviousSortOrder
	project.Revision++
	project.UpdatedAt = now
	if _, err := tx.Exec(ctx, `UPDATE projects SET parent_project_id=$3,sort_order=$4,revision=$5,updated_at=$6 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, project.ID, project.ParentProjectID, project.SortOrder, project.Revision, now); err != nil {
		return PackageConversionResult{}, err
	}
	packageProject.Revision++
	packageProject.UpdatedAt = now
	if conversion.CreatedPackage {
		reason := "撤销项目包转换：" + input.Reason
		packageProject.Status = StatusCancelled
		packageProject.CancellationReason = &reason
		if _, err := tx.Exec(ctx, `UPDATE projects SET status='cancelled',cancellation_reason=$3,revision=$4,updated_at=$5 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, packageProject.ID, reason, packageProject.Revision, now); err != nil {
			return PackageConversionResult{}, err
		}
	} else if err := updateProjectRevision(ctx, tx, packageProject); err != nil {
		return PackageConversionResult{}, err
	}
	payload := map[string]any{"conversion_id": conversion.ID, "package_id": packageProject.ID, "reason": input.Reason, "restored_parent_project_id": conversion.PreviousParentProjectID, "project_id_preserved": true}
	if err := insertHistory(ctx, tx, project, actor, input.CommandID, "project_package_conversion_rolled_back", payload); err != nil {
		return PackageConversionResult{}, err
	}
	if err := insertHistory(ctx, tx, packageProject, actor, input.CommandID, "package_child_detached", payload); err != nil {
		return PackageConversionResult{}, err
	}
	conversion.Status = ConversionRolledBack
	conversion.RolledBackBy = &actor.ID
	conversion.RolledBackAt = &now
	conversion.RollbackReason = &input.Reason
	if _, err := tx.Exec(ctx, `UPDATE project_package_conversions SET status='rolled_back',rolled_back_by=$3,rolled_back_at=$4,rollback_reason=$5 WHERE tenant_id=$1 AND id=$2 AND status='active'`, actor.TenantID, conversion.ID, actor.ID, now, input.Reason); err != nil {
		return PackageConversionResult{}, err
	}
	result := PackageConversionResult{Conversion: conversion, Project: project, Package: packageProject}
	if err := recordCommand(ctx, tx, actor.TenantID, input.CommandID, hash, 200, result, now); err != nil {
		return PackageConversionResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PackageConversionResult{}, err
	}
	return result, nil
}

func insertProject(ctx context.Context, tx pgx.Tx, p Project) error {
	_, err := tx.Exec(ctx, `INSERT INTO projects(id,tenant_id,project_number,project_number_normalized,node_type,parent_project_id,sort_order,name,summary,planned_start_date,planned_end_date,project_type_keys,status,revision,cancellation_reason,created_by,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`, p.ID, p.TenantID, p.ProjectNumber, strings.ToLower(p.ProjectNumber), p.NodeType, p.ParentProjectID, p.SortOrder, p.Name, p.Summary, p.PlannedStartDate, p.PlannedEndDate, p.ProjectTypeKeys, p.Status, p.Revision, p.CancellationReason, p.CreatedBy, p.CreatedAt, p.UpdatedAt)
	return mapWriteError(err)
}

func loadConversionForUpdate(ctx context.Context, tx pgx.Tx, tenantID, id string) (PackageConversion, error) {
	var v PackageConversion
	err := tx.QueryRow(ctx, `SELECT id,tenant_id,operation,project_id,package_id,previous_parent_project_id,previous_sort_order,created_package,created_project_revision,created_package_revision,status,created_by,created_at,rolled_back_by,rolled_back_at,rollback_reason FROM project_package_conversions WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, tenantID, id).Scan(&v.ID, &v.TenantID, &v.Operation, &v.ProjectID, &v.PackageID, &v.PreviousParentProjectID, &v.PreviousSortOrder, &v.CreatedPackage, &v.CreatedProjectRevision, &v.CreatedPackageRevision, &v.Status, &v.CreatedBy, &v.CreatedAt, &v.RolledBackBy, &v.RolledBackAt, &v.RollbackReason)
	if errors.Is(err, pgx.ErrNoRows) {
		return PackageConversion{}, ErrNotFound
	}
	return v, err
}
