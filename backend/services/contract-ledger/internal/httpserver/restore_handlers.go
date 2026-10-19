package httpserver

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"omsbizfoundry/contract-ledger/internal/ledgerrestore"
)

type restorePageData struct {
	commonPageData
	CommandID string
	Error     string
}

func (s *Server) showRestore(w http.ResponseWriter, r *http.Request) {
	commandID, err := ledgerrestore.NewCommandID()
	if err != nil {
		http.Error(w, "无法生成恢复请求标识", http.StatusInternalServerError)
		return
	}
	s.renderRestorePage(w, r, http.StatusOK, restorePageData{CommandID: commandID})
}

func (s *Server) restoreContractLedger(w http.ResponseWriter, r *http.Request) {
	const multipartOverhead = int64(1 << 20)
	r.Body = http.MaxBytesReader(w, r.Body, ledgerrestore.MaxArchiveBytes+multipartOverhead)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			http.Error(w, "恢复归档超过大小限制", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "恢复请求格式无效", http.StatusBadRequest)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	commandID := strings.TrimSpace(r.PostForm.Get("command_id"))
	page := restorePageData{CommandID: commandID}
	if !validCSRFToken(r) {
		page.Error = "请求校验失败，请刷新页面后重试"
		s.renderRestorePage(w, r, http.StatusForbidden, page)
		return
	}
	if r.PostForm.Get("confirm_empty") != "yes" {
		page.Error = "必须确认目标业务库为空并允许写入"
		s.renderRestorePage(w, r, http.StatusUnprocessableEntity, page)
		return
	}
	file, _, err := r.FormFile("archive")
	if err != nil {
		page.Error = "必须选择 Contract Ledger ZIP 归档"
		s.renderRestorePage(w, r, http.StatusUnprocessableEntity, page)
		return
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, ledgerrestore.MaxArchiveBytes+1))
	if err != nil {
		http.Error(w, "读取恢复归档失败", http.StatusBadRequest)
		return
	}
	if int64(len(content)) > ledgerrestore.MaxArchiveBytes {
		http.Error(w, "恢复归档超过大小限制", http.StatusRequestEntityTooLarge)
		return
	}
	result, err := s.ledgerRestores.Restore(r.Context(), ledgerrestore.Input{
		CommandID: commandID, Archive: content, Actor: mustPrincipal(r).Actor(),
	})
	if err != nil {
		status, message := restoreHTTPError(err)
		if status == http.StatusInternalServerError {
			http.Error(w, message, status)
			return
		}
		page.Error = message
		s.renderRestorePage(w, r, status, page)
		return
	}
	query := url.Values{"restored": {"1"}, "missing_evidence": {strconv.Itoa(result.MissingEvidenceVersions)}}
	http.Redirect(w, r, "/contracts/"+result.ContractID+"?"+query.Encode(), http.StatusSeeOther)
}

func restoreHTTPError(err error) (int, string) {
	var validation *ledgerrestore.ValidationError
	var unsupportedRelations *ledgerrestore.UnsupportedRelationsError
	var tooLarge *ledgerrestore.TooLargeError
	var destinationNotEmpty *ledgerrestore.DestinationNotEmptyError
	var destinationConflict *ledgerrestore.DestinationConflictError
	var idempotency *ledgerrestore.IdempotencyConflictError
	switch {
	case errors.As(err, &tooLarge):
		return http.StatusRequestEntityTooLarge, tooLarge.Error()
	case errors.As(err, &validation), errors.As(err, &unsupportedRelations):
		return http.StatusUnprocessableEntity, err.Error()
	case errors.As(err, &destinationNotEmpty), errors.As(err, &destinationConflict), errors.As(err, &idempotency):
		return http.StatusConflict, err.Error()
	default:
		return http.StatusInternalServerError, "合同账簿恢复失败"
	}
}

func (s *Server) renderRestorePage(w http.ResponseWriter, r *http.Request, status int, data restorePageData) {
	csrfToken, err := s.ensureCSRFToken(w, r)
	if err != nil {
		http.Error(w, "无法生成请求校验信息", http.StatusInternalServerError)
		return
	}
	data.commonPageData = commonData(mustPrincipal(r), csrfToken, s.publicBasePath, s.externalIdentity)
	s.renderTemplate(w, status, s.restorePage, "restore.html", data)
}
