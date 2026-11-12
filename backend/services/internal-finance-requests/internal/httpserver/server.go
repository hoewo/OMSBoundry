package httpserver

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"omsbizfoundry/internal-finance-requests/internal/auth"
	"omsbizfoundry/internal-finance-requests/internal/finance"
	"strconv"
	"strings"
	"time"
)

const maxRequestBytes = 1 << 20

type UseCases interface {
	Ping(context.Context) error
	Create(context.Context, finance.Actor, finance.CreateInput) (finance.Request, error)
	Update(context.Context, finance.Actor, string, finance.UpdateInput) (finance.Request, error)
	Submit(context.Context, finance.Actor, string, finance.SubmitInput) (finance.Request, error)
	Withdraw(context.Context, finance.Actor, string, finance.WithdrawInput) (finance.Request, error)
	Restore(context.Context, finance.Actor, string, finance.RestoreInput) (finance.Request, error)
	DeleteDraft(context.Context, finance.Actor, string, finance.DeleteDraftInput) (finance.Request, error)
	ConfirmPayment(context.Context, finance.Actor, string, finance.ConfirmPaymentInput) (finance.Request, error)
	ConfirmPayments(context.Context, finance.Actor, finance.BatchPaymentInput) finance.BatchPaymentResult
	Get(context.Context, string, string) (finance.Request, error)
	GetSensitive(context.Context, finance.Actor, string, string) (finance.Request, error)
	List(context.Context, string, finance.ListFilter) (finance.Page, error)
	ListCategories(context.Context, string) (finance.CategoryPage, error)
	CreateCategory(context.Context, finance.Actor, finance.CategoryCreateInput) (finance.ExpenseCategory, error)
	UpdateCategory(context.Context, finance.Actor, string, finance.CategoryUpdateInput) (finance.ExpenseCategory, error)
	IsEvidenceReferenced(context.Context, string, string, int) (bool, error)
}
type Server struct {
	useCases      UseCases
	authenticator auth.Authenticator
	handler       http.Handler
}
type principalKey struct{}

