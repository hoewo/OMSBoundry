package results

import (
	"context"
	"errors"
	"testing"
	"time"
)

type serviceStore struct {
	current      Result
	revision     Revision
	recorded     ResolvedResult
	corrected    ResolvedResult
	recordCalls  int
	correctCalls int
	replay       Result
	replayFound  bool
}

func (s *serviceStore) Ping(context.Context) error { return nil }
func (s *serviceStore) Replay(context.Context, Actor, string, string) (Result, bool, error) {
	return s.replay, s.replayFound, nil
}
func (s *serviceStore) Record(_ context.Context, actor Actor, input RecordInput, resolved ResolvedResult, _ string) (Result, error) {
	s.recordCalls++
	s.recorded = resolved
	return Result{ID: id(9), TenantID: actor.TenantID, Project: resolved.Project, Kind: input.Kind, State: StateRecorded, AmountComponents: resolved.AmountComponents, TotalAmountMinor: resolved.TotalAmountMinor, Source: resolved.Source, Revision: 1}, nil
}
func (s *serviceStore) Get(context.Context, string, string) (Result, error) {
	if s.current.ID == "" {
		return Result{}, ErrNotFound
	}
	return s.current, nil
}
func (s *serviceStore) ListProject(context.Context, string, string) ([]Result, error) {
	if s.current.ID == "" {
		return []Result{}, nil
	}
	return []Result{s.current}, nil
}
func (s *serviceStore) GetRevision(context.Context, string, string, int64) (Revision, error) {
	if s.revision.ResultID == "" {
		return Revision{}, ErrNotFound
	}
	return s.revision, nil
}
func (s *serviceStore) History(context.Context, string, string) ([]Revision, error) { return nil, nil }
func (s *serviceStore) Correct(_ context.Context, _ Actor, _ string, _ CorrectInput, resolved ResolvedResult, _ string) (Result, error) {
	s.correctCalls++
	s.corrected = resolved
	return Result{ID: s.current.ID, TenantID: s.current.TenantID, Project: resolved.Project, Kind: s.current.Kind, State: StateRecorded, Revision: s.current.Revision + 1}, nil
}
func (s *serviceStore) Withdraw(context.Context, Actor, string, WithdrawInput, string) (Result, error) {
	return Result{}, nil
}

type serviceResolver struct {
	project           ProjectReference
	source            SourceReference
	evidence          map[string]EvidenceReference
	sourceCalls       int
	evidenceError     error
	projectRefs       []ProjectRef
	sourceProjectRefs []ProjectRef
}

func (r *serviceResolver) ResolveProject(_ context.Context, _ string, ref ProjectRef) (ProjectReference, error) {
	r.projectRefs = append(r.projectRefs, ref)
	return r.project, nil
}
func (r *serviceResolver) ResolveSource(_ context.Context, _ string, projectRef ProjectRef, _ SourceRef) (SourceReference, error) {
	r.sourceCalls++
	r.sourceProjectRefs = append(r.sourceProjectRefs, projectRef)
	return r.source, nil
}
func (r *serviceResolver) ResolveEvidence(_ context.Context, _ string, ref EvidenceRef) (EvidenceReference, error) {
	if r.evidenceError != nil {
		return EvidenceReference{}, r.evidenceError
	}
	return r.evidence[ref.FileID], nil
}

