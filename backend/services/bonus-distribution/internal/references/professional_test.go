package references

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	core "omsbizfoundry/bonus-distribution/internal/bonus"
	rb "omsbizfoundry/bonus-distribution/internal/referencebonus"
)

const profPerson = "33333333-3333-4333-8333-333333333333"
const profAssignment = "44444444-4444-4444-8444-444444444444"
const profDiscipline = "55555555-5555-4555-8555-555555555555"

type profCredential struct{ calls [][4]string }

func (c *profCredential) Credential(_ context.Context, audience, action, kind, id string) (string, error) {
	c.calls = append(c.calls, [4]string{audience, action, kind, id})
	return "native-reference-token", nil
}
func (*profCredential) Invalidate(string, string, string, string) {}

func professionalFixture(t *testing.T) (*HTTPResolver, *profCredential, map[string]any, map[string]any) {
	t.Helper()
	assignment := map[string]any{"schema_version": "1", "source_application": "project-workspace", "profile": rb.ProfessionalSource, "resource_type": professionalResource, "tenant_id": testTenant, "project_id": testProject, "project_revision": int64(9), "project_status": "active", "id": profAssignment, "revision": int64(5), "role_key": "designer", "person": map[string]any{"id": profPerson, "revision": int64(2), "name": "林设计"}, "discipline": map[string]any{"id": profDiscipline, "revision": int64(1), "name": "桥梁"}, "state": "active", "eligible": true, "participant_not_applicable": true}
	person := map[string]any{"schema_version": "1", "source_application": "people-groups", "resource_type": "person", "tenant_id": testTenant, "id": profPerson, "revision": int64(2), "display_name": "林设计", "status": "active"}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer native-reference-token" {
			t.Error("native credential missing")
		}
		switch r.URL.Path {
		case professionalPath(testProject) + "/" + profAssignment:
			_ = json.NewEncoder(w).Encode(assignment)
		case "/api/v1/references/persons/" + profPerson:
			_ = json.NewEncoder(w).Encode(person)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(s.Close)
	credentials := &profCredential{}
	endpoint := Endpoint{BaseURL: s.URL, Audience: "project-workspace", Action: "project-workspace.read", Credentials: credentials}
	people := endpoint
	people.Audience = "people-groups"
	people.Action = "people-groups.person.reference.read"
	resolver, err := NewHTTPResolver(time.Second, endpoint, people, endpoint, Endpoint{})
	if err != nil {
		t.Fatal(err)
	}
	endpoint.Action = "project-workspace.reference-oa-personnel.read"
	if err = resolver.WithProfessionalEndpoint(endpoint); err != nil {
		t.Fatal(err)
	}
	return resolver, credentials, assignment, person
}

func TestProfessionalResolverUsesDistinctNativeActionAndFrozenSourceDigests(t *testing.T) {
	r, c, _, _ := professionalFixture(t)
	v, err := r.ResolveProfessionalParticipant(context.Background(), testTenant, core.ProjectSnapshot{ID: testProject, Revision: 9}, core.PersonRef{ID: profPerson, Revision: 2}, core.AssignmentRef{ID: profAssignment, Revision: 5})
	if err != nil {
		t.Fatal(err)
	}
	if v.AssignmentSource != rb.ProfessionalSource || v.Discipline.ID != profDiscipline || v.Person.DisplayName != "林设计" || v.Person.Digest == v.Assignment.Digest || len(v.Person.Digest) != 71 {
		t.Fatalf("source snapshot incomplete: %+v", v)
	}
	want := [][4]string{{"project-workspace", "project-workspace.reference-oa-personnel.read", "project", testProject}, {"people-groups", "people-groups.person.reference.read", "person", profPerson}}
	if len(c.calls) != len(want) {
		t.Fatal(c.calls)
	}
	for i := range want {
		if c.calls[i] != want[i] {
			t.Fatal(c.calls)
		}
	}
	bad := r.professional
	bad.Action = "project-workspace.read"
	if r.WithProfessionalEndpoint(bad) == nil {
		t.Fatal("old Base read substituted for native personnel action")
	}
}

func TestProfessionalResolverRejectsMismatchedReferenceBeforeBonusCanWrite(t *testing.T) {
	for _, field := range []string{"schema_version", "source_application", "profile", "resource_type", "tenant_id", "project_id", "project_revision", "project_status", "revision", "state", "eligible", "role_key", "person_revision", "person_name", "people_revision", "people_name", "people_status"} {
		t.Run(field, func(t *testing.T) {
			r, _, a, p := professionalFixture(t)
			switch field {
			case "project_revision", "revision":
				a[field] = int64(99)
			case "eligible":
				a[field] = false
			case "person_revision":
				a["person"].(map[string]any)["revision"] = int64(99)
			case "person_name":
				a["person"].(map[string]any)["name"] = "另一人"
			case "people_revision":
				p["revision"] = int64(99)
			case "people_name":
				p["display_name"] = "改名未续分工"
			case "people_status":
				p["status"] = "inactive"
			default:
				a[field] = "wrong"
			}
			_, err := r.ResolveProfessionalParticipant(context.Background(), testTenant, core.ProjectSnapshot{ID: testProject, Revision: 9}, core.PersonRef{ID: profPerson, Revision: 2}, core.AssignmentRef{ID: profAssignment, Revision: 5})
			if !errors.Is(err, core.ErrReferenceInvalid) {
				t.Fatalf("invalid %s accepted: %v", field, err)
			}
		})
	}
}
