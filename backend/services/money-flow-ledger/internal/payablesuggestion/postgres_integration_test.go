//go:build integration

package payablesuggestion

import (
	"context"
	"errors"
	"testing"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/database"
	"omsbizfoundry/money-flow-ledger/internal/obligation"
	"omsbizfoundry/money-flow-ledger/internal/party"
	"omsbizfoundry/money-flow-ledger/internal/testdatabase"
	contract "omsbizfoundry/payable-suggestion-contract"
)

func TestPostgresSuggestionAcceptanceOwnsObligationAndPreservesSource(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	actor := audit.ActorRef{Kind: audit.ActorKindIAMSubject, ID: actorID, DisplayName: "资金负责人"}
	partyService := party.NewService(party.NewPostgresRepository(environment.Pool))
	owner, err := partyService.Create(ctx, party.CreateInput{CommandID: "12000000-0000-4000-8000-000000000001", Kind: "organization", LegalName: "Money 本方", Actor: actor})
	if err != nil {
		t.Fatal(err)
	}
	counterparty, err := partyService.Create(ctx, party.CreateInput{CommandID: "12000000-0000-4000-8000-000000000002", Kind: "organization", LegalName: "Money 协作方", Actor: actor})
	if err != nil {
		t.Fatal(err)
	}
	batchID := "12000000-0000-4000-8000-000000000003"
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	for index, value := range []struct {
		localID, directoryID, name string
		directoryRevision          int64
	}{{owner.Party.ID, ownerID, "设计单位", 2}, {counterparty.Party.ID, counterpartyID, "协作单位", 4}} {
		commandID := []string{"12000000-0000-4000-8000-000000000004", "12000000-0000-4000-8000-000000000005"}[index]
		if _, err = environment.Pool.Exec(ctx, `INSERT INTO party_migration_mappings(tenant_id,migration_batch_id,command_id,source_application,local_party_id,local_party_revision,local_snapshot_sha256,decision,target_party_id,target_party_revision,target_kind,target_legal_name,target_registration_identifier,reviewed_by_subject_id,reviewed_by_display_name,review_reason,target_verified_at,recorded_at,request_sha256) VALUES($1,$2,$3,'money-flow-ledger',$4,1,$5,'map_existing_directory_party',$6,$7,'organization',$8,NULL,$9,'迁移负责人','C6 付款建议联调映射',$10,$10,$5)`, tenantID, batchID, commandID, value.localID, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", value.directoryID, value.directoryRevision, value.name, actorID, now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = environment.Pool.Exec(ctx, `INSERT INTO party_runtime_transitions(tenant_id,command_id,action,migration_batch_id,prior_sequence,reviewed_by_subject_id,reviewed_by_display_name,reason,mapped_party_count,referenced_party_count,request_sha256,created_at) VALUES($1,$2,'cutover',$3,NULL,$4,'迁移负责人','C6 付款建议联调切换',2,0,$5,$6)`, tenantID, "12000000-0000-4000-8000-000000000006", batchID, actorID, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", now); err != nil {
		t.Fatal(err)
	}

	repository := NewPostgresRepository(environment.Pool)
	service, err := NewService(repository, obligation.NewService(obligation.NewPostgresRepository(environment.Pool)))
	if err != nil {
		t.Fatal(err)
	}
	submission := validSubmission(t)
	result, err := service.Submit(ctx, actor, submission)
	if err != nil || result.Intake.Status != contract.StatusPending || result.Intake.LedgerOwnerLocalPartyID != owner.Party.ID || result.Intake.CounterpartyLocalPartyID != counterparty.Party.ID {
		t.Fatalf("submit=%#v err=%v", result, err)
	}
	decision := contract.Decision{CommandID: decisionID, ExpectedRevision: 1, Decision: contract.DecisionAccept, Reason: "资金审核通过"}
	accepted, err := service.Decide(ctx, tenantID, suggestionID, actor, decision)
	if err != nil || accepted.Intake.Status != contract.StatusAccepted || accepted.Intake.Obligation == nil {
		t.Fatalf("accept=%#v err=%v", accepted, err)
	}
	var direction, status, sourceKind, sourceApplication, sourceType, sourceResourceID, sourceDigest string
	var sourceRevision, currentRevision int64
	if err = environment.Pool.QueryRow(ctx, `SELECT direction,status,source_kind,source_application,source_resource_type,source_resource_id::text,source_revision,source_digest,current_revision FROM obligations WHERE id=$1`, accepted.Intake.Obligation.ID).Scan(&direction, &status, &sourceKind, &sourceApplication, &sourceType, &sourceResourceID, &sourceRevision, &sourceDigest, &currentRevision); err != nil {
		t.Fatal(err)
	}
	if direction != "payable" || status != "open" || sourceKind != obligation.SourceKindCostOutsourcingSuggestion || sourceApplication != contract.SourceApplication || sourceType != contract.SourceEngagement || sourceResourceID != sourceID || sourceRevision != 3 || sourceDigest != submission.Source.Digest || currentRevision != 1 {
		t.Fatalf("unexpected obligation source: %s/%s/%s/%s/%s/%s/%d/%s/%d", direction, status, sourceKind, sourceApplication, sourceType, sourceResourceID, sourceRevision, sourceDigest, currentRevision)
	}
	replayed, err := service.Decide(ctx, tenantID, suggestionID, actor, decision)
	if err != nil || !replayed.Replayed || replayed.Intake.Obligation.ID != accepted.Intake.Obligation.ID {
		t.Fatalf("replay=%#v err=%v", replayed, err)
	}
	var obligationCount, intakeRevisions int
	if err = environment.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM obligations WHERE source_resource_id=$1),(SELECT count(*) FROM payable_suggestion_intake_revisions WHERE tenant_id=$2 AND suggestion_id=$3)`, sourceID, tenantID, suggestionID).Scan(&obligationCount, &intakeRevisions); err != nil {
		t.Fatal(err)
	}
	if obligationCount != 1 || intakeRevisions != 2 {
		t.Fatalf("expected one obligation and two intake revisions, got %d/%d", obligationCount, intakeRevisions)
	}
	if _, err = environment.Pool.Exec(ctx, `UPDATE payable_suggestion_intake_revisions SET event_kind='rejected' WHERE tenant_id=$1 AND suggestion_id=$2 AND revision=1`, tenantID, suggestionID); err == nil {
		t.Fatal("intake history mutation must be rejected")
	}

	rejectedSubmission := validSubmission(t)
	rejectedSubmission.SuggestionID = "11000000-0000-4000-8000-000000000010"
	rejectedSubmission.Source.ResourceType = contract.SourceCostEntry
	rejectedSubmission.Source.ResourceID = "11000000-0000-4000-8000-000000000011"
	rejectedSubmission.Source.Revision = 2
	rejectedSubmission.Source.Digest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	rejectedSubmission.AmountMinor = 330000
	rejectedSubmission.Purpose = "现场差旅成本应付建议"
	rejectedSubmission.RequestDigest = ""
	rejectedSubmission.RequestDigest, err = contract.Digest(rejectedSubmission)
	if err != nil {
		t.Fatal(err)
	}
	rejectedResult, err := service.Submit(ctx, actor, rejectedSubmission)
	if err != nil || rejectedResult.Intake.Status != contract.StatusPending || rejectedResult.Intake.Obligation != nil {
		t.Fatalf("rejected submit=%#v err=%v", rejectedResult, err)
	}
	rejectDecision := contract.Decision{CommandID: "11000000-0000-4000-8000-000000000012", ExpectedRevision: 1, Decision: contract.DecisionReject, Reason: "来源不属于本账套承担范围"}
	rejected, err := service.Decide(ctx, tenantID, rejectedSubmission.SuggestionID, actor, rejectDecision)
	if err != nil || rejected.Intake.Status != contract.StatusRejected || rejected.Intake.Revision != 2 || rejected.Intake.Obligation != nil || rejected.Intake.DecisionReason != rejectDecision.Reason {
		t.Fatalf("reject=%#v err=%v", rejected, err)
	}
	rejectedReplay, err := service.Decide(ctx, tenantID, rejectedSubmission.SuggestionID, actor, rejectDecision)
	if err != nil || !rejectedReplay.Replayed || rejectedReplay.Intake.Status != contract.StatusRejected || rejectedReplay.Intake.Obligation != nil {
		t.Fatalf("reject replay=%#v err=%v", rejectedReplay, err)
	}
	if err = environment.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM obligations WHERE source_resource_id=$1),(SELECT count(*) FROM payable_suggestion_intake_revisions WHERE tenant_id=$2 AND suggestion_id=$3)`, rejectedSubmission.Source.ResourceID, tenantID, rejectedSubmission.SuggestionID).Scan(&obligationCount, &intakeRevisions); err != nil {
		t.Fatal(err)
	}
	if obligationCount != 0 || intakeRevisions != 2 {
		t.Fatalf("rejection must create no obligation and two intake revisions, got %d/%d", obligationCount, intakeRevisions)
	}
}

