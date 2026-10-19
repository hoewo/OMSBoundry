//go:build integration

package projects_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"omsbizfoundry/project-workspace/internal/database"
	"omsbizfoundry/project-workspace/internal/projects"
	"omsbizfoundry/project-workspace/internal/testdatabase"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

type contextRestoreReferences struct{}

func (contextRestoreReferences) ValidatePeople(_ context.Context, _, _, id string, revision int64) (projects.ReferenceSnapshot, error) {
	return projects.ReferenceSnapshot{DisplayName: "经营负责人"}, nil
}
func (contextRestoreReferences) ValidateParty(_ context.Context, _, id string, revision int64) (projects.PartySnapshot, error) {
	return projects.PartySnapshot{PartyID: id, SourceRevision: revision, Kind: "organization", LegalName: "创建时的已验证主体"}, nil
}
func (contextRestoreReferences) ValidateResource(context.Context, string, string, string, string, *int64) error {
	return projects.ErrReferenceUnavailable
}

func TestCounterpartyContextBackupRestorePreservesVersionsIdentityAndReplay(t *testing.T) {
	ctx := context.Background()
	env := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, env.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := projects.NewPostgresStore(env.Pool)
	service, _ := projects.NewService(store, contextRestoreReferences{})
	actor := projects.Actor{ID: materialActor, TenantID: materialTenant}
	cmd := func(n byte) string { return "a0000000-0000-4000-8000-00000000040" + string(n) }
	p, err := service.Create(ctx, actor, projects.CreateInput{CommandID: cmd('1'), Name: "恢复演练项目", ProjectNumber: "CONTEXT-RESTORE-2026", ProjectTypeKeys: []string{"design"}, ClientParty: &projects.PartyReferenceInput{PartyID: materialFile, SourceRevision: 1}})
	if err != nil {
		t.Fatal(err)
	}
	a, err := service.ApplyAssignment(ctx, actor, p.ID, projects.AssignmentCommand{CommandID: cmd('2'), ExpectedRevision: p.Revision, Operation: "assign", SubjectKind: "person", SubjectID: materialActor, SourceRevision: 1, RoleKey: projects.RoleBusinessOwner})
	if err != nil {
		t.Fatal(err)
	}
	p, err = store.CompleteInitiation(ctx, actor, p.ID, projects.RevisionCommand{CommandID: cmd('3'), ExpectedRevision: a.Project.Revision, InitiationProfile: projects.InitiationProfileBusinessOwnerV1})
	if err != nil {
		t.Fatal(err)
	}
	in := projects.CounterpartyContextInput{CommandID: cmd('4'), ExpectedRevision: p.Revision, Reason: "项目内联系资料", CounterpartyContact: projects.CounterpartyContact{Name: "与注册主体不同的项目联系名称", ContactName: "旧联系人", ContactPhone: "123-456"}}
	first, err := service.ChangeCounterpartyContext(ctx, actor, p.ID, in)
	if err != nil {
		t.Fatal(err)
	}
	next := in
	next.CommandID = cmd('5')
	next.ExpectedRevision = first.ProjectRevision
	next.ContactName = "新联系人"
	if _, err := service.ChangeCounterpartyContext(ctx, actor, p.ID, next); err != nil {
		t.Fatal(err)
	}
	current, err := store.CounterpartyContext(ctx, actor.TenantID, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	details, _ := store.Get(ctx, actor.TenantID, p.ID, true)
	history, _ := store.History(ctx, actor.TenantID, p.ID, "", 100)
	before := projectMaterialFacts(t, env.Pool)
	dump := filepath.Join(t.TempDir(), "project-context.dump")
	if output, err := exec.CommandContext(ctx, "pg_dump", "--format=custom", "--no-owner", "--no-acl", "--file="+dump, "--dbname="+env.URL).CombinedOutput(); err != nil {
		t.Fatalf("dump failed: %s", output)
	}
	if err := os.Chmod(dump, 0600); err != nil {
		t.Fatal(err)
	}
	restored := testdatabase.New(t, testdatabase.RequireURL(t))
	if output, err := exec.CommandContext(ctx, "pg_restore", "--exit-on-error", "--no-owner", "--no-acl", "--dbname="+restored.URL, dump).CombinedOutput(); err != nil {
		t.Fatalf("restore failed: %s", output)
	}
	if _, err := database.RequireCurrentSchema(ctx, restored.Pool); err != nil {
		t.Fatal(err)
	}
	if projectMaterialFacts(t, restored.Pool) != before {
		t.Fatal("restore changed owned canonical facts")
	}
	recovered, _ := projects.NewPostgresStore(restored.Pool)
	view, err := recovered.CounterpartyContext(ctx, actor.TenantID, p.ID)
	if err != nil || !reflect.DeepEqual(view, current) {
		t.Fatalf("context restore=%#v %v", view, err)
	}
	recoveredDetails, _ := recovered.Get(ctx, actor.TenantID, p.ID, true)
	recoveredHistory, _ := recovered.History(ctx, actor.TenantID, p.ID, "", 100)
	if !reflect.DeepEqual(recoveredDetails, details) || !reflect.DeepEqual(recoveredHistory, history) || recoveredDetails.Project.ClientParty.LegalName != "创建时的已验证主体" {
		t.Fatal("identity, reference snapshot or history changed")
	}
	replay, err := recovered.ChangeCounterpartyContext(ctx, actor, p.ID, in)
	if err != nil || !reflect.DeepEqual(replay, first) || projectMaterialFacts(t, restored.Pool) != before {
		t.Fatal("restored replay lost receipt or wrote again")
	}
	next.CommandID = cmd('6')
	next.ExpectedRevision = view.ProjectRevision
	next.ContactPhone = ""
	continued, err := recovered.ChangeCounterpartyContext(ctx, actor, p.ID, next)
	if err != nil || continued.ProjectRevision != view.ProjectRevision+1 || continued.Context.ContactPhone != "" {
		t.Fatalf("continued=%#v %v", continued, err)
	}
	raw, err := os.ReadFile(dump)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	t.Logf("context_canonical_before_restore=%s dump_sha256=%s restored_revision=%d continued_revision=%d identity_unchanged=true historical_contacts_preserved=true", before, hex.EncodeToString(digest[:]), view.ProjectRevision, continued.ProjectRevision)
}
