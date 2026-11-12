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

	"omsbizfoundry/workforce-attendance/internal/attendance"
	"omsbizfoundry/workforce-attendance/internal/auth"
	"omsbizfoundry/workforce-attendance/internal/config"
	"omsbizfoundry/workforce-attendance/internal/database"
	"omsbizfoundry/workforce-attendance/internal/httpserver"
	"omsbizfoundry/workforce-attendance/internal/personreference"
	workloadidentity "omsbizfoundry/workload-identity"
)

func main() {
	if err := run(); err != nil {
		slog.Error("workforce attendance stopped", "error", err)
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
		err = database.Migrate(ctx, pool)
	} else {
		_, err = database.RequireCurrentSchema(ctx, pool)
	}
	if err != nil {
		return err
	}
	store, err := attendance.NewPostgresStore(pool)
	if err != nil {
		return err
	}
	var people *personreference.HTTPClient
	if cfg.IdentityProvider == "iam" {
		credentials, credentialErr := workloadidentity.NewTokenSource(cfg.IAMServiceCredentialURL, cfg.IAMTenantID, cfg.WorkloadID, cfg.WorkloadSubjectID, cfg.WorkloadKeyFile, cfg.ReferenceTimeout, cfg.WorkloadRefreshBefore)
		if credentialErr != nil {
			return credentialErr
		}
		people, err = personreference.NewHTTPClient(cfg.PeopleBaseURL, "", credentials, cfg.ReferenceTimeout)
	} else {
		people, err = personreference.NewHTTPClient(cfg.PeopleBaseURL, cfg.PeopleAccessToken, nil, cfg.ReferenceTimeout)
	}
	if err != nil {
		return err
	}
	service, err := attendance.NewService(store, people)
	if err != nil {
		return err
	}
	var authenticator auth.Authenticator
	if cfg.IdentityProvider == "iam" {
		authenticator, err = auth.NewIAMProvider(cfg.IAMIssuer, cfg.IAMTenantID, cfg.IAMPublicKeysPath)
	} else {
		authenticator, err = auth.NewTemplateProvider(cfg.TemplateTenantID, cfg.TemplateSubjectID, cfg.TemplateDisplayName, cfg.TemplateAccessToken)
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
	go func() {
		slog.Info("workforce attendance listening", "address", cfg.HTTPAddress)
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
