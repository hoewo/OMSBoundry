package projects

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	personnel "omsbizfoundry/project-workspace/internal/referenceoa/productionpersonnel"
)

const OAProductionAssignmentResource = "reference_oa_production_assignment"

// A distinct, versioned read port. It never converts customer roles into Base
// assignments, and freezes only Project-owned facts (not an IAM directory).
type OAProductionAssignmentReference struct {
	SchemaVersion            string                        `json:"schema_version"`
	SourceApplication        string                        `json:"source_application"`
	Profile                  string                        `json:"profile"`
	ResourceType             string                        `json:"resource_type"`
	TenantID                 string                        `json:"tenant_id"`
	ProjectID                string                        `json:"project_id"`
	ProjectRevision          int64                         `json:"project_revision"`
	ProjectStatus            string                        `json:"project_status"`
	ID                       string                        `json:"id"`
	Revision                 int64                         `json:"revision"`
	RoleKey                  string                        `json:"role_key"`
	Person                   personnel.PersonSnapshot      `json:"person"`
	Discipline               *personnel.DisciplineSnapshot `json:"discipline,omitempty"`
	State                    string                        `json:"state"`
	Eligible                 bool                          `json:"eligible"`
	ParticipantNotApplicable bool                          `json:"participant_not_applicable"`
}
type OAProductionAssignmentReferences struct {
	SchemaVersion     string                            `json:"schema_version"`
	SourceApplication string                            `json:"source_application"`
	Profile           string                            `json:"profile"`
	ResourceType      string                            `json:"resource_type"`
	TenantID          string                            `json:"tenant_id"`
	ProjectID         string                            `json:"project_id"`
	ProjectRevision   int64                             `json:"project_revision"`
	ProjectStatus     string                            `json:"project_status"`
	Items             []OAProductionAssignmentReference `json:"items"`
}

func assignmentReference(p Project, na bool, f OAProductionFact) OAProductionAssignmentReference {
	v := OAProductionAssignmentReference{SchemaVersion: "1", SourceApplication: "project-workspace", Profile: personnel.Profile, ResourceType: OAProductionAssignmentResource, TenantID: p.TenantID, ProjectID: p.ID, ProjectRevision: p.Revision, ProjectStatus: p.Status, ID: f.ID, Revision: f.CreatedProjectRevision, RoleKey: f.Role, Person: f.Person, Discipline: f.Discipline, State: "active", Eligible: p.Status == StatusActive, ParticipantNotApplicable: na}
	if f.EndedAt != nil {
		v.State = "ended"
		v.Eligible = false
		v.Revision = *f.EndedProjectRevision
	}
	return v
}

// Empty assignmentID means bounded active discovery. Single-item lookup can
// still describe an ended reference, so consumers can reject new writes while
// their own immutable historical snapshots remain readable.
func (s *PostgresStore) OAProductionAssignmentReferences(ctx context.Context, tenant, projectID, assignmentID string) (OAProductionAssignmentReferences, error) {
	v := OAProductionAssignmentReferences{SchemaVersion: "1", SourceApplication: "project-workspace", Profile: personnel.Profile, ResourceType: OAProductionAssignmentResource, Items: []OAProductionAssignmentReference{}}
	if !uuidPattern.MatchString(tenant) || !uuidPattern.MatchString(projectID) || assignmentID != "" && !uuidPattern.MatchString(assignmentID) {
		return v, personnel.ErrInvalid
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return v, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	p, err := loadProject(ctx, tx, tenant, projectID, "")
	if err != nil {
		return v, err
	}
	v.TenantID, v.ProjectID, v.ProjectRevision, v.ProjectStatus = tenant, projectID, p.Revision, p.Status
	var profile string
	var na bool
	if err = tx.QueryRow(ctx, `SELECT profile_key,participant_not_applicable FROM reference_oa_production_profiles WHERE tenant_id=$1 AND project_id=$2`, tenant, projectID).Scan(&profile, &na); errors.Is(err, pgx.ErrNoRows) {
		// List discovery for a real, unenrolled Project is a valid empty set.
		// It must not auto-enroll the Overlay or turn a business prerequisite
		// into a broken reference. Exact assignment lookup remains not found.
		if assignmentID != "" {
			return v, ErrNotFound
		}
		if err = tx.Commit(ctx); err != nil {
			return v, err
		}
		return v, nil
	} else if err != nil {
		return v, err
	}
	if profile != personnel.Profile {
		return v, ErrStateConflict
	}
	rows, err := tx.Query(ctx, `SELECT id,role_key,person_id,person_revision,person_name,discipline_id,discipline_revision,discipline_name,created_project_revision,started_at,ended_at,ended_project_revision,end_reason FROM reference_oa_production_assignments WHERE tenant_id=$1 AND project_id=$2 AND (($3='' AND ended_at IS NULL) OR id::text=$3) ORDER BY started_at,id LIMIT 1001`, tenant, projectID, assignmentID)
	if err != nil {
		return v, err
	}
	for rows.Next() {
		var f OAProductionFact
		var did, dname *string
		var drev *int64
		if err = rows.Scan(&f.ID, &f.Role, &f.Person.ID, &f.Person.Revision, &f.Person.Name, &did, &drev, &dname, &f.CreatedProjectRevision, &f.StartedAt, &f.EndedAt, &f.EndedProjectRevision, &f.EndReason); err != nil {
			rows.Close()
			return v, err
		}
		if did != nil && drev != nil && dname != nil {
			f.Discipline = &personnel.DisciplineSnapshot{Ref: personnel.Ref{ID: *did, Revision: *drev}, Name: *dname}
		}
		if f.EndedAt != nil && f.EndedProjectRevision == nil {
			rows.Close()
			return v, ErrStateConflict
		}
		v.Items = append(v.Items, assignmentReference(p, na, f))
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return v, err
	}
	if len(v.Items) > 1000 {
		return v, fmt.Errorf("OA production reference discovery exceeds 1000 assignments")
	}
	if assignmentID != "" && len(v.Items) != 1 {
		return v, ErrNotFound
	}
	if err = tx.Commit(ctx); err != nil {
		return v, err
	}
	return v, nil
}

func (s *OAProductionService) AssignmentReferences(ctx context.Context, tenant, project, assignment string) (OAProductionAssignmentReferences, error) {
	return s.store.OAProductionAssignmentReferences(ctx, tenant, project, assignment)
}
