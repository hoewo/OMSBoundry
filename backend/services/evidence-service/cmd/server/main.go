package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"omsbizfoundry/evidence-service/internal/auth"
	"omsbizfoundry/evidence-service/internal/bootstrap"
	"omsbizfoundry/evidence-service/internal/buildinfo"
	"omsbizfoundry/evidence-service/internal/config"
	"omsbizfoundry/evidence-service/internal/database"
	"omsbizfoundry/evidence-service/internal/evidence"
	"omsbizfoundry/evidence-service/internal/httpserver"
)

func main() {
	if buildinfo.Requested(os.Args[1:]) {
		if err := buildinfo.Write(os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
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
	provider, err := bootstrap.Provider(cfg)
	if err != nil {
		logger.Error("create storage provider", "error", err)
		os.Exit(1)
	}
	service, err := evidence.NewService(pool, provider, cfg.UploadSessionTTL, cfg.MaxUploadBytes)
	if err != nil {
		logger.Error("create evidence service", "error", err)
		os.Exit(1)
	}
	var authenticator auth.Authenticator
	if cfg.IdentityProvider == config.IdentityIAM {
		authenticator, err = auth.NewIAMProvider(cfg.IAMIssuer, cfg.IAMTenantID, cfg.IAMPublicKeysPath)
	} else {
		authenticator, err = auth.NewTemplateProvider(cfg.TemplateTenantID, cfg.TemplateSubjectID, cfg.TemplateDisplayName, cfg.TemplateAccessToken)
	}
	if err != nil {
		logger.Error("create identity provider", "error", err)
		os.Exit(1)
	}
	handler, err := httpserver.New(service, authenticator, provider, cfg.MaxUploadBytes)
	if err != nil {
		logger.Error("create HTTP server", "error", err)
		os.Exit(1)
	}
	server := &http.Server{Addr: cfg.HTTPAddress, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: cfg.UploadSessionTTL + time.Minute, WriteTimeout: cfg.UploadSessionTTL + time.Minute, IdleTimeout: 2 * time.Minute}
	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown server", "error", err)
		}
	}()
	logger.Info("evidence-service listening", "address", cfg.HTTPAddress, "identity_provider", authenticator.ProviderName(), "storage_provider", provider.Name())
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("serve HTTP", "error", err)
		os.Exit(1)
	}
}
