package referencebonus

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	core "omsbizfoundry/bonus-distribution/internal/bonus"
)

type Resolver interface {
	ResolveProject(context.Context, string, core.ProjectRef) (core.ProjectReference, error)
	ResolveManualParticipant(context.Context, string, core.ProjectSnapshot, string, core.PersonRef, core.AssignmentRef) (core.ParticipantReference, error)
	ListManualCandidates(context.Context, string, core.ProjectRef, string) ([]core.ParticipantReference, error)
}
type ProfessionalResolver interface {
	ResolveProfessionalParticipant(context.Context, string, core.ProjectSnapshot, core.PersonRef, core.AssignmentRef) (Participant, error)
	ListProfessionalCandidates(context.Context, string, core.ProjectRef) ([]Participant, error)
}
type Store interface {
	Get(context.Context, string, string) (Record, error)
	List(context.Context, string, string, string) ([]Record, error)
	History(context.Context, string, string) ([]Revision, error)
	Replay(context.Context, core.Actor, string, string) ([]Record, bool, error)
	Commit(context.Context, core.Actor, string, string, string, string, int64, []Record) ([]Record, error)
}
type Service struct {
	store    Store
	resolver Resolver
	profile  string
}

func New(store Store, resolver Resolver) (*Service, error) {
	if store == nil || resolver == nil {
		return nil, fmt.Errorf("store and resolver are required")
	}
	return &Service{store: store, resolver: resolver, profile: Profile}, nil
}
func NewWithProfile(store Store, resolver Resolver, profile string) (*Service, error) {
	s, err := New(store, resolver)
	if err != nil {
		return nil, err
	}
	if profile != Profile && profile != ProfessionalProfile {
		return nil, fmt.Errorf("invalid manual bonus profile")
	}
	if profile == ProfessionalProfile {
		if _, ok := resolver.(ProfessionalResolver); !ok {
			return nil, fmt.Errorf("professional resolver required")
		}
	}
	s.profile = profile
	return s, nil
}
func (s *Service) Profile() string { return s.profile }
func hash(a core.Actor, op, id string, in any) string {
	b, e := json.Marshal(struct {
		Actor, Operation, ID string
		Input                any
	}{a.ID, op, id, in})
	if e != nil {
		panic(e)
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
func validateParticipant(v core.ParticipantReference, a core.Actor, p core.ProjectSnapshot, scope string, in ItemInput) bool {
	role := EligibleRole(scope, v.Assignment.RoleKey)
	if in.AssignmentSource == ProfessionalSource {
		role = scope == core.TypeProduction && ProfessionalRole(v.Assignment.RoleKey)
	}
	return v.TenantID == a.TenantID && v.BonusType == scope && v.Active && v.Eligible && v.Person.ID == in.PersonRef.ID && v.Person.Revision == in.PersonRef.Revision && strings.TrimSpace(v.Person.DisplayName) != "" && digest.MatchString(v.Person.Digest) && v.Assignment.ID == in.AssignmentRef.ID && v.Assignment.Revision == in.AssignmentRef.Revision && v.Assignment.ProjectID == p.ID && v.Assignment.PersonID == v.Person.ID && digest.MatchString(v.Assignment.Digest) && role
}
func (s *Service) Create(ctx context.Context, a core.Actor, in CreateInput) ([]Record, error) {
	if !Can(a, ActionRecord, in.ProjectRef.ID, in.Scope, "") || !Can(a, ActionAmountRead, in.ProjectRef.ID, in.Scope, "") {
		return nil, core.ErrForbidden
	}
	in, base, amounts, err := normalize(in)
	if err != nil {
		return nil, err
	}
	for _, item := range in.Items {
		if item.AssignmentSource != "" && s.profile != ProfessionalProfile {
			return nil, core.ErrReferenceInvalid
		}
	}
	h := hash(a, "record", "", in)
	if rows, found, e := s.store.Replay(ctx, a, in.CommandID, h); e != nil || found {
		return rows, e
	}
	p, err := s.resolver.ResolveProject(ctx, a.TenantID, in.ProjectRef)
	if err != nil {
		return nil, err
	}
	if p.TenantID != a.TenantID || p.ID != in.ProjectRef.ID || p.Revision != in.ProjectRef.Revision || p.State != "active" || strings.TrimSpace(p.DisplayName) == "" || !digest.MatchString(p.Digest) {
		return nil, core.ErrReferenceInvalid
	}
	now, batch := time.Now().UTC(), newID()
	rows := make([]Record, 0, len(in.Items))
	for i, item := range in.Items {
		var v core.ParticipantReference
		var context Participant
		var e error
		if item.AssignmentSource == ProfessionalSource {
			context, e = s.resolver.(ProfessionalResolver).ResolveProfessionalParticipant(ctx, a.TenantID, p.ProjectSnapshot, item.PersonRef, item.AssignmentRef)
			v = context.ParticipantReference
			if e == nil && (context.AssignmentSource != ProfessionalSource || !validDiscipline(v.Assignment.RoleKey, context.Discipline)) {
				e = core.ErrReferenceInvalid
			}
		} else {
			v, e = s.resolver.ResolveManualParticipant(ctx, a.TenantID, p.ProjectSnapshot, in.Scope, item.PersonRef, item.AssignmentRef)
		}
		if e != nil {
			return nil, e
		}
		if !validateParticipant(v, a, p.ProjectSnapshot, in.Scope, item) {
			return nil, core.ErrReferenceInvalid
		}
		fixed, _ := minor(item.FixedAmountMinor)
		rows = append(rows, Record{ID: newID(), BatchID: batch, TenantID: a.TenantID, Project: p.ProjectSnapshot, Scope: in.Scope, Currency: "CNY", BaseAmountMinor: base, Person: v.Person, Assignment: v.Assignment, Method: item.Method, RatioBasisPoints: item.RatioBasisPoints, FixedAmountMinor: fixed, AmountMinor: amounts[i], OccurredOn: in.OccurredOn, Description: in.Description, State: "recorded", Revision: 1, CreatedBy: a.ID, UpdatedBy: a.ID, CreatedAt: now, UpdatedAt: now})
		if item.AssignmentSource != "" {
			rows[len(rows)-1].Profile = ProfessionalProfile
			rows[len(rows)-1].AssignmentSource = item.AssignmentSource
			if context.Discipline != nil {
				frozen := *context.Discipline
				rows[len(rows)-1].Discipline = &frozen
			}
		}
	}
	return s.store.Commit(ctx, a, in.CommandID, h, "record", "", 0, rows)
}
func (s *Service) Get(ctx context.Context, a core.Actor, id string) (Record, error) {
	if !ValidActor(a) {
		return Record{}, core.ErrForbidden
	}
	if !uuid.MatchString(id) {
		return Record{}, fmt.Errorf("invalid manual bonus id")
	}
	v, e := s.store.Get(ctx, a.TenantID, id)
	if e != nil {
		return Record{}, e
	}
	if !Can(a, ActionRead, v.Project.ID, v.Scope, id) {
		return Record{}, core.ErrForbidden
	}
	return v, nil
}
func (s *Service) List(ctx context.Context, a core.Actor, project, scope string) ([]Record, error) {
	if !Can(a, ActionRead, project, scope, "") {
		exact := false
		for _, v := range a.ResourceScopes {
			exact = exact || v.Type == ResourceType && uuid.MatchString(v.ID)
		}
		allowed := false
		for _, v := range a.Actions {
			allowed = allowed || v == ActionRead
		}
		if !ValidActor(a) || !uuid.MatchString(project) || SideKind(scope) == "" || !exact || !allowed {
			return nil, core.ErrForbidden
		}
	}
	rows, e := s.store.List(ctx, a.TenantID, project, scope)
	if e != nil {
		return nil, e
	}
	out := []Record{}
	for _, v := range rows {
		if Can(a, ActionRead, v.Project.ID, v.Scope, v.ID) {
			out = append(out, v)
		}
	}
	return out, nil
}
func (s *Service) History(ctx context.Context, a core.Actor, id string) ([]Revision, error) {
	v, e := s.Get(ctx, a, id)
	if e != nil {
		return nil, e
	}
	if !Can(a, ActionHistory, v.Project.ID, v.Scope, id) {
		return nil, core.ErrForbidden
	}
	return s.store.History(ctx, a.TenantID, id)
}
func (s *Service) Withdraw(ctx context.Context, a core.Actor, id string, in WithdrawInput) (Record, error) {
	v, e := s.Get(ctx, a, id)
	if e != nil {
		return Record{}, e
	}
	if !Can(a, ActionWithdraw, v.Project.ID, v.Scope, id) {
		return Record{}, core.ErrForbidden
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if !uuid.MatchString(in.CommandID) || in.ExpectedRevision < 1 || in.Reason == "" || utf8.RuneCountInString(in.Reason) > 500 {
		return Record{}, fmt.Errorf("invalid manual bonus withdrawal")
	}
	h := hash(a, "withdraw", id, in)
	if rows, found, e := s.store.Replay(ctx, a, in.CommandID, h); e != nil || found {
		if e != nil {
			return Record{}, e
		}
		return rows[0], nil
	}
	if v.State != "recorded" {
		return Record{}, core.ErrStateConflict
	}
	if v.Revision != in.ExpectedRevision {
		return Record{}, core.ErrRevisionConflict
	}
	v.State = "withdrawn"
	v.Revision++
	v.UpdatedBy = a.ID
	v.UpdatedAt = time.Now().UTC()
	rows, e := s.store.Commit(ctx, a, in.CommandID, h, "withdraw", in.Reason, in.ExpectedRevision, []Record{v})
	if e != nil {
		return Record{}, e
	}
	return rows[0], nil
}
func (s *Service) Candidates(ctx context.Context, a core.Actor, ref core.ProjectRef, scope string) ([]Participant, error) {
	if !Can(a, ActionRecord, ref.ID, scope, "") {
		return nil, core.ErrForbidden
	}
	if !uuid.MatchString(ref.ID) || ref.Revision < 1 {
		return nil, fmt.Errorf("invalid manual bonus project reference")
	}
	legacy, e := s.resolver.ListManualCandidates(ctx, a.TenantID, ref, scope)
	if e != nil {
		return nil, e
	}
	rows := []Participant{}
	for _, v := range legacy {
		rows = append(rows, Participant{ParticipantReference: v})
	}
	if s.profile == ProfessionalProfile && scope == core.TypeProduction {
		professionals, err := s.resolver.(ProfessionalResolver).ListProfessionalCandidates(ctx, a.TenantID, ref)
		if err != nil {
			return nil, err
		}
		rows = append(rows, professionals...)
	}
	if len(rows) > 100 {
		return nil, core.ErrReferenceInvalid
	}
	seen := map[string]bool{}
	for _, v := range rows {
		in := ItemInput{AssignmentSource: v.AssignmentSource, PersonRef: core.PersonRef{ID: v.Person.ID, Revision: v.Person.Revision}, AssignmentRef: core.AssignmentRef{ID: v.Assignment.ID, Revision: v.Assignment.Revision}}
		key := v.AssignmentSource + ":" + v.Assignment.ID
		if (v.AssignmentSource != "" && (v.AssignmentSource != ProfessionalSource || s.profile != ProfessionalProfile || !validDiscipline(v.Assignment.RoleKey, v.Discipline))) || !uuid.MatchString(v.Person.ID) || !uuid.MatchString(v.Assignment.ID) || v.Person.Revision < 1 || v.Assignment.Revision < 1 || seen[key] || !validateParticipant(v.ParticipantReference, a, core.ProjectSnapshot{ID: ref.ID}, scope, in) {
			return nil, core.ErrReferenceInvalid
		}
		seen[key] = true
	}
	return rows, nil
}
