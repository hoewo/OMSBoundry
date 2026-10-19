package main

import "testing"

func TestCommandSetProducesDistinctUUIDs(t *testing.T) {
	commands := commandSet{nonce: "0123456789ab"}
	first, second := commands.id(1), commands.id(2)
	if first == second || first != "aaaaaaaa-aaaa-4aaa-8aaa-012345678901" || second != "aaaaaaaa-aaaa-4aaa-8aaa-012345678902" {
		t.Fatalf("unexpected command IDs: %q %q", first, second)
	}
}

func TestScopedID(t *testing.T) {
	if got := scopedID("11111111-1111-4111-8111", "0123456789ab"); got != "11111111-1111-4111-8111-0123456789ab" {
		t.Fatalf("scopedID = %q", got)
	}
}
