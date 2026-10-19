// Package productionpersonnel is the closed reference OA customer extension.
// It is not the Base Assignment role catalogue, an HTTP port or a repository.
// A caller must authenticate, resolve sources and commit the delta atomically.
package productionpersonnel

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const Profile = "reference-oa-production-personnel-v1"
const (
	ReplaceDiscipline = "replace_discipline"
	ReplaceReviewers  = "replace_reviewers"
	EndDiscipline     = "end_discipline"
	Replace           = "replace"
	Preserve          = "preserve"
	End               = "end"
)

var (
	ErrInvalid        = errors.New("invalid OA personnel intention or snapshot")
	ErrStale          = errors.New("OA personnel project revision changed")
	ErrState          = errors.New("OA personnel operation is unavailable in this state")
	ErrSource         = errors.New("OA personnel source is missing, inactive or inconsistent")
	ErrTargetOccupied = errors.New("target discipline already has personnel")
)

var uuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
var professionalRoles = []string{"designer", "participant", "reviewer", "auditor"}
var globalRoles = []string{"approver", "estimator"}

type Ref struct {
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
}
type PersonSnapshot struct {
	Ref
	Name string `json:"name"`
}
type DisciplineSnapshot struct {
	Ref
	Name string `json:"name"`
}

// Assignment here is a real extension fact, never a display alias of a Base
// participant/owner. Active rows only are supplied to the planner; an ending
// delta does not delete or rewrite the persisted historical fact.
type Assignment struct {
	ID                     string              `json:"id"`
	Role                   string              `json:"role"`
	Person                 PersonSnapshot      `json:"person"`
	Discipline             *DisciplineSnapshot `json:"discipline,omitempty"`
	CreatedProjectRevision int64               `json:"created_project_revision"`
}
type Current struct {
	Profile                  string
	TenantID                 string
	ProjectID                string
	ProjectRevision          int64
	ProjectStatus            string
	ParticipantNotApplicable bool
	Active                   []Assignment
}
type RoleChange struct {
	Role      string `json:"role"`
	Operation string `json:"operation"`
	People    []Ref  `json:"people"`
}

// Intention is the canonical client intention. Source display data and Actor
// must NOT be added to its command hash or accepted from the client.
type Intention struct {
	Profile                 string       `json:"profile"`
	CommandID               string       `json:"command_id"`
	ExpectedProjectRevision int64        `json:"expected_project_revision"`
	Operation               string       `json:"operation"`
	SourceDisciplineID      string       `json:"source_discipline_id,omitempty"`
	TargetDiscipline        *Ref         `json:"target_discipline,omitempty"`
	Changes                 []RoleChange `json:"changes,omitempty"`
	ParticipantOperation    string       `json:"participant_operation,omitempty"`
}
type ResolvedPerson struct {
	TenantID string
	State    string
	PersonSnapshot
}
type ResolvedDiscipline struct {
	TenantID string
	State    string
	DisciplineSnapshot
}

// Resolved is supplied by trusted versioned source resolvers, not JSON input.
// Missing/stale/inactive references cannot become zero-valued new facts.
type Resolved struct {
	People     map[Ref]ResolvedPerson
	Discipline *ResolvedDiscipline
}
type NewAssignment struct {
	Role       string              `json:"role"`
	Person     PersonSnapshot      `json:"person"`
	Discipline *DisciplineSnapshot `json:"discipline,omitempty"`
}
type Delta struct {
	EndIDs []string        `json:"end_ids"`
	Add    []NewAssignment `json:"add"`
}

