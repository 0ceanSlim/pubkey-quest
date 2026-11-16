package game

import "testing"

// A save's d tag is read back from relays and its id becomes a file path.
func TestSaveIDFromDTag(t *testing.T) {
	cases := map[string]string{
		"pubkey-quest:save:save_1792707234":       "save_1792707234",
		"pubkey-quest:save:../../ledger/npub1abc": "",
		"pubkey-quest:save:save_1/../x":           "",
		"pubkey-quest:save:":                      "",
		"other:save:save_1":                       "",
	}
	for d, want := range cases {
		if got := SaveIDFromDTag(d); got != want {
			t.Errorf("%q → %q, want %q", d, got, want)
		}
	}
}
