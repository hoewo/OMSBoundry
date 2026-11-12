package payroll

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	attendancecontract "omsbizfoundry/attendance-summary-contract"
)

var (
	ErrRunLocked           = errors.New("payroll run is locked by payment facts")
	ErrPaymentAccount      = errors.New("payroll payment account is missing")
	ErrPaymentNotReady     = errors.New("payroll line is not ready for payment")
	ErrPaymentUnknown      = errors.New("payroll payment result is unknown")
	ErrPaymentFailed       = errors.New("payroll payment failed")
	ErrPreviewStale        = errors.New("payroll preview is stale")
	ErrOverageConfirmation = errors.New("payroll overage confirmation is required")
)

type Adjustment struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	AmountMinor int64  `json:"amount_minor"`
	Reason      string `json:"reason"`
}

type ContributionAmount struct {
	Key                  string `json:"key"`
	BaseMinor            int64  `json:"base_minor"`
	IndividualRateBPS    int64  `json:"individual_rate_bps"`
	EmployerRateBPS      int64  `json:"employer_rate_bps"`
	IndividualFixedMinor int64  `json:"individual_fixed_minor,omitempty"`
	EmployerFixedMinor   int64  `json:"employer_fixed_minor,omitempty"`
	IndividualMinor      int64  `json:"individual_minor"`
	EmployerMinor        int64  `json:"employer_minor"`
	RoundingMode         string `json:"rounding_mode"`
}

type PayrollLine struct {
	ID                          string                             `json:"id"`
	TenantID                    string                             `json:"tenant_id"`
	PayrollRunID                string                             `json:"payroll_run_id"`
	PersonID                    string                             `json:"person_id"`
	PersonReferenceCode         string                             `json:"person_reference_code,omitempty"`
	PersonDisplayName           string                             `json:"person_display_name"`
	PersonSourceRevision        int64                              `json:"person_source_revision"`
	PersonSourceDigest          string                             `json:"person_source_digest"`
	PersonStatusSnapshot        string                             `json:"person_status_snapshot"`
	CompensationProfileID       string                             `json:"compensation_profile_id,omitempty"`
	CompensationProfileRevision int64                              `json:"compensation_profile_revision,omitempty"`
	PaymentProfileID            string                             `json:"payment_profile_id,omitempty"`
	PaymentProfileRevision      int64                              `json:"payment_profile_revision,omitempty"`
	PaymentAccountName          string                             `json:"payment_account_name,omitempty"`
	PaymentBankName             string                             `json:"payment_bank_name,omitempty"`
	MaskedPaymentAccount        string                             `json:"masked_payment_account,omitempty"`
	PaymentAccountMissing       bool                               `json:"payment_account_missing"`
	FixedSalaryMinor            int64                              `json:"fixed_salary_minor"`
	AttendanceSummary           *attendancecontract.MonthlySummary `json:"attendance_summary,omitempty"`
	AttendanceProrationMinor    int64                              `json:"attendance_proration_minor"`
	SocialInsuranceBaseMinor    int64                              `json:"social_insurance_base_minor"`
	HousingFundBaseMinor        int64                              `json:"housing_fund_base_minor"`
	PerformanceMinor            int64                              `json:"performance_minor"`
	Adjustments                 []Adjustment                       `json:"adjustments"`
	GrossMinor                  int64                              `json:"gross_minor"`
	Contributions               []ContributionAmount               `json:"contributions"`
	EmployeeContributionMinor   int64                              `json:"employee_contribution_minor"`
	EmployerContributionMinor   int64                              `json:"employer_contribution_minor"`
	TaxMinor                    int64                              `json:"tax_minor"`
	TaxSourceKind               string                             `json:"tax_source_kind"`
	TaxSourceNote               string                             `json:"tax_source_note,omitempty"`
	PreTaxPayableMinor          int64                              `json:"pre_tax_payable_minor"`
	NetPayableMinor             int64                              `json:"net_payable_minor"`
	EmployerCostMinor           int64                              `json:"employer_cost_minor"`
	ActualPaidMinor             int64                              `json:"actual_paid_minor"`
	AllocatedPaidMinor          int64                              `json:"allocated_paid_minor"`
	OverageMinor                int64                              `json:"overage_minor"`
	Status                      string                             `json:"status"`
	Revision                    int64                              `json:"revision"`
	LineKind                    string                             `json:"line_kind"`
	SourcePayrollLineID         string                             `json:"source_payroll_line_id,omitempty"`
	SourcePayrollLineRevision   int64                              `json:"source_payroll_line_revision,omitempty"`
	AdjustmentReason            string                             `json:"adjustment_reason,omitempty"`
	CreatedAt                   time.Time                          `json:"created_at"`
	UpdatedAt                   time.Time                          `json:"updated_at"`
	paymentAccountNumber        string
}

