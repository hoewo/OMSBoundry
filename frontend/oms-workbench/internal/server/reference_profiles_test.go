package server

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReferenceUIProfilesAreSelectedNotAutomaticAuthority(t *testing.T) {
	selected := ReferenceOAProfiles{ProductionPersonnel: "reference-oa-production-personnel-v1", Bonus: "reference-oa-bonus-v3"}
	makeManifest := func(disabled string) string {
		t.Helper()
		apps := []application{}
		for _, id := range []string{"project-workspace", "people-groups", "bonus-distribution"} {
			apps = append(apps, application{ID: id, Audience: id, Enabled: id != disabled, RoutePrefix: "/" + id, EntryAction: id + ".read"})
		}
		v := suite{SchemaVersion: "1", SuiteID: "selected", TenantID: "22222222-2222-4222-8222-222222222222", Applications: apps}
		b, _ := json.Marshal(v)
		p := filepath.Join(t.TempDir(), "suite.json")
		if err := os.WriteFile(p, b, 0600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := makeManifest("")
	s, err := NewWithProfiles(p, selected)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/?view=legacy", nil)
	r.Header.Set("X-OMS-Subject-ID", "11111111-1111-4111-8111-111111111111")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	for _, marker := range []string{`data-production-personnel-profile="reference-oa-production-personnel-v1"`, `data-bonus-profile="reference-oa-bonus-v3"`} {
		if !strings.Contains(w.Body.String(), marker) {
			t.Fatal("explicit UI selection missing")
		}
	}
	old, err := New(p)
	if err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	old.ServeHTTP(w, r)
	if strings.Contains(w.Body.String(), `data-bonus-profile="reference-oa-bonus-v3"`) {
		t.Fatal("default UI silently selected")
	}
	for _, id := range []string{"project-workspace", "people-groups", "bonus-distribution"} {
		if _, err = NewWithProfiles(makeManifest(id), selected); err == nil {
			t.Fatal("disabled owner selected: " + id)
		}
	}
	if _, err = NewWithProfiles(p, ReferenceOAProfiles{Bonus: selected.Bonus}); err == nil {
		t.Fatal("v3 without personnel profile")
	}
}
