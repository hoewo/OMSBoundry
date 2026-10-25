package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"omsbizfoundry/evidence-service/internal/bootstrap"
	"omsbizfoundry/evidence-service/internal/buildinfo"
	"omsbizfoundry/evidence-service/internal/config"
	"omsbizfoundry/evidence-service/internal/database"
	"omsbizfoundry/evidence-service/internal/evidence"
	"omsbizfoundry/evidence-service/internal/referencecheck"
)

func main() {
	if buildinfo.Requested(os.Args[1:]) {
		if err := buildinfo.Write(os.Stdout); err != nil {
			fail(err)
		}
		return
	}
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: evidence-ops check|cleanup|migrate|version")
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		fail(err)
	}
	root, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	timeout := 30 * time.Second
	if os.Args[1] == "cleanup" || os.Args[1] == "migrate" {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(root, timeout)
	defer cancel()
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		fail(err)
	}
	defer pool.Close()
	switch os.Args[1] {
	case "migrate":
		if err := database.Migrate(ctx, pool); err != nil {
			fail(err)
		}
		version, err := database.RequireCurrentSchema(ctx, pool)
		if err != nil {
			fail(err)
		}
		writeJSON(map[string]any{"service": "evidence-service", "operation": "migrate", "schema_version": version, "build": buildinfo.Current()})
	case "check":
		version, err := database.RequireCurrentSchema(ctx, pool)
		if err != nil {
			fail(err)
		}
		provider, err := bootstrap.Provider(cfg)
		if err != nil {
			fail(err)
		}
		if err := provider.Ready(ctx); err != nil {
			fail(fmt.Errorf("storage provider is not ready: %w", err))
		}
		writeJSON(map[string]any{"service": "evidence-service", "operation": "check", "schema_version": version, "storage_provider": provider.Name(), "build": buildinfo.Current()})
	case "cleanup":
		if _, err := database.RequireCurrentSchema(ctx, pool); err != nil {
			fail(err)
		}
		provider, err := bootstrap.Provider(cfg)
		if err != nil {
			fail(err)
		}
		service, err := evidence.NewService(pool, provider, cfg.UploadSessionTTL, cfg.MaxUploadBytes)
		if err != nil {
			fail(err)
		}
		result, err := service.Cleanup(ctx, cfg.CleanupBatchSize, time.Now().UTC().Add(-cfg.CleanupStaleAfter))
		if err != nil {
			fail(err)
		}
		if cfg.ReferenceProtectionURL == "" || len(cfg.ReferenceAccessToken) < 24 {
			writeJSON(map[string]any{"upload_sessions": result, "unreferenced_versions": map[string]any{"status": "failed_closed", "reason": "reference protection is not configured"}})
			fail(fmt.Errorf("REFERENCE_PROTECTION_URL and REFERENCE_PROTECTION_ACCESS_TOKEN are required for unreferenced cleanup"))
		}
		references, err := referencecheck.New(cfg.ReferenceProtectionURL, cfg.ReferenceAccessToken, cfg.ReferenceTimeout)
		if err != nil {
			fail(err)
		}
		service, err = evidence.NewServiceWithReferenceProtector(pool, provider, cfg.UploadSessionTTL, cfg.MaxUploadBytes, references)
		if err != nil {
			fail(err)
		}
		unreferenced, err := service.CleanupUnreferenced(ctx, cfg.CleanupBatchSize, time.Now().UTC().Add(-cfg.UnreferencedRetention))
		if err != nil {
			fail(err)
		}
		writeJSON(map[string]any{"upload_sessions": result, "unreferenced_versions": unreferenced})
		if unreferenced.Failed > 0 {
			fail(fmt.Errorf("unreferenced cleanup has %d failed item(s)", unreferenced.Failed))
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: evidence-ops check|cleanup|migrate|version")
		os.Exit(2)
	}
}

func writeJSON(value any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		fail(err)
	}
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
