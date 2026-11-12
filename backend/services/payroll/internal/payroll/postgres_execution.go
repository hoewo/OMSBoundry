package payroll

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const executionRunSelect = `SELECT id::text,tenant_id::text,period,label,status,revision,created_by_subject_id::text,created_at,updated_at,run_kind,COALESCE(replaces_run_id::text,''),planned_pay_date::text,approval_policy_snapshot,approval_status,COALESCE(contribution_scheme_id::text,''),COALESCE(contribution_scheme_revision,0),contribution_scheme_snapshot,void_reason FROM payroll_runs`

const executionLineSelect = `SELECT id::text,tenant_id::text,payroll_run_id::text,person_id::text,person_reference_code,person_display_name,person_source_revision,person_source_digest,person_status_snapshot,COALESCE(compensation_profile_id::text,''),COALESCE(compensation_profile_revision,0),COALESCE(payment_profile_id::text,''),COALESCE(payment_profile_revision,0),payment_account_name,payment_bank_name,payment_account_number,payment_account_missing,fixed_salary_minor,attendance_summary_snapshot,attendance_proration_minor,social_insurance_base_minor,housing_fund_base_minor,performance_minor,adjustments,gross_minor,employee_contributions,employer_contributions,employee_contribution_minor,employer_contribution_minor,tax_minor,tax_source_kind,tax_source_note,pre_tax_payable_minor,net_payable_minor,employer_cost_minor,actual_paid_minor,allocated_paid_minor,overage_minor,status,revision,line_kind,COALESCE(source_payroll_line_id::text,''),COALESCE(source_payroll_line_revision,0),adjustment_reason,created_at,updated_at FROM payroll_lines`

