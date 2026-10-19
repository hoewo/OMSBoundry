package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"mime/multipart"
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
	commandContractCreate     = "61000000-0000-4000-8000-000000000001"
	commandOwnerLink          = "61000000-0000-4000-8000-000000000002"
	commandClientLink         = "61000000-0000-4000-8000-000000000003"
	commandActivate           = "61000000-0000-4000-8000-000000000004"
	commandSupplement         = "61000000-0000-4000-8000-000000000005"
	commandCorrection         = "61000000-0000-4000-8000-000000000006"
	commandTermination        = "61000000-0000-4000-8000-000000000007"
	commandInvalidAfterTerm   = "61000000-0000-4000-8000-000000000008"
	commandRelatedCreate      = "61000000-0000-4000-8000-000000000009"
	commandRelatedOwnerLink   = "61000000-0000-4000-8000-000000000010"
	commandRelatedClientLink  = "61000000-0000-4000-8000-000000000011"
	commandRelatedActivate    = "61000000-0000-4000-8000-000000000012"
	commandRelation           = "61000000-0000-4000-8000-000000000013"
	commandEvidenceCreate     = "61000000-0000-4000-8000-000000000014"
	commandEvidenceVersion    = "61000000-0000-4000-8000-000000000015"
	commandProgressSettlement = "61000000-0000-4000-8000-000000000016"
	commandFinalSettlement    = "61000000-0000-4000-8000-000000000017"
	commandClosure            = "61000000-0000-4000-8000-000000000018"
	commandInvalidAfterClose  = "61000000-0000-4000-8000-000000000019"

	ownerCode  = "BF-DEMO-OWNER"
	clientCode = "BF-DEMO-CLIENT"
)

type config struct {
	baseURL        string
	username       string
	password       string
	referenceToken string
	statePath      string
}

type demoState struct {
	ContractID          string `json:"contract_id"`
	RelatedContractID   string `json:"related_contract_id"`
	EvidenceDocumentID  string `json:"evidence_document_id"`
	OwnerCreated        bool   `json:"owner_created"`
	ClientCreated       bool   `json:"client_created"`
	OwnerLinked         bool   `json:"owner_linked"`
	ClientLinked        bool   `json:"client_linked"`
	Activated           bool   `json:"activated"`
	Changed             bool   `json:"changed"`
	Corrected           bool   `json:"corrected"`
	RelatedOwnerLinked  bool   `json:"related_owner_linked"`
	RelatedClientLinked bool   `json:"related_client_linked"`
	RelatedActivated    bool   `json:"related_activated"`
	RelationLinked      bool   `json:"relation_linked"`
	EvidenceV1          bool   `json:"evidence_v1"`
	EvidenceV2          bool   `json:"evidence_v2"`
	Terminated          bool   `json:"terminated"`
	ProgressSettled     bool   `json:"progress_settled"`
	FinalSettled        bool   `json:"final_settled"`
	Closed              bool   `json:"closed"`
}

