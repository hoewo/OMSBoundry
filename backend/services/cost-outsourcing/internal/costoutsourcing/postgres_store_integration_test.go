//go:build integration

package costoutsourcing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"sync"
	"testing"

	contract "omsbizfoundry/payable-suggestion-contract"

	"github.com/jackc/pgx/v5/pgxpool"
	"omsbizfoundry/cost-outsourcing/internal/database"
)

func TestPostgresCostAndOutsourcingLifecycle(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	actor, resolver := integrationFixture()
	service, _ := NewService(store, resolver)

	costInput := RecordCostInput{CommandID: id(10), ProjectRef: ProjectRef{ID: resolver.project.ID, Revision: resolver.project.Revision}, Category: CategoryVehicle, OccurredOn: "2026-09-28", Currency: "CNY", AmountMinor: 12345, DistanceMeters: 6789, Description: "项目现场车辆成本", EvidenceRefs: []EvidenceRef{resolver.evidence.EvidenceRef}}
	cost, err := service.RecordCost(ctx, actor, costInput)
	if err != nil {
		t.Fatal(err)
	}
	if cost.Revision != 1 || cost.AmountMinor != 12345 || cost.Project.Revision != 7 || cost.Evidence[0].Version != 2 {
		t.Fatalf("unexpected cost: %#v", cost)
	}
	replayed, err := service.RecordCost(ctx, actor, costInput)
	if err != nil || replayed.ID != cost.ID || replayed.Revision != 1 {
		t.Fatalf("bad cost replay: %#v %v", replayed, err)
	}
	conflict := costInput
	conflict.Description = "different payload"
	if _, err = service.RecordCost(ctx, actor, conflict); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("command conflict expected, got %v", err)
	}
	cost2, err := service.CorrectCost(ctx, actor, cost.ID, CorrectCostInput{CommandID: id(11), ExpectedRevision: 1, Reason: "补录停车费", Category: CategoryVehicle, OccurredOn: "2026-09-28", Currency: "CNY", AmountMinor: 12445, DistanceMeters: 6789, Description: "项目现场车辆及停车成本", EvidenceRefs: []EvidenceRef{resolver.evidence.EvidenceRef}})
	if err != nil || cost2.Revision != 2 || cost2.AmountMinor != 12445 {
		t.Fatalf("bad cost correction: %#v %v", cost2, err)
	}
	if _, err = service.CorrectCost(ctx, actor, cost.ID, CorrectCostInput{CommandID: id(12), ExpectedRevision: 1, Reason: "过期写入", Category: CategoryVehicle, OccurredOn: "2026-09-28", Currency: "CNY", AmountMinor: 1, DistanceMeters: 1, Description: "stale"}); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale cost correction expected, got %v", err)
	}
	exactResourceActor := actor
	exactResourceActor.ProjectIDs = nil
	exactResourceActor.ResourceScopes = []ResourceScope{{Type: "cost_entry", ID: cost.ID}}
	if exact, exactErr := service.GetCost(ctx, exactResourceActor, cost.ID); exactErr != nil || exact.ID != cost.ID {
		t.Fatalf("exact resource scope must read its cost: %#v %v", exact, exactErr)
	}
	missingEvidenceAction := exactResourceActor
	missingEvidenceAction.Actions = []string{ActionCostCorrect}
	if _, err = service.CorrectCost(ctx, missingEvidenceAction, cost.ID, CorrectCostInput{CommandID: id(13), ExpectedRevision: 2, Reason: "移除凭证", Category: cost2.Category, OccurredOn: cost2.OccurredOn, Currency: cost2.Currency, AmountMinor: cost2.AmountMinor, DistanceMeters: cost2.DistanceMeters, Description: cost2.Description}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("removing evidence without evidence capability must fail: %v", err)
	}

	contract := resolver.contract.ContractRef
	engagementInput := RecordEngagementInput{CommandID: id(20), ProjectRef: ProjectRef{ID: resolver.project.ID, Revision: 7}, ProviderRef: PartyRef{ID: resolver.party.ID, Revision: 5}, ContactName: "李工", ContactPhone: "13800000000", Scope: "桥梁测量协作", Currency: "CNY", AmountMinor: 500000, ContractRef: &contract, EvidenceRefs: []EvidenceRef{resolver.evidence.EvidenceRef}, Note: "按成果验收"}
	engagement, err := service.RecordEngagement(ctx, actor, engagementInput)
	if err != nil {
		t.Fatal(err)
	}
	if engagement.Revision != 1 || engagement.Provider.Revision != 5 || engagement.Contract.Revision != 9 || engagement.AmountMinor != 500000 || engagement.ContactName != "李工" || engagement.ContactPhone != "13800000000" {
		t.Fatalf("unexpected engagement: %#v", engagement)
	}
	assessment, err := service.Assess(ctx, actor, engagement.ID, AssessInput{CommandID: id(21), ExpectedEngagementRevision: 1, ExpectedAssessmentRevision: 0, Score: 4, Reason: "成果完整"})
	if err != nil || assessment.Revision != 1 || assessment.RuleKey != AssessmentRuleReferenceOAFivePointV1 {
		t.Fatalf("bad assessment: %#v %v", assessment, err)
	}
	engagement2, err := service.CorrectEngagement(ctx, actor, engagement.ID, CorrectEngagementInput{CommandID: id(22), ExpectedRevision: 1, Reason: "补充工作范围", ProviderRef: engagementInput.ProviderRef, ContactName: "王工", ContactPhone: "13900000000", Scope: "桥梁测量及复核协作", Currency: "CNY", AmountMinor: 520000, ContractRef: &contract, EvidenceRefs: engagementInput.EvidenceRefs, Note: "按成果验收"})
	if err != nil || engagement2.Revision != 2 || engagement2.AmountMinor != 520000 || engagement2.ContactName != "王工" || engagement2.ContactPhone != "13900000000" {
		t.Fatalf("bad engagement correction: %#v %v", engagement2, err)
	}
	assessment2, err := service.Assess(ctx, actor, engagement.ID, AssessInput{CommandID: id(23), ExpectedEngagementRevision: 2, ExpectedAssessmentRevision: 1, Score: 5, Reason: "复核通过"})
	if err != nil || assessment2.ID != assessment.ID || assessment2.Revision != 2 {
		t.Fatalf("bad assessment correction: %#v %v", assessment2, err)
	}
	withdrawn, err := service.WithdrawEngagement(ctx, actor, engagement.ID, WithdrawInput{CommandID: id(24), ExpectedRevision: 2, Reason: "书面终止"})
	if err != nil || withdrawn.State != StateWithdrawn || withdrawn.Revision != 3 {
		t.Fatalf("bad withdrawal: %#v %v", withdrawn, err)
	}
	if _, err = service.Assess(ctx, actor, engagement.ID, AssessInput{CommandID: id(25), ExpectedEngagementRevision: 3, ExpectedAssessmentRevision: 2, Score: 1, Reason: "不可追加"}); !errors.Is(err, ErrWithdrawn) {
		t.Fatalf("withdrawn engagement must reject assessment: %v", err)
	}

	costHistory, err := service.CostHistory(ctx, actor, cost.ID)
	if err != nil || len(costHistory) != 2 || costHistory[0].Snapshot.AmountMinor != 12345 || costHistory[1].Snapshot.AmountMinor != 12445 {
		t.Fatalf("bad cost history: %#v %v", costHistory, err)
	}
	engagementHistory, err := service.EngagementHistory(ctx, actor, engagement.ID)
	if err != nil || len(engagementHistory) != 3 || engagementHistory[0].Snapshot.AmountMinor != 500000 || engagementHistory[0].Snapshot.ContactName != "李工" || engagementHistory[1].Snapshot.ContactName != "王工" || engagementHistory[2].Snapshot.State != StateWithdrawn {
		t.Fatalf("bad engagement history: %#v %v", engagementHistory, err)
	}
	assessmentHistory, err := service.AssessmentHistory(ctx, actor, engagement.ID)
	if err != nil || len(assessmentHistory) != 2 || assessmentHistory[0].Score != 4 || assessmentHistory[1].Score != 5 {
		t.Fatalf("bad assessment history: %#v %v", assessmentHistory, err)
	}
	latestAssessments, err := service.ListLatestAssessments(ctx, actor, resolver.project.ID)
	if err != nil || len(latestAssessments) != 1 || latestAssessments[0].EngagementID != engagement.ID || latestAssessments[0].Revision != 2 || latestAssessments[0].Score != 5 {
		t.Fatalf("bad latest assessment projection: %#v %v", latestAssessments, err)
	}

	wrongProjectActor := actor
	wrongProjectActor.ProjectIDs = []string{id(99)}
	if _, err = service.GetCost(ctx, wrongProjectActor, cost.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-project read must be forbidden: %v", err)
	}
	wrongTenantActor := actor
	wrongTenantActor.TenantID = id(98)
	if _, err = service.GetCost(ctx, wrongTenantActor, cost.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant read must look not found: %v", err)
	}

	if _, err = pool.Exec(ctx, `UPDATE cost_entry_revisions SET reason='tampered' WHERE tenant_id=$1 AND cost_entry_id=$2 AND revision=1`, actor.TenantID, cost.ID); err == nil {
		t.Fatal("cost revision mutation must be rejected")
	}
	if _, err = pool.Exec(ctx, `DELETE FROM outsourcing_engagements WHERE tenant_id=$1 AND id=$2`, actor.TenantID, engagement.ID); err == nil {
		t.Fatal("engagement physical delete must be rejected")
	}
}