func Normalize(in Intention) (Intention, error) {
	in.CommandID = strings.ToLower(strings.TrimSpace(in.CommandID))
	in.Operation = strings.ToLower(strings.TrimSpace(in.Operation))
	in.SourceDisciplineID = strings.ToLower(strings.TrimSpace(in.SourceDisciplineID))
	if in.Profile != Profile || !uuid.MatchString(in.CommandID) || in.ExpectedProjectRevision < 1 {
		return Intention{}, ErrInvalid
	}
	roles := professionalRoles
	switch in.Operation {
	case ReplaceDiscipline:
		if in.TargetDiscipline == nil || !validRef(*in.TargetDiscipline) || (in.SourceDisciplineID != "" && !uuid.MatchString(in.SourceDisciplineID)) || in.ParticipantOperation != "" {
			return Intention{}, ErrInvalid
		}
		ref := *in.TargetDiscipline
		in.TargetDiscipline = &ref
	case ReplaceReviewers:
		roles = globalRoles
		if in.TargetDiscipline != nil || in.SourceDisciplineID != "" || in.ParticipantOperation != "" {
			return Intention{}, ErrInvalid
		}
	case EndDiscipline:
		if !uuid.MatchString(in.SourceDisciplineID) || in.TargetDiscipline != nil || len(in.Changes) != 0 || (in.ParticipantOperation != Preserve && in.ParticipantOperation != End) {
			return Intention{}, ErrInvalid
		}
		return in, nil
	default:
		return Intention{}, ErrInvalid
	}
	if len(in.Changes) != len(roles) {
		return Intention{}, ErrInvalid
	}
	in.Changes = append([]RoleChange(nil), in.Changes...)
	seen := map[string]bool{}
	for i, change := range in.Changes {
		if !contains(roles, change.Role) || seen[change.Role] {
			return Intention{}, ErrInvalid
		}
		seen[change.Role] = true
		switch change.Operation {
		case Preserve:
			if change.Role != "participant" || change.People != nil {
				return Intention{}, ErrInvalid
			}
		case Replace:
			// nil/omission is not an explicit empty replacement.
			if change.People == nil || len(change.People) > 256 || (single(change.Role) && len(change.People) > 1) {
				return Intention{}, ErrInvalid
			}
			change.People = append([]Ref{}, change.People...)
			people := map[string]bool{}
			for _, person := range change.People {
				if !validRef(person) || people[person.ID] {
					return Intention{}, ErrInvalid
				}
				people[person.ID] = true
			}
			sort.Slice(change.People, func(a, b int) bool { return change.People[a].ID < change.People[b].ID })
		default:
			return Intention{}, ErrInvalid
		}
		in.Changes[i] = change
	}
	sort.Slice(in.Changes, func(a, b int) bool { return in.Changes[a].Role < in.Changes[b].Role })
	return in, nil
}

func Prepare(current Current, input Intention, sources Resolved) (Delta, error) {
	in, err := Normalize(input)
	if err != nil {
		return Delta{}, err
	}
	if err := validateCurrent(current); err != nil {
		return Delta{}, err
	}
	if current.ProjectRevision != in.ExpectedProjectRevision {
		return Delta{}, ErrStale
	}
	if current.ProjectStatus != "draft" && current.ProjectStatus != "pending_completion" && current.ProjectStatus != "active" {
		return Delta{}, ErrState
	}
	result := Delta{EndIDs: []string{}, Add: []NewAssignment{}}
	if in.Operation == EndDiscipline {
		if current.ParticipantNotApplicable && in.ParticipantOperation != Preserve {
			return Delta{}, ErrState
		}
		for _, fact := range current.Active {
			if disciplineID(fact.Discipline) == in.SourceDisciplineID && !(fact.Role == "participant" && in.ParticipantOperation == Preserve) {
				result.EndIDs = append(result.EndIDs, fact.ID)
			}
		}
		sort.Strings(result.EndIDs)
		return result, nil
	}
	var target *DisciplineSnapshot
	sourceID := in.SourceDisciplineID
	if in.Operation == ReplaceDiscipline {
		d := sources.Discipline
		if d == nil || d.TenantID != current.TenantID || d.Ref != *in.TargetDiscipline || d.State != "active" || !validName(d.Name, 200) {
			return Delta{}, ErrSource
		}
		for _, fact := range current.Active {
			if fact.Discipline != nil && fact.Discipline.Ref == d.Ref && fact.Discipline.Name != d.Name {
				return Delta{}, ErrSource
			}
		}
		value := d.DisciplineSnapshot
		target = &value
		if sourceID == "" {
			sourceID = target.ID
		}
		if in.SourceDisciplineID != "" {
			found := false
			for _, fact := range current.Active {
				found = found || disciplineID(fact.Discipline) == sourceID
			}
			if !found {
				return Delta{}, ErrState
			}
		}
		if sourceID != target.ID {
			for _, fact := range current.Active {
				if disciplineID(fact.Discipline) == target.ID {
					return Delta{}, ErrTargetOccupied
				}
			}
		}
	}
	for _, change := range in.Changes {
		if change.Role == "participant" && current.ParticipantNotApplicable && change.Operation != Preserve {
			return Delta{}, ErrState
		}
		if change.Operation == Preserve {
			continue
		}
		desired := map[string]NewAssignment{}
		for _, ref := range change.People {
			person, found := sources.People[ref]
			if !found || person.TenantID != current.TenantID || person.Ref != ref || person.State != "active" || !validName(person.Name, 300) {
				return Delta{}, ErrSource
			}
			for _, fact := range current.Active {
				if fact.Person.Ref == ref && fact.Person.Name != person.Name {
					return Delta{}, ErrSource
				}
			}
			desired[ref.ID] = NewAssignment{Role: change.Role, Person: person.PersonSnapshot, Discipline: copyDiscipline(target)}
		}
		for _, fact := range current.Active {
			if fact.Role != change.Role || disciplineID(fact.Discipline) != sourceID {
				continue
			}
			want, found := desired[fact.Person.ID]
			if found && fact.Person == want.Person && equalDiscipline(fact.Discipline, want.Discipline) {
				delete(desired, fact.Person.ID) // retain the real original ID/snapshot
				continue
			}
			result.EndIDs = append(result.EndIDs, fact.ID)
		}
		for _, fact := range desired {
			result.Add = append(result.Add, fact)
		}
	}
	sort.Strings(result.EndIDs)
	sort.Slice(result.Add, func(a, b int) bool {
		left, right := result.Add[a], result.Add[b]
		return left.Role+"/"+left.Person.ID < right.Role+"/"+right.Person.ID
	})
	return result, nil
}

