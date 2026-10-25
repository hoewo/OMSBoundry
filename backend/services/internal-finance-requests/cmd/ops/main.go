package main

import (
	"context"
	"fmt"
	"omsbizfoundry/internal-finance-requests/internal/database"
	"omsbizfoundry/internal-finance-requests/internal/finance"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 2 || (os.Args[1] != "migrate" && os.Args[1] != "check" && os.Args[1] != "archive-stale-drafts") {
		return fmt.Errorf("usage: internal-finance-requests-ops migrate|check|archive-stale-drafts")
	}
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if os.Args[1] == "migrate" {
		if err = database.Migrate(ctx, pool); err != nil {
			return err
		}
	}
	version, err := database.RequireCurrentSchema(ctx, pool)
	if err != nil {
		return err
	}
	fmt.Printf("internal-finance-requests schema=%s\n", version)
	if os.Args[1] == "archive-stale-drafts" {
		days, err := positiveIntEnv("DRAFT_RETENTION_DAYS", 90, 3650)
		if err != nil {
			return err
		}
		batch, err := positiveIntEnv("DRAFT_ARCHIVE_BATCH_SIZE", 500, 1000)
		if err != nil {
			return err
		}
		store, err := finance.NewPostgresStore(pool)
		if err != nil {
			return err
		}
		service, err := finance.NewService(store)
		if err != nil {
			return err
		}
		before := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
		total := 0
		for {
			count, archiveErr := service.ArchiveStaleDrafts(ctx, before, batch)
			if archiveErr != nil {
				return archiveErr
			}
			total += count
			if count < batch {
				break
			}
		}
		fmt.Printf("internal-finance-requests archived_stale_drafts=%d stale_before=%s\n", total, before.Format(time.RFC3339))
	}
	return nil
}

func positiveIntEnv(name string, fallback, maximum int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > maximum {
		return 0, fmt.Errorf("%s must be between 1 and %d", name, maximum)
	}
	return value, nil
}
