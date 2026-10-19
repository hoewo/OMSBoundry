package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

const credentialColumns = `
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
	updated_at,
	password_hash
`

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

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('money-flow-ledger:identity-bootstrap'))`); err != nil {
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
	if _, err := tx.Exec(ctx, `
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
	); err != nil {
		return false, fmt.Errorf("insert bootstrap administrator: %w", err)
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
	credential, err := scanCredential(r.pool.QueryRow(ctx, `
		SELECT `+credentialColumns+`
		FROM local_users
		WHERE username = $1
	`, username))
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
		return fmt.Errorf("local user disappeared during login")
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
		return fmt.Errorf("local user disappeared during login")
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

func scanCredential(row pgx.Row) (Credential, error) {
	var credential Credential
	var lockedUntil *time.Time
	err := row.Scan(
		&credential.ID,
		&credential.Username,
		&credential.DisplayName,
		&credential.Role,
		&credential.Status,
		&credential.SessionGeneration,
		&credential.FailedLoginAttempts,
		&lockedUntil,
		&credential.PasswordChangedAt,
		&credential.CreatedAt,
		&credential.UpdatedAt,
		&credential.PasswordHash,
	)
	if lockedUntil != nil {
		credential.LockedUntil = *lockedUntil
	}
	return credential, err
}
