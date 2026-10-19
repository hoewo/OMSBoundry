//go:build integration

package httpserver

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"omsbizfoundry/approval-audit-results/internal/auth"
	"omsbizfoundry/approval-audit-results/internal/results"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

func TestServeResultsMaterialsForIsolatedWorkloadProbe(t *testing.T) {
	addr, project, file, keys, issuer, output := os.Getenv("MATERIAL_PROBE_ADDR"), os.Getenv("MATERIAL_PROBE_PROJECT_ID"), os.Getenv("MATERIAL_PROBE_FILE_ID"), os.Getenv("MATERIAL_PROBE_KEYS"), os.Getenv("MATERIAL_PROBE_ISSUER"), os.Getenv("MATERIAL_PROBE_OUTPUT")
	if addr == "" || project == "" || file == "" || keys == "" || issuer == "" || output == "" {
		t.Skip("explicit private process probe not selected")
	}
	ctx := context.Background()
	pool, cleanup := httpIntegrationPool(t, ctx)
	defer cleanup()
	store, _ := results.NewPostgresStore(pool)
	writer, _ := results.NewService(store, &materialSeedResolver{})
	input := results.RecordInput{CommandID: httpCommand, ProjectRef: results.ProjectRef{ID: project, Revision: 7}, Kind: results.KindApproval, Currency: "CNY", SourceRef: results.SourceRef{Application: "contract-ledger", ResourceType: "settlement", ResourceID: httpContract, Revision: 4}, EvidenceRefs: []results.EvidenceRef{{FileID: file, Version: 2, Slot: "approval_document"}}, ExternalConfirmedBy: "synthetic bootstrap", ExternalConfirmedAt: time.Now().UTC()}
	value, err := writer.Record(ctx, results.Actor{ID: httpSubject, TenantID: httpTenant, Kind: "test_subject", CredentialID: "synthetic-bootstrap"}, input)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := results.MaterialReference(value)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal([]results.MaterialReferenceProjection{projection})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, raw, 0600); err != nil {
		t.Fatal(err)
	}
	reader, _ := results.NewService(store, &materialReadNoUpstream{})
	provider, err := auth.NewIAMProvider(issuer, httpTenant, keys)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := New(reader, provider)
	if err != nil {
		t.Fatal(err)
	}
	before := resultsMaterialHTTPFacts(t, pool)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 2 * time.Second}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	wait, stop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	select {
	case <-wait.Done():
	case err := <-served:
		t.Fatalf("source stopped early: %v", err)
	}
	shutdown, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdown); err != nil {
		t.Fatal(err)
	}
	if resultsMaterialHTTPFacts(t, pool) != before {
		t.Fatal("actual workload GET changed Results-owned facts")
	}
	t.Log("actual_formal_Results_process_reads_owned_digest_unchanged=true bootstrap_references_synthetic=true")
}
