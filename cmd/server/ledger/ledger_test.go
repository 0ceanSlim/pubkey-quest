package ledger

import (
	"testing"
)

func TestRecordGetListRemove(t *testing.T) {
	Dir = t.TempDir()
	if _, ok := Get("npub1abc", "pubkey-quest:save:1"); ok {
		t.Fatal("empty ledger returned an entry")
	}
	if err := Record("npub1abc", "pubkey-quest:save:1", Entry{EventID: "e1", Relays: []string{"wss://a"}, CreatedAt: 10, Version: 1}); err != nil {
		t.Fatal(err)
	}
	if err := Record("npub1abc", "pubkey-quest:save:1", Entry{EventID: "e2", Relays: []string{"wss://b"}, CreatedAt: 20, Version: 1}); err != nil {
		t.Fatal(err)
	}
	e, ok := Get("npub1abc", "pubkey-quest:save:1")
	if !ok || e.EventID != "e2" || e.Relays[0] != "wss://b" || e.AcceptedAt == 0 {
		t.Fatalf("latest entry wrong: %+v", e)
	}
	Record("npub1abc", "pubkey-quest:save:2", Entry{EventID: "x"})
	if len(List("npub1abc")) != 2 {
		t.Error("list should hold both slots")
	}
	Remove("npub1abc", "pubkey-quest:save:2")
	if _, ok := Get("npub1abc", "pubkey-quest:save:2"); ok {
		t.Error("removed slot still there")
	}
	if err := Record("../evil", "d", Entry{}); err == nil {
		t.Error("a path-like npub must be refused")
	}
}
