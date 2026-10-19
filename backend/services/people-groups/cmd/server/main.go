package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"omsbizfoundry/people-groups/internal/auth"
	"omsbizfoundry/people-groups/internal/config"
	"omsbizfoundry/people-groups/internal/database"
	"omsbizfoundry/people-groups/internal/httpserver"
	"omsbizfoundry/people-groups/internal/peoplegroups"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("load configuration", "error", err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("open database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	if cfg.MigrationMode == config.MigrationAuto {
		err = database.Migrate(ctx, pool)
	} else {
		_, err = database.RequireCurrentSchema(ctx, pool)
	}
	if err != nil {
		logger.Error("prepare database", "error", err)
		os.Exit(1)
	}
	service, err := peoplegroups.NewService(pool)
	if err != nil {
		logger.Error("create service", "error", err)
		os.Exit(1)
	}
	var authenticator auth.Authenticator
	var templateProvider *auth.TemplateProvider
	if cfg.IdentityProvider == config.IdentityIAM {
		authenticator, err = auth.NewIAMProvider(cfg.IAMIssuer, cfg.IAMTenantID, cfg.IAMPublicKeysPath)
	} else {
		templateProvider, err = auth.NewTemplateProvider(cfg.TemplateTenantID, cfg.TemplateSubjectID, cfg.TemplateDisplayName, cfg.TemplateAccessToken)
		authenticator = templateProvider
	}
	if err != nil {
		logger.Error("create identity provider", "error", err)
		os.Exit(1)
	}
	handler, err := httpserver.New(service, authenticator, httpserver.Options{PublicBasePath: cfg.PublicBasePath, SessionCookieSecure: cfg.SessionCookieSecure, TemplateProvider: templateProvider})
	if err != nil {
		logger.Error("create HTTP server", "error", err)
		os.Exit(1)
	}
	server := &http.Server{Addr: cfg.HTTPAddress, Handler: handler, ReadHeaderTimeout: 5e9}
	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown server", "error", err)
		}
	}()
	logger.Info("people-groups listening", "address", cfg.HTTPAddress, "identity_provider", authenticator.ProviderName())
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("serve HTTP", "error", err)
		os.Exit(1)
	}
}
