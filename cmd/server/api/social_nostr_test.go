package api

import (
	"strings"
	"testing"

	nostr "github.com/0ceanslim/grain/server/types"
)

func TestOneEntryChangeGuardsFollowLists(t *testing.T) {
	a, b, c := strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64)
	cur := &nostr.Event{Kind: 3, CreatedAt: 100, Content: "", Tags: [][]string{{"p", a}, {"p", b}, {"t", "games"}}}

	ok := func(tags [][]string, created int64, content string) error {
		return oneEntryChange(cur, &nostr.Event{Kind: 3, CreatedAt: created, Content: content, Tags: tags})
	}
	if err := ok([][]string{{"p", a}, {"p", b}, {"t", "games"}, {"p", c}}, 101, ""); err != nil {
		t.Errorf("adding one follow refused: %v", err)
	}
	if err := ok([][]string{{"p", a}, {"t", "games"}}, 101, ""); err != nil {
		t.Errorf("removing one follow refused: %v", err)
	}
	if ok([][]string{{"p", c}}, 101, "") == nil {
		t.Error("replacing the whole list was allowed")
	}
	if ok([][]string{{"p", a}, {"p", b}, {"p", c}}, 101, "") == nil {
		t.Error("dropping a non-p tag while adding was allowed")
	}
	if ok([][]string{{"p", a}, {"p", b}, {"t", "games"}, {"p", c}}, 100, "") == nil {
		t.Error("an edit that isn't newer was allowed")
	}
	if ok([][]string{{"p", a}, {"p", b}, {"t", "games"}, {"p", c}}, 101, "secret") == nil {
		t.Error("changing the private content was allowed")
	}
	if oneEntryChange(nil, &nostr.Event{Kind: 3, Tags: [][]string{{"p", a}, {"p", b}}}) == nil {
		t.Error("an unreadable list must not be replaced by a multi-entry one")
	}
}