func validateCurrent(v Current) error {
	if v.Profile != Profile || !uuid.MatchString(v.TenantID) || !uuid.MatchString(v.ProjectID) || v.ProjectRevision < 1 || v.Active == nil || len(v.Active) > 4096 {
		return ErrInvalid
	}
	ids, positions, singles := map[string]bool{}, map[string]bool{}, map[string]bool{}
	peopleNames, disciplineNames := map[Ref]string{}, map[Ref]string{}
	for _, fact := range v.Active {
		if !uuid.MatchString(fact.ID) || ids[fact.ID] || !validRef(fact.Person.Ref) || !validName(fact.Person.Name, 300) || fact.CreatedProjectRevision < 1 || fact.CreatedProjectRevision > v.ProjectRevision {
			return ErrInvalid
		}
		ids[fact.ID] = true
		professional := contains(professionalRoles, fact.Role)
		if !professional && !contains(globalRoles, fact.Role) {
			return ErrInvalid
		}
		if professional != (fact.Discipline != nil) || (professional && (!validRef(fact.Discipline.Ref) || !validName(fact.Discipline.Name, 200))) {
			return ErrInvalid
		}
		if old, found := peopleNames[fact.Person.Ref]; found && old != fact.Person.Name {
			return ErrInvalid
		}
		peopleNames[fact.Person.Ref] = fact.Person.Name
		if professional {
			if old, found := disciplineNames[fact.Discipline.Ref]; found && old != fact.Discipline.Name {
				return ErrInvalid
			}
			disciplineNames[fact.Discipline.Ref] = fact.Discipline.Name
		}
		position := fact.Role + "/" + disciplineID(fact.Discipline)
		if positions[position+"/"+fact.Person.ID] || (single(fact.Role) && singles[position]) {
			return fmt.Errorf("%w: duplicate active position", ErrInvalid)
		}
		positions[position+"/"+fact.Person.ID] = true
		singles[position] = true
	}
	return nil
}
func validRef(v Ref) bool { return uuid.MatchString(v.ID) && v.Revision > 0 }
func validName(v string, maximum int) bool {
	return strings.TrimSpace(v) != "" && len([]rune(v)) <= maximum
}
func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
func single(role string) bool {
	return role == "reviewer" || role == "auditor" || role == "approver" || role == "estimator"
}
func disciplineID(v *DisciplineSnapshot) string {
	if v == nil {
		return ""
	}
	return v.ID
}
func copyDiscipline(v *DisciplineSnapshot) *DisciplineSnapshot {
	if v == nil {
		return nil
	}
	copy := *v
	return &copy
}
func equalDiscipline(a, b *DisciplineSnapshot) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
