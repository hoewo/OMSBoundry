package referenceoa

import (
	"errors"
	"omsbizfoundry/contract-ledger/internal/audit"
	"testing"
)

func input() SaveInput {
	return SaveInput{CommandID: "11111111-1111-4111-8111-111111111111", ContractID: "22222222-2222-4222-8222-222222222222", TenantID: "33333333-3333-4333-8333-333333333333", ProjectID: "44444444-4444-4444-8444-444444444444", ChainType: "client_to_qualification", ExpectedDraftVersion: 1, Fees: Fees{"9007199254740993.01", "0.09", "0.90"}, SettlementFollows: true, Actor: audit.ActorRef{Kind: audit.ActorKindIAMSubject, ID: "55555555-5555-4555-8555-555555555555"}}
}
func TestExactFeesAndNullableZeroRate(t *testing.T) {
	v, total, err := normalize(input())
	if err != nil || total != "9007199254740994.00" {
		t.Fatalf("%+v %s %v", v, total, err)
	}
	if v.RateBPS != nil {
		t.Fatal("null rate fabricated")
	}
	zero := int64(0)
	in := input()
	in.RateBPS = &zero
	v, _, err = normalize(in)
	if err != nil || v.RateBPS == nil || *v.RateBPS != 0 {
		t.Fatal("zero rate lost")
	}
	if intentHash(v) == intentHash(input()) {
		t.Fatal("null and zero replay collapsed")
	}
}
func TestInvalidInputMatrix(t *testing.T) {
	for name, edit := range map[string]func(*SaveInput){
		"precision": func(v *SaveInput) { v.Fees.Design = "1.001" }, "exponent": func(v *SaveInput) { v.Fees.Design = "1e9" }, "negative base": func(v *SaveInput) { v.Fees.Design = "-0.01" }, "chain": func(v *SaveInput) { v.ChainType = "unknown" }, "tenant": func(v *SaveInput) { v.TenantID = "" }, "local actor": func(v *SaveInput) { v.Actor.Kind = audit.ActorKindLocalUser }, "negative revision": func(v *SaveInput) { v.ExpectedRevision = -1 }, "follow manual": func(v *SaveInput) { n := "1.00"; v.SettlementAmount = &n }, "rate range": func(v *SaveInput) { r := int64(10001); v.RateBPS = &r }, "base sum overflow": func(v *SaveInput) { v.Fees = Fees{"999999999999999999.99", "0.01", "0"} },
	} {
		t.Run(name, func(t *testing.T) {
			v := input()
			edit(&v)
			if _, _, err := normalize(v); !errors.Is(err, ErrInvalid) {
				t.Fatalf("wanted invalid, got %v", err)
			}
		})
	}
}
func TestAmendmentPreviewDoesNotBecomeConfirmation(t *testing.T) {
	v := input()
	v.Fees = Fees{"10.00", "20.00", "30.00"}
	v.Amendments = []Amendment{{ID: "66666666-6666-4666-8666-666666666666", Number: "A1", SignedOn: "2026-10-04", ChangeType: "adjustment", Fees: Fees{"-1.01", "0.01", "0.00"}}, {ID: "77777777-7777-4777-8777-777777777777", Number: "A2", SignedOn: "2026-10-05", ChangeType: "revision", Fees: Fees{"1.00", "2.00", "3.00"}}, {ID: "88888888-8888-4888-8888-888888888888", Number: "A3", SignedOn: "2026-10-06", ChangeType: "no_amount_change", Fees: Fees{"0", "0", "0"}}}
	normalized, total, err := normalize(v)
	if err != nil || total != "60.00" {
		t.Fatal(total, err)
	}
	fees, err := PreviewFees(normalized.Fees, normalized.Amendments)
	if err != nil || fees != (Fees{"1.00", "2.00", "3.00"}) {
		t.Fatal(fees, err)
	}
	v.Amendments[1].Cancelled = true
	fees, err = PreviewFees(v.Fees, v.Amendments)
	if err != nil || fees != (Fees{"8.99", "20.01", "30.00"}) {
		t.Fatal(fees, err)
	}
	v.Amendments[0].Fees.Design = "-11"
	if _, _, err = normalize(v); !errors.Is(err, ErrInvalid) {
		t.Fatal("negative effective fee accepted")
	}
}

func TestNormalizationDoesNotMutateOrAliasCaller(t *testing.T) {
	in := input()
	rate, amendmentRate := int64(0), int64(1500)
	in.RateBPS = &rate
	in.Amendments = []Amendment{{ID: "66666666-6666-4666-8666-666666666666", Number: " A1 ", SignedOn: "2026-10-04", ChangeType: "adjustment", Fees: Fees{"-0.01", "0", "0"}, RateBPS: &amendmentRate, Description: " note "}}
	out, _, err := normalize(in)
	if err != nil {
		t.Fatal(err)
	}
	if in.Amendments[0].Number != " A1 " || in.Amendments[0].Fees.Survey != "0" || in.Amendments[0].Description != " note " {
		t.Fatal("normalization mutated caller")
	}
	out.Amendments[0].Cancelled = true
	*out.RateBPS = 1
	*out.Amendments[0].RateBPS = 2
	if in.Amendments[0].Cancelled || rate != 0 || amendmentRate != 1500 {
		t.Fatal("normalized receipt aliases caller")
	}
}