type ExecutionRun struct {
	Run
	RunKind                    string             `json:"run_kind"`
	ReplacesRunID              string             `json:"replaces_run_id,omitempty"`
	PlannedPayDate             string             `json:"planned_pay_date"`
	ApprovalPolicySnapshot     map[string]any     `json:"approval_policy_snapshot"`
	ApprovalStatus             string             `json:"approval_status"`
	ContributionSchemeID       string             `json:"contribution_scheme_id,omitempty"`
	ContributionSchemeRevision int64              `json:"contribution_scheme_revision,omitempty"`
	ContributionSchemeSnapshot ContributionScheme `json:"contribution_scheme_snapshot"`
	VoidReason                 string             `json:"void_reason,omitempty"`
	Lines                      []PayrollLine      `json:"lines"`
}

type CreateRegularRunInput struct {
	CommandID                  string   `json:"command_id"`
	Period                     string   `json:"period"`
	Label                      string   `json:"label"`
	PlannedPayDate             string   `json:"planned_pay_date"`
	ContributionSchemeID       string   `json:"contribution_scheme_id"`
	ContributionSchemeRevision int64    `json:"contribution_scheme_revision"`
	PersonIDs                  []string `json:"person_ids"`
	ReplacesRunID              string   `json:"replaces_run_id,omitempty"`
	ReplacesRunRevision        int64    `json:"replaces_run_revision,omitempty"`
}

type AdjustmentRunItem struct {
	SourcePayrollLineID       string `json:"source_payroll_line_id"`
	SourcePayrollLineRevision int64  `json:"source_payroll_line_revision"`
	Direction                 string `json:"direction"`
	AmountMinor               int64  `json:"amount_minor"`
	Reason                    string `json:"reason"`
}

type CreateAdjustmentRunInput struct {
	CommandID      string              `json:"command_id"`
	Period         string              `json:"period"`
	Label          string              `json:"label"`
	PlannedPayDate string              `json:"planned_pay_date"`
	Items          []AdjustmentRunItem `json:"items"`
}

type ReviseLineInput struct {
	CommandID            string       `json:"command_id"`
	ExpectedRunRevision  int64        `json:"expected_run_revision"`
	ExpectedLineRevision int64        `json:"expected_line_revision"`
	PerformanceMinor     int64        `json:"performance_minor"`
	TaxMinor             int64        `json:"tax_minor"`
	TaxSourceKind        string       `json:"tax_source_kind"`
	TaxSourceNote        string       `json:"tax_source_note"`
	Adjustments          []Adjustment `json:"adjustments"`
	Reason               string       `json:"reason"`
}

type ApplyAttendanceSummaryInput struct {
	CommandID            string `json:"command_id"`
	ExpectedRunRevision  int64  `json:"expected_run_revision"`
	ExpectedLineRevision int64  `json:"expected_line_revision"`
	SummaryID            string `json:"summary_id"`
	Reason               string `json:"reason"`
}

