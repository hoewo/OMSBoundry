package systembackup

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

type PostgreSQLClient interface {
	DatabaseName() string
	Dump(context.Context, string, string, io.Writer) error
	Restore(context.Context, string, string, string) error
}

type CommandClient struct {
	databaseName  string
	environment   []string
	dumpBinary    string
	restoreBinary string
}

func NewCommandClient(databaseURL, dumpBinary, restoreBinary string) (*CommandClient, error) {
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse PostgreSQL client configuration: %w", err)
	}
	if strings.TrimSpace(config.Database) == "" {
		return nil, fmt.Errorf("PostgreSQL client database name is required")
	}
	if strings.TrimSpace(dumpBinary) == "" {
		dumpBinary = "pg_dump"
	}
	if strings.TrimSpace(restoreBinary) == "" {
		restoreBinary = "pg_restore"
	}
	environment := withoutPostgreSQLEnvironment(os.Environ())
	environment = append(environment,
		"PGHOST="+config.Host,
		"PGPORT="+strconv.FormatUint(uint64(config.Port), 10),
		"PGDATABASE="+config.Database,
		"PGUSER="+config.User,
		"PGPASSWORD="+config.Password,
	)
	if sslMode := postgresSSLMode(databaseURL); sslMode != "" {
		environment = append(environment, "PGSSLMODE="+sslMode)
	}
	return &CommandClient{
		databaseName: config.Database, environment: environment,
		dumpBinary: dumpBinary, restoreBinary: restoreBinary,
	}, nil
}

func (client *CommandClient) DatabaseName() string { return client.databaseName }

func (client *CommandClient) Dump(ctx context.Context, snapshot, serverVersion string, output io.Writer) error {
	if strings.TrimSpace(snapshot) == "" {
		return fmt.Errorf("PostgreSQL snapshot is required")
	}
	if err := client.requireMajorVersion(ctx, client.dumpBinary, "pg_dump", serverVersion); err != nil {
		return err
	}
	command := exec.CommandContext(ctx, client.dumpBinary,
		"--format=custom",
		"--no-owner",
		"--no-privileges",
		"--snapshot="+snapshot,
	)
	command.Env = client.environment
	command.Stdout = output
	var stderr cappedBuffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("run pg_dump: %w%s", err, stderr.suffix())
	}
	return nil
}

func (client *CommandClient) Restore(ctx context.Context, dumpPath, backupServerVersion, targetServerVersion string) error {
	backupMajor, err := postgresqlMajor(backupServerVersion)
	if err != nil {
		return fmt.Errorf("read backup PostgreSQL major version: %w", err)
	}
	targetMajor, err := postgresqlMajor(targetServerVersion)
	if err != nil {
		return fmt.Errorf("read target PostgreSQL major version: %w", err)
	}
	if backupMajor != targetMajor {
		return fmt.Errorf("backup PostgreSQL major %d does not match target major %d", backupMajor, targetMajor)
	}
	if err := client.requireMajorVersion(ctx, client.restoreBinary, "pg_restore", targetServerVersion); err != nil {
		return err
	}
	dump, err := os.Open(dumpPath)
	if err != nil {
		return fmt.Errorf("open database dump for pg_restore: %w", err)
	}
	defer dump.Close()
	command := exec.CommandContext(ctx, client.restoreBinary,
		"--exit-on-error",
		"--single-transaction",
		"--no-owner",
		"--no-privileges",
		"--dbname="+client.databaseName,
	)
	command.Env = client.environment
	command.Stdin = dump
	var stderr cappedBuffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("run pg_restore: %w%s", err, stderr.suffix())
	}
	return nil
}

var postgresqlVersionPattern = regexp.MustCompile(`(?:^|[^0-9])([0-9]+)(?:\.[0-9]+)?(?:[^0-9]|$)`)

func (client *CommandClient) requireMajorVersion(ctx context.Context, binary, label, serverVersion string) error {
	serverMajor, err := postgresqlMajor(serverVersion)
	if err != nil {
		return fmt.Errorf("read PostgreSQL server major version: %w", err)
	}
	command := exec.CommandContext(ctx, binary, "--version")
	command.Env = client.environment
	var output cappedBuffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		return fmt.Errorf("run %s --version: %w%s", label, err, output.suffix())
	}
	clientMajor, err := postgresqlMajor(output.value.String())
	if err != nil {
		return fmt.Errorf("read %s major version: %w", label, err)
	}
	if clientMajor != serverMajor {
		return fmt.Errorf("%s major %d does not match PostgreSQL server major %d", label, clientMajor, serverMajor)
	}
	return nil
}

func postgresqlMajor(value string) (int, error) {
	match := postgresqlVersionPattern.FindStringSubmatch(value)
	if len(match) != 2 {
		return 0, fmt.Errorf("version %q is invalid", strings.TrimSpace(value))
	}
	major, err := strconv.Atoi(match[1])
	if err != nil || major < 1 {
		return 0, fmt.Errorf("version %q is invalid", strings.TrimSpace(value))
	}
	return major, nil
}

func withoutPostgreSQLEnvironment(values []string) []string {
	blocked := map[string]struct{}{
		"PGHOST": {}, "PGPORT": {}, "PGDATABASE": {}, "PGUSER": {}, "PGPASSWORD": {}, "PGSSLMODE": {},
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		name, _, found := strings.Cut(value, "=")
		if _, remove := blocked[name]; found && remove {
			continue
		}
		result = append(result, value)
	}
	return result
}

func postgresSSLMode(databaseURL string) string {
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return ""
	}
	return parsed.Query().Get("sslmode")
}

type cappedBuffer struct{ value bytes.Buffer }

func (buffer *cappedBuffer) Write(value []byte) (int, error) {
	const limit = 8 << 10
	original := len(value)
	if buffer.value.Len() < limit {
		remaining := limit - buffer.value.Len()
		if len(value) > remaining {
			value = value[:remaining]
		}
		_, _ = buffer.value.Write(value)
	}
	return original, nil
}

func (buffer *cappedBuffer) suffix() string {
	message := strings.TrimSpace(buffer.value.String())
	if message == "" {
		return ""
	}
	return ": " + message
}