func New(useCases UseCases, authenticator auth.Authenticator) (*Server, error) {
	if useCases == nil || authenticator == nil {
		return nil, fmt.Errorf("use cases and authenticator are required")
	}
	s := &Server{useCases: useCases, authenticator: authenticator}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", s.live)
	mux.HandleFunc("GET /health/ready", s.ready)
	mux.Handle("POST /api/v1/finance-requests", s.protected(auth.ActionSelfManage, http.HandlerFunc(s.create)))
	mux.Handle("GET /api/v1/finance-requests", s.protectedAny([]string{auth.ActionSelfRead, auth.ActionTenantRead}, http.HandlerFunc(s.list)))
	mux.Handle("GET /api/v1/finance-requests/{request_id}", s.protectedAny([]string{auth.ActionSelfRead, auth.ActionTenantRead}, http.HandlerFunc(s.get)))
	mux.Handle("PUT /api/v1/finance-requests/{request_id}", s.protected(auth.ActionSelfManage, http.HandlerFunc(s.update)))
	mux.Handle("DELETE /api/v1/finance-requests/{request_id}", s.protected(auth.ActionSelfManage, http.HandlerFunc(s.deleteDraft)))
	mux.Handle("POST /api/v1/finance-requests/{request_id}/submit", s.protected(auth.ActionSelfManage, http.HandlerFunc(s.submit)))
	mux.Handle("POST /api/v1/finance-requests/{request_id}/withdraw", s.protected(auth.ActionSelfManage, http.HandlerFunc(s.withdraw)))
	mux.Handle("POST /api/v1/finance-requests/{request_id}/payments", s.protected(auth.ActionPaymentConfirm, http.HandlerFunc(s.confirmPayment)))
	mux.Handle("POST /api/v1/finance-payments/batch", s.protected(auth.ActionPaymentBatchConfirm, http.HandlerFunc(s.confirmPayments)))
	mux.Handle("GET /api/v1/finance-requests/{request_id}/sensitive-account", s.protected(auth.ActionSensitiveAccountRead, http.HandlerFunc(s.getSensitive)))
	mux.Handle("GET /api/v1/finance-request-exports", s.protected(auth.ActionExport, http.HandlerFunc(s.exportRequests)))
	mux.Handle("GET /api/v1/finance-request-sensitive-exports", s.protected(auth.ActionSensitiveExport, http.HandlerFunc(s.exportSensitiveRequests)))
	mux.Handle("GET /api/v1/references/finance-requests/{request_id}", s.protected(auth.ActionReferenceRead, http.HandlerFunc(s.reference)))
	mux.Handle("GET /api/v1/references/evidence-files/{file_id}/versions/{version}/usage", s.protected(auth.ActionReferenceRead, http.HandlerFunc(s.evidenceReferenceUsage)))
	mux.Handle("GET /api/v1/expense-categories", s.protectedAny([]string{auth.ActionSelfRead, auth.ActionSelfManage, auth.ActionTenantRead}, http.HandlerFunc(s.listCategories)))
	mux.Handle("POST /api/v1/expense-categories", s.protected(auth.ActionCategoryManage, http.HandlerFunc(s.createCategory)))
	mux.Handle("PUT /api/v1/expense-categories/{category_id}", s.protected(auth.ActionCategoryManage, http.HandlerFunc(s.updateCategory)))
	mux.Handle("POST /api/v1/open-api/reimbursements", s.protected(auth.ActionSelfManage, http.HandlerFunc(s.openCreateReimbursement)))
	mux.Handle("GET /api/v1/open-api/reimbursements", s.protected(auth.ActionSelfRead, http.HandlerFunc(s.openListReimbursements)))
	mux.Handle("GET /api/v1/open-api/reimbursements/{request_id}", s.protected(auth.ActionSelfRead, http.HandlerFunc(s.openGetReimbursement)))
	mux.Handle("PUT /api/v1/open-api/reimbursements/{request_id}", s.protected(auth.ActionSelfManage, http.HandlerFunc(s.openUpdateReimbursement)))
	mux.Handle("POST /api/v1/open-api/reimbursements/{request_id}/submit", s.protected(auth.ActionSelfManage, http.HandlerFunc(s.openSubmitReimbursement)))
	mux.Handle("POST /api/v1/open-api/reimbursements/{request_id}/withdraw", s.protected(auth.ActionSelfManage, http.HandlerFunc(s.openWithdrawReimbursement)))
	mux.Handle("POST /api/v1/open-api/reimbursements/{request_id}/restore", s.protected(auth.ActionSelfManage, http.HandlerFunc(s.openRestoreReimbursement)))
	mux.Handle("DELETE /api/v1/open-api/reimbursements/{request_id}", s.protected(auth.ActionSelfManage, http.HandlerFunc(s.openDeleteReimbursement)))
	// Gateway strips the public, versioned /internal-finance/v1 prefix.
	mux.Handle("POST /finance-requests", s.protected(auth.ActionSelfManage, http.HandlerFunc(s.create)))
	mux.Handle("GET /finance-requests", s.protectedAny([]string{auth.ActionSelfRead, auth.ActionTenantRead}, http.HandlerFunc(s.list)))
	mux.Handle("GET /finance-requests/{request_id}", s.protectedAny([]string{auth.ActionSelfRead, auth.ActionTenantRead}, http.HandlerFunc(s.get)))
	mux.Handle("PUT /finance-requests/{request_id}", s.protected(auth.ActionSelfManage, http.HandlerFunc(s.update)))
	mux.Handle("DELETE /finance-requests/{request_id}", s.protected(auth.ActionSelfManage, http.HandlerFunc(s.deleteDraft)))
	mux.Handle("POST /finance-requests/{request_id}/submit", s.protected(auth.ActionSelfManage, http.HandlerFunc(s.submit)))
	mux.Handle("POST /finance-requests/{request_id}/withdraw", s.protected(auth.ActionSelfManage, http.HandlerFunc(s.withdraw)))
	mux.Handle("POST /finance-requests/{request_id}/payments", s.protected(auth.ActionPaymentConfirm, http.HandlerFunc(s.confirmPayment)))
	mux.Handle("POST /finance-payments/batch", s.protected(auth.ActionPaymentBatchConfirm, http.HandlerFunc(s.confirmPayments)))
	mux.Handle("GET /finance-requests/{request_id}/sensitive-account", s.protected(auth.ActionSensitiveAccountRead, http.HandlerFunc(s.getSensitive)))
	mux.Handle("GET /finance-request-exports", s.protected(auth.ActionExport, http.HandlerFunc(s.exportRequests)))
	mux.Handle("GET /finance-request-sensitive-exports", s.protected(auth.ActionSensitiveExport, http.HandlerFunc(s.exportSensitiveRequests)))
	mux.Handle("GET /references/finance-requests/{request_id}", s.protected(auth.ActionReferenceRead, http.HandlerFunc(s.reference)))
	mux.Handle("GET /expense-categories", s.protectedAny([]string{auth.ActionSelfRead, auth.ActionSelfManage, auth.ActionTenantRead}, http.HandlerFunc(s.listCategories)))
	mux.Handle("POST /expense-categories", s.protected(auth.ActionCategoryManage, http.HandlerFunc(s.createCategory)))
	mux.Handle("PUT /expense-categories/{category_id}", s.protected(auth.ActionCategoryManage, http.HandlerFunc(s.updateCategory)))
	s.handler = securityHeaders(mux)
	return s, nil
}