func TestDependencyAndAuthorizationFailuresLeaveNoFacts(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	actor, resolver := integrationFixture()
	service, _ := NewService(store, resolver)
	input := RecordCostInput{CommandID: id(30), ProjectRef: ProjectRef{ID: resolver.project.ID, Revision: 7}, Category: CategoryOther, OccurredOn: "2026-09-28", Currency: "CNY", AmountMinor: 1, Description: "依赖失败测试", EvidenceRefs: []EvidenceRef{resolver.evidence.EvidenceRef}}

	unauthorized := actor
	unauthorized.Actions = []string{ActionCostCreate}
	if _, err := service.RecordCost(ctx, unauthorized, input); !errors.Is(err, ErrForbidden) {
		t.Fatalf("missing evidence capability must fail: %v", err)
	}
	resolver.failEvidence = ErrReferenceUnavailable
	if _, err := service.RecordCost(ctx, actor, input); !errors.Is(err, ErrReferenceUnavailable) {
		t.Fatalf("dependency failure expected, got %v", err)
	}
	var current, revisions, commands, audits int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM cost_entries),(SELECT count(*) FROM cost_entry_revisions),(SELECT count(*) FROM cost_outsourcing_commands),(SELECT count(*) FROM cost_outsourcing_audit_events)`).Scan(&current, &revisions, &commands, &audits); err != nil {
		t.Fatal(err)
	}
	if current != 0 || revisions != 0 || commands != 0 || audits != 0 {
		t.Fatalf("failed requests created partial facts: %d/%d/%d/%d", current, revisions, commands, audits)
	}
	resolver.failEvidence = nil
	created, err := service.RecordCost(ctx, actor, input)
	if err != nil || created.Revision != 1 {
		t.Fatalf("recovery command must commit once: %#v %v", created, err)
	}
	resolver.failEvidence = ErrReferenceUnavailable
	replayed, err := service.RecordCost(ctx, actor, input)
	if err != nil || replayed.ID != created.ID {
		t.Fatalf("successful replay must not call failed dependency: %#v %v", replayed, err)
	}
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM cost_entries),(SELECT count(*) FROM cost_entry_revisions),(SELECT count(*) FROM cost_outsourcing_commands),(SELECT count(*) FROM cost_outsourcing_audit_events)`).Scan(&current, &revisions, &commands, &audits); err != nil {
		t.Fatal(err)
	}
	if current != 1 || revisions != 1 || commands != 1 || audits != 1 {
		t.Fatalf("recovery/replay must commit one fact: %d/%d/%d/%d", current, revisions, commands, audits)
	}
}

