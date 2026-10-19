package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormParsingKeepsIDsAndEscapedValues(t *testing.T) {
	page := `<form action="/contracts/contract-1/parties">
<input type="hidden" name="csrf_token" value="token&amp;value">
<option value="11111111-1111-4111-8111-111111111111">星禾科技有限公司（OA-DEMO-CLIENT）</option>
</form>`
	form, err := formByAction(page, "/contracts/contract-1/parties")
	if err != nil {
		t.Fatal(err)
	}
	if got := hiddenValue(form, "csrf_token"); got != "token&value" {
		t.Fatalf("hiddenValue() = %q", got)
	}
	got, err := optionValue(form, "OA-DEMO-CLIENT")
	if err != nil {
		t.Fatal(err)
	}
	if want := "11111111-1111-4111-8111-111111111111"; got != want {
		t.Fatalf("optionValue() = %q, want %q", got, want)
	}
}

func TestRedirectIDParsingRejectsMissingOrNestedIDs(t *testing.T) {
	if got, err := queryID("/contracts?contract_id=contract-1", "contract_id"); err != nil || got != "contract-1" {
		t.Fatalf("queryID() = %q, %v", got, err)
	}
	if _, err := queryID("/contracts", "contract_id"); err == nil {
		t.Fatal("queryID() accepted a missing id")
	}
	if got, err := pathID("/obligations/obligation-1", "/obligations/"); err != nil || got != "obligation-1" {
		t.Fatalf("pathID() = %q, %v", got, err)
	}
	if _, err := pathID("/obligations/obligation-1/extra", "/obligations/"); err == nil {
		t.Fatal("pathID() accepted a nested resource path")
	}
}

func TestStateRoundTripUsesOwnerOnlyPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime", "demo-state.json")
	want := state{ProjectID: "project-1", ContractID: "contract-1", EvidenceVersion: 1}
	if err := writeState(path, want); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("state permissions = %o, want 600", got)
	}
	got, err := readState(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProjectID != want.ProjectID || got.ContractID != want.ContractID || got.EvidenceVersion != want.EvidenceVersion {
		t.Fatalf("readState() = %+v, want %+v", got, want)
	}
}

func TestReadStateAcceptsPreHighFidelityState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "demo-state.json")
	legacy := []byte(`{
  "project_id": "project-legacy",
  "contract_id": "contract-legacy",
  "obligation_id": "obligation-legacy",
  "cash_transaction_id": "cash-legacy",
  "allocation_id": "allocation-legacy",
  "evidence_file_id": "evidence-legacy",
  "evidence_version": 1
}`)
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := readState(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProjectID != "project-legacy" || got.ContractID != "contract-legacy" || got.ObligationID != "obligation-legacy" || got.CashTransactionID != "cash-legacy" || got.AllocationID != "allocation-legacy" || got.EvidenceFileID != "evidence-legacy" || got.EvidenceVersion != 1 {
		t.Fatalf("readState() lost legacy facts: %+v", got)
	}
	if got.ContractChanged || got.ContractCorrected || got.ContractTerminated || got.ContractProgressSettled || got.ContractFinalSettled || got.ContractClosed || got.MoneyAdjusted || got.MoneyRemissionRecorded || got.ObligationClosed || got.ErrorCashReversed {
		t.Fatalf("readState() invented high-fidelity progress: %+v", got)
	}
}
