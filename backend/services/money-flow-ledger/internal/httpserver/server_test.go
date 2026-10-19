package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/allocation"
	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/cashtransaction"
	"omsbizfoundry/money-flow-ledger/internal/identity"
	"omsbizfoundry/money-flow-ledger/internal/internaltransfer"
	"omsbizfoundry/money-flow-ledger/internal/ledgerexport"
	"omsbizfoundry/money-flow-ledger/internal/obligation"
	"omsbizfoundry/money-flow-ledger/internal/operatingsummary"
	"omsbizfoundry/money-flow-ledger/internal/party"
)

type readinessStub struct{ err error }

func (s readinessStub) Ping(context.Context) error { return s.err }

type identityStub struct {
	loginResult identity.LoginResult
	loginErr    error
	principal   identity.Principal
	found       bool
	resolveErr  error
	logoutErr   error
	loginUser   string
	loginPass   string
	resolved    []string
	loggedOut   []string
}

func (s *identityStub) Login(_ context.Context, username, password string) (identity.LoginResult, error) {
	s.loginUser = username
	s.loginPass = password
	return s.loginResult, s.loginErr
}

func (s *identityStub) ResolveSession(_ context.Context, token string) (identity.Principal, bool, error) {
	s.resolved = append(s.resolved, token)
	return s.principal, s.found, s.resolveErr
}

func (s *identityStub) Logout(_ context.Context, token string) error {
	s.loggedOut = append(s.loggedOut, token)
	return s.logoutErr
}

type partyStub struct {
	createInput  party.CreateInput
	createResult party.CreateResult
	createErr    error
	listResult   []party.Party
	listErr      error
	createCalls  int
	listCalls    int
}

func (s *partyStub) Create(_ context.Context, input party.CreateInput) (party.CreateResult, error) {
	s.createCalls++
	s.createInput = input
	return s.createResult, s.createErr
}

func (s *partyStub) List(context.Context) ([]party.Party, error) {
	s.listCalls++
	return s.listResult, s.listErr
}

type obligationStub struct {
	createInput           obligation.CreateDraftInput
	createResult          obligation.CreateResult
	createErr             error
	confirmInput          obligation.ConfirmInput
	confirmResult         obligation.ConfirmResult
	confirmErr            error
	adjustInput           obligation.AdjustAmountInput
	adjustResult          obligation.AdjustAmountResult
	adjustErr             error
	remissionInput        obligation.RecordRemissionInput
	remissionResult       obligation.RecordRemissionResult
	remissionErr          error
	closeInput            obligation.CloseInput
	closeResult           obligation.CloseResult
	closeErr              error
	agingInput            obligation.CurrentAgingInput
	agingResult           obligation.AgingReport
	agingErr              error
	historicalInput       obligation.HistoricalBalanceInput
	historicalResult      obligation.HistoricalSnapshot
	historicalErr         error
	listResult            []obligation.Obligation
	listErr               error
	getResult             obligation.Obligation
	detailsResult         obligation.Details
	getFound              bool
	getErr                error
	getID                 string
	createCalls           int
	confirmCalls          int
	adjustCalls           int
	remissionCalls        int
	closeCalls            int
	agingCalls            int
	historicalCalls       int
	listCalls             int
	getCalls              int
	cashExpectationResult obligation.RecordCashExpectationResult
	cashExpectationErr    error
	cashExpectationInput  obligation.RecordCashExpectationInput
	cashExpectationCalls  int
	cashForecastResult    obligation.CashForecastReport
	cashForecastErr       error
	cashForecastCalls     int
	cashHistoryResult     obligation.CashExpectationHistory
	cashHistoryErr        error
	cashHistoryID         string
	cashHistoryCalls      int
}

type cashTransactionStub struct {
	createInput   cashtransaction.CreateInput
	createResult  cashtransaction.CreateResult
	createErr     error
	reverseInput  cashtransaction.ReverseInput
	reverseResult cashtransaction.ReverseResult
	reverseErr    error
	listResult    []cashtransaction.Transaction
	listErr       error
	getResult     cashtransaction.Transaction
	getFound      bool
	getErr        error
	getID         string
	createCalls   int
	reverseCalls  int
	listCalls     int
	getCalls      int
}

type allocationStub struct {
	applyInput              allocation.ApplyInput
	applyResult             allocation.ApplyResult
	applyErr                error
	reverseInput            allocation.ReverseInput
	reverseResult           allocation.ReverseResult
	reverseErr              error
	listResult              []allocation.Allocation
	listErr                 error
	listForCashResult       []allocation.Allocation
	listForCashErr          error
	listForCashID           string
	listForObligationResult []allocation.Allocation
	listForObligationErr    error
	listForObligationID     string
	getResult               allocation.Allocation
	getFound                bool
	getErr                  error
	getID                   string
	applyCalls              int
	reverseCalls            int
	listCalls               int
	listForCashCalls        int
	listForObligationCalls  int
	getCalls                int
}

type internalTransferStub struct {
	createInput  internaltransfer.CreateInput
	createResult internaltransfer.CreateResult
	createErr    error
	listResult   []internaltransfer.Transfer
	listErr      error
	getResult    internaltransfer.Transfer
	getFound     bool
	getErr       error
	getID        string
	createCalls  int
	listCalls    int
	getCalls     int
}

type operatingSummaryStub struct {
	currentResult   operatingsummary.Report
	currentErr      error
	drilldownInput  operatingsummary.DrilldownInput
	drilldownResult operatingsummary.Drilldown
	drilldownErr    error
	currentCalls    int
	drilldownCalls  int
}

type ledgerExportStub struct {
	input    ledgerexport.Input
	artifact ledgerexport.Artifact
	err      error
	calls    int
}

func (s *ledgerExportStub) Export(_ context.Context, input ledgerexport.Input) (ledgerexport.Artifact, error) {
	s.calls++
	s.input = input
	return s.artifact, s.err
}

func (s *operatingSummaryStub) Current(context.Context) (operatingsummary.Report, error) {
	s.currentCalls++
	return s.currentResult, s.currentErr
}

func (s *operatingSummaryStub) Drilldown(_ context.Context, input operatingsummary.DrilldownInput) (operatingsummary.Drilldown, error) {
	s.drilldownCalls++
	s.drilldownInput = input
	return s.drilldownResult, s.drilldownErr
}

func (s *internalTransferStub) Create(_ context.Context, input internaltransfer.CreateInput) (internaltransfer.CreateResult, error) {
	s.createCalls++
	s.createInput = input
	return s.createResult, s.createErr
}

func (s *internalTransferStub) List(context.Context) ([]internaltransfer.Transfer, error) {
	s.listCalls++
	return s.listResult, s.listErr
}

func (s *internalTransferStub) Get(_ context.Context, id string) (internaltransfer.Transfer, bool, error) {
	s.getCalls++
	s.getID = id
	return s.getResult, s.getFound, s.getErr
}

func (s *allocationStub) Apply(_ context.Context, input allocation.ApplyInput) (allocation.ApplyResult, error) {
	s.applyCalls++
	s.applyInput = input
	return s.applyResult, s.applyErr
}

func (s *allocationStub) Reverse(_ context.Context, input allocation.ReverseInput) (allocation.ReverseResult, error) {
	s.reverseCalls++
	s.reverseInput = input
	return s.reverseResult, s.reverseErr
}

func (s *allocationStub) List(context.Context) ([]allocation.Allocation, error) {
	s.listCalls++
	return s.listResult, s.listErr
}

func (s *allocationStub) ListForCashTransaction(_ context.Context, id string) ([]allocation.Allocation, error) {
	s.listForCashCalls++
	s.listForCashID = id
	return s.listForCashResult, s.listForCashErr
}

func (s *allocationStub) ListForObligation(_ context.Context, id string) ([]allocation.Allocation, error) {
	s.listForObligationCalls++
	s.listForObligationID = id
	return s.listForObligationResult, s.listForObligationErr
}

func (s *allocationStub) Get(_ context.Context, id string) (allocation.Allocation, bool, error) {
	s.getCalls++
	s.getID = id
	return s.getResult, s.getFound, s.getErr
}

func (s *cashTransactionStub) Create(_ context.Context, input cashtransaction.CreateInput) (cashtransaction.CreateResult, error) {
	s.createCalls++
	s.createInput = input
	return s.createResult, s.createErr
}

func (s *cashTransactionStub) Reverse(_ context.Context, input cashtransaction.ReverseInput) (cashtransaction.ReverseResult, error) {
	s.reverseCalls++
	s.reverseInput = input
	return s.reverseResult, s.reverseErr
}

func (s *cashTransactionStub) List(context.Context) ([]cashtransaction.Transaction, error) {
	s.listCalls++
	return s.listResult, s.listErr
}

func (s *cashTransactionStub) Get(_ context.Context, id string) (cashtransaction.Transaction, bool, error) {
	s.getCalls++
	s.getID = id
	return s.getResult, s.getFound, s.getErr
}

func (s *obligationStub) CreateDraft(_ context.Context, input obligation.CreateDraftInput) (obligation.CreateResult, error) {
	s.createCalls++
	s.createInput = input
	return s.createResult, s.createErr
}

func (s *obligationStub) List(context.Context) ([]obligation.Obligation, error) {
	s.listCalls++
	return s.listResult, s.listErr
}

func (s *obligationStub) CurrentAging(_ context.Context, input obligation.CurrentAgingInput) (obligation.AgingReport, error) {
	s.agingCalls++
	s.agingInput = input
	return s.agingResult, s.agingErr
}

func (s *obligationStub) HistoricalBalance(_ context.Context, input obligation.HistoricalBalanceInput) (obligation.HistoricalSnapshot, error) {
	s.historicalCalls++
	s.historicalInput = input
	return s.historicalResult, s.historicalErr
}

func (s *obligationStub) RecordCashExpectation(_ context.Context, input obligation.RecordCashExpectationInput) (obligation.RecordCashExpectationResult, error) {
	s.cashExpectationCalls++
	s.cashExpectationInput = input
	return s.cashExpectationResult, s.cashExpectationErr
}

func (s *obligationStub) CurrentCashForecast(context.Context) (obligation.CashForecastReport, error) {
	s.cashForecastCalls++
	return s.cashForecastResult, s.cashForecastErr
}

func (s *obligationStub) CashExpectationHistory(_ context.Context, id string) (obligation.CashExpectationHistory, error) {
	s.cashHistoryCalls++
	s.cashHistoryID = id
	return s.cashHistoryResult, s.cashHistoryErr
}

func (s *obligationStub) Confirm(_ context.Context, input obligation.ConfirmInput) (obligation.ConfirmResult, error) {
	s.confirmCalls++
	s.confirmInput = input
	return s.confirmResult, s.confirmErr
}

func (s *obligationStub) AdjustAmount(_ context.Context, input obligation.AdjustAmountInput) (obligation.AdjustAmountResult, error) {
	s.adjustCalls++
	s.adjustInput = input
	return s.adjustResult, s.adjustErr
}

func (s *obligationStub) RecordRemission(_ context.Context, input obligation.RecordRemissionInput) (obligation.RecordRemissionResult, error) {
	s.remissionCalls++
	s.remissionInput = input
	return s.remissionResult, s.remissionErr
}

func (s *obligationStub) Close(_ context.Context, input obligation.CloseInput) (obligation.CloseResult, error) {
	s.closeCalls++
	s.closeInput = input
	return s.closeResult, s.closeErr
}

func (s *obligationStub) Get(_ context.Context, id string) (obligation.Obligation, bool, error) {
	s.getCalls++
	s.getID = id
	return s.getResult, s.getFound, s.getErr
}

func (s *obligationStub) GetDetails(_ context.Context, id string) (obligation.Details, bool, error) {
	s.getCalls++
	s.getID = id
	if s.detailsResult.Obligation.ID != "" || len(s.detailsResult.Revisions) > 0 || len(s.detailsResult.Remissions) > 0 {
		return s.detailsResult, s.getFound, s.getErr
	}
	return obligation.Details{Obligation: s.getResult}, s.getFound, s.getErr
}

func newHandler(t *testing.T, readiness ReadinessChecker, identities IdentityUseCases, secure bool) http.Handler {
	t.Helper()
	handler, err := New(readiness, identities, &partyStub{}, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{SessionCookieSecure: secure}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return handler
}

func TestNewRequiresDependencies(t *testing.T) {
	if _, err := New(nil, &identityStub{}, &partyStub{}, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{}); err == nil {
		t.Fatal("New(nil readiness) error = nil")
	}
	if _, err := New(readinessStub{}, nil, &partyStub{}, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{}); err == nil {
		t.Fatal("New(nil identities) error = nil")
	}
	if _, err := New(readinessStub{}, &identityStub{}, nil, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{}); err == nil {
		t.Fatal("New(nil parties) error = nil")
	}
	if _, err := New(readinessStub{}, &identityStub{}, &partyStub{}, nil, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{}); err == nil {
		t.Fatal("New(nil obligations) error = nil")
	}
	if _, err := New(readinessStub{}, &identityStub{}, &partyStub{}, &obligationStub{}, nil, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{}); err == nil {
		t.Fatal("New(nil cash transactions) error = nil")
	}
	if _, err := New(readinessStub{}, &identityStub{}, &partyStub{}, &obligationStub{}, &cashTransactionStub{}, nil, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{}); err == nil {
		t.Fatal("New(nil allocations) error = nil")
	}
	if _, err := New(readinessStub{}, &identityStub{}, &partyStub{}, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, nil, &ledgerExportStub{}, Options{}, &internalTransferStub{}); err == nil {
		t.Fatal("New(nil operating summaries) error = nil")
	}
	if _, err := New(readinessStub{}, &identityStub{}, &partyStub{}, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, nil, Options{}, &internalTransferStub{}); err == nil {
		t.Fatal("New(nil ledger exports) error = nil")
	}
	if _, err := New(readinessStub{}, &identityStub{}, &partyStub{}, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, nil); err == nil {
		t.Fatal("New(nil internal transfers) error = nil")
	}
}

func TestHealthEndpointsRemainPublic(t *testing.T) {
	handler := newHandler(t, readinessStub{}, &identityStub{}, false)
	for _, path := range []string{"/health/live", "/health/ready"} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), serviceName) {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
			if response.Header().Get("X-Frame-Options") != "DENY" {
				t.Fatalf("security headers = %#v", response.Header())
			}
		})
	}
}

func TestReadinessFailure(t *testing.T) {
	handler := newHandler(t, readinessStub{err: errors.New("database unavailable")}, &identityStub{}, false)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestProtectedHomeRedirectsToLocalLogin(t *testing.T) {
	handler := newHandler(t, readinessStub{}, &identityStub{}, false)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/?tab=summary", nil))
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login?next=%2F%3Ftab%3Dsummary" {
		t.Fatalf("status=%d location=%q", response.Code, response.Header().Get("Location"))
	}

	postResponse := httptest.NewRecorder()
	handler.ServeHTTP(postResponse, httptest.NewRequest(http.MethodPost, "/logout", nil))
	if postResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated POST status = %d", postResponse.Code)
	}
}

