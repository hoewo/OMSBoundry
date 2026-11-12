package payroll

import (
	"context"
	"sort"
	"time"

	attendancecontract "omsbizfoundry/attendance-summary-contract"
)

func withAttendanceAdjustment(manual []Adjustment, fixedSalaryMinor int64, summary *attendancecontract.MonthlySummary) ([]Adjustment, AttendanceProration, error) {
	result := make([]Adjustment, 0, len(manual)+1)
	for _, adjustment := range manual {
		if adjustment.Kind != "attendance_proration" {
			result = append(result, adjustment)
		}
	}
	if summary == nil {
		return result, AttendanceProration{}, nil
	}
	proration, err := ApplyAttendanceProration(fixedSalaryMinor, *summary)
	if err != nil {
		return nil, AttendanceProration{}, err
	}
	if proration.AdjustmentMinor != 0 {
		result = append(result, Adjustment{ID: summary.ID, Kind: "attendance_proration", AmountMinor: proration.AdjustmentMinor, Reason: proration.Reason})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, proration, nil
}

func (s *PostgresStore) ApplyAttendanceSummary(ctx context.Context, actor Actor, runID, lineID string, input ApplyAttendanceSummaryInput, summary attendancecontract.MonthlySummary, requestHash string) (ExecutionRun, error) {
	tx, err := s.begin(ctx, actor.TenantID, input.CommandID)
	if err != nil {
		return ExecutionRun{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, replayErr := replayValue[ExecutionRun](ctx, tx, actor, input.CommandID, requestHash); replayErr != nil || found {
		return value, replayErr
	}
	run, err := loadExecutionRun(ctx, tx, actor.TenantID, runID, "FOR UPDATE")
	if err != nil {
		return ExecutionRun{}, err
	}
	if run.Revision != input.ExpectedRunRevision {
		return ExecutionRun{}, ErrRevisionConflict
	}
	if run.RunKind != "regular" || run.Status == "voided" || hasBlockingDeliveries(ctx, tx, actor.TenantID, runID, "succeeded", "unknown") {
		return ExecutionRun{}, ErrRunLocked
	}
	line, index := findLine(run.Lines, lineID)
	if index < 0 {
		return ExecutionRun{}, ErrNotFound
	}
	if line.Revision != input.ExpectedLineRevision {
		return ExecutionRun{}, ErrRevisionConflict
	}
	if summary.TenantID != actor.TenantID || summary.PersonID != line.PersonID || summary.Period != run.Period {
		return ExecutionRun{}, ErrRevisionConflict
	}
	if line.AttendanceSummary != nil && line.AttendanceSummary.ID == summary.ID && line.AttendanceSummary.SourceRevision == summary.SourceRevision {
		if err = recordValue(ctx, tx, actor, input.CommandID, requestHash, run, time.Now().UTC()); err == nil {
			err = tx.Commit(ctx)
		}
		return run, err
	}
	adjustments, proration, err := withAttendanceAdjustment(line.Adjustments, line.FixedSalaryMinor, &summary)
	if err != nil {
		return ExecutionRun{}, err
	}
	gross, contributions, employee, employer, preTax, net, cost, err := CalculatePayrollLine(line.FixedSalaryMinor, line.SocialInsuranceBaseMinor, line.HousingFundBaseMinor, line.PerformanceMinor, line.TaxMinor, adjustments, run.ContributionSchemeSnapshot.Components)
	if err != nil {
		return ExecutionRun{}, err
	}
	now := time.Now().UTC()
	line.AttendanceSummary = &summary
	line.AttendanceProrationMinor = proration.AdjustmentMinor
	line.Adjustments = adjustments
	line.GrossMinor, line.Contributions = gross, contributions
	line.EmployeeContributionMinor, line.EmployerContributionMinor = employee, employer
	line.PreTaxPayableMinor, line.NetPayableMinor, line.EmployerCostMinor = preTax, net, cost
	line.Status = "review_pending"
	line.Revision++
	line.UpdatedAt = now
	run.Revision++
	run.Status = "review_pending"
	run.UpdatedAt = now
	run.Lines[index] = line
	if err = updatePayrollLine(ctx, tx, line); err != nil {
		return ExecutionRun{}, err
	}
	if err = updateExecutionRun(ctx, tx, run); err != nil {
		return ExecutionRun{}, err
	}
	if err = insertPayrollLineRevision(ctx, tx, line, run.Revision, actor, input.CommandID, "attendance_applied", now); err != nil {
		return ExecutionRun{}, err
	}
	if err = insertExecutionRunRevision(ctx, tx, run, actor, input.CommandID, "attendance_applied"); err != nil {
		return ExecutionRun{}, err
	}
	if err = insertAudit(ctx, tx, run.Run, actor, input.CommandID, input.ExpectedRunRevision, "payroll_attendance_applied"); err != nil {
		return ExecutionRun{}, err
	}
	if err = recordValue(ctx, tx, actor, input.CommandID, requestHash, run, now); err != nil {
		return ExecutionRun{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return ExecutionRun{}, err
	}
	return run, nil
}

var _ interface {
	ApplyAttendanceSummary(context.Context, Actor, string, string, ApplyAttendanceSummaryInput, attendancecontract.MonthlySummary, string) (ExecutionRun, error)
} = (*PostgresStore)(nil)
