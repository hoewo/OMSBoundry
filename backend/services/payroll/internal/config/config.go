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
	HTTPAddress, DatabaseURL                                                                                                                                  string
	ShutdownTimeout                                                                                                                                           time.Duration
	MigrationMode, IdentityProvider, IAMIssuer, IAMTenantID, IAMPublicKeysPath, TemplateTenantID, TemplateSubjectID, TemplateDisplayName, TemplateAccessToken string
	PeopleBaseURL, PeopleAccessToken, IAMServiceCredentialURL, WorkloadID, WorkloadSubjectID, WorkloadKeyFile                                                 string
	ReferenceTimeout, WorkloadRefreshBefore                                                                                                                   time.Duration
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func Load() (Config, error) {
	c := Config{HTTPAddress: env("HTTP_ADDR", ":8093"), DatabaseURL: env("DATABASE_URL", "postgres://payroll:payroll_dev@127.0.0.1:55443/payroll?sslmode=disable"), ShutdownTimeout: 10 * time.Second, ReferenceTimeout: 3 * time.Second, WorkloadRefreshBefore: 15 * time.Second, MigrationMode: strings.ToLower(env("MIGRATION_MODE", "auto")), IdentityProvider: strings.ToLower(env("IDENTITY_PROVIDER", "template")), IAMIssuer: strings.TrimRight(strings.TrimSpace(os.Getenv("IAM_ISSUER")), "/"), IAMTenantID: strings.ToLower(strings.TrimSpace(os.Getenv("IAM_TENANT_ID"))), IAMPublicKeysPath: strings.TrimSpace(os.Getenv("IAM_PUBLIC_KEYS_PATH")), TemplateTenantID: strings.ToLower(strings.TrimSpace(os.Getenv("TEMPLATE_TENANT_ID"))), TemplateSubjectID: strings.ToLower(strings.TrimSpace(os.Getenv("TEMPLATE_SUBJECT_ID"))), TemplateDisplayName: strings.TrimSpace(os.Getenv("TEMPLATE_DISPLAY_NAME")), TemplateAccessToken: os.Getenv("TEMPLATE_ACCESS_TOKEN"), PeopleBaseURL: strings.TrimRight(strings.TrimSpace(os.Getenv("PEOPLE_BASE_URL")), "/"), PeopleAccessToken: os.Getenv("PEOPLE_ACCESS_TOKEN"), IAMServiceCredentialURL: strings.TrimRight(strings.TrimSpace(os.Getenv("IAM_SERVICE_CREDENTIAL_URL")), "/"), WorkloadID: strings.TrimSpace(os.Getenv("WORKLOAD_ID")), WorkloadSubjectID: strings.ToLower(strings.TrimSpace(os.Getenv("WORKLOAD_SUBJECT_ID"))), WorkloadKeyFile: strings.TrimSpace(os.Getenv("WORKLOAD_KEY_FILE"))}
	for name, item := range map[string]struct {
		value *time.Duration
		max   time.Duration
	}{"REFERENCE_TIMEOUT": {&c.ReferenceTimeout, 30 * time.Second}, "WORKLOAD_REFRESH_BEFORE": {&c.WorkloadRefreshBefore, time.Minute}} {
		if raw := strings.TrimSpace(os.Getenv(name)); raw != "" {
			value, err := time.ParseDuration(raw)
			if err != nil || value <= 0 || value > item.max {
				return Config{}, fmt.Errorf("%s is invalid", name)
			}
			*item.value = value
		}
	}
	if c.MigrationMode != "auto" && c.MigrationMode != "verify" {
		return Config{}, fmt.Errorf("MIGRATION_MODE must be auto or verify")
	}
	if c.IdentityProvider == "template" {
		if !uuidPattern.MatchString(c.TemplateTenantID) || !uuidPattern.MatchString(c.TemplateSubjectID) || c.TemplateDisplayName == "" || len(c.TemplateAccessToken) < 24 {
			return Config{}, fmt.Errorf("template mode requires valid template identity and a 24+ byte token")
		}
		if (c.PeopleBaseURL == "") != (c.PeopleAccessToken == "") || (c.PeopleBaseURL != "" && (!validURL(c.PeopleBaseURL) || len(c.PeopleAccessToken) < 24)) {
			return Config{}, fmt.Errorf("template People reference endpoint and token must be configured together")
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
	configured := c.PeopleBaseURL != "" || c.IAMServiceCredentialURL != "" || c.WorkloadID != "" || c.WorkloadSubjectID != "" || c.WorkloadKeyFile != ""
	if configured && (!validURL(c.PeopleBaseURL) || !validURL(c.IAMServiceCredentialURL) || c.WorkloadID != "payroll" || !uuidPattern.MatchString(c.WorkloadSubjectID) || c.WorkloadKeyFile == "") {
		return Config{}, fmt.Errorf("IAM People reference settings must be complete")
	}
	return c, nil
}
func validURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}
func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
