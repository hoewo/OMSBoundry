package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPAddress              = ":8081"
	defaultDatabaseURL              = "postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable"
	defaultShutdownPeriod           = 10 * time.Second
	defaultSessionTTL               = 12 * time.Hour
	defaultEvidenceStorageDirectory = ".data/evidence"
	defaultEvidenceMaxUploadBytes   = int64(20 << 20)
	defaultMigrationMode            = MigrationModeAuto
	defaultIdentityProvider         = IdentityProviderLocal
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type MigrationMode string
type IdentityProvider string

const (
	MigrationModeAuto     MigrationMode    = "auto"
	MigrationModeVerify   MigrationMode    = "verify"
	IdentityProviderLocal IdentityProvider = "local"
	IdentityProviderIAM   IdentityProvider = "iam"
)

type Config struct {
	HTTPAddress                 string
	DatabaseURL                 string
	ShutdownPeriod              time.Duration
	SessionTTL                  time.Duration
	SessionCookieSecure         bool
	BootstrapAdminUsername      string
	BootstrapAdminDisplayName   string
	BootstrapAdminPassword      string
	EvidenceStorageDirectory    string
	EvidenceMaxUploadBytes      int64
	MigrationMode               MigrationMode
	IdentityProvider            IdentityProvider
	IAMIssuer                   string
	IAMTenantID                 string
	IAMPublicKeysPath           string
	PublicBasePath              string
	ReferenceTenantID           string
	ReferenceAccessToken        string
	ExternalEvidenceBaseURL     string
	ExternalEvidenceAccessToken string
	IAMServiceCredentialURL     string
	WorkloadID                  string
	WorkloadSubjectID           string
	WorkloadKeyFile             string
	ReferenceTimeout            time.Duration
}

