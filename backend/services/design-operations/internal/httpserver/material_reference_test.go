package httpserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omsbizfoundry/design-operations/internal/auth"
	"omsbizfoundry/design-operations/internal/design"
)

type materialUseCases struct {
	authorizationUseCases
	calls int
	err   error
}

func (u *materialUseCases) ReferenceMaterials(_ context.Context, tenant, id string) (design.MaterialReferenceProjection, error) {
	u.calls++
	if tenant != authTenant || id != authItem {
		panic("material HTTP widened the resource request")
	}
	return design.MaterialReferenceProjection{SchemaVersion: "1", SourceApplication: "production-operations", ID: id, TenantID: tenant, ProjectID: authProject, ResourceType: "production-work-item", Status: "in-progress", Revision: 5, Digest: strings.Repeat("a", 64), Items: []design.MaterialProjectionItem{{RelationKey: "report.primary-output", FileID: authSubject, Version: 2, EvidenceSourceRevision: 7, DisplayName: "咨询成果.pdf"}}}, u.err
}

func TestProductionMaterialReferenceNativeCapabilityAndScopes(t *testing.T) {
	for _, test := range []struct {
		name      string
		action    string
		scope     string
		resources map[string]map[string]bool
		authErr   error
		status    int
	}{
		{"anonymous", "", "", nil, auth.ErrUnauthenticated, 401},
		{"no action", "", "tenant", nil, nil, 403},
		{"legacy action", auth.ActionReferenceRead, "tenant", nil, nil, 403},
		{"neighbor item", auth.ProductionActionReferenceRead, "resources", map[string]map[string]bool{"production-work-item": {authSubject: true}}, nil, 403},
		{"neighbor project", auth.ProductionActionReferenceRead, "resources", map[string]map[string]bool{"project": {authSubject: true}}, nil, 403},
		{"legacy scope", auth.ProductionActionReferenceRead, "resources", map[string]map[string]bool{"design-work-item": {authItem: true}}, nil, 403},
		{"exact item", auth.ProductionActionReferenceRead, "resources", map[string]map[string]bool{"production-work-item": {authItem: true}}, nil, 200},
		{"project is not a native reference scope", auth.ProductionActionReferenceRead, "resources", map[string]map[string]bool{"project": {authProject: true}}, nil, 403},
		{"tenant", auth.ProductionActionReferenceRead, "tenant", nil, nil, 200},
	} {
		t.Run(test.name, func(t *testing.T) {
			useCases := &materialUseCases{}
			authenticator := &authorizationAuthenticator{err: test.authErr, principal: auth.Principal{TenantID: authTenant, SubjectID: authSubject, ScopeKind: test.scope, AllowedActions: map[string]bool{test.action: true}, Resources: test.resources}}
			server, err := NewWithProduction(useCases, authenticator, authenticator)
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodGet, "/api/v1/references/production-work-items/"+authItem+"/materials", nil)
			request.Header.Set("X-Actor-ID", "forged-actor")
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)
			if response.Code != test.status || response.Header().Get("Cache-Control") != "no-store" || authenticator.sawActorHeader != "" {
				t.Fatalf("status=%d body=%s headers=%v", response.Code, response.Body.String(), response.Header())
			}
			if test.status == 200 {
				if useCases.calls != 1 || !strings.Contains(response.Body.String(), `"source_application":"production-operations"`) || !strings.Contains(response.Body.String(), `"evidence_source_revision":7`) {
					t.Fatalf("projection calls=%d body=%s", useCases.calls, response.Body.String())
				}
			} else if useCases.calls != 0 {
				t.Fatal("denied caller reached projection")
			}
		})
	}
}

func TestProductionMaterialReferenceErrorsAreNotEmptySuccessOrStorageLeak(t *testing.T) {
	for _, test := range []struct {
		err    error
		status int
	}{{design.ErrNotFound, 404}, {design.ErrReferenceInvalid, 422}, {design.ErrReferenceUnavailable, 503}, {errors.New("private-object-key-secret"), 500}} {
		useCases := &materialUseCases{err: test.err}
		authenticator := &authorizationAuthenticator{principal: auth.Principal{TenantID: authTenant, SubjectID: authSubject, ScopeKind: "tenant", AllowedActions: map[string]bool{auth.ProductionActionReferenceRead: true}}}
		server, _ := NewWithProduction(useCases, authenticator, authenticator)
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/references/production-work-items/"+authItem+"/materials", nil))
		if response.Code != test.status || strings.Contains(response.Body.String(), "private-object-key-secret") || strings.Contains(response.Body.String(), `"items"`) || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("source error status=%d body=%s", response.Code, response.Body.String())
		}
	}
}

func TestProductionMaterialReferenceDoesNotCreateLegacyOrUnselectedRoute(t *testing.T) {
	useCases := &materialUseCases{}
	authenticator := &authorizationAuthenticator{principal: auth.Principal{TenantID: authTenant, SubjectID: authSubject, ScopeKind: "tenant", AllowedActions: map[string]bool{auth.ActionReferenceRead: true}}}
	legacy, _ := New(useCases, authenticator)
	canonical, _ := NewWithProduction(useCases, authenticator, authenticator)
	for _, test := range []struct {
		server *Server
		path   string
	}{{legacy, "/api/v1/references/production-work-items/" + authItem + "/materials"}, {canonical, "/api/v1/references/design-work-items/" + authItem + "/materials"}} {
		response := httptest.NewRecorder()
		test.server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
		if response.Code != 404 || useCases.calls != 0 {
			t.Fatalf("unexpected compatibility route status=%d calls=%d", response.Code, useCases.calls)
		}
	}
}
