package config

import (
	"fmt"
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
	HTTPAddress                   string
	DatabaseURL                   string
	ShutdownTimeout               time.Duration
	MigrationMode                 MigrationMode
	IdentityProvider              IdentityProvider
	IAMIssuer                     string
	IAMTenantID                   string
	IAMPublicKeysPath             string
	TemplateTenantID              string
	TemplateSubjectID             string
	TemplateDisplayName           string
	TemplateAccessToken           string
	PeopleBaseURL                 string
	PeopleAccessToken             string
	PartyBaseURL                  string
	PartyAccessToken              string
	IAMServiceCredentialURL       string
	WorkloadID                    string
	WorkloadSubjectID             string
	WorkloadKeyFile               string
	WorkloadRefreshBefore         time.Duration
	ContractBaseURL               string
	ContractAccessToken           string
	MoneyBaseURL                  string
	MoneyAccessToken              string
	EvidenceBaseURL               string
	EvidenceAccessToken           string
	DesignBaseURL                 string
	DesignAccessToken             string
	ProductionBaseURL             string
	ProductionAccessToken         string
	BillingBaseURL                string
	BillingAccessToken            string
	BusinessEngagementBaseURL     string
	BusinessEngagementAccessToken string
	ResultsBaseURL                string
	ResultsAccessToken            string
	CostBaseURL                   string
	CostAccessToken               string
	ReferenceTimeout              time.Duration
	ReferenceOAProductionProfile  string
}

