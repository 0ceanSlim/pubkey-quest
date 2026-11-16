package session

import "testing"

func TestValidSlot(t *testing.T) {
	good := "npub1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqshp52w2"
	if err := ValidSlot(good, "save_1792707234"); err != nil {
		t.Fatalf("a real slot refused: %v", err)
	}
	for _, c := range [][2]string{
		{good, "../../ledger/" + good},
		{good, "./save_1"},
		{good, "save_1.json"},
		{good, ""},
		{"0000000000000000000000000000000000000000000000000000000000000001", "save_1"}, // hex: a second spelling
		{"npub1test", "save_1"},
		{"../npub", "save_1"},
	} {
		if ValidSlot(c[0], c[1]) == nil {
			t.Errorf("accepted %q / %q", c[0], c[1])
		}
	}
	if _, err := NewSessionManager().LoadSession(good, "../x", nil, nil, nil, nil); err == nil {
		t.Error("LoadSession took a path for a save id")
	}
}
