package utils

import (
	"encoding/json"
	"log"
	"os"

	"pubkey-quest/types"
)

// UpdatesPath is the player-facing update notes, newest first. The homepage
// Updates tab renders it; the in-game "what's new" popup fetches the same file
// from /res/updates.json (src/ui/whatsNew.js); the Dungeon Master posts each.
var UpdatesPath = "www/res/updates.json"

// LoadUpdates reads the update notes, newest first; a missing or broken file
// means none.
func LoadUpdates() []types.UpdateEntry {
	b, err := os.ReadFile(UpdatesPath)
	if err != nil {
		return nil
	}
	var f struct {
		Updates []types.UpdateEntry `json:"updates"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		log.Printf("⚠️ %s: %v", UpdatesPath, err)
		return nil
	}
	return f.Updates
}
