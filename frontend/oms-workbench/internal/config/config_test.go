package config

import "testing"

func TestExplicitReferenceProfilesRemainClosed(t *testing.T) {
	t.Setenv("SUITE_MANIFEST_PATH", "test.json")
	for _, p := range []string{"", "reference-oa-production-personnel-v1", "unknown"} {
		t.Setenv("REFERENCE_OA_PRODUCTION_PERSONNEL_PROFILE", p)
		for _, b := range []string{"", "reference-oa-bonus-v2", "reference-oa-bonus-v3", "unknown"} {
			t.Setenv("REFERENCE_OA_BONUS_PROFILE", b)
			_, err := Load()
			if (p == "unknown" || b == "unknown") != (err != nil) {
				t.Fatalf("profile %q/%q error=%v", p, b, err)
			}
		}
	}
}
func TestExplicitStageProfileRemainsClosed(t *testing.T) {
	t.Setenv("SUITE_MANIFEST_PATH", "test.json")
	for _, profile := range []string{"", "reference-oa-stage-materials-v1", "unknown"} {
		t.Setenv("REFERENCE_OA_PRODUCTION_STAGE_PROFILE", profile)
		cfg, err := Load()
		if profile == "unknown" {
			if err == nil {
				t.Fatal("unknown accepted")
			}
		} else if err != nil || cfg.ProductionStageProfile != profile {
			t.Fatal(profile, err)
		}
	}
}

func TestOMSQualificationPartyIDMustBeCanonical(t *testing.T) {
	t.Setenv("SUITE_MANIFEST_PATH", "test.json")
	t.Setenv("OMS_QUALIFICATION_PARTY_ID", "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	cfg, err := Load()
	if err != nil || cfg.OMSQualificationPartyID != "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" {
		t.Fatalf("valid qualification Party ID cfg=%+v err=%v", cfg, err)
	}
	t.Setenv("OMS_QUALIFICATION_PARTY_ID", "not-a-party-id")
	if _, err := Load(); err == nil {
		t.Fatal("invalid qualification Party ID accepted")
	}
}
