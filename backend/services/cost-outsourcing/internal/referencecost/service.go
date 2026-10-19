package referencecost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	core "omsbizfoundry/cost-outsourcing/internal/costoutsourcing"
)

type Resolver interface {
	ResolveProject(context.Context, string, core.ProjectRef) (core.ProjectReference, error)
	ResolveEvidence(context.Context, string, core.EvidenceRef) (core.EvidenceReference, error)
}
type Store interface {
	Get(context.Context, string, string) (Record, error)
	List(context.Context, string, string, string) ([]Record, error)
	History(context.Context, string, string) ([]Revision, error)
	Replay(context.Context, core.Actor, string, string) (Record, bool, error)
	Commit(context.Context, core.Actor, string, string, string, string, int64, Record) (Record, error)
}
type Service struct {
	store    Store
	resolver Resolver
}

func New(store Store, resolver Resolver) (*Service, error) {
	if store == nil || resolver == nil {
		return nil, fmt.Errorf("store and resolver are required")
	}
	return &Service{store, resolver}, nil
}
func hash(a core.Actor, op, id string, input any) string {
	bytes, err := json.Marshal(struct {
		Actor, Operation, ID string
		Input                any
	}{a.ID, op, id, input})
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}
func (s *Service) evidence(ctx context.Context, a core.Actor, refs []core.EvidenceRef) ([]core.EvidenceSnapshot, error) {
	values := make([]core.EvidenceSnapshot, 0, len(refs))
	for _, ref := range refs {
		v, err := s.resolver.ResolveEvidence(ctx, a.TenantID, ref)
		if err != nil {
			return nil, err
		}
		if v.TenantID != a.TenantID || v.FileID != ref.FileID || v.Version != ref.Version || v.Status != "available" || !digest.MatchString(v.Digest) || v.DisplayName == "" {
			return nil, core.ErrReferenceInvalid
		}
		v.Slot = ref.Slot
		values = append(values, v.EvidenceSnapshot)
	}
	return values, nil
}
func (s *Service) Create(ctx context.Context, a core.Actor, in CreateInput) (Record, error) {
	if !Can(a, core.ActionCostCreate, in.ProjectRef.ID, in.Scope, "") || !Can(a, core.ActionCostAmountRead, in.ProjectRef.ID, in.Scope, "") {
		return Record{}, core.ErrForbidden
	}
	if err := validateCommand(in.CommandID); err != nil {
		return Record{}, err
	}
	if !uuid.MatchString(in.ProjectRef.ID) || in.ProjectRef.Revision < 1 {
		return Record{}, fmt.Errorf("invalid cost project reference")
	}
	f, amount, err := normalizeFields(in.Fields, in.Scope)
	if err != nil {
		return Record{}, err
	}
	in.Fields = f
	if len(f.EvidenceRefs) > 0 && !Can(a, core.ActionCostEvidenceManage, in.ProjectRef.ID, in.Scope, "") {
		return Record{}, core.ErrForbidden
	}
	h := hash(a, "create", "", in)
	if v, found, err := s.store.Replay(ctx, a, in.CommandID, h); err != nil || found {
		return v, err
	}
	p, err := s.resolver.ResolveProject(ctx, a.TenantID, in.ProjectRef)
	if err != nil {
		return Record{}, err
	}
	if p.TenantID != a.TenantID || p.ID != in.ProjectRef.ID || p.Revision != in.ProjectRef.Revision || p.Status != "active" || p.DisplayName == "" {
		return Record{}, core.ErrReferenceInvalid
	}
	files, err := s.evidence(ctx, a, f.EvidenceRefs)
	if err != nil {
		return Record{}, err
	}
	now := time.Now().UTC()
	v := Record{ID: newID(), TenantID: a.TenantID, Project: p.ProjectSnapshot, Scope: in.Scope, Category: f.Category, Currency: "CNY", AmountMinor: amount, OccurredOn: f.OccurredOn, Description: f.Description, Evidence: files, State: "recorded", Revision: 1, CreatedBy: a.ID, UpdatedBy: a.ID, CreatedAt: now, UpdatedAt: now}
	return s.store.Commit(ctx, a, in.CommandID, h, "create", "", 0, v)
}
func (s *Service) Get(ctx context.Context, a core.Actor, id string) (Record, error) {
	if !ValidActor(a) {
		return Record{}, core.ErrForbidden
	}
	if !uuid.MatchString(id) {
		return Record{}, fmt.Errorf("invalid cost id")
	}
	v, err := s.store.Get(ctx, a.TenantID, id)
	if err != nil {
		return Record{}, err
	}
	if !Can(a, core.ActionCostRead, v.Project.ID, v.Scope, v.ID) {
		return Record{}, core.ErrForbidden
	}
	return v, nil
}
func (s *Service) List(ctx context.Context, a core.Actor, projectID, scope string) ([]Record, error) {
	if !ValidActor(a) {
		return nil, core.ErrForbidden
	}
	if !uuid.MatchString(projectID) || SideKind(scope) == "" {
		return nil, fmt.Errorf("invalid cost list scope")
	}
	read := false
	for _, action := range a.Actions {
		if action == core.ActionCostRead {
			read = true
		}
	}
	if !read {
		return nil, core.ErrForbidden
	}
	wide := Can(a, core.ActionCostRead, projectID, scope, "")
	exact := false
	for _, r := range a.ResourceScopes {
		if r.Type == ResourceType && uuid.MatchString(r.ID) {
			exact = true
		}
	}
	if !wide && !exact {
		return nil, core.ErrForbidden
	}
	all, err := s.store.List(ctx, a.TenantID, projectID, scope)
	if err != nil {
		return nil, err
	}
	visible := make([]Record, 0, len(all))
	for _, v := range all {
		if Can(a, core.ActionCostRead, v.Project.ID, v.Scope, v.ID) {
			visible = append(visible, v)
		}
	}
	return visible, nil
}
func (s *Service) History(ctx context.Context, a core.Actor, id string) ([]Revision, error) {
	v, err := s.Get(ctx, a, id)
	if err != nil {
		return nil, err
	}
	if !Can(a, core.ActionCostHistory, v.Project.ID, v.Scope, v.ID) {
		return nil, core.ErrForbidden
	}
	return s.store.History(ctx, a.TenantID, id)
}
func (s *Service) Correct(ctx context.Context, a core.Actor, id string, in ChangeInput) (Record, error) {
	v, err := s.Get(ctx, a, id)
	if err != nil {
		return Record{}, err
	}
	if !Can(a, core.ActionCostCorrect, v.Project.ID, v.Scope, id) || !Can(a, core.ActionCostAmountRead, v.Project.ID, v.Scope, id) {
		return Record{}, core.ErrForbidden
	}
	if err = validateCommand(in.CommandID); err != nil {
		return Record{}, err
	}
	if in.ExpectedRevision < 1 {
		return Record{}, fmt.Errorf("invalid expected cost revision")
	}
	in.Reason, err = reason(in.Reason)
	if err != nil {
		return Record{}, err
	}
	f, amount, err := normalizeFields(in.Fields, v.Scope)
	if err != nil {
		return Record{}, err
	}
	in.Fields = f
	old := make([]core.EvidenceRef, 0, len(v.Evidence))
	for _, f := range v.Evidence {
		old = append(old, f.EvidenceRef)
	}
	if !reflect.DeepEqual(old, f.EvidenceRefs) && !Can(a, core.ActionCostEvidenceManage, v.Project.ID, v.Scope, id) {
		return Record{}, core.ErrForbidden
	}
	h := hash(a, "correct", id, in)
	if replay, found, err := s.store.Replay(ctx, a, in.CommandID, h); err != nil || found {
		return replay, err
	}
	if v.State != "recorded" {
		return Record{}, core.ErrWithdrawn
	}
	if v.Revision != in.ExpectedRevision {
		return Record{}, core.ErrRevisionConflict
	}
	files, err := s.evidence(ctx, a, f.EvidenceRefs)
	if err != nil {
		return Record{}, err
	}
	v.Category = f.Category
	v.AmountMinor = amount
	v.OccurredOn = f.OccurredOn
	v.Description = f.Description
	v.Evidence = files
	v.Revision++
	v.UpdatedAt = time.Now().UTC()
	v.UpdatedBy = a.ID
	return s.store.Commit(ctx, a, in.CommandID, h, "correct", in.Reason, in.ExpectedRevision, v)
}
func (s *Service) Withdraw(ctx context.Context, a core.Actor, id string, in WithdrawInput) (Record, error) {
	v, err := s.Get(ctx, a, id)
	if err != nil {
		return Record{}, err
	}
	if !Can(a, core.ActionCostWithdraw, v.Project.ID, v.Scope, id) {
		return Record{}, core.ErrForbidden
	}
	if err = validateCommand(in.CommandID); err != nil {
		return Record{}, err
	}
	if in.ExpectedRevision < 1 {
		return Record{}, fmt.Errorf("invalid expected cost revision")
	}
	in.Reason, err = reason(in.Reason)
	if err != nil {
		return Record{}, err
	}
	h := hash(a, "withdraw", id, in)
	if replay, found, err := s.store.Replay(ctx, a, in.CommandID, h); err != nil || found {
		return replay, err
	}
	if v.State != "recorded" {
		return Record{}, core.ErrWithdrawn
	}
	if v.Revision != in.ExpectedRevision {
		return Record{}, core.ErrRevisionConflict
	}
	v.State = "withdrawn"
	v.Revision++
	v.UpdatedAt = time.Now().UTC()
	v.UpdatedBy = a.ID
	return s.store.Commit(ctx, a, in.CommandID, h, "withdraw", in.Reason, in.ExpectedRevision, v)
}
