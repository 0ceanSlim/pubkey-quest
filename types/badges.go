package types

// Badges (game-data/systems/badges.json, docs/draft/badges.md): each is an
// achievement the Dungeon Master awards, judged at a kept save.

// BadgesConfig is badges.json.
type BadgesConfig struct {
	Badges []BadgeDef `json:"badges"`
}

// BadgeDef is one badge.
type BadgeDef struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Rim         string    `json:"rim"`            // gold, silver, bronze, gems, iron, legend, level
	Rank        string    `json:"rank,omitempty"` // a series shown by its highest earned ("level")
	Rule        BadgeRule `json:"rule"`
}

// BadgeRule is what earns a badge: exactly one of these is set.
type BadgeRule struct {
	Quest    string   `json:"quest,omitempty"`     // this quest is done
	AnyQuest []string `json:"any_quest,omitempty"` // one of these is done
	// Collection: every one of a set. fish, gems and ores are lifetime (any
	// of the player's characters); songs, towns, wilds and places are one
	// character's save.
	Collection string `json:"collection,omitempty"`
	Level      int    `json:"level,omitempty"`   // a character reached this level
	Counter    string `json:"counter,omitempty"` // a lifetime counter reached At
	At         int64  `json:"at,omitempty"`
}

// The collections a BadgeRule can name.
const (
	CollectionFish   = "fish"
	CollectionGems   = "gems"
	CollectionOres   = "ores"
	CollectionSongs  = "songs"
	CollectionTowns  = "towns"
	CollectionWilds  = "wilds"
	CollectionPlaces = "places"
)

// BadgeMediaConfig is badge-media.json: where each badge's art is on Blossom.
type BadgeMediaConfig struct {
	Server string                `json:"server"`
	Badges map[string]BadgeMedia `json:"badges"`
}

// BadgeMedia is a badge's image (1024) and thumbs (256, 64).
type BadgeMedia struct {
	Image    string `json:"image"`
	Thumb256 string `json:"thumb_256"`
	Thumb64  string `json:"thumb_64"`
}
