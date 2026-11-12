package attendance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	contract "omsbizfoundry/attendance-summary-contract"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) (*PostgresStore, error) {
	if pool == nil {
		return nil, fmt.Errorf("attendance database is required")
	}
	return &PostgresStore{pool: pool}, nil
}
func (s *PostgresStore) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

func replay[T any](ctx context.Context, tx pgx.Tx, tenant, command, hash, kind string) (T, bool, error) {
	var zero T
	var storedHash, storedKind string
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT request_hash,response_kind,response_json FROM attendance_commands WHERE tenant_id=$1 AND command_id=$2`, tenant, command).Scan(&storedHash, &storedKind, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return zero, false, nil
	}
	if err != nil {
		return zero, false, err
	}
	if storedHash != hash || storedKind != kind {
		return zero, false, ErrIdempotencyConflict
	}
	if err = json.Unmarshal(raw, &zero); err != nil {
		return zero, false, err
	}
	return zero, true, nil
}
func saveCommand(ctx context.Context, tx pgx.Tx, tenant, command, hash, kind string, value any, at time.Time) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO attendance_commands(tenant_id,command_id,request_hash,response_kind,response_json,created_at) VALUES($1,$2,$3,$4,$5,$6)`, tenant, command, hash, kind, raw, at)
	return err
}
func saveAudit(ctx context.Context, tx pgx.Tx, actor Actor, resourceType, resourceID, action string, source, result int64, at time.Time) error {
	id, err := newUUID()
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO attendance_audit_events(id,tenant_id,resource_type,resource_id,action,source_revision,result_revision,actor_subject_id,credential_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, id, actor.TenantID, resourceType, resourceID, action, source, result, actor.SubjectID, actor.CredentialID, at)
	return err
}

