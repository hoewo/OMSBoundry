package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const subjectColumns = `id::text, tenant_id::text, username, display_name, status, administrator, system_administrator, revision, session_generation, failed_login_attempts, locked_until, password_changed_at, created_at, updated_at`

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) HasSubjects(ctx context.Context) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM subjects)`).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (r *PostgresRepository) Bootstrap(ctx context.Context, tenantID, tenantName string, subject Subject, passwordHash string, entry AuditEntry) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('omsbizfoundry:identity-access:bootstrap'))`); err != nil {
		return false, err
	}
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM subjects`).Scan(&count); err != nil {
		return false, err
	}
	if count > 0 {
		return false, tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO tenants(id,name,created_at) VALUES($1,$2,$3)`, tenantID, tenantName, subject.CreatedAt); err != nil {
		return false, fmt.Errorf("insert tenant: %w", err)
	}
	if err := insertSubject(ctx, tx, subject, &passwordHash); err != nil {
		return false, err
	}
	if err := insertAudit(ctx, tx, entry); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (r *PostgresRepository) ProvisionTenant(ctx context.Context, tenantID, tenantName string, subject Subject, passwordHash string, entry AuditEntry) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('omsbizfoundry:identity-access:tenant-provisioning'))`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO tenants(id,name,created_at) VALUES($1,$2,$3)`, tenantID, tenantName, subject.CreatedAt); err != nil {
		if isUnique(err) {
			return &ConflictError{Message: "租户已存在"}
		}
		return fmt.Errorf("insert tenant: %w", err)
	}
	if err := insertSubject(ctx, tx, subject, &passwordHash); err != nil {
		return err
	}
	if err := insertAudit(ctx, tx, entry); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) ProvisionTenantInvitation(ctx context.Context, tenantID, tenantName string, subject Subject, token ActivationToken, entry AuditEntry) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('omsbizfoundry:identity-access:tenant-provisioning'))`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO tenants(id,name,created_at) VALUES($1,$2,$3)`, tenantID, tenantName, subject.CreatedAt); err != nil {
		if isUnique(err) {
			return &ConflictError{Message: "组织已存在"}
		}
		return fmt.Errorf("insert tenant: %w", err)
	}
	if err := insertSubject(ctx, tx, subject, nil); err != nil {
		return err
	}
	if err := insertToken(ctx, tx, token); err != nil {
		return err
	}
	if err := insertAudit(ctx, tx, entry); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) TenantExists(ctx context.Context, tenantID string) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM tenants WHERE id=$1)`, tenantID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (r *PostgresRepository) FindSubject(ctx context.Context, tenantID, subjectID string) (Subject, bool, error) {
	value, err := scanSubject(r.pool.QueryRow(ctx, `SELECT `+subjectColumns+` FROM subjects WHERE tenant_id=$1 AND id=$2`, tenantID, subjectID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Subject{}, false, nil
	}
	if err != nil {
		return Subject{}, false, err
	}
	return value, true, nil
}

func (r *PostgresRepository) FindCredential(ctx context.Context, username string) (Credential, bool, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+subjectColumns+`, password_hash FROM subjects WHERE username=$1`, username)
	var value Credential
	var lockedUntil, passwordChangedAt *time.Time
	err := row.Scan(&value.ID, &value.TenantID, &value.Username, &value.DisplayName, &value.Status, &value.Administrator, &value.SystemAdministrator, &value.Revision, &value.SessionGeneration, &value.FailedLoginAttempts, &lockedUntil, &passwordChangedAt, &value.CreatedAt, &value.UpdatedAt, &value.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Credential{}, false, nil
	}
	if err != nil {
		return Credential{}, false, err
	}
	if lockedUntil != nil {
		value.LockedUntil = *lockedUntil
	}
	if passwordChangedAt != nil {
		value.PasswordChangedAt = *passwordChangedAt
	}
	return value, true, nil
}

func (r *PostgresRepository) RecordLoginFailure(ctx context.Context, subjectID string, now time.Time, maximum int, duration time.Duration) error {
	_, err := r.pool.Exec(ctx, `UPDATE subjects SET failed_login_attempts=failed_login_attempts+1, locked_until=CASE WHEN failed_login_attempts+1 >= $3 THEN $2::timestamptz + $4::interval ELSE locked_until END, updated_at=$2 WHERE id=$1`, subjectID, now, maximum, duration.String())
	return err
}

func (r *PostgresRepository) ResetLoginFailures(ctx context.Context, subjectID string, now time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE subjects SET failed_login_attempts=0, locked_until=NULL, updated_at=GREATEST(updated_at,$2) WHERE id=$1`, subjectID, now)
	return err
}

