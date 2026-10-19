package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omsbizfoundry/business-engagement/internal/auth"
	"omsbizfoundry/business-engagement/internal/engagement"
)

func TestMaterialCollectionUsesExistingCapabilityAndResourceGate(t *testing.T) {
	for _, operation := range []string{"append", "remove_one"} {
		for _, permission := range []string{"missing-action", "wrong-project", "native-engagement", "current-project"} {
			t.Run(operation+"/"+permission, func(t *testing.T) {
				cases := &fakeUseCases{detail: engagement.Details{Engagement: engagement.Engagement{ID: engagementA, TenantID: tenantID, ProjectID: projectA}}}
				principal := auth.Principal{SubjectID: subjectID, TenantID: tenantID, ScopeKind: "resources", AllowedActions: map[string]bool{auth.ActionEvidenceManage: true}, Resources: map[string]map[string]bool{"project": {projectA: true}}}
				switch permission {
				case "missing-action":
					principal.AllowedActions = map[string]bool{}
				case "wrong-project":
					principal.Resources = map[string]map[string]bool{"project": {projectB: true}}
				case "native-engagement":
					principal.Resources = map[string]map[string]bool{"business-engagement": {engagementA: true}}
				}
				server, _ := New(cases, &fakeAuthenticator{principal: principal})
				input := engagement.MaterialInput{CommandID: commandID, ExpectedRevision: 4, Operation: operation, RequirementKey: engagement.RequirementBidDocument}
				if operation == "append" {
					input.EvidenceFileID = engagementB
					input.EvidenceVersion = 1
					input.EvidenceSourceRevision = 1
				} else {
					input.MaterialReferenceID = engagementB
				}
				raw, _ := json.Marshal(input)
				response := httptest.NewRecorder()
				request := httptest.NewRequest(http.MethodPost, "/api/v1/business-engagements/"+engagementA+"/materials", strings.NewReader(string(raw)))
				request.Header.Set("Content-Type", "application/json")
				server.ServeHTTP(response, request)
				if permission == "missing-action" || permission == "wrong-project" {
					if response.Code != 403 || cases.called != "" {
						t.Fatalf("status=%d called=%s", response.Code, cases.called)
					}
				} else if response.Code != 200 || cases.called != "material" || cases.materialInput != input {
					t.Fatalf("status=%d called=%s input=%#v", response.Code, cases.called, cases.materialInput)
				}
			})
		}
	}
}
