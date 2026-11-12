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

func Open(ctx context.Context, raw string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(raw)
	if err != nil {
		return nil, fmt.Errorf("parse database configuration: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if err := check(ctx, pool); err != nil {
		return err
	}
	versions, err := versions()
	if err != nil {
		return err
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('omsbizfoundry:workforce-attendance:migrations'))`); err != nil {
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
		script, e := migrationFiles.ReadFile("migrations/" + version)
		if e != nil {
			return e
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
	expected, err := versions()
	if err != nil {
		return "", err
	}
	rows, err := pool.Query(ctx, `SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var actual []string
	for rows.Next() {
		var value string
		if err = rows.Scan(&value); err != nil {
			return "", err
		}
		actual = append(actual, value)
	}
	if err = rows.Err(); err != nil {
		return "", err
	}
	if len(actual) != len(expected) {
		return "", fmt.Errorf("database migration count %d does not match expected %d", len(actual), len(expected))
	}
	for i := range expected {
		if actual[i] != expected[i] {
			return "", fmt.Errorf("database migration mismatch")
		}
	}
	return expected[len(expected)-1], nil
}
func check(ctx context.Context, pool *pgxpool.Pool) error {
	var major int
	if err := pool.QueryRow(ctx, `SELECT current_setting('server_version_num')::integer/10000`).Scan(&major); err != nil {
		return err
	}
	if major != SupportedPostgreSQLMajor {
		return fmt.Errorf("PostgreSQL major %d is unsupported; expected %d", major, SupportedPostgreSQLMajor)
	}
	return nil
}
func versions() ([]string, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, err
	}
	var values []string
	for _, entry := range entries {
		if !entry.IsDir() {
			values = append(values, entry.Name())
		}
	}
	sort.Strings(values)
	if len(values) == 0 {
		return nil, fmt.Errorf("no embedded migrations")
	}
	return values, nil
}
