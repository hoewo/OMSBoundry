package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"omsbizfoundry/identity-access/internal/capability"
	"omsbizfoundry/identity-access/internal/config"
	"omsbizfoundry/identity-access/internal/database"
	"omsbizfoundry/identity-access/internal/policy"
	"omsbizfoundry/identity-access/internal/workload"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 || (os.Args[1] != "check" && os.Args[1] != "migrate" && os.Args[1] != "validate-contracts") {
		return fmt.Errorf("usage: ops check|migrate|validate-contracts")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if os.Args[1] == "validate-contracts" {
		return validateContracts(cfg)
	}
	root, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(root, 60*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := database.CheckPostgreSQL(ctx, pool); err != nil {
		return err
	}
	if os.Args[1] == "migrate" {
		if err := database.Migrate(ctx, pool); err != nil {
			return err
		}
	}
	version, err := database.RequireCurrentSchema(ctx, pool)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]string{"service": "identity-access", "schema_version": version, "status": "current"})
}

func validateContracts(cfg config.Config) error {
	if _, err := capability.Load(cfg.CapabilityManifestPaths); err != nil {
		return err
	}
	if cfg.PolicyEnabled() {
		if err := policy.Validate(cfg.ProjectAssignmentPolicyPaths); err != nil {
			return err
		}
	}
	if cfg.WorkloadIdentityPolicyPath != "" {
		if err := workload.Validate(cfg.WorkloadIdentityPolicyPath); err != nil {
			return err
		}
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]string{"service": "identity-access", "status": "contracts-valid"})
}
