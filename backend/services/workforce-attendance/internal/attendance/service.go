package attendance

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	contract "omsbizfoundry/attendance-summary-contract"
)

var (
	ErrInvalid             = errors.New("attendance input is invalid")
	ErrNotFound            = errors.New("attendance resource not found")
	ErrConflict            = errors.New("attendance revision conflict")
	ErrIdempotencyConflict = errors.New("attendance idempotency conflict")
	ErrDependency          = errors.New("attendance dependency unavailable")
)

type Actor struct {
	TenantID, SubjectID, DisplayName, CredentialID string
}

type PolicyRecord struct {
	TenantID       string    `json:"tenant_id"`
	Key            string    `json:"key"`
	Period         string    `json:"period"`
	Revision       int64     `json:"revision"`
	Basis          string    `json:"basis"`
	ScheduledDates []string  `json:"scheduled_dates,omitempty"`
	Reason         string    `json:"reason"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type UpsertPolicyInput struct {
	CommandID        string   `json:"command_id"`
	ExpectedRevision int64    `json:"expected_revision"`
	Key              string   `json:"key"`
	Period           string   `json:"period"`
	Basis            string   `json:"basis"`
	ScheduledDates   []string `json:"scheduled_dates,omitempty"`
	Reason           string   `json:"reason"`
}

type AttendanceRecord struct {
	ID, TenantID, PersonID, WorkDate, Status, Note string
	Units, Revision                                int64
	UpdatedAt                                      time.Time
}

type AttendanceRecordView struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	PersonID  string    `json:"person_id"`
	WorkDate  string    `json:"work_date"`
	Status    string    `json:"status"`
	Units     int64     `json:"units"`
	Note      string    `json:"note,omitempty"`
	Revision  int64     `json:"revision"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RecordAttendanceInput struct {
	CommandID        string `json:"command_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	PersonID         string `json:"person_id"`
	WorkDate         string `json:"work_date"`
	Status           string `json:"status"`
	Units            int64  `json:"units"`
	Note             string `json:"note,omitempty"`
}

type Leave struct {
	ID, TenantID, PersonID, From, Through, LeaveType, Reason string
	UnitsPerDay, PaidRatioBPS, Revision                      int64
	UpdatedAt                                                time.Time
}

type LeaveView struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	PersonID     string    `json:"person_id"`
	From         string    `json:"from"`
	Through      string    `json:"through"`
	LeaveType    string    `json:"leave_type"`
	UnitsPerDay  int64     `json:"units_per_day"`
	PaidRatioBPS int64     `json:"paid_ratio_bps"`
	Reason       string    `json:"reason"`
	Revision     int64     `json:"revision"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type RecordLeaveInput struct {
	CommandID        string `json:"command_id"`
	LeaveID          string `json:"leave_id,omitempty"`
	ExpectedRevision int64  `json:"expected_revision"`
	PersonID         string `json:"person_id"`
	From             string `json:"from"`
	Through          string `json:"through"`
	LeaveType        string `json:"leave_type"`
	UnitsPerDay      int64  `json:"units_per_day"`
	PaidRatioBPS     int64  `json:"paid_ratio_bps"`
	Reason           string `json:"reason"`
}

type CalculateSummaryInput struct {
	CommandID string `json:"command_id"`
	PersonID  string `json:"person_id"`
	Period    string `json:"period"`
	PolicyKey string `json:"policy_key"`
}

type Store interface {
	Ping(context.Context) error
	UpsertPolicy(context.Context, Actor, UpsertPolicyInput, string) (PolicyRecord, error)
	GetPolicy(context.Context, string, string) (PolicyRecord, error)
	ListPolicies(context.Context, string) ([]PolicyRecord, error)
	RecordAttendance(context.Context, Actor, RecordAttendanceInput, string) (AttendanceRecord, error)
	ListAttendance(context.Context, string, string, string) ([]AttendanceRecord, error)
	RecordLeave(context.Context, Actor, RecordLeaveInput, string) (Leave, error)
	ListLeaves(context.Context, string, string, string) ([]Leave, error)
	SaveSummary(context.Context, Actor, string, contract.MonthlySummary, string) (contract.MonthlySummary, error)
	GetSummary(context.Context, string, string) (contract.MonthlySummary, error)
	ListSummaries(context.Context, string, string) ([]contract.MonthlySummary, error)
}

type EmploymentPort interface {
	Snapshot(context.Context, string, string) (EmploymentSnapshot, error)
}

type Service struct {
	store  Store
	people EmploymentPort
	now    func() time.Time
}

func NewService(store Store, people EmploymentPort) (*Service, error) {
	if store == nil || people == nil {
		return nil, fmt.Errorf("attendance store and People port are required")
	}
	return &Service{store: store, people: people, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (s *Service) Ping(ctx context.Context) error { return s.store.Ping(ctx) }

func (s *Service) UpsertPolicy(ctx context.Context, actor Actor, input UpsertPolicyInput) (PolicyRecord, error) {
	input.CommandID, input.Key, input.Basis, input.Reason = clean(input.CommandID), strings.TrimSpace(input.Key), clean(input.Basis), strings.TrimSpace(input.Reason)
	if validateActor(actor) != nil || !uuidPattern.MatchString(input.CommandID) || input.ExpectedRevision < 0 || input.Key == "" || len(input.Key) > 80 || input.Key != strings.TrimSpace(input.Key) || !periodPattern.MatchString(input.Period) || (input.Basis != "calendar_days" && input.Basis != "scheduled_workdays") || input.Reason == "" || len([]rune(input.Reason)) > 500 {
		return PolicyRecord{}, ErrInvalid
	}
	if err := validatePolicySchedule(input.Period, input.Basis, input.ScheduledDates); err != nil {
		return PolicyRecord{}, ErrInvalid
	}
	if input.ScheduledDates == nil {
		input.ScheduledDates = []string{}
	}
	return s.store.UpsertPolicy(ctx, actor, input, intentHash(input))
}

func validatePolicySchedule(period, basis string, dates []string) error {
	if basis == "calendar_days" {
		if len(dates) != 0 {
			return ErrInvalid
		}
		return nil
	}
	if len(dates) == 0 {
		return ErrInvalid
	}
	seen := map[string]bool{}
	for _, value := range dates {
		if !validDate(value) || seen[value] {
			return ErrInvalid
		}
		seen[value] = true
		if value[:7] != period {
			return ErrInvalid
		}
	}
	return nil
}

func (s *Service) GetPolicy(ctx context.Context, tenantID, key string) (PolicyRecord, error) {
	return s.store.GetPolicy(ctx, clean(tenantID), strings.TrimSpace(key))
}
func (s *Service) ListPolicies(ctx context.Context, tenantID string) ([]PolicyRecord, error) {
	return s.store.ListPolicies(ctx, clean(tenantID))
}

func (s *Service) RecordAttendance(ctx context.Context, actor Actor, input RecordAttendanceInput) (AttendanceRecordView, error) {
	input.CommandID, input.PersonID, input.Status, input.Note = clean(input.CommandID), clean(input.PersonID), clean(input.Status), strings.TrimSpace(input.Note)
	if validateActor(actor) != nil || !uuidPattern.MatchString(input.CommandID) || !uuidPattern.MatchString(input.PersonID) || input.ExpectedRevision < 0 || !validDate(input.WorkDate) || (input.Status != "present" && input.Status != "absent") || input.Units < 1 || input.Units > contract.FullDayUnits || len([]rune(input.Note)) > 500 {
		return AttendanceRecordView{}, ErrInvalid
	}
	value, err := s.store.RecordAttendance(ctx, actor, input, intentHash(input))
	return attendanceView(value), err
}

func (s *Service) ListAttendance(ctx context.Context, tenantID, personID, period string) ([]AttendanceRecordView, error) {
	if !uuidPattern.MatchString(clean(tenantID)) || (personID != "" && !uuidPattern.MatchString(clean(personID))) || !periodPattern.MatchString(period) {
		return nil, ErrInvalid
	}
	values, err := s.store.ListAttendance(ctx, clean(tenantID), clean(personID), period)
	result := make([]AttendanceRecordView, 0, len(values))
	for _, value := range values {
		result = append(result, attendanceView(value))
	}
	return result, err
}

func (s *Service) RecordLeave(ctx context.Context, actor Actor, input RecordLeaveInput) (LeaveView, error) {
	input.CommandID, input.LeaveID, input.PersonID, input.LeaveType, input.Reason = clean(input.CommandID), clean(input.LeaveID), clean(input.PersonID), clean(input.LeaveType), strings.TrimSpace(input.Reason)
	from, fromErr := time.Parse("2006-01-02", input.From)
	through, throughErr := time.Parse("2006-01-02", input.Through)
	if validateActor(actor) != nil || !uuidPattern.MatchString(input.CommandID) || (input.LeaveID != "" && !uuidPattern.MatchString(input.LeaveID)) || !uuidPattern.MatchString(input.PersonID) || input.ExpectedRevision < 0 || (input.LeaveID == "" && input.ExpectedRevision != 0) || (input.LeaveID != "" && input.ExpectedRevision < 1) || fromErr != nil || throughErr != nil || through.Before(from) || input.LeaveType == "" || len(input.LeaveType) > 40 || input.UnitsPerDay < 1 || input.UnitsPerDay > contract.FullDayUnits || input.PaidRatioBPS < 0 || input.PaidRatioBPS > 10000 || input.Reason == "" || len([]rune(input.Reason)) > 500 {
		return LeaveView{}, ErrInvalid
	}
	value, err := s.store.RecordLeave(ctx, actor, input, intentHash(input))
	return leaveView(value), err
}

func (s *Service) ListLeaves(ctx context.Context, tenantID, personID, period string) ([]LeaveView, error) {
	if !uuidPattern.MatchString(clean(tenantID)) || (personID != "" && !uuidPattern.MatchString(clean(personID))) || !periodPattern.MatchString(period) {
		return nil, ErrInvalid
	}
	values, err := s.store.ListLeaves(ctx, clean(tenantID), clean(personID), period)
	result := make([]LeaveView, 0, len(values))
	for _, value := range values {
		result = append(result, leaveView(value))
	}
	return result, err
}

func (s *Service) CalculateSummary(ctx context.Context, actor Actor, input CalculateSummaryInput) (contract.MonthlySummary, error) {
	input.CommandID, input.PersonID, input.PolicyKey = clean(input.CommandID), clean(input.PersonID), strings.TrimSpace(input.PolicyKey)
	if validateActor(actor) != nil || !uuidPattern.MatchString(input.CommandID) || !uuidPattern.MatchString(input.PersonID) || !periodPattern.MatchString(input.Period) || input.PolicyKey == "" {
		return contract.MonthlySummary{}, ErrInvalid
	}
	policy, err := s.store.GetPolicy(ctx, actor.TenantID, input.PolicyKey)
	if err != nil {
		return contract.MonthlySummary{}, err
	}
	if policy.Period != input.Period {
		return contract.MonthlySummary{}, ErrConflict
	}
	employment, err := s.people.Snapshot(ctx, actor.TenantID, input.PersonID)
	if err != nil {
		return contract.MonthlySummary{}, ErrDependency
	}
	attendance, err := s.store.ListAttendance(ctx, actor.TenantID, input.PersonID, input.Period)
	if err != nil {
		return contract.MonthlySummary{}, err
	}
	leaves, err := s.store.ListLeaves(ctx, actor.TenantID, input.PersonID, input.Period)
	if err != nil {
		return contract.MonthlySummary{}, err
	}
	absences := make([]AbsenceRecord, 0)
	for _, value := range attendance {
		if value.Status == "absent" {
			absences = append(absences, AbsenceRecord{ID: value.ID, Date: value.WorkDate, Units: value.Units})
		}
	}
	leaveInputs := make([]LeaveRecord, 0, len(leaves))
	for _, value := range leaves {
		leaveInputs = append(leaveInputs, LeaveRecord{ID: value.ID, From: value.From, Through: value.Through, UnitsPerDay: value.UnitsPerDay, PaidRatioBPS: value.PaidRatioBPS})
	}
	summaryID, err := newUUID()
	if err != nil {
		return contract.MonthlySummary{}, err
	}
	summary, err := Calculate(CalculateInput{SummaryID: summaryID, TenantID: actor.TenantID, Period: input.Period, Policy: Policy{Key: policy.Key, Revision: policy.Revision, Basis: policy.Basis, ScheduledDates: policy.ScheduledDates}, Employment: employment, Leaves: leaveInputs, Absences: absences, SourceRevision: 1})
	if err != nil {
		return contract.MonthlySummary{}, err
	}
	return s.store.SaveSummary(ctx, actor, input.CommandID, summary, intentHash(input))
}

func (s *Service) GetSummary(ctx context.Context, tenantID, id string) (contract.MonthlySummary, error) {
	return s.store.GetSummary(ctx, clean(tenantID), clean(id))
}
func (s *Service) ListSummaries(ctx context.Context, tenantID, period string) ([]contract.MonthlySummary, error) {
	if !periodPattern.MatchString(period) {
		return nil, ErrInvalid
	}
	return s.store.ListSummaries(ctx, clean(tenantID), period)
}

func validateActor(actor Actor) error {
	if !uuidPattern.MatchString(clean(actor.TenantID)) || !uuidPattern.MatchString(clean(actor.SubjectID)) || strings.TrimSpace(actor.DisplayName) == "" || strings.TrimSpace(actor.CredentialID) == "" {
		return ErrInvalid
	}
	return nil
}
func clean(value string) string   { return strings.ToLower(strings.TrimSpace(value)) }
func validDate(value string) bool { _, err := time.Parse("2006-01-02", value); return err == nil }
func intentHash(value any) string {
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
func attendanceView(value AttendanceRecord) AttendanceRecordView {
	return AttendanceRecordView{ID: value.ID, TenantID: value.TenantID, PersonID: value.PersonID, WorkDate: value.WorkDate, Status: value.Status, Units: value.Units, Note: value.Note, Revision: value.Revision, UpdatedAt: value.UpdatedAt}
}
func leaveView(value Leave) LeaveView {
	return LeaveView{ID: value.ID, TenantID: value.TenantID, PersonID: value.PersonID, From: value.From, Through: value.Through, LeaveType: value.LeaveType, UnitsPerDay: value.UnitsPerDay, PaidRatioBPS: value.PaidRatioBPS, Reason: value.Reason, Revision: value.Revision, UpdatedAt: value.UpdatedAt}
}
func newUUID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16]), nil
}
