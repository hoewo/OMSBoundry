package projects

// Native storage only: the explicitly selected service/HTTP composition must
// authenticate and authorize before supplying a trusted Actor and People
// resolutions. Project-owned discipline resolution is locked in this store.
import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	personnel "omsbizfoundry/project-workspace/internal/referenceoa/productionpersonnel"
)

type OAProductionFact struct {
	personnel.Assignment
	StartedAt            time.Time  `json:"started_at"`
	EndedAt              *time.Time `json:"ended_at,omitempty"`
	EndedProjectRevision *int64     `json:"ended_project_revision,omitempty"`
	EndReason            *string    `json:"end_reason,omitempty"`
}
type OAProductionView struct {
	Profile                  string             `json:"profile"`
	ProjectID                string             `json:"project_id"`
	ProjectRevision          int64              `json:"project_revision"`
	ProjectStatus            string             `json:"project_status"`
	ParticipantNotApplicable bool               `json:"participant_not_applicable"`
	Facts                    []OAProductionFact `json:"facts"`
}
type OAProductionProfileContext struct {
	Profile         string `json:"profile"`
	ProjectID       string `json:"project_id"`
	ProjectRevision int64  `json:"project_revision"`
	ProjectStatus   string `json:"project_status"`
	Installed       bool   `json:"installed"`
}
type OAProductionReceipt struct {
	Profile                  string             `json:"profile"`
	ProjectID                string             `json:"project_id"`
	ProjectRevision          int64              `json:"project_revision"`
	Changed                  bool               `json:"changed"`
	ParticipantNotApplicable bool               `json:"participant_not_applicable"`
	EndIDs                   []string           `json:"end_ids"`
	Added                    []OAProductionFact `json:"added"`
}