type openReimbursementWrite struct {
	ExpectedRevision int64                       `json:"expected_revision,omitempty"`
	Title            string                      `json:"title"`
	Lines            []finance.Line              `json:"lines"`
	Payee            openReimbursementPayee      `json:"payee"`
	Attribution      finance.AttributionSnapshot `json:"attribution,omitempty"`
}

type openReimbursementPayee struct {
	BankName      string `json:"bank_name"`
	AccountNumber string `json:"account_number"`
}

func (s *Server) openCreateReimbursement(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	key, command, ok := openCommand(w, r, p, "reimbursements.create")
	if !ok || !requireOpenPerson(w, p) {
		return
	}
	var input openReimbursementWrite
	if !decodeJSON(w, r, &input) {
		return
	}
	prepareOpenLines(input.Lines, p, key, "reimbursements.create")
	details, ok := openDetails(w, p, input)
	if !ok {
		return
	}
	value, err := s.useCases.Create(r.Context(), actorFrom(p), finance.CreateInput{CommandID: command, RequestType: "reimbursement", Title: input.Title, AmountMinor: sumOpenLines(input.Lines), Currency: "CNY", Details: details})
	s.writeOpenRequest(w, http.StatusCreated, value, p, err)
}

func (s *Server) openUpdateReimbursement(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	key, command, ok := openCommand(w, r, p, "reimbursements.update")
	if !ok || !requireOpenPerson(w, p) {
		return
	}
	var input openReimbursementWrite
	if !decodeJSON(w, r, &input) {
		return
	}
	prepareOpenLines(input.Lines, p, key, "reimbursements.update")
	details, ok := openDetails(w, p, input)
	if !ok {
		return
	}
	value, err := s.useCases.Update(r.Context(), actorFrom(p), strings.ToLower(strings.TrimSpace(r.PathValue("request_id"))), finance.UpdateInput{CommandID: command, ExpectedRevision: input.ExpectedRevision, Title: input.Title, AmountMinor: sumOpenLines(input.Lines), Details: details})
	s.writeOpenRequest(w, http.StatusOK, value, p, err)
}

