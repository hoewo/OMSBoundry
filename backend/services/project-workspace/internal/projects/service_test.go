package projects

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	testTenant  = "11111111-1111-4111-8111-111111111111"
	testActorID = "22222222-2222-4222-8222-222222222222"
	testProject = "33333333-3333-4333-8333-333333333333"
	testCommand = "44444444-4444-4444-8444-444444444444"
	testSubject = "55555555-5555-4555-8555-555555555555"
)

type storeStub struct {
	namesCalls            int
	namesQuery            CounterpartyNamesQuery
	contextInput          CounterpartyContextInput
	contextCalls          int
	basicChangeInput      BasicInfoChangeInput
	basicChangeCalls      int
	basicChangeReplay     BasicInfoChangeResult
	basicChangeFound      bool
	initiationChangeCalls int
	createInput           CreateInput
	createReplay          Project
	createReplayFound     bool
	createReplayErr       error
	createCalls           int
	assignmentInput       AssignmentCommand
	resourceInput         ResourceLinkCommand
	updateInput           UpdateInput
	correctionInput       ProfileCorrectionInput
	correctionCalls       int
	assignmentCalls       int
	resourceCalls         int
	updateCalls           int
	details               Details
	completion            *CompletionConfirmation
	completionInput       CompletionCommand
	completionView        CompletionAssessment
	completionCalls       int
	completionReplay      CompletionConfirmation
	completionFound       bool
	completionReplayErr   error
	initiationInput       RevisionCommand
	initiationCalls       int
	cancelCalls           int
	dispatchInput         InitiationDispatchInput
	dispatchCalls         int
	dispatchReplay        Project
	dispatchFound         bool
	dispatchReplayErr     error
}

func (*storeStub) Ping(context.Context) error { return nil }
func (s *storeStub) CounterpartyNameSuggestions(_ context.Context, _ string, q CounterpartyNamesQuery) (CounterpartyNames, error) {
	s.namesCalls++
	s.namesQuery = q
	return CounterpartyNames{SchemaVersion: CounterpartyNamesSchema, Names: []string{}}, nil
}
func (*storeStub) CounterpartyContext(context.Context, string, string) (CounterpartyContextView, error) {
	return CounterpartyContextView{}, nil
}
func (s *storeStub) ChangeCounterpartyContext(_ context.Context, _ Actor, _ string, in CounterpartyContextInput) (CounterpartyContextView, error) {
	s.contextCalls++
	s.contextInput = in
	return CounterpartyContextView{}, nil
}
func (s *storeStub) ReplayBasicInfoChange(context.Context, string, string, BasicInfoChangeInput) (BasicInfoChangeResult, bool, error) {
	return s.basicChangeReplay, s.basicChangeFound, nil
}
func (s *storeStub) ChangeBasicInfo(_ context.Context, _ Actor, _ string, in BasicInfoChangeInput) (BasicInfoChangeResult, error) {
	s.basicChangeCalls++
	s.basicChangeInput = in
	return BasicInfoChangeResult{}, nil
}
func (s *storeStub) ReplayInitiationInfoChange(context.Context, string, string, BasicInfoChangeInput) (BasicInfoChangeResult, bool, error) {
	return s.basicChangeReplay, s.basicChangeFound, nil
}
func (s *storeStub) ChangeInitiationInfo(_ context.Context, _ Actor, _ string, in BasicInfoChangeInput) (BasicInfoChangeResult, error) {
	s.initiationChangeCalls++
	s.basicChangeInput = in
	return BasicInfoChangeResult{}, nil
}
func (*storeStub) List(context.Context, string, int) ([]Project, error) { return nil, nil }
func (*storeStub) Directory(context.Context, string, DirectoryQuery) (DirectoryPage, error) {
	return DirectoryPage{}, nil
}
func (s *storeStub) Create(_ context.Context, actor Actor, input CreateInput) (Project, error) {
	s.createCalls++
	s.createInput = input
	return Project{ID: testProject, TenantID: actor.TenantID, ProjectNumber: input.ProjectNumber, ClientParty: input.validatedClientParty}, nil
}
func (s *storeStub) ReplayCreate(context.Context, string, CreateInput) (Project, bool, error) {
	return s.createReplay, s.createReplayFound, s.createReplayErr
}
func (s *storeStub) DispatchInitiation(_ context.Context, _ Actor, input InitiationDispatchInput) (Project, error) {
	s.dispatchInput = input
	s.dispatchCalls++
	return Project{ID: testProject, Status: StatusDraft}, nil
}
func (s *storeStub) ReplayInitiationDispatch(context.Context, string, InitiationDispatchInput) (Project, bool, error) {
	return s.dispatchReplay, s.dispatchFound, s.dispatchReplayErr
}
func (*storeStub) CreatePackage(context.Context, Actor, PackageCreateInput) (Project, error) {
	return Project{}, nil
}
func (*storeStub) ConvertPackage(context.Context, Actor, string, PackageConversionCommand) (PackageConversionResult, error) {
	return PackageConversionResult{}, nil
}
func (*storeStub) RollbackPackageConversion(context.Context, Actor, string, string, PackageRollbackCommand) (PackageConversionResult, error) {
	return PackageConversionResult{}, nil
}
func (s *storeStub) Update(_ context.Context, _ Actor, _ string, input UpdateInput) (Project, error) {
	s.updateCalls++
	s.updateInput = input
	return s.details.Project, nil
}
func (s *storeStub) CorrectProfile(_ context.Context, _ Actor, _ string, input ProfileCorrectionInput) (Project, error) {
	s.correctionCalls++
	s.correctionInput = input
	return s.details.Project, nil
}
func (*storeStub) ReplayAssignment(context.Context, string, string, AssignmentCommand) (AssignmentResult, bool, error) {
	return AssignmentResult{}, false, nil
}
func (s *storeStub) ApplyAssignment(_ context.Context, _ Actor, _ string, input AssignmentCommand) (AssignmentResult, error) {
	s.assignmentCalls++
	s.assignmentInput = input
	return AssignmentResult{Assignment: Assignment{SubjectID: input.SubjectID, DisplaySnapshot: input.DisplaySnapshot}}, nil
}
func (s *storeStub) ApplyResourceLink(_ context.Context, _ Actor, _ string, input ResourceLinkCommand) (ResourceLinkResult, error) {
	s.resourceCalls++
	s.resourceInput = input
	return ResourceLinkResult{}, nil
}
func (*storeStub) ReplayResourceLink(context.Context, string, string, ResourceLinkCommand) (ResourceLinkResult, bool, error) {
	return ResourceLinkResult{}, false, nil
}
func (s *storeStub) CompleteInitiation(_ context.Context, _ Actor, _ string, input RevisionCommand) (Project, error) {
	s.initiationCalls++
	s.initiationInput = input
	return Project{}, nil
}
func (s *storeStub) Cancel(context.Context, Actor, string, CancelCommand) (Project, error) {
	s.cancelCalls++
	return Project{}, nil
}
func (s *storeStub) Get(context.Context, string, string, bool) (Details, error) {
	return s.details, nil
}
func (*storeStub) History(context.Context, string, string, string, int) (HistoryPage, error) {
	return HistoryPage{}, nil
}
func (s *storeStub) LatestCompletionConfirmation(context.Context, string, string, string) (*CompletionConfirmation, error) {
	return s.completion, nil
}
func (s *storeStub) ReplayCompletion(context.Context, string, string, CompletionCommand) (CompletionConfirmation, bool, error) {
	return s.completionReplay, s.completionFound, s.completionReplayErr
}
func (s *storeStub) ConfirmCompletion(_ context.Context, _ Actor, _ string, input CompletionCommand, assessment CompletionAssessment) (CompletionConfirmation, error) {
	s.completionCalls++
	s.completionInput = input
	s.completionView = assessment
	return CompletionConfirmation{ID: testCommand, ProjectID: testProject, ProfileKey: input.ProfileKey, Sources: input.ExpectedSources}, nil
}

