package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"omsbizfoundry/business-engagement/internal/auth"
	"omsbizfoundry/business-engagement/internal/config"
	"omsbizfoundry/business-engagement/internal/database"
	"omsbizfoundry/business-engagement/internal/engagement"
	"omsbizfoundry/business-engagement/internal/httpserver"
	"omsbizfoundry/business-engagement/internal/references"
	workloadidentity "omsbizfoundry/workload-identity"
)

func main() {
	if err := run(); err != nil {
		slog.Error("business-engagement stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if cfg.MigrationMode == config.MigrationAuto {
		if err := database.Migrate(ctx, pool); err != nil {
			return err
		}
	} else if _, err := database.RequireCurrentSchema(ctx, pool); err != nil {
		return err
	}
	store, err := engagement.NewPostgresStore(pool)
	if err != nil {
		return err
	}
	projectEndpoint := references.Endpoint{BaseURL: cfg.ProjectBaseURL, AccessToken: cfg.ProjectAccessToken}
	evidenceEndpoint := references.Endpoint{BaseURL: cfg.EvidenceBaseURL, AccessToken: cfg.EvidenceAccessToken}
	if cfg.IdentityProvider == config.IdentityIAM {
		credentials, credentialErr := workloadidentity.NewTokenSource(cfg.IAMServiceCredentialURL, cfg.IAMTenantID, cfg.WorkloadID, cfg.WorkloadSubjectID, cfg.WorkloadKeyFile, cfg.ReferenceTimeout, cfg.WorkloadRefreshBefore)
		if credentialErr != nil {
			return credentialErr
		}
		projectEndpoint = references.Endpoint{BaseURL: cfg.ProjectBaseURL, CredentialSource: credentials, Audience: "project-workspace", Action: "project-workspace.read"}
		evidenceEndpoint = references.Endpoint{BaseURL: cfg.EvidenceBaseURL, CredentialSource: credentials, Audience: "evidence-service", Action: "evidence-service.view"}
	}
	validator, err := references.NewHTTPValidator(cfg.ReferenceTimeout, projectEndpoint, evidenceEndpoint)
	if err != nil {
		return err
	}
	service, err := engagement.NewService(store, validator)
	if err != nil {
		return err
	}
	var authenticator auth.Authenticator
	if cfg.IdentityProvider == config.IdentityTemplate {
		authenticator, err = auth.NewTemplateProvider(cfg.TemplateTenantID, cfg.TemplateSubjectID, cfg.TemplateDisplayName, cfg.TemplateAccessToken)
	} else {
		authenticator, err = auth.NewIAMProvider(cfg.IAMIssuer, cfg.IAMTenantID, cfg.IAMPublicKeysPath)
	}
	if err != nil {
		return err
	}
	handler, err := httpserver.New(service, authenticator)
	if err != nil {
		return err
	}
	server := &http.Server{Addr: cfg.HTTPAddress, Handler: handler, ReadHeaderTimeout: cfg.ReferenceTimeout}
	result := make(chan error, 1)
	go func() {
		slog.Info("business-engagement listening", "address", cfg.HTTPAddress, "identity_provider", authenticator.ProviderName())
		result <- server.ListenAndServe()
	}()
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}
