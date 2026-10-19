package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	commandOwnerParty         = "62000000-0000-4000-8000-000000000001"
	commandClientParty        = "62000000-0000-4000-8000-000000000002"
	commandObligationCreate   = "62000000-0000-4000-8000-000000000003"
	commandObligationConfirm  = "62000000-0000-4000-8000-000000000004"
	commandObligationAdjust   = "62000000-0000-4000-8000-000000000005"
	commandCashCreate         = "62000000-0000-4000-8000-000000000006"
	commandAllocationCreate   = "62000000-0000-4000-8000-000000000007"
	commandRemission          = "62000000-0000-4000-8000-000000000008"
	commandClosure            = "62000000-0000-4000-8000-000000000009"
	commandErrorCashCreate    = "62000000-0000-4000-8000-000000000010"
	commandErrorCashReverse   = "62000000-0000-4000-8000-000000000011"
	commandInvalidAdjustment  = "62000000-0000-4000-8000-000000000012"
	commandBlockedCashReverse = "62000000-0000-4000-8000-000000000013"
	commandAllocationReverse  = "62000000-0000-4000-8000-000000000014"
	commandReplacementAlloc   = "62000000-0000-4000-8000-000000000015"
	commandSecondCashCreate   = "62000000-0000-4000-8000-000000000016"
	commandSecondAllocation   = "62000000-0000-4000-8000-000000000017"
	commandCashExpectation    = "62000000-0000-4000-8000-000000000018"
	commandInternalTransfer   = "62000000-0000-4000-8000-000000000019"
)

type config struct {
	baseURL        string
	username       string
	password       string
	referenceToken string
	statePath      string
}

type demoState struct {
	OwnerPartyID            string `json:"owner_party_id"`
	ClientPartyID           string `json:"client_party_id"`
	ObligationID            string `json:"obligation_id"`
	ObligationConfirmed     bool   `json:"obligation_confirmed"`
	ObligationAdjusted      bool   `json:"obligation_adjusted"`
	CashExpectationSet      bool   `json:"cash_expectation_set"`
	CashTransactionID       string `json:"cash_transaction_id"`
	AllocationID            string `json:"allocation_id"`
	InitialAllocationAt     string `json:"initial_allocation_at"`
	AllocationReversed      bool   `json:"allocation_reversed"`
	AllocationReversedAt    string `json:"allocation_reversed_at"`
	ReplacementAllocationID string `json:"replacement_allocation_id"`
	SecondCashTransactionID string `json:"second_cash_transaction_id"`
	SecondAllocationID      string `json:"second_allocation_id"`
	FullyAllocatedAt        string `json:"fully_allocated_at"`
	InternalTransferID      string `json:"internal_transfer_id"`
	RemissionRecorded       bool   `json:"remission_recorded"`
	ObligationClosed        bool   `json:"obligation_closed"`
	ObligationClosedAt      string `json:"obligation_closed_at"`
	ErrorTransactionID      string `json:"error_transaction_id"`
	ErrorCashReversed       bool   `json:"error_cash_reversed"`
}

type resourceReference struct {
	ID           string `json:"id"`
	TenantID     string `json:"tenant_id"`
	ResourceType string `json:"resource_type"`
	Status       string `json:"status"`
	Revision     int64  `json:"revision"`
}

