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

	"omsbizfoundry/money-flow-ledger/internal/config"
	"omsbizfoundry/money-flow-ledger/internal/database"
	"omsbizfoundry/money-flow-ledger/internal/partymigration"
)

func runPartyDirectoryProject(ctx context.Context, arguments []string) error {
	flags := flag.NewFlagSet("ops party-directory-project", flag.ContinueOnError)
	inputPath := flags.String("input", "", "reviewed Party Directory runtime projection request JSON file")
	partyBaseURL := flags.String("party-base-url", strings.TrimSpace(os.Getenv("PARTY_BASE_URL")), "Party Directory internal base URL")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || strings.TrimSpace(*inputPath) == "" {
		return fmt.Errorf("party-directory-project requires --input and accepts no positional arguments")
	}
	request, err := readRuntimeProjectionRequest(*inputPath)
	if err != nil {
		return err
	}
	tenantID := strings.ToLower(strings.TrimSpace(os.Getenv("IAM_TENANT_ID")))
	if strings.ToLower(strings.TrimSpace(request.TenantID)) != tenantID {
		return fmt.Errorf("runtime projection tenant_id must equal IAM_TENANT_ID")
	}
	credentialURL := strings.TrimSpace(os.Getenv("IAM_SERVICE_CREDENTIAL_URL"))
	workloadID := strings.TrimSpace(os.Getenv("WORKLOAD_ID"))
	workloadSubjectID := strings.TrimSpace(os.Getenv("WORKLOAD_SUBJECT_ID"))
	workloadKeyFile := strings.TrimSpace(os.Getenv("WORKLOAD_KEY_FILE"))
	if credentialURL == "" || workloadID == "" || workloadSubjectID == "" || workloadKeyFile == "" {
		return fmt.Errorf("IAM_SERVICE_CREDENTIAL_URL, IAM_TENANT_ID, WORKLOAD_ID, WORKLOAD_SUBJECT_ID, and WORKLOAD_KEY_FILE are required")
	}
	credentials, err := workloadidentity.NewTokenSource(credentialURL, tenantID, workloadID, workloadSubjectID, workloadKeyFile, 5*time.Second, 30*time.Second)
	if err != nil {
		return fmt.Errorf("configure workload identity: %w", err)
	}
	validator, err := partymigration.NewHTTPPartyValidator(5*time.Second, partymigration.PartyEndpoint{
		BaseURL: strings.TrimSpace(*partyBaseURL), Audience: "party-directory", Action: "party-directory.reference.read", Credentials: credentials,
	})
	if err != nil {
		return err
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
	if _, err = database.RequireCurrentSchema(operationContext, pool); err != nil {
		return err
	}
	result, err := partymigration.ApplyRuntimeProjection(operationContext, pool, validator, request, time.Now().UTC())
	if err != nil {
		return err
	}
	return writeJSON(result)
}

func readRuntimeProjectionRequest(path string) (partymigration.RuntimeProjectionRequest, error) {
	file, err := os.Open(path)
	if err != nil {
		return partymigration.RuntimeProjectionRequest{}, fmt.Errorf("open Party Directory runtime projection request: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	var request partymigration.RuntimeProjectionRequest
	if err := decoder.Decode(&request); err != nil {
		return partymigration.RuntimeProjectionRequest{}, fmt.Errorf("decode Party Directory runtime projection request: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return partymigration.RuntimeProjectionRequest{}, fmt.Errorf("Party Directory runtime projection request must contain one JSON document")
	}
	return request, nil
}
