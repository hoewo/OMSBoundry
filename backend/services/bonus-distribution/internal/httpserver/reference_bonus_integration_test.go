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
	"time"

	"omsbizfoundry/bonus-distribution/internal/bonus"
	rb "omsbizfoundry/bonus-distribution/internal/referencebonus"
	"omsbizfoundry/bonus-distribution/internal/references"
)

func TestManualBonusProtectedIAMHTTPPostgresLifecycleAndIsolation(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := httpIntegrationPool(t, ctx)
	defer cleanup()
	mode := &referenceMode{}
	mode.set(true, true)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		healthy, valid := mode.values()
		if !healthy {
			w.WriteHeader(503)
			return
		}
		if r.Header.Get("Authorization") != "Bearer exact-reference-token" {
			w.WriteHeader(401)
			return
		}
		role := "business_owner"
		if !valid {
			role = "participant"
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/references/projects/" + httpProject:
			fmt.Fprintf(w, `{"id":%q,"tenant_id":%q,"resource_type":"project","project_number":"HTTP-MANUAL","display_summary":"手工分配隔离验证","status":"active","revision":7}`, httpProject, httpTenant)
		case "/api/v1/projects/" + httpProject:
			fmt.Fprintf(w, `{"project":{"id":%q,"tenant_id":%q,"revision":7,"status":"active"},"assignments":[{"id":%q,"tenant_id":%q,"project_id":%q,"subject_kind":"person","subject_id":%q,"source_revision":5,"role_key":%q,"created_project_revision":4}]}`, httpProject, httpTenant, httpAssignment, httpTenant, httpProject, httpPerson, role)
		case "/api/v1/references/projects/" + httpProject + "/assignments/" + httpAssignment:
			fmt.Fprintf(w, `{"id":%q,"tenant_id":%q,"project_id":%q,"resource_type":"project-assignment","subject_kind":"person","subject_id":%q,"source_revision":5,"role_key":%q,"status":"active","revision":4,"effective_from":"2026-09-01T00:00:00Z","display_name":"分配参与人"}`, httpAssignment, httpTenant, httpProject, httpPerson, role)
		case "/api/v1/references/persons/" + httpPerson:
			fmt.Fprintf(w, `{"schema_version":"1","source_application":"people-groups","id":%q,"tenant_id":%q,"resource_type":"person","display_name":"分配参与人","status":"active","revision":5}`, httpPerson, httpTenant)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	credentials := &httpCredentials{}
	endpoint := func(aud, action string) references.Endpoint {
		return references.Endpoint{BaseURL: upstream.URL, Audience: aud, Action: action, Credentials: credentials}
	}
	resolver, e := references.NewHTTPResolver(time.Second, endpoint("project-workspace", "project-workspace.read"), endpoint("people-groups", "people-groups.person.reference.read"), endpoint("contract-ledger", "contract-ledger.reference.read"), references.Endpoint{})
	if e != nil {
		t.Fatal(e)
	}
	store, e := rb.NewPostgresStore(pool)
	if e != nil {
		t.Fatal(e)
	}
	service, e := rb.New(store, resolver)
	if e != nil {
		t.Fatal(e)
	}
	provider, key := integrationIAMProvider(t)
	server, e := New(&unitUseCases{}, provider, WithReferenceBonus(service))
	if e != nil {
		t.Fatal(e)
	}
	host := httptest.NewServer(server)
	defer host.Close()
	disabled, e := New(&unitUseCases{}, provider)
	if e != nil {
		t.Fatal(e)
	}
	disabledHost := httptest.NewServer(disabled)
	defer disabledHost.Close()
	base := host.URL + "/api/v1/reference-oa/bonus-allocations"
	list := base + "?project_id=" + httpProject + "&scope=business"
	token := integrationToken(t, key, "bonus-distribution", httpTenant, rb.Actions, "business_bonus_project", httpProject)
	body := []byte(fmt.Sprintf(`{"command_id":%q,"project_ref":{"id":%q,"revision":7},"scope":"business","base_amount_minor":"101","occurred_on":"2026-10-04","description":"原组件手工登记","items":[{"person_ref":{"id":%q,"revision":5},"assignment_ref":{"id":%q,"revision":4},"method":"ratio","ratio_basis_points":5000,"fixed_amount_minor":"0"}]}`, httpCommand, httpProject, httpPerson, httpAssignment))
	counts := func(records, revisions, commands, audits int) {
		t.Helper()
		var actual [4]int
		e := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM reference_oa_bonus_allocations),(SELECT count(*) FROM reference_oa_bonus_revisions),(SELECT count(*) FROM reference_oa_bonus_commands),(SELECT count(*) FROM reference_oa_bonus_audit)`).Scan(&actual[0], &actual[1], &actual[2], &actual[3])
		if e != nil {
			t.Fatal(e)
		}
		if actual != [4]int{records, revisions, commands, audits} {
			t.Fatalf("manual counts=%v", actual)
		}
		assertCounts(t, pool, 0, 0, 0, 0, 0, 0)
	}
	response := doHTTP(t, "GET", disabledHost.URL+"/api/v1/reference-oa/bonus-allocations", token, nil)
	if response.StatusCode != 404 {
		t.Fatalf("unselected overlay status=%d", response.StatusCode)
	}
	response.Body.Close()
	response = doHTTP(t, "POST", base, "", body)
	if response.StatusCode != 401 {
		t.Fatalf("anonymous=%d", response.StatusCode)
	}
	response.Body.Close()
	for _, tc := range []struct {
		name, audience, tenant, kind, id string
		actions                          []string
	}{
		{"wrong project", "bonus-distribution", httpTenant, "business_bonus_project", httpProjectB, rb.Actions},
		{"wrong side", "bonus-distribution", httpTenant, "production_bonus_project", httpProject, rb.Actions},
		{"legacy generic scope", "bonus-distribution", httpTenant, "project", httpProject, rb.Actions},
		{"old capability namespace", "bonus-distribution", httpTenant, "business_bonus_project", httpProject, []string{bonus.ActionAllocationManage, bonus.ActionAllocationAmountRead}},
		{"amount denied", "bonus-distribution", httpTenant, "business_bonus_project", httpProject, []string{rb.ActionRead, rb.ActionRecord}},
		{"wrong audience", "project-workspace", httpTenant, "business_bonus_project", httpProject, rb.Actions},
		{"wrong tenant", "bonus-distribution", httpTenantB, "business_bonus_project", httpProject, rb.Actions},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := doHTTP(t, "POST", base, integrationToken(t, key, tc.audience, tc.tenant, tc.actions, tc.kind, tc.id), body)
			defer r.Body.Close()
			if r.StatusCode != 401 && r.StatusCode != 403 {
				t.Fatalf("denied status=%d", r.StatusCode)
			}
			counts(0, 0, 0, 0)
		})
	}
	mode.set(false, true)
	assertProblem(t, doHTTP(t, "POST", base, token, body), 503, "REFERENCE_SERVICE_UNAVAILABLE")
	counts(0, 0, 0, 0)
	mode.set(true, false)
	assertProblem(t, doHTTP(t, "POST", base, token, body), 422, "REFERENCE_OR_POLICY_INVALID")
	counts(0, 0, 0, 0)
	mode.set(true, true)
	response = doHTTP(t, "GET", base+"/candidates?project_id="+httpProject+"&project_revision=7&scope=business", token, nil)
	if response.StatusCode != 200 {
		t.Fatalf("candidates=%d %s", response.StatusCode, readBody(response))
	}
	var candidates struct {
		Items []bonus.ParticipantReference `json:"items"`
	}
	decodeResponse(t, response, &candidates)
	if len(candidates.Items) != 1 || candidates.Items[0].Assignment.RoleKey != "business_owner" {
		t.Fatalf("candidates=%+v", candidates)
	}
	response = doHTTP(t, "POST", base, token, body)
	if response.StatusCode != 201 {
		t.Fatalf("create=%d %s", response.StatusCode, readBody(response))
	}
	var created struct {
		Items []manualBonusView `json:"items"`
	}
	decodeResponse(t, response, &created)
	if len(created.Items) != 1 || created.Items[0].AmountMinor == nil || *created.Items[0].AmountMinor != "51" {
		t.Fatalf("created=%+v", created)
	}
	id := created.Items[0].ID
	counts(1, 1, 1, 1)
	mode.set(false, true)
	calls := mode.count()
	response = doHTTP(t, "POST", base, token, body)
	if response.StatusCode != 201 {
		t.Fatalf("replay=%d", response.StatusCode)
	}
	var replay struct {
		Items []manualBonusView `json:"items"`
	}
	decodeResponse(t, response, &replay)
	if len(replay.Items) != 1 || replay.Items[0].ID != id || mode.count() != calls {
		t.Fatal("replay changed facts or consulted mutable upstream")
	}
	counts(1, 1, 1, 1)
	redacted := integrationToken(t, key, "bonus-distribution", httpTenant, []string{rb.ActionRead, rb.ActionHistory}, "business_bonus_project", httpProject)
	for _, url := range []string{list, base + "/" + id, base + "/" + id + "/history"} {
		r := doHTTP(t, "GET", url, redacted, nil)
		if r.StatusCode != 200 {
			t.Fatalf("redacted=%d", r.StatusCode)
		}
		b := readBody(r)
		for _, field := range []string{`"amount_minor"`, `"base_amount_minor"`, `"fixed_amount_minor"`, `"ratio_basis_points"`, `"currency"`} {
			if strings.Contains(b, field) {
				t.Fatalf("redaction leaked %s", field)
			}
		}
	}
	assertProblem(t, doHTTP(t, "GET", base+"/"+id, integrationToken(t, key, "bonus-distribution", httpTenant, rb.Actions, "production_bonus_project", httpProject), nil), 403, "SCOPE_DENIED")
	withdrawal := []byte(fmt.Sprintf(`{"command_id":%q,"expected_revision":1,"reason":"撤回测试，保留事实"}`, httpCommand2))
	response = doHTTP(t, "POST", base+"/"+id+"/withdrawal", token, withdrawal)
	if response.StatusCode != 200 {
		t.Fatalf("withdraw=%d %s", response.StatusCode, readBody(response))
	}
	var withdrawn manualBonusView
	decodeResponse(t, response, &withdrawn)
	if withdrawn.State != "withdrawn" || withdrawn.Revision != 2 || containsString(withdrawn.AllowedActions, rb.ActionWithdraw) {
		t.Fatalf("withdrawn=%+v", withdrawn)
	}
	counts(1, 2, 2, 2)
	response = doHTTP(t, "GET", base+"/"+id+"/history", token, nil)
	var history struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	decodeResponse(t, response, &history)
	if len(history.Items) != 2 {
		t.Fatalf("history=%+v", history)
	}
	// The caller's exact downstream reference intents stay native; no source money
	// request is made by this manually declared overlay.
	credentials.mu.Lock()
	defer credentials.mu.Unlock()
	if len(credentials.calls) == 0 {
		t.Fatal("missing downstream credentials")
	}
	for _, call := range credentials.calls {
		if strings.HasPrefix(call, "contract-ledger:") || strings.HasPrefix(call, "money-flow-ledger:") {
			t.Fatalf("manual source crossed financial boundary: %s", call)
		}
	}
}
