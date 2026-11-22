package stats

// The lifetime stats play adds (docs/draft/competition-plan.md, slice 2).
// Badges and highscores are judged on these.
const (
	FishLanded   = "fish_landed"  // counter: every fish landed
	FishSpecies  = "fish_species" // set: species caught (every-fish)
	FishOunces   = "fish_oz:"     // best, + species: the biggest of each, in ounces
	GemTypes     = "gem_types"    // set: gem types found, pearls too (every-gem)
	OresMined    = "ores"         // set: ores mined (every-ore)
	Slain        = "monsters_slain"
	ToughestKill = "toughest_kill" // best: the highest-XP monster defeated; label = its id
	Dailies      = "dailies_completed"
	Weeklies     = "weeklies_completed"
)
