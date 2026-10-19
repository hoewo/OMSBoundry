package projects

import (
	"strings"
	"testing"
)

func TestDirectoryQueryIsBoundedAndDoesNotMixTenantAndResourceScopes(t *testing.T) {
	for _, input := range []DirectoryQuery{
		{ScopeKind: "tenant", Offset: -1}, {ScopeKind: "tenant", Limit: 101},
		{ScopeKind: "tenant", Limit: -1}, {ScopeKind: "tenant", Keyword: strings.Repeat("界", 201)},
		{ScopeKind: "tenant", ResourceIDs: []string{testProject}},
		{ScopeKind: "resources", ResourceIDs: []string{"*"}}, {ScopeKind: "none"},
	} {
		if _, err := normalizeDirectoryQuery(input); err == nil {
			t.Fatalf("accepted %#v", input)
		}
	}
	valid, err := normalizeDirectoryQuery(DirectoryQuery{ScopeKind: "resources", Keyword: " 空间%_ "})
	if err != nil || valid.Limit != 100 || valid.Keyword != "空间%_" {
		t.Fatalf("normalized %#v: %v", valid, err)
	}
}
