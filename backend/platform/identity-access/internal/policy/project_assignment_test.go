package policy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"omsbizfoundry/identity-access/internal/identity"
)

const (
	testTenant     = "22222222-2222-4222-8222-222222222222"
	testService    = "33333333-3333-4333-8333-333333333333"
	testSubject    = "44444444-4444-4444-8444-444444444444"
	testProject    = "55555555-5555-4555-8555-555555555555"
	testAssignment = "66666666-6666-4666-8666-666666666666"
	testPerson     = "77777777-7777-4777-8777-777777777777"
)

type staticSigner struct{}

func (staticSigner) Sign(any) (string, error) { return "service-token", nil }

func TestProjectAssignmentPolicyUsesVersionedProjectAndPeopleFacts(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	project := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/policy-facts/projects/"+testProject+"/assignments/"+testAssignment || r.Header.Get("Authorization") != "Bearer service-token" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": testAssignment, "tenant_id": testTenant, "project_id": testProject,
			"resource_type": "project-assignment", "subject_kind": "person", "subject_id": testPerson,
			"source_revision": 4, "role_key": "business_owner", "status": "active", "revision": 2,
			"effective_from": now.Add(-time.Hour), "display_name": "业务负责人",
		})
	}))
	defer project.Close()
	people := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/policy-facts/persons/"+testPerson || r.Header.Get("Authorization") != "Bearer service-token" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"schema_version": "1", "source_application": "people-groups", "fact_kind": "person-iam-subject-link",
			"id": testPerson, "tenant_id": testTenant, "status": "active", "revision": 4, "iam_subject_id": testSubject,
		})
	}))
	defer people.Close()

	evaluator := newTestEvaluator(t, project.URL, people.URL)
	evaluator.now = func() time.Time { return now }
	decision, err := evaluator.Evaluate(context.Background(), identity.Principal{SubjectID: testSubject, TenantID: testTenant}, "project-workspace", projectScope(), identity.BusinessFactReference{Kind: "project_assignment", Source: "project-workspace", ID: testAssignment, Revision: 2})
	if err != nil {
		t.Fatal(err)
	}
	if decision.PolicyID != "project-assignment-access" || decision.PolicyVersion != "1.0.0" || decision.FactRevision != 2 || len(decision.Actions) != 2 || decision.Actions[0] != "project-workspace.read" || decision.Actions[1] != "project-workspace.update" {
		t.Fatalf("unexpected decision: %#v", decision)
	}
}

func TestProjectAssignmentPolicyRejectsStaleRevision(t *testing.T) {
	project := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": testAssignment, "tenant_id": testTenant, "project_id": testProject,
			"resource_type": "project-assignment", "subject_kind": "person", "subject_id": testPerson,
			"source_revision": 4, "role_key": "business_owner", "status": "active", "revision": 3,
			"effective_from": time.Now().UTC().Add(-time.Hour), "display_name": "业务负责人",
		})
	}))
	defer project.Close()
	people := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("stale assignment must be rejected before reading People")
	}))
	defer people.Close()

	evaluator := newTestEvaluator(t, project.URL, people.URL)
	_, err := evaluator.Evaluate(context.Background(), identity.Principal{SubjectID: testSubject, TenantID: testTenant}, "project-workspace", projectScope(), identity.BusinessFactReference{Kind: "project_assignment", Source: "project-workspace", ID: testAssignment, Revision: 2})
	var denied *identity.AuthorizationError
	if !errors.As(err, &denied) {
		t.Fatalf("error = %v, want AuthorizationError", err)
	}
}

