package npc

import "testing"

// The booking says what to bring and when, in the innkeeper's words.
func TestShowWords(t *testing.T) {
	for _, tc := range []struct {
		ids  []string
		want string
	}{
		{[]string{"lute"}, "a lute"},
		{[]string{"lute", "flute"}, "a lute and a flute"},
		{[]string{"pan-flute", "dulcimer", "drum"}, "a pan flute, a dulcimer and a drum"},
		{nil, "your instrument"},
	} {
		if got := InstrumentList(tc.ids); got != tc.want {
			t.Errorf("InstrumentList(%v) = %q, want %q", tc.ids, got, tc.want)
		}
	}
	for _, tc := range []struct {
		at   int
		want string
	}{
		{1260, "9 PM"}, {1200, "8 PM"}, {1230, "8:30 PM"}, {0, "12 AM"}, {720, "12 PM"}, {90, "1:30 AM"},
	} {
		if got := ShowClock(tc.at); got != tc.want {
			t.Errorf("ShowClock(%d) = %q, want %q", tc.at, got, tc.want)
		}
	}
	show := map[string]interface{}{"required_instruments": []interface{}{"lute", "flute"}}
	if got := ShowInstruments(show); len(got) != 2 || got[0] != "lute" {
		t.Errorf("ShowInstruments = %v", got)
	}
}
