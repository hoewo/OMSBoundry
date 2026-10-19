package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omsbizfoundry/cost-outsourcing/internal/auth"
	"omsbizfoundry/cost-outsourcing/internal/costoutsourcing"
)

const (
	unitTenant  = "11111111-1111-4111-8111-111111111111"
	unitSubject = "22222222-2222-4222-8222-222222222222"
	unitProject = "33333333-3333-4333-8333-333333333333"
)

type unitUseCases struct {
	costs       []costoutsourcing.CostEntry
	engagements []costoutsourcing.Engagement
	assessments []costoutsourcing.Assessment
	suggestions []costoutsourcing.PayableSuggestion
}

func (*unitUseCases) Ping(context.Context) error { return nil }
func (*unitUseCases) RecordCost(context.Context, costoutsourcing.Actor, costoutsourcing.RecordCostInput) (costoutsourcing.CostEntry, error) {
	return costoutsourcing.CostEntry{}, nil
}
func (*unitUseCases) GetCost(context.Context, costoutsourcing.Actor, string) (costoutsourcing.CostEntry, error) {
	return costoutsourcing.CostEntry{}, costoutsourcing.ErrNotFound
}
func (u *unitUseCases) ListCosts(context.Context, costoutsourcing.Actor, string) ([]costoutsourcing.CostEntry, error) {
	return u.costs, nil
}
func (*unitUseCases) CostHistory(context.Context, costoutsourcing.Actor, string) ([]costoutsourcing.CostRevision, error) {
	return nil, nil
}
func (*unitUseCases) CorrectCost(context.Context, costoutsourcing.Actor, string, costoutsourcing.CorrectCostInput) (costoutsourcing.CostEntry, error) {
	return costoutsourcing.CostEntry{}, nil
}
func (*unitUseCases) WithdrawCost(context.Context, costoutsourcing.Actor, string, costoutsourcing.WithdrawInput) (costoutsourcing.CostEntry, error) {
	return costoutsourcing.CostEntry{}, nil
}
func (*unitUseCases) RecordEngagement(context.Context, costoutsourcing.Actor, costoutsourcing.RecordEngagementInput) (costoutsourcing.Engagement, error) {
	return costoutsourcing.Engagement{}, nil
}
func (*unitUseCases) GetEngagement(context.Context, costoutsourcing.Actor, string) (costoutsourcing.Engagement, error) {
	return costoutsourcing.Engagement{}, costoutsourcing.ErrNotFound
}
func (u *unitUseCases) ListEngagements(context.Context, costoutsourcing.Actor, string) ([]costoutsourcing.Engagement, error) {
	return u.engagements, nil
}
func (*unitUseCases) EngagementHistory(context.Context, costoutsourcing.Actor, string) ([]costoutsourcing.EngagementRevision, error) {
	return nil, nil
}
func (*unitUseCases) CorrectEngagement(context.Context, costoutsourcing.Actor, string, costoutsourcing.CorrectEngagementInput) (costoutsourcing.Engagement, error) {
	return costoutsourcing.Engagement{}, nil
}
func (*unitUseCases) WithdrawEngagement(context.Context, costoutsourcing.Actor, string, costoutsourcing.WithdrawInput) (costoutsourcing.Engagement, error) {
	return costoutsourcing.Engagement{}, nil
}
func (*unitUseCases) Assess(context.Context, costoutsourcing.Actor, string, costoutsourcing.AssessInput) (costoutsourcing.Assessment, error) {
	return costoutsourcing.Assessment{}, nil
}
func (*unitUseCases) AssessmentHistory(context.Context, costoutsourcing.Actor, string) ([]costoutsourcing.Assessment, error) {
	return nil, nil
}
func (u *unitUseCases) ListLatestAssessments(context.Context, costoutsourcing.Actor, string) ([]costoutsourcing.Assessment, error) {
	return u.assessments, nil
}
func (*unitUseCases) ProposePayableSuggestion(context.Context, costoutsourcing.Actor, costoutsourcing.ProposeSuggestionInput) (costoutsourcing.PayableSuggestion, error) {
	return costoutsourcing.PayableSuggestion{}, nil
}
func (*unitUseCases) GetPayableSuggestion(context.Context, costoutsourcing.Actor, string) (costoutsourcing.PayableSuggestion, error) {
	return costoutsourcing.PayableSuggestion{}, costoutsourcing.ErrNotFound
}
func (u *unitUseCases) ListPayableSuggestions(context.Context, costoutsourcing.Actor, string) ([]costoutsourcing.PayableSuggestion, error) {
	return u.suggestions, nil
}
func (*unitUseCases) PayableSuggestionHistory(context.Context, costoutsourcing.Actor, string) ([]costoutsourcing.PayableSuggestionRevision, error) {
	return nil, nil
}
func (*unitUseCases) SynchronizePayableSuggestion(context.Context, costoutsourcing.Actor, string, costoutsourcing.SuggestionCommandInput) (costoutsourcing.PayableSuggestion, error) {
	return costoutsourcing.PayableSuggestion{}, nil
}
func (*unitUseCases) WithdrawPayableSuggestion(context.Context, costoutsourcing.Actor, string, costoutsourcing.SuggestionCommandInput) (costoutsourcing.PayableSuggestion, error) {
	return costoutsourcing.PayableSuggestion{}, nil
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
	return auth.Principal{SubjectID: unitSubject, TenantID: unitTenant, DisplayName: "C6 tester", AllowedActions: allowed, ScopeKind: "resources", Resources: map[string]map[string]bool{"project": {unitProject: true}}}
}

