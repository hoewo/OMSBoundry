package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

type productionReference struct {
	WorkItemID     string   `json:"work_item_id"`
	TypeKey        string   `json:"type_key"`
	ResourceType   string   `json:"resource_type"`
	Status         string   `json:"status"`
	Revision       int64    `json:"revision"`
	CompletionGaps []string `json:"completion_gaps"`
}

type workItemList struct {
	Items []workItem `json:"items"`
}

func runConsultationDelivery(ctx context.Context) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	c := &client{http: &http.Client{Timeout: 10 * time.Second}, token: cfg.token}
	current, err := readState(cfg.statePath)
	if err != nil {
		return err
	}

	fmt.Println("[C4-A11] 仅用 People、Project、Evidence 与 Production 完成咨询交付")
	if err := c.json(ctx, http.MethodPost, cfg.peopleURL+"/api/v1/definitions/group_type", cmdConsultGroupType, map[string]any{
		"command_id": cmdConsultGroupType, "key": "consulting-team", "display_name": "咨询生产组", "description": "咨询报告交付的生产团队",
	}, http.StatusCreated, nil); err != nil {
		return err
	}
	var owner person
	if err := c.json(ctx, http.MethodPost, cfg.peopleURL+"/api/v1/persons", cmdConsultPerson, map[string]any{
		"command_id": cmdConsultPerson, "reference_code": "OA-CONSULT-001", "display_name": "顾远",
	}, http.StatusCreated, &owner); err != nil {
		return err
	}
	var team group
	if err := c.json(ctx, http.MethodPost, cfg.peopleURL+"/api/v1/groups", cmdConsultGroup, map[string]any{
		"command_id": cmdConsultGroup, "group_type_key": "consulting-team", "name": "青岚咨询交付组", "purpose": "独立完成咨询报告交付",
	}, http.StatusCreated, &team); err != nil {
		return err
	}

	var p project
	if err := c.json(ctx, http.MethodPost, cfg.projectURL+"/api/v1/projects", "", map[string]any{
		"command_id": cmdConsultProject, "project_number": "OA-CONSULT-2026-001", "name": "青岚运营咨询", "summary": "不选择 design 的咨询型最小组合", "project_type_keys": []string{"consultation"},
	}, http.StatusCreated, &p); err != nil {
		return err
	}
	var business assignmentResult
	if err := c.json(ctx, http.MethodPost, cfg.projectURL+"/api/v1/projects/"+p.ID+"/assignments", "", map[string]any{
		"command_id": cmdConsultBusiness, "expected_revision": int64(1), "operation": "assign", "subject_kind": "person", "subject_id": owner.ID, "source_revision": owner.Revision, "role_key": "business_owner",
	}, http.StatusOK, &business); err != nil {
		return err
	}
	var production assignmentResult
	if err := c.json(ctx, http.MethodPost, cfg.projectURL+"/api/v1/projects/"+p.ID+"/assignments", "", map[string]any{
		"command_id": cmdConsultProduction, "expected_revision": int64(2), "operation": "assign", "subject_kind": "group", "subject_id": team.ID, "source_revision": team.Revision, "role_key": "production_owner",
	}, http.StatusOK, &production); err != nil {
		return err
	}
	if err := c.json(ctx, http.MethodPost, cfg.projectURL+"/api/v1/projects/"+p.ID+"/complete-initiation", "", map[string]any{
		"command_id": cmdConsultInitiation, "expected_revision": int64(3),
	}, http.StatusOK, &p); err != nil {
		return err
	}
	if p.Status != "active" || p.Revision != 4 {
		return fmt.Errorf("consultation-only project did not reach active revision 4: status=%s revision=%d", p.Status, p.Revision)
	}
	if current.ConsultationProjectID != "" && current.ConsultationProjectID != p.ID {
		return fmt.Errorf("consultation state project %s does not match idempotent project %s", current.ConsultationProjectID, p.ID)
	}
	current.ConsultationProjectID = p.ID
	if err := writeState(cfg.statePath, current); err != nil {
		return err
	}

	report, err := c.ensureNamedEvidenceForContext(ctx, cfg, current.ConsultationEvidenceFileID, current.ConsultationEvidenceVersion, "qinglan-consultation-report.txt", "青岚运营咨询——正式咨询报告\n", "oa-consultation-report", "OA-CONSULT-2026-001", "consultation-report")
	if err != nil {
		return err
	}
	current.ConsultationEvidenceFileID, current.ConsultationEvidenceVersion = report.FileID, report.Version
	if err := writeState(cfg.statePath, current); err != nil {
		return err
	}

	var details designDetails
	if err := c.json(ctx, http.MethodPost, cfg.designURL+"/api/v1/production-work-items", "", map[string]any{
		"command_id": cmdConsultWorkItem, "project_id": p.ID, "project_source_revision": p.Revision, "type_key": "consultation-v1",
	}, http.StatusCreated, &details); err != nil {
		return err
	}
	if details.WorkItem.TypeKey != "consultation-v1" || details.WorkItem.Revision != 1 {
		return fmt.Errorf("unexpected consultation work item: %+v", details.WorkItem)
	}
	if err := c.json(ctx, http.MethodPost, cfg.designURL+"/api/v1/production-work-items/"+details.WorkItem.ID+"/owner", "", map[string]any{
		"command_id": cmdConsultOwner, "expected_revision": int64(1), "assignment_id": production.Assignment.ID, "assignment_source_revision": production.Assignment.CreatedProjectRevision,
	}, http.StatusOK, &details); err != nil {
		return err
	}
	if err := c.json(ctx, http.MethodPost, cfg.designURL+"/api/v1/production-work-items/"+details.WorkItem.ID+"/stages/scheme/applicability", "", map[string]any{
		"command_id": cmdConsultSkipScheme, "expected_revision": int64(2), "operation": "set-not-applicable", "reason": "本咨询任务直接形成正式报告",
	}, http.StatusOK, &details); err != nil {
		return err
	}
	var deliverable json.RawMessage
	if err := c.json(ctx, http.MethodPost, cfg.designURL+"/api/v1/production-work-items/"+details.WorkItem.ID+"/stages/report/deliverables", "", map[string]any{
		"command_id": cmdConsultReport, "expected_revision": int64(3), "requirement_key": "primary-output", "evidence_file_id": report.FileID, "evidence_version": report.Version, "evidence_source_revision": report.Revision,
	}, http.StatusOK, &deliverable); err != nil {
		return err
	}
	if err := c.json(ctx, http.MethodPost, cfg.designURL+"/api/v1/production-work-items/"+details.WorkItem.ID+"/stages/report/complete", "", map[string]any{
		"command_id": cmdConsultComplete, "expected_revision": int64(4),
	}, http.StatusOK, &details); err != nil {
		return err
	}
	if details.WorkItem.Status != "completed" || details.WorkItem.Revision != 5 {
		return fmt.Errorf("consultation work item did not complete at revision 5: %+v", details.WorkItem)
	}
	if current.ConsultationWorkItemID != "" && current.ConsultationWorkItemID != details.WorkItem.ID {
		return fmt.Errorf("consultation state work item %s does not match idempotent work item %s", current.ConsultationWorkItemID, details.WorkItem.ID)
	}
	current.ConsultationWorkItemID = details.WorkItem.ID

	var reference productionReference
	if err := c.json(ctx, http.MethodGet, cfg.designURL+"/api/v1/references/production-work-items/"+details.WorkItem.ID, "", nil, http.StatusOK, &reference); err != nil {
		return err
	}
	if reference.WorkItemID != details.WorkItem.ID || reference.TypeKey != "consultation-v1" || reference.ResourceType != "production-work-item" || reference.Status != "completed" || reference.Revision != 5 || len(reference.CompletionGaps) != 0 {
		return fmt.Errorf("canonical consultation reference is incomplete: %+v", reference)
	}
	var history designHistoryPage
	if err := c.json(ctx, http.MethodGet, cfg.designURL+"/api/v1/production-work-items/"+details.WorkItem.ID+"/history?limit=100", "", nil, http.StatusOK, &history); err != nil {
		return err
	}
	if len(history.Items) != 5 {
		return fmt.Errorf("consultation history has %d events, want 5", len(history.Items))
	}
	if err := c.json(ctx, http.MethodGet, cfg.designURL+"/api/v1/design-work-items/"+details.WorkItem.ID, "", nil, http.StatusNotFound, nil); err != nil {
		return err
	}
	var canonicalList workItemList
	if err := c.json(ctx, http.MethodGet, cfg.designURL+"/api/v1/production-work-items", "", nil, http.StatusOK, &canonicalList); err != nil {
		return err
	}
	if len(canonicalList.Items) != 1 || canonicalList.Items[0].ID != details.WorkItem.ID || canonicalList.Items[0].TypeKey != "consultation-v1" {
		return fmt.Errorf("canonical production list is not consultation-only: %+v", canonicalList.Items)
	}
	var legacyList workItemList
	if err := c.json(ctx, http.MethodGet, cfg.designURL+"/api/v1/design-work-items", "", nil, http.StatusOK, &legacyList); err != nil {
		return err
	}
	if len(legacyList.Items) != 0 {
		return fmt.Errorf("legacy design list exposed consultation data: %+v", legacyList.Items)
	}
	if business.Project.Revision != 2 || production.Project.Revision != 3 {
		return errors.New("project assignment revisions are not stable")
	}
	if err := writeState(cfg.statePath, current); err != nil {
		return err
	}
	fmt.Printf("  project=%s consultation-only/r%d; report=%s/v%d; work-item=%s/%s/r%d\n", p.ID, p.Revision, report.FileID, report.Version, details.WorkItem.ID, details.WorkItem.TypeKey, details.WorkItem.Revision)
	fmt.Println("PASS: C4-A11 consultation-only minimum business composition")
	return nil
}
