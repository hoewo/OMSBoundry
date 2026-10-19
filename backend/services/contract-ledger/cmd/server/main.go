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

	"omsbizfoundry/contract-ledger/internal/audit"
	"omsbizfoundry/contract-ledger/internal/buildinfo"
	"omsbizfoundry/contract-ledger/internal/config"
	"omsbizfoundry/contract-ledger/internal/contract"
	"omsbizfoundry/contract-ledger/internal/contractevidence"
	"omsbizfoundry/contract-ledger/internal/database"
	"omsbizfoundry/contract-ledger/internal/evidence"
	"omsbizfoundry/contract-ledger/internal/httpserver"
	"omsbizfoundry/contract-ledger/internal/identity"
	"omsbizfoundry/contract-ledger/internal/ledgerexport"
	"omsbizfoundry/contract-ledger/internal/ledgerrestore"
	"omsbizfoundry/contract-ledger/internal/party"
	"omsbizfoundry/contract-ledger/internal/partymigration"
	"omsbizfoundry/contract-ledger/internal/referenceoa"
	workloadidentity "omsbizfoundry/workload-identity"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if buildinfo.Requested(os.Args[1:]) {
		if err := buildinfo.Write(os.Stdout); err != nil {
			logger.Error("write Contract Ledger version", "error", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) != 1 {
		logger.Error("unknown Contract Ledger server arguments")
		os.Exit(2)
	}
	if err := run(logger); err != nil {
		logger.Error("contract ledger stopped", "error", err)
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

	partyService := party.NewService(party.NewPostgresRepository(pool))
	contractService := contract.NewService(contract.NewPostgresRepository(pool))
	evidenceStore, err := evidence.NewLocalStore(cfg.EvidenceStorageDirectory)
	if err != nil {
		return err
	}
	if err := evidenceStore.CheckWritable(startupContext); err != nil {
		return err
	}
	evidenceService, err := evidence.NewService(
		evidence.NewPostgresRepository(pool),
		evidenceStore,
		cfg.EvidenceMaxUploadBytes,
	)
	if err != nil {
		return err
	}
	var identityProvider httpserver.IdentityUseCases
	refOA := referenceoa.NewStore(pool)
	var externalEvidenceValidator referenceoa.EvidenceValidator
	if cfg.ExternalEvidenceBaseURL != "" {
		var credentials referenceoa.CredentialSource
		if cfg.IdentityProvider == config.IdentityProviderIAM {
			credentials, err = workloadidentity.NewTokenSource(cfg.IAMServiceCredentialURL, cfg.IAMTenantID, cfg.WorkloadID, cfg.WorkloadSubjectID, cfg.WorkloadKeyFile, cfg.ReferenceTimeout, 15*time.Second)
			if err != nil {
				return err
			}
		}
		reader, e := referenceoa.NewHTTPFileValidator(cfg.ExternalEvidenceBaseURL, cfg.ExternalEvidenceAccessToken, credentials, cfg.ReferenceTimeout)
		if e != nil {
			return e
		}
		refOA.SetEvidenceValidator(reader)
		externalEvidenceValidator = reader
	}
	identityOptions := httpserver.Options{
		SessionCookieSecure:  cfg.SessionCookieSecure,
		PublicBasePath:       cfg.PublicBasePath,
		ReferenceTenantID:    cfg.ReferenceTenantID,
		ReferenceAccessToken: cfg.ReferenceAccessToken,
		PartyRuntime:         partymigration.NewRuntimeRegistry(pool),
		ReferenceOADrafts:    refOA,
	}
	if externalEvidenceValidator != nil {
		contractEvidence, serviceErr := contractevidence.NewService(contractevidence.NewPostgresRepository(pool), externalEvidenceValidator)
		if serviceErr != nil {
			return serviceErr
		}
		identityOptions.ContractEvidence = contractEvidence
	}
	if cfg.IdentityProvider == config.IdentityProviderIAM {
		iamProvider, err := identity.NewIAMProvider(cfg.IAMIssuer, cfg.IAMTenantID, cfg.IAMPublicKeysPath)
		if err != nil {
			return err
		}
		identityProvider = iamProvider
		identityOptions.ExternalIdentity = true
		identityOptions.SessionCookieName = "oms_cap_contract_ledger"
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
	} else {
		localProvider, err := identity.NewService(identity.NewPostgresRepository(pool), cfg.SessionTTL)
		if err != nil {
			return err
		}
		if _, err := localProvider.EnsureBootstrap(startupContext, identity.BootstrapInput{Username: cfg.BootstrapAdminUsername, DisplayName: cfg.BootstrapAdminDisplayName, Password: cfg.BootstrapAdminPassword}); err != nil {
			return err
		}
		identityProvider = localProvider
	}
	ledgerExportService, err := ledgerexport.NewService(ledgerexport.NewPostgresRepository(pool))
	if err != nil {
		return err
	}
	ledgerRestoreService, err := ledgerrestore.NewService(ledgerrestore.NewPostgresRepository(pool))
	if err != nil {
		return err
	}
	handler, err := httpserver.NewServer(
		pool,
		partyService,
		contractService,
		evidenceService,
		ledgerExportService,
		ledgerRestoreService,
		identityProvider,
		audit.NewRepository(pool),
		identityOptions,
	)
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
		logger.Info("contract ledger listening",
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
