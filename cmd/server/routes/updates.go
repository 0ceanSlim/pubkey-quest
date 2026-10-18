package routes

import (
	"encoding/json"
	"log"
	"os"
)

// updatesPath is the player-facing update notes, newest first. The homepage
// Updates tab renders it here; the in-game "what's new" popup fetches the same
// file from /res/updates.json (src/ui/whatsNew.js).
const updatesPath = "www/res/updates.json"

// UpdateEntry is one update in the notes.
type UpdateEntry struct {
	ID         string   `json:"id"`
	When       string   `json:"when"`
	Icon       string   `json:"icon"`
	Size       string   `json:"size"` // "major" or "minor"
	Title      string   `json:"title"`
	Summary    string   `json:"summary"`
	Highlights []string `json:"highlights"`
}

// loadUpdates reads the update notes; a missing or broken file means none.
func loadUpdates() []UpdateEntry {
	b, err := os.ReadFile(updatesPath)
	if err != nil {
		return nil
	}
	var f struct {
		Updates []UpdateEntry `json:"updates"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		log.Printf("⚠️ %s: %v", updatesPath, err)
		return nil
	}
	return f.Updates
}
