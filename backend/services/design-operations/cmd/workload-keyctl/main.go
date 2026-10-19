package main

import (
	"fmt"
	"os"
	"time"

	workloadidentity "omsbizfoundry/workload-identity"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	if len(arguments) != 1 {
		return fmt.Errorf("usage: workload-keyctl init|rotate|prune")
	}
	privatePath, publicPath := os.Getenv("WORKLOAD_KEY_FILE"), os.Getenv("WORKLOAD_PUBLIC_KEY_FILE")
	workloadID, subjectID := os.Getenv("WORKLOAD_ID"), os.Getenv("WORKLOAD_SUBJECT_ID")
	now := time.Now().UTC()
	switch arguments[0] {
	case "init":
		return workloadidentity.Initialize(privatePath, publicPath, workloadID, subjectID, now)
	case "rotate":
		keyID, err := workloadidentity.Rotate(privatePath, publicPath, workloadID, subjectID, now, 2*time.Minute)
		if err == nil {
			fmt.Println(keyID)
		}
		return err
	case "prune":
		return workloadidentity.Prune(privatePath, publicPath, workloadID, subjectID, now)
	default:
		return fmt.Errorf("unknown workload key operation %q", arguments[0])
	}
}
