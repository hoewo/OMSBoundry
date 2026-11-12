package attendancesummarycontract

import "testing"

func TestSummaryEquationIsExplicit(t *testing.T) {
	value := MonthlySummary{
		SchemaVersion: SchemaVersion, ID: "11111111-1111-4111-8111-111111111111",
		TenantID: "22222222-2222-4222-8222-222222222222", PersonID: "33333333-3333-4333-8333-333333333333",
		PersonDisplayName: "测试成员",
		PersonSourceRevision: 2, PersonSourceDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Period: "2026-11", PolicyKey: "calendar-v1", PolicyRevision: 1,
		PolicyDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Basis:        "calendar_days", StandardUnits: 300000, EmploymentEligibleUnits: 200000,
		UnpaidLeaveDeductionUnits: 10000, AbsenceDeductionUnits: 5000, PayableUnits: 185000, SourceRevision: 3,
	}
	if err := Validate(value); err != nil {
		t.Fatal(err)
	}
	value.PayableUnits++
	if err := Validate(value); err == nil {
		t.Fatal("invalid payable unit equation accepted")
	}
}
