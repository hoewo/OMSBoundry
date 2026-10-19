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

var (
	uuidPattern       = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{2,63}$`)
)

type Config struct {
	HTTPAddress, DatabaseURL                                                      string
	ShutdownTimeout                                                               time.Duration
	MigrationMode                                                                 MigrationMode
	IdentityProvider                                                              IdentityProvider
	IAMIssuer, IAMTenantID, IAMPublicKeysPath                                     string
	TemplateTenantID, TemplateSubjectID, TemplateDisplayName, TemplateAccessToken string
	ProjectBaseURL, ProjectAccessToken, PeopleBaseURL, PeopleAccessToken          string
	EvidenceBaseURL, EvidenceAccessToken                                          string
	ReferenceTimeout                                                              time.Duration
	IAMServiceCredentialURL, WorkloadID, WorkloadSubjectID, WorkloadKeyFile       string
	WorkloadRefreshBefore                                                         time.Duration
	ReferenceStageMaterialProfile                                                 string
}

func Load() (Config, error) {
	c := Config{
		HTTPAddress:                   env("HTTP_ADDR", ":8087"),
		DatabaseURL:                   env("DATABASE_URL", "postgres://design_operations:design_operations_dev@127.0.0.1:55438/design_operations?sslmode=disable"),
		ShutdownTimeout:               10 * time.Second,
		MigrationMode:                 MigrationMode(strings.ToLower(strings.TrimSpace(env("MIGRATION_MODE", "auto")))),
		IdentityProvider:              IdentityProvider(strings.ToLower(strings.TrimSpace(env("IDENTITY_PROVIDER", "template")))),
		IAMIssuer:                     strings.TrimRight(strings.TrimSpace(os.Getenv("IAM_ISSUER")), "/"),
		IAMTenantID:                   strings.ToLower(strings.TrimSpace(os.Getenv("IAM_TENANT_ID"))),
		IAMPublicKeysPath:             strings.TrimSpace(os.Getenv("IAM_PUBLIC_KEYS_PATH")),
		TemplateTenantID:              strings.ToLower(strings.TrimSpace(os.Getenv("TEMPLATE_TENANT_ID"))),
		TemplateSubjectID:             strings.ToLower(strings.TrimSpace(os.Getenv("TEMPLATE_SUBJECT_ID"))),
		TemplateDisplayName:           strings.TrimSpace(os.Getenv("TEMPLATE_DISPLAY_NAME")),
		TemplateAccessToken:           os.Getenv("TEMPLATE_ACCESS_TOKEN"),
		ProjectBaseURL:                strings.TrimRight(strings.TrimSpace(os.Getenv("PROJECT_BASE_URL")), "/"),
		ProjectAccessToken:            os.Getenv("PROJECT_ACCESS_TOKEN"),
		PeopleBaseURL:                 strings.TrimRight(strings.TrimSpace(os.Getenv("PEOPLE_BASE_URL")), "/"),
		PeopleAccessToken:             os.Getenv("PEOPLE_ACCESS_TOKEN"),
		EvidenceBaseURL:               strings.TrimRight(strings.TrimSpace(os.Getenv("EVIDENCE_BASE_URL")), "/"),
		EvidenceAccessToken:           os.Getenv("EVIDENCE_ACCESS_TOKEN"),
		ReferenceTimeout:              3 * time.Second,
		IAMServiceCredentialURL:       strings.TrimRight(strings.TrimSpace(os.Getenv("IAM_SERVICE_CREDENTIAL_URL")), "/"),
		WorkloadID:                    strings.TrimSpace(os.Getenv("WORKLOAD_ID")),
		WorkloadSubjectID:             strings.ToLower(strings.TrimSpace(os.Getenv("WORKLOAD_SUBJECT_ID"))),
		WorkloadKeyFile:               strings.TrimSpace(os.Getenv("WORKLOAD_KEY_FILE")),
		WorkloadRefreshBefore:         15 * time.Second,
		ReferenceStageMaterialProfile: strings.TrimSpace(os.Getenv("REFERENCE_STAGE_MATERIAL_PROFILE")),
	}
	if raw := strings.TrimSpace(os.Getenv("SHUTDOWN_TIMEOUT")); raw != "" {
		v, err := time.ParseDuration(raw)
		if err != nil || v <= 0 {
			return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT must be positive")
		}
		c.ShutdownTimeout = v
	}
	if raw := strings.TrimSpace(os.Getenv("REFERENCE_TIMEOUT")); raw != "" {
		v, err := time.ParseDuration(raw)
		if err != nil || v <= 0 || v > 30*time.Second {
			return Config{}, fmt.Errorf("REFERENCE_TIMEOUT must be between 1ns and 30s")
		}
		c.ReferenceTimeout = v
	}
	if raw := strings.TrimSpace(os.Getenv("WORKLOAD_REFRESH_BEFORE")); raw != "" {
		v, err := time.ParseDuration(raw)
		if err != nil || v <= 0 || v > time.Minute {
			return Config{}, fmt.Errorf("WORKLOAD_REFRESH_BEFORE must be a positive duration no greater than 1m0s")
		}
		c.WorkloadRefreshBefore = v
	}
	if c.MigrationMode != MigrationAuto && c.MigrationMode != MigrationVerify {
		return Config{}, fmt.Errorf("MIGRATION_MODE must be auto or verify")
	}
	if c.ReferenceStageMaterialProfile != "" && c.ReferenceStageMaterialProfile != "reference-oa-stage-materials-v1" {
		return Config{}, fmt.Errorf("invalid REFERENCE_STAGE_MATERIAL_PROFILE")
	}
	if c.IdentityProvider != IdentityIAM && c.IdentityProvider != IdentityTemplate {
		return Config{}, fmt.Errorf("IDENTITY_PROVIDER must be iam or template")
	}
	if c.IdentityProvider == IdentityIAM {
		if !validHTTPSURL(c.IAMIssuer) || !uuidPattern.MatchString(c.IAMTenantID) || c.IAMPublicKeysPath == "" {
			return Config{}, fmt.Errorf("iam mode requires HTTPS IAM_ISSUER, valid IAM_TENANT_ID and IAM_PUBLIC_KEYS_PATH")
		}
		if c.TemplateTenantID != "" || c.TemplateSubjectID != "" || c.TemplateDisplayName != "" || c.TemplateAccessToken != "" || c.ProjectAccessToken != "" || c.PeopleAccessToken != "" || c.EvidenceAccessToken != "" {
			return Config{}, fmt.Errorf("template and static reference credentials are not allowed in iam mode")
		}
		if !validHTTPURL(c.IAMServiceCredentialURL) || !identifierPattern.MatchString(c.WorkloadID) || !uuidPattern.MatchString(c.WorkloadSubjectID) || c.WorkloadKeyFile == "" {
			return Config{}, fmt.Errorf("iam mode requires IAM_SERVICE_CREDENTIAL_URL, valid WORKLOAD_ID, WORKLOAD_SUBJECT_ID and WORKLOAD_KEY_FILE")
		}
	} else {
		if !uuidPattern.MatchString(c.TemplateTenantID) || !uuidPattern.MatchString(c.TemplateSubjectID) || c.TemplateDisplayName == "" || len(c.TemplateAccessToken) < 24 || strings.HasPrefix(c.TemplateAccessToken, "replace-with-") {
			return Config{}, fmt.Errorf("template mode requires valid template identity and replaced 24+ byte token")
		}
		if c.IAMIssuer != "" || c.IAMTenantID != "" || c.IAMPublicKeysPath != "" || c.IAMServiceCredentialURL != "" || c.WorkloadID != "" || c.WorkloadSubjectID != "" || c.WorkloadKeyFile != "" {
			return Config{}, fmt.Errorf("IAM and workload identity settings are not allowed in template mode")
		}
	}
	if !validHTTPURL(c.ProjectBaseURL) || !validHTTPURL(c.PeopleBaseURL) || !validHTTPURL(c.EvidenceBaseURL) {
		return Config{}, fmt.Errorf("PROJECT_BASE_URL, PEOPLE_BASE_URL and EVIDENCE_BASE_URL must be absolute http or https URLs")
	}
	if c.IdentityProvider == IdentityTemplate {
		for _, v := range []struct{ name, token string }{{"PROJECT", c.ProjectAccessToken}, {"PEOPLE", c.PeopleAccessToken}, {"EVIDENCE", c.EvidenceAccessToken}} {
			if len(v.token) < 24 || strings.HasPrefix(v.token, "replace-with-") {
				return Config{}, fmt.Errorf("template mode requires replaced 24+ byte %s_ACCESS_TOKEN", v.name)
			}
		}
	}
	return c, nil
}

func validHTTPURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func validHTTPSURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