type referencesStub struct {
	peopleResult  ReferenceSnapshot
	peopleErr     error
	resourceErr   error
	partyResult   PartySnapshot
	partyErr      error
	peopleCalls   int
	resourceCalls int
	partyCalls    int
}

type commercialReferencesStub struct {
	started chan string
	release chan struct{}
	values  map[string]ResourceReference
	errors  map[string]error
}

type materialReferencesStub struct {
	values map[string]ResourceReference
	errors map[string]error
	calls  []string
	mu     sync.Mutex
}

func (*materialReferencesStub) ValidateParty(context.Context, string, string, int64) (PartySnapshot, error) {
	return PartySnapshot{}, nil
}
func (*materialReferencesStub) ValidatePeople(context.Context, string, string, string, int64) (ReferenceSnapshot, error) {
	return ReferenceSnapshot{}, nil
}
func (*materialReferencesStub) ValidateResource(context.Context, string, string, string, string, *int64) error {
	return nil
}
func (s *materialReferencesStub) ResolveResource(_ context.Context, _, service, resourceType, id string) (ResourceReference, error) {
	key := service + "/" + resourceType + "/" + id
	s.mu.Lock()
	s.calls = append(s.calls, key)
	s.mu.Unlock()
	value, ok := s.values[key]
	if !ok {
		value = s.values[id]
	}
	err, ok := s.errors[key]
	if !ok {
		err = s.errors[id]
	}
	return value, err
}

type completionReferencesStub struct {
	values []CompletionResultReference
	err    error
	calls  int
}

func (*completionReferencesStub) ValidateParty(context.Context, string, string, int64) (PartySnapshot, error) {
	return PartySnapshot{}, nil
}
func (*completionReferencesStub) ValidatePeople(context.Context, string, string, string, int64) (ReferenceSnapshot, error) {
	return ReferenceSnapshot{}, nil
}
func (*completionReferencesStub) ValidateResource(context.Context, string, string, string, string, *int64) error {
	return nil
}
func (s *completionReferencesStub) ResolveCompletionResults(context.Context, string, string) ([]CompletionResultReference, error) {
	s.calls++
	return s.values, s.err
}

func (*commercialReferencesStub) ValidateParty(context.Context, string, string, int64) (PartySnapshot, error) {
	return PartySnapshot{}, nil
}
func (*commercialReferencesStub) ValidatePeople(context.Context, string, string, string, int64) (ReferenceSnapshot, error) {
	return ReferenceSnapshot{}, nil
}
func (*commercialReferencesStub) ValidateResource(context.Context, string, string, string, string, *int64) error {
	return nil
}
func (s *commercialReferencesStub) ResolveResource(_ context.Context, _, service, resourceType, _ string) (ResourceReference, error) {
	key := service + "/" + resourceType
	s.started <- key
	<-s.release
	return s.values[key], s.errors[key]
}

func (s *referencesStub) ValidateParty(context.Context, string, string, int64) (PartySnapshot, error) {
	s.partyCalls++
	return s.partyResult, s.partyErr
}

func (s *referencesStub) ValidatePeople(context.Context, string, string, string, int64) (ReferenceSnapshot, error) {
	s.peopleCalls++
	return s.peopleResult, s.peopleErr
}
func (s *referencesStub) ValidateResource(context.Context, string, string, string, string, *int64) error {
	s.resourceCalls++
	return s.resourceErr
}

