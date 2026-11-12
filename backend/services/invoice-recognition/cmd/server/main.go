package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"omsbizfoundry/invoice-recognition/internal/auth"
	"omsbizfoundry/invoice-recognition/internal/config"
	"omsbizfoundry/invoice-recognition/internal/database"
	"omsbizfoundry/invoice-recognition/internal/evidence"
	"omsbizfoundry/invoice-recognition/internal/httpserver"
	"omsbizfoundry/invoice-recognition/internal/provider"
	"omsbizfoundry/invoice-recognition/internal/recognition"
	workloadidentity "omsbizfoundry/workload-identity"
)

func main() {
	if err := run(); err != nil {
		slog.Error("invoice recognition stopped", "error", err)
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
	repository, err := recognition.NewPostgresRepository(pool)
	if err != nil {
		return err
	}
	var credentials *workloadidentity.TokenSource
	if cfg.IdentityProvider == "iam" {
		credentials, err = workloadidentity.NewTokenSource(
			cfg.IAMServiceCredentialURL, cfg.IAMTenantID, cfg.WorkloadID,
			cfg.WorkloadSubjectID, cfg.WorkloadKeyFile, cfg.ReferenceTimeout, cfg.WorkloadRefreshBefore,
		)
		if err != nil {
			return err
		}
	}
	evidenceClient, err := evidence.NewHTTPClient(cfg.EvidenceBaseURL, cfg.EvidenceAccessToken, credentials, cfg.ReferenceTimeout)
	if err != nil {
		return err
	}
	var ai recognition.Provider
	if cfg.AIProviderURL != "" {
		ai, err = provider.NewHTTP(cfg.AIProviderURL, cfg.AIProviderAPIKey, cfg.AIProviderModel, cfg.AIProviderTimeout)
		if err != nil {
			return err
		}
	}
	var localOCR recognition.Provider
	if cfg.LocalOCREnabled {
		localOCR, err = provider.NewLocalImage(cfg.LocalOCRCommand, cfg.PDFRasterizerCommand, cfg.LocalOCRTimeout)
		if err != nil {
			return err
		}
	}
	service := recognition.New(evidenceClient, provider.NewRouter(ai, localOCR), repository, newUUID, func() time.Time { return time.Now().UTC() })
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
		slog.Info("invoice recognition listening", "address", cfg.HTTPAddress)
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

func newUUID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic("secure random source unavailable: " + err.Error())
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16])
}
