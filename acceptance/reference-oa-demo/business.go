package main

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	cmdContractCreate     = "40000000-0000-4000-8000-000000000001"
	cmdContractOwnerLink  = "40000000-0000-4000-8000-000000000002"
	cmdContractClientLink = "40000000-0000-4000-8000-000000000003"
	cmdContractActivate   = "40000000-0000-4000-8000-000000000004"
	cmdContractChange     = "40000000-0000-4000-8000-000000000005"
	cmdContractCorrection = "40000000-0000-4000-8000-000000000006"
	cmdContractTerminate  = "40000000-0000-4000-8000-000000000007"
	cmdContractProgress   = "40000000-0000-4000-8000-000000000008"
	cmdContractFinal      = "40000000-0000-4000-8000-000000000009"
	cmdContractClose      = "40000000-0000-4000-8000-000000000010"
	cmdMoneyOwnerParty    = "50000000-0000-4000-8000-000000000001"
	cmdMoneyClientParty   = "50000000-0000-4000-8000-000000000002"
	cmdObligationCreate   = "50000000-0000-4000-8000-000000000003"
	cmdObligationConfirm  = "50000000-0000-4000-8000-000000000004"
	cmdCashCreate         = "50000000-0000-4000-8000-000000000005"
	cmdAllocationCreate   = "50000000-0000-4000-8000-000000000006"
	cmdObligationAdjust   = "50000000-0000-4000-8000-000000000007"
	cmdAllocationReverse  = "50000000-0000-4000-8000-000000000008"
	cmdReplacementAlloc   = "50000000-0000-4000-8000-000000000009"
	cmdSecondCash         = "50000000-0000-4000-8000-000000000010"
	cmdSecondAllocation   = "50000000-0000-4000-8000-000000000011"
	cmdMoneyRemission     = "50000000-0000-4000-8000-000000000012"
	cmdObligationClose    = "50000000-0000-4000-8000-000000000013"
	cmdErrorCash          = "50000000-0000-4000-8000-000000000014"
	cmdErrorCashReverse   = "50000000-0000-4000-8000-000000000015"
)

type stableReference struct {
	ID           string `json:"id"`
	ResourceType string `json:"resource_type"`
	Status       string `json:"status"`
	Revision     int64  `json:"revision"`
}

type financialRefs struct {
	Contract    stableReference
	Obligation  stableReference
	Transaction stableReference
}

type formClient struct {
	baseURL  string
	username string
	password string
	http     *http.Client
}

func runJ2(ctx context.Context, cfg config, current *state) (financialRefs, error) {
	contractForms, err := newFormClient(cfg.contractURL, cfg.adminUsername, cfg.adminPassword)
	if err != nil {
		return financialRefs{}, err
	}
	if err := contractForms.login(ctx); err != nil {
		return financialRefs{}, fmt.Errorf("Contract Ledger login: %w", err)
	}
	if err := ensureContract(ctx, contractForms, cfg.statePath, current); err != nil {
		return financialRefs{}, err
	}

	moneyForms, err := newFormClient(cfg.moneyURL, cfg.adminUsername, cfg.adminPassword)
	if err != nil {
		return financialRefs{}, err
	}
	if err := moneyForms.login(ctx); err != nil {
		return financialRefs{}, fmt.Errorf("Money Flow Ledger login: %w", err)
	}
	if err := ensureMoneyFlow(ctx, moneyForms, cfg.statePath, current); err != nil {
		return financialRefs{}, err
	}

	references := &client{http: &http.Client{Timeout: 10 * time.Second}, token: cfg.referenceToken}
	var result financialRefs
	if err := references.json(ctx, http.MethodGet, cfg.contractURL+"/api/v1/references/contracts/"+current.ContractID, "", nil, http.StatusOK, &result.Contract); err != nil {
		return financialRefs{}, err
	}
	if err := references.json(ctx, http.MethodGet, cfg.moneyURL+"/api/v1/references/obligations/"+current.ObligationID, "", nil, http.StatusOK, &result.Obligation); err != nil {
		return financialRefs{}, err
	}
	if err := references.json(ctx, http.MethodGet, cfg.moneyURL+"/api/v1/references/transactions/"+current.CashTransactionID, "", nil, http.StatusOK, &result.Transaction); err != nil {
		return financialRefs{}, err
	}
	if result.Contract.Status != "closed" || result.Contract.Revision != 4 || result.Obligation.Status != "closed" || result.Obligation.Revision != 2 || result.Transaction.Status != "recorded" || result.Transaction.Revision != 1 {
		return financialRefs{}, fmt.Errorf("J2 references are not formal: contract=%+v obligation=%+v transaction=%+v", result.Contract, result.Obligation, result.Transaction)
	}
	fmt.Printf("  contract=%s/%s/r%d; obligation=%s/%s/r%d; receipts=%s,%s; reversed-allocation=%s; replacements=%s,%s; erroneous-receipt-reversed=%s\n", result.Contract.ID, result.Contract.Status, result.Contract.Revision, result.Obligation.ID, result.Obligation.Status, result.Obligation.Revision, result.Transaction.ID, current.SecondCashTransactionID, current.AllocationID, current.ReplacementAllocationID, current.SecondAllocationID, current.ErrorCashTransactionID)
	return result, nil
}

