package character

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"pubkey-quest/types"
)

// LoadAdvancement loads XP progression data from the database.
// The advancement table stores the raw JSON blob from advancement.json.
func LoadAdvancement(db *sql.DB) ([]types.AdvancementEntry, error) {
	var dataJSON string
	err := db.QueryRow("SELECT data FROM advancement WHERE id = 'advancement'").Scan(&dataJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("advancement data not found in database — run migration first")
		}
		return nil, fmt.Errorf("failed to query advancement: %v", err)
	}

	var entries []types.AdvancementEntry
	if err := json.Unmarshal([]byte(dataJSON), &entries); err != nil {
		return nil, fmt.Errorf("failed to parse advancement data: %v", err)
	}

	return entries, nil
}

// GetLevelFromXP returns the character level for the given total XP.
func GetLevelFromXP(xp int, advancement []types.AdvancementEntry) int {
	level := 1
	for _, entry := range advancement {
		if xp >= entry.ExperiencePoints {
			level = entry.Level
		}
	}
	return level
}

// GetXPMultiplierForLevel returns the XP multiplier applied at the given level.
func GetXPMultiplierForLevel(level int, advancement []types.AdvancementEntry) float64 {
	for _, entry := range advancement {
		if entry.Level == level {
			return entry.XPMultiplier
		}
	}
	return 1.0
}

// WillLevelUp returns true if adding gainedXP to currentXP crosses a level boundary.
func WillLevelUp(currentXP, gainedXP int, advancement []types.AdvancementEntry) bool {
	return GetLevelFromXP(currentXP+gainedXP, advancement) > GetLevelFromXP(currentXP, advancement)
}

// GrantXP is the single entry point for awarding experience from any source —
// combat, performances, quests, exploration. It adds the XP, recomputes the
// derived maxima for the resulting level, and on a level gain restores HP/Mana
// to full (the level-up reward), returning old→new values so the caller can
// surface a level-up moment. Route ALL experience through here so level-ups are
// detected and shown consistently rather than only in combat.
func GrantXP(save *types.SaveFile, amount int, advancement []types.AdvancementEntry) types.LevelUpResult {
	res := types.LevelUpResult{GainedXP: amount}
	if save == nil {
		return res
	}

	oldLevel := GetLevelFromXP(save.Experience, advancement)
	res.OldLevel = oldLevel
	res.OldMaxHP = DeriveMaxHP(save.Class, oldLevel, save.Stats)
	res.OldMaxMana = DeriveMaxMana(save.Class, oldLevel, save.Stats)

	save.Experience += amount
	if save.Experience < 0 {
		save.Experience = 0
	}

	newLevel := GetLevelFromXP(save.Experience, advancement)
	res.NewLevel = newLevel
	save.MaxHP = DeriveMaxHP(save.Class, newLevel, save.Stats)
	save.MaxMana = DeriveMaxMana(save.Class, newLevel, save.Stats)
	res.NewMaxHP = save.MaxHP
	res.NewMaxMana = save.MaxMana

	if newLevel > oldLevel {
		res.Leveled = true
		// Level-up reward: restore to the new maxima.
		save.HP = save.MaxHP
		save.Mana = save.MaxMana
	} else {
		// Keep current vitals within the (unchanged) maxima.
		if save.HP > save.MaxHP {
			save.HP = save.MaxHP
		}
		if save.Mana > save.MaxMana {
			save.Mana = save.MaxMana
		}
	}
	return res
}
