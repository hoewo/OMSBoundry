package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func initiationWorkbench(t *testing.T, disablePeople ...bool) *Server {
	t.Helper()
	peopleEnabled := len(disablePeople) == 0 || !disablePeople[0]
	manifest := suite{SchemaVersion: "1", SuiteID: "oa-ui-test", TenantID: "11111111-1111-4111-8111-111111111111", Applications: []application{
		{ID: "identity-access", Enabled: true, Version: "0.1.0", RoutePrefix: "/configured-iam"},
		{ID: "project-workspace", Enabled: true, Version: "0.1.0", RoutePrefix: "/configured-project", EntryAction: "project-workspace.read"},
		{ID: "people-groups", Enabled: peopleEnabled, Version: "0.1.0", RoutePrefix: "/configured-people", EntryAction: "people-groups.read"},
	}}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "suite.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	handler, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func TestReferenceOAPersonnelRouteAbsentWhenPeopleNotSelected(t *testing.T) {
	handler := initiationWorkbench(t, true)
	request := httptest.NewRequest(http.MethodGet, "/?view=legacy", nil)
	request.Header.Set("X-OMS-Subject-ID", "88888888-8888-4888-8888-888888888888")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), `data-real-application="people-groups"`) {
		t.Fatal("disabled People must not receive a candidate-service fallback route")
	}
}

func TestReferenceOAAllSelectedNativeRoutesReachOriginalComponents(t *testing.T) {
	manifest := suite{SchemaVersion: "1", SuiteID: "oa-route-test", TenantID: "11111111-1111-4111-8111-111111111111"}
	hidden := false
	for _, id := range []string{"project-workspace", "contract-ledger", "money-flow-ledger", "billing-invoice", "party-directory", "approval-audit-results", "cost-outsourcing", "bonus-distribution"} {
		manifest.Applications = append(manifest.Applications, application{ID: id, Enabled: true, Version: "0.1.0", RoutePrefix: "/configured-" + id, EntryAction: id + ".read", ShowInWorkbench: &hidden})
	}
	manifest.Applications = append(manifest.Applications, application{ID: "evidence-service", Enabled: false, Version: "0.1.0", RoutePrefix: "/not-selected", EntryAction: "evidence-service.view"})
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "suite.json")
	if err = os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	handler, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/?view=legacy", nil)
	request.Header.Set("X-OMS-Subject-ID", "88888888-8888-4888-8888-888888888888")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatal(response.Code)
	}
	body := response.Body.String()
	for _, app := range manifest.Applications {
		marker := `data-real-application="` + app.ID + `" data-real-route="` + app.RoutePrefix + `"`
		if strings.Contains(body, marker) != app.Enabled {
			t.Fatalf("route selection mismatch: %s", app.ID)
		}
	}
}

