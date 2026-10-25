package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

type Config struct {
	HTTPAddress, DatabaseURL                                                                                                                                                                  string
	ShutdownTimeout                                                                                                                                                                           time.Duration
	MigrationMode, IdentityProvider, IAMIssuer, IAMTenantID, IAMPublicKeysPath, TemplateTenantID, TemplateSubjectID, TemplateDisplayName, TemplateAccessToken, MoneyBaseURL, MoneyAccessToken string
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func Load() (Config, error) {
	c := Config{HTTPAddress: env("HTTP_ADDR", ":8092"), DatabaseURL: env("DATABASE_URL", "postgres://internal_finance_requests:internal_finance_requests_dev@127.0.0.1:55442/internal_finance_requests?sslmode=disable"), ShutdownTimeout: 10 * time.Second, MigrationMode: strings.ToLower(env("MIGRATION_MODE", "auto")), IdentityProvider: strings.ToLower(env("IDENTITY_PROVIDER", "template")), IAMIssuer: strings.TrimRight(strings.TrimSpace(os.Getenv("IAM_ISSUER")), "/"), IAMTenantID: strings.ToLower(strings.TrimSpace(os.Getenv("IAM_TENANT_ID"))), IAMPublicKeysPath: strings.TrimSpace(os.Getenv("IAM_PUBLIC_KEYS_PATH")), TemplateTenantID: strings.ToLower(strings.TrimSpace(os.Getenv("TEMPLATE_TENANT_ID"))), TemplateSubjectID: strings.ToLower(strings.TrimSpace(os.Getenv("TEMPLATE_SUBJECT_ID"))), TemplateDisplayName: strings.TrimSpace(os.Getenv("TEMPLATE_DISPLAY_NAME")), TemplateAccessToken: os.Getenv("TEMPLATE_ACCESS_TOKEN"), MoneyBaseURL: strings.TrimRight(strings.TrimSpace(os.Getenv("MONEY_BASE_URL")), "/"), MoneyAccessToken: os.Getenv("MONEY_ACCESS_TOKEN")}
	if (c.MoneyBaseURL == "") != (c.MoneyAccessToken == "") {
		return Config{}, fmt.Errorf("MONEY_BASE_URL and MONEY_ACCESS_TOKEN must be configured together")
	}
	if c.MigrationMode != "auto" && c.MigrationMode != "verify" {
		return Config{}, fmt.Errorf("MIGRATION_MODE must be auto or verify")
	}
	if c.IdentityProvider == "template" {
		if !uuidPattern.MatchString(c.TemplateTenantID) || !uuidPattern.MatchString(c.TemplateSubjectID) || c.TemplateDisplayName == "" || len(c.TemplateAccessToken) < 24 {
			return Config{}, fmt.Errorf("template mode requires valid template identity and a 24+ byte token")
		}
		return c, nil
	}
	if c.IdentityProvider != "iam" {
		return Config{}, fmt.Errorf("IDENTITY_PROVIDER must be iam or template")
	}
	parsed, err := url.Parse(c.IAMIssuer)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || !uuidPattern.MatchString(c.IAMTenantID) || c.IAMPublicKeysPath == "" {
		return Config{}, fmt.Errorf("iam mode requires HTTPS issuer, tenant and public keys")
	}
	return c, nil
}
func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
