package httpserver

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"html/template"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/allocation"
	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/cashevidence"
	"omsbizfoundry/money-flow-ledger/internal/cashtransaction"
	"omsbizfoundry/money-flow-ledger/internal/identity"
	"omsbizfoundry/money-flow-ledger/internal/internaltransfer"
	"omsbizfoundry/money-flow-ledger/internal/ledgerexport"
	"omsbizfoundry/money-flow-ledger/internal/obligation"
	"omsbizfoundry/money-flow-ledger/internal/operatingsummary"
	"omsbizfoundry/money-flow-ledger/internal/party"
	"omsbizfoundry/money-flow-ledger/internal/partymigration"
	"omsbizfoundry/money-flow-ledger/internal/payablesuggestion"
	"omsbizfoundry/money-flow-ledger/internal/receiptset"
	contract "omsbizfoundry/payable-suggestion-contract"
)

const (
	serviceName       = "money-flow-ledger"
	sessionCookieName = "money_flow_ledger_session"
	csrfCookieName    = "money_flow_ledger_csrf"
)

//go:embed templates/*.html
var templateFiles embed.FS

type ReadinessChecker interface {
	Ping(context.Context) error
}

type IdentityUseCases interface {
	Login(context.Context, string, string) (identity.LoginResult, error)
	ResolveSession(context.Context, string) (identity.Principal, bool, error)
	Logout(context.Context, string) error
}

type PartyUseCases interface {
	Create(context.Context, party.CreateInput) (party.CreateResult, error)
	List(context.Context) ([]party.Party, error)
}

type PartyRuntime interface {
	State(context.Context, string) (partymigration.RuntimeState, error)
	LocalPartyWritesAllowed(context.Context, string) (bool, error)
	ResolveReferences(context.Context, string, []string) ([]partymigration.RuntimePartyReference, error)
}

type DirectoryPartyProjector interface {
	ResolveOrProject(context.Context, string, string, int64, string, string) (partymigration.RuntimePartyReference, error)
}

type ObligationUseCases interface {
	CreateDraft(context.Context, obligation.CreateDraftInput) (obligation.CreateResult, error)
	Confirm(context.Context, obligation.ConfirmInput) (obligation.ConfirmResult, error)
	AdjustAmount(context.Context, obligation.AdjustAmountInput) (obligation.AdjustAmountResult, error)
	RecordRemission(context.Context, obligation.RecordRemissionInput) (obligation.RecordRemissionResult, error)
	Close(context.Context, obligation.CloseInput) (obligation.CloseResult, error)
	CurrentAging(context.Context, obligation.CurrentAgingInput) (obligation.AgingReport, error)
	HistoricalBalance(context.Context, obligation.HistoricalBalanceInput) (obligation.HistoricalSnapshot, error)
	RecordCashExpectation(context.Context, obligation.RecordCashExpectationInput) (obligation.RecordCashExpectationResult, error)
	CurrentCashForecast(context.Context) (obligation.CashForecastReport, error)
	CashExpectationHistory(context.Context, string) (obligation.CashExpectationHistory, error)
	List(context.Context) ([]obligation.Obligation, error)
	GetDetails(context.Context, string) (obligation.Details, bool, error)
}

type CashTransactionUseCases interface {
	Create(context.Context, cashtransaction.CreateInput) (cashtransaction.CreateResult, error)
	Reverse(context.Context, cashtransaction.ReverseInput) (cashtransaction.ReverseResult, error)
	List(context.Context) ([]cashtransaction.Transaction, error)
	Get(context.Context, string) (cashtransaction.Transaction, bool, error)
}

type CashEvidenceUseCases interface {
	Current(context.Context, string) (cashevidence.Snapshot, bool, error)
	Replace(context.Context, cashevidence.ReplaceInput) (cashevidence.Result, error)
}

type AllocationUseCases interface {
	Apply(context.Context, allocation.ApplyInput) (allocation.ApplyResult, error)
	Reverse(context.Context, allocation.ReverseInput) (allocation.ReverseResult, error)
	List(context.Context) ([]allocation.Allocation, error)
	ListForCashTransaction(context.Context, string) ([]allocation.Allocation, error)
	ListForObligation(context.Context, string) ([]allocation.Allocation, error)
	Get(context.Context, string) (allocation.Allocation, bool, error)
}

type OperatingSummaryUseCases interface {
	Current(context.Context) (operatingsummary.Report, error)
	Drilldown(context.Context, operatingsummary.DrilldownInput) (operatingsummary.Drilldown, error)
}

type LedgerExportUseCases interface {
	Export(context.Context, ledgerexport.Input) (ledgerexport.Artifact, error)
}

type InternalTransferUseCases interface {
	Create(context.Context, internaltransfer.CreateInput) (internaltransfer.CreateResult, error)
	List(context.Context) ([]internaltransfer.Transfer, error)
	Get(context.Context, string) (internaltransfer.Transfer, bool, error)
}

type PayableSuggestionUseCases interface {
	Submit(context.Context, audit.ActorRef, contract.Submission) (payablesuggestion.SubmitResult, error)
	Get(context.Context, string, string) (payablesuggestion.Intake, error)
	History(context.Context, string, string) ([]payablesuggestion.Revision, error)
	Decide(context.Context, string, string, audit.ActorRef, contract.Decision) (payablesuggestion.DecisionResult, error)
	Withdraw(context.Context, string, string, audit.ActorRef, contract.Withdrawal) (payablesuggestion.DecisionResult, error)
}

type ReceiptSetUseCases interface {
	Confirm(context.Context, receiptset.ConfirmInput) (receiptset.ConfirmResult, error)
	Get(context.Context, string, string) (receiptset.Set, error)
}

type Options struct {
	SessionCookieSecure  bool
	SessionCookieName    string
	ExternalIdentity     bool
	PublicBasePath       string
	ReferenceTenantID    string
	ReferenceAccessToken string
	PartyRuntime         PartyRuntime
	DirectoryParties     DirectoryPartyProjector
	PayableSuggestions   PayableSuggestionUseCases
	ReceiptSets          ReceiptSetUseCases
	CashEvidence         CashEvidenceUseCases
}

type Server struct {
	readiness              ReadinessChecker
	identities             IdentityUseCases
	parties                PartyUseCases
	partyRuntime           PartyRuntime
	directoryParties       DirectoryPartyProjector
	obligations            ObligationUseCases
	cashTransactions       CashTransactionUseCases
	allocations            AllocationUseCases
	operatingSummaries     OperatingSummaryUseCases
	ledgerExports          LedgerExportUseCases
	internalTransfers      InternalTransferUseCases
	payableSuggestions     PayableSuggestionUseCases
	receiptSets            ReceiptSetUseCases
	cashEvidence           CashEvidenceUseCases
	homePage               *template.Template
	loginPage              *template.Template
	partyPage              *template.Template
	obligationPage         *template.Template
	obligationDetail       *template.Template
	historicalBalancePage  *template.Template
	agingPage              *template.Template
	cashForecastPage       *template.Template
	operatingSummaryPage   *template.Template
	operatingDetailPage    *template.Template
	ledgerExportPage       *template.Template
	cashTransactionPage    *template.Template
	cashTransactionDetail  *template.Template
	allocationPage         *template.Template
	allocationDetail       *template.Template
	internalTransferPage   *template.Template
	internalTransferDetail *template.Template
	sessionCookieSecure    bool
	sessionCookieName      string
	externalIdentity       bool
	publicBasePath         string
	referenceTenantID      string
	referenceAccessToken   string
}

type principalContextKey struct{}

