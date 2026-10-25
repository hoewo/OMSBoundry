package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"omsbizfoundry/gateway/internal/config"
	"omsbizfoundry/gateway/internal/credential"
	gatewayserver "omsbizfoundry/gateway/internal/gateway"
	"omsbizfoundry/gateway/internal/manifest"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("gateway stopped", "error", err)
		os.Exit(1)
	}
}
func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	suite, err := manifest.Load(cfg.SuiteManifestPath)
	if err != nil {
		return err
	}
	verifier := credential.New(cfg.PublicKeysPath, cfg.Issuer, suite.TenantID)
	if err := verifier.Ready(); err != nil {
		return err
	}
	handler, err := gatewayserver.New(suite, verifier, logger, gatewayserver.Options{CookieSecure: cfg.CookieSecure, TrustProxyClientIP: cfg.TrustProxyClientIP, RequestsPerMinute: cfg.RequestsPerMinute, OpenAPIKeyRequestsPerMinute: cfg.OpenAPIKeyRequestsPerMinute, OpenAPITenantRequestsPerMinute: cfg.OpenAPITenantRequestsPerMinute, OpenAPIUploadConcurrency: cfg.OpenAPIUploadConcurrency, OpenAPIRequireHTTPS: cfg.OpenAPIRequireHTTPS})
	if err != nil {
		return err
	}
	server := &http.Server{Addr: cfg.HTTPAddress, Handler: gatewayserver.SecurityHeaders(handler), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	root, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errorsChannel := make(chan error, 1)
	go func() {
		logger.Info("gateway listening", "address", cfg.HTTPAddress, "suite", suite.SuiteID)
		errorsChannel <- server.ListenAndServe()
	}()
	select {
	case <-root.Done():
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownPeriod)
		defer cancel()
		return server.Shutdown(ctx)
	case err := <-errorsChannel:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
