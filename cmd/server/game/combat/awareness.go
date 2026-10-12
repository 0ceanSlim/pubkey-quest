package combat

import (
	"database/sql"
	"fmt"
	"math/rand"

	"pubkey-quest/types"
)

// ─── Stealth openings (party-combat-plan §7) ─────────────────────────────────
//
// When an encounter starts, both sides roll to notice each other:
//   - you notice them: your passive Perception vs each monster's Stealth roll
//     (spotting any one of them gives the group away);
//   - they notice you: your Stealth roll vs the sharpest monster's passive
//     Perception.
//
// You notice them, they don't → the "unaware" choice: sneak away, sneak attack,
// or approach. They notice you, you don't → an ambush (you're surprised).
// Anything else → a normal fight.

// Opening is how an encounter begins.
type Opening string

const (
	OpeningNormal  Opening = "normal"
	OpeningUnaware Opening = "unaware" // they haven't noticed you: your choice
	OpeningAmbush  Opening = "ambush"  // they noticed you first: you're surprised
)

// AwarenessRoll is the result of both sides' noticing checks.
type AwarenessRoll struct {
	Opening Opening
	Log     []string
}

// monsterStealthMod is a monster's Stealth bonus: its stealth skill, else DEX.
func monsterStealthMod(m *types.MonsterData) int {
	if v, ok := m.Skills["stealth"]; ok {
		return v
	}
	return StatMod(m.Stats.Dexterity)
}

// monsterPassivePerception reads the stat block's passive Perception, falling
// back to 10 + its perception skill (or WIS).
func monsterPassivePerception(m *types.MonsterData) int {
	if m.Senses.PassivePerception > 0 {
		return m.Senses.PassivePerception
	}
	if v, ok := m.Skills["perception"]; ok {
		return 10 + v
	}
	return 10 + StatMod(m.Stats.Wisdom)
}

// sharpestPerception is the best passive Perception in the group.
func sharpestPerception(monsters []*types.MonsterData) int {
	best := 0
	for _, m := range monsters {
		if p := monsterPassivePerception(m); p > best {
			best = p
		}
	}
	return best
}

// RollAwareness rolls who noticed whom. stealthMod and passivePerception are
// the player's (from their Thieving and Perception skills).
func RollAwareness(rng *rand.Rand, stealthMod, passivePerception int, monsters []*types.MonsterData) AwarenessRoll {
	if len(monsters) == 0 {
		return AwarenessRoll{Opening: OpeningNormal}
	}
	// You notice them if you'd catch the least stealthy one.
	spotted := false
	for _, m := range monsters {
		if rng.Intn(20)+1+monsterStealthMod(m) <= passivePerception {
			spotted = true
			break
		}
	}
	dc := sharpestPerception(monsters)
	roll := rng.Intn(20) + 1
	hidden := roll+stealthMod >= dc

	out := AwarenessRoll{Opening: OpeningNormal}
	switch {
	case spotted && hidden:
		out.Opening = OpeningUnaware
	case !spotted && !hidden:
		out.Opening = OpeningAmbush
	}
	out.Log = []string{fmt.Sprintf("  👁 Stealth: rolled %d%s vs their perception %d — %s.",
		roll, formatModifier(stealthMod), dc, map[bool]string{true: "unseen", false: "seen"}[hidden])}
	return out
}

// SneakAway rolls to slip off unseen: your Stealth vs the sharpest passive
// Perception. On a failure the fight starts normally.
func SneakAway(rng *rand.Rand, stealthMod int, monsters []*types.MonsterData) (bool, string) {
	dc := sharpestPerception(monsters)
	roll := rng.Intn(20) + 1
	ok := roll+stealthMod >= dc
	line := fmt.Sprintf("  👁 Stealth: rolled %d%s vs their perception %d.", roll, formatModifier(stealthMod), dc)
	return ok, line
}

// LoadEncounterMonsters loads the stat blocks for an encounter's monster ids.
func LoadEncounterMonsters(db *sql.DB, ids []string) ([]*types.MonsterData, error) {
	out := make([]*types.MonsterData, 0, len(ids))
	for _, id := range ids {
		m, err := LoadMonsterByID(db, id)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}
