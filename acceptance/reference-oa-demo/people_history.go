package main

import (
	"context"
	"fmt"
	"net/http"
)

const oaOwnerIAMSubject = "10000000-0000-4000-8000-100000000001"

type peopleHistoryResult struct {
	FirstSnapshot  snapshotSummary
	SecondSnapshot snapshotSummary
	ActiveMembers  int
}

// runJ1History deliberately runs after Project has captured the initial person and
// group revisions. Later People changes must not rewrite those Project snapshots.
func runJ1History(ctx context.Context, c *client, cfg config, owner person, team group, ownerMembership membership) (peopleHistoryResult, error) {
	var second person
	if err := c.json(ctx, http.MethodPost, cfg.peopleURL+"/api/v1/persons", cmdSecondPerson, map[string]any{
		"command_id": cmdSecondPerson, "reference_code": "OA-DEMO-002", "display_name": "周予安",
	}, http.StatusCreated, &second); err != nil {
		return peopleHistoryResult{}, err
	}
	var secondMembership membership
	if err := c.json(ctx, http.MethodPost, cfg.peopleURL+"/api/v1/groups/"+team.ID+"/memberships", cmdSecondMembership, map[string]any{
		"command_id": cmdSecondMembership, "expected_revision": int64(2), "person_id": second.ID, "role_key": "designer", "start_date": "2026-09-25",
	}, http.StatusCreated, &secondMembership); err != nil {
		return peopleHistoryResult{}, err
	}
	var assigned capability
	if err := c.json(ctx, http.MethodPost, cfg.peopleURL+"/api/v1/groups/"+team.ID+"/capabilities", cmdCapability, map[string]any{
		"command_id": cmdCapability, "expected_revision": int64(3), "capability_key": "design_delivery", "note": "承担办公空间方案与施工图交付", "start_date": "2026-09-25",
	}, http.StatusCreated, &assigned); err != nil {
		return peopleHistoryResult{}, err
	}
	var first snapshotSummary
	if err := c.json(ctx, http.MethodPost, cfg.peopleURL+"/api/v1/groups/"+team.ID+"/snapshots", cmdRosterSnapshotOne, map[string]any{
		"command_id": cmdRosterSnapshotOne, "expected_revision": int64(4),
	}, http.StatusCreated, &first); err != nil {
		return peopleHistoryResult{}, err
	}
	if first.GroupRevision != 4 || first.SHA256 == "" {
		return peopleHistoryResult{}, fmt.Errorf("first People snapshot is invalid: %+v", first)
	}
	if err := c.json(ctx, http.MethodPost, cfg.peopleURL+"/api/v1/persons/"+owner.ID+"/profile", cmdOwnerProfile, map[string]any{
		"command_id": cmdOwnerProfile, "expected_revision": int64(1), "reference_code": "OA-DEMO-001", "display_name": "林澄（设计负责人）",
	}, http.StatusOK, &owner); err != nil {
		return peopleHistoryResult{}, err
	}
	if err := c.json(ctx, http.MethodPost, cfg.peopleURL+"/api/v1/persons/"+owner.ID+"/iam-subject", cmdOwnerIAMSubject, map[string]any{
		"command_id": cmdOwnerIAMSubject, "expected_revision": int64(2), "iam_subject_id": oaOwnerIAMSubject,
	}, http.StatusOK, &owner); err != nil {
		return peopleHistoryResult{}, err
	}
	var promoted membership
	if err := c.json(ctx, http.MethodPost, cfg.peopleURL+"/api/v1/groups/"+team.ID+"/memberships/"+ownerMembership.ID+"/role", cmdOwnerRole, map[string]any{
		"command_id": cmdOwnerRole, "expected_group_revision": int64(6), "expected_revision": int64(1), "role_key": "lead_designer",
	}, http.StatusOK, &promoted); err != nil {
		return peopleHistoryResult{}, err
	}
	var ended membership
	if err := c.json(ctx, http.MethodPost, cfg.peopleURL+"/api/v1/groups/"+team.ID+"/memberships/"+secondMembership.ID+"/end", cmdEndSecondMember, map[string]any{
		"command_id": cmdEndSecondMember, "expected_group_revision": int64(7), "expected_revision": int64(1), "end_date": "2026-10-15",
	}, http.StatusOK, &ended); err != nil {
		return peopleHistoryResult{}, err
	}
	if err := c.json(ctx, http.MethodPost, cfg.peopleURL+"/api/v1/persons/"+second.ID+"/status", cmdDeactivateSecond, map[string]any{
		"command_id": cmdDeactivateSecond, "expected_revision": int64(1), "status": "inactive",
	}, http.StatusOK, &second); err != nil {
		return peopleHistoryResult{}, err
	}
	var secondSnapshot snapshotSummary
	if err := c.json(ctx, http.MethodPost, cfg.peopleURL+"/api/v1/groups/"+team.ID+"/snapshots", cmdRosterSnapshotTwo, map[string]any{
		"command_id": cmdRosterSnapshotTwo, "expected_revision": int64(8),
	}, http.StatusCreated, &secondSnapshot); err != nil {
		return peopleHistoryResult{}, err
	}
	if secondSnapshot.GroupRevision != 8 || secondSnapshot.SHA256 == "" || secondSnapshot.SHA256 == first.SHA256 {
		return peopleHistoryResult{}, fmt.Errorf("second People snapshot did not preserve a distinct revision: first=%+v second=%+v", first, secondSnapshot)
	}
	var details groupDetails
	if err := c.json(ctx, http.MethodGet, cfg.peopleURL+"/api/v1/groups/"+team.ID, "", nil, http.StatusOK, &details); err != nil {
		return peopleHistoryResult{}, err
	}
	active := 0
	for _, item := range details.Memberships {
		if item.Status == "active" {
			active++
		}
	}
	if details.Group.Status != "active" || details.Group.Revision != 8 || active != 1 || len(details.Snapshots) != 2 || owner.Revision != 3 || owner.IAMSubjectID != oaOwnerIAMSubject || promoted.RoleKey != "lead_designer" || ended.Status != "ended" || second.Status != "inactive" || assigned.ID == "" || assigned.Status != "active" {
		return peopleHistoryResult{}, fmt.Errorf("People lifecycle mismatch: group=%+v owner=%+v second=%+v promoted=%+v ended=%+v snapshots=%d active=%d", details.Group, owner, second, promoted, ended, len(details.Snapshots), active)
	}
	return peopleHistoryResult{FirstSnapshot: first, SecondSnapshot: secondSnapshot, ActiveMembers: active}, nil
}