func New(
	readiness ReadinessChecker,
	identities IdentityUseCases,
	parties PartyUseCases,
	obligations ObligationUseCases,
	cashTransactions CashTransactionUseCases,
	allocations AllocationUseCases,
	operatingSummaries OperatingSummaryUseCases,
	ledgerExports LedgerExportUseCases,
	options Options,
	internalTransfers InternalTransferUseCases,
) (http.Handler, error) {
	if readiness == nil {
		return nil, errors.New("readiness checker is required")
	}
	if identities == nil {
		return nil, errors.New("identity use cases are required")
	}
	if parties == nil {
		return nil, errors.New("party use cases are required")
	}
	if obligations == nil {
		return nil, errors.New("obligation use cases are required")
	}
	if cashTransactions == nil {
		return nil, errors.New("cash transaction use cases are required")
	}
	if allocations == nil {
		return nil, errors.New("allocation use cases are required")
	}
	if operatingSummaries == nil {
		return nil, errors.New("operating summary use cases are required")
	}
	if ledgerExports == nil {
		return nil, errors.New("ledger export use cases are required")
	}
	if internalTransfers == nil {
		return nil, errors.New("internal transfer use cases are required")
	}
	if err := validateReferenceOptions(options.ReferenceTenantID, options.ReferenceAccessToken); err != nil {
		return nil, err
	}
	sessionName := options.SessionCookieName
	if sessionName == "" {
		sessionName = sessionCookieName
	}
	logoutPath := options.PublicBasePath + "/logout"
	if options.ExternalIdentity {
		logoutPath = "/gateway/logout"
	}
	parsePage := func(name string) (*template.Template, error) {
		return template.New(name).Funcs(template.FuncMap{
			"basePath":   func() string { return options.PublicBasePath },
			"logoutPath": func() string { return logoutPath },
			"can": func(principal identity.Principal, permission string) bool {
				return principal.Can(identity.Permission(permission))
			},
		}).ParseFS(templateFiles, "templates/"+name)
	}
	homePage, err := parsePage("home.html")
	if err != nil {
		return nil, err
	}
	loginPage, err := parsePage("login.html")
	if err != nil {
		return nil, err
	}
	partyPage, err := parsePage("parties.html")
	if err != nil {
		return nil, err
	}
	obligationPage, err := parsePage("obligations.html")
	if err != nil {
		return nil, err
	}
	obligationDetail, err := parsePage("obligation_detail.html")
	if err != nil {
		return nil, err
	}
	historicalBalancePage, err := parsePage("historical_balance.html")
	if err != nil {
		return nil, err
	}
	agingPage, err := parsePage("aging.html")
	if err != nil {
		return nil, err
	}
	cashForecastPage, err := parsePage("cash_forecast.html")
	if err != nil {
		return nil, err
	}
	operatingSummaryPage, err := parsePage("operating_summary.html")
	if err != nil {
		return nil, err
	}
	operatingDetailPage, err := parsePage("operating_summary_detail.html")
	if err != nil {
		return nil, err
	}
	ledgerExportPage, err := parsePage("ledger_exports.html")
	if err != nil {
		return nil, err
	}
	cashTransactionPage, err := parsePage("cash_transactions.html")
	if err != nil {
		return nil, err
	}
	cashTransactionDetail, err := parsePage("cash_transaction_detail.html")
	if err != nil {
		return nil, err
	}
	allocationPage, err := parsePage("allocations.html")
	if err != nil {
		return nil, err
	}
	allocationDetail, err := parsePage("allocation_detail.html")
	if err != nil {
		return nil, err
	}
	internalTransferPage, err := parsePage("internal_transfers.html")
	if err != nil {
		return nil, err
	}
	internalTransferDetail, err := parsePage("internal_transfer_detail.html")
	if err != nil {
		return nil, err
	}
	server := &Server{
		readiness:              readiness,
		identities:             identities,
		parties:                parties,
		partyRuntime:           options.PartyRuntime,
		directoryParties:       options.DirectoryParties,
		obligations:            obligations,
		cashTransactions:       cashTransactions,
		allocations:            allocations,
		operatingSummaries:     operatingSummaries,
		ledgerExports:          ledgerExports,
		internalTransfers:      internalTransfers,
		payableSuggestions:     options.PayableSuggestions,
		receiptSets:            options.ReceiptSets,
		cashEvidence:           options.CashEvidence,
		homePage:               homePage,
		loginPage:              loginPage,
		partyPage:              partyPage,
		obligationPage:         obligationPage,
		obligationDetail:       obligationDetail,
		historicalBalancePage:  historicalBalancePage,
		agingPage:              agingPage,
		cashForecastPage:       cashForecastPage,
		operatingSummaryPage:   operatingSummaryPage,
		operatingDetailPage:    operatingDetailPage,
		ledgerExportPage:       ledgerExportPage,
		cashTransactionPage:    cashTransactionPage,
		cashTransactionDetail:  cashTransactionDetail,
		allocationPage:         allocationPage,
		allocationDetail:       allocationDetail,
		internalTransferPage:   internalTransferPage,
		internalTransferDetail: internalTransferDetail,
		sessionCookieSecure:    options.SessionCookieSecure,
		sessionCookieName:      sessionName,
		externalIdentity:       options.ExternalIdentity,
		publicBasePath:         options.PublicBasePath,
		referenceTenantID:      options.ReferenceTenantID,
		referenceAccessToken:   options.ReferenceAccessToken,
	}

	protectedMux := http.NewServeMux()
	protectedMux.HandleFunc("GET /{$}", server.home)
	protectedMux.HandleFunc("GET /parties", server.showParties)
	protectedMux.Handle("POST /parties", server.requirePermission(identity.PermissionPartyRegister, http.HandlerFunc(server.createParty)))
	protectedMux.HandleFunc("GET /obligations", server.showObligations)
	protectedMux.Handle("POST /obligations", server.requirePermission(identity.PermissionObligationManage, http.HandlerFunc(server.createObligation)))
	protectedMux.HandleFunc("GET /obligations/{obligationID}", server.showObligationDetail)
	protectedMux.HandleFunc("GET /obligations/{obligationID}/history", server.showHistoricalObligationBalance)
	protectedMux.Handle("POST /obligations/{obligationID}/confirm", server.requirePermission(identity.PermissionObligationManage, http.HandlerFunc(server.confirmObligation)))
	protectedMux.Handle("POST /obligations/{obligationID}/adjust-amount", server.requirePermission(identity.PermissionObligationManage, http.HandlerFunc(server.adjustObligationAmount)))
	protectedMux.Handle("POST /obligations/{obligationID}/remissions", server.requirePermission(identity.PermissionObligationManage, http.HandlerFunc(server.recordObligationRemission)))
	protectedMux.Handle("POST /obligations/{obligationID}/close", server.requirePermission(identity.PermissionObligationManage, http.HandlerFunc(server.closeObligation)))
	protectedMux.HandleFunc("GET /aging", server.showCurrentAging)
	protectedMux.HandleFunc("GET /cash-forecast", server.showCashForecast)
	protectedMux.Handle("POST /cash-expectations", server.requirePermission(identity.PermissionObligationManage, http.HandlerFunc(server.recordCashExpectation)))
	protectedMux.HandleFunc("GET /operating-summary", server.showOperatingSummary)
	protectedMux.HandleFunc("GET /operating-summary/details", server.showOperatingSummaryDetail)
	protectedMux.HandleFunc("GET /ledger-exports", server.showLedgerExports)
	protectedMux.Handle("POST /ledger-exports", server.requirePermission(identity.PermissionExport, http.HandlerFunc(server.exportOwnerLedger)))
	protectedMux.HandleFunc("GET /cash-transactions", server.showCashTransactions)
	protectedMux.Handle("POST /cash-transactions", server.requirePermission(identity.PermissionCashRecord, http.HandlerFunc(server.createCashTransaction)))
	protectedMux.HandleFunc("GET /cash-transactions/{cashTransactionID}", server.showCashTransactionDetail)
	protectedMux.Handle("POST /cash-transactions/{cashTransactionID}/reverse", server.requirePermission(identity.PermissionCashReverse, http.HandlerFunc(server.reverseCashTransaction)))
	protectedMux.HandleFunc("GET /allocations", server.showAllocations)
	protectedMux.Handle("POST /allocations", server.requirePermission(identity.PermissionAllocationManage, http.HandlerFunc(server.applyAllocation)))
	protectedMux.HandleFunc("GET /allocations/{allocationID}", server.showAllocationDetail)
	protectedMux.Handle("POST /allocations/{allocationID}/reverse", server.requirePermission(identity.PermissionAllocationManage, http.HandlerFunc(server.reverseAllocation)))
	protectedMux.HandleFunc("GET /internal-transfers", server.showInternalTransfers)
	protectedMux.Handle("POST /internal-transfers", server.requirePermission(identity.PermissionTransferRecord, http.HandlerFunc(server.createInternalTransfer)))
	protectedMux.HandleFunc("GET /internal-transfers/{internalTransferID}", server.showInternalTransferDetail)

	publicMux := http.NewServeMux()
	server.registerCashWorkspace(publicMux)
	publicMux.HandleFunc("GET /assets/demo-workbench.css", demoWorkbenchStyle)
	publicMux.HandleFunc("GET /health/live", server.live)
	publicMux.HandleFunc("GET /health/ready", server.ready)
	publicMux.Handle("GET /api/v1/metrics/operating-summary", server.authenticate(server.requirePermission(identity.PermissionStatisticsRead, http.HandlerFunc(server.operatingMetricProjection))))
	if !server.externalIdentity {
		publicMux.HandleFunc("GET /login", server.showLogin)
		publicMux.HandleFunc("POST /login", server.login)
		publicMux.Handle("POST /logout", server.authenticate(http.HandlerFunc(server.logout)))
	}
	publicMux.Handle("GET /api/v1/references/obligations/{obligationID}", server.authenticateReference(identity.PermissionReferenceRead, "obligation", "obligationID", http.HandlerFunc(server.obligationReference)))
	publicMux.Handle("GET /api/v1/references/transactions/{transactionID}", server.authenticateReference(identity.PermissionReferenceRead, "transaction", "transactionID", http.HandlerFunc(server.transactionReference)))
	if server.receiptSets != nil {
		publicMux.Handle("POST /api/v1/receipt-sets", server.authenticateReceiptSetCommand(http.HandlerFunc(server.confirmReceiptSet)))
		publicMux.Handle("GET /api/v1/references/receipt-sets/{receiptSetID}", server.authenticateReference(identity.PermissionReceiptSetReferenceRead, "receipt_set", "receiptSetID", http.HandlerFunc(server.receiptSetReference)))
	}
	if server.payableSuggestions != nil {
		publicMux.Handle("POST /api/v1/payable-suggestion-intakes", server.authenticatePayableSuggestion(identity.PermissionPayableSuggestionSubmit, http.HandlerFunc(server.submitPayableSuggestion)))
		publicMux.Handle("GET /api/v1/payable-suggestion-intakes/{suggestionID}", server.authenticatePayableSuggestion(identity.PermissionPayableSuggestionRead, http.HandlerFunc(server.getPayableSuggestion)))
		publicMux.Handle("GET /api/v1/payable-suggestion-intakes/{suggestionID}/history", server.authenticatePayableSuggestion(identity.PermissionPayableSuggestionRead, http.HandlerFunc(server.payableSuggestionHistory)))
		publicMux.Handle("POST /api/v1/payable-suggestion-intakes/{suggestionID}/decisions", server.authenticatePayableSuggestion(identity.PermissionPayableSuggestionDecide, http.HandlerFunc(server.decidePayableSuggestion)))
		publicMux.Handle("POST /api/v1/payable-suggestion-intakes/{suggestionID}/withdrawal", server.authenticatePayableSuggestion(identity.PermissionPayableSuggestionWithdraw, http.HandlerFunc(server.withdrawPayableSuggestion)))
	}
	publicMux.Handle("/", server.authenticate(server.requirePermission(identity.PermissionView, protectedMux)))

	return securityHeaders(publicMux), nil
}

