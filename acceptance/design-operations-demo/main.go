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
	"reflect"
	"strings"
	"time"
)

const tenantID = "99999999-9999-4999-8999-999999999999"

type workItem struct {
	ID                    string  `json:"id"`
	TenantID              string  `json:"tenant_id"`
	ProjectID             string  `json:"project_id"`
	ProjectSourceRevision int64   `json:"project_source_revision"`
	Status                string  `json:"status"`
	Revision              int64   `json:"revision"`
	OwnerAssignmentID     *string `json:"owner_assignment_id"`
	CurrentStageKey       *string `json:"current_stage_key"`
	CancellationReason    *string `json:"cancellation_reason"`
}

type stage struct {
	Key                 string  `json:"stage_key"`
	Status              string  `json:"status"`
	NotApplicableReason *string `json:"not_applicable_reason"`
}

type deliverable struct {
	ID                     string `json:"id"`
	StageKey               string `json:"stage_key"`
	EvidenceFileID         string `json:"evidence_file_id"`
	EvidenceVersion        int    `json:"evidence_version"`
	EvidenceSourceRevision int64  `json:"evidence_source_revision"`
	DisplaySnapshot        string `json:"display_snapshot"`
}

type referenceStatus struct {
	Kind, ID, Status string
}

func (r *referenceStatus) UnmarshalJSON(data []byte) error {
	var raw struct {
		Kind   string `json:"kind"`
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	r.Kind, r.ID, r.Status = raw.Kind, raw.ID, raw.Status
	return nil
}

type details struct {
	WorkItem          workItem          `json:"work_item"`
	Stages            []stage           `json:"stages"`
	Deliverables      []deliverable     `json:"deliverables"`
	ReferenceStatuses []referenceStatus `json:"reference_statuses"`
}

type historyEvent struct {
	ID               string         `json:"id"`
	WorkItemRevision int64          `json:"work_item_revision"`
	EventSequence    int64          `json:"event_sequence"`
	EventType        string         `json:"event_type"`
	CommandID        string         `json:"command_id"`
	ActorSubject     string         `json:"actor_subject"`
	Payload          map[string]any `json:"payload"`
}

type historyPage struct {
	Items []historyEvent `json:"items"`
}

type referenceView struct {
	ID             string   `json:"id"`
	ProjectID      string   `json:"project_id"`
	ResourceType   string   `json:"resource_type"`
	Status         string   `json:"status"`
	Revision       int64    `json:"revision"`
	CompletionGaps []string `json:"completion_gaps"`
}

type demoClient struct {
	baseURL string
	token   string
	http    *http.Client
}

type scenarioIDs struct {
	project, missingProject, unavailableProject, inactiveProject      string
	flakyOwner, owner, missingOwner, unavailableOwner, invalidOwner   string
	schemeOne, schemeTwo, preliminary, drawing, flakySupport, support string
	missingEvidence, unavailableEvidence                              string
}

type commandSet struct{ nonce string }

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: design-operations-demo")
		os.Exit(2)
	}
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "Design Operations demo failed:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("DEMO_DESIGN_URL")), "/")
	token := os.Getenv("DEMO_TEMPLATE_ACCESS_TOKEN")
	parsed, err := url.Parse(baseURL)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || len(token) < 24 {
		return errors.New("DEMO_DESIGN_URL and a 24+ byte DEMO_TEMPLATE_ACCESS_TOKEN are required")
	}
	client := &demoClient{baseURL: baseURL, token: token, http: &http.Client{Timeout: 10 * time.Second}}
	nonce, err := randomHex(6)
	if err != nil {
		return err
	}
	ids := scenarioIDs{
		project: scopedID("88888888-8888-4888-8888", nonce), missingProject: scopedID("99999999-9999-4999-8999", nonce),
		unavailableProject: scopedID("33333333-3333-4333-8333", nonce), inactiveProject: scopedID("44444444-4444-4444-8444", nonce),
		flakyOwner: scopedID("11111111-1111-4111-8111", nonce), owner: scopedID("22222222-2222-4222-8222", nonce),
		missingOwner: scopedID("99999999-9999-4999-8999", nonce), unavailableOwner: scopedID("33333333-3333-4333-8333", nonce), invalidOwner: scopedID("44444444-4444-4444-8444", nonce),
		schemeOne: scopedID("55555551-5555-4555-8555", nonce), schemeTwo: scopedID("55555552-5555-4555-8555", nonce), preliminary: scopedID("55555553-5555-4555-8555", nonce),
		drawing: scopedID("55555554-5555-4555-8555", nonce), flakySupport: scopedID("66666666-6666-4666-8666", nonce), support: scopedID("55555555-5555-4555-8555", nonce),
		missingEvidence: scopedID("99999999-9999-4999-8999", nonce), unavailableEvidence: scopedID("77777777-7777-4777-8777", nonce),
	}
	commands := commandSet{nonce: nonce}

	fmt.Println("[1/10] 验证 Design 就绪，且匿名与错误模板令牌失败关闭")
	var ready map[string]string
	if err := client.json(ctx, http.MethodGet, "/health/ready", nil, http.StatusOK, &ready); err != nil {
		return err
	}
	probe := createPayload(commands.id(1), ids.project)
	if err := client.jsonWithToken(ctx, http.MethodPost, "/api/v1/design-work-items", "", probe, http.StatusUnauthorized, nil); err != nil {
		return err
	}
	if err := client.jsonWithToken(ctx, http.MethodPost, "/api/v1/design-work-items", "wrong-template-token", probe, http.StatusUnauthorized, nil); err != nil {
		return err
	}

	fmt.Println("[2/10] Project 引用三态失败关闭，再创建并幂等重放 Work Item")
	for _, item := range []struct {
		id     string
		status int
		seq    int
	}{{ids.missingProject, http.StatusUnprocessableEntity, 2}, {ids.unavailableProject, http.StatusServiceUnavailable, 3}, {ids.inactiveProject, http.StatusUnprocessableEntity, 4}} {
		if err := client.json(ctx, http.MethodPost, "/api/v1/design-work-items", createPayload(commands.id(item.seq), item.id), item.status, nil); err != nil {
			return err
		}
	}
	create := createPayload(commands.id(5), ids.project)
	var value details
	if err := client.json(ctx, http.MethodPost, "/api/v1/design-work-items", create, http.StatusCreated, &value); err != nil {
		return err
	}
	if value.WorkItem.ID == "" || value.WorkItem.Status != "draft" || value.WorkItem.Revision != 1 || len(value.Stages) != 4 || stageStatus(value, "scheme") != "ready" {
		return fmt.Errorf("unexpected created work item: %#v", value)
	}
	var replay details
	if err := client.json(ctx, http.MethodPost, "/api/v1/design-work-items", create, http.StatusCreated, &replay); err != nil {
		return err
	}
	if replay.WorkItem.ID != value.WorkItem.ID || replay.WorkItem.Revision != 1 {
		return errors.New("create replay returned a different work item")
	}
	if err := client.json(ctx, http.MethodPost, "/api/v1/design-work-items", createPayload(commands.id(6), ids.project), http.StatusConflict, nil); err != nil {
		return fmt.Errorf("active uniqueness: %w", err)
	}

	fmt.Println("[3/10] Assignment 引用三态与角色校验失败关闭，并从负责人失效中恢复")
	ownerPath := "/api/v1/design-work-items/" + value.WorkItem.ID + "/owner"
	for _, item := range []struct {
		id     string
		status int
		seq    int
	}{{ids.missingOwner, http.StatusUnprocessableEntity, 7}, {ids.unavailableOwner, http.StatusServiceUnavailable, 8}, {ids.invalidOwner, http.StatusUnprocessableEntity, 9}} {
		if err := client.json(ctx, http.MethodPost, ownerPath, ownerPayload(commands.id(item.seq), 1, item.id), item.status, nil); err != nil {
			return err
		}
	}
	if err := client.assertRevision(ctx, value.WorkItem.ID, 1); err != nil {
		return fmt.Errorf("failed owner reference caused partial write: %w", err)
	}
	if err := client.json(ctx, http.MethodPost, ownerPath, ownerPayload(commands.id(10), 1, ids.flakyOwner), http.StatusOK, &value); err != nil {
		return err
	}
	if value.WorkItem.Revision != 2 || value.WorkItem.Status != "in-progress" {
		return fmt.Errorf("unexpected first owner result: %#v", value.WorkItem)
	}
	if err := client.json(ctx, http.MethodPost, stagePath(value.WorkItem.ID, "scheme", "complete"), revisionPayload(commands.id(11), 2), http.StatusServiceUnavailable, nil); err != nil {
		return fmt.Errorf("ended owner guard: %w", err)
	}
	stableOwner := ownerPayload(commands.id(12), 2, ids.owner)
	if err := client.json(ctx, http.MethodPost, ownerPath, stableOwner, http.StatusOK, &value); err != nil {
		return err
	}
	if value.WorkItem.Revision != 3 {
		return fmt.Errorf("stable owner revision=%d, want 3", value.WorkItem.Revision)
	}
	var ownerReplay details
	if err := client.json(ctx, http.MethodPost, ownerPath, stableOwner, http.StatusOK, &ownerReplay); err != nil || ownerReplay.WorkItem.Revision != 3 {
		return fmt.Errorf("owner replay failed: %w", err)
	}

	fmt.Println("[4/10] Evidence 不存在／不可用与缺成果均不推进 Stage")
	if err := client.json(ctx, http.MethodPost, stagePath(value.WorkItem.ID, "scheme", "complete"), revisionPayload(commands.id(13), 3), http.StatusUnprocessableEntity, nil); err != nil {
		return err
	}
	for _, item := range []struct {
		id     string
		status int
		seq    int
	}{{ids.missingEvidence, http.StatusUnprocessableEntity, 14}, {ids.unavailableEvidence, http.StatusServiceUnavailable, 15}} {
		if err := client.json(ctx, http.MethodPost, stagePath(value.WorkItem.ID, "scheme", "deliverables"), deliverablePayload(commands.id(item.seq), 3, item.id), item.status, nil); err != nil {
			return err
		}
	}
	if err := client.assertRevision(ctx, value.WorkItem.ID, 3); err != nil {
		return fmt.Errorf("failed evidence caused partial write: %w", err)
	}

	fmt.Println("[5/10] 登记、幂等重放并替换方案成果，再完成方案阶段")
	schemePath := stagePath(value.WorkItem.ID, "scheme", "deliverables")
	firstDeliverable := deliverablePayload(commands.id(16), 3, ids.schemeOne)
	var first, firstReplay deliverable
	if err := client.json(ctx, http.MethodPost, schemePath, firstDeliverable, http.StatusOK, &first); err != nil {
		return err
	}
	if err := client.json(ctx, http.MethodPost, schemePath, firstDeliverable, http.StatusOK, &firstReplay); err != nil || firstReplay.ID != first.ID {
		return fmt.Errorf("deliverable replay failed: %w", err)
	}
	conflicting := cloneMap(firstDeliverable)
	conflicting["evidence_file_id"] = ids.schemeTwo
	if err := client.json(ctx, http.MethodPost, schemePath, conflicting, http.StatusConflict, nil); err != nil {
		return err
	}
	var second deliverable
	if err := client.json(ctx, http.MethodPost, schemePath, deliverablePayload(commands.id(17), 4, ids.schemeTwo), http.StatusOK, &second); err != nil {
		return err
	}
	if second.ID == first.ID || second.EvidenceFileID != ids.schemeTwo {
		return fmt.Errorf("scheme replacement is not append-only: first=%#v second=%#v", first, second)
	}
	if err := client.json(ctx, http.MethodPost, stagePath(value.WorkItem.ID, "scheme", "complete"), revisionPayload(commands.id(18), 5), http.StatusOK, &value); err != nil {
		return err
	}
	if value.WorkItem.Revision != 6 || stageStatus(value, "scheme") != "completed" || stageStatus(value, "preliminary-design") != "ready" {
		return fmt.Errorf("scheme did not complete correctly: %#v", value)
	}

	fmt.Println("[6/10] 显式跳过初步设计，并推进到施工图")
	preliminaryApplicability := stagePath(value.WorkItem.ID, "preliminary-design", "applicability")
	if err := client.json(ctx, http.MethodPost, preliminaryApplicability, applicabilityPayload(commands.id(19), 6, "set-not-applicable", "演示第一次明确跳过初设"), http.StatusOK, &value); err != nil {
		return err
	}
	if value.WorkItem.Revision != 7 || stageStatus(value, "preliminary-design") != "not-applicable" || stageStatus(value, "construction-drawing") != "ready" {
		return fmt.Errorf("preliminary skip failed: %#v", value)
	}

	fmt.Println("[7/10] 禁止施工图设为不适用，并顺序完成施工图")
	if err := client.json(ctx, http.MethodPost, stagePath(value.WorkItem.ID, "construction-drawing", "applicability"), applicabilityPayload(commands.id(20), 7, "set-not-applicable", "非法跳过"), http.StatusUnprocessableEntity, nil); err != nil {
		return err
	}
	if err := client.json(ctx, http.MethodPost, stagePath(value.WorkItem.ID, "construction-drawing", "deliverables"), deliverablePayload(commands.id(21), 7, ids.drawing), http.StatusOK, &first); err != nil {
		return err
	}
	if err := client.json(ctx, http.MethodPost, stagePath(value.WorkItem.ID, "construction-drawing", "complete"), revisionPayload(commands.id(22), 8), http.StatusOK, &value); err != nil {
		return err
	}
	if value.WorkItem.Revision != 9 || stageStatus(value, "construction-support") != "ready" {
		return fmt.Errorf("drawing completion failed: %#v", value)
	}

	fmt.Println("[8/10] 完成最后阶段，核对 completed 引用与 Evidence 降级展示")
	if err := client.json(ctx, http.MethodPost, stagePath(value.WorkItem.ID, "construction-support", "deliverables"), deliverablePayload(commands.id(23), 9, ids.flakySupport), http.StatusOK, &first); err != nil {
		return err
	}
	if err := client.json(ctx, http.MethodPost, stagePath(value.WorkItem.ID, "construction-support", "complete"), revisionPayload(commands.id(24), 10), http.StatusOK, &value); err != nil {
		return err
	}
	if value.WorkItem.Status != "completed" || value.WorkItem.Revision != 11 || value.WorkItem.CurrentStageKey != nil {
		return fmt.Errorf("work item did not complete: %#v", value.WorkItem)
	}
	var reference referenceView
	if err := client.json(ctx, http.MethodGet, "/api/v1/references/design-work-items/"+value.WorkItem.ID, nil, http.StatusOK, &reference); err != nil {
		return err
	}
	if reference.ID != value.WorkItem.ID || reference.Status != "completed" || reference.Revision != 11 || len(reference.CompletionGaps) != 0 {
		return fmt.Errorf("unexpected completed reference: %#v", reference)
	}
	var degraded details
	if err := client.json(ctx, http.MethodGet, "/api/v1/design-work-items/"+value.WorkItem.ID, nil, http.StatusOK, &degraded); err != nil {
		return err
	}
	statuses := map[string]string{}
	for _, item := range degraded.ReferenceStatuses {
		statuses[item.ID] = item.Status
	}
	if statuses[ids.flakySupport] != "unavailable" || statuses[ids.drawing] != "available" {
		return fmt.Errorf("unexpected degraded references: %#v", statuses)
	}

	fmt.Println("[9/10] 恢复并补做初设，替换失效成果，再验证施工图重开与级联回退")
	if err := client.json(ctx, http.MethodPost, preliminaryApplicability, applicabilityPayload(commands.id(25), 11, "restore-applicability", "客户要求补做初设"), http.StatusOK, &value); err != nil {
		return err
	}
	if value.WorkItem.Revision != 12 || value.WorkItem.Status != "in-progress" || stageStatus(value, "preliminary-design") != "ready" || stageStatus(value, "construction-drawing") != "pending" || stageStatus(value, "construction-support") != "pending" {
		return fmt.Errorf("preliminary restore failed: %#v", value)
	}
	if err := client.json(ctx, http.MethodPost, stagePath(value.WorkItem.ID, "preliminary-design", "deliverables"), deliverablePayload(commands.id(26), 12, ids.preliminary), http.StatusOK, &first); err != nil {
		return err
	}
	if err := client.json(ctx, http.MethodPost, stagePath(value.WorkItem.ID, "preliminary-design", "complete"), revisionPayload(commands.id(27), 13), http.StatusOK, &value); err != nil {
		return err
	}
	if value.WorkItem.Revision != 14 || stageStatus(value, "construction-drawing") != "ready" {
		return fmt.Errorf("preliminary completion failed: %#v", value)
	}
	if err := client.json(ctx, http.MethodPost, stagePath(value.WorkItem.ID, "construction-drawing", "complete"), revisionPayload(commands.id(28), 14), http.StatusOK, &value); err != nil {
		return err
	}
	if err := client.json(ctx, http.MethodPost, stagePath(value.WorkItem.ID, "construction-support", "complete"), revisionPayload(commands.id(29), 15), http.StatusServiceUnavailable, nil); err != nil {
		return err
	}
	if err := client.assertRevision(ctx, value.WorkItem.ID, 15); err != nil {
		return fmt.Errorf("unavailable evidence caused partial completion: %w", err)
	}
	if err := client.json(ctx, http.MethodPost, stagePath(value.WorkItem.ID, "construction-support", "deliverables"), deliverablePayload(commands.id(30), 15, ids.support), http.StatusOK, &first); err != nil {
		return err
	}
	if err := client.json(ctx, http.MethodPost, stagePath(value.WorkItem.ID, "construction-support", "complete"), revisionPayload(commands.id(31), 16), http.StatusOK, &value); err != nil {
		return err
	}
	if value.WorkItem.Status != "completed" || value.WorkItem.Revision != 17 {
		return fmt.Errorf("work item did not complete after restoration: %#v", value.WorkItem)
	}
	if err := client.json(ctx, http.MethodPost, stagePath(value.WorkItem.ID, "construction-drawing", "reopen"), reasonPayload(commands.id(32), 17, "施工图审查意见要求回退"), http.StatusOK, &value); err != nil {
		return err
	}
	if value.WorkItem.Revision != 18 || value.WorkItem.Status != "in-progress" || stageStatus(value, "construction-drawing") != "ready" || stageStatus(value, "construction-support") != "pending" {
		return fmt.Errorf("reopen failed: %#v", value)
	}
	if err := client.json(ctx, http.MethodPost, stagePath(value.WorkItem.ID, "construction-drawing", "complete"), revisionPayload(commands.id(33), 18), http.StatusOK, &value); err != nil {
		return err
	}
	if err := client.json(ctx, http.MethodPost, stagePath(value.WorkItem.ID, "construction-support", "complete"), revisionPayload(commands.id(34), 19), http.StatusOK, &value); err != nil {
		return err
	}
	if value.WorkItem.Status != "completed" || value.WorkItem.Revision != 20 {
		return fmt.Errorf("work item did not re-complete after reopen: %#v", value.WorkItem)
	}

	fmt.Println("[10/10] 取消 Work Item、拒绝终态写入并核对 revision 1～21 历史")
	if err := client.json(ctx, http.MethodPost, "/api/v1/design-work-items/"+value.WorkItem.ID+"/cancel", reasonPayload(commands.id(35), 20, "独立 Demo 完成后的受控取消"), http.StatusOK, &value); err != nil {
		return err
	}
	if value.WorkItem.Status != "cancelled" || value.WorkItem.Revision != 21 || value.WorkItem.CancellationReason == nil {
		return fmt.Errorf("unexpected cancellation: %#v", value.WorkItem)
	}
	if err := client.json(ctx, http.MethodPost, ownerPath, ownerPayload(commands.id(36), 21, ids.owner), http.StatusConflict, nil); err != nil {
		return err
	}
	var history historyPage
	if err := client.json(ctx, http.MethodGet, "/api/v1/design-work-items/"+value.WorkItem.ID+"/history?limit=100", nil, http.StatusOK, &history); err != nil {
		return err
	}
	if err := verifyHistory(history, 21, ids.schemeOne, ids.schemeTwo); err != nil {
		return err
	}

	result := map[string]any{"status": "passed", "work_item_id": value.WorkItem.ID, "status_final": value.WorkItem.Status, "revision": value.WorkItem.Revision, "history_events": len(history.Items), "deliverable_replacement": true}
	encoded, _ := json.Marshal(result)
	fmt.Println(string(encoded))
	return nil
}

