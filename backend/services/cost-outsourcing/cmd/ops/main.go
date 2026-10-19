package main

import (
	"context"
	"fmt"
	"os"

	"omsbizfoundry/cost-outsourcing/internal/config"
	"omsbizfoundry/cost-outsourcing/internal/database"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: cost-outsourcing-ops migrate|check")
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer pool.Close()

	switch os.Args[1] {
	case "migrate":
		err = database.Migrate(ctx, pool)
	case "check":
		var version string
		version, err = database.RequireCurrentSchema(ctx, pool)
		if err == nil {
			fmt.Println(version)
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: cost-outsourcing-ops migrate|check")
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
