package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omsbizfoundry/bonus-distribution/internal/auth"
	"omsbizfoundry/bonus-distribution/internal/bonus"
)

const (
	unitTenant  = "11111111-1111-4111-8111-111111111111"
	unitSubject = "22222222-2222-4222-8222-222222222222"
	unitProject = "33333333-3333-4333-8333-333333333333"
	unitPool    = "44444444-4444-4444-8444-444444444444"
)

type unitUseCases struct{ pools []bonus.BonusPool }

func (*unitUseCases) Ping(context.Context) error { return nil }
func (*unitUseCases) CreateBusinessDraft(context.Context, bonus.Actor, bonus.CreateBusinessDraftInput) (bonus.BonusPool, error) {
	return bonus.BonusPool{}, nil
}
func (*unitUseCases) CreateProductionDraft(context.Context, bonus.Actor, bonus.CreateProductionDraftInput) (bonus.BonusPool, error) {
	return bonus.BonusPool{}, nil
}
func (*unitUseCases) Publish(context.Context, bonus.Actor, string, bonus.PoolCommandInput) (bonus.BonusPool, error) {
	return bonus.BonusPool{}, nil
}
func (*unitUseCases) Supersede(context.Context, bonus.Actor, bonus.SupersedeInput) (bonus.SupersessionResult, error) {
	return bonus.SupersessionResult{}, nil
}
func (*unitUseCases) Withdraw(context.Context, bonus.Actor, string, bonus.PoolCommandInput) (bonus.BonusPool, error) {
	return bonus.BonusPool{}, nil
}
func (*unitUseCases) GetPool(context.Context, bonus.Actor, string) (bonus.BonusPool, error) {
	return bonus.BonusPool{}, bonus.ErrNotFound
}
func (u *unitUseCases) ListPools(context.Context, bonus.Actor, string) ([]bonus.BonusPool, error) {
	return u.pools, nil
}
func (*unitUseCases) PoolHistory(context.Context, bonus.Actor, string) ([]bonus.PoolRevision, error) {
	return nil, nil
}
func (*unitUseCases) AllocationHistory(context.Context, bonus.Actor, string) ([]bonus.AllocationRevision, error) {
	return nil, nil
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
		SubjectID: unitSubject, TenantID: unitTenant, DisplayName: "C7 tester",
		AllowedActions: allowed, ScopeKind: "resources",
		Resources: map[string]map[string]bool{"project": {unitProject: true}},
	}
}

func TestBonusWorkspaceIsHumanUsableAndServerDriven(t *testing.T) {
	pool := bonus.BonusPool{
		ID: unitPool, TenantID: unitTenant, BonusType: bonus.TypeBusiness, State: bonus.StateDraft,
		Project:  bonus.ProjectSnapshot{ID: unitProject, Revision: 7, DisplayName: "C7 项目"},
		Source:   bonus.SourceSnapshot{Type: bonus.SourceContractSettlement, ID: "55555555-5555-4555-8555-555555555555", Revision: 1, ProjectID: unitProject, Currency: "CNY", TotalAmountMinor: 106500, State: "recorded", Digest: "source-digest"},
		Policy:   bonus.PolicySnapshot{Key: "reference-oa-bonus-v1", Version: "1.0.0", BonusType: bonus.TypeBusiness, RateBasisPoints: 1000, Rounding: "half_up", Formula: "fixed formula", ParametersDigest: "policy-digest"},
		Currency: "CNY", AmountMinor: 10000, Revision: 1,
	}
	principal := unitPrincipal(
		bonus.ActionPoolRead, bonus.ActionPoolHistory, bonus.ActionAllocationRead, bonus.ActionAllocationAmountRead,
		bonus.ActionAllocationManage, bonus.ActionBusinessCalculate, bonus.ActionProductionCalculate, bonus.ActionBusinessPublish,
	)
	server, err := New(&unitUseCases{pools: []bonus.BonusPool{pool}}, &unitAuthenticator{principal: principal})
	if err != nil {
		t.Fatal(err)
	}

	page := httptest.NewRecorder()
	server.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	for _, marker := range []string{"奖金分配", "奖池工作区", "建立经营草稿", "assets/bonus-distribution.js"} {
		if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), marker) {
			t.Fatalf("page missing %q status=%d", marker, page.Code)
		}
	}
	if csp := page.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") || !strings.Contains(csp, "style-src 'unsafe-inline'") || !strings.Contains(csp, "connect-src 'self'") {
		t.Fatalf("workspace CSP does not permit its own script/style/API: %q", csp)
	}

	asset := httptest.NewRecorder()
	server.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/assets/bonus-distribution.js", nil))
	for _, marker := range []string{"allowed_actions", "supersession", "allocation-history", "本次没有产生部分奖池", "sessionStorage", "amount_minor"} {
		if asset.Code != http.StatusOK || !strings.Contains(asset.Body.String(), marker) {
			t.Fatalf("asset missing %q status=%d", marker, asset.Code)
		}
	}

	list := httptest.NewRecorder()
	server.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/v1/bonus-pools?project_id="+unitProject, nil))
	for _, marker := range []string{`"bonus_type":"business"`, `"amount_minor":10000`, `"allowed_actions":["bonus-pool.business.calculate"`, `"bonus-pool.business.publish"`} {
		if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), marker) {
			t.Fatalf("list missing %q status=%d body=%s", marker, list.Code, list.Body.String())
		}
	}
}
