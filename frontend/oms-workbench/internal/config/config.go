package config

import (
	"fmt"
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
	OMSQualificationPartyID    string
}

func Load() (Config, error) {
	value := Config{HTTPAddress: os.Getenv("HTTP_ADDR"), SuiteManifestPath: strings.TrimSpace(os.Getenv("SUITE_MANIFEST_PATH")), ProductionPersonnelProfile: strings.TrimSpace(os.Getenv("REFERENCE_OA_PRODUCTION_PERSONNEL_PROFILE")), BonusProfile: strings.TrimSpace(os.Getenv("REFERENCE_OA_BONUS_PROFILE")), OMSQualificationPartyID: strings.TrimSpace(os.Getenv("OMS_QUALIFICATION_PARTY_ID"))}
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
	if value.OMSQualificationPartyID != "" && !canonicalUUIDPattern.MatchString(value.OMSQualificationPartyID) {
		return Config{}, fmt.Errorf("OMS_QUALIFICATION_PARTY_ID must be a canonical lowercase UUID")
	}
	return value, nil
}