func (s *Server) requirePermission(permission identity.Permission, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := principalFromContext(r.Context())
		if !ok || !principal.Can(permission) {
			http.Error(w, "无权访问该功能", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"service": serviceName, "status": "ok"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.readiness.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"service": serviceName, "status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"service": serviceName, "status": "ready"})
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(s.sessionCookieName)
		if err != nil {
			s.requireLogin(w, r)
			return
		}
		principal, found, err := s.identities.ResolveSession(r.Context(), cookie.Value)
		if err != nil {
			http.Error(w, "无法验证登录会话", http.StatusInternalServerError)
			return
		}
		if !found {
			if !s.externalIdentity {
				s.clearSessionCookie(w)
			}
			s.requireLogin(w, r)
			return
		}
		ctx := context.WithValue(r.Context(), principalContextKey{}, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) requireLogin(w http.ResponseWriter, r *http.Request) {
	if s.externalIdentity {
		http.Error(w, "需要有效的 IAM Capability Credential", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "请先登录", http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, s.publicBasePath+"/login?next="+url.QueryEscape(safeReturnPath(r.URL.RequestURI())), http.StatusSeeOther)
}

func safeReturnPath(raw string) string {
	if raw == "" {
		return "/"
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/") || strings.HasPrefix(parsed.Path, "//") || strings.Contains(parsed.Path, `\`) {
		return "/"
	}
	if parsed.Path == "/login" {
		return "/"
	}
	return parsed.RequestURI()
}

type loginPageData struct {
	CSRFToken string
	Username  string
	Next      string
	Error     string
}

func (s *Server) showLogin(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(s.sessionCookieName); err == nil {
		if _, found, resolveErr := s.identities.ResolveSession(r.Context(), cookie.Value); resolveErr == nil && found {
			http.Redirect(w, r, safeReturnPath(r.URL.Query().Get("next")), http.StatusSeeOther)
			return
		}
	}
	token, err := s.ensureCSRFToken(w, r)
	if err != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, http.StatusOK, s.loginPage, "login.html", loginPageData{
		CSRFToken: token,
		Next:      safeReturnPath(r.URL.Query().Get("next")),
	})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}
	username := r.PostForm.Get("username")
	next := safeReturnPath(r.PostForm.Get("next"))
	result, err := s.identities.Login(r.Context(), username, r.PostForm.Get("password"))
	if err != nil {
		var authenticationError *identity.AuthenticationError
		if !errors.As(err, &authenticationError) {
			http.Error(w, "登录失败", http.StatusInternalServerError)
			return
		}
		token, tokenErr := s.ensureCSRFToken(w, r)
		if tokenErr != nil {
			http.Error(w, "登录失败", http.StatusInternalServerError)
			return
		}
		s.renderTemplate(w, http.StatusUnauthorized, s.loginPage, "login.html", loginPageData{
			CSRFToken: token,
			Username:  username,
			Next:      next,
			Error:     authenticationError.Error(),
		})
		return
	}
	maxAge := int(time.Until(result.ExpiresAt).Seconds())
	if maxAge < 1 {
		http.Error(w, "登录会话已过期", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     s.sessionCookieName,
		Value:    result.Token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.sessionCookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
		Expires:  result.ExpiresAt,
	})
	s.clearCSRFCookie(w)
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}
	if cookie, err := r.Cookie(s.sessionCookieName); err == nil {
		if err := s.identities.Logout(r.Context(), cookie.Value); err != nil {
			http.Error(w, "注销失败", http.StatusInternalServerError)
			return
		}
	}
	s.clearSessionCookie(w)
	s.clearCSRFCookie(w)
	http.Redirect(w, r, s.publicBasePath+"/login", http.StatusSeeOther)
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	token, err := s.ensureCSRFToken(w, r)
	if err != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	principal, _ := principalFromContext(r.Context())
	s.renderTemplate(w, http.StatusOK, s.homePage, "home.html", struct {
		Title     string
		Status    string
		Principal identity.Principal
		CSRFToken string
	}{
		Title:     "Money Flow Ledger",
		Status:    "M4 进行中：义务、实际资金、核销纠错、经营汇总与可校验台账导出",
		Principal: principal,
		CSRFToken: token,
	})
}

type cashTransactionForm struct {
	CommandID                    string
	Type                         string
	LedgerOwnerPartyID           string
	CounterpartyPartyID          string
	Description                  string
	Amount                       string
	Currency                     string
	OccurredOn                   string
	AccountReference             string
	ManualSourceReference        string
	ExternalTransactionReference string
}

type cashTransactionPageData struct {
	Principal    identity.Principal
	CSRFToken    string
	Form         cashTransactionForm
	Parties      []party.Party
	Transactions []cashtransaction.Transaction
	Error        string
	CanCreate    bool
}

type cashTransactionDetailData struct {
	Principal               identity.Principal
	CSRFToken               string
	Transaction             cashtransaction.Transaction
	Allocations             []allocation.Allocation
	ReverseCommandID        string
	ReversalReason          string
	Error                   string
	Created                 bool
	Reversed                bool
	CanReverse              bool
	NeedsAllocationReversal bool
}

func (s *Server) showCashTransactions(w http.ResponseWriter, r *http.Request) {
	commandID, err := cashtransaction.NewCommandID()
	if err != nil {
		http.Error(w, "无法建立实际收付款表单", http.StatusInternalServerError)
		return
	}
	s.renderCashTransactionPage(w, r, http.StatusOK, cashTransactionForm{
		CommandID: commandID,
		Type:      string(cashtransaction.TypeReceipt),
		Currency:  "CNY",
	}, "")
}

func (s *Server) createCashTransaction(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	form := cashTransactionForm{
		CommandID:                    r.PostForm.Get("command_id"),
		Type:                         r.PostForm.Get("type"),
		LedgerOwnerPartyID:           r.PostForm.Get("ledger_owner_party_id"),
		CounterpartyPartyID:          r.PostForm.Get("counterparty_party_id"),
		Description:                  r.PostForm.Get("description"),
		Amount:                       r.PostForm.Get("amount"),
		Currency:                     r.PostForm.Get("currency"),
		OccurredOn:                   r.PostForm.Get("occurred_on"),
		AccountReference:             r.PostForm.Get("account_reference"),
		ManualSourceReference:        r.PostForm.Get("manual_source_reference"),
		ExternalTransactionReference: r.PostForm.Get("external_transaction_reference"),
	}
	result, err := s.cashTransactions.Create(r.Context(), cashtransaction.CreateInput{
		CommandID:                    form.CommandID,
		Type:                         form.Type,
		LedgerOwnerPartyID:           form.LedgerOwnerPartyID,
		CounterpartyPartyID:          form.CounterpartyPartyID,
		Description:                  form.Description,
		Amount:                       form.Amount,
		Currency:                     form.Currency,
		OccurredOn:                   form.OccurredOn,
		AccountReference:             form.AccountReference,
		ManualSourceReference:        form.ManualSourceReference,
		ExternalTransactionReference: form.ExternalTransactionReference,
		Actor:                        principal.Actor(),
	})
	if err != nil {
		status := http.StatusInternalServerError
		message := "实际收付款登记失败"
		var validationError *cashtransaction.ValidationError
		var partyNotFoundError *cashtransaction.PartyNotFoundError
		var idempotencyConflict *cashtransaction.IdempotencyConflictError
		var externalReferenceConflict *cashtransaction.ExternalReferenceConflictError
		switch {
		case errors.As(err, &validationError):
			status = http.StatusUnprocessableEntity
			message = validationError.Error()
		case errors.As(err, &partyNotFoundError):
			status = http.StatusUnprocessableEntity
			message = partyNotFoundError.Error()
		case errors.As(err, &idempotencyConflict):
			status = http.StatusConflict
			message = idempotencyConflict.Error()
		case errors.As(err, &externalReferenceConflict):
			status = http.StatusConflict
			message = externalReferenceConflict.Error()
		default:
			http.Error(w, message, status)
			return
		}
		commandID, commandErr := cashtransaction.NewCommandID()
		if commandErr != nil {
			http.Error(w, "无法重建实际收付款表单", http.StatusInternalServerError)
			return
		}
		form.CommandID = commandID
		s.renderCashTransactionPage(w, r, status, form, message)
		return
	}
	http.Redirect(w, r, "/cash-transactions/"+url.PathEscape(result.Transaction.ID)+"?created=1", http.StatusSeeOther)
}

func (s *Server) renderCashTransactionPage(w http.ResponseWriter, r *http.Request, status int, form cashTransactionForm, message string) {
	token, err := s.ensureCSRFToken(w, r)
	if err != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	parties, err := s.parties.List(r.Context())
	if err != nil {
		http.Error(w, "无法读取主体列表", http.StatusInternalServerError)
		return
	}
	transactions, err := s.cashTransactions.List(r.Context())
	if err != nil {
		http.Error(w, "无法读取实际收付款列表", http.StatusInternalServerError)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, status, s.cashTransactionPage, "cash_transactions.html", cashTransactionPageData{
		Principal:    principal,
		CSRFToken:    token,
		Form:         form,
		Parties:      parties,
		Transactions: transactions,
		Error:        message,
		CanCreate:    len(parties) >= 2,
	})
}

func (s *Server) showCashTransactionDetail(w http.ResponseWriter, r *http.Request) {
	s.renderCashTransactionDetail(w, r, http.StatusOK, "", "")
}

func (s *Server) reverseCashTransaction(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	reason := r.PostForm.Get("reason")
	result, err := s.cashTransactions.Reverse(r.Context(), cashtransaction.ReverseInput{
		CommandID:         r.PostForm.Get("command_id"),
		CashTransactionID: r.PathValue("cashTransactionID"),
		Reason:            reason,
		Actor:             principal.Actor(),
	})
	if err == nil {
		http.Redirect(w, r, "/cash-transactions/"+url.PathEscape(result.Reversal.CashTransactionID)+"?reversed=1", http.StatusSeeOther)
		return
	}

	status := http.StatusInternalServerError
	message := "实际收付款冲销失败"
	var validationError *cashtransaction.ValidationError
	var notFound *cashtransaction.TransactionNotFoundError
	var idempotencyConflict *cashtransaction.ReversalIdempotencyConflictError
	var alreadyReversed *cashtransaction.TransactionAlreadyReversedError
	var allocatedConflict *cashtransaction.TransactionAllocatedConflictError
	switch {
	case errors.As(err, &validationError) && validationError.Field == "cash_transaction_id":
		http.NotFound(w, r)
		return
	case errors.As(err, &notFound):
		http.NotFound(w, r)
		return
	case errors.As(err, &validationError):
		status = http.StatusUnprocessableEntity
		message = validationError.Error()
	case errors.As(err, &idempotencyConflict):
		status = http.StatusConflict
		message = idempotencyConflict.Error()
	case errors.As(err, &alreadyReversed):
		status = http.StatusConflict
		message = alreadyReversed.Error()
	case errors.As(err, &allocatedConflict):
		status = http.StatusConflict
		message = allocatedConflict.Error()
	default:
		http.Error(w, message, status)
		return
	}
	s.renderCashTransactionDetail(w, r, status, reason, message)
}

func (s *Server) renderCashTransactionDetail(w http.ResponseWriter, r *http.Request, status int, reversalReason, message string) {
	value, found, err := s.cashTransactions.Get(r.Context(), r.PathValue("cashTransactionID"))
	if err != nil {
		var validationError *cashtransaction.ValidationError
		if errors.As(err, &validationError) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "无法读取实际收付款", http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	allocations, err := s.allocations.ListForCashTransaction(r.Context(), value.ID)
	if err != nil {
		http.Error(w, "无法读取实际收付款的分配历史", http.StatusInternalServerError)
		return
	}
	canReverse := value.Status == cashtransaction.StatusRecorded && value.AllocatedAmount == "0"
	commandID := ""
	if canReverse {
		commandID, err = cashtransaction.NewCommandID()
		if err != nil {
			http.Error(w, "无法建立实际收付款冲销表单", http.StatusInternalServerError)
			return
		}
	}
	token, err := s.ensureCSRFToken(w, r)
	if err != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, status, s.cashTransactionDetail, "cash_transaction_detail.html", cashTransactionDetailData{
		Principal:               principal,
		CSRFToken:               token,
		Transaction:             value,
		Allocations:             allocations,
		ReverseCommandID:        commandID,
		ReversalReason:          reversalReason,
		Error:                   message,
		Created:                 r.URL.Query().Get("created") == "1",
		Reversed:                r.URL.Query().Get("reversed") == "1",
		CanReverse:              canReverse,
		NeedsAllocationReversal: value.Status == cashtransaction.StatusRecorded && value.AllocatedAmount != "0",
	})
}

type internalTransferForm struct {
	CommandID                   string
	LedgerOwnerPartyID          string
	Description                 string
	Amount                      string
	Currency                    string
	OccurredOn                  string
	SourceAccountReference      string
	DestinationAccountReference string
	ManualSourceReference       string
	ExternalTransferReference   string
}

type internalTransferPageData struct {
	Principal identity.Principal
	CSRFToken string
	Form      internalTransferForm
	Parties   []party.Party
	Transfers []internaltransfer.Transfer
	Error     string
	CanCreate bool
}

type internalTransferDetailData struct {
	Principal identity.Principal
	CSRFToken string
	Transfer  internaltransfer.Transfer
	Created   bool
}

func (s *Server) showInternalTransfers(w http.ResponseWriter, r *http.Request) {
	commandID, err := internaltransfer.NewCommandID()
	if err != nil {
		http.Error(w, "无法建立内部转账表单", http.StatusInternalServerError)
		return
	}
	s.renderInternalTransferPage(w, r, http.StatusOK, internalTransferForm{CommandID: commandID, Currency: "CNY"}, "")
}

func (s *Server) createInternalTransfer(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	form := internalTransferForm{
		CommandID:                   r.PostForm.Get("command_id"),
		LedgerOwnerPartyID:          r.PostForm.Get("ledger_owner_party_id"),
		Description:                 r.PostForm.Get("description"),
		Amount:                      r.PostForm.Get("amount"),
		Currency:                    r.PostForm.Get("currency"),
		OccurredOn:                  r.PostForm.Get("occurred_on"),
		SourceAccountReference:      r.PostForm.Get("source_account_reference"),
		DestinationAccountReference: r.PostForm.Get("destination_account_reference"),
		ManualSourceReference:       r.PostForm.Get("manual_source_reference"),
		ExternalTransferReference:   r.PostForm.Get("external_transfer_reference"),
	}
	result, err := s.internalTransfers.Create(r.Context(), internaltransfer.CreateInput{
		CommandID:                   form.CommandID,
		LedgerOwnerPartyID:          form.LedgerOwnerPartyID,
		Description:                 form.Description,
		Amount:                      form.Amount,
		Currency:                    form.Currency,
		OccurredOn:                  form.OccurredOn,
		SourceAccountReference:      form.SourceAccountReference,
		DestinationAccountReference: form.DestinationAccountReference,
		ManualSourceReference:       form.ManualSourceReference,
		ExternalTransferReference:   form.ExternalTransferReference,
		Actor:                       principal.Actor(),
	})
	if err == nil {
		http.Redirect(w, r, "/internal-transfers/"+url.PathEscape(result.Transfer.ID)+"?created=1", http.StatusSeeOther)
		return
	}

	status := http.StatusInternalServerError
	message := "内部账户转账登记失败"
	var validationError *internaltransfer.ValidationError
	var partyNotFoundError *internaltransfer.PartyNotFoundError
	var idempotencyConflict *internaltransfer.IdempotencyConflictError
	var externalReferenceConflict *internaltransfer.ExternalReferenceConflictError
	switch {
	case errors.As(err, &validationError):
		status = http.StatusUnprocessableEntity
		message = validationError.Error()
	case errors.As(err, &partyNotFoundError):
		status = http.StatusUnprocessableEntity
		message = partyNotFoundError.Error()
	case errors.As(err, &idempotencyConflict):
		status = http.StatusConflict
		message = idempotencyConflict.Error()
	case errors.As(err, &externalReferenceConflict):
		status = http.StatusConflict
		message = externalReferenceConflict.Error()
	default:
		http.Error(w, message, status)
		return
	}
	commandID, commandErr := internaltransfer.NewCommandID()
	if commandErr != nil {
		http.Error(w, "无法重建内部转账表单", http.StatusInternalServerError)
		return
	}
	form.CommandID = commandID
	s.renderInternalTransferPage(w, r, status, form, message)
}

func (s *Server) renderInternalTransferPage(w http.ResponseWriter, r *http.Request, status int, form internalTransferForm, message string) {
	token, err := s.ensureCSRFToken(w, r)
	if err != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	parties, err := s.parties.List(r.Context())
	if err != nil {
		http.Error(w, "无法读取主体列表", http.StatusInternalServerError)
		return
	}
	transfers, err := s.internalTransfers.List(r.Context())
	if err != nil {
		http.Error(w, "无法读取内部转账列表", http.StatusInternalServerError)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, status, s.internalTransferPage, "internal_transfers.html", internalTransferPageData{
		Principal: principal,
		CSRFToken: token,
		Form:      form,
		Parties:   parties,
		Transfers: transfers,
		Error:     message,
		CanCreate: len(parties) >= 1,
	})
}

func (s *Server) showInternalTransferDetail(w http.ResponseWriter, r *http.Request) {
	value, found, err := s.internalTransfers.Get(r.Context(), r.PathValue("internalTransferID"))
	if err != nil {
		var validationError *internaltransfer.ValidationError
		if errors.As(err, &validationError) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "无法读取内部转账", http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	token, err := s.ensureCSRFToken(w, r)
	if err != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, http.StatusOK, s.internalTransferDetail, "internal_transfer_detail.html", internalTransferDetailData{
		Principal: principal,
		CSRFToken: token,
		Transfer:  value,
		Created:   r.URL.Query().Get("created") == "1",
	})
}

type allocationForm struct {
	CommandID             string
	CashTransactionID     string
	ObligationSelection   string
	Amount                string
	ManualSourceReference string
}

type allocationObligationOption struct {
	Value      string
	Obligation obligation.Obligation
}

type allocationPageData struct {
	Principal        identity.Principal
	CSRFToken        string
	Form             allocationForm
	CashTransactions []cashtransaction.Transaction
	Obligations      []allocationObligationOption
	Allocations      []allocation.Allocation
	Error            string
	CanApply         bool
}

type allocationDetailData struct {
	Principal        identity.Principal
	CSRFToken        string
	Allocation       allocation.Allocation
	ReverseCommandID string
	ReversalReason   string
	Error            string
	Created          bool
	Reversed         bool
	CanReverse       bool
}

func (s *Server) showAllocations(w http.ResponseWriter, r *http.Request) {
	commandID, err := allocation.NewCommandID()
	if err != nil {
		http.Error(w, "无法建立资金分配表单", http.StatusInternalServerError)
		return
	}
	s.renderAllocationPage(w, r, http.StatusOK, allocationForm{CommandID: commandID}, "")
}

func (s *Server) applyAllocation(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	form := allocationForm{
		CommandID:             r.PostForm.Get("command_id"),
		CashTransactionID:     r.PostForm.Get("cash_transaction_id"),
		ObligationSelection:   r.PostForm.Get("obligation_selection"),
		Amount:                r.PostForm.Get("amount"),
		ManualSourceReference: r.PostForm.Get("manual_source_reference"),
	}
	obligationID, expectedRevision := parseObligationSelection(form.ObligationSelection)
	result, err := s.allocations.Apply(r.Context(), allocation.ApplyInput{
		CommandID:                  form.CommandID,
		CashTransactionID:          form.CashTransactionID,
		ObligationID:               obligationID,
		ExpectedObligationRevision: expectedRevision,
		Amount:                     form.Amount,
		ManualSourceReference:      form.ManualSourceReference,
		Actor:                      principal.Actor(),
	})
	if err == nil {
		http.Redirect(w, r, "/allocations/"+url.PathEscape(result.Allocation.ID)+"?created=1", http.StatusSeeOther)
		return
	}

	status := http.StatusInternalServerError
	message := "资金分配失败"
	var validationError *allocation.ValidationError
	var cashNotFound *allocation.CashTransactionNotFoundError
	var cashStateConflict *allocation.CashTransactionStateConflictError
	var obligationNotFound *allocation.ObligationNotFoundError
	var idempotencyConflict *allocation.IdempotencyConflictError
	var stateConflict *allocation.ObligationStateConflictError
	var revisionConflict *allocation.ObligationRevisionConflictError
	var compatibilityConflict *allocation.CompatibilityConflictError
	var cashBalanceConflict *allocation.CashBalanceConflictError
	var obligationBalanceConflict *allocation.ObligationBalanceConflictError
	switch {
	case errors.As(err, &validationError):
		status = http.StatusUnprocessableEntity
		message = validationError.Error()
	case errors.As(err, &cashNotFound):
		status = http.StatusUnprocessableEntity
		message = cashNotFound.Error()
	case errors.As(err, &cashStateConflict):
		status = http.StatusConflict
		message = cashStateConflict.Error()
	case errors.As(err, &obligationNotFound):
		status = http.StatusUnprocessableEntity
		message = obligationNotFound.Error()
	case errors.As(err, &idempotencyConflict):
		status = http.StatusConflict
		message = idempotencyConflict.Error()
	case errors.As(err, &stateConflict):
		status = http.StatusConflict
		message = stateConflict.Error()
	case errors.As(err, &revisionConflict):
		status = http.StatusConflict
		message = revisionConflict.Error()
	case errors.As(err, &compatibilityConflict):
		status = http.StatusConflict
		message = compatibilityConflict.Error()
	case errors.As(err, &cashBalanceConflict):
		status = http.StatusConflict
		message = cashBalanceConflict.Error()
	case errors.As(err, &obligationBalanceConflict):
		status = http.StatusConflict
		message = obligationBalanceConflict.Error()
	default:
		http.Error(w, message, status)
		return
	}
	commandID, commandErr := allocation.NewCommandID()
	if commandErr != nil {
		http.Error(w, "无法重建资金分配表单", http.StatusInternalServerError)
		return
	}
	form.CommandID = commandID
	s.renderAllocationPage(w, r, status, form, message)
}

func parseObligationSelection(value string) (string, int64) {
	separator := strings.LastIndex(value, ":")
	if separator < 0 {
		return value, 0
	}
	revision, err := strconv.ParseInt(strings.TrimSpace(value[separator+1:]), 10, 64)
	if err != nil {
		return strings.TrimSpace(value[:separator]), 0
	}
	return strings.TrimSpace(value[:separator]), revision
}

func (s *Server) renderAllocationPage(w http.ResponseWriter, r *http.Request, status int, form allocationForm, message string) {
	token, err := s.ensureCSRFToken(w, r)
	if err != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	cashValues, err := s.cashTransactions.List(r.Context())
	if err != nil {
		http.Error(w, "无法读取实际收付款列表", http.StatusInternalServerError)
		return
	}
	obligationValues, err := s.obligations.List(r.Context())
	if err != nil {
		http.Error(w, "无法读取义务列表", http.StatusInternalServerError)
		return
	}
	allocationValues, err := s.allocations.List(r.Context())
	if err != nil {
		http.Error(w, "无法读取资金分配列表", http.StatusInternalServerError)
		return
	}
	availableCash := make([]cashtransaction.Transaction, 0, len(cashValues))
	for _, value := range cashValues {
		if value.UnallocatedAmount != "" && value.UnallocatedAmount != "0" {
			availableCash = append(availableCash, value)
		}
	}
	availableObligations := make([]allocationObligationOption, 0, len(obligationValues))
	for _, value := range obligationValues {
		if value.Status != obligation.StatusOpen || value.CurrentRevision < 1 || value.OutstandingAmount == "" || value.OutstandingAmount == "0" {
			continue
		}
		availableObligations = append(availableObligations, allocationObligationOption{
			Value:      value.ID + ":" + strconv.FormatInt(value.CurrentRevision, 10),
			Obligation: value,
		})
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, status, s.allocationPage, "allocations.html", allocationPageData{
		Principal:        principal,
		CSRFToken:        token,
		Form:             form,
		CashTransactions: availableCash,
		Obligations:      availableObligations,
		Allocations:      allocationValues,
		Error:            message,
		CanApply:         len(availableCash) > 0 && len(availableObligations) > 0,
	})
}

func (s *Server) showAllocationDetail(w http.ResponseWriter, r *http.Request) {
	s.renderAllocationDetail(w, r, http.StatusOK, "", "")
}

func (s *Server) reverseAllocation(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	reason := r.PostForm.Get("reason")
	result, err := s.allocations.Reverse(r.Context(), allocation.ReverseInput{
		CommandID:    r.PostForm.Get("command_id"),
		AllocationID: r.PathValue("allocationID"),
		Reason:       reason,
		Actor:        principal.Actor(),
	})
	if err == nil {
		http.Redirect(w, r, "/allocations/"+url.PathEscape(result.Reversal.AllocationID)+"?reversed=1", http.StatusSeeOther)
		return
	}

	status := http.StatusInternalServerError
	message := "反向核销失败"
	var validationError *allocation.ValidationError
	var allocationNotFound *allocation.AllocationNotFoundError
	var idempotencyConflict *allocation.ReversalIdempotencyConflictError
	var alreadyReversed *allocation.AllocationAlreadyReversedError
	var obligationStateConflict *allocation.ReversalObligationStateConflictError
	switch {
	case errors.As(err, &validationError) && validationError.Field == "allocation_id":
		http.NotFound(w, r)
		return
	case errors.As(err, &allocationNotFound):
		http.NotFound(w, r)
		return
	case errors.As(err, &validationError):
		status = http.StatusUnprocessableEntity
		message = validationError.Error()
	case errors.As(err, &idempotencyConflict):
		status = http.StatusConflict
		message = idempotencyConflict.Error()
	case errors.As(err, &alreadyReversed):
		status = http.StatusConflict
		message = alreadyReversed.Error()
	case errors.As(err, &obligationStateConflict):
		status = http.StatusConflict
		message = obligationStateConflict.Error()
	default:
		http.Error(w, message, status)
		return
	}
	s.renderAllocationDetail(w, r, status, reason, message)
}

func (s *Server) renderAllocationDetail(w http.ResponseWriter, r *http.Request, status int, reversalReason, message string) {
	value, found, err := s.allocations.Get(r.Context(), r.PathValue("allocationID"))
	if err != nil {
		var validationError *allocation.ValidationError
		if errors.As(err, &validationError) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "无法读取资金分配", http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	canReverse := false
	if value.Status == allocation.StatusActive {
		obligationDetails, obligationFound, obligationErr := s.obligations.GetDetails(r.Context(), value.ObligationID)
		if obligationErr != nil {
			http.Error(w, "无法读取资金分配关联义务", http.StatusInternalServerError)
			return
		}
		if !obligationFound {
			http.Error(w, "资金分配关联义务不存在", http.StatusInternalServerError)
			return
		}
		canReverse = obligationDetails.Obligation.Status == obligation.StatusOpen
	}
	commandID := ""
	if canReverse {
		commandID, err = allocation.NewCommandID()
		if err != nil {
			http.Error(w, "无法建立反向核销表单", http.StatusInternalServerError)
			return
		}
	}
	token, err := s.ensureCSRFToken(w, r)
	if err != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, status, s.allocationDetail, "allocation_detail.html", allocationDetailData{
		Principal:        principal,
		CSRFToken:        token,
		Allocation:       value,
		ReverseCommandID: commandID,
		ReversalReason:   reversalReason,
		Error:            message,
		Created:          r.URL.Query().Get("created") == "1",
		Reversed:         r.URL.Query().Get("reversed") == "1",
		CanReverse:       canReverse,
	})
}

type obligationForm struct {
	CommandID             string
	Direction             string
	LedgerOwnerPartyID    string
	CounterpartyPartyID   string
	Title                 string
	OriginalAmount        string
	Currency              string
	DueOn                 string
	ManualSourceReference string
}

type obligationPageData struct {
	Principal   identity.Principal
	CSRFToken   string
	Form        obligationForm
	Parties     []party.Party
	Obligations []obligation.Obligation
	Error       string
	CanCreate   bool
}

type obligationDetailData struct {
	Principal           identity.Principal
	CSRFToken           string
	Obligation          obligation.Obligation
	Revisions           []obligation.Revision
	Adjustments         []obligation.Adjustment
	Remissions          []obligation.Remission
	Closure             *obligation.Closure
	Allocations         []allocation.Allocation
	ConfirmationCommand string
	AdjustmentCommand   string
	AdjustmentType      string
	AdjustmentAmount    string
	AdjustmentReason    string
	RemissionCommand    string
	RemissionAmount     string
	RemissionReason     string
	RemissionSource     string
	ClosureCommand      string
	ClosureReason       string
	Created             bool
	Confirmed           bool
	Adjusted            bool
	Remitted            bool
	Closed              bool
	CanAdjust           bool
	CanRemit            bool
	CanClose            bool
	Error               string
}

type agingPageData struct {
	Principal identity.Principal
	CSRFToken string
	AsOfDate  string
	Report    obligation.AgingReport
	Error     string
}

type cashExpectationForm struct {
	CommandID               string
	ObligationID            string
	ExpectedCurrentRevision int64
	ExpectedOn              string
	Reason                  string
}

type scheduledCashForecastItem struct {
	Row            obligation.CashExpectationRow
	ClearCommandID string
}

type cashForecastPageData struct {
	Principal      identity.Principal
	CSRFToken      string
	Report         obligation.CashForecastReport
	ScheduledItems []scheduledCashForecastItem
	SetForm        cashExpectationForm
	History        obligation.CashExpectationHistory
	HasHistory     bool
	Recorded       bool
	Cleared        bool
	Error          string
}

type operatingSummaryPageData struct {
	Principal identity.Principal
	CSRFToken string
	Report    operatingsummary.Report
}

type operatingSummaryDetailPageData struct {
	Principal identity.Principal
	CSRFToken string
	Details   operatingsummary.Drilldown
}

type ledgerExportPageData struct {
	Principal          identity.Principal
	CSRFToken          string
	Parties            []party.Party
	LedgerOwnerPartyID string
	Error              string
}

type historicalBalancePageData struct {
	Principal    identity.Principal
	CSRFToken    string
	ObligationID string
	AsOfInput    string
	Snapshot     obligation.HistoricalSnapshot
	HasSnapshot  bool
	Error        string
}

func (s *Server) showCurrentAging(w http.ResponseWriter, r *http.Request) {
	report, err := s.obligations.CurrentAging(r.Context(), obligation.CurrentAgingInput{
		AsOfDate: r.URL.Query().Get("as_of_date"),
	})
	status := http.StatusOK
	message := ""
	asOfDate := report.AsOfDate
	if err != nil {
		var validationError *obligation.ValidationError
		if !errors.As(err, &validationError) {
			http.Error(w, "无法生成当前义务账龄", http.StatusInternalServerError)
			return
		}
		status = http.StatusUnprocessableEntity
		message = validationError.Error()
		asOfDate = r.URL.Query().Get("as_of_date")
	}
	token, err := s.ensureCSRFToken(w, r)
	if err != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, status, s.agingPage, "aging.html", agingPageData{
		Principal: principal,
		CSRFToken: token,
		AsOfDate:  asOfDate,
		Report:    report,
		Error:     message,
	})
}