func (r *PostgresRepository) CreateSession(ctx context.Context, tokenHash []byte, subjectID string, generation int64, authTime, expiresAt time.Time) error {
	result, err := r.pool.Exec(ctx, `INSERT INTO suite_sessions(token_hash,subject_id,session_generation,auth_time,expires_at) SELECT $1,id,session_generation,$4,$5 FROM subjects WHERE id=$2 AND session_generation=$3 AND status='active'`, tokenHash, subjectID, generation, authTime, expiresAt)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return &AuthenticationError{}
	}
	return nil
}

func (r *PostgresRepository) ResolveSession(ctx context.Context, tokenHash []byte, now time.Time) (Principal, bool, error) {
	var value Principal
	err := r.pool.QueryRow(ctx, `SELECT u.id::text,u.tenant_id::text,u.username,u.display_name,u.administrator,u.system_administrator,u.session_generation,s.auth_time FROM suite_sessions s JOIN subjects u ON u.id=s.subject_id WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>$2 AND u.status='active' AND u.session_generation=s.session_generation`, tokenHash, now).Scan(&value.SubjectID, &value.TenantID, &value.Username, &value.DisplayName, &value.Administrator, &value.SystemAdministrator, &value.SessionGeneration, &value.AuthTime)
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, false, nil
	}
	if err != nil {
		return Principal{}, false, err
	}
	return value, true, nil
}