func (s *PostgresStore) UpsertPolicy(ctx context.Context, actor Actor, input UpsertPolicyInput, hash string) (PolicyRecord, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return PolicyRecord{}, err
	}
	defer tx.Rollback(context.Background())
	if value, ok, e := replay[PolicyRecord](ctx, tx, actor.TenantID, input.CommandID, hash, "policy"); e != nil || ok {
		return value, e
	}
	var current PolicyRecord
	var datesRaw []byte
	err = tx.QueryRow(ctx, `SELECT tenant_id::text,policy_key,period,revision,basis,scheduled_dates,reason,updated_at FROM attendance_policies WHERE tenant_id=$1 AND policy_key=$2 FOR UPDATE`, actor.TenantID, input.Key).Scan(&current.TenantID, &current.Key, &current.Period, &current.Revision, &current.Basis, &datesRaw, &current.Reason, &current.UpdatedAt)
	source := int64(0)
	if err == nil {
		source = current.Revision
		if current.Revision != input.ExpectedRevision {
			return PolicyRecord{}, ErrConflict
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return PolicyRecord{}, err
	} else if input.ExpectedRevision != 0 {
		return PolicyRecord{}, ErrConflict
	}
	at := time.Now().UTC()
	value := PolicyRecord{TenantID: actor.TenantID, Key: input.Key, Period: input.Period, Revision: source + 1, Basis: input.Basis, ScheduledDates: input.ScheduledDates, Reason: input.Reason, UpdatedAt: at}
	dates, _ := json.Marshal(value.ScheduledDates)
	if source == 0 {
		_, err = tx.Exec(ctx, `INSERT INTO attendance_policies(tenant_id,policy_key,period,basis,scheduled_dates,reason,revision,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, value.TenantID, value.Key, value.Period, value.Basis, dates, value.Reason, value.Revision, at)
	} else {
		_, err = tx.Exec(ctx, `UPDATE attendance_policies SET period=$1,basis=$2,scheduled_dates=$3,reason=$4,revision=$5,updated_at=$6 WHERE tenant_id=$7 AND policy_key=$8 AND revision=$9`, value.Period, value.Basis, dates, value.Reason, value.Revision, at, value.TenantID, value.Key, source)
	}
	if err != nil {
		return PolicyRecord{}, err
	}
	snapshot, _ := json.Marshal(value)
	if _, err = tx.Exec(ctx, `INSERT INTO attendance_policy_revisions(tenant_id,policy_key,revision,command_id,snapshot,actor_subject_id,credential_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, actor.TenantID, value.Key, value.Revision, input.CommandID, snapshot, actor.SubjectID, actor.CredentialID, at); err != nil {
		return PolicyRecord{}, err
	}
	if err = saveCommand(ctx, tx, actor.TenantID, input.CommandID, hash, "policy", value, at); err != nil {
		return PolicyRecord{}, err
	}
	if err = saveAudit(ctx, tx, actor, "attendance_policy", value.Key, "policy_upserted", source, value.Revision, at); err != nil {
		return PolicyRecord{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return PolicyRecord{}, err
	}
	return value, nil
}
func scanPolicy(row pgx.Row) (PolicyRecord, error) {
	var value PolicyRecord
	var raw []byte
	err := row.Scan(&value.TenantID, &value.Key, &value.Period, &value.Revision, &value.Basis, &raw, &value.Reason, &value.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return value, ErrNotFound
		}
		return value, err
	}
	if err = json.Unmarshal(raw, &value.ScheduledDates); err != nil {
		return value, err
	}
	return value, nil
}
func (s *PostgresStore) GetPolicy(ctx context.Context, tenant, key string) (PolicyRecord, error) {
	return scanPolicy(s.pool.QueryRow(ctx, `SELECT tenant_id::text,policy_key,period,revision,basis,scheduled_dates,reason,updated_at FROM attendance_policies WHERE tenant_id=$1 AND policy_key=$2`, tenant, key))
}
func (s *PostgresStore) ListPolicies(ctx context.Context, tenant string) ([]PolicyRecord, error) {
	rows, err := s.pool.Query(ctx, `SELECT tenant_id::text,policy_key,period,revision,basis,scheduled_dates,reason,updated_at FROM attendance_policies WHERE tenant_id=$1 ORDER BY period DESC,policy_key`, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []PolicyRecord{}
	for rows.Next() {
		value, scanErr := scanPolicy(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *PostgresStore) RecordAttendance(ctx context.Context, actor Actor, input RecordAttendanceInput, hash string) (AttendanceRecord, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AttendanceRecord{}, err
	}
	defer tx.Rollback(context.Background())
	if value, ok, e := replay[AttendanceRecord](ctx, tx, actor.TenantID, input.CommandID, hash, "attendance"); e != nil || ok {
		return value, e
	}
	var current AttendanceRecord
	err = tx.QueryRow(ctx, `SELECT id::text,tenant_id::text,person_id::text,work_date::text,status,units,note,revision,updated_at FROM attendance_records WHERE tenant_id=$1 AND person_id=$2 AND work_date=$3::date FOR UPDATE`, actor.TenantID, input.PersonID, input.WorkDate).Scan(&current.ID, &current.TenantID, &current.PersonID, &current.WorkDate, &current.Status, &current.Units, &current.Note, &current.Revision, &current.UpdatedAt)
	source := int64(0)
	id := ""
	if err == nil {
		source = current.Revision
		id = current.ID
		if source != input.ExpectedRevision {
			return AttendanceRecord{}, ErrConflict
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return AttendanceRecord{}, err
	} else if input.ExpectedRevision != 0 {
		return AttendanceRecord{}, ErrConflict
	}
	if id == "" {
		id, err = newUUID()
		if err != nil {
			return AttendanceRecord{}, err
		}
	}
	at := time.Now().UTC()
	value := AttendanceRecord{ID: id, TenantID: actor.TenantID, PersonID: input.PersonID, WorkDate: input.WorkDate, Status: input.Status, Units: input.Units, Note: input.Note, Revision: source + 1, UpdatedAt: at}
	if source == 0 {
		_, err = tx.Exec(ctx, `INSERT INTO attendance_records(id,tenant_id,person_id,work_date,status,units,note,revision,updated_at) VALUES($1,$2,$3,$4::date,$5,$6,$7,$8,$9)`, id, actor.TenantID, input.PersonID, input.WorkDate, input.Status, input.Units, input.Note, value.Revision, at)
	} else {
		_, err = tx.Exec(ctx, `UPDATE attendance_records SET status=$1,units=$2,note=$3,revision=$4,updated_at=$5 WHERE tenant_id=$6 AND id=$7 AND revision=$8`, input.Status, input.Units, input.Note, value.Revision, at, actor.TenantID, id, source)
	}
	if err != nil {
		return AttendanceRecord{}, err
	}
	snapshot, _ := json.Marshal(attendanceView(value))
	if _, err = tx.Exec(ctx, `INSERT INTO attendance_record_revisions(tenant_id,record_id,revision,command_id,snapshot,actor_subject_id,credential_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, actor.TenantID, id, value.Revision, input.CommandID, snapshot, actor.SubjectID, actor.CredentialID, at); err != nil {
		return AttendanceRecord{}, err
	}
	if err = saveCommand(ctx, tx, actor.TenantID, input.CommandID, hash, "attendance", value, at); err != nil {
		return AttendanceRecord{}, err
	}
	if err = saveAudit(ctx, tx, actor, "attendance_record", id, "attendance_recorded", source, value.Revision, at); err != nil {
		return AttendanceRecord{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return AttendanceRecord{}, err
	}
	return value, nil
}
func scanAttendance(row pgx.Row) (AttendanceRecord, error) {
	var v AttendanceRecord
	err := row.Scan(&v.ID, &v.TenantID, &v.PersonID, &v.WorkDate, &v.Status, &v.Units, &v.Note, &v.Revision, &v.UpdatedAt)
	return v, err
}
func (s *PostgresStore) ListAttendance(ctx context.Context, tenant, person, period string) ([]AttendanceRecord, error) {
	query := `SELECT id::text,tenant_id::text,person_id::text,work_date::text,status,units,note,revision,updated_at FROM attendance_records WHERE tenant_id=$1 AND work_date>=($2||'-01')::date AND work_date<(($2||'-01')::date+interval '1 month')`
	args := []any{tenant, period}
	if person != "" {
		query += ` AND person_id=$3`
		args = append(args, person)
	}
	query += ` ORDER BY work_date,person_id`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []AttendanceRecord{}
	for rows.Next() {
		v, e := scanAttendance(rows)
		if e != nil {
			return nil, e
		}
		values = append(values, v)
	}
	return values, rows.Err()
}

func (s *PostgresStore) RecordLeave(ctx context.Context, actor Actor, input RecordLeaveInput, hash string) (Leave, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Leave{}, err
	}
	defer tx.Rollback(context.Background())
	if value, ok, e := replay[Leave](ctx, tx, actor.TenantID, input.CommandID, hash, "leave"); e != nil || ok {
		return value, e
	}
	source := int64(0)
	id := input.LeaveID
	if id != "" {
		var current Leave
		err = tx.QueryRow(ctx, `SELECT id::text,tenant_id::text,person_id::text,from_date::text,through_date::text,leave_type,units_per_day,paid_ratio_bps,reason,revision,updated_at FROM leave_records WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, actor.TenantID, id).Scan(&current.ID, &current.TenantID, &current.PersonID, &current.From, &current.Through, &current.LeaveType, &current.UnitsPerDay, &current.PaidRatioBPS, &current.Reason, &current.Revision, &current.UpdatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return Leave{}, ErrNotFound
		}
		if err != nil {
			return Leave{}, err
		}
		source = current.Revision
		if source != input.ExpectedRevision || current.PersonID != input.PersonID {
			return Leave{}, ErrConflict
		}
	} else {
		id, err = newUUID()
		if err != nil {
			return Leave{}, err
		}
	}
	at := time.Now().UTC()
	value := Leave{ID: id, TenantID: actor.TenantID, PersonID: input.PersonID, From: input.From, Through: input.Through, LeaveType: input.LeaveType, UnitsPerDay: input.UnitsPerDay, PaidRatioBPS: input.PaidRatioBPS, Reason: input.Reason, Revision: source + 1, UpdatedAt: at}
	if source == 0 {
		_, err = tx.Exec(ctx, `INSERT INTO leave_records(id,tenant_id,person_id,from_date,through_date,leave_type,units_per_day,paid_ratio_bps,reason,revision,updated_at) VALUES($1,$2,$3,$4::date,$5::date,$6,$7,$8,$9,$10,$11)`, id, actor.TenantID, input.PersonID, input.From, input.Through, input.LeaveType, input.UnitsPerDay, input.PaidRatioBPS, input.Reason, value.Revision, at)
	} else {
		_, err = tx.Exec(ctx, `UPDATE leave_records SET from_date=$1::date,through_date=$2::date,leave_type=$3,units_per_day=$4,paid_ratio_bps=$5,reason=$6,revision=$7,updated_at=$8 WHERE tenant_id=$9 AND id=$10 AND revision=$11`, input.From, input.Through, input.LeaveType, input.UnitsPerDay, input.PaidRatioBPS, input.Reason, value.Revision, at, actor.TenantID, id, source)
	}
	if err != nil {
		return Leave{}, err
	}
	snapshot, _ := json.Marshal(leaveView(value))
	if _, err = tx.Exec(ctx, `INSERT INTO leave_record_revisions(tenant_id,leave_id,revision,command_id,snapshot,actor_subject_id,credential_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, actor.TenantID, id, value.Revision, input.CommandID, snapshot, actor.SubjectID, actor.CredentialID, at); err != nil {
		return Leave{}, err
	}
	if err = saveCommand(ctx, tx, actor.TenantID, input.CommandID, hash, "leave", value, at); err != nil {
		return Leave{}, err
	}
	if err = saveAudit(ctx, tx, actor, "leave_record", id, "leave_recorded", source, value.Revision, at); err != nil {
		return Leave{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Leave{}, err
	}
	return value, nil
}
func scanLeave(row pgx.Row) (Leave, error) {
	var v Leave
	err := row.Scan(&v.ID, &v.TenantID, &v.PersonID, &v.From, &v.Through, &v.LeaveType, &v.UnitsPerDay, &v.PaidRatioBPS, &v.Reason, &v.Revision, &v.UpdatedAt)
	return v, err
}
func (s *PostgresStore) ListLeaves(ctx context.Context, tenant, person, period string) ([]Leave, error) {
	query := `SELECT id::text,tenant_id::text,person_id::text,from_date::text,through_date::text,leave_type,units_per_day,paid_ratio_bps,reason,revision,updated_at FROM leave_records WHERE tenant_id=$1 AND through_date>=($2||'-01')::date AND from_date<(($2||'-01')::date+interval '1 month')`
	args := []any{tenant, period}
	if person != "" {
		query += ` AND person_id=$3`
		args = append(args, person)
	}
	query += ` ORDER BY from_date,person_id,id`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []Leave{}
	for rows.Next() {
		v, e := scanLeave(rows)
		if e != nil {
			return nil, e
		}
		values = append(values, v)
	}
	return values, rows.Err()
}

func (s *PostgresStore) SaveSummary(ctx context.Context, actor Actor, command string, summary contract.MonthlySummary, hash string) (contract.MonthlySummary, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return summary, err
	}
	defer tx.Rollback(context.Background())
	if value, ok, e := replay[contract.MonthlySummary](ctx, tx, actor.TenantID, command, hash, "summary"); e != nil || ok {
		return value, e
	}
	var id string
	var revision int64
	err = tx.QueryRow(ctx, `SELECT id::text,revision FROM attendance_monthly_summaries WHERE tenant_id=$1 AND person_id=$2 AND period=$3 FOR UPDATE`, actor.TenantID, summary.PersonID, summary.Period).Scan(&id, &revision)
	source := int64(0)
	if err == nil {
		source = revision
		summary.ID = id
		summary.SourceRevision = source + 1
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return summary, err
	}
	if err = contract.Validate(summary); err != nil {
		return summary, err
	}
	raw, _ := json.Marshal(summary)
	at := time.Now().UTC()
	if source == 0 {
		_, err = tx.Exec(ctx, `INSERT INTO attendance_monthly_summaries(id,tenant_id,person_id,period,summary,revision,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$7)`, summary.ID, actor.TenantID, summary.PersonID, summary.Period, raw, summary.SourceRevision, at)
	} else {
		_, err = tx.Exec(ctx, `UPDATE attendance_monthly_summaries SET summary=$1,revision=$2,updated_at=$3 WHERE tenant_id=$4 AND id=$5 AND revision=$6`, raw, summary.SourceRevision, at, actor.TenantID, summary.ID, source)
	}
	if err != nil {
		return summary, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO attendance_monthly_summary_revisions(tenant_id,summary_id,revision,command_id,snapshot,actor_subject_id,credential_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, actor.TenantID, summary.ID, summary.SourceRevision, command, raw, actor.SubjectID, actor.CredentialID, at); err != nil {
		return summary, err
	}
	if err = saveCommand(ctx, tx, actor.TenantID, command, hash, "summary", summary, at); err != nil {
		return summary, err
	}
	if err = saveAudit(ctx, tx, actor, "monthly_summary", summary.ID, "summary_calculated", source, summary.SourceRevision, at); err != nil {
		return summary, err
	}
	if err = tx.Commit(ctx); err != nil {
		return summary, err
	}
	return summary, nil
}
func scanSummary(row pgx.Row) (contract.MonthlySummary, error) {
	var raw []byte
	err := row.Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return contract.MonthlySummary{}, ErrNotFound
	}
	if err != nil {
		return contract.MonthlySummary{}, err
	}
	var value contract.MonthlySummary
	if err = json.Unmarshal(raw, &value); err != nil {
		return value, err
	}
	return value, contract.Validate(value)
}
func (s *PostgresStore) GetSummary(ctx context.Context, tenant, id string) (contract.MonthlySummary, error) {
	return scanSummary(s.pool.QueryRow(ctx, `SELECT summary FROM attendance_monthly_summaries WHERE tenant_id=$1 AND id=$2`, tenant, id))
}
func (s *PostgresStore) ListSummaries(ctx context.Context, tenant, period string) ([]contract.MonthlySummary, error) {
	rows, err := s.pool.Query(ctx, `SELECT summary FROM attendance_monthly_summaries WHERE tenant_id=$1 AND period=$2 ORDER BY summary->>'person_display_name',id`, tenant, period)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []contract.MonthlySummary{}
	for rows.Next() {
		v, e := scanSummary(rows)
		if e != nil {
			return nil, e
		}
		values = append(values, v)
	}
	return values, rows.Err()
}

var _ Store = (*PostgresStore)(nil)
