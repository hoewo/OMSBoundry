package main

import (
	"context"
	"fmt"
	"net/http"
)

type projectTraceabilityView struct {
	Project     project `json:"project"`
	Assignments []struct {
		ID              string `json:"id"`
		SubjectKind     string `json:"subject_kind"`
		SourceRevision  int64  `json:"source_revision"`
		DisplaySnapshot string `json:"display_snapshot"`
		RoleKey         string `json:"role_key"`
	} `json:"assignments"`
	ResourceLinks []struct {
		ID             string `json:"id"`
		TargetService  string `json:"target_service"`
		ResourceType   string `json:"resource_type"`
		SourceRevision *int64 `json:"source_revision"`
	} `json:"resource_links"`
	ReferenceStatuses []struct {
		LinkID string `json:"link_id"`
		Status string `json:"status"`
	} `json:"reference_statuses"`
}

type projectHistoryPage struct {
	Items []struct {
		ProjectRevision int64  `json:"project_revision"`
		EventType       string `json:"event_type"`
		CommandID       string `json:"command_id"`
	} `json:"items"`
}

func verifyProjectTraceability(ctx context.Context, c *client, cfg config, projectID string, expectedProjectRevision int64, expectedAssignmentRevisions, expectedSourceRevisions map[string]int64) error {
	var details projectTraceabilityView
	if err := c.json(ctx, http.MethodGet, cfg.projectURL+"/api/v1/projects/"+projectID, "", nil, http.StatusOK, &details); err != nil {
		return err
	}
	if details.Project.Status != "active" || details.Project.Revision != expectedProjectRevision || len(details.Assignments) != 2 || len(details.ResourceLinks) != len(expectedSourceRevisions) || len(details.ReferenceStatuses) != len(expectedSourceRevisions) {
		return fmt.Errorf("Project traceability shape mismatch: project=%s/r%d assignments=%d links=%d statuses=%d", details.Project.Status, details.Project.Revision, len(details.Assignments), len(details.ResourceLinks), len(details.ReferenceStatuses))
	}
	assignmentRevisions := map[string]int64{}
	for _, item := range details.Assignments {
		if item.DisplaySnapshot == "" {
			return fmt.Errorf("Project assignment %s has no immutable display snapshot", item.ID)
		}
		assignmentRevisions[item.RoleKey] = item.SourceRevision
	}
	if assignmentRevisions["business_owner"] != expectedAssignmentRevisions["business_owner"] || assignmentRevisions["production_owner"] != expectedAssignmentRevisions["production_owner"] {
		return fmt.Errorf("Project assignments were silently rewritten after People changes: %+v", assignmentRevisions)
	}
	services := make(map[string]string, len(details.ResourceLinks))
	for _, link := range details.ResourceLinks {
		key := link.TargetService
		if link.TargetService == "money-flow-ledger" {
			key += "/" + link.ResourceType
		}
		if link.SourceRevision == nil || *link.SourceRevision != expectedSourceRevisions[key] {
			return fmt.Errorf("Project link %s did not preserve source revision: service=%s type=%s revision=%v", link.ID, link.TargetService, link.ResourceType, link.SourceRevision)
		}
		services[link.ID] = key
	}
	for _, status := range details.ReferenceStatuses {
		if services[status.LinkID] == "" || status.Status != "available" {
			return fmt.Errorf("Project reference was not available: service=%s status=%s", services[status.LinkID], status.Status)
		}
	}
	var history projectHistoryPage
	if err := c.json(ctx, http.MethodGet, cfg.projectURL+"/api/v1/projects/"+projectID+"/history?limit=100", "", nil, http.StatusOK, &history); err != nil {
		return err
	}
	if int64(len(history.Items)) != expectedProjectRevision {
		return fmt.Errorf("Project history has %d events, want %d", len(history.Items), expectedProjectRevision)
	}
	seen := map[int64]bool{}
	for _, event := range history.Items {
		seen[event.ProjectRevision] = true
		if event.CommandID == "" || event.EventType == "" {
			return fmt.Errorf("Project history event is incomplete: %+v", event)
		}
	}
	for revision := int64(1); revision <= expectedProjectRevision; revision++ {
		if !seen[revision] {
			return fmt.Errorf("Project history is missing revision %d", revision)
		}
	}
	return nil
}