func (r *PostgresRepository) RevokeSession(ctx context.Context, tokenHash []byte, now time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE suite_sessions SET revoked_at=$2 WHERE token_hash=$1 AND revoked_at IS NULL`, tokenHash, now)
	return err
}

func (r *PostgresRepository) Invite(ctx context.Context, subject Subject, token ActivationToken, entry AuditEntry) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := insertSubject(ctx, tx, subject, nil); err != nil {
		return err
	}
	if err := insertToken(ctx, tx, token); err != nil {
		return err
	}
	if err := insertAudit(ctx, tx, entry); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) Activate(ctx context.Context, tokenHash []byte, kind TokenKind, passwordHash string, now time.Time, entry AuditEntry) (Subject, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Subject{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var tokenID, subjectID, tenantID string
	err = tx.QueryRow(ctx, `SELECT t.id::text,t.subject_id::text,s.tenant_id::text FROM activation_tokens t JOIN subjects s ON s.id=t.subject_id WHERE t.token_hash=$1 AND t.kind=$2 AND t.used_at IS NULL AND t.expires_at>$3 FOR UPDATE OF t,s`, tokenHash, kind, now).Scan(&tokenID, &subjectID, &tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Subject{}, &AuthenticationError{}
	}
	if err != nil {
		return Subject{}, err
	}
	var result pgconn.CommandTag
	if kind == TokenInvitation {
		result, err = tx.Exec(ctx, `UPDATE subjects SET password_hash=$2,status='active',revision=revision+1,password_changed_at=$3,updated_at=$3 WHERE id=$1 AND status='pending'`, subjectID, passwordHash, now)
	} else {
		result, err = tx.Exec(ctx, `UPDATE subjects SET password_hash=$2,session_generation=session_generation+1,failed_login_attempts=0,locked_until=NULL,password_changed_at=$3,updated_at=$3 WHERE id=$1 AND status='active'`, subjectID, passwordHash, now)
	}
	if err != nil {
		return Subject{}, err
	}
	if result.RowsAffected() != 1 {
		return Subject{}, &ConflictError{Message: "账号状态不允许使用该凭据"}
	}
	if _, err := tx.Exec(ctx, `UPDATE activation_tokens SET used_at=$2 WHERE id=$1`, tokenID, now); err != nil {
		return Subject{}, err
	}
	entry.ObjectID = subjectID
	entry.TenantID = tenantID
	entry.Actor.ID = tokenID
	if err := insertAudit(ctx, tx, entry); err != nil {
		return Subject{}, err
	}
	subject, err := scanSubject(tx.QueryRow(ctx, `SELECT `+subjectColumns+` FROM subjects WHERE id=$1`, subjectID))
	if err != nil {
		return Subject{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Subject{}, err
	}
	return subject, nil
}

func (r *PostgresRepository) CreateActivationToken(ctx context.Context, tenantID string, token ActivationToken, entry AuditEntry) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var status Status
	if err := tx.QueryRow(ctx, `SELECT status FROM subjects WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, tenantID, token.SubjectID).Scan(&status); errors.Is(err, pgx.ErrNoRows) {
		return &NotFoundError{Message: "用户不存在"}
	} else if err != nil {
		return err
	}
	if token.Kind == TokenInvitation && status != StatusPending {
		return &ConflictError{Message: "只有待激活账号可以重新生成激活链接"}
	}
	if token.Kind == TokenPasswordReset && status != StatusActive {
		return &ConflictError{Message: "只有有效账号可以重置密码"}
	}
	if _, err := tx.Exec(ctx, `UPDATE activation_tokens SET used_at=$2 WHERE subject_id=$1 AND kind=$3 AND used_at IS NULL`, token.SubjectID, token.CreatedAt, token.Kind); err != nil {
		return err
	}
	if err := insertToken(ctx, tx, token); err != nil {
		return err
	}
	if err := insertAudit(ctx, tx, entry); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) Grant(ctx context.Context, tenantID string, grant Grant, entry AuditEntry) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var inserted bool
	if grant.Scope.Kind == "tenant" {
		result, err := tx.Exec(ctx, `INSERT INTO capability_grants(subject_id,audience,action,scope_kind,granted_by,granted_at,revoked_at,revoked_by) SELECT target.id,$3,$4,'tenant',actor.id,$6,NULL,NULL FROM subjects target JOIN subjects actor ON actor.id=$5 AND actor.tenant_id=$1 WHERE target.tenant_id=$1 AND target.id=$2 AND target.status='active' ON CONFLICT (subject_id,audience,action) DO UPDATE SET granted_by=EXCLUDED.granted_by,granted_at=EXCLUDED.granted_at,revoked_at=NULL,revoked_by=NULL WHERE capability_grants.revoked_at IS NOT NULL`, tenantID, grant.SubjectID, grant.Audience, grant.Action, grant.GrantedBy, grant.GrantedAt)
		if err != nil {
			if isUnique(err) {
				return &ConflictError{Message: "Capability 已授予"}
			}
			return err
		}
		inserted = result.RowsAffected() == 1
		if !inserted {
			active, err := activeSubjectExists(ctx, tx, tenantID, grant.SubjectID)
			if err != nil {
				return err
			}
			if active {
				return &ConflictError{Message: "Capability 已授予"}
			}
		}
	} else {
		for _, resource := range grant.Scope.Resources {
			result, err := tx.Exec(ctx, `INSERT INTO capability_resource_grants(subject_id,audience,action,resource_type,resource_id,granted_by,granted_at,revoked_at,revoked_by) SELECT target.id,$3,$4,$5,$6,actor.id,$8,NULL,NULL FROM subjects target JOIN subjects actor ON actor.id=$7 AND actor.tenant_id=$1 WHERE target.tenant_id=$1 AND target.id=$2 AND target.status='active' ON CONFLICT (subject_id,audience,action,resource_type,resource_id) DO UPDATE SET granted_by=EXCLUDED.granted_by,granted_at=EXCLUDED.granted_at,revoked_at=NULL,revoked_by=NULL WHERE capability_resource_grants.revoked_at IS NOT NULL`, tenantID, grant.SubjectID, grant.Audience, grant.Action, resource.Type, resource.ID, grant.GrantedBy, grant.GrantedAt)
			if err != nil {
				if isUnique(err) {
					return &ConflictError{Message: "Resource Capability 已授予"}
				}
				return err
			}
			if result.RowsAffected() != 1 {
				active, err := activeSubjectExists(ctx, tx, tenantID, grant.SubjectID)
				if err != nil {
					return err
				}
				if active {
					return &ConflictError{Message: "Resource Capability 已授予"}
				}
				return &NotFoundError{Message: "有效用户不存在"}
			}
			inserted = true
		}
	}
	if !inserted {
		return &NotFoundError{Message: "有效用户不存在"}
	}
	if err := insertAudit(ctx, tx, entry); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func activeSubjectExists(ctx context.Context, tx pgx.Tx, tenantID, subjectID string) (bool, error) {
	var active bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM subjects WHERE tenant_id=$1 AND id=$2 AND status='active')`, tenantID, subjectID).Scan(&active); err != nil {
		return false, err
	}
	return active, nil
}

func (r *PostgresRepository) RevokeGrant(ctx context.Context, tenantID string, grant Grant, entry AuditEntry) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if grant.Scope.Kind == "tenant" {
		result, err := tx.Exec(ctx, `UPDATE capability_grants g SET revoked_at=$5,revoked_by=$6 FROM subjects s WHERE s.id=g.subject_id AND s.tenant_id=$1 AND g.subject_id=$2 AND g.audience=$3 AND g.action=$4 AND g.revoked_at IS NULL`, tenantID, grant.SubjectID, grant.Audience, grant.Action, entry.OccurredAt, entry.Actor.ID)
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return &NotFoundError{Message: "有效 Capability Grant 不存在"}
		}
	} else {
		for _, resource := range grant.Scope.Resources {
			result, err := tx.Exec(ctx, `UPDATE capability_resource_grants g SET revoked_at=$7,revoked_by=$8 FROM subjects s WHERE s.id=g.subject_id AND s.tenant_id=$1 AND g.subject_id=$2 AND g.audience=$3 AND g.action=$4 AND g.resource_type=$5 AND g.resource_id=$6 AND g.revoked_at IS NULL`, tenantID, grant.SubjectID, grant.Audience, grant.Action, resource.Type, resource.ID, entry.OccurredAt, entry.Actor.ID)
			if err != nil {
				return err
			}
			if result.RowsAffected() != 1 {
				return &NotFoundError{Message: "有效 Resource Capability Grant 不存在"}
			}
		}
	}
	if err := insertAudit(ctx, tx, entry); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) ListGrants(ctx context.Context, tenantID, subjectID, audience string) ([]Grant, error) {
	rows, err := r.pool.Query(ctx, `