func TestCompletionAssessmentIsReadOnlyAndDetectsDrift(t *testing.T) {
	approvalID := "66666666-6666-4666-8666-666666666666"
	auditID := "77777777-7777-4777-8777-777777777777"
	store := &storeStub{details: Details{Project: Project{ID: testProject, TenantID: testTenant, NodeType: NodeTypeProject, Status: StatusActive, Revision: 7}}}
	refs := &completionReferencesStub{values: []CompletionResultReference{
		{ID: approvalID, TenantID: testTenant, ProjectID: testProject, ResourceType: "approval_audit_result", Kind: "approval", State: "recorded", Revision: 2, Digest: "approval-r2", DisplaySummary: "批复 r2"},
		{ID: auditID, TenantID: testTenant, ProjectID: testProject, ResourceType: "approval_audit_result", Kind: "audit", State: "recorded", Revision: 1, Digest: "audit-r1", DisplaySummary: "审计 r1"},
	}}
	service, _ := NewService(store, refs)
	view, err := service.EvaluateCompletion(context.Background(), testTenant, testProject, CompletionProfileReferenceOAC5V1)
	if err != nil || view.OverallStatus != CompletionSatisfied || view.ProjectRevision != 7 || view.Drifted || store.completionCalls != 0 {
		t.Fatalf("view=%#v writes=%d err=%v", view, store.completionCalls, err)
	}
	store.completion = &CompletionConfirmation{ProfileKey: CompletionProfileReferenceOAC5V1, Sources: []CompletionSource{
		{RequirementKey: "approval_result_recorded", ResultID: approvalID, ResultKind: "approval", Revision: 1, Digest: "approval-r1"},
		{RequirementKey: "audit_result_recorded", ResultID: auditID, ResultKind: "audit", Revision: 1, Digest: "audit-r1"},
	}}
	view, err = service.EvaluateCompletion(context.Background(), testTenant, testProject, CompletionProfileReferenceOAC5V1)
	if err != nil || !view.Drifted || store.completionCalls != 0 {
		t.Fatalf("drift view=%#v writes=%d err=%v", view, store.completionCalls, err)
	}
}

func TestCompletionAssessmentReturnsUnavailableWithoutProjectWrite(t *testing.T) {
	store := &storeStub{details: Details{Project: Project{ID: testProject, TenantID: testTenant, NodeType: NodeTypeProject, Status: StatusActive, Revision: 4}}}
	refs := &completionReferencesStub{err: ErrReferenceUnavailable}
	service, _ := NewService(store, refs)
	view, err := service.EvaluateCompletion(context.Background(), testTenant, testProject, CompletionProfileReferenceOAC5V1)
	if err != nil || view.OverallStatus != CompletionUnavailable || len(view.Requirements) != 2 || store.completionCalls != 0 {
		t.Fatalf("view=%#v writes=%d err=%v", view, store.completionCalls, err)
	}
}

func TestConfirmCompletionReturnsUnavailableWithoutProjectWrite(t *testing.T) {
	store := &storeStub{details: Details{Project: Project{ID: testProject, TenantID: testTenant, NodeType: NodeTypeProject, Status: StatusActive, Revision: 4}}}
	refs := &completionReferencesStub{err: ErrReferenceUnavailable}
	service, _ := NewService(store, refs)
	_, err := service.ConfirmCompletion(context.Background(), Actor{ID: testActorID, TenantID: testTenant}, testProject, CompletionCommand{
		CommandID: testCommand, ExpectedRevision: 4, ProfileKey: CompletionProfileReferenceOAC5V1,
		ExpectedSources: []CompletionSource{
			{RequirementKey: "approval_result_recorded", ResultID: "66666666-6666-4666-8666-666666666666", ResultKind: "approval", Revision: 1, Digest: "approval-r1"},
			{RequirementKey: "audit_result_recorded", ResultID: "77777777-7777-4777-8777-777777777777", ResultKind: "audit", Revision: 1, Digest: "audit-r1"},
		},
	})
	if !errors.Is(err, ErrReferenceUnavailable) || refs.calls != 1 || store.completionCalls != 0 {
		t.Fatalf("dependency_calls=%d writes=%d err=%v", refs.calls, store.completionCalls, err)
	}
}

func TestConfirmCompletionFreezesExactSourcesAndRejectsStaleInput(t *testing.T) {
	approvalID := "66666666-6666-4666-8666-666666666666"
	auditID := "77777777-7777-4777-8777-777777777777"
	store := &storeStub{details: Details{Project: Project{ID: testProject, TenantID: testTenant, NodeType: NodeTypeProject, Status: StatusActive, Revision: 7}}}
	refs := &completionReferencesStub{values: []CompletionResultReference{
		{ID: approvalID, TenantID: testTenant, ProjectID: testProject, ResourceType: "approval_audit_result", Kind: "approval", State: "recorded", Revision: 2, Digest: "approval-r2", DisplaySummary: "批复 r2"},
		{ID: auditID, TenantID: testTenant, ProjectID: testProject, ResourceType: "approval_audit_result", Kind: "audit", State: "recorded", Revision: 1, Digest: "audit-r1", DisplaySummary: "审计 r1"},
	}}
	service, _ := NewService(store, refs)
	sources := []CompletionSource{
		{RequirementKey: "approval_result_recorded", ResultID: approvalID, ResultKind: "approval", Revision: 2, Digest: "approval-r2"},
		{RequirementKey: "audit_result_recorded", ResultID: auditID, ResultKind: "audit", Revision: 1, Digest: "audit-r1"},
	}
	value, err := service.ConfirmCompletion(context.Background(), Actor{ID: testActorID, TenantID: testTenant}, testProject, CompletionCommand{CommandID: testCommand, ExpectedRevision: 7, ProfileKey: CompletionProfileReferenceOAC5V1, ExpectedSources: sources})
	if err != nil || value.ProfileKey != CompletionProfileReferenceOAC5V1 || store.completionCalls != 1 || len(store.completionView.Requirements) != 2 {
		t.Fatalf("value=%#v writes=%d assessment=%#v err=%v", value, store.completionCalls, store.completionView, err)
	}
	stale := append([]CompletionSource(nil), sources...)
	stale[0].Revision = 1
	if _, err := service.ConfirmCompletion(context.Background(), Actor{ID: testActorID, TenantID: testTenant}, testProject, CompletionCommand{CommandID: "88888888-8888-4888-8888-888888888888", ExpectedRevision: 7, ProfileKey: CompletionProfileReferenceOAC5V1, ExpectedSources: stale}); !errors.Is(err, ErrCompletionDrift) {
		t.Fatalf("stale error=%v", err)
	}
	if store.completionCalls != 1 {
		t.Fatalf("stale input wrote project %d times", store.completionCalls)
	}
}

