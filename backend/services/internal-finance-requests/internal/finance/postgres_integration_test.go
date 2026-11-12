//go:build integration

package finance

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"omsbizfoundry/internal-finance-requests/internal/database"
	"os"
	"strings"
	"testing"
	"time"
)

type integrationMoney struct{ calls int }

func (m *integrationMoney) Record(_ context.Context, in MoneyPaymentRequest) (MoneyPaymentResult, error) {
	m.calls++
	allocated := in.ActualAmountMinor
	if m.calls == 2 {
		allocated = 4000
	}
	return MoneyPaymentResult{Status: "recorded", MoneyPaymentID: fmt.Sprintf("70000000-0000-4000-8000-%012d", m.calls), MoneyPaymentRevision: 1, MoneyAllocationID: fmt.Sprintf("71000000-0000-4000-8000-%012d", m.calls), MoneyAllocationRevision: 1, MoneyObligationID: "72000000-0000-4000-8000-000000000001", MoneyObligationRevision: int64(m.calls + 1), AllocatedAmountMinor: allocated, OverageAmountMinor: in.ActualAmountMinor - allocated}, nil
}

type rejectedMoney struct{ calls int }

func (m *rejectedMoney) Record(context.Context, MoneyPaymentRequest) (MoneyPaymentResult, error) {
	m.calls++
	return MoneyPaymentResult{}, ErrPaymentRejected
}

type integrationEvidence struct{}

