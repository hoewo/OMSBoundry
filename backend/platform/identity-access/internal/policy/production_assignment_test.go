package policy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"omsbizfoundry/identity-access/internal/identity"
)

func TestProductionOperationsPolicyAllowsOnlyCurrentProductionResponsibility(t *testing.T) {
	now := time.Date(2026, 9, 27, 16, 0, 0, 0, time.UTC)
	otherProject := "88888888-8888-4888-8888-888888888888"
	otherTenant := "99999999-9999-4999-8999-999999999999"
	tests := []struct {
		name               string
		role               string
		status             string
		assignmentProject  string
		assignmentTenant   string
		assignmentRevision int64
		endedAt            *time.Time
		scopeProject       string
		principalTenant    string
		wantAllowed        bool
	}{
		{name: "active production owner", role: "production_owner", status: "active", assignmentProject: testProject, assignmentTenant: testTenant, assignmentRevision: 2, scopeProject: testProject, principalTenant: testTenant, wantAllowed: true},
		{name: "business owner is not production authority", role: "business_owner", status: "active", assignmentProject: testProject, assignmentTenant: testTenant, assignmentRevision: 2, scopeProject: testProject, principalTenant: testTenant},
		{name: "adjacent project is denied", role: "production_owner", status: "active", assignmentProject: testProject, assignmentTenant: testTenant, assignmentRevision: 2, scopeProject: otherProject, principalTenant: testTenant},
		{name: "wrong tenant is denied", role: "production_owner", status: "active", assignmentProject: testProject, assignmentTenant: testTenant, assignmentRevision: 2, scopeProject: testProject, principalTenant: otherTenant},
		{name: "stale assignment is denied", role: "production_owner", status: "active", assignmentProject: testProject, assignmentTenant: testTenant, assignmentRevision: 3, scopeProject: testProject, principalTenant: testTenant},
		{name: "ended assignment is denied", role: "production_owner", status: "active", assignmentProject: testProject, assignmentTenant: testTenant, assignmentRevision: 2, endedAt: ptrTime(now.Add(-time.Minute)), scopeProject: testProject, principalTenant: testTenant},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				fact := map[string]any{
					"id": testAssignment, "tenant_id": test.assignmentTenant, "project_id": test.assignmentProject,
					"resource_type": "project-assignment", "subject_kind": "person", "subject_id": testPerson,
					"source_revision": 4, "role_key": test.role, "status": test.status, "revision": test.assignmentRevision,
					"effective_from": now.Add(-time.Hour), "display_name": "项目职责",
				}
				if test.endedAt != nil {
					fact["ended_at"] = test.endedAt
				}
				_ = json.NewEncoder(w).Encode(fact)
			}))
			defer project.Close()
			people := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"schema_version": "1", "source_application": "people-groups", "fact_kind": "person-iam-subject-link",
					"id": testPerson, "tenant_id": testTenant, "status": "active", "revision": 4, "iam_subject_id": testSubject,
				})
			}))
			defer people.Close()

			policyPath := filepath.Join("..", "..", "..", "..", "..", "contracts", "access", "v1", "examples", "production-operations-project-assignment-policy.json")
			evaluator, err := NewProjectAssignmentEvaluator(Config{
				ProjectBaseURL: project.URL, PeopleBaseURL: people.URL, ServiceSubjectID: testService,
				TenantID: testTenant, Issuer: "https://oms.example.test/auth", PolicyPaths: []string{policyPath}, Timeout: time.Second,
			}, staticSigner{})
			if err != nil {
				t.Fatal(err)
			}
			evaluator.now = func() time.Time { return now }
			decision, err := evaluator.Evaluate(context.Background(), identity.Principal{SubjectID: testSubject, TenantID: test.principalTenant}, "production-operations", identity.Scope{Kind: "resources", Resources: []identity.ResourceRef{{Type: "project", ID: test.scopeProject}}}, identity.BusinessFactReference{Kind: "project_assignment", Source: "project-workspace", ID: testAssignment, Revision: 2})
			if test.wantAllowed {
				if err != nil {
					t.Fatal(err)
				}
				if decision.PolicyID != "production-operations-project-access" || decision.PolicyVersion != "1.0.0" || decision.FactRevision != 2 || len(decision.Actions) != 7 {
					t.Fatalf("unexpected decision: %#v", decision)
				}
				return
			}
			var denied *identity.AuthorizationError
			if !errors.As(err, &denied) {
				t.Fatalf("decision=%#v error=%v, want AuthorizationError", decision, err)
			}
		})
	}
}

func ptrTime(value time.Time) *time.Time { return &value }