func TestConfirmCompletionReplayDoesNotRequireResultsDependency(t *testing.T) {
	replay := CompletionConfirmation{ID: testCommand, ProjectID: testProject, ProfileKey: CompletionProfileReferenceOAC5V1}
	store := &storeStub{completionReplay: replay, completionFound: true}
	refs := &completionReferencesStub{err: ErrReferenceUnavailable}
	service, _ := NewService(store, refs)
	value, err := service.ConfirmCompletion(context.Background(), Actor{ID: testActorID, TenantID: testTenant}, testProject, CompletionCommand{
		CommandID: testCommand, ExpectedRevision: 7, ProfileKey: CompletionProfileReferenceOAC5V1,
		ExpectedSources: []CompletionSource{
			{RequirementKey: "approval_result_recorded", ResultID: "66666666-6666-4666-8666-666666666666", ResultKind: "approval", Revision: 2, Digest: "approval-r2"},
			{RequirementKey: "audit_result_recorded", ResultID: "77777777-7777-4777-8777-777777777777", ResultKind: "audit", Revision: 1, Digest: "audit-r1"},
		},
	})
	if err != nil || value.ID != replay.ID || refs.calls != 0 || store.completionCalls != 0 {
		t.Fatalf("value=%#v dependency_calls=%d writes=%d err=%v", value, refs.calls, store.completionCalls, err)
	}
}

