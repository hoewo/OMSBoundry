//go:build integration

package payrollpayment

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
	"omsbizfoundry/money-flow-ledger/internal/testdatabase"
)

func TestPostgresPayrollPaymentOwnsBindingAndRecordsPartialOverageExactlyOnce(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 25, 9, 30, 0, 0, time.UTC)
	if _, err := environment.Pool.Exec(ctx, `INSERT INTO parties(id,kind,legal_name,revision,created_at,updated_at) VALUES ($1,'organization','测试企业',1,$3,$3),($2,'individual','成员甲',1,$3,$3)`, testOwner, testPersonParty, now); err != nil {
		t.Fatal(err)
	}
	if _, err := environment.Pool.Exec(ctx, `INSERT INTO payroll_ledger_owner_setting_revisions(tenant_id,revision,command_id,local_party_id,local_party_revision,local_party_kind,local_party_legal_name,reason,actor_kind,actor_id,actor_display_name,recorded_at,request_sha256) VALUES($1,1,$2,$3,1,'organization','测试企业','测试工资账簿主体','iam_subject',$4,'测试经办人',$5,repeat('a',64))`, testTenant, "77000000-0000-4000-8000-000000000001", testOwner, testActor, now); err != nil {
		t.Fatal(err)
	}
	if _, err := environment.Pool.Exec(ctx, `INSERT INTO payable_party_source_mappings(tenant_id,command_id,source_application,source_resource_type,source_resource_id,source_revision,source_digest,local_party_id,local_party_revision,local_party_kind,local_party_legal_name,reviewed_by_actor_kind,reviewed_by_actor_id,reviewed_by_actor_display_name,review_reason,recorded_at,request_sha256) VALUES($1,$2,'people-groups','person',$3,2,$4,$5,1,'individual','成员甲','iam_subject',$6,'测试经办人','工资应付映射',$7,repeat('b',64))`, testTenant, "77000000-0000-4000-8000-000000000002", testPerson, "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", testPersonParty, testActor, now); err != nil {
		t.Fatal(err)
	}
	obligations := obligation.NewService(obligation.NewPostgresRepository(environment.Pool))
	cash := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	allocations := allocation.NewService(allocation.NewPostgresRepository(environment.Pool))
	service, err := NewService(NewPostgresRepository(environment.Pool), obligations, cash, allocations)
	if err != nil {
		t.Fatal(err)
	}
	actor := audit.ActorRef{Kind: audit.ActorKindIAMSubject, ID: testActor, DisplayName: "测试经办人"}
	first := validRequest("78000000-0000-4000-8000-000000000001", 40000)
	firstResult, err := service.Record(ctx, actor, first)
	if err != nil || firstResult.AllocatedAmountMinor != 40000 || firstResult.OverageAmountMinor != 0 {
		t.Fatalf("first=%#v err=%v", firstResult, err)
	}
	second := validRequest("78000000-0000-4000-8000-000000000002", 70000)
	second.PayrollLineRevision = 4
	secondResult, err := service.Record(ctx, actor, second)
	if err != nil || secondResult.AllocatedAmountMinor != 60000 || secondResult.OverageAmountMinor != 10000 || secondResult.MoneyObligationID != firstResult.MoneyObligationID {
		t.Fatalf("second=%#v err=%v", secondResult, err)
	}
	replay, err := service.Record(ctx, actor, second)
	if err != nil || !replay.Replayed || replay.MoneyPaymentID != secondResult.MoneyPaymentID {
		t.Fatalf("replay=%#v err=%v", replay, err)
	}
	var obligationsCount, cashCount, allocationsCount, bindingsCount, deliveriesCount int
	if err = environment.Pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM obligations WHERE source_kind='payroll_payment'),
		(SELECT count(*) FROM cash_transactions),
		(SELECT count(*) FROM allocations),
		(SELECT count(*) FROM payroll_payment_obligation_bindings),
		(SELECT count(*) FROM payroll_payment_deliveries)
	`).Scan(&obligationsCount, &cashCount, &allocationsCount, &bindingsCount, &deliveriesCount); err != nil {
		t.Fatal(err)
	}
	if obligationsCount != 1 || cashCount != 2 || allocationsCount != 2 || bindingsCount != 1 || deliveriesCount != 2 {
		t.Fatalf("obligations=%d cash=%d allocations=%d bindings=%d deliveries=%d", obligationsCount, cashCount, allocationsCount, bindingsCount, deliveriesCount)
	}
	details, found, err := obligations.GetDetails(ctx, firstResult.MoneyObligationID)
	if err != nil || !found || details.Obligation.OutstandingAmount != "0" || details.Obligation.SourceApplication != "payroll" || details.Obligation.SourceResourceType != "payroll_line" {
		t.Fatalf("obligation=%#v found=%v err=%v", details.Obligation, found, err)
	}
	transaction, found, err := cash.Get(ctx, secondResult.MoneyPaymentID)
	if err != nil || !found || transaction.UnallocatedAmount != "100" || transaction.AccountReference != "测试银行 ****1234" {
		t.Fatalf("cash=%#v found=%v err=%v", transaction, found, err)
	}
	for _, statement := range []string{
		`UPDATE payroll_ledger_owner_setting_revisions SET reason='changed'`,
		`DELETE FROM payroll_payment_obligation_bindings`,
		`UPDATE payroll_payment_deliveries SET actual_amount_minor=1`,
	} {
		_, statementErr := environment.Pool.Exec(ctx, statement)
		var databaseError *pgconn.PgError
		if !errors.As(statementErr, &databaseError) || databaseError.Code != "55000" {
			t.Fatalf("immutable statement error=%v", statementErr)
		}
	}
}
