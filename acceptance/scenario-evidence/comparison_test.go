package scenarioevidence

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCompareScenariosProducesConservativeDeterministicReport(t *testing.T) {
	baseline := readDocument(t, contractRoot+"/examples/reference-oa.json")
	candidate := finalCustomerScenario(t, baseline, "passed")
	candidate.Applications.Enabled = append(candidate.Applications.Enabled, "customer-export")

	report, err := CompareScenarios(baseline, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != ComparisonSchemaVersion || report.ReviewStatus != "human_review_required" {
		t.Fatalf("unexpected report identity: %#v", report)
	}
	if report.AutomatedConclusion.CrossCustomerValidated || report.AutomatedConclusion.Result != "not_determined" {
		t.Fatalf("comparison made an unsupported product conclusion: %#v", report.AutomatedConclusion)
	}
	if len(report.HumanReviewObligations) != 6 {
		t.Fatalf("expected six explicit human obligations, got %d", len(report.HumanReviewObligations))
	}
	if !containsString(report.Applications.CandidateOnly, "customer-export") {
		t.Fatalf("candidate-only application was not reported: %#v", report.Applications)
	}
	if len(report.CustomizationLayers) != 6 || len(report.Dispositions) != 4 || len(report.BoundarySignals) != 7 {
		t.Fatalf("report omitted fixed comparison dimensions: %#v", report)
	}

	first, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	secondReport, err := CompareScenarios(baseline, candidate)
	if err != nil {
		t.Fatal(err)
	}
	second, err := json.Marshal(secondReport)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("comparison report is not deterministic")
	}
}

func TestCompareScenariosPreservesFailedBoundaryEvidence(t *testing.T) {
	baseline := readDocument(t, contractRoot+"/examples/reference-oa.json")
	candidate := finalCustomerScenario(t, baseline, "failed")
	one := 1
	candidate.BoundarySignals.OnsiteCorePatches = &one

	report, err := CompareScenarios(baseline, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if report.ReviewStatus != "candidate_failed" {
		t.Fatalf("expected candidate_failed, got %s", report.ReviewStatus)
	}
	if !hasGap(report.CandidateFindings, "boundary_signals.onsite_core_patches") {
		t.Fatalf("failed boundary evidence was hidden: %#v", report.CandidateFindings)
	}
	if value := boundaryCandidateValue(report.BoundarySignals, "onsite_core_patches"); value != 1 {
		t.Fatalf("expected candidate boundary value 1, got %d", value)
	}
	if report.AutomatedConclusion.CrossCustomerValidated {
		t.Fatal("failed candidate was reported as cross-customer validated")
	}
}

func TestCompareScenariosFailsClosed(t *testing.T) {
	baseline := readDocument(t, contractRoot+"/examples/reference-oa.json")
	tests := []struct {
		name    string
		mutate  func(*Document, *Document)
		message string
	}{
		{
			name: "wrong baseline identity",
			mutate: func(baseline, _ *Document) {
				baseline.SourceKind = "real_customer_anonymized"
				baseline.ClaimScope = "single_customer_scenario"
				baseline.SourceEvidence.CustomerConfirmationRefs = []string{"controlled-confirmation:baseline"}
			},
			message: "baseline must be a passed reference_product/reference_baseline",
		},
		{
			name: "candidate is not final",
			mutate: func(_, candidate *Document) {
				candidate.AssessmentStatus = "evidence_ready"
				candidate.SourceEvidence.CustomerConfirmationRefs = nil
			},
			message: "candidate must have a final passed or failed assessment",
		},
		{
			name: "duplicate scenario identity",
			mutate: func(baseline, candidate *Document) {
				candidate.ScenarioID = baseline.ScenarioID
			},
			message: "scenario_id must differ",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			baselineCopy := cloneDocument(t, baseline)
			candidate := finalCustomerScenario(t, baseline, "passed")
			test.mutate(&baselineCopy, &candidate)
			_, err := CompareScenarios(baselineCopy, candidate)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("expected %q, got %v", test.message, err)
			}
		})
	}
}

func finalCustomerScenario(t *testing.T, baseline Document, status string) Document {
	t.Helper()
	candidate := cloneDocument(t, baseline)
	candidate.ScenarioID = "customer-scenario-two"
	candidate.Title = "脱敏第二客户场景"
	candidate.SourceKind = "real_customer_anonymized"
	candidate.ClaimScope = "single_customer_scenario"
	candidate.AssessmentStatus = status
	candidate.SourceEvidence.CustomerConfirmationRefs = []string{"controlled-confirmation:customer-scenario-two"}
	return candidate
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func hasGap(gaps []ReadinessGap, expected string) bool {
	for _, gap := range gaps {
		if gap.Path == expected {
			return true
		}
	}
	return false
}

func boundaryCandidateValue(values []BoundarySignalComparison, expected string) int {
	for _, value := range values {
		if value.Name == expected {
			return value.Candidate
		}
	}
	return -1
}
