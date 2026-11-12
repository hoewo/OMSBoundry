package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"omsbizfoundry/payroll/internal/auth"
	domain "omsbizfoundry/payroll/internal/payroll"
	"strings"
)

const maxRequestBytes = 1 << 20

type UseCases interface {
	Ping(context.Context) error
	Create(context.Context, domain.Actor, domain.CreateInput) (domain.Run, error)
	Update(context.Context, domain.Actor, string, domain.UpdateInput) (domain.Run, error)
	Get(context.Context, string, string) (domain.Run, error)
	List(context.Context, string) (domain.Page, error)
}
type ProfileUseCases interface {
	UpsertCompensation(context.Context, domain.Actor, domain.UpsertCompensationProfileInput) (domain.CompensationProfile, error)
	GetCompensation(context.Context, string, string) (domain.CompensationProfile, error)
	ListCompensations(context.Context, string) ([]domain.CompensationProfile, error)
	UpsertPayment(context.Context, domain.Actor, domain.UpsertPaymentProfileInput) (domain.PaymentProfile, error)
	GetPayment(context.Context, string, string) (domain.PaymentProfile, error)
	ListPayments(context.Context, string) ([]domain.PaymentProfile, error)
	ListPaymentHistory(context.Context, string, string) ([]domain.PaymentProfile, error)
	ReadSensitivePayment(context.Context, domain.Actor, string, string) (domain.SensitivePaymentProfile, error)
	CreateCustomScheme(context.Context, domain.Actor, domain.CreateCustomSchemeInput) (domain.ContributionScheme, error)
	ReviseCustomScheme(context.Context, domain.Actor, string, domain.ReviseCustomSchemeInput) (domain.ContributionScheme, error)
	ListSchemes(context.Context, string) ([]domain.ContributionScheme, error)
}
type ExecutionUseCases interface {
	CreateRegularRun(context.Context, domain.Actor, domain.CreateRegularRunInput) (domain.ExecutionRun, error)
	CreateAdjustmentRun(context.Context, domain.Actor, domain.CreateAdjustmentRunInput) (domain.ExecutionRun, error)
	PreviewPayrollImport(context.Context, domain.Actor, string, domain.PreviewPayrollImportInput) (domain.PayrollImportPreview, error)
	CommitPayrollImport(context.Context, domain.Actor, string, domain.CommitPayrollImportInput) (domain.ExecutionRun, error)
	GetExecutionRun(context.Context, string, string) (domain.ExecutionRun, error)
	ListExecutionRuns(context.Context, string) ([]domain.ExecutionRun, error)
	RevisePayrollLine(context.Context, domain.Actor, string, string, domain.ReviseLineInput) (domain.ExecutionRun, error)
	ApplyAttendanceSummary(context.Context, domain.Actor, string, string, domain.ApplyAttendanceSummaryInput) (domain.ExecutionRun, error)
	ReviewPayrollRun(context.Context, domain.Actor, string, domain.RunCommandInput) (domain.ExecutionRun, error)
	ConfirmZeroLine(context.Context, domain.Actor, string, string, domain.LineCommandInput) (domain.ExecutionRun, error)
	VoidPayrollRun(context.Context, domain.Actor, string, domain.RunCommandInput) (domain.ExecutionRun, error)
	RecordPayment(context.Context, domain.Actor, string, string, domain.RecordPaymentInput) (domain.ExecutionRun, error)
	SyncMembers(context.Context, domain.Actor, string, domain.SyncMembersInput) (domain.ExecutionRun, error)
	PreviewCompensationSync(context.Context, domain.Actor, string, string) (domain.SnapshotSyncPreview, error)
	ConfirmCompensationSync(context.Context, domain.Actor, string, string, domain.ConfirmSnapshotSyncInput) (domain.ExecutionRun, error)
	PreviewPaymentProfileSync(context.Context, domain.Actor, string, string) (domain.SnapshotSyncPreview, error)
	ConfirmPaymentProfileSync(context.Context, domain.Actor, string, string, domain.ConfirmSnapshotSyncInput) (domain.ExecutionRun, error)
	ListSelfPayslips(context.Context, domain.Actor) (domain.SelfPayslipPage, error)
}
type Server struct {
	useCases      UseCases
	profiles      ProfileUseCases
	execution     ExecutionUseCases
	authenticator auth.Authenticator
	handler       http.Handler
}
type principalKey struct{}

