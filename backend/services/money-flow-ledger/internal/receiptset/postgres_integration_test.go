//go:build integration

package receiptset

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"omsbizfoundry/money-flow-ledger/internal/allocation"
	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/cashtransaction"
	"omsbizfoundry/money-flow-ledger/internal/database"
	"omsbizfoundry/money-flow-ledger/internal/obligation"
	"omsbizfoundry/money-flow-ledger/internal/party"
	"omsbizfoundry/money-flow-ledger/internal/testdatabase"
)

func TestPostgresReceiptSetConfirmsImmutableExactCNYSnapshot(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	actor := audit.ActorRef{Kind: audit.ActorKindIAMSubject, ID: "10000000-0000-4000-8000-000000000001", DisplayName: "资金负责人"}
	partyService := party.NewService(party.NewPostgresRepository(environment.Pool))
	owner, err := partyService.Create(ctx, party.CreateInput{CommandID: "10000000-0000-4000-8000-000000000002", Kind: "organization", LegalName: "设计单位", Actor: actor})
	if err != nil {
		t.Fatal(err)
	}
	customer, err := partyService.Create(ctx, party.CreateInput{CommandID: "10000000-0000-4000-8000-000000000003", Kind: "organization", LegalName: "建设单位", Actor: actor})
	if err != nil {
		t.Fatal(err)
	}
	obligationService := obligation.NewService(obligation.NewPostgresRepository(environment.Pool))
	draft, err := obligationService.CreateDraft(ctx, obligation.CreateDraftInput{CommandID: "10000000-0000-4000-8000-000000000004", Direction: "receivable", LedgerOwnerPartyID: owner.Party.ID, CounterpartyPartyID: customer.Party.ID, Title: "设计费应收", OriginalAmount: "1065.00", Currency: "CNY", DueOn: "2026-10-31", ManualSourceReference: "合同结算", Actor: actor})
	if err != nil {
		t.Fatal(err)
	}
	confirmed, err := obligationService.Confirm(ctx, obligation.ConfirmInput{CommandID: "10000000-0000-4000-8000-000000000005", ObligationID: draft.Obligation.ID, ExpectedDraftVersion: 1, Actor: actor})
	if err != nil {
		t.Fatal(err)
	}
	cashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	cash, err := cashService.Create(ctx, cashtransaction.CreateInput{CommandID: "10000000-0000-4000-8000-000000000006", Type: "receipt", LedgerOwnerPartyID: owner.Party.ID, CounterpartyPartyID: customer.Party.ID, Description: "首期回款", Amount: "1065.00", Currency: "CNY", OccurredOn: "2026-09-28", AccountReference: "CNY-001", ManualSourceReference: "银行回单", Actor: actor})
	if err != nil {
		t.Fatal(err)
	}
	allocationService := allocation.NewService(allocation.NewPostgresRepository(environment.Pool))
	allocated, err := allocationService.Apply(ctx, allocation.ApplyInput{CommandID: "10000000-0000-4000-8000-000000000007", CashTransactionID: cash.Transaction.ID, ObligationID: confirmed.Obligation.ID, ExpectedObligationRevision: confirmed.Obligation.CurrentRevision, Amount: "1065.00", ManualSourceReference: "核销确认", Actor: actor})
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(NewPostgresRepository(environment.Pool))
	input := ConfirmInput{CommandID: "10000000-0000-4000-8000-000000000008", TenantID: "10000000-0000-4000-8000-000000000009", ProjectID: "10000000-0000-4000-8000-000000000010", AllocationIDs: []string{allocated.Allocation.ID}, Actor: actor}
	result, err := service.Confirm(ctx, input)
	if err != nil || result.Replayed || result.Set.TotalAmountMinor != 106500 || len(result.Set.Items) != 1 {
		t.Fatalf("confirm=%#v err=%v", result, err)
	}
	item := result.Set.Items[0]
	if item.TransactionID != cash.Transaction.ID || item.TransactionRevision != 1 || item.AllocationID != allocated.Allocation.ID || item.AllocationRevision != 1 || item.Currency != CurrencyCNY {
		t.Fatalf("item=%#v", item)
	}
	replay, err := service.Confirm(ctx, input)
	if err != nil || !replay.Replayed || replay.Set.ID != result.Set.ID {
		t.Fatalf("replay=%#v err=%v", replay, err)
	}
	changed := input
	changed.ProjectID = "10000000-0000-4000-8000-000000000011"
	if _, err := service.Confirm(ctx, changed); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("changed command error=%v", err)
	}
	got, err := service.Get(ctx, input.TenantID, result.Set.ID)
	if err != nil || got.TotalAmountMinor != result.Set.TotalAmountMinor || len(got.Items) != 1 {
		t.Fatalf("get=%#v err=%v", got, err)
	}
	var sets, items, commands, audits int
	if err := environment.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM receipt_sets), (SELECT count(*) FROM receipt_set_items), (SELECT count(*) FROM receipt_set_confirm_commands), (SELECT count(*) FROM audit_entries WHERE action='receipt_set_confirmed')`).Scan(&sets, &items, &commands, &audits); err != nil {
		t.Fatal(err)
	}
	if sets != 1 || items != 1 || commands != 1 || audits != 1 {
		t.Fatalf("facts sets=%d items=%d commands=%d audits=%d", sets, items, commands, audits)
	}
	for _, statement := range []string{
		`UPDATE receipt_sets SET total_amount_minor=1 WHERE id=$1`,
		`DELETE FROM receipt_set_items WHERE receipt_set_id=$1`,
		`UPDATE receipt_set_confirm_commands SET actor_display_name='changed' WHERE receipt_set_id=$1`,
	} {
		_, err := environment.Pool.Exec(ctx, statement, result.Set.ID)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "55000" {
			t.Fatalf("immutable statement error=%v", err)
		}
	}
}

func TestPostgresReceiptSetRejectsFractionBeyondCNYMinorUnit(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	// The database-level projection must never round a Money decimal into a bonus amount.
	if got, err := decimalToMinor("1.001000"); got != 0 || !errors.Is(err, ErrStateConflict) {
		t.Fatalf("decimal projection=%d,%v", got, err)
	}
}