type RunCommandInput struct {
	CommandID        string `json:"command_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Reason           string `json:"reason"`
}

type LineCommandInput struct {
	CommandID            string `json:"command_id"`
	ExpectedRunRevision  int64  `json:"expected_run_revision"`
	ExpectedLineRevision int64  `json:"expected_line_revision"`
	Reason               string `json:"reason"`
}

type RecordPaymentInput struct {
	CommandID            string `json:"command_id"`
	ExpectedRunRevision  int64  `json:"expected_run_revision"`
	ExpectedLineRevision int64  `json:"expected_line_revision"`
	ActualAmountMinor    int64  `json:"actual_amount_minor"`
	OccurredOn           string `json:"occurred_on"`
	Note                 string `json:"note"`
	OverageConfirmed     bool   `json:"overage_confirmed"`
	OverageReason        string `json:"overage_reason"`
}

type SyncMembersInput struct {
	CommandID        string   `json:"command_id"`
	ExpectedRevision int64    `json:"expected_revision"`
	PersonIDs        []string `json:"person_ids"`
	Reason           string   `json:"reason"`
}

type SnapshotSyncPreview struct {
	Kind                  string      `json:"kind"`
	PayrollRunID          string      `json:"payroll_run_id"`
	PayrollRunRevision    int64       `json:"payroll_run_revision"`
	PayrollLineID         string      `json:"payroll_line_id"`
	PayrollLineRevision   int64       `json:"payroll_line_revision"`
	SourceProfileID       string      `json:"source_profile_id"`
	SourceProfileRevision int64       `json:"source_profile_revision"`
	Before                PayrollLine `json:"before"`
	After                 PayrollLine `json:"after"`
	PreviewHash           string      `json:"preview_hash"`
}

type ConfirmSnapshotSyncInput struct {
	CommandID            string `json:"command_id"`
	ExpectedRunRevision  int64  `json:"expected_run_revision"`
	ExpectedLineRevision int64  `json:"expected_line_revision"`
	PreviewHash          string `json:"preview_hash"`
	Reason               string `json:"reason"`
}

type PaymentIntent struct {
	CommandID              string `json:"command_id"`
	TenantID               string `json:"tenant_id"`
	PayrollRunID           string `json:"payroll_run_id"`
	PayrollPeriod          string `json:"payroll_period"`
	PayrollLabel           string `json:"payroll_label"`
	PayrollLineID          string `json:"payroll_line_id"`
	PayrollLineRevision    int64  `json:"payroll_line_revision"`
	PayrollLineDigest      string `json:"payroll_line_digest"`
	PersonID               string `json:"person_id"`
	PersonRevision         int64  `json:"person_revision"`
	PersonDigest           string `json:"person_digest"`
	PersonDisplayName      string `json:"person_display_name"`
	PaymentProfileID       string `json:"payment_profile_id"`
	PaymentProfileRevision int64  `json:"payment_profile_revision"`
	TotalNetPayableMinor   int64  `json:"total_net_payable_minor"`
	AmountMinor            int64  `json:"amount_minor"`
	OccurredOn             string `json:"occurred_on"`
	Note                   string `json:"note"`
	AccountName            string `json:"account_name"`
	BankName               string `json:"bank_name"`
	MaskedAccount          string `json:"masked_account"`
}

type PaymentDeliveryResult struct {
	Status                  string `json:"status"`
	MoneyPaymentID          string `json:"money_payment_id,omitempty"`
	MoneyPaymentRevision    int64  `json:"money_payment_revision,omitempty"`
	MoneyAllocationID       string `json:"money_allocation_id,omitempty"`
	MoneyAllocationRevision int64  `json:"money_allocation_revision,omitempty"`
	AllocatedAmountMinor    int64  `json:"allocated_amount_minor,omitempty"`
	OverageAmountMinor      int64  `json:"overage_amount_minor,omitempty"`
	FailureCode             string `json:"failure_code,omitempty"`
}

type PaymentDelivery struct {
	ID, TenantID, PayrollRunID, PayrollLineID, CommandID        string
	PayrollLineRevision                                         int64
	Status                                                      string
	ActualAmountMinor, AllocatedAmountMinor, OverageAmountMinor int64
	OccurredOn, Note, OverageReason                             string
	OverageConfirmed                                            bool
	PaymentProfileID                                            string
	PaymentProfileRevision                                      int64
	PaymentAccountName, PaymentBankName                         string
	paymentAccountNumber                                        string
	PaymentDeliveryResult
	ActorSubjectID, CredentialID string
	Revision                     int64
	CreatedAt, UpdatedAt         time.Time
	requestHashValue             string
	intent                       PaymentIntent
}

type PaymentDeliveryPort interface {
	Deliver(context.Context, PaymentIntent) (PaymentDeliveryResult, error)
}

type ExecutionStore interface {
	CreateRegularRun(context.Context, Actor, CreateRegularRunInput, []PersonReference, string) (ExecutionRun, error)
	CreateAdjustmentRun(context.Context, Actor, CreateAdjustmentRunInput, string) (ExecutionRun, error)
	GetExecutionRun(context.Context, string, string) (ExecutionRun, error)
	ListExecutionRuns(context.Context, string) ([]ExecutionRun, error)
	RevisePayrollLine(context.Context, Actor, string, string, ReviseLineInput, string) (ExecutionRun, error)
	ApplyAttendanceSummary(context.Context, Actor, string, string, ApplyAttendanceSummaryInput, attendancecontract.MonthlySummary, string) (ExecutionRun, error)
	ReviewPayrollRun(context.Context, Actor, string, RunCommandInput, string) (ExecutionRun, error)
	ConfirmZeroLine(context.Context, Actor, string, string, LineCommandInput, string) (ExecutionRun, error)
	VoidPayrollRun(context.Context, Actor, string, RunCommandInput, string) (ExecutionRun, error)
	PreparePaymentDelivery(context.Context, Actor, string, string, RecordPaymentInput, string) (PaymentDelivery, PaymentIntent, error)
	FinishPaymentDelivery(context.Context, Actor, PaymentDelivery, PaymentDeliveryResult) (ExecutionRun, error)
	SyncMembers(context.Context, Actor, string, SyncMembersInput, []PersonReference, string) (ExecutionRun, error)
	PreviewCompensationSync(context.Context, string, string, string) (SnapshotSyncPreview, error)
	ConfirmCompensationSync(context.Context, Actor, string, string, ConfirmSnapshotSyncInput, string) (ExecutionRun, error)
	PreviewPaymentProfileSync(context.Context, string, string, string) (SnapshotSyncPreview, error)
	ConfirmPaymentProfileSync(context.Context, Actor, string, string, ConfirmSnapshotSyncInput, string) (ExecutionRun, error)
}

type AttendanceSummaryPort interface {
	GetMonthlySummary(context.Context, string, string) (attendancecontract.MonthlySummary, error)
}

func (s *Service) executionStore() (ExecutionStore, error) {
	store, ok := s.store.(ExecutionStore)
	if !ok {
		return nil, fmt.Errorf("payroll execution store is unavailable")
	}
	return store, nil
}

func (s *Service) CreateRegularRun(ctx context.Context, actor Actor, in CreateRegularRunInput) (ExecutionRun, error) {
	if err := validateActor(actor); err != nil {
		return ExecutionRun{}, err
	}
	in.CommandID = strings.ToLower(strings.TrimSpace(in.CommandID))
	in.Period = strings.TrimSpace(in.Period)
	in.Label = strings.TrimSpace(in.Label)
	in.PlannedPayDate = strings.TrimSpace(in.PlannedPayDate)
	in.ContributionSchemeID = strings.ToLower(strings.TrimSpace(in.ContributionSchemeID))
	in.ReplacesRunID = strings.ToLower(strings.TrimSpace(in.ReplacesRunID))
	if _, dateErr := time.Parse("2006-01-02", in.PlannedPayDate); !uuidPattern.MatchString(in.CommandID) || !periodPattern.MatchString(in.Period) || in.Label == "" || len([]rune(in.Label)) > 200 || !datePattern.MatchString(in.PlannedPayDate) || dateErr != nil || !uuidPattern.MatchString(in.ContributionSchemeID) || in.ContributionSchemeRevision < 1 || len(in.PersonIDs) == 0 || (in.ReplacesRunID != "" && (!uuidPattern.MatchString(in.ReplacesRunID) || in.ReplacesRunRevision < 1)) {
		return ExecutionRun{}, fmt.Errorf("invalid regular payroll run")
	}
	seen := map[string]bool{}
	people := make([]PersonReference, 0, len(in.PersonIDs))
	for _, raw := range in.PersonIDs {
		id := strings.ToLower(strings.TrimSpace(raw))
		if !uuidPattern.MatchString(id) || seen[id] {
			return ExecutionRun{}, fmt.Errorf("invalid payroll person selection")
		}
		seen[id] = true
		person, err := s.validatePerson(ctx, actor.TenantID, id)
		if err != nil {
			return ExecutionRun{}, err
		}
		people = append(people, person)
	}
	sort.Slice(people, func(i, j int) bool { return people[i].ID < people[j].ID })
	in.PersonIDs = in.PersonIDs[:0]
	for _, p := range people {
		in.PersonIDs = append(in.PersonIDs, p.ID)
	}
	store, err := s.executionStore()
	if err != nil {
		return ExecutionRun{}, err
	}
	return store.CreateRegularRun(ctx, actor, in, people, hash(in))
}

func (s *Service) CreateAdjustmentRun(ctx context.Context, actor Actor, in CreateAdjustmentRunInput) (ExecutionRun, error) {
	if err := validateActor(actor); err != nil {
		return ExecutionRun{}, err
	}
	in.CommandID, in.Period, in.Label, in.PlannedPayDate = strings.ToLower(strings.TrimSpace(in.CommandID)), strings.TrimSpace(in.Period), strings.TrimSpace(in.Label), strings.TrimSpace(in.PlannedPayDate)
	if _, dateErr := time.Parse("2006-01-02", in.PlannedPayDate); !uuidPattern.MatchString(in.CommandID) || !periodPattern.MatchString(in.Period) || in.Label == "" || len([]rune(in.Label)) > 200 || !datePattern.MatchString(in.PlannedPayDate) || dateErr != nil || len(in.Items) == 0 || len(in.Items) > 1000 {
		return ExecutionRun{}, fmt.Errorf("invalid payroll adjustment run")
	}
	seen := map[string]bool{}
	for index := range in.Items {
		item := &in.Items[index]
		item.SourcePayrollLineID = strings.ToLower(strings.TrimSpace(item.SourcePayrollLineID))
		item.Direction, item.Reason = strings.ToLower(strings.TrimSpace(item.Direction)), strings.TrimSpace(item.Reason)
		if !uuidPattern.MatchString(item.SourcePayrollLineID) || item.SourcePayrollLineRevision < 1 || (item.Direction != "supplement" && item.Direction != "recovery") || item.AmountMinor <= 0 || item.AmountMinor > maxPayrollMinor || item.Reason == "" || len([]rune(item.Reason)) > 500 || seen[item.SourcePayrollLineID] {
			return ExecutionRun{}, fmt.Errorf("invalid payroll adjustment item")
		}
		seen[item.SourcePayrollLineID] = true
	}
	store, err := s.executionStore()
	if err != nil {
		return ExecutionRun{}, err
	}
	return store.CreateAdjustmentRun(ctx, actor, in, hash(in))
}

func (s *Service) GetExecutionRun(ctx context.Context, tenant, id string) (ExecutionRun, error) {
	store, err := s.executionStore()
	if err != nil {
		return ExecutionRun{}, err
	}
	return store.GetExecutionRun(ctx, tenant, strings.ToLower(strings.TrimSpace(id)))
}
func (s *Service) ListExecutionRuns(ctx context.Context, tenant string) ([]ExecutionRun, error) {
	store, err := s.executionStore()
	if err != nil {
		return nil, err
	}
	return store.ListExecutionRuns(ctx, tenant)
}

func (s *Service) SyncMembers(ctx context.Context, actor Actor, runID string, in SyncMembersInput) (ExecutionRun, error) {
	if err := validateActor(actor); err != nil {
		return ExecutionRun{}, err
	}
	runID = strings.ToLower(strings.TrimSpace(runID))
	in.CommandID = strings.ToLower(strings.TrimSpace(in.CommandID))
	in.Reason = strings.TrimSpace(in.Reason)
	if !uuidPattern.MatchString(runID) || !uuidPattern.MatchString(in.CommandID) || in.ExpectedRevision < 1 || len(in.PersonIDs) == 0 || len(in.PersonIDs) > 1000 || in.Reason == "" || len([]rune(in.Reason)) > 500 {
		return ExecutionRun{}, fmt.Errorf("invalid payroll member synchronization")
	}
	seen := map[string]bool{}
	people := make([]PersonReference, 0, len(in.PersonIDs))
	for _, raw := range in.PersonIDs {
		id := strings.ToLower(strings.TrimSpace(raw))
		if !uuidPattern.MatchString(id) || seen[id] {
			return ExecutionRun{}, fmt.Errorf("invalid payroll member synchronization")
		}
		seen[id] = true
		person, err := s.validatePerson(ctx, actor.TenantID, id)
		if err != nil {
			return ExecutionRun{}, err
		}
		people = append(people, person)
	}
	sort.Slice(people, func(i, j int) bool { return people[i].ID < people[j].ID })
	for index := range people {
		in.PersonIDs[index] = people[index].ID
	}
	store, err := s.executionStore()
	if err != nil {
		return ExecutionRun{}, err
	}
	return store.SyncMembers(ctx, actor, runID, in, people, hash(struct {
		RunID string
		Input SyncMembersInput
	}{runID, in}))
}

func normalizeSnapshotSync(runID, lineID string, in ConfirmSnapshotSyncInput) (string, string, ConfirmSnapshotSyncInput, error) {
	runID, lineID = strings.ToLower(strings.TrimSpace(runID)), strings.ToLower(strings.TrimSpace(lineID))
	in.CommandID, in.PreviewHash, in.Reason = strings.ToLower(strings.TrimSpace(in.CommandID)), strings.ToLower(strings.TrimSpace(in.PreviewHash)), strings.TrimSpace(in.Reason)
	if !uuidPattern.MatchString(runID) || !uuidPattern.MatchString(lineID) || !uuidPattern.MatchString(in.CommandID) || in.ExpectedRunRevision < 1 || in.ExpectedLineRevision < 1 || len(in.PreviewHash) != 64 || in.Reason == "" || len([]rune(in.Reason)) > 500 {
		return runID, lineID, in, fmt.Errorf("invalid payroll snapshot synchronization")
	}
	return runID, lineID, in, nil
}
func (s *Service) PreviewCompensationSync(ctx context.Context, actor Actor, runID, lineID string) (SnapshotSyncPreview, error) {
	if err := validateActor(actor); err != nil {
		return SnapshotSyncPreview{}, err
	}
	store, err := s.executionStore()
	if err != nil {
		return SnapshotSyncPreview{}, err
	}
	return store.PreviewCompensationSync(ctx, actor.TenantID, strings.ToLower(strings.TrimSpace(runID)), strings.ToLower(strings.TrimSpace(lineID)))
}
func (s *Service) ConfirmCompensationSync(ctx context.Context, actor Actor, runID, lineID string, in ConfirmSnapshotSyncInput) (ExecutionRun, error) {
	if err := validateActor(actor); err != nil {
		return ExecutionRun{}, err
	}
	var err error
	if runID, lineID, in, err = normalizeSnapshotSync(runID, lineID, in); err != nil {
		return ExecutionRun{}, err
	}
	store, err := s.executionStore()
	if err != nil {
		return ExecutionRun{}, err
	}
	return store.ConfirmCompensationSync(ctx, actor, runID, lineID, in, hash(struct {
		RunID, LineID string
		Input         ConfirmSnapshotSyncInput
	}{runID, lineID, in}))
}
func (s *Service) PreviewPaymentProfileSync(ctx context.Context, actor Actor, runID, lineID string) (SnapshotSyncPreview, error) {
	if err := validateActor(actor); err != nil {
		return SnapshotSyncPreview{}, err
	}
	store, err := s.executionStore()
	if err != nil {
		return SnapshotSyncPreview{}, err
	}
	return store.PreviewPaymentProfileSync(ctx, actor.TenantID, strings.ToLower(strings.TrimSpace(runID)), strings.ToLower(strings.TrimSpace(lineID)))
}
func (s *Service) ConfirmPaymentProfileSync(ctx context.Context, actor Actor, runID, lineID string, in ConfirmSnapshotSyncInput) (ExecutionRun, error) {
	if err := validateActor(actor); err != nil {
		return ExecutionRun{}, err
	}
	var err error
	if runID, lineID, in, err = normalizeSnapshotSync(runID, lineID, in); err != nil {
		return ExecutionRun{}, err
	}
	store, err := s.executionStore()
	if err != nil {
		return ExecutionRun{}, err
	}
	return store.ConfirmPaymentProfileSync(ctx, actor, runID, lineID, in, hash(struct {
		RunID, LineID string
		Input         ConfirmSnapshotSyncInput
	}{runID, lineID, in}))
}

func normalizeAdjustments(values []Adjustment) ([]Adjustment, error) {
	result := make([]Adjustment, 0, len(values))
	seen := map[string]bool{}
	for _, v := range values {
		v.ID = strings.ToLower(strings.TrimSpace(v.ID))
		v.Kind = strings.ToLower(strings.TrimSpace(v.Kind))
		v.Reason = strings.TrimSpace(v.Reason)
		if v.ID == "" {
			v.ID = newUUID()
		}
		if !uuidPattern.MatchString(v.ID) || seen[v.ID] || (v.Kind != "supplement" && v.Kind != "recovery" && v.Kind != "manual") || v.AmountMinor == 0 || v.Reason == "" || len([]rune(v.Reason)) > 500 {
			return nil, fmt.Errorf("invalid payroll adjustment")
		}
		seen[v.ID] = true
		result = append(result, v)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (s *Service) RevisePayrollLine(ctx context.Context, actor Actor, runID, lineID string, in ReviseLineInput) (ExecutionRun, error) {
	if err := validateActor(actor); err != nil {
		return ExecutionRun{}, err
	}
	runID = strings.ToLower(strings.TrimSpace(runID))
	lineID = strings.ToLower(strings.TrimSpace(lineID))
	in.CommandID = strings.ToLower(strings.TrimSpace(in.CommandID))
	in.TaxSourceKind = strings.ToLower(strings.TrimSpace(in.TaxSourceKind))
	in.TaxSourceNote = strings.TrimSpace(in.TaxSourceNote)
	in.Reason = strings.TrimSpace(in.Reason)
	adjustments, err := normalizeAdjustments(in.Adjustments)
	if err != nil {
		return ExecutionRun{}, err
	}
	in.Adjustments = adjustments
	if !uuidPattern.MatchString(runID) || !uuidPattern.MatchString(lineID) || !uuidPattern.MatchString(in.CommandID) || in.ExpectedRunRevision < 1 || in.ExpectedLineRevision < 1 || in.PerformanceMinor < 0 || in.PerformanceMinor > maxPayrollMinor || in.TaxMinor < 0 || in.TaxMinor > maxPayrollMinor || (in.TaxSourceKind != "manual" && in.TaxSourceKind != "import") || in.Reason == "" || len([]rune(in.Reason)) > 500 {
		return ExecutionRun{}, fmt.Errorf("invalid payroll line revision")
	}
	store, err := s.executionStore()
	if err != nil {
		return ExecutionRun{}, err
	}
	return store.RevisePayrollLine(ctx, actor, runID, lineID, in, hash(struct {
		RunID, LineID string
		Input         ReviseLineInput
	}{runID, lineID, in}))
}

func (s *Service) ApplyAttendanceSummary(ctx context.Context, actor Actor, runID, lineID string, in ApplyAttendanceSummaryInput) (ExecutionRun, error) {
	if err := validateActor(actor); err != nil {
		return ExecutionRun{}, err
	}
	runID, lineID = strings.ToLower(strings.TrimSpace(runID)), strings.ToLower(strings.TrimSpace(lineID))
	in.CommandID, in.SummaryID, in.Reason = strings.ToLower(strings.TrimSpace(in.CommandID)), strings.ToLower(strings.TrimSpace(in.SummaryID)), strings.TrimSpace(in.Reason)
	if !uuidPattern.MatchString(runID) || !uuidPattern.MatchString(lineID) || !uuidPattern.MatchString(in.CommandID) || !uuidPattern.MatchString(in.SummaryID) || in.ExpectedRunRevision < 1 || in.ExpectedLineRevision < 1 || in.Reason == "" || len([]rune(in.Reason)) > 500 || s.attendance == nil {
		return ExecutionRun{}, fmt.Errorf("invalid attendance summary application")
	}
	summary, err := s.attendance.GetMonthlySummary(ctx, actor.TenantID, in.SummaryID)
	if err != nil {
		return ExecutionRun{}, err
	}
	if err = attendancecontract.Validate(summary); err != nil || summary.TenantID != actor.TenantID {
		return ExecutionRun{}, fmt.Errorf("invalid attendance summary")
	}
	store, err := s.executionStore()
	if err != nil {
		return ExecutionRun{}, err
	}
	return store.ApplyAttendanceSummary(ctx, actor, runID, lineID, in, summary, hash(struct {
		RunID, LineID string
		Input         ApplyAttendanceSummaryInput
		Summary       attendancecontract.MonthlySummary
	}{runID, lineID, in, summary}))
}

func normalizeRunCommand(in RunCommandInput) (RunCommandInput, error) {
	in.CommandID = strings.ToLower(strings.TrimSpace(in.CommandID))
	in.Reason = strings.TrimSpace(in.Reason)
	if !uuidPattern.MatchString(in.CommandID) || in.ExpectedRevision < 1 || in.Reason == "" || len([]rune(in.Reason)) > 500 {
		return in, fmt.Errorf("invalid payroll run command")
	}
	return in, nil
}
func (s *Service) ReviewPayrollRun(ctx context.Context, actor Actor, runID string, in RunCommandInput) (ExecutionRun, error) {
	if err := validateActor(actor); err != nil {
		return ExecutionRun{}, err
	}
	var err error
	if in, err = normalizeRunCommand(in); err != nil {
		return ExecutionRun{}, err
	}
	runID = strings.ToLower(strings.TrimSpace(runID))
	if !uuidPattern.MatchString(runID) {
		return ExecutionRun{}, fmt.Errorf("invalid payroll run")
	}
	store, err := s.executionStore()
	if err != nil {
		return ExecutionRun{}, err
	}
	return store.ReviewPayrollRun(ctx, actor, runID, in, hash(struct {
		RunID string
		Input RunCommandInput
	}{runID, in}))
}
func (s *Service) VoidPayrollRun(ctx context.Context, actor Actor, runID string, in RunCommandInput) (ExecutionRun, error) {
	if err := validateActor(actor); err != nil {
		return ExecutionRun{}, err
	}
	var err error
	if in, err = normalizeRunCommand(in); err != nil {
		return ExecutionRun{}, err
	}
	runID = strings.ToLower(strings.TrimSpace(runID))
	if !uuidPattern.MatchString(runID) {
		return ExecutionRun{}, fmt.Errorf("invalid payroll run")
	}
	store, err := s.executionStore()
	if err != nil {
		return ExecutionRun{}, err
	}
	return store.VoidPayrollRun(ctx, actor, runID, in, hash(struct {
		RunID string
		Input RunCommandInput
	}{runID, in}))
}
func (s *Service) ConfirmZeroLine(ctx context.Context, actor Actor, runID, lineID string, in LineCommandInput) (ExecutionRun, error) {
	if err := validateActor(actor); err != nil {
		return ExecutionRun{}, err
	}
	runID = strings.ToLower(strings.TrimSpace(runID))
	lineID = strings.ToLower(strings.TrimSpace(lineID))
	in.CommandID = strings.ToLower(strings.TrimSpace(in.CommandID))
	in.Reason = strings.TrimSpace(in.Reason)
	if !uuidPattern.MatchString(runID) || !uuidPattern.MatchString(lineID) || !uuidPattern.MatchString(in.CommandID) || in.ExpectedRunRevision < 1 || in.ExpectedLineRevision < 1 || in.Reason == "" {
		return ExecutionRun{}, fmt.Errorf("invalid zero payment confirmation")
	}
	store, err := s.executionStore()
	if err != nil {
		return ExecutionRun{}, err
	}
	return store.ConfirmZeroLine(ctx, actor, runID, lineID, in, hash(struct {
		RunID, LineID string
		Input         LineCommandInput
	}{runID, lineID, in}))
}

func (s *Service) RecordPayment(ctx context.Context, actor Actor, runID, lineID string, in RecordPaymentInput) (ExecutionRun, error) {
	if err := validateActor(actor); err != nil {
		return ExecutionRun{}, err
	}
	if s.paymentDelivery == nil {
		return ExecutionRun{}, ErrReferenceUnavailable
	}
	runID = strings.ToLower(strings.TrimSpace(runID))
	lineID = strings.ToLower(strings.TrimSpace(lineID))
	in.CommandID = strings.ToLower(strings.TrimSpace(in.CommandID))
	in.OccurredOn = strings.TrimSpace(in.OccurredOn)
	in.Note = strings.TrimSpace(in.Note)
	in.OverageReason = strings.TrimSpace(in.OverageReason)
	if _, dateErr := time.Parse("2006-01-02", in.OccurredOn); !uuidPattern.MatchString(runID) || !uuidPattern.MatchString(lineID) || !uuidPattern.MatchString(in.CommandID) || in.ExpectedRunRevision < 1 || in.ExpectedLineRevision < 1 || in.ActualAmountMinor <= 0 || !datePattern.MatchString(in.OccurredOn) || dateErr != nil || len([]rune(in.Note)) > 500 {
		return ExecutionRun{}, fmt.Errorf("invalid payroll payment")
	}
	store, err := s.executionStore()
	if err != nil {
		return ExecutionRun{}, err
	}
	prepared, intent, err := store.PreparePaymentDelivery(ctx, actor, runID, lineID, in, hash(struct {
		RunID, LineID string
		Input         RecordPaymentInput
	}{runID, lineID, in}))
	if err != nil {
		return ExecutionRun{}, err
	}
	if prepared.Status == "succeeded" {
		return store.GetExecutionRun(ctx, actor.TenantID, runID)
	}
	result, deliveryErr := s.paymentDelivery.Deliver(ctx, intent)
	if deliveryErr != nil {
		if errors.Is(deliveryErr, ErrPaymentUnknown) {
			result.Status = "unknown"
		} else {
			result.Status = "failed"
			result.FailureCode = "PAYMENT_DELIVERY_FAILED"
		}
	}
	value, finishErr := store.FinishPaymentDelivery(ctx, actor, prepared, result)
	if finishErr != nil {
		return ExecutionRun{}, finishErr
	}
	if deliveryErr != nil {
		return value, deliveryErr
	}
	return value, nil
}

func calculateContribution(base, floor, cap, rate int64, mode string) (int64, error) {
	if base < floor {
		base = floor
	}
	if cap > 0 && base > cap {
		base = cap
	}
	if base < 0 || base > maxPayrollMinor || rate < 0 || rate > 10000 || (rate != 0 && base > math.MaxInt64/rate) {
		return 0, fmt.Errorf("invalid payroll contribution")
	}
	product := base * rate
	switch mode {
	case "yuan_half_up":
		return ((product + 500000) / 1000000) * 100, nil
	case "down":
		return product / 10000, nil
	case "up":
		if product == 0 {
			return 0, nil
		}
		return (product + 9999) / 10000, nil
	default:
		return (product + 5000) / 10000, nil
	}
}
func addMinor(left, right int64) (int64, error) {
	if (right > 0 && left > math.MaxInt64-right) || (right < 0 && left < math.MinInt64-right) {
		return 0, fmt.Errorf("payroll amount overflow")
	}
	return left + right, nil
}
func CalculatePayrollLine(fixed, socialBase, housingBase, performance, tax int64, adjustments []Adjustment, components []ContributionComponent) (int64, []ContributionAmount, int64, int64, int64, int64, int64, error) {
	if fixed < 0 || fixed > maxPayrollMinor || socialBase < 0 || socialBase > maxPayrollMinor || housingBase < 0 || housingBase > maxPayrollMinor || performance < 0 || performance > maxPayrollMinor || tax < 0 || tax > maxPayrollMinor || len(adjustments) > 1000 {
		return 0, nil, 0, 0, 0, 0, 0, fmt.Errorf("invalid payroll amounts")
	}
	gross, err := addMinor(fixed, performance)
	if err != nil {
		return 0, nil, 0, 0, 0, 0, 0, err
	}
	for _, a := range adjustments {
		if a.AmountMinor < -maxPayrollMinor || a.AmountMinor > maxPayrollMinor {
			return 0, nil, 0, 0, 0, 0, 0, fmt.Errorf("invalid payroll adjustment amount")
		}
		gross, err = addMinor(gross, a.AmountMinor)
		if err != nil {
			return 0, nil, 0, 0, 0, 0, 0, err
		}
	}
	contributions := make([]ContributionAmount, 0, len(components))
	var employee, employer int64
	for _, c := range components {
		base := socialBase
		if c.BaseKind == "housing" {
			base = housingBase
		}
		individual, contributionErr := calculateContribution(base, c.BaseFloorMinor, c.BaseCapMinor, c.IndividualRateBPS, c.RoundingMode)
		if contributionErr != nil {
			return 0, nil, 0, 0, 0, 0, 0, contributionErr
		}
		individual, contributionErr = addMinor(individual, c.IndividualFixedMinor)
		if contributionErr != nil || individual > maxPayrollMinor {
			return 0, nil, 0, 0, 0, 0, 0, fmt.Errorf("invalid payroll contribution")
		}
		company, contributionErr := calculateContribution(base, c.BaseFloorMinor, c.BaseCapMinor, c.EmployerRateBPS, c.RoundingMode)
		if contributionErr != nil {
			return 0, nil, 0, 0, 0, 0, 0, contributionErr
		}
		company, contributionErr = addMinor(company, c.EmployerFixedMinor)
		if contributionErr != nil || company > maxPayrollMinor {
			return 0, nil, 0, 0, 0, 0, 0, fmt.Errorf("invalid payroll contribution")
		}
		contributions = append(contributions, ContributionAmount{Key: c.Key, BaseMinor: max(c.BaseFloorMinor, minNonzeroCap(base, c.BaseCapMinor)), IndividualRateBPS: c.IndividualRateBPS, EmployerRateBPS: c.EmployerRateBPS, IndividualFixedMinor: c.IndividualFixedMinor, EmployerFixedMinor: c.EmployerFixedMinor, IndividualMinor: individual, EmployerMinor: company, RoundingMode: c.RoundingMode})
		employee, err = addMinor(employee, individual)
		if err != nil {
			return 0, nil, 0, 0, 0, 0, 0, err
		}
		employer, err = addMinor(employer, company)
		if err != nil {
			return 0, nil, 0, 0, 0, 0, 0, err
		}
	}
	preTax, err := addMinor(gross, -employee)
	if err != nil {
		return 0, nil, 0, 0, 0, 0, 0, err
	}
	net, err := addMinor(preTax, -tax)
	if err != nil {
		return 0, nil, 0, 0, 0, 0, 0, err
	}
	cost, err := addMinor(gross, employer)
	if err != nil {
		return 0, nil, 0, 0, 0, 0, 0, err
	}
	return gross, contributions, employee, employer, preTax, net, cost, nil
}
func minNonzeroCap(value, cap int64) int64 {
	if cap > 0 && value > cap {
		return cap
	}
	return value
}
