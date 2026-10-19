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
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if err := checkPostgreSQL(ctx, pool); err != nil {
		return err
	}
	versions, err := migrationVersions()
	if err != nil {
		return err
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('omsbizfoundry:approval-audit-results:migrations'))`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(version text PRIMARY KEY,applied_at timestamptz NOT NULL DEFAULT clock_timestamp())`); err != nil {
		return err
	}
	for _, version := range versions {
		var applied bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, version).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		script, readErr := migrationFiles.ReadFile("migrations/" + version)
		if readErr != nil {
			return readErr
		}
		if _, err = tx.Exec(ctx, string(script)); err != nil {
			return fmt.Errorf("execute migration %s: %w", version, err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, version); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func RequireCurrentSchema(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	versions, err := migrationVersions()
	if err != nil {
		return "", err
	}
	var exists bool
	if err = pool.QueryRow(ctx, `SELECT to_regclass(current_schema() || '.schema_migrations') IS NOT NULL`).Scan(&exists); err != nil {
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
	var applied []string
	for rows.Next() {
		var version string
		if err = rows.Scan(&version); err != nil {
			return "", err
		}
		applied = append(applied, version)
	}
	if err = rows.Err(); err != nil {
		return "", err
	}
	if len(applied) != len(versions) {
		return "", fmt.Errorf("database migration count %d does not match expected %d", len(applied), len(versions))
	}
	for index := range versions {
		if applied[index] != versions[index] {
			return "", fmt.Errorf("database migration %q is incompatible with %q", applied[index], versions[index])
		}
	}
	return versions[len(versions)-1], nil
}

func checkPostgreSQL(ctx context.Context, pool *pgxpool.Pool) error {
	var major int
	if err := pool.QueryRow(ctx, `SELECT current_setting('server_version_num')::integer / 10000`).Scan(&major); err != nil {
		return err
	}
	if major != SupportedPostgreSQLMajor {
		return fmt.Errorf("PostgreSQL major %d is unsupported; expected %d", major, SupportedPostgreSQLMajor)
	}
	return nil
}

func migrationVersions() ([]string, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, err
	}
	versions := make([]string, 0, len(entries))
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
