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

const payrollImportPreviewTTL = 15 * time.Minute

type PayrollImportRowInput struct {
	PersonID             string       `json:"person_id"`
	MemberReferenceCode  string       `json:"member_reference_code"`
	ExpectedLineRevision int64        `json:"expected_line_revision"`
	PerformanceMinor     int64        `json:"performance_minor"`
	TaxMinor             int64        `json:"tax_minor"`
	TaxSourceNote        string       `json:"tax_source_note"`
	Adjustments          []Adjustment `json:"adjustments"`
}

type PreviewPayrollImportInput struct {
	ExpectedRunRevision int64                   `json:"expected_run_revision"`
	Rows                []PayrollImportRowInput `json:"rows"`
}

type PayrollImportPreviewItem struct {
	InputIndex          int         `json:"input_index"`
	ProvidedPersonID    string      `json:"provided_person_id,omitempty"`
	MemberReferenceCode string      `json:"member_reference_code,omitempty"`
	MatchStatus         string      `json:"match_status"`
	ErrorCode           string      `json:"error_code,omitempty"`
	ErrorMessage        string      `json:"error_message,omitempty"`
	PersonID            string      `json:"person_id,omitempty"`
	PayrollLineID       string      `json:"payroll_line_id,omitempty"`
	ChangedFields       []string    `json:"changed_fields"`
	Before              PayrollLine `json:"before"`
	After               PayrollLine `json:"after"`
}

type PayrollImportPreview struct {
	ID                 string                     `json:"id"`
	PayrollRunID       string                     `json:"payroll_run_id"`
	PayrollRunRevision int64                      `json:"payroll_run_revision"`
	PreviewHash        string                     `json:"preview_hash"`
	ExpiresAt          time.Time                  `json:"expires_at"`
	Confirmable        bool                       `json:"confirmable"`
	Items              []PayrollImportPreviewItem `json:"items"`
}

type CommitPayrollImportInput struct {
	CommandID           string `json:"command_id"`
	PreviewID           string `json:"preview_id"`
	PreviewHash         string `json:"preview_hash"`
	ExpectedRunRevision int64  `json:"expected_run_revision"`
	Reason              string `json:"reason"`
}

type payrollImportStore interface {
	PreviewPayrollImport(context.Context, Actor, string, PreviewPayrollImportInput) (PayrollImportPreview, error)
	CommitPayrollImport(context.Context, Actor, string, CommitPayrollImportInput, string) (ExecutionRun, error)
}

func normalizePayrollImportRows(rows []PayrollImportRowInput) ([]PayrollImportRowInput, error) {
	if len(rows) == 0 || len(rows) > 1000 {
		return nil, fmt.Errorf("invalid payroll import rows")
	}
	result := make([]PayrollImportRowInput, len(rows))
	for index, value := range rows {
		value.PersonID = strings.ToLower(strings.TrimSpace(value.PersonID))
		value.MemberReferenceCode = strings.TrimSpace(value.MemberReferenceCode)
		value.TaxSourceNote = strings.TrimSpace(value.TaxSourceNote)
		result[index] = value
	}
	return result, nil
}

func matchPayrollImportLine(lines []PayrollLine, row PayrollImportRowInput) (PayrollLine, int, string, string) {
	hasPersonID := row.PersonID != ""
	hasReferenceCode := row.MemberReferenceCode != ""
	if hasPersonID == hasReferenceCode {
		return PayrollLine{}, -1, "identifier_required", "每行必须且只能填写 Person ID 或成员编号之一"
	}
	if hasPersonID {
		if !uuidPattern.MatchString(row.PersonID) {
			return PayrollLine{}, -1, "invalid_person_id", "Person ID 格式不正确"
		}
		line, index := findLineByPerson(lines, row.PersonID)
		if index < 0 {
			return PayrollLine{}, -1, "person_not_found", "当前工资表中没有该 Person ID"
		}
		return line, index, "", ""
	}
	if len([]rune(row.MemberReferenceCode)) > 64 {
		return PayrollLine{}, -1, "invalid_reference_code", "成员编号长度不能超过 64 个字符"
	}
	for index, line := range lines {
		if line.PersonReferenceCode != "" && strings.EqualFold(line.PersonReferenceCode, row.MemberReferenceCode) {
			return line, index, "", ""
		}
	}
	return PayrollLine{}, -1, "reference_code_not_found", "当前工资表中没有该成员编号"
}

