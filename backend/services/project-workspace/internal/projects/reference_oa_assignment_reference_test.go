package projects

import (
	personnel "omsbizfoundry/project-workspace/internal/referenceoa/productionpersonnel"
	"testing"
	"time"
)

func TestOAProductionReferenceDoesNotReclassifyHiddenOrEndedFacts(t *testing.T) {
	p := Project{ID: "project", TenantID: "tenant", Revision: 10, Status: StatusActive}
	f := OAProductionFact{Assignment: personnel.Assignment{ID: "assignment", Role: "participant", CreatedProjectRevision: 4, Person: personnel.PersonSnapshot{Ref: personnel.Ref{ID: "person", Revision: 2}, Name: "甲"}}}
	v := assignmentReference(p, true, f)
	if v.ResourceType != OAProductionAssignmentResource || v.Profile != personnel.Profile || v.Revision != 4 || !v.Eligible || !v.ParticipantNotApplicable || v.State != "active" {
		t.Fatalf("hidden active=%+v", v)
	}
	now := time.Now()
	rev := int64(8)
	f.EndedAt = &now
	f.EndedProjectRevision = &rev
	v = assignmentReference(p, false, f)
	if v.State != "ended" || v.Eligible || v.Revision != 8 || v.Person != f.Person {
		t.Fatalf("ended=%+v", v)
	}
	f.EndedAt = nil
	p.Status = StatusCancelled
	if assignmentReference(p, false, f).Eligible {
		t.Fatal("cancelled project eligible")
	}
}
