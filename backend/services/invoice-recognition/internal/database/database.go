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
	config, err := pgxpool.ParseConfig(raw)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
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
	var major int
	if err := pool.QueryRow(ctx, `SELECT current_setting('server_version_num')::integer/10000`).Scan(&major); err != nil || major != SupportedPostgreSQLMajor {
		return fmt.Errorf("PostgreSQL 18 is required")
	}
	versions, err := versionNames()
	if err != nil {
		return err
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('omsbizfoundry:invoice-recognition:migrations')); CREATE TABLE IF NOT EXISTS schema_migrations(version text PRIMARY KEY,applied_at timestamptz NOT NULL DEFAULT clock_timestamp())`); err != nil {
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
		script, _ := migrationFiles.ReadFile("migrations/" + version)
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
	expected, err := versionNames()
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
	if len(actual) != len(expected) {
		return "", fmt.Errorf("database migration count does not match")
	}
	for index := range expected {
		if actual[index] != expected[index] {
			return "", fmt.Errorf("database migration mismatch")
		}
	}
	return expected[len(expected)-1], rows.Err()
}
func versionNames() ([]string, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, err
	}
	values := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			values = append(values, entry.Name())
		}
	}
	sort.Strings(values)
	if len(values) == 0 {
		return nil, fmt.Errorf("no migrations")
	}
	return values, nil
}
