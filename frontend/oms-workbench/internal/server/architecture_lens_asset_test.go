package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestArchitectureLensAssetAndExplanationKeepOwnerBoundaries(t *testing.T) {
	handler := initiationWorkbench(t)
	r := httptest.NewRequest(http.MethodGet, "/?view=legacy", nil)
	r.Header.Set("X-OMS-Subject-ID", "88888888-8888-4888-8888-888888888888")
	r.Header.Set("X-OMS-Administrator", "true")
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, r)
	body := page.Body.String()
	asset, bridge := strings.Index(body, `src="assets/reference-oa-architecture-lens.js"`), strings.Index(body, `src="assets/workbench.js"`)
	if page.Code != http.StatusOK || asset < 0 || asset >= bridge {
		t.Fatal("architecture model must be served before the bridge")
	}
	for _, text := range []string{"不是服务器审计", "不记录表单内容", "Business Engagement 拥有承接和材料引用", "Evidence 拥有文件与版本", "移除业务引用不删除文件"} {
		if !strings.Contains(body, text) {
			t.Fatalf("missing owner explanation %q", text)
		}
	}
	model := httptest.NewRecorder()
	handler.ServeHTTP(model, httptest.NewRequest(http.MethodGet, "/assets/reference-oa-architecture-lens.js", nil))
	if model.Code != http.StatusOK || model.Header().Get("Content-Type") != "application/javascript; charset=utf-8" || !strings.Contains(model.Body.String(), "observedEpoch!==epoch") {
		t.Fatal("bounded model with late-operation guard is unavailable")
	}
	client := httptest.NewRecorder()
	handler.ServeHTTP(client, httptest.NewRequest(http.MethodGet, "/assets/workbench.js", nil))
	if !strings.Contains(client.Body.String(), "architectureTrace?.record") || strings.Contains(client.Body.String(), "omsbizfoundry-architecture-lens-v1") {
		t.Fatal("bridge must use current model, not the global fixed Project log")
	}
}
