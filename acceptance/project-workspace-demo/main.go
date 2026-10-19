package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

const tenantID = "99999999-9999-4999-8999-999999999999"

type config struct {
	baseURL     string
	accessToken string
}

type project struct {
	ID                 string   `json:"id"`
	TenantID           string   `json:"tenant_id"`
	ProjectNumber      string   `json:"project_number"`
	Name               string   `json:"name"`
	Summary            string   `json:"summary"`
	ProjectTypeKeys    []string `json:"project_type_keys"`
	Status             string   `json:"status"`
	Revision           int64    `json:"revision"`
	CancellationReason *string  `json:"cancellation_reason"`
}

type assignment struct {
	ID                     string `json:"id"`
	ProjectID              string `json:"project_id"`
	SubjectKind            string `json:"subject_kind"`
	SubjectID              string `json:"subject_id"`
	SourceRevision         int64  `json:"source_revision"`
	DisplaySnapshot        string `json:"display_snapshot"`
	RoleKey                string `json:"role_key"`
	EndedProjectRevision   *int64 `json:"ended_project_revision"`
	CreatedProjectRevision int64  `json:"created_project_revision"`
}

type resourceLink struct {
	ID                      string `json:"id"`
	ProjectID               string `json:"project_id"`
	TargetService           string `json:"target_service"`
	ResourceType            string `json:"resource_type"`
	ResourceID              string `json:"resource_id"`
	RelationKey             string `json:"relation_key"`
	CreatedProjectRevision  int64  `json:"created_project_revision"`
	UnlinkedProjectRevision *int64 `json:"unlinked_project_revision"`
}

type assignmentResult struct {
	Project    project    `json:"project"`
	Assignment assignment `json:"assignment"`
}

type resourceLinkResult struct {
	Project      project      `json:"project"`
	ResourceLink resourceLink `json:"resource_link"`
}

type referenceStatus struct {
	LinkID string `json:"link_id"`
	Status string `json:"status"`
}

type details struct {
	Project           project           `json:"project"`
	Assignments       []assignment      `json:"assignments"`
	ResourceLinks     []resourceLink    `json:"resource_links"`
	ReferenceStatuses []referenceStatus `json:"reference_statuses"`
}

type historyEvent struct {
	ID              string `json:"id"`
	ProjectRevision int64  `json:"project_revision"`
	EventSequence   int64  `json:"event_sequence"`
	EventType       string `json:"event_type"`
	CommandID       string `json:"command_id"`
	ActorSubject    string `json:"actor_subject"`
}

type historyPage struct {
	Items []historyEvent `json:"items"`
}

type projectReference struct {
	ID              string   `json:"id"`
	TenantID        string   `json:"tenant_id"`
	ResourceType    string   `json:"resource_type"`
	Status          string   `json:"status"`
	Revision        int64    `json:"revision"`
	ProjectTypeKeys []string `json:"project_type_keys"`
}

type assignmentReference struct {
	ID           string `json:"id"`
	ProjectID    string `json:"project_id"`
	ResourceType string `json:"resource_type"`
	RoleKey      string `json:"role_key"`
	Status       string `json:"status"`
	Revision     int64  `json:"revision"`
	DisplayName  string `json:"display_name"`
}

type demoClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: project-workspace-demo")
		os.Exit(2)
	}
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "Project Workspace demo failed:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	client := &demoClient{baseURL: cfg.baseURL, token: cfg.accessToken, http: &http.Client{Timeout: 10 * time.Second}}
	nonce, err := randomHex(6)
	if err != nil {
		return err
	}
	ids := scenarioIDs{
		person:              scopedID("11111111-1111-4111-8111", nonce),
		group:               scopedID("22222222-2222-4222-8222", nonce),
		missingPerson:       scopedID("99999999-9999-4999-8999", nonce),
		unavailablePerson:   scopedID("33333333-3333-4333-8333", nonce),
		contract:            scopedID("55555555-5555-4555-8555", nonce),
		missingContract:     scopedID("99999999-9999-4999-8999", nonce),
		unavailableContract: scopedID("66666666-6666-4666-8666", nonce),
		evidence:            scopedID("77777777-7777-4777-8777", nonce),
	}
	commands := commandSet{nonce: nonce}

	fmt.Println("[1/10] 验证 Project 就绪，且匿名与错误模板令牌失败关闭")
	var ready map[string]string
	if err := client.json(ctx, http.MethodGet, "/health/ready", nil, http.StatusOK, &ready); err != nil {
		return err
	}
	if ready["status"] != "ready" {
		return fmt.Errorf("unexpected readiness payload: %#v", ready)
	}
	probe := map[string]any{"command_id": commands.id(1), "project_number": "PROBE-" + nonce, "name": "probe", "project_type_keys": []string{"design"}}
	if err := client.jsonWithToken(ctx, http.MethodPost, "/api/v1/projects", "", probe, http.StatusUnauthorized, nil); err != nil {
		return fmt.Errorf("anonymous create did not fail closed: %w", err)
	}
	if err := client.jsonWithToken(ctx, http.MethodPost, "/api/v1/projects", "wrong-template-token", probe, http.StatusUnauthorized, nil); err != nil {
		return fmt.Errorf("wrong token did not fail closed: %w", err)
	}

	fmt.Println("[2/10] 创建并幂等重放设计项目，拒绝同 Command ID 的不同载荷")
	createPayload := map[string]any{
		"command_id": commands.id(2), "project_number": "BF-PROJECT-" + strings.ToUpper(nonce),
		"name": "青岚办公空间改造", "summary": "Project 独立 Demo 的立项草稿", "project_type_keys": []string{"design"},
		"planned_start_date": "2026-10-01T00:00:00Z", "planned_end_date": "2026-12-31T00:00:00Z",
	}
	var value project
	if err := client.json(ctx, http.MethodPost, "/api/v1/projects", createPayload, http.StatusCreated, &value); err != nil {
		return err
	}
	if value.ID == "" || value.Status != "draft" || value.Revision != 1 || value.TenantID != tenantID {
		return fmt.Errorf("unexpected created project: %#v", value)
	}
	var replay project
	if err := client.json(ctx, http.MethodPost, "/api/v1/projects", createPayload, http.StatusCreated, &replay); err != nil {
		return err
	}
	if replay.ID != value.ID || replay.Revision != 1 {
		return errors.New("create replay returned a different project")
	}
	conflictPayload := cloneMap(createPayload)
	conflictPayload["name"] = "不同项目"
	if err := client.json(ctx, http.MethodPost, "/api/v1/projects", conflictPayload, http.StatusConflict, nil); err != nil {
		return fmt.Errorf("create command conflict: %w", err)
	}

	fmt.Println("[3/10] 更新草稿并拒绝过期 revision；不完整立项进入待补全态")
	updatePayload := cloneMap(createPayload)
	updatePayload["command_id"] = commands.id(3)
	updatePayload["expected_revision"] = 1
	updatePayload["summary"] = "已明确时间范围、经营负责人和设计生产组的立项资料"
	if err := client.json(ctx, http.MethodPut, "/api/v1/projects/"+value.ID, updatePayload, http.StatusOK, &value); err != nil {
		return err
	}
	if value.Revision != 2 || value.Status != "draft" {
		return fmt.Errorf("unexpected updated project: %#v", value)
	}
	staleUpdate := cloneMap(updatePayload)
	staleUpdate["command_id"] = commands.id(4)
	staleUpdate["summary"] = "过期写入"
	if err := client.json(ctx, http.MethodPut, "/api/v1/projects/"+value.ID, staleUpdate, http.StatusConflict, nil); err != nil {
		return fmt.Errorf("stale update was not rejected: %w", err)
	}
	var incomplete project
	if err := client.json(ctx, http.MethodPost, "/api/v1/projects", map[string]any{
		"command_id": commands.id(5), "project_number": "BF-INCOMPLETE-" + strings.ToUpper(nonce), "name": "待补全项目", "project_type_keys": []string{"design"},
	}, http.StatusCreated, &incomplete); err != nil {
		return err
	}
	if err := client.json(ctx, http.MethodPost, "/api/v1/projects/"+incomplete.ID+"/complete-initiation", map[string]any{
		"command_id": commands.id(6), "expected_revision": 1,
	}, http.StatusUnprocessableEntity, nil); err != nil {
		return fmt.Errorf("incomplete initiation was not rejected: %w", err)
	}
	var incompleteDetails details
	if err := client.json(ctx, http.MethodGet, "/api/v1/projects/"+incomplete.ID, nil, http.StatusOK, &incompleteDetails); err != nil {
		return err
	}
	if incompleteDetails.Project.Status != "pending_completion" || incompleteDetails.Project.Revision != 2 {
		return fmt.Errorf("incomplete project did not preserve pending state: %#v", incompleteDetails.Project)
	}

	fmt.Println("[4/10] 引用不存在和暂不可用时不写入 Assignment")
	assignmentPath := "/api/v1/projects/" + value.ID + "/assignments"
	if err := client.json(ctx, http.MethodPost, assignmentPath, assignmentPayload(commands.id(7), 2, ids.missingPerson, "person", 1, "participant"), http.StatusUnprocessableEntity, nil); err != nil {
		return fmt.Errorf("missing person: %w", err)
	}
	if err := client.json(ctx, http.MethodPost, assignmentPath, assignmentPayload(commands.id(8), 2, ids.unavailablePerson, "person", 1, "participant"), http.StatusServiceUnavailable, nil); err != nil {
		return fmt.Errorf("unavailable person: %w", err)
	}
	if err := client.assertShape(ctx, value.ID, 2, 0, 0); err != nil {
		return fmt.Errorf("failed reference caused partial write: %w", err)
	}

	fmt.Println("[5/10] 分配经营负责人和设计生产组，并证明重放不重新依赖上游")
	businessPayload := assignmentPayload(commands.id(9), 2, ids.person, "person", 1, "business_owner")
	var business assignmentResult
	if err := client.json(ctx, http.MethodPost, assignmentPath, businessPayload, http.StatusOK, &business); err != nil {
		return err
	}
	if business.Project.Revision != 3 || business.Assignment.DisplaySnapshot != "经营负责人 林清和" {
		return fmt.Errorf("unexpected business owner result: %#v", business)
	}
	var businessReplay assignmentResult
	if err := client.json(ctx, http.MethodPost, assignmentPath, businessPayload, http.StatusOK, &businessReplay); err != nil {
		return fmt.Errorf("assignment replay: %w", err)
	}
	if businessReplay.Assignment.ID != business.Assignment.ID || businessReplay.Project.Revision != 3 {
		return errors.New("assignment replay returned a different fact")
	}
	if err := client.json(ctx, http.MethodPost, assignmentPath, assignmentPayload(commands.id(10), 3, ids.person, "person", 1, "participant"), http.StatusServiceUnavailable, nil); err != nil {
		return fmt.Errorf("reference stub did not prove replay isolation: %w", err)
	}
	var production assignmentResult
	if err := client.json(ctx, http.MethodPost, assignmentPath, assignmentPayload(commands.id(11), 3, ids.group, "group", 2, "production_owner"), http.StatusOK, &production); err != nil {
		return err
	}
	if production.Project.Revision != 4 || production.Assignment.DisplaySnapshot != "青岚设计生产组" {
		return fmt.Errorf("unexpected production owner result: %#v", production)
	}

	fmt.Println("[6/10] 补全立项成为 active Project")
	if err := client.json(ctx, http.MethodPost, "/api/v1/projects/"+value.ID+"/complete-initiation", map[string]any{
		"command_id": commands.id(12), "expected_revision": 4,
	}, http.StatusOK, &value); err != nil {
		return err
	}
	if value.Status != "active" || value.Revision != 5 {
		return fmt.Errorf("project did not become active revision 5: %#v", value)
	}

	fmt.Println("[7/10] 资源引用不存在／不可用时失败关闭，再建立两条稳定 Link")
	linkPath := "/api/v1/projects/" + value.ID + "/resource-links"
	if err := client.json(ctx, http.MethodPost, linkPath, linkPayload(commands.id(13), 5, "contract-ledger", "contract", ids.missingContract, 3, "missing-contract"), http.StatusUnprocessableEntity, nil); err != nil {
		return fmt.Errorf("missing contract: %w", err)
	}
	if err := client.json(ctx, http.MethodPost, linkPath, linkPayload(commands.id(14), 5, "contract-ledger", "contract", ids.unavailableContract, 3, "unavailable-contract"), http.StatusServiceUnavailable, nil); err != nil {
		return fmt.Errorf("unavailable contract: %w", err)
	}
	contractPayload := linkPayload(commands.id(15), 5, "contract-ledger", "contract", ids.contract, 3, "primary-contract")
	var contractLink resourceLinkResult
	if err := client.json(ctx, http.MethodPost, linkPath, contractPayload, http.StatusOK, &contractLink); err != nil {
		return err
	}
	if contractLink.Project.Revision != 6 {
		return fmt.Errorf("contract link revision=%d, want 6", contractLink.Project.Revision)
	}
	var contractReplay resourceLinkResult
	if err := client.json(ctx, http.MethodPost, linkPath, contractPayload, http.StatusOK, &contractReplay); err != nil {
		return fmt.Errorf("resource link replay: %w", err)
	}
	if contractReplay.ResourceLink.ID != contractLink.ResourceLink.ID || contractReplay.Project.Revision != 6 {
		return errors.New("resource link replay returned a different fact")
	}
	var evidenceLink resourceLinkResult
	if err := client.json(ctx, http.MethodPost, linkPath, linkPayload(commands.id(16), 6, "evidence-service", "evidence-file", ids.evidence, 1, "brief-evidence"), http.StatusOK, &evidenceLink); err != nil {
		return err
	}
	if evidenceLink.Project.Revision != 7 {
		return fmt.Errorf("evidence link revision=%d, want 7", evidenceLink.Project.Revision)
	}

	fmt.Println("[8/10] 查询时明确展示一条 available 与一条 unavailable 引用，再受控解除 Link")
	var linkedDetails details
	if err := client.json(ctx, http.MethodGet, "/api/v1/projects/"+value.ID, nil, http.StatusOK, &linkedDetails); err != nil {
		return err
	}
	statuses := map[string]string{}
	for _, item := range linkedDetails.ReferenceStatuses {
		statuses[item.LinkID] = item.Status
	}
	if linkedDetails.Project.Revision != 7 || len(linkedDetails.Assignments) != 2 || len(linkedDetails.ResourceLinks) != 2 || statuses[contractLink.ResourceLink.ID] != "unavailable" || statuses[evidenceLink.ResourceLink.ID] != "available" {
		return fmt.Errorf("unexpected linked details: %#v statuses=%#v", linkedDetails, statuses)
	}
	var unlinked resourceLinkResult
	if err := client.json(ctx, http.MethodPost, linkPath, map[string]any{
		"command_id": commands.id(17), "expected_revision": 7, "operation": "unlink", "link_id": evidenceLink.ResourceLink.ID,
	}, http.StatusOK, &unlinked); err != nil {
		return err
	}
	if unlinked.Project.Revision != 8 || unlinked.ResourceLink.UnlinkedProjectRevision == nil || *unlinked.ResourceLink.UnlinkedProjectRevision != 8 {
		return fmt.Errorf("unexpected unlink result: %#v", unlinked)
	}

	fmt.Println("[9/10] 核对公开 Project／Assignment 引用与不可变历史")
	var projectRef projectReference
	if err := client.json(ctx, http.MethodGet, "/api/v1/references/projects/"+value.ID, nil, http.StatusOK, &projectRef); err != nil {
		return err
	}
	if projectRef.ID != value.ID || projectRef.ResourceType != "project" || projectRef.Status != "active" || projectRef.Revision != 8 || !contains(projectRef.ProjectTypeKeys, "design") {
		return fmt.Errorf("unexpected project reference: %#v", projectRef)
	}
	var assignmentRef assignmentReference
	if err := client.json(ctx, http.MethodGet, "/api/v1/references/projects/"+value.ID+"/assignments/"+production.Assignment.ID, nil, http.StatusOK, &assignmentRef); err != nil {
		return err
	}
	if assignmentRef.ID != production.Assignment.ID || assignmentRef.RoleKey != "production_owner" || assignmentRef.Status != "active" || assignmentRef.DisplayName != "青岚设计生产组" {
		return fmt.Errorf("unexpected assignment reference: %#v", assignmentRef)
	}

	fmt.Println("[10/10] 取消项目、拒绝终态写入，并核对 revision 1～9 历史")
	if err := client.json(ctx, http.MethodPost, "/api/v1/projects/"+value.ID+"/cancel", map[string]any{
		"command_id": commands.id(18), "expected_revision": 8, "reason": "独立 Demo 完成后的受控取消",
	}, http.StatusOK, &value); err != nil {
		return err
	}
	if value.Status != "cancelled" || value.Revision != 9 || value.CancellationReason == nil {
		return fmt.Errorf("unexpected cancelled project: %#v", value)
	}
	if err := client.json(ctx, http.MethodPost, assignmentPath, map[string]any{
		"command_id": commands.id(19), "expected_revision": 9, "operation": "end", "assignment_id": business.Assignment.ID, "end_reason": "终态后不应执行",
	}, http.StatusConflict, nil); err != nil {
		return fmt.Errorf("cancelled project accepted a write: %w", err)
	}
	var history historyPage
	if err := client.json(ctx, http.MethodGet, "/api/v1/projects/"+value.ID+"/history?limit=100", nil, http.StatusOK, &history); err != nil {
		return err
	}
	if err := verifyHistory(history, 9); err != nil {
		return err
	}

	result := map[string]any{
		"status": "passed", "project_id": value.ID, "status_final": value.Status, "revision": value.Revision,
		"assignments": 2, "resource_links_total": 2, "history_events": len(history.Items),
	}
	encoded, _ := json.Marshal(result)
	fmt.Println(string(encoded))
	return nil
}

