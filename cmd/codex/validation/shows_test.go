package validation

import (
	"encoding/json"
	"testing"
)

// An innkeeper who offers to book shows has some to book.
func TestShowProblems(t *testing.T) {
	npc := func(s string) map[string]interface{} {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(s), &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	offers := `"schedule": [{"available_actions": ["open_shop", "book_show"]}]`
	good := `"show_config": {"show_time": 1260, "booking_deadline": 1200, "shows_by_day": {"0": [
		{"id": "x-ballads", "name": "Ballads", "required_instruments": ["lute"]}]}}`

	for _, tc := range []struct {
		name, json string
		bad        bool
	}{
		{"offers, no shows", `{` + offers + `}`, true},
		{"offers in dialogue, no shows", `{"dialogue": {"see": {"action": "book_show"}}}`, true},
		{"offers, has shows", `{` + offers + `, ` + good + `}`, false},
		{"no offer, no shows", `{"schedule": [{"available_actions": ["open_shop"]}]}`, false},
		{"booking closes after the show", `{"show_config": {"show_time": 1200, "booking_deadline": 1260, "shows_by_day": {"0": [
			{"id": "a", "name": "A", "required_instruments": ["lute"]}]}}}`, true},
		{"a show with no instrument", `{"show_config": {"show_time": 1260, "booking_deadline": 1200, "shows_by_day": {"0": [
			{"id": "a", "name": "A", "required_instruments": []}]}}}`, true},
	} {
		if got := showProblems(npc(tc.json)); (len(got) > 0) != tc.bad {
			t.Errorf("%s: %v", tc.name, got)
		}
	}
}
