package database

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const SupportedPostgreSQLMajor = 18

type SchemaState string

const (
	SchemaEmpty        SchemaState = "empty"
	SchemaPending      SchemaState = "pending"
	SchemaCurrent      SchemaState = "current"
	SchemaIncompatible SchemaState = "incompatible"
)

type PostgreSQLStatus struct {
	Version string `json:"version"`
	Major   int    `json:"major"`
}

type SchemaStatus struct {
	State       SchemaState `json:"state"`
	Current     string      `json:"current,omitempty"`
	Latest      string      `json:"latest"`
	Applied     []string    `json:"applied"`
	Pending     []string    `json:"pending"`
	Unknown     []string    `json:"unknown"`
	Reason      string      `json:"reason,omitempty"`
	ServerReady bool        `json:"server_ready"`
}

type schemaQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func CheckPostgreSQL(ctx context.Context, pool *pgxpool.Pool) (PostgreSQLStatus, error) {
	if pool == nil {
		return PostgreSQLStatus{}, fmt.Errorf("database pool is required")
	}
	var version, versionNumber string
	if err := pool.QueryRow(ctx, `
		SELECT current_setting('server_version'), current_setting('server_version_num')
	`).Scan(&version, &versionNumber); err != nil {
		return PostgreSQLStatus{}, fmt.Errorf("read PostgreSQL version: %w", err)
	}
	number, err := strconv.Atoi(versionNumber)
	if err != nil || number < 10000 {
		return PostgreSQLStatus{}, fmt.Errorf("PostgreSQL server_version_num %q is invalid", versionNumber)
	}
	status := PostgreSQLStatus{Version: version, Major: number / 10000}
	if status.Major != SupportedPostgreSQLMajor {
		return status, fmt.Errorf("PostgreSQL major %d is unsupported; expected %d", status.Major, SupportedPostgreSQLMajor)
	}
	return status, nil
}

func InspectSchema(ctx context.Context, pool *pgxpool.Pool) (SchemaStatus, error) {
	if pool == nil {
		return SchemaStatus{}, fmt.Errorf("database pool is required")
	}
	versions, err := migrationVersions()
	if err != nil {
		return SchemaStatus{}, err
	}
	return inspectSchema(ctx, pool, versions)
}

func RequireCurrentSchema(ctx context.Context, pool *pgxpool.Pool) (SchemaStatus, error) {
	status, err := InspectSchema(ctx, pool)
	if err != nil {
		return SchemaStatus{}, err
	}
	if status.State != SchemaCurrent {
		return status, fmt.Errorf("database schema is %s, expected current migration %s%s", status.State, status.Latest, schemaReasonSuffix(status.Reason))
	}
	return status, nil
}

func migrationVersions() ([]string, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
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

func inspectSchema(ctx context.Context, query schemaQuerier, expected []string) (SchemaStatus, error) {
	var migrationsTableExists bool
	if err := query.QueryRow(ctx, `
		SELECT to_regclass(current_schema() || '.schema_migrations') IS NOT NULL
	`).Scan(&migrationsTableExists); err != nil {
		return SchemaStatus{}, fmt.Errorf("inspect schema migration table: %w", err)
	}
	applied := make([]string, 0)
	if migrationsTableExists {
		rows, err := query.Query(ctx, `SELECT version FROM schema_migrations ORDER BY version`)
		if err != nil {
			return SchemaStatus{}, fmt.Errorf("query applied migrations: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var version string
			if err := rows.Scan(&version); err != nil {
				return SchemaStatus{}, fmt.Errorf("scan applied migration: %w", err)
			}
			applied = append(applied, version)
		}
		if err := rows.Err(); err != nil {
			return SchemaStatus{}, fmt.Errorf("iterate applied migrations: %w", err)
		}
	}
	var unmanagedRelations int64
	if err := query.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
		  AND n.nspname !~ '^pg_toast'
		  AND c.relkind IN ('r', 'p', 'v', 'm', 'S', 'f')
		  AND NOT (n.nspname = current_schema() AND c.relname = 'schema_migrations')
	`).Scan(&unmanagedRelations); err != nil {
		return SchemaStatus{}, fmt.Errorf("count unmanaged schema relations: %w", err)
	}
	return classifySchema(expected, applied, migrationsTableExists, unmanagedRelations), nil
}

func classifySchema(expected, applied []string, migrationsTableExists bool, unmanagedRelations int64) SchemaStatus {
	status := SchemaStatus{
		Applied: append([]string{}, applied...),
		Pending: make([]string, 0),
		Unknown: make([]string, 0),
	}
	if len(expected) > 0 {
		status.Latest = expected[len(expected)-1]
	}
	if len(applied) == 0 && unmanagedRelations > 0 {
		status.State = SchemaIncompatible
		status.Reason = fmt.Sprintf("database has %d user relations but no schema_migrations table", unmanagedRelations)
		return status
	}
	expectedSet := make(map[string]struct{}, len(expected))
	for _, version := range expected {
		expectedSet[version] = struct{}{}
	}
	for _, version := range applied {
		if _, known := expectedSet[version]; !known {
			status.Unknown = append(status.Unknown, version)
		}
	}
	if len(status.Unknown) > 0 {
		status.State = SchemaIncompatible
		status.Reason = "database contains migrations unknown to this binary"
		return status
	}
	if len(applied) > len(expected) {
		status.State = SchemaIncompatible
		status.Reason = "database has more applied migrations than this binary"
		return status
	}
	for index, version := range applied {
		if expected[index] != version {
			status.State = SchemaIncompatible
			status.Reason = "applied migrations are not a strict prefix of the embedded migration list"
			return status
		}
	}
	if len(applied) > 0 {
		status.Current = applied[len(applied)-1]
	}
	status.Pending = append(status.Pending, expected[len(applied):]...)
	switch {
	case len(applied) == 0:
		status.State = SchemaEmpty
	case len(status.Pending) > 0:
		status.State = SchemaPending
	default:
		status.State = SchemaCurrent
		status.ServerReady = true
	}
	return status
}

func validateMigratable(status SchemaStatus) error {
	switch status.State {
	case SchemaEmpty, SchemaPending, SchemaCurrent:
		return nil
	default:
		return fmt.Errorf("database schema is incompatible%s", schemaReasonSuffix(status.Reason))
	}
}

func schemaReasonSuffix(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ""
	}
	return ": " + reason
}
