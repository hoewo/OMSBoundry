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
var identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{2,63}$`)

type Config struct {
	HTTPAddress, DatabaseURL                                                      string
	ShutdownTimeout, ReferenceTimeout                                             time.Duration
	MigrationMode                                                                 MigrationMode
	IdentityProvider                                                              IdentityProvider
	IAMIssuer, IAMTenantID, IAMPublicKeysPath                                     string
	TemplateTenantID, TemplateSubjectID, TemplateDisplayName, TemplateAccessToken string
	ProjectBaseURL, ProjectAccessToken                                            string
	ContractBaseURL, ContractAccessToken                                          string
	EvidenceBaseURL, EvidenceAccessToken                                          string
	IAMServiceCredentialURL, WorkloadID, WorkloadSubjectID, WorkloadKeyFile       string
	WorkloadRefreshBefore                                                         time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddress: env("HTTP_ADDR", ":8095"), DatabaseURL: env("DATABASE_URL", "postgres://approval_audit_results:approval_audit_results_dev@127.0.0.1:55445/approval_audit_results?sslmode=disable"),
		ShutdownTimeout: 10 * time.Second, ReferenceTimeout: 3 * time.Second, WorkloadRefreshBefore: 15 * time.Second,
		MigrationMode: MigrationMode(strings.ToLower(strings.TrimSpace(env("MIGRATION_MODE", string(MigrationAuto))))), IdentityProvider: IdentityProvider(strings.ToLower(strings.TrimSpace(env("IDENTITY_PROVIDER", string(IdentityTemplate))))),
		IAMIssuer: strings.TrimRight(strings.TrimSpace(os.Getenv("IAM_ISSUER")), "/"), IAMTenantID: strings.ToLower(strings.TrimSpace(os.Getenv("IAM_TENANT_ID"))), IAMPublicKeysPath: strings.TrimSpace(os.Getenv("IAM_PUBLIC_KEYS_PATH")),
		TemplateTenantID: strings.ToLower(strings.TrimSpace(os.Getenv("TEMPLATE_TENANT_ID"))), TemplateSubjectID: strings.ToLower(strings.TrimSpace(os.Getenv("TEMPLATE_SUBJECT_ID"))), TemplateDisplayName: strings.TrimSpace(os.Getenv("TEMPLATE_DISPLAY_NAME")), TemplateAccessToken: os.Getenv("TEMPLATE_ACCESS_TOKEN"),
		ProjectBaseURL: cleanURL(os.Getenv("PROJECT_BASE_URL")), ProjectAccessToken: os.Getenv("PROJECT_ACCESS_TOKEN"), ContractBaseURL: cleanURL(os.Getenv("CONTRACT_BASE_URL")), ContractAccessToken: os.Getenv("CONTRACT_ACCESS_TOKEN"), EvidenceBaseURL: cleanURL(os.Getenv("EVIDENCE_BASE_URL")), EvidenceAccessToken: os.Getenv("EVIDENCE_ACCESS_TOKEN"),
		IAMServiceCredentialURL: cleanURL(os.Getenv("IAM_SERVICE_CREDENTIAL_URL")), WorkloadID: strings.TrimSpace(os.Getenv("WORKLOAD_ID")), WorkloadSubjectID: strings.ToLower(strings.TrimSpace(os.Getenv("WORKLOAD_SUBJECT_ID"))), WorkloadKeyFile: strings.TrimSpace(os.Getenv("WORKLOAD_KEY_FILE")),
	}
	if err := loadDurations(&cfg); err != nil {
		return Config{}, err
	}
	if cfg.MigrationMode != MigrationAuto && cfg.MigrationMode != MigrationVerify {
		return Config{}, fmt.Errorf("MIGRATION_MODE must be auto or verify")
	}
	if cfg.IdentityProvider != IdentityIAM && cfg.IdentityProvider != IdentityTemplate {
		return Config{}, fmt.Errorf("IDENTITY_PROVIDER must be iam or template")
	}
	for name, raw := range map[string]string{"PROJECT_BASE_URL": cfg.ProjectBaseURL, "CONTRACT_BASE_URL": cfg.ContractBaseURL, "EVIDENCE_BASE_URL": cfg.EvidenceBaseURL} {
		if !validHTTPURL(raw) {
			return Config{}, fmt.Errorf("%s must be an absolute http or https URL", name)
		}
	}
	if cfg.IdentityProvider == IdentityTemplate {
		if !uuidPattern.MatchString(cfg.TemplateTenantID) || !uuidPattern.MatchString(cfg.TemplateSubjectID) || cfg.TemplateDisplayName == "" || !validToken(cfg.TemplateAccessToken) {
			return Config{}, fmt.Errorf("template mode requires valid template identity and a replaced 24+ byte access token")
		}
		if cfg.IAMIssuer != "" || cfg.IAMTenantID != "" || cfg.IAMPublicKeysPath != "" || cfg.IAMServiceCredentialURL != "" || cfg.WorkloadID != "" || cfg.WorkloadSubjectID != "" || cfg.WorkloadKeyFile != "" {
			return Config{}, fmt.Errorf("IAM and workload identity settings are not allowed in template mode")
		}
		for name, token := range map[string]string{"PROJECT_ACCESS_TOKEN": cfg.ProjectAccessToken, "CONTRACT_ACCESS_TOKEN": cfg.ContractAccessToken, "EVIDENCE_ACCESS_TOKEN": cfg.EvidenceAccessToken} {
			if !validToken(token) {
				return Config{}, fmt.Errorf("template mode requires replaced 24+ byte %s", name)
			}
		}
		return cfg, nil
	}
	if !validHTTPSURL(cfg.IAMIssuer) || !uuidPattern.MatchString(cfg.IAMTenantID) || cfg.IAMPublicKeysPath == "" {
		return Config{}, fmt.Errorf("iam mode requires HTTPS IAM_ISSUER, valid IAM_TENANT_ID and IAM_PUBLIC_KEYS_PATH")
	}
	if cfg.TemplateTenantID != "" || cfg.TemplateSubjectID != "" || cfg.TemplateDisplayName != "" || cfg.TemplateAccessToken != "" || cfg.ProjectAccessToken != "" || cfg.ContractAccessToken != "" || cfg.EvidenceAccessToken != "" {
		return Config{}, fmt.Errorf("template and static reference credentials are not allowed in iam mode")
	}
	if !validHTTPURL(cfg.IAMServiceCredentialURL) || !identifierPattern.MatchString(cfg.WorkloadID) || !uuidPattern.MatchString(cfg.WorkloadSubjectID) || cfg.WorkloadKeyFile == "" {
		return Config{}, fmt.Errorf("iam mode requires IAM_SERVICE_CREDENTIAL_URL, valid WORKLOAD_ID, WORKLOAD_SUBJECT_ID and WORKLOAD_KEY_FILE")
	}
	return cfg, nil
}

func loadDurations(cfg *Config) error {
	for _, item := range []struct {
		name  string
		value *time.Duration
		max   time.Duration
	}{
		{"SHUTDOWN_TIMEOUT", &cfg.ShutdownTimeout, time.Minute}, {"REFERENCE_TIMEOUT", &cfg.ReferenceTimeout, 30 * time.Second}, {"WORKLOAD_REFRESH_BEFORE", &cfg.WorkloadRefreshBefore, time.Minute},
	} {
		if raw := strings.TrimSpace(os.Getenv(item.name)); raw != "" {
			value, err := time.ParseDuration(raw)
			if err != nil || value <= 0 || value > item.max {
				return fmt.Errorf("%s must be a positive duration no greater than %s", item.name, item.max)
			}
			*item.value = value
		}
	}
	return nil
}

func validToken(value string) bool {
	return len(value) >= 24 && !strings.HasPrefix(value, "replace-with-")
}
func cleanURL(value string) string { return strings.TrimRight(strings.TrimSpace(value), "/") }
func validHTTPURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}
func validHTTPSURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}
func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
