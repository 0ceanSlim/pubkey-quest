package combat_test

import (
	"strings"
	"testing"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/combat"
)

// Creeping up unseen (a sneak attack, or a POI's "surprise" monster node) puts
// you first with the monsters surprised, so no initiative dice are shown. A
// normal fight still rolls them.
func TestSneakAttackSkipsTheInitiativeRoll(t *testing.T) {
	combatSetup(t)
	adv, err := character.LoadAdvancement(db.GetDB())
	if err != nil {
		t.Fatal(err)
	}
	for _, sneak := range []bool{true, false} {
		cs, err := combat.StartEncounter(db.GetDB(), fighterSave(), "npub_test",
			combat.EncounterSpec{MonsterIDs: []string{"bandit", "bandit"}, EnvironmentID: "forest", MonstersSurprised: sneak}, adv)
		if err != nil {
			t.Fatal(err)
		}
		rolled := strings.Contains(strings.Join(cs.Log, "\n"), "Rolling initiative")
		if rolled == sneak {
			t.Errorf("sneak=%v: initiative rolled in the log = %v", sneak, rolled)
		}
		if !sneak {
			continue
		}
		if cs.Initiative[0].Type != "player" {
			t.Errorf("sneak attack: %s goes first, not the player", cs.Initiative[0].Name)
		}
		for _, m := range cs.Monsters {
			if !m.Surprised {
				t.Errorf("%s isn't surprised", m.Name)
			}
		}
	}
}
