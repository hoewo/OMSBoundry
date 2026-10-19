package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"omsbizfoundry/billing-invoice/internal/auth"
	"omsbizfoundry/billing-invoice/internal/config"
	"omsbizfoundry/billing-invoice/internal/database"
	"omsbizfoundry/billing-invoice/internal/httpserver"
	"omsbizfoundry/billing-invoice/internal/invoice"
	"omsbizfoundry/billing-invoice/internal/references"
	workloadidentity "omsbizfoundry/workload-identity"
)

func main() {
	if err := run(); err != nil {
		slog.Error("billing-invoice stopped", "error", err)
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
		if err = database.Migrate(ctx, pool); err != nil {
			return err
		}
	} else if _, err = database.RequireCurrentSchema(ctx, pool); err != nil {
		return err
	}
	store, err := invoice.NewPostgresStore(pool)
	if err != nil {
		return err
	}
	party := references.Endpoint{BaseURL: cfg.PartyBaseURL, AccessToken: cfg.PartyAccessToken}
	project := references.Endpoint{BaseURL: cfg.ProjectBaseURL, AccessToken: cfg.ProjectAccessToken}
	contract := references.Endpoint{BaseURL: cfg.ContractBaseURL, AccessToken: cfg.ContractAccessToken}
	money := references.Endpoint{BaseURL: cfg.MoneyBaseURL, AccessToken: cfg.MoneyAccessToken}
	evidence := references.Endpoint{BaseURL: cfg.EvidenceBaseURL, AccessToken: cfg.EvidenceAccessToken}
	if cfg.IdentityProvider == config.IdentityIAM {
		credentials, credentialErr := workloadidentity.NewTokenSource(cfg.IAMServiceCredentialURL, cfg.IAMTenantID, cfg.WorkloadID, cfg.WorkloadSubjectID, cfg.WorkloadKeyFile, cfg.ReferenceTimeout, cfg.WorkloadRefreshBefore)
		if credentialErr != nil {
			return credentialErr
		}
		party = dynamicEndpoint(cfg.PartyBaseURL, "party-directory", "party-directory.reference.read", credentials)
		project = dynamicEndpoint(cfg.ProjectBaseURL, "project-workspace", "project-workspace.read", credentials)
		contract = dynamicEndpoint(cfg.ContractBaseURL, "contract-ledger", "contract-ledger.reference.read", credentials)
		money = dynamicEndpoint(cfg.MoneyBaseURL, "money-flow-ledger", "money-flow-ledger.reference.read", credentials)
		evidence = dynamicEndpoint(cfg.EvidenceBaseURL, "evidence-service", "evidence-service.metadata.read", credentials)
	}
	resolver, err := references.NewHTTPResolver(cfg.ReferenceTimeout, party, project, contract, money, evidence)
	if err != nil {
		return err
	}
	service, err := invoice.NewService(store, resolver, cfg.AllowedCurrencies...)
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
		slog.Info("billing-invoice listening", "address", cfg.HTTPAddress, "identity_provider", authenticator.ProviderName())
		result <- server.ListenAndServe()
	}()
	select {
	case err = <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}

func dynamicEndpoint(baseURL, audience, action string, credentials references.CredentialSource) references.Endpoint {
	return references.Endpoint{BaseURL: baseURL, Audience: audience, Action: action, Credentials: credentials}
}
