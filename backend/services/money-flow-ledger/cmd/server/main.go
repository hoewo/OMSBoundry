package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	workloadidentity "omsbizfoundry/workload-identity"

	"omsbizfoundry/money-flow-ledger/internal/allocation"
	"omsbizfoundry/money-flow-ledger/internal/buildinfo"
	"omsbizfoundry/money-flow-ledger/internal/cashevidence"
	"omsbizfoundry/money-flow-ledger/internal/cashtransaction"
	"omsbizfoundry/money-flow-ledger/internal/config"
	"omsbizfoundry/money-flow-ledger/internal/database"
	"omsbizfoundry/money-flow-ledger/internal/httpserver"
	"omsbizfoundry/money-flow-ledger/internal/identity"
	"omsbizfoundry/money-flow-ledger/internal/internaltransfer"
	"omsbizfoundry/money-flow-ledger/internal/ledgerexport"
	"omsbizfoundry/money-flow-ledger/internal/obligation"
	"omsbizfoundry/money-flow-ledger/internal/operatingsummary"
	"omsbizfoundry/money-flow-ledger/internal/party"
	"omsbizfoundry/money-flow-ledger/internal/partymigration"
	"omsbizfoundry/money-flow-ledger/internal/payablesuggestion"
	"omsbizfoundry/money-flow-ledger/internal/receiptset"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if buildinfo.Requested(os.Args[1:]) {
		if err := buildinfo.Write(os.Stdout); err != nil {
			logger.Error("write Money Flow Ledger version", "error", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) != 1 {
		logger.Error("unknown Money Flow Ledger server arguments")
		os.Exit(2)
	}
	if err := run(logger); err != nil {
		logger.Error("money flow ledger stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	rootContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startupContext, cancelStartup := context.WithTimeout(rootContext, 15*time.Second)
	defer cancelStartup()

	pool, err := database.Open(startupContext, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	postgresStatus, err := database.CheckPostgreSQL(startupContext, pool)
	if err != nil {
		return err
	}
	if cfg.MigrationMode == config.MigrationModeAuto {
		if err := database.Migrate(startupContext, pool); err != nil {
			return err
		}
	}
	schemaStatus, err := database.RequireCurrentSchema(startupContext, pool)
	if err != nil {
		return err
	}
	var identityProvider httpserver.IdentityUseCases
	identityOptions := httpserver.Options{
		SessionCookieSecure:  cfg.SessionCookieSecure,
		PublicBasePath:       cfg.PublicBasePath,
		ReferenceTenantID:    cfg.ReferenceTenantID,
		ReferenceAccessToken: cfg.ReferenceAccessToken,
		PartyRuntime:         partymigration.NewRuntimeRegistry(pool),
	}
	if cfg.IdentityProvider == config.IdentityProviderIAM {
		iamProvider, providerErr := identity.NewIAMProvider(cfg.IAMIssuer, cfg.IAMTenantID, cfg.IAMPublicKeysPath)
		if providerErr != nil {
			return providerErr
		}
		identityProvider = iamProvider
		identityOptions.ExternalIdentity = true
		identityOptions.SessionCookieName = "oms_cap_money_flow_ledger"
		partyBaseURL := strings.TrimSpace(os.Getenv("PARTY_BASE_URL"))
		credentialURL := strings.TrimSpace(os.Getenv("IAM_SERVICE_CREDENTIAL_URL"))
		workloadID := strings.TrimSpace(os.Getenv("WORKLOAD_ID"))
		workloadSubjectID := strings.TrimSpace(os.Getenv("WORKLOAD_SUBJECT_ID"))
		workloadKeyFile := strings.TrimSpace(os.Getenv("WORKLOAD_KEY_FILE"))
		if partyBaseURL == "" || credentialURL == "" || workloadID == "" || workloadSubjectID == "" || workloadKeyFile == "" {
			return errors.New("PARTY_BASE_URL, IAM_SERVICE_CREDENTIAL_URL, WORKLOAD_ID, WORKLOAD_SUBJECT_ID and WORKLOAD_KEY_FILE are required in IAM mode")
		}
		credentials, credentialErr := workloadidentity.NewTokenSource(credentialURL, cfg.IAMTenantID, workloadID, workloadSubjectID, workloadKeyFile, 5*time.Second, 30*time.Second)
		if credentialErr != nil {
			return credentialErr
		}
		validator, validatorErr := partymigration.NewHTTPPartyValidator(5*time.Second, partymigration.PartyEndpoint{
			BaseURL: partyBaseURL, Audience: "party-directory", Action: "party-directory.reference.read", Credentials: credentials,
		})
		if validatorErr != nil {
			return validatorErr
		}
		identityOptions.DirectoryParties = partymigration.NewRuntimeDirectoryProjector(pool, validator)
		if cfg.EvidenceBaseURL != "" {
			evidenceValidator, validatorErr := cashevidence.NewHTTPValidator(5*time.Second, cashevidence.Endpoint{
				BaseURL: cfg.EvidenceBaseURL, Audience: "evidence-service", Action: "evidence-service.metadata.read", Credentials: credentials,
			})
			if validatorErr != nil {
				return validatorErr
			}
			evidenceService, serviceErr := cashevidence.NewService(cashevidence.NewPostgresRepository(pool), evidenceValidator)
			if serviceErr != nil {
				return serviceErr
			}
			identityOptions.CashEvidence = evidenceService
		}
	} else {
		localProvider, providerErr := identity.NewService(identity.NewPostgresRepository(pool), cfg.SessionTTL)
		if providerErr != nil {
			return providerErr
		}
		bootstrapped, bootstrapErr := localProvider.EnsureBootstrap(startupContext, identity.BootstrapInput{
			Username: cfg.BootstrapAdminUsername, DisplayName: cfg.BootstrapAdminDisplayName, Password: cfg.BootstrapAdminPassword,
		})
		if bootstrapErr != nil {
			return bootstrapErr
		}
		if bootstrapped {
			logger.Info("money flow local administrator bootstrapped", "username", cfg.BootstrapAdminUsername)
		}
		identityProvider = localProvider
		if cfg.EvidenceBaseURL != "" {
			evidenceValidator, validatorErr := cashevidence.NewHTTPValidator(5*time.Second, cashevidence.Endpoint{
				BaseURL: cfg.EvidenceBaseURL, AccessToken: cfg.EvidenceAccessToken,
			})
			if validatorErr != nil {
				return validatorErr
			}
			evidenceService, serviceErr := cashevidence.NewService(cashevidence.NewPostgresRepository(pool), evidenceValidator)
			if serviceErr != nil {
				return serviceErr
			}
			identityOptions.CashEvidence = evidenceService
		}
	}

	partyService := party.NewService(party.NewPostgresRepository(pool))
	obligationService := obligation.NewService(obligation.NewPostgresRepository(pool))
	payableSuggestionService, err := payablesuggestion.NewService(payablesuggestion.NewPostgresRepository(pool), obligationService)
	if err != nil {
		return err
	}
	identityOptions.PayableSuggestions = payableSuggestionService
	identityOptions.ReceiptSets = receiptset.NewService(receiptset.NewPostgresRepository(pool))
	cashTransactionService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(pool))
	allocationService := allocation.NewService(allocation.NewPostgresRepository(pool))
	operatingSummaryService := operatingsummary.NewService(operatingsummary.NewPostgresRepository(pool))
	ledgerExportService, err := ledgerexport.NewService(ledgerexport.NewPostgresRepository(pool))
	if err != nil {
		return err
	}
	internalTransferService := internaltransfer.NewService(internaltransfer.NewPostgresRepository(pool))
	handler, err := httpserver.New(pool, identityProvider, partyService, obligationService, cashTransactionService, allocationService, operatingSummaryService, ledgerExportService, identityOptions, internalTransferService)
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              cfg.HTTPAddress,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		build := buildinfo.Current()
		logger.Info("money flow ledger listening",
			"address", cfg.HTTPAddress,
			"version", build.Version,
			"commit", build.Commit,
			"migration_mode", cfg.MigrationMode,
			"schema_version", schemaStatus.Current,
			"postgresql_version", postgresStatus.Version,
		)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case <-rootContext.Done():
		shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), cfg.ShutdownPeriod)
		defer cancelShutdown()
		return server.Shutdown(shutdownContext)
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
