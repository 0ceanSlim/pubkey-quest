package presence

import (
	"testing"
	"time"
)

func TestHereFiltersSpotTimeAndOnline(t *testing.T) {
	now := time.Now()
	spot := "goldenhaven|east|sailors_tavern|taproom"
	at := func(npub string, abs int, seenAgo time.Duration, where string) {
		Publish(Entry{Npub: npub, Name: npub, Spot: where, AbsMinutes: abs, LastSeen: now.Add(-seenAgo)})
	}
	t.Cleanup(func() {
		for _, n := range []string{"me", "near", "late", "gone", "elsewhere", "road"} {
			Remove(n)
		}
	})

	at("me", 1000, 0, spot)
	at("near", 1045, time.Second, spot)                                // same room, 45 game-minutes apart
	at("late", 1180, time.Second, spot)                                // same room, 3 hours apart
	at("gone", 1000, 2*OnlineWindow, spot)                             // went quiet
	at("elsewhere", 1000, time.Second, "goldenhaven|east|sailors_tavern|cellar") // other room
	at("road", 1000, time.Second, "")                                  // travelling

	got := Here("me", spot, 1000, now)
	if len(got) != 1 || got[0].Npub != "near" {
		t.Fatalf("Here = %+v, want only 'near'", got)
	}
	if Here("road", "", 1000, now) != nil {
		t.Error("a player on the road sees no one")
	}
}

func TestPublishReplacesAndPrune(t *testing.T) {
	now := time.Now()
	Publish(Entry{Npub: "p", SaveID: "a", LastSeen: now})
	Publish(Entry{Npub: "p", SaveID: "b", LastSeen: now})
	if e, _ := Get("p"); e.SaveID != "b" {
		t.Errorf("latest save should win, got %q", e.SaveID)
	}
	Prune(time.Minute, now.Add(2*time.Minute))
	if _, ok := Get("p"); ok {
		t.Error("a long-quiet player should be pruned")
	}
}
