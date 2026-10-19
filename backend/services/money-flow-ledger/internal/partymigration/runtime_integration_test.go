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

func TestRuntimeCutoverRollbackPreservesMappingsAndFreezesLocalAuthority(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 28, 1, 2, 3, 0, time.UTC)
	if _, err := environment.Pool.Exec(ctx, `INSERT INTO parties(id,kind,legal_name,revision,created_at,updated_at) VALUES($1,'organization','本地公司',1,$2,$2)`, testLocalID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO internal_transfers(
			id,description,ledger_owner_party_id,ledger_owner_party_revision,ledger_owner_kind,ledger_owner_legal_name,
			amount,currency,occurred_on,source_account_reference,destination_account_reference,source_kind,
			manual_source_reference,recorded_by_actor_kind,recorded_by_actor_id,recorded_by_actor_display_name,recorded_at
		) VALUES(
			'10101010-1010-4010-8010-101010101010','运行时切换引用转账',$1,1,'organization','本地公司',
			100,'CNY','2026-09-28','账户-A','账户-B','manual','runtime-test','user','runtime-reviewer','迁移审核员',$2
		)
	`, testLocalID, now); err != nil {
		t.Fatal(err)
	}
	digest, err := snapshotDigest(LegacyParty{ID: testLocalID, Kind: "organization", LegalName: "本地公司", Revision: 1, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	mapping := validMappingRequest()
	mapping.Mappings[0].LocalSnapshotSHA256 = digest
	validator := &countingValidator{value: TargetSnapshot{PartyID: testTargetID, Revision: 3, Kind: "organization", LegalName: "目标公司"}}
	if _, err := ApplyMappings(ctx, environment.Pool, validator, mapping, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	cutover := RuntimeTransitionRequest{
		Format: RuntimeTransitionRequestFormat, TenantID: testTenantID,
		CommandID: "99999999-9999-4999-8999-999999999999", Action: RuntimeActionCutover,
		MigrationBatchID: testBatchID, ReviewedBySubjectID: testSubjectID,
		ReviewedByDisplayName: "迁移审核员", Reason: "映射已人工复核",
	}
	result, err := ApplyRuntimeTransition(ctx, environment.Pool, validator, cutover, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if result.State.Mode != RuntimeModeDirectory || result.MappedPartyCount != 1 || result.ReferencedPartyCount != 1 {
		t.Fatalf("unexpected cutover result: %#v", result)
	}
	registry := NewRuntimeRegistry(environment.Pool)
	resolved, err := registry.ResolveReferences(ctx, testTenantID, []string{testLocalID})
	if err != nil || len(resolved) != 1 || resolved[0].PartyID != testTargetID || resolved[0].Revision != 3 || resolved[0].Source != RuntimeModeDirectory {
		t.Fatalf("directory runtime reference = %#v, %v", resolved, err)
	}
	rollback := cutover
	rollback.CommandID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	rollback.Action = RuntimeActionRollback
	rollback.Reason = "验证可逆兼容读"
	rolledBack, err := ApplyRuntimeTransition(ctx, environment.Pool, nil, rollback, now.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	allowed, err := registry.LocalPartyWritesAllowed(ctx, testTenantID)
	if err != nil || allowed || rolledBack.State.Mode != RuntimeModeLocalCompatibility {
		t.Fatalf("rollback authority = %#v allowed=%v err=%v", rolledBack.State, allowed, err)
	}
	resolved, err = registry.ResolveReferences(ctx, testTenantID, []string{testLocalID})
	if err != nil || resolved[0].PartyID != testLocalID || resolved[0].Source != RuntimeModeLocal {
		t.Fatalf("compatibility runtime reference = %#v, %v", resolved, err)
	}
	if _, err := environment.Pool.Exec(ctx, `UPDATE party_runtime_transitions SET reason='changed'`); err == nil {
		t.Fatal("immutable runtime transition was updated")
	}
	secondLocal := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	if _, err := environment.Pool.Exec(ctx, `INSERT INTO parties(id,kind,legal_name,revision,created_at,updated_at) VALUES($1,'organization','后加主体',1,$2,$2)`, secondLocal, now); err != nil {
		t.Fatal(err)
	}
	secondDigest, _ := snapshotDigest(LegacyParty{ID: secondLocal, Kind: "organization", LegalName: "后加主体", Revision: 1, CreatedAt: now, UpdatedAt: now})
	late := validMappingRequest()
	late.Mappings[0].CommandID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	late.Mappings[0].LocalPartyID = secondLocal
	late.Mappings[0].LocalSnapshotSHA256 = secondDigest
	late.Mappings[0].TargetPartyID = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	validator.value.PartyID = late.Mappings[0].TargetPartyID
	if _, err := ApplyMappings(ctx, environment.Pool, validator, late, now.Add(4*time.Minute)); !errors.Is(err, ErrMappingConflict) {
		t.Fatalf("sealed mapping batch error = %v", err)
	}
}
