//go:build integration

package payroll

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type profilePersonReferences struct{}

func (profilePersonReferences) ValidatePerson(_ context.Context, tenant, id string) (PersonReference, error) {
	if id == "99999999-9999-4999-8999-999999999999" {
		return PersonReference{}, ErrReferenceInvalid
	}
	status := "active"
	if id == "88888888-8888-4888-8888-888888888888" {
		status = "inactive"
	}
	return PersonReference{ID: id, TenantID: tenant, ReferenceCode: "EMP-" + strings.ToUpper(id[len(id)-4:]), DisplayName: "成员甲", Status: status, Revision: 7, Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, nil
}
func (profilePersonReferences) ResolvePersonForSubject(_ context.Context, tenant, subject string) (PersonReference, error) {
	if tenant == "" || subject == "" {
		return PersonReference{}, ErrReferenceInvalid
	}
	return PersonReference{ID: subject, TenantID: tenant, ReferenceCode: "EMP-001", DisplayName: "成员甲", Status: "active", Revision: 7, Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, nil
}

func TestMemberProfilesRevisionMaskingSchemesAndIsolation(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	service, _ := NewService(store, profilePersonReferences{})
	actor := Actor{ID: "22222222-2222-4222-8222-222222222222", TenantID: "11111111-1111-4111-8111-111111111111", CredentialID: "33333333-3333-4333-8333-333333333333"}
	person := "44444444-4444-4444-8444-444444444444"
	compIn := UpsertCompensationProfileInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa1", PersonID: person, EffectiveMonth: "2026-10", FixedSalaryMinor: 1000000, SocialInsuranceBaseMinor: 800000, HousingFundBaseMinor: 800000, Reason: "首次建档"}
	comp, err := service.UpsertCompensation(ctx, actor, compIn)
	if err != nil || comp.Revision != 1 {
		t.Fatalf("create compensation: %#v %v", comp, err)
	}
	replay, err := service.UpsertCompensation(ctx, actor, compIn)
	if err != nil || replay.ID != comp.ID {
		t.Fatalf("replay compensation: %#v %v", replay, err)
	}
	compIn.CommandID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa2"
	compIn.ExpectedRevision = 1
	compIn.EffectiveMonth = "2026-11"
	compIn.FixedSalaryMinor = 1100000
	compIn.Reason = "调薪"
	comp, err = service.UpsertCompensation(ctx, actor, compIn)
	if err != nil || comp.Revision != 2 {
		t.Fatalf("revise compensation: %#v %v", comp, err)
	}
	if _, err = service.GetCompensation(ctx, "55555555-5555-4555-8555-555555555555", person); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("cross tenant compensation read=%v", err)
	}
	inactivePerson := "88888888-8888-4888-8888-888888888888"
	if _, err = service.UpsertCompensation(ctx, actor, UpsertCompensationProfileInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa3", PersonID: inactivePerson, EffectiveMonth: "2026-10", FixedSalaryMinor: 900000, SocialInsuranceBaseMinor: 800000, HousingFundBaseMinor: 800000, Reason: "离职结算档案"}); err != nil {
		t.Fatalf("inactive person compensation must remain maintainable: %v", err)
	}

	paymentIn := UpsertPaymentProfileInput{CommandID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbb1", PersonID: person, AccountName: "成员甲", BankName: "测试银行", AccountNumber: "6222021234567890", Status: "active", EffectiveMonth: "2026-10", Reason: "首次建档", Confirmed: true}
	payment, err := service.UpsertPayment(ctx, actor, paymentIn)
	if err != nil || payment.Revision != 1 || !strings.HasSuffix(payment.MaskedAccountNumber, "7890") {
		t.Fatalf("create payment: %#v %v", payment, err)
	}
	raw, _ := json.Marshal(payment)
	if strings.Contains(string(raw), "6222021234567890") {
		t.Fatalf("masked response leaked full account: %s", raw)
	}
	paymentIn.CommandID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbb2"
	paymentIn.ExpectedRevision = 1
	paymentIn.AccountNumber = "6222029876543210"
	paymentIn.EffectiveMonth = "2026-11"
	paymentIn.Reason = "换卡"
	payment, err = service.UpsertPayment(ctx, actor, paymentIn)
	if err != nil || payment.Revision != 2 {
		t.Fatalf("revise payment: %#v %v", payment, err)
	}
	history, err := service.ListPaymentHistory(ctx, actor.TenantID, person)
	if err != nil || len(history) != 2 || !strings.HasSuffix(history[0].MaskedAccountNumber, "7890") || !strings.HasSuffix(history[1].MaskedAccountNumber, "3210") {
		t.Fatalf("payment history: %#v %v", history, err)
	}
	full, err := service.ReadSensitivePayment(ctx, actor, person, "工资发放前核对")
	if err != nil || full.AccountNumber != "6222029876543210" {
		t.Fatalf("sensitive read: %#v %v", full, err)
	}
	var accesses int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM payroll_sensitive_account_access WHERE tenant_id=$1 AND person_id=$2`, actor.TenantID, person).Scan(&accesses); err != nil || accesses != 1 {
		t.Fatalf("sensitive audit=%d %v", accesses, err)
	}

	components := []ContributionComponent{{Key: "pension", BaseKind: "social", IndividualRateBPS: 800, EmployerRateBPS: 1600, BaseFloorMinor: 300000, BaseCapMinor: 3000000, RoundingMode: "half_up"}, {Key: "medical", BaseKind: "social", IndividualRateBPS: 200, EmployerRateBPS: 900, BaseFloorMinor: 300000, BaseCapMinor: 3000000, RoundingMode: "half_up"}, {Key: "unemployment", BaseKind: "social", IndividualRateBPS: 50, EmployerRateBPS: 50, BaseFloorMinor: 300000, BaseCapMinor: 3000000, RoundingMode: "half_up"}, {Key: "work_injury", BaseKind: "social", IndividualRateBPS: 0, EmployerRateBPS: 20, BaseFloorMinor: 300000, BaseCapMinor: 3000000, RoundingMode: "half_up"}, {Key: "maternity", BaseKind: "social", IndividualRateBPS: 0, EmployerRateBPS: 80, BaseFloorMinor: 300000, BaseCapMinor: 3000000, RoundingMode: "half_up"}, {Key: "housing_fund", BaseKind: "housing", IndividualRateBPS: 700, EmployerRateBPS: 700, BaseFloorMinor: 300000, BaseCapMinor: 3000000, RoundingMode: "half_up"}}
	seeded, err := service.ListSchemes(ctx, actor.TenantID)
	if err != nil || len(seeded) != 1 || seeded[0].ID != "20260000-0004-4000-8000-000000000001" || componentRate(seeded[0].Components, "housing_fund") != 500 {
		t.Fatalf("seeded Beijing reference scheme: %#v %v", seeded, err)
	}
	if medical := contributionComponent(seeded[0].Components, "medical"); medical.IndividualFixedMinor != 300 {
		t.Fatalf("seeded Beijing medical fixed contribution: %#v", medical)
	}
	standard, err := service.CreateStandardScheme(ctx, actor, CreateStandardSchemeInput{CommandID: "cccccccc-cccc-4ccc-8ccc-ccccccccccc1", Title: "测试权威方案", RegionCode: "CN-TEST", RegionName: "测试地区", EffectiveMonth: "2026-10", AuthoritySource: "https://authority.example.test/rules/2026", AuthorityVerifiedAt: time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC), Components: components, Reason: "权威规则导入"})
	if err != nil || standard.SchemeKind != "standard" {
		t.Fatalf("standard scheme: %#v %v", standard, err)
	}
	customComponents := append([]ContributionComponent(nil), components...)
	for i := range customComponents {
		if customComponents[i].Key == "housing_fund" {
			customComponents[i].IndividualRateBPS = 1200
			customComponents[i].EmployerRateBPS = 1200
		}
	}
	custom, err := service.CreateCustomScheme(ctx, actor, CreateCustomSchemeInput{CommandID: "dddddddd-dddd-4ddd-8ddd-ddddddddddd1", SourceSchemeID: standard.ID, SourceSchemeRevision: 1, Title: "本公司方案", EffectiveMonth: "2026-11", Components: customComponents, Reason: "租户公积金方案"})
	if err != nil || custom.SchemeKind != "custom" || custom.SourceSchemeID != standard.ID {
		t.Fatalf("custom scheme: %#v %v", custom, err)
	}
	revisedComponents := append([]ContributionComponent(nil), customComponents...)
	for i := range revisedComponents {
		if revisedComponents[i].Key == "housing_fund" {
			revisedComponents[i].IndividualRateBPS = 1000
			revisedComponents[i].EmployerRateBPS = 1000
		}
	}
	custom, err = service.ReviseCustomScheme(ctx, actor, custom.ID, ReviseCustomSchemeInput{CommandID: "dddddddd-dddd-4ddd-8ddd-ddddddddddd2", ExpectedRevision: 1, Title: "本公司方案", EffectiveMonth: "2026-12", Components: revisedComponents, Reason: "公积金比例变更"})
	if err != nil || custom.Revision != 2 || custom.SourceSchemeID != standard.ID || componentRate(custom.Components, "housing_fund") != 1000 {
		t.Fatalf("revise custom scheme: %#v %v", custom, err)
	}
	if _, err = service.ReviseCustomScheme(ctx, actor, custom.ID, ReviseCustomSchemeInput{CommandID: "dddddddd-dddd-4ddd-8ddd-ddddddddddd3", ExpectedRevision: 1, Title: "本公司方案", EffectiveMonth: "2027-01", Components: revisedComponents, Reason: "过期修改"}); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale custom scheme revision=%v", err)
	}
	var schemeHistory int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM contribution_scheme_revisions WHERE scheme_id=$1`, custom.ID).Scan(&schemeHistory); err != nil || schemeHistory != 2 {
		t.Fatalf("custom scheme history=%d %v", schemeHistory, err)
	}
	schemes, err := service.ListSchemes(ctx, actor.TenantID)
	if err != nil || len(schemes) != 3 {
		t.Fatalf("scheme list: %#v %v", schemes, err)
	}
	other, err := service.ListSchemes(ctx, "55555555-5555-4555-8555-555555555555")
	if err != nil || len(other) != 2 || other[0].SchemeKind != "standard" || other[1].SchemeKind != "standard" {
		t.Fatalf("tenant scheme isolation: %#v %v", other, err)
	}

	if _, err = pool.Exec(ctx, `UPDATE payment_profiles SET account_number='1234' WHERE tenant_id=$1 AND person_id=$2`, actor.TenantID, person); err == nil {
		t.Fatal("payment current projection tamper must fail")
	}
}

func componentRate(values []ContributionComponent, key string) int64 {
	for _, value := range values {
		if value.Key == key {
			return value.IndividualRateBPS
		}
	}
	return -1
}

func contributionComponent(values []ContributionComponent, key string) ContributionComponent {
	for _, value := range values {
		if value.Key == key {
			return value
		}
	}
	return ContributionComponent{}
}
