package projects

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

type BasicInfoFields struct {
	ProjectNumber    string     `json:"project_number"`
	Name             string     `json:"name"`
	Summary          string     `json:"summary"`
	PlannedStartDate *time.Time `json:"planned_start_date"`
	PlannedEndDate   *time.Time `json:"planned_end_date"`
	ProjectTypeKeys  []string   `json:"project_type_keys"`
}

func (f BasicInfoFields) createInput(command string) CreateInput {
	return CreateInput{CommandID: command, ProjectNumber: f.ProjectNumber, Name: f.Name, Summary: f.Summary, PlannedStartDate: f.PlannedStartDate, PlannedEndDate: f.PlannedEndDate, ProjectTypeKeys: f.ProjectTypeKeys}
}
func basicInfoFields(f CreateInput) BasicInfoFields {
	return BasicInfoFields{ProjectNumber: f.ProjectNumber, Name: f.Name, Summary: f.Summary, PlannedStartDate: f.PlannedStartDate, PlannedEndDate: f.PlannedEndDate, ProjectTypeKeys: f.ProjectTypeKeys}
}

type OwnerChange struct {
	RoleKey   string                    `json:"role_key"`
	Operation string                    `json:"operation"`
	Person    *InitiationOwnerReference `json:"person,omitempty"`
	snapshot  string
}
type BasicInfoChangeInput struct {
	CommandID        string           `json:"command_id"`
	ExpectedRevision int64            `json:"expected_revision"`
	Reason           string           `json:"reason"`
	Profile          *BasicInfoFields `json:"profile,omitempty"`
	Owners           []OwnerChange    `json:"owners,omitempty"`
}
type OwnerChangeResult struct {
	RoleKey  string      `json:"role_key"`
	Changed  bool        `json:"changed"`
	Previous *Assignment `json:"previous,omitempty"`
	Current  *Assignment `json:"current,omitempty"`
}
type BasicInfoChangeResult struct {
	Project Project             `json:"project"`
	Owners  []OwnerChangeResult `json:"owners"`
}

func NormalizeBasicInfoChange(in BasicInfoChangeInput) (BasicInfoChangeInput, error) {
	in.CommandID = strings.ToLower(strings.TrimSpace(in.CommandID))
	in.Reason = strings.TrimSpace(in.Reason)
	if !ValidUUID(in.CommandID) || in.ExpectedRevision < 1 || in.Reason == "" || len([]rune(in.Reason)) > 500 || len(in.Owners) > 2 || (in.Profile == nil && len(in.Owners) == 0) {
		return in, fmt.Errorf("basic info change requires command, revision, reason and explicit changes")
	}
	in.Owners = append([]OwnerChange(nil), in.Owners...)
	seen := map[string]bool{}
	for i := range in.Owners {
		v := &in.Owners[i]
		v.RoleKey = strings.ToLower(strings.TrimSpace(v.RoleKey))
		v.Operation = strings.ToLower(strings.TrimSpace(v.Operation))
		if (v.RoleKey != RoleBusinessOwner && v.RoleKey != RoleProductionOwner) || seen[v.RoleKey] {
			return in, fmt.Errorf("only two distinct owner roles may be changed")
		}
		seen[v.RoleKey] = true
		switch v.Operation {
		case "set":
			if v.Person == nil {
				return in, fmt.Errorf("set owner requires versioned person")
			}
			person := *v.Person
			person.PersonID = strings.ToLower(strings.TrimSpace(person.PersonID))
			if !ValidUUID(person.PersonID) || person.SourceRevision < 1 {
				return in, fmt.Errorf("invalid owner person reference")
			}
			v.Person = &person
		case "clear":
			if v.Person != nil {
				return in, fmt.Errorf("clear owner cannot contain a person")
			}
		default:
			return in, fmt.Errorf("owner operation must explicitly be set or clear")
		}
	}
	sort.Slice(in.Owners, func(i, j int) bool { return in.Owners[i].RoleKey < in.Owners[j].RoleKey })
	return in, nil
}
func basicInfoChangeHash(id string, in BasicInfoChangeInput) (string, error) {
	return ownerInfoChangeHash(id, in, "project-basic-info-change-v1")
}
func ownerInfoChangeHash(id string, in BasicInfoChangeInput, kind string) (string, error) {
	return requestHash(struct {
		Kind, ID string
		Input    BasicInfoChangeInput
	}{kind, id, in})
}
func (s *Service) ChangeBasicInfo(ctx context.Context, actor Actor, id string, input BasicInfoChangeInput) (BasicInfoChangeResult, error) {
	return s.changeOwnerInfo(ctx, actor, id, input, false)
}
func (s *Service) ChangeInitiationInfo(ctx context.Context, actor Actor, id string, input BasicInfoChangeInput) (BasicInfoChangeResult, error) {
	return s.changeOwnerInfo(ctx, actor, id, input, true)
}
func (s *Service) changeOwnerInfo(ctx context.Context, actor Actor, id string, input BasicInfoChangeInput, initiation bool) (BasicInfoChangeResult, error) {
	in, err := NormalizeBasicInfoChange(input)
	if err != nil {
		return BasicInfoChangeResult{}, err
	}
	if !ValidUUID(id) {
		return BasicInfoChangeResult{}, fmt.Errorf("invalid project id")
	}
	if in.Profile != nil {
		profile, err := s.normalizeProfile(ctx, actor.TenantID, id, UpdateInput{CreateInput: in.Profile.createInput(in.CommandID), ExpectedRevision: in.ExpectedRevision})
		if err != nil {
			return BasicInfoChangeResult{}, err
		}
		fields := basicInfoFields(profile.CreateInput)
		in.Profile = &fields
	}
	replay := s.store.ReplayBasicInfoChange
	if initiation {
		replay = s.store.ReplayInitiationInfoChange
	}
	if result, found, err := replay(ctx, actor.TenantID, id, in); err != nil {
		return result, err
	} else if found {
		return result, nil
	}
	for i := range in.Owners {
		if in.Owners[i].Operation == "set" {
			v := &in.Owners[i]
			snapshot, err := s.references.ValidatePeople(ctx, actor.TenantID, "person", v.Person.PersonID, v.Person.SourceRevision)
			if err != nil {
				return BasicInfoChangeResult{}, err
			}
			v.snapshot = snapshot.DisplayName
		}
	}
	if initiation {
		return s.store.ChangeInitiationInfo(ctx, actor, id, in)
	}
	return s.store.ChangeBasicInfo(ctx, actor, id, in)
}
