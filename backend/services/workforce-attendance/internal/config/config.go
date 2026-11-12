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
	c := Config{
		HTTPAddress: env("HTTP_ADDR", ":8096"), DatabaseURL: env("DATABASE_URL", "postgres://workforce_attendance:workforce_attendance_dev@127.0.0.1:55446/workforce_attendance?sslmode=disable"),
		ShutdownTimeout: 10 * time.Second, ReferenceTimeout: 3 * time.Second, WorkloadRefreshBefore: 15 * time.Second,
		MigrationMode: strings.ToLower(env("MIGRATION_MODE", "auto")), IdentityProvider: strings.ToLower(env("IDENTITY_PROVIDER", "template")),
		IAMIssuer: strings.TrimRight(strings.TrimSpace(os.Getenv("IAM_ISSUER")), "/"), IAMTenantID: clean(os.Getenv("IAM_TENANT_ID")), IAMPublicKeysPath: strings.TrimSpace(os.Getenv("IAM_PUBLIC_KEYS_PATH")),
		TemplateTenantID: clean(os.Getenv("TEMPLATE_TENANT_ID")), TemplateSubjectID: clean(os.Getenv("TEMPLATE_SUBJECT_ID")), TemplateDisplayName: strings.TrimSpace(os.Getenv("TEMPLATE_DISPLAY_NAME")), TemplateAccessToken: os.Getenv("TEMPLATE_ACCESS_TOKEN"),
		PeopleBaseURL: strings.TrimRight(strings.TrimSpace(os.Getenv("PEOPLE_BASE_URL")), "/"), PeopleAccessToken: os.Getenv("PEOPLE_ACCESS_TOKEN"), IAMServiceCredentialURL: strings.TrimRight(strings.TrimSpace(os.Getenv("IAM_SERVICE_CREDENTIAL_URL")), "/"),
		WorkloadID: strings.TrimSpace(os.Getenv("WORKLOAD_ID")), WorkloadSubjectID: clean(os.Getenv("WORKLOAD_SUBJECT_ID")), WorkloadKeyFile: strings.TrimSpace(os.Getenv("WORKLOAD_KEY_FILE")),
	}
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
		if !uuidPattern.MatchString(c.TemplateTenantID) || !uuidPattern.MatchString(c.TemplateSubjectID) || c.TemplateDisplayName == "" || len(c.TemplateAccessToken) < 24 || !validURL(c.PeopleBaseURL) || len(c.PeopleAccessToken) < 24 {
			return Config{}, fmt.Errorf("template mode requires identity and People employment reference settings")
		}
		return c, nil
	}
	if c.IdentityProvider != "iam" {
		return Config{}, fmt.Errorf("IDENTITY_PROVIDER must be iam or template")
	}
	issuer, err := url.Parse(c.IAMIssuer)
	if err != nil || issuer.Scheme != "https" || issuer.Host == "" || !uuidPattern.MatchString(c.IAMTenantID) || c.IAMPublicKeysPath == "" || c.PeopleAccessToken != "" || !validURL(c.PeopleBaseURL) || !validURL(c.IAMServiceCredentialURL) || c.WorkloadID != "workforce-attendance" || !uuidPattern.MatchString(c.WorkloadSubjectID) || c.WorkloadKeyFile == "" {
		return Config{}, fmt.Errorf("IAM mode requires issuer, keys and complete workload identity settings")
	}
	return c, nil
}

func validURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}
func clean(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
