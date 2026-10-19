package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"omsbizfoundry/cost-outsourcing/internal/auth"
	"omsbizfoundry/cost-outsourcing/internal/config"
	"omsbizfoundry/cost-outsourcing/internal/costoutsourcing"
	"omsbizfoundry/cost-outsourcing/internal/database"
	"omsbizfoundry/cost-outsourcing/internal/httpserver"
	"omsbizfoundry/cost-outsourcing/internal/money"
	"omsbizfoundry/cost-outsourcing/internal/referencecost"
	"omsbizfoundry/cost-outsourcing/internal/references"
	workloadidentity "omsbizfoundry/workload-identity"
)

func main() {
	if err := run(); err != nil {
		slog.Error("cost-outsourcing stopped", "error", err)
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
	store, err := costoutsourcing.NewPostgresStore(pool)
	if err != nil {
		return err
	}
	project := references.Endpoint{BaseURL: cfg.ProjectBaseURL, AccessToken: cfg.ProjectAccessToken}
	party := references.Endpoint{BaseURL: cfg.PartyBaseURL, AccessToken: cfg.PartyAccessToken}
	contract := references.Endpoint{BaseURL: cfg.ContractBaseURL, AccessToken: cfg.ContractAccessToken}
	evidence := references.Endpoint{BaseURL: cfg.EvidenceBaseURL, AccessToken: cfg.EvidenceAccessToken}
	moneyEndpoint := money.Endpoint{BaseURL: cfg.MoneyBaseURL, AccessToken: cfg.MoneyAccessToken}
	if cfg.IdentityProvider == config.IdentityIAM {
		credentials, credentialErr := workloadidentity.NewTokenSource(cfg.IAMServiceCredentialURL, cfg.IAMTenantID, cfg.WorkloadID, cfg.WorkloadSubjectID, cfg.WorkloadKeyFile, cfg.ReferenceTimeout, cfg.WorkloadRefreshBefore)
		if credentialErr != nil {
			return credentialErr
		}
		project = dynamicEndpoint(cfg.ProjectBaseURL, "project-workspace", "project-workspace.read", credentials)
		party = dynamicEndpoint(cfg.PartyBaseURL, "party-directory", "party-directory.reference.read", credentials)
		contract = dynamicEndpoint(cfg.ContractBaseURL, "contract-ledger", "contract-ledger.reference.read", credentials)
		evidence = dynamicEndpoint(cfg.EvidenceBaseURL, "evidence-service", "evidence-service.metadata.read", credentials)
		if cfg.MoneyBaseURL != "" {
			moneyEndpoint = money.Endpoint{BaseURL: cfg.MoneyBaseURL, Credentials: credentials}
		}
	}
	resolver, err := references.NewHTTPResolver(cfg.ReferenceTimeout, project, party, contract, evidence)
	if err != nil {
		return err
	}
	var service *costoutsourcing.Service
	if cfg.MoneyBaseURL == "" {
		service, err = costoutsourcing.NewService(store, resolver)
	} else {
		moneyClient, clientErr := money.NewClient(cfg.ReferenceTimeout, moneyEndpoint)
		if clientErr != nil {
			return clientErr
		}
		service, err = costoutsourcing.NewService(store, resolver, moneyClient)
	}
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
	var referenceCosts *referencecost.Service
	if cfg.ReferenceOACostProfile == referencecost.Profile {
		referenceStore, storeErr := referencecost.NewPostgresStore(pool)
		if storeErr != nil {
			return storeErr
		}
		referenceCosts, err = referencecost.New(referenceStore, resolver)
		if err != nil {
			return err
		}
	}
	handler, err := httpserver.New(service, authenticator, referenceCosts)
	if err != nil {
		return err
	}
	server := &http.Server{Addr: cfg.HTTPAddress, Handler: handler, ReadHeaderTimeout: cfg.ReferenceTimeout}
	result := make(chan error, 1)
	go func() {
		slog.Info("cost-outsourcing listening", "address", cfg.HTTPAddress, "identity_provider", authenticator.ProviderName())
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
