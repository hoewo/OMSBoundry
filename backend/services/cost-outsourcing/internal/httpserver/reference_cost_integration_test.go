//go:build integration

package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omsbizfoundry/cost-outsourcing/internal/auth"
	core "omsbizfoundry/cost-outsourcing/internal/costoutsourcing"
	rc "omsbizfoundry/cost-outsourcing/internal/referencecost"
)

type costOverlayResolver struct{ unavailable bool }

func (r *costOverlayResolver) ResolveProject(_ context.Context, tenant string, ref core.ProjectRef) (core.ProjectReference, error) {
	if r.unavailable {
		return core.ProjectReference{}, core.ErrReferenceUnavailable
	}
	return core.ProjectReference{TenantID: tenant, Status: "active", ProjectSnapshot: core.ProjectSnapshot{ID: ref.ID, Revision: ref.Revision, Code: "OA-COST-HTTP", DisplayName: "同一验收项目"}}, nil
}
func (*costOverlayResolver) ResolveEvidence(_ context.Context, tenant string, ref core.EvidenceRef) (core.EvidenceReference, error) {
	return core.EvidenceReference{TenantID: tenant, Status: "available", EvidenceSnapshot: core.EvidenceSnapshot{EvidenceRef: ref, Digest: "sha256:" + strings.Repeat("a", 64), DisplayName: "保留发票.png"}}, nil
}