type contractReference struct {
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
		fmt.Fprintln(os.Stderr, "usage: contract-ledger-demo")
		os.Exit(2)
	}
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "Contract Ledger demo failed:", err)
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

	fmt.Println("[1/12] 验证本地模板身份的失败关闭")
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

	fmt.Println("[2/12] 登记两方主体并创建合同草稿")
	if err := ensureParty(ctx, client, cfg.statePath, &state, &state.OwnerCreated, ownerCode, "青岚设计工作室"); err != nil {
		return err
	}
	if err := ensureParty(ctx, client, cfg.statePath, &state, &state.ClientCreated, clientCode, "星禾科技有限公司"); err != nil {
		return err
	}
	if err := ensureContract(ctx, client, cfg.statePath, &state); err != nil {
		return err
	}

	fmt.Println("[3/12] 关联合同方并激活不可变 revision 1")
	if err := ensurePartyLink(ctx, client, cfg.statePath, &state, &state.OwnerLinked, ownerCode, "受托方", commandOwnerLink); err != nil {
		return err
	}
	if err := ensurePartyLink(ctx, client, cfg.statePath, &state, &state.ClientLinked, clientCode, "委托方", commandClientLink); err != nil {
		return err
	}
	if err := ensureActivation(ctx, client, cfg.statePath, &state); err != nil {
		return err
	}

	fmt.Println("[4/12] 登记补充协议并精确调整金额")
	if err := ensureSupplement(ctx, client, cfg.statePath, &state); err != nil {
		return err
	}

	fmt.Println("[5/12] 通过受控更正形成新 revision")
	if err := ensureCorrection(ctx, client, cfg.statePath, &state); err != nil {
		return err
	}

	fmt.Println("[6/12] 建立第二份正式合同和不可变合同关系")
	if err := ensureRelatedContract(ctx, client, cfg.statePath, &state); err != nil {
		return err
	}
	if err := ensureRelation(ctx, client, cfg.statePath, &state); err != nil {
		return err
	}

	fmt.Println("[7/12] 上传当前遗留本地 Evidence 的两个不可变版本")
	if err := ensureEvidenceVersions(ctx, client, cfg.statePath, &state); err != nil {
		return err
	}

	fmt.Println("[8/12] 登记终止文书并拒绝终止后的普通变更")
	if err := ensureTermination(ctx, client, cfg.statePath, &state); err != nil {
		return err
	}
	if err := verifyInvalidChangeAfterTermination(ctx, client, state.ContractID); err != nil {
		return err
	}

	fmt.Println("[9/12] 登记阶段与最终结算，并重放最终结算命令")
	if err := ensureSettlements(ctx, client, cfg.statePath, &state); err != nil {
		return err
	}

	fmt.Println("[10/12] 操作性关闭合同并拒绝关闭后的新结算")
	if err := ensureClosure(ctx, client, cfg.statePath, &state); err != nil {
		return err
	}
	if err := verifyInvalidSettlementAfterClosure(ctx, client, state.ContractID); err != nil {
		return err
	}

	fmt.Println("[11/12] 重放补充协议命令并核对关系、结算、文件、查询、导出和审计")
	if err := replaySupplement(ctx, client, state.ContractID); err != nil {
		return err
	}
	if err := verifyBusinessViews(ctx, client, state.ContractID); err != nil {
		return err
	}

	fmt.Println("[12/12] 核对版本化 reference-read 及错误凭证")
	reference, err := readReference(ctx, cfg.baseURL, cfg.referenceToken, state.ContractID, http.StatusOK)
	if err != nil {
		return err
	}
	if reference.ID != state.ContractID || reference.ResourceType != "contract" || reference.Status != "closed" || reference.Revision != 4 || reference.TenantID == "" {
		return fmt.Errorf("unexpected contract reference: %+v", reference)
	}
	if _, err := readReference(ctx, cfg.baseURL, "wrong-reference-token", state.ContractID, http.StatusUnauthorized); err != nil {
		return err
	}

	fmt.Printf("PASS: Contract Ledger 独立 Demo contract=%s status=%s revision=%d\n", reference.ID, reference.Status, reference.Revision)
	fmt.Println("      关系、Evidence v1/v2、阶段/最终结算、关闭和审计均已核对；关闭后写入失败，命令重放未新增事实。")
	fmt.Println("      本地 Evidence 是待迁移遗留能力；目标架构仍由独立 Evidence Service 拥有文件技术事实。")
	fmt.Println("      浏览器入口：" + cfg.baseURL + "/contracts")
	return nil
}