func (c commandSet) id(sequence int) string {
	return "aaaaaaaa-aaaa-4aaa-8aaa-" + c.nonce[:10] + fmt.Sprintf("%02x", sequence)
}

func scopedID(prefix, nonce string) string { return prefix + "-" + nonce }

func createPayload(command, projectID string) map[string]any {
	return map[string]any{"command_id": command, "project_id": projectID, "project_source_revision": int64(4)}
}

func ownerPayload(command string, revision int64, assignmentID string) map[string]any {
	return map[string]any{"command_id": command, "expected_revision": revision, "assignment_id": assignmentID, "assignment_source_revision": int64(3)}
}

func deliverablePayload(command string, revision int64, evidenceID string) map[string]any {
	return map[string]any{"command_id": command, "expected_revision": revision, "requirement_key": "primary-output", "evidence_file_id": evidenceID, "evidence_version": 1, "evidence_source_revision": int64(1)}
}

func revisionPayload(command string, revision int64) map[string]any {
	return map[string]any{"command_id": command, "expected_revision": revision}
}

func reasonPayload(command string, revision int64, reason string) map[string]any {
	value := revisionPayload(command, revision)
	value["reason"] = reason
	return value
}

func applicabilityPayload(command string, revision int64, operation, reason string) map[string]any {
	value := reasonPayload(command, revision, reason)
	value["operation"] = operation
	return value
}

