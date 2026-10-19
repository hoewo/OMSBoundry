package scenarioevidence

import (
	"fmt"
	"strings"
)

type ReadinessStage string

const (
	ReadinessDraft         ReadinessStage = "draft"
	ReadinessEvidenceReady ReadinessStage = "evidence_ready"
	ReadinessFinalReady    ReadinessStage = "final_ready"
)

type ReadinessGap struct {
	Path    string
	Message string
}

type ReadinessReport struct {
	Stage             ReadinessStage
	EvidenceReadyGaps []ReadinessGap
	FinalGaps         []ReadinessGap
	BoundaryFindings  []ReadinessGap
}

func AssessReadiness(document Document) ReadinessReport {
	evidenceGaps := evidenceReadyGaps(document)
	finalGaps := append([]ReadinessGap{}, evidenceGaps...)

	if len(document.AcceptanceEvidence) == 0 {
		finalGaps = appendGap(finalGaps, "acceptance_evidence", "add executed acceptance evidence before a final customer result")
	}
	if document.SourceKind == "real_customer_anonymized" && len(document.SourceEvidence.CustomerConfirmationRefs) == 0 {
		finalGaps = appendGap(finalGaps, "source_evidence.customer_confirmation_refs", "add an approved customer confirmation reference before a final customer result")
	}
	finalGaps = append(finalGaps, boundaryCompletenessGaps(document.BoundarySignals)...)
	boundaryFindings := boundaryFindings(document.BoundarySignals)

	stage := ReadinessDraft
	if len(evidenceGaps) == 0 {
		stage = ReadinessEvidenceReady
	}
	if len(finalGaps) == 0 {
		stage = ReadinessFinalReady
	}
	return ReadinessReport{
		Stage:             stage,
		EvidenceReadyGaps: evidenceGaps,
		FinalGaps:         finalGaps,
		BoundaryFindings:  boundaryFindings,
	}
}

func (report ReadinessReport) Meets(required ReadinessStage) bool {
	switch required {
	case ReadinessDraft:
		return true
	case ReadinessEvidenceReady:
		return report.Stage == ReadinessEvidenceReady || report.Stage == ReadinessFinalReady
	case ReadinessFinalReady:
		return report.Stage == ReadinessFinalReady
	default:
		return false
	}
}

func evidenceReadyGaps(document Document) []ReadinessGap {
	var gaps []ReadinessGap
	checkPlaceholder(&gaps, "scenario_id", document.ScenarioID)
	checkPlaceholder(&gaps, "title", document.Title)
	checkPlaceholder(&gaps, "problem.statement", document.Problem.Statement)
	if len(document.Problem.ObservableOutcomes) == 0 {
		gaps = appendGap(gaps, "problem.observable_outcomes", "record at least one observable business outcome")
	}
	if len(document.Problem.OutOfScope) == 0 {
		gaps = appendGap(gaps, "problem.out_of_scope", "state at least one adjacent problem this scenario does not solve")
	}
	if len(document.SourceEvidence.EvidenceRefs) == 0 {
		gaps = appendGap(gaps, "source_evidence.evidence_refs", "add at least one approved anonymized source-evidence reference")
	}
	if len(document.Facts) == 0 {
		gaps = appendGap(gaps, "facts", "identify at least one business fact and its unique owner")
	}
	for index, fact := range document.Facts {
		prefix := fmt.Sprintf("facts[%d]", index)
		checkPlaceholder(&gaps, prefix+".id", fact.ID)
		checkPlaceholder(&gaps, prefix+".owner", fact.Owner)
		checkPlaceholder(&gaps, prefix+".meaning", fact.Meaning)
		checkPlaceholder(&gaps, prefix+".correction", fact.Correction)
		for stateIndex, state := range fact.States {
			checkPlaceholder(&gaps, fmt.Sprintf("%s.states[%d]", prefix, stateIndex), state)
		}
		if len(fact.EvidenceRefs) == 0 {
			gaps = appendGap(gaps, prefix+".evidence_refs", "link the fact meaning, lifecycle and correction rule to approved source evidence")
		}
	}
	if len(document.Applications.Enabled) == 0 {
		gaps = appendGap(gaps, "applications.enabled", "select the smallest application set needed by the observable outcomes")
	}
	if len(document.Differences) == 0 {
		gaps = appendGap(gaps, "differences", "classify every customer difference into exactly one customization layer")
	}
	for index, difference := range document.Differences {
		prefix := fmt.Sprintf("differences[%d]", index)
		checkPlaceholder(&gaps, prefix+".id", difference.ID)
		checkPlaceholder(&gaps, prefix+".requirement", difference.Requirement)
		checkPlaceholder(&gaps, prefix+".rationale", difference.Rationale)
		if difference.Disposition == "implemented" {
			if len(difference.DeliveryArtifacts) == 0 {
				gaps = appendGap(gaps, prefix+".delivery_artifacts", "name the configuration, policy, extension, connector or assembly artifact that implements this difference")
			}
			if len(difference.EvidenceRefs) == 0 {
				gaps = appendGap(gaps, prefix+".evidence_refs", "link the implemented difference to repeatable evidence")
			}
			if difference.Layer == "policy" {
				gaps = append(gaps, policyReadinessGaps(prefix+".policy_evidence", difference.PolicyEvidence)...)
			}
		}
	}
	return gaps
}

