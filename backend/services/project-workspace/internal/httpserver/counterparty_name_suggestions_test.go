package httpserver

import (
	"errors"
	"net/http/httptest"
	"omsbizfoundry/project-workspace/internal/auth"
	"reflect"
	"testing"
)

func TestCounterpartyNameSuggestionsNativeCapabilitySignedScopeAndQuery(t *testing.T) {
	for _, tc := range []struct {
		name, url string
		read      bool
		scope     string
		anonymous bool
		code      int
	}{
		{"tenant", "?q=示例", true, "tenant", false, 200}, {"resource", "?q=示例&limit=2", true, "resources", false, 200},
		{"no capability", "?q=示例", false, "tenant", false, 403}, {"invalid scope", "?q=示例", true, "invalid", false, 403},
		{"anonymous", "?q=示例", true, "tenant", true, 401}, {"override", "?q=示例&tenant_id=other", true, "tenant", false, 400},
		{"duplicate", "?q=a&q=b", true, "tenant", false, 400}, {"unbounded", "?q=a&limit=51", true, "tenant", false, 400}, {"zero limit", "?q=a&limit=0", true, "tenant", false, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := fullPrincipal()
			p.AllowedActions = map[string]bool{auth.ActionCounterpartyContextRead: tc.read}
			p.ScopeKind = tc.scope
			p.ResourceIDs = map[string]bool{httpProject: true, httpCommand: false}
			a := &authenticatorStub{principal: p}
			if tc.anonymous {
				a.err = errors.New("no credential")
			}
			stub := &useCasesStub{}
			s, _ := New(stub, a)
			req := httptest.NewRequest("GET", "/api/v1/counterparty-context/name-suggestions"+tc.url, nil)
			req.Header.Set("X-Tenant-ID", "other")
			req.Header.Set("X-Actor-ID", "forged")
			w := httptest.NewRecorder()
			s.ServeHTTP(w, req)
			if w.Code != tc.code {
				t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
			}
			if a.sawHeader != "" {
				t.Fatal("untrusted actor header reached authenticator")
			}
			if tc.code != 200 && stub.namesCalls != 0 {
				t.Fatal("denied call reached domain")
			}
			if tc.code == 200 {
				if stub.namesTenant != httpTenant || stub.namesQuery.ScopeKind != tc.scope || stub.namesQuery.Query != "示例" {
					t.Fatal("trusted scope changed")
				}
				if tc.scope == "resources" && !reflect.DeepEqual(stub.namesQuery.ResourceIDs, []string{httpProject}) {
					t.Fatal("resource scope changed")
				}
			}
		})
	}
}
