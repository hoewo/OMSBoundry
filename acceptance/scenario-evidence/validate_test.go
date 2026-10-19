package scenarioevidence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	contractRoot = "../../contracts/scenario-evidence/v1"
	templatePath = "../../docs/templates/customer-scenario-evidence-v1.json"
)

func TestPublishedSchemaHasStableIdentity(t *testing.T) {
	data := readFile(t, filepath.Join(contractRoot, "scenario-evidence.schema.json"))
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	if schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
		t.Fatalf("unexpected schema declaration: %v", schema["$schema"])
	}
	if schema["$id"] != "https://omsbizfoundry.local/contracts/scenario-evidence/v1/scenario-evidence.schema.json" {
		t.Fatalf("unexpected schema id: %v", schema["$id"])
	}
}

func TestPublishedReferenceAndTemplateValidate(t *testing.T) {
	paths := []string{
		filepath.Join(contractRoot, "examples/reference-oa.json"),
		filepath.Join(contractRoot, "examples/reference-oms.json"),
		templatePath,
	}
	for _, name := range paths {
		t.Run(filepath.Base(name), func(t *testing.T) {
			if err := ValidateBytes(readFile(t, name)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestUnknownFieldsAndVersionsFailClosed(t *testing.T) {
	data := readFile(t, filepath.Join(contractRoot, "examples/reference-oa.json"))
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	raw["unsupported_claim"] = true
	mutated, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateBytes(mutated); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown field was not rejected: %v", err)
	}

	document := readDocument(t, filepath.Join(contractRoot, "examples/reference-oa.json"))
	document.SchemaVersion = "customer-scenario-evidence/v2"
	assertInvalid(t, document, "schema_version")
}

func TestMissingRequiredStructureFailsClosed(t *testing.T) {
	data := readFile(t, filepath.Join(contractRoot, "examples/reference-oa.json"))
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	delete(raw, "limitations")
	mutated, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateBytes(mutated); err == nil || !strings.Contains(err.Error(), "limitations is required") {
		t.Fatalf("missing required field was not rejected: %v", err)
	}
}

func TestUnsupportedAcceptanceClaimsFailClosed(t *testing.T) {
	valid := readDocument(t, filepath.Join(contractRoot, "examples/reference-oa.json"))
	tests := []struct {
		name    string
		mutate  func(*Document)
		message string
	}{
		{
			name: "reference product claims customer acceptance",
			mutate: func(value *Document) {
				value.ClaimScope = "single_customer_scenario"
			},
			message: "reference_product may only claim",
		},
		{
			name: "real customer lacks confirmation",
			mutate: func(value *Document) {
				value.SourceKind = "real_customer_anonymized"
				value.ClaimScope = "single_customer_scenario"
				value.SourceEvidence.CustomerConfirmationRefs = nil
			},
			message: "customer_confirmation_refs",
		},
		{
			name: "hypothetical probe reports passed",
			mutate: func(value *Document) {
				value.SourceKind = "hypothetical_probe"
				value.ClaimScope = "none"
			},
			message: "hypothetical_probe cannot have a final",
		},
		{
			name: "final result lacks acceptance evidence",
			mutate: func(value *Document) {
				value.AcceptanceEvidence = nil
			},
			message: "acceptance_evidence",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := cloneDocument(t, valid)
			test.mutate(&value)
			assertInvalid(t, value, test.message)
		})
	}
}

func TestBoundaryAndOwnershipViolationsFailClosed(t *testing.T) {
	valid := readDocument(t, filepath.Join(contractRoot, "examples/reference-oa.json"))
	tests := []struct {
		name    string
		mutate  func(*Document)
		message string
	}{
		{
			name: "customer core branch",
			mutate: func(value *Document) {
				one := 1
				value.BoundarySignals.CustomerCoreBranches = &one
			},
			message: "customer_core_branches to be zero",
		},
		{
			name: "unknown fact reference",
			mutate: func(value *Document) {
				value.Differences[0].FactRefs = []string{"missing-fact"}
			},
			message: "unknown fact",
		},
		{
			name: "fact without correction",
			mutate: func(value *Document) {
				value.Facts[0].Correction = ""
			},
			message: "correction must not be empty",
		},
		{
			name: "implemented difference without evidence",
			mutate: func(value *Document) {
				value.Differences[0].EvidenceRefs = nil
			},
			message: "requires evidence_refs",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := cloneDocument(t, valid)
			test.mutate(&value)
			assertInvalid(t, value, test.message)
		})
	}
}

func TestEmptyPolicyAndConnectorClaimsFailClosed(t *testing.T) {
	valid := readDocument(t, filepath.Join(contractRoot, "examples/reference-oa.json"))

	t.Run("implemented policy requires runtime evidence", func(t *testing.T) {
		value := cloneDocument(t, valid)
		value.Differences[0].Layer = "policy"
		value.Differences[0].PolicyEvidence = &PolicyEvidence{
			Version:    "policy-v1",
			Inputs:     []string{"people-fact"},
			Output:     "derived value",
			Correction: "superseding policy decision",
		}
		assertInvalid(t, value, "runtime_evidence_refs")
	})

	t.Run("connector requires complete semantics", func(t *testing.T) {
		value := cloneDocument(t, valid)
		value.Connectors = []Connector{{
			ID:                 "contract-money-connector",
			DifferenceRef:      "manual-obligation-confirmation",
			SourceApplication:  "contract-ledger",
			TargetApplication:  "money-flow-ledger",
			SourceContract:     "",
			TargetContract:     "money obligation command v1",
			Idempotency:        "source revision",
			FailureRecovery:    "operator retry",
			TargetConfirmation: "money owner confirms",
		}}
		assertInvalid(t, value, "source_contract must not be empty")
	})

	t.Run("connector cannot be hidden in stable core", func(t *testing.T) {
		value := cloneDocument(t, valid)
		value.Connectors = []Connector{{
			ID:                 "design-project-connector",
			DifferenceRef:      "design-reference-compatible-id",
			SourceApplication:  "design-operations",
			TargetApplication:  "project-workspace",
			SourceContract:     "design reference read v1",
			TargetContract:     "project resource link v1",
			Idempotency:        "work item and project resource key",
			FailureRecovery:    "retry without duplicating link",
			TargetConfirmation: "project validates reference before link",
		}}
		assertInvalid(t, value, "must belong to customer_assembly or customer_specific")
	})
}

func TestReadinessSeparatesDraftEvidenceAndFinalObligations(t *testing.T) {
	t.Run("published reference is final ready", func(t *testing.T) {
		document := readDocument(t, filepath.Join(contractRoot, "examples/reference-oa.json"))
		report := AssessReadiness(document)
		if report.Stage != ReadinessFinalReady {
			t.Fatalf("expected final_ready, got %s: evidence=%v final=%v", report.Stage, report.EvidenceReadyGaps, report.FinalGaps)
		}
	})

	t.Run("OMS reference is evidence ready without final claims", func(t *testing.T) {
		document := readDocument(t, filepath.Join(contractRoot, "examples/reference-oms.json"))
		report := AssessReadiness(document)
		if report.Stage != ReadinessEvidenceReady {
			t.Fatalf("expected evidence_ready, got %s: evidence=%v final=%v", report.Stage, report.EvidenceReadyGaps, report.FinalGaps)
		}
		assertReadinessGap(t, report.FinalGaps, "boundary_signals.customer_core_branches")
	})

	t.Run("blank customer template reports actionable draft gaps", func(t *testing.T) {
		document := readDocument(t, templatePath)
		report := AssessReadiness(document)
		if report.Stage != ReadinessDraft {
			t.Fatalf("expected draft, got %s", report.Stage)
		}
		for _, path := range []string{"scenario_id", "problem.observable_outcomes", "source_evidence.evidence_refs", "applications.enabled", "facts[0].evidence_refs"} {
			assertReadinessGap(t, report.EvidenceReadyGaps, path)
		}
	})

	t.Run("customer confirmation is a final-only obligation", func(t *testing.T) {
		document := cloneDocument(t, readDocument(t, filepath.Join(contractRoot, "examples/reference-oa.json")))
		document.SourceKind = "real_customer_anonymized"
		document.ClaimScope = "single_customer_scenario"
		document.AssessmentStatus = "evidence_ready"
		document.SourceEvidence.CustomerConfirmationRefs = nil
		report := AssessReadiness(document)
		if report.Stage != ReadinessEvidenceReady {
			t.Fatalf("expected evidence_ready, got %s: evidence=%v final=%v", report.Stage, report.EvidenceReadyGaps, report.FinalGaps)
		}
		assertReadinessGap(t, report.FinalGaps, "source_evidence.customer_confirmation_refs")
	})

	t.Run("implemented draft difference cannot hide missing delivery evidence", func(t *testing.T) {
		document := cloneDocument(t, readDocument(t, filepath.Join(contractRoot, "examples/reference-oa.json")))
		document.AssessmentStatus = "draft"
		document.Differences[0].DeliveryArtifacts = nil
		document.Differences[0].EvidenceRefs = nil
		report := AssessReadiness(document)
		assertReadinessGap(t, report.EvidenceReadyGaps, "differences[0].delivery_artifacts")
		assertReadinessGap(t, report.EvidenceReadyGaps, "differences[0].evidence_refs")
	})

	t.Run("measured boundary violation is final-ready failure evidence", func(t *testing.T) {
		document := cloneDocument(t, readDocument(t, filepath.Join(contractRoot, "examples/reference-oa.json")))
		document.SourceKind = "real_customer_anonymized"
		document.ClaimScope = "single_customer_scenario"
		document.AssessmentStatus = "failed"
		document.SourceEvidence.CustomerConfirmationRefs = []string{"controlled-confirmation:customer-review-1"}
		one := 1
		document.BoundarySignals.CustomerCoreBranches = &one
		if err := Validate(document); err != nil {
			t.Fatalf("failed evidence with a measured boundary violation should validate: %v", err)
		}
		report := AssessReadiness(document)
		if report.Stage != ReadinessFinalReady {
			t.Fatalf("expected final_ready failed evidence, got %s: final=%v", report.Stage, report.FinalGaps)
		}
		assertReadinessGap(t, report.BoundaryFindings, "boundary_signals.customer_core_branches")
	})
}

func readDocument(t *testing.T, path string) Document {
	t.Helper()
	document, err := Parse(readFile(t, path))
	if err != nil {
		t.Fatal(err)
	}
	return document
}

func cloneDocument(t *testing.T, source Document) Document {
	t.Helper()
	data, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	document, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return document
}

func assertInvalid(t *testing.T, document Document, message string) {
	t.Helper()
	err := Validate(document)
	if err == nil {
		t.Fatal("invalid scenario evidence was accepted")
	}
	if !strings.Contains(err.Error(), message) {
		t.Fatalf("expected %q in error, got %v", message, err)
	}
}

func assertReadinessGap(t *testing.T, gaps []ReadinessGap, path string) {
	t.Helper()
	for _, gap := range gaps {
		if gap.Path == path {
			return
		}
	}
	t.Fatalf("missing readiness gap %s in %#v", path, gaps)
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