func New(useCases UseCases, authenticator auth.Authenticator) (*Server, error) {
	if useCases == nil || authenticator == nil {
		return nil, fmt.Errorf("use cases and authenticator are required")
	}
	s := &Server{useCases: useCases, authenticator: authenticator}
	if profiles, ok := useCases.(ProfileUseCases); ok {
		s.profiles = profiles
	}
	if execution, ok := useCases.(ExecutionUseCases); ok {
		s.execution = execution
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", s.live)
	mux.HandleFunc("GET /health/ready", s.ready)
	mux.Handle("POST /api/v1/payroll-runs", s.protected(auth.ActionRunManage, http.HandlerFunc(s.create)))
	mux.Handle("GET /api/v1/payroll-runs", s.protected(auth.ActionTenantRead, http.HandlerFunc(s.list)))
	mux.Handle("GET /api/v1/payroll-runs/{run_id}", s.protected(auth.ActionTenantRead, http.HandlerFunc(s.get)))
	mux.Handle("PUT /api/v1/payroll-runs/{run_id}", s.protected(auth.ActionRunManage, http.HandlerFunc(s.update)))
	mux.Handle("GET /api/v1/references/payroll-runs/{run_id}", s.protected(auth.ActionReferenceRead, http.HandlerFunc(s.reference)))
	s.registerProfileRoutes(mux, "/api/v1")
	s.registerExecutionRoutes(mux, "/api/v1")
	// Gateway strips the public, versioned /payroll/v1 prefix.
	mux.Handle("POST /payroll-runs", s.protected(auth.ActionRunManage, http.HandlerFunc(s.create)))
	mux.Handle("GET /payroll-runs", s.protected(auth.ActionTenantRead, http.HandlerFunc(s.list)))
	mux.Handle("GET /payroll-runs/{run_id}", s.protected(auth.ActionTenantRead, http.HandlerFunc(s.get)))
	mux.Handle("PUT /payroll-runs/{run_id}", s.protected(auth.ActionRunManage, http.HandlerFunc(s.update)))
	mux.Handle("GET /references/payroll-runs/{run_id}", s.protected(auth.ActionReferenceRead, http.HandlerFunc(s.reference)))
	s.registerProfileRoutes(mux, "")
	s.registerExecutionRoutes(mux, "")
	s.handler = securityHeaders(mux)
	return s, nil
}

func (s *Server) registerExecutionRoutes(mux *http.ServeMux, prefix string) {
	mux.Handle("POST "+prefix+"/payroll-periods", s.protected(auth.ActionRunManage, http.HandlerFunc(s.createRegularRun)))
	mux.Handle("POST "+prefix+"/payroll-adjustment-runs", s.protected(auth.ActionRunManage, http.HandlerFunc(s.createAdjustmentRun)))
	mux.Handle("POST "+prefix+"/payroll-periods/{run_id}/imports:preview", s.protected(auth.ActionRunManage, http.HandlerFunc(s.previewPayrollImport)))
	mux.Handle("POST "+prefix+"/payroll-periods/{run_id}/imports:commit", s.protected(auth.ActionRunManage, http.HandlerFunc(s.commitPayrollImport)))
	mux.Handle("GET "+prefix+"/payroll-periods", s.protected(auth.ActionTenantRead, http.HandlerFunc(s.listExecutionRuns)))
	mux.Handle("GET "+prefix+"/payroll-periods/{run_id}", s.protected(auth.ActionTenantRead, http.HandlerFunc(s.getExecutionRun)))
	mux.Handle("PUT "+prefix+"/payroll-periods/{run_id}/lines/{line_id}", s.protected(auth.ActionRunManage, http.HandlerFunc(s.revisePayrollLine)))
	mux.Handle("POST "+prefix+"/payroll-periods/{run_id}/lines/{line_id}/attendance-summary", s.protected(auth.ActionRunManage, http.HandlerFunc(s.applyAttendanceSummary)))
	mux.Handle("POST "+prefix+"/payroll-periods/{run_id}/review", s.protected(auth.ActionRunManage, http.HandlerFunc(s.reviewPayrollRun)))
	mux.Handle("POST "+prefix+"/payroll-periods/{run_id}/void", s.protected(auth.ActionRunManage, http.HandlerFunc(s.voidPayrollRun)))
	mux.Handle("POST "+prefix+"/payroll-periods/{run_id}/lines/{line_id}/confirm-no-payment", s.protected(auth.ActionRunManage, http.HandlerFunc(s.confirmZeroLine)))
	mux.Handle("POST "+prefix+"/payroll-periods/{run_id}/lines/{line_id}/payments", s.protected(auth.ActionPaymentConfirm, http.HandlerFunc(s.recordPayment)))
	mux.Handle("POST "+prefix+"/payroll-periods/{run_id}/payments:batch", s.protected(auth.ActionPaymentBatchConfirm, http.HandlerFunc(s.recordPaymentsBatch)))
	mux.Handle("POST "+prefix+"/payroll-periods/{run_id}/members:sync", s.protected(auth.ActionRunManage, http.HandlerFunc(s.syncMembers)))
	mux.Handle("GET "+prefix+"/payroll-periods/{run_id}/lines/{line_id}/compensation-sync-preview", s.protected(auth.ActionRunManage, http.HandlerFunc(s.previewCompensationSync)))
	mux.Handle("POST "+prefix+"/payroll-periods/{run_id}/lines/{line_id}/compensation:sync", s.protected(auth.ActionRunManage, http.HandlerFunc(s.confirmCompensationSync)))
	mux.Handle("GET "+prefix+"/payroll-periods/{run_id}/lines/{line_id}/payment-profile-sync-preview", s.protected(auth.ActionRunManage, http.HandlerFunc(s.previewPaymentProfileSync)))
	mux.Handle("POST "+prefix+"/payroll-periods/{run_id}/lines/{line_id}/payment-profile:sync", s.protected(auth.ActionRunManage, http.HandlerFunc(s.confirmPaymentProfileSync)))
	mux.Handle("GET "+prefix+"/me/payslips", s.protected(auth.ActionSelfRead, http.HandlerFunc(s.listSelfPayslips)))
}
func (s *Server) registerProfileRoutes(mux *http.ServeMux, prefix string) {
	mux.Handle("GET "+prefix+"/compensation-profiles", s.protected(auth.ActionTenantRead, http.HandlerFunc(s.listCompensations)))
	mux.Handle("GET "+prefix+"/compensation-profiles/{person_id}", s.protected(auth.ActionTenantRead, http.HandlerFunc(s.getCompensation)))
	mux.Handle("PUT "+prefix+"/compensation-profiles/{person_id}", s.protected(auth.ActionCompensationManage, http.HandlerFunc(s.upsertCompensation)))
	mux.Handle("GET "+prefix+"/payment-profiles", s.protected(auth.ActionTenantRead, http.HandlerFunc(s.listPayments)))
	mux.Handle("GET "+prefix+"/payment-profiles/{person_id}", s.protected(auth.ActionTenantRead, http.HandlerFunc(s.getPayment)))
	mux.Handle("PUT "+prefix+"/payment-profiles/{person_id}", s.protected(auth.ActionPaymentProfileManage, http.HandlerFunc(s.upsertPayment)))
	mux.Handle("GET "+prefix+"/payment-profiles/{person_id}/history", s.protected(auth.ActionTenantRead, http.HandlerFunc(s.paymentHistory)))
	mux.Handle("POST "+prefix+"/payment-profiles/{person_id}/sensitive-account", s.protected(auth.ActionSensitiveAccountRead, http.HandlerFunc(s.sensitivePayment)))
	mux.Handle("GET "+prefix+"/contribution-schemes", s.protected(auth.ActionTenantRead, http.HandlerFunc(s.listSchemes)))
	mux.Handle("POST "+prefix+"/contribution-schemes/custom", s.protected(auth.ActionContributionSchemeManage, http.HandlerFunc(s.createCustomScheme)))
	mux.Handle("PUT "+prefix+"/contribution-schemes/custom/{scheme_id}", s.protected(auth.ActionContributionSchemeManage, http.HandlerFunc(s.reviseCustomScheme)))
}

func (s *Server) requireProfiles(w http.ResponseWriter) bool {
	if s.profiles == nil {
		writeProblem(w, 503, "PROFILE_OWNER_UNAVAILABLE", "薪酬档案暂不可用", true)
		return false
	}
	return true
}
func (s *Server) requireExecution(w http.ResponseWriter) bool {
	if s.execution == nil {
		writeProblem(w, 503, "PAYROLL_EXECUTION_UNAVAILABLE", "工资执行暂不可用", true)
		return false
	}
	return true
}
func requireTenantScope(w http.ResponseWriter, p auth.Principal) bool {
	if p.ScopeKind != "tenant" {
		writeProblem(w, 403, "SCOPE_DENIED", "该操作需要租户范围", false)
		return false
	}
	return true
}
func requireRunScope(w http.ResponseWriter, p auth.Principal, runID string) bool {
	if p.CanResourceType("payroll_run", runID) {
		return true
	}
	writeProblem(w, 403, "SCOPE_DENIED", "凭证范围不包含该工资表", false)
	return false
}
func requireRunLineScope(w http.ResponseWriter, p auth.Principal, runID, lineID string) bool {
	if p.ScopeKind == "tenant" || (p.CanResourceType("payroll_run", runID) && p.CanResourceType("payroll_line", lineID)) {
		return true
	}
	writeProblem(w, 403, "SCOPE_DENIED", "凭证范围不包含该工资表和工资行", false)
	return false
}
func executionIDs(r *http.Request) (string, string) {
	return strings.ToLower(strings.TrimSpace(r.PathValue("run_id"))), strings.ToLower(strings.TrimSpace(r.PathValue("line_id")))
}

func (s *Server) createRegularRun(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if !s.requireExecution(w) || !requireTenantScope(w, p) {
		return
	}
	var in domain.CreateRegularRunInput
	if !decodeJSON(w, r, &in) {
		return
	}
	v, err := s.execution.CreateRegularRun(r.Context(), actorFrom(p), in)
	s.writeExecution(w, http.StatusCreated, v, err)
}
func (s *Server) createAdjustmentRun(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if !s.requireExecution(w) || !requireTenantScope(w, p) {
		return
	}
	var in domain.CreateAdjustmentRunInput
	if !decodeJSON(w, r, &in) {
		return
	}
	v, err := s.execution.CreateAdjustmentRun(r.Context(), actorFrom(p), in)
	s.writeExecution(w, http.StatusCreated, v, err)
}
func (s *Server) previewPayrollImport(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	runID, _ := executionIDs(r)
	if !s.requireExecution(w) || !requireRunScope(w, p, runID) {
		return
	}
	var in domain.PreviewPayrollImportInput
	if !decodeJSON(w, r, &in) {
		return
	}
	v, err := s.execution.PreviewPayrollImport(r.Context(), actorFrom(p), runID, in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
func (s *Server) commitPayrollImport(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	runID, _ := executionIDs(r)
	if !s.requireExecution(w) || !requireRunScope(w, p, runID) {
		return
	}
	var in domain.CommitPayrollImportInput
	if !decodeJSON(w, r, &in) {
		return
	}
	v, err := s.execution.CommitPayrollImport(r.Context(), actorFrom(p), runID, in)
	s.writeExecution(w, http.StatusOK, v, err)
}
func (s *Server) listExecutionRuns(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if !s.requireExecution(w) || !requireTenantScope(w, p) {
		return
	}
	v, err := s.execution.ListExecutionRuns(r.Context(), p.TenantID)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": v})
}
func (s *Server) getExecutionRun(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	runID, _ := executionIDs(r)
	if !s.requireExecution(w) || !requireRunScope(w, p, runID) {
		return
	}
	v, err := s.execution.GetExecutionRun(r.Context(), p.TenantID, runID)
	s.writeExecution(w, http.StatusOK, v, err)
}
func (s *Server) revisePayrollLine(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	runID, lineID := executionIDs(r)
	if !s.requireExecution(w) || !requireRunLineScope(w, p, runID, lineID) {
		return
	}
	var in domain.ReviseLineInput
	if !decodeJSON(w, r, &in) {
		return
	}
	v, err := s.execution.RevisePayrollLine(r.Context(), actorFrom(p), runID, lineID, in)
	s.writeExecution(w, http.StatusOK, v, err)
}
func (s *Server) applyAttendanceSummary(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	runID, lineID := executionIDs(r)
	if !s.requireExecution(w) || !requireRunLineScope(w, p, runID, lineID) {
		return
	}
	var in domain.ApplyAttendanceSummaryInput
	if !decodeJSON(w, r, &in) {
		return
	}
	v, err := s.execution.ApplyAttendanceSummary(r.Context(), actorFrom(p), runID, lineID, in)
	s.writeExecution(w, http.StatusOK, v, err)
}
func (s *Server) reviewPayrollRun(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	runID, _ := executionIDs(r)
	if !s.requireExecution(w) || !requireRunScope(w, p, runID) {
		return
	}
	var in domain.RunCommandInput
	if !decodeJSON(w, r, &in) {
		return
	}
	v, err := s.execution.ReviewPayrollRun(r.Context(), actorFrom(p), runID, in)
	s.writeExecution(w, http.StatusOK, v, err)
}
func (s *Server) voidPayrollRun(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	runID, _ := executionIDs(r)
	if !s.requireExecution(w) || !requireRunScope(w, p, runID) {
		return
	}
	var in domain.RunCommandInput
	if !decodeJSON(w, r, &in) {
		return
	}
	v, err := s.execution.VoidPayrollRun(r.Context(), actorFrom(p), runID, in)
	s.writeExecution(w, http.StatusOK, v, err)
}
func (s *Server) confirmZeroLine(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	runID, lineID := executionIDs(r)
	if !s.requireExecution(w) || !requireRunLineScope(w, p, runID, lineID) {
		return
	}
	var in domain.LineCommandInput
	if !decodeJSON(w, r, &in) {
		return
	}
	v, err := s.execution.ConfirmZeroLine(r.Context(), actorFrom(p), runID, lineID, in)
	s.writeExecution(w, http.StatusOK, v, err)
}
func (s *Server) recordPayment(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	runID, lineID := executionIDs(r)
	if !s.requireExecution(w) || !requireRunLineScope(w, p, runID, lineID) {
		return
	}
	var in domain.RecordPaymentInput
	if !decodeJSON(w, r, &in) {
		return
	}
	v, err := s.execution.RecordPayment(r.Context(), actorFrom(p), runID, lineID, in)
	s.writeExecution(w, http.StatusOK, v, err)
}
func (s *Server) syncMembers(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	runID, _ := executionIDs(r)
	if !s.requireExecution(w) || !requireRunScope(w, p, runID) {
		return
	}
	var in domain.SyncMembersInput
	if !decodeJSON(w, r, &in) {
		return
	}
	v, err := s.execution.SyncMembers(r.Context(), actorFrom(p), runID, in)
	s.writeExecution(w, http.StatusOK, v, err)
}
func (s *Server) previewCompensationSync(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	runID, lineID := executionIDs(r)
	if !s.requireExecution(w) || !requireRunLineScope(w, p, runID, lineID) {
		return
	}
	v, err := s.execution.PreviewCompensationSync(r.Context(), actorFrom(p), runID, lineID)
	s.writeProfile(w, v, err)
}
func (s *Server) confirmCompensationSync(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	runID, lineID := executionIDs(r)
	if !s.requireExecution(w) || !requireRunLineScope(w, p, runID, lineID) {
		return
	}
	var in domain.ConfirmSnapshotSyncInput
	if !decodeJSON(w, r, &in) {
		return
	}
	v, err := s.execution.ConfirmCompensationSync(r.Context(), actorFrom(p), runID, lineID, in)
	s.writeExecution(w, http.StatusOK, v, err)
}
func (s *Server) previewPaymentProfileSync(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	runID, lineID := executionIDs(r)
	if !s.requireExecution(w) || !requireRunLineScope(w, p, runID, lineID) {
		return
	}
	v, err := s.execution.PreviewPaymentProfileSync(r.Context(), actorFrom(p), runID, lineID)
	s.writeProfile(w, v, err)
}
func (s *Server) confirmPaymentProfileSync(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	runID, lineID := executionIDs(r)
	if !s.requireExecution(w) || !requireRunLineScope(w, p, runID, lineID) {
		return
	}
	var in domain.ConfirmSnapshotSyncInput
	if !decodeJSON(w, r, &in) {
		return
	}
	v, err := s.execution.ConfirmPaymentProfileSync(r.Context(), actorFrom(p), runID, lineID, in)
	s.writeExecution(w, http.StatusOK, v, err)
}
func (s *Server) listSelfPayslips(w http.ResponseWriter, r *http.Request) {
	if !s.requireExecution(w) {
		return
	}
	value, err := s.execution.ListSelfPayslips(r.Context(), actorFrom(principalFrom(r)))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

type batchPaymentRequest struct {
	Items []struct {
		LineID string                    `json:"line_id"`
		Input  domain.RecordPaymentInput `json:"input"`
	} `json:"items"`
}
type batchPaymentResult struct {
	LineID string               `json:"line_id"`
	Status string               `json:"status"`
	Run    *domain.ExecutionRun `json:"run,omitempty"`
	Error  string               `json:"error,omitempty"`
}

func (s *Server) recordPaymentsBatch(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	runID, _ := executionIDs(r)
	if !s.requireExecution(w) || !requireRunScope(w, p, runID) {
		return
	}
	var in batchPaymentRequest
	if !decodeJSON(w, r, &in) {
		return
	}
	if len(in.Items) == 0 || len(in.Items) > 500 {
		writeProblem(w, 400, "INVALID_REQUEST", "批量付款必须包含 1 至 500 条工资行", false)
		return
	}
	current, err := s.execution.GetExecutionRun(r.Context(), p.TenantID, runID)
	if err != nil {
		s.fail(w, err)
		return
	}
	lineRevisions := make(map[string]int64, len(current.Lines))
	for _, line := range current.Lines {
		lineRevisions[line.ID] = line.Revision
	}
	results := make([]batchPaymentResult, 0, len(in.Items))
	seen := map[string]bool{}
	for _, item := range in.Items {
		lineID := strings.ToLower(strings.TrimSpace(item.LineID))
		if lineID == "" || seen[lineID] {
			writeProblem(w, 400, "INVALID_REQUEST", "批量付款工资行不能为空或重复", false)
			return
		}
		if !requireRunLineScope(w, p, runID, lineID) {
			return
		}
		if item.Input.ExpectedRunRevision != current.Revision || lineRevisions[lineID] == 0 || item.Input.ExpectedLineRevision != lineRevisions[lineID] {
			writeProblem(w, 409, "PAYROLL_PREVIEW_STALE", "批量付款预览已经过期，请重新核对", true)
			return
		}
		seen[lineID] = true
	}
	counts := map[string]int{"succeeded": 0, "failed": 0, "unknown": 0, "unexecuted": 0}
	for index, item := range in.Items {
		lineID := strings.ToLower(strings.TrimSpace(item.LineID))
		item.Input.ExpectedRunRevision = current.Revision
		value, err := s.execution.RecordPayment(r.Context(), actorFrom(p), runID, lineID, item.Input)
		result := batchPaymentResult{LineID: lineID, Status: "succeeded"}
		if err == nil {
			result.Run = &value
			current = value
		} else {
			result.Status = "failed"
			if errors.Is(err, domain.ErrPaymentUnknown) {
				result.Status = "unknown"
			}
			result.Error = err.Error()
		}
		counts[result.Status]++
		results = append(results, result)
		if errors.Is(err, domain.ErrRevisionConflict) || errors.Is(err, domain.ErrPreviewStale) {
			for _, remaining := range in.Items[index+1:] {
				results = append(results, batchPaymentResult{LineID: strings.ToLower(strings.TrimSpace(remaining.LineID)), Status: "unexecuted", Error: "batch preview became stale"})
				counts["unexecuted"]++
			}
			break
		}
	}
	writeJSON(w, http.StatusMultiStatus, map[string]any{"items": results, "summary": counts})
}
func (s *Server) writeExecution(w http.ResponseWriter, status int, value domain.ExecutionRun, err error) {
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, status, value)
}
func (s *Server) listCompensations(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if !s.requireProfiles(w) || !requireTenantScope(w, p) {
		return
	}
	v, e := s.profiles.ListCompensations(r.Context(), p.TenantID)
	s.writeProfile(w, v, e)
}
func (s *Server) getCompensation(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if !s.requireProfiles(w) || !requireTenantScope(w, p) {
		return
	}
	v, e := s.profiles.GetCompensation(r.Context(), p.TenantID, strings.ToLower(strings.TrimSpace(r.PathValue("person_id"))))
	s.writeProfile(w, v, e)
}
func (s *Server) upsertCompensation(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if !s.requireProfiles(w) || !requireTenantScope(w, p) {
		return
	}
	var in domain.UpsertCompensationProfileInput
	if !decodeJSON(w, r, &in) {
		return
	}
	in.PersonID = strings.ToLower(strings.TrimSpace(r.PathValue("person_id")))
	v, e := s.profiles.UpsertCompensation(r.Context(), actorFrom(p), in)
	s.writeProfile(w, v, e)
}
func (s *Server) listPayments(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if !s.requireProfiles(w) || !requireTenantScope(w, p) {
		return
	}
	v, e := s.profiles.ListPayments(r.Context(), p.TenantID)
	s.writeProfile(w, v, e)
}
func (s *Server) getPayment(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if !s.requireProfiles(w) || !requireTenantScope(w, p) {
		return
	}
	v, e := s.profiles.GetPayment(r.Context(), p.TenantID, strings.ToLower(strings.TrimSpace(r.PathValue("person_id"))))
	s.writeProfile(w, v, e)
}
func (s *Server) upsertPayment(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if !s.requireProfiles(w) || !requireTenantScope(w, p) {
		return
	}
	var in domain.UpsertPaymentProfileInput
	if !decodeJSON(w, r, &in) {
		return
	}
	in.PersonID = strings.ToLower(strings.TrimSpace(r.PathValue("person_id")))
	v, e := s.profiles.UpsertPayment(r.Context(), actorFrom(p), in)
	s.writeProfile(w, v, e)
}
func (s *Server) paymentHistory(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if !s.requireProfiles(w) || !requireTenantScope(w, p) {
		return
	}
	v, e := s.profiles.ListPaymentHistory(r.Context(), p.TenantID, strings.ToLower(strings.TrimSpace(r.PathValue("person_id"))))
	s.writeProfile(w, v, e)
}
func (s *Server) sensitivePayment(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if !s.requireProfiles(w) {
		return
	}
	person := strings.ToLower(strings.TrimSpace(r.PathValue("person_id")))
	masked, e := s.profiles.GetPayment(r.Context(), p.TenantID, person)
	if e != nil {
		s.fail(w, e)
		return
	}
	if p.ScopeKind != "resources" || !p.CanResourceType("payment_profile", masked.ID) {
		writeProblem(w, 403, "SCOPE_DENIED", "完整账号读取需要精确支付档案范围", false)
		return
	}
	var in domain.SensitiveAccountReadInput
	if !decodeJSON(w, r, &in) {
		return
	}
	v, e := s.profiles.ReadSensitivePayment(r.Context(), actorFrom(p), person, in.Purpose)
	s.writeProfile(w, v, e)
}
func (s *Server) listSchemes(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if !s.requireProfiles(w) || !requireTenantScope(w, p) {
		return
	}
	v, e := s.profiles.ListSchemes(r.Context(), p.TenantID)
	s.writeProfile(w, v, e)
}
func (s *Server) createCustomScheme(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if !s.requireProfiles(w) || !requireTenantScope(w, p) {
		return
	}
	var in domain.CreateCustomSchemeInput
	if !decodeJSON(w, r, &in) {
		return
	}
	v, e := s.profiles.CreateCustomScheme(r.Context(), actorFrom(p), in)
	s.writeProfileStatus(w, 201, v, e)
}
func (s *Server) reviseCustomScheme(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if !s.requireProfiles(w) || !requireTenantScope(w, p) {
		return
	}
	var in domain.ReviseCustomSchemeInput
	if !decodeJSON(w, r, &in) {
		return
	}
	v, e := s.profiles.ReviseCustomScheme(r.Context(), actorFrom(p), strings.ToLower(strings.TrimSpace(r.PathValue("scheme_id"))), in)
	s.writeProfile(w, v, e)
}
func (s *Server) writeProfile(w http.ResponseWriter, value any, err error) {
	s.writeProfileStatus(w, 200, value, err)
}
func (s *Server) writeProfileStatus(w http.ResponseWriter, status int, value any, err error) {
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, status, value)
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }
func (s *Server) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]string{"service": "payroll", "status": "live"})
}
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.useCases.Ping(r.Context()); err != nil {
		writeProblem(w, 503, "NOT_READY", "数据库暂不可用", true)
		return
	}
	writeJSON(w, 200, map[string]string{"service": "payroll", "status": "ready"})
}
func (s *Server) protected(action string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stripUntrustedIdentityHeaders(r.Header)
		principal, err := s.authenticator.Authenticate(r)
		if err != nil {
			writeProblem(w, 401, "UNAUTHENTICATED", "需要有效访问凭证", false)
			return
		}
		if !principal.Can(action) {
			writeProblem(w, 403, "CAPABILITY_DENIED", "缺少 "+action, false)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, principal)))
	})
}
func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if p.ScopeKind != "tenant" {
		writeProblem(w, 403, "SCOPE_DENIED", "新建工资批次需要租户范围", false)
		return
	}
	var input domain.CreateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.Create(r.Context(), actorFrom(p), input)
	s.writeRun(w, 201, value, p, err)
}
func (s *Server) update(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	id := strings.ToLower(strings.TrimSpace(r.PathValue("run_id")))
	if !p.CanResource(id) {
		writeProblem(w, 403, "SCOPE_DENIED", "凭证范围不包含该工资批次", false)
		return
	}
	var input domain.UpdateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.Update(r.Context(), actorFrom(p), id, input)
	s.writeRun(w, 200, value, p, err)
}
func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	id := strings.ToLower(strings.TrimSpace(r.PathValue("run_id")))
	if !p.CanResource(id) {
		writeProblem(w, 403, "SCOPE_DENIED", "凭证范围不包含该工资批次", false)
		return
	}
	value, err := s.useCases.Get(r.Context(), p.TenantID, id)
	s.writeRun(w, 200, value, p, err)
}

