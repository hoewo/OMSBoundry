package payroll

import (
	"testing"

	attendance "omsbizfoundry/attendance-summary-contract"
)

func attendanceSummary() attendance.MonthlySummary {
	return attendance.MonthlySummary{
		SchemaVersion: attendance.SchemaVersion, ID: "11111111-1111-4111-8111-111111111111",
		TenantID: "22222222-2222-4222-8222-222222222222", PersonID: "33333333-3333-4333-8333-333333333333",
		PersonDisplayName:    "测试成员",
		PersonSourceRevision: 2, PersonSourceDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Period: "2026-11", PolicyKey: "calendar-v1", PolicyRevision: 1,
		PolicyDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Basis:        "calendar_days", StandardUnits: 300000, EmploymentEligibleUnits: 200000,
		UnpaidLeaveDeductionUnits: 10000, AbsenceDeductionUnits: 5000, PayableUnits: 185000, SourceRevision: 4,
	}
}

func TestAttendanceProrationUsesExactHalfUpMinorUnits(t *testing.T) {
	result, err := ApplyAttendanceProration(1000000, attendanceSummary())
	if err != nil {
		t.Fatal(err)
	}
	if result.ProratedFixedSalaryMinor != 616667 || result.AdjustmentMinor != -383333 {
		t.Fatalf("unexpected exact proration: %#v", result)
	}
}

func TestAttendanceProrationRejectsUnversionedOrInconsistentSummary(t *testing.T) {
	summary := attendanceSummary()
	summary.PayableUnits++
	if _, err := ApplyAttendanceProration(1000000, summary); err == nil {
		t.Fatal("inconsistent attendance summary accepted")
	}
}
