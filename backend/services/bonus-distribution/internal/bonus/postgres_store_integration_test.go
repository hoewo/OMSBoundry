//go:build integration

package bonus_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/bonus-distribution/internal/bonus"
	"omsbizfoundry/bonus-distribution/internal/database"
	"omsbizfoundry/bonus-distribution/internal/overlays/referenceoa"
	contract "omsbizfoundry/payable-suggestion-contract"
)

type moneyPayableFixture struct {
	fail        bool
	submissions map[string]bonus.PayableSubmission
	intakes     map[string]bonus.PayableIntake
}

func (m *moneyPayableFixture) SubmitPayableSuggestion(_ context.Context, tenantID, suggestionID string, value bonus.PayableSubmission) (bonus.PayableIntake, error) {
	if m.fail {
		return bonus.PayableIntake{}, bonus.ErrTargetUnavailable
	}
	if current, ok := m.intakes[suggestionID]; ok {
		return current, nil
	}
	intake := bonus.PayableIntake{SuggestionID: suggestionID, TenantID: tenantID, RequestDigest: value.RequestDigest, Status: contract.StatusPending, Revision: 1}
	m.submissions[suggestionID], m.intakes[suggestionID] = value, intake
	return intake, nil
}

func (m *moneyPayableFixture) GetPayableSuggestion(_ context.Context, _, suggestionID string) (bonus.PayableIntake, error) {
	if m.fail {
		return bonus.PayableIntake{}, bonus.ErrTargetUnavailable
	}
	value, ok := m.intakes[suggestionID]
	if !ok {
		return bonus.PayableIntake{}, bonus.ErrTargetUnavailable
	}
	return value, nil
}

func (m *moneyPayableFixture) WithdrawPayableSuggestion(_ context.Context, _, suggestionID, _ string, expectedRevision int64, reason string) (bonus.PayableIntake, error) {
	value, ok := m.intakes[suggestionID]
	if m.fail || !ok || value.Revision != expectedRevision {
		return bonus.PayableIntake{}, bonus.ErrTargetUnavailable
	}
	value.Status, value.Revision, value.DecisionReason = contract.StatusWithdrawn, value.Revision+1, reason
	m.intakes[suggestionID] = value
	return value, nil
}

