package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"omsbizfoundry/party-directory/internal/buildinfo"
	"omsbizfoundry/party-directory/internal/database"
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
	if len(os.Args) != 2 || (os.Args[1] != "migrate" && os.Args[1] != "check") {
		return fmt.Errorf("usage: party-directory-ops migrate|check|version")
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
		if err := database.Migrate(ctx, pool); err != nil {
			return err
		}
	}
	version, err := database.RequireCurrentSchema(ctx, pool)
	if err != nil {
		return err
	}
	fmt.Printf("party-directory schema=%s\n", version)
	return nil
}
