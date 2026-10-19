package scenarioevidence

import (
	"errors"
	"fmt"
	"sort"
)

const ComparisonSchemaVersion = "customer-scenario-comparison-report/v1"

var customizationLayers = []string{
	"stable_core",
	"configuration",
	"policy",
	"typed_extension",
	"customer_assembly",
	"customer_specific",
}

var differenceDispositions = []string{
	"implemented",
	"accepted_deferred",
	"rejected",
	"needs_business_evidence",
}

type ComparisonReport struct {
	SchemaVersion          string                     `json:"schema_version"`
	ReviewStatus           string                     `json:"review_status"`
	Baseline               ComparisonScenario         `json:"baseline"`
	Candidate              ComparisonScenario         `json:"candidate"`
	Applications           ApplicationComparison      `json:"applications"`
	CustomizationLayers    []CountComparison          `json:"customization_layers"`
	Dispositions           []CountComparison          `json:"dispositions"`
	BoundarySignals        []BoundarySignalComparison `json:"boundary_signals"`
	CandidateFindings      []ReadinessGap             `json:"candidate_boundary_findings"`
	HumanReviewObligations []string                   `json:"human_review_obligations"`
	AutomatedConclusion    AutomatedConclusion        `json:"automated_conclusion"`
}

type ComparisonScenario struct {
	ScenarioID       string `json:"scenario_id"`
	Title            string `json:"title"`
	SourceKind       string `json:"source_kind"`
	ClaimScope       string `json:"claim_scope"`
	AssessmentStatus string `json:"assessment_status"`
}

type ApplicationComparison struct {
	Shared        []string `json:"shared"`
	BaselineOnly  []string `json:"baseline_only"`
	CandidateOnly []string `json:"candidate_only"`
}

type CountComparison struct {
	Name      string `json:"name"`
	Baseline  int    `json:"baseline"`
	Candidate int    `json:"candidate"`
}

type BoundarySignalComparison struct {
	Name      string `json:"name"`
	Baseline  int    `json:"baseline"`
	Candidate int    `json:"candidate"`
}

type AutomatedConclusion struct {
	CrossCustomerValidated bool   `json:"cross_customer_validated"`
	Result                 string `json:"result"`
	Reason                 string `json:"reason"`
}

func CompareScenarios(baseline, candidate Document) (ComparisonReport, error) {
	if err := Validate(baseline); err != nil {
		return ComparisonReport{}, fmt.Errorf("validate baseline: %w", err)
	}
	if err := Validate(candidate); err != nil {
		return ComparisonReport{}, fmt.Errorf("validate candidate: %w", err)
	}
	if err := validateComparisonInputs(baseline, candidate); err != nil {
		return ComparisonReport{}, err
	}

	reviewStatus := "human_review_required"
	if candidate.AssessmentStatus == "failed" {
		reviewStatus = "candidate_failed"
	}

	return ComparisonReport{
		SchemaVersion:       ComparisonSchemaVersion,
		ReviewStatus:        reviewStatus,
		Baseline:            comparisonScenario(baseline),
		Candidate:           comparisonScenario(candidate),
		Applications:        compareApplications(baseline.Applications.Enabled, candidate.Applications.Enabled),
		CustomizationLayers: compareCounts(customizationLayers, baseline.Differences, candidate.Differences, func(difference Difference) string { return difference.Layer }),
		Dispositions:        compareCounts(differenceDispositions, baseline.Differences, candidate.Differences, func(difference Difference) string { return difference.Disposition }),
		BoundarySignals:     compareBoundarySignals(baseline.BoundarySignals, candidate.BoundarySignals),
		CandidateFindings:   append([]ReadinessGap{}, boundaryFindings(candidate.BoundarySignals)...),
		HumanReviewObligations: []string{
			"Compare each business fact's meaning, unique owner, lifecycle states and correction semantics; matching counts or names are not semantic equivalence.",
			"Review every stable_core difference and accept it only when evidence shows it is customer-independent and useful to all supported deployments.",
			"Confirm configuration, policy, typed_extension, customer_assembly and customer_specific differences remain in the narrowest valid layer.",
			"Review implemented, deferred, rejected and needs-business-evidence differences individually, including requirements present in only one scenario.",
			"Compare delivery time, core-code change scope and operational burden using external evidence because Customer Scenario Evidence v1 does not encode those measurements.",
			"Record a human decision that the current product boundary is upheld, revised or falsified; this report cannot make that decision.",
		},
		AutomatedConclusion: AutomatedConclusion{
			CrossCustomerValidated: false,
			Result:                 "not_determined",
			Reason:                 "The report collates two evidence records deterministically but cannot decide business equivalence, generality or cross-customer product validity.",
		},
	}, nil
}

