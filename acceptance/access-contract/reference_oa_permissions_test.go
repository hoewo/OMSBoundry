package accesscontract_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

type referenceOAPermissionRules struct {
	SchemaVersion       string `json:"schema_version"`
	RulesetID           string `json:"ruleset_id"`
	RulesetVersion      string `json:"ruleset_version"`
	Source              string `json:"source"`
	AuthorizationLayers []struct {
		ID          string `json:"id"`
		Owner       string `json:"owner"`
		Description string `json:"description"`
	} `json:"authorization_layers"`
	Groups []struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"groups"`
	Items []struct {
		Key      string `json:"key"`
		GroupKey string `json:"group_key"`
		Name     string `json:"name"`
		Risk     string `json:"risk"`
	} `json:"items"`
	Templates map[string]struct {
		Name           string   `json:"name"`
		Scope          string   `json:"scope"`
		Keys           []string `json:"keys"`
		AllPermissions bool     `json:"all_permissions"`
		Inherits       string   `json:"inherits"`
	} `json:"templates"`
	NativeResolution struct {
		IdentitySuperAdmin struct {
			BusinessActions []string `json:"business_actions"`
		} `json:"identity_super_admin"`
		OABusinessAdmin struct {
			Profile string `json:"profile"`
			Rule    string `json:"rule"`
		} `json:"oa_business_admin"`
	} `json:"native_resolution"`
}

type referenceOAAdministratorProfile struct {
	ProfileID                           string `json:"profile_id"`
	ScopeKind                           string `json:"scope_kind"`
	IdentityAdministratorImpliesProfile bool   `json:"identity_administrator_implies_profile"`
	ActionResolution                    struct {
		Mode                      string `json:"mode"`
		WildcardsAllowed          bool   `json:"wildcards_allowed"`
		ReconcileOnManifestChange bool   `json:"reconcile_on_manifest_change"`
	} `json:"action_resolution"`
	TargetAuthorization struct {
		ResourceScopeRequired             bool `json:"resource_scope_required"`
		BusinessStateRequired             bool `json:"business_state_required"`
		ExpectedRevisionRequiredForWrites bool `json:"expected_revision_required_for_writes"`
		AllowedActionsAreAuthoritative    bool `json:"allowed_actions_are_authoritative"`
	} `json:"target_authorization"`
}

func TestReferenceOAPermissionRulesKeepFourLayersAndOriginal49Keys(t *testing.T) {
	var rules referenceOAPermissionRules
	decodeFile(t, filepath.Join(contractRoot, "examples/reference-oa-permission-rules-v1.json"), &rules)
	if rules.SchemaVersion != "1" || rules.RulesetID != "reference-oa-permission-rules" || rules.RulesetVersion != "1.0.0" {
		t.Fatal("unstable reference OA permission rules identity")
	}
	expectedLayers := []string{"identity_governance", "oa_site_position", "project_role_and_temporary_grant", "native_application_authorization"}
	actualLayers := make([]string, 0, len(rules.AuthorizationLayers))
	for _, layer := range rules.AuthorizationLayers {
		actualLayers = append(actualLayers, layer.ID)
		if layer.Owner == "" {
			t.Fatalf("layer %q has no owner", layer.ID)
		}
	}
	if !slices.Equal(actualLayers, expectedLayers) {
		t.Fatalf("authorization layers changed: %v", actualLayers)
	}
	if len(rules.Items) != 49 {
		t.Fatalf("original OA catalog must contain 49 rules, got %d", len(rules.Items))
	}
	groups, keys := map[string]bool{}, map[string]bool{}
	for _, group := range rules.Groups {
		groups[group.Key] = true
	}
	for _, item := range rules.Items {
		if keys[item.Key] || !groups[item.GroupKey] {
			t.Fatalf("invalid or duplicate OA rule %#v", item)
		}
		keys[item.Key] = true
	}
	admin, ok := rules.Templates["admin"]
	if !ok || !admin.AllPermissions || admin.Scope != "tenant" {
		t.Fatal("OA administrator is not the original all-rules tenant template")
	}
	for name, template := range rules.Templates {
		if template.Scope != "tenant" && template.Scope != "project" {
			t.Fatalf("template %q has invalid scope", name)
		}
		for _, key := range template.Keys {
			if !keys[key] {
				t.Fatalf("template %q invents rule %q", name, key)
			}
		}
		if template.Inherits != "" {
			if _, ok := rules.Templates[template.Inherits]; !ok {
				t.Fatalf("template %q inherits missing template", name)
			}
		}
	}
	if len(rules.NativeResolution.IdentitySuperAdmin.BusinessActions) != 0 {
		t.Fatal("identity super administrator was upgraded to OA business administrator")
	}
	if rules.NativeResolution.OABusinessAdmin.Profile != "reference-oa-administrator-v1" {
		t.Fatal("OA administrator profile changed")
	}
}

func TestReferenceOAAdministratorExpandsExactInstalledCatalogWithoutBypass(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(contractRoot, "examples/reference-oa-administrator-profile-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var profile referenceOAAdministratorProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		t.Fatal(err)
	}
	if profile.ProfileID != "reference-oa-administrator-v1" || profile.ScopeKind != "tenant" || profile.IdentityAdministratorImpliesProfile {
		t.Fatal("administrator identity/profile separation changed")
	}
	if profile.ActionResolution.Mode != "installed-capability-catalog-exact-expansion" || profile.ActionResolution.WildcardsAllowed || !profile.ActionResolution.ReconcileOnManifestChange {
		t.Fatal("installed catalog reconciliation changed")
	}
	if !profile.TargetAuthorization.ResourceScopeRequired || !profile.TargetAuthorization.BusinessStateRequired || !profile.TargetAuthorization.ExpectedRevisionRequiredForWrites || !profile.TargetAuthorization.AllowedActionsAreAuthoritative {
		t.Fatal("target final authorization was weakened")
	}
}
