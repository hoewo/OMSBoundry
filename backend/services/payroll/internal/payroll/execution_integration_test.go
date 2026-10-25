//go:build integration

package payroll

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

type scriptedPaymentPort struct {
	mu      sync.Mutex
	results []PaymentDeliveryResult
	errors  []error
	calls   int
}

type subjectPersonReferences struct {
	profilePersonReferences
	person PersonReference
}

func (r subjectPersonReferences) ResolvePersonForSubject(_ context.Context, tenant, _ string) (PersonReference, error) {
	value := r.person
	value.TenantID = tenant
	return value, nil
}

func (p *scriptedPaymentPort) Deliver(_ context.Context, intent PaymentIntent) (PaymentDeliveryResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	index := p.calls
	p.calls++
	var result PaymentDeliveryResult
	var err error
	if index < len(p.results) {
		result = p.results[index]
	}
	if result.Status == "succeeded" && result.AllocatedAmountMinor == 0 && result.OverageAmountMinor == 0 {
		result.AllocatedAmountMinor = intent.AmountMinor
	}
	if result.Status == "succeeded" && result.MoneyAllocationID == "" {
		result.MoneyAllocationID = fmt.Sprintf("90000000-0000-4000-8000-%012d", index+1)
		result.MoneyAllocationRevision = 1
	}
	if index < len(p.errors) {
		err = p.errors[index]
	}
	return result, err
}

