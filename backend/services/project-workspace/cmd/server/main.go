package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"omsbizfoundry/project-workspace/internal/auth"
	"omsbizfoundry/project-workspace/internal/config"
	"omsbizfoundry/project-workspace/internal/database"
	"omsbizfoundry/project-workspace/internal/httpserver"
	"omsbizfoundry/project-workspace/internal/projects"
	"omsbizfoundry/project-workspace/internal/references"
	workloadidentity "omsbizfoundry/workload-identity"
)

func main() {
	if err := run(); err != nil {
		slog.Error("project-workspace stopped", "error", err)
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

	store, err := projects.NewPostgresStore(pool)
	if err != nil {
		return err
	}
	peopleEndpoint := references.Endpoint{BaseURL: cfg.PeopleBaseURL, AccessToken: cfg.PeopleAccessToken}
	partyEndpoint := references.Endpoint{BaseURL: cfg.PartyBaseURL, AccessToken: cfg.PartyAccessToken}
	resourceEndpoints := map[string]references.Endpoint{
		"party-directory":        partyEndpoint,
		"contract-ledger":        {BaseURL: cfg.ContractBaseURL, AccessToken: cfg.ContractAccessToken},
		"money-flow-ledger":      {BaseURL: cfg.MoneyBaseURL, AccessToken: cfg.MoneyAccessToken},
		"evidence-service":       {BaseURL: cfg.EvidenceBaseURL, AccessToken: cfg.EvidenceAccessToken},
		"design-operations":      {BaseURL: cfg.DesignBaseURL, AccessToken: cfg.DesignAccessToken},
		"production-operations":  {BaseURL: cfg.ProductionBaseURL, AccessToken: cfg.ProductionAccessToken},
		"billing-invoice":        {BaseURL: cfg.BillingBaseURL, AccessToken: cfg.BillingAccessToken},
		"business-engagement":    {BaseURL: cfg.BusinessEngagementBaseURL, AccessToken: cfg.BusinessEngagementAccessToken},
		"approval-audit-results": {BaseURL: cfg.ResultsBaseURL, AccessToken: cfg.ResultsAccessToken},
		"cost-outsourcing":       {BaseURL: cfg.CostBaseURL, AccessToken: cfg.CostAccessToken},
	}
	if cfg.IdentityProvider == config.IdentityIAM && cfg.IAMServiceCredentialURL != "" {
		credentials, credentialErr := workloadidentity.NewTokenSource(cfg.IAMServiceCredentialURL, cfg.IAMTenantID, cfg.WorkloadID, cfg.WorkloadSubjectID, cfg.WorkloadKeyFile, cfg.ReferenceTimeout, cfg.WorkloadRefreshBefore)
		if credentialErr != nil {
			return credentialErr
		}
		if cfg.PeopleBaseURL != "" {
			peopleEndpoint = references.Endpoint{BaseURL: cfg.PeopleBaseURL, Audience: "people-groups", Action: "people-groups.view", Credentials: credentials}
		}
		if cfg.PartyBaseURL != "" {
			resourceEndpoints["party-directory"] = references.Endpoint{BaseURL: cfg.PartyBaseURL, Audience: "party-directory", Action: "party-directory.reference.read", Credentials: credentials}
		}
		for service, value := range map[string]struct {
			baseURL, audience, action string
		}{
			"contract-ledger":        {cfg.ContractBaseURL, "contract-ledger", "contract-ledger.reference.read"},
			"money-flow-ledger":      {cfg.MoneyBaseURL, "money-flow-ledger", "money-flow-ledger.reference.read"},
			"evidence-service":       {cfg.EvidenceBaseURL, "evidence-service", "evidence-service.metadata.read"},
			"design-operations":      {cfg.DesignBaseURL, "design-operations", "design-work-item.reference.read"},
			"production-operations":  {cfg.ProductionBaseURL, "production-operations", "production-work-item.reference.read"},
			"billing-invoice":        {cfg.BillingBaseURL, "billing-invoice", "billing-invoice.reference.read"},
			"business-engagement":    {cfg.BusinessEngagementBaseURL, "business-engagement", "business-engagement.material-reference.read"},
			"approval-audit-results": {cfg.ResultsBaseURL, "approval-audit-results", "approval-audit-result.read"},
			"cost-outsourcing":       {cfg.CostBaseURL, "cost-outsourcing", "cost-entry.read"},
		} {
			if value.baseURL != "" {
				resourceEndpoints[service] = references.Endpoint{BaseURL: value.baseURL, Audience: value.audience, Action: value.action, Credentials: credentials}
			}
		}
	}
	validator, err := references.NewHTTPValidator(cfg.ReferenceTimeout,
		peopleEndpoint,
		resourceEndpoints,
	)
	if err != nil {
		return err
	}
	service, err := projects.NewService(store, validator)
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
	var oaProduction httpserver.OAProductionUseCases
	if cfg.ReferenceOAProductionProfile != "" {
		oaProduction, err = projects.NewOAProductionService(store, validator)
		if err != nil {
			return err
		}
	}
	handler, err := httpserver.NewWithOAProduction(service, authenticator, oaProduction)
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddress,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReferenceTimeout,
	}
	result := make(chan error, 1)
	go func() {
		slog.Info("project-workspace listening", "address", cfg.HTTPAddress, "identity_provider", authenticator.ProviderName())
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
