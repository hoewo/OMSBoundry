package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	workloadidentity "omsbizfoundry/workload-identity"

	"omsbizfoundry/contract-ledger/internal/config"
	"omsbizfoundry/contract-ledger/internal/database"
	"omsbizfoundry/contract-ledger/internal/partymigration"
)

func runPartyRuntimeTransition(ctx context.Context, arguments []string) error {
	flags := flag.NewFlagSet("ops party-runtime-transition", flag.ContinueOnError)
	inputPath := flags.String("input", "", "reviewed Party runtime transition request JSON file")
	partyBaseURL := flags.String("party-base-url", strings.TrimSpace(os.Getenv("PARTY_BASE_URL")), "Party Directory internal base URL")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || strings.TrimSpace(*inputPath) == "" {
		return fmt.Errorf("party-runtime-transition requires --input and accepts no positional arguments")
	}
	request, err := readRuntimeTransitionRequest(*inputPath)
	if err != nil {
		return err
	}
	tenantID := strings.ToLower(strings.TrimSpace(os.Getenv("IAM_TENANT_ID")))
	if strings.ToLower(strings.TrimSpace(request.TenantID)) != tenantID {
		return fmt.Errorf("runtime transition tenant_id must equal IAM_TENANT_ID")
	}

	var validator partymigration.TargetValidator
	if strings.EqualFold(strings.TrimSpace(request.Action), partymigration.RuntimeActionCutover) {
		credentialURL := strings.TrimSpace(os.Getenv("IAM_SERVICE_CREDENTIAL_URL"))
		workloadID := strings.TrimSpace(os.Getenv("WORKLOAD_ID"))
		workloadSubjectID := strings.TrimSpace(os.Getenv("WORKLOAD_SUBJECT_ID"))
		workloadKeyFile := strings.TrimSpace(os.Getenv("WORKLOAD_KEY_FILE"))
		if credentialURL == "" || workloadID == "" || workloadSubjectID == "" || workloadKeyFile == "" {
			return fmt.Errorf("IAM_SERVICE_CREDENTIAL_URL, IAM_TENANT_ID, WORKLOAD_ID, WORKLOAD_SUBJECT_ID, and WORKLOAD_KEY_FILE are required for cutover")
		}
		credentials, credentialErr := workloadidentity.NewTokenSource(credentialURL, tenantID, workloadID, workloadSubjectID, workloadKeyFile, 5*time.Second, 30*time.Second)
		if credentialErr != nil {
			return fmt.Errorf("configure workload identity: %w", credentialErr)
		}
		validator, err = partymigration.NewHTTPPartyValidator(5*time.Second, partymigration.PartyEndpoint{
			BaseURL: strings.TrimSpace(*partyBaseURL), Audience: "party-directory", Action: "party-directory.reference.read", Credentials: credentials,
		})
		if err != nil {
			return err
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	operationContext, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	pool, err := database.Open(operationContext, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if _, err := database.RequireCurrentSchema(operationContext, pool); err != nil {
		return err
	}
	result, err := partymigration.ApplyRuntimeTransition(operationContext, pool, validator, request, time.Now().UTC())
	if err != nil {
		return err
	}
	return writeJSON(result)
}

func readRuntimeTransitionRequest(path string) (partymigration.RuntimeTransitionRequest, error) {
	file, err := os.Open(path)
	if err != nil {
		return partymigration.RuntimeTransitionRequest{}, fmt.Errorf("open Party runtime transition request: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	var request partymigration.RuntimeTransitionRequest
	if err := decoder.Decode(&request); err != nil {
		return partymigration.RuntimeTransitionRequest{}, fmt.Errorf("decode Party runtime transition request: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return partymigration.RuntimeTransitionRequest{}, fmt.Errorf("Party runtime transition request must contain one JSON document")
	}
	return request, nil
}
