package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

type MigrationMode string
type IdentityProvider string

const (
	MigrationAuto    MigrationMode    = "auto"
	MigrationVerify  MigrationMode    = "verify"
	IdentityIAM      IdentityProvider = "iam"
	IdentityTemplate IdentityProvider = "template"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type Config struct {
	HTTPAddress, DatabaseURL, CursorHMACKey                                       string
	ShutdownTimeout                                                               time.Duration
	MigrationMode                                                                 MigrationMode
	IdentityProvider                                                              IdentityProvider
	IAMIssuer, IAMTenantID, IAMPublicKeysPath                                     string
	TemplateTenantID, TemplateSubjectID, TemplateDisplayName, TemplateAccessToken string
}

func Load() (Config, error) {
	cfg := Config{HTTPAddress: env("HTTP_ADDR", ":8090"), DatabaseURL: env("DATABASE_URL", "postgres://party_directory:party_directory_dev@127.0.0.1:55440/party_directory?sslmode=disable"), CursorHMACKey: os.Getenv("CURSOR_HMAC_KEY"), ShutdownTimeout: 10 * time.Second, MigrationMode: MigrationMode(strings.ToLower(strings.TrimSpace(env("MIGRATION_MODE", string(MigrationAuto))))), IdentityProvider: IdentityProvider(strings.ToLower(strings.TrimSpace(env("IDENTITY_PROVIDER", string(IdentityTemplate))))), IAMIssuer: strings.TrimRight(strings.TrimSpace(os.Getenv("IAM_ISSUER")), "/"), IAMTenantID: strings.ToLower(strings.TrimSpace(os.Getenv("IAM_TENANT_ID"))), IAMPublicKeysPath: strings.TrimSpace(os.Getenv("IAM_PUBLIC_KEYS_PATH")), TemplateTenantID: strings.ToLower(strings.TrimSpace(os.Getenv("TEMPLATE_TENANT_ID"))), TemplateSubjectID: strings.ToLower(strings.TrimSpace(os.Getenv("TEMPLATE_SUBJECT_ID"))), TemplateDisplayName: strings.TrimSpace(os.Getenv("TEMPLATE_DISPLAY_NAME")), TemplateAccessToken: os.Getenv("TEMPLATE_ACCESS_TOKEN")}
	if raw := strings.TrimSpace(os.Getenv("SHUTDOWN_TIMEOUT")); raw != "" {
		value, err := time.ParseDuration(raw)
		if err != nil || value <= 0 || value > time.Minute {
			return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT must be positive and no greater than 1m")
		}
		cfg.ShutdownTimeout = value
	}
	if len(cfg.CursorHMACKey) < 32 || strings.HasPrefix(cfg.CursorHMACKey, "replace-with-") {
		return Config{}, fmt.Errorf("CURSOR_HMAC_KEY must be a replaced 32+ byte secret")
	}
	if cfg.MigrationMode != MigrationAuto && cfg.MigrationMode != MigrationVerify {
		return Config{}, fmt.Errorf("MIGRATION_MODE must be auto or verify")
	}
	if cfg.IdentityProvider != IdentityIAM && cfg.IdentityProvider != IdentityTemplate {
		return Config{}, fmt.Errorf("IDENTITY_PROVIDER must be iam or template")
	}
	if cfg.IdentityProvider == IdentityTemplate {
		if !uuidPattern.MatchString(cfg.TemplateTenantID) || !uuidPattern.MatchString(cfg.TemplateSubjectID) || cfg.TemplateDisplayName == "" || len(cfg.TemplateAccessToken) < 24 || strings.HasPrefix(cfg.TemplateAccessToken, "replace-with-") {
			return Config{}, fmt.Errorf("template mode requires valid template identity and a replaced 24+ byte token")
		}
		if cfg.IAMIssuer != "" || cfg.IAMTenantID != "" || cfg.IAMPublicKeysPath != "" {
			return Config{}, fmt.Errorf("IAM settings are not allowed in template mode")
		}
		return cfg, nil
	}
	parsed, err := url.Parse(cfg.IAMIssuer)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || !uuidPattern.MatchString(cfg.IAMTenantID) || cfg.IAMPublicKeysPath == "" {
		return Config{}, fmt.Errorf("iam mode requires HTTPS issuer, tenant and public keys")
	}
	if cfg.TemplateTenantID != "" || cfg.TemplateSubjectID != "" || cfg.TemplateDisplayName != "" || cfg.TemplateAccessToken != "" {
		return Config{}, fmt.Errorf("template settings are not allowed in iam mode")
	}
	return cfg, nil
}
func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