func (s *Server) openSubmitReimbursement(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	_, command, ok := openCommand(w, r, p, "reimbursements.submit")
	if !ok || !requireOpenPerson(w, p) {
		return
	}
	var input struct {
		ExpectedRevision               int64 `json:"expected_revision"`
		SuspectedDuplicateAcknowledged bool  `json:"suspected_duplicate_acknowledged"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.Submit(r.Context(), actorFrom(p), strings.ToLower(strings.TrimSpace(r.PathValue("request_id"))), finance.SubmitInput{CommandID: command, ExpectedRevision: input.ExpectedRevision, SuspectedDuplicateAcknowledged: input.SuspectedDuplicateAcknowledged})
	s.writeOpenRequest(w, http.StatusOK, value, p, err)
}

func (s *Server) openWithdrawReimbursement(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	_, command, ok := openCommand(w, r, p, "reimbursements.withdraw")
	if !ok || !requireOpenPerson(w, p) {
		return
	}
	var input struct {
		ExpectedRevision int64  `json:"expected_revision"`
		Reason           string `json:"reason"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.Withdraw(r.Context(), actorFrom(p), strings.ToLower(strings.TrimSpace(r.PathValue("request_id"))), finance.WithdrawInput{CommandID: command, ExpectedRevision: input.ExpectedRevision, Reason: input.Reason})
	s.writeOpenRequest(w, http.StatusOK, value, p, err)
}

func (s *Server) openRestoreReimbursement(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	_, command, ok := openCommand(w, r, p, "reimbursements.restore")
	if !ok || !requireOpenPerson(w, p) {
		return
	}
	value, err := s.useCases.Restore(r.Context(), actorFrom(p), strings.ToLower(strings.TrimSpace(r.PathValue("request_id"))), finance.RestoreInput{CommandID: command})
	s.writeOpenRequest(w, http.StatusOK, value, p, err)
}

func (s *Server) openDeleteReimbursement(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	_, command, ok := openCommand(w, r, p, "reimbursements.delete")
	if !ok || !requireOpenPerson(w, p) {
		return
	}
	var input struct {
		Confirmation string `json:"confirmation"`
		Reason       string `json:"reason"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.DeleteDraft(r.Context(), actorFrom(p), strings.ToLower(strings.TrimSpace(r.PathValue("request_id"))), finance.DeleteDraftInput{CommandID: command, Confirmation: input.Confirmation, Reason: input.Reason})
	if err != nil {
		s.fail(w, err)
		return
	}
	value.AllowedActions = nil
	w.Header().Set("ETag", requestETag(value))
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) openGetReimbursement(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if !requireOpenPerson(w, p) {
		return
	}
	value, err := s.useCases.Get(r.Context(), p.TenantID, strings.ToLower(strings.TrimSpace(r.PathValue("request_id"))))
	if err == nil && (value.ApplicantSubjectID != p.SubjectID || value.RequestType != "reimbursement") {
		err = finance.ErrNotFound
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	value.AllowedActions = allowedActions(value, p)
	value.Details.Duplicate.MatchedRequestIDs = nil
	writeETagJSON(w, r, requestETag(value), value)
}

func (s *Server) openListReimbursements(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if !requireOpenPerson(w, p) {
		return
	}
	filter, err := openListFilter(r)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false)
		return
	}
	filter.ApplicantSubjectID = p.SubjectID
	filter.RequestType = "reimbursement"
	page, err := s.useCases.List(r.Context(), p.TenantID, filter)
	if err != nil {
		s.fail(w, err)
		return
	}
	items := page.Items[:0]
	for _, value := range page.Items {
		if value.RequestType != "reimbursement" {
			continue
		}
		value.AllowedActions = allowedActions(value, p)
		value.Details.Duplicate.MatchedRequestIDs = nil
		items = append(items, value)
	}
	page.Items = items
	raw, _ := json.Marshal(page)
	digest := sha256.Sum256(raw)
	writeETagJSON(w, r, fmt.Sprintf("\"reimbursements-%x\"", digest[:16]), page)
}

func openDetails(w http.ResponseWriter, p auth.Principal, input openReimbursementWrite) (finance.DetailSnapshot, bool) {
	for _, line := range input.Lines {
		if line.Invoice == nil || len(line.Evidence) == 0 {
			writeProblem(w, http.StatusUnprocessableEntity, "INVOICE_REQUIRED", "每条报销明细必须包含发票信息和附件", false)
			return finance.DetailSnapshot{}, false
		}
	}
	person := finance.ReferenceSnapshot{ID: p.PersonID, Revision: p.PersonRevision, Digest: p.PersonDigest, DisplayName: p.PersonDisplayName}
	return finance.DetailSnapshot{Applicant: person, Payee: finance.PayeeSnapshot{Kind: "person", Reference: person, DisplayName: p.PersonDisplayName, BankName: input.Payee.BankName, AccountNumber: input.Payee.AccountNumber}, Lines: input.Lines, Attribution: input.Attribution}, true
}

func requireOpenPerson(w http.ResponseWriter, p auth.Principal) bool {
	if p.Provider != "iam" || p.ScopeKind != "tenant" || p.PersonID == "" || p.PersonRevision < 1 || p.PersonDisplayName == "" || p.PersonDigest == "" {
		writeProblem(w, http.StatusForbidden, "PERSON_BINDING_REQUIRED", "当前凭证未绑定有效成员", false)
		return false
	}
	return true
}

func openCommand(w http.ResponseWriter, r *http.Request, p auth.Principal, action string) (string, string, bool) {
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(key) < 8 || len(key) > 128 {
		writeProblem(w, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "写请求必须提供 8 到 128 字符的 Idempotency-Key", false)
		return "", "", false
	}
	return key, deterministicUUID(p.TenantID + ":" + p.SubjectID + ":" + action + ":" + key), true
}

func prepareOpenLines(lines []finance.Line, p auth.Principal, key, action string) {
	for i := range lines {
		if strings.TrimSpace(lines[i].ID) == "" {
			lines[i].ID = deterministicUUID(p.TenantID + ":" + p.SubjectID + ":" + action + ":" + key + ":line:" + strconv.Itoa(i))
		}
	}
}

func deterministicUUID(value string) string {
	digest := sha256.Sum256([]byte(value))
	digest[6] = (digest[6] & 0x0f) | 0x50
	digest[8] = (digest[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", digest[0:4], digest[4:6], digest[6:8], digest[8:10], digest[10:16])
}

func sumOpenLines(lines []finance.Line) int64 {
	var total int64
	for _, line := range lines {
		total += line.AmountMinor
	}
	return total
}

func openListFilter(r *http.Request) (finance.ListFilter, error) {
	query := r.URL.Query()
	filter := finance.ListFilter{Status: strings.TrimSpace(query.Get("status")), SubmittedFrom: strings.TrimSpace(query.Get("submitted_from")), SubmittedTo: strings.TrimSpace(query.Get("submitted_to"))}
	if raw := strings.TrimSpace(query.Get("archived")); raw != "" {
		if raw != "include" {
			return finance.ListFilter{}, fmt.Errorf("archived 只接受 include")
		}
		filter.IncludeArchived = true
	}
	if raw := strings.TrimSpace(query.Get("updated_since")); raw != "" {
		value, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return finance.ListFilter{}, fmt.Errorf("updated_since 必须是 RFC3339 时间")
		}
		filter.UpdatedSince = &value
	}
	if raw := strings.TrimSpace(query.Get("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100 {
			return finance.ListFilter{}, fmt.Errorf("limit 必须为 1 到 100")
		}
		filter.Limit = value
	}
	if raw := strings.TrimSpace(query.Get("cursor")); raw != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			return finance.ListFilter{}, fmt.Errorf("cursor 无效")
		}
		var cursor struct {
			UpdatedAt time.Time `json:"updated_at"`
			ID        string    `json:"id"`
		}
		if json.Unmarshal(decoded, &cursor) != nil || cursor.UpdatedAt.IsZero() || cursor.ID == "" {
			return finance.ListFilter{}, fmt.Errorf("cursor 无效")
		}
		filter.BeforeUpdatedAt, filter.BeforeID = &cursor.UpdatedAt, cursor.ID
	}
	return filter, nil
}

func (s *Server) writeOpenRequest(w http.ResponseWriter, status int, value finance.Request, p auth.Principal, err error) {
	if err != nil {
		s.fail(w, err)
		return
	}
	value.AllowedActions = allowedActions(value, p)
	value.Details.Duplicate.MatchedRequestIDs = nil
	w.Header().Set("ETag", requestETag(value))
	writeJSON(w, status, value)
}

func requestETag(value finance.Request) string {
	return fmt.Sprintf("\"finance-%s-r%d\"", value.ID, value.Revision)
}

func writeETagJSON(w http.ResponseWriter, r *http.Request, etag string, value any) {
	w.Header().Set("ETag", etag)
	if strings.TrimSpace(r.Header.Get("If-None-Match")) == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }
func (s *Server) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]string{"service": "internal-finance-requests", "status": "live"})
}
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.useCases.Ping(r.Context()); err != nil {
		writeProblem(w, 503, "NOT_READY", "数据库暂不可用", true)
		return
	}
	writeJSON(w, 200, map[string]string{"service": "internal-finance-requests", "status": "ready"})
}

func (s *Server) evidenceReferenceUsage(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if p.ScopeKind != "tenant" {
		writeProblem(w, http.StatusForbidden, "SCOPE_DENIED", "引用保护查询需要租户范围", false)
		return
	}
	fileID := strings.ToLower(strings.TrimSpace(r.PathValue("file_id")))
	version, err := strconv.Atoi(strings.TrimSpace(r.PathValue("version")))
	if err != nil || version < 1 {
		writeProblem(w, http.StatusBadRequest, "INVALID_REQUEST", "文件版本无效", false)
		return
	}
	referenced, err := s.useCases.IsEvidenceReferenced(r.Context(), p.TenantID, fileID, version)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenant_id": p.TenantID, "file_id": fileID, "version": version, "referenced": referenced})
}

func (s *Server) protected(action string, next http.Handler) http.Handler {
	return s.protectedAny([]string{action}, next)
}
func (s *Server) protectedAny(actions []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stripUntrustedIdentityHeaders(r.Header)
		principal, err := s.authenticator.Authenticate(r)
		if err != nil {
			writeProblem(w, 401, "UNAUTHENTICATED", "需要有效访问凭证", false)
			return
		}
		allowed := false
		for _, action := range actions {
			allowed = allowed || principal.Can(action)
		}
		if !allowed {
			writeProblem(w, 403, "CAPABILITY_DENIED", "缺少所需能力", false)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, principal)))
	})
}
func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if p.ScopeKind != "tenant" {
		writeProblem(w, 403, "SCOPE_DENIED", "新建申请需要租户范围", false)
		return
	}
	var input finance.CreateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.Create(r.Context(), actorFrom(p), input)
	s.writeRequest(w, 201, value, p, err)
}
func (s *Server) update(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	id := strings.ToLower(strings.TrimSpace(r.PathValue("request_id")))
	if !p.CanResource(id) {
		writeProblem(w, 403, "SCOPE_DENIED", "凭证范围不包含该申请", false)
		return
	}
	var input finance.UpdateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.Update(r.Context(), actorFrom(p), id, input)
	s.writeRequest(w, 200, value, p, err)
}
func (s *Server) deleteDraft(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	id := strings.ToLower(strings.TrimSpace(r.PathValue("request_id")))
	if !p.CanResource(id) {
		writeProblem(w, 403, "SCOPE_DENIED", "凭证范围不包含该申请", false)
		return
	}
	var input finance.DeleteDraftInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.DeleteDraft(r.Context(), actorFrom(p), id, input)
	s.writeRequest(w, 200, value, p, err)
}
func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	id := strings.ToLower(strings.TrimSpace(r.PathValue("request_id")))
	if !p.CanResource(id) {
		writeProblem(w, 403, "SCOPE_DENIED", "凭证范围不包含该申请", false)
		return
	}
	value, err := s.useCases.Get(r.Context(), p.TenantID, id)
	if err == nil && !p.Can(auth.ActionTenantRead) && value.ApplicantSubjectID != p.SubjectID {
		err = finance.ErrNotFound
	}
	if err == nil && !p.Can(auth.ActionTenantRead) {
		value.Details.Duplicate.MatchedRequestIDs = nil
	}
	s.writeRequest(w, 200, value, p, err)
}

func (s *Server) submit(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	id := strings.ToLower(strings.TrimSpace(r.PathValue("request_id")))
	if !p.CanResource(id) {
		writeProblem(w, 403, "SCOPE_DENIED", "凭证范围不包含该申请", false)
		return
	}
	var input finance.SubmitInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.Submit(r.Context(), actorFrom(p), id, input)
	s.writeRequest(w, 200, value, p, err)
}
func (s *Server) withdraw(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	id := strings.ToLower(strings.TrimSpace(r.PathValue("request_id")))
	if !p.CanResource(id) {
		writeProblem(w, 403, "SCOPE_DENIED", "凭证范围不包含该申请", false)
		return
	}
	var input finance.WithdrawInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.Withdraw(r.Context(), actorFrom(p), id, input)
	s.writeRequest(w, 200, value, p, err)
}
func (s *Server) confirmPayment(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	id := strings.ToLower(strings.TrimSpace(r.PathValue("request_id")))
	if p.ScopeKind != "tenant" || !p.CanResource(id) {
		writeProblem(w, 403, "SCOPE_DENIED", "确认付款需要本租户范围", false)
		return
	}
	var input finance.ConfirmPaymentInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.ConfirmPayment(r.Context(), actorFrom(p), id, input)
	s.writeRequest(w, 200, value, p, err)
}
func (s *Server) confirmPayments(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if p.ScopeKind != "tenant" {
		writeProblem(w, 403, "SCOPE_DENIED", "批量确认付款需要本租户范围", false)
		return
	}
	var input finance.BatchPaymentInput
	if !decodeJSON(w, r, &input) {
		return
	}
	writeJSON(w, 200, s.useCases.ConfirmPayments(r.Context(), actorFrom(p), input))
}
func (s *Server) getSensitive(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	id := strings.ToLower(strings.TrimSpace(r.PathValue("request_id")))
	if p.ScopeKind != "tenant" || !p.CanResource(id) {
		writeProblem(w, 403, "SCOPE_DENIED", "完整账户读取需要本租户范围", false)
		return
	}
	purpose := strings.TrimSpace(r.URL.Query().Get("purpose"))
	if purpose == "" || len([]rune(purpose)) > 200 {
		writeProblem(w, 400, "SENSITIVE_ACCESS_PURPOSE_REQUIRED", "查看完整账户必须填写用途", false)
		return
	}
	value, err := s.useCases.GetSensitive(r.Context(), actorFrom(p), id, purpose)
	s.writeRequest(w, 200, value, p, err)
}

func (s *Server) exportRequests(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if p.ScopeKind != "tenant" {
		writeProblem(w, 403, "SCOPE_DENIED", "导出需要本租户范围", false)
		return
	}
	page, err := s.useCases.List(r.Context(), p.TenantID, finance.ListFilter{})
	if err != nil {
		s.fail(w, err)
		return
	}
	w.Header().Set("X-Content-Sensitivity", "masked")
	writeJSON(w, 200, map[string]any{"schema_version": "1", "sensitive": false, "generated_at": time.Now().UTC(), "items": page.Items})
}

func (s *Server) exportSensitiveRequests(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if p.ScopeKind != "tenant" {
		writeProblem(w, 403, "SCOPE_DENIED", "敏感导出需要本租户范围", false)
		return
	}
	purpose := strings.TrimSpace(r.URL.Query().Get("purpose"))
	if purpose == "" || len([]rune(purpose)) > 200 {
		writeProblem(w, 400, "SENSITIVE_ACCESS_PURPOSE_REQUIRED", "敏感导出必须填写用途", false)
		return
	}
	page, err := s.useCases.List(r.Context(), p.TenantID, finance.ListFilter{})
	if err != nil {
		s.fail(w, err)
		return
	}
	items := make([]finance.Request, 0, len(page.Items))
	for _, item := range page.Items {
		value, readErr := s.useCases.GetSensitive(r.Context(), actorFrom(p), item.ID, purpose)
		if readErr != nil {
			s.fail(w, readErr)
			return
		}
		items = append(items, value)
	}
	w.Header().Set("X-Content-Sensitivity", "full-bank-account")
	writeJSON(w, 200, map[string]any{"schema_version": "1", "sensitive": true, "purpose": purpose, "generated_at": time.Now().UTC(), "items": items})
}

func (s *Server) reference(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	id := strings.ToLower(strings.TrimSpace(r.PathValue("request_id")))
	if p.ScopeKind != "resources" || !p.CanResource(id) {
		writeProblem(w, 403, "SCOPE_DENIED", "引用读取只接受精确财务申请范围", false)
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
	filter := finance.ListFilter{}
	if !p.Can(auth.ActionTenantRead) {
		filter.ApplicantSubjectID = p.SubjectID
	}
	page, err := s.useCases.List(r.Context(), p.TenantID, filter)
	if err != nil {
		s.fail(w, err)
		return
	}
	readable := make(map[string]bool, len(page.Items))
	for i := range page.Items {
		readable[page.Items[i].ID] = true
	}
	for i := range page.Items {
		page.Items[i].AllowedActions = allowedActions(page.Items[i], p)
		if !p.Can(auth.ActionTenantRead) {
			retainReadableDuplicateIDs(&page.Items[i], readable)
		}
	}
	writeJSON(w, 200, page)
}

func (s *Server) listCategories(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if p.ScopeKind != "tenant" {
		writeProblem(w, 403, "SCOPE_DENIED", "费用类别列表需要租户范围", false)
		return
	}
	page, err := s.useCases.ListCategories(r.Context(), p.TenantID)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 200, page)
}

func (s *Server) createCategory(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if p.ScopeKind != "tenant" {
		writeProblem(w, 403, "SCOPE_DENIED", "费用类别管理需要租户范围", false)
		return
	}
	var input finance.CategoryCreateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.CreateCategory(r.Context(), actorFrom(p), input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 201, value)
}

func (s *Server) updateCategory(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	if p.ScopeKind != "tenant" {
		writeProblem(w, 403, "SCOPE_DENIED", "费用类别管理需要租户范围", false)
		return
	}
	var input finance.CategoryUpdateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.UpdateCategory(r.Context(), actorFrom(p), strings.ToLower(strings.TrimSpace(r.PathValue("category_id"))), input)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 200, value)
}
func (s *Server) writeRequest(w http.ResponseWriter, status int, value finance.Request, p auth.Principal, err error) {
	if err != nil {
		s.fail(w, err)
		return
	}
	value.AllowedActions = allowedActions(value, p)
	writeJSON(w, status, value)
}
func (s *Server) fail(w http.ResponseWriter, err error) {
	var duplicate *finance.DuplicateError
	if errors.As(err, &duplicate) {
		code, detail := "INVOICE_SUSPECTED_DUPLICATE", "发票疑似重复，确认核对后可使用原幂等键并显式确认"
		if duplicate.Strong {
			code, detail = "INVOICE_STRONG_DUPLICATE", "发票已在其他财务申请中提交"
		}
		writeProblemFields(w, 409, code, detail, false, map[string]any{"matched_request_ids": duplicate.MatchedRequestIDs})
		return
	}
	switch {
	case errors.Is(err, finance.ErrNotFound):
		writeProblem(w, 404, "FINANCE_REQUEST_NOT_FOUND", err.Error(), false)
	case errors.Is(err, finance.ErrRevisionConflict):
		writeProblem(w, 409, "FINANCE_REQUEST_REVISION_CONFLICT", err.Error(), true)
	case errors.Is(err, finance.ErrCommandConflict):
		writeProblem(w, 409, "COMMAND_PAYLOAD_CONFLICT", err.Error(), false)
	case errors.Is(err, finance.ErrStateConflict):
		writeProblem(w, 409, "FINANCE_REQUEST_STATE_CONFLICT", err.Error(), false)
	case errors.Is(err, finance.ErrPaymentUnknown):
		writeProblem(w, 503, "PAYMENT_OUTCOME_UNKNOWN", err.Error(), true)
	case errors.Is(err, finance.ErrPaymentRejected):
		writeProblem(w, 422, "PAYMENT_REJECTED", err.Error(), false)
	case errors.Is(err, finance.ErrDependency):
		writeProblem(w, 503, "DEPENDENCY_UNAVAILABLE", "依赖服务暂不可用", true)
	case strings.Contains(err.Error(), "invalid"):
		writeProblem(w, 422, "VALIDATION_FAILED", err.Error(), false)
	default:
		writeProblem(w, 503, "DEPENDENCY_UNAVAILABLE", "服务暂不可用", true)
	}
}

func retainReadableDuplicateIDs(value *finance.Request, readable map[string]bool) {
	matched := value.Details.Duplicate.MatchedRequestIDs[:0]
	for _, id := range value.Details.Duplicate.MatchedRequestIDs {
		if readable[id] {
			matched = append(matched, id)
		}
	}
	value.Details.Duplicate.MatchedRequestIDs = matched
}
func allowedActions(value finance.Request, p auth.Principal) []string {
	var actions []string
	if p.Can(auth.ActionTenantRead) || (p.Can(auth.ActionSelfRead) && value.ApplicantSubjectID == p.SubjectID) {
		actions = append(actions, "read")
	}
	if p.Can(auth.ActionSelfManage) && value.ApplicantSubjectID == p.SubjectID && value.ArchivedAt != nil && value.DeletedAt == nil {
		actions = append(actions, "restore", "delete")
	}
	if p.Can(auth.ActionSelfManage) && value.ApplicantSubjectID == p.SubjectID && value.Status == "draft" && value.ArchivedAt == nil && value.DeletedAt == nil {
		actions = append(actions, "update")
	}
	if p.Can(auth.ActionSelfManage) && value.ApplicantSubjectID == p.SubjectID && value.Status == "draft" && value.ArchivedAt == nil && value.DeletedAt == nil {
		actions = append(actions, "delete")
	}
	if p.Can(auth.ActionSelfManage) && value.ApplicantSubjectID == p.SubjectID && value.ArchivedAt == nil && value.DeletedAt == nil && (value.Status == "draft" || value.Status == "withdrawn") && value.Details.Duplicate.Status != "strong" {
		actions = append(actions, "submit")
	}
	if p.Can(auth.ActionSelfManage) && value.ApplicantSubjectID == p.SubjectID && value.ArchivedAt == nil && value.DeletedAt == nil && value.Status == "payable" && value.PaidMinor == 0 && value.PaymentUnknownCount == 0 {
		actions = append(actions, "withdraw")
	}
	if p.Can(auth.ActionPaymentConfirm) && value.PaymentUnknownCount == 0 && (value.Status == "payable" || value.Status == "partially_paid") {
		actions = append(actions, "confirm_payment")
	}
	if p.Can(auth.ActionSensitiveAccountRead) && value.Details.Payee.AccountLast4 != "" {
		actions = append(actions, "read_sensitive_account")
	}
	return actions
}
func actorFrom(p auth.Principal) finance.Actor {
	kind := "iam_subject"
	if p.Provider == "template" {
		kind = "template_subject"
	}
	return finance.Actor{ID: p.SubjectID, TenantID: p.TenantID, DisplayName: p.DisplayName, Kind: kind, CredentialID: p.CredentialID}
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
	writeProblemFields(w, status, code, detail, retryable, nil)
}
func writeProblemFields(w http.ResponseWriter, status int, code, detail string, retryable bool, fields map[string]any) {
	value := map[string]any{"code": code, "detail": detail, "retryable": retryable}
	for key, field := range fields {
		value[key] = field
	}
	writeJSON(w, status, value)
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
