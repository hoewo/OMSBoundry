package main

import (
	"encoding/json"
	"fmt"
	"os"

	scenarioevidence "omsbizfoundry/scenario-evidence-acceptance"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: compare BASELINE_SCENARIO_JSON CANDIDATE_SCENARIO_JSON")
		os.Exit(2)
	}
	baseline, err := readDocument(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenario comparison failed: %v\n", err)
		os.Exit(1)
	}
	candidate, err := readDocument(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenario comparison failed: %v\n", err)
		os.Exit(1)
	}
	report, err := scenarioevidence.CompareScenarios(baseline, candidate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenario comparison failed: %v\n", err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintf(os.Stderr, "write scenario comparison: %v\n", err)
		os.Exit(1)
	}
}

func readDocument(path string) (scenarioevidence.Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return scenarioevidence.Document{}, fmt.Errorf("read %s: %w", path, err)
	}
	document, err := scenarioevidence.Parse(data)
	if err != nil {
		return scenarioevidence.Document{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return document, nil
}
