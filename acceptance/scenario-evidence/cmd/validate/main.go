package main

import (
	"fmt"
	"os"
	"path/filepath"

	scenarioevidence "omsbizfoundry/scenario-evidence-acceptance"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: validate <scenario-evidence.json> [...]")
		os.Exit(2)
	}
	for _, name := range os.Args[1:] {
		data, err := os.ReadFile(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
			os.Exit(1)
		}
		if err := scenarioevidence.ValidateBytes(data); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
			os.Exit(1)
		}
		fmt.Printf("SCENARIO_EVIDENCE_STATUS=passed file=%s\n", filepath.Clean(name))
	}
}