func mapImportErrorStatus(code string) string {
	if code == "person_not_found" || code == "reference_code_not_found" {
		return "unmatched"
	}
	return "error"
}

func (s *Service) PreviewPayrollImport(ctx context.Context, actor Actor, runID string, input PreviewPayrollImportInput) (PayrollImportPreview, error) {
	if err := validateActor(actor); err != nil {
		return PayrollImportPreview{}, err
	}
	runID = strings.ToLower(strings.TrimSpace(runID))
	rows, err := normalizePayrollImportRows(input.Rows)
	if err != nil || !uuidPattern.MatchString(runID) || input.ExpectedRunRevision < 1 {
		return PayrollImportPreview{}, fmt.Errorf("invalid payroll import preview")
	}
	input.Rows = rows
	store, ok := s.store.(payrollImportStore)
	if !ok {
		return PayrollImportPreview{}, ErrReferenceUnavailable
	}
	return store.PreviewPayrollImport(ctx, actor, runID, input)
}

func (s *Service) CommitPayrollImport(ctx context.Context, actor Actor, runID string, input CommitPayrollImportInput) (ExecutionRun, error) {
	if err := validateActor(actor); err != nil {
		return ExecutionRun{}, err
	}
	runID = strings.ToLower(strings.TrimSpace(runID))
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	input.PreviewID = strings.ToLower(strings.TrimSpace(input.PreviewID))
	input.PreviewHash = strings.ToLower(strings.TrimSpace(input.PreviewHash))
	input.Reason = strings.TrimSpace(input.Reason)
	if !uuidPattern.MatchString(runID) || !uuidPattern.MatchString(input.CommandID) || !uuidPattern.MatchString(input.PreviewID) || len(input.PreviewHash) != 64 || input.ExpectedRunRevision < 1 || input.Reason == "" || len([]rune(input.Reason)) > 500 {
		return ExecutionRun{}, fmt.Errorf("invalid payroll import confirmation")
	}
	store, ok := s.store.(payrollImportStore)
	if !ok {
		return ExecutionRun{}, ErrReferenceUnavailable
	}
	return store.CommitPayrollImport(ctx, actor, runID, input, hash(struct {
		RunID string
		Input CommitPayrollImportInput
	}{runID, input}))
}

