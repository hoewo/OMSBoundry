package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omsbizfoundry/project-workspace/internal/auth"
	"omsbizfoundry/project-workspace/internal/projects"
)

func TestBasicInfoChangeRequiresEveryIncludedNativeCapability(t *testing.T) {
	profile := &projects.BasicInfoFields{Name: "项目", ProjectNumber: "OA-001", ProjectTypeKeys: []string{"design"}}
	base := projects.BasicInfoChangeInput{CommandID: httpCommand, ExpectedRevision: 3, Reason: "修改项目基本信息"}
	for _, test := range []struct {
		name    string
		actions []string
		profile bool
		owners  []projects.OwnerChange
		code    int
	}{
		{"profile only", []string{auth.ActionProfileCorrect}, true, nil, 200},
		{"business only", []string{auth.ActionAssignmentBusinessOwnerManage}, false, []projects.OwnerChange{{RoleKey: projects.RoleBusinessOwner, Operation: "clear"}}, 200},
		{"production only", []string{auth.ActionAssignmentProductionOwnerManage}, false, []projects.OwnerChange{{RoleKey: projects.RoleProductionOwner, Operation: "clear"}}, 200},
		{"production cannot business", []string{auth.ActionAssignmentProductionOwnerManage}, false, []projects.OwnerChange{{RoleKey: projects.RoleBusinessOwner, Operation: "clear"}}, 403},
		{"core cannot owner", []string{auth.ActionProfileCorrect}, true, []projects.OwnerChange{{RoleKey: projects.RoleProductionOwner, Operation: "clear"}}, 403},
		{"owner cannot core", []string{auth.ActionAssignmentManage}, true, []projects.OwnerChange{{RoleKey: projects.RoleProductionOwner, Operation: "clear"}}, 403},
		{"both required", []string{auth.ActionAssignmentBusinessOwnerManage}, false, []projects.OwnerChange{{RoleKey: projects.RoleBusinessOwner, Operation: "clear"}, {RoleKey: projects.RoleProductionOwner, Operation: "clear"}}, 403},
		{"all", []string{auth.ActionProfileCorrect, auth.ActionAssignmentManage}, true, []projects.OwnerChange{{RoleKey: projects.RoleBusinessOwner, Operation: "clear"}, {RoleKey: projects.RoleProductionOwner, Operation: "clear"}}, 200},
		{"missing operation", []string{auth.ActionAssignmentManage}, false, []projects.OwnerChange{{RoleKey: projects.RoleProductionOwner}}, 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			principal := fullPrincipal()
			principal.AllowedActions = map[string]bool{}
			for _, v := range test.actions {
				principal.AllowedActions[v] = true
			}
			stub := &useCasesStub{}
			server, _ := New(stub, &authenticatorStub{principal: principal})
			in := base
			in.Owners = test.owners
			if test.profile {
				in.Profile = profile
			}
			raw, _ := json.Marshal(in)
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+httpProject+"/basic-info-changes", strings.NewReader(string(raw))))
			calls := 0
			if test.code == 200 {
				calls = 1
			}
			if response.Code != test.code || stub.basicChangeCalls != calls {
				t.Fatalf("code=%d calls=%d body=%s", response.Code, stub.basicChangeCalls, response.Body.String())
			}
		})
	}
	principal := fullPrincipal()
	principal.ScopeKind = "resources"
	principal.ResourceIDs = map[string]bool{}
	stub := &useCasesStub{}
	server, _ := New(stub, &authenticatorStub{principal: principal})
	raw, _ := json.Marshal(base)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+httpProject+"/basic-info-changes", strings.NewReader(string(raw))))
	if response.Code != 403 || stub.basicChangeCalls != 0 {
		t.Fatalf("scope=%d", response.Code)
	}
}

func TestInitiationRecoveryRequiresUpdateAndEachOwnerNativeCapability(t *testing.T) {
	for _, tc := range []struct {
		actions []string
		profile bool
		owners  []projects.OwnerChange
		code    int
	}{
		{[]string{auth.ActionUpdate}, true, nil, 200},
		{[]string{auth.ActionProfileCorrect}, true, nil, 403},
		{[]string{auth.ActionAssignmentBusinessOwnerManage}, false, []projects.OwnerChange{{RoleKey: projects.RoleBusinessOwner, Operation: "set", Person: &projects.InitiationOwnerReference{PersonID: httpSubject, SourceRevision: 1}}}, 200},
		{[]string{auth.ActionAssignmentProductionOwnerManage}, false, []projects.OwnerChange{{RoleKey: projects.RoleBusinessOwner, Operation: "clear"}}, 403},
		{[]string{auth.ActionUpdate}, true, []projects.OwnerChange{{RoleKey: projects.RoleProductionOwner, Operation: "clear"}}, 403},
		{[]string{auth.ActionAssignmentManage}, true, []projects.OwnerChange{{RoleKey: projects.RoleBusinessOwner, Operation: "clear"}}, 403},
		{[]string{auth.ActionUpdate, auth.ActionAssignmentManage}, true, []projects.OwnerChange{{RoleKey: projects.RoleBusinessOwner, Operation: "clear"}, {RoleKey: projects.RoleProductionOwner, Operation: "clear"}}, 200},
	} {
		p := fullPrincipal()
		p.AllowedActions = map[string]bool{}
		for _, action := range tc.actions {
			p.AllowedActions[action] = true
		}
		stub := &useCasesStub{}
		server, _ := New(stub, &authenticatorStub{principal: p})
		in := projects.BasicInfoChangeInput{CommandID: httpCommand, ExpectedRevision: 6, Reason: "补齐立项", Owners: tc.owners}
		if tc.profile {
			in.Profile = &projects.BasicInfoFields{Name: "项目", ProjectNumber: "OA-101", ProjectTypeKeys: []string{"design"}}
		}
		raw, _ := json.Marshal(in)
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+httpProject+"/initiation-info-changes", strings.NewReader(string(raw))))
		expected := 0
		if tc.code == 200 {
			expected = 1
		}
		if response.Code != tc.code || stub.basicChangeCalls != expected {
			t.Fatalf("capabilities=%v code=%d calls=%d body=%s", tc.actions, response.Code, stub.basicChangeCalls, response.Body.String())
		}
	}
}
