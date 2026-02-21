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