func TestReferenceOAInitiationAssetsKeepConfiguredBoundaries(t *testing.T) {
	handler := initiationWorkbench(t)
	request := httptest.NewRequest(http.MethodGet, "/?view=legacy", nil)
	request.Header.Set("X-OMS-Subject-ID", "88888888-8888-4888-8888-888888888888")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	body := response.Body.String()
	if !strings.Contains(body, `data-identity-route-prefix="/configured-iam"`) {
		t.Fatal("CSRF renewal must use the configured IAM route, including non-navigation applications")
	}
	if !strings.Contains(body, `data-real-application="people-groups" data-real-route="/configured-people"`) {
		t.Fatal("original personnel candidate port must receive its enabled Manifest route")
	}
	for _, marker := range []string{`data-tenant-id="11111111-1111-4111-8111-111111111111"`, `data-project-route-prefix="/configured-project"`, `data-people-route-prefix="/configured-people"`, `data-initiation-business-owner`, `data-initiation-production-owner`, `data-real-profile-business-owner`, `data-real-profile-production-owner`, `data-view="project-real-detail"`, `data-real-profile-open`, `data-real-profile-submit`, `data-real-profile-reload`, `data-real-profile-drawer`, `type="hidden" value="修改项目基本资料"`, `data-real-detail-tab="business"`, `data-real-client-input="name"`, `data-real-client-input="contact_name"`, `data-real-client-input="contact_phone"`, `data-real-client-submit`, `data-real-client-cancel`, `data-real-client-reload`, `名称和联系人直接保存到当前项目，不会新建或修改甲方主数据。`} {
		if !strings.Contains(body, marker) {
			t.Fatalf("missing %q", marker)
		}
	}
	for _, marker := range []string{`data-initiation-create-number`, `data-initiation-create-type`, `data-initiation-complete-handoff`, `data-real-owner-open`, `data-real-owner-submit`} {
		if strings.Contains(body, marker) {
			t.Fatalf("obsolete early-input or native-app handoff remains: %q", marker)
		}
	}
	for _, marker := range []string{`data-real-client-name-hint`, `aria-label="清空甲方名称"`, `aria-label="清空联系人"`, `aria-label="清空联系电话"`, `class="ant-input"`, `reference-entity-form__field`, `历史名称行内匹配后可按 Tab 补全`} {
		if !strings.Contains(body, marker) {
			t.Fatalf("original client form interaction or styling missing: %q", marker)
		}
	}
	bridge := httptest.NewRecorder()
	handler.ServeHTTP(bridge, httptest.NewRequest(http.MethodGet, "/assets/workbench.js", nil))
	if bridge.Code != 200 || !strings.Contains(bridge.Body.String(), "referenceOAController.detailError(error)") {
		t.Fatal("initial detail failures must be visible in the project list, not a hidden initiation panel")
	}
	if !strings.Contains(bridge.Body.String(), "await referenceOACSRF.token()") || strings.Contains(bridge.Body.String(), "csrf=initiationWorkspace.dataset.projectCsrfToken") {
		t.Fatal("Project authorization must use the current cookie, not the initial page token")
	}
	runtimeIndex, mainIndex := strings.Index(body, `src="assets/reference-oa-initiation-runtime.js"`), strings.Index(body, `src="assets/workbench.js"`)
	directoryIndex := strings.Index(body, `src="assets/reference-oa-project-directory.js"`)
	if directoryIndex < 0 || directoryIndex >= runtimeIndex || !strings.Contains(body, `data-subject-id="88888888-8888-4888-8888-888888888888"`) || !strings.Contains(body, `data-real-list-mode="timeline"`) || !strings.Contains(body, `data-real-page-size`) {
		t.Fatal("directory context, views or asset order missing")
	}
	if runtimeIndex < 0 || mainIndex < 0 || runtimeIndex >= mainIndex {
		t.Fatal("initiation runtime must load before the workbench bridge")
	}
	asset := httptest.NewRecorder()
	handler.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/assets/reference-oa-initiation-runtime.js", nil))
	if asset.Code != 200 || asset.Header().Get("Content-Type") != "application/javascript; charset=utf-8" {
		t.Fatal("runtime asset not available")
	}
	for _, marker := range []string{"/api/v1/initiation-dispatches", "business-owner-v1", "expected_revision", "this.openDetail(id)", "this.root.dataset.peopleRoutePrefix", "source_revision", "当前账号没有完成"} {
		if !strings.Contains(asset.Body.String(), marker) {
			t.Fatalf("runtime missing %q", marker)
		}
	}
	for _, marker := range []string{"/api/v1/counterparty-context/name-suggestions?q=", "updateClientClearButtons()", "detailError(error)", "Normal Tab focus movement is intentionally not prevented"} {
		if !strings.Contains(asset.Body.String(), marker) {
			t.Fatalf("client completion or failure handling missing: %q", marker)
		}
	}
	if strings.Contains(asset.Body.String(), "location.assign") || strings.Contains(asset.Body.String(), "Bearer ") {
		t.Fatal("customer flow may not jump into a native app or embed credentials")
	}
	if !strings.Contains(asset.Body.String(), "await this.csrf.token()") || strings.Contains(asset.Body.String(), "csrf_token: this.root.dataset.projectCsrfToken") {
		t.Fatal("People authorization must share the bounded CSRF renewal helper")
	}
}
