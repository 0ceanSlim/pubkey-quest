package types

// UpdateEntry is one player-facing update note (www/res/updates.json): the
// homepage Updates tab, the in-game "what's new" popup, and the Dungeon
// Master's post when it goes live.
type UpdateEntry struct {
	ID         string   `json:"id"`
	When       string   `json:"when"`
	Icon       string   `json:"icon"`
	Size       string   `json:"size"` // "major" or "minor"
	Title      string   `json:"title"`
	Summary    string   `json:"summary"`
	Highlights []string `json:"highlights"`
}
