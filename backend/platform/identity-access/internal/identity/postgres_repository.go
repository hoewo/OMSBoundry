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

const subjectColumns = `id::text, account_id::text, tenant_id::text, username, display_name, status, administrator, system_administrator, revision, session_generation, failed_login_attempts, locked_until, password_changed_at, created_at, updated_at`

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
	if err := insertAccount(ctx, tx, subject, &passwordHash); err != nil {
		return false, err
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
	if err := insertAccount(ctx, tx, subject, &passwordHash); err != nil {
		return err
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
	if err := insertAccount(ctx, tx, subject, nil); err != nil {
		return err
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

func (r *PostgresRepository) FindAccountSubject(ctx context.Context, tenantID, accountID string) (Subject, bool, error) {
	value, err := scanSubject(r.pool.QueryRow(ctx, `SELECT `+subjectColumns+` FROM subjects WHERE tenant_id=$1 AND account_id=$2`, tenantID, accountID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Subject{}, false, nil
	}
	if err != nil {
		return Subject{}, false, err
	}
	return value, true, nil
}

func (r *PostgresRepository) FindCredential(ctx context.Context, username string) (Credential, bool, error) {
	row := r.pool.QueryRow(ctx, `SELECT s.id::text,s.account_id::text,s.tenant_id::text,s.username,s.display_name,s.status,s.administrator,s.system_administrator,s.revision,s.session_generation,a.failed_login_attempts,a.locked_until,a.password_changed_at,s.created_at,s.updated_at,a.password_hash
FROM login_accounts a
JOIN LATERAL (
    SELECT s.* FROM subjects s WHERE s.account_id=a.id
    ORDER BY (s.status='active') DESC, (s.id=a.primary_subject_id) DESC, s.created_at, s.id
    LIMIT 1
) s ON true
WHERE a.login_identifier=$1`, username)
	var value Credential
	var lockedUntil, passwordChangedAt *time.Time
	err := row.Scan(&value.ID, &value.AccountID, &value.TenantID, &value.Username, &value.DisplayName, &value.Status, &value.Administrator, &value.SystemAdministrator, &value.Revision, &value.SessionGeneration, &value.FailedLoginAttempts, &lockedUntil, &passwordChangedAt, &value.CreatedAt, &value.UpdatedAt, &value.PasswordHash)
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
	_, err := r.pool.Exec(ctx, `UPDATE login_accounts a SET failed_login_attempts=a.failed_login_attempts+1, locked_until=CASE WHEN a.failed_login_attempts+1 >= $3 THEN $2::timestamptz + $4::interval ELSE a.locked_until END, updated_at=$2 FROM subjects s WHERE s.id=$1 AND a.id=s.account_id`, subjectID, now, maximum, duration.String())
	return err
}

func (r *PostgresRepository) ResetLoginFailures(ctx context.Context, subjectID string, now time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE login_accounts a SET failed_login_attempts=0,locked_until=NULL,updated_at=GREATEST(a.updated_at,$2) FROM subjects s WHERE s.id=$1 AND a.id=s.account_id`, subjectID, now)
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

func (r *PostgresRepository) RotateSession(ctx context.Context, oldTokenHash, newTokenHash []byte, subjectID string, generation int64, authTime, expiresAt time.Time) (Principal, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Principal{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	result, err := tx.Exec(ctx, `UPDATE suite_sessions SET revoked_at=$2 WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at>$2`, oldTokenHash, authTime)
	if err != nil {
		return Principal{}, err
	}
	if result.RowsAffected() != 1 {
		return Principal{}, &AuthenticationError{}
	}
	result, err = tx.Exec(ctx, `INSERT INTO suite_sessions(token_hash,subject_id,session_generation,auth_time,expires_at) SELECT $1,id,session_generation,$4,$5 FROM subjects WHERE id=$2 AND session_generation=$3 AND status='active'`, newTokenHash, subjectID, generation, authTime, expiresAt)
	if err != nil {
		return Principal{}, err
	}
	if result.RowsAffected() != 1 {
		return Principal{}, &AuthenticationError{}
	}
	var value Principal
	err = tx.QueryRow(ctx, `SELECT id::text,account_id::text,tenant_id::text,username,display_name,administrator,system_administrator,session_generation,$2::timestamptz FROM subjects WHERE id=$1 AND status='active'`, subjectID, authTime).Scan(&value.SubjectID, &value.AccountID, &value.TenantID, &value.Username, &value.DisplayName, &value.Administrator, &value.SystemAdministrator, &value.SessionGeneration, &value.AuthTime)
	if err != nil {
		return Principal{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Principal{}, err
	}
	return value, nil
}

func (r *PostgresRepository) ResolveSession(ctx context.Context, tokenHash []byte, now time.Time) (Principal, bool, error) {
	var value Principal
	err := r.pool.QueryRow(ctx, `SELECT u.id::text,u.account_id::text,u.tenant_id::text,u.username,u.display_name,u.administrator,u.system_administrator,u.session_generation,s.auth_time FROM suite_sessions s JOIN subjects u ON u.id=s.subject_id WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>$2 AND u.status='active' AND u.session_generation=s.session_generation`, tokenHash, now).Scan(&value.SubjectID, &value.AccountID, &value.TenantID, &value.Username, &value.DisplayName, &value.Administrator, &value.SystemAdministrator, &value.SessionGeneration, &value.AuthTime)
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
	if err := insertAccount(ctx, tx, subject, nil); err != nil {
		return err
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

func (r *PostgresRepository) JoinAccount(ctx context.Context, subject Subject, entry AuditEntry) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := insertAccountSubject(ctx, tx, subject); err != nil {
		return err
	}
	if err := insertAudit(ctx, tx, entry); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) ProvisionTenantMembership(ctx context.Context, tenantID, tenantName string, subject Subject, entry AuditEntry) error {
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
	if err := insertAccountSubject(ctx, tx, subject); err != nil {
		return err
	}
	if err := insertAudit(ctx, tx, entry); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) ListAccountMemberships(ctx context.Context, subjectID string) ([]OrganizationMembership, error) {
	rows, err := r.pool.Query(ctx, `SELECT s.tenant_id::text,t.name,s.id::text,s.display_name,s.status,s.administrator,s.created_at
FROM subjects current
JOIN subjects s ON s.account_id=current.account_id
JOIN tenants t ON t.id=s.tenant_id
WHERE current.id=$1 AND NOT s.system_administrator
ORDER BY (s.id=current.id) DESC,t.name,s.created_at,s.id`, subjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]OrganizationMembership, 0)
	for rows.Next() {
		var value OrganizationMembership
		if err := rows.Scan(&value.TenantID, &value.TenantName, &value.SubjectID, &value.DisplayName, &value.Status, &value.Administrator, &value.CreatedAt); err != nil {
			return nil, err
		}
		value.Current = value.SubjectID == subjectID
		values = append(values, value)
	}
	return values, rows.Err()
}

func (r *PostgresRepository) Activate(ctx context.Context, tokenHash []byte, kind TokenKind, passwordHash string, now time.Time, entry AuditEntry) (Subject, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Subject{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var tokenID, subjectID, accountID, tenantID string
	err = tx.QueryRow(ctx, `SELECT t.id::text,t.subject_id::text,s.account_id::text,s.tenant_id::text FROM activation_tokens t JOIN subjects s ON s.id=t.subject_id WHERE t.token_hash=$1 AND t.kind=$2 AND t.used_at IS NULL AND t.expires_at>$3 FOR UPDATE OF t,s`, tokenHash, kind, now).Scan(&tokenID, &subjectID, &accountID, &tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Subject{}, &AuthenticationError{}
	}
	if err != nil {
		return Subject{}, err
	}
	var result pgconn.CommandTag
	if kind == TokenInvitation {
		result, err = tx.Exec(ctx, `UPDATE subjects SET password_hash=$2,status='active',revision=revision+1,password_changed_at=$3,updated_at=$3 WHERE id=$1 AND status='pending'`, subjectID, passwordHash, now)
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE login_accounts SET password_hash=$2,password_changed_at=$3,failed_login_attempts=0,locked_until=NULL,updated_at=$3 WHERE id=$1 AND password_hash IS NULL`, accountID, passwordHash, now)
		}
	} else {
		result, err = tx.Exec(ctx, `UPDATE subjects SET password_hash=$2,session_generation=session_generation+1,failed_login_attempts=0,locked_until=NULL,password_changed_at=$3,updated_at=$3 WHERE account_id=$1 AND status IN ('active','disabled')`, accountID, passwordHash, now)
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE login_accounts SET password_hash=$2,password_changed_at=$3,failed_login_attempts=0,locked_until=NULL,updated_at=$3 WHERE id=$1`, accountID, passwordHash, now)
		}
	}
	if err != nil {
		return Subject{}, err
	}
	if (kind == TokenInvitation && result.RowsAffected() != 1) || (kind == TokenPasswordReset && result.RowsAffected() < 1) {
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

func (r *PostgresRepository) PreviewRegistrationInvitation(ctx context.Context, tokenHash []byte, now time.Time) (RegistrationInvitation, error) {
	var value RegistrationInvitation
	err := r.pool.QueryRow(ctx, `SELECT s.tenant_id::text,t.name,s.administrator,a.expires_at
FROM activation_tokens a
JOIN subjects s ON s.id=a.subject_id
JOIN tenants t ON t.id=s.tenant_id
WHERE a.token_hash=$1 AND a.kind='invitation' AND a.used_at IS NULL AND a.expires_at>$2
  AND s.status='pending' AND s.username LIKE 'invite-%'`, tokenHash, now).Scan(&value.TenantID, &value.TenantName, &value.Administrator, &value.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return RegistrationInvitation{}, &AuthenticationError{}
	}
	if err != nil {
		return RegistrationInvitation{}, err
	}
	return value, nil
}

func (r *PostgresRepository) CompleteRegistrationInvitation(ctx context.Context, tokenHash []byte, username, displayName, passwordHash string, existing *Credential, now time.Time, entry AuditEntry) (RegistrationResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return RegistrationResult{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var tokenID, subjectID, placeholderAccountID, tenantID string
	var administrator bool
	err = tx.QueryRow(ctx, `SELECT a.id::text,s.id::text,s.account_id::text,s.tenant_id::text,s.administrator
FROM activation_tokens a
JOIN subjects s ON s.id=a.subject_id
WHERE a.token_hash=$1 AND a.kind='invitation' AND a.used_at IS NULL AND a.expires_at>$2
  AND s.status='pending' AND s.username LIKE 'invite-%'
FOR UPDATE OF a,s`, tokenHash, now).Scan(&tokenID, &subjectID, &placeholderAccountID, &tenantID, &administrator)
	if errors.Is(err, pgx.ErrNoRows) {
		return RegistrationResult{}, &AuthenticationError{}
	}
	if err != nil {
		return RegistrationResult{}, err
	}
	action := "registered"
	accountID := placeholderAccountID
	passwordChangedAt := now
	if existing != nil {
		accountID = existing.AccountID
		if accountID == "" || accountID == placeholderAccountID || existing.Status != StatusActive || existing.SystemAdministrator {
			return RegistrationResult{}, &ConflictError{Message: "该登录账号不能接受当前邀请"}
		}
		var currentHash string
		if err := tx.QueryRow(ctx, `SELECT password_hash FROM login_accounts WHERE id=$1 FOR UPDATE`, accountID).Scan(&currentHash); errors.Is(err, pgx.ErrNoRows) {
			return RegistrationResult{}, &ConflictError{Message: "登录账号不存在"}
		} else if err != nil {
			return RegistrationResult{}, err
		}
		if currentHash != passwordHash {
			return RegistrationResult{}, &AuthenticationError{}
		}
		var joined bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM subjects WHERE tenant_id=$1 AND account_id=$2 AND id<>$3)`, tenantID, accountID, subjectID).Scan(&joined); err != nil {
			return RegistrationResult{}, err
		}
		if joined {
			return RegistrationResult{}, &ConflictError{Message: "该账号已经加入当前组织"}
		}
		passwordChangedAt = existing.PasswordChangedAt
		action = "joined"
	} else {
		result, err := tx.Exec(ctx, `UPDATE login_accounts SET login_identifier=$2,password_hash=$3,password_changed_at=$4,failed_login_attempts=0,locked_until=NULL,updated_at=$4 WHERE id=$1 AND password_hash IS NULL`, placeholderAccountID, username, passwordHash, now)
		if err != nil {
			if isUnique(err) {
				return RegistrationResult{}, &ConflictError{Message: "该手机号或邮箱已注册，请使用原密码接受邀请"}
			}
			return RegistrationResult{}, err
		}
		if result.RowsAffected() != 1 {
			return RegistrationResult{}, &ConflictError{Message: "邀请状态已经变化，请重新打开链接"}
		}
	}
	result, err := tx.Exec(ctx, `UPDATE subjects SET account_id=$2,username=$3,display_name=$4,password_hash=$5,status='active',revision=revision+1,password_changed_at=$6,updated_at=$7 WHERE id=$1 AND status='pending'`, subjectID, accountID, username, displayName, passwordHash, passwordChangedAt, now)
	if err != nil {
		if isUnique(err) {
			return RegistrationResult{}, &ConflictError{Message: "该账号已经加入当前组织"}
		}
		return RegistrationResult{}, err
	}
	if result.RowsAffected() != 1 {
		return RegistrationResult{}, &ConflictError{Message: "邀请状态已经变化，请重新打开链接"}
	}
	if existing != nil {
		if _, err := tx.Exec(ctx, `DELETE FROM login_accounts WHERE id=$1`, placeholderAccountID); err != nil {
			return RegistrationResult{}, err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE activation_tokens SET used_at=$2 WHERE id=$1`, tokenID, now); err != nil {
		return RegistrationResult{}, err
	}
	entry.ObjectID = subjectID
	entry.TenantID = tenantID
	entry.Actor.ID = tokenID
	entry.Details["administrator"] = administrator
	entry.Details["action"] = action
	if err := insertAudit(ctx, tx, entry); err != nil {
		return RegistrationResult{}, err
	}
	subject, err := scanSubject(tx.QueryRow(ctx, `SELECT `+subjectColumns+` FROM subjects WHERE id=$1`, subjectID))
	if err != nil {
		return RegistrationResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return RegistrationResult{}, err
	}
	return RegistrationResult{Action: action, Subject: subject}, nil
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
		sourceKind := grant.SourceKind
		if sourceKind == "" {
			sourceKind = "manual"
		}
		result, err := tx.Exec(ctx, `INSERT INTO capability_grants(subject_id,audience,action,scope_kind,source_kind,granted_by,granted_at,revoked_at,revoked_by) SELECT target.id,$3,$4,'tenant',$7,actor.id,$6,NULL,NULL FROM subjects target JOIN subjects actor ON actor.id=$5 AND actor.tenant_id=$1 WHERE target.tenant_id=$1 AND target.id=$2 AND target.status='active' ON CONFLICT (subject_id,audience,action) DO UPDATE SET source_kind=EXCLUDED.source_kind,granted_by=EXCLUDED.granted_by,granted_at=EXCLUDED.granted_at,revoked_at=NULL,revoked_by=NULL WHERE capability_grants.revoked_at IS NOT NULL OR (capability_grants.source_kind='administrator_profile' AND EXCLUDED.source_kind='manual')`, tenantID, grant.SubjectID, grant.Audience, grant.Action, grant.GrantedBy, grant.GrantedAt, sourceKind)
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
SELECT subject_id::text,audience,action,'tenant' AS scope_kind,'' AS resource_type,'' AS resource_id,source_kind,granted_by::text,granted_at
FROM capability_grants g WHERE g.subject_id=$2 AND g.audience=$3 AND g.revoked_at IS NULL AND EXISTS (SELECT 1 FROM subjects s WHERE s.tenant_id=$1 AND s.id=g.subject_id)
UNION ALL
SELECT subject_id::text,audience,action,'resources' AS scope_kind,resource_type,resource_id,'manual' AS source_kind,granted_by::text,granted_at
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
		if err := rows.Scan(&value.SubjectID, &value.Audience, &value.Action, &scopeKind, &resourceType, &resourceID, &value.SourceKind, &value.GrantedBy, &value.GrantedAt); err != nil {
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

func (r *PostgresRepository) RevokeAdministratorProfile(ctx context.Context, tenantID, subjectID, sourceKind string, entry AuditEntry) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	result, err := tx.Exec(ctx, `UPDATE capability_grants g SET revoked_at=$4,revoked_by=$2 FROM subjects s WHERE s.id=g.subject_id AND s.tenant_id=$1 AND g.subject_id=$2 AND g.source_kind=$3 AND g.revoked_at IS NULL`, tenantID, subjectID, sourceKind, entry.OccurredAt)
	if err != nil {
		return 0, err
	}
	count := result.RowsAffected()
	if count == 0 {
		return 0, nil
	}
	entry.Details["revoked_grant_count"] = count
	entry.Details["source_kind"] = sourceKind
	if err := insertAudit(ctx, tx, entry); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return count, nil
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

func (r *PostgresRepository) UpdateSubjectDisplayName(ctx context.Context, tenantID, subjectID string, expectedRevision int64, displayName string, entry AuditEntry) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var previous string
	var currentRevision int64
	if err := tx.QueryRow(ctx, `SELECT display_name,revision FROM subjects WHERE tenant_id=$1 AND id=$2 AND NOT system_administrator FOR UPDATE`, tenantID, subjectID).Scan(&previous, &currentRevision); errors.Is(err, pgx.ErrNoRows) {
		return &NotFoundError{Message: "用户不存在"}
	} else if err != nil {
		return err
	}
	if currentRevision != expectedRevision {
		return &ConflictError{Message: "账号资料已被更新，请刷新后重试"}
	}
	if previous == displayName {
		return nil
	}
	result, err := tx.Exec(ctx, `UPDATE subjects SET display_name=$3,revision=revision+1,updated_at=$4 WHERE tenant_id=$1 AND id=$2 AND revision=$5`, tenantID, subjectID, displayName, entry.OccurredAt, expectedRevision)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return &ConflictError{Message: "账号资料已被更新，请刷新后重试"}
	}
	if entry.Details == nil {
		entry.Details = map[string]any{}
	}
	entry.Details["previous_display_name"] = previous
	entry.Details["display_name"] = displayName
	entry.Details["revision"] = expectedRevision + 1
	if err := insertAudit(ctx, tx, entry); err != nil {
		return err
	}
	return tx.Commit(ctx)
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

func (r *PostgresRepository) UpdateTenantName(ctx context.Context, tenantID, name string, entry AuditEntry) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var previous string
	if err := tx.QueryRow(ctx, `SELECT name FROM tenants WHERE id=$1 FOR UPDATE`, tenantID).Scan(&previous); errors.Is(err, pgx.ErrNoRows) {
		return &NotFoundError{Message: "组织不存在"}
	} else if err != nil {
		return err
	}
	if previous == name {
		return nil
	}
	if _, err := tx.Exec(ctx, `UPDATE tenants SET name=$2 WHERE id=$1`, tenantID, name); err != nil {
		return err
	}
	if entry.Details == nil {
		entry.Details = map[string]any{}
	}
	entry.Details["previous_name"] = previous
	entry.Details["name"] = name
	if err := insertAudit(ctx, tx, entry); err != nil {
		return err
	}
	return tx.Commit(ctx)
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

func (r *PostgresRepository) ChangeAdministrator(ctx context.Context, tenantID, subjectID string, administrator bool, profileSource string, entry AuditEntry) error {
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
	if !administrator {
		profileRevocation, err := tx.Exec(ctx, `UPDATE capability_grants g SET revoked_at=$4,revoked_by=$2 FROM subjects s WHERE s.id=g.subject_id AND s.tenant_id=$1 AND g.subject_id=$2 AND g.source_kind=$3 AND g.revoked_at IS NULL`, tenantID, subjectID, profileSource, entry.OccurredAt)
		if err != nil {
			return err
		}
		entry.Details["revoked_profile_grant_count"] = profileRevocation.RowsAffected()
		entry.Details["profile_source_kind"] = profileSource
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
	err := row.Scan(&value.ID, &value.AccountID, &value.TenantID, &value.Username, &value.DisplayName, &value.Status, &value.Administrator, &value.SystemAdministrator, &value.Revision, &value.SessionGeneration, &value.FailedLoginAttempts, &locked, &passwordChanged, &value.CreatedAt, &value.UpdatedAt)
	if locked != nil {
		value.LockedUntil = *locked
	}
	if passwordChanged != nil {
		value.PasswordChangedAt = *passwordChanged
	}
	return value, err
}

func insertAccount(ctx context.Context, tx pgx.Tx, subject Subject, passwordHash *string) error {
	accountID := subject.AccountID
	if accountID == "" {
		accountID = subject.ID
	}
	_, err := tx.Exec(ctx, `INSERT INTO login_accounts(id,login_identifier,password_hash,failed_login_attempts,locked_until,password_changed_at,primary_subject_id,created_at,updated_at) VALUES($1,$2,$3,0,NULL,$4,$5,$6,$7)`, accountID, subject.Username, passwordHash, nullableTime(subject.PasswordChangedAt), subject.ID, subject.CreatedAt, subject.UpdatedAt)
	if err != nil {
		if isUnique(err) {
			return &ConflictError{Message: "手机号、邮箱或用户名已被使用"}
		}
		return err
	}
	return nil
}

func insertSubject(ctx context.Context, tx pgx.Tx, subject Subject, passwordHash *string) error {
	accountID := subject.AccountID
	if accountID == "" {
		accountID = subject.ID
	}
	_, err := tx.Exec(ctx, `INSERT INTO subjects(id,account_id,tenant_id,username,display_name,password_hash,status,administrator,system_administrator,revision,session_generation,failed_login_attempts,locked_until,password_changed_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,0,NULL,$12,$13,$14)`, subject.ID, accountID, subject.TenantID, subject.Username, subject.DisplayName, passwordHash, subject.Status, subject.Administrator, subject.SystemAdministrator, subject.Revision, subject.SessionGeneration, nullableTime(subject.PasswordChangedAt), subject.CreatedAt, subject.UpdatedAt)
	if err != nil {
		if isUnique(err) {
			return &ConflictError{Message: "该账号已经加入当前组织"}
		}
		return err
	}
	return nil
}

func insertAccountSubject(ctx context.Context, tx pgx.Tx, subject Subject) error {
	var username string
	var passwordHash *string
	var passwordChangedAt *time.Time
	if err := tx.QueryRow(ctx, `SELECT login_identifier,password_hash,password_changed_at FROM login_accounts WHERE id=$1 FOR UPDATE`, subject.AccountID).Scan(&username, &passwordHash, &passwordChangedAt); errors.Is(err, pgx.ErrNoRows) {
		return &NotFoundError{Message: "登录账号不存在"}
	} else if err != nil {
		return err
	}
	subject.Username = username
	if passwordChangedAt != nil {
		subject.PasswordChangedAt = *passwordChangedAt
	}
	return insertSubject(ctx, tx, subject, passwordHash)
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
