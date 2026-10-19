package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/contract-ledger/internal/audit"
)

const localUserColumns = `
	id::text,
	username,
	display_name,
	role,
	status,
	session_generation,
	failed_login_attempts,
	locked_until,
	password_changed_at,
	created_at,
	updated_at
`

const credentialColumns = localUserColumns + `, password_hash`

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) HasUsers(ctx context.Context) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM local_users)`).Scan(&exists); err != nil {
		return false, fmt.Errorf("query local user existence: %w", err)
	}
	return exists, nil
}

func (r *PostgresRepository) Bootstrap(
	ctx context.Context,
	user User,
	passwordHash string,
	entry audit.Entry,
) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin bootstrap administrator: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('contract-ledger:identity-bootstrap'))`); err != nil {
		return false, fmt.Errorf("lock identity bootstrap: %w", err)
	}
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM local_users`).Scan(&count); err != nil {
		return false, fmt.Errorf("count local users: %w", err)
	}
	if count > 0 {
		if err := tx.Commit(ctx); err != nil {
			return false, fmt.Errorf("commit skipped identity bootstrap: %w", err)
		}
		return false, nil
	}
	if err := insertUser(ctx, tx, user, passwordHash); err != nil {
		return false, err
	}
	if err := audit.Insert(ctx, tx, entry); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit identity bootstrap: %w", err)
	}
	return true, nil
}

func (r *PostgresRepository) FindCredential(ctx context.Context, username string) (Credential, bool, error) {
	return scanCredentialResult(r.pool.QueryRow(ctx, `
		SELECT `+credentialColumns+`
		FROM local_users
		WHERE username = $1
	`, username))
}

func (r *PostgresRepository) FindCredentialByID(ctx context.Context, userID string) (Credential, bool, error) {
	return scanCredentialResult(r.pool.QueryRow(ctx, `
		SELECT `+credentialColumns+`
		FROM local_users
		WHERE id = $1
	`, userID))
}

func scanCredentialResult(row pgx.Row) (Credential, bool, error) {
	credential, err := scanCredential(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Credential{}, false, nil
	}
	if err != nil {
		return Credential{}, false, fmt.Errorf("scan local credential: %w", err)
	}
	return credential, true, nil
}

func (r *PostgresRepository) RecordLoginFailure(
	ctx context.Context,
	userID string,
	now time.Time,
	maximumAttempts int,
	lockDuration time.Duration,
) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE local_users
		SET failed_login_attempts = failed_login_attempts + 1,
			locked_until = CASE
				WHEN failed_login_attempts + 1 >= $3 THEN $2::timestamptz + $4::interval
				ELSE locked_until
			END,
			updated_at = $2
		WHERE id = $1
	`, userID, now, maximumAttempts, lockDuration.String())
	if err != nil {
		return fmt.Errorf("update failed login count: %w", err)
	}
	if result.RowsAffected() != 1 {
		return &UserNotFoundError{UserID: userID}
	}
	return nil
}

