package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"omsbizfoundry/payroll/internal/auth"
	"omsbizfoundry/payroll/internal/config"
	"omsbizfoundry/payroll/internal/database"
	"omsbizfoundry/payroll/internal/httpserver"
	"omsbizfoundry/payroll/internal/moneypayment"
	domain "omsbizfoundry/payroll/internal/payroll"
	"omsbizfoundry/payroll/internal/personreference"
	workloadidentity "omsbizfoundry/workload-identity"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		slog.Error("payroll stopped", "error", err)
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
	if cfg.MigrationMode == "auto" {
		if err = database.Migrate(ctx, pool); err != nil {
			return err
		}
	} else if _, err = database.RequireCurrentSchema(ctx, pool); err != nil {
		return err
	}
	store, err := domain.NewPostgresStore(pool)
	if err != nil {
		return err
	}
	var credentials *workloadidentity.TokenSource
	if (cfg.PeopleBaseURL != "" || cfg.MoneyBaseURL != "") && cfg.IdentityProvider == "iam" {
		credentials, err = workloadidentity.NewTokenSource(cfg.IAMServiceCredentialURL, cfg.IAMTenantID, cfg.WorkloadID, cfg.WorkloadSubjectID, cfg.WorkloadKeyFile, cfg.ReferenceTimeout, cfg.WorkloadRefreshBefore)
		if err != nil {
			return err
		}
	}
	var personReferences domain.PersonReferencePort
	if cfg.PeopleBaseURL != "" && cfg.IdentityProvider == "iam" {
		personReferences, err = personreference.NewHTTPValidator(cfg.PeopleBaseURL, "", credentials, cfg.ReferenceTimeout)
	} else if cfg.PeopleBaseURL != "" {
		personReferences, err = personreference.NewHTTPValidator(cfg.PeopleBaseURL, cfg.PeopleAccessToken, nil, cfg.ReferenceTimeout)
	}
	if err != nil {
		return err
	}
	var paymentDelivery domain.PaymentDeliveryPort
	if cfg.MoneyBaseURL != "" && cfg.IdentityProvider == "iam" {
		paymentDelivery, err = moneypayment.NewHTTPClient(cfg.MoneyBaseURL, "", credentials, cfg.ReferenceTimeout)
	} else if cfg.MoneyBaseURL != "" {
		paymentDelivery, err = moneypayment.NewHTTPClient(cfg.MoneyBaseURL, cfg.MoneyAccessToken, nil, cfg.ReferenceTimeout)
	}
	if err != nil {
		return err
	}
	dependencies := make([]any, 0, 2)
	if personReferences != nil {
		dependencies = append(dependencies, personReferences)
	}
	if paymentDelivery != nil {
		dependencies = append(dependencies, paymentDelivery)
	}
	service, err := domain.NewService(store, dependencies...)
	if err != nil {
		return err
	}
	var authenticator auth.Authenticator
	if cfg.IdentityProvider == "template" {
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
	server := &http.Server{Addr: cfg.HTTPAddress, Handler: handler, ReadHeaderTimeout: 3 * time.Second}
	result := make(chan error, 1)
	go func() { slog.Info("payroll listening", "address", cfg.HTTPAddress); result <- server.ListenAndServe() }()
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
