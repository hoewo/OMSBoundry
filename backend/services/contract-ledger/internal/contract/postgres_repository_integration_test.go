//go:build integration

package contract_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/contract-ledger/internal/audit"
	"omsbizfoundry/contract-ledger/internal/contract"
	"omsbizfoundry/contract-ledger/internal/database"
	"omsbizfoundry/contract-ledger/internal/party"
)

var integrationActor = contract.ActorRef{
	Kind:        "local_user",
	ID:          "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
	DisplayName: "集成测试操作者",
}

func TestPostgresRepositoryCreateListAndIdempotency(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("second database.Migrate() error = %v", err)
	}

	commandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	contractNumber := "INTEGRATION-" + strings.ToUpper(commandID[len(commandID)-8:])
	input := contract.CreateDraftInput{
		Actor:           integrationActor,
		CommandID:       commandID,
		NumberNamespace: "integration",
		ContractNumber:  contractNumber,
		Title:           "合同草稿集成测试",
		Category:        "test",
		Currency:        "cny",
		TotalAmount:     "0001000.500000",
		SignedOn:        "2026-09-19",
		EffectiveOn:     "2026-10-01",
	}

	service := contract.NewService(contract.NewPostgresRepository(pool))
	created, err := service.CreateDraft(ctx, input)
	if err != nil {
		t.Fatalf("CreateDraft() error = %v", err)
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		cleanupAuditEntries(t, pool, []string{created.ID})
		if _, err := pool.Exec(cleanupCtx, "DELETE FROM contracts WHERE id = $1", created.ID); err != nil {
			t.Errorf("clean up contract %q: %v", created.ID, err)
		}
	}()
	if created.TotalAmount != "1000.5" || created.Currency != "CNY" || created.CurrentRevision != 0 {
		t.Fatalf("created contract = %#v", created)
	}

	replayed, err := service.CreateDraft(ctx, input)
	if err != nil {
		t.Fatalf("idempotent CreateDraft() error = %v", err)
	}
	if replayed.ID != created.ID {
		t.Fatalf("idempotent ID = %q, want %q", replayed.ID, created.ID)
	}
	var createAuditCount int
	var createActorID string
	if err := pool.QueryRow(ctx, `
		SELECT count(*), min(actor_id)
		FROM audit_entries
		WHERE object_type = 'contract' AND object_id = $1 AND action = 'contract_draft_created'
	`, created.ID).Scan(&createAuditCount, &createActorID); err != nil {
		t.Fatalf("read create audit: %v", err)
	}
	if createAuditCount != 1 || createActorID != integrationActor.ID {
		t.Fatalf("create audit count=%d actor=%q", createAuditCount, createActorID)
	}

	changed := input
	changed.Title = "使用同一命令修改负载"
	_, err = service.CreateDraft(ctx, changed)
	var idempotencyConflict *contract.IdempotencyConflictError
	if !errors.As(err, &idempotencyConflict) {
		t.Fatalf("changed replay error = %v, want IdempotencyConflictError", err)
	}

	duplicate := input
	duplicate.CommandID, err = contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	duplicate.NumberNamespace = strings.ToUpper(input.NumberNamespace)
	duplicate.ContractNumber = strings.ToLower(input.ContractNumber)
	_, err = service.CreateDraft(ctx, duplicate)
	var duplicateNumber *contract.DuplicateNumberError
	if !errors.As(err, &duplicateNumber) {
		t.Fatalf("duplicate number error = %v, want DuplicateNumberError", err)
	}

	contracts, err := service.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	found := false
	for _, value := range contracts {
		if value.ID == created.ID {
			found = value.Title == input.Title && value.TotalAmount == "1000.5"
			break
		}
	}
	if !found {
		t.Fatalf("created contract not found in %#v", contracts)
	}

	existingAuditID := ""
	if err := pool.QueryRow(ctx, `SELECT id::text FROM audit_entries WHERE object_id = $1 LIMIT 1`, created.ID).Scan(&existingAuditID); err != nil {
		t.Fatalf("read existing create audit ID: %v", err)
	}
	rollbackContractID, err := contract.NewCommandID()
	if err != nil {
		t.Fatal(err)
	}
	rollbackCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = contract.NewPostgresRepository(pool).CreateDraft(ctx, contract.Contract{
		ID: rollbackContractID, CommandID: rollbackCommandID,
		NumberNamespace: "integration", ContractNumber: "ROLLBACK-" + rollbackContractID[len(rollbackContractID)-8:],
		Title: "审计失败回滚合同", Currency: "CNY", Status: contract.StatusDraft, DraftVersion: 1,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}, audit.Entry{
		ID: existingAuditID, ObjectType: audit.ObjectContract, ObjectID: rollbackContractID,
		Action: contract.AuditActionDraftCreated, CommandID: rollbackCommandID, Actor: integrationActor,
		OccurredAt: time.Now().UTC(), Summary: "应当回滚", Details: map[string]any{"resulting_draft_version": 1},
	})
	if err == nil {
		t.Fatal("CreateDraft() with duplicate audit ID error = nil")
	}
	var rollbackContractCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM contracts WHERE id = $1`, rollbackContractID).Scan(&rollbackContractCount); err != nil {
		t.Fatalf("count rollback contract: %v", err)
	}
	if rollbackContractCount != 0 {
		t.Fatalf("audit failure left contract")
	}
}

func TestPostgresRepositorySearchUsesLiteralFiltersAndStableKeysetPagination(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("second database.Migrate() error = %v", err)
	}

	seedID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	suffix := strings.ToLower(seedID[len(seedID)-8:])
	pageNamespace := "search-page-" + suffix
	literalNamespace := "search-literal-" + suffix
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, cleanupErr := pool.Exec(cleanupCtx, `DELETE FROM contracts WHERE number_namespace IN ($1, $2)`, pageNamespace, literalNamespace); cleanupErr != nil {
			t.Errorf("clean up search contracts: %v", cleanupErr)
		}
	}()

	insertContract := func(namespace, number, title, currency string, createdAt time.Time) string {
		t.Helper()
		id, idErr := contract.NewCommandID()
		if idErr != nil {
			t.Fatalf("create contract ID: %v", idErr)
		}
		commandID, commandErr := contract.NewCommandID()
		if commandErr != nil {
			t.Fatalf("create command ID: %v", commandErr)
		}
		if _, insertErr := pool.Exec(ctx, `
			INSERT INTO contracts (
				id, command_id, number_namespace, contract_number, title, currency,
				status, draft_version, current_revision, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, 'draft', 1, NULL, $7, $7)
		`, id, commandID, namespace, number, title, currency, createdAt); insertErr != nil {
			t.Fatalf("insert search contract %q: %v", number, insertErr)
		}
		return id
	}

	baseTime := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	wantIDs := make([]string, 53)
	for index := range wantIDs {
		wantIDs[index] = insertContract(
			pageNamespace,
			fmt.Sprintf("PAGE-%03d", index+1),
			fmt.Sprintf("分页稳定性合同 %03d", index+1),
			"CNY",
			baseTime.Add(-time.Duration(index)*time.Second),
		)
	}
	literalIDs := map[string]string{
		"%wild-" + suffix:       insertContract(literalNamespace, "LITERAL-PERCENT", "字面 %wild-"+suffix, "USD", baseTime),
		"_under-" + suffix:      insertContract(literalNamespace, "LITERAL-UNDERSCORE", "字面 _under-"+suffix, "USD", baseTime.Add(-time.Second)),
		`\slash-` + suffix:      insertContract(literalNamespace, "LITERAL-BACKSLASH", `字面 \slash-`+suffix, "USD", baseTime.Add(-2*time.Second)),
		"'quote-" + suffix:      insertContract(literalNamespace, "LITERAL-QUOTE", "字面 'quote-"+suffix, "USD", baseTime.Add(-3*time.Second)),
		"mixed-title-" + suffix: insertContract(literalNamespace, "LITERAL-MIXED-CASE", "字面 MiXeD-TiTle-"+suffix, "USD", baseTime.Add(-4*time.Second)),
	}

	service := contract.NewService(contract.NewPostgresRepository(pool))
	first, err := service.Search(ctx, contract.SearchContractsInput{
		Keyword: strings.ToUpper(pageNamespace), Status: " DRAFT ", Currency: "cny",
	})
	if err != nil {
		t.Fatalf("Search(first page) error = %v", err)
	}
	if len(first.Contracts) != 50 || first.NextCursor == "" {
		t.Fatalf("first page count=%d cursor=%q", len(first.Contracts), first.NextCursor)
	}
	for index, value := range first.Contracts {
		if value.ID != wantIDs[index] {
			t.Fatalf("first page[%d] ID = %q, want %q", index, value.ID, wantIDs[index])
		}
	}

	newerID := insertContract(pageNamespace, "PAGE-NEWER", "第一页后新增合同", "CNY", baseTime.Add(time.Minute))
	second, err := service.Search(ctx, contract.SearchContractsInput{
		Keyword: strings.ToUpper(pageNamespace), Status: "draft", Currency: "CNY", Cursor: first.NextCursor,
	})
	if err != nil {
		t.Fatalf("Search(second page) error = %v", err)
	}
	if len(second.Contracts) != 3 || second.NextCursor != "" {
		t.Fatalf("second page count=%d cursor=%q", len(second.Contracts), second.NextCursor)
	}
	for index, value := range second.Contracts {
		if value.ID != wantIDs[index+50] || value.ID == newerID {
			t.Fatalf("second page[%d] ID = %q, want %q", index, value.ID, wantIDs[index+50])
		}
	}
	fieldSearches := map[string]string{
		"page-053":    wantIDs[52],
		"分页稳定性合同 010": wantIDs[9],
	}
	for keyword, wantID := range fieldSearches {
		page, searchErr := service.Search(ctx, contract.SearchContractsInput{
			Keyword: keyword, Status: "draft", Currency: "cny",
		})
		if searchErr != nil {
			t.Fatalf("Search field %q error = %v", keyword, searchErr)
		}
		if len(page.Contracts) != 1 || page.Contracts[0].ID != wantID {
			t.Fatalf("Search field %q contracts = %#v, want ID %q", keyword, page.Contracts, wantID)
		}
	}

	for keyword, wantID := range literalIDs {
		page, searchErr := service.Search(ctx, contract.SearchContractsInput{
			Keyword: keyword, Status: "draft", Currency: "usd",
		})
		if searchErr != nil {
			t.Fatalf("Search(%q) error = %v", keyword, searchErr)
		}
		if len(page.Contracts) != 1 || page.Contracts[0].ID != wantID {
			t.Fatalf("Search(%q) contracts = %#v, want ID %q", keyword, page.Contracts, wantID)
		}
	}
}

func TestPostgresRepositoryContractRelationGraph(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("second database.Migrate() error = %v", err)
	}

	contractIDs := make([]string, 0, 5)
	partyIDs := make([]string, 0, 2)
	defer func() { cleanupActivationFixtures(t, pool, contractIDs, partyIDs) }()
	seedID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	suffix := strings.ToUpper(seedID[len(seedID)-8:])
	service := contract.NewService(contract.NewPostgresRepository(pool))
	partyService := party.NewService(party.NewPostgresRepository(pool))
	firstParty, err := partyService.Create(ctx, party.CreateInput{
		Actor: integrationActor, Kind: "organization", LegalName: "关系图主体甲",
		RegistrationIdentifier: "REL-A-" + suffix,
	})
	if err != nil {
		t.Fatalf("create first relation party: %v", err)
	}
	partyIDs = append(partyIDs, firstParty.ID)
	secondParty, err := partyService.Create(ctx, party.CreateInput{
		Actor: integrationActor, Kind: "organization", LegalName: "关系图主体乙",
		RegistrationIdentifier: "REL-B-" + suffix,
	})
	if err != nil {
		t.Fatalf("create second relation party: %v", err)
	}
	partyIDs = append(partyIDs, secondParty.ID)

	createDraft := func(label, title string) contract.Contract {
		t.Helper()
		commandID, commandErr := contract.NewCommandID()
		if commandErr != nil {
			t.Fatalf("create draft command ID: %v", commandErr)
		}
		created, createErr := service.CreateDraft(ctx, contract.CreateDraftInput{
			Actor: integrationActor, CommandID: commandID, NumberNamespace: "relation-integration",
			ContractNumber: label + "-" + suffix, Title: title, Currency: "CNY", TotalAmount: "1000",
			SignedOn: "2026-09-19", EffectiveOn: "2026-10-01",
		})
		if createErr != nil {
			t.Fatalf("CreateDraft(%q) error = %v", title, createErr)
		}
		contractIDs = append(contractIDs, created.ID)
		return created
	}
	activate := func(created contract.Contract) contract.Contract {
		t.Helper()
		for index, relationParty := range []party.Party{firstParty, secondParty} {
			commandID, commandErr := contract.NewCommandID()
			if commandErr != nil {
				t.Fatalf("create party command ID: %v", commandErr)
			}
			if _, addErr := service.AddDraftParty(ctx, contract.AddDraftPartyInput{
				Actor: integrationActor, CommandID: commandID, ContractID: created.ID,
				PartyID: relationParty.ID, Role: []string{"委托方", "受托方"}[index],
				ExpectedDraftVersion: int64(index + 1),
			}); addErr != nil {
				t.Fatalf("AddDraftParty(%q) error = %v", created.Title, addErr)
			}
		}
		commandID, commandErr := contract.NewCommandID()
		if commandErr != nil {
			t.Fatalf("create activation command ID: %v", commandErr)
		}
		if _, activateErr := service.Activate(ctx, contract.ActivateContractInput{
			Actor: integrationActor, CommandID: commandID, ContractID: created.ID, ExpectedDraftVersion: 3,
		}); activateErr != nil {
			t.Fatalf("Activate(%q) error = %v", created.Title, activateErr)
		}
		created.Status = contract.StatusActive
		created.CurrentRevision = 1
		return created
	}
	a := activate(createDraft("A", "关系图合同 A"))
	b := activate(createDraft("B", "关系图合同 B"))
	c := activate(createDraft("C", "关系图合同 C"))
	d := activate(createDraft("D", "关系图合同 D"))
	draft := createDraft("DRAFT", "尚未激活的关系图合同")

	newLinkInput := func(sourceID, targetID string, relationType contract.RelationType) contract.LinkContractRelationInput {
		t.Helper()
		commandID, commandErr := contract.NewCommandID()
		if commandErr != nil {
			t.Fatalf("create relation command ID: %v", commandErr)
		}
		return contract.LinkContractRelationInput{
			Actor: integrationActor, CommandID: commandID, SourceContractID: sourceID, TargetContractID: targetID,
			RelationType: string(relationType), ExpectedSourceRevision: 1, ExpectedTargetRevision: 1,
			EffectiveOn: "2026-10-02", Note: "关系图集成验证",
		}
	}

	relationTypes := []contract.RelationType{
		contract.RelationTypeFundingSourceFor,
		contract.RelationTypeEntrustmentBasisFor,
		contract.RelationTypeSubcontractOf,
		contract.RelationTypeSpecificUnderFramework,
		contract.RelationTypeSupersedes,
		contract.RelationTypeReferences,
	}
	var firstInput contract.LinkContractRelationInput
	var firstResult contract.ContractRelationResult
	for index, relationType := range relationTypes {
		input := newLinkInput(a.ID, b.ID, relationType)
		result, linkErr := service.LinkRelation(ctx, input)
		if linkErr != nil {
			t.Fatalf("LinkRelation(%s) error = %v", relationType, linkErr)
		}
		if result.Relation.Source.ContractID != a.ID || result.Relation.Source.Revision != 1 ||
			result.Relation.Source.Title != a.Title || result.Relation.Target.ContractID != b.ID ||
			result.Relation.Target.Revision != 1 || result.Relation.Target.Title != b.Title || result.Replayed {
			t.Fatalf("linked relation %s = %#v", relationType, result)
		}
		if index == 0 {
			firstInput, firstResult = input, result
		}
	}
	replayed, err := service.LinkRelation(ctx, firstInput)
	if err != nil || !replayed.Replayed || replayed.Relation.ID != firstResult.Relation.ID {
		t.Fatalf("replayed LinkRelation() = %#v, error=%v", replayed, err)
	}
	changedReplay := firstInput
	changedReplay.Note = "同命令改变说明"
	_, err = service.LinkRelation(ctx, changedReplay)
	var idempotencyConflict *contract.IdempotencyConflictError
	if !errors.As(err, &idempotencyConflict) {
		t.Fatalf("changed relation replay error = %v", err)
	}

	duplicateInput := newLinkInput(a.ID, b.ID, contract.RelationTypeFundingSourceFor)
	_, err = service.LinkRelation(ctx, duplicateInput)
	var duplicateRelation *contract.DuplicateContractRelationError
	if !errors.As(err, &duplicateRelation) {
		t.Fatalf("duplicate relation error = %v", err)
	}
	stateInput := newLinkInput(draft.ID, a.ID, contract.RelationTypeReferences)
	_, err = service.LinkRelation(ctx, stateInput)
	var stateConflict *contract.ContractRelationStateConflictError
	if !errors.As(err, &stateConflict) || stateConflict.Side != "source" || stateConflict.Status != contract.StatusDraft {
		t.Fatalf("draft relation error = %v", err)
	}
	cancelCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CancelDraft(ctx, contract.CancelDraftInput{
		Actor: integrationActor, CommandID: cancelCommandID, ContractID: draft.ID,
		ExpectedDraftVersion: 1, Reason: "验证已取消合同不能建立关系",
	}); err != nil {
		t.Fatalf("CancelDraft(relation fixture) error = %v", err)
	}
	cancelledTargetInput := newLinkInput(a.ID, draft.ID, contract.RelationTypeReferences)
	_, err = service.LinkRelation(ctx, cancelledTargetInput)
	if !errors.As(err, &stateConflict) || stateConflict.Side != "target" || stateConflict.Status != contract.StatusCancelled {
		t.Fatalf("cancelled target relation error = %v", err)
	}
	sourceRevisionInput := newLinkInput(d.ID, a.ID, contract.RelationTypeSpecificUnderFramework)
	sourceRevisionInput.ExpectedSourceRevision = 2
	_, err = service.LinkRelation(ctx, sourceRevisionInput)
	var revisionConflict *contract.ContractRelationRevisionConflictError
	if !errors.As(err, &revisionConflict) || revisionConflict.Side != "source" || revisionConflict.Expected != 2 || revisionConflict.Actual != 1 {
		t.Fatalf("source relation revision error = %v", err)
	}
	revisionInput := newLinkInput(d.ID, a.ID, contract.RelationTypeSpecificUnderFramework)
	revisionInput.ExpectedTargetRevision = 2
	_, err = service.LinkRelation(ctx, revisionInput)
	if !errors.As(err, &revisionConflict) || revisionConflict.Side != "target" || revisionConflict.Expected != 2 || revisionConflict.Actual != 1 {
		t.Fatalf("relation revision error = %v", err)
	}

	if _, err := service.LinkRelation(ctx, newLinkInput(b.ID, c.ID, contract.RelationTypeSubcontractOf)); err != nil {
		t.Fatalf("link second subcontract edge: %v", err)
	}
	_, err = service.LinkRelation(ctx, newLinkInput(c.ID, a.ID, contract.RelationTypeSubcontractOf))
	var cycle *contract.ContractRelationCycleError
	if !errors.As(err, &cycle) {
		t.Fatalf("indirect relation cycle error = %v", err)
	}
	if _, err := service.LinkRelation(ctx, newLinkInput(b.ID, a.ID, contract.RelationTypeReferences)); err != nil {
		t.Fatalf("reverse reference should be allowed: %v", err)
	}

	reverseCycleInputs := []contract.LinkContractRelationInput{
		newLinkInput(c.ID, d.ID, contract.RelationTypeFundingSourceFor),
		newLinkInput(d.ID, c.ID, contract.RelationTypeFundingSourceFor),
	}
	type relationOutcome struct{ err error }
	cycleStart := make(chan struct{})
	cycleOutcomes := make(chan relationOutcome, 2)
	for _, input := range reverseCycleInputs {
		input := input
		go func() {
			<-cycleStart
			_, linkErr := service.LinkRelation(ctx, input)
			cycleOutcomes <- relationOutcome{err: linkErr}
		}()
	}
	close(cycleStart)
	cycleSuccesses, cycleConflicts := 0, 0
	for range reverseCycleInputs {
		outcome := <-cycleOutcomes
		if outcome.err == nil {
			cycleSuccesses++
			continue
		}
		if !errors.As(outcome.err, &cycle) {
			t.Fatalf("reverse cycle outcome error = %v", outcome.err)
		}
		cycleConflicts++
	}
	if cycleSuccesses != 1 || cycleConflicts != 1 {
		t.Fatalf("reverse cycle successes=%d conflicts=%d", cycleSuccesses, cycleConflicts)
	}

	duplicateRaceInputs := []contract.LinkContractRelationInput{
		newLinkInput(c.ID, d.ID, contract.RelationTypeEntrustmentBasisFor),
		newLinkInput(c.ID, d.ID, contract.RelationTypeEntrustmentBasisFor),
	}
	duplicateStart := make(chan struct{})
	duplicateOutcomes := make(chan relationOutcome, 2)
	for _, input := range duplicateRaceInputs {
		input := input
		go func() {
			<-duplicateStart
			_, linkErr := service.LinkRelation(ctx, input)
			duplicateOutcomes <- relationOutcome{err: linkErr}
		}()
	}
	close(duplicateStart)
	duplicateSuccesses, duplicateConflicts := 0, 0
	for range duplicateRaceInputs {
		outcome := <-duplicateOutcomes
		if outcome.err == nil {
			duplicateSuccesses++
			continue
		}
		if !errors.As(outcome.err, &duplicateRelation) {
			t.Fatalf("duplicate race outcome error = %v", outcome.err)
		}
		duplicateConflicts++
	}
	if duplicateSuccesses != 1 || duplicateConflicts != 1 {
		t.Fatalf("duplicate race successes=%d conflicts=%d", duplicateSuccesses, duplicateConflicts)
	}

	aDetails, err := service.GetDraft(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetDraft(source relations) error = %v", err)
	}
	bDetails, err := service.GetDraft(ctx, b.ID)
	if err != nil {
		t.Fatalf("GetDraft(target relations) error = %v", err)
	}
	if len(aDetails.OutgoingRelations) != 6 || len(aDetails.IncomingRelations) != 1 || len(bDetails.IncomingRelations) != 6 {
		t.Fatalf("relation projections A(out=%d,in=%d) B(in=%d)", len(aDetails.OutgoingRelations), len(aDetails.IncomingRelations), len(bDetails.IncomingRelations))
	}
	var relationAuditCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM audit_entries
		WHERE object_id = $1 AND action = 'contract_relation_linked'
	`, a.ID).Scan(&relationAuditCount); err != nil {
		t.Fatalf("count relation audits: %v", err)
	}
	if relationAuditCount != 6 {
		t.Fatalf("source relation audit count = %d, want 6", relationAuditCount)
	}

	assertImmutableDatabaseFact(t, pool, `UPDATE contract_relations SET note = 'tampered' WHERE id = $1`, firstResult.Relation.ID)
	assertImmutableDatabaseFact(t, pool, `DELETE FROM contract_relations WHERE id = $1`, firstResult.Relation.ID)
	assertImmutableDatabaseFact(t, pool, `UPDATE contract_relation_commands SET note = 'tampered' WHERE command_id = $1`, firstInput.CommandID)
	assertImmutableDatabaseFact(t, pool, `DELETE FROM contract_relation_commands WHERE command_id = $1`, firstInput.CommandID)

	var existingAuditID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM audit_entries WHERE object_id = $1 LIMIT 1`, a.ID).Scan(&existingAuditID); err != nil {
		t.Fatalf("read existing relation fixture audit ID: %v", err)
	}
	atomicCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatal(err)
	}
	atomicRelationID, err := contract.NewCommandID()
	if err != nil {
		t.Fatal(err)
	}
	repository := contract.NewPostgresRepository(pool)
	_, _, _, err = repository.LinkRelation(ctx, contract.ContractRelationCommand{
		CommandID: atomicCommandID, AuditID: existingAuditID, RelationID: atomicRelationID,
		SourceContractID: c.ID, TargetContractID: d.ID, RelationType: contract.RelationTypeReferences,
		ExpectedSourceRevision: 1, ExpectedTargetRevision: 1,
		EffectiveOn: "2026-10-03", Note: "审计失败应回滚", Actor: integrationActor, OccurredAt: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("LinkRelation() with duplicate audit ID error = nil")
	}
	var atomicFactCount int
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM contract_relations WHERE id = $1) +
			(SELECT count(*) FROM contract_relation_commands WHERE command_id = $2)
	`, atomicRelationID, atomicCommandID).Scan(&atomicFactCount); err != nil {
		t.Fatalf("count failed relation facts: %v", err)
	}
	if atomicFactCount != 0 {
		t.Fatalf("audit failure left %d relation facts", atomicFactCount)
	}
}

