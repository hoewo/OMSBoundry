package httpserver

import (
	"context"
	"testing"

	"omsbizfoundry/money-flow-ledger/internal/partymigration"
)

type partyRuntimeStub struct {
	values []partymigration.RuntimePartyReference
}

func (s partyRuntimeStub) State(context.Context, string) (partymigration.RuntimeState, error) {
	return partymigration.RuntimeState{Mode: partymigration.RuntimeModeDirectory, DirectoryAuthorityEstablished: true}, nil
}

func (s partyRuntimeStub) LocalPartyWritesAllowed(context.Context, string) (bool, error) {
	return false, nil
}

func (s partyRuntimeStub) ResolveReferences(context.Context, string, []string) ([]partymigration.RuntimePartyReference, error) {
	return s.values, nil
}

func TestResolveReferencePartiesProjectsDirectoryIdentity(t *testing.T) {
	registration := "91330000TARGET"
	server := &Server{partyRuntime: partyRuntimeStub{values: []partymigration.RuntimePartyReference{{
		LocalPartyID: "11111111-1111-4111-8111-111111111111",
		PartyID:      "22222222-2222-4222-8222-222222222222", Revision: 7,
		Kind: "organization", LegalName: "目录主体", RegistrationIdentifier: &registration,
		Source: partymigration.RuntimeModeDirectory,
	}}}}
	ids, references, err := server.resolveReferenceParties(context.Background(), referenceTestTenant, []string{"11111111-1111-4111-8111-111111111111"})
	if err != nil || len(ids) != 1 || ids[0] != "22222222-2222-4222-8222-222222222222" || len(references) != 1 || references[0].Revision != 7 || references[0].Source != partymigration.RuntimeModeDirectory {
		t.Fatalf("projected Party references ids=%#v refs=%#v err=%v", ids, references, err)
	}
}

func TestResolveReferencePartiesKeepsLegacyShapeWithoutRuntime(t *testing.T) {
	local := []string{"11111111-1111-4111-8111-111111111111"}
	ids, references, err := (&Server{}).resolveReferenceParties(context.Background(), referenceTestTenant, local)
	if err != nil || len(ids) != 1 || ids[0] != local[0] || references != nil {
		t.Fatalf("legacy Party references ids=%#v refs=%#v err=%v", ids, references, err)
	}
}
