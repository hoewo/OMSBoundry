package engagement

import (
	"testing"
	"time"
)

func TestRequirementMatrixAndDerivedStatus(t *testing.T) {
	tender := MethodTender
	value := Engagement{Applicability: ApplicabilityApplicable, Method: &tender, Status: StatusDraft}
	if got := RequiredKeys(tender); len(got) != 3 || got[0] != RequirementTenderDocument {
		t.Fatalf("tender requirements = %#v", got)
	}
	if got := DeriveStatus(value, nil); got != StatusCollectingEvidence {
		t.Fatalf("empty tender status = %s", got)
	}
	materials := []MaterialReference{
		{RequirementKey: RequirementTenderDocument},
		{RequirementKey: RequirementBidDocument},
		{RequirementKey: RequirementAwardDocument},
	}
	if got := DeriveStatus(value, materials); got != StatusReady {
		t.Fatalf("complete tender status = %s", got)
	}
	materials[2].SupersededAt = ptrTime()
	if got := DeriveStatus(value, materials); got != StatusCollectingEvidence {
		t.Fatalf("superseded material status = %s", got)
	}

	comparison := MethodComparison
	value.Method = &comparison
	details := BuildDetails(value, materials[:2])
	if len(details.MissingRequirements) != 1 || details.MissingRequirements[0] != RequirementAwardDocument {
		t.Fatalf("comparison missing requirements = %#v", details.MissingRequirements)
	}
	if details.Requirements[0].Required {
		t.Fatal("tender document cannot be required for comparison")
	}
}

func TestTerminalStatesOverrideRequirements(t *testing.T) {
	method := MethodTender
	value := Engagement{Applicability: ApplicabilityNotApplicable, Method: &method, Status: StatusNotApplicable}
	if got := DeriveStatus(value, nil); got != StatusNotApplicable {
		t.Fatalf("not applicable status = %s", got)
	}
	value.Status = StatusCancelled
	if got := DeriveStatus(value, nil); got != StatusCancelled {
		t.Fatalf("cancelled status = %s", got)
	}
}

func ptrTime() *time.Time {
	value := time.Now().UTC()
	return &value
}
