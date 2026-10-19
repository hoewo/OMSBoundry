package main

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type projectFailureView struct {
	Project     project `json:"project"`
	Assignments []struct {
		RoleKey         string `json:"role_key"`
		SourceRevision  int64  `json:"source_revision"`
		DisplaySnapshot string `json:"display_snapshot"`
	} `json:"assignments"`
	ResourceLinks []struct {
		ID            string `json:"id"`
		TargetService string `json:"target_service"`
	} `json:"resource_links"`
	ReferenceStatuses []struct {
		LinkID string `json:"link_id"`
		Status string `json:"status"`
	} `json:"reference_statuses"`
}

func runFailureIsolation(ctx context.Context, unavailableService string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	current, err := readState(cfg.statePath)
	if err != nil {
		return err
	}
	if current.ProjectID == "" {
		return fmt.Errorf("demo state has no project_id; run the demo first")
	}
	c := &client{http: &http.Client{Timeout: 10 * time.Second}, token: cfg.token}
	endpoints := map[string]string{
		"contract-ledger":   cfg.contractURL,
		"money-flow-ledger": cfg.moneyURL,
		"people-groups":     cfg.peopleURL,
		"evidence-service":  cfg.evidenceURL,
		"project-workspace": cfg.projectURL,
		"design-operations": cfg.designURL,
	}
	expectedUnavailable := map[string]int{"contract-ledger": 1, "money-flow-ledger": 2, "people-groups": 0, "evidence-service": 1, "design-operations": 1}
	wantUnavailable, known := expectedUnavailable[unavailableService]
	if !known {
		return fmt.Errorf("unsupported unavailable service %q", unavailableService)
	}
	for name, endpoint := range endpoints {
		if name == unavailableService {
			continue
		}
		if err := c.json(ctx, http.MethodGet, endpoint+"/health/ready", "", nil, http.StatusOK, nil); err != nil {
			return fmt.Errorf("%s was not ready while %s was unavailable: %w", name, unavailableService, err)
		}
	}
	var details projectFailureView
	if err := c.json(ctx, http.MethodGet, cfg.projectURL+"/api/v1/projects/"+current.ProjectID, "", nil, http.StatusOK, &details); err != nil {
		return err
	}
	services := make(map[string]string, len(details.ResourceLinks))
	for _, link := range details.ResourceLinks {
		services[link.ID] = link.TargetService
	}
	available, unavailable := 0, 0
	for _, status := range details.ReferenceStatuses {
		switch {
		case services[status.LinkID] == unavailableService && status.Status == "unavailable":
			unavailable++
		case services[status.LinkID] != unavailableService && status.Status == "available":
			available++
		default:
			return fmt.Errorf("unexpected reference state: service=%s status=%s", services[status.LinkID], status.Status)
		}
	}
	if details.Project.Status != "active" || details.Project.Revision != 9 || unavailable != wantUnavailable || available != 5-wantUnavailable || len(details.Assignments) != 2 {
		return fmt.Errorf("failure isolation mismatch: project=%s/r%d available=%d unavailable=%d", details.Project.Status, details.Project.Revision, available, unavailable)
	}
	for _, assignment := range details.Assignments {
		if assignment.DisplaySnapshot == "" {
			return fmt.Errorf("assignment %s lost its immutable snapshot while %s was unavailable", assignment.RoleKey, unavailableService)
		}
	}
	fmt.Printf("PASS: Project remained active at revision %d while %s was unavailable; references available=%d unavailable=%d; assignments preserved=%d\n", details.Project.Revision, unavailableService, available, unavailable, len(details.Assignments))
	return nil
}
