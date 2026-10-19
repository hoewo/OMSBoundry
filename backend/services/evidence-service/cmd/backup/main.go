package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"omsbizfoundry/evidence-service/internal/buildinfo"
	"omsbizfoundry/evidence-service/internal/database"
	"omsbizfoundry/evidence-service/internal/systembackup"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if buildinfo.Requested(os.Args[1:]) {
		return buildinfo.Write(os.Stdout)
	}
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: evidence-backup create|restore [options]")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	switch os.Args[1] {
	case "create":
		return create(ctx, os.Args[2:])
	case "restore":
		return restore(ctx, os.Args[2:])
	default:
		return fmt.Errorf("unknown operation %q", os.Args[1])
	}
}

func create(ctx context.Context, arguments []string) error {
	flags := flag.NewFlagSet("create", flag.ContinueOnError)
	databaseURL := flags.String("database-url", env("DATABASE_URL", ""), "application PostgreSQL URL")
	clientURL := flags.String("client-database-url", env("POSTGRES_CLIENT_DATABASE_URL", ""), "pg_dump PostgreSQL URL")
	provider := flags.String("provider", env("STORAGE_PROVIDER", "local"), "local, aliyun-oss or hybrid; OSS bytes require separate backup")
	root := flags.String("storage-root", env("LOCAL_STORAGE_ROOT", ""), "absolute local storage root")
	output := flags.String("output", "", "new backup ZIP path")
	dumpBinary := flags.String("pg-dump-bin", env("PG_DUMP_BIN", "pg_dump"), "pg_dump executable")
	stopped := flags.Bool("confirm-service-stopped", false, "confirm all writers are stopped")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || *databaseURL == "" || *output == "" || !*stopped {
		return fmt.Errorf("create requires --database-url, --output and --confirm-service-stopped")
	}
	if *clientURL == "" {
		*clientURL = *databaseURL
	}
	pool, err := database.Open(ctx, *databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	client, err := systembackup.NewCommandClient(*clientURL, *dumpBinary, "pg_restore")
	if err != nil {
		return err
	}
	manifest, err := systembackup.Create(ctx, systembackup.CreateOptions{Pool: pool, Client: client, Provider: *provider, StorageRoot: *root, OutputPath: *output})
	if err != nil {
		return err
	}
	fmt.Printf("created %s: schema=%s files=%d versions=%d objects=%d\n", *output, manifest.Database.SchemaVersion, manifest.Database.Counts.Files, manifest.Database.Counts.Versions, len(manifest.Storage.Objects))
	return nil
}

func restore(ctx context.Context, arguments []string) error {
	flags := flag.NewFlagSet("restore", flag.ContinueOnError)
	databaseURL := flags.String("database-url", env("DATABASE_URL", ""), "empty target PostgreSQL URL")
	clientURL := flags.String("client-database-url", env("POSTGRES_CLIENT_DATABASE_URL", ""), "pg_restore PostgreSQL URL")
	provider := flags.String("provider", env("STORAGE_PROVIDER", "local"), "local, aliyun-oss or hybrid; must match archive")
	root := flags.String("storage-root", env("LOCAL_STORAGE_ROOT", ""), "new absolute local storage root")
	input := flags.String("input", "", "backup ZIP path")
	restoreBinary := flags.String("pg-restore-bin", env("PG_RESTORE_BIN", "pg_restore"), "pg_restore executable")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || *databaseURL == "" || *input == "" {
		return fmt.Errorf("restore requires --database-url and --input")
	}
	if *clientURL == "" {
		*clientURL = *databaseURL
	}
	pool, err := database.Open(ctx, *databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	client, err := systembackup.NewCommandClient(*clientURL, "pg_dump", *restoreBinary)
	if err != nil {
		return err
	}
	manifest, err := systembackup.Restore(ctx, systembackup.RestoreOptions{Pool: pool, Client: client, Provider: *provider, StorageRoot: *root, InputPath: *input})
	if err != nil {
		return err
	}
	fmt.Printf("restored %s: schema=%s files=%d versions=%d objects=%d\n", *input, manifest.Database.SchemaVersion, manifest.Database.Counts.Files, manifest.Database.Counts.Versions, len(manifest.Storage.Objects))
	return nil
}

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