type scenarioIDs struct {
	person, group, missingPerson, unavailablePerson          string
	contract, missingContract, unavailableContract, evidence string
}

type commandSet struct{ nonce string }

func (c commandSet) id(sequence int) string {
	suffix := c.nonce[:10] + fmt.Sprintf("%02x", sequence)
	return "aaaaaaaa-aaaa-4aaa-8aaa-" + suffix
}

func scopedID(prefix, nonce string) string { return prefix + "-" + nonce }

func assignmentPayload(command string, revision int64, subjectID, kind string, sourceRevision int64, role string) map[string]any {
	return map[string]any{
		"command_id": command, "expected_revision": revision, "operation": "assign", "subject_kind": kind,
		"subject_id": subjectID, "source_revision": sourceRevision, "role_key": role,
	}
}

func linkPayload(command string, revision int64, service, resourceType, resourceID string, sourceRevision int64, relation string) map[string]any {
	return map[string]any{
		"command_id": command, "expected_revision": revision, "operation": "link", "target_service": service,
		"resource_type": resourceType, "resource_id": resourceID, "source_revision": sourceRevision, "relation_key": relation,
	}
}

func loadConfig() (config, error) {
	value := config{baseURL: strings.TrimRight(strings.TrimSpace(os.Getenv("DEMO_PROJECT_URL")), "/"), accessToken: os.Getenv("DEMO_TEMPLATE_ACCESS_TOKEN")}
	if value.baseURL == "" || len(value.accessToken) < 24 {
		return config{}, errors.New("DEMO_PROJECT_URL and a 24+ byte DEMO_TEMPLATE_ACCESS_TOKEN are required")
	}
	parsed, err := url.Parse(value.baseURL)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" {
		return config{}, errors.New("DEMO_PROJECT_URL must be an absolute URL")
	}
	return value, nil
}

