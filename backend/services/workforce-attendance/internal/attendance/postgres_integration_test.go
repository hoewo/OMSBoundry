//go:build integration

package attendance

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"omsbizfoundry/workforce-attendance/internal/database"
)

type fixedEmployment struct{ value EmploymentSnapshot }

func (f fixedEmployment) Snapshot(context.Context, string, string) (EmploymentSnapshot, error) {
	return f.value, nil
}

func TestAttendanceLifecycleRevisionReplayAndCalculation(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	actor := Actor{TenantID: "11111111-1111-4111-8111-111111111111", SubjectID: "22222222-2222-4222-8222-222222222222", DisplayName: "管理员", CredentialID: "33333333-3333-4333-8333-333333333333"}
	personID := "44444444-4444-4444-8444-444444444444"
	service, _ := NewService(store, fixedEmployment{value: EmploymentSnapshot{PersonID: personID, DisplayName: "员工一", SourceRevision: 3, SourceDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Intervals: []EmploymentInterval{{From: "2026-11-16"}}}})
	policy, err := service.UpsertPolicy(ctx, actor, UpsertPolicyInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa1", Key: "beijing-calendar", Period: "2026-11", Basis: "calendar_days", Reason: "月中入职按日折算"})
	if err != nil || policy.Revision != 1 {
		t.Fatalf("policy=%+v err=%v", policy, err)
	}
	if replay, replayErr := service.UpsertPolicy(ctx, actor, UpsertPolicyInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa1", Key: "beijing-calendar", Period: "2026-11", Basis: "calendar_days", Reason: "月中入职按日折算"}); replayErr != nil || replay.Revision != 1 {
		t.Fatalf("replay=%+v err=%v", replay, replayErr)
	}
	if _, conflict := service.UpsertPolicy(ctx, actor, UpsertPolicyInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa1", Key: "beijing-calendar", Period: "2026-11", Basis: "calendar_days", Reason: "冲突"}); !errors.Is(conflict, ErrIdempotencyConflict) {
		t.Fatalf("expected command conflict, got %v", conflict)
	}
	leave, err := service.RecordLeave(ctx, actor, RecordLeaveInput{CommandID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbb1", PersonID: personID, From: "2026-11-20", Through: "2026-11-20", LeaveType: "事假", UnitsPerDay: 10000, PaidRatioBPS: 0, Reason: "个人事项"})
	if err != nil || leave.Revision != 1 {
		t.Fatalf("leave=%+v err=%v", leave, err)
	}
	summary, err := service.CalculateSummary(ctx, actor, CalculateSummaryInput{CommandID: "cccccccc-cccc-4ccc-8ccc-ccccccccccc1", PersonID: personID, Period: "2026-11", PolicyKey: policy.Key})
	if err != nil || summary.StandardUnits != 300000 || summary.EmploymentEligibleUnits != 150000 || summary.PayableUnits != 140000 {
		t.Fatalf("summary=%+v err=%v", summary, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE attendance_monthly_summary_revisions SET snapshot='{}' WHERE tenant_id=$1 AND summary_id=$2`, actor.TenantID, summary.ID); err == nil {
		t.Fatal("summary revision must be immutable")
	}
}

func integrationPool(t *testing.T, ctx context.Context) (*pgxpool.Pool, func()) {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	admin, err := pgxpool.New(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	var random [8]byte
	_, _ = rand.Read(random[:])
	schema := "attendance_test_" + hex.EncodeToString(random[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	config, _ := pgxpool.ParseConfig(raw)
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	if err = database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool, func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
		admin.Close()
	}
}
