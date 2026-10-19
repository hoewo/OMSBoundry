//go:build integration

package httpserver

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"omsbizfoundry/design-operations/internal/auth"
	"omsbizfoundry/design-operations/internal/database"
	"omsbizfoundry/design-operations/internal/design"
	"omsbizfoundry/design-operations/internal/testdatabase"
)

func TestServeProductionMaterialsForIsolatedWorkloadProbe(t *testing.T) {
	addr, project, file, keys, issuer, output := os.Getenv("MATERIAL_PROBE_ADDR"), os.Getenv("MATERIAL_PROBE_PROJECT_ID"), os.Getenv("MATERIAL_PROBE_FILE_ID"), os.Getenv("MATERIAL_PROBE_KEYS"), os.Getenv("MATERIAL_PROBE_ISSUER"), os.Getenv("MATERIAL_PROBE_OUTPUT")
	if addr == "" || project == "" || file == "" || keys == "" || issuer == "" || output == "" {
		t.Skip("explicit private process probe not selected")
	}
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := design.NewPostgresStore(environment.Pool)
	writer, _ := design.NewService(store, &materialSeedReferences{})
	actor := design.Actor{Kind: "test_subject", ID: systemSubject, TenantID: systemTenant}
	created, err := writer.Create(ctx, actor, design.CreateInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-000000000501", ProjectID: project, ProjectSourceRevision: 1, TypeKey: design.ConsultationTypeKey})
	if err != nil {
		t.Fatal(err)
	}
	id := created.WorkItem.ID
	if _, err := writer.SetOwner(ctx, actor, id, design.OwnerInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-000000000502", ExpectedRevision: 1, AssignmentID: systemAssignment, AssignmentSourceRevision: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.PutDeliverable(ctx, actor, id, "scheme", design.DeliverableInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-000000000503", ExpectedRevision: 2, RequirementKey: design.RequirementPrimary, EvidenceFileID: file, EvidenceVersion: 2, EvidenceSourceRevision: 2}); err != nil {
		t.Fatal(err)
	}
	reader, _ := design.NewService(store, &materialReadNoUpstream{})
	projection, err := reader.ReferenceMaterials(ctx, systemTenant, id)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal([]design.MaterialReferenceProjection{projection})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, raw, 0600); err != nil {
		t.Fatal(err)
	}
	legacy, err := auth.NewIAMProvider(issuer, systemTenant, keys)
	if err != nil {
		t.Fatal(err)
	}
	native, err := auth.NewProductionIAMProvider(issuer, systemTenant, keys)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewWithProduction(reader, legacy, native)
	if err != nil {
		t.Fatal(err)
	}
	before := productionMaterialFacts(t, environment.Pool)
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
	if productionMaterialFacts(t, environment.Pool) != before {
		t.Fatal("actual workload GET changed Production-owned facts")
	}
	t.Log("actual_formal_Production_process_reads_owned_digest_unchanged=true bootstrap_references_synthetic=true")
}
