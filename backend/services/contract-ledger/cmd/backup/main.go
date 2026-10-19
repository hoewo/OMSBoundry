package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"omsbizfoundry/contract-ledger/internal/buildinfo"
	"omsbizfoundry/contract-ledger/internal/database"
	"omsbizfoundry/contract-ledger/internal/systembackup"
)

func main() {
	if err := run(); err != nil {
		slog.Error("Contract Ledger backup operation failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	if buildinfo.Requested(os.Args[1:]) {
		return buildinfo.Write(os.Stdout)
	}
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: backup create|restore|version [options]")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	switch os.Args[1] {
	case "create":
		return runCreate(ctx, os.Args[2:])
	case "restore":
		return runRestore(ctx, os.Args[2:])
	default:
		return fmt.Errorf("unknown operation %q; expected create or restore", os.Args[1])
	}
}

func runCreate(ctx context.Context, arguments []string) error {
	flags := flag.NewFlagSet("backup create", flag.ContinueOnError)
	databaseURL := flags.String("database-url", env("DATABASE_URL", ""), "application PostgreSQL URL")
	clientDatabaseURL := flags.String("client-database-url", env("POSTGRES_CLIENT_DATABASE_URL", ""), "pg_dump PostgreSQL URL; defaults to database-url")
	evidenceRoot := flags.String("evidence-dir", env("EVIDENCE_STORAGE_DIR", ".data/evidence"), "Evidence storage root")
	output := flags.String("output", "", "new backup ZIP path")
	dumpBinary := flags.String("pg-dump-bin", env("PG_DUMP_BIN", "pg_dump"), "pg_dump executable")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || strings.TrimSpace(*databaseURL) == "" || strings.TrimSpace(*output) == "" {
		return fmt.Errorf("create requires --database-url and --output")
	}
	if strings.TrimSpace(*clientDatabaseURL) == "" {
		*clientDatabaseURL = *databaseURL
	}
	pool, err := database.Open(ctx, *databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	client, err := systembackup.NewCommandClient(*clientDatabaseURL, *dumpBinary, "pg_restore")
	if err != nil {
		return err
	}
	manifest, err := systembackup.Create(ctx, systembackup.CreateOptions{
		Pool: pool, Client: client, EvidenceRoot: *evidenceRoot, OutputPath: *output,
	})
	if err != nil {
		return err
	}
	fmt.Printf("created %s: schema=%s contracts=%d evidence=%d\n",
		*output,
		manifest.Database.SchemaVersion,
		manifest.Database.Counts.Contracts,
		manifest.Database.Counts.AvailableEvidenceVersion,
	)
	return nil
}

func runRestore(ctx context.Context, arguments []string) error {
	flags := flag.NewFlagSet("backup restore", flag.ContinueOnError)
	databaseURL := flags.String("database-url", env("DATABASE_URL", ""), "empty target PostgreSQL URL")
	clientDatabaseURL := flags.String("client-database-url", env("POSTGRES_CLIENT_DATABASE_URL", ""), "pg_restore PostgreSQL URL; defaults to database-url")
	evidenceRoot := flags.String("evidence-dir", env("EVIDENCE_STORAGE_DIR", ".data/evidence"), "new Evidence storage root")
	input := flags.String("input", "", "backup ZIP path")
	restoreBinary := flags.String("pg-restore-bin", env("PG_RESTORE_BIN", "pg_restore"), "pg_restore executable")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || strings.TrimSpace(*databaseURL) == "" || strings.TrimSpace(*input) == "" {
		return fmt.Errorf("restore requires --database-url and --input")
	}
	if strings.TrimSpace(*clientDatabaseURL) == "" {
		*clientDatabaseURL = *databaseURL
	}
	pool, err := database.Open(ctx, *databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	client, err := systembackup.NewCommandClient(*clientDatabaseURL, "pg_dump", *restoreBinary)
	if err != nil {
		return err
	}
	manifest, err := systembackup.Restore(ctx, systembackup.RestoreOptions{
		Pool: pool, Client: client, EvidenceRoot: *evidenceRoot, InputPath: *input,
	})
	if err != nil {
		return err
	}
	fmt.Printf("restored %s: schema=%s contracts=%d evidence=%d\n",
		*input,
		manifest.Database.SchemaVersion,
		manifest.Database.Counts.Contracts,
		manifest.Database.Counts.AvailableEvidenceVersion,
	)
	return nil
}

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
