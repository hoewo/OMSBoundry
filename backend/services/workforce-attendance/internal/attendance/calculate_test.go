package attendance

import (
	"testing"

	contract "omsbizfoundry/attendance-summary-contract"
)

func baseInput() CalculateInput {
	return CalculateInput{
		SummaryID: "11111111-1111-4111-8111-111111111111",
		TenantID:  "22222222-2222-4222-8222-222222222222", Period: "2026-11",
		Policy: Policy{Key: "calendar-v1", Revision: 1, Basis: "calendar_days"},
		Employment: EmploymentSnapshot{
			PersonID: "33333333-3333-4333-8333-333333333333", DisplayName: "测试成员", SourceRevision: 2,
			SourceDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Intervals:    []EmploymentInterval{{From: "2026-11-11", Through: "2026-11-30"}},
		},
		SourceRevision: 4,
	}
}

func TestRehireIntervalsExcludeGap(t *testing.T) {
	input := baseInput()
	input.Employment.Intervals = []EmploymentInterval{{From: "2026-11-01", Through: "2026-11-10"}, {From: "2026-11-21"}}
	summary, err := Calculate(input)
	if err != nil {
		t.Fatal(err)
	}
	if summary.EmploymentEligibleUnits != 200000 || summary.PayableUnits != 200000 {
		t.Fatalf("unexpected rehire summary: %#v", summary)
	}
}

func TestCalendarProrationCombinesMidMonthEmploymentLeaveAndAbsence(t *testing.T) {
	input := baseInput()
	input.Leaves = []LeaveRecord{{
		ID: "44444444-4444-4444-8444-444444444444", From: "2026-11-16", Through: "2026-11-17",
		UnitsPerDay: contract.FullDayUnits, PaidRatioBPS: 5000,
	}}
	input.Absences = []AbsenceRecord{{ID: "55555555-5555-4555-8555-555555555555", Date: "2026-11-20", Units: 5000}}
	summary, err := Calculate(input)
	if err != nil {
		t.Fatal(err)
	}
	if summary.StandardUnits != 300000 || summary.EmploymentEligibleUnits != 200000 ||
		summary.UnpaidLeaveDeductionUnits != 10000 || summary.AbsenceDeductionUnits != 5000 || summary.PayableUnits != 185000 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
}

func TestScheduledPolicyRequiresExplicitWorkdays(t *testing.T) {
	input := baseInput()
	input.Policy = Policy{Key: "working-v1", Revision: 1, Basis: "scheduled_workdays"}
	if _, err := Calculate(input); err == nil {
		t.Fatal("implicit weekday schedule accepted")
	}
	input.Policy.ScheduledDates = []string{"2026-11-10", "2026-11-11", "2026-11-12"}
	summary, err := Calculate(input)
	if err != nil {
		t.Fatal(err)
	}
	if summary.StandardUnits != 30000 || summary.EmploymentEligibleUnits != 20000 || summary.PayableUnits != 20000 {
		t.Fatalf("unexpected scheduled summary: %#v", summary)
	}
}

func TestPolicyKeyMustBeCanonical(t *testing.T) {
	input := baseInput()
	input.Policy.Key = " calendar-v1 "
	if _, err := Calculate(input); err == nil {
		t.Fatal("non-canonical policy key accepted")
	}
}

func TestDailyRecordsCannotDeductMoreThanOneUnit(t *testing.T) {
	input := baseInput()
	input.Leaves = []LeaveRecord{{
		ID: "44444444-4444-4444-8444-444444444444", From: "2026-11-16", Through: "2026-11-16",
		UnitsPerDay: 7500, PaidRatioBPS: 0,
	}}
	input.Absences = []AbsenceRecord{{ID: "55555555-5555-4555-8555-555555555555", Date: "2026-11-16", Units: 5000}}
	if _, err := Calculate(input); err == nil {
		t.Fatal("more than one day of deductions accepted for one date")
	}
}

func TestCalculationRejectsDuplicateLeaveAndMalformedSourceDigest(t *testing.T) {
	input := baseInput()
	leave := LeaveRecord{
		ID: "44444444-4444-4444-8444-444444444444", From: "2026-11-16", Through: "2026-11-16",
		UnitsPerDay: contract.FullDayUnits, PaidRatioBPS: 0,
	}
	input.Leaves = []LeaveRecord{leave, leave}
	if _, err := Calculate(input); err == nil {
		t.Fatal("duplicate leave record accepted")
	}

	input = baseInput()
	input.Employment.SourceDigest = "sha256:not-a-digest"
	if _, err := Calculate(input); err == nil {
		t.Fatal("malformed employment source digest accepted")
	}
}