func TestAuditCopiesExactApprovalRevisionWithoutRereadingContract(t *testing.T) {
	actor := Actor{ID: id(1), TenantID: id(2), Kind: "user", CredentialID: "cred-1"}
	project := ProjectSnapshot{ID: id(3), Revision: 7, Code: "P-001", DisplayName: "道路设计项目"}
	components := standardComponents(100, 20, 5)
	approval := Result{ID: id(4), TenantID: actor.TenantID, Project: project, Kind: KindApproval, State: StateRecorded, AmountProfile: AmountProfileReferenceOAV1, Currency: "CNY", AmountComponents: components, TotalAmountMinor: 125, Revision: 1}
	store := &serviceStore{revision: Revision{ResultID: approval.ID, TenantID: actor.TenantID, Revision: 1, Snapshot: approval}}
	resolver := &serviceResolver{project: ProjectReference{ProjectSnapshot: project, TenantID: actor.TenantID, Status: "active"}}
	service, err := NewService(store, resolver)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Record(context.Background(), actor, RecordInput{
		CommandID: id(5), ProjectRef: ProjectRef{ID: project.ID, Revision: project.Revision}, Kind: KindAudit, Currency: "CNY",
		SourceRef:           SourceRef{Application: "approval-audit-results", ResourceType: "approval-result", ResourceID: approval.ID, Revision: 1},
		ExternalConfirmedBy: "审计结果单", ExternalConfirmedAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolver.sourceCalls != 0 {
		t.Fatalf("audit unexpectedly reread an external source %d time(s)", resolver.sourceCalls)
	}
	if result.TotalAmountMinor != 125 || result.Source.Revision != 1 || result.Source.ResourceID != approval.ID {
		t.Fatalf("audit did not preserve exact approval revision: %#v", result)
	}
}

func TestAmountOverrideRequiresReason(t *testing.T) {
	actor, project, resolver, store := approvalFixture()
	service, _ := NewService(store, resolver)
	input := approvalInput(project)
	input.AmountComponents = standardComponents(101, 20, 5)
	if _, err := service.Record(context.Background(), actor, input); err == nil {
		t.Fatal("amount override without reason was accepted")
	}
	if store.recordCalls != 0 {
		t.Fatal("store was called for invalid override")
	}
	input.OverrideReason = "合同外书面调增 1 分"
	if _, err := service.Record(context.Background(), actor, input); err != nil {
		t.Fatal(err)
	}
	if store.recorded.TotalAmountMinor != 126 {
		t.Fatalf("total=%d, want 126", store.recorded.TotalAmountMinor)
	}
}

func TestEvidenceFailureLeavesNoPartialResult(t *testing.T) {
	actor, project, resolver, store := approvalFixture()
	resolver.evidenceError = ErrReferenceUnavailable
	service, _ := NewService(store, resolver)
	input := approvalInput(project)
	input.EvidenceRefs = []EvidenceRef{{FileID: id(8), Version: 2, Slot: "approval_document"}}
	if _, err := service.Record(context.Background(), actor, input); !errors.Is(err, ErrReferenceUnavailable) {
		t.Fatalf("err=%v", err)
	}
	if store.recordCalls != 0 {
		t.Fatal("result was persisted despite dependency failure")
	}
}

func TestReplayReturnsBeforeDependencies(t *testing.T) {
	actor, project, resolver, store := approvalFixture()
	store.replayFound = true
	store.replay = Result{ID: id(9), TenantID: actor.TenantID, Revision: 1}
	resolver.evidenceError = errors.New("must not be called")
	service, _ := NewService(store, resolver)
	result, err := service.Record(context.Background(), actor, approvalInput(project))
	if err != nil {
		t.Fatal(err)
	}
	if result.ID != store.replay.ID || resolver.sourceCalls != 0 || store.recordCalls != 0 {
		t.Fatalf("replay was not short-circuited: %#v", result)
	}
}

func TestCorrectionUsesCurrentProjectAfterIndependentProjectRevisionAdvance(t *testing.T) {
	actor, frozenProject, resolver, store := approvalFixture()
	store.current = Result{
		ID: id(9), TenantID: actor.TenantID, Project: frozenProject, Kind: KindApproval,
		State: StateRecorded, Revision: 1, Currency: "CNY", AmountProfile: AmountProfileReferenceOAV1,
		AmountComponents: standardComponents(100, 20, 5), TotalAmountMinor: 125,
		Source: resolver.source.SourceSnapshot,
	}
	resolver.project.ProjectSnapshot.Revision = frozenProject.Revision + 1
	service, _ := NewService(store, resolver)

	value, err := service.Correct(context.Background(), actor, store.current.ID, CorrectInput{
		CommandID: id(6), ExpectedRevision: 1, Reason: "Project 检查点确认后更正外部确认人",
		Currency: "CNY", SourceRef: store.current.Source.SourceRef,
		ExternalConfirmedBy: "更正后的外部确认人", ExternalConfirmedAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if store.correctCalls != 1 || value.Revision != 2 || store.corrected.Project.Revision != frozenProject.Revision+1 {
		t.Fatalf("correction did not freeze the current Project revision: value=%#v resolved=%#v", value, store.corrected)
	}
	if len(resolver.projectRefs) != 1 || resolver.projectRefs[0] != (ProjectRef{ID: frozenProject.ID}) {
		t.Fatalf("Project lookup must request the current revision: %#v", resolver.projectRefs)
	}
	if len(resolver.sourceProjectRefs) != 1 || resolver.sourceProjectRefs[0] != (ProjectRef{ID: frozenProject.ID}) {
		t.Fatalf("source validation must use the same current Project lookup: %#v", resolver.sourceProjectRefs)
	}
}

func approvalFixture() (Actor, ProjectSnapshot, *serviceResolver, *serviceStore) {
	actor := Actor{ID: id(1), TenantID: id(2), Kind: "user", CredentialID: "cred-1"}
	project := ProjectSnapshot{ID: id(3), Revision: 7, Code: "P-001", DisplayName: "道路设计项目"}
	components := standardComponents(100, 20, 5)
	ref := SourceRef{Application: "contract-ledger", ResourceType: "settlement", ResourceID: id(4), Revision: 9}
	resolver := &serviceResolver{
		project:  ProjectReference{ProjectSnapshot: project, TenantID: actor.TenantID, Status: "active"},
		source:   SourceReference{SourceSnapshot: SourceSnapshot{SourceRef: ref, State: "confirmed", ProjectID: project.ID, Currency: "CNY", AmountProfile: AmountProfileReferenceOAV1, AmountComponents: components, TotalAmountMinor: 125, Digest: "sha256:contract-rev-9", DisplaySummary: "结算 revision 9"}, TenantID: actor.TenantID, Eligible: true},
		evidence: map[string]EvidenceReference{},
	}
	return actor, project, resolver, &serviceStore{}
}

func approvalInput(project ProjectSnapshot) RecordInput {
	return RecordInput{CommandID: id(5), ProjectRef: ProjectRef{ID: project.ID, Revision: project.Revision}, Kind: KindApproval, Currency: "CNY", SourceRef: SourceRef{Application: "contract-ledger", ResourceType: "settlement", ResourceID: id(4), Revision: 9}, ExternalConfirmedBy: "OA 审批记录", ExternalConfirmedAt: time.Now()}
}

func standardComponents(design, survey, consultation int64) []AmountComponent {
	return []AmountComponent{{Kind: ComponentDesignFee, AmountMinor: design}, {Kind: ComponentSurveyFee, AmountMinor: survey}, {Kind: ComponentConsultationFee, AmountMinor: consultation}}
}