func TestIAMModeUsesCapabilityCookiePrefixesLinksAndEnforcesAction(t *testing.T) {
	principal := identity.Principal{
		UserID: "11111111-1111-4111-8111-111111111111", DisplayName: "中心资金用户",
		Provider: "iam", TenantID: "22222222-2222-4222-8222-222222222222",
		AllowedActions: map[string]bool{"money-flow-ledger.view": true},
	}
	identities := &identityStub{found: true, principal: principal}
	handler, err := New(readinessStub{}, identities, &partyStub{}, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{ExternalIdentity: true, SessionCookieName: "oms_cap_money_flow_ledger", PublicBasePath: "/money"}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/", nil))
	if missing.Code != http.StatusUnauthorized {
		t.Fatalf("missing IAM credential status=%d", missing.Code)
	}
	request := httptest.NewRequest(http.MethodGet, "/parties", nil)
	request.AddCookie(&http.Cookie{Name: "oms_cap_money_flow_ledger", Value: "valid-session"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `href="/money/"`) || !strings.Contains(response.Body.String(), `action="/gateway/logout"`) {
		t.Fatalf("IAM page status=%d body=%q", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), `action="/money/parties"`) {
		t.Fatalf("view-only principal saw write form: %s", response.Body.String())
	}
	write := httptest.NewRequest(http.MethodPost, "/parties", nil)
	write.AddCookie(&http.Cookie{Name: "oms_cap_money_flow_ledger", Value: "valid-session"})
	writeResponse := httptest.NewRecorder()
	handler.ServeHTTP(writeResponse, write)
	if writeResponse.Code != http.StatusForbidden {
		t.Fatalf("view-only principal wrote party: %d", writeResponse.Code)
	}
	identities.principal.AllowedActions["money-flow-ledger.party.register"] = true
	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/parties", nil)
	request.AddCookie(&http.Cookie{Name: "oms_cap_money_flow_ledger", Value: "valid-session"})
	handler.ServeHTTP(response, request)
	if !strings.Contains(response.Body.String(), `action="/money/parties"`) {
		t.Fatalf("party.register principal missing write form: %s", response.Body.String())
	}
	login := httptest.NewRecorder()
	handler.ServeHTTP(login, httptest.NewRequest(http.MethodGet, "/login", nil))
	if login.Code != http.StatusUnauthorized {
		t.Fatalf("IAM mode local login status=%d", login.Code)
	}
}

func TestOperatingMetricProjectionScopesCurrentTenantAndSeparatesAmountPermission(t *testing.T) {
	tenantID := "22222222-2222-4222-8222-222222222222"
	ownerID := "55555555-5555-4555-8555-555555555555"
	observedAt := time.Date(2026, 9, 29, 9, 30, 0, 0, time.UTC)
	summaries := &operatingSummaryStub{currentResult: operatingsummary.Report{
		GeneratedAt: observedAt,
		Groups: []operatingsummary.Group{{
			LedgerOwnerPartyID: ownerID, LedgerOwnerLegalName: "本方主体", Currency: "CNY",
			ReceivableOutstanding: operatingsummary.AmountMetric{Metric: operatingsummary.MetricReceivableOutstanding, Amount: "12.5", FactCount: 2},
			PayableOutstanding:    operatingsummary.AmountMetric{Metric: operatingsummary.MetricPayableOutstanding, Amount: "3", FactCount: 1},
			ReceiptEffective:      operatingsummary.AmountMetric{Metric: operatingsummary.MetricReceiptEffective, Amount: "20", FactCount: 3},
			PaymentEffective:      operatingsummary.AmountMetric{Metric: operatingsummary.MetricPaymentEffective, Amount: "4", FactCount: 1},
			ReceiptUnallocated:    operatingsummary.AmountMetric{Metric: operatingsummary.MetricReceiptUnallocated, Amount: "2", FactCount: 1},
			PaymentUnallocated:    operatingsummary.AmountMetric{Metric: operatingsummary.MetricPaymentUnallocated, Amount: "1", FactCount: 1},
		}},
	}}
	principal := identity.Principal{
		UserID: "11111111-1111-4111-8111-111111111111", DisplayName: "经营查看人",
		Provider: "iam", TenantID: tenantID, ScopeKind: "tenant", Resources: map[string]map[string]bool{},
		AllowedActions: map[string]bool{
			"money-flow-ledger.statistics.read": true,
			"money-flow-ledger.amount.read":     true,
		},
	}
	identities := &identityStub{found: true, principal: principal}
	handler, err := New(readinessStub{}, identities, &partyStub{}, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, summaries, &ledgerExportStub{}, Options{ExternalIdentity: true, SessionCookieName: "oms_cap_money_flow_ledger"}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/metrics/operating-summary", nil)
	request.AddCookie(&http.Cookie{Name: "oms_cap_money_flow_ledger", Value: "credential"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d headers=%#v body=%q", response.Code, response.Header(), response.Body.String())
	}
	var projection metricProjection
	if err := json.Unmarshal(response.Body.Bytes(), &projection); err != nil {
		t.Fatal(err)
	}
	if projection.SchemaVersion != "1" || projection.SourceApplication != serviceName || projection.TenantID != tenantID || projection.DefinitionVersion != operatingMetricDefinitionVersion || projection.ObservedAt != observedAt.Format(time.RFC3339Nano) || len(projection.Items) != 6 {
		t.Fatalf("projection=%#v", projection)
	}
	first := projection.Items[0]
	if first.MetricKey != string(operatingsummary.MetricReceivableOutstanding) || first.Scope.ResourceID != ownerID || first.Scope.TenantID != tenantID || first.Value == nil || *first.Value != "12.5" || first.Redacted || first.SourceDigest == nil || !strings.HasPrefix(*first.SourceDigest, "sha256:") || first.DeepLink == "" {
		t.Fatalf("first metric=%#v", first)
	}

	delete(identities.principal.AllowedActions, "money-flow-ledger.amount.read")
	request = httptest.NewRequest(http.MethodGet, "/api/v1/metrics/operating-summary", nil)
	request.AddCookie(&http.Cookie{Name: "oms_cap_money_flow_ledger", Value: "credential"})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "12.5") {
		t.Fatalf("redacted status=%d body=%q", response.Code, response.Body.String())
	}
	projection = metricProjection{}
	if err := json.Unmarshal(response.Body.Bytes(), &projection); err != nil {
		t.Fatal(err)
	}
	if len(projection.Items) != 6 || projection.Items[0].Value != nil || !projection.Items[0].Redacted || projection.Items[0].SourceDigest != nil {
		t.Fatalf("redacted metric=%#v", projection.Items[0])
	}

	delete(identities.principal.AllowedActions, "money-flow-ledger.statistics.read")
	request = httptest.NewRequest(http.MethodGet, "/api/v1/metrics/operating-summary", nil)
	request.AddCookie(&http.Cookie{Name: "oms_cap_money_flow_ledger", Value: "credential"})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || summaries.currentCalls != 2 {
		t.Fatalf("forbidden status=%d calls=%d", response.Code, summaries.currentCalls)
	}

	identities.principal.AllowedActions["money-flow-ledger.statistics.read"] = true
	summaries.currentErr = errors.New("database unavailable")
	request = httptest.NewRequest(http.MethodGet, "/api/v1/metrics/operating-summary", nil)
	request.AddCookie(&http.Cookie{Name: "oms_cap_money_flow_ledger", Value: "credential"})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "来源暂时不可用") {
		t.Fatalf("unavailable status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestLoginPageCreatesSecureCSRFToken(t *testing.T) {
	handler := newHandler(t, readinessStub{}, &identityStub{}, true)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/login?next=https://evil.example", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "登录 Money Flow Ledger") ||
		!strings.Contains(response.Body.String(), `name="next" value="/"`) {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	cookie := findCookie(t, response.Result().Cookies(), csrfCookieName)
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || !validCSRFTokenShape(cookie.Value) ||
		!strings.Contains(response.Body.String(), `value="`+cookie.Value+`"`) {
		t.Fatalf("csrf cookie = %#v body=%q", cookie, response.Body.String())
	}
}

func TestLoginRejectsMissingCSRF(t *testing.T) {
	handler := newHandler(t, readinessStub{}, &identityStub{}, false)
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(url.Values{
		"username": {"admin"}, "password": {"correct-password"},
	}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestLoginFailureUsesGenericError(t *testing.T) {
	identities := &identityStub{loginErr: &identity.AuthenticationError{}}
	handler := newHandler(t, readinessStub{}, identities, false)
	token := csrfToken()
	request := formRequest(http.MethodPost, "/login", url.Values{
		"csrf_token": {token}, "username": {"<admin>"}, "password": {"wrong"}, "next": {"/"},
	})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), "用户名或密码错误") ||
		strings.Contains(response.Body.String(), `value="<admin>"`) {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	if identities.loginUser != "<admin>" || identities.loginPass != "wrong" {
		t.Fatalf("login input = %q %q", identities.loginUser, identities.loginPass)
	}
}

func TestSuccessfulLoginSetsSessionCookieAndSanitizesRedirect(t *testing.T) {
	identities := &identityStub{loginResult: identity.LoginResult{
		Principal: identity.Principal{UserID: "11111111-1111-4111-8111-111111111111", Username: "admin", DisplayName: "资金管理员", Role: identity.RoleAdministrator},
		Token:     "session-token",
		ExpiresAt: time.Now().Add(time.Hour),
	}}
	handler := newHandler(t, readinessStub{}, identities, true)
	token := csrfToken()
	request := formRequest(http.MethodPost, "/login", url.Values{
		"csrf_token": {token}, "username": {"admin"}, "password": {"correct-password"}, "next": {"//evil.example"},
	})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/" {
		t.Fatalf("status=%d location=%q", response.Code, response.Header().Get("Location"))
	}
	cookie := findCookie(t, response.Result().Cookies(), sessionCookieName)
	if cookie.Value != "session-token" || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge < 1 {
		t.Fatalf("session cookie = %#v", cookie)
	}
}

func TestAuthenticatedHomeUsesResolvedPrincipal(t *testing.T) {
	identities := &identityStub{
		found: true,
		principal: identity.Principal{
			UserID: "11111111-1111-4111-8111-111111111111", Username: "admin", DisplayName: "资金管理员", Role: identity.RoleAdministrator,
		},
	}
	handler := newHandler(t, readinessStub{}, identities, false)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "valid-session"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "资金管理员") ||
		!strings.Contains(response.Body.String(), string(identity.RoleAdministrator)) || !strings.Contains(response.Body.String(), "注销") {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	if len(identities.resolved) != 1 || identities.resolved[0] != "valid-session" {
		t.Fatalf("resolved tokens = %#v", identities.resolved)
	}
	csrf := findCookie(t, response.Result().Cookies(), csrfCookieName)
	if !strings.Contains(response.Body.String(), csrf.Value) {
		t.Fatal("home form does not contain CSRF token")
	}
}

func TestInvalidSessionIsClearedAndRedirected(t *testing.T) {
	handler := newHandler(t, readinessStub{}, &identityStub{}, false)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "invalid"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login?next=%2F" {
		t.Fatalf("status=%d location=%q", response.Code, response.Header().Get("Location"))
	}
	if findCookie(t, response.Result().Cookies(), sessionCookieName).MaxAge != -1 {
		t.Fatal("invalid session cookie was not cleared")
	}
}

func TestLogoutRequiresCSRFAndRevokesServerSession(t *testing.T) {
	identities := &identityStub{found: true, principal: identity.Principal{UserID: "11111111-1111-4111-8111-111111111111"}}
	handler := newHandler(t, readinessStub{}, identities, false)

	withoutCSRF := formRequest(http.MethodPost, "/logout", url.Values{})
	withoutCSRF.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "valid-session"})
	withoutResponse := httptest.NewRecorder()
	handler.ServeHTTP(withoutResponse, withoutCSRF)
	if withoutResponse.Code != http.StatusForbidden || len(identities.loggedOut) != 0 {
		t.Fatalf("without CSRF status=%d logout=%#v", withoutResponse.Code, identities.loggedOut)
	}

	token := csrfToken()
	request := formRequest(http.MethodPost, "/logout", url.Values{"csrf_token": {token}})
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "valid-session"})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login" || len(identities.loggedOut) != 1 || identities.loggedOut[0] != "valid-session" {
		t.Fatalf("status=%d location=%q logout=%#v", response.Code, response.Header().Get("Location"), identities.loggedOut)
	}
	if findCookie(t, response.Result().Cookies(), sessionCookieName).MaxAge != -1 ||
		findCookie(t, response.Result().Cookies(), csrfCookieName).MaxAge != -1 {
		t.Fatal("logout did not clear cookies")
	}
}

