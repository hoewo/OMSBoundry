package systembackup

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	Format        = "omsbizfoundry.money-flow-ledger/full-backup-v1"
	FormatVersion = 1

	manifestEntry = "manifest.json"
	databaseEntry = "database.dump"
)

var (
	sha256Pattern    = regexp.MustCompile(`^[0-9a-f]{64}$`)
	tableNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
)

type Manifest struct {
	Format      string           `json:"format"`
	Version     int              `json:"version"`
	GeneratedAt string           `json:"generated_at"`
	Database    DatabaseManifest `json:"database"`
}

type DatabaseManifest struct {
	Name              string       `json:"name"`
	PostgreSQLVersion string       `json:"postgresql_version"`
	SchemaVersion     string       `json:"schema_version"`
	DumpPath          string       `json:"dump_path"`
	DumpBytes         int64        `json:"dump_bytes"`
	DumpSHA256        string       `json:"dump_sha256"`
	SessionsExcluded  bool         `json:"sessions_excluded"`
	Tables            []TableCount `json:"tables"`
}

type TableCount struct {
	Name string `json:"name"`
	Rows int64  `json:"rows"`
}

func (manifest Manifest) validate() error {
	if manifest.Format != Format || manifest.Version != FormatVersion {
		return fmt.Errorf("unsupported backup format")
	}
	generatedAt, err := time.Parse(time.RFC3339Nano, manifest.GeneratedAt)
	if err != nil || generatedAt.Location() != time.UTC {
		return fmt.Errorf("generated_at must be an RFC3339 UTC timestamp")
	}
	database := manifest.Database
	if strings.TrimSpace(database.Name) == "" || strings.ContainsAny(database.Name, "\x00\r\n") {
		return fmt.Errorf("database name is invalid")
	}
	if strings.TrimSpace(database.PostgreSQLVersion) == "" || strings.ContainsAny(database.PostgreSQLVersion, "\x00\r\n") {
		return fmt.Errorf("postgresql version is invalid")
	}
	if strings.TrimSpace(database.SchemaVersion) == "" || strings.ContainsAny(database.SchemaVersion, "\x00\r\n") {
		return fmt.Errorf("schema version is invalid")
	}
	if database.DumpPath != databaseEntry || database.DumpBytes < 1 || database.DumpBytes > maxArchiveBytes || !sha256Pattern.MatchString(database.DumpSHA256) {
		return fmt.Errorf("database dump metadata is invalid")
	}
	if !database.SessionsExcluded {
		return fmt.Errorf("backup must exclude local session data")
	}
	if len(database.Tables) == 0 || len(database.Tables) > maxTableCountEntries {
		return fmt.Errorf("database table counts are invalid")
	}
	previous := ""
	hasMigrations := false
	for _, table := range database.Tables {
		if !tableNamePattern.MatchString(table.Name) || table.Name == "local_sessions" || table.Rows < 0 {
			return fmt.Errorf("database table count is invalid")
		}
		if previous != "" && table.Name <= previous {
			return fmt.Errorf("database table counts must be uniquely sorted by name")
		}
		previous = table.Name
		if table.Name == "schema_migrations" {
			hasMigrations = true
		}
	}
	if !hasMigrations {
		return fmt.Errorf("database table counts do not include schema_migrations")
	}
	return nil
}

func marshalManifest(manifest Manifest) ([]byte, error) {
	if err := manifest.validate(); err != nil {
		return nil, err
	}
	value, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode backup manifest: %w", err)
	}
	return append(value, '\n'), nil
}

func tableCountsEqual(left, right []TableCount) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
