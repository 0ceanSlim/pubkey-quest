package game

import (
	"testing"

	"pubkey-quest/cmd/server/game/locks"
	"pubkey-quest/cmd/server/game/poi"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/types"
)

// No saving mid-fight or mid-anything, in either save mode.
func TestSaveBlockedMidActivity(t *testing.T) {
	if why := saveBlocked(&session.GameSession{}); why != "" {
		t.Fatalf("a resting session should save: %q", why)
	}
	for name, s := range map[string]*session.GameSession{
		"fight":   {ActiveCombat: &types.CombatSession{}},
		"unaware": {PendingEncounter: &types.PendingEncounter{}},
		"poi":     {ActivePOI: &poi.Session{}},
		"lock":    {LockSession: &locks.Session{}},
	} {
		if saveBlocked(s) == "" {
			t.Errorf("saving allowed mid-%s", name)
		}
	}
}
