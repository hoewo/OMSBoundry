package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFormHelpersScopeValuesToAction(t *testing.T) {
	page := `<form action="/other"><input name="csrf_token" value="wrong"></form>` +
		`<form method="post" action="/contracts/id/changes">` +
		`<input name="csrf_token" value="right&amp;safe">` +
		`<option value="11111111-1111-4111-8111-111111111111">甲方（BF-DEMO-CLIENT）</option>` +
		`</form>`
	form, err := formByAction(page, "/contracts/id/changes")
	if err != nil {
		t.Fatal(err)
	}
	if got := hiddenValue(form, "csrf_token"); got != "right&safe" {
		t.Fatalf("hiddenValue() = %q", got)
	}
	if got, err := optionValue(form, clientCode); err != nil || got != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("optionValue() = %q, %v", got, err)
	}
}

func TestStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	want := demoState{ContractID: "contract-1", Activated: true, Changed: true}
	if err := writeState(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := readState(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("state = %#v, want %#v", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("state mode = %o", info.Mode().Perm())
	}
}

func TestCheckedValuesPreservesFormOrderAndUnescapes(t *testing.T) {
	form := `<input type="checkbox" name="confirming_party_position" value="1&amp;owner" checked>` +
		`<input type="checkbox" name="other" value="ignored" checked>` +
		`<input type="checkbox" name="confirming_party_position" value="2" checked>`
	want := []string{"1&owner", "2"}
	if got := checkedValues(form, "confirming_party_position"); !reflect.DeepEqual(got, want) {
		t.Fatalf("checkedValues() = %#v, want %#v", got, want)
	}
}

func TestEvidenceDocumentIDScopesToContract(t *testing.T) {
	const (
		contractID = "11111111-1111-4111-8111-111111111111"
		documentID = "22222222-2222-4222-8222-222222222222"
	)
	page := `<form action="/contracts/99999999-9999-4999-8999-999999999999/evidence/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa/versions"></form>` +
		`<form action="/contracts/` + contractID + `/evidence/` + documentID + `/versions"></form>`
	got, err := evidenceDocumentID(page, contractID)
	if err != nil {
		t.Fatal(err)
	}
	if got != documentID {
		t.Fatalf("evidenceDocumentID() = %q, want %q", got, documentID)
	}
}

func TestEvidenceDocumentIDRejectsMissingVersionForm(t *testing.T) {
	if _, err := evidenceDocumentID(`<form action="/contracts/other"></form>`, "11111111-1111-4111-8111-111111111111"); err == nil {
		t.Fatal("evidenceDocumentID() accepted a page without the contract evidence form")
	}
}