func validateComparisonInputs(baseline, candidate Document) error {
	var issues []string
	if baseline.SourceKind != "reference_product" || baseline.ClaimScope != "reference_baseline" || baseline.AssessmentStatus != "passed" {
		issues = append(issues, "baseline must be a passed reference_product/reference_baseline")
	}
	if AssessReadiness(baseline).Stage != ReadinessFinalReady {
		issues = append(issues, "baseline must be final_ready")
	}
	if candidate.SourceKind != "real_customer_anonymized" || candidate.ClaimScope != "single_customer_scenario" {
		issues = append(issues, "candidate must be real_customer_anonymized/single_customer_scenario")
	}
	if candidate.AssessmentStatus != "passed" && candidate.AssessmentStatus != "failed" {
		issues = append(issues, "candidate must have a final passed or failed assessment")
	}
	if AssessReadiness(candidate).Stage != ReadinessFinalReady {
		issues = append(issues, "candidate must be final_ready")
	}
	if baseline.ScenarioID == candidate.ScenarioID {
		issues = append(issues, "baseline and candidate scenario_id must differ")
	}
	if len(issues) > 0 {
		return errors.New(joinIssues(issues))
	}
	return nil
}

func joinIssues(issues []string) string {
	result := ""
	for index, issue := range issues {
		if index > 0 {
			result += "; "
		}
		result += issue
	}
	return result
}

func comparisonScenario(document Document) ComparisonScenario {
	return ComparisonScenario{
		ScenarioID:       document.ScenarioID,
		Title:            document.Title,
		SourceKind:       document.SourceKind,
		ClaimScope:       document.ClaimScope,
		AssessmentStatus: document.AssessmentStatus,
	}
}

func compareApplications(baseline, candidate []string) ApplicationComparison {
	baselineSet := stringSet(baseline)
	candidateSet := stringSet(candidate)
	comparison := ApplicationComparison{
		Shared:        []string{},
		BaselineOnly:  []string{},
		CandidateOnly: []string{},
	}
	for application := range baselineSet {
		if _, exists := candidateSet[application]; exists {
			comparison.Shared = append(comparison.Shared, application)
		} else {
			comparison.BaselineOnly = append(comparison.BaselineOnly, application)
		}
	}
	for application := range candidateSet {
		if _, exists := baselineSet[application]; !exists {
			comparison.CandidateOnly = append(comparison.CandidateOnly, application)
		}
	}
	sort.Strings(comparison.Shared)
	sort.Strings(comparison.BaselineOnly)
	sort.Strings(comparison.CandidateOnly)
	return comparison
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func compareCounts(names []string, baseline, candidate []Difference, key func(Difference) string) []CountComparison {
	baselineCounts := countDifferences(baseline, key)
	candidateCounts := countDifferences(candidate, key)
	result := make([]CountComparison, 0, len(names))
	for _, name := range names {
		result = append(result, CountComparison{Name: name, Baseline: baselineCounts[name], Candidate: candidateCounts[name]})
	}
	return result
}

func countDifferences(differences []Difference, key func(Difference) string) map[string]int {
	counts := make(map[string]int)
	for _, difference := range differences {
		counts[key(difference)]++
	}
	return counts
}

func compareBoundarySignals(baseline, candidate BoundarySignals) []BoundarySignalComparison {
	return []BoundarySignalComparison{
		{Name: "customer_core_branches", Baseline: *baseline.CustomerCoreBranches, Candidate: *candidate.CustomerCoreBranches},
		{Name: "customer_databases", Baseline: *baseline.CustomerDatabases, Candidate: *candidate.CustomerDatabases},
		{Name: "shared_business_tables", Baseline: *baseline.SharedBusinessTables, Candidate: *candidate.SharedBusinessTables},
		{Name: "customer_business_migrations", Baseline: *baseline.CustomerBusinessMigrations, Candidate: *candidate.CustomerBusinessMigrations},
		{Name: "cross_app_internal_reads", Baseline: *baseline.CrossAppInternalReads, Candidate: *candidate.CrossAppInternalReads},
		{Name: "onsite_core_patches", Baseline: *baseline.OnsiteCorePatches, Candidate: *candidate.OnsiteCorePatches},
		{Name: "hidden_mandatory_dependencies", Baseline: *baseline.HiddenMandatoryDependencies, Candidate: *candidate.HiddenMandatoryDependencies},
	}
}
