package attendance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	contract "omsbizfoundry/attendance-summary-contract"
)

var (
	uuidPattern   = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	periodPattern = regexp.MustCompile(`^[0-9]{4}-(0[1-9]|1[0-2])$`)
)

type Policy struct {
	Key            string   `json:"key"`
	Revision       int64    `json:"revision"`
	Basis          string   `json:"basis"`
	ScheduledDates []string `json:"scheduled_dates,omitempty"`
}

type EmploymentSnapshot struct {
	PersonID       string               `json:"person_id"`
	DisplayName    string               `json:"display_name"`
	SourceRevision int64                `json:"source_revision"`
	SourceDigest   string               `json:"source_digest"`
	Intervals      []EmploymentInterval `json:"intervals"`
}

type EmploymentInterval struct {
	From    string `json:"from"`
	Through string `json:"through,omitempty"`
}

type LeaveRecord struct {
	ID           string `json:"id"`
	From         string `json:"from"`
	Through      string `json:"through"`
	UnitsPerDay  int64  `json:"units_per_day"`
	PaidRatioBPS int64  `json:"paid_ratio_bps"`
}

type AbsenceRecord struct {
	ID    string `json:"id"`
	Date  string `json:"date"`
	Units int64  `json:"units"`
}

type CalculateInput struct {
	SummaryID      string             `json:"summary_id"`
	TenantID       string             `json:"tenant_id"`
	Period         string             `json:"period"`
	Policy         Policy             `json:"policy"`
	Employment     EmploymentSnapshot `json:"employment"`
	Leaves         []LeaveRecord      `json:"leaves"`
	Absences       []AbsenceRecord    `json:"absences"`
	SourceRevision int64              `json:"source_revision"`
}

func Calculate(input CalculateInput) (contract.MonthlySummary, error) {
	monthStart, monthEnd, err := monthBounds(input.Period)
	if err != nil || !uuidPattern.MatchString(input.SummaryID) || !uuidPattern.MatchString(input.TenantID) ||
		!uuidPattern.MatchString(input.Employment.PersonID) || input.Employment.SourceRevision < 1 ||
		!digestPattern.MatchString(input.Employment.SourceDigest) || input.SourceRevision < 1 {
		return contract.MonthlySummary{}, fmt.Errorf("attendance calculation identity is invalid")
	}
	standardDates, err := policyDates(input.Policy, monthStart, monthEnd)
	if err != nil {
		return contract.MonthlySummary{}, err
	}
	if strings.TrimSpace(input.Employment.DisplayName) == "" || len(input.Employment.Intervals) == 0 {
		return contract.MonthlySummary{}, fmt.Errorf("employment snapshot is invalid")
	}
	intervals := make([][2]time.Time, 0, len(input.Employment.Intervals))
	for _, interval := range input.Employment.Intervals {
		from, fromErr := parseDate(interval.From)
		through := monthEnd
		var throughErr error
		if interval.Through != "" {
			through, throughErr = parseDate(interval.Through)
		}
		if fromErr != nil || throughErr != nil || through.Before(from) {
			return contract.MonthlySummary{}, fmt.Errorf("employment interval is invalid")
		}
		intervals = append(intervals, [2]time.Time{from, through})
	}
	sort.Slice(intervals, func(i, j int) bool { return intervals[i][0].Before(intervals[j][0]) })
	for i := 1; i < len(intervals); i++ {
		if !intervals[i][0].After(intervals[i-1][1]) {
			return contract.MonthlySummary{}, fmt.Errorf("employment intervals overlap")
		}
	}
	eligible := map[string]bool{}
	for _, date := range standardDates {
		for _, interval := range intervals {
			if !date.Before(interval[0]) && !date.After(interval[1]) {
				eligible[date.Format("2006-01-02")] = true
				break
			}
		}
	}
	deductions := map[string]int64{}
	var unpaidLeave int64
	seenLeave := map[string]bool{}
	for _, leave := range input.Leaves {
		if !uuidPattern.MatchString(leave.ID) || seenLeave[leave.ID] || leave.UnitsPerDay < 1 || leave.UnitsPerDay > contract.FullDayUnits || leave.PaidRatioBPS < 0 || leave.PaidRatioBPS > 10000 {
			return contract.MonthlySummary{}, fmt.Errorf("leave record is invalid")
		}
		seenLeave[leave.ID] = true
		from, fromErr := parseDate(leave.From)
		through, throughErr := parseDate(leave.Through)
		if fromErr != nil || throughErr != nil || through.Before(from) {
			return contract.MonthlySummary{}, fmt.Errorf("leave range is invalid")
		}
		for date := from; !date.After(through); date = date.AddDate(0, 0, 1) {
			key := date.Format("2006-01-02")
			if !eligible[key] {
				continue
			}
			deduction := roundHalfUp(leave.UnitsPerDay*(10000-leave.PaidRatioBPS), 10000)
			if deductions[key]+deduction > contract.FullDayUnits {
				return contract.MonthlySummary{}, fmt.Errorf("daily leave and absence exceed one payable unit")
			}
			deductions[key] += deduction
			unpaidLeave += deduction
		}
	}
	var absence int64
	seenAbsence := map[string]bool{}
	for _, record := range input.Absences {
		if !uuidPattern.MatchString(record.ID) || record.Units < 1 || record.Units > contract.FullDayUnits || seenAbsence[record.ID] {
			return contract.MonthlySummary{}, fmt.Errorf("absence record is invalid")
		}
		seenAbsence[record.ID] = true
		date, dateErr := parseDate(record.Date)
		if dateErr != nil {
			return contract.MonthlySummary{}, fmt.Errorf("absence date is invalid")
		}
		key := date.Format("2006-01-02")
		if !eligible[key] {
			continue
		}
		if deductions[key]+record.Units > contract.FullDayUnits {
			return contract.MonthlySummary{}, fmt.Errorf("daily leave and absence exceed one payable unit")
		}
		deductions[key] += record.Units
		absence += record.Units
	}
	policyDigest := digest(input.Policy)
	summary := contract.MonthlySummary{
		SchemaVersion: contract.SchemaVersion, ID: input.SummaryID, TenantID: input.TenantID,
		PersonID: input.Employment.PersonID, PersonDisplayName: strings.TrimSpace(input.Employment.DisplayName), PersonSourceRevision: input.Employment.SourceRevision,
		PersonSourceDigest: input.Employment.SourceDigest, Period: input.Period, PolicyKey: input.Policy.Key,
		PolicyRevision: input.Policy.Revision, PolicyDigest: policyDigest, Basis: input.Policy.Basis,
		StandardUnits:             int64(len(standardDates)) * contract.FullDayUnits,
		EmploymentEligibleUnits:   int64(len(eligible)) * contract.FullDayUnits,
		UnpaidLeaveDeductionUnits: unpaidLeave, AbsenceDeductionUnits: absence,
		PayableUnits:   int64(len(eligible))*contract.FullDayUnits - unpaidLeave - absence,
		SourceRevision: input.SourceRevision,
	}
	if err := contract.Validate(summary); err != nil {
		return contract.MonthlySummary{}, err
	}
	return summary, nil
}