func TestPostgresRepositoryDraftPartyLifecycle(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("second database.Migrate() error = %v", err)
	}

	var cleanupContractID string
	cleanupPartyIDs := make([]string, 0, 2)
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		auditObjectIDs := append([]string{}, cleanupPartyIDs...)
		if cleanupContractID != "" {
			auditObjectIDs = append(auditObjectIDs, cleanupContractID)
		}
		cleanupAuditEntries(t, pool, auditObjectIDs)
		if cleanupContractID != "" {
			if _, err := pool.Exec(cleanupCtx, "DELETE FROM contracts WHERE id = $1", cleanupContractID); err != nil {
				t.Errorf("clean up contract %q: %v", cleanupContractID, err)
			}
		}
		for _, partyID := range cleanupPartyIDs {
			if _, err := pool.Exec(cleanupCtx, "DELETE FROM parties WHERE id = $1", partyID); err != nil {
				t.Errorf("clean up party %q: %v", partyID, err)
			}
		}
	}()

	seedID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	suffix := strings.ToUpper(seedID[len(seedID)-8:])
	partyService := party.NewService(party.NewPostgresRepository(pool))
	firstParty, err := partyService.Create(ctx, party.CreateInput{
		Actor:                  integrationActor,
		Kind:                   "organization",
		LegalName:              "签约主体集成测试一",
		RegistrationIdentifier: "DRAFT-PARTY-A-" + suffix,
	})
	if err != nil {
		t.Fatalf("create first party: %v", err)
	}
	cleanupPartyIDs = append(cleanupPartyIDs, firstParty.ID)
	secondParty, err := partyService.Create(ctx, party.CreateInput{
		Actor:                  integrationActor,
		Kind:                   "organization",
		LegalName:              "签约主体集成测试二",
		RegistrationIdentifier: "DRAFT-PARTY-B-" + suffix,
	})
	if err != nil {
		t.Fatalf("create second party: %v", err)
	}
	cleanupPartyIDs = append(cleanupPartyIDs, secondParty.ID)

	contractService := contract.NewService(contract.NewPostgresRepository(pool))
	created, err := contractService.CreateDraft(ctx, contract.CreateDraftInput{
		Actor:           integrationActor,
		CommandID:       seedID,
		NumberNamespace: "integration-party",
		ContractNumber:  "PARTY-" + suffix,
		Title:           "合同签约主体集成测试",
		Currency:        "CNY",
	})
	if err != nil {
		t.Fatalf("CreateDraft() error = %v", err)
	}
	cleanupContractID = created.ID

	details, err := contractService.GetDraft(ctx, created.ID)
	if err != nil {
		t.Fatalf("initial GetDraft() error = %v", err)
	}
	if details.Contract.DraftVersion != 1 || len(details.Parties) != 0 {
		t.Fatalf("initial details = %#v", details)
	}

	addFirstCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	addFirstInput := contract.AddDraftPartyInput{
		Actor:                integrationActor,
		CommandID:            addFirstCommandID,
		ContractID:           created.ID,
		PartyID:              firstParty.ID,
		Role:                 "委托方",
		ExpectedDraftVersion: 1,
	}
	addedFirst, err := contractService.AddDraftParty(ctx, addFirstInput)
	if err != nil {
		t.Fatalf("AddDraftParty(first) error = %v", err)
	}
	if addedFirst.DraftVersion != 2 || addedFirst.Replayed || addedFirst.Party.PartyLegalName != firstParty.LegalName {
		t.Fatalf("AddDraftParty(first) = %#v", addedFirst)
	}
	replayedAdd, err := contractService.AddDraftParty(ctx, addFirstInput)
	if err != nil {
		t.Fatalf("replayed AddDraftParty() error = %v", err)
	}
	if !replayedAdd.Replayed || replayedAdd.Party.ID != addedFirst.Party.ID || replayedAdd.DraftVersion != 2 {
		t.Fatalf("replayed AddDraftParty() = %#v", replayedAdd)
	}
	changedAdd := addFirstInput
	changedAdd.Role = "受托方"
	_, err = contractService.AddDraftParty(ctx, changedAdd)
	var idempotencyConflict *contract.IdempotencyConflictError
	if !errors.As(err, &idempotencyConflict) {
		t.Fatalf("changed add replay error = %v, want IdempotencyConflictError", err)
	}

	addRoleCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	addedRole, err := contractService.AddDraftParty(ctx, contract.AddDraftPartyInput{
		Actor:                integrationActor,
		CommandID:            addRoleCommandID,
		ContractID:           created.ID,
		PartyID:              firstParty.ID,
		Role:                 "受托方",
		ExpectedDraftVersion: 2,
	})
	if err != nil || addedRole.DraftVersion != 3 {
		t.Fatalf("AddDraftParty(second role) = %#v, error = %v", addedRole, err)
	}

	addSecondCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	addedSecond, err := contractService.AddDraftParty(ctx, contract.AddDraftPartyInput{
		Actor:                integrationActor,
		CommandID:            addSecondCommandID,
		ContractID:           created.ID,
		PartyID:              secondParty.ID,
		Role:                 "委托方",
		ExpectedDraftVersion: 3,
	})
	if err != nil || addedSecond.DraftVersion != 4 {
		t.Fatalf("AddDraftParty(second party) = %#v, error = %v", addedSecond, err)
	}

	duplicateCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	_, err = contractService.AddDraftParty(ctx, contract.AddDraftPartyInput{
		Actor:                integrationActor,
		CommandID:            duplicateCommandID,
		ContractID:           created.ID,
		PartyID:              firstParty.ID,
		Role:                 "委托方",
		ExpectedDraftVersion: 4,
	})
	var duplicate *contract.DuplicateDraftPartyError
	if !errors.As(err, &duplicate) {
		t.Fatalf("duplicate active link error = %v, want DuplicateDraftPartyError", err)
	}

	staleCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	_, err = contractService.AddDraftParty(ctx, contract.AddDraftPartyInput{
		Actor:                integrationActor,
		CommandID:            staleCommandID,
		ContractID:           created.ID,
		PartyID:              secondParty.ID,
		Role:                 "保证方",
		ExpectedDraftVersion: 3,
	})
	var stale *contract.DraftVersionConflictError
	if !errors.As(err, &stale) || stale.Actual != 4 {
		t.Fatalf("stale version error = %#v, want actual version 4", err)
	}
	missingPartyCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	missingPartyID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	_, err = contractService.AddDraftParty(ctx, contract.AddDraftPartyInput{
		Actor:                integrationActor,
		CommandID:            missingPartyCommandID,
		ContractID:           created.ID,
		PartyID:              missingPartyID,
		Role:                 "保证方",
		ExpectedDraftVersion: 4,
	})
	var missingParty *contract.PartyNotFoundError
	if !errors.As(err, &missingParty) {
		t.Fatalf("missing party error = %v, want PartyNotFoundError", err)
	}

	removeCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	removeInput := contract.RemoveDraftPartyInput{
		Actor:                integrationActor,
		CommandID:            removeCommandID,
		ContractID:           created.ID,
		PartyLinkID:          addedFirst.Party.ID,
		ExpectedDraftVersion: 4,
	}
	removed, err := contractService.RemoveDraftParty(ctx, removeInput)
	if err != nil {
		t.Fatalf("RemoveDraftParty() error = %v", err)
	}
	if removed.DraftVersion != 5 || removed.Replayed || removed.Party.Active() {
		t.Fatalf("RemoveDraftParty() = %#v", removed)
	}
	replayedRemove, err := contractService.RemoveDraftParty(ctx, removeInput)
	if err != nil {
		t.Fatalf("replayed RemoveDraftParty() error = %v", err)
	}
	if !replayedRemove.Replayed || replayedRemove.DraftVersion != 5 {
		t.Fatalf("replayed RemoveDraftParty() = %#v", replayedRemove)
	}
	missingLinkCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	missingLinkID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	_, err = contractService.RemoveDraftParty(ctx, contract.RemoveDraftPartyInput{
		Actor:                integrationActor,
		CommandID:            missingLinkCommandID,
		ContractID:           created.ID,
		PartyLinkID:          missingLinkID,
		ExpectedDraftVersion: 5,
	})
	var missingLink *contract.DraftPartyNotFoundError
	if !errors.As(err, &missingLink) {
		t.Fatalf("missing link error = %v, want DraftPartyNotFoundError", err)
	}

	_, err = contractService.RemoveDraftParty(ctx, contract.RemoveDraftPartyInput{
		Actor:                integrationActor,
		CommandID:            addFirstCommandID,
		ContractID:           created.ID,
		PartyLinkID:          addedRole.Party.ID,
		ExpectedDraftVersion: 5,
	})
	if !errors.As(err, &idempotencyConflict) {
		t.Fatalf("cross-action command reuse error = %v, want IdempotencyConflictError", err)
	}

	readdCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	readded, err := contractService.AddDraftParty(ctx, contract.AddDraftPartyInput{
		Actor:                integrationActor,
		CommandID:            readdCommandID,
		ContractID:           created.ID,
		PartyID:              firstParty.ID,
		Role:                 "委托方",
		ExpectedDraftVersion: 5,
	})
	if err != nil || readded.DraftVersion != 6 || readded.Party.ID == addedFirst.Party.ID {
		t.Fatalf("re-add removed role = %#v, error = %v", readded, err)
	}

	details, err = contractService.GetDraft(ctx, created.ID)
	if err != nil {
		t.Fatalf("final GetDraft() error = %v", err)
	}
	if details.Contract.DraftVersion != 6 || len(details.Parties) != 3 {
		t.Fatalf("final details = %#v", details)
	}

	var activeCount, commandCount, removedCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM contract_draft_parties WHERE contract_id = $1 AND removed_at IS NULL`, created.ID).Scan(&activeCount); err != nil {
		t.Fatalf("count active draft parties: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM contract_draft_party_commands WHERE contract_id = $1`, created.ID).Scan(&commandCount); err != nil {
		t.Fatalf("count draft party commands: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM contract_draft_parties WHERE contract_id = $1 AND removed_at IS NOT NULL`, created.ID).Scan(&removedCount); err != nil {
		t.Fatalf("count removed draft parties: %v", err)
	}
	if activeCount != 3 || removedCount != 1 || commandCount != 5 {
		t.Fatalf("stored counts active=%d removed=%d commands=%d", activeCount, removedCount, commandCount)
	}
	var addedAuditCount, removedAuditCount int
	if err := pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE action = 'contract_draft_party_added'),
			count(*) FILTER (WHERE action = 'contract_draft_party_removed')
		FROM audit_entries
		WHERE object_type = 'contract' AND object_id = $1
	`, created.ID).Scan(&addedAuditCount, &removedAuditCount); err != nil {
		t.Fatalf("count draft party audits: %v", err)
	}
	if addedAuditCount != 4 || removedAuditCount != 1 {
		t.Fatalf("draft party audit counts added=%d removed=%d", addedAuditCount, removedAuditCount)
	}

	if _, err := pool.Exec(ctx, `UPDATE contracts SET status = 'active', current_revision = 1 WHERE id = $1`, created.ID); err != nil {
		t.Fatalf("prepare non-draft contract: %v", err)
	}
	nonDraftCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	_, err = contractService.AddDraftParty(ctx, contract.AddDraftPartyInput{
		Actor:                integrationActor,
		CommandID:            nonDraftCommandID,
		ContractID:           created.ID,
		PartyID:              firstParty.ID,
		Role:                 "见证方",
		ExpectedDraftVersion: 6,
	})
	var stateConflict *contract.ContractStateConflictError
	if !errors.As(err, &stateConflict) {
		t.Fatalf("non-draft mutation error = %v, want ContractStateConflictError", err)
	}
}

func TestPostgresRepositoryDraftEditAndCancel(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("second database.Migrate() error = %v", err)
	}

	cleanupContractIDs := make([]string, 0, 2)
	cleanupPartyIDs := make([]string, 0, 1)
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		auditObjectIDs := append(append([]string{}, cleanupContractIDs...), cleanupPartyIDs...)
		cleanupAuditEntries(t, pool, auditObjectIDs)
		for _, contractID := range cleanupContractIDs {
			if _, err := pool.Exec(cleanupCtx, "DELETE FROM contracts WHERE id = $1", contractID); err != nil {
				t.Errorf("clean up contract %q: %v", contractID, err)
			}
		}
		for _, partyID := range cleanupPartyIDs {
			if _, err := pool.Exec(cleanupCtx, "DELETE FROM parties WHERE id = $1", partyID); err != nil {
				t.Errorf("clean up party %q: %v", partyID, err)
			}
		}
	}()

	seedID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	suffix := strings.ToUpper(seedID[len(seedID)-8:])
	partyService := party.NewService(party.NewPostgresRepository(pool))
	linkedParty, err := partyService.Create(ctx, party.CreateInput{
		Actor:                  integrationActor,
		Kind:                   "organization",
		LegalName:              "草稿编辑取消集成主体",
		RegistrationIdentifier: "DRAFT-CHANGE-" + suffix,
	})
	if err != nil {
		t.Fatalf("create party: %v", err)
	}
	cleanupPartyIDs = append(cleanupPartyIDs, linkedParty.ID)

	service := contract.NewService(contract.NewPostgresRepository(pool))
	created, err := service.CreateDraft(ctx, contract.CreateDraftInput{
		Actor:           integrationActor,
		CommandID:       seedID,
		NumberNamespace: "change-integration",
		ContractNumber:  "ORIGINAL-" + suffix,
		Title:           "编辑前合同草稿",
		Category:        "before",
		Currency:        "CNY",
		TotalAmount:     "1000.50",
		SignedOn:        "2026-09-19",
	})
	if err != nil {
		t.Fatalf("CreateDraft() error = %v", err)
	}
	cleanupContractIDs = append(cleanupContractIDs, created.ID)
	originalCreatedAt := created.CreatedAt
	originalCreateCommandID := created.CommandID

	duplicateCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	duplicateTarget, err := service.CreateDraft(ctx, contract.CreateDraftInput{
		Actor:           integrationActor,
		CommandID:       duplicateCommandID,
		NumberNamespace: "change-integration",
		ContractNumber:  "DUPLICATE-" + suffix,
		Title:           "编号占用合同",
		Currency:        "CNY",
	})
	if err != nil {
		t.Fatalf("create duplicate target: %v", err)
	}
	cleanupContractIDs = append(cleanupContractIDs, duplicateTarget.ID)

	addPartyCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	added, err := service.AddDraftParty(ctx, contract.AddDraftPartyInput{
		Actor:                integrationActor,
		CommandID:            addPartyCommandID,
		ContractID:           created.ID,
		PartyID:              linkedParty.ID,
		Role:                 "委托方",
		ExpectedDraftVersion: 1,
	})
	if err != nil || added.DraftVersion != 2 {
		t.Fatalf("AddDraftParty() = %#v, error = %v", added, err)
	}

	updateCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	updateInput := contract.UpdateDraftInput{
		Actor:                integrationActor,
		CommandID:            updateCommandID,
		ContractID:           created.ID,
		ExpectedDraftVersion: 2,
		NumberNamespace:      " revised ",
		ContractNumber:       " EDITED-" + suffix + " ",
		Title:                " 编辑后的合同草稿 ",
		Category:             " 服务 ",
		Currency:             " usd ",
		TotalAmount:          " 0002000.250000 ",
		SignedOn:             "2026-09-20",
		EffectiveOn:          "2026-09-01",
	}
	updated, err := service.UpdateDraft(ctx, updateInput)
	if err != nil {
		t.Fatalf("UpdateDraft() error = %v", err)
	}
	if updated.DraftVersion != 3 || updated.Replayed {
		t.Fatalf("UpdateDraft() = %#v", updated)
	}
	replayedUpdate, err := service.UpdateDraft(ctx, updateInput)
	if err != nil || !replayedUpdate.Replayed || replayedUpdate.DraftVersion != 3 {
		t.Fatalf("replayed UpdateDraft() = %#v, error = %v", replayedUpdate, err)
	}
	changedReplay := updateInput
	changedReplay.Title = "复用命令修改负载"
	_, err = service.UpdateDraft(ctx, changedReplay)
	var idempotencyConflict *contract.IdempotencyConflictError
	if !errors.As(err, &idempotencyConflict) {
		t.Fatalf("changed update replay error = %v, want IdempotencyConflictError", err)
	}

	details, err := service.GetDraft(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetDraft() after update error = %v", err)
	}
	if details.Contract.NumberNamespace != "revised" || details.Contract.Title != "编辑后的合同草稿" || details.Contract.TotalAmount != "2000.25" || details.Contract.Currency != "USD" {
		t.Fatalf("updated contract = %#v", details.Contract)
	}
	if details.Contract.CreatedAt != originalCreatedAt || details.Contract.CommandID != originalCreateCommandID || len(details.Parties) != 1 || details.Parties[0].ID != added.Party.ID {
		t.Fatalf("stable identity or linked parties changed: %#v", details)
	}

	noChangeCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	noChangeInput := updateInput
	noChangeInput.CommandID = noChangeCommandID
	noChangeInput.ExpectedDraftVersion = 3
	_, err = service.UpdateDraft(ctx, noChangeInput)
	var noChanges *contract.NoDraftChangesError
	if !errors.As(err, &noChanges) {
		t.Fatalf("no-change update error = %v, want NoDraftChangesError", err)
	}

	duplicateUpdateCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	duplicateUpdate := noChangeInput
	duplicateUpdate.CommandID = duplicateUpdateCommandID
	duplicateUpdate.NumberNamespace = duplicateTarget.NumberNamespace
	duplicateUpdate.ContractNumber = strings.ToLower(duplicateTarget.ContractNumber)
	_, err = service.UpdateDraft(ctx, duplicateUpdate)
	var duplicateNumber *contract.DuplicateNumberError
	if !errors.As(err, &duplicateNumber) {
		t.Fatalf("duplicate number update error = %v, want DuplicateNumberError", err)
	}

	staleCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	staleUpdate := noChangeInput
	staleUpdate.CommandID = staleCommandID
	staleUpdate.ExpectedDraftVersion = 2
	staleUpdate.Title = "过期编辑"
	_, err = service.UpdateDraft(ctx, staleUpdate)
	var versionConflict *contract.DraftVersionConflictError
	if !errors.As(err, &versionConflict) || versionConflict.Actual != 3 {
		t.Fatalf("stale update error = %#v, want actual version 3", err)
	}

	cancelCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	cancelInput := contract.CancelDraftInput{
		Actor:                integrationActor,
		CommandID:            cancelCommandID,
		ContractID:           created.ID,
		ExpectedDraftVersion: 3,
		Reason:               " 双方决定终止签署准备 ",
	}
	cancelled, err := service.CancelDraft(ctx, cancelInput)
	if err != nil {
		t.Fatalf("CancelDraft() error = %v", err)
	}
	if cancelled.DraftVersion != 4 || cancelled.Status != contract.StatusCancelled || cancelled.Replayed {
		t.Fatalf("CancelDraft() = %#v", cancelled)
	}
	replayedCancel, err := service.CancelDraft(ctx, cancelInput)
	if err != nil || !replayedCancel.Replayed || replayedCancel.DraftVersion != 4 {
		t.Fatalf("replayed CancelDraft() = %#v, error = %v", replayedCancel, err)
	}
	changedCancel := cancelInput
	changedCancel.Reason = "另一个取消原因"
	_, err = service.CancelDraft(ctx, changedCancel)
	if !errors.As(err, &idempotencyConflict) {
		t.Fatalf("changed cancel replay error = %v, want IdempotencyConflictError", err)
	}

	details, err = service.GetDraft(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetDraft() after cancel error = %v", err)
	}
	if details.Contract.Status != contract.StatusCancelled || details.Contract.DraftVersion != 4 || details.Contract.CurrentRevision != 0 {
		t.Fatalf("cancelled contract = %#v", details.Contract)
	}
	if details.Cancellation == nil || details.Cancellation.Reason != "双方决定终止签署准备" || details.Cancellation.DraftVersion != 4 || details.Cancellation.CancelledAt.IsZero() {
		t.Fatalf("cancellation = %#v", details.Cancellation)
	}
	if len(details.Parties) != 1 || details.Parties[0].ID != added.Party.ID {
		t.Fatalf("cancel changed linked parties: %#v", details.Parties)
	}

	postCancelUpdateCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	postCancelUpdate := noChangeInput
	postCancelUpdate.CommandID = postCancelUpdateCommandID
	postCancelUpdate.ExpectedDraftVersion = 4
	postCancelUpdate.Title = "取消后编辑"
	_, err = service.UpdateDraft(ctx, postCancelUpdate)
	var stateConflict *contract.ContractStateConflictError
	if !errors.As(err, &stateConflict) {
		t.Fatalf("post-cancel update error = %v, want ContractStateConflictError", err)
	}

	postCancelAddCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	_, err = service.AddDraftParty(ctx, contract.AddDraftPartyInput{
		Actor:                integrationActor,
		CommandID:            postCancelAddCommandID,
		ContractID:           created.ID,
		PartyID:              linkedParty.ID,
		Role:                 "受托方",
		ExpectedDraftVersion: 4,
	})
	if !errors.As(err, &stateConflict) {
		t.Fatalf("post-cancel party mutation error = %v, want ContractStateConflictError", err)
	}

	secondCancelCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	_, err = service.CancelDraft(ctx, contract.CancelDraftInput{
		Actor:                integrationActor,
		CommandID:            secondCancelCommandID,
		ContractID:           created.ID,
		ExpectedDraftVersion: 4,
		Reason:               "再次取消",
	})
	if !errors.As(err, &stateConflict) {
		t.Fatalf("second cancel error = %v, want ContractStateConflictError", err)
	}

	crossActionUpdate := postCancelUpdate
	crossActionUpdate.CommandID = cancelCommandID
	crossActionUpdate.ExpectedDraftVersion = 3
	_, err = service.UpdateDraft(ctx, crossActionUpdate)
	if !errors.As(err, &idempotencyConflict) {
		t.Fatalf("cross-action command reuse error = %v, want IdempotencyConflictError", err)
	}

	var changeCommandCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM contract_draft_change_commands WHERE contract_id = $1`, created.ID).Scan(&changeCommandCount); err != nil {
		t.Fatalf("count draft change commands: %v", err)
	}
	if changeCommandCount != 2 {
		t.Fatalf("draft change command count = %d, want 2", changeCommandCount)
	}
	var updateAuditCount, cancelAuditCount int
	if err := pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE action = 'contract_draft_updated'),
			count(*) FILTER (WHERE action = 'contract_draft_cancelled')
		FROM audit_entries
		WHERE object_type = 'contract' AND object_id = $1
	`, created.ID).Scan(&updateAuditCount, &cancelAuditCount); err != nil {
		t.Fatalf("count draft change audits: %v", err)
	}
	if updateAuditCount != 1 || cancelAuditCount != 1 {
		t.Fatalf("draft change audit counts update=%d cancel=%d", updateAuditCount, cancelAuditCount)
	}
}

func TestPostgresRepositoryActivationRevisionAndImmutability(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("second database.Migrate() error = %v", err)
	}

	contractIDs := make([]string, 0, 4)
	partyIDs := make([]string, 0, 2)
	defer func() { cleanupActivationFixtures(t, pool, contractIDs, partyIDs) }()

	seedID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	suffix := strings.ToUpper(seedID[len(seedID)-8:])
	service := contract.NewService(contract.NewPostgresRepository(pool))
	partyService := party.NewService(party.NewPostgresRepository(pool))
	actor := integrationActor

	createDraft := func(number, title, signedOn, effectiveOn string) contract.Contract {
		t.Helper()
		commandID, err := contract.NewCommandID()
		if err != nil {
			t.Fatalf("NewCommandID() error = %v", err)
		}
		created, err := service.CreateDraft(ctx, contract.CreateDraftInput{
			Actor:           actor,
			CommandID:       commandID,
			NumberNamespace: "activation-integration",
			ContractNumber:  number + "-" + suffix,
			Title:           title,
			Category:        "activation-test",
			Currency:        "CNY",
			TotalAmount:     "0012345.670000",
			SignedOn:        signedOn,
			EffectiveOn:     effectiveOn,
		})
		if err != nil {
			t.Fatalf("CreateDraft(%q) error = %v", title, err)
		}
		contractIDs = append(contractIDs, created.ID)
		return created
	}

	missingSigned := createDraft("MISSING-SIGNED", "缺少签署日期激活测试", "", "2026-10-01")
	missingSignedCommand, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	_, err = service.Activate(ctx, contract.ActivateContractInput{
		CommandID: missingSignedCommand, ContractID: missingSigned.ID, ExpectedDraftVersion: 1, Actor: actor,
	})
	var precondition *contract.ActivationPreconditionError
	if !errors.As(err, &precondition) || precondition.Field != "signed_on" {
		t.Fatalf("missing signed date error = %v, want signed_on precondition", err)
	}
	draftTerminationCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	_, err = service.Terminate(ctx, contract.TerminateContractInput{
		CommandID: draftTerminationCommandID, ContractID: missingSigned.ID, ExpectedRevision: 1,
		InstrumentNumber: "DRAFT-TERM-" + suffix, Title: "草稿终止文书", TerminationReason: "草稿状态必须拒绝",
		SignedOn: "2026-10-02", EffectiveOn: "2026-10-02", EffectiveOrder: 2, Actor: actor,
	})
	var terminationStateConflict *contract.ContractTerminationStateConflictError
	if !errors.As(err, &terminationStateConflict) || terminationStateConflict.Status != contract.StatusDraft {
		t.Fatalf("draft termination error = %v", err)
	}

	cancelledForTermination := createDraft("CANCELLED-TERMINATION", "已取消合同终止测试", "2026-09-19", "2026-10-01")
	cancelCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	if _, err := service.CancelDraft(ctx, contract.CancelDraftInput{
		CommandID: cancelCommandID, ContractID: cancelledForTermination.ID,
		ExpectedDraftVersion: 1, Reason: "取消后不能终止", Actor: actor,
	}); err != nil {
		t.Fatalf("CancelDraft(cancelled termination fixture) error = %v", err)
	}
	cancelledTerminationCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	_, err = service.Terminate(ctx, contract.TerminateContractInput{
		CommandID: cancelledTerminationCommandID, ContractID: cancelledForTermination.ID, ExpectedRevision: 1,
		InstrumentNumber: "CANCELLED-TERM-" + suffix, Title: "已取消合同终止文书", TerminationReason: "已取消状态必须拒绝",
		SignedOn: "2026-10-02", EffectiveOn: "2026-10-02", EffectiveOrder: 2, Actor: actor,
	})
	if !errors.As(err, &terminationStateConflict) || terminationStateConflict.Status != contract.StatusCancelled {
		t.Fatalf("cancelled contract termination error = %v", err)
	}

	missingEffective := createDraft("MISSING-EFFECTIVE", "缺少生效日期激活测试", "2026-09-19", "")
	missingEffectiveCommand, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	_, err = service.Activate(ctx, contract.ActivateContractInput{
		CommandID: missingEffectiveCommand, ContractID: missingEffective.ID, ExpectedDraftVersion: 1, Actor: actor,
	})
	if !errors.As(err, &precondition) || precondition.Field != "effective_on" {
		t.Fatalf("missing effective date error = %v, want effective_on precondition", err)
	}

	firstParty, err := partyService.Create(ctx, party.CreateInput{
		Actor: actor,
		Kind:  "organization", LegalName: "激活集成主体甲", RegistrationIdentifier: "ACT-A-" + suffix,
	})
	if err != nil {
		t.Fatalf("create first party: %v", err)
	}
	partyIDs = append(partyIDs, firstParty.ID)
	secondParty, err := partyService.Create(ctx, party.CreateInput{
		Actor: actor,
		Kind:  "organization", LegalName: "激活集成主体乙", RegistrationIdentifier: "ACT-B-" + suffix,
	})
	if err != nil {
		t.Fatalf("create second party: %v", err)
	}
	partyIDs = append(partyIDs, secondParty.ID)

	primary := createDraft("PRIMARY", "激活与 revision 1 集成测试", "2026-09-19", "2026-10-01")
	addParty := func(contractID, partyID, role string, expectedVersion int64) contract.DraftPartyMutationResult {
		t.Helper()
		commandID, err := contract.NewCommandID()
		if err != nil {
			t.Fatalf("NewCommandID() error = %v", err)
		}
		result, err := service.AddDraftParty(ctx, contract.AddDraftPartyInput{
			Actor:     actor,
			CommandID: commandID, ContractID: contractID, PartyID: partyID, Role: role, ExpectedDraftVersion: expectedVersion,
		})
		if err != nil {
			t.Fatalf("AddDraftParty(%q) error = %v", role, err)
		}
		return result
	}
	activateTwoParty := func(number, title string) contract.Contract {
		t.Helper()
		created := createDraft(number, title, "2026-09-19", "2026-10-01")
		addParty(created.ID, firstParty.ID, "委托方", 1)
		addParty(created.ID, secondParty.ID, "受托方", 2)
		commandID, err := contract.NewCommandID()
		if err != nil {
			t.Fatalf("NewCommandID() error = %v", err)
		}
		if _, err := service.Activate(ctx, contract.ActivateContractInput{
			CommandID: commandID, ContractID: created.ID, ExpectedDraftVersion: 3, Actor: actor,
		}); err != nil {
			t.Fatalf("Activate(%q) error = %v", title, err)
		}
		return created
	}
	addParty(primary.ID, firstParty.ID, "委托方", 1)
	addParty(primary.ID, firstParty.ID, "付款方", 2)

	onePartyCommand, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	_, err = service.Activate(ctx, contract.ActivateContractInput{
		CommandID: onePartyCommand, ContractID: primary.ID, ExpectedDraftVersion: 3, Actor: actor,
	})
	if !errors.As(err, &precondition) || precondition.Field != "parties" {
		t.Fatalf("one distinct party error = %v, want parties precondition", err)
	}
	addParty(primary.ID, secondParty.ID, "受托方", 3)

	staleCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	_, err = service.Activate(ctx, contract.ActivateContractInput{
		CommandID: staleCommandID, ContractID: primary.ID, ExpectedDraftVersion: 3, Actor: actor,
	})
	var versionConflict *contract.DraftVersionConflictError
	if !errors.As(err, &versionConflict) || versionConflict.Actual != 4 {
		t.Fatalf("stale activation error = %#v, want actual version 4", err)
	}

	activateCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	activateInput := contract.ActivateContractInput{
		CommandID: activateCommandID, ContractID: primary.ID, ExpectedDraftVersion: 4, Actor: actor,
	}
	activated, err := service.Activate(ctx, activateInput)
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	if activated.Revision != 1 || activated.Replayed {
		t.Fatalf("Activate() = %#v", activated)
	}
	replayed, err := service.Activate(ctx, activateInput)
	if err != nil || !replayed.Replayed || replayed.Revision != 1 {
		t.Fatalf("replayed Activate() = %#v, error = %v", replayed, err)
	}
	changedReplay := activateInput
	changedReplay.Actor.ID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	_, err = service.Activate(ctx, changedReplay)
	var idempotencyConflict *contract.IdempotencyConflictError
	if !errors.As(err, &idempotencyConflict) {
		t.Fatalf("changed activation replay error = %v, want IdempotencyConflictError", err)
	}
	differentCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	_, err = service.Activate(ctx, contract.ActivateContractInput{
		CommandID: differentCommandID, ContractID: primary.ID, ExpectedDraftVersion: 4, Actor: actor,
	})
	var stateConflict *contract.ContractStateConflictError
	if !errors.As(err, &stateConflict) {
		t.Fatalf("second activation error = %v, want ContractStateConflictError", err)
	}

	details, err := service.GetDraft(ctx, primary.ID)
	if err != nil {
		t.Fatalf("GetDraft(active) error = %v", err)
	}
	if details.Contract.Status != contract.StatusActive || details.Contract.CurrentRevision != 1 || details.Contract.DraftVersion != 4 {
		t.Fatalf("active contract = %#v", details.Contract)
	}
	if details.Revision == nil || details.Revision.Revision != 1 || details.Revision.SourceDraftVersion != 4 || details.Revision.TotalAmount != "12345.67" || details.Revision.Actor != actor {
		t.Fatalf("revision = %#v", details.Revision)
	}
	if len(details.Revisions) != 1 || len(details.Instruments) != 1 ||
		details.Instruments[0].InstrumentType != contract.InstrumentTypeOriginalContract ||
		details.Instruments[0].EffectiveOrder != 1 || details.Instruments[0].ResultingRevision != 1 {
		t.Fatalf("initial instrument timeline revisions=%#v instruments=%#v", details.Revisions, details.Instruments)
	}
	if len(details.PartySnapshots) != 3 || details.PartySnapshots[0].LegalName != firstParty.LegalName || details.PartySnapshots[2].LegalName != secondParty.LegalName {
		t.Fatalf("party snapshots = %#v", details.PartySnapshots)
	}
	if len(details.AuditEntries) != 5 ||
		details.AuditEntries[len(details.AuditEntries)-1].EventType != contract.AuditEventContractActivated ||
		details.AuditEntries[len(details.AuditEntries)-1].Actor != actor {
		t.Fatalf("audit entries = %#v", details.AuditEntries)
	}

	if _, err := pool.Exec(ctx, `UPDATE parties SET legal_name = '激活后当前名称', revision = revision + 1, updated_at = clock_timestamp() WHERE id = $1`, firstParty.ID); err != nil {
		t.Fatalf("change current party after activation: %v", err)
	}
	detailsAfterPartyChange, err := service.GetDraft(ctx, primary.ID)
	if err != nil {
		t.Fatalf("GetDraft() after party change error = %v", err)
	}
	if detailsAfterPartyChange.PartySnapshots[0].LegalName != firstParty.LegalName || detailsAfterPartyChange.PartySnapshots[0].SourcePartyRevision != firstParty.Revision {
		t.Fatalf("party snapshot changed with current party: %#v", detailsAfterPartyChange.PartySnapshots[0])
	}

	changeCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	changeInput := contract.AppendContractChangeInput{
		CommandID: changeCommandID, ContractID: primary.ID, ExpectedRevision: 1,
		InstrumentNumber: "SUPP-1-" + suffix, Title: "第一份生效补充协议",
		ChangeSummary: "精确减少合同金额", SignedOn: "2026-10-02", EffectiveOn: "2026-10-02",
		EffectiveOrder: 2, AmountEffect: contract.AmountEffectAdjustment, AmountValue: "-345.67", Actor: actor,
	}
	changed, err := service.AppendChange(ctx, changeInput)
	if err != nil {
		t.Fatalf("AppendChange(adjustment) error = %v", err)
	}
	if changed.Revision != 2 || changed.EffectiveAmount != "12000" || changed.Replayed {
		t.Fatalf("AppendChange(adjustment) = %#v", changed)
	}
	replayedChange, err := service.AppendChange(ctx, changeInput)
	if err != nil || !replayedChange.Replayed || replayedChange.Revision != 2 || replayedChange.InstrumentID != changed.InstrumentID {
		t.Fatalf("replayed AppendChange() = %#v, error=%v", replayedChange, err)
	}
	changedPayload := changeInput
	changedPayload.Title = "篡改后的补充协议"
	_, err = service.AppendChange(ctx, changedPayload)
	if !errors.As(err, &idempotencyConflict) {
		t.Fatalf("changed supplement replay error = %v, want IdempotencyConflictError", err)
	}

	staleChangeCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	staleChange := changeInput
	staleChange.CommandID = staleChangeCommandID
	staleChange.InstrumentNumber = "STALE-" + suffix
	_, err = service.AppendChange(ctx, staleChange)
	var revisionConflict *contract.ContractRevisionConflictError
	if !errors.As(err, &revisionConflict) || revisionConflict.Actual != 2 {
		t.Fatalf("stale supplement error = %v, want actual revision 2", err)
	}

	backdatedCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	backdated := changeInput
	backdated.CommandID = backdatedCommandID
	backdated.ExpectedRevision = 2
	backdated.EffectiveOrder = 3
	backdated.InstrumentNumber = "BACKDATED-" + suffix
	backdated.EffectiveOn = "2026-09-30"
	_, err = service.AppendChange(ctx, backdated)
	var changePrecondition *contract.ContractChangePreconditionError
	if !errors.As(err, &changePrecondition) || changePrecondition.Field != "instrument_effective_on" {
		t.Fatalf("backdated supplement error = %v", err)
	}

	duplicateNumberCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	duplicateNumber := changeInput
	duplicateNumber.CommandID = duplicateNumberCommandID
	duplicateNumber.ExpectedRevision = 2
	duplicateNumber.EffectiveOrder = 3
	duplicateNumber.InstrumentNumber = strings.ToLower(changeInput.InstrumentNumber)
	duplicateNumber.EffectiveOn = "2026-10-03"
	_, err = service.AppendChange(ctx, duplicateNumber)
	var duplicateInstrument *contract.DuplicateInstrumentNumberError
	if !errors.As(err, &duplicateInstrument) {
		t.Fatalf("duplicate instrument number error = %v", err)
	}

	restatementCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	restatement := changeInput
	restatement.CommandID = restatementCommandID
	restatement.ExpectedRevision = 2
	restatement.EffectiveOrder = 3
	restatement.InstrumentNumber = "SUPP-2-" + suffix
	restatement.Title = "第二份生效补充协议"
	restatement.ChangeSummary = "重述当前有效金额"
	restatement.EffectiveOn = "2026-10-03"
	restatement.AmountEffect = contract.AmountEffectRestatement
	restatement.AmountValue = "15000.125000"
	restated, err := service.AppendChange(ctx, restatement)
	if err != nil || restated.Revision != 3 || restated.EffectiveAmount != "15000.125" {
		t.Fatalf("AppendChange(restatement) = %#v, error=%v", restated, err)
	}

	unchangedCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	unchanged := restatement
	unchanged.CommandID = unchangedCommandID
	unchanged.ExpectedRevision = 3
	unchanged.EffectiveOrder = 4
	unchanged.InstrumentNumber = "SUPP-3-" + suffix
	unchanged.Title = "第三份生效补充协议"
	unchanged.ChangeSummary = "只调整非金额条款说明"
	unchanged.AmountEffect = contract.AmountEffectUnchanged
	unchanged.AmountValue = ""
	unchangedResult, err := service.AppendChange(ctx, unchanged)
	if err != nil || unchangedResult.Revision != 4 || unchangedResult.EffectiveAmount != "15000.125" {
		t.Fatalf("AppendChange(unchanged) = %#v, error=%v", unchangedResult, err)
	}

	changedDetails, err := service.GetDraft(ctx, primary.ID)
	if err != nil {
		t.Fatalf("GetDraft() after supplements error = %v", err)
	}
	if changedDetails.Contract.CurrentRevision != 4 || changedDetails.Contract.TotalAmount != "15000.125" ||
		changedDetails.Revision == nil || changedDetails.Revision.Revision != 4 || changedDetails.Revision.TotalAmount != "15000.125" {
		t.Fatalf("current changed contract=%#v revision=%#v", changedDetails.Contract, changedDetails.Revision)
	}
	if len(changedDetails.Instruments) != 4 || len(changedDetails.Revisions) != 4 || len(changedDetails.PartySnapshots) != 3 {
		t.Fatalf("changed timeline instruments=%d revisions=%d parties=%d", len(changedDetails.Instruments), len(changedDetails.Revisions), len(changedDetails.PartySnapshots))
	}
	wantAmounts := []string{"12345.67", "12000", "15000.125", "15000.125"}
	for index, revision := range changedDetails.Revisions {
		if revision.Revision != int64(index+1) || revision.TotalAmount != wantAmounts[index] {
			t.Fatalf("revision history[%d] = %#v", index, revision)
		}
	}
	for _, snapshot := range changedDetails.PartySnapshots {
		if snapshot.Revision != 4 {
			t.Fatalf("current snapshot revision = %#v", snapshot)
		}
	}
	listedContracts, err := service.List(ctx)
	if err != nil {
		t.Fatalf("List() after supplements error = %v", err)
	}
	listedCurrentAmount := ""
	for _, listed := range listedContracts {
		if listed.ID == primary.ID {
			listedCurrentAmount = listed.TotalAmount
		}
	}
	if listedCurrentAmount != "15000.125" {
		t.Fatalf("listed current amount = %q", listedCurrentAmount)
	}
	activeSearch, err := service.Search(ctx, contract.SearchContractsInput{
		Keyword: primary.ContractNumber, Status: "active", Currency: "cny",
	})
	if err != nil {
		t.Fatalf("Search(active contract) error = %v", err)
	}
	if len(activeSearch.Contracts) != 1 || activeSearch.Contracts[0].ID != primary.ID ||
		activeSearch.Contracts[0].TotalAmount != "15000.125" || activeSearch.Contracts[0].CurrentRevision != 4 {
		t.Fatalf("active search page = %#v", activeSearch)
	}
	var changeAuditCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM audit_entries
		WHERE object_id = $1 AND action = 'contract_change_activated'
	`, primary.ID).Scan(&changeAuditCount); err != nil {
		t.Fatalf("count contract change audits: %v", err)
	}
	if changeAuditCount != 3 {
		t.Fatalf("change audit count = %d, want 3", changeAuditCount)
	}

	postActivationUpdateID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	_, err = service.UpdateDraft(ctx, contract.UpdateDraftInput{
		Actor:     actor,
		CommandID: postActivationUpdateID, ContractID: primary.ID, ExpectedDraftVersion: 4,
		NumberNamespace: primary.NumberNamespace, ContractNumber: primary.ContractNumber, Title: "激活后编辑",
		Currency: primary.Currency, TotalAmount: primary.TotalAmount, SignedOn: primary.SignedOn, EffectiveOn: primary.EffectiveOn,
	})
	if !errors.As(err, &stateConflict) {
		t.Fatalf("post-activation update error = %v, want ContractStateConflictError", err)
	}
	postActivationCancelID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	_, err = service.CancelDraft(ctx, contract.CancelDraftInput{
		Actor:     actor,
		CommandID: postActivationCancelID, ContractID: primary.ID, ExpectedDraftVersion: 4, Reason: "不允许",
	})
	if !errors.As(err, &stateConflict) {
		t.Fatalf("post-activation cancel error = %v, want ContractStateConflictError", err)
	}

	assertImmutableDatabaseFact(t, pool, `UPDATE contract_revisions SET title = '篡改' WHERE contract_id = $1 AND revision = 1`, primary.ID)
	assertImmutableDatabaseFact(t, pool, `DELETE FROM contract_revision_parties WHERE contract_id = $1 AND revision = 4`, primary.ID)
	assertImmutableDatabaseFact(t, pool, `UPDATE contract_instruments SET title = '篡改' WHERE contract_id = $1`, primary.ID)
	assertImmutableDatabaseFact(t, pool, `DELETE FROM contract_change_commands WHERE contract_id = $1`, primary.ID)
	assertImmutableDatabaseFact(t, pool, `UPDATE contract_audit_entries SET actor_id = 'tampered' WHERE contract_id = $1`, primary.ID)
	assertImmutableDatabaseFact(t, pool, `DELETE FROM contract_activation_commands WHERE contract_id = $1`, primary.ID)

	var existingAuditID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM contract_audit_entries WHERE contract_id = $1`, primary.ID).Scan(&existingAuditID); err != nil {
		t.Fatalf("read existing audit ID: %v", err)
	}
	repository := contract.NewPostgresRepository(pool)
	rollbackChangeCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	rollbackInstrumentID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	_, _, err = repository.AppendChange(ctx, contract.ContractChangeCommand{
		CommandID: rollbackChangeCommandID, AuditID: existingAuditID, InstrumentID: rollbackInstrumentID,
		ContractID: primary.ID, ExpectedRevision: 4,
		InstrumentNumber: "ROLLBACK-" + suffix, Title: "审计失败回滚补充协议", ChangeSummary: "验证原子回滚",
		SignedOn: "2026-10-04", EffectiveOn: "2026-10-04", EffectiveOrder: 5,
		AmountEffect: contract.AmountEffectUnchanged, Actor: actor, ResultingRevision: 5, OccurredAt: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("AppendChange() with duplicate audit ID error = nil, want transaction failure")
	}
	var rollbackCurrentRevision int64
	var rollbackFactCount int
	if err := pool.QueryRow(ctx, `SELECT current_revision FROM contracts WHERE id = $1`, primary.ID).Scan(&rollbackCurrentRevision); err != nil {
		t.Fatalf("read contract after failed change: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM contract_instruments WHERE id = $2) +
			(SELECT count(*) FROM contract_change_commands WHERE command_id = $3) +
			(SELECT count(*) FROM contract_revisions WHERE contract_id = $1 AND revision = 5) +
			(SELECT count(*) FROM contract_revision_parties WHERE contract_id = $1 AND revision = 5)
	`, primary.ID, rollbackInstrumentID, rollbackChangeCommandID).Scan(&rollbackFactCount); err != nil {
		t.Fatalf("count failed contract change facts: %v", err)
	}
	if rollbackCurrentRevision != 4 || rollbackFactCount != 0 {
		t.Fatalf("failed change left current revision=%d facts=%d", rollbackCurrentRevision, rollbackFactCount)
	}

	concurrentInputs := make([]contract.AppendContractChangeInput, 2)
	for index := range concurrentInputs {
		commandID, err := contract.NewCommandID()
		if err != nil {
			t.Fatalf("NewCommandID() error = %v", err)
		}
		concurrentInputs[index] = contract.AppendContractChangeInput{
			CommandID: commandID, ContractID: primary.ID, ExpectedRevision: 4,
			InstrumentNumber: "CONCURRENT-" + string(rune('A'+index)) + "-" + suffix,
			Title:            "并发补充协议", ChangeSummary: "验证同一前置 revision 只能成功一次",
			SignedOn: "2026-10-04", EffectiveOn: "2026-10-04", EffectiveOrder: 5,
			AmountEffect: contract.AmountEffectUnchanged, Actor: actor,
		}
	}
	type concurrentChangeOutcome struct {
		result contract.ContractChangeResult
		err    error
	}
	startConcurrentChanges := make(chan struct{})
	concurrentOutcomes := make(chan concurrentChangeOutcome, len(concurrentInputs))
	for _, input := range concurrentInputs {
		input := input
		go func() {
			<-startConcurrentChanges
			result, err := service.AppendChange(ctx, input)
			concurrentOutcomes <- concurrentChangeOutcome{result: result, err: err}
		}()
	}
	close(startConcurrentChanges)
	concurrentSuccesses, concurrentConflicts := 0, 0
	for range concurrentInputs {
		outcome := <-concurrentOutcomes
		if outcome.err == nil {
			concurrentSuccesses++
			if outcome.result.Revision != 5 || outcome.result.EffectiveAmount != "15000.125" {
				t.Fatalf("concurrent successful change = %#v", outcome.result)
			}
			continue
		}
		var conflict *contract.ContractRevisionConflictError
		if !errors.As(outcome.err, &conflict) || conflict.Expected != 4 || conflict.Actual != 5 {
			t.Fatalf("concurrent losing change error = %v, want revision 4 -> 5 conflict", outcome.err)
		}
		concurrentConflicts++
	}
	if concurrentSuccesses != 1 || concurrentConflicts != 1 {
		t.Fatalf("concurrent changes successes=%d conflicts=%d", concurrentSuccesses, concurrentConflicts)
	}
	var concurrentCurrentRevision, concurrentInstrumentCount, concurrentRevisionCount, concurrentChangeAuditCount int
	if err := pool.QueryRow(ctx, `
		SELECT c.current_revision,
			(SELECT count(*) FROM contract_instruments WHERE contract_id = c.id),
			(SELECT count(*) FROM contract_revisions WHERE contract_id = c.id),
			(SELECT count(*) FROM audit_entries WHERE object_id = c.id AND action = 'contract_change_activated')
		FROM contracts c
		WHERE c.id = $1
	`, primary.ID).Scan(&concurrentCurrentRevision, &concurrentInstrumentCount, &concurrentRevisionCount, &concurrentChangeAuditCount); err != nil {
		t.Fatalf("read concurrent contract change facts: %v", err)
	}
	if concurrentCurrentRevision != 5 || concurrentInstrumentCount != 5 || concurrentRevisionCount != 5 || concurrentChangeAuditCount != 4 {
		t.Fatalf("concurrent facts revision=%d instruments=%d revisions=%d audits=%d", concurrentCurrentRevision, concurrentInstrumentCount, concurrentRevisionCount, concurrentChangeAuditCount)
	}

	backdatedTerminationCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	_, err = service.Terminate(ctx, contract.TerminateContractInput{
		CommandID: backdatedTerminationCommandID, ContractID: primary.ID, ExpectedRevision: 5,
		InstrumentNumber: "TERM-BACKDATED-" + suffix, Title: "倒退日期终止文书", TerminationReason: "验证日期顺序",
		SignedOn: "2026-10-05", EffectiveOn: "2026-10-03", EffectiveOrder: 6, Actor: actor,
	})
	var terminationPrecondition *contract.ContractTerminationPreconditionError
	if !errors.As(err, &terminationPrecondition) || terminationPrecondition.Field != "instrument_effective_on" {
		t.Fatalf("backdated termination error = %v", err)
	}

	duplicateTerminationCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	_, err = service.Terminate(ctx, contract.TerminateContractInput{
		CommandID: duplicateTerminationCommandID, ContractID: primary.ID, ExpectedRevision: 5,
		InstrumentNumber: strings.ToLower(changeInput.InstrumentNumber), Title: "重复编号终止文书", TerminationReason: "验证编号唯一",
		SignedOn: "2026-10-05", EffectiveOn: "2026-10-05", EffectiveOrder: 6, Actor: actor,
	})
	if !errors.As(err, &duplicateInstrument) {
		t.Fatalf("duplicate termination instrument number error = %v", err)
	}

	terminationCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	terminationInput := contract.TerminateContractInput{
		CommandID: terminationCommandID, ContractID: primary.ID, ExpectedRevision: 5,
		InstrumentNumber: "TERM-1-" + suffix, Title: "生效终止协议", TerminationReason: "双方确认合同自本日终止",
		SignedOn: "2026-10-05", EffectiveOn: "2026-10-05", EffectiveOrder: 6, Actor: actor,
	}
	terminated, err := service.Terminate(ctx, terminationInput)
	if err != nil || terminated.Revision != 6 || terminated.EffectiveAmount != "15000.125" || terminated.Status != contract.StatusTerminated || terminated.Replayed {
		t.Fatalf("Terminate() = %#v, error=%v", terminated, err)
	}
	replayedTermination, err := service.Terminate(ctx, terminationInput)
	if err != nil || !replayedTermination.Replayed || replayedTermination.Revision != 6 || replayedTermination.InstrumentID != terminated.InstrumentID {
		t.Fatalf("replayed Terminate() = %#v, error=%v", replayedTermination, err)
	}
	changedTermination := terminationInput
	changedTermination.TerminationReason = "篡改终止原因"
	_, err = service.Terminate(ctx, changedTermination)
	if !errors.As(err, &idempotencyConflict) {
		t.Fatalf("changed termination replay error = %v, want IdempotencyConflictError", err)
	}
	terminatedDetails, err := service.GetDraft(ctx, primary.ID)
	if err != nil {
		t.Fatalf("GetDraft(terminated) error = %v", err)
	}
	if terminatedDetails.Contract.Status != contract.StatusTerminated || terminatedDetails.Contract.CurrentRevision != 6 ||
		terminatedDetails.Contract.TotalAmount != "15000.125" || terminatedDetails.Revision == nil ||
		terminatedDetails.Revision.Revision != 6 || terminatedDetails.Revision.TotalAmount != "15000.125" {
		t.Fatalf("terminated contract=%#v revision=%#v", terminatedDetails.Contract, terminatedDetails.Revision)
	}
	if len(terminatedDetails.Instruments) != 6 || len(terminatedDetails.Revisions) != 6 || len(terminatedDetails.PartySnapshots) != 3 ||
		terminatedDetails.Instruments[5].InstrumentType != contract.InstrumentTypeTermination {
		t.Fatalf("terminated timeline instruments=%#v revisions=%d parties=%d", terminatedDetails.Instruments, len(terminatedDetails.Revisions), len(terminatedDetails.PartySnapshots))
	}
	terminatedSearch, err := service.Search(ctx, contract.SearchContractsInput{
		Keyword: primary.ContractNumber, Status: "terminated", Currency: "cny",
	})
	if err != nil {
		t.Fatalf("Search(terminated contract) error = %v", err)
	}
	if len(terminatedSearch.Contracts) != 1 || terminatedSearch.Contracts[0].ID != primary.ID ||
		terminatedSearch.Contracts[0].TotalAmount != "15000.125" || terminatedSearch.Contracts[0].CurrentRevision != 6 {
		t.Fatalf("terminated search page = %#v", terminatedSearch)
	}
	for _, snapshot := range terminatedDetails.PartySnapshots {
		if snapshot.Revision != 6 {
			t.Fatalf("termination snapshot = %#v", snapshot)
		}
	}
	var terminationAuditCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM audit_entries
		WHERE object_id = $1 AND action = 'contract_terminated'
	`, primary.ID).Scan(&terminationAuditCount); err != nil {
		t.Fatalf("count termination audits: %v", err)
	}
	if terminationAuditCount != 1 {
		t.Fatalf("termination audit count = %d, want 1", terminationAuditCount)
	}
	postTerminationChangeCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	postTerminationChange := unchanged
	postTerminationChange.CommandID = postTerminationChangeCommandID
	postTerminationChange.ExpectedRevision = 6
	postTerminationChange.EffectiveOrder = 7
	postTerminationChange.InstrumentNumber = "POST-TERM-SUPP-" + suffix
	postTerminationChange.EffectiveOn = "2026-10-06"
	_, err = service.AppendChange(ctx, postTerminationChange)
	var changeStateConflict *contract.ContractChangeStateConflictError
	if !errors.As(err, &changeStateConflict) || changeStateConflict.Status != contract.StatusTerminated {
		t.Fatalf("post-termination supplement error = %v", err)
	}
	secondTerminationCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	secondTermination := terminationInput
	secondTermination.CommandID = secondTerminationCommandID
	secondTermination.ExpectedRevision = 6
	secondTermination.EffectiveOrder = 7
	secondTermination.InstrumentNumber = "TERM-2-" + suffix
	secondTermination.EffectiveOn = "2026-10-06"
	_, err = service.Terminate(ctx, secondTermination)
	terminationStateConflict = nil
	if !errors.As(err, &terminationStateConflict) || terminationStateConflict.Status != contract.StatusTerminated {
		t.Fatalf("second termination error = %v", err)
	}
	assertImmutableDatabaseFact(t, pool, `UPDATE contract_termination_commands SET title = '篡改' WHERE contract_id = $1`, primary.ID)

	terminationRace := activateTwoParty("TERMINATION-RACE", "并发终止测试")
	terminationRaceInputs := make([]contract.TerminateContractInput, 2)
	for index := range terminationRaceInputs {
		commandID, err := contract.NewCommandID()
		if err != nil {
			t.Fatalf("NewCommandID() error = %v", err)
		}
		terminationRaceInputs[index] = contract.TerminateContractInput{
			CommandID: commandID, ContractID: terminationRace.ID, ExpectedRevision: 1,
			InstrumentNumber: "TERM-RACE-" + string(rune('A'+index)) + "-" + suffix,
			Title:            "并发终止文书", TerminationReason: "验证只有一个终止命令成功",
			SignedOn: "2026-10-02", EffectiveOn: "2026-10-02", EffectiveOrder: 2, Actor: actor,
		}
	}
	type concurrentTerminationOutcome struct {
		result contract.TerminationResult
		err    error
	}
	startConcurrentTerminations := make(chan struct{})
	terminationOutcomes := make(chan concurrentTerminationOutcome, len(terminationRaceInputs))
	for _, input := range terminationRaceInputs {
		input := input
		go func() {
			<-startConcurrentTerminations
			result, err := service.Terminate(ctx, input)
			terminationOutcomes <- concurrentTerminationOutcome{result: result, err: err}
		}()
	}
	close(startConcurrentTerminations)
	terminationSuccesses, terminationConflicts := 0, 0
	for range terminationRaceInputs {
		outcome := <-terminationOutcomes
		if outcome.err == nil {
			terminationSuccesses++
			if outcome.result.Revision != 2 || outcome.result.Status != contract.StatusTerminated {
				t.Fatalf("concurrent successful termination = %#v", outcome.result)
			}
			continue
		}
		var conflict *contract.ContractTerminationStateConflictError
		if !errors.As(outcome.err, &conflict) || conflict.Status != contract.StatusTerminated {
			t.Fatalf("concurrent losing termination error = %v", outcome.err)
		}
		terminationConflicts++
	}
	if terminationSuccesses != 1 || terminationConflicts != 1 {
		t.Fatalf("concurrent terminations successes=%d conflicts=%d", terminationSuccesses, terminationConflicts)
	}

	crossActionRace := activateTwoParty("CHANGE-TERMINATION-RACE", "补充协议与终止并发测试")
	crossChangeCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	crossTerminationCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	crossStart := make(chan struct{})
	type crossActionOutcome struct {
		action string
		err    error
	}
	crossOutcomes := make(chan crossActionOutcome, 2)
	go func() {
		<-crossStart
		_, err := service.AppendChange(ctx, contract.AppendContractChangeInput{
			CommandID: crossChangeCommandID, ContractID: crossActionRace.ID, ExpectedRevision: 1,
			InstrumentNumber: "CROSS-SUPP-" + suffix, Title: "并发补充协议", ChangeSummary: "与终止竞争同一 revision",
			SignedOn: "2026-10-02", EffectiveOn: "2026-10-02", EffectiveOrder: 2,
			AmountEffect: contract.AmountEffectUnchanged, Actor: actor,
		})
		crossOutcomes <- crossActionOutcome{action: "change", err: err}
	}()
	go func() {
		<-crossStart
		_, err := service.Terminate(ctx, contract.TerminateContractInput{
			CommandID: crossTerminationCommandID, ContractID: crossActionRace.ID, ExpectedRevision: 1,
			InstrumentNumber: "CROSS-TERM-" + suffix, Title: "并发终止文书", TerminationReason: "与补充协议竞争同一 revision",
			SignedOn: "2026-10-02", EffectiveOn: "2026-10-02", EffectiveOrder: 2, Actor: actor,
		})
		crossOutcomes <- crossActionOutcome{action: "termination", err: err}
	}()
	close(crossStart)
	crossSuccesses, crossConflicts := 0, 0
	for range 2 {
		outcome := <-crossOutcomes
		if outcome.err == nil {
			crossSuccesses++
			continue
		}
		switch outcome.action {
		case "change":
			var conflict *contract.ContractChangeStateConflictError
			if !errors.As(outcome.err, &conflict) || conflict.Status != contract.StatusTerminated {
				t.Fatalf("cross-action change error = %v", outcome.err)
			}
		case "termination":
			var conflict *contract.ContractRevisionConflictError
			if !errors.As(outcome.err, &conflict) || conflict.Actual != 2 {
				t.Fatalf("cross-action termination error = %v", outcome.err)
			}
		}
		crossConflicts++
	}
	if crossSuccesses != 1 || crossConflicts != 1 {
		t.Fatalf("cross-action race successes=%d conflicts=%d", crossSuccesses, crossConflicts)
	}
	var crossStatus contract.Status
	var crossRevision, crossInstrumentCount, crossCommandCount int
	if err := pool.QueryRow(ctx, `
		SELECT c.status, c.current_revision,
			(SELECT count(*) FROM contract_instruments WHERE contract_id = c.id),
			(SELECT count(*) FROM contract_change_commands WHERE contract_id = c.id) +
			(SELECT count(*) FROM contract_termination_commands WHERE contract_id = c.id)
		FROM contracts c WHERE c.id = $1
	`, crossActionRace.ID).Scan(&crossStatus, &crossRevision, &crossInstrumentCount, &crossCommandCount); err != nil {
		t.Fatalf("read cross-action race facts: %v", err)
	}
	if crossRevision != 2 || crossInstrumentCount != 2 || crossCommandCount != 1 ||
		(crossStatus != contract.StatusActive && crossStatus != contract.StatusTerminated) {
		t.Fatalf("cross-action facts status=%s revision=%d instruments=%d commands=%d", crossStatus, crossRevision, crossInstrumentCount, crossCommandCount)
	}

	unknownCreateCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	unknownAmount, err := service.CreateDraft(ctx, contract.CreateDraftInput{
		Actor: actor, CommandID: unknownCreateCommandID,
		NumberNamespace: "activation-integration", ContractNumber: "UNKNOWN-AMOUNT-" + suffix,
		Title: "金额待确定补充协议测试", Category: "activation-test", Currency: "CNY",
		SignedOn: "2026-09-19", EffectiveOn: "2026-10-01",
	})
	if err != nil {
		t.Fatalf("CreateDraft(unknown amount) error = %v", err)
	}
	contractIDs = append(contractIDs, unknownAmount.ID)
	draftChangeCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	_, err = service.AppendChange(ctx, contract.AppendContractChangeInput{
		CommandID: draftChangeCommandID, ContractID: unknownAmount.ID, ExpectedRevision: 1,
		InstrumentNumber: "DRAFT-SUPP-" + suffix, Title: "草稿补充协议", ChangeSummary: "草稿状态必须拒绝",
		SignedOn: "2026-10-02", EffectiveOn: "2026-10-02", EffectiveOrder: 2,
		AmountEffect: contract.AmountEffectUnchanged, Actor: actor,
	})
	var draftChangeStateConflict *contract.ContractChangeStateConflictError
	if !errors.As(err, &draftChangeStateConflict) || draftChangeStateConflict.Status != contract.StatusDraft {
		t.Fatalf("draft contract change error = %v, want ContractChangeStateConflictError", err)
	}
	addParty(unknownAmount.ID, firstParty.ID, "委托方", 1)
	addParty(unknownAmount.ID, secondParty.ID, "受托方", 2)
	unknownActivationCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	if _, err := service.Activate(ctx, contract.ActivateContractInput{
		CommandID: unknownActivationCommandID, ContractID: unknownAmount.ID, ExpectedDraftVersion: 3, Actor: actor,
	}); err != nil {
		t.Fatalf("Activate(unknown amount) error = %v", err)
	}
	unknownChangeCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	unknownChangeInput := contract.AppendContractChangeInput{
		CommandID: unknownChangeCommandID, ContractID: unknownAmount.ID, ExpectedRevision: 1,
		InstrumentNumber: "UNKNOWN-SUPP-" + suffix, Title: "金额保持待确定补充协议", ChangeSummary: "非金额条款发生变化",
		SignedOn: "2026-10-02", EffectiveOn: "2026-10-02", EffectiveOrder: 2,
		AmountEffect: contract.AmountEffectUnchanged, Actor: actor,
	}
	unknownChanged, err := service.AppendChange(ctx, unknownChangeInput)
	if err != nil || unknownChanged.Revision != 2 || unknownChanged.EffectiveAmount != "" || unknownChanged.Replayed {
		t.Fatalf("AppendChange(unknown unchanged) = %#v, error=%v", unknownChanged, err)
	}
	unknownReplayed, err := service.AppendChange(ctx, unknownChangeInput)
	if err != nil || unknownReplayed.Revision != 2 || unknownReplayed.EffectiveAmount != "" || !unknownReplayed.Replayed {
		t.Fatalf("replayed AppendChange(unknown unchanged) = %#v, error=%v", unknownReplayed, err)
	}
	unknownDetails, err := service.GetDraft(ctx, unknownAmount.ID)
	if err != nil {
		t.Fatalf("GetDraft(unknown changed) error = %v", err)
	}
	if unknownDetails.Contract.CurrentRevision != 2 || unknownDetails.Contract.TotalAmount != "" ||
		unknownDetails.Revision == nil || unknownDetails.Revision.TotalAmount != "" {
		t.Fatalf("unknown changed details contract=%#v revision=%#v", unknownDetails.Contract, unknownDetails.Revision)
	}
	unknownTerminationCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	unknownTerminated, err := service.Terminate(ctx, contract.TerminateContractInput{
		CommandID: unknownTerminationCommandID, ContractID: unknownAmount.ID, ExpectedRevision: 2,
		InstrumentNumber: "UNKNOWN-TERM-" + suffix, Title: "金额待确定终止文书", TerminationReason: "验证终止不生成结算金额",
		SignedOn: "2026-10-03", EffectiveOn: "2026-10-03", EffectiveOrder: 3, Actor: actor,
	})
	if err != nil || unknownTerminated.Revision != 3 || unknownTerminated.EffectiveAmount != "" || unknownTerminated.Status != contract.StatusTerminated {
		t.Fatalf("Terminate(unknown amount) = %#v, error=%v", unknownTerminated, err)
	}
	unknownTerminatedDetails, err := service.GetDraft(ctx, unknownAmount.ID)
	if err != nil {
		t.Fatalf("GetDraft(unknown terminated) error = %v", err)
	}
	if unknownTerminatedDetails.Contract.Status != contract.StatusTerminated || unknownTerminatedDetails.Contract.CurrentRevision != 3 ||
		unknownTerminatedDetails.Contract.TotalAmount != "" || unknownTerminatedDetails.Revision == nil ||
		unknownTerminatedDetails.Revision.TotalAmount != "" {
		t.Fatalf("unknown terminated details contract=%#v revision=%#v", unknownTerminatedDetails.Contract, unknownTerminatedDetails.Revision)
	}

	terminationFailure := activateTwoParty("TERMINATION-ATOMIC-FAILURE", "终止原子回滚测试")
	terminationFailureCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	terminationFailureInstrumentID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	_, _, err = repository.Terminate(ctx, contract.TerminationCommand{
		CommandID: terminationFailureCommandID, AuditID: existingAuditID, InstrumentID: terminationFailureInstrumentID,
		ContractID: terminationFailure.ID, ExpectedRevision: 1,
		InstrumentNumber: "TERM-ROLLBACK-" + suffix, Title: "审计失败回滚终止文书", TerminationReason: "验证原子回滚",
		SignedOn: "2026-10-02", EffectiveOn: "2026-10-02", EffectiveOrder: 2,
		Actor: actor, ResultingRevision: 2, OccurredAt: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("Terminate() with duplicate audit ID error = nil, want transaction failure")
	}
	var terminationFailureStatus contract.Status
	var terminationFailureRevision int64
	var terminationFailureFactCount int
	if err := pool.QueryRow(ctx, `SELECT status, current_revision FROM contracts WHERE id = $1`, terminationFailure.ID).Scan(&terminationFailureStatus, &terminationFailureRevision); err != nil {
		t.Fatalf("read contract after failed termination: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM contract_instruments WHERE id = $2) +
			(SELECT count(*) FROM contract_termination_commands WHERE command_id = $3) +
			(SELECT count(*) FROM contract_revisions WHERE contract_id = $1 AND revision = 2) +
			(SELECT count(*) FROM contract_revision_parties WHERE contract_id = $1 AND revision = 2) +
			(SELECT count(*) FROM audit_entries WHERE command_id = $3)
	`, terminationFailure.ID, terminationFailureInstrumentID, terminationFailureCommandID).Scan(&terminationFailureFactCount); err != nil {
		t.Fatalf("count failed termination facts: %v", err)
	}
	if terminationFailureStatus != contract.StatusActive || terminationFailureRevision != 1 || terminationFailureFactCount != 0 {
		t.Fatalf("failed termination left status=%s revision=%d facts=%d", terminationFailureStatus, terminationFailureRevision, terminationFailureFactCount)
	}

	failing := createDraft("ATOMIC-FAILURE", "原子回滚激活测试", "2026-09-19", "2026-10-01")
	addParty(failing.ID, firstParty.ID, "委托方", 1)
	addParty(failing.ID, secondParty.ID, "受托方", 2)
	failingCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	failingInstrumentID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	_, _, err = repository.Activate(ctx, contract.ActivationCommand{
		CommandID: failingCommandID, AuditID: existingAuditID, InstrumentID: failingInstrumentID, ContractID: failing.ID,
		ExpectedDraftVersion: 3, Actor: actor, ResultingRevision: 1, OccurredAt: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("Activate() with duplicate audit ID error = nil, want transaction failure")
	}
	var failureStatus contract.Status
	var failureRevision *int64
	var factCount int
	if err := pool.QueryRow(ctx, `SELECT status, current_revision FROM contracts WHERE id = $1`, failing.ID).Scan(&failureStatus, &failureRevision); err != nil {
		t.Fatalf("read failed activation contract: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM contract_activation_commands WHERE contract_id = $1) +
			(SELECT count(*) FROM contract_instruments WHERE contract_id = $1) +
			(SELECT count(*) FROM contract_revisions WHERE contract_id = $1) +
			(SELECT count(*) FROM contract_revision_parties WHERE contract_id = $1) +
			(SELECT count(*) FROM contract_audit_entries WHERE contract_id = $1)
	`, failing.ID).Scan(&factCount); err != nil {
		t.Fatalf("count failed activation facts: %v", err)
	}
	if failureStatus != contract.StatusDraft || failureRevision != nil || factCount != 0 {
		t.Fatalf("failed activation left state status=%q revision=%v facts=%d", failureStatus, failureRevision, factCount)
	}
}

func assertImmutableDatabaseFact(t *testing.T, pool *pgxpool.Pool, statement string, contractID string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), statement, contractID)
	var databaseError *pgconn.PgError
	if !errors.As(err, &databaseError) || databaseError.Code != "55000" {
		t.Fatalf("immutable statement error = %v, want PostgreSQL 55000", err)
	}
}

func cleanupActivationFixtures(t *testing.T, pool *pgxpool.Pool, contractIDs, partyIDs []string) {
	t.Helper()
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	connection, err := pool.Acquire(cleanupCtx)
	if err != nil {
		t.Errorf("acquire activation cleanup connection: %v", err)
		return
	}
	defer connection.Release()
	if _, err := connection.Exec(cleanupCtx, `SET session_replication_role = replica`); err != nil {
		t.Errorf("disable triggers for activation cleanup: %v", err)
		return
	}
	defer func() {
		if _, err := connection.Exec(context.Background(), `SET session_replication_role = origin`); err != nil {
			t.Errorf("restore triggers after activation cleanup: %v", err)
		}
	}()
	for _, contractID := range contractIDs {
		for _, statement := range []string{
			`DELETE FROM audit_entries WHERE object_id = $1`,
			`DELETE FROM contract_evidence_commands WHERE contract_id = $1`,
			`DELETE FROM contract_evidence_versions WHERE document_id IN (SELECT id FROM contract_evidence_documents WHERE contract_id = $1)`,
			`DELETE FROM contract_evidence_documents WHERE contract_id = $1`,
			`DELETE FROM contract_correction_commands WHERE contract_id = $1`,
			`DELETE FROM contract_closure_commands WHERE contract_id = $1`,
			`DELETE FROM contract_closures WHERE contract_id = $1`,
			`DELETE FROM contract_settlement_parties WHERE settlement_id IN (SELECT id FROM contract_settlements WHERE contract_id = $1)`,
			`DELETE FROM contract_settlement_commands WHERE contract_id = $1`,
			`DELETE FROM contract_settlements WHERE contract_id = $1`,
			`DELETE FROM contract_relation_commands WHERE source_contract_id = $1 OR target_contract_id = $1`,
			`DELETE FROM contract_relations WHERE source_contract_id = $1 OR target_contract_id = $1`,
			`DELETE FROM contract_audit_entries WHERE contract_id = $1`,
			`DELETE FROM contract_termination_commands WHERE contract_id = $1`,
			`DELETE FROM contract_change_commands WHERE contract_id = $1`,
			`DELETE FROM contract_revision_parties WHERE contract_id = $1`,
			`DELETE FROM contract_revisions WHERE contract_id = $1`,
			`DELETE FROM contract_instruments WHERE contract_id = $1`,
			`DELETE FROM contract_activation_commands WHERE contract_id = $1`,
			`DELETE FROM contract_draft_party_commands WHERE contract_id = $1`,
			`DELETE FROM contract_draft_parties WHERE contract_id = $1`,
			`DELETE FROM contract_draft_change_commands WHERE contract_id = $1`,
			`DELETE FROM contracts WHERE id = $1`,
		} {
			if _, err := connection.Exec(cleanupCtx, statement, contractID); err != nil {
				t.Errorf("activation cleanup contract %q: %v", contractID, err)
			}
		}
	}
	for _, partyID := range partyIDs {
		if _, err := connection.Exec(cleanupCtx, `DELETE FROM audit_entries WHERE object_id = $1`, partyID); err != nil {
			t.Errorf("activation cleanup party audit %q: %v", partyID, err)
		}
		if _, err := connection.Exec(cleanupCtx, `DELETE FROM parties WHERE id = $1`, partyID); err != nil {
			t.Errorf("activation cleanup party %q: %v", partyID, err)
		}
	}
}

func cleanupAuditEntries(t *testing.T, pool *pgxpool.Pool, objectIDs []string) {
	t.Helper()
	if len(objectIDs) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, err := pool.Acquire(ctx)
	if err != nil {
		t.Errorf("acquire audit cleanup connection: %v", err)
		return
	}
	defer connection.Release()
	if _, err := connection.Exec(ctx, `SET session_replication_role = replica`); err != nil {
		t.Errorf("disable audit cleanup trigger: %v", err)
		return
	}
	defer func() {
		if _, err := connection.Exec(context.Background(), `SET session_replication_role = origin`); err != nil {
			t.Errorf("restore audit cleanup trigger: %v", err)
		}
	}()
	for _, objectID := range objectIDs {
		if _, err := connection.Exec(ctx, `DELETE FROM audit_entries WHERE object_id = $1`, objectID); err != nil {
			t.Errorf("clean up audit entries for %q: %v", objectID, err)
		}
	}
}