func ensureContract(ctx context.Context, forms *formClient, statePath string, current *state) error {
	if current.ContractID == "" {
		page, err := forms.get(ctx, "/contracts")
		if err != nil {
			return err
		}
		location, err := forms.post(ctx, "/contracts", url.Values{
			"csrf_token": {hiddenValue(page, "csrf_token")}, "command_id": {cmdContractCreate},
			"number_namespace": {"oa-demo"}, "contract_number": {"OA-DEMO-2026-001"}, "title": {"青岚办公空间改造设计合同"},
			"category": {"设计服务"}, "currency": {"CNY"}, "total_amount": {"120000.00"}, "signed_on": {"2026-09-25"}, "effective_on": {"2026-09-25"},
		})
		if err != nil {
			return fmt.Errorf("create contract: %w", err)
		}
		current.ContractID, err = queryID(location, "contract_id")
		if err != nil {
			return err
		}
		if err := writeState(statePath, *current); err != nil {
			return err
		}
	}

	partySpecs := []struct {
		created *bool
		name    string
		code    string
	}{
		{&current.ContractOwnerPartyCreated, "青岚设计工作室", "OA-DEMO-OWNER"},
		{&current.ContractClientPartyCreated, "星禾科技有限公司", "OA-DEMO-CLIENT"},
	}
	for _, spec := range partySpecs {
		if *spec.created {
			continue
		}
		page, err := forms.get(ctx, "/parties")
		if err != nil {
			return err
		}
		if !strings.Contains(page, spec.code) {
			if _, err := forms.post(ctx, "/parties", url.Values{"csrf_token": {hiddenValue(page, "csrf_token")}, "kind": {"organization"}, "legal_name": {spec.name}, "registration_identifier": {spec.code}}); err != nil {
				return fmt.Errorf("create Contract party %s: %w", spec.code, err)
			}
		}
		*spec.created = true
		if err := writeState(statePath, *current); err != nil {
			return err
		}
	}

	links := []struct {
		linked *bool
		code   string
		role   string
		cmd    string
	}{
		{&current.ContractOwnerLinked, "OA-DEMO-OWNER", "受托方", cmdContractOwnerLink},
		{&current.ContractClientLinked, "OA-DEMO-CLIENT", "委托方", cmdContractClientLink},
	}
	for _, spec := range links {
		if *spec.linked {
			continue
		}
		path := "/contracts/" + current.ContractID
		page, err := forms.get(ctx, path)
		if err != nil {
			return err
		}
		form, err := formByAction(page, path+"/parties")
		if err != nil {
			if strings.Contains(page, spec.code) {
				*spec.linked = true
				if err := writeState(statePath, *current); err != nil {
					return err
				}
				continue
			}
			return err
		}
		partyID, err := optionValue(form, spec.code)
		if err != nil {
			if strings.Contains(page, spec.code) {
				*spec.linked = true
				if err := writeState(statePath, *current); err != nil {
					return err
				}
				continue
			}
			return err
		}
		if _, err := forms.post(ctx, path+"/parties", url.Values{
			"csrf_token": {hiddenValue(form, "csrf_token")}, "command_id": {spec.cmd}, "expected_draft_version": {hiddenValue(form, "expected_draft_version")}, "party_id": {partyID}, "role": {spec.role},
		}); err != nil {
			return fmt.Errorf("link Contract party %s: %w", spec.code, err)
		}
		*spec.linked = true
		if err := writeState(statePath, *current); err != nil {
			return err
		}
	}

	if !current.ContractActivated {
		path := "/contracts/" + current.ContractID
		page, err := forms.get(ctx, path)
		if err != nil {
			return err
		}
		form, err := formByAction(page, path+"/activate")
		if err != nil {
			if strings.Contains(page, "合同已激活") || strings.Contains(page, "当前 revision") {
				current.ContractActivated = true
				return writeState(statePath, *current)
			}
			return err
		}
		if _, err := forms.post(ctx, path+"/activate", url.Values{
			"csrf_token": {hiddenValue(form, "csrf_token")}, "command_id": {cmdContractActivate}, "expected_draft_version": {hiddenValue(form, "expected_draft_version")},
		}); err != nil {
			return fmt.Errorf("activate contract: %w", err)
		}
		current.ContractActivated = true
		if err := writeState(statePath, *current); err != nil {
			return err
		}
	}
	return ensureContractHistory(ctx, forms, statePath, current)
}

