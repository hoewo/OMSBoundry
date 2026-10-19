package projects

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *PostgresStore) ReplayBasicInfoChange(ctx context.Context, tenant, id string, in BasicInfoChangeInput) (BasicInfoChangeResult, bool, error) {
	return s.replayOwnerInfoChange(ctx, tenant, id, in, "project-basic-info-change-v1")
}
func (s *PostgresStore) ReplayInitiationInfoChange(ctx context.Context, tenant, id string, in BasicInfoChangeInput) (BasicInfoChangeResult, bool, error) {
	return s.replayOwnerInfoChange(ctx, tenant, id, in, "project-initiation-info-change-v1")
}
func (s *PostgresStore) replayOwnerInfoChange(ctx context.Context, tenant, id string, in BasicInfoChangeInput, kind string) (BasicInfoChangeResult, bool, error) {
	hash, err := ownerInfoChangeHash(id, in, kind)
	if err != nil {
		return BasicInfoChangeResult{}, false, err
	}
	var result BasicInfoChangeResult
	found, _, err := lookupCommand(ctx, s.pool, tenant, in.CommandID, hash, &result)
	return result, found, err
}
func (s *PostgresStore) ChangeBasicInfo(ctx context.Context, actor Actor, id string, input BasicInfoChangeInput) (BasicInfoChangeResult, error) {
	return s.changeOwnerInfo(ctx, actor, id, input, false)
}
func (s *PostgresStore) ChangeInitiationInfo(ctx context.Context, actor Actor, id string, input BasicInfoChangeInput) (BasicInfoChangeResult, error) {
	return s.changeOwnerInfo(ctx, actor, id, input, true)
}
func (s *PostgresStore) changeOwnerInfo(ctx context.Context, actor Actor, id string, input BasicInfoChangeInput, initiation bool) (BasicInfoChangeResult, error) {
	in, err := NormalizeBasicInfoChange(input)
	if err != nil {
		return BasicInfoChangeResult{}, err
	}
	kind := "project-basic-info-change-v1"
	if initiation {
		kind = "project-initiation-info-change-v1"
	}
	hash, err := ownerInfoChangeHash(id, in, kind)
	if err != nil {
		return BasicInfoChangeResult{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return BasicInfoChangeResult{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lockCommand(ctx, tx, actor.TenantID, in.CommandID); err != nil {
		return BasicInfoChangeResult{}, err
	}
	var result BasicInfoChangeResult
	if found, _, err := lookupCommand(ctx, tx, actor.TenantID, in.CommandID, hash, &result); err != nil {
		return result, err
	} else if found {
		return result, nil
	}
	p, err := loadProjectForUpdate(ctx, tx, actor.TenantID, id)
	if err != nil {
		return result, err
	}
	if p.Revision != in.ExpectedRevision {
		return result, ErrRevisionConflict
	}
	if (!initiation && p.Status != StatusActive) || (initiation && p.Status != StatusDraft && p.Status != StatusPendingCompletion) {
		return result, ErrStateConflict
	}
	result.Owners = []OwnerChangeResult{}
	now := time.Now().UTC()
	changed := false
	if in.Profile != nil {
		fields := in.Profile.createInput(in.CommandID)
		// Store callers must also meet the original profile invariants.
		if p.NodeType == NodeTypePackage {
			_, err = NormalizePackage(PackageCreateInput{CommandID: in.CommandID, ProjectNumber: fields.ProjectNumber, Name: fields.Name, Summary: fields.Summary, PlannedStartDate: fields.PlannedStartDate, PlannedEndDate: fields.PlannedEndDate, ProjectTypeKeys: fields.ProjectTypeKeys})
		} else {
			_, err = NormalizeCreate(fields)
		}
		if err != nil {
			return result, err
		}
		changed = !sameDetails(p, fields)
		p.ProjectNumber, p.Name, p.Summary = fields.ProjectNumber, fields.Name, fields.Summary
		p.PlannedStartDate, p.PlannedEndDate, p.ProjectTypeKeys = fields.PlannedStartDate, fields.PlannedEndDate, fields.ProjectTypeKeys
	}
	for _, v := range in.Owners {
		old, err := scanAssignment(tx.QueryRow(ctx, `SELECT id,tenant_id,project_id,subject_kind,subject_id,source_revision,display_snapshot,role_key,effective_from,ended_at,end_reason,created_project_revision,ended_project_revision,created_at FROM project_assignments WHERE tenant_id=$1 AND project_id=$2 AND role_key=$3 AND ended_at IS NULL FOR UPDATE`, actor.TenantID, id, v.RoleKey))
		if err != nil && !errors.Is(err, ErrNotFound) {
			return result, err
		}
		entry := OwnerChangeResult{RoleKey: v.RoleKey}
		if err == nil {
			if old.EffectiveFrom.After(now) {
				return result, ErrStateConflict
			}
			copy := old
			entry.Previous = &copy
			entry.Current = &copy
		}
		entry.Changed = (v.Operation == "clear" && entry.Previous != nil) || (v.Operation == "set" && (entry.Previous == nil || old.SubjectKind != "person" || old.SubjectID != v.Person.PersonID))
		if entry.Changed {
			changed = true
			if entry.Previous != nil {
				previous := *entry.Previous
				revision := p.Revision + 1
				reason := in.Reason
				previous.EndedAt = &now
				previous.EndReason = &reason
				previous.EndedProjectRevision = &revision
				entry.Previous = &previous
				if _, err := tx.Exec(ctx, `UPDATE project_assignments SET ended_at=$4,end_reason=$5,ended_project_revision=$6 WHERE tenant_id=$1 AND project_id=$2 AND id=$3 AND ended_at IS NULL`, actor.TenantID, id, previous.ID, now, in.Reason, revision); err != nil {
					return result, err
				}
			}
			entry.Current = nil
			if v.Operation == "set" {
				if strings.TrimSpace(v.snapshot) == "" || len([]rune(v.snapshot)) > 200 {
					return result, fmt.Errorf("validated owner snapshot required")
				}
				current := Assignment{ID: newUUID(), TenantID: actor.TenantID, ProjectID: id, SubjectKind: "person", SubjectID: v.Person.PersonID, SourceRevision: v.Person.SourceRevision, DisplaySnapshot: v.snapshot, RoleKey: v.RoleKey, EffectiveFrom: now, CreatedProjectRevision: p.Revision + 1, CreatedAt: now}
				if _, err := tx.Exec(ctx, `INSERT INTO project_assignments(id,tenant_id,project_id,subject_kind,subject_id,source_revision,display_snapshot,role_key,effective_from,created_project_revision,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, current.ID, current.TenantID, current.ProjectID, current.SubjectKind, current.SubjectID, current.SourceRevision, current.DisplaySnapshot, current.RoleKey, current.EffectiveFrom, current.CreatedProjectRevision, current.CreatedAt); err != nil {
					return result, mapWriteError(err)
				}
				entry.Current = &current
			}
		}
		result.Owners = append(result.Owners, entry)
	}
	if changed {
		p.Revision++
		p.UpdatedAt = now
		_, err = tx.Exec(ctx, `UPDATE projects SET project_number=$3,project_number_normalized=$4,name=$5,summary=$6,planned_start_date=$7,planned_end_date=$8,project_type_keys=$9,revision=$10,updated_at=$11 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, id, p.ProjectNumber, strings.ToLower(p.ProjectNumber), p.Name, p.Summary, p.PlannedStartDate, p.PlannedEndDate, p.ProjectTypeKeys, p.Revision, p.UpdatedAt)
		if err != nil {
			return result, mapWriteError(err)
		}
		payload := projectHistoryPayload(p)
		payload["schema_version"] = kind
		payload["correction_reason"] = in.Reason
		payload["previous_revision"] = in.ExpectedRevision
		payload["owner_changes"] = result.Owners
		if err := insertHistory(ctx, tx, p, actor, in.CommandID, "project_details_replaced", payload); err != nil {
			return result, err
		}
	}
	result.Project = p
	if err := recordCommand(ctx, tx, actor.TenantID, in.CommandID, hash, 200, result, now); err != nil {
		return result, err
	}
	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}