func TestProjectAssignmentPolicyReportsUnavailableDependency(t *testing.T) {
	project := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
	}))
	defer project.Close()
	people := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("Project outage must stop policy evaluation before reading People")
	}))
	defer people.Close()

	evaluator := newTestEvaluator(t, project.URL, people.URL)
	_, err := evaluator.Evaluate(context.Background(), identity.Principal{SubjectID: testSubject, TenantID: testTenant}, "project-workspace", projectScope(), identity.BusinessFactReference{Kind: "project_assignment", Source: "project-workspace", ID: testAssignment, Revision: 2})
	var unavailable *identity.DependencyUnavailableError
	if !errors.As(err, &unavailable) || unavailable.Dependency != "project-workspace" {
		t.Fatalf("error = %v, want Project dependency unavailable", err)
	}
}

func TestProjectAssignmentPolicySelectsTargetApplicationPolicy(t *testing.T) {
	now := time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC)
	project := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": testAssignment, "tenant_id": testTenant, "project_id": testProject,
			"resource_type": "project-assignment", "subject_kind": "person", "subject_id": testPerson,
			"source_revision": 4, "role_key": "business_owner", "status": "active", "revision": 2,
			"effective_from": now.Add(-time.Hour), "display_name": "业务负责人",
		})
	}))
	defer project.Close()
	people := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"schema_version": "1", "source_application": "people-groups", "fact_kind": "person-iam-subject-link",
			"id": testPerson, "tenant_id": testTenant, "status": "active", "revision": 4, "iam_subject_id": testSubject,
		})
	}))
	defer people.Close()

	directory := t.TempDir()
	projectPolicy := filepath.Join(directory, "project.json")
	engagementPolicy := filepath.Join(directory, "engagement.json")
	if err := os.WriteFile(projectPolicy, []byte(`{"schema_version":"1","policy_id":"project-assignment-access","policy_version":"1.0.0","audience":"project-workspace","roles":{"business_owner":["project-workspace.read"]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(engagementPolicy, []byte(`{"schema_version":"1","policy_id":"business-engagement-project-access","policy_version":"1.0.0","audience":"business-engagement","roles":{"business_owner":["business-engagement.read","business-engagement.method.select"]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	evaluator, err := NewProjectAssignmentEvaluator(Config{
		ProjectBaseURL: project.URL, PeopleBaseURL: people.URL, ServiceSubjectID: testService,
		TenantID: testTenant, Issuer: "https://oms.example.test/auth",
		PolicyPaths: []string{projectPolicy, engagementPolicy}, Timeout: time.Second,
	}, staticSigner{})
	if err != nil {
		t.Fatal(err)
	}
	evaluator.now = func() time.Time { return now }
	decision, err := evaluator.Evaluate(context.Background(), identity.Principal{SubjectID: testSubject, TenantID: testTenant}, "business-engagement", projectScope(), identity.BusinessFactReference{Kind: "project_assignment", Source: "project-workspace", ID: testAssignment, Revision: 2})
	if err != nil {
		t.Fatal(err)
	}
	if decision.PolicyID != "business-engagement-project-access" || len(decision.Actions) != 2 || decision.Actions[0] != "business-engagement.method.select" || decision.Actions[1] != "business-engagement.read" {
		t.Fatalf("unexpected Business Engagement decision: %#v", decision)
	}
}

func newTestEvaluator(t *testing.T, projectURL, peopleURL string) *ProjectAssignmentEvaluator {
	t.Helper()
	policyPath := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(policyPath, []byte(`{"schema_version":"1","policy_id":"project-assignment-access","policy_version":"1.0.0","audience":"project-workspace","roles":{"business_owner":["project-workspace.read","project-workspace.update"]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	evaluator, err := NewProjectAssignmentEvaluator(Config{ProjectBaseURL: projectURL, PeopleBaseURL: peopleURL, ServiceSubjectID: testService, TenantID: testTenant, Issuer: "https://oms.example.test/auth", PolicyPaths: []string{policyPath}, Timeout: time.Second}, staticSigner{})
	if err != nil {
		t.Fatal(err)
	}
	return evaluator
}

func projectScope() identity.Scope {
	return identity.Scope{Kind: "resources", Resources: []identity.ResourceRef{{Type: "project", ID: testProject}}}
}
