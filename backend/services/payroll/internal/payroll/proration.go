package payroll

import (
	"fmt"
	"math/big"

	attendance "omsbizfoundry/attendance-summary-contract"
)

type AttendanceProration struct {
	SummaryID                string `json:"summary_id"`
	SummarySourceRevision    int64  `json:"summary_source_revision"`
	FixedSalaryMinor         int64  `json:"fixed_salary_minor"`
	ProratedFixedSalaryMinor int64  `json:"prorated_fixed_salary_minor"`
	AdjustmentMinor          int64  `json:"adjustment_minor"`
	Reason                   string `json:"reason"`
}

func ApplyAttendanceProration(fixedSalaryMinor int64, summary attendance.MonthlySummary) (AttendanceProration, error) {
	if fixedSalaryMinor < 0 {
		return AttendanceProration{}, fmt.Errorf("fixed salary must not be negative")
	}
	if err := attendance.Validate(summary); err != nil {
		return AttendanceProration{}, err
	}
	prorated, err := multiplyDivideHalfUp(fixedSalaryMinor, summary.PayableUnits, summary.StandardUnits)
	if err != nil {
		return AttendanceProration{}, err
	}
	return AttendanceProration{
		SummaryID: summary.ID, SummarySourceRevision: summary.SourceRevision,
		FixedSalaryMinor: fixedSalaryMinor, ProratedFixedSalaryMinor: prorated,
		AdjustmentMinor: prorated - fixedSalaryMinor,
		Reason:          "考勤折算 " + summary.Period + " · " + summary.PolicyKey,
	}, nil
}

func multiplyDivideHalfUp(value, numerator, denominator int64) (int64, error) {
	if denominator <= 0 || value < 0 || numerator < 0 {
		return 0, fmt.Errorf("proration ratio is invalid")
	}
	product := new(big.Int).Mul(big.NewInt(value), big.NewInt(numerator))
	product.Add(product, new(big.Int).Div(big.NewInt(denominator), big.NewInt(2)))
	result := product.Div(product, big.NewInt(denominator))
	if !result.IsInt64() {
		return 0, fmt.Errorf("prorated salary exceeds integer range")
	}
	return result.Int64(), nil
}
