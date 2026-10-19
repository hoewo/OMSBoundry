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
	defaultHTTPAddress         = ":8083"
	defaultDatabaseURL         = "postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable"
	defaultShutdownPeriod      = 10 * time.Second
	defaultSessionTTL          = 12 * time.Hour
	defaultSessionCookieSecure = false
	defaultMigrationMode       = MigrationModeAuto
	defaultIdentityProvider    = IdentityProviderLocal
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
	HTTPAddress               string
	DatabaseURL               string
	ShutdownPeriod            time.Duration
	SessionTTL                time.Duration
	SessionCookieSecure       bool
	BootstrapAdminUsername    string
	BootstrapAdminDisplayName string
	BootstrapAdminPassword    string
	MigrationMode             MigrationMode
	IdentityProvider          IdentityProvider
	IAMIssuer                 string
	IAMTenantID               string
	IAMPublicKeysPath         string
	PublicBasePath            string
	ReferenceTenantID         string
	ReferenceAccessToken      string
	EvidenceBaseURL           string
	EvidenceAccessToken       string
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
	sessionCookieSecure := defaultSessionCookieSecure
	if raw := os.Getenv("SESSION_COOKIE_SECURE"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("parse SESSION_COOKIE_SECURE: %w", err)
		}
		sessionCookieSecure = parsed
	}
	bootstrapUsername := strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_USERNAME"))
	bootstrapDisplayName := strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_DISPLAY_NAME"))
	bootstrapPassword := os.Getenv("BOOTSTRAP_ADMIN_PASSWORD")
	if (bootstrapUsername == "") != (bootstrapPassword == "") {
		return Config{}, fmt.Errorf("BOOTSTRAP_ADMIN_USERNAME and BOOTSTRAP_ADMIN_PASSWORD must be set together")
	}
	if bootstrapUsername == "" && bootstrapDisplayName != "" {
		return Config{}, fmt.Errorf("BOOTSTRAP_ADMIN_DISPLAY_NAME requires BOOTSTRAP_ADMIN_USERNAME")
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
	evidenceBaseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("EVIDENCE_BASE_URL")), "/")
	evidenceAccessToken := os.Getenv("EVIDENCE_ACCESS_TOKEN")
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
		if evidenceAccessToken != "" {
			return Config{}, fmt.Errorf("static Evidence credentials are not allowed in iam mode")
		}
	}
	if (referenceTenantID == "") != (referenceAccessToken == "") {
		return Config{}, fmt.Errorf("REFERENCE_TENANT_ID and REFERENCE_ACCESS_TOKEN must be set together")
	}
	if referenceTenantID != "" && (!uuidPattern.MatchString(referenceTenantID) || len(referenceAccessToken) < 24 || strings.HasPrefix(referenceAccessToken, "replace-with-")) {
		return Config{}, fmt.Errorf("reference access requires a valid tenant id and replaced 24+ byte token")
	}
	if identityProvider == IdentityProviderLocal && ((evidenceBaseURL == "") != (evidenceAccessToken == "")) {
		return Config{}, fmt.Errorf("EVIDENCE_BASE_URL and EVIDENCE_ACCESS_TOKEN must be set together in local mode")
	}
	if evidenceBaseURL != "" {
		parsed, err := url.Parse(evidenceBaseURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return Config{}, fmt.Errorf("EVIDENCE_BASE_URL must be an absolute http or https URL")
		}
		if identityProvider == IdentityProviderLocal && (len(evidenceAccessToken) < 24 || strings.HasPrefix(evidenceAccessToken, "replace-with-")) {
			return Config{}, fmt.Errorf("local Evidence access requires a replaced 24+ byte token")
		}
	}

	return Config{
		HTTPAddress:               envOrDefault("HTTP_ADDR", defaultHTTPAddress),
		DatabaseURL:               envOrDefault("DATABASE_URL", defaultDatabaseURL),
		ShutdownPeriod:            shutdownPeriod,
		SessionTTL:                sessionTTL,
		SessionCookieSecure:       sessionCookieSecure,
		BootstrapAdminUsername:    bootstrapUsername,
		BootstrapAdminDisplayName: bootstrapDisplayName,
		BootstrapAdminPassword:    bootstrapPassword,
		MigrationMode:             migrationMode,
		IdentityProvider:          identityProvider,
		IAMIssuer:                 iamIssuer,
		IAMTenantID:               iamTenantID,
		IAMPublicKeysPath:         iamPublicKeysPath,
		PublicBasePath:            publicBasePath,
		ReferenceTenantID:         referenceTenantID,
		ReferenceAccessToken:      referenceAccessToken,
		EvidenceBaseURL:           evidenceBaseURL,
		EvidenceAccessToken:       evidenceAccessToken,
	}, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