SELECT subject_id::text,audience,action,'tenant' AS scope_kind,'' AS resource_type,'' AS resource_id,granted_by::text,granted_at
FROM capability_grants g WHERE g.subject_id=$2 AND g.audience=$3 AND g.revoked_at IS NULL AND EXISTS (SELECT 1 FROM subjects s WHERE s.tenant_id=$1 AND s.id=g.subject_id)
UNION ALL
SELECT subject_id::text,audience,action,'resources' AS scope_kind,resource_type,resource_id,granted_by::text,granted_at
FROM capability_resource_grants g WHERE g.subject_id=$2 AND g.audience=$3 AND g.revoked_at IS NULL AND EXISTS (SELECT 1 FROM subjects s WHERE s.tenant_id=$1 AND s.id=g.subject_id)
ORDER BY action,scope_kind,resource_type,resource_id`, tenantID, subjectID, audience)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]Grant, 0)
	for rows.Next() {
		var value Grant
		var scopeKind, resourceType, resourceID string
		if err := rows.Scan(&value.SubjectID, &value.Audience, &value.Action, &scopeKind, &resourceType, &resourceID, &value.GrantedBy, &value.GrantedAt); err != nil {
			return nil, err
		}
		value.Scope = Scope{Kind: scopeKind, Resources: []ResourceRef{}}
		if scopeKind == "resources" {
			value.Scope.Resources = []ResourceRef{{Type: resourceType, ID: resourceID}}
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (r *PostgresRepository) RecordAudit(ctx context.Context, entry AuditEntry) error {
	details, err := json.Marshal(entry.Details)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `INSERT INTO iam_audit_entries(id,tenant_id,action,object_type,object_id,actor_kind,actor_id,actor_display_name,occurred_at,details) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, entry.ID, entry.TenantID, entry.Action, entry.ObjectType, entry.ObjectID, entry.Actor.Kind, entry.Actor.ID, entry.Actor.DisplayName, entry.OccurredAt, details)
	return err
}

