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

	"omsbizfoundry/party-directory/internal/auth"
	"omsbizfoundry/party-directory/internal/buildinfo"
	"omsbizfoundry/party-directory/internal/config"
	"omsbizfoundry/party-directory/internal/database"
	"omsbizfoundry/party-directory/internal/httpserver"
	"omsbizfoundry/party-directory/internal/party"
)

func main() {
	if err := run(); err != nil {
		slog.Error("party-directory stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	if buildinfo.Requested(os.Args[1:]) {
		return buildinfo.Write(os.Stdout)
	}
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
	store, err := party.NewPostgresStore(pool)
	if err != nil {
		return err
	}
	service, err := party.NewService(store, cfg.CursorHMACKey)
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
	server := &http.Server{Addr: cfg.HTTPAddress, Handler: handler, ReadHeaderTimeout: 3 * time.Second}
	result := make(chan error, 1)
	go func() {
		slog.Info("party-directory listening", "address", cfg.HTTPAddress, "identity_provider", authenticator.ProviderName())
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
