package main

import (
	"net/url"
	"strings"
	"testing"
)

func TestTamperQueryPreservesOtherValues(t *testing.T) {
	raw := "http://127.0.0.1/file?disposition=inline&file_name=proof.pdf&token=abc"
	changed, err := tamperQuery(raw, "file_name", func(string) string { return "changed.pdf" })
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(changed)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Query().Get("file_name") != "changed.pdf" || parsed.Query().Get("token") != "abc" || parsed.Query().Get("disposition") != "inline" {
		t.Fatalf("unexpected query: %s", parsed.RawQuery)
	}
}

func TestSHA256Hex(t *testing.T) {
	got := sha256Hex([]byte("evidence"))
	if len(got) != 64 || strings.ToLower(got) != got {
		t.Fatalf("sha256Hex returned %q", got)
	}
}
