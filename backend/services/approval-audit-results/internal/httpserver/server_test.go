package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omsbizfoundry/approval-audit-results/internal/auth"
	"omsbizfoundry/approval-audit-results/internal/results"
)

const (
	unitTenant  = "11111111-1111-4111-8111-111111111111"
	unitProject = "33333333-3333-4333-8333-333333333333"
)

type unitUseCases struct {
	listed         []results.Result
	listCalls      int
	referenceCalls int
}

func (*unitUseCases) Ping(context.Context) error { return nil }
func (*unitUseCases) Record(context.Context, results.Actor, results.RecordInput) (results.Result, error) {
	return results.Result{}, nil
}
func (*unitUseCases) Get(context.Context, string, string) (results.Result, error) {
	return results.Result{}, results.ErrNotFound
}
func (u *unitUseCases) ListProject(context.Context, string, string) ([]results.Result, error) {
	u.listCalls++
	return u.listed, nil
}
func (u *unitUseCases) ReferenceProjectResults(context.Context, string, string) (results.ProjectResultReferenceList, error) {
	u.referenceCalls++
	return results.ProjectResultReferenceList{SchemaVersion: "1", SourceApplication: "approval-audit-results", ProjectID: unitProject}, nil
}
func (*unitUseCases) History(context.Context, string, string) ([]results.Revision, error) {
	return nil, nil
}
func (*unitUseCases) Correct(context.Context, results.Actor, string, results.CorrectInput) (results.Result, error) {
	return results.Result{}, nil
}
func (*unitUseCases) Withdraw(context.Context, results.Actor, string, results.WithdrawInput) (results.Result, error) {
	return results.Result{}, nil
}

type unitAuthenticator struct{ principal auth.Principal }

func (*unitAuthenticator) ProviderName() string { return "test" }
func (a *unitAuthenticator) Authenticate(*http.Request) (auth.Principal, error) {
	return a.principal, nil
}

func unitPrincipal(actions ...string) auth.Principal {
	allowed := map[string]bool{}
	for _, action := range actions {
		allowed[action] = true
	}
	return auth.Principal{
		SubjectID: "22222222-2222-4222-8222-222222222222", TenantID: unitTenant,
		AllowedActions: allowed, ScopeKind: "resources", Resources: map[string]map[string]bool{"project": {unitProject: true}},
	}
}

func TestResultsWorkspaceIsHumanUsableAndProjectScoped(t *testing.T) {
	useCases := &unitUseCases{listed: []results.Result{{ID: "44444444-4444-4444-8444-444444444444", TenantID: unitTenant, Project: results.ProjectSnapshot{ID: unitProject}, Kind: results.KindApproval, State: results.StateRecorded, Revision: 2}}}
	server, _ := New(useCases, &unitAuthenticator{principal: unitPrincipal(auth.ActionRead)})

	page := httptest.NewRecorder()
	server.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "批复与审计结果") || !strings.Contains(page.Body.String(), "使用说明") || !strings.Contains(page.Body.String(), "assets/results.js") {
		t.Fatalf("page status=%d body=%q", page.Code, page.Body.String())
	}
	asset := httptest.NewRecorder()
	server.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/assets/results.js", nil))
	if asset.Code != http.StatusOK || !strings.Contains(asset.Body.String(), "allowed_actions") || !strings.Contains(asset.Body.String(), "corrections") || !strings.Contains(asset.Body.String(), "withdrawal") {
		t.Fatalf("asset status=%d body=%q", asset.Code, asset.Body.String())
	}
	list := httptest.NewRecorder()
	server.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/v1/approval-audit-results?project_id="+unitProject, nil))
	if list.Code != http.StatusOK || useCases.listCalls != 1 || !strings.Contains(list.Body.String(), `"kind":"approval"`) || !strings.Contains(list.Body.String(), `"evidence":[]`) {
		t.Fatalf("list status=%d calls=%d body=%s", list.Code, useCases.listCalls, list.Body.String())
	}

	denied := unitPrincipal(auth.ActionRead)
	denied.Resources = map[string]map[string]bool{"project": {"55555555-5555-4555-8555-555555555555": true}}
	server, _ = New(useCases, &unitAuthenticator{principal: denied})
	reference := httptest.NewRecorder()
	server.ServeHTTP(reference, httptest.NewRequest(http.MethodGet, "/api/v1/references/projects/"+unitProject+"/approval-audit-results", nil))
	if reference.Code != http.StatusForbidden || useCases.referenceCalls != 0 {
		t.Fatalf("reference status=%d calls=%d body=%s", reference.Code, useCases.referenceCalls, reference.Body.String())
	}
}