func TestCreateValidatesAndFreezesClientPartyBeforeWriting(t *testing.T) {
	store := &storeStub{}
	references := &referencesStub{partyResult: PartySnapshot{PartyID: testSubject, SourceRevision: 3, Kind: "organization", LegalName: "甲方公司"}}
	service, _ := NewService(store, references)
	result, err := service.Create(context.Background(), Actor{TenantID: testTenant}, CreateInput{
		CommandID: testCommand, ProjectNumber: "OA-PARTY", Name: "主体引用项目", ProjectTypeKeys: []string{"design"},
		ClientParty: &PartyReferenceInput{PartyID: testSubject, SourceRevision: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if references.partyCalls != 1 || store.createCalls != 1 || result.ClientParty == nil || result.ClientParty.LegalName != "甲方公司" || store.createInput.validatedClientParty == nil {
		t.Fatalf("refs=%d writes=%d result=%#v input=%#v", references.partyCalls, store.createCalls, result, store.createInput)
	}
}

func TestCreateFailsClosedWhenClientPartyUnavailable(t *testing.T) {
	store := &storeStub{}
	references := &referencesStub{partyErr: ErrReferenceUnavailable}
	service, _ := NewService(store, references)
	_, err := service.Create(context.Background(), Actor{TenantID: testTenant}, CreateInput{
		CommandID: testCommand, ProjectNumber: "OA-PARTY", Name: "主体引用项目", ProjectTypeKeys: []string{"design"},
		ClientParty: &PartyReferenceInput{PartyID: testSubject, SourceRevision: 3},
	})
	if !errors.Is(err, ErrReferenceUnavailable) || store.createCalls != 0 {
		t.Fatalf("error=%v writes=%d", err, store.createCalls)
	}
}

func TestCreateReplayDoesNotCallClientParty(t *testing.T) {
	replay := Project{ID: testProject, TenantID: testTenant, ProjectNumber: "OA-PARTY", ClientParty: &PartySnapshot{PartyID: testSubject, SourceRevision: 3, Kind: "organization", LegalName: "旧名称"}}
	store := &storeStub{createReplay: replay, createReplayFound: true}
	references := &referencesStub{partyErr: ErrReferenceUnavailable}
	service, _ := NewService(store, references)
	result, err := service.Create(context.Background(), Actor{TenantID: testTenant}, CreateInput{
		CommandID: testCommand, ProjectNumber: "OA-PARTY", Name: "主体引用项目", ProjectTypeKeys: []string{"design"},
		ClientParty: &PartyReferenceInput{PartyID: testSubject, SourceRevision: 3},
	})
	if err != nil || result.ID != testProject || references.partyCalls != 0 || store.createCalls != 0 {
		t.Fatalf("result=%#v error=%v refs=%d writes=%d", result, err, references.partyCalls, store.createCalls)
	}
}

func TestUpdateAllowsAnUnnumberedPackageToRemainUnnumbered(t *testing.T) {
	store := &storeStub{details: Details{Project: Project{ID: testProject, TenantID: testTenant, NodeType: NodeTypePackage, Revision: 1}}}
	service, _ := NewService(store, &referencesStub{})
	_, err := service.Update(context.Background(), Actor{TenantID: testTenant}, testProject, UpdateInput{
		CreateInput:      CreateInput{CommandID: testCommand, Name: "华东项目包", Summary: "更新说明", ProjectTypeKeys: []string{"design"}},
		ExpectedRevision: 1,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if store.updateCalls != 1 || store.updateInput.ProjectNumber != "" || store.updateInput.Name != "华东项目包" {
		t.Fatalf("update input=%#v calls=%d", store.updateInput, store.updateCalls)
	}
}

func TestUpdateStillRequiresNumberForAProject(t *testing.T) {
	store := &storeStub{details: Details{Project: Project{ID: testProject, TenantID: testTenant, NodeType: NodeTypeProject, Revision: 1}}}
	service, _ := NewService(store, &referencesStub{})
	_, err := service.Update(context.Background(), Actor{TenantID: testTenant}, testProject, UpdateInput{
		CreateInput: CreateInput{CommandID: testCommand, Name: "无编号项目"}, ExpectedRevision: 1,
	})
	if err == nil || store.updateCalls != 0 {
		t.Fatalf("error=%v update calls=%d", err, store.updateCalls)
	}
}

func TestApplyAssignmentValidatesPeopleBeforeWriting(t *testing.T) {
	store := &storeStub{}
	references := &referencesStub{peopleResult: ReferenceSnapshot{Revision: 3, DisplayName: "设计负责人"}}
	service, _ := NewService(store, references)
	result, err := service.ApplyAssignment(context.Background(), Actor{TenantID: testTenant}, testProject, AssignmentCommand{
		CommandID: testCommand, ExpectedRevision: 1, Operation: "assign", SubjectKind: "person", SubjectID: testSubject, SourceRevision: 3, RoleKey: RoleProductionOwner,
	})
	if err != nil {
		t.Fatalf("ApplyAssignment() error = %v", err)
	}
	if references.peopleCalls != 1 || store.assignmentCalls != 1 || result.Assignment.DisplaySnapshot != "设计负责人" || store.assignmentInput.EffectiveFrom != nil {
		t.Fatalf("unexpected validation/write result: refs=%d writes=%d input=%#v", references.peopleCalls, store.assignmentCalls, store.assignmentInput)
	}
}

func TestApplyAssignmentFailsClosedWhenPeopleUnavailable(t *testing.T) {
	store := &storeStub{}
	references := &referencesStub{peopleErr: ErrReferenceUnavailable}
	service, _ := NewService(store, references)
	_, err := service.ApplyAssignment(context.Background(), Actor{TenantID: testTenant}, testProject, AssignmentCommand{
		CommandID: testCommand, ExpectedRevision: 1, Operation: "assign", SubjectKind: "group", SubjectID: testSubject, SourceRevision: 1, RoleKey: RoleBusinessOwner,
	})
	if !errors.Is(err, ErrReferenceUnavailable) || store.assignmentCalls != 0 {
		t.Fatalf("error=%v writes=%d", err, store.assignmentCalls)
	}
}

func TestApplyResourceLinkValidatesTargetBeforeWriting(t *testing.T) {
	store := &storeStub{}
	references := &referencesStub{}
	service, _ := NewService(store, references)
	revision := int64(4)
	_, err := service.ApplyResourceLink(context.Background(), Actor{TenantID: testTenant}, testProject, ResourceLinkCommand{
		CommandID: testCommand, ExpectedRevision: 2, Operation: "link", TargetService: "contract-ledger", ResourceType: "contract", ResourceID: testSubject, SourceRevision: &revision, RelationKey: "primary-contract",
	})
	if err != nil || references.resourceCalls != 1 || store.resourceCalls != 1 {
		t.Fatalf("error=%v refs=%d writes=%d", err, references.resourceCalls, store.resourceCalls)
	}
}

func TestApplyPartyResourceLinkUsesExactPartyRevision(t *testing.T) {
	store := &storeStub{}
	references := &referencesStub{partyResult: PartySnapshot{PartyID: testSubject, SourceRevision: 4, Kind: "organization", LegalName: "施工方"}}
	service, _ := NewService(store, references)
	revision := int64(4)
	_, err := service.ApplyResourceLink(context.Background(), Actor{TenantID: testTenant}, testProject, ResourceLinkCommand{
		CommandID: testCommand, ExpectedRevision: 2, Operation: "link", TargetService: "party-directory", ResourceType: "party", ResourceID: testSubject, SourceRevision: &revision, RelationKey: "contractor-party",
	})
	if err != nil || references.partyCalls != 1 || references.resourceCalls != 0 || store.resourceCalls != 1 {
		t.Fatalf("error=%v party=%d resource=%d writes=%d", err, references.partyCalls, references.resourceCalls, store.resourceCalls)
	}
	withoutRevision := store.resourceCalls
	_, err = service.ApplyResourceLink(context.Background(), Actor{TenantID: testTenant}, testProject, ResourceLinkCommand{
		CommandID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", ExpectedRevision: 2, Operation: "link", TargetService: "party-directory", ResourceType: "party", ResourceID: testSubject, RelationKey: "contractor-party",
	})
	if err == nil || store.resourceCalls != withoutRevision {
		t.Fatalf("missing revision error=%v writes=%d", err, store.resourceCalls)
	}
}

func TestApplyResourceLinkFailsClosedWhenTargetUnavailable(t *testing.T) {
	store := &storeStub{}
	references := &referencesStub{resourceErr: ErrReferenceUnavailable}
	service, _ := NewService(store, references)
	_, err := service.ApplyResourceLink(context.Background(), Actor{TenantID: testTenant}, testProject, ResourceLinkCommand{
		CommandID: testCommand, ExpectedRevision: 2, Operation: "link", TargetService: "evidence-service", ResourceType: "evidence-file", ResourceID: testSubject, RelationKey: "brief",
	})
	if !errors.Is(err, ErrReferenceUnavailable) || store.resourceCalls != 0 {
		t.Fatalf("error=%v writes=%d", err, store.resourceCalls)
	}
}

func TestGetKeepsProjectReadableAndMarksUnavailableReferences(t *testing.T) {
	store := &storeStub{details: Details{
		Project:       Project{ID: testProject, TenantID: testTenant},
		ResourceLinks: []ResourceLink{{ID: testSubject, TargetService: "contract-ledger", ResourceType: "contract", ResourceID: testActorID}},
	}}
	references := &referencesStub{resourceErr: ErrReferenceUnavailable}
	service, _ := NewService(store, references)
	details, err := service.Get(context.Background(), testTenant, testProject, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(details.ReferenceStatuses) != 1 || details.ReferenceStatuses[0].Status != "unavailable" || details.Project.ID != testProject {
		t.Fatalf("details=%#v", details)
	}
}

func TestGetValidatesPartyResourceLinkWithoutGenericResourceLookup(t *testing.T) {
	revision := int64(3)
	store := &storeStub{details: Details{
		Project:       Project{ID: testProject, TenantID: testTenant},
		ResourceLinks: []ResourceLink{{ID: testSubject, TargetService: "party-directory", ResourceType: "party", ResourceID: testActorID, SourceRevision: &revision, RelationKey: "contractor-party"}},
	}}
	references := &referencesStub{partyErr: ErrReferenceUnavailable}
	service, _ := NewService(store, references)
	details, err := service.Get(context.Background(), testTenant, testProject, false)
	if err != nil || references.partyCalls != 1 || references.resourceCalls != 0 || len(details.ReferenceStatuses) != 1 || details.ReferenceStatuses[0].Status != "unavailable" {
		t.Fatalf("details=%#v party=%d resource=%d error=%v", details, references.partyCalls, references.resourceCalls, err)
	}
}

func TestCommercialSummaryReadsSourcesInParallelAndIsolatesFailures(t *testing.T) {
	contractRevision := int64(1)
	moneyRevision := int64(4)
	billingRevision := int64(2)
	store := &storeStub{details: Details{
		Project: Project{ID: testProject, TenantID: testTenant, Revision: 9},
		ResourceLinks: []ResourceLink{
			{ID: "66666666-6666-4666-8666-666666666661", TargetService: "contract-ledger", ResourceType: "contract", ResourceID: testActorID, RelationKey: "primary-contract", SourceRevision: &contractRevision},
			{ID: "66666666-6666-4666-8666-666666666662", TargetService: "money-flow-ledger", ResourceType: "obligation", ResourceID: testSubject, RelationKey: "receivable", SourceRevision: &moneyRevision},
			{ID: "66666666-6666-4666-8666-666666666663", TargetService: "billing-invoice", ResourceType: "invoice_registration", ResourceID: testCommand, RelationKey: "invoice", SourceRevision: &billingRevision},
			{ID: "66666666-6666-4666-8666-666666666664", TargetService: "evidence-service", ResourceType: "evidence-file", ResourceID: testCommand, RelationKey: "attachment"},
		},
	}}
	references := &commercialReferencesStub{
		started: make(chan string, 3), release: make(chan struct{}),
		values: map[string]ResourceReference{
			"contract-ledger/contract":             {SourceApplication: "contract-ledger", ID: testActorID, Revision: 2, Status: "active", DisplaySummary: "设计合同", Contract: &ContractCommercialFacts{Currency: "CNY"}},
			"billing-invoice/invoice_registration": {SourceApplication: "billing-invoice", ID: testCommand, Revision: 2, Status: "registered", DisplaySummary: "开票登记", Invoice: &InvoiceCommercialFacts{IssuedOn: "2026-09-27", AmountMinor: 120000, Currency: "CNY"}},
		},
		errors: map[string]error{"money-flow-ledger/obligation": ErrReferenceUnavailable},
	}
	service, _ := NewService(store, references)
	result := make(chan CommercialSummary, 1)
	failures := make(chan error, 1)
	go func() {
		value, err := service.CommercialSummary(context.Background(), testTenant, testProject)
		result <- value
		failures <- err
	}()
	started := map[string]bool{}
	for len(started) < 3 {
		select {
		case key := <-references.started:
			started[key] = true
		case <-time.After(time.Second):
			t.Fatalf("only %d source reads started before release: %#v", len(started), started)
		}
	}
	close(references.release)
	value := <-result
	if err := <-failures; err != nil {
		t.Fatal(err)
	}
	if value.ProjectID != testProject || value.ProjectRevision != 9 || len(value.Segments) != 3 {
		t.Fatalf("summary=%#v", value)
	}
	if value.Segments[0].State != CommercialSegmentRevisionChanged || value.Segments[0].Contract == nil || value.Segments[0].SourceRevision == nil || *value.Segments[0].SourceRevision != 2 {
		t.Fatalf("contract segment=%#v", value.Segments[0])
	}
	if value.Segments[1].State != CommercialSegmentUnavailable || value.Segments[1].ErrorCode != "source_unavailable" || value.Segments[1].Obligation != nil {
		t.Fatalf("money segment=%#v", value.Segments[1])
	}
	if value.Segments[2].State != CommercialSegmentAvailable || value.Segments[2].Invoice == nil || value.Segments[2].Invoice.AmountMinor != 120000 {
		t.Fatalf("billing segment=%#v", value.Segments[2])
	}
}

func TestMaterialsOnlyComposeActiveDirectEvidenceLinksAndIsolateFailures(t *testing.T) {
	availableID := "66666666-6666-4666-8666-666666666661"
	staleID := "66666666-6666-4666-8666-666666666662"
	forbiddenID := "66666666-6666-4666-8666-666666666663"
	unavailableID := "66666666-6666-4666-8666-666666666664"
	invalidID := "66666666-6666-4666-8666-666666666665"
	linkedOne, linkedTwo := int64(1), int64(2)
	unlinkedAt := time.Now().UTC()
	store := &storeStub{details: Details{
		Project: Project{ID: testProject, TenantID: testTenant, Revision: 9},
		ResourceLinks: []ResourceLink{
			{ID: availableID, TargetService: "evidence-service", ResourceType: "evidence-file", ResourceID: availableID, RelationKey: "brief", SourceRevision: &linkedOne},
			{ID: staleID, TargetService: "evidence-service", ResourceType: "evidence-file", ResourceID: staleID, RelationKey: "result", SourceRevision: &linkedOne},
			{ID: forbiddenID, TargetService: "evidence-service", ResourceType: "evidence-file", ResourceID: forbiddenID, RelationKey: "private"},
			{ID: unavailableID, TargetService: "evidence-service", ResourceType: "evidence-file", ResourceID: unavailableID, RelationKey: "offline"},
			{ID: invalidID, TargetService: "evidence-service", ResourceType: "evidence-file", ResourceID: invalidID, RelationKey: "missing"},
			{ID: testActorID, TargetService: "contract-ledger", ResourceType: "contract", ResourceID: testActorID, RelationKey: "not-material"},
			{ID: testCommand, TargetService: "evidence-service", ResourceType: "evidence-file", ResourceID: testCommand, RelationKey: "old", UnlinkedAt: &unlinkedAt},
		},
	}}
	metadata := func(id, name string, revision int64) ResourceReference {
		return ResourceReference{
			SourceApplication: "evidence-service", ID: id, TenantID: testTenant, ResourceType: "evidence-file",
			DisplaySummary: name, Status: "available", Revision: revision,
			Evidence: &EvidenceFileFacts{Version: revision, FileName: name, SizeBytes: 42, MediaType: "application/pdf", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		}
	}
	references := &materialReferencesStub{
		values: map[string]ResourceReference{availableID: metadata(availableID, "立项资料.pdf", 1), staleID: metadata(staleID, "成果.pdf", linkedTwo)},
		errors: map[string]error{forbiddenID: ErrReferenceForbidden, unavailableID: ErrReferenceUnavailable, invalidID: ErrReferenceInvalid},
	}
	service, _ := NewService(store, references)
	value, err := service.Materials(context.Background(), testTenant, testProject)
	if err != nil || value.ProjectID != testProject || value.ProjectRevision != 9 || value.State != MaterialCollectionAvailable || len(value.Segments) != 5 {
		t.Fatalf("materials=%#v error=%v", value, err)
	}
	wantStates := []string{MaterialSegmentAvailable, MaterialSegmentRevisionChanged, MaterialSegmentForbidden, MaterialSegmentUnavailable, MaterialSegmentInvalid}
	for index, want := range wantStates {
		if value.Segments[index].State != want {
			t.Fatalf("segment[%d]=%#v want state=%s", index, value.Segments[index], want)
		}
	}
	if value.Segments[0].FileName != "立项资料.pdf" || value.Segments[0].SourceDigest == "" || value.Segments[0].SourceRevision == nil || *value.Segments[0].SourceRevision != 1 {
		t.Fatalf("available segment=%#v", value.Segments[0])
	}
	if value.Segments[1].LinkedRevision == nil || *value.Segments[1].LinkedRevision != 1 || value.Segments[1].SourceRevision == nil || *value.Segments[1].SourceRevision != 2 {
		t.Fatalf("stale segment=%#v", value.Segments[1])
	}
}

func TestMaterialsReturnExplicitEmptyWithoutResourceReaderCalls(t *testing.T) {
	store := &storeStub{details: Details{Project: Project{ID: testProject, TenantID: testTenant, Revision: 2}}}
	service, _ := NewService(store, &referencesStub{})
	value, err := service.Materials(context.Background(), testTenant, testProject)
	if err != nil || value.State != MaterialCollectionEmpty || len(value.Segments) != 0 {
		t.Fatalf("materials=%#v error=%v", value, err)
	}
}

func TestMaterialsComposeBusinessSourceReferencesWithIndependentEvidenceReads(t *testing.T) {
	engagementID := "66666666-6666-4666-8666-666666666671"
	firstFileID := "66666666-6666-4666-8666-666666666672"
	secondFileID := "66666666-6666-4666-8666-666666666673"
	thirdFileID := "66666666-6666-4666-8666-666666666675"
	linkedSourceRevision := int64(5)
	store := &storeStub{details: Details{
		Project: Project{ID: testProject, TenantID: testTenant, Revision: 11},
		ResourceLinks: []ResourceLink{{
			ID: "66666666-6666-4666-8666-666666666674", TargetService: "business-engagement",
			ResourceType: "business-engagement", ResourceID: engagementID,
			RelationKey: "engagement-materials", SourceRevision: &linkedSourceRevision,
		}},
	}}
	digest := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	evidence := func(id, name string, revision int64) ResourceReference {
		return ResourceReference{
			SourceApplication: "evidence-service", ID: id, TenantID: testTenant, ResourceType: "evidence-file",
			Status: "available", Revision: revision,
			Evidence: &EvidenceFileFacts{Version: revision, FileName: name, SizeBytes: 64, MediaType: "application/pdf", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		}
	}
	references := &materialReferencesStub{values: map[string]ResourceReference{
		"business-engagement/business-engagement/" + engagementID: {
			SourceApplication: "business-engagement", ID: engagementID, TenantID: testTenant,
			ResourceType: "business-engagement", Status: "ready", Revision: 5,
			MaterialSource: &MaterialSourceFacts{ProjectID: testProject, Digest: digest, Items: []MaterialSourceItem{
				{RelationKey: "award_document", FileID: firstFileID, Version: 2, EvidenceSourceRevision: 2, DisplayName: "中标通知书.pdf"},
				{RelationKey: "bid_document", FileID: secondFileID, Version: 1, EvidenceSourceRevision: 1, DisplayName: "投标文件.pdf"},
				{RelationKey: "tender_document", FileID: thirdFileID, Version: 1, EvidenceSourceRevision: 1, DisplayName: "招标文件.pdf"},
			}},
		},
		"evidence-service/evidence-file/" + firstFileID:  evidence(firstFileID, "中标通知书.pdf", 2),
		"evidence-service/evidence-file/" + secondFileID: evidence(secondFileID, "投标文件-v2.pdf", 2),
	}, errors: map[string]error{"evidence-service/evidence-file/" + thirdFileID: ErrReferenceUnavailable}}
	service, _ := NewService(store, references)
	value, err := service.Materials(context.Background(), testTenant, testProject)
	if err != nil || value.State != MaterialCollectionAvailable || len(value.Segments) != 3 {
		t.Fatalf("materials=%#v error=%v", value, err)
	}
	if value.Segments[0].State != MaterialSegmentAvailable || value.Segments[0].SourceApplication != "business-engagement" ||
		value.Segments[0].EvidenceFileID != firstFileID || value.Segments[0].SourceDigest != digest || value.Segments[0].EvidenceDigest == "" ||
		value.Segments[0].SourceRelationKey != "engagement-materials" {
		t.Fatalf("available indirect segment=%#v", value.Segments[0])
	}
	if value.Segments[1].State != MaterialSegmentRevisionChanged || value.Segments[1].EvidenceLinkedRevision == nil ||
		*value.Segments[1].EvidenceLinkedRevision != 1 || value.Segments[1].EvidenceSourceRevision == nil || *value.Segments[1].EvidenceSourceRevision != 2 {
		t.Fatalf("changed indirect segment=%#v", value.Segments[1])
	}
	if value.Segments[2].State != MaterialSegmentUnavailable || value.Segments[2].EvidenceFileID != thirdFileID {
		t.Fatalf("unavailable evidence segment=%#v", value.Segments[2])
	}
	if len(references.calls) != 4 || references.calls[0] != "business-engagement/business-engagement/"+engagementID {
		t.Fatalf("reference calls=%#v", references.calls)
	}
}

func TestMaterialsKeepDirectEvidenceWhenBusinessSourceIsForbidden(t *testing.T) {
	directID := "66666666-6666-4666-8666-666666666676"
	engagementID := "66666666-6666-4666-8666-666666666677"
	store := &storeStub{details: Details{Project: Project{ID: testProject, TenantID: testTenant, Revision: 5}, ResourceLinks: []ResourceLink{
		{ID: "66666666-6666-4666-8666-666666666678", TargetService: "evidence-service", ResourceType: "evidence-file", ResourceID: directID, RelationKey: "direct"},
		{ID: "66666666-6666-4666-8666-666666666679", TargetService: "business-engagement", ResourceType: "business-engagement", ResourceID: engagementID, RelationKey: "engagement-materials"},
	}}}
	references := &materialReferencesStub{
		values: map[string]ResourceReference{directID: {
			SourceApplication: "evidence-service", ID: directID, TenantID: testTenant, ResourceType: "evidence-file", Status: "available", Revision: 1,
			Evidence: &EvidenceFileFacts{Version: 1, FileName: "直接资料.pdf", SizeBytes: 32, MediaType: "application/pdf", SHA256: strings.Repeat("a", 64)},
		}},
		errors: map[string]error{"business-engagement/business-engagement/" + engagementID: ErrReferenceForbidden},
	}
	service, _ := NewService(store, references)
	value, err := service.Materials(context.Background(), testTenant, testProject)
	if err != nil || len(value.Segments) != 2 || value.Segments[0].State != MaterialSegmentAvailable || value.Segments[1].State != MaterialSegmentForbidden {
		t.Fatalf("materials=%#v error=%v", value, err)
	}
}

func TestMaterialsRejectCrossProjectBusinessSourceWithoutReadingEvidence(t *testing.T) {
	engagementID := "66666666-6666-4666-8666-666666666681"
	fileID := "66666666-6666-4666-8666-666666666682"
	store := &storeStub{details: Details{Project: Project{ID: testProject, TenantID: testTenant, Revision: 4}, ResourceLinks: []ResourceLink{{
		ID: "66666666-6666-4666-8666-666666666683", TargetService: "business-engagement", ResourceType: "business-engagement",
		ResourceID: engagementID, RelationKey: "engagement-materials",
	}}}}
	references := &materialReferencesStub{values: map[string]ResourceReference{
		"business-engagement/business-engagement/" + engagementID: {
			SourceApplication: "business-engagement", ID: engagementID, Revision: 2, Status: "ready",
			MaterialSource: &MaterialSourceFacts{ProjectID: testActorID, Digest: strings.Repeat("b", 64), Items: []MaterialSourceItem{{FileID: fileID, Version: 1, EvidenceSourceRevision: 1}}},
		},
	}}
	service, _ := NewService(store, references)
	value, err := service.Materials(context.Background(), testTenant, testProject)
	if err != nil || len(value.Segments) != 1 || value.Segments[0].State != MaterialSegmentInvalid || len(references.calls) != 1 {
		t.Fatalf("materials=%#v calls=%#v error=%v", value, references.calls, err)
	}
}

func TestReferenceReadsExposeOnlyStableProjection(t *testing.T) {
	linkedRevision := int64(5)
	unlinkedAt := time.Now().UTC()
	store := &storeStub{details: Details{
		Project:     Project{ID: testProject, TenantID: testTenant, ProjectNumber: "OA-001", Name: "示范设计项目", Status: StatusActive, Revision: 7, ProjectTypeKeys: []string{"design"}},
		Assignments: []Assignment{{ID: testSubject, TenantID: testTenant, ProjectID: testProject, RoleKey: RoleProductionOwner, DisplaySnapshot: "设计组", CreatedProjectRevision: 3}},
		ResourceLinks: []ResourceLink{
			{TargetService: "contract-ledger", ResourceType: "contract", ResourceID: testActorID, SourceRevision: &linkedRevision, RelationKey: "approval-source"},
			{TargetService: "contract-ledger", ResourceType: "settlement", ResourceID: testCommand, SourceRevision: &linkedRevision, RelationKey: "old-source", UnlinkedAt: &unlinkedAt},
		},
	}}
	service, _ := NewService(store, &referencesStub{})
	project, err := service.ReferenceProject(context.Background(), testTenant, testProject)
	if err != nil || project.ID != testProject || project.ResourceType != "project" || project.ProjectNumber != "OA-001" || project.DisplaySummary != "示范设计项目" || project.Revision != 7 {
		t.Fatalf("project=%#v err=%v", project, err)
	}
	if len(project.ResourceLinks) != 1 || project.ResourceLinks[0].ResourceID != testActorID || project.ResourceLinks[0].SourceRevision == nil || *project.ResourceLinks[0].SourceRevision != 5 {
		t.Fatalf("active resource links=%#v", project.ResourceLinks)
	}
	assignment, err := service.ReferenceAssignment(context.Background(), testTenant, testProject, testSubject)
	if err != nil || assignment.Status != "active" || assignment.Revision != 3 || assignment.DisplayName != "设计组" {
		t.Fatalf("assignment=%#v err=%v", assignment, err)
	}
}