func (s *Server) showCashForecast(w http.ResponseWriter, r *http.Request) {
	s.renderCashForecastPage(
		w,
		r,
		http.StatusOK,
		cashExpectationForm{},
		"",
		r.URL.Query().Get("history_obligation_id"),
	)
}

func (s *Server) recordCashExpectation(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	expectedRevision, parseErr := strconv.ParseInt(strings.TrimSpace(r.PostForm.Get("expected_current_revision")), 10, 64)
	if parseErr != nil {
		expectedRevision = 0
	}
	obligationID := r.PostForm.Get("obligation_id")
	if strings.TrimSpace(obligationID) == "" {
		obligationID, expectedRevision = parseObligationSelection(r.PostForm.Get("obligation_selection"))
	}
	form := cashExpectationForm{
		CommandID:               r.PostForm.Get("command_id"),
		ObligationID:            obligationID,
		ExpectedCurrentRevision: expectedRevision,
		ExpectedOn:              r.PostForm.Get("expected_on"),
		Reason:                  r.PostForm.Get("expectation_reason"),
	}
	action := r.PostForm.Get("action")
	result, err := s.obligations.RecordCashExpectation(r.Context(), obligation.RecordCashExpectationInput{
		CommandID: form.CommandID, ObligationID: form.ObligationID,
		ExpectedCurrentRevision: form.ExpectedCurrentRevision,
		Action:                  action, ExpectedOn: form.ExpectedOn, Reason: form.Reason,
		Actor: principal.Actor(),
	})
	if err == nil {
		query := url.Values{"history_obligation_id": {result.Expectation.ObligationID}}
		if result.Expectation.Action == obligation.CashExpectationActionClear {
			query.Set("cleared", "1")
		} else {
			query.Set("recorded", "1")
		}
		http.Redirect(w, r, "/cash-forecast?"+query.Encode()+"#history", http.StatusSeeOther)
		return
	}

	status := http.StatusInternalServerError
	message := "预计现金日期记录失败"
	var validation *obligation.ValidationError
	var notFound *obligation.ObligationNotFoundError
	var idempotency *obligation.CashExpectationIdempotencyConflictError
	var stateConflict *obligation.CashExpectationStateConflictError
	var revisionConflict *obligation.CurrentRevisionConflictError
	var balanceConflict *obligation.CashExpectationBalanceConflictError
	var clearConflict *obligation.CashExpectationClearConflictError
	switch {
	case errors.As(err, &validation):
		status = http.StatusUnprocessableEntity
		message = validation.Error()
	case errors.As(err, &notFound):
		http.NotFound(w, r)
		return
	case errors.As(err, &idempotency):
		status = http.StatusConflict
		message = idempotency.Error()
	case errors.As(err, &stateConflict):
		status = http.StatusConflict
		message = stateConflict.Error()
	case errors.As(err, &revisionConflict):
		status = http.StatusConflict
		message = revisionConflict.Error()
	case errors.As(err, &balanceConflict):
		status = http.StatusConflict
		message = balanceConflict.Error()
	case errors.As(err, &clearConflict):
		status = http.StatusConflict
		message = clearConflict.Error()
	default:
		http.Error(w, message, status)
		return
	}
	commandID, commandErr := obligation.NewCommandID()
	if commandErr != nil {
		http.Error(w, "无法重建预计现金日期表单", http.StatusInternalServerError)
		return
	}
	if action == string(obligation.CashExpectationActionSet) {
		form.CommandID = commandID
	} else {
		form = cashExpectationForm{CommandID: commandID}
	}
	s.renderCashForecastPage(w, r, status, form, message, obligationID)
}