// OAProductionProfile is the target-owned, read-only capability context used
// before the optional personnel profile is installed. It deliberately does
// not create a profile or infer authority from a Workbench/Project role.
func (s *PostgresStore) OAProductionProfile(ctx context.Context, tenant, id string) (OAProductionProfileContext, error) {
	if !uuidPattern.MatchString(tenant) || !uuidPattern.MatchString(id) {
		return OAProductionProfileContext{}, personnel.ErrInvalid
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return OAProductionProfileContext{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	p, err := loadProject(ctx, tx, tenant, id, "")
	if err != nil {
		return OAProductionProfileContext{}, err
	}
	if p.NodeType != NodeTypeProject {
		return OAProductionProfileContext{}, ErrStateConflict
	}
	var installed bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM reference_oa_production_profiles WHERE tenant_id=$1 AND project_id=$2 AND profile_key=$3)`, tenant, id, personnel.Profile).Scan(&installed); err != nil {
		return OAProductionProfileContext{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return OAProductionProfileContext{}, err
	}
	return OAProductionProfileContext{Profile: personnel.Profile, ProjectID: p.ID, ProjectRevision: p.Revision, ProjectStatus: p.Status, Installed: installed}, nil
}

// EnrollOAProduction is deliberately explicit and internal. Migrating a normal
// Base Project does not select the overlay or expand any existing capability.
func (s *PostgresStore) EnrollOAProduction(ctx context.Context, actor Actor, id string, input RevisionCommand) (OAProductionReceipt, error) {
	if !validOAActor(actor, id) || !uuidPattern.MatchString(input.CommandID) || input.ExpectedRevision < 1 || input.InitiationProfile != "" {
		return OAProductionReceipt{}, personnel.ErrInvalid
	}
	hash, err := requestHash(struct {
		Kind, ID string
		Input    RevisionCommand
	}{personnel.Profile + ":install", id, input})
	if err != nil {
		return OAProductionReceipt{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return OAProductionReceipt{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lockCommand(ctx, tx, actor.TenantID, input.CommandID); err != nil {
		return OAProductionReceipt{}, err
	}
	var receipt OAProductionReceipt
	if found, _, err := lookupCommand(ctx, tx, actor.TenantID, input.CommandID, hash, &receipt); err != nil {
		return receipt, err
	} else if found {
		return receipt, nil
	}
	p, err := loadProjectForUpdate(ctx, tx, actor.TenantID, id)
	if err != nil {
		return receipt, err
	}
	if p.Revision != input.ExpectedRevision {
		return receipt, ErrRevisionConflict
	}
	if p.NodeType != NodeTypeProject || p.Status == StatusCancelled {
		return receipt, ErrStateConflict
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM reference_oa_production_profiles WHERE tenant_id=$1 AND project_id=$2)`, actor.TenantID, id).Scan(&exists); err != nil {
		return receipt, err
	}
	if exists {
		return receipt, ErrStateConflict
	}
	p.Revision++
	p.UpdatedAt = time.Now().UTC()
	if err := updateProjectRevision(ctx, tx, p); err != nil {
		return receipt, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO reference_oa_production_profiles(tenant_id,project_id,profile_key,installed_project_revision,installed_at) VALUES($1,$2,$3,$4,$5)`, actor.TenantID, id, personnel.Profile, p.Revision, p.UpdatedAt); err != nil {
		return receipt, err
	}
	receipt = oaReceipt(p, false)
	receipt.Changed = true
	if err := recordOAProduction(ctx, tx, actor, p, input.CommandID, hash, "profile_installed", receipt); err != nil {
		return OAProductionReceipt{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return OAProductionReceipt{}, err
	}
	return receipt, nil
}

func (s *PostgresStore) OAProductionPersonnel(ctx context.Context, tenant, id string, includeEnded bool) (OAProductionView, error) {
	if !uuidPattern.MatchString(tenant) || !uuidPattern.MatchString(id) {
		return OAProductionView{}, personnel.ErrInvalid
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return OAProductionView{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	p, err := loadProject(ctx, tx, tenant, id, "")
	if err != nil {
		return OAProductionView{}, err
	}
	v, err := loadOAProduction(ctx, tx, p, includeEnded)
	if err != nil {
		return OAProductionView{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return OAProductionView{}, err
	}
	return v, nil
}

func (s *PostgresStore) ReplayOAProduction(ctx context.Context, tenant, id string, input personnel.Intention) (OAProductionReceipt, bool, error) {
	if !uuidPattern.MatchString(tenant) || !uuidPattern.MatchString(id) {
		return OAProductionReceipt{}, false, personnel.ErrInvalid
	}
	in, hash, err := oaProductionHash(id, input)
	if err != nil {
		return OAProductionReceipt{}, false, err
	}
	var receipt OAProductionReceipt
	found, _, err := lookupCommand(ctx, s.pool, tenant, in.CommandID, hash, &receipt)
	return receipt, found, err
}

func (s *PostgresStore) ApplyOAProductionPersonnel(ctx context.Context, actor Actor, id string, input personnel.Intention, resolved personnel.Resolved) (OAProductionReceipt, error) {
	if !validOAActor(actor, id) {
		return OAProductionReceipt{}, personnel.ErrInvalid
	}
	in, hash, err := oaProductionHash(id, input)
	if err != nil {
		return OAProductionReceipt{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return OAProductionReceipt{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lockCommand(ctx, tx, actor.TenantID, in.CommandID); err != nil {
		return OAProductionReceipt{}, err
	}
	var receipt OAProductionReceipt
	// A committed reply is returned before source access, stale checks or writes.
	if found, _, err := lookupCommand(ctx, tx, actor.TenantID, in.CommandID, hash, &receipt); err != nil {
		return receipt, err
	} else if found {
		return receipt, nil
	}
	p, err := loadProjectForUpdate(ctx, tx, actor.TenantID, id)
	if err != nil {
		return receipt, err
	}
	if p.NodeType != NodeTypeProject {
		return receipt, ErrStateConflict
	}
	view, err := loadOAProduction(ctx, tx, p, false)
	if err != nil {
		return receipt, err
	}
	if in.Operation == personnel.ReplaceDiscipline {
		// Source owned by this Project database: lock its head against rename or
		// archive until the personnel transaction commits. Never trust a name
		// or active flag supplied by a UI or an external catalogue.
		d, err := loadOADiscipline(ctx, tx, actor.TenantID, in.TargetDiscipline.ID, "FOR SHARE OF h")
		if err != nil {
			return receipt, err
		}
		owned := personnel.ResolvedDiscipline{TenantID: actor.TenantID, State: d.State, DisciplineSnapshot: personnel.DisciplineSnapshot{Ref: personnel.Ref{ID: d.ID, Revision: d.Revision}, Name: d.Name}}
		if resolved.Discipline != nil && *resolved.Discipline != owned {
			return receipt, personnel.ErrSource
		}
		resolved.Discipline = &owned
	}
	current := personnel.Current{Profile: view.Profile, TenantID: actor.TenantID, ProjectID: id, ProjectRevision: p.Revision, ProjectStatus: p.Status, ParticipantNotApplicable: view.ParticipantNotApplicable, Active: []personnel.Assignment{}}
	for _, fact := range view.Facts {
		current.Active = append(current.Active, fact.Assignment)
	}
	delta, err := personnel.Prepare(current, in, resolved)
	if err != nil {
		return receipt, err
	}
	receipt = oaReceipt(p, view.ParticipantNotApplicable)
	receipt.Changed = len(delta.EndIDs)+len(delta.Add) > 0
	if receipt.Changed {
		p.Revision++
		p.UpdatedAt = time.Now().UTC()
		if err := updateProjectRevision(ctx, tx, p); err != nil {
			return OAProductionReceipt{}, err
		}
		receipt.ProjectRevision = p.Revision
		for _, factID := range delta.EndIDs {
			tag, err := tx.Exec(ctx, `UPDATE reference_oa_production_assignments SET ended_at=$4,ended_project_revision=$5,end_reason=$6 WHERE tenant_id=$1 AND project_id=$2 AND id=$3 AND ended_at IS NULL`, actor.TenantID, id, factID, p.UpdatedAt, p.Revision, "reference-oa:"+in.Operation)
			if err != nil {
				return OAProductionReceipt{}, err
			}
			if tag.RowsAffected() != 1 {
				return OAProductionReceipt{}, ErrRevisionConflict
			}
			receipt.EndIDs = append(receipt.EndIDs, factID)
		}
		for _, added := range delta.Add {
			fact := OAProductionFact{Assignment: personnel.Assignment{ID: newUUID(), Role: added.Role, Person: added.Person, Discipline: added.Discipline, CreatedProjectRevision: p.Revision}, StartedAt: p.UpdatedAt}
			var disciplineID, disciplineName *string
			var disciplineRevision *int64
			if fact.Discipline != nil {
				disciplineID = &fact.Discipline.ID
				disciplineName = &fact.Discipline.Name
				disciplineRevision = &fact.Discipline.Revision
			}
			if _, err := tx.Exec(ctx, `INSERT INTO reference_oa_production_assignments(id,tenant_id,project_id,role_key,person_id,person_revision,person_name,discipline_id,discipline_revision,discipline_name,created_project_revision,started_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, fact.ID, actor.TenantID, id, fact.Role, fact.Person.ID, fact.Person.Revision, fact.Person.Name, disciplineID, disciplineRevision, disciplineName, p.Revision, p.UpdatedAt); err != nil {
				return OAProductionReceipt{}, err
			}
			receipt.Added = append(receipt.Added, fact)
		}
	}
	if err := recordOAProduction(ctx, tx, actor, p, in.CommandID, hash, "personnel_changed", receipt); err != nil {
		return OAProductionReceipt{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return OAProductionReceipt{}, err
	}
	return receipt, nil
}