func TestAdministratorCanOpenPartyList(t *testing.T) {
	identities := administratorIdentityStub()
	parties := &partyStub{listResult: []party.Party{
		{ID: "22222222-2222-4222-8222-222222222222", Kind: party.KindOrganization, LegalName: "示例公司", RegistrationIdentifier: "91310000TEST", Revision: 1},
		{ID: "33333333-3333-4333-8333-333333333333", Kind: party.KindIndividual, LegalName: "张三", Revision: 1},
	}}
	handler, err := New(readinessStub{}, identities, parties, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	request := authenticatedRequest(http.MethodGet, "/parties", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "主体目录") ||
		!strings.Contains(response.Body.String(), "示例公司") || !strings.Contains(response.Body.String(), "张三") ||
		!strings.Contains(response.Body.String(), `name="command_id" value="`) {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	if parties.listCalls != 1 {
		t.Fatalf("list calls = %d", parties.listCalls)
	}
	csrf := findCookie(t, response.Result().Cookies(), csrfCookieName)
	if !strings.Contains(response.Body.String(), csrf.Value) {
		t.Fatal("party form does not contain CSRF token")
	}
}

func TestAdministratorCreatesPartyWithPrincipalActor(t *testing.T) {
	identities := administratorIdentityStub()
	parties := &partyStub{createResult: party.CreateResult{Party: party.Party{ID: "44444444-4444-4444-8444-444444444444"}}}
	handler, err := New(readinessStub{}, identities, parties, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	token := csrfToken()
	request := authenticatedRequest(http.MethodPost, "/parties", url.Values{
		"csrf_token":              {token},
		"command_id":              {"55555555-5555-4555-8555-555555555555"},
		"kind":                    {"organization"},
		"legal_name":              {"  示例公司  "},
		"registration_identifier": {" 91310000TEST "},
		"actor_kind":              {"forged"},
		"actor_id":                {"forged"},
		"actor_display_name":      {"伪造用户"},
	})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/parties?created=1&party_id=44444444-4444-4444-8444-444444444444" {
		t.Fatalf("status=%d location=%q body=%q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	if parties.createCalls != 1 || parties.createInput.CommandID != "55555555-5555-4555-8555-555555555555" ||
		parties.createInput.LegalName != "  示例公司  " || parties.createInput.RegistrationIdentifier != " 91310000TEST " {
		t.Fatalf("create input = %#v", parties.createInput)
	}
	wantActor := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: identities.principal.UserID, DisplayName: identities.principal.DisplayName}
	if parties.createInput.Actor != wantActor {
		t.Fatalf("actor = %#v, want %#v", parties.createInput.Actor, wantActor)
	}
}

func TestPartyCreateValidationAndConflictRenderWithoutLosingIntent(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "validation", err: &party.ValidationError{Field: "legal_name", Message: "法定名称不能为空"}, status: http.StatusUnprocessableEntity},
		{name: "conflict", err: &party.IdempotencyConflictError{CommandID: "55555555-5555-4555-8555-555555555555"}, status: http.StatusConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			identities := administratorIdentityStub()
			parties := &partyStub{createErr: test.err}
			handler, err := New(readinessStub{}, identities, parties, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			token := csrfToken()
			request := authenticatedRequest(http.MethodPost, "/parties", url.Values{
				"csrf_token": {token}, "command_id": {"55555555-5555-4555-8555-555555555555"},
				"kind": {"individual"}, "legal_name": {"保留的姓名"},
			})
			request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status || !strings.Contains(response.Body.String(), test.err.Error()) ||
				!strings.Contains(response.Body.String(), `name="command_id" value="55555555-5555-4555-8555-555555555555"`) ||
				!strings.Contains(response.Body.String(), `value="保留的姓名"`) {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestPartyEndpointsRequireAdministrator(t *testing.T) {
	identities := administratorIdentityStub()
	identities.principal.Role = identity.Role("viewer")
	parties := &partyStub{}
	handler, err := New(readinessStub{}, identities, parties, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	for _, request := range []*http.Request{
		authenticatedRequest(http.MethodGet, "/parties", nil),
		authenticatedRequest(http.MethodPost, "/parties", url.Values{"legal_name": {"越权主体"}}),
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("%s status = %d", request.Method, response.Code)
		}
	}
	if parties.createCalls != 0 || parties.listCalls != 0 {
		t.Fatalf("party use cases called: create=%d list=%d", parties.createCalls, parties.listCalls)
	}
}

func TestPartyListFailureIsNotRenderedAsEmpty(t *testing.T) {
	identities := administratorIdentityStub()
	parties := &partyStub{listErr: errors.New("database unavailable")}
	handler, err := New(readinessStub{}, identities, parties, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/parties", nil))
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "尚未登记主体") {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestAdministratorCanOpenObligationDraftList(t *testing.T) {
	identities := administratorIdentityStub()
	parties := &partyStub{listResult: obligationParties()}
	obligations := &obligationStub{listResult: []obligation.Obligation{sampleObligation()}}
	handler, err := New(readinessStub{}, identities, parties, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/obligations", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "人工应收／应付义务") ||
		!strings.Contains(response.Body.String(), "九月服务费") || !strings.Contains(response.Body.String(), "Owner Company") ||
		!strings.Contains(response.Body.String(), "Counter Person") || !strings.Contains(response.Body.String(), `name="command_id" value="`) {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	if parties.listCalls != 1 || obligations.listCalls != 1 {
		t.Fatalf("calls parties=%d obligations=%d", parties.listCalls, obligations.listCalls)
	}
	csrf := findCookie(t, response.Result().Cookies(), csrfCookieName)
	if !strings.Contains(response.Body.String(), csrf.Value) {
		t.Fatal("obligation form does not contain CSRF token")
	}
}

func TestObligationDraftPageRequiresTwoParties(t *testing.T) {
	identities := administratorIdentityStub()
	parties := &partyStub{listResult: obligationParties()[:1]}
	handler, err := New(readinessStub{}, identities, parties, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/obligations", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "至少需要在主体目录登记两个不同主体") ||
		strings.Contains(response.Body.String(), `action="/obligations"`) {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestAdministratorCanOpenCurrentAgingAndDrillIntoObligation(t *testing.T) {
	identities := administratorIdentityStub()
	value := sampleOpenObligation()
	value.DueOn = "2026-10-31"
	value.OutstandingAmount = "40.5"
	obligations := &obligationStub{agingResult: obligation.AgingReport{
		AsOfDate:    "2026-11-30",
		GeneratedAt: time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC),
		Summaries: []obligation.AgingSummary{{
			Direction: obligation.DirectionReceivable, Currency: "CNY", ObligationCount: 1, OverdueCount: 1,
			OutstandingAmount: "40.5", OverdueAmount: "40.5", Days1To30Amount: "40.5",
		}},
		Items: []obligation.AgingItem{{Obligation: value, Bucket: obligation.AgingBucketDays1To30, DaysPastDue: 30}},
	}}
	handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/aging?as_of_date=2026-11-30", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "当前 Obligation 逾期与账龄") ||
		!strings.Contains(body, "40.5 CNY") || !strings.Contains(body, "逾期 1–30 天") ||
		!strings.Contains(body, `/obligations/`+value.ID) || !strings.Contains(body, `value="2026-11-30"`) {
		t.Fatalf("response = %d %q", response.Code, body)
	}
	if obligations.agingCalls != 1 || obligations.agingInput.AsOfDate != "2026-11-30" {
		t.Fatalf("aging calls=%d input=%#v", obligations.agingCalls, obligations.agingInput)
	}
	csrf := findCookie(t, response.Result().Cookies(), csrfCookieName)
	if !strings.Contains(body, csrf.Value) {
		t.Fatal("aging page does not contain logout CSRF token")
	}
}

func TestCurrentAgingValidationAndReadFailuresAreVisible(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{name: "validation", err: &obligation.ValidationError{Field: "as_of_date", Message: "历史时点余额需要独立重放能力"}, wantStatus: http.StatusUnprocessableEntity, wantBody: "历史时点余额需要独立重放能力"},
		{name: "repository", err: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError, wantBody: "无法生成当前义务账龄"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			obligations := &obligationStub{agingErr: test.err}
			handler, err := New(readinessStub{}, administratorIdentityStub(), &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/aging?as_of_date=2026-01-01", nil))
			if response.Code != test.wantStatus || !strings.Contains(response.Body.String(), test.wantBody) {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestAdministratorCanOpenCurrentCashForecastAndHistory(t *testing.T) {
	value := sampleOpenObligation()
	value.OutstandingAmount = "40.5"
	expectation := obligation.CashExpectation{
		ID: "77777777-7777-4777-8777-777777777777", ObligationID: value.ID,
		Sequence: 2, Action: obligation.CashExpectationActionSet, ExpectedOn: "2026-10-20",
		ObligationRevision: value.CurrentRevision, SourceKind: obligation.SourceKindManual,
		Reason: "客户确认付款日期", RecordedBy: administratorIdentityStub().principal.Actor(),
		RecordedAt: time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC),
	}
	unscheduled := value
	unscheduled.ID = "66666666-6666-4666-8666-666666666666"
	unscheduled.Title = "未排期服务费"
	obligations := &obligationStub{
		cashForecastResult: obligation.CashForecastReport{
			GeneratedAt: time.Date(2026, 9, 20, 12, 30, 0, 0, time.UTC),
			ScheduledSummaries: []obligation.CashForecastSummary{{
				ExpectedOn: "2026-10-20", LedgerOwnerPartyID: value.LedgerOwnerPartyID,
				LedgerOwnerLegalName: "Owner Company", Direction: value.Direction,
				Currency: "CNY", ObligationCount: 1, OutstandingAmount: "40.5",
			}},
			UnscheduledSummaries: []obligation.CashForecastSummary{{
				LedgerOwnerPartyID: value.LedgerOwnerPartyID, LedgerOwnerLegalName: "Owner Company",
				Direction: value.Direction, Currency: "CNY", ObligationCount: 1, OutstandingAmount: "40.5",
			}},
			ScheduledItems:   []obligation.CashExpectationRow{{Obligation: value, Expectation: &expectation}},
			UnscheduledItems: []obligation.CashExpectationRow{{Obligation: unscheduled}},
		},
		cashHistoryResult: obligation.CashExpectationHistory{Obligation: value, Facts: []obligation.CashExpectation{expectation}},
	}
	handler, err := New(readinessStub{}, administratorIdentityStub(), &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/cash-forecast?history_obligation_id="+value.ID, nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "当前预计现金排期") ||
		!strings.Contains(body, "2026-10-20") || !strings.Contains(body, "40.5 CNY") ||
		!strings.Contains(body, "未排期服务费") || !strings.Contains(body, "客户确认付款日期") ||
		!strings.Contains(body, `action="/cash-expectations"`) || !strings.Contains(body, `name="action" value="clear"`) {
		t.Fatalf("response = %d %q", response.Code, body)
	}
	if obligations.cashForecastCalls != 1 || obligations.cashHistoryCalls != 1 || obligations.cashHistoryID != value.ID {
		t.Fatalf("forecast/history calls = %d/%d id=%q", obligations.cashForecastCalls, obligations.cashHistoryCalls, obligations.cashHistoryID)
	}
	csrf := findCookie(t, response.Result().Cookies(), csrfCookieName)
	if !strings.Contains(body, csrf.Value) {
		t.Fatal("cash forecast page does not contain CSRF token")
	}
}

func TestAdministratorRecordsCashExpectationWithPrincipalActor(t *testing.T) {
	value := sampleOpenObligation()
	expectationID := "77777777-7777-4777-8777-777777777777"
	obligations := &obligationStub{cashExpectationResult: obligation.RecordCashExpectationResult{
		Expectation: obligation.CashExpectation{ID: expectationID, ObligationID: value.ID, Action: obligation.CashExpectationActionSet},
	}}
	handler, err := New(readinessStub{}, administratorIdentityStub(), &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	token := csrfToken()
	request := authenticatedRequest(http.MethodPost, "/cash-expectations", url.Values{
		"csrf_token":           {token},
		"command_id":           {"66666666-6666-4666-8666-666666666666"},
		"action":               {"set"},
		"obligation_selection": {value.ID + ":1"},
		"expected_on":          {"2026-10-20"},
		"expectation_reason":   {" 客户确认付款日期 "},
		"actor_kind":           {"forged"},
		"actor_id":             {"forged"},
	})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || !strings.Contains(response.Header().Get("Location"), "/cash-forecast?") ||
		!strings.Contains(response.Header().Get("Location"), "recorded=1") || !strings.HasSuffix(response.Header().Get("Location"), "#history") {
		t.Fatalf("response = %d location=%q body=%q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	input := obligations.cashExpectationInput
	if obligations.cashExpectationCalls != 1 || input.ObligationID != value.ID || input.ExpectedCurrentRevision != 1 ||
		input.Action != "set" || input.ExpectedOn != "2026-10-20" || input.Reason != " 客户确认付款日期 " ||
		input.Actor != administratorIdentityStub().principal.Actor() {
		t.Fatalf("cash expectation calls=%d input=%#v", obligations.cashExpectationCalls, input)
	}
}

func TestCashExpectationErrorsAndAuthorizationAreExplicit(t *testing.T) {
	value := sampleOpenObligation()
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{name: "validation", err: &obligation.ValidationError{Field: "expected_on", Message: "预计现金日期无效"}, wantStatus: http.StatusUnprocessableEntity, wantBody: "预计现金日期无效"},
		{name: "idempotency", err: &obligation.CashExpectationIdempotencyConflictError{}, wantStatus: http.StatusConflict, wantBody: "不同的预计现金日期意图"},
		{name: "state", err: &obligation.CashExpectationStateConflictError{Status: obligation.StatusClosed}, wantStatus: http.StatusConflict, wantBody: "只有 open 状态"},
		{name: "revision", err: &obligation.CurrentRevisionConflictError{Expected: 1, Actual: 2}, wantStatus: http.StatusConflict, wantBody: "current revision 已变化"},
		{name: "balance", err: &obligation.CashExpectationBalanceConflictError{}, wantStatus: http.StatusConflict, wantBody: "当前剩余金额大于 0"},
		{name: "clear", err: &obligation.CashExpectationClearConflictError{}, wantStatus: http.StatusConflict, wantBody: "没有可清除"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			obligations := &obligationStub{cashExpectationErr: test.err, cashHistoryResult: obligation.CashExpectationHistory{Obligation: value}}
			handler, err := New(readinessStub{}, administratorIdentityStub(), &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatal(err)
			}
			token := csrfToken()
			request := authenticatedRequest(http.MethodPost, "/cash-expectations", url.Values{
				"csrf_token": {token}, "command_id": {"66666666-6666-4666-8666-666666666666"},
				"action": {"set"}, "obligation_selection": {value.ID + ":1"},
				"expected_on": {"2026-10-20"}, "expectation_reason": {"测试依据"},
			})
			request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus || !strings.Contains(response.Body.String(), test.wantBody) {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}

	obligations := &obligationStub{}
	handler, err := New(readinessStub{}, administratorIdentityStub(), &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodPost, "/cash-expectations", url.Values{}))
	if response.Code != http.StatusForbidden || obligations.cashExpectationCalls != 0 {
		t.Fatalf("missing CSRF response=%d calls=%d", response.Code, obligations.cashExpectationCalls)
	}

	viewer := administratorIdentityStub()
	viewer.principal.Role = identity.Role("viewer")
	handler, err = New(readinessStub{}, viewer, &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/cash-forecast", nil))
	if response.Code != http.StatusForbidden || obligations.cashForecastCalls != 0 {
		t.Fatalf("viewer response=%d forecast calls=%d", response.Code, obligations.cashForecastCalls)
	}
}

func TestCashForecastReadFailuresAreNotRenderedAsEmpty(t *testing.T) {
	obligations := &obligationStub{cashForecastErr: errors.New("database unavailable")}
	handler, err := New(readinessStub{}, administratorIdentityStub(), &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/cash-forecast", nil))
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "无法生成当前预计现金排期") {
		t.Fatalf("response=%d body=%q", response.Code, response.Body.String())
	}
}

func TestAdministratorCanOpenCurrentOperatingSummaryAndMetricLinks(t *testing.T) {
	ownerID := "55555555-5555-4555-8555-555555555555"
	summaries := &operatingSummaryStub{currentResult: operatingsummary.Report{
		GeneratedAt: time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC),
		Groups: []operatingsummary.Group{{
			LedgerOwnerPartyID: ownerID, LedgerOwnerLegalName: "测试本方主体", Currency: "CNY",
			ReceivableOutstanding: operatingsummary.AmountMetric{Metric: operatingsummary.MetricReceivableOutstanding, Amount: "40.5", FactCount: 2},
			PayableOutstanding:    operatingsummary.AmountMetric{Metric: operatingsummary.MetricPayableOutstanding, Amount: "0", FactCount: 0},
			ReceiptEffective:      operatingsummary.AmountMetric{Metric: operatingsummary.MetricReceiptEffective, Amount: "100.5", FactCount: 2},
			PaymentEffective:      operatingsummary.AmountMetric{Metric: operatingsummary.MetricPaymentEffective, Amount: "20", FactCount: 1},
			ReceiptUnallocated:    operatingsummary.AmountMetric{Metric: operatingsummary.MetricReceiptUnallocated, Amount: "40.25", FactCount: 1},
			PaymentUnallocated:    operatingsummary.AmountMetric{Metric: operatingsummary.MetricPaymentUnallocated, Amount: "20", FactCount: 1},
		}},
	}}
	handler, err := New(readinessStub{}, administratorIdentityStub(), &partyStub{}, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, summaries, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/operating-summary", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "当前经营收支汇总") ||
		!strings.Contains(body, "测试本方主体") || !strings.Contains(body, "40.5 CNY（2 条）") ||
		!strings.Contains(body, "100.5 CNY（2 条）") || !strings.Contains(body, "0 CNY") ||
		!strings.Contains(body, "metric=receivable_outstanding") ||
		!strings.Contains(body, "ledger_owner_party_id="+ownerID) || !strings.Contains(body, "currency=CNY") ||
		strings.Contains(body, "metric=payable_outstanding") || summaries.currentCalls != 1 {
		t.Fatalf("response = %d %q calls=%d", response.Code, body, summaries.currentCalls)
	}
	csrf := findCookie(t, response.Result().Cookies(), csrfCookieName)
	if !strings.Contains(body, csrf.Value) {
		t.Fatal("operating summary does not contain logout CSRF token")
	}
}

func TestAdministratorCanDrillIntoOperatingSummaryFacts(t *testing.T) {
	ownerID := "55555555-5555-4555-8555-555555555555"
	obligationID := "66666666-6666-4666-8666-666666666666"
	cashID := "77777777-7777-4777-8777-777777777777"
	tests := []struct {
		name       string
		path       string
		details    operatingsummary.Drilldown
		wantBodies []string
	}{
		{
			name: "obligation",
			path: "/operating-summary/details?metric=receivable_outstanding&ledger_owner_party_id=" + ownerID + "&currency=CNY",
			details: operatingsummary.Drilldown{
				Metric: operatingsummary.MetricReceivableOutstanding, LedgerOwnerPartyID: ownerID,
				LedgerOwnerLegalName: "测试本方主体", Currency: "CNY", TotalAmount: "40.5", FactCount: 1,
				GeneratedAt: time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC),
				ObligationFacts: []operatingsummary.ObligationFact{{
					ID: obligationID, Title: "服务费应收", DirectionLabel: "应收", CounterpartyLegalName: "往来方",
					Currency: "CNY", DueOn: "2026-12-31", CurrentRevision: 2,
					EffectiveAmount: "100.5", NetAllocatedAmount: "50", RemittedAmount: "10", ContributionAmount: "40.5",
				}},
			},
			wantBodies: []string{"当前应收余额下钻", "40.5 CNY", "服务费应收", "/obligations/" + obligationID, "100.5 CNY", "50 CNY", "10 CNY"},
		},
		{
			name: "cash",
			path: "/operating-summary/details?metric=receipt_unallocated&ledger_owner_party_id=" + ownerID + "&currency=CNY",
			details: operatingsummary.Drilldown{
				Metric: operatingsummary.MetricReceiptUnallocated, LedgerOwnerPartyID: ownerID,
				LedgerOwnerLegalName: "测试本方主体", Currency: "CNY", TotalAmount: "40.25", FactCount: 1,
				GeneratedAt: time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC),
				CashFacts: []operatingsummary.CashFact{{
					ID: cashID, Description: "客户收款", TypeLabel: "收款", CounterpartyLegalName: "往来方",
					Currency: "CNY", OccurredOn: "2026-09-20", AccountReference: "基本户",
					OriginalAmount: "100.5", EffectiveAmount: "100.5", NetAllocatedAmount: "60.25",
					UnallocatedAmount: "40.25", ContributionAmount: "40.25",
				}},
			},
			wantBodies: []string{"未分配收款下钻", "40.25 CNY", "客户收款", "/cash-transactions/" + cashID, "100.5 CNY", "60.25 CNY", "基本户"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			summaries := &operatingSummaryStub{drilldownResult: test.details}
			handler, err := New(readinessStub{}, administratorIdentityStub(), &partyStub{}, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, summaries, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, test.path, nil))
			body := response.Body.String()
			if response.Code != http.StatusOK || summaries.drilldownCalls != 1 {
				t.Fatalf("response = %d %q calls=%d", response.Code, body, summaries.drilldownCalls)
			}
			for _, want := range test.wantBodies {
				if !strings.Contains(body, want) {
					t.Fatalf("response missing %q: %q", want, body)
				}
			}
			if summaries.drilldownInput.Metric != string(test.details.Metric) ||
				summaries.drilldownInput.LedgerOwnerPartyID != ownerID || summaries.drilldownInput.Currency != "CNY" {
				t.Fatalf("drilldown input = %#v", summaries.drilldownInput)
			}
		})
	}
}

func TestOperatingSummaryErrorsAndAuthorizationAreExplicit(t *testing.T) {
	ownerID := "55555555-5555-4555-8555-555555555555"
	for _, test := range []struct {
		name       string
		path       string
		currentErr error
		detailErr  error
		status     int
		body       string
	}{
		{name: "summary read", path: "/operating-summary", currentErr: errors.New("database unavailable"), status: http.StatusInternalServerError, body: "无法生成当前经营收支汇总"},
		{name: "invalid detail", path: "/operating-summary/details?metric=bad", detailErr: &operatingsummary.ValidationError{Field: "metric", Message: "汇总指标无效"}, status: http.StatusUnprocessableEntity, body: "汇总指标无效"},
		{name: "missing detail", path: "/operating-summary/details?metric=receipt_effective&ledger_owner_party_id=" + ownerID + "&currency=CNY", detailErr: &operatingsummary.DrilldownNotFoundError{}, status: http.StatusNotFound, body: "404 page not found"},
		{name: "detail read", path: "/operating-summary/details?metric=receipt_effective&ledger_owner_party_id=" + ownerID + "&currency=CNY", detailErr: errors.New("database unavailable"), status: http.StatusInternalServerError, body: "无法读取当前经营收支明细"},
	} {
		t.Run(test.name, func(t *testing.T) {
			summaries := &operatingSummaryStub{currentErr: test.currentErr, drilldownErr: test.detailErr}
			handler, err := New(readinessStub{}, administratorIdentityStub(), &partyStub{}, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, summaries, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, test.path, nil))
			if response.Code != test.status || !strings.Contains(response.Body.String(), test.body) {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}

	summaries := &operatingSummaryStub{}
	handler, err := New(readinessStub{}, administratorIdentityStub(), &partyStub{}, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, summaries, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/operating-summary", nil))
	if response.Code != http.StatusSeeOther || summaries.currentCalls != 0 {
		t.Fatalf("anonymous response = %d calls=%d", response.Code, summaries.currentCalls)
	}

	viewer := administratorIdentityStub()
	viewer.principal.Role = identity.Role("viewer")
	handler, err = New(readinessStub{}, viewer, &partyStub{}, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, summaries, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/operating-summary", "/operating-summary/details?metric=receipt_effective"} {
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusForbidden {
			t.Fatalf("viewer %s status = %d", path, response.Code)
		}
	}
	if summaries.currentCalls != 0 || summaries.drilldownCalls != 0 {
		t.Fatalf("unauthorized calls current=%d drilldown=%d", summaries.currentCalls, summaries.drilldownCalls)
	}
}

func TestAdministratorCanOpenOwnerLedgerExportPage(t *testing.T) {
	parties := &partyStub{listResult: obligationParties()}
	exports := &ledgerExportStub{}
	handler, err := New(
		readinessStub{}, administratorIdentityStub(), parties, &obligationStub{}, &cashTransactionStub{},
		&allocationStub{}, &operatingSummaryStub{}, exports, Options{}, &internalTransferStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/ledger-exports", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "本方主体可校验台账导出") ||
		!strings.Contains(body, "Owner Company") || !strings.Contains(body, "Counter Person") ||
		!strings.Contains(body, "按文本导入") || !strings.Contains(body, `method="post" action="/ledger-exports"`) ||
		parties.listCalls != 1 || exports.calls != 0 {
		t.Fatalf("response=%d body=%q partyCalls=%d exportCalls=%d", response.Code, body, parties.listCalls, exports.calls)
	}
	csrf := findCookie(t, response.Result().Cookies(), csrfCookieName)
	if !strings.Contains(body, csrf.Value) {
		t.Fatal("ledger export page does not contain CSRF token")
	}
}

func TestAdministratorDownloadsOwnerLedgerExportWithPrincipalActor(t *testing.T) {
	ownerID := "22222222-2222-4222-8222-222222222222"
	identities := administratorIdentityStub()
	exports := &ledgerExportStub{artifact: ledgerexport.Artifact{
		Filename: "money-flow-ledger-" + ownerID + ".zip",
		Content:  []byte("test-zip-content"),
	}}
	handler, err := New(
		readinessStub{}, identities, &partyStub{}, &obligationStub{}, &cashTransactionStub{},
		&allocationStub{}, &operatingSummaryStub{}, exports, Options{}, &internalTransferStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	token := csrfToken()
	request := authenticatedRequest(http.MethodPost, "/ledger-exports", url.Values{
		"csrf_token":            {token},
		"ledger_owner_party_id": {ownerID},
		"actor_kind":            {"forged"},
		"actor_id":              {"forged"},
		"actor_display_name":    {"伪造用户"},
	})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "test-zip-content" ||
		response.Header().Get("Content-Type") != "application/zip" ||
		!strings.Contains(response.Header().Get("Content-Disposition"), "money-flow-ledger-"+ownerID+".zip") ||
		response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("response=%d headers=%#v body=%q", response.Code, response.Header(), response.Body.String())
	}
	wantActor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: identities.principal.UserID, DisplayName: identities.principal.DisplayName,
	}
	if exports.calls != 1 || exports.input.LedgerOwnerPartyID != ownerID || exports.input.Actor != wantActor {
		t.Fatalf("export calls=%d input=%#v", exports.calls, exports.input)
	}
}

func TestOwnerLedgerExportErrorsCSRFAndAuthorizationAreExplicit(t *testing.T) {
	ownerID := "22222222-2222-4222-8222-222222222222"
	for _, test := range []struct {
		name       string
		exportErr  error
		status     int
		body       string
		partyCalls int
	}{
		{name: "validation", exportErr: &ledgerexport.ValidationError{Field: "ledger_owner_party_id", Message: "本方账簿主体标识无效"}, status: http.StatusUnprocessableEntity, body: "本方账簿主体标识无效", partyCalls: 1},
		{name: "not found", exportErr: &ledgerexport.NotFoundError{LedgerOwnerPartyID: ownerID}, status: http.StatusNotFound, body: "404 page not found"},
		{name: "read or audit failure", exportErr: errors.New("database unavailable"), status: http.StatusInternalServerError, body: "无法生成本方主体台账导出"},
	} {
		t.Run(test.name, func(t *testing.T) {
			parties := &partyStub{listResult: obligationParties()}
			exports := &ledgerExportStub{err: test.exportErr}
			handler, err := New(
				readinessStub{}, administratorIdentityStub(), parties, &obligationStub{}, &cashTransactionStub{},
				&allocationStub{}, &operatingSummaryStub{}, exports, Options{}, &internalTransferStub{},
			)
			if err != nil {
				t.Fatal(err)
			}
			token := csrfToken()
			request := authenticatedRequest(http.MethodPost, "/ledger-exports", url.Values{
				"csrf_token": {token}, "ledger_owner_party_id": {ownerID},
			})
			request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status || !strings.Contains(response.Body.String(), test.body) ||
				exports.calls != 1 || parties.listCalls != test.partyCalls {
				t.Fatalf("response=%d body=%q exportCalls=%d partyCalls=%d", response.Code, response.Body.String(), exports.calls, parties.listCalls)
			}
		})
	}

	exports := &ledgerExportStub{}
	handler, err := New(
		readinessStub{}, administratorIdentityStub(), &partyStub{}, &obligationStub{}, &cashTransactionStub{},
		&allocationStub{}, &operatingSummaryStub{}, exports, Options{}, &internalTransferStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodPost, "/ledger-exports", url.Values{
		"ledger_owner_party_id": {ownerID},
	}))
	if response.Code != http.StatusForbidden || exports.calls != 0 {
		t.Fatalf("missing CSRF response=%d calls=%d", response.Code, exports.calls)
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, formRequest(http.MethodPost, "/ledger-exports", url.Values{}))
	if response.Code != http.StatusUnauthorized || exports.calls != 0 {
		t.Fatalf("anonymous response=%d calls=%d", response.Code, exports.calls)
	}

	viewer := administratorIdentityStub()
	viewer.principal.Role = identity.Role("viewer")
	handler, err = New(
		readinessStub{}, viewer, &partyStub{}, &obligationStub{}, &cashTransactionStub{},
		&allocationStub{}, &operatingSummaryStub{}, exports, Options{}, &internalTransferStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []*http.Request{
		authenticatedRequest(http.MethodGet, "/ledger-exports", nil),
		authenticatedRequest(http.MethodPost, "/ledger-exports", url.Values{}),
	} {
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden || exports.calls != 0 {
			t.Fatalf("viewer response=%d calls=%d", response.Code, exports.calls)
		}
	}
}

func TestOwnerLedgerExportPageFailsWhenPartiesCannotBeRead(t *testing.T) {
	parties := &partyStub{listErr: errors.New("database unavailable")}
	handler, err := New(
		readinessStub{}, administratorIdentityStub(), parties, &obligationStub{}, &cashTransactionStub{},
		&allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/ledger-exports", nil))
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "无法读取主体列表") {
		t.Fatalf("response=%d body=%q", response.Code, response.Body.String())
	}
}

func TestAdministratorCanReplayHistoricalObligationBalance(t *testing.T) {
	value := sampleOpenObligation()
	asOf := time.Date(2026, 9, 20, 12, 34, 56, 123456000, time.UTC)
	obligations := &obligationStub{historicalResult: obligation.HistoricalSnapshot{
		AsOf:               asOf,
		ObligationID:       value.ID,
		Revision:           2,
		Direction:          value.Direction,
		Title:              value.Title,
		LedgerOwnerPartyID: value.LedgerOwnerPartyID,
		Creditor:           value.Creditor,
		Debtor:             value.Debtor,
		EffectiveAmount:    "120.5",
		NetAllocatedAmount: "60.5",
		RemittedAmount:     "60",
		OutstandingAmount:  "0",
		BalanceSequence:    3,
		Currency:           "CNY",
		DueOn:              value.DueOn,
		RevisionRecordedAt: time.Date(2026, 9, 20, 11, 0, 0, 0, time.UTC),
		Status:             obligation.StatusClosed,
		Closure: &obligation.HistoricalClosure{
			ID: "99999999-9999-4999-8999-999999999999", SettlementBasis: obligation.ClosureBasisMixed,
			ClosedAt: time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC),
		},
	}}
	handler, err := New(readinessStub{}, administratorIdentityStub(), &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	requestedAsOf := "2026-09-20T20:34:56.123456+08:00"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet,
		"/obligations/"+value.ID+"/history?as_of="+url.QueryEscape(requestedAsOf), nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "历史 Obligation 余额") ||
		!strings.Contains(body, `value="2026-09-20T12:34:56.123456Z"`) ||
		!strings.Contains(body, "120.5 − 60.5 − 60 = 0 CNY") ||
		!strings.Contains(body, "资金分配与减免组合归零") ||
		!strings.Contains(body, `/obligations/`+value.ID) {
		t.Fatalf("response = %d %q", response.Code, body)
	}
	if obligations.historicalCalls != 1 || obligations.historicalInput.ObligationID != value.ID ||
		obligations.historicalInput.AsOf != requestedAsOf {
		t.Fatalf("historical calls=%d input=%#v", obligations.historicalCalls, obligations.historicalInput)
	}
	csrf := findCookie(t, response.Result().Cookies(), csrfCookieName)
	if !strings.Contains(body, csrf.Value) {
		t.Fatal("historical balance page does not contain logout CSRF token")
	}
}

func TestHistoricalObligationBalanceFailuresAreExplicit(t *testing.T) {
	value := sampleOpenObligation()
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{name: "invalid time", err: &obligation.ValidationError{Field: "as_of", Message: "历史时刻必须是带时区的 RFC3339 时间"}, wantStatus: http.StatusUnprocessableEntity, wantBody: "历史时刻必须是带时区的 RFC3339 时间"},
		{name: "invalid obligation ID", err: &obligation.ValidationError{Field: "obligation_id", Message: "义务标识无效"}, wantStatus: http.StatusNotFound, wantBody: "404"},
		{name: "missing", err: &obligation.ObligationNotFoundError{ObligationID: value.ID}, wantStatus: http.StatusNotFound, wantBody: "404"},
		{name: "before confirmation", err: &obligation.HistoricalNotConfirmedError{ObligationID: value.ID}, wantStatus: http.StatusUnprocessableEntity, wantBody: "该时刻义务尚未确认"},
		{name: "invariant", err: &obligation.HistoricalInvariantError{ObligationID: value.ID, Reason: "negative balance"}, wantStatus: http.StatusInternalServerError, wantBody: "无法重放历史义务余额"},
		{name: "repository", err: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError, wantBody: "无法重放历史义务余额"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			obligations := &obligationStub{historicalErr: test.err}
			handler, err := New(readinessStub{}, administratorIdentityStub(), &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, authenticatedRequest(http.MethodGet,
				"/obligations/"+value.ID+"/history?as_of="+url.QueryEscape("bad time"), nil))
			if response.Code != test.wantStatus || !strings.Contains(response.Body.String(), test.wantBody) {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestAdministratorCreatesObligationDraftWithPrincipalActor(t *testing.T) {
	identities := administratorIdentityStub()
	parties := &partyStub{listResult: obligationParties()}
	obligations := &obligationStub{createResult: obligation.CreateResult{Obligation: obligation.Obligation{ID: "55555555-5555-4555-8555-555555555555"}}}
	handler, err := New(readinessStub{}, identities, parties, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	token := csrfToken()
	request := authenticatedRequest(http.MethodPost, "/obligations", url.Values{
		"csrf_token":              {token},
		"command_id":              {"66666666-6666-4666-8666-666666666666"},
		"direction":               {"receivable"},
		"ledger_owner_party_id":   {"22222222-2222-4222-8222-222222222222"},
		"counterparty_party_id":   {"33333333-3333-4333-8333-333333333333"},
		"title":                   {"  九月服务费  "},
		"original_amount":         {" 100.50 "},
		"currency":                {" cny "},
		"due_on":                  {"2026-10-31"},
		"manual_source_reference": {" 人工确认单 OBL-001 "},
		"actor_kind":              {"forged"},
		"actor_id":                {"forged"},
		"actor_display_name":      {"伪造用户"},
	})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/obligations/55555555-5555-4555-8555-555555555555?created=1" {
		t.Fatalf("status=%d location=%q body=%q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	input := obligations.createInput
	if obligations.createCalls != 1 || input.CommandID != "66666666-6666-4666-8666-666666666666" ||
		input.Title != "  九月服务费  " || input.OriginalAmount != " 100.50 " || input.Currency != " cny " ||
		input.ManualSourceReference != " 人工确认单 OBL-001 " {
		t.Fatalf("create input = %#v", input)
	}
	wantActor := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: identities.principal.UserID, DisplayName: identities.principal.DisplayName}
	if input.Actor != wantActor {
		t.Fatalf("actor = %#v, want %#v", input.Actor, wantActor)
	}
}

func TestObligationDraftCreateRequiresCSRF(t *testing.T) {
	identities := administratorIdentityStub()
	obligations := &obligationStub{}
	handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodPost, "/obligations", url.Values{"title": {"无 CSRF"}}))
	if response.Code != http.StatusForbidden || obligations.createCalls != 0 {
		t.Fatalf("status=%d createCalls=%d", response.Code, obligations.createCalls)
	}
}

func TestObligationDraftCreateErrorsPreserveIntent(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "validation", err: &obligation.ValidationError{Field: "title", Message: "义务标题不能为空"}, status: http.StatusUnprocessableEntity},
		{name: "party missing", err: &obligation.PartyNotFoundError{PartyID: "33333333-3333-4333-8333-333333333333"}, status: http.StatusUnprocessableEntity},
		{name: "conflict", err: &obligation.IdempotencyConflictError{CommandID: "66666666-6666-4666-8666-666666666666"}, status: http.StatusConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			identities := administratorIdentityStub()
			parties := &partyStub{listResult: obligationParties()}
			obligations := &obligationStub{createErr: test.err}
			handler, err := New(readinessStub{}, identities, parties, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			token := csrfToken()
			request := authenticatedRequest(http.MethodPost, "/obligations", url.Values{
				"csrf_token": {token}, "command_id": {"66666666-6666-4666-8666-666666666666"},
				"direction": {"payable"}, "ledger_owner_party_id": {"22222222-2222-4222-8222-222222222222"},
				"counterparty_party_id": {"33333333-3333-4333-8333-333333333333"},
				"title":                 {"保留的义务标题"}, "original_amount": {"88.5"}, "currency": {"USD"},
				"due_on": {"2026-11-30"}, "manual_source_reference": {"保留的人工依据"},
			})
			request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status || !strings.Contains(response.Body.String(), test.err.Error()) ||
				!strings.Contains(response.Body.String(), `name="command_id" value="66666666-6666-4666-8666-666666666666"`) ||
				!strings.Contains(response.Body.String(), `value="保留的义务标题"`) ||
				!strings.Contains(response.Body.String(), `value="保留的人工依据"`) {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestAdministratorCanReadObligationDraftDetail(t *testing.T) {
	identities := administratorIdentityStub()
	value := sampleObligation()
	obligations := &obligationStub{getResult: value, getFound: true}
	allocations := &allocationStub{}
	handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, &cashTransactionStub{}, allocations, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/obligations/"+value.ID+"?created=1", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "义务草稿已建立") ||
		!strings.Contains(response.Body.String(), "债权方 creditor") || !strings.Contains(response.Body.String(), "Owner Company") ||
		!strings.Contains(response.Body.String(), "Counter Person") || !strings.Contains(response.Body.String(), "100.5 CNY") ||
		!strings.Contains(response.Body.String(), `action="/obligations/55555555-5555-4555-8555-555555555555/confirm"`) ||
		!strings.Contains(response.Body.String(), "该义务尚未形成任何资金分配") ||
		obligations.getID != value.ID || allocations.listForObligationID != value.ID || allocations.listForObligationCalls != 1 {
		t.Fatalf("response = %d %q getID=%q", response.Code, response.Body.String(), obligations.getID)
	}
}

func TestAdministratorConfirmsObligationWithPrincipalActor(t *testing.T) {
	identities := administratorIdentityStub()
	value := sampleObligation()
	obligations := &obligationStub{confirmResult: obligation.ConfirmResult{
		Obligation: obligation.Obligation{ID: value.ID, Status: obligation.StatusOpen, CurrentRevision: 1},
		Revision:   obligation.Revision{ObligationID: value.ID, Revision: 1},
	}}
	handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	token := csrfToken()
	request := authenticatedRequest(http.MethodPost, "/obligations/"+value.ID+"/confirm", url.Values{
		"csrf_token":             {token},
		"command_id":             {"66666666-6666-4666-8666-666666666666"},
		"expected_draft_version": {"1"},
		"actor_kind":             {"forged"},
		"actor_id":               {"forged"},
		"actor_display_name":     {"伪造用户"},
	})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/obligations/"+value.ID+"?confirmed=1" {
		t.Fatalf("status=%d location=%q body=%q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	wantActor := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: identities.principal.UserID, DisplayName: identities.principal.DisplayName}
	input := obligations.confirmInput
	if obligations.confirmCalls != 1 || input.CommandID != "66666666-6666-4666-8666-666666666666" ||
		input.ObligationID != value.ID || input.ExpectedDraftVersion != 1 || input.Actor != wantActor {
		t.Fatalf("confirm input = %#v", input)
	}
}

func TestObligationConfirmationRequiresCSRF(t *testing.T) {
	identities := administratorIdentityStub()
	obligations := &obligationStub{}
	handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodPost,
		"/obligations/55555555-5555-4555-8555-555555555555/confirm", url.Values{"command_id": {"bad"}}))
	if response.Code != http.StatusForbidden || obligations.confirmCalls != 0 {
		t.Fatalf("status=%d confirmCalls=%d", response.Code, obligations.confirmCalls)
	}
}

func TestObligationConfirmationErrorsAreExplicit(t *testing.T) {
	value := sampleObligation()
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "validation", err: &obligation.ValidationError{Field: "expected_draft_version", Message: "Expected Draft Version 无效"}, status: http.StatusUnprocessableEntity},
		{name: "idempotency", err: &obligation.ConfirmationIdempotencyConflictError{CommandID: "66666666-6666-4666-8666-666666666666"}, status: http.StatusConflict},
		{name: "state", err: &obligation.ObligationStateConflictError{Status: obligation.StatusOpen}, status: http.StatusConflict},
		{name: "version", err: &obligation.DraftVersionConflictError{Expected: 1, Actual: 2}, status: http.StatusConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			identities := administratorIdentityStub()
			obligations := &obligationStub{confirmErr: test.err, getResult: value, getFound: true}
			handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			token := csrfToken()
			request := authenticatedRequest(http.MethodPost, "/obligations/"+value.ID+"/confirm", url.Values{
				"csrf_token": {token}, "command_id": {"66666666-6666-4666-8666-666666666666"}, "expected_draft_version": {"1"},
			})
			request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status || !strings.Contains(response.Body.String(), test.err.Error()) || obligations.getCalls != 1 {
				t.Fatalf("response = %d %q getCalls=%d", response.Code, response.Body.String(), obligations.getCalls)
			}
		})
	}
}

func TestOpenObligationDetailShowsRevisionWithoutConfirmationForm(t *testing.T) {
	identities := administratorIdentityStub()
	value := sampleObligation()
	value.Status = obligation.StatusOpen
	value.CurrentRevision = 1
	revision := obligation.Revision{
		ObligationID: value.ID, Revision: 1, SourceDraftVersion: 1,
		EffectiveAmount: value.EffectiveAmount, Currency: value.Currency, RevisionKind: obligation.RevisionKindConfirmation,
		RecordedBy: audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: identities.principal.UserID, DisplayName: identities.principal.DisplayName},
		RecordedAt: time.Date(2026, 9, 19, 15, 0, 0, 0, time.UTC),
	}
	obligations := &obligationStub{detailsResult: obligation.Details{Obligation: value, Revisions: []obligation.Revision{revision}}, getFound: true}
	allocations := &allocationStub{listForObligationResult: []allocation.Allocation{sampleAllocation()}}
	handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, &cashTransactionStub{}, allocations, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/obligations/"+value.ID+"?confirmed=1", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "义务已确认并形成 revision 1") ||
		!strings.Contains(body, "义务 revision 历史") || !strings.Contains(body, identities.principal.DisplayName) ||
		!strings.Contains(body, "资金分配历史") || !strings.Contains(body, "九月服务费收款") ||
		!strings.Contains(body, "人工核对资金和义务") || allocations.listForObligationID != value.ID ||
		strings.Contains(body, `action="/obligations/`+value.ID+`/confirm"`) {
		t.Fatalf("response = %d %q", response.Code, body)
	}
}

func TestAdministratorAdjustsObligationAmountWithPrincipalActor(t *testing.T) {
	identities := administratorIdentityStub()
	value := sampleOpenObligation()
	obligations := &obligationStub{adjustResult: obligation.AdjustAmountResult{
		Obligation: obligation.Obligation{ID: value.ID, Status: obligation.StatusOpen, CurrentRevision: 2, EffectiveAmount: "120.5"},
		Adjustment: obligation.Adjustment{ID: "66666666-6666-4666-8666-666666666666", ObligationID: value.ID, ResultingRevision: 2},
		Revision:   obligation.Revision{ObligationID: value.ID, Revision: 2, EffectiveAmount: "120.5"},
	}}
	handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	token := csrfToken()
	request := authenticatedRequest(http.MethodPost, "/obligations/"+value.ID+"/adjust-amount", url.Values{
		"csrf_token":                 {token},
		"command_id":                 {"77777777-7777-4777-8777-777777777777"},
		"expected_current_revision":  {"1"},
		"adjustment_type":            {"increase"},
		"adjustment_amount":          {"20.000000"},
		"adjustment_reason":          {"追加服务范围"},
		"previous_effective_amount":  {"999"},
		"resulting_effective_amount": {"1"},
		"currency":                   {"USD"},
		"actor_kind":                 {"forged"},
		"actor_id":                   {"forged"},
		"actor_display_name":         {"伪造用户"},
		"adjusted_at":                {"2000-01-01T00:00:00Z"},
	})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/obligations/"+value.ID+"?adjusted=1" {
		t.Fatalf("status=%d location=%q body=%q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	wantActor := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: identities.principal.UserID, DisplayName: identities.principal.DisplayName}
	input := obligations.adjustInput
	if obligations.adjustCalls != 1 || input.CommandID != "77777777-7777-4777-8777-777777777777" ||
		input.ObligationID != value.ID || input.ExpectedCurrentRevision != 1 || input.Type != "increase" ||
		input.Amount != "20.000000" || input.Reason != "追加服务范围" || input.Actor != wantActor {
		t.Fatalf("adjust input = %#v", input)
	}
}

func TestObligationAmountAdjustmentRequiresCSRF(t *testing.T) {
	identities := administratorIdentityStub()
	obligations := &obligationStub{}
	handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodPost,
		"/obligations/55555555-5555-4555-8555-555555555555/adjust-amount", url.Values{"command_id": {"bad"}}))
	if response.Code != http.StatusForbidden || obligations.adjustCalls != 0 {
		t.Fatalf("status=%d adjustCalls=%d", response.Code, obligations.adjustCalls)
	}
}

func TestObligationAmountAdjustmentErrorsAreExplicit(t *testing.T) {
	value := sampleOpenObligation()
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "validation", err: &obligation.ValidationError{Field: "adjustment_amount", Message: "调整金额无效"}, status: http.StatusUnprocessableEntity},
		{name: "idempotency", err: &obligation.AmountAdjustmentIdempotencyConflictError{CommandID: "77777777-7777-4777-8777-777777777777"}, status: http.StatusConflict},
		{name: "state", err: &obligation.AmountAdjustmentStateConflictError{Status: obligation.StatusDraft}, status: http.StatusConflict},
		{name: "revision", err: &obligation.CurrentRevisionConflictError{Expected: 1, Actual: 2}, status: http.StatusConflict},
		{name: "balance", err: &obligation.AmountAdjustmentResultConflictError{Reason: "below_allocated"}, status: http.StatusConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			identities := administratorIdentityStub()
			obligations := &obligationStub{adjustErr: test.err, getResult: value, getFound: true}
			handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			token := csrfToken()
			request := authenticatedRequest(http.MethodPost, "/obligations/"+value.ID+"/adjust-amount", url.Values{
				"csrf_token": {token}, "command_id": {"77777777-7777-4777-8777-777777777777"},
				"expected_current_revision": {"1"}, "adjustment_type": {"decrease"},
				"adjustment_amount": {"20"}, "adjustment_reason": {"调整原因"},
			})
			request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status || !strings.Contains(response.Body.String(), test.err.Error()) ||
				!strings.Contains(response.Body.String(), `value="20"`) ||
				!strings.Contains(response.Body.String(), `value="调整原因"`) || obligations.getCalls != 1 {
				t.Fatalf("response = %d %q getCalls=%d", response.Code, response.Body.String(), obligations.getCalls)
			}
		})
	}
}

func TestObligationAmountAdjustmentNotFoundAndFailure(t *testing.T) {
	identities := administratorIdentityStub()
	for _, test := range []struct {
		name   string
		err    error
		status int
	}{
		{name: "not found", err: &obligation.ObligationNotFoundError{ObligationID: "55555555-5555-4555-8555-555555555555"}, status: http.StatusNotFound},
		{name: "database failure", err: errors.New("database unavailable"), status: http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			obligations := &obligationStub{adjustErr: test.err}
			handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			token := csrfToken()
			request := authenticatedRequest(http.MethodPost, "/obligations/55555555-5555-4555-8555-555555555555/adjust-amount", url.Values{
				"csrf_token": {token}, "command_id": {"77777777-7777-4777-8777-777777777777"},
				"expected_current_revision": {"1"}, "adjustment_type": {"increase"},
				"adjustment_amount": {"20"}, "adjustment_reason": {"调整原因"},
			})
			request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status || obligations.getCalls != 0 {
				t.Fatalf("response = %d %q getCalls=%d", response.Code, response.Body.String(), obligations.getCalls)
			}
		})
	}
}

func TestAdjustedObligationDetailShowsCurrentAmountAndHistory(t *testing.T) {
	identities := administratorIdentityStub()
	value := sampleOpenObligation()
	value.CurrentRevision = 2
	value.EffectiveAmount = "120.5"
	value.OutstandingAmount = "80.5"
	value.AllocatedAmount = "40"
	recordedBy := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: identities.principal.UserID, DisplayName: identities.principal.DisplayName}
	details := obligation.Details{
		Obligation: value,
		Revisions: []obligation.Revision{
			{ObligationID: value.ID, Revision: 1, SourceDraftVersion: 1, RevisionKind: obligation.RevisionKindConfirmation, EffectiveAmount: "100.5", Currency: "CNY", RecordedBy: recordedBy},
			{ObligationID: value.ID, Revision: 2, SourceRevision: 1, RevisionKind: obligation.RevisionKindAmountAdjustment, EffectiveAmount: "120.5", Currency: "CNY", RecordedBy: recordedBy},
		},
		Adjustments: []obligation.Adjustment{{
			ID: "66666666-6666-4666-8666-666666666666", ObligationID: value.ID,
			SourceRevision: 1, ResultingRevision: 2, Type: obligation.AdjustmentTypeIncrease,
			Amount: "20", PreviousEffectiveAmount: "100.5", ResultingEffectiveAmount: "120.5",
			Currency: "CNY", Reason: "追加服务范围", AdjustedBy: recordedBy,
		}},
	}
	obligations := &obligationStub{detailsResult: details, getFound: true}
	handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/obligations/"+value.ID+"?adjusted=1", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "义务金额已调整并形成新的 revision") ||
		!strings.Contains(body, "原始金额</dt><dd>100.5 CNY") ||
		!strings.Contains(body, "当前有效金额</dt><dd>120.5 CNY") ||
		!strings.Contains(body, "金额调整历史") || !strings.Contains(body, "追加服务范围") ||
		!strings.Contains(body, "100.5 CNY") || !strings.Contains(body, "120.5 CNY") ||
		!strings.Contains(body, `/obligations/`+value.ID+`/history`) ||
		!strings.Contains(body, `action="/obligations/`+value.ID+`/adjust-amount"`) ||
		strings.Contains(body, `action="/obligations/`+value.ID+`/confirm"`) {
		t.Fatalf("response = %d %q", response.Code, body)
	}
}

func TestAdministratorRecordsObligationRemissionWithPrincipalActor(t *testing.T) {
	identities := administratorIdentityStub()
	value := sampleOpenObligation()
	obligations := &obligationStub{remissionResult: obligation.RecordRemissionResult{
		Obligation: obligation.Obligation{ID: value.ID, Status: obligation.StatusOpen, CurrentRevision: 1, RemittedAmount: "20", OutstandingAmount: "80.5"},
		Remission:  obligation.Remission{ID: "66666666-6666-4666-8666-666666666666", ObligationID: value.ID, ObligationRevision: 1, Amount: "20"},
	}}
	handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	token := csrfToken()
	request := authenticatedRequest(http.MethodPost, "/obligations/"+value.ID+"/remissions", url.Values{
		"csrf_token":                {token},
		"command_id":                {"77777777-7777-4777-8777-777777777777"},
		"expected_current_revision": {"1"},
		"remission_amount":          {"20.000000"},
		"remission_reason":          {"双方确认免除部分服务费"},
		"manual_source_reference":   {"减免确认函 RM-001"},
		"recorded_effective_amount": {"999"},
		"currency":                  {"USD"},
		"actor_kind":                {"forged"},
		"actor_id":                  {"forged"},
		"actor_display_name":        {"伪造用户"},
		"recorded_at":               {"2000-01-01T00:00:00Z"},
	})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/obligations/"+value.ID+"?remitted=1" {
		t.Fatalf("status=%d location=%q body=%q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	wantActor := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: identities.principal.UserID, DisplayName: identities.principal.DisplayName}
	input := obligations.remissionInput
	if obligations.remissionCalls != 1 || input.CommandID != "77777777-7777-4777-8777-777777777777" ||
		input.ObligationID != value.ID || input.ExpectedCurrentRevision != 1 || input.Amount != "20.000000" ||
		input.Reason != "双方确认免除部分服务费" || input.ManualSourceReference != "减免确认函 RM-001" || input.Actor != wantActor {
		t.Fatalf("remission input = %#v", input)
	}
}

func TestObligationRemissionRequiresCSRF(t *testing.T) {
	identities := administratorIdentityStub()
	obligations := &obligationStub{}
	handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodPost,
		"/obligations/55555555-5555-4555-8555-555555555555/remissions", url.Values{"command_id": {"bad"}}))
	if response.Code != http.StatusForbidden || obligations.remissionCalls != 0 {
		t.Fatalf("status=%d remissionCalls=%d", response.Code, obligations.remissionCalls)
	}
}

func TestObligationRemissionErrorsAreExplicit(t *testing.T) {
	value := sampleOpenObligation()
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "validation", err: &obligation.ValidationError{Field: "remission_amount", Message: "减免金额无效"}, status: http.StatusUnprocessableEntity},
		{name: "idempotency", err: &obligation.RemissionIdempotencyConflictError{CommandID: "77777777-7777-4777-8777-777777777777"}, status: http.StatusConflict},
		{name: "state", err: &obligation.RemissionStateConflictError{Status: obligation.StatusDraft}, status: http.StatusConflict},
		{name: "revision", err: &obligation.CurrentRevisionConflictError{Expected: 1, Actual: 2}, status: http.StatusConflict},
		{name: "balance", err: &obligation.RemissionBalanceConflictError{Requested: "20", Outstanding: "10"}, status: http.StatusConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			identities := administratorIdentityStub()
			obligations := &obligationStub{remissionErr: test.err, getResult: value, getFound: true}
			handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			token := csrfToken()
			request := authenticatedRequest(http.MethodPost, "/obligations/"+value.ID+"/remissions", url.Values{
				"csrf_token": {token}, "command_id": {"77777777-7777-4777-8777-777777777777"},
				"expected_current_revision": {"1"}, "remission_amount": {"20"},
				"remission_reason": {"减免原因"}, "manual_source_reference": {"减免来源依据"},
			})
			request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			body := response.Body.String()
			if response.Code != test.status || !strings.Contains(body, test.err.Error()) ||
				!strings.Contains(body, `value="20"`) || !strings.Contains(body, `value="减免原因"`) ||
				!strings.Contains(body, `value="减免来源依据"`) || obligations.getCalls != 1 {
				t.Fatalf("response = %d %q getCalls=%d", response.Code, body, obligations.getCalls)
			}
		})
	}
}

func TestObligationRemissionNotFoundAndFailure(t *testing.T) {
	identities := administratorIdentityStub()
	for _, test := range []struct {
		name   string
		err    error
		status int
	}{
		{name: "not found", err: &obligation.ObligationNotFoundError{ObligationID: "55555555-5555-4555-8555-555555555555"}, status: http.StatusNotFound},
		{name: "invalid id", err: &obligation.ValidationError{Field: "obligation_id", Message: "义务标识无效"}, status: http.StatusNotFound},
		{name: "database failure", err: errors.New("database unavailable"), status: http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			obligations := &obligationStub{remissionErr: test.err}
			handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			token := csrfToken()
			request := authenticatedRequest(http.MethodPost, "/obligations/55555555-5555-4555-8555-555555555555/remissions", url.Values{
				"csrf_token": {token}, "command_id": {"77777777-7777-4777-8777-777777777777"},
				"expected_current_revision": {"1"}, "remission_amount": {"20"},
				"remission_reason": {"减免原因"}, "manual_source_reference": {"减免来源依据"},
			})
			request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status || obligations.getCalls != 0 {
				t.Fatalf("response = %d %q getCalls=%d", response.Code, response.Body.String(), obligations.getCalls)
			}
		})
	}
}

func TestRemittedObligationDetailShowsBalancesHistoryAndForms(t *testing.T) {
	identities := administratorIdentityStub()
	value := sampleOpenObligation()
	value.RemittedAmount = "20"
	value.OutstandingAmount = "80.5"
	recordedBy := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: identities.principal.UserID, DisplayName: identities.principal.DisplayName}
	details := obligation.Details{
		Obligation: value,
		Revisions: []obligation.Revision{{
			ObligationID: value.ID, Revision: 1, SourceDraftVersion: 1,
			RevisionKind: obligation.RevisionKindConfirmation, EffectiveAmount: "100.5", Currency: "CNY", RecordedBy: recordedBy,
		}},
		Remissions: []obligation.Remission{{
			ID: "66666666-6666-4666-8666-666666666666", ObligationID: value.ID,
			ObligationRevision: 1, Amount: "20", RecordedEffectiveAmount: "100.5", Currency: "CNY",
			Reason: "双方确认免除部分服务费", ManualSourceReference: "减免确认函 RM-001", RecordedBy: recordedBy,
		}},
	}
	obligations := &obligationStub{detailsResult: details, getFound: true}
	handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/obligations/"+value.ID+"?remitted=1", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "义务减免已记录，原 revision 保持不变") ||
		!strings.Contains(body, "已减免金额</dt><dd>20 CNY") || !strings.Contains(body, "剩余金额</dt><dd>80.5 CNY") ||
		!strings.Contains(body, "减免历史") || !strings.Contains(body, "双方确认免除部分服务费") ||
		!strings.Contains(body, "减免确认函 RM-001") ||
		!strings.Contains(body, `action="/obligations/`+value.ID+`/remissions"`) ||
		!strings.Contains(body, `action="/obligations/`+value.ID+`/adjust-amount"`) ||
		strings.Contains(body, `action="/obligations/`+value.ID+`/confirm"`) {
		t.Fatalf("response = %d %q", response.Code, body)
	}
}

func TestAdministratorClosesObligationWithPrincipalActor(t *testing.T) {
	identities := administratorIdentityStub()
	value := sampleClosableObligation()
	closed := value
	closed.Status = obligation.StatusClosed
	closure := sampleObligationClosure(value)
	obligations := &obligationStub{closeResult: obligation.CloseResult{
		Obligation: closed,
		Closure:    closure,
	}}
	handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	token := csrfToken()
	request := authenticatedRequest(http.MethodPost, "/obligations/"+value.ID+"/close", url.Values{
		"csrf_token":                {token},
		"command_id":                {"66666666-6666-4666-8666-666666666666"},
		"expected_current_revision": {"1"},
		"expected_balance_sequence": {"3"},
		"closure_reason":            {" 余额事实已由双方复核 "},
		"recorded_effective_amount": {"999"},
		"net_allocated_amount":      {"999"},
		"remitted_amount":           {"999"},
		"settlement_basis":          {"forged"},
		"closed_at":                 {"2000-01-01T00:00:00Z"},
		"actor_id":                  {"forged"},
	})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/obligations/"+value.ID+"?closed=1" {
		t.Fatalf("status=%d location=%q body=%q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	input := obligations.closeInput
	if obligations.closeCalls != 1 || input.CommandID != "66666666-6666-4666-8666-666666666666" ||
		input.ObligationID != value.ID || input.ExpectedCurrentRevision != 1 || input.ExpectedBalanceSequence != 3 ||
		input.Reason != " 余额事实已由双方复核 " {
		t.Fatalf("close input = %#v calls=%d", input, obligations.closeCalls)
	}
	wantActor := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: identities.principal.UserID, DisplayName: identities.principal.DisplayName}
	if input.Actor != wantActor {
		t.Fatalf("actor = %#v, want %#v", input.Actor, wantActor)
	}
}

func TestObligationCloseRequiresCSRF(t *testing.T) {
	identities := administratorIdentityStub()
	obligations := &obligationStub{}
	handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodPost, "/obligations/55555555-5555-4555-8555-555555555555/close", url.Values{
		"closure_reason": {"错误"},
	}))
	if response.Code != http.StatusForbidden || obligations.closeCalls != 0 {
		t.Fatalf("status=%d closeCalls=%d", response.Code, obligations.closeCalls)
	}
}

func TestObligationCloseErrorsPreserveReasonWithFreshCommand(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		status int
	}{
		{name: "validation", err: &obligation.ValidationError{Field: "closure_reason", Message: "关闭原因不能为空"}, status: http.StatusUnprocessableEntity},
		{name: "idempotency", err: &obligation.ClosureIdempotencyConflictError{}, status: http.StatusConflict},
		{name: "state", err: &obligation.ClosureStateConflictError{Status: obligation.StatusClosed}, status: http.StatusConflict},
		{name: "revision", err: &obligation.CurrentRevisionConflictError{Expected: 1, Actual: 2}, status: http.StatusConflict},
		{name: "sequence", err: &obligation.BalanceSequenceConflictError{Expected: 2, Actual: 3}, status: http.StatusConflict},
		{name: "balance", err: &obligation.ClosureBalanceConflictError{Outstanding: "1"}, status: http.StatusConflict},
	} {
		t.Run(test.name, func(t *testing.T) {
			identities := administratorIdentityStub()
			value := sampleClosableObligation()
			obligations := &obligationStub{
				closeErr: test.err, detailsResult: obligation.Details{Obligation: value}, getFound: true,
			}
			handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatal(err)
			}
			token := csrfToken()
			request := authenticatedRequest(http.MethodPost, "/obligations/"+value.ID+"/close", url.Values{
				"csrf_token": {token}, "command_id": {"66666666-6666-4666-8666-666666666666"},
				"expected_current_revision": {"1"}, "expected_balance_sequence": {"3"},
				"closure_reason": {"保留的关闭原因"},
			})
			request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			body := response.Body.String()
			if response.Code != test.status || !strings.Contains(body, test.err.Error()) ||
				!strings.Contains(body, `value="保留的关闭原因"`) ||
				strings.Contains(body, `name="command_id" value="66666666-6666-4666-8666-666666666666"`) {
				t.Fatalf("response = %d %q", response.Code, body)
			}
		})
	}
}

func TestObligationCloseNotFoundAndFailure(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		status int
	}{
		{name: "invalid ID", err: &obligation.ValidationError{Field: "obligation_id", Message: "义务标识无效"}, status: http.StatusNotFound},
		{name: "not found", err: &obligation.ObligationNotFoundError{}, status: http.StatusNotFound},
		{name: "database failure", err: errors.New("database unavailable"), status: http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			identities := administratorIdentityStub()
			obligations := &obligationStub{closeErr: test.err}
			handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatal(err)
			}
			token := csrfToken()
			request := authenticatedRequest(http.MethodPost, "/obligations/55555555-5555-4555-8555-555555555555/close", url.Values{
				"csrf_token": {token}, "command_id": {"66666666-6666-4666-8666-666666666666"},
				"expected_current_revision": {"1"}, "expected_balance_sequence": {"1"}, "closure_reason": {"错误"},
			})
			request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestObligationDetailShowsClosureActionAndTerminalFact(t *testing.T) {
	identities := administratorIdentityStub()
	value := sampleClosableObligation()
	obligations := &obligationStub{detailsResult: obligation.Details{Obligation: value}, getFound: true}
	handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/obligations/"+value.ID, nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, `action="/obligations/`+value.ID+`/close"`) ||
		!strings.Contains(body, `name="expected_balance_sequence" value="3"`) ||
		!strings.Contains(body, "余额事实序号</dt><dd>3") {
		t.Fatalf("open zero-balance response = %d %q", response.Code, body)
	}

	closed := value
	closed.Status = obligation.StatusClosed
	closure := sampleObligationClosure(closed)
	obligations.detailsResult = obligation.Details{Obligation: closed, Closure: &closure}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/obligations/"+value.ID+"?closed=1", nil))
	body = response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "零余额义务已受控关闭") ||
		!strings.Contains(body, closure.ID) || !strings.Contains(body, closure.Reason) ||
		!strings.Contains(body, closure.SettlementBasis.Label()) ||
		strings.Contains(body, `action="/obligations/`+value.ID+`/close"`) ||
		strings.Contains(body, `action="/obligations/`+value.ID+`/adjust-amount"`) ||
		strings.Contains(body, `action="/obligations/`+value.ID+`/remissions"`) {
		t.Fatalf("closed response = %d %q", response.Code, body)
	}
}

func TestObligationDraftDetailNotFoundAndFailure(t *testing.T) {
	identities := administratorIdentityStub()
	for _, test := range []struct {
		name        string
		obligations *obligationStub
		allocations *allocationStub
		status      int
	}{
		{name: "not found", obligations: &obligationStub{}, allocations: &allocationStub{}, status: http.StatusNotFound},
		{name: "obligation failure", obligations: &obligationStub{getErr: errors.New("database unavailable")}, allocations: &allocationStub{}, status: http.StatusInternalServerError},
		{name: "history failure", obligations: &obligationStub{getResult: sampleObligation(), getFound: true}, allocations: &allocationStub{listForObligationErr: errors.New("allocation database unavailable")}, status: http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler, err := New(readinessStub{}, identities, &partyStub{}, test.obligations, &cashTransactionStub{}, test.allocations, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/obligations/55555555-5555-4555-8555-555555555555", nil))
			if response.Code != test.status {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestObligationEndpointsRequireAdministrator(t *testing.T) {
	identities := administratorIdentityStub()
	identities.principal.Role = identity.Role("viewer")
	parties := &partyStub{}
	obligations := &obligationStub{}
	handler, err := New(readinessStub{}, identities, parties, obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	for _, request := range []*http.Request{
		authenticatedRequest(http.MethodGet, "/obligations", nil),
		authenticatedRequest(http.MethodGet, "/aging", nil),
		authenticatedRequest(http.MethodGet, "/obligations/55555555-5555-4555-8555-555555555555/history", nil),
		authenticatedRequest(http.MethodPost, "/obligations", url.Values{"title": {"越权草稿"}}),
		authenticatedRequest(http.MethodGet, "/obligations/55555555-5555-4555-8555-555555555555", nil),
		authenticatedRequest(http.MethodPost, "/obligations/55555555-5555-4555-8555-555555555555/confirm", url.Values{"command_id": {"越权确认"}}),
		authenticatedRequest(http.MethodPost, "/obligations/55555555-5555-4555-8555-555555555555/adjust-amount", url.Values{"command_id": {"越权调整"}}),
		authenticatedRequest(http.MethodPost, "/obligations/55555555-5555-4555-8555-555555555555/remissions", url.Values{"command_id": {"越权减免"}}),
		authenticatedRequest(http.MethodPost, "/obligations/55555555-5555-4555-8555-555555555555/close", url.Values{"command_id": {"越权关闭"}}),
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("%s %s status = %d", request.Method, request.URL.Path, response.Code)
		}
	}
	if parties.listCalls != 0 || obligations.createCalls != 0 || obligations.confirmCalls != 0 || obligations.adjustCalls != 0 || obligations.remissionCalls != 0 || obligations.closeCalls != 0 || obligations.agingCalls != 0 || obligations.historicalCalls != 0 || obligations.listCalls != 0 || obligations.getCalls != 0 {
		t.Fatalf("use cases called parties=%d create=%d confirm=%d adjust=%d remission=%d close=%d aging=%d historical=%d list=%d get=%d", parties.listCalls, obligations.createCalls, obligations.confirmCalls, obligations.adjustCalls, obligations.remissionCalls, obligations.closeCalls, obligations.agingCalls, obligations.historicalCalls, obligations.listCalls, obligations.getCalls)
	}
}

func TestObligationListFailuresAreNotRenderedAsEmpty(t *testing.T) {
	identities := administratorIdentityStub()
	for _, test := range []struct {
		name        string
		parties     *partyStub
		obligations *obligationStub
	}{
		{name: "parties", parties: &partyStub{listErr: errors.New("party database unavailable")}, obligations: &obligationStub{}},
		{name: "obligations", parties: &partyStub{listResult: obligationParties()}, obligations: &obligationStub{listErr: errors.New("obligation database unavailable")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler, err := New(readinessStub{}, identities, test.parties, test.obligations, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/obligations", nil))
			if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "尚未建立义务草稿") {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestAdministratorCanOpenCashTransactionList(t *testing.T) {
	identities := administratorIdentityStub()
	parties := &partyStub{listResult: obligationParties()}
	cashTransactions := &cashTransactionStub{listResult: []cashtransaction.Transaction{sampleCashTransaction()}}
	handler, err := New(readinessStub{}, identities, parties, &obligationStub{}, cashTransactions, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/cash-transactions", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "实际收款／付款") ||
		!strings.Contains(body, "九月服务费收款") || !strings.Contains(body, "Owner Company") ||
		!strings.Contains(body, "Counter Person") || !strings.Contains(body, "100.5 CNY") ||
		!strings.Contains(body, `name="command_id" value="`) {
		t.Fatalf("response = %d %q", response.Code, body)
	}
	if parties.listCalls != 1 || cashTransactions.listCalls != 1 {
		t.Fatalf("calls parties=%d cash=%d", parties.listCalls, cashTransactions.listCalls)
	}
	csrf := findCookie(t, response.Result().Cookies(), csrfCookieName)
	if !strings.Contains(body, csrf.Value) {
		t.Fatal("cash transaction form does not contain CSRF token")
	}
}

func TestCashTransactionPageRequiresTwoParties(t *testing.T) {
	identities := administratorIdentityStub()
	parties := &partyStub{listResult: obligationParties()[:1]}
	handler, err := New(readinessStub{}, identities, parties, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/cash-transactions", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "至少需要在主体目录登记两个不同主体") ||
		strings.Contains(response.Body.String(), `action="/cash-transactions"`) {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestAdministratorCreatesCashTransactionWithPrincipalActor(t *testing.T) {
	identities := administratorIdentityStub()
	parties := &partyStub{listResult: obligationParties()}
	cashTransactions := &cashTransactionStub{createResult: cashtransaction.CreateResult{
		Transaction: cashtransaction.Transaction{ID: "55555555-5555-4555-8555-555555555555"},
	}}
	handler, err := New(readinessStub{}, identities, parties, &obligationStub{}, cashTransactions, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	token := csrfToken()
	request := authenticatedRequest(http.MethodPost, "/cash-transactions", url.Values{
		"csrf_token":                     {token},
		"command_id":                     {"66666666-6666-4666-8666-666666666666"},
		"type":                           {"receipt"},
		"ledger_owner_party_id":          {"22222222-2222-4222-8222-222222222222"},
		"counterparty_party_id":          {"33333333-3333-4333-8333-333333333333"},
		"description":                    {"  九月服务费收款  "},
		"amount":                         {" 100.50 "},
		"currency":                       {" cny "},
		"occurred_on":                    {"2026-09-19"},
		"account_reference":              {" 基本户尾号 1234 "},
		"manual_source_reference":        {" 银行回单截图 CASH-001 "},
		"external_transaction_reference": {" BANK-TXN-001 "},
		"actor_kind":                     {"forged"},
		"actor_id":                       {"forged"},
		"actor_display_name":             {"伪造用户"},
	})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/cash-transactions/55555555-5555-4555-8555-555555555555?created=1" {
		t.Fatalf("status=%d location=%q body=%q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	input := cashTransactions.createInput
	if cashTransactions.createCalls != 1 || input.CommandID != "66666666-6666-4666-8666-666666666666" ||
		input.Description != "  九月服务费收款  " || input.Amount != " 100.50 " || input.Currency != " cny " ||
		input.AccountReference != " 基本户尾号 1234 " || input.ManualSourceReference != " 银行回单截图 CASH-001 " ||
		input.ExternalTransactionReference != " BANK-TXN-001 " {
		t.Fatalf("create input = %#v", input)
	}
	wantActor := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: identities.principal.UserID, DisplayName: identities.principal.DisplayName}
	if input.Actor != wantActor {
		t.Fatalf("actor = %#v, want %#v", input.Actor, wantActor)
	}
}

func TestCashTransactionCreateRequiresCSRF(t *testing.T) {
	identities := administratorIdentityStub()
	cashTransactions := &cashTransactionStub{}
	handler, err := New(readinessStub{}, identities, &partyStub{}, &obligationStub{}, cashTransactions, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodPost, "/cash-transactions", url.Values{"description": {"无 CSRF"}}))
	if response.Code != http.StatusForbidden || cashTransactions.createCalls != 0 {
		t.Fatalf("status=%d createCalls=%d", response.Code, cashTransactions.createCalls)
	}
}

func TestCashTransactionCreateErrorsPreserveIntentWithFreshCommand(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "validation", err: &cashtransaction.ValidationError{Field: "description", Message: "收付款描述不能为空"}, status: http.StatusUnprocessableEntity},
		{name: "party missing", err: &cashtransaction.PartyNotFoundError{PartyID: "33333333-3333-4333-8333-333333333333"}, status: http.StatusUnprocessableEntity},
		{name: "idempotency", err: &cashtransaction.IdempotencyConflictError{CommandID: "66666666-6666-4666-8666-666666666666"}, status: http.StatusConflict},
		{name: "external duplicate", err: &cashtransaction.ExternalReferenceConflictError{ExistingTransactionID: "55555555-5555-4555-8555-555555555555"}, status: http.StatusConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			identities := administratorIdentityStub()
			parties := &partyStub{listResult: obligationParties()}
			cashTransactions := &cashTransactionStub{createErr: test.err}
			handler, err := New(readinessStub{}, identities, parties, &obligationStub{}, cashTransactions, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			token := csrfToken()
			request := authenticatedRequest(http.MethodPost, "/cash-transactions", url.Values{
				"csrf_token": {token}, "command_id": {"66666666-6666-4666-8666-666666666666"},
				"type": {"payment"}, "ledger_owner_party_id": {"22222222-2222-4222-8222-222222222222"},
				"counterparty_party_id": {"33333333-3333-4333-8333-333333333333"},
				"description":           {"保留的付款描述"}, "amount": {"88.5"}, "currency": {"USD"},
				"occurred_on": {"2026-09-19"}, "account_reference": {"保留的账户标签"},
				"manual_source_reference": {"保留的人工依据"}, "external_transaction_reference": {"保留的外部流水"},
			})
			request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			body := response.Body.String()
			if response.Code != test.status || !strings.Contains(body, test.err.Error()) ||
				!strings.Contains(body, `value="保留的付款描述"`) || !strings.Contains(body, `value="保留的账户标签"`) ||
				!strings.Contains(body, `value="保留的人工依据"`) || !strings.Contains(body, `value="保留的外部流水"`) ||
				strings.Contains(body, `name="command_id" value="66666666-6666-4666-8666-666666666666"`) {
				t.Fatalf("response = %d %q", response.Code, body)
			}
		})
	}
}

func TestAdministratorCanReadCashTransactionDetail(t *testing.T) {
	identities := administratorIdentityStub()
	value := sampleCashTransaction()
	value.AllocatedAmount = "60.5"
	value.UnallocatedAmount = "40"
	cashTransactions := &cashTransactionStub{getResult: value, getFound: true}
	allocations := &allocationStub{listForCashResult: []allocation.Allocation{sampleAllocation()}}
	handler, err := New(readinessStub{}, identities, &partyStub{}, &obligationStub{}, cashTransactions, allocations, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/cash-transactions/"+value.ID+"?created=1", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "实际收付款已登记") ||
		!strings.Contains(body, "付款方 payer") || !strings.Contains(body, "Owner Company") ||
		!strings.Contains(body, "Counter Person") || !strings.Contains(body, "100.5 CNY") ||
		!strings.Contains(body, "BANK-TXN-001") || !strings.Contains(body, "资金分配历史") ||
		!strings.Contains(body, "九月服务费") || !strings.Contains(body, "人工核对资金和义务") ||
		!strings.Contains(body, "暂不能冲销") || !strings.Contains(body, "60.5 CNY 净分配额") ||
		strings.Contains(body, "确认完整冲销") ||
		cashTransactions.getID != value.ID || allocations.listForCashID != value.ID || allocations.listForCashCalls != 1 {
		t.Fatalf("response = %d %q getID=%q", response.Code, body, cashTransactions.getID)
	}
}

func TestAdministratorReversesCashTransactionWithPrincipalActor(t *testing.T) {
	identities := administratorIdentityStub()
	value := sampleCashTransaction()
	cashTransactions := &cashTransactionStub{reverseResult: cashtransaction.ReverseResult{Reversal: cashtransaction.Reversal{
		ID: "88888888-8888-4888-8888-888888888888", CashTransactionID: value.ID,
	}}}
	handler, err := New(readinessStub{}, identities, &partyStub{}, &obligationStub{}, cashTransactions, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	token := csrfToken()
	request := authenticatedRequest(http.MethodPost, "/cash-transactions/"+value.ID+"/reverse", url.Values{
		"csrf_token":         {token},
		"command_id":         {"66666666-6666-4666-8666-666666666666"},
		"reason":             {" 原收款重复录入 "},
		"transaction_type":   {"payment"},
		"amount":             {"0.01"},
		"currency":           {"USD"},
		"actor_kind":         {"forged"},
		"actor_id":           {"forged"},
		"actor_display_name": {"伪造用户"},
		"reversed_at":        {"2000-01-01T00:00:00Z"},
	})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/cash-transactions/"+value.ID+"?reversed=1" {
		t.Fatalf("status=%d location=%q body=%q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	input := cashTransactions.reverseInput
	if cashTransactions.reverseCalls != 1 || input.CommandID != "66666666-6666-4666-8666-666666666666" ||
		input.CashTransactionID != value.ID || input.Reason != " 原收款重复录入 " {
		t.Fatalf("reverse input = %#v calls=%d", input, cashTransactions.reverseCalls)
	}
	wantActor := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: identities.principal.UserID, DisplayName: identities.principal.DisplayName}
	if input.Actor != wantActor {
		t.Fatalf("actor = %#v, want %#v", input.Actor, wantActor)
	}
}

func TestCashTransactionReverseRequiresCSRF(t *testing.T) {
	identities := administratorIdentityStub()
	cashTransactions := &cashTransactionStub{}
	handler, err := New(readinessStub{}, identities, &partyStub{}, &obligationStub{}, cashTransactions, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodPost, "/cash-transactions/55555555-5555-4555-8555-555555555555/reverse", url.Values{"reason": {"错误"}}))
	if response.Code != http.StatusForbidden || cashTransactions.reverseCalls != 0 {
		t.Fatalf("status=%d reverseCalls=%d", response.Code, cashTransactions.reverseCalls)
	}
}

func TestCashTransactionReverseErrorsPreserveReasonWithFreshCommand(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		status int
	}{
		{name: "validation", err: &cashtransaction.ValidationError{Field: "reason", Message: "冲销原因不能为空"}, status: http.StatusUnprocessableEntity},
		{name: "idempotency", err: &cashtransaction.ReversalIdempotencyConflictError{}, status: http.StatusConflict},
	} {
		t.Run(test.name, func(t *testing.T) {
			identities := administratorIdentityStub()
			value := sampleCashTransaction()
			cashTransactions := &cashTransactionStub{reverseErr: test.err, getResult: value, getFound: true}
			handler, err := New(readinessStub{}, identities, &partyStub{}, &obligationStub{}, cashTransactions, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatal(err)
			}
			token := csrfToken()
			request := authenticatedRequest(http.MethodPost, "/cash-transactions/"+value.ID+"/reverse", url.Values{
				"csrf_token": {token}, "command_id": {"66666666-6666-4666-8666-666666666666"}, "reason": {"保留的冲销原因"},
			})
			request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			body := response.Body.String()
			if response.Code != test.status || !strings.Contains(body, test.err.Error()) ||
				!strings.Contains(body, `value="保留的冲销原因"`) ||
				strings.Contains(body, `name="command_id" value="66666666-6666-4666-8666-666666666666"`) {
				t.Fatalf("response = %d %q", response.Code, body)
			}
		})
	}
}

func TestCashTransactionAllocatedConflictShowsRequiredAllocationReversal(t *testing.T) {
	identities := administratorIdentityStub()
	value := sampleCashTransaction()
	value.AllocatedAmount = "60.5"
	value.UnallocatedAmount = "40"
	conflict := &cashtransaction.TransactionAllocatedConflictError{CashTransactionID: value.ID, AllocatedAmount: value.AllocatedAmount}
	cashTransactions := &cashTransactionStub{reverseErr: conflict, getResult: value, getFound: true}
	allocations := &allocationStub{listForCashResult: []allocation.Allocation{sampleAllocation()}}
	handler, err := New(readinessStub{}, identities, &partyStub{}, &obligationStub{}, cashTransactions, allocations, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	token := csrfToken()
	request := authenticatedRequest(http.MethodPost, "/cash-transactions/"+value.ID+"/reverse", url.Values{
		"csrf_token": {token}, "command_id": {"66666666-6666-4666-8666-666666666666"}, "reason": {"原记录错误"},
	})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	body := response.Body.String()
	if response.Code != http.StatusConflict || !strings.Contains(body, conflict.Error()) ||
		!strings.Contains(body, "先在下方历史中进入每条 active Allocation") ||
		!strings.Contains(body, sampleAllocation().ID) || strings.Contains(body, "确认完整冲销") {
		t.Fatalf("response = %d %q", response.Code, body)
	}
}

func TestCashTransactionReverseConflictRendersPersistedReversal(t *testing.T) {
	identities := administratorIdentityStub()
	value := sampleReversedCashTransaction()
	conflict := &cashtransaction.TransactionAlreadyReversedError{CashTransactionID: value.ID, ReversalID: value.Reversal.ID}
	cashTransactions := &cashTransactionStub{reverseErr: conflict, getResult: value, getFound: true}
	handler, err := New(readinessStub{}, identities, &partyStub{}, &obligationStub{}, cashTransactions, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	token := csrfToken()
	request := authenticatedRequest(http.MethodPost, "/cash-transactions/"+value.ID+"/reverse", url.Values{
		"csrf_token": {token}, "command_id": {"66666666-6666-4666-8666-666666666666"}, "reason": {"再次冲销"},
	})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	body := response.Body.String()
	if response.Code != http.StatusConflict || !strings.Contains(body, conflict.Error()) ||
		!strings.Contains(body, value.Reversal.Reason) || strings.Contains(body, "确认完整冲销") {
		t.Fatalf("response = %d %q", response.Code, body)
	}
}

func TestCashTransactionReverseNotFoundAndFailure(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		status int
	}{
		{name: "invalid ID", err: &cashtransaction.ValidationError{Field: "cash_transaction_id", Message: "实际收付款标识无效"}, status: http.StatusNotFound},
		{name: "not found", err: &cashtransaction.TransactionNotFoundError{}, status: http.StatusNotFound},
		{name: "database failure", err: errors.New("database unavailable"), status: http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			identities := administratorIdentityStub()
			cashTransactions := &cashTransactionStub{reverseErr: test.err}
			handler, err := New(readinessStub{}, identities, &partyStub{}, &obligationStub{}, cashTransactions, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatal(err)
			}
			token := csrfToken()
			request := authenticatedRequest(http.MethodPost, "/cash-transactions/55555555-5555-4555-8555-555555555555/reverse", url.Values{
				"csrf_token": {token}, "command_id": {"66666666-6666-4666-8666-666666666666"}, "reason": {"错误"},
			})
			request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestAdministratorCanReadReversedCashTransactionDetail(t *testing.T) {
	identities := administratorIdentityStub()
	value := sampleReversedCashTransaction()
	cashTransactions := &cashTransactionStub{getResult: value, getFound: true}
	handler, err := New(readinessStub{}, identities, &partyStub{}, &obligationStub{}, cashTransactions, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/cash-transactions/"+value.ID+"?reversed=1", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "当前有效金额和未分配金额已归零") ||
		!strings.Contains(body, value.Reversal.ID) || !strings.Contains(body, value.Reversal.Reason) ||
		!strings.Contains(body, value.Reversal.ReversedBy.DisplayName) || !strings.Contains(body, "当前有效金额</dt><dd>0 CNY") ||
		strings.Contains(body, "确认完整冲销") {
		t.Fatalf("response = %d %q", response.Code, body)
	}
}

func TestCashTransactionDetailNotFoundAndFailure(t *testing.T) {
	identities := administratorIdentityStub()
	for _, test := range []struct {
		name             string
		cashTransactions *cashTransactionStub
		allocations      *allocationStub
		status           int
	}{
		{name: "not found", cashTransactions: &cashTransactionStub{}, allocations: &allocationStub{}, status: http.StatusNotFound},
		{name: "cash failure", cashTransactions: &cashTransactionStub{getErr: errors.New("database unavailable")}, allocations: &allocationStub{}, status: http.StatusInternalServerError},
		{name: "history failure", cashTransactions: &cashTransactionStub{getResult: sampleCashTransaction(), getFound: true}, allocations: &allocationStub{listForCashErr: errors.New("allocation database unavailable")}, status: http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler, err := New(readinessStub{}, identities, &partyStub{}, &obligationStub{}, test.cashTransactions, test.allocations, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/cash-transactions/55555555-5555-4555-8555-555555555555", nil))
			if response.Code != test.status {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestCashTransactionEndpointsRequireAdministrator(t *testing.T) {
	identities := administratorIdentityStub()
	identities.principal.Role = identity.Role("viewer")
	parties := &partyStub{}
	cashTransactions := &cashTransactionStub{}
	handler, err := New(readinessStub{}, identities, parties, &obligationStub{}, cashTransactions, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	for _, request := range []*http.Request{
		authenticatedRequest(http.MethodGet, "/cash-transactions", nil),
		authenticatedRequest(http.MethodPost, "/cash-transactions", url.Values{"description": {"越权资金"}}),
		authenticatedRequest(http.MethodGet, "/cash-transactions/55555555-5555-4555-8555-555555555555", nil),
		authenticatedRequest(http.MethodPost, "/cash-transactions/55555555-5555-4555-8555-555555555555/reverse", url.Values{"reason": {"越权冲销"}}),
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("%s %s status = %d", request.Method, request.URL.Path, response.Code)
		}
	}
	if parties.listCalls != 0 || cashTransactions.createCalls != 0 || cashTransactions.reverseCalls != 0 || cashTransactions.listCalls != 0 || cashTransactions.getCalls != 0 {
		t.Fatalf("use cases called parties=%d create=%d reverse=%d list=%d get=%d", parties.listCalls, cashTransactions.createCalls, cashTransactions.reverseCalls, cashTransactions.listCalls, cashTransactions.getCalls)
	}
}

func TestCashTransactionListFailuresAreNotRenderedAsEmpty(t *testing.T) {
	identities := administratorIdentityStub()
	for _, test := range []struct {
		name             string
		parties          *partyStub
		cashTransactions *cashTransactionStub
	}{
		{name: "parties", parties: &partyStub{listErr: errors.New("party database unavailable")}, cashTransactions: &cashTransactionStub{}},
		{name: "cash transactions", parties: &partyStub{listResult: obligationParties()}, cashTransactions: &cashTransactionStub{listErr: errors.New("cash database unavailable")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler, err := New(readinessStub{}, identities, test.parties, &obligationStub{}, test.cashTransactions, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/cash-transactions", nil))
			if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "尚未登记实际收付款") {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestAdministratorCanOpenAllocationList(t *testing.T) {
	identities := administratorIdentityStub()
	cashTransactions := &cashTransactionStub{listResult: []cashtransaction.Transaction{sampleCashTransaction()}}
	obligations := &obligationStub{listResult: []obligation.Obligation{sampleOpenObligation()}}
	allocations := &allocationStub{listResult: []allocation.Allocation{sampleAllocation()}}
	handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, cashTransactions, allocations, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/allocations", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "资金分配与余额") ||
		!strings.Contains(body, "九月服务费收款") || !strings.Contains(body, "未分配 100.5 CNY") ||
		!strings.Contains(body, "九月服务费") || !strings.Contains(body, "剩余 100.5 CNY") ||
		!strings.Contains(body, "人工核对资金和义务") || !strings.Contains(body, `name="command_id" value="`) {
		t.Fatalf("response = %d %q", response.Code, body)
	}
	if cashTransactions.listCalls != 1 || obligations.listCalls != 1 || allocations.listCalls != 1 {
		t.Fatalf("calls cash=%d obligations=%d allocations=%d", cashTransactions.listCalls, obligations.listCalls, allocations.listCalls)
	}
	csrf := findCookie(t, response.Result().Cookies(), csrfCookieName)
	if !strings.Contains(body, csrf.Value) {
		t.Fatal("allocation form does not contain CSRF token")
	}
}

func TestAllocationPageRequiresAvailableCashAndOpenObligation(t *testing.T) {
	identities := administratorIdentityStub()
	for _, test := range []struct {
		name       string
		cash       cashtransaction.Transaction
		obligation obligation.Obligation
	}{
		{name: "cash exhausted", cash: func() cashtransaction.Transaction {
			value := sampleCashTransaction()
			value.AllocatedAmount = value.Amount
			value.UnallocatedAmount = "0"
			return value
		}(), obligation: sampleOpenObligation()},
		{name: "obligation draft", cash: sampleCashTransaction(), obligation: sampleObligation()},
		{name: "obligation settled", cash: sampleCashTransaction(), obligation: func() obligation.Obligation {
			value := sampleOpenObligation()
			value.AllocatedAmount = value.OriginalAmount
			value.OutstandingAmount = "0"
			return value
		}()},
	} {
		t.Run(test.name, func(t *testing.T) {
			cashTransactions := &cashTransactionStub{listResult: []cashtransaction.Transaction{test.cash}}
			obligations := &obligationStub{listResult: []obligation.Obligation{test.obligation}}
			handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, cashTransactions, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/allocations", nil))
			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "至少需要一笔仍有未分配金额") ||
				strings.Contains(response.Body.String(), `action="/allocations"`) {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestAdministratorAppliesAllocationWithPrincipalActor(t *testing.T) {
	identities := administratorIdentityStub()
	cashTransactions := &cashTransactionStub{listResult: []cashtransaction.Transaction{sampleCashTransaction()}}
	obligations := &obligationStub{listResult: []obligation.Obligation{sampleOpenObligation()}}
	allocations := &allocationStub{applyResult: allocation.ApplyResult{
		Allocation: allocation.Allocation{ID: "77777777-7777-4777-8777-777777777777"},
	}}
	handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, cashTransactions, allocations, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	token := csrfToken()
	request := authenticatedRequest(http.MethodPost, "/allocations", url.Values{
		"csrf_token":              {token},
		"command_id":              {"66666666-6666-4666-8666-666666666666"},
		"cash_transaction_id":     {sampleCashTransaction().ID},
		"obligation_selection":    {sampleOpenObligation().ID + ":1"},
		"amount":                  {" 60.50 "},
		"manual_source_reference": {" 人工核对资金和义务 "},
		"actor_kind":              {"forged"},
		"actor_id":                {"forged"},
		"actor_display_name":      {"伪造用户"},
		"currency":                {"USD"},
		"cash_unallocated_amount": {"999999"},
		"obligation_outstanding":  {"999999"},
		"applied_at":              {"2000-01-01T00:00:00Z"},
	})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/allocations/77777777-7777-4777-8777-777777777777?created=1" {
		t.Fatalf("status=%d location=%q body=%q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	input := allocations.applyInput
	if allocations.applyCalls != 1 || input.CommandID != "66666666-6666-4666-8666-666666666666" ||
		input.CashTransactionID != sampleCashTransaction().ID || input.ObligationID != sampleOpenObligation().ID ||
		input.ExpectedObligationRevision != 1 || input.Amount != " 60.50 " ||
		input.ManualSourceReference != " 人工核对资金和义务 " {
		t.Fatalf("apply input = %#v", input)
	}
	wantActor := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: identities.principal.UserID, DisplayName: identities.principal.DisplayName}
	if input.Actor != wantActor {
		t.Fatalf("actor = %#v, want %#v", input.Actor, wantActor)
	}
}

func TestAllocationApplyRequiresCSRF(t *testing.T) {
	identities := administratorIdentityStub()
	allocations := &allocationStub{}
	handler, err := New(readinessStub{}, identities, &partyStub{}, &obligationStub{}, &cashTransactionStub{}, allocations, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodPost, "/allocations", url.Values{"amount": {"1"}}))
	if response.Code != http.StatusForbidden || allocations.applyCalls != 0 {
		t.Fatalf("status=%d applyCalls=%d", response.Code, allocations.applyCalls)
	}
}

func TestAllocationApplyErrorsPreserveIntentWithFreshCommand(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "validation", err: &allocation.ValidationError{Field: "amount", Message: "分配金额必须大于 0"}, status: http.StatusUnprocessableEntity},
		{name: "cash missing", err: &allocation.CashTransactionNotFoundError{}, status: http.StatusUnprocessableEntity},
		{name: "cash reversed", err: &allocation.CashTransactionStateConflictError{Status: "reversed"}, status: http.StatusConflict},
		{name: "obligation missing", err: &allocation.ObligationNotFoundError{}, status: http.StatusUnprocessableEntity},
		{name: "idempotency", err: &allocation.IdempotencyConflictError{}, status: http.StatusConflict},
		{name: "state", err: &allocation.ObligationStateConflictError{Status: "draft"}, status: http.StatusConflict},
		{name: "revision", err: &allocation.ObligationRevisionConflictError{Expected: 1, Actual: 2}, status: http.StatusConflict},
		{name: "compatibility", err: &allocation.CompatibilityConflictError{Reason: "currency"}, status: http.StatusConflict},
		{name: "cash balance", err: &allocation.CashBalanceConflictError{Requested: "60", Available: "20"}, status: http.StatusConflict},
		{name: "obligation balance", err: &allocation.ObligationBalanceConflictError{Requested: "60", Available: "20"}, status: http.StatusConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			identities := administratorIdentityStub()
			cashTransactions := &cashTransactionStub{listResult: []cashtransaction.Transaction{sampleCashTransaction()}}
			obligations := &obligationStub{listResult: []obligation.Obligation{sampleOpenObligation()}}
			allocations := &allocationStub{applyErr: test.err}
			handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, cashTransactions, allocations, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatal(err)
			}
			token := csrfToken()
			request := authenticatedRequest(http.MethodPost, "/allocations", url.Values{
				"csrf_token": {token}, "command_id": {"66666666-6666-4666-8666-666666666666"},
				"cash_transaction_id":  {sampleCashTransaction().ID},
				"obligation_selection": {sampleOpenObligation().ID + ":1"},
				"amount":               {"60.5"}, "manual_source_reference": {"保留的分配依据"},
			})
			request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			body := response.Body.String()
			if response.Code != test.status || !strings.Contains(body, test.err.Error()) ||
				!strings.Contains(body, `value="60.5"`) || !strings.Contains(body, `value="保留的分配依据"`) ||
				strings.Contains(body, `name="command_id" value="66666666-6666-4666-8666-666666666666"`) {
				t.Fatalf("response = %d %q", response.Code, body)
			}
		})
	}
}

func TestAdministratorCanReadAllocationDetail(t *testing.T) {
	identities := administratorIdentityStub()
	value := sampleAllocation()
	allocations := &allocationStub{getResult: value, getFound: true}
	obligations := &obligationStub{getResult: sampleOpenObligation(), getFound: true}
	handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, &cashTransactionStub{}, allocations, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/allocations/"+value.ID+"?created=1", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "资金分配已经生效") ||
		!strings.Contains(body, "九月服务费") || !strings.Contains(body, "九月服务费收款") ||
		!strings.Contains(body, "60.5 CNY") || !strings.Contains(body, "人工核对资金和义务") ||
		!strings.Contains(body, `action="/allocations/`+value.ID+`/reverse"`) || allocations.getID != value.ID {
		t.Fatalf("response = %d %q getID=%q", response.Code, body, allocations.getID)
	}
}

func TestClosedObligationHidesAllocationReversalAndMapsStateConflict(t *testing.T) {
	identities := administratorIdentityStub()
	value := sampleAllocation()
	allocations := &allocationStub{getResult: value, getFound: true}
	closed := sampleClosableObligation()
	closed.Status = obligation.StatusClosed
	closure := sampleObligationClosure(closed)
	obligations := &obligationStub{
		detailsResult: obligation.Details{Obligation: closed, Closure: &closure},
		getFound:      true,
	}
	handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, &cashTransactionStub{}, allocations, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/allocations/"+value.ID, nil))
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "确认完整反向核销") {
		t.Fatalf("closed obligation allocation detail = %d %q", response.Code, response.Body.String())
	}

	conflict := &allocation.ReversalObligationStateConflictError{Status: obligation.StatusClosed}
	allocations.reverseErr = conflict
	token := csrfToken()
	request := authenticatedRequest(http.MethodPost, "/allocations/"+value.ID+"/reverse", url.Values{
		"csrf_token": {token}, "command_id": {"66666666-6666-4666-8666-666666666666"}, "reason": {"不得反向"},
	})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), conflict.Error()) ||
		strings.Contains(response.Body.String(), "确认完整反向核销") {
		t.Fatalf("closed obligation reversal conflict = %d %q", response.Code, response.Body.String())
	}
}

func TestAdministratorReversesAllocationWithPrincipalActor(t *testing.T) {
	identities := administratorIdentityStub()
	value := sampleAllocation()
	reversalID := "88888888-8888-4888-8888-888888888888"
	allocations := &allocationStub{
		getResult: value,
		getFound:  true,
		reverseResult: allocation.ReverseResult{Reversal: allocation.Reversal{
			ID: reversalID, AllocationID: value.ID,
		}},
	}
	handler, err := New(readinessStub{}, identities, &partyStub{}, &obligationStub{}, &cashTransactionStub{}, allocations, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	token := csrfToken()
	request := authenticatedRequest(http.MethodPost, "/allocations/"+value.ID+"/reverse", url.Values{
		"csrf_token":          {token},
		"command_id":          {"66666666-6666-4666-8666-666666666666"},
		"reason":              {" 金额关联错误 "},
		"amount":              {"0.01"},
		"currency":            {"USD"},
		"cash_transaction_id": {"forged"},
		"obligation_id":       {"forged"},
		"obligation_revision": {"99"},
		"actor_kind":          {"forged"},
		"actor_id":            {"forged"},
		"actor_display_name":  {"伪造用户"},
		"reversed_at":         {"2000-01-01T00:00:00Z"},
	})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/allocations/"+value.ID+"?reversed=1" {
		t.Fatalf("status=%d location=%q body=%q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	input := allocations.reverseInput
	if allocations.reverseCalls != 1 || input.CommandID != "66666666-6666-4666-8666-666666666666" ||
		input.AllocationID != value.ID || input.Reason != " 金额关联错误 " {
		t.Fatalf("reverse input = %#v calls=%d", input, allocations.reverseCalls)
	}
	wantActor := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: identities.principal.UserID, DisplayName: identities.principal.DisplayName}
	if input.Actor != wantActor {
		t.Fatalf("actor = %#v, want %#v", input.Actor, wantActor)
	}
}

func TestAllocationReverseRequiresCSRF(t *testing.T) {
	identities := administratorIdentityStub()
	allocations := &allocationStub{}
	handler, err := New(readinessStub{}, identities, &partyStub{}, &obligationStub{}, &cashTransactionStub{}, allocations, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodPost, "/allocations/77777777-7777-4777-8777-777777777777/reverse", url.Values{"reason": {"错误"}}))
	if response.Code != http.StatusForbidden || allocations.reverseCalls != 0 {
		t.Fatalf("status=%d reverseCalls=%d", response.Code, allocations.reverseCalls)
	}
}

func TestAllocationReverseErrorsPreserveReasonWithFreshCommand(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		status int
	}{
		{name: "validation", err: &allocation.ValidationError{Field: "reason", Message: "反向核销原因不能为空"}, status: http.StatusUnprocessableEntity},
		{name: "idempotency", err: &allocation.ReversalIdempotencyConflictError{}, status: http.StatusConflict},
	} {
		t.Run(test.name, func(t *testing.T) {
			identities := administratorIdentityStub()
			value := sampleAllocation()
			allocations := &allocationStub{reverseErr: test.err, getResult: value, getFound: true}
			obligations := &obligationStub{getResult: sampleOpenObligation(), getFound: true}
			handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, &cashTransactionStub{}, allocations, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatal(err)
			}
			token := csrfToken()
			request := authenticatedRequest(http.MethodPost, "/allocations/"+value.ID+"/reverse", url.Values{
				"csrf_token": {token}, "command_id": {"66666666-6666-4666-8666-666666666666"}, "reason": {"保留的反向原因"},
			})
			request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			body := response.Body.String()
			if response.Code != test.status || !strings.Contains(body, test.err.Error()) ||
				!strings.Contains(body, `value="保留的反向原因"`) ||
				strings.Contains(body, `name="command_id" value="66666666-6666-4666-8666-666666666666"`) {
				t.Fatalf("response = %d %q", response.Code, body)
			}
		})
	}
}

func TestAllocationReverseConflictRendersPersistedReversal(t *testing.T) {
	identities := administratorIdentityStub()
	value := sampleReversedAllocation()
	conflict := &allocation.AllocationAlreadyReversedError{AllocationID: value.ID, ReversalID: value.Reversal.ID}
	allocations := &allocationStub{reverseErr: conflict, getResult: value, getFound: true}
	handler, err := New(readinessStub{}, identities, &partyStub{}, &obligationStub{}, &cashTransactionStub{}, allocations, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	token := csrfToken()
	request := authenticatedRequest(http.MethodPost, "/allocations/"+value.ID+"/reverse", url.Values{
		"csrf_token": {token}, "command_id": {"66666666-6666-4666-8666-666666666666"}, "reason": {"再次反向"},
	})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	body := response.Body.String()
	if response.Code != http.StatusConflict || !strings.Contains(body, conflict.Error()) ||
		!strings.Contains(body, value.Reversal.Reason) || strings.Contains(body, "确认完整反向核销") {
		t.Fatalf("response = %d %q", response.Code, body)
	}
}

func TestAllocationReverseNotFoundAndFailure(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		status int
	}{
		{name: "invalid ID", err: &allocation.ValidationError{Field: "allocation_id", Message: "资金分配标识无效"}, status: http.StatusNotFound},
		{name: "not found", err: &allocation.AllocationNotFoundError{}, status: http.StatusNotFound},
		{name: "database failure", err: errors.New("database unavailable"), status: http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			identities := administratorIdentityStub()
			allocations := &allocationStub{reverseErr: test.err}
			handler, err := New(readinessStub{}, identities, &partyStub{}, &obligationStub{}, &cashTransactionStub{}, allocations, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatal(err)
			}
			token := csrfToken()
			request := authenticatedRequest(http.MethodPost, "/allocations/77777777-7777-4777-8777-777777777777/reverse", url.Values{
				"csrf_token": {token}, "command_id": {"66666666-6666-4666-8666-666666666666"}, "reason": {"错误"},
			})
			request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestAdministratorCanReadReversedAllocationDetail(t *testing.T) {
	identities := administratorIdentityStub()
	value := sampleReversedAllocation()
	allocations := &allocationStub{getResult: value, getFound: true}
	handler, err := New(readinessStub{}, identities, &partyStub{}, &obligationStub{}, &cashTransactionStub{}, allocations, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/allocations/"+value.ID+"?reversed=1", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "相关余额已释放") ||
		!strings.Contains(body, value.Reversal.ID) || !strings.Contains(body, value.Reversal.Reason) ||
		!strings.Contains(body, value.Reversal.ReversedBy.DisplayName) || strings.Contains(body, "确认完整反向核销") {
		t.Fatalf("response = %d %q", response.Code, body)
	}
}

func TestAllocationDetailNotFoundAndFailure(t *testing.T) {
	identities := administratorIdentityStub()
	for _, test := range []struct {
		name   string
		stub   *allocationStub
		status int
	}{
		{name: "not found", stub: &allocationStub{}, status: http.StatusNotFound},
		{name: "failure", stub: &allocationStub{getErr: errors.New("database unavailable")}, status: http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler, err := New(readinessStub{}, identities, &partyStub{}, &obligationStub{}, &cashTransactionStub{}, test.stub, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/allocations/77777777-7777-4777-8777-777777777777", nil))
			if response.Code != test.status {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestAllocationEndpointsRequireAdministrator(t *testing.T) {
	identities := administratorIdentityStub()
	identities.principal.Role = identity.Role("viewer")
	cashTransactions := &cashTransactionStub{}
	obligations := &obligationStub{}
	allocations := &allocationStub{}
	handler, err := New(readinessStub{}, identities, &partyStub{}, obligations, cashTransactions, allocations, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []*http.Request{
		authenticatedRequest(http.MethodGet, "/allocations", nil),
		authenticatedRequest(http.MethodPost, "/allocations", url.Values{"amount": {"1"}}),
		authenticatedRequest(http.MethodGet, "/allocations/77777777-7777-4777-8777-777777777777", nil),
		authenticatedRequest(http.MethodPost, "/allocations/77777777-7777-4777-8777-777777777777/reverse", url.Values{"reason": {"错误"}}),
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("%s %s status = %d", request.Method, request.URL.Path, response.Code)
		}
	}
	if cashTransactions.listCalls != 0 || obligations.listCalls != 0 || allocations.applyCalls != 0 || allocations.reverseCalls != 0 || allocations.listCalls != 0 || allocations.getCalls != 0 {
		t.Fatalf("use cases called cash=%d obligations=%d apply=%d reverse=%d list=%d get=%d",
			cashTransactions.listCalls, obligations.listCalls, allocations.applyCalls, allocations.reverseCalls, allocations.listCalls, allocations.getCalls)
	}
}

func TestAllocationListFailuresAreNotRenderedAsEmpty(t *testing.T) {
	identities := administratorIdentityStub()
	for _, test := range []struct {
		name        string
		cash        *cashTransactionStub
		obligations *obligationStub
		allocations *allocationStub
	}{
		{name: "cash", cash: &cashTransactionStub{listErr: errors.New("cash unavailable")}, obligations: &obligationStub{}, allocations: &allocationStub{}},
		{name: "obligations", cash: &cashTransactionStub{listResult: []cashtransaction.Transaction{sampleCashTransaction()}}, obligations: &obligationStub{listErr: errors.New("obligations unavailable")}, allocations: &allocationStub{}},
		{name: "allocations", cash: &cashTransactionStub{listResult: []cashtransaction.Transaction{sampleCashTransaction()}}, obligations: &obligationStub{listResult: []obligation.Obligation{sampleOpenObligation()}}, allocations: &allocationStub{listErr: errors.New("allocations unavailable")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler, err := New(readinessStub{}, identities, &partyStub{}, test.obligations, test.cash, test.allocations, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/allocations", nil))
			if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "尚未形成资金分配") {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestAdministratorCanOpenInternalTransferList(t *testing.T) {
	identities := administratorIdentityStub()
	parties := &partyStub{listResult: obligationParties()[:1]}
	transfers := &internalTransferStub{listResult: []internaltransfer.Transfer{sampleInternalTransfer()}}
	handler, err := New(readinessStub{}, identities, parties, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, transfers)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/internal-transfers", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "内部账户转账") ||
		!strings.Contains(body, "基本户向备用金账户转账") || !strings.Contains(body, "Owner Company") ||
		!strings.Contains(body, "100.5 CNY") || !strings.Contains(body, `name="command_id" value="`) {
		t.Fatalf("response = %d %q", response.Code, body)
	}
	if parties.listCalls != 1 || transfers.listCalls != 1 {
		t.Fatalf("calls parties=%d transfers=%d", parties.listCalls, transfers.listCalls)
	}
	csrf := findCookie(t, response.Result().Cookies(), csrfCookieName)
	if !strings.Contains(body, csrf.Value) {
		t.Fatal("internal transfer form does not contain CSRF token")
	}
}

func TestInternalTransferPageRequiresOneParty(t *testing.T) {
	identities := administratorIdentityStub()
	transfers := &internalTransferStub{}
	handler, err := New(readinessStub{}, identities, &partyStub{}, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, transfers)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/internal-transfers", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "至少需要在主体目录登记一个本方主体") ||
		strings.Contains(response.Body.String(), `action="/internal-transfers"`) {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestAdministratorCreatesInternalTransferWithPrincipalActor(t *testing.T) {
	identities := administratorIdentityStub()
	transfers := &internalTransferStub{createResult: internaltransfer.CreateResult{
		Transfer: internaltransfer.Transfer{ID: "55555555-5555-4555-8555-555555555555"},
	}}
	handler, err := New(readinessStub{}, identities, &partyStub{}, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, transfers)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	token := csrfToken()
	request := authenticatedRequest(http.MethodPost, "/internal-transfers", url.Values{
		"csrf_token":                    {token},
		"command_id":                    {"66666666-6666-4666-8666-666666666666"},
		"ledger_owner_party_id":         {"22222222-2222-4222-8222-222222222222"},
		"description":                   {"  基本户向备用金账户转账  "},
		"amount":                        {" 100.50 "},
		"currency":                      {" cny "},
		"occurred_on":                   {"2026-09-19"},
		"source_account_reference":      {" 基本户尾号 1234 "},
		"destination_account_reference": {" 备用金账户 "},
		"manual_source_reference":       {" 银行回单 TRANSFER-001 "},
		"external_transfer_reference":   {" BANK-TRANSFER-001 "},
		"actor_kind":                    {"forged"},
		"actor_id":                      {"forged"},
		"actor_display_name":            {"伪造用户"},
	})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/internal-transfers/55555555-5555-4555-8555-555555555555?created=1" {
		t.Fatalf("status=%d location=%q body=%q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	input := transfers.createInput
	if transfers.createCalls != 1 || input.CommandID != "66666666-6666-4666-8666-666666666666" ||
		input.Description != "  基本户向备用金账户转账  " || input.Amount != " 100.50 " || input.Currency != " cny " ||
		input.SourceAccountReference != " 基本户尾号 1234 " || input.DestinationAccountReference != " 备用金账户 " ||
		input.ManualSourceReference != " 银行回单 TRANSFER-001 " || input.ExternalTransferReference != " BANK-TRANSFER-001 " {
		t.Fatalf("create input = %#v", input)
	}
	wantActor := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: identities.principal.UserID, DisplayName: identities.principal.DisplayName}
	if input.Actor != wantActor {
		t.Fatalf("actor = %#v, want %#v", input.Actor, wantActor)
	}
}

func TestInternalTransferCreateRequiresCSRF(t *testing.T) {
	identities := administratorIdentityStub()
	transfers := &internalTransferStub{}
	handler, err := New(readinessStub{}, identities, &partyStub{}, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, transfers)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodPost, "/internal-transfers", url.Values{"description": {"无 CSRF"}}))
	if response.Code != http.StatusForbidden || transfers.createCalls != 0 {
		t.Fatalf("status=%d createCalls=%d", response.Code, transfers.createCalls)
	}
}

func TestInternalTransferCreateErrorsPreserveIntentWithFreshCommand(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "validation", err: &internaltransfer.ValidationError{Field: "description", Message: "转账描述不能为空"}, status: http.StatusUnprocessableEntity},
		{name: "party missing", err: &internaltransfer.PartyNotFoundError{PartyID: "22222222-2222-4222-8222-222222222222"}, status: http.StatusUnprocessableEntity},
		{name: "idempotency", err: &internaltransfer.IdempotencyConflictError{CommandID: "66666666-6666-4666-8666-666666666666"}, status: http.StatusConflict},
		{name: "external duplicate", err: &internaltransfer.ExternalReferenceConflictError{ExistingTransferID: "55555555-5555-4555-8555-555555555555"}, status: http.StatusConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			identities := administratorIdentityStub()
			parties := &partyStub{listResult: obligationParties()[:1]}
			transfers := &internalTransferStub{createErr: test.err}
			handler, err := New(readinessStub{}, identities, parties, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, transfers)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			token := csrfToken()
			request := authenticatedRequest(http.MethodPost, "/internal-transfers", url.Values{
				"csrf_token": {token}, "command_id": {"66666666-6666-4666-8666-666666666666"},
				"ledger_owner_party_id": {"22222222-2222-4222-8222-222222222222"},
				"description":           {"保留的内部转账"}, "amount": {"88.5"}, "currency": {"USD"},
				"occurred_on": {"2026-09-19"}, "source_account_reference": {"保留的来源账户"},
				"destination_account_reference": {"保留的目标账户"}, "manual_source_reference": {"保留的人工依据"},
				"external_transfer_reference": {"保留的外部流水"},
			})
			request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			body := response.Body.String()
			if response.Code != test.status || !strings.Contains(body, test.err.Error()) ||
				!strings.Contains(body, `value="保留的内部转账"`) || !strings.Contains(body, `value="保留的来源账户"`) ||
				!strings.Contains(body, `value="保留的目标账户"`) || !strings.Contains(body, `value="保留的人工依据"`) ||
				!strings.Contains(body, `value="保留的外部流水"`) ||
				strings.Contains(body, `name="command_id" value="66666666-6666-4666-8666-666666666666"`) {
				t.Fatalf("response = %d %q", response.Code, body)
			}
		})
	}
}

func TestAdministratorCanReadInternalTransferDetail(t *testing.T) {
	identities := administratorIdentityStub()
	value := sampleInternalTransfer()
	transfers := &internalTransferStub{getResult: value, getFound: true}
	handler, err := New(readinessStub{}, identities, &partyStub{}, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, transfers)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/internal-transfers/"+value.ID+"?created=1", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "内部转账已登记") ||
		!strings.Contains(body, "基本户向备用金账户转账") || !strings.Contains(body, "100.5 CNY") ||
		!strings.Contains(body, "BANK-TRANSFER-001") || !strings.Contains(body, "Owner Company") || transfers.getID != value.ID {
		t.Fatalf("response = %d %q getID=%q", response.Code, body, transfers.getID)
	}
}

func TestInternalTransferDetailNotFoundAndFailure(t *testing.T) {
	identities := administratorIdentityStub()
	for _, test := range []struct {
		name   string
		stub   *internalTransferStub
		status int
	}{
		{name: "not found", stub: &internalTransferStub{}, status: http.StatusNotFound},
		{name: "failure", stub: &internalTransferStub{getErr: errors.New("database unavailable")}, status: http.StatusInternalServerError},
		{name: "invalid", stub: &internalTransferStub{getErr: &internaltransfer.ValidationError{Field: "internal_transfer_id", Message: "invalid"}}, status: http.StatusNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler, err := New(readinessStub{}, identities, &partyStub{}, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, test.stub)
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/internal-transfers/55555555-5555-4555-8555-555555555555", nil))
			if response.Code != test.status {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestInternalTransferEndpointsRequireAdministrator(t *testing.T) {
	identities := administratorIdentityStub()
	identities.principal.Role = identity.Role("viewer")
	parties := &partyStub{}
	transfers := &internalTransferStub{}
	handler, err := New(readinessStub{}, identities, parties, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, transfers)
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []*http.Request{
		authenticatedRequest(http.MethodGet, "/internal-transfers", nil),
		authenticatedRequest(http.MethodPost, "/internal-transfers", url.Values{"amount": {"1"}}),
		authenticatedRequest(http.MethodGet, "/internal-transfers/55555555-5555-4555-8555-555555555555", nil),
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("%s %s status = %d", request.Method, request.URL.Path, response.Code)
		}
	}
	if parties.listCalls != 0 || transfers.createCalls != 0 || transfers.listCalls != 0 || transfers.getCalls != 0 {
		t.Fatalf("use cases called parties=%d create=%d list=%d get=%d", parties.listCalls, transfers.createCalls, transfers.listCalls, transfers.getCalls)
	}
}

func TestInternalTransferListFailuresAreNotRenderedAsEmpty(t *testing.T) {
	identities := administratorIdentityStub()
	for _, test := range []struct {
		name      string
		parties   *partyStub
		transfers *internalTransferStub
	}{
		{name: "parties", parties: &partyStub{listErr: errors.New("party database unavailable")}, transfers: &internalTransferStub{}},
		{name: "transfers", parties: &partyStub{listResult: obligationParties()[:1]}, transfers: &internalTransferStub{listErr: errors.New("transfer database unavailable")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler, err := New(readinessStub{}, identities, test.parties, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, test.transfers)
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/internal-transfers", nil))
			if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "尚未登记内部转账") {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestSafeReturnPath(t *testing.T) {
	tests := map[string]string{
		"":                     "/",
		"/":                    "/",
		"/items?status=open":   "/items?status=open",
		"https://evil.example": "/",
		"//evil.example/path":  "/",
		`/\evil.example/path`:  "/",
		"/login":               "/",
	}
	for raw, want := range tests {
		if got := safeReturnPath(raw); got != want {
			t.Errorf("safeReturnPath(%q) = %q, want %q", raw, got, want)
		}
	}
}

func formRequest(method, path string, values url.Values) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}

func authenticatedRequest(method, path string, values url.Values) *http.Request {
	var request *http.Request
	if values == nil {
		request = httptest.NewRequest(method, path, nil)
	} else {
		request = formRequest(method, path, values)
	}
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "valid-session"})
	return request
}

func administratorIdentityStub() *identityStub {
	return &identityStub{
		found: true,
		principal: identity.Principal{
			UserID: "11111111-1111-4111-8111-111111111111", Username: "admin", DisplayName: "资金管理员", Role: identity.RoleAdministrator,
		},
	}
}

func obligationParties() []party.Party {
	return []party.Party{
		{ID: "22222222-2222-4222-8222-222222222222", Kind: party.KindOrganization, LegalName: "Owner Company", RegistrationIdentifier: "OWNER-001", Revision: 1},
		{ID: "33333333-3333-4333-8333-333333333333", Kind: party.KindIndividual, LegalName: "Counter Person", Revision: 1},
	}
}

func sampleObligation() obligation.Obligation {
	return obligation.Obligation{
		ID:                 "55555555-5555-4555-8555-555555555555",
		Direction:          obligation.DirectionReceivable,
		Title:              "九月服务费",
		LedgerOwnerPartyID: "22222222-2222-4222-8222-222222222222",
		Creditor: obligation.PartySnapshot{
			PartyID: "22222222-2222-4222-8222-222222222222", PartyRevision: 1,
			Kind: party.KindOrganization, LegalName: "Owner Company", RegistrationIdentifier: "OWNER-001",
		},
		Debtor: obligation.PartySnapshot{
			PartyID: "33333333-3333-4333-8333-333333333333", PartyRevision: 1,
			Kind: party.KindIndividual, LegalName: "Counter Person",
		},
		OriginalAmount: "100.5", EffectiveAmount: "100.5", Currency: "CNY", DueOn: "2026-10-31",
		AllocatedAmount: "0", RemittedAmount: "0", OutstandingAmount: "100.5",
		SourceKind: obligation.SourceKindManual, ManualSourceReference: "人工确认单 OBL-001",
		Status: obligation.StatusDraft, DraftVersion: 1,
	}
}

func sampleOpenObligation() obligation.Obligation {
	value := sampleObligation()
	value.Status = obligation.StatusOpen
	value.CurrentRevision = 1
	return value
}

func sampleClosableObligation() obligation.Obligation {
	value := sampleOpenObligation()
	value.AllocatedAmount = "60.5"
	value.RemittedAmount = "40"
	value.OutstandingAmount = "0"
	value.BalanceSequence = 3
	return value
}

func sampleObligationClosure(value obligation.Obligation) obligation.Closure {
	return obligation.Closure{
		ID:                      "99999999-9999-4999-8999-999999999999",
		ObligationID:            value.ID,
		ObligationRevision:      value.CurrentRevision,
		BalanceSequence:         value.BalanceSequence,
		RecordedEffectiveAmount: value.EffectiveAmount,
		NetAllocatedAmount:      value.AllocatedAmount,
		RemittedAmount:          value.RemittedAmount,
		Currency:                value.Currency,
		SettlementBasis:         obligation.ClosureBasisMixed,
		SourceKind:              obligation.SourceKindManual,
		Reason:                  "余额事实已由双方复核",
		ClosedBy: audit.ActorRef{
			Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "资金管理员",
		},
		ClosedAt: time.Date(2026, 9, 19, 23, 0, 0, 0, time.UTC),
	}
}

func sampleCashTransaction() cashtransaction.Transaction {
	return cashtransaction.Transaction{
		ID:                 "55555555-5555-4555-8555-555555555555",
		Type:               cashtransaction.TypeReceipt,
		Description:        "九月服务费收款",
		LedgerOwnerPartyID: "22222222-2222-4222-8222-222222222222",
		Payer: cashtransaction.PartySnapshot{
			PartyID: "33333333-3333-4333-8333-333333333333", PartyRevision: 1,
			Kind: party.KindIndividual, LegalName: "Counter Person",
		},
		Payee: cashtransaction.PartySnapshot{
			PartyID: "22222222-2222-4222-8222-222222222222", PartyRevision: 1,
			Kind: party.KindOrganization, LegalName: "Owner Company", RegistrationIdentifier: "OWNER-001",
		},
		Amount: "100.5", EffectiveAmount: "100.5", AllocatedAmount: "0", UnallocatedAmount: "100.5",
		Currency: "CNY", OccurredOn: "2026-09-19",
		AccountReference: "基本户尾号 1234", SourceKind: cashtransaction.SourceKindManual,
		ManualSourceReference: "银行回单截图 CASH-001", ExternalTransactionReference: "BANK-TXN-001",
		Status:     cashtransaction.StatusRecorded,
		RecordedBy: audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "资金管理员"},
		RecordedAt: time.Date(2026, 9, 19, 16, 0, 0, 0, time.UTC),
	}
}

func sampleReversedCashTransaction() cashtransaction.Transaction {
	value := sampleCashTransaction()
	value.Status = cashtransaction.StatusReversed
	value.EffectiveAmount = "0"
	value.AllocatedAmount = "0"
	value.UnallocatedAmount = "0"
	value.Reversal = &cashtransaction.Reversal{
		ID:                "88888888-8888-4888-8888-888888888888",
		CashTransactionID: value.ID,
		TransactionType:   value.Type,
		Amount:            value.Amount,
		Currency:          value.Currency,
		SourceKind:        cashtransaction.SourceKindManual,
		Reason:            "原收款重复录入",
		ReversedBy:        audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "资金管理员"},
		ReversedAt:        time.Date(2026, 9, 19, 21, 0, 0, 0, time.UTC),
	}
	return value
}

func sampleAllocation() allocation.Allocation {
	return allocation.Allocation{
		ID:                "77777777-7777-4777-8777-777777777777",
		CashTransactionID: sampleCashTransaction().ID, CashDescription: "九月服务费收款", CashType: "receipt",
		ObligationID: sampleOpenObligation().ID, ObligationRevision: 1,
		ObligationTitle: "九月服务费", ObligationDirection: "receivable",
		LedgerOwnerPartyID: "22222222-2222-4222-8222-222222222222", LedgerOwnerLegalName: "Owner Company",
		CounterpartyPartyID: "33333333-3333-4333-8333-333333333333", CounterpartyLegalName: "Counter Person",
		Amount: "60.5", Currency: "CNY", SourceKind: allocation.SourceKindManual,
		ManualSourceReference: "人工核对资金和义务", Status: allocation.StatusActive,
		AppliedBy: audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "资金管理员"},
		AppliedAt: time.Date(2026, 9, 19, 18, 0, 0, 0, time.UTC),
	}
}

func sampleReversedAllocation() allocation.Allocation {
	value := sampleAllocation()
	value.Status = allocation.StatusReversed
	value.Reversal = &allocation.Reversal{
		ID:                 "88888888-8888-4888-8888-888888888888",
		AllocationID:       value.ID,
		CashTransactionID:  value.CashTransactionID,
		ObligationID:       value.ObligationID,
		ObligationRevision: value.ObligationRevision,
		Amount:             value.Amount,
		Currency:           value.Currency,
		SourceKind:         allocation.SourceKindManual,
		Reason:             "错误关联到了九月义务",
		ReversedBy:         audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "资金管理员"},
		ReversedAt:         time.Date(2026, 9, 19, 19, 0, 0, 0, time.UTC),
	}
	return value
}

func sampleInternalTransfer() internaltransfer.Transfer {
	return internaltransfer.Transfer{
		ID: "55555555-5555-4555-8555-555555555555", Description: "基本户向备用金账户转账",
		LedgerOwner: internaltransfer.PartySnapshot{
			PartyID: "22222222-2222-4222-8222-222222222222", PartyRevision: 1,
			Kind: party.KindOrganization, LegalName: "Owner Company", RegistrationIdentifier: "OWNER-001",
		},
		Amount: "100.5", Currency: "CNY", OccurredOn: "2026-09-19",
		SourceAccountReference: "基本户尾号 1234", DestinationAccountReference: "备用金账户",
		SourceKind: internaltransfer.SourceKindManual, ManualSourceReference: "银行回单 TRANSFER-001",
		ExternalTransferReference: "BANK-TRANSFER-001", Status: internaltransfer.StatusRecorded,
		RecordedBy: audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "资金管理员"},
		RecordedAt: time.Date(2026, 9, 19, 16, 0, 0, 0, time.UTC),
	}
}

func csrfToken() string {
	return "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
}

func findCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("cookie %q not found in %#v", name, cookies)
	return nil
}
