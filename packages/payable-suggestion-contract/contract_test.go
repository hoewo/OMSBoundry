package payablesuggestioncontract

import "testing"

func TestDigestDistinguishesPartySourceAndBonusSource(t *testing.T) {
	base := Submission{
		SchemaVersion: SchemaVersion,
		SuggestionID:  "11000000-0000-4000-8000-000000000001",
		TenantID:      "22000000-0000-4000-8000-000000000001",
		ProjectID:     "33000000-0000-4000-8000-000000000001",
		Source: SourceSnapshot{
			Application: SourceApplicationBonusDistribution, ResourceType: SourceBonusAllocation,
			ResourceID: "44000000-0000-4000-8000-000000000001", Revision: 2,
			Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		LedgerOwner: PartySnapshot{
			SourceApplication: PartySourceApplicationMoneyFlow, ResourceType: PartyResourceParty,
			ID: "55000000-0000-4000-8000-000000000001", Revision: 1, Kind: "organization",
		},
		Counterparty: PartySnapshot{
			SourceApplication: PartySourceApplicationPeopleGroups, ResourceType: PartyResourcePerson,
			ID: "66000000-0000-4000-8000-000000000001", Revision: 3, Kind: "individual", LegalName: "张三",
		},
		AmountMinor: 10000, Currency: "CNY", DueOn: "2026-10-10", Purpose: "项目奖金",
	}
	first, err := Digest(base)
	if err != nil {
		t.Fatal(err)
	}
	base.Counterparty.SourceApplication = PartySourceApplicationPartyDirectory
	second, err := Digest(base)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("party source identity must affect the immutable request digest")
	}
}
