package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"omsbizfoundry/internal-finance-requests/internal/auth"
	"omsbizfoundry/internal-finance-requests/internal/config"
	"omsbizfoundry/internal-finance-requests/internal/database"
	"omsbizfoundry/internal-finance-requests/internal/finance"
	"omsbizfoundry/internal-finance-requests/internal/httpserver"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		slog.Error("internal-finance-requests stopped", "error", err)
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
	store, err := finance.NewPostgresStore(pool)
	if err != nil {
		return err
	}
	service, _ := finance.NewService(store)
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
	go func() {
		slog.Info("internal-finance-requests listening", "address", cfg.HTTPAddress)
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
