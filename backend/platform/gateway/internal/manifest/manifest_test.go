package manifest

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestD1RendererUsesCompatibleEntryAnySet(t *testing.T) {
	for _, profile := range []string{"", "reference-oa-bonus-v2", "reference-oa-bonus-v3"} {
		t.Run("bonus-profile-"+profile, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "suite.json")
			cmd := exec.Command("sh", "../../../../../deployments/d1/ops/render-suite-manifest", path)
			cmd.Env = append(os.Environ(), "TENANT_ID=22222222-2222-4222-8222-222222222222", "REFERENCE_OA_BONUS_PROFILE="+profile)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("render D1: %v %s", err, output)
			}
			value, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			app, found := value.Application("evidence-service")
			if !found || app.EntryAction != "" || len(app.EntryCapabilities()) != 4 {
				t.Fatal("renderer and Gateway entry contracts diverge")
			}
			bonus, found := value.Application("bonus-distribution")
			if !found {
				t.Fatal("missing Bonus application")
			}
			if profile == "" && (bonus.CapabilityNamespace != "bonus-pool" || bonus.EntryAction != "bonus-pool.read" || bonus.CapabilityNamespaces != nil) {
				t.Fatal("legacy Bonus admission changed")
			}
			if profile != "" && (bonus.CapabilityNamespace != "" || len(bonus.CapabilityNamespaces) != 2 || len(bonus.EntryActions) != 2 || bonus.EntryActions[0] != "bonus-pool.read" || bonus.EntryActions[1] != "bonus-manual-allocation.read") {
				t.Fatal("candidate Bonus native admission missing")
			}
		})
	}
	path := filepath.Join(t.TempDir(), "suite.json")
	cmd := exec.Command("sh", "../../../../../deployments/d1/ops/render-suite-manifest", path)
	cmd.Env = append(os.Environ(), "TENANT_ID=22222222-2222-4222-8222-222222222222", "REFERENCE_OA_BONUS_PROFILE=unknown")
	if _, err := cmd.CombinedOutput(); err == nil {
		t.Fatal("unknown profile rendered")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("invalid profile wrote output")
	}
}

func TestMultipleNativeNamespacesRemainBoundedAndExclusive(t *testing.T) {
	base := Suite{SchemaVersion: "1", SuiteID: "test-suite", TenantID: "22222222-2222-4222-8222-222222222222", Applications: []Application{
		{ID: "identity-access", Audience: "identity-access", Enabled: true, Required: true, RoutePrefix: "/auth", ReadinessPath: "/health/ready", Upstream: "http://identity:8080"},
		{ID: "bonus-distribution", Audience: "bonus-distribution", Enabled: true, RoutePrefix: "/bonus", ReadinessPath: "/health/ready", Upstream: "http://bonus:8080", CapabilityNamespaces: []string{"bonus-pool", "bonus-manual-allocation"}, EntryActions: []string{"bonus-pool.read", "bonus-manual-allocation.read"}},
	}}
	if err := validate(base); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Application){
		func(a *Application) { a.CapabilityNamespace = "bonus-pool" },
		func(a *Application) { a.CapabilityNamespaces = []string{} },
		func(a *Application) { a.CapabilityNamespaces = []string{"bonus-pool", "bonus-pool"} },
		func(a *Application) { a.CapabilityNamespaces = []string{"*"} },
		func(a *Application) { a.CapabilityNamespaces = make([]string, 17) },
		func(a *Application) { a.CapabilityNamespaces = []string{"bonus-pool"} },
		func(a *Application) { a.EntryActions = []string{"money-flow-ledger.read"} },
	} {
		copy := base
		copy.Applications = append([]Application(nil), base.Applications...)
		mutate(&copy.Applications[1])
		if err := validate(copy); err == nil {
			t.Fatalf("invalid native namespace accepted: %+v", copy.Applications[1])
		}
	}
}

func TestEntryActionsAreBoundedExclusiveNativeAnySet(t *testing.T) {
	base := Suite{SchemaVersion: "1", SuiteID: "test-suite", TenantID: "22222222-2222-4222-8222-222222222222", Applications: []Application{
		{ID: "identity-access", Audience: "identity-access", Enabled: true, Required: true, RoutePrefix: "/auth", ReadinessPath: "/health/ready", Upstream: "http://identity:8080"},
		{ID: "evidence-service", Audience: "evidence-service", Enabled: true, RoutePrefix: "/evidence", ReadinessPath: "/health/ready", Upstream: "http://evidence:8080", EntryActions: []string{"evidence-service.metadata.read", "evidence-service.download"}},
	}}
	if err := validate(base); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "suite.json")
	data, _ := json.Marshal(base)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	app, _ := loaded.Application("evidence-service")
	if len(app.EntryCapabilities()) != 2 || app.EntryAction != "" {
		t.Fatal("ANY set not retained")
	}
	tests := []struct {
		name   string
		mutate func(*Suite)
	}{
		{"both", func(s *Suite) { s.Applications[1].EntryAction = "evidence-service.view" }},
		{"empty", func(s *Suite) { s.Applications[1].EntryActions = []string{} }},
		{"missing", func(s *Suite) { s.Applications[1].EntryActions = nil }},
		{"duplicate", func(s *Suite) {
			s.Applications[1].EntryActions = []string{"evidence-service.download", "evidence-service.download"}
		}},
		{"wildcard", func(s *Suite) { s.Applications[1].EntryActions = []string{"evidence-service.*"} }},
		{"wrong namespace", func(s *Suite) { s.Applications[1].EntryActions = []string{"money-flow-ledger.view"} }},
		{"blank", func(s *Suite) { s.Applications[1].EntryActions = []string{""} }},
		{"too many", func(s *Suite) { s.Applications[1].EntryActions = make([]string, 17) }},
		{"platform", func(s *Suite) { s.Applications[0].EntryActions = []string{"identity-access.read"} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			copy := base
			copy.Applications = append([]Application(nil), base.Applications...)
			test.mutate(&copy)
			if err := validate(copy); err == nil {
				t.Fatal("invalid entry set accepted")
			}
		})
	}
	base.Applications[1].EntryActions = nil
	base.Applications[1].EntryAction = "evidence-service.view"
	if err := validate(base); err != nil {
		t.Fatal("legacy singleton changed", err)
	}
}

func TestLoadAllowsExplicitCapabilityNamespace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "suite.json")
	data := []byte(`{
  "schema_version":"1",
  "suite_id":"test-suite",
  "tenant_id":"22222222-2222-4222-8222-222222222222",
  "applications":[
    {"id":"identity-access","version":"0.1.0","enabled":true,"required":true,"route_prefix":"/auth","upstream":"http://identity-access:8080","readiness_path":"/health/ready","audience":"identity-access"},
    {"id":"production-operations","version":"0.1.0","enabled":true,"required":false,"route_prefix":"/production","upstream":"http://production-operations:8087","readiness_path":"/health/ready","audience":"production-operations","capability_namespace":"production-work-item","entry_action":"production-work-item.read"}
  ]
}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	value, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	app, found := value.Application("production-operations")
	if !found || app.CapabilityNamespace != "production-work-item" {
		t.Fatal("explicit capability namespace was not retained")
	}
}
