//go:build integration

package projects

import (
	"context"
	"fmt"
	"testing"

	"omsbizfoundry/project-workspace/internal/database"
	"omsbizfoundry/project-workspace/internal/testdatabase"
)

func TestPostgresDirectoryPaginationHierarchyScopeAndReadOnly(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(environment.Pool)
	actor := Actor{ID: intActorA, TenantID: intTenantA}
	sequence := 2000
	active := func(actor Actor, node, name, number string) Project {
		t.Helper()
		sequence += 4
		created, err := store.DispatchInitiation(ctx, actor, InitiationDispatchInput{CommandID: command(sequence), NodeType: node, Name: name, BusinessOwner: InitiationOwnerReference{PersonID: intPersonA, SourceRevision: 1}, ownerSnapshot: "经营快照"})
		if err != nil {
			t.Fatal(err)
		}
		updated, err := store.Update(ctx, actor, created.ID, UpdateInput{CreateInput: CreateInput{CommandID: command(sequence + 1), Name: name, ProjectNumber: number, ProjectTypeKeys: []string{"design"}}, ExpectedRevision: created.Revision})
		if err != nil {
			t.Fatal(err)
		}
		value, err := store.CompleteInitiation(ctx, actor, created.ID, RevisionCommand{CommandID: command(sequence + 2), ExpectedRevision: updated.Revision, InitiationProfile: InitiationProfileBusinessOwnerV1})
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	for index := 0; index < 105; index++ {
		active(actor, NodeTypeProject, fmt.Sprintf("分页%d", index), fmt.Sprintf("HNJH2026-%03d", index+1))
	}
	parent := active(actor, NodeTypePackage, "北环项目包", "")
	child := active(actor, NodeTypeProject, "独特子项%_", "HNJH2025-009J01")
	attached, err := store.ConvertPackage(ctx, actor, child.ID, PackageConversionCommand{CommandID: command(2900), Operation: ConversionAttachExisting, ExpectedProjectRevision: child.Revision, TargetPackageID: parent.ID, ExpectedPackageRevision: parent.Revision, SortOrder: 2})
	if err != nil {
		t.Fatal(err)
	}
	partyRevision := int64(7)
	linked, err := store.ApplyResourceLink(ctx, actor, child.ID, ResourceLinkCommand{CommandID: command(2902), ExpectedRevision: attached.Project.Revision, Operation: "link", TargetService: "party-directory", ResourceType: "party", ResourceID: intContract, SourceRevision: &partyRevision, RelationKey: "contractor-party"})
	if err != nil {
		t.Fatal(err)
	}
	active(Actor{ID: intActorA, TenantID: intTenantB}, NodeTypeProject, "另租户秘密", "HNJH2026-999")
	if _, err := store.DispatchInitiation(ctx, actor, InitiationDispatchInput{CommandID: command(2901), NodeType: NodeTypeProject, Name: "未完成隐藏", BusinessOwner: InitiationOwnerReference{PersonID: intPersonA, SourceRevision: 1}, ownerSnapshot: "经营快照"}); err != nil {
		t.Fatal(err)
	}
	draft, err := store.Create(ctx, actor, CreateInput{CommandID: command(2903), ProjectNumber: "OMS-DRAFT-001", Name: "OMS 新建项目", ProjectTypeKeys: []string{"operations"}})
	if err != nil {
		t.Fatal(err)
	}
	counts := func() string {
		t.Helper()
		var digest string
		if err := environment.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM projects)||':'||(SELECT sum(revision) FROM projects)||':'||(SELECT count(*) FROM project_history_events)||':'||(SELECT count(*) FROM project_commands)||':'||(SELECT count(*) FROM project_assignments)||':'||(SELECT count(*) FROM project_resource_links)`).Scan(&digest); err != nil {
			t.Fatal(err)
		}
		return digest
	}
	before := counts()
	query := DirectoryQuery{ScopeKind: "tenant", Limit: 100}
	first, err := store.Directory(ctx, intTenantA, query)
	if err != nil || first.Total != 106 || len(first.Items) != 100 || !first.HasMore {
		t.Fatalf("first %#v error=%v", first, err)
	}
	query.Offset = 100
	second, err := store.Directory(ctx, intTenantA, query)
	if err != nil || second.Total != 106 || len(second.Items) != 6 || second.HasMore {
		t.Fatalf("second %#v error=%v", second, err)
	}
	all, err := store.Directory(ctx, intTenantA, DirectoryQuery{ScopeKind: "tenant", Status: "all", Keyword: "OMS 新建项目"})
	if err != nil || all.Total != 1 || len(all.Items) != 1 || all.Items[0].ID != draft.ID || all.Items[0].Status != StatusDraft {
		t.Fatalf("all-status directory did not expose newly created draft: %#v error=%v", all, err)
	}
	seen := map[string]bool{}
	for _, entry := range append(first.Items, second.Items...) {
		if seen[entry.ID] || entry.Status != StatusActive || entry.TenantID != intTenantA || len(entry.Assignments) != 1 || entry.Assignments[0].DisplaySnapshot != "经营快照" {
			t.Fatalf("bad/duplicate entry %#v", entry)
		}
		seen[entry.ID] = true
	}
	for _, keyword := range []string{"独特", "%_", "HNJH2025-009J01", "北环"} {
		page, err := store.Directory(ctx, intTenantA, DirectoryQuery{ScopeKind: "tenant", Keyword: keyword})
		if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != parent.ID || len(page.Items[0].Children) != 1 || page.Items[0].Children[0].ID != child.ID || len(page.Items[0].Children[0].ResourceLinks) != 1 || page.Items[0].Children[0].ResourceLinks[0].RelationKey != "contractor-party" {
			t.Fatalf("keyword %s %#v error=%v", keyword, page, err)
		}
	}
	for _, fixture := range []struct {
		ids      []string
		root     string
		children int
	}{
		{[]string{parent.ID}, parent.ID, 0}, {[]string{child.ID}, child.ID, 0}, {[]string{parent.ID, child.ID}, parent.ID, 1},
	} {
		page, err := store.Directory(ctx, intTenantA, DirectoryQuery{ScopeKind: "resources", ResourceIDs: fixture.ids})
		if err != nil || page.Total != 1 || page.Items[0].ID != fixture.root || len(page.Items[0].Children) != fixture.children {
			t.Fatalf("scope %#v error=%v", page, err)
		}
	}
	empty, err := store.Directory(ctx, intTenantA, DirectoryQuery{ScopeKind: "resources"})
	if err != nil || empty.Total != 0 || len(empty.Items) != 0 {
		t.Fatalf("empty scope %#v error=%v", empty, err)
	}
	foreign, err := store.Directory(ctx, intTenantB, DirectoryQuery{ScopeKind: "resources", ResourceIDs: []string{parent.ID, child.ID}})
	if err != nil || foreign.Total != 0 {
		t.Fatalf("foreign scope %#v error=%v", foreign, err)
	}
	if after := counts(); after != before {
		t.Fatalf("GET changed facts %s -> %s", before, after)
	}
	details, err := store.Get(ctx, intTenantA, child.ID, true)
	if err != nil || details.Project.Revision != linked.Project.Revision {
		t.Fatalf("revision changed %#v %v", details, err)
	}
}