type formClient struct {
	baseURL string
	http    *http.Client
}

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: money-flow-demo")
		os.Exit(2)
	}
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "Money Flow Ledger demo failed:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	state, err := readState(cfg.statePath)
	if err != nil {
		return err
	}

	fmt.Println("[1/13] 验证本地模板身份的失败关闭")
	if err := verifyAnonymousRedirect(ctx, cfg.baseURL); err != nil {
		return err
	}
	client, err := newFormClient(cfg.baseURL)
	if err != nil {
		return err
	}
	if err := client.verifyWrongPassword(ctx, cfg.username); err != nil {
		return err
	}
	if err := client.login(ctx, cfg.username, cfg.password); err != nil {
		return err
	}

	fmt.Println("[2/13] 登记本方与往来方主体")
	if err := ensureParties(ctx, client, cfg.statePath, &state); err != nil {
		return err
	}
	fmt.Println("[3/13] 创建并确认应收义务 revision 1")
	if err := ensureObligation(ctx, client, cfg.statePath, &state); err != nil {
		return err
	}
	fmt.Println("[4/13] 受控调额形成不可变 revision 2")
	if err := ensureAdjustment(ctx, client, cfg.statePath, &state); err != nil {
		return err
	}
	fmt.Println("[5/13] 为当前 125,000 元未结余额设置预计回款日")
	if err := ensureCashExpectation(ctx, client, cfg.statePath, &state); err != nil {
		return err
	}
	fmt.Println("[6/13] 登记首笔 40,000 元收款并核销到义务")
	if err := ensureCashAndAllocation(ctx, client, cfg.statePath, &state); err != nil {
		return err
	}
	fmt.Println("[7/13] 反向错误核销并建立正确的替代核销")
	if err := ensureAllocationReversalAndReplacement(ctx, client, cfg.statePath, &state); err != nil {
		return err
	}
	fmt.Println("[8/13] 登记第二笔 30,000 元收款，形成多笔有效核销")
	if err := ensureSecondCashAndAllocation(ctx, client, cfg.statePath, &state); err != nil {
		return err
	}
	fmt.Println("[9/13] 登记一笔不进入经营收支和义务核销的内部账户转账")
	if err := ensureInternalTransfer(ctx, client, cfg.statePath, &state); err != nil {
		return err
	}
	fmt.Println("[10/13] 减免剩余 55,000 元并受控关闭零余额义务")
	if err := ensureRemissionAndClosure(ctx, client, cfg.statePath, &state); err != nil {
		return err
	}
	fmt.Println("[11/13] 登记并冲销一笔错误收款，同时拒绝已核销收款的直接冲销")
	if err := ensureErrorCashReversal(ctx, client, cfg.statePath, &state); err != nil {
		return err
	}
	if err := verifyBlockedAllocatedCashReversal(ctx, client, state.CashTransactionID); err != nil {
		return err
	}
	fmt.Println("[12/13] 拒绝关闭后的调额，重放既有命令并核对历史余额、预测、查询、汇总和导出")
	if err := verifyInvalidAdjustmentAfterClosure(ctx, client, state.ObligationID); err != nil {
		return err
	}
	if err := replayAdjustment(ctx, client, state.ObligationID); err != nil {
		return err
	}
	if err := verifyBusinessViews(ctx, client, state); err != nil {
		return err
	}

	fmt.Println("[13/13] 核对版本化 reference-read 及错误凭证")
	obligationRef, err := readReference(ctx, cfg.baseURL, cfg.referenceToken, "/api/v1/references/obligations/"+state.ObligationID, http.StatusOK)
	if err != nil {
		return err
	}
	cashRef, err := readReference(ctx, cfg.baseURL, cfg.referenceToken, "/api/v1/references/transactions/"+state.CashTransactionID, http.StatusOK)
	if err != nil {
		return err
	}
	if obligationRef.ID != state.ObligationID || obligationRef.ResourceType != "obligation" || obligationRef.Status != "closed" || obligationRef.Revision != 2 || obligationRef.TenantID == "" {
		return fmt.Errorf("unexpected obligation reference: %+v", obligationRef)
	}
	if cashRef.ID != state.CashTransactionID || cashRef.ResourceType != "transaction" || cashRef.Status != "recorded" || cashRef.Revision != 1 || cashRef.TenantID == "" {
		return fmt.Errorf("unexpected cash reference: %+v", cashRef)
	}
	if _, err := readReference(ctx, cfg.baseURL, "wrong-reference-token", "/api/v1/references/obligations/"+state.ObligationID, http.StatusUnauthorized); err != nil {
		return err
	}

	fmt.Printf("PASS: Money Flow Ledger 独立 Demo obligation=%s status=%s revision=%d receipt=%s\n", obligationRef.ID, obligationRef.Status, obligationRef.Revision, cashRef.ID)
	fmt.Println("      125,000 CNY = 两笔收款 70,000 已核销 + 55,000 已减免；错误核销和错误收款均保留冲销轨迹。")
	fmt.Println("      预计现金、历史余额和内部转账已经纳入同一条可重放业务故事；重放未新增 revision。")
	fmt.Println("      浏览器入口：" + cfg.baseURL + "/operating-summary")
	return nil
}

func loadConfig() (config, error) {
	cfg := config{
		baseURL:        envOr("DEMO_MONEY_URL", "http://127.0.0.1:28083"),
		username:       strings.TrimSpace(os.Getenv("DEMO_ADMIN_USERNAME")),
		password:       os.Getenv("DEMO_ADMIN_PASSWORD"),
		referenceToken: os.Getenv("DEMO_REFERENCE_ACCESS_TOKEN"),
		statePath:      envOr("DEMO_STATE_PATH", "deployments/demos/money-flow-ledger/.runtime/state.json"),
	}
	if cfg.username == "" || cfg.password == "" || len(cfg.referenceToken) < 24 {
		return config{}, errors.New("DEMO_ADMIN_USERNAME, DEMO_ADMIN_PASSWORD and a 24+ byte DEMO_REFERENCE_ACCESS_TOKEN are required")
	}
	return cfg, nil
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return strings.TrimRight(value, "/")
	}
	return fallback
}

func verifyAnonymousRedirect(ctx context.Context, baseURL string) error {
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/obligations", nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("anonymous GET /obligations: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusSeeOther || !strings.HasPrefix(response.Header.Get("Location"), "/login?next=") {
		return fmt.Errorf("anonymous GET /obligations status=%d location=%q", response.StatusCode, response.Header.Get("Location"))
	}
	return nil
}

