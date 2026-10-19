package invoice

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
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

func (s *PostgresStore) Replay(ctx context.Context, actor Actor, commandID, hash string) (Registration, bool, error) {
	return replay(ctx, s.pool, actor, commandID, hash)
}

func (s *PostgresStore) Create(ctx context.Context, actor Actor, input CreateInput, resolved ResolvedCreate, hash string) (Registration, error) {
	tx, err := s.begin(ctx, actor.TenantID, input.CommandID)
	if err != nil {
		return Registration{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, replayErr := replay(ctx, tx, actor, input.CommandID, hash); replayErr != nil || found {
		return value, replayErr
	}
	now := time.Now().UTC()
	value := Registration{ID: newUUID(), TenantID: actor.TenantID, Status: StatusRecorded, IssuedOn: input.IssuedOn, AmountMinor: input.AmountMinor, Currency: input.Currency, IssuerParty: resolved.IssuerParty, RecipientParty: resolved.RecipientParty, SourceRefs: resolved.SourceRefs, EvidenceRefs: []EvidenceRef{}, Note: input.Note, Revision: 1, CreatedAt: now, UpdatedAt: now}
	if err = insertCurrent(ctx, tx, value); err != nil {
		return Registration{}, err
	}
	if err = insertRevision(ctx, tx, value, actor, input.CommandID, EventRecorded, "initial registration"); err != nil {
		return Registration{}, err
	}
	if err = insertAudit(ctx, tx, value, actor, input.CommandID, 0, "invoice_registration_recorded", "initial registration"); err != nil {
		return Registration{}, err
	}
	if err = recordCommand(ctx, tx, actor, input.CommandID, hash, value, now); err != nil {
		return Registration{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Registration{}, err
	}
	return value, nil
}

func (s *PostgresStore) Get(ctx context.Context, tenantID, id string) (Registration, error) {
	return loadRegistration(ctx, s.pool, tenantID, id, "")
}

func (s *PostgresStore) List(ctx context.Context, tenantID string, filter ListFilter) (ListPage, error) {
	const where = ` tenant_id=$1
		AND ($2='' OR source_refs @> jsonb_build_array(jsonb_build_object('kind','project','resource_id',$2)))
		AND ($3='' OR issuer_party->>'id'=$3 OR recipient_party->>'id'=$3)
		AND ($4='' OR status=$4)
		AND ($5='' OR issued_on>=NULLIF($5,'')::date)
		AND ($6='' OR issued_on<=NULLIF($6,'')::date)`
	args := []any{tenantID, filter.ProjectID, filter.PartyID, filter.Status, filter.IssuedFrom, filter.IssuedTo}
	rows, err := s.pool.Query(ctx, `SELECT id,tenant_id,status,issued_on::text,amount_minor,currency,issuer_party,recipient_party,source_refs,evidence_refs,note,revision,created_at,updated_at
		FROM invoice_registrations WHERE`+where+`
		AND ($7='' OR (issued_on,id)<(NULLIF($7,'')::date,NULLIF($8,'')::uuid))
		ORDER BY issued_on DESC,id DESC LIMIT $9`, append(args, filter.BeforeDate, filter.BeforeID, filter.Limit+1)...)
	if err != nil {
		return ListPage{}, err
	}
	defer rows.Close()
	items := make([]Registration, 0, filter.Limit+1)
	for rows.Next() {
		value, scanErr := scanRegistration(rows)
		if scanErr != nil {
			return ListPage{}, scanErr
		}
		items = append(items, value)
	}
	if err = rows.Err(); err != nil {
		return ListPage{}, err
	}
	var effective int64
	var currency string
	err = s.pool.QueryRow(ctx, `SELECT COALESCE(sum(amount_minor),0)::bigint,
		CASE WHEN count(DISTINCT currency)=1 THEN min(currency) ELSE '' END
		FROM invoice_registrations WHERE`+where+` AND status='recorded'`, args...).Scan(&effective, &currency)
	if err != nil {
		return ListPage{}, err
	}
	return ListPage{Items: items, EffectiveAmountMinor: effective, Currency: currency}, nil
}

func (s *PostgresStore) History(ctx context.Context, tenantID, id string, before *int64, limit int) (HistoryPage, error) {
	if _, err := loadRegistration(ctx, s.pool, tenantID, id, ""); err != nil {
		return HistoryPage{}, err
	}
	upper := int64(9223372036854775807)
	if before != nil {
		upper = *before
	}
	rows, err := s.pool.Query(ctx, `SELECT invoice_registration_id,tenant_id,revision,event_kind,command_id,status,issued_on::text,amount_minor,currency,issuer_party,recipient_party,source_refs,evidence_refs,note,reason,actor_ref,created_at,updated_at,occurred_at FROM invoice_registration_revisions WHERE tenant_id=$1 AND invoice_registration_id=$2 AND revision<$3 ORDER BY revision DESC LIMIT $4`, tenantID, id, upper, limit+1)
	if err != nil {
		return HistoryPage{}, err
	}
	defer rows.Close()
	items := make([]Revision, 0, limit+1)
	for rows.Next() {
		value, scanErr := scanRevision(rows)
		if scanErr != nil {
			return HistoryPage{}, scanErr
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

func (s *PostgresStore) Correct(ctx context.Context, actor Actor, id string, input CorrectInput, resolved ResolvedCorrection, hash string) (Registration, error) {
	return s.mutate(ctx, actor, id, input.CommandID, hash, input.ExpectedRevision, input.Reason, func(value *Registration) (string, string, error) {
		before := *value
		if input.Patch.IssuedOn != nil {
			value.IssuedOn = *input.Patch.IssuedOn
		}
		if input.Patch.AmountMinor != nil {
			value.AmountMinor = *input.Patch.AmountMinor
		}
		if input.Patch.Currency != nil {
			value.Currency = *input.Patch.Currency
		}
		if resolved.IssuerParty != nil {
			value.IssuerParty = *resolved.IssuerParty
		}
		if resolved.RecipientParty != nil {
			value.RecipientParty = *resolved.RecipientParty
		}
		if resolved.SourceRefs != nil {
			value.SourceRefs = *resolved.SourceRefs
		}
		if input.Patch.Note != nil {
			value.Note = *input.Patch.Note
		}
		if value.IssuerParty.ID == value.RecipientParty.ID {
			return "", "", ErrReferenceInvalid
		}
		before.Revision, before.UpdatedAt = 0, time.Time{}
		comparison := *value
		comparison.Revision, comparison.UpdatedAt = 0, time.Time{}
		if reflect.DeepEqual(before, comparison) {
			return "", "", ErrNoEffectiveChange
		}
		return EventDetailsCorrected, "invoice_registration_corrected", nil
	})
}

func (s *PostgresStore) Invalidate(ctx context.Context, actor Actor, id string, input InvalidateInput, hash string) (Registration, error) {
	return s.mutate(ctx, actor, id, input.CommandID, hash, input.ExpectedRevision, input.Reason, func(value *Registration) (string, string, error) {
		if value.Status != StatusRecorded {
			return "", "", ErrRegistrationInvalidated
		}
		value.Status = StatusRegistrationInvalidated
		return EventRegistrationInvalidated, "invoice_registration_invalidated", nil
	})
}

func (s *PostgresStore) ReplaceEvidence(ctx context.Context, actor Actor, id string, input EvidenceInput, hash string) (Registration, error) {
	return s.mutate(ctx, actor, id, input.CommandID, hash, input.ExpectedRevision, input.Reason, func(value *Registration) (string, string, error) {
		if reflect.DeepEqual(value.EvidenceRefs, input.EvidenceRefs) {
			return "", "", ErrNoEffectiveChange
		}
		value.EvidenceRefs = append([]EvidenceRef{}, input.EvidenceRefs...)
		return EventEvidenceChanged, "invoice_registration_evidence_changed", nil
	})
}

type changeFunc func(*Registration) (string, string, error)

func (s *PostgresStore) mutate(ctx context.Context, actor Actor, id, commandID, hash string, expected int64, reason string, change changeFunc) (Registration, error) {
	tx, err := s.begin(ctx, actor.TenantID, commandID)
	if err != nil {
		return Registration{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, replayErr := replay(ctx, tx, actor, commandID, hash); replayErr != nil || found {
		return value, replayErr
	}
	value, err := loadRegistration(ctx, tx, actor.TenantID, id, "FOR UPDATE")
	if err != nil {
		return Registration{}, err
	}
	if value.Revision != expected {
		return Registration{}, ErrRevisionConflict
	}
	if value.Status != StatusRecorded {
		return Registration{}, ErrRegistrationInvalidated
	}
	eventKind, action, err := change(&value)
	if err != nil {
		return Registration{}, err
	}
	source := value.Revision
	value.Revision++
	value.UpdatedAt = time.Now().UTC()
	if err = updateCurrent(ctx, tx, value); err != nil {
		return Registration{}, err
	}
	if err = insertRevision(ctx, tx, value, actor, commandID, eventKind, reason); err != nil {
		return Registration{}, err
	}
	if err = insertAudit(ctx, tx, value, actor, commandID, source, action, reason); err != nil {
		return Registration{}, err
	}
	if err = recordCommand(ctx, tx, actor, commandID, hash, value, value.UpdatedAt); err != nil {
		return Registration{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Registration{}, err
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

type rowScanner interface{ Scan(...any) error }
type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func replay(ctx context.Context, q rowQuerier, actor Actor, commandID, hash string) (Registration, bool, error) {
	var storedHash, storedActor string
	var raw []byte
	err := q.QueryRow(ctx, `SELECT request_hash,actor_subject::text,response_json FROM invoice_registration_commands WHERE tenant_id=$1 AND command_id=$2`, actor.TenantID, commandID).Scan(&storedHash, &storedActor, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Registration{}, false, nil
	}
	if err != nil {
		return Registration{}, false, err
	}
	if storedHash != hash || storedActor != actor.ID {
		return Registration{}, false, ErrCommandConflict
	}
	var value Registration
	if err = json.Unmarshal(raw, &value); err != nil {
		return Registration{}, false, err
	}
	return value, true, nil
}

func insertCurrent(ctx context.Context, tx pgx.Tx, value Registration) error {
	issuer, recipient, sources, evidence, err := encodeParts(value)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO invoice_registrations(id,tenant_id,status,issued_on,amount_minor,currency,issuer_party,recipient_party,source_refs,evidence_refs,note,revision,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$13)`, value.ID, value.TenantID, value.Status, value.IssuedOn, value.AmountMinor, value.Currency, issuer, recipient, sources, evidence, value.Note, value.Revision, value.CreatedAt)
	return err
}

func updateCurrent(ctx context.Context, tx pgx.Tx, value Registration) error {
	issuer, recipient, sources, evidence, err := encodeParts(value)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE invoice_registrations SET status=$3,issued_on=$4,amount_minor=$5,currency=$6,issuer_party=$7,recipient_party=$8,source_refs=$9,evidence_refs=$10,note=$11,revision=$12,updated_at=$13 WHERE tenant_id=$1 AND id=$2`, value.TenantID, value.ID, value.Status, value.IssuedOn, value.AmountMinor, value.Currency, issuer, recipient, sources, evidence, value.Note, value.Revision, value.UpdatedAt)
	return err
}

func insertRevision(ctx context.Context, tx pgx.Tx, value Registration, actor Actor, commandID, eventKind, reason string) error {
	issuer, recipient, sources, evidence, err := encodeParts(value)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO invoice_registration_revisions(tenant_id,invoice_registration_id,revision,event_kind,command_id,status,issued_on,amount_minor,currency,issuer_party,recipient_party,source_refs,evidence_refs,note,reason,actor_ref,created_at,updated_at,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$18)`, value.TenantID, value.ID, value.Revision, eventKind, commandID, value.Status, value.IssuedOn, value.AmountMinor, value.Currency, issuer, recipient, sources, evidence, value.Note, reason, actor.Kind+":"+actor.ID, value.CreatedAt, value.UpdatedAt)
	return err
}

func insertAudit(ctx context.Context, tx pgx.Tx, value Registration, actor Actor, commandID string, source int64, action, reason string) error {
	_, err := tx.Exec(ctx, `INSERT INTO invoice_registration_audit_events(id,tenant_id,invoice_registration_id,command_id,source_revision,result_revision,action,reason,actor_subject,actor_kind,credential_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, newUUID(), value.TenantID, value.ID, commandID, source, value.Revision, action, reason, actor.ID, actor.Kind, actor.CredentialID, value.UpdatedAt)
	return err
}

func recordCommand(ctx context.Context, tx pgx.Tx, actor Actor, commandID, hash string, value Registration, now time.Time) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO invoice_registration_commands(tenant_id,command_id,request_hash,actor_subject,response_json,created_at) VALUES($1,$2,$3,$4,$5,$6)`, actor.TenantID, commandID, hash, actor.ID, raw, now)
	return err
}

func encodeParts(value Registration) ([]byte, []byte, []byte, []byte, error) {
	issuer, err := json.Marshal(value.IssuerParty)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	recipient, err := json.Marshal(value.RecipientParty)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	sources, err := json.Marshal(value.SourceRefs)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	evidence, err := json.Marshal(value.EvidenceRefs)
	return issuer, recipient, sources, evidence, err
}

func loadRegistration(ctx context.Context, q rowQuerier, tenantID, id, suffix string) (Registration, error) {
	return scanRegistration(q.QueryRow(ctx, `SELECT id,tenant_id,status,issued_on::text,amount_minor,currency,issuer_party,recipient_party,source_refs,evidence_refs,note,revision,created_at,updated_at FROM invoice_registrations WHERE tenant_id=$1 AND id=$2 `+suffix, tenantID, id))
}

func scanRegistration(row rowScanner) (Registration, error) {
	var value Registration
	var issuer, recipient, sources, evidence []byte
	err := row.Scan(&value.ID, &value.TenantID, &value.Status, &value.IssuedOn, &value.AmountMinor, &value.Currency, &issuer, &recipient, &sources, &evidence, &value.Note, &value.Revision, &value.CreatedAt, &value.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Registration{}, ErrNotFound
	}
	if err != nil {
		return Registration{}, err
	}
	if err = decodeParts(&value, issuer, recipient, sources, evidence); err != nil {
		return Registration{}, err
	}
	return value, nil
}

func scanRevision(row rowScanner) (Revision, error) {
	var value Revision
	var snapshot Registration
	var issuer, recipient, sources, evidence []byte
	err := row.Scan(&value.InvoiceRegistrationID, &value.TenantID, &value.Revision, &value.EventKind, &value.CommandID, &snapshot.Status, &snapshot.IssuedOn, &snapshot.AmountMinor, &snapshot.Currency, &issuer, &recipient, &sources, &evidence, &snapshot.Note, &value.Reason, &value.ActorRef, &snapshot.CreatedAt, &snapshot.UpdatedAt, &value.OccurredAt)
	if err != nil {
		return Revision{}, err
	}
	snapshot.ID, snapshot.TenantID, snapshot.Revision = value.InvoiceRegistrationID, value.TenantID, value.Revision
	if err = decodeParts(&snapshot, issuer, recipient, sources, evidence); err != nil {
		return Revision{}, err
	}
	value.Snapshot = snapshot
	return value, nil
}

func decodeParts(value *Registration, issuer, recipient, sources, evidence []byte) error {
	if err := json.Unmarshal(issuer, &value.IssuerParty); err != nil {
		return err
	}
	if err := json.Unmarshal(recipient, &value.RecipientParty); err != nil {
		return err
	}
	if err := json.Unmarshal(sources, &value.SourceRefs); err != nil {
		return err
	}
	return json.Unmarshal(evidence, &value.EvidenceRefs)
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