func TestTypedSourcesExactSnapshotsAndAppendOnlyLifecycle(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := bonus.NewPostgresStore(pool)
	actor, resolver := fixture()
	service, _ := bonus.NewService(store, resolver, referenceoa.Evaluator{})

	businessInput := businessDraftInput(10, resolver)
	draft, err := service.CreateBusinessDraft(ctx, actor, businessInput)
	if err != nil {
		t.Fatal(err)
	}
	if draft.State != bonus.StateDraft || draft.Revision != 1 || draft.AmountMinor != 100_000 || draft.UnallocatedMinor != 10_000 || len(draft.Allocations) != 2 || draft.Allocations[0].AmountMinor+draft.Allocations[1].AmountMinor != 90_000 {
		t.Fatalf("unexpected exact business draft: %#v", draft)
	}
	published, err := service.Publish(ctx, actor, draft.ID, bonus.PoolCommandInput{CommandID: id(11), ExpectedRevision: 1, Reason: "确认经营奖金"})
	if err != nil || published.State != bonus.StatePublished || published.Revision != 2 {
		t.Fatalf("publish failed: %#v %v", published, err)
	}
	resolver.failProject = bonus.ErrReferenceUnavailable
	publishReplay, err := service.Publish(ctx, actor, draft.ID, bonus.PoolCommandInput{CommandID: id(11), ExpectedRevision: 1, Reason: "确认经营奖金"})
	if err != nil || publishReplay.ID != published.ID || publishReplay.Revision != published.Revision {
		t.Fatalf("successful publish replay must not re-read dependencies: %#v %v", publishReplay, err)
	}
	resolver.failProject = nil
	frozen := published
	resolver.business.TotalAmountMinor = 2_130_000
	resolver.business.Items[0].AmountMinor = 2_130_000
	resolver.business.Digest = digest("settlement-changed")
	resolver.business.Items[0].Digest = digest("settlement-item-changed")
	loaded, err := service.GetPool(ctx, actor, published.ID)
	if err != nil || loaded.AmountMinor != frozen.AmountMinor || loaded.Source.Digest != frozen.Source.Digest || loaded.Allocations[0].Person.DisplayName != frozen.Allocations[0].Person.DisplayName {
		t.Fatalf("upstream change rewrote published history: %#v %v", loaded, err)
	}

	businessOnly := actor
	businessOnly.Actions = []string{bonus.ActionBusinessCalculate, bonus.ActionAllocationManage}
	if _, err = service.CreateProductionDraft(ctx, businessOnly, productionDraftInput(12, resolver)); !errors.Is(err, bonus.ErrForbidden) {
		t.Fatalf("business capability must not create production pool: %v", err)
	}
	production, err := service.CreateProductionDraft(ctx, actor, productionDraftInput(13, resolver))
	if err != nil || production.Source.Type != bonus.SourceMoneyReceiptSet || production.BonusType != bonus.TypeProduction || resolver.businessCalls == 0 || resolver.productionCalls == 0 {
		t.Fatalf("typed source isolation failed: %#v %v calls=%d/%d", production, err, resolver.businessCalls, resolver.productionCalls)
	}

	resolver.business.ID = id(44)
	resolver.business.Revision = 10
	resolver.business.TotalAmountMinor = 2_130_000
	resolver.business.Digest = digest("settlement-10")
	resolver.business.Items = []bonus.SourceItemSnapshot{{Type: bonus.SourceContractSettlement, ID: id(44), Revision: 10, AmountMinor: 2_130_000, Currency: "CNY", State: "confirmed", Digest: digest("settlement-item-10")}}
	replacementInput := businessDraftInput(14, resolver)
	replacementInput.Source = bonus.BusinessSourceRef{SettlementID: id(44), Revision: 10}
	replacement, err := service.CreateBusinessDraft(ctx, actor, replacementInput)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Supersede(ctx, actor, bonus.SupersedeInput{CommandID: id(15), PublishedPoolID: published.ID, ExpectedPublishedRevision: 2, ReplacementPoolID: replacement.ID, ExpectedReplacementRevision: 1, Reason: "结算 revision 变化后显式取代"})
	if err != nil || result.Superseded.State != bonus.StateSuperseded || result.Superseded.Revision != 3 || result.Published.State != bonus.StatePublished || result.Published.Revision != 2 || result.Published.Supersedes == nil || result.Published.Supersedes.PoolID != published.ID {
		t.Fatalf("supersession failed: %#v %v", result, err)
	}
	resolver.failProject = bonus.ErrReferenceUnavailable
	supersedeReplay, err := service.Supersede(ctx, actor, bonus.SupersedeInput{CommandID: id(15), PublishedPoolID: published.ID, ExpectedPublishedRevision: 2, ReplacementPoolID: replacement.ID, ExpectedReplacementRevision: 1, Reason: "结算 revision 变化后显式取代"})
	if err != nil || supersedeReplay.Published.ID != result.Published.ID || supersedeReplay.Superseded.Revision != result.Superseded.Revision {
		t.Fatalf("successful supersession replay must not re-read dependencies: %#v %v", supersedeReplay, err)
	}
	resolver.failProject = nil
	withdrawn, err := service.Withdraw(ctx, actor, replacement.ID, bonus.PoolCommandInput{CommandID: id(16), ExpectedRevision: 2, Reason: "业务取消"})
	if err != nil || withdrawn.State != bonus.StateWithdrawn || withdrawn.Revision != 3 {
		t.Fatalf("withdraw failed: %#v %v", withdrawn, err)
	}
	withdrawReplay, err := service.Withdraw(ctx, actor, replacement.ID, bonus.PoolCommandInput{CommandID: id(16), ExpectedRevision: 2, Reason: "业务取消"})
	if err != nil || withdrawReplay.ID != withdrawn.ID || withdrawReplay.Revision != withdrawn.Revision {
		t.Fatalf("withdraw replay failed: %#v %v", withdrawReplay, err)
	}
	oldHistory, err := service.PoolHistory(ctx, actor, published.ID)
	if err != nil || len(oldHistory) != 3 || oldHistory[0].EventKind != "drafted" || oldHistory[1].EventKind != "published" || oldHistory[2].EventKind != "superseded" {
		t.Fatalf("old history mismatch: %#v %v", oldHistory, err)
	}
	allocationHistory, err := service.AllocationHistory(ctx, actor, published.Allocations[0].ID)
	if err != nil || len(allocationHistory) != 3 || allocationHistory[2].Snapshot.Person.Revision != published.Allocations[0].Person.Revision {
		t.Fatalf("allocation history mismatch: %#v %v", allocationHistory, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE bonus_pool_revisions SET reason='tampered' WHERE tenant_id=$1 AND pool_id=$2 AND revision=1`, actor.TenantID, published.ID); err == nil {
		t.Fatal("pool revision mutation must be rejected")
	}
	if _, err = pool.Exec(ctx, `UPDATE bonus_allocation_revisions SET reason='tampered' WHERE tenant_id=$1 AND allocation_id=$2 AND revision=1`, actor.TenantID, published.Allocations[0].ID); err == nil {
		t.Fatal("allocation revision mutation must be rejected")
	}
	if _, err = pool.Exec(ctx, `DELETE FROM bonus_pools WHERE tenant_id=$1 AND id=$2`, actor.TenantID, replacement.ID); err == nil {
		t.Fatal("pool physical deletion must be rejected")
	}
}

func TestFailuresReplayConcurrencyAndIsolation(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := bonus.NewPostgresStore(pool)
	actor, resolver := fixture()
	service, _ := bonus.NewService(store, resolver, referenceoa.Evaluator{})
	input := businessDraftInput(20, resolver)

	resolver.failProject = bonus.ErrReferenceUnavailable
	if _, err := service.CreateBusinessDraft(ctx, actor, input); !errors.Is(err, bonus.ErrReferenceUnavailable) {
		t.Fatalf("dependency failure expected: %v", err)
	}
	assertCounts(t, pool, 0, 0, 0, 0, 0, 0)
	resolver.failProject = nil
	created, err := service.CreateBusinessDraft(ctx, actor, input)
	if err != nil {
		t.Fatal(err)
	}
	resolver.failProject = bonus.ErrReferenceUnavailable
	replayed, err := service.CreateBusinessDraft(ctx, actor, input)
	if err != nil || replayed.ID != created.ID {
		t.Fatalf("successful replay must not call failed dependencies: %#v %v", replayed, err)
	}
	changed := input
	changed.Allocations = append([]bonus.AllocationRequest(nil), input.Allocations...)
	changed.Allocations[0].RatioBasisPoints = 5000
	if _, err = service.CreateBusinessDraft(ctx, actor, changed); !errors.Is(err, bonus.ErrCommandConflict) {
		t.Fatalf("same command with a changed payload must conflict: %v", err)
	}
	resolver.failProject = nil

	concurrentInput := businessDraftInput(21, resolver)
	const workers = 8
	ids, errs := make(chan string, workers), make(chan error, workers)
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			value, createErr := service.CreateBusinessDraft(ctx, actor, concurrentInput)
			if createErr != nil {
				errs <- createErr
				return
			}
			ids <- value.ID
		}()
	}
	group.Wait()
	close(ids)
	close(errs)
	for createErr := range errs {
		t.Fatal(createErr)
	}
	first, count := "", 0
	for value := range ids {
		count++
		if first == "" {
			first = value
		} else if value != first {
			t.Fatalf("concurrent replay returned different pools: %s / %s", first, value)
		}
	}
	if count != workers {
		t.Fatalf("got %d successful workers", count)
	}

	resolver.business.Digest = digest("drifted-before-publish")
	if _, err = service.Publish(ctx, actor, first, bonus.PoolCommandInput{CommandID: id(22), ExpectedRevision: 1, Reason: "漂移时不得发布"}); !errors.Is(err, bonus.ErrReferenceInvalid) {
		t.Fatalf("source drift must fail closed: %v", err)
	}
	current, err := service.GetPool(ctx, actor, first)
	if err != nil || current.State != bonus.StateDraft || current.Revision != 1 {
		t.Fatalf("drift failure changed current: %#v %v", current, err)
	}
	resolver.business.Digest = digest("settlement-9")

	results := make(chan error, 2)
	for _, command := range []int{23, 24} {
		group.Add(1)
		go func(command int) {
			defer group.Done()
			_, publishErr := service.Publish(ctx, actor, first, bonus.PoolCommandInput{CommandID: id(command), ExpectedRevision: 1, Reason: "并发发布"})
			results <- publishErr
		}(command)
	}
	group.Wait()
	close(results)
	successes, conflicts := 0, 0
	for publishErr := range results {
		if publishErr == nil {
			successes++
		} else if errors.Is(publishErr, bonus.ErrRevisionConflict) {
			conflicts++
		} else {
			t.Fatalf("unexpected concurrent publish error: %v", publishErr)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("expected one publish and one revision conflict, got %d/%d", successes, conflicts)
	}

	wrongTenant := actor
	wrongTenant.TenantID = id(98)
	if _, err = service.GetPool(ctx, wrongTenant, created.ID); !errors.Is(err, bonus.ErrNotFound) {
		t.Fatalf("cross-tenant read must look not found: %v", err)
	}
	wrongProject := actor
	wrongProject.ProjectIDs = []string{id(99)}
	if _, err = service.GetPool(ctx, wrongProject, created.ID); !errors.Is(err, bonus.ErrForbidden) {
		t.Fatalf("cross-project read must be forbidden: %v", err)
	}

	invalid := businessDraftInput(25, resolver)
	invalid.Allocations[0].RatioBasisPoints = 9000
	invalid.Allocations[1].RatioBasisPoints = 2000
	if _, err = service.CreateBusinessDraft(ctx, actor, invalid); !errors.Is(err, bonus.ErrAllocationInvalid) {
		t.Fatalf("over-allocation must fail before persistence: %v", err)
	}
	assertCounts(t, pool, 2, 3, 4, 6, 3, 3)
}

func TestPublishedAllocationProposesPayableWithOutageRecoveryAndIndependentMoneyDecision(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := bonus.NewPostgresStore(pool)
	actor, resolver := fixture()
	actor.Actions = append(actor.Actions, bonus.ActionSuggestionCreate, bonus.ActionSuggestionRead, bonus.ActionSuggestionWithdraw)

	service, _ := bonus.NewService(store, resolver, referenceoa.Evaluator{})
	draft, err := service.CreateBusinessDraft(ctx, actor, businessDraftInput(70, resolver))
	if err != nil {
		t.Fatal(err)
	}
	published, err := service.Publish(ctx, actor, draft.ID, bonus.PoolCommandInput{CommandID: id(71), ExpectedRevision: 1, Reason: "确认奖金分配"})
	if err != nil || len(published.Allocations) != 2 {
		t.Fatalf("publish=%#v err=%v", published, err)
	}
	firstInput := bonus.ProposePayableSuggestionInput{CommandID: id(72), AllocationID: published.Allocations[0].ID, AllocationRevision: published.Allocations[0].Revision, LedgerOwner: bonus.MoneyPartyRef{ID: id(80), Revision: 1, Kind: "organization", LegalName: "设计单位"}, DueOn: "2026-10-31", Purpose: "经营奖金应付"}
	failed, err := service.ProposePayableSuggestion(ctx, actor, firstInput)
	if err != nil || failed.State != bonus.SuggestionTargetUnavailable || failed.Revision != 1 || failed.TargetRevision != 0 {
		t.Fatalf("target outage proposal=%#v err=%v", failed, err)
	}

	money := &moneyPayableFixture{submissions: map[string]bonus.PayableSubmission{}, intakes: map[string]bonus.PayableIntake{}}
	service, _ = bonus.NewService(store, resolver, referenceoa.Evaluator{}, money)
	proposed, err := service.ProposePayableSuggestion(ctx, actor, firstInput)
	if err != nil || proposed.ID != failed.ID || proposed.State != bonus.SuggestionProposed || proposed.Revision != 2 || proposed.TargetRevision != 1 {
		t.Fatalf("recovered proposal=%#v err=%v", proposed, err)
	}
	submitted := money.submissions[proposed.ID]
	if submitted.Source.AllocationID != published.Allocations[0].ID || submitted.Source.Person.Digest != published.Allocations[0].Person.Digest || submitted.AmountMinor != published.Allocations[0].AmountMinor {
		t.Fatalf("Money submission did not freeze the allocation and Person snapshots: %#v", submitted)
	}
	if replayed, replayErr := service.ProposePayableSuggestion(ctx, actor, firstInput); replayErr != nil || replayed.Revision != proposed.Revision || len(money.submissions) != 1 {
		t.Fatalf("proposal replay=%#v err=%v submissions=%d", replayed, replayErr, len(money.submissions))
	}
	accepted := money.intakes[proposed.ID]
	accepted.Status, accepted.Revision = contract.StatusAccepted, 2
	accepted.DecisionReason = "资金负责人确认应付"
	accepted.Obligation = &bonus.ObligationReference{ID: id(81), Revision: 1}
	money.intakes[proposed.ID] = accepted
	syncInput := bonus.SuggestionCommandInput{CommandID: id(73), ExpectedRevision: proposed.Revision, Reason: "同步资金决定"}
	synchronized, err := service.SynchronizePayableSuggestion(ctx, actor, proposed.ID, syncInput)
	if err != nil || synchronized.State != bonus.SuggestionAccepted || synchronized.Revision != 3 || synchronized.Obligation == nil || synchronized.Obligation.ID != id(81) {
		t.Fatalf("accepted synchronization=%#v err=%v", synchronized, err)
	}
	if replayed, replayErr := service.SynchronizePayableSuggestion(ctx, actor, proposed.ID, syncInput); replayErr != nil || replayed.Revision != synchronized.Revision || replayed.Obligation.ID != synchronized.Obligation.ID {
		t.Fatalf("synchronization replay=%#v err=%v", replayed, replayErr)
	}

	secondInput := bonus.ProposePayableSuggestionInput{CommandID: id(74), AllocationID: published.Allocations[1].ID, AllocationRevision: published.Allocations[1].Revision, LedgerOwner: firstInput.LedgerOwner, DueOn: "2026-11-15", Purpose: "另一参与人奖金应付"}
	second, err := service.ProposePayableSuggestion(ctx, actor, secondInput)
	if err != nil || second.State != bonus.SuggestionProposed {
		t.Fatalf("second proposal=%#v err=%v", second, err)
	}
	rejected := money.intakes[second.ID]
	rejected.Status, rejected.Revision, rejected.DecisionReason = contract.StatusRejected, 2, "不属于本账套"
	money.intakes[second.ID] = rejected
	rejectedResult, err := service.SynchronizePayableSuggestion(ctx, actor, second.ID, bonus.SuggestionCommandInput{CommandID: id(75), ExpectedRevision: second.Revision, Reason: "同步拒绝决定"})
	if err != nil || rejectedResult.State != bonus.SuggestionRejected || rejectedResult.Obligation != nil || rejectedResult.DecisionReason != "不属于本账套" {
		t.Fatalf("rejected synchronization=%#v err=%v", rejectedResult, err)
	}
	history, err := service.PayableSuggestionHistory(ctx, actor, proposed.ID)
	if err != nil || len(history) != 3 || history[0].EventKind != "proposed" || history[1].EventKind != "submitted" || history[2].EventKind != "accepted" {
		t.Fatalf("suggestion history=%#v err=%v", history, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE bonus_payable_suggestion_revisions SET reason='tampered' WHERE tenant_id=$1 AND suggestion_id=$2 AND revision=1`, actor.TenantID, proposed.ID); err == nil {
		t.Fatal("suggestion history mutation must be rejected")
	}
}

type resolverFixture struct {
	project         bonus.ProjectReference
	business        bonus.SourceSnapshot
	production      bonus.SourceSnapshot
	participants    map[string]bonus.ParticipantReference
	failProject     error
	failBusiness    error
	failProduction  error
	failParticipant error
	businessCalls   int
	productionCalls int
	mu              sync.Mutex
}

func (r *resolverFixture) ResolveProject(_ context.Context, _ string, _ bonus.ProjectRef) (bonus.ProjectReference, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failProject != nil {
		return bonus.ProjectReference{}, r.failProject
	}
	return r.project, nil
}

func (r *resolverFixture) ResolveBusinessSource(_ context.Context, _ string, _ bonus.ProjectSnapshot, _ bonus.BusinessSourceRef) (bonus.SourceSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.businessCalls++
	if r.failBusiness != nil {
		return bonus.SourceSnapshot{}, r.failBusiness
	}
	return cloneSource(r.business), nil
}

func (r *resolverFixture) ResolveProductionSource(_ context.Context, _ string, _ bonus.ProjectSnapshot, _ bonus.ProductionSourceRef) (bonus.SourceSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.productionCalls++
	if r.failProduction != nil {
		return bonus.SourceSnapshot{}, r.failProduction
	}
	return cloneSource(r.production), nil
}

func (r *resolverFixture) ResolveParticipant(_ context.Context, _ string, _ bonus.ProjectSnapshot, bonusType string, _ bonus.PersonRef, assignment bonus.AssignmentRef) (bonus.ParticipantReference, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failParticipant != nil {
		return bonus.ParticipantReference{}, r.failParticipant
	}
	value, ok := r.participants[bonusType+":"+assignment.ID]
	if !ok {
		return bonus.ParticipantReference{}, bonus.ErrReferenceInvalid
	}
	return value, nil
}

func fixture() (bonus.Actor, *resolverFixture) {
	tenantID, projectID := id(1), id(2)
	project := bonus.ProjectReference{ProjectSnapshot: bonus.ProjectSnapshot{ID: projectID, Revision: 7, Code: "P-001", DisplayName: "道路设计项目", Digest: digest("project-7")}, TenantID: tenantID, State: "active"}
	business := bonus.SourceSnapshot{Type: bonus.SourceContractSettlement, ID: id(4), Revision: 9, ProjectID: projectID, Currency: "CNY", TotalAmountMinor: 1_065_000, State: "confirmed", Digest: digest("settlement-9"), Items: []bonus.SourceItemSnapshot{{Type: bonus.SourceContractSettlement, ID: id(4), Revision: 9, AmountMinor: 1_065_000, Currency: "CNY", State: "confirmed", Digest: digest("settlement-item-9")}}}
	production := bonus.SourceSnapshot{Type: bonus.SourceMoneyReceiptSet, ID: id(5), Revision: 4, ProjectID: projectID, Currency: "CNY", TotalAmountMinor: 600_000, State: "confirmed", Digest: digest("receipt-set-4"), Items: []bonus.SourceItemSnapshot{{Type: bonus.SourceMoneyReceiptItem, ID: id(6), Revision: 2, TransactionID: id(60), TransactionRevision: 5, AmountMinor: 250_000, Currency: "CNY", State: "confirmed", Digest: digest("receipt-6")}, {Type: bonus.SourceMoneyReceiptItem, ID: id(7), Revision: 3, TransactionID: id(61), TransactionRevision: 6, AmountMinor: 350_000, Currency: "CNY", State: "confirmed", Digest: digest("receipt-7")}}}
	participants := map[string]bonus.ParticipantReference{}
	for _, item := range []struct {
		bonusType  string
		person     int
		assignment int
		role       string
		name       string
	}{
		{bonus.TypeBusiness, 31, 41, "business_manager", "经营负责人"},
		{bonus.TypeBusiness, 32, 42, "business_participant", "经营参与人"},
		{bonus.TypeProduction, 33, 43, "production_manager", "生产负责人"},
		{bonus.TypeProduction, 34, 44, "production_participant", "生产参与人"},
	} {
		personID, assignmentID := id(item.person), id(item.assignment)
		participants[item.bonusType+":"+assignmentID] = bonus.ParticipantReference{TenantID: tenantID, BonusType: item.bonusType, Active: true, Eligible: true, Person: bonus.PersonSnapshot{ID: personID, Revision: 3, DisplayName: item.name, Digest: digest("person-" + personID)}, Assignment: bonus.AssignmentSnapshot{ID: assignmentID, Revision: 5, ProjectID: projectID, PersonID: personID, RoleKey: item.role, Digest: digest("assignment-" + assignmentID)}}
	}
	actions := []string{bonus.ActionBusinessCalculate, bonus.ActionBusinessPublish, bonus.ActionBusinessSupersede, bonus.ActionBusinessWithdraw, bonus.ActionProductionCalculate, bonus.ActionProductionPublish, bonus.ActionProductionSupersede, bonus.ActionProductionWithdraw, bonus.ActionPoolRead, bonus.ActionPoolHistory, bonus.ActionAllocationManage, bonus.ActionAllocationRead, bonus.ActionAllocationAmountRead}
	actor := bonus.Actor{ID: id(3), TenantID: tenantID, ProjectIDs: []string{projectID}, Actions: actions, Credential: "credential-1", SubjectKind: "user"}
	return actor, &resolverFixture{project: project, business: business, production: production, participants: participants}
}

func businessDraftInput(command int, resolver *resolverFixture) bonus.CreateBusinessDraftInput {
	return bonus.CreateBusinessDraftInput{CommandID: id(command), Project: bonus.ProjectRef{ID: resolver.project.ID, Revision: resolver.project.Revision}, Source: bonus.BusinessSourceRef{SettlementID: resolver.business.ID, Revision: resolver.business.Revision}, Policy: bonus.PolicyRequest{Key: referenceoa.PolicyKey, Version: referenceoa.PolicyVersion, RateBasisPoints: 1000, Rounding: bonus.RoundingHalfUp}, Allocations: []bonus.AllocationRequest{participantRequest(resolver, bonus.TypeBusiness, 41, 6000, "经营负责人分配"), participantRequest(resolver, bonus.TypeBusiness, 42, 3000, "经营参与人分配")}}
}

func productionDraftInput(command int, resolver *resolverFixture) bonus.CreateProductionDraftInput {
	return bonus.CreateProductionDraftInput{CommandID: id(command), Project: bonus.ProjectRef{ID: resolver.project.ID, Revision: resolver.project.Revision}, Source: bonus.ProductionSourceRef{ReceiptSetID: resolver.production.ID, Revision: resolver.production.Revision}, Policy: bonus.PolicyRequest{Key: referenceoa.PolicyKey, Version: referenceoa.PolicyVersion, RateBasisPoints: 1000, Rounding: bonus.RoundingHalfUp}, Allocations: []bonus.AllocationRequest{participantRequest(resolver, bonus.TypeProduction, 43, 5000, "生产负责人分配"), participantRequest(resolver, bonus.TypeProduction, 44, 5000, "生产参与人分配")}}
}

func participantRequest(resolver *resolverFixture, bonusType string, assignment int, ratio int64, reason string) bonus.AllocationRequest {
	value := resolver.participants[bonusType+":"+id(assignment)]
	return bonus.AllocationRequest{Person: bonus.PersonRef{ID: value.Person.ID, Revision: value.Person.Revision}, Assignment: bonus.AssignmentRef{ID: value.Assignment.ID, Revision: value.Assignment.Revision}, RatioBasisPoints: ratio, Reason: reason}
}

func cloneSource(value bonus.SourceSnapshot) bonus.SourceSnapshot {
	value.Items = append([]bonus.SourceItemSnapshot(nil), value.Items...)
	return value
}

func assertCounts(t *testing.T, pool *pgxpool.Pool, pools, poolRevisions, allocations, allocationRevisions, commands, audits int) {
	t.Helper()
	var got [6]int
	err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM bonus_pools),(SELECT count(*) FROM bonus_pool_revisions),(SELECT count(*) FROM bonus_allocations),(SELECT count(*) FROM bonus_allocation_revisions),(SELECT count(*) FROM bonus_distribution_commands),(SELECT count(*) FROM bonus_distribution_audit_events)`).Scan(&got[0], &got[1], &got[2], &got[3], &got[4], &got[5])
	if err != nil {
		t.Fatal(err)
	}
	want := [6]int{pools, poolRevisions, allocations, allocationRevisions, commands, audits}
	if got != want {
		t.Fatalf("unexpected database counts: got %v want %v", got, want)
	}
}

func integrationPool(t *testing.T, ctx context.Context, legacySchema2 ...bool) (*pgxpool.Pool, func()) {
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
	schema := "bonus_distribution_test_" + hex.EncodeToString(random[:])
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
	if len(legacySchema2) > 0 && legacySchema2[0] {
		if _, err = pool.Exec(ctx, `CREATE TABLE schema_migrations(version text PRIMARY KEY,applied_at timestamptz NOT NULL DEFAULT clock_timestamp())`); err != nil {
			t.Fatal(err)
		}
		for _, version := range []string{"0001_bonus_distribution.sql", "0002_bonus_payable_suggestions.sql"} {
			data, e := os.ReadFile("../database/migrations/" + version)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = pool.Exec(ctx, string(data)); e != nil {
				t.Fatal(e)
			}
			if _, e = pool.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, version); e != nil {
				t.Fatal(e)
			}
		}
	} else if err = database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
		admin.Close()
	}
	return pool, cleanup
}

func id(value int) string { return fmt.Sprintf("00000000-0000-4000-8000-%012d", value) }

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}