func (integrationEvidence) Resolve(_ context.Context, _ string, ref EvidenceRef) (EvidenceVersion, error) {
	return EvidenceVersion{FileID: ref.FileID, Version: ref.Version, Status: "available", SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", MediaType: "application/pdf", SizeBytes: 128, ValidationProfile: "invoice-v1", SecurityStatus: "passed"}, nil
}

type fingerprintEvidence struct{}

func (fingerprintEvidence) Resolve(_ context.Context, _ string, ref EvidenceRef) (EvidenceVersion, error) {
	digit := strings.ReplaceAll(ref.FileID, "-", "")[:1]
	return EvidenceVersion{FileID: ref.FileID, Version: ref.Version, Status: "available", SHA256: strings.Repeat(digit, 64), MediaType: "application/pdf", SizeBytes: 128, ValidationProfile: "invoice-v1", SecurityStatus: "passed"}, nil
}

func TestLifecycleIdempotencyRevisionAuditAndTenantIsolation(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	service, _ := NewService(store)
	actor := Actor{ID: "22222222-2222-4222-8222-222222222222", TenantID: "11111111-1111-4111-8111-111111111111", CredentialID: "33333333-3333-4333-8333-333333333333"}
	input := CreateInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", RequestType: "reimbursement", Title: "差旅报销", AmountMinor: 12345, Currency: "CNY"}
	created, err := service.Create(ctx, actor, input)
	if err != nil || created.Revision != 1 {
		t.Fatalf("create: %#v %v", created, err)
	}
	replayed, err := service.Create(ctx, actor, input)
	if err != nil || replayed.ID != created.ID {
		t.Fatalf("replay: %#v %v", replayed, err)
	}
	changed := input
	changed.Title = "冲突"
	if _, err = service.Create(ctx, actor, changed); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("expected command conflict, got %v", err)
	}
	updated, err := service.Update(ctx, actor, created.ID, UpdateInput{CommandID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", ExpectedRevision: 1, Title: "差旅报销更正", AmountMinor: 12000})
	if err != nil || updated.Revision != 2 {
		t.Fatalf("update: %#v %v", updated, err)
	}
	if _, err = service.Update(ctx, actor, created.ID, UpdateInput{CommandID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", ExpectedRevision: 1, Title: "过期", AmountMinor: 1}); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("expected revision conflict, got %v", err)
	}
	if _, err = service.Get(ctx, "99999999-9999-4999-8999-999999999999", created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross tenant read must be 404, got %v", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE finance_requests SET title='tampered' WHERE tenant_id=$1 AND id=$2`, actor.TenantID, created.ID); err == nil {
		t.Fatal("current projection mutation without revision must fail")
	}
	if _, err = pool.Exec(ctx, `UPDATE finance_request_revisions SET title='tampered' WHERE tenant_id=$1 AND request_id=$2 AND revision=1`, actor.TenantID, created.ID); err == nil {
		t.Fatal("revision mutation must fail")
	}
	var requests, revisions, commands, audits int
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM finance_requests),(SELECT count(*) FROM finance_request_revisions),(SELECT count(*) FROM finance_request_commands),(SELECT count(*) FROM finance_request_audit_events)`).Scan(&requests, &revisions, &commands, &audits); err != nil {
		t.Fatal(err)
	}
	if requests != 1 || revisions != 2 || commands != 2 || audits != 2 {
		t.Fatalf("unexpected counts %d/%d/%d/%d", requests, revisions, commands, audits)
	}
}

func TestStableListUsesDescendingIDForEqualTimestamps(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	service, _ := NewService(store)
	actor := Actor{ID: "22222222-2222-4222-8222-222222222222", TenantID: "11111111-1111-4111-8111-111111111111", CredentialID: "33333333-3333-4333-8333-333333333333"}
	first, err := service.Create(ctx, actor, CreateInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa1", RequestType: "reimbursement", Title: "分页一", AmountMinor: 100, Currency: "CNY"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Create(ctx, actor, CreateInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa2", RequestType: "reimbursement", Title: "分页二", AmountMinor: 100, Currency: "CNY"})
	if err != nil {
		t.Fatal(err)
	}
	fixed := time.Date(2026, 10, 26, 8, 0, 0, 0, time.UTC)
	if _, err = pool.Exec(ctx, `UPDATE finance_requests SET updated_at=$1 WHERE tenant_id=$2 AND id=ANY($3::uuid[])`, fixed, actor.TenantID, []string{first.ID, second.ID}); err != nil {
		t.Fatal(err)
	}
	higher, lower := first.ID, second.ID
	if higher < lower {
		higher, lower = lower, higher
	}
	page, err := service.List(ctx, actor.TenantID, ListFilter{ApplicantSubjectID: actor.ID, RequestType: "reimbursement", Limit: 1})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != higher || page.NextCursor == "" {
		t.Fatalf("first page=%#v err=%v", page, err)
	}
	page, err = service.List(ctx, actor.TenantID, ListFilter{ApplicantSubjectID: actor.ID, RequestType: "reimbursement", BeforeUpdatedAt: &fixed, BeforeID: higher, Limit: 1})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != lower {
		t.Fatalf("second page=%#v err=%v", page, err)
	}
}

func TestFormalSubmitPartialAndOverpaymentLifecycle(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	money := &integrationMoney{}
	service, _ := NewServiceWithDependencies(store, money, integrationEvidence{})
	actor := Actor{ID: "22222222-2222-4222-8222-222222222222", TenantID: "11111111-1111-4111-8111-111111111111", CredentialID: "33333333-3333-4333-8333-333333333333"}
	digest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	details := DetailSnapshot{Applicant: ReferenceSnapshot{ID: "44444444-4444-4444-8444-444444444444", Revision: 1, Digest: digest, DisplayName: "申请人"}, Payee: PayeeSnapshot{Kind: "person", Reference: ReferenceSnapshot{ID: "44444444-4444-4444-8444-444444444444", Revision: 1, Digest: digest, DisplayName: "申请人"}, DisplayName: "申请人", BankName: "测试银行", AccountNumber: "6222021234567890"}, Lines: []Line{{ID: "55555555-5555-4555-8555-555555555555", BusinessDate: "2026-10-25", CategoryKey: "travel", CategoryName: "差旅费", CategoryRev: 1, Purpose: "客户现场差旅", AmountMinor: 10000, Evidence: []EvidenceRef{{FileID: "66666666-6666-4666-8666-666666666666", Version: 1, Purpose: "发票"}}}}}
	created, err := service.Create(ctx, actor, CreateInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", RequestType: "reimbursement", Title: "差旅报销", AmountMinor: 1, Currency: "CNY", Details: details})
	if err != nil || created.AmountMinor != 10000 {
		t.Fatalf("create=%#v err=%v", created, err)
	}
	if created.Details.Payee.AccountNumber != "" || created.Details.Payee.AccountLast4 != "7890" {
		t.Fatalf("create response must be masked: %#v", created.Details.Payee)
	}
	referenced, err := service.IsEvidenceReferenced(ctx, actor.TenantID, "66666666-6666-4666-8666-666666666666", 1)
	if err != nil || !referenced {
		t.Fatalf("created draft evidence was not protected: referenced=%v err=%v", referenced, err)
	}
	referenced, err = service.IsEvidenceReferenced(ctx, actor.TenantID, "66666666-6666-4666-8666-666666666666", 2)
	if err != nil || referenced {
		t.Fatalf("unreferenced version was protected: referenced=%v err=%v", referenced, err)
	}
	sensitive, err := service.GetSensitive(ctx, actor, created.ID, "财务付款前核对")
	if err != nil || sensitive.Details.Payee.AccountNumber != "6222021234567890" {
		t.Fatalf("sensitive read: %v", err)
	}
	submitted, err := service.Submit(ctx, actor, created.ID, SubmitInput{CommandID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", ExpectedRevision: 1})
	if err != nil || submitted.Status != "payable" || submitted.ApprovalPolicy != "not_required" {
		t.Fatalf("submit=%#v err=%v", submitted, err)
	}
	if submitted.Details.Payee.AccountNumber != "" || submitted.Details.Payee.AccountLast4 != "7890" {
		t.Fatalf("submit response must be masked: %#v", submitted.Details.Payee)
	}
	submitReplay, err := service.Submit(ctx, actor, created.ID, SubmitInput{CommandID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", ExpectedRevision: 1})
	if err != nil || submitReplay.Status != "payable" || submitReplay.Revision != submitted.Revision {
		t.Fatalf("submit replay=%#v err=%v", submitReplay, err)
	}
	partial, err := service.ConfirmPayment(ctx, actor, created.ID, ConfirmPaymentInput{CommandID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", ExpectedRevision: 2, ActualAmountMinor: 6000, OccurredOn: "2026-10-25", Note: "首笔"})
	if err != nil || partial.Status != "partially_paid" || partial.AllocatedMinor != 6000 || partial.Revision != 3 {
		t.Fatalf("partial=%#v err=%v", partial, err)
	}
	paid, err := service.ConfirmPayment(ctx, actor, created.ID, ConfirmPaymentInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", ExpectedRevision: 3, ActualAmountMinor: 5000, OccurredOn: "2026-10-26", Note: "尾款含超额", OverageConfirmed: true, OverageReason: "银行实际多付"})
	if err != nil || paid.Status != "paid_with_overpayment" || paid.PaidMinor != 11000 || paid.AllocatedMinor != 10000 || paid.OverageMinor != 1000 || paid.Revision != 4 {
		t.Fatalf("paid=%#v err=%v", paid, err)
	}
	replay, err := service.ConfirmPayment(ctx, actor, created.ID, ConfirmPaymentInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", ExpectedRevision: 3, ActualAmountMinor: 5000, OccurredOn: "2026-10-26", Note: "尾款含超额", OverageConfirmed: true, OverageReason: "银行实际多付"})
	if err != nil || replay.Revision != 4 || money.calls != 2 {
		t.Fatalf("replay=%#v calls=%d err=%v", replay, money.calls, err)
	}
	if _, err = service.Withdraw(ctx, actor, created.ID, WithdrawInput{CommandID: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", ExpectedRevision: 4, Reason: "不允许"}); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("withdraw after payment=%v", err)
	}
	var obligations, deliveries, events, sensitiveAudits int
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM finance_request_revisions WHERE request_id=$1),(SELECT count(*) FROM finance_payment_deliveries WHERE request_id=$1),(SELECT count(*) FROM finance_payment_delivery_events),(SELECT count(*) FROM finance_sensitive_access_audit_events WHERE request_id=$1)`, created.ID).Scan(&obligations, &deliveries, &events, &sensitiveAudits); err != nil {
		t.Fatal(err)
	}
	if obligations != 4 || deliveries != 2 || events != 4 || sensitiveAudits != 1 {
		t.Fatalf("counts revisions/deliveries/events/sensitive=%d/%d/%d/%d", obligations, deliveries, events, sensitiveAudits)
	}
}

func TestExpenseCategoryRevisionsAndDeterministicPaymentRejection(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	money := &rejectedMoney{}
	service, _ := NewServiceWithDependencies(store, money, integrationEvidence{})
	actor := Actor{ID: "22222222-2222-4222-8222-222222222222", TenantID: "11111111-1111-4111-8111-111111111111", CredentialID: "33333333-3333-4333-8333-333333333333"}

	initial, err := service.ListCategories(ctx, actor.TenantID)
	if err != nil || len(initial.Items) != len(platformCategories) {
		t.Fatalf("platform categories=%d err=%v", len(initial.Items), err)
	}
	createdCategory, err := service.CreateCategory(ctx, actor, CategoryCreateInput{CommandID: "11111111-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Key: "client_gift", DisplayName: "客户礼品费", Reason: "租户财务需要"})
	if err != nil || createdCategory.Revision != 1 || createdCategory.Source != "tenant" {
		t.Fatalf("created category=%#v err=%v", createdCategory, err)
	}
	updatedCategory, err := service.UpdateCategory(ctx, actor, createdCategory.ID, CategoryUpdateInput{CommandID: "22222222-aaaa-4aaa-8aaa-aaaaaaaaaaaa", ExpectedRevision: 1, DisplayName: "客户礼品", Enabled: false, Reason: "停止新选择"})
	if err != nil || updatedCategory.Revision != 2 || updatedCategory.Enabled {
		t.Fatalf("updated category=%#v err=%v", updatedCategory, err)
	}
	platformDisabled, err := service.UpdateCategory(ctx, actor, platformCategories[0].ID, CategoryUpdateInput{CommandID: "33333333-aaaa-4aaa-8aaa-aaaaaaaaaaaa", ExpectedRevision: 1, DisplayName: platformCategories[0].DisplayName, Enabled: false, Reason: "本租户停用"})
	if err != nil || platformDisabled.Revision != 2 || platformDisabled.Source != "platform" {
		t.Fatalf("platform override=%#v err=%v", platformDisabled, err)
	}

	digest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	details := DetailSnapshot{Applicant: ReferenceSnapshot{ID: "44444444-4444-4444-8444-444444444444", Revision: 1, Digest: digest, DisplayName: "申请人"}, Payee: PayeeSnapshot{Kind: "person", Reference: ReferenceSnapshot{ID: "44444444-4444-4444-8444-444444444444", Revision: 1, Digest: digest, DisplayName: "申请人"}, DisplayName: "申请人"}, Lines: []Line{{ID: "55555555-5555-4555-8555-555555555555", BusinessDate: "2026-10-25", CategoryKey: "travel", CategoryName: "差旅费", CategoryRev: 1, Purpose: "客户现场差旅", AmountMinor: 10000, Evidence: []EvidenceRef{{FileID: "66666666-6666-4666-8666-666666666666", Version: 1, Purpose: "发票"}}}}}
	created, err := service.Create(ctx, actor, CreateInput{CommandID: "44444444-aaaa-4aaa-8aaa-aaaaaaaaaaaa", RequestType: "reimbursement", Title: "被拒绝付款", AmountMinor: 1, Currency: "CNY", Details: details})
	if err != nil {
		t.Fatal(err)
	}
	submitted, err := service.Submit(ctx, actor, created.ID, SubmitInput{CommandID: "55555555-aaaa-4aaa-8aaa-aaaaaaaaaaaa", ExpectedRevision: created.Revision})
	if err != nil {
		t.Fatal(err)
	}
	payment := ConfirmPaymentInput{CommandID: "66666666-aaaa-4aaa-8aaa-aaaaaaaaaaaa", ExpectedRevision: submitted.Revision, ActualAmountMinor: 10000, OccurredOn: "2026-10-25"}
	if _, err = service.ConfirmPayment(ctx, actor, created.ID, payment); !errors.Is(err, ErrPaymentRejected) {
		t.Fatalf("expected deterministic rejection, got %v", err)
	}
	if _, err = service.ConfirmPayment(ctx, actor, created.ID, payment); !errors.Is(err, ErrPaymentRejected) || money.calls != 1 {
		t.Fatalf("rejected replay must not call Money again: calls=%d err=%v", money.calls, err)
	}
	var status string
	if err = pool.QueryRow(ctx, `SELECT status FROM finance_payment_deliveries WHERE tenant_id=$1 AND command_id=$2`, actor.TenantID, payment.CommandID).Scan(&status); err != nil || status != "failed" {
		t.Fatalf("delivery status=%q err=%v", status, err)
	}
}

func TestInvoiceStrongAndSuspectedDuplicateRules(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	service, _ := NewServiceWithDependencies(store, nil, fingerprintEvidence{})
	actor := Actor{ID: "22222222-2222-4222-8222-222222222222", TenantID: "11111111-1111-4111-8111-111111111111", CredentialID: "33333333-3333-4333-8333-333333333333"}
	digest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	makeDetails := func(lineID, fileID, code, number string) DetailSnapshot {
		person := ReferenceSnapshot{ID: "44444444-4444-4444-8444-444444444444", Revision: 1, Digest: digest, DisplayName: "申请人"}
		return DetailSnapshot{Applicant: person, Payee: PayeeSnapshot{Kind: "person", Reference: person, DisplayName: "申请人", BankName: "测试银行", AccountNumber: "6222021234567890"}, Lines: []Line{{ID: lineID, BusinessDate: "2026-10-25", CategoryKey: "travel", CategoryName: "差旅费", CategoryRev: 1, Purpose: "差旅", AmountMinor: 10000, Evidence: []EvidenceRef{{FileID: fileID, Version: 1, Purpose: "发票"}}, Invoice: &InvoiceSnapshot{Code: code, Number: number, IssuerName: "测试开票方", InvoiceDate: "2026-10-24", AmountMinor: 10000}}}}
	}
	firstDetails := makeDetails("20000000-0000-4000-8000-000000000001", "60000000-0000-4000-8000-000000000001", "A001", "N001")
	firstDetails.Payee = PayeeSnapshot{Kind: "one_time", EntityKind: "organization", DisplayName: "测试开票方", PaymentMethod: "cash"}
	first, err := service.Create(ctx, actor, CreateInput{CommandID: "10000000-0000-4000-8000-000000000001", RequestType: "payment", Title: "首单付款", AmountMinor: 1, Currency: "CNY", Details: firstDetails})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Submit(ctx, actor, first.ID, SubmitInput{CommandID: "10000000-0000-4000-8000-000000000002", ExpectedRevision: first.Revision}); err != nil {
		t.Fatal(err)
	}
	strong, err := service.Create(ctx, actor, CreateInput{CommandID: "10000000-0000-4000-8000-000000000003", RequestType: "reimbursement", Title: "强重复", AmountMinor: 1, Currency: "CNY", Details: makeDetails("20000000-0000-4000-8000-000000000002", "70000000-0000-4000-8000-000000000001", "A001", "N001")})
	if err != nil {
		t.Fatal(err)
	}
	projected, err := service.Get(ctx, actor.TenantID, strong.ID)
	if err != nil || projected.Details.Duplicate.Status != "strong" || len(projected.Details.Duplicate.MatchedRequestIDs) != 1 || projected.Details.Duplicate.MatchedRequestIDs[0] != first.ID {
		t.Fatalf("draft duplicate projection=%#v err=%v", projected.Details.Duplicate, err)
	}
	if _, err = service.Submit(ctx, actor, strong.ID, SubmitInput{CommandID: "10000000-0000-4000-8000-000000000004", ExpectedRevision: strong.Revision}); !errors.Is(err, ErrStrongDuplicate) {
		t.Fatalf("expected strong duplicate, got %v", err)
	}
	var duplicate *DuplicateError
	if !errors.As(err, &duplicate) || len(duplicate.MatchedRequestIDs) != 1 || duplicate.MatchedRequestIDs[0] != first.ID {
		t.Fatalf("strong duplicate reference=%#v err=%v", duplicate, err)
	}
	suspected, err := service.Create(ctx, actor, CreateInput{CommandID: "10000000-0000-4000-8000-000000000005", RequestType: "reimbursement", Title: "疑似重复", AmountMinor: 1, Currency: "CNY", Details: makeDetails("20000000-0000-4000-8000-000000000003", "80000000-0000-4000-8000-000000000001", "A002", "N002")})
	if err != nil {
		t.Fatal(err)
	}
	submit := SubmitInput{CommandID: "10000000-0000-4000-8000-000000000006", ExpectedRevision: suspected.Revision}
	if _, err = service.Submit(ctx, actor, suspected.ID, submit); !errors.Is(err, ErrSuspectedDuplicate) {
		t.Fatalf("expected suspected duplicate, got %v", err)
	}
	submit.SuspectedDuplicateAcknowledged = true
	accepted, err := service.Submit(ctx, actor, suspected.ID, submit)
	if err != nil || accepted.Details.Duplicate.Status != "suspected" {
		t.Fatalf("accepted=%#v err=%v", accepted, err)
	}
}

func TestFullyDigitalInvoiceNumberIsStrongDuplicateWithoutLegacyCode(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	service, _ := NewServiceWithDependencies(store, nil, fingerprintEvidence{})
	actor := Actor{ID: "22222222-2222-4222-8222-222222222222", TenantID: "11111111-1111-4111-8111-111111111111", CredentialID: "33333333-3333-4333-8333-333333333333"}
	digest := "sha256:" + strings.Repeat("a", 64)
	makeDetails := func(lineID, fileID string) DetailSnapshot {
		person := ReferenceSnapshot{ID: "44444444-4444-4444-8444-444444444444", Revision: 1, Digest: digest, DisplayName: "申请人"}
		return DetailSnapshot{Applicant: person, Payee: PayeeSnapshot{Kind: "person", Reference: person, DisplayName: "申请人"}, Lines: []Line{{ID: lineID, BusinessDate: "2026-11-08", CategoryKey: "travel", CategoryName: "差旅费", CategoryRev: 1, Purpose: "全电发票报销", AmountMinor: 268000, Evidence: []EvidenceRef{{FileID: fileID, Version: 1, Purpose: "发票"}}, Invoice: &InvoiceSnapshot{Number: "24932000000038888512", IssuerName: "全电发票销售方", InvoiceDate: "2026-11-08", AmountMinor: 268000}}}}
	}
	first, err := service.Create(ctx, actor, CreateInput{CommandID: "11000000-0000-4000-8000-000000000001", RequestType: "reimbursement", Title: "首张全电发票", AmountMinor: 268000, Currency: "CNY", Details: makeDetails("21000000-0000-4000-8000-000000000001", "61000000-0000-4000-8000-000000000001")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Submit(ctx, actor, first.ID, SubmitInput{CommandID: "11000000-0000-4000-8000-000000000002", ExpectedRevision: first.Revision}); err != nil {
		t.Fatal(err)
	}
	duplicate, err := service.Create(ctx, actor, CreateInput{CommandID: "11000000-0000-4000-8000-000000000003", RequestType: "reimbursement", Title: "重复全电发票", AmountMinor: 268000, Currency: "CNY", Details: makeDetails("21000000-0000-4000-8000-000000000002", "71000000-0000-4000-8000-000000000001")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Submit(ctx, actor, duplicate.ID, SubmitInput{CommandID: "11000000-0000-4000-8000-000000000004", ExpectedRevision: duplicate.Revision}); !errors.Is(err, ErrStrongDuplicate) {
		t.Fatalf("expected strong duplicate for fully digital invoice number, got %v", err)
	}
}

func TestDraftRetentionArchiveRestoreAndSoftDelete(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	service, _ := NewService(store)
	actor := Actor{ID: "22222222-2222-4222-8222-222222222222", TenantID: "11111111-1111-4111-8111-111111111111", CredentialID: "33333333-3333-4333-8333-333333333333"}
	created, err := service.Create(ctx, actor, CreateInput{CommandID: "90000000-0000-4000-8000-000000000001", RequestType: "reimbursement", Title: "待归档草稿", AmountMinor: 100, Currency: "CNY"})
	if err != nil {
		t.Fatal(err)
	}
	staleAt := time.Now().UTC().Add(-91 * 24 * time.Hour)
	if _, err = pool.Exec(ctx, `UPDATE finance_requests SET updated_at=$3 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, created.ID, staleAt); err != nil {
		t.Fatal(err)
	}
	count, err := service.ArchiveStaleDrafts(ctx, time.Now().UTC().Add(-90*24*time.Hour), 10)
	if err != nil || count != 1 {
		t.Fatalf("archive count=%d err=%v", count, err)
	}
	page, err := service.List(ctx, actor.TenantID, ListFilter{ApplicantSubjectID: actor.ID})
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("default list=%#v err=%v", page, err)
	}
	page, err = service.List(ctx, actor.TenantID, ListFilter{ApplicantSubjectID: actor.ID, IncludeArchived: true})
	if err != nil || len(page.Items) != 1 || page.Items[0].ArchivedAt == nil {
		t.Fatalf("archived list=%#v err=%v", page, err)
	}
	restored, err := service.Restore(ctx, actor, created.ID, RestoreInput{CommandID: "90000000-0000-4000-8000-000000000002"})
	if err != nil || restored.Revision != 2 || restored.ArchivedAt != nil {
		t.Fatalf("restore=%#v err=%v", restored, err)
	}
	deleted, err := service.DeleteDraft(ctx, actor, created.ID, DeleteDraftInput{CommandID: "90000000-0000-4000-8000-000000000003", Confirmation: "DELETE", Reason: "重复发票草稿"})
	if err != nil || deleted.DeletedAt == nil {
		t.Fatalf("delete=%#v err=%v", deleted, err)
	}
	if _, err = service.Get(ctx, actor.TenantID, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted draft must be hidden, got %v", err)
	}
	page, err = service.List(ctx, actor.TenantID, ListFilter{ApplicantSubjectID: actor.ID, IncludeArchived: true, IncludeDeleted: true})
	if err != nil || len(page.Items) != 1 || page.Items[0].DeletedAt == nil {
		t.Fatalf("audit list=%#v err=%v", page, err)
	}
	var revisions, retentionEvents int
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM finance_request_revisions WHERE request_id=$1),(SELECT count(*) FROM finance_request_retention_events WHERE request_id=$1)`, created.ID).Scan(&revisions, &retentionEvents); err != nil {
		t.Fatal(err)
	}
	if revisions != 2 || retentionEvents != 3 {
		t.Fatalf("revisions=%d retention_events=%d", revisions, retentionEvents)
	}
	var deletionReason, deletionActor, deletionCredential string
	if err = pool.QueryRow(ctx, `SELECT details->>'reason',actor_subject_id::text,credential_id FROM finance_request_retention_events WHERE request_id=$1 AND action='deleted'`, created.ID).Scan(&deletionReason, &deletionActor, &deletionCredential); err != nil {
		t.Fatal(err)
	}
	if deletionReason != "重复发票草稿" || deletionActor != actor.ID || deletionCredential != actor.CredentialID {
		t.Fatalf("deletion log reason=%q actor=%q credential=%q", deletionReason, deletionActor, deletionCredential)
	}
}

func integrationPool(t *testing.T, ctx context.Context) (*pgxpool.Pool, func()) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	var random [8]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	schema := "finance_test_" + hex.EncodeToString(random[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool, func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
		admin.Close()
	}
}
