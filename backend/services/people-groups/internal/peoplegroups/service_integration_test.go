//go:build integration

package peoplegroups

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"sync"
	"testing"

	"omsbizfoundry/people-groups/internal/database"
)

const testTenant = "33333333-3333-4333-8333-333333333333"

func TestVerticalSliceWithHistorySnapshotAndConcurrency(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `TRUNCATE people_groups_audit_entries,business_commands,roster_snapshots,group_capability_revisions,group_capability_assignments,membership_revisions,memberships,group_revisions,groups,definition_revisions,definitions,person_employment_events,person_revisions,persons`)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(pool)
	if err != nil {
		t.Fatal(err)
	}
	actor := Actor{Kind: "template_subject", ID: "44444444-4444-4444-8444-444444444444", DisplayName: "Integration Admin", TenantID: testTenant}

	groupType, _ := service.CreateDefinition(ctx, actor, DefinitionGroupType, CreateDefinitionInput{CommandID: id(1), Key: "construction_team", DisplayName: "施工队"})
	if groupType.Revision != 1 {
		t.Fatalf("unexpected definition: %+v", groupType)
	}
	_, err = service.CreateDefinition(ctx, actor, DefinitionMembershipRole, CreateDefinitionInput{CommandID: id(2), Key: "leader", DisplayName: "负责人"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.CreateDefinition(ctx, actor, DefinitionMembershipRole, CreateDefinitionInput{CommandID: id(3), Key: "member", DisplayName: "成员"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.CreateDefinition(ctx, actor, DefinitionCapability, CreateDefinitionInput{CommandID: id(4), Key: "site_installation", DisplayName: "现场安装"})
	if err != nil {
		t.Fatal(err)
	}

	alice, err := service.CreatePerson(ctx, actor, CreatePersonInput{CommandID: id(10), ReferenceCode: "P-001", DisplayName: "张三"})
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := service.CreatePerson(ctx, actor, CreatePersonInput{CommandID: id(10), ReferenceCode: "P-001", DisplayName: "张三"})
	if err != nil || replayed.ID != alice.ID {
		t.Fatalf("idempotent replay failed: %+v %v", replayed, err)
	}
	if _, err := service.CreatePerson(ctx, actor, CreatePersonInput{CommandID: id(10), ReferenceCode: "P-002", DisplayName: "冲突"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected payload conflict, got %v", err)
	}
	bob, err := service.CreatePerson(ctx, actor, CreatePersonInput{CommandID: id(11), ReferenceCode: "P-002", DisplayName: "李四"})
	if err != nil {
		t.Fatal(err)
	}
	carol, err := service.CreatePerson(ctx, actor, CreatePersonInput{CommandID: id(12), ReferenceCode: "P-003", DisplayName: "王五"})
	if err != nil {
		t.Fatal(err)
	}
	dave, err := service.CreatePerson(ctx, actor, CreatePersonInput{CommandID: id(13), ReferenceCode: "P-004", DisplayName: "赵六"})
	if err != nil {
		t.Fatal(err)
	}

	team1, err := service.CreateGroup(ctx, actor, CreateGroupInput{CommandID: id(20), GroupTypeKey: "construction_team", Name: "第一协作组", Purpose: "现场安装协作"})
	if err != nil {
		t.Fatal(err)
	}
	team2, err := service.CreateGroup(ctx, actor, CreateGroupInput{CommandID: id(21), GroupTypeKey: "construction_team", Name: "第二协作组"})
	if err != nil {
		t.Fatal(err)
	}
	m1, err := service.JoinMembership(ctx, actor, team1.ID, JoinMembershipInput{CommandID: id(30), ExpectedRevision: team1.Revision, PersonID: alice.ID, RoleKey: "leader", StartDate: "2026-09-01"})
	if err != nil {
		t.Fatal(err)
	}
	team1.Revision = m1.GroupRevision
	m2, err := service.JoinMembership(ctx, actor, team1.ID, JoinMembershipInput{CommandID: id(31), ExpectedRevision: team1.Revision, PersonID: bob.ID, RoleKey: "member", StartDate: "2026-09-02"})
	if err != nil {
		t.Fatal(err)
	}
	team1.Revision = m2.GroupRevision
	other, err := service.JoinMembership(ctx, actor, team2.ID, JoinMembershipInput{CommandID: id(32), ExpectedRevision: team2.Revision, PersonID: alice.ID, RoleKey: "member", StartDate: "2026-09-03"})
	if err != nil {
		t.Fatal(err)
	}
	team2.Revision = other.GroupRevision

	changed, err := service.ChangeMembershipRole(ctx, actor, team1.ID, m2.ID, ChangeMembershipRoleInput{CommandID: id(33), ExpectedGroupRevision: team1.Revision, ExpectedRevision: m2.Revision, RoleKey: "leader"})
	if err != nil {
		t.Fatal(err)
	}
	team1.Revision = changed.GroupRevision
	capability, err := service.AssignCapability(ctx, actor, team1.ID, AssignCapabilityInput{CommandID: id(34), ExpectedRevision: team1.Revision, CapabilityKey: "site_installation", Note: "可承担现场安装", StartDate: "2026-09-01"})
	if err != nil {
		t.Fatal(err)
	}
	team1.Revision = capability.GroupRevision
	snapshot, err := service.PublishSnapshot(ctx, actor, team1.ID, GroupCommandInput{CommandID: id(35), ExpectedRevision: team1.Revision})
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(snapshot.Payload)
	if hex.EncodeToString(digest[:]) != snapshot.SHA256 {
		t.Fatal("snapshot digest is not reproducible")
	}
	originalPayload := string(snapshot.Payload)

	alice, err = service.UpdatePersonProfile(ctx, actor, alice.ID, UpdatePersonProfileInput{CommandID: id(36), ExpectedRevision: alice.Revision, ReferenceCode: alice.ReferenceCode, DisplayName: "张三（更新）"})
	if err != nil {
		t.Fatal(err)
	}
	team1, err = service.GetGroup(ctx, testTenant, team1.ID)
	if err != nil {
		t.Fatal(err)
	}
	team2, err = service.GetGroup(ctx, testTenant, team2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if team1.Revision != snapshot.GroupRevision+1 || team2.Revision != other.GroupRevision+1 {
		t.Fatalf("person change did not advance active rosters: team1=%d team2=%d", team1.Revision, team2.Revision)
	}
	old, err := service.GetSnapshot(ctx, testTenant, team1.ID, snapshot.GroupRevision)
	if err != nil {
		t.Fatal(err)
	}
	if string(old.Payload) != originalPayload {
		t.Fatal("published snapshot changed after person update")
	}

	employee, err := service.CreatePerson(ctx, actor, CreatePersonInput{CommandID: id(70), ReferenceCode: "P-EMP", DisplayName: "任职周期成员", Department: "财务部", PositionTitle: "出纳", EmploymentStartDate: "2026-01-10"})
	if err != nil || employee.StatusEffectiveDate != "2026-01-10" {
		t.Fatalf("create employment profile: %+v %v", employee, err)
	}
	employee, err = service.SetPersonStatus(ctx, actor, employee.ID, SetPersonStatusInput{CommandID: id(71), ExpectedRevision: employee.Revision, Status: "inactive", EffectiveDate: "2026-08-31", Reason: "离职"})
	if err != nil || employee.Status != "inactive" || employee.Revision != 2 {
		t.Fatalf("offboard person: %+v %v", employee, err)
	}
	employee, err = service.SetPersonStatus(ctx, actor, employee.ID, SetPersonStatusInput{CommandID: id(72), ExpectedRevision: employee.Revision, Status: "active", EffectiveDate: "2026-10-01", Reason: "重新入职"})
	if err != nil || employee.Status != "active" || employee.Revision != 3 {
		t.Fatalf("rehire person: %+v %v", employee, err)
	}
	history, err := service.ListEmploymentEvents(ctx, testTenant, employee.ID)
	if err != nil || len(history) != 3 || history[0].EventKind != "hired" || history[1].EventKind != "offboarded" || history[2].EventKind != "rehired" || history[2].PersonID != employee.ID {
		t.Fatalf("employment history: %+v %v", history, err)
	}

	ended, err := service.EndMembership(ctx, actor, team1.ID, m1.ID, EndMembershipInput{CommandID: id(37), ExpectedGroupRevision: team1.Revision, ExpectedRevision: m1.Revision, EndDate: "2026-09-10"})
	if err != nil {
		t.Fatal(err)
	}
	team1.Revision = ended.GroupRevision
	rejoined, err := service.JoinMembership(ctx, actor, team1.ID, JoinMembershipInput{CommandID: id(38), ExpectedRevision: team1.Revision, PersonID: alice.ID, RoleKey: "member", StartDate: "2026-09-11"})
	if err != nil {
		t.Fatal(err)
	}
	team1.Revision = rejoined.GroupRevision
	if _, err := service.JoinMembership(ctx, actor, team1.ID, JoinMembershipInput{CommandID: id(39), ExpectedRevision: team1.Revision, PersonID: alice.ID, RoleKey: "member", StartDate: "2026-09-12"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected one-active-membership conflict, got %v", err)
	}

	startRevision := team1.Revision
	var wg sync.WaitGroup
	wg.Add(2)
	results := make(chan error, 2)
	for index, person := range []Person{carol, dave} {
		go func(i int, p Person) {
			defer wg.Done()
			_, err := service.JoinMembership(ctx, actor, team1.ID, JoinMembershipInput{CommandID: id(50 + i), ExpectedRevision: startRevision, PersonID: p.ID, RoleKey: "member", StartDate: "2026-09-12"})
			results <- err
		}(index, person)
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrConflict) {
			conflict++
		} else {
			t.Fatalf("unexpected concurrent result: %v", err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("expected one success and one conflict, got %d/%d", success, conflict)
	}

	team1, err = service.GetGroup(ctx, testTenant, team1.ID)
	if err != nil {
		t.Fatal(err)
	}
	archived, err := service.ArchiveGroup(ctx, actor, team1.ID, GroupCommandInput{CommandID: id(60), ExpectedRevision: team1.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.PublishSnapshot(ctx, actor, team1.ID, GroupCommandInput{CommandID: id(61), ExpectedRevision: archived.Revision}); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected archived snapshot rejection, got %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE membership_revisions SET role_key='tampered' WHERE tenant_id=$1 AND membership_id=$2`, testTenant, m1.ID); err == nil {
		t.Fatal("membership history update unexpectedly succeeded")
	}
}

func id(value int) string { return "00000000-0000-4000-8000-" + leftPad(value) }
func leftPad(value int) string {
	raw := []byte("000000000000")
	digits := []byte(string(rune('0' + value%10)))
	if value >= 10 {
		digits = []byte{byte('0' + value/10), byte('0' + value%10)}
	}
	copy(raw[len(raw)-len(digits):], digits)
	return string(raw)
}
