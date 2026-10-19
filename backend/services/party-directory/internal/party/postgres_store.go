package party

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
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

func (s *PostgresStore) Create(ctx context.Context, actor Actor, input CreateInput, hash string) (Party, error) {
	tx, err := s.begin(ctx, actor.TenantID, input.CommandID)
	if err != nil {
		return Party{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, replayErr := replay(ctx, tx, actor, input.CommandID, hash); replayErr != nil || found {
		return value, replayErr
	}
	now := time.Now().UTC()
	value := Party{ID: newUUID(), TenantID: actor.TenantID, Kind: input.Kind, LegalName: input.LegalName, RegistrationIdentifier: input.RegistrationIdentifier, Status: StatusActive, Revision: 1, CreatedAt: now, UpdatedAt: now}
	_, err = tx.Exec(ctx, `INSERT INTO parties(id,tenant_id,kind,legal_name,legal_name_normalized,registration_identifier,registration_identifier_normalized,status,revision,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,'active',1,$8,$8)`, value.ID, value.TenantID, value.Kind, value.LegalName, NormalizeName(value.LegalName), value.RegistrationIdentifier, normalizeIdentifier(value.RegistrationIdentifier), now)
	if err != nil {
		return Party{}, err
	}
	if err = insertRevision(ctx, tx, value, actor, input.CommandID, EventCreated, ""); err != nil {
		return Party{}, err
	}
	if err = insertAudit(ctx, tx, value, actor, input.CommandID, 0, "party_created", ""); err != nil {
		return Party{}, err
	}
	if err = recordCommand(ctx, tx, actor, input.CommandID, hash, value, now); err != nil {
		return Party{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Party{}, err
	}
	return value, nil
}

func (s *PostgresStore) List(ctx context.Context, tenantID string, filter ListFilter) ([]Party, error) {
	query := `SELECT id,tenant_id,kind,legal_name,registration_identifier,status,revision,created_at,updated_at FROM parties WHERE tenant_id=$1 AND ($2='' OR kind=$2) AND ($3='all' OR status=$3) AND ($4='' OR legal_name_normalized LIKE '%'||$4||'%' OR COALESCE(registration_identifier_normalized,'') LIKE '%'||$4||'%')`
	args := []any{tenantID, filter.Kind, filter.Status, filter.Query}
	if filter.Cursor != nil {
		query += ` AND (legal_name_normalized,legal_name,id) > ($5,$6,$7)`
		args = append(args, filter.Cursor.NormalizedName, filter.Cursor.LegalName, filter.Cursor.PartyID)
	}
	query += fmt.Sprintf(` ORDER BY legal_name_normalized,legal_name,id LIMIT $%d`, len(args)+1)
	args = append(args, filter.Limit+1)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Party, 0, filter.Limit+1)
	for rows.Next() {
		value, scanErr := scanParty(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, value)
	}
	return items, rows.Err()
}

func (s *PostgresStore) Get(ctx context.Context, tenantID, id string) (Party, error) {
	return loadParty(ctx, s.pool, tenantID, id, "")
}

func (s *PostgresStore) History(ctx context.Context, tenantID, id string, before *int64, limit int) (HistoryPage, error) {
	if _, err := loadParty(ctx, s.pool, tenantID, id, ""); err != nil {
		return HistoryPage{}, err
	}
	upper := int64(9223372036854775807)
	if before != nil {
		upper = *before
	}
	rows, err := s.pool.Query(ctx, `SELECT party_id,tenant_id,revision,event_kind,command_id,kind,legal_name,registration_identifier,status,reason,actor_ref,occurred_at FROM party_revisions WHERE tenant_id=$1 AND party_id=$2 AND revision<$3 ORDER BY revision DESC LIMIT $4`, tenantID, id, upper, limit+1)
	if err != nil {
		return HistoryPage{}, err
	}
	defer rows.Close()
	items := make([]Revision, 0, limit+1)
	for rows.Next() {
		var value Revision
		if err = rows.Scan(&value.PartyID, &value.TenantID, &value.Revision, &value.EventKind, &value.CommandID, &value.Kind, &value.LegalName, &value.RegistrationIdentifier, &value.Status, &value.Reason, &value.ActorRef, &value.OccurredAt); err != nil {
			return HistoryPage{}, err
		}
		items = append(items, value)
	}
	if err = rows.Err(); err != nil {
		return HistoryPage{}, err
	}
	page := HistoryPage{Items: items}
	if len(items) > limit {
		visible := items[:limit]
		next := visible[len(visible)-1].Revision
		page.Items = visible
		page.NextBeforeRevision = &next
	}
	return page, nil
}

func (s *PostgresStore) Update(ctx context.Context, actor Actor, id string, input UpdateInput, hash string) (Party, error) {
	return s.mutate(ctx, actor, id, input.CommandID, hash, input.ExpectedRevision, func(value *Party) (string, string, error) {
		if value.Kind == input.Kind && value.LegalName == input.LegalName && sameOptional(value.RegistrationIdentifier, input.RegistrationIdentifier) {
			return "", "", ErrNoEffectiveChange
		}
		value.Kind = input.Kind
		value.LegalName = input.LegalName
		value.RegistrationIdentifier = input.RegistrationIdentifier
		return EventProfileUpdated, "party_profile_updated", nil
	}, input.Reason)
}

func (s *PostgresStore) ChangeStatus(ctx context.Context, actor Actor, id string, input StatusInput, hash string) (Party, error) {
	return s.mutate(ctx, actor, id, input.CommandID, hash, input.ExpectedRevision, func(value *Party) (string, string, error) {
		if value.Status == input.Status {
			return "", "", ErrNoEffectiveChange
		}
		value.Status = input.Status
		return EventStatusChanged, "party_status_changed", nil
	}, input.Reason)
}

type changeFunc func(*Party) (string, string, error)

func (s *PostgresStore) mutate(ctx context.Context, actor Actor, id, commandID, hash string, expected int64, change changeFunc, reason string) (Party, error) {
	tx, err := s.begin(ctx, actor.TenantID, commandID)
	if err != nil {
		return Party{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, replayErr := replay(ctx, tx, actor, commandID, hash); replayErr != nil || found {
		return value, replayErr
	}
	value, err := loadParty(ctx, tx, actor.TenantID, id, "FOR UPDATE")
	if err != nil {
		return Party{}, err
	}
	if value.Revision != expected {
		return Party{}, ErrRevisionConflict
	}
	eventKind, auditAction, err := change(&value)
	if err != nil {
		return Party{}, err
	}
	source := value.Revision
	value.Revision++
	value.UpdatedAt = time.Now().UTC()
	_, err = tx.Exec(ctx, `UPDATE parties SET kind=$3,legal_name=$4,legal_name_normalized=$5,registration_identifier=$6,registration_identifier_normalized=$7,status=$8,revision=$9,updated_at=$10 WHERE tenant_id=$1 AND id=$2`, value.TenantID, value.ID, value.Kind, value.LegalName, NormalizeName(value.LegalName), value.RegistrationIdentifier, normalizeIdentifier(value.RegistrationIdentifier), value.Status, value.Revision, value.UpdatedAt)
	if err != nil {
		return Party{}, err
	}
	if err = insertRevision(ctx, tx, value, actor, commandID, eventKind, reason); err != nil {
		return Party{}, err
	}
	if err = insertAudit(ctx, tx, value, actor, commandID, source, auditAction, reason); err != nil {
		return Party{}, err
	}
	if err = recordCommand(ctx, tx, actor, commandID, hash, value, value.UpdatedAt); err != nil {
		return Party{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Party{}, err
	}
	return value, nil
}

func (s *PostgresStore) begin(ctx context.Context, tenantID, commandID string) (pgx.Tx, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, tenantID+":"+commandID); err != nil {
		_ = tx.Rollback(context.Background())
		return nil, err
	}
	return tx, nil
}

func replay(ctx context.Context, tx pgx.Tx, actor Actor, commandID, hash string) (Party, bool, error) {
	var stored, storedActor string
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT request_hash,actor_subject::text,response_json FROM party_commands WHERE tenant_id=$1 AND command_id=$2`, actor.TenantID, commandID).Scan(&stored, &storedActor, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Party{}, false, nil
	}
	if err != nil {
		return Party{}, false, err
	}
	if stored != hash || storedActor != actor.ID {
		return Party{}, false, ErrCommandConflict
	}
	var value Party
	if err = json.Unmarshal(raw, &value); err != nil {
		return Party{}, false, err
	}
	return value, true, nil
}
func recordCommand(ctx context.Context, tx pgx.Tx, actor Actor, commandID, hash string, value Party, now time.Time) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO party_commands(tenant_id,command_id,request_hash,actor_subject,response_json,created_at) VALUES($1,$2,$3,$4,$5,$6)`, actor.TenantID, commandID, hash, actor.ID, raw, now)
	return err
}
func insertRevision(ctx context.Context, tx pgx.Tx, value Party, actor Actor, commandID, eventKind, reason string) error {
	_, err := tx.Exec(ctx, `INSERT INTO party_revisions(tenant_id,party_id,revision,event_kind,command_id,kind,legal_name,registration_identifier,status,reason,actor_ref,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, value.TenantID, value.ID, value.Revision, eventKind, commandID, value.Kind, value.LegalName, value.RegistrationIdentifier, value.Status, reason, actor.Kind+":"+actor.ID, value.UpdatedAt)
	return err
}
func insertAudit(ctx context.Context, tx pgx.Tx, value Party, actor Actor, commandID string, source int64, action, reason string) error {
	_, err := tx.Exec(ctx, `INSERT INTO party_audit_events(id,tenant_id,party_id,command_id,source_revision,result_revision,action,reason,actor_subject,actor_kind,credential_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, newUUID(), value.TenantID, value.ID, commandID, source, value.Revision, action, reason, actor.ID, actor.Kind, actor.CredentialID, value.UpdatedAt)
	return err
}

type rowScanner interface{ Scan(...any) error }

func scanParty(row rowScanner) (Party, error) {
	var value Party
	err := row.Scan(&value.ID, &value.TenantID, &value.Kind, &value.LegalName, &value.RegistrationIdentifier, &value.Status, &value.Revision, &value.CreatedAt, &value.UpdatedAt)
	return value, err
}
func loadParty(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, tenantID, id, suffix string) (Party, error) {
	value, err := scanParty(q.QueryRow(ctx, `SELECT id,tenant_id,kind,legal_name,registration_identifier,status,revision,created_at,updated_at FROM parties WHERE tenant_id=$1 AND id=$2 `+suffix, tenantID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Party{}, ErrNotFound
	}
	return value, err
}
func normalizeIdentifier(value *string) *string {
	if value == nil {
		return nil
	}
	normalized := strings.ToLower(strings.TrimSpace(*value))
	return &normalized
}
func sameOptional(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
func newUUID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(err)
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16])
}
