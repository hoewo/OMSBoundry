package buildinfo

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestVersionOutput(t *testing.T) {
	previousVersion, previousCommit, previousBuiltAt := Version, Commit, BuiltAt
	t.Cleanup(func() { Version, Commit, BuiltAt = previousVersion, previousCommit, previousBuiltAt })
	Version, Commit, BuiltAt = "v1.2.3", "abc123", "2026-09-20T12:00:00Z"
	var output bytes.Buffer
	if err := Write(&output); err != nil {
		t.Fatal(err)
	}
	var info Info
	if err := json.Unmarshal(output.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info.Service != "money-flow-ledger" || info.Version != Version || info.Commit != Commit || info.BuiltAt != BuiltAt || info.GoVersion == "" || info.Target == "" {
		t.Fatalf("version info = %#v", info)
	}
	if !Requested([]string{"version"}) || !Requested([]string{"--version"}) || Requested(nil) || Requested([]string{"version", "extra"}) {
		t.Fatal("Requested() version argument handling is invalid")
	}
}