func Load() (Config, error) {
	cfg := Config{
		ReferenceOAProductionProfile:  strings.TrimSpace(os.Getenv("REFERENCE_OA_PRODUCTION_PERSONNEL_PROFILE")),
		HTTPAddress:                   env("HTTP_ADDR", ":8085"),
		DatabaseURL:                   env("DATABASE_URL", "postgres://project_workspace:project_workspace_dev@127.0.0.1:55437/project_workspace?sslmode=disable"),
		ShutdownTimeout:               10 * time.Second,
		MigrationMode:                 MigrationMode(strings.ToLower(strings.TrimSpace(env("MIGRATION_MODE", string(MigrationAuto))))),
		IdentityProvider:              IdentityProvider(strings.ToLower(strings.TrimSpace(env("IDENTITY_PROVIDER", string(IdentityTemplate))))),
		IAMIssuer:                     strings.TrimRight(strings.TrimSpace(os.Getenv("IAM_ISSUER")), "/"),
		IAMTenantID:                   strings.ToLower(strings.TrimSpace(os.Getenv("IAM_TENANT_ID"))),
		IAMPublicKeysPath:             strings.TrimSpace(os.Getenv("IAM_PUBLIC_KEYS_PATH")),
		TemplateTenantID:              strings.ToLower(strings.TrimSpace(os.Getenv("TEMPLATE_TENANT_ID"))),
		TemplateSubjectID:             strings.ToLower(strings.TrimSpace(os.Getenv("TEMPLATE_SUBJECT_ID"))),
		TemplateDisplayName:           strings.TrimSpace(os.Getenv("TEMPLATE_DISPLAY_NAME")),
		TemplateAccessToken:           os.Getenv("TEMPLATE_ACCESS_TOKEN"),
		PeopleBaseURL:                 strings.TrimRight(strings.TrimSpace(os.Getenv("PEOPLE_BASE_URL")), "/"),
		PeopleAccessToken:             os.Getenv("PEOPLE_ACCESS_TOKEN"),
		PartyBaseURL:                  strings.TrimRight(strings.TrimSpace(os.Getenv("PARTY_BASE_URL")), "/"),
		PartyAccessToken:              os.Getenv("PARTY_ACCESS_TOKEN"),
		IAMServiceCredentialURL:       strings.TrimRight(strings.TrimSpace(os.Getenv("IAM_SERVICE_CREDENTIAL_URL")), "/"),
		WorkloadID:                    strings.TrimSpace(os.Getenv("WORKLOAD_ID")),
		WorkloadSubjectID:             strings.ToLower(strings.TrimSpace(os.Getenv("WORKLOAD_SUBJECT_ID"))),
		WorkloadKeyFile:               strings.TrimSpace(os.Getenv("WORKLOAD_KEY_FILE")),
		WorkloadRefreshBefore:         15 * time.Second,
		ContractBaseURL:               strings.TrimRight(strings.TrimSpace(os.Getenv("CONTRACT_BASE_URL")), "/"),
		ContractAccessToken:           os.Getenv("CONTRACT_ACCESS_TOKEN"),
		MoneyBaseURL:                  strings.TrimRight(strings.TrimSpace(os.Getenv("MONEY_BASE_URL")), "/"),
		MoneyAccessToken:              os.Getenv("MONEY_ACCESS_TOKEN"),
		EvidenceBaseURL:               strings.TrimRight(strings.TrimSpace(os.Getenv("EVIDENCE_BASE_URL")), "/"),
		EvidenceAccessToken:           os.Getenv("EVIDENCE_ACCESS_TOKEN"),
		DesignBaseURL:                 strings.TrimRight(strings.TrimSpace(os.Getenv("DESIGN_BASE_URL")), "/"),
		DesignAccessToken:             os.Getenv("DESIGN_ACCESS_TOKEN"),
		ProductionBaseURL:             strings.TrimRight(strings.TrimSpace(os.Getenv("PRODUCTION_BASE_URL")), "/"),
		ProductionAccessToken:         os.Getenv("PRODUCTION_ACCESS_TOKEN"),
		BillingBaseURL:                strings.TrimRight(strings.TrimSpace(os.Getenv("BILLING_BASE_URL")), "/"),
		BillingAccessToken:            os.Getenv("BILLING_ACCESS_TOKEN"),
		BusinessEngagementBaseURL:     strings.TrimRight(strings.TrimSpace(os.Getenv("BUSINESS_ENGAGEMENT_BASE_URL")), "/"),
		BusinessEngagementAccessToken: os.Getenv("BUSINESS_ENGAGEMENT_ACCESS_TOKEN"),
		ResultsBaseURL:                strings.TrimRight(strings.TrimSpace(os.Getenv("RESULTS_BASE_URL")), "/"),
		ResultsAccessToken:            os.Getenv("RESULTS_ACCESS_TOKEN"),
		CostBaseURL:                   strings.TrimRight(strings.TrimSpace(os.Getenv("COST_BASE_URL")), "/"),
		CostAccessToken:               os.Getenv("COST_ACCESS_TOKEN"),
		ReferenceTimeout:              3 * time.Second,
	}
	if raw := strings.TrimSpace(os.Getenv("SHUTDOWN_TIMEOUT")); raw != "" {
		value, err := time.ParseDuration(raw)
		if err != nil || value <= 0 {
			return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT must be a positive duration")
		}
		cfg.ShutdownTimeout = value
	}
	if raw := strings.TrimSpace(os.Getenv("REFERENCE_TIMEOUT")); raw != "" {
		value, err := time.ParseDuration(raw)
		if err != nil || value <= 0 || value > 30*time.Second {
			return Config{}, fmt.Errorf("REFERENCE_TIMEOUT must be between 1ns and 30s")
		}
		cfg.ReferenceTimeout = value
	}
	if raw := strings.TrimSpace(os.Getenv("WORKLOAD_REFRESH_BEFORE")); raw != "" {
		value, err := time.ParseDuration(raw)
		if err != nil || value <= 0 || value > time.Minute {
			return Config{}, fmt.Errorf("WORKLOAD_REFRESH_BEFORE must be between 1ns and 1m")
		}
		cfg.WorkloadRefreshBefore = value
	}
	if cfg.MigrationMode != MigrationAuto && cfg.MigrationMode != MigrationVerify {
		return Config{}, fmt.Errorf("MIGRATION_MODE must be auto or verify")
	}
	if cfg.IdentityProvider != IdentityIAM && cfg.IdentityProvider != IdentityTemplate {
		return Config{}, fmt.Errorf("IDENTITY_PROVIDER must be iam or template")
	}
	if cfg.IdentityProvider == IdentityIAM {
		if cfg.IAMIssuer == "" || !uuidPattern.MatchString(cfg.IAMTenantID) || cfg.IAMPublicKeysPath == "" {
			return Config{}, fmt.Errorf("IAM_ISSUER, IAM_TENANT_ID and IAM_PUBLIC_KEYS_PATH are required in iam mode")
		}
		if cfg.TemplateAccessToken != "" {
			return Config{}, fmt.Errorf("template credentials are not allowed in iam mode")
		}
		if cfg.PeopleAccessToken != "" {
			return Config{}, fmt.Errorf("PEOPLE_ACCESS_TOKEN is not allowed in iam mode")
		}
		if cfg.PartyAccessToken != "" {
			return Config{}, fmt.Errorf("PARTY_ACCESS_TOKEN is not allowed in iam mode")
		}
		for _, credential := range []struct {
			name, value string
		}{
			{"CONTRACT_ACCESS_TOKEN", cfg.ContractAccessToken},
			{"MONEY_ACCESS_TOKEN", cfg.MoneyAccessToken},
			{"EVIDENCE_ACCESS_TOKEN", cfg.EvidenceAccessToken},
			{"DESIGN_ACCESS_TOKEN", cfg.DesignAccessToken},
			{"PRODUCTION_ACCESS_TOKEN", cfg.ProductionAccessToken},
			{"BILLING_ACCESS_TOKEN", cfg.BillingAccessToken},
			{"BUSINESS_ENGAGEMENT_ACCESS_TOKEN", cfg.BusinessEngagementAccessToken},
			{"RESULTS_ACCESS_TOKEN", cfg.ResultsAccessToken},
			{"COST_ACCESS_TOKEN", cfg.CostAccessToken},
		} {
			if credential.value != "" {
				return Config{}, fmt.Errorf("%s is not allowed in iam mode", credential.name)
			}
		}
		workloadValues := []string{cfg.IAMServiceCredentialURL, cfg.WorkloadID, cfg.WorkloadSubjectID, cfg.WorkloadKeyFile}
		configured := 0
		for _, value := range workloadValues {
			if value != "" {
				configured++
			}
		}
		hasExternalEndpoint := cfg.PeopleBaseURL != "" || cfg.PartyBaseURL != "" || cfg.ContractBaseURL != "" ||
			cfg.MoneyBaseURL != "" || cfg.EvidenceBaseURL != "" || cfg.DesignBaseURL != "" || cfg.BillingBaseURL != "" ||
			cfg.BusinessEngagementBaseURL != "" || cfg.ResultsBaseURL != "" || cfg.ProductionBaseURL != "" || cfg.CostBaseURL != ""
		if hasExternalEndpoint && configured != len(workloadValues) {
			return Config{}, fmt.Errorf("workload identity settings are required with external reference endpoints in iam mode")
		}
		if configured != 0 && configured != len(workloadValues) {
			return Config{}, fmt.Errorf("workload identity settings must be configured together")
		}
		if configured == len(workloadValues) {
			if !regexp.MustCompile(`^[a-z][a-z0-9-]{2,63}$`).MatchString(cfg.WorkloadID) || !uuidPattern.MatchString(cfg.WorkloadSubjectID) {
				return Config{}, fmt.Errorf("workload identity is invalid")
			}
			if !strings.HasPrefix(cfg.IAMServiceCredentialURL, "http://") && !strings.HasPrefix(cfg.IAMServiceCredentialURL, "https://") {
				return Config{}, fmt.Errorf("IAM_SERVICE_CREDENTIAL_URL must use http or https")
			}
		}
	} else {
		if !uuidPattern.MatchString(cfg.TemplateTenantID) || !uuidPattern.MatchString(cfg.TemplateSubjectID) || cfg.TemplateDisplayName == "" || len(cfg.TemplateAccessToken) < 24 || strings.HasPrefix(cfg.TemplateAccessToken, "replace-with-") {
			return Config{}, fmt.Errorf("template mode requires valid TEMPLATE_TENANT_ID, TEMPLATE_SUBJECT_ID, TEMPLATE_DISPLAY_NAME and a replaced 24+ byte TEMPLATE_ACCESS_TOKEN")
		}
		if cfg.IAMIssuer != "" || cfg.IAMTenantID != "" || cfg.IAMPublicKeysPath != "" {
			return Config{}, fmt.Errorf("IAM settings are not allowed in template mode")
		}
	}
	for _, endpoint := range []struct {
		name, baseURL, token string
	}{
		{"CONTRACT", cfg.ContractBaseURL, cfg.ContractAccessToken},
		{"MONEY", cfg.MoneyBaseURL, cfg.MoneyAccessToken},
		{"EVIDENCE", cfg.EvidenceBaseURL, cfg.EvidenceAccessToken},
		{"DESIGN", cfg.DesignBaseURL, cfg.DesignAccessToken},
		{"PRODUCTION", cfg.ProductionBaseURL, cfg.ProductionAccessToken},
		{"BILLING", cfg.BillingBaseURL, cfg.BillingAccessToken},
		{"BUSINESS_ENGAGEMENT", cfg.BusinessEngagementBaseURL, cfg.BusinessEngagementAccessToken},
		{"RESULTS", cfg.ResultsBaseURL, cfg.ResultsAccessToken},
		{"COST", cfg.CostBaseURL, cfg.CostAccessToken},
	} {
		if cfg.IdentityProvider == IdentityTemplate && (endpoint.baseURL == "") != (endpoint.token == "") {
			return Config{}, fmt.Errorf("%s_BASE_URL and %s_ACCESS_TOKEN must be configured together", endpoint.name, endpoint.name)
		}
		if endpoint.baseURL != "" && !strings.HasPrefix(endpoint.baseURL, "http://") && !strings.HasPrefix(endpoint.baseURL, "https://") {
			return Config{}, fmt.Errorf("%s_BASE_URL must use http or https", endpoint.name)
		}
	}
	if cfg.IdentityProvider == IdentityTemplate && (cfg.PeopleBaseURL == "") != (cfg.PeopleAccessToken == "") {
		return Config{}, fmt.Errorf("PEOPLE_BASE_URL and PEOPLE_ACCESS_TOKEN must be configured together in template mode")
	}
	if cfg.IdentityProvider == IdentityTemplate && (cfg.PartyBaseURL == "") != (cfg.PartyAccessToken == "") {
		return Config{}, fmt.Errorf("PARTY_BASE_URL and PARTY_ACCESS_TOKEN must be configured together in template mode")
	}
	if cfg.PartyBaseURL != "" && !strings.HasPrefix(cfg.PartyBaseURL, "http://") && !strings.HasPrefix(cfg.PartyBaseURL, "https://") {
		return Config{}, fmt.Errorf("PARTY_BASE_URL must use http or https")
	}
	if cfg.ReferenceOAProductionProfile != "" && cfg.ReferenceOAProductionProfile != "reference-oa-production-personnel-v1" {
		return Config{}, fmt.Errorf("REFERENCE_OA_PRODUCTION_PERSONNEL_PROFILE must be empty or reference-oa-production-personnel-v1")
	}
	return cfg, nil
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
