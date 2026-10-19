package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	workloadidentity "omsbizfoundry/workload-identity"
)

const maxResponseBytes = 1 << 20

func main() {
	if len(os.Args) != 6 {
		fatal("usage: reference-probe TARGET_URL AUDIENCE ACTION RESOURCE_TYPE RESOURCE_ID")
	}
	endpoint := requiredEnv("IAM_SERVICE_CREDENTIAL_URL")
	tenantID := requiredEnv("IAM_TENANT_ID")
	workloadID := requiredEnv("WORKLOAD_ID")
	subjectID := requiredEnv("WORKLOAD_SUBJECT_ID")
	keyFile := requiredEnv("WORKLOAD_KEY_FILE")
	targetURL, audience, action := os.Args[1], os.Args[2], os.Args[3]
	resourceType, resourceID := os.Args[4], os.Args[5]

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tokens, err := workloadidentity.NewTokenSource(endpoint, tenantID, workloadID, subjectID, keyFile, 5*time.Second, 15*time.Second)
	if err != nil {
		fatal("configure credential source: %v", err)
	}
	token, err := tokens.Credential(ctx, audience, action, resourceType, resourceID)
	if err != nil {
		fatal("exchange credential: %v", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		fatal("build request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		fatal("request target: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil || len(body) > maxResponseBytes {
		fatal("read target response")
	}
	fmt.Printf("HTTP %d\n%s\n", response.StatusCode, body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		os.Exit(1)
	}
}

func requiredEnv(name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		fatal("%s is required", name)
	}
	return value
}

func fatal(format string, arguments ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", arguments...)
	os.Exit(1)
}
