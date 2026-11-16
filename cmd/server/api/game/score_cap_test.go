package game

import (
	"testing"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/tests/helpers"
	"pubkey-quest/types"
)

// A skill stops at 20 however much gear and effects add to it.
func TestSkillsStopAtTwenty(t *testing.T) {
	helpers.SetupTestEnvironment(t)
	if err := serverdb.InitDatabase(); err != nil {
		t.Fatal(err)
	}
	save := &types.SaveFile{
		Location: "kingdom",
		Stats: map[string]interface{}{
			"Strength": 20.0, "Dexterity": 20.0, "Constitution": 20.0,
			"Intelligence": 20.0, "Wisdom": 20.0, "Charisma": 20.0,
		},
		ActiveEffects: []types.ActiveEffect{{EffectID: "attuned-to-nature"}}, // Survival +2
		Inventory:     map[string]interface{}{},
	}
	if got := buildQuestContext(save).SkillValue("survival"); got != types.ScoreCap {
		t.Errorf("survival = %d, want the cap of %d", got, types.ScoreCap)
	}
}