func TestCostOutsourcingWorkspaceIsHumanUsableAndServerDriven(t *testing.T) {
	useCases := &unitUseCases{costs: []costoutsourcing.CostEntry{{ID: "44444444-4444-4444-8444-444444444444", TenantID: unitTenant, Project: costoutsourcing.ProjectSnapshot{ID: unitProject}, Description: "现场车辆成本", State: costoutsourcing.StateRecorded, Revision: 1}}}
	server, err := New(useCases, &unitAuthenticator{principal: unitPrincipal(auth.ActionCostRead, auth.ActionCostCreate, auth.ActionCostAmountRead)})
	if err != nil {
		t.Fatal(err)
	}
	page := httptest.NewRecorder()
	server.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "成本与外协") || !strings.Contains(page.Body.String(), "业务记录") || !strings.Contains(page.Body.String(), "assets/cost-outsourcing.js") {
		t.Fatalf("page status=%d body=%q", page.Code, page.Body.String())
	}
	if csp := page.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") || !strings.Contains(csp, "style-src 'unsafe-inline'") || !strings.Contains(csp, "connect-src 'self'") {
		t.Fatalf("workspace CSP does not permit its own script/style/API: %q", csp)
	}
	asset := httptest.NewRecorder()
	server.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/assets/cost-outsourcing.js", nil))
	for _, marker := range []string{"allowed_actions", "corrections", "withdrawal", "assessments", "payable-suggestions", "synchronizeSuggestion", "sessionStorage"} {
		if asset.Code != http.StatusOK || !strings.Contains(asset.Body.String(), marker) {
			t.Fatalf("asset missing %q status=%d", marker, asset.Code)
		}
	}
	list := httptest.NewRecorder()
	server.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/v1/cost-entries?project_id="+unitProject, nil))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"allowed_actions":["cost-entry.create"`) || !strings.Contains(list.Body.String(), `"description":"现场车辆成本"`) {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	engagementServer, err := New(&unitUseCases{
		engagements: []costoutsourcing.Engagement{{ID: "66666666-6666-4666-8666-666666666666", TenantID: unitTenant, Project: costoutsourcing.ProjectSnapshot{ID: unitProject, Revision: 3}, Provider: costoutsourcing.PartySnapshot{ID: "77777777-7777-4777-8777-777777777777", Revision: 2, LegalName: "测试受托方"}, ContactName: "李工", ContactPhone: "13800000000", Scope: "测量协作", Currency: "CNY", AmountMinor: 1200, State: costoutsourcing.StateRecorded, Revision: 1}},
		assessments: []costoutsourcing.Assessment{{ID: "88888888-8888-4888-8888-888888888888", TenantID: unitTenant, EngagementID: "66666666-6666-4666-8666-666666666666", EngagementRevision: 1, RuleKey: costoutsourcing.AssessmentRuleReferenceOAFivePointV1, Score: 5, Revision: 1}},
	}, &unitAuthenticator{principal: unitPrincipal(auth.ActionEngagementRead, auth.ActionEngagementAmountRead, auth.ActionEngagementHistory, auth.ActionSuggestionRead)})
	if err != nil {
		t.Fatal(err)
	}
	engagements := httptest.NewRecorder()
	engagementServer.ServeHTTP(engagements, httptest.NewRequest(http.MethodGet, "/api/v1/outsourcing-engagements?project_id="+unitProject, nil))
	if engagements.Code != http.StatusOK || !strings.Contains(engagements.Body.String(), `"contact_name":"李工"`) || !strings.Contains(engagements.Body.String(), `"contact_phone":"13800000000"`) || !strings.Contains(engagements.Body.String(), `"suggestion_allowed_actions":["outsourcing-payable-suggestion.read"]`) {
		t.Fatalf("engagement list status=%d body=%s", engagements.Code, engagements.Body.String())
	}
	latest := httptest.NewRecorder()
	engagementServer.ServeHTTP(latest, httptest.NewRequest(http.MethodGet, "/api/v1/outsourcing-engagement-assessments?project_id="+unitProject, nil))
	if latest.Code != http.StatusOK || !strings.Contains(latest.Body.String(), `"engagement_id":"66666666-6666-4666-8666-666666666666"`) || !strings.Contains(latest.Body.String(), `"score":5`) {
		t.Fatalf("latest assessment status=%d body=%s", latest.Code, latest.Body.String())
	}
	suggestionServer, err := New(&unitUseCases{suggestions: []costoutsourcing.PayableSuggestion{{ID: "55555555-5555-4555-8555-555555555555", TenantID: unitTenant, Project: costoutsourcing.ProjectSnapshot{ID: unitProject}, Purpose: "外协付款", State: costoutsourcing.SuggestionProposed, Revision: 1}}}, &unitAuthenticator{principal: unitPrincipal(auth.ActionSuggestionRead, auth.ActionSuggestionCreate, auth.ActionSuggestionWithdraw)})
	if err != nil {
		t.Fatal(err)
	}
	suggestions := httptest.NewRecorder()
	suggestionServer.ServeHTTP(suggestions, httptest.NewRequest(http.MethodGet, "/api/v1/payable-suggestions?project_id="+unitProject, nil))
	if suggestions.Code != http.StatusOK || !strings.Contains(suggestions.Body.String(), `"allowed_actions":["outsourcing-payable-suggestion.create","outsourcing-payable-suggestion.read","outsourcing-payable-suggestion.withdraw"]`) || !strings.Contains(suggestions.Body.String(), `"purpose":"外协付款"`) {
		t.Fatalf("suggestions status=%d body=%s", suggestions.Code, suggestions.Body.String())
	}
}
