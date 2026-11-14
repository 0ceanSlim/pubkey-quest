package types

// Mining config (docs/draft/mining-system.md), mirrors
// game-data/systems/mining.json. A seam's faces live on POI "mine" nodes
// (POIStep.Seam); anything a seam leaves out falls back to its tier here.

// MiningConfig is mining.json.
type MiningConfig struct {
	Tiers []MiningTier         `json:"tiers"` // tier 1 first
	Ores  map[string]MiningOre `json:"ores"`  // item id → what it takes to cut
	Rules MiningRules          `json:"rules"`
}

// MiningOre is an ore's tier and the pick tier that cuts it.
type MiningOre struct {
	Tier int `json:"tier"`
	Pick int `json:"pick"`
}

// MiningTier is what a face of a tier is like by default.
type MiningTier struct {
	Tier        int     `json:"tier"`
	DC          int     `json:"dc"`
	FacesPerDay int     `json:"faces_per_day"`
	Grid        [2]int  `json:"grid"`   // Veins: columns × rows
	Veins       [2]int  `json:"veins"`  // Veins: ore veins, [min, max]
	Gems        [2]int  `json:"gems"`   // Veins: gem cells
	Faults      [2]int  `json:"faults"` // Veins: fault cells
	Hard        float64 `json:"hard"`   // Veins: share of hard rock
	CaveInDice  int     `json:"cave_in_dice"`
	XP          int     `json:"xp"`       // base XP for a face worked well
	Exertion    int     `json:"exertion"` // work toward fatigue per face (rules.exertion_per_fatigue)
}

// MiningRules are the times and costs shared by every face.
type MiningRules struct {
	SwingMinutes       int `json:"swing_minutes"`
	SoundMinutes       int `json:"sound_minutes"`
	QuickMinutes       int `json:"quick_minutes"`
	ExertionPerFatigue int `json:"exertion_per_fatigue"`
}

// Seam is a mine node's rock face. Zero fields take the tier's defaults.
type Seam struct {
	Tier        int       `json:"tier"`
	DC          int       `json:"dc,omitempty"`
	FacesPerDay int       `json:"faces_per_day,omitempty"`
	Ores        []SeamOre `json:"ores"`
	// QuickLoot is what a quick roll yields on a success. Left out, it's one
	// roll per ore weight, twice.
	QuickLoot *POILootTable `json:"quick_loot,omitempty"`
}

// SeamOre is one ore a face can hold.
type SeamOre struct {
	Item     string `json:"item"`
	Weight   int    `json:"weight"`
	Quantity [2]int `json:"quantity"`
}
