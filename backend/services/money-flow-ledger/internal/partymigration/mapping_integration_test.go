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

type countingValidator struct {
	calls int
	value TargetSnapshot
}

func (v *countingValidator) VerifyParty(_ context.Context, _, _ string, _ int64) (TargetSnapshot, error) {
	v.calls++
	return v.value, nil
}

func TestReviewedMappingIsImmutableReplayableAndFailsClosedOnSourceDrift(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 27, 1, 2, 3, 0, time.UTC)
	if _, err := environment.Pool.Exec(ctx, `INSERT INTO parties(id,kind,legal_name,revision,created_at,updated_at) VALUES($1,'organization','本地公司',1,$2,$2)`, testLocalID, now); err != nil {
		t.Fatal(err)
	}
	digest, err := snapshotDigest(LegacyParty{ID: testLocalID, Kind: "organization", LegalName: "本地公司", Revision: 1, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	request := validMappingRequest()
	request.Mappings[0].LocalSnapshotSHA256 = digest
	validator := &countingValidator{value: TargetSnapshot{PartyID: testTargetID, Revision: 3, Kind: "organization", LegalName: "目标公司"}}
	result, err := ApplyMappings(ctx, environment.Pool, validator, request, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if result.Recorded != 1 || result.Replayed != 0 || result.ReadyForCutover || validator.calls != 1 {
		t.Fatalf("unexpected result: %#v calls=%d", result, validator.calls)
	}
	replay, err := ApplyMappings(ctx, environment.Pool, validator, request, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if replay.Recorded != 0 || replay.Replayed != 1 || validator.calls != 1 {
		t.Fatalf("unexpected replay: %#v calls=%d", replay, validator.calls)
	}
	if _, err := environment.Pool.Exec(ctx, `UPDATE party_migration_mappings SET review_reason='changed'`); err == nil {
		t.Fatal("immutable mapping was updated")
	}

	second := request
	second.MigrationBatchID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	second.Mappings = append([]MappingDecision(nil), request.Mappings...)
	second.Mappings[0].CommandID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	if _, err := environment.Pool.Exec(ctx, `UPDATE parties SET legal_name='已变化',revision=2,updated_at=$2 WHERE id=$1`, testLocalID, now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyMappings(ctx, environment.Pool, validator, second, now.Add(4*time.Minute)); !errors.Is(err, ErrSourceDrift) {
		t.Fatalf("source drift error = %v", err)
	}
	var count int
	if err := environment.Pool.QueryRow(ctx, `SELECT count(*) FROM party_migration_mappings`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("mapping count after failed batch = %d", count)
	}
}
