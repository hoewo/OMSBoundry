package httpserver

import (
	"strings"
	"testing"
)

func TestBusinessNavigationKeepsCurrentTabVisible(t *testing.T) {
	entries, err := templateFiles.ReadDir("templates")
	if err != nil {
		t.Fatalf("read templates: %v", err)
	}

	checked := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".html") {
			continue
		}
		body, err := templateFiles.ReadFile("templates/" + entry.Name())
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		page := string(body)
		if !strings.Contains(page, "<nav") {
			continue
		}
		checked++
		if count := strings.Count(page, `aria-current="page"`); count != 1 {
			t.Errorf("%s current navigation markers = %d, want 1", entry.Name(), count)
		}
		if !strings.Contains(page, `aria-label="合同台账主导航"`) {
			t.Errorf("%s does not use the stable product navigation", entry.Name())
		}
	}

	if checked == 0 {
		t.Fatal("no navigable templates checked")
	}
	if !strings.Contains(demoWorkbenchCSS, `nav a[aria-current="page"]`) {
		t.Fatal("current navigation item has no visible style")
	}
}
