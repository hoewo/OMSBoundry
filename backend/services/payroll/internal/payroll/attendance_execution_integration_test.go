//go:build integration

package payroll

import (
	"context"
	"testing"

	attendancecontract "omsbizfoundry/attendance-summary-contract"
)

type fixedAttendanceSummary struct{ value attendancecontract.MonthlySummary }

func (f fixedAttendanceSummary) GetMonthlySummary(_ context.Context, tenant, id string) (attendancecontract.MonthlySummary, error) {
	value := f.value
	value.TenantID, value.ID = tenant, id
	return value, nil
}

func TestAttendanceSummaryApplicationFreezesSourceAndSurvivesManualRevision(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	actor := executionActor()
	personID := "44444444-4444-4444-8444-444444444441"
	summaryID := "77777777-7777-4777-8777-777777777777"
	summary := attendancecontract.MonthlySummary{
		SchemaVersion: attendancecontract.SchemaVersion, ID: summaryID, TenantID: actor.TenantID, PersonID: personID, PersonDisplayName: "员工一",
		PersonSourceRevision: 2, PersonSourceDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Period: "2026-11",
		PolicyKey: "beijing-calendar", PolicyRevision: 1, PolicyDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Basis: "calendar_days",
		StandardUnits: 300000, EmploymentEligibleUnits: 150000, UnpaidLeaveDeductionUnits: 10000, PayableUnits: 140000, SourceRevision: 1,
	}
	service, err := NewService(store, profilePersonReferences{}, fixedAttendanceSummary{value: summary})
	if err != nil {
		t.Fatal(err)
	}
	seedCompensation(t, ctx, service, actor, personID, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaa01", 1000000, 0, 0)
	scheme := seedScheme(t, ctx, service, actor)
	run, err := service.CreateRegularRun(ctx, actor, CreateRegularRunInput{CommandID: "cccccccc-cccc-4ccc-8ccc-cccccccccc01", Period: "2026-11", Label: "十一月工资", PlannedPayDate: "2026-11-30", ContributionSchemeID: scheme.ID, ContributionSchemeRevision: scheme.Revision, PersonIDs: []string{personID}})
	if err != nil {
		t.Fatal(err)
	}
	line := lineForPerson(t, run, personID)
	run, err = service.ApplyAttendanceSummary(ctx, actor, run.ID, line.ID, ApplyAttendanceSummaryInput{CommandID: "dddddddd-dddd-4ddd-8ddd-dddddddddd01", ExpectedRunRevision: run.Revision, ExpectedLineRevision: line.Revision, SummaryID: summaryID, Reason: "应用十一月考勤"})
	if err != nil {
		t.Fatal(err)
	}
	line = lineForPerson(t, run, personID)
	if line.AttendanceSummary == nil || line.AttendanceSummary.SourceRevision != 1 || line.AttendanceProrationMinor != -533333 || line.GrossMinor != 466667 {
		t.Fatalf("attendance line=%+v", line)
	}
	run, err = service.RevisePayrollLine(ctx, actor, run.ID, line.ID, ReviseLineInput{CommandID: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeee01", ExpectedRunRevision: run.Revision, ExpectedLineRevision: line.Revision, PerformanceMinor: 10000, TaxSourceKind: "manual", Adjustments: []Adjustment{{ID: "ffffffff-ffff-4fff-8fff-fffffffffff1", Kind: "manual", AmountMinor: 5000, Reason: "餐补"}}, Reason: "补充绩效"})
	if err != nil {
		t.Fatal(err)
	}
	line = lineForPerson(t, run, personID)
	if line.AttendanceSummary == nil || line.AttendanceProrationMinor != -533333 || len(line.Adjustments) != 2 || line.GrossMinor != 481667 {
		t.Fatalf("revised attendance line=%+v", line)
	}
}