func ensureMoneyFlow(ctx context.Context, forms *formClient, statePath string, current *state) error {
	partySpecs := []struct {
		id   *string
		name string
		code string
		cmd  string
	}{
		{&current.MoneyOwnerPartyID, "青岚设计工作室", "OA-DEMO-OWNER", cmdMoneyOwnerParty},
		{&current.MoneyClientPartyID, "星禾科技有限公司", "OA-DEMO-CLIENT", cmdMoneyClientParty},
	}
	for _, spec := range partySpecs {
		if *spec.id != "" {
			continue
		}
		page, err := forms.get(ctx, "/parties")
		if err != nil {
			return err
		}
		location, err := forms.post(ctx, "/parties", url.Values{
			"csrf_token": {hiddenValue(page, "csrf_token")}, "command_id": {spec.cmd}, "kind": {"organization"}, "legal_name": {spec.name}, "registration_identifier": {spec.code},
		})
		if err != nil {
			return fmt.Errorf("create Money party %s: %w", spec.code, err)
		}
		*spec.id, err = queryID(location, "party_id")
		if err != nil {
			return err
		}
		if err := writeState(statePath, *current); err != nil {
			return err
		}
	}

	if current.ObligationID == "" {
		page, err := forms.get(ctx, "/obligations")
		if err != nil {
			return err
		}
		location, err := forms.post(ctx, "/obligations", url.Values{
			"csrf_token": {hiddenValue(page, "csrf_token")}, "command_id": {cmdObligationCreate}, "direction": {"receivable"},
			"ledger_owner_party_id": {current.MoneyOwnerPartyID}, "counterparty_party_id": {current.MoneyClientPartyID}, "title": {"青岚项目首期设计服务费"},
			"original_amount": {"120000.00"}, "currency": {"CNY"}, "due_on": {"2026-10-31"}, "manual_source_reference": {"责任人确认：Contract " + current.ContractID + " revision 4 的最终结算依据"},
		})
		if err != nil {
			return fmt.Errorf("create obligation: %w", err)
		}
		current.ObligationID, err = pathID(location, "/obligations/")
		if err != nil {
			return err
		}
		if err := writeState(statePath, *current); err != nil {
			return err
		}
	}
	if !current.ObligationConfirmed {
		path := "/obligations/" + current.ObligationID
		page, err := forms.get(ctx, path)
		if err != nil {
			return err
		}
		if _, err := forms.post(ctx, path+"/confirm", url.Values{
			"csrf_token": {hiddenValue(page, "csrf_token")}, "command_id": {cmdObligationConfirm}, "expected_draft_version": {"1"},
		}); err != nil {
			return fmt.Errorf("confirm obligation: %w", err)
		}
		current.ObligationConfirmed = true
		if err := writeState(statePath, *current); err != nil {
			return err
		}
	}
	if err := ensureMoneyAdjustment(ctx, forms, statePath, current); err != nil {
		return err
	}
	if current.CashTransactionID == "" {
		page, err := forms.get(ctx, "/cash-transactions")
		if err != nil {
			return err
		}
		location, err := forms.post(ctx, "/cash-transactions", url.Values{
			"csrf_token": {hiddenValue(page, "csrf_token")}, "command_id": {cmdCashCreate}, "type": {"receipt"},
			"ledger_owner_party_id": {current.MoneyOwnerPartyID}, "counterparty_party_id": {current.MoneyClientPartyID}, "description": {"青岚项目首期收款"},
			"amount": {"40000.00"}, "currency": {"CNY"}, "occurred_on": {"2026-09-25"}, "account_reference": {"演示账户尾号 2026"},
			"manual_source_reference": {"演示银行回单 OA-DEMO-RECEIPT-001"}, "external_transaction_reference": {"OA-DEMO-BANK-001"},
		})
		if err != nil {
			return fmt.Errorf("create cash receipt: %w", err)
		}
		current.CashTransactionID, err = pathID(location, "/cash-transactions/")
		if err != nil {
			return err
		}
		if err := writeState(statePath, *current); err != nil {
			return err
		}
	}
	if current.AllocationID == "" {
		page, err := forms.get(ctx, "/allocations")
		if err != nil {
			return err
		}
		location, err := forms.post(ctx, "/allocations", url.Values{
			"csrf_token": {hiddenValue(page, "csrf_token")}, "command_id": {cmdAllocationCreate}, "cash_transaction_id": {current.CashTransactionID},
			"obligation_selection": {current.ObligationID + ":2"}, "amount": {"40000.00"}, "manual_source_reference": {"首期收款首次核销（随后复核纠正）"},
		})
		if err != nil {
			return fmt.Errorf("allocate cash receipt: %w", err)
		}
		current.AllocationID, err = pathID(location, "/allocations/")
		if err != nil {
			return err
		}
		if err := writeState(statePath, *current); err != nil {
			return err
		}
	}
	return ensureMoneyHistory(ctx, forms, statePath, current)
}

