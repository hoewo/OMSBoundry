package referenceoa

import (
	"omsbizfoundry/contract-ledger/internal/audit"
	"omsbizfoundry/contract-ledger/internal/contract"
	"testing"
)

func confirmationInput() ConfirmInput {
	id := "11111111-1111-4111-8111-111111111111"
	return ConfirmInput{CommandID: id, ContractID: id, TenantID: id, AmendmentID: id, ExpectedOverlayRevision: 1, ExpectedContractRevision: 1, EffectiveOn: "2026-10-07", Actor: audit.ActorRef{Kind: audit.ActorKindIAMSubject, ID: id, DisplayName: "测试"}, Amendment: Amendment{ID: id, Number: " A01 ", SignedOn: "2026-10-06", ChangeType: "adjustment", Fees: Fees{"-0.01", "0", "0"}}}
}
func TestConfirmationExactNativeSemanticsAndReplayIdentity(t *testing.T) {
	for _, test := range []struct{ kind, design, effect, amount string }{{"adjustment", "-0.01", contract.AmountEffectAdjustment, "-0.01"}, {"revision", "9007199254740993.01", contract.AmountEffectRestatement, "9007199254740993.01"}, {"no_amount_change", "0", contract.AmountEffectUnchanged, ""}} {
		t.Run(test.kind, func(t *testing.T) {
			in := confirmationInput()
			in.Amendment.ChangeType = test.kind
			in.Amendment.Fees.Design = test.design
			got, effect, amount, e := normalizeConfirmation(in)
			if e != nil || effect != test.effect || amount != test.amount || got.Amendment.Number != "A01" {
				t.Fatalf("normalization=%#v %s %s %v", got, effect, amount, e)
			}
			hash := confirmationHash(got)
			got.Actor.DisplayName = "新显示名称"
			if confirmationHash(got) != hash {
				t.Fatal("display name changed replay identity")
			}
			got.ExpectedOverlayRevision++
			if confirmationHash(got) == hash {
				t.Fatal("stale context ignored")
			}
		})
	}
	for _, mutate := range []func(*ConfirmInput){func(v *ConfirmInput) { v.Amendment.Cancelled = true }, func(v *ConfirmInput) { v.Amendment.ID = "22222222-2222-4222-8222-222222222222" }, func(v *ConfirmInput) { v.EffectiveOn = "2026-02-30" }, func(v *ConfirmInput) { v.ExpectedContractRevision = 0 }, func(v *ConfirmInput) { v.Amendment.Fees.Design = "0" }, func(v *ConfirmInput) { v.Actor.Kind = audit.ActorKindLocalUser }} {
		in := confirmationInput()
		mutate(&in)
		if _, _, _, e := normalizeConfirmation(in); e == nil {
			t.Fatal("invalid confirmation accepted")
		}
	}
}
