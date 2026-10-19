package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"omsbizfoundry/unified-workbench/internal/config"
	"omsbizfoundry/unified-workbench/internal/server"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("workbench stopped", "error", err)
		os.Exit(1)
	}
}
func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	handler, err := server.NewWithProfiles(cfg.SuiteManifestPath, server.ReferenceOAProfiles{ProductionPersonnel: cfg.ProductionPersonnelProfile, Bonus: cfg.BonusProfile, ProductionStage: cfg.ProductionStageProfile, OMSQualificationPartyID: cfg.OMSQualificationPartyID})
	if err != nil {
		return err
	}
	httpServer := &http.Server{Addr: cfg.HTTPAddress, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	root, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errorsChannel := make(chan error, 1)
	go func() {
		logger.Info("unified workbench listening", "address", cfg.HTTPAddress)
		errorsChannel <- httpServer.ListenAndServe()
	}()
	select {
	case <-root.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(ctx)
	case err := <-errorsChannel:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
