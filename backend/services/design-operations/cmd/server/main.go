package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"omsbizfoundry/design-operations/internal/auth"
	"omsbizfoundry/design-operations/internal/config"
	"omsbizfoundry/design-operations/internal/database"
	"omsbizfoundry/design-operations/internal/design"
	"omsbizfoundry/design-operations/internal/httpserver"
	"omsbizfoundry/design-operations/internal/references"
	workloadidentity "omsbizfoundry/workload-identity"
)

func main() {
	if err := run(); err != nil {
		slog.Error("design-operations stopped", "error", err)
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
		err = database.Migrate(ctx, pool)
	} else {
		_, err = database.RequireCurrentSchema(ctx, pool)
	}
	if err != nil {
		return err
	}
	store, err := design.NewPostgresStore(pool)
	if err != nil {
		return err
	}
	projectEndpoint := references.Endpoint{BaseURL: cfg.ProjectBaseURL, AccessToken: cfg.ProjectAccessToken}
	peopleEndpoint := references.Endpoint{BaseURL: cfg.PeopleBaseURL, AccessToken: cfg.PeopleAccessToken}
	evidenceEndpoint := references.Endpoint{BaseURL: cfg.EvidenceBaseURL, AccessToken: cfg.EvidenceAccessToken}
	if cfg.IdentityProvider == config.IdentityIAM {
		credentials, credentialErr := workloadidentity.NewTokenSource(cfg.IAMServiceCredentialURL, cfg.IAMTenantID, cfg.WorkloadID, cfg.WorkloadSubjectID, cfg.WorkloadKeyFile, cfg.ReferenceTimeout, cfg.WorkloadRefreshBefore)
		if credentialErr != nil {
			return credentialErr
		}
		projectEndpoint = references.Endpoint{BaseURL: cfg.ProjectBaseURL, CredentialSource: credentials, Audience: "project-workspace", Action: "project-workspace.read"}
		peopleEndpoint = references.Endpoint{BaseURL: cfg.PeopleBaseURL, CredentialSource: credentials, Audience: "people-groups", Action: "people-groups.policy-fact.read"}
		evidenceEndpoint = references.Endpoint{BaseURL: cfg.EvidenceBaseURL, CredentialSource: credentials, Audience: "evidence-service", Action: "evidence-service.view"}
		if cfg.ReferenceStageMaterialProfile == design.StageMaterialProfile {
			evidenceEndpoint.Action = "evidence-service.metadata.read"
		}
	}
	validator, err := references.NewHTTPValidator(cfg.ReferenceTimeout, projectEndpoint, peopleEndpoint, evidenceEndpoint)
	if err != nil {
		return err
	}
	service, err := design.NewServiceWithStageMaterials(store, validator, cfg.ReferenceStageMaterialProfile)
	if err != nil {
		return err
	}
	var designAuthenticator auth.Authenticator
	var productionAuthenticator auth.Authenticator
	if cfg.IdentityProvider == config.IdentityTemplate {
		designAuthenticator, err = auth.NewTemplateProvider(cfg.TemplateTenantID, cfg.TemplateSubjectID, cfg.TemplateDisplayName, cfg.TemplateAccessToken)
		if err == nil {
			productionAuthenticator, err = auth.NewProductionTemplateProvider(cfg.TemplateTenantID, cfg.TemplateSubjectID, cfg.TemplateDisplayName, cfg.TemplateAccessToken)
		}
	} else {
		designAuthenticator, err = auth.NewIAMProvider(cfg.IAMIssuer, cfg.IAMTenantID, cfg.IAMPublicKeysPath)
		if err == nil {
			productionAuthenticator, err = auth.NewProductionIAMProvider(cfg.IAMIssuer, cfg.IAMTenantID, cfg.IAMPublicKeysPath)
		}
	}
	if err != nil {
		return err
	}
	handler, err := httpserver.NewWithProduction(service, designAuthenticator, productionAuthenticator)
	if err != nil {
		return err
	}
	server := &http.Server{Addr: cfg.HTTPAddress, Handler: handler, ReadHeaderTimeout: cfg.ReferenceTimeout}
	result := make(chan error, 1)
	go func() {
		slog.Info("design-operations listening", "address", cfg.HTTPAddress, "identity_provider", designAuthenticator.ProviderName())
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