func stagePath(workItemID, stage, action string) string {
	return "/api/v1/design-work-items/" + workItemID + "/stages/" + stage + "/" + action
}

func stageStatus(value details, key string) string {
	for _, item := range value.Stages {
		if item.Key == key {
			return item.Status
		}
	}
	return ""
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
		target := reflect.ValueOf(destination)
		if target.Kind() != reflect.Pointer || target.IsNil() {
			return fmt.Errorf("decode %s %s: destination must be a non-nil pointer", method, path)
		}
		target.Elem().Set(reflect.Zero(target.Elem().Type()))
		if err := json.Unmarshal(data, destination); err != nil {
			return fmt.Errorf("decode %s %s: %w (%s)", method, path, err, strings.TrimSpace(string(data)))
		}
	}
	return nil
}

func (c *demoClient) assertRevision(ctx context.Context, id string, revision int64) error {
	var value details
	if err := c.json(ctx, http.MethodGet, "/api/v1/design-work-items/"+id, nil, http.StatusOK, &value); err != nil {
		return err
	}
	if value.WorkItem.Revision != revision {
		return fmt.Errorf("revision=%d, want %d", value.WorkItem.Revision, revision)
	}
	return nil
}

func verifyHistory(value historyPage, finalRevision int64, oldEvidence, newEvidence string) error {
	if len(value.Items) != int(finalRevision) {
		return fmt.Errorf("history has %d events, want %d", len(value.Items), finalRevision)
	}
	seen := map[int64]bool{}
	evidenceIDs := map[string]bool{}
	for _, item := range value.Items {
		if item.ID == "" || item.CommandID == "" || item.EventType == "" || item.ActorSubject == "" || item.ActorSubject == "forged-actor" {
			return fmt.Errorf("invalid history event: %#v", item)
		}
		seen[item.WorkItemRevision] = true
		if item.EventType == "deliverable_registered" {
			evidenceIDs[fmt.Sprint(item.Payload["evidence_file_id"])] = true
		}
	}
	for revision := int64(1); revision <= finalRevision; revision++ {
		if !seen[revision] {
			return fmt.Errorf("history is missing work item revision %d", revision)
		}
	}
	if !evidenceIDs[oldEvidence] || !evidenceIDs[newEvidence] {
		return fmt.Errorf("history lost scheme replacement: old=%t new=%t", evidenceIDs[oldEvidence], evidenceIDs[newEvidence])
	}
	return nil
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
