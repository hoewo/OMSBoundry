package payroll

import (
	"context"
	"encoding/json"
	"strings"
)

type Payslip struct {
	PayrollRunID              string               `json:"payroll_run_id"`
	PayrollRunRevision        int64                `json:"payroll_run_revision"`
	PayrollLineID             string               `json:"payroll_line_id"`
	PayrollLineRevision       int64                `json:"payroll_line_revision"`
	Period                    string               `json:"period"`
	Label                     string               `json:"label"`
	RunKind                   string               `json:"run_kind"`
	LineKind                  string               `json:"line_kind"`
	PlannedPayDate            string               `json:"planned_pay_date"`
	PaidOn                    string               `json:"paid_on,omitempty"`
	Status                    string               `json:"status"`
	FixedSalaryMinor          int64                `json:"fixed_salary_minor"`
	PerformanceMinor          int64                `json:"performance_minor"`
	Adjustments               []Adjustment         `json:"adjustments"`
	GrossMinor                int64                `json:"gross_minor"`
	Contributions             []ContributionAmount `json:"contributions"`
	EmployeeContributionMinor int64                `json:"employee_contribution_minor"`
	TaxMinor                  int64                `json:"tax_minor"`
	PreTaxPayableMinor        int64                `json:"pre_tax_payable_minor"`
	NetPayableMinor           int64                `json:"net_payable_minor"`
	ActualPaidMinor           int64                `json:"actual_paid_minor"`
	AllocatedPaidMinor        int64                `json:"allocated_paid_minor"`
	OverageMinor              int64                `json:"overage_minor"`
	SourcePayrollLineID       string               `json:"source_payroll_line_id,omitempty"`
	SourcePayrollLineRevision int64                `json:"source_payroll_line_revision,omitempty"`
	AdjustmentReason          string               `json:"adjustment_reason,omitempty"`
}

type SelfPayslipPage struct {
	PersonID             string    `json:"person_id"`
	PersonDisplayName    string    `json:"person_display_name"`
	PersonStatus         string    `json:"person_status"`
	PersonSourceRevision int64     `json:"person_source_revision"`
	Items                []Payslip `json:"items"`
}

type selfPayslipStore interface {
	ListSettledPayslips(context.Context, string, string) ([]Payslip, error)
}

func (s *Service) ListSelfPayslips(ctx context.Context, actor Actor) (SelfPayslipPage, error) {
	if err := validateActor(actor); err != nil {
		return SelfPayslipPage{}, err
	}
	if s.personReferences == nil {
		return SelfPayslipPage{}, ErrReferenceUnavailable
	}
	person, err := s.personReferences.ResolvePersonForSubject(ctx, actor.TenantID, actor.ID)
	if err != nil {
		return SelfPayslipPage{}, err
	}
	if !uuidPattern.MatchString(person.ID) || person.TenantID != actor.TenantID || person.Revision < 1 || (person.Status != "active" && person.Status != "inactive") || strings.TrimSpace(person.DisplayName) == "" || !sourceDigestPattern.MatchString(person.Digest) {
		return SelfPayslipPage{}, ErrReferenceInvalid
	}
	store, ok := s.store.(selfPayslipStore)
	if !ok {
		return SelfPayslipPage{}, ErrReferenceUnavailable
	}
	items, err := store.ListSettledPayslips(ctx, actor.TenantID, person.ID)
	if err != nil {
		return SelfPayslipPage{}, err
	}
	return SelfPayslipPage{PersonID: person.ID, PersonDisplayName: person.DisplayName, PersonStatus: person.Status, PersonSourceRevision: person.Revision, Items: items}, nil
}

func (s *PostgresStore) ListSettledPayslips(ctx context.Context, tenant, person string) ([]Payslip, error) {
	rows, err := s.pool.Query(ctx, `SELECT r.id::text,r.revision,l.id::text,l.revision,r.period,r.label,r.run_kind,l.line_kind,r.planned_pay_date::text,
		COALESCE((SELECT to_char(max(d.occurred_on),'YYYY-MM-DD') FROM payroll_payment_deliveries d WHERE d.tenant_id=l.tenant_id AND d.payroll_line_id=l.id AND d.status='succeeded'),''),
		l.status,l.fixed_salary_minor,l.performance_minor,l.adjustments,l.gross_minor,l.employee_contributions,l.employee_contribution_minor,l.tax_minor,l.pre_tax_payable_minor,l.net_payable_minor,l.actual_paid_minor,l.allocated_paid_minor,l.overage_minor,COALESCE(l.source_payroll_line_id::text,''),COALESCE(l.source_payroll_line_revision,0),l.adjustment_reason
		FROM payroll_lines l JOIN payroll_runs r ON r.tenant_id=l.tenant_id AND r.id=l.payroll_run_id
		WHERE l.tenant_id=$1 AND l.person_id=$2 AND l.status IN ('paid','no_payment_required') AND r.status<>'voided'
		ORDER BY r.period DESC,r.created_at DESC,l.created_at DESC,l.id`, tenant, person)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Payslip{}
	for rows.Next() {
		var value Payslip
		var adjustmentsRaw, contributionsRaw []byte
		if err = rows.Scan(&value.PayrollRunID, &value.PayrollRunRevision, &value.PayrollLineID, &value.PayrollLineRevision, &value.Period, &value.Label, &value.RunKind, &value.LineKind, &value.PlannedPayDate, &value.PaidOn, &value.Status, &value.FixedSalaryMinor, &value.PerformanceMinor, &adjustmentsRaw, &value.GrossMinor, &contributionsRaw, &value.EmployeeContributionMinor, &value.TaxMinor, &value.PreTaxPayableMinor, &value.NetPayableMinor, &value.ActualPaidMinor, &value.AllocatedPaidMinor, &value.OverageMinor, &value.SourcePayrollLineID, &value.SourcePayrollLineRevision, &value.AdjustmentReason); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(adjustmentsRaw, &value.Adjustments); err != nil {
			return nil, err
		}
		var contributionMap map[string]ContributionAmount
		if err = json.Unmarshal(contributionsRaw, &contributionMap); err != nil {
			return nil, err
		}
		value.Contributions = []ContributionAmount{}
		for _, contribution := range contributionMap {
			value.Contributions = append(value.Contributions, contribution)
		}
		sortContributionAmounts(value.Contributions)
		items = append(items, value)
	}
	return items, rows.Err()
}