func newFormClient(baseURL string) (*formClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &formClient{baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{
		Timeout:       10 * time.Second,
		Jar:           jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func (c *formClient) verifyWrongPassword(ctx context.Context, username string) error {
	page, err := c.get(ctx, "/login")
	if err != nil {
		return err
	}
	form, err := formByAction(page, "/login")
	if err != nil {
		return err
	}
	body, _, err := c.postExpect(ctx, "/login", url.Values{
		"csrf_token": {hiddenValue(form, "csrf_token")}, "username": {username},
		"password": {"definitely-wrong-demo-password"}, "next": {"/"},
	}, http.StatusUnauthorized)
	if err != nil {
		return err
	}
	if !strings.Contains(body, "用户名或密码错误") {
		return errors.New("wrong-password response did not contain the expected authentication failure")
	}
	return nil
}

func (c *formClient) login(ctx context.Context, username, password string) error {
	page, err := c.get(ctx, "/login")
	if err != nil {
		return err
	}
	form, err := formByAction(page, "/login")
	if err != nil {
		return err
	}
	_, _, err = c.postExpect(ctx, "/login", url.Values{
		"csrf_token": {hiddenValue(form, "csrf_token")}, "username": {username}, "password": {password}, "next": {"/"},
	}, http.StatusSeeOther)
	return err
}

func ensureParties(ctx context.Context, client *formClient, statePath string, state *demoState) error {
	specs := []struct {
		id      *string
		name    string
		code    string
		command string
	}{
		{&state.OwnerPartyID, "青岚设计工作室", "BF-MONEY-DEMO-OWNER", commandOwnerParty},
		{&state.ClientPartyID, "星禾科技有限公司", "BF-MONEY-DEMO-CLIENT", commandClientParty},
	}
	for _, spec := range specs {
		if *spec.id != "" {
			continue
		}
		page, err := client.get(ctx, "/parties")
		if err != nil {
			return err
		}
		form, err := formByAction(page, "/parties")
		if err != nil {
			return err
		}
		_, location, err := client.postExpect(ctx, "/parties", url.Values{
			"csrf_token": {hiddenValue(form, "csrf_token")}, "command_id": {spec.command}, "kind": {"organization"},
			"legal_name": {spec.name}, "registration_identifier": {spec.code},
		}, http.StatusSeeOther)
		if err != nil {
			return fmt.Errorf("create party %s: %w", spec.code, err)
		}
		*spec.id, err = queryID(location, "party_id")
		if err != nil {
			return err
		}
		if err := writeState(statePath, *state); err != nil {
			return err
		}
	}
	return nil
}

func ensureObligation(ctx context.Context, client *formClient, statePath string, state *demoState) error {
	if state.ObligationID == "" {
		page, err := client.get(ctx, "/obligations")
		if err != nil {
			return err
		}
		form, err := formByAction(page, "/obligations")
		if err != nil {
			return err
		}
		_, location, err := client.postExpect(ctx, "/obligations", url.Values{
			"csrf_token": {hiddenValue(form, "csrf_token")}, "command_id": {commandObligationCreate}, "direction": {"receivable"},
			"ledger_owner_party_id": {state.OwnerPartyID}, "counterparty_party_id": {state.ClientPartyID},
			"title": {"青岚项目设计服务费"}, "original_amount": {"120000.00"}, "currency": {"CNY"},
			"due_on": {"2026-10-31"}, "manual_source_reference": {"BF Money Demo 合同快照 revision 1"},
		}, http.StatusSeeOther)
		if err != nil {
			return fmt.Errorf("create obligation: %w", err)
		}
		state.ObligationID, err = pathID(location, "/obligations/")
		if err != nil {
			return err
		}
		if err := writeState(statePath, *state); err != nil {
			return err
		}
	}
	if state.ObligationConfirmed {
		return nil
	}
	path := "/obligations/" + state.ObligationID
	page, err := client.get(ctx, path)
	if err != nil {
		return err
	}
	form, err := formByAction(page, path+"/confirm")
	if err != nil {
		if strings.Contains(page, "Current revision</dt><dd>1</dd>") {
			state.ObligationConfirmed = true
			return writeState(statePath, *state)
		}
		return err
	}
	if _, _, err := client.postExpect(ctx, path+"/confirm", url.Values{
		"csrf_token": {hiddenValue(form, "csrf_token")}, "command_id": {commandObligationConfirm},
		"expected_draft_version": {hiddenValue(form, "expected_draft_version")},
	}, http.StatusSeeOther); err != nil {
		return fmt.Errorf("confirm obligation: %w", err)
	}
	state.ObligationConfirmed = true
	return writeState(statePath, *state)
}

func adjustmentValues(csrf string) url.Values {
	return url.Values{
		"csrf_token": {csrf}, "command_id": {commandObligationAdjust}, "expected_current_revision": {"1"},
		"adjustment_type": {"increase"}, "adjustment_amount": {"5000.00"}, "adjustment_reason": {"补充共享会议区设计范围"},
	}
}

func ensureAdjustment(ctx context.Context, client *formClient, statePath string, state *demoState) error {
	if state.ObligationAdjusted {
		return nil
	}
	path := "/obligations/" + state.ObligationID
	page, err := client.get(ctx, path)
	if err != nil {
		return err
	}
	form, err := formByAction(page, path+"/adjust-amount")
	if err != nil {
		if strings.Contains(page, "Current revision</dt><dd>2</dd>") {
			state.ObligationAdjusted = true
			return writeState(statePath, *state)
		}
		return err
	}
	values := adjustmentValues(hiddenValue(form, "csrf_token"))
	values.Set("expected_current_revision", hiddenValue(form, "expected_current_revision"))
	if _, _, err := client.postExpect(ctx, path+"/adjust-amount", values, http.StatusSeeOther); err != nil {
		return fmt.Errorf("adjust obligation: %w", err)
	}
	state.ObligationAdjusted = true
	return writeState(statePath, *state)
}

func ensureCashExpectation(ctx context.Context, client *formClient, statePath string, state *demoState) error {
	if state.CashExpectationSet {
		return nil
	}
	page, err := client.get(ctx, "/cash-forecast")
	if err != nil {
		return err
	}
	form, err := formByAction(page, "/cash-expectations")
	if err != nil {
		return err
	}
	if _, _, err := client.postExpect(ctx, "/cash-expectations", url.Values{
		"csrf_token":           {hiddenValue(form, "csrf_token")},
		"command_id":           {commandCashExpectation},
		"action":               {"set"},
		"obligation_selection": {state.ObligationID + ":2"},
		"expected_on":          {"2026-10-20"},
		"expectation_reason":   {"客户确认预计分两笔回款"},
	}, http.StatusSeeOther); err != nil {
		return fmt.Errorf("set cash expectation: %w", err)
	}
	state.CashExpectationSet = true
	if err := writeState(statePath, *state); err != nil {
		return err
	}
	forecast, err := client.get(ctx, "/cash-forecast?history_obligation_id="+state.ObligationID)
	if err != nil {
		return err
	}
	for _, want := range []string{"2026-10-20", "125000 CNY", "客户确认预计分两笔回款", "seq 1"} {
		if !strings.Contains(forecast, want) {
			return fmt.Errorf("cash forecast is missing %q", want)
		}
	}
	return nil
}

func ensureCashAndAllocation(ctx context.Context, client *formClient, statePath string, state *demoState) error {
	if state.CashTransactionID == "" {
		page, err := client.get(ctx, "/cash-transactions")
		if err != nil {
			return err
		}
		form, err := formByAction(page, "/cash-transactions")
		if err != nil {
			return err
		}
		_, location, err := client.postExpect(ctx, "/cash-transactions", cashValues(hiddenValue(form, "csrf_token"), commandCashCreate, state, "青岚项目首笔阶段收款", "40000.00", "BF-MONEY-BANK-001"), http.StatusSeeOther)
		if err != nil {
			return fmt.Errorf("create cash receipt: %w", err)
		}
		state.CashTransactionID, err = pathID(location, "/cash-transactions/")
		if err != nil {
			return err
		}
		if err := writeState(statePath, *state); err != nil {
			return err
		}
	}
	if state.AllocationID != "" {
		return nil
	}
	page, err := client.get(ctx, "/allocations")
	if err != nil {
		return err
	}
	form, err := formByAction(page, "/allocations")
	if err != nil {
		return err
	}
	_, location, err := client.postExpect(ctx, "/allocations", url.Values{
		"csrf_token": {hiddenValue(form, "csrf_token")}, "command_id": {commandAllocationCreate},
		"cash_transaction_id": {state.CashTransactionID}, "obligation_selection": {state.ObligationID + ":2"},
		"amount": {"40000.00"}, "manual_source_reference": {"银行回单 BF-MONEY-BANK-001 首次核销至设计服务费"},
	}, http.StatusSeeOther)
	if err != nil {
		return fmt.Errorf("allocate cash receipt: %w", err)
	}
	state.AllocationID, err = pathID(location, "/allocations/")
	if err != nil {
		return err
	}
	state.InitialAllocationAt = checkpointNow()
	return writeState(statePath, *state)
}

func ensureAllocationReversalAndReplacement(ctx context.Context, client *formClient, statePath string, state *demoState) error {
	if !state.AllocationReversed {
		path := "/allocations/" + state.AllocationID
		page, err := client.get(ctx, path)
		if err != nil {
			return err
		}
		form, err := formByAction(page, path+"/reverse")
		if err != nil {
			return err
		}
		if _, _, err := client.postExpect(ctx, path+"/reverse", url.Values{
			"csrf_token": {hiddenValue(form, "csrf_token")},
			"command_id": {commandAllocationReverse},
			"reason":     {"首次核销依据匹配错误，完整释放后重新建立"},
		}, http.StatusSeeOther); err != nil {
			return fmt.Errorf("reverse initial allocation: %w", err)
		}
		state.AllocationReversed = true
		state.AllocationReversedAt = checkpointNow()
		if err := writeState(statePath, *state); err != nil {
			return err
		}
	}
	if state.ReplacementAllocationID != "" {
		return nil
	}
	page, err := client.get(ctx, "/allocations")
	if err != nil {
		return err
	}
	form, err := formByAction(page, "/allocations")
	if err != nil {
		return err
	}
	_, location, err := client.postExpect(ctx, "/allocations", url.Values{
		"csrf_token": {hiddenValue(form, "csrf_token")}, "command_id": {commandReplacementAlloc},
		"cash_transaction_id": {state.CashTransactionID}, "obligation_selection": {state.ObligationID + ":2"},
		"amount": {"40000.00"}, "manual_source_reference": {"银行回单 BF-MONEY-BANK-001 复核后重新核销"},
	}, http.StatusSeeOther)
	if err != nil {
		return fmt.Errorf("create replacement allocation: %w", err)
	}
	state.ReplacementAllocationID, err = pathID(location, "/allocations/")
	if err != nil {
		return err
	}
	return writeState(statePath, *state)
}

func ensureSecondCashAndAllocation(ctx context.Context, client *formClient, statePath string, state *demoState) error {
	if state.SecondCashTransactionID == "" {
		page, err := client.get(ctx, "/cash-transactions")
		if err != nil {
			return err
		}
		form, err := formByAction(page, "/cash-transactions")
		if err != nil {
			return err
		}
		_, location, err := client.postExpect(ctx, "/cash-transactions", cashValues(hiddenValue(form, "csrf_token"), commandSecondCashCreate, state, "青岚项目第二笔阶段收款", "30000.00", "BF-MONEY-BANK-002"), http.StatusSeeOther)
		if err != nil {
			return fmt.Errorf("create second cash receipt: %w", err)
		}
		state.SecondCashTransactionID, err = pathID(location, "/cash-transactions/")
		if err != nil {
			return err
		}
		if err := writeState(statePath, *state); err != nil {
			return err
		}
	}
	if state.SecondAllocationID != "" {
		return nil
	}
	page, err := client.get(ctx, "/allocations")
	if err != nil {
		return err
	}
	form, err := formByAction(page, "/allocations")
	if err != nil {
		return err
	}
	_, location, err := client.postExpect(ctx, "/allocations", url.Values{
		"csrf_token": {hiddenValue(form, "csrf_token")}, "command_id": {commandSecondAllocation},
		"cash_transaction_id": {state.SecondCashTransactionID}, "obligation_selection": {state.ObligationID + ":2"},
		"amount": {"30000.00"}, "manual_source_reference": {"银行回单 BF-MONEY-BANK-002 核销至设计服务费"},
	}, http.StatusSeeOther)
	if err != nil {
		return fmt.Errorf("allocate second cash receipt: %w", err)
	}
	state.SecondAllocationID, err = pathID(location, "/allocations/")
	if err != nil {
		return err
	}
	state.FullyAllocatedAt = checkpointNow()
	return writeState(statePath, *state)
}

func ensureInternalTransfer(ctx context.Context, client *formClient, statePath string, state *demoState) error {
	if state.InternalTransferID != "" {
		return nil
	}
	page, err := client.get(ctx, "/internal-transfers")
	if err != nil {
		return err
	}
	form, err := formByAction(page, "/internal-transfers")
	if err != nil {
		return err
	}
	_, location, err := client.postExpect(ctx, "/internal-transfers", url.Values{
		"csrf_token": {hiddenValue(form, "csrf_token")}, "command_id": {commandInternalTransfer},
		"ledger_owner_party_id": {state.OwnerPartyID}, "description": {"基本户向项目备用金账户转账"},
		"amount": {"5000.00"}, "currency": {"CNY"}, "occurred_on": {"2026-09-25"},
		"source_account_reference": {"基本户尾号 2026"}, "destination_account_reference": {"项目备用金账户"},
		"manual_source_reference": {"银行回单 BF-MONEY-TRANSFER-001"}, "external_transfer_reference": {"BF-MONEY-TRANSFER-001"},
	}, http.StatusSeeOther)
	if err != nil {
		return fmt.Errorf("create internal transfer: %w", err)
	}
	state.InternalTransferID, err = pathID(location, "/internal-transfers/")
	if err != nil {
		return err
	}
	return writeState(statePath, *state)
}

func cashValues(csrf, command string, state *demoState, description, amount, externalReference string) url.Values {
	return url.Values{
		"csrf_token": {csrf}, "command_id": {command}, "type": {"receipt"},
		"ledger_owner_party_id": {state.OwnerPartyID}, "counterparty_party_id": {state.ClientPartyID},
		"description": {description}, "amount": {amount}, "currency": {"CNY"}, "occurred_on": {"2026-09-25"},
		"account_reference": {"演示账户尾号 2026"}, "manual_source_reference": {"人工核对银行回单"},
		"external_transaction_reference": {externalReference},
	}
}

func ensureRemissionAndClosure(ctx context.Context, client *formClient, statePath string, state *demoState) error {
	path := "/obligations/" + state.ObligationID
	if !state.RemissionRecorded {
		page, err := client.get(ctx, path)
		if err != nil {
			return err
		}
		form, err := formByAction(page, path+"/remissions")
		if err != nil {
			return err
		}
		if _, _, err := client.postExpect(ctx, path+"/remissions", url.Values{
			"csrf_token": {hiddenValue(form, "csrf_token")}, "command_id": {commandRemission},
			"expected_current_revision": {hiddenValue(form, "expected_current_revision")}, "remission_amount": {"55000.00"},
			"remission_reason": {"双方确认取消后续未实施范围"}, "manual_source_reference": {"BF-MONEY-DEMO-REMISSION-001"},
		}, http.StatusSeeOther); err != nil {
			return fmt.Errorf("record obligation remission: %w", err)
		}
		state.RemissionRecorded = true
		if err := writeState(statePath, *state); err != nil {
			return err
		}
	}
	if state.ObligationClosed {
		return nil
	}
	page, err := client.get(ctx, path)
	if err != nil {
		return err
	}
	form, err := formByAction(page, path+"/close")
	if err != nil {
		if strings.Contains(page, "关闭事实") {
			state.ObligationClosed = true
			return writeState(statePath, *state)
		}
		return err
	}
	if _, _, err := client.postExpect(ctx, path+"/close", url.Values{
		"csrf_token": {hiddenValue(form, "csrf_token")}, "command_id": {commandClosure},
		"expected_current_revision": {hiddenValue(form, "expected_current_revision")},
		"expected_balance_sequence": {hiddenValue(form, "expected_balance_sequence")},
		"closure_reason":            {"收款核销与范围减免已逐项复核，余额归零"},
	}, http.StatusSeeOther); err != nil {
		return fmt.Errorf("close obligation: %w", err)
	}
	state.ObligationClosed = true
	state.ObligationClosedAt = checkpointNow()
	return writeState(statePath, *state)
}

func ensureErrorCashReversal(ctx context.Context, client *formClient, statePath string, state *demoState) error {
	if state.ErrorTransactionID == "" {
		page, err := client.get(ctx, "/cash-transactions")
		if err != nil {
			return err
		}
		form, err := formByAction(page, "/cash-transactions")
		if err != nil {
			return err
		}
		_, location, err := client.postExpect(ctx, "/cash-transactions", cashValues(hiddenValue(form, "csrf_token"), commandErrorCashCreate, state, "误录测试收款", "1000.00", "BF-MONEY-BANK-ERROR-001"), http.StatusSeeOther)
		if err != nil {
			return fmt.Errorf("create erroneous cash receipt: %w", err)
		}
		state.ErrorTransactionID, err = pathID(location, "/cash-transactions/")
		if err != nil {
			return err
		}
		if err := writeState(statePath, *state); err != nil {
			return err
		}
	}
	if state.ErrorCashReversed {
		return nil
	}
	path := "/cash-transactions/" + state.ErrorTransactionID
	page, err := client.get(ctx, path)
	if err != nil {
		return err
	}
	form, err := formByAction(page, path+"/reverse")
	if err != nil {
		if strings.Contains(page, "资金冲销事实") {
			state.ErrorCashReversed = true
			return writeState(statePath, *state)
		}
		return err
	}
	if _, _, err := client.postExpect(ctx, path+"/reverse", url.Values{
		"csrf_token": {hiddenValue(form, "csrf_token")}, "command_id": {commandErrorCashReverse}, "reason": {"发现重复录入，保留冲销轨迹"},
	}, http.StatusSeeOther); err != nil {
		return fmt.Errorf("reverse erroneous cash receipt: %w", err)
	}
	state.ErrorCashReversed = true
	return writeState(statePath, *state)
}

func verifyBlockedAllocatedCashReversal(ctx context.Context, client *formClient, transactionID string) error {
	path := "/cash-transactions/" + transactionID
	page, err := client.get(ctx, path)
	if err != nil {
		return err
	}
	body, _, err := client.postExpect(ctx, path+"/reverse", url.Values{
		"csrf_token": {hiddenValue(page, "csrf_token")}, "command_id": {commandBlockedCashReverse}, "reason": {"验证已核销资金不能直接冲销"},
	}, http.StatusConflict)
	if err != nil {
		return err
	}
	if !strings.Contains(body, "请先") || !strings.Contains(body, "Allocation") {
		return errors.New("allocated cash reversal did not fail with the expected allocation dependency")
	}
	return nil
}

func verifyInvalidAdjustmentAfterClosure(ctx context.Context, client *formClient, obligationID string) error {
	path := "/obligations/" + obligationID
	page, err := client.get(ctx, path)
	if err != nil {
		return err
	}
	body, _, err := client.postExpect(ctx, path+"/adjust-amount", url.Values{
		"csrf_token": {hiddenValue(page, "csrf_token")}, "command_id": {commandInvalidAdjustment},
		"expected_current_revision": {"2"}, "adjustment_type": {"increase"}, "adjustment_amount": {"1.00"}, "adjustment_reason": {"关闭后不得调额"},
	}, http.StatusConflict)
	if err != nil {
		return err
	}
	if !strings.Contains(body, "只有 open 状态的义务可以调整金额") {
		return errors.New("closed obligation did not return the expected adjustment conflict")
	}
	return nil
}

func replayAdjustment(ctx context.Context, client *formClient, obligationID string) error {
	path := "/obligations/" + obligationID
	page, err := client.get(ctx, path)
	if err != nil {
		return err
	}
	if _, _, err := client.postExpect(ctx, path+"/adjust-amount", adjustmentValues(hiddenValue(page, "csrf_token")), http.StatusSeeOther); err != nil {
		return fmt.Errorf("replay adjustment command: %w", err)
	}
	detail, err := client.get(ctx, path)
	if err != nil {
		return err
	}
	if !strings.Contains(detail, "Current revision</dt><dd>2</dd>") || !strings.Contains(detail, "关闭事实") {
		return errors.New("adjustment replay changed history or reopened the obligation")
	}
	return nil
}

func verifyBusinessViews(ctx context.Context, client *formClient, state demoState) error {
	for name, value := range map[string]string{
		"initial allocation checkpoint":  state.InitialAllocationAt,
		"allocation reversal checkpoint": state.AllocationReversedAt,
		"fully allocated checkpoint":     state.FullyAllocatedAt,
		"obligation closure checkpoint":  state.ObligationClosedAt,
	} {
		if value == "" {
			return fmt.Errorf("%s is missing from demo state", name)
		}
	}
	obligation, err := client.get(ctx, "/obligations/"+state.ObligationID)
	if err != nil {
		return err
	}
	for _, want := range []string{"青岚项目设计服务费", "closed", "Current revision</dt><dd>2</dd>", "当前有效金额</dt><dd>125000 CNY", "已分配金额</dt><dd>70000 CNY", "已减免金额</dt><dd>55000 CNY", "剩余金额</dt><dd>0 CNY", "资金分配与减免组合归零", "关闭事实"} {
		if !strings.Contains(obligation, want) {
			return fmt.Errorf("obligation detail is missing %q", want)
		}
	}
	mainCash, err := client.get(ctx, "/cash-transactions/"+state.CashTransactionID)
	if err != nil {
		return err
	}
	for _, want := range []string{"青岚项目首笔阶段收款", "recorded", "当前有效金额</dt><dd>40000 CNY", "已分配金额</dt><dd>40000 CNY", "未分配金额</dt><dd>0 CNY", state.AllocationID, state.ReplacementAllocationID} {
		if !strings.Contains(mainCash, want) {
			return fmt.Errorf("main cash detail is missing %q", want)
		}
	}
	secondCash, err := client.get(ctx, "/cash-transactions/"+state.SecondCashTransactionID)
	if err != nil {
		return err
	}
	for _, want := range []string{"青岚项目第二笔阶段收款", "recorded", "当前有效金额</dt><dd>30000 CNY", "已分配金额</dt><dd>30000 CNY", "未分配金额</dt><dd>0 CNY", state.SecondAllocationID} {
		if !strings.Contains(secondCash, want) {
			return fmt.Errorf("second cash detail is missing %q", want)
		}
	}
	reversedAllocation, err := client.get(ctx, "/allocations/"+state.AllocationID)
	if err != nil {
		return err
	}
	for _, want := range []string{"reversed", "反向核销事实", "首次核销依据匹配错误", "40000 CNY"} {
		if !strings.Contains(reversedAllocation, want) {
			return fmt.Errorf("reversed allocation detail is missing %q", want)
		}
	}
	replacementAllocation, err := client.get(ctx, "/allocations/"+state.ReplacementAllocationID)
	if err != nil {
		return err
	}
	for _, want := range []string{"active", "银行回单 BF-MONEY-BANK-001 复核后重新核销", "40000 CNY"} {
		if !strings.Contains(replacementAllocation, want) {
			return fmt.Errorf("replacement allocation detail is missing %q", want)
		}
	}
	transfer, err := client.get(ctx, "/internal-transfers/"+state.InternalTransferID)
	if err != nil {
		return err
	}
	for _, want := range []string{"基本户向项目备用金账户转账", "5000 CNY", "基本户尾号 2026", "项目备用金账户", "不参与义务核销", "BF-MONEY-TRANSFER-001"} {
		if !strings.Contains(transfer, want) {
			return fmt.Errorf("internal transfer detail is missing %q", want)
		}
	}
	errorCash, err := client.get(ctx, "/cash-transactions/"+state.ErrorTransactionID)
	if err != nil {
		return err
	}
	for _, want := range []string{"误录测试收款", "reversed", "当前有效金额</dt><dd>0 CNY", "资金冲销事实", "发现重复录入"} {
		if !strings.Contains(errorCash, want) {
			return fmt.Errorf("reversed cash detail is missing %q", want)
		}
	}
	aging, err := client.get(ctx, "/aging?as_of_date=2099-12-31")
	if err != nil {
		return err
	}
	if strings.Contains(aging, state.ObligationID) || !strings.Contains(aging, "当前没有状态为 open 且剩余金额大于 0 的义务") {
		return errors.New("closed zero-balance obligation was not excluded from aging")
	}
	forecast, err := client.get(ctx, "/cash-forecast?history_obligation_id="+state.ObligationID)
	if err != nil {
		return err
	}
	for _, want := range []string{"预计日期历史", "2026-10-20", "客户确认预计分两笔回款", "设置"} {
		if !strings.Contains(forecast, want) {
			return fmt.Errorf("cash forecast history is missing %q", want)
		}
	}
	if err := verifyHistoricalBalance(ctx, client, state.ObligationID, state.InitialAllocationAt,
		"Revision</dt><dd>2", "125000 − 40000 − 0 = 85000 CNY", "历史余额事实序号</dt><dd>1", "open"); err != nil {
		return err
	}
	if err := verifyHistoricalBalance(ctx, client, state.ObligationID, state.AllocationReversedAt,
		"Revision</dt><dd>2", "125000 − 0 − 0 = 125000 CNY", "历史余额事实序号</dt><dd>2", "open"); err != nil {
		return err
	}
	if err := verifyHistoricalBalance(ctx, client, state.ObligationID, state.FullyAllocatedAt,
		"Revision</dt><dd>2", "125000 − 70000 − 0 = 55000 CNY", "历史余额事实序号</dt><dd>4", "open"); err != nil {
		return err
	}
	if err := verifyHistoricalBalance(ctx, client, state.ObligationID, state.ObligationClosedAt,
		"Revision</dt><dd>2", "125000 − 70000 − 55000 = 0 CNY", "closed", "资金分配与减免组合归零"); err != nil {
		return err
	}
	summary, err := client.get(ctx, "/operating-summary")
	if err != nil {
		return err
	}
	for _, want := range []string{"当前经营收支汇总", "青岚设计工作室", "70000"} {
		if !strings.Contains(summary, want) {
			return fmt.Errorf("operating summary is missing %q", want)
		}
	}
	exportPage, err := client.get(ctx, "/ledger-exports")
	if err != nil {
		return err
	}
	form, err := formByAction(exportPage, "/ledger-exports")
	if err != nil {
		return err
	}
	body, headers, err := client.postBytes(ctx, "/ledger-exports", url.Values{
		"csrf_token": {hiddenValue(form, "csrf_token")}, "ledger_owner_party_id": {state.OwnerPartyID},
	}, http.StatusOK)
	if err != nil {
		return err
	}
	if len(body) < 4 || string(body[:4]) != "PK\x03\x04" || headers.Get("Content-Type") != "application/zip" || headers.Get("Cache-Control") != "no-store" {
		return fmt.Errorf("ledger export is not a protected ZIP archive: headers=%v size=%d", headers, len(body))
	}
	return nil
}

func verifyHistoricalBalance(ctx context.Context, client *formClient, obligationID, asOf string, wants ...string) error {
	page, err := client.get(ctx, "/obligations/"+obligationID+"/history?as_of="+url.QueryEscape(asOf))
	if err != nil {
		return fmt.Errorf("read historical balance at %s: %w", asOf, err)
	}
	if !strings.Contains(page, "历史 Obligation 余额") {
		return fmt.Errorf("historical balance at %s did not render a snapshot", asOf)
	}
	for _, want := range wants {
		if !strings.Contains(page, want) {
			return fmt.Errorf("historical balance at %s is missing %q", asOf, want)
		}
	}
	return nil
}

func checkpointNow() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func readReference(ctx context.Context, baseURL, token, path string, expected int) (resourceReference, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
	if err != nil {
		return resourceReference{}, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		return resourceReference{}, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil {
		return resourceReference{}, err
	}
	if response.StatusCode != expected {
		return resourceReference{}, fmt.Errorf("reference read %s status=%d want=%d body=%s", path, response.StatusCode, expected, strings.TrimSpace(string(data)))
	}
	if expected != http.StatusOK {
		return resourceReference{}, nil
	}
	var value resourceReference
	if err := json.Unmarshal(data, &value); err != nil {
		return resourceReference{}, err
	}
	return value, nil
}

func (c *formClient) get(ctx context.Context, path string) (string, error) {
	body, _, err := c.getBytes(ctx, path, http.StatusOK)
	return string(body), err
}

func (c *formClient) getBytes(ctx context.Context, path string, expected int) ([]byte, http.Header, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, nil, err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return nil, nil, err
	}
	if response.StatusCode != expected {
		return nil, response.Header, fmt.Errorf("GET %s returned %d, want %d: %s", path, response.StatusCode, expected, strings.TrimSpace(string(data)))
	}
	return data, response.Header, nil
}

func (c *formClient) postExpect(ctx context.Context, path string, values url.Values, expected int) (string, string, error) {
	body, headers, err := c.postBytes(ctx, path, values, expected)
	if err != nil {
		return "", "", err
	}
	return string(body), headers.Get("Location"), nil
}

func (c *formClient) postBytes(ctx context.Context, path string, values url.Values, expected int) ([]byte, http.Header, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.http.Do(request)
	if err != nil {
		return nil, nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return nil, nil, err
	}
	if response.StatusCode != expected {
		return nil, response.Header, fmt.Errorf("POST %s returned %d, want %d: %s", path, response.StatusCode, expected, strings.TrimSpace(string(data)))
	}
	return data, response.Header, nil
}

func hiddenValue(page, name string) string {
	pattern := regexp.MustCompile(`name="` + regexp.QuoteMeta(name) + `" value="([^"]*)"`)
	match := pattern.FindStringSubmatch(page)
	if len(match) != 2 {
		return ""
	}
	return html.UnescapeString(match[1])
}

func formByAction(page, action string) (string, error) {
	pattern := regexp.MustCompile(`(?s)<form[^>]*action="` + regexp.QuoteMeta(action) + `"[^>]*>(.*?)</form>`)
	match := pattern.FindStringSubmatch(page)
	if len(match) != 2 {
		return "", fmt.Errorf("form action %s was not found", action)
	}
	return match[1], nil
}

func queryID(location, key string) (string, error) {
	parsed, err := url.Parse(location)
	if err != nil {
		return "", err
	}
	value := parsed.Query().Get(key)
	if value == "" {
		return "", fmt.Errorf("redirect %s did not contain %s", location, key)
	}
	return value, nil
}

func pathID(location, prefix string) (string, error) {
	parsed, err := url.Parse(location)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(parsed.Path, prefix) {
		return "", fmt.Errorf("redirect %s did not start with %s", location, prefix)
	}
	value := strings.TrimPrefix(parsed.Path, prefix)
	if value == "" || strings.Contains(value, "/") {
		return "", fmt.Errorf("redirect %s did not contain one resource id", location)
	}
	return value, nil
}

func readState(path string) (demoState, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return demoState{}, nil
	}
	if err != nil {
		return demoState{}, err
	}
	var state demoState
	if err := json.Unmarshal(data, &state); err != nil {
		return demoState{}, fmt.Errorf("decode demo state: %w", err)
	}
	return state, nil
}

func writeState(path string, state demoState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".state-*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(state); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
