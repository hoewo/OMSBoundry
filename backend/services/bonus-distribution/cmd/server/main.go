package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"omsbizfoundry/bonus-distribution/internal/auth"
	"omsbizfoundry/bonus-distribution/internal/bonus"
	"omsbizfoundry/bonus-distribution/internal/config"
	"omsbizfoundry/bonus-distribution/internal/database"
	"omsbizfoundry/bonus-distribution/internal/httpserver"
	moneyclient "omsbizfoundry/bonus-distribution/internal/money"
	"omsbizfoundry/bonus-distribution/internal/overlays/referenceoa"
	"omsbizfoundry/bonus-distribution/internal/referencebonus"
	"omsbizfoundry/bonus-distribution/internal/references"
	workloadidentity "omsbizfoundry/workload-identity"
)

func main() {
	if err := run(); err != nil {
		slog.Error("bonus-distribution stopped", "error", err)
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
	store, err := bonus.NewPostgresStore(pool)
	if err != nil {
		return err
	}
	project := references.Endpoint{BaseURL: cfg.ProjectBaseURL, AccessToken: cfg.ProjectAccessToken}
	people := references.Endpoint{BaseURL: cfg.PeopleBaseURL, AccessToken: cfg.PeopleAccessToken}
	professional := project
	contract := optionalStaticEndpoint(cfg.ContractBaseURL, cfg.ContractAccessToken)
	moneyReference := optionalStaticEndpoint(cfg.MoneyBaseURL, cfg.MoneyAccessToken)
	var moneyCredentials moneyclient.CredentialSource
	if cfg.IdentityProvider == config.IdentityIAM {
		credentials, credentialErr := workloadidentity.NewTokenSource(cfg.IAMServiceCredentialURL, cfg.IAMTenantID, cfg.WorkloadID, cfg.WorkloadSubjectID, cfg.WorkloadKeyFile, cfg.ReferenceTimeout, cfg.WorkloadRefreshBefore)
		if credentialErr != nil {
			return credentialErr
		}
		project = dynamicEndpoint(cfg.ProjectBaseURL, "project-workspace", "project-workspace.read", credentials)
		professional = dynamicEndpoint(cfg.ProjectBaseURL, "project-workspace", "project-workspace.reference-oa-personnel.read", credentials)
		people = dynamicEndpoint(cfg.PeopleBaseURL, "people-groups", "people-groups.person.reference.read", credentials)
		contract = optionalDynamicEndpoint(cfg.ContractBaseURL, "contract-ledger", "contract-ledger.reference.read", credentials)
		moneyReference = optionalDynamicEndpoint(cfg.MoneyBaseURL, "money-flow-ledger", "money-flow-ledger.receipt-set.reference.read", credentials)
		moneyCredentials = credentials
	}
	moneyPayable, err := newMoneyPayable(cfg, moneyCredentials)
	if err != nil {
		return err
	}
	resolver, err := references.NewHTTPResolver(cfg.ReferenceTimeout, project, people, contract, moneyReference)
	if err != nil {
		return err
	}
	if cfg.ReferenceOABonusProfile == referencebonus.ProfessionalProfile {
		if err = resolver.WithProfessionalEndpoint(professional); err != nil {
			return err
		}
	}
	service, err := bonus.NewService(store, resolver, referenceoa.Evaluator{}, moneyPayable)
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
	options := []httpserver.Option{}
	if cfg.ReferenceOABonusProfile == referencebonus.Profile || cfg.ReferenceOABonusProfile == referencebonus.ProfessionalProfile {
		manualStore, e := referencebonus.NewPostgresStore(pool)
		if e != nil {
			return e
		}
		manualService, e := referencebonus.NewWithProfile(manualStore, resolver, cfg.ReferenceOABonusProfile)
		if e != nil {
			return e
		}
		options = append(options, httpserver.WithReferenceBonus(manualService))
	}
	handler, err := httpserver.New(service, authenticator, options...)
	if err != nil {
		return err
	}
	server := &http.Server{Addr: cfg.HTTPAddress, Handler: handler, ReadHeaderTimeout: cfg.ReferenceTimeout}
	result := make(chan error, 1)
	go func() {
		slog.Info("bonus-distribution listening", "address", cfg.HTTPAddress, "identity_provider", authenticator.ProviderName())
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

func optionalStaticEndpoint(baseURL, token string) references.Endpoint {
	if baseURL == "" {
		return references.Endpoint{}
	}
	return references.Endpoint{BaseURL: baseURL, AccessToken: token}
}

func dynamicEndpoint(baseURL, audience, action string, credentials references.CredentialSource) references.Endpoint {
	return references.Endpoint{BaseURL: baseURL, Audience: audience, Action: action, Credentials: credentials}
}

func newMoneyPayable(cfg config.Config, credentials moneyclient.CredentialSource) (bonus.MoneyPayablePort, error) {
	if cfg.MoneyBaseURL == "" {
		return nil, nil
	}
	endpoint := moneyclient.Endpoint{BaseURL: cfg.MoneyBaseURL}
	if cfg.IdentityProvider == config.IdentityIAM {
		endpoint.Credentials = credentials
	} else {
		endpoint.AccessToken = cfg.MoneyAccessToken
	}
	return moneyclient.NewClient(cfg.ReferenceTimeout, endpoint)
}

func optionalDynamicEndpoint(baseURL, audience, action string, credentials references.CredentialSource) references.Endpoint {
	if baseURL == "" {
		return references.Endpoint{}
	}
	return dynamicEndpoint(baseURL, audience, action, credentials)
}
