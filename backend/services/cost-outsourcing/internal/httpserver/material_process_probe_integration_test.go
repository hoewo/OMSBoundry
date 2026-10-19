//go:build integration

package httpserver

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"omsbizfoundry/cost-outsourcing/internal/auth"
	"omsbizfoundry/cost-outsourcing/internal/costoutsourcing"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

// Explicit acceptance source process. Owned seed writes use this application's
// Service/Store and declared synthetic bootstrap references; GET uses the
// actual formal verifier and cannot call any upstream resolver.
func TestServeCostMaterialsForIsolatedWorkloadProbe(t *testing.T) {
	addr, project, file, keys, issuer, output := os.Getenv("MATERIAL_PROBE_ADDR"), os.Getenv("MATERIAL_PROBE_PROJECT_ID"), os.Getenv("MATERIAL_PROBE_FILE_ID"), os.Getenv("MATERIAL_PROBE_KEYS"), os.Getenv("MATERIAL_PROBE_ISSUER"), os.Getenv("MATERIAL_PROBE_OUTPUT")
	if addr == "" || project == "" || file == "" || keys == "" || issuer == "" || output == "" {
		t.Skip("explicit private process probe not selected")
	}
	ctx := context.Background()
	pool, cleanup := httpIntegrationPool(t, ctx)
	defer cleanup()
	store, _ := costoutsourcing.NewPostgresStore(pool)
	writer, _ := costoutsourcing.NewService(store, &materialSeedResolver{})
	actor := costoutsourcing.Actor{ID: httpSubject, TenantID: httpTenant, Kind: "test_subject", CredentialID: "synthetic-bootstrap", ProjectIDs: []string{project}, Actions: []string{costoutsourcing.ActionCostCreate, costoutsourcing.ActionCostEvidenceManage, costoutsourcing.ActionEngageCreate, costoutsourcing.ActionEngageEvidenceManage}}
	files := []costoutsourcing.EvidenceRef{{FileID: file, Version: 2, Slot: "supporting_document"}}
	cost, err := writer.RecordCost(ctx, actor, costoutsourcing.RecordCostInput{CommandID: httpCommand, ProjectRef: costoutsourcing.ProjectRef{ID: project, Revision: 7}, Category: costoutsourcing.CategoryOther, OccurredOn: "2026-10-01", Currency: "CNY", AmountMinor: 125, Description: "synthetic workload fixture", EvidenceRefs: files})
	if err != nil {
		t.Fatal(err)
	}
	engagement, err := writer.RecordEngagement(ctx, actor, costoutsourcing.RecordEngagementInput{CommandID: httpCommand2, ProjectRef: costoutsourcing.ProjectRef{ID: project, Revision: 7}, ProviderRef: costoutsourcing.PartyRef{ID: httpParty, Revision: 5}, Scope: "synthetic workload fixture", Currency: "CNY", AmountMinor: 500, EvidenceRefs: files})
	if err != nil {
		t.Fatal(err)
	}
	cp, err := costoutsourcing.CostMaterialReference(cost)
	if err != nil {
		t.Fatal(err)
	}
	ep, err := costoutsourcing.EngagementMaterialReference(engagement)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal([]costoutsourcing.MaterialReferenceProjection{cp, ep})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, raw, 0600); err != nil {
		t.Fatal(err)
	}
	reader, _ := costoutsourcing.NewService(store, &materialReadNoUpstream{})
	provider, err := auth.NewIAMProvider(issuer, httpTenant, keys)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := New(reader, provider)
	if err != nil {
		t.Fatal(err)
	}
	before := costMaterialHTTPFacts(t, pool)
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
	if costMaterialHTTPFacts(t, pool) != before {
		t.Fatal("actual workload GET changed Cost-owned facts")
	}
	t.Log("actual_formal_Cost_and_Engagement_process_reads_owned_digest_unchanged=true bootstrap_references_synthetic=true")
}
