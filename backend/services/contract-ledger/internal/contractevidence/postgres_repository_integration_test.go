//go:build integration

package contractevidence

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"omsbizfoundry/contract-ledger/internal/audit"
	"omsbizfoundry/contract-ledger/internal/contract"
	"omsbizfoundry/contract-ledger/internal/database"
	"omsbizfoundry/contract-ledger/internal/referenceoa"
	"omsbizfoundry/contract-ledger/internal/testdatabase"
)

func TestPostgresContractEvidenceRevisionReplayConflictAndImmutability(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	actor := audit.ActorRef{Kind: audit.ActorKindIAMSubject, ID: "55555555-5555-4555-8555-555555555555", DisplayName: "合同会计"}
	contractService := contract.NewService(contract.NewPostgresRepository(environment.Pool))
	value, err := contractService.CreateDraft(ctx, contract.CreateDraftInput{
		CommandID: "66666666-6666-4666-8666-666666666666", NumberNamespace: "OMS", ContractNumber: "HT-001",
		Title: "项目合同", Category: "项目", Currency: "CNY", TotalAmount: "8000000.00", SignedOn: "2026-10-13", EffectiveOn: "2026-10-13", Actor: actor,
	})
	if err != nil {
		t.Fatal(err)
	}
	validator := &validatorStub{value: referenceoa.FileReference{ID: testFile, FileID: testFile, TenantID: testTenant, Version: 1, Revision: 1, Status: "available", Name: "合同.pdf"}}
	service, _ := NewService(NewPostgresRepository(environment.Pool), validator)
	input := ReplaceInput{CommandID: testCommand, ExpectedRevision: 0, Reason: "上传合同", EvidenceRefs: []Ref{{FileID: testFile, Version: 1, Purpose: "contract_document"}}, Actor: actor, TenantID: testTenant, ContractID: value.ID}
	created, err := service.Replace(ctx, input)
	if err != nil || created.Replayed || created.Snapshot.Revision != 1 || created.Snapshot.EvidenceRefs[0].DisplayName != "合同.pdf" {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	replayed, err := service.Replace(ctx, input)
	if err != nil || !replayed.Replayed || replayed.Snapshot.Revision != 1 {
		t.Fatalf("replayed=%#v err=%v", replayed, err)
	}
	changed := input
	changed.Reason = "不同意图"
	if _, err = service.Replace(ctx, changed); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("changed replay error=%v", err)
	}
	stale := input
	stale.CommandID = "77777777-7777-4777-8777-777777777777"
	if _, err = service.Replace(ctx, stale); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale revision error=%v", err)
	}
	current, found, err := service.Current(ctx, testTenant, value.ID)
	if err != nil || !found || current.Revision != 1 || len(current.EvidenceRefs) != 1 || current.Actor.ID != actor.ID {
		t.Fatalf("current=%#v found=%t err=%v", current, found, err)
	}
	_, err = environment.Pool.Exec(ctx, `UPDATE contract_external_evidence_revisions SET reason='tampered' WHERE contract_id=$1`, value.ID)
	var databaseError *pgconn.PgError
	if !errors.As(err, &databaseError) || databaseError.Code != "P0001" {
		t.Fatalf("immutable update error=%v", err)
	}
}