func TestPayrollExecutionLifecyclePaymentsAndRecovery(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	port := &scriptedPaymentPort{results: []PaymentDeliveryResult{
		{Status: "succeeded", MoneyPaymentID: "10000000-0000-4000-8000-000000000001", MoneyPaymentRevision: 1},
		{Status: "succeeded", MoneyPaymentID: "10000000-0000-4000-8000-000000000002", MoneyPaymentRevision: 1, AllocatedAmountMinor: 520000, OverageAmountMinor: 10000},
	}}
	service, _ := NewService(store, profilePersonReferences{}, port)
	actor := executionActor()
	personA := "44444444-4444-4444-8444-444444444441"
	personB := "44444444-4444-4444-8444-444444444442"
	seedCompensation(t, ctx, service, actor, personA, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaa01", 1000000, 800000, 800000)
	seedCompensation(t, ctx, service, actor, personB, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaa02", 900000, 800000, 800000)
	seedPayment(t, ctx, service, actor, personA, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbb01")
	scheme := seedScheme(t, ctx, service, actor)

	create := CreateRegularRunInput{CommandID: "cccccccc-cccc-4ccc-8ccc-cccccccccc01", Period: "2026-10", Label: "2026年10月工资", PlannedPayDate: "2026-10-31", ContributionSchemeID: scheme.ID, ContributionSchemeRevision: scheme.Revision, PersonIDs: []string{personB, personA}}
	run, err := service.CreateRegularRun(ctx, actor, create)
	if err != nil || run.Revision != 1 || len(run.Lines) != 2 || run.ApprovalStatus != "not_required" {
		t.Fatalf("create regular run: %#v %v", run, err)
	}
	lineA := lineForPerson(t, run, personA)
	lineB := lineForPerson(t, run, personB)
	if lineA.NetPayableMinor != 860000 || lineA.EmployeeContributionMinor != 140000 || lineA.EmployerContributionMinor != 268000 || lineA.PaymentAccountMissing || !lineB.PaymentAccountMissing {
		t.Fatalf("unexpected frozen lines: A=%#v B=%#v", lineA, lineB)
	}
	create.CommandID = "cccccccc-cccc-4ccc-8ccc-cccccccccc02"
	create.PersonIDs = []string{personA}
	replayed, err := service.CreateRegularRun(ctx, actor, create)
	if err != nil || replayed.ID != run.ID || len(replayed.Lines) != 2 {
		t.Fatalf("same period must return existing run: %#v %v", replayed, err)
	}

	run, err = service.RevisePayrollLine(ctx, actor, run.ID, lineA.ID, ReviseLineInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd01", ExpectedRunRevision: run.Revision, ExpectedLineRevision: lineA.Revision, PerformanceMinor: 100000, TaxMinor: 50000, TaxSourceKind: "import", TaxSourceNote: "外部个税表第 1 行", Adjustments: []Adjustment{{ID: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeee01", Kind: "supplement", AmountMinor: 12345, Reason: "补贴"}, {ID: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeee02", Kind: "recovery", AmountMinor: -2345, Reason: "扣回"}}, Reason: "录入绩效与个税"})
	if err != nil {
		t.Fatal(err)
	}
	lineA = lineForPerson(t, run, personA)
	if lineA.GrossMinor != 1110000 || lineA.PreTaxPayableMinor != 970000 || lineA.NetPayableMinor != 920000 || run.Status != "review_pending" {
		t.Fatalf("revised calculation: %#v", lineA)
	}
	run, err = service.ReviewPayrollRun(ctx, actor, run.ID, RunCommandInput{CommandID: "ffffffff-ffff-4fff-8fff-fffffffffff1", ExpectedRevision: run.Revision, Reason: "工资核对完成"})
	if err != nil || run.Status != "ready_to_pay" {
		t.Fatalf("review: %#v %v", run, err)
	}
	lineA, lineB = lineForPerson(t, run, personA), lineForPerson(t, run, personB)
	if _, err = service.RecordPayment(ctx, actor, run.ID, lineB.ID, RecordPaymentInput{CommandID: "11111111-1111-4111-8111-111111111101", ExpectedRunRevision: run.Revision, ExpectedLineRevision: lineB.Revision, ActualAmountMinor: lineB.NetPayableMinor, OccurredOn: "2026-10-31"}); !errors.Is(err, ErrPaymentAccount) {
		t.Fatalf("missing account must block payment: %v", err)
	}

	partial := RecordPaymentInput{CommandID: "11111111-1111-4111-8111-111111111102", ExpectedRunRevision: run.Revision, ExpectedLineRevision: lineA.Revision, ActualAmountMinor: 400000, OccurredOn: "2026-10-31", Note: "第一笔"}
	run, err = service.RecordPayment(ctx, actor, run.ID, lineA.ID, partial)
	if err != nil || run.Status != "partially_paid" {
		t.Fatalf("partial payment: %#v %v", run, err)
	}
	lineA = lineForPerson(t, run, personA)
	if lineA.Status != "partially_paid" || lineA.ActualPaidMinor != 400000 || lineA.AllocatedPaidMinor != 400000 {
		t.Fatalf("partial line: %#v", lineA)
	}
	if _, err = service.RecordPayment(ctx, actor, run.ID, lineA.ID, partial); err != nil || port.calls != 1 {
		t.Fatalf("successful payment replay must not call Money twice: calls=%d err=%v", port.calls, err)
	}
	overage := RecordPaymentInput{CommandID: "11111111-1111-4111-8111-111111111103", ExpectedRunRevision: run.Revision, ExpectedLineRevision: lineA.Revision, ActualAmountMinor: 530000, OccurredOn: "2026-10-31"}
	if _, err = service.RecordPayment(ctx, actor, run.ID, lineA.ID, overage); !errors.Is(err, ErrOverageConfirmation) {
		t.Fatalf("overage without explicit confirmation: %v", err)
	}
	overage.OverageConfirmed, overage.OverageReason = true, "银行实际多转 100 元"
	run, err = service.RecordPayment(ctx, actor, run.ID, lineA.ID, overage)
	if err != nil {
		t.Fatal(err)
	}
	lineA = lineForPerson(t, run, personA)
	if lineA.Status != "paid" || lineA.ActualPaidMinor != 930000 || lineA.AllocatedPaidMinor != 920000 || lineA.OverageMinor != 10000 {
		t.Fatalf("overage projection: %#v", lineA)
	}
	if _, err = service.VoidPayrollRun(ctx, actor, run.ID, RunCommandInput{CommandID: "11111111-1111-4111-8111-111111111104", ExpectedRevision: run.Revision, Reason: "不应允许"}); !errors.Is(err, ErrRunLocked) {
		t.Fatalf("paid run must not be voided: %v", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE payroll_lines SET fixed_salary_minor=fixed_salary_minor+1 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, lineA.ID); err == nil {
		t.Fatal("payroll line current projection tamper must fail")
	}
}

func TestPayrollUnknownRetryZeroNegativeVoidReplacementAndConcurrentPeriod(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	port := &scriptedPaymentPort{
		results: []PaymentDeliveryResult{{}, {Status: "succeeded", MoneyPaymentID: "20000000-0000-4000-8000-000000000001", MoneyPaymentRevision: 1}},
		errors:  []error{ErrPaymentUnknown, nil},
	}
	service, _ := NewService(store, profilePersonReferences{}, port)
	actor := executionActor()
	people := []string{"44444444-4444-4444-8444-444444444451", "44444444-4444-4444-8444-444444444452", "44444444-4444-4444-8444-444444444453"}
	for index, person := range people {
		seedCompensation(t, ctx, service, actor, person, fmt.Sprintf("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaab%02d", index+1), 100000, 0, 0)
		seedPayment(t, ctx, service, actor, person, fmt.Sprintf("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbc%02d", index+1))
	}
	scheme := seedScheme(t, ctx, service, actor)

	unknownRun := createRun(t, ctx, service, actor, scheme, "2026-11", []string{people[0]}, "cccccccc-cccc-4ccc-8ccc-cccccccccd01")
	unknownRun, _ = service.ReviewPayrollRun(ctx, actor, unknownRun.ID, RunCommandInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd11", ExpectedRevision: unknownRun.Revision, Reason: "核对完成"})
	unknownLine := unknownRun.Lines[0]
	payment := RecordPaymentInput{CommandID: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeee11", ExpectedRunRevision: unknownRun.Revision, ExpectedLineRevision: unknownLine.Revision, ActualAmountMinor: unknownLine.NetPayableMinor, OccurredOn: "2026-11-30"}
	if _, err := service.RecordPayment(ctx, actor, unknownRun.ID, unknownLine.ID, payment); !errors.Is(err, ErrPaymentUnknown) {
		t.Fatalf("first delivery should be unknown: %v", err)
	}
	otherCommand := payment
	otherCommand.CommandID = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeee12"
	if _, err := service.RecordPayment(ctx, actor, unknownRun.ID, unknownLine.ID, otherCommand); !errors.Is(err, ErrPaymentUnknown) {
		t.Fatalf("unknown delivery must block a new command: %v", err)
	}
	unknownRun, err := service.RecordPayment(ctx, actor, unknownRun.ID, unknownLine.ID, payment)
	if err != nil || unknownRun.Lines[0].Status != "paid" || port.calls != 2 {
		t.Fatalf("same unknown command retry: %#v calls=%d err=%v", unknownRun, port.calls, err)
	}

	mixed := createRun(t, ctx, service, actor, scheme, "2026-12", people[1:], "cccccccc-cccc-4ccc-8ccc-cccccccccd02")
	zeroLine, negativeLine := lineForPerson(t, mixed, people[1]), lineForPerson(t, mixed, people[2])
	mixed, err = service.RevisePayrollLine(ctx, actor, mixed.ID, zeroLine.ID, ReviseLineInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd12", ExpectedRunRevision: mixed.Revision, ExpectedLineRevision: zeroLine.Revision, TaxMinor: zeroLine.PreTaxPayableMinor, TaxSourceKind: "manual", TaxSourceNote: "零实发", Reason: "零实发测试"})
	if err != nil {
		t.Fatal(err)
	}
	negativeLine = lineForPerson(t, mixed, people[2])
	mixed, err = service.RevisePayrollLine(ctx, actor, mixed.ID, negativeLine.ID, ReviseLineInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd13", ExpectedRunRevision: mixed.Revision, ExpectedLineRevision: negativeLine.Revision, TaxMinor: negativeLine.PreTaxPayableMinor + 100, TaxSourceKind: "manual", TaxSourceNote: "负实发", Reason: "负实发测试"})
	if err != nil {
		t.Fatal(err)
	}
	mixed, err = service.ReviewPayrollRun(ctx, actor, mixed.ID, RunCommandInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd14", ExpectedRevision: mixed.Revision, Reason: "核对零负实发"})
	if err != nil || mixed.Status != "recovery_pending" {
		t.Fatalf("review zero/negative: %#v %v", mixed, err)
	}
	zeroLine = lineForPerson(t, mixed, people[1])
	mixed, err = service.ConfirmZeroLine(ctx, actor, mixed.ID, zeroLine.ID, LineCommandInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd15", ExpectedRunRevision: mixed.Revision, ExpectedLineRevision: zeroLine.Revision, Reason: "确认无需发放"})
	if err != nil || mixed.Status != "recovery_pending" || lineForPerson(t, mixed, people[1]).Status != "no_payment_required" {
		t.Fatalf("zero confirmation: %#v %v", mixed, err)
	}

	voided := createRun(t, ctx, service, actor, scheme, "2027-01", []string{people[0]}, "cccccccc-cccc-4ccc-8ccc-cccccccccd03")
	voided, err = service.VoidPayrollRun(ctx, actor, voided.ID, RunCommandInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd16", ExpectedRevision: voided.Revision, Reason: "成员范围错误"})
	if err != nil || voided.Status != "voided" {
		t.Fatalf("void run: %#v %v", voided, err)
	}
	replacement := CreateRegularRunInput{CommandID: "cccccccc-cccc-4ccc-8ccc-cccccccccd04", Period: voided.Period, Label: "替代工资表", PlannedPayDate: "2027-01-31", ContributionSchemeID: scheme.ID, ContributionSchemeRevision: scheme.Revision, PersonIDs: []string{people[0]}, ReplacesRunID: voided.ID, ReplacesRunRevision: voided.Revision}
	replaced, err := service.CreateRegularRun(ctx, actor, replacement)
	if err != nil || replaced.ID == voided.ID || replaced.ReplacesRunID != voided.ID {
		t.Fatalf("replacement run: %#v %v", replaced, err)
	}

	inputs := []CreateRegularRunInput{
		{CommandID: "cccccccc-cccc-4ccc-8ccc-cccccccccd05", Period: "2027-02", Label: "并发工资表 A", PlannedPayDate: "2027-02-28", ContributionSchemeID: scheme.ID, ContributionSchemeRevision: scheme.Revision, PersonIDs: []string{people[0]}},
		{CommandID: "cccccccc-cccc-4ccc-8ccc-cccccccccd06", Period: "2027-02", Label: "并发工资表 B", PlannedPayDate: "2027-02-28", ContributionSchemeID: scheme.ID, ContributionSchemeRevision: scheme.Revision, PersonIDs: []string{people[0]}},
	}
	var wg sync.WaitGroup
	ids, errs := make([]string, 2), make([]error, 2)
	for i := range inputs {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			value, createErr := service.CreateRegularRun(ctx, actor, inputs[index])
			ids[index], errs[index] = value.ID, createErr
		}(i)
	}
	wg.Wait()
	if errs[0] != nil || errs[1] != nil || ids[0] == "" || ids[0] != ids[1] {
		t.Fatalf("concurrent monthly uniqueness: ids=%v errors=%v", ids, errs)
	}
}

func TestPayrollExplicitMemberCompensationAndPaymentProfileSynchronization(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	port := &scriptedPaymentPort{results: []PaymentDeliveryResult{{Status: "succeeded", MoneyPaymentID: "30000000-0000-4000-8000-000000000001", MoneyPaymentRevision: 1}}}
	service, _ := NewService(store, profilePersonReferences{}, port)
	actor := executionActor()
	personA := "44444444-4444-4444-8444-444444444461"
	personB := "44444444-4444-4444-8444-444444444462"
	seedCompensation(t, ctx, service, actor, personA, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaac01", 1000000, 800000, 800000)
	seedCompensation(t, ctx, service, actor, personB, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaac02", 700000, 500000, 500000)
	seedPayment(t, ctx, service, actor, personA, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbac01")
	scheme := seedScheme(t, ctx, service, actor)
	run := createRun(t, ctx, service, actor, scheme, "2027-03", []string{personA}, "cccccccc-cccc-4ccc-8ccc-ccccccccce01")
	lineA := run.Lines[0]

	if _, err := service.UpsertCompensation(ctx, actor, UpsertCompensationProfileInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaac03", ExpectedRevision: 1, PersonID: personA, EffectiveMonth: "2027-03", FixedSalaryMinor: 1200000, SocialInsuranceBaseMinor: 900000, HousingFundBaseMinor: 900000, Reason: "当月调薪"}); err != nil {
		t.Fatal(err)
	}
	compPreview, err := service.PreviewCompensationSync(ctx, actor, run.ID, lineA.ID)
	if err != nil || compPreview.Before.FixedSalaryMinor != 1000000 || compPreview.After.FixedSalaryMinor != 1200000 || compPreview.PreviewHash == "" {
		t.Fatalf("compensation preview: %#v %v", compPreview, err)
	}
	run, err = service.ConfirmCompensationSync(ctx, actor, run.ID, lineA.ID, ConfirmSnapshotSyncInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd21", ExpectedRunRevision: run.Revision, ExpectedLineRevision: lineA.Revision, PreviewHash: compPreview.PreviewHash, Reason: "确认同步调薪"})
	if err != nil {
		t.Fatal(err)
	}
	lineA = lineForPerson(t, run, personA)
	if lineA.FixedSalaryMinor != 1200000 || lineA.CompensationProfileRevision != 2 || lineA.Status != "review_pending" {
		t.Fatalf("compensation sync: %#v", lineA)
	}
	stalePreview, err := service.PreviewCompensationSync(ctx, actor, run.ID, lineA.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.UpsertCompensation(ctx, actor, UpsertCompensationProfileInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaac04", ExpectedRevision: 2, PersonID: personA, EffectiveMonth: "2027-03", FixedSalaryMinor: 1300000, SocialInsuranceBaseMinor: 900000, HousingFundBaseMinor: 900000, Reason: "再次调薪"}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ConfirmCompensationSync(ctx, actor, run.ID, lineA.ID, ConfirmSnapshotSyncInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd22", ExpectedRunRevision: run.Revision, ExpectedLineRevision: lineA.Revision, PreviewHash: stalePreview.PreviewHash, Reason: "过期预览"}); !errors.Is(err, ErrPreviewStale) {
		t.Fatalf("stale compensation preview must fail: %v", err)
	}

	run, err = service.SyncMembers(ctx, actor, run.ID, SyncMembersInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd23", ExpectedRevision: run.Revision, PersonIDs: []string{personA, personB}, Reason: "同步新成员"})
	if err != nil || len(run.Lines) != 2 {
		t.Fatalf("member sync: %#v %v", run, err)
	}
	lineB := lineForPerson(t, run, personB)
	if !lineB.PaymentAccountMissing {
		t.Fatalf("new member must preserve missing payment profile: %#v", lineB)
	}
	seedPayment(t, ctx, service, actor, personB, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbac02")
	paymentPreview, err := service.PreviewPaymentProfileSync(ctx, actor, run.ID, lineB.ID)
	if err != nil || !paymentPreview.Before.PaymentAccountMissing || paymentPreview.After.PaymentAccountMissing || paymentPreview.After.MaskedPaymentAccount == "" {
		t.Fatalf("payment profile preview: %#v %v", paymentPreview, err)
	}
	run, err = service.ConfirmPaymentProfileSync(ctx, actor, run.ID, lineB.ID, ConfirmSnapshotSyncInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd24", ExpectedRunRevision: run.Revision, ExpectedLineRevision: lineB.Revision, PreviewHash: paymentPreview.PreviewHash, Reason: "确认补齐工资卡"})
	if err != nil {
		t.Fatal(err)
	}
	lineB = lineForPerson(t, run, personB)
	if lineB.PaymentAccountMissing || lineB.PaymentProfileRevision != 1 || lineB.Status != "review_pending" {
		t.Fatalf("payment profile sync: %#v", lineB)
	}

	run, err = service.ReviewPayrollRun(ctx, actor, run.ID, RunCommandInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd25", ExpectedRevision: run.Revision, Reason: "同步后重新核对"})
	if err != nil {
		t.Fatal(err)
	}
	lineA = lineForPerson(t, run, personA)
	run, err = service.RecordPayment(ctx, actor, run.ID, lineA.ID, RecordPaymentInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd26", ExpectedRunRevision: run.Revision, ExpectedLineRevision: lineA.Revision, ActualAmountMinor: lineA.NetPayableMinor, OccurredOn: "2027-03-31"})
	if err != nil {
		t.Fatal(err)
	}
	lineB = lineForPerson(t, run, personB)
	if _, err = service.ConfirmCompensationSync(ctx, actor, run.ID, lineB.ID, ConfirmSnapshotSyncInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd27", ExpectedRunRevision: run.Revision, ExpectedLineRevision: lineB.Revision, PreviewHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Reason: "发放后禁止同步"}); !errors.Is(err, ErrRunLocked) {
		t.Fatalf("any successful payment must lock compensation sync: %v", err)
	}
	if _, err = service.SyncMembers(ctx, actor, run.ID, SyncMembersInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd28", ExpectedRevision: run.Revision, PersonIDs: []string{personA, personB}, Reason: "发放后禁止同步"}); !errors.Is(err, ErrRunLocked) {
		t.Fatalf("any successful payment must lock member sync: %v", err)
	}
}

func TestPayrollSupplementAndRecoveryAdjustmentRunsPreserveSourceFacts(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	port := &scriptedPaymentPort{results: []PaymentDeliveryResult{
		{Status: "succeeded", MoneyPaymentID: "40000000-0000-4000-8000-000000000001", MoneyPaymentRevision: 1},
		{Status: "succeeded", MoneyPaymentID: "40000000-0000-4000-8000-000000000002", MoneyPaymentRevision: 1},
	}}
	service, _ := NewService(store, profilePersonReferences{}, port)
	actor := executionActor()
	person := "44444444-4444-4444-8444-444444444471"
	seedCompensation(t, ctx, service, actor, person, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaad01", 1000000, 800000, 800000)
	seedPayment(t, ctx, service, actor, person, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbad01")
	scheme := seedScheme(t, ctx, service, actor)

	sourceRun := createRun(t, ctx, service, actor, scheme, "2027-04", []string{person}, "cccccccc-cccc-4ccc-8ccc-cccccccccf01")
	var err error
	sourceRun, err = service.ReviewPayrollRun(ctx, actor, sourceRun.ID, RunCommandInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd31", ExpectedRevision: sourceRun.Revision, Reason: "核对原工资"})
	if err != nil {
		t.Fatal(err)
	}
	sourceLine := sourceRun.Lines[0]
	sourceRun, err = service.RecordPayment(ctx, actor, sourceRun.ID, sourceLine.ID, RecordPaymentInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd32", ExpectedRunRevision: sourceRun.Revision, ExpectedLineRevision: sourceLine.Revision, ActualAmountMinor: sourceLine.NetPayableMinor, OccurredOn: "2027-04-30"})
	if err != nil {
		t.Fatal(err)
	}
	sourceLine = sourceRun.Lines[0]

	supplementInput := CreateAdjustmentRunInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd33", Period: "2027-05", Label: "四月工资补发", PlannedPayDate: "2027-05-10", Items: []AdjustmentRunItem{{SourcePayrollLineID: sourceLine.ID, SourcePayrollLineRevision: sourceLine.Revision, Direction: "supplement", AmountMinor: 12345, Reason: "四月绩效补发"}}}
	supplement, err := service.CreateAdjustmentRun(ctx, actor, supplementInput)
	if err != nil {
		t.Fatal(err)
	}
	if supplement.RunKind != "adjustment" || supplement.Status != "review_pending" || len(supplement.Lines) != 1 {
		t.Fatalf("unexpected supplement run: %#v", supplement)
	}
	supplementLine := supplement.Lines[0]
	if supplementLine.LineKind != "supplement" || supplementLine.SourcePayrollLineID != sourceLine.ID || supplementLine.SourcePayrollLineRevision != sourceLine.Revision || supplementLine.NetPayableMinor != 12345 || supplementLine.AdjustmentReason != "四月绩效补发" {
		t.Fatalf("unexpected supplement line: %#v", supplementLine)
	}
	replayed, err := service.CreateAdjustmentRun(ctx, actor, supplementInput)
	if err != nil || replayed.ID != supplement.ID || len(replayed.Lines) != 1 {
		t.Fatalf("adjustment replay: %#v %v", replayed, err)
	}
	staleInput := supplementInput
	staleInput.CommandID = "dddddddd-dddd-4ddd-8ddd-dddddddddd34"
	staleInput.Items[0].SourcePayrollLineRevision--
	if _, err = service.CreateAdjustmentRun(ctx, actor, staleInput); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale source revision must fail: %v", err)
	}

	supplement, err = service.ReviewPayrollRun(ctx, actor, supplement.ID, RunCommandInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd35", ExpectedRevision: supplement.Revision, Reason: "核对补发"})
	if err != nil || supplement.Status != "ready_to_pay" {
		t.Fatalf("review supplement: %#v %v", supplement, err)
	}
	supplementLine = supplement.Lines[0]
	supplement, err = service.RecordPayment(ctx, actor, supplement.ID, supplementLine.ID, RecordPaymentInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd36", ExpectedRunRevision: supplement.Revision, ExpectedLineRevision: supplementLine.Revision, ActualAmountMinor: supplementLine.NetPayableMinor, OccurredOn: "2027-05-10"})
	if err != nil || supplement.Status != "paid" || supplement.Lines[0].Status != "paid" {
		t.Fatalf("pay supplement: %#v %v", supplement, err)
	}

	recoveryInput := CreateAdjustmentRunInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd37", Period: "2027-05", Label: "四月工资追缴", PlannedPayDate: "2027-05-15", Items: []AdjustmentRunItem{{SourcePayrollLineID: sourceLine.ID, SourcePayrollLineRevision: sourceLine.Revision, Direction: "recovery", AmountMinor: 2222, Reason: "四月多发追缴"}}}
	recovery, err := service.CreateAdjustmentRun(ctx, actor, recoveryInput)
	if err != nil || recovery.Lines[0].LineKind != "recovery" || recovery.Lines[0].NetPayableMinor != -2222 {
		t.Fatalf("create recovery: %#v %v", recovery, err)
	}
	recovery, err = service.ReviewPayrollRun(ctx, actor, recovery.ID, RunCommandInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd38", ExpectedRevision: recovery.Revision, Reason: "核对追缴"})
	if err != nil || recovery.Status != "recovery_pending" || recovery.Lines[0].Status != "recovery_pending" {
		t.Fatalf("review recovery: %#v %v", recovery, err)
	}
	reloadedSource, err := service.GetExecutionRun(ctx, actor.TenantID, sourceRun.ID)
	if err != nil || reloadedSource.Lines[0].Revision != sourceLine.Revision || reloadedSource.Lines[0].NetPayableMinor != sourceLine.NetPayableMinor {
		t.Fatalf("adjustments must not rewrite source facts: %#v %v", reloadedSource, err)
	}
}

func TestPayrollImportPreviewCommitStaleExpiryAndReplay(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	service, _ := NewService(store, profilePersonReferences{})
	actor := executionActor()
	personA := "44444444-4444-4444-8444-444444444491"
	personB := "44444444-4444-4444-8444-444444444492"
	seedCompensation(t, ctx, service, actor, personA, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaf01", 1000000, 0, 0)
	seedCompensation(t, ctx, service, actor, personB, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaf02", 900000, 0, 0)
	scheme := seedScheme(t, ctx, service, actor)
	run := createRun(t, ctx, service, actor, scheme, "2027-07", []string{personA, personB}, "cccccccc-cccc-4ccc-8ccc-ccccccccd101")
	lineA, lineB := lineForPerson(t, run, personA), lineForPerson(t, run, personB)
	invalidPreview, err := service.PreviewPayrollImport(ctx, actor, run.ID, PreviewPayrollImportInput{ExpectedRunRevision: run.Revision, Rows: []PayrollImportRowInput{
		{MemberReferenceCode: lineA.PersonReferenceCode, ExpectedLineRevision: lineA.Revision, PerformanceMinor: 1, Adjustments: []Adjustment{}},
		{MemberReferenceCode: "EMP-NOT-FOUND", ExpectedLineRevision: 1, PerformanceMinor: 1, Adjustments: []Adjustment{}},
		{PersonID: personA, ExpectedLineRevision: lineA.Revision, PerformanceMinor: 2, Adjustments: []Adjustment{}},
	}})
	if err != nil || invalidPreview.Confirmable || len(invalidPreview.Items) != 3 || invalidPreview.Items[0].MatchStatus != "matched" || invalidPreview.Items[1].MatchStatus != "unmatched" || invalidPreview.Items[1].ErrorCode != "reference_code_not_found" || invalidPreview.Items[2].ErrorCode != "duplicate_person" {
		t.Fatalf("row-level import diagnostics: %#v %v", invalidPreview, err)
	}
	if _, err = service.CommitPayrollImport(ctx, actor, run.ID, CommitPayrollImportInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd50", PreviewID: invalidPreview.ID, PreviewHash: invalidPreview.PreviewHash, ExpectedRunRevision: run.Revision, Reason: "不应部分提交"}); !errors.Is(err, ErrPreviewStale) {
		t.Fatalf("non-confirmable preview must not commit: %v", err)
	}
	rows := []PayrollImportRowInput{
		{PersonID: personA, ExpectedLineRevision: lineA.Revision, PerformanceMinor: 10000, TaxMinor: 20000, TaxSourceNote: "个税文件第 1 行", Adjustments: []Adjustment{{ID: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeef01", Kind: "supplement", AmountMinor: 5000, Reason: "餐补"}}},
		{PersonID: personB, ExpectedLineRevision: lineB.Revision, PerformanceMinor: 20000, TaxMinor: 10000, TaxSourceNote: "个税文件第 2 行", Adjustments: []Adjustment{}},
	}
	preview, err := service.PreviewPayrollImport(ctx, actor, run.ID, PreviewPayrollImportInput{ExpectedRunRevision: run.Revision, Rows: rows})
	if err != nil || !preview.Confirmable || preview.PreviewHash == "" || len(preview.Items) != 2 || !preview.ExpiresAt.After(time.Now().UTC()) {
		t.Fatalf("import preview: %#v %v", preview, err)
	}
	if got := preview.Items[0].After.NetPayableMinor; got != 995000 {
		t.Fatalf("preview calculation=%d want 995000", got)
	}
	if preview.Items[0].Before.Revision != preview.Items[0].After.Revision || preview.Items[0].After.TaxSourceKind != "import" {
		t.Fatalf("preview must not create a revision and must identify imported tax: %#v", preview.Items[0])
	}

	run, err = service.RevisePayrollLine(ctx, actor, run.ID, lineA.ID, ReviseLineInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd51", ExpectedRunRevision: run.Revision, ExpectedLineRevision: lineA.Revision, PerformanceMinor: 3000, TaxSourceKind: "manual", TaxSourceNote: "预览后人工修改", Adjustments: []Adjustment{}, Reason: "制造过期预览"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.CommitPayrollImport(ctx, actor, run.ID, CommitPayrollImportInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd52", PreviewID: preview.ID, PreviewHash: preview.PreviewHash, ExpectedRunRevision: preview.PayrollRunRevision, Reason: "不应提交过期预览"}); !errors.Is(err, ErrPreviewStale) {
		t.Fatalf("stale import preview must fail: %v", err)
	}

	lineA, lineB = lineForPerson(t, run, personA), lineForPerson(t, run, personB)
	rows[0].ExpectedLineRevision = lineA.Revision
	rows[0].PerformanceMinor = 12000
	rows[1].ExpectedLineRevision = lineB.Revision
	preview, err = service.PreviewPayrollImport(ctx, actor, run.ID, PreviewPayrollImportInput{ExpectedRunRevision: run.Revision, Rows: rows})
	if err != nil {
		t.Fatal(err)
	}
	beforeRunRevision := run.Revision
	beforeLineARevision, beforeLineBRevision := lineA.Revision, lineB.Revision
	commit := CommitPayrollImportInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd53", PreviewID: preview.ID, PreviewHash: preview.PreviewHash, ExpectedRunRevision: run.Revision, Reason: "确认绩效和个税导入"}
	run, err = service.CommitPayrollImport(ctx, actor, run.ID, commit)
	if err != nil {
		t.Fatal(err)
	}
	lineA, lineB = lineForPerson(t, run, personA), lineForPerson(t, run, personB)
	if run.Revision != beforeRunRevision+1 || lineA.Revision != beforeLineARevision+1 || lineB.Revision != beforeLineBRevision+1 || lineA.PerformanceMinor != 12000 || lineB.PerformanceMinor != 20000 || lineA.TaxSourceKind != "import" || lineA.Status != "review_pending" {
		t.Fatalf("committed import must create one coherent revision: %#v", run)
	}
	var runRevisionCount, lineRevisionCount, auditCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM payroll_run_revisions WHERE tenant_id=$1 AND payroll_run_id=$2`, actor.TenantID, run.ID).Scan(&runRevisionCount); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM payroll_line_revisions WHERE tenant_id=$1 AND payroll_run_id=$2`, actor.TenantID, run.ID).Scan(&lineRevisionCount); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM payroll_audit_events WHERE tenant_id=$1 AND payroll_run_id=$2 AND action='payroll_import_committed'`, actor.TenantID, run.ID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if runRevisionCount != int(run.Revision) || lineRevisionCount != int(lineA.Revision+lineB.Revision) || auditCount != 1 {
		t.Fatalf("unexpected import history counts run=%d line=%d audit=%d", runRevisionCount, lineRevisionCount, auditCount)
	}
	replayed, err := service.CommitPayrollImport(ctx, actor, run.ID, commit)
	if err != nil || replayed.Revision != run.Revision {
		t.Fatalf("import confirmation replay: %#v %v", replayed, err)
	}
	var replayAuditCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM payroll_audit_events WHERE tenant_id=$1 AND payroll_run_id=$2 AND action='payroll_import_committed'`, actor.TenantID, run.ID).Scan(&replayAuditCount); err != nil || replayAuditCount != 1 {
		t.Fatalf("replay duplicated import audit count=%d err=%v", replayAuditCount, err)
	}

	lineA = lineForPerson(t, run, personA)
	expiring, err := service.PreviewPayrollImport(ctx, actor, run.ID, PreviewPayrollImportInput{ExpectedRunRevision: run.Revision, Rows: []PayrollImportRowInput{{PersonID: personA, ExpectedLineRevision: lineA.Revision, PerformanceMinor: lineA.PerformanceMinor + 1, TaxMinor: lineA.TaxMinor, TaxSourceNote: lineA.TaxSourceNote, Adjustments: lineA.Adjustments}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE payroll_import_previews SET created_at=now()-interval '2 hours',expires_at=now()-interval '1 hour' WHERE tenant_id=$1 AND id=$2`, actor.TenantID, expiring.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = service.CommitPayrollImport(ctx, actor, run.ID, CommitPayrollImportInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd54", PreviewID: expiring.ID, PreviewHash: expiring.PreviewHash, ExpectedRunRevision: run.Revision, Reason: "不应提交过期预览"}); !errors.Is(err, ErrPreviewStale) {
		t.Fatalf("expired import preview must fail: %v", err)
	}
}

func TestSelfPayslipsResolveTrustedSubjectAndExposeOnlySettledOwnLines(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	port := &scriptedPaymentPort{results: []PaymentDeliveryResult{
		{Status: "succeeded", MoneyPaymentID: "50000000-0000-4000-8000-000000000001", MoneyPaymentRevision: 1},
		{Status: "succeeded", MoneyPaymentID: "50000000-0000-4000-8000-000000000002", MoneyPaymentRevision: 1},
	}}
	actor := executionActor()
	personA := "44444444-4444-4444-8444-444444444481"
	personB := "44444444-4444-4444-8444-444444444482"
	references := subjectPersonReferences{person: PersonReference{ID: personA, ReferenceCode: "EMP-A", DisplayName: "离职成员甲", Status: "inactive", Revision: 9, Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
	service, _ := NewService(store, references, port)
	seedCompensation(t, ctx, service, actor, personA, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaae01", 1000000, 0, 0)
	seedCompensation(t, ctx, service, actor, personB, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaae02", 2000000, 0, 0)
	seedPayment(t, ctx, service, actor, personA, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbae01")
	seedPayment(t, ctx, service, actor, personB, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbae02")
	scheme := seedScheme(t, ctx, service, actor)
	run := createRun(t, ctx, service, actor, scheme, "2027-06", []string{personA, personB}, "cccccccc-cccc-4ccc-8ccc-ccccccccd001")
	run, err := service.ReviewPayrollRun(ctx, actor, run.ID, RunCommandInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd41", ExpectedRevision: run.Revision, Reason: "核对工资"})
	if err != nil {
		t.Fatal(err)
	}
	for index, person := range []string{personA, personB} {
		line := lineForPerson(t, run, person)
		run, err = service.RecordPayment(ctx, actor, run.ID, line.ID, RecordPaymentInput{CommandID: fmt.Sprintf("dddddddd-dddd-4ddd-8ddd-dddddddddd4%d", index+2), ExpectedRunRevision: run.Revision, ExpectedLineRevision: line.Revision, ActualAmountMinor: line.NetPayableMinor, OccurredOn: "2027-06-30"})
		if err != nil {
			t.Fatal(err)
		}
	}
	page, err := service.ListSelfPayslips(ctx, actor)
	if err != nil || page.PersonID != personA || page.PersonStatus != "inactive" || len(page.Items) != 1 {
		t.Fatalf("self payslips: %#v %v", page, err)
	}
	item := page.Items[0]
	if item.Status != "paid" || item.PaidOn != "2027-06-30" || item.NetPayableMinor != 1000000 || item.ActualPaidMinor != 1000000 {
		t.Fatalf("unexpected own payslip: %#v", item)
	}
	if item.NetPayableMinor == 2000000 {
		t.Fatal("another member's salary leaked into self payslips")
	}
}

func executionActor() Actor {
	return Actor{ID: "22222222-2222-4222-8222-222222222222", TenantID: "11111111-1111-4111-8111-111111111111", CredentialID: "33333333-3333-4333-8333-333333333333"}
}
func seedCompensation(t *testing.T, ctx context.Context, service *Service, actor Actor, person, command string, fixed, social, housing int64) {
	t.Helper()
	if _, err := service.UpsertCompensation(ctx, actor, UpsertCompensationProfileInput{CommandID: command, PersonID: person, EffectiveMonth: "2026-10", FixedSalaryMinor: fixed, SocialInsuranceBaseMinor: social, HousingFundBaseMinor: housing, Reason: "测试建档"}); err != nil {
		t.Fatal(err)
	}
}
func seedPayment(t *testing.T, ctx context.Context, service *Service, actor Actor, person, command string) {
	t.Helper()
	if _, err := service.UpsertPayment(ctx, actor, UpsertPaymentProfileInput{CommandID: command, PersonID: person, AccountName: "测试成员", BankName: "测试银行", AccountNumber: "6222021234567890", Status: "active", EffectiveMonth: "2026-10", Reason: "测试建档", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
}
func seedScheme(t *testing.T, ctx context.Context, service *Service, actor Actor) ContributionScheme {
	t.Helper()
	components := []ContributionComponent{{Key: "pension", BaseKind: "social", IndividualRateBPS: 800, EmployerRateBPS: 1600, BaseFloorMinor: 0, BaseCapMinor: 3000000, RoundingMode: "half_up"}, {Key: "medical", BaseKind: "social", IndividualRateBPS: 200, EmployerRateBPS: 900, BaseFloorMinor: 0, BaseCapMinor: 3000000, RoundingMode: "half_up"}, {Key: "unemployment", BaseKind: "social", IndividualRateBPS: 50, EmployerRateBPS: 50, BaseFloorMinor: 0, BaseCapMinor: 3000000, RoundingMode: "half_up"}, {Key: "work_injury", BaseKind: "social", IndividualRateBPS: 0, EmployerRateBPS: 20, BaseFloorMinor: 0, BaseCapMinor: 3000000, RoundingMode: "half_up"}, {Key: "maternity", BaseKind: "social", IndividualRateBPS: 0, EmployerRateBPS: 80, BaseFloorMinor: 0, BaseCapMinor: 3000000, RoundingMode: "half_up"}, {Key: "housing_fund", BaseKind: "housing", IndividualRateBPS: 700, EmployerRateBPS: 700, BaseFloorMinor: 0, BaseCapMinor: 3000000, RoundingMode: "half_up"}}
	value, err := service.CreateStandardScheme(ctx, actor, CreateStandardSchemeInput{CommandID: "99999999-9999-4999-8999-999999999991", Title: "P4 测试方案", RegionCode: "CN-TEST", RegionName: "测试地区", EffectiveMonth: "2026-10", AuthoritySource: "https://authority.example.test/payroll", AuthorityVerifiedAt: time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC), Components: components, Reason: "P4 测试"})
	if err != nil {
		t.Fatal(err)
	}
	return value
}
func createRun(t *testing.T, ctx context.Context, service *Service, actor Actor, scheme ContributionScheme, period string, people []string, command string) ExecutionRun {
	t.Helper()
	value, err := service.CreateRegularRun(ctx, actor, CreateRegularRunInput{CommandID: command, Period: period, Label: period + " 工资", PlannedPayDate: period + "-28", ContributionSchemeID: scheme.ID, ContributionSchemeRevision: scheme.Revision, PersonIDs: people})
	if err != nil {
		t.Fatal(err)
	}
	return value
}
func lineForPerson(t *testing.T, run ExecutionRun, person string) PayrollLine {
	t.Helper()
	for _, line := range run.Lines {
		if line.PersonID == person {
			return line
		}
	}
	t.Fatalf("person %s missing from run", person)
	return PayrollLine{}
}
