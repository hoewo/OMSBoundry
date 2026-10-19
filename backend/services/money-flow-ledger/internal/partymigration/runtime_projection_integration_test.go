//go:build integration

package partymigration

import (
	"context"
	"errors"
	"testing"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/database"
	"omsbizfoundry/money-flow-ledger/internal/testdatabase"
)

func TestRuntimeProjectionAddsExactDirectorySnapshotWithoutReopeningLocalAuthority(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	if _, err := environment.Pool.Exec(ctx, `INSERT INTO parties(id,kind,legal_name,revision,created_at,updated_at) VALUES($1,'organization','迁移前本方',1,$2,$2)`, testLocalID, now); err != nil {
		t.Fatal(err)
	}
	localDigest, err := snapshotDigest(LegacyParty{ID: testLocalID, Kind: "organization", LegalName: "迁移前本方", Revision: 1, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	mapping := validMappingRequest()
	mapping.Mappings[0].LocalSnapshotSHA256 = localDigest
	validator := &countingValidator{value: TargetSnapshot{PartyID: testTargetID, Revision: 3, Kind: "organization", LegalName: "目录本方"}}
	if _, err = ApplyMappings(ctx, environment.Pool, validator, mapping, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	cutover := RuntimeTransitionRequest{
		Format: RuntimeTransitionRequestFormat, TenantID: testTenantID,
		CommandID: "30303030-3030-4030-8030-303030303030", Action: RuntimeActionCutover,
		MigrationBatchID: testBatchID, ReviewedBySubjectID: testSubjectID,
		ReviewedByDisplayName: "迁移审核员", Reason: "既有主体映射已复核",
	}
	if _, err = ApplyRuntimeTransition(ctx, environment.Pool, validator, cutover, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}

	request := validRuntimeProjectionRequest()
	validator.value = TargetSnapshot{PartyID: request.TargetPartyID, Revision: request.TargetPartyRevision, Kind: "individual", LegalName: "切换后新增人员主体"}
	result, err := ApplyRuntimeProjection(ctx, environment.Pool, validator, request, now.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if result.Replayed || result.LocalPartyRevision != 1 || result.Target.Kind != "individual" || result.RuntimeTransitionSequence < 1 {
		t.Fatalf("unexpected runtime projection result: %#v", result)
	}
	resolved, err := NewRuntimeRegistry(environment.Pool).ResolveReferences(ctx, testTenantID, []string{result.LocalPartyID})
	if err != nil || len(resolved) != 1 || resolved[0].PartyID != request.TargetPartyID || resolved[0].Revision != request.TargetPartyRevision || resolved[0].Source != RuntimeModeDirectory {
		t.Fatalf("projected runtime reference = %#v, %v", resolved, err)
	}
	var projections, localParties, audits int
	if err = environment.Pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM party_directory_runtime_projections WHERE tenant_id=$1),
		(SELECT count(*) FROM parties WHERE id=$2),
		(SELECT count(*) FROM audit_entries WHERE action='party_directory_runtime_projected' AND command_id=$3)
	`, testTenantID, result.LocalPartyID, request.CommandID).Scan(&projections, &localParties, &audits); err != nil {
		t.Fatal(err)
	}
	if projections != 1 || localParties != 1 || audits != 1 {
		t.Fatalf("runtime projection facts = %d/%d/%d", projections, localParties, audits)
	}
	replay, err := ApplyRuntimeProjection(ctx, environment.Pool, validator, request, now.Add(4*time.Minute))
	if err != nil || !replay.Replayed || replay.LocalPartyID != result.LocalPartyID {
		t.Fatalf("runtime projection replay = %#v, %v", replay, err)
	}
	changed := request
	changed.TargetPartyRevision++
	if _, err = ApplyRuntimeProjection(ctx, environment.Pool, validator, changed, now.Add(5*time.Minute)); !errors.Is(err, ErrRuntimeProjectionCommandConflict) {
		t.Fatalf("changed command replay error = %v", err)
	}
	duplicate := request
	duplicate.CommandID = "40404040-4040-4040-8040-404040404040"
	if _, err = ApplyRuntimeProjection(ctx, environment.Pool, validator, duplicate, now.Add(6*time.Minute)); !errors.Is(err, ErrRuntimeProjectionConflict) {
		t.Fatalf("duplicate target projection error = %v", err)
	}
	projector := NewRuntimeDirectoryProjector(environment.Pool, validator)
	projector.now = func() time.Time { return now.Add(7 * time.Minute) }
	existing, err := projector.ResolveOrProject(ctx, testTenantID, request.TargetPartyID, request.TargetPartyRevision, testSubjectID, "资金登记人")
	if err != nil || existing.LocalPartyID != result.LocalPartyID || existing.PartyID != request.TargetPartyID || existing.Revision != request.TargetPartyRevision {
		t.Fatalf("resolve existing runtime projection = %#v, %v", existing, err)
	}
	newTargetID := "50505050-5050-4050-8050-505050505050"
	validator.value = TargetSnapshot{PartyID: newTargetID, Revision: 2, Kind: "organization", LegalName: "资金登记时新增主体"}
	projected, err := projector.ResolveOrProject(ctx, testTenantID, newTargetID, 2, testSubjectID, "资金登记人")
	if err != nil || projected.LocalPartyID == "" || projected.PartyID != newTargetID || projected.Revision != 2 || projected.Source != RuntimeModeDirectory {
		t.Fatalf("resolve or project new runtime Party = %#v, %v", projected, err)
	}
	replayed, err := projector.ResolveOrProject(ctx, testTenantID, newTargetID, 2, testSubjectID, "资金登记人")
	if err != nil || replayed.LocalPartyID != projected.LocalPartyID {
		t.Fatalf("resolve projected runtime Party = %#v, %v", replayed, err)
	}
	if err = environment.Pool.QueryRow(ctx, `SELECT count(*) FROM party_directory_runtime_projections WHERE tenant_id=$1`, testTenantID).Scan(&projections); err != nil || projections != 2 {
		t.Fatalf("runtime projection count = %d, %v", projections, err)
	}
	if _, err = environment.Pool.Exec(ctx, `UPDATE parties SET legal_name='tampered',updated_at=now() WHERE id=$1`, result.LocalPartyID); err == nil {
		t.Fatal("projected Money Party snapshot mutation was accepted")
	}
	if _, err = environment.Pool.Exec(ctx, `UPDATE party_directory_runtime_projections SET review_reason='tampered' WHERE tenant_id=$1`, testTenantID); err == nil {
		t.Fatal("runtime projection review mutation was accepted")
	}
	allowed, err := NewRuntimeRegistry(environment.Pool).LocalPartyWritesAllowed(ctx, testTenantID)
	if err != nil || allowed {
		t.Fatalf("runtime projection reopened local Party authority: allowed=%v err=%v", allowed, err)
	}
}

func TestRuntimeProjectionRejectsLocalMode(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	request := validRuntimeProjectionRequest()
	validator := &countingValidator{value: TargetSnapshot{PartyID: request.TargetPartyID, Revision: request.TargetPartyRevision, Kind: "individual", LegalName: "未切换人员"}}
	if _, err := ApplyRuntimeProjection(ctx, environment.Pool, validator, request, time.Now().UTC()); !errors.Is(err, ErrRuntimeProjectionState) {
		t.Fatalf("local-mode runtime projection error = %v", err)
	}
	var parties, projections int
	if err := environment.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM parties),(SELECT count(*) FROM party_directory_runtime_projections)`).Scan(&parties, &projections); err != nil {
		t.Fatal(err)
	}
	if parties != 0 || projections != 0 {
		t.Fatalf("failed runtime projection wrote facts: parties=%d projections=%d", parties, projections)
	}
}
