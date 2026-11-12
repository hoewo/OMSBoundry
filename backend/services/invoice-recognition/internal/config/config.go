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

type Config struct {
	HTTPAddress, DatabaseURL, MigrationMode, IdentityProvider, IAMIssuer, IAMTenantID, IAMPublicKeysPath          string
	TemplateTenantID, TemplateSubjectID, TemplateDisplayName, TemplateAccessToken                                 string
	EvidenceBaseURL, EvidenceAccessToken, IAMServiceCredentialURL, WorkloadID, WorkloadSubjectID, WorkloadKeyFile string
	AIProviderURL, AIProviderAPIKey, AIProviderModel                                                              string
	LocalOCREnabled                                                                                               bool
	LocalOCRCommand, PDFRasterizerCommand                                                                         string
	ShutdownTimeout, ReferenceTimeout, AIProviderTimeout, LocalOCRTimeout, WorkloadRefreshBefore                  time.Duration
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func Load() (Config, error) {
	c := Config{HTTPAddress: env("HTTP_ADDR", ":8097"), DatabaseURL: env("DATABASE_URL", "postgres://invoice_recognition:invoice_recognition_dev@127.0.0.1:55447/invoice_recognition?sslmode=disable"), MigrationMode: strings.ToLower(env("MIGRATION_MODE", "auto")), IdentityProvider: strings.ToLower(env("IDENTITY_PROVIDER", "template")), IAMIssuer: strings.TrimRight(strings.TrimSpace(os.Getenv("IAM_ISSUER")), "/"), IAMTenantID: clean(os.Getenv("IAM_TENANT_ID")), IAMPublicKeysPath: strings.TrimSpace(os.Getenv("IAM_PUBLIC_KEYS_PATH")), TemplateTenantID: clean(os.Getenv("TEMPLATE_TENANT_ID")), TemplateSubjectID: clean(os.Getenv("TEMPLATE_SUBJECT_ID")), TemplateDisplayName: strings.TrimSpace(os.Getenv("TEMPLATE_DISPLAY_NAME")), TemplateAccessToken: os.Getenv("TEMPLATE_ACCESS_TOKEN"), EvidenceBaseURL: strings.TrimRight(strings.TrimSpace(os.Getenv("EVIDENCE_BASE_URL")), "/"), EvidenceAccessToken: os.Getenv("EVIDENCE_ACCESS_TOKEN"), IAMServiceCredentialURL: strings.TrimRight(strings.TrimSpace(os.Getenv("IAM_SERVICE_CREDENTIAL_URL")), "/"), WorkloadID: strings.TrimSpace(os.Getenv("WORKLOAD_ID")), WorkloadSubjectID: clean(os.Getenv("WORKLOAD_SUBJECT_ID")), WorkloadKeyFile: strings.TrimSpace(os.Getenv("WORKLOAD_KEY_FILE")), AIProviderURL: strings.TrimSpace(os.Getenv("INVOICE_AI_PROVIDER_URL")), AIProviderAPIKey: os.Getenv("INVOICE_AI_PROVIDER_API_KEY"), AIProviderModel: strings.TrimSpace(os.Getenv("INVOICE_AI_PROVIDER_MODEL")), LocalOCREnabled: true, LocalOCRCommand: env("INVOICE_LOCAL_OCR_COMMAND", "tesseract"), PDFRasterizerCommand: env("INVOICE_PDF_RASTERIZER_COMMAND", "pdftoppm"), ShutdownTimeout: 10 * time.Second, ReferenceTimeout: 5 * time.Second, AIProviderTimeout: 90 * time.Second, LocalOCRTimeout: 90 * time.Second, WorkloadRefreshBefore: 15 * time.Second}
	var err error
	if raw := strings.TrimSpace(os.Getenv("INVOICE_LOCAL_OCR_ENABLED")); raw != "" {
		c.LocalOCREnabled, err = strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("INVOICE_LOCAL_OCR_ENABLED is invalid")
		}
	}
	for name, item := range map[string]struct {
		value *time.Duration
		max   time.Duration
	}{"REFERENCE_TIMEOUT": {&c.ReferenceTimeout, 30 * time.Second}, "INVOICE_AI_PROVIDER_TIMEOUT": {&c.AIProviderTimeout, 2 * time.Minute}, "INVOICE_LOCAL_OCR_TIMEOUT": {&c.LocalOCRTimeout, 2 * time.Minute}, "WORKLOAD_REFRESH_BEFORE": {&c.WorkloadRefreshBefore, time.Minute}} {
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
	aiCount := 0
	for _, value := range []string{c.AIProviderURL, c.AIProviderAPIKey, c.AIProviderModel} {
		if strings.TrimSpace(value) != "" {
			aiCount++
		}
	}
	if aiCount != 0 && aiCount != 3 {
		return Config{}, fmt.Errorf("invoice AI provider settings must be complete")
	}
	if aiCount == 3 && !validURL(c.AIProviderURL) {
		return Config{}, fmt.Errorf("invoice AI provider URL is invalid")
	}
	if c.IdentityProvider == "template" {
		if !uuidPattern.MatchString(c.TemplateTenantID) || !uuidPattern.MatchString(c.TemplateSubjectID) || c.TemplateDisplayName == "" || len(c.TemplateAccessToken) < 24 || !validURL(c.EvidenceBaseURL) || len(c.EvidenceAccessToken) < 24 {
			return Config{}, fmt.Errorf("template mode requires identity and Evidence settings")
		}
		return c, nil
	}
	if c.IdentityProvider != "iam" {
		return Config{}, fmt.Errorf("IDENTITY_PROVIDER must be iam or template")
	}
	issuer, err := url.Parse(c.IAMIssuer)
	if err != nil || issuer.Scheme != "https" || issuer.Host == "" || !uuidPattern.MatchString(c.IAMTenantID) || c.IAMPublicKeysPath == "" || c.EvidenceAccessToken != "" || !validURL(c.EvidenceBaseURL) || !validURL(c.IAMServiceCredentialURL) || c.WorkloadID != "invoice-recognition" || !uuidPattern.MatchString(c.WorkloadSubjectID) || c.WorkloadKeyFile == "" {
		return Config{}, fmt.Errorf("IAM mode requires issuer, Evidence and workload identity settings")
	}
	return c, nil
}
func clean(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
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