func (r *PostgresRepository) ListSubjects(ctx context.Context, tenantID string) ([]Subject, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+subjectColumns+` FROM subjects WHERE tenant_id=$1 AND NOT system_administrator ORDER BY username,id`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]Subject, 0)
	for rows.Next() {
		value, err := scanSubject(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (r *PostgresRepository) ListTenants(ctx context.Context) ([]TenantSummary, error) {
	rows, err := r.pool.Query(ctx, `SELECT t.id::text,t.name,t.created_at,count(s.id) FILTER (WHERE NOT s.system_administrator)::int,count(s.id) FILTER (WHERE s.administrator AND s.status='active' AND NOT s.system_administrator)::int FROM tenants t LEFT JOIN subjects s ON s.tenant_id=t.id GROUP BY t.id,t.name,t.created_at ORDER BY t.created_at,t.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]TenantSummary, 0)
	for rows.Next() {
		var value TenantSummary
		if err := rows.Scan(&value.ID, &value.Name, &value.CreatedAt, &value.UserCount, &value.AdministratorCount); err != nil {
			return nil, err
		}
		value.Status = "active"
		values = append(values, value)
	}
	return values, rows.Err()
}

func (r *PostgresRepository) ChangeStatus(ctx context.Context, tenantID, subjectID string, status Status, entry AuditEntry) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('omsbizfoundry:identity-access:administrator-guard'))`); err != nil {
		return err
	}
	var current Status
	var administrator bool
	if err := tx.QueryRow(ctx, `SELECT status,administrator FROM subjects WHERE tenant_id=$1 AND id=$2 AND NOT system_administrator FOR UPDATE`, tenantID, subjectID).Scan(&current, &administrator); errors.Is(err, pgx.ErrNoRows) {
		return &NotFoundError{Message: "用户不存在"}
	} else if err != nil {
		return err
	}
	if current == status {
		return &ConflictError{Message: "账号状态没有变化"}
	}
	if administrator && current == StatusActive && status == StatusDisabled {
		var others int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM subjects WHERE tenant_id=$1 AND administrator AND status='active' AND id<>$2`, tenantID, subjectID).Scan(&others); err != nil {
			return err
		}
		if others == 0 {
			return &ConflictError{Message: "必须保留至少一名有效管理员"}
		}
	}
	result, err := tx.Exec(ctx, `UPDATE subjects SET status=$3,revision=revision+1,session_generation=CASE WHEN $3='disabled' THEN session_generation+1 ELSE session_generation END,updated_at=$4 WHERE tenant_id=$1 AND id=$2`, tenantID, subjectID, status, entry.OccurredAt)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return &NotFoundError{Message: "用户不存在"}
	}
	if err := insertAudit(ctx, tx, entry); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) ChangeAdministrator(ctx context.Context, tenantID, subjectID string, administrator bool, entry AuditEntry) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('omsbizfoundry:identity-access:administrator-guard'))`); err != nil {
		return err
	}
	var current bool
	var status Status
	if err := tx.QueryRow(ctx, `SELECT administrator,status FROM subjects WHERE tenant_id=$1 AND id=$2 AND NOT system_administrator FOR UPDATE`, tenantID, subjectID).Scan(&current, &status); errors.Is(err, pgx.ErrNoRows) {
		return &NotFoundError{Message: "用户不存在"}
	} else if err != nil {
		return err
	}
	if current == administrator {
		return &ConflictError{Message: "账号管理员身份没有变化"}
	}
	if current && !administrator && status == StatusActive {
		var others int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM subjects WHERE tenant_id=$1 AND administrator AND status='active' AND id<>$2`, tenantID, subjectID).Scan(&others); err != nil {
			return err
		}
		if others == 0 {
			return &ConflictError{Message: "必须保留至少一名有效管理员"}
		}
	}
	result, err := tx.Exec(ctx, `UPDATE subjects SET administrator=$3,revision=revision+1,session_generation=session_generation+1,updated_at=$4 WHERE tenant_id=$1 AND id=$2`, tenantID, subjectID, administrator, entry.OccurredAt)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return &NotFoundError{Message: "用户不存在"}
	}
	if err := insertAudit(ctx, tx, entry); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type rowScanner interface{ Scan(...any) error }

func scanSubject(row rowScanner) (Subject, error) {
	var value Subject
	var locked, passwordChanged *time.Time
	err := row.Scan(&value.ID, &value.TenantID, &value.Username, &value.DisplayName, &value.Status, &value.Administrator, &value.SystemAdministrator, &value.Revision, &value.SessionGeneration, &value.FailedLoginAttempts, &locked, &passwordChanged, &value.CreatedAt, &value.UpdatedAt)
	if locked != nil {
		value.LockedUntil = *locked
	}
	if passwordChanged != nil {
		value.PasswordChangedAt = *passwordChanged
	}
	return value, err
}

func insertSubject(ctx context.Context, tx pgx.Tx, subject Subject, passwordHash *string) error {
	_, err := tx.Exec(ctx, `INSERT INTO subjects(id,tenant_id,username,display_name,password_hash,status,administrator,system_administrator,revision,session_generation,failed_login_attempts,locked_until,password_changed_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,0,NULL,$11,$12,$13)`, subject.ID, subject.TenantID, subject.Username, subject.DisplayName, passwordHash, subject.Status, subject.Administrator, subject.SystemAdministrator, subject.Revision, subject.SessionGeneration, nullableTime(subject.PasswordChangedAt), subject.CreatedAt, subject.UpdatedAt)
	if err != nil {
		if isUnique(err) {
			return &ConflictError{Message: "手机号、邮箱或用户名已被使用"}
		}
		return err
	}
	return nil
}
func insertToken(ctx context.Context, tx pgx.Tx, token ActivationToken) error {
	_, err := tx.Exec(ctx, `INSERT INTO activation_tokens(id,subject_id,kind,token_hash,expires_at,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, token.ID, token.SubjectID, token.Kind, token.TokenHash, token.ExpiresAt, token.CreatedBy, token.CreatedAt)
	return err
}
func insertAudit(ctx context.Context, tx pgx.Tx, entry AuditEntry) error {
	details, err := json.Marshal(entry.Details)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO iam_audit_entries(id,tenant_id,action,object_type,object_id,actor_kind,actor_id,actor_display_name,occurred_at,details) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, entry.ID, entry.TenantID, entry.Action, entry.ObjectType, entry.ObjectID, entry.Actor.Kind, entry.Actor.ID, entry.Actor.DisplayName, entry.OccurredAt, details)
	return err
}
func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}
func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