func (c *demoClient) json(ctx context.Context, method, path string, payload any, expected int, destination any) error {
	return c.jsonWithToken(ctx, method, path, c.token, payload, expected, destination)
}

func (c *demoClient) jsonWithToken(ctx context.Context, method, path, token string, payload any, expected int, destination any) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	request.Header.Set("X-Actor-ID", "forged-actor")
	request.Header.Set("X-Tenant-ID", "forged-tenant")
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return err
	}
	if response.StatusCode != expected {
		return fmt.Errorf("%s %s returned %d, want %d: %s", method, path, response.StatusCode, expected, strings.TrimSpace(string(data)))
	}
	if destination != nil {
		if err := json.Unmarshal(data, destination); err != nil {
			return fmt.Errorf("decode %s %s: %w (%s)", method, path, err, strings.TrimSpace(string(data)))
		}
	}
	return nil
}

func (c *demoClient) assertShape(ctx context.Context, projectID string, revision int64, assignments, links int) error {
	var value details
	if err := c.json(ctx, http.MethodGet, "/api/v1/projects/"+projectID, nil, http.StatusOK, &value); err != nil {
		return err
	}
	if value.Project.Revision != revision || len(value.Assignments) != assignments || len(value.ResourceLinks) != links {
		return fmt.Errorf("revision=%d assignments=%d links=%d", value.Project.Revision, len(value.Assignments), len(value.ResourceLinks))
	}
	return nil
}

func verifyHistory(value historyPage, finalRevision int64) error {
	if len(value.Items) < int(finalRevision) {
		return fmt.Errorf("history has %d events, want at least %d", len(value.Items), finalRevision)
	}
	revisions := make([]int, 0, len(value.Items))
	seen := map[int64]bool{}
	for _, item := range value.Items {
		if item.ID == "" || item.CommandID == "" || item.EventType == "" || item.ActorSubject == "" || item.ActorSubject == "forged-actor" {
			return fmt.Errorf("invalid history event: %#v", item)
		}
		seen[item.ProjectRevision] = true
		revisions = append(revisions, int(item.ProjectRevision))
	}
	for revision := int64(1); revision <= finalRevision; revision++ {
		if !seen[revision] {
			return fmt.Errorf("history is missing project revision %d; observed=%v", revision, revisions)
		}
	}
	sort.Ints(revisions)
	return nil
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func cloneMap(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}

func randomHex(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
