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

func TestOMSOperatingPartyIDMustBeCanonicalAndLegacyAliasRemainsCompatible(t *testing.T) {
	t.Setenv("SUITE_MANIFEST_PATH", "test.json")
	t.Setenv("OMS_QUALIFICATION_PARTY_ID", "")
	t.Setenv("OMS_OPERATING_PARTY_ID", "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	cfg, err := Load()
	if err != nil || cfg.OMSOperatingPartyID != "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" {
		t.Fatalf("valid operating Party ID cfg=%+v err=%v", cfg, err)
	}
	t.Setenv("OMS_OPERATING_PARTY_ID", "not-a-party-id")
	if _, err := Load(); err == nil {
		t.Fatal("invalid operating Party ID accepted")
	}
	t.Setenv("OMS_OPERATING_PARTY_ID", "")
	t.Setenv("OMS_QUALIFICATION_PARTY_ID", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
	cfg, err = Load()
	if err != nil || cfg.OMSOperatingPartyID != "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb" {
		t.Fatalf("legacy operating Party alias cfg=%+v err=%v", cfg, err)
	}
	t.Setenv("OMS_OPERATING_PARTY_ID", "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	if _, err := Load(); err == nil {
		t.Fatal("conflicting operating Party declarations accepted")
	}
}

func TestFeedbackSDKURLMustBeHTTPSOrLoopbackHTTP(t *testing.T) {
	t.Setenv("SUITE_MANIFEST_PATH", "test.json")
	for _, valid := range []string{"", "https://feedback.example.test/sdk-v2/index.html?embed=web", "http://127.0.0.1:4180/sdk"} {
		t.Setenv("FEEDBACK_SDK_URL", valid)
		cfg, err := Load()
		if err != nil || cfg.FeedbackSDKURL != valid {
			t.Fatalf("valid feedback SDK URL %q rejected: %#v %v", valid, cfg, err)
		}
	}
	for _, invalid := range []string{"http://feedback.example.test/sdk", "javascript:alert(1)", "https://user:pass@feedback.example.test/sdk", "https://feedback.example.test/sdk#secret"} {
		t.Setenv("FEEDBACK_SDK_URL", invalid)
		if _, err := Load(); err == nil {
			t.Fatalf("invalid feedback SDK URL %q accepted", invalid)
		}
	}
}
