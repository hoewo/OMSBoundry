package server

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStatisticsAndPermissionMountSourceBoundUI(t *testing.T) {
	handler := initiationWorkbench(t)
	r := httptest.NewRequest("GET", "/?view=legacy", nil)
	r.Header.Set("X-OMS-Subject-ID", "22222222-2222-4222-8222-222222222222")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	for _, marker := range []string{`data-statistics-legacy-fixture`, `统计工作台需要 JavaScript`, `来源只读`} {
		if !strings.Contains(w.Body.String(), marker) {
			t.Fatalf("statistics safe fallback missing %q", marker)
		}
	}
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/assets/workbench.js", nil))
	for _, marker := range []string{"createStatisticsWorkspace", "createPermissionWorkspace", "statisticsCostScope", "statisticsExport", "/export-view", "value.revision!==row.revision"} {
		if !strings.Contains(w.Body.String(), marker) {
			t.Fatalf("source-bound UI bridge missing %q", marker)
		}
	}
}
