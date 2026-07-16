package status_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/status"
	"pubkey-quest/types"
)

// A full night (FullRestMinutes) restores HP and mana in full; a partial rest
// restores proportionally; it never overheals; and because it's a fraction of
// Max, a higher-level (bigger MaxHP) character recovers more absolute HP for the
// same time rested — so a night still restores most/all at any level.
func TestRestoreVitalsForRest(t *testing.T) {
	full := &types.SaveFile{HP: 0, MaxHP: 80, Mana: 0, MaxMana: 40}
	hp, mana := status.RestoreVitalsForRest(full, status.FullRestMinutes)
	if full.HP != 80 || full.Mana != 40 || hp != 80 || mana != 40 {
		t.Fatalf("full rest should top off: HP=%d Mana=%d (returned %d/%d)", full.HP, full.Mana, hp, mana)
	}

	half := &types.SaveFile{HP: 0, MaxHP: 80, Mana: 0, MaxMana: 40}
	status.RestoreVitalsForRest(half, status.FullRestMinutes/2)
	if half.HP != 40 || half.Mana != 20 {
		t.Fatalf("half rest should restore ~half: HP=%d Mana=%d", half.HP, half.Mana)
	}

	capped := &types.SaveFile{HP: 78, MaxHP: 80, Mana: 40, MaxMana: 40}
	if hp, mana := status.RestoreVitalsForRest(capped, status.FullRestMinutes); capped.HP != 80 || capped.Mana != 40 || hp != 2 || mana != 0 {
		t.Fatalf("must not overheal: HP=%d Mana=%d (returned %d/%d)", capped.HP, capped.Mana, hp, mana)
	}

	low := &types.SaveFile{HP: 0, MaxHP: 12}
	high := &types.SaveFile{HP: 0, MaxHP: 96}
	lowHP, _ := status.RestoreVitalsForRest(low, status.FullRestMinutes/2)
	highHP, _ := status.RestoreVitalsForRest(high, status.FullRestMinutes/2)
	if highHP <= lowHP {
		t.Fatalf("recovery should scale with level: low=%d high=%d", lowHP, highHP)
	}
}
