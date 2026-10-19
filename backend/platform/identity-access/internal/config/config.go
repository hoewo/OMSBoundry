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

type MigrationMode string

const (
	MigrationModeAuto   MigrationMode = "auto"
	MigrationModeVerify MigrationMode = "verify"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type Config struct {
	HTTPAddress                  string
	DatabaseURL                  string
	MigrationMode                MigrationMode
	ShutdownPeriod               time.Duration
	SessionTTL                   time.Duration
	CredentialTTL                time.Duration
	SessionCookieSecure          bool
	TenantID                     string
	TenantName                   string
	PublicIssuer                 string
	SigningKeyFile               string
	BootstrapAdminUsername       string
	BootstrapAdminDisplayName    string
	BootstrapAdminPassword       string
	CapabilityManifestPaths      []string
	ProjectPolicyBaseURL         string
	PeoplePolicyBaseURL          string
	PolicyServiceSubjectID       string
	ProjectAssignmentPolicyPaths []string
	PolicyTimeout                time.Duration
	WorkloadIdentityPolicyPath   string
	WorkloadCredentialTTL        time.Duration
}

func Load() (Config, error) {
	config := Config{
		HTTPAddress:                  envOrDefault("HTTP_ADDR", ":8080"),
		DatabaseURL:                  envOrDefault("DATABASE_URL", "postgres://identity_access:identity_access_dev@127.0.0.1:55434/identity_access?sslmode=disable"),
		MigrationMode:                MigrationMode(strings.ToLower(strings.TrimSpace(envOrDefault("MIGRATION_MODE", "auto")))),
		ShutdownPeriod:               10 * time.Second,
		SessionTTL:                   12 * time.Hour,
		CredentialTTL:                5 * time.Minute,
		TenantID:                     strings.ToLower(strings.TrimSpace(os.Getenv("TENANT_ID"))),
		TenantName:                   strings.TrimSpace(os.Getenv("TENANT_NAME")),
		PublicIssuer:                 strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_ISSUER")), "/"),
		SigningKeyFile:               envOrDefault("SIGNING_KEY_FILE", ".data/identity-access/signing-keys.json"),
		BootstrapAdminUsername:       strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_USERNAME")),
		BootstrapAdminDisplayName:    strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_DISPLAY_NAME")),
		BootstrapAdminPassword:       os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"),
		CapabilityManifestPaths:      splitList(os.Getenv("CAPABILITY_MANIFEST_PATHS")),
		ProjectPolicyBaseURL:         strings.TrimRight(strings.TrimSpace(os.Getenv("PROJECT_POLICY_BASE_URL")), "/"),
		PeoplePolicyBaseURL:          strings.TrimRight(strings.TrimSpace(os.Getenv("PEOPLE_POLICY_BASE_URL")), "/"),
		PolicyServiceSubjectID:       strings.ToLower(strings.TrimSpace(os.Getenv("POLICY_SERVICE_SUBJECT_ID"))),
		ProjectAssignmentPolicyPaths: splitList(os.Getenv("PROJECT_ASSIGNMENT_POLICY_PATHS")),
		PolicyTimeout:                3 * time.Second,
		WorkloadIdentityPolicyPath:   strings.TrimSpace(os.Getenv("WORKLOAD_IDENTITY_POLICY_PATH")),
		WorkloadCredentialTTL:        time.Minute,
	}
	if len(config.ProjectAssignmentPolicyPaths) == 0 {
		config.ProjectAssignmentPolicyPaths = splitList(os.Getenv("PROJECT_ASSIGNMENT_POLICY_PATH"))
	}
	var err error
	if config.ShutdownPeriod, err = durationEnv("SHUTDOWN_TIMEOUT", config.ShutdownPeriod); err != nil {
		return Config{}, err
	}
	if config.SessionTTL, err = durationEnv("SESSION_TTL", config.SessionTTL); err != nil {
		return Config{}, err
	}
	if config.CredentialTTL, err = durationEnv("CREDENTIAL_TTL", config.CredentialTTL); err != nil {
		return Config{}, err
	}
	if config.PolicyTimeout, err = durationEnv("POLICY_TIMEOUT", config.PolicyTimeout); err != nil {
		return Config{}, err
	}
	if config.WorkloadCredentialTTL, err = durationEnv("WORKLOAD_CREDENTIAL_TTL", config.WorkloadCredentialTTL); err != nil {
		return Config{}, err
	}
	if config.PolicyTimeout > 30*time.Second {
		return Config{}, fmt.Errorf("POLICY_TIMEOUT cannot exceed 30s")
	}
	if config.CredentialTTL > 5*time.Minute {
		return Config{}, fmt.Errorf("CREDENTIAL_TTL cannot exceed 5m")
	}
	if config.WorkloadCredentialTTL > time.Minute {
		return Config{}, fmt.Errorf("WORKLOAD_CREDENTIAL_TTL cannot exceed 1m")
	}
	if raw := os.Getenv("SESSION_COOKIE_SECURE"); raw != "" {
		config.SessionCookieSecure, err = strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("parse SESSION_COOKIE_SECURE: %w", err)
		}
	}
	if config.MigrationMode != MigrationModeAuto && config.MigrationMode != MigrationModeVerify {
		return Config{}, fmt.Errorf("MIGRATION_MODE must be auto or verify")
	}
	if !uuidPattern.MatchString(config.TenantID) {
		return Config{}, fmt.Errorf("TENANT_ID must be a lowercase RFC 4122 UUID")
	}
	if config.TenantName == "" || len([]rune(config.TenantName)) > 100 {
		return Config{}, fmt.Errorf("TENANT_NAME must contain 1 to 100 characters")
	}
	issuer, parseErr := url.Parse(config.PublicIssuer)
	if parseErr != nil || issuer.Scheme != "https" || issuer.Host == "" || issuer.RawQuery != "" || issuer.Fragment != "" {
		return Config{}, fmt.Errorf("PUBLIC_ISSUER must be an absolute HTTPS URL without query or fragment")
	}
	if config.SigningKeyFile == "" {
		return Config{}, fmt.Errorf("SIGNING_KEY_FILE is required")
	}
	if (config.BootstrapAdminUsername == "") != (config.BootstrapAdminPassword == "") {
		return Config{}, fmt.Errorf("BOOTSTRAP_ADMIN_USERNAME and BOOTSTRAP_ADMIN_PASSWORD must be set together")
	}
	policyValues := []string{config.ProjectPolicyBaseURL, config.PeoplePolicyBaseURL, config.PolicyServiceSubjectID}
	configured := 0
	for _, value := range policyValues {
		if value != "" {
			configured++
		}
	}
	if len(config.ProjectAssignmentPolicyPaths) > 0 {
		configured++
	}
	if configured != 0 && configured != len(policyValues)+1 {
		return Config{}, fmt.Errorf("PROJECT_POLICY_BASE_URL, PEOPLE_POLICY_BASE_URL, POLICY_SERVICE_SUBJECT_ID and PROJECT_ASSIGNMENT_POLICY_PATHS must be configured together")
	}
	if configured == len(policyValues)+1 {
		if !uuidPattern.MatchString(config.PolicyServiceSubjectID) {
			return Config{}, fmt.Errorf("POLICY_SERVICE_SUBJECT_ID must be a lowercase RFC 4122 UUID")
		}
		for name, raw := range map[string]string{"PROJECT_POLICY_BASE_URL": config.ProjectPolicyBaseURL, "PEOPLE_POLICY_BASE_URL": config.PeoplePolicyBaseURL} {
			parsed, parseErr := url.Parse(raw)
			if parseErr != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
				return Config{}, fmt.Errorf("%s must be an absolute http or https URL without query or fragment", name)
			}
		}
	}
	return config, nil
}

func (c Config) PolicyEnabled() bool { return len(c.ProjectAssignmentPolicyPaths) > 0 }

func splitList(raw string) []string {
	values := make([]string, 0)
	for _, value := range strings.Split(raw, ",") {
		if value = strings.TrimSpace(value); value != "" {
			values = append(values, value)
		}
	}
	return values
}

func durationEnv(name string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return value, nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