func policyReadinessGaps(path string, evidence *PolicyEvidence) []ReadinessGap {
	if evidence == nil {
		return []ReadinessGap{{Path: path, Message: "record policy version, fact inputs, output, correction and runtime evidence"}}
	}
	var gaps []ReadinessGap
	if strings.TrimSpace(evidence.Version) == "" {
		gaps = appendGap(gaps, path+".version", "record a stable policy version")
	}
	if len(evidence.Inputs) == 0 {
		gaps = appendGap(gaps, path+".inputs", "identify the versioned business-fact inputs")
	}
	if strings.TrimSpace(evidence.Output) == "" {
		gaps = appendGap(gaps, path+".output", "describe the policy output without assigning target-application ownership")
	}
	if strings.TrimSpace(evidence.Correction) == "" {
		gaps = appendGap(gaps, path+".correction", "define how a wrong or superseded result is corrected")
	}
	if len(evidence.RuntimeEvidenceRefs) == 0 {
		gaps = appendGap(gaps, path+".runtime_evidence_refs", "link an executed policy example and replay evidence")
	}
	return gaps
}

func boundaryCompletenessGaps(signals BoundarySignals) []ReadinessGap {
	values := []struct {
		name  string
		value *int
	}{
		{"customer_core_branches", signals.CustomerCoreBranches},
		{"customer_databases", signals.CustomerDatabases},
		{"shared_business_tables", signals.SharedBusinessTables},
		{"customer_business_migrations", signals.CustomerBusinessMigrations},
		{"cross_app_internal_reads", signals.CrossAppInternalReads},
		{"onsite_core_patches", signals.OnsiteCorePatches},
		{"hidden_mandatory_dependencies", signals.HiddenMandatoryDependencies},
	}
	var gaps []ReadinessGap
	for _, item := range values {
		path := "boundary_signals." + item.name
		if item.value == nil {
			gaps = appendGap(gaps, path, "measure this boundary signal before a final result")
		}
	}
	return gaps
}

func boundaryFindings(signals BoundarySignals) []ReadinessGap {
	values := []struct {
		name  string
		value *int
	}{
		{"customer_core_branches", signals.CustomerCoreBranches},
		{"customer_databases", signals.CustomerDatabases},
		{"shared_business_tables", signals.SharedBusinessTables},
		{"customer_business_migrations", signals.CustomerBusinessMigrations},
		{"cross_app_internal_reads", signals.CrossAppInternalReads},
		{"onsite_core_patches", signals.OnsiteCorePatches},
		{"hidden_mandatory_dependencies", signals.HiddenMandatoryDependencies},
	}
	var findings []ReadinessGap
	for _, item := range values {
		if item.value != nil && *item.value != 0 {
			findings = appendGap(
				findings,
				"boundary_signals."+item.name,
				fmt.Sprintf("measured %d; the final assessment cannot be passed unless a new evidence run measures zero", *item.value),
			)
		}
	}
	return findings
}

func checkPlaceholder(gaps *[]ReadinessGap, path, value string) {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(trimmed, "替换为") || strings.HasPrefix(lower, "replace-") || strings.HasPrefix(lower, "replace ") {
		*gaps = appendGap(*gaps, path, "replace the template placeholder with anonymized scenario evidence")
	}
}

func appendGap(gaps []ReadinessGap, path, message string) []ReadinessGap {
	for _, gap := range gaps {
		if gap.Path == path && gap.Message == message {
			return gaps
		}
	}
	return append(gaps, ReadinessGap{Path: path, Message: message})
}
