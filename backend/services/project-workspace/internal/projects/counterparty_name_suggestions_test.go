package projects

import (
	"context"
	"strings"
	"testing"
)

func TestCounterpartyNameSuggestionsValidationAndEmptyZeroStore(t *testing.T) {
	store := &storeStub{}
	svc, err := NewService(store, &referencesStub{})
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []CounterpartyNamesQuery{{ScopeKind: "bad"}, {ScopeKind: "tenant", Limit: 51}, {ScopeKind: "resources", ResourceIDs: []string{"bad"}}, {ScopeKind: "tenant", ResourceIDs: []string{testProject}}, {ScopeKind: "tenant", Query: strings.Repeat("甲", 201)}, {ScopeKind: "tenant", Query: "甲\n方"}} {
		if _, err := svc.CounterpartyNameSuggestions(context.Background(), testTenant, q); err == nil {
			t.Fatalf("accepted %#v", q)
		}
	}
	got, err := svc.CounterpartyNameSuggestions(context.Background(), testTenant, CounterpartyNamesQuery{ScopeKind: "resources", Query: "  "})
	if err != nil || len(got.Names) != 0 || store.namesCalls != 0 {
		t.Fatal("empty query reached store")
	}
	_, err = svc.CounterpartyNameSuggestions(context.Background(), testTenant, CounterpartyNamesQuery{ScopeKind: "resources", ResourceIDs: []string{testProject}, Query: "  甲方  "})
	if err != nil || store.namesCalls != 1 || store.namesQuery.Query != "甲方" || store.namesQuery.Limit != 10 {
		t.Fatal("normalization failed")
	}
}