func (s *Server) renderCashForecastPage(
	w http.ResponseWriter,
	r *http.Request,
	status int,
	form cashExpectationForm,
	message string,
	historyObligationID string,
) {
	report, err := s.obligations.CurrentCashForecast(r.Context())
	if err != nil {
		http.Error(w, "无法生成当前预计现金排期", http.StatusInternalServerError)
		return
	}
	if form.CommandID == "" {
		form.CommandID, err = obligation.NewCommandID()
		if err != nil {
			http.Error(w, "无法建立预计现金日期表单", http.StatusInternalServerError)
			return
		}
	}
	scheduledItems := make([]scheduledCashForecastItem, 0, len(report.ScheduledItems))
	for _, row := range report.ScheduledItems {
		commandID, commandErr := obligation.NewCommandID()
		if commandErr != nil {
			http.Error(w, "无法建立清除排期表单", http.StatusInternalServerError)
			return
		}
		scheduledItems = append(scheduledItems, scheduledCashForecastItem{Row: row, ClearCommandID: commandID})
	}
	var history obligation.CashExpectationHistory
	hasHistory := strings.TrimSpace(historyObligationID) != ""
	if hasHistory {
		history, err = s.obligations.CashExpectationHistory(r.Context(), historyObligationID)
		if err != nil {
			var validation *obligation.ValidationError
			var notFound *obligation.ObligationNotFoundError
			if errors.As(err, &validation) || errors.As(err, &notFound) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "无法读取预计现金日期历史", http.StatusInternalServerError)
			return
		}
	}
	token, err := s.ensureCSRFToken(w, r)
	if err != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, status, s.cashForecastPage, "cash_forecast.html", cashForecastPageData{
		Principal: principal, CSRFToken: token, Report: report, ScheduledItems: scheduledItems,
		SetForm: form, History: history, HasHistory: hasHistory,
		Recorded: r.URL.Query().Get("recorded") == "1", Cleared: r.URL.Query().Get("cleared") == "1",
		Error: message,
	})
}