func (r *PostgresRepository) ResetLoginFailures(ctx context.Context, userID string, now time.Time) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE local_users
		SET failed_login_attempts = 0,
			locked_until = NULL,
			updated_at = GREATEST(updated_at, $2)
		WHERE id = $1
	`, userID, now)
	if err != nil {
		return fmt.Errorf("reset failed login count: %w", err)
	}
	if result.RowsAffected() != 1 {
		return &UserNotFoundError{UserID: userID}
	}
	return nil
}

func (r *PostgresRepository) CreateSession(
	ctx context.Context,
	tokenHash []byte,
	userID string,
	sessionGeneration int64,
	createdAt time.Time,
	expiresAt time.Time,
) error {
	result, err := r.pool.Exec(ctx, `
		INSERT INTO local_sessions (
			token_hash, user_id, session_generation, created_at, expires_at
		)
		SELECT $1, id, session_generation, $4, $5
		FROM local_users
		WHERE id = $2
			AND status = 'active'
			AND session_generation = $3
	`, tokenHash, userID, sessionGeneration, createdAt, expiresAt)
	if err != nil {
		return fmt.Errorf("insert local session: %w", err)
	}
	if result.RowsAffected() != 1 {
		return &AuthenticationError{}
	}
	return nil
}

func (r *PostgresRepository) ResolveSession(
	ctx context.Context,
	tokenHash []byte,
	now time.Time,
) (Principal, bool, error) {
	var principal Principal
	err := r.pool.QueryRow(ctx, `
		SELECT u.id::text, u.username, u.display_name, u.role, u.session_generation
		FROM local_sessions s
		JOIN local_users u ON u.id = s.user_id
		WHERE s.token_hash = $1
			AND s.revoked_at IS NULL
			AND s.expires_at > $2
			AND u.status = 'active'
			AND u.session_generation = s.session_generation
	`, tokenHash, now).Scan(
		&principal.UserID,
		&principal.Username,
		&principal.DisplayName,
		&principal.Role,
		&principal.SessionGeneration,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, false, nil
	}
	if err != nil {
		return Principal{}, false, fmt.Errorf("query local session: %w", err)
	}
	return principal, true, nil
}

func (r *PostgresRepository) RevokeSession(ctx context.Context, tokenHash []byte, now time.Time) error {
	if _, err := r.pool.Exec(ctx, `
		UPDATE local_sessions
		SET revoked_at = $2
		WHERE token_hash = $1 AND revoked_at IS NULL
	`, tokenHash, now); err != nil {
		return fmt.Errorf("mark local session revoked: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+localUserColumns+`
		FROM local_users
		ORDER BY username, id
	`)
	if err != nil {
		return nil, fmt.Errorf("query local users: %w", err)
	}
	defer rows.Close()
	users := make([]User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scan local user: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate local users: %w", err)
	}
	return users, nil
}

func (r *PostgresRepository) CreateUser(
	ctx context.Context,
	user User,
	passwordHash string,
	entry audit.Entry,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin create local user: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := insertUser(ctx, tx, user, passwordHash); err != nil {
		return err
	}
	if err := audit.Insert(ctx, tx, entry); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit create local user: %w", err)
	}
	return nil
}

func insertUser(ctx context.Context, tx pgx.Tx, user User, passwordHash string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO local_users (
			id, username, display_name, password_hash, role, status,
			session_generation, failed_login_attempts, locked_until,
			password_changed_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 0, NULL, $8, $9, $10)
	`,
		user.ID,
		user.Username,
		user.DisplayName,
		passwordHash,
		user.Role,
		user.Status,
		user.SessionGeneration,
		user.PasswordChangedAt,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		var databaseError *pgconn.PgError
		if errors.As(err, &databaseError) && databaseError.Code == "23505" && databaseError.ConstraintName == "local_users_username_unique" {
			return &DuplicateUsernameError{Username: user.Username}
		}
		return fmt.Errorf("insert local user: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ChangeRole(
	ctx context.Context,
	userID string,
	role Role,
	entry audit.Entry,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin change local user role: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lockAdministratorGuard(ctx, tx); err != nil {
		return err
	}
	current, err := lockUser(ctx, tx, userID)
	if err != nil {
		return err
	}
	if current.Role == role {
		return &NoChangeError{}
	}
	if current.Status == StatusActive && current.Role == RoleAdministrator && role != RoleAdministrator {
		if err := requireAnotherActiveAdministrator(ctx, tx); err != nil {
			return err
		}
	}
	result, err := tx.Exec(ctx, `
		UPDATE local_users
		SET role = $2, updated_at = $3
		WHERE id = $1
	`, userID, role, entry.OccurredAt)
	if err != nil {
		return fmt.Errorf("update local user role: %w", err)
	}
	if result.RowsAffected() != 1 {
		return &UserNotFoundError{UserID: userID}
	}
	if err := audit.Insert(ctx, tx, entry); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit local user role: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ChangeStatus(
	ctx context.Context,
	userID string,
	status Status,
	actorID string,
	entry audit.Entry,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin change local user status: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lockAdministratorGuard(ctx, tx); err != nil {
		return err
	}
	current, err := lockUser(ctx, tx, userID)
	if err != nil {
		return err
	}
	if current.Status == status {
		return &NoChangeError{}
	}
	if status == StatusDisabled && userID == actorID {
		return &SelfDisableError{}
	}
	if current.Status == StatusActive && current.Role == RoleAdministrator && status == StatusDisabled {
		if err := requireAnotherActiveAdministrator(ctx, tx); err != nil {
			return err
		}
	}
	result, err := tx.Exec(ctx, `
		UPDATE local_users
		SET status = $2,
			session_generation = CASE WHEN $2 = 'disabled' THEN session_generation + 1 ELSE session_generation END,
			updated_at = $3
		WHERE id = $1
	`, userID, status, entry.OccurredAt)
	if err != nil {
		return fmt.Errorf("update local user status: %w", err)
	}
	if result.RowsAffected() != 1 {
		return &UserNotFoundError{UserID: userID}
	}
	if err := audit.Insert(ctx, tx, entry); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit local user status: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ChangePassword(
	ctx context.Context,
	userID string,
	expectedSessionGeneration int64,
	passwordHash string,
	now time.Time,
	entry audit.Entry,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin change local password: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	result, err := tx.Exec(ctx, `
		UPDATE local_users
		SET password_hash = $3,
			session_generation = session_generation + 1,
			failed_login_attempts = 0,
			locked_until = NULL,
			password_changed_at = $4,
			updated_at = $4
		WHERE id = $1 AND session_generation = $2 AND status = 'active'
	`, userID, expectedSessionGeneration, passwordHash, now)
	if err != nil {
		return fmt.Errorf("update local password: %w", err)
	}
	if result.RowsAffected() != 1 {
		return &AuthenticationError{}
	}
	if err := audit.Insert(ctx, tx, entry); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit local password change: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ResetPassword(
	ctx context.Context,
	userID string,
	passwordHash string,
	now time.Time,
	entry audit.Entry,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin reset local password: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	result, err := tx.Exec(ctx, `
		UPDATE local_users
		SET password_hash = $2,
			session_generation = session_generation + 1,
			failed_login_attempts = 0,
			locked_until = NULL,
			password_changed_at = $3,
			updated_at = $3
		WHERE id = $1
	`, userID, passwordHash, now)
	if err != nil {
		return fmt.Errorf("reset local password: %w", err)
	}
	if result.RowsAffected() != 1 {
		return &UserNotFoundError{UserID: userID}
	}
	if err := audit.Insert(ctx, tx, entry); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit local password reset: %w", err)
	}
	return nil
}

func lockAdministratorGuard(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('contract-ledger:active-administrator-guard'))`); err != nil {
		return fmt.Errorf("lock active administrator guard: %w", err)
	}
	return nil
}

func lockUser(ctx context.Context, tx pgx.Tx, userID string) (User, error) {
	user, err := scanUser(tx.QueryRow(ctx, `
		SELECT `+localUserColumns+`
		FROM local_users
		WHERE id = $1
		FOR UPDATE
	`, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, &UserNotFoundError{UserID: userID}
	}
	if err != nil {
		return User{}, fmt.Errorf("lock local user: %w", err)
	}
	return user, nil
}

func requireAnotherActiveAdministrator(ctx context.Context, tx pgx.Tx) error {
	var count int
	if err := tx.QueryRow(ctx, `
		SELECT count(*)
		FROM local_users
		WHERE role = 'administrator' AND status = 'active'
	`).Scan(&count); err != nil {
		return fmt.Errorf("count active administrators: %w", err)
	}
	if count <= 1 {
		return &LastAdministratorError{}
	}
	return nil
}

func scanUser(row interface{ Scan(...any) error }) (User, error) {
	var value User
	var lockedUntil *time.Time
	if err := row.Scan(
		&value.ID,
		&value.Username,
		&value.DisplayName,
		&value.Role,
		&value.Status,
		&value.SessionGeneration,
		&value.FailedLoginAttempts,
		&lockedUntil,
		&value.PasswordChangedAt,
		&value.CreatedAt,
		&value.UpdatedAt,
	); err != nil {
		return User{}, err
	}
	if lockedUntil != nil {
		value.LockedUntil = *lockedUntil
	}
	return value, nil
}

func scanCredential(row interface{ Scan(...any) error }) (Credential, error) {
	var value Credential
	var lockedUntil *time.Time
	if err := row.Scan(
		&value.ID,
		&value.Username,
		&value.DisplayName,
		&value.Role,
		&value.Status,
		&value.SessionGeneration,
		&value.FailedLoginAttempts,
		&lockedUntil,
		&value.PasswordChangedAt,
		&value.CreatedAt,
		&value.UpdatedAt,
		&value.PasswordHash,
	); err != nil {
		return Credential{}, err
	}
	if lockedUntil != nil {
		value.LockedUntil = *lockedUntil
	}
	return value, nil
}