func TestPostgresBonusSuggestionUsesReviewedPersonMappingAndCreatesBonusObligation(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	actor := audit.ActorRef{Kind: audit.ActorKindIAMSubject, ID: actorID, DisplayName: "资金负责人"}
	partyService := party.NewService(party.NewPostgresRepository(environment.Pool))
	owner, err := partyService.Create(ctx, party.CreateInput{CommandID: "13000000-0000-4000-8000-000000000001", Kind: "organization", LegalName: "Money 设计单位", Actor: actor})
	if err != nil {
		t.Fatal(err)
	}
	beneficiary, err := partyService.Create(ctx, party.CreateInput{CommandID: "13000000-0000-4000-8000-000000000002", Kind: "individual", LegalName: "奖金参与人", Actor: actor})
	if err != nil {
		t.Fatal(err)
	}
	personID := "13000000-0000-4000-8000-000000000003"
	personDigest := "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	if _, err = environment.Pool.Exec(ctx, `INSERT INTO payable_party_source_mappings(tenant_id,command_id,source_application,source_resource_type,source_resource_id,source_revision,source_digest,local_party_id,local_party_revision,local_party_kind,local_party_legal_name,reviewed_by_actor_kind,reviewed_by_actor_id,reviewed_by_actor_display_name,review_reason,recorded_at,request_sha256)
		VALUES($1,$2,'people-groups','person',$3,3,$4,$5,1,'individual',$6,'iam_subject',$7,'资金负责人','奖金参与人主体经人工复核', $8,$9)`, tenantID, "13000000-0000-4000-8000-000000000004", personID, personDigest, beneficiary.Party.ID, beneficiary.Party.LegalName, actorID, now, "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"); err != nil {
		t.Fatal(err)
	}

	submission := contract.Submission{SchemaVersion: contract.SchemaVersion, SuggestionID: "13000000-0000-4000-8000-000000000005", TenantID: tenantID, ProjectID: projectID,
		Source:       contract.SourceSnapshot{Application: contract.SourceApplicationBonusDistribution, ResourceType: contract.SourceBonusAllocation, ResourceID: "13000000-0000-4000-8000-000000000006", Revision: 2, Digest: "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"},
		LedgerOwner:  contract.PartySnapshot{SourceApplication: contract.PartySourceApplicationMoneyFlow, ResourceType: contract.PartyResourceParty, ID: owner.Party.ID, Revision: owner.Party.Revision, Kind: string(owner.Party.Kind), LegalName: owner.Party.LegalName},
		Counterparty: contract.PartySnapshot{SourceApplication: contract.PartySourceApplicationPeopleGroups, ResourceType: contract.PartyResourcePerson, ID: personID, Revision: 3, Kind: "individual", LegalName: "奖金参与人", Digest: personDigest},
		AmountMinor:  10650, Currency: "CNY", DueOn: "2026-11-30", Purpose: "经营奖金应付"}
	submission.RequestDigest, err = contract.Digest(submission)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(NewPostgresRepository(environment.Pool), obligation.NewService(obligation.NewPostgresRepository(environment.Pool)))
	if err != nil {
		t.Fatal(err)
	}
	submitted, err := service.Submit(ctx, actor, submission)
	if err != nil || submitted.Intake.LedgerOwnerLocalPartyID != owner.Party.ID || submitted.Intake.CounterpartyLocalPartyID != beneficiary.Party.ID {
		t.Fatalf("submit=%#v err=%v", submitted, err)
	}
	decision := contract.Decision{CommandID: "13000000-0000-4000-8000-000000000007", ExpectedRevision: 1, Decision: contract.DecisionAccept, Reason: "奖金应付审核通过"}
	accepted, err := service.Decide(ctx, tenantID, submission.SuggestionID, actor, decision)
	if err != nil || accepted.Intake.Status != contract.StatusAccepted || accepted.Intake.Obligation == nil {
		t.Fatalf("accept=%#v err=%v", accepted, err)
	}
	var sourceKind, sourceApplication, sourceType, sourceResourceID string
	var sourceRevision int64
	if err = environment.Pool.QueryRow(ctx, `SELECT source_kind,source_application,source_resource_type,source_resource_id::text,source_revision FROM obligations WHERE id=$1`, accepted.Intake.Obligation.ID).Scan(&sourceKind, &sourceApplication, &sourceType, &sourceResourceID, &sourceRevision); err != nil {
		t.Fatal(err)
	}
	if sourceKind != obligation.SourceKindBonusSuggestion || sourceApplication != contract.SourceApplicationBonusDistribution || sourceType != contract.SourceBonusAllocation || sourceResourceID != submission.Source.ResourceID || sourceRevision != submission.Source.Revision {
		t.Fatalf("unexpected Bonus obligation source %s/%s/%s/%s/%d", sourceKind, sourceApplication, sourceType, sourceResourceID, sourceRevision)
	}
	if _, err = environment.Pool.Exec(ctx, `UPDATE payable_party_source_mappings SET review_reason='tampered' WHERE tenant_id=$1 AND source_resource_id=$2`, tenantID, personID); err == nil {
		t.Fatal("reviewed Person mapping must be immutable")
	}

	unmapped := submission
	unmapped.SuggestionID = "13000000-0000-4000-8000-000000000008"
	unmapped.Counterparty.Revision = 4
	unmapped.Counterparty.Digest = "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	unmapped.RequestDigest = ""
	unmapped.RequestDigest, err = contract.Digest(unmapped)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Submit(ctx, actor, unmapped); !errors.Is(err, ErrPartyMapping) {
		t.Fatalf("unreviewed Person revision must fail closed: %v", err)
	}
}
