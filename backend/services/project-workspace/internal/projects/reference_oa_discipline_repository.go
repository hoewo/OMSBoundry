package projects

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	personnel "omsbizfoundry/project-workspace/internal/referenceoa/productionpersonnel"
)

var ErrOADisciplineNameConflict = errors.New("active OA discipline name conflict")

type OADiscipline struct {
	ID          string `json:"id"`
	Revision    int64  `json:"revision"`
	Name        string `json:"name"`
	Description string `json:"description"`
	State       string `json:"state"`
}
type OADisciplineCommand struct {
	CommandID        string `json:"command_id"`
	Operation        string `json:"operation"`
	ID               string `json:"id,omitempty"`
	ExpectedRevision int64  `json:"expected_revision"`
	Name             string `json:"name"`
	Description      string `json:"description"`
}

func NormalizeOADiscipline(input OADisciplineCommand) (OADisciplineCommand, error) {
	in := input
	in.CommandID = strings.ToLower(strings.TrimSpace(in.CommandID))
	in.ID = strings.ToLower(strings.TrimSpace(in.ID))
	in.Operation = strings.ToLower(strings.TrimSpace(in.Operation))
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	if !ValidUUID(in.CommandID) || len([]rune(in.Description)) > 1000 {
		return in, personnel.ErrInvalid
	}
	switch in.Operation {
	case "create":
		if in.ID != "" || in.ExpectedRevision != 0 || in.Name == "" || len([]rune(in.Name)) > 200 {
			return in, personnel.ErrInvalid
		}
	case "update":
		if !ValidUUID(in.ID) || in.ExpectedRevision < 1 || in.Name == "" || len([]rune(in.Name)) > 200 {
			return in, personnel.ErrInvalid
		}
	case "archive", "reactivate":
		if !ValidUUID(in.ID) || in.ExpectedRevision < 1 || in.Name != "" || in.Description != "" {
			return in, personnel.ErrInvalid
		}
	default:
		return in, personnel.ErrInvalid
	}
	return in, nil
}
func (s *PostgresStore) OADisciplines(ctx context.Context, tenant string, includeArchived bool) ([]OADiscipline, error) {
	if !ValidUUID(tenant) {
		return nil, personnel.ErrInvalid
	}
	rows, err := s.pool.Query(ctx, `SELECT v.id,v.revision,v.name,v.description,v.state FROM reference_oa_production_discipline_heads h JOIN reference_oa_production_discipline_versions v ON (v.tenant_id,v.id,v.revision)=(h.tenant_id,h.id,h.revision) WHERE h.tenant_id=$1 AND ($2 OR h.state='active') ORDER BY v.name COLLATE "C",v.id LIMIT 1001`, tenant, includeArchived)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []OADiscipline{}
	for rows.Next() {
		var v OADiscipline
		if err := rows.Scan(&v.ID, &v.Revision, &v.Name, &v.Description, &v.State); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	if len(result) > 1000 {
		return nil, errors.New("OA discipline catalogue exceeds supported list bound")
	}
	return result, rows.Err()
}
func (s *PostgresStore) ApplyOADiscipline(ctx context.Context, actor Actor, input OADisciplineCommand) (OADiscipline, error) {
	if !ValidUUID(actor.TenantID) || !ValidUUID(actor.ID) {
		return OADiscipline{}, personnel.ErrInvalid
	}
	in, err := NormalizeOADiscipline(input)
	if err != nil {
		return OADiscipline{}, err
	}
	hash, err := requestHash(struct {
		Kind  string
		Input OADisciplineCommand
	}{personnel.Profile + ":discipline", in})
	if err != nil {
		return OADiscipline{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return OADiscipline{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lockCommand(ctx, tx, actor.TenantID, in.CommandID); err != nil {
		return OADiscipline{}, err
	}
	var v OADiscipline
	if found, _, err := lookupCommand(ctx, tx, actor.TenantID, in.CommandID, hash, &v); err != nil {
		return v, err
	} else if found {
		return v, nil
	}
	if in.Operation == "create" {
		v = OADiscipline{ID: newUUID(), Revision: 1, Name: in.Name, Description: in.Description, State: "active"}
	} else {
		v, err = loadOADiscipline(ctx, tx, actor.TenantID, in.ID, "FOR UPDATE OF h")
		if err != nil {
			return v, err
		}
		if v.Revision != in.ExpectedRevision {
			return OADiscipline{}, ErrRevisionConflict
		}
		switch in.Operation {
		case "update":
			if v.State != "active" {
				return OADiscipline{}, ErrStateConflict
			}
			v.Name = in.Name
			v.Description = in.Description
		case "archive":
			if v.State != "active" {
				return OADiscipline{}, ErrStateConflict
			}
			v.State = "archived"
		case "reactivate":
			if v.State != "archived" {
				return OADiscipline{}, ErrStateConflict
			}
			v.State = "active"
		}
		v.Revision++
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, `INSERT INTO reference_oa_production_discipline_versions(tenant_id,id,revision,name,description,state,command_id,actor_subject,recorded_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, actor.TenantID, v.ID, v.Revision, v.Name, v.Description, v.State, in.CommandID, actor.ID, now); err != nil {
		return OADiscipline{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO reference_oa_production_discipline_heads(tenant_id,id,revision,name_normalized,state) VALUES($1,$2,$3,$4,$5) ON CONFLICT(tenant_id,id) DO UPDATE SET revision=EXCLUDED.revision,name_normalized=EXCLUDED.name_normalized,state=EXCLUDED.state`, actor.TenantID, v.ID, v.Revision, strings.ToLower(v.Name), v.State); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.ConstraintName == "reference_oa_production_discipline_active_name_idx" {
			return OADiscipline{}, ErrOADisciplineNameConflict
		}
		return OADiscipline{}, err
	}
	if err := recordCommand(ctx, tx, actor.TenantID, in.CommandID, hash, 200, v, now); err != nil {
		return OADiscipline{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return OADiscipline{}, err
	}
	return v, nil
}
func loadOADiscipline(ctx context.Context, tx pgx.Tx, tenant, id, suffix string) (OADiscipline, error) {
	var v OADiscipline
	err := tx.QueryRow(ctx, `SELECT v.id,v.revision,v.name,v.description,v.state FROM reference_oa_production_discipline_heads h JOIN reference_oa_production_discipline_versions v ON (v.tenant_id,v.id,v.revision)=(h.tenant_id,h.id,h.revision) WHERE h.tenant_id=$1 AND h.id=$2 `+suffix, tenant, id).Scan(&v.ID, &v.Revision, &v.Name, &v.Description, &v.State)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, ErrNotFound
	}
	return v, err
}
