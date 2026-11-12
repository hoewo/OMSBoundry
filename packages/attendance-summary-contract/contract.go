package attendancesummarycontract

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	SchemaVersion = "attendance.monthly-summary/v1"
	FullDayUnits  = int64(10000)
)

var (
	uuidPattern   = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	periodPattern = regexp.MustCompile(`^[0-9]{4}-(0[1-9]|1[0-2])$`)
	digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type MonthlySummary struct {
	SchemaVersion             string `json:"schema_version"`
	ID                        string `json:"id"`
	TenantID                  string `json:"tenant_id"`
	PersonID                  string `json:"person_id"`
	PersonDisplayName         string `json:"person_display_name"`
	PersonSourceRevision      int64  `json:"person_source_revision"`
	PersonSourceDigest        string `json:"person_source_digest"`
	Period                    string `json:"period"`
	PolicyKey                 string `json:"policy_key"`
	PolicyRevision            int64  `json:"policy_revision"`
	PolicyDigest              string `json:"policy_digest"`
	Basis                     string `json:"basis"`
	StandardUnits             int64  `json:"standard_units"`
	EmploymentEligibleUnits   int64  `json:"employment_eligible_units"`
	UnpaidLeaveDeductionUnits int64  `json:"unpaid_leave_deduction_units"`
	AbsenceDeductionUnits     int64  `json:"absence_deduction_units"`
	PayableUnits              int64  `json:"payable_units"`
	SourceRevision            int64  `json:"source_revision"`
}

func Validate(summary MonthlySummary) error {
	if summary.SchemaVersion != SchemaVersion || !uuidPattern.MatchString(summary.ID) ||
		!uuidPattern.MatchString(summary.TenantID) || !uuidPattern.MatchString(summary.PersonID) || strings.TrimSpace(summary.PersonDisplayName) == "" ||
		summary.PersonSourceRevision < 1 || !digestPattern.MatchString(summary.PersonSourceDigest) ||
		!periodPattern.MatchString(summary.Period) || summary.PolicyKey == "" || summary.PolicyRevision < 1 ||
		!digestPattern.MatchString(summary.PolicyDigest) || (summary.Basis != "calendar_days" && summary.Basis != "scheduled_workdays") ||
		summary.SourceRevision < 1 {
		return fmt.Errorf("attendance summary identity or source is invalid")
	}
	if summary.StandardUnits < FullDayUnits || summary.EmploymentEligibleUnits < 0 ||
		summary.EmploymentEligibleUnits > summary.StandardUnits || summary.UnpaidLeaveDeductionUnits < 0 ||
		summary.AbsenceDeductionUnits < 0 || summary.PayableUnits < 0 {
		return fmt.Errorf("attendance summary units are invalid")
	}
	deductions := summary.UnpaidLeaveDeductionUnits + summary.AbsenceDeductionUnits
	if deductions > summary.EmploymentEligibleUnits || summary.PayableUnits != summary.EmploymentEligibleUnits-deductions {
		return fmt.Errorf("attendance summary unit equation is invalid")
	}
	return nil
}