func TestReferenceCostNativeHTTPExactIAMSideScopesReplayHistoryAndRedaction(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := httpIntegrationPool(t, ctx)
	defer cleanup()
	store, _ := rc.NewPostgresStore(pool)
	resolver := &costOverlayResolver{}
	service, _ := rc.New(store, resolver)
	provider, key := integrationIAMProvider(t)
	server, e := New(&unitUseCases{}, provider, service)
	if e != nil {
		t.Fatal(e)
	}
	h := httptest.NewServer(server)
	defer h.Close()
	base := h.URL + "/api/v1/reference-oa/cost-entries"
	token := integrationToken(t, key, "cost-outsourcing", httpTenant, auth.CostActions, rc.SideKind("business"), httpProject)
	in := rc.CreateInput{CommandID: httpCommand, ProjectRef: core.ProjectRef{ID: httpProject, Revision: 7}, Scope: "business", Fields: rc.Fields{Category: "vehicle", AmountMinor: "9223372036854775807", EvidenceRefs: []core.EvidenceRef{}}}
	body, _ := json.Marshal(in)
	for _, scope := range []string{"project", rc.SideKind("production")} {
		wrong := integrationToken(t, key, "cost-outsourcing", httpTenant, auth.CostActions, scope, httpProject)
		assertProblem(t, doHTTP(t, http.MethodPost, base, wrong, body), 403, "SCOPE_DENIED")
	}
	assertProblem(t, doHTTP(t, http.MethodPost, base, "", body), 401, "UNAUTHENTICATED")
	missingAmount := integrationToken(t, key, "cost-outsourcing", httpTenant, []string{auth.ActionCostCreate}, rc.SideKind("business"), httpProject)
	assertProblem(t, doHTTP(t, http.MethodPost, base, missingAmount, body), 403, "SCOPE_DENIED")
	response := doHTTP(t, http.MethodPost, base, token, body)
	if response.StatusCode != 201 {
		t.Fatalf("create=%d %s", response.StatusCode, readBody(response))
	}
	var first referenceCostView
	decodeResponse(t, response, &first)
	if first.AmountMinor == nil || *first.AmountMinor != "9223372036854775807" || first.OccurredOn != nil || first.Description != nil || first.Project.Revision != 7 {
		t.Fatalf("exact optional view=%#v", first)
	}
	resolver.unavailable = true
	response = doHTTP(t, http.MethodPost, base, token, body)
	var replay referenceCostView
	decodeResponse(t, response, &replay)
	if response.StatusCode != 201 || replay.ID != first.ID {
		t.Fatal("committed create did not replay")
	}
	changed := in
	changed.Fields.AmountMinor = "1"
	bad, _ := json.Marshal(changed)
	assertProblem(t, doHTTP(t, http.MethodPost, base, token, bad), 409, "COMMAND_PAYLOAD_CONFLICT")
	resolver.unavailable = false
	correction := rc.ChangeInput{CommandID: httpCommand2, ExpectedRevision: 1, Reason: "追加纠正", Fields: rc.Fields{Category: "vehicle", AmountMinor: "101", EvidenceRefs: []core.EvidenceRef{}}}
	body2, _ := json.Marshal(correction)
	response = doHTTP(t, http.MethodPost, base+"/"+first.ID+"/corrections", token, body2)
	var corrected referenceCostView
	decodeResponse(t, response, &corrected)
	if response.StatusCode != 200 || corrected.Revision != 2 || *corrected.AmountMinor != "101" {
		t.Fatal("native correction failed")
	}
	withdraw, _ := json.Marshal(rc.WithdrawInput{CommandID: httpCommand3, ExpectedRevision: 2, Reason: "原事实保留"})
	response = doHTTP(t, http.MethodPost, base+"/"+first.ID+"/withdrawal", token, withdraw)
	var withdrawn referenceCostView
	decodeResponse(t, response, &withdrawn)
	if response.StatusCode != 200 || withdrawn.Revision != 3 || withdrawn.State != "withdrawn" || *withdrawn.AmountMinor != "101" {
		t.Fatal("native withdrawal failed")
	}
	for _, action := range withdrawn.AllowedActions {
		if action == auth.ActionCostCorrect || action == auth.ActionCostWithdraw {
			t.Fatal("withdrawn write action exposed")
		}
	}
	resolver.unavailable = true
	response = doHTTP(t, http.MethodPost, base+"/"+first.ID+"/corrections", token, body2)
	decodeResponse(t, response, &replay)
	if response.StatusCode != 200 || replay.Revision != 2 {
		t.Fatal("correction replay after withdrawal failed")
	}
	readToken := integrationToken(t, key, "cost-outsourcing", httpTenant, []string{auth.ActionCostRead, auth.ActionCostHistory}, rc.ResourceType, first.ID)
	response = doHTTP(t, http.MethodGet, base+"/"+first.ID, readToken, nil)
	wire := readBody(response)
	if response.StatusCode != 200 || strings.Contains(wire, "amount_minor") || strings.Contains(wire, "currency") {
		t.Fatalf("native redaction failed: %s", wire)
	}
	response = doHTTP(t, http.MethodGet, base+"/"+first.ID+"/history", readToken, nil)
	wire = readBody(response)
	if response.StatusCode != 200 || strings.Contains(wire, "amount_minor") || strings.Contains(wire, "9223372036854775807") {
		t.Fatal("history leaked hidden amount")
	}
	response = doHTTP(t, http.MethodGet, base+"/"+first.ID+"/history", token, nil)
	var history struct {
		Items []struct {
			Snapshot referenceCostView `json:"snapshot"`
		} `json:"items"`
	}
	decodeResponse(t, response, &history)
	if len(history.Items) != 3 || *history.Items[2].Snapshot.AmountMinor != "9223372036854775807" || *history.Items[0].Snapshot.AmountMinor != "101" {
		t.Fatal("append history not retained")
	}
	opposite := integrationToken(t, key, "cost-outsourcing", httpTenant, []string{auth.ActionCostRead}, rc.SideKind("production"), httpProject)
	assertProblem(t, doHTTP(t, http.MethodGet, base+"/"+first.ID, opposite, nil), 403, "SCOPE_DENIED")
	var count int
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM reference_oa_cost_entries`).Scan(&count); e != nil || count != 1 {
		t.Fatalf("duplicate facts %d %v", count, e)
	}
	defaultServer, _ := New(&unitUseCases{}, provider)
	rr := httptest.NewRecorder()
	defaultServer.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/reference-oa/cost-entries", strings.NewReader(string(body))))
	if rr.Code != 404 && rr.Code != 405 {
		t.Fatalf("unselected overlay exposed status=%d", rr.Code)
	}
	rr = httptest.NewRecorder()
	defaultServer.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/reference-oa/cost-entries", nil))
	if rr.Code != 404 {
		t.Fatalf("unselected overlay read exposed status=%d", rr.Code)
	}
	// A JSON null cannot bypass the database snapshot identity/type checks.
	for _, field := range []string{"id", "tenant_id", "revision", "scope", "currency", "category"} {
		_, e = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO reference_oa_cost_entries SELECT tenant_id,'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa'::uuid,project_id,scope,state,revision,jsonb_set(jsonb_set(snapshot,'{id}','"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"'),'{%s}','null') FROM reference_oa_cost_entries`, field))
		if e == nil {
			t.Fatalf("null snapshot accepted: %s", field)
		}
	}
}
