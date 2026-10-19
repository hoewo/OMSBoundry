//go:build integration

package referencebonus

import (
	"context"
	core "omsbizfoundry/bonus-distribution/internal/bonus"
	"reflect"
	"testing"
)

func TestProfessionalPG18AtomicSnapshotsHistoryAndSQLGuard(t *testing.T) {
	pool, cleanup := integrationPool(t)
	defer cleanup()
	ctx := context.Background()
	store, _ := NewPostgresStore(pool)
	a, f, in := professionalTestFixture()
	s, _ := NewWithProfile(store, f, ProfessionalProfile)
	rows, err := s.Create(ctx, a, in)
	if err != nil || len(rows) != 6 {
		t.Fatalf("rows=%+v %v", rows, err)
	}
	again, err := s.Create(ctx, a, in)
	if err != nil || !reflect.DeepEqual(rows, again) {
		t.Fatal("replay", err)
	}
	v, err := s.Withdraw(ctx, a, rows[0].ID, WithdrawInput{CommandID: testID(901), ExpectedRevision: 1, Reason: "原生产奖金撤回"})
	if err != nil || v.Discipline.DisplayName != "桥梁" || v.State != "withdrawn" {
		t.Fatal(v, err)
	}
	h, err := s.History(ctx, a, v.ID)
	if err != nil || len(h) != 2 || h[0].Snapshot.AssignmentSource != ProfessionalSource {
		t.Fatal(h, err)
	}
	for _, statement := range []string{
		`UPDATE reference_oa_bonus_allocations SET snapshot=jsonb_set(snapshot,'{discipline,display_name}','"篡改专业"') WHERE id=$1`,
		`DELETE FROM reference_oa_bonus_allocations WHERE id=$1`,
	} {
		if _, err = pool.Exec(ctx, statement, rows[0].ID); err == nil {
			t.Fatal("immutable guard accepted", statement)
		}
	}
	var before string
	const digestSQL = `SELECT md5(string_agg(snapshot::text,E'\n' ORDER BY id)) FROM reference_oa_bonus_allocations`
	if err = pool.QueryRow(ctx, digestSQL).Scan(&before); err != nil {
		t.Fatal(err)
	}
	bad := rows[1]
	bad.ID = testID(902)
	bad.BatchID = testID(903)
	bad.AssignmentSource = ""
	bad.Profile = ""
	bad.Discipline = nil
	bad.Assignment.RoleKey = "designer"
	good := rows[2]
	good.ID = testID(904)
	good.BatchID = bad.BatchID
	if _, err = store.Commit(ctx, a, testID(905), hash(a, "record", "", in), "record", "", 0, []Record{good, bad}); err == nil {
		t.Fatal("v2 professional role forged")
	}
	var after string
	if err = pool.QueryRow(ctx, digestSQL).Scan(&after); err != nil || after != before {
		t.Fatal("partial batch persisted", err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM reference_oa_bonus_commands WHERE command_id=$1`, testID(905)).Scan(&count); err != nil || count != 0 {
		t.Fatal("failed receipt persisted", count, err)
	}
	// Source-qualified IDs are independent namespaces. A legacy production-owner
	// ID collision cannot collapse or impersonate a professional assignment.
	legacy := rows[1]
	legacy.ID = testID(906)
	legacy.BatchID = testID(907)
	legacy.Profile = ""
	legacy.AssignmentSource = ""
	legacy.Discipline = nil
	legacy.Assignment.RoleKey = "production_owner"
	qualified := rows[1]
	qualified.ID = testID(908)
	qualified.BatchID = legacy.BatchID
	if _, err = store.Commit(ctx, a, testID(909), hash(a, "record", "", []core.AssignmentRef{}), "record", "", 0, []Record{legacy, qualified}); err != nil {
		t.Fatal("source-qualified index collision", err)
	}
}