func TestConcurrentCostReplayProducesOneFact(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	actor, resolver := integrationFixture()
	service, _ := NewService(store, resolver)
	input := RecordCostInput{CommandID: id(40), ProjectRef: ProjectRef{ID: resolver.project.ID, Revision: 7}, Category: CategoryOther, OccurredOn: "2026-09-28", Currency: "CNY", AmountMinor: 99, Description: "并发幂等测试"}
	const workers = 8
	ids, errs := make(chan string, workers), make(chan error, workers)
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			value, err := service.RecordCost(ctx, actor, input)
			if err != nil {
				errs <- err
				return
			}
			ids <- value.ID
		}()
	}
	group.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	first, count := "", 0
	for value := range ids {
		count++
		if first == "" {
			first = value
		} else if value != first {
			t.Fatalf("different ids: %s %s", first, value)
		}
	}
	if count != workers {
		t.Fatalf("got %d successful workers", count)
	}
	var current, revisions, commands, audits int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM cost_entries),(SELECT count(*) FROM cost_entry_revisions),(SELECT count(*) FROM cost_outsourcing_commands),(SELECT count(*) FROM cost_outsourcing_audit_events)`).Scan(&current, &revisions, &commands, &audits); err != nil {
		t.Fatal(err)
	}
	if current != 1 || revisions != 1 || commands != 1 || audits != 1 {
		t.Fatalf("expected one fact, got %d/%d/%d/%d", current, revisions, commands, audits)
	}
}

type integrationResolver struct {
	project      ProjectReference
	party        PartyReference
	owner        PartyReference
	contract     ContractReference
	evidence     EvidenceReference
	failEvidence error
}

func (r *integrationResolver) ResolveProject(context.Context, string, ProjectRef) (ProjectReference, error) {
	return r.project, nil
}

func (r *integrationResolver) ResolveParty(_ context.Context, _ string, ref PartyRef) (PartyReference, error) {
	if ref.ID == r.owner.ID {
		return r.owner, nil
	}
	return r.party, nil
}
func (r *integrationResolver) ResolveContract(context.Context, string, ProjectRef, PartyRef, ContractRef) (ContractReference, error) {
	return r.contract, nil
}
func (r *integrationResolver) ResolveEvidence(context.Context, string, EvidenceRef) (EvidenceReference, error) {
	if r.failEvidence != nil {
		return EvidenceReference{}, r.failEvidence
	}
	return r.evidence, nil
}

func integrationFixture() (Actor, *integrationResolver) {
	tenantID, actorID, projectID, partyID := id(1), id(2), id(3), id(4)
	project := ProjectReference{ProjectSnapshot: ProjectSnapshot{ID: projectID, Revision: 7, Code: "P-001", DisplayName: "道路设计项目"}, TenantID: tenantID, Status: "active"}
	party := PartyReference{PartySnapshot: PartySnapshot{ID: partyID, Revision: 5, Kind: "organization", LegalName: "协作单位甲"}, TenantID: tenantID, Status: "active"}
	contractRef := ContractRef{Application: "contract-ledger", ResourceType: "contract", ResourceID: id(5), Revision: 9}
	contract := ContractReference{ContractSnapshot: ContractSnapshot{ContractRef: contractRef, ProjectID: projectID, PartyID: partyID, State: "active", Currency: "CNY", AmountMinor: 500000, Digest: "sha256:contract-9", DisplaySummary: "测量协作合同 revision 9"}, TenantID: tenantID, Eligible: true}
	evidenceRef := EvidenceRef{FileID: id(6), Version: 2, Slot: "supporting_document"}
	evidence := EvidenceReference{EvidenceSnapshot: EvidenceSnapshot{EvidenceRef: evidenceRef, Digest: "sha256:evidence-v2", DisplayName: "receipt.pdf"}, TenantID: tenantID, Status: "available"}
	owner := PartyReference{PartySnapshot: PartySnapshot{ID: id(7), Revision: 3, Kind: "organization", LegalName: "道路设计院"}, TenantID: tenantID, Status: "active"}
	actions := []string{ActionCostCreate, ActionCostRead, ActionCostAmountRead, ActionCostCorrect, ActionCostWithdraw, ActionCostEvidenceManage, ActionCostHistory, ActionEngageCreate, ActionEngageRead, ActionEngageAmountRead, ActionEngageCorrect, ActionEngageWithdraw, ActionEngageAssess, ActionEngageEvidenceManage, ActionEngageHistory, ActionSuggestionCreate, ActionSuggestionRead, ActionSuggestionWithdraw}
	actor := Actor{ID: actorID, TenantID: tenantID, Kind: "user", CredentialID: "cred-1", ProjectIDs: []string{projectID}, Actions: actions}
	return actor, &integrationResolver{project: project, party: party, owner: owner, contract: contract, evidence: evidence}
}

type integrationMoney struct {
	fail   bool
	intake PayableIntake
}

func (m *integrationMoney) SubmitPayableSuggestion(_ context.Context, tenantID, suggestionID string, value PayableSubmission) (PayableIntake, error) {
	if m.fail {
		return PayableIntake{}, ErrTargetUnavailable
	}
	if m.intake.Revision == 0 {
		m.intake = PayableIntake{SuggestionID: suggestionID, TenantID: tenantID, RequestDigest: value.RequestDigest, Status: contract.StatusPending, Revision: 1}
	}
	return m.intake, nil
}
func (m *integrationMoney) GetPayableSuggestion(context.Context, string, string) (PayableIntake, error) {
	if m.fail {
		return PayableIntake{}, ErrTargetUnavailable
	}
	return m.intake, nil
}
func (m *integrationMoney) WithdrawPayableSuggestion(_ context.Context, tenantID, suggestionID, _ string, expected int64, reason string) (PayableIntake, error) {
	if m.fail || expected != m.intake.Revision {
		return PayableIntake{}, ErrTargetUnavailable
	}
	m.intake = PayableIntake{SuggestionID: suggestionID, TenantID: tenantID, RequestDigest: m.intake.RequestDigest, Status: contract.StatusWithdrawn, Revision: expected + 1, DecisionReason: reason}
	return m.intake, nil
}

func TestPayableSuggestionPersistsBeforeTargetAndSynchronizesDecision(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	actor, resolver := integrationFixture()
	money := &integrationMoney{fail: true}
	service, _ := NewService(store, resolver, money)
	engagement, err := service.RecordEngagement(ctx, actor, RecordEngagementInput{CommandID: id(60), ProjectRef: ProjectRef{ID: resolver.project.ID, Revision: resolver.project.Revision}, ProviderRef: PartyRef{ID: resolver.party.ID, Revision: resolver.party.Revision}, Scope: "桥梁测量协作", Currency: "CNY", AmountMinor: 880000, Note: "验收后付款"})
	if err != nil {
		t.Fatal(err)
	}
	input := ProposeSuggestionInput{CommandID: id(61), SourceType: contract.SourceEngagement, SourceID: engagement.ID, SourceRevision: engagement.Revision, LedgerOwnerRef: PartyRef{ID: resolver.owner.ID, Revision: resolver.owner.Revision}, DueOn: "2026-10-31", Purpose: "桥梁测量协作应付款"}
	created, err := service.ProposePayableSuggestion(ctx, actor, input)
	if err != nil || created.State != SuggestionTargetUnavailable || created.Revision != 1 || created.TargetRevision != 0 {
		t.Fatalf("target failure must leave retryable local fact: %#v err=%v", created, err)
	}
	money.fail = false
	synced, err := service.SynchronizePayableSuggestion(ctx, actor, created.ID, SuggestionCommandInput{CommandID: id(62), ExpectedRevision: 1, Reason: "Money 恢复后重试"})
	if err != nil || synced.State != SuggestionProposed || synced.Revision != 2 || synced.TargetRevision != 1 {
		t.Fatalf("submission recovery failed: %#v err=%v", synced, err)
	}
	replayed, err := service.SynchronizePayableSuggestion(ctx, actor, created.ID, SuggestionCommandInput{CommandID: id(62), ExpectedRevision: 1, Reason: "Money 恢复后重试"})
	if err != nil || replayed.Revision != 2 || replayed.ID != created.ID {
		t.Fatalf("synchronization replay failed: %#v err=%v", replayed, err)
	}
	money.intake = PayableIntake{SuggestionID: created.ID, TenantID: actor.TenantID, RequestDigest: created.RequestDigest, Status: contract.StatusAccepted, Revision: 2, Obligation: &ObligationReference{ID: id(63), Revision: 1}, DecisionReason: "资金负责人确认"}
	accepted, err := service.SynchronizePayableSuggestion(ctx, actor, created.ID, SuggestionCommandInput{CommandID: id(64), ExpectedRevision: 2, Reason: "读取 Money 独立决定"})
	if err != nil || accepted.State != SuggestionAccepted || accepted.Revision != 3 || accepted.Obligation == nil || accepted.Obligation.ID != id(63) {
		t.Fatalf("decision synchronization failed: %#v err=%v", accepted, err)
	}
	history, err := service.PayableSuggestionHistory(ctx, actor, created.ID)
	if err != nil || len(history) != 3 || history[0].Snapshot.State != SuggestionTargetUnavailable || history[2].Snapshot.State != SuggestionAccepted {
		t.Fatalf("unexpected suggestion history: %#v err=%v", history, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE payable_suggestion_revisions SET reason='tampered' WHERE tenant_id=$1 AND suggestion_id=$2 AND revision=1`, actor.TenantID, created.ID); err == nil {
		t.Fatal("payable suggestion history mutation must be rejected")
	}

	money.intake = PayableIntake{}
	rejectedEngagement, err := service.RecordEngagement(ctx, actor, RecordEngagementInput{CommandID: id(65), ProjectRef: ProjectRef{ID: resolver.project.ID, Revision: resolver.project.Revision}, ProviderRef: PartyRef{ID: resolver.party.ID, Revision: resolver.party.Revision}, Scope: "现场踏勘协作", Currency: "CNY", AmountMinor: 330000, Note: "独立拒绝路径"})
	if err != nil {
		t.Fatal(err)
	}
	rejectInput := input
	rejectInput.CommandID = id(66)
	rejectInput.SourceID = rejectedEngagement.ID
	rejectInput.SourceRevision = rejectedEngagement.Revision
	rejectInput.Purpose = "独立外协来源的拒绝验证"
	rejectable, err := service.ProposePayableSuggestion(ctx, actor, rejectInput)
	if err != nil || rejectable.State != SuggestionProposed || rejectable.Revision != 2 || rejectable.TargetRevision != 1 {
		t.Fatalf("rejection candidate submission failed: %#v err=%v", rejectable, err)
	}
	money.intake = PayableIntake{SuggestionID: rejectable.ID, TenantID: actor.TenantID, RequestDigest: rejectable.RequestDigest, Status: contract.StatusRejected, Revision: 2, DecisionReason: "Money 独立拒绝承担"}
	rejected, err := service.SynchronizePayableSuggestion(ctx, actor, rejectable.ID, SuggestionCommandInput{CommandID: id(67), ExpectedRevision: 2, Reason: "同步 Money 拒绝决定"})
	if err != nil || rejected.State != SuggestionRejected || rejected.Revision != 3 || rejected.TargetRevision != 2 || rejected.Obligation != nil || rejected.DecisionReason != "Money 独立拒绝承担" {
		t.Fatalf("rejection synchronization failed: %#v err=%v", rejected, err)
	}
	rejectedReplay, err := service.SynchronizePayableSuggestion(ctx, actor, rejectable.ID, SuggestionCommandInput{CommandID: id(67), ExpectedRevision: 2, Reason: "同步 Money 拒绝决定"})
	if err != nil || rejectedReplay.ID != rejected.ID || rejectedReplay.Revision != rejected.Revision || rejectedReplay.State != SuggestionRejected {
		t.Fatalf("rejection replay failed: %#v err=%v", rejectedReplay, err)
	}
	rejectedHistory, err := service.PayableSuggestionHistory(ctx, actor, rejectable.ID)
	if err != nil || len(rejectedHistory) != 3 || rejectedHistory[0].Snapshot.State != SuggestionTargetUnavailable || rejectedHistory[1].Snapshot.State != SuggestionProposed || rejectedHistory[2].Snapshot.State != SuggestionRejected {
		t.Fatalf("unexpected rejected suggestion history: %#v err=%v", rejectedHistory, err)
	}
}

func integrationPool(t *testing.T, ctx context.Context) (*pgxpool.Pool, func()) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	var random [8]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	schema := "cost_outsourcing_test_" + hex.EncodeToString(random[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	if err = database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
		admin.Close()
	}
	return pool, cleanup
}