func (s *Server) showOperatingSummary(w http.ResponseWriter, r *http.Request) {
	report, err := s.operatingSummaries.Current(r.Context())
	if err != nil {
		http.Error(w, "无法生成当前经营收支汇总", http.StatusInternalServerError)
		return
	}
	token, err := s.ensureCSRFToken(w, r)
	if err != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, http.StatusOK, s.operatingSummaryPage, "operating_summary.html", operatingSummaryPageData{
		Principal: principal,
		CSRFToken: token,
		Report:    report,
	})
}

func (s *Server) showOperatingSummaryDetail(w http.ResponseWriter, r *http.Request) {
	details, err := s.operatingSummaries.Drilldown(r.Context(), operatingsummary.DrilldownInput{
		Metric:             r.URL.Query().Get("metric"),
		LedgerOwnerPartyID: r.URL.Query().Get("ledger_owner_party_id"),
		Currency:           r.URL.Query().Get("currency"),
	})
	if err != nil {
		var validationError *operatingsummary.ValidationError
		var notFound *operatingsummary.DrilldownNotFoundError
		switch {
		case errors.As(err, &validationError):
			http.Error(w, validationError.Error(), http.StatusUnprocessableEntity)
		case errors.As(err, &notFound):
			http.NotFound(w, r)
		default:
			http.Error(w, "无法读取当前经营收支明细", http.StatusInternalServerError)
		}
		return
	}
	token, err := s.ensureCSRFToken(w, r)
	if err != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, http.StatusOK, s.operatingDetailPage, "operating_summary_detail.html", operatingSummaryDetailPageData{
		Principal: principal,
		CSRFToken: token,
		Details:   details,
	})
}

func (s *Server) showLedgerExports(w http.ResponseWriter, r *http.Request) {
	s.renderLedgerExportPage(w, r, http.StatusOK, "", "")
}

