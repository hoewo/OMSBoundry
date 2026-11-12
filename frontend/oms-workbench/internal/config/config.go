package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
)

var canonicalUUIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type Config struct {
	HTTPAddress                string
	SuiteManifestPath          string
	ProductionPersonnelProfile string
	BonusProfile               string
	ProductionStageProfile     string
	OMSOperatingPartyID        string
	OMSOperatingPartyBindings  map[string]string
	FeedbackSDKURL             string
}

func parseOperatingPartyBindings(raw string) (map[string]string, error) {
	bindings := map[string]string{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return bindings, nil
	}
	for _, entry := range strings.Split(raw, ",") {
		parts := strings.Split(strings.TrimSpace(entry), "=")
		if len(parts) != 2 || !canonicalUUIDPattern.MatchString(parts[0]) || !canonicalUUIDPattern.MatchString(parts[1]) {
			return nil, fmt.Errorf("OMS_OPERATING_PARTY_BINDINGS must contain tenant_uuid=party_uuid pairs")
		}
		if _, exists := bindings[parts[0]]; exists {
			return nil, fmt.Errorf("OMS_OPERATING_PARTY_BINDINGS contains a duplicate tenant")
		}
		bindings[parts[0]] = parts[1]
	}
	return bindings, nil
}

func Load() (Config, error) {
	operatingPartyID := strings.TrimSpace(os.Getenv("OMS_OPERATING_PARTY_ID"))
	legacyPartyID := strings.TrimSpace(os.Getenv("OMS_QUALIFICATION_PARTY_ID"))
	if operatingPartyID != "" && legacyPartyID != "" && operatingPartyID != legacyPartyID {
		return Config{}, fmt.Errorf("OMS operating Party ID conflicts with legacy OMS_QUALIFICATION_PARTY_ID")
	}
	if operatingPartyID == "" {
		operatingPartyID = legacyPartyID
	}
	bindings, err := parseOperatingPartyBindings(os.Getenv("OMS_OPERATING_PARTY_BINDINGS"))
	if err != nil {
		return Config{}, err
	}
	value := Config{HTTPAddress: os.Getenv("HTTP_ADDR"), SuiteManifestPath: strings.TrimSpace(os.Getenv("SUITE_MANIFEST_PATH")), ProductionPersonnelProfile: strings.TrimSpace(os.Getenv("REFERENCE_OA_PRODUCTION_PERSONNEL_PROFILE")), BonusProfile: strings.TrimSpace(os.Getenv("REFERENCE_OA_BONUS_PROFILE")), OMSOperatingPartyID: operatingPartyID, OMSOperatingPartyBindings: bindings, FeedbackSDKURL: strings.TrimSpace(os.Getenv("FEEDBACK_SDK_URL"))}
	value.ProductionStageProfile = strings.TrimSpace(os.Getenv("REFERENCE_OA_PRODUCTION_STAGE_PROFILE"))
	if value.HTTPAddress == "" {
		value.HTTPAddress = ":8082"
	}
	if value.SuiteManifestPath == "" {
		return Config{}, fmt.Errorf("SUITE_MANIFEST_PATH is required")
	}
	if value.ProductionPersonnelProfile != "" && value.ProductionPersonnelProfile != "reference-oa-production-personnel-v1" {
		return Config{}, fmt.Errorf("unsupported REFERENCE_OA_PRODUCTION_PERSONNEL_PROFILE")
	}
	if value.BonusProfile != "" && value.BonusProfile != "reference-oa-bonus-v2" && value.BonusProfile != "reference-oa-bonus-v3" {
		return Config{}, fmt.Errorf("unsupported REFERENCE_OA_BONUS_PROFILE")
	}
	if value.ProductionStageProfile != "" && value.ProductionStageProfile != "reference-oa-stage-materials-v1" {
		return Config{}, fmt.Errorf("unsupported REFERENCE_OA_PRODUCTION_STAGE_PROFILE")
	}
	if value.OMSOperatingPartyID != "" && !canonicalUUIDPattern.MatchString(value.OMSOperatingPartyID) {
		return Config{}, fmt.Errorf("OMS_OPERATING_PARTY_ID must be a canonical lowercase UUID")
	}
	if value.FeedbackSDKURL != "" {
		parsed, err := url.Parse(value.FeedbackSDKURL)
		loopback := parsed != nil && (parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1" || parsed.Hostname() == "::1")
		if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || (parsed.Scheme != "https" && !(parsed.Scheme == "http" && loopback)) {
			return Config{}, fmt.Errorf("FEEDBACK_SDK_URL must be an absolute HTTPS URL or loopback HTTP URL")
		}
	}
	return value, nil
}
