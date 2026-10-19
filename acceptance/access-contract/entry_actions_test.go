package accesscontract_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRenderedBonusNamespaceCompatibilityClosure(t *testing.T) {
	for _, profile := range []string{"", "reference-oa-bonus-v2", "reference-oa-bonus-v3"} {
		t.Run(profile, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "suite.json")
			cmd := exec.Command("sh", "../../deployments/d1/ops/render-suite-manifest", path)
			cmd.Env = append(os.Environ(), "TENANT_ID=22222222-2222-4222-8222-222222222222", "REFERENCE_OA_BONUS_PROFILE="+profile)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("render: %v %s", err, out)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var value suiteManifest
			if err := decodeStrict(data, &value); err != nil {
				t.Fatal(err)
			}
			if err := validateSuiteManifest(value); err != nil {
				t.Fatal(err)
			}
			for _, app := range value.Applications {
				if app.ID != "bonus-distribution" {
					continue
				}
				if profile == "" {
					if app.CapabilityNamespace != "bonus-pool" || app.EntryAction != "bonus-pool.read" {
						t.Fatal("legacy Bonus changed")
					}
					return
				}
				for _, mutate := range []func(*suiteApplication){func(a *suiteApplication) { a.CapabilityNamespace = "bonus-pool" }, func(a *suiteApplication) { a.CapabilityNamespaces = []string{} }, func(a *suiteApplication) { a.CapabilityNamespaces = []string{"bonus-pool", "bonus-pool"} }, func(a *suiteApplication) { a.CapabilityNamespaces = make([]string, 17) }, func(a *suiteApplication) { a.CapabilityNamespaces = []string{"*"} }, func(a *suiteApplication) { a.EntryActions = []string{"money-flow-ledger.read"} }, func(a *suiteApplication) { a.CapabilityNamespaces = []string{"bonus-pool"} }} {
					copy := value
					copy.Applications = append([]suiteApplication(nil), value.Applications...)
					for i := range copy.Applications {
						if copy.Applications[i].ID == app.ID {
							mutate(&copy.Applications[i])
						}
					}
					if err := validateSuiteManifest(copy); err == nil {
						t.Fatal("invalid namespaces admitted")
					}
				}
				// Contract remains serializable for strict readers that support this field.
				if _, err := json.Marshal(value); err != nil {
					t.Fatal(err)
				}
				return
			}
			t.Fatal("missing Bonus entry")
		})
	}
}

func TestSuiteEntryActionsOnlyDeclareNativeBoundedAdmission(t *testing.T) {
	value := readStrict[suiteManifest](t, "examples/suite-manifest.json")
	index := -1
	for i, app := range value.Applications {
		if app.ID == "evidence-service" {
			index = i
		}
	}
	if index < 0 {
		t.Fatal("missing Evidence entry")
	}
	app := value.Applications[index]
	if app.EntryAction != "" || len(app.EntryActions) != 4 {
		t.Fatal("legacy broad view still required")
	}
	caps := readStrict[capabilityManifest](t, "examples/evidence-service-capabilities.json")
	native := map[string]bool{}
	for _, cap := range caps.Capabilities {
		native[cap.ID] = true
	}
	for _, action := range app.EntryActions {
		if !native[action] {
			t.Fatalf("entry invents target action %q", action)
		}
	}
	for _, actions := range [][]string{nil, {}, {""}, {"evidence-service.*"}, {"money-flow-ledger.view"}, {"evidence-service.download", "evidence-service.download"}, make([]string, 17)} {
		copy := value
		copy.Applications = append([]suiteApplication(nil), value.Applications...)
		copy.Applications[index].EntryActions = actions
		if err := validateSuiteManifest(copy); err == nil {
			t.Fatalf("invalid actions accepted: %v", actions)
		}
	}
	copy := value
	copy.Applications = append([]suiteApplication(nil), value.Applications...)
	copy.Applications[index].EntryAction = "evidence-service.view"
	if err := validateSuiteManifest(copy); err == nil {
		t.Fatal("both forms accepted")
	}
	copy.Applications[index].EntryActions = nil
	if err := validateSuiteManifest(copy); err != nil {
		t.Fatal("legacy manifest changed", err)
	}
}