func (s *PostgresStore) PreviewPayrollImport(ctx context.Context, actor Actor, runID string, input PreviewPayrollImportInput) (PayrollImportPreview, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return PayrollImportPreview{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	run, err := loadExecutionRun(ctx, tx, actor.TenantID, runID, "FOR UPDATE")
	if err != nil {
		return PayrollImportPreview{}, err
	}
	if run.Revision != input.ExpectedRunRevision {
		return PayrollImportPreview{}, ErrRevisionConflict
	}
	if run.Status == "voided" || hasBlockingDeliveries(ctx, tx, actor.TenantID, runID, "succeeded", "unknown") {
		return PayrollImportPreview{}, ErrRunLocked
	}
	preview := PayrollImportPreview{ID: newUUID(), PayrollRunID: run.ID, PayrollRunRevision: run.Revision, ExpiresAt: time.Now().UTC().Add(payrollImportPreviewTTL), Confirmable: true, Items: []PayrollImportPreviewItem{}}
	changed := false
	seen := map[string]bool{}
	for rowIndex, row := range input.Rows {
		item := PayrollImportPreviewItem{InputIndex: rowIndex, ProvidedPersonID: row.PersonID, MemberReferenceCode: row.MemberReferenceCode, MatchStatus: "error", ChangedFields: []string{}}
		before, lineIndex, code, message := matchPayrollImportLine(run.Lines, row)
		if code != "" {
			item.MatchStatus, item.ErrorCode, item.ErrorMessage = mapImportErrorStatus(code), code, message
			preview.Confirmable = false
			preview.Items = append(preview.Items, item)
			continue
		}
		if seen[before.ID] {
			item.PersonID, item.PayrollLineID = before.PersonID, before.ID
			item.ErrorCode, item.ErrorMessage = "duplicate_person", "同一成员在导入文件中出现多次"
			preview.Confirmable = false
			preview.Items = append(preview.Items, item)
			continue
		}
		seen[before.ID] = true
		row.PersonID = before.PersonID
		input.Rows[rowIndex] = row
		if before.Revision != row.ExpectedLineRevision {
			item.PersonID, item.PayrollLineID = before.PersonID, before.ID
			item.ErrorCode, item.ErrorMessage = "revision_conflict", "工资明细已变化，请重新下载模板或刷新后再预览"
			preview.Confirmable = false
			preview.Items = append(preview.Items, item)
			continue
		}
		adjustments, normalizeErr := normalizeAdjustments(row.Adjustments)
		if normalizeErr != nil || row.PerformanceMinor < 0 || row.PerformanceMinor > maxPayrollMinor || row.TaxMinor < 0 || row.TaxMinor > maxPayrollMinor || len([]rune(row.TaxSourceNote)) > 500 {
			item.PersonID, item.PayrollLineID = before.PersonID, before.ID
			item.ErrorCode, item.ErrorMessage = "invalid_values", "绩效、个税、调整项或说明格式不正确"
			preview.Confirmable = false
			preview.Items = append(preview.Items, item)
			continue
		}
		row.Adjustments = adjustments
		input.Rows[rowIndex] = row
		after, applyErr := applyImportedPayrollLine(before, run.ContributionSchemeSnapshot, row)
		if applyErr != nil {
			item.PersonID, item.PayrollLineID = before.PersonID, before.ID
			item.ErrorCode, item.ErrorMessage = "calculation_failed", "导入值无法形成有效工资结果"
			preview.Confirmable = false
			preview.Items = append(preview.Items, item)
			continue
		}
		fields := importedChangedFields(before, after)
		if len(fields) > 0 {
			changed = true
		} else {
			item.ErrorCode, item.ErrorMessage = "no_changes", "该行与当前工资明细一致"
			preview.Confirmable = false
		}
		item.MatchStatus, item.PersonID, item.PayrollLineID, item.ChangedFields, item.Before, item.After = "matched", before.PersonID, before.ID, fields, before, after
		_ = lineIndex
		preview.Items = append(preview.Items, item)
	}
	if !changed {
		preview.Confirmable = false
	}
	preview.PreviewHash = hash(struct {
		RunID       string
		RunRevision int64
		Rows        []PayrollImportRowInput
	}{run.ID, run.Revision, input.Rows})
	raw, _ := json.Marshal(input.Rows)
	createdAt := preview.ExpiresAt.Add(-payrollImportPreviewTTL)
	_, err = tx.Exec(ctx, `INSERT INTO payroll_import_previews(id,tenant_id,payroll_run_id,payroll_run_revision,preview_hash,rows_snapshot,confirmable,status,created_by_subject_id,created_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,'open',$8,$9,$10)`, preview.ID, actor.TenantID, run.ID, run.Revision, preview.PreviewHash, raw, preview.Confirmable, actor.ID, createdAt, preview.ExpiresAt)
	if err != nil {
		return PayrollImportPreview{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return PayrollImportPreview{}, err
	}
	return preview, nil
}

func (s *PostgresStore) CommitPayrollImport(ctx context.Context, actor Actor, runID string, input CommitPayrollImportInput, requestHash string) (ExecutionRun, error) {
	tx, err := s.begin(ctx, actor.TenantID, input.CommandID)
	if err != nil {
		return ExecutionRun{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, replayErr := replayValue[ExecutionRun](ctx, tx, actor, input.CommandID, requestHash); replayErr != nil || found {
		return value, replayErr
	}
	var previewRunID, previewHash, status, createdBy string
	var previewRunRevision int64
	var confirmable bool
	var rowsRaw []byte
	var expiresAt time.Time
	err = tx.QueryRow(ctx, `SELECT payroll_run_id::text,payroll_run_revision,preview_hash,rows_snapshot,confirmable,status,created_by_subject_id::text,expires_at FROM payroll_import_previews WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, actor.TenantID, input.PreviewID).Scan(&previewRunID, &previewRunRevision, &previewHash, &rowsRaw, &confirmable, &status, &createdBy, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ExecutionRun{}, ErrPreviewStale
	}
	if err != nil {
		return ExecutionRun{}, err
	}
	now := time.Now().UTC()
	if previewRunID != runID || previewRunRevision != input.ExpectedRunRevision || previewHash != input.PreviewHash || !confirmable || status != "open" || createdBy != actor.ID || !expiresAt.After(now) {
		return ExecutionRun{}, ErrPreviewStale
	}
	var rows []PayrollImportRowInput
	if err = json.Unmarshal(rowsRaw, &rows); err != nil {
		return ExecutionRun{}, ErrPreviewStale
	}
	run, err := loadExecutionRun(ctx, tx, actor.TenantID, runID, "FOR UPDATE")
	if err != nil {
		return ExecutionRun{}, err
	}
	if run.Revision != input.ExpectedRunRevision {
		return ExecutionRun{}, ErrPreviewStale
	}
	if run.Status == "voided" || hasBlockingDeliveries(ctx, tx, actor.TenantID, runID, "succeeded", "unknown") {
		return ExecutionRun{}, ErrRunLocked
	}
	run.Revision++
	run.Status = "review_pending"
	run.UpdatedAt = now
	for _, row := range rows {
		line, index := findLineByPerson(run.Lines, row.PersonID)
		if index < 0 || line.Revision != row.ExpectedLineRevision {
			return ExecutionRun{}, ErrPreviewStale
		}
		line, err = applyImportedPayrollLine(line, run.ContributionSchemeSnapshot, row)
		if err != nil {
			return ExecutionRun{}, err
		}
		line.Revision++
		line.UpdatedAt = now
		run.Lines[index] = line
		if err = updatePayrollLine(ctx, tx, line); err != nil {
			return ExecutionRun{}, err
		}
		if err = insertPayrollLineRevision(ctx, tx, line, run.Revision, actor, input.CommandID, "import_committed", now); err != nil {
			return ExecutionRun{}, err
		}
	}
	if err = updateExecutionRun(ctx, tx, run); err != nil {
		return ExecutionRun{}, err
	}
	if err = insertExecutionRunRevision(ctx, tx, run, actor, input.CommandID, "import_committed"); err != nil {
		return ExecutionRun{}, err
	}
	if err = insertAudit(ctx, tx, run.Run, actor, input.CommandID, input.ExpectedRunRevision, "payroll_import_committed"); err != nil {
		return ExecutionRun{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE payroll_import_previews SET status='committed',committed_command_id=$3,committed_at=$4 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, input.PreviewID, input.CommandID, now); err != nil {
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

func applyImportedPayrollLine(line PayrollLine, scheme ContributionScheme, row PayrollImportRowInput) (PayrollLine, error) {
	gross, contributions, employee, employer, preTax, net, cost, err := CalculatePayrollLine(line.FixedSalaryMinor, line.SocialInsuranceBaseMinor, line.HousingFundBaseMinor, row.PerformanceMinor, row.TaxMinor, row.Adjustments, scheme.Components)
	if err != nil {
		return PayrollLine{}, err
	}
	line.PerformanceMinor = row.PerformanceMinor
	line.TaxMinor = row.TaxMinor
	line.TaxSourceKind = "import"
	line.TaxSourceNote = row.TaxSourceNote
	line.Adjustments = row.Adjustments
	line.GrossMinor = gross
	line.Contributions = contributions
	line.EmployeeContributionMinor = employee
	line.EmployerContributionMinor = employer
	line.PreTaxPayableMinor = preTax
	line.NetPayableMinor = net
	line.EmployerCostMinor = cost
	line.Status = "review_pending"
	return line, nil
}

func importedChangedFields(before, after PayrollLine) []string {
	fields := []string{}
	if before.PerformanceMinor != after.PerformanceMinor {
		fields = append(fields, "performance_minor")
	}
	if before.TaxMinor != after.TaxMinor || before.TaxSourceKind != after.TaxSourceKind || before.TaxSourceNote != after.TaxSourceNote {
		fields = append(fields, "tax_snapshot")
	}
	if hash(before.Adjustments) != hash(after.Adjustments) {
		fields = append(fields, "adjustments")
	}
	if before.NetPayableMinor != after.NetPayableMinor {
		fields = append(fields, "net_payable_minor")
	}
	return fields
}

func findLineByPerson(lines []PayrollLine, personID string) (PayrollLine, int) {
	for index, line := range lines {
		if line.PersonID == personID {
			return line, index
		}
	}
	return PayrollLine{}, -1
}