func (s *Server) reference(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	id := strings.ToLower(strings.TrimSpace(r.PathValue("run_id")))
	if p.ScopeKind != "resources" || !p.CanResource(id) {
		writeProblem(w, 403, "SCOPE_DENIED", "引用读取只接受精确工资批次范围", false)
		return
	}
	value, err := s.useCases.Get(r.Context(), p.TenantID, id)
	if err != nil {
		s.fail(w, err)
		return
	}
	value.AllowedActions = nil
	writeJSON(w, 200, value)
}
func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if p.ScopeKind != "tenant" {
		writeProblem(w, 403, "SCOPE_DENIED", "列表查询需要租户范围", false)
		return
	}
	page, err := s.useCases.List(r.Context(), p.TenantID)
	if err != nil {
		s.fail(w, err)
		return
	}
	for i := range page.Items {
		page.Items[i].AllowedActions = allowedActions(page.Items[i], p)
	}
	writeJSON(w, 200, page)
}
func (s *Server) writeRun(w http.ResponseWriter, status int, value domain.Run, p auth.Principal, err error) {
	if err != nil {
		s.fail(w, err)
		return
	}
	value.AllowedActions = allowedActions(value, p)
	writeJSON(w, status, value)
}
func (s *Server) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeProblem(w, 404, "PAYROLL_RUN_NOT_FOUND", err.Error(), false)
	case errors.Is(err, domain.ErrRevisionConflict):
		writeProblem(w, 409, "PAYROLL_RUN_REVISION_CONFLICT", err.Error(), true)
	case errors.Is(err, domain.ErrCommandConflict):
		writeProblem(w, 409, "COMMAND_PAYLOAD_CONFLICT", err.Error(), false)
	case errors.Is(err, domain.ErrProfileNotFound):
		writeProblem(w, 404, "PAYROLL_PROFILE_NOT_FOUND", err.Error(), false)
	case errors.Is(err, domain.ErrReferenceInvalid):
		writeProblem(w, 409, "PERSON_REFERENCE_INVALID", err.Error(), false)
	case errors.Is(err, domain.ErrReferenceUnavailable):
		writeProblem(w, 503, "PERSON_REFERENCE_UNAVAILABLE", err.Error(), true)
	case errors.Is(err, domain.ErrRunLocked):
		writeProblem(w, 409, "PAYROLL_RUN_LOCKED", err.Error(), false)
	case errors.Is(err, domain.ErrPreviewStale):
		writeProblem(w, 409, "PAYROLL_PREVIEW_STALE", err.Error(), true)
	case errors.Is(err, domain.ErrPaymentUnknown):
		writeProblem(w, 409, "PAYROLL_PAYMENT_UNKNOWN", err.Error(), true)
	case errors.Is(err, domain.ErrPaymentAccount):
		writeProblem(w, 422, "PAYMENT_ACCOUNT_MISSING", err.Error(), false)
	case errors.Is(err, domain.ErrPaymentNotReady):
		writeProblem(w, 422, "PAYROLL_PAYMENT_NOT_READY", err.Error(), false)
	case errors.Is(err, domain.ErrOverageConfirmation):
		writeProblem(w, 422, "PAYROLL_OVERAGE_CONFIRMATION_REQUIRED", err.Error(), false)
	case errors.Is(err, domain.ErrPaymentFailed):
		writeProblem(w, 502, "PAYROLL_PAYMENT_FAILED", err.Error(), true)
	case strings.Contains(err.Error(), "invalid"):
		writeProblem(w, 400, "INVALID_REQUEST", err.Error(), false)
	default:
		writeProblem(w, 503, "DEPENDENCY_UNAVAILABLE", "服务暂不可用", true)
	}
}
func allowedActions(value domain.Run, p auth.Principal) []string {
	var actions []string
	if p.Can(auth.ActionTenantRead) {
		actions = append(actions, "read")
	}
	if p.Can(auth.ActionRunManage) && value.Status == "draft" {
		actions = append(actions, "update")
	}
	return actions
}
func actorFrom(p auth.Principal) domain.Actor {
	kind := "iam_subject"
	if p.Provider == "template" {
		kind = "template_subject"
	}
	return domain.Actor{ID: p.SubjectID, TenantID: p.TenantID, DisplayName: p.DisplayName, Kind: kind, CredentialID: p.CredentialID}
}
func principalFrom(r *http.Request) auth.Principal {
	return r.Context().Value(principalKey{}).(auth.Principal)
}
func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	reader := http.MaxBytesReader(w, r.Body, maxRequestBytes)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeProblem(w, 400, "INVALID_REQUEST", "JSON 请求无效", false)
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		writeProblem(w, 400, "INVALID_REQUEST", "请求只能包含一个 JSON 值", false)
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeProblem(w http.ResponseWriter, status int, code, detail string, retryable bool) {
	writeJSON(w, status, map[string]any{"code": code, "detail": detail, "retryable": retryable})
}
func stripUntrustedIdentityHeaders(header http.Header) {
	for name := range header {
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "x-oms-") || strings.HasPrefix(lower, "x-auth-") || strings.HasPrefix(lower, "x-user-") || lower == "x-tenant-id" {
			header.Del(name)
		}
	}
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
