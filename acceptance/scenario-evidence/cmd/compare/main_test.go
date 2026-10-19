package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	scenarioevidence "omsbizfoundry/scenario-evidence-acceptance"
)

func TestCompareCommandEmitsReport(t *testing.T) {
	repositoryRoot := filepath.Join("..", "..", "..", "..")
	baselinePath := filepath.Join(repositoryRoot, "contracts", "scenario-evidence", "v1", "examples", "reference-oa.json")
	baselineData, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := scenarioevidence.Parse(baselineData)
	if err != nil {
		t.Fatal(err)
	}
	candidate.ScenarioID = "command-customer-scenario"
	candidate.Title = "命令集成测试客户场景"
	candidate.SourceKind = "real_customer_anonymized"
	candidate.ClaimScope = "single_customer_scenario"
	candidate.SourceEvidence.CustomerConfirmationRefs = []string{"controlled-confirmation:command-test"}
	candidateData, err := json.Marshal(candidate)
	if err != nil {
		t.Fatal(err)
	}
	candidatePath := filepath.Join(t.TempDir(), "candidate.json")
	if err := os.WriteFile(candidatePath, candidateData, 0o600); err != nil {
		t.Fatal(err)
	}

	command := exec.Command("go", "run", ".", baselinePath, candidatePath)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("compare command failed: %v\n%s", err, output)
	}
	var report scenarioevidence.ComparisonReport
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatalf("decode command output: %v\n%s", err, output)
	}
	if report.SchemaVersion != scenarioevidence.ComparisonSchemaVersion {
		t.Fatalf("unexpected report schema: %s", report.SchemaVersion)
	}
	if report.AutomatedConclusion.CrossCustomerValidated {
		t.Fatal("command emitted an unsupported cross-customer conclusion")
	}
}
