package types

// ScrollFilter narrows a "random-scroll" loot entry: a random spell scroll
// whose spell matches, picked with spell-level weights (common spells drop
// often, higher levels rarely — scroll-scribing.json loot_level_weights).
// Zero values don't filter (MaxLevel 0 = the scribing table's max).
type ScrollFilter struct {
	MinLevel int      `json:"min_level,omitempty"`
	MaxLevel int      `json:"max_level,omitempty"`
	Classes  []string `json:"classes,omitempty"` // any of these class lists
	Schools  []string `json:"schools,omitempty"` // any of these schools
}

// RandomScrollItem is the loot entry item id that rolls a random scroll.
const RandomScrollItem = "random-scroll"
