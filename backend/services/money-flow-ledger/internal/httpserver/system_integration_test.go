//go:build integration

package httpserver_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/allocation"
	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/cashtransaction"
	"omsbizfoundry/money-flow-ledger/internal/database"
	"omsbizfoundry/money-flow-ledger/internal/httpserver"
	"omsbizfoundry/money-flow-ledger/internal/identity"
	"omsbizfoundry/money-flow-ledger/internal/internaltransfer"
	"omsbizfoundry/money-flow-ledger/internal/ledgerexport"
	"omsbizfoundry/money-flow-ledger/internal/obligation"
	"omsbizfoundry/money-flow-ledger/internal/operatingsummary"
	"omsbizfoundry/money-flow-ledger/internal/party"
	"omsbizfoundry/money-flow-ledger/internal/testdatabase"
)

var csrfFieldPattern = regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)
var commandFieldPattern = regexp.MustCompile(`name="command_id" value="([^"]+)"`)
var balanceSequenceFieldPattern = regexp.MustCompile(`name="expected_balance_sequence" value="([0-9]+)"`)

func TestMoneyFlowAdministratorPartyAndObligationFlowAgainstPostgreSQL(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	identityService, err := identity.NewService(identity.NewPostgresRepository(environment.Pool), time.Hour)
	if err != nil {
		t.Fatalf("identity.NewService() error = %v", err)
	}
	created, err := identityService.EnsureBootstrap(ctx, identity.BootstrapInput{
		Username: "system-admin", DisplayName: "系统联调管理员", Password: "system-admin-password",
	})
	if err != nil || !created {
		t.Fatalf("EnsureBootstrap() = %v, %v", created, err)
	}
	partyService := party.NewService(party.NewPostgresRepository(environment.Pool))
	obligationService := obligation.NewService(obligation.NewPostgresRepository(environment.Pool))
	cashTransactionService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	allocationService := allocation.NewService(allocation.NewPostgresRepository(environment.Pool))
	operatingSummaryService := operatingsummary.NewService(operatingsummary.NewPostgresRepository(environment.Pool))
	ledgerExportService, err := ledgerexport.NewService(ledgerexport.NewPostgresRepository(environment.Pool))
	if err != nil {
		t.Fatalf("ledgerexport.NewService() error = %v", err)
	}
	internalTransferService := internaltransfer.NewService(internaltransfer.NewPostgresRepository(environment.Pool))
	handler, err := httpserver.New(environment.Pool, identityService, partyService, obligationService, cashTransactionService, allocationService, operatingSummaryService, ledgerExportService, httpserver.Options{}, internalTransferService)
	if err != nil {
		t.Fatalf("httpserver.New() error = %v", err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error = %v", err)
	}
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	response, err := client.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET protected home: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/login?next=%2F" {
		t.Fatalf("protected home status=%d location=%q", response.StatusCode, response.Header.Get("Location"))
	}
	closeResponse(t, response)

	loginPage := getPage(t, client, server.URL+"/login")
	csrf := hiddenCSRF(t, loginPage)
	response, err = client.PostForm(server.URL+"/login", url.Values{
		"csrf_token": {csrf},
		"next":       {"/"},
		"username":   {"system-admin"},
		"password":   {"system-admin-password"},
	})
	if err != nil {
		t.Fatalf("POST login: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/" {
		t.Fatalf("login status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	homePage := getPage(t, client, server.URL+"/")
	if !strings.Contains(homePage, "系统联调管理员") || !strings.Contains(homePage, "administrator") || !strings.Contains(homePage, "注销") {
		t.Fatalf("authenticated home = %q", homePage)
	}
	partyPage := getPage(t, client, server.URL+"/parties")
	partyCSRF := hiddenCSRF(t, partyPage)
	partyCommand := hiddenCommand(t, partyPage)
	createValues := url.Values{
		"csrf_token":              {partyCSRF},
		"command_id":              {partyCommand},
		"kind":                    {"organization"},
		"legal_name":              {"  系统联调供应商  "},
		"registration_identifier": {"  M3-SYSTEM-001  "},
		"actor_kind":              {"forged"},
		"actor_id":                {"forged"},
		"actor_display_name":      {"伪造用户"},
	}
	response, err = client.PostForm(server.URL+"/parties", createValues)
	if err != nil {
		t.Fatalf("POST party: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("party create status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	location, err := url.Parse(response.Header.Get("Location"))
	if err != nil || location.Path != "/parties" || location.Query().Get("created") != "1" || location.Query().Get("party_id") == "" {
		t.Fatalf("party create location=%q error=%v", response.Header.Get("Location"), err)
	}
	createdPartyID := location.Query().Get("party_id")
	closeResponse(t, response)

	response, err = client.PostForm(server.URL+"/parties", createValues)
	if err != nil {
		t.Fatalf("POST party replay: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || !strings.Contains(response.Header.Get("Location"), "party_id="+createdPartyID) {
		t.Fatalf("party replay status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	changedValues := url.Values{}
	for key, values := range createValues {
		changedValues[key] = append([]string(nil), values...)
	}
	changedValues.Set("legal_name", "改变后的联调供应商")
	response, err = client.PostForm(server.URL+"/parties", changedValues)
	if err != nil {
		t.Fatalf("POST changed party replay: %v", err)
	}
	changedBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(changedBody, "不同的主体登记意图") {
		t.Fatalf("changed replay status=%d body=%q", response.StatusCode, changedBody)
	}
	closeResponse(t, response)

	partyPage = getPage(t, client, server.URL+"/parties?created=1&party_id="+url.QueryEscape(createdPartyID))
	if !strings.Contains(partyPage, "系统联调供应商") || !strings.Contains(partyPage, "M3-SYSTEM-001") ||
		!strings.Contains(partyPage, "主体已登记") {
		t.Fatalf("party page = %q", partyPage)
	}
	var users, sessions, audits int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM local_users),
			(SELECT count(*) FROM local_sessions WHERE revoked_at IS NULL),
			(SELECT count(*) FROM audit_entries WHERE action = 'local_user_bootstrapped')
	`).Scan(&users, &sessions, &audits); err != nil {
		t.Fatalf("read login facts: %v", err)
	}
	if users != 1 || sessions != 1 || audits != 1 {
		t.Fatalf("login facts users=%d sessions=%d audits=%d", users, sessions, audits)
	}
	var storedName, storedIdentifier, actorKind, actorID, actorName string
	var revision, partyCount, commandCount, partyAuditCount int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			p.legal_name,
			COALESCE(p.registration_identifier, ''),
			p.revision,
			a.actor_kind,
			a.actor_id,
			a.actor_display_name,
			(SELECT count(*) FROM parties),
			(SELECT count(*) FROM party_create_commands),
			(SELECT count(*) FROM audit_entries WHERE action = 'party_created')
		FROM parties p
		JOIN audit_entries a ON a.object_type = 'party' AND a.object_id = p.id
		WHERE p.id = $1 AND a.command_id = $2
	`, createdPartyID, partyCommand).Scan(
		&storedName, &storedIdentifier, &revision, &actorKind, &actorID, &actorName,
		&partyCount, &commandCount, &partyAuditCount,
	); err != nil {
		t.Fatalf("read party HTTP facts: %v", err)
	}
	var actualUserID string
	if err := environment.Pool.QueryRow(ctx, `SELECT id::text FROM local_users WHERE username = 'system-admin'`).Scan(&actualUserID); err != nil {
		t.Fatalf("read system user ID: %v", err)
	}
	if storedName != "系统联调供应商" || storedIdentifier != "M3-SYSTEM-001" || revision != 1 ||
		actorKind != "local_user" || actorID != actualUserID || actorName != "系统联调管理员" ||
		partyCount != 1 || commandCount != 1 || partyAuditCount != 1 {
		t.Fatalf("party HTTP facts name=%q identifier=%q revision=%d actor=%s/%s/%s counts=%d/%d/%d",
			storedName, storedIdentifier, revision, actorKind, actorID, actorName, partyCount, commandCount, partyAuditCount)
	}

	partyPage = getPage(t, client, server.URL+"/parties")
	secondPartyCommand := hiddenCommand(t, partyPage)
	response, err = client.PostForm(server.URL+"/parties", url.Values{
		"csrf_token":              {hiddenCSRF(t, partyPage)},
		"command_id":              {secondPartyCommand},
		"kind":                    {"individual"},
		"legal_name":              {"系统联调客户"},
		"registration_identifier": {""},
	})
	if err != nil {
		t.Fatalf("POST second party: %v", err)
	}
	location, err = url.Parse(response.Header.Get("Location"))
	if response.StatusCode != http.StatusSeeOther || err != nil || location.Query().Get("party_id") == "" {
		t.Fatalf("second party status=%d location=%q error=%v", response.StatusCode, response.Header.Get("Location"), err)
	}
	secondPartyID := location.Query().Get("party_id")
	closeResponse(t, response)

	obligationPage := getPage(t, client, server.URL+"/obligations")
	obligationCSRF := hiddenCSRF(t, obligationPage)
	obligationCommand := hiddenCommand(t, obligationPage)
	obligationValues := url.Values{
		"csrf_token":              {obligationCSRF},
		"command_id":              {obligationCommand},
		"direction":               {"receivable"},
		"ledger_owner_party_id":   {createdPartyID},
		"counterparty_party_id":   {secondPartyID},
		"title":                   {"  系统联调九月服务费  "},
		"original_amount":         {"000100.500000"},
		"currency":                {" cny "},
		"due_on":                  {"2026-10-31"},
		"manual_source_reference": {"  人工确认单 HTTP-OBL-001  "},
		"actor_kind":              {"forged"},
		"actor_id":                {"forged"},
		"actor_display_name":      {"伪造用户"},
	}
	response, err = client.PostForm(server.URL+"/obligations", obligationValues)
	if err != nil {
		t.Fatalf("POST obligation draft: %v", err)
	}
	location, err = url.Parse(response.Header.Get("Location"))
	if response.StatusCode != http.StatusSeeOther || err != nil || !strings.HasPrefix(location.Path, "/obligations/") || location.Query().Get("created") != "1" {
		t.Fatalf("obligation create status=%d location=%q error=%v body=%q", response.StatusCode, response.Header.Get("Location"), err, readResponse(t, response))
	}
	createdObligationID := strings.TrimPrefix(location.Path, "/obligations/")
	if createdObligationID == "" {
		t.Fatal("created obligation ID is empty")
	}
	closeResponse(t, response)

	response, err = client.PostForm(server.URL+"/obligations", obligationValues)
	if err != nil {
		t.Fatalf("POST obligation replay: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || !strings.Contains(response.Header.Get("Location"), createdObligationID) {
		t.Fatalf("obligation replay status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	changedObligationValues := url.Values{}
	for key, values := range obligationValues {
		changedObligationValues[key] = append([]string(nil), values...)
	}
	changedObligationValues.Set("original_amount", "101")
	response, err = client.PostForm(server.URL+"/obligations", changedObligationValues)
	if err != nil {
		t.Fatalf("POST changed obligation replay: %v", err)
	}
	changedObligationBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(changedObligationBody, "不同的义务草稿创建意图") {
		t.Fatalf("changed obligation status=%d body=%q", response.StatusCode, changedObligationBody)
	}
	closeResponse(t, response)

	obligationDetail := getPage(t, client, server.URL+"/obligations/"+createdObligationID+"?created=1")
	if !strings.Contains(obligationDetail, "义务草稿已建立") || !strings.Contains(obligationDetail, "系统联调九月服务费") ||
		!strings.Contains(obligationDetail, "100.5 CNY") || !strings.Contains(obligationDetail, "系统联调供应商") ||
		!strings.Contains(obligationDetail, "系统联调客户") || !strings.Contains(obligationDetail, "人工确认单 HTTP-OBL-001") {
		t.Fatalf("obligation detail = %q", obligationDetail)
	}
	obligationPage = getPage(t, client, server.URL+"/obligations")
	if !strings.Contains(obligationPage, "系统联调九月服务费") || !strings.Contains(obligationPage, "100.5 CNY") {
		t.Fatalf("obligation list = %q", obligationPage)
	}

	var direction, title, ledgerOwnerID, creditorName, debtorName, amount, currency, dueOn, sourceKind, sourceReference, obligationStatus string
	var creditorRevision, debtorRevision, draftVersion, obligationCount, obligationCommandCount, obligationAuditCount int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			o.direction,
			o.title,
			o.ledger_owner_party_id::text,
			o.creditor_legal_name,
			o.creditor_party_revision,
			o.debtor_legal_name,
			o.debtor_party_revision,
			o.original_amount::text,
			o.currency,
			o.due_on::text,
			o.source_kind,
			o.manual_source_reference,
			o.status,
			o.draft_version,
			a.actor_kind,
			a.actor_id,
			a.actor_display_name,
			(SELECT count(*) FROM obligations),
			(SELECT count(*) FROM obligation_create_commands),
			(SELECT count(*) FROM audit_entries WHERE action = 'obligation_draft_created')
		FROM obligations o
		JOIN audit_entries a ON a.object_type = 'obligation' AND a.object_id = o.id
		WHERE o.id = $1 AND a.command_id = $2
	`, createdObligationID, obligationCommand).Scan(
		&direction, &title, &ledgerOwnerID,
		&creditorName, &creditorRevision, &debtorName, &debtorRevision,
		&amount, &currency, &dueOn, &sourceKind, &sourceReference, &obligationStatus, &draftVersion,
		&actorKind, &actorID, &actorName,
		&obligationCount, &obligationCommandCount, &obligationAuditCount,
	); err != nil {
		t.Fatalf("read obligation HTTP facts: %v", err)
	}
	if direction != "receivable" || title != "系统联调九月服务费" || ledgerOwnerID != createdPartyID ||
		creditorName != "系统联调供应商" || creditorRevision != 1 || debtorName != "系统联调客户" || debtorRevision != 1 ||
		amount != "100.500000" || currency != "CNY" || dueOn != "2026-10-31" || sourceKind != "manual" ||
		sourceReference != "人工确认单 HTTP-OBL-001" || obligationStatus != "draft" || draftVersion != 1 ||
		actorKind != "local_user" || actorID != actualUserID || actorName != "系统联调管理员" ||
		obligationCount != 1 || obligationCommandCount != 1 || obligationAuditCount != 1 {
		t.Fatalf("obligation HTTP facts direction=%q title=%q owner=%q parties=%q/%d,%q/%d amount=%q %s due=%s source=%s/%q status=%s/%d actor=%s/%s/%s counts=%d/%d/%d",
			direction, title, ledgerOwnerID, creditorName, creditorRevision, debtorName, debtorRevision,
			amount, currency, dueOn, sourceKind, sourceReference, obligationStatus, draftVersion,
			actorKind, actorID, actorName, obligationCount, obligationCommandCount, obligationAuditCount)
	}

	confirmationCommand := hiddenCommand(t, obligationDetail)
	confirmationValues := url.Values{
		"csrf_token":             {hiddenCSRF(t, obligationDetail)},
		"command_id":             {confirmationCommand},
		"expected_draft_version": {"1"},
		"actor_kind":             {"forged"},
		"actor_id":               {"forged"},
		"actor_display_name":     {"伪造确认人"},
	}
	response, err = client.PostForm(server.URL+"/obligations/"+createdObligationID+"/confirm", confirmationValues)
	if err != nil {
		t.Fatalf("POST obligation confirmation: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/obligations/"+createdObligationID+"?confirmed=1" {
		t.Fatalf("obligation confirmation status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	response, err = client.PostForm(server.URL+"/obligations/"+createdObligationID+"/confirm", confirmationValues)
	if err != nil {
		t.Fatalf("POST obligation confirmation replay: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/obligations/"+createdObligationID+"?confirmed=1" {
		t.Fatalf("obligation confirmation replay status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	confirmedDetail := getPage(t, client, server.URL+"/obligations/"+createdObligationID+"?confirmed=1")
	if !strings.Contains(confirmedDetail, "义务已确认并形成 revision 1") ||
		!strings.Contains(confirmedDetail, "义务 revision 历史") ||
		!strings.Contains(confirmedDetail, "系统联调管理员") ||
		strings.Contains(confirmedDetail, `action="/obligations/`+createdObligationID+`/confirm"`) {
		t.Fatalf("confirmed obligation detail = %q", confirmedDetail)
	}
	obligationPage = getPage(t, client, server.URL+"/obligations")
	if !strings.Contains(obligationPage, "open / draft version 1 / current revision 1") {
		t.Fatalf("confirmed obligation list = %q", obligationPage)
	}

	cashForecastPage := getPage(t, client, server.URL+"/cash-forecast")
	if !strings.Contains(cashForecastPage, "当前预计现金排期") ||
		!strings.Contains(cashForecastPage, "系统联调九月服务费") ||
		!strings.Contains(cashForecastPage, "100.5 CNY") ||
		!strings.Contains(cashForecastPage, "未排期") ||
		strings.Contains(cashForecastPage, "2099-10-20</td>") {
		t.Fatalf("initial cash forecast page = %q", cashForecastPage)
	}
	cashExpectationValues := url.Values{
		"csrf_token":           {hiddenCSRF(t, cashForecastPage)},
		"command_id":           {hiddenCommand(t, cashForecastPage)},
		"action":               {"set"},
		"obligation_selection": {createdObligationID + ":1"},
		"expected_on":          {"2099-10-20"},
		"expectation_reason":   {"客户确认预计回款日"},
		"actor_kind":           {"forged"},
		"actor_id":             {"forged"},
	}
	response, err = client.PostForm(server.URL+"/cash-expectations", cashExpectationValues)
	if err != nil {
		t.Fatalf("POST cash expectation: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || !strings.Contains(response.Header.Get("Location"), "recorded=1") ||
		!strings.Contains(response.Header.Get("Location"), createdObligationID) {
		t.Fatalf("cash expectation status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)
	response, err = client.PostForm(server.URL+"/cash-expectations", cashExpectationValues)
	if err != nil {
		t.Fatalf("POST cash expectation replay: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || !strings.Contains(response.Header.Get("Location"), "recorded=1") {
		t.Fatalf("cash expectation replay status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)
	cashForecastPage = getPage(t, client, server.URL+"/cash-forecast?history_obligation_id="+createdObligationID)
	if !strings.Contains(cashForecastPage, "2099-10-20") || !strings.Contains(cashForecastPage, "100.5 CNY") ||
		!strings.Contains(cashForecastPage, "客户确认预计回款日") || !strings.Contains(cashForecastPage, "seq 1") ||
		!strings.Contains(cashForecastPage, "系统联调管理员") {
		t.Fatalf("scheduled cash forecast page = %q", cashForecastPage)
	}

	var currentRevision, revisionNumber, sourceDraftVersion int
	var revisionDirection, revisionTitle, revisionCreditorName, revisionDebtorName, revisionAmount, revisionCurrency, revisionDueOn, revisionSourceKind, revisionSourceReference string
	var confirmedActorKind, confirmedActorID, confirmedActorName string
	var revisionCount, confirmationCommandCount, confirmationAuditCount int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			o.status,
			o.current_revision,
			r.revision,
			r.source_draft_version,
			r.direction,
			r.title,
			r.creditor_legal_name,
			r.debtor_legal_name,
			r.effective_amount::text,
			r.currency,
			r.due_on::text,
			r.source_kind,
			r.manual_source_reference,
			r.recorded_by_actor_kind,
			r.recorded_by_actor_id,
			r.recorded_by_actor_display_name,
			(SELECT count(*) FROM obligation_revisions WHERE obligation_id = o.id),
			(SELECT count(*) FROM obligation_confirmation_commands WHERE obligation_id = o.id),
			(SELECT count(*) FROM audit_entries WHERE object_type = 'obligation' AND object_id = o.id AND action = 'obligation_confirmed')
		FROM obligations o
		JOIN obligation_revisions r ON r.obligation_id = o.id AND r.revision = o.current_revision
		WHERE o.id = $1
	`, createdObligationID).Scan(
		&obligationStatus, &currentRevision, &revisionNumber, &sourceDraftVersion,
		&revisionDirection, &revisionTitle, &revisionCreditorName, &revisionDebtorName,
		&revisionAmount, &revisionCurrency, &revisionDueOn, &revisionSourceKind, &revisionSourceReference,
		&confirmedActorKind, &confirmedActorID, &confirmedActorName,
		&revisionCount, &confirmationCommandCount, &confirmationAuditCount,
	); err != nil {
		t.Fatalf("read obligation confirmation HTTP facts: %v", err)
	}
	if obligationStatus != "open" || currentRevision != 1 || revisionNumber != 1 || sourceDraftVersion != 1 ||
		revisionDirection != "receivable" || revisionTitle != "系统联调九月服务费" ||
		revisionCreditorName != "系统联调供应商" || revisionDebtorName != "系统联调客户" ||
		revisionAmount != "100.500000" || revisionCurrency != "CNY" || revisionDueOn != "2026-10-31" ||
		revisionSourceKind != "manual" || revisionSourceReference != "人工确认单 HTTP-OBL-001" ||
		confirmedActorKind != "local_user" || confirmedActorID != actualUserID || confirmedActorName != "系统联调管理员" ||
		revisionCount != 1 || confirmationCommandCount != 1 || confirmationAuditCount != 1 {
		t.Fatalf("confirmation HTTP facts status=%s current=%d revision=%d draft=%d direction=%s title=%q parties=%q/%q amount=%q %s due=%s source=%s/%q actor=%s/%s/%s counts=%d/%d/%d",
			obligationStatus, currentRevision, revisionNumber, sourceDraftVersion, revisionDirection, revisionTitle,
			revisionCreditorName, revisionDebtorName, revisionAmount, revisionCurrency, revisionDueOn,
			revisionSourceKind, revisionSourceReference, confirmedActorKind, confirmedActorID, confirmedActorName,
			revisionCount, confirmationCommandCount, confirmationAuditCount)
	}

	agingPage := getPage(t, client, server.URL+"/aging?as_of_date=2099-12-31")
	if !strings.Contains(agingPage, "当前 Obligation 逾期与账龄") ||
		!strings.Contains(agingPage, "系统联调九月服务费") ||
		!strings.Contains(agingPage, "100.5 CNY") ||
		!strings.Contains(agingPage, "逾期 91 天及以上") ||
		!strings.Contains(agingPage, "/obligations/"+createdObligationID) ||
		!strings.Contains(agingPage, "未来分类不是余额预测") {
		t.Fatalf("current aging page = %q", agingPage)
	}
	response, err = client.Get(server.URL + "/aging?as_of_date=2000-01-01")
	if err != nil {
		t.Fatalf("GET historical aging date: %v", err)
	}
	pastAgingBody := readResponse(t, response)
	if response.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(pastAgingBody, "历史时点余额需要独立重放能力") {
		t.Fatalf("historical aging status=%d body=%q", response.StatusCode, pastAgingBody)
	}
	closeResponse(t, response)

	cashPage := getPage(t, client, server.URL+"/cash-transactions")
	cashCSRF := hiddenCSRF(t, cashPage)
	cashCommand := hiddenCommand(t, cashPage)
	cashValues := url.Values{
		"csrf_token":                     {cashCSRF},
		"command_id":                     {cashCommand},
		"type":                           {"receipt"},
		"ledger_owner_party_id":          {createdPartyID},
		"counterparty_party_id":          {secondPartyID},
		"description":                    {"  系统联调实际收款  "},
		"amount":                         {"0002500.750000"},
		"currency":                       {" cny "},
		"occurred_on":                    {"2026-09-19"},
		"account_reference":              {" 系统联调基本户尾号 1234 "},
		"manual_source_reference":        {" 银行回单截图 HTTP-CASH-001 "},
		"external_transaction_reference": {" HTTP-BANK-TXN-001 "},
		"actor_kind":                     {"forged"},
		"actor_id":                       {"forged"},
		"actor_display_name":             {"伪造资金登记人"},
	}
	response, err = client.PostForm(server.URL+"/cash-transactions", cashValues)
	if err != nil {
		t.Fatalf("POST cash transaction: %v", err)
	}
	cashLocation, err := url.Parse(response.Header.Get("Location"))
	if response.StatusCode != http.StatusSeeOther || err != nil || !strings.HasPrefix(cashLocation.Path, "/cash-transactions/") || cashLocation.Query().Get("created") != "1" {
		t.Fatalf("cash create status=%d location=%q error=%v body=%q", response.StatusCode, response.Header.Get("Location"), err, readResponse(t, response))
	}
	createdCashTransactionID := strings.TrimPrefix(cashLocation.Path, "/cash-transactions/")
	if createdCashTransactionID == "" {
		t.Fatal("created cash transaction ID is empty")
	}
	closeResponse(t, response)

	response, err = client.PostForm(server.URL+"/cash-transactions", cashValues)
	if err != nil {
		t.Fatalf("POST cash transaction replay: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || !strings.Contains(response.Header.Get("Location"), createdCashTransactionID) {
		t.Fatalf("cash replay status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	changedCashValues := url.Values{}
	for key, values := range cashValues {
		changedCashValues[key] = append([]string(nil), values...)
	}
	changedCashValues.Set("amount", "2501")
	response, err = client.PostForm(server.URL+"/cash-transactions", changedCashValues)
	if err != nil {
		t.Fatalf("POST changed cash replay: %v", err)
	}
	changedCashBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(changedCashBody, "不同的实际收付款登记意图") {
		t.Fatalf("changed cash status=%d body=%q", response.StatusCode, changedCashBody)
	}
	closeResponse(t, response)

	duplicatePage := getPage(t, client, server.URL+"/cash-transactions")
	duplicateCashValues := url.Values{}
	for key, values := range cashValues {
		duplicateCashValues[key] = append([]string(nil), values...)
	}
	duplicateCashValues.Set("csrf_token", hiddenCSRF(t, duplicatePage))
	duplicateCashValues.Set("command_id", hiddenCommand(t, duplicatePage))
	duplicateCashValues.Set("description", "重复流水尝试")
	duplicateCashValues.Set("external_transaction_reference", "http-bank-txn-001")
	response, err = client.PostForm(server.URL+"/cash-transactions", duplicateCashValues)
	if err != nil {
		t.Fatalf("POST duplicate cash reference: %v", err)
	}
	duplicateCashBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(duplicateCashBody, "已存在相同的外部流水号") {
		t.Fatalf("duplicate cash status=%d body=%q", response.StatusCode, duplicateCashBody)
	}
	closeResponse(t, response)

	cashDetail := getPage(t, client, server.URL+"/cash-transactions/"+createdCashTransactionID+"?created=1")
	if !strings.Contains(cashDetail, "实际收付款已登记") || !strings.Contains(cashDetail, "系统联调实际收款") ||
		!strings.Contains(cashDetail, "2500.75 CNY") || !strings.Contains(cashDetail, "系统联调供应商") ||
		!strings.Contains(cashDetail, "系统联调客户") || !strings.Contains(cashDetail, "HTTP-BANK-TXN-001") ||
		!strings.Contains(cashDetail, "系统联调管理员") {
		t.Fatalf("cash detail = %q", cashDetail)
	}
	cashPage = getPage(t, client, server.URL+"/cash-transactions")
	if !strings.Contains(cashPage, "系统联调实际收款") || !strings.Contains(cashPage, "2500.75 CNY") ||
		!strings.Contains(cashPage, "recorded") {
		t.Fatalf("cash list = %q", cashPage)
	}

	var cashType, cashDescription, cashOwnerID, payerName, payeeName, cashAmount, cashCurrency, cashOccurredOn, accountReference, cashSourceKind, cashSourceReference, externalReference string
	var payerRevision, payeeRevision, cashCount, cashCommandCount, cashAuditCount int
	var cashActorKind, cashActorID, cashActorName string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			c.type,
			c.description,
			c.ledger_owner_party_id::text,
			c.payer_legal_name,
			c.payer_party_revision,
			c.payee_legal_name,
			c.payee_party_revision,
			c.amount::text,
			c.currency,
			c.occurred_on::text,
			c.account_reference,
			c.source_kind,
			c.manual_source_reference,
			c.external_transaction_reference,
			c.recorded_by_actor_kind,
			c.recorded_by_actor_id,
			c.recorded_by_actor_display_name,
			(SELECT count(*) FROM cash_transactions),
			(SELECT count(*) FROM cash_transaction_create_commands),
			(SELECT count(*) FROM audit_entries WHERE action = 'cash_transaction_recorded')
		FROM cash_transactions c
		WHERE c.id = $1
	`, createdCashTransactionID).Scan(
		&cashType, &cashDescription, &cashOwnerID,
		&payerName, &payerRevision, &payeeName, &payeeRevision,
		&cashAmount, &cashCurrency, &cashOccurredOn, &accountReference,
		&cashSourceKind, &cashSourceReference, &externalReference,
		&cashActorKind, &cashActorID, &cashActorName,
		&cashCount, &cashCommandCount, &cashAuditCount,
	); err != nil {
		t.Fatalf("read cash HTTP facts: %v", err)
	}
	if cashType != "receipt" || cashDescription != "系统联调实际收款" || cashOwnerID != createdPartyID ||
		payerName != "系统联调客户" || payerRevision != 1 || payeeName != "系统联调供应商" || payeeRevision != 1 ||
		cashAmount != "2500.750000" || cashCurrency != "CNY" || cashOccurredOn != "2026-09-19" ||
		accountReference != "系统联调基本户尾号 1234" || cashSourceKind != "manual" ||
		cashSourceReference != "银行回单截图 HTTP-CASH-001" || externalReference != "HTTP-BANK-TXN-001" ||
		cashActorKind != "local_user" || cashActorID != actualUserID || cashActorName != "系统联调管理员" ||
		cashCount != 1 || cashCommandCount != 1 || cashAuditCount != 1 {
		t.Fatalf("cash HTTP facts type=%s description=%q owner=%s parties=%q/%d,%q/%d amount=%q %s occurred=%s account=%q source=%s/%q external=%q actor=%s/%s/%s counts=%d/%d/%d",
			cashType, cashDescription, cashOwnerID, payerName, payerRevision, payeeName, payeeRevision,
			cashAmount, cashCurrency, cashOccurredOn, accountReference, cashSourceKind, cashSourceReference,
			externalReference, cashActorKind, cashActorID, cashActorName, cashCount, cashCommandCount, cashAuditCount)
	}

	allocationPage := getPage(t, client, server.URL+"/allocations")
	if !strings.Contains(allocationPage, "资金分配与余额") || !strings.Contains(allocationPage, "系统联调实际收款") ||
		!strings.Contains(allocationPage, "未分配 2500.75 CNY") || !strings.Contains(allocationPage, "系统联调九月服务费") ||
		!strings.Contains(allocationPage, "剩余 100.5 CNY") {
		t.Fatalf("allocation page = %q", allocationPage)
	}
	allocationCommand := hiddenCommand(t, allocationPage)
	allocationValues := url.Values{
		"csrf_token":                    {hiddenCSRF(t, allocationPage)},
		"command_id":                    {allocationCommand},
		"cash_transaction_id":           {createdCashTransactionID},
		"obligation_selection":          {createdObligationID + ":1"},
		"amount":                        {"000060.250000"},
		"manual_source_reference":       {" 人工核对回单 HTTP-ALLOC-001 "},
		"actor_kind":                    {"forged"},
		"actor_id":                      {"forged"},
		"actor_display_name":            {"伪造分配人"},
		"currency":                      {"USD"},
		"cash_unallocated_amount":       {"999999"},
		"obligation_outstanding_amount": {"999999"},
		"applied_at":                    {"2000-01-01T00:00:00Z"},
	}
	response, err = client.PostForm(server.URL+"/allocations", allocationValues)
	if err != nil {
		t.Fatalf("POST allocation: %v", err)
	}
	allocationLocation, err := url.Parse(response.Header.Get("Location"))
	if response.StatusCode != http.StatusSeeOther || err != nil || !strings.HasPrefix(allocationLocation.Path, "/allocations/") || allocationLocation.Query().Get("created") != "1" {
		t.Fatalf("allocation create status=%d location=%q error=%v body=%q", response.StatusCode, response.Header.Get("Location"), err, readResponse(t, response))
	}
	createdAllocationID := strings.TrimPrefix(allocationLocation.Path, "/allocations/")
	if createdAllocationID == "" {
		t.Fatal("created allocation ID is empty")
	}
	closeResponse(t, response)

	response, err = client.PostForm(server.URL+"/allocations", allocationValues)
	if err != nil {
		t.Fatalf("POST allocation replay: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || !strings.Contains(response.Header.Get("Location"), createdAllocationID) {
		t.Fatalf("allocation replay status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	changedAllocationValues := url.Values{}
	for key, values := range allocationValues {
		changedAllocationValues[key] = append([]string(nil), values...)
	}
	changedAllocationValues.Set("amount", "60.26")
	response, err = client.PostForm(server.URL+"/allocations", changedAllocationValues)
	if err != nil {
		t.Fatalf("POST changed allocation replay: %v", err)
	}
	changedAllocationBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(changedAllocationBody, "不同的资金分配意图") {
		t.Fatalf("changed allocation status=%d body=%q", response.StatusCode, changedAllocationBody)
	}
	closeResponse(t, response)

	overAllocationPage := getPage(t, client, server.URL+"/allocations")
	overAllocationValues := url.Values{
		"csrf_token":              {hiddenCSRF(t, overAllocationPage)},
		"command_id":              {hiddenCommand(t, overAllocationPage)},
		"cash_transaction_id":     {createdCashTransactionID},
		"obligation_selection":    {createdObligationID + ":1"},
		"amount":                  {"40.250001"},
		"manual_source_reference": {"超额尝试"},
	}
	response, err = client.PostForm(server.URL+"/allocations", overAllocationValues)
	if err != nil {
		t.Fatalf("POST excessive allocation: %v", err)
	}
	overAllocationBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(overAllocationBody, "超过该义务的剩余金额") {
		t.Fatalf("excessive allocation status=%d body=%q", response.StatusCode, overAllocationBody)
	}
	closeResponse(t, response)

	allocationDetail := getPage(t, client, server.URL+"/allocations/"+createdAllocationID+"?created=1")
	if !strings.Contains(allocationDetail, "资金分配已经生效") || !strings.Contains(allocationDetail, "系统联调九月服务费") ||
		!strings.Contains(allocationDetail, "系统联调实际收款") || !strings.Contains(allocationDetail, "60.25 CNY") ||
		!strings.Contains(allocationDetail, "人工核对回单 HTTP-ALLOC-001") || !strings.Contains(allocationDetail, "系统联调管理员") {
		t.Fatalf("allocation detail = %q", allocationDetail)
	}
	allocationPage = getPage(t, client, server.URL+"/allocations")
	if !strings.Contains(allocationPage, "60.25 CNY") || !strings.Contains(allocationPage, "人工核对回单 HTTP-ALLOC-001") ||
		!strings.Contains(allocationPage, "未分配 2440.5 CNY") || !strings.Contains(allocationPage, "剩余 40.25 CNY") {
		t.Fatalf("allocation list after apply = %q", allocationPage)
	}
	cashDetail = getPage(t, client, server.URL+"/cash-transactions/"+createdCashTransactionID)
	if !strings.Contains(cashDetail, "已分配金额</dt><dd>60.25 CNY") || !strings.Contains(cashDetail, "未分配金额</dt><dd>2440.5 CNY") {
		t.Fatalf("cash balance detail = %q", cashDetail)
	}
	confirmedDetail = getPage(t, client, server.URL+"/obligations/"+createdObligationID)
	if !strings.Contains(confirmedDetail, "已分配金额</dt><dd>60.25 CNY") || !strings.Contains(confirmedDetail, "剩余金额</dt><dd>40.25 CNY") {
		t.Fatalf("obligation balance detail = %q", confirmedDetail)
	}
	agingPage = getPage(t, client, server.URL+"/aging?as_of_date=2099-12-31")
	if !strings.Contains(agingPage, "系统联调九月服务费") || !strings.Contains(agingPage, "40.25 CNY") {
		t.Fatalf("aging after allocation = %q", agingPage)
	}
	cashForecastPage = getPage(t, client, server.URL+"/cash-forecast?history_obligation_id="+createdObligationID)
	if !strings.Contains(cashForecastPage, "2099-10-20") || !strings.Contains(cashForecastPage, "40.25 CNY") {
		t.Fatalf("cash forecast after allocation = %q", cashForecastPage)
	}
	clearExpectationValues := url.Values{
		"csrf_token":                {hiddenCSRF(t, cashForecastPage)},
		"command_id":                {"ca000000-0000-4000-8000-000000000001"},
		"action":                    {"clear"},
		"obligation_id":             {createdObligationID},
		"expected_current_revision": {"1"},
		"expectation_reason":        {"预计日期重新待定"},
	}
	response, err = client.PostForm(server.URL+"/cash-expectations", clearExpectationValues)
	if err != nil {
		t.Fatalf("POST clear cash expectation: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || !strings.Contains(response.Header.Get("Location"), "cleared=1") {
		t.Fatalf("clear cash expectation status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)
	cashForecastPage = getPage(t, client, server.URL+"/cash-forecast?history_obligation_id="+createdObligationID)
	if !strings.Contains(cashForecastPage, "当前未排期金额") || !strings.Contains(cashForecastPage, "40.25 CNY") ||
		!strings.Contains(cashForecastPage, "预计日期重新待定") || !strings.Contains(cashForecastPage, "清除排期") {
		t.Fatalf("cash forecast after clear = %q", cashForecastPage)
	}

	var allocationAmount, allocationCurrency, allocationSourceKind, allocationSourceReference string
	var allocationActorKind, allocationActorID, allocationActorName string
	var allocationCount, allocationCommandCount, allocationAuditCount int
	var allocatedCash, unallocatedCash, allocatedObligation, outstandingObligation string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			a.amount::text,
			a.currency,
			a.source_kind,
			a.manual_source_reference,
			a.applied_by_actor_kind,
			a.applied_by_actor_id,
			a.applied_by_actor_display_name,
			(SELECT count(*) FROM allocations),
			(SELECT count(*) FROM allocation_apply_commands),
			(SELECT count(*) FROM audit_entries WHERE action = 'allocation_applied'),
			(SELECT COALESCE(SUM(amount), 0)::text FROM allocations WHERE cash_transaction_id = $2),
			(SELECT (amount - COALESCE((SELECT SUM(amount) FROM allocations WHERE cash_transaction_id = c.id), 0))::text FROM cash_transactions c WHERE id = $2),
			(SELECT COALESCE(SUM(amount), 0)::text FROM allocations WHERE obligation_id = $3),
			(SELECT (r.effective_amount - COALESCE((SELECT SUM(amount) FROM allocations WHERE obligation_id = r.obligation_id), 0))::text FROM obligation_revisions r WHERE r.obligation_id = $3 AND r.revision = 1)
		FROM allocations a
		WHERE a.id = $1
	`, createdAllocationID, createdCashTransactionID, createdObligationID).Scan(
		&allocationAmount, &allocationCurrency, &allocationSourceKind, &allocationSourceReference,
		&allocationActorKind, &allocationActorID, &allocationActorName,
		&allocationCount, &allocationCommandCount, &allocationAuditCount,
		&allocatedCash, &unallocatedCash, &allocatedObligation, &outstandingObligation,
	); err != nil {
		t.Fatalf("read allocation HTTP facts: %v", err)
	}
	if allocationAmount != "60.250000" || allocationCurrency != "CNY" || allocationSourceKind != "manual" ||
		allocationSourceReference != "人工核对回单 HTTP-ALLOC-001" || allocationActorKind != "local_user" ||
		allocationActorID != actualUserID || allocationActorName != "系统联调管理员" ||
		allocationCount != 1 || allocationCommandCount != 1 || allocationAuditCount != 1 ||
		allocatedCash != "60.250000" || unallocatedCash != "2440.500000" ||
		allocatedObligation != "60.250000" || outstandingObligation != "40.250000" {
		t.Fatalf("allocation HTTP facts amount=%s %s source=%s/%q actor=%s/%s/%s counts=%d/%d/%d balances=%s/%s/%s/%s",
			allocationAmount, allocationCurrency, allocationSourceKind, allocationSourceReference,
			allocationActorKind, allocationActorID, allocationActorName,
			allocationCount, allocationCommandCount, allocationAuditCount,
			allocatedCash, unallocatedCash, allocatedObligation, outstandingObligation)
	}

	activeAllocationDetail := getPage(t, client, server.URL+"/allocations/"+createdAllocationID)
	blankReversalValues := url.Values{
		"csrf_token": {hiddenCSRF(t, activeAllocationDetail)},
		"command_id": {hiddenCommand(t, activeAllocationDetail)},
		"reason":     {"   "},
	}
	response, err = client.PostForm(server.URL+"/allocations/"+createdAllocationID+"/reverse", blankReversalValues)
	if err != nil {
		t.Fatalf("POST blank allocation reversal: %v", err)
	}
	blankReversalBody := readResponse(t, response)
	if response.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(blankReversalBody, "反向核销原因不能为空") {
		t.Fatalf("blank reversal status=%d body=%q", response.StatusCode, blankReversalBody)
	}
	closeResponse(t, response)

	reversalCommand := hiddenCommand(t, blankReversalBody)
	reversalValues := url.Values{
		"csrf_token":          {hiddenCSRF(t, blankReversalBody)},
		"command_id":          {reversalCommand},
		"reason":              {" 错误关联到九月服务费 "},
		"amount":              {"0.01"},
		"currency":            {"USD"},
		"cash_transaction_id": {"forged"},
		"obligation_id":       {"forged"},
		"obligation_revision": {"99"},
		"actor_kind":          {"forged"},
		"actor_id":            {"forged"},
		"actor_display_name":  {"伪造反向人"},
		"reversed_at":         {"2000-01-01T00:00:00Z"},
	}
	response, err = client.PostForm(server.URL+"/allocations/"+createdAllocationID+"/reverse", reversalValues)
	if err != nil {
		t.Fatalf("POST allocation reversal: %v", err)
	}
	reversalLocation, err := url.Parse(response.Header.Get("Location"))
	if response.StatusCode != http.StatusSeeOther || err != nil || reversalLocation.Path != "/allocations/"+createdAllocationID || reversalLocation.Query().Get("reversed") != "1" {
		t.Fatalf("reversal status=%d location=%q error=%v body=%q", response.StatusCode, response.Header.Get("Location"), err, readResponse(t, response))
	}
	closeResponse(t, response)

	response, err = client.PostForm(server.URL+"/allocations/"+createdAllocationID+"/reverse", reversalValues)
	if err != nil {
		t.Fatalf("POST allocation reversal replay: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/allocations/"+createdAllocationID+"?reversed=1" {
		t.Fatalf("reversal replay status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	changedReversalValues := url.Values{}
	for key, values := range reversalValues {
		changedReversalValues[key] = append([]string(nil), values...)
	}
	changedReversalValues.Set("reason", "改变后的反向原因")
	response, err = client.PostForm(server.URL+"/allocations/"+createdAllocationID+"/reverse", changedReversalValues)
	if err != nil {
		t.Fatalf("POST changed allocation reversal replay: %v", err)
	}
	changedReversalBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(changedReversalBody, "不同的反向核销意图") ||
		!strings.Contains(changedReversalBody, "错误关联到九月服务费") {
		t.Fatalf("changed reversal status=%d body=%q", response.StatusCode, changedReversalBody)
	}
	closeResponse(t, response)

	secondReversalValues := url.Values{}
	for key, values := range reversalValues {
		secondReversalValues[key] = append([]string(nil), values...)
	}
	secondReversalValues.Set("command_id", "99000000-0000-4000-8000-000000000001")
	response, err = client.PostForm(server.URL+"/allocations/"+createdAllocationID+"/reverse", secondReversalValues)
	if err != nil {
		t.Fatalf("POST second allocation reversal: %v", err)
	}
	secondReversalBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(secondReversalBody, "已经完成反向核销") {
		t.Fatalf("second reversal status=%d body=%q", response.StatusCode, secondReversalBody)
	}
	closeResponse(t, response)

	reversedDetail := getPage(t, client, server.URL+"/allocations/"+createdAllocationID+"?reversed=1")
	if !strings.Contains(reversedDetail, "相关余额已释放") || !strings.Contains(reversedDetail, "reversed") ||
		!strings.Contains(reversedDetail, "错误关联到九月服务费") || !strings.Contains(reversedDetail, "系统联调管理员") ||
		strings.Contains(reversedDetail, "确认完整反向核销") {
		t.Fatalf("reversed allocation detail = %q", reversedDetail)
	}
	allocationPage = getPage(t, client, server.URL+"/allocations")
	if !strings.Contains(allocationPage, "未分配 2500.75 CNY") || !strings.Contains(allocationPage, "剩余 100.5 CNY") ||
		!strings.Contains(allocationPage, "reversed") || !strings.Contains(allocationPage, "错误关联到九月服务费") {
		t.Fatalf("allocation list after reversal = %q", allocationPage)
	}
	cashDetail = getPage(t, client, server.URL+"/cash-transactions/"+createdCashTransactionID)
	if !strings.Contains(cashDetail, "已分配金额</dt><dd>0 CNY") || !strings.Contains(cashDetail, "未分配金额</dt><dd>2500.75 CNY") ||
		!strings.Contains(cashDetail, "reversed") || !strings.Contains(cashDetail, "错误关联到九月服务费") {
		t.Fatalf("cash detail after reversal = %q", cashDetail)
	}
	confirmedDetail = getPage(t, client, server.URL+"/obligations/"+createdObligationID)
	if !strings.Contains(confirmedDetail, "已分配金额</dt><dd>0 CNY") || !strings.Contains(confirmedDetail, "剩余金额</dt><dd>100.5 CNY") ||
		!strings.Contains(confirmedDetail, "reversed") || !strings.Contains(confirmedDetail, "错误关联到九月服务费") {
		t.Fatalf("obligation detail after reversal = %q", confirmedDetail)
	}
	agingPage = getPage(t, client, server.URL+"/aging?as_of_date=2099-12-31")
	if !strings.Contains(agingPage, "系统联调九月服务费") || !strings.Contains(agingPage, "100.5 CNY") {
		t.Fatalf("aging after allocation reversal = %q", agingPage)
	}

	replacementAllocationValues := url.Values{
		"csrf_token":              {hiddenCSRF(t, allocationPage)},
		"command_id":              {hiddenCommand(t, allocationPage)},
		"cash_transaction_id":     {createdCashTransactionID},
		"obligation_selection":    {createdObligationID + ":1"},
		"amount":                  {"60.25"},
		"manual_source_reference": {"纠正后的人工核对 HTTP-ALLOC-002"},
	}
	response, err = client.PostForm(server.URL+"/allocations", replacementAllocationValues)
	if err != nil {
		t.Fatalf("POST replacement allocation: %v", err)
	}
	replacementLocation, err := url.Parse(response.Header.Get("Location"))
	if response.StatusCode != http.StatusSeeOther || err != nil || !strings.HasPrefix(replacementLocation.Path, "/allocations/") {
		t.Fatalf("replacement allocation status=%d location=%q error=%v body=%q", response.StatusCode, response.Header.Get("Location"), err, readResponse(t, response))
	}
	replacementAllocationID := strings.TrimPrefix(replacementLocation.Path, "/allocations/")
	if replacementAllocationID == "" || replacementAllocationID == createdAllocationID {
		t.Fatalf("replacement allocation ID = %q, original = %q", replacementAllocationID, createdAllocationID)
	}
	closeResponse(t, response)

	var reversalID, reversalAllocationID, reversalCashID, reversalObligationID string
	var reversalRevision int64
	var reversalAmount, reversalCurrency, reversalReason, reversalActorKind, reversalActorID, reversalActorName string
	var reversalCount, reversalCommandCount, reversalAuditCount int
	var finalAllocationCount, finalApplyCommandCount, finalApplyAuditCount int
	var netCashAllocated, finalCashUnallocated, netObligationAllocated, finalObligationOutstanding string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			r.id::text,
			r.allocation_id::text,
			r.cash_transaction_id::text,
			r.obligation_id::text,
			r.obligation_revision,
			r.amount::text,
			r.currency,
			r.reversal_reason,
			r.reversed_by_actor_kind,
			r.reversed_by_actor_id,
			r.reversed_by_actor_display_name,
			(SELECT count(*) FROM allocation_reversals),
			(SELECT count(*) FROM allocation_reversal_create_commands),
			(SELECT count(*) FROM audit_entries WHERE action = 'allocation_reversed'),
			(SELECT count(*) FROM allocations),
			(SELECT count(*) FROM allocation_apply_commands),
			(SELECT count(*) FROM audit_entries WHERE action = 'allocation_applied'),
			(SELECT (COALESCE(SUM(a.amount), 0) - COALESCE(SUM(ar.amount), 0))::text
			 FROM allocations a LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
			 WHERE a.cash_transaction_id = $2),
			(SELECT (c.amount - (COALESCE(SUM(a.amount), 0) - COALESCE(SUM(ar.amount), 0)))::text
			 FROM cash_transactions c
			 LEFT JOIN allocations a ON a.cash_transaction_id = c.id
			 LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
			 WHERE c.id = $2 GROUP BY c.amount),
			(SELECT (COALESCE(SUM(a.amount), 0) - COALESCE(SUM(ar.amount), 0))::text
			 FROM allocations a LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
			 WHERE a.obligation_id = $3),
			(SELECT (orev.effective_amount - (COALESCE(SUM(a.amount), 0) - COALESCE(SUM(ar.amount), 0)))::text
			 FROM obligation_revisions orev
			 LEFT JOIN allocations a ON a.obligation_id = orev.obligation_id AND a.obligation_revision = orev.revision
			 LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
			 WHERE orev.obligation_id = $3 AND orev.revision = 1 GROUP BY orev.effective_amount)
		FROM allocation_reversals r
		WHERE r.allocation_id = $1
	`, createdAllocationID, createdCashTransactionID, createdObligationID).Scan(
		&reversalID, &reversalAllocationID, &reversalCashID, &reversalObligationID, &reversalRevision,
		&reversalAmount, &reversalCurrency, &reversalReason,
		&reversalActorKind, &reversalActorID, &reversalActorName,
		&reversalCount, &reversalCommandCount, &reversalAuditCount,
		&finalAllocationCount, &finalApplyCommandCount, &finalApplyAuditCount,
		&netCashAllocated, &finalCashUnallocated, &netObligationAllocated, &finalObligationOutstanding,
	); err != nil {
		t.Fatalf("read allocation reversal HTTP facts: %v", err)
	}
	if reversalID == "" || reversalAllocationID != createdAllocationID || reversalCashID != createdCashTransactionID ||
		reversalObligationID != createdObligationID || reversalRevision != 1 || reversalAmount != "60.250000" ||
		reversalCurrency != "CNY" || reversalReason != "错误关联到九月服务费" ||
		reversalActorKind != "local_user" || reversalActorID != actualUserID || reversalActorName != "系统联调管理员" ||
		reversalCount != 1 || reversalCommandCount != 1 || reversalAuditCount != 1 ||
		finalAllocationCount != 2 || finalApplyCommandCount != 2 || finalApplyAuditCount != 2 ||
		netCashAllocated != "60.250000" || finalCashUnallocated != "2440.500000" ||
		netObligationAllocated != "60.250000" || finalObligationOutstanding != "40.250000" {
		t.Fatalf("reversal HTTP facts id=%s allocation=%s cash=%s obligation=%s/%d amount=%s %s reason=%q actor=%s/%s/%s counts=%d/%d/%d apply=%d/%d/%d balances=%s/%s/%s/%s",
			reversalID, reversalAllocationID, reversalCashID, reversalObligationID, reversalRevision,
			reversalAmount, reversalCurrency, reversalReason, reversalActorKind, reversalActorID, reversalActorName,
			reversalCount, reversalCommandCount, reversalAuditCount,
			finalAllocationCount, finalApplyCommandCount, finalApplyAuditCount,
			netCashAllocated, finalCashUnallocated, netObligationAllocated, finalObligationOutstanding)
	}
	allocationPage = getPage(t, client, server.URL+"/allocations")
	if !strings.Contains(allocationPage, "纠正后的人工核对 HTTP-ALLOC-002") || !strings.Contains(allocationPage, "active") ||
		!strings.Contains(allocationPage, "reversed") || !strings.Contains(allocationPage, "未分配 2440.5 CNY") ||
		!strings.Contains(allocationPage, "剩余 40.25 CNY") {
		t.Fatalf("allocation list after replacement = %q", allocationPage)
	}

	cashDetail = getPage(t, client, server.URL+"/cash-transactions/"+createdCashTransactionID)
	if !strings.Contains(cashDetail, "已分配金额</dt><dd>60.25 CNY") ||
		!strings.Contains(cashDetail, "暂不能冲销") || !strings.Contains(cashDetail, replacementAllocationID) ||
		strings.Contains(cashDetail, "确认完整冲销") {
		t.Fatalf("cash detail before allocation prerequisite = %q", cashDetail)
	}
	blockedCashReversalValues := url.Values{
		"csrf_token": {hiddenCSRF(t, cashDetail)},
		"command_id": {"aa000000-0000-4000-8000-000000000001"},
		"reason":     {"原实际收款重复录入"},
	}
	response, err = client.PostForm(server.URL+"/cash-transactions/"+createdCashTransactionID+"/reverse", blockedCashReversalValues)
	if err != nil {
		t.Fatalf("POST cash reversal with active allocation: %v", err)
	}
	blockedCashReversalBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(blockedCashReversalBody, "仍有未反向的资金分配") ||
		!strings.Contains(blockedCashReversalBody, replacementAllocationID) {
		t.Fatalf("blocked cash reversal status=%d body=%q", response.StatusCode, blockedCashReversalBody)
	}
	closeResponse(t, response)

	replacementAllocationDetail := getPage(t, client, server.URL+"/allocations/"+replacementAllocationID)
	replacementAllocationReverseValues := url.Values{
		"csrf_token": {hiddenCSRF(t, replacementAllocationDetail)},
		"command_id": {hiddenCommand(t, replacementAllocationDetail)},
		"reason":     {"原资金事实错误，先释放核销"},
	}
	response, err = client.PostForm(server.URL+"/allocations/"+replacementAllocationID+"/reverse", replacementAllocationReverseValues)
	if err != nil {
		t.Fatalf("POST replacement allocation reversal: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/allocations/"+replacementAllocationID+"?reversed=1" {
		t.Fatalf("replacement allocation reversal status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	cashDetail = getPage(t, client, server.URL+"/cash-transactions/"+createdCashTransactionID)
	if !strings.Contains(cashDetail, "已分配金额</dt><dd>0 CNY") ||
		!strings.Contains(cashDetail, "未分配金额</dt><dd>2500.75 CNY") ||
		!strings.Contains(cashDetail, "原资金事实错误，先释放核销") ||
		!strings.Contains(cashDetail, "确认完整冲销") {
		t.Fatalf("cash detail after allocation prerequisite = %q", cashDetail)
	}
	blankCashReversalValues := url.Values{
		"csrf_token": {hiddenCSRF(t, cashDetail)},
		"command_id": {hiddenCommand(t, cashDetail)},
		"reason":     {"   "},
	}
	response, err = client.PostForm(server.URL+"/cash-transactions/"+createdCashTransactionID+"/reverse", blankCashReversalValues)
	if err != nil {
		t.Fatalf("POST blank cash reversal: %v", err)
	}
	blankCashReversalBody := readResponse(t, response)
	if response.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(blankCashReversalBody, "冲销原因不能为空") {
		t.Fatalf("blank cash reversal status=%d body=%q", response.StatusCode, blankCashReversalBody)
	}
	closeResponse(t, response)

	cashReversalCommand := hiddenCommand(t, blankCashReversalBody)
	cashReversalValues := url.Values{
		"csrf_token":         {hiddenCSRF(t, blankCashReversalBody)},
		"command_id":         {cashReversalCommand},
		"reason":             {" 原实际收款重复录入 "},
		"transaction_type":   {"payment"},
		"amount":             {"0.01"},
		"currency":           {"USD"},
		"actor_kind":         {"forged"},
		"actor_id":           {"forged"},
		"actor_display_name": {"伪造冲销人"},
		"reversed_at":        {"2000-01-01T00:00:00Z"},
	}
	response, err = client.PostForm(server.URL+"/cash-transactions/"+createdCashTransactionID+"/reverse", cashReversalValues)
	if err != nil {
		t.Fatalf("POST cash reversal: %v", err)
	}
	cashReversalLocation, err := url.Parse(response.Header.Get("Location"))
	if response.StatusCode != http.StatusSeeOther || err != nil || cashReversalLocation.Path != "/cash-transactions/"+createdCashTransactionID ||
		cashReversalLocation.Query().Get("reversed") != "1" {
		t.Fatalf("cash reversal status=%d location=%q error=%v body=%q", response.StatusCode, response.Header.Get("Location"), err, readResponse(t, response))
	}
	closeResponse(t, response)

	response, err = client.PostForm(server.URL+"/cash-transactions/"+createdCashTransactionID+"/reverse", cashReversalValues)
	if err != nil {
		t.Fatalf("POST cash reversal replay: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/cash-transactions/"+createdCashTransactionID+"?reversed=1" {
		t.Fatalf("cash reversal replay status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	changedCashReversalValues := url.Values{}
	for key, values := range cashReversalValues {
		changedCashReversalValues[key] = append([]string(nil), values...)
	}
	changedCashReversalValues.Set("reason", "改变后的资金冲销原因")
	response, err = client.PostForm(server.URL+"/cash-transactions/"+createdCashTransactionID+"/reverse", changedCashReversalValues)
	if err != nil {
		t.Fatalf("POST changed cash reversal replay: %v", err)
	}
	changedCashReversalBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(changedCashReversalBody, "不同的实际收付款冲销意图") ||
		!strings.Contains(changedCashReversalBody, "原实际收款重复录入") {
		t.Fatalf("changed cash reversal status=%d body=%q", response.StatusCode, changedCashReversalBody)
	}
	closeResponse(t, response)

	secondCashReversalValues := url.Values{}
	for key, values := range cashReversalValues {
		secondCashReversalValues[key] = append([]string(nil), values...)
	}
	secondCashReversalValues.Set("command_id", "aa000000-0000-4000-8000-000000000002")
	response, err = client.PostForm(server.URL+"/cash-transactions/"+createdCashTransactionID+"/reverse", secondCashReversalValues)
	if err != nil {
		t.Fatalf("POST second cash reversal: %v", err)
	}
	secondCashReversalBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(secondCashReversalBody, "已经完成冲销") {
		t.Fatalf("second cash reversal status=%d body=%q", response.StatusCode, secondCashReversalBody)
	}
	closeResponse(t, response)

	reversedCashDetail := getPage(t, client, server.URL+"/cash-transactions/"+createdCashTransactionID+"?reversed=1")
	if !strings.Contains(reversedCashDetail, "当前有效金额和未分配金额已归零") ||
		!strings.Contains(reversedCashDetail, "原始金额</dt><dd>2500.75 CNY") ||
		!strings.Contains(reversedCashDetail, "当前有效金额</dt><dd>0 CNY") ||
		!strings.Contains(reversedCashDetail, "已分配金额</dt><dd>0 CNY") ||
		!strings.Contains(reversedCashDetail, "未分配金额</dt><dd>0 CNY") ||
		!strings.Contains(reversedCashDetail, "原实际收款重复录入") ||
		strings.Contains(reversedCashDetail, "确认完整冲销") {
		t.Fatalf("reversed cash detail = %q", reversedCashDetail)
	}
	cashPage = getPage(t, client, server.URL+"/cash-transactions")
	if !strings.Contains(cashPage, "reversed") || !strings.Contains(cashPage, "原实际收款重复录入") ||
		!strings.Contains(cashPage, "2500.75 CNY") || !strings.Contains(cashPage, ">0 CNY</td>") {
		t.Fatalf("cash list after reversal = %q", cashPage)
	}
	allocationPage = getPage(t, client, server.URL+"/allocations")
	if strings.Contains(allocationPage, `<option value="`+createdCashTransactionID+`"`) ||
		!strings.Contains(allocationPage, "至少需要一笔仍有未分配金额") {
		t.Fatalf("allocation page after cash reversal = %q", allocationPage)
	}
	response, err = client.PostForm(server.URL+"/allocations", url.Values{
		"csrf_token":              {hiddenCSRF(t, allocationPage)},
		"command_id":              {"aa000000-0000-4000-8000-000000000003"},
		"cash_transaction_id":     {createdCashTransactionID},
		"obligation_selection":    {createdObligationID + ":1"},
		"amount":                  {"1"},
		"manual_source_reference": {"构造已冲销资金分配"},
	})
	if err != nil {
		t.Fatalf("POST allocation against reversed cash: %v", err)
	}
	allocationAfterCashReversalBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(allocationAfterCashReversalBody, "已冲销的实际收付款不能形成新的资金分配") {
		t.Fatalf("allocation after cash reversal status=%d body=%q", response.StatusCode, allocationAfterCashReversalBody)
	}
	closeResponse(t, response)

	var cashReversalID, cashReversalTransactionID, cashReversalType string
	var cashReversalAmount, cashReversalCurrency, cashReversalReason string
	var cashReversalActorKind, cashReversalActorID, cashReversalActorName string
	var cashReversalCount, cashReversalCommandCount, cashReversalAuditCount int
	var finalAllocationReversalCount, finalAllocationReversalCommandCount, finalAllocationReversalAuditCount int
	var finalNetAllocated, finalEffectiveAmount, finalUnallocated, finalObligationAllocated, cashFinalObligationOutstanding string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			cr.id::text,
			cr.cash_transaction_id::text,
			cr.transaction_type,
			cr.amount::text,
			cr.currency,
			cr.reversal_reason,
			cr.reversed_by_actor_kind,
			cr.reversed_by_actor_id,
			cr.reversed_by_actor_display_name,
			(SELECT count(*) FROM cash_transaction_reversals),
			(SELECT count(*) FROM cash_transaction_reversal_create_commands),
			(SELECT count(*) FROM audit_entries WHERE action = 'cash_transaction_reversed'),
			(SELECT count(*) FROM allocation_reversals),
			(SELECT count(*) FROM allocation_reversal_create_commands),
			(SELECT count(*) FROM audit_entries WHERE action = 'allocation_reversed'),
			(SELECT (COALESCE(SUM(a.amount), 0) - COALESCE(SUM(ar.amount), 0))::text
			 FROM allocations a LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
			 WHERE a.cash_transaction_id = $1),
			(CASE WHEN cr.id IS NULL THEN c.amount ELSE 0::numeric END)::text,
			((CASE WHEN cr.id IS NULL THEN c.amount ELSE 0::numeric END) -
			 (SELECT COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0)
			  FROM allocations a LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
			  WHERE a.cash_transaction_id = c.id))::text,
			(SELECT (COALESCE(SUM(a.amount), 0) - COALESCE(SUM(ar.amount), 0))::text
			 FROM allocations a LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
			 WHERE a.obligation_id = $2),
			(SELECT (orev.effective_amount - (COALESCE(SUM(a.amount), 0) - COALESCE(SUM(ar.amount), 0)))::text
			 FROM obligation_revisions orev
			 LEFT JOIN allocations a ON a.obligation_id = orev.obligation_id AND a.obligation_revision = orev.revision
			 LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
			 WHERE orev.obligation_id = $2 AND orev.revision = 1 GROUP BY orev.effective_amount)
		FROM cash_transaction_reversals cr
		JOIN cash_transactions c ON c.id = cr.cash_transaction_id
		WHERE cr.cash_transaction_id = $1
	`, createdCashTransactionID, createdObligationID).Scan(
		&cashReversalID, &cashReversalTransactionID, &cashReversalType,
		&cashReversalAmount, &cashReversalCurrency, &cashReversalReason,
		&cashReversalActorKind, &cashReversalActorID, &cashReversalActorName,
		&cashReversalCount, &cashReversalCommandCount, &cashReversalAuditCount,
		&finalAllocationReversalCount, &finalAllocationReversalCommandCount, &finalAllocationReversalAuditCount,
		&finalNetAllocated, &finalEffectiveAmount, &finalUnallocated,
		&finalObligationAllocated, &cashFinalObligationOutstanding,
	); err != nil {
		t.Fatalf("read cash reversal HTTP facts: %v", err)
	}
	if cashReversalID == "" || cashReversalTransactionID != createdCashTransactionID || cashReversalType != "receipt" ||
		cashReversalAmount != "2500.750000" || cashReversalCurrency != "CNY" || cashReversalReason != "原实际收款重复录入" ||
		cashReversalActorKind != "local_user" || cashReversalActorID != actualUserID || cashReversalActorName != "系统联调管理员" ||
		cashReversalCount != 1 || cashReversalCommandCount != 1 || cashReversalAuditCount != 1 ||
		finalAllocationReversalCount != 2 || finalAllocationReversalCommandCount != 2 || finalAllocationReversalAuditCount != 2 ||
		finalNetAllocated != "0.000000" || finalEffectiveAmount != "0" || finalUnallocated != "0.000000" ||
		finalObligationAllocated != "0.000000" || cashFinalObligationOutstanding != "100.500000" {
		t.Fatalf("cash reversal HTTP facts id=%s cash=%s type=%s amount=%s %s reason=%q actor=%s/%s/%s counts=%d/%d/%d allocation_reversals=%d/%d/%d balances=%s/%s/%s/%s/%s",
			cashReversalID, cashReversalTransactionID, cashReversalType, cashReversalAmount, cashReversalCurrency,
			cashReversalReason, cashReversalActorKind, cashReversalActorID, cashReversalActorName,
			cashReversalCount, cashReversalCommandCount, cashReversalAuditCount,
			finalAllocationReversalCount, finalAllocationReversalCommandCount, finalAllocationReversalAuditCount,
			finalNetAllocated, finalEffectiveAmount, finalUnallocated, finalObligationAllocated, cashFinalObligationOutstanding)
	}

	adjustmentDetail := getPage(t, client, server.URL+"/obligations/"+createdObligationID)
	if !strings.Contains(adjustmentDetail, "调整义务金额") ||
		!strings.Contains(adjustmentDetail, "原始金额</dt><dd>100.5 CNY") ||
		!strings.Contains(adjustmentDetail, "当前有效金额</dt><dd>100.5 CNY") ||
		!strings.Contains(adjustmentDetail, "Current revision</dt><dd>1") {
		t.Fatalf("obligation before amount adjustment = %q", adjustmentDetail)
	}
	blankAdjustmentValues := url.Values{
		"csrf_token":                {hiddenCSRF(t, adjustmentDetail)},
		"command_id":                {hiddenCommand(t, adjustmentDetail)},
		"expected_current_revision": {"1"},
		"adjustment_type":           {"increase"},
		"adjustment_amount":         {"20"},
		"adjustment_reason":         {"   "},
	}
	response, err = client.PostForm(server.URL+"/obligations/"+createdObligationID+"/adjust-amount", blankAdjustmentValues)
	if err != nil {
		t.Fatalf("POST blank obligation amount adjustment: %v", err)
	}
	blankAdjustmentBody := readResponse(t, response)
	if response.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(blankAdjustmentBody, "调整原因不能为空") {
		t.Fatalf("blank amount adjustment status=%d body=%q", response.StatusCode, blankAdjustmentBody)
	}
	closeResponse(t, response)

	firstAdjustmentCommand := hiddenCommand(t, blankAdjustmentBody)
	firstAdjustmentValues := url.Values{
		"csrf_token":                 {hiddenCSRF(t, blankAdjustmentBody)},
		"command_id":                 {firstAdjustmentCommand},
		"expected_current_revision":  {"1"},
		"adjustment_type":            {"increase"},
		"adjustment_amount":          {"020.000000"},
		"adjustment_reason":          {" 补充确认追加服务范围 "},
		"previous_effective_amount":  {"999999"},
		"resulting_effective_amount": {"1"},
		"currency":                   {"USD"},
		"actor_kind":                 {"forged"},
		"actor_id":                   {"forged"},
		"actor_display_name":         {"伪造调整人"},
		"adjusted_at":                {"2000-01-01T00:00:00Z"},
	}
	response, err = client.PostForm(server.URL+"/obligations/"+createdObligationID+"/adjust-amount", firstAdjustmentValues)
	if err != nil {
		t.Fatalf("POST first obligation amount adjustment: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/obligations/"+createdObligationID+"?adjusted=1" {
		t.Fatalf("first amount adjustment status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	response, err = client.PostForm(server.URL+"/obligations/"+createdObligationID+"/adjust-amount", firstAdjustmentValues)
	if err != nil {
		t.Fatalf("POST first amount adjustment replay: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/obligations/"+createdObligationID+"?adjusted=1" {
		t.Fatalf("first amount adjustment replay status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	changedAdjustmentValues := url.Values{}
	for key, values := range firstAdjustmentValues {
		changedAdjustmentValues[key] = append([]string(nil), values...)
	}
	changedAdjustmentValues.Set("adjustment_reason", "改变后的金额调整原因")
	response, err = client.PostForm(server.URL+"/obligations/"+createdObligationID+"/adjust-amount", changedAdjustmentValues)
	if err != nil {
		t.Fatalf("POST changed amount adjustment replay: %v", err)
	}
	changedAdjustmentBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(changedAdjustmentBody, "不同的义务金额调整意图") ||
		!strings.Contains(changedAdjustmentBody, "补充确认追加服务范围") {
		t.Fatalf("changed amount adjustment status=%d body=%q", response.StatusCode, changedAdjustmentBody)
	}
	closeResponse(t, response)

	staleAdjustmentValues := url.Values{}
	for key, values := range firstAdjustmentValues {
		staleAdjustmentValues[key] = append([]string(nil), values...)
	}
	staleAdjustmentValues.Set("command_id", "ab000000-0000-4000-8000-000000000001")
	response, err = client.PostForm(server.URL+"/obligations/"+createdObligationID+"/adjust-amount", staleAdjustmentValues)
	if err != nil {
		t.Fatalf("POST stale amount adjustment: %v", err)
	}
	staleAdjustmentBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(staleAdjustmentBody, "current revision 已变化") {
		t.Fatalf("stale amount adjustment status=%d body=%q", response.StatusCode, staleAdjustmentBody)
	}
	closeResponse(t, response)

	adjustmentDetail = getPage(t, client, server.URL+"/obligations/"+createdObligationID+"?adjusted=1")
	if !strings.Contains(adjustmentDetail, "义务金额已调整并形成新的 revision") ||
		!strings.Contains(adjustmentDetail, "Current revision</dt><dd>2") ||
		!strings.Contains(adjustmentDetail, "当前有效金额</dt><dd>120.5 CNY") ||
		!strings.Contains(adjustmentDetail, "补充确认追加服务范围") ||
		!strings.Contains(adjustmentDetail, "100.5 CNY") || !strings.Contains(adjustmentDetail, "120.5 CNY") {
		t.Fatalf("obligation after first amount adjustment = %q", adjustmentDetail)
	}
	secondAdjustmentValues := url.Values{
		"csrf_token":                {hiddenCSRF(t, adjustmentDetail)},
		"command_id":                {hiddenCommand(t, adjustmentDetail)},
		"expected_current_revision": {"2"},
		"adjustment_type":           {"decrease"},
		"adjustment_amount":         {"20"},
		"adjustment_reason":         {"移除未发生的追加范围"},
	}
	response, err = client.PostForm(server.URL+"/obligations/"+createdObligationID+"/adjust-amount", secondAdjustmentValues)
	if err != nil {
		t.Fatalf("POST second obligation amount adjustment: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/obligations/"+createdObligationID+"?adjusted=1" {
		t.Fatalf("second amount adjustment status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	adjustmentCashPage := getPage(t, client, server.URL+"/cash-transactions")
	adjustmentCashValues := url.Values{
		"csrf_token":                     {hiddenCSRF(t, adjustmentCashPage)},
		"command_id":                     {hiddenCommand(t, adjustmentCashPage)},
		"type":                           {"receipt"},
		"ledger_owner_party_id":          {createdPartyID},
		"counterparty_party_id":          {secondPartyID},
		"description":                    {"金额调整后实际收款"},
		"amount":                         {"30"},
		"currency":                       {"CNY"},
		"occurred_on":                    {"2026-09-20"},
		"account_reference":              {"金额调整后测试账户"},
		"manual_source_reference":        {"金额调整后测试回单"},
		"external_transaction_reference": {"HTTP-ADJUST-CASH-001"},
	}
	response, err = client.PostForm(server.URL+"/cash-transactions", adjustmentCashValues)
	if err != nil {
		t.Fatalf("POST cash after obligation adjustment: %v", err)
	}
	adjustmentCashLocation, err := url.Parse(response.Header.Get("Location"))
	if response.StatusCode != http.StatusSeeOther || err != nil || !strings.HasPrefix(adjustmentCashLocation.Path, "/cash-transactions/") {
		t.Fatalf("cash after adjustment status=%d location=%q error=%v body=%q", response.StatusCode, response.Header.Get("Location"), err, readResponse(t, response))
	}
	adjustmentCashID := strings.TrimPrefix(adjustmentCashLocation.Path, "/cash-transactions/")
	closeResponse(t, response)

	adjustmentAllocationPage := getPage(t, client, server.URL+"/allocations")
	if !strings.Contains(adjustmentAllocationPage, `<option value="`+adjustmentCashID+`"`) ||
		!strings.Contains(adjustmentAllocationPage, `value="`+createdObligationID+`:3"`) {
		t.Fatalf("allocation page after amount adjustment = %q", adjustmentAllocationPage)
	}
	adjustmentAllocationValues := url.Values{
		"csrf_token":              {hiddenCSRF(t, adjustmentAllocationPage)},
		"command_id":              {hiddenCommand(t, adjustmentAllocationPage)},
		"cash_transaction_id":     {adjustmentCashID},
		"obligation_selection":    {createdObligationID + ":3"},
		"amount":                  {"10"},
		"manual_source_reference": {"按调整后 revision 3 分配"},
	}
	response, err = client.PostForm(server.URL+"/allocations", adjustmentAllocationValues)
	if err != nil {
		t.Fatalf("POST allocation after obligation adjustment: %v", err)
	}
	adjustmentAllocationLocation, err := url.Parse(response.Header.Get("Location"))
	if response.StatusCode != http.StatusSeeOther || err != nil || !strings.HasPrefix(adjustmentAllocationLocation.Path, "/allocations/") {
		t.Fatalf("allocation after adjustment status=%d location=%q error=%v body=%q", response.StatusCode, response.Header.Get("Location"), err, readResponse(t, response))
	}
	adjustmentAllocationID := strings.TrimPrefix(adjustmentAllocationLocation.Path, "/allocations/")
	closeResponse(t, response)

	adjustmentDetail = getPage(t, client, server.URL+"/obligations/"+createdObligationID)
	belowAllocatedValues := url.Values{
		"csrf_token":                {hiddenCSRF(t, adjustmentDetail)},
		"command_id":                {hiddenCommand(t, adjustmentDetail)},
		"expected_current_revision": {"3"},
		"adjustment_type":           {"decrease"},
		"adjustment_amount":         {"91"},
		"adjustment_reason":         {"构造低于已分配净额"},
	}
	response, err = client.PostForm(server.URL+"/obligations/"+createdObligationID+"/adjust-amount", belowAllocatedValues)
	if err != nil {
		t.Fatalf("POST amount adjustment below allocated: %v", err)
	}
	belowAllocatedBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(belowAllocatedBody, "不能低于当前已分配净额") ||
		!strings.Contains(belowAllocatedBody, adjustmentAllocationID) {
		t.Fatalf("below allocated adjustment status=%d body=%q", response.StatusCode, belowAllocatedBody)
	}
	closeResponse(t, response)

	thirdAdjustmentValues := url.Values{
		"csrf_token":                {hiddenCSRF(t, belowAllocatedBody)},
		"command_id":                {hiddenCommand(t, belowAllocatedBody)},
		"expected_current_revision": {"3"},
		"adjustment_type":           {"decrease"},
		"adjustment_amount":         {"90.5"},
		"adjustment_reason":         {"最终确认当前义务仅余十元"},
	}
	response, err = client.PostForm(server.URL+"/obligations/"+createdObligationID+"/adjust-amount", thirdAdjustmentValues)
	if err != nil {
		t.Fatalf("POST third obligation amount adjustment: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/obligations/"+createdObligationID+"?adjusted=1" {
		t.Fatalf("third amount adjustment status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	adjustmentAllocationPage = getPage(t, client, server.URL+"/allocations")
	response, err = client.PostForm(server.URL+"/allocations", url.Values{
		"csrf_token":              {hiddenCSRF(t, adjustmentAllocationPage)},
		"command_id":              {"ab000000-0000-4000-8000-000000000002"},
		"cash_transaction_id":     {adjustmentCashID},
		"obligation_selection":    {createdObligationID + ":3"},
		"amount":                  {"1"},
		"manual_source_reference": {"构造过期 revision 分配"},
	})
	if err != nil {
		t.Fatalf("POST allocation against stale adjusted revision: %v", err)
	}
	staleAllocationBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(staleAllocationBody, "义务 revision 已经变化") {
		t.Fatalf("stale adjusted allocation status=%d body=%q", response.StatusCode, staleAllocationBody)
	}
	closeResponse(t, response)

	adjustmentDetail = getPage(t, client, server.URL+"/obligations/"+createdObligationID+"?adjusted=1")
	if !strings.Contains(adjustmentDetail, "Current revision</dt><dd>4") ||
		!strings.Contains(adjustmentDetail, "原始金额</dt><dd>100.5 CNY") ||
		!strings.Contains(adjustmentDetail, "当前有效金额</dt><dd>10 CNY") ||
		!strings.Contains(adjustmentDetail, "已分配金额</dt><dd>10 CNY") ||
		!strings.Contains(adjustmentDetail, "剩余金额</dt><dd>0 CNY") ||
		!strings.Contains(adjustmentDetail, "最终确认当前义务仅余十元") ||
		!strings.Contains(adjustmentDetail, adjustmentAllocationID) ||
		!strings.Contains(adjustmentDetail, "open") {
		t.Fatalf("final adjusted obligation detail = %q", adjustmentDetail)
	}

	var adjustedOriginal, adjustedEffective, adjustedAllocated, adjustedOutstanding, adjustedStatus string
	var adjustedCurrentRevision, adjustmentRevisionCount, adjustmentCount, adjustmentCommandCount, adjustmentAuditCount int
	var firstAdjustmentType, firstAdjustmentAmount, firstPrevious, firstResult, firstReason string
	var firstAdjustmentActorKind, firstAdjustmentActorID, firstAdjustmentActorName string
	var activeAdjustedAllocationRevision int64
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			initial.effective_amount::text,
			current.effective_amount::text,
			(COALESCE(SUM(a.amount), 0) - COALESCE(SUM(ar.amount), 0))::text,
			(current.effective_amount - (COALESCE(SUM(a.amount), 0) - COALESCE(SUM(ar.amount), 0)))::text,
			o.status,
			o.current_revision,
			(SELECT count(*) FROM obligation_revisions WHERE obligation_id = o.id),
			(SELECT count(*) FROM obligation_amount_adjustments WHERE obligation_id = o.id),
			(SELECT count(*) FROM obligation_amount_adjustment_commands WHERE obligation_id = o.id),
			(SELECT count(*) FROM audit_entries WHERE object_type = 'obligation' AND object_id = o.id AND action = 'obligation_amount_adjusted'),
			first_adjustment.adjustment_type,
			first_adjustment.adjustment_amount::text,
			first_adjustment.previous_effective_amount::text,
			first_adjustment.resulting_effective_amount::text,
			first_adjustment.adjustment_reason,
			first_adjustment.adjusted_by_actor_kind,
			first_adjustment.adjusted_by_actor_id,
			first_adjustment.adjusted_by_actor_display_name,
			(SELECT obligation_revision FROM allocations WHERE id = $2)
		FROM obligations o
		JOIN obligation_revisions initial ON initial.obligation_id = o.id AND initial.revision = 1
		JOIN obligation_revisions current ON current.obligation_id = o.id AND current.revision = o.current_revision
		JOIN obligation_amount_adjustments first_adjustment ON first_adjustment.obligation_id = o.id AND first_adjustment.source_revision = 1
		LEFT JOIN allocations a ON a.obligation_id = o.id
		LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
		WHERE o.id = $1
		GROUP BY o.id, initial.obligation_id, initial.revision, current.obligation_id, current.revision,
			first_adjustment.id
	`, createdObligationID, adjustmentAllocationID).Scan(
		&adjustedOriginal, &adjustedEffective, &adjustedAllocated, &adjustedOutstanding,
		&adjustedStatus, &adjustedCurrentRevision,
		&adjustmentRevisionCount, &adjustmentCount, &adjustmentCommandCount, &adjustmentAuditCount,
		&firstAdjustmentType, &firstAdjustmentAmount, &firstPrevious, &firstResult, &firstReason,
		&firstAdjustmentActorKind, &firstAdjustmentActorID, &firstAdjustmentActorName,
		&activeAdjustedAllocationRevision,
	); err != nil {
		t.Fatalf("read obligation amount adjustment HTTP facts: %v", err)
	}
	if adjustedOriginal != "100.500000" || adjustedEffective != "10.000000" ||
		adjustedAllocated != "10.000000" || adjustedOutstanding != "0.000000" || adjustedStatus != "open" ||
		adjustedCurrentRevision != 4 || adjustmentRevisionCount != 4 || adjustmentCount != 3 ||
		adjustmentCommandCount != 3 || adjustmentAuditCount != 3 ||
		firstAdjustmentType != "increase" || firstAdjustmentAmount != "20.000000" ||
		firstPrevious != "100.500000" || firstResult != "120.500000" || firstReason != "补充确认追加服务范围" ||
		firstAdjustmentActorKind != "local_user" || firstAdjustmentActorID != actualUserID ||
		firstAdjustmentActorName != "系统联调管理员" || activeAdjustedAllocationRevision != 3 {
		t.Fatalf("obligation adjustment HTTP facts amounts=%s/%s/%s/%s status=%s/%d counts=%d/%d/%d/%d first=%s/%s/%s/%s/%q actor=%s/%s/%s allocation_revision=%d",
			adjustedOriginal, adjustedEffective, adjustedAllocated, adjustedOutstanding,
			adjustedStatus, adjustedCurrentRevision,
			adjustmentRevisionCount, adjustmentCount, adjustmentCommandCount, adjustmentAuditCount,
			firstAdjustmentType, firstAdjustmentAmount, firstPrevious, firstResult, firstReason,
			firstAdjustmentActorKind, firstAdjustmentActorID, firstAdjustmentActorName, activeAdjustedAllocationRevision)
	}

	remissionDetail := getPage(t, client, server.URL+"/obligations/"+createdObligationID)
	increaseForRemissionValues := url.Values{
		"csrf_token":                {hiddenCSRF(t, remissionDetail)},
		"command_id":                {"ac000000-0000-4000-8000-000000000001"},
		"expected_current_revision": {"4"},
		"adjustment_type":           {"increase"},
		"adjustment_amount":         {"100"},
		"adjustment_reason":         {"为减免联调补充确认金额"},
	}
	response, err = client.PostForm(server.URL+"/obligations/"+createdObligationID+"/adjust-amount", increaseForRemissionValues)
	if err != nil {
		t.Fatalf("POST increase before obligation remission: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/obligations/"+createdObligationID+"?adjusted=1" {
		t.Fatalf("increase before remission status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	remissionDetail = getPage(t, client, server.URL+"/obligations/"+createdObligationID)
	blankRemissionValues := url.Values{
		"csrf_token":                {hiddenCSRF(t, remissionDetail)},
		"command_id":                {"ad000000-0000-4000-8000-000000000001"},
		"expected_current_revision": {"5"},
		"remission_amount":          {"30"},
		"remission_reason":          {""},
		"manual_source_reference":   {""},
	}
	response, err = client.PostForm(server.URL+"/obligations/"+createdObligationID+"/remissions", blankRemissionValues)
	if err != nil {
		t.Fatalf("POST blank obligation remission: %v", err)
	}
	blankRemissionBody := readResponse(t, response)
	if response.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(blankRemissionBody, "减免原因不能为空") ||
		!strings.Contains(blankRemissionBody, `value="30"`) {
		t.Fatalf("blank remission status=%d body=%q", response.StatusCode, blankRemissionBody)
	}
	closeResponse(t, response)

	firstRemissionValues := url.Values{
		"csrf_token":                {hiddenCSRF(t, blankRemissionBody)},
		"command_id":                {"ad000000-0000-4000-8000-000000000002"},
		"expected_current_revision": {"5"},
		"remission_amount":          {"030.000000"},
		"remission_reason":          {" 双方确认免除部分服务费 "},
		"manual_source_reference":   {" 减免确认函 RM-HTTP-001 "},
		"recorded_effective_amount": {"999999"},
		"currency":                  {"USD"},
		"actor_kind":                {"forged"},
		"actor_id":                  {"forged"},
		"actor_display_name":        {"伪造减免人"},
		"recorded_at":               {"2000-01-01T00:00:00Z"},
	}
	response, err = client.PostForm(server.URL+"/obligations/"+createdObligationID+"/remissions", firstRemissionValues)
	if err != nil {
		t.Fatalf("POST first obligation remission: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/obligations/"+createdObligationID+"?remitted=1" {
		t.Fatalf("first remission status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	response, err = client.PostForm(server.URL+"/obligations/"+createdObligationID+"/remissions", firstRemissionValues)
	if err != nil {
		t.Fatalf("POST first obligation remission replay: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/obligations/"+createdObligationID+"?remitted=1" {
		t.Fatalf("first remission replay status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	changedRemissionValues := url.Values{}
	for key, values := range firstRemissionValues {
		changedRemissionValues[key] = append([]string(nil), values...)
	}
	changedRemissionValues.Set("remission_reason", "改变后的减免原因")
	response, err = client.PostForm(server.URL+"/obligations/"+createdObligationID+"/remissions", changedRemissionValues)
	if err != nil {
		t.Fatalf("POST changed obligation remission replay: %v", err)
	}
	changedRemissionBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(changedRemissionBody, "不同的义务减免意图") ||
		!strings.Contains(changedRemissionBody, "改变后的减免原因") ||
		!strings.Contains(changedRemissionBody, "双方确认免除部分服务费") {
		t.Fatalf("changed remission status=%d body=%q", response.StatusCode, changedRemissionBody)
	}
	closeResponse(t, response)

	secondRemissionValues := url.Values{
		"csrf_token":                {hiddenCSRF(t, changedRemissionBody)},
		"command_id":                {"ad000000-0000-4000-8000-000000000003"},
		"expected_current_revision": {"5"},
		"remission_amount":          {"60"},
		"remission_reason":          {"最终减免确认"},
		"manual_source_reference":   {"减免确认函 RM-HTTP-002"},
	}
	response, err = client.PostForm(server.URL+"/obligations/"+createdObligationID+"/remissions", secondRemissionValues)
	if err != nil {
		t.Fatalf("POST second obligation remission: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/obligations/"+createdObligationID+"?remitted=1" {
		t.Fatalf("second remission status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	remissionDetail = getPage(t, client, server.URL+"/obligations/"+createdObligationID+"?remitted=1")
	if !strings.Contains(remissionDetail, "义务减免已记录，原 revision 保持不变") ||
		!strings.Contains(remissionDetail, "Current revision</dt><dd>5") ||
		!strings.Contains(remissionDetail, "当前有效金额</dt><dd>110 CNY") ||
		!strings.Contains(remissionDetail, "已分配金额</dt><dd>10 CNY") ||
		!strings.Contains(remissionDetail, "已减免金额</dt><dd>90 CNY") ||
		!strings.Contains(remissionDetail, "剩余金额</dt><dd>10 CNY") ||
		!strings.Contains(remissionDetail, "双方确认免除部分服务费") ||
		!strings.Contains(remissionDetail, "最终减免确认") ||
		!strings.Contains(remissionDetail, "减免确认函 RM-HTTP-001") ||
		!strings.Contains(remissionDetail, "减免确认函 RM-HTTP-002") {
		t.Fatalf("obligation after partial remissions = %q", remissionDetail)
	}

	overRemissionValues := url.Values{
		"csrf_token":                {hiddenCSRF(t, remissionDetail)},
		"command_id":                {"ad000000-0000-4000-8000-000000000004"},
		"expected_current_revision": {"5"},
		"remission_amount":          {"11"},
		"remission_reason":          {"构造超过剩余金额"},
		"manual_source_reference":   {"越界减免测试"},
	}
	response, err = client.PostForm(server.URL+"/obligations/"+createdObligationID+"/remissions", overRemissionValues)
	if err != nil {
		t.Fatalf("POST obligation remission over outstanding: %v", err)
	}
	overRemissionBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(overRemissionBody, "减免金额不能超过义务当前剩余金额") ||
		!strings.Contains(overRemissionBody, "最终减免确认") {
		t.Fatalf("over remission status=%d body=%q", response.StatusCode, overRemissionBody)
	}
	closeResponse(t, response)

	remissionAllocationPage := getPage(t, client, server.URL+"/allocations")
	response, err = client.PostForm(server.URL+"/allocations", url.Values{
		"csrf_token":              {hiddenCSRF(t, remissionAllocationPage)},
		"command_id":              {"ae000000-0000-4000-8000-000000000001"},
		"cash_transaction_id":     {adjustmentCashID},
		"obligation_selection":    {createdObligationID + ":5"},
		"amount":                  {"10"},
		"manual_source_reference": {"按减免后的剩余金额分配"},
	})
	if err != nil {
		t.Fatalf("POST allocation after obligation remission: %v", err)
	}
	remissionAllocationLocation, err := url.Parse(response.Header.Get("Location"))
	if response.StatusCode != http.StatusSeeOther || err != nil || !strings.HasPrefix(remissionAllocationLocation.Path, "/allocations/") {
		t.Fatalf("allocation after remission status=%d location=%q error=%v body=%q", response.StatusCode, response.Header.Get("Location"), err, readResponse(t, response))
	}
	remissionAllocationID := strings.TrimPrefix(remissionAllocationLocation.Path, "/allocations/")
	closeResponse(t, response)

	remissionDetail = getPage(t, client, server.URL+"/obligations/"+createdObligationID)
	response, err = client.PostForm(server.URL+"/obligations/"+createdObligationID+"/remissions", url.Values{
		"csrf_token":                {hiddenCSRF(t, remissionDetail)},
		"command_id":                {"ad000000-0000-4000-8000-000000000005"},
		"expected_current_revision": {"5"},
		"remission_amount":          {"1"},
		"remission_reason":          {"构造余额归零后继续减免"},
		"manual_source_reference":   {"余额归零测试"},
	})
	if err != nil {
		t.Fatalf("POST remission after outstanding reached zero: %v", err)
	}
	zeroBalanceRemissionBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(zeroBalanceRemissionBody, "减免金额不能超过义务当前剩余金额") ||
		!strings.Contains(zeroBalanceRemissionBody, remissionAllocationID) {
		t.Fatalf("zero balance remission status=%d body=%q", response.StatusCode, zeroBalanceRemissionBody)
	}
	closeResponse(t, response)

	response, err = client.PostForm(server.URL+"/obligations/"+createdObligationID+"/adjust-amount", url.Values{
		"csrf_token":                {hiddenCSRF(t, zeroBalanceRemissionBody)},
		"command_id":                {"ac000000-0000-4000-8000-000000000002"},
		"expected_current_revision": {"5"},
		"adjustment_type":           {"decrease"},
		"adjustment_amount":         {"1"},
		"adjustment_reason":         {"构造低于已分配与已减免合计"},
	})
	if err != nil {
		t.Fatalf("POST adjustment below allocated and remitted: %v", err)
	}
	belowCommittedBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(belowCommittedBody, "不能低于当前已分配净额与已减免金额之和") ||
		!strings.Contains(belowCommittedBody, "最终减免确认") {
		t.Fatalf("below allocated and remitted status=%d body=%q", response.StatusCode, belowCommittedBody)
	}
	closeResponse(t, response)

	remissionDetail = getPage(t, client, server.URL+"/obligations/"+createdObligationID)
	if !strings.Contains(remissionDetail, "Current revision</dt><dd>5") ||
		!strings.Contains(remissionDetail, "原始金额</dt><dd>100.5 CNY") ||
		!strings.Contains(remissionDetail, "当前有效金额</dt><dd>110 CNY") ||
		!strings.Contains(remissionDetail, "已分配金额</dt><dd>20 CNY") ||
		!strings.Contains(remissionDetail, "已减免金额</dt><dd>90 CNY") ||
		!strings.Contains(remissionDetail, "剩余金额</dt><dd>0 CNY") ||
		!strings.Contains(remissionDetail, remissionAllocationID) ||
		!strings.Contains(remissionDetail, "open") {
		t.Fatalf("final remitted obligation detail = %q", remissionDetail)
	}

	var remittedOriginal, remittedEffective, remittedAllocated, remittedAmount, remittedOutstanding, remittedStatus string
	var remittedCurrentRevision, remissionRevisionCount, remissionAdjustmentCount, remissionAdjustmentCommandCount int
	var remissionAdjustmentAuditCount, remissionCount, remissionCommandCount, remissionAuditCount int
	var firstRemissionRevision, finalRemissionAllocationRevision int64
	var firstRemissionAmount, firstRemissionEffective, firstRemissionCurrency, firstRemissionReason, firstRemissionSource string
	var firstRemissionActorKind, firstRemissionActorID, firstRemissionActorName string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			initial.effective_amount::text,
			current.effective_amount::text,
			(SELECT COALESCE(SUM(a.amount), 0) - COALESCE(SUM(ar.amount), 0)
			 FROM allocations a LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
			 WHERE a.obligation_id = o.id)::text,
			(SELECT COALESCE(SUM(amount), 0) FROM obligation_remissions WHERE obligation_id = o.id)::text,
			(current.effective_amount
			 - (SELECT COALESCE(SUM(a.amount), 0) - COALESCE(SUM(ar.amount), 0)
			    FROM allocations a LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
			    WHERE a.obligation_id = o.id)
			 - (SELECT COALESCE(SUM(amount), 0) FROM obligation_remissions WHERE obligation_id = o.id))::text,
			o.status,
			o.current_revision,
			(SELECT count(*) FROM obligation_revisions WHERE obligation_id = o.id),
			(SELECT count(*) FROM obligation_amount_adjustments WHERE obligation_id = o.id),
			(SELECT count(*) FROM obligation_amount_adjustment_commands WHERE obligation_id = o.id),
			(SELECT count(*) FROM audit_entries WHERE object_type = 'obligation' AND object_id = o.id AND action = 'obligation_amount_adjusted'),
			(SELECT count(*) FROM obligation_remissions WHERE obligation_id = o.id),
			(SELECT count(*) FROM obligation_remission_commands WHERE obligation_id = o.id),
			(SELECT count(*) FROM audit_entries WHERE object_type = 'obligation' AND object_id = o.id AND action = 'obligation_remission_recorded'),
			first_remission.obligation_revision,
			first_remission.amount::text,
			first_remission.recorded_effective_amount::text,
			first_remission.currency,
			first_remission.remission_reason,
			first_remission.manual_source_reference,
			first_remission.recorded_by_actor_kind,
			first_remission.recorded_by_actor_id,
			first_remission.recorded_by_actor_display_name,
			(SELECT obligation_revision FROM allocations WHERE id = $2)
		FROM obligations o
		JOIN obligation_revisions initial ON initial.obligation_id = o.id AND initial.revision = 1
		JOIN obligation_revisions current ON current.obligation_id = o.id AND current.revision = o.current_revision
		JOIN obligation_remissions first_remission ON first_remission.obligation_id = o.id AND first_remission.amount = 30
		WHERE o.id = $1
	`, createdObligationID, remissionAllocationID).Scan(
		&remittedOriginal, &remittedEffective, &remittedAllocated, &remittedAmount, &remittedOutstanding,
		&remittedStatus, &remittedCurrentRevision,
		&remissionRevisionCount, &remissionAdjustmentCount, &remissionAdjustmentCommandCount, &remissionAdjustmentAuditCount,
		&remissionCount, &remissionCommandCount, &remissionAuditCount,
		&firstRemissionRevision, &firstRemissionAmount, &firstRemissionEffective, &firstRemissionCurrency,
		&firstRemissionReason, &firstRemissionSource,
		&firstRemissionActorKind, &firstRemissionActorID, &firstRemissionActorName,
		&finalRemissionAllocationRevision,
	); err != nil {
		t.Fatalf("read obligation remission HTTP facts: %v", err)
	}
	if remittedOriginal != "100.500000" || remittedEffective != "110.000000" ||
		remittedAllocated != "20.000000" || remittedAmount != "90.000000" || remittedOutstanding != "0.000000" ||
		remittedStatus != "open" || remittedCurrentRevision != 5 || remissionRevisionCount != 5 ||
		remissionAdjustmentCount != 4 || remissionAdjustmentCommandCount != 4 || remissionAdjustmentAuditCount != 4 ||
		remissionCount != 2 || remissionCommandCount != 2 || remissionAuditCount != 2 ||
		firstRemissionRevision != 5 || firstRemissionAmount != "30.000000" || firstRemissionEffective != "110.000000" ||
		firstRemissionCurrency != "CNY" || firstRemissionReason != "双方确认免除部分服务费" ||
		firstRemissionSource != "减免确认函 RM-HTTP-001" || firstRemissionActorKind != "local_user" ||
		firstRemissionActorID != actualUserID || firstRemissionActorName != "系统联调管理员" || finalRemissionAllocationRevision != 5 {
		t.Fatalf("obligation remission HTTP facts amounts=%s/%s/%s/%s/%s status=%s/%d counts=%d/%d/%d/%d remissions=%d/%d/%d first=%d/%s/%s/%s/%q/%q actor=%s/%s/%s allocation_revision=%d",
			remittedOriginal, remittedEffective, remittedAllocated, remittedAmount, remittedOutstanding,
			remittedStatus, remittedCurrentRevision,
			remissionRevisionCount, remissionAdjustmentCount, remissionAdjustmentCommandCount, remissionAdjustmentAuditCount,
			remissionCount, remissionCommandCount, remissionAuditCount,
			firstRemissionRevision, firstRemissionAmount, firstRemissionEffective, firstRemissionCurrency,
			firstRemissionReason, firstRemissionSource,
			firstRemissionActorKind, firstRemissionActorID, firstRemissionActorName, finalRemissionAllocationRevision)
	}
	agingPage = getPage(t, client, server.URL+"/aging?as_of_date=2099-12-31")
	if strings.Contains(agingPage, "系统联调九月服务费") || !strings.Contains(agingPage, "当前没有状态为 open 且剩余金额大于 0 的义务") {
		t.Fatalf("zero-balance obligation should be excluded from aging = %q", agingPage)
	}

	closureDetail := getPage(t, client, server.URL+"/obligations/"+createdObligationID)
	if !strings.Contains(closureDetail, `action="/obligations/`+createdObligationID+`/close"`) ||
		!strings.Contains(closureDetail, "受控关闭零余额义务") {
		t.Fatalf("zero-balance obligation has no closure action: %q", closureDetail)
	}
	balanceSequence := hiddenBalanceSequence(t, closureDetail)
	blankClosureValues := url.Values{
		"csrf_token":                {hiddenCSRF(t, closureDetail)},
		"command_id":                {"af000000-0000-4000-8000-000000000001"},
		"expected_current_revision": {"5"},
		"expected_balance_sequence": {balanceSequence},
		"closure_reason":            {""},
	}
	response, err = client.PostForm(server.URL+"/obligations/"+createdObligationID+"/close", blankClosureValues)
	if err != nil {
		t.Fatalf("POST blank obligation closure: %v", err)
	}
	blankClosureBody := readResponse(t, response)
	if response.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(blankClosureBody, "关闭原因不能为空") ||
		!strings.Contains(blankClosureBody, `name="expected_balance_sequence" value="`+balanceSequence+`"`) {
		t.Fatalf("blank closure status=%d body=%q", response.StatusCode, blankClosureBody)
	}
	closeResponse(t, response)

	closureValues := url.Values{
		"csrf_token":                {hiddenCSRF(t, blankClosureBody)},
		"command_id":                {"af000000-0000-4000-8000-000000000002"},
		"expected_current_revision": {"5"},
		"expected_balance_sequence": {balanceSequence},
		"closure_reason":            {" 余额事实已逐项复核并完成结清 "},
		"recorded_effective_amount": {"999999"},
		"net_allocated_amount":      {"999999"},
		"remitted_amount":           {"999999"},
		"currency":                  {"USD"},
		"settlement_basis":          {"forged"},
		"actor_kind":                {"forged"},
		"actor_id":                  {"forged"},
		"actor_display_name":        {"伪造关闭人"},
		"closed_at":                 {"2000-01-01T00:00:00Z"},
	}
	response, err = client.PostForm(server.URL+"/obligations/"+createdObligationID+"/close", closureValues)
	if err != nil {
		t.Fatalf("POST obligation closure: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/obligations/"+createdObligationID+"?closed=1" {
		t.Fatalf("closure status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	response, err = client.PostForm(server.URL+"/obligations/"+createdObligationID+"/close", closureValues)
	if err != nil {
		t.Fatalf("POST obligation closure replay: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/obligations/"+createdObligationID+"?closed=1" {
		t.Fatalf("closure replay status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	changedClosureValues := url.Values{}
	for key, values := range closureValues {
		changedClosureValues[key] = append([]string(nil), values...)
	}
	changedClosureValues.Set("closure_reason", "改变后的关闭原因")
	response, err = client.PostForm(server.URL+"/obligations/"+createdObligationID+"/close", changedClosureValues)
	if err != nil {
		t.Fatalf("POST changed obligation closure replay: %v", err)
	}
	changedClosureBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(changedClosureBody, "不同的义务关闭意图") ||
		!strings.Contains(changedClosureBody, "余额事实已逐项复核并完成结清") {
		t.Fatalf("changed closure status=%d body=%q", response.StatusCode, changedClosureBody)
	}
	closeResponse(t, response)

	closedDetail := getPage(t, client, server.URL+"/obligations/"+createdObligationID+"?closed=1")
	if !strings.Contains(closedDetail, "零余额义务已受控关闭") ||
		!strings.Contains(closedDetail, "状态</dt><dd>closed") ||
		!strings.Contains(closedDetail, "Current revision</dt><dd>5") ||
		!strings.Contains(closedDetail, "当前有效金额</dt><dd>110 CNY") ||
		!strings.Contains(closedDetail, "已分配金额</dt><dd>20 CNY") ||
		!strings.Contains(closedDetail, "已减免金额</dt><dd>90 CNY") ||
		!strings.Contains(closedDetail, "剩余金额</dt><dd>0 CNY") ||
		!strings.Contains(closedDetail, "资金分配与减免组合归零") ||
		!strings.Contains(closedDetail, "余额事实已逐项复核并完成结清") ||
		strings.Contains(closedDetail, `action="/obligations/`+createdObligationID+`/close"`) ||
		strings.Contains(closedDetail, `action="/obligations/`+createdObligationID+`/adjust-amount"`) ||
		strings.Contains(closedDetail, `action="/obligations/`+createdObligationID+`/remissions"`) {
		t.Fatalf("closed obligation detail = %q", closedDetail)
	}
	agingPage = getPage(t, client, server.URL+"/aging?as_of_date=2099-12-31")
	if strings.Contains(agingPage, "系统联调九月服务费") || !strings.Contains(agingPage, "当前没有状态为 open 且剩余金额大于 0 的义务") {
		t.Fatalf("closed obligation should be excluded from aging = %q", agingPage)
	}

	response, err = client.PostForm(server.URL+"/obligations/"+createdObligationID+"/adjust-amount", url.Values{
		"csrf_token": {hiddenCSRF(t, closedDetail)}, "command_id": {"b0000000-0000-4000-8000-000000000001"},
		"expected_current_revision": {"5"}, "adjustment_type": {"increase"},
		"adjustment_amount": {"1"}, "adjustment_reason": {"关闭后不得调额"},
	})
	if err != nil {
		t.Fatalf("POST adjustment after closure: %v", err)
	}
	closedAdjustmentBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(closedAdjustmentBody, "只有 open 状态的义务可以调整金额") ||
		!strings.Contains(closedAdjustmentBody, "余额事实已逐项复核并完成结清") {
		t.Fatalf("adjustment after closure status=%d body=%q", response.StatusCode, closedAdjustmentBody)
	}
	closeResponse(t, response)

	response, err = client.PostForm(server.URL+"/obligations/"+createdObligationID+"/remissions", url.Values{
		"csrf_token": {hiddenCSRF(t, closedAdjustmentBody)}, "command_id": {"b0000000-0000-4000-8000-000000000002"},
		"expected_current_revision": {"5"}, "remission_amount": {"1"},
		"remission_reason": {"关闭后不得减免"}, "manual_source_reference": {"关闭终态测试"},
	})
	if err != nil {
		t.Fatalf("POST remission after closure: %v", err)
	}
	closedRemissionBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(closedRemissionBody, "只有 open 状态的义务可以记录减免") ||
		!strings.Contains(closedRemissionBody, "余额事实已逐项复核并完成结清") {
		t.Fatalf("remission after closure status=%d body=%q", response.StatusCode, closedRemissionBody)
	}
	closeResponse(t, response)

	response, err = client.PostForm(server.URL+"/obligations/"+createdObligationID+"/close", url.Values{
		"csrf_token": {hiddenCSRF(t, closedRemissionBody)}, "command_id": {"b0000000-0000-4000-8000-000000000003"},
		"expected_current_revision": {"5"}, "expected_balance_sequence": {balanceSequence},
		"closure_reason": {"关闭后不得再次关闭"},
	})
	if err != nil {
		t.Fatalf("POST second closure: %v", err)
	}
	secondClosureBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(secondClosureBody, "只有 open 状态的义务可以关闭") ||
		!strings.Contains(secondClosureBody, "余额事实已逐项复核并完成结清") {
		t.Fatalf("second closure status=%d body=%q", response.StatusCode, secondClosureBody)
	}
	closeResponse(t, response)

	closedAllocationDetail := getPage(t, client, server.URL+"/allocations/"+remissionAllocationID)
	if strings.Contains(closedAllocationDetail, "确认完整反向核销") {
		t.Fatalf("closed obligation allocation still offers reversal: %q", closedAllocationDetail)
	}
	response, err = client.PostForm(server.URL+"/allocations/"+remissionAllocationID+"/reverse", url.Values{
		"csrf_token": {hiddenCSRF(t, closedAllocationDetail)}, "command_id": {"b0000000-0000-4000-8000-000000000004"},
		"reason": {"关闭后不得反向"},
	})
	if err != nil {
		t.Fatalf("POST allocation reversal after closure: %v", err)
	}
	closedReversalBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(closedReversalBody, "只有 open 状态的义务可以反向核销资金分配") ||
		strings.Contains(closedReversalBody, "确认完整反向核销") {
		t.Fatalf("allocation reversal after closure status=%d body=%q", response.StatusCode, closedReversalBody)
	}
	closeResponse(t, response)

	var closureStatus, closureRevision, closureSequence string
	var closureEffective, closureAllocated, closureRemitted, closureCurrency, closureBasis, closureReason string
	var closureActorKind, closureActorID, closureActorName string
	var closureCount, closureCommandCount, closureAuditCount int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			o.status,
			c.obligation_revision::text,
			c.balance_sequence::text,
			c.recorded_effective_amount::text,
			c.net_allocated_amount::text,
			c.remitted_amount::text,
			c.currency,
			c.settlement_basis,
			c.closure_reason,
			c.closed_by_actor_kind,
			c.closed_by_actor_id,
			c.closed_by_actor_display_name,
			(SELECT count(*) FROM obligation_closures WHERE obligation_id = o.id),
			(SELECT count(*) FROM obligation_closure_commands WHERE obligation_id = o.id),
			(SELECT count(*) FROM audit_entries WHERE object_type = 'obligation' AND object_id = o.id AND action = 'obligation_closed')
		FROM obligations o
		JOIN obligation_closures c ON c.obligation_id = o.id
		WHERE o.id = $1
	`, createdObligationID).Scan(
		&closureStatus, &closureRevision, &closureSequence,
		&closureEffective, &closureAllocated, &closureRemitted, &closureCurrency, &closureBasis, &closureReason,
		&closureActorKind, &closureActorID, &closureActorName,
		&closureCount, &closureCommandCount, &closureAuditCount,
	); err != nil {
		t.Fatalf("read obligation closure HTTP facts: %v", err)
	}
	if closureStatus != "closed" || closureRevision != "5" || closureSequence != balanceSequence ||
		closureEffective != "110.000000" || closureAllocated != "20.000000" || closureRemitted != "90.000000" ||
		closureCurrency != "CNY" || closureBasis != "mixed" || closureReason != "余额事实已逐项复核并完成结清" ||
		closureActorKind != "local_user" || closureActorID != actualUserID || closureActorName != "系统联调管理员" ||
		closureCount != 1 || closureCommandCount != 1 || closureAuditCount != 1 {
		t.Fatalf("obligation closure HTTP facts status=%s revision=%s sequence=%s amounts=%s/%s/%s currency=%s basis=%s reason=%q actor=%s/%s/%s counts=%d/%d/%d",
			closureStatus, closureRevision, closureSequence,
			closureEffective, closureAllocated, closureRemitted, closureCurrency, closureBasis, closureReason,
			closureActorKind, closureActorID, closureActorName,
			closureCount, closureCommandCount, closureAuditCount)
	}

	var confirmationRecordedAt, firstAllocationAppliedAt, firstReversalRecordedAt, closureRecordedAt time.Time
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT recorded_at FROM obligation_revisions WHERE obligation_id = $1 AND revision = 1),
			(SELECT applied_at FROM allocations WHERE id = $2),
			(SELECT reversed_at FROM allocation_reversals WHERE allocation_id = $2),
			(SELECT closed_at FROM obligation_closures WHERE obligation_id = $1)
	`, createdObligationID, createdAllocationID).Scan(
		&confirmationRecordedAt, &firstAllocationAppliedAt, &firstReversalRecordedAt, &closureRecordedAt,
	); err != nil {
		t.Fatalf("read historical balance HTTP checkpoints: %v", err)
	}
	response, err = client.Get(server.URL + "/obligations/" + createdObligationID + "/history?as_of=" +
		url.QueryEscape(confirmationRecordedAt.Add(-time.Microsecond).UTC().Format(time.RFC3339Nano)))
	if err != nil {
		t.Fatalf("GET historical balance before confirmation: %v", err)
	}
	beforeConfirmationBody := readResponse(t, response)
	if response.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(beforeConfirmationBody, "该时刻义务尚未确认") {
		t.Fatalf("historical balance before confirmation status=%d body=%q", response.StatusCode, beforeConfirmationBody)
	}
	closeResponse(t, response)

	assertHistoricalBalancePage(t, client, server.URL, createdObligationID, confirmationRecordedAt,
		"Revision</dt><dd>1", "100.5 − 0 − 0 = 100.5 CNY", "历史余额事实序号</dt><dd>0", "open")
	assertHistoricalBalancePage(t, client, server.URL, createdObligationID, firstAllocationAppliedAt,
		"Revision</dt><dd>1", "100.5 − 60.25 − 0 = 40.25 CNY", "历史余额事实序号</dt><dd>1", "open")
	assertHistoricalBalancePage(t, client, server.URL, createdObligationID, firstReversalRecordedAt,
		"Revision</dt><dd>1", "100.5 − 0 − 0 = 100.5 CNY", "历史余额事实序号</dt><dd>2", "open")
	assertHistoricalBalancePage(t, client, server.URL, createdObligationID, closureRecordedAt,
		"Revision</dt><dd>5", "110 − 20 − 90 = 0 CNY", "历史余额事实序号</dt><dd>"+balanceSequence,
		"closed", "资金分配与减免组合归零")

	operatingReport, err := operatingSummaryService.Current(ctx)
	if err != nil {
		t.Fatalf("read operating summary after full HTTP flow: %v", err)
	}
	var operatingGroup *operatingsummary.Group
	for index := range operatingReport.Groups {
		if operatingReport.Groups[index].LedgerOwnerPartyID == createdPartyID && operatingReport.Groups[index].Currency == "CNY" {
			operatingGroup = &operatingReport.Groups[index]
			break
		}
	}
	if operatingGroup == nil || operatingGroup.ReceivableOutstanding.Amount != "0" ||
		operatingGroup.ReceivableOutstanding.FactCount != 0 || operatingGroup.ReceiptEffective.FactCount == 0 {
		t.Fatalf("operating group after closure = %#v", operatingGroup)
	}
	operatingPage := getPage(t, client, server.URL+"/operating-summary")
	if !strings.Contains(operatingPage, "当前经营收支汇总") ||
		!strings.Contains(operatingPage, "系统联调供应商") ||
		!strings.Contains(operatingPage, operatingGroup.ReceiptEffective.Amount+" CNY") ||
		!strings.Contains(operatingPage, "metric=receipt_effective") ||
		strings.Contains(operatingPage, "metric=receivable_outstanding") {
		t.Fatalf("operating summary page = %q", operatingPage)
	}
	response, err = client.Get(server.URL + "/operating-summary/details?metric=receipt_effective&ledger_owner_party_id=" +
		url.QueryEscape(createdPartyID) + "&currency=CNY")
	if err != nil {
		t.Fatalf("GET operating receipt drilldown: %v", err)
	}
	operatingDetailBody := readResponse(t, response)
	if response.StatusCode != http.StatusOK || !strings.Contains(operatingDetailBody, "当前有效实际收款下钻") ||
		!strings.Contains(operatingDetailBody, operatingGroup.ReceiptEffective.Amount+" CNY") ||
		strings.Contains(operatingDetailBody, createdCashTransactionID) {
		t.Fatalf("operating receipt drilldown status=%d body=%q", response.StatusCode, operatingDetailBody)
	}
	closeResponse(t, response)
	response, err = client.Get(server.URL + "/operating-summary/details?metric=receivable_outstanding&ledger_owner_party_id=" +
		url.QueryEscape(createdPartyID) + "&currency=CNY")
	if err != nil {
		t.Fatalf("GET empty receivable drilldown: %v", err)
	}
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("empty receivable drilldown status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	closeResponse(t, response)

	exportPage := getPage(t, client, server.URL+"/ledger-exports")
	if !strings.Contains(exportPage, "本方主体可校验台账导出") ||
		!strings.Contains(exportPage, createdPartyID) || !strings.Contains(exportPage, "按文本导入") {
		t.Fatalf("owner ledger export page = %q", exportPage)
	}
	response, err = client.PostForm(server.URL+"/ledger-exports", url.Values{
		"csrf_token":            {hiddenCSRF(t, exportPage)},
		"ledger_owner_party_id": {createdPartyID},
	})
	if err != nil {
		t.Fatalf("POST owner ledger export: %v", err)
	}
	archiveBytes, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read owner ledger export: %v", err)
	}
	closeResponse(t, response)
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "application/zip" ||
		response.Header.Get("Cache-Control") != "no-store" ||
		!strings.Contains(response.Header.Get("Content-Disposition"), "money-flow-ledger-"+createdPartyID+".zip") {
		t.Fatalf("owner ledger export status=%d headers=%#v body=%q", response.StatusCode, response.Header, archiveBytes)
	}
	exportFiles := readOwnerLedgerExportArchive(t, archiveBytes)
	if len(exportFiles) != 14 {
		t.Fatalf("owner ledger export file count = %d", len(exportFiles))
	}
	var exportManifest ledgerexport.Manifest
	if err := json.Unmarshal(exportFiles["manifest.json"], &exportManifest); err != nil {
		t.Fatalf("decode owner ledger manifest: %v", err)
	}
	if exportManifest.FormatID != ledgerexport.FormatID || exportManifest.FormatVersion != ledgerexport.FormatVersion ||
		exportManifest.LedgerOwner.ID != createdPartyID ||
		exportManifest.SourceSchemaVersion != "000025_payroll_payments.sql" || len(exportManifest.Files) != 13 {
		t.Fatalf("owner ledger manifest = %#v", exportManifest)
	}
	for _, file := range exportManifest.Files {
		content, ok := exportFiles[file.Filename]
		if !ok {
			t.Fatalf("manifest file %s is missing", file.Filename)
		}
		digest := sha256.Sum256(content)
		if file.Bytes != len(content) || file.SHA256 != hex.EncodeToString(digest[:]) {
			t.Fatalf("manifest file mismatch for %s: %#v", file.Filename, file)
		}
	}
	if !csvArchiveContains(t, exportFiles["obligations.csv"], createdObligationID) ||
		!csvArchiveContains(t, exportFiles["obligation_cash_expectations.csv"], "2099-10-20") ||
		!csvArchiveContains(t, exportFiles["obligation_cash_expectations.csv"], "预计日期重新待定") ||
		!csvArchiveContains(t, exportFiles["cash_transactions.csv"], createdCashTransactionID) ||
		!csvArchiveContains(t, exportFiles["allocations.csv"], createdAllocationID) ||
		!csvArchiveContains(t, exportFiles["allocation_reversals.csv"], createdAllocationID) {
		t.Fatal("owner ledger export is missing the full HTTP flow facts")
	}
	if strings.Contains(string(exportFiles["audit_entries.csv"]), ledgerexport.ActionMoneyFlowLedgerExported) {
		t.Fatal("owner ledger export contains its own export audit")
	}
	var ownerExportAudits int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT count(*) FROM audit_entries
		WHERE object_type = 'ledger_export' AND object_id = $1 AND action = $2
	`, createdPartyID, ledgerexport.ActionMoneyFlowLedgerExported).Scan(&ownerExportAudits); err != nil {
		t.Fatalf("count owner ledger export audits: %v", err)
	}
	if ownerExportAudits != 1 {
		t.Fatalf("owner ledger export audit count = %d", ownerExportAudits)
	}

	response, err = client.PostForm(server.URL+"/logout", url.Values{"csrf_token": {hiddenCSRF(t, homePage)}})
	if err != nil {
		t.Fatalf("POST logout: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/login" {
		t.Fatalf("logout status=%d location=%q", response.StatusCode, response.Header.Get("Location"))
	}
	closeResponse(t, response)
	var revokedSessions int
	if err := environment.Pool.QueryRow(ctx, `SELECT count(*) FROM local_sessions WHERE revoked_at IS NOT NULL`).Scan(&revokedSessions); err != nil {
		t.Fatalf("count revoked sessions: %v", err)
	}
	if revokedSessions != 1 {
		t.Fatalf("revoked sessions = %d", revokedSessions)
	}
	response, err = client.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET home after logout: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/login?next=%2F" {
		t.Fatalf("home after logout status=%d location=%q", response.StatusCode, response.Header.Get("Location"))
	}
	closeResponse(t, response)
}

func TestMoneyFlowHTTPInternalTransferIsNonOperatingAgainstPostgreSQL(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	identityService, err := identity.NewService(identity.NewPostgresRepository(environment.Pool), time.Hour)
	if err != nil {
		t.Fatalf("identity.NewService() error = %v", err)
	}
	created, err := identityService.EnsureBootstrap(ctx, identity.BootstrapInput{
		Username: "transfer-admin", DisplayName: "内部转账联调管理员", Password: "transfer-admin-password",
	})
	if err != nil || !created {
		t.Fatalf("EnsureBootstrap() = %v, %v", created, err)
	}
	var userID string
	if err := environment.Pool.QueryRow(ctx, `SELECT id::text FROM local_users WHERE username = 'transfer-admin'`).Scan(&userID); err != nil {
		t.Fatalf("read bootstrap user: %v", err)
	}
	actor := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: userID, DisplayName: "内部转账联调管理员"}

	partyService := party.NewService(party.NewPostgresRepository(environment.Pool))
	owner, err := partyService.Create(ctx, party.CreateInput{
		CommandID: "12000000-0000-4000-8000-000000000001", Kind: "organization",
		LegalName: "内部转账本方", RegistrationIdentifier: "TRANSFER-OWNER", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	counterparty, err := partyService.Create(ctx, party.CreateInput{
		CommandID: "12000000-0000-4000-8000-000000000002", Kind: "organization",
		LegalName: "经营往来方", RegistrationIdentifier: "OPERATING-COUNTERPARTY", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create counterparty: %v", err)
	}
	cashTransactionService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	for _, input := range []cashtransaction.CreateInput{
		{
			CommandID: "13000000-0000-4000-8000-000000000001", Type: "receipt",
			LedgerOwnerPartyID: owner.Party.ID, CounterpartyPartyID: counterparty.Party.ID,
			Description: "经营收款", Amount: "125.5", Currency: "CNY", OccurredOn: "2026-09-18",
			AccountReference: "基本户", ManualSourceReference: "经营收款回单", Actor: actor,
		},
		{
			CommandID: "13000000-0000-4000-8000-000000000002", Type: "payment",
			LedgerOwnerPartyID: owner.Party.ID, CounterpartyPartyID: counterparty.Party.ID,
			Description: "经营付款", Amount: "25.25", Currency: "CNY", OccurredOn: "2026-09-18",
			AccountReference: "基本户", ManualSourceReference: "经营付款回单", Actor: actor,
		},
	} {
		if _, err := cashTransactionService.Create(ctx, input); err != nil {
			t.Fatalf("create cash fixture: %v", err)
		}
	}

	obligationService := obligation.NewService(obligation.NewPostgresRepository(environment.Pool))
	allocationService := allocation.NewService(allocation.NewPostgresRepository(environment.Pool))
	operatingSummaryService := operatingsummary.NewService(operatingsummary.NewPostgresRepository(environment.Pool))
	ledgerExportService, err := ledgerexport.NewService(ledgerexport.NewPostgresRepository(environment.Pool))
	if err != nil {
		t.Fatalf("ledgerexport.NewService() error = %v", err)
	}
	internalTransferService := internaltransfer.NewService(internaltransfer.NewPostgresRepository(environment.Pool))
	handler, err := httpserver.New(
		environment.Pool, identityService, partyService, obligationService, cashTransactionService, allocationService,
		operatingSummaryService, ledgerExportService, httpserver.Options{}, internalTransferService,
	)
	if err != nil {
		t.Fatalf("httpserver.New() error = %v", err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error = %v", err)
	}
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	for _, healthPath := range []string{"/health/live", "/health/ready"} {
		response, err := client.Get(server.URL + healthPath)
		if err != nil {
			t.Fatalf("GET %s: %v", healthPath, err)
		}
		if response.StatusCode != http.StatusOK {
			t.Fatalf("GET %s status=%d body=%q", healthPath, response.StatusCode, readResponse(t, response))
		}
		closeResponse(t, response)
	}

	loginPage := getPage(t, client, server.URL+"/login")
	response, err := client.PostForm(server.URL+"/login", url.Values{
		"csrf_token": {hiddenCSRF(t, loginPage)}, "next": {"/internal-transfers"},
		"username": {"transfer-admin"}, "password": {"transfer-admin-password"},
	})
	if err != nil {
		t.Fatalf("POST login: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/internal-transfers" {
		t.Fatalf("login status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	transferPage := getPage(t, client, server.URL+"/internal-transfers")
	csrf := hiddenCSRF(t, transferPage)
	commandID := hiddenCommand(t, transferPage)
	values := url.Values{
		"csrf_token": {csrf}, "command_id": {commandID}, "ledger_owner_party_id": {owner.Party.ID},
		"description": {" 基本户向备用金账户转账 "}, "amount": {"100.50"}, "currency": {"cny"},
		"occurred_on": {"2026-09-19"}, "source_account_reference": {"基本户"},
		"destination_account_reference": {"备用金账户"}, "manual_source_reference": {"银行回单 TRANSFER-HTTP-001"},
		"external_transfer_reference": {"BANK-TRANSFER-HTTP-001"},
		"actor_kind":                  {"forged"}, "actor_id": {"forged"}, "actor_display_name": {"伪造用户"},
	}
	response, err = client.PostForm(server.URL+"/internal-transfers", values)
	if err != nil {
		t.Fatalf("POST internal transfer: %v", err)
	}
	location, err := url.Parse(response.Header.Get("Location"))
	if response.StatusCode != http.StatusSeeOther || err != nil || !strings.HasPrefix(location.Path, "/internal-transfers/") || location.Query().Get("created") != "1" {
		t.Fatalf("create status=%d location=%q error=%v body=%q", response.StatusCode, response.Header.Get("Location"), err, readResponse(t, response))
	}
	transferID := strings.TrimPrefix(location.Path, "/internal-transfers/")
	closeResponse(t, response)

	response, err = client.PostForm(server.URL+"/internal-transfers", values)
	if err != nil {
		t.Fatalf("POST internal transfer replay: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || !strings.Contains(response.Header.Get("Location"), transferID) {
		t.Fatalf("replay status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	changed := url.Values{}
	for key, entries := range values {
		changed[key] = append([]string(nil), entries...)
	}
	changed.Set("amount", "101")
	response, err = client.PostForm(server.URL+"/internal-transfers", changed)
	if err != nil {
		t.Fatalf("POST changed internal transfer replay: %v", err)
	}
	changedBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(changedBody, "不同的内部账户转账登记意图") {
		t.Fatalf("changed replay status=%d body=%q", response.StatusCode, changedBody)
	}
	closeResponse(t, response)

	duplicate := url.Values{}
	for key, entries := range values {
		duplicate[key] = append([]string(nil), entries...)
	}
	duplicate.Set("command_id", hiddenCommand(t, changedBody))
	response, err = client.PostForm(server.URL+"/internal-transfers", duplicate)
	if err != nil {
		t.Fatalf("POST duplicate external transfer: %v", err)
	}
	duplicateBody := readResponse(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(duplicateBody, "相同的外部转账参考") {
		t.Fatalf("duplicate status=%d body=%q", response.StatusCode, duplicateBody)
	}
	closeResponse(t, response)

	detail := getPage(t, client, server.URL+"/internal-transfers/"+transferID+"?created=1")
	if !strings.Contains(detail, "内部转账已登记") || !strings.Contains(detail, "基本户向备用金账户转账") ||
		!strings.Contains(detail, "100.5 CNY") || !strings.Contains(detail, "不参与义务核销") ||
		!strings.Contains(detail, "内部转账本方") || !strings.Contains(detail, "BANK-TRANSFER-HTTP-001") {
		t.Fatalf("internal transfer detail = %q", detail)
	}
	list := getPage(t, client, server.URL+"/internal-transfers")
	if !strings.Contains(list, "基本户向备用金账户转账") || !strings.Contains(list, "基本户") || !strings.Contains(list, "备用金账户") {
		t.Fatalf("internal transfer list = %q", list)
	}

	var transfers, commands, audits, cashTransactions, allocations int
	var receiptTotal, paymentTotal, actorKind, actorID, actorName, classification string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM internal_transfers),
			(SELECT count(*) FROM internal_transfer_create_commands),
			(SELECT count(*) FROM audit_entries WHERE action = 'internal_transfer_recorded'),
			(SELECT count(*) FROM cash_transactions),
			(SELECT count(*) FROM allocations),
			(SELECT sum(amount)::text FROM cash_transactions WHERE type = 'receipt'),
			(SELECT sum(amount)::text FROM cash_transactions WHERE type = 'payment'),
			a.actor_kind, a.actor_id, a.actor_display_name,
			a.details->>'operating_cash_flow_classification'
		FROM audit_entries a
		WHERE a.action = 'internal_transfer_recorded' AND a.object_id = $1
	`, transferID).Scan(
		&transfers, &commands, &audits, &cashTransactions, &allocations,
		&receiptTotal, &paymentTotal, &actorKind, &actorID, &actorName, &classification,
	); err != nil {
		t.Fatalf("read internal transfer HTTP facts: %v", err)
	}
	if transfers != 1 || commands != 1 || audits != 1 || cashTransactions != 2 || allocations != 0 ||
		receiptTotal != "125.500000" || paymentTotal != "25.250000" ||
		actorKind != actor.Kind || actorID != actor.ID || actorName != actor.DisplayName || classification != "excluded_internal_transfer" {
		t.Fatalf("HTTP facts transfers=%d commands=%d audits=%d cash=%d allocations=%d totals=%s/%s actor=%s/%s/%s class=%s",
			transfers, commands, audits, cashTransactions, allocations, receiptTotal, paymentTotal,
			actorKind, actorID, actorName, classification)
	}
}

func TestMoneyFlowHTTPMultiAllocationCompositionAgainstPostgreSQL(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	identityService, err := identity.NewService(identity.NewPostgresRepository(environment.Pool), time.Hour)
	if err != nil {
		t.Fatalf("identity.NewService() error = %v", err)
	}
	created, err := identityService.EnsureBootstrap(ctx, identity.BootstrapInput{
		Username: "composition-admin", DisplayName: "组合核销管理员", Password: "composition-admin-password",
	})
	if err != nil || !created {
		t.Fatalf("EnsureBootstrap() = %v, %v", created, err)
	}
	var userID string
	if err := environment.Pool.QueryRow(ctx, `SELECT id::text FROM local_users WHERE username = 'composition-admin'`).Scan(&userID); err != nil {
		t.Fatalf("read bootstrap user: %v", err)
	}
	actor := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: userID, DisplayName: "组合核销管理员"}

	partyService := party.NewService(party.NewPostgresRepository(environment.Pool))
	owner, err := partyService.Create(ctx, party.CreateInput{
		CommandID: "11000000-0000-4000-8000-000000000001", Kind: "organization",
		LegalName: "组合核销本方", RegistrationIdentifier: "COMPOSITION-OWNER", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	counterparty, err := partyService.Create(ctx, party.CreateInput{
		CommandID: "11000000-0000-4000-8000-000000000002", Kind: "organization",
		LegalName: "组合核销客户", RegistrationIdentifier: "COMPOSITION-CUSTOMER", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create counterparty: %v", err)
	}

	obligationService := obligation.NewService(obligation.NewPostgresRepository(environment.Pool))
	createObligation := func(draftCommandID, confirmCommandID, title, amount string) obligation.Obligation {
		t.Helper()
		draft, createErr := obligationService.CreateDraft(ctx, obligation.CreateDraftInput{
			CommandID: draftCommandID, Direction: "receivable",
			LedgerOwnerPartyID: owner.Party.ID, CounterpartyPartyID: counterparty.Party.ID,
			Title: title, OriginalAmount: amount, Currency: "CNY", DueOn: "2026-10-31",
			ManualSourceReference: title + " 人工依据", Actor: actor,
		})
		if createErr != nil {
			t.Fatalf("create obligation %s: %v", title, createErr)
		}
		confirmed, confirmErr := obligationService.Confirm(ctx, obligation.ConfirmInput{
			CommandID: confirmCommandID, ObligationID: draft.Obligation.ID,
			ExpectedDraftVersion: draft.Obligation.DraftVersion, Actor: actor,
		})
		if confirmErr != nil {
			t.Fatalf("confirm obligation %s: %v", title, confirmErr)
		}
		return confirmed.Obligation
	}
	obligationX := createObligation(
		"21000000-0000-4000-8000-000000000001", "22000000-0000-4000-8000-000000000001", "组合义务 X", "100",
	)
	obligationY := createObligation(
		"21000000-0000-4000-8000-000000000002", "22000000-0000-4000-8000-000000000002", "组合义务 Y", "80",
	)

	cashTransactionService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	createCash := func(commandID, description, amount, externalReference string) cashtransaction.Transaction {
		t.Helper()
		result, createErr := cashTransactionService.Create(ctx, cashtransaction.CreateInput{
			CommandID: commandID, Type: "receipt",
			LedgerOwnerPartyID: owner.Party.ID, CounterpartyPartyID: counterparty.Party.ID,
			Description: description, Amount: amount, Currency: "CNY", OccurredOn: "2026-09-19",
			AccountReference: "组合核销测试账户", ManualSourceReference: description + " 人工回单",
			ExternalTransactionReference: externalReference, Actor: actor,
		})
		if createErr != nil {
			t.Fatalf("create cash %s: %v", description, createErr)
		}
		return result.Transaction
	}
	cashA := createCash("31000000-0000-4000-8000-000000000001", "组合资金 A", "120", "COMPOSITION-CASH-A")
	cashB := createCash("31000000-0000-4000-8000-000000000002", "组合资金 B", "80", "COMPOSITION-CASH-B")

	allocationService := allocation.NewService(allocation.NewPostgresRepository(environment.Pool))
	operatingSummaryService := operatingsummary.NewService(operatingsummary.NewPostgresRepository(environment.Pool))
	ledgerExportService, err := ledgerexport.NewService(ledgerexport.NewPostgresRepository(environment.Pool))
	if err != nil {
		t.Fatalf("ledgerexport.NewService() error = %v", err)
	}
	internalTransferService := internaltransfer.NewService(internaltransfer.NewPostgresRepository(environment.Pool))
	handler, err := httpserver.New(
		environment.Pool, identityService, partyService, obligationService, cashTransactionService, allocationService, operatingSummaryService, ledgerExportService, httpserver.Options{}, internalTransferService,
	)
	if err != nil {
		t.Fatalf("httpserver.New() error = %v", err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error = %v", err)
	}
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	loginPage := getPage(t, client, server.URL+"/login")
	response, err := client.PostForm(server.URL+"/login", url.Values{
		"csrf_token": {hiddenCSRF(t, loginPage)}, "next": {"/"},
		"username": {"composition-admin"}, "password": {"composition-admin-password"},
	})
	if err != nil {
		t.Fatalf("POST login: %v", err)
	}
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/" {
		t.Fatalf("login status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	apply := func(cashTransactionID, obligationID, amount, sourceReference string) string {
		t.Helper()
		page := getPage(t, client, server.URL+"/allocations")
		response, postErr := client.PostForm(server.URL+"/allocations", url.Values{
			"csrf_token": {hiddenCSRF(t, page)}, "command_id": {hiddenCommand(t, page)},
			"cash_transaction_id": {cashTransactionID}, "obligation_selection": {obligationID + ":1"},
			"amount": {amount}, "manual_source_reference": {sourceReference},
		})
		if postErr != nil {
			t.Fatalf("POST allocation %s: %v", sourceReference, postErr)
		}
		location, parseErr := url.Parse(response.Header.Get("Location"))
		if response.StatusCode != http.StatusSeeOther || parseErr != nil || !strings.HasPrefix(location.Path, "/allocations/") {
			t.Fatalf("allocation %s status=%d location=%q error=%v body=%q",
				sourceReference, response.StatusCode, response.Header.Get("Location"), parseErr, readResponse(t, response))
		}
		closeResponse(t, response)
		return strings.TrimPrefix(location.Path, "/allocations/")
	}
	allocationAX := apply(cashA.ID, obligationX.ID, "40", "组合 A→X")
	allocationAY := apply(cashA.ID, obligationY.ID, "80", "组合 A→Y")
	allocationBX := apply(cashB.ID, obligationX.ID, "60", "组合 B→X")

	assertContainsAll := func(name, page string, expected ...string) {
		t.Helper()
		for _, value := range expected {
			if !strings.Contains(page, value) {
				t.Fatalf("%s does not contain %q: %q", name, value, page)
			}
		}
	}
	assertExcludesAll := func(name, page string, excluded ...string) {
		t.Helper()
		for _, value := range excluded {
			if strings.Contains(page, value) {
				t.Fatalf("%s unexpectedly contains %q: %q", name, value, page)
			}
		}
	}

	cashAPage := getPage(t, client, server.URL+"/cash-transactions/"+cashA.ID)
	assertContainsAll("cash A", cashAPage,
		"已分配金额</dt><dd>120 CNY", "未分配金额</dt><dd>0 CNY", "组合义务 X", "组合义务 Y",
		"组合 A→X", "组合 A→Y", allocationAX, allocationAY,
	)
	assertExcludesAll("cash A", cashAPage, "组合 B→X", allocationBX)
	cashBPage := getPage(t, client, server.URL+"/cash-transactions/"+cashB.ID)
	assertContainsAll("cash B", cashBPage,
		"已分配金额</dt><dd>60 CNY", "未分配金额</dt><dd>20 CNY", "组合义务 X", "组合 B→X", allocationBX,
	)
	assertExcludesAll("cash B", cashBPage, "组合 A→X", "组合 A→Y", allocationAX, allocationAY)

	obligationXPage := getPage(t, client, server.URL+"/obligations/"+obligationX.ID)
	assertContainsAll("obligation X", obligationXPage,
		"状态</dt><dd>open", "已分配金额</dt><dd>100 CNY", "剩余金额</dt><dd>0 CNY",
		"组合资金 A", "组合资金 B", "组合 A→X", "组合 B→X", allocationAX, allocationBX,
	)
	assertExcludesAll("obligation X", obligationXPage, "组合 A→Y", allocationAY)
	obligationYPage := getPage(t, client, server.URL+"/obligations/"+obligationY.ID)
	assertContainsAll("obligation Y", obligationYPage,
		"状态</dt><dd>open", "已分配金额</dt><dd>80 CNY", "剩余金额</dt><dd>0 CNY",
		"组合资金 A", "组合 A→Y", allocationAY,
	)
	assertExcludesAll("obligation Y", obligationYPage, "组合 A→X", "组合 B→X", allocationAX, allocationBX)

	for allocationID, sourceReference := range map[string]string{
		allocationAX: "组合 A→X", allocationAY: "组合 A→Y", allocationBX: "组合 B→X",
	} {
		detail := getPage(t, client, server.URL+"/allocations/"+allocationID)
		assertContainsAll("allocation detail", detail, allocationID, sourceReference, "组合核销管理员")
	}
	allocationPage := getPage(t, client, server.URL+"/allocations")
	assertContainsAll("allocation list", allocationPage, "组合 A→X", "组合 A→Y", "组合 B→X")

	var allocationCount, commandCount, auditCount int
	var allocationTotal, cashAAllocated, cashAUnallocated, cashBAllocated, cashBUnallocated string
	var obligationXAllocated, obligationXOutstanding, obligationYAllocated, obligationYOutstanding string
	var obligationXStatus, obligationYStatus string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM allocations),
			(SELECT count(*) FROM allocation_apply_commands),
			(SELECT count(*) FROM audit_entries WHERE action = 'allocation_applied'),
			(SELECT COALESCE(SUM(amount), 0)::text FROM allocations),
			(SELECT COALESCE(SUM(amount), 0)::text FROM allocations WHERE cash_transaction_id = $1),
			(SELECT (amount - COALESCE((SELECT SUM(amount) FROM allocations WHERE cash_transaction_id = c.id), 0))::text FROM cash_transactions c WHERE id = $1),
			(SELECT COALESCE(SUM(amount), 0)::text FROM allocations WHERE cash_transaction_id = $2),
			(SELECT (amount - COALESCE((SELECT SUM(amount) FROM allocations WHERE cash_transaction_id = c.id), 0))::text FROM cash_transactions c WHERE id = $2),
			(SELECT COALESCE(SUM(amount), 0)::text FROM allocations WHERE obligation_id = $3),
			(SELECT (r.effective_amount - COALESCE((SELECT SUM(amount) FROM allocations WHERE obligation_id = r.obligation_id), 0))::text FROM obligation_revisions r WHERE r.obligation_id = $3 AND r.revision = 1),
			(SELECT COALESCE(SUM(amount), 0)::text FROM allocations WHERE obligation_id = $4),
			(SELECT (r.effective_amount - COALESCE((SELECT SUM(amount) FROM allocations WHERE obligation_id = r.obligation_id), 0))::text FROM obligation_revisions r WHERE r.obligation_id = $4 AND r.revision = 1),
			(SELECT status FROM obligations WHERE id = $3),
			(SELECT status FROM obligations WHERE id = $4)
	`, cashA.ID, cashB.ID, obligationX.ID, obligationY.ID).Scan(
		&allocationCount, &commandCount, &auditCount, &allocationTotal,
		&cashAAllocated, &cashAUnallocated, &cashBAllocated, &cashBUnallocated,
		&obligationXAllocated, &obligationXOutstanding, &obligationYAllocated, &obligationYOutstanding,
		&obligationXStatus, &obligationYStatus,
	); err != nil {
		t.Fatalf("read multi-allocation HTTP facts: %v", err)
	}
	if allocationCount != 3 || commandCount != 3 || auditCount != 3 || allocationTotal != "180.000000" ||
		cashAAllocated != "120.000000" || cashAUnallocated != "0.000000" ||
		cashBAllocated != "60.000000" || cashBUnallocated != "20.000000" ||
		obligationXAllocated != "100.000000" || obligationXOutstanding != "0.000000" ||
		obligationYAllocated != "80.000000" || obligationYOutstanding != "0.000000" ||
		obligationXStatus != obligation.StatusOpen || obligationYStatus != obligation.StatusOpen {
		t.Fatalf("composition HTTP facts counts=%d/%d/%d total=%s cash=%s/%s,%s/%s obligations=%s/%s/%s,%s/%s/%s",
			allocationCount, commandCount, auditCount, allocationTotal,
			cashAAllocated, cashAUnallocated, cashBAllocated, cashBUnallocated,
			obligationXAllocated, obligationXOutstanding, obligationXStatus,
			obligationYAllocated, obligationYOutstanding, obligationYStatus)
	}
}

func readOwnerLedgerExportArchive(t *testing.T, content []byte) map[string][]byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("open owner ledger export ZIP: %v", err)
	}
	result := make(map[string][]byte, len(reader.File))
	for _, file := range reader.File {
		if file.FileInfo().IsDir() || file.Mode()&0o777 != 0o600 {
			t.Fatalf("unsafe owner ledger ZIP entry %q mode=%o", file.Name, file.Mode())
		}
		if _, exists := result[file.Name]; exists {
			t.Fatalf("duplicate owner ledger ZIP entry %q", file.Name)
		}
		opened, err := file.Open()
		if err != nil {
			t.Fatalf("open owner ledger ZIP entry %q: %v", file.Name, err)
		}
		value, err := io.ReadAll(opened)
		_ = opened.Close()
		if err != nil {
			t.Fatalf("read owner ledger ZIP entry %q: %v", file.Name, err)
		}
		result[file.Name] = value
	}
	return result
}

func csvArchiveContains(t *testing.T, content []byte, expected string) bool {
	t.Helper()
	rows, err := csv.NewReader(bytes.NewReader(content)).ReadAll()
	if err != nil {
		t.Fatalf("parse owner ledger CSV: %v", err)
	}
	for _, row := range rows {
		for _, cell := range row {
			if cell == expected {
				return true
			}
		}
	}
	return false
}

func getPage(t *testing.T, client *http.Client, target string) string {
	t.Helper()
	response, err := client.Get(target)
	if err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status=%d body=%q", target, response.StatusCode, readResponse(t, response))
	}
	return readResponse(t, response)
}

func assertHistoricalBalancePage(
	t *testing.T,
	client *http.Client,
	serverURL, obligationID string,
	asOf time.Time,
	wants ...string,
) {
	t.Helper()
	target := serverURL + "/obligations/" + obligationID + "/history?as_of=" +
		url.QueryEscape(asOf.UTC().Format(time.RFC3339Nano))
	page := getPage(t, client, target)
	if !strings.Contains(page, "历史 Obligation 余额") ||
		!strings.Contains(page, `value="`+asOf.UTC().Format(time.RFC3339Nano)+`"`) {
		t.Fatalf("historical balance page at %s = %q", asOf.UTC().Format(time.RFC3339Nano), page)
	}
	for _, want := range wants {
		if !strings.Contains(page, want) {
			t.Fatalf("historical balance page at %s missing %q: %q", asOf.UTC().Format(time.RFC3339Nano), want, page)
		}
	}
}

func hiddenCSRF(t *testing.T, page string) string {
	t.Helper()
	match := csrfFieldPattern.FindStringSubmatch(page)
	if len(match) != 2 {
		t.Fatalf("CSRF field not found in %q", page)
	}
	return match[1]
}

func hiddenCommand(t *testing.T, page string) string {
	t.Helper()
	match := commandFieldPattern.FindStringSubmatch(page)
	if len(match) != 2 {
		t.Fatalf("command field not found in %q", page)
	}
	return match[1]
}

func hiddenBalanceSequence(t *testing.T, page string) string {
	t.Helper()
	match := balanceSequenceFieldPattern.FindStringSubmatch(page)
	if len(match) != 2 {
		t.Fatalf("expected balance sequence field not found in %q", page)
	}
	return match[1]
}

func readResponse(t *testing.T, response *http.Response) string {
	t.Helper()
	content, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	return string(content)
}

func closeResponse(t *testing.T, response *http.Response) {
	t.Helper()
	if err := response.Body.Close(); err != nil {
		t.Errorf("close response: %v", err)
	}
}