func loadConfig() (config, error) {
	cfg := config{
		baseURL:        envOr("DEMO_CONTRACT_URL", "http://127.0.0.1:28081"),
		username:       strings.TrimSpace(os.Getenv("DEMO_ADMIN_USERNAME")),
		password:       os.Getenv("DEMO_ADMIN_PASSWORD"),
		referenceToken: os.Getenv("DEMO_REFERENCE_ACCESS_TOKEN"),
		statePath:      envOr("DEMO_STATE_PATH", "deployments/demos/contract-ledger/.runtime/state.json"),
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
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/contracts", nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("anonymous GET /contracts: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusSeeOther || !strings.HasPrefix(response.Header.Get("Location"), "/login?next=") {
		return fmt.Errorf("anonymous GET /contracts status=%d location=%q", response.StatusCode, response.Header.Get("Location"))
	}
	return nil
}

func newFormClient(baseURL string) (*formClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &formClient{baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{
		Timeout: 10 * time.Second,
		Jar:     jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
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
		"csrf_token": {hiddenValue(form, "csrf_token")},
		"username":   {username},
		"password":   {"definitely-wrong-demo-password"},
		"next":       {"/"},
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
		"csrf_token": {hiddenValue(form, "csrf_token")},
		"username":   {username},
		"password":   {password},
		"next":       {"/"},
	}, http.StatusSeeOther)
	return err
}

func ensureParty(ctx context.Context, client *formClient, statePath string, state *demoState, done *bool, code, name string) error {
	if *done {
		return nil
	}
	page, err := client.get(ctx, "/parties")
	if err != nil {
		return err
	}
	if !strings.Contains(page, code) {
		form, err := formByAction(page, "/parties")
		if err != nil {
			return err
		}
		if _, _, err := client.postExpect(ctx, "/parties", url.Values{
			"csrf_token":              {hiddenValue(form, "csrf_token")},
			"kind":                    {"organization"},
			"legal_name":              {name},
			"registration_identifier": {code},
		}, http.StatusSeeOther); err != nil {
			return fmt.Errorf("create party %s: %w", code, err)
		}
	}
	*done = true
	return writeState(statePath, *state)
}

func ensureContract(ctx context.Context, client *formClient, statePath string, state *demoState) error {
	if state.ContractID != "" {
		return nil
	}
	page, err := client.get(ctx, "/contracts")
	if err != nil {
		return err
	}
	form, err := formByAction(page, "/contracts")
	if err != nil {
		return err
	}
	_, location, err := client.postExpect(ctx, "/contracts", url.Values{
		"csrf_token":       {hiddenValue(form, "csrf_token")},
		"command_id":       {commandContractCreate},
		"number_namespace": {"demo"},
		"contract_number":  {"BF-DEMO-2026-001"},
		"title":            {"青岚办公空间改造设计合同"},
		"category":         {"设计服务"},
		"currency":         {"CNY"},
		"total_amount":     {"120000.00"},
		"signed_on":        {"2026-09-25"},
		"effective_on":     {"2026-09-25"},
	}, http.StatusSeeOther)
	if err != nil {
		return fmt.Errorf("create contract: %w", err)
	}
	state.ContractID, err = queryID(location, "contract_id")
	if err != nil {
		return err
	}
	return writeState(statePath, *state)
}

func ensurePartyLink(ctx context.Context, client *formClient, statePath string, state *demoState, done *bool, code, role, commandID string) error {
	if *done {
		return nil
	}
	path := "/contracts/" + state.ContractID
	page, err := client.get(ctx, path)
	if err != nil {
		return err
	}
	form, err := formByAction(page, path+"/parties")
	if err != nil {
		if strings.Contains(page, code) && strings.Contains(page, role) {
			*done = true
			return writeState(statePath, *state)
		}
		return err
	}
	partyID, err := optionValue(form, code)
	if err != nil {
		return err
	}
	if _, _, err := client.postExpect(ctx, path+"/parties", url.Values{
		"csrf_token":             {hiddenValue(form, "csrf_token")},
		"command_id":             {commandID},
		"expected_draft_version": {hiddenValue(form, "expected_draft_version")},
		"party_id":               {partyID},
		"role":                   {role},
	}, http.StatusSeeOther); err != nil {
		return fmt.Errorf("link contract party %s: %w", code, err)
	}
	*done = true
	return writeState(statePath, *state)
}

func ensureActivation(ctx context.Context, client *formClient, statePath string, state *demoState) error {
	if state.Activated {
		return nil
	}
	path := "/contracts/" + state.ContractID
	page, err := client.get(ctx, path)
	if err != nil {
		return err
	}
	form, err := formByAction(page, path+"/activate")
	if err != nil {
		if strings.Contains(page, "当前 revision") {
			state.Activated = true
			return writeState(statePath, *state)
		}
		return err
	}
	if _, _, err := client.postExpect(ctx, path+"/activate", url.Values{
		"csrf_token":             {hiddenValue(form, "csrf_token")},
		"command_id":             {commandActivate},
		"expected_draft_version": {hiddenValue(form, "expected_draft_version")},
	}, http.StatusSeeOther); err != nil {
		return fmt.Errorf("activate contract: %w", err)
	}
	state.Activated = true
	return writeState(statePath, *state)
}

func ensureRelatedContract(ctx context.Context, client *formClient, statePath string, state *demoState) error {
	if state.RelatedContractID == "" {
		page, err := client.get(ctx, "/contracts")
		if err != nil {
			return err
		}
		form, err := formByAction(page, "/contracts")
		if err != nil {
			return err
		}
		_, location, err := client.postExpect(ctx, "/contracts", url.Values{
			"csrf_token":       {hiddenValue(form, "csrf_token")},
			"command_id":       {commandRelatedCreate},
			"number_namespace": {"demo"},
			"contract_number":  {"BF-DEMO-2026-002"},
			"title":            {"青岚办公空间家具采购合同"},
			"category":         {"采购"},
			"currency":         {"CNY"},
			"total_amount":     {"30000.00"},
			"signed_on":        {"2026-09-26"},
			"effective_on":     {"2026-09-26"},
		}, http.StatusSeeOther)
		if err != nil {
			return fmt.Errorf("create related contract: %w", err)
		}
		state.RelatedContractID, err = queryID(location, "contract_id")
		if err != nil {
			return err
		}
		if err := writeState(statePath, *state); err != nil {
			return err
		}
	}
	if err := ensurePartyLinkForContract(ctx, client, statePath, state, state.RelatedContractID, &state.RelatedOwnerLinked, ownerCode, "受托方", commandRelatedOwnerLink); err != nil {
		return err
	}
	if err := ensurePartyLinkForContract(ctx, client, statePath, state, state.RelatedContractID, &state.RelatedClientLinked, clientCode, "委托方", commandRelatedClientLink); err != nil {
		return err
	}
	return ensureActivationForContract(ctx, client, statePath, state, state.RelatedContractID, &state.RelatedActivated, commandRelatedActivate)
}

func ensurePartyLinkForContract(ctx context.Context, client *formClient, statePath string, state *demoState, contractID string, done *bool, code, role, commandID string) error {
	if *done {
		return nil
	}
	path := "/contracts/" + contractID
	page, err := client.get(ctx, path)
	if err != nil {
		return err
	}
	form, err := formByAction(page, path+"/parties")
	if err != nil {
		if strings.Contains(page, code) && strings.Contains(page, role) {
			*done = true
			return writeState(statePath, *state)
		}
		return err
	}
	partyID, err := optionValue(form, code)
	if err != nil {
		return err
	}
	if _, _, err := client.postExpect(ctx, path+"/parties", url.Values{
		"csrf_token":             {hiddenValue(form, "csrf_token")},
		"command_id":             {commandID},
		"expected_draft_version": {hiddenValue(form, "expected_draft_version")},
		"party_id":               {partyID},
		"role":                   {role},
	}, http.StatusSeeOther); err != nil {
		return fmt.Errorf("link related contract party %s: %w", code, err)
	}
	*done = true
	return writeState(statePath, *state)
}

func ensureActivationForContract(ctx context.Context, client *formClient, statePath string, state *demoState, contractID string, done *bool, commandID string) error {
	if *done {
		return nil
	}
	path := "/contracts/" + contractID
	page, err := client.get(ctx, path)
	if err != nil {
		return err
	}
	form, err := formByAction(page, path+"/activate")
	if err != nil {
		if strings.Contains(page, "当前 revision") {
			*done = true
			return writeState(statePath, *state)
		}
		return err
	}
	if _, _, err := client.postExpect(ctx, path+"/activate", url.Values{
		"csrf_token":             {hiddenValue(form, "csrf_token")},
		"command_id":             {commandID},
		"expected_draft_version": {hiddenValue(form, "expected_draft_version")},
	}, http.StatusSeeOther); err != nil {
		return fmt.Errorf("activate related contract: %w", err)
	}
	*done = true
	return writeState(statePath, *state)
}

func ensureRelation(ctx context.Context, client *formClient, statePath string, state *demoState) error {
	if state.RelationLinked {
		return nil
	}
	path := "/contracts/" + state.ContractID + "/relations/new?target_id=" + url.QueryEscape(state.RelatedContractID)
	page, err := client.get(ctx, path)
	if err != nil {
		return err
	}
	action := "/contracts/" + state.ContractID + "/relations"
	form, err := formByAction(page, action)
	if err != nil {
		return err
	}
	if _, _, err := client.postExpect(ctx, action, url.Values{
		"csrf_token":               {hiddenValue(form, "csrf_token")},
		"command_id":               {commandRelation},
		"target_contract_id":       {hiddenValue(form, "target_contract_id")},
		"expected_source_revision": {hiddenValue(form, "expected_source_revision")},
		"expected_target_revision": {hiddenValue(form, "expected_target_revision")},
		"relation_type":            {"references"},
		"effective_on":             {"2026-10-04"},
		"note":                     {"主设计合同引用家具采购合同作为交付边界依据"},
	}, http.StatusSeeOther); err != nil {
		return fmt.Errorf("link contract relation: %w", err)
	}
	state.RelationLinked = true
	return writeState(statePath, *state)
}

func ensureEvidenceVersions(ctx context.Context, client *formClient, statePath string, state *demoState) error {
	path := "/contracts/" + state.ContractID
	versionOne := []byte("BizFoundry contract evidence version 1\n")
	versionTwo := []byte("BizFoundry contract evidence version 2 - confirmed\n")
	if !state.EvidenceV1 {
		page, err := client.get(ctx, path)
		if err != nil {
			return err
		}
		form, err := formByAction(page, path+"/evidence")
		if err != nil {
			return err
		}
		if _, _, err := client.postMultipartExpect(ctx, path+"/evidence", map[string]string{
			"csrf_token":       hiddenValue(form, "csrf_token"),
			"command_id":       commandEvidenceCreate,
			"evidence_target":  "contract:" + state.ContractID,
			"evidence_title":   "合同范围确认文件",
			"evidence_purpose": "证明当前遗留附件的不可变版本和摘要；该存储后续迁移至独立 Evidence Service",
		}, "scope-confirmation-v1.txt", versionOne, http.StatusSeeOther); err != nil {
			return fmt.Errorf("create contract evidence: %w", err)
		}
		page, err = client.get(ctx, path)
		if err != nil {
			return err
		}
		state.EvidenceDocumentID, err = evidenceDocumentID(page, state.ContractID)
		if err != nil {
			return err
		}
		if err := client.verifyLatestEvidenceDownload(ctx, page, state.ContractID, state.EvidenceDocumentID, versionOne); err != nil {
			return err
		}
		state.EvidenceV1 = true
		if err := writeState(statePath, *state); err != nil {
			return err
		}
	}
	if state.EvidenceDocumentID == "" {
		page, err := client.get(ctx, path)
		if err != nil {
			return err
		}
		state.EvidenceDocumentID, err = evidenceDocumentID(page, state.ContractID)
		if err != nil {
			return err
		}
		if err := writeState(statePath, *state); err != nil {
			return err
		}
	}
	if state.EvidenceV2 {
		return nil
	}
	page, err := client.get(ctx, path)
	if err != nil {
		return err
	}
	action := path + "/evidence/" + state.EvidenceDocumentID + "/versions"
	form, err := formByAction(page, action)
	if err != nil {
		return err
	}
	fields := map[string]string{
		"csrf_token":              hiddenValue(form, "csrf_token"),
		"command_id":              commandEvidenceVersion,
		"expected_latest_version": hiddenValue(form, "expected_latest_version"),
	}
	if _, _, err := client.postMultipartExpect(ctx, action, fields, "scope-confirmation-v2.txt", versionTwo, http.StatusSeeOther); err != nil {
		return fmt.Errorf("append contract evidence version: %w", err)
	}
	if _, _, err := client.postMultipartExpect(ctx, action, fields, "scope-confirmation-v2.txt", versionTwo, http.StatusSeeOther); err != nil {
		return fmt.Errorf("replay contract evidence version: %w", err)
	}
	page, err = client.get(ctx, path)
	if err != nil {
		return err
	}
	if err := client.verifyLatestEvidenceDownload(ctx, page, state.ContractID, state.EvidenceDocumentID, versionTwo); err != nil {
		return err
	}
	if strings.Count(page, "scope-confirmation-v2.txt") != 1 {
		return errors.New("evidence version replay created an unexpected duplicate")
	}
	state.EvidenceV2 = true
	return writeState(statePath, *state)
}

func supplementValues(csrf string) url.Values {
	return url.Values{
		"csrf_token":              {csrf},
		"command_id":              {commandSupplement},
		"expected_revision":       {"1"},
		"effective_order":         {"2"},
		"instrument_number":       {"BF-DEMO-SUP-001"},
		"instrument_title":        {"扩展空间设计补充协议"},
		"change_summary":          {"增加共享会议区设计范围并调整合同金额"},
		"instrument_signed_on":    {"2026-10-01"},
		"instrument_effective_on": {"2026-10-02"},
		"amount_effect":           {"adjustment"},
		"amount_value":            {"5000.00"},
	}
}

func ensureSupplement(ctx context.Context, client *formClient, statePath string, state *demoState) error {
	if state.Changed {
		return nil
	}
	path := "/contracts/" + state.ContractID
	page, err := client.get(ctx, path)
	if err != nil {
		return err
	}
	if strings.Contains(page, "BF-DEMO-SUP-001") {
		state.Changed = true
		return writeState(statePath, *state)
	}
	form, err := formByAction(page, path+"/changes")
	if err != nil {
		return err
	}
	values := supplementValues(hiddenValue(form, "csrf_token"))
	values.Set("expected_revision", hiddenValue(form, "expected_revision"))
	values.Set("effective_order", hiddenValue(form, "effective_order"))
	if _, _, err := client.postExpect(ctx, path+"/changes", values, http.StatusSeeOther); err != nil {
		return fmt.Errorf("append contract supplement: %w", err)
	}
	state.Changed = true
	return writeState(statePath, *state)
}

func ensureCorrection(ctx context.Context, client *formClient, statePath string, state *demoState) error {
	if state.Corrected {
		return nil
	}
	path := "/contracts/" + state.ContractID
	page, err := client.get(ctx, path)
	if err != nil {
		return err
	}
	if strings.Contains(page, "BF-DEMO-COR-001") {
		state.Corrected = true
		return writeState(statePath, *state)
	}
	form, err := formByAction(page, path+"/corrections")
	if err != nil {
		return err
	}
	if _, _, err := client.postExpect(ctx, path+"/corrections", url.Values{
		"csrf_token":                         {hiddenValue(form, "csrf_token")},
		"command_id":                         {commandCorrection},
		"expected_status":                    {hiddenValue(form, "expected_status")},
		"expected_revision":                  {hiddenValue(form, "expected_revision")},
		"effective_order":                    {hiddenValue(form, "effective_order")},
		"correction_instrument_number":       {"BF-DEMO-COR-001"},
		"correction_instrument_title":        {"合同录入更正确认单"},
		"correction_reason":                  {"核对签署原件后补充标题说明，金额与商业条款不变"},
		"correction_instrument_signed_on":    {"2026-10-03"},
		"correction_instrument_effective_on": {"2026-10-03"},
		"corrected_title":                    {"青岚办公空间改造设计合同（录入更正）"},
		"corrected_category":                 {"设计服务"},
		"corrected_total_amount":             {"125000.00"},
		"corrected_signed_on":                {"2026-09-25"},
		"corrected_effective_on":             {"2026-09-25"},
	}, http.StatusSeeOther); err != nil {
		return fmt.Errorf("correct contract: %w", err)
	}
	state.Corrected = true
	return writeState(statePath, *state)
}

func ensureTermination(ctx context.Context, client *formClient, statePath string, state *demoState) error {
	if state.Terminated {
		return nil
	}
	path := "/contracts/" + state.ContractID
	page, err := client.get(ctx, path)
	if err != nil {
		return err
	}
	if strings.Contains(page, "BF-DEMO-TERM-001") && strings.Contains(page, "已终止") {
		state.Terminated = true
		return writeState(statePath, *state)
	}
	form, err := formByAction(page, path+"/terminate")
	if err != nil {
		return err
	}
	if _, _, err := client.postExpect(ctx, path+"/terminate", url.Values{
		"csrf_token":                    {hiddenValue(form, "csrf_token")},
		"command_id":                    {commandTermination},
		"expected_revision":             {hiddenValue(form, "expected_revision")},
		"effective_order":               {hiddenValue(form, "effective_order")},
		"termination_instrument_number": {"BF-DEMO-TERM-001"},
		"termination_instrument_title":  {"合同终止协议"},
		"termination_reason":            {"双方完成阶段性交付并协商终止后续服务"},
		"termination_signed_on":         {"2026-10-10"},
		"termination_effective_on":      {"2026-10-15"},
	}, http.StatusSeeOther); err != nil {
		return fmt.Errorf("terminate contract: %w", err)
	}
	state.Terminated = true
	return writeState(statePath, *state)
}

func ensureSettlements(ctx context.Context, client *formClient, statePath string, state *demoState) error {
	path := "/contracts/" + state.ContractID
	if !state.ProgressSettled {
		page, err := client.get(ctx, path)
		if err != nil {
			return err
		}
		form, err := formByAction(page, path+"/settlements")
		if err != nil {
			return err
		}
		values, err := settlementValues(form, commandProgressSettlement, "progress", "BF-DEMO-SETTLE-001", "第一阶段累计结算", "100000.00", "2026-10-20", "双方按阶段成果和终止范围确认累计结算金额")
		if err != nil {
			return err
		}
		if _, _, err := client.postExpect(ctx, path+"/settlements", values, http.StatusSeeOther); err != nil {
			return fmt.Errorf("record progress settlement: %w", err)
		}
		state.ProgressSettled = true
		if err := writeState(statePath, *state); err != nil {
			return err
		}
	}
	if state.FinalSettled {
		return nil
	}
	page, err := client.get(ctx, path)
	if err != nil {
		return err
	}
	form, err := formByAction(page, path+"/settlements")
	if err != nil {
		return err
	}
	values, err := settlementValues(form, commandFinalSettlement, "final", "BF-DEMO-SETTLE-002", "最终累计结算", "118000.00", "2026-10-30", "双方根据最终交付和终止协议确认最终累计结算金额")
	if err != nil {
		return err
	}
	if _, _, err := client.postExpect(ctx, path+"/settlements", values, http.StatusSeeOther); err != nil {
		return fmt.Errorf("record final settlement: %w", err)
	}
	if _, _, err := client.postExpect(ctx, path+"/settlements", values, http.StatusSeeOther); err != nil {
		return fmt.Errorf("replay final settlement: %w", err)
	}
	page, err = client.get(ctx, path)
	if err != nil {
		return err
	}
	if !strings.Contains(page, "BF-DEMO-SETTLE-002") || !strings.Contains(page, "2 / 最终结算") || !strings.Contains(page, "118000") {
		return errors.New("final settlement replay changed the settlement timeline")
	}
	if _, err := formByAction(page, path+"/settlements"); err == nil {
		return errors.New("finalized settlement timeline still exposed a new-settlement form")
	}
	state.FinalSettled = true
	return writeState(statePath, *state)
}

func settlementValues(form, commandID, kind, number, title, amount, confirmedOn, basis string) (url.Values, error) {
	positions := checkedValues(form, "confirming_party_position")
	if len(positions) == 0 {
		return nil, errors.New("settlement form did not expose confirming party positions")
	}
	values := url.Values{
		"csrf_token":                   {hiddenValue(form, "csrf_token")},
		"command_id":                   {commandID},
		"expected_revision":            {hiddenValue(form, "expected_revision")},
		"expected_settlement_sequence": {hiddenValue(form, "expected_settlement_sequence")},
		"settlement_kind":              {kind},
		"settlement_number":            {number},
		"settlement_title":             {title},
		"settlement_amount":            {amount},
		"settlement_confirmed_on":      {confirmedOn},
		"settlement_basis":             {basis},
	}
	for _, position := range positions {
		values.Add("confirming_party_position", position)
	}
	return values, nil
}

func ensureClosure(ctx context.Context, client *formClient, statePath string, state *demoState) error {
	if state.Closed {
		return nil
	}
	path := "/contracts/" + state.ContractID
	page, err := client.get(ctx, path)
	if err != nil {
		return err
	}
	form, err := formByAction(page, path+"/close")
	if err != nil {
		return err
	}
	values := url.Values{
		"csrf_token":                   {hiddenValue(form, "csrf_token")},
		"command_id":                   {commandClosure},
		"expected_revision":            {hiddenValue(form, "expected_revision")},
		"expected_settlement_sequence": {hiddenValue(form, "expected_settlement_sequence")},
		"settlement_disposition":       {hiddenValue(form, "settlement_disposition")},
		"closed_on":                    {"2026-10-31"},
		"closure_reason":               {"终止范围、最终结算和遗留资料均已确认，结束常规合同维护"},
	}
	if _, _, err := client.postExpect(ctx, path+"/close", values, http.StatusSeeOther); err != nil {
		return fmt.Errorf("close contract: %w", err)
	}
	if _, _, err := client.postExpect(ctx, path+"/close", values, http.StatusSeeOther); err != nil {
		return fmt.Errorf("replay contract closure: %w", err)
	}
	page, err = client.get(ctx, path)
	if err != nil {
		return err
	}
	if !strings.Contains(page, "已关闭") || !strings.Contains(page, "已登记最终结算") {
		return errors.New("closed contract did not expose the closure and settlement disposition")
	}
	state.Closed = true
	return writeState(statePath, *state)
}

func verifyInvalidSettlementAfterClosure(ctx context.Context, client *formClient, contractID string) error {
	path := "/contracts/" + contractID
	page, err := client.get(ctx, path)
	if err != nil {
		return err
	}
	values := url.Values{
		"csrf_token":                   {hiddenValue(page, "csrf_token")},
		"command_id":                   {commandInvalidAfterClose},
		"expected_revision":            {"4"},
		"expected_settlement_sequence": {"2"},
		"settlement_kind":              {"progress"},
		"settlement_number":            {"BF-DEMO-INVALID-AFTER-CLOSE"},
		"settlement_title":             {"关闭后不应接受的结算"},
		"settlement_amount":            {"118000.00"},
		"settlement_confirmed_on":      {"2026-11-01"},
		"settlement_basis":             {"验证关闭终态保护"},
		"confirming_party_position":    {"1", "2"},
	}
	body, _, err := client.postExpect(ctx, path+"/settlements", values, http.StatusConflict)
	if err != nil {
		return err
	}
	if !strings.Contains(body, "只有已激活或已终止合同可以登记结算") {
		return errors.New("closed contract did not return the expected settlement state conflict")
	}
	return nil
}

func verifyInvalidChangeAfterTermination(ctx context.Context, client *formClient, contractID string) error {
	path := "/contracts/" + contractID
	page, err := client.get(ctx, path)
	if err != nil {
		return err
	}
	values := supplementValues(hiddenValue(page, "csrf_token"))
	values.Set("command_id", commandInvalidAfterTerm)
	values.Set("expected_revision", "4")
	values.Set("effective_order", "5")
	values.Set("instrument_number", "BF-DEMO-INVALID-AFTER-TERM")
	body, _, err := client.postExpect(ctx, path+"/changes", values, http.StatusConflict)
	if err != nil {
		return err
	}
	if !strings.Contains(body, "只有已激活合同") {
		return errors.New("terminated contract did not return the expected state-conflict message")
	}
	return nil
}

func replaySupplement(ctx context.Context, client *formClient, contractID string) error {
	path := "/contracts/" + contractID
	page, err := client.get(ctx, path)
	if err != nil {
		return err
	}
	if _, _, err := client.postExpect(ctx, path+"/changes", supplementValues(hiddenValue(page, "csrf_token")), http.StatusSeeOther); err != nil {
		return fmt.Errorf("replay supplement command: %w", err)
	}
	page, err = client.get(ctx, path)
	if err != nil {
		return err
	}
	if !strings.Contains(page, "BF-DEMO-SUP-001") || !strings.Contains(page, "当前 revision</dt><dd>4</dd>") {
		return errors.New("supplement replay changed history or advanced the contract revision")
	}
	return nil
}

func verifyBusinessViews(ctx context.Context, client *formClient, contractID string) error {
	detailPath := "/contracts/" + contractID
	detail, err := client.get(ctx, detailPath)
	if err != nil {
		return err
	}
	for _, want := range []string{
		"青岚办公空间改造设计合同（录入更正）",
		"BF-DEMO-SUP-001",
		"BF-DEMO-COR-001",
		"BF-DEMO-TERM-001",
		"BF-DEMO-2026-002",
		"主设计合同引用家具采购合同作为交付边界依据",
		"scope-confirmation-v1.txt",
		"scope-confirmation-v2.txt",
		"BF-DEMO-SETTLE-001",
		"BF-DEMO-SETTLE-002",
		"118000",
		"revision 4",
		"已关闭",
	} {
		if !strings.Contains(detail, want) {
			return fmt.Errorf("contract detail is missing %q", want)
		}
	}
	search, err := client.get(ctx, "/contracts?q=BF-DEMO-2026-001&status=closed&currency=CNY")
	if err != nil {
		return err
	}
	if !strings.Contains(search, "BF-DEMO-2026-001") || !strings.Contains(search, contractID) {
		return errors.New("contract search did not return the terminated demo contract")
	}
	auditPage, err := client.get(ctx, "/audit")
	if err != nil {
		return err
	}
	for _, event := range []string{
		"contract_activated",
		"contract_change_activated",
		"contract_correction_recorded",
		"contract_relation_linked",
		"contract_evidence_created",
		"contract_evidence_version_added",
		"contract_terminated",
		"contract_settlement_recorded",
		"contract_closed",
	} {
		if !strings.Contains(auditPage, event) {
			return fmt.Errorf("audit page is missing %s", event)
		}
	}
	body, headers, err := client.getBytes(ctx, detailPath+"/export", http.StatusOK)
	if err != nil {
		return err
	}
	if len(body) < 4 || string(body[:4]) != "PK\x03\x04" || headers.Get("Content-Type") != "application/zip" || headers.Get("Cache-Control") != "no-store" {
		return fmt.Errorf("ledger export is not a protected ZIP archive: headers=%v size=%d", headers, len(body))
	}
	return nil
}

func readReference(ctx context.Context, baseURL, token, contractID string, expected int) (contractReference, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/v1/references/contracts/"+contractID, nil)
	if err != nil {
		return contractReference{}, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		return contractReference{}, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil {
		return contractReference{}, err
	}
	if response.StatusCode != expected {
		return contractReference{}, fmt.Errorf("reference read status=%d want=%d body=%s", response.StatusCode, expected, strings.TrimSpace(string(data)))
	}
	if expected != http.StatusOK {
		return contractReference{}, nil
	}
	var value contractReference
	if err := json.Unmarshal(data, &value); err != nil {
		return contractReference{}, err
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
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, strings.NewReader(values.Encode()))
	if err != nil {
		return "", "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.http.Do(request)
	if err != nil {
		return "", "", err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return "", "", err
	}
	if response.StatusCode != expected {
		return "", response.Header.Get("Location"), fmt.Errorf("POST %s returned %d, want %d: %s", path, response.StatusCode, expected, strings.TrimSpace(string(data)))
	}
	return string(data), response.Header.Get("Location"), nil
}

func (c *formClient) postMultipartExpect(ctx context.Context, path string, fields map[string]string, filename string, content []byte, expected int) (string, string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			return "", "", err
		}
	}
	part, err := writer.CreateFormFile("evidence_file", filename)
	if err != nil {
		return "", "", err
	}
	if _, err := part.Write(content); err != nil {
		return "", "", err
	}
	if err := writer.Close(); err != nil {
		return "", "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, &body)
	if err != nil {
		return "", "", err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := c.http.Do(request)
	if err != nil {
		return "", "", err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return "", "", err
	}
	if response.StatusCode != expected {
		return "", response.Header.Get("Location"), fmt.Errorf("multipart POST %s returned %d, want %d: %s", path, response.StatusCode, expected, strings.TrimSpace(string(data)))
	}
	return string(data), response.Header.Get("Location"), nil
}

func (c *formClient) verifyLatestEvidenceDownload(ctx context.Context, page, contractID, documentID string, expected []byte) error {
	pattern := regexp.MustCompile(`/contracts/` + regexp.QuoteMeta(contractID) + `/evidence/` + regexp.QuoteMeta(documentID) + `/versions/[0-9a-f-]{36}/download`)
	matches := pattern.FindAllString(page, -1)
	if len(matches) == 0 {
		return errors.New("evidence download link was not found")
	}
	body, headers, err := c.getBytes(ctx, matches[len(matches)-1], http.StatusOK)
	if err != nil {
		return err
	}
	if !bytes.Equal(body, expected) {
		return fmt.Errorf("evidence download bytes differ: got=%q want=%q", string(body), string(expected))
	}
	if !strings.Contains(headers.Get("Cache-Control"), "no-store") || !strings.Contains(headers.Get("Content-Disposition"), "attachment") {
		return fmt.Errorf("evidence download headers are unsafe: %v", headers)
	}
	return nil
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

func checkedValues(form, name string) []string {
	pattern := regexp.MustCompile(`name="` + regexp.QuoteMeta(name) + `" value="([^"]+)"`)
	matches := pattern.FindAllStringSubmatch(form, -1)
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		values = append(values, html.UnescapeString(match[1]))
	}
	return values
}

func evidenceDocumentID(page, contractID string) (string, error) {
	pattern := regexp.MustCompile(`/contracts/` + regexp.QuoteMeta(contractID) + `/evidence/([0-9a-f-]{36})/versions`)
	match := pattern.FindStringSubmatch(page)
	if len(match) != 2 {
		return "", errors.New("evidence document version form was not found")
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