func (s *Server) exportOwnerLedger(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	ownerID := r.PostForm.Get("ledger_owner_party_id")
	artifact, err := s.ledgerExports.Export(r.Context(), ledgerexport.Input{
		LedgerOwnerPartyID: ownerID,
		Actor:              principal.Actor(),
	})
	if err != nil {
		var validationError *ledgerexport.ValidationError
		var notFound *ledgerexport.NotFoundError
		switch {
		case errors.As(err, &validationError):
			s.renderLedgerExportPage(w, r, http.StatusUnprocessableEntity, ownerID, validationError.Error())
		case errors.As(err, &notFound):
			http.NotFound(w, r)
		default:
			http.Error(w, "无法生成本方主体台账导出", http.StatusInternalServerError)
		}
		return
	}
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": artifact.Filename})
	if disposition == "" {
		http.Error(w, "无法生成安全下载文件名", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", disposition)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.Itoa(len(artifact.Content)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(artifact.Content)
}

func (s *Server) renderLedgerExportPage(
	w http.ResponseWriter,
	r *http.Request,
	status int,
	ownerID string,
	message string,
) {
	token, err := s.ensureCSRFToken(w, r)
	if err != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	parties, err := s.parties.List(r.Context())
	if err != nil {
		http.Error(w, "无法读取主体列表", http.StatusInternalServerError)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, status, s.ledgerExportPage, "ledger_exports.html", ledgerExportPageData{
		Principal:          principal,
		CSRFToken:          token,
		Parties:            parties,
		LedgerOwnerPartyID: ownerID,
		Error:              message,
	})
}

func (s *Server) showHistoricalObligationBalance(w http.ResponseWriter, r *http.Request) {
	obligationID := r.PathValue("obligationID")
	rawAsOf := r.URL.Query().Get("as_of")
	snapshot, err := s.obligations.HistoricalBalance(r.Context(), obligation.HistoricalBalanceInput{
		ObligationID: obligationID,
		AsOf:         rawAsOf,
	})
	status := http.StatusOK
	message := ""
	hasSnapshot := err == nil
	asOfInput := rawAsOf
	if err == nil {
		asOfInput = snapshot.AsOf.UTC().Format(time.RFC3339Nano)
	} else {
		var validationError *obligation.ValidationError
		var notFound *obligation.ObligationNotFoundError
		var notConfirmed *obligation.HistoricalNotConfirmedError
		switch {
		case errors.As(err, &notFound):
			http.NotFound(w, r)
			return
		case errors.As(err, &validationError) && validationError.Field == "obligation_id":
			http.NotFound(w, r)
			return
		case errors.As(err, &validationError):
			status = http.StatusUnprocessableEntity
			message = validationError.Error()
		case errors.As(err, &notConfirmed):
			status = http.StatusUnprocessableEntity
			message = notConfirmed.Error()
		default:
			http.Error(w, "无法重放历史义务余额", http.StatusInternalServerError)
			return
		}
	}
	token, err := s.ensureCSRFToken(w, r)
	if err != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, status, s.historicalBalancePage, "historical_balance.html", historicalBalancePageData{
		Principal:    principal,
		CSRFToken:    token,
		ObligationID: obligationID,
		AsOfInput:    asOfInput,
		Snapshot:     snapshot,
		HasSnapshot:  hasSnapshot,
		Error:        message,
	})
}

func (s *Server) showObligations(w http.ResponseWriter, r *http.Request) {
	commandID, err := obligation.NewCommandID()
	if err != nil {
		http.Error(w, "无法建立义务草稿表单", http.StatusInternalServerError)
		return
	}
	s.renderObligationPage(w, r, http.StatusOK, obligationForm{
		CommandID: commandID,
		Direction: string(obligation.DirectionReceivable),
		Currency:  "CNY",
	}, "")
}

func (s *Server) createObligation(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	form := obligationForm{
		CommandID:             r.PostForm.Get("command_id"),
		Direction:             r.PostForm.Get("direction"),
		LedgerOwnerPartyID:    r.PostForm.Get("ledger_owner_party_id"),
		CounterpartyPartyID:   r.PostForm.Get("counterparty_party_id"),
		Title:                 r.PostForm.Get("title"),
		OriginalAmount:        r.PostForm.Get("original_amount"),
		Currency:              r.PostForm.Get("currency"),
		DueOn:                 r.PostForm.Get("due_on"),
		ManualSourceReference: r.PostForm.Get("manual_source_reference"),
	}
	result, err := s.obligations.CreateDraft(r.Context(), obligation.CreateDraftInput{
		CommandID:             form.CommandID,
		Direction:             form.Direction,
		LedgerOwnerPartyID:    form.LedgerOwnerPartyID,
		CounterpartyPartyID:   form.CounterpartyPartyID,
		Title:                 form.Title,
		OriginalAmount:        form.OriginalAmount,
		Currency:              form.Currency,
		DueOn:                 form.DueOn,
		ManualSourceReference: form.ManualSourceReference,
		Actor:                 principal.Actor(),
	})
	if err != nil {
		status := http.StatusInternalServerError
		message := "义务草稿创建失败"
		var validationError *obligation.ValidationError
		var partyNotFoundError *obligation.PartyNotFoundError
		var conflictError *obligation.IdempotencyConflictError
		switch {
		case errors.As(err, &validationError):
			status = http.StatusUnprocessableEntity
			message = validationError.Error()
		case errors.As(err, &partyNotFoundError):
			status = http.StatusUnprocessableEntity
			message = partyNotFoundError.Error()
		case errors.As(err, &conflictError):
			status = http.StatusConflict
			message = conflictError.Error()
		}
		if status == http.StatusInternalServerError {
			http.Error(w, message, status)
			return
		}
		s.renderObligationPage(w, r, status, form, message)
		return
	}
	http.Redirect(w, r, "/obligations/"+url.PathEscape(result.Obligation.ID)+"?created=1", http.StatusSeeOther)
}

func (s *Server) renderObligationPage(w http.ResponseWriter, r *http.Request, status int, form obligationForm, message string) {
	token, err := s.ensureCSRFToken(w, r)
	if err != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	parties, err := s.parties.List(r.Context())
	if err != nil {
		http.Error(w, "无法读取主体列表", http.StatusInternalServerError)
		return
	}
	obligations, err := s.obligations.List(r.Context())
	if err != nil {
		http.Error(w, "无法读取义务草稿列表", http.StatusInternalServerError)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, status, s.obligationPage, "obligations.html", obligationPageData{
		Principal:   principal,
		CSRFToken:   token,
		Form:        form,
		Parties:     parties,
		Obligations: obligations,
		Error:       message,
		CanCreate:   len(parties) >= 2,
	})
}

func (s *Server) showObligationDetail(w http.ResponseWriter, r *http.Request) {
	details, found, err := s.obligations.GetDetails(r.Context(), r.PathValue("obligationID"))
	if err != nil {
		var validationError *obligation.ValidationError
		if errors.As(err, &validationError) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "无法读取义务", http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	allocations, err := s.allocations.ListForObligation(r.Context(), details.Obligation.ID)
	if err != nil {
		http.Error(w, "无法读取义务的分配历史", http.StatusInternalServerError)
		return
	}
	token, err := s.ensureCSRFToken(w, r)
	if err != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	var confirmationCommand, adjustmentCommand, remissionCommand, closureCommand string
	if details.Obligation.Status == obligation.StatusDraft {
		confirmationCommand, err = obligation.NewCommandID()
	} else if details.Obligation.Status == obligation.StatusOpen {
		adjustmentCommand, err = obligation.NewCommandID()
		if err == nil {
			remissionCommand, err = obligation.NewCommandID()
		}
		if err == nil && details.Obligation.OutstandingAmount == "0" {
			closureCommand, err = obligation.NewCommandID()
		}
	}
	if err != nil {
		http.Error(w, "无法建立义务操作表单", http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, http.StatusOK, s.obligationDetail, "obligation_detail.html", obligationDetailData{
		Principal:           principal,
		CSRFToken:           token,
		Obligation:          details.Obligation,
		Revisions:           details.Revisions,
		Adjustments:         details.Adjustments,
		Remissions:          details.Remissions,
		Closure:             details.Closure,
		Allocations:         allocations,
		ConfirmationCommand: confirmationCommand,
		AdjustmentCommand:   adjustmentCommand,
		RemissionCommand:    remissionCommand,
		ClosureCommand:      closureCommand,
		AdjustmentType:      string(obligation.AdjustmentTypeIncrease),
		Created:             r.URL.Query().Get("created") == "1",
		Confirmed:           r.URL.Query().Get("confirmed") == "1",
		Adjusted:            r.URL.Query().Get("adjusted") == "1",
		Remitted:            r.URL.Query().Get("remitted") == "1",
		Closed:              r.URL.Query().Get("closed") == "1",
		CanAdjust:           details.Obligation.Status == obligation.StatusOpen,
		CanRemit:            details.Obligation.Status == obligation.StatusOpen,
		CanClose:            details.Obligation.Status == obligation.StatusOpen && details.Obligation.OutstandingAmount == "0",
	})
}

func (s *Server) confirmObligation(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	expectedDraftVersion, parseErr := strconv.ParseInt(strings.TrimSpace(r.PostForm.Get("expected_draft_version")), 10, 64)
	if parseErr != nil {
		expectedDraftVersion = 0
	}
	result, err := s.obligations.Confirm(r.Context(), obligation.ConfirmInput{
		CommandID:            r.PostForm.Get("command_id"),
		ObligationID:         r.PathValue("obligationID"),
		ExpectedDraftVersion: expectedDraftVersion,
		Actor:                principal.Actor(),
	})
	if err == nil {
		http.Redirect(w, r, "/obligations/"+url.PathEscape(result.Obligation.ID)+"?confirmed=1", http.StatusSeeOther)
		return
	}

	status := http.StatusInternalServerError
	message := "义务确认失败"
	var validationError *obligation.ValidationError
	var notFoundError *obligation.ObligationNotFoundError
	var idempotencyConflict *obligation.ConfirmationIdempotencyConflictError
	var stateConflict *obligation.ObligationStateConflictError
	var versionConflict *obligation.DraftVersionConflictError
	switch {
	case errors.As(err, &validationError):
		status = http.StatusUnprocessableEntity
		message = validationError.Error()
	case errors.As(err, &notFoundError):
		http.NotFound(w, r)
		return
	case errors.As(err, &idempotencyConflict):
		status = http.StatusConflict
		message = idempotencyConflict.Error()
	case errors.As(err, &stateConflict):
		status = http.StatusConflict
		message = stateConflict.Error()
	case errors.As(err, &versionConflict):
		status = http.StatusConflict
		message = versionConflict.Error()
	default:
		http.Error(w, message, status)
		return
	}

	details, found, loadErr := s.obligations.GetDetails(r.Context(), r.PathValue("obligationID"))
	if loadErr != nil {
		http.Error(w, "无法重新读取义务", http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	allocations, allocationsErr := s.allocations.ListForObligation(r.Context(), details.Obligation.ID)
	if allocationsErr != nil {
		http.Error(w, "无法读取义务的分配历史", http.StatusInternalServerError)
		return
	}
	commandID, commandErr := obligation.NewCommandID()
	if commandErr != nil {
		http.Error(w, "无法重建义务确认表单", http.StatusInternalServerError)
		return
	}
	token, tokenErr := s.ensureCSRFToken(w, r)
	if tokenErr != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, status, s.obligationDetail, "obligation_detail.html", obligationDetailData{
		Principal:           principal,
		CSRFToken:           token,
		Obligation:          details.Obligation,
		Revisions:           details.Revisions,
		Adjustments:         details.Adjustments,
		Remissions:          details.Remissions,
		Closure:             details.Closure,
		Allocations:         allocations,
		ConfirmationCommand: commandID,
		Error:               message,
	})
}

func (s *Server) adjustObligationAmount(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	expectedCurrentRevision, parseErr := strconv.ParseInt(strings.TrimSpace(r.PostForm.Get("expected_current_revision")), 10, 64)
	if parseErr != nil {
		expectedCurrentRevision = 0
	}
	adjustmentType := r.PostForm.Get("adjustment_type")
	adjustmentAmount := r.PostForm.Get("adjustment_amount")
	adjustmentReason := r.PostForm.Get("adjustment_reason")
	result, err := s.obligations.AdjustAmount(r.Context(), obligation.AdjustAmountInput{
		CommandID:               r.PostForm.Get("command_id"),
		ObligationID:            r.PathValue("obligationID"),
		ExpectedCurrentRevision: expectedCurrentRevision,
		Type:                    adjustmentType,
		Amount:                  adjustmentAmount,
		Reason:                  adjustmentReason,
		Actor:                   principal.Actor(),
	})
	if err == nil {
		http.Redirect(w, r, "/obligations/"+url.PathEscape(result.Obligation.ID)+"?adjusted=1", http.StatusSeeOther)
		return
	}

	status := http.StatusInternalServerError
	message := "义务金额调整失败"
	var validationError *obligation.ValidationError
	var notFoundError *obligation.ObligationNotFoundError
	var idempotencyConflict *obligation.AmountAdjustmentIdempotencyConflictError
	var stateConflict *obligation.AmountAdjustmentStateConflictError
	var revisionConflict *obligation.CurrentRevisionConflictError
	var resultConflict *obligation.AmountAdjustmentResultConflictError
	switch {
	case errors.As(err, &validationError):
		status = http.StatusUnprocessableEntity
		message = validationError.Error()
	case errors.As(err, &notFoundError):
		http.NotFound(w, r)
		return
	case errors.As(err, &idempotencyConflict):
		status = http.StatusConflict
		message = idempotencyConflict.Error()
	case errors.As(err, &stateConflict):
		status = http.StatusConflict
		message = stateConflict.Error()
	case errors.As(err, &revisionConflict):
		status = http.StatusConflict
		message = revisionConflict.Error()
	case errors.As(err, &resultConflict):
		status = http.StatusConflict
		message = resultConflict.Error()
	default:
		http.Error(w, message, status)
		return
	}

	details, found, loadErr := s.obligations.GetDetails(r.Context(), r.PathValue("obligationID"))
	if loadErr != nil {
		http.Error(w, "无法重新读取义务", http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	allocations, allocationsErr := s.allocations.ListForObligation(r.Context(), details.Obligation.ID)
	if allocationsErr != nil {
		http.Error(w, "无法读取义务的分配历史", http.StatusInternalServerError)
		return
	}
	commandID, commandErr := obligation.NewCommandID()
	if commandErr != nil {
		http.Error(w, "无法重建义务金额调整表单", http.StatusInternalServerError)
		return
	}
	remissionCommandID, remissionCommandErr := obligation.NewCommandID()
	if remissionCommandErr != nil {
		http.Error(w, "无法重建义务减免表单", http.StatusInternalServerError)
		return
	}
	closureCommandID := ""
	if details.Obligation.Status == obligation.StatusOpen && details.Obligation.OutstandingAmount == "0" {
		closureCommandID, commandErr = obligation.NewCommandID()
		if commandErr != nil {
			http.Error(w, "无法重建义务关闭表单", http.StatusInternalServerError)
			return
		}
	}
	token, tokenErr := s.ensureCSRFToken(w, r)
	if tokenErr != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, status, s.obligationDetail, "obligation_detail.html", obligationDetailData{
		Principal:         principal,
		CSRFToken:         token,
		Obligation:        details.Obligation,
		Revisions:         details.Revisions,
		Adjustments:       details.Adjustments,
		Remissions:        details.Remissions,
		Closure:           details.Closure,
		Allocations:       allocations,
		AdjustmentCommand: commandID,
		RemissionCommand:  remissionCommandID,
		ClosureCommand:    closureCommandID,
		AdjustmentType:    adjustmentType,
		AdjustmentAmount:  adjustmentAmount,
		AdjustmentReason:  adjustmentReason,
		CanAdjust:         details.Obligation.Status == obligation.StatusOpen,
		CanRemit:          details.Obligation.Status == obligation.StatusOpen,
		CanClose:          details.Obligation.Status == obligation.StatusOpen && details.Obligation.OutstandingAmount == "0",
		Error:             message,
	})
}

func (s *Server) recordObligationRemission(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	expectedCurrentRevision, parseErr := strconv.ParseInt(strings.TrimSpace(r.PostForm.Get("expected_current_revision")), 10, 64)
	if parseErr != nil {
		expectedCurrentRevision = 0
	}
	remissionAmount := r.PostForm.Get("remission_amount")
	remissionReason := r.PostForm.Get("remission_reason")
	remissionSource := r.PostForm.Get("manual_source_reference")
	result, err := s.obligations.RecordRemission(r.Context(), obligation.RecordRemissionInput{
		CommandID:               r.PostForm.Get("command_id"),
		ObligationID:            r.PathValue("obligationID"),
		ExpectedCurrentRevision: expectedCurrentRevision,
		Amount:                  remissionAmount,
		Reason:                  remissionReason,
		ManualSourceReference:   remissionSource,
		Actor:                   principal.Actor(),
	})
	if err == nil {
		http.Redirect(w, r, "/obligations/"+url.PathEscape(result.Obligation.ID)+"?remitted=1", http.StatusSeeOther)
		return
	}

	status := http.StatusInternalServerError
	message := "义务减免记录失败"
	var validationError *obligation.ValidationError
	var notFoundError *obligation.ObligationNotFoundError
	var idempotencyConflict *obligation.RemissionIdempotencyConflictError
	var stateConflict *obligation.RemissionStateConflictError
	var revisionConflict *obligation.CurrentRevisionConflictError
	var balanceConflict *obligation.RemissionBalanceConflictError
	switch {
	case errors.As(err, &validationError) && validationError.Field == "obligation_id":
		http.NotFound(w, r)
		return
	case errors.As(err, &validationError):
		status = http.StatusUnprocessableEntity
		message = validationError.Error()
	case errors.As(err, &notFoundError):
		http.NotFound(w, r)
		return
	case errors.As(err, &idempotencyConflict):
		status = http.StatusConflict
		message = idempotencyConflict.Error()
	case errors.As(err, &stateConflict):
		status = http.StatusConflict
		message = stateConflict.Error()
	case errors.As(err, &revisionConflict):
		status = http.StatusConflict
		message = revisionConflict.Error()
	case errors.As(err, &balanceConflict):
		status = http.StatusConflict
		message = balanceConflict.Error()
	default:
		http.Error(w, message, status)
		return
	}

	details, found, loadErr := s.obligations.GetDetails(r.Context(), r.PathValue("obligationID"))
	if loadErr != nil {
		http.Error(w, "无法重新读取义务", http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	allocations, allocationsErr := s.allocations.ListForObligation(r.Context(), details.Obligation.ID)
	if allocationsErr != nil {
		http.Error(w, "无法读取义务的分配历史", http.StatusInternalServerError)
		return
	}
	remissionCommandID, commandErr := obligation.NewCommandID()
	if commandErr != nil {
		http.Error(w, "无法重建义务减免表单", http.StatusInternalServerError)
		return
	}
	adjustmentCommandID, adjustmentCommandErr := obligation.NewCommandID()
	if adjustmentCommandErr != nil {
		http.Error(w, "无法重建义务金额调整表单", http.StatusInternalServerError)
		return
	}
	closureCommandID := ""
	if details.Obligation.Status == obligation.StatusOpen && details.Obligation.OutstandingAmount == "0" {
		closureCommandID, commandErr = obligation.NewCommandID()
		if commandErr != nil {
			http.Error(w, "无法重建义务关闭表单", http.StatusInternalServerError)
			return
		}
	}
	token, tokenErr := s.ensureCSRFToken(w, r)
	if tokenErr != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, status, s.obligationDetail, "obligation_detail.html", obligationDetailData{
		Principal:         principal,
		CSRFToken:         token,
		Obligation:        details.Obligation,
		Revisions:         details.Revisions,
		Adjustments:       details.Adjustments,
		Remissions:        details.Remissions,
		Closure:           details.Closure,
		Allocations:       allocations,
		AdjustmentCommand: adjustmentCommandID,
		AdjustmentType:    string(obligation.AdjustmentTypeIncrease),
		RemissionCommand:  remissionCommandID,
		RemissionAmount:   remissionAmount,
		RemissionReason:   remissionReason,
		RemissionSource:   remissionSource,
		ClosureCommand:    closureCommandID,
		CanAdjust:         details.Obligation.Status == obligation.StatusOpen,
		CanRemit:          details.Obligation.Status == obligation.StatusOpen,
		CanClose:          details.Obligation.Status == obligation.StatusOpen && details.Obligation.OutstandingAmount == "0",
		Error:             message,
	})
}

func (s *Server) closeObligation(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	expectedCurrentRevision, revisionParseErr := strconv.ParseInt(strings.TrimSpace(r.PostForm.Get("expected_current_revision")), 10, 64)
	if revisionParseErr != nil {
		expectedCurrentRevision = 0
	}
	expectedBalanceSequence, sequenceParseErr := strconv.ParseInt(strings.TrimSpace(r.PostForm.Get("expected_balance_sequence")), 10, 64)
	if sequenceParseErr != nil {
		expectedBalanceSequence = 0
	}
	closureReason := r.PostForm.Get("closure_reason")
	result, err := s.obligations.Close(r.Context(), obligation.CloseInput{
		CommandID:               r.PostForm.Get("command_id"),
		ObligationID:            r.PathValue("obligationID"),
		ExpectedCurrentRevision: expectedCurrentRevision,
		ExpectedBalanceSequence: expectedBalanceSequence,
		Reason:                  closureReason,
		Actor:                   principal.Actor(),
	})
	if err == nil {
		http.Redirect(w, r, "/obligations/"+url.PathEscape(result.Obligation.ID)+"?closed=1", http.StatusSeeOther)
		return
	}

	status := http.StatusInternalServerError
	message := "义务关闭失败"
	var validationError *obligation.ValidationError
	var notFoundError *obligation.ObligationNotFoundError
	var idempotencyConflict *obligation.ClosureIdempotencyConflictError
	var stateConflict *obligation.ClosureStateConflictError
	var revisionConflict *obligation.CurrentRevisionConflictError
	var sequenceConflict *obligation.BalanceSequenceConflictError
	var balanceConflict *obligation.ClosureBalanceConflictError
	switch {
	case errors.As(err, &validationError) && validationError.Field == "obligation_id":
		http.NotFound(w, r)
		return
	case errors.As(err, &validationError):
		status = http.StatusUnprocessableEntity
		message = validationError.Error()
	case errors.As(err, &notFoundError):
		http.NotFound(w, r)
		return
	case errors.As(err, &idempotencyConflict):
		status = http.StatusConflict
		message = idempotencyConflict.Error()
	case errors.As(err, &stateConflict):
		status = http.StatusConflict
		message = stateConflict.Error()
	case errors.As(err, &revisionConflict):
		status = http.StatusConflict
		message = revisionConflict.Error()
	case errors.As(err, &sequenceConflict):
		status = http.StatusConflict
		message = sequenceConflict.Error()
	case errors.As(err, &balanceConflict):
		status = http.StatusConflict
		message = balanceConflict.Error()
	default:
		http.Error(w, message, status)
		return
	}

	details, found, loadErr := s.obligations.GetDetails(r.Context(), r.PathValue("obligationID"))
	if loadErr != nil {
		http.Error(w, "无法重新读取义务", http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	allocations, allocationsErr := s.allocations.ListForObligation(r.Context(), details.Obligation.ID)
	if allocationsErr != nil {
		http.Error(w, "无法读取义务的分配历史", http.StatusInternalServerError)
		return
	}
	adjustmentCommandID := ""
	remissionCommandID := ""
	closureCommandID := ""
	if details.Obligation.Status == obligation.StatusOpen {
		adjustmentCommandID, loadErr = obligation.NewCommandID()
		if loadErr == nil {
			remissionCommandID, loadErr = obligation.NewCommandID()
		}
		if loadErr == nil && details.Obligation.OutstandingAmount == "0" {
			closureCommandID, loadErr = obligation.NewCommandID()
		}
	}
	if loadErr != nil {
		http.Error(w, "无法重建义务操作表单", http.StatusInternalServerError)
		return
	}
	token, tokenErr := s.ensureCSRFToken(w, r)
	if tokenErr != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, status, s.obligationDetail, "obligation_detail.html", obligationDetailData{
		Principal:         principal,
		CSRFToken:         token,
		Obligation:        details.Obligation,
		Revisions:         details.Revisions,
		Adjustments:       details.Adjustments,
		Remissions:        details.Remissions,
		Closure:           details.Closure,
		Allocations:       allocations,
		AdjustmentCommand: adjustmentCommandID,
		AdjustmentType:    string(obligation.AdjustmentTypeIncrease),
		RemissionCommand:  remissionCommandID,
		ClosureCommand:    closureCommandID,
		ClosureReason:     closureReason,
		CanAdjust:         details.Obligation.Status == obligation.StatusOpen,
		CanRemit:          details.Obligation.Status == obligation.StatusOpen,
		CanClose:          details.Obligation.Status == obligation.StatusOpen && details.Obligation.OutstandingAmount == "0",
		Error:             message,
	})
}

type partyForm struct {
	CommandID              string
	Kind                   string
	LegalName              string
	RegistrationIdentifier string
}

type partyPageData struct {
	Principal identity.Principal
	CSRFToken string
	Form      partyForm
	Parties   []party.Party
	Error     string
	Created   bool
}

func (s *Server) showParties(w http.ResponseWriter, r *http.Request) {
	commandID, err := party.NewCommandID()
	if err != nil {
		http.Error(w, "无法建立主体登记表单", http.StatusInternalServerError)
		return
	}
	s.renderPartyPage(w, r, http.StatusOK, partyForm{CommandID: commandID, Kind: string(party.KindOrganization)}, "", r.URL.Query().Get("created") == "1")
}

func (s *Server) createParty(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	if s.partyRuntime != nil {
		allowed, err := s.partyRuntime.LocalPartyWritesAllowed(r.Context(), principal.TenantID)
		if err != nil {
			http.Error(w, "主体目录状态暂时不可用", http.StatusServiceUnavailable)
			return
		}
		if !allowed {
			http.Error(w, "主体目录已切换到 Party Directory，本地主体登记保持冻结", http.StatusConflict)
			return
		}
	}
	form := partyForm{
		CommandID:              r.PostForm.Get("command_id"),
		Kind:                   r.PostForm.Get("kind"),
		LegalName:              r.PostForm.Get("legal_name"),
		RegistrationIdentifier: r.PostForm.Get("registration_identifier"),
	}
	result, err := s.parties.Create(r.Context(), party.CreateInput{
		CommandID:              form.CommandID,
		Kind:                   form.Kind,
		LegalName:              form.LegalName,
		RegistrationIdentifier: form.RegistrationIdentifier,
		Actor:                  principal.Actor(),
	})
	if err != nil {
		status := http.StatusInternalServerError
		message := "主体登记失败"
		var validationError *party.ValidationError
		var conflictError *party.IdempotencyConflictError
		switch {
		case errors.As(err, &validationError):
			status = http.StatusUnprocessableEntity
			message = validationError.Error()
		case errors.As(err, &conflictError):
			status = http.StatusConflict
			message = conflictError.Error()
		}
		if status == http.StatusInternalServerError {
			http.Error(w, message, status)
			return
		}
		s.renderPartyPage(w, r, status, form, message, false)
		return
	}
	http.Redirect(w, r, "/parties?created=1&party_id="+url.QueryEscape(result.Party.ID), http.StatusSeeOther)
}

func (s *Server) renderPartyPage(w http.ResponseWriter, r *http.Request, status int, form partyForm, message string, created bool) {
	token, err := s.ensureCSRFToken(w, r)
	if err != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	parties, err := s.parties.List(r.Context())
	if err != nil {
		http.Error(w, "无法读取主体列表", http.StatusInternalServerError)
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		http.Error(w, "无法确定当前用户", http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, status, s.partyPage, "parties.html", partyPageData{
		Principal: principal,
		CSRFToken: token,
		Form:      form,
		Parties:   parties,
		Error:     message,
		Created:   created,
	})
}

func principalFromContext(ctx context.Context) (identity.Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(identity.Principal)
	return principal, ok
}

func (s *Server) renderTemplate(w http.ResponseWriter, status int, page *template.Template, name string, data any) {
	var rendered bytes.Buffer
	if err := page.ExecuteTemplate(&rendered, name, data); err != nil {
		http.Error(w, "页面渲染失败", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = rendered.WriteTo(w)
}

func (s *Server) ensureCSRFToken(w http.ResponseWriter, r *http.Request) (string, error) {
	if cookie, err := r.Cookie(csrfCookieName); err == nil && validCSRFTokenShape(cookie.Value) {
		return cookie.Value, nil
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.sessionCookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int((12 * time.Hour).Seconds()),
	})
	return token, nil
}

func validCSRFToken(r *http.Request) bool {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || !validCSRFTokenShape(cookie.Value) {
		return false
	}
	formToken := strings.TrimSpace(r.PostForm.Get("csrf_token"))
	if !validCSRFTokenShape(formToken) || len(cookie.Value) != len(formToken) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(formToken)) == 1
}

func validCSRFTokenShape(token string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(decoded) == 32
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.sessionCookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
	})
}

func (s *Server) clearCSRFCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.sessionCookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'self'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}
