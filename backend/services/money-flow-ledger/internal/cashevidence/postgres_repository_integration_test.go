//go:build integration

package cashevidence

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/cashtransaction"
	"omsbizfoundry/money-flow-ledger/internal/database"
	"omsbizfoundry/money-flow-ledger/internal/party"
	"omsbizfoundry/money-flow-ledger/internal/testdatabase"
)

func TestPostgresEvidenceRevisionReplayConflictAndImmutability(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	actor := audit.ActorRef{Kind: audit.ActorKindIAMSubject, ID: "55555555-5555-4555-8555-555555555555", DisplayName: "资金会计"}
	partyService := party.NewService(party.NewPostgresRepository(environment.Pool))
	owner, err := partyService.Create(ctx, party.CreateInput{CommandID: "66666666-6666-4666-8666-666666666666", Kind: "organization", LegalName: "资质单位", Actor: actor})
	if err != nil {
		t.Fatal(err)
	}
	counterparty, err := partyService.Create(ctx, party.CreateInput{CommandID: "77777777-7777-4777-8777-777777777777", Kind: "organization", LegalName: "甲方", Actor: actor})
	if err != nil {
		t.Fatal(err)
	}
	cashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	cash, err := cashService.Create(ctx, cashtransaction.CreateInput{
		CommandID: "88888888-8888-4888-8888-888888888888", Type: "receipt", LedgerOwnerPartyID: owner.Party.ID,
		CounterpartyPartyID: counterparty.Party.ID, Description: "工程回款", Amount: "100.01", Currency: "CNY",
		OccurredOn: "2026-10-13", AccountReference: "基本户", ManualSourceReference: "project:aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Actor: actor,
	})
	if err != nil {
		t.Fatal(err)
	}
	validator := &validatorStub{ref: FileReference{TenantID: testTenant, FileID: testFile, Version: 1, Revision: 1, Status: "available", Name: "回单.pdf"}}
	service, _ := NewService(NewPostgresRepository(environment.Pool), validator)
	input := ReplaceInput{CommandID: testCommand, ExpectedRevision: 0, Reason: "登记回款凭证", EvidenceRefs: []Ref{{FileID: testFile, Version: 1, Purpose: "receipt_proof"}}, Actor: actor, TenantID: testTenant, CashTransactionID: cash.Transaction.ID}
	created, err := service.Replace(ctx, input)
	if err != nil || created.Replayed || created.Snapshot.Revision != 1 || created.Snapshot.EvidenceRefs[0].DisplayName != "回单.pdf" {
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
	stale.CommandID = "99999999-9999-4999-8999-999999999999"
	stale.ExpectedRevision = 0
	if _, err = service.Replace(ctx, stale); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale revision error=%v", err)
	}
	current, found, err := service.Current(ctx, cash.Transaction.ID)
	if err != nil || !found || current.Revision != 1 || len(current.EvidenceRefs) != 1 || current.Actor.ID != actor.ID {
		t.Fatalf("current=%#v found=%t err=%v", current, found, err)
	}
	_, err = environment.Pool.Exec(ctx, `UPDATE cash_transaction_evidence_revisions SET reason='tampered' WHERE cash_transaction_id=$1`, cash.Transaction.ID)
	var databaseError *pgconn.PgError
	if !errors.As(err, &databaseError) || databaseError.Code != "P0001" {
		t.Fatalf("immutable update error=%v", err)
	}
}
