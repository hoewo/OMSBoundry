package database

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const SupportedPostgreSQLMajor = 18

//go:embed migrations/*.sql
var migrationFiles embed.FS

func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database configuration: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("open database pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}
func CheckPostgreSQL(ctx context.Context, pool *pgxpool.Pool) error {
	var major int
	if err := pool.QueryRow(ctx, `SELECT current_setting('server_version_num')::integer / 10000`).Scan(&major); err != nil {
		return fmt.Errorf("read PostgreSQL version: %w", err)
	}
	if major != SupportedPostgreSQLMajor {
		return fmt.Errorf("PostgreSQL major %d is unsupported; expected %d", major, SupportedPostgreSQLMajor)
	}
	return nil
}
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if err := CheckPostgreSQL(ctx, pool); err != nil {
		return err
	}
	versions, err := migrationVersions()
	if err != nil {
		return err
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin migrations: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('omsbizfoundry:evidence-service:migrations'))`); err != nil {
		return fmt.Errorf("lock migrations: %w", err)
	}
	if _, err := tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT clock_timestamp())`); err != nil {
		return fmt.Errorf("ensure schema migrations: %w", err)
	}
	for _, version := range versions {
		var applied bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version=$1)`, version).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %s: %w", version, err)
		}
		if applied {
			continue
		}
		script, err := migrationFiles.ReadFile("migrations/" + version)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", version, err)
		}
		if _, err := tx.Exec(ctx, string(script)); err != nil {
			return fmt.Errorf("execute migration %s: %w", version, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, version); err != nil {
			return fmt.Errorf("record migration %s: %w", version, err)
		}
	}
	return tx.Commit(ctx)
}
func RequireCurrentSchema(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	if err := CheckPostgreSQL(ctx, pool); err != nil {
		return "", err
	}
	versions, err := migrationVersions()
	if err != nil {
		return "", err
	}
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass(current_schema() || '.schema_migrations') IS NOT NULL`).Scan(&exists); err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf("database schema is empty")
	}
	rows, err := pool.Query(ctx, `SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	applied := []string{}
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return "", err
		}
		applied = append(applied, version)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(applied) != len(versions) {
		return "", fmt.Errorf("database migration count %d does not match expected %d", len(applied), len(versions))
	}
	for i := range versions {
		if versions[i] != applied[i] {
			return "", fmt.Errorf("database migration %q is incompatible with %q", applied[i], versions[i])
		}
	}
	return versions[len(versions)-1], nil
}
func migrationVersions() ([]string, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, err
	}
	versions := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			versions = append(versions, entry.Name())
		}
	}
	sort.Strings(versions)
	if len(versions) == 0 {
		return nil, fmt.Errorf("no embedded migrations")
	}
	return versions, nil
}