func newFormClient(baseURL, username, password string) (*formClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &formClient{baseURL: strings.TrimRight(baseURL, "/"), username: username, password: password, http: &http.Client{
		Timeout: 10 * time.Second, Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func (c *formClient) login(ctx context.Context) error {
	page, err := c.get(ctx, "/login")
	if err != nil {
		return err
	}
	_, err = c.post(ctx, "/login", url.Values{"csrf_token": {hiddenValue(page, "csrf_token")}, "username": {c.username}, "password": {c.password}, "next": {"/"}})
	return err
}

func (c *formClient) get(ctx context.Context, path string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return "", err
	}
	response, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	data, readErr := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if readErr != nil {
		return "", readErr
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s returned %d: %s", path, response.StatusCode, strings.TrimSpace(string(data)))
	}
	return string(data), nil
}

func (c *formClient) post(ctx context.Context, path string, values url.Values) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, strings.NewReader(values.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		data, _ := io.ReadAll(io.LimitReader(response.Body, 16<<10))
		return "", fmt.Errorf("POST %s returned %d: %s", path, response.StatusCode, strings.TrimSpace(string(data)))
	}
	return response.Header.Get("Location"), nil
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

func optionValue(form, identifier string) (string, error) {
	pattern := regexp.MustCompile(`(?s)<option value="([0-9a-f-]{36})"[^>]*>(.*?)</option>`)
	for _, match := range pattern.FindAllStringSubmatch(form, -1) {
		if strings.Contains(html.UnescapeString(match[2]), identifier) {
			return match[1], nil
		}
	}
	return "", fmt.Errorf("party option %s was not found", identifier)
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
