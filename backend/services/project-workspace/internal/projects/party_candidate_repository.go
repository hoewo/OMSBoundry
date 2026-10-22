package projects

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

const partyCandidateColumns = "party_id,role_key,source_revision,registration_command_id,registered_by,registered_at"

func (s *PostgresStore) ListPartyCandidates(ctx context.Context, tenantID string) ([]PartyCandidate, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+partyCandidateColumns+` FROM project_party_candidates WHERE tenant_id=$1 ORDER BY registered_at DESC,party_id,role_key`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []PartyCandidate{}
	for rows.Next() {
		value, err := scanPartyCandidate(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, value)
	}
	return items, rows.Err()
}

func (s *PostgresStore) ReplayPartyCandidate(ctx context.Context, tenantID string, input PartyCandidateInput) (PartyCandidate, bool, error) {
	hash, err := partyCandidateRequestHash(input)
	if err != nil {
		return PartyCandidate{}, false, err
	}
	var replay PartyCandidate
	found, status, err := lookupCommand(ctx, s.pool, tenantID, input.CommandID, hash, &replay)
	if errors.Is(err, ErrCommandConflict) || (found && status == 409) {
		return PartyCandidate{}, false, ErrCommandConflict
	}
	return replay, found, err
}

func (s *PostgresStore) RegisterPartyCandidate(ctx context.Context, actor Actor, input PartyCandidateInput) (PartyCandidate, error) {
	hash, err := partyCandidateRequestHash(input)
	if err != nil {
		return PartyCandidate{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return PartyCandidate{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lockCommand(ctx, tx, actor.TenantID, input.CommandID); err != nil {
		return PartyCandidate{}, err
	}
	var replay PartyCandidate
	if found, _, err := lookupCommand(ctx, tx, actor.TenantID, input.CommandID, hash, &replay); err != nil {
		return PartyCandidate{}, err
	} else if found {
		return replay, nil
	}
	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `INSERT INTO project_party_candidates(tenant_id,party_id,role_key,source_revision,registration_command_id,registered_by,registered_at) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (tenant_id,party_id,role_key) DO NOTHING`, actor.TenantID, input.PartyID, input.RoleKey, input.SourceRevision, input.CommandID, actor.ID, now)
	if err != nil {
		return PartyCandidate{}, mapWriteError(err)
	}
	value, err := loadPartyCandidate(ctx, tx, actor.TenantID, input.PartyID, input.RoleKey)
	if err != nil {
		return PartyCandidate{}, err
	}
	if err := recordCommand(ctx, tx, actor.TenantID, input.CommandID, hash, 201, value, now); err != nil {
		return PartyCandidate{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PartyCandidate{}, err
	}
	return value, nil
}

func partyCandidateRequestHash(input PartyCandidateInput) (string, error) {
	return requestHash(struct {
		Kind  string
		Input PartyCandidateInput
	}{"project-party-candidate-v1", input})
}

type partyCandidateScanner interface {
	Scan(...any) error
}

func scanPartyCandidate(row partyCandidateScanner) (PartyCandidate, error) {
	var value PartyCandidate
	err := row.Scan(&value.PartyID, &value.RoleKey, &value.SourceRevision, &value.RegistrationCommandID, &value.RegisteredBy, &value.RegisteredAt)
	return value, err
}

func loadPartyCandidate(ctx context.Context, tx pgx.Tx, tenantID, partyID, roleKey string) (PartyCandidate, error) {
	value, err := scanPartyCandidate(tx.QueryRow(ctx, `SELECT `+partyCandidateColumns+` FROM project_party_candidates WHERE tenant_id=$1 AND party_id=$2 AND role_key=$3`, tenantID, partyID, roleKey))
	if errors.Is(err, pgx.ErrNoRows) {
		return PartyCandidate{}, ErrNotFound
	}
	return value, err
}
