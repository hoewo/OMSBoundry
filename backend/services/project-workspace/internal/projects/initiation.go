package projects

import (
	"context"
	"fmt"
	"strings"
)

// Dispatch records the initial project and its responsible person atomically.
// Formal number and project types are supplied later when initiation completes.
type InitiationDispatchInput struct {
	CommandID     string                   `json:"command_id"`
	NodeType      string                   `json:"node_type"`
	Name          string                   `json:"name"`
	BusinessOwner InitiationOwnerReference `json:"business_owner"`
	ownerSnapshot string
}

type InitiationOwnerReference struct {
	PersonID       string `json:"person_id"`
	SourceRevision int64  `json:"source_revision"`
}

func NormalizeInitiationDispatch(input InitiationDispatchInput) (InitiationDispatchInput, error) {
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	input.NodeType = strings.ToLower(strings.TrimSpace(input.NodeType))
	input.Name = strings.TrimSpace(input.Name)
	input.BusinessOwner.PersonID = strings.ToLower(strings.TrimSpace(input.BusinessOwner.PersonID))
	if !ValidUUID(input.CommandID) || !ValidUUID(input.BusinessOwner.PersonID) || input.BusinessOwner.SourceRevision < 1 {
		return input, fmt.Errorf("command id and a versioned business owner are required")
	}
	if input.NodeType != NodeTypeProject && input.NodeType != NodeTypePackage {
		return input, fmt.Errorf("node type must be project or package")
	}
	if input.Name == "" || len([]rune(input.Name)) > 200 {
		return input, fmt.Errorf("project name must contain 1 to 200 characters")
	}
	return input, nil
}

func (s *Service) DispatchInitiation(ctx context.Context, actor Actor, input InitiationDispatchInput) (Project, error) {
	input, err := NormalizeInitiationDispatch(input)
	if err != nil {
		return Project{}, err
	}
	if replay, found, err := s.store.ReplayInitiationDispatch(ctx, actor.TenantID, input); err != nil {
		return Project{}, err
	} else if found {
		return replay, nil
	}
	snapshot, err := s.references.ValidatePeople(ctx, actor.TenantID, "person", input.BusinessOwner.PersonID, input.BusinessOwner.SourceRevision)
	if err != nil {
		return Project{}, err
	}
	input.ownerSnapshot = snapshot.DisplayName
	return s.store.DispatchInitiation(ctx, actor, input)
}

// An omitted profile preserves the original command hash and dual-owner gate.
// These are versioned readiness rules, not permission grants or workflow scripts.
func normalizeInitiationProfile(profile string) (string, error) {
	profile = strings.ToLower(strings.TrimSpace(profile))
	switch profile {
	case "", InitiationProfileDualOwnerV1, InitiationProfileBusinessOwnerV1:
		return profile, nil
	default:
		return "", fmt.Errorf("unsupported initiation profile")
	}
}

func initiationOwnerRoles(profile string) []string {
	if profile == InitiationProfileBusinessOwnerV1 {
		return []string{RoleBusinessOwner}
	}
	return []string{RoleBusinessOwner, RoleProductionOwner}
}

func initiationMissing(project Project, profile string, owners map[string]bool) []string {
	missing := make([]string, 0)
	numberOptional := profile == InitiationProfileBusinessOwnerV1 && project.NodeType == NodeTypePackage
	if !numberOptional && strings.TrimSpace(project.ProjectNumber) == "" {
		missing = append(missing, "project_number")
	}
	if strings.TrimSpace(project.Name) == "" {
		missing = append(missing, "name")
	}
	if len(project.ProjectTypeKeys) == 0 {
		missing = append(missing, "project_type")
	}
	for _, role := range initiationOwnerRoles(profile) {
		if !owners[role] {
			missing = append(missing, role)
		}
	}
	return missing
}
