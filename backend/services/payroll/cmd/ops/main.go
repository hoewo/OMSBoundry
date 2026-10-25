package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"omsbizfoundry/payroll/internal/database"
	domain "omsbizfoundry/payroll/internal/payroll"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) < 2 || (os.Args[1] != "migrate" && os.Args[1] != "check" && os.Args[1] != "import-standard-scheme") || (os.Args[1] == "import-standard-scheme" && len(os.Args) != 3) || (os.Args[1] != "import-standard-scheme" && len(os.Args) != 2) {
		return fmt.Errorf("usage: payroll-ops migrate|check|import-standard-scheme FILE")
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
	if os.Args[1] == "migrate" || os.Args[1] == "import-standard-scheme" {
		if err = database.Migrate(ctx, pool); err != nil {
			return err
		}
	}
	version, err := database.RequireCurrentSchema(ctx, pool)
	if err != nil {
		return err
	}
	if os.Args[1] == "import-standard-scheme" {
		file, openErr := os.Open(os.Args[2])
		if openErr != nil {
			return openErr
		}
		defer file.Close()
		decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
		decoder.DisallowUnknownFields()
		var input domain.CreateStandardSchemeInput
		if decodeErr := decoder.Decode(&input); decodeErr != nil {
			return fmt.Errorf("decode standard scheme: %w", decodeErr)
		}
		var trailing any
		if decodeErr := decoder.Decode(&trailing); decodeErr != io.EOF {
			return fmt.Errorf("standard scheme file must contain one JSON value")
		}
		actor := domain.Actor{TenantID: strings.ToLower(strings.TrimSpace(os.Getenv("PLATFORM_TENANT_ID"))), ID: strings.ToLower(strings.TrimSpace(os.Getenv("PLATFORM_ACTOR_ID"))), DisplayName: strings.TrimSpace(os.Getenv("PLATFORM_ACTOR_NAME")), Kind: "iam_subject", CredentialID: strings.TrimSpace(os.Getenv("PLATFORM_CREDENTIAL_ID"))}
		store, storeErr := domain.NewPostgresStore(pool)
		if storeErr != nil {
			return storeErr
		}
		service, serviceErr := domain.NewService(store)
		if serviceErr != nil {
			return serviceErr
		}
		value, createErr := service.CreateStandardScheme(ctx, actor, input)
		if createErr != nil {
			return createErr
		}
		fmt.Printf("imported standard contribution scheme id=%s revision=%d\n", value.ID, value.Revision)
		return nil
	}
	fmt.Printf("payroll schema=%s\n", version)
	return nil
}