func Load() (Config, error) {
	shutdownPeriod := defaultShutdownPeriod
	if raw := os.Getenv("SHUTDOWN_TIMEOUT"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("parse SHUTDOWN_TIMEOUT: %w", err)
		}
		if parsed <= 0 {
			return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT must be positive")
		}
		shutdownPeriod = parsed
	}

	sessionTTL := defaultSessionTTL
	if raw := os.Getenv("SESSION_TTL"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("parse SESSION_TTL: %w", err)
		}
		if parsed <= 0 {
			return Config{}, fmt.Errorf("SESSION_TTL must be positive")
		}
		sessionTTL = parsed
	}
	sessionCookieSecure := false
	if raw := os.Getenv("SESSION_COOKIE_SECURE"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("parse SESSION_COOKIE_SECURE: %w", err)
		}
		sessionCookieSecure = parsed
	}
	bootstrapUsername := strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_USERNAME"))
	bootstrapPassword := os.Getenv("BOOTSTRAP_ADMIN_PASSWORD")
	if (bootstrapUsername == "") != (bootstrapPassword == "") {
		return Config{}, fmt.Errorf("BOOTSTRAP_ADMIN_USERNAME and BOOTSTRAP_ADMIN_PASSWORD must be set together")
	}
	evidenceMaxUploadBytes := defaultEvidenceMaxUploadBytes
	if raw := strings.TrimSpace(os.Getenv("EVIDENCE_MAX_UPLOAD_BYTES")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("parse EVIDENCE_MAX_UPLOAD_BYTES: %w", err)
		}
		if parsed <= 0 {
			return Config{}, fmt.Errorf("EVIDENCE_MAX_UPLOAD_BYTES must be positive")
		}
		evidenceMaxUploadBytes = parsed
	}
	migrationMode := MigrationMode(strings.ToLower(strings.TrimSpace(envOrDefault("MIGRATION_MODE", string(defaultMigrationMode)))))
	if migrationMode != MigrationModeAuto && migrationMode != MigrationModeVerify {
		return Config{}, fmt.Errorf("MIGRATION_MODE must be auto or verify")
	}
	identityProvider := IdentityProvider(strings.ToLower(strings.TrimSpace(envOrDefault("IDENTITY_PROVIDER", string(defaultIdentityProvider)))))
	if identityProvider != IdentityProviderLocal && identityProvider != IdentityProviderIAM {
		return Config{}, fmt.Errorf("IDENTITY_PROVIDER must be local or iam")
	}
	iamIssuer := strings.TrimRight(strings.TrimSpace(os.Getenv("IAM_ISSUER")), "/")
	iamTenantID := strings.ToLower(strings.TrimSpace(os.Getenv("IAM_TENANT_ID")))
	iamPublicKeysPath := strings.TrimSpace(os.Getenv("IAM_PUBLIC_KEYS_PATH"))
	publicBasePath := strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_BASE_PATH")), "/")
	referenceTenantID := strings.ToLower(strings.TrimSpace(os.Getenv("REFERENCE_TENANT_ID")))
	referenceAccessToken := os.Getenv("REFERENCE_ACCESS_TOKEN")
	if publicBasePath != "" && (!strings.HasPrefix(publicBasePath, "/") || strings.HasPrefix(publicBasePath, "//") || strings.Contains(publicBasePath, "\\")) {
		return Config{}, fmt.Errorf("PUBLIC_BASE_PATH must be empty or a single-origin absolute path")
	}
	if identityProvider == IdentityProviderIAM {
		if iamIssuer == "" || iamTenantID == "" || iamPublicKeysPath == "" {
			return Config{}, fmt.Errorf("IAM_ISSUER, IAM_TENANT_ID and IAM_PUBLIC_KEYS_PATH are required in iam mode")
		}
		if bootstrapUsername != "" || bootstrapPassword != "" {
			return Config{}, fmt.Errorf("local bootstrap credentials are not allowed in iam mode")
		}
		if referenceAccessToken != "" {
			return Config{}, fmt.Errorf("static reference credentials are not allowed in iam mode")
		}
	}
	if (referenceTenantID == "") != (referenceAccessToken == "") {
		return Config{}, fmt.Errorf("REFERENCE_TENANT_ID and REFERENCE_ACCESS_TOKEN must be set together")
	}
	if referenceTenantID != "" && (!uuidPattern.MatchString(referenceTenantID) || len(referenceAccessToken) < 24 || strings.HasPrefix(referenceAccessToken, "replace-with-")) {
		return Config{}, fmt.Errorf("reference access requires a valid tenant id and replaced 24+ byte token")
	}
	externalEvidenceBaseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("EXTERNAL_EVIDENCE_BASE_URL")), "/")
	externalEvidenceAccessToken := os.Getenv("EXTERNAL_EVIDENCE_ACCESS_TOKEN")
	iamServiceCredentialURL := strings.TrimSpace(os.Getenv("IAM_SERVICE_CREDENTIAL_URL"))
	workloadID := strings.TrimSpace(os.Getenv("WORKLOAD_ID"))
	workloadSubjectID := strings.TrimSpace(os.Getenv("WORKLOAD_SUBJECT_ID"))
	workloadKeyFile := strings.TrimSpace(os.Getenv("WORKLOAD_KEY_FILE"))
	referenceTimeout := 3 * time.Second
	if raw := os.Getenv("REFERENCE_TIMEOUT"); raw != "" {
		var e error
		referenceTimeout, e = time.ParseDuration(raw)
		if e != nil || referenceTimeout <= 0 || referenceTimeout > 30*time.Second {
			return Config{}, fmt.Errorf("invalid REFERENCE_TIMEOUT")
		}
	}
	if externalEvidenceBaseURL == "" {
		if externalEvidenceAccessToken != "" || iamServiceCredentialURL != "" || workloadID != "" || workloadSubjectID != "" || workloadKeyFile != "" {
			return Config{}, fmt.Errorf("external Evidence credentials require EXTERNAL_EVIDENCE_BASE_URL")
		}
	} else {
		validURL := func(raw string) bool {
			u, e := url.Parse(raw)
			return e == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil && u.RawQuery == "" && u.Fragment == ""
		}
		if !validURL(externalEvidenceBaseURL) {
			return Config{}, fmt.Errorf("invalid EXTERNAL_EVIDENCE_BASE_URL")
		}
		if identityProvider == IdentityProviderIAM {
			if externalEvidenceAccessToken != "" || !validURL(iamServiceCredentialURL) || !regexp.MustCompile(`^[a-z][a-z0-9-]{2,63}$`).MatchString(workloadID) || !uuidPattern.MatchString(workloadSubjectID) || workloadKeyFile == "" {
				return Config{}, fmt.Errorf("external Evidence in IAM mode requires exact workload credentials, not a static token")
			}
		} else if len(externalEvidenceAccessToken) < 24 || strings.HasPrefix(externalEvidenceAccessToken, "replace-with-") || iamServiceCredentialURL != "" || workloadID != "" || workloadSubjectID != "" || workloadKeyFile != "" {
			return Config{}, fmt.Errorf("local external Evidence requires a replaced token and no IAM workload settings")
		}
	}

	return Config{
		HTTPAddress:               envOrDefault("HTTP_ADDR", defaultHTTPAddress),
		DatabaseURL:               envOrDefault("DATABASE_URL", defaultDatabaseURL),
		ShutdownPeriod:            shutdownPeriod,
		SessionTTL:                sessionTTL,
		SessionCookieSecure:       sessionCookieSecure,
		BootstrapAdminUsername:    bootstrapUsername,
		BootstrapAdminDisplayName: strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_DISPLAY_NAME")),
		BootstrapAdminPassword:    bootstrapPassword,
		EvidenceStorageDirectory:  envOrDefault("EVIDENCE_STORAGE_DIR", defaultEvidenceStorageDirectory),
		EvidenceMaxUploadBytes:    evidenceMaxUploadBytes,
		MigrationMode:             migrationMode,
		IdentityProvider:          identityProvider,
		IAMIssuer:                 iamIssuer,
		IAMTenantID:               iamTenantID,
		IAMPublicKeysPath:         iamPublicKeysPath,
		PublicBasePath:            publicBasePath,
		ReferenceTenantID:         referenceTenantID,
		ReferenceAccessToken:      referenceAccessToken,
		ExternalEvidenceBaseURL:   externalEvidenceBaseURL, ExternalEvidenceAccessToken: externalEvidenceAccessToken, IAMServiceCredentialURL: iamServiceCredentialURL, WorkloadID: workloadID, WorkloadSubjectID: workloadSubjectID, WorkloadKeyFile: workloadKeyFile, ReferenceTimeout: referenceTimeout,
	}, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
