//go:build integration

package httpserver_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/contract-ledger/internal/audit"
	"omsbizfoundry/contract-ledger/internal/contract"
	"omsbizfoundry/contract-ledger/internal/database"
	"omsbizfoundry/contract-ledger/internal/evidence"
	"omsbizfoundry/contract-ledger/internal/httpserver"
	"omsbizfoundry/contract-ledger/internal/identity"
	"omsbizfoundry/contract-ledger/internal/ledgerexport"
	"omsbizfoundry/contract-ledger/internal/ledgerrestore"
	"omsbizfoundry/contract-ledger/internal/party"
)

var csrfInputPattern = regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)

func TestEmptyDatabaseHTTPIdentityPermissionAuditAndRevisionFlow(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pool := openIsolatedSystemDatabase(t, ctx, databaseURL)

	identityService, err := identity.NewService(identity.NewPostgresRepository(pool), 2*time.Hour)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if _, err := identityService.EnsureBootstrap(ctx, identity.BootstrapInput{}); err == nil {
		t.Fatal("empty database started without bootstrap credentials")
	}
	created, err := identityService.EnsureBootstrap(ctx, identity.BootstrapInput{
		Username: "system-admin", DisplayName: "系统验收管理员", Password: "system-admin-password",
	})
	if err != nil || !created {
		t.Fatalf("EnsureBootstrap() created=%v error=%v", created, err)
	}
	created, err = identityService.EnsureBootstrap(ctx, identity.BootstrapInput{})
	if err != nil || created {
		t.Fatalf("repeated EnsureBootstrap() created=%v error=%v", created, err)
	}
	ledgerExportService, err := ledgerexport.NewService(ledgerexport.NewPostgresRepository(pool))
	if err != nil {
		t.Fatalf("NewService(ledger export) error = %v", err)
	}
	ledgerRestoreService, err := ledgerrestore.NewService(ledgerrestore.NewPostgresRepository(pool))
	if err != nil {
		t.Fatalf("NewService(ledger restore) error = %v", err)
	}

	handler, err := httpserver.NewServer(
		pool,
		party.NewService(party.NewPostgresRepository(pool)),
		contract.NewService(contract.NewPostgresRepository(pool)),
		newSystemEvidenceService(t, pool),
		ledgerExportService,
		ledgerRestoreService,
		identityService,
		audit.NewRepository(pool),
		httpserver.Options{},
	)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	testServer := httptest.NewServer(handler)
	t.Cleanup(testServer.Close)

	anonymous := newSessionClient(t)
	response := request(t, anonymous, http.MethodGet, testServer.URL+"/health/ready", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("ready status = %d", response.StatusCode)
	}
	closeResponse(t, response)
	response = request(t, anonymous, http.MethodGet, testServer.URL+"/parties", nil)
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/login?next=%2Fparties" {
		t.Fatalf("anonymous parties status=%d location=%q", response.StatusCode, response.Header.Get("Location"))
	}
	closeResponse(t, response)

	adminClient := newSessionClient(t)
	login(t, adminClient, testServer.URL, "system-admin", "system-admin-password")
	adminUsersPage := getPage(t, adminClient, testServer.URL+"/admin/users", http.StatusOK)
	createUserForm := url.Values{
		"csrf_token": {csrfToken(t, adminUsersPage)},
		"username":   {"system-editor"}, "display_name": {"系统验收编辑者"},
		"password": {"system-editor-password"}, "role": {"editor"},
	}
	response = postForm(t, adminClient, testServer.URL+"/admin/users", createUserForm)
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("create editor status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	closeResponse(t, response)
	adminUsersPage = getPage(t, adminClient, testServer.URL+"/admin/users", http.StatusOK)
	createUserForm = url.Values{
		"csrf_token": {csrfToken(t, adminUsersPage)},
		"username":   {"system-viewer"}, "display_name": {"系统验收查看者"},
		"password": {"system-viewer-password"}, "role": {"viewer"},
	}
	response = postForm(t, adminClient, testServer.URL+"/admin/users", createUserForm)
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("create viewer status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	closeResponse(t, response)

	var editorID, viewerID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM local_users WHERE username = 'system-editor'`).Scan(&editorID); err != nil {
		t.Fatalf("read editor ID: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT id::text FROM local_users WHERE username = 'system-viewer'`).Scan(&viewerID); err != nil {
		t.Fatalf("read viewer ID: %v", err)
	}

	editorClient := newSessionClient(t)
	login(t, editorClient, testServer.URL, "system-editor", "system-editor-password")
	partyPage := getPage(t, editorClient, testServer.URL+"/parties", http.StatusOK)
	response = postForm(t, editorClient, testServer.URL+"/parties", url.Values{
		"csrf_token": {csrfToken(t, partyPage)},
		"kind":       {"organization"}, "legal_name": {"端到端验收主体"},
		"registration_identifier": {"E2E-CL-M1-006"},
		"actor_kind":              {"forged"}, "actor_id": {"forged"},
	})
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("editor party create status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	closeResponse(t, response)

	var partyCount, auditCount int
	var actorKind, actorID, actorDisplayName string
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM parties WHERE legal_name = '端到端验收主体'`).Scan(&partyCount); err != nil {
		t.Fatalf("count created party: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*), min(actor_kind), min(actor_id), min(actor_display_name)
		FROM audit_entries
		WHERE action = 'party_created' AND object_id IN (
			SELECT id FROM parties WHERE legal_name = '端到端验收主体'
		)
	`).Scan(&auditCount, &actorKind, &actorID, &actorDisplayName); err != nil {
		t.Fatalf("read party audit: %v", err)
	}
	if partyCount != 1 || auditCount != 1 || actorKind != audit.ActorKindLocalUser || actorID != editorID || actorDisplayName != "系统验收编辑者" {
		t.Fatalf("party=%d audit=%d actor=%s:%s (%s), editor=%s", partyCount, auditCount, actorKind, actorID, actorDisplayName, editorID)
	}

	partyPage = getPage(t, editorClient, testServer.URL+"/parties", http.StatusOK)
	response = postForm(t, editorClient, testServer.URL+"/parties", url.Values{
		"csrf_token": {csrfToken(t, partyPage)},
		"kind":       {"organization"}, "legal_name": {"端到端验收主体乙"},
		"registration_identifier": {"E2E-CL-M2-001-B"},
	})
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("editor second party create status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	closeResponse(t, response)

	var firstPartyID, secondPartyID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM parties WHERE registration_identifier = 'E2E-CL-M1-006'`).Scan(&firstPartyID); err != nil {
		t.Fatalf("read first party ID: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT id::text FROM parties WHERE registration_identifier = 'E2E-CL-M2-001-B'`).Scan(&secondPartyID); err != nil {
		t.Fatalf("read second party ID: %v", err)
	}

	contractsPage := getPage(t, editorClient, testServer.URL+"/contracts", http.StatusOK)
	response = postForm(t, editorClient, testServer.URL+"/contracts", url.Values{
		"csrf_token":       {formHiddenValue(t, contractsPage, "/contracts", "csrf_token")},
		"command_id":       {formHiddenValue(t, contractsPage, "/contracts", "command_id")},
		"number_namespace": {"e2e"},
		"contract_number":  {"E2E-CL-M2-001"},
		"title":            {"端到端补充协议合同"},
		"category":         {"验收"},
		"currency":         {"CNY"},
		"total_amount":     {"1000.00"},
		"signed_on":        {"2026-09-19"},
		"effective_on":     {"2026-10-01"},
		"actor_kind":       {"forged"}, "actor_id": {"forged"},
	})
	if response.StatusCode != http.StatusSeeOther || !strings.HasPrefix(response.Header.Get("Location"), "/contracts?created=1&contract_id=") {
		t.Fatalf("editor contract create status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	var contractID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM contracts WHERE contract_number = 'E2E-CL-M2-001'`).Scan(&contractID); err != nil {
		t.Fatalf("read contract ID: %v", err)
	}
	contractPath := "/contracts/" + contractID
	addPartyPath := contractPath + "/parties"
	contractPage := getPage(t, editorClient, testServer.URL+contractPath, http.StatusOK)
	response = postForm(t, editorClient, testServer.URL+addPartyPath, url.Values{
		"csrf_token":             {formHiddenValue(t, contractPage, addPartyPath, "csrf_token")},
		"command_id":             {formHiddenValue(t, contractPage, addPartyPath, "command_id")},
		"expected_draft_version": {formHiddenValue(t, contractPage, addPartyPath, "expected_draft_version")},
		"party_id":               {firstPartyID},
		"role":                   {"委托方"},
	})
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != contractPath+"?party_added=1" {
		t.Fatalf("add first contract party status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	contractPage = getPage(t, editorClient, testServer.URL+contractPath, http.StatusOK)
	response = postForm(t, editorClient, testServer.URL+addPartyPath, url.Values{
		"csrf_token":             {formHiddenValue(t, contractPage, addPartyPath, "csrf_token")},
		"command_id":             {formHiddenValue(t, contractPage, addPartyPath, "command_id")},
		"expected_draft_version": {formHiddenValue(t, contractPage, addPartyPath, "expected_draft_version")},
		"party_id":               {secondPartyID},
		"role":                   {"受托方"},
	})
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != contractPath+"?party_added=1" {
		t.Fatalf("add second contract party status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	activatePath := contractPath + "/activate"
	contractPage = getPage(t, editorClient, testServer.URL+contractPath, http.StatusOK)
	response = postForm(t, editorClient, testServer.URL+activatePath, url.Values{
		"csrf_token":             {formHiddenValue(t, contractPage, activatePath, "csrf_token")},
		"command_id":             {formHiddenValue(t, contractPage, activatePath, "command_id")},
		"expected_draft_version": {formHiddenValue(t, contractPage, activatePath, "expected_draft_version")},
		"actor_kind":             {"forged"}, "actor_id": {"forged"},
	})
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != contractPath+"?activated=1" {
		t.Fatalf("activate contract status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	contractPage = getPage(t, editorClient, testServer.URL+contractPath+"?activated=1", http.StatusOK)
	if !strings.Contains(contractPage, "合同已激活并形成 revision 1") ||
		!strings.Contains(contractPage, "原合同 / E2E-CL-M2-001") ||
		!strings.Contains(contractPage, "登记生效补充协议") {
		t.Fatalf("active contract page missing revision 1 timeline or change form: %q", contractPage)
	}

	changePath := contractPath + "/changes"
	response = postForm(t, editorClient, testServer.URL+changePath, url.Values{
		"csrf_token":              {formHiddenValue(t, contractPage, changePath, "csrf_token")},
		"command_id":              {formHiddenValue(t, contractPage, changePath, "command_id")},
		"expected_revision":       {formHiddenValue(t, contractPage, changePath, "expected_revision")},
		"effective_order":         {formHiddenValue(t, contractPage, changePath, "effective_order")},
		"instrument_number":       {"E2E-SUPP-001"},
		"instrument_title":        {"端到端金额调整补充协议"},
		"change_summary":          {"验证生效补充协议形成下一版事实"},
		"instrument_signed_on":    {"2026-10-02"},
		"instrument_effective_on": {"2026-10-02"},
		"amount_effect":           {"adjustment"},
		"amount_value":            {"-100.25"},
		"actor_display_name":      {"伪造操作者"},
	})
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != contractPath+"?changed=1" {
		t.Fatalf("append contract change status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	contractPage = getPage(t, editorClient, testServer.URL+contractPath+"?changed=1", http.StatusOK)
	if !strings.Contains(contractPage, "生效补充协议已登记并形成新的合同 revision") ||
		!strings.Contains(contractPage, "补充协议 / E2E-SUPP-001") ||
		!strings.Contains(contractPage, "899.75 CNY") ||
		!strings.Contains(contractPage, "revision 2") {
		t.Fatalf("changed contract page missing current revision facts: %q", contractPage)
	}

	var currentRevision int64
	var currentAmount string
	var instrumentCount, revisionCount, revisionTwoPartyCount, changeAuditCount int
	if err := pool.QueryRow(ctx, `
		SELECT c.current_revision, r.total_amount::text,
			(SELECT count(*) FROM contract_instruments WHERE contract_id = c.id),
			(SELECT count(*) FROM contract_revisions WHERE contract_id = c.id),
			(SELECT count(*) FROM contract_revision_parties WHERE contract_id = c.id AND revision = 2),
			(SELECT count(*) FROM audit_entries WHERE object_id = c.id AND action = 'contract_change_activated')
		FROM contracts c
		JOIN contract_revisions r ON r.contract_id = c.id AND r.revision = c.current_revision
		WHERE c.id = $1
	`, contractID).Scan(&currentRevision, &currentAmount, &instrumentCount, &revisionCount, &revisionTwoPartyCount, &changeAuditCount); err != nil {
		t.Fatalf("read changed contract database facts: %v", err)
	}
	if currentRevision != 2 || currentAmount != "899.750000" || instrumentCount != 2 || revisionCount != 2 || revisionTwoPartyCount != 2 || changeAuditCount != 1 {
		t.Fatalf("changed facts revision=%d amount=%s instruments=%d revisions=%d parties=%d audits=%d", currentRevision, currentAmount, instrumentCount, revisionCount, revisionTwoPartyCount, changeAuditCount)
	}
	if err := pool.QueryRow(ctx, `
		SELECT actor_kind, actor_id, actor_display_name
		FROM audit_entries
		WHERE object_id = $1 AND action = 'contract_change_activated'
	`, contractID).Scan(&actorKind, &actorID, &actorDisplayName); err != nil {
		t.Fatalf("read contract change audit actor: %v", err)
	}
	if actorKind != audit.ActorKindLocalUser || actorID != editorID || actorDisplayName != "系统验收编辑者" {
		t.Fatalf("change audit actor=%s:%s (%s), editor=%s", actorKind, actorID, actorDisplayName, editorID)
	}

	settlementPath := contractPath + "/settlements"
	response = postForm(t, editorClient, testServer.URL+settlementPath, url.Values{
		"csrf_token":                   {formHiddenValue(t, contractPage, settlementPath, "csrf_token")},
		"command_id":                   {formHiddenValue(t, contractPage, settlementPath, "command_id")},
		"expected_revision":            {formHiddenValue(t, contractPage, settlementPath, "expected_revision")},
		"expected_settlement_sequence": {formHiddenValue(t, contractPage, settlementPath, "expected_settlement_sequence")},
		"settlement_kind":              {"progress"},
		"settlement_number":            {"E2E-SETTLEMENT-001"},
		"settlement_title":             {"端到端阶段结算"},
		"settlement_amount":            {"850.50"},
		"settlement_confirmed_on":      {"2026-10-02"},
		"settlement_basis":             {"双方确认截至本阶段的累计结算金额"},
		"confirming_party_position":    {"1", "2"},
		"actor_id":                     {"forged-settlement-actor"},
	})
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != contractPath+"?settlement_recorded=1" {
		t.Fatalf("progress settlement status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)
	contractPage = getPage(t, editorClient, testServer.URL+contractPath+"?settlement_recorded=1", http.StatusOK)
	if !strings.Contains(contractPage, "结算事实已登记") || !strings.Contains(contractPage, "E2E-SETTLEMENT-001") ||
		!strings.Contains(contractPage, "850.5 CNY") || !strings.Contains(contractPage, "阶段结算") ||
		!strings.Contains(contractPage, "端到端验收主体（委托方") || !strings.Contains(contractPage, "端到端验收主体乙") {
		t.Fatalf("progress settlement page mismatch: %q", contractPage)
	}
	var settlementCount, settlementPartyCount, settlementAuditCount int
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM contract_settlements WHERE contract_id = $1),
			(SELECT count(*) FROM contract_settlement_parties p JOIN contract_settlements s ON s.id = p.settlement_id WHERE s.contract_id = $1),
			(SELECT count(*) FROM audit_entries WHERE object_id = $1 AND action = 'contract_settlement_recorded')
	`, contractID).Scan(&settlementCount, &settlementPartyCount, &settlementAuditCount); err != nil {
		t.Fatalf("read progress settlement database facts: %v", err)
	}
	if settlementCount != 1 || settlementPartyCount != 2 || settlementAuditCount != 1 {
		t.Fatalf("progress settlement facts settlements=%d parties=%d audits=%d", settlementCount, settlementPartyCount, settlementAuditCount)
	}

	terminatePath := contractPath + "/terminate"
	response = postForm(t, editorClient, testServer.URL+terminatePath, url.Values{
		"csrf_token":                    {formHiddenValue(t, contractPage, terminatePath, "csrf_token")},
		"command_id":                    {formHiddenValue(t, contractPage, terminatePath, "command_id")},
		"expected_revision":             {formHiddenValue(t, contractPage, terminatePath, "expected_revision")},
		"effective_order":               {formHiddenValue(t, contractPage, terminatePath, "effective_order")},
		"termination_instrument_number": {"E2E-TERM-001"},
		"termination_instrument_title":  {"端到端合同终止协议"},
		"termination_reason":            {"验证终止形成终态且不执行结算"},
		"termination_signed_on":         {"2026-10-03"},
		"termination_effective_on":      {"2026-10-03"},
		"actor_display_name":            {"伪造终止操作者"},
	})
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != contractPath+"?terminated=1" {
		t.Fatalf("terminate contract status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)

	contractPage = getPage(t, editorClient, testServer.URL+contractPath+"?terminated=1", http.StatusOK)
	if !strings.Contains(contractPage, "生效终止文书已登记，合同已终止") ||
		!strings.Contains(contractPage, "终止文书 / E2E-TERM-001") ||
		!strings.Contains(contractPage, "899.75 CNY") ||
		!strings.Contains(contractPage, "revision 3") ||
		strings.Contains(contractPage, `action="`+changePath+`"`) ||
		strings.Contains(contractPage, `action="`+terminatePath+`"`) ||
		!strings.Contains(contractPage, `action="`+settlementPath+`"`) {
		t.Fatalf("terminated contract page facts/forms mismatch: %q", contractPage)
	}

	var contractStatus string
	var terminationCommandCount, terminationAuditCount int
	if err := pool.QueryRow(ctx, `
		SELECT c.status, c.current_revision, r.total_amount::text,
			(SELECT count(*) FROM contract_instruments WHERE contract_id = c.id),
			(SELECT count(*) FROM contract_revisions WHERE contract_id = c.id),
			(SELECT count(*) FROM contract_revision_parties WHERE contract_id = c.id AND revision = 3),
			(SELECT count(*) FROM contract_termination_commands WHERE contract_id = c.id),
			(SELECT count(*) FROM audit_entries WHERE object_id = c.id AND action = 'contract_terminated')
		FROM contracts c
		JOIN contract_revisions r ON r.contract_id = c.id AND r.revision = c.current_revision
		WHERE c.id = $1
	`, contractID).Scan(
		&contractStatus,
		&currentRevision,
		&currentAmount,
		&instrumentCount,
		&revisionCount,
		&revisionTwoPartyCount,
		&terminationCommandCount,
		&terminationAuditCount,
	); err != nil {
		t.Fatalf("read terminated contract database facts: %v", err)
	}
	if contractStatus != string(contract.StatusTerminated) || currentRevision != 3 || currentAmount != "899.750000" ||
		instrumentCount != 3 || revisionCount != 3 || revisionTwoPartyCount != 2 ||
		terminationCommandCount != 1 || terminationAuditCount != 1 {
		t.Fatalf("terminated facts status=%s revision=%d amount=%s instruments=%d revisions=%d parties=%d commands=%d audits=%d", contractStatus, currentRevision, currentAmount, instrumentCount, revisionCount, revisionTwoPartyCount, terminationCommandCount, terminationAuditCount)
	}
	if err := pool.QueryRow(ctx, `
		SELECT actor_kind, actor_id, actor_display_name
		FROM audit_entries
		WHERE object_id = $1 AND action = 'contract_terminated'
	`, contractID).Scan(&actorKind, &actorID, &actorDisplayName); err != nil {
		t.Fatalf("read termination audit actor: %v", err)
	}
	if actorKind != audit.ActorKindLocalUser || actorID != editorID || actorDisplayName != "系统验收编辑者" {
		t.Fatalf("termination audit actor=%s:%s (%s), editor=%s", actorKind, actorID, actorDisplayName, editorID)
	}

	response = postForm(t, editorClient, testServer.URL+settlementPath, url.Values{
		"csrf_token":                   {formHiddenValue(t, contractPage, settlementPath, "csrf_token")},
		"command_id":                   {formHiddenValue(t, contractPage, settlementPath, "command_id")},
		"expected_revision":            {formHiddenValue(t, contractPage, settlementPath, "expected_revision")},
		"expected_settlement_sequence": {formHiddenValue(t, contractPage, settlementPath, "expected_settlement_sequence")},
		"settlement_kind":              {"final"},
		"settlement_number":            {"E2E-SETTLEMENT-FINAL"},
		"settlement_title":             {"端到端最终结算"},
		"settlement_amount":            {"875.25"},
		"settlement_confirmed_on":      {"2026-10-05"},
		"settlement_basis":             {"合同终止后双方确认最终累计结算金额"},
		"confirming_party_position":    {"1", "2"},
		"actor_id":                     {"forged-final-settlement-actor"},
	})
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != contractPath+"?settlement_recorded=1" {
		t.Fatalf("final settlement status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)
	contractPage = getPage(t, editorClient, testServer.URL+contractPath+"?settlement_recorded=1", http.StatusOK)
	if !strings.Contains(contractPage, "E2E-SETTLEMENT-FINAL") || !strings.Contains(contractPage, "875.25 CNY") ||
		!strings.Contains(contractPage, "最终结算") || strings.Contains(contractPage, `action="`+settlementPath+`"`) {
		t.Fatalf("final settlement page mismatch: %q", contractPage)
	}
	var latestSettlementID, latestSettlementKind, latestSettlementAmount, settlementActorID string
	if err := pool.QueryRow(ctx, `
		SELECT id::text, settlement_kind, total_amount::text, actor_id
		FROM contract_settlements
		WHERE contract_id = $1
		ORDER BY settlement_sequence DESC
		LIMIT 1
	`, contractID).Scan(&latestSettlementID, &latestSettlementKind, &latestSettlementAmount, &settlementActorID); err != nil {
		t.Fatalf("read final settlement database fact: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM contract_settlements WHERE contract_id = $1),
			(SELECT count(*) FROM contract_settlement_parties p JOIN contract_settlements s ON s.id = p.settlement_id WHERE s.contract_id = $1),
			(SELECT count(*) FROM audit_entries WHERE object_id = $1 AND action = 'contract_settlement_recorded')
	`, contractID).Scan(&settlementCount, &settlementPartyCount, &settlementAuditCount); err != nil {
		t.Fatalf("read final settlement counts: %v", err)
	}
	if latestSettlementKind != "final" || latestSettlementAmount != "875.250000" || settlementActorID != editorID ||
		settlementCount != 2 || settlementPartyCount != 4 || settlementAuditCount != 2 {
		t.Fatalf("final settlement kind=%s amount=%s actor=%s settlements=%d parties=%d audits=%d", latestSettlementKind, latestSettlementAmount, settlementActorID, settlementCount, settlementPartyCount, settlementAuditCount)
	}

	closePath := contractPath + "/close"
	response = postForm(t, editorClient, testServer.URL+closePath, url.Values{
		"csrf_token":                   {formHiddenValue(t, contractPage, closePath, "csrf_token")},
		"command_id":                   {formHiddenValue(t, contractPage, closePath, "command_id")},
		"expected_revision":            {formHiddenValue(t, contractPage, closePath, "expected_revision")},
		"expected_settlement_sequence": {formHiddenValue(t, contractPage, closePath, "expected_settlement_sequence")},
		"settlement_disposition":       {formHiddenValue(t, contractPage, closePath, "settlement_disposition")},
		"closed_on":                    {"2026-10-06"},
		"closure_reason":               {"最终结算已经确认，合同常规维护完成"},
		"actor_id":                     {"forged-closure-actor"},
	})
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != contractPath+"?closed=1" {
		t.Fatalf("close contract status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)
	contractPage = getPage(t, editorClient, testServer.URL+contractPath+"?closed=1", http.StatusOK)
	if !strings.Contains(contractPage, "合同已完成操作性关闭") || !strings.Contains(contractPage, "操作性关闭事实") ||
		!strings.Contains(contractPage, "最终结算已经确认，合同常规维护完成") || !strings.Contains(contractPage, "已登记最终结算") ||
		!strings.Contains(contractPage, "revision 3 / settlement sequence 2") ||
		strings.Contains(contractPage, `action="`+changePath+`"`) || strings.Contains(contractPage, `action="`+terminatePath+`"`) ||
		strings.Contains(contractPage, `action="`+settlementPath+`"`) || strings.Contains(contractPage, `action="`+closePath+`"`) {
		t.Fatalf("closed contract page facts/forms mismatch: %q", contractPage)
	}
	var closureCount, closureCommandCount, closureAuditCount int
	var closureID, closureFinalSettlementID, closureDisposition, closureActorID string
	if err := pool.QueryRow(ctx, `
		SELECT c.status, c.current_revision, r.total_amount::text,
			(SELECT count(*) FROM contract_instruments WHERE contract_id = c.id),
			(SELECT count(*) FROM contract_revisions WHERE contract_id = c.id),
			(SELECT count(*) FROM contract_closures WHERE contract_id = c.id),
			(SELECT count(*) FROM contract_closure_commands WHERE contract_id = c.id),
			(SELECT count(*) FROM audit_entries WHERE object_id = c.id AND action = 'contract_closed')
		FROM contracts c
		JOIN contract_revisions r ON r.contract_id = c.id AND r.revision = c.current_revision
		WHERE c.id = $1
	`, contractID).Scan(
		&contractStatus, &currentRevision, &currentAmount, &instrumentCount, &revisionCount,
		&closureCount, &closureCommandCount, &closureAuditCount,
	); err != nil {
		t.Fatalf("read closed contract database facts: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT id::text, final_settlement_id::text, settlement_disposition, actor_id
		FROM contract_closures WHERE contract_id = $1
	`, contractID).Scan(&closureID, &closureFinalSettlementID, &closureDisposition, &closureActorID); err != nil {
		t.Fatalf("read closure fact: %v", err)
	}
	if contractStatus != string(contract.StatusClosed) || currentRevision != 3 || currentAmount != "899.750000" ||
		instrumentCount != 3 || revisionCount != 3 || closureCount != 1 || closureCommandCount != 1 || closureAuditCount != 1 ||
		closureFinalSettlementID != latestSettlementID || closureDisposition != "final_recorded" || closureActorID != editorID {
		t.Fatalf("closed facts status=%s revision=%d amount=%s instruments=%d revisions=%d closures=%d commands=%d audits=%d final=%s/%s disposition=%s actor=%s", contractStatus, currentRevision, currentAmount, instrumentCount, revisionCount, closureCount, closureCommandCount, closureAuditCount, closureFinalSettlementID, latestSettlementID, closureDisposition, closureActorID)
	}

	evidencePath := contractPath + "/evidence"
	evidenceVersionOneContent := []byte("<script>alert('download-only evidence')</script>")
	response = postMultipart(t, editorClient, testServer.URL+evidencePath, url.Values{
		"csrf_token":       {formHiddenValue(t, contractPage, evidencePath, "csrf_token")},
		"command_id":       {formHiddenValue(t, contractPage, evidencePath, "command_id")},
		"evidence_target":  {"closure:" + closureID},
		"evidence_title":   {"操作性关闭佐证"},
		"evidence_purpose": {"证明本次关闭所依据的双方确认材料"},
	}, "evidence_file", "../../closure-proof.html", evidenceVersionOneContent)
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != contractPath+"?evidence_uploaded=1" {
		t.Fatalf("upload evidence status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)
	contractPage = getPage(t, editorClient, testServer.URL+contractPath+"?evidence_uploaded=1", http.StatusOK)
	if !strings.Contains(contractPage, "合同证据文件已保存") || !strings.Contains(contractPage, "操作性关闭佐证") ||
		!strings.Contains(contractPage, "证明本次关闭所依据的双方确认材料") || !strings.Contains(contractPage, "操作性关闭 / 2026-10-06") ||
		!strings.Contains(contractPage, "closure-proof.html") || !strings.Contains(contractPage, "SHA-256") || !strings.Contains(contractPage, "v1") {
		t.Fatalf("evidence version 1 page mismatch: %q", contractPage)
	}

	var evidenceDocumentID, evidenceVersionOneID string
	if err := pool.QueryRow(ctx, `
		SELECT d.id::text, v.id::text
		FROM contract_evidence_documents d
		JOIN contract_evidence_versions v ON v.document_id = d.id AND v.version = 1
		WHERE d.contract_id = $1 AND d.title = '操作性关闭佐证'
	`, contractID).Scan(&evidenceDocumentID, &evidenceVersionOneID); err != nil {
		t.Fatalf("read evidence version 1 IDs: %v", err)
	}
	evidenceVersionPath := evidencePath + "/" + evidenceDocumentID + "/versions"
	evidenceVersionTwoContent := []byte("%PDF-1.7\nreplacement scan")
	response = postMultipart(t, editorClient, testServer.URL+evidenceVersionPath, url.Values{
		"csrf_token":              {formHiddenValue(t, contractPage, evidenceVersionPath, "csrf_token")},
		"command_id":              {formHiddenValue(t, contractPage, evidenceVersionPath, "command_id")},
		"expected_latest_version": {formHiddenValue(t, contractPage, evidenceVersionPath, "expected_latest_version")},
	}, "evidence_file", "closure-proof-v2.pdf", evidenceVersionTwoContent)
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != contractPath+"?evidence_uploaded=1" {
		t.Fatalf("upload evidence version 2 status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)
	contractPage = getPage(t, editorClient, testServer.URL+contractPath+"?evidence_uploaded=1", http.StatusOK)
	if !strings.Contains(contractPage, "closure-proof.html") || !strings.Contains(contractPage, "closure-proof-v2.pdf") ||
		!strings.Contains(contractPage, "v1") || !strings.Contains(contractPage, "v2") {
		t.Fatalf("evidence version history page mismatch: %q", contractPage)
	}

	var evidenceVersionTwoID, evidenceTargetType, evidenceTargetID, evidenceOriginalFilename, evidenceActorID, evidenceSHA string
	var evidenceDocumentCount, evidenceVersionCount, evidenceCommandCount, evidenceUploadAuditCount int
	if err := pool.QueryRow(ctx, `
		SELECT v.id::text, d.target_type, d.target_id::text, v.original_filename, v.actor_id, v.sha256
		FROM contract_evidence_documents d
		JOIN contract_evidence_versions v ON v.document_id = d.id AND v.version = 2
		WHERE d.id = $1
	`, evidenceDocumentID).Scan(&evidenceVersionTwoID, &evidenceTargetType, &evidenceTargetID, &evidenceOriginalFilename, &evidenceActorID, &evidenceSHA); err != nil {
		t.Fatalf("read evidence version 2 fact: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM contract_evidence_documents WHERE contract_id = $1),
			(SELECT count(*) FROM contract_evidence_versions v JOIN contract_evidence_documents d ON d.id = v.document_id WHERE d.contract_id = $1),
			(SELECT count(*) FROM contract_evidence_commands WHERE contract_id = $1),
			(SELECT count(*) FROM audit_entries WHERE object_id = $1 AND action IN ('contract_evidence_created', 'contract_evidence_version_added'))
	`, contractID).Scan(&evidenceDocumentCount, &evidenceVersionCount, &evidenceCommandCount, &evidenceUploadAuditCount); err != nil {
		t.Fatalf("read evidence database counts: %v", err)
	}
	wantEvidenceSHA := fmt.Sprintf("%x", sha256.Sum256(evidenceVersionTwoContent))
	if evidenceTargetType != "closure" || evidenceTargetID != closureID || evidenceOriginalFilename != "closure-proof-v2.pdf" ||
		evidenceActorID != editorID || evidenceSHA != wantEvidenceSHA || evidenceDocumentCount != 1 || evidenceVersionCount != 2 ||
		evidenceCommandCount != 2 || evidenceUploadAuditCount != 2 {
		t.Fatalf("evidence facts target=%s/%s filename=%s actor=%s sha=%s/%s counts=%d/%d/%d/%d", evidenceTargetType, evidenceTargetID, evidenceOriginalFilename, evidenceActorID, evidenceSHA, wantEvidenceSHA, evidenceDocumentCount, evidenceVersionCount, evidenceCommandCount, evidenceUploadAuditCount)
	}

	evidenceVersionOneDownloadPath := evidenceVersionPath + "/" + evidenceVersionOneID + "/download"
	response = request(t, anonymous, http.MethodGet, testServer.URL+evidenceVersionOneDownloadPath, nil)
	if response.StatusCode != http.StatusSeeOther || !strings.HasPrefix(response.Header.Get("Location"), "/login?next=") {
		t.Fatalf("anonymous evidence download status=%d location=%q", response.StatusCode, response.Header.Get("Location"))
	}
	closeResponse(t, response)
	response = request(t, editorClient, http.MethodGet, testServer.URL+evidenceVersionOneDownloadPath, nil)
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "application/octet-stream" ||
		!strings.Contains(response.Header.Get("Content-Disposition"), "attachment") ||
		!strings.Contains(response.Header.Get("Content-Disposition"), "closure-proof.html") ||
		response.Header.Get("Cache-Control") != "private, no-store" || response.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("editor evidence download status=%d headers=%v body=%q", response.StatusCode, response.Header, readResponse(t, response))
	}
	if body := []byte(readResponse(t, response)); !bytes.Equal(body, evidenceVersionOneContent) {
		t.Fatalf("editor evidence download body=%q", body)
	}
	closeResponse(t, response)
	var evidenceDownloadAuditCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_entries WHERE object_id = $1 AND action = 'contract_evidence_downloaded' AND actor_id = $2`, contractID, editorID).Scan(&evidenceDownloadAuditCount); err != nil {
		t.Fatalf("count editor evidence download audit: %v", err)
	}
	if evidenceDownloadAuditCount != 1 {
		t.Fatalf("editor evidence download audit count = %d", evidenceDownloadAuditCount)
	}

	closedListPage := getPage(t, editorClient, testServer.URL+"/contracts", http.StatusOK)
	if !strings.Contains(closedListPage, "已关闭 r3") || !strings.Contains(closedListPage, "899.75 CNY") {
		t.Fatalf("contract list missing closed revision or preserved amount: %q", closedListPage)
	}
	searchQuery := url.Values{
		"q": {"E2E-CL-M2-001"}, "status": {"closed"}, "currency": {"cny"},
	}
	closedSearchPage := getPage(t, editorClient, testServer.URL+"/contracts?"+searchQuery.Encode(), http.StatusOK)
	if !strings.Contains(closedSearchPage, "端到端补充协议合同") ||
		!strings.Contains(closedSearchPage, "已关闭 r3") || !strings.Contains(closedSearchPage, "899.75 CNY") ||
		!strings.Contains(closedSearchPage, `value="CNY"`) {
		t.Fatalf("filtered closed contract page mismatch: %q", closedSearchPage)
	}
	invalidCursorPage := getPage(t, editorClient, testServer.URL+"/contracts?cursor=not-a-cursor", http.StatusUnprocessableEntity)
	if !strings.Contains(invalidCursorPage, "分页游标无效，请从第一页重新查询") {
		t.Fatalf("invalid cursor page missing validation message: %q", invalidCursorPage)
	}

	contractsPage = getPage(t, editorClient, testServer.URL+"/contracts", http.StatusOK)
	response = postForm(t, editorClient, testServer.URL+"/contracts", url.Values{
		"csrf_token":       {formHiddenValue(t, contractsPage, "/contracts", "csrf_token")},
		"command_id":       {formHiddenValue(t, contractsPage, "/contracts", "command_id")},
		"number_namespace": {"e2e"},
		"contract_number":  {"E2E-REL-TARGET"},
		"title":            {"端到端关系目标合同"},
		"currency":         {"CNY"},
		"total_amount":     {"500.00"},
		"signed_on":        {"2026-09-20"},
		"effective_on":     {"2026-10-01"},
	})
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("relation target contract create status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	closeResponse(t, response)
	var relationTargetID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM contracts WHERE contract_number = 'E2E-REL-TARGET'`).Scan(&relationTargetID); err != nil {
		t.Fatalf("read relation target contract ID: %v", err)
	}
	relationTargetPath := "/contracts/" + relationTargetID
	for index, partyID := range []string{firstPartyID, secondPartyID} {
		relationTargetPage := getPage(t, editorClient, testServer.URL+relationTargetPath, http.StatusOK)
		response = postForm(t, editorClient, testServer.URL+relationTargetPath+"/parties", url.Values{
			"csrf_token":             {formHiddenValue(t, relationTargetPage, relationTargetPath+"/parties", "csrf_token")},
			"command_id":             {formHiddenValue(t, relationTargetPage, relationTargetPath+"/parties", "command_id")},
			"expected_draft_version": {fmt.Sprintf("%d", index+1)},
			"party_id":               {partyID},
			"role":                   {[]string{"委托方", "受托方"}[index]},
		})
		if response.StatusCode != http.StatusSeeOther {
			t.Fatalf("relation target add party %d status=%d body=%q", index, response.StatusCode, readResponse(t, response))
		}
		closeResponse(t, response)
	}
	relationTargetPage := getPage(t, editorClient, testServer.URL+relationTargetPath, http.StatusOK)
	response = postForm(t, editorClient, testServer.URL+relationTargetPath+"/activate", url.Values{
		"csrf_token":             {formHiddenValue(t, relationTargetPage, relationTargetPath+"/activate", "csrf_token")},
		"command_id":             {formHiddenValue(t, relationTargetPage, relationTargetPath+"/activate", "command_id")},
		"expected_draft_version": {"3"},
	})
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("relation target activate status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	closeResponse(t, response)

	relationSearchQuery := url.Values{"q": {"E2E-REL-TARGET"}, "status": {"active"}, "currency": {"cny"}}
	relationSearchPage := getPage(t, editorClient, testServer.URL+contractPath+"/relations/new?"+relationSearchQuery.Encode(), http.StatusOK)
	if !strings.Contains(relationSearchPage, "端到端关系目标合同") || !strings.Contains(relationSearchPage, "选择为目标") {
		t.Fatalf("relation target search page mismatch: %q", relationSearchPage)
	}
	relationConfirmPage := getPage(t, editorClient, testServer.URL+contractPath+"/relations/new?target_id="+relationTargetID, http.StatusOK)
	relationPath := contractPath + "/relations"
	response = postForm(t, editorClient, testServer.URL+relationPath, url.Values{
		"csrf_token":               {formHiddenValue(t, relationConfirmPage, relationPath, "csrf_token")},
		"command_id":               {formHiddenValue(t, relationConfirmPage, relationPath, "command_id")},
		"target_contract_id":       {relationTargetID},
		"expected_source_revision": {"3"},
		"expected_target_revision": {"1"},
		"relation_type":            {"references"},
		"effective_on":             {"2026-10-04"},
		"note":                     {"终止合同引用后续独立合同"},
		"actor_id":                 {"forged-relation-actor"},
	})
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != contractPath+"?relation_linked=1" {
		t.Fatalf("link relation status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)
	contractPage = getPage(t, editorClient, testServer.URL+contractPath+"?relation_linked=1", http.StatusOK)
	if !strings.Contains(contractPage, "合同关系已建立") || !strings.Contains(contractPage, "端到端关系目标合同") ||
		!strings.Contains(contractPage, "终止合同引用后续独立合同") || !strings.Contains(contractPage, "引用") {
		t.Fatalf("source relation detail mismatch: %q", contractPage)
	}
	relationTargetPage = getPage(t, editorClient, testServer.URL+relationTargetPath, http.StatusOK)
	if !strings.Contains(relationTargetPage, "端到端补充协议合同") || !strings.Contains(relationTargetPage, "终止合同引用后续独立合同") {
		t.Fatalf("target incoming relation detail mismatch: %q", relationTargetPage)
	}
	var relationCount, relationCommandCount, relationAuditCount int
	var relationActorID, relationType string
	var relationSourceRevision, relationTargetRevision int64
	if err := pool.QueryRow(ctx, `
		SELECT count(*), min(relation_type), min(source_revision), min(target_revision), min(actor_id)
		FROM contract_relations
		WHERE source_contract_id = $1 AND target_contract_id = $2
	`, contractID, relationTargetID).Scan(&relationCount, &relationType, &relationSourceRevision, &relationTargetRevision, &relationActorID); err != nil {
		t.Fatalf("read relation database facts: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM contract_relation_commands WHERE source_contract_id = $1 AND target_contract_id = $2`, contractID, relationTargetID).Scan(&relationCommandCount); err != nil {
		t.Fatalf("count relation commands: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_entries WHERE object_id = $1 AND action = 'contract_relation_linked'`, contractID).Scan(&relationAuditCount); err != nil {
		t.Fatalf("count relation audits: %v", err)
	}
	if relationCount != 1 || relationCommandCount != 1 || relationAuditCount != 1 || relationType != "references" ||
		relationSourceRevision != 3 || relationTargetRevision != 1 || relationActorID != editorID {
		t.Fatalf("relation facts count=%d commands=%d audits=%d type=%s revisions=%d/%d actor=%s", relationCount, relationCommandCount, relationAuditCount, relationType, relationSourceRevision, relationTargetRevision, relationActorID)
	}
	postClosureChangeCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	response = postForm(t, editorClient, testServer.URL+changePath, url.Values{
		"csrf_token":              {csrfToken(t, contractPage)},
		"command_id":              {postClosureChangeCommandID},
		"expected_revision":       {"3"},
		"effective_order":         {"4"},
		"instrument_number":       {"E2E-POST-TERM-SUPP"},
		"instrument_title":        {"终止后伪造补充协议"},
		"change_summary":          {"不应写入"},
		"instrument_signed_on":    {"2026-10-04"},
		"instrument_effective_on": {"2026-10-04"},
		"amount_effect":           {"unchanged"},
	})
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("post-closure editor change status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	closeResponse(t, response)
	if err := pool.QueryRow(ctx, `SELECT current_revision FROM contracts WHERE id = $1`, contractID).Scan(&currentRevision); err != nil {
		t.Fatalf("read revision after post-closure editor change: %v", err)
	}
	if currentRevision != 3 {
		t.Fatalf("post-closure editor change advanced revision to %d", currentRevision)
	}

	correctionPath := contractPath + "/corrections"
	if strings.Contains(contractPage, `action="`+correctionPath+`"`) {
		t.Fatalf("editor page exposes administrator-only correction form: %q", contractPage)
	}
	response = postForm(t, editorClient, testServer.URL+correctionPath, url.Values{
		"csrf_token": {csrfToken(t, contractPage)}, "expected_status": {"closed"},
		"expected_revision": {"3"}, "effective_order": {"4"},
		"correction_instrument_number": {"E2E-CORR-FORBIDDEN"},
		"correction_instrument_title":  {"编辑者伪造更正文书"},
		"correction_reason":            {"不应写入"},
	})
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("editor correction status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	closeResponse(t, response)
	if err := pool.QueryRow(ctx, `SELECT current_revision FROM contracts WHERE id = $1`, contractID).Scan(&currentRevision); err != nil {
		t.Fatalf("read revision after forbidden editor correction: %v", err)
	}
	if currentRevision != 3 {
		t.Fatalf("forbidden editor correction advanced revision to %d", currentRevision)
	}

	adminContractPage := getPage(t, adminClient, testServer.URL+contractPath, http.StatusOK)
	if !strings.Contains(adminContractPage, `action="`+correctionPath+`"`) ||
		formHiddenValue(t, adminContractPage, correctionPath, "expected_status") != "closed" ||
		formHiddenValue(t, adminContractPage, correctionPath, "expected_revision") != "3" ||
		formHiddenValue(t, adminContractPage, correctionPath, "effective_order") != "4" ||
		!strings.Contains(adminContractPage, `name="corrected_title" value="端到端补充协议合同"`) {
		t.Fatalf("closed administrator correction form mismatch: %q", adminContractPage)
	}
	response = postForm(t, adminClient, testServer.URL+correctionPath, url.Values{
		"csrf_token":                         {formHiddenValue(t, adminContractPage, correctionPath, "csrf_token")},
		"command_id":                         {formHiddenValue(t, adminContractPage, correctionPath, "command_id")},
		"expected_status":                    {formHiddenValue(t, adminContractPage, correctionPath, "expected_status")},
		"expected_revision":                  {formHiddenValue(t, adminContractPage, correctionPath, "expected_revision")},
		"effective_order":                    {formHiddenValue(t, adminContractPage, correctionPath, "effective_order")},
		"correction_instrument_number":       {"E2E-CORR-001"},
		"correction_instrument_title":        {"端到端结构化记录更正确认单"},
		"correction_reason":                  {"核对原始合同后修正标题、分类、金额和日期录入"},
		"correction_instrument_signed_on":    {"2026-10-08"},
		"correction_instrument_effective_on": {"2026-10-08"},
		"corrected_title":                    {"端到端受控更正合同"},
		"corrected_category":                 {"更正后验收分类"},
		"corrected_total_amount":             {"901.125000"},
		"corrected_signed_on":                {"2026-09-20"},
		"corrected_effective_on":             {"2026-10-01"},
		"actor_id":                           {"forged-correction-actor"},
	})
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != contractPath+"?corrected=1" {
		t.Fatalf("closed correction status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)
	adminContractPage = getPage(t, adminClient, testServer.URL+contractPath+"?corrected=1", http.StatusOK)
	if !strings.Contains(adminContractPage, "合同核心字段已受控更正并形成新的 revision；生命周期状态未改变") ||
		!strings.Contains(adminContractPage, "端到端受控更正合同") || !strings.Contains(adminContractPage, "更正后验收分类") ||
		!strings.Contains(adminContractPage, "901.125 CNY") || !strings.Contains(adminContractPage, "更正文书 / E2E-CORR-001") ||
		!strings.Contains(adminContractPage, "更正为 901.125") || !strings.Contains(adminContractPage, "revision 3 / settlement sequence 2") {
		t.Fatalf("corrected closed contract page mismatch: %q", adminContractPage)
	}

	var adminID, correctionInstrumentID, correctionActorID, correctionNumber, correctedTitle, correctedCategory string
	var priorTitle, priorAmount, correctedAmount string
	var correctionCommandCount, correctionAuditCount, revisionFourPartyCount int
	var closureSourceRevision, finalSettlementSourceRevision, preservedRelationSourceRevision int64
	if err := pool.QueryRow(ctx, `SELECT id::text FROM local_users WHERE username = 'system-admin'`).Scan(&adminID); err != nil {
		t.Fatalf("read administrator ID: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT c.status, c.current_revision, r.title, COALESCE(r.category, ''), r.total_amount::text,
			to_char(r.signed_on, 'YYYY-MM-DD'), to_char(r.effective_on, 'YYYY-MM-DD'),
			(SELECT count(*) FROM contract_correction_commands WHERE contract_id = c.id),
			(SELECT count(*) FROM audit_entries WHERE object_id = c.id AND action = 'contract_correction_recorded'),
			(SELECT count(*) FROM contract_revision_parties WHERE contract_id = c.id AND revision = 4)
		FROM contracts c
		JOIN contract_revisions r ON r.contract_id = c.id AND r.revision = c.current_revision
		WHERE c.id = $1
	`, contractID).Scan(
		&contractStatus, &currentRevision, &correctedTitle, &correctedCategory, &correctedAmount,
		new(string), new(string), &correctionCommandCount, &correctionAuditCount, &revisionFourPartyCount,
	); err != nil {
		t.Fatalf("read corrected current contract facts: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT i.id::text, i.instrument_number, i.actor_id, r3.title, r3.total_amount::text,
			cl.source_revision, fs.source_revision, rel.source_revision
		FROM contract_instruments i
		JOIN contract_revisions r3 ON r3.contract_id = i.contract_id AND r3.revision = 3
		JOIN contract_closures cl ON cl.contract_id = i.contract_id
		JOIN contract_settlements fs ON fs.id = cl.final_settlement_id
		JOIN contract_relations rel ON rel.source_contract_id = i.contract_id
		WHERE i.contract_id = $1 AND i.instrument_type = 'correction_instrument'
	`, contractID).Scan(
		&correctionInstrumentID, &correctionNumber, &correctionActorID, &priorTitle, &priorAmount,
		&closureSourceRevision, &finalSettlementSourceRevision, &preservedRelationSourceRevision,
	); err != nil {
		t.Fatalf("read correction and preserved-reference facts: %v", err)
	}
	if contractStatus != string(contract.StatusClosed) || currentRevision != 4 || correctedTitle != "端到端受控更正合同" ||
		correctedCategory != "更正后验收分类" || correctedAmount != "901.125000" || correctionCommandCount != 1 ||
		correctionAuditCount != 1 || revisionFourPartyCount != 2 || correctionNumber != "E2E-CORR-001" ||
		correctionActorID != adminID || priorTitle != "端到端补充协议合同" || priorAmount != "899.750000" ||
		closureSourceRevision != 3 || finalSettlementSourceRevision != 3 || preservedRelationSourceRevision != 3 {
		t.Fatalf("correction facts status=%s revision=%d title=%s category=%s amount=%s commands=%d audits=%d parties=%d instrument=%s actor=%s/%s prior=%s/%s references=%d/%d/%d",
			contractStatus, currentRevision, correctedTitle, correctedCategory, correctedAmount, correctionCommandCount, correctionAuditCount,
			revisionFourPartyCount, correctionNumber, correctionActorID, adminID, priorTitle, priorAmount,
			closureSourceRevision, finalSettlementSourceRevision, preservedRelationSourceRevision)
	}

	correctionEvidenceContent := []byte("correction instrument supporting material")
	response = postMultipart(t, adminClient, testServer.URL+evidencePath, url.Values{
		"csrf_token":       {formHiddenValue(t, adminContractPage, evidencePath, "csrf_token")},
		"command_id":       {formHiddenValue(t, adminContractPage, evidencePath, "command_id")},
		"evidence_target":  {"instrument:" + correctionInstrumentID},
		"evidence_title":   {"更正文书佐证"},
		"evidence_purpose": {"验证更正文书可以作为 Evidence 引用目标"},
	}, "evidence_file", "correction-proof.txt", correctionEvidenceContent)
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != contractPath+"?evidence_uploaded=1" {
		t.Fatalf("upload correction evidence status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	closeResponse(t, response)
	adminContractPage = getPage(t, adminClient, testServer.URL+contractPath+"?evidence_uploaded=1", http.StatusOK)
	if !strings.Contains(adminContractPage, "更正文书佐证") || !strings.Contains(adminContractPage, "更正文书 / E2E-CORR-001") ||
		!strings.Contains(adminContractPage, "correction-proof.txt") {
		t.Fatalf("correction evidence page mismatch: %q", adminContractPage)
	}
	var correctionEvidenceTargetID string
	if err := pool.QueryRow(ctx, `
		SELECT target_id::text FROM contract_evidence_documents
		WHERE contract_id = $1 AND target_type = 'instrument' AND title = '更正文书佐证'
	`, contractID).Scan(&correctionEvidenceTargetID); err != nil {
		t.Fatalf("read correction evidence target: %v", err)
	}
	if correctionEvidenceTargetID != correctionInstrumentID {
		t.Fatalf("correction evidence target=%s want=%s", correctionEvidenceTargetID, correctionInstrumentID)
	}
	if err := pool.QueryRow(ctx, `SELECT current_revision FROM contracts WHERE id = $1`, contractID).Scan(&currentRevision); err != nil {
		t.Fatalf("read revision after correction evidence upload: %v", err)
	}
	if currentRevision != 4 {
		t.Fatalf("correction evidence upload advanced revision to %d", currentRevision)
	}

	exportPath := contractPath + "/export"
	response = request(t, editorClient, http.MethodGet, testServer.URL+exportPath, nil)
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("editor ledger export status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	closeResponse(t, response)
	var exportAuditCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_entries WHERE object_id = $1 AND action = 'contract_ledger_exported'`, contractID).Scan(&exportAuditCount); err != nil {
		t.Fatalf("count forbidden editor export audits: %v", err)
	}
	if exportAuditCount != 0 {
		t.Fatalf("forbidden editor export created %d audits", exportAuditCount)
	}

	response = request(t, adminClient, http.MethodGet, testServer.URL+exportPath, nil)
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "application/zip" ||
		response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("X-Content-Type-Options") != "nosniff" ||
		!strings.Contains(response.Header.Get("Content-Disposition"), "contract-"+contractID+"-ledger.zip") {
		t.Fatalf("administrator ledger export status=%d headers=%v body=%q", response.StatusCode, response.Header, readResponse(t, response))
	}
	archiveContent := []byte(readResponse(t, response))
	closeResponse(t, response)
	manifest, exportedLedger, rawLedger := decodeLedgerArchive(t, archiveContent)
	ledgerDigest := fmt.Sprintf("%x", sha256.Sum256(rawLedger))
	if manifest.FormatID != ledgerexport.FormatID || manifest.FormatVersion != ledgerexport.FormatVersion ||
		manifest.ContractID != contractID || manifest.LedgerFilename != "ledger.json" ||
		manifest.LedgerBytes != len(rawLedger) || manifest.LedgerSHA256 != ledgerDigest || manifest.EvidenceContentIncluded {
		t.Fatalf("export manifest=%#v digest=%s", manifest, ledgerDigest)
	}
	if bytes.Contains(rawLedger, []byte("storage_key")) || bytes.Contains(rawLedger, []byte("storage_provider")) ||
		bytes.Contains(rawLedger, correctionEvidenceContent) || bytes.Contains(rawLedger, evidenceVersionTwoContent) {
		t.Fatalf("ledger export leaked file or storage internals: %s", rawLedger)
	}
	if exportedLedger.Contract.ID != contractID || exportedLedger.Contract.Status != string(contract.StatusClosed) ||
		exportedLedger.Contract.CurrentRevision == nil || *exportedLedger.Contract.CurrentRevision != 4 ||
		len(exportedLedger.DraftParties) != 2 || len(exportedLedger.Revisions) != 4 || len(exportedLedger.Instruments) != 4 ||
		len(exportedLedger.Relations) != 1 || len(exportedLedger.Settlements) != 2 || exportedLedger.Closure == nil ||
		exportedLedger.Closure.SourceRevision != 3 || len(exportedLedger.Evidence) < 2 {
		t.Fatalf("exported ledger shape=%#v", exportedLedger)
	}
	for _, revision := range exportedLedger.Revisions {
		if len(revision.Parties) != 2 {
			t.Fatalf("revision %d exported parties=%d", revision.Revision, len(revision.Parties))
		}
	}
	if exportedLedger.Relations[0].Source.Revision != 3 || exportedLedger.Settlements[1].SourceRevision != 3 {
		t.Fatalf("historical references relation=%d settlement=%d", exportedLedger.Relations[0].Source.Revision, exportedLedger.Settlements[1].SourceRevision)
	}
	for _, entry := range exportedLedger.Audit {
		if entry.Action == ledgerexport.ActionContractLedgerExported {
			t.Fatal("first export contains its own export audit")
		}
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM audit_entries
		WHERE object_id = $1 AND action = 'contract_ledger_exported'
			AND actor_kind = 'local_user' AND actor_id = $2
	`, contractID, adminID).Scan(&exportAuditCount); err != nil {
		t.Fatalf("count administrator export audit: %v", err)
	}
	if exportAuditCount != 1 {
		t.Fatalf("administrator export audit count=%d", exportAuditCount)
	}
	if err := pool.QueryRow(ctx, `SELECT current_revision FROM contracts WHERE id = $1`, contractID).Scan(&currentRevision); err != nil {
		t.Fatalf("read revision after ledger export: %v", err)
	}
	if currentRevision != 4 {
		t.Fatalf("ledger export advanced revision to %d", currentRevision)
	}

	response = request(t, adminClient, http.MethodGet, testServer.URL+exportPath, nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("second administrator export status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	secondArchive := []byte(readResponse(t, response))
	closeResponse(t, response)
	_, secondLedger, _ := decodeLedgerArchive(t, secondArchive)
	priorExportAudits := 0
	for _, entry := range secondLedger.Audit {
		if entry.Action == ledgerexport.ActionContractLedgerExported {
			priorExportAudits++
		}
	}
	if priorExportAudits != 1 {
		t.Fatalf("second export prior audit count=%d", priorExportAudits)
	}

	response = request(t, adminClient, http.MethodGet, testServer.URL+"/contracts/"+relationTargetID+"/export", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("relation target ledger export status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	targetArchive := []byte(readResponse(t, response))
	closeResponse(t, response)
	_, targetLedger, _ := decodeLedgerArchive(t, targetArchive)
	if len(targetLedger.Relations) != 1 || targetLedger.Relations[0].Source.ContractID != contractID ||
		targetLedger.Relations[0].Target.ContractID != relationTargetID {
		t.Fatalf("target ledger incoming relations=%#v", targetLedger.Relations)
	}

	contractsPage = getPage(t, editorClient, testServer.URL+"/contracts", http.StatusOK)
	response = postForm(t, editorClient, testServer.URL+"/contracts", url.Values{
		"csrf_token":       {formHiddenValue(t, contractsPage, "/contracts", "csrf_token")},
		"command_id":       {formHiddenValue(t, contractsPage, "/contracts", "command_id")},
		"number_namespace": {"e2e"}, "contract_number": {"E2E-EXPORT-DRAFT"},
		"title": {"端到端导出草稿"}, "currency": {"CNY"},
	})
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("create export draft status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	closeResponse(t, response)
	var exportDraftID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM contracts WHERE contract_number = 'E2E-EXPORT-DRAFT'`).Scan(&exportDraftID); err != nil {
		t.Fatalf("read export draft ID: %v", err)
	}
	exportDraftPath := "/contracts/" + exportDraftID
	response = request(t, adminClient, http.MethodGet, testServer.URL+exportDraftPath+"/export", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("draft ledger export status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	draftArchive := []byte(readResponse(t, response))
	closeResponse(t, response)
	_, draftLedger, draftLedgerBytes := decodeLedgerArchive(t, draftArchive)
	if draftLedger.Contract.Status != string(contract.StatusDraft) || draftLedger.Cancellation != nil ||
		len(draftLedger.Revisions) != 0 || len(draftLedger.Instruments) != 0 ||
		!bytes.Contains(draftLedgerBytes, []byte(`"revisions": []`)) {
		t.Fatalf("draft ledger=%#v", draftLedger)
	}

	exportDraftPage := getPage(t, editorClient, testServer.URL+exportDraftPath, http.StatusOK)
	cancelPath := exportDraftPath + "/cancel"
	response = postForm(t, editorClient, testServer.URL+cancelPath, url.Values{
		"csrf_token":             {formHiddenValue(t, exportDraftPage, cancelPath, "csrf_token")},
		"command_id":             {formHiddenValue(t, exportDraftPage, cancelPath, "command_id")},
		"expected_draft_version": {formHiddenValue(t, exportDraftPage, cancelPath, "expected_draft_version")},
		"cancellation_reason":    {"验证取消事实进入可验证账簿"},
	})
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("cancel export draft status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	closeResponse(t, response)
	response = request(t, adminClient, http.MethodGet, testServer.URL+exportDraftPath+"/export", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("cancelled ledger export status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	cancelledArchive := []byte(readResponse(t, response))
	closeResponse(t, response)
	_, cancelledLedger, _ := decodeLedgerArchive(t, cancelledArchive)
	if cancelledLedger.Contract.Status != string(contract.StatusCancelled) || cancelledLedger.Cancellation == nil ||
		cancelledLedger.Cancellation.Reason != "验证取消事实进入可验证账簿" ||
		cancelledLedger.Cancellation.Actor.ID != editorID || len(cancelledLedger.Revisions) != 0 {
		t.Fatalf("cancelled ledger=%#v", cancelledLedger)
	}

	viewerClient := newSessionClient(t)
	login(t, viewerClient, testServer.URL, "system-viewer", "system-viewer-password")
	viewerPartyPage := getPage(t, viewerClient, testServer.URL+"/parties", http.StatusOK)
	if strings.Contains(viewerPartyPage, "新建主体") {
		t.Fatalf("viewer page exposes write form: %q", viewerPartyPage)
	}
	response = postForm(t, viewerClient, testServer.URL+"/parties", url.Values{
		"csrf_token": {csrfToken(t, viewerPartyPage)},
		"kind":       {"organization"}, "legal_name": {"只读用户伪造主体"},
	})
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer party create status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	closeResponse(t, response)
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM parties WHERE legal_name = '只读用户伪造主体'`).Scan(&partyCount); err != nil {
		t.Fatalf("count forbidden party: %v", err)
	}
	if partyCount != 0 {
		t.Fatalf("forbidden viewer write created %d parties", partyCount)
	}
	viewerContractPage := getPage(t, viewerClient, testServer.URL+contractPath, http.StatusOK)
	if !strings.Contains(viewerContractPage, "端到端受控更正合同") ||
		!strings.Contains(viewerContractPage, "补充协议 / E2E-SUPP-001") ||
		!strings.Contains(viewerContractPage, "终止文书 / E2E-TERM-001") ||
		!strings.Contains(viewerContractPage, "更正文书 / E2E-CORR-001") ||
		!strings.Contains(viewerContractPage, "操作性关闭事实") ||
		!strings.Contains(viewerContractPage, "已登记最终结算") ||
		!strings.Contains(viewerContractPage, "操作性关闭佐证") ||
		!strings.Contains(viewerContractPage, "closure-proof.html") ||
		!strings.Contains(viewerContractPage, "closure-proof-v2.pdf") ||
		!strings.Contains(viewerContractPage, "更正文书佐证") ||
		!strings.Contains(viewerContractPage, "correction-proof.txt") ||
		!strings.Contains(viewerContractPage, "端到端关系目标合同") ||
		strings.Contains(viewerContractPage, `action="`+changePath+`"`) ||
		strings.Contains(viewerContractPage, `action="`+terminatePath+`"`) ||
		strings.Contains(viewerContractPage, `action="`+settlementPath+`"`) ||
		strings.Contains(viewerContractPage, `action="`+closePath+`"`) ||
		strings.Contains(viewerContractPage, `action="`+correctionPath+`"`) ||
		strings.Contains(viewerContractPage, `action="`+evidencePath+`"`) ||
		strings.Contains(viewerContractPage, `action="`+evidenceVersionPath+`"`) ||
		strings.Contains(viewerContractPage, exportPath) ||
		strings.Contains(viewerContractPage, contractPath+`/relations/new`) {
		t.Fatalf("viewer contract page timeline/form mismatch: %q", viewerContractPage)
	}
	response = request(t, viewerClient, http.MethodGet, testServer.URL+exportPath, nil)
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer ledger export status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	closeResponse(t, response)
	forbiddenCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	response = postForm(t, viewerClient, testServer.URL+changePath, url.Values{
		"csrf_token":              {csrfToken(t, viewerContractPage)},
		"command_id":              {forbiddenCommandID},
		"expected_revision":       {"3"},
		"effective_order":         {"4"},
		"instrument_number":       {"VIEWER-FORGED-SUPP"},
		"instrument_title":        {"只读用户伪造补充协议"},
		"change_summary":          {"不应写入"},
		"instrument_signed_on":    {"2026-10-03"},
		"instrument_effective_on": {"2026-10-03"},
		"amount_effect":           {"unchanged"},
	})
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer append change status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	closeResponse(t, response)
	if err := pool.QueryRow(ctx, `SELECT current_revision FROM contracts WHERE id = $1`, contractID).Scan(&currentRevision); err != nil {
		t.Fatalf("read revision after forbidden viewer change: %v", err)
	}
	if currentRevision != 4 {
		t.Fatalf("forbidden viewer change advanced revision to %d", currentRevision)
	}
	forbiddenTerminationCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	response = postForm(t, viewerClient, testServer.URL+terminatePath, url.Values{
		"csrf_token":                    {csrfToken(t, viewerContractPage)},
		"command_id":                    {forbiddenTerminationCommandID},
		"expected_revision":             {"3"},
		"effective_order":               {"4"},
		"termination_instrument_number": {"VIEWER-FORGED-TERM"},
		"termination_instrument_title":  {"只读用户伪造终止文书"},
		"termination_reason":            {"不应写入"},
		"termination_signed_on":         {"2026-10-04"},
		"termination_effective_on":      {"2026-10-04"},
	})
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer terminate status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	closeResponse(t, response)
	response = postForm(t, viewerClient, testServer.URL+correctionPath, url.Values{
		"csrf_token": {csrfToken(t, viewerContractPage)}, "expected_status": {"closed"},
		"expected_revision": {"4"}, "effective_order": {"5"},
		"correction_instrument_number": {"VIEWER-FORGED-CORR"},
	})
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer correction status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	closeResponse(t, response)
	response = postForm(t, viewerClient, testServer.URL+relationPath, url.Values{
		"csrf_token": {csrfToken(t, viewerContractPage)}, "target_contract_id": {relationTargetID},
		"relation_type": {"references"}, "expected_source_revision": {"3"}, "expected_target_revision": {"1"},
	})
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer link relation status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	closeResponse(t, response)
	response = postForm(t, viewerClient, testServer.URL+closePath, url.Values{
		"csrf_token": {csrfToken(t, viewerContractPage)}, "expected_revision": {"3"},
		"expected_settlement_sequence": {"2"}, "settlement_disposition": {"final_recorded"},
		"closed_on": {"2026-10-07"}, "closure_reason": {"只读用户不应能够关闭合同"},
	})
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer close contract status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	closeResponse(t, response)
	response = postForm(t, viewerClient, testServer.URL+evidencePath, url.Values{
		"csrf_token": {csrfToken(t, viewerContractPage)}, "evidence_title": {"只读用户伪造证据"},
	})
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer upload evidence status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	closeResponse(t, response)
	evidenceVersionTwoDownloadPath := evidenceVersionPath + "/" + evidenceVersionTwoID + "/download"
	response = request(t, viewerClient, http.MethodGet, testServer.URL+evidenceVersionTwoDownloadPath, nil)
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("viewer evidence download status=%d headers=%v body=%q", response.StatusCode, response.Header, readResponse(t, response))
	}
	if body := []byte(readResponse(t, response)); !bytes.Equal(body, evidenceVersionTwoContent) {
		t.Fatalf("viewer evidence download body=%q", body)
	}
	closeResponse(t, response)
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_entries WHERE object_id = $1 AND action = 'contract_evidence_downloaded' AND actor_id = $2`, contractID, viewerID).Scan(&evidenceDownloadAuditCount); err != nil {
		t.Fatalf("count viewer evidence download audit: %v", err)
	}
	if evidenceDownloadAuditCount != 1 {
		t.Fatalf("viewer evidence download audit count = %d", evidenceDownloadAuditCount)
	}
	viewerAuditPage := getPage(t, viewerClient, testServer.URL+"/audit", http.StatusOK)
	if !strings.Contains(viewerAuditPage, "系统验收编辑者") || !strings.Contains(viewerAuditPage, "party_created") ||
		!strings.Contains(viewerAuditPage, "contract_change_activated") || !strings.Contains(viewerAuditPage, "contract_terminated") ||
		!strings.Contains(viewerAuditPage, "contract_correction_recorded") ||
		!strings.Contains(viewerAuditPage, "contract_closed") || !strings.Contains(viewerAuditPage, "contract_evidence_created") ||
		!strings.Contains(viewerAuditPage, "contract_evidence_version_added") || !strings.Contains(viewerAuditPage, "contract_evidence_downloaded") ||
		!strings.Contains(viewerAuditPage, "contract_relation_linked") {
		t.Fatalf("viewer audit page missing trusted business audit: %q", viewerAuditPage)
	}

	var bootstrapAudits, createdUserAudits int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_entries WHERE action = 'local_user_bootstrapped'`).Scan(&bootstrapAudits); err != nil {
		t.Fatalf("count bootstrap audits: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_entries WHERE action = 'local_user_created'`).Scan(&createdUserAudits); err != nil {
		t.Fatalf("count created-user audits: %v", err)
	}
	if bootstrapAudits != 1 || createdUserAudits != 2 {
		t.Fatalf("identity audit counts bootstrap=%d user_created=%d", bootstrapAudits, createdUserAudits)
	}
}

func TestEmptyBusinessDatabaseHTTPRestoreAndReExport(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	sourcePool := openIsolatedSystemDatabase(t, ctx, databaseURL)
	targetPool := openIsolatedSystemDatabase(t, ctx, databaseURL)

	sourceActor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "71717171-7171-4717-8717-717171717171", DisplayName: "源管理员",
	}
	commandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatal(err)
	}
	sourceContract, err := contract.NewService(contract.NewPostgresRepository(sourcePool)).CreateDraft(ctx, contract.CreateDraftInput{
		CommandID: commandID, NumberNamespace: "HTTP-RESTORE", ContractNumber: "HR-001",
		Title: "HTTP 空库恢复合同", Category: "验收", Currency: "CNY", TotalAmount: "88.50",
		Actor: sourceActor,
	})
	if err != nil {
		t.Fatalf("create source contract: %v", err)
	}
	sourceExport, err := ledgerexport.NewService(ledgerexport.NewPostgresRepository(sourcePool))
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := sourceExport.Export(ctx, ledgerexport.Input{ContractID: sourceContract.ID, Actor: sourceActor})
	if err != nil {
		t.Fatalf("export source contract: %v", err)
	}

	identityService, err := identity.NewService(identity.NewPostgresRepository(targetPool), 2*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	created, err := identityService.EnsureBootstrap(ctx, identity.BootstrapInput{
		Username: "restore-admin", DisplayName: "恢复验收管理员", Password: "restore-admin-password",
	})
	if err != nil || !created {
		t.Fatalf("bootstrap target created=%v error=%v", created, err)
	}
	ledgerExportService, _ := ledgerexport.NewService(ledgerexport.NewPostgresRepository(targetPool))
	ledgerRestoreService, _ := ledgerrestore.NewService(ledgerrestore.NewPostgresRepository(targetPool))
	handler, err := httpserver.NewServer(
		targetPool,
		party.NewService(party.NewPostgresRepository(targetPool)),
		contract.NewService(contract.NewPostgresRepository(targetPool)),
		newSystemEvidenceService(t, targetPool),
		ledgerExportService,
		ledgerRestoreService,
		identityService,
		audit.NewRepository(targetPool),
		httpserver.Options{},
	)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	testServer := httptest.NewServer(handler)
	t.Cleanup(testServer.Close)
	client := newSessionClient(t)
	login(t, client, testServer.URL, "restore-admin", "restore-admin-password")
	restorePage := getPage(t, client, testServer.URL+"/admin/restore", http.StatusOK)
	restoreCommandID := formHiddenValue(t, restorePage, "/admin/restore", "command_id")
	response := postMultipart(t, client, testServer.URL+"/admin/restore", url.Values{
		"csrf_token": {csrfToken(t, restorePage)}, "command_id": {restoreCommandID}, "confirm_empty": {"yes"},
	}, "archive", artifact.Filename, artifact.Content)
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("restore status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	location := response.Header.Get("Location")
	closeResponse(t, response)
	if !strings.HasPrefix(location, "/contracts/"+sourceContract.ID+"?") || !strings.Contains(location, "restored=1") {
		t.Fatalf("restore location=%q", location)
	}
	detailPage := getPage(t, client, testServer.URL+location, http.StatusOK)
	if !strings.Contains(detailPage, "单合同账簿已恢复") || !strings.Contains(detailPage, "HTTP 空库恢复合同") {
		t.Fatalf("restored detail page=%q", detailPage)
	}

	response = request(t, client, http.MethodGet, testServer.URL+"/contracts/"+sourceContract.ID+"/export", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("re-export status=%d body=%q", response.StatusCode, readResponse(t, response))
	}
	reexported := []byte(readResponse(t, response))
	closeResponse(t, response)
	_, reexportedLedger, _ := decodeLedgerArchive(t, reexported)
	if reexportedLedger.Contract.ID != sourceContract.ID || reexportedLedger.Contract.Status != "draft" ||
		len(reexportedLedger.Audit) != 2 {
		t.Fatalf("re-exported ledger=%#v", reexportedLedger)
	}
	var contracts, restores, restoredAudits int
	if err := targetPool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM contracts),
			(SELECT count(*) FROM contract_restore_operations),
			(SELECT count(*) FROM audit_entries WHERE action = 'contract_ledger_restored')
	`).Scan(&contracts, &restores, &restoredAudits); err != nil {
		t.Fatal(err)
	}
	if contracts != 1 || restores != 1 || restoredAudits != 1 {
		t.Fatalf("contracts=%d restores=%d restored audits=%d", contracts, restores, restoredAudits)
	}
}

func decodeLedgerArchive(t *testing.T, content []byte) (ledgerexport.Manifest, ledgerexport.Ledger, []byte) {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("open ledger archive: %v", err)
	}
	if len(reader.File) != 2 {
		t.Fatalf("ledger archive file count=%d", len(reader.File))
	}
	files := make(map[string][]byte, 2)
	for _, file := range reader.File {
		if file.FileInfo().IsDir() || (file.Name != "manifest.json" && file.Name != "ledger.json") {
			t.Fatalf("unexpected ledger archive entry=%q", file.Name)
		}
		if _, exists := files[file.Name]; exists {
			t.Fatalf("duplicate ledger archive entry=%q", file.Name)
		}
		stream, err := file.Open()
		if err != nil {
			t.Fatalf("open ledger archive entry %q: %v", file.Name, err)
		}
		files[file.Name], err = io.ReadAll(stream)
		_ = stream.Close()
		if err != nil {
			t.Fatalf("read ledger archive entry %q: %v", file.Name, err)
		}
	}
	var manifest ledgerexport.Manifest
	if err := json.Unmarshal(files["manifest.json"], &manifest); err != nil {
		t.Fatalf("decode ledger manifest: %v", err)
	}
	var ledger ledgerexport.Ledger
	if err := json.Unmarshal(files["ledger.json"], &ledger); err != nil {
		t.Fatalf("decode ledger: %v", err)
	}
	return manifest, ledger, files["ledger.json"]
}

func newSystemEvidenceService(t *testing.T, pool *pgxpool.Pool) *evidence.Service {
	t.Helper()
	store, err := evidence.NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalStore() error = %v", err)
	}
	service, err := evidence.NewService(evidence.NewPostgresRepository(pool), store, 1<<20)
	if err != nil {
		t.Fatalf("evidence.NewService() error = %v", err)
	}
	return service
}

func newSessionClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error = %v", err)
	}
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 10 * time.Second,
	}
}

func login(t *testing.T, client *http.Client, baseURL, username, password string) {
	t.Helper()
	page := getPage(t, client, baseURL+"/login", http.StatusOK)
	response := postForm(t, client, baseURL+"/login", url.Values{
		"csrf_token": {csrfToken(t, page)}, "username": {username}, "password": {password}, "next": {"/"},
	})
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/" {
		t.Fatalf("login %q status=%d location=%q body=%q", username, response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
}

func getPage(t *testing.T, client *http.Client, target string, expectedStatus int) string {
	t.Helper()
	response := request(t, client, http.MethodGet, target, nil)
	defer closeResponse(t, response)
	body := readResponse(t, response)
	if response.StatusCode != expectedStatus {
		t.Fatalf("GET %s status=%d body=%q", target, response.StatusCode, body)
	}
	return body
}

func postForm(t *testing.T, client *http.Client, target string, values url.Values) *http.Response {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, target, strings.NewReader(values.Encode()))
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("POST %s error = %v", target, err)
	}
	return response
}

func postMultipart(t *testing.T, client *http.Client, target string, values url.Values, fileField, filename string, content []byte) *http.Response {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, fieldValues := range values {
		for _, value := range fieldValues {
			if err := writer.WriteField(name, value); err != nil {
				t.Fatalf("write multipart field %q: %v", name, err)
			}
		}
	}
	part, err := writer.CreateFormFile(fileField, filename)
	if err != nil {
		t.Fatalf("create multipart file %q: %v", filename, err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write multipart file %q: %v", filename, err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	request, err := http.NewRequest(http.MethodPost, target, &body)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("POST multipart %s error = %v", target, err)
	}
	return response
}

func request(t *testing.T, client *http.Client, method, target string, body io.Reader) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, target, body)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("%s %s error = %v", method, target, err)
	}
	return response
}

func csrfToken(t *testing.T, body string) string {
	t.Helper()
	match := csrfInputPattern.FindStringSubmatch(body)
	if len(match) != 2 {
		t.Fatalf("page missing CSRF token: %q", body)
	}
	return match[1]
}

func formHiddenValue(t *testing.T, body, action, name string) string {
	t.Helper()
	formPattern := regexp.MustCompile(`(?s)<form[^>]*action="` + regexp.QuoteMeta(action) + `"[^>]*>(.*?)</form>`)
	formMatch := formPattern.FindStringSubmatch(body)
	if len(formMatch) != 2 {
		t.Fatalf("page missing form action %q: %q", action, body)
	}
	inputPattern := regexp.MustCompile(`name="` + regexp.QuoteMeta(name) + `" value="([^"]*)"`)
	inputMatch := inputPattern.FindStringSubmatch(formMatch[1])
	if len(inputMatch) != 2 {
		t.Fatalf("form %q missing hidden input %q: %q", action, name, formMatch[1])
	}
	return inputMatch[1]
}

func readResponse(t *testing.T, response *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	return string(body)
}

func closeResponse(t *testing.T, response *http.Response) {
	t.Helper()
	if err := response.Body.Close(); err != nil {
		t.Errorf("close response: %v", err)
	}
}

func openIsolatedSystemDatabase(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
	t.Helper()
	baseConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	adminConfig := baseConfig.Copy()
	adminConfig.ConnConfig.Database = "postgres"
	adminPool, err := pgxpool.NewWithConfig(ctx, adminConfig)
	if err != nil {
		t.Fatalf("open administration database: %v", err)
	}
	defer adminPool.Close()

	databaseName := fmt.Sprintf("contract_ledger_system_%d", time.Now().UnixNano())
	quotedName := pgx.Identifier{databaseName}.Sanitize()
	if _, err := adminPool.Exec(ctx, `CREATE DATABASE `+quotedName); err != nil {
		t.Fatalf("create isolated database: %v", err)
	}

	testConfig := baseConfig.Copy()
	testConfig.ConnConfig.Database = databaseName
	pool, err := pgxpool.NewWithConfig(ctx, testConfig)
	if err != nil {
		t.Fatalf("open isolated database: %v", err)
	}
	if err := database.Migrate(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("migrate isolated database: %v", err)
	}
	if err := database.Migrate(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("repeat migrations: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cleanupConfig := baseConfig.Copy()
		cleanupConfig.ConnConfig.Database = "postgres"
		cleanupPool, cleanupErr := pgxpool.NewWithConfig(cleanupCtx, cleanupConfig)
		if cleanupErr != nil {
			t.Errorf("open cleanup database: %v", cleanupErr)
			return
		}
		defer cleanupPool.Close()
		if _, cleanupErr := cleanupPool.Exec(cleanupCtx, `DROP DATABASE `+quotedName+` WITH (FORCE)`); cleanupErr != nil {
			t.Errorf("drop isolated database %q: %v", databaseName, cleanupErr)
		}
	})
	return pool
}

func TestSystemDatabaseNameIsSafe(t *testing.T) {
	name := pgx.Identifier{"contract_ledger_system_123"}.Sanitize()
	if strings.ContainsAny(name, ";\n") {
		t.Fatalf("unsafe identifier = %q", name)
	}
}