func (s *PostgresStore) CreateRegularRun(ctx context.Context, actor Actor, in CreateRegularRunInput, people []PersonReference, requestHash string) (ExecutionRun, error) {
	tx, err := s.begin(ctx, actor.TenantID, in.CommandID)
	if err != nil {
		return ExecutionRun{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, e := replayValue[ExecutionRun](ctx, tx, actor, in.CommandID, requestHash); e != nil || found {
		return value, e
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, actor.TenantID+":regular:"+in.Period); err != nil {
		return ExecutionRun{}, err
	}
	if existing, e := loadExecutionRunByPeriod(ctx, tx, actor.TenantID, in.Period); e == nil {
		if e = recordValue(ctx, tx, actor, in.CommandID, requestHash, existing, time.Now().UTC()); e != nil {
			return ExecutionRun{}, e
		}
		if e = tx.Commit(ctx); e != nil {
			return ExecutionRun{}, e
		}
		return existing, nil
	} else if !errors.Is(e, ErrNotFound) {
		return ExecutionRun{}, e
	}
	if in.ReplacesRunID != "" {
		var period, status string
		var revision int64
		e := tx.QueryRow(ctx, `SELECT period,status,revision FROM payroll_runs WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, actor.TenantID, in.ReplacesRunID).Scan(&period, &status, &revision)
		if errors.Is(e, pgx.ErrNoRows) {
			return ExecutionRun{}, ErrNotFound
		}
		if e != nil {
			return ExecutionRun{}, e
		}
		if period != in.Period || status != "voided" || revision != in.ReplacesRunRevision {
			return ExecutionRun{}, ErrRevisionConflict
		}
	}
	scheme, err := loadSchemeRevision(ctx, tx, in.ContributionSchemeID, in.ContributionSchemeRevision)
	if err != nil {
		return ExecutionRun{}, err
	}
	if scheme.TenantID != "" && scheme.TenantID != actor.TenantID {
		return ExecutionRun{}, ErrProfileNotFound
	}
	now := time.Now().UTC()
	run := ExecutionRun{Run: Run{ID: newUUID(), TenantID: actor.TenantID, Period: in.Period, Label: in.Label, Status: "review_pending", Revision: 1, CreatedBySubjectID: actor.ID, CreatedAt: now, UpdatedAt: now}, RunKind: "regular", ReplacesRunID: in.ReplacesRunID, PlannedPayDate: in.PlannedPayDate, ApprovalPolicySnapshot: map[string]any{"mode": "not_required", "version": 1}, ApprovalStatus: "not_required", ContributionSchemeID: scheme.ID, ContributionSchemeRevision: scheme.Revision, ContributionSchemeSnapshot: scheme}
	policyRaw, _ := json.Marshal(run.ApprovalPolicySnapshot)
	schemeRaw, _ := json.Marshal(scheme)
	_, err = tx.Exec(ctx, `INSERT INTO payroll_runs(id,tenant_id,period,label,status,revision,created_by_subject_id,created_at,updated_at,run_kind,replaces_run_id,planned_pay_date,approval_policy_snapshot,approval_status,contribution_scheme_id,contribution_scheme_revision,contribution_scheme_snapshot,void_reason) VALUES($1,$2,$3,$4,$5,1,$6,$7,$7,'regular',NULLIF($8,'')::uuid,$9::date,$10,'not_required',$11,$12,$13,'')`, run.ID, run.TenantID, run.Period, run.Label, run.Status, actor.ID, now, run.ReplacesRunID, run.PlannedPayDate, policyRaw, scheme.ID, scheme.Revision, schemeRaw)
	if err != nil {
		return ExecutionRun{}, err
	}
	if err = insertExecutionRunRevision(ctx, tx, run, actor, in.CommandID, "generated"); err != nil {
		return ExecutionRun{}, err
	}
	for _, person := range people {
		line, e := s.buildGeneratedLine(ctx, tx, run, person, scheme, now)
		if e != nil {
			return ExecutionRun{}, e
		}
		if e = insertPayrollLine(ctx, tx, line); e != nil {
			return ExecutionRun{}, e
		}
		if e = insertPayrollLineRevision(ctx, tx, line, run.Revision, actor, in.CommandID, "generated", now); e != nil {
			return ExecutionRun{}, e
		}
		run.Lines = append(run.Lines, line)
	}
	if err = insertAudit(ctx, tx, run.Run, actor, in.CommandID, 0, "payroll_run_generated"); err != nil {
		return ExecutionRun{}, err
	}
	if err = recordValue(ctx, tx, actor, in.CommandID, requestHash, run, now); err != nil {
		return ExecutionRun{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return ExecutionRun{}, err
	}
	return run, nil
}

func (s *PostgresStore) CreateAdjustmentRun(ctx context.Context, actor Actor, in CreateAdjustmentRunInput, requestHash string) (ExecutionRun, error) {
	tx, err := s.begin(ctx, actor.TenantID, in.CommandID)
	if err != nil {
		return ExecutionRun{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, replayErr := replayValue[ExecutionRun](ctx, tx, actor, in.CommandID, requestHash); replayErr != nil || found {
		return value, replayErr
	}
	now := time.Now().UTC()
	run := ExecutionRun{Run: Run{ID: newUUID(), TenantID: actor.TenantID, Period: in.Period, Label: in.Label, Status: "review_pending", Revision: 1, CreatedBySubjectID: actor.ID, CreatedAt: now, UpdatedAt: now}, RunKind: "adjustment", PlannedPayDate: in.PlannedPayDate, ApprovalPolicySnapshot: map[string]any{"mode": "not_required", "version": 1}, ApprovalStatus: "not_required"}
	policyRaw, _ := json.Marshal(run.ApprovalPolicySnapshot)
	schemeRaw, _ := json.Marshal(run.ContributionSchemeSnapshot)
	_, err = tx.Exec(ctx, `INSERT INTO payroll_runs(id,tenant_id,period,label,status,revision,created_by_subject_id,created_at,updated_at,run_kind,planned_pay_date,approval_policy_snapshot,approval_status,contribution_scheme_snapshot,void_reason) VALUES($1,$2,$3,$4,'review_pending',1,$5,$6,$6,'adjustment',$7::date,$8,'not_required',$9,'')`, run.ID, run.TenantID, run.Period, run.Label, actor.ID, now, run.PlannedPayDate, policyRaw, schemeRaw)
	if err != nil {
		return ExecutionRun{}, err
	}
	if err = insertExecutionRunRevision(ctx, tx, run, actor, in.CommandID, "adjustment_generated"); err != nil {
		return ExecutionRun{}, err
	}
	seenPeople := map[string]bool{}
	for _, item := range in.Items {
		var sourceRunID string
		if err = tx.QueryRow(ctx, `SELECT payroll_run_id::text FROM payroll_lines WHERE tenant_id=$1 AND id=$2`, actor.TenantID, item.SourcePayrollLineID).Scan(&sourceRunID); errors.Is(err, pgx.ErrNoRows) {
			return ExecutionRun{}, ErrNotFound
		} else if err != nil {
			return ExecutionRun{}, err
		}
		sourceRun, loadErr := loadExecutionRun(ctx, tx, actor.TenantID, sourceRunID, "")
		if loadErr != nil {
			return ExecutionRun{}, loadErr
		}
		source, index := findLine(sourceRun.Lines, item.SourcePayrollLineID)
		if index < 0 {
			return ExecutionRun{}, ErrNotFound
		}
		if sourceRun.RunKind != "regular" || sourceRun.Status == "voided" || source.Revision != item.SourcePayrollLineRevision || (source.Status != "paid" && source.Status != "no_payment_required" && source.Status != "recovery_pending") {
			return ExecutionRun{}, ErrRevisionConflict
		}
		if seenPeople[source.PersonID] {
			return ExecutionRun{}, fmt.Errorf("invalid duplicate adjustment person")
		}
		seenPeople[source.PersonID] = true
		signed := item.AmountMinor
		if item.Direction == "recovery" {
			signed = -signed
		}
		line := PayrollLine{ID: newUUID(), TenantID: actor.TenantID, PayrollRunID: run.ID, PersonID: source.PersonID, PersonReferenceCode: source.PersonReferenceCode, PersonDisplayName: source.PersonDisplayName, PersonSourceRevision: source.PersonSourceRevision, PersonSourceDigest: source.PersonSourceDigest, PersonStatusSnapshot: source.PersonStatusSnapshot, PaymentProfileID: source.PaymentProfileID, PaymentProfileRevision: source.PaymentProfileRevision, PaymentAccountName: source.PaymentAccountName, PaymentBankName: source.PaymentBankName, MaskedPaymentAccount: source.MaskedPaymentAccount, PaymentAccountMissing: source.PaymentAccountMissing, Adjustments: []Adjustment{{ID: newUUID(), Kind: item.Direction, AmountMinor: signed, Reason: item.Reason}}, GrossMinor: signed, Contributions: []ContributionAmount{}, TaxSourceKind: "manual", PreTaxPayableMinor: signed, NetPayableMinor: signed, EmployerCostMinor: signed, Status: "review_pending", Revision: 1, LineKind: item.Direction, SourcePayrollLineID: source.ID, SourcePayrollLineRevision: source.Revision, AdjustmentReason: item.Reason, CreatedAt: now, UpdatedAt: now, paymentAccountNumber: source.paymentAccountNumber}
		if item.Direction == "supplement" && line.PaymentAccountMissing {
			return ExecutionRun{}, ErrPaymentAccount
		}
		if err = insertPayrollLine(ctx, tx, line); err != nil {
			return ExecutionRun{}, err
		}
		if err = insertPayrollLineRevision(ctx, tx, line, run.Revision, actor, in.CommandID, "adjustment_generated", now); err != nil {
			return ExecutionRun{}, err
		}
		run.Lines = append(run.Lines, line)
	}
	if err = insertAudit(ctx, tx, run.Run, actor, in.CommandID, 0, "payroll_adjustment_run_generated"); err != nil {
		return ExecutionRun{}, err
	}
	if err = recordValue(ctx, tx, actor, in.CommandID, requestHash, run, now); err != nil {
		return ExecutionRun{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return ExecutionRun{}, err
	}
	return run, nil
}

func (s *PostgresStore) buildGeneratedLine(ctx context.Context, tx pgx.Tx, run ExecutionRun, person PersonReference, scheme ContributionScheme, now time.Time) (PayrollLine, error) {
	comp, err := loadCompensationForPeriod(ctx, tx, run.TenantID, person.ID, run.Period)
	if err != nil {
		return PayrollLine{}, err
	}
	payment, paymentErr := loadPaymentForPeriod(ctx, tx, run.TenantID, person.ID, run.Period)
	if paymentErr != nil && !errors.Is(paymentErr, ErrProfileNotFound) {
		return PayrollLine{}, paymentErr
	}
	gross, contributions, employee, employer, preTax, net, cost, err := CalculatePayrollLine(comp.FixedSalaryMinor, comp.SocialInsuranceBaseMinor, comp.HousingFundBaseMinor, 0, 0, nil, scheme.Components)
	if err != nil {
		return PayrollLine{}, err
	}
	line := PayrollLine{ID: newUUID(), TenantID: run.TenantID, PayrollRunID: run.ID, PersonID: person.ID, PersonReferenceCode: person.ReferenceCode, PersonDisplayName: person.DisplayName, PersonSourceRevision: person.Revision, PersonSourceDigest: person.Digest, PersonStatusSnapshot: person.Status, CompensationProfileID: comp.ID, CompensationProfileRevision: comp.Revision, FixedSalaryMinor: comp.FixedSalaryMinor, SocialInsuranceBaseMinor: comp.SocialInsuranceBaseMinor, HousingFundBaseMinor: comp.HousingFundBaseMinor, Adjustments: []Adjustment{}, GrossMinor: gross, Contributions: contributions, EmployeeContributionMinor: employee, EmployerContributionMinor: employer, TaxSourceKind: "manual", PreTaxPayableMinor: preTax, NetPayableMinor: net, EmployerCostMinor: cost, Status: "review_pending", Revision: 1, LineKind: "regular", CreatedAt: now, UpdatedAt: now}
	if errors.Is(paymentErr, ErrProfileNotFound) {
		line.PaymentAccountMissing = true
	} else {
		line.PaymentProfileID = payment.ID
		line.PaymentProfileRevision = payment.Revision
		line.PaymentAccountName = payment.AccountName
		line.PaymentBankName = payment.BankName
		line.paymentAccountNumber = payment.accountNumber
		line.MaskedPaymentAccount = payment.MaskedAccountNumber
	}
	return line, nil
}

func loadCompensationForPeriod(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, tenant, person, period string) (CompensationProfile, error) {
	var v CompensationProfile
	e := q.QueryRow(ctx, `SELECT profile_id::text,tenant_id::text,person_id::text,person_display_name,person_source_revision,effective_month,fixed_salary_minor,social_insurance_base_minor,housing_fund_base_minor,currency,reason,revision,occurred_at,occurred_at FROM compensation_profile_revisions WHERE tenant_id=$1 AND person_id=$2 AND effective_month<=$3 ORDER BY effective_month DESC,revision DESC LIMIT 1`, tenant, person, period).Scan(&v.ID, &v.TenantID, &v.PersonID, &v.PersonDisplayName, &v.PersonSourceRevision, &v.EffectiveMonth, &v.FixedSalaryMinor, &v.SocialInsuranceBaseMinor, &v.HousingFundBaseMinor, &v.Currency, &v.Reason, &v.Revision, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(e, pgx.ErrNoRows) {
		return v, ErrProfileNotFound
	}
	return v, e
}
func loadPaymentForPeriod(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, tenant, person, period string) (PaymentProfile, error) {
	var v PaymentProfile
	e := q.QueryRow(ctx, `SELECT profile_id::text,tenant_id::text,person_id::text,person_display_name,person_source_revision,account_name,bank_name,account_number,status,effective_month,reason,revision,occurred_at,occurred_at FROM payment_profile_revisions WHERE tenant_id=$1 AND person_id=$2 AND effective_month<=$3 AND status='active' ORDER BY effective_month DESC,revision DESC LIMIT 1`, tenant, person, period).Scan(&v.ID, &v.TenantID, &v.PersonID, &v.PersonDisplayName, &v.PersonSourceRevision, &v.AccountName, &v.BankName, &v.accountNumber, &v.Status, &v.EffectiveMonth, &v.Reason, &v.Revision, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(e, pgx.ErrNoRows) {
		return v, ErrProfileNotFound
	}
	v.MaskedAccountNumber = maskAccount(v.accountNumber)
	return v, e
}

func insertPayrollLine(ctx context.Context, tx pgx.Tx, v PayrollLine) error {
	adjustments, _ := json.Marshal(v.Adjustments)
	attendance, _ := json.Marshal(v.AttendanceSummary)
	if v.AttendanceSummary == nil {
		attendance = nil
	}
	employee, employer := contributionJSON(v.Contributions)
	_, e := tx.Exec(ctx, `INSERT INTO payroll_lines(id,tenant_id,payroll_run_id,person_id,person_display_name,person_source_revision,person_status_snapshot,compensation_profile_id,compensation_profile_revision,payment_profile_id,payment_profile_revision,payment_account_name,payment_bank_name,payment_account_number,payment_account_missing,fixed_salary_minor,attendance_summary_snapshot,attendance_proration_minor,social_insurance_base_minor,housing_fund_base_minor,performance_minor,adjustments,gross_minor,employee_contributions,employer_contributions,employee_contribution_minor,employer_contribution_minor,tax_minor,tax_source_kind,tax_source_note,pre_tax_payable_minor,net_payable_minor,employer_cost_minor,actual_paid_minor,allocated_paid_minor,overage_minor,status,revision,line_kind,source_payroll_line_id,source_payroll_line_revision,adjustment_reason,created_at,updated_at,person_reference_code,person_source_digest) VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,'')::uuid,NULLIF($9,0),NULLIF($10,'')::uuid,NULLIF($11,0),$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33,$34,$35,$36,$37,$38,$39,NULLIF($40,'')::uuid,NULLIF($41,0),$42,$43,$43,$44,$45)`, v.ID, v.TenantID, v.PayrollRunID, v.PersonID, v.PersonDisplayName, v.PersonSourceRevision, v.PersonStatusSnapshot, v.CompensationProfileID, v.CompensationProfileRevision, v.PaymentProfileID, v.PaymentProfileRevision, v.PaymentAccountName, v.PaymentBankName, v.paymentAccountNumber, v.PaymentAccountMissing, v.FixedSalaryMinor, attendance, v.AttendanceProrationMinor, v.SocialInsuranceBaseMinor, v.HousingFundBaseMinor, v.PerformanceMinor, adjustments, v.GrossMinor, employee, employer, v.EmployeeContributionMinor, v.EmployerContributionMinor, v.TaxMinor, v.TaxSourceKind, v.TaxSourceNote, v.PreTaxPayableMinor, v.NetPayableMinor, v.EmployerCostMinor, v.ActualPaidMinor, v.AllocatedPaidMinor, v.OverageMinor, v.Status, v.Revision, v.LineKind, v.SourcePayrollLineID, v.SourcePayrollLineRevision, v.AdjustmentReason, v.CreatedAt, v.PersonReferenceCode, v.PersonSourceDigest)
	return e
}
func contributionJSON(values []ContributionAmount) ([]byte, []byte) {
	employee := map[string]ContributionAmount{}
	employer := map[string]ContributionAmount{}
	for _, v := range values {
		employee[v.Key] = v
		employer[v.Key] = v
	}
	a, _ := json.Marshal(employee)
	b, _ := json.Marshal(employer)
	return a, b
}

func (s *PostgresStore) GetExecutionRun(ctx context.Context, tenant, id string) (ExecutionRun, error) {
	return loadExecutionRun(ctx, s.pool, tenant, id, "")
}
func (s *PostgresStore) ListExecutionRuns(ctx context.Context, tenant string) ([]ExecutionRun, error) {
	rows, e := s.pool.Query(ctx, executionRunSelect+` WHERE tenant_id=$1 ORDER BY period DESC,created_at DESC,id`, tenant)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	values := []ExecutionRun{}
	for rows.Next() {
		v, e := scanExecutionRun(rows)
		if e != nil {
			return nil, e
		}
		v.Lines, e = loadExecutionLines(ctx, s.pool, tenant, v.ID)
		if e != nil {
			return nil, e
		}
		values = append(values, v)
	}
	return values, rows.Err()
}

func (s *PostgresStore) SyncMembers(ctx context.Context, actor Actor, runID string, in SyncMembersInput, people []PersonReference, requestHash string) (ExecutionRun, error) {
	tx, err := s.begin(ctx, actor.TenantID, in.CommandID)
	if err != nil {
		return ExecutionRun{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, replayErr := replayValue[ExecutionRun](ctx, tx, actor, in.CommandID, requestHash); replayErr != nil || found {
		return value, replayErr
	}
	run, err := loadExecutionRun(ctx, tx, actor.TenantID, runID, "FOR UPDATE")
	if err != nil {
		return ExecutionRun{}, err
	}
	if run.Revision != in.ExpectedRevision {
		return ExecutionRun{}, ErrRevisionConflict
	}
	if run.RunKind != "regular" || run.Status == "voided" || hasBlockingDeliveries(ctx, tx, actor.TenantID, runID, "succeeded", "unknown") {
		return ExecutionRun{}, ErrRunLocked
	}
	existing := make(map[string]bool, len(run.Lines))
	for _, line := range run.Lines {
		existing[line.PersonID] = true
	}
	added := make([]PayrollLine, 0, len(people))
	now := time.Now().UTC()
	for _, person := range people {
		if existing[person.ID] {
			continue
		}
		line, buildErr := s.buildGeneratedLine(ctx, tx, run, person, run.ContributionSchemeSnapshot, now)
		if buildErr != nil {
			return ExecutionRun{}, buildErr
		}
		added = append(added, line)
		existing[person.ID] = true
	}
	if len(added) == 0 {
		if err = recordValue(ctx, tx, actor, in.CommandID, requestHash, run, now); err == nil {
			err = tx.Commit(ctx)
		}
		return run, err
	}
	run.Revision++
	run.Status = "review_pending"
	run.UpdatedAt = now
	for _, line := range added {
		if err = insertPayrollLine(ctx, tx, line); err != nil {
			return ExecutionRun{}, err
		}
		if err = insertPayrollLineRevision(ctx, tx, line, run.Revision, actor, in.CommandID, "member_synced", now); err != nil {
			return ExecutionRun{}, err
		}
		run.Lines = append(run.Lines, line)
	}
	if err = updateExecutionRun(ctx, tx, run); err != nil {
		return ExecutionRun{}, err
	}
	if err = insertExecutionRunRevision(ctx, tx, run, actor, in.CommandID, "member_synced"); err != nil {
		return ExecutionRun{}, err
	}
	if err = insertAudit(ctx, tx, run.Run, actor, in.CommandID, in.ExpectedRevision, "payroll_member_synced"); err != nil {
		return ExecutionRun{}, err
	}
	if err = recordValue(ctx, tx, actor, in.CommandID, requestHash, run, now); err != nil {
		return ExecutionRun{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return ExecutionRun{}, err
	}
	return run, nil
}

func compensationSyncPreview(ctx context.Context, q queryRows, run ExecutionRun, line PayrollLine) (SnapshotSyncPreview, error) {
	comp, err := loadCompensationForPeriod(ctx, q, run.TenantID, line.PersonID, run.Period)
	if err != nil {
		return SnapshotSyncPreview{}, err
	}
	after := line
	after.CompensationProfileID = comp.ID
	after.CompensationProfileRevision = comp.Revision
	after.FixedSalaryMinor = comp.FixedSalaryMinor
	after.SocialInsuranceBaseMinor = comp.SocialInsuranceBaseMinor
	after.HousingFundBaseMinor = comp.HousingFundBaseMinor
	var proration AttendanceProration
	after.Adjustments, proration, err = withAttendanceAdjustment(after.Adjustments, after.FixedSalaryMinor, after.AttendanceSummary)
	after.AttendanceProrationMinor = proration.AdjustmentMinor
	if err == nil {
		after.GrossMinor, after.Contributions, after.EmployeeContributionMinor, after.EmployerContributionMinor, after.PreTaxPayableMinor, after.NetPayableMinor, after.EmployerCostMinor, err = CalculatePayrollLine(after.FixedSalaryMinor, after.SocialInsuranceBaseMinor, after.HousingFundBaseMinor, after.PerformanceMinor, after.TaxMinor, after.Adjustments, run.ContributionSchemeSnapshot.Components)
	}
	if err != nil {
		return SnapshotSyncPreview{}, err
	}
	after.Status = "review_pending"
	after.Revision++
	preview := SnapshotSyncPreview{Kind: "compensation", PayrollRunID: run.ID, PayrollRunRevision: run.Revision, PayrollLineID: line.ID, PayrollLineRevision: line.Revision, SourceProfileID: comp.ID, SourceProfileRevision: comp.Revision, Before: line, After: after}
	preview.PreviewHash = snapshotSyncHash(preview)
	return preview, nil
}

func paymentProfileSyncPreview(ctx context.Context, q queryRows, run ExecutionRun, line PayrollLine) (SnapshotSyncPreview, error) {
	payment, err := loadPaymentForPeriod(ctx, q, run.TenantID, line.PersonID, run.Period)
	if err != nil {
		return SnapshotSyncPreview{}, err
	}
	after := line
	after.PaymentProfileID = payment.ID
	after.PaymentProfileRevision = payment.Revision
	after.PaymentAccountName = payment.AccountName
	after.PaymentBankName = payment.BankName
	after.paymentAccountNumber = payment.accountNumber
	after.MaskedPaymentAccount = payment.MaskedAccountNumber
	after.PaymentAccountMissing = false
	after.Status = "review_pending"
	after.Revision++
	preview := SnapshotSyncPreview{Kind: "payment_profile", PayrollRunID: run.ID, PayrollRunRevision: run.Revision, PayrollLineID: line.ID, PayrollLineRevision: line.Revision, SourceProfileID: payment.ID, SourceProfileRevision: payment.Revision, Before: line, After: after}
	preview.PreviewHash = snapshotSyncHash(preview)
	return preview, nil
}

func snapshotSyncHash(preview SnapshotSyncPreview) string {
	preview.PreviewHash = ""
	return hash(preview)
}

func (s *PostgresStore) PreviewCompensationSync(ctx context.Context, tenant, runID, lineID string) (SnapshotSyncPreview, error) {
	run, err := loadExecutionRun(ctx, s.pool, tenant, runID, "")
	if err != nil {
		return SnapshotSyncPreview{}, err
	}
	line, index := findLine(run.Lines, lineID)
	if index < 0 {
		return SnapshotSyncPreview{}, ErrNotFound
	}
	return compensationSyncPreview(ctx, s.pool, run, line)
}
func (s *PostgresStore) PreviewPaymentProfileSync(ctx context.Context, tenant, runID, lineID string) (SnapshotSyncPreview, error) {
	run, err := loadExecutionRun(ctx, s.pool, tenant, runID, "")
	if err != nil {
		return SnapshotSyncPreview{}, err
	}
	line, index := findLine(run.Lines, lineID)
	if index < 0 {
		return SnapshotSyncPreview{}, ErrNotFound
	}
	return paymentProfileSyncPreview(ctx, s.pool, run, line)
}

func (s *PostgresStore) ConfirmCompensationSync(ctx context.Context, actor Actor, runID, lineID string, in ConfirmSnapshotSyncInput, requestHash string) (ExecutionRun, error) {
	return s.confirmSnapshotSync(ctx, actor, runID, lineID, in, requestHash, "compensation")
}
func (s *PostgresStore) ConfirmPaymentProfileSync(ctx context.Context, actor Actor, runID, lineID string, in ConfirmSnapshotSyncInput, requestHash string) (ExecutionRun, error) {
	return s.confirmSnapshotSync(ctx, actor, runID, lineID, in, requestHash, "payment_profile")
}
func (s *PostgresStore) confirmSnapshotSync(ctx context.Context, actor Actor, runID, lineID string, in ConfirmSnapshotSyncInput, requestHash, kind string) (ExecutionRun, error) {
	tx, err := s.begin(ctx, actor.TenantID, in.CommandID)
	if err != nil {
		return ExecutionRun{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, replayErr := replayValue[ExecutionRun](ctx, tx, actor, in.CommandID, requestHash); replayErr != nil || found {
		return value, replayErr
	}
	run, err := loadExecutionRun(ctx, tx, actor.TenantID, runID, "FOR UPDATE")
	if err != nil {
		return ExecutionRun{}, err
	}
	if run.Revision != in.ExpectedRunRevision {
		return ExecutionRun{}, ErrRevisionConflict
	}
	if run.Status == "voided" {
		return ExecutionRun{}, ErrRunLocked
	}
	if kind == "compensation" && hasBlockingDeliveries(ctx, tx, actor.TenantID, runID, "succeeded", "unknown") {
		return ExecutionRun{}, ErrRunLocked
	}
	if kind == "payment_profile" && hasBlockingLineDeliveries(ctx, tx, actor.TenantID, lineID, "succeeded", "unknown") {
		return ExecutionRun{}, ErrRunLocked
	}
	line, index := findLine(run.Lines, lineID)
	if index < 0 {
		return ExecutionRun{}, ErrNotFound
	}
	if line.Revision != in.ExpectedLineRevision {
		return ExecutionRun{}, ErrRevisionConflict
	}
	var preview SnapshotSyncPreview
	if kind == "compensation" {
		preview, err = compensationSyncPreview(ctx, tx, run, line)
	} else {
		preview, err = paymentProfileSyncPreview(ctx, tx, run, line)
	}
	if err != nil {
		return ExecutionRun{}, err
	}
	if preview.PreviewHash != in.PreviewHash {
		return ExecutionRun{}, ErrPreviewStale
	}
	if (kind == "compensation" && line.CompensationProfileRevision == preview.SourceProfileRevision) || (kind == "payment_profile" && line.PaymentProfileRevision == preview.SourceProfileRevision && !line.PaymentAccountMissing) {
		if err = recordValue(ctx, tx, actor, in.CommandID, requestHash, run, time.Now().UTC()); err == nil {
			err = tx.Commit(ctx)
		}
		return run, err
	}
	now := time.Now().UTC()
	updated := preview.After
	updated.UpdatedAt = now
	run.Revision++
	run.Status = "review_pending"
	run.UpdatedAt = now
	run.Lines[index] = updated
	if err = updatePayrollLine(ctx, tx, updated); err != nil {
		return ExecutionRun{}, err
	}
	if err = updateExecutionRun(ctx, tx, run); err != nil {
		return ExecutionRun{}, err
	}
	event, runEvent, audit := "compensation_synced", "compensation_synced", "payroll_compensation_synced"
	if kind == "payment_profile" {
		event, runEvent, audit = "payment_profile_synced", "payment_profile_synced", "payroll_payment_profile_synced"
	}
	if err = insertPayrollLineRevision(ctx, tx, updated, run.Revision, actor, in.CommandID, event, now); err != nil {
		return ExecutionRun{}, err
	}
	if err = insertExecutionRunRevision(ctx, tx, run, actor, in.CommandID, runEvent); err != nil {
		return ExecutionRun{}, err
	}
	if err = insertAudit(ctx, tx, run.Run, actor, in.CommandID, in.ExpectedRunRevision, audit); err != nil {
		return ExecutionRun{}, err
	}
	if err = recordValue(ctx, tx, actor, in.CommandID, requestHash, run, now); err != nil {
		return ExecutionRun{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return ExecutionRun{}, err
	}
	return run, nil
}
func loadExecutionRunByPeriod(ctx context.Context, q queryRows, tenant, period string) (ExecutionRun, error) {
	row := q.QueryRow(ctx, executionRunSelect+` WHERE tenant_id=$1 AND period=$2 AND run_kind='regular' AND status<>'voided'`, tenant, period)
	v, e := scanExecutionRun(row)
	if errors.Is(e, pgx.ErrNoRows) {
		return v, ErrNotFound
	}
	if e != nil {
		return v, e
	}
	v.Lines, e = loadExecutionLines(ctx, q, tenant, v.ID)
	return v, e
}

type queryRows interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func loadExecutionRun(ctx context.Context, q queryRows, tenant, id, suffix string) (ExecutionRun, error) {
	v, e := scanExecutionRun(q.QueryRow(ctx, executionRunSelect+` WHERE tenant_id=$1 AND id=$2 `+suffix, tenant, id))
	if errors.Is(e, pgx.ErrNoRows) {
		return v, ErrNotFound
	}
	if e != nil {
		return v, e
	}
	v.Lines, e = loadExecutionLines(ctx, q, tenant, id)
	return v, e
}
func scanExecutionRun(row scanner) (ExecutionRun, error) {
	var v ExecutionRun
	var policyRaw, schemeRaw []byte
	e := row.Scan(&v.ID, &v.TenantID, &v.Period, &v.Label, &v.Status, &v.Revision, &v.CreatedBySubjectID, &v.CreatedAt, &v.UpdatedAt, &v.RunKind, &v.ReplacesRunID, &v.PlannedPayDate, &policyRaw, &v.ApprovalStatus, &v.ContributionSchemeID, &v.ContributionSchemeRevision, &schemeRaw, &v.VoidReason)
	if e == nil {
		e = json.Unmarshal(policyRaw, &v.ApprovalPolicySnapshot)
	}
	if e == nil && len(schemeRaw) > 2 {
		e = json.Unmarshal(schemeRaw, &v.ContributionSchemeSnapshot)
	}
	return v, e
}
func loadExecutionLines(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, tenant, run string) ([]PayrollLine, error) {
	rows, e := q.Query(ctx, executionLineSelect+` WHERE tenant_id=$1 AND payroll_run_id=$2 ORDER BY lower(person_display_name),person_id`, tenant, run)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	values := []PayrollLine{}
	for rows.Next() {
		v, e := scanExecutionLine(rows)
		if e != nil {
			return nil, e
		}
		values = append(values, v)
	}
	return values, rows.Err()
}
func scanExecutionLine(row scanner) (PayrollLine, error) {
	var v PayrollLine
	var attendanceRaw, adjustmentsRaw, employeeRaw, employerRaw []byte
	e := row.Scan(&v.ID, &v.TenantID, &v.PayrollRunID, &v.PersonID, &v.PersonReferenceCode, &v.PersonDisplayName, &v.PersonSourceRevision, &v.PersonSourceDigest, &v.PersonStatusSnapshot, &v.CompensationProfileID, &v.CompensationProfileRevision, &v.PaymentProfileID, &v.PaymentProfileRevision, &v.PaymentAccountName, &v.PaymentBankName, &v.paymentAccountNumber, &v.PaymentAccountMissing, &v.FixedSalaryMinor, &attendanceRaw, &v.AttendanceProrationMinor, &v.SocialInsuranceBaseMinor, &v.HousingFundBaseMinor, &v.PerformanceMinor, &adjustmentsRaw, &v.GrossMinor, &employeeRaw, &employerRaw, &v.EmployeeContributionMinor, &v.EmployerContributionMinor, &v.TaxMinor, &v.TaxSourceKind, &v.TaxSourceNote, &v.PreTaxPayableMinor, &v.NetPayableMinor, &v.EmployerCostMinor, &v.ActualPaidMinor, &v.AllocatedPaidMinor, &v.OverageMinor, &v.Status, &v.Revision, &v.LineKind, &v.SourcePayrollLineID, &v.SourcePayrollLineRevision, &v.AdjustmentReason, &v.CreatedAt, &v.UpdatedAt)
	if e != nil {
		return v, e
	}
	v.MaskedPaymentAccount = maskOptionalAccount(v.paymentAccountNumber)
	if len(attendanceRaw) != 0 {
		if e = json.Unmarshal(attendanceRaw, &v.AttendanceSummary); e != nil {
			return v, e
		}
	}
	if e = json.Unmarshal(adjustmentsRaw, &v.Adjustments); e != nil {
		return v, e
	}
	var components map[string]ContributionAmount
	if e = json.Unmarshal(employeeRaw, &components); e != nil {
		return v, e
	}
	v.Contributions = []ContributionAmount{}
	for _, item := range components {
		v.Contributions = append(v.Contributions, item)
	}
	sortContributionAmounts(v.Contributions)
	return v, nil
}
func sortContributionAmounts(v []ContributionAmount) {
	order := map[string]int{"pension": 0, "medical": 1, "unemployment": 2, "work_injury": 3, "maternity": 4, "housing_fund": 5}
	for i := 0; i < len(v); i++ {
		for j := i + 1; j < len(v); j++ {
			if order[v[j].Key] < order[v[i].Key] {
				v[i], v[j] = v[j], v[i]
			}
		}
	}
}
func maskOptionalAccount(v string) string {
	if v == "" {
		return ""
	}
	return maskAccount(v)
}

func (s *PostgresStore) RevisePayrollLine(ctx context.Context, actor Actor, runID, lineID string, in ReviseLineInput, requestHash string) (ExecutionRun, error) {
	tx, e := s.begin(ctx, actor.TenantID, in.CommandID)
	if e != nil {
		return ExecutionRun{}, e
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if v, found, e := replayValue[ExecutionRun](ctx, tx, actor, in.CommandID, requestHash); e != nil || found {
		return v, e
	}
	run, e := loadExecutionRun(ctx, tx, actor.TenantID, runID, "FOR UPDATE")
	if e != nil {
		return ExecutionRun{}, e
	}
	if run.Revision != in.ExpectedRunRevision {
		return ExecutionRun{}, ErrRevisionConflict
	}
	if run.Status == "voided" || hasBlockingDeliveries(ctx, tx, actor.TenantID, runID, "succeeded", "unknown") {
		return ExecutionRun{}, ErrRunLocked
	}
	line, index := findLine(run.Lines, lineID)
	if index < 0 {
		return ExecutionRun{}, ErrNotFound
	}
	if line.Revision != in.ExpectedLineRevision {
		return ExecutionRun{}, ErrRevisionConflict
	}
	effectiveAdjustments, proration, e := withAttendanceAdjustment(in.Adjustments, line.FixedSalaryMinor, line.AttendanceSummary)
	if e != nil {
		return ExecutionRun{}, e
	}
	gross, contrib, employee, employer, preTax, net, cost, e := CalculatePayrollLine(line.FixedSalaryMinor, line.SocialInsuranceBaseMinor, line.HousingFundBaseMinor, in.PerformanceMinor, in.TaxMinor, effectiveAdjustments, run.ContributionSchemeSnapshot.Components)
	if e != nil {
		return ExecutionRun{}, e
	}
	line.PerformanceMinor = in.PerformanceMinor
	line.TaxMinor = in.TaxMinor
	line.TaxSourceKind = in.TaxSourceKind
	line.TaxSourceNote = in.TaxSourceNote
	line.Adjustments = effectiveAdjustments
	line.AttendanceProrationMinor = proration.AdjustmentMinor
	line.GrossMinor = gross
	line.Contributions = contrib
	line.EmployeeContributionMinor = employee
	line.EmployerContributionMinor = employer
	line.PreTaxPayableMinor = preTax
	line.NetPayableMinor = net
	line.EmployerCostMinor = cost
	line.Status = "review_pending"
	line.Revision++
	line.UpdatedAt = time.Now().UTC()
	run.Revision++
	run.Status = "review_pending"
	run.UpdatedAt = line.UpdatedAt
	if e = updatePayrollLine(ctx, tx, line); e != nil {
		return ExecutionRun{}, e
	}
	run.Lines[index] = line
	if e = updateExecutionRun(ctx, tx, run); e != nil {
		return ExecutionRun{}, e
	}
	if e = insertPayrollLineRevision(ctx, tx, line, run.Revision, actor, in.CommandID, "revised", line.UpdatedAt); e != nil {
		return ExecutionRun{}, e
	}
	if e = insertExecutionRunRevision(ctx, tx, run, actor, in.CommandID, "line_revised"); e != nil {
		return ExecutionRun{}, e
	}
	if e = insertAudit(ctx, tx, run.Run, actor, in.CommandID, in.ExpectedRunRevision, "payroll_line_revised"); e != nil {
		return ExecutionRun{}, e
	}
	if e = recordValue(ctx, tx, actor, in.CommandID, requestHash, run, run.UpdatedAt); e != nil {
		return ExecutionRun{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return ExecutionRun{}, e
	}
	return run, nil
}

func (s *PostgresStore) ReviewPayrollRun(ctx context.Context, actor Actor, runID string, in RunCommandInput, requestHash string) (ExecutionRun, error) {
	tx, e := s.begin(ctx, actor.TenantID, in.CommandID)
	if e != nil {
		return ExecutionRun{}, e
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if v, found, e := replayValue[ExecutionRun](ctx, tx, actor, in.CommandID, requestHash); e != nil || found {
		return v, e
	}
	run, e := loadExecutionRun(ctx, tx, actor.TenantID, runID, "FOR UPDATE")
	if e != nil {
		return ExecutionRun{}, e
	}
	if run.Revision != in.ExpectedRevision {
		return ExecutionRun{}, ErrRevisionConflict
	}
	if run.Status == "voided" || hasBlockingDeliveries(ctx, tx, actor.TenantID, runID, "succeeded", "unknown") {
		return ExecutionRun{}, ErrRunLocked
	}
	now := time.Now().UTC()
	run.Revision++
	for i := range run.Lines {
		line := run.Lines[i]
		switch {
		case line.NetPayableMinor > 0:
			line.Status = "ready_to_pay"
		case line.NetPayableMinor == 0:
			line.Status = "zero_pending"
		default:
			line.Status = "recovery_pending"
		}
		line.Revision++
		line.UpdatedAt = now
		if e = updatePayrollLine(ctx, tx, line); e != nil {
			return ExecutionRun{}, e
		}
		if e = insertPayrollLineRevision(ctx, tx, line, run.Revision, actor, in.CommandID, "reviewed", now); e != nil {
			return ExecutionRun{}, e
		}
		run.Lines[i] = line
	}
	run.Status = projectRunStatus(run.Lines)
	run.UpdatedAt = now
	if e = updateExecutionRun(ctx, tx, run); e != nil {
		return ExecutionRun{}, e
	}
	if e = insertExecutionRunRevision(ctx, tx, run, actor, in.CommandID, "reviewed"); e != nil {
		return ExecutionRun{}, e
	}
	if e = insertAudit(ctx, tx, run.Run, actor, in.CommandID, in.ExpectedRevision, "payroll_run_reviewed"); e != nil {
		return ExecutionRun{}, e
	}
	if e = recordValue(ctx, tx, actor, in.CommandID, requestHash, run, now); e != nil {
		return ExecutionRun{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return ExecutionRun{}, e
	}
	return run, nil
}

func (s *PostgresStore) ConfirmZeroLine(ctx context.Context, actor Actor, runID, lineID string, in LineCommandInput, requestHash string) (ExecutionRun, error) {
	return s.changeSingleLine(ctx, actor, runID, lineID, in, requestHash, "zero_confirmed", "payroll_line_zero_confirmed", func(line *PayrollLine) error {
		if line.NetPayableMinor != 0 || line.Status != "zero_pending" {
			return ErrPaymentNotReady
		}
		line.Status = "no_payment_required"
		return nil
	})
}
func (s *PostgresStore) changeSingleLine(ctx context.Context, actor Actor, runID, lineID string, in LineCommandInput, requestHash, event, audit string, change func(*PayrollLine) error) (ExecutionRun, error) {
	tx, e := s.begin(ctx, actor.TenantID, in.CommandID)
	if e != nil {
		return ExecutionRun{}, e
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if v, found, e := replayValue[ExecutionRun](ctx, tx, actor, in.CommandID, requestHash); e != nil || found {
		return v, e
	}
	run, e := loadExecutionRun(ctx, tx, actor.TenantID, runID, "FOR UPDATE")
	if e != nil {
		return ExecutionRun{}, e
	}
	if run.Revision != in.ExpectedRunRevision {
		return ExecutionRun{}, ErrRevisionConflict
	}
	line, index := findLine(run.Lines, lineID)
	if index < 0 {
		return ExecutionRun{}, ErrNotFound
	}
	if line.Revision != in.ExpectedLineRevision {
		return ExecutionRun{}, ErrRevisionConflict
	}
	if e = change(&line); e != nil {
		return ExecutionRun{}, e
	}
	now := time.Now().UTC()
	line.Revision++
	line.UpdatedAt = now
	run.Revision++
	run.Lines[index] = line
	run.Status = projectRunStatus(run.Lines)
	run.UpdatedAt = now
	if e = updatePayrollLine(ctx, tx, line); e != nil {
		return ExecutionRun{}, e
	}
	if e = updateExecutionRun(ctx, tx, run); e != nil {
		return ExecutionRun{}, e
	}
	if e = insertPayrollLineRevision(ctx, tx, line, run.Revision, actor, in.CommandID, event, now); e != nil {
		return ExecutionRun{}, e
	}
	if e = insertExecutionRunRevision(ctx, tx, run, actor, in.CommandID, "line_zero_confirmed"); e != nil {
		return ExecutionRun{}, e
	}
	if e = insertAudit(ctx, tx, run.Run, actor, in.CommandID, in.ExpectedRunRevision, audit); e != nil {
		return ExecutionRun{}, e
	}
	if e = recordValue(ctx, tx, actor, in.CommandID, requestHash, run, now); e != nil {
		return ExecutionRun{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return ExecutionRun{}, e
	}
	return run, nil
}

func (s *PostgresStore) VoidPayrollRun(ctx context.Context, actor Actor, runID string, in RunCommandInput, requestHash string) (ExecutionRun, error) {
	tx, e := s.begin(ctx, actor.TenantID, in.CommandID)
	if e != nil {
		return ExecutionRun{}, e
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if v, found, e := replayValue[ExecutionRun](ctx, tx, actor, in.CommandID, requestHash); e != nil || found {
		return v, e
	}
	run, e := loadExecutionRun(ctx, tx, actor.TenantID, runID, "FOR UPDATE")
	if e != nil {
		return ExecutionRun{}, e
	}
	if run.Revision != in.ExpectedRevision {
		return ExecutionRun{}, ErrRevisionConflict
	}
	if run.Status == "voided" || hasBlockingDeliveries(ctx, tx, actor.TenantID, runID, "succeeded", "unknown") {
		return ExecutionRun{}, ErrRunLocked
	}
	now := time.Now().UTC()
	run.Revision++
	run.Status = "voided"
	run.VoidReason = in.Reason
	run.UpdatedAt = now
	for i := range run.Lines {
		run.Lines[i].Revision++
		run.Lines[i].Status = "voided"
		run.Lines[i].UpdatedAt = now
		if e = updatePayrollLine(ctx, tx, run.Lines[i]); e != nil {
			return ExecutionRun{}, e
		}
		if e = insertPayrollLineRevision(ctx, tx, run.Lines[i], run.Revision, actor, in.CommandID, "voided", now); e != nil {
			return ExecutionRun{}, e
		}
	}
	if e = updateExecutionRun(ctx, tx, run); e != nil {
		return ExecutionRun{}, e
	}
	if e = insertExecutionRunRevision(ctx, tx, run, actor, in.CommandID, "voided"); e != nil {
		return ExecutionRun{}, e
	}
	if e = insertAudit(ctx, tx, run.Run, actor, in.CommandID, in.ExpectedRevision, "payroll_run_voided"); e != nil {
		return ExecutionRun{}, e
	}
	if e = recordValue(ctx, tx, actor, in.CommandID, requestHash, run, now); e != nil {
		return ExecutionRun{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return ExecutionRun{}, e
	}
	return run, nil
}

func (s *PostgresStore) PreparePaymentDelivery(ctx context.Context, actor Actor, runID, lineID string, in RecordPaymentInput, requestHash string) (PaymentDelivery, PaymentIntent, error) {
	tx, e := s.begin(ctx, actor.TenantID, in.CommandID)
	if e != nil {
		return PaymentDelivery{}, PaymentIntent{}, e
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if existing, found, e := loadPaymentDeliveryByCommand(ctx, tx, actor.TenantID, in.CommandID); e != nil {
		return PaymentDelivery{}, PaymentIntent{}, e
	} else if found {
		if existing.requestHashValue != requestHash || existing.ActorSubjectID != actor.ID {
			return PaymentDelivery{}, PaymentIntent{}, ErrCommandConflict
		}
		intent := existing.intent
		if e = tx.Commit(ctx); e != nil {
			return PaymentDelivery{}, PaymentIntent{}, e
		}
		return existing, intent, nil
	}
	run, e := loadExecutionRun(ctx, tx, actor.TenantID, runID, "FOR UPDATE")
	if e != nil {
		return PaymentDelivery{}, PaymentIntent{}, e
	}
	if run.Revision != in.ExpectedRunRevision {
		return PaymentDelivery{}, PaymentIntent{}, ErrRevisionConflict
	}
	line, index := findLine(run.Lines, lineID)
	if index < 0 {
		return PaymentDelivery{}, PaymentIntent{}, ErrNotFound
	}
	if line.Revision != in.ExpectedLineRevision {
		return PaymentDelivery{}, PaymentIntent{}, ErrRevisionConflict
	}
	var blockingCount int
	if e = tx.QueryRow(ctx, `SELECT count(*) FROM payroll_payment_deliveries WHERE tenant_id=$1 AND payroll_line_id=$2 AND status IN ('pending','unknown')`, actor.TenantID, lineID).Scan(&blockingCount); e != nil {
		return PaymentDelivery{}, PaymentIntent{}, e
	}
	if blockingCount > 0 {
		return PaymentDelivery{}, PaymentIntent{}, ErrPaymentUnknown
	}
	if line.NetPayableMinor <= 0 || (line.Status != "ready_to_pay" && line.Status != "partially_paid") {
		return PaymentDelivery{}, PaymentIntent{}, ErrPaymentNotReady
	}
	if line.PaymentAccountMissing || line.paymentAccountNumber == "" {
		return PaymentDelivery{}, PaymentIntent{}, ErrPaymentAccount
	}
	remaining := line.NetPayableMinor - line.AllocatedPaidMinor
	if remaining <= 0 {
		return PaymentDelivery{}, PaymentIntent{}, ErrPaymentNotReady
	}
	if in.ActualAmountMinor > remaining && (!in.OverageConfirmed || in.OverageReason == "") {
		return PaymentDelivery{}, PaymentIntent{}, ErrOverageConfirmation
	}
	now := time.Now().UTC()
	value := PaymentDelivery{ID: newUUID(), TenantID: actor.TenantID, PayrollRunID: runID, PayrollLineID: lineID, PayrollLineRevision: line.Revision, CommandID: in.CommandID, Status: "pending", ActualAmountMinor: in.ActualAmountMinor, OccurredOn: in.OccurredOn, Note: in.Note, OverageConfirmed: in.OverageConfirmed, OverageReason: in.OverageReason, PaymentProfileID: line.PaymentProfileID, PaymentProfileRevision: line.PaymentProfileRevision, PaymentAccountName: line.PaymentAccountName, PaymentBankName: line.PaymentBankName, paymentAccountNumber: line.paymentAccountNumber, ActorSubjectID: actor.ID, CredentialID: actor.CredentialID, Revision: 1, CreatedAt: now, UpdatedAt: now, requestHashValue: requestHash}
	value.intent = deliveryIntent(value, run, line)
	intentRaw, _ := json.Marshal(value.intent)
	_, e = tx.Exec(ctx, `INSERT INTO payroll_payment_deliveries(id,tenant_id,payroll_run_id,payroll_line_id,payroll_line_revision,command_id,request_hash,intent_snapshot,status,actual_amount_minor,occurred_on,note,overage_confirmed,overage_reason,payment_profile_id,payment_profile_revision,payment_account_name,payment_bank_name,payment_account_number,actor_subject_id,credential_id,revision,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'pending',$9,$10::date,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,1,$21,$21)`, value.ID, value.TenantID, value.PayrollRunID, value.PayrollLineID, value.PayrollLineRevision, value.CommandID, requestHash, intentRaw, value.ActualAmountMinor, value.OccurredOn, value.Note, value.OverageConfirmed, value.OverageReason, value.PaymentProfileID, value.PaymentProfileRevision, value.PaymentAccountName, value.PaymentBankName, value.paymentAccountNumber, value.ActorSubjectID, value.CredentialID, now)
	if e != nil {
		return PaymentDelivery{}, PaymentIntent{}, e
	}
	if e = insertPaymentDeliveryRevision(ctx, tx, value); e != nil {
		return PaymentDelivery{}, PaymentIntent{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return PaymentDelivery{}, PaymentIntent{}, e
	}
	return value, value.intent, nil
}

func deliveryIntent(d PaymentDelivery, run ExecutionRun, line PayrollLine) PaymentIntent {
	return PaymentIntent{CommandID: d.CommandID, TenantID: d.TenantID, PayrollRunID: d.PayrollRunID, PayrollPeriod: run.Period, PayrollLabel: run.Label, PayrollLineID: d.PayrollLineID, PayrollLineRevision: d.PayrollLineRevision, PayrollLineDigest: "sha256:" + hash(line), PersonID: line.PersonID, PersonRevision: line.PersonSourceRevision, PersonDigest: line.PersonSourceDigest, PersonDisplayName: line.PersonDisplayName, TotalNetPayableMinor: line.NetPayableMinor, AmountMinor: d.ActualAmountMinor, OccurredOn: d.OccurredOn, Note: d.Note, PaymentProfileID: d.PaymentProfileID, PaymentProfileRevision: d.PaymentProfileRevision, AccountName: d.PaymentAccountName, BankName: d.PaymentBankName, MaskedAccount: line.MaskedPaymentAccount}
}

func (s *PostgresStore) FinishPaymentDelivery(ctx context.Context, actor Actor, prepared PaymentDelivery, result PaymentDeliveryResult) (ExecutionRun, error) {
	if result.Status != "succeeded" && result.Status != "failed" && result.Status != "unknown" {
		return ExecutionRun{}, fmt.Errorf("invalid payment delivery status")
	}
	tx, e := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if e != nil {
		return ExecutionRun{}, e
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	current, found, e := loadPaymentDeliveryByCommand(ctx, tx, actor.TenantID, prepared.CommandID)
	if e != nil || !found {
		return ExecutionRun{}, e
	}
	if current.Status == "succeeded" {
		run, e := loadExecutionRun(ctx, tx, actor.TenantID, current.PayrollRunID, "")
		if e == nil {
			e = tx.Commit(ctx)
		}
		return run, e
	}
	run, e := loadExecutionRun(ctx, tx, actor.TenantID, current.PayrollRunID, "FOR UPDATE")
	if e != nil {
		return ExecutionRun{}, e
	}
	line, index := findLine(run.Lines, current.PayrollLineID)
	if index < 0 {
		return ExecutionRun{}, ErrNotFound
	}
	now := time.Now().UTC()
	current.Status = result.Status
	current.PaymentDeliveryResult = result
	current.Revision++
	current.UpdatedAt = now
	if result.Status == "succeeded" {
		current.MoneyPaymentID = strings.ToLower(current.MoneyPaymentID)
		current.MoneyAllocationID = strings.ToLower(current.MoneyAllocationID)
		if !uuidPattern.MatchString(current.MoneyPaymentID) || result.MoneyPaymentRevision < 1 || !uuidPattern.MatchString(current.MoneyAllocationID) || result.MoneyAllocationRevision < 1 {
			return ExecutionRun{}, fmt.Errorf("invalid Money payment result")
		}
		remaining := line.NetPayableMinor - line.AllocatedPaidMinor
		allocated := current.ActualAmountMinor
		if allocated > remaining {
			allocated = remaining
		}
		if result.AllocatedAmountMinor != allocated || result.OverageAmountMinor != current.ActualAmountMinor-allocated {
			return ExecutionRun{}, fmt.Errorf("Money payment allocation does not match payroll")
		}
		current.AllocatedAmountMinor = allocated
		current.OverageAmountMinor = current.ActualAmountMinor - allocated
		line.ActualPaidMinor += current.ActualAmountMinor
		line.AllocatedPaidMinor += allocated
		line.OverageMinor += current.OverageAmountMinor
		if line.AllocatedPaidMinor >= line.NetPayableMinor {
			line.Status = "paid"
		} else {
			line.Status = "partially_paid"
		}
	}
	_, e = tx.Exec(ctx, `UPDATE payroll_payment_deliveries SET status=$3,allocated_amount_minor=$4,overage_amount_minor=$5,money_payment_id=NULLIF($6,'')::uuid,money_payment_revision=NULLIF($7,0),money_allocation_id=NULLIF($8,'')::uuid,money_allocation_revision=NULLIF($9,0),failure_code=$10,revision=$11,updated_at=$12 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, current.ID, current.Status, current.AllocatedAmountMinor, current.OverageAmountMinor, current.MoneyPaymentID, current.MoneyPaymentRevision, current.MoneyAllocationID, current.MoneyAllocationRevision, current.FailureCode, current.Revision, now)
	if e != nil {
		return ExecutionRun{}, e
	}
	if e = insertPaymentDeliveryRevision(ctx, tx, current); e != nil {
		return ExecutionRun{}, e
	}
	if result.Status != "succeeded" {
		if e = tx.Commit(ctx); e != nil {
			return ExecutionRun{}, e
		}
		return run, nil
	}
	run.Revision++
	run.UpdatedAt = now
	line.Revision++
	line.UpdatedAt = now
	run.Lines[index] = line
	run.Status = projectRunStatus(run.Lines)
	if e = updatePayrollLine(ctx, tx, line); e != nil {
		return ExecutionRun{}, e
	}
	if e = updateExecutionRun(ctx, tx, run); e != nil {
		return ExecutionRun{}, e
	}
	if e = insertPayrollLineRevision(ctx, tx, line, run.Revision, actor, current.CommandID, "payment_recorded", now); e != nil {
		return ExecutionRun{}, e
	}
	if e = insertExecutionRunRevision(ctx, tx, run, actor, current.CommandID, "payment_recorded"); e != nil {
		return ExecutionRun{}, e
	}
	if e = insertAudit(ctx, tx, run.Run, actor, current.CommandID, run.Revision-1, "payroll_payment_recorded"); e != nil {
		return ExecutionRun{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return ExecutionRun{}, e
	}
	return run, nil
}

func loadPaymentDeliveryByCommand(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, tenant, command string) (PaymentDelivery, bool, error) {
	var v PaymentDelivery
	var intentRaw []byte
	e := q.QueryRow(ctx, `SELECT id::text,tenant_id::text,payroll_run_id::text,payroll_line_id::text,payroll_line_revision,command_id::text,request_hash,intent_snapshot,status,actual_amount_minor,allocated_amount_minor,overage_amount_minor,occurred_on::text,note,overage_confirmed,overage_reason,payment_profile_id::text,payment_profile_revision,payment_account_name,payment_bank_name,payment_account_number,COALESCE(money_payment_id::text,''),COALESCE(money_payment_revision,0),COALESCE(money_allocation_id::text,''),COALESCE(money_allocation_revision,0),failure_code,actor_subject_id::text,credential_id,revision,created_at,updated_at FROM payroll_payment_deliveries WHERE tenant_id=$1 AND command_id=$2 FOR UPDATE`, tenant, command).Scan(&v.ID, &v.TenantID, &v.PayrollRunID, &v.PayrollLineID, &v.PayrollLineRevision, &v.CommandID, &v.requestHashValue, &intentRaw, &v.Status, &v.ActualAmountMinor, &v.AllocatedAmountMinor, &v.OverageAmountMinor, &v.OccurredOn, &v.Note, &v.OverageConfirmed, &v.OverageReason, &v.PaymentProfileID, &v.PaymentProfileRevision, &v.PaymentAccountName, &v.PaymentBankName, &v.paymentAccountNumber, &v.MoneyPaymentID, &v.MoneyPaymentRevision, &v.MoneyAllocationID, &v.MoneyAllocationRevision, &v.FailureCode, &v.ActorSubjectID, &v.CredentialID, &v.Revision, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(e, pgx.ErrNoRows) {
		return v, false, nil
	}
	if e != nil {
		return v, false, e
	}
	if e = json.Unmarshal(intentRaw, &v.intent); e != nil {
		return v, false, e
	}
	return v, true, nil
}

func insertPaymentDeliveryRevision(ctx context.Context, tx pgx.Tx, v PaymentDelivery) error {
	raw, _ := json.Marshal(map[string]any{"status": v.Status, "money_payment_id": v.MoneyPaymentID, "money_payment_revision": v.MoneyPaymentRevision, "money_allocation_id": v.MoneyAllocationID, "money_allocation_revision": v.MoneyAllocationRevision, "failure_code": v.FailureCode})
	_, e := tx.Exec(ctx, `INSERT INTO payroll_payment_delivery_revisions(tenant_id,delivery_id,revision,status,response_snapshot,occurred_at) VALUES($1,$2,$3,$4,$5,$6)`, v.TenantID, v.ID, v.Revision, v.Status, raw, v.UpdatedAt)
	return e
}

func findLine(lines []PayrollLine, id string) (PayrollLine, int) {
	for i, v := range lines {
		if v.ID == id {
			return v, i
		}
	}
	return PayrollLine{}, -1
}
func hasBlockingDeliveries(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, tenant, run string, statuses ...string) bool {
	var count int
	_ = q.QueryRow(ctx, `SELECT count(*) FROM payroll_payment_deliveries WHERE tenant_id=$1 AND payroll_run_id=$2 AND status=ANY($3)`, tenant, run, statuses).Scan(&count)
	return count > 0
}
func hasBlockingLineDeliveries(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, tenant, line string, statuses ...string) bool {
	var count int
	_ = q.QueryRow(ctx, `SELECT count(*) FROM payroll_payment_deliveries WHERE tenant_id=$1 AND payroll_line_id=$2 AND status=ANY($3)`, tenant, line, statuses).Scan(&count)
	return count > 0
}
func projectRunStatus(lines []PayrollLine) string {
	if len(lines) == 0 {
		return "review_pending"
	}
	review, payment, complete, recovery, progress := false, false, true, false, false
	for _, line := range lines {
		switch line.Status {
		case "review_pending":
			review = true
			complete = false
		case "ready_to_pay", "zero_pending":
			payment = true
			complete = false
		case "partially_paid":
			payment = true
			progress = true
			complete = false
		case "paid", "no_payment_required":
			progress = true
		case "recovery_pending":
			recovery = true
			complete = false
		case "voided":
			complete = false
		}
	}
	if review {
		return "review_pending"
	}
	if complete {
		return "paid"
	}
	if recovery {
		return "recovery_pending"
	}
	if progress {
		return "partially_paid"
	}
	if payment {
		return "ready_to_pay"
	}
	return "review_pending"
}

func updateExecutionRun(ctx context.Context, tx pgx.Tx, v ExecutionRun) error {
	policy, _ := json.Marshal(v.ApprovalPolicySnapshot)
	scheme, _ := json.Marshal(v.ContributionSchemeSnapshot)
	_, e := tx.Exec(ctx, `UPDATE payroll_runs SET label=$3,status=$4,revision=$5,planned_pay_date=$6::date,approval_policy_snapshot=$7,approval_status=$8,contribution_scheme_snapshot=$9,void_reason=$10,updated_at=$11 WHERE tenant_id=$1 AND id=$2`, v.TenantID, v.ID, v.Label, v.Status, v.Revision, v.PlannedPayDate, policy, v.ApprovalStatus, scheme, v.VoidReason, v.UpdatedAt)
	return e
}
func updatePayrollLine(ctx context.Context, tx pgx.Tx, v PayrollLine) error {
	adjustments, _ := json.Marshal(v.Adjustments)
	attendance, _ := json.Marshal(v.AttendanceSummary)
	if v.AttendanceSummary == nil {
		attendance = nil
	}
	employee, employer := contributionJSON(v.Contributions)
	_, e := tx.Exec(ctx, `UPDATE payroll_lines SET compensation_profile_id=NULLIF($3,'')::uuid,compensation_profile_revision=NULLIF($4,0),payment_profile_id=NULLIF($5,'')::uuid,payment_profile_revision=NULLIF($6,0),payment_account_name=$7,payment_bank_name=$8,payment_account_number=$9,payment_account_missing=$10,fixed_salary_minor=$11,attendance_summary_snapshot=$12,attendance_proration_minor=$13,social_insurance_base_minor=$14,housing_fund_base_minor=$15,performance_minor=$16,adjustments=$17,gross_minor=$18,employee_contributions=$19,employer_contributions=$20,employee_contribution_minor=$21,employer_contribution_minor=$22,tax_minor=$23,tax_source_kind=$24,tax_source_note=$25,pre_tax_payable_minor=$26,net_payable_minor=$27,employer_cost_minor=$28,actual_paid_minor=$29,allocated_paid_minor=$30,overage_minor=$31,status=$32,revision=$33,updated_at=$34 WHERE tenant_id=$1 AND id=$2`, v.TenantID, v.ID, v.CompensationProfileID, v.CompensationProfileRevision, v.PaymentProfileID, v.PaymentProfileRevision, v.PaymentAccountName, v.PaymentBankName, v.paymentAccountNumber, v.PaymentAccountMissing, v.FixedSalaryMinor, attendance, v.AttendanceProrationMinor, v.SocialInsuranceBaseMinor, v.HousingFundBaseMinor, v.PerformanceMinor, adjustments, v.GrossMinor, employee, employer, v.EmployeeContributionMinor, v.EmployerContributionMinor, v.TaxMinor, v.TaxSourceKind, v.TaxSourceNote, v.PreTaxPayableMinor, v.NetPayableMinor, v.EmployerCostMinor, v.ActualPaidMinor, v.AllocatedPaidMinor, v.OverageMinor, v.Status, v.Revision, v.UpdatedAt)
	return e
}
func insertExecutionRunRevision(ctx context.Context, tx pgx.Tx, v ExecutionRun, actor Actor, command, event string) error {
	policy, _ := json.Marshal(v.ApprovalPolicySnapshot)
	scheme, _ := json.Marshal(v.ContributionSchemeSnapshot)
	_, e := tx.Exec(ctx, `INSERT INTO payroll_run_revisions(tenant_id,payroll_run_id,revision,command_id,event_kind,period,label,status,actor_subject_id,credential_id,occurred_at,run_kind,replaces_run_id,planned_pay_date,approval_policy_snapshot,approval_status,contribution_scheme_id,contribution_scheme_revision,contribution_scheme_snapshot,void_reason) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NULLIF($13,'')::uuid,$14::date,$15,$16,NULLIF($17,'')::uuid,NULLIF($18,0),$19,$20)`, v.TenantID, v.ID, v.Revision, command, event, v.Period, v.Label, v.Status, actor.ID, actor.CredentialID, v.UpdatedAt, v.RunKind, v.ReplacesRunID, v.PlannedPayDate, policy, v.ApprovalStatus, v.ContributionSchemeID, v.ContributionSchemeRevision, scheme, v.VoidReason)
	return e
}
func insertPayrollLineRevision(ctx context.Context, tx pgx.Tx, v PayrollLine, runRevision int64, actor Actor, command, event string, at time.Time) error {
	raw, _ := json.Marshal(v)
	var snapshot map[string]any
	_ = json.Unmarshal(raw, &snapshot)
	snapshot["payment_account_number"] = v.paymentAccountNumber
	raw, _ = json.Marshal(snapshot)
	_, e := tx.Exec(ctx, `INSERT INTO payroll_line_revisions(tenant_id,payroll_run_id,payroll_run_revision,payroll_line_id,revision,command_id,event_kind,snapshot,actor_subject_id,credential_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, v.TenantID, v.PayrollRunID, runRevision, v.ID, v.Revision, command, event, raw, actor.ID, actor.CredentialID, at)
	return e
}

var _ ExecutionStore = (*PostgresStore)(nil)