// This flag is scoped to the closed personnel profile, not a general OA
// applicability bag. Toggling it never changes or ends any assignment.
func (s *PostgresStore) SetOAProductionParticipantApplicable(ctx context.Context, actor Actor, id string, input RevisionCommand, notApplicable bool) (OAProductionReceipt, error) {
	if !validOAActor(actor, id) || !uuidPattern.MatchString(input.CommandID) || input.ExpectedRevision < 1 || input.InitiationProfile != "" {
		return OAProductionReceipt{}, personnel.ErrInvalid
	}
	hash, err := requestHash(struct {
		Kind, ID      string
		Input         RevisionCommand
		NotApplicable bool
	}{personnel.Profile + ":applicability", id, input, notApplicable})
	if err != nil {
		return OAProductionReceipt{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return OAProductionReceipt{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lockCommand(ctx, tx, actor.TenantID, input.CommandID); err != nil {
		return OAProductionReceipt{}, err
	}
	var receipt OAProductionReceipt
	if found, _, err := lookupCommand(ctx, tx, actor.TenantID, input.CommandID, hash, &receipt); err != nil {
		return receipt, err
	} else if found {
		return receipt, nil
	}
	p, err := loadProjectForUpdate(ctx, tx, actor.TenantID, id)
	if err != nil {
		return receipt, err
	}
	if p.Revision != input.ExpectedRevision {
		return receipt, ErrRevisionConflict
	}
	if p.NodeType != NodeTypeProject || p.Status == StatusCancelled {
		return receipt, ErrStateConflict
	}
	v, err := loadOAProduction(ctx, tx, p, false)
	if err != nil {
		return receipt, err
	}
	receipt = oaReceipt(p, notApplicable)
	receipt.Changed = v.ParticipantNotApplicable != notApplicable
	if receipt.Changed {
		p.Revision++
		p.UpdatedAt = time.Now().UTC()
		receipt.ProjectRevision = p.Revision
		if err := updateProjectRevision(ctx, tx, p); err != nil {
			return OAProductionReceipt{}, err
		}
		if _, err := tx.Exec(ctx, `UPDATE reference_oa_production_profiles SET participant_not_applicable=$3 WHERE tenant_id=$1 AND project_id=$2`, actor.TenantID, id, notApplicable); err != nil {
			return OAProductionReceipt{}, err
		}
	}
	if err := recordOAProduction(ctx, tx, actor, p, input.CommandID, hash, "participant_applicability_changed", receipt); err != nil {
		return OAProductionReceipt{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return OAProductionReceipt{}, err
	}
	return receipt, nil
}

func validOAActor(a Actor, id string) bool {
	return uuidPattern.MatchString(a.ID) && uuidPattern.MatchString(a.TenantID) && uuidPattern.MatchString(id)
}
func oaReceipt(p Project, notApplicable bool) OAProductionReceipt {
	return OAProductionReceipt{Profile: personnel.Profile, ProjectID: p.ID, ProjectRevision: p.Revision, ParticipantNotApplicable: notApplicable, EndIDs: []string{}, Added: []OAProductionFact{}}
}
func oaProductionHash(id string, input personnel.Intention) (personnel.Intention, string, error) {
	in, err := personnel.Normalize(input)
	if err != nil {
		return in, "", err
	}
	hash, err := requestHash(struct {
		Kind, ID string
		Input    personnel.Intention
	}{personnel.Profile + ":personnel", id, in})
	return in, hash, err
}
func recordOAProduction(ctx context.Context, tx pgx.Tx, actor Actor, p Project, command, hash, event string, receipt OAProductionReceipt) error {
	if receipt.Changed {
		raw, err := json.Marshal(receipt)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO reference_oa_production_events(id,tenant_id,project_id,project_revision,event_type,command_id,actor_subject,occurred_at,payload) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, newUUID(), actor.TenantID, p.ID, p.Revision, event, command, actor.ID, p.UpdatedAt, raw); err != nil {
			return err
		}
	}
	return recordCommand(ctx, tx, actor.TenantID, command, hash, 200, receipt, p.UpdatedAt)
}
func loadOAProduction(ctx context.Context, tx pgx.Tx, p Project, includeEnded bool) (OAProductionView, error) {
	v := OAProductionView{ProjectID: p.ID, ProjectRevision: p.Revision, ProjectStatus: p.Status, Facts: []OAProductionFact{}}
	if err := tx.QueryRow(ctx, `SELECT profile_key,participant_not_applicable FROM reference_oa_production_profiles WHERE tenant_id=$1 AND project_id=$2`, p.TenantID, p.ID).Scan(&v.Profile, &v.ParticipantNotApplicable); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v, ErrNotFound
		}
		return v, err
	}
	rows, err := tx.Query(ctx, `SELECT id,role_key,person_id,person_revision,person_name,discipline_id,discipline_revision,discipline_name,created_project_revision,started_at,ended_at,ended_project_revision,end_reason FROM reference_oa_production_assignments WHERE tenant_id=$1 AND project_id=$2 AND ($3 OR ended_at IS NULL) ORDER BY started_at,id`, p.TenantID, p.ID, includeEnded)
	if err != nil {
		return v, err
	}
	defer rows.Close()
	for rows.Next() {
		var f OAProductionFact
		var disciplineID, disciplineName *string
		var disciplineRevision *int64
		if err := rows.Scan(&f.ID, &f.Role, &f.Person.ID, &f.Person.Revision, &f.Person.Name, &disciplineID, &disciplineRevision, &disciplineName, &f.CreatedProjectRevision, &f.StartedAt, &f.EndedAt, &f.EndedProjectRevision, &f.EndReason); err != nil {
			return v, err
		}
		if disciplineID != nil && disciplineRevision != nil && disciplineName != nil {
			f.Discipline = &personnel.DisciplineSnapshot{Ref: personnel.Ref{ID: *disciplineID, Revision: *disciplineRevision}, Name: *disciplineName}
		}
		v.Facts = append(v.Facts, f)
	}
	return v, rows.Err()
}
