package combat_test

import (
	"strings"
	"testing"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/combat"
	"pubkey-quest/types"
)

// Saves store ability scores capitalised; combat once read them as the default
// 10 because it only looked up lowercase keys.
func TestCombatReadsCapitalisedStats(t *testing.T) {
	stats := map[string]interface{}{"Strength": float64(16), "Dexterity": 14}
	if got := combat.GetStatFromMap(stats, "strength"); got != 16 {
		t.Errorf("Strength read as %d, want 16", got)
	}
	if got := combat.GetStatFromMap(stats, "dexterity"); got != 14 {
		t.Errorf("Dexterity read as %d, want 14", got)
	}
}

// Monster hit dice carry a modifier ("2d8+2"); they once parsed as malformed and
// spawned the monster at 1 HP.
func TestDiceWithModifiers(t *testing.T) {
	for i := 0; i < 200; i++ {
		if v := combat.RollDice("2d8+2", false); v < 4 || v > 18 {
			t.Fatalf("2d8+2 rolled %d", v)
		}
		if v := combat.RollDice("1d4-1", false); v < 1 || v > 3 {
			t.Fatalf("1d4-1 rolled %d", v)
		}
	}
	if c, s, m, err := combat.ParseDiceExpr("7d10+21"); err != nil || c != 7 || s != 10 || m != 21 {
		t.Errorf("7d10+21 → %d %d %d %v", c, s, m, err)
	}
}

func downedFight(t *testing.T, monsterID string) (*types.CombatSession, *types.SaveFile) {
	t.Helper()
	combatSetup(t)
	adv, err := character.LoadAdvancement(db.GetDB())
	if err != nil {
		t.Fatal(err)
	}
	save := fighterSave()
	cs, err := combat.StartEncounter(db.GetDB(), save, "npub_test", combat.EncounterSpec{MonsterIDs: []string{monsterID}, EnvironmentID: "forest"}, adv)
	if err != nil {
		t.Fatal(err)
	}
	st := &combat.Actor(cs).CombatState
	st.CurrentHP, st.IsUnconscious = 0, true
	cs.Phase = "death_saves"
	return cs, save
}

func TestNonRelentlessMonsterStopsAttackingTheDowned(t *testing.T) {
	cs, save := downedFight(t, "giant-rat") // a beast, not relentless, unhurt
	for round := 0; round < 10 && cs.Phase == "death_saves"; round++ {
		log := strings.Join(combat.ProcessDeathSave(cs, save), "\n")
		if strings.Contains(log, "attacks:") {
			t.Fatalf("a non-relentless monster attacked a downed player:\n%s", log)
		}
	}
	switch cs.Phase {
	case "active":
		if combat.Actor(cs).CombatState.CurrentHP != 1 {
			t.Error("getting back up should be at 1 HP")
		}
	case "defeat":
		// three failed saves, fair enough
	default:
		t.Errorf("ended in phase %q", cs.Phase)
	}
}

func TestHurtPredatorWalksAwayFromTheDowned(t *testing.T) {
	cs, save := downedFight(t, "giant-rat")
	m := &cs.Monsters[0]
	m.CurrentHP = max(1, m.MaxHP/2)
	log := strings.Join(combat.ProcessDeathSave(cs, save), "\n")
	if cs.Phase == "defeat" {
		t.Skip("rolled a natural 1 on the save before the rat's turn")
	}
	if !strings.Contains(log, "limps away") && cs.Phase != "active" {
		t.Fatalf("a hurt beast should leave a downed player:\n%s", log)
	}
	if cs.Phase == "death_saves" {
		t.Fatalf("with the only monster gone the player should be up:\n%s", log)
	}
	if hp := combat.Actor(cs).CombatState.CurrentHP; hp != 1 {
		t.Errorf("came to with %d HP, want 1", hp)
	}
}
