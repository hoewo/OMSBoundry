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

func runPartyMigrationMap(ctx context.Context, arguments []string) error {
	flags := flag.NewFlagSet("ops party-migration-map", flag.ContinueOnError)
	inputPath := flags.String("input", "", "reviewed Party mapping request JSON file")
	partyBaseURL := flags.String("party-base-url", strings.TrimSpace(os.Getenv("PARTY_BASE_URL")), "Party Directory internal base URL")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || strings.TrimSpace(*inputPath) == "" {
		return fmt.Errorf("party-migration-map requires --input and accepts no positional arguments")
	}

	request, err := readMappingRequest(*inputPath)
	if err != nil {
		return err
	}
	tenantID := strings.ToLower(strings.TrimSpace(os.Getenv("IAM_TENANT_ID")))
	if strings.ToLower(strings.TrimSpace(request.TenantID)) != tenantID {
		return fmt.Errorf("mapping request tenant_id must equal IAM_TENANT_ID")
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
	if _, err := database.RequireCurrentSchema(operationContext, pool); err != nil {
		return err
	}
	result, err := partymigration.ApplyMappings(operationContext, pool, validator, request, time.Now().UTC())
	if err != nil {
		return err
	}
	return writeJSON(result)
}

func readMappingRequest(path string) (partymigration.MappingRequest, error) {
	file, err := os.Open(path)
	if err != nil {
		return partymigration.MappingRequest{}, fmt.Errorf("open Party mapping request: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 4<<20))
	decoder.DisallowUnknownFields()
	var request partymigration.MappingRequest
	if err := decoder.Decode(&request); err != nil {
		return partymigration.MappingRequest{}, fmt.Errorf("decode Party mapping request: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return partymigration.MappingRequest{}, fmt.Errorf("Party mapping request must contain one JSON document")
	}
	return request, nil
}
