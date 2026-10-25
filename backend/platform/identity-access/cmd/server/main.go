package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"omsbizfoundry/identity-access/internal/capability"
	"omsbizfoundry/identity-access/internal/config"
	"omsbizfoundry/identity-access/internal/database"
	"omsbizfoundry/identity-access/internal/httpserver"
	"omsbizfoundry/identity-access/internal/identity"
	"omsbizfoundry/identity-access/internal/policy"
	"omsbizfoundry/identity-access/internal/signing"
	"omsbizfoundry/identity-access/internal/workload"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("identity access stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	root, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(root, 15*time.Second)
	defer cancel()
	pool, err := database.Open(startup, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if cfg.MigrationMode == config.MigrationModeAuto {
		if err := database.Migrate(startup, pool); err != nil {
			return err
		}
	}
	schemaVersion, err := database.RequireCurrentSchema(startup, pool)
	if err != nil {
		return err
	}
	keys, err := signing.Open(cfg.SigningKeyFile, time.Now().UTC())
	if err != nil {
		return err
	}
	catalog, err := capability.Load(cfg.CapabilityManifestPaths)
	if err != nil {
		return err
	}
	service, err := identity.NewService(identity.NewPostgresRepository(pool), keys, catalog, cfg.TenantID, cfg.TenantName, cfg.PublicIssuer, cfg.SessionTTL, cfg.CredentialTTL)
	if err != nil {
		return err
	}
	if err := service.SetWorkloadCredentialTTL(cfg.WorkloadCredentialTTL); err != nil {
		return err
	}
	var workloads *workload.Authenticator
	if cfg.WorkloadIdentityPolicyPath != "" {
		workloads, err = workload.Load(cfg.WorkloadIdentityPolicyPath, workload.NewPostgresNonceStore(pool))
		if err != nil {
			return err
		}
	}
	if cfg.PolicyEnabled() {
		evaluator, err := policy.NewProjectAssignmentEvaluator(policy.Config{
			ProjectBaseURL: cfg.ProjectPolicyBaseURL, PeopleBaseURL: cfg.PeoplePolicyBaseURL,
			ServiceSubjectID: cfg.PolicyServiceSubjectID,
			Issuer:           cfg.PublicIssuer, PolicyPaths: cfg.ProjectAssignmentPolicyPaths, Timeout: cfg.PolicyTimeout,
		}, keys)
		if err != nil {
			return err
		}
		if err := service.SetPolicyEvaluator(evaluator); err != nil {
			return err
		}
	}
	if _, err := service.EnsureBootstrap(startup, identity.BootstrapInput{Username: cfg.BootstrapAdminUsername, DisplayName: cfg.BootstrapAdminDisplayName, Password: cfg.BootstrapAdminPassword}); err != nil {
		return err
	}
	handler, err := httpserver.New(pool, service, keys, httpserver.Options{SessionCookieSecure: cfg.SessionCookieSecure, Workloads: workloads})
	if err != nil {
		return err
	}
	server := &http.Server{Addr: cfg.HTTPAddress, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	errorsChannel := make(chan error, 1)
	go func() {
		logger.Info("identity access listening", "address", cfg.HTTPAddress, "schema_version", schemaVersion)
		errorsChannel <- server.ListenAndServe()
	}()
	select {
	case <-root.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), cfg.ShutdownPeriod)
		defer cancel()
		return server.Shutdown(shutdown)
	case err := <-errorsChannel:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
