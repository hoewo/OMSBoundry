package server

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestD1RendererLoadsWithEntryAnySet(t *testing.T) {
	for _, profile := range []string{"", "reference-oa-bonus-v2", "reference-oa-bonus-v3"} {
		t.Run("bonus-profile-"+profile, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "suite.json")
			cmd := exec.Command("sh", "../../../../deployments/d1/ops/render-suite-manifest", path)
			cmd.Env = append(os.Environ(), "TENANT_ID=22222222-2222-4222-8222-222222222222", "REFERENCE_OA_BONUS_PROFILE="+profile)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("render D1: %v %s", err, output)
			}
			value, err := New(path)
			if err != nil {
				t.Fatal(err)
			}
			_ = value // Actual strict loader, not a separately mirrored contract parser.
		})
	}
}

func TestWorkbenchMultipleNativeNamespacesAreCompatibleAndBounded(t *testing.T) {
	app := application{ID: "bonus-distribution", Audience: "bonus-distribution", CapabilityNamespaces: []string{"bonus-pool", "bonus-manual-allocation"}, EntryActions: []string{"bonus-pool.read", "bonus-manual-allocation.read"}}
	if err := app.validateEntry(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*application){
		func(a *application) { a.CapabilityNamespace = "bonus-pool" },
		func(a *application) { a.CapabilityNamespaces = []string{} },
		func(a *application) { a.CapabilityNamespaces = []string{"bonus-pool", "bonus-pool"} },
		func(a *application) { a.CapabilityNamespaces = []string{"*"} },
		func(a *application) { a.CapabilityNamespaces = make([]string, 17) },
		func(a *application) { a.CapabilityNamespaces = []string{"bonus-pool"} },
		func(a *application) { a.EntryActions = []string{"money-flow-ledger.read"} },
	} {
		copy := app
		mutate(&copy)
		if err := copy.validateEntry(); err == nil {
			t.Fatalf("invalid namespace accepted: %+v", copy)
		}
	}
}

func TestWorkbenchLoadsAnyEntryAndRespectsNavigationFlags(t *testing.T) {
	app := application{ID: "evidence-service", Audience: "evidence-service", Enabled: true, RoutePrefix: "/evidence", EntryActions: []string{"evidence-service.metadata.read", "evidence-service.download"}}
	build := func(app application) (*Server, error) {
		value := suite{SchemaVersion: "1", SuiteID: "test-suite", TenantID: "77777777-7777-4777-8777-777777777777", Applications: []application{app}}
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if app.EntryActions != nil {
			// omitempty would erase an explicitly empty invalid array from this fixture.
			var raw map[string]any
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatal(err)
			}
			raw["applications"].([]any)[0].(map[string]any)["entry_actions"] = app.EntryActions
			data, err = json.Marshal(raw)
			if err != nil {
				t.Fatal(err)
			}
		}
		path := filepath.Join(t.TempDir(), "suite.json")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		return New(path)
	}
	shown := func(app application) bool {
		handler, err := build(app)
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest("GET", "/?view=legacy", nil)
		request.Header.Set("X-OMS-Subject-ID", "88888888-8888-4888-8888-888888888888")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != 200 {
			t.Fatal(response.Code)
		}
		return strings.Contains(response.Body.String(), "/gateway/authorize/evidence-service")
	}
	if !shown(app) {
		t.Fatal("ANY entry missing from navigation")
	}
	hidden := false
	app.ShowInWorkbench = &hidden
	if shown(app) {
		t.Fatal("hidden technical entry visible")
	}
	app.ShowInWorkbench = nil
	app.Enabled = false
	if shown(app) {
		t.Fatal("disabled entry visible")
	}
	app.Enabled = true
	for _, bad := range []application{
		{ID: "evidence-service", EntryActions: []string{}},
		{ID: "evidence-service", EntryAction: "evidence-service.view", EntryActions: app.EntryActions},
		{ID: "evidence-service", EntryActions: []string{"evidence-service.download", "evidence-service.download"}},
		{ID: "evidence-service", EntryActions: []string{"evidence-service.*"}},
		{ID: "evidence-service", EntryActions: []string{"money-flow-ledger.view"}},
		{ID: "identity-access", EntryActions: []string{"identity-access.read"}},
	} {
		if _, err := build(bad); err == nil {
			t.Fatalf("invalid entry accepted: %+v", bad)
		}
	}
	app.EntryActions = nil
	app.EntryAction = "evidence-service.view"
	if !shown(app) {
		t.Fatal("legacy entry navigation changed")
	}
}
