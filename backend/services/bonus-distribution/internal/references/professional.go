package references

import (
	"context"
	"net/url"
	"strings"

	core "omsbizfoundry/bonus-distribution/internal/bonus"
	rb "omsbizfoundry/bonus-distribution/internal/referencebonus"
)

const professionalResource = "reference_oa_production_assignment"

// Called only by the explicitly selected v3 composition. Its native action is
// separate from Base Project read; no legacy workload credential substitution.
func (r *HTTPResolver) WithProfessionalEndpoint(endpoint Endpoint) error {
	if err := validateEndpoint(endpoint, false); err != nil {
		return err
	}
	if endpoint.Credentials != nil && (endpoint.Audience != "project-workspace" || endpoint.Action != "project-workspace.reference-oa-personnel.read") {
		return core.ErrReferenceInvalid
	}
	r.professional = endpoint
	return nil
}

type professionalReference struct {
	SchemaVersion     string `json:"schema_version"`
	SourceApplication string `json:"source_application"`
	Profile           string `json:"profile"`
	ResourceType      string `json:"resource_type"`
	TenantID          string `json:"tenant_id"`
	ProjectID         string `json:"project_id"`
	ProjectRevision   int64  `json:"project_revision"`
	ProjectStatus     string `json:"project_status"`
	ID                string `json:"id"`
	Revision          int64  `json:"revision"`
	RoleKey           string `json:"role_key"`
	Person            struct {
		ID       string `json:"id"`
		Revision int64  `json:"revision"`
		Name     string `json:"name"`
	} `json:"person"`
	Discipline *struct {
		ID       string `json:"id"`
		Revision int64  `json:"revision"`
		Name     string `json:"name"`
	} `json:"discipline,omitempty"`
	State    string `json:"state"`
	Eligible bool   `json:"eligible"`
}

func (v professionalReference) valid(tenant string, p core.ProjectSnapshot, person core.PersonRef, a core.AssignmentRef) bool {
	return v.SchemaVersion == "1" && v.SourceApplication == "project-workspace" && v.Profile == rb.ProfessionalSource && v.ResourceType == professionalResource && v.TenantID == tenant && v.ProjectID == p.ID && v.ProjectRevision == p.Revision && v.ProjectStatus == "active" && v.ID == a.ID && v.Revision == a.Revision && v.State == "active" && v.Eligible && rb.ProfessionalRole(v.RoleKey) && v.Person.ID == person.ID && v.Person.Revision == person.Revision && strings.TrimSpace(v.Person.Name) != ""
}
func professionalPath(id string) string {
	return "/api/v1/references/projects/" + url.PathEscape(id) + "/reference-oa/production-assignments"
}
func (r *HTTPResolver) ResolveProfessionalParticipant(ctx context.Context, tenant string, p core.ProjectSnapshot, person core.PersonRef, a core.AssignmentRef) (rb.Participant, error) {
	if r.professional.BaseURL == "" {
		return rb.Participant{}, core.ErrReferenceUnavailable
	}
	body, err := r.get(ctx, r.professional, professionalPath(p.ID)+"/"+url.PathEscape(a.ID), "project", p.ID)
	if err != nil {
		return rb.Participant{}, err
	}
	var v professionalReference
	if decode(body, &v) != nil {
		return rb.Participant{}, core.ErrReferenceUnavailable
	}
	if !v.valid(tenant, p, person, a) {
		return rb.Participant{}, core.ErrReferenceInvalid
	}
	personBody, err := r.get(ctx, r.people, "/api/v1/references/persons/"+url.PathEscape(person.ID), "person", person.ID)
	if err != nil {
		return rb.Participant{}, err
	}
	var current struct {
		SchemaVersion     string `json:"schema_version"`
		SourceApplication string `json:"source_application"`
		ResourceType      string `json:"resource_type"`
		TenantID          string `json:"tenant_id"`
		ID                string `json:"id"`
		Revision          int64  `json:"revision"`
		DisplayName       string `json:"display_name"`
		Status            string `json:"status"`
	}
	if decode(personBody, &current) != nil {
		return rb.Participant{}, core.ErrReferenceUnavailable
	}
	if current.SchemaVersion != "1" || current.SourceApplication != "people-groups" || current.ResourceType != "person" || current.TenantID != tenant || current.ID != person.ID || current.Revision != person.Revision || current.Status != "active" || current.DisplayName != v.Person.Name {
		return rb.Participant{}, core.ErrReferenceInvalid
	}
	out := rb.Participant{AssignmentSource: rb.ProfessionalSource, ParticipantReference: core.ParticipantReference{TenantID: tenant, BonusType: core.TypeProduction, Active: true, Eligible: true, Person: core.PersonSnapshot{ID: current.ID, Revision: current.Revision, DisplayName: current.DisplayName, Digest: digest(personBody)}, Assignment: core.AssignmentSnapshot{ID: v.ID, Revision: v.Revision, ProjectID: p.ID, PersonID: current.ID, RoleKey: v.RoleKey, Digest: digest(body)}}}
	if v.Discipline != nil {
		out.Discipline = &rb.DisciplineSnapshot{ID: v.Discipline.ID, Revision: v.Discipline.Revision, DisplayName: v.Discipline.Name}
	}
	return out, nil
}
func (r *HTTPResolver) ListProfessionalCandidates(ctx context.Context, tenant string, ref core.ProjectRef) ([]rb.Participant, error) {
	p, err := r.ResolveProject(ctx, tenant, ref)
	if err != nil {
		return nil, err
	}
	if r.professional.BaseURL == "" {
		return nil, core.ErrReferenceUnavailable
	}
	body, err := r.get(ctx, r.professional, professionalPath(ref.ID), "project", ref.ID)
	if err != nil {
		return nil, err
	}
	var list struct {
		SchemaVersion     string                  `json:"schema_version"`
		SourceApplication string                  `json:"source_application"`
		Profile           string                  `json:"profile"`
		ResourceType      string                  `json:"resource_type"`
		TenantID          string                  `json:"tenant_id"`
		ProjectID         string                  `json:"project_id"`
		ProjectRevision   int64                   `json:"project_revision"`
		ProjectStatus     string                  `json:"project_status"`
		Items             []professionalReference `json:"items"`
	}
	if decode(body, &list) != nil {
		return nil, core.ErrReferenceUnavailable
	}
	if list.SchemaVersion != "1" || list.SourceApplication != "project-workspace" || list.Profile != rb.ProfessionalSource || list.ResourceType != professionalResource || list.TenantID != tenant || list.ProjectID != ref.ID || list.ProjectRevision != ref.Revision || list.ProjectStatus != "active" || list.Items == nil || len(list.Items) > 100 {
		return nil, core.ErrReferenceInvalid
	}
	out := []rb.Participant{}
	seen := map[string]bool{}
	for _, v := range list.Items {
		if seen[v.ID] || !v.valid(tenant, p.ProjectSnapshot, core.PersonRef{ID: v.Person.ID, Revision: v.Person.Revision}, core.AssignmentRef{ID: v.ID, Revision: v.Revision}) {
			return nil, core.ErrReferenceInvalid
		}
		seen[v.ID] = true
		participant, e := r.ResolveProfessionalParticipant(ctx, tenant, p.ProjectSnapshot, core.PersonRef{ID: v.Person.ID, Revision: v.Person.Revision}, core.AssignmentRef{ID: v.ID, Revision: v.Revision})
		if e != nil {
			return nil, e
		}
		out = append(out, participant)
	}
	return out, nil
}