func policyDates(policy Policy, monthStart, monthEnd time.Time) ([]time.Time, error) {
	if policy.Key == "" || policy.Key != strings.TrimSpace(policy.Key) || policy.Revision < 1 || (policy.Basis != "calendar_days" && policy.Basis != "scheduled_workdays") {
		return nil, fmt.Errorf("attendance policy is invalid")
	}
	if policy.Basis == "calendar_days" {
		if len(policy.ScheduledDates) != 0 {
			return nil, fmt.Errorf("calendar day policy must not carry a work schedule")
		}
		var dates []time.Time
		for date := monthStart; !date.After(monthEnd); date = date.AddDate(0, 0, 1) {
			dates = append(dates, date)
		}
		return dates, nil
	}
	if len(policy.ScheduledDates) == 0 {
		return nil, fmt.Errorf("scheduled workday policy requires explicit dates")
	}
	seen := map[string]bool{}
	dates := make([]time.Time, 0, len(policy.ScheduledDates))
	for _, raw := range policy.ScheduledDates {
		date, err := parseDate(raw)
		if err != nil || date.Before(monthStart) || date.After(monthEnd) || seen[raw] {
			return nil, fmt.Errorf("scheduled workday is invalid or duplicated")
		}
		seen[raw] = true
		dates = append(dates, date)
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
	return dates, nil
}

func monthBounds(period string) (time.Time, time.Time, error) {
	start, err := time.Parse("2006-01", period)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return start, start.AddDate(0, 1, -1), nil
}

func parseDate(value string) (time.Time, error) { return time.Parse("2006-01-02", value) }

func roundHalfUp(numerator, denominator int64) int64 {
	return (numerator + denominator/2) / denominator
}

func digest(value any) string {
	encoded, _ := json.Marshal(value)
	hash := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(hash[:])
}
