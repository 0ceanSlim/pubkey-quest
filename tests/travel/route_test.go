package travel_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/travel"
)

// A POI a fifth of the way along the route (from Connects[0]) is a fifth of the
// way along your bar when you set out from that end, and four fifths when you
// set out from the other end — or turn back.
func TestRouteFractionMirrorsByDirection(t *testing.T) {
	env := &travel.EnvironmentData{ID: "merchants-highway", Connects: []string{"kingdom-east", "goldenhaven-west"}}
	if got := travel.RouteFraction(env, "kingdom-east", 0.2); got != 0.2 {
		t.Errorf("from the first end: %v, want 0.2", got)
	}
	if got := travel.RouteFraction(env, "goldenhaven-west", 0.2); got < 0.799 || got > 0.801 {
		t.Errorf("from the far end: %v, want 0.8", got)
	}
	// Its own inverse: progress → route → progress.
	p := 0.37
	if back := travel.RouteFraction(env, "goldenhaven-west", travel.RouteFraction(env, "goldenhaven-west", p)); back < p-1e-9 || back > p+1e-9 {
		t.Errorf("round trip %v → %v", p, back)
	}
}
