package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
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
	HTTPAddress         string
	DatabaseURL         string
	ShutdownTimeout     time.Duration
	MigrationMode       MigrationMode
	IdentityProvider    IdentityProvider
	PublicBasePath      string
	SessionCookieSecure bool
	IAMIssuer           string
	IAMTenantID         string
	IAMPublicKeysPath   string
	TemplateTenantID    string
	TemplateSubjectID   string
	TemplateDisplayName string
	TemplateAccessToken string
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddress:         env("HTTP_ADDR", ":8084"),
		DatabaseURL:         env("DATABASE_URL", "postgres://people_groups:people_groups_dev@127.0.0.1:55435/people_groups?sslmode=disable"),
		ShutdownTimeout:     10 * time.Second,
		MigrationMode:       MigrationMode(strings.ToLower(strings.TrimSpace(env("MIGRATION_MODE", string(MigrationAuto))))),
		IdentityProvider:    IdentityProvider(strings.ToLower(strings.TrimSpace(env("IDENTITY_PROVIDER", string(IdentityTemplate))))),
		PublicBasePath:      strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_BASE_PATH")), "/"),
		IAMIssuer:           strings.TrimRight(strings.TrimSpace(os.Getenv("IAM_ISSUER")), "/"),
		IAMTenantID:         strings.ToLower(strings.TrimSpace(os.Getenv("IAM_TENANT_ID"))),
		IAMPublicKeysPath:   strings.TrimSpace(os.Getenv("IAM_PUBLIC_KEYS_PATH")),
		TemplateTenantID:    strings.ToLower(strings.TrimSpace(os.Getenv("TEMPLATE_TENANT_ID"))),
		TemplateSubjectID:   strings.ToLower(strings.TrimSpace(os.Getenv("TEMPLATE_SUBJECT_ID"))),
		TemplateDisplayName: strings.TrimSpace(os.Getenv("TEMPLATE_DISPLAY_NAME")),
		TemplateAccessToken: os.Getenv("TEMPLATE_ACCESS_TOKEN"),
	}
	if raw := strings.TrimSpace(os.Getenv("SHUTDOWN_TIMEOUT")); raw != "" {
		value, err := time.ParseDuration(raw)
		if err != nil || value <= 0 {
			return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT must be a positive duration")
		}
		cfg.ShutdownTimeout = value
	}
	if raw := strings.TrimSpace(os.Getenv("SESSION_COOKIE_SECURE")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("parse SESSION_COOKIE_SECURE: %w", err)
		}
		cfg.SessionCookieSecure = value
	}
	if cfg.MigrationMode != MigrationAuto && cfg.MigrationMode != MigrationVerify {
		return Config{}, fmt.Errorf("MIGRATION_MODE must be auto or verify")
	}
	if cfg.IdentityProvider != IdentityIAM && cfg.IdentityProvider != IdentityTemplate {
		return Config{}, fmt.Errorf("IDENTITY_PROVIDER must be iam or template")
	}
	if cfg.PublicBasePath != "" && (!strings.HasPrefix(cfg.PublicBasePath, "/") || strings.HasPrefix(cfg.PublicBasePath, "//") || strings.Contains(cfg.PublicBasePath, "\\")) {
		return Config{}, fmt.Errorf("PUBLIC_BASE_PATH must be empty or a single-origin absolute path")
	}
	if cfg.IdentityProvider == IdentityIAM {
		if cfg.IAMIssuer == "" || !uuidPattern.MatchString(cfg.IAMTenantID) || cfg.IAMPublicKeysPath == "" {
			return Config{}, fmt.Errorf("IAM_ISSUER, IAM_TENANT_ID and IAM_PUBLIC_KEYS_PATH are required in iam mode")
		}
		if cfg.TemplateAccessToken != "" {
			return Config{}, fmt.Errorf("template credentials are not allowed in iam mode")
		}
	} else {
		if !uuidPattern.MatchString(cfg.TemplateTenantID) || !uuidPattern.MatchString(cfg.TemplateSubjectID) || cfg.TemplateDisplayName == "" || len(cfg.TemplateAccessToken) < 24 || strings.HasPrefix(cfg.TemplateAccessToken, "replace-with-") {
			return Config{}, fmt.Errorf("template mode requires valid TEMPLATE_TENANT_ID, TEMPLATE_SUBJECT_ID, TEMPLATE_DISPLAY_NAME and a replaced 24+ byte TEMPLATE_ACCESS_TOKEN")
		}
		if cfg.IAMIssuer != "" || cfg.IAMTenantID != "" || cfg.IAMPublicKeysPath != "" {
			return Config{}, fmt.Errorf("IAM settings are not allowed in template mode")
		}
	}
	return cfg, nil
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
