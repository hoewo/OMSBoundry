package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	scenarioevidence "omsbizfoundry/scenario-evidence-acceptance"
)

func main() {
	required, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	failed := false
	for _, name := range files {
		data, err := os.ReadFile(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
			failed = true
			continue
		}
		document, err := scenarioevidence.Parse(data)
		if err == nil {
			err = scenarioevidence.Validate(document)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
			failed = true
			continue
		}
		report := scenarioevidence.AssessReadiness(document)
		fmt.Printf("SCENARIO_READINESS=%s file=%s\n", report.Stage, filepath.Clean(name))
		for _, gap := range report.EvidenceReadyGaps {
			fmt.Printf("EVIDENCE_READY_GAP=%s: %s\n", gap.Path, gap.Message)
		}
		if len(report.EvidenceReadyGaps) == 0 {
			for _, gap := range report.FinalGaps {
				fmt.Printf("FINAL_READY_GAP=%s: %s\n", gap.Path, gap.Message)
			}
		}
		for _, finding := range report.BoundaryFindings {
			fmt.Printf("BOUNDARY_FINDING=%s: %s\n", finding.Path, finding.Message)
		}
		if required != "" && !report.Meets(scenarioevidence.ReadinessStage(required)) {
			fmt.Fprintf(os.Stderr, "%s: readiness %s does not meet required stage %s\n", name, report.Stage, required)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

func parseArgs(args []string) (string, []string, error) {
	required := ""
	var files []string
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--require":
			index++
			if index >= len(args) {
				return "", nil, fmt.Errorf("usage: readiness [--require evidence_ready|final_ready] <scenario-evidence.json> [...]")
			}
			required = strings.TrimSpace(args[index])
			if required != string(scenarioevidence.ReadinessEvidenceReady) && required != string(scenarioevidence.ReadinessFinalReady) {
				return "", nil, fmt.Errorf("unsupported readiness stage %q", required)
			}
		default:
			files = append(files, args[index])
		}
	}
	if len(files) == 0 {
		return "", nil, fmt.Errorf("usage: readiness [--require evidence_ready|final_ready] <scenario-evidence.json> [...]")
	}
	return required, files, nil
}
