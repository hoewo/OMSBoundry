package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omsbizfoundry/cost-outsourcing/internal/auth"
	"omsbizfoundry/cost-outsourcing/internal/costoutsourcing"
)

const materialResourceID = "44444444-4444-4444-8444-444444444444"

type materialUseCases struct {
	unitUseCases
	cost       costoutsourcing.CostEntry
	engagement costoutsourcing.Engagement
	calls      int
	actor      costoutsourcing.Actor
}

func (u *materialUseCases) GetCost(_ context.Context, actor costoutsourcing.Actor, _ string) (costoutsourcing.CostEntry, error) {
	u.calls++
	u.actor = actor
	return u.cost, nil
}
func (u *materialUseCases) GetEngagement(_ context.Context, actor costoutsourcing.Actor, _ string) (costoutsourcing.Engagement, error) {
	u.calls++
	u.actor = actor
	return u.engagement, nil
}

type materialUnauthenticated struct{}

func (*materialUnauthenticated) ProviderName() string { return "test" }
func (*materialUnauthenticated) Authenticate(*http.Request) (auth.Principal, error) {
	return auth.Principal{}, auth.ErrUnauthenticated
}

func TestMaterialReferenceHTTPAggregateActionsScopesAndRedaction(t *testing.T) {
	for _, aggregate := range []struct{ path, kind, read, amount, other string }{
		{"cost-entries", "cost_entry", auth.ActionCostRead, auth.ActionCostAmountRead, auth.ActionEngagementRead},
		{"outsourcing-engagements", "outsourcing_engagement", auth.ActionEngagementRead, auth.ActionEngagementAmountRead, auth.ActionCostRead},
	} {
		for _, test := range []struct {
			name          string
			status, calls int
		}{
			{"project", 200, 1}, {"exact", 200, 1}, {"tenant", 200, 1}, {"neighbor", 403, 1}, {"amount-only", 403, 0}, {"other-read", 403, 0}, {"anonymous", 401, 0}, {"bad-tenant", 422, 1}, {"bad-id", 422, 1}, {"bad-file", 422, 1},
		} {
			t.Run(aggregate.kind+"/"+test.name, func(t *testing.T) {
				files := []costoutsourcing.EvidenceSnapshot{{EvidenceRef: costoutsourcing.EvidenceRef{FileID: "55555555-5555-4555-8555-555555555555", Version: 2, Slot: "receipt"}, DisplayName: "凭证.pdf"}}
				u := &materialUseCases{cost: costoutsourcing.CostEntry{ID: materialResourceID, TenantID: unitTenant, Project: costoutsourcing.ProjectSnapshot{ID: unitProject}, State: costoutsourcing.StateRecorded, Revision: 3, Evidence: files, AmountMinor: 9999}, engagement: costoutsourcing.Engagement{ID: materialResourceID, TenantID: unitTenant, Project: costoutsourcing.ProjectSnapshot{ID: unitProject}, State: costoutsourcing.StateRecorded, Revision: 3, Evidence: files, AmountMinor: 9999}}
				p := unitPrincipal(aggregate.read)
				switch test.name {
				case "exact":
					p.Resources = map[string]map[string]bool{aggregate.kind: {materialResourceID: true}}
				case "tenant":
					p.ScopeKind, p.Resources = "tenant", nil
				case "neighbor":
					p.Resources = map[string]map[string]bool{aggregate.kind: {unitSubject: true}}
				case "amount-only":
					p.AllowedActions = map[string]bool{aggregate.amount: true}
				case "other-read":
					p.AllowedActions = map[string]bool{aggregate.other: true}
				case "bad-tenant":
					u.cost.TenantID, u.engagement.TenantID = unitSubject, unitSubject
				case "bad-id":
					u.cost.ID, u.engagement.ID = unitSubject, unitSubject
				case "bad-file":
					files[0].Version = 0
				}
				var provider auth.Authenticator = &unitAuthenticator{principal: p}
				if test.name == "anonymous" {
					provider = &materialUnauthenticated{}
				}
				server, err := New(u, provider)
				if err != nil {
					t.Fatal(err)
				}
				response := httptest.NewRecorder()
				request := httptest.NewRequest("GET", "/api/v1/references/"+aggregate.path+"/"+materialResourceID+"/materials", nil)
				request.Header.Set("X-Tenant-ID", "forged")
				request.Header.Set("X-Actor-ID", "forged")
				server.ServeHTTP(response, request)
				if response.Code != test.status || u.calls != test.calls || response.Header().Get("Cache-Control") != "no-store" {
					t.Fatalf("status=%d calls=%d body=%s", response.Code, u.calls, response.Body.String())
				}
				if test.status == 200 {
					var value costoutsourcing.MaterialReferenceProjection
					if json.Unmarshal(response.Body.Bytes(), &value) != nil || value.ResourceType != aggregate.kind || value.TenantID != unitTenant || value.Revision != 3 || len(value.Items) != 1 || u.actor.TenantID != unitTenant || u.actor.ID != unitSubject {
						t.Fatal("wrong identity or projection")
					}
					for _, forbidden := range []string{"amount", "currency", "provider", "contract", "object_key", "url", "credential"} {
						if strings.Contains(response.Body.String(), forbidden) {
							t.Fatalf("leaked %q", forbidden)
						}
					}
				}
			})
		}
	}
}
