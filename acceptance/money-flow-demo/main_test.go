package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormHelpersScopeValuesToAction(t *testing.T) {
	page := `<form action="/other"><input name="csrf_token" value="wrong"></form>` +
		`<form method="post" action="/obligations/id/close">` +
		`<input name="csrf_token" value="right&amp;safe">` +
		`<input name="expected_balance_sequence" value="3">` +
		`</form>`
	form, err := formByAction(page, "/obligations/id/close")
	if err != nil {
		t.Fatal(err)
	}
	if got := hiddenValue(form, "csrf_token"); got != "right&safe" {
		t.Fatalf("hiddenValue(csrf_token) = %q", got)
	}
	if got := hiddenValue(form, "expected_balance_sequence"); got != "3" {
		t.Fatalf("hiddenValue(expected_balance_sequence) = %q", got)
	}
}

func TestStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	want := demoState{OwnerPartyID: "owner-1", ObligationID: "obligation-1", ObligationClosed: true}
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
